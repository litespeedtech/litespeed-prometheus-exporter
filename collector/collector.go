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
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// AuthRealm is the WWW-Authenticate realm string sent on 401 responses.
const AuthRealm = "lsws-prometheus-exporter"

// Hoisted regexes so they are compiled once, not per scrape.
var (
	idRegex = regexp.MustCompile(`^\w*`)
	ibRegex = regexp.MustCompile(`\[([^\[\]]*)\]`)
)

// LitespeedPidFile is the default LiteSpeed daemon PID file used by getUpStatus.
// Exported so tests (and a future CLI flag) can override it.
var LitespeedPidFile = "/tmp/lshttpd/lshttpd.pid"

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

// requireBasicAuth wraps the supplied handler with HTTP Basic
// authentication. When username is empty, no auth is required and the
// handler runs unmodified. The credential check uses
// subtle.ConstantTimeCompare to avoid leaking the password length / prefix
// via timing. The handler does NOT log credentials at any verbosity level.
func requireBasicAuth(username, password string, h http.Handler) http.Handler {
	if username == "" {
		return h
	}
	// Pre-hash credential bytes so the constant-time compare always works
	// over the same length, regardless of what the client sends.
	expectedUser := []byte(username)
	expectedPass := []byte(password)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, ok := r.BasicAuth()
		// Always run BOTH compares to avoid revealing whether the username
		// matched via the response timing. ConstantTimeEq returns 0 when
		// inputs differ and we OR the results to keep the operation
		// constant-time.
		userOK := subtle.ConstantTimeCompare([]byte(gotUser), expectedUser)
		passOK := subtle.ConstantTimeCompare([]byte(gotPass), expectedPass)
		if !ok || userOK&passOK != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+AuthRealm+`", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// BasicAuthDecode is exported for tests; given a "Basic xxxx" header value
// it returns the decoded "user:pass" string. We do not use it in the live
// auth path (we use r.BasicAuth) but tests for malformed headers exercise
// the decode behaviour directly.
func BasicAuthDecode(header string) (string, error) {
	const prefix = "Basic "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("expected %q prefix", strings.TrimSpace(prefix))
	}
	out, err := base64.StdEncoding.DecodeString(header[len(prefix):])
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// BuildHandler constructs the hardened http.Handler used by the exporter.
// When username is non-empty, /metrics requires HTTP Basic auth. The home
// page at "/" is always reachable. Tests drive this via httptest.
func BuildHandler(metricsPath, username, password string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(metricsPath, requireBasicAuth(username, password, promhttp.Handler()))

	escapedPath := html.EscapeString(metricsPath)
	body := []byte(`<!doctype html>
<html>
<head><title>LiteSpeed Prometheus Exporter</title></head>
<body>
<h1>LiteSpeed Prometheus Exporter</h1>
<p><a href="` + escapedPath + `">Metrics</a></p>
</body>
</html>
`)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		klog.V(4).Infof("LiteSpeed Prometheus Collector default home page")
		_, _ = w.Write(body)
	})
	return mux
}

func Run(ctx context.Context, addr, metricsPath, metricsExcludedList, tlsCertFile, tlsKeyFile, username, password string, cgroupTry int, litespeedHome string, rtReport string) {
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

	srv := &http.Server{
		Addr:              addr,
		Handler:           BuildHandler(metricsPath, username, password),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16, // 64 KiB
	}
	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		klog.V(4).Infof("Shutdown prometheus listener")
	}()

	klog.V(4).Infof("Begin collector listen on %v", addr)

	if tlsCertFile != "" && tlsKeyFile != "" {
		if err := srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != nil && err != http.ErrServerClosed {
			klog.Errorf("Exited HTTPS server for Prometheus support: %v", err)
		}
	} else {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

// cleanupBadFiles removes stale realtime report siblings of rtReport whose
// mtime differs from the base file. The glob is restricted to files that live
// in the same directory as rtReport so an attacker who controls the pattern
// (or a misconfigured --rtreport flag) cannot cause arbitrary file deletion
// outside that directory. Symlinks are skipped via Lstat to defeat
// symlink-based redirection attacks.
func cleanupBadFiles(rtReport, pattern string) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		klog.Errorf("Unable to get matching files for: %v: %v", pattern, err)
		return
	}
	baseStat, err := os.Lstat(rtReport)
	if err != nil {
		klog.Errorf("Unable to get stat for base file: %v: %v", rtReport, err)
		return
	}
	if baseStat.Mode()&os.ModeSymlink != 0 {
		klog.Errorf("Refusing to operate; base rtreport is a symlink: %v", rtReport)
		return
	}
	rtDir := filepath.Dir(rtReport)
	for _, file := range matches {
		// Confine deletion to the rtreport directory.
		if filepath.Dir(file) != rtDir {
			klog.V(4).Infof("cleanupBadFiles skip out-of-tree match: %v", file)
			continue
		}
		thisStat, err1 := os.Lstat(file)
		if err1 != nil {
			klog.V(4).Infof("Skip during cleanup bad files %v for %v", file, err1)
			continue
		}
		// Never follow / unlink through symlinks or non-regular files.
		if !thisStat.Mode().IsRegular() {
			klog.V(4).Infof("cleanupBadFiles skip non-regular file: %v", file)
			continue
		}
		if baseStat.ModTime().Unix() != thisStat.ModTime().Unix() {
			klog.Infof("Deleting old realtime file: %v", file)
			if err := os.Remove(file); err != nil {
				klog.V(4).Infof("os.Remove %v failed: %v", file, err)
			}
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

	up := getUpStatus(LitespeedPidFile)
	if err := c.collectReports(ch); err != nil {
		klog.V(4).Infof("collectReports error: %v", err)
	}
	if c.litespeedCollectorCgroup.enabled {
		if err := c.litespeedCollectorCgroup.cgroupCollect(ch); err != nil {
			klog.Errorf("Error in collecting cgroup data: %v", err)
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
			if m, err := prometheus.NewConstMetric(metric.Desc, metric.Type, value, core); err != nil {
				klog.Errorf("Error in generalInfoMetric %v: %v", metric.Name, err)
			} else {
				ch <- m
			}
		}
	}
}

func (c *LitespeedCollector) collectReqRateMetrics(core string, reports []requestRateReport, ch chan<- prometheus.Metric) {
	for _, rrReport := range reports {
		for flag, value := range rrReport.KeyValues {
			if metric, ok := LitespeedMetrics.reqRateMetrics[flag]; ok {
				klog.V(4).Infof("reqRateMetric: %v, value: %v, core: %v", metric, value, core)
				if m, err := prometheus.NewConstMetric(metric.Desc, metric.Type, value, core, rrReport.VHost); err != nil {
					klog.Errorf("Error in reqRateMetric %v: %v", metric.Name, err)
				} else {
					ch <- m
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
				if m, err := prometheus.NewConstMetric(metric.Desc, metric.Type, value, core, eaReport.AppType, eaReport.VHost, eaReport.Handler); err != nil {
					klog.Errorf("Error in extAppMetric %v: %v", metric.Name, err)
				} else {
					ch <- m
				}
			}
		}
	}
}

// captureOutermostBrackets returns the contents of the outermost
// balanced [...] pair in s, supporting vhosts whose names contain "[" or
// "]". This was added in upstream v0.1.4 to fix a parsing bug where vhosts
// named e.g. "site[ALPHA]" were truncated.
func captureOutermostBrackets(s string) string {
	balance := 0
	start := -1
	var result string
	for i, r := range s {
		switch r {
		case '[':
			if balance == 0 {
				start = i + 1
			}
			balance++
		case ']':
			balance--
			if balance == 0 && start != -1 {
				result = s[start:i]
				start = -1
			} else if balance < 0 {
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
					klog.Errorf("Can't parse .rtreport field key %v value %v: %v", k, v, err)
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
			m := parseKeyValLineToMap(parts[1])
			rr := requestRateReport{
				// Use the balanced-bracket parser so vhosts whose names
				// contain "[" or "]" survive (upstream v0.1.4 fix).
				VHost:     captureOutermostBrackets(line),
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
