package gameServerManagerBot

import (
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
	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		},
		DeferredWork: func() error {
			return manager.stopServer(ctx, interaction)
		},
		AcknowledgementResponse: "Saving world and destroying server, please wait...",
	}, nil
}

func (m *Manager) stopServer(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	label := optionString(interaction, "label")
	if label == "" {
		return fmt.Errorf("missing required option: label")
	}

	instance, err := m.vultrLayer.GetInstanceByLabel(ctx, label)
	if err != nil {
		return fmt.Errorf("cannot find server with label %q: %w", label, err)
	}
	if instance.MainIP == "" {
		return fmt.Errorf("instance %q has no IP yet — wait for it to finish provisioning", label)
	}

	worldName := extractWorldName(label)
	s3Key := fmt.Sprintf("%s/%s.tar.gz", gameNameDir, worldName)
	uploadURL, err := m.GeneratePresignedPutURL(ctx, secrets.Secrets.R2BucketName, s3Key, saveURLExpiry)
	if err != nil {
		return fmt.Errorf("cannot generate save upload URL: %w", err)
	}

	if err := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Saving world `%s`...", worldName)); err != nil {
		log.Printf("Error sending followup: %s", err)
	}

	if err := m.callAgentShutdown(ctx, instance.MainIP, uploadURL); err != nil {
		return fmt.Errorf("agent shutdown failed for %q (instance NOT destroyed — save may not have uploaded): %w", label, err)
	}

	if err := sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("World `%s` saved to S3, destroying instance...", worldName)); err != nil {
		log.Printf("Error sending followup: %s", err)
	}

	if err := m.vultrLayer.DestroyInstance(ctx, instance.ID); err != nil {
		return fmt.Errorf("cannot destroy instance %q: %w", label, err)
	}

	return sendFollowup(ctx, interaction.Interaction, fmt.Sprintf("Server `%s` saved and destroyed.", label))
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

// extractWorldName strips the leading game prefix from a VPS label.
// e.g. "corekeeper-myworld" → "myworld", "CoreKeeper-myworld" → "myworld"
func extractWorldName(label string) string {
	if idx := strings.Index(label, "-"); idx != -1 {
		return label[idx+1:]
	}
	return label
}
