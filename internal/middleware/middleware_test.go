package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var discardOptions = Options{Logger: slog.New(slog.DiscardHandler)}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

func TestStackAddsSecurityHeadersAndBlocksTrace(t *testing.T) {
	handler := Stack(http.HandlerFunc(okHandler), discardOptions)

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

func TestStackSetsContentSecurityPolicy(t *testing.T) {
	handler := Stack(http.HandlerFunc(okHandler), discardOptions)

	request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	csp := recorder.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'self'", "script-src 'self'", "style-src 'self'", "object-src 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("Content-Security-Policy %q is missing %q", csp, directive)
		}
	}
	if got := recorder.Header().Get("Cross-Origin-Opener-Policy"); got != "same-origin" {
		t.Errorf("Cross-Origin-Opener-Policy = %q, want same-origin", got)
	}
	if got := recorder.Header().Get("Permissions-Policy"); got == "" {
		t.Error("Permissions-Policy is not set")
	}
	if got := recorder.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("Strict-Transport-Security = %q, want it unset", got)
	}
}

func TestStackRecoversFromPanicsAndLogs500(t *testing.T) {
	var logs bytes.Buffer
	handler := Stack(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}), Options{Logger: slog.New(slog.NewJSONHandler(&logs, nil))})

	request := httptest.NewRequest(http.MethodGet, "http://example.com/boom", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("panic returned %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	requestLog := findLog(t, &logs, "request")
	if got := requestLog["status"]; got != float64(http.StatusInternalServerError) {
		t.Errorf("logged status = %v, want 500", got)
	}
	if got := requestLog["path"]; got != "/boom" {
		t.Errorf("logged path = %v, want /boom", got)
	}
	findLog(t, &logs, "panic serving request")
}

func TestStackAbortsPanicsAfterHeaders(t *testing.T) {
	var logs bytes.Buffer
	handler := Stack(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial"))
		panic("test panic")
	}), Options{Logger: slog.New(slog.NewJSONHandler(&logs, nil))})

	request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	request.RemoteAddr = "192.0.2.1:1234"

	defer func() {
		// net/http treats ErrAbortHandler as "close the connection silently".
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", recovered)
		}
		if entry := findLog(t, &logs, "request"); entry["aborted"] != true {
			t.Errorf("request log = %v, want aborted=true", entry)
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), request)
	t.Fatal("ServeHTTP returned normally, want an aborted response")
}

func TestRequestLoggerRecordsResponse(t *testing.T) {
	var logs bytes.Buffer
	handler := Stack(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}), Options{
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)),
		TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
	})

	request := httptest.NewRequest(http.MethodPost, "http://example.com/items", nil)
	request.RemoteAddr = "10.0.0.5:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.7")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	entry := findLog(t, &logs, "request")
	want := map[string]any{
		"method": "POST",
		"path":   "/items",
		"status": float64(http.StatusCreated),
		"bytes":  float64(len("created")),
		"ip":     "198.51.100.7",
	}
	for key, value := range want {
		if entry[key] != value {
			t.Errorf("logged %s = %v, want %v", key, entry[key], value)
		}
	}
	if _, ok := entry["duration"]; !ok {
		t.Error("duration was not logged")
	}
}

func TestCrossOriginProtection(t *testing.T) {
	handler := Stack(http.HandlerFunc(okHandler), discardOptions)

	tests := []struct {
		name   string
		method string
		header map[string]string
		want   int
	}{
		{"cross-site POST", http.MethodPost, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"cross-origin POST without fetch metadata", http.MethodPost, map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"same-origin POST", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"POST from non-browser client", http.MethodPost, nil, http.StatusOK},
		{"cross-site GET", http.MethodGet, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusOK},
		{"cross-site HEAD", http.MethodHead, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "http://example.com/examples/counter", nil)
			request.RemoteAddr = "192.0.2.1:1234"
			for key, value := range test.header {
				request.Header.Set(key, value)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != test.want {
				t.Fatalf("%s returned %d, want %d", test.method, recorder.Code, test.want)
			}
		})
	}
}

func TestIPRateLimiterUsesRemoteAddress(t *testing.T) {
	limiter := newIPRateLimiter(2, time.Minute, nil)
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

func TestIPRateLimiterSeparatesClientsBehindTrustedProxy(t *testing.T) {
	limiter := newIPRateLimiter(1, time.Minute, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, client := range []string{"198.51.100.1", "198.51.100.2"} {
		request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		request.RemoteAddr = "10.0.0.1:1234"
		request.Header.Set("X-Forwarded-For", client)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("client %s returned %d, want %d", client, recorder.Code, http.StatusNoContent)
		}
	}
}

func TestIPRateLimiterGroupsIPv6By64(t *testing.T) {
	limiter := newIPRateLimiter(1, time.Minute, nil)
	handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	codes := make([]int, 0, 2)
	for _, remote := range []string{"[2001:db8:1:2::1]:1234", "[2001:db8:1:2::ffff]:1234"} {
		request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		request.RemoteAddr = remote
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		codes = append(codes, recorder.Code)
	}
	if codes[0] != http.StatusNoContent || codes[1] != http.StatusTooManyRequests {
		t.Fatalf("status codes = %v, want [204 429]", codes)
	}
}

func TestIPRateLimiterExemptsStaticAndHealthz(t *testing.T) {
	limiter := newIPRateLimiter(1, time.Minute, nil)
	handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, path := range []string{"/static/css/styles.css", "/static/js/htmx.min.js", "/healthz", "/healthz", "/"} {
		request := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
		request.RemoteAddr = "192.0.2.1:1234"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s returned %d, want %d", path, recorder.Code, http.StatusNoContent)
		}
	}

	// The exempt requests above did not use up the single allowed request.
	request := httptest.NewRequest(http.MethodGet, "http://example.com/about", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("second limited request returned %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
}

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("2001:db8:ffff::/48"),
	}

	tests := []struct {
		name    string
		remote  string
		xff     []string
		trusted []netip.Prefix
		want    string
	}{
		{"no trusted proxies ignores header", "192.0.2.1:1234", []string{"198.51.100.7"}, nil, "192.0.2.1"},
		{"spoofed header from untrusted peer", "192.0.2.1:1234", []string{"198.51.100.7"}, trusted, "192.0.2.1"},
		{"trusted proxy", "10.0.0.1:1234", []string{"198.51.100.7"}, trusted, "198.51.100.7"},
		{"client-supplied prefix ignored", "10.0.0.1:1234", []string{"203.0.113.9, 198.51.100.7"}, trusted, "198.51.100.7"},
		{"proxy chain", "10.0.0.1:1234", []string{"203.0.113.9, 198.51.100.7, 10.0.0.2"}, trusted, "198.51.100.7"},
		{"multiple header lines", "10.0.0.1:1234", []string{"203.0.113.9", "198.51.100.7, 10.0.0.2"}, trusted, "198.51.100.7"},
		{"all hops trusted", "10.0.0.1:1234", []string{"10.0.0.3, 10.0.0.2"}, trusted, "10.0.0.3"},
		{"garbage stops the walk", "10.0.0.1:1234", []string{"unknown, 10.0.0.2"}, trusted, "10.0.0.2"},
		{"missing header", "10.0.0.1:1234", nil, trusted, "10.0.0.1"},
		{"IPv4-mapped trusted peer", "[::ffff:10.0.0.1]:1234", []string{"198.51.100.7"}, trusted, "198.51.100.7"},
		{"IPv6 proxy and client", "[2001:db8:ffff::1]:1234", []string{"2001:db8:1::5"}, trusted, "2001:db8:1::5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
			request.RemoteAddr = test.remote
			for _, value := range test.xff {
				request.Header.Add("X-Forwarded-For", value)
			}

			if got := clientIP(request, test.trusted).String(); got != test.want {
				t.Fatalf("clientIP = %s, want %s", got, test.want)
			}
		})
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
	if got := decodeGzip(t, recorder.Body); got != "hello from Go" {
		t.Fatalf("decoded body = %q, want %q", got, "hello from Go")
	}
}

// TestGzipResponsesReusesWriters runs concurrent requests through the pooled
// compressors; with -race it also catches writers shared between responses.
func TestGzipResponsesReusesWriters(t *testing.T) {
	handler := gzipResponses(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(strings.Repeat(r.URL.Query().Get("body"), 100)))
	}))

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Go(func() {
			body := "response-" + strconv.Itoa(i)
			request := httptest.NewRequest(http.MethodGet, "http://example.com/?body="+body, nil)
			request.Header.Set("Accept-Encoding", "gzip")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if got, want := decodeGzip(t, recorder.Body), strings.Repeat(body, 100); got != want {
				t.Errorf("request %d decoded %d bytes, want %q repeated", i, len(got), body)
			}
		})
	}
	wg.Wait()
}

func TestGzipResponsesSkipsEncodedResponses(t *testing.T) {
	handler := gzipResponses(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Content-Encoding", "br")
		_, _ = w.Write([]byte("precompressed"))
	}))

	request := httptest.NewRequest(http.MethodGet, "http://example.com/static/css/styles.css", nil)
	request.Header.Set("Accept-Encoding", "gzip, br")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
	if got := recorder.Body.String(); got != "precompressed" {
		t.Fatalf("body = %q, want it passed through unchanged", got)
	}
}

// TestStackFlushesGzipStreams checks that a flushed chunk reaches the client
// before the handler returns, through every wrapper in the stack.
func TestStackFlushesGzipStreams(t *testing.T) {
	release := make(chan struct{})
	handler := Stack(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("first"))
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("flush: %v", err)
		}
		<-release
		_, _ = w.Write([]byte("second"))
	}), discardOptions)

	server := httptest.NewServer(handler)
	defer server.Close()
	defer close(release)

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Setting Accept-Encoding stops the transport from decoding transparently.
	request.Header.Set("Accept-Encoding", "gzip")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if got := response.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}

	chunk := make(chan string, 1)
	go func() {
		reader, err := gzip.NewReader(response.Body)
		if err != nil {
			chunk <- "error: " + err.Error()
			return
		}
		buf := make([]byte, len("first"))
		if _, err := io.ReadFull(reader, buf); err != nil {
			chunk <- "error: " + err.Error()
			return
		}
		chunk <- string(buf)
	}()

	select {
	case got := <-chunk:
		if got != "first" {
			t.Fatalf("first chunk = %q, want %q", got, "first")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("flushed chunk did not arrive before the handler finished")
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

func decodeGzip(t *testing.T, body io.Reader) string {
	t.Helper()
	reader, err := gzip.NewReader(body)
	if err != nil {
		t.Errorf("opening gzip response: %v", err)
		return ""
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("reading gzip response: %v", err)
	}
	return string(decoded)
}

// findLog returns the first JSON log entry with the given message, or nil.
func findLog(t *testing.T, logs *bytes.Buffer, message string) map[string]any {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("parsing log line %q: %v", line, err)
		}
		if entry["msg"] == message {
			return entry
		}
	}
	t.Errorf("no %q log entry in:\n%s", message, logs)
	return nil
}
