package games

import (
	"fmt"
	"sort"
)

type GameName string

// GameMeta holds the game-specific values the bot and agent need at runtime.
type GameMeta struct {
	SaveDirectory string // S3 key prefix for saves, e.g. "CoreKeeper"
	ContainerName string // Docker container name on the VPS
	SaveDir       string // Path on the VPS that the agent archives
	JoinInfoPath  string // Path on the VPS where the game writes join info (e.g. a Game ID file)
}

var (
	gameMetas              = map[GameName]GameMeta{}
	startupScriptTemplates = map[GameName]string{}
)

// Register is called from each game's init() to make it available.
func Register(name GameName, meta GameMeta, template string) {
	gameMetas[name] = meta
	startupScriptTemplates[name] = template
}

// Meta returns the GameMeta for the given game, or an error if unknown.
func Meta(game GameName) (GameMeta, error) {
	meta, ok := gameMetas[game]
	if !ok {
		return GameMeta{}, fmt.Errorf("unknown game %q", game)
	}
	return meta, nil
}

// StartupScriptTemplate returns the cloud-init script template for the given game.
func StartupScriptTemplate(game GameName) (string, error) {
	t, ok := startupScriptTemplates[game]
	if !ok {
		return "", fmt.Errorf("no startup template for game %q", game)
	}
	return t, nil
}

// AllGameNames returns all registered game names in alphabetical order.
func AllGameNames() []GameName {
	names := make([]GameName, 0, len(gameMetas))
	for k := range gameMetas {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}
