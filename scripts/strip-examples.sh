#!/usr/bin/env bash

# Remove the template's demo content and its own documentation, leaving a
# clean base for a new site: the /examples page, its handlers, templates and
# tests, the links to it, and the README/QUICKSTART/COMPARISON that describe
# the template rather than your project. Safe to re-run.
# Usage: bash scripts/strip-examples.sh   (or: make strip-examples)

set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

# Everything the demo needs lives in these files.
rm -f -- \
    internal/pages/examples.go \
    internal/pages/examples_test.go \
    templates/pages/examples.templ templates/pages/examples_templ.go \
    templates/components/examples.templ templates/components/examples_templ.go \
    QUICKSTART.md COMPARISON.md create-go-htmx.sh

# drop_lines <pattern> <file>...: delete matching lines (missing files are skipped).
drop_lines() {
    local pattern="$1" file
    shift
    for file in "$@"; do
        [[ -f "$file" ]] || continue
        grep -vE -- "$pattern" "$file" > "$file.strip" || true
        if cmp -s -- "$file" "$file.strip"; then
            rm -f -- "$file.strip"
        else
            mv -- "$file.strip" "$file"
        fi
    done
}

# The route registration, sitemap entry, nav link and home page button.
drop_lines 'registerExampleRoutes|"/examples"' \
    internal/pages/pages.go templates/layouts/layout.templ templates/pages/index.templ
# Pointers to the live examples in the agent docs.
drop_lines 'examples\.(go|templ)' AGENTS.md docs/agents/recipes.md docs/agents/architecture.md

# Replace the template's README, but never one you have already rewritten.
if [[ -f README.md ]] && grep -q '^# Go-HTMX Starter Template' README.md; then
    name="$(awk '$1 == "module" { n = split($2, parts, "/"); print parts[n]; exit }' go.mod)"
    cat > README.md <<README
# ${name}

Server-rendered website: Go (\`net/http\`), templ, htmx, Tailwind CSS and daisyUI, built into one static
binary. Based on the [go-htmx](https://github.com/tonymmm1/go-htmx) template.

\`\`\`bash
make setup   # once: .env, dependencies, CSS tools
make dev     # http://localhost:7331 with live reload
make check   # format check, lint, tests, build
make build   # ./bin/server with all assets embedded
\`\`\`

- Site name, canonical URL, footer, rate limit and extra CSP sources: \`internal/site/site.go\`
- Environment variables (\`PORT\`, \`APP_ENV\`, \`TRUSTED_PROXIES\`): \`.env.example\`
- How the code is organised and how to add pages, forms and assets: [AGENTS.md](AGENTS.md) and \`docs/agents/\`
- Styling and themes: [docs/ui.md](docs/ui.md)

Behind a reverse proxy, set \`TRUSTED_PROXIES\`, or every visitor shares one rate limit.
README
fi

echo "Removed the examples and the template's own docs."
echo "Next: edit internal/site/site.go, then run 'make generate' and 'make check'."
