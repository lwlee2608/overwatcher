# Projects Redesign — Remaining Work

Companion to [projects-redesign.md](./projects-redesign.md). All five steps are done.

## Done

- **Step 1** — Additive migration for `users`, `projects`, `services` (migration 0010). `agents.project_id` and `deploy_intents.project_id` added as nullable.
- **Step 1b** — sqlc queries for users/projects/services.
- **Step 2** — `user` and `project` service packages, HTTP handlers (`/api/v1/users`, `/api/v1/projects`, nested `/services`), DTOs, router wiring, and a 14-case system test suite.
- **Step 3** — Webhook cutover. `handlePush` matches by `(repo, branch)`, filters by `root_directory ∩ changed paths`, groups by project, and enqueues one intent per project. Intent dedup moved to partial unique `(delivery_id, project_id)`. `compose_file` is now a property of the project and rides on each intent; `X-Agent-Compose-File` header and `agent.compose_file` config removed. Migration 0011 backfills a bootstrap user + project per legacy mapping and binds agents.
- **Step 4** — Frontend. Users page (CRUD), Projects page (list grouped by owner + CRUD), Project detail page with inline services editor (reorderable rows, "Save services" posts the full list via `PUT /projects/:id/services`), Deployments table with a project column linking to the detail page.
- **Step 4b** — Agent binding. `PUT /api/v1/agents/:id/project` binds (or clears with empty `project_id`) the agent↔project link. `AgentStatusResponse` now carries `project_id`. Project detail page has an Agent panel with a selector and connection indicator; reassigning moves future deploys to the new agent.
- **Step 5** — Drop legacy. Migration 0012 drops `agents.compose_file`, `deploy_intents.stack_index` + its legacy unique, and the `deploy_mapping_services` / `deploy_mappings` tables. `internal/service/mapping`, its handler, DTO, routes, systemtest, and the frontend Mappings page are gone. `ServiceSpecDTO` moved into `dto/deploy.go`; `ServiceSpec` (frontend) moved into `types/deployment.ts`. `Down` is a no-op — drops are not reversible without backup restore.

Open questions (ongoing):
- Multi-service compose stacks across multiple repos — the UI should make it clear that services in one project can come from different repos.

## Follow-ups (not blocking)

- **Truncated-push fallback.** GitHub omits the commit file lists for pushes bigger than ~20 commits or ~3000 files. `changedPathsFromPush` currently returns an empty slice and `pathMatchesRoot` conservatively dispatches everything. Good enough for now, but a real fix is to call the Compare API (`/repos/{owner}/{repo}/compare/{before}...{after}`) and union `files[].filename` when `event.Commits` looks truncated.
- **Webhook system test for path filtering.** Exercise a monorepo push that touches `web/` and asserts only the web service in project A gets an intent (no noise for project B whose root is `api/`). Currently only unit-tested via `TestPathMatchesRoot`.
- **Per-project concurrency guard.** The in-flight-by-stack guard now keys on project name (via `intent.Stack = project.Name`). Same semantics, but worth a comment on why we kept the name instead of switching to project_id.
