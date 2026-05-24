#!/usr/bin/env bash
# Overwatcher agent installer (systemd).
#
# Usage (typically served by the coordinator at /install.sh with placeholders
# already substituted):
#
#   curl -fsSL https://<coordinator>/install.sh | \
#   sudo AGENT_NAME=my-agent \
#   AGENT_SHARED_SECRET=<secret> \
#   bash
#
# (The env vars must be passed to `sudo`, not to `curl` — a `VAR=val cmd1 |
# cmd2` shell pipeline scopes the vars to cmd1 only, so they never reach
# sudo. Putting them after `sudo` uses sudo's own VAR=val syntax, which
# sets them in the child process regardless of env_reset.)
#
# Re-running upgrades the binary in place: the file is swapped and the unit
# is restarted; the env file is left untouched if it already exists.
#
# Template variables (substituted by the coordinator before serving):
#   {{COORDINATOR_URL}} — the URL agents should poll
#   {{RELEASE_TAG}}     — GitHub release tag to install (e.g. "latest" or "agent-v0.3.0")

set -euo pipefail

COORDINATOR_URL="{{COORDINATOR_URL}}"
RELEASE_TAG="{{RELEASE_TAG}}"
REPO="lwlee2608/overwatcher"

SERVICE_NAME="overwatcher-agent"
BINARY_PATH="/usr/local/bin/${SERVICE_NAME}"
ENV_FILE="/etc/${SERVICE_NAME}.env"
UNIT_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
USER_NAME="overwatcher"

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
err() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || err "must run as root (try: curl ... | sudo AGENT_NAME=... AGENT_SHARED_SECRET=... bash)"
[[ -n "${AGENT_NAME:-}" ]] || err "AGENT_NAME must be set (pass it to sudo: curl ... | sudo AGENT_NAME=foo AGENT_SHARED_SECRET=bar bash)"
[[ -n "${AGENT_SHARED_SECRET:-}" ]] || err "AGENT_SHARED_SECRET must be set (pass it to sudo: curl ... | sudo AGENT_NAME=foo AGENT_SHARED_SECRET=bar bash)"

# Detect docker compose plugin upfront — failing here is much friendlier
# than the agent erroring on every deploy attempt later.
if ! command -v docker >/dev/null 2>&1; then
  err "docker is not installed. Install Docker before running this script."
fi
if ! docker compose version >/dev/null 2>&1; then
  err "docker compose plugin is missing. On Debian/Ubuntu: sudo apt install docker-compose-plugin"
fi

case "$(uname -m)" in
  x86_64|amd64)  ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) err "unsupported architecture: $(uname -m)" ;;
esac

ASSET="overwatcher-agent_linux_${ARCH}"
if [[ "$RELEASE_TAG" == "latest" ]]; then
  BASE_URL="https://github.com/${REPO}/releases/latest/download"
else
  BASE_URL="https://github.com/${REPO}/releases/download/${RELEASE_TAG}"
fi

log "creating ${USER_NAME} system user (if absent)"
if ! id -u "$USER_NAME" >/dev/null 2>&1; then
  useradd --system --no-create-home --shell /usr/sbin/nologin "$USER_NAME"
fi
# `docker` group lets the agent talk to /var/run/docker.sock without root.
if getent group docker >/dev/null 2>&1; then
  usermod -aG docker "$USER_NAME"
else
  err "docker group not found — is Docker installed correctly?"
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

log "downloading ${ASSET} from ${RELEASE_TAG}"
curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP}/${ASSET}"
curl -fsSL "${BASE_URL}/SHA256SUMS" -o "${TMP}/SHA256SUMS"

log "verifying SHA256"
(cd "$TMP" && grep " ${ASSET}\$" SHA256SUMS | sha256sum -c -) \
  || err "SHA256 mismatch for ${ASSET}"

log "installing binary to ${BINARY_PATH}"
install -m 0755 -o root -g root "${TMP}/${ASSET}" "${BINARY_PATH}"

if [[ -f "$ENV_FILE" ]]; then
  log "env file already exists at ${ENV_FILE}; leaving untouched"
else
  log "writing ${ENV_FILE}"
  umask 077
  cat > "$ENV_FILE" <<EOF
AGENT_NAME=${AGENT_NAME}
AGENT_SHARED_SECRET=${AGENT_SHARED_SECRET}
AGENT_COORDINATOR_URL=${COORDINATOR_URL}
EOF
  chown "${USER_NAME}:${USER_NAME}" "$ENV_FILE"
  chmod 0600 "$ENV_FILE"
fi

log "writing ${UNIT_FILE}"
cat > "$UNIT_FILE" <<'UNIT'
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
UNIT

log "enabling and starting ${SERVICE_NAME}"
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
# restart (not start) so re-runs pick up the swapped binary
systemctl restart "${SERVICE_NAME}"

# Give the unit a moment to settle, then surface recent logs so the user sees
# success (or fails fast).
sleep 1
log "recent log lines:"
journalctl -u "${SERVICE_NAME}" -n 20 --no-pager || true

log "done. follow logs with:  journalctl -u ${SERVICE_NAME} -f"
