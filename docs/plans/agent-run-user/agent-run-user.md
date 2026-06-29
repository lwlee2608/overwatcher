# Agent run user

## Context

The systemd installer (`install-agent.sh`, served at `/install.sh`) always
creates a dedicated `overwatcher` system user (`--system --no-create-home
--shell /usr/sbin/nologin`), adds it to the `docker` group, and runs the
agent as that user:

```ini
[Service]
User=overwatcher
Group=docker
```

The agent then runs `docker compose` against a compose file that lives in
some operator's directory (e.g. `/home/dev/velo-deployment/`).

## Problem

Running as a *different* user than the one that owns the deployment files and
the registry login produces two failure classes that look like agent bugs but
are really ownership mismatches:

1. **`.env` traversal denied.** `docker compose` stats `.env` next to the
   compose file. The deployment dir is group-`docker` readable, but the path
   to it isn't traversable:

   ```
   namei -l /home/dev/velo-deployment/.env
   drwxr-x--- dev  dev    dev              ← overwatcher (uid 999, grp docker)
                                             is not dev, not in grp dev → BLOCKED
   drwxrwxr-x dev  docker velo-deployment  ✓ grp docker ok
   -rw-rw-r-- dev  docker .env             ✓ grp docker ok
   ```

   → `stat /home/dev/velo-deployment/.env: permission denied`

2. **GHCR `unauthorized`.** Docker registry auth is per-user
   (`$HOME/.docker/config.json`). The operator ran `docker login ghcr.io`;
   `overwatcher` has no docker config, so private pulls fail:

   ```
   Image ghcr.io/.../server:latest  error from registry: unauthorized
   ```

Both were hit in production on the `velo` VM and patched by hand
(`chmod o+x /home/dev`; copying the operator's `~/.docker/config.json` into
`/home/overwatcher/.docker/`). Every future install on a home-dir-rooted
deployment hits the same wall.

The common root cause: **agent user ≠ file/credential owner.**

## Solution

Default the agent to run as the **invoking login user** — the human who ran
`sudo`. That user owns the deployment dir, has the GHCR login, and is already
in the `docker` group, so both failure classes disappear without any manual
chmod/credential-copy.

Keep the dedicated `overwatcher` user available behind an explicit opt-in flag,
for operators who want process isolation and are willing to provision the
service user's own credentials and directory permissions.

```
why the default fixes things
────────────────────────────
run as $SUDO_USER  →  owns /home/<user>      → no .env traversal trap
                   →  uses ~/.docker/config  → GHCR auth already present
                   →  already in docker grp  → socket access works
```

### Detecting "the login user"

`curl | sudo … bash` runs the script as root, but sudo exports `$SUDO_USER` =
the invoking human. The installer resolves:

```
RUN_USER = ${AGENT_RUN_USER:-$SUDO_USER}
```

| `AGENT_RUN_USER`        | Result                                              |
|-------------------------|-----------------------------------------------------|
| unset (default)         | run as `$SUDO_USER` (the login user)                |
| `overwatcher`           | today's behavior — create system user if absent     |
| `<other existing user>` | run as that user (must already exist)               |

Passing `AGENT_RUN_USER=overwatcher` reproduces the current install exactly,
so this is backward-compatible.

The flag is a **runtime env var** passed to `sudo`, like `AGENT_TOKEN` — not a
template placeholder. So the coordinator handler (`install.go`) needs **no
change**.

### Trade-off the operator is choosing

|              | login user (default)                       | overwatcher (opt-in)                          |
|--------------|--------------------------------------------|-----------------------------------------------|
| perms/creds  | zero friction — owns files, has GHCR login | must grant dir traverse + give service its own creds |
| isolation    | runs with that human's privileges          | `nologin`, `docker` group only                |
| blast radius | larger — inherits sudo/wheel, shell, home  | smaller — no sudo, no human shell/home         |

Both users are root-equivalent via the `docker` group (a container can mount
the host fs), so the isolation win is partial — but `overwatcher` still avoids
the `sudo` group and doesn't entangle the agent with a human's environment.
The installer must **print this trade-off** at install time, not bury it in docs.

## Implementation

### 1. `install-agent.sh` (bulk of the work)

- Resolve `RUN_USER=${AGENT_RUN_USER:-$SUDO_USER}`.
- **Guards:**
  - error if `RUN_USER` resolves empty (raw root, no `SUDO_USER`) — tell the
    operator to pass `AGENT_RUN_USER` explicitly.
  - refuse `RUN_USER=root` (running the agent as root is worse than either
    intended path).
  - require the user to exist (`getent passwd "$RUN_USER"`) — **only**
    auto-create when `RUN_USER == overwatcher` (preserve the existing
    `useradd --system --no-create-home --shell /usr/sbin/nologin` path).
- `usermod -aG docker "$RUN_USER"` for any run user (idempotent).
- **Template the unit:** `User=${RUN_USER}` instead of hardcoded `overwatcher`;
  keep `Group=docker`. Requires switching the unit heredoc from quoted
  (`<<'UNIT'`) to interpolated, or `sed`-ing the value in.
- **chown the env file** `${RUN_USER}:${RUN_USER}` (or `:docker`), mode 0600 as
  today.
- **Print the security warning** based on the chosen path:
  - login user — agent now runs with that human's privileges; call out if the
    user is in `sudo`/`wheel`.
  - `overwatcher` — remind it needs its *own* GHCR creds and a deployment dir
    it can traverse (the trap above).
- **Upgrade safety — refuse on run-user change.** On re-run, read the existing
  unit's `User=`; if it differs from `RUN_USER`, **hard-fail with remediation**
  (mirrors the existing stale-token guard). Silently switching users orphans
  file ownership and registry creds under the old user.

### 2. `InstallAgentCard.tsx` (systemd tab only)

- Add a "Run as" selector: **Current login user (default)** vs **Dedicated
  `overwatcher` user**.
- Dedicated → append `AGENT_RUN_USER=overwatcher \` to the `sudo` line in
  `buildSystemd`; default → emit nothing extra.
- Add an inline amber warning summarizing the trade-off.
- Fix the now-inaccurate copy ("The installer adds a system user…") — the
  default path no longer adds a user.

### 3. Docs — `docs/architecture/agent-systemd.md`

- Step 1 ("Creates an `overwatcher` user") becomes conditional on the run user.
- Document `AGENT_RUN_USER`, the new default, and the trade-off table.
- Uninstall: don't `userdel` if the agent ran as a real login user.

### 4. Backend — `install.go`

No change. `AGENT_RUN_USER` flows through at runtime, not as a template var.

## Decisions

- **Upgrade with a different run user → refuse + remediation** (not warn,
  not silent rewrite). Avoids orphaning files/creds under the old user.
- **Dashboard defaults the selector to the login user**, matching the script
  default; `overwatcher` is the opt-in behind the warning.

## Out of scope

- Container-side permissions (e.g. a `root:root data/` volume a non-root
  container can't write) — independent of which user the agent runs as.
- The Docker-image agent tab — already runs as root-in-container; unaffected.
- Replacing the shared operator PAT with a dedicated machine token for the
  agent — orthogonal credential-hygiene improvement, tracked separately.
