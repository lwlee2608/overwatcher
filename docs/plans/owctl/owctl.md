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
- **Skill rewrite in this plan** — yes, final phase; PR #67's curl-based skill changes must be present on the integration branch before Phase 5 (already merged there; merging #67 to `main` separately is not required)
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
- **Current build checkpoint** — Phase 5 authorized; pause after it merges, before starting the Demo or opening the final PR
- **Branching** — integration branch `owctl-integrate` (from `main`); each phase branches from it and merges back via PR; `owctl-integrate` merges to `main` after the final verify
- **Verification** — no per-phase Verify; one end-to-end verify after Phase 5, run as the Demo. Per-phase test tasks stay. Risk accepted: an early-phase bug surfaces late

## Progress
Phases 1–4 of 5 merged · 25/25 tasks implemented · **Phase 5 awaiting review; pause before the Demo.**
- Phase 1: PR #68; two review rounds, terminal-interrupt cleanup fixed, final review clean.
- Phase 2: PR #69; project create/delete, YAML/JSON/stdin service replacement, agent list/bind, and project name/ID resolution. Two review rounds; date-like YAML scalar corruption fixed, final review clean.
- Both phases: `make build build-agent build-owctl`, `make test` (real-router system tests with disposable Postgres), `go vet ./...`, `git diff --check`, and CI passed after fixes. No findings skipped.
- Phase 3 implementation: typed agent create/get/delete and SSH install with confirmation, preflight, stdin-only token transport, bounded install/connect wait, bind-on-connect, and independent failure cleanup. Fake SSH executes the remote shell with simulated VM commands and real-router/Postgres heartbeats; success, active-agent refusal, missing Compose, installer failure, timeout, and declined confirmation covered. `make build build-agent build-owctl`, `make test`, `go vet ./...`, and `git diff --check` passed using disposable testcontainers Postgres. Initial test-fixture issues (unbound heartbeat returns 412; bound agents must unbind before deletion) corrected. No deployed hosts used; real SSH/sudo/systemd remain rollout-only. Failed installs revoke the created agent but may leave remote files/service requiring operator cleanup.
- Phase 3 review round 1: fixed all three findings. Ambiguous bind failures now unbind before deleting the created agent; confirmation exits on SIGINT/SIGTERM; project ownership is checked before SSH or agent creation. Regression coverage injects a failed response after a real committed binding and a member-role response, and signals subprocesses blocked at confirmation. `make build build-agent build-owctl`, `make test` (disposable Postgres), `go vet ./...`, and `git diff --check` passed after fixes. No findings skipped.
- Phase 3 review round 2: scoped signal interception to `agent install`, restoring normal termination for unrelated commands without changing login's terminal-restoration handler. Added subprocess regressions for SIGINT/SIGTERM while `service set -f -` waits on an open input stream; install-confirmation and login signal tests remain passing. `make build build-agent build-owctl`, `make test` (disposable Postgres), `go vet ./...`, and `git diff --check` passed. No findings skipped.
- Phase 3: PR #70 merged with a merge commit after three review rounds; round 3 clean. All four findings fixed, none skipped. Checks passed after the last fix and CI passed.
- Phase 4 implementation: added four Linux/macOS owctl release binaries and shared SHA256SUMS without removing agent assets; `v*` publishing stamps the tag version. Added release-templated `/cli.sh`, checksum verification, user-local installation, web-domain proxy, and README install/login instructions. `make build build-agent build-owctl`, `make test` (disposable testcontainers Postgres), `go vet ./...`, `make release-agent release-owctl COMMIT_SHA=`, all six artifact checksums/platform formats, POSIX shell syntax and handler tests, disposable nginx configuration validation, and `git diff --check` passed. Test containers were removed; no credentials, deployed hosts, or shared deployments used. Release publishing and the installer end-to-end Demo remain unrun. No Phase 5 tasks started.
- Phase 4 review round 1: fixed the release/coordinator version consistency finding. Release publishing now requires the tag to match committed `VERSION`, and binaries retain the same VERSION-based stamping as coordinator builds. Rollout now requires committing the intended version before tagging and deploying that version after assets publish. The exact workflow guard passed with a matching tag and rejected a mismatching tag in a disposable directory. `make build build-agent build-owctl`, `make test` (disposable Postgres), `go vet ./...`, `make release-agent release-owctl COMMIT_SHA=`, all six artifact checksums, and `git diff --check` passed. No findings skipped; no version bump, tag, release, deployment, or Phase 5 work performed.
- Phase 4 review round 2: corrected the CLI README navigation to **user menu → API keys**. The previous version consistency fix was confirmed; no other actionable defects found. `make build build-agent build-owctl`, `make test` (disposable Postgres), `go vet ./...`, and `git diff --check` passed after the documentation correction. No findings skipped; Phase 5 sections remain unchanged.
- Phase 4: PR #71 merged with a merge commit after three review rounds; round 3 clean. Both findings fixed, none skipped. Checks passed after the last fix and CI passed. Stopped at the requested checkpoint; no final PR to `main` opened.
- Phase 5 implementation: rewrote the Overwatcher skill and README API-access/skill sections around `/cli.sh`, environment authentication, `--json`, service replacement, and `agent install --ssh`. Checked syntax and safety claims against the command/client/installer implementation and local CLI help, including binding displacement, token transport versus VM persistence, and best-effort cleanup. `make build build-agent build-owctl`, `make test` (disposable testcontainers Postgres), `go vet ./...`, and `git diff --check` passed from the backend module as applicable. An initial make invocation at the repository root failed because it has no build target; rerunning in the backend module passed. Test containers were removed; no credential files inspected, deployed hosts, shared databases, or admin login used. Documentation-only change; no CLI scope expansion. Functional verification remains deferred; the Demo was not run.
- Phase 5 review round 1: corrected the installation-recovery link and both matching README references to `docs/architecture/agent-systemd.md`. All three targets resolve. `make build build-agent build-owctl`, `make test` (disposable testcontainers Postgres; system tests reran, other packages cached), `go vet ./...`, and `git diff --check` passed after the fix. No findings skipped; no Demo, deployed hosts, shared databases, credential inspection, or admin login used.
- End-to-end verification remains deferred to the Phase 5 Demo. Work is on `owctl-integrate`; `main` is unchanged.

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
- [x] Add project create/delete, replace services, agent list/bind to the client (internal/client)
- [x] Add `owctl project create|delete` (cmd/owctl)
- [x] Add `owctl service set <project> -f` reading YAML, JSON or stdin (cmd/owctl)
- [x] Add `owctl agent list|bind` (cmd/owctl)
- [x] Accept project name or ID in every project-scoped command (cmd/owctl)
- [x] Extend the owctl system test (systemtest/tests)
**Verify:** deferred — single end-to-end verify after Phase 5 (see Demo)

### Phase 3 — Install an agent on a VM with one command
`owctl agent install --ssh user@vm --project <name>` leaves a connected, bound agent.
**Blocked by:** 2
- [x] Add agent create/get/delete to the client (internal/client)
- [x] Add `owctl agent install --ssh <target> [--project] [--name] [--yes] [--timeout]` per the install-safety decision (cmd/owctl)
- [x] Add a fake `ssh` harness that simulates the VM and heartbeats with the received agent token (systemtest)
- [x] Test success, VM already running an agent, failed install cleanup, and connect timeout (systemtest/tests)
**Verify:** deferred — single end-to-end verify after Phase 5 (see Demo)

### Phase 4 — Install owctl with one curl command
`curl <coordinator>/cli.sh | sh` installs the `owctl` matching the coordinator's release.
**Blocked by:** 3
- [x] Add `release-owctl` target for linux/darwin × amd64/arm64 with SHA256SUMS (services/overwatcher-backend/Makefile)
- [x] Publish owctl assets on `v*` tags (.github/workflows/agent-release.yml)
- [x] Serve `/cli.sh` templated with the coordinator's release tag (internal/api/http/handler, router.go)
- [x] Proxy `/cli.sh` on the web domain (services/overwatcher-frontend/nginx.conf)
- [x] Document owctl install and login (README.md)
**Verify:** deferred — single end-to-end verify after Phase 5 (see Demo)

### Phase 5 — The overwatcher skill drives owctl instead of curl
An AI agent with the skill sets up a project, including the VM install, using only owctl commands.
**Blocked by:** 3, 4
- [x] Rewrite the skill around owctl: install via `/cli.sh`, env-based auth, `--json` commands, `agent install --ssh` (skills/overwatcher/SKILL.md)
- [x] Update the README API-access and skill sections (README.md)
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
1. PR #67's curl-based skill changes must be present on `owctl-integrate` before Phase 5; satisfied by merge commit `cac847e`. A separate merge of #67 to `main` is not required.
2. Commit `services/overwatcher-backend/VERSION` as the intended release (`vX.Y.Z`) before tagging that commit with the identical tag. The release workflow rejects a tag/VERSION mismatch. After its owctl assets are published, deploy the coordinator from that same version so `/cli.sh` points at the published release.
3. On a spare VM: `owctl agent install --ssh <user>@<vm> --project <test-project>` — confirms real sudo/systemd install, then delete the test agent and project.
