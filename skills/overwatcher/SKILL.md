---
name: overwatcher
description: Use when creating, configuring, or inspecting an Overwatcher deploy project through its API — setting the compose file, mapping GitHub repos to compose services, or installing and binding a VM agent so pushes auto-deploy.
user-invocable: true
---

# Managing Overwatcher Projects

Overwatcher deploys Docker Compose services on a VM when a GitHub repo is pushed or its CI workflow succeeds. This skill drives its HTTP API with a personal API key to set up a project end to end.

## Rules

1. **Read credentials from the environment.** Use `$OVERWATCHER_API_KEY` (starts with `owk_`). `$OVERWATCHER_URL` is optional and defaults to `https://overwatcher-web-production.up.railway.app` (base URL, no trailing `/api/v1`). If the key is missing, stop and ask the user — keys are created in the Overwatcher UI under user menu → API keys. Never echo the key or write it to a file.

2. **Call the API with this exact shape:**
   ```bash
   ow() { curl -sS -H "Authorization: Bearer $OVERWATCHER_API_KEY" -H "Content-Type: application/json" -X "$1" "${OVERWATCHER_URL:-https://overwatcher-web-production.up.railway.app}/api/v1$2" ${3:+--data} ${3:+"$3"}; }
   ow GET /projects
   ```
   Define `ow` in the same command as its calls — shell functions usually don't persist between agent tool calls. The split `${3:+…}` form is deliberate: it passes the JSON body as one argument in both bash and zsh. Errors come back as `{"error": "..."}`; report them verbatim.

3. **Inspect before creating.** Run `ow GET /projects` and `ow GET /agents` first. Reuse an existing project with the same name instead of creating a duplicate (names are unique per user; a duplicate returns 409).

4. **Create the project with an absolute host path to the compose file:**
   ```bash
   ow POST /projects '{"name":"my-app","compose_file":"/opt/stacks/my-app/docker-compose.yml","environment":"production"}'
   ```
   `compose_file` is the path on the VM where the agent runs `docker compose -f <path>`. Ask the user if unknown — do not guess.

5. **Set services with one replace call:**
   ```bash
   ow PUT /projects/$PROJECT_ID/services '{"services":[
     {"name":"web","repo":"acme/my-app","image":"ghcr.io/acme/my-app","tag":"latest","branch":"main","root_directory":"/","workflow":"build.yml"}
   ]}'
   ```
   - `name` **must equal the service key in the compose file** — the agent runs `docker compose pull <name>` / `up -d <name>`.
   - `repo` is `owner/repo`. `image` is required. Defaults: `branch` `main`, `tag` `latest`, `root_directory` `/`.
   - Set `workflow` (filename only, e.g. `build.yml`) when CI builds the image. Without it, deploy fires on push and can race CI, pulling a stale image.
   - `root_directory` limits push triggers to changes under that path (monorepos). Ignored for `workflow` triggers.
   - PUT replaces the whole list. To add one service without touching others, use `ow POST /projects/$PROJECT_ID/services '{...}'`.

6. **Install an agent on the VM when no free one exists.** The user is expected to have SSH access to the VM. Ask for the SSH target (e.g. `ubuntu@10.0.0.5`) and confirm it before running anything — the installer runs as root there. Then:
   ```bash
   ssh "$VM" 'systemctl is-active overwatcher-agent' # "active" means the VM already runs an agent: stop and ask the user
   ssh "$VM" 'docker compose version'                # must succeed; the installer needs Docker + the compose plugin
   ```
   Create the agent and install it **in one command**, so the token stays in a shell variable and is never printed or written to a file:
   ```bash
   R=$(ow POST /agents '{"name":"my-vm"}') && AGENT_ID=$(jq -er .agent_id <<<"$R") && TOKEN=$(jq -er .agent_token <<<"$R") || { echo "create failed: $R"; exit 1; }
   echo "agent_id=$AGENT_ID"
   ssh "$VM" "curl -fsSL ${OVERWATCHER_URL:-https://overwatcher-web-production.up.railway.app}/install.sh | sudo AGENT_TOKEN=$TOKEN bash"
   ```
   - The guard matters: API errors still exit 0, and without it the installer runs as root with the token `null`, leaving a broken agent on the VM. If create fails with 409, the name is taken — pick another and retry.
   - The SSH user needs passwordless sudo. If sudo prompts, stop and give the user the command to run themselves.
   - The agent runs as the SSH login user. Private image pulls need that user's `docker login` on the VM.
   - If the install fails, run `ow DELETE /agents/$AGENT_ID` so a never-connected agent is not left behind.
   - Poll `ow GET /agents/$AGENT_ID` every 5s for up to 1 minute until `status` is `connected`. The installer exits 0 even if the agent crashes on start, so a timeout is a failure: run `ssh "$VM" 'journalctl -u overwatcher-agent -n 50 --no-pager'`, report the output, and ask the user before deleting or binding the agent (deleting it leaves the service running on the VM).

7. **Bind an agent only if it is free.** Use the agent from step 6, or one from `ow GET /agents` whose `project_id` is empty:
   ```bash
   ow PUT /agents/$AGENT_ID/project "{\"project_id\":\"$PROJECT_ID\"}"
   ```
   Binding is 1:1 and moves silently: binding an agent that serves another project takes it away from that project, and binding a project that already has an agent unbinds the old one. Ask the user before touching a bound agent.

8. **Remind the user about GitHub.** The Overwatcher GitHub App must be installed on each `repo`, subscribed to `push` (and `workflow_run` when `workflow` is set). The API cannot check this.

## Verification procedure

1. `ow GET /projects/$PROJECT_ID` — confirm `compose_file`, `enabled: true`, and every service's `name`, `repo`, `image`, `workflow`.
2. `ow GET /agents/$AGENT_ID` — confirm `project_id` matches and `status` is `connected`. Any other status means the agent is not polling; deploys will queue but not run.
3. Report the project ID, agent name, and the trigger per service (`push` or `workflow_run: <file>`).
