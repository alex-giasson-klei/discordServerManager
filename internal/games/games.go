package games

import "fmt"

type GameName string

var startupScriptTemplates map[GameName]string

func init() {
	startupScriptTemplates = make(map[GameName]string)

	startupScriptTemplates[CoreKeeperGameName] = coreKeeperStartupScriptTemplate
}

func StartupScriptTemplate(game GameName) (string, error) {
	template := startupScriptTemplates[game]
	if template == "" {
		return "", fmt.Errorf("no template for game %q", game)
	}

	return template, nil
}
