#!/bin/bash
# Build the release tarball. Does NOT auto-commit binaries; does NOT delete
# existing tags. Intended to be run from CI (or locally) after `make controller`.

set -euo pipefail

VERSION="${VERSION:-${1:-}}"
if [[ -z "${VERSION}" ]]; then
    echo "[ERROR] VERSION is required (env var or first positional arg)" >&2
    exit 1
fi
if ! [[ "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][A-Za-z0-9._-]+)?$ ]]; then
    echo "[ERROR] VERSION '${VERSION}' is not a valid semver-ish string" >&2
    exit 1
fi

echo "Creating distribution tarball for v${VERSION}"

if [[ ! -d dist ]]; then
    echo "[ERROR] dist/ directory not found; run 'make controller' first" >&2
    exit 1
fi
if [[ ! -x dist/lsws-prometheus-exporter ]]; then
    echo "[ERROR] dist/lsws-prometheus-exporter not built; run 'make controller' first" >&2
    exit 1
fi

# Remove any old tarballs and sidecars from the working tree (NOT from git)
# before rebuilding.
rm -f "lsws-prometheus-exporter."*.tgz "lsws-prometheus-exporter."*.tgz.sha256

TARBALL="lsws-prometheus-exporter.${VERSION}.tgz"

# --sort=name and --mtime / --owner / --group make the tarball reproducible:
# byte-identical output for byte-identical inputs, regardless of who built it
# or when. This lets consumers re-build and `cmp` against the published asset.
tar czf "${TARBALL}" \
    --transform 's,^dist,lsws-prometheus-exporter,' \
    --sort=name \
    --owner=0 --group=0 --numeric-owner \
    --mtime='UTC 2020-01-01' \
    dist

# Generate the SHA-256 sidecar so local builds and CI builds are
# interchangeable, and so install.sh has something to verify against.
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${TARBALL}" > "${TARBALL}.sha256"
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${TARBALL}" > "${TARBALL}.sha256"
else
    echo "[WARN] neither sha256sum nor shasum available; no .sha256 sidecar written" >&2
fi

echo "Wrote ${TARBALL}"
if [[ -f "${TARBALL}.sha256" ]]; then
    echo "Wrote ${TARBALL}.sha256"
    echo "  $(cat "${TARBALL}.sha256")"
fi
echo
echo "Next steps (manual, intentionally not automated):"
echo "  1. Verify the tarball:    tar tzf ${TARBALL}"
echo "  2. Verify the checksum:   sha256sum -c ${TARBALL}.sha256"
echo "  3. Tag the release:       git tag -a v${VERSION} -m v${VERSION} && git push --tags"
echo "  4. Upload ${TARBALL} and ${TARBALL}.sha256 as GitHub Release assets;"
echo "     do NOT commit them to git."
