package httpserver

import (
	"net/http"
	"regexp"
)

// hashedAssetPattern matches build-hashed filenames such as
// "app.a1b2c3d4.js" or "styles.deadbeef01.css" — the content-hash
// convention produced by the Astro build (PR 1b onward).
var hashedAssetPattern = regexp.MustCompile(`\.[0-9a-f]{8,}\.[A-Za-z0-9]+$`)

func isHashedAsset(path string) bool {
	return hashedAssetPattern.MatchString(path)
}

// withCacheHeaders is the single source of truth for cache-control
// headers (PRD §14.3): a content-hashed asset is safe to cache forever
// because its filename changes whenever its content does, so it gets a
// long max-age plus immutable; anything else (in particular non-hashed
// HTML) does not.
func withCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isHashedAsset(r.URL.Path) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}
