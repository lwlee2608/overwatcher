# Infrastructure

Snapshot of where Overwatcher and its currently-connected agents run in production.

## Topology

```
      ┌──────────────────────┐                ┌────────────────────────────┐
      │ Browser              │                │ GitHub                     │
      │  · admin UI users    │                │  · workflow_run webhooks   │
      │                      │                │  · Deployments API         │
      └──────────┬───────────┘                └─────────────┬──────────────┘
                 │ HTTPS                                    │ HTTPS
                 │                                          │ /api/v1/github/webhook
   ┌─────────────┼──────────────────────────────────────────┼───────────────────────────────┐
   │ Railway     │                                          │                               │
   │             ▼                                          ▼                               │
   │   ┌──────────────────────┐                 ┌───────────────────────┐       ┌─────────┐ │
   │   │ frontend             │  proxy /api/*   │ backend               │       │Postgres │ │
   │   │ (React + Vite)       │                 │ (Go coordinator)      │       │         │ │
   │   │  nginx               │────────────────▶│   HTTP API            │──────▶│         │ │
   │   └──────────────────────┘                 └───────────────────────┘       └─────────┘ │
   │                                                        ▲                               │
   │                                                        │                               │
   │                                                        │                               │
   └────────────────────────────────────────────────────────┼───────────────────────────────┘
                                                            │ outbound long-poll (HTTPS)
                                                            │ GET /api/v1/deploy/next
                  ┌───────────────────────────┬─────────────┴──────────────────────────────┐
                  ▲                           ▲                                            ▲
                  │                           │                                            │
   ┌──────────────┴───────────┐ ┌─────────────┴────────────┐                ┌──────────────┴───────────┐
   │ Alibaba Cloud ECS        │ │ Google Cloud GCE         │                │ Future VM (TBD)          │
   │ 47.254.192.25            │ │ 34.142.204.214           │                │ (IP TBD)                 │
   │                          │ │                          │                │                          │
   │ agent: medtutor (docker) │ │ agent: twister-chat      │                │ agent: agent-xx          │
   │ ┌──────────────────────┐ │ │        (systemd)         │                │        (?)               │
   │ │ overwatcher-agent    │ │ │ ┌──────────────────────┐ │                │ ┌──────────────────────┐ │
   │ │  container           │ │ │ │ overwatcher-agent.svc│ │                │ │ overwatcher-agent.svc│ │
   │ │  + docker.sock       │ │ │ │  (host-level unit)   │ │                │ │  (host-level unit)   │ │
   │ └──────────┬───────────┘ │ │ └──────────┬───────────┘ │                │ └──────────┬───────────┘ │
   │            │ pull + up   │ │            │ pull + up   │                │            │ pull + up   │
   │            ▼             │ │            ▼             │                │            ▼             │
   │ ┌──────────────────────┐ │ │ ┌──────────────────────┐ │                │ ┌──────────────────────┐ │
   │ │ compose stack        │ │ │ │ compose stack        │ │                │ │ compose stack        │ │
   │ │  (app containers)    │ │ │ │  (app containers)    │ │                │ │  (app containers)    │ │
   │ └──────────────────────┘ │ │ └──────────────────────┘ │                │ └──────────────────────┘ │
   └──────────────────────────┘ └──────────────────────────┘                └──────────────────────────┘
```

## Components

### Coordinator plane — Railway

- **backend** — Go coordinator, exposes the HTTP API and the agent long-poll endpoints. GitHub posts `workflow_run` webhooks directly here (`/api/v1/github/webhook`), agents long-poll it directly (URL baked into `install.sh` via `agent.public_url`), and it reports status back via the Deployments API.
- **frontend** — React + Vite SPA on a separate Railway service. nginx serves the static assets and proxies `/api/*` and `/install.sh` to the backend; browser admin UI traffic enters here.
- **PostgreSQL** — Railway-managed Postgres. Stores users, projects, services, deploy intents, agent registrations, and the webhook event log.

### Agent plane

Agents connect **outbound only** — Railway never initiates a connection toward the VM. There are two packaging modes:

- **docker mode**: the agent runs as a container inside the same compose stack it manages, with `/var/run/docker.sock` bind-mounted in.
- **systemd mode**: the agent runs as a host-level systemd unit installed via `install.sh`, and shells out to `docker compose` against one or more stacks on the host.

See [high-level-design.md](high-level-design.md) for the deploy flow and [agent-protocol.md](agent-protocol.md) for the wire protocol.
