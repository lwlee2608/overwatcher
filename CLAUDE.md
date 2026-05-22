# CLAUDE.md

Monorepo for **Overwatcher** — a GitHub App that handles CD for VM-based Docker Compose deployments. See [README.md](README.md) for product overview.

## Layout

- `services/overwatcher-backend/` — Go coordinator + agent. Build via `make` (`make test`, `make build`, `make run`).
- `services/overwatcher-frontend/` — React + Vite UI. Uses **pnpm** (`pnpm install`, `pnpm lint`, `pnpm dev`).
- `services/overwatcher-db/` — database setup.
- `example/` — reference agent deployment.
- `docs/` — design notes.
