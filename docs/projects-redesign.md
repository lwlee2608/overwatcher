# Projects Redesign (Railway-style)

## Motivation

The current model is keyed by `(repo → agent)`: a `deploy_mapping` binds one repo to one agent, and services (image/tag pairs) hang off the mapping. That shape breaks down in two places:

1. **Monorepos with many services.** A repo containing `web/`, `api/`, and `worker/` has three CI pipelines publishing three images. Today they all share one mapping row, one list of services, and — critically — one set of triggering rules. There's no way to say "a push that only touches `web/` should redeploy only `web`."
2. **One deployable unit across repos.** A compose stack often stitches together services from several repos (e.g. `frontend-repo` + `backend-repo` running on the same VM). Today those are two separate mappings that happen to point at the same agent; there is no object representing "the thing deployed on that VM."

Railway solves both with a **Project** as the unit of deployment: one compose file, one agent, and N services — each service independently linked to a repo (and a path inside it). We want the same shape.

## Concepts

| Concept | What it is | Relationship |
|---|---|---|
| **User** | The owner of projects. The unit of authentication for the UI and of isolation between tenants. | 1 user — N projects |
| **Project** | The deployable unit. Owns exactly one compose file and exactly one agent. | N projects — 1 user, 1 project — 1 agent, 1 project — N services |
| **Agent** | The process on a VM that runs `docker compose` for its project. Binds to a pre-existing project on first poll via a project-scoped token. | 1 agent — 1 project |
| **Service** | A compose service. Links to one GitHub repo (+ root directory) and one image:tag. | N services — 1 project. Many services can share a repo. |
| **DeployIntent** | Queue item: "project P, deploy services [s1, s2] at SHA X". Immutable event log. | N intents — 1 project |

Key invariants:

- **A project belongs to exactly one user.** Enforced by `projects.user_id NOT NULL`. Deleting a user cascades to their projects (and through them to services and agents); historical `deploy_intents` rows are preserved with `project_id = NULL`, same as project deletion today.
- **Project names are unique per user, not globally.** Two users can both own a project called `staging`. Enforced by `UNIQUE(user_id, name)` on `projects`.
- **Repos are not scoped to users.** The webhook handler fans out across every user's services that match the pushed repo. A single GitHub repo can feed projects owned by different users without any cross-tenant coordination.
- **Project ↔ Agent is 1:1.** An agent can't serve two projects; a project can't have two agents. Enforced by `agents.project_id UNIQUE`.
- **A repo may appear in many services across many projects.** A webhook for `owner/repo` fans out to every service whose `repo = 'owner/repo'` and whose `root_directory` matches the push's changed paths.
- **Intents are per-project, not per-service.** A single webhook that affects services `web` and `api` in the same project produces one intent with two services in its spec, so compose is invoked once.
- **Projects are created before agents.** A user creates a project in the UI, receives a project-scoped registration token, and drops the agent into their compose file with that token. The agent's first poll binds it to the named project; the agent never creates a project implicitly.
- **Intents survive agent downtime.** If the agent is offline when an intent is enqueued, the row stays in `created` until the agent next polls. Intents that reach `dispatched` but don't report back are requeued by the reaper after the in-flight timeout (unchanged from today).

## Architecture

```
  Developer ─git push──▶ GitHub ─webhook──▶ Overwatcher Coordinator
                                                │
                                                │ 1. find all services where repo=R
                                                │ 2. filter by root_directory ∩ changed paths
                                                │ 3. group services by project
                                                │ 4. enqueue one intent per project
                                                ▼
                                         deploy_intents (Postgres)
                                                │
                    ┌───────────────────────────┼───────────────────────────┐
                    │ long-poll                 │ long-poll                 │ long-poll
                    ▼                           ▼                           ▼
           ┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
           │   Project A     │         │   Project B     │         │   Project C     │
           │  ┌───────────┐  │         │  ┌───────────┐  │         │  ┌───────────┐  │
           │  │   Agent   │  │         │  │   Agent   │  │         │  │   Agent   │  │
           │  │ (compose) │  │         │  │ (compose) │  │         │  │ (compose) │  │
           │  └─────┬─────┘  │         │  └─────┬─────┘  │         │  └─────┬─────┘  │
           │        │        │         │        │        │         │        │        │
           │   web, api,     │         │   frontend,     │         │   worker        │
           │   worker        │         │   backend       │         │                 │
           │ (3 services,    │         │ (2 services,    │         │ (1 service,     │
           │  same repo,     │         │  different      │         │  shared repo    │
           │  different      │         │  repos)         │         │  with Project A)│
           │  root dirs)     │         │                 │         │                 │
           └─────────────────┘         └─────────────────┘         └─────────────────┘
```

Example `service` rows (showing shared-repo and monorepo cases):

| project | name | repo | root_directory | image | tag |
|---|---|---|---|---|---|
| A | web | owner/monorepo | web/ | ghcr.io/owner/web | latest |
| A | api | owner/monorepo | api/ | ghcr.io/owner/api | latest |
| A | worker | owner/monorepo | worker/ | ghcr.io/owner/worker | latest |
| B | frontend | owner/frontend-repo | / | ghcr.io/owner/frontend | latest |
| B | backend | owner/backend-repo | / | ghcr.io/owner/backend | latest |
| C | worker | owner/monorepo | worker/ | ghcr.io/owner/worker | stable |

A push to `owner/monorepo` touching `web/src/App.tsx` triggers exactly one intent against project A, with `services_spec = [{web, …}]`. A push touching `worker/main.go` triggers two intents: one for project A (`worker`), one for project C (`worker`).

## Schema

```
 ┌──────────────────────────────────┐
 │             users                │
 ├──────────────────────────────────┤
 │ id            UUID PK            │
 │ email         VARCHAR(255) UQ    │
 │ name          VARCHAR(255)       │
 │ created_at    TIMESTAMPTZ        │
 │ updated_at    TIMESTAMPTZ        │
 └──────────────┬───────────────────┘
                │ 1:N
                ▼
 ┌──────────────────────────────────┐
 │            projects              │
 ├──────────────────────────────────┤
 │ id            UUID PK            │
 │ user_id       UUID FK → users (CASCADE) NOT NULL │
 │ name          VARCHAR(255)       │
 │ description   TEXT               │
 │ compose_file  VARCHAR(512)       │  ← path on the agent VM
 │ environment   VARCHAR(64)        │  ← prod/staging/etc.
 │ enabled       BOOLEAN            │
 │ created_at    TIMESTAMPTZ        │
 │ updated_at    TIMESTAMPTZ        │
 │                                  │
 │ UQ(user_id, name)                │
 └──────────────┬───────────────────┘
                │ 1:1                │ 1:N
                ▼                    ▼
 ┌──────────────────────┐   ┌──────────────────────────────────┐
 │       agents         │   │            services              │
 ├──────────────────────┤   ├──────────────────────────────────┤
 │ id         UUID PK   │   │ id              UUID PK          │
 │ project_id UUID FK UQ│   │ project_id      UUID FK → projects (CASCADE) │
 │ name       VARCHAR UQ│   │ name            VARCHAR(255)     │ ← compose service
 │ remote_ip  VARCHAR   │   │ repo            VARCHAR(512)     │ ← owner/name
 │ last_seen  TIMESTAMPTZ│  │ root_directory  VARCHAR(512)     │ ← "/" or "web/"
 │ created_at TIMESTAMPTZ│  │ branch          VARCHAR(255)     │ ← default "main"
 │ updated_at TIMESTAMPTZ│  │ image           VARCHAR(512)     │
 └──────────────────────┘   │ tag             VARCHAR(255)     │
                            │ position        INTEGER          │
                            │ created_at      TIMESTAMPTZ      │
                            │ updated_at      TIMESTAMPTZ      │
                            │                                  │
                            │ UQ(project_id, name)             │
                            │ IDX(repo)                        │
                            └──────────────────────────────────┘


 ┌──────────────────────────────────┐
 │     deploy_intents (kept)        │
 ├──────────────────────────────────┤
 │ id              UUID PK          │
 │ project_id      UUID    (nullable, for history after project deletion) │
 │ delivery_id     VARCHAR(255)     │
 │ repo            VARCHAR(512)     │  ← repo that triggered this intent
 │ git_ref         VARCHAR(255)     │
 │ sha             VARCHAR(64)      │
 │ environment     VARCHAR(64)      │
 │ deployment_id   BIGINT           │
 │ installation_id BIGINT           │
 │ status          VARCHAR(32)      │
 │ attempts        INTEGER          │
 │ services_spec   JSONB            │  ← snapshot: [{name,image,tag}, …]
 │ dispatched_at   TIMESTAMPTZ      │
 │ created_at      TIMESTAMPTZ      │
 │ updated_at      TIMESTAMPTZ      │
 │                                  │
 │ UQ(delivery_id, project_id)      │
 └──────────────────────────────────┘
```

### Notes on the schema

- **`users` is the tenant root.** Every project has a non-null `user_id`; deleting a user cascades through projects → services and agents. `deploy_intents` do not FK to users (they reach users transitively via `project_id`), so a user deletion preserves deployment history the same way a project deletion does today.
- **`projects.name` is unique *per user*, not globally.** The index is `UNIQUE(user_id, name)` so two users can each have a `staging` project. This also means the project-scoped registration token — not the name — is what the agent uses to identify which project to bind to.
- **`projects` owns the compose file.** It was previously on `agents`; moving it reflects that the compose file is a property of the deployment, not the runtime.
- **`agents.project_id UNIQUE`** enforces the 1:1. An agent registers by claiming a project (e.g. via a project-scoped token); if the project already has a live agent, the registration fails or rotates.
- **`services` replaces `deploy_mappings` + `deploy_mapping_services`.** The repo moves from mapping-level to service-level, which is the whole point of the redesign.
- **`services.root_directory`** is new. Defaults to `/` for single-service repos; required (non-empty) for monorepo services. Used to filter GitHub push events by `commit.modified ∪ added ∪ removed`.
- **`services.branch`** is new. The current code only deploys on `main`/`master`; per-service branch lets staging projects track `develop` without code changes.
- **Dedupe key is now `(delivery_id, project_id)`, replacing the old `(delivery_id, stack_index)`.** `stack_index` worked before because there was at most one mapping per `(repo, agent)`, so an integer ordinal was a stable identifier. With projects as the natural grouping, `project_id` is the correct key — and unlike an enumerated index, it doesn't depend on the order the webhook handler walks the result set, so a redelivered push can't bypass the constraint by producing rows in a different order. `project_id` is non-null at insert time; it only becomes NULL if the project is later deleted, and historical rows can't collide with live ones.
- **`deploy_intents` does not FK into `projects`.** Same rationale as today: it's an event log, and deleting a project shouldn't erase its deployment history.

## Webhook → intent flow (new)

1. Receive `push` webhook, verify signature.
2. Extract `repo`, `ref`, `sha`, and changed file paths. GitHub caps the push payload at 20 commits with a limited number of files per commit, so treat the payload as potentially truncated: if `commits` is empty, or its length is less than the `before..after` range, fall back to the Compare API (`GET /repos/{owner}/{repo}/compare/{before}...{after}`) for the authoritative changed-path list. If the Compare API also returns no file list (empty push, unreachable, etc.), treat the push as "touches everything" and include every service in every matched project — safe default, matches the pre-redesign behavior.
3. `SELECT … FROM services WHERE repo = $1 AND branch = $2` → candidate services.
4. For each candidate, keep it if `root_directory = '/'` OR any changed path starts with `root_directory`.
5. `GROUP BY project_id`. For each group: enqueue one intent with `services_spec = [{name,image,tag}, …]` for just those services. The dedupe key is `(delivery_id, project_id)` — redelivered webhooks collapse onto existing rows regardless of the order projects come back in.
6. Create a GitHub Deployment per intent (one commit can have several deployments — one per project affected). Existing status-reporting flow is unchanged.

The dispatch side (agent long-poll, runner, result reporting) is unchanged, except the runner now receives the project-level compose file path from the intent rather than looking it up from the agent's row.

## Migration path

Every existing `deploy_mapping` becomes one project under a single bootstrap user:

1. Create `users`, `projects`, `services` tables. Keep `agents`, `deploy_mappings`, `deploy_mapping_services` in place.
2. **Backfill**: insert a bootstrap user (e.g. email `admin@local`) to own all pre-existing deployments. For each `deploy_mapping` row, insert a `project` under the bootstrap user (name = `{repo}-{environment}` or similar), move `agents.compose_file` → `projects.compose_file`, set `agents.project_id`. Copy `deploy_mapping_services` rows to `services` with `repo = deploy_mappings.repo`, `root_directory = '/'`, `branch = 'main'`.
3. Cut the webhook handler over to the new query path.
4. Drop `deploy_mappings`, `deploy_mapping_services`, and the `compose_file` column on `agents`.

Steps 1–3 are reversible; step 4 is the point of no return and should land in its own migration after the new path has been running for a cycle.

## Out of scope (for this redesign)

- **Per-service independent deploys within one compose file.** Today's agent runs `docker compose up -d <service>` per service, which already gives us this — no change needed.
- **Multi-environment on one project.** A project has one `environment`. Staging vs prod = two projects. (Railway treats them as sibling environments within a project; we can add that later if it becomes painful, but for now "one project = one deployment" is simpler.)
- **Secrets / env var management.** Still lives in the compose file on the VM; Overwatcher does not manage application secrets (explicitly out of scope per `high-level-design.md`).
- **Agent self-update.** Unchanged open problem from the current roadmap.

