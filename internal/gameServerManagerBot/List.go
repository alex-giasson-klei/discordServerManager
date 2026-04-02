package gameServerManagerBot

import (
	"4dmiral/discordServerManager/internal/games"
	"4dmiral/discordServerManager/internal/secrets"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	govultr "github.com/vultr/govultr/v3"
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
	// Fetch running instances and saved worlds concurrently.
	type instancesResult struct {
		instances []govultr.Instance
		err       error
	}
	type worldsResult struct {
		worlds map[games.GameName][]string
		err    error
	}

	instCh := make(chan instancesResult, 1)
	worldsCh := make(chan worldsResult, 1)

	go func() {
		inst, err := m.vultrLayer.ListInstances(ctx)
		instCh <- instancesResult{inst, err}
	}()
	go func() {
		w, err := m.ListSavedWorlds(ctx, secrets.Secrets.R2BucketName)
		worldsCh <- worldsResult{w, err}
	}()

	ir := <-instCh
	if ir.err != nil {
		return fmt.Errorf("cannot list instances: %w", ir.err)
	}
	wr := <-worldsCh
	if wr.err != nil {
		return fmt.Errorf("cannot list saved worlds: %w", wr.err)
	}

	// Fetch join info for all running instances concurrently.
	type joinResult struct {
		label    string
		joinInfo agentInfoResponse
	}
	joinResults := make([]joinResult, len(ir.instances))
	var wg sync.WaitGroup
	for i, inst := range ir.instances {
		wg.Add(1)
		go func(i int, label, ip string) {
			defer wg.Done()
			joinResults[i] = joinResult{label: label, joinInfo: fetchJoinInfo(ip)}
		}(i, inst.Label, inst.MainIP)
	}
	wg.Wait()

	// Build a set of running labels for fast lookup.
	running := make(map[string]agentInfoResponse, len(ir.instances))
	for _, r := range joinResults {
		running[r.label] = r.joinInfo
	}

	var sb strings.Builder

	// Running servers first.
	if len(ir.instances) > 0 {
		for _, inst := range ir.instances {
			game := extractGameName(inst.Label)
			world := extractWorldName(inst.Label)
			info := running[inst.Label]
			if info.Ready {
				fmt.Fprintf(&sb, "• `%s` world `%s` — join: `%s`\n", game, world, info.JoinInfo)
			} else {
				fmt.Fprintf(&sb, "• `%s` world `%s` — ⏳ still starting up\n", game, world)
			}
		}
	}

	// Stopped worlds (saved in R2 but no running instance).
	for _, gameName := range games.AllGameNames() {
		for _, worldName := range wr.worlds[gameName] {
			label := fmt.Sprintf("%s-%s", gameName, worldName)
			if _, isRunning := running[label]; !isRunning {
				fmt.Fprintf(&sb, "• `%s` world `%s` — stopped\n", gameName, worldName)
			}
		}
	}

	if sb.Len() == 0 {
		return sendFollowup(ctx, interaction.Interaction, "No servers running and no saved worlds found.")
	}
	return sendFollowup(ctx, interaction.Interaction, sb.String())
}
