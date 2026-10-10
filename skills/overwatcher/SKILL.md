---
name: overwatcher
description: Use when creating, configuring, or inspecting an Overwatcher deploy project with owctl — setting the compose file, mapping GitHub repos to compose services, or installing and binding a VM agent so pushes auto-deploy.
user-invocable: true
---

# Managing Overwatcher Projects

Overwatcher deploys Docker Compose services on a VM when a GitHub repo is pushed or its CI workflow succeeds. Use `owctl` with environment-based authentication for this workflow; do not assemble curl API calls or handle agent tokens yourself.

## Rules

1. **Require environment credentials.** Have the user supply `OVERWATCHER_API_KEY` (starts with `owk_`) through the session environment. If missing, stop and ask them to create a key under **user menu → API keys** and supply it securely. Never echo it, include it in tool arguments, or write it to a file. Do not run `owctl login` for this workflow: login saves credentials. Disable shell tracing. Set the coordinator base URL explicitly to avoid accidentally using a saved login's URL:
   ```sh
   export OVERWATCHER_URL="${OVERWATCHER_URL:-https://overwatcher-web-production.up.railway.app}"
   ```
   Use a base URL without `/api/v1`. Ensure both environment variables reach every tool invocation; shell exports may not persist between calls. Nonempty environment settings override saved owctl settings; `--url` overrides the URL.

2. **Install owctl if missing.** On Linux or macOS (amd64/arm64):
   ```sh
   curl -fsSL "${OVERWATCHER_URL:-https://overwatcher-web-production.up.railway.app}/cli.sh" | sh
   export PATH="$HOME/.local/bin:$PATH"
   owctl version --json
   ```
   The installer checks SHA256 checksums and installs to `~/.local/bin` without sudo, matching the coordinator's release (development coordinators use latest). Keep that directory on PATH in subsequent calls. Re-run after coordinator upgrades. Curl is only needed for CLI bootstrap; use owctl for API operations. If the installer fails (for example a 404 for the release asset), stop and report it; do not build owctl from source unless the user asks.

3. **Inspect before changing anything.** Use `--json` for raw API JSON on stdout; errors exit nonzero and appear on stderr. Stop on errors and report them without exposing secrets.
   ```sh
   owctl project list --json
   owctl agent list --json
   ```
   Reuse an existing project with the same name rather than creating a duplicate. Project references accept names or IDs; use the ID if a name is ambiguous. Inspect an existing project's complete configuration before changing it:
   ```sh
   owctl project get my-app --json
   ```

4. **Inspect the VM before asking setup questions.** First ask only for the VM's SSH target (`user@host`, IP, or `~/.ssh/config` alias). Resolve aliases with `ssh -G <target>`; do not grep SSH config files. Then run this read-only probe:
   ```sh
   ssh -T -o BatchMode=yes <target> sh -s <<'EOF'
   echo "user: $(id -un)"
   sudo -n true 2>/dev/null && echo "sudo: passwordless" || echo "sudo: needs password"
   command -v systemctl >/dev/null && echo "systemd: ok" || echo "systemd: missing"
   command -v curl >/dev/null && echo "curl: ok" || echo "curl: missing"
   systemctl show -p LoadState -p ActiveState overwatcher-agent 2>/dev/null
   docker compose version 2>/dev/null || echo "compose v2: missing"
   docker compose ls --all 2>/dev/null || echo "docker: no access for $(id -un)"
   EOF
   ```
   If SSH itself fails, report the error and ask the user to fix access; do not guess another target. The remote login shell may be zsh, where an unmatched glob aborts the whole command: send further remote checks through `sh -s` and quote globs locally too. Do not read agent config, env files, or service logs on the VM, even filtered through `grep -v`: they can contain the agent token. Then ask the user only what the probe and `owctl` output could not answer, stating findings with each question:
   - `LoadState=loaded` means an agent is already installed. Match it against `agent list` (name defaults to the SSH host) and offer to bind it instead of reinstalling. Ask before touching an inactive or failed installation.
   - Offer the `CONFIG FILES` from `docker compose ls` as compose path choices.
   - Report missing prerequisites (passwordless sudo, systemd, curl, Compose v2, docker access for the SSH user) and stop until the user fixes them or chooses another VM.

5. **Create only with a confirmed compose path.** Have the user confirm the absolute path on the VM; do not guess. The file must already exist there; owctl does not upload it. Check it with `ssh <target> test -r <path>`.
   ```sh
   owctl project create my-app \
     --compose-file /opt/stacks/my-app/docker-compose.yml \
     --environment production --json
   ```

6. **Derive services from the app repo, then replace deliberately.** For each compose service built from the repo, read its fields from the source rather than using defaults: `image` and tag variable from the compose `image:` line, `workflow` from the CI file that pushes that image, `tag` from the tags it pushes, and `root_directory` from its build `context`. Use the user's existing projects (`owctl project get`) as a reference pattern. Write `services.yaml` with the complete desired list, preserving existing services unless their removal was requested:
   ```yaml
   services:
     - name: web
       repo: acme/my-app
       image: ghcr.io/acme/my-app
       tag: latest
       branch: main
       root_directory: /
       workflow: build.yml
   ```
   ```sh
   owctl service set my-app -f services.yaml --json
   ```
   JSON files also work; `-f -` reads stdin. This is the services request body, not a Docker Compose file. It replaces the whole list; `services: []` clears it. There is no CLI append command.
   - `name` must equal the compose service key: the agent runs `docker compose pull <name>` / `up -d <name>`.
   - `repo` is `owner/repo`; `image` is required. Defaults are `branch: main`, `tag: latest`, `root_directory: /`.
   - Set `workflow` to a filename such as `build.yml` when CI builds the image. Without it, deployment fires on push and can pull a stale image before CI finishes.
   - In monorepos, set `root_directory` to the service's source directory, never `/`. It filters push triggers; workflow triggers ignore it.
   - If one workflow builds several images, every successful run redeploys all services mapped to it. Tell the user; per-image workflows avoid this.

7. **Bind only with consent to any displacement.** Read `agent list --json`: choose a free agent (`project_id` absent or empty) and check whether another agent already serves the project. Binding silently moves an agent away from its old project and replaces a project's previous agent. Ask before either displacement, including when installing a new agent with `--project`.
   ```sh
   owctl agent bind <agent-id> my-app --json
   ```

8. **Install over SSH when no suitable free agent exists.** Confirm the probed target, project, and permission to install a root-managed systemd service before running:
   ```sh
   owctl agent install --ssh ubuntu@vm --project my-app --json
   ```
   The CLI prints the SSH target and prompts unless `--yes` is passed. For non-interactive execution, add `--yes` only after the user has explicitly approved that target and operation.
   - Use a non-root SSH login with passwordless sudo, working non-interactive SSH, systemd, and Docker Compose v2. The VM needs curl and access to the coordinator and release downloads. System SSH inherits config aliases, ssh-agent, ProxyJump, and known_hosts; do not bypass host-key verification.
   - Preflight refuses an active `overwatcher-agent` and requires `docker compose`. It does not prove an inactive installation is absent; ask before retrying or altering an existing installation.
   - `--name` defaults to the SSH host. `--timeout 60s` is the default total budget for preflight, installation, and connection. `--project` requires project ownership; omitting it leaves the new agent unbound.
   - owctl creates the agent, sends its token through SSH stdin (not command arguments or output), waits for a heartbeat, and binds it. The installer persists the agent token on the VM for the service; the personal API key is not sent to the VM. Remote installer output is suppressed to prevent token leaks.
   - On install, connection, or bind failure, owctl attempts to revoke the created agent. Cleanup can fail: report that error and agent ID. Remote files or a service may remain even after successful revocation; do not claim rollback or blindly retry. Ask the user to inspect/clean up through the UI and the systemd troubleshooting docs before retrying. owctl has no agent delete/uninstall command.
   - The agent runs as the SSH login user. Private image pulls need that user's registry login on the VM; the compose path must be readable by that user.

9. **Confirm GitHub prerequisites.** The Overwatcher GitHub App must be installed on each repo and subscribed to `push` (and `workflow_run` when configured). owctl cannot verify this; remind the user.

## Verification procedure

1. Run `owctl project get my-app --json`. Confirm `compose_file`, `enabled: true`, and every service's `name`, `repo`, `image`, and `workflow`.
2. Run `owctl agent list --json`. In `agents`, confirm the intended agent's `project_id` equals the project ID and `status` is `connected`. Project output does not include agent status. Any other status needs investigation before claiming deployments will run.
3. Report the project ID, agent name, and trigger per service (`push` or `workflow_run: <file>`). Configuration and connection checks do not prove an actual deployment succeeded.

## Common mistakes

- Inventing unsupported commands: this CLI covers projects, service replacement, and agent list/bind/install, not deployments, events, or users.
- Replacing services without preserving the existing list, or binding without checking both sides for displacement.
- Treating a failed install as a full VM rollback, or dumping remote logs/configuration that could expose tokens. Filtering `journalctl` output with `grep -v token` is not redaction.
- Asking the user for facts the VM probe, repo, or existing projects already answer.
- Leaving `root_directory: /` for services in a monorepo.
