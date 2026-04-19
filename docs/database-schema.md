# Database Schema

## Current State

Only the `deploy_intents` table exists. Agent registration is in-memory (heartbeat tracker), and all routing config lives in YAML files:

- **Coordinator** `application.yml` — repo-to-stack mappings
- **Agent** `application-agent.yml` — stack-to-compose-file mappings

## New Design

Move routing config to PostgreSQL so it can be managed via UI. Agents auto-register with the coordinator on startup.

### Key simplification: 1 agent = 1 compose file

The previous design had a separate `stacks` table allowing a single agent to manage multiple compose files. This added unnecessary complexity — the agent already lives inside the compose stack it manages. Enforcing a 1:1 relationship means the agent *is* the stack, eliminating the `stacks` table entirely and reducing the schema to two new tables: `agents` and `deploy_mappings`.

### Per-service image/tag

Each service in a mapping carries its own image and tag, stored in a `deploy_mapping_services` child table. A single mapping can deploy N services atomically (e.g. frontend + backend + worker in one repo). Intents denormalize the services list as a `services_spec` JSONB snapshot so historical deployments remain readable even after the mapping changes.

## ER Diagram

```
 ┌──────────────────────────────────┐
 │            agents                │
 ├──────────────────────────────────┤
 │ id            UUID PK            │
 │ name          VARCHAR(255) UQ    │
 │ compose_file  VARCHAR(512)       │
 │ remote_ip     VARCHAR(45)        │
 │ last_seen_at  TIMESTAMPTZ        │
 │ created_at    TIMESTAMPTZ        │
 │ updated_at    TIMESTAMPTZ        │
 └──────────────┬───────────────────┘
                │
                │ 1:N
                │
 ┌──────────────┴───────────────────┐
 │        deploy_mappings           │
 ├──────────────────────────────────┤
 │ id            UUID PK            │
 │ repo          VARCHAR(512)       │
 │ agent_id      UUID FK → agents   │
 │ environment   VARCHAR(255)       │
 │ enabled       BOOLEAN            │
 │ created_at    TIMESTAMPTZ        │
 │ updated_at    TIMESTAMPTZ        │
 │                                  │
 │ UQ(repo, agent_id)               │
 └──────────────┬───────────────────┘
                │
                │ 1:N
                │
 ┌──────────────┴───────────────────┐
 │   deploy_mapping_services        │
 ├──────────────────────────────────┤
 │ id            UUID PK            │
 │ mapping_id    UUID FK → mappings │
 │ name          VARCHAR(255)       │
 │ image         VARCHAR(512)       │
 │ tag           VARCHAR(255)       │
 │ position      INTEGER            │
 │                                  │
 │ UQ(mapping_id, name)             │
 └──────────────────────────────────┘


 ┌──────────────────────────────────┐
 │     deploy_intents (existing)    │
 ├──────────────────────────────────┤
 │ id              UUID PK          │
 │ delivery_id     VARCHAR(255)     │
 │ stack_index     INTEGER          │
 │ repo            VARCHAR(512)     │
 │ git_ref         VARCHAR(255)     │
 │ sha             VARCHAR(64)      │
 │ stack           VARCHAR(255)     │
 │ services_spec   JSONB            │
 │ environment     VARCHAR(255)     │
 │ deployment_id   BIGINT           │
 │ installation_id BIGINT           │
 │ status          VARCHAR(32)      │
 │ attempts        INTEGER          │
 │ created_at      TIMESTAMPTZ      │
 │ updated_at      TIMESTAMPTZ      │
 │ dispatched_at   TIMESTAMPTZ      │
 │                                  │
 │ UQ(delivery_id, stack_index)     │
 └──────────────────────────────────┘
```

`deploy_intents` is an event log. It stores denormalized copies of stack/services/repo at the time of the event, so it does not FK into the config tables. Deleting a mapping does not erase deployment history. `services_spec` is a JSONB array of `{name, image, tag}` objects captured when the intent is enqueued.

## Table Details

### agents

Persists agent registration. Replaces both the in-memory `Tracker` and the `application-agent.yml` config file. Each agent manages exactly one compose file — the one it lives in. The agent auto-discovers its compose file from a conventional mount path and upserts its row on first poll. `last_seen_at` is updated on every subsequent poll.

| Column       | Type         | Notes                              |
|--------------|--------------|------------------------------------|
| id           | UUID PK      | gen_random_uuid()                  |
| name         | VARCHAR(255) | unique, set by agent or hostname   |
| compose_file | VARCHAR(512) | e.g. "/opt/stacks/medtutor/docker-compose.prod.yml" |
| remote_ip    | VARCHAR(45)  | updated on each heartbeat          |
| last_seen_at | TIMESTAMPTZ  | updated on each heartbeat          |
| created_at   | TIMESTAMPTZ  | default NOW()                      |
| updated_at   | TIMESTAMPTZ  | default NOW()                      |

### deploy_mappings

Routing rules configured via UI. Replaces the `deployments.mappings` list in `application.yml`. When a webhook arrives for `repo`, the coordinator queries this table to find which agent(s) and service(s) to deploy.

| Column      | Type         | Notes                                  |
|-------------|--------------|----------------------------------------|
| id          | UUID PK      | gen_random_uuid()                      |
| repo        | VARCHAR(512) | "owner/repo-name"                      |
| agent_id    | UUID FK      | references agents(id)                  |
| environment | VARCHAR(255) | default "production"                   |
| enabled     | BOOLEAN      | default true; allows disabling without deleting |
| created_at  | TIMESTAMPTZ  | default NOW()                          |
| updated_at  | TIMESTAMPTZ  | default NOW()                          |

Unique constraint on `(repo, agent_id)` — one mapping per repo per agent.

### deploy_mapping_services

Per-service image/tag attached to a mapping. The runner iterates this list and runs `docker compose pull <name>` / `up -d <name>` once per row with `IMAGE` and `IMAGE_TAG` env set from the row. An empty `name` means "apply to the whole compose stack" (preserved from the legacy empty-services semantics).

| Column     | Type         | Notes                                          |
|------------|--------------|------------------------------------------------|
| id         | UUID PK      | gen_random_uuid()                              |
| mapping_id | UUID FK      | references deploy_mappings(id) ON DELETE CASCADE |
| name       | VARCHAR(255) | compose service name; "" = whole stack         |
| image      | VARCHAR(512) | e.g. "ghcr.io/owner/web"                       |
| tag        | VARCHAR(255) | default "latest"                               |
| position   | INTEGER      | ordering within the mapping                    |

Unique constraint on `(mapping_id, name)`.

## Setup Flow (Before vs After)

**Before (YAML):**
1. Edit coordinator `application.yml` with repo-to-stack mappings, redeploy coordinator
2. Write `application-agent.yml` with stack-to-compose mappings
3. Mount the config file into the agent container
4. Keep "stack" names in sync across both files

**After (DB + UI):**
1. Add the agent to your docker-compose file with just env vars (secret + coordinator URL)
2. Agent auto-registers itself and its compose file with the coordinator
3. Open UI, see registered agents, create mappings (repo → agent + services)
