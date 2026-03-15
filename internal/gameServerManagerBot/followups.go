package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/discord"
	"4dmiral/discordServerManager/internal/secrets"
	"fmt"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/hashicorp/go-retryablehttp"
)

func sendFollowup(interaction *discordgo.Interaction, msg string) error {
	if interaction == nil {
		return fmt.Errorf("interaction is nil")
	}

	url := discord.BaseURL + fmt.Sprintf("/webhooks/%s/%s/", secrets.Secrets.DiscordAppID, interaction.Token)

	body := msg
	resp, err := retryablehttp.NewRequest(http.MethodPost, url, body)
	log.Printf("err %q, resp %+v", err, resp)
	if err != nil {
		return err
	}
	return nil
}
