#!/bin/bash

# Script to generate a new reusable component
# Usage: bash scripts/new-component.sh componentname

set -e

if [ -z "$1" ]; then
    echo "Usage: bash scripts/new-component.sh <componentname>"
    echo "Example: bash scripts/new-component.sh card"
    exit 1
fi

COMPONENT_NAME=$1
COMPONENT_NAME_LOWER=$(echo "$COMPONENT_NAME" | tr '[:upper:]' '[:lower:]')
COMPONENT_NAME_TITLE=$(echo "$COMPONENT_NAME" | sed 's/.*/\u&/')

echo "Creating new component: $COMPONENT_NAME_TITLE"

# Create templ file
cat > "templates/components/${COMPONENT_NAME_LOWER}.templ" << EOF
package components

templ ${COMPONENT_NAME_TITLE}(title string) {
	<div class="card bg-base-100 shadow-xl">
		<div class="card-body">
			<h2 class="card-title">{ title }</h2>
			<p>This is a reusable ${COMPONENT_NAME_TITLE} component.</p>
		</div>
	</div>
}
EOF

echo "✓ Created templates/components/${COMPONENT_NAME_LOWER}.templ"
echo ""
echo "You can now use this component in your pages:"
echo "  @components.${COMPONENT_NAME_TITLE}(\"My Title\")"
echo ""
echo "Don't forget to run: templ generate"
echo ""

