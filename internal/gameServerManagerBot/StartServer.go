package gameServerManagerBot

import (
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	s3BucketName  = "game-servers-982247710512-us-west-2-an"
	gameNameDir   = "CoreKeeper"
	saveURLExpiry = 24 * time.Hour

	// TODO write a builder function with options for this templating
	// Current substitution order: SaveURL, WorldName, DiscordWebhookURL
	startupScriptTemplate = `#!/bin/bash
set -e

apt-get update -y
apt-get install -y docker.io curl

systemctl enable docker
systemctl start docker

mkdir -p /tmp/core-keeper-data
mkdir -p /tmp/core-keeper-dedicated

SAVE_URL="%s"
if [ -n "$SAVE_URL" ]; then
    curl -fSL "$SAVE_URL" -o /tmp/save.tar.gz
    tar -xzf /tmp/save.tar.gz -C /tmp/core-keeper-data
    rm /tmp/save.tar.gz
fi

docker pull escaping/core-keeper-dedicated:v2.8.1

docker run -d \
    --name core-keeper-dedicated \
    --restart unless-stopped \
    -e WORLD_NAME="%s" \
    -e MAX_PLAYERS=5 \
	-e DISCORD_WEBHOOK_URL="%s" \
    -v /tmp/core-keeper-data:/home/steam/core-keeper-data \
    -v /tmp/core-keeper-dedicated:/home/steam/core-keeper-dedicated \
    escaping/core-keeper-dedicated:v2.8.1
`
)

func handlerStartServer(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*HandlerResult, error) {
	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		},
		DeferredWork: func() error {
			return manager.startServer(ctx, interaction)
		},
		AcknowledgementResponse: "Creating server, this may take a few minutes...",
	}, nil
}

func (m *Manager) startServer(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	gameName := CoreKeeperGameName
	if !slices.Contains(SupportedGames, gameName) {
		return fmt.Errorf("unsupported game %q", gameName)
	}

	worldName := optionString(interaction, "world")
	if worldName == "" {
		return fmt.Errorf("missing required option: world")
	}

	instances, err := m.vultrLayer.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("cannot list instances: %w", err)
	}
	if len(instances) >= vultrlayer.MaxServerCount {
		return fmt.Errorf("server limit of %d reached — destroy an existing server before creating a new one", vultrlayer.MaxServerCount)
	}

	isNew := strings.EqualFold(worldName, "new")

	var label string
	if isNew {
		label = fmt.Sprintf("corekeeper-new-%d", time.Now().UnixMilli())
	} else {
		label = fmt.Sprintf("corekeeper-%s", worldName)
	}

	var saveURL string
	if !isNew {
		s3Key := fmt.Sprintf("%s/%s.tar.gz", gameNameDir, worldName)
		saveURL, err = m.GeneratePresignedGetURL(ctx, s3BucketName, s3Key, saveURLExpiry)
		if err != nil {
			return fmt.Errorf("cannot generate save download URL: %w", err)
		}
	}

	startupScript := fmt.Sprintf(startupScriptTemplate, saveURL, label)

	if err := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Provisioning instance `%s`...", label)); err != nil {
		log.Printf("Error sending followup: %s", err)
	}

	instance, err := m.vultrLayer.CreateInstance(ctx, label, startupScript)
	if err != nil {
		return fmt.Errorf("cannot create instance %q: %w", label, err)
	}

	return sendFollowup(ctx, interaction.Interaction, fmt.Sprintf(
		"Server `%s` created (ID: `%s`). It will be ready in a few minutes once the startup script completes.",
		instance.Label, instance.ID,
	))
}

// optionString returns the string value of a named slash-command option, or "".
func optionString(interaction *discordgo.InteractionCreate, name string) string {
	for _, opt := range interaction.ApplicationCommandData().Options {
		if opt.Name == name {
			return opt.StringValue()
		}
	}
	return ""
}
