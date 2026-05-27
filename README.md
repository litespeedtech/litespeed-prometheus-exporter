# LiteSpeed Prometheus Exporter

The LiteSpeed Prometheus Exporter is a specially designed Prometheus application and uses the LiteSpeed Enterprise or the OpenLiteSpeed Web Server controller to export Prometheus compatible data which can also be used by Grafana and other compatible applications.

Besides giving useful information about LiteSpeed itself, it is an integral part of the LiteSpeed Containers product, in particular in exporting to Prometheus statistical information useful about individual user's resource consumption.  If LiteSpeed Containers are activated, cgroups information will be automatically exported.

## Installation

The exporter is distributed as a pre-built Linux/amd64 binary attached to each
[GitHub Release](https://github.com/litespeedtech/litespeed-prometheus-exporter/releases).
You must install it on the LiteSpeed machine you want to monitor; Prometheus
itself can run elsewhere.

There are three supported install paths, in order of convenience:

### Option 1 — One-line installer (recommended)

The `install.sh` at the root of this repository resolves the latest release,
downloads the tarball from GitHub, **verifies its SHA-256 checksum** against
the `.sha256` sidecar published alongside it, extracts it, and runs the
bundled service installer:

```
curl -fsSL https://raw.githubusercontent.com/litespeedtech/litespeed-prometheus-exporter/main/install.sh | sudo sh
```

To pin a specific version, set `VERSION`:

```
curl -fsSL https://raw.githubusercontent.com/litespeedtech/litespeed-prometheus-exporter/main/install.sh \
  | sudo VERSION=0.2.0 sh
```

Required tools on the host: `curl`, `tar`, and either `sha256sum` (coreutils)
or `shasum` (BSD / macOS).

### Option 2 — Manual download

If you'd rather not pipe a remote script to a shell, download and verify the
release tarball yourself. Replace `VERSION` with the version you want
(e.g. `0.2.0`):

```
VERSION=0.2.0
URL=https://github.com/litespeedtech/litespeed-prometheus-exporter/releases/download/v${VERSION}

curl -fLO ${URL}/lsws-prometheus-exporter.${VERSION}.tgz
curl -fLO ${URL}/lsws-prometheus-exporter.${VERSION}.tgz.sha256

sha256sum -c lsws-prometheus-exporter.${VERSION}.tgz.sha256

tar xf lsws-prometheus-exporter.${VERSION}.tgz
cd lsws-prometheus-exporter
sudo ./install.sh
```

Each release is also published with [build-provenance attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds),
which you can verify with the GitHub CLI:

```
gh attestation verify lsws-prometheus-exporter.${VERSION}.tgz \
  --repo litespeedtech/litespeed-prometheus-exporter
```

### Option 3 — Build from source

Requires Go 1.25 or newer. From a clone of this repository:

```
make controller
sudo ./dist/install.sh
```

`make controller` produces `litespeed-prometheus-exporter` at the repository
root and copies it to `dist/lsws-prometheus-exporter`. `make all` additionally
runs `mkdist.sh` to produce a `.tgz` you could distribute internally.

### Installer prompts

Whichever path you take, `install.sh` will then prompt:

```
Cert file name [ENTER for no HTTPS]:
```

Press **[ENTER]** to use plain HTTP (recommended only when the `:9936`
listener is firewalled to the Prometheus host — see *Security
considerations* below). To require HTTPS, supply a PEM-encoded certificate
path; you will then be prompted for a matching key path. The service is
installed and started automatically.

To remove the exporter later, run `sudo /usr/local/lsws-prometheus-exporter/uninstall.sh`.

## Security considerations

The exporter is a small Prometheus collector that reads LiteSpeed status
files from the local filesystem and exposes them over HTTP. It is not a
hardened, authenticated public API. Operators are responsible for
restricting network access. See [`SECURITY.md`](SECURITY.md) for vulnerability
reporting.

### Threat model

- **In scope:** robust parsing of LiteSpeed report files; safe handling of
  the local filesystem (PID files, report cleanup, TLS cert/key loading);
  HTTP server hardening against malformed requests, slowloris-style
  resource exhaustion, and reflected-input bugs; HTTP Basic auth with
  constant-time credential comparison.
- **Out of scope:** stronger auth schemes than Basic (use a reverse proxy
  for OAuth/mTLS), end-to-end transport secrecy on the loopback interface,
  and protection of the underlying LiteSpeed daemon.
- **Trust assumptions:** the LiteSpeed `.rtreport*` files, the LiteSpeed
  PID file, and the cgroup files under `/sys/fs/cgroup` are produced by
  the local LiteSpeed daemon (or the kernel) and are trusted inputs. The
  service runs as `root` by default to read these files. The
  `--password-file` is read once at startup and held in memory.

### Information disclosure

The `/metrics` endpoint exposes the LiteSpeed version string, the names of
configured virtual hosts, per-application pool internals, request rates,
and (when LiteSpeed Containers is enabled) per-UID resource consumption.
This is reconnaissance-grade data and **must not** be exposed to untrusted
networks.

### Recommended hardening checklist

1. **Bind locally or firewall the port.** The default listen address is
   `:9936` (all interfaces). If your Prometheus server runs on the same
   host, pass `--metrics-service-addr=127.0.0.1:9936`. Otherwise, restrict
   access with `iptables`/`nftables`/cloud security groups so only the
   Prometheus scraper can reach the port.
2. **Prefer HTTPS** when crossing untrusted networks (`--tls-cert-file`
   / `--tls-key-file`). The cert and key must be regular PEM files; the
   exporter validates this at startup.
3. **Enable built-in HTTP Basic authentication** with `--username` and
   `--password-file`, or add a reverse proxy enforcing OAuth/mTLS before
   exposing the exporter to a network you do not fully control. Built-in
   auth uses constant-time credential comparison and emits a
   `WWW-Authenticate: Basic realm="lsws-prometheus-exporter"` header on
   401 so Prometheus and other clients can negotiate. **The password file
   must be plain text, mode `0600`, and owned by the exporter's user.**
   Note that Basic auth without TLS sends credentials in clear text on
   the wire — pair it with `--tls-cert-file` / `--tls-key-file`.
4. **Run as a system user, not root.** While LiteSpeed often expects to be
   monitored by root, on hosts where the report files are readable by a
   dedicated user you can drop privileges via `User=` in the systemd unit.
5. **Lock down the systemd unit.** Add the following directives to
   `/etc/systemd/system/lsws-prometheus-exporter.service` under
   `[Service]`:

   ```ini
   NoNewPrivileges=true
   ProtectSystem=strict
   ProtectHome=true
   PrivateTmp=true
   PrivateDevices=true
   ProtectKernelTunables=true
   ProtectKernelModules=true
   ProtectControlGroups=true
   RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
   RestrictNamespaces=true
   LockPersonality=true
   MemoryDenyWriteExecute=true
   SystemCallArchitectures=native
   RuntimeDirectory=lsws-prometheus-exporter
   ```

   These are layered defenses; they don't replace network restrictions
   but greatly reduce blast radius if the process is compromised. After
   editing, run `systemctl daemon-reload && systemctl restart lsws-prometheus-exporter`.
6. **Verify release artifacts.** Always check the `.sha256` sidecar:

   ```
   sha256sum -c lsws-prometheus-exporter.${VERSION}.tgz.sha256
   ```

   For releases built by GitHub Actions, you can additionally verify the
   build provenance attestation:

   ```
   gh attestation verify lsws-prometheus-exporter.${VERSION}.tgz \
     --repo litespeedtech/litespeed-prometheus-exporter
   ```
7. **Subscribe to release notifications.** "Watch → Releases only" on
   GitHub so you get notified when new versions ship — many of those
   ship security fixes in dependencies.

## Configuring Prometheus

Prometheus is generally configured using the `prometheus.yml` file in the `prometheus` directory.  You should see the [Prometheus Configuration documentation](https://prometheus.io/docs/prometheus/latest/configuration/configuration/) for details.  To add a LiteSpeed server running on the local machine, add to the `scrape-configs:` section:

```
  - job_name: "litespeed_prometheus_exporter"
    static_configs:
      - targets: ["localhost:9936"]
    scrape_interval: 1m
```

A similar configuration but with the requirement of HTTPS (assuming you provided the cert and key files during exporter install):

```
  - job_name: "litespeed_prometheus_exporter"
    scheme: https
    static_configs:
      - targets: ["localhost:9936"]
    scrape_interval: 1m
```

If you also enabled HTTP Basic auth, add `basic_auth` to the scrape config
with the same username and password file you supplied to the exporter:

```yaml
  - job_name: "litespeed_prometheus_exporter"
    scheme: https
    basic_auth:
        username: 'USER'
        password_file: '/usr/local/lsws-prometheus-exporter/pwd.txt'
    static_configs:
      - targets: ["localhost:9936"]
    scrape_interval: 1m
```

If you use basic authentication you will need to add after `job_name` your user name and password file (values in single quotes).  These values should be the same as entered during installation.  For example for a username named USER and a password file named /usr/local/lsws-prometheus-exporter/pwd.txt you would specify after job_name:

```
    basic_auth:
        username: 'USER'
        password_file: '/usr/local/lsws-prometheus-exporter/pwd.txt'
```

## Metrics Exported

### Overall Metrics

The LiteSpeed metrics export includes the following overall metrics. In the `.rtreport` files, these metrics are at the top and don't repeat. For example:

```
VERSION: LiteSpeed Web Server/Enterprise/6.1.2
UPTIME: 02:56:01
BPS_IN: 0, BPS_OUT: 0, SSL_BPS_IN: 0, SSL_BPS_OUT: 0
MAXCONN: 10000, MAXSSL_CONN: 10000, PLAINCONN: 0, AVAILCONN: 10000, IDLECONN: 0, SSLCONN: 0, AVAILSSL: 10000
```

The titles to the table mean:

- **Name** is the Prometheus name for the metric.  Each name will have a `litespeed_` prefix.
- **Scraped Value** is the source from the `.rtreport` file the value originates from
- **Description** is a simple description of the meaning of the parameter.
- **Type** is either `Gauge` for values which can go up or down or `Counter` for values which can only go up.


| Name | Scraped Value | Description | Type |
| - | - | - | - |
| `litespeed_available_connections` | `AVAILCONN` | Available number of connections | Gauge |
| `litespeed_available_ssl_connections` | `AVAILSSL` | Available number of SSL (https) connections | Gauge |
| `litespeed_current_http_connections` | `PLAINCONN` | Current number of http connections | Gauge |
| `litespeed_current_idle_connections` | `IDLECONN` | Current number of idle connections | Gauge |
| `litespeed_current_ssl_connections` | `SSLCONN` | Current number of SSL (https) connections | Gauge |
| `litespeed_exporter_scrapes_failures_total` | - | The number of failed scrapes. | Counter |
| `litespeed_exporter_scrapes_total` | - | The total number of scrapes. | Counter |
| `litespeed_incoming_http_bytes_per_second` | `BPS_IN` | Incoming number of bytes per second over HTTP | Gauge |
| `litespeed_incoming_ssl_bytes_per_second` | `SSL_BPS_IN` | Incoming number of bytes per second over HTTPS | Gauge |
| `litespeed_maximum_http_connections` | `MAXCONN` | Maximum configured http connections | Counter |
| `litespeed_maximum_ssl_connections` | `MAXSSL_CONN` | Maximum configurations SSL (https) connections | Counter |
| `litespeed_outgoing_http_bytes_per_second` | `BPS_OUT` | Outgoing number of bytes per second over HTTP | Gauge |
| `litespeed_outgoing_ssl_bytes_per_second` | `SSL_BPS_OUT` | Outgoing number of bytes per second over HTTPS | Gauge |
| `litespeed_up` | - | Whether LiteSpeed is up or down (`1` or `0`) | Gauge |
| `litespeed_version` | `VERSION` | Returns whether LiteSpeed is up or down and the `version` field returns the text `LiteSpeed Web Server/Enterprise/6.1.2` | Gauge |

### VHost (REQRATE) Metrics 

The LiteSpeed metrics exported include the following VHost (virtual host) metrics.  In the `.rtreport*` files, these metrics repeat and have a `REQ_RATE` prefix with the first line representing the total and subsequent lines for VHosts which are defined and accessed in the conventional way.  For example:

```
REQ_RATE []: REQ_PROCESSING: 0, REQ_PER_SEC: 0.2, TOT_REQS: 10, PUB_CACHE_HITS_PER_SEC: 0.0, TOTAL_PUB_CACHE_HITS: 0, PRIVATE_CACHE_HITS_PER_SEC: 0.0, TOTAL_PRIVATE_CACHE_HITS: 0, STATIC_HITS_PER_SEC: 0.0, TOTAL_STATIC_HITS: 0
REQ_RATE [Example]: REQ_PROCESSING: 0, REQ_PER_SEC: 0.2, TOT_REQS: 10, PUB_CACHE_HITS_PER_SEC: 0.0, TOTAL_PUB_CACHE_HITS: 0, PRIVATE_CACHE_HITS_PER_SEC: 0.0, TOTAL_PRIVATE_CACHE_HITS: 0, STATIC_HITS_PER_SEC: 0.0, TOTAL_STATIC_HITS: 0
```

Note that in the Prometheus table each VHost, including the overall one will be assigned a separate line; in the graph, each VHost will be assigned a separate color.

Each Prometheus Name will include, besides the `litespeed_` prefix, a `_per_vhost` suffix.

| Name | Scraped Value | Description | Type |
| - | - | - | - |
| `litespeed_current_requests_per_vhost` | `REQ_PROCESSING` | Current number of requests in flight | Gauge |
| `litespeed_outgoing_bytes_per_second_per_vhost` | `BPS_OUT` | Current number of bytes per second outgoing.  Only available for configured VHosts | Gauge |
| `litespeed_private_cache_hits_per_second_per_vhost` | `PRIVATE_CACHE_HITS_PER_SEC` | Private cache hits per second | Gauge |
| `litespeed_private_cache_hits_per_vhost` | `TOTAL_PRIVATE_CACHE_HITS` | Total private cache hits | Counter |
| `litespeed_public_cache_hits_per_second_per_vhost` | `PUB_CACHE_HITS_PER_SEC` | Public cache hits per second | Gauge |
| `litespeed_public_cache_hits_per_vhost` | `TOTAL_PUB_CACHE_HITS` | Total public cache hits | Counter |
| `litespeed_requests_per_second_per_vhost` | `REQ_PER_SEC` | Requests per second | Gauge |
| `litespeed_static_hits_per_second_per_vhost` | `STATIC_HITS_PER_SEC` | Static file requests per second | Gauge |
| `litespeed_static_hits_per_vhost` | `TOTAL_STATIC_HITS` | Total number of static file hits | Counter |
| `litespeed_total_requests_per_vhost` | `TOT_REQS` | Total number of requests | Counter |


### Applications Metrics (EXTAPP)

LiteSpeed exports what is prefixed as external application metrics (`EXTAPP`).  There are 3 names in brackets before the metrics:

- The application type.  In the example below it's LSAPI
- The VHost (if the application is defined per VHost).
- The application name.  The application in the example below is a wsgiApp, which is a mechanism for Python applications.

```
EXTAPP [LSAPI] [] [wsgiApp]: CMAXCONN: 35, EMAXCONN: 35, POOL_SIZE: 1, INUSE_CONN: 0, IDLE_CONN: 1, WAITQUE_DEPTH: 0, REQ_PER_SEC: 0.1, TOT_REQS: 1
```

Each Prometheus Name will include, besides the `litespeed_` prefix, a `_per_app` suffix.

| Name | Scraped Value | Description | Type |
| - | - | - | - |
| `litespeed_config_max_connections_per_app` | `CMAXCONN` | Configured maximum number of connections | Gauge |
| `litespeed_connections_idle_per_app` | `IDLE_CONN` | Number of idle connections | Gauge |
| `litespeed_connections_in_use_per_app` | `INUSE_CONN` | Number of connections in use | Gauge |
| `litespeed_current_sessions_per_app` | `SESSIONS` | Current number of sessions | Gauge |
| `litespeed_pool_count_per_app` | `POOL_SIZE` | Total number of pools | Gauge |
| `litespeed_pool_max_connections_per_backend` | `EMAXCONN` | Maximum number of connections for the pool | Gauge |
| `litespeed_requests_per_second_per_backend` | `REQ_PER_SEC` | Number of requests per second | Gauge |
| `litespeed_total_requests_per_backend` | `TOT_REQS` | Total number of requests | Counter |
| `litespeed_wait_queue_depth_per_backend` | `WAITQUE_DEPTH` | Depth of the waiting queue | Gauge |

### CGroups metrics

CGroups metrics will be exported by default if LiteSpeed Containers is enabled and the system is capable of cgroups v2.  Metrics are exported in the following form:

```
   cgroups_PREFIX_SUFFIX
```

Where PREFIX is one of the following:
- **cpu**: CPU utilization statistics.
- **io**: Read and write utilization statistics.
- **memory**: Amount of memory utilization.
- **pids**: Number of tasks.
  
SUFFIX names are listed in each table below.

Statistics are in two forms: raw and calculated.  The calculated ones tend to be the most useful, however they are calculated using the raw statistics.

You are given each statistic with a `uid` qualifier.  The `uid` of `.` is used to represent the system as a whole.  All other uids are numeric system UIDs.

For example, there is an exported value `cgroups_cpu_difference_microseconds`.  It has a PREFIX of `cpu` and a SUFFIX of `difference_microseconds` and is documented below.

#### CPU prefix

| Suffix | Calculated | Description | Type |
| - | - | - | - |
| difference_microseconds | X | CPU difference in the last interval in microseconds per user | Gauge
| loadavg_percent |   | The contents of the /proc/loadavg file for the last minute for the system as a whole.  Not available for each uid. | Gauge
| microseconds | | Total CPU usage in microseconds per user. | Counter
| percent | X | CPU usage as a percent of microseconds used per user. | Gauge
| system_microseconds | | Kernel-space CPU usage in microseconds per user | Counter
| user_microseconds | | User-space CPU usage in microseconds per user | Counter

#### IO prefix

| Suffix | Calculated | Description | Type |
| - | - | - | - |
| op_per_second | X | Read and write operations per second per user | Gauge
| per_second | X | Read and written bytes per second per user | Gauge
| read_bytes | | Total bytes read per user | Counter
| reads_total | | Total number of reads per user | Counter
| write_bytes | | Total bytes written per user | Counter
| writes_total | | Total number of writes per user | Counter

#### Memory prefix

| Suffix | Calculated | Description | Type |
| - | - | - | - |
| bytes | | Total amount of memory currently being used per user | Gauge
| percent | X | Memory usage as a percent per user | Gauge
| swap_bytes | | Amount of swap memory currently being used per user | Gauge

#### Pids prefix

| Suffix | Calculated | Description | Type |
| - | - | - | - |
| percent | X | Number of tasks active as a percent per user | Gauge
| total | | Total number of tasks active per user | Gauge

## Configuring the Prometheus Exporter

The `lsws-prometheus-exporter` program is started as a service and it can be modified by updating the configuration in the service definition.  In a SystemD system (most systems), this will be a file in the `/etc/systemd/system` folder with the name `lsws-prometheus-exporter.service`.  To add a command line parameter, add it to the `ExecStart` definition after the program starts.  For example, if you installed the exporter with a certificate and key file pointing to the default LiteSpeed admin files you'd see:

```
ExecStart=/usr/local/lsws-prometheus-exporter/lsws-prometheus-exporter --tls-cert-file=/usr/local/lsws/admin/conf/webadmin.crt --tls-key-file=/usr/local/lsws/admin/conf/webadmin.key
```

### Command line parameters

| Name | Description | Default |
| - | - | - |
| `--cgroups` | Whether cgroups v2 user information will be collected.  0 requests disabling, 1 requests enabling if cgroups v2 and LiteSpeed Containers are enabled. | 1 |
| `--litespeed-home` | Home directory for LiteSpeed, if cgroups are enabled. | /usr/local/lsws |
| `--litespeed-pid-file` | LiteSpeed daemon PID file used for the `litespeed_up` probe. | /tmp/lshttpd/lshttpd.pid |
| `--metrics-excluded-list` | A comma separated list of metrics to exclude, using the Prometheus name without the prefix `litespeed_`. | None |
| `--metrics-service-addr` | The address and port to use to listen for prometheus collection requests.  Form: addr:port; a blank addr listens on all addresses. Set to `127.0.0.1:9936` if Prometheus runs on the same host. | `:9936` |
| `--metrics-service-path` | The HTTP path to service requests on. | `/metrics` |
| `--password-file` | Plain-text password file used with `--username` for HTTP Basic auth on `/metrics`. The file must be `chmod 0600` and owned by the exporter's user. | None |
| `--pid-directory` | Directory for the exporter's own PID file. Empty means `/run/lsws-prometheus-exporter` when writable, otherwise `/tmp/lsws-prometheus-exporter`. | (auto) |
| `--rtreport` | The fully qualified path to the LiteSpeed real time report file.  | /tmp/lshttpd/.rtreport |
| `--tls-cert-file` | If you want to require https to access metrics you must specify a `tls-cert-file` and a `tls-key-file` which are PEM encoded files | None |
| `--tls-key-file` | If you want to require https to access metrics you must specify a `tls-cert-file` and a `tls-key-file` which are PEM encoded files | None |
| `--username` | Username required for HTTP Basic auth on `/metrics`. Must be paired with `--password-file`. | None |
| `--v` | Sets info loggings.  `--v=4` is the most verbose. | `2` |

## Troubleshooting

The exporter writes its errors and important messages to standard output.  If you use the install script, this will have any messages written to the system log.  On SystemD systems, these are read using `journalctl`.

## Building the Exporter

The exporter requires **Go 1.25 or newer** to build. With Go's `GOTOOLCHAIN=auto`
default, any Go ≥ 1.21 toolchain will auto-download a matching 1.25.x
release on demand. If `GOTOOLCHAIN=local`, install Go 1.25 yourself.
The produced binary is statically linked (`CGO_ENABLED=0`) and runs on any
modern Linux kernel — including Ubuntu 20.04+, AlmaLinux 8/9, RHEL 8/9, and
Debian 11+.

Build steps:

```
make controller        # produces litespeed-prometheus-exporter at repo root
make all               # also produces lsws-prometheus-exporter.${VERSION}.tgz
```

The version number is set in the `Makefile`. `make all` runs `mkdist.sh`
which builds the tarball but **does not** auto-commit binaries or
manipulate git tags — releases are produced by the `release.yml` GitHub
Actions workflow on tag push.

### Running tests

```
go test ./...           # unit tests
go test -race ./...     # race detector
```

## Notable changes

### 0.2.0

> **Upgrading from 0.1.x?** The install procedure has changed. Read the
> [Installation](#installation) section above before running the new
> `install.sh` — in particular note the new one-line installer
> (`curl … | sudo sh`), the SHA-256 sidecar verification step, and the
> `gh attestation verify` build-provenance check.
>
> The bundled `dist/install.sh` now also prompts for an optional
> **basic-auth username and password file** in addition to the existing
> cert/key prompts. If you accept the default (ENTER), behaviour is
> identical to 0.1.3.
>
> The systemd unit produced by the installer now writes its PID file to
> `/run/lsws-prometheus-exporter/` (via `RuntimeDirectory=`) instead of
> `/tmp`. Existing 0.1.x installs are migrated automatically on the next
> `service start`. If you have custom scripts that read the old
> `/tmp/lsws-prometheus-exporter/lsws-prometheus-exporter.pid`, update
> them to read `/run/lsws-prometheus-exporter/lsws-prometheus-exporter.pid`.
>
> If you have an existing v0.1.4 systemd unit referring to
> `--password_file=…`, it will keep working — that spelling is accepted
> as an alias for the canonical `--password-file`.

- [Install] **New top-level `install.sh`** — a one-line
  `curl … | sudo sh` installer that resolves the latest release,
  downloads the tarball from GitHub Releases, verifies its SHA-256, and
  runs the bundled installer. See *Option 1 — One-line installer* in
  the [Installation](#installation) section. The previous "git clone +
  make" path still works (now documented as Option 3 — Build from
  source).
- [Install] **Manual download path** now includes a SHA-256 sidecar
  (`*.tgz.sha256`) and an optional `gh attestation verify` step. The
  release workflow generates SLSA build-provenance attestations that
  let consumers verify which CI run produced their binary.
- [Install] **`dist/install.sh` prompts for basic auth** in addition to
  cert/key. Accept the default (ENTER) for identical 0.1.x behaviour.
- [Feature] HTTP Basic authentication on `/metrics` via `--username` /
  `--password-file`. Credential check uses `crypto/subtle.ConstantTimeCompare`
  to defeat timing attacks. The 401 response sets a proper
  `WWW-Authenticate: Basic realm="..."` header.
- [Feature] Outer-bracket VHost name parser — vhosts whose names contain
  `[` or `]` are now reported correctly.
- [Security] HTTP server now sets read/header/write/idle timeouts and a
  64 KiB header cap to defeat Slowloris-style DoS.
- [Security] Default `/` handler now rejects non-`GET`/`HEAD` requests,
  returns `404` for unknown paths, and HTML-escapes `--metrics-service-path`
  before reflecting it. Uses a dedicated `http.ServeMux` instead of the
  global default mux (no more accidental pprof exposure on a transitive
  import).
- [Security] `cleanupBadFiles` no longer follows symlinks and confines
  deletions to the directory of `--rtreport`.
- [Security] PID file is created with mode `0600` using `O_EXCL` and
  prefers `/run/lsws-prometheus-exporter` over `/tmp` when available.
- [Security] TLS cert/key flags are validated as regular files; file
  descriptors no longer leak.
- [Security] Password file permissions are checked at startup; world- or
  group-readable files emit a warning when the exporter runs as root.
- [Security] **No credentials are ever logged**, at any verbosity.
- [Security] Replaced `prometheus.MustNewConstMetric` with
  `prometheus.NewConstMetric` + error log so a label cardinality bug can
  no longer panic the scrape goroutine.
- [Security] Bumped Go directive to `1.25` and refreshed dependencies.
  Releases are built with the latest Go 1.25.x patch release; at tag
  time, `govulncheck ./...` reports **zero** reachable stdlib CVEs.
  Picks up stdlib fixes accumulated across 1.22→1.25 (HTTP/2
  CONTINUATION flood, net/netip, net DNS, html/template, x509, gob,
  archive/zip, net/http chunked-reader, parser stack-exhaustion). The
  compiled binary is statically linked (`CGO_ENABLED=0`) and still runs
  on every Linux distro the v0.1.x line supported. Also bumps protobuf
  past CVE-2024-24786.
- [Feature] New flag `--litespeed-pid-file` to override the LSWS PID-file
  probe path.
- [Feature] New flag `--pid-directory` to override the exporter's own PID
  directory.
- [Build] Releases are now published via GitHub Actions with SHA-256
  sidecars and build-provenance attestations. The `mkdist.sh` script
  produces reproducible tarballs (sorted entries, fixed mtime, numeric
  owner) and emits a SHA-256 sidecar.
- [Compat] Accepts the legacy `--password_file` flag spelling from v0.1.4
  systemd units, but the canonical name is `--password-file`.
- [Ops] The bundled systemd unit now includes layered hardening
  (`NoNewPrivileges`, `ProtectSystem=strict`, `MemoryDenyWriteExecute`,
  `RestrictAddressFamilies`, `RestrictNamespaces`, `LockPersonality`,
  `SystemCallArchitectures=native`, etc.) and uses
  `RuntimeDirectory=lsws-prometheus-exporter` so the PID file lives
  under `/run` instead of `/tmp`. See *Security considerations* for the
  hardening checklist.
- [Docs] New `SECURITY.md` (vulnerability reporting policy, scope,
  embargo timeline) and `RELEASING.md` (full release procedure,
  including signed-tag guidance, GitHub Actions pipeline, post-release
  verification, and hotfix workflow).

### 0.1.4
- [Feature] Initial basic authentication support.
- [Bug Fix] Support nested brackets in the VHost name in REQ_RATE.

### 0.1.3
- [Feature] Make the location of the LiteSpeed real-time report file command line configurable

### 0.1.2
- [Bug Fix] Include missing scraped fields from the CMAXCONN line.
- The litespeed-containers branch was merged to master.

### 0.1.1
- [Bug Fix] Tolerate missing cgroups io.stat file.
- [Bug Fix] Work correctly if .tz file is exploded in place from clone.

### 0.1.0
- [Feature] Add cgroups support for LiteSpeed Containers.

### 0.0.2 
- [Feature] The install.sh script supports a "-n" flag to disable SSL file prompts.

