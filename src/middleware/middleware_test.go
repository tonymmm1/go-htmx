package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestStackAddsSecurityHeadersAndBlocksTrace(t *testing.T) {
	handler := Stack(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	request := httptest.NewRequest(http.MethodTrace, "http://example.com/", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("TRACE returned %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestStackRecoversFromPanics(t *testing.T) {
	handler := Stack(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))

	request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("panic returned %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestIPRateLimiterUsesRemoteAddress(t *testing.T) {
	limiter := newIPRateLimiter(2, time.Minute)
	handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for attempt := 1; attempt <= 3; attempt++ {
		request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		request.RemoteAddr = "192.0.2.1:1234"
		request.Header.Set("X-Forwarded-For", "198.51.100."+strconv.Itoa(attempt))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		want := http.StatusNoContent
		if attempt == 3 {
			want = http.StatusTooManyRequests
		}
		if recorder.Code != want {
			t.Fatalf("attempt %d returned %d, want %d", attempt, recorder.Code, want)
		}
	}
}

func TestGzipResponses(t *testing.T) {
	handler := gzipResponses(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello from Go"))
	}))

	request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	request.Header.Set("Accept-Encoding", "br, gzip")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	reader, err := gzip.NewReader(recorder.Body)
	if err != nil {
		t.Fatalf("opening gzip response: %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading gzip response: %v", err)
	}
	if got := string(body); got != "hello from Go" {
		t.Fatalf("decoded body = %q, want %q", got, "hello from Go")
	}
}

func TestAcceptsGzipHonorsZeroQuality(t *testing.T) {
	if acceptsGzip("gzip; q=0") {
		t.Fatal("gzip with q=0 should not be accepted")
	}
	if !acceptsGzip("br, gzip; q=0.5") {
		t.Fatal("gzip with a positive quality should be accepted")
	}
}
