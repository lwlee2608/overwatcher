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

Set the `workflow` field on a service from the Project detail page in the UI. The GitHub App must be subscribed to the **`workflow_run`** event (in addition to `push`) for this trigger to work. See [`docs/workflow-run-trigger.md`](docs/workflow-run-trigger.md) for the full setup checklist and examples.

## Agent Setup

The agent has two deployment modes. Pick one.

### systemd (recommended)

Run the agent as a native binary under systemd. One copy-paste from the
agent dashboard sets it up — the UI shows the install command, you paste
it onto the VM.

```bash
curl -fsSL https://<coordinator>/install.sh | \
sudo AGENT_NAME=my-agent \
AGENT_SHARED_SECRET=<paste-secret> \
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
    - AGENT_SHARED_SECRET=${AGENT_SHARED_SECRET}
    - AGENT_COORDINATOR_URL=https://your-coordinator.example.com
    - AGENT_NAME=my-agent
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    - /path/to/your/deployment:/opt/stacks/my-stack
    - /home/dev/.docker/config.json:/root/.docker/config.json:ro
```

The agent uses **Docker Compose v2** (`docker compose` plugin); the
`docker:27-cli` base image ships with it.
