package xlsx

// Remediation batch (verify-report WARNING W6): see the identical
// white-box proof in app/internal/adapters/ine/client_internal_test.go
// for the full rationale -- every adapter shared the same
// http.DefaultClient (Timeout: 0, unbounded) fallback.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func TestNewClient_NilHTTPClientDefaultsToABoundedTimeout(t *testing.T) {
	c := NewClient(config.XLSXSchemaConfig{}, nil)
	if c.httpClient.Timeout <= 0 {
		t.Fatalf("expected NewClient(schema, nil)'s default http.Client to carry a positive Timeout, got %v (unbounded)", c.httpClient.Timeout)
	}
}
