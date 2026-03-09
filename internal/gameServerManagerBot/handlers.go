package gameServerManagerBot

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

var Handlers = map[string]func(context.Context, *discordgo.InteractionCreate, *Manager) (*discordgo.InteractionResponse, error){
	CommandStartServer: handlerStartServer,
	CommandStopServer:  handlerStopServer,
	CommandStatus:      handlerStatus,
}

func handlerStartServer(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*discordgo.InteractionResponse, error) {
	go manager.startServer(interaction)
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Server starting, this may take a minute...",
		},
	}, nil
}

func handlerStopServer(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*discordgo.InteractionResponse, error) {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Server stopping...",
		},
	}, nil
}

func handlerStatus(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*discordgo.InteractionResponse, error) {
	status := "some status"
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Server status: " + status,
		},
	}, nil
}
