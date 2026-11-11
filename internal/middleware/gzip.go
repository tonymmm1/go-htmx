package middleware

import (
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/tonymmm1/go-htmx/internal/contentcoding"
)

// gzipWriters reuses compressors; each gzip.Writer holds several hundred KB
// of state that would otherwise be allocated per response.
var gzipWriters = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

// gzipResponses compresses text-like responses for clients that accept gzip.
// Responses that already have a Content-Encoding (such as precompressed static
// assets) or an ETag are passed through untouched: a handler that sets an ETag
// manages its own representations, and compressing on the fly would give two
// different bodies the same strong validator.
func gzipResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead || r.Header.Get("Range") != "" || contentcoding.Negotiate(r, "gzip") == "" {
			next.ServeHTTP(w, r)
			return
		}

		writer := &gzipResponseWriter{ResponseWriter: w}
		defer writer.close()
		next.ServeHTTP(writer, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter  *gzip.Writer
	wroteHeader bool
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	// 1xx responses (e.g. 103 Early Hints) may precede the final status.
	if status < http.StatusOK {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	if status != http.StatusNoContent && status != http.StatusNotModified && status != http.StatusPartialContent &&
		w.Header().Get("Content-Encoding") == "" && w.Header().Get("ETag") == "" &&
		compressibleContentType(w.Header().Get("Content-Type")) {
		w.Header().Del("Content-Length")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Set("Content-Encoding", "gzip")
		w.gzipWriter = gzipWriters.Get().(*gzip.Writer)
		w.gzipWriter.Reset(w.ResponseWriter)
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(data))
		}
		w.WriteHeader(http.StatusOK)
	}
	if w.gzipWriter != nil {
		return w.gzipWriter.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

// Flush sends buffered compressed data to the client, so streaming responses
// keep working through the compressor.
func (w *gzipResponseWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.gzipWriter != nil {
		_ = w.gzipWriter.Flush()
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// close finishes the gzip stream and returns the compressor to the pool.
func (w *gzipResponseWriter) close() {
	if w.gzipWriter == nil {
		return
	}
	// Close only fails when the client has gone away; nothing to recover.
	_ = w.gzipWriter.Close()
	w.gzipWriter.Reset(io.Discard)
	gzipWriters.Put(w.gzipWriter)
	w.gzipWriter = nil
}

func compressibleContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	if strings.HasPrefix(mediaType, "text/") {
		return mediaType != "text/event-stream"
	}
	switch mediaType {
	case "application/javascript", "application/json", "application/xml", "image/svg+xml":
		return true
	default:
		return false
	}
}
