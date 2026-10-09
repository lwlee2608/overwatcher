# owctl — CLI for the Overwatcher API
**The Job** — A Go CLI that wraps the Overwatcher REST API with API-key auth: manage projects, services and agents, including `agent install --ssh` (create agent → install on VM over SSH → wait for connect → bind).
**The Why** — AI agents and humans drive the API today with hand-rolled curl. The `overwatcher` skill (PR #67) carries fragile shell glue — bash/zsh quoting, non-persistent functions, token-in-variable handling, a multi-step VM install with manual cleanup — that an LLM must follow exactly from prose.
**The Guardrail** — No REST API changes required; coordinator and agent binaries behave the same; existing agent release assets keep their names so `install.sh` keeps working.
**Done means** — From a machine with only `owctl` and SSH access, `owctl agent install --ssh user@vm --project <name>` leaves a connected agent bound to the project, and the skill uses `owctl` instead of curl.

## Decisions
- **Interview depth** — key decisions only
- **Binary name** — `owctl`
- **v1 command scope** — core workflows: `project list|get|create|delete`, `service set`, `agent list|bind|install --ssh`. No `apply -f`, no users/deployments/events
- **Distribution** — release workflow also builds `owctl` for linux+darwin × amd64+arm64; coordinator serves `/cli.sh` installer (like `/install.sh`); `go install` works too
- **SSH implementation** — shell out to the system `ssh` binary (inherits ~/.ssh/config, ssh-agent, ProxyJump, known_hosts)
- **Credentials** — `owctl login` reads the key without echo and writes `~/.config/owctl/config.yaml` (0600); `OVERWATCHER_API_KEY` / `OVERWATCHER_URL` override it; URL defaults to `https://overwatcher-web-production.up.railway.app`
- **Default output format** — human table by default; `--json` prints the raw API JSON
- **Skill rewrite in this plan** — yes, final phase; PR #67 merges first as the curl-based stopgap
- **`service set` input** — `-f <file>` (YAML or JSON, `-` = stdin), same shape as the `PUT /projects/:id/services` body
- **Code location** — `cmd/owctl` in the backend Go module; reuses `internal/api/http/dto` types so request/response shapes can't drift `research`
- **Release pipeline** — `agent-release.yml` builds on `v*` tags via a `make release-agent` target; linux amd64/arm64 only today `research`
- **CLI framework** — cobra (subcommands, help, shell completion) instead of hand-rolled `flag` dispatch `agent`
- **Layering** — `internal/client` is a typed REST client (no CLI concerns); `cmd/owctl` holds commands, flags, output only `agent`
- **Project references** — commands accept a project name or ID; names resolve via `GET /projects` `agent`
- **Versioning** — `owctl` ships on the same `v*` tags as the agent; `/cli.sh` installs the coordinator's release tag so client and server match; `owctl version` prints both `agent`
- **`agent install` safety** — prints the SSH target and asks for confirmation unless `--yes`; preflight checks `overwatcher-agent` not already active and `docker compose` present; deletes the created agent if install or connect fails; agent token never printed; `--name` defaults to the SSH host; `--timeout` default 60s `agent`
- **Tests** — client and commands tested against the real gin router via `httptest` on the systemtest Postgres; `agent install` tested with a fake `ssh` script on `PATH` that simulates the VM `agent`
- **Phase count** — 5, sequential
- **Build mode** — Orchestrator; worker `velocirouter/gpt-6-astra` at `medium`, fresh reviewer `velocirouter/gpt-6-astra` at `high` each round
- **Current build checkpoint** — pause after Phase 2 merges; do not start Phase 3
- **Branching** — integration branch `owctl-integrate` (from `main`); each phase branches from it and merges back via PR; `owctl-integrate` merges to `main` after the final verify
- **Verification** — no per-phase Verify; one end-to-end verify after Phase 5, run as the Demo. Per-phase test tasks stay. Risk accepted: an early-phase bug surfaces late

## Progress
Phase 1 of 5 complete · 8/25 tasks (review/merge pending)

### Phase 1 — Log in and list projects from the terminal
A user with an API key can run `owctl login`, then `owctl project list|get` against the coordinator.
**Blocked by:** none
- [x] Add typed REST client with Bearer auth and API error mapping (internal/client)
- [x] Add project list/get and server version calls to the client (internal/client)
- [x] Add cobra root command with `--json`, `--url`, config file + env resolution (cmd/owctl)
- [x] Add `owctl login` storing the key without echo, file mode 0600 (cmd/owctl)
- [x] Add `owctl project list|get` with table and JSON output (cmd/owctl)
- [x] Add `owctl version` printing client and server versions (cmd/owctl)
- [x] Add `build-owctl` target (services/overwatcher-backend/Makefile)
- [x] Add owctl system test against the real router (systemtest/tests)
**Verify:** deferred — single end-to-end verify after Phase 5 (see Demo)

### Phase 2 — Create a project with services and bind an existing agent
A user can set up a whole project from the terminal when a free agent already exists.
**Blocked by:** 1
- [ ] Add project create/delete, replace services, agent list/bind to the client (internal/client)
- [ ] Add `owctl project create|delete` (cmd/owctl)
- [ ] Add `owctl service set <project> -f` reading YAML, JSON or stdin (cmd/owctl)
- [ ] Add `owctl agent list|bind` (cmd/owctl)
- [ ] Accept project name or ID in every project-scoped command (cmd/owctl)
- [ ] Extend the owctl system test (systemtest/tests)
**Verify:** deferred — single end-to-end verify after Phase 5 (see Demo)

### Phase 3 — Install an agent on a VM with one command
`owctl agent install --ssh user@vm --project <name>` leaves a connected, bound agent.
**Blocked by:** 2
- [ ] Add agent create/get/delete to the client (internal/client)
- [ ] Add `owctl agent install --ssh <target> [--project] [--name] [--yes] [--timeout]` per the install-safety decision (cmd/owctl)
- [ ] Add a fake `ssh` harness that simulates the VM and heartbeats with the received agent token (systemtest)
- [ ] Test success, VM already running an agent, failed install cleanup, and connect timeout (systemtest/tests)
**Verify:** deferred — single end-to-end verify after Phase 5 (see Demo)

### Phase 4 — Install owctl with one curl command
`curl <coordinator>/cli.sh | sh` installs the `owctl` matching the coordinator's release.
**Blocked by:** 3
- [ ] Add `release-owctl` target for linux/darwin × amd64/arm64 with SHA256SUMS (services/overwatcher-backend/Makefile)
- [ ] Publish owctl assets on `v*` tags (.github/workflows/agent-release.yml)
- [ ] Serve `/cli.sh` templated with the coordinator's release tag (internal/api/http/handler, router.go)
- [ ] Proxy `/cli.sh` on the web domain (services/overwatcher-frontend/nginx.conf)
- [ ] Document owctl install and login (README.md)
**Verify:** deferred — single end-to-end verify after Phase 5 (see Demo)

### Phase 5 — The overwatcher skill drives owctl instead of curl
An AI agent with the skill sets up a project, including the VM install, using only owctl commands.
**Blocked by:** 3, 4
- [ ] Rewrite the skill around owctl: install via `/cli.sh`, env-based auth, `--json` commands, `agent install --ssh` (skills/overwatcher/SKILL.md)
- [ ] Update the README API-access and skill sections (README.md)
**Verify:** deferred — single end-to-end verify after Phase 5 (see Demo)

## Demo
Agent-run end-to-end verify after Phase 5 — the only proof for every phase. On a local coordinator (scratch Postgres in Docker):
1. `make test` passes, including all owctl system tests.
2. `make release-owctl` produces 4 binaries + SHA256SUMS; the local `/cli.sh`, pointed at a file server over `dist/`, installs `owctl` into a temp dir. *(Phase 4)*
3. Create a user and API key; `owctl login` writes a 0600 config; `owctl version` shows client and server; a bad key exits non-zero with "invalid api key". *(Phase 1)*
4. Following SKILL.md command by command: `project create`, `service set -f services.yaml`, `agent install --ssh` against the fake `ssh` harness. *(Phases 2, 3, 5)*
5. `owctl project get --json` shows the services and a connected, bound agent; the agent token never appeared in owctl output. *(Phases 2, 3)*
6. A failing fake installer leaves no agent row; `project delete` then `project get` exits with "project not found". *(Phases 2, 3)*

Real SSH, sudo and systemd are not exercised — see rollout.

## Post-merge rollout (user runs)
1. Merge PR #67 (curl-based skill) before Phase 5 starts.
2. Tag a release (`vX.Y.Z`) so owctl assets exist on GitHub; Railway redeploys the coordinator so `/cli.sh` points at it.
3. On a spare VM: `owctl agent install --ssh <user>@<vm> --project <test-project>` — confirms real sudo/systemd install, then delete the test agent and project.
