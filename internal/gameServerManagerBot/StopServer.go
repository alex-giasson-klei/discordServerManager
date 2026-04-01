package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/games"
	"4dmiral/discordServerManager/internal/secrets"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const agentShutdownTimeout = 10 * time.Minute

type agentShutdownPayload struct {
	UploadURL string `json:"upload_url"`
}

func handlerStopServer(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*HandlerResult, error) {
	game := optionString(interaction, "game")
	world := optionString(interaction, "world")
	ack := fmt.Sprintf("Saving `%s` world `%s` and destroying server...", game, world)

	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		},
		DeferredWork: func() error {
			return manager.stopServer(ctx, interaction)
		},
		AcknowledgementResponse: ack,
	}, nil
}

func (m *Manager) stopServer(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	gameName := games.GameName(optionString(interaction, "game"))
	if gameName == "" {
		return fmt.Errorf("missing required option: game")
	}
	worldName := optionString(interaction, "world")
	if worldName == "" {
		return fmt.Errorf("missing required option: world")
	}

	meta, err := games.Meta(gameName)
	if err != nil {
		return fmt.Errorf("unrecognised game %q: %w", gameName, err)
	}

	label := fmt.Sprintf("%s-%s", gameName, worldName)

	instance, err := m.vultrLayer.GetInstanceByLabel(ctx, label)
	if err != nil {
		return fmt.Errorf("cannot find %q world %q. Use `/list`: %w", gameName, worldName, err)
	}
	if instance.MainIP == "" {
		return fmt.Errorf("instance %q has no IP yet — wait for it to finish provisioning", label)
	}

	s3Key := meta.SaveKey(worldName)
	uploadURL, err := m.GeneratePresignedPutURL(ctx, secrets.Secrets.R2BucketName, s3Key, saveURLExpiry)
	if err != nil {
		return fmt.Errorf("cannot generate save upload URL: %w", err)
	}

	if err := m.checkAgentReady(instance.MainIP); err != nil {
		return fmt.Errorf("server `%s` is still initializing — wait a few minutes and try again", label)
	}

	if err := m.RotateSave(ctx, secrets.Secrets.R2BucketName, s3Key); err != nil {
		return fmt.Errorf("cannot back up existing save for %q (shutdown cancelled for safety): %w", label, err)
	}

	if err := m.callAgentShutdown(ctx, instance.MainIP, uploadURL); err != nil {
		return fmt.Errorf("agent shutdown failed for %q (instance NOT destroyed — save may not have uploaded): %w", label, err)
	}

	if err := m.vultrLayer.DestroyInstance(ctx, instance.ID); err != nil {
		return fmt.Errorf("cannot destroy instance %q: %w", label, err)
	}

	if err := m.DeleteAutoShutdownSchedule(ctx, label); err != nil {
		log.Printf("warning: failed to delete auto-shutdown schedule for %q: %v", label, err)
	}

	return sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("`%s` world `%s` saved and destroyed.", gameName, worldName))
}

func (m *Manager) checkAgentReady(ip string) error {
	url := fmt.Sprintf("http://%s:%d/health", ip, agentPort)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("agent not reachable: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent health check returned %d", resp.StatusCode)
	}
	return nil
}

func (m *Manager) callAgentShutdown(ctx context.Context, ip, uploadURL string) error {
	url := fmt.Sprintf("http://%s:%d/shutdown", ip, agentPort)

	payload := agentShutdownPayload{UploadURL: uploadURL}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal shutdown payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create shutdown request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+secrets.Secrets.GameServerAgentSecret)

	client := &http.Client{Timeout: agentShutdownTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call agent shutdown: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

// extractGameName returns the game name portion of a VPS label.
// e.g. "CoreKeeper-myworld" → "CoreKeeper"
func extractGameName(label string) string {
	if idx := strings.Index(label, "-"); idx != -1 {
		return label[:idx]
	}
	return label
}

// extractWorldName strips the leading game prefix from a VPS label.
// e.g. "CoreKeeper-myworld" → "myworld", "CoreKeeper-my-world" → "my-world"
func extractWorldName(label string) string {
	if idx := strings.Index(label, "-"); idx != -1 {
		return label[idx+1:]
	}
	return label
}
