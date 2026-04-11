# High-Level Design

Overwatcher automates the **CD** half of the pipeline for projects deployed as Docker Compose stacks on a VM. CI remains on GitHub Actions — it builds and publishes the image. Overwatcher's job is to replace the manual `ssh → docker pull → docker compose up -d` cycle.

## Architecture

Overwatcher is a central coordinator that integrates with GitHub. The actual `docker pull` + restart is executed by a small **agent** that lives inside each target VM's compose stack. The agent mounts `/var/run/docker.sock` so it can manage its sibling containers, and opens an **outbound** connection to Overwatcher (long-poll, SSE, or gRPC stream) to receive deploy commands.

```
Developer ──git push main──> GitHub repo ──triggers workflow──> GitHub Actions (CI: build image)
                                │                                    │ push image
                                │ webhook event                      ▼
                                ▼                        Container registry
                          Overwatcher                                │
                          (central coordinator)                      │
                                │                       image pulled │
                                │ outbound long-poll/                │
                                │ stream                             │
                                ▼                                    │
                      ┌───────────────────────────┐                  │
                      │ Target VM (compose stack) │                  │
                      │                           │                  │
                      │  overwatcher-agent ◀──────┼─── ─ ─ -- ─ ─ ─ ─┘
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
3. GitHub delivers a webhook event to Overwatcher. Overwatcher verifies the signature and decides a deploy is needed.
4. Overwatcher hands the deploy command to the agent over the agent's outbound connection.
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
