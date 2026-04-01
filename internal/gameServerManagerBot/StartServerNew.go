package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/games"
	"4dmiral/discordServerManager/internal/secrets"
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func handlerStartServerNew(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*HandlerResult, error) {
	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		},
		DeferredWork: func() error {
			return manager.startServerNew(ctx, interaction)
		},
		AcknowledgementResponse: "Creating server, this may take a few minutes...",
	}, nil
}

func (m *Manager) startServerNew(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	instances, err := m.vultrLayer.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("cannot list instances: %w", err)
	}
	if len(instances) >= vultrlayer.MaxServerCount {
		return fmt.Errorf("server limit of %d reached — destroy an existing server before creating a new one", vultrlayer.MaxServerCount)
	}

	gameName := games.CoreKeeperGameName
	template, err := games.StartupScriptTemplate(gameName)
	if err != nil {
		return err
	}

	worldName := optionString(interaction, "world")
	if worldName == "" {
		return fmt.Errorf("missing required option: world")
	}

	label := fmt.Sprintf("%s-%s", gameName, worldName)

	webhookURL := secrets.Secrets.GuildWebhooks[interaction.GuildID]

	agentBinaryURL, err := m.GeneratePresignedGetURL(ctx, secrets.Secrets.R2BucketName, agentBinaryKey, saveURLExpiry)
	if err != nil {
		return fmt.Errorf("cannot generate agent binary URL: %w", err)
	}

	startupScript := fmt.Sprintf(template,
		worldName,
		agentBinaryURL,
		secrets.Secrets.GameServerAgentSecret,
		"",
		webhookURL,
	)

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
