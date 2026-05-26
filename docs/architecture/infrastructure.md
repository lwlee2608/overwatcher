# Infrastructure

Snapshot of where Overwatcher and its currently-connected agents run in production.

## Topology

```
                       ┌───────────────────────────────┐
                       │ GitHub                        │
                       │  · workflow_run webhooks      │
                       │  · Deployments API            │
                       └──────────────┬────────────────┘
                                      │ HTTPS
                                      ▼
   ┌───────────────────────────────────────────────────────────────────────────────────┐
   │ Railway                                                                           │
   │                                                                                   │
   │   ┌────────────────────┐    ┌─────────────────────────┐    ┌────────────────────┐ │
   │   │ frontend           │    │ backend                 │    │ PostgreSQL         │ │
   │   │ (React + Vite)     │───▶│ (Go coordinator)        │───▶│                    │ │
   │   └────────────────────┘    │   HTTP API + UI         │    └────────────────────┘ │
   │                             └────────────┬────────────┘                           │
   │                                          │                                        │
   └──────────────────────────────────────────┼────────────────────────────────────────┘
                                              │ outbound long-poll (HTTPS)
                  ┌───────────────────────────┼─────────────────────────────────┐
                  │                           │                                 │
                  ▼                           ▼                                 ▼
   ┌──────────────────────────┐ ┌──────────────────────────┐       ┌──────────────────────────┐
   │ Alibaba Cloud ECS        │ │ Google Cloud GCE         │       │ Future VM (TBD)          │
   │ 47.254.192.25            │ │ 34.142.204.214           │       │ (IP TBD)                 │
   │                          │ │                          │       │                          │
   │ agent: medtutor (docker) │ │ agent: twister-chat      │       │ agent: agent-xx          │
   │ ┌──────────────────────┐ │ │        (systemd)         │       │        (?)               │
   │ │ overwatcher-agent    │ │ │ ┌──────────────────────┐ │       │ ┌──────────────────────┐ │
   │ │  container           │ │ │ │ overwatcher-agent.svc│ │       │ │ overwatcher-agent.svc│ │
   │ │  + docker.sock       │ │ │ │  (host-level unit)   │ │       │ │  (host-level unit)   │ │
   │ └──────────┬───────────┘ │ │ └──────────┬───────────┘ │       │ └──────────┬───────────┘ │
   │            │ pull + up   │ │            │ pull + up   │       │            │ pull + up   │
   │            ▼             │ │            ▼             │       │            ▼             │
   │ ┌──────────────────────┐ │ │ ┌──────────────────────┐ │       │ ┌──────────────────────┐ │
   │ │ compose stack        │ │ │ │ compose stack        │ │       │ │ compose stack        │ │
   │ │  (app containers)    │ │ │ │  (app containers)    │ │       │ │  (app containers)    │ │
   │ └──────────────────────┘ │ │ └──────────────────────┘ │       │ └──────────────────────┘ │
   └──────────────────────────┘ └──────────────────────────┘       └──────────────────────────┘
```

## Components

### Coordinator plane — Railway

- **backend** — Go coordinator, exposes the HTTP API, UI assets, and the agent long-poll endpoints. Receives `workflow_run` webhooks from GitHub and reports status back via the Deployments API.
- **frontend** — React + Vite SPA, served as a separate Railway service.
- **PostgreSQL** — Railway-managed Postgres. Stores users, projects, services, deploy intents, agent registrations, and the webhook event log.

### Agent plane

Agents connect **outbound only** — Railway never initiates a connection toward the VM. There are two packaging modes:

- **docker mode**: the agent runs as a container inside the same compose stack it manages, with `/var/run/docker.sock` bind-mounted in.
- **systemd mode**: the agent runs as a host-level systemd unit installed via `install.sh`, and shells out to `docker compose` against one or more stacks on the host.

See [high-level-design.md](high-level-design.md) for the deploy flow and [agent-protocol.md](agent-protocol.md) for the wire protocol.
