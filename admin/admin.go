// Package admin provides the legacy HTML/JSON monitoring endpoints for sseserver.
package admin

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mroth/sseserver"
)

//go:embed index.html
var html []byte

// Handles serving the static HTML page
func adminStatusHTMLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
}

// Handles serving the JSON status data, effectively the admin API endpoint
func adminStatusDataHandler(w http.ResponseWriter, r *http.Request, s *sseserver.Server) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.MarshalIndent(s.Status(), "", "  ")
	fmt.Fprint(w, string(b))
}

func AdminHandler(s *sseserver.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Options.DisableAdminEndpoints {
			http.Error(w, "403 admin endpoint disabled", http.StatusForbidden)
			return
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/admin/", adminStatusHTMLHandler)
		mux.HandleFunc("/admin/status.json", func(w http.ResponseWriter, r *http.Request) {
			adminStatusDataHandler(w, r, s)
		})
		mux.ServeHTTP(w, r)
	})
}
