package main

import (
	"4dmiral/discordServerManager/internal"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/bwmarrin/discordgo"
)

var discordPublicKey ed25519.PublicKey

func init() {
	if err := internal.GetSecretsWithSDK(context.Background()); err != nil {
		log.Fatalf("Error getting secrets: %s", err)
	}

	key, err := hex.DecodeString(internal.Secrets.DiscordPublicKey)
	if err != nil {
		log.Fatalf("Error decoding public key: %s", err)
	}
	discordPublicKey = ed25519.PublicKey(key)
}

type Input struct{}

func handler(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	// we only allow discord to call this endpoint
	if !verifyDiscordRequest(req) {
		return events.LambdaFunctionURLResponse{StatusCode: 401}, nil
	}

	// parse interaction
	var interaction discordgo.Interaction
	if err := json.Unmarshal([]byte(req.Body), &interaction); err != nil {
		return events.LambdaFunctionURLResponse{StatusCode: 400}, nil
	}

	resp, err := responseForInteraction(interaction)
	if err != nil {
		return events.LambdaFunctionURLResponse{StatusCode: 500}, nil
	}

	return resp, nil

}

func responseForInteraction(interaction discordgo.Interaction) (events.LambdaFunctionURLResponse, error) {
	switch interaction.Type {
	case discordgo.InteractionPing:
		return responsePong()
	case discordgo.InteractionApplicationCommand:
		return responseApplicationCommand(interaction)
	default:
		return events.LambdaFunctionURLResponse{StatusCode: 400}, nil
	}
}

func main() {
	lambda.Start(handler)
}

func verifyDiscordRequest(req events.LambdaFunctionURLRequest) bool {
	r, _ := http.NewRequest("POST", "/", strings.NewReader(req.Body))
	for k, v := range req.Headers {
		r.Header.Set(k, v)
	}
	return discordgo.VerifyInteraction(r, discordPublicKey)
}

func responsePong() (events.LambdaFunctionURLResponse, error) {
	resp := discordgo.InteractionResponse{
		Type: discordgo.InteractionResponsePong,
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return events.LambdaFunctionURLResponse{}, err
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: 200,
		Body:       string(body),
		Headers:    map[string]string{"Content-Type": "application/json"}}, nil
}

// Slash command
func responseApplicationCommand(interaction discordgo.Interaction) (events.LambdaFunctionURLResponse, error) {
	return events.LambdaFunctionURLResponse{}, nil
}
