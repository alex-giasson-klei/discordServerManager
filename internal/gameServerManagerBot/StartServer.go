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
	game := optionString(interaction, "game")
	world := optionString(interaction, "world")
	isNew := optionBool(interaction, "new")

	var ack string
	if isNew {
		ack = fmt.Sprintf("Creating new `%s` world `%s`... this may take a few minutes.", game, world)
	} else {
		ack = fmt.Sprintf("Loading `%s` world `%s`... this may take a few minutes.", game, world)
	}

	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		},
		DeferredWork: func() error {
			return manager.startServer(ctx, interaction)
		},
		AcknowledgementResponse: ack,
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

	s3Key := meta.SaveKey(worldName)

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

	saveExists, err := m.SaveExists(ctx, secrets.Secrets.R2BucketName, s3Key)
	if err != nil {
		return fmt.Errorf("cannot check for existing save: %w", err)
	}

	var saveURL string
	if isNew {
		if saveExists {
			return fmt.Errorf("a save already exists for `%s/%s` — choose a different world name, or use `/startserver new:False` to load %s/%s", gameName, worldName, gameName, worldName)
		}
	} else {
		if !saveExists {
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

	shutdownDuration := AutoShutdownDefaultDuration
	if timerStr := optionString(interaction, "auto_shutdown_timer"); timerStr != "" {
		parsed, err := time.ParseDuration(timerStr)
		if err != nil {
			return fmt.Errorf("invalid auto_shutdown_timer %q — use a format like 2h, 30m, or 1h30m", timerStr)
		}
		if parsed < autoShutdownMinDuration {
			return fmt.Errorf("auto_shutdown_timer must be at least %s", formatDuration(autoShutdownMinDuration))
		}
		if parsed > AutoShutdownMaxDuration {
			return fmt.Errorf("auto_shutdown_timer must be at most %s", formatDuration(AutoShutdownMaxDuration))
		}
		shutdownDuration = parsed
	}

	if err := m.CreateAutoShutdownSchedule(ctx, label, interaction.GuildID, shutdownDuration); err != nil {
		return fmt.Errorf("cannot create auto-shutdown schedule (instance NOT created): %w", err)
	}

	_, err = m.vultrLayer.CreateInstance(ctx, label, startupScript)
	if err != nil {
		if schedErr := m.DeleteAutoShutdownSchedule(ctx, label); schedErr != nil {
			log.Printf("warning: failed to clean up auto-shutdown schedule for %q after instance creation failure: %v", label, schedErr)
		}
		return fmt.Errorf("cannot create instance %q: %w", label, err)
	}

	return sendFollowup(ctx, interaction.Interaction, fmt.Sprintf(
		"`%s` world `%s` started. The Join Information will be posted in a few minutes when the server is ready! Auto-shutdown is in %s. To stop the server manually, use `/stopserver`",
		gameName, worldName, formatDuration(shutdownDuration),
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

// optionInt returns the int64 value of a named slash-command option, or 0.
func optionInt(interaction *discordgo.InteractionCreate, name string) int64 {
	for _, opt := range interaction.ApplicationCommandData().Options {
		if opt.Name == name {
			return opt.IntValue()
		}
	}
	return 0
}
