#!/bin/bash

# create-go-htmx: Scaffold a new go-htmx project
# Usage: bash create-go-htmx.sh [project-name]
# Or: curl -sSL <url> | bash -s my-project

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

TEMPLATE_REPO="https://github.com/tonymmm1/go-htmx.git"

# Banner
echo -e "${BLUE}"
cat << "EOF"
   ____          _   _ _____ __  ____  __
  / ___| ___    | | | |_   _|  \/  \ \/ /
 | |  _ / _ \   | |_| | | | | |\/| |\  / 
 | |_| | (_) |  |  _  | | | | |  | |/  \ 
  \____|\___/___|_| |_| |_| |_|  |_/_/\_\
           |_____|                        

Project Scaffolder
EOF
echo -e "${NC}"

# Get project name
if [ -z "$1" ]; then
    echo -e "${YELLOW}Enter project name:${NC}"
    read -r PROJECT_NAME
else
    PROJECT_NAME="$1"
fi

if [ -z "$PROJECT_NAME" ]; then
    echo -e "${RED}Error: Project name cannot be empty${NC}"
    exit 1
fi

# Validate project name
if [[ ! "$PROJECT_NAME" =~ ^[a-zA-Z0-9_-]+$ ]]; then
    echo -e "${RED}Error: Project name can only contain letters, numbers, hyphens, and underscores${NC}"
    exit 1
fi

# Check if directory exists
if [ -d "$PROJECT_NAME" ]; then
    echo -e "${RED}Error: Directory '$PROJECT_NAME' already exists${NC}"
    exit 1
fi

# Get module path
echo -e "${YELLOW}Enter Go module path (e.g., github.com/username/$PROJECT_NAME):${NC}"
read -r MODULE_PATH

if [ -z "$MODULE_PATH" ]; then
    MODULE_PATH="github.com/$(whoami)/$PROJECT_NAME"
    echo -e "${BLUE}Using default: $MODULE_PATH${NC}"
fi

# Clone template
echo ""
echo -e "${BLUE}📦 Cloning template...${NC}"
git clone --depth 1 "$TEMPLATE_REPO" "$PROJECT_NAME"
cd "$PROJECT_NAME"

# Remove git history
echo -e "${BLUE}🧹 Cleaning up...${NC}"
rm -rf .git
rm -f create-go-htmx.sh

# Update module path
echo -e "${BLUE}🔧 Updating module path to $MODULE_PATH...${NC}"
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' "s|github.com/tonymmm1/go-htmx|$MODULE_PATH|g" go.mod
    find src -type f -name "*.go" -exec sed -i '' "s|github.com/tonymmm1/go-htmx|$MODULE_PATH|g" {} +
else
    # Linux
    sed -i "s|github.com/tonymmm1/go-htmx|$MODULE_PATH|g" go.mod
    find src -type f -name "*.go" -exec sed -i "s|github.com/tonymmm1/go-htmx|$MODULE_PATH|g" {} +
fi

# Initialize git
echo -e "${BLUE}📝 Initializing git repository...${NC}"
git init
git add .
git commit -m "Initial commit from go-htmx template"

# Run setup
echo ""
echo -e "${BLUE}🚀 Running setup...${NC}"
echo ""

# Make setup script executable
chmod +x setup.sh

# Run setup
bash setup.sh

# Success
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}🎉 Project '$PROJECT_NAME' created successfully!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo -e "  ${YELLOW}cd $PROJECT_NAME${NC}"
echo -e "  ${YELLOW}make dev${NC}"
echo ""
echo -e "${BLUE}Your app will be available at:${NC}"
echo -e "  ${GREEN}http://localhost:8080${NC}"
echo ""
echo -e "${BLUE}Happy coding! 🚀${NC}"
echo ""

