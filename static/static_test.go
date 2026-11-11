package static

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/andybalholm/brotli"
)

var (
	cssBody = strings.Repeat(".btn { color: red; }\n", 200)
	pngBody = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"css/app.css":  {Data: []byte(cssBody)},
		"css/.gitkeep": {Data: nil},
		"js/app.js":    {Data: []byte(strings.Repeat("console.log('hi');\n", 100))},
		"img/logo.png": {Data: pngBody},
		"tiny.txt":     {Data: []byte("a")},
		".env":         {Data: []byte("SECRET=1")},
		".git/config":  {Data: []byte("[core]")},
		"static.go":    {Data: []byte("package static")},
	}
}

func setup(t *testing.T, fsys fstest.MapFS, dev bool) {
	t.Helper()
	if err := configureFS(fsys, dev); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { current.Store(nil) })
}

func get(t *testing.T, method, target string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	return rec
}

func shortHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:hashLen]
}

func gunzip(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func unbrotli(t *testing.T, data []byte) string {
	t.Helper()
	out, err := io.ReadAll(brotli.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// cssETag is the ETag of css/app.css for a variant suffix ("", "-gz", "-br").
func cssETag(suffix string) string {
	return `"` + shortHash([]byte(cssBody)) + suffix + `"`
}

func TestPathIncludesContentHash(t *testing.T) {
	setup(t, testFS(), false)

	got := Path("css/app.css")
	if !regexp.MustCompile(`^/static/css/app\.css\?v=[0-9a-f]{12}$`).MatchString(got) {
		t.Fatalf("Path = %q, want hashed URL", got)
	}
	if want := "/static/css/app.css?v=" + shortHash([]byte(cssBody)); got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}

func TestPathUnknownAssetInProduction(t *testing.T) {
	setup(t, testFS(), false)

	if got := Path("css/missing.css"); got != "/static/css/missing.css" {
		t.Fatalf("Path = %q, want unversioned URL", got)
	}
}

func TestVersionedRequestIsImmutable(t *testing.T) {
	setup(t, testFS(), false)

	rec := get(t, http.MethodGet, Path("css/app.css"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != immutableCache {
		t.Fatalf("Cache-Control = %q, want %q", got, immutableCache)
	}
	if got := rec.Body.String(); got != cssBody {
		t.Fatal("unexpected body")
	}
}

func TestUnversionedRequestRevalidates(t *testing.T) {
	setup(t, testFS(), false)

	for _, target := range []string{"/static/css/app.css", "/static/css/app.css?v=000000000000"} {
		rec := get(t, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", target, rec.Code)
		}
		if got := rec.Header().Get("Cache-Control"); got != revalidateCache {
			t.Fatalf("%s: Cache-Control = %q, want %q", target, got, revalidateCache)
		}
		if got, want := rec.Header().Get("ETag"), `"`+shortHash([]byte(cssBody))+`"`; got != want {
			t.Fatalf("%s: ETag = %q, want %q", target, got, want)
		}
	}
}

func TestIfNoneMatchReturnsNotModified(t *testing.T) {
	setup(t, testFS(), false)

	variants := map[string]string{"": cssETag(""), "gzip": cssETag("-gz"), "br": cssETag("-br")}
	for encoding, etag := range variants {
		first := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", encoding)
		if got := first.Header().Get("ETag"); got != etag {
			t.Fatalf("encoding %q: ETag = %q, want %q", encoding, got, etag)
		}

		rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", encoding, "If-None-Match", etag)
		if rec.Code != http.StatusNotModified {
			t.Fatalf("encoding %q: status = %d, want 304", encoding, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("encoding %q: 304 has a body", encoding)
		}
		if got := rec.Header().Get("ETag"); got != etag {
			t.Fatalf("encoding %q: 304 ETag = %q, want %q", encoding, got, etag)
		}
	}

	// A cached variant does not match a request that negotiates another one.
	rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", "br, gzip", "If-None-Match", cssETag("-gz"))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("status = %d, Content-Encoding = %q, want 200 br", rec.Code, rec.Header().Get("Content-Encoding"))
	}
}

func TestGzipVariant(t *testing.T) {
	setup(t, testFS(), false)

	rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", "gzip, deflate")
	h := rec.Header()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := h.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := h.Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q", got)
	}
	if got := h.Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got, want := h.Get("Content-Length"), strconv.Itoa(rec.Body.Len()); got != want {
		t.Fatalf("Content-Length = %q, want %q", got, want)
	}
	if rec.Body.Len() >= len(cssBody) {
		t.Fatalf("gzip body (%d bytes) not smaller than original (%d)", rec.Body.Len(), len(cssBody))
	}
	if got := gunzip(t, rec.Body.Bytes()); got != cssBody {
		t.Fatal("gzip body does not decompress to the original")
	}
	if got, want := h.Get("ETag"), cssETag("-gz"); got != want {
		t.Fatalf("ETag = %q, want %q", got, want)
	}
}

func TestBrotliVariant(t *testing.T) {
	setup(t, testFS(), false)

	rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", "br")
	h := rec.Header()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := h.Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
	if got := h.Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q", got)
	}
	if got := h.Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got, want := h.Get("Content-Length"), strconv.Itoa(rec.Body.Len()); got != want {
		t.Fatalf("Content-Length = %q, want %q", got, want)
	}
	if got := h.Get("Cache-Control"); got != revalidateCache {
		t.Fatalf("Cache-Control = %q, want %q", got, revalidateCache)
	}
	if rec.Body.Len() >= len(cssBody) {
		t.Fatalf("br body (%d bytes) not smaller than original (%d)", rec.Body.Len(), len(cssBody))
	}
	if got := unbrotli(t, rec.Body.Bytes()); got != cssBody {
		t.Fatal("br body does not decompress to the original")
	}
	if got, want := h.Get("ETag"), cssETag("-br"); got != want {
		t.Fatalf("ETag = %q, want %q", got, want)
	}

	versioned := get(t, http.MethodGet, Path("css/app.css"), "Accept-Encoding", "br")
	if got := versioned.Header().Get("Cache-Control"); got != immutableCache {
		t.Fatalf("versioned Cache-Control = %q, want %q", got, immutableCache)
	}
	if got := versioned.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("versioned Content-Encoding = %q, want br", got)
	}
}

func TestEncodingPreference(t *testing.T) {
	setup(t, testFS(), false)

	cases := map[string]string{
		"gzip, deflate, br, zstd": "br",
		"br, gzip":                "br",
		"br;q=0.5, gzip;q=0.5":    "br",
		"gzip;q=1.0, br;q=0.9":    "gzip",
		"br;q=0, gzip":            "gzip",
		"gzip, br;q=0":            "gzip",
		"*":                       "br",
		"br;q=0, *":               "gzip",
		"gzip, *;q=0":             "gzip",
		"*;q=0":                   "",
		"br;q=0, gzip;q=0, *":     "",
		"identity":                "",
		"zstd, deflate":           "",
		"BR;Q=0.8, GZIP;Q=0.9":    "gzip",
	}
	for header, want := range cases {
		rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", header)
		if got := rec.Header().Get("Content-Encoding"); got != want {
			t.Errorf("%q: Content-Encoding = %q, want %q", header, got, want)
		}
	}
}

func TestIdentityVariant(t *testing.T) {
	setup(t, testFS(), false)

	cases := map[string][]string{
		"no Accept-Encoding":  nil,
		"gzip refused":        {"Accept-Encoding", "gzip;q=0, deflate"},
		"br and gzip refused": {"Accept-Encoding", "br;q=0, gzip;q=0, *"},
		"other codings only":  {"Accept-Encoding", "zstd, deflate"},
	}
	for name, headers := range cases {
		rec := get(t, http.MethodGet, "/static/css/app.css", headers...)
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("%s: Content-Encoding = %q, want identity", name, got)
		}
		if got := rec.Header().Get("Vary"); got != "Accept-Encoding" {
			t.Fatalf("%s: Vary = %q", name, got)
		}
		if got, want := rec.Header().Get("Content-Length"), strconv.Itoa(len(cssBody)); got != want {
			t.Fatalf("%s: Content-Length = %q, want %q", name, got, want)
		}
		if rec.Body.String() != cssBody {
			t.Fatalf("%s: unexpected body", name)
		}
		if got, want := rec.Header().Get("ETag"), cssETag(""); got != want {
			t.Fatalf("%s: ETag = %q, want %q", name, got, want)
		}
	}
}

func TestRangeIsServedFromIdentity(t *testing.T) {
	setup(t, testFS(), false)

	for _, encoding := range []string{"gzip", "br", "br, gzip"} {
		rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", encoding, "Range", "bytes=0-3")
		if rec.Code != http.StatusPartialContent {
			t.Fatalf("%q: status = %d, want 206", encoding, rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("%q: Content-Encoding = %q, want identity", encoding, got)
		}
		if got, want := rec.Header().Get("ETag"), cssETag(""); got != want {
			t.Fatalf("%q: ETag = %q, want %q", encoding, got, want)
		}
		if got := rec.Body.String(); got != cssBody[:4] {
			t.Fatalf("%q: body = %q", encoding, got)
		}
	}
}

func TestOnlySmallerCompressibleFilesAreCompressed(t *testing.T) {
	setup(t, testFS(), false)

	for _, target := range []string{"/static/tiny.txt", "/static/img/logo.png"} {
		rec := get(t, http.MethodGet, target, "Accept-Encoding", "br, gzip")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", target, rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("%s: Content-Encoding = %q, want identity", target, got)
		}
	}
	if got := get(t, http.MethodGet, "/static/img/logo.png").Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("png Content-Type = %q", got)
	}

}

// TestDroppedBrotliFallsBackToGzip covers a file whose Brotli variant was not
// smaller than the original but whose gzip variant was.
func TestDroppedBrotliFallsBackToGzip(t *testing.T) {
	setup(t, testFS(), false)
	load().files["css/app.css"].br = nil

	rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", "br, gzip")
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got, want := rec.Header().Get("ETag"), cssETag("-gz"); got != want {
		t.Fatalf("ETag = %q, want %q", got, want)
	}

	rec = get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", "br")
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("br only: Content-Encoding = %q, want identity", got)
	}
	if got := rec.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("br only: Vary = %q", got)
	}
}

func TestHead(t *testing.T) {
	setup(t, testFS(), false)

	for _, encoding := range []string{"", "gzip", "br"} {
		full := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", encoding)
		rec := get(t, http.MethodHead, "/static/css/app.css", "Accept-Encoding", encoding)
		if rec.Code != http.StatusOK {
			t.Fatalf("encoding %q: status = %d", encoding, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("encoding %q: HEAD response has a body", encoding)
		}
		if got, want := rec.Header().Get("Content-Length"), full.Header().Get("Content-Length"); got != want || got == "" {
			t.Fatalf("encoding %q: Content-Length = %q, want %q", encoding, got, want)
		}
		if got, want := rec.Header().Get("Content-Encoding"), full.Header().Get("Content-Encoding"); got != want {
			t.Fatalf("encoding %q: Content-Encoding = %q, want %q", encoding, got, want)
		}
		if got, want := rec.Header().Get("ETag"), full.Header().Get("ETag"); got != want {
			t.Fatalf("encoding %q: ETag = %q, want %q", encoding, got, want)
		}
	}
}

var notFoundTargets = []string{
	"/static/",
	"/static/css",
	"/static/css/",
	"/static/.env",
	"/static/.git/config",
	"/static/css/.gitkeep",
	"/static/static.go",
	"/static/css/missing.css",
}

func TestNotFound(t *testing.T) {
	setup(t, testFS(), false)

	for _, target := range notFoundTargets {
		if rec := get(t, http.MethodGet, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", target, rec.Code)
		}
	}
}

func writeFile(t *testing.T, name, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDevelopmentServesFromDisk(t *testing.T) {
	dir := t.TempDir()
	cssPath := filepath.Join(dir, "css", "app.css")
	writeFile(t, cssPath, "body{color:red}")
	writeFile(t, filepath.Join(dir, ".env"), "SECRET=1")
	writeFile(t, filepath.Join(dir, "css", ".gitkeep"), "")
	writeFile(t, filepath.Join(dir, "static.go"), "package static")
	writeFile(t, filepath.Join(dir, ".git", "config"), "[core]")
	Configure(true, dir)
	t.Cleanup(func() { current.Store(nil) })

	if got := Path("css/app.css"); got != "/static/css/app.css" {
		t.Fatalf("Path = %q, want unversioned URL", got)
	}

	rec := get(t, http.MethodGet, "/static/css/app.css")
	if rec.Code != http.StatusOK || rec.Body.String() != "body{color:red}" {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != devCache {
		t.Fatalf("Cache-Control = %q, want %q", got, devCache)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}

	writeFile(t, cssPath, "body{color:blue}")
	if rec := get(t, http.MethodGet, "/static/css/app.css"); rec.Body.String() != "body{color:blue}" {
		t.Fatalf("body after change = %q", rec.Body.String())
	}

	if rec := get(t, http.MethodHead, "/static/css/app.css"); rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("HEAD: status = %d, body length = %d", rec.Code, rec.Body.Len())
	}

	for _, target := range notFoundTargets {
		if rec := get(t, http.MethodGet, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", target, rec.Code)
		}
	}
}

func TestDevelopmentPathPanicsForMissingFile(t *testing.T) {
	setup(t, fstest.MapFS{"css/app.css": {Data: []byte("x")}}, true)

	defer func() {
		if recover() == nil {
			t.Fatal("Path did not panic for a missing file")
		}
	}()
	Path("css/missing.css")
}

func TestPathIsSafeForConcurrentUse(t *testing.T) {
	setup(t, testFS(), false)

	want := Path("css/app.css")
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				if got := Path("css/app.css"); got != want {
					t.Errorf("Path = %q, want %q", got, want)
				}
				Path("css/missing.css")
			}
		})
	}
	wg.Wait()
}

func TestEmbeddedAssets(t *testing.T) {
	Configure(false, "ignored")
	t.Cleanup(func() { current.Store(nil) })

	target := Path("js/htmx.min.js")
	if !strings.Contains(target, "?v=") {
		t.Fatalf("Path = %q, want hashed URL", target)
	}
	rec := get(t, http.MethodGet, target, "Accept-Encoding", "gzip")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "javascript") {
		t.Fatalf("Content-Type = %q", got)
	}
	data, err := embedded.ReadFile("js/htmx.min.js")
	if err != nil {
		t.Fatal(err)
	}
	rec = get(t, http.MethodGet, target, "Accept-Encoding", "gzip, deflate, br, zstd")
	if got := rec.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
	if unbrotli(t, rec.Body.Bytes()) != string(data) {
		t.Fatal("br body does not decompress to htmx.min.js")
	}
	if rec := get(t, http.MethodGet, "/static/static.go"); rec.Code != http.StatusNotFound {
		t.Fatalf("static.go: status = %d, want 404", rec.Code)
	}
	if rec := get(t, http.MethodGet, "/static/css/.gitkeep"); rec.Code != http.StatusNotFound {
		t.Fatalf(".gitkeep: status = %d, want 404", rec.Code)
	}
}

// TestVendoredHTMXIntegrity pins the vendored htmx build to its published
// subresource integrity hash. Update both together.
func TestVendoredHTMXIntegrity(t *testing.T) {
	const want = "sha384-Qp9rdC8H7EqOoX+nR0P3RPG2Noji+qdwJlGpfzwQJsfOpp3sGwIdSP9xO3+UlEhn"

	data, err := embedded.ReadFile("js/htmx.min.js")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha512.Sum384(data)
	if got := "sha384-" + base64.StdEncoding.EncodeToString(sum[:]); got != want {
		t.Fatalf("htmx.min.js integrity = %s, want %s", got, want)
	}
}

// A failed precondition (412) has no body, so the encoded representation's
// Content-Encoding and Content-Length must not be sent with it.
func TestPreconditionFailedDropsEncodingHeaders(t *testing.T) {
	setup(t, testFS(), false)

	for _, accept := range []string{"br", "gzip"} {
		rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", accept, "If-Match", `"other"`)
		if rec.Code != http.StatusPreconditionFailed {
			t.Fatalf("%s: status = %d, want 412", accept, rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("%s: Content-Encoding = %q, want none", accept, got)
		}
		if got := rec.Header().Get("Content-Length"); got != "" {
			t.Errorf("%s: Content-Length = %q, want none", accept, got)
		}
	}

	// A matching If-Match still gets the encoded body.
	rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", "br", "If-Match", cssETag("-br"))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("matching If-Match: status = %d, Content-Encoding = %q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
	if got, want := rec.Header().Get("Content-Length"), strconv.Itoa(rec.Body.Len()); got != want {
		t.Fatalf("Content-Length = %q, want %q", got, want)
	}
}

// When only a Brotli variant exists, a gzip-only client gets the identity
// bytes with the identity ETag. The response carries an ETag, so the gzip
// middleware leaves it alone rather than compressing it again.
func TestBrotliOnlyAssetServesIdentityToGzipClients(t *testing.T) {
	setup(t, testFS(), false)
	load().files["css/app.css"].gz = nil

	rec := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", "gzip")
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want identity", got)
	}
	if got, want := rec.Header().Get("ETag"), cssETag(""); got != want {
		t.Fatalf("ETag = %q, want %q", got, want)
	}
	if got := rec.Body.String(); got != cssBody {
		t.Fatal("body is not the identity bytes")
	}
	if got := get(t, http.MethodGet, "/static/css/app.css", "Accept-Encoding", "gzip, br").Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("br-capable client: Content-Encoding = %q, want br", got)
	}
}

// The embedded assets are compressed once per process, however many times
// Configure runs (each test server calls it).
func TestEmbeddedAssetsAreBuiltOnce(t *testing.T) {
	t.Cleanup(func() { current.Store(nil) })
	Configure(false, "ignored")
	first := current.Load()
	Configure(false, "ignored")
	if current.Load() != first {
		t.Fatal("Configure rebuilt the embedded assets")
	}
}

func TestCompressAllKeepsOnlySmallerVariants(t *testing.T) {
	files := map[string]*asset{
		"big.css":  {data: []byte(cssBody), ctype: "text/css; charset=utf-8"},
		"tiny.txt": {data: []byte("a"), ctype: "text/plain; charset=utf-8"},
		"logo.png": {data: pngBody, ctype: "image/png"},
	}
	if err := compressAll(files); err != nil {
		t.Fatal(err)
	}
	if f := files["big.css"]; f.gz == nil || f.br == nil {
		t.Error("big.css should have gzip and br variants")
	} else if unbrotli(t, f.br) != cssBody || gunzip(t, f.gz) != cssBody {
		t.Error("variants do not decompress to the original")
	}
	if f := files["tiny.txt"]; f.gz != nil || f.br != nil {
		t.Error("tiny.txt variants are not smaller and should be dropped")
	}
	if f := files["logo.png"]; f.gz != nil || f.br != nil {
		t.Error("png is not compressible and should have no variants")
	}
}
