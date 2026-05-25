# Database Schema

PostgreSQL backs the entire control plane. Migrations live under `services/overwatcher-backend/internal/db/migrations/` and are applied with [goose](https://github.com/pressly/goose) on coordinator startup. SQLC generates the Go query layer from `internal/db/queries/*.sql`.

## Shape

The schema is split along three axes:

- **Auth & tenancy** — `users`, `sessions`, `project_members`. The UI logs in with email + password (cookie-based sessions); a "bootstrap admin" is seeded with a one-time password on first start. `project_members` lets the owner share a project with other users.
- **Deploy model** — `projects`, `services`, `agents`, `deploy_intents`. A user owns N projects. A project has exactly one compose file, exactly one agent (1:1, partial unique), and N services. A service is a `(repo, root_directory, branch, image, tag, workflow)` row that triggers a deploy when its `workflow_run` arrives.
- **Observability** — `event_logs`. Every received GitHub webhook delivery is recorded so the UI can show what was seen, even when it didn't produce a deploy.

`deploy_intents` is the deploy event log. It denormalizes everything it needs from the project/services rows at enqueue time (`compose_file`, `services_spec` JSONB, `repo`, `sha`) so deletions in the config tables don't erase deploy history.

## ER Diagram

```
 ┌──────────────────────────────────┐         ┌──────────────────────────────────┐
 │             users                │         │           sessions               │
 ├──────────────────────────────────┤  1:N    ├──────────────────────────────────┤
 │ id                    UUID PK    │────────▶│ token       VARCHAR(64) PK       │
 │ email                 VARCHAR UQ │         │ user_id     UUID FK → users      │
 │ name                  VARCHAR    │         │ expires_at  TIMESTAMPTZ          │
 │ password_hash         VARCHAR    │         │ created_at  TIMESTAMPTZ          │
 │ password_is_bootstrap BOOLEAN    │         └──────────────────────────────────┘
 │ created_at            TIMESTAMPTZ│
 │ updated_at            TIMESTAMPTZ│
 └────────────────┬─────────────────┘
                  │ 1:N
                  ▼
 ┌──────────────────────────────────┐         ┌──────────────────────────────────┐
 │           projects               │  1:1    │            agents                │
 ├──────────────────────────────────┤◀────────├──────────────────────────────────┤
 │ id           UUID PK             │         │ id            UUID PK            │
 │ user_id      UUID FK → users     │         │ name          VARCHAR UQ         │
 │ name         VARCHAR             │         │ project_id    UUID FK → projects │
 │ description  TEXT                │         │ remote_ip     VARCHAR(45)        │
 │ compose_file VARCHAR(512)        │         │ agent_type    TEXT               │
 │ environment  VARCHAR(64)         │         │ version       TEXT               │
 │ enabled      BOOLEAN             │         │ last_seen_at  TIMESTAMPTZ        │
 │ created_at   TIMESTAMPTZ         │         │ created_at    TIMESTAMPTZ        │
 │ updated_at   TIMESTAMPTZ         │         │ updated_at    TIMESTAMPTZ        │
 │                                  │         │                                  │
 │ UQ(user_id, name)                │         │ partial UQ(project_id)           │
 │                                  │         │   WHERE project_id IS NOT NULL   │
 └────────────────┬─────────────────┘         └──────────────────────────────────┘
                  │ 1:N
                  ▼
 ┌──────────────────────────────────┐         ┌──────────────────────────────────┐
 │           services               │         │        project_members           │
 ├──────────────────────────────────┤         ├──────────────────────────────────┤
 │ id              UUID PK          │         │ project_id  UUID FK → projects   │
 │ project_id      UUID FK→projects │         │ user_id     UUID FK → users      │
 │ name            VARCHAR          │         │ role        VARCHAR(16)          │
 │ repo            VARCHAR(512)     │         │ added_by    UUID FK → users      │
 │ root_directory  VARCHAR(512)     │         │ created_at  TIMESTAMPTZ          │
 │ branch          VARCHAR(255)     │         │                                  │
 │ image           VARCHAR(512)     │         │ PK(project_id, user_id)          │
 │ tag             VARCHAR(255)     │         └──────────────────────────────────┘
 │ workflow        VARCHAR(255)     │
 │ position        INTEGER          │
 │ created_at      TIMESTAMPTZ      │
 │ updated_at      TIMESTAMPTZ      │
 │                                  │
 │ UQ(project_id, name)             │
 └──────────────────────────────────┘


 ┌──────────────────────────────────┐         ┌──────────────────────────────────┐
 │         deploy_intents           │         │          event_logs              │
 ├──────────────────────────────────┤         ├──────────────────────────────────┤
 │ id               UUID PK         │         │ id          UUID PK              │
 │ delivery_id      VARCHAR(255)    │         │ delivery_id VARCHAR(255)         │
 │ project_id       UUID            │         │ event_type  VARCHAR(100)         │
 │ repo             VARCHAR(512)    │         │ repo        VARCHAR(512)         │
 │ git_ref          VARCHAR(255)    │         │ sender      VARCHAR(255)         │
 │ sha              VARCHAR(64)     │         │ summary     TEXT                 │
 │ stack            VARCHAR(255)    │         │ created_at  TIMESTAMPTZ          │
 │ services_spec    JSONB           │         └──────────────────────────────────┘
 │ compose_file     VARCHAR(512)    │
 │ environment      VARCHAR(255)    │
 │ deployment_id    BIGINT          │
 │ installation_id  BIGINT          │
 │ status           VARCHAR(32)     │
 │ attempts         INTEGER         │
 │ created_at       TIMESTAMP       │
 │ updated_at       TIMESTAMP       │
 │ dispatched_at    TIMESTAMP       │
 │                                  │
 │ partial UQ(delivery_id,          │
 │           project_id)            │
 └──────────────────────────────────┘
```

`deploy_intents.project_id` is intentionally **not** a FK — deleting a project keeps its history rows readable. The partial unique on `(delivery_id, project_id) WHERE project_id IS NOT NULL` provides webhook-redelivery dedup.

## Table Details

### users

UI auth. `password_hash` uses bcrypt; `password_is_bootstrap = true` marks accounts still on their seeded password and forces a change on first login. Cleared on the user's first `ChangePassword`.

### sessions

Server-side session store. Cookies carry only the opaque `token`. An hourly reaper deletes rows where `expires_at < NOW()`. Password changes revoke all sessions for that user.

### projects

The deployable unit (Railway-style). Each project owns exactly one `compose_file` and binds to exactly one agent. `UNIQUE(user_id, name)` lets two users both have a project called `staging`. Deleting a user cascades to their projects, and through them to services.

### services

A compose service definition. The webhook handler looks up services by `(LOWER(repo), workflow)` (partial index) — a `workflow_run` for `owner/repo` from workflow `deploy.yml` matches any service with `repo='owner/repo'` and `workflow='deploy.yml'`. `root_directory` filters by changed paths within the repo. `image`/`tag` drive what the agent passes to `docker compose pull` + `up -d`. `position` orders services within a project.

### agents

Persistent agent registration. The agent upserts its row on first poll, binds itself to a project (1:1, enforced by partial unique `idx_agents_project_id WHERE project_id IS NOT NULL`), and `last_seen_at` updates on every poll. `compose_file` is **not** stored here — it lives on the project, since an agent serves exactly one project. `agent_type` (`docker` or `systemd`) and `version` are reported by the agent via `X-Agent-Type` / `X-Agent-Version` headers on every poll; empty values leave any existing stored value intact.

### project_members

Shares a project with users other than the owner. The owner (the user pointed to by `projects.user_id`) is implicit — no `project_members` row is needed for them. Membership is checked on read/write paths to gate access; `role` is currently always `member` but reserved for future role expansion. Adding the owner as a member is rejected at the service layer (`ErrCannotAddOwner`).

### deploy_intents

Append-only deploy event log. A webhook produces one intent per affected project, with `services_spec` as a JSONB array of `{name, image, tag}` objects captured at enqueue time. The dispatcher transitions `status` through `created → dispatched → succeeded|failed`; `attempts` and `dispatched_at` drive retry/reaper logic. `compose_file` is denormalized so the agent can run the right compose file even after the project is renamed or deleted.

### event_logs

Every received GitHub webhook delivery, including ones that didn't match any service. The frontend's Event Log dashboard reads from this table.

## Migration History

Migrations are numbered 0001–0019 and applied in order. Notable points:

- `0001` — `deploy_intents` (the original MVP table).
- `0003` — original `agents` + `deploy_mappings` schema.
- `0006` — `event_logs`.
- `0009` — replaces `deploy_intents.image/tag/services` with `services_spec JSONB`.
- `0010` — adds `users`, `projects`, `services`; `agents.project_id` and `deploy_intents.project_id` added additively (coexist with legacy `deploy_mappings`).
- `0011` — adds `deploy_intents.compose_file` and partial unique on `(delivery_id, project_id)`; backfills legacy mappings into a bootstrap user/project/services.
- `0012` — drops `deploy_mappings`, `deploy_mapping_services`, `deploy_intents.stack_index`, and `agents.compose_file` after cutover.
- `0013` — `services.workflow` for the `workflow_run` trigger.
- `0014`–`0016` — login auth: `users.password_hash`, `sessions`, `users.password_is_bootstrap`.
- `0017` — `project_members` for sharing a project with users other than its owner.
- `0018` — `agents.agent_type` (`docker` / `systemd`), reported by the agent on each poll.
- `0019` — `agents.version`, the agent's build version, reported on each poll.
