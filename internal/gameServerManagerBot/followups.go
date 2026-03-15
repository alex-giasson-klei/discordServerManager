package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/discord"
	"4dmiral/discordServerManager/internal/secrets"
	"bytes"
	"encoding/json"
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

	url := discord.BaseURL + fmt.Sprintf("/webhooks/%s/%s", secrets.Secrets.DiscordAppID, interaction.Token)

	payload := struct {
		Content string `json:"content"`
	}{Content: msg}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal followup payload: %w", err)
	}

	req, err := retryablehttp.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create followup request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := retryablehttp.NewClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send followup: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("followup response status: %d", resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord API returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}
