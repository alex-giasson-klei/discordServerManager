package gameServerManagerBot

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

// HandlerResult holds the immediate interaction response and optional deferred work.
// If DeferredWork is set, handleInteraction will ACK Discord immediately via the
// callback API, run DeferredWork synchronously, then return the HTTP response.
type HandlerResult struct {
	Response                *discordgo.InteractionResponse
	DeferredWork            func() error
	AcknowledgementResponse string
}

var Handlers = map[string]func(context.Context, *discordgo.InteractionCreate, *Manager) (*HandlerResult, error){
	CommandStartServer: handlerStartServer,
	CommandStopServer:  handlerStopServer,
	CommandStatus:      handlerStatus,
	CommandTest:        handlerTest,
}

func handlerTest(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*HandlerResult, error) {
	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			//Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "test",
			},
		},
		//DeferredWork: func() error {
		//	return manager.startTest(ctx, interaction)
		//},
		AcknowledgementResponse: "Test command received, this may take a few minutes...",
	}, nil
}

func (m *Manager) startTest(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	time.Sleep(time.Second * 2)
	if err := sendFollowup(ctx, interaction.Interaction, "Followup test response"); err != nil {
		log.Printf("Error sending followup: %s", err)
		return err
	}
	return nil
}
