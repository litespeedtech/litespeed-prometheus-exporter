module github.com/rperper/lsws-prometheus-exporter.git

go 1.16

require (
	github.com/prometheus/client_golang v1.14.0
	github.com/spf13/cobra v1.6.0
	k8s.io/klog/v2 v2.80.1
)

replace github.com/rperper/lsws-prometheus-exporter/collector => ./collector
