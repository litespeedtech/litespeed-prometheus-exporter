# Release Process

This document describes how to cut a new release of
`lsws-prometheus-exporter`. The pipeline is mostly automated by GitHub
Actions; your job as releaser is to bump the version, verify locally, push
a tag, and confirm the workflow uploaded the artifacts.

> **TL;DR for an experienced operator**
>
> ```
> # 1. Bump VERSION in Makefile, update README "Notable changes"
> # 2. Locally verify
> make controller && go test -race ./... && go vet ./...
> # 3. Commit, tag, push
> git commit -am "Version X.Y.Z"
> git tag -s vX.Y.Z -m vX.Y.Z   # signed if you have a key, otherwise -a
> git push origin master
> git push origin vX.Y.Z
> # 4. Watch the release workflow on GitHub; verify the assets
> ```

---

## When to bump which digit

We follow [SemVer](https://semver.org/). For this project specifically:

| Change | Bump |
| --- | --- |
| Bug fix that doesn't change CLI flags or scrape output | **patch** (`X.Y.Z+1`) |
| New CLI flag, new metric, new scrape field, new install prompt — but old configs still work | **minor** (`X.Y+1.0`) |
| Removed/renamed CLI flag, removed metric, breaking change to install (e.g. systemd unit path move), Go directive bump that drops a supported distro | **major** (`X+1.0.0`) |

Security fixes that don't break the CLI surface go in patch or minor (call
it out clearly in the changelog with a `[Security]` tag).

---

## Pre-release checklist

Run all of these from a clean checkout of `master`:

1. **Pull latest:**

   ```
   git checkout master
   git pull --ff-only origin master
   ```

2. **Confirm working tree is clean:**

   ```
   git status            # should show "nothing to commit, working tree clean"
   ```

3. **Bump the version in `Makefile`:**

   ```
   sed -i 's/^VERSION="[0-9.]\+"/VERSION="X.Y.Z"/' Makefile
   ```

   Replace `X.Y.Z` with the new version (e.g. `0.2.1`).

4. **Update `README.md` "Notable changes"** with a new `### X.Y.Z` section.
   Group entries under `[Feature]`, `[Bug Fix]`, `[Security]`, `[Build]`,
   `[Compat]`, `[Ops]`. Be specific — operators read these to decide
   whether to upgrade.

5. **Run the full local verification:**

   ```
   go vet ./...
   go test -race -count=1 ./...
   make controller          # produces dist/lsws-prometheus-exporter
   VERSION=X.Y.Z ./mkdist.sh X.Y.Z
   sha256sum -c lsws-prometheus-exporter.X.Y.Z.tgz.sha256
   tar tzf lsws-prometheus-exporter.X.Y.Z.tgz | head    # spot check
   ```

   All must pass. If `go test -race` fails on a flaky test, do NOT release —
   investigate.

6. **(Optional but recommended)** Run [`govulncheck`](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck):

   ```
   go install golang.org/x/vuln/cmd/govulncheck@latest
   govulncheck ./...
   ```

   Address anything reported. The release workflow runs this too, but
   catching it locally saves a CI round-trip.

7. **Smoke-test the binary** (won't show LSWS metrics on a dev box but
   confirms the binary starts):

   ```
   ./litespeed-prometheus-exporter --metrics-service-addr=127.0.0.1:9936 --v=4 &
   curl -s http://127.0.0.1:9936/metrics | head
   curl -s http://127.0.0.1:9936/                # should return the home page
   curl -s -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:9936/   # 405
   kill %1
   ```

   If you bumped basic-auth code, additionally:

   ```
   echo -n 's3cret' > /tmp/pwd.txt && chmod 600 /tmp/pwd.txt
   ./litespeed-prometheus-exporter --username=alice --password-file=/tmp/pwd.txt &
   curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9936/metrics             # 401
   curl -su alice:wrong http://127.0.0.1:9936/metrics -o /dev/null -w '%{http_code}\n' # 401
   curl -su alice:s3cret http://127.0.0.1:9936/metrics | head                          # 200 + metrics
   kill %1; rm /tmp/pwd.txt
   ```

8. **Clean up build artifacts** before committing:

   ```
   rm -f litespeed-prometheus-exporter \
         lsws-prometheus-exporter.*.tgz \
         lsws-prometheus-exporter.*.tgz.sha256 \
         dist/lsws-prometheus-exporter
   git status                # should show only Makefile + README modifications
   ```

---

## Cutting the release

Once pre-release verification is green:

1. **Commit the version bump:**

   ```
   git add Makefile README.md
   git commit -m "Version X.Y.Z"
   ```

2. **Tag the release.** Prefer a signed tag (`-s`) if you have a GPG/SSH
   signing key configured; otherwise an annotated tag (`-a`) is acceptable:

   ```
   git tag -s vX.Y.Z -m vX.Y.Z          # signed (preferred)
   # or
   git tag -a vX.Y.Z -m vX.Y.Z          # annotated, unsigned
   ```

   **Never use a lightweight tag** (`git tag vX.Y.Z` with no flags) — Go
   modules and `go install` rely on annotated tags carrying metadata.

   **Never re-use a tag that's been pushed.** If you need to abandon a
   version, bump the patch and tag the new one. Re-pushing a tag breaks
   `go.sum` checksum locks for everyone who pulled the old one.

3. **Push the branch and tag in one step:**

   ```
   git push --atomic origin master vX.Y.Z
   ```

   The `--atomic` flag ensures both the commit and tag arrive together (or
   neither does).

---

## What happens automatically

Pushing the `vX.Y.Z` tag triggers `.github/workflows/release.yml`. It will:

1. Check out the tag.
2. Set up Go 1.25+ (pinned to `1.25` in `release.yml`; `check-latest: true` so the workflow grabs the newest 1.25.x patch).
3. Verify `go.mod`/`go.sum` are tidy (fails if you forgot `go mod tidy`).
4. Run `govulncheck ./...`.
5. Run `go test -race ./...`.
6. Build the static Linux/amd64 binary with release ldflags.
7. Run `mkdist.sh` to produce the `.tgz` and `.sha256` sidecar.
8. Generate a GitHub build-provenance attestation
   ([SLSA](https://slsa.dev/) for free).
9. Create a GitHub Release titled `vX.Y.Z` with auto-generated release
   notes, and upload:
   - `lsws-prometheus-exporter.X.Y.Z.tgz`
   - `lsws-prometheus-exporter.X.Y.Z.tgz.sha256`
   - `dist/lsws-prometheus-exporter` (the standalone binary)

Watch the run: <https://github.com/litespeedtech/litespeed-prometheus-exporter/actions>

---

## Post-release verification

Once the workflow succeeds:

1. **Visit the Release page:**
   <https://github.com/litespeedtech/litespeed-prometheus-exporter/releases/tag/vX.Y.Z>

   Confirm all three assets are attached.

2. **Verify the published checksum** (don't trust GitHub's UI; download
   and check):

   ```
   curl -fLO https://github.com/litespeedtech/litespeed-prometheus-exporter/releases/download/vX.Y.Z/lsws-prometheus-exporter.X.Y.Z.tgz
   curl -fLO https://github.com/litespeedtech/litespeed-prometheus-exporter/releases/download/vX.Y.Z/lsws-prometheus-exporter.X.Y.Z.tgz.sha256
   sha256sum -c lsws-prometheus-exporter.X.Y.Z.tgz.sha256
   ```

3. **Verify the build provenance attestation:**

   ```
   gh attestation verify lsws-prometheus-exporter.X.Y.Z.tgz \
     --repo litespeedtech/litespeed-prometheus-exporter
   ```

4. **Test the install flow end-to-end** on a disposable VM or container:

   ```
   curl -fsSL https://raw.githubusercontent.com/litespeedtech/litespeed-prometheus-exporter/main/install.sh \
     | sudo VERSION=X.Y.Z sh
   systemctl status lsws-prometheus-exporter
   curl http://127.0.0.1:9936/metrics
   ```

5. **Announce** in the appropriate channel (LiteSpeed forum, Slack,
   security advisory if applicable).

---

## Hotfix releases

If you need to ship a fix to an older release (e.g. a security fix to
`v0.2.x` while `master` is on `0.3.x`):

1. Branch from the old tag:

   ```
   git switch -c hotfix/0.2.1 v0.2.0
   ```

2. Apply the fix, run the full pre-release checklist on this branch.

3. Bump `Makefile` `VERSION` to `0.2.1`, update README changelog with a
   `### 0.2.1` section above `### 0.2.0`.

4. Commit, tag (`v0.2.1`), and push the tag. The release workflow runs
   regardless of which branch the tag points at.

5. **Don't merge the hotfix branch into `master` blindly.** Cherry-pick
   the actual fix commits onto `master` separately so master's history
   doesn't accumulate the version-bump commits.

---

## Yanking a bad release

If a release ships with a serious bug:

1. **Mark the GitHub Release as a draft or "pre-release"** so the install
   script's "latest" lookup skips it. Do NOT delete the release — that
   leaves consumers who already pulled it with no way to verify what they
   got.

2. **Keep the git tag.** Deleting tags breaks `go.sum` for everyone who
   pulled it.

3. **Cut a new patch release** with the fix as soon as possible.

4. **Publish a security advisory** if the issue is exploitable; see
   [`SECURITY.md`](SECURITY.md).

---

## Common pitfalls

- **Forgot `go mod tidy`?** The workflow catches this in step 3 and fails.
  Re-run locally, commit, force-push? **No** — bump the patch and tag a
  new release. Never force-push tags.
- **Forgot to update README "Notable changes"?** Workflow doesn't catch
  this. Add a follow-up commit *before* tagging.
- **Tagged the wrong commit?** Bump the patch and tag the right commit;
  do not move the existing tag.
- **Build worked locally but failed in CI?** Check `GOFLAGS`, `CGO_ENABLED`,
  and your local Go version vs the workflow's pinned 1.25. The workflow
  uses `check-latest: true` so it pulls the newest 1.25.x patch release.
  If your local Go is older than 1.21, `GOTOOLCHAIN=auto` won't trigger
  the auto-download; install a current toolchain first.
- **Provenance attestation step fails?** Most often a missing
  `attestations: write` permission in `release.yml`. Check the workflow
  permissions block at the top of the file.

---

## Glossary

- **Annotated tag** — `git tag -a` or `git tag -s`. Carries a tagger,
  date, and message. Required for Go modules.
- **Lightweight tag** — `git tag vX.Y.Z` with no flag. Just a pointer.
  Don't use for releases.
- **Provenance attestation** — a cryptographically signed claim that
  artifact `X` was built by workflow `Y` from commit `Z`. Lets consumers
  verify what they're running came from your CI, not from a maintainer's
  laptop or a compromised build server.
- **SHA-256 sidecar** — the `.sha256` file next to the tarball. The
  installer verifies this before extracting; reproducible builds let
  anyone regenerate it from source and compare.
