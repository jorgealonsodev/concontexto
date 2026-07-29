package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/migrate"
)

// TestRunServe_NeverInvokesMigrateOnBoot covers "Boot does not migrate":
// starting serve against a schema one migration behind (simulated here
// by migrate never having run at all) must apply zero migrations.
// Auto-migrate races across replicas and fires during rollbacks
// (design.md), so schema mutation is only ever allowed through the
// explicit `migrate` subcommand.
func TestRunServe_NeverInvokesMigrateOnBoot(t *testing.T) {
	baseline := migrate.ExecutionCount()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind test listener: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runServe(ctx, ln, fstest.MapFS{}) }()

	waitForHealthz(t, ln.Addr().String())

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runServe returned an error on shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runServe did not shut down within the deadline")
	}

	if got := migrate.ExecutionCount(); got != baseline {
		t.Fatalf("serve must never invoke migrate: ExecutionCount changed from %d to %d", baseline, got)
	}
}

// TestCmdServe_StartsTheScheduler covers "the CRITICAL C7 wiring
// obligation": the schedule.go doc comment claims cmdServe launches the
// in-process scheduler, but before this batch nothing pinned that claim
// -- deleting the startScheduler call left every package green
// (verify-report mutation M1). This test substitutes schedulerStarter
// with a spy and drives cmdServe through a REAL signal-based shutdown
// (the same syscall.SIGTERM cmdServe's own signal.NotifyContext
// listens for), so it exercises cmdServe itself, not runScheduler or
// scheduleSourceOp called directly the way schedule_test.go and
// schedule_integration_test.go already do -- this is the wiring point
// those tests cannot see. Mutation-tested: removing the
// schedulerStarter(ctx, stderr) call from cmdServe makes this test fail
// (verify-report C7 proof obligation); restoring it makes it pass again.
func TestCmdServe_StartsTheScheduler(t *testing.T) {
	prevStarter := schedulerStarter
	started := make(chan struct{}, 1)
	schedulerStarter = func(context.Context, io.Writer) {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	t.Cleanup(func() { schedulerStarter = prevStarter })

	t.Setenv("PORT", "0")
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- cmdServe(nil, &stdout, &stderr) }()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("cmdServe did not start the scheduler within the deadline")
	}

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("sending SIGTERM to self to trigger graceful shutdown: %v", err)
	}

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("cmdServe exited %d, stderr=%q", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cmdServe did not shut down within the deadline after SIGTERM")
	}
}

// TestSchedulerStarterDefaultBindingResolvesToStartScheduler covers the
// other half of the C7/C8 wiring obligation (verify-report CRITICAL
// C8/M4): TestCmdServe_StartsTheScheduler above pins that cmdServe calls
// whatever schedulerStarter currently holds, but that test REPLACES
// schedulerStarter with its own spy, so it can never prove what the
// package-level var is bound to by default -- rebinding
// `var schedulerStarter = startScheduler` to a no-op left the entire
// suite green. This test asserts the DEFAULT (package-init) binding
// resolves to the real startScheduler function, comparing the two
// func values' code pointers via reflect -- the standard Go technique for
// asserting plain top-level function identity.
func TestSchedulerStarterDefaultBindingResolvesToStartScheduler(t *testing.T) {
	got := reflect.ValueOf(schedulerStarter).Pointer()
	want := reflect.ValueOf(startScheduler).Pointer()
	if got != want {
		t.Fatalf("schedulerStarter's default binding does not resolve to startScheduler (got func @%#x, want @%#x) -- "+
			"cmdServe's pinned call to schedulerStarter would silently launch a different function in production",
			got, want)
	}
}

func waitForHealthz(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not become healthy before the deadline")
}
