// Package frontend embeds the production React application served by the
// SyncSpace backend. The browser UI is deliberately local-only; peer devices
// interact with the versioned transfer protocol instead.
package frontend

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed dist
var assets embed.FS

// Handler returns an immutable-asset-aware SPA handler. Unknown paths fall
// back to index.html so future client-side routes remain refresh-safe.
func Handler() http.Handler {
	root, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		assetPath := strings.TrimPrefix(path.Clean("/"+request.URL.Path), "/")
		if assetPath == "." || assetPath == "" {
			assetPath = "index.html"
		}
		contents, readErr := fs.ReadFile(root, assetPath)
		if readErr != nil {
			assetPath = "index.html"
			contents, readErr = fs.ReadFile(root, assetPath)
		}
		if readErr != nil {
			http.Error(response, "SyncSpace frontend is unavailable", http.StatusInternalServerError)
			return
		}
		if strings.HasPrefix(assetPath, "assets/") {
			response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			response.Header().Set("Cache-Control", "no-cache")
		}
		response.Header().Set("X-Content-Type-Options", "nosniff")
		if contentType := mime.TypeByExtension(path.Ext(assetPath)); contentType != "" {
			response.Header().Set("Content-Type", contentType)
		}
		response.WriteHeader(http.StatusOK)
		if request.Method != http.MethodHead {
			_, _ = response.Write(contents)
		}
	})
}
