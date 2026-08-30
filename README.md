# Go-HTMX Starter Template

[![CI](https://github.com/tonymmm1/go-htmx/actions/workflows/ci.yml/badge.svg)](https://github.com/tonymmm1/go-htmx/actions/workflows/ci.yml)

**The easiest way to start a Go + HTMX web application.** Just like `create-nuxt-app` or `create-react-app`, but for Go!

A production-ready starter template for building modern web applications with Go, HTMX, Templ, and Tailwind CSS. This template provides a solid foundation with hot-reloading, type-safe templating, and beautiful UI components out of the box.

### Why This Template?

| Feature | This Template | From Scratch |
|---------|--------------|--------------|
| **Setup Time** | 2 minutes | 2+ hours |
| **Dependencies** | ✅ Auto-installed | ❌ Manual setup |
| **Hot Reload** | ✅ Included | ❌ Configure yourself |
| **Page Generation** | ✅ `make new-page` | ❌ Manual creation |
| **Component Generation** | ✅ `make new-component` | ❌ Manual creation |
| **Production Ready** | ✅ Docker included | ❌ DIY deployment |
| **Type-Safe Templates** | ✅ Templ | ❌ html/template |

📚 **Documentation:** [Quick Start](QUICKSTART.md) | [Framework Comparison](COMPARISON.md)

## Tech Stack

- **[Go](https://go.dev/)** 1.27 - Fast, reliable backend language
- **[Go net/http](https://pkg.go.dev/net/http)** - Standard library HTTP server and routing
- **[HTMX](https://htmx.org/)** 2.0.10 - Modern interactivity without JavaScript frameworks
- **[Templ](https://templ.guide/)** 0.3.1020 - Type-safe Go templating language
- **[Tailwind CSS](https://tailwindcss.com/)** 4.3.3 - Utility-first CSS framework
- **[Tailwind Typography](https://github.com/tailwindlabs/tailwindcss-typography)** 0.5.20 - Rich text styling
- **[DaisyUI](https://daisyui.com/)** 5.7.22 - Beautiful component library for Tailwind
- **[Air](https://github.com/air-verse/air)** 1.67.4 - Live reload for Go apps

## Features

✅ Hot-reloading for Go, Templ, and CSS files  
✅ Type-safe HTML templating with Templ  
✅ Standard-library middleware (rate limiting, gzip, logging, recovery, security headers)<br>
✅ Docker support for production deployment  
✅ Beautiful default UI with DaisyUI components  
✅ Working HTMX examples with fragment responses<br>
✅ HTTP/2 support  
✅ Production-ready project structure  
✅ GitHub Actions CI for tests, audits, and production builds

## Quick Start

### Prerequisites

- Go 1.27 or later
- Node.js 22 or later
- Make

### Three Commands to Get Started

```bash
# 1. Clone this template (or use it as a GitHub template)
git clone https://github.com/tonymmm1/go-htmx.git my-project
cd my-project

# 2. Run setup (installs everything automatically)
make setup
# Or: bash setup.sh

# 3. Start development server
make dev
```

That's it! Visit `http://localhost:8080` 🎉

Open `http://localhost:8080/examples` to try the included HTMX interactions.

The setup command will:
- ✅ Check prerequisites (Go, Node.js, npm)
- ✅ Install Go dependencies
- ✅ Install Go tools (Air, Templ)
- ✅ Install npm dependencies (Tailwind, DaisyUI, Concurrently)
- ✅ Generate Templ files
- ✅ Build initial CSS
- ✅ Create `.env` file
- ✅ Set up directory structure

### Starting Fresh Project from Template

If you want to use this as a template for a new project:

```bash
# Clone and setup
git clone https://github.com/tonymmm1/go-htmx.git my-awesome-project
cd my-awesome-project

# Update module name in go.mod to your own
# e.g., change "github.com/tonymmm1/go-htmx" to "github.com/yourname/my-awesome-project"

# Run setup (automatically fixes all import paths!)
make setup

# Start coding!
make dev
```

**Note:** The `setup` script automatically detects your module path from `go.mod` and updates all imports in your code. No manual find/replace needed!

## Project Structure

```
.
├── src/
│   ├── cmd/
│   │   └── main.go           # Application entry point
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── middleware/
│   │   └── middleware.go     # HTTP middleware stack
│   ├── pages/
│   │   ├── pages.go          # Page and HTMX fragment handlers
│   │   └── pages_test.go     # Handler examples and tests
│   ├── server/
│   │   └── server.go         # HTTP server setup
│   └── styles/
│       └── input.css         # Tailwind CSS entry point
├── templates/
│   ├── layouts/
│   │   └── layout.templ      # Base layout template
│   ├── pages/
│   │   ├── index.templ       # Home page
│   │   ├── about.templ       # About page
│   │   └── examples.templ    # Working HTMX demo page
│   └── components/
│       └── examples.templ    # HTMX response fragments
├── static/
│   ├── css/                  # Generated CSS (auto-created)
│   └── images/               # Static assets
├── scripts/
│   ├── new-page.sh           # Page generator
│   └── new-component.sh      # Component generator
├── Makefile                  # Build automation
├── Dockerfile                # Production container
├── docker-compose.yml        # Docker Compose setup
├── .air.toml                 # Air configuration
└── go.mod                    # Go dependencies
```

## Available Commands

### Setup & Development
```bash
make setup         # Complete project setup (run this first!)
make dev           # Start development server with hot reload
make build         # Build production binary
make run           # Build and run the server
```

### Generators (like Nuxt!)
```bash
make new-page contact        # Generate a new page
make new-component card      # Generate a new component
```

### Docker
```bash
make docker-build   # Build Docker image
make docker-up      # Run Docker container (standalone)
make docker-down    # Stop Docker container
make compose-up     # Start with docker-compose (production)
make compose-dev    # Start with docker-compose (dev with hot reload)
make compose-down   # Stop docker-compose services
```

### Maintenance
```bash
make test          # Run tests
make clean         # Clean build artifacts
make clean-all     # Clean everything including dependencies
make help          # Show all available commands
```

## Development Workflow

### Adding New Pages (Automatic!)

Use the page generator (like Nuxt's page generation):

```bash
make new-page contact
```

This creates `templates/pages/contact.templ`, adds its handler, and registers
`GET /contact`. Hyphenated names such as `contact-us` become the valid Templ
component `ContactUs`, and generated imports always use the module from your
project's `go.mod`.

### Creating Reusable Components

```bash
make new-component card
```

Use components in your templates:

```templ
package pagetemplates

import "your/module/templates/layouts"
import "your/module/templates/components"

templ MyPage() {
    @layouts.Layout() {
        <div>
            @components.Card("My Card Title")
        </div>
    }
}
```

### Learning from the HTMX Examples

Visit `/examples` for three small, working patterns:

- `hx-get` replaces a target with a server-rendered time fragment.
- A debounced search sends its input value and swaps the result list.
- `hx-post` submits a counter form and replaces the complete component.

The browser markup lives in `templates/pages/examples.templ`, reusable response
fragments live in `templates/components/examples.templ`, and the Go handlers are
in `src/pages/pages.go`.

### Customizing Middleware

Edit `src/middleware/middleware.go` to modify:
- Rate limiting
- Response compression
- Logging and panic recovery
- Security headers or custom middleware

The default limiter keys requests by the direct TCP peer and intentionally
ignores forwarded-IP headers. When deploying behind a trusted proxy, configure
rate limiting at that proxy or add an explicit trusted-proxy policy first. HTMX
requests are same-origin, so CORS middleware is only needed if you later expose
an API to other origins.

### Environment Variables

Create a `.env` file in the project root:

```bash
PORT=8080
# Add other environment variables here
```

## Production Deployment

### Build Binary

```bash
make build
./bin/server
```

### Docker (Standalone)

```bash
# Build and run with Docker
make docker-up

# Or manually:
docker build -t my-app .
docker run -p 8080:8080 my-app
```

### Docker Compose (Recommended)

```bash
# Production
make compose-up

# Development with hot reload
make compose-dev

# Stop
make compose-down
```

The Docker image is optimized:
- ✅ Multi-stage build (~30MB final image)
- ✅ Non-root user for security
- ✅ Health checks included
- ✅ Only contains necessary files

## Customization Guide

### Change Theme

Edit the DaisyUI plugin block in `src/styles/input.css` to change themes:

```css
@plugin "daisyui" {
  themes: light --default, dark --prefersdark, cupcake, cyberpunk;
}
```

### Add Custom CSS

Add styles to `src/styles/input.css`:

```css
@import "tailwindcss" source(none);

@source "../../templates";
@source "../";

@plugin "@tailwindcss/typography";

@plugin "daisyui" {
  themes: light --default, dark --prefersdark, cupcake;
}

/* Your custom styles */
.my-custom-class {
  /* ... */
}
```

### Configure the Server

Modify `src/config/config.go` to add more configuration options.

## Troubleshooting

### Port Already in Use

```bash
# Change port in .env
PORT=3000
```

### Hot Reload Not Working

```bash
# Reinstall tools
make clean
make tools
make dev
```

### Templ Files Not Generating

```bash
# Manually generate
templ generate

# Or reinstall templ
go install github.com/a-h/templ/cmd/templ@v0.3.1020
```

## Contributing

This is a template repository. Feel free to fork and customize for your needs!

## License

MIT License - feel free to use this template for any project.

## Resources

- [Go Documentation](https://go.dev/doc/)
- [HTMX Documentation](https://htmx.org/docs/)
- [Templ Documentation](https://templ.guide/)
- [Tailwind CSS Documentation](https://tailwindcss.com/docs)
- [DaisyUI Components](https://daisyui.com/components/)
- [Go net/http Documentation](https://pkg.go.dev/net/http)
