# Roadmap

Five phases from the current state to the full architecture described in [high-level-design.md](high-level-design.md). Each phase is independently valuable and testable before moving on. Phases 2 and 3 can be merged into one sprint if the goal is to reach a working end-to-end slice as quickly as possible.

## Phase 1 — GitHub-side scaffolding *(largely done)*

The webhook path, GitHub App installation auth, signature verification, and "create a GitHub Deployment on push to main" already exist in `internal/service/`. Nothing on the VM happens yet, but the GitHub-facing seam is real.

## Phase 2 — Deploy intent + repo→stack mapping

Teach Overwatcher *what* to deploy *where*, without actually talking to a VM yet.

- Config (in `application.yml` or an Adder section) mapping: repo → target agent/stack → image + tag strategy.
- On push, produce an in-memory "deploy intent" (who, what, which commit).
- Mark the GitHub Deployment as `queued` when an intent is created.
- No agent yet — intents just sit in a queue and log. This phase nails down the data shape without the transport.

## Phase 3 — Agent + coordinator↔agent transport (MVP end-to-end)

The smallest possible agent that makes a real push actually restart a real container.

- Minimal agent image, mounts `/var/run/docker.sock`.
- Pick the transport — long-poll is probably the right default: agent `GET /v1/deploy/next`, coordinator holds the request until an intent is ready.
- Agent runs `docker compose pull` + `docker compose up -d <service>` and reports result.
- Coordinator updates the GitHub Deployment status with success/failure.
- Scope: one repo, one VM, one stack, happy path only. No auth beyond a shared secret.

**This is the milestone where the project becomes useful.** If the goal is to reach a working end-to-end slice quickly, phases 2 and 3 can be merged.

## Phase 4 — Hardening

Everything that separates a demo from something trustworthy for real services.

- Proper agent auth (per-agent token, or mTLS).
- Deploy log capture — agent streams stdout back to coordinator, coordinator attaches it to the GitHub Deployment.
- Retries, timeouts, and cancellation.
- Concurrency guard: don't start a second deploy to the same stack while one is in flight.
- Persistent intent storage (sqlite?) so a coordinator restart doesn't drop queued deploys.
- Structured logging and basic metrics.

## Phase 5 — Fleet features

Everything that only matters once there's more than one target.

- Multi-VM, multi-stack-per-VM, multi-repo routing.
- Agent self-update strategy — the agent can't `pull + restart` its own container, so this likely needs a tiny host-level systemd unit or a watchdog sidecar.
- Agent heartbeats and "is stack Y currently reachable?" health view.
- A CLI or small web view to inspect queued / running / past deploys.

---

**Framing:** treat Phase 3 as the MVP goal. Everything after it is "earn the right to run this unattended." Phases 4 and 5 can reorder based on which pain hits first — solo use on one VM means Phase 5 can wait; running it for other people means Phase 4 becomes urgent.
