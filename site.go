package main

import (
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// The entire site is compiled into the binary, so deploying is copying one file
// and there is no way for a running container to be missing an asset.
//
//go:embed index.html 404.html assets fragments
var embedded embed.FS

// file is one embedded asset, held in memory with a precomputed strong ETag.
// The site is roughly 130 KB in total, so buffering it costs nothing and lets
// the handler answer conditional requests without touching a filesystem.
type file struct {
	body     []byte
	etag     string
	mimeType string
}

var files = mustLoad()

func mustLoad() map[string]file {
	root, err := fs.Sub(embedded, ".")
	if err != nil {
		panic(err)
	}

	out := make(map[string]file)
	err = fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		b, err := fs.ReadFile(root, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[p] = file{
			body:     b,
			etag:     `"` + hex.EncodeToString(sum[:8]) + `"`,
			mimeType: contentType(p),
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	if len(out) == 0 {
		panic("portfolio: no files were embedded")
	}
	return out
}

// contentType resolves a media type from the extension. mime.TypeByExtension is
// not enough on its own: it can return "text/plain" for SVG, and a stylesheet or
// a script served as text/plain is blocked by a strict CSP.
func contentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json", ".webmanifest", ".map":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".pdf":
		return "application/pdf"
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// compressibleExt lists the extensions worth gzipping. Images and fonts are
// already compressed, so compressing them would burn CPU to save nothing.
var compressibleExt = map[string]bool{
	".html": true, ".htm": true, ".css": true, ".js": true, ".mjs": true,
	".json": true, ".map": true, ".xml": true, ".svg": true, ".txt": true,
	".webmanifest": true,
}

func newHandler(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// Liveness and readiness both mean "the process is up and the site is
	// embedded". There is no upstream dependency to check, which is the usual
	// reason to keep the two endpoints separate in the first place.
	probe := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		_, _ = io.WriteString(w, "ok\n")
	}
	mux.HandleFunc("GET /healthz", probe)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		_, _ = io.WriteString(w, strconv.Itoa(len(files))+" files embedded\n")
	})

	// Everything else is a static file, or the 404 page.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		serveFile(w, r, resolve(r.URL.Path))
	})

	var h http.Handler = mux
	h = securityHeaders(h)
	h = compress(h)
	return h
}

// resolve maps a request path onto an embedded file name. The path is cleaned
// first, so "..", encoded traversal and duplicate separators cannot reach
// outside the embedded tree.
func resolve(urlPath string) string {
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" || name == "." {
		return "index.html"
	}
	// A trailing slash means a directory. Nothing links to one, and directory
	// listings are not interesting, so treat it as not found.
	if strings.HasSuffix(urlPath, "/") {
		return ""
	}
	return name
}

// serveFile writes an embedded file, honouring conditional requests.
func serveFile(w http.ResponseWriter, r *http.Request, name string) {
	f, ok := files[name]
	if !ok {
		notFound(w, r)
		return
	}

	h := w.Header()
	h.Set("Content-Type", f.mimeType)
	h.Set("ETag", f.etag)
	h.Set("Cache-Control", cacheControl(name))
	h.Set("Vary", "Accept-Encoding")

	if match := r.Header.Get("If-None-Match"); match != "" && etagMatches(match, f.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	h.Set("Content-Length", strconv.Itoa(len(f.body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(f.body)
}

// cacheControl picks the caching policy per file. Nothing is cached
// unconditionally: asset filenames are stable rather than content-hashed, so an
// immutable max-age would keep serving a stale stylesheet after a deploy.
// Revalidating against a strong ETag costs one 304 and a few header bytes,
// which is the right trade for a site this size.
func cacheControl(name string) string {
	if strings.HasSuffix(name, ".html") {
		return "no-cache"
	}
	return "public, max-age=0, must-revalidate"
}

// etagMatches implements the If-None-Match comparison from RFC 9110, including
// weak comparison and the "*" wildcard.
func etagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == "*" {
			return true
		}
		if strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(etag, "W/") {
			return true
		}
	}
	return false
}

func notFound(w http.ResponseWriter, r *http.Request) {
	f, ok := files["404.html"]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h := w.Header()
	h.Set("Content-Type", f.mimeType)
	h.Set("Cache-Control", "no-cache")
	h.Set("Vary", "Accept-Encoding")
	w.WriteHeader(http.StatusNotFound)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(f.body)
}

// securityHeaders sets the small set of headers that matter for a static page.
// The CSP permits the inline theme bootstrap, which has to run before first
// paint to avoid a flash of the wrong theme, and nothing else.
func securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"form-action 'none'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"object-src 'none'"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// gzipResponseWriter pipes the body through a compressor and drops the
// uncompressed Content-Length, which the inner handler has already set by the
// time WriteHeader is called.
//
// The compressor is created lazily, on the first body write. A 204 or a 304 has
// no body, and flushing an unused gzip.Writer emits 18 bytes of empty-gzip
// framing downstream, which commits the response as 200 before the real status
// is ever written. Staying unopened until there is something to compress keeps
// those responses bodiless, which is the whole point of a 304.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz     *gzip.Writer
	status int
}

func (g *gzipResponseWriter) WriteHeader(status int) {
	if g.status != 0 {
		return
	}
	g.status = status
	// A bodiless status keeps its declared length, if it had one. Anything else
	// is about to have its body rewritten by the compressor, so the length the
	// inner handler computed no longer holds.
	if status != http.StatusNoContent && status != http.StatusNotModified {
		g.Header().Del("Content-Length")
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if g.status == 0 {
		g.WriteHeader(http.StatusOK)
	}
	if g.gz == nil {
		g.gz = gzip.NewWriter(g.ResponseWriter)
	}
	return g.gz.Write(b)
}

// finish flushes the compressor, if one was ever opened. It runs after the
// handler returns, because nothing past that point may write.
func (g *gzipResponseWriter) finish() {
	if g.gz != nil {
		_ = g.gz.Close()
	}
}

// compress gzips eligible responses when the client asks for it. Whether to
// compress is decided from the resolved file name, before the handler runs, so
// the response headers are known before anything is written and there is no
// need to buffer the body to find out what it turned out to be. Resolving first
// matters because the site root has no extension of its own: "/" is index.html.
func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet ||
			r.Header.Get("Range") != "" ||
			!strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") ||
			!compressibleExt[strings.ToLower(path.Ext(resolve(r.URL.Path)))] {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")

		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.finish()
		next.ServeHTTP(gw, r)
	})
}
