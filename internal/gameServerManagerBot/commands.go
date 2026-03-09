package gameServerManagerBot

import "github.com/bwmarrin/discordgo"

const (
	CommandStartServer = "startserver"
	CommandStopServer  = "stopserver"
	CommandStatus      = "status"
)

var Commands = []*discordgo.ApplicationCommand{
	{
		Name:        CommandStartServer,
		Description: "Start a Core Keeper server",
	},
	{
		Name:        CommandStopServer,
		Description: "Stop a Core Keeper server",
	},
	{
		Name:        CommandStatus,
		Description: "Check server status",
	},
}
