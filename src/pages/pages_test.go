package pages

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestExamplesPageIncludesWorkingHTMXPatterns(t *testing.T) {
	recorder := performRequest(http.MethodGet, "/examples", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /examples returned %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	for _, expected := range []string{
		`hx-get="/examples/time"`,
		`hx-get="/examples/search"`,
		`hx-post="/examples/counter"`,
		`hx-trigger="input changed delay:300ms, search"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("GET /examples response does not contain %q", expected)
		}
	}
}

func TestExampleFragments(t *testing.T) {
	t.Run("server time", func(t *testing.T) {
		recorder := performRequest(http.MethodGet, "/examples/time", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET /examples/time returned %d, want %d", recorder.Code, http.StatusOK)
		}
		if !strings.Contains(recorder.Body.String(), "Server rendered this at") {
			t.Fatalf("GET /examples/time did not return the expected fragment: %s", recorder.Body.String())
		}
	})

	t.Run("live search", func(t *testing.T) {
		recorder := performRequest(http.MethodGet, "/examples/search?q=htmx", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET /examples/search returned %d, want %d", recorder.Code, http.StatusOK)
		}

		body := recorder.Body.String()
		if !strings.Contains(body, "HTMX partial swaps") {
			t.Errorf("search response did not include the matching topic: %s", body)
		}
		if strings.Contains(body, "Docker deployment") {
			t.Errorf("search response included a non-matching topic: %s", body)
		}
	})

	t.Run("counter post", func(t *testing.T) {
		form := url.Values{"count": {"3"}, "action": {"increment"}}.Encode()
		recorder := performRequest(http.MethodPost, "/examples/counter", form)
		if recorder.Code != http.StatusOK {
			t.Fatalf("POST /examples/counter returned %d, want %d", recorder.Code, http.StatusOK)
		}

		body := recorder.Body.String()
		if !strings.Contains(body, `value="4"`) || !strings.Contains(body, ">4</output>") {
			t.Errorf("counter response did not contain the incremented value: %s", body)
		}
	})

	t.Run("counter clamps submitted values", func(t *testing.T) {
		form := url.Values{"count": {"999"}, "action": {"increment"}}.Encode()
		recorder := performRequest(http.MethodPost, "/examples/counter", form)
		if recorder.Code != http.StatusOK {
			t.Fatalf("POST /examples/counter returned %d, want %d", recorder.Code, http.StatusOK)
		}
		if !strings.Contains(recorder.Body.String(), `value="10"`) {
			t.Errorf("counter response did not clamp the value: %s", recorder.Body.String())
		}
	})
}

func TestExampleCounterRejectsInvalidInput(t *testing.T) {
	form := url.Values{"count": {"not-a-number"}, "action": {"increment"}}.Encode()
	recorder := performRequest(http.MethodPost, "/examples/counter", form)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST /examples/counter returned %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func performRequest(method, target, form string) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	RegisterPageRoutes(&Handler{}, router)

	request := httptest.NewRequest(method, target, strings.NewReader(form))
	if form != "" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
