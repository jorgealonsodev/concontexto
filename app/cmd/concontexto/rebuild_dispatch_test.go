package main

// CRITICAL-28 (RED), link 2: "The dispatcher is never composed in the
// deployed stack." buildDispatcher used to return a bare nil whenever
// GITHUB_DISPATCH_REPO or GITHUB_DISPATCH_TOKEN was empty, and
// publishing.Publish then returned silently. That is correct for local
// development and for tests -- nobody asked for a dispatch -- and wrong for
// a deployed stack, where nobody asked precisely BECAUSE the compose file
// forgot to pass the variables, and the entire publish loop then runs to
// completion with no rebuild and no sound.
//
// These tests pin the three states an operator must be able to tell apart,
// and pin that the distinction survives into the STRUCTURED log rather than
// only into a human-readable line: "off" and "broken" produce different
// records, at different levels, naming the reason.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

// captureStructuredLog swaps slog's process-wide default for a JSON handler
// over a buffer for the duration of fn, and returns every record emitted.
// The same slog.Default() convention alerting.LogSink already relies on, so
// what this observes is exactly what an operator's container log driver
// receives.
func captureStructuredLog(t *testing.T, fn func()) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	prior := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prior)

	fn()

	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("structured log line is not JSON: %q: %v", line, err)
		}
		records = append(records, rec)
	}
	return records
}

// findRebuildDispatchRecord returns the one rebuild-dispatch record in
// records, failing the test when there is not exactly one.
func findRebuildDispatchRecord(t *testing.T, records []map[string]any) map[string]any {
	t.Helper()
	var found []map[string]any
	for _, rec := range records {
		if rec["component"] == "rebuild-dispatch" {
			found = append(found, rec)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one component=rebuild-dispatch structured record, got %d: %v", len(found), records)
	}
	return found[0]
}

func TestBuildDispatcher_NotConfiguredAndNotExpectedIsDeliberatelyOff(t *testing.T) {
	t.Setenv("GITHUB_DISPATCH_REPO", "")
	t.Setenv("GITHUB_DISPATCH_TOKEN", "")
	t.Setenv("APP_REBUILD_DISPATCH", "")

	var dispatcher any
	records := captureStructuredLog(t, func() {
		d := buildDispatcher()
		if d == nil {
			dispatcher = nil
		} else {
			dispatcher = d
		}
	})

	if dispatcher != nil {
		t.Error("expected a nil dispatcher when nothing is configured and no rebuild is expected")
	}
	rec := findRebuildDispatchRecord(t, records)
	if rec["state"] != "disabled" {
		t.Errorf("expected state=disabled, got %v", rec["state"])
	}
	if rec["level"] != "INFO" {
		t.Errorf("a deliberate opt-out is not a fault: expected level INFO, got %v", rec["level"])
	}
}

func TestBuildDispatcher_ExplicitlyOffIsDeliberatelyOffEvenWhenConfigured(t *testing.T) {
	t.Setenv("GITHUB_DISPATCH_REPO", "owner/repo")
	t.Setenv("GITHUB_DISPATCH_TOKEN", "ghp_secret")
	t.Setenv("APP_REBUILD_DISPATCH", "off")

	var got any
	records := captureStructuredLog(t, func() {
		if d := buildDispatcher(); d != nil {
			got = d
		}
	})

	if got != nil {
		t.Error("APP_REBUILD_DISPATCH=off must win over a configured repo/token: an operator turning dispatch off means it")
	}
	if rec := findRebuildDispatchRecord(t, records); rec["state"] != "disabled" {
		t.Errorf("expected state=disabled, got %v", rec["state"])
	}
}

func TestBuildDispatcher_FullyConfiguredDispatches(t *testing.T) {
	t.Setenv("GITHUB_DISPATCH_REPO", "owner/repo")
	t.Setenv("GITHUB_DISPATCH_TOKEN", "ghp_secret")
	t.Setenv("APP_REBUILD_DISPATCH", "required")

	var dispatcher func(context.Context, time.Time, string) error
	records := captureStructuredLog(t, func() {
		dispatcher = buildDispatcher()
	})

	if dispatcher == nil {
		t.Fatal("expected a real dispatcher when repo and token are both set")
	}
	rec := findRebuildDispatchRecord(t, records)
	if rec["state"] != "enabled" {
		t.Errorf("expected state=enabled, got %v", rec["state"])
	}
	if rec["repo"] != "owner/repo" {
		t.Errorf("expected the record to name the target repository, got %v", rec["repo"])
	}
	// The token must never reach a log record.
	for k, v := range rec {
		if s, ok := v.(string); ok && strings.Contains(s, "ghp_secret") {
			t.Errorf("the dispatch token leaked into the structured log at key %q", k)
		}
	}
}

func TestBuildDispatcher_ExpectedButUnconfiguredIsBrokenNotOff(t *testing.T) {
	tests := []struct {
		name        string
		repo        string
		token       string
		wantNamedIn string
	}{
		{name: "neither set", repo: "", token: "", wantNamedIn: "GITHUB_DISPATCH_REPO"},
		{name: "repo set, token missing", repo: "owner/repo", token: "", wantNamedIn: "GITHUB_DISPATCH_TOKEN"},
		{name: "token set, repo missing", repo: "", token: "ghp_secret", wantNamedIn: "GITHUB_DISPATCH_REPO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_DISPATCH_REPO", tt.repo)
			t.Setenv("GITHUB_DISPATCH_TOKEN", tt.token)
			t.Setenv("APP_REBUILD_DISPATCH", "required")

			var dispatcher func(context.Context, time.Time, string) error
			records := captureStructuredLog(t, func() {
				dispatcher = buildDispatcher()
			})

			if dispatcher == nil {
				t.Fatal("a rebuild that is EXPECTED but unconfigured must never yield a nil dispatcher: nil is publishing.Publish's silent-skip path, which is exactly the silence CRITICAL-28 is about")
			}

			err := dispatcher(context.Background(), time.Now(), "digest")
			if err == nil {
				t.Fatal("expected the unconfigured dispatcher to fail immediately so publishing.Publish raises alerting.DispatchFailed")
			}
			if !strings.Contains(err.Error(), tt.wantNamedIn) {
				t.Errorf("expected the error to name the missing variable %s, got %v", tt.wantNamedIn, err)
			}

			rec := findRebuildDispatchRecord(t, records)
			if rec["state"] != "unconfigured" {
				t.Errorf("expected state=unconfigured (broken), never state=disabled (deliberately off), got %v", rec["state"])
			}
			if rec["level"] != "ERROR" {
				t.Errorf("an expected-but-unconfigured dispatch is a fault: expected level ERROR, got %v", rec["level"])
			}
		})
	}
}

func TestBuildDispatcher_AnUnrecognisedModeFailsLoudRatherThanSilent(t *testing.T) {
	t.Setenv("GITHUB_DISPATCH_REPO", "")
	t.Setenv("GITHUB_DISPATCH_TOKEN", "")
	t.Setenv("APP_REBUILD_DISPATCH", "yes-please")

	var dispatcher func(context.Context, time.Time, string) error
	records := captureStructuredLog(t, func() {
		dispatcher = buildDispatcher()
	})

	if dispatcher == nil {
		t.Fatal("an unparsable APP_REBUILD_DISPATCH must fail LOUD (treated as required), never silently disable the whole publish loop over a typo")
	}
	if rec := findRebuildDispatchRecord(t, records); rec["state"] != "unconfigured" {
		t.Errorf("expected state=unconfigured, got %v", rec["state"])
	}
}

// CRITICAL-28 (RED), link 2, caller half. runIngest printed
// "ingest: publish: exported and dispatched to <dir>" unconditionally --
// including in the deployed stack, where the dispatcher was nil and nothing
// whatsoever was dispatched. An operator reading the container log was told
// a rebuild had been triggered when none had.
func TestPublishOutcomeMessage_NeverClaimsADispatchThatDidNotHappen(t *testing.T) {
	dispatchedAt := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		result      publishing.PublishResult
		wantSaid    []string
		wantNotSaid []string
		wantFault   bool
	}{
		{
			name:      "dispatched",
			result:    publishing.PublishResult{DispatchedAt: &dispatchedAt},
			wantSaid:  []string{"exported and dispatched", "/tmp/out", "published"},
			wantFault: false,
		},
		{
			name:        "deliberately off",
			result:      publishing.PublishResult{DispatchSkipped: true},
			wantSaid:    []string{"exported", "/tmp/out", "rebuild dispatch is disabled"},
			wantNotSaid: []string{"and dispatched"},
			wantFault:   false,
		},
		{
			name:        "dispatch attempted and failed",
			result:      publishing.PublishResult{},
			wantSaid:    []string{"exported", "/tmp/out", "rebuild dispatch failed"},
			wantNotSaid: []string{"and dispatched"},
			wantFault:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, fault := publishOutcomeMessage("/tmp/out", "published", tt.result)
			for _, want := range tt.wantSaid {
				if !strings.Contains(message, want) {
					t.Errorf("expected the message to contain %q, got %q", want, message)
				}
			}
			for _, notWant := range tt.wantNotSaid {
				if strings.Contains(message, notWant) {
					t.Errorf("the message claims %q when no dispatch happened: %q", notWant, message)
				}
			}
			if fault != tt.wantFault {
				t.Errorf("fault = %v, want %v (only a FAILED dispatch is a fault; a deliberate opt-out is not)", fault, tt.wantFault)
			}
		})
	}
}

// An unquoted `off` in a compose file is YAML 1.1's boolean FALSE, so
// `APP_REBUILD_DISPATCH: off` arrives here as the string "false". An
// operator who wrote that meant off, and must get off -- the loud path is
// for values whose intent is genuinely unclear, not for a quoting rule
// invisible from where they typed it.
func TestBuildDispatcher_YAMLManglesOffIntoFalseAndOffStillMeansOff(t *testing.T) {
	for _, value := range []string{"off", "OFF", " off ", "false", "0"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("GITHUB_DISPATCH_REPO", "")
			t.Setenv("GITHUB_DISPATCH_TOKEN", "")
			t.Setenv("APP_REBUILD_DISPATCH", value)

			var got any
			captureStructuredLog(t, func() {
				if d := buildDispatcher(); d != nil {
					got = d
				}
			})
			if got != nil {
				t.Errorf("APP_REBUILD_DISPATCH=%q must mean off, not required", value)
			}
		})
	}
}
