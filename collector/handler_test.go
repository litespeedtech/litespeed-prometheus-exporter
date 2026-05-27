package collector

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestBuildHandler_NotFound asserts unknown paths return 404 (not the home
// page reflecting back into HTML).
func TestBuildHandler_NotFound(t *testing.T) {
	h := BuildHandler("/metrics", "", "")
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/random-path")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", resp.StatusCode)
	}
}

// TestBuildHandler_RejectsNonGET asserts POST is rejected on the home page
// rather than served the HTML.
func TestBuildHandler_RejectsNonGET(t *testing.T) {
	h := BuildHandler("/metrics", "", "")
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/", "text/plain", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("got status %d, want 405", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got == "" {
		t.Fatalf("Allow header should be set on 405, got empty")
	}
}

// TestBuildHandler_HomePage asserts a GET to / returns 200 and embeds the
// metrics path inside HTML-escaped href.
func TestBuildHandler_HomePage(t *testing.T) {
	h := BuildHandler(`/metrics"><script>x</script>`, "", "")
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}
	body := readAll(t, resp.Body.Read)
	// Raw script tag must NOT appear unescaped in the output.
	if strings.Contains(body, "<script>x</script>") {
		t.Fatalf("expected metrics-path to be HTML-escaped, body was: %s", body)
	}
	// And it must include the escaped form.
	if !strings.Contains(body, "&lt;script&gt;") && !strings.Contains(body, "&#34;") {
		// Either form (entity or hex) is fine.
		t.Fatalf("expected escaped metrics path in body, body was: %s", body)
	}
}

// TestBuildHandler_MetricsServed asserts /metrics returns Prometheus
// content. We don't register the LiteSpeed collector (that needs an rtreport
// file), but the default Go runtime collectors are always present.
func TestBuildHandler_MetricsServed(t *testing.T) {
	h := BuildHandler("/metrics", "", "")
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}
	body := readAll(t, resp.Body.Read)
	if !strings.Contains(body, "# HELP") && !strings.Contains(body, "# TYPE") {
		// promhttp default registry should always emit go_* metrics.
		t.Fatalf("expected Prometheus exposition format, got: %.200s", body)
	}
}

// readAll is a tiny helper that reads everything from a Reader-shaped fn.
func readAll(t *testing.T, read func(p []byte) (int, error)) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return sb.String()
}
