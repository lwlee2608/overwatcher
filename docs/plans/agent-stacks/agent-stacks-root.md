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

- **The UI field is mislabeled.** It says *"absolute path on agent host"* but the agent actually shells `docker compose -f <path>` from *inside* the container, so the value must be the container-side path. An operator who reads the label literally will enter the host path and only find out at the next deploy that the file "doesn't exist".
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

Concretely:

- The agent gains one env var (e.g. `AGENT_STACKS_DIR`, default `/stacks`) and resolves the project's compose path against it.
- The Projects UI field becomes a **relative** path. The label and helper text describe it as project-relative, removing the "host vs container" confusion entirely.
- The reference agent deployment in `example/` shrinks to a single bind mount: `/home/dev:/stacks`. Adding a new stack on the host needs zero changes to the agent container.
- `COMPOSE_PROJECT_NAME` is dropped from the agent's environment. The agent passes `--project-name` per-command using the project's own name, so each stack is scoped correctly even when an agent manages several.

## Why this is worth doing

- **Removes the most error-prone field** in the project setup flow. Operators no longer think about container paths.
- **Eliminates an entire class of silent bugs** (`COMPOSE_PROJECT_NAME` bleed) before multi-stack agents become common — which Phase 5 explicitly anticipates.
- **Cheap to migrate.** Existing absolute paths can be detected at read time and either auto-rewritten or flagged; no destructive change to the DB schema is required.

## Non-goals

- No change to how agents authenticate or how intents are dispatched.
- No change to the deploy execution model (`docker compose pull` + `up -d`).
- Not introducing per-project compose templating or generation — the operator still owns the compose file on the host.
