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

For a self-hosted coordinator, use its `/cli.sh` URL and run `owctl login --url https://your-coordinator`. `login` prompts for an API key without echo and saves it in `~/.config/owctl/config.yaml` with mode `0600`. Create a key in **user menu → API keys**. `OVERWATCHER_URL` and `OVERWATCHER_API_KEY` override saved settings; `--url` overrides the URL. Use environment-based auth for non-interactive sessions. Do not commit API keys.

Re-run the installer after upgrading the coordinator. Published releases include `owctl_linux_amd64`, `owctl_linux_arm64`, `owctl_darwin_amd64`, `owctl_darwin_arm64`, and `SHA256SUMS`. Development coordinators without a release tag use the latest release. Installing from source also works from `services/overwatcher-backend` with `go install ./cmd/owctl` (requires Go; install location follows `GOBIN`/`GOPATH`).

## API access

Use [owctl](#cli) for scripts and AI agents. Create a personal API key under
**user menu → API keys** (shown once), then supply `OVERWATCHER_API_KEY` through
your session environment or secret manager. Do not put the key in command
arguments, logs, or committed files. Set `OVERWATCHER_URL` to the coordinator
base URL (without `/api/v1`); nonempty environment values override saved settings.
`--json` returns raw API JSON; failures exit nonzero with errors on stderr.

```sh
export OVERWATCHER_URL="${OVERWATCHER_URL:-https://overwatcher-web-production.up.railway.app}"
owctl project list --json
owctl agent list --json

# Create only if absent; this compose file must already exist on the VM.
owctl project create my-app --compose-file /opt/stacks/my-app/docker-compose.yml --json
```

Write `services.yaml` with the complete desired services list:

```yaml
services:
  - name: web
    repo: acme/my-app
    image: ghcr.io/acme/my-app
    workflow: build.yml
```

```sh
owctl service set my-app -f services.yaml --json
owctl agent install --ssh ubuntu@vm --project my-app --json
owctl project get my-app --json
owctl agent list --json
```

`service set` replaces all services (YAML or JSON; `-f -` for stdin). The service
name must match its Compose key. Project references accept a name or ID.
For an existing free agent, use `owctl agent bind <agent-id> my-app --json`
instead of installing. Check both sides before binding: it silently displaces
existing bindings, including through `agent install --project`.

SSH installation asks for confirmation, requires non-interactive SSH and a
non-root login with passwordless sudo on a systemd VM, and checks for Compose v2
and an already-active agent. Use `--yes` only after approving the target and
operation. It creates an agent, installs it, waits for a heartbeat, then binds
it. `--timeout` defaults to `60s` for the whole operation. The agent token travels
via SSH stdin and is not printed; the installer stores it on the VM. On failure,
owctl attempts to delete the created agent, but remote files/service may remain
and cleanup can fail. Inspect errors before retrying; see the
[systemd guide](docs/agent-systemd.md) for troubleshooting.

The CLI covers core project/service/agent workflows, not every API route.
Direct API clients can still send the personal key as a Bearer token on
`/api/v1` UI routes. It acts as you, except that managing API keys and changing
your password require a login session.

### Agent skill

[`skills/overwatcher`](skills/overwatcher/SKILL.md)
teaches an AI agent (Claude Code or any agent that loads `SKILL.md`) the owctl
workflow: CLI bootstrap through `/cli.sh`, environment-based auth, JSON output,
service replacement, and confirmed SSH installation. It does not use `owctl login`
or hand-written curl API calls. Install for Claude Code, then securely supply
`OVERWATCHER_API_KEY` and the coordinator's `OVERWATCHER_URL` in the agent's
environment:

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
