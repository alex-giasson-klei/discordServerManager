package gameServerManagerBot

import (
	vultrlayer "4dmiral/discordServerManager/internal/vultr"
	"context"

	"github.com/bwmarrin/discordgo"
)

var Handlers = map[string]func(context.Context, *discordgo.InteractionCreate, *Manager) (*discordgo.InteractionResponse, error){

	CommandStartServer: func(ctx context.Context, i *discordgo.InteractionCreate, manager *Manager) (*discordgo.InteractionResponse, error) {
		instanceID, err := manager.vultrLayer.GetSingleServerInstanceByLabel(ctx, vultrlayer.SingleServerLabel)
		if err != nil {
			return &discordgo.InteractionResponse{}, err
		}
		err = manager.vultrLayer.StartInstance(ctx, instanceID)
		if err != nil {
			return &discordgo.InteractionResponse{}, err
		}

		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Server starting",
			},
		}, nil
	},

	CommandStopServer: func(ctx context.Context, i *discordgo.InteractionCreate, manager *Manager) (*discordgo.InteractionResponse, error) {

		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Server stopping...",
			},
		}, nil
	},

	CommandStatus: func(ctx context.Context, i *discordgo.InteractionCreate, manager *Manager) (*discordgo.InteractionResponse, error) {
		status := "some status"
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Server status: " + status,
			},
		}, nil
	},
}
