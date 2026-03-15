package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/discord"
	"4dmiral/discordServerManager/internal/secrets"
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bwmarrin/discordgo"
)

type Manager struct {
	vultrLayer  *vultrlayer.VultrLayer
	s3Presigner *s3.PresignClient
}

func New(vultrLayer *vultrlayer.VultrLayer, s3Presigner *s3.PresignClient) *Manager {
	return &Manager{
		vultrLayer:  vultrLayer,
		s3Presigner: s3Presigner,
	}
}

// GeneratePresignedGetURL returns a pre-signed S3 GET URL valid for the given duration.
func (m *Manager) GeneratePresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	req, err := m.s3Presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate pre-signed URL for s3://%s/%s: %w", bucket, key, err)
	}
	return req.URL, nil
}

func (m *Manager) VerifyDiscordInteractionRequest(req *events.LambdaFunctionURLRequest, publicKey string) (bool, error) {
	key, err := hex.DecodeString(publicKey)
	if err != nil {
		return false, fmt.Errorf("can't decode public key: %s", err)
	}

	r, _ := http.NewRequest("POST", "/", strings.NewReader(req.Body))
	for k, v := range req.Headers {
		r.Header.Set(k, v)
	}

	return discord.VerifyRequest(r, key), nil
}

func (m *Manager) HandleRequest(ctx context.Context, req *events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	// Only Discord Interactions is allowed to call this lambda
	verified, err := m.VerifyDiscordInteractionRequest(req, secrets.Secrets.DiscordPublicKey)
	if err != nil {
		return events.LambdaFunctionURLResponse{}, fmt.Errorf("verify discord request failed: %v", err)
	}
	if !verified {
		return events.LambdaFunctionURLResponse{StatusCode: 401}, nil
	}

	// Parse interaction
	var interaction discordgo.Interaction
	if err := json.Unmarshal([]byte(req.Body), &interaction); err != nil {
		return events.LambdaFunctionURLResponse{StatusCode: 400}, nil
	}
	log.Printf("%+v", interaction)

	resp, err := m.handleInteraction(ctx, interaction)
	if err != nil {
		return events.LambdaFunctionURLResponse{}, err
	}

	return resp, nil
}

func (m *Manager) handleInteraction(ctx context.Context, interaction discordgo.Interaction) (events.LambdaFunctionURLResponse, error) {
	switch interaction.Type {

	case discordgo.InteractionPing:
		resp := discord.ResponsePong(interaction)
		body, err := json.Marshal(resp)
		if err != nil {
			return events.LambdaFunctionURLResponse{}, fmt.Errorf("can't marshal pong response to json: %w", err)
		}
		return events.LambdaFunctionURLResponse{
			StatusCode: 200,
			Body:       string(body),
			Headers:    map[string]string{"Content-Type": "application/json"}}, nil

	case discordgo.InteractionApplicationCommand:
		data := interaction.ApplicationCommandData()
		log.Printf("applicationCommandData: %+v", data)
		handler, ok := Handlers[data.Name]
		if !ok {
			return unknownCommandResponse()
		}

		result, err := handler(ctx, &discordgo.InteractionCreate{Interaction: &interaction}, m)
		if err != nil {
			return events.LambdaFunctionURLResponse{}, fmt.Errorf("handler error: %w", err)
		}

		if result.DeferredWork != nil {
			if ackErr := acknowledgeInteraction(&interaction, result.AcknowledgementResponse); ackErr != nil {
				log.Printf("Failed to acknowledge interaction: %s", ackErr)
			}
			if workErr := result.DeferredWork(); workErr != nil {
				_ = sendFollowup(ctx, &interaction, workErr.Error())
				log.Printf("Error in deferred work: %s", workErr)
			}
		}

		body, err := json.Marshal(result.Response)
		if err != nil {
			return events.LambdaFunctionURLResponse{}, fmt.Errorf("can't marshal interaction response to json: %w", err)
		}
		return events.LambdaFunctionURLResponse{
			StatusCode: 200,
			Body:       string(body),
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	default:
		return events.LambdaFunctionURLResponse{}, fmt.Errorf("unsupported interaction type %s", interaction.Type)
	}
}

func unknownCommandResponse() (events.LambdaFunctionURLResponse, error) {
	resp := discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Unknown command",
		},
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return events.LambdaFunctionURLResponse{}, fmt.Errorf("can't marshal response to json: %w", err)
	}
	return events.LambdaFunctionURLResponse{
		StatusCode: 200,
		Body:       string(body),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}
