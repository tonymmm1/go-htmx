#!/bin/bash

# Setup script for go-htmx template
# Makes setup as easy as create-nuxt-app

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color
AIR_VERSION="v1.67.4"
TEMPL_VERSION="v0.3.1020"

# Banner
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

# Check prerequisites
echo -e "${BLUE}Checking prerequisites...${NC}"

if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go is not installed. Please install Go 1.27 or later.${NC}"
    exit 1
fi
GO_VERSION=$(go env GOVERSION)
if [[ ! "$GO_VERSION" =~ ^go([0-9]+)\.([0-9]+) ]] ||
    (( BASH_REMATCH[1] < 1 || (BASH_REMATCH[1] == 1 && BASH_REMATCH[2] < 27) )); then
    echo -e "${RED}✗ Go 1.27 or later is required (found ${GO_VERSION}).${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Go ${GO_VERSION}${NC}"

if ! command -v node &> /dev/null; then
    echo -e "${RED}✗ Node.js is not installed. Please install Node.js 22 or later.${NC}"
    exit 1
fi
NODE_VERSION=$(node --version)
if [[ ! "$NODE_VERSION" =~ ^v([0-9]+)\. ]] || (( BASH_REMATCH[1] < 22 )); then
    echo -e "${RED}✗ Node.js 22 or later is required (found ${NODE_VERSION}).${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Node.js ${NODE_VERSION}${NC}"

if ! command -v npm &> /dev/null; then
    echo -e "${RED}✗ npm is not installed.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ npm $(npm --version)${NC}"

if ! command -v make &> /dev/null; then
    echo -e "${RED}✗ make is not installed.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ make${NC}"

echo ""

# Fix module paths
echo -e "${BLUE}Checking module paths...${NC}"
CURRENT_MODULE=$(grep '^module ' go.mod | awk '{print $2}')
TEMPLATE_MODULE="github.com/tonymmm1/go-htmx"

if [ "$CURRENT_MODULE" != "$TEMPLATE_MODULE" ]; then
    echo -e "${YELLOW}Current module: $CURRENT_MODULE${NC}"
    echo -e "${YELLOW}Updating import paths from $TEMPLATE_MODULE...${NC}"
    
    replace_module_path() {
        local file="$1"
        local temporary="${file}.module-update"
        sed "s|$TEMPLATE_MODULE|$CURRENT_MODULE|g" "$file" > "$temporary"
        mv "$temporary" "$file"
    }

    while IFS= read -r -d '' file; do
        replace_module_path "$file"
    done < <(find cmd internal -type f -name "*.go" -print0)

    while IFS= read -r -d '' file; do
        replace_module_path "$file"
    done < <(find templates -type f -name "*.templ" -print0)
    
    echo -e "${GREEN}✓ Import paths updated to $CURRENT_MODULE${NC}"
else
    echo -e "${GREEN}✓ Module paths are correct${NC}"
fi

echo ""

# Create .env if it doesn't exist
if [ ! -f .env ]; then
    echo -e "${BLUE}Creating .env file...${NC}"
    echo "PORT=8080" > .env
    echo -e "${GREEN}✓ .env file created${NC}"
else
    echo -e "${YELLOW}⚠ .env file already exists, skipping${NC}"
fi

# Install Go tools FIRST (needed for templ generate)
echo ""
echo -e "${BLUE}Installing Go tools (air, templ)...${NC}"
go install github.com/air-verse/air@"${AIR_VERSION}"
go install github.com/a-h/templ/cmd/templ@"${TEMPL_VERSION}"
echo -e "${GREEN}✓ Go tools installed${NC}"

# Generate templ files BEFORE go mod tidy
echo ""
echo -e "${BLUE}Generating Templ files...${NC}"
templ generate
echo -e "${GREEN}✓ Templ files generated${NC}"

# NOW run go mod tidy (after templ files exist)
if [ "$CURRENT_MODULE" != "$TEMPLATE_MODULE" ]; then
    echo ""
    echo -e "${BLUE}Running go mod tidy...${NC}"
    go mod tidy
    echo -e "${GREEN}✓ Dependencies updated${NC}"
fi

# Install Go dependencies
echo ""
echo -e "${BLUE}Installing Go dependencies...${NC}"
go mod download
echo -e "${GREEN}✓ Go dependencies installed${NC}"

# Install npm dependencies
echo ""
echo -e "${BLUE}Installing npm dependencies...${NC}"
npm install
echo -e "${GREEN}✓ npm dependencies installed${NC}"

# Create necessary directories
echo ""
echo -e "${BLUE}Creating necessary directories...${NC}"
mkdir -p static/css static/images/icons static/images/logos tmp
echo -e "${GREEN}✓ Directories created${NC}"

# Build initial CSS
echo ""
echo -e "${BLUE}Building initial CSS...${NC}"
npm run build:css
echo -e "${GREEN}✓ CSS built${NC}"

# Success message
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ Setup complete! Your project is ready.${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo ""
echo -e "${BLUE}To start the development server:${NC}"
echo -e "  ${YELLOW}make dev${NC}"
echo ""
echo -e "${BLUE}Other useful commands:${NC}"
echo -e "  ${YELLOW}make build${NC}       - Build for production"
echo -e "  ${YELLOW}make run${NC}         - Build and run"
echo -e "  ${YELLOW}make docker-up${NC}   - Run in Docker"
echo -e "  ${YELLOW}make help${NC}        - Show all commands"
echo ""
echo -e "${BLUE}Your app will be available at:${NC}"
echo -e "  ${GREEN}http://localhost:8080${NC}"
echo ""
