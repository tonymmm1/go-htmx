# Quick Start Guide

## For New Projects

### Option 1: Using the Scaffolder (Recommended)

```bash
# Download and run the scaffolder
curl -sSL https://raw.githubusercontent.com/tonymmm1/go-htmx/master/create-go-htmx.sh | bash -s my-project

# Or download first, then run
wget https://raw.githubusercontent.com/tonymmm1/go-htmx/master/create-go-htmx.sh
bash create-go-htmx.sh my-project
```

This will:
1. Clone the template
2. Update module paths
3. Install all dependencies
4. Set up the project structure
5. Initialize git repository

### Option 2: Manual Clone

```bash
# Clone the repository
git clone https://github.com/tonymmm1/go-htmx.git my-project
cd my-project

# Update go.mod with your module path
# Change "github.com/tonymmm1/go-htmx" to your own

# Update import paths in all .go files
find src -type f -name "*.go" -exec sed -i 's|github.com/tonymmm1/go-htmx|github.com/yourname/my-project|g' {} +

# Run setup
make setup

# Start development
make dev
```

## For This Existing Project

If you've already cloned this repository:

```bash
# Just run setup
make setup

# Start development server
make dev
```

Visit http://localhost:8080

## Your First Changes

### Create a New Page

```bash
make new-page pricing
```

This creates `src/pages/pricing.templ` and adds the handler. Then add the route:

```go
// In src/pages/pages.go
func RegisterPageRoutes(h *Handler, r chi.Router) {
    r.Get("/", h.HandleIndex)
    r.Get("/about", h.HandleAbout)
    r.Get("/pricing", h.HandlePricing) // Add this
}
```

### Create a Component

```bash
make new-component hero
```

Use it in your pages:

```templ
@components.Hero("Welcome to My App")
```

### Customize the Theme

Edit `tailwind.config.js`:

```javascript
daisyui: {
  themes: ["light", "dark", "cyberpunk"], // Choose your themes
}
```

Change the theme in `src/pages/layout.templ`:

```html
<html lang="en" data-theme="cyberpunk">
```

## Common Commands

```bash
make dev          # Start dev server (hot reload)
make build        # Build for production
make docker-up    # Run in Docker
make help         # Show all commands
```

## Next Steps

1. **Read the full README.md** for detailed documentation
2. **Check src/pages/** to see example pages
3. **Check src/middleware/** to configure CORS, rate limiting, etc.
4. **Add your own routes** in `src/pages/pages.go`
5. **Deploy** - the built binary is self-contained!

## Troubleshooting

### "command not found: templ"

```bash
make tools
```

### Port 8080 already in use

Edit `.env`:
```
PORT=3000
```

### Dependencies not installed

```bash
make deps
```

## Getting Help

- Check the [README.md](README.md) for full documentation
- Look at example pages in `src/pages/`
- Review the middleware setup in `src/middleware/`

## Production Deployment

```bash
# Build binary
make build

# Run it
./bin/server

# Or use Docker
make docker-up
```

The binary is self-contained and includes all templates!

