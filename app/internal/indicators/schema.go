package indicators

// ObservedSchema is what a source payload actually exposed for this
// run, as extracted by whichever adapter produced the incoming
// observations. Rule 1 (schema) stays pure by only ever COMPARING
// ObservedSchema against the declared expectation in
// config.SeriesConfig.Schema — it never reads bytes, parses a workbook
// or touches a filesystem itself. For non-columnar sources (INE/
// Eurostat JSON) only Fields is meaningful; the XLSX-specific fields
// are populated by the XLSX adapter (slice 8, not built yet).
type ObservedSchema struct {
	// Fields lists the field/column names actually present in the
	// payload, so rule 1 can name a specific missing one.
	Fields []string

	// XLSX-only structural facts (design.md "For workbook sources the
	// check MUST include sheet name, header row index, column anchors
	// and a header fingerprint").
	SheetName         string
	HeaderRow         int
	ColumnAnchors     map[string]string // declared field -> observed column reference
	HeaderFingerprint string
}
