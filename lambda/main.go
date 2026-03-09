package main

import (
	"4dmiral/discordServerManager/internal/gameServerManagerBot"
	"4dmiral/discordServerManager/internal/secrets"
	"4dmiral/discordServerManager/internal/vultr"
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func init() {
	if err := secrets.GetSecretsWithSDK(context.Background()); err != nil {
		log.Fatalf("Error getting secrets: %s", err)
	}
}

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	log.Printf("%+v", req)
	vultrLayer := vultrlayer.New(ctx, secrets.Secrets.VultrAPIKey)
	bot := gameServerManagerBot.New(vultrLayer)

	return bot.HandleRequest(ctx, &req)
}
