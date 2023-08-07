package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "litespeed"
)

type metricInfo struct {
	Name       string
	ScrapeName string
	Desc       *prometheus.Desc
	Type       prometheus.ValueType
}

type metricMap map[string]metricInfo

type metrics struct {
	/* The map keys are the ScrapeName */
	generalInfoMetrics metricMap
	reqRateMetrics     metricMap
	extAppMetrics      metricMap
}

var (
	// TODO normalize the names & descriptions of each metric

	// LitespeedMetrics includes all available LiteSpeed metrics
	LitespeedMetrics = metrics{
		generalInfoMetrics: metricMap{
			bpsInField:      newGeneralInfoMetric("incoming_http_bytes_per_second", bpsInField, "Incoming number of bytes per second over HTTP", prometheus.GaugeValue),
			bpsOutField:     newGeneralInfoMetric("outgoing_http_bytes_per_second", bpsOutField, "Outgoing number of bytes per second over HTTP", prometheus.GaugeValue),
			sslBpsInField:   newGeneralInfoMetric("incoming_ssl_bytes_per_second", sslBpsInField, "Incoming number of bytes per second using SSL (HTTPS)", prometheus.GaugeValue),
			sslBpsOutField:  newGeneralInfoMetric("outgoing_ssl_bytes_per_second", sslBpsOutField, "Outgoing number of bytes per second using SSL (HTTPS)", prometheus.GaugeValue),
			maxConnField:    newGeneralInfoMetric("maximum_http_connections", maxConnField, "Maximum configured http connections", prometheus.CounterValue),
			maxSslConnField: newGeneralInfoMetric("maximum_ssl_connections", maxSslConnField, "Maximum configured ssl (https) connections", prometheus.CounterValue),
			plainconnField:  newGeneralInfoMetric("current_http_connections", plainconnField, "Current number of http connections", prometheus.GaugeValue),
			availConnField:  newGeneralInfoMetric("available_connections", availConnField, "Available number of connections", prometheus.GaugeValue),
			idleconnField:   newGeneralInfoMetric("current_idle_connections", idleconnField, "Current number of idle connections", prometheus.GaugeValue),
			sslconnField:    newGeneralInfoMetric("current_ssl_connections", sslconnField, "Current number of SSL (https) connections", prometheus.GaugeValue),
			availSslField:   newGeneralInfoMetric("available_ssl_connections", availSslField, "Available number of SSL (https) connections", prometheus.GaugeValue),
		},
		reqRateMetrics: metricMap{
			reqRateReqProcessingField:          newReqRateMetric("current_requests", reqRateReqProcessingField, "Current number of requests in flight", prometheus.GaugeValue),
			reqRateReqPerSecField:              newReqRateMetric("requests_per_second", reqRateReqPerSecField, "Requests per second", prometheus.GaugeValue),
			reqRateTotReqsField:                newReqRateMetric("total_requests", reqRateTotReqsField, "Total number of requests", prometheus.CounterValue),
			reqRatePubCacheHitsPerSecField:     newReqRateMetric("public_cache_hits_per_second", reqRatePubCacheHitsPerSecField, "Public cached hits per second", prometheus.GaugeValue),
			reqRateTotalPubCacheHitsField:      newReqRateMetric("public_cache_hits", reqRateTotalPubCacheHitsField, "Total public cached hits", prometheus.CounterValue),
			reqRatePrivateCacheHitsPerSecField: newReqRateMetric("private_cache_hits_per_second", reqRatePrivateCacheHitsPerSecField, "Private cached hits per second", prometheus.GaugeValue),
			reqRateTotalPrivateCacheHitsField:  newReqRateMetric("private_cache_hits", reqRateTotalPrivateCacheHitsField, "Total private cached hits", prometheus.CounterValue),
			reqRateStaticHitsPerSecField:       newReqRateMetric("static_hits_per_second", reqRateStaticHitsPerSecField, "Static hits per second", prometheus.GaugeValue),
			reqRateTotalStaticHitsField:        newReqRateMetric("static_hits", reqRateTotalStaticHitsField, "Total static hits", prometheus.CounterValue),
		},
		extAppMetrics: metricMap{
			extappCmaxconnField:     newExtappMetric("config_max_connections", extappCmaxconnField, "Configured maximum number of connections", prometheus.GaugeValue),
			extappEmaxconnField:     newExtappMetric("pool_max_connections", extappEmaxconnField, "Maximum number of connections for the pool", prometheus.GaugeValue),
			extappPoolSizeField:     newExtappMetric("pool_count", extappPoolSizeField, "Total number of pools", prometheus.GaugeValue),
			extappInuseConnField:    newExtappMetric("connections_in_use", extappInuseConnField, "Number of connections in use", prometheus.GaugeValue),
			extappIdleConnField:     newExtappMetric("connections_idle", extappIdleConnField, "Number of idle connections", prometheus.GaugeValue),
			extappWaitqueDepthField: newExtappMetric("wait_queue_depth", extappWaitqueDepthField, "Depth of the waiting queue", prometheus.GaugeValue),
			extappReqPerSecField:    newExtappMetric("requests_per_second", extappReqPerSecField, "Number of requests per second", prometheus.GaugeValue),
			extappTotReqsField:      newExtappMetric("total_requests", extappTotReqsField, "Total number of requests", prometheus.CounterValue),
		},
	}
	litespeedVersion = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "version"), "A metric with a constant '1' value labeled by the LiteSpeed version.", []string{"version"}, nil)
	litespeedUp      = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "up"), "Was the last scrape of LiteSpeed successful.", nil, nil)
)

/*
func (m metrics) String() string {
	s := []string{}
	for k, v := range GeneralInfoMetrics {
		s = append(s, k)
	}
	sort.Strings(s)
	return strings.Join(s, ", ")
}
*/

func newGeneralInfoMetric(name, scrapeName, help string, t prometheus.ValueType) metricInfo {
	return metricInfo{
		Name:       name,
		ScrapeName: scrapeName,
		Desc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", name),
			help,
			[]string{"core"},
			nil,
		),
		Type: t,
	}
}

func newReqRateMetric(name, scrapeName, help string, t prometheus.ValueType) metricInfo {
	return metricInfo{
		Name:       name + "_per_vhost",
		ScrapeName: reqRateField + "_" + scrapeName,
		Desc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", name+"_per_vhost"),
			help+" per virtual host",
			[]string{"core", "vhost"},
			nil,
		),
		Type: t,
	}
}

func newExtappMetric(name, scrapeName, help string, t prometheus.ValueType) metricInfo {
	return metricInfo{
		Name:       name + "_per_app",
		ScrapeName: extappField + "_" + scrapeName,
		Desc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", name+"_per_app"),
			help+" per app",
			[]string{"core", "app_type", "vhost", "app_name"},
			nil,
		),
		Type: t,
	}
}
