package migrate_test

import (
	"context"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/migrate"
)

type fakeRunner struct {
	upCalled, downCalled, statusCalled bool
	upErr, downErr, statusErr          error
	statusMsg                          string
}

func (f *fakeRunner) Up(context.Context) error   { f.upCalled = true; return f.upErr }
func (f *fakeRunner) Down(context.Context) error { f.downCalled = true; return f.downErr }
func (f *fakeRunner) Status(context.Context) (string, error) {
	f.statusCalled = true
	return f.statusMsg, f.statusErr
}

// TestExecute_DispatchesToRunner triangulates Execute's up/down/status
// dispatch plus the unknown-command error path.
func TestExecute_DispatchesToRunner(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{"up dispatches to Up", "up", false},
		{"down dispatches to Down", "down", false},
		{"status dispatches to Status", "status", false},
		{"unknown command errors", "bogus", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{statusMsg: "at version 3"}

			msg, err := migrate.Execute(context.Background(), tt.cmd, runner)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for cmd %q", tt.cmd)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			switch tt.cmd {
			case "up":
				if !runner.upCalled {
					t.Error("expected Up to be called")
				}
			case "down":
				if !runner.downCalled {
					t.Error("expected Down to be called")
				}
			case "status":
				if !runner.statusCalled {
					t.Error("expected Status to be called")
				}
				if msg != "at version 3" {
					t.Errorf("expected the status message to propagate, got %q", msg)
				}
			}
		})
	}
}

// TestExecute_IncrementsExecutionCount proves the counter that
// serve_test.go relies on for its boot-safety invariant actually
// reflects real invocations.
func TestExecute_IncrementsExecutionCount(t *testing.T) {
	baseline := migrate.ExecutionCount()

	_, _ = migrate.Execute(context.Background(), "status", &fakeRunner{})

	if got := migrate.ExecutionCount(); got != baseline+1 {
		t.Fatalf("expected ExecutionCount to increment by 1 from %d, got %d", baseline, got)
	}
}

// TestNotConfiguredRunner_AllOperationsFail proves the PR-1a placeholder
// Runner fails clearly instead of silently doing nothing.
func TestNotConfiguredRunner_AllOperationsFail(t *testing.T) {
	r := migrate.NotConfiguredRunner{}

	if err := r.Up(context.Background()); err == nil {
		t.Error("expected Up to fail")
	}
	if err := r.Down(context.Background()); err == nil {
		t.Error("expected Down to fail")
	}
	if _, err := r.Status(context.Background()); err == nil {
		t.Error("expected Status to fail")
	}
}
