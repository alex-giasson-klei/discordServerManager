package main

import (
	"4dmiral/discordServerManager/internal/gameServerManagerBot"
	"4dmiral/discordServerManager/internal/secrets"
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Error loading AWS config: %s", err)
	}
	s3Presigner := s3.NewPresignClient(s3.NewFromConfig(awsCfg))

	bot := gameServerManagerBot.New(vultrLayer, s3Presigner)
	return bot.HandleRequest(ctx, &req)
}
