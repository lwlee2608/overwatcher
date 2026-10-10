#!/bin/sh
# Install the coordinator's owctl release without requiring root.
set -eu

RELEASE_TAG='{{RELEASE_TAG}}'
INSTALL_DIR="${OWCTL_INSTALL_DIR:-$HOME/.local/bin}"

err() { printf 'error: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) err "unsupported operating system: $(uname -s)" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) err "unsupported architecture: $(uname -m)" ;;
esac

command -v curl >/dev/null 2>&1 || err 'curl is required'
if command -v sha256sum >/dev/null 2>&1; then
  CHECKSUM=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  CHECKSUM=shasum
else
  err 'sha256sum or shasum is required'
fi

ASSET="owctl_${OS}_${ARCH}"
BASE_URL="https://github.com/lwlee2608/overwatcher/releases/download/${RELEASE_TAG}"
if [ "$RELEASE_TAG" = latest ]; then
  BASE_URL='https://github.com/lwlee2608/overwatcher/releases/latest/download'
fi

TMP=$(mktemp -d)
STAGED=''
trap 'rm -rf "$TMP"; if [ -n "$STAGED" ]; then rm -f "$STAGED"; fi' 0
trap 'exit 1' HUP INT TERM

printf 'Downloading %s (%s)\n' "$ASSET" "$RELEASE_TAG"
curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP}/${ASSET}"
curl -fsSL "${BASE_URL}/SHA256SUMS" -o "${TMP}/SHA256SUMS"
# Select only this binary; the manifest also contains other platforms and agents.
awk -v asset="$ASSET" '$2 == asset { print }' "${TMP}/SHA256SUMS" > "${TMP}/checksum"
[ "$(wc -l < "${TMP}/checksum" | tr -d ' ')" = 1 ] || err "missing or duplicate checksum for ${ASSET}"
if [ "$CHECKSUM" = sha256sum ]; then
  (cd "$TMP" && sha256sum -c checksum) || err 'SHA256 verification failed'
else
  (cd "$TMP" && shasum -a 256 -c checksum) || err 'SHA256 verification failed'
fi

mkdir -p "$INSTALL_DIR"
STAGED=$(mktemp "${INSTALL_DIR}/.owctl.XXXXXX")
cp "${TMP}/${ASSET}" "$STAGED"
chmod 0755 "$STAGED"
mv -f "$STAGED" "${INSTALL_DIR}/owctl"
STAGED=''
printf 'Installed owctl to %s/owctl\n' "$INSTALL_DIR"
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) printf 'Add %s to your PATH to run owctl.\n' "$INSTALL_DIR" ;;
esac
