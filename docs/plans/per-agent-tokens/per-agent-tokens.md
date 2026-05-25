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

## Solution

Mint one long-lived token per agent at registration time. Bind the token to the agent row.

```
admin clicks "Install agent" in UI
        │
        ▼
POST /api/v1/agents               coordinator
  { name: "demo-1" }              ──────────►  agent row created
                                               install_token minted (single-use, 1h TTL)
        ◄──────────────────────────────────
  { agent_id, install_token, install_command }
        │
        ▼
operator pastes command on VM
        │
        ▼
agent first contact:
POST /api/v1/agents/{id}/claim    coordinator
  Bearer <install_token>          ──────────►  verify token + TTL + unused
                                               mint agent_token (long-lived)
                                               mark install_token consumed
        ◄──────────────────────────────────
  { agent_token }
        │
        ▼
agent writes agent_token to /etc/overwatcher-agent.env
agent uses it for all subsequent /deploy/* calls
```

Middleware changes from "compare to global" to "look up token → resolve agent → attach agent context". The existing `AgentHeartbeat` middleware already runs after auth; it can now trust the resolved agent identity instead of trusting the `X-Agent-Name` header.

### Migration

Existing agents in the field still hold the global secret. Two-step:

1. Ship per-agent tokens behind a fallback: middleware tries token lookup first, falls back to global-secret compare. Log a warning each time the fallback fires.
2. Add an admin action ("rotate") that mints a fresh token for an existing agent and surfaces a one-liner to re-run on the VM. Once the warning logs go quiet, drop the global-secret path.

## Why this is worth doing

- **Real revocation.** Delete an agent row → its token stops working. No fleet-wide impact.
- **Identity in audit.** Every `/deploy/*` request carries a resolved agent ID. Logs, metrics, and event history all become per-agent.
- **Sets up project-scoped auth.** Once tokens resolve to agents, and agents are already bound to projects (`agents.project_id` exists today), gating "this token may only accept intents for project X" is a one-line check.
- **Better install UX as a side effect.** The install card no longer needs to display a shared secret at all — it provisions an agent and shows a command containing a one-time token. Mistypes get a clean "token expired or already used" instead of "401 forever".
- **Removes the current global-secret-in-UI question** entirely. (The stopgap of fetching the global secret into the install card becomes moot — there is no global secret to fetch.)

## Non-goals

- No change to the deploy execution model or compose handling.
- No change to how the coordinator gets *its* secrets (webhook signing, GitHub App key, DB creds).
- Not introducing user-scoped tokens. Tokens belong to agents; humans authenticate via session cookies as today.
- Not implementing token rotation policies (expiry, forced re-issue). One token per agent, valid until revoked. Rotation can come later if needed.

## Open questions

- **Install token transport.** Embed in the curl command as an env var (matches the current shape), or pass as a `?token=` query string to the install script (lets the script fetch agent-specific config without env-var sprawl)?
- **Token storage on the agent.** Reuse `/etc/overwatcher-agent.env` (systemd) and a bind-mounted env file (Docker), or introduce a dedicated token file with stricter perms?
- **Token format.** Random opaque string (simplest, requires DB lookup per request) vs. signed token carrying agent ID (no lookup, but rotation/revocation needs a denylist or short TTL + refresh)? Opaque + indexed lookup is fine at this scale; revisit only if `/deploy/next` latency matters.
