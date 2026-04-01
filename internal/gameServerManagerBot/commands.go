package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/games"

	"github.com/bwmarrin/discordgo"
)

const (
	CommandStartServer   = "startserver"
	CommandStopServer    = "stopserver"
	CommandStatus        = "status"
	CommandTest          = "test"
	CommandStartNewWorld = "startserver-new"
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
			Description: "Run a saved game server",
			Options: []*discordgo.ApplicationCommandOption{
				gameOption,
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "world",
					Description: "World name to load",
					Required:    true,
				},
			},
		},
		{
			Name:        CommandStartNewWorld,
			Description: "Start a new game world",
			Options: []*discordgo.ApplicationCommandOption{
				gameOption,
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "world",
					Description: "Name for the new world (e.g. myAwesomeWorld)",
					Required:    true,
				},
			},
		},
		{
			Name:        CommandStopServer,
			Description: "Save and destroy a running server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "label",
					Description: "Server label to destroy (e.g. CoreKeeper-myworld)",
					Required:    true,
				},
			},
		},
		{
			Name:        CommandStatus,
			Description: "Check server status",
		},
		{
			Name:        CommandTest,
			Description: "Test Command",
		},
	}
}
