# Agent stacks root

## Context

To register a project today, an operator has to align three things by hand:

1. A host directory containing the stack's `docker-compose.yml`.
2. A bind mount into the agent container.
3. A "Compose file" path entered in the Projects UI.

A typical agent deployment looks like:

```
volumes:
  - /home/dev/medtutor-deployment:/opt/stacks/medtutor
```

…and the matching project row stores `/opt/stacks/medtutor/docker-compose.yml`.

## Problem

The setup is easy to get subtly wrong and the failure modes are unfriendly.

- **The UI field is mislabeled.** It says *"absolute path on agent host"* but the agent shells `docker compose -f <path>` from *inside* the container, so the value must be the container-side path. An operator who reads the label literally enters the host path and only finds out at the next deploy that the file "doesn't exist".
- **Two paths must stay in sync.** The container mount target and the project's compose path are independent fields in two different systems (agent compose file vs. coordinator DB). Nothing checks them against each other.
- **`COMPOSE_PROJECT_NAME` is set on the agent container itself.** That env leaks into every `docker compose` invocation the agent makes. It happens to be harmless while an agent owns exactly one stack, but it silently produces wrong project naming the moment an agent manages more than one.
- **Three names for one concept.** `AGENT_NAME`, the Projects UI name, and `COMPOSE_PROJECT_NAME` all describe the same stack but can drift apart.

The net effect: registering a new project is a copy-paste exercise across three places, with no validation, and the error messages don't point at the real mistake.

## Solution

Adopt a convention: every agent has a single **stacks root** directory, and project compose paths are stored relative to it.

```
HOST                AGENT CONTAINER
/home/dev/      ──► /stacks/
  ├─ medtutor/        ├─ medtutor/
  │  └─ compose.yml   │  └─ compose.yml   ← project stores "medtutor/compose.yml"
  └─ chatbot/         └─ chatbot/
     └─ compose.yml      └─ compose.yml   ← project stores "chatbot/compose.yml"
```

### Agent changes

- New env var `AGENT_STACKS_DIR` (default `/stacks`). The agent resolves each intent's compose path against it.
- `COMPOSE_PROJECT_NAME` is dropped from the agent's environment. The agent passes `--project-name <slug>` per command, so each stack is scoped correctly even when one agent manages several.
- `AGENT_NAME` is retained — it identifies the *agent process* to the coordinator (auth, intent dispatch) and is conceptually distinct from a stack. The "three names" problem collapses to two: the agent's identity, and the project's identity (which the per-command `--project-name` now derives from).

### Path resolution and validation

The compose path is treated as a relative path under `AGENT_STACKS_DIR`. Before invoking compose, the agent must:

1. Reject empty paths, absolute paths, and any path containing a `..` segment.
2. Clean the path (`filepath.Clean`) and re-check that the result is still relative and does not start with `..`.
3. Join against `AGENT_STACKS_DIR` and verify the joined path is still lexically contained within it (defense in depth against symlinked-root edge cases).
4. Stat the resolved file and fail fast with a message that names *both* the relative path and the resolved absolute path, so operators see what the agent actually looked for.

### Project-name slug

`docker compose --project-name` requires `[a-z0-9][a-z0-9_-]*`. The UI display name is free-form, so the coordinator derives a deterministic slug at project-create time and stores it on the project row:

- Lowercase, replace runs of non-`[a-z0-9_-]` with `-`, trim leading/trailing `-`.
- Reject if the result is empty or collides with an existing project's slug.
- The slug is immutable after creation (renaming the project changes the display name only). This avoids orphaning containers that were started under the old slug.

The agent receives the slug on each intent and passes it as `--project-name`.

### UI changes

- The "Compose file" field becomes a **relative** path. Label: *"Path to compose file, relative to the agent's stacks root"*. Helper text gives an example (`medtutor/compose.yml`) and links to the agent setup docs.
- Client-side validation mirrors the agent's rules (no leading `/`, no `..`).
- The project-create form shows the derived slug as a read-only preview next to the name field, so the operator sees what `--project-name` will be.

### Reference deployment

The `example/` agent deployment shrinks to a single bind mount:

```
volumes:
  - /home/dev:/stacks
```

Adding a new stack on the host needs zero changes to the agent container.

### Migration

Existing rows store absolute container paths like `/opt/stacks/medtutor/docker-compose.yml`. The plan is **flag, don't auto-rewrite**:

- At intent dispatch (coordinator side) and again on receipt (agent side), reject any compose path that is absolute or contains `..`. The error surfaces in the UI with a "this project needs to be reconfigured" banner and a link to edit the path.
- A one-shot admin script (or a manual SQL note in the migration doc) lists affected rows so operators can fix them deliberately.
- Slugs are backfilled from existing display names; collisions are surfaced for manual resolution.

Auto-rewriting was considered and rejected: the agent no longer knows the old mount prefix after the bind mount changes, so any rewrite would be a guess. A loud failure is safer than a silent guess.

## Why this is worth doing

- **Removes the most error-prone field** in the project setup flow. Operators no longer think about container paths.
- **Eliminates an entire class of silent bugs** (`COMPOSE_PROJECT_NAME` bleed) before multi-stack agents become common — which Phase 5 explicitly anticipates.
- **Cheap on the schema.** No destructive DB change; one new column for the slug, existing `compose_file` column repurposed as relative.

## Non-goals

- No change to how agents authenticate or how intents are dispatched.
- No change to the compose lifecycle commands used (`docker compose pull` + `up -d`); only their invocation arguments change (`--project-name` added, path resolved differently).
- Not introducing per-project compose templating or generation — the operator still owns the compose file on the host.
- Not supporting multiple stacks roots per agent. One agent, one root.
