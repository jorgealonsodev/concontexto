// Package publishing is the Fase 0-reserved, never-built export layer
// (ADR-7, design D-1/D-2): the Go pipeline's ONLY route to
// public/data-derived/, the versioned build-time contract the Astro
// build reads. It composes at the command layer (app/cmd/concontexto),
// never inside app/internal/ingestion.IngestSeries — the export runs
// once per ingest CYCLE, not once per series fetch.
//
// This file defines the in-memory artifact model IS public/data-derived/
// (design D-1: "the artifact IS /data-derived/", not an internal
// interchange format that also happens to publish data). Two documents
// exist per export: one Manifest and one SeriesDoc per published series.
package publishing

import "time"

// SchemaVersion is the export artifact's current schema version (design
// D-1, "Read side ... requires schema_version to equal the one version
// the loader supports -- exact integer equality, not >="). A future
// incompatible shape change MUST bump this constant in the same PR that
// updates the Astro Zod loader reading it (design D-1's "Version-bump
// protocol").
const SchemaVersion = 1

// Freshness is the artifact's reader-facing two-state semaphore (design
// D-2, orchestrator-settled narrowing of the proposal's three-state
// exit criterion: "the reader-facing amber semaphore means exactly 'the
// source has not published the expected period yet'"). The 30-minute
// publish-latency watchdog state is a SEPARATE, ops-only concept (slice
// 4, scheduler) that never reaches this artifact.
const (
	FreshnessFresh         = "fresh"
	FreshnessSourcePending = "source-pending"
)

// The three page states of PRD §6.1.3 (spec indicator-page, "The three
// page states of PRD §6.1.3"). These are the ONLY values
// SeriesDoc.PageState.Kind may take; ValidateArtifact rejects anything
// else before a byte reaches disk.
//
// Deliberately distinct from Freshness above, which is a different axis
// answering a different question. Freshness is about the SOURCE ("has it
// published the expected period yet"); page state is about OUR pipeline
// and the series' editorial status ("did our validation reject what the
// source published", "has the source retired this series"). A series can
// be source-pending and fresh-stated, or fresh and validation-failed;
// collapsing the two axes into one enum would make those combinations
// inexpressible.
const (
	PageStateFresh             = "fresh"
	PageStateValidationFailure = "validation-failure"
	PageStateDiscontinued      = "discontinued"
)

// PageStateRef is a series doc's "pageState" section -- the data path
// PRD §6.1.3's three banners are driven by, closing verify-report
// CRITICAL-4 (before it, page state was a hand-maintained constant in
// web/src/content/indicators/methodology.ts, so a real validation
// failure produced no banner without a source edit and a redeploy).
//
// It is a REQUIRED object on every series document, never omitted, and
// all three keys are always emitted -- explicit null rather than an
// absent key (no omitempty on the two pointers). A reader can then tell
// "this series has no successor" from "this artifact predates page
// state" without guessing; an absent key would conflate them.
//
// LastCorrectUpdate is an ISO calendar date ("2006-01-02"), non-nil ONLY
// when Kind is PageStateValidationFailure: it is the date of the last
// SUCCEEDED ingestion run, the {fecha} the spec's banner names verbatim
// ("Última actualización correcta: {fecha}"). It is a plain date string,
// not a time.Time, because the banner names a DAY -- serialising an
// instant would invite a timezone to shift the rendered date by one.
//
// SuccessorSlug is non-nil ONLY when Kind is PageStateDiscontinued AND a
// successor is configured (spec: "a successor link is shown when a
// successor is configured" -- the successor is genuinely optional; a
// source can retire a series without publishing a replacement).
type PageStateRef struct {
	Kind              string  `json:"kind"`
	LastCorrectUpdate *string `json:"lastCorrectUpdate"`
	SuccessorSlug     *string `json:"successorSlug"`
}

// Manifest is manifest.json's shape: the declared schema version, the
// generation instant, and a per-file sha256 digest so the read side can
// verify every fetched file before parsing it (design D-1, "verifies
// every sha256 against the manifest"). Digests is keyed by each file's
// path relative to the artifact directory root (e.g.
// "series/tasa-de-paro-epa.json").
type Manifest struct {
	SchemaVersion int               `json:"schema_version"`
	GeneratedAt   time.Time         `json:"generated_at"`
	Series        []string          `json:"series"`
	Digests       map[string]string `json:"digests"`
}

// PruneOutcome records what Export DELETED from outDir on top of what it
// wrote (export.go's pruneUnpublishedFiles). It exists because a removal
// is reader-facing data disappearing: an export that silently deletes a
// file a reader could reach is its own integrity problem, so the counts
// travel back to the command layer exactly the way the editorial
// reconcile already reports inserted/updated/retired.
//
// Removed holds manifest-relative paths ("series/{slug}.json",
// "csv/{slug}.csv") -- the same keys Manifest.Digests uses -- sorted, so
// an operator reading a log line sees the two projections of one
// withdrawn series adjacent rather than interleaved with another's.
//
// Skipped is the discriminator an empty Removed alone cannot provide, and
// it exists for the same reason PublishResult.DispatchSkipped does: an
// empty Removed means BOTH "there was nothing stale to remove" and "the
// zero-series guard REFUSED to remove anything", and those are opposite
// operational facts. The first is a healthy steady state; the second says
// the directory is knowingly holding more than the manifest declares. A
// guard that fires in silence cannot be told from one that never fired.
type PruneOutcome struct {
	Removed []string
	Skipped bool
}

// Artifact is the full in-memory export: the manifest plus one SeriesDoc
// per published series. publishing.Export builds this; ValidateArtifact
// checks it before a single byte reaches disk; writeArtifact serialises
// it into public/data-derived/.
//
// Prune is deliberately NOT part of that content: it describes what the
// WRITE did to a directory, not what the artifact says. Nothing
// serialises an Artifact as a whole today (Export marshals Manifest and
// each SeriesDoc individually), and the `json:"-"` tag is there so that
// stays true if anything ever does -- a removal report has no place in a
// document a reader fetches.
type Artifact struct {
	Manifest Manifest
	Series   []SeriesDoc
	Prune    PruneOutcome `json:"-"`
}

// SourceRef is a series doc's "source" section (spec publishing-export,
// "series metadata (source ...)"). LicenceURL resolves from
// source.licence_url (migration 0004, slice 4) -- the distinct licence-
// terms URL config.LicenceConfig.URL has always schema-validated, now
// actually persisted (postgres.reconcileSource) and read back
// (postgres.ListPublishedSeries) instead of falling back to the source's
// general website. See export.go's sourceLicenceURL for the transitional
// fallback covering a row not yet re-reconciled since the migration.
type SourceRef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Attribution string `json:"attribution"`
	LicenceName string `json:"licenceName"`
	LicenceURL  string `json:"licenceUrl"`
}

// OriginRef is a series doc's "origin" section: the pinned source
// identifier (series_source_mapping) plus the exact URL the most recent
// contributing run actually downloaded from (raw_file.url of the run
// backing Vintage below) -- never a reconstructed URL, so this field
// never drifts from what was genuinely requested.
type OriginRef struct {
	Kind       string `json:"kind"`
	Ref        string `json:"ref"`
	RequestURL string `json:"requestUrl"`
}

// Vintage is the series-level "which run does this document represent"
// convenience field (design D-1: "each series doc records vintage:
// {ingestion_run_id, extracted_at}"). It is the current maximum
// ingestion_run_id among the series' own current observations -- the
// most recent run that contributed any of the currently published data.
// Per-OBSERVATION provenance does not stop here: see Vintages below
// (D1's resolution).
type Vintage struct {
	IngestionRunID int64     `json:"ingestionRunId"`
	ExtractedAt    time.Time `json:"extractedAt"`
}

// Point is one published (period, value) pair, carrying every
// provenance-adjacent fact the spec's "full observation history"
// requirement names directly (period, value, status, version,
// ingestion_run_id) -- source_status is carried separately in SeriesDoc
// (design D-3's own convention: null/absent for most periods, so a
// sparse map avoids repeating an empty field on every point).
// IngestionRunID is the key into SeriesDoc.Vintages (D1's fix): looking
// a point's own run up there resolves its extraction timestamp and
// raw-file SHA-256 with no further database query.
type Point struct {
	Period         string  `json:"period"`
	Value          float64 `json:"value"`
	Status         string  `json:"status"` // "P" | "D" -- never "W" (withdrawn rows go to Withdrawn)
	Version        int     `json:"version"`
	IngestionRunID int64   `json:"ingestionRunId"`
}

// RunProvenance is D1's resolution: the reconciliation table found the
// design's illustrative schema carried ONE vintage.ingestionRunId per
// series document and NO raw-file hash at all -- insufficient for the
// spec's "Provenance survives the export" scenario, which requires
// source, origin identifier, extraction timestamp, ingestion run AND
// raw-file SHA-256 to be reachable per OBSERVATION, not once per
// series. Every ingestion_run_id referenced by any Point or Withdrawn
// entry in a SeriesDoc resolves here -- a per-run lookup (an
// "equivalent" to a literal per-version lookup, task 3.1: many
// observations from the same run share an identical extraction instant
// and raw file, so keying by run avoids repeating those two fields on
// every single point) reachable with zero further database queries.
type RunProvenance struct {
	ExtractedAt   time.Time `json:"extractedAt"`
	RawFileSHA256 string    `json:"rawFileSha256"`
	RequestURL    string    `json:"requestUrl"`
}

// BreakRef and EventRef are the series doc's break/event sections (spec
// "the series breaks that resolve for it, the editorial events and
// government entries that apply to it"). Structurally present since
// slice 3 so ValidateArtifact can validate them (task 3.2's "rejects
// unresolved break/event reference" scenario); slice 4 wires both fields
// to real data via two new Deps ports (export.go: ResolveActiveBreaksForSeries,
// ListActiveEvents) and postgres.ListActiveEvents (events_read.go, new
// this slice -- ResolveActiveBreaksForSeries itself already existed).
type BreakRef struct {
	Key       string  `json:"key"`
	Date      string  `json:"date"`
	Kind      string  `json:"kind"`
	NoteMD    string  `json:"noteMd"`
	SourceURL *string `json:"sourceUrl,omitempty"`
}

// EventRef carries NO scope. The scope is what
// postgres.ListActiveEvents already USED to decide that this entry belongs
// on this series' document at all, so repeating it here would publish the
// filter's input alongside its output — a second copy of a fact, for a
// reader who has no question it answers.
type EventRef struct {
	ID        string  `json:"id"`
	Group     string  `json:"group"`
	Name      string  `json:"name"`
	DateStart string  `json:"dateStart"`
	DateEnd   *string `json:"dateEnd,omitempty"`
	NoteMD    *string `json:"noteMd,omitempty"`

	// SourceURL is the document the entry was verified against, where the
	// registry recorded one — the same `*string` + `omitempty` shape
	// BreakRef already uses, so an absent citation OMITS the key rather
	// than emitting null.
	SourceURL *string `json:"sourceUrl,omitempty"`
}

// SeriesDoc is one series/{slug}.json document -- the per-series shape
// the spec's "Versioned build-time export artifact" requirement lists
// field by field: "canonical slug, series metadata (source, statistical
// operation, origin series identifier, unit, base, decimals, cadence,
// licence and attribution), the full observation history (...), the
// series breaks that resolve for it, the editorial events and
// government entries that apply to it, its source-relative freshness
// state, and the vintage identifier the artifact represents".
type SeriesDoc struct {
	SchemaVersion int    `json:"schema_version"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Unit          string `json:"unit"`
	Frequency     string `json:"frequency"`
	Decimals      int    `json:"decimals"`
	Geo           string `json:"geo"`

	// Operation is the "statistical operation" the spec's field list
	// requires. Fase 0/1's config has no dedicated descriptive field for
	// it yet (config.SeriesConfig/DatasetConfig carry no "operation"
	// name; postgres.reconcileDataset sets dataset.name literally equal
	// to the dataset id, e.g. "ine-epa") -- Operation is populated from
	// that existing, honest DB fact rather than a fabricated Spanish
	// description. Disclosed as a new Open Question in design.md, not
	// silently invented (P4).
	Operation string `json:"operation"`

	// Base is the index base period (e.g. "2021=100" for IPC-family
	// series). No configured series declares one anywhere in
	// config/series/*.yaml today -- there is no field to read it from.
	// Always nil until a future slice adds one; disclosed, not
	// fabricated.
	Base *string `json:"base"`

	Source SourceRef `json:"source"`
	Origin OriginRef `json:"origin"`

	Vintage Vintage `json:"vintage"`

	Points       []Point           `json:"points"`
	SourceStatus map[string]string `json:"sourceStatus"`
	Withdrawn    []string          `json:"withdrawn"`

	// Vintages is D1's resolution (task 3.1) -- see RunProvenance's own
	// doc comment. Keyed by ingestion_run_id formatted as a decimal
	// string (JSON object keys are always strings).
	Vintages map[string]RunProvenance `json:"vintages"`

	Breaks []BreakRef `json:"breaks"`
	Events []EventRef `json:"events"`

	// Freshness is "fresh" or "source-pending" (design D-2's settled
	// two-reader-state narrowing) -- ArtifactFreshness maps the existing,
	// already-live postgres.SeriesFreshness/freshness.State onto these
	// two labels; see export.go.
	Freshness string `json:"freshness"`

	// PageState is which of PRD §6.1.3's three states this series' page
	// must render, composed by SeriesPageState (export.go) from the
	// series' validation outcome and its editorial discontinuation. See
	// PageStateRef's own doc comment for the field-level contract.
	PageState PageStateRef `json:"pageState"`
}
