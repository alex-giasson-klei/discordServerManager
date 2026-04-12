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
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        autoShutdownOptionName,
					Description: "How long before auto-shutdown, e.g. 3h, 30m, 4h30m (min: 10m, max: 5h, default: 3h)",
					Required:    false,
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
	}
}
