# LiteSpeed Prometheus Exporter

The LiteSpeed Prometheus Exporter is a specially designed Prometheus application and uses the LiteSpeed Enterprise or the OpenLiteSpeed Web Server controller to export Prometheus compatible data which can also be used by Grafana and other compatible applications.

Besides giving useful information about LiteSpeed itself, it is an integral part of the LiteSpeed Containers product, in particular in exporting to Prometheus statistical information useful about individual user's resource consumption.  If LiteSpeed Containers are activated, cgroups information will be automatically exported.

## Installation

These installation instructions assume you're downloading the compressed binary and installing from there, which will work for most x64 environments.  You can also build the package from the included Makefile if you need it on another architecture.

You must install the exporter on the LiteSpeed machine where the information is to be exported from.  Prometheus can run on another machine, but this software must be installed on the LiteSpeed machine to be monitored.

From a command prompt, after downloading the software, use the tar command to extract the binary (substitute `VERSION` with the actual version of the software):

```
tar xf lsws-prometheus-exporter.VERSION.tgz
```

That will create a `lsws-prometheus-exporter` directory.  Make that your default directory and run the install script as root:

```
cd lsws-prometheus-exporter
sudo ./install.sh
```

You are then prompted:

```
Cert file name [ENTER for no HTTPS]: 
```

Press [ENTER] by itself to use HTTP only for Prometheus connections to the exporter.  If you want to require HTTPS connections from Prometheus, enter a cert file name which is stored in a permanent location to be used by the service in PEM file format.  You will then be asked for a matching Key file name.

The service is then installed and started.

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
| `--metrics-excluded-list` | A comma separated list of metrics to exclude, using the Prometheus name without the prefix `litespeed_`. | None |
| `--metrics-service-addr` | The address and port to use to listen for prometheus collection requests within the pod.  Form: addr:port; a blank addr listens on all addresses. | `:9936` |
| `--metrics-service-path` | The HTTP path to service requests on. | `/metrics` |
| `--tls-cert-file` | If you want to require https to access metrics you must specify a `tls-cert-file` and a `tls-key-file` which are PEM encoded files | None |
| `--tls-key-file` | If you want to require https to access metrics you must specify a `tls-cert-file` and a `tls-key-file` which are PEM encoded files | None |
| `--v` | Sets info loggings.  `--v=4` is the most verbose. | `2` |

## Troubleshooting

The exporter writes its errors and important messages to standard output.  If you use the install script, this will have any messages written to the system log.  On SystemD systems, these are read using `journalctl`.

## Building the Exporter

The exporter is built using the included Makefile.  If there's a change, update the script with the new version number.  If you wish to build the full package, make sure that `STAGING` is set to `0`; with staging set to `1` only the binary will be built.

## Notable changes

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

