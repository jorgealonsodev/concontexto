package publishing

// Export is publishing's ONE entry point for building, validating and
// writing an export artifact (design D-2: "Export(ctx, deps, asOf) (pure
// read -> model -> validate -> write)"). It is composed at the command
// layer (app/cmd/concontexto/export_cmd.go) -- never called from
// app/internal/ingestion.IngestSeries directly -- and reads through
// exactly three narrow, function-typed ports (Deps) so a test can
// substitute fakes with zero database (task 3.4's "Export() test with
// fake ports").
//
// Task 3.5's own wording names "ListCurrentObservations" as one of the
// three read ports. That existing function (app/internal/adapters/
// postgres/observation.go) deliberately returns only Period+Value --
// indicators.Observation "carries nothing about provenance, versioning
// or persistence" by its own doc comment -- because validation rules
// (its only prior caller) never needed more. Task 3.1 (D1's resolution,
// read this file alongside artifact.go's RunProvenance doc comment)
// requires status, source_status, version, ingestion_run_id AND a
// resolvable raw-file SHA-256 per observation, which that function
// structurally cannot supply without breaking its own contract for
// every existing caller. Deps therefore names a NEW, distinctly-named
// port -- ListObservations, backed by the new
// postgres.ListPublishedObservations (published_series.go) -- rather
// than silently repurposing the existing one. This is the deliberate,
// disclosed resolution D1 asked for, not a silent deviation from
// tasks.md's literal text.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
)

// Deps is the narrow read boundary Export depends on -- function-typed,
// not an interface, so a test builds a fake in three lines with no mock
// framework and no database (mirrors this codebase's established
// preference for plain package-level functions over interfaces, e.g.
// postgres.SeriesFreshness itself).
type Deps struct {
	// ListPublishedSeries returns every non-retired series' identity and
	// metadata (postgres.ListPublishedSeries).
	ListPublishedSeries func(ctx context.Context) ([]postgres.PublishedSeries, error)

	// ListObservations returns seriesID's full current vintage with
	// provenance (postgres.ListPublishedObservations). An empty result
	// means the series has never published anything yet -- Export skips
	// it entirely rather than emitting an empty, all-zero-value doc.
	ListObservations func(ctx context.Context, seriesID string) ([]postgres.PublishedObservation, error)

	// SeriesFreshness resolves the series' freshness as of asOf
	// (postgres.SeriesFreshness, its first real reader-facing consumer --
	// archive-report W16 stands corrected). See ArtifactFreshness below
	// for how its two ops-facing values map onto the artifact's two
	// reader-facing labels.
	SeriesFreshness func(ctx context.Context, seriesID string, asOf time.Time) (freshness.State, error)

	// ResolveActiveBreaksForSeries resolves seriesID's currently active,
	// family-scoped series_break rows (postgres.ResolveActiveBreaksForSeries,
	// which already existed before this slice -- only this Export-level
	// wiring is new). Slice 3 left SeriesDoc.Breaks structurally present
	// but always empty; task 4.2/4.6 closes that gap.
	ResolveActiveBreaksForSeries func(ctx context.Context, seriesID string) ([]postgres.SeriesBreak, error)

	// ListActiveEvents resolves the currently active (non-retired) events
	// applicable to seriesID (postgres.ListActiveEvents, new this slice --
	// see events_read.go for why it does not actually filter on seriesID
	// today). Slice 3 left SeriesDoc.Events structurally present but
	// always empty; task 4.2/4.6 closes that gap.
	ListActiveEvents func(ctx context.Context, seriesID string) ([]postgres.Event, error)

	// SeriesValidationOutcome resolves whether seriesID's most recent
	// ingestion run failed validation, and when it last succeeded
	// (postgres.SeriesValidationOutcome, Remediation B / CRITICAL-4).
	// It is one HALF of the page state: the other half, the editorial
	// discontinuation, arrives on postgres.PublishedSeries itself
	// because it is a column on `series`, not a fact about runs.
	// SeriesPageState composes the two.
	//
	// REQUIRED, and uniquely worth stating because it was not always.
	// This port shipped OPTIONAL, with nil meaning "no failure known" --
	// which resolves to PageStateFresh for every live series. The
	// intention was to keep pre-existing callers compiling; the effect
	// was that the single line binding it in production (buildExportDeps,
	// app/cmd/concontexto/export_cmd.go) became silently load-bearing:
	// deleting it would have reverted CRITICAL-4 in full -- every page
	// reporting "fresh" whatever its real validation outcome -- with the
	// entire Go suite still green, since nothing depended on the binding
	// (verify-report WARNING-17).
	//
	// Export now REFUSES a nil port instead. "I cannot know whether this
	// series' latest run failed" is not the same claim as "this series'
	// latest run did not fail", and only the second is what a "fresh"
	// page state tells a reader; emitting it from an absent port is the
	// fabrication P4 forbids, the same reasoning
	// firstUnclassifiedStatus (app/internal/ingestion/ingest.go) applies
	// to an unclassified observation status. Refusing to export at all
	// makes the regression IMPOSSIBLE rather than merely detectable,
	// which is the stronger of the two guarantees WARNING-17 asked to
	// choose between.
	SeriesValidationOutcome func(ctx context.Context, seriesID string) (postgres.ValidationOutcome, error)
}

// ArtifactFreshness maps postgres.SeriesFreshness's existing ops-facing
// State (fresh/failed, computed only from the source's last successful
// download attempt against a fixed 24h window -- no new computation, no
// comparison against build time) onto the artifact's reader-facing
// two-state semaphore (design D-2's orchestrator-settled narrowing: "the
// reader-facing amber semaphore means exactly 'the source has not
// published the expected period yet'"). StateFailed reads, from a
// reader's perspective, as exactly that fact -- relabelled, not
// recomputed. The SEPARATE 30-minute publish-latency watchdog state
// (design D-2: "this state is ops-only") is a slice-4/scheduler concern
// that never reaches this function or this artifact.
func ArtifactFreshness(state freshness.State) string {
	if state == freshness.StateFresh {
		return FreshnessFresh
	}
	return FreshnessSourcePending
}

// SeriesPageState composes which of PRD §6.1.3's three states a series'
// page must render, from the series' editorial discontinuation (a
// `series` column, migration 0005) and its validation outcome (derived
// from `ingestion_run`, postgres.SeriesValidationOutcome). Pure: it
// touches no database and no clock, so every branch below is testable
// with two plain structs.
//
// PRECEDENCE is discontinued > validation-failure > fresh.
//
// A retired series is permanently retired: no further period will ever
// arrive from that source. A validation banner layered on top of the
// discontinuation banner would be noise about a pipeline the reader no
// longer has a stake in -- it would ask them to wait for a correction to
// an update that is never coming. The permanent fact wins over the
// transient one. This also keeps the page to ONE banner, which is what
// the spec describes in each of its three scenarios.
//
// THE "FAILURE WITH NO PRIOR SUCCESS" EDGE CASE. postgres.
// SeriesValidationOutcome can honestly report LatestRunFailedValidation
// with LastSucceededAt == nil: a series whose very first run failed
// validation has a real failure and no last-correct-update date. The
// banner's copy names a date. There is none.
//
// Three resolutions were available, and two are wrong:
//
//   - Substitute a date (the run's own start, the extraction instant,
//     today). Rejected outright: the banner would assert "the last
//     correct update was {date}" when no correct update has ever
//     happened. That is a fabricated fact presented to a reader as
//     provenance -- precisely what PRD principle P4 (never fabricate;
//     an unknown is stated as unknown) forbids, and the same reasoning
//     BreakConfig.DateStatus already applies to an unconfirmed break
//     date rather than guessing one.
//   - Fall back to PageStateFresh. Also rejected: it hides a real,
//     recorded validation failure behind a page that claims everything
//     is normal. That is worse than the bug CRITICAL-4 reports, because
//     it would be a silent suppression rather than a missing wire.
//
// CHOSEN: emit PageStateValidationFailure with LastCorrectUpdate nil.
// The state is reported truthfully and the date is stated as absent, so
// the web layer -- which owns reader-facing copy, and is the only layer
// that can phrase the alternative sentence in Spanish -- selects wording
// that names no date. The artifact's job is to carry the facts, not to
// choose the sentence. web/src/lib/indicator/pageState.ts must therefore
// treat lastCorrectUpdate as nullable EVEN when kind is
// "validation-failure"; that is a real, load-bearing part of this
// contract and not a defensive nicety.
func SeriesPageState(ps postgres.PublishedSeries, outcome postgres.ValidationOutcome) PageStateRef {
	if ps.DiscontinuedSince != nil {
		return PageStateRef{Kind: PageStateDiscontinued, SuccessorSlug: ps.DiscontinuedSuccessorSlug}
	}
	if outcome.LatestRunFailedValidation {
		var lastCorrectUpdate *string
		if outcome.LastSucceededAt != nil {
			d := dateOnly(*outcome.LastSucceededAt)
			lastCorrectUpdate = &d
		}
		return PageStateRef{Kind: PageStateValidationFailure, LastCorrectUpdate: lastCorrectUpdate}
	}
	return PageStateRef{Kind: PageStateFresh}
}

func runKey(ingestionRunID int64) string { return strconv.FormatInt(ingestionRunID, 10) }

// Export reads every published series through deps, builds the
// in-memory Artifact, validates it (ValidateArtifact -- a violation
// aborts BEFORE any write, spec "MUST NOT write a partial artifact, and
// MUST leave the previously exported artifact intact"), and only then
// writes public/data-derived/ under outDir: series/{slug}.json first,
// manifest.json last (its own per-file digests can only be computed
// once every series document's final bytes are known -- design D-1,
// "digests computed last"). Each file is written via a temp-file-plus-
// rename swap in its own directory (writeFileAtomic below) so a reader
// never observes a partially-written JSON file, even though a crash
// between two files' renames can still leave the directory in a mixed
// old/new state -- a disclosed, narrower atomicity boundary than a
// single whole-directory swap would give (design D-1 states the
// intent, "write to data-derived.tmp, rename", without resolving that
// os.Rename cannot atomically replace a NON-EMPTY existing directory on
// POSIX; per-file atomic replace is this slice's chosen "equivalent").
func Export(ctx context.Context, deps Deps, asOf time.Time, outDir string) (Artifact, error) {
	// Composition check before any read and any write (verify-report
	// WARNING-17). Only this one port is checked explicitly, because it is
	// the only one whose absence used to be ABSORBED: Export calls every
	// other port unconditionally below, so a nil one panics on first use,
	// which is loud by itself. See Deps.SeriesValidationOutcome's own doc
	// comment for why "unknown" must not resolve to "fresh".
	if deps.SeriesValidationOutcome == nil {
		return Artifact{}, fmt.Errorf("publishing: Deps.SeriesValidationOutcome is not bound: refusing to export a page state that would report every series as fresh")
	}

	seriesList, err := deps.ListPublishedSeries(ctx)
	if err != nil {
		return Artifact{}, fmt.Errorf("publishing: listing published series: %w", err)
	}

	var docs []SeriesDoc
	for _, ps := range seriesList {
		obs, err := deps.ListObservations(ctx, ps.Slug)
		if err != nil {
			return Artifact{}, fmt.Errorf("publishing: listing observations for %s: %w", ps.Slug, err)
		}
		if len(obs) == 0 {
			continue // never published anything -- not yet a "published series"
		}

		state, err := deps.SeriesFreshness(ctx, ps.Slug, asOf)
		if err != nil {
			return Artifact{}, fmt.Errorf("publishing: resolving freshness for %s: %w", ps.Slug, err)
		}

		breaks, err := deps.ResolveActiveBreaksForSeries(ctx, ps.Slug)
		if err != nil {
			return Artifact{}, fmt.Errorf("publishing: resolving active breaks for %s: %w", ps.Slug, err)
		}
		events, err := deps.ListActiveEvents(ctx, ps.Slug)
		if err != nil {
			return Artifact{}, fmt.Errorf("publishing: listing active events for %s: %w", ps.Slug, err)
		}

		// One half of the page state; the discontinuation half travels on
		// ps, not through this port. The port is required (checked once at
		// the top of this function), so there is no "unknown outcome"
		// branch here to silently mean "fresh".
		outcome, err := deps.SeriesValidationOutcome(ctx, ps.Slug)
		if err != nil {
			return Artifact{}, fmt.Errorf("publishing: resolving the validation outcome for %s: %w", ps.Slug, err)
		}

		docs = append(docs, buildSeriesDoc(ps, obs, state, breaks, events, outcome))
	}

	sort.Slice(docs, func(i, j int) bool { return docs[i].Slug < docs[j].Slug })

	manifest := Manifest{SchemaVersion: SchemaVersion, GeneratedAt: asOf.UTC(), Digests: map[string]string{}}
	for _, d := range docs {
		manifest.Series = append(manifest.Series, d.Slug)
	}

	artifact := Artifact{Manifest: manifest, Series: docs}
	if err := ValidateArtifact(artifact); err != nil {
		return Artifact{}, err
	}

	seriesJSON := make(map[string][]byte, len(docs))
	for _, d := range docs {
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return Artifact{}, fmt.Errorf("publishing: marshalling series doc %s: %w", d.Slug, err)
		}
		seriesJSON[d.Slug] = b
		sum := sha256.Sum256(b)
		manifest.Digests["series/"+d.Slug+".json"] = hex.EncodeToString(sum[:])
	}
	artifact.Manifest = manifest

	for _, d := range docs {
		path := filepath.Join(outDir, "series", d.Slug+".json")
		if err := writeFileAtomic(path, seriesJSON[d.Slug]); err != nil {
			return Artifact{}, err
		}
		if err := writeSeriesCSV(outDir, d); err != nil {
			return Artifact{}, err
		}
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Artifact{}, fmt.Errorf("publishing: marshalling manifest: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(outDir, "manifest.json"), manifestBytes); err != nil {
		return Artifact{}, err
	}

	return artifact, nil
}

// buildSeriesDoc assembles one SeriesDoc from a series' metadata and its
// full current-vintage observation set. Withdrawn (status W) rows are
// excluded from Points and listed under Withdrawn instead (design D-1);
// every observation, withdrawn or not, still contributes its run to
// Vintages, because a withdrawal is itself provenance-bearing data (P7:
// disclose, never erase). The series-level Vintage/Origin.RequestURL
// convenience fields resolve to the MAXIMUM ingestion_run_id among the
// series' own current observations -- the most recent run that
// contributed any of the currently published state.
// sourceLicenceURL prefers the distinct licence-terms URL (migration 0004,
// slice 4 -- closes the slice-3 disclosed gap: source.licence_url now
// exists and is reconciled from config.LicenceConfig.URL on every ingest
// cycle). It falls back to the source's general website (SourceURL) only
// for a row not yet re-reconciled since that migration shipped -- the
// exact prior slice-3 behaviour, kept as a transitional fallback rather
// than emitting an empty string, disclosed rather than silently narrowed.
func sourceLicenceURL(ps postgres.PublishedSeries) string {
	if ps.SourceLicenceURL != "" {
		return ps.SourceLicenceURL
	}
	return ps.SourceURL
}

// dateOnly formats a time.Time as a bare calendar date ("2006-01-02"),
// UTC-normalised -- the format BreakRef.Date/EventRef.DateStart/DateEnd
// use, distinct from Point.Period's period-label format (e.g. "2002-Q1"):
// a break/event date is a calendar day, never a series period.
func dateOnly(t time.Time) string { return t.UTC().Format("2006-01-02") }

func dateOnlyPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := dateOnly(*t)
	return &s
}

func toBreakRefs(breaks []postgres.SeriesBreak) []BreakRef {
	out := make([]BreakRef, 0, len(breaks))
	for _, b := range breaks {
		out = append(out, BreakRef{
			Key: b.BreakKey, Date: dateOnly(b.Date), Kind: b.Kind, NoteMD: b.NoteMD, SourceURL: b.SourceURL,
		})
	}
	return out
}

func toEventRefs(events []postgres.Event) []EventRef {
	out := make([]EventRef, 0, len(events))
	for _, e := range events {
		var noteMD *string
		if e.NoteMD != nil && *e.NoteMD != "" {
			noteMD = e.NoteMD
		}
		out = append(out, EventRef{
			ID: e.ID, Group: e.Group, Name: e.Name,
			DateStart: dateOnly(e.DateStart), DateEnd: dateOnlyPtr(e.DateEnd), NoteMD: noteMD,
		})
	}
	return out
}

func buildSeriesDoc(ps postgres.PublishedSeries, obs []postgres.PublishedObservation, state freshness.State, breaks []postgres.SeriesBreak, events []postgres.Event, outcome postgres.ValidationOutcome) SeriesDoc {
	doc := SeriesDoc{
		SchemaVersion: SchemaVersion,
		Slug:          ps.Slug,
		Name:          ps.Name,
		Unit:          ps.Unit,
		Frequency:     ps.Frequency,
		Decimals:      ps.Decimals,
		Geo:           ps.Geo,
		Operation:     ps.DatasetID,
		Source: SourceRef{
			ID:          ps.SourceID,
			Name:        ps.SourceName,
			Attribution: ps.SourceAttribution,
			LicenceName: ps.SourceLicenceName,
			LicenceURL:  sourceLicenceURL(ps),
		},
		Origin:       OriginRef{Kind: ps.OriginKind, Ref: ps.OriginRef},
		Points:       []Point{},
		SourceStatus: map[string]string{},
		Withdrawn:    []string{},
		Vintages:     map[string]RunProvenance{},
		Breaks:       toBreakRefs(breaks),
		Events:       toEventRefs(events),
		Freshness:    ArtifactFreshness(state),
		PageState:    SeriesPageState(ps, outcome),
	}

	var vintageRunID int64
	for _, o := range obs {
		doc.Vintages[runKey(o.IngestionRunID)] = RunProvenance{
			ExtractedAt:   o.ExtractedAt.UTC(),
			RawFileSHA256: o.RawFileSHA256,
			RequestURL:    o.RequestURL,
		}
		if o.IngestionRunID > vintageRunID {
			vintageRunID = o.IngestionRunID
		}

		if o.Status == postgres.StatusWithdrawn {
			doc.Withdrawn = append(doc.Withdrawn, o.Period)
			continue
		}
		value := 0.0
		if o.Value != nil {
			value = *o.Value
		}
		doc.Points = append(doc.Points, Point{
			Period: o.Period, Value: value, Status: string(o.Status),
			Version: o.Version, IngestionRunID: o.IngestionRunID,
		})
		if o.SourceStatus != nil && *o.SourceStatus != "" {
			doc.SourceStatus[o.Period] = *o.SourceStatus
		}
	}

	if vintageRunID > 0 {
		rp := doc.Vintages[runKey(vintageRunID)]
		doc.Vintage = Vintage{IngestionRunID: vintageRunID, ExtractedAt: rp.ExtractedAt}
		doc.Origin.RequestURL = rp.RequestURL
	}

	sort.Slice(doc.Points, func(i, j int) bool { return doc.Points[i].Period < doc.Points[j].Period })
	sort.Strings(doc.Withdrawn)

	return doc
}

// writeFileAtomic writes data to path via a temp file in the same
// directory followed by os.Rename -- atomic on every POSIX filesystem
// regardless of whether path already exists, unlike attempting to
// rename a whole directory onto a non-empty existing one (see Export's
// own doc comment). Parent directories are created as needed; that is
// pure filesystem scaffolding, not artifact content, so it is safe to
// perform even on a path that ultimately fails validation upstream --
// though in practice Export never calls this before ValidateArtifact
// has already passed.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("publishing: creating directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("publishing: creating a temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("publishing: writing temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("publishing: closing temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("publishing: renaming temp file into place at %s: %w", path, err)
	}
	return nil
}
