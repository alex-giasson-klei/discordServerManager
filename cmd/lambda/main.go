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
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
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

	r2Client, err := newR2Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("create R2 client: %w", err)
	}

	schedulerClient, err := newSchedulerClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create scheduler client: %w", err)
	}

	lambdaClient, err := newLambdaClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create lambda client: %w", err)
	}

	bot := gameServerManagerBot.New(vultrLayer, r2Client, s3.NewPresignClient(r2Client), schedulerClient, lambdaClient)

	// Detect event type by probing well-known fields.
	var peek struct {
		Type           *string          `json:"type"`
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

	if peek.Type != nil && *peek.Type == "deferred_command" {
		var payload gameServerManagerBot.DeferredCommandPayload
		if err := json.Unmarshal(rawEvent, &payload); err != nil {
			return nil, fmt.Errorf("unmarshal deferred command payload: %w", err)
		}
		return nil, bot.HandleDeferredCommand(ctx, payload.Interaction)
	}

	// Otherwise treat it as an EventBridge Scheduler auto-shutdown event.
	var event gameServerManagerBot.AutoShutdownEvent
	if err := json.Unmarshal(rawEvent, &event); err != nil {
		return nil, fmt.Errorf("unmarshal auto-shutdown event: %w", err)
	}
	return nil, bot.HandleAutoShutdown(ctx, event)
}

// newR2Client builds an S3 client pointed at Cloudflare R2.
// R2 is S3-compatible so the AWS SDK is used with a custom endpoint.
func newR2Client(ctx context.Context) (*s3.Client, error) {
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

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = &r2Endpoint
		o.UsePathStyle = true
	}), nil
}

// newLambdaClient builds a Lambda client using the Lambda's ambient IAM role.
func newLambdaClient(ctx context.Context) (*lambdaSDK.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for lambda client: %w", err)
	}
	return lambdaSDK.NewFromConfig(cfg), nil
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
