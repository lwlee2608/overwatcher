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

Set the `workflow` field on a service from the Project detail page in the UI. The GitHub App must be subscribed to the **`workflow_run`** event (in addition to `push`) for this trigger to work.

## Agent Setup

Run the agent as a Docker container alongside your application stack. See [`example/`](example/) for a complete setup.

1. Create `application-agent.yml` to map stack names to compose file paths:

   ```yaml
   agent:
     stacks:
       my-stack: /opt/stacks/my-stack/docker-compose.yml
   ```

2. Add the agent to your `docker-compose.yml`:

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
       - ./application-agent.yml:/go/bin/application-agent.yml
   ```

3. Start the agent:

   ```bash
   AGENT_SHARED_SECRET=<secret> docker compose up -d overwatcher-agent
   ```

Key points:
- The coordinator URL **must** include the `https://` scheme.
- Mount the deployment directory into `/opt/stacks/<stack-name>` so the agent can access the compose file.
- Mount `application-agent.yml` to `/go/bin/application-agent.yml` to configure stacks.
- The agent needs the Docker socket to run `docker compose` commands on the host.
- The agent uses **Docker Compose v2** (`docker compose` plugin). The `docker:27-cli` base image ships with the plugin, so it works regardless of what's installed on the host.
