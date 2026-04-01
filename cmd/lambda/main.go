package main

import (
	"4dmiral/discordServerManager/internal/gameServerManagerBot"
	"4dmiral/discordServerManager/internal/secrets"
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
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

	vultrLayer, err := vultrlayer.New(ctx, secrets.Secrets.VultrAPIKey)
	if err != nil {
		return events.LambdaFunctionURLResponse{StatusCode: 500}, fmt.Errorf("error creating vultr layer: %w", err)
	}

	r2Presigner, err := newR2Presigner(ctx)
	if err != nil {
		return events.LambdaFunctionURLResponse{StatusCode: 500}, fmt.Errorf("error creating R2 presigner: %w", err)
	}

	bot := gameServerManagerBot.New(vultrLayer, r2Presigner)
	return bot.HandleRequest(ctx, &req)
}

// newR2Presigner builds a pre-sign client pointed at Cloudflare R2.
// R2 is S3-compatible so the AWS SDK is used with a custom endpoint.
func newR2Presigner(ctx context.Context) (*s3.PresignClient, error) {
	r2Endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", secrets.Secrets.R2AccountID)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			secrets.Secrets.R2AccessKeyID,
			secrets.Secrets.R2SecretAccessKey,
			"",
		)),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("load R2 config: %w", err)
	}

	r2Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = &r2Endpoint
		o.UsePathStyle = true
	})
	return s3.NewPresignClient(r2Client), nil
}
