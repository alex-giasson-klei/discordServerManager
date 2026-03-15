package gameServerManagerBot

import "github.com/bwmarrin/discordgo"

const (
	CommandStartServer   = "startserver"
	CommandStopServer    = "stopserver"
	CommandStatus        = "status"
	CommandTest          = "test"
	CommandStartNewWorld = "startserver-new"
)

var Commands = []*discordgo.ApplicationCommand{
	{
		Name:        CommandStartServer,
		Description: "Run a saved Core Keeper server",
		Options: []*discordgo.ApplicationCommandOption{
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
		Description: "Start a new Core Keeper world",
		Options: []*discordgo.ApplicationCommandOption{
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
		Description: "Destroy a Core Keeper server",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "label",
				Description: "Server label to destroy (e.g. corekeeper-myworld)",
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
