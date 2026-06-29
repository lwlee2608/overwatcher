# Agent (systemd)

The systemd agent runs as a native Linux binary on the VM, talking to the
local Docker daemon the same way a human operator would. Use it for new
deployments — the older Docker-image agent stays supported but isn't the
recommended path.

## Install

On the target VM:

```bash
curl -fsSL https://<coordinator-host>/install.sh | \
sudo AGENT_TOKEN=owa_<token> \
bash
```

What this does:

1. Resolves the **run user** (see [Run user](#run-user) below) and adds it to
   the `docker` group. By default this is the invoking login user; the
   dedicated `overwatcher` system user is created only when you opt in.
2. Downloads `overwatcher-agent_linux_<arch>` from the configured GitHub
   release, verifies it against `SHA256SUMS`, installs it to
   `/usr/local/bin/overwatcher-agent`.
3. Writes `/etc/overwatcher-agent.env` (mode 0600, owned by the run user) with
   `AGENT_TOKEN`, `AGENT_COORDINATOR_URL`.
4. Writes `/etc/systemd/system/overwatcher-agent.service` with `User=` set to
   the run user.
5. Enables and starts the unit, prints the first ~20 log lines and the
   run-user trade-off.

The UI's agent dashboard has an "Install a new agent" card: name the agent,
and it provisions the agent, mints its token, and shows the full command
ready to paste — the token is part of the command. The token is shown once;
if you lose it, re-issue a new one from the agent's menu.

### Prerequisites

- Docker is installed and the `docker compose` plugin is available
  (`docker compose version` must succeed).
- The host runs systemd (any reasonably current Debian / Ubuntu / RHEL /
  Amazon Linux / Fedora works). SELinux-hardened distros are not yet
  supported — use the Docker-image agent there.

## Run user

The agent runs `docker compose` against a compose file that lives in some
operator's directory and pulls images using a per-user registry login. If the
agent runs as a *different* user than the one that owns those files and that
ran `docker login`, two things break that look like agent bugs but are really
ownership mismatches:

- **`.env` permission denied** — `docker compose` stats `.env` next to the
  compose file; a home-rooted deployment dir (`/home/<user>/…`) isn't
  traversable by another user even when the files are group-`docker` readable.
- **GHCR `unauthorized`** — registry auth is per-user
  (`$HOME/.docker/config.json`); a user that never ran `docker login` can't
  pull private images.

To avoid this, the installer **defaults to the invoking login user** — the
human who ran `sudo`. That user owns the deployment dir, has the registry
login, and is already in the `docker` group, so both failure classes
disappear with no manual `chmod` or credential copying.

Override with the `AGENT_RUN_USER` env var (passed to `sudo`, like
`AGENT_TOKEN`):

| `AGENT_RUN_USER`        | Result                                              |
|-------------------------|-----------------------------------------------------|
| unset (default)         | run as `$SUDO_USER` (the login user)                |
| `overwatcher`           | create a dedicated `nologin` system user if absent  |
| `<other existing user>` | run as that user (must already exist)               |

```bash
# opt into the dedicated, isolated service user
curl -fsSL https://<coordinator-host>/install.sh | \
sudo AGENT_RUN_USER=overwatcher AGENT_TOKEN=owa_<token> \
bash
```

### Trade-off

|              | login user (default)                       | `overwatcher` (opt-in)                          |
|--------------|--------------------------------------------|-------------------------------------------------|
| perms/creds  | zero friction — owns files, has GHCR login | must grant dir traverse + give it its own creds |
| isolation    | runs with that human's privileges          | `nologin`, `docker` group only                  |
| blast radius | larger — inherits sudo/wheel, shell, home  | smaller — no sudo, no human shell/home          |

Both users are root-equivalent via the `docker` group (a container can mount
the host filesystem), so the isolation win is partial — but `overwatcher` still
avoids the `sudo` group and doesn't entangle the agent with a human's
environment. If you pick `overwatcher`, you must run `docker login` as that
user and make every parent of the deployment dir traversable (`o+x`) and the
dir itself readable by it.

> Re-running the installer with a **different** run user is refused: switching
> would orphan the deployment files and registry creds owned by the old user.
> To switch intentionally, uninstall first (see below) and reinstall.

## Upgrade

Re-run the same install one-liner. The binary is swapped in place and the
unit restarted. The env file at `/etc/overwatcher-agent.env` is left
untouched — only edit it by hand if you need to change the token or the
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
| `AGENT_TOKEN`             | yes      | —                             |
| `AGENT_NAME`              | no       | — (descriptive label only)    |
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
sudo systemctl daemon-reload
```

Only delete the run user if the installer **created** it — i.e. you used
`AGENT_RUN_USER=overwatcher`. Never `userdel` a real login user.

```bash
sudo userdel overwatcher    # only if the dedicated user was created
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

**`unauthorized` (401) from coordinator** — `AGENT_TOKEN` in the env file
doesn't match any agent on the coordinator (revoked, mistyped, or the agent
row was deleted). Re-issue a token from the agent's menu in the dashboard and
update the env file, or provision a fresh agent.

**`permission denied` on the Docker socket** — the run user isn't in the
`docker` group. The installer adds it, but if Docker was installed *after*
the agent was installed, re-run the install one-liner.

**`unauthorized` pulling private images, or `.env permission denied`** — the
agent is running as a user that lacks the registry login or can't traverse the
deployment dir. This is the trap [Run user](#run-user) describes. The default
(login user) avoids it; if you opted into `overwatcher`, run `docker login` as
that user and make the deployment dir traversable/readable by it.

## Why systemd vs. the Docker-image agent

The container agent has to translate between host paths and container
paths for every compose file it touches, and has to be handed the Docker
socket and `~/.docker/config.json` so it can pull private images. The
systemd agent runs as a host user, so there is nothing to translate — it
sees the same filesystem, the same registry credentials, and the same
Docker daemon as a human operator. Setup goes from a checklist of seven
bind-mounts and env vars to one curl|bash.
