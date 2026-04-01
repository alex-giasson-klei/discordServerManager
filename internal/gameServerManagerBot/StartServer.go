package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/games"
	"4dmiral/discordServerManager/internal/secrets"
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	saveURLExpiry  = 24 * time.Hour
	agentBinaryKey = "bin/gameserver-agent"
	agentPort      = 8080
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
	gameName := games.GameName(optionString(interaction, "game"))
	meta, err := games.Meta(gameName)
	if err != nil {
		return err
	}
	template, err := games.StartupScriptTemplate(gameName)
	if err != nil {
		return err
	}

	worldName := optionString(interaction, "world")
	if worldName == "" {
		return fmt.Errorf("missing required option: world")
	}

	isNew := optionBool(interaction, "new")

	instances, err := m.vultrLayer.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("cannot list instances: %w", err)
	}
	if len(instances) >= vultrlayer.MaxServerCount {
		return fmt.Errorf("server limit of %d reached — destroy an existing server before creating a new one", vultrlayer.MaxServerCount)
	}

	label := fmt.Sprintf("%s-%s", gameName, worldName)

	if existing, _ := m.vultrLayer.GetInstanceByLabel(ctx, label); existing != nil {
		return fmt.Errorf("server `%s` is already running", label)
	}

	var saveURL string
	if !isNew {
		s3Key := fmt.Sprintf("%s/%s.tar.gz", meta.SaveDirectory, worldName)
		exists, err := m.SaveExists(ctx, secrets.Secrets.R2BucketName, s3Key)
		if err != nil {
			return fmt.Errorf("cannot check for existing save: %w", err)
		}
		if !exists {
			return fmt.Errorf("no save found for `%s/%s` — use `/startserver new:True` to create a fresh world", gameName, worldName)
		}
		saveURL, err = m.GeneratePresignedGetURL(ctx, secrets.Secrets.R2BucketName, s3Key, saveURLExpiry)
		if err != nil {
			return fmt.Errorf("cannot generate save download URL: %w", err)
		}
	}

	agentBinaryURL, err := m.GeneratePresignedGetURL(ctx, secrets.Secrets.R2BucketName, agentBinaryKey, saveURLExpiry)
	if err != nil {
		return fmt.Errorf("cannot generate agent binary URL: %w", err)
	}

	webhookURL, ok := secrets.Secrets.GuildWebhooks[interaction.GuildID]
	if !ok || webhookURL == "" {
		return fmt.Errorf("no Discord webhook configured for this server — ask an admin to add one")
	}

	startupScript := fmt.Sprintf(template,
		worldName,
		agentBinaryURL,
		secrets.Secrets.GameServerAgentSecret,
		saveURL,
		webhookURL,
	)

	if err := m.CreateAutoShutdownSchedule(ctx, label, interaction.GuildID); err != nil {
		return fmt.Errorf("cannot create auto-shutdown schedule (instance NOT created): %w", err)
	}

	if err := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Provisioning instance `%s`...", label)); err != nil {
		log.Printf("Error sending followup: %s", err)
	}

	instance, err := m.vultrLayer.CreateInstance(ctx, label, startupScript)
	if err != nil {
		m.DeleteAutoShutdownSchedule(ctx, label)
		return fmt.Errorf("cannot create instance %q: %w", label, err)
	}

	return sendFollowup(ctx, interaction.Interaction, fmt.Sprintf(
		"Server `%s` created (ID: `%s`). It will be ready in a few minutes. Auto-shutdown in 5 hours.",
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

// optionBool returns the bool value of a named slash-command option, or false.
func optionBool(interaction *discordgo.InteractionCreate, name string) bool {
	for _, opt := range interaction.ApplicationCommandData().Options {
		if opt.Name == name {
			return opt.BoolValue()
		}
	}
	return false
}
