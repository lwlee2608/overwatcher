# High-Level Design

Overwatcher automates the **CD** half of the pipeline for projects deployed as Docker Compose stacks on a VM. CI remains on GitHub Actions — it builds and publishes the image. Overwatcher's job is to replace the manual `ssh → docker pull → docker compose up -d` cycle.

## Architecture

Overwatcher is a central coordinator that integrates with GitHub. The actual `docker pull` + restart is executed by a small **agent** that lives inside each target VM's compose stack. The agent mounts `/var/run/docker.sock` so it can manage its sibling containers, and opens an **outbound** HTTP long-poll connection to Overwatcher to receive deploy commands.

The coordinator is backed by PostgreSQL — it persists users, projects, services (the repo→stack mapping), deploy intents, agent registrations, and the webhook event log. A React frontend provides dashboards for users, projects, agents, deployments, and event logs. Login auth (sessions + cookies) gates the UI and management APIs; the agent transport uses a separate shared-secret bearer token.

```
Developer ──git push main──> GitHub repo ──> GitHub Actions (CI: build image)
                                                    │ push image
                                                    ▼
                                          Container registry
                                                    │
            workflow_run webhook                    │
                  │                                 │
                  ▼                                 │
           ┌─────────────────────┐                  │
           │ Overwatcher         │                  │
           │ (coordinator)       │                  │
           │   HTTP API + UI     │                  │
           │   PostgreSQL        │                  │
           └─────────┬───────────┘                  │
                     │ outbound long-poll           │
                     ▼                              │
           ┌───────────────────────────┐            │
           │ Target VM (compose stack) │            │
           │                           │            │
           │  overwatcher-agent ◀──────┼╌╌╌╌╌╌╌╌╌╌╌╌┘
           │  (mounts docker.sock)     │
           │       │                   │
           │       │ pull + restart    │
           │       ▼                   │
           │  neighbour containers     │
           │  (app, db, etc.)          │
           └───────────────────────────┘
```

## Flow

1. Developer pushes to `main`.
2. GitHub Actions runs CI, builds the Docker image, and pushes it to the container registry.
3. On workflow success GitHub fires a `workflow_run` webhook to Overwatcher. Overwatcher verifies the signature, looks up the matching project/service, and persists a deploy intent.
4. The agent's outbound long-poll picks up the intent.
5. The agent uses `docker.sock` to pull the new image and restart its neighbour containers.
6. The agent reports status back to Overwatcher, which updates the GitHub Deployments API so the result is visible on the commit/PR.

## Why this shape

- **No inbound ports on the VM.** The agent only needs outbound network to Overwatcher and the registry — a much better fit for EC2 instances that shouldn't expose SSH publicly.
- **No central SSH key store.** Credentials don't grow with the fleet.
- **Scales naturally.** Adding a new VM means dropping the agent into its compose file; nothing needs to be registered centrally.
- **Self-contained.** The agent ships as part of the stack it manages.

## Trade-offs to handle

- **Two components to ship.** Overwatcher and the agent image both need to be built, versioned, and released.
- **`docker.sock` is effectively host root.** The agent image must be trustworthy and minimal.
- **Agent self-update is awkward** — the agent can't `pull + restart` the container it's running in. Likely needs a small helper or a host-level watchdog to restart the agent itself.
- **Registry credentials live on every VM.** Usually fine with a read-only pull token per host.
- **Status reporting has two hops.** Agent → Overwatcher → GitHub, rather than direct.

## Scope

- **In scope:** receiving GitHub events, deciding when to deploy, coordinating with the agent, reporting status to GitHub.
- **Out of scope:** building images, running tests, managing application secrets on the VM, provisioning the VM itself.
