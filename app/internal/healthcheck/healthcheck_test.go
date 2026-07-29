package healthcheck_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/healthcheck"
)

func newHealthyServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func addrOf(srv *httptest.Server) string {
	return strings.TrimPrefix(srv.URL, "http://")
}

// TestCheck_ShallowSucceedsWhenServing covers "Shallow healthcheck
// succeeds": a running serve process answers 0 (nil error) when
// re-invoked as healthcheck.
func TestCheck_ShallowSucceedsWhenServing(t *testing.T) {
	srv := newHealthyServer(t)

	err := healthcheck.Check(context.Background(), srv.Client(), addrOf(srv), false, nil)

	if err != nil {
		t.Fatalf("expected shallow healthcheck to succeed, got %v", err)
	}
}

// TestCheck_DeepFailsWhenPostgresUnreachable covers "Deep healthcheck
// fails when the database is unreachable".
func TestCheck_DeepFailsWhenPostgresUnreachable(t *testing.T) {
	srv := newHealthyServer(t)
	failingPing := func(ctx context.Context) error { return errors.New("postgres unreachable") }

	err := healthcheck.Check(context.Background(), srv.Client(), addrOf(srv), true, failingPing)

	if err == nil {
		t.Fatal("expected deep healthcheck to fail when postgres is unreachable")
	}
}

// TestCheck_DeepSucceedsWhenPostgresReachable covers the same scenario's
// positive half: --deep succeeds when Postgres is reachable.
func TestCheck_DeepSucceedsWhenPostgresReachable(t *testing.T) {
	srv := newHealthyServer(t)
	succeedingPing := func(ctx context.Context) error { return nil }

	err := healthcheck.Check(context.Background(), srv.Client(), addrOf(srv), true, succeedingPing)

	if err != nil {
		t.Fatalf("expected deep healthcheck to succeed when postgres is reachable, got %v", err)
	}
}

// TestCheck_PlainHealthcheckIgnoresPingEvenIfItWouldFail covers "a plain
// healthcheck without --deep still exits 0" — it must never even invoke
// the DB ping.
func TestCheck_PlainHealthcheckIgnoresPingEvenIfItWouldFail(t *testing.T) {
	srv := newHealthyServer(t)
	pingWasCalled := false
	failingPing := func(ctx context.Context) error {
		pingWasCalled = true
		return errors.New("should never be invoked")
	}

	err := healthcheck.Check(context.Background(), srv.Client(), addrOf(srv), false, failingPing)

	if err != nil {
		t.Fatalf("expected plain healthcheck to succeed regardless of DB state, got %v", err)
	}
	if pingWasCalled {
		t.Fatal("expected the ping function to never be invoked when deep is false")
	}
}

// TestCheck_FailsWhenHealthzUnreachable proves the shallow probe itself
// is meaningful: it fails when nothing is serving.
func TestCheck_FailsWhenHealthzUnreachable(t *testing.T) {
	err := healthcheck.Check(context.Background(), http.DefaultClient, "127.0.0.1:1", false, nil)

	if err == nil {
		t.Fatal("expected an error when /healthz cannot be reached")
	}
}

// TestTCPPing_SucceedsAgainstOpenPortAndFailsAgainstClosed triangulates
// the default --deep Postgres reachability check.
func TestTCPPing_SucceedsAgainstOpenPortAndFailsAgainstClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind test listener: %v", err)
	}
	defer ln.Close()

	openPing := healthcheck.TCPPing(ln.Addr().String(), time.Second)
	if err := openPing(context.Background()); err != nil {
		t.Fatalf("expected ping to succeed against an open port, got %v", err)
	}

	closedPing := healthcheck.TCPPing("127.0.0.1:1", time.Second)
	if err := closedPing(context.Background()); err == nil {
		t.Fatal("expected ping to fail against a closed/refused port")
	}
}

// TestRun_ShallowSucceedsAgainstRunningServer exercises the CLI entry
// point end to end (flag parsing + PORT env resolution).
func TestRun_ShallowSucceedsAgainstRunningServer(t *testing.T) {
	srv := newHealthyServer(t)
	port := strings.TrimPrefix(addrOf(srv), "127.0.0.1:")
	t.Setenv("PORT", port)

	var stdout, stderr bytes.Buffer
	code := healthcheck.Run(nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}
}

// TestRun_DeepFailsWhenPostgresAddrUnreachable exercises --deep through
// the CLI entry point.
func TestRun_DeepFailsWhenPostgresAddrUnreachable(t *testing.T) {
	srv := newHealthyServer(t)
	port := strings.TrimPrefix(addrOf(srv), "127.0.0.1:")
	t.Setenv("PORT", port)
	t.Setenv("POSTGRES_ADDR", "127.0.0.1:1")

	var stdout, stderr bytes.Buffer
	code := healthcheck.Run([]string{"--deep"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d; stderr=%q", code, stderr.String())
	}
}
