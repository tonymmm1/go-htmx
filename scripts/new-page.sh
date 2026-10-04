#!/usr/bin/env bash

# Generate a page, handler, and route.
# Usage: bash scripts/new-page.sh contact-us

set -euo pipefail

readonly PAGE_NAME="${1:-}"
if [[ -z "$PAGE_NAME" ]]; then
    echo "Usage: bash scripts/new-page.sh <page-name>"
    echo "Example: bash scripts/new-page.sh contact-us"
    exit 1
fi

if [[ ! "$PAGE_NAME" =~ ^[a-zA-Z][a-zA-Z0-9]*([_-][a-zA-Z0-9]+)*$ ]]; then
    echo "Error: page name must start with a letter and contain only letters, numbers, hyphens, or underscores."
    exit 1
fi

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

readonly MODULE_PATH="$(awk '$1 == "module" { gsub(/"/, "", $2); print $2; exit }' go.mod)"
if [[ -z "$MODULE_PATH" ]]; then
    echo "Error: could not determine the module path from go.mod."
    exit 1
fi

readonly PAGE_SLUG="$(printf '%s' "$PAGE_NAME" | tr '[:upper:]_' '[:lower:]-')"
readonly PAGE_COMPONENT="$(printf '%s\n' "$PAGE_NAME" | awk -F '[-_]' '{ for (i = 1; i <= NF; i++) printf "%s%s", toupper(substr($i, 1, 1)), substr($i, 2); print "" }')"
readonly TEMPLATE_FILE="templates/pages/${PAGE_SLUG}.templ"
readonly HANDLERS_FILE="internal/pages/pages.go"
readonly HANDLERS_TMP="${HANDLERS_FILE}.tmp"
readonly TEMPLATE_TMP="${TEMPLATE_FILE}.tmp"

if [[ -e "$TEMPLATE_FILE" ]]; then
    echo "Error: $TEMPLATE_FILE already exists."
    exit 1
fi

if grep -Fq "Handle${PAGE_COMPONENT}" "$HANDLERS_FILE"; then
    echo "Error: handler Handle${PAGE_COMPONENT} already exists."
    exit 1
fi

if grep -Fq "mux.HandleFunc(\"GET /${PAGE_SLUG}\"" "$HANDLERS_FILE"; then
    echo "Error: route /${PAGE_SLUG} already exists."
    exit 1
fi

if ! grep -Fq "// scaffold:routes" "$HANDLERS_FILE"; then
    echo "Error: route marker not found in $HANDLERS_FILE."
    exit 1
fi

trap 'rm -f -- "$HANDLERS_TMP" "$TEMPLATE_TMP"' EXIT
mkdir -p templates/pages

cat > "$TEMPLATE_TMP" <<EOF
package pagetemplates

import "$MODULE_PATH/templates/layouts"

templ ${PAGE_COMPONENT}() {
	@layouts.Layout() {
		<div class="container mx-auto px-4 py-8">
			<div class="prose lg:prose-xl mx-auto">
				<h1>${PAGE_COMPONENT}</h1>
				<p>Welcome to the ${PAGE_COMPONENT} page.</p>
				<div class="mt-8">
					<a href="/" class="btn btn-primary">Back to Home</a>
				</div>
			</div>
		</div>
	}
}
EOF

awk -v route="mux.HandleFunc(\"GET /${PAGE_SLUG}\", h.Handle${PAGE_COMPONENT})" '
    /\/\/ scaffold:routes/ { print "\t" route }
    { print }
' "$HANDLERS_FILE" > "$HANDLERS_TMP"

cat >> "$HANDLERS_TMP" <<EOF

func (h *Handler) Handle${PAGE_COMPONENT}(w http.ResponseWriter, r *http.Request) {
	render(w, r, pagetemplates.${PAGE_COMPONENT}())
}
EOF

mv -- "$TEMPLATE_TMP" "$TEMPLATE_FILE"
mv -- "$HANDLERS_TMP" "$HANDLERS_FILE"
gofmt -w "$HANDLERS_FILE"
trap - EXIT

echo "Created $TEMPLATE_FILE"
echo "Added /${PAGE_SLUG} and Handle${PAGE_COMPONENT} to $HANDLERS_FILE"
echo "Run 'templ generate' (or leave 'make dev' running) to generate the Go template."
