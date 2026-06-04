# Overwatcher Backend

Overwatcher automates the CD half of the pipeline for projects deployed as Docker Compose stacks on VMs. GitHub Actions handles CI (building and publishing images); Overwatcher replaces the manual `ssh -> docker pull -> docker compose up -d` cycle.

## Architecture

```
┌──────────┐       ┌─────────────────────────────────────────────┐
│  GitHub  │       │              Coordinator                    │
│          │       │                                             │
│  push to ├──────>│  Webhook ──> Mapping ──> Intent Store       │
│  main    │  POST │  Service     Index       (PostgreSQL)       │
│          │       │                              │              │
│          │       │              Reaper          │  TakeNext    │
│          │       │          (sweep timeouts)    │  (long-poll) │
│          │       │                              ▼              │
│Deployment│<──────│  Dispatch <── Deploy  <── /deploy/next      │
│  Status  │  API  │  Service     Handler                        │
│          │       │                                             │
│          │       │  Agent Tracker <── Heartbeat Middleware     │
└──────────┘       └──────────────────────────────┬──────────────┘
                                                  │
                                           long-poll (HTTP)
                                                  │
                   ┌──────────────────────────────▼──────────────┐
                   │                 Agent (VM)                  │
                   │                                             │
                   │  Poller ──> Runner ──> docker compose pull  │
                   │                        docker compose up -d │
                   │                                             │
                   │  POST /deploy/{id}/result ──> Coordinator   │
                   └─────────────────────────────────────────────┘
```

## Components

### Coordinator (`cmd/overwatcher/`)

The central HTTP server. Receives GitHub webhooks, manages the deploy intent queue, and serves intents to agents via long-poll.

| Package                         | Role                                                                                                  |
| ------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `internal/api/http/handler/`    | HTTP handlers: webhook ingress, deploy long-poll, result reporting, agent listing                     |
| `internal/api/http/middleware/` | Webhook signature verification, bearer token auth, agent heartbeat tracking, request logging          |
| `internal/api/http/dto/`        | Request/response DTOs for deploy, webhook, agent, and health endpoints                                |
| `internal/service/webhook/`     | Parses push events, looks up repo-to-stack mapping, creates GitHub Deployments, enqueues intents      |
| `internal/service/intent/`      | `Store` interface backed by `DBStore` (PostgreSQL). `MemoryStore` exists for tests only               |
| `internal/service/dispatch/`    | Consumes intents via long-poll, updates GitHub Deployment status, owns the `Reaper` for timeout/retry |
| `internal/service/mapping/`     | In-memory index mapping repos to target stacks, resolves image and tag conventions                    |
| `internal/service/agent/`       | In-memory agent tracker with heartbeat TTL, exposes connection status                                 |
| `internal/github/`              | GitHub App client wrapper (ghinstallation auth)                                                       |
| `internal/db/`                  | PostgreSQL pool init, goose migrations, sqlc-generated queries                                        |

### Agent (`cmd/agent/`)

A lightweight binary that runs on each target VM. It long-polls the coordinator for work, executes Docker Compose commands, and reports the result.

| Package               | Role                                                         |
| --------------------- | ------------------------------------------------------------ |
| `cmd/agent/poll.go`   | Long-poll loop with exponential backoff                      |
| `cmd/agent/runner.go` | Shells out to `docker compose pull` + `docker compose up -d` |

The agent mounts `/var/run/docker.sock` and is single-threaded by design (one deploy at a time).

## Request Flow

1. GitHub sends a push webhook to `POST /api/v1/github/webhook`
2. Middleware validates the HMAC-SHA256 signature
3. Webhook service matches the repo against configured mappings
4. For each match: create a GitHub Deployment, enqueue a `DeployIntent`, mark it "queued"
5. Agent long-polls `GET /api/v1/deploy/next` (25s server timeout)
6. Dispatch service returns the next dispatchable intent (concurrency guard: one per stack)
7. Agent runs `docker compose pull` + `docker compose up -d`
8. Agent posts result to `POST /api/v1/deploy/{id}/result`
9. Dispatch service updates GitHub Deployment to success or failure

## Intent Store

Deploy intents are persisted in PostgreSQL. The `DBStore` uses sqlc-generated queries with atomic `FOR UPDATE SKIP LOCKED` for dispatch and `UNIQUE(delivery_id, stack_index)` for webhook redelivery dedup. Intents survive coordinator restarts.

Only one intent per stack is dispatched at a time (concurrency guard), preventing duplicate deployments to the same stack.

## Configuration

### Coordinator (`application.yml`)

```yaml
log:
  level: debug
http:
  port: 8080
github:
  app_id: 0
  webhook_secret: "" # env: GITHUB_WEBHOOK_SECRET
  private_key: "" # env: GITHUB_PRIVATE_KEY
deployments:
  mappings:
    - repo: "owner/my-app"
      stack: "my-stack"
      services: ["app"] # optional; empty = all services
      environment: "production" # optional; default "production"
      image: "ghcr.io/owner/app" # optional; default ghcr.io/<repo>
      tag: "latest" # optional; default commit SHA
agent:
  public_url: "" # env: AGENT_PUBLIC_URL; empty falls back to install.sh request Host
database:
  url: "" # required; env: DATABASE_URL
  schema: "" # optional; defaults to "public"
dispatch:
  in_flight_timeout: 10m # reaper timeout threshold
  max_attempts: 3 # max retries before permanent failure
  sweep_interval: 1m # reaper scan frequency
  shutdown_timeout: 30s # graceful shutdown deadline
```

### Agent

Defaults live in `internal/agent/application-agent.yml` and are embedded
into the binary via `//go:embed`. Production deployments override them via
env vars (`AGENT_TOKEN`, `AGENT_COORDINATOR_URL`, `AGENT_NAME`,
`AGENT_POLL_TIMEOUT`, `LOG_LEVEL`); for local dev, place an
`application-agent.yml` next to the binary to override the embedded copy.

```yaml
log:
  level: info
agent:
  name: ""                  # optional, descriptive only; env: AGENT_NAME
  coordinator_url: "http://localhost:8080"
  token: ""                 # required; env: AGENT_TOKEN
  poll_timeout: 30s         # must exceed coordinator's 25s long-poll
```

The compose-file path is **carried on each deploy intent** by the
coordinator (`projects.compose_file`), not configured per-agent.

## Build

```
make build        # coordinator -> bin/overwatcher
make build-agent  # agent -> bin/agent
make test         # run all tests
make generate     # regenerate sqlc code
make dep          # install sqlc tooling
```
