# Monorepo Multi-Image Deploy

## Background

Overwatcher now supports configurable `image` and `tag` per deploy mapping. Each
mapping specifies which container image (and tag) the agent should pull when a push
event is received. This replaced the old hardcoded `ghcr.io/<repo>` convention.

## Monorepo challenge

For repos that produce a single Docker image, one mapping is sufficient. Monorepos
like `byte-twister/medtutor-server` build 3 separate images from one repo:

| Service dir        | Docker-compose service | Image (Alibaba CR)                          |
|--------------------|------------------------|---------------------------------------------|
| medtutor-backend   | medtutor-server        | crpi-...aliyuncs.com/axcova/medtutor-server |
| medtutor-frontend  | medtutor-frontend      | crpi-...aliyuncs.com/axcova/medtutor-web    |
| medtutor-admin     | medtutor-admin         | crpi-...aliyuncs.com/axcova/medtutor-admin  |

Currently a single mapping can only carry one `image`. To deploy all three services,
you would need three separate mappings for the same repo, each with its own image
and service. The `UNIQUE(repo, agent_id)` constraint on `deploy_mappings` prevents
this — it must be relaxed first.

## Remaining work

1. **Relax the unique constraint** to allow multiple mappings per repo+agent pair.
2. **Update docker-compose** on target VMs so each service references `${IMAGE}:${IMAGE_TAG}`.
3. **Add CI workflow** to build and push all 3 images on push to main.
