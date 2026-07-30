// Package alerting is the operational-alert emission policy (spec
// pipeline-operations, "Operational alerts": "The system MUST alert on
// failed ingestion, failed validation and a source recorded as down.
// Alerts MUST name the source and the affected series"). This batch
// (task 9.7/9.8) wires the two triggers task 9.7's own RED scenario and
// task 9.1/9.2's scheduler name explicitly: a validation-failed publish
// gate (app/internal/adapters/postgres.ApplyGate, slice 4) and a source
// recorded down (app/internal/scheduler.Runner, via
// freshness.State.RaisesIncident, slice 5a/6b). A third trigger the
// requirement also names -- plain fetch/transport failure short of the
// 24h incident window -- is already independently auditable via
// download_attempt (spec raw-file-archive) and is not paged separately in
// this batch; see this package's own callers for the exact wiring.
//
// Sink is an interface, not a concrete transport, so this package knows
// nothing about HTTP, email or a paging webhook. The default Sink
// (DefaultSink, LogSink) emits a genuine, operator-visible ERROR-level
// structured log record -- a real, working alert destination requiring no
// new environment variable -- rather than shipping a Sink that silently
// discards everything until a richer transport exists. A concrete
// external transport (Slack, email, PagerDuty) would need its own
// documented env var per this project's env.example convention and is
// deliberately deferred, not half-built without one.
package alerting

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
)

// Kind distinguishes why an Alert was raised.
type Kind string

const (
	KindValidationFailed Kind = "validation-failed"
	KindSourceDown       Kind = "source-down"
	// KindBreakSignalUncovered is raised when a source-reported break/
	// definition-differs flag (Eurostat's b/d, design D-3) has no active
	// series_break covering its period yet -- an operational disclosure,
	// never a publish block (task 2b.6).
	KindBreakSignalUncovered Kind = "break-signal-uncovered"

	// KindDispatchFailed is raised when publishing.Publish's rebuild
	// dispatch (adapters/github, design D-2) cannot be delivered -- never
	// a retry loop, never a failed ingest: the ingested data is safe, only
	// publication latency suffers (slice 4, spec pipeline-operations "A
	// dispatch failure is itself an alert").
	KindDispatchFailed Kind = "dispatch-failed"

	// KindPublishLatencyBreach is raised by the scheduler watchdog when a
	// successful ingestion's rebuild has not completed within the
	// configured publish-latency budget (spec pipeline-operations "An
	// ingestion not followed by a rebuild alerts operators" -- default 30
	// minutes, slice 4). Ops-only: this condition never reaches any
	// reader-facing page (that same requirement's "The condition never
	// reaches a reader" scenario).
	KindPublishLatencyBreach Kind = "publish-latency-breach"
)

// Alert is one operator-facing alert (spec "Alerts MUST name the source
// and the affected series").
type Alert struct {
	Kind         Kind
	Source       string
	Series       string   // empty for a source-level alert (KindSourceDown)
	FailingRules []string // populated only for KindValidationFailed
	Message      string
}

// Sink is the destination for a raised Alert.
type Sink interface {
	Alert(ctx context.Context, a Alert) error
}

// NoopSink discards every alert. Useful as an explicit opt-out (a test
// that wants to assert NOTHING was sent, or a caller that has its own
// alerting already) -- never the implicit default, see LogSink.
type NoopSink struct{}

// Alert satisfies Sink by doing nothing.
func (NoopSink) Alert(context.Context, Alert) error { return nil }

var _ Sink = NoopSink{}

// LogSink emits an Alert as a structured ERROR-level slog record (the
// same slog.Default() convention app/internal/ingestion's own structured
// pipeline logging uses, task 9.5/9.6): an operator watching stderr or a
// container's log driver sees source, series and every failing rule
// without any additional transport to configure. Logger may be left nil,
// in which case slog.Default() is consulted at call time (so a test that
// swaps slog.SetDefault still observes LogSink's output without
// reconstructing it).
type LogSink struct{ Logger *slog.Logger }

// Alert satisfies Sink.
func (s LogSink) Alert(_ context.Context, a Alert) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	attrs := []slog.Attr{
		slog.String("kind", string(a.Kind)),
		slog.String("source", a.Source),
	}
	if a.Series != "" {
		attrs = append(attrs, slog.String("series", a.Series))
	}
	if len(a.FailingRules) > 0 {
		attrs = append(attrs, slog.Any("failing_rules", a.FailingRules))
	}
	logger.LogAttrs(context.Background(), slog.LevelError, "operational alert: "+a.Message, attrs...)
	return nil
}

var _ Sink = LogSink{}

var (
	defaultSinkMu sync.RWMutex
	defaultSink   Sink = LogSink{}
)

// DefaultSink returns the process-wide default alert destination
// (LogSink, unless overridden by SetDefaultSink) -- the same
// package-level-default convention log/slog itself uses for
// slog.Default(), chosen here so IngestSeries and scheduler.Runner never
// need their own exported signature changed to thread a Sink through
// every existing caller (design note: this mirrors task 9.5/9.6's own
// slog.Default() wiring decision for the same reason).
func DefaultSink() Sink {
	defaultSinkMu.RLock()
	defer defaultSinkMu.RUnlock()
	return defaultSink
}

// SetDefaultSink overrides the process-wide default alert destination.
// Tests use this (with a defer restoring the prior value) to inject a spy
// and observe exactly which alerts were raised, without a real transport.
// A nil sink resets to NoopSink -- never to the zero value of the Sink
// interface, which would panic on the first Alert call.
func SetDefaultSink(s Sink) {
	defaultSinkMu.Lock()
	defer defaultSinkMu.Unlock()
	if s == nil {
		s = NoopSink{}
	}
	defaultSink = s
}

// resolve returns sink, or DefaultSink() when sink is nil -- so a caller
// (scheduler.Runner.AlertSink left at its zero value, for instance) never
// needs a nil check of its own.
func resolve(sink Sink) Sink {
	if sink == nil {
		return DefaultSink()
	}
	return sink
}

// ValidationFailed raises a KindValidationFailed alert naming source,
// series and every blocking rule (spec "A validation failure alerts
// without publishing"). It never touches the published datum itself: the
// publish gate's own Block branch (postgres.ApplyGate, slice 4) already
// left the current observation unchanged before this is ever called, so
// "the previously published datum is still served" is true by
// construction of the caller's own ordering, not by anything this
// function does.
func ValidationFailed(ctx context.Context, sink Sink, source, series string, failingRules []string) error {
	return resolve(sink).Alert(ctx, Alert{
		Kind:         KindValidationFailed,
		Source:       source,
		Series:       series,
		FailingRules: failingRules,
		Message:      fmt.Sprintf("validation failed for %s/%s: %s", source, series, strings.Join(failingRules, ", ")),
	})
}

// SourceDown raises a KindSourceDown alert (spec "MUST alert on ... a
// source recorded as down"). It is called from
// app/internal/scheduler.Runner.Run at the exact point
// freshness.State.RaisesIncident() first becomes true for a scheduled
// attempt -- never re-deriving the 24h boundary here, only reporting it.
func SourceDown(ctx context.Context, sink Sink, source string) error {
	return resolve(sink).Alert(ctx, Alert{
		Kind:    KindSourceDown,
		Source:  source,
		Message: fmt.Sprintf("source %s has been down for over %s", source, freshness.Window),
	})
}

// BreakSignalUncovered raises a KindBreakSignalUncovered alert (task
// 2b.6, design D-3): a source-reported break ("b") or definition-differs
// ("d") flag whose period has no already-active series_break covering it
// yet. It is called from app/internal/ingestion.IngestSeries right after
// breaks are resolved for the run. The observation itself still
// publishes as definitive (b/d are metadata, never a status, spec
// "Break and definition flags are metadata, not statuses") -- this alert
// is purely operational, telling an editor the series_break metadata
// linkage (rupturas.yaml, the only writer) is still missing, not that
// anything failed to publish.
func BreakSignalUncovered(ctx context.Context, sink Sink, source, series, period, flag string) error {
	return resolve(sink).Alert(ctx, Alert{
		Kind:    KindBreakSignalUncovered,
		Source:  source,
		Series:  series,
		Message: fmt.Sprintf("%s/%s reported a %q flag at period %s with no covering series_break yet", source, series, flag, period),
	})
}

// DispatchFailed raises a KindDispatchFailed alert (spec pipeline-
// operations, "A dispatch failure is itself an alert" -- "naming the run
// and the dispatch failure"). Called from publishing.Publish the moment
// its Dispatcher returns an error -- best-effort, same convention as
// every other alert in this package: a failed alert delivery must never
// fail the publish cycle itself.
//
// Publish runs once per INGEST CYCLE (design D-2), which may cover
// several series each with its own ingestion_run_id -- there is no
// single canonical run id at that layer. generatedAt (the artifact's own
// manifest.generated_at, always set) names the specific export/publish
// cycle instead -- a reasoned, disclosed reading of "naming the run"
// given this layer genuinely has no single run identifier to name.
func DispatchFailed(ctx context.Context, sink Sink, generatedAt time.Time, dispatchErr error) error {
	return resolve(sink).Alert(ctx, Alert{
		Kind:    KindDispatchFailed,
		Message: fmt.Sprintf("rebuild dispatch failed for the publish cycle generated at %s: %v", generatedAt.UTC().Format(time.RFC3339), dispatchErr),
	})
}

// PublishLatencyBreach raises a KindPublishLatencyBreach alert (spec
// pipeline-operations, "A stalled rebuild raises an alert" / "A within-
// budget cycle raises nothing" -- callers only call this once a breach is
// already decided, this function never re-derives the budget itself).
func PublishLatencyBreach(ctx context.Context, sink Sink, source, series string, elapsed time.Duration) error {
	return resolve(sink).Alert(ctx, Alert{
		Kind:    KindPublishLatencyBreach,
		Source:  source,
		Series:  series,
		Message: fmt.Sprintf("%s/%s: publish latency budget breached, %s elapsed since ingestion success", source, series, elapsed),
	})
}
