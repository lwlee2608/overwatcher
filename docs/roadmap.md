# Roadmap

Five phases from the current state to the full architecture described in [high-level-design.md](high-level-design.md). Each phase is independently valuable and testable before moving on. Phases 2 and 3 can be merged into one sprint if the goal is to reach a working end-to-end slice as quickly as possible.

## Phase 1 — GitHub-side scaffolding *(done)*

The webhook path, GitHub App installation auth, signature verification, and "create a GitHub Deployment on push to main" already exist in `internal/service/`.

## Phase 2 — Deploy intent + repo→stack mapping *(done)*

Teach Overwatcher *what* to deploy *where*, without actually talking to a VM yet.

- ✅ Config (in `application.yml` or an Adder section) mapping: repo → target agent/stack → image + tag strategy.
- ✅ On push, produce an in-memory "deploy intent" (who, what, which commit).
- ✅ Mark the GitHub Deployment as `queued` when an intent is created.

## Phase 3 — Agent + coordinator↔agent transport *(done — MVP end-to-end)*

The smallest possible agent that makes a real push actually restart a real container.

- ✅ Minimal agent image, mounts `/var/run/docker.sock`.
- ✅ Long-poll transport: agent `GET /api/v1/deploy/next`, coordinator holds the request until an intent is ready (25s timeout).
- ✅ Agent runs `docker compose pull` + `docker compose up -d <service>` and reports result.
- ✅ Coordinator updates the GitHub Deployment status with success/failure.
- ✅ Shared secret bearer token auth.

## Phase 4 — Hardening

Everything that separates a demo from something trustworthy for real services. Too big for one PR — split into three sub-phases that can land independently.

### Phase 4a — Persistence + idempotency *(done)*

Replaced the in-memory `IntentStore` with a PostgreSQL-backed `DBStore` (originally planned as SQLite). Webhook redelivery dedup via `UNIQUE(delivery_id, stack_index)` with `ON CONFLICT DO NOTHING`, atomic dispatch via `FOR UPDATE SKIP LOCKED`. Intents survive coordinator restarts.

### Phase 4b — Dispatch reliability *(done)*

Builds on 4a's persistent in-flight state.

- ✅ In-flight timeout — Reaper requeues dispatched intents stuck past `in_flight_timeout` (default 10m).
- ✅ Max attempts — after `max_attempts` (default 3) retries, permanently failed with GitHub update.
- ✅ Concurrency guard — `TakeNext` skips stacks that already have a dispatched intent.
- ✅ Coordinator graceful shutdown — `signal.NotifyContext` + `server.Shutdown(ctx)` with configurable timeout.
- ✅ Agent report-on-shutdown with a fresh `context.Background()` so SIGTERM doesn't prevent result posting.

### Phase 4c — Per-agent auth + observability *(partial)*

- ❌ Per-agent tokens — still uses single global `AGENT_SHARED_SECRET`.
- ❌ Deploy log capture — agent captures `docker compose` output but only logs it locally; not streamed to GitHub.
- ❌ Prometheus metrics — not implemented.
- ✅ Structured logging — slog used throughout with context fields and status-based log levels.
- ❌ `BearerTokenAuth` test — not implemented.

## Phase 5 — Fleet features *(partial)*

Everything that only matters once there's more than one target.

- ❌ Multi-VM, multi-stack-per-VM, multi-repo routing — mapping supports one repo → multiple stacks, but no multi-VM dispatch strategy.
- ❌ Agent self-update strategy — the agent can't `pull + restart` its own container, so this likely needs a tiny host-level systemd unit or a watchdog sidecar.
- ✅ Agent heartbeats and health view — `Tracker` records implicit heartbeats from agent poll requests via middleware; `GET /api/v1/agents` exposes connection status; frontend dashboard shows agent status with real-time polling.
- ❌ A CLI or small web view to inspect queued / running / past deploys — only agent status view exists, no deploy history.

---

**Framing:** treat Phase 3 as the MVP goal. Everything after it is "earn the right to run this unattended." Phases 4 and 5 can reorder based on which pain hits first — solo use on one VM means Phase 5 can wait; running it for other people means Phase 4 becomes urgent.
