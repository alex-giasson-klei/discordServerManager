package games

const ValheimGameName GameName = "Valheim"

func init() {
	Register(ValheimGameName, GameMeta{
		SaveDirectory: "Valheim",
		ContainerName: "valheim-server",
		SaveDir:       "/opt/valheim/config/worlds_local",
		JoinInfoPath:  "/opt/valheim/config/ready.txt",
	}, valheimStartupScriptTemplate)
}

// valheimStartupScriptTemplate is the cloud-init user-data script for Valheim VPS instances.
// Substitution order: WorldName, AgentBinaryURL, AgentSecret, SaveURL, DiscordWebhookURL
var valheimStartupScriptTemplate = `#!/bin/bash
set -e

# No unattended upgrades
systemctl stop unattended-upgrades
apt remove unattended-upgrades -y

apt-get update -y
apt-get install -y docker.io curl

systemctl enable docker
systemctl start docker

# Disable IPv6 so Valheim binds game ports as IPv4 (0.0.0.0) not IPv6 dual-stack (*).
# Without this, port 2456 binds as an IPv6 socket and the ip6tables DROP policy blocks players.
sysctl -w net.ipv6.conf.all.disable_ipv6=1
sysctl -w net.ipv6.conf.default.disable_ipv6=1

# Open agent port before starting the agent
iptables -I INPUT -p tcp --dport 8080 -j ACCEPT
# Open Valheim game ports
iptables -I INPUT -p udp --dport 2456:2457 -j ACCEPT

mkdir -p /opt/valheim/config/worlds_local
mkdir -p /opt/valheim/data

WORLD_NAME="%s"
PUBLIC_IP=$(curl -s ifconfig.me)

# Download and start the gameserver management agent
curl -fSL "%s" -o /usr/local/bin/gameserver-agent
chmod +x /usr/local/bin/gameserver-agent
AGENT_SECRET="%s" WORLD_NAME="$WORLD_NAME" CONTAINER_NAME="valheim-server" SAVE_DIR="/opt/valheim/config/worlds_local" JOIN_INFO_PATH="/opt/valheim/config/ready.txt" nohup /usr/local/bin/gameserver-agent > /var/log/gameserver-agent.log 2>&1 &

SAVE_URL="%s"
if [ -n "$SAVE_URL" ]; then
    curl -fSL "$SAVE_URL" -o /tmp/save.tar.gz
    tar -xzf /tmp/save.tar.gz -C /opt/valheim/config/worlds_local
    rm /tmp/save.tar.gz
fi

docker run -d \
    --name valheim-server \
    --restart unless-stopped \
    --cap-add=sys_nice \
    --stop-timeout 120 \
    --network host \
    -v /opt/valheim/config:/config \
    -v /opt/valheim/data:/opt/valheim \
    -e SERVER_NAME="$WORLD_NAME" \
    -e WORLD_NAME="$WORLD_NAME" \
    -e SERVER_PASS="valheim" \
    -e SERVER_PUBLIC=true \
    -e BACKUPS=false \
    -e UPDATE_CRON="" \
    -e RESTART_CRON="" \
    -e "POST_SERVER_LISTENING_HOOK=echo '${PUBLIC_IP}:2456 | password: valheim' > /config/ready.txt && curl -sf -X POST -H 'Content-Type: application/json' -d '{\"username\":\"Valheim Game\",\"content\":Join info: \"${PUBLIC_IP}:2456 | password: valheim\"}' \"\$DISCORD_WEBHOOK\"" \
    -e DISCORD_WEBHOOK="%s" \
    ghcr.io/community-valheim-tools/valheim-server
`
