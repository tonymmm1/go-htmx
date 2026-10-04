package pages

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestNotFoundPage(t *testing.T) {
	recorder := performRequest(http.MethodGet, "/no/such/page", "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET /no/such/page returned %d, want %d", recorder.Code, http.StatusNotFound)
	}

	body := recorder.Body.String()
	for _, expected := range []string{
		"<title>Page not found · Go-HTMX</title>",
		"Page not found",
		"/no/such/page",
		`href="/"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("404 page does not contain %q", expected)
		}
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("404 Content-Type = %q, want text/html", got)
	}
}

func TestPagesRenderContentOnlyForHTMXRequests(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		wantFull bool
	}{
		{name: "normal navigation", wantFull: true},
		{name: "boosted link", headers: map[string]string{"HX-Request": "true", "HX-Boosted": "true"}, wantFull: true},
		{name: "history restore", headers: map[string]string{"HX-Request": "true", "HX-History-Restore-Request": "true"}, wantFull: true},
		{name: "htmx request", headers: map[string]string{"HX-Request": "true"}, wantFull: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := performRequestWithHeaders(http.MethodGet, "/about", "", test.headers)
			if recorder.Code != http.StatusOK {
				t.Fatalf("GET /about returned %d, want %d", recorder.Code, http.StatusOK)
			}

			body := recorder.Body.String()
			if !strings.Contains(body, "About This Project") {
				t.Errorf("response is missing the page content: %s", body)
			}
			if full := strings.Contains(body, "<html"); full != test.wantFull {
				t.Errorf("response contains <html> = %v, want %v", full, test.wantFull)
			}
			if vary := recorder.Header().Values("Vary"); !strings.Contains(strings.Join(vary, ","), "HX-Request") {
				t.Errorf("Vary = %q, want it to include HX-Request", vary)
			}
		})
	}

	t.Run("not found keeps its status", func(t *testing.T) {
		recorder := performRequestWithHeaders(http.MethodGet, "/missing", "", map[string]string{"HX-Request": "true"})
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("GET /missing returned %d, want %d", recorder.Code, http.StatusNotFound)
		}
		if body := recorder.Body.String(); strings.Contains(body, "<html") || !strings.Contains(body, "Page not found") {
			t.Errorf("htmx 404 should be the content only: %s", body)
		}
	})

	t.Run("fragments do not vary", func(t *testing.T) {
		recorder := performRequestWithHeaders(http.MethodGet, "/examples/search?q=go", "", map[string]string{"HX-Request": "true"})
		if vary := recorder.Header().Get("Vary"); vary != "" {
			t.Errorf("fragment Vary = %q, want none", vary)
		}
	})
}

var staticURL = regexp.MustCompile(`(?:src|href)="(/static/[^"]*)"`)

func TestLayoutHead(t *testing.T) {
	recorder := performRequest(http.MethodGet, "/about", "")
	body := recorder.Body.String()

	for _, expected := range []string{
		`<html lang="en">`,
		"<title>About · Go-HTMX</title>",
		`<meta name="description" content="What is included in the Go-HTMX starter template.">`,
		`<meta name="htmx-config" content="{&#34;includeIndicatorStyles&#34;:false`,
		`<script src="/static/js/app.js?v=`,
		`<script src="/static/js/htmx.min.js?v=`,
		`<link rel="icon" href="/static/img/favicon.svg?v=`,
		`hx-boost="true"`,
		`<a href="/about" class="menu-active" aria-current="page">About</a>`,
		`<div id="toasts"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("GET /about does not contain %q", expected)
		}
	}
	// Assets missing from the //go:embed directive would 404 in production;
	// static.Path signals that by omitting the content hash.
	for _, match := range staticURL.FindAllStringSubmatch(body, -1) {
		if !strings.Contains(match[1], "?v=") {
			t.Errorf("asset %s is not embedded (no ?v= hash)", match[1])
		}
	}
	if strings.Contains(body, "data-theme=") {
		t.Error("layout hardcodes data-theme; the OS preference should decide by default")
	}
	if strings.Count(body, `aria-current="page"`) != 1 {
		t.Errorf("want exactly one aria-current link, got %d", strings.Count(body, `aria-current="page"`))
	}

	home := performRequest(http.MethodGet, "/", "").Body.String()
	if !strings.Contains(home, "<title>Go-HTMX</title>") {
		t.Error("home page title should be just the site name")
	}
}

// The production CSP is script-src 'self'; style-src 'self', so rendered pages
// must not rely on inline scripts, inline styles, or hx-on handlers.
func TestPagesAreCSPCompatible(t *testing.T) {
	scriptTag := regexp.MustCompile(`<script\b[^>]*>`)

	for _, target := range []string{"/", "/about", "/examples", "/missing"} {
		t.Run(target, func(t *testing.T) {
			body := performRequest(http.MethodGet, target, "").Body.String()
			for _, tag := range scriptTag.FindAllString(body, -1) {
				if !strings.Contains(tag, " src=") {
					t.Errorf("inline script: %s", tag)
				}
			}
			for _, forbidden := range []string{` style="`, "<style", "hx-on", "javascript:"} {
				if strings.Contains(body, forbidden) {
					t.Errorf("page contains %q", forbidden)
				}
			}
		})
	}
}

func TestRender(t *testing.T) {
	t.Run("writes status and content length", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		Render(recorder, request, http.StatusTeapot, templ.Raw("<p>hello</p>"))

		if recorder.Code != http.StatusTeapot {
			t.Errorf("status = %d, want %d", recorder.Code, http.StatusTeapot)
		}
		if got, want := recorder.Header().Get("Content-Length"), strconv.Itoa(len("<p>hello</p>")); got != want {
			t.Errorf("Content-Length = %q, want %q", got, want)
		}
		if recorder.Body.String() != "<p>hello</p>" {
			t.Errorf("body = %q", recorder.Body.String())
		}
	})

	t.Run("render error becomes a clean 500", func(t *testing.T) {
		failing := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			if _, err := io.WriteString(w, "<p>half a page"); err != nil {
				return err
			}
			return errors.New("database went away")
		})

		defer slog.SetDefault(slog.Default())
		slog.SetDefault(slog.New(slog.DiscardHandler))

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		RenderPage(recorder, request, http.StatusOK, failing)

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}
		body := recorder.Body.String()
		if strings.Contains(body, "half a page") || strings.Contains(body, "database") {
			t.Errorf("500 response leaked partial output or the error: %q", body)
		}
	})
}
