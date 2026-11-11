package contentcoding

import (
	"net/http/httptest"
	"testing"
)

func TestNegotiate(t *testing.T) {
	both := []string{"br", "gzip"}
	gzipOnly := []string{"gzip"}

	for _, c := range []struct {
		header  string
		offered []string
		want    string
	}{
		{"", both, ""},
		{"gzip", both, "gzip"},
		{"br", both, "br"},
		{"gzip, deflate, br, zstd", both, "br"},
		{"br;q=0.5, gzip", both, "gzip"},
		{"br;q=0, gzip", both, "gzip"},
		{"br, gzip;q=0", gzipOnly, ""},
		{"gzip;q=1.0, br;q=1.0", both, "br"}, // tie: server preference
		{"*", both, "br"},
		{"*;q=0", both, ""},
		{"*, br;q=0", both, "gzip"},
		{"x-gzip", both, "gzip"},
		{"GZIP; Q=0.5", both, "gzip"},
		{"identity", both, ""},
		{"deflate, zstd", both, ""},
		{"br;level=1;q=0.8, gzip;q=0.5", both, "br"},
		// Malformed or out-of-range q-values refuse the coding rather than
		// falling back to "*".
		{"br;q=-1, *", both, "gzip"},
		{"br;q=2, gzip;q=0.5", both, "gzip"},
		{"gzip;q=NaN", both, ""},
		{"gzip;q=abc, br", both, "br"},
		{"gzip", nil, ""},
	} {
		if got := negotiate(c.header, c.offered); got != c.want {
			t.Errorf("negotiate(%q, %q) = %q, want %q", c.header, c.offered, got, c.want)
		}
	}
}

func TestNegotiateCombinesRepeatedHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Add("Accept-Encoding", "gzip;q=0.5")
	r.Header.Add("Accept-Encoding", "br")
	if got := Negotiate(r, "br", "gzip"); got != "br" {
		t.Errorf("Negotiate = %q, want br", got)
	}
}
