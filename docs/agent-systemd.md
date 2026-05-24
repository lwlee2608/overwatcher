# Agent (systemd)

The systemd agent runs as a native Linux binary on the VM, talking to the
local Docker daemon the same way a human operator would. Use it for new
deployments — the older Docker-image agent stays supported but isn't the
recommended path.

## Install

On the target VM:

```bash
AGENT_NAME=my-agent \
AGENT_SHARED_SECRET=<paste-your-shared-secret> \
curl -fsSL https://<coordinator-host>/install.sh | sudo -E bash
```

What this does:

1. Creates a `overwatcher` system user and adds it to the `docker` group.
2. Downloads `overwatcher-agent_linux_<arch>` from the configured GitHub
   release, verifies it against `SHA256SUMS`, installs it to
   `/usr/local/bin/overwatcher-agent`.
3. Writes `/etc/overwatcher-agent.env` (mode 0600) with `AGENT_NAME`,
   `AGENT_SHARED_SECRET`, `AGENT_COORDINATOR_URL`.
4. Writes `/etc/systemd/system/overwatcher-agent.service`.
5. Enables and starts the unit, prints the first ~20 log lines.

The UI's agent dashboard has an "Install a new agent" card that fills in the
URL and the agent name for you — paste your shared secret into the
placeholder and copy.

### Prerequisites

- Docker is installed and the `docker compose` plugin is available
  (`docker compose version` must succeed).
- The host runs systemd (any reasonably current Debian / Ubuntu / RHEL /
  Amazon Linux / Fedora works). SELinux-hardened distros are not yet
  supported — use the Docker-image agent there.

## Upgrade

Re-run the same install one-liner. The binary is swapped in place and the
unit restarted. The env file at `/etc/overwatcher-agent.env` is left
untouched — only edit it by hand if you need to change `AGENT_NAME` or the
coordinator URL.

## Logs

```bash
journalctl -u overwatcher-agent -f          # follow
journalctl -u overwatcher-agent -n 200      # last 200 lines
systemctl status overwatcher-agent          # current state + last lines
```

## Configuration

Configuration lives in `/etc/overwatcher-agent.env`. Edit it, then
`sudo systemctl restart overwatcher-agent` to apply.

| Variable                  | Required | Default                       |
|---------------------------|----------|-------------------------------|
| `AGENT_NAME`              | yes      | —                             |
| `AGENT_SHARED_SECRET`     | yes      | —                             |
| `AGENT_COORDINATOR_URL`   | no       | (filled by installer)         |
| `AGENT_POLL_TIMEOUT`      | no       | `30s`                         |
| `LOG_LEVEL`               | no       | `info`                        |

Defaults beyond the table above are baked into the binary via
`//go:embed application-agent.yml`. There is no on-disk YAML in the systemd
deployment.

## Uninstall

```bash
sudo systemctl disable --now overwatcher-agent
sudo rm /etc/systemd/system/overwatcher-agent.service
sudo rm /etc/overwatcher-agent.env
sudo rm /usr/local/bin/overwatcher-agent
sudo userdel overwatcher    # optional
sudo systemctl daemon-reload
```

## Troubleshooting

**`docker compose plugin is missing`** during install — install it before
re-running:

```bash
sudo apt install docker-compose-plugin   # Debian/Ubuntu
```

**Agent starts then logs `connection refused` against the coordinator** —
the coordinator URL written into `/etc/overwatcher-agent.env` is wrong.
Either the coordinator's `agent.public_url` config is unset and the
installer fell back to the request's `Host` header (which may have been a
LAN address rather than the public URL), or the coordinator isn't reachable
from this VM. Set `AGENT_PUBLIC_URL` on the coordinator to fix it for
future installs; edit the env file on this host to fix it now.

**`unauthorized` from coordinator** — `AGENT_SHARED_SECRET` in the env file
doesn't match what the coordinator was started with. The shared secret is a
single global value on the coordinator side today; check
`AGENT_SHARED_SECRET` in the coordinator's environment.

**`permission denied` on the Docker socket** — the `overwatcher` user
isn't in the `docker` group. The installer adds it, but if Docker was
installed *after* the agent was installed, re-run the install one-liner.

## Why systemd vs. the Docker-image agent

The container agent has to translate between host paths and container
paths for every compose file it touches, and has to be handed the Docker
socket and `~/.docker/config.json` so it can pull private images. The
systemd agent runs as a host user, so there is nothing to translate — it
sees the same filesystem, the same registry credentials, and the same
Docker daemon as a human operator. Setup goes from a checklist of seven
bind-mounts and env vars to one curl|bash.
