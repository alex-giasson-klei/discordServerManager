package gameServerManagerBot

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

func handlerStopServer(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*HandlerResult, error) {
	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Server stopping...",
			},
		},
	}, nil
}
