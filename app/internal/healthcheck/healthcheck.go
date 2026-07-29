// Package healthcheck implements the shell-free "healthcheck" subcommand
// (spec platform-runtime, "Health endpoint and shell-free healthcheck").
// The distroless production image has no shell and no curl, so the
// container HEALTHCHECK re-invokes this binary as `healthcheck` instead
// of shelling out to curl.
package healthcheck

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// Pinger reports whether a dependency (e.g. PostgreSQL) is reachable.
type Pinger func(ctx context.Context) error

// Check performs the shallow GET /healthz probe against addr, and
// additionally invokes ping when deep is true and ping is non-nil. A
// plain (non-deep) check never invokes ping, regardless of what it would
// return.
func Check(ctx context.Context, client *http.Client, addr string, deep bool, ping Pinger) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: /healthz returned status %d", resp.StatusCode)
	}
	if deep && ping != nil {
		return ping(ctx)
	}
	return nil
}

// TCPPing returns a Pinger that performs a raw TCP dial against addr. It
// is the PR-1a placeholder for --deep's PostgreSQL reachability check; a
// protocol-level ping arrives once the postgres adapter exists (PR 2a).
func TCPPing(addr string, timeout time.Duration) Pinger {
	return func(ctx context.Context) error {
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("healthcheck: postgres unreachable at %s: %w", addr, err)
		}
		return conn.Close()
	}
}

// Run is the subcommand entry point: parses --deep, probes /healthz on
// 127.0.0.1:$PORT, and exits 0 on success or 1 on failure.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	deep := fs.Bool("deep", false, "also verify PostgreSQL reachability")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "127.0.0.1:" + port

	var ping Pinger
	if *deep {
		dbAddr := os.Getenv("POSTGRES_ADDR")
		if dbAddr == "" {
			dbAddr = "127.0.0.1:5432"
		}
		ping = TCPPing(dbAddr, 2*time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Second}

	if err := Check(ctx, client, addr, *deep, ping); err != nil {
		fmt.Fprintln(stderr, "healthcheck: failed:", err)
		return 1
	}
	fmt.Fprintln(stdout, "healthcheck: ok")
	return 0
}
