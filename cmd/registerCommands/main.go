package main

import (
	"4dmiral/discordServerManager/internal/discord"
	"4dmiral/discordServerManager/internal/gameServerManagerBot"
	"4dmiral/discordServerManager/internal/secrets"
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
)

func main() {
	if err := secrets.GetSecretsWithSDK(context.Background()); err != nil {
		log.Fatalf("Error getting secrets: %s", err)
	}

	discordToken := discord.NewDiscordSessionToken(secrets.Secrets.DiscordToken)
	discordSession, err := discordgo.New(discordToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %s", err)
	}
	defer discordSession.Close()

	for _, guildID := range secrets.Secrets.GuildIDs {
		_, err := discordSession.ApplicationCommandBulkOverwrite(secrets.Secrets.DiscordAppID, guildID, gameServerManagerBot.Commands)
		if err != nil {
			log.Fatalf("Error bulk overwriting commands: %s", err)
		}
	}
}
