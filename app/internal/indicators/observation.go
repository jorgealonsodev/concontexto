package indicators

// Observation is one value at one period, as seen by the validation
// rules (task 4.2, design.md "type Rule func(ctx SeriesContext, incoming
// []indicators.Observation) []Finding"). It deliberately carries
// nothing about provenance, versioning or persistence — that is
// postgres.Observation's job (task 2.7) — the rule engine only needs
// the two facts a rule can reason about: which period, and what value.
type Observation struct {
	Period Period
	Value  *float64 // nil only for a withdrawn/absent observation
}
