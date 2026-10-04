#!/usr/bin/env bash

# Generate a reusable Templ component.
# Usage: bash scripts/new-component.sh feature-card

set -euo pipefail

readonly COMPONENT_NAME="${1:-}"
if [[ -z "$COMPONENT_NAME" ]]; then
    echo "Usage: bash scripts/new-component.sh <component-name>"
    echo "Example: bash scripts/new-component.sh feature-card"
    exit 1
fi

if [[ ! "$COMPONENT_NAME" =~ ^[a-zA-Z][a-zA-Z0-9]*([_-][a-zA-Z0-9]+)*$ ]]; then
    echo "Error: component name must start with a letter and contain only letters, numbers, hyphens, or underscores."
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

readonly COMPONENT_SLUG="$(printf '%s' "$COMPONENT_NAME" | tr '[:upper:]_' '[:lower:]-')"
readonly COMPONENT_TYPE="$(printf '%s\n' "$COMPONENT_NAME" | awk -F '[-_]' '{ for (i = 1; i <= NF; i++) printf "%s%s", toupper(substr($i, 1, 1)), substr($i, 2); print "" }')"
readonly COMPONENT_FILE="templates/components/${COMPONENT_SLUG}.templ"
readonly COMPONENT_TMP="${COMPONENT_FILE}.tmp"

if [[ -e "$COMPONENT_FILE" ]]; then
    echo "Error: $COMPONENT_FILE already exists."
    exit 1
fi

trap 'rm -f -- "$COMPONENT_TMP"' EXIT
mkdir -p templates/components

cat > "$COMPONENT_TMP" <<EOF
package components

templ ${COMPONENT_TYPE}(title string) {
	<div class="card bg-base-100 shadow-xl">
		<div class="card-body">
			<h2 class="card-title">{ title }</h2>
			<p>This is a reusable ${COMPONENT_TYPE} component.</p>
		</div>
	</div>
}
EOF

mv -- "$COMPONENT_TMP" "$COMPONENT_FILE"
trap - EXIT

echo "Created $COMPONENT_FILE"
echo "Import and use it in a page:"
echo "  import \"$MODULE_PATH/templates/components\""
echo "  @components.${COMPONENT_TYPE}(\"My Title\")"
echo "Run 'make generate' (or leave 'make dev' running) to generate the Go template."
