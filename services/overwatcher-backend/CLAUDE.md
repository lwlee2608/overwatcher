# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

This project uses a Makefile. Prefer `make` targets over calling `go` directly.

- `make build` - Build binary to `bin/overwatcher`
- `make run` - Run the application
- `make test` - Run all tests (`go test -v ./...`)
- `make clean` - Clean test cache and bin directory

To run a single test: `go test -v -run TestName ./path/to/package/...`

## Architecture

Overwatcher is a GitHub App that receives webhook events and takes automated actions (e.g., creating GitHub deployments on push to main/master).

**Stack:** Go 1.25, Gin HTTP framework, `google/go-github` for GitHub API, `bradleyfalzon/ghinstallation` for GitHub App auth.

**Configuration:** Uses `lwlee2608/adder` (Viper-like config library) to load `application.yml` with env var overrides (e.g., `GITHUB_WEBHOOK_SECRET`). Supports `.env` files via godotenv.

### Layers

- `cmd/overwatcher/` - Entrypoint, config loading, logger setup
- `internal/api/http/` - Gin router, handlers, middleware (request logging, error handling, webhook signature verification)
- `internal/service/` - Business logic, split into sub-packages: `intent/` (shared queue + in-flight store), `mapping/` (repo→stack config), `webhook/` (produces intents from push events), `dispatch/` (consumes intents, long-poll transport to agents)
- `internal/github/` - GitHub API client wrapper (App installation auth)

### Request Flow

GitHub webhook -> signature verification middleware -> `WebhookHandler` -> `webhook.Service.HandleEvent()` -> event-specific handler (e.g., `handlePush` creates a deployment)

## Git Conventions

- Do not add "Co-Authored-By: Claude" lines to commit messages
