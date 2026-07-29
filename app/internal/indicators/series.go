package indicators

// Series is the minimal per-series identity a validation rule needs:
// which cadence its periods advance at, plus the descriptive metadata
// rules not in this batch (rule 5, metadata completeness) will read.
// Fase 0's full series identity lives in config.SeriesConfig; Series is
// the pure-domain projection of it that rules are allowed to depend on
// (design.md "type SeriesContext struct{ Series indicators.Series //
// unit, frequency, decimals, metadata incl. licence ... }").
type Series struct {
	Slug      string
	Unit      string
	Frequency Frequency
	Decimals  int

	// Source and Licence back rule 5 (metadata completeness, Phase 4 PR
	// 4b): design.md "5 Metadata completeness (source, unit, frequency,
	// licence)". Series only carries the already-resolved strings a
	// rule can check for presence -- whichever caller builds
	// SeriesContext (config.SeriesConfig.Source / the owning
	// config.SourceConfig.Licence.Name, not built here) is responsible
	// for populating them; empty means "not resolved / missing".
	Source  string
	Licence string
}
