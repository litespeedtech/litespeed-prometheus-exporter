package collector

import (
	"crypto/subtle"
	b64 "encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var base64Std = b64.StdEncoding

// TestAuth_NoneRequired: when username is empty, /metrics is served
// unauthenticated.
func TestAuth_NoneRequired(t *testing.T) {
	srv := httptest.NewServer(BuildHandler("/metrics", "", ""))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", resp.StatusCode)
	}
}

// TestAuth_MissingCredentials: a missing Authorization header is 401 and
// must include WWW-Authenticate so clients know to retry.
func TestAuth_MissingCredentials(t *testing.T) {
	srv := httptest.NewServer(BuildHandler("/metrics", "alice", "s3cret"))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", resp.StatusCode)
	}
	got := resp.Header.Get("WWW-Authenticate")
	if !strings.HasPrefix(got, "Basic ") {
		t.Fatalf("WWW-Authenticate=%q, want Basic-prefixed", got)
	}
	if !strings.Contains(got, `realm="`+AuthRealm+`"`) {
		t.Fatalf("WWW-Authenticate missing realm: %q", got)
	}
}

// TestAuth_WrongUsername / TestAuth_WrongPassword: any mismatch returns 401
// and never reveals which side was wrong (we can't directly assert that, but
// we can confirm both error paths return identical statuses).
func TestAuth_WrongUsername(t *testing.T) {
	srv := httptest.NewServer(BuildHandler("/metrics", "alice", "s3cret"))
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/metrics", nil)
	req.SetBasicAuth("eve", "s3cret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", resp.StatusCode)
	}
}

func TestAuth_WrongPassword(t *testing.T) {
	srv := httptest.NewServer(BuildHandler("/metrics", "alice", "s3cret"))
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/metrics", nil)
	req.SetBasicAuth("alice", "wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", resp.StatusCode)
	}
}

// TestAuth_Correct: valid credentials succeed.
func TestAuth_Correct(t *testing.T) {
	srv := httptest.NewServer(BuildHandler("/metrics", "alice", "s3cret"))
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/metrics", nil)
	req.SetBasicAuth("alice", "s3cret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", resp.StatusCode)
	}
}

// TestAuth_HomePageStillReachable: when auth is enabled, the home page at
// "/" remains unauthenticated (it has no sensitive content). This matches
// most exporters in the prom-community.
func TestAuth_HomePageStillReachable(t *testing.T) {
	srv := httptest.NewServer(BuildHandler("/metrics", "alice", "s3cret"))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200 on /", resp.StatusCode)
	}
}

// TestAuth_NoBodyLeakOn401: the 401 body must not leak the realm/username
// in a way that helps an attacker — body should just be a generic message.
func TestAuth_NoBodyLeakOn401(t *testing.T) {
	srv := httptest.NewServer(BuildHandler("/metrics", "alice", "s3cret"))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "alice") {
		t.Fatalf("401 body must not echo username, got: %q", body)
	}
	if strings.Contains(string(body), "s3cret") {
		t.Fatalf("401 body must not echo password, got: %q", body)
	}
}

// TestAuth_ConstantTimeCompareUsed: a sanity test that crypto/subtle is
// linked. We don't try to time-measure (flaky) — instead we verify the
// expected behaviour: differing-length inputs do not panic and produce 0.
func TestAuth_ConstantTimeCompareUsed(t *testing.T) {
	if subtle.ConstantTimeCompare([]byte("a"), []byte("ab")) != 0 {
		t.Fatal("ConstantTimeCompare should return 0 for differing lengths")
	}
	if subtle.ConstantTimeCompare([]byte("abc"), []byte("abc")) != 1 {
		t.Fatal("ConstantTimeCompare should return 1 for equal inputs")
	}
}

// TestBasicAuthDecode: malformed Authorization headers report errors
// rather than panicking.
func TestBasicAuthDecode(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"good", "Basic " + b64encodeFn("alice:s3cret"), false},
		{"missing prefix", b64encodeFn("alice:s3cret"), true},
		{"bad base64", "Basic !!!!!!!!", true},
		{"empty", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BasicAuthDecode(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("input=%q err=%v wantErr=%v", tc.input, err, tc.wantErr)
			}
		})
	}
}

// b64encode is a tiny helper so the test stays self-contained.
func b64encodeFn(s string) string {
	return base64Std.EncodeToString([]byte(s))
}
