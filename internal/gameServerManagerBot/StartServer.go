package gameServerManagerBot

import (
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func handlerStartServer(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*discordgo.InteractionResponse, error) {
	go manager.startServer(interaction)
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Server starting, this may take a minute...",
		},
	}, nil
}

func (m *Manager) startServer(interaction *discordgo.InteractionCreate) {
	log.Printf("getting server instance")
	ctx := context.Background()
	followupErr := sendFollowup(interaction.Interaction, "Getting server instance...")
	if followupErr != nil {
		log.Printf("Error sending followup: %s", followupErr)
	}
	log.Printf("Getting instance by label %s", vultrlayer.SingleServerLabel)
	instance, err := m.vultrLayer.GetSingleServerInstanceByLabel(ctx, vultrlayer.SingleServerLabel)
	if err != nil {
		log.Printf("Error getting vultr instance: %s", err)
		if followupErr := sendFollowup(interaction.Interaction, fmt.Sprintf("Error getting instance by label: %s", err)); followupErr != nil {
			log.Printf("Error sending followup: %s", followupErr)
		}
		return
	}

	if followupErr := sendFollowup(interaction.Interaction, fmt.Sprintf("Starting server %s %s", instance.ID, instance.Label)); followupErr != nil {
		log.Printf("Error sending followup: %s", followupErr)
	}
	log.Printf("Starting server %s %s", instance.ID, instance.Label)
	err = m.vultrLayer.StartInstance(ctx, instance.ID)
	if err != nil {
		log.Printf("Error starting server: %s", err)
		if followupErr := sendFollowup(interaction.Interaction, fmt.Sprintf("Error starting instance: %s", err)); followupErr != nil {
			log.Printf("Error sending followup: %s", followupErr)
		}
		return
	}

	if followupErr := sendFollowup(interaction.Interaction, "Server started"); followupErr != nil {
		log.Printf("Error sending followup: %s", followupErr)
	}
}
