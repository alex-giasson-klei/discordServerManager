package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/discord"
	"4dmiral/discordServerManager/internal/games"
	"4dmiral/discordServerManager/internal/secrets"
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/scheduler"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaSDKTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/bwmarrin/discordgo"
)

// DeferredCommandPayload is sent from the sync Lambda invocation to an async
// self-invocation so that long-running work happens outside the 3-second
// Discord response window.
type DeferredCommandPayload struct {
	Type        string               `json:"type"` // always "deferred_command"
	Interaction discordgo.Interaction `json:"interaction"`
}

type Manager struct {
	vultrLayer      *vultrlayer.VultrLayer
	s3Client        *s3.Client
	s3Presigner     *s3.PresignClient
	schedulerClient *scheduler.Client
	lambdaClient    *lambdaSDK.Client
}

func New(vultrLayer *vultrlayer.VultrLayer, s3Client *s3.Client, s3Presigner *s3.PresignClient, schedulerClient *scheduler.Client, lambdaClient *lambdaSDK.Client) *Manager {
	return &Manager{
		vultrLayer:      vultrLayer,
		s3Client:        s3Client,
		s3Presigner:     s3Presigner,
		schedulerClient: schedulerClient,
		lambdaClient:    lambdaClient,
	}
}

// SaveExists reports whether an object exists in R2 without downloading it.
func (m *Manager) SaveExists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := m.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *s3types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("check save existence: %w", err)
	}
	return true, nil
}

const maxSaveVersions = 3

// RotateSave copies the current save to a timestamped backup key, then prunes
// old backups so that only maxSaveVersions are kept. Call this before each
// shutdown so the incoming upload overwrites the latest without losing history.
//
// Backup layout:
//
//	{saveKey}                            ← current save (unchanged)
//	{base}/backups/20060102T150405Z.tar.gz  ← timestamped copies
//
// If no current save exists (first save), rotation is a no-op.
// A rotation failure is logged but never blocks the actual shutdown.
func (m *Manager) RotateSave(ctx context.Context, bucket, saveKey string) error {
	exists, err := m.SaveExists(ctx, bucket, saveKey)
	if err != nil {
		return fmt.Errorf("rotate save: check existence: %w", err)
	}
	if !exists {
		return nil // first-ever save; nothing to back up
	}

	base := saveKey[:strings.LastIndex(saveKey, "/")]
	backupPrefix := base + "/backups/"
	backupKey := backupPrefix + time.Now().UTC().Format("20060102T150405Z") + ".tar.gz"

	_, err = m.s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(bucket + "/" + saveKey),
		Key:        aws.String(backupKey),
	})
	if err != nil {
		return fmt.Errorf("rotate save: copy to backup %q: %w", backupKey, err)
	}

	list, err := m.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(backupPrefix),
	})
	if err != nil {
		return fmt.Errorf("rotate save: list backups: %w", err)
	}

	if len(list.Contents) <= maxSaveVersions {
		return nil
	}

	// Keys are ISO timestamps; lexicographic sort = chronological order.
	sort.Slice(list.Contents, func(i, j int) bool {
		return aws.ToString(list.Contents[i].Key) < aws.ToString(list.Contents[j].Key)
	})

	toDelete := list.Contents[:len(list.Contents)-maxSaveVersions]
	ids := make([]s3types.ObjectIdentifier, len(toDelete))
	for i, obj := range toDelete {
		ids[i] = s3types.ObjectIdentifier{Key: obj.Key}
	}
	_, err = m.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3types.Delete{Objects: ids},
	})
	if err != nil {
		return fmt.Errorf("rotate save: prune old backups: %w", err)
	}
	return nil
}

// ListSavedWorlds returns all world names that have a save in R2, grouped by game.
// It uses ListObjectsV2 with a delimiter to find "directories" without reading file content.
func (m *Manager) ListSavedWorlds(ctx context.Context, bucket string) (map[games.GameName][]string, error) {
	result := map[games.GameName][]string{}
	for _, gameName := range games.AllGameNames() {
		meta, _ := games.Meta(gameName)
		prefix := meta.SaveDirectory + "/"
		out, err := m.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:    aws.String(bucket),
			Prefix:    aws.String(prefix),
			Delimiter: aws.String("/"),
		})
		if err != nil {
			return nil, fmt.Errorf("list worlds for %s: %w", gameName, err)
		}
		for _, cp := range out.CommonPrefixes {
			// CommonPrefix looks like "CoreKeeper/myworld/" — strip prefix and trailing slash
			worldName := strings.TrimSuffix(strings.TrimPrefix(aws.ToString(cp.Prefix), prefix), "/")
			if worldName != "" {
				result[gameName] = append(result[gameName], worldName)
			}
		}
	}
	return result, nil
}

// GeneratePresignedGetURL returns a pre-signed S3 GET URL valid for the given duration.
func (m *Manager) GeneratePresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	req, err := m.s3Presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate pre-signed GET URL for s3://%s/%s: %w", bucket, key, err)
	}
	return req.URL, nil
}

// GeneratePresignedPutURL returns a pre-signed S3 PUT URL valid for the given duration.
func (m *Manager) GeneratePresignedPutURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	req, err := m.s3Presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to generate pre-signed PUT URL for s3://%s/%s: %w", bucket, key, err)
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
			log.Printf("handler error: %s", err)
			return discordErrorResponse(err)
		}

		if result.DeferredWork != nil {
			if err := m.invokeSelf(ctx, interaction); err != nil {
				log.Printf("failed to invoke self async: %s", err)
				return discordErrorResponse(fmt.Errorf("failed to start background processing: %w", err))
			}
			ackResp := &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: result.AcknowledgementResponse,
				},
			}
			body, err := json.Marshal(ackResp)
			if err != nil {
				return events.LambdaFunctionURLResponse{}, fmt.Errorf("can't marshal ack response: %w", err)
			}
			return events.LambdaFunctionURLResponse{
				StatusCode: 200,
				Body:       string(body),
				Headers:    map[string]string{"Content-Type": "application/json"},
			}, nil
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

// invokeSelf asynchronously invokes this Lambda with a DeferredCommandPayload
// so that long-running work runs outside Discord's 3-second response window.
func (m *Manager) invokeSelf(ctx context.Context, interaction discordgo.Interaction) error {
	payload, err := json.Marshal(DeferredCommandPayload{
		Type:        "deferred_command",
		Interaction: interaction,
	})
	if err != nil {
		return fmt.Errorf("marshal deferred command payload: %w", err)
	}
	_, err = m.lambdaClient.Invoke(ctx, &lambdaSDK.InvokeInput{
		FunctionName:   aws.String(secrets.Secrets.LambdaFunctionARN),
		InvocationType: lambdaSDKTypes.InvocationTypeEvent,
		Payload:        payload,
	})
	if err != nil {
		return fmt.Errorf("invoke self async: %w", err)
	}
	return nil
}

func unknownCommandResponse() (events.LambdaFunctionURLResponse, error) {	resp := discordgo.InteractionResponse{
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

func discordErrorResponse(err error) (events.LambdaFunctionURLResponse, error) {
	resp := discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("❌ Error: %s", err.Error()),
		},
	}
	body, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		return events.LambdaFunctionURLResponse{}, fmt.Errorf("can't marshal error response: %w", marshalErr)
	}
	return events.LambdaFunctionURLResponse{
		StatusCode: 200,
		Body:       string(body),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}
