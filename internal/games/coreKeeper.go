package games

const CoreKeeperGameName GameName = "CoreKeeper"

func init() {
	Register(CoreKeeperGameName, GameMeta{
		SaveDirectory: "CoreKeeper",
		ContainerName: "core-keeper-dedicated",
		SaveDir:       "/tmp/core-keeper-data",
		JoinInfoPath:  "/tmp/core-keeper-dedicated/GameID.txt",
	}, coreKeeperStartupScriptTemplate)
}

// coreKeeperStartupScriptTemplate is the cloud-init user-data script for Core Keeper VPS instances.
// Substitution order: WorldName, AgentBinaryURL, AgentSecret, SaveURL, DiscordWebhookURL
var coreKeeperStartupScriptTemplate = `#!/bin/bash
set -e

# No unattended upgrades
systemctl stop unattended-upgrades
apt remove unattended-upgrades -y

apt-get update -y
apt-get install -y docker.io curl

systemctl enable docker
systemctl start docker

# Open agent port before starting the agent
iptables -I INPUT -p tcp --dport 8080 -j ACCEPT

mkdir -p /tmp/core-keeper-data
mkdir -p /tmp/core-keeper-dedicated

WORLD_NAME="%s"

# Download and start the gameserver management agent
curl -fSL "%s" -o /usr/local/bin/gameserver-agent
chmod +x /usr/local/bin/gameserver-agent
AGENT_SECRET="%s" WORLD_NAME="$WORLD_NAME" CONTAINER_NAME="core-keeper-dedicated" SAVE_DIR="/tmp/core-keeper-data" JOIN_INFO_PATH="/tmp/core-keeper-dedicated/GameID.txt" nohup /usr/local/bin/gameserver-agent > /var/log/gameserver-agent.log 2>&1 &

SAVE_URL="%s"
if [ -n "$SAVE_URL" ]; then
    curl -fSL "$SAVE_URL" -o /tmp/save.tar.gz
    tar -xzf /tmp/save.tar.gz -C /tmp/core-keeper-data
    rm /tmp/save.tar.gz
fi

docker pull escaping/core-keeper-dedicated:v2.8.1

docker run -d \
    --name core-keeper-dedicated \
    --restart unless-stopped \
    -e WORLD_NAME="$WORLD_NAME" \
    -e MAX_PLAYERS=5 \
    -e DISCORD_WEBHOOK_URL="%s" \
    -e DISCORD_PLAYER_LEAVE_ENABLED=false \
    -v /tmp/core-keeper-data:/home/steam/core-keeper-data \
    -v /tmp/core-keeper-dedicated:/home/steam/core-keeper-dedicated \
    escaping/core-keeper-dedicated:v2.8.1
`
