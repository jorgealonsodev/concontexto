package main

// Task 7.15 + remediation batch (verify-report CRITICAL C2): wire both
// ingestion.ReconcileEditorialConfig ("ingest --reconcile") and
// ingestion.IngestSeries ("ingest --series=<slug>" / "ingest
// --source=<source-id>") into the `ingest` subcommand. Before this
// batch, IngestSeries had ten call sites, all _test.go -- the binary
// could not ingest a single configured series in production; every
// invocation other than "--reconcile" kept PR 1a's placeholder.
//
// runIngestReconcile/runIngest are the testable cores (mirrors
// runValidateConfig's pattern in validate_config_cmd.go / runServe's in
// serve.go): they take an already-open db, an already-loaded
// *config.Config and (runIngest) an already-constructed *filestore.Store
// instead of resolving DATABASE_URL / the embedded tree / /app_data
// themselves, so tests can inject all of them.

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

// runIngestReconcile reconciles cfg's editorial YAML into
// series_break/event via ingestion.ReconcileEditorialConfig and reports
// the resulting counts plus any still-pending (unconfirmed-date) entries
// on stdout.
func runIngestReconcile(ctx context.Context, db postgres.TxBeginner, cfg *config.Config, stdout, stderr io.Writer) int {
	result, err := ingestion.ReconcileEditorialConfig(ctx, db, *cfg)
	if err != nil {
		fmt.Fprintln(stderr, "ingest --reconcile:", err)
		return 1
	}
	fmt.Fprintf(stdout, "ingest --reconcile: breaks inserted=%d updated=%d retired=%d; events inserted=%d updated=%d retired=%d\n",
		result.Breaks.Inserted, result.Breaks.Updated, result.Breaks.Retired,
		result.Events.Inserted, result.Events.Updated, result.Events.Retired)
	if len(result.PendingBreakIDs) > 0 {
		fmt.Fprintf(stdout, "ingest --reconcile: pending (unconfirmed date) breaks: %s\n", strings.Join(result.PendingBreakIDs, ", "))
	}
	if len(result.PendingEventIDs) > 0 {
		fmt.Fprintf(stdout, "ingest --reconcile: pending (unconfirmed date) events: %s\n", strings.Join(result.PendingEventIDs, ", "))
	}
	return 0
}

// appDataRoot returns the directory raw files and the archive-side hash
// listing live under -- fixed at /app_data inside the container
// (docker-compose.yml's own app_data volume mount), overridable for a
// source checkout run outside Docker, mirroring serve.go's
// staticAssetRoot/STATIC_ROOT convention exactly.
func appDataRoot() string {
	if root := os.Getenv("APP_DATA_ROOT"); root != "" {
		return root
	}
	return "/app_data"
}

// cmdIngest dispatches "ingest --reconcile", "ingest --series=<slug>" and
// "ingest --source=<source-id>" to the real embedded config and
// DATABASE_URL. Exactly one target flag is required; a bare `ingest`
// (or any other unrecognised invocation) prints usage naming all three
// and exits non-zero -- there is no longer a "not yet implemented"
// placeholder to fall back to (verify-report CRITICAL C2).
func cmdIngest(args []string, stdout, stderr io.Writer) int {
	var seriesFlag, sourceFlag string
	reconcile := false
	for _, a := range args {
		switch {
		case a == "--reconcile":
			reconcile = true
		case strings.HasPrefix(a, "--series="):
			seriesFlag = strings.TrimPrefix(a, "--series=")
		case strings.HasPrefix(a, "--source="):
			sourceFlag = strings.TrimPrefix(a, "--source=")
		}
	}
	if !reconcile && seriesFlag == "" && sourceFlag == "" {
		fmt.Fprintln(stderr, "ingest: usage: ingest --reconcile | --series=<slug> | --source=<source-id>")
		return 1
	}

	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(stderr, "ingest: DATABASE_URL is not set")
		return 1
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(stderr, "ingest:", err)
		return 1
	}
	defer pool.Close()

	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		fmt.Fprintln(stderr, "ingest:", err)
		return 1
	}
	cfg, err := config.Load(sub)
	if err != nil {
		fmt.Fprintln(stderr, "ingest:", err)
		return 1
	}

	if reconcile {
		return runIngestReconcile(ctx, pool, cfg, stdout, stderr)
	}

	return cmdIngestRun(ctx, pool, cfg, seriesFlag, sourceFlag, stdout, stderr)
}

// resolveIngestPaths computes the archive-side and public hash-listing
// paths from root and staticRoot -- the exact formula both cmdIngestRun
// and startSchedulerLoop (schedule.go) need. Extracted so a test can pin
// this resolution directly and so cmdIngestRun's own call site can be
// exercised end to end (verify-report CRITICAL C8/M7): before this
// extraction, cmdIngest's own publicHashPath line (ingest_cmd.go:130 in
// the pre-remediation source) was never executed by any test -- setting
// it to "" silently disables PRD §14.2's public hash-listing publication,
// gated by ingest.go's own "both paths non-empty" check, with the entire
// suite green.
func resolveIngestPaths(root, staticRoot string) (archiveHashPath, publicHashPath string) {
	archiveHashPath = filepath.Join(root, "raw_files.sha256")
	publicHashPath = filepath.Join(staticRoot, "transparencia", "raw-files.sha256")
	return archiveHashPath, publicHashPath
}

// cmdIngestRun is cmdIngest's non-reconcile composition core, extracted
// for testability (verify-report CRITICAL C8/M7): it takes an already-
// connected db and an already-loaded cfg instead of resolving
// DATABASE_URL/the embedded config tree itself, mirroring
// runIngestReconcile's already-established pattern in this same file, so
// a test can pin cmdIngest's own hash-listing path resolution end to end
// -- through IngestSeries' real publish gate (ingest.go:192) -- without a
// live DATABASE_URL.
func cmdIngestRun(ctx context.Context, db postgres.TxBeginner, cfg *config.Config, seriesFlag, sourceFlag string, stdout, stderr io.Writer) int {
	root := appDataRoot()
	store := filestore.NewStore(filepath.Join(root, "raw"))
	archiveHashPath, publicHashPath := resolveIngestPaths(root, staticAssetRoot())
	return runIngest(ctx, db, cfg, store, archiveHashPath, publicHashPath, time.Now().UTC(), seriesFlag, sourceFlag,
		exportOutputDir(false), buildDispatcher(), stdout, stderr)
}

// buildDispatcher, which resolves the rebuild-trigger adapter this
// function passes to runIngest, now lives in rebuild_dispatch.go together
// with the APP_REBUILD_DISPATCH mode that decides whether an unconfigured
// dispatch is a deliberate opt-out or a fault (verify-report CRITICAL-28,
// link 2).
//
// retentionHistoryDir resolves where publishing.Publish archives each
// export snapshot for rollback (task 4.9/4.10, design's own
// Migration/Rollout note: "last N artifacts retained"). Deliberately a
// sibling of APP_DATA_ROOT's own raw-download archive -- never under
// STATIC_ROOT -- so a retained (possibly since-corrected) snapshot is
// never itself publicly served the way the live outDir is: the whole
// point of "rollback" is recovering from a bad publish, which would be
// defeated if the bad snapshot stayed reachable at its own URL the entire
// time it was retained.
func retentionHistoryDir() string {
	return filepath.Join(appDataRoot(), "data-derived-history")
}

// retainedArtifacts resolves APP_PUBLISH_RETAIN_ARTIFACTS, falling back to
// (and flooring at) publishing.DefaultRetainedArtifacts when unset,
// unparsable or below the floor -- mirrors scheduleInterval's own
// resilience convention (spec/design: "last N artifacts retained, N
// configurable >= 5"). An invalid or sub-floor override is logged, never
// fatal -- the same resilience choice this package already makes for
// APP_SCHEDULE_INTERVAL.
func retainedArtifacts(logs io.Writer) int {
	raw := os.Getenv("APP_PUBLISH_RETAIN_ARTIFACTS")
	if raw == "" {
		return publishing.DefaultRetainedArtifacts
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < publishing.DefaultRetainedArtifacts {
		fmt.Fprintf(logs, "ingest: publish: APP_PUBLISH_RETAIN_ARTIFACTS=%q is invalid or below the %d-artifact floor, using default %d\n",
			raw, publishing.DefaultRetainedArtifacts, publishing.DefaultRetainedArtifacts)
		return publishing.DefaultRetainedArtifacts
	}
	return n
}

// buildSourceClient resolves the indicators.SourceClient ref's kind
// requires -- *ine.Client, *eurostat.Client or *xlsx.Client, all three
// satisfying the same port (design.md/spec source-ingestion-eurostat,
// "reuse the same domain types ... only the adapter differs"). httpClient
// is left nil: every adapter's own NewClient now defaults to a bounded
// Timeout (verify-report WARNING W6), so no caller here needs to build
// one by hand.
//
// Remediation batch (verify-report WARNING W12, the sixth instance of
// "parsed and ignored" this change has produced): every source YAML sets
// api.max_response_bytes, and config.APIConfig.MaxResponseBytes was
// schema-validated -- but before this batch nothing here ever read it,
// so every adapter silently fell back to its own hardcoded
// defaultMaxResponseBytes constant regardless of what config declared.
// eurostat and xlsx both already expose WithMaxResponseBytes; when
// src.API.MaxResponseBytes is set (>0) it now overrides their default.
//
// Remediation batch 4 closes the gap disclosed by the batch above:
// adapters/ine now carries the identical WithMaxResponseBytes option
// (previously it had none at all -- doRequest read the full body
// unbounded despite ine.yaml declaring the same max_response_bytes
// key), and this switch wires it exactly like eurostat and xlsx below.
func buildSourceClient(src config.SourceConfig, s config.SeriesConfig, ref config.SourceRef) (indicators.SourceClient, error) {
	switch ref.Kind {
	case "ine-series-cod":
		if src.API == nil {
			return nil, fmt.Errorf("source %q has no api configuration", src.ID)
		}
		var opts []ine.Option
		if src.API.MaxResponseBytes > 0 {
			opts = append(opts, ine.WithMaxResponseBytes(src.API.MaxResponseBytes))
		}
		return ine.NewClient(src.API.BaseURL, nil, opts...), nil
	case "eurostat-dataset":
		if src.API == nil {
			return nil, fmt.Errorf("source %q has no api configuration", src.ID)
		}
		var opts []eurostat.Option
		if src.API.MaxResponseBytes > 0 {
			opts = append(opts, eurostat.WithMaxResponseBytes(src.API.MaxResponseBytes))
		}
		return eurostat.NewClient(src.API.BaseURL, ref.Filters, nil, opts...), nil
	case "xlsx-url":
		if s.Schema.XLSX == nil {
			return nil, fmt.Errorf("series %q has no schema.xlsx configuration", s.Slug)
		}
		var opts []xlsx.Option
		if src.API != nil && src.API.MaxResponseBytes > 0 {
			opts = append(opts, xlsx.WithMaxResponseBytes(src.API.MaxResponseBytes))
		}
		return xlsx.NewClient(*s.Schema.XLSX, nil, opts...), nil
	default:
		return nil, fmt.Errorf("series %q: unsupported source_ref kind %q", s.Slug, ref.Kind)
	}
}

// seriesCadenceSegments converts s.CadenceSegments (config's plain-string
// YAML shape) into the domain indicators.CadenceSegment values IngestSeries'
// SourceClient.Decode and validation.Rule2Continuity both consume (design
// D-4, task 1.2/1.6). An empty s.CadenceSegments returns nil, the ordinary
// uniform-cadence case for five of the six milestone-0.2 series.
func seriesCadenceSegments(s config.SeriesConfig) ([]indicators.CadenceSegment, error) {
	if len(s.CadenceSegments) == 0 {
		return nil, nil
	}
	out := make([]indicators.CadenceSegment, 0, len(s.CadenceSegments))
	for _, seg := range s.CadenceSegments {
		parsed, err := indicators.ParseCadenceSegment(seg.From, seg.To, seg.Cadence, seg.Present)
		if err != nil {
			return nil, fmt.Errorf("series %q: cadence_segments: %w", s.Slug, err)
		}
		out = append(out, parsed)
	}
	return out, nil
}

// exportReason names, on stdout, WHY a cycle exported -- an operator
// reading a log needs to tell "we published new data" from "we published
// the fact that a source's latest datum was rejected", because the second
// is an editorial follow-up waiting to happen and the first is not. Both
// legitimately export (see the export gate in runIngest); silently
// conflating them in the log would hide a validation failure behind a line
// that reads like an ordinary successful publication.
func exportReason(published, failedValidation bool) string {
	switch {
	case published && failedValidation:
		return "some series published, another failed validation"
	case failedValidation:
		return "no series published; exporting the recorded validation failure so the page state reaches readers"
	default:
		return "at least one series published"
	}
}

// runIngest resolves seriesFlag/sourceFlag (exactly one non-empty,
// enforced by cmdIngest) against cfg, reconciles the dimension rows every
// target needs to exist first (postgres.ReconcileDimensions -- see that
// file's own doc comment for why this was never wired before), then runs
// ingestion.IngestSeries for each resolved series, continuing past a
// per-series failure so one broken source_ref cannot block every other
// series in the same "--source" run (spec pipeline-operations, "one
// scheduled job per source" implies one series' failure is that series'
// own, not the whole batch's).
// outDir/dispatch are new in slice 4 (design D-2: "the command layer
// ... calls Publish once after an ingest cycle in which at least one
// series published"). Every existing test call site passes ""/nil,
// preserving prior behaviour exactly -- Publish/Export are simply never
// reached when outDir is empty (see the trailing block below), matching
// this codebase's established "not configured, not attempted" pattern.
//
// D-2's "at least one series published" condition was WIDENED by the
// CRITICAL-16 remediation to "at least one series recorded a terminal run
// outcome the artifact must now reflect" -- see exportWorthy below and
// design.md's D-2 resolution for the full reasoning.
func runIngest(ctx context.Context, db postgres.TxBeginner, cfg *config.Config, store *filestore.Store,
	archiveHashPath, publicHashPath string, now time.Time, seriesFlag, sourceFlag string,
	outDir string, dispatch publishing.Dispatcher, stdout, stderr io.Writer) int {

	var targets []config.SeriesConfig
	switch {
	case seriesFlag != "":
		found := false
		for _, s := range cfg.Series {
			if s.Slug == seriesFlag {
				targets = append(targets, s)
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(stderr, "ingest --series=%s: no configured series with that slug\n", seriesFlag)
			return 1
		}
	case sourceFlag != "":
		if _, ok := cfg.Sources[sourceFlag]; !ok {
			fmt.Fprintf(stderr, "ingest --source=%s: no configured source with that id\n", sourceFlag)
			return 1
		}
		for _, s := range cfg.Series {
			if s.Source == sourceFlag {
				targets = append(targets, s)
			}
		}
		if len(targets) == 0 {
			fmt.Fprintf(stderr, "ingest --source=%s: no configured series reference this source\n", sourceFlag)
			return 1
		}
	}

	if err := postgres.ReconcileDimensions(ctx, db, cfg, now); err != nil {
		fmt.Fprintln(stderr, "ingest: reconciling dimensions:", err)
		return 1
	}

	failed := 0
	published := false
	failedValidation := false
	for _, s := range targets {
		src, ok := cfg.Sources[s.Source]
		if !ok {
			fmt.Fprintf(stderr, "ingest: series %s references unknown source %s\n", s.Slug, s.Source)
			failed++
			continue
		}
		ref, ok := postgres.ActiveSourceRef(s)
		if !ok {
			fmt.Fprintf(stderr, "ingest: series %s has no active source_ref\n", s.Slug)
			failed++
			continue
		}
		client, err := buildSourceClient(src, s, ref)
		if err != nil {
			fmt.Fprintf(stderr, "ingest: series %s: %v\n", s.Slug, err)
			failed++
			continue
		}

		cadenceSegments, err := seriesCadenceSegments(s)
		if err != nil {
			fmt.Fprintf(stderr, "ingest: series %s: %v\n", s.Slug, err)
			failed++
			continue
		}

		icfg := ingestion.SeriesIngestConfig{
			SourceID: s.Source, DatasetID: s.Dataset, SeriesID: s.Slug, COD: ref.Ref,
			Series: indicators.Series{
				Slug: s.Slug, Unit: s.Unit, Frequency: indicators.Frequency(s.Frequency), Decimals: s.Decimals,
				Source: s.Source, Licence: src.Licence.Name, CadenceSegments: cadenceSegments,
			},
			Validation:             s.Validation,
			Schema:                 s.Schema,
			HashListingArchivePath: archiveHashPath,
			HashListingPublicPath:  publicHashPath,
		}

		result, err := ingestion.IngestSeries(ctx, db, store, client, icfg, now)

		// The result is read BEFORE the error is handled, and that ordering
		// is load-bearing (verify-report CRITICAL-16). A returned error does
		// NOT mean nothing was recorded: IngestSeries's decode-failure and
		// unclassified-status paths return a real Block result AND an error,
		// because both already went through the publish gate and left an
		// ingestion_run row reading 'validation-failed' (see that function's
		// own doc comment: "decode and validation failures converge on one
		// 'record failed, write nothing' effect instead of two"). A FETCH
		// failure, by contrast, returns the ZERO Result -- no run row exists
		// to reflect, because there were never any bytes to validate -- and
		// so does every infrastructure failure below the gate. Reading
		// result.Outcome is therefore the exact discriminator between "a
		// verdict was recorded" and "nothing happened", and it works without
		// inspecting the error at all.
		if result.Outcome == validation.GateBlock {
			failedValidation = true
		}
		if len(result.Published) > 0 {
			published = true
		}

		if err != nil {
			fmt.Fprintf(stderr, "ingest: series %s: %v\n", s.Slug, err)
			failed++
			continue
		}
		fmt.Fprintf(stdout, "ingest: series=%s outcome=%s published=%d run_id=%d\n", s.Slug, result.Outcome, len(result.Published), result.RunID)
	}

	// THE EXPORT GATE. Design D-2's original condition was "at least one
	// series published (GateApplyResult.Published non-empty)", which
	// implemented spec publishing-export's then-current "THEN no new
	// artifact is exported" on a failed run. That clause has been NARROWED
	// (verify-report CRITICAL-16; see publishing-export/spec.md's amended
	// "A failed ingestion exports the failure state, never the suspect
	// datum" scenario and design.md's D-2 resolution). The short version:
	// the requirement's intent is that a SUSPECT DATUM must never be
	// published, not that the pipeline must go silent. The publish gate
	// already guarantees the first structurally -- a blocked run writes no
	// observation, so the export physically cannot read one back -- while
	// suppressing the export made the failure INVISIBLE to readers, which
	// is the exact opposite of what PRD §6.1.3's validation banner exists
	// for, and left the same spec's own "A series whose latest run failed
	// MUST still appear, carrying its last valid data" unreachable.
	//
	// The condition is therefore "this cycle recorded a terminal run
	// outcome the artifact must now reflect", which is true in exactly two
	// cases, and the three failing kinds are deliberately NOT collapsed
	// into one another:
	//
	//   - published: a run wrote at least one observation. The artifact
	//     must carry the new datum. (Unchanged.)
	//   - failedValidation: a run reached the publish gate and was blocked,
	//     so ingestion_run now reads 'validation-failed' and
	//     postgres.SeriesValidationOutcome will resolve the series to
	//     PageStateValidationFailure. The artifact must carry the FAILURE,
	//     with the last valid datum still standing as the latest value.
	//     Covers a decode failure and an unclassified source status too:
	//     both converge on the same gate and the same recorded outcome.
	//   - neither: NOTHING new is knowable about any target series, so
	//     there is nothing to say and the currently published artifact
	//     stays exactly as it is. That is a fetch failure (no payload, no
	//     ingestion_run row -- the zero Result), a config-resolution
	//     failure that never reached a source, an infrastructure failure
	//     below the gate, and a 'nothing-new' run (a value in
	//     ingestion_run.outcome's documented set that nothing in Go writes
	//     yet -- see postgres.RunOutcome; when it lands it produces neither
	//     a published observation nor a Block verdict, so it falls in this
	//     branch by the same rule rather than needing a new one).
	//
	// A re-fetch that returns an UNCHANGED payload still publishes today
	// (postgres.ApplyGate records every candidate it hands the writer as
	// Published, whether or not WriteRevision actually changed a row), so
	// it exports. That is pre-existing slice-4 behaviour, deliberately
	// untouched here: narrowing it would change what a SUCCESSFUL cycle
	// does, which is outside CRITICAL-16's scope and would need its own
	// decision about what `generated_at` means.
	//
	// Still unconditional on `failed`: an unrelated series' failure in the
	// same batch does not withhold publication of what DID succeed (spec
	// pipeline-operations, "one scheduled job per source" implies one
	// series' failure is that series' own). outDir == "" (every pre-slice-4
	// test call site) skips this block entirely, preserving the "not
	// configured, not attempted" convention.
	if (published || failedValidation) && outDir != "" {
		result, err := publishing.Publish(ctx, buildExportDeps(db), dispatch, now, outDir, retentionHistoryDir(), retainedArtifacts(stderr))
		if err != nil {
			fmt.Fprintf(stderr, "ingest: publish: %v\n", err)
		} else {
			// Names which of the three dispatch states this cycle ended in
			// rather than claiming a dispatch unconditionally (verify-report
			// CRITICAL-28, link 2) -- see publishOutcomeMessage in
			// rebuild_dispatch.go.
			message, fault := publishOutcomeMessage(outDir, exportReason(published, failedValidation), result)
			if fault {
				fmt.Fprintln(stderr, message)
			} else {
				fmt.Fprintln(stdout, message)
			}
			// A publish that withdrew a series' files from the served
			// directory says so, by name -- the same reason the reconcile
			// above prints its retired counts rather than only its inserts.
			// See pruneOutcomeMessage (export_cmd.go).
			if message, ok := pruneOutcomeMessage("ingest: publish", outDir, result.Artifact.Prune); ok {
				fmt.Fprintln(stdout, message)
			}
			if result.ArchiveErr != nil {
				fmt.Fprintf(stderr, "ingest: publish: archiving retained artifact: %v\n", result.ArchiveErr)
			}
		}
	}

	if failed > 0 {
		fmt.Fprintf(stderr, "ingest: %d/%d series failed\n", failed, len(targets))
		return 1
	}
	return 0
}
