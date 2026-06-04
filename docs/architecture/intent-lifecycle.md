# Deploy Intent Lifecycle

A **deploy intent** is the unit of work that flows from a GitHub event to a `docker compose up -d` on a VM. It's a row in `deploy_intents`, owned and mutated by the coordinator. Agents never write state — they pull work and post a result.

For the wire contract between coordinator and agent, see [agent-protocol.md](agent-protocol.md). This document focuses on the state machine and who flips each transition.

## States

| Status | Meaning |
|---|---|
| `created` | Persisted from a webhook, waiting for an agent to claim it |
| `dispatched` | An agent has claimed it and is running the deploy |
| `succeeded` | Agent reported success |
| `failed` | Agent reported failure (terminal — no auto-retry) |
| `permanently_failed` | Reaper exhausted `max_attempts` without a result |

Defined in `internal/service/intent/intent.go`.

## State diagram

```
                       webhook
                          │
                          ▼
                    ┌───────────┐
                    │  created  │◀──────────────┐
                    └─────┬─────┘               │
                          │                     │ reaper:
              agent poll  │  TakeNext           │ timeout AND
              (atomic):   │  attempts++         │ attempts < max
              status,     │  dispatched_at=now  │
              dispatched_at, attempts           │
                          ▼                     │
                    ┌───────────┐               │
                    │dispatched │───────────────┘
                    └─────┬─────┘
                          │
        ┌─────────────────┼──────────────────┐
        │                 │                  │
   agent POST         agent POST         reaper:
   result=success     result=failure     timeout AND
        │                 │              attempts >= max
        ▼                 ▼                  │
  ┌───────────┐    ┌───────────┐             ▼
  │ succeeded │    │  failed   │      ┌────────────────────┐
  └───────────┘    └───────────┘      │ permanently_failed │
                                      └────────────────────┘
```

## Who triggers each transition

Every transition is a coordinator-side DB write. The agent's only inputs are "I'm here, give me work" and "here's the result of work N".

| Transition | Trigger | Code |
|---|---|---|
| → `created` | `push` or `workflow_run` webhook matches an enabled project | `intent.DBStore.Enqueue` (called by `webhook.Service`) |
| `created` → `dispatched` | Agent long-poll claims the row | `intent.DBStore.TakeNext` (CTE: `SELECT … FOR UPDATE SKIP LOCKED` → `UPDATE … RETURNING`) |
| `dispatched` → `succeeded` / `failed` | Agent POSTs `/result` | `dispatch.Service.Report` → `intent.DBStore.Complete` |
| `dispatched` → `created` | Reaper sees `dispatched_at` older than `dispatch_timeout`, attempts under cap | `intent.DBStore.SweepTimedOut` (`RequeueTimedOutIntents`) — `attempts` is **not** reset |
| `dispatched` → `permanently_failed` | Same sweep, attempts at or above cap | `intent.DBStore.SweepTimedOut` (`FailTimedOutIntents`) |

The agent's `POST /result` carries `{"state": "success" \| "failure"}` — a `failure` state lands at `failed`, not back at `created`. **No auto-retry on application-level failure.** A fresh intent is created by the next git push, workflow rerun, or manual redeploy. Only *no answer at all* (the reaper case) requeues the same row.

## Why state lives on the coordinator

- **Agents are stateless.** They store no per-project config and survive restarts mid-deploy by simply forgetting. The DB row is the only durable trace.
- **At-least-once delivery.** `TakeNext` selects its candidate row with `FOR UPDATE SKIP LOCKED` and updates it in the same CTE — two agents polling the same instant can't both grab the same row. A crashed agent's intent gets requeued by the reaper, not by a peer.
- **GitHub Deployments is updated best-effort.** `dispatch.Service.Report` writes the DB row first, then pushes the outcome to GitHub. If the GitHub call fails it's logged and swallowed — the `deploy_intents` row is authoritative, the Deployments API is a side-effect mirror. There is no separate reconciler, so a failed push leaves GitHub stale until the next event.

## The reaper

Background goroutine in `dispatch.Reaper` (`reaper.go`). On every tick (default 1m, see `sweep_interval` below) it runs `SweepTimedOut` which does two SQL passes:

```
  rows where status='dispatched' AND dispatched_at < now - timeout
       │
       ├── attempts < max_attempts ──► status='created', notify pollers   (retry)
       │
       └── attempts >= max_attempts ─► status='permanently_failed'        (give up)
                                       │
                                       └─► coordinator pushes 'failure'
                                           to GitHub Deployments
```

Defaults live under `dispatch:` in `application.yml`:

| Key | Default | Meaning |
|---|---|---|
| `in_flight_timeout` | `10m` | how long a row may sit `dispatched` before it's swept |
| `max_attempts` | `3` | total tries before `permanently_failed` |
| `sweep_interval` | `1m` | reaper tick |

A successful report from the agent *after* the reaper has already requeued is harmless — the row's status is `created` (or `permanently_failed`), and `Complete` filters on `status='dispatched'`, so the late POST is a no-op and returns 404 to the agent.

### Why 10 minutes

10 minutes is calibrated to "longer than any realistic `docker pull` + `compose up -d` we expect to see, short enough that a crashed agent's work recovers quickly." For typical compose stacks the deploy finishes in seconds to a couple of minutes, so 10m is ~5× headroom over the common case.

It is **not** sized for pathological cases (multi-gigabyte images on residential upload speeds). If you run those, raise `in_flight_timeout` per environment. The cost of leaving it too short is a wasted duplicate deploy after a late `/result` 404s; the cost of leaving it too long is a crashed agent's intent sitting `dispatched` for hours before retry. We err on "recover fast" — duplicate deploys are recoverable, hangs look like the system is broken.

### When attempts are exhausted

`attempts` is the count of times an agent has claimed the row via `TakeNext` — it is **not** incremented by the reaper. With `max_attempts=3`, the timeline is: claim 1 times out → requeue, claim 2 times out → requeue, claim 3 times out → `permanently_failed`. That's `max_attempts - 1` rescues, then the last dispatch flips to terminal.

When the third dispatch times out, the reaper gives up. In one sweep it:

1. Sets the row to `permanently_failed` (`FailTimedOutIntents`).
2. Pushes `state=failure` to the GitHub Deployments API with a `"timed out after N attempts"` description, so the commit/PR shows red.
3. Logs the event.

`permanently_failed` is terminal — `TakeNext` only sees `created`, so no agent picks the row up again. A late `/result` from the original (presumed-dead) agent is a no-op for the same reason `Complete` filters on `status='dispatched'`. To redeploy, the user pushes again, re-runs the workflow, or uses the dashboard's **Redeploy** action. Redeploy clones the original intent into a fresh row with `attempts=0`; it does not mutate the terminal row.

Time to `permanently_failed` is **only bounded if an agent keeps reclaiming the row** after each requeue. In the lucky case — agent comes back up between sweeps and claims immediately — it's roughly `max_attempts × (in_flight_timeout + sweep_interval)` = `3 × (10m + 1m)` ≈ **33 minutes**.

If the agent stays down (process crashed, host offline, network partitioned), the reaper requeues once and then the intent sits in `created` forever — `attempts` doesn't move without a `TakeNext` claim, so `FailTimedOutIntents` never fires. There is no wall-clock cap. Monitoring the `agents` table's last-seen heartbeat is the intended way to notice this; see `agentregistry`.

## Concurrency model

```
  one project ── 1:1 ── one agent
                          │
                          ▼
                    serial poll loop
                    (one deploy at a time)
```

Per `internal/agent/poll.go`: the agent's poller is single-threaded by design — fetch one, run it, report, loop. Multiple intents queued for the same agent serialize naturally; the next `TakeNext` happens only after `/result` lands. The single-threaded poller is also why the reaper exists at all: if the agent process dies between `TakeNext` and `/result`, nothing else on that agent will ever finish the row.

Across projects there is no contention: `TakeNext` filters by the agent's bound project (token → resolved agent → `agents.project_id`), so agent A polling never returns project B's intent.

### Per-stack serialization

`TakeNext` carries a second guard beyond the project filter:

```sql
AND di.stack NOT IN (
    SELECT DISTINCT stack FROM deploy_intents WHERE status = 'dispatched'
)
```

A given `stack` can have **at most one `dispatched` intent at a time across the entire system**. Today the schema already enforces one-agent-per-project (`UNIQUE INDEX idx_agents_project_id ON agents(project_id) WHERE project_id IS NOT NULL`), so within a single project this guard is redundant with the poller's serial loop. It earns its keep if that 1:1 constraint is ever relaxed (multiple agents per project, shared stacks across projects) — the guard ensures two `docker compose up -d` runs can never race on the same stack directory regardless of how many pollers exist. A flurry of pushes to the same stack queues serially: the next intent is invisible to every poller until the in-flight one reaches a terminal state or the reaper requeues it.

## What doesn't have its own state

- **GitHub Deployment status** is a side effect, not a state. The coordinator updates it from three places, all best-effort: `dispatch.Service.Next` pushes `in_progress` when an intent is handed to an agent, `Report` pushes `success`/`failure` when the agent reports back, and the reaper pushes `failure` on the permanent-fail path. If any GitHub API call fails, we log and continue — the `deploy_intents` row is still authoritative locally.
- **Per-attempt history** is collapsed onto the row (`attempts`, `dispatched_at`). We don't keep an attempt log; if you need forensics, the logs and `event_logs` table cover what the row doesn't.
