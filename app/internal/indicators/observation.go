package indicators

// ObservationStatus is the shared domain classification of one observed
// value: provisional, definitive or withdrawn (design D-3). It mirrors
// postgres.ObservationStatus's own P/D/W value set exactly, but is
// declared as its own type here — never imported from postgres — because
// indicators MUST NOT depend on any adapter (validation/purity_test.go's
// import-graph guard; the same reason postgres.Observation, not this
// type, owns provenance/versioning).
type ObservationStatus string

const (
	ObservationStatusProvisional ObservationStatus = "P"
	ObservationStatusDefinitive  ObservationStatus = "D"
	ObservationStatusWithdrawn   ObservationStatus = "W"
)

// Observation is one value at one period, as seen by the validation
// rules (task 4.2, design.md "type Rule func(ctx SeriesContext, incoming
// []indicators.Observation) []Finding"). It deliberately carries
// nothing about provenance, versioning or persistence — that is
// postgres.Observation's job (task 2.7) — the rule engine only needs
// the two facts a rule can reason about: which period, and what value.
//
// Status and SourceStatus are the one deliberate exception (design D-3,
// spec data-model-vintages "The source's verbatim status token is
// preserved"): every validation rule ignores both fields (asserted by
// validation/purity_test.go and rule4_revision_test.go's own
// status-only-change case), so their presence here does not weaken "the
// rule engine only needs...which period, and what value" — they exist
// purely to flow a source adapter's already-classified status (e.g.
// ine.Client.Decode's classifyTipoDato, fail-closed on an unrecognised
// token) through to postgres.ObservationInput without IngestSeries
// re-deriving or hardcoding it. Status is the classified P/D/W value;
// SourceStatus is the source's own verbatim token (INE's T3_TipoDato) or
// empty when the source carries none yet (an adapter that does not
// classify status at all, or Eurostat's genuine absent-flag case).
type Observation struct {
	Period       Period
	Value        *float64 // nil only for a withdrawn/absent observation
	Status       ObservationStatus
	SourceStatus string
}
