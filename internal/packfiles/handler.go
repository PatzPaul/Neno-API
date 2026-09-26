// Package packfiles serves built offline packs (see internal/packs) as immutable static files.
package packfiles

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

// Only exact pack filenames, e.g. bible-SUV-v3.sqlite / egw-sw-SC-v1.sqlite; no dots or slashes elsewhere.
var nameRe = regexp.MustCompile(`^[a-z]+(-[A-Za-z0-9]+)+-v[1-9][0-9]*\.sqlite$`)

// Handler serves GET /packs/{file} from dir. Files are versioned, so they are cached as immutable.
func Handler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("file")
		if !nameRe.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "application/vnd.sqlite3")
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		h.Set("ETag", fmt.Sprintf(`"%s-%d-%d"`, name, st.Size(), st.ModTime().Unix()))
		// ServeContent handles Range, If-None-Match and HEAD.
		http.ServeContent(w, r, name, st.ModTime(), f)
	})
}
