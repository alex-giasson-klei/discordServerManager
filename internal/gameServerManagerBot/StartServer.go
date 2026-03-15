package gameServerManagerBot

import (
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func handlerStartServer(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*HandlerResult, error) {
	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		},
		DeferredWork: func() error {
			return manager.startServer(ctx, interaction)
		},
		AcknowledgementResponse: "Starting server, this may take a few minutes...",
	}, nil
}

func (m *Manager) startServer(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	log.Printf("Getting instance by label %s", vultrlayer.SingleServerLabel)
	instance, err := m.vultrLayer.GetSingleServerInstanceByLabel(ctx, vultrlayer.SingleServerLabel)
	if err != nil {
		return fmt.Errorf("cannot get server %q: %w", vultrlayer.SingleServerLabel, err)
	}

	if err := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Starting server %s %s", instance.ID, instance.Label)); err != nil {
		log.Printf("Error sending followup: %s", err)
		return fmt.Errorf("cannot send followup message: %w", err)
	}

	if err := m.vultrLayer.StartInstance(ctx, instance.ID); err != nil {
		return fmt.Errorf("cannot start instance %s %s: %w", instance.ID, instance.Label, err)
	}

	if err := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Server started %s", instance.Label)); err != nil {
		log.Printf("Error sending followup: %s", err)
		return fmt.Errorf("cannot send followup message: %w", err)
	}
	
	return nil
}
