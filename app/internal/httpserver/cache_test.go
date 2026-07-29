package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jorgealonsodev/concontexto/app/internal/httpserver"
)

// TestServeHTTP_CacheHeaders covers "Hashed asset receives immutable
// caching" (PRD §14.3, single source of truth in the Go handler): a
// content-hashed asset path gets a long max-age with immutable; a
// non-hashed HTML document (and other non-hashed assets) does not.
func TestServeHTTP_CacheHeaders(t *testing.T) {
	root := fstest.MapFS{
		"assets/app.a1b2c3d4.js":       &fstest.MapFile{Data: []byte("console.log('hi')")},
		"assets/styles.deadbeef01.css": &fstest.MapFile{Data: []byte("body{}")},
		"index.html":                   &fstest.MapFile{Data: []byte("<html></html>")},
		"legacy.js":                    &fstest.MapFile{Data: []byte("var x = 1;")},
	}
	handler := httpserver.NewServer(root)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	tests := []struct {
		name          string
		path          string
		wantImmutable bool
	}{
		{"hashed js asset", "/assets/app.a1b2c3d4.js", true},
		{"hashed css asset", "/assets/styles.deadbeef01.css", true},
		{"non-hashed html document", "/index.html", false},
		{"non-hashed plain js", "/legacy.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(srv.URL + tt.path)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d", tt.path, resp.StatusCode)
			}

			cacheControl := resp.Header.Get("Cache-Control")
			gotImmutable := strings.Contains(cacheControl, "immutable")

			if gotImmutable != tt.wantImmutable {
				t.Errorf("path %s: Cache-Control=%q, immutable=%v, want immutable=%v",
					tt.path, cacheControl, gotImmutable, tt.wantImmutable)
			}
			if tt.wantImmutable && !strings.Contains(cacheControl, "max-age=31536000") {
				t.Errorf("path %s: expected a long max-age, got Cache-Control=%q", tt.path, cacheControl)
			}
		})
	}
}
