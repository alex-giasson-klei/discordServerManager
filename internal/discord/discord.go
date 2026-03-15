package discord

import (
	"crypto/ed25519"
	"net/http"

	"github.com/bwmarrin/discordgo"
)

const BaseURL = "https://discord.com/api/v10"

func NewDiscordSessionToken(token string) string {
	return "Bot " + token
}

func VerifyRequest(r *http.Request, key ed25519.PublicKey) bool {
	return discordgo.VerifyInteraction(r, key)
}

func ResponsePong(interaction discordgo.Interaction) discordgo.InteractionResponse {
	resp := discordgo.InteractionResponse{
		Type: discordgo.InteractionResponsePong,
	}
	return resp
}
