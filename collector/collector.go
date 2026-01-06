/*
Copyright © 2023-2024 LiteSpeed Technologies <litespeedtech.com>

Licensed under the GPLv3 License (the "License"); you may not use this file
except in compliance with the License.  You may obtain a copy of the License at

    https://www.gnu.org/licenses/gpl-3.0.en.html

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package collector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// LitespeedCollectorOpts carries the options used in LitespeedCollector
type LitespeedCollectorOpts struct {
	ReqRatesByHost  bool
	MetricsByCore   bool
	ExcludeExtapp   bool
	ExcludedMetrics map[string]bool // external name is the key
	CgroupTry       int
	LitespeedHome   string
	RtReport        string
	FilePattern     string
}

// LitespeedCollector collects LiteSpeed stats from the given files and exports them as Prometheus metrics
type LitespeedCollector struct {
	mutex                        sync.RWMutex
	options                      LitespeedCollectorOpts
	totalScrapes, scrapeFailures prometheus.Counter
	litespeedCollectorCgroup     *LitespeedCollectorCgroup
}

// customMetricsHandler wraps the promhttp.Handler to add custom logic.
func customMetricsHandler(username string, password string) http.Handler {
	// Get the default Prometheus handler
	h := promhttp.Handler()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// You can read request headers here
		userAgent := r.Header.Get("User-Agent")
		ok := false
		if username == "" {
			ok = true
		} else {
			authorization := r.Header.Get("Authorization")
			klog.V(4).Infof("Metrics request received with User-Agent: %s Authorization: %s\n", userAgent, authorization)
			if authorization == "" {
				klog.Errorf("Could not find authorization header")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			} else if auth64, found := strings.CutPrefix(authorization, "Basic "); !found {
				klog.Errorf("Expecting but did not find Basic authorization string")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			} else if decodedBytes, err := base64.StdEncoding.DecodeString(string(auth64)); err != nil {
				klog.Errorf("Could not decode authorization %v %v", string(auth64), err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			} else if auths := strings.SplitN(string(decodedBytes), ":", 2); len(auths) != 2 {
				klog.Errorf("Could not find authorization sep in %v (%v)", string(decodedBytes), auths)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			} else if auths[0] != username {
				klog.Errorf("Invalid user for metrics request")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			} else if auths[1] != password {
				klog.Errorf("Invalid password for metrics request")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				klog.V(4).Infof("%s != %s", auths[1], password)
			} else {
				klog.V(4).Infof("Valid basic authentication")
				ok = true
			}
		}

		// You can also set response headers here
		//w.Header().Set("X-Custom-Header", "Prometheus-Metrics-Endpoint")

		if ok {
			// Call the actual promhttp.Handler to serve the metrics
			klog.V(4).Infof("Serving metrics")
			h.ServeHTTP(w, r)
		}
	})
}

func Run(ctx context.Context, addr, metricsPath, metricsExcludedList, tlsCertFile, tlsKeyFile string, username string, password string, cgroupTry int, litespeedHome string, rtReport string) {
	excludedMetricFlags := strings.Split(metricsExcludedList, ",")
	collector := NewLitespeedCollector(
		LitespeedCollectorOpts{
			ReqRatesByHost:  true,
			MetricsByCore:   true,
			ExcludeExtapp:   false,
			ExcludedMetrics: ParseFlagsToMap(excludedMetricFlags),
			CgroupTry:       cgroupTry,
			LitespeedHome:   litespeedHome,
			RtReport:        rtReport,
			FilePattern:     rtReport + "*",
		},
	)
	prometheus.MustRegister(collector)

	klog.V(4).Infof("listenAddr: %v", addr)

	http.Handle(metricsPath, customMetricsHandler(username, password))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		klog.V(4).Infof("LiteSpeed Prometheus Collector default home page")
		w.Write([]byte(`
			<html>
            <head><title>LiteSpeed Prometheus Exporter</title></head>
            <body>
            <h1>LiteSpeed Prometheus Exporter</h1>
            <p><a href='` + metricsPath + `'>Metrics</a></p>
            </body>
			</html>
		`))
	})

	srv := http.Server{Addr: addr}
	go func() {
		<-ctx.Done()

		srv.Shutdown(ctx)
		klog.V(4).Infof("Shutdown prometheus listener")
	}()

	klog.V(4).Infof("Begin collector listen on %v", addr)

	if tlsCertFile != "" && tlsKeyFile != "" {
		if err := srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != nil {
			klog.Errorf("Exited HTTPS server for Prometheus support: %v", err)
		}
	} else {
		if err := srv.ListenAndServe(); err != nil {
			klog.Errorf("Exited HTTP server for Prometheus support: %v", err)
		}
	}
	klog.V(4).Infof("Exiting collector.Run()")

}

// NewLitespeedCollector returns constructed collector
func NewLitespeedCollector(opts LitespeedCollectorOpts) *LitespeedCollector {
	cleanupBadFiles(opts.RtReport, opts.FilePattern)
	collector := &LitespeedCollector{
		options: opts,
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total LiteSpeed scrapes.",
		}),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while scraping files.",
		}),
	}
	collector.litespeedCollectorCgroup = NewLitespeedCollectorCgroup(collector)
	return collector
}

func cleanupBadFiles(rtReport, pattern string) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		klog.Errorf("Unable to get matching files for: %v: %v", pattern, err)
		return
	}
	baseStat, err := os.Stat(rtReport)
	if err != nil {
		klog.Errorf("Unable to get stat for base file: %v: %v", rtReport, err)
		return
	}
	for _, file := range matches {
		thisStat, err1 := os.Stat(file)
		if err1 != nil {
			klog.V(4).Infof("Skip during cleanup bad files %v for %v", file, err1)
			continue
		}
		if baseStat.ModTime().Unix() != thisStat.ModTime().Unix() {
			klog.Infof("Deleting old realtime file: %v", file)
			os.Remove(file)
		}
	}
}

func (c *LitespeedCollector) metricIsTracked(flag string) bool {
	_, ok := c.options.ExcludedMetrics[flag]
	if ok {
		klog.V(4).Infof("Exclude metric: %v", flag)
	}
	return !ok
}

// Describe describes all the metrics that can be exported by the LiteSpeed exporter
func (c *LitespeedCollector) Describe(ch chan<- *prometheus.Desc) {
	klog.V(4).Infof("collector Describe")

	for _, metric := range LitespeedMetrics.generalInfoMetrics {
		if c.metricIsTracked(metric.Name) {
			ch <- metric.Desc
		}
	}
	for _, metric := range LitespeedMetrics.reqRateMetrics {
		if c.metricIsTracked(metric.Name) {
			ch <- metric.Desc
		}
	}
	for _, metric := range LitespeedMetrics.extAppMetrics {
		if c.metricIsTracked(metric.Name) {
			ch <- metric.Desc
		}
	}
	if c.litespeedCollectorCgroup.enabled {
		c.litespeedCollectorCgroup.cgroupDescribe(ch)
	}
	ch <- litespeedVersion
	ch <- litespeedUp
	ch <- c.totalScrapes.Desc()
	ch <- c.scrapeFailures.Desc()
	klog.V(4).Infof("collector Describe done")
}

// Collect fetches the stats from target files and delivers them as Prometheus metrics
func (c *LitespeedCollector) Collect(ch chan<- prometheus.Metric) {
	klog.V(4).Infof("collector Collect")

	c.mutex.Lock()
	defer c.mutex.Unlock()

	up := getUpStatus("/tmp/lshttpd/lshttpd.pid")
	c.collectReports(ch)
	if c.litespeedCollectorCgroup.enabled {
		if err := c.litespeedCollectorCgroup.cgroupCollect(ch); err != nil {
			klog.Errorf("Error in collecting cgroup data: ", err)
		}
	}

	ch <- prometheus.MustNewConstMetric(litespeedUp, prometheus.GaugeValue, up)
	ch <- c.totalScrapes
	ch <- c.scrapeFailures
	//klog.V(4).Infof("collector Collect done")
}

func getUpStatus(pidFile string) float64 {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(data)))
	if err != nil {
		return 0
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return 0
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil && err != syscall.EPERM {
		return 0
	}

	return 1
}

func (c *LitespeedCollector) collectReports(ch chan<- prometheus.Metric) error {
	c.totalScrapes.Inc()

	reports, err := c.scrapeReports(c.options.FilePattern)
	if err != nil {
		c.scrapeFailures.Inc()
		return err
	}

	versionScraped := false

	for core, report := range reports {
		if !versionScraped {
			ch <- prometheus.MustNewConstMetric(litespeedVersion, prometheus.GaugeValue, 1, report.GeneralInfo.Version)
			versionScraped = true
		}

		c.collectGeneralInfoMetrics(core, report.GeneralInfo, ch)
		c.collectReqRateMetrics(core, report.ReqRates, ch)
		c.collectExtAppMetrics(core, report.ExtApps, ch)
	}

	return nil
}

func (c *LitespeedCollector) collectGeneralInfoMetrics(core string, generalInfo generalInfoReport, ch chan<- prometheus.Metric) {
	for flag, value := range generalInfo.KeyValues {
		if metric, ok := LitespeedMetrics.generalInfoMetrics[flag]; ok {
			klog.V(4).Infof("generalInfoMetric: %v", metric)
			if metricOut, err := prometheus.NewConstMetric(metric.Desc, metric.Type, value, core); err != nil {
				klog.Errorf("Error in collecting generalInfoMetrics: %v: %v", metric, err)
			} else {
				ch <- metricOut
			}
		}
	}
}

func (c *LitespeedCollector) collectReqRateMetrics(core string, reports []requestRateReport, ch chan<- prometheus.Metric) {
	for _, rrReport := range reports {
		for flag, value := range rrReport.KeyValues {
			if metric, ok := LitespeedMetrics.reqRateMetrics[flag]; ok {
				klog.V(4).Infof("reqRateMetric: %v, value: %v, core: %v", metric, value, core)
				if metricOut, err := prometheus.NewConstMetric(metric.Desc, metric.Type, value, core, rrReport.VHost); err != nil {
					klog.Errorf("Error in collecting reqRateMetric: %v: %v", metric, err)
				} else {
					ch <- metricOut
				}
			}
		}
	}
}

func (c *LitespeedCollector) collectExtAppMetrics(core string, reports []externalAppReport, ch chan<- prometheus.Metric) {
	for _, eaReport := range reports {
		for flag, value := range eaReport.KeyValues {
			if metric, ok := LitespeedMetrics.extAppMetrics[flag]; ok {
				klog.V(4).Infof("extAppMetric: %v, value: %v, core: %v", metric, value, core)
				if metricOut, err := prometheus.NewConstMetric(metric.Desc, metric.Type, value, core, eaReport.AppType, eaReport.VHost, eaReport.Handler); err != nil {
					klog.Errorf("Error in collecting ExtAppMetric: %v: %v", metric, err)
				} else {
					ch <- metricOut
				}
			}
		}
	}
}

func captureOutermostBrackets(s string) string {
	var result string
	balance := 0
	start := -1

	for i, r := range s {
		switch r {
		case '[':
			if balance == 0 {
				start = i + 1 // Start capturing after the opening bracket
			}
			balance++
		case ']':
			balance--
			if balance == 0 && start != -1 {
				// End capturing before the closing bracket
				result = s[start:i]
				start = -1 // Reset start for the next potential outermost bracket set
			} else if balance < 0 {
				// Handle malformed strings if necessary (e.g., extra closing bracket)
				balance = 0
				start = -1
			}
		}
	}
	return result
}

func (c *LitespeedCollector) scrapeFile(fileName string) (report *litespeedReport, err error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	defer func() {
		file.Close()
		if r := recover(); r != nil {
			err = fmt.Errorf("failed scraping file: %s", r)
			report = nil
		}
	}()

	idRegex := regexp.MustCompile(`^\w*`)
	ibRegex := regexp.MustCompile(`\[([^\[\]]*)\]`)

	report = &litespeedReport{
		GeneralInfo: generalInfoReport{KeyValues: make(map[string]float64)},
		ReqRates:    []requestRateReport{},
		ExtApps:     []externalAppReport{},
	}
	reader := bufio.NewReader(file)
	var line string

	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}

		line = strings.TrimRight(line, "\n")

		identifier := idRegex.FindString(line)
		if identifier == "" {
			continue
		}

		switch identifier {
		case versionField:
			_, v := parseKeyValPair(line, ": ")
			report.GeneralInfo.Version = v
		case uptimeField:
			_, v := parseKeyValPair(line, ": ")
			report.GeneralInfo.Uptime = v
		case bpsInField, plainconnField, maxConnField:
			m := parseKeyValLineToMap(line)
			for k, v := range m {
				vf, err := parseMetricValue(k, v)
				if err != nil {
					klog.Errorf("Can't parse .rtreport field value: %v: %v", err)
					c.scrapeFailures.Inc()
				} else if val, ok := LitespeedMetrics.generalInfoMetrics[k]; !ok || !c.metricIsTracked(val.Name) {
					klog.V(4).Infof("Overall report skip not found or requested key: %v", k)
					continue
				} else {
					report.GeneralInfo.KeyValues[k] = vf
				}
			}
		case reqRateField:
			parts := strings.SplitN(line, ": ", 2)
			matches := ibRegex.FindStringSubmatch(line)

			m := parseKeyValLineToMap(parts[1])
			rr := requestRateReport{
				VHost: matches[1],
				//VHost:     captureOutermostBrackets(line),
				KeyValues: make(map[string]float64),
			}
			klog.V(4).Infof("reqRate report, vhost: %v kvs: %v", rr.VHost, m)
			for k, v := range m {
				if val, ok := LitespeedMetrics.reqRateMetrics[k]; !ok || !c.metricIsTracked(val.Name) {
					klog.V(4).Infof("reqRate report skip not found or requested key: %v", k)
					continue
				}

				vf, err := parseMetricValue(k, v)
				if err != nil {
					klog.Errorf("Error parsing value key %v in %v: %v", k, v, err)
					c.scrapeFailures.Inc()
				} else {
					rr.KeyValues[k] = vf
				}
			}
			report.ReqRates = append(report.ReqRates, rr)
		case extappField:
			if c.options.ExcludeExtapp {
				break
			}

			parts := strings.SplitN(line, ": ", 2)
			m := parseKeyValLineToMap(parts[1])
			matches := ibRegex.FindAllStringSubmatch(line, -1)
			vhost := ""
			if matches[1][1] == matches[2][1] {
				vhost = matches[1][1]
			}
			if !c.options.ReqRatesByHost && vhost != "" {
				klog.V(4).Infof("extApp report skip host %v in %v", vhost, line)
				continue
			}

			//klog.V(4).Infof("extApp report, service: %v, Hostname: %v, Handler: %v", matches[0][1], hostname, matches[2][1])
			er := externalAppReport{
				AppType:   matches[0][1],
				VHost:     vhost,
				Handler:   matches[2][1],
				KeyValues: make(map[string]float64),
			}
			for k, v := range m {
				if val, ok := LitespeedMetrics.extAppMetrics[k]; !ok || !c.metricIsTracked(val.Name) {
					klog.V(4).Infof("extApp report skip not found or requested key: %v", k)
					continue
				}

				vf, err := parseMetricValue(k, v)
				if err != nil {
					klog.Errorf("Error parsing value key %v in %v: %v", k, v, err)
					c.scrapeFailures.Inc()
				} else {
					er.KeyValues[k] = vf
				}
			}
			report.ExtApps = append(report.ExtApps, er)
		}
	}

	if err != io.EOF {
		return nil, err
	}

	return report, nil
}

func (c *LitespeedCollector) scrapeReports(filePattern string) (map[string]litespeedReport, error) {
	matches, err := filepath.Glob(filePattern)
	if err != nil {
		return nil, err
	}

	reports := make(map[string]litespeedReport)
	for _, match := range matches {
		report, err := c.scrapeFile(match)
		if err == nil {
			reports[match] = *report
		}
	}

	if !c.options.MetricsByCore {
		return map[string]litespeedReport{"": *sumReports(reports)}, nil
	}

	return reports, nil
}
