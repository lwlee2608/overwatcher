# Per-agent tokens

## Context

Every agent today authenticates to `/api/v1/deploy/*` with the same global bearer token, set on the coordinator as `AGENT_SHARED_SECRET` and pasted by the operator into each VM's `AGENT_SHARED_SECRET` env var. The middleware (`BearerTokenAuth`) does a single string compare.

The dashboard's "Install a new agent" card embeds the literal `<your-AGENT_SHARED_SECRET>` placeholder — the operator has to look the secret up out-of-band.

This is roadmap Phase 4c, currently unimplemented.

## Problem

One secret guards the whole fleet.

- **No revocation.** Decommissioning one VM, or suspecting one was compromised, means rotating the secret on every other agent simultaneously.
- **No identity in audit.** Logs say "an agent called `/deploy/next`"; they can't say *which* agent. Anyone holding the secret can impersonate any agent name on the wire.
- **No scope.** Any agent could (in principle) accept a deploy intent for any project — nothing at the auth layer ties a token to a project binding.
- **Install UX is bad.** The operator copies a secret from a config file into a shell command. Mistakes are silent until first poll fails.

There is also a parallel gap on the **user→coordinator** path: `GET /api/v1/agents` returns every agent — and every agent's bound `project_id` / `project_name` — to any signed-in user. That gap predates this plan but is tightly coupled to it, because per-agent tokens need an "owner" concept to be revoked/managed by the right humans. Both are fixed together below.

## Solution

Mint one random opaque token per agent at registration time. Bind the token to the agent row; embed it directly in the install command.

```
   Admin (dashboard)            Coordinator                  Agent (VM)
        │                            │                            │
        │  POST /api/v1/agents       │                            │
        │  { name: "demo-1" }        │                            │
        ├───────────────────────────►│                            │
        │                            │ create agent row           │
        │                            │ installed_by_user = caller │
        │                            │ mint agent_token (random)  │
        │  200 { agent_id,           │                            │
        │        agent_token,        │                            │
        │        install_command }   │                            │
        │◄───────────────────────────┤                            │
        │                            │                            │
        │       operator pastes install_command on the VM         │
        │ ───────────────────────────────────────────────────────►
        │                            │     write agent_token to   │
        │                            │ /etc/overwatcher-agent.env │
        │                            │                            │
        │                            │  GET /api/v1/deploy/next   │
        │                            │  Bearer <agent_token>      │
        │                            │◄───────────────────────────┤
        │                            │ look up token → agent      │
        │                            │ attach agent context       │
        │                            │   200 { intent | empty }   │
        │                            ├───────────────────────────►│
        │                            │                            │
```

Middleware changes from "compare to global" to "look up token → resolve agent → attach agent context". The existing `AgentHeartbeat` middleware already runs after auth; it can now trust the resolved agent identity instead of trusting the `X-Agent-Name` header.

### Why one token, not a bootstrap pair

An earlier draft used the two-token "bootstrap" pattern (a single-use, short-TTL *install token* that the agent exchanges via `POST /agents/{id}/claim` for a long-lived *agent token*). Its only real benefit: the long-lived credential never appears in shell history / chat / install logs — only the throwaway install token does.

That benefit doesn't pay for itself here:

- The token is **opaque and DB-indexed, so revocation is instant** (delete row → token dead). The bootstrap dance mostly earns its keep when revocation is *hard* (signed/stateless tokens needing a denylist). Paying for both is redundant.
- It doesn't fully close the paste-leak hole anyway — it just narrows it to a trust-on-first-use race (if the command leaks before the real agent claims, an attacker claims first).
- It costs a `/claim` endpoint, a consumed-state flag, TTL logic, and an extra column — for an operator-controlled VM fleet of modest size.

So: one token, embedded in the command, revocable by deleting the agent. The trade-off accepted is that the long-lived token lands in shell history / install logs. If audit hygiene or fleet size later justifies it, the bootstrap pair is a clean additive follow-up — shipping one token now doesn't block adding `/claim` later.

### Access control on agent endpoints

A new column `agents.installed_by_user_id` is added at registration time. It is **not** a permanent owner — it exists so a freshly installed, unbound agent has a single human who can see and manage it before it gets linked to a team project.

Visibility follows the agent's lifecycle:

```
   agent lifecycle                  who can see / manage it
   ───────────────                  ───────────────────────
   ┌───────────────┐
   │    unbound    │  ───────────►  installed_by_user_id  (+ admins)
   └───────┬───────┘
           │ PUT /api/v1/agents/:id/project
           ▼
   ┌───────────────┐
   │ bound to P    │  ───────────►  members of project P  (existing project ACL)
   └───────────────┘
```

This is layered on top of the existing project membership model rather than replacing it. A separate flat `agent.owner_user_id` was considered and rejected: if alice installs an agent and binds it to a team project, then leaves the team, bob (still on the project) needs to manage it. Tying visibility to the installer forever would block that — project membership handles the handover automatically once binding happens.

Applied to the HTTP layer:

- `GET /api/v1/agents` — `WHERE (project_id IS NULL AND installed_by_user_id = caller) OR project_id IN (caller's project memberships)`. Admins bypass.
- `GET /api/v1/agents/:id`, `DELETE /api/v1/agents/:id`, `PUT /api/v1/agents/:id/project` — same gate.
- Per-agent **tokens** still belong to the *agent*, not the user. Who is permitted to mint or revoke a token is governed by the rules above; the token itself carries no user identity.

### Migration

One pass — no global-secret fallback. The middleware switches to token-lookup-only; the `AGENT_SHARED_SECRET` compare is deleted in the same change.

The fleet is small and operator-controlled, so a coordinated cutover beats carrying a dual-auth path:

1. For each existing agent, mint a token and surface a one-liner to re-run on the VM (the install command, pointed at the existing agent row).
2. Operator re-runs it on each VM. Until they do, that agent's `/deploy/*` calls 401 — a loud, expected signal, not a silent fallback.
3. Once all agents are re-issued, drop `AGENT_SHARED_SECRET` from the coordinator config.

Keeping a fallback path was rejected: it doubles the auth surface, hides which agents haven't migrated behind "it still works," and the global secret is exactly the thing this plan exists to delete — leaving it live defeats the point.

## Why this is worth doing

- **Real revocation.** Delete an agent row → its token stops working. No fleet-wide impact.
- **Identity in audit.** Every `/deploy/*` request carries a resolved agent ID. Logs, metrics, and event history all become per-agent.
- **Sets up project-scoped auth.** Once tokens resolve to agents, and agents are already bound to projects (`agents.project_id` exists today), gating "this token may only accept intents for project X" is a one-line check.
- **Better install UX as a side effect.** The install card no longer needs to display a shared secret at all — it provisions an agent and shows a ready-to-paste command containing that agent's token. No out-of-band secret lookup.
- **Removes the current global-secret-in-UI question** entirely. (The stopgap of fetching the global secret into the install card becomes moot — there is no global secret to fetch.)
- **Closes the dashboard info-leak.** `installed_by_user_id` plus the existing project ACL means `/api/v1/agents` stops returning every team's agents — and their project bindings — to every signed-in user.

## Non-goals

- No change to the deploy execution model or compose handling.
- No change to how the coordinator gets *its* secrets (webhook signing, GitHub App key, DB creds).
- Not introducing user-scoped tokens. Tokens belong to agents; humans authenticate via session cookies as today.
- Not implementing token rotation policies (expiry, forced re-issue). One token per agent, valid until revoked. Rotation can come later if needed.
- No bootstrap/`claim` indirection. Long-lived token is handed out at registration and pasted directly (see [Why one token, not a bootstrap pair](#why-one-token-not-a-bootstrap-pair)).

## Open questions

- **Token storage on the agent.** Reuse `/etc/overwatcher-agent.env` (systemd) and a bind-mounted env file (Docker), or introduce a dedicated token file with stricter perms?
- **Installer leaves the system.** If `installed_by_user_id` references a user that gets deleted, what happens to an agent that is still unbound? Options: cascade-delete the agent, reassign to an admin, or leave it visible only to admins. Project-bound agents are unaffected (ACL flips to project membership).
- **Admin role.** The access rules above assume an "admin bypass." That role isn't formalized today; either piggy-back on a future admin flag or scope this plan to non-admin rules only and revisit when admin is introduced.
