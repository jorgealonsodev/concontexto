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
	// response's actual periodicity against expectedFrequency.
	Decode(raw []byte, ref string, expectedFrequency Frequency) (SourceResult, error)
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
type SourceResult struct {
	Name           string
	Observations   []Observation
	ObservedSchema ObservedSchema
}
