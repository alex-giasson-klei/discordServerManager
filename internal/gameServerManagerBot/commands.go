package gameServerManagerBot

import "github.com/bwmarrin/discordgo"

const (
	CommandStartServer = "startserver"
	CommandStopServer  = "stopserver"
	CommandStatus      = "status"
)

var Commands = []*discordgo.ApplicationCommand{
	{
		Name:        string(CommandStartServer),
		Description: "Start a Core Keeper server",
	},
	{
		Name:        string(CommandStopServer),
		Description: "Stop a Core Keeper server",
	},
	{
		Name:        string(CommandStatus),
		Description: "Check server status",
	},
}
