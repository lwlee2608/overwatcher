# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

This project uses a Makefile. Prefer `make` targets over calling `go` directly.

- `make build` - Build coordinator to `bin/overwatcher`
- `make build-agent` - Build agent to `bin/agent`
- `make run` - Run the coordinator
- `make run-agent` - Run the agent
- `make test` - Run all tests (`go test -v ./...`)
- `make generate` - Regenerate sqlc code from SQL queries
- `make dep` - Install sqlc tooling
- `make clean` - Clean test cache and bin directory

To run a single test: `go test -v -run TestName ./path/to/package/...`

## Architecture

Overwatcher is a GitHub App that receives push webhooks, creates GitHub Deployments, and dispatches deploy intents to agents running on target VMs.

**Stack:** Go 1.25, Gin HTTP framework, `google/go-github` for GitHub API, `bradleyfalzon/ghinstallation` for GitHub App auth, PostgreSQL via `pgx/v5` + `sqlc`, `goose` for migrations.

**Configuration:** Uses `lwlee2608/adder` (Viper-like config library) to load `application.yml` with env var overrides (e.g., `GITHUB_WEBHOOK_SECRET`, `DATABASE_URL`). Supports `.env` files via godotenv.

### Layers

- `cmd/overwatcher/` - Coordinator entrypoint, config loading, DB init, graceful shutdown
- `cmd/agent/` - Agent entrypoint, long-poll loop, Docker Compose executor
- `internal/api/http/` - Gin router, handlers, middleware (request logging, error handling, webhook signature verification, bearer token auth)
- `internal/service/` - Business logic, split into sub-packages:
  - `intent/` - `Store` interface + `DBStore` (PostgreSQL-backed, production) + `MemoryStore` (tests only)
  - `mapping/` - Repo-to-stack config index
  - `webhook/` - Produces intents from push events
  - `dispatch/` - Consumes intents via long-poll, reports to GitHub. Owns `Reaper` for timeout/retry
- `internal/github/` - GitHub API client wrapper (App installation auth)
- `internal/db/` - PostgreSQL pool init, goose migrations, sqlc-generated queries

### Request Flow

GitHub webhook -> signature verification middleware -> `WebhookHandler` -> `webhook.Service.HandleEvent()` -> `handlePush` creates GitHub Deployment + enqueues intent -> agent long-polls `/api/v1/deploy/next` -> dispatch returns intent -> agent runs docker compose -> agent posts result -> dispatch updates GitHub

## Git Conventions

- Do not add "Co-Authored-By: Claude" lines to commit messages
