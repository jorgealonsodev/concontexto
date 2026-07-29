package ine

// Remediation batch (verify-report WARNING W6): NewClient(baseURL, nil)
// fell back to http.DefaultClient, which carries Timeout: 0 (unbounded).
// The orchestrator's own live-probe attempt hung until killed at 180s
// rather than failing -- an unresponsive source blocks an ingestion job
// forever, defeating §9.2's retry/backoff design (you cannot back off
// from a call that never returns). This is a white-box (package ine, not
// ine_test) test specifically so it can inspect the unexported
// httpClient field directly, proving the DEFAULT client construction
// path carries a bounded Timeout without waiting out a real timeout in
// CI.

import "testing"

func TestNewClient_NilHTTPClientDefaultsToABoundedTimeout(t *testing.T) {
	c := NewClient("https://example.test", nil)
	if c.httpClient.Timeout <= 0 {
		t.Fatalf("expected NewClient(baseURL, nil)'s default http.Client to carry a positive Timeout, got %v (unbounded -- exactly the hang the live probe hit)", c.httpClient.Timeout)
	}
}
