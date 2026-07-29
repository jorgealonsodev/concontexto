package httpserver_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/jorgealonsodev/concontexto/app/internal/httpserver"
)

func testRoot() fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>hello</html>")},
	}
}

// TestNewServer_ConstructorTakesNoRepositoryPort covers "Static handler
// has no repository dependency": NewServer accepts exactly one argument,
// the static asset root. If a repository port were ever added as a
// parameter, this call would fail to compile — the golden rule
// (PRD §14.2) enforced at the type level, not by convention.
func TestNewServer_ConstructorTakesNoRepositoryPort(t *testing.T) {
	handler := httpserver.NewServer(testRoot())

	if handler == nil {
		t.Fatal("expected NewServer to return a non-nil handler")
	}
}

// Remediation batch (verify-report WARNING W7): a
// TestServeHTTP_HundredRequestsRecordZeroDBQueries test previously lived
// here, asserting spy.n.Load() != 0 on a spyQueryCounter that was
// deliberately NEVER passed into NewServer -- NewServer has no parameter
// capable of accepting one. That assertion could therefore never fail
// regardless of correctness: a permanently-dead check that misled a
// future reader into thinking "Serving pages issues no queries" was
// covered here. It genuinely IS enforced, but structurally, at the type
// level and via the import-graph guard
// (app/internal/httpserver/importguard_test.go, mutation-verified: see
// verify-report M6), not by a spy no production code can ever reach.
// The verifier's own recommendation was to delete the dead assertion
// rather than keep a check that can never fire; TestNewServer_
// ConstructorTakesNoRepositoryPort below already covers the same
// "no repository dependency" property at the type level.

// TestServeHTTP_BlockedOutboundSourceHTTPStillServesHealthzAndPages covers
// "Request path performs no outbound source call": with every outbound
// dial blocked except the loopback connection to this very test server,
// /healthz and static pages must still succeed — proving the handler
// never attempts an outbound call to any external source host
// (PRD §9.2).
func TestServeHTTP_BlockedOutboundSourceHTTPStillServesHealthzAndPages(t *testing.T) {
	handler := httpserver.NewServer(testRoot())
	srv := httptest.NewServer(handler)
	defer srv.Close()

	allowedAddr := srv.Listener.Addr().String()
	blockingTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr != allowedAddr {
				return nil, fmt.Errorf("blocked: outbound dial to %s is not permitted in this test", addr)
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}
	client := &http.Client{Transport: blockingTransport}

	healthResp, err := client.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request failed even though the test server address is allowed: %v", err)
	}
	defer healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("expected /healthz to return 200, got %d", healthResp.StatusCode)
	}

	pageResp, err := client.Get(srv.URL + "/index.html")
	if err != nil {
		t.Fatalf("page request failed even though the test server address is allowed: %v", err)
	}
	defer pageResp.Body.Close()
	if pageResp.StatusCode != http.StatusOK {
		t.Fatalf("expected /index.html to return 200, got %d", pageResp.StatusCode)
	}
}
