package gameServerManagerBot

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

func handlerStatus(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*discordgo.InteractionResponse, error) {
	status := "some status"
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Server status: " + status,
		},
	}, nil
}
