package indicators

import "context"

// SourceClient is the port every source adapter (ine, eurostat, and
// later xlsx) implements so the application layer
// (app/internal/ingestion.IngestSeries) can fetch and decode one series
// without depending on any adapter's concrete type (design.md package
// layout, "indicators/ ... ports.go: SeriesRepo, ObservationWriter,
// RawFileStore, SourceClient" -- planned since PR 4a's design, built now
// because slice 6 is the first caller that needs IngestSeries to work
// with more than one adapter: spec source-ingestion-eurostat, "It MUST
// reuse the same domain types, observation writer and validation
// harness as the INE adapter; only the adapter differs").
//
// ref is a source-specific origin reference: an INE series COD
// (adapters/ine) or a Eurostat dataset code (adapters/eurostat). Every
// adapter's Client already carries whatever ELSE is needed to build a
// request (an INE client's base URL; a Eurostat client's base URL plus
// its already-pinned dimension filters) — ref is only the piece that
// varies per call.
type SourceClient interface {
	// RequestURL returns the exact request URL the client will issue for
	// ref -- used for download_attempt/raw_file bookkeeping (the
	// ingestion orchestrator archives raw bytes under this URL, spec
	// raw-file-archive).
	RequestURL(ref string) string

	// FetchRaw fetches ref's raw, undecoded response bytes -- no
	// periodicity assertion, no normalization -- so a caller can archive
	// the payload BEFORE any parsing happens (spec raw-file-archive's
	// ordering guarantee).
	FetchRaw(ctx context.Context, ref string) ([]byte, error)

	// Decode decodes and normalizes an already-fetched response body
	// (see FetchRaw) into canonical observations, asserting the
	// response's actual periodicity against expectedFrequency over the
	// WHOLE payload (spec source-ingestion-ine, "Periodicity MUST be
	// detected over the whole payload, not from a single observation").
	//
	// segments is variadic, not a plain slice, purely so every existing
	// three-argument call site across the adapters/probe/scheduler test
	// suites keeps compiling unchanged (task 1.2's own instruction: this
	// slice's job is the guard fix, not an unrelated call-site rewrite
	// across a dozen files). It carries the series' declared cadence
	// segments (design D-4); empty means the ordinary uniform-cadence
	// case AssertCadence already treats as ordinary dense/uniform.
	Decode(raw []byte, ref string, expectedFrequency Frequency, segments ...CadenceSegment) (SourceResult, error)
}

// SourceResult is a SourceClient's decoded outcome: the source's own
// descriptive name for ref (INE's "Nombre", Eurostat's JSON-stat
// "label") plus its normalized, periodicity-checked observations.
//
// ObservedSchema is additive (slice 8, XLSX): whichever adapter produced
// this result MAY populate it with what THIS run's payload actually
// exposed (sheet name, header row, anchors, fingerprint), so
// validation.Rule1Schema can compare it against the declared expectation
// without any adapter having to know about the validation package. INE
// and Eurostat never set it (JSON-stat/Tempus3 payloads have no sheet/
// column structure for rule 1 to compare), so it stays the zero value
// for both -- a strictly additive field, not a behaviour change.
//
// BreakSignals is additive (slice 2b, design D-3): a source-reported
// flag that is metadata rather than a status -- Eurostat's "b" (break in
// time series) and "d" (definition differs) -- routed here instead of
// into Observation.Status. IngestSeries logs and alerts when a signal's
// Period has no already-active series_break covering it; it never
// writes series_break itself (rupturas.yaml remains the only writer,
// editorial follow-up). INE never populates it (T3_TipoDato carries no
// such flag), so it stays nil for INE -- a strictly additive field.
type SourceResult struct {
	Name           string
	Observations   []Observation
	ObservedSchema ObservedSchema
	BreakSignals   []BreakSignal
}

// BreakSignal is one source-reported break/definition-differs flag at a
// given Period (design D-3's routing decision: "b"/"d" MUST NOT be
// mapped into the observation status enum"). Flag carries the source's
// own verbatim character ("b" or "d" for Eurostat today).
type BreakSignal struct {
	Period Period
	Flag   string
}
