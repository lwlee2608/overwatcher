# Agent as a systemd daemon

## Context

The current agent ships as a Docker image (`lwlee2608/agent:latest`) that runs
inside a container and shells out to `docker compose` on the host via the
mounted socket. Setting it up for a new VM today requires aligning several
moving parts by hand:

1. `AGENT_NAME` must exactly match the agent record created in the UI.
2. `AGENT_SHARED_SECRET` must match the coordinator's secret.
3. `AGENT_COORDINATOR_URL` must point at the coordinator.
4. `/var/run/docker.sock` must be bind-mounted in.
5. The deployment directory must be bind-mounted to whatever container-side
   path the project's `compose_file` column expects (e.g.
   `/opt/stacks/my-stack`).
6. `~/.docker/config.json` must be mounted in to pull private images.
7. `.env` files referenced by the compose file must live next to it in the
   mounted directory.

Every one of those is a footgun, and the failure modes are unfriendly: the
deploy succeeds the first time, then breaks at registry auth, or compose
parses a path that doesn't exist inside the container, or `${VAR}` in the
compose file resolves to empty because `.env` isn't where compose looks.

The root cause is dual-environment: container paths and host paths must
stay in sync, and the agent runs as a stranger to the host filesystem and
auth state.

## Goal

Ship the agent as a **native binary running under systemd on the VM**.
The agent runs as a normal host user in the `docker` group, talking to the
local Docker daemon the same way a human operator would. The whole class of
container/host-path bugs disappears because there is no container.

Setup must be **one copy-paste from the UI**. The user creates an agent in
the UI, copies a command, pastes it into the VM's shell, and the agent is
running.

This deployment model targets VMs (EC2, GCE, on-prem Linux hosts). A
Kubernetes-native agent is a separate future effort and is out of scope.

## Non-goals

- Replacing the Docker-image agent. Existing prod deployments (e.g.
  medtutor) keep working unchanged. The systemd agent becomes the
  recommended path for new deployments; the Docker agent stays supported.
- Per-agent shared secrets. The current single-secret model
  (`internal/api/http/router.go:29`) is unchanged. Per-agent tokens are a
  separate improvement that this plan does not block on.
- Auto-update of the binary on the VM. The installer is re-runnable to
  upgrade, but there is no background updater.

## Design

### One binary, two deployment modes

No new Go code path. The existing `cmd/agent` binary already runs natively
on a host — the Docker image just wraps it. The systemd deployment uses
the same binary; only the packaging and configuration story changes.

```
                        ┌─────────────────┐
                        │  cmd/agent      │  ← same code
                        │  (Go binary)    │
                        └────────┬────────┘
                                 │
                ┌────────────────┴────────────────┐
                │                                 │
        ┌───────▼────────┐               ┌────────▼─────────┐
        │ Docker image   │               │ systemd unit     │
        │ (existing)     │               │ (new)            │
        │                │               │                  │
        │ /opt/stacks/   │               │ runs as host     │
        │ paths,         │               │ user, native     │
        │ socket mount   │               │ paths, native    │
        │                │               │ ~/.docker creds  │
        └────────────────┘               └──────────────────┘
```

### Configuration

The systemd agent is **env-var only**. No YAML file. Variables:

| Variable                  | Required | Default                          |
|---------------------------|----------|----------------------------------|
| `AGENT_NAME`              | yes      | —                                |
| `AGENT_SHARED_SECRET`     | yes      | —                                |
| `AGENT_COORDINATOR_URL`   | no       | hosted coordinator URL (TBD)     |
| `AGENT_POLL_TIMEOUT`      | no       | `30s`                            |
| `LOG_LEVEL`               | no       | `info`                           |

These are written to `/etc/overwatcher-agent.env` by the installer and
loaded by the unit via `EnvironmentFile=`.

To make this work without `application-agent.yml` present at runtime, the
binary must carry defaults itself. The current `InitConfig` requires the
YAML file (`adder.ReadInConfig` returns an error if missing). The fix:

- Embed the existing `application-agent.yml` via `//go:embed`, fall back to
  it if no on-disk file is found. Env vars still override.
- Alternative: bake defaults via `adder.SetDefault(...)` calls. Slightly
  more code, no embedded file, but contradicts the existing comment in
  `config.go` that warns against in-code defaults.

Pick the embed approach: it preserves "YAML is the single source of truth
for defaults" while letting the binary run standalone.

### Installer script

A single hosted script: `https://<host>/install.sh` (URL TBD — likely the
coordinator itself serves it).

What it does, top to bottom:

1. Validate `AGENT_NAME` and `AGENT_SHARED_SECRET` are set in env.
2. Detect arch (`uname -m`), select the right release binary URL.
3. Create system user `overwatcher` if absent, add to `docker` group.
4. Download binary to `/usr/local/bin/overwatcher-agent`, chmod +x.
5. Write `/etc/overwatcher-agent.env` (mode 0600, owned by `overwatcher`)
   containing `AGENT_NAME`, `AGENT_SHARED_SECRET`,
   `AGENT_COORDINATOR_URL`.
6. Write `/etc/systemd/system/overwatcher-agent.service`.
7. `systemctl daemon-reload && systemctl enable --now overwatcher-agent`.
8. Tail the first ~20 log lines so the user sees success or failure
   immediately.

Re-running the installer with the same env upgrades the binary in place
(`systemctl restart` after the binary swap).

### The systemd unit

```ini
[Unit]
Description=Overwatcher deployment agent
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
User=overwatcher
Group=docker
EnvironmentFile=/etc/overwatcher-agent.env
ExecStart=/usr/local/bin/overwatcher-agent
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

`Group=docker` is the key bit — it gives the agent access to
`/var/run/docker.sock` without needing root. The agent reads
`/home/overwatcher/.docker/config.json` for registry auth, the same way a
human operator would. No mount juggling.

### Binary distribution

Use GitHub Releases. The Makefile already builds `bin/agent` with version
ldflags. Add a release workflow (or manual step for v1) that:

- Builds `overwatcher-agent_linux_amd64` and `overwatcher-agent_linux_arm64`.
- Uploads to a tagged GitHub release.
- The installer downloads from the latest release tag (or a pinned tag the
  installer hardcodes per version).

A SHA256SUMS file is published alongside; the installer verifies the
download before installing.

### UI integration

When the user creates an agent in the UI, the agent detail/create page
shows a copy-paste install command:

```bash
AGENT_NAME=my-agent \
AGENT_SHARED_SECRET=<paste-or-show-existing> \
curl -fsSL https://<coordinator>/install.sh | sudo -E bash
```

The `-E` flag preserves the env vars through `sudo`. The page also shows
the systemd commands the user might want later (`systemctl status`,
`journalctl -u overwatcher-agent -f`).

Open question: where does the shared secret come from in the UI? Today
it's a global coordinator env var and isn't displayed anywhere. Options:

- **a)** The UI page shows a placeholder (`<your-AGENT_SHARED_SECRET>`) and
  the admin pastes the secret from wherever they keep it. Zero backend
  changes. Less polished, but honest about the global-secret model.
- **b)** Add an admin-only endpoint that returns the current secret. Lets
  the UI fill it in. Mild security tradeoff — anyone with admin UI access
  can read it (which is approximately true today via env vars on the
  coordinator host anyway).

Start with (a); revisit when per-agent tokens land.

### Docs

- Update top-level `README.md` to mention two deployment options, with
  systemd as the recommended path.
- New page: `docs/agent-systemd.md` covering install, upgrade, logs,
  uninstall, troubleshooting.
- Add a "Deprecation note" to `example/docker-compose.yml` pointing at the
  systemd path for new users; keep the file for the docker path.

## Implementation steps

Ordered for incremental review:

1. **Binary works without YAML on disk.** Embed
   `application-agent.yml` via `//go:embed`, fall back when no file is
   present. Env vars still override. Add a test that exercises the
   no-file path.
2. **Release artifacts.** Add a `make release-agent` target that cross-
   compiles linux/amd64 and linux/arm64, strips, and writes a SHA256SUMS
   file. Wire a GitHub Actions workflow that publishes these to a tagged
   release.
3. **Installer script.** Add `scripts/install-agent.sh` to the repo.
   Decide hosting — likely served by the coordinator at `/install.sh`
   (small static handler that returns the script with the coordinator URL
   substituted in). Verify SHA256 on download.
4. **Systemd unit template.** Embed the unit file content in the
   installer script (no separate file to fetch). Use a heredoc with
   variable substitution.
5. **UI install command card.** Add an "Install command" section to the
   agent create/detail page. Show the one-liner with `AGENT_NAME`
   filled in and a `<your-AGENT_SHARED_SECRET>` placeholder. Show the
   journalctl/systemctl tips below.
6. **Docs.** New `docs/agent-systemd.md`; README updates; note in
   `example/docker-compose.yml`.

Each step is independently reviewable and shippable. The installer + unit
work (1–4) can land before the UI work (5).

## Risks and mitigations

- **The installer requires `sudo`.** Users who don't trust the script can
  read it first (`curl ... | less`) or use the documented manual path
  (download binary, write env file, install unit). Document both.
- **`docker compose` plugin missing on the VM.** Installer should check
  for it and error early with a clear message ("install docker-compose
  plugin: `sudo apt install docker-compose-plugin`").
- **SELinux / hardened distros.** Out of scope for v1. Document as a
  known limitation; users on those distros run the Docker agent.
- **Upgrade across schema changes in `/etc/overwatcher-agent.env`.** Keep
  the env file format flat (`KEY=value`) and only add new keys, never
  rename, so old env files keep working.

## Future: Kubernetes agent

When this lands cleanly, a third deployment mode becomes natural: a
Helm chart that runs the same coordinator-polling logic but executes via
the Kubernetes API (Deployments / Jobs) instead of `docker compose`.
That's a new binary path (`cmd/agent-k8s`) sharing the coordinator
protocol code, not a wrapper around this one. Out of scope here, listed
so future-us doesn't accidentally entangle the two.

## Open decisions

1. **Coordinator URL default.** What hosted URL does the installer
   default to? Needed before the installer can ship.
2. **Where is `install.sh` served?** Coordinator endpoint vs. a separate
   static host (GitHub Pages, a cloud bucket). Coordinator is simpler;
   static is cheaper.
3. **How does the UI get the shared secret?** Option (a) placeholder vs.
   (b) admin endpoint — see UI integration section.
4. **Per-agent shared secrets.** Worth doing after systemd-agent ships,
   or punt indefinitely?
