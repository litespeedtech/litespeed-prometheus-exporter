LiteSpeed Prometheus Exporter
=============================

The LiteSpeed Prometheus Exporter is a specially designed Prometheus application and uses the LiteSpeed Enterprise or the OpenLiteSpeed Web Server controller to export Prometheus compatible data which can also be used by Grafana and other compatible applications.

## Installation

There is an `install.sh` script included with the exporter.  This script will include the Prometheus exporter as a service 


### Command line parameters

| Name | Description | Value |
| - | - | - |
| `--metrics-evaluation-interval` | How often Prometheus should evaluate the data (in time format). | `1m` |
| `--metrics-scrape-interval` | Specify how often Prometheus should scrape the .rtreport file (in time format). | `1m` |
| `--metrics-service-target-port` | The port to be used to access metrics, within the pod, if enabled. This is the reserved port and is rarely changed. | `9936` |
| `--prometheus-port` | The port that will be exported to use Prometheus, if installed.  | `9090` |
| `--prometheus-remote-password` | The prometheus remote_write password.  Often your Grafana Prometheus Metrics API Key. | none |
| `--prometheus-remote-url` | The prometheus remote_write url.  Often your Grafana Prometheus Metrics service. | none |
| `--prometheus-remote-user` | The prometheus remote_write username.  Often your Grafana Prometheus Metrics username (a number). | none |
| `--prometheus-target-port` | The port that will be used within the pod for Prometheus, if installed. | `9091` |

