package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

var SecretStoreID = os.Getenv("SECRET_STORE_ID")
var SecretStoreRegion = os.Getenv("SECRET_STORE_REGION")

var Secrets LambdaSecrets

type LambdaSecrets struct {
	VultrAPIKey      string
	DiscordAppID     string
	DiscordToken     string
	DiscordPublicKey string
	GuildIDs         []string
}

func GetSecretsWithSDK(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("error loading AWS SDK config: %s", err)
	}
	cfg.Region = SecretStoreRegion

	secretsClient := secretsmanager.NewFromConfig(cfg)

	input := secretsmanager.GetSecretValueInput{
		SecretId: aws.String(SecretStoreID),
	}
	value, err := secretsClient.GetSecretValue(ctx, &input)
	if err != nil {
		return fmt.Errorf("error retrieving secret value: %s", err)
	}

	if err := json.Unmarshal([]byte(*value.SecretString), &Secrets); err != nil {
		return fmt.Errorf("error unmarshalling secret value: %s", err)
	}

	return nil
}
