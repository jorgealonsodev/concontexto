// Package httpserver implements the driving HTTP adapter: static assets
// plus /healthz. It intentionally has NO repository port and issues NO
// outbound HTTP of its own — PRD §14.2's golden rule ("zero DB queries
// and zero computation at request time") and PRD §9.2 ("no external
// source call at request time") are both enforced by this package's
// import graph, proven by the import-graph guard test in this package.
package httpserver

import (
	"io/fs"
	"net/http"
)

// NewServer builds the static-file + /healthz handler. The single
// argument is the static asset root; there is no repository port
// parameter, by design (PRD §14.2 golden rule) — nothing in this package
// can query a database because nothing here holds a connection.
func NewServer(root fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.Handle("/", withCacheHeaders(http.FileServer(http.FS(root))))
	return mux
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
