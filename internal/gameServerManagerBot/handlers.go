package gameServerManagerBot

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

var Handlers = map[string]func(context.Context, *discordgo.InteractionCreate, *Manager) (*discordgo.InteractionResponse, error){
	CommandStartServer: handlerStartServer,
	CommandStopServer:  handlerStopServer,
	CommandStatus:      handlerStatus,
	CommandTest:        handlerTest,
}

func handlerTest(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*discordgo.InteractionResponse, error) {
	go manager.startTest(interaction)
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Test response",
		},
	}, nil
}

func (m *Manager) startTest(interaction *discordgo.InteractionCreate) {
	time.Sleep(time.Second * 2)
	followupErr := sendFollowup(interaction.Interaction, "Followup test response")
	if followupErr != nil {
		log.Printf("Error sending followup: %s", followupErr)
	}
}
