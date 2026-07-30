// Package ingestion is the APPLICATION layer (design.md package layout,
// "ingestion/ # APPLICATION: IngestSeries, ReconcileEditorialConfig,
// RunSyntheticProbe"). IngestSeries is the first of those three built --
// it composes a driven source adapter (ine or eurostat, both satisfying
// indicators.SourceClient), filestore, postgres, and the pure validation
// package into the whole ingestion pipeline (design.md "Ingest data
// flow"): fetch, archive, decode, validate, publish gate. Slice 6 (task
// 6.5) generalised IngestSeries from INE-only to source-agnostic, per
// spec source-ingestion-eurostat's "reuse the same domain types,
// observation writer and validation harness as the INE adapter; only
// the adapter differs".
//
// ReconcileEditorialConfig and RunSyntheticProbe are listed here by
// design.md as SEPARATE functions and are NOT built in this batch:
// IngestSeries assumes the dimension rows (source/dataset/series/
// series_source_mapping) it references already exist. Reconciling
// config/series/*.yaml into those rows is ReconcileEditorialConfig's
// job.
package ingestion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/pipelinelog"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// SeriesIngestConfig is the pre-resolved input IngestSeries needs to run
// one series through the full pipeline, regardless of which source it
// comes from: the already-persisted dimension identity (SourceID,
// DatasetID, SeriesID) plus the resolved active series origin reference
// (an INE COD or a Eurostat dataset code -- COD's name is kept from its
// original INE-only shape, see the field's own comment) and every
// validation threshold from series/{slug}.yaml (config.ValidationConfig/
// config.SchemaConfig, reused verbatim -- the same "no parallel config
// model" decision validation.SeriesContext already made in PR 4a).
//
// Renamed from INESeriesIngestConfig in slice 6 (task 6.5, spec
// source-ingestion-eurostat "reuse the same domain types, observation
// writer and validation harness as the INE adapter"): the type itself
// was already source-agnostic in shape, only its INE-flavoured name was
// not.
type SeriesIngestConfig struct {
	SourceID  string
	DatasetID string
	SeriesID  string // = slug

	// COD is the resolved active series origin reference: an INE COD
	// (adapters/ine) or a Eurostat dataset code (adapters/eurostat). The
	// field keeps its original INE-only name across the slice 6 rename
	// (see the type's own doc comment) to minimise the diff against
	// every existing call site; it means the same thing indicators.
	// SourceClient's ref parameter means everywhere else.
	COD string

	Series     indicators.Series
	Validation config.ValidationConfig
	Schema     config.SchemaConfig

	// HashListingArchivePath/HashListingPublicPath name where the public
	// raw-file hash listing (spec raw-file-archive, "Raw-file hashes are
	// listed in the repository"; PRD §14.2) is refreshed after archiving
	// this run's raw payload -- postgres.PublishRawFileHashListing's own
	// archivePath/publicPath. Both left at their zero value (every
	// existing caller before this remediation batch) means "do not
	// publish", so no existing call site changes behaviour; only the
	// production orchestrator (app/cmd/concontexto's ingest subcommand)
	// sets them (verify-report CRITICAL C3: PublishRawFileHashListing had
	// no production call site before this).
	HashListingArchivePath string
	HashListingPublicPath  string
}

// Result is IngestSeries's full outcome: the publish gate's verdict
// (embedded postgres.GateApplyResult, which itself embeds
// validation.GateResult) plus the ingestion_run id it recorded against.
type Result struct {
	postgres.GateApplyResult
	RunID int64
}

// sixRules is every applicable validation rule (spec data-validation),
// run in the same fixed order every time so Gate's "reports every
// failure, not only the first" is deterministic across runs.
var sixRules = []validation.Rule{
	validation.Rule1Schema,
	validation.Rule2Continuity,
	validation.Rule3Plausibility,
	validation.Rule4Revision,
	validation.Rule5MetadataCompleteness,
	validation.Rule6NonEmpty,
}

// IngestSeries runs the whole ingestion pipeline for one series,
// regardless of its source (design.md "Ingest data flow"; spec
// source-ingestion-eurostat "reuse the same domain types, observation
// writer and validation harness as the INE adapter; only the adapter
// differs"): fetch raw bytes through client (an indicators.SourceClient
// -- *ine.Client or *eurostat.Client, chosen by the caller), archive
// them BEFORE any parsing happens (raw_file/download_attempt persist
// even if decode or validation later fails -- spec raw-file-archive's
// whole audit-trail guarantee), decode+normalize, run every applicable
// validation rule, and let the publish gate decide whether anything
// becomes current.
//
// On a FETCH failure (the source refused, or every retry was exhausted),
// the failure is classified and recorded as a download_attempt with no
// resulting hash, and IngestSeries returns that classified error --
// nothing else is written, because there were no raw bytes to archive
// yet (no ingestion_run row either: CreateIngestionRun always requires an
// already-archived raw_file_hash).
//
// On a DECODE failure (periodicity mismatch, malformed body) the raw
// file and its ingestion_run row already persist -- the failure is
// recorded through the exact same publish-gate Block path a validation
// failure takes (a synthetic all-blocking finding), so decode and
// validation failures converge on one "record failed, write nothing"
// effect instead of two.
//
// Breaks (editorial rupturas.yaml exemptions, rule 3) are resolved from
// postgres via resolveBreaks -- ReconcileEditorialConfig (slice 7)
// projects rupturas.yaml into series_break ahead of any ingest run, and
// postgres.ResolveActiveBreaksForSeries expands scope (series ⊂ dataset ⊂
// source) into this exact series' already-widened, already period-keyed
// break list (indicators.Break's own doc comment). A series with no
// active break resolves to an empty (nil-safe) slice, so an
// un-reconciled or break-free series behaves exactly as it did before
// this was wired.
// Task 2a.8/2b (GREEN): TipoDato (Provisional/Definitivo) and Eurostat
// flag status mapping. ine.Client.Decode (design D-3;
// adapters/ine/tipodato_test.go) and eurostat.Decode
// (adapters/eurostat/envelope_test.go, task 2b) both now classify every
// observation's status fail-closed before IngestSeries ever sees it -- a
// source-decoded observation reaching the candidate-building loop below
// therefore always carries a valid, already-classified Status.
// mapObservationStatus/sourceStatusPtr below do the trivial remaining
// conversion into postgres's own types. Slice 2a's disclosed
// compatibility fallback (defaulting a never-classified Status to
// Definitive, scoped to Eurostat's then-not-yet-built decoding) is
// CLOSED by this slice: firstUnclassifiedStatus, below, now fails the
// whole run closed instead -- see mapObservationStatus's own doc comment
// for why removing the default, rather than only guarding it, is safe.
func IngestSeries(ctx context.Context, db postgres.TxBeginner, store *filestore.Store, client indicators.SourceClient, cfg SeriesIngestConfig, now time.Time) (Result, error) {
	// startedAt is a real wall-clock read solely to measure this run's
	// own log Duration (task 9.5/9.6) -- distinct from `now`, which
	// stays the one explicit parameter every DECISION in this function
	// (archiving timestamps, extraction timestamps) is made against. A
	// duration metric has no decision to make deterministic; nothing
	// above this line reads the clock.
	startedAt := time.Now()

	seriesURL := client.RequestURL(cfg.COD)

	raw, fetchErr := client.FetchRaw(ctx, cfg.COD)
	if fetchErr != nil {
		if _, err := postgres.RecordDownloadAttempt(ctx, db, postgres.DownloadAttempt{
			SourceID: cfg.SourceID, URL: seriesURL, AttemptedAt: now,
			Outcome: classifyDownloadOutcome(fetchErr),
		}); err != nil {
			return Result{}, fmt.Errorf("ingestion: recording failed download attempt for %s: %w", cfg.SeriesID, err)
		}
		return Result{}, fmt.Errorf("ingestion: fetching %s: %w", cfg.SeriesID, fetchErr)
	}

	archived, isNew, err := postgres.ArchiveRawFile(ctx, db, store, cfg.SourceID, seriesURL, raw, now)
	if err != nil {
		return Result{}, fmt.Errorf("ingestion: archiving raw payload for %s: %w", cfg.SeriesID, err)
	}
	downloadOutcome := postgres.OutcomeNewFile
	if !isNew {
		downloadOutcome = postgres.OutcomeUnchanged
	}
	resultingHash := archived.Hash
	if _, err := postgres.RecordDownloadAttempt(ctx, db, postgres.DownloadAttempt{
		SourceID: cfg.SourceID, URL: seriesURL, AttemptedAt: now,
		ResultingHash: &resultingHash, Outcome: downloadOutcome,
	}); err != nil {
		return Result{}, fmt.Errorf("ingestion: recording download attempt for %s: %w", cfg.SeriesID, err)
	}

	// Refresh the public hash listing right after archiving (verify-
	// report CRITICAL C3): design.md's sequence diagram places this step
	// alongside the raw-file archive itself, not after validation --
	// every archived file, published or later blocked, belongs in the
	// listing (PRD §14.2's traceability claim covers the whole audit
	// trail, not only published runs). Both paths empty (the default for
	// every caller that does not opt in) skips publication entirely.
	if cfg.HashListingArchivePath != "" && cfg.HashListingPublicPath != "" {
		if err := postgres.PublishRawFileHashListing(ctx, db, cfg.HashListingArchivePath, cfg.HashListingPublicPath); err != nil {
			return Result{}, fmt.Errorf("ingestion: publishing the raw-file hash listing for %s: %w", cfg.SeriesID, err)
		}
	}

	runID, err := postgres.CreateIngestionRun(ctx, db, cfg.DatasetID, cfg.SeriesID, now, archived.Hash)
	if err != nil {
		return Result{}, fmt.Errorf("ingestion: creating ingestion_run for %s: %w", cfg.SeriesID, err)
	}

	decoded, decodeErr := client.Decode(raw, cfg.COD, cfg.Series.Frequency, cfg.Series.CadenceSegments...)
	if decodeErr != nil {
		findings := []validation.Finding{{
			Rule: "source-decode", Severity: validation.SeverityBlock, Message: decodeErr.Error(),
		}}
		result, gateErr := postgres.ApplyGate(ctx, db, runID, findings, nil)
		if gateErr != nil {
			return Result{}, fmt.Errorf("ingestion: recording decode failure for %s: %w", cfg.SeriesID, gateErr)
		}
		logAndAlertRun(ctx, cfg, runID, archived.Hash, result, findings, startedAt)
		return Result{GateApplyResult: result, RunID: runID}, fmt.Errorf("ingestion: decoding %s: %w", cfg.SeriesID, decodeErr)
	}

	incoming := decoded.Observations

	// Task 2b (closes slice 2a's disclosed compatibility shim): both
	// adapters this change governs now classify EVERY observation's
	// status before IngestSeries ever sees it (ine.Client.Decode's
	// classifyTipoDato; eurostat.Decode's own flag classification,
	// envelope.go). An observation reaching this point with a still-
	// unclassified (zero-value) Status can therefore only come from a
	// SourceClient that does not classify status at all -- design D-3's
	// mapping table has no row for such a source (adapters/xlsx today,
	// a disclosed, pre-existing, out-of-scope gap; see apply-progress).
	// Per D-3's own governing principle ("Coercing unknown tokens to D
	// ... is exactly the silent lie this change exists to remove"), that
	// case now fails the WHOLE run closed -- through the same
	// synthetic-Block-finding path a decode failure already takes, so it
	// converges on the file's own "record failed, write nothing" effect
	// -- rather than silently defaulting to Definitive.
	// mapObservationStatus below no longer has a default branch to fall
	// into, because this guard makes that branch structurally
	// unreachable past this point.
	if period, found := firstUnclassifiedStatus(incoming); found {
		findings := []validation.Finding{{
			Rule: "source-status", Severity: validation.SeverityBlock, Period: period.String(),
			Message: fmt.Sprintf("%s: observation at %s reached the candidate loop with no classified status (internal defect, not a source failure)", cfg.SeriesID, period),
		}}
		result, gateErr := postgres.ApplyGate(ctx, db, runID, findings, nil)
		if gateErr != nil {
			return Result{}, fmt.Errorf("ingestion: recording unclassified-status failure for %s: %w", cfg.SeriesID, gateErr)
		}
		logAndAlertRun(ctx, cfg, runID, archived.Hash, result, findings, startedAt)
		return Result{GateApplyResult: result, RunID: runID}, fmt.Errorf("ingestion: %s: observation at %s reached the candidate loop with no classified status", cfg.SeriesID, period)
	}

	prior, err := postgres.ListCurrentObservations(ctx, db, cfg.SeriesID)
	if err != nil {
		return Result{}, fmt.Errorf("ingestion: reading prior vintage for %s: %w", cfg.SeriesID, err)
	}

	breaks, err := resolveBreaks(ctx, db, cfg.SeriesID, cfg.Series.Frequency)
	if err != nil {
		return Result{}, fmt.Errorf("ingestion: resolving active breaks for %s: %w", cfg.SeriesID, err)
	}

	// Task 2b.6 (GREEN): a decoded break/definition-differs flag (design
	// D-3, Eurostat's b/d, envelope.go's BreakSignals) whose period no
	// already-active series_break covers yet is logged and alerted --
	// never blocked, never written as series_break itself (rupturas.yaml
	// remains the only writer, editorial follow-up).
	alertUncoveredBreakSignals(ctx, cfg, decoded.BreakSignals, breaks)

	seriesCtx := validation.SeriesContext{
		Series:         cfg.Series,
		Validation:     cfg.Validation,
		Schema:         cfg.Schema,
		ObservedSchema: decoded.ObservedSchema, // slice 8: populated by the XLSX adapter, zero value for INE/Eurostat
		Prior:          prior,
		Breaks:         breaks,
	}

	var findings []validation.Finding
	for _, rule := range sixRules {
		findings = append(findings, rule(seriesCtx, incoming)...)
	}

	candidates := make([]postgres.ObservationInput, 0, len(incoming))
	for _, o := range incoming {
		candidates = append(candidates, postgres.ObservationInput{
			SeriesID:       cfg.SeriesID,
			Period:         o.Period.String(),
			Value:          o.Value,
			Status:         mapObservationStatus(o.Status),
			SourceStatus:   sourceStatusPtr(o.SourceStatus),
			ExtractedAt:    now,
			IngestionRunID: runID,
		})
	}

	result, err := postgres.ApplyGate(ctx, db, runID, findings, candidates)
	if err != nil {
		return Result{}, fmt.Errorf("ingestion: applying the publish gate for %s: %w", cfg.SeriesID, err)
	}
	logAndAlertRun(ctx, cfg, runID, archived.Hash, result, findings, startedAt)
	return Result{GateApplyResult: result, RunID: runID}, nil
}

// mapObservationStatus converts the shared domain classification
// (indicators.ObservationStatus) into postgres's own status enum. The
// two types carry an identical P/D/W value set by design (indicators
// never imports postgres -- see indicators/observation.go's own doc
// comment), so this is a direct conversion.
//
// Slice 2a's disclosed compatibility shim -- defaulting a zero-value
// (never-classified) Status to postgres.StatusDefinitive, scoped to
// adapters/eurostat's then-not-yet-built status decoding -- is REMOVED
// here (task 2b, this slice's own closing item). It is safe to remove,
// not merely hidden, because firstUnclassifiedStatus already rejects the
// whole run (IngestSeries, above) before any candidate reaches this
// function: mapObservationStatus can now assume status is always
// classified, exactly like sourceStatusPtr below already assumes a
// non-empty token means something and an empty one means "none
// recorded".
func mapObservationStatus(status indicators.ObservationStatus) postgres.ObservationStatus {
	return postgres.ObservationStatus(status)
}

// firstUnclassifiedStatus returns the period of the first observation in
// incoming whose Status is still the domain zero value, and true, or the
// zero Period and false when every observation already carries a
// classified status (design D-3; see IngestSeries's own doc comment
// above for the full rationale).
func firstUnclassifiedStatus(incoming []indicators.Observation) (indicators.Period, bool) {
	for _, o := range incoming {
		if o.Status == "" {
			return o.Period, true
		}
	}
	return indicators.Period{}, false
}

// alertUncoveredBreakSignals logs and raises an alerting.
// KindBreakSignalUncovered alert (task 2b.6) for every signal in signals
// whose Period no break in breaks already covers. It never blocks
// publication and never writes series_break itself -- purely an
// operational disclosure that the metadata linkage (rupturas.yaml, the
// only writer) is still missing for that period.
func alertUncoveredBreakSignals(ctx context.Context, cfg SeriesIngestConfig, signals []indicators.BreakSignal, breaks []indicators.Break) {
	if len(signals) == 0 {
		return
	}
	covered := make(map[string]bool, len(breaks))
	for _, b := range breaks {
		covered[b.Period.String()] = true
	}
	for _, s := range signals {
		if covered[s.Period.String()] {
			continue
		}
		slog.Default().LogAttrs(ctx, slog.LevelWarn, "uncovered source break/definition signal",
			slog.String("source", cfg.SourceID), slog.String("series", cfg.SeriesID),
			slog.String("period", s.Period.String()), slog.String("flag", s.Flag))
		_ = alerting.BreakSignalUncovered(ctx, alerting.DefaultSink(), cfg.SourceID, cfg.SeriesID, s.Period.String(), s.Flag)
	}
}

// sourceStatusPtr converts the domain's empty-string-means-none
// convention into source_status's NULL-means-none column convention
// (migration 0003; spec data-model-vintages, "A null token is valid and
// means definitive for Eurostat").
func sourceStatusPtr(token string) *string {
	if token == "" {
		return nil
	}
	return &token
}

// logAndAlertRun emits this run's structured log line (spec pipeline-
// operations, "Structured pipeline logging": task 9.5/9.6) and, on a
// Block outcome, raises a validation-failed alert naming source, series
// and every failing rule (task 9.7/9.8's own scenario). It is called
// from every path that reaches a real ApplyGate call -- both the decode-
// failure synthetic-finding branch and the ordinary post-validation
// branch -- so "a completed run" and "a failed run" converge on one
// logging/alerting call, exactly like they already converge on one
// ApplyGate call (this file's own package doc comment).
//
// Both the structured logger (slog.Default()) and the alert sink
// (alerting.DefaultSink()) are process-wide defaults, deliberately NOT
// added as new IngestSeries parameters: every existing call site of this
// already-widely-used exported function (this package's own tests, a
// future scheduler daemon) keeps compiling unchanged, and a caller that
// wants different behaviour swaps the default (slog.SetDefault,
// alerting.SetDefaultSink) exactly like this batch's own tests do.
func logAndAlertRun(ctx context.Context, cfg SeriesIngestConfig, runID int64, rawFileHash string, result postgres.GateApplyResult, findings []validation.Finding, startedAt time.Time) {
	failedRules := pipelinelog.FailedRules(findings)

	level := slog.LevelInfo
	if result.Outcome == validation.GateBlock {
		level = slog.LevelWarn
	}
	slog.Default().LogAttrs(ctx, level, "ingestion run completed", pipelinelog.Attrs(pipelinelog.Entry{
		RunID: runID, Source: cfg.SourceID, Dataset: cfg.DatasetID, Series: cfg.SeriesID,
		Outcome: string(result.Outcome), RawFileHash: rawFileHash, Duration: time.Since(startedAt),
		Verdicts: pipelinelog.Verdicts(findings), FailedRules: failedRules,
	})...)

	if result.Outcome == validation.GateBlock {
		// Best-effort: a failed alert delivery must never fail (or
		// re-fail) an already-decided ingestion run.
		_ = alerting.ValidationFailed(ctx, alerting.DefaultSink(), cfg.SourceID, cfg.SeriesID, failedRules)
	}
}

// classifyDownloadOutcome mirrors a fetch failure's sourceerr.FailureClass
// onto postgres.DownloadOutcome -- a direct 1:1 map, since sourceerr.go's
// own doc comment already states the taxonomy mirrors DownloadOutcome's
// value set exactly. An error that is not a classified *sourceerr.Error
// at all (should not happen -- every ine.Client failure path returns one)
// falls back to RetryableTransport, the most conservative classification.
// resolveBreaks resolves seriesID's active postgres.SeriesBreak rows
// (scope already expanded series ⊂ dataset ⊂ source by
// postgres.ResolveActiveBreaksForSeries) and converts each into the
// already period-keyed indicators.Break rule 3 consumes (verify-report
// CRITICAL C1: production never called this before, so a genuine
// methodology break was reported as an implausible jump exactly like an
// un-exempted one). db satisfies postgres.DBTX (TxBeginner embeds it), so
// this runs inside IngestSeries's own db handle without opening a second
// transaction.
func resolveBreaks(ctx context.Context, db postgres.DBTX, seriesID string, freq indicators.Frequency) ([]indicators.Break, error) {
	resolved, err := postgres.ResolveActiveBreaksForSeries(ctx, db, seriesID)
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, nil
	}
	breaks := make([]indicators.Break, 0, len(resolved))
	for _, sb := range resolved {
		breaks = append(breaks, indicators.Break{
			ID:     sb.BreakKey,
			Period: indicators.PeriodFromDate(sb.Date, freq),
			Kind:   sb.Kind,
			Note:   sb.NoteMD,
		})
	}
	return breaks, nil
}

func classifyDownloadOutcome(err error) postgres.DownloadOutcome {
	var classified *sourceerr.Error
	if errors.As(err, &classified) {
		switch classified.Class {
		case sourceerr.RetryableTransport:
			return postgres.OutcomeRetryableTransport
		case sourceerr.SourceRefusal:
			return postgres.OutcomeSourceRefusal
		case sourceerr.SilentEmpty:
			return postgres.OutcomeSilentEmpty
		case sourceerr.SchemaDrift:
			return postgres.OutcomeSchemaDrift
		case sourceerr.ResponseTooLarge:
			return postgres.OutcomeResponseTooLarge
		}
	}
	return postgres.OutcomeRetryableTransport
}
