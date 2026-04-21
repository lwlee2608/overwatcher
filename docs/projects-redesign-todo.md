# Projects Redesign — Remaining Work

Companion to [projects-redesign.md](./projects-redesign.md). Steps 1–4 are done; this file tracks what's left before the redesign is complete.

## Done

- **Step 1** — Additive migration for `users`, `projects`, `services` (migration 0010). `agents.project_id` and `deploy_intents.project_id` added as nullable.
- **Step 1b** — sqlc queries for users/projects/services.
- **Step 2** — `user` and `project` service packages, HTTP handlers (`/api/v1/users`, `/api/v1/projects`, nested `/services`), DTOs, router wiring, and a 14-case system test suite.
- **Step 3** — Webhook cutover. `handlePush` matches by `(repo, branch)`, filters by `root_directory ∩ changed paths`, groups by project, and enqueues one intent per project. Intent dedup moved to partial unique `(delivery_id, project_id)`. `compose_file` is now a property of the project and rides on each intent; `X-Agent-Compose-File` header and `agent.compose_file` config removed. Migration 0011 backfills a bootstrap user + project per legacy mapping and binds agents.
- **Step 4** — Frontend. Users page (CRUD), Projects page (list grouped by owner + CRUD), Project detail page with inline services editor (reorderable rows, "Save services" posts the full list via `PUT /projects/:id/services`), Deployments table with a project column linking to the detail page. Mappings page left in place until parity is confirmed.

Open questions (deferred — Mappings page still available as fallback):
- Multi-service compose stacks across multiple repos — the UI should make it clear that services in one project can come from different repos.
- Where agent binding lives: on the Agents page or on the Project edit page? Doc says Project↔Agent is 1:1 — neither edge currently exposes it; the DB has `agents.project_id` but no UI surface yet. Pick one side before Step 5 or agents will be orphaned.

## Step 5 — Drop legacy

Only after Step 4 is live and deploys are flowing through the new path for long enough that we're confident.

Schema:
- Drop `agents.compose_file` column.
- Drop `deploy_intents.stack_index` and its `(delivery_id, stack_index)` unique constraint.
- Drop `deploy_mapping_services` table.
- Drop `deploy_mappings` table (cascades from the above).

Code:
- Delete `internal/service/mapping` package.
- Delete `internal/api/http/handler/mapping.go` and `internal/api/http/dto/mapping.go`.
- Remove `MappingService` from `Services` struct and router.
- Remove `/api/v1/mappings` routes.
- Delete `systemtest/tests/mappings.go` and the `Mappings` sub-test in `main_test.go`.
- Drop the Mappings page + route from the frontend.
- Regenerate sqlc (the mapping queries and the `stack_index` / `compose_file` columns disappear from generated models).

Risks:
- Migration that drops `deploy_mappings` must run *after* every environment is on the new webhook path. Worth double-checking the backfill ran cleanly on each deployment.
- `deploy_intents.stack_index` is NOT NULL today — dropping it is safe, but any rolling deploy where the coordinator still writes `stack_index` would break. Do this on a clean version bump, not mid-rollout.

## Follow-ups (not blocking)

- **Truncated-push fallback.** GitHub omits the commit file lists for pushes bigger than ~20 commits or ~3000 files. `changedPathsFromPush` currently returns an empty slice and `pathMatchesRoot` conservatively dispatches everything. Good enough for now, but a real fix is to call the Compare API (`/repos/{owner}/{repo}/compare/{before}...{after}`) and union `files[].filename` when `event.Commits` looks truncated.
- **Webhook system test for path filtering.** Exercise a monorepo push that touches `web/` and asserts only the web service in project A gets an intent (no noise for project B whose root is `api/`). Currently only unit-tested via `TestPathMatchesRoot`.
- **Per-project concurrency guard.** The in-flight-by-stack guard now keys on project name (via `intent.Stack = project.Name`). Same semantics, but worth a comment on why we kept the name instead of switching to project_id.
