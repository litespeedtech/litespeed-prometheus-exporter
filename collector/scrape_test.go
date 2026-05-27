package collector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestScrapeFile_GoldenReport validates that scrapeFile correctly parses a
// representative .rtreport file and populates the report struct.
func TestScrapeFile_GoldenReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".rtreport")
	const sample = `VERSION: LiteSpeed Web Server/Enterprise/6.1.2
UPTIME: 02:56:01
BPS_IN: 1, BPS_OUT: 2, SSL_BPS_IN: 3, SSL_BPS_OUT: 4
MAXCONN: 100, MAXSSL_CONN: 200, PLAINCONN: 5, AVAILCONN: 95, IDLECONN: 0, SSLCONN: 0, AVAILSSL: 200
REQ_RATE []: REQ_PROCESSING: 0, REQ_PER_SEC: 0.5, TOT_REQS: 10, PUB_CACHE_HITS_PER_SEC: 0.0, TOTAL_PUB_CACHE_HITS: 0, PRIVATE_CACHE_HITS_PER_SEC: 0.0, TOTAL_PRIVATE_CACHE_HITS: 0, STATIC_HITS_PER_SEC: 0.0, TOTAL_STATIC_HITS: 0
REQ_RATE [example.com]: REQ_PROCESSING: 0, REQ_PER_SEC: 0.2, TOT_REQS: 5, PUB_CACHE_HITS_PER_SEC: 0.0, TOTAL_PUB_CACHE_HITS: 0, PRIVATE_CACHE_HITS_PER_SEC: 0.0, TOTAL_PRIVATE_CACHE_HITS: 0, STATIC_HITS_PER_SEC: 0.0, TOTAL_STATIC_HITS: 0
EXTAPP [LSAPI] [example.com] [example.com]: CMAXCONN: 35, EMAXCONN: 35, POOL_SIZE: 1, INUSE_CONN: 0, IDLE_CONN: 1, WAITQUE_DEPTH: 0, REQ_PER_SEC: 0.1, TOT_REQS: 1
`
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &LitespeedCollector{
		options: LitespeedCollectorOpts{
			ReqRatesByHost:  true,
			MetricsByCore:   true,
			ExcludedMetrics: map[string]bool{},
			RtReport:        path,
			FilePattern:     path + "*",
		},
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{Name: "test_failures"}),
	}

	report, err := c.scrapeFile(path)
	if err != nil {
		t.Fatalf("scrapeFile: %v", err)
	}
	if report.GeneralInfo.Version != "LiteSpeed Web Server/Enterprise/6.1.2" {
		t.Fatalf("version: %q", report.GeneralInfo.Version)
	}
	if report.GeneralInfo.Uptime != "02:56:01" {
		t.Fatalf("uptime: %q", report.GeneralInfo.Uptime)
	}
	if got := report.GeneralInfo.KeyValues[bpsInField]; got != 1 {
		t.Fatalf("BPS_IN: got %v want 1", got)
	}
	if got := report.GeneralInfo.KeyValues[maxConnField]; got != 100 {
		t.Fatalf("MAXCONN: got %v want 100", got)
	}
	if len(report.ReqRates) != 2 {
		t.Fatalf("ReqRates: got %d, want 2", len(report.ReqRates))
	}
	// Find the example.com vhost line.
	var found bool
	for _, rr := range report.ReqRates {
		if rr.VHost == "example.com" {
			found = true
			if rr.KeyValues[reqRateReqPerSecField] != 0.2 {
				t.Fatalf("REQ_PER_SEC for example.com: got %v want 0.2", rr.KeyValues[reqRateReqPerSecField])
			}
		}
	}
	if !found {
		t.Fatalf("expected example.com vhost in ReqRates")
	}
	if len(report.ExtApps) != 1 {
		t.Fatalf("ExtApps: got %d, want 1", len(report.ExtApps))
	}
	app := report.ExtApps[0]
	// Note: the existing parser sets VHost only when bracket[2] == bracket[3]
	// in the EXTAPP line; preserving that behavior in test.
	if app.AppType != "LSAPI" || app.VHost != "example.com" || app.Handler != "example.com" {
		t.Fatalf("ExtApps[0] = %+v, want AppType=LSAPI VHost=example.com Handler=example.com", app)
	}
	if app.KeyValues[extappCmaxconnField] != 35 {
		t.Fatalf("CMAXCONN: got %v want 35", app.KeyValues[extappCmaxconnField])
	}
}

// TestGetUpStatus_MissingFile returns 0 cleanly.
func TestGetUpStatus_MissingFile(t *testing.T) {
	if got := getUpStatus(filepath.Join(t.TempDir(), "missing")); got != 0 {
		t.Fatalf("got %v want 0 for missing pid file", got)
	}
}

// TestGetUpStatus_LiveProcess uses our own PID, which is guaranteed alive.
func TestGetUpStatus_LiveProcess(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "lshttpd.pid")
	if err := os.WriteFile(pidFile, []byte(itoa(os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := getUpStatus(pidFile); got != 1 {
		t.Fatalf("got %v want 1 for live pid", got)
	}
}

// itoa avoids pulling strconv into a tiny helper above.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
