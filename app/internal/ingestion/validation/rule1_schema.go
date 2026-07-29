package validation

// Task 4.4 (GREEN): rule 1 (schema). Spec data-validation, "Rule 1 —
// schema": expected fields must be present with correct types before
// normalisation; for XLSX sources the check also covers sheet name,
// header row index, column anchors and the header fingerprint.
//
// Rule1Schema never reads bytes, parses a workbook, or touches a
// filesystem: it only compares the declared expectation
// (ctx.Schema, config.SeriesConfig.Schema) against what THIS run's
// payload already exposed (ctx.ObservedSchema, populated by whichever
// adapter produced `incoming`) — the same "consume the pre-resolved
// input" pattern rule 3 uses for breaks.
//
// NOTE for slice 8 (do NOT implement here): the real Social Security
// workbook needs an additional ARITHMETIC invariant — the declared
// total column must equal the sum of the declared component columns,
// because that workbook's header row does not align with its data
// columns. config.XLSXSchemaConfig is a plain, additive struct so that
// check can be added later as a new field without breaking the checks
// implemented here.

import (
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func containsField(fields []string, name string) bool {
	for _, f := range fields {
		if f == name {
			return true
		}
	}
	return false
}

// Rule1Schema is documented at length in the package's method table
// (spec "A renamed column fails schema validation ... the failure names
// the missing field").
func Rule1Schema(ctx SeriesContext, incoming []indicators.Observation) []Finding {
	var findings []Finding

	for _, field := range ctx.Schema.ExpectedFields {
		if !containsField(ctx.ObservedSchema.Fields, field) {
			findings = append(findings, Finding{
				Rule:     "rule1-schema",
				Severity: SeverityBlock,
				Message:  fmt.Sprintf("expected field %q is missing from the payload", field),
			})
		}
	}

	xlsx := ctx.Schema.XLSX
	if xlsx == nil {
		return findings
	}
	observed := ctx.ObservedSchema

	if xlsx.SheetName != "" && observed.SheetName != xlsx.SheetName {
		findings = append(findings, Finding{
			Rule: "rule1-schema", Severity: SeverityBlock,
			Message: fmt.Sprintf("expected sheet %q, payload has %q", xlsx.SheetName, observed.SheetName),
		})
	}
	if xlsx.HeaderRow != 0 && observed.HeaderRow != xlsx.HeaderRow {
		findings = append(findings, Finding{
			Rule: "rule1-schema", Severity: SeverityBlock,
			Message: fmt.Sprintf("expected header row %d, payload has %d", xlsx.HeaderRow, observed.HeaderRow),
		})
	}
	if xlsx.HeaderFingerprint != "" && observed.HeaderFingerprint != xlsx.HeaderFingerprint {
		findings = append(findings, Finding{
			Rule: "rule1-schema", Severity: SeverityBlock,
			Message: "header fingerprint mismatch: the header row no longer matches what was declared (columns likely renamed or reordered)",
		})
	}
	for field, expectedAnchor := range xlsx.ColumnAnchors {
		observedAnchor := ""
		if observed.ColumnAnchors != nil {
			observedAnchor = observed.ColumnAnchors[field]
		}
		if observedAnchor != expectedAnchor {
			findings = append(findings, Finding{
				Rule: "rule1-schema", Severity: SeverityBlock,
				Message: fmt.Sprintf("column anchor for %q moved: expected %s, payload has %s", field, expectedAnchor, observedAnchor),
			})
		}
	}

	return findings
}
