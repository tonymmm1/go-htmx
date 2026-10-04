#!/usr/bin/env bash

# Fail if agent-facing docs mention repo paths or make targets that don't exist,
# so AGENTS.md stays accurate as the code changes. A path is checked when its
# first segment is a top-level entry of the repo (e.g. `internal/pages/pages.go`),
# so Go import paths like `net/http` are ignored.
# Usage: bash scripts/check-docs.sh [file...]

set -euo pipefail

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

if [[ $# -eq 0 ]]; then
    set -- AGENTS.md docs/ui.md
fi

readonly TARGETS="$(grep -oE '^[a-zA-Z][a-zA-Z0-9_-]*:' Makefile | tr -d ':')"
status=0

for doc in "$@"; do
    # Repo paths in `code spans` and relative markdown links.
    while IFS= read -r path; do
        path="${path%%:*}"  # strip ":line" or trailing colon
        path="${path%/}"
        [[ -e "${path%%/*}" ]] || continue
        [[ -e "$path" ]] && continue
        echo "$doc: path does not exist: $path"
        status=1
    done < <(
        {
            grep -oE '`[^` ]+`' "$doc" | tr -d '`'
            grep -oE '\]\([^)#]+\)' "$doc" | sed -E 's/^\]\(//; s/\)$//'
        } | grep -E '^[a-zA-Z0-9_.-]+(/[a-zA-Z0-9_.-]+)*/?$' \
          | sort -u
    )

    # `make <target>` invocations.
    while IFS= read -r target; do
        grep -qx "$target" <<<"$TARGETS" && continue
        echo "$doc: make target does not exist: $target"
        status=1
    done < <(grep -oE 'make [a-z][a-z0-9-]*' "$doc" | awk '{ print $2 }' | sort -u)
done

if [[ $status -eq 0 ]]; then
    echo "Docs reference only existing paths and make targets: $*"
fi
exit "$status"
