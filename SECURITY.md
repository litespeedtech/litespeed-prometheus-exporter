# Security Policy

## Supported versions

We provide security fixes for the latest released minor version on the `main`
branch. Older releases may receive critical fixes at our discretion. If you
are running an older version and discover a vulnerability, please upgrade to
the latest release first; if the issue persists there, follow the reporting
process below.

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < latest| :x:                |

## Reporting a vulnerability

**Please do not file public GitHub issues for suspected security
vulnerabilities.** Coordinated disclosure protects users.

Use one of the following channels:

1. **GitHub Private Vulnerability Reporting** (preferred):
   <https://github.com/litespeedtech/litespeed-prometheus-exporter/security/advisories/new>
2. **Email:** `support@litespeedtech.com` with subject line beginning
   `[lsws-prometheus-exporter]`. Encrypt with our PGP key if the report
   contains exploit details.

Please include:

- A description of the issue and its potential impact.
- Steps to reproduce, or a proof-of-concept.
- The version, OS, and kernel where you observed the issue.
- Whether you are willing to be credited in the release notes.

We aim to:

- Acknowledge receipt within **3 business days**.
- Provide an initial assessment within **7 business days**.
- Coordinate a fix and disclosure timeline together — typical embargo is
  30–90 days, longer for issues that require upstream LiteSpeed changes.

## Scope

In scope for this repository:

- The Go code under this repository (`main.go`, `collector/`).
- The release artifacts published on GitHub Releases for this repository.
- The installer scripts under `dist/` and the top-level `install.sh`.
- The release/build pipeline under `.github/workflows/`.

Out of scope (please report upstream):

- Vulnerabilities in the LiteSpeed Web Server itself (report to LiteSpeed
  Technologies directly).
- Vulnerabilities in Prometheus, Grafana, or other downstream consumers.
- Vulnerabilities in transitive Go dependencies — please report to the
  dependency's upstream first; we will pick up patched versions.
- Findings that require already-root access on the same host (the
  exporter runs as root and trusts local files; see the threat model in
  the README).
- Theoretical issues without a demonstrated security impact.

## Disclosure

Once a fix is released, we will:

- Publish a GitHub Security Advisory describing the issue and its impact.
- Credit reporters who consent to be named.
- Document the fix in the release notes / `README.md` changelog.

## Hardening guidance for operators

See the `Security considerations` section of [`README.md`](README.md) for a
hardening checklist (network restriction, TLS, systemd sandboxing,
artifact verification).
