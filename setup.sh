#!/usr/bin/env bash

# Setup script for the go-htmx template.
#
# Usage: bash setup.sh [module-path]
#
# Safe to re-run. It:
#   1. checks prerequisites (Go 1.27+, Node.js 22+, npm, make),
#   2. optionally sets the module path in go.mod, then rewrites any remaining
#      template import paths in .go/.templ files (also fixes up projects created
#      with `gonew`, which only rewrites .go files),
#   3. creates .env from .env.example,
#   4. installs dependencies, generates templ code, builds CSS and compiles.

set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

readonly TEMPLATE_MODULE="github.com/tonymmm1/go-htmx"
readonly NEW_MODULE="${1:-}"

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"

step() { echo -e "\n${BLUE}$*${NC}"; }
ok() { echo -e "${GREEN}✓ $*${NC}"; }
fail() { echo -e "${RED}✗ $*${NC}" >&2; exit 1; }

echo -e "${BLUE}"
cat << "EOF"
   ____          _   _ _____ __  ____  __
  / ___| ___    | | | |_   _|  \/  \ \/ /
 | |  _ / _ \   | |_| | | | | |\/| |\  /
 | |_| | (_) |  |  _  | | | | |  | |/  \
  \____|\___/___|_| |_| |_| |_|  |_/_/\_\
           |_____|

Go + HTMX + Templ + Tailwind CSS Starter
EOF
echo -e "${NC}"

# --- Prerequisites ----------------------------------------------------------
step "Checking prerequisites..."

command -v go &> /dev/null || fail "Go is not installed. Please install Go 1.27 or later."
GO_VERSION=$(go env GOVERSION)
if [[ ! "$GO_VERSION" =~ ^go([0-9]+)\.([0-9]+) ]] ||
    (( BASH_REMATCH[1] < 1 || (BASH_REMATCH[1] == 1 && BASH_REMATCH[2] < 27) )); then
    fail "Go 1.27 or later is required (found ${GO_VERSION})."
fi
ok "Go ${GO_VERSION}"

command -v node &> /dev/null || fail "Node.js is not installed. Please install Node.js 22 or later."
NODE_VERSION=$(node --version)
if [[ ! "$NODE_VERSION" =~ ^v([0-9]+)\. ]] || (( BASH_REMATCH[1] < 22 )); then
    fail "Node.js 22 or later is required (found ${NODE_VERSION})."
fi
ok "Node.js ${NODE_VERSION}"

command -v npm &> /dev/null || fail "npm is not installed."
ok "npm $(npm --version)"

command -v make &> /dev/null || fail "make is not installed."
ok "make"

# --- Module path ------------------------------------------------------------
step "Checking module path..."

if [[ -n "$NEW_MODULE" ]]; then
    [[ "$NEW_MODULE" =~ ^[A-Za-z0-9._~-]+(/[A-Za-z0-9._~-]+)*$ ]] || fail "Invalid module path: $NEW_MODULE"
    go mod edit -module "$NEW_MODULE"
fi

CURRENT_MODULE=$(awk '$1 == "module" { gsub(/"/, "", $2); print $2; exit }' go.mod)

if [[ "$CURRENT_MODULE" != "$TEMPLATE_MODULE" ]]; then
    # Match the template path only as a whole path element, so re-running is a
    # no-op even when the new path starts with the template path.
    old_pattern="${TEMPLATE_MODULE//./\\.}\\([/\"]\\)"
    updated=0
    while IFS= read -r -d '' file; do
        if grep -q "${TEMPLATE_MODULE}[/\"]" "$file"; then
            sed "s#${old_pattern}#${CURRENT_MODULE}\\1#g" "$file" > "$file.module-update"
            mv -- "$file.module-update" "$file"
            updated=$((updated + 1))
        fi
    done < <(find cmd internal static templates scripts -type f \
        \( -name '*.go' -o -name '*.templ' -o -name '*.sh' \) ! -name '*_templ.go' -print0)
    ok "Module is $CURRENT_MODULE (updated $updated file(s))"
else
    ok "Module is $CURRENT_MODULE"
fi

# --- Environment ------------------------------------------------------------
step "Checking .env..."
if [[ -f .env ]]; then
    echo -e "${YELLOW}⚠ .env already exists, leaving it alone${NC}"
else
    cp .env.example .env
    ok ".env created from .env.example"
fi

# --- Dependencies, code generation, CSS ------------------------------------
step "Installing dependencies..."
make --no-print-directory deps
ok "Go modules and npm packages installed"

step "Generating templ code and CSS..."
make --no-print-directory generate css
ok "Templ code and CSS generated"

step "Compiling..."
go build ./...
ok "Project compiles"

echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ Setup complete! Your project is ready.${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo ""
echo -e "${BLUE}Start the development server (hot reload):${NC}"
echo -e "  ${YELLOW}make dev${NC}   then open ${GREEN}http://localhost:7331${NC}"
echo ""
echo -e "${BLUE}Other useful commands:${NC}"
echo -e "  ${YELLOW}make build${NC}       - Build for production"
echo -e "  ${YELLOW}make check${NC}       - Format check, lint, tests (what CI runs)"
echo -e "  ${YELLOW}make docker-up${NC}   - Run in Docker"
echo -e "  ${YELLOW}make help${NC}        - Show all commands"
echo ""
