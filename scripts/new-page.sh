#!/bin/bash

# Script to generate a new page quickly
# Usage: bash scripts/new-page.sh pagename

set -e

if [ -z "$1" ]; then
    echo "Usage: bash scripts/new-page.sh <pagename>"
    echo "Example: bash scripts/new-page.sh contact"
    exit 1
fi

PAGE_NAME=$1
PAGE_NAME_LOWER=$(echo "$PAGE_NAME" | tr '[:upper:]' '[:lower:]')
PAGE_NAME_TITLE=$(echo "$PAGE_NAME" | sed 's/.*/\u&/')

echo "Creating new page: $PAGE_NAME_TITLE"

# Create templ file
cat > "templates/pages/${PAGE_NAME_LOWER}.templ" << EOF
package pages

templ ${PAGE_NAME_TITLE}() {
	@Layout() {
		<div class="container mx-auto px-4 py-8">
			<div class="prose lg:prose-xl mx-auto">
				<h1>${PAGE_NAME_TITLE}</h1>
				<p>Welcome to the ${PAGE_NAME_TITLE} page.</p>
				<div class="mt-8">
					<a href="/" class="btn btn-primary">Back to Home</a>
				</div>
			</div>
		</div>
	}
}
EOF

echo "✓ Created templates/pages/${PAGE_NAME_LOWER}.templ"

# Add handler to pages.go
cat >> "src/pages/pages.go" << EOF

func (h *Handler) Handle${PAGE_NAME_TITLE}(w http.ResponseWriter, r *http.Request) {
	component := ${PAGE_NAME_TITLE}()
	component.Render(r.Context(), w)
}
EOF

echo "✓ Added handler to src/pages/pages.go"

# Instructions for adding route
echo ""
echo "To complete setup, add this route to RegisterPageRoutes in src/pages/pages.go:"
echo "  r.Get(\"/${PAGE_NAME_LOWER}\", h.Handle${PAGE_NAME_TITLE})"
echo ""
echo "Then run: templ generate"
echo ""

