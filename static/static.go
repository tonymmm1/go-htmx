// Package static serves the files in this directory (CSS, JS, images).
//
// In production the asset directories listed in the //go:embed directive below
// are compiled into the binary, so a deployment only needs the executable. At
// startup every file is hashed (SHA-256, first 12 hex characters) and
// compressible files are compressed once with Brotli (quality 11) and gzip
// (level 9) and kept in memory; a variant is kept only if it is smaller than
// the original:
//
//   - Path returns "/static/<name>?v=<hash>". Requests carrying the current
//     hash are cached for a year as immutable; any other request gets a short
//     cache lifetime and revalidates with an ETag (If-None-Match -> 304).
//   - The encoding is negotiated from Accept-Encoding (q-values, q=0 and "*"
//     are honored): Brotli is preferred over gzip, and gzip over the identity
//     bytes. Each variant has its own ETag ("<hash>-br", "<hash>-gz",
//     "<hash>"). Range requests always get the identity bytes.
//   - Dotfiles, Go source files and directories are never served (404).
//
// In development (Configure(true, dir)) files are read from dir on every
// request with "Cache-Control: no-cache", so Tailwind's watch output shows up
// without rebuilding the binary. Path returns unversioned URLs and panics if
// the file does not exist, so typos fail loudly. In production an unknown
// name degrades to the unversioned URL and is logged once.
//
// Vendored files:
//
//   - js/htmx.min.js: htmx 2.0.10, copied from package/dist/htmx.min.js in
//     https://registry.npmjs.org/htmx.org/-/htmx.org-2.0.10.tgz
//     (sha384-H5SrcfygHmAuTDZphMHqBJLc3FhssKjG7w/CeCpFReSfwBWDTKpkzPP8c+cLsK+V).
//     To update, download the new version's tarball, replace the file, check
//     it against the SRI hash published by htmx, and update the version and
//     hash here and in static_test.go.
package static

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
)

// embedded holds the production assets. css/ is Tailwind build output and is
// gitignored except for css/.gitkeep, which keeps the directory present (the
// all: prefix lets it match) so the package compiles before CSS is built.
// Add new asset directories (img, fonts, ...) to this directive.
//
//go:embed all:css img js
var embedded embed.FS

const (
	hashLen         = 12
	immutableCache  = "public, max-age=31536000, immutable"
	revalidateCache = "public, max-age=300"
	devCache        = "no-cache"
)

type asset struct {
	data  []byte
	gz    []byte // nil unless gzip is smaller than data
	br    []byte // nil unless Brotli is smaller than data
	ctype string
	hash  string
}

type assets struct {
	dev  bool
	fsys fs.FS
	// files indexes every servable file by name (production only).
	files map[string]*asset
	// embedded is checked in development to warn about files that would 404
	// in production. nil disables the check.
	embedded fs.FS
	warned   sync.Map
}

var current atomic.Pointer[assets]

// Configure selects how assets are served. In development, files are read from
// dir on every request so Tailwind's watcher output shows up without a rebuild.
// In production dir is ignored and the embedded files are served.
// It should be called once at startup, before Handler or Path are used;
// otherwise the embedded production assets are used.
func Configure(dev bool, dir string) {
	var fsys fs.FS = embedded
	if dev {
		fsys = os.DirFS(dir)
	}
	start := time.Now()
	a, err := build(fsys, dev)
	if err != nil {
		panic(fmt.Sprintf("static: %v", err))
	}
	if dev {
		a.embedded = embedded
		log.Printf("static: serving assets from %s (development)", dir)
	} else {
		var raw, gz, br int
		for _, f := range a.files {
			raw += len(f.data)
			gz += len(f.body(negotiateEncoding("gzip", f.br != nil, f.gz != nil)))
			br += len(f.body(negotiateEncoding("br, gzip", f.br != nil, f.gz != nil)))
		}
		log.Printf("static: serving %d embedded assets (%d bytes, %d gzip, %d br; prepared in %s)",
			len(a.files), raw, gz, br, time.Since(start).Round(time.Millisecond))
	}
	current.Store(a)
}

// configureFS is the test seam: it serves fsys instead of the embedded files.
func configureFS(fsys fs.FS, dev bool) error {
	a, err := build(fsys, dev)
	if err != nil {
		return err
	}
	current.Store(a)
	return nil
}

func load() *assets {
	if a := current.Load(); a != nil {
		return a
	}
	a, err := build(embedded, false)
	if err != nil {
		panic(fmt.Sprintf("static: %v", err))
	}
	current.CompareAndSwap(nil, a)
	return current.Load()
}

func build(fsys fs.FS, dev bool) (*assets, error) {
	a := &assets{dev: dev, fsys: fsys}
	if dev {
		return a, nil
	}

	a.files = make(map[string]*asset)
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !servable(name) {
			return nil
		}

		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		f := &asset{
			data:  data,
			ctype: contentType(name, data),
			hash:  hex.EncodeToString(sum[:])[:hashLen],
		}
		if compressible(f.ctype) {
			if err := f.compress(); err != nil {
				return fmt.Errorf("compressing %s: %w", name, err)
			}
		}
		a.files[name] = f
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("indexing assets: %w", err)
	}
	return a, nil
}

// Handler serves assets. It is mounted at "GET /static/" and expects the
// full request path (it strips the /static/ prefix itself).
func Handler() http.Handler {
	return http.StripPrefix("/static/", http.HandlerFunc(serve))
}

func serve(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path
	if !fs.ValidPath(name) || !servable(name) {
		http.NotFound(w, r)
		return
	}

	a := load()
	if a.dev {
		a.serveDisk(w, r, name)
		return
	}

	f, ok := a.files[name]
	if !ok {
		http.NotFound(w, r)
		return
	}

	h := w.Header()
	h.Set("Content-Type", f.ctype)
	if r.URL.Query().Get("v") == f.hash {
		h.Set("Cache-Control", immutableCache)
	} else {
		h.Set("Cache-Control", revalidateCache)
	}

	coding := ""
	if f.br != nil || f.gz != nil {
		h.Add("Vary", "Accept-Encoding")
		// Ranges are only served from the identity bytes.
		if r.Header.Get("Range") == "" {
			accept := strings.Join(r.Header.Values("Accept-Encoding"), ",")
			coding = negotiateEncoding(accept, f.br != nil, f.gz != nil)
		}
	}
	body, etag := f.body(coding), f.hash
	switch coding {
	case "br":
		etag += "-br"
	case "gzip":
		etag += "-gz"
	}
	h.Set("ETag", `"`+etag+`"`)
	if coding != "" {
		h.Set("Content-Encoding", coding)
		// ServeContent omits Content-Length when Content-Encoding is set.
		h.Set("Content-Length", strconv.Itoa(len(body)))
	}

	// ServeContent handles HEAD, If-None-Match, Range and Content-Length.
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
}

func (a *assets) serveDisk(w http.ResponseWriter, r *http.Request, name string) {
	file, err := a.fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	content, ok := file.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		content = bytes.NewReader(data)
	}

	w.Header().Set("Cache-Control", devCache)
	http.ServeContent(w, r, name, info.ModTime(), content)
}

// Path returns the URL for an asset relative to this directory, for example
// Path("css/styles.css"). Production URLs include a content hash so they can
// be cached forever.
func Path(name string) string {
	a := load()
	url := "/static/" + name

	if a.dev {
		if !fs.ValidPath(name) || !servable(name) {
			panic(fmt.Sprintf("static.Path(%q): not a servable asset name", name))
		}
		if _, err := fs.Stat(a.fsys, name); err != nil {
			panic(fmt.Sprintf("static.Path(%q): %v (has the asset been built?)", name, err))
		}
		if a.embedded != nil {
			if _, err := fs.Stat(a.embedded, name); err != nil {
				a.warnOnce(name, "static.Path(%q): file is not embedded and will 404 in production; rebuild the binary or add its directory to the //go:embed directive in static/static.go", name)
			}
		}
		return url
	}

	if f, ok := a.files[name]; ok {
		return url + "?v=" + f.hash
	}
	a.warnOnce(name, "static.Path(%q): no such embedded asset; using an unversioned URL", name)
	return url
}

func (a *assets) warnOnce(name, format string, args ...any) {
	if _, loaded := a.warned.LoadOrStore(name, struct{}{}); !loaded {
		log.Printf(format, args...)
	}
}

// body returns the bytes to send for a coding chosen by negotiateEncoding:
// "br", "gzip" or "" (identity).
func (f *asset) body(coding string) []byte {
	switch {
	case coding == "br" && f.br != nil:
		return f.br
	case coding == "gzip" && f.gz != nil:
		return f.gz
	}
	return f.data
}

// compress stores the gzip and Brotli variants of f.data, each only if it is
// smaller than the original.
func (f *asset) compress() error {
	gz, err := gzipBytes(f.data)
	if err != nil {
		return err
	}
	if len(gz) < len(f.data) {
		f.gz = gz
	}
	br, err := brotliBytes(f.data)
	if err != nil {
		return err
	}
	if len(br) < len(f.data) {
		f.br = br
	}
	return nil
}

// servable rejects dotfiles (at any depth) and Go source files.
func servable(name string) bool {
	if strings.HasSuffix(name, ".go") {
		return false
	}
	for part := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

func contentType(name string, data []byte) string {
	if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
		return ctype
	}
	return http.DetectContentType(data)
}

func compressible(ctype string) bool {
	mediaType, _, _ := strings.Cut(ctype, ";")
	mediaType = strings.TrimSpace(strings.ToLower(mediaType))
	if strings.HasPrefix(mediaType, "text/") ||
		strings.HasSuffix(mediaType, "+json") ||
		strings.HasSuffix(mediaType, "+xml") {
		return true
	}
	switch mediaType {
	case "application/javascript", "application/json", "application/xml",
		"application/wasm", "image/x-icon", "image/vnd.microsoft.icon",
		"font/ttf", "font/otf":
		return true
	}
	return false
}

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// brotliBytes compresses data at the highest quality. The window is the
// smallest that covers the whole input, which keeps the decoder's memory use
// small for small files without costing compression.
func brotliBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	bw := brotli.NewWriterOptions(&buf, brotli.WriterOptions{
		Quality: brotli.BestCompression,
		LGWin:   brotliWindow(len(data)),
	})
	if _, err := bw.Write(data); err != nil {
		return nil, err
	}
	if err := bw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// brotliWindow returns the base-2 log of the smallest Brotli window (10 to 24)
// that can reference all n bytes. Brotli windows hold 2^lgwin-16 bytes.
func brotliWindow(n int) int {
	lgwin := 10
	for lgwin < 24 && 1<<lgwin-16 < n {
		lgwin++
	}
	return lgwin
}

// negotiateEncoding picks the content coding to serve from an Accept-Encoding
// header, given which precompressed variants exist: "br", "gzip" or "" for the
// identity bytes. A coding is acceptable if its q-value (or, if it is not
// listed, the q-value of "*") is above zero; the highest q-value wins and ties
// prefer br over gzip. Codings missing from the header are not acceptable,
// so an empty header gets the identity bytes. The identity bytes are always
// the fallback; an "identity" entry in the header is ignored.
func negotiateEncoding(header string, haveBr, haveGzip bool) string {
	brQ, gzipQ, wildcardQ := -1.0, -1.0, -1.0
	for part := range strings.SplitSeq(header, ",") {
		coding, params, _ := strings.Cut(part, ";")
		q := 1.0
		for param := range strings.SplitSeq(params, ";") {
			name, value, ok := strings.Cut(param, "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(name), "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil {
				parsed = 0
			}
			q = parsed
		}
		switch strings.ToLower(strings.TrimSpace(coding)) {
		case "br":
			brQ = q
		case "gzip", "x-gzip":
			gzipQ = q
		case "*":
			wildcardQ = q
		}
	}
	if brQ < 0 {
		brQ = wildcardQ
	}
	if gzipQ < 0 {
		gzipQ = wildcardQ
	}
	if !haveBr {
		brQ = 0
	}
	if !haveGzip {
		gzipQ = 0
	}

	switch {
	case brQ > 0 && brQ >= gzipQ:
		return "br"
	case gzipQ > 0:
		return "gzip"
	}
	return ""
}
