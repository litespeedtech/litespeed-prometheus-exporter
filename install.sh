#!/bin/sh
# One-liner installer for the LiteSpeed Prometheus Exporter.
#
#   curl -fsSL https://raw.githubusercontent.com/litespeedtech/litespeed-prometheus-exporter/main/install.sh | sh
#
# Or, to pin a specific version:
#
#   curl -fsSL https://raw.githubusercontent.com/litespeedtech/litespeed-prometheus-exporter/main/install.sh | VERSION=0.1.4 sh
#
# The script downloads the release tarball from GitHub, verifies its SHA-256
# against the .sha256 sidecar published alongside it, extracts it, and runs
# the contained `install.sh` (which sets up the systemd / init.d service).
#
# Requires: curl, tar, sha256sum (or shasum).
set -eu

REPO="${REPO:-litespeedtech/litespeed-prometheus-exporter}"
VERSION="${VERSION:-}"
INSTALL_DIR="${INSTALL_DIR:-/tmp/lsws-prometheus-exporter-install.$$}"

err() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
log() { printf '==> %s\n' "$*"; }

need() {
    command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

need curl
need tar
if command -v sha256sum >/dev/null 2>&1; then
    SHA="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
    SHA="shasum -a 256"
else
    err "need either sha256sum or shasum"
fi

if [ -z "${VERSION}" ]; then
    log "Resolving latest release for ${REPO}"
    LATEST_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")"
    VERSION="$(printf '%s\n' "$LATEST_JSON" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\{0,1\}\([^"]*\)".*/\1/p' | head -n1)"
    [ -n "${VERSION}" ] || err "could not resolve latest release tag"
fi

# Strip a leading 'v' if a user passed it in.
VERSION="${VERSION#v}"

TARBALL="lsws-prometheus-exporter.${VERSION}.tgz"
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"

log "Installing v${VERSION} from ${BASE_URL}"

mkdir -p "${INSTALL_DIR}"
trap 'rm -rf "${INSTALL_DIR}"' EXIT INT TERM

cd "${INSTALL_DIR}"

log "Downloading ${TARBALL}"
curl -fsSL -o "${TARBALL}"        "${BASE_URL}/${TARBALL}"
curl -fsSL -o "${TARBALL}.sha256" "${BASE_URL}/${TARBALL}.sha256"

log "Verifying SHA-256"
# The sidecar produced by sha256sum has the form "<hex>  <filename>". Some
# release tools rewrite the filename, so we compute and compare the hex
# directly rather than relying on `sha256sum -c`.
EXPECTED="$(awk '{print $1; exit}' "${TARBALL}.sha256")"
ACTUAL="$(${SHA} "${TARBALL}" | awk '{print $1}')"
[ "${EXPECTED}" = "${ACTUAL}" ] || err "checksum mismatch (expected ${EXPECTED}, got ${ACTUAL})"
log "SHA-256 OK: ${ACTUAL}"

log "Extracting ${TARBALL}"
tar xzf "${TARBALL}"
[ -d lsws-prometheus-exporter ] || err "tarball did not contain expected lsws-prometheus-exporter/ directory"

cd lsws-prometheus-exporter
[ -x ./install.sh ] || err "bundled install.sh not found or not executable"

log "Running bundled install.sh (you may be prompted for cert/key paths)"
exec ./install.sh "$@"
