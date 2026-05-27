package collector

import (
	"reflect"
	"testing"
)

func TestParseKeyValPair(t *testing.T) {
	k, v := parseKeyValPair("FOO: 42", ": ")
	if k != "FOO" || v != "42" {
		t.Fatalf("got (%q,%q), want (FOO,42)", k, v)
	}
}

func TestParseKeyValLineToMap(t *testing.T) {
	got := parseKeyValLineToMap("BPS_IN: 1, BPS_OUT: 2, SSL_BPS_IN: 3, SSL_BPS_OUT: 4")
	want := map[string]string{
		"BPS_IN":      "1",
		"BPS_OUT":     "2",
		"SSL_BPS_IN":  "3",
		"SSL_BPS_OUT": "4",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseKeyValLineToMap_IgnoresBadPair(t *testing.T) {
	// A pair without ": " separator must be skipped, not panic.
	got := parseKeyValLineToMap("FOO: 1, BAREWORD, BAR: 2")
	want := map[string]string{"FOO": "1", "BAR": "2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseMetricValue(t *testing.T) {
	cases := []struct {
		flag    string
		value   string
		want    float64
		wantErr bool
	}{
		{bpsInField, "1024", 1024, false},
		{reqRateReqPerSecField, "12.5", 12.5, false}, // float-typed flag
		{bpsInField, "not-a-number", 0, true},
	}
	for _, tc := range cases {
		got, err := parseMetricValue(tc.flag, tc.value)
		if (err != nil) != tc.wantErr {
			t.Fatalf("flag=%v value=%v: err=%v wantErr=%v", tc.flag, tc.value, err, tc.wantErr)
		}
		if !tc.wantErr && got != tc.want {
			t.Fatalf("flag=%v value=%v: got %v want %v", tc.flag, tc.value, got, tc.want)
		}
	}
}

func TestParseFlagsToMap(t *testing.T) {
	got := ParseFlagsToMap([]string{"a", "b", ""})
	for _, k := range []string{"a", "b", ""} {
		if !got[k] {
			t.Fatalf("missing key %q in %#v", k, got)
		}
	}
}
