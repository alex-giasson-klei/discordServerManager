package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/games"
	"4dmiral/discordServerManager/internal/secrets"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/scheduler"
	schedulertypes "github.com/aws/aws-sdk-go-v2/service/scheduler/types"
)

const (
	AutoShutdownDuration  = 5 * time.Hour
	autoShutdownGroupName = "gameServerAutoShutdown"
)

// AutoShutdownEvent is the payload EventBridge Scheduler sends to the Lambda
// when the auto-shutdown timer fires.
type AutoShutdownEvent struct {
	Label        string `json:"label"`
	GuildID      string `json:"guild_id"`
	ScheduleName string `json:"schedule_name"`
}

// formatDuration formats a duration as "Xh Ym"
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

func autoShutdownScheduleName(label string) string {
	return "autoshutdown-" + label
}

// CreateAutoShutdownSchedule registers a one-time EventBridge Scheduler rule
// that will invoke this Lambda with an AutoShutdownEvent after autoShutdownDuration.
func (m *Manager) CreateAutoShutdownSchedule(ctx context.Context, label, guildID string) error {
	scheduleName := autoShutdownScheduleName(label)

	event := AutoShutdownEvent{
		Label:        label,
		GuildID:      guildID,
		ScheduleName: scheduleName,
	}
	inputJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal auto-shutdown event: %w", err)
	}

	fireAt := time.Now().UTC().Add(AutoShutdownDuration)
	expression := fmt.Sprintf("at(%s)", fireAt.Format("2006-01-02T15:04:05"))

	_, err = m.schedulerClient.CreateSchedule(ctx, &scheduler.CreateScheduleInput{
		Name:               aws.String(scheduleName),
		ScheduleExpression: aws.String(expression),
		FlexibleTimeWindow: &schedulertypes.FlexibleTimeWindow{
			Mode: schedulertypes.FlexibleTimeWindowModeOff,
		},
		Target: &schedulertypes.Target{
			Arn:     aws.String(secrets.Secrets.LambdaFunctionARN),
			RoleArn: aws.String(secrets.Secrets.SchedulerRoleARN),
			Input:   aws.String(string(inputJSON)),
		},
		ActionAfterCompletion: schedulertypes.ActionAfterCompletionDelete,
		GroupName:             aws.String(autoShutdownGroupName),
	})
	if err != nil {
		return fmt.Errorf("create schedule %q: %w", scheduleName, err)
	}
	return nil
}

// DeleteAutoShutdownSchedule removes the pending auto-shutdown schedule for a
// server. Returns an error if the delete fails for any reason other than the
// schedule already being gone (fired and auto-deleted).
func (m *Manager) DeleteAutoShutdownSchedule(ctx context.Context, label string) error {
	scheduleName := autoShutdownScheduleName(label)
	_, err := m.schedulerClient.DeleteSchedule(ctx, &scheduler.DeleteScheduleInput{
		Name:      aws.String(scheduleName),
		GroupName: aws.String(autoShutdownGroupName),
	})
	if err != nil {
		if _, ok := errors.AsType[*schedulertypes.ResourceNotFoundException](err); ok {
			// Already fired and auto-deleted — not an error.
			return nil
		}
		return fmt.Errorf("delete auto-shutdown schedule %q: %w", scheduleName, err)
	}
	return nil
}

// HandleAutoShutdown runs the full save+destroy flow triggered by EventBridge.
// Discord notifications go via the guild webhook (not an interaction token).
func (m *Manager) HandleAutoShutdown(ctx context.Context, event AutoShutdownEvent) error {
	log.Printf("auto-shutdown triggered for %q (guild %s)", event.Label, event.GuildID)

	webhookURL := secrets.Secrets.GuildWebhooks[event.GuildID]
	notify := func(msg string) {
		if webhookURL == "" {
			return
		}
		if err := postToWebhook(webhookURL, msg); err != nil {
			log.Printf("failed to notify Discord webhook: %v", err)
		}
	}

	notify(fmt.Sprintf("⏰ Server `%s` has reached its 5-hour limit and is shutting down automatically...", event.Label))

	instance, err := m.vultrLayer.GetInstanceByLabel(ctx, event.Label)
	if err != nil {
		notify(fmt.Sprintf("❌ Auto-shutdown failed: could not find instance `%s`: %v", event.Label, err))
		return fmt.Errorf("auto-shutdown find instance: %w", err)
	}

	worldName := extractWorldName(event.Label)
	gameName := games.GameName(extractGameName(event.Label))
	meta, err := games.Meta(gameName)
	if err != nil {
		notify(fmt.Sprintf("❌ Auto-shutdown failed: unrecognised game in label `%s`: %v", event.Label, err))
		return fmt.Errorf("auto-shutdown unknown game: %w", err)
	}
	s3Key := meta.SaveKey(worldName)
	uploadURL, err := m.GeneratePresignedPutURL(ctx, secrets.Secrets.R2BucketName, s3Key, saveURLExpiry)
	if err != nil {
		notify(fmt.Sprintf("❌ Auto-shutdown failed: could not generate upload URL: %v", err))
		return fmt.Errorf("auto-shutdown generate upload URL: %w", err)
	}

	if err := m.checkAgentReady(instance.MainIP); err != nil {
		log.Printf("auto-shutdown: agent not ready for %q, force-destroying without save: %v", event.Label, err)
		notify(fmt.Sprintf("⚠️ Server `%s` agent was unreachable at shutdown time — destroying without saving.", event.Label))
		if err := m.vultrLayer.DestroyInstance(ctx, instance.ID); err != nil {
			notify(fmt.Sprintf("❌ Auto-shutdown: failed to destroy instance: %v", err))
			return fmt.Errorf("auto-shutdown destroy (no-save path): %w", err)
		}
		notify(fmt.Sprintf("🗑️ Server `%s` destroyed (no save — agent was unreachable).", event.Label))
		return nil
	}

	if err := m.RotateSave(ctx, secrets.Secrets.R2BucketName, s3Key); err != nil {
		notify(fmt.Sprintf("❌ Auto-shutdown: cannot back up existing save — shutdown cancelled for safety: %v", err))
		return fmt.Errorf("auto-shutdown rotate save: %w", err)
	}

	if err := m.callAgentShutdown(ctx, instance.MainIP, uploadURL); err != nil {
		notify(fmt.Sprintf("❌ Auto-shutdown: agent shutdown failed — instance NOT destroyed: %v", err))
		return fmt.Errorf("auto-shutdown agent: %w", err)
	}

	if err := m.vultrLayer.DestroyInstance(ctx, instance.ID); err != nil {
		notify(fmt.Sprintf("❌ Auto-shutdown: failed to destroy instance: %v", err))
		return fmt.Errorf("auto-shutdown destroy: %w", err)
	}

	notify(fmt.Sprintf("✅ Server `%s` auto-shutdown complete — world saved.", event.Label))
	return nil
}

func postToWebhook(webhookURL, content string) error {
	payload := struct {
		Content string `json:"content"`
	}{Content: content}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
