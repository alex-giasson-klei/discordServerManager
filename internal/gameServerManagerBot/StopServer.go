package gameServerManagerBot

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func handlerStopServer(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*HandlerResult, error) {
	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		},
		DeferredWork: func() error {
			return manager.stopServer(ctx, interaction)
		},
		AcknowledgementResponse: "Destroying server, please wait...",
	}, nil
}

func (m *Manager) stopServer(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	label := optionString(interaction, "label")
	if label == "" {
		return fmt.Errorf("missing required option: label")
	}

	instance, err := m.vultrLayer.GetInstanceByLabel(ctx, label)
	if err != nil {
		return fmt.Errorf("cannot find server with label %q: %w", label, err)
	}

	if err := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Destroying instance `%s` (ID: `%s`)...", instance.Label, instance.ID)); err != nil {
		log.Printf("Error sending followup: %s", err)
	}

	if err := m.vultrLayer.DestroyInstance(ctx, instance.ID); err != nil {
		return fmt.Errorf("cannot destroy instance %q: %w", label, err)
	}

	return sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Server `%s` has been destroyed.", label))
}
