package postgres_test

// Task 8.10/8.11 (RED/GREEN, interaction half): "the repository receives
// zero write calls" is an INTERACTION property, not a state property --
// a real database's post-run state cannot distinguish "the writer was
// never called" from "the writer was called, then rolled back". A real
// database (this package's own testcontainers harness, reused
// unmodified from gate_test.go) proves the STATE half. This file proves
// the CALL-COUNT half with a test-only spy substituted for the real
// *postgres.ObservationWriter through the new ApplyGateWithWriter seam
// (gate.go's ObservationWriterPort), per the orchestrator's explicit
// design decision for slice 8b: "use BOTH, because the spec asks for two
// different things."
//
// Why this test is written ONCE, generically, rather than once per
// malformed-XLSX case: every one of the nine malformed-file cases (spec
// source-ingestion-xlsx, "Every malformed workbook writes nothing and
// preserves the published datum") reaches this exact interaction through
// the SAME codepath. Either xlsx.Decode itself fails, in which case
// app/internal/ingestion.IngestSeries calls ApplyGate with a synthetic
// all-Block finding and a literal nil candidates slice (ingest.go); or
// Decode succeeds but a later rule (e.g. rule1-schema's header
// fingerprint check) blocks, in which case ApplyGate is called with a
// non-empty candidates slice but a Block-severity finding. Either way,
// ApplyGate's Block branch returns before the writer loop is ever
// reached -- the loop below simply never runs when Outcome==GateBlock,
// regardless of how many candidates were passed in. That is a property
// of ApplyGate itself, not of any one malformed fixture, so proving it
// once here is complete proof for all nine cases; the per-case burden
// (app/internal/adapters/xlsx/malformed_test.go) is to show each
// fixture genuinely reaches a Block outcome.
//
// The one deliberately duplicated case is partial-success-within-one-
// workbook, "the sharpest case" per the batch instructions: it gets its
// OWN end-to-end proof against a real Postgres AND a real malformed
// fixture in app/internal/ingestion/malformed_xlsx_test.go, because that
// is the case most worth proving all the way through the real stack
// rather than relying on the structural argument alone.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// spyObservationWriter records every WriteRevision call it receives.
// Satisfies postgres.ObservationWriterPort; never touches a database --
// it exists purely to make "zero write calls" a directly observable,
// falsifiable fact instead of an inference from unchanged state.
type spyObservationWriter struct {
	calls []postgres.ObservationInput
}

func (s *spyObservationWriter) WriteRevision(_ context.Context, in postgres.ObservationInput) (postgres.Observation, bool, error) {
	s.calls = append(s.calls, in)
	return postgres.Observation{}, true, nil
}

var _ postgres.ObservationWriterPort = (*spyObservationWriter)(nil)

// TestApplyGate_BlockNeverCallsTheObservationWriter is the interaction
// proof described above. It mirrors exactly what
// app/internal/ingestion.IngestSeries does on a decode failure: one
// synthetic "source-decode" Block finding (ingest.go's own literal
// finding shape) and nil candidates -- but ALSO covers the
// non-nil-candidates case (a validation-rule Block after Decode
// succeeded), since a spy substituted here would catch a regression
// either way.
func TestApplyGate_BlockNeverCallsTheObservationWriter(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	run := seedIngestionRun(t, ctx, tx, "hash-malformed-run", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	cases := []struct {
		name       string
		findings   []validation.Finding
		candidates []postgres.ObservationInput
	}{
		{
			name: "decode failure (nil candidates, mirrors ingest.go's decodeErr branch)",
			findings: []validation.Finding{
				{Rule: "source-decode", Severity: validation.SeverityBlock, Message: "xlsx: opening workbook: zip: not a valid zip file"},
			},
			candidates: nil,
		},
		{
			name: "validation-rule block after a successful decode (non-nil candidates)",
			findings: []validation.Finding{
				{Rule: "rule1-schema", Severity: validation.SeverityBlock, Message: "header fingerprint mismatch"},
			},
			candidates: []postgres.ObservationInput{{
				SeriesID: "tasa-de-paro-epa", Period: "2026-Q1", Value: ptr(1.0),
				Status: postgres.StatusDefinitive, ExtractedAt: time.Now(), IngestionRunID: run,
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &spyObservationWriter{}
			result, err := postgres.ApplyGateWithWriter(ctx, tx, spy, run, tc.findings, tc.candidates)
			if err != nil {
				t.Fatalf("ApplyGateWithWriter: %v", err)
			}
			if result.Outcome != validation.GateBlock {
				t.Fatalf("expected GateBlock, got %v", result.Outcome)
			}
			if len(spy.calls) != 0 {
				t.Fatalf("expected the observation writer to receive ZERO calls on a blocked run, got %d: %+v", len(spy.calls), spy.calls)
			}
			if len(result.Published) != 0 {
				t.Fatalf("expected zero published observations, got %d", len(result.Published))
			}
		})
	}
}
