# Overwatcher

A GitHub App that automates the **CD** half of a CI/CD pipeline for projects deployed to a VM (e.g. an EC2 instance running Docker Compose).

## What it does

CI stays where it already works well — a normal GitHub Actions workflow on a GitHub runner builds and publishes the Docker image. Overwatcher takes over from there:

1. Listens for the build/deployment event on the target repo.
2. Connects to the target VM and pulls the new image.
3. Restarts the affected Docker Compose services.

## Why

Today the "deploy" step means SSH-ing into the VM, running `docker pull`, then `docker compose up -d` by hand. Overwatcher exists to remove that manual step so a push to `main` is all it takes to ship.

## Architecture

```
GitHub ──webhook──▸ Coordinator (Railway) ──long-poll──▸ Agent (your VM)
                                                            │
                                                    docker compose pull
                                                    docker compose up -d
```

- **Coordinator** — hosted backend that receives GitHub webhooks and queues deploy intents.
- **Agent** — lightweight container running on each target VM. Long-polls the coordinator for work and executes Docker Compose deployments.

## Triggers: push vs. workflow_run

Each service in a project can deploy in one of two ways:

- **`push`** (default, when no `workflow` is configured) — deploy fires the moment GitHub sends a push. Simple, but races CI: if your image hasn't finished building, the agent pulls a stale tag or hits `manifest unknown`.
- **`workflow_run`** (when a `workflow` filename is set on the service, e.g. `build-and-publish.yml`) — deploy fires only after the named GitHub Actions workflow completes successfully. The new image is guaranteed to exist before the agent pulls it.

Set the `workflow` field on a service from the Project detail page in the UI. The GitHub App must be subscribed to the **`workflow_run`** event (in addition to `push`) for this trigger to work. See [`docs/architecture/workflow-run-trigger.md`](docs/architecture/workflow-run-trigger.md) for the full setup checklist and examples.

## Agent Setup

The agent has two deployment modes. Pick one.

### systemd (recommended)

Run the agent as a native binary under systemd. One copy-paste from the
agent dashboard sets it up — you name the agent in the UI, it mints a
token and shows a ready-to-paste install command, you run it on the VM.

```bash
curl -fsSL https://<coordinator>/install.sh | \
sudo AGENT_TOKEN=owa_<token> \
bash
```

The agent runs as a host user in the `docker` group, so there is no
container/host path translation, no socket mounting, and no
`~/.docker/config.json` juggling. See [`docs/agent-systemd.md`](docs/agent-systemd.md)
for install, upgrade, logs, uninstall, and troubleshooting.

### Docker container

Still supported for existing deployments. See [`example/`](example/) for a
working `docker-compose.yml`. The agent talks to the host daemon via a
mounted socket and reads private-registry credentials from a mounted
`~/.docker/config.json`. Defaults are embedded in the binary, so no
`application-agent.yml` needs to be mounted — env vars are enough.

```yaml
overwatcher-agent:
  image: lwlee2608/agent:latest
  environment:
    - AGENT_TOKEN=${AGENT_TOKEN}
    - AGENT_COORDINATOR_URL=https://your-coordinator.example.com
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    - /path/to/your/deployment:/opt/stacks/my-stack
    - /home/dev/.docker/config.json:/root/.docker/config.json:ro
```

The agent uses **Docker Compose v2** (`docker compose` plugin); the
`docker:27-cli` base image ships with it.

## CLI

Install `owctl` for Linux or macOS (amd64/arm64), matching the coordinator's release:

```sh
curl -fsSL https://overwatcher-web-production.up.railway.app/cli.sh | sh
export PATH="$HOME/.local/bin:$PATH"
owctl login
owctl version
owctl project list
```

The installer verifies SHA256 checksums and installs to `~/.local/bin` without sudo. Add that directory to your shell's startup `PATH`. To choose another directory, pass `OWCTL_INSTALL_DIR` to `sh`, not `curl`:

```sh
curl -fsSL https://overwatcher-web-production.up.railway.app/cli.sh | OWCTL_INSTALL_DIR="$HOME/bin" sh
```

For a self-hosted coordinator, use its `/cli.sh` URL and run `owctl login --url https://your-coordinator`. `login` prompts for an API key without echo and saves it in `~/.config/owctl/config.yaml` with mode `0600`. Create a key in **Settings → API keys**. `OVERWATCHER_URL` and `OVERWATCHER_API_KEY` override saved settings; `--url` overrides the URL. Use environment-based auth for non-interactive sessions. Do not commit API keys.

Re-run the installer after upgrading the coordinator. Published releases include `owctl_linux_amd64`, `owctl_linux_arm64`, `owctl_darwin_amd64`, `owctl_darwin_arm64`, and `SHA256SUMS`. Development coordinators without a release tag use the latest release. Installing from source also works from `services/overwatcher-backend` with `go install ./cmd/owctl` (requires Go; install location follows `GOBIN`/`GOPATH`).

## API access

Scripts and AI agents can drive the coordinator API with a personal API key.
Create one under **user menu → API keys**; it's shown once. Send it as a
Bearer token — it acts as you on every `/api/v1` UI route, except managing
API keys and changing your password (those need a login session).

```bash
API=https://<coordinator>/api/v1
AUTH="Authorization: Bearer owk_<key>"

# 1. create the project
PROJECT_ID=$(curl -sf -H "$AUTH" -X POST $API/projects \
  -d '{"name":"my-app","compose_file":"/opt/stacks/my-app/docker-compose.yml"}' | jq -r .id)

# 2. set its services
curl -sf -H "$AUTH" -X PUT $API/projects/$PROJECT_ID/services \
  -d '{"services":[{"name":"web","repo":"acme/my-app","image":"acme/my-app","workflow":"build.yml"}]}'

# 3. bind an agent (see GET $API/agents)
curl -sf -H "$AUTH" -X PUT $API/agents/<agent-id>/project -d "{\"project_id\":\"$PROJECT_ID\"}"
```

### Agent skill

[`skills/overwatcher`](skills/overwatcher/SKILL.md)
teaches an AI agent (Claude Code or any agent that loads `SKILL.md`) this
workflow. Install for Claude Code, then export `OVERWATCHER_API_KEY`
(and `OVERWATCHER_URL` for a self-hosted coordinator):

```bash
mkdir -p ~/.claude/skills/overwatcher
curl -fsSL https://raw.githubusercontent.com/lwlee2608/overwatcher/main/skills/overwatcher/SKILL.md \
  -o ~/.claude/skills/overwatcher/SKILL.md
```

## Docs

- [`docs/architecture/high-level-design.md`](docs/architecture/high-level-design.md) — system shape, components, flow, trade-offs.
- [`docs/architecture/database-schema.md`](docs/architecture/database-schema.md) — Postgres tables and migration history.
- [`docs/architecture/agent-protocol.md`](docs/architecture/agent-protocol.md) — coordinator ↔ agent HTTP contract.
- [`docs/architecture/workflow-run-trigger.md`](docs/architecture/workflow-run-trigger.md) — `workflow_run` setup and failure modes.
- [`docs/agent-systemd.md`](docs/agent-systemd.md) — install, upgrade, logs, and troubleshooting for the systemd agent.
