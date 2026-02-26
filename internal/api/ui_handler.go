package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// uiHandler serves the embedded UI filesystem with SPA fallback.
// Unknown paths fall back to index.html for client-side routing.
func uiHandler(uiFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(uiFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		f, err := uiFS.Open(path)
		if err != nil {
			// SPA fallback: serve index.html for unknown paths (client-side routing).
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}
