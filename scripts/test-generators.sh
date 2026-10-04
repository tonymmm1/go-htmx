#!/usr/bin/env bash

# Smoke-test both generators after changing the project module path.

set -euo pipefail

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
readonly SOURCE_MODULE="$(awk '$1 == "module" { gsub(/"/, "", $2); print $2; exit }' "$PROJECT_ROOT/go.mod")"
readonly TEST_MODULE="example.com/scaffold/site"
readonly TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/go-htmx-generators.XXXXXX")"

cleanup() {
    case "$TEST_ROOT" in
        "${TMPDIR:-/tmp}"/go-htmx-generators.*) rm -rf -- "$TEST_ROOT" ;;
        *) echo "Refusing to remove unexpected test directory: $TEST_ROOT" >&2 ;;
    esac
}
trap cleanup EXIT

mkdir -p "$TEST_ROOT"
cp "$PROJECT_ROOT/go.mod" "$PROJECT_ROOT/go.sum" "$TEST_ROOT/"
cp -R "$PROJECT_ROOT/cmd" "$PROJECT_ROOT/internal" "$PROJECT_ROOT/static" "$PROJECT_ROOT/templates" "$PROJECT_ROOT/scripts" "$TEST_ROOT/"

replace_module() {
    local file="$1"
    local temporary="${file}.module-test"
    sed "s|$SOURCE_MODULE|$TEST_MODULE|g" "$file" > "$temporary"
    mv -- "$temporary" "$file"
}

replace_module "$TEST_ROOT/go.mod"
while IFS= read -r -d '' file; do
    replace_module "$file"
done < <(find "$TEST_ROOT/cmd" "$TEST_ROOT/internal" "$TEST_ROOT/static" "$TEST_ROOT/templates" -type f \( -name '*.go' -o -name '*.templ' \) -print0)

cd "$TEST_ROOT"
bash scripts/new-page.sh contact-us
bash scripts/new-component.sh feature-card

grep -Fq 'example.com/scaffold/site/templates/layouts' templates/pages/contact-us.templ
grep -Fq 'mux.HandleFunc("GET /contact-us", h.HandleContactUs)' internal/pages/pages.go
grep -Fq 'func (h *Handler) HandleContactUs' internal/pages/pages.go
grep -Fq 'RenderPage(w, r, http.StatusOK, pagetemplates.ContactUs())' internal/pages/pages.go
grep -Fq '@layouts.Layout(layouts.Meta{' templates/pages/contact-us.templ
grep -Fq 'Title:       "Contact Us",' templates/pages/contact-us.templ
grep -Fq 'Path:        "/contact-us",' templates/pages/contact-us.templ
grep -Fq 'templ FeatureCard(title string)' templates/components/feature-card.templ

# The generated route must be served by the new handler, not the 404 catch-all.
cat > internal/pages/generated_page_test.go <<'EOF'
package pages

import (
	"net/http"
	"strings"
	"testing"
)

func TestGeneratedPage(t *testing.T) {
	recorder := performRequest(http.MethodGet, "/contact-us", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /contact-us returned %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"<title>Contact Us · Go-HTMX</title>", "Welcome to the Contact Us page."} {
		if !strings.Contains(body, expected) {
			t.Errorf("GET /contact-us does not contain %q", expected)
		}
	}
}
EOF

go tool templ generate
go vet ./...
go test ./...

echo "Generator smoke test passed with module $TEST_MODULE"
