# Overwatcher

A GitHub App that automates the **CD** half of a CI/CD pipeline for projects deployed to a VM (e.g. an EC2 instance running Docker Compose).

## What it does

CI stays where it already works well — a normal GitHub Actions workflow on a GitHub runner builds and publishes the Docker image. Overwatcher takes over from there:

1. Listens for the build/deployment event on the target repo.
2. Connects to the target VM and pulls the new image.
3. Restarts the affected Docker Compose services.

## Why

Today the "deploy" step means SSH-ing into the VM, running `docker pull`, then `docker compose up -d` by hand. Overwatcher exists to remove that manual step so a push to `main` is all it takes to ship.
