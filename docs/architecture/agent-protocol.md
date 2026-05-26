# Coordinator ↔ Agent Protocol

Two HTTP endpoints, the agent always initiates, long-poll for liveness.

## Who calls whom

The agent is always the HTTP client. It opens outbound TCP to the coordinator and never accepts inbound connections — so a VM behind NAT or a closed firewall works fine.

```
  coordinator                                  agent
  ┌─────────────┐                          ┌─────────────┐
  │             │  GET /deploy/next        │             │
  │             │ ◄─────────────────────── │             │
  │             │  200 + DeployIntent      │             │
  │             │      OR 204 No Content   │             │
  │             │ ───────────────────────► │             │
  │             │                          │             │
  │             │  POST /deploy/{id}/result│             │
  │             │ ◄─────────────────────── │             │
  │             │  204 No Content          │             │
  │             │ ───────────────────────► │             │
  └─────────────┘                          └─────────────┘
```

## The endpoints

**`GET /api/v1/deploy/next`** — long-poll for work. The coordinator parks the request for up to 25s (`LongPollTimeout` in `handler/deploy.go`). If an intent arrives in that window, it replies `200` with the body. If the timer fires first, it replies `204` and the agent immediately re-polls.

The agent's HTTP client timeout is 30s (`agent.poll_timeout`). The 5s gap is deliberate — the client must outlast the server's hold-open window, or it disconnects right before the server would have replied.

**`POST /api/v1/deploy/{id}/result`** — report the outcome after running `docker compose`. Body is `{"state": "success" | "failure", "error": "..."}`. The coordinator responds `204`. Unknown intent IDs return `404`; failed deploys don't auto-retry — the next git push creates a new intent.

Both endpoints authenticate with `Authorization: Bearer <AGENT_SHARED_SECRET>` and identify the agent with `X-Agent-Name: <name>`. Two optional headers report the agent's deployment surface: `X-Agent-Type` (`docker` or `systemd`, auto-detected from `/.dockerenv`) and `X-Agent-Version` (the agent's build version). The coordinator stores these on the agent row so the dashboard can render a badge per agent; empty values leave the stored value intact.

### Request headers

| Header | Required | Value |
|---|---|---|
| `Authorization` | yes | `Bearer <AGENT_SHARED_SECRET>` |
| `X-Agent-Name` | yes | agent name, e.g. `medtutor` |
| `X-Agent-Type` | optional | `docker` or `systemd` (auto-detected from `/.dockerenv`) |
| `X-Agent-Version` | optional | agent build version |
| `Content-Type` | on `/result` | `application/json` |

`/deploy/next` additionally requires the agent to be **bound to a project**. An unbound agent gets `412 Precondition Failed` — the binding is the project↔agent 1:1 row, set from the project's Agent panel in the UI. A `X-Agent-Name` that's never been seen returns `404` ("agent not registered"); the agent registers itself implicitly on its first authenticated poll via the heartbeat middleware.

## A full poll cycle

```
  t=0s    agent  ── GET /deploy/next ──►  coordinator   [parks request]

          ... no webhook arrives ...

  t=25s   agent  ◄── 204 No Content ───
  t=25s   agent  ── GET /deploy/next ──►  coordinator   [parks again]

  t=32s   webhook fires → intent persisted → parked poll woken
  t=32s   agent  ◄── 200 + DeployIntent ──

          docker compose -f <compose_file> pull
          docker compose -f <compose_file> up -d

  t=40s   agent  ── POST /deploy/{id}/result ──►   {"state":"success"}
  t=40s   agent  ── GET /deploy/next ──────────►   [back to waiting]
```

So the agent is in an infinite loop, but most iterations are one TCP connection parked for 25s. Cheap.

## The wire types

Defined in `internal/protocol/deploy.go` and imported by both coordinator and agent from the same Go module — a field rename breaks both compiles until they agree.

The intent the agent receives:

```go
type DeployIntentResponse struct {
    ID           string           `json:"id"`
    CreatedAt    time.Time        `json:"created_at"`
    DeliveryID   string           `json:"delivery_id"`
    ProjectID    string           `json:"project_id"`
    ComposeFile  string           `json:"compose_file"`   // absolute path on the VM
    Repo, Ref, SHA, Stack string
    Services     []ServiceSpecDTO `json:"services"`
    Environment  string           `json:"environment"`
    DeploymentID int64            `json:"deployment_id"`  // GitHub Deployments API id
}
```

`ServiceSpecDTO` is `{Name, Image, Tag}` per service; an empty `Name` means "apply to every service in the compose file."

## Where the compose file path comes from

The agent is stateless — it stores nothing per project. The coordinator reads `projects.compose_file` from the DB at enqueue time and stamps it onto the intent:

```
  DB (projects.compose_file)  ──►  intent  ──►  docker compose -f <path>
       set once in the UI         per-deploy       every deploy
```

The older agent design kept a `stacks: { name → path }` map in agent YAML and the intent only carried a stack name. Adding a project meant SSHing in to edit YAML and restart the agent, and the YAML could drift out of sync with the DB. Putting the path on the intent removes both problems — one agent serves many projects without ever being reconfigured.

Trade-off: the coordinator now "knows" filesystem paths on agent VMs, which is a mild layering smell. But the path is just an opaque string the agent passes to `docker compose -f`, and the alternative was worse.

## Status codes

| Code | When | Agent reaction |
|---|---|---|
| `200 OK` | intent ready on `/deploy/next` | decode, run deploy |
| `204 No Content` | `/deploy/next` timed out, or `/result` accepted | re-poll immediately |
| `401` | bad/missing bearer token | backoff, log; won't self-heal |
| `404` on `/deploy/next` | agent not registered yet | backoff; resolves once heartbeat upserts the row |
| `404` on `/result` | `/result` for unknown intent id | log, continue |
| `412 Precondition Failed` | agent has no project binding | backoff; resolves once an operator binds the agent in the UI |
| `5xx` / network error | coordinator down or transient glitch | backoff (see `poll.go`) |

## Auth and heartbeat

Two separate auth paths: agent transport uses `AGENT_SHARED_SECRET` (a single global value today — per-agent tokens are future work), while the UI and management APIs use session cookies from the login flow.

There is no explicit heartbeat endpoint. Middleware records "last seen" on every request carrying `X-Agent-Name`, so the act of polling *is* the heartbeat — that's what drives the green/red dot in the agent dashboard.

## Wire stability

Adding fields is safe (old agents ignore unknown JSON keys). Removing or renaming fields, or changing types, is breaking — agents already deployed decode into the old shape. Adding endpoints is safe; old agents simply don't call them.

This is what makes "deploy the server, upgrade agents later" safe: as long as nothing existing is renamed or removed, old agents keep working against new coordinators indefinitely.

## Why this shape

- **Outbound-only:** no inbound port on the VM, works behind NAT.
- **Long-poll, not webhooks:** the coordinator never needs to discover or reach an agent.
- **Stateless agent:** add a project in the UI and it works — no SSH, no per-agent config.
- **Shared Go module for wire types:** the compiler is the first line of contract enforcement.
