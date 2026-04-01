package main

import (
	"4dmiral/discordServerManager/internal/gameServerManagerBot"
	"4dmiral/discordServerManager/internal/secrets"
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/scheduler"
)

func init() {
	if err := secrets.GetSecretsWithSDK(context.Background()); err != nil {
		log.Fatalf("Error getting secrets: %s", err)
	}
}

func main() {
	lambda.Start(handler)
}

// handler accepts both Lambda Function URL requests (from Discord) and
// EventBridge Scheduler payloads (for auto-shutdown). The two are distinguished
// by the presence of the "requestContext" field.
func handler(ctx context.Context, rawEvent json.RawMessage) (interface{}, error) {
	vultrLayer, err := vultrlayer.New(ctx, secrets.Secrets.VultrAPIKey)
	if err != nil {
		return nil, fmt.Errorf("create vultr layer: %w", err)
	}

	r2Presigner, err := newR2Presigner(ctx)
	if err != nil {
		return nil, fmt.Errorf("create R2 presigner: %w", err)
	}

	schedulerClient, err := newSchedulerClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create scheduler client: %w", err)
	}

	bot := gameServerManagerBot.New(vultrLayer, r2Presigner, schedulerClient)

	// Function URL events always have a "requestContext" field.
	var peek struct {
		RequestContext *json.RawMessage `json:"requestContext"`
	}
	json.Unmarshal(rawEvent, &peek)

	if peek.RequestContext != nil {
		var req events.LambdaFunctionURLRequest
		if err := json.Unmarshal(rawEvent, &req); err != nil {
			return nil, fmt.Errorf("unmarshal function URL event: %w", err)
		}
		return bot.HandleRequest(ctx, &req)
	}

	// Otherwise treat it as an EventBridge Scheduler auto-shutdown event.
	var event gameServerManagerBot.AutoShutdownEvent
	if err := json.Unmarshal(rawEvent, &event); err != nil {
		return nil, fmt.Errorf("unmarshal auto-shutdown event: %w", err)
	}
	return nil, bot.HandleAutoShutdown(ctx, event)
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

// newSchedulerClient builds an EventBridge Scheduler client using the
// Lambda's ambient IAM role credentials.
func newSchedulerClient(ctx context.Context) (*scheduler.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return scheduler.NewFromConfig(cfg), nil
}
