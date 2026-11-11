package layouts

import (
	"cmp"
	"context"

	"github.com/tonymmm1/go-htmx/internal/site"
	"github.com/tonymmm1/go-htmx/static"
)

// Meta describes a page for the document <head> and navigation.
type Meta struct {
	// Title is the page name, rendered as "Title · <site.Name>". Leave it empty
	// on the home page to use just the site name.
	Title string
	// Description is used for <meta name="description"> (search snippets and
	// link previews). Aim for one sentence under ~160 characters.
	Description string
	// Path is the page's URL path. The nav link with the same href is marked
	// aria-current="page", and it forms the canonical URL. Leave it empty on
	// error pages.
	Path string
	// Image is the link-preview image as an asset name under static/, for
	// example "img/pricing.png". Empty means site.Image.
	Image string
}

// FullTitle returns the text for the <title> element.
func (m Meta) FullTitle() string {
	if m.Title == "" {
		return site.Name
	}
	return m.Title + " · " + site.Name
}

// URL returns the page's canonical absolute URL, or "" if site.URL or Path
// is not set.
func (m Meta) URL() string {
	if site.URL == "" || m.Path == "" {
		return ""
	}
	return site.URL + m.Path
}

// ImageURL returns the absolute URL of the link-preview image, or "" if
// site.URL is not set or there is no image. Crawlers need absolute URLs.
func (m Meta) ImageURL() string {
	image := cmp.Or(m.Image, site.Image)
	if site.URL == "" || image == "" {
		return ""
	}
	return site.URL + static.Path(image)
}

type contentOnlyKey struct{}

// WithContentOnly returns a context in which Layout renders only its children,
// without the document shell. pages.RenderPage uses it for htmx requests that
// swap a page into part of the current document.
func WithContentOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, contentOnlyKey{}, true)
}

func contentOnly(ctx context.Context) bool {
	only, _ := ctx.Value(contentOnlyKey{}).(bool)
	return only
}
