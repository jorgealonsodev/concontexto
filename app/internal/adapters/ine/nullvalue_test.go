package ine_test

// verify-report WARNING-30 (RED/GREEN): a DATOS_SERIE row carrying an
// explicit "Valor": null MUST fail the decode closed as a named
// sourceerr.SchemaDrift, rather than being carried through as an
// observation with a nil value (spec source-ingestion-ine, "A null Valor
// has no decided meaning and fails closed").
//
// Why this is NOT the Eurostat fix (befa81f, adapters/eurostat): a sparse
// JSON-stat value map simply has NO entry at a position, which is the
// ABSENCE of a datum and is decidable -- the source published nothing for
// that period, so no observation is emitted. INE is the opposite shape: a
// row EXISTS and carries an explicit null. That is a positive act by the
// source, and silently discarding it would throw away something INE chose
// to publish.
//
// Why refusal rather than any projection: the five fields a DATOS_SERIE
// row carries (Fecha, Anyo, T3_Periodo, T3_TipoDato, Valor -- verified
// live 2026-07-30 across all six configured series, 1,032 rows) include
// NO secrecy marker, no not-applicable flag and no annotation of any
// kind. A null therefore arrives carrying no information whatsoever about
// why, so statistical secrecy, a not-applicable period and a genuine gap
// are indistinguishable. Every available interpretation would be an
// invention -- and marking it withdrawn in particular would fabricate a
// retraction, the exact falsehood befa81f refused for Eurostat. This
// mirrors ingestion.firstUnclassifiedStatus, which fails a whole run
// closed rather than default an unclassified status.
//
// The case is latent, not live: those same 1,032 rows carry zero nulls.
// These tests exist so the day INE does publish one, the run stops with a
// legible refusal naming the series, the period and the row, instead of
// the cryptic, far-downstream `violates check constraint
// "observation_check" (SQLSTATE 23514)` the publish gate produces today
// from migration 0001's CHECK (value IS NOT NULL OR status = 'W').

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// nullValorRow renders one DATOS_SERIE Data[] entry with an explicit
// null Valor, built by hand from the REAL five-field row shape (Fecha,
// Anyo, T3_Periodo, T3_TipoDato, Valor) so no field INE does not send is
// invented into the fixture. Fecha is included because the real row
// carries it, even though the adapter's wire type deliberately does not
// decode it -- the canonical period comes from Anyo + T3_Periodo
// (period.go), and an undecoded field must still be tolerated.
func nullValorRow(anyo int, periodo, tipoDato string) string {
	if tipoDato == "" {
		// A tip=A response that omits T3_TipoDato entirely -- the same
		// shape tipodato_test.go's "missing token" case exercises.
		return fmt.Sprintf(
			`{"Fecha": "%d-01-01T00:00:00.000+01:00", "T3_Periodo": %q, "Anyo": %d, "Valor": null}`,
			anyo, periodo, anyo)
	}
	return fmt.Sprintf(
		`{"Fecha": "%d-01-01T00:00:00.000+01:00", "T3_TipoDato": %q, "T3_Periodo": %q, "Anyo": %d, "Valor": null}`,
		anyo, tipoDato, periodo, anyo)
}

// TestDecodeSeries_NullValorFailsClosedRatherThanEmittingANilValuedObservation
// is the headline case: the exact payload the finding describes -- a
// published null alongside a DEFINITIVE T3_TipoDato, which is what makes
// it reach migration 0001's CHECK constraint at all (a withdrawn status
// would satisfy it, and manufacturing one is precisely what this refuses).
func TestDecodeSeries_NullValorFailsClosedRatherThanEmittingANilValuedObservation(t *testing.T) {
	body := wireBody([]string{
		wireRow(2026, "T1", 10.0),
		nullValorRow(2026, "T2", "Definitivo"),
	})

	result, err := ine.DecodeSeries([]byte(body), "test-null-valor-definitive", indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatalf("expected a null Valor to fail the decode closed, got %d observations", len(result.Observations))
	}
	assertSchemaDrift(t, err)

	// The refusal must be legible to whoever first hits it, possibly
	// years from now: it names WHICH series and WHICH period, echoes the
	// row as decoded, and says plainly that the project has no decided
	// meaning for this rather than naming a Postgres constraint.
	for _, want := range []string{
		"test-null-valor-definitive", // the series
		"2026-Q2",                    // the canonical period, not the raw "T2"
		"Definitivo",                 // the row's own status token
		"null Valor",                 // the source behaviour, in INE's own field name
		"no decided meaning",         // the refusal itself
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected the refusal to contain %q, got: %v", want, err)
		}
	}

	// It must also name a way forward -- the same register the
	// acknowledgement registry's `todo` uses, where an unsigned record
	// states exactly what the reviewer must do rather than only that
	// something is missing.
	if !strings.Contains(err.Error(), "source-ingestion-ine") {
		t.Errorf("expected the refusal to name the spec where the projection must be decided, got: %v", err)
	}

	// Nothing partial survives: the refusal replaces the whole decode,
	// so no caller can receive the surrounding rows and quietly proceed
	// with a series that is missing a period nobody declared missing.
	if len(result.Observations) != 0 {
		t.Errorf("expected no observations to survive the refusal, got %d", len(result.Observations))
	}
}

// TestDecodeSeries_NullValorFailsClosedUnderEveryTipoDatoToken proves the
// decision does not depend on which status token accompanies the null.
// Two of these four cases are surprising in a SECOND way as well (an
// unrecognised token, an omitted one), and the null still wins: the
// value-presence branch lives in decodeAndNormalize, one layer BELOW
// Client.Decode's token classification, deliberately -- see the GREEN
// implementation's comment for why containment beats matching Eurostat's
// opposite ordering here.
func TestDecodeSeries_NullValorFailsClosedUnderEveryTipoDatoToken(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tipoDato string
	}{
		{name: "definitive", tipoDato: "Definitivo"},
		{name: "provisional", tipoDato: "Provisional"},
		{name: "unrecognised token", tipoDato: "Estimado"},
		{name: "omitted token", tipoDato: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := wireBody([]string{nullValorRow(2026, "T1", tc.tipoDato)})

			// Both exported decode entry points must refuse. DecodeSeries
			// and FetchSeries never run classifyTipoDato at all, so a check
			// placed only in Client.Decode would leave them able to hand
			// back a nil-valued ine.Observation.
			if _, err := ine.DecodeSeries([]byte(body), "test-null-valor-any-token", indicators.FrequencyQuarterly); err == nil {
				t.Error("expected DecodeSeries to refuse a null Valor")
			} else {
				assertSchemaDrift(t, err)
				if !strings.Contains(err.Error(), "null Valor") {
					t.Errorf("expected the refusal to name the null Valor, got: %v", err)
				}
			}

			client := ine.NewClient("https://example.test", nil)
			if _, err := client.Decode([]byte(body), "test-null-valor-any-token", indicators.FrequencyQuarterly); err == nil {
				t.Error("expected Client.Decode to refuse a null Valor")
			} else {
				assertSchemaDrift(t, err)
				if !strings.Contains(err.Error(), "null Valor") {
					t.Errorf("expected the refusal to name the null Valor, got: %v", err)
				}
			}
		})
	}
}

// TestDecodeSeries_ZeroIsAPublishedValueNotAMissingOne guards the obvious
// way to get this wrong: 0 is falsy in most languages and is a perfectly
// ordinary published figure (an inflation rate of exactly 0.0 is a real
// measurement, not an absence). The refusal must key on JSON null --
// which the *float64 wire pointer distinguishes -- never on the numeric
// value.
func TestDecodeSeries_ZeroIsAPublishedValueNotAMissingOne(t *testing.T) {
	body := wireBody([]string{wireRow(2026, "T1", 0.0)})

	result, err := ine.DecodeSeries([]byte(body), "test-zero-is-a-value", indicators.FrequencyQuarterly)
	if err != nil {
		t.Fatalf("expected a published 0.0 to decode normally, got: %v", err)
	}
	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(result.Observations))
	}
	if result.Observations[0].Value == nil {
		t.Fatal("expected a non-nil value for a published 0.0")
	}
	if got := *result.Observations[0].Value; got != 0 {
		t.Errorf("expected the published value 0, got %v", got)
	}
}

// TestDecodeSeries_EveryDecodedObservationCarriesANonNilValue states the
// package-level invariant the refusal buys: once decodeAndNormalize
// returns successfully, ine.Observation.Value is never nil, so no caller
// -- the ingestion orchestrator, the observation writer, a future adapter
// consumer -- has to defend against one.
func TestDecodeSeries_EveryDecodedObservationCarriesANonNilValue(t *testing.T) {
	_, fixture := loadDatosSerieFixture(t)

	result, err := ine.DecodeSeries(fixture, "test-non-nil-invariant", indicators.FrequencyQuarterly)
	if err != nil {
		t.Fatalf("DecodeSeries: %v", err)
	}
	if len(result.Observations) == 0 {
		t.Fatal("expected the recorded fixture to yield observations")
	}
	for _, o := range result.Observations {
		if o.Value == nil {
			t.Errorf("observation at %s carries a nil value, which decode must make impossible", o.Period)
		}
	}
}
