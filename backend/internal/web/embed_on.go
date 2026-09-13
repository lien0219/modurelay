//go:build embed

package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	htmlpkg "html"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

const (
	// NonceHTMLPlaceholder is the placeholder for nonce in HTML script tags
	NonceHTMLPlaceholder = "__CSP_NONCE_VALUE__"
)

//go:embed all:dist
var frontendFS embed.FS

// PublicSettingsProvider is an interface to fetch public settings
type PublicSettingsProvider interface {
	GetPublicSettingsForInjection(ctx context.Context) (any, error)
}

// FrontendServer serves the embedded frontend with settings injection
type FrontendServer struct {
	distFS                   fs.FS
	fileServer               http.Handler
	infiniteCanvasFS         fs.FS
	infiniteCanvasFileServer http.Handler
	baseHTML                 []byte
	infiniteCanvasHTML       []byte
	cache                    *HTMLCache
	settings                 PublicSettingsProvider
	overrideDir              string // local file override directory
}

// NewFrontendServer creates a new frontend server with settings injection
func NewFrontendServer(settingsProvider PublicSettingsProvider) (*FrontendServer, error) {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		return nil, err
	}

	// Read base HTML once
	file, err := distFS.Open("index.html")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	baseHTML, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	cache := NewHTMLCache()
	cache.SetBaseHTML(baseHTML)
	infiniteCanvasHTML, _ := fs.ReadFile(distFS, "infinite-canvas/index.html")
	infiniteCanvasFS, _ := fs.Sub(distFS, "infinite-canvas")
	var infiniteCanvasFileServer http.Handler
	if infiniteCanvasFS != nil {
		infiniteCanvasFileServer = http.FileServer(http.FS(infiniteCanvasFS))
	}

	return &FrontendServer{
		distFS:                   distFS,
		fileServer:               http.FileServer(http.FS(distFS)),
		infiniteCanvasFS:         infiniteCanvasFS,
		infiniteCanvasFileServer: infiniteCanvasFileServer,
		baseHTML:                 baseHTML,
		infiniteCanvasHTML:       infiniteCanvasHTML,
		cache:                    cache,
		settings:                 settingsProvider,
		overrideDir:              filepath.Join("data", "public"),
	}, nil
}

// InvalidateCache invalidates the HTML cache (call when settings change)
func (s *FrontendServer) InvalidateCache() {
	if s != nil && s.cache != nil {
		s.cache.Invalidate()
	}
}

// Middleware returns the Gin middleware handler
func (s *FrontendServer) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip API routes
		if shouldBypassEmbeddedFrontend(path) {
			c.Next()
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// For index.html or SPA routes, serve with injected settings. Static
		// namespaces must fail closed so a missing vendor asset is not mistaken
		// for the application shell.
		if cleanPath == "index.html" {
			s.serveIndexHTML(c)
			return
		}
		if isInfiniteCanvasIndexPath(cleanPath) {
			s.serveInfiniteCanvasIndexHTML(c)
			return
		}
		if isInfiniteCanvasPath(cleanPath) {
			// Only the known React asset namespaces are static. All other
			// paths belong to the React router and must receive its shell so
			// browser refreshes/deep links keep working. Missing files inside
			// an asset namespace fail closed with 404 instead of returning HTML.
			if isInfiniteCanvasStaticPath(cleanPath) {
				relativePath := strings.TrimPrefix(strings.TrimPrefix(cleanPath, "infinite-canvas/"), "/")
				s.serveInfiniteCanvasStatic(c, relativePath)
				return
			}
			s.serveInfiniteCanvasIndexHTML(c)
			return
		}
		if !s.fileExists(cleanPath) {
			if isEmbeddedStaticNamespacePath(cleanPath) || isInfiniteCanvasStaticPath(cleanPath) {
				c.Status(http.StatusNotFound)
				c.Abort()
				return
			}
			s.serveIndexHTML(c)
			return
		}

		// Try local override first
		if s.tryServeOverride(c, cleanPath) {
			return
		}

		// Serve static files normally (hashed assets get long-lived cache headers)
		applyStaticAssetCacheHeaders(c.Writer.Header(), cleanPath)
		s.fileServer.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

func (s *FrontendServer) serveInfiniteCanvasIndexHTML(c *gin.Context) {
	if len(s.infiniteCanvasHTML) == 0 {
		c.Status(http.StatusNotFound)
		c.Abort()
		return
	}
	nonce := middleware.GetNonceFromContext(c)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", replaceNoncePlaceholder(s.infiniteCanvasHTML, nonce))
	c.Abort()
}

func (s *FrontendServer) serveInfiniteCanvasStatic(c *gin.Context, cleanPath string) {
	if s.infiniteCanvasFileServer == nil || cleanPath == "" || !s.fileExists("infinite-canvas/"+cleanPath) {
		c.Status(http.StatusNotFound)
		c.Abort()
		return
	}
	applyStaticAssetCacheHeaders(c.Writer.Header(), "infinite-canvas/"+cleanPath)
	originalPath := c.Request.URL.Path
	c.Request.URL.Path = "/" + cleanPath
	s.infiniteCanvasFileServer.ServeHTTP(c.Writer, c.Request)
	c.Request.URL.Path = originalPath
	c.Abort()
}

func (s *FrontendServer) fileExists(path string) bool {
	if s == nil || s.distFS == nil {
		return false
	}
	info, err := fs.Stat(s.distFS, path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// tryServeOverride checks if a local override file exists and serves it.
// Files in overrideDir take precedence over embedded files.
func (s *FrontendServer) tryServeOverride(c *gin.Context, cleanPath string) bool {
	if s.overrideDir == "" {
		return false
	}
	filePath := filepath.Join(s.overrideDir, filepath.Clean("/"+cleanPath))
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}
	c.File(filePath)
	c.Abort()
	return true
}

func (s *FrontendServer) serveIndexHTML(c *gin.Context) {
	// Get nonce from context (generated by SecurityHeaders middleware)
	nonce := middleware.GetNonceFromContext(c)
	// The final HTML contains a per-response CSP nonce. It must never be
	// browser-revalidated with a 304 because the cached body would retain the
	// previous nonce while the new response carries a different CSP header.
	c.Header("Cache-Control", "no-store")

	// Check cache first
	cached := s.cache.Get()
	if cached != nil {
		// Replace nonce placeholder with actual nonce before serving
		content := replaceNoncePlaceholder(cached.Content, nonce)

		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
		c.Abort()
		return
	}

	// Cache miss - fetch settings and render
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	settings, err := s.settings.GetPublicSettingsForInjection(ctx)
	if err != nil {
		// Fallback: serve without injection
		c.Data(http.StatusOK, "text/html; charset=utf-8", s.baseHTML)
		c.Abort()
		return
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		// Fallback: serve without injection
		c.Data(http.StatusOK, "text/html; charset=utf-8", s.baseHTML)
		c.Abort()
		return
	}

	rendered := s.injectSettings(settingsJSON)
	s.cache.Set(rendered, settingsJSON)

	// Replace nonce placeholder with actual nonce before serving
	content := replaceNoncePlaceholder(rendered, nonce)

	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	c.Abort()
}

func (s *FrontendServer) injectSettings(settingsJSON []byte) []byte {
	// Create the script tag to inject with nonce placeholder
	// The placeholder will be replaced with actual nonce at request time
	script := []byte(`<script nonce="` + NonceHTMLPlaceholder + `">window.__APP_CONFIG__=` + string(settingsJSON) + `;</script>`)

	// Inject before </head>
	headClose := []byte("</head>")
	result := bytes.Replace(s.baseHTML, headClose, append(script, headClose...), 1)

	// Apply custom branding before the browser paints the static defaults.
	result = injectSiteTitle(result, settingsJSON)
	result = injectSiteFavicon(result, settingsJSON)

	return result
}

// injectSiteFavicon replaces the static favicon with a configured, browser-safe image URL.
func injectSiteFavicon(html, settingsJSON []byte) []byte {
	var cfg struct {
		SiteLogo string `json:"site_logo"`
	}
	if err := json.Unmarshal(settingsJSON, &cfg); err != nil {
		return html
	}

	logoURL := safeImageURL(cfg.SiteLogo)
	if logoURL == "" {
		return html
	}

	linkStart := bytes.Index(html, []byte(`<link rel="icon"`))
	if linkStart == -1 {
		return html
	}
	linkEndOffset := bytes.IndexByte(html[linkStart:], '>')
	if linkEndOffset == -1 {
		return html
	}
	linkEnd := linkStart + linkEndOffset + 1
	replacement := []byte(`<link rel="icon" href="` + htmlpkg.EscapeString(logoURL) + `" />`)

	var buf bytes.Buffer
	buf.Write(html[:linkStart])
	buf.Write(replacement)
	buf.Write(html[linkEnd:])
	return buf.Bytes()
}

func safeImageURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//") {
		return trimmed
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "data:image/") {
		return trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return trimmed
}

// injectSiteTitle replaces the static <title> in HTML with the configured site name.
// This ensures the browser tab shows the correct title before JS executes.
func injectSiteTitle(html, settingsJSON []byte) []byte {
	var cfg struct {
		SiteName string `json:"site_name"`
	}
	if err := json.Unmarshal(settingsJSON, &cfg); err != nil || cfg.SiteName == "" {
		return html
	}

	// Find and replace the existing <title>...</title>
	titleStart := bytes.Index(html, []byte("<title>"))
	titleEnd := bytes.Index(html, []byte("</title>"))
	if titleStart == -1 || titleEnd == -1 || titleEnd <= titleStart {
		return html
	}

	newTitle := []byte("<title>" + htmlpkg.EscapeString(cfg.SiteName) + " - ModuRelay AI Gateway</title>")
	var buf bytes.Buffer
	buf.Write(html[:titleStart])
	buf.Write(newTitle)
	buf.Write(html[titleEnd+len("</title>"):])
	return buf.Bytes()
}

// replaceNoncePlaceholder replaces the nonce placeholder with actual nonce value
func replaceNoncePlaceholder(html []byte, nonce string) []byte {
	return bytes.ReplaceAll(html, []byte(NonceHTMLPlaceholder), []byte(nonce))
}

// ServeEmbeddedFrontend returns a middleware for serving embedded frontend
// This is the legacy function for backward compatibility when no settings provider is available
func ServeEmbeddedFrontend() gin.HandlerFunc {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		panic("failed to get dist subdirectory: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(distFS))
	overrideDir := filepath.Join("data", "public")

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if shouldBypassEmbeddedFrontend(path) {
			c.Next()
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}
		if isInfiniteCanvasIndexPath(cleanPath) {
			serveHTMLFile(c, distFS, "infinite-canvas/index.html", true)
			return
		}

		if file, err := distFS.Open(cleanPath); err == nil {
			_ = file.Close()
			// Try local override first
			if tryServeOverrideFile(c, overrideDir, cleanPath) {
				return
			}
			applyStaticAssetCacheHeaders(c.Writer.Header(), cleanPath)
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		if isEmbeddedStaticNamespacePath(cleanPath) || isInfiniteCanvasStaticPath(cleanPath) {
			c.Status(http.StatusNotFound)
			c.Abort()
			return
		}
		if isInfiniteCanvasPath(cleanPath) {
			serveHTMLFile(c, distFS, "infinite-canvas/index.html", true)
			return
		}

		serveIndexHTML(c, distFS)
	}
}

// tryServeOverrideFile is a standalone version of tryServeOverride for legacy usage.
func tryServeOverrideFile(c *gin.Context, overrideDir, cleanPath string) bool {
	if overrideDir == "" {
		return false
	}
	filePath := filepath.Join(overrideDir, filepath.Clean("/"+cleanPath))
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}
	c.File(filePath)
	c.Abort()
	return true
}

func shouldBypassEmbeddedFrontend(path string) bool {
	trimmed := strings.TrimSpace(path)
	return strings.HasPrefix(trimmed, "/api/") ||
		strings.HasPrefix(trimmed, "/v1/") ||
		strings.HasPrefix(trimmed, "/v1beta/") ||
		strings.HasPrefix(trimmed, "/backend-api/") ||
		strings.HasPrefix(trimmed, "/antigravity/") ||
		strings.HasPrefix(trimmed, "/setup/") ||
		trimmed == "/health" ||
		trimmed == "/models" ||
		trimmed == "/responses" ||
		strings.HasPrefix(trimmed, "/responses/") ||
		trimmed == "/alpha/search" ||
		strings.HasPrefix(trimmed, "/images/") ||
		strings.HasPrefix(trimmed, "/videos/")
}

func isEmbeddedStaticNamespacePath(cleanPath string) bool {
	return strings.HasPrefix(strings.TrimPrefix(cleanPath, "/"), "threeui/")
}

func isInfiniteCanvasPath(cleanPath string) bool {
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	return cleanPath == "infinite-canvas" || strings.HasPrefix(cleanPath, "infinite-canvas/")
}

func isInfiniteCanvasIndexPath(cleanPath string) bool {
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	return cleanPath == "infinite-canvas" || cleanPath == "infinite-canvas/" || cleanPath == "infinite-canvas/index.html"
}

func isInfiniteCanvasStaticPath(cleanPath string) bool {
	relative := strings.TrimPrefix(strings.TrimPrefix(cleanPath, "/"), "infinite-canvas/")
	return strings.HasPrefix(relative, "assets/") ||
		strings.HasPrefix(relative, "icons/") ||
		strings.HasPrefix(relative, "plugins/") ||
		relative == "config.js" ||
		relative == "logo.svg"
}

func serveIndexHTML(c *gin.Context, fsys fs.FS) {
	serveHTMLFile(c, fsys, "index.html", false)
}

func serveHTMLFile(c *gin.Context, fsys fs.FS, filename string, replaceNonce bool) {
	file, err := fsys.Open(filename)
	if err != nil {
		c.String(http.StatusNotFound, "Frontend not found")
		c.Abort()
		return
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(file)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read index.html")
		c.Abort()
		return
	}

	if replaceNonce {
		content = replaceNoncePlaceholder(content, middleware.GetNonceFromContext(c))
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	c.Abort()
}

func HasEmbeddedFrontend() bool {
	_, err := frontendFS.ReadFile("dist/index.html")
	return err == nil
}
