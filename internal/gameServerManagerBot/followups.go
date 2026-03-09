package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/secrets"
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func sendFollowup(interaction *discordgo.Interaction, msg string) error {
	if interaction == nil {
		return fmt.Errorf("interaction is nil")
	}

	session, err := discordgo.New("Bot " + secrets.Secrets.DiscordToken)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}
	defer session.Close()

	_, err = session.FollowupMessageCreate(
		interaction,
		true,
		&discordgo.WebhookParams{Content: msg},
	)
	if err != nil {
		return fmt.Errorf("send followup message: %w", err)
	}

	return nil
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
