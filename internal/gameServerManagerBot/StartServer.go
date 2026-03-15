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
	}, nil
}

func (m *Manager) startServer(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	log.Printf("getting server instance")
	if err := sendFollowup(ctx, interaction.Interaction, "Getting server instance..."); err != nil {
		log.Printf("Error sending followup: %s", err)
	}

	log.Printf("Getting instance by label %s", vultrlayer.SingleServerLabel)
	instance, err := m.vultrLayer.GetSingleServerInstanceByLabel(ctx, vultrlayer.SingleServerLabel)
	if err != nil {
		log.Printf("Error getting vultr instance: %s", err)
		if followupErr := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Error getting instance by label: %s", err)); followupErr != nil {
			log.Printf("Error sending followup: %s", followupErr)
		}
		return err
	}

	if err := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Starting server %s %s", instance.ID, instance.Label)); err != nil {
		log.Printf("Error sending followup: %s", err)
	}
	log.Printf("Starting server %s %s", instance.ID, instance.Label)

	if err := m.vultrLayer.StartInstance(ctx, instance.ID); err != nil {
		log.Printf("Error starting server: %s", err)
		if followupErr := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Error starting instance: %s", err)); followupErr != nil {
			log.Printf("Error sending followup: %s", followupErr)
		}
		return err
	}

	if err := sendFollowup(ctx, interaction.Interaction, "Server started"); err != nil {
		log.Printf("Error sending followup: %s", err)
	}
	return nil
}
