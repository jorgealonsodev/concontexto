// Package validation holds the six pure validation rules and the
// publish gate (§9.3, design.md "Validation: five pure rules + gate" —
// spec data-validation supersedes that count with six explicit rules,
// see tasks.md's 4.1-4.16 breakdown). Every rule in this package is a
// pure function over domain types and its own configuration: no I/O, no
// clock access, deterministic for identical inputs (spec
// data-validation, "Validation rules are pure functions"). That
// invariant is made falsifiable, not a convention, by
// purity_test.go's import-graph guard (task 4.1).
//
// This batch (PR 4a, tasks 4.1-4.8) implements the framework plus rules
// 1 (schema), 2 (continuity) and 3 (plausibility). Rules 4-6 and the
// publish gate (tasks 4.9-4.16) are a separate PR 4b.
package validation

import (
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// Severity classifies a Finding's effect on the publish gate (design.md
// "Finding struct{ Rule string; Severity Severity; ... } //
// Info|Block|BlockRequiresSignoff").
type Severity string

const (
	SeverityInfo                 Severity = "info"
	SeverityBlock                Severity = "block"
	SeverityBlockRequiresSignoff Severity = "block-requires-signoff" // rule 4 (revision), PR 4b
)

// Finding is one rule's verdict about one aspect of a run. Rule and
// Period together let the publish gate (task 4.16, PR 4b) report every
// failure, not only the first (spec "The gate reports every failure,
// not only the first").
type Finding struct {
	Rule     string
	Severity Severity
	Period   string // canonical period label this finding concerns; "" when not period-specific
	Message  string
}

// Rule is the shape every validation rule satisfies: a pure function of
// a resolved SeriesContext and the incoming candidate observations,
// returning zero or more findings. No I/O, no clock — see
// purity_test.go (spec "A rule is deterministic and side-effect free").
type Rule func(ctx SeriesContext, incoming []indicators.Observation) []Finding

// SeriesContext is everything a rule may look at, pre-resolved by the
// caller before validation.Run executes — rules themselves never fetch
// or compute any of this (design.md "Callers pre-resolve SeriesContext,
// rules do no I/O").
//
// This splits design.md's single "Config SeriesValidationConfig" field
// into Validation and Schema, reusing PR 3's actual config types
// verbatim (config.ValidationConfig, the new config.SchemaConfig)
// instead of inventing a parallel config model, per explicit
// instruction. This is a disclosed, deliberate deviation from design.md's
// prose, not a silent one.
type SeriesContext struct {
	// Series carries unit, frequency, decimals and (for rule 5, PR 4b)
	// the metadata needed for a metadata-completeness check.
	Series indicators.Series

	// Validation carries the per-series thresholds parsed from
	// series/{slug}.yaml: plausibility (rule 3), continuity (rule 2)
	// and revision (rule 4, PR 4b).
	Validation config.ValidationConfig

	// Schema carries rule 1's expected-fields/XLSX-anchor declaration,
	// also parsed from series/{slug}.yaml.
	Schema config.SchemaConfig

	// ObservedSchema is what THIS run's payload actually exposed —
	// populated by whichever adapter produced `incoming` (e.g. the
	// XLSX adapter reading a real sheet, slice 8). Rule 1 only ever
	// compares ObservedSchema against Schema; it never parses bytes
	// itself.
	ObservedSchema indicators.ObservedSchema

	// Prior is the current vintage before this run — rules 2-4 compare
	// against it, never against the database (design.md "Prior
	// []indicators.Observation // current vintage before this run").
	Prior []indicators.Observation

	// Breaks are already resolved for this exact series (design.md
	// "Breaks []indicators.Break // pre-resolved by scope expansion
	// (series ⊂ dataset ⊂ source)"). Rule 3 consumes this; it never
	// loads config/rupturas.yaml itself (that reconcile is slice 7).
	Breaks []indicators.Break
}
