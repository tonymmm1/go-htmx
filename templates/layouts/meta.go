package layouts

import "context"

// SiteName is appended to every page title and used as the brand in the nav.
const SiteName = "Go-HTMX"

// Meta describes a page for the document <head> and navigation.
type Meta struct {
	// Title is the page name, rendered as "Title · Go-HTMX". Leave it empty on
	// the home page to use just the site name.
	Title string
	// Description is used for <meta name="description"> (search snippets and
	// link previews). Aim for one sentence under ~160 characters.
	Description string
	// Path is the page's URL path. The nav link with the same href is marked
	// aria-current="page".
	Path string
}

// FullTitle returns the text for the <title> element.
func (m Meta) FullTitle() string {
	if m.Title == "" {
		return SiteName
	}
	return m.Title + " · " + SiteName
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
