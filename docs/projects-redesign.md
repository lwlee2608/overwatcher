# Projects Redesign (Railway-style)

## Motivation

The current model is keyed by `(repo → agent)`: a `deploy_mapping` binds one repo to one agent, and services (image/tag pairs) hang off the mapping. That shape breaks down in two places:

1. **Monorepos with many services.** A repo containing `web/`, `api/`, and `worker/` has three CI pipelines publishing three images. Today they all share one mapping row, one list of services, and — critically — one set of triggering rules. There's no way to say "a push that only touches `web/` should redeploy only `web`."
2. **One deployable unit across repos.** A compose stack often stitches together services from several repos (e.g. `frontend-repo` + `backend-repo` running on the same VM). Today those are two separate mappings that happen to point at the same agent; there is no object representing "the thing deployed on that VM."

Railway solves both with a **Project** as the unit of deployment: one compose file, one agent, and N services — each service independently linked to a repo (and a path inside it). We want the same shape.

## Concepts

| Concept | What it is | Relationship |
|---|---|---|
| **Project** | The deployable unit. Owns exactly one compose file and exactly one agent. | 1 project — 1 agent, 1 project — N services |
| **Agent** | The process on a VM that runs `docker compose` for its project. Registers itself on first poll. | 1 agent — 1 project |
| **Service** | A compose service. Links to one GitHub repo (+ root directory) and one image:tag. | N services — 1 project. Many services can share a repo. |
| **DeployIntent** | Queue item: "project P, deploy services [s1, s2] at SHA X". Immutable event log. | N intents — 1 project |

Key invariants:

- **Project ↔ Agent is 1:1.** An agent can't serve two projects; a project can't have two agents. Enforced by `agents.project_id UNIQUE`.
- **A repo may appear in many services across many projects.** A webhook for `owner/repo` fans out to every service whose `repo = 'owner/repo'` and whose `root_directory` matches the push's changed paths.
- **Intents are per-project, not per-service.** A single webhook that affects services `web` and `api` in the same project produces one intent with two services in its spec, so compose is invoked once.

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
 │            projects              │
 ├──────────────────────────────────┤
 │ id            UUID PK            │
 │ name          VARCHAR(255) UQ    │
 │ description   TEXT               │
 │ compose_file  VARCHAR(512)       │  ← path on the agent VM
 │ environment   VARCHAR(64)        │  ← prod/staging/etc.
 │ enabled       BOOLEAN            │
 │ created_at    TIMESTAMPTZ        │
 │ updated_at    TIMESTAMPTZ        │
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
 │ stack_index     INTEGER          │  ← now = index within (delivery, project)
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
 │ UQ(delivery_id, stack_index)     │
 └──────────────────────────────────┘
```

### Notes on the schema

- **`projects` owns the compose file.** It was previously on `agents`; moving it reflects that the compose file is a property of the deployment, not the runtime.
- **`agents.project_id UNIQUE`** enforces the 1:1. An agent registers by claiming a project (e.g. via a project-scoped token); if the project already has a live agent, the registration fails or rotates.
- **`services` replaces `deploy_mappings` + `deploy_mapping_services`.** The repo moves from mapping-level to service-level, which is the whole point of the redesign.
- **`services.root_directory`** is new. Defaults to `/` for single-service repos; required (non-empty) for monorepo services. Used to filter GitHub push events by `commit.modified ∪ added ∪ removed`.
- **`services.branch`** is new. The current code only deploys on `main`/`master`; per-service branch lets staging projects track `develop` without code changes.
- **`deploy_intents.stack_index`** keeps its role as a uniqueness discriminator for fan-out, but is now scoped per-project rather than per-mapping. The `UNIQUE(delivery_id, stack_index)` still dedupes redelivered webhooks.
- **`deploy_intents` does not FK into `projects`.** Same rationale as today: it's an event log, and deleting a project shouldn't erase its deployment history.

## Webhook → intent flow (new)

1. Receive `push` webhook, verify signature.
2. Extract `repo`, `ref`, `sha`, and changed file paths.
3. `SELECT … FROM services WHERE repo = $1 AND branch = $2` → candidate services.
4. For each candidate, keep it if `root_directory = '/'` OR any changed path starts with `root_directory`.
5. `GROUP BY project_id`. For each group: enqueue one intent with `services_spec = [{name,image,tag}, …]` for just those services, and `stack_index = <incrementing index in the group order>`.
6. Create a GitHub Deployment per intent (one commit can have several deployments — one per project affected). Existing status-reporting flow is unchanged.

The dispatch side (agent long-poll, runner, result reporting) is unchanged, except the runner now receives the project-level compose file path from the intent rather than looking it up from the agent's row.

## Migration path

Every existing `deploy_mapping` becomes one project:

1. Create `projects`, `services` tables. Keep `agents`, `deploy_mappings`, `deploy_mapping_services` in place.
2. **Backfill**: for each `deploy_mapping` row, insert a `project` (name = `{repo}-{environment}` or similar), move `agents.compose_file` → `projects.compose_file`, set `agents.project_id`. Copy `deploy_mapping_services` rows to `services` with `repo = deploy_mappings.repo`, `root_directory = '/'`, `branch = 'main'`.
3. Cut the webhook handler over to the new query path.
4. Drop `deploy_mappings`, `deploy_mapping_services`, and the `compose_file` column on `agents`.

Steps 1–3 are reversible; step 4 is the point of no return and should land in its own migration after the new path has been running for a cycle.

## Out of scope (for this redesign)

- **Per-service independent deploys within one compose file.** Today's agent runs `docker compose up -d <service>` per service, which already gives us this — no change needed.
- **Multi-environment on one project.** A project has one `environment`. Staging vs prod = two projects. (Railway treats them as sibling environments within a project; we can add that later if it becomes painful, but for now "one project = one deployment" is simpler.)
- **Secrets / env var management.** Still lives in the compose file on the VM; Overwatcher does not manage application secrets (explicitly out of scope per `high-level-design.md`).
- **Agent self-update.** Unchanged open problem from the current roadmap.

## Open questions

1. **Project creation order vs agent registration.** If the agent auto-registers on first poll, does it create the project too, or must the project exist first? Recommendation: **project-first**. A user creates a project in the UI, gets a project-scoped registration token, drops the agent into their compose file with that token. The agent's first poll binds it to the named project.
2. **Changed-path detection for force-pushes / merges.** GitHub's push payload lists `commits[*].modified/added/removed`, but these can be empty for large pushes. Fallback: if the list is empty, deploy all services in the project (current behavior — safe default).
3. **What happens if the agent is offline when an intent is enqueued?** Today: the intent sits in `created`, gets picked up whenever the agent next polls. That behavior is preserved. The reaper still requeues `dispatched` intents past the in-flight timeout.
