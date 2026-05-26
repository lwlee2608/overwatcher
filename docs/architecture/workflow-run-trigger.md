# `workflow_run` Trigger

## Why this exists

The default trigger is `push`: as soon as GitHub sees a commit, Overwatcher
queues a deploy. That works for repos that publish images outside of CI, but
it races CI in the common setup where a GitHub Actions workflow builds and
pushes the image. The agent's `docker compose pull` can fire before the new
tag exists in the registry — so it pulls the previous image, or fails with
`manifest unknown`, while the GitHub Deployment is still marked successful.

`workflow_run` fixes that. Overwatcher waits for a named build workflow to
complete with `conclusion == success` before queuing the deploy. The new
image is guaranteed to exist by then.

```
push ──▸ CI builds + publishes ──▸ workflow_run(success) ──▸ deploy
                  │
                  └─ failure / cancelled → no deploy
```

## Choosing per service

The trigger is **per service**, not per project, so a monorepo can mix
strategies (e.g. one service publishes via Actions, another is published
externally and is fine with `push`).

- **`workflow` field unset** → service deploys on `push` (legacy behavior).
- **`workflow` field set** to a filename like `build-and-publish.yml` →
  service deploys on `workflow_run` for that workflow only.

The match is on the **filename** (basename of the workflow path), not the
`name:` field inside the workflow YAML. Filenames are stable across renames
of the display name.

## Setup checklist

There are three places to configure: the GitHub App, Overwatcher itself, and
the monitored repo. None of them require code changes to your application.

### 1. GitHub App webhook subscription

Add `workflow_run` to the App's subscribed events. Without this the
coordinator never receives the event.

- Go to your App's settings page on GitHub: **Settings → Developer settings
  → GitHub Apps → \<your app\> → Permissions & events → Subscribe to events**.
- Tick **Workflow run**.
- Permissions: set **Actions** to **Read-only**. `push` webhooks can work without this permission, so verify it explicitly when enabling `workflow_run`.
- Save. Existing installations pick up the new subscription automatically.

### 2. Overwatcher service config

In the Overwatcher UI, open the **Project detail** page and edit the
service. Set the **Workflow** column to the workflow filename, e.g.
`build-and-publish.yml`. Leave it empty to keep `push` behavior.

The branch filter still applies — only `workflow_run` events whose
`head_branch` matches the service's configured branch enqueue a deploy.

### 3. Monitored repo (the one being deployed)

Your build workflow must:

1. Run on the branch the service is configured for (typically `main`).
2. Push the image with the tag Overwatcher expects (commonly the commit
   SHA or `latest`, depending on how the service's `tag` field is set).
3. Exit with `success` only when the image is fully published.

A minimal example for `.github/workflows/build-and-publish.yml`:

```yaml
name: Build and publish
on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: |
            ghcr.io/${{ github.repository }}:${{ github.sha }}
            ghcr.io/${{ github.repository }}:latest
```

Notes:

- The service's **Workflow** field would be `build-and-publish.yml` to match
  this file.
- `workflow_run` only fires for workflows on the repo's **default branch**
  initially. Once a workflow file has run on `main` at least once, GitHub
  will send `workflow_run` events for runs on other branches too.
- Push the image **inside the same job that completes the workflow**. If
  publishing happens in a downstream job that the workflow doesn't wait
  for, `workflow_run(success)` can fire before the image lands.

## Belt-and-braces: pull retry

Even with `workflow_run`, the registry can lag a few seconds behind a
successful workflow (especially GHCR under load). The agent retries
`docker compose pull` up to 5 times with exponential backoff on
`manifest unknown` / `not found` before giving up. Retries on other failure
classes (auth, malformed compose, network down) are intentionally skipped
so a real problem fails fast.

## Failure modes and what happens

| Scenario | Behavior |
|---|---|
| Service has no `workflow` set | Deploys on `push` as before |
| `workflow` set, CI succeeds | Deploys after `workflow_run(success)` |
| `workflow` set, CI fails or is cancelled | No deploy. No GitHub Deployment created. |
| `workflow` set, no CI exists for that filename | No deploy (no event ever fires) |
| `workflow_run(success)` arrives before registry has the image | Pull retries, succeeds when image lands |
| Registry never publishes the image | Pull exhausts retries, deploy fails — surfaced in deployment status |

## Migrating an existing service

1. Add a `workflow_run`-eligible workflow to the repo if you don't already
   have one (or rename the existing one if needed — match by filename).
2. Subscribe the GitHub App to `workflow_run` (one-time).
3. Edit the service in Overwatcher and set the **Workflow** field.

The change takes effect on the next event. There's no DB migration or
restart required for existing projects — services without the field keep
their current `push` behavior.
