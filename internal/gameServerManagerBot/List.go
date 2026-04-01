package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/secrets"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

func handlerList(ctx context.Context, interaction *discordgo.InteractionCreate, manager *Manager) (*HandlerResult, error) {
	return &HandlerResult{
		Response: &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		},
		DeferredWork: func() error {
			return manager.listServers(ctx, interaction)
		},
		AcknowledgementResponse: "Fetching server list...",
	}, nil
}

type agentInfoResponse struct {
	Ready    bool   `json:"ready"`
	JoinInfo string `json:"join_info"`
}

func fetchJoinInfo(ip string) agentInfoResponse {
	url := fmt.Sprintf("http://%s:%d/info", ip, agentPort)
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return agentInfoResponse{}
	}
	req.Header.Set("Authorization", "Bearer "+secrets.Secrets.GameServerAgentSecret)
	resp, err := client.Do(req)
	if err != nil {
		return agentInfoResponse{}
	}
	defer resp.Body.Close()

	var info agentInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return agentInfoResponse{}
	}
	return info
}

func (m *Manager) listServers(ctx context.Context, interaction *discordgo.InteractionCreate) error {
	instances, err := m.vultrLayer.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("cannot list instances: %w", err)
	}

	if len(instances) == 0 {
		return sendFollowup(ctx, interaction.Interaction, "No servers currently running.")
	}

	type result struct {
		joinInfo agentInfoResponse
	}
	results := make([]result, len(instances))
	var wg sync.WaitGroup
	for i, inst := range instances {
		wg.Add(1)
		go func(i int, ip string) {
			defer wg.Done()
			results[i] = result{joinInfo: fetchJoinInfo(ip)}
		}(i, inst.MainIP)
	}
	wg.Wait()

	var sb strings.Builder
	fmt.Fprintf(&sb, "**%d server(s) running:**\n", len(instances))
	for i, inst := range instances {
		game := extractGameName(inst.Label)
		world := extractWorldName(inst.Label)
		if results[i].joinInfo.Ready {
			fmt.Fprintf(&sb, "• `%s` world `%s` — join: `%s`\n", game, world, results[i].joinInfo.JoinInfo)
		} else {
			fmt.Fprintf(&sb, "• `%s` world `%s` — ⏳ still starting up\n", game, world)
		}
	}
	return sendFollowup(ctx, interaction.Interaction, sb.String())
}
