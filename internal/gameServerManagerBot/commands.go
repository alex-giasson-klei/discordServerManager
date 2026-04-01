package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/games"

	"github.com/bwmarrin/discordgo"
)

const (
	CommandStartServer = "startserver"
	CommandStopServer  = "stopserver"
	CommandStatus      = "list"
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
					Description: "Create a fresh world instead of loading a save (default: false)",
					Required:    true,
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
			Name:        CommandStatus,
			Description: "List running game servers",
		},
		{
			Name:        CommandTest,
			Description: "Test Command",
		},
	}
}
