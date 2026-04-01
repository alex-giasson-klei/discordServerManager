package gameServerManagerBot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func handlerStatus(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*HandlerResult, error) {
	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		},
		DeferredWork: func() error {
			return manager.statusReport(ctx, interaction)
		},
		AcknowledgementResponse: "Checking server status...",
	}, nil
}

func (m *Manager) statusReport(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	instances, err := m.vultrLayer.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("cannot list instances: %w", err)
	}

	if len(instances) == 0 {
		return sendFollowup(ctx, interaction.Interaction, "No servers currently running.")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "**%d server(s) running:**\n", len(instances))
	for _, inst := range instances {
		fmt.Fprintf(&sb, "• `%s` — `%s` | status: `%s`\n", inst.Label, inst.MainIP, inst.Status)
	}
	return sendFollowup(ctx, interaction.Interaction, sb.String())
}
