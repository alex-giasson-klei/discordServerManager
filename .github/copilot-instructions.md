# Discord Game Server Manager — Copilot Instructions

## What this project does
A Discord slash-command bot (AWS Lambda + Vultr VPS) that spins game servers up and down on demand.
Players never pay for compute when not playing. Save files persist in AWS S3 between sessions.
Currently supports **Core Keeper** only.

## Architecture

```
Discord slash command
  → AWS Lambda (Function URL, Go binary)
    → Vultr API: create/destroy VPS instances
    → AWS S3: store/load world save files (CoreKeeper/<worldName>.tar.gz)
    → AWS Secrets Manager: all credentials
    → Discord webhooks: async followup messages

VPS (Vultr, Ubuntu 24.04, created fresh each time)
  → cloud-init startup script (embedded in Lambda response)
    → downloads save from S3 pre-signed URL (if existing world)
    → starts gameserver-agent HTTP binary (port 8080) ← manages graceful shutdown/save
    → starts Core Keeper dedicated server in Docker
```

## Key directories and packages

| Path | Purpose |
|------|---------|
| `lambda/` | Lambda entry point — loads secrets, wires dependencies, calls `HandleRequest` |
| `internal/gameServerManagerBot/` | All command handlers and bot logic |
| `internal/vultr/` | Thin wrapper around `govultr/v3` for instance lifecycle |
| `internal/secrets/` | Loads `LambdaSecrets` from AWS Secrets Manager at cold-start |
| `internal/discord/` | Request verification, pong response, session token helper |
| `registerCommands/` | One-shot CLI tool to bulk-register slash commands with Discord |
| `cmd/gameserver/` | HTTP agent binary deployed to each game VPS — handles graceful shutdown and save upload |

## Slash commands

| Command | Handler file | What it does |
|---------|-------------|-------------|
| `/startserver world:<name>` | `StartServer.go` | Loads existing save from S3, provisions VPS |
| `/startserver-new world:<name>` | `StartServerNew.go` | Creates a fresh world, provisions VPS |
| `/stopserver label:<label>` | `StopServer.go` | Signals agent to save → uploads to S3 → destroys VPS |
| `/status` | `Status.go` | Lists running instances (stub, needs implementation) |
| `/test` | `handlers.go` | Dev test command |

## Interaction / response pattern
Discord requires a response within 3 seconds. Handlers use a **deferred work** pattern:
1. Return `HandlerResult` with `DeferredWork` set.
2. `handleInteraction` immediately POSTs an ACK to Discord's callback URL.
3. `DeferredWork()` runs synchronously in the Lambda (up to ~15 min).
4. Progress/errors are sent via `sendFollowup`.

## Storage: Cloudflare R2
Save files and the agent binary are stored in **Cloudflare R2** (S3-compatible, ~free at this scale).
The AWS SDK S3 client is used with a custom endpoint — no Cloudflare-specific SDK needed.

R2 layout:
```
r2://<R2BucketName>/
  CoreKeeper/<worldName>.tar.gz   ← tarball of /tmp/core-keeper-data from the VPS
  bin/gameserver-agent            ← Linux amd64 binary, uploaded by `make uploadGameserverAgent`
```

## VPS startup script (`startupScriptTemplate` in `StartServer.go`)
Substitution order: **WorldName, AgentBinaryURL, AgentSecret, SaveURL, DiscordWebhookURL**
- Downloads and starts `gameserver-agent` on port 8080 before starting the game.
- If `SaveURL` is non-empty, downloads and unpacks the save before starting Docker.

## Graceful shutdown / save flow
```
/stopserver label:<label>
  → Lambda: parse worldName from label (strip first segment before "-")
  → Lambda: generate S3 pre-signed PUT URL for CoreKeeper/<worldName>.tar.gz
  → Lambda: POST http://<vps-ip>:8080/shutdown  {upload_url: "..."}
             Authorization: Bearer <GameServerAgentSecret>
  → gameserver-agent: docker stop core-keeper-dedicated
  → gameserver-agent: tar /tmp/core-keeper-data → PUT to S3
  → gameserver-agent: respond 200
  → Lambda: DestroyInstance
  → Lambda: Discord followup "saved and destroyed"
```

## gameserver-agent (`cmd/gameserver/main.go`)
- Pure stdlib Go binary, no external deps.
- Reads `AGENT_SECRET` env var for auth.
- Endpoints: `GET /health` (no auth), `POST /shutdown` (Bearer auth).
- Shutdown: stops Docker container → `tar.gz` save dir → PUT to pre-signed S3 URL.

## Secrets (AWS Secrets Manager: `discordServerManagerBot`)
```json
{
  "VultrAPIKey": "...",
  "DiscordAppID": "...",
  "DiscordToken": "...",
  "DiscordPublicKey": "...",
  "GuildIDs": ["..."],
  "GuildWebhooks": {"<guildID>": "<webhookURL>"},
  "VultrSSHKeyID": "...",
  "GameServerAgentSecret": "...",
  "R2AccountID": "...",
  "R2AccessKeyID": "...",
  "R2SecretAccessKey": "...",
  "R2BucketName": "..."
}
```

## Vultr instance configuration
- Region: `sea` (Seattle)
- Plan: `vx1-g-2c-8g-120s` (2 vCPU, 8 GB RAM, 120 GB NVMe)
- OS: Ubuntu 24.04 (OSID 2284)
- Max concurrent instances: 3

## Adding a new game
1. Add game name constant to `games.go`.
2. Add slash commands to `commands.go`.
3. Add handler to `handlers.go` map.
4. Create handler file (e.g. `StartMyGame.go`).
5. Write a startup script template for that game.
6. Update `registerCommands` if needed.

## Build & deploy

```bash
make deploy              # build Lambda + register commands + zip + push to AWS
make buildGameserverAgent   # cross-compile agent for Linux amd64
make uploadGameserverAgent  # build + upload agent binary to S3
```

## Common gotchas
- The Lambda must respond to Discord within 3 s — always use the deferred work pattern for anything slow.
- `StopServer` must call the agent `/shutdown` endpoint and wait for 200 **before** calling `DestroyInstance`. Never destroy first.
- `extractWorldName(label)` splits on the first `-` to recover the world name from a VPS label.
- `gameserver-agent` binary must be re-uploaded to S3 (`make uploadGameserverAgent`) whenever `cmd/gameserver/` changes.
- Pre-signed S3 URLs used in startup scripts expire in 24 h — instances must start within that window.
