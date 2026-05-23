# Backend directory layout refactor

## Context

`services/overwatcher-backend/` houses two binaries today (`cmd/overwatcher`
and `cmd/agent`) and will house a third in the future (a Kubernetes-native
agent — see [agent-systemd.md](agent-systemd.md)). The current layout grew
organically around the coordinator and doesn't have a clean seam for the
agent variants. This document captures the shape we want before any
agent-related refactor lands.

## Current shape

```
services/overwatcher-backend/
├── cmd/
│   ├── overwatcher/    thin main (coordinator)
│   └── agent/          NOT thin — runner.go, poll.go, runner_test.go live here
└── internal/
    ├── api/http/dto/   ← agent imports deploy.go from here
    ├── service/
    │   ├── agent/      ← coordinator-side agent tracking (name clash!)
    │   ├── project/  auth/  dispatch/  webhook/  user/  intent/  eventlog/
    ├── db/  github/    coordinator-only
    └── util/
```

## Problems

1. **`cmd/agent/` is fat.** Runner retry logic, transient-error
   classification, the poll loop, and config all live under `cmd/` — which
   is conventionally a thin main package. `cmd/overwatcher/` already
   matches that convention; agent should too. Testable code belongs in
   `internal/`.
2. **Wire contract is misnamed.** `internal/api/http/dto/deploy.go` is
   the protocol between coordinator and agent — the one thing both
   binaries genuinely share. Calling it "HTTP DTOs" makes it look
   coordinator-internal.
3. **`internal/service/agent/` is not the agent.** It's the coordinator's
   tracker for connected agents (heartbeats, status). Future readers
   will trip on the name once a real `internal/agent/` exists.
4. **No seam for a second agent flavor.** The docker-compose runner is
   welded into `cmd/agent/`. The future k8s agent shares ~everything
   except the runner. The seam needs to exist before the second impl
   lands, not during.

## Proposed shape

```
internal/
  protocol/                ← deploy intent + agent wire types (shared)
  coordinator/             ← optional grouping; not done in v1
    api/http/{handler,middleware}
    service/{project,auth,dispatch,webhook,user,intent,agentregistry}
    db/  github/
  agent/                   ← shared agent code
    poll.go
    config.go
    runner.go              ← Runner interface (extract when 2nd impl lands)
    docker/runner.go       ← current compose runner moves here
    k8s/runner.go          ← future
cmd/
  overwatcher/main.go      ← unchanged
  agent/main.go            ← thin: wire docker runner + poll loop
  agent-k8s/main.go        ← future
```

## Concrete moves, smallest-first

1. **Extract `internal/protocol/`.** Move the agent-facing types out of
   `internal/api/http/dto/deploy.go`. Coordinator handler keeps a thin
   adapter if it wants. ~1 file move, mechanical.
2. **Move agent logic to `internal/agent/`.** `cmd/agent/{config,poll,
   runner,logger}.go` → `internal/agent/`. `cmd/agent/main.go` becomes
   ~30 lines that wires it up. This is also the natural place for the
   embedded-yaml-defaults change from the systemd plan.
3. **Rename `internal/service/agent/` → `internal/service/agentregistry/`**
   (or `agentstatus`). Pure rename, removes name clash before
   `internal/agent/` lands.
4. **Don't group coordinator packages under `internal/coordinator/` yet.**
   It's the right shape long-term but it's a big diff with no concrete
   payoff until the flat layout becomes confusing — which it isn't yet,
   because `internal/agent/` and `internal/protocol/` are clearly named.
5. **Don't extract the Runner interface yet.** Wait until the k8s runner
   actually starts. An interface for one impl is just ceremony.

## Tradeoffs

- **Bundle vs. split into separate Go modules.** Keep one module. Two
  modules means version skew between agent and coordinator becomes a real
  thing to maintain. One module + `internal/protocol/` gives a
  compile-time guarantee that both ends agree on the wire types.
- **Grouping coordinator dirs.** Tempting for cleanliness, but it touches
  every import path in the repo for zero behavior change. Worth doing
  only if a third binary actually makes the flat layout confusing.
- **Timing.** Moves 1–3 should land **before** the systemd-agent work,
  because that work changes `cmd/agent/`'s shape (embedded defaults, no
  YAML on disk). Easier to do that in `internal/agent/` than to refactor
  mid-feature.

## Out of scope

- Splitting into multiple Go modules.
- Grouping coordinator packages under `internal/coordinator/`.
- Extracting the Runner interface (deferred until a second impl exists).
- Any behavior change. This refactor is structure-only; all tests should
  pass with no logic edits.
