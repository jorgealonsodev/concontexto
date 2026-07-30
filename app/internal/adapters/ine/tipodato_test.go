package ine_test

// Task 2a.1/2a.2 (RED/GREEN): T3_TipoDato is carried verbatim through
// decodeAndNormalize/DecodeSeries into Observation.SourceStatus, WITHOUT
// classification or validation at that layer (spec source-ingestion-ine,
// "The source's data-type token is carried through to the domain"). Kept
// deliberately unvalidated here so this package's other unit tests
// (cadence_test.go, periodicity_test.go, fetchraw_test.go) — none of
// which supply a status token — keep exercising periodicity/cadence in
// isolation without also having to supply one.
//
// Task 2a.7/2a.8 (RED/GREEN): the FAIL-CLOSED classification itself
// happens one layer up, in Client.Decode (indicators.SourceClient's own
// entry point) — "Definitivo"->D, "Provisional"->P, anything else
// (including a genuinely missing token) -> sourceerr.SchemaDrift naming
// the series and the token, never silently defaulted to definitive (spec
// source-ingestion-ine, "An unknown data-type token fails closed").

import (
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func TestDecodeSeries_TipoDatoCarriesVerbatimTokenToSourceStatusUnvalidated(t *testing.T) {
	body := `{"Nombre": "Total Nacional", "Data": [
		{"Anyo": 2026, "T3_Periodo": "T1", "T3_TipoDato": "Provisional", "Valor": 10.0},
		{"Anyo": 2026, "T3_Periodo": "T2", "T3_TipoDato": "Definitivo", "Valor": 10.1},
		{"Anyo": 2026, "T3_Periodo": "T3", "Valor": 10.2}
	]}`

	result, err := ine.DecodeSeries([]byte(body), "test-tipodato-carry-through", indicators.FrequencyQuarterly)
	if err != nil {
		t.Fatalf("DecodeSeries: %v", err)
	}
	if len(result.Observations) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(result.Observations))
	}
	want := []string{"Provisional", "Definitivo", ""} // third row omits T3_TipoDato entirely
	for i, w := range want {
		if got := result.Observations[i].SourceStatus; got != w {
			t.Errorf("observation %d: expected SourceStatus %q, got %q", i, w, got)
		}
	}
}

func TestClientDecode_ClassifiesTipoDatoFailingClosedOnUnrecognisedOrMissingToken(t *testing.T) {
	client := ine.NewClient("https://example.test", nil)

	t.Run("Definitivo maps to the definitive domain status", func(t *testing.T) {
		body := `{"Nombre":"Total Nacional","Data":[{"Anyo":2026,"T3_Periodo":"T1","T3_TipoDato":"Definitivo","Valor":10.0}]}`
		result, err := client.Decode([]byte(body), "test-status-definitivo", indicators.FrequencyQuarterly)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if len(result.Observations) != 1 {
			t.Fatalf("expected 1 observation, got %d", len(result.Observations))
		}
		obs := result.Observations[0]
		if obs.Status != indicators.ObservationStatusDefinitive {
			t.Errorf("expected Status D, got %q", obs.Status)
		}
		if obs.SourceStatus != "Definitivo" {
			t.Errorf("expected SourceStatus %q, got %q", "Definitivo", obs.SourceStatus)
		}
	})

	t.Run("Provisional maps to the provisional domain status", func(t *testing.T) {
		body := `{"Nombre":"Total Nacional","Data":[{"Anyo":2026,"T3_Periodo":"T1","T3_TipoDato":"Provisional","Valor":10.0}]}`
		result, err := client.Decode([]byte(body), "test-status-provisional", indicators.FrequencyQuarterly)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if result.Observations[0].Status != indicators.ObservationStatusProvisional {
			t.Errorf("expected Status P, got %q", result.Observations[0].Status)
		}
	})

	t.Run("an unrecognised token fails as schema drift naming the series and the token", func(t *testing.T) {
		body := `{"Nombre":"Total Nacional","Data":[{"Anyo":2026,"T3_Periodo":"T1","T3_TipoDato":"Estimado","Valor":10.0}]}`
		_, err := client.Decode([]byte(body), "test-status-unrecognised", indicators.FrequencyQuarterly)
		assertSchemaDrift(t, err)
		if !strings.Contains(err.Error(), "test-status-unrecognised") {
			t.Errorf("expected the error to name the series, got: %v", err)
		}
		if !strings.Contains(err.Error(), "Estimado") {
			t.Errorf("expected the error to name the unrecognised token, got: %v", err)
		}
	})

	t.Run("a missing token fails rather than defaulting to definitive", func(t *testing.T) {
		body := `{"Nombre":"Total Nacional","Data":[{"Anyo":2026,"T3_Periodo":"T1","Valor":10.0}]}`
		_, err := client.Decode([]byte(body), "test-status-missing", indicators.FrequencyQuarterly)
		assertSchemaDrift(t, err)
	})
}
