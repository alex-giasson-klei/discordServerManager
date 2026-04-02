package gameServerManagerBot

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// HandleDeferredCommand is called by the async self-invocation. It re-runs the
// handler to recreate DeferredWork, executes it, and sends followups via webhook.
func (m *Manager) HandleDeferredCommand(ctx context.Context, interaction discordgo.Interaction) error {
	data := interaction.ApplicationCommandData()
	handler, ok := Handlers[data.Name]
	if !ok {
		return fmt.Errorf("unknown command in deferred payload: %q", data.Name)
	}

	result, err := handler(ctx, &discordgo.InteractionCreate{Interaction: &interaction}, m)
	if err != nil {
		log.Printf("deferred command handler error: %v", err)
		if followupErr := sendFollowup(ctx, &interaction, fmt.Sprintf("❌ Error: %s", err)); followupErr != nil {
			log.Printf("failed to send error followup: %v", followupErr)
		}
		return err
	}

	if result.DeferredWork == nil {
		return nil
	}

	if workErr := result.DeferredWork(); workErr != nil {
		log.Printf("deferred work error: %v", workErr)
		if followupErr := sendFollowup(ctx, &interaction, fmt.Sprintf("❌ Error: %s", workErr)); followupErr != nil {
			log.Printf("failed to send error followup: %v", followupErr)
		}
		return workErr
	}
	return nil
}
