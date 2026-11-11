package pages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tonymmm1/go-htmx/internal/site"
)

func TestStandardLibraryRouting(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		want   int
	}{
		{name: "exact root", method: http.MethodGet, target: "/", want: http.StatusOK},
		{name: "unknown route", method: http.MethodGet, target: "/missing", want: http.StatusNotFound},
		{name: "nested unknown route", method: http.MethodGet, target: "/about/missing", want: http.StatusNotFound},
		{name: "method mismatch", method: http.MethodPost, target: "/about", want: http.StatusMethodNotAllowed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := performRequest(test.method, test.target, "")
			if recorder.Code != test.want {
				t.Fatalf("%s %s returned %d, want %d", test.method, test.target, recorder.Code, test.want)
			}
		})
	}
}

func TestRobotsSitemapAndFavicon(t *testing.T) {
	robots := performRequest(http.MethodGet, "/robots.txt", "")
	if robots.Code != http.StatusOK || !strings.HasPrefix(robots.Body.String(), "User-agent: *\n") {
		t.Errorf("GET /robots.txt returned %d %q", robots.Code, robots.Body.String())
	}
	if strings.Contains(robots.Body.String(), "Sitemap:") {
		t.Error("robots.txt advertises a sitemap although site.URL is empty")
	}
	if code := performRequest(http.MethodGet, "/sitemap.xml", "").Code; code != http.StatusNotFound {
		t.Errorf("GET /sitemap.xml without site.URL returned %d, want 404", code)
	}

	favicon := performRequest(http.MethodGet, "/favicon.ico", "")
	if location := favicon.Header().Get("Location"); favicon.Code != http.StatusFound || !strings.HasPrefix(location, "/static/img/favicon.svg?v=") {
		t.Errorf("GET /favicon.ico returned %d, Location %q", favicon.Code, location)
	}

	setSiteURL(t, "https://example.com")

	if body := performRequest(http.MethodGet, "/robots.txt", "").Body.String(); !strings.Contains(body, "Sitemap: https://example.com/sitemap.xml\n") {
		t.Errorf("robots.txt does not point at the sitemap: %q", body)
	}
	sitemap := performRequest(http.MethodGet, "/sitemap.xml", "")
	if sitemap.Code != http.StatusOK || !strings.HasPrefix(sitemap.Header().Get("Content-Type"), "application/xml") {
		t.Fatalf("GET /sitemap.xml returned %d %q", sitemap.Code, sitemap.Header().Get("Content-Type"))
	}
	for _, expected := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`,
		"<url><loc>https://example.com/</loc></url>",
		"<url><loc>https://example.com/about</loc></url>",
	} {
		if !strings.Contains(sitemap.Body.String(), expected) {
			t.Errorf("sitemap does not contain %q: %s", expected, sitemap.Body.String())
		}
	}
}

// Every page in the sitemap must be served, or crawlers are sent to 404s.
func TestSitemapPathsAreRouted(t *testing.T) {
	for _, path := range sitemapPaths {
		if code := performRequest(http.MethodGet, path, "").Code; code != http.StatusOK {
			t.Errorf("GET %s returned %d, want %d", path, code, http.StatusOK)
		}
	}
}

// setSiteURL sets site.URL for the rest of the test.
func setSiteURL(t *testing.T, url string) {
	t.Helper()
	previous := site.URL
	site.URL = url
	t.Cleanup(func() { site.URL = previous })
}

func performRequest(method, target, form string) *httptest.ResponseRecorder {
	return performRequestWithHeaders(method, target, form, nil)
}

func performRequestWithHeaders(method, target, form string, headers map[string]string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	RegisterPageRoutes(&Handler{}, mux)

	request := httptest.NewRequest(method, target, strings.NewReader(form))
	if form != "" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}
