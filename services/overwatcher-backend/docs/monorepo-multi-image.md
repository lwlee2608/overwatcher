# Monorepo Multi-Image Deploy Problem

## Context

Overwatcher currently resolves a single image per deploy intent (`ghcr.io/<repo>`).
This works for repos that produce one Docker image, but breaks for monorepos like
`byte-twister/medtutor-server` which builds 3 separate images from one repo:

| Service dir        | Docker-compose service | Current image (Alibaba CR)                |
|--------------------|------------------------|-------------------------------------------|
| medtutor-backend   | medtutor-server        | crpi-...aliyuncs.com/axcova/medtutor-server |
| medtutor-frontend  | medtutor-frontend      | crpi-...aliyuncs.com/axcova/medtutor-web    |
| medtutor-admin     | medtutor-admin         | crpi-...aliyuncs.com/axcova/medtutor-admin  |

## How it breaks

1. `webhook.go` calls `mapping.ResolveImage(repo)` which returns a single image name.
2. `runner.go` injects that as `IMAGE` env var into `docker compose pull/up`.
3. Each compose service has a different image — a single `IMAGE` cannot address all three.
4. The compose file uses hardcoded image references, so `IMAGE` and `IMAGE_TAG` are ignored.

## Proposed fix

Since all services share the same git SHA (same repo, same push), the **tag** is the
common denominator, not the image name.

### 1. Docker-compose: use `IMAGE_TAG` only

Each service keeps its own image name but references the shared tag:

```yaml
medtutor-server:
  image: ghcr.io/byte-twister/medtutor-server/backend:${IMAGE_TAG:-latest}

medtutor-frontend:
  image: ghcr.io/byte-twister/medtutor-server/frontend:${IMAGE_TAG:-latest}

medtutor-admin:
  image: ghcr.io/byte-twister/medtutor-server/admin:${IMAGE_TAG:-latest}
```

### 2. CI: build and push all 3 images on push to main

Add a GitHub Actions workflow that builds each service's Dockerfile and pushes to
ghcr.io (or the Alibaba registry) tagged with the git SHA.

### 3. Overwatcher: no code change needed

The runner already sets `IMAGE_TAG=<sha>`. The `IMAGE` env var goes unused, which is
fine. No overwatcher code changes required for this approach.

## Status

Deferred — testing with a single image first to validate the end-to-end flow.
