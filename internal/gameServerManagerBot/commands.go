package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/games"

	"github.com/bwmarrin/discordgo"
)

const (
	CommandStartServer = "startserver"
	CommandStopServer  = "stopserver"
	CommandList        = "list"
	CommandTest        = "test"
)

var Commands []*discordgo.ApplicationCommand

func init() {
	gameChoices := make([]*discordgo.ApplicationCommandOptionChoice, 0)
	for _, name := range games.AllGameNames() {
		gameChoices = append(gameChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  string(name),
			Value: string(name),
		})
	}

	gameOption := &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        "game",
		Description: "Which game to run",
		Required:    true,
		Choices:     gameChoices,
	}

	Commands = []*discordgo.ApplicationCommand{
		{
			Name:        CommandStartServer,
			Description: "Start a game server (loads existing save, or use new:True for a fresh world)",
			Options: []*discordgo.ApplicationCommandOption{
				gameOption,
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "world",
					Description: "World name",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "new",
					Description: "Create a fresh world instead of loading a save",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "autoshutdown",
					Description: "Auto-shutdown timer in minutes (10-300, default: 300)",
					Required:    false,
					MinValue:    func() *float64 { v := float64(10); return &v }(),
					MaxValue:    300,
				},
			},
		},
		{
			Name:        CommandStopServer,
			Description: "Save and destroy a running server",
			Options: []*discordgo.ApplicationCommandOption{
				gameOption,
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "world",
					Description: "World name",
					Required:    true,
				},
			},
		},
		{
			Name:        CommandList,
			Description: "List running game servers and saved worlds",
		},
		{
			Name:        CommandTest,
			Description: "Test Command",
		},
	}
}
