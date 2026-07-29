package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/httpserver"
)

const defaultPort = "8080"

// schedulerStarter is the function cmdServe calls to launch the
// in-process scheduler. A package-level var (not a direct startScheduler
// call) so a test can substitute a spy and prove cmdServe genuinely
// wires the scheduler -- independent of a live DATABASE_URL or a real
// pgxpool -- without needing a full startScheduler run (verify-report
// CRITICAL C7: before this batch, the startScheduler call in cmdServe
// had zero test references anywhere, so deleting it left every package
// green; see TestCmdServe_StartsTheScheduler in serve_test.go, which
// fails the instant this wiring is removed).
var schedulerStarter = startScheduler

// staticAssetRoot returns the directory the static handler serves from.
// PR 1b builds the Astro hello-world into this path; until then the
// directory may not exist, which os.DirFS tolerates lazily (requests
// simply 404 instead of failing the process).
func staticAssetRoot() string {
	if root := os.Getenv("STATIC_ROOT"); root != "" {
		return root
	}
	return "web/dist"
}

// runServe starts the HTTP server on ln and blocks until ctx is
// cancelled or the listener fails. It never imports or calls the
// migrate package — schema migrations run ONLY via the explicit
// `migrate` subcommand, never on boot (spec platform-runtime,
// "Migrations run only on explicit command").
func runServe(ctx context.Context, ln net.Listener, root fs.FS) error {
	httpSrv := &http.Server{Handler: httpserver.NewServer(root)}

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.Serve(ln) }()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}

func cmdServe(args []string, stdout, stderr io.Writer) int {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// schedulerStarter (== startScheduler in production) runs strictly
	// BESIDE the HTTP server below, never inside its request path
	// (verify-report CRITICAL C4; design.md "serve = static server +
	// /healthz + in-process scheduler"). It returns immediately; its own
	// goroutine is bound to ctx and stops on the same shutdown signal
	// runServe does.
	schedulerStarter(ctx, stderr)

	if err := runServe(ctx, ln, os.DirFS(staticAssetRoot())); err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 1
	}
	fmt.Fprintln(stdout, "serve: stopped")
	return 0
}
