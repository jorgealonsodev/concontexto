package ingestion_test

// Task 1.8 (triage): runs a full-history population (ECP320) payload --
// semiannual across its historical span, quarterly recently, exactly the
// verified real shape (exploration.md §4) -- through the real IngestSeries
// pipeline twice: once with the CORRECTED config/series/poblacion-residente.yaml
// cadence_segments declaration (must publish), and once with the cadence
// declared uniformly quarterly, mirroring Fase 0's config (must now be
// rejected). This is the concrete, end-to-end proof behind the D4
// consequence: "Fixing the guard may reject data that previously ingested.
// That is the correct outcome, not a regression" (spec data-validation).
//
// The other five pinned series are not repeated here: they are dense,
// uniformly-cadenced series with no known interior gap, already proven
// end to end (unchanged) by TestIngestSeries_AllSixSeriesLoadTheirTrimmedFixtureHistoryAndValidate
// in this same package. This offline session has no live network access,
// so this fixture is a constructed-but-realistic full history matching
// the verified boundary facts recorded in config/series/poblacion-residente.yaml's
// own comment (1977-1980 semiannual, continuous quarterly from 2023-Q3),
// not a byte-for-byte capture of INE's real DATOS_SERIE/ECP320 response.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// populationFullHistoryFixture renders a synthetic-but-realistic ECP320
// DATOS_SERIE body: semiannual ("1 de enero de"/"1 de julio de") from
// 1977 through 2022, then ordinary "T1".."T4" quarterly from 2023-Q3
// through 2026-Q2 -- matching config/series/poblacion-residente.yaml's
// own declared boundary. Every row carries "T3_TipoDato": "Definitivo"
// (slice 2a: Client.Decode now classifies that token fail-closed, so a
// fixture omitting it would fail for an unrelated reason before ever
// reaching the cadence guard this test exists to prove).
func populationFullHistoryFixture(cod string) []byte {
	var rows []string
	for y := 1977; y <= 2022; y++ {
		rows = append(rows,
			fmt.Sprintf(`{"Anyo": %d, "T3_Periodo": "1 de enero de", "T3_TipoDato": "Definitivo", "Valor": %d}`, y, 30000000+y*1000),
			fmt.Sprintf(`{"Anyo": %d, "T3_Periodo": "1 de julio de", "T3_TipoDato": "Definitivo", "Valor": %d}`, y, 30000000+y*1000+500),
		)
	}
	// 2023-Q1 ("1 de enero de") is the LAST semiannual point before the
	// quarterly era begins at 2023-Q3 -- 2023-Q2 is never observed under
	// either regime, exactly matching config/series/poblacion-residente.yaml's
	// declared segment boundary (semiannual segment's "to": 2023-Q2).
	//
	// The values below continue the same arithmetic ramp as the semiannual
	// rows above, and that continuity is now load-bearing (verify-report
	// CRITICAL-37). This fixture used to step from 32,022,500 to 47,023,000
	// across the era boundary -- a 15-million jump, an artefact of the
	// generator that nothing in this file ever asserted on. It was harmless
	// only for as long as ineIngestConfig passed NO thresholds; under
	// config/series/poblacion-residente.yaml's real max_delta_abs of
	// 500,000 that step is a rule3-plausibility block, and this cadence test
	// would fail for a reason that has nothing to do with cadence. Making
	// the synthetic history plausible is the honest fix; suppressing the
	// series' real thresholds here to keep the old numbers would reinstate
	// exactly the blindness CRITICAL-37 is about.
	rows = append(rows, fmt.Sprintf(`{"Anyo": 2023, "T3_Periodo": "1 de enero de", "T3_TipoDato": "Definitivo", "Valor": %d}`, 30000000+2023*1000))
	for y, q := 2023, 3; ; {
		rows = append(rows, fmt.Sprintf(`{"Anyo": %d, "T3_Periodo": "T%d", "T3_TipoDato": "Definitivo", "Valor": %d}`, y, q, 30000000+y*1000+q*100))
		if y == 2026 && q == 2 {
			break
		}
		q++
		if q > 4 {
			q = 1
			y++
		}
	}
	return []byte(fmt.Sprintf(`{"COD": %q, "Nombre": "Total Nacional", "Data": [%s]}`, cod, strings.Join(rows, ",")))
}

func expectedPopulationObservationCount() int {
	semiannual := (2022-1977+1)*2 + 1 // +1 for the standalone 2023-Q1 point
	quarterly := 0
	for y, q := 2023, 3; ; {
		quarterly++
		if y == 2026 && q == 2 {
			break
		}
		q++
		if q > 4 {
			q = 1
			y++
		}
	}
	return semiannual + quarterly
}

func runPopulationIngest(t *testing.T, cadenceSegments []indicators.CadenceSegment) (ingestion.Result, error) {
	t.Helper()
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	const cod = "test-population-full-history"
	sc := sixSeriesCase{slug: "poblacion-residente", datasetID: "ine-ecp", unit: "personas", frequency: indicators.FrequencyQuarterly, decimals: 0}
	seedDimensions(t, ctx, tx, sc, cod)

	fixture := populationFullHistoryFixture(cod)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	icfg := ineIngestConfig(t, sc, cod)
	icfg.Series.CadenceSegments = cadenceSegments

	return ingestion.IngestSeries(ctx, tx, store, client, icfg, now)
}

// TestPopulationCadenceTriage_CorrectedSegmentedConfigPublishes proves the
// corrected config/series/poblacion-residente.yaml declaration (semiannual
// 1977-2022, quarterly from 2023-Q3) ingests its full constructed history
// cleanly under the corrected guards.
func TestPopulationCadenceTriage_CorrectedSegmentedConfigPublishes(t *testing.T) {
	semiannual, err := indicators.ParseCadenceSegment("1977-Q1", "2023-Q2", "semiannual", []int{1, 3})
	if err != nil {
		t.Fatalf("ParseCadenceSegment: %v", err)
	}
	quarterly, err := indicators.ParseCadenceSegment("2023-Q3", "", "quarterly", nil)
	if err != nil {
		t.Fatalf("ParseCadenceSegment: %v", err)
	}

	result, err := runPopulationIngest(t, []indicators.CadenceSegment{semiannual, quarterly})
	if err != nil {
		t.Fatalf("IngestSeries with the corrected segmented cadence: %v", err)
	}
	if result.Outcome != validation.GatePublish {
		t.Fatalf("expected the corrected config to publish, got outcome=%v findings=%+v", result.Outcome, result.Findings)
	}
	if want := expectedPopulationObservationCount(); len(result.Published) != want {
		t.Fatalf("expected %d published observations, got %d", want, len(result.Published))
	}
}

// TestPopulationCadenceTriage_UncorrectedUniformConfigNowFails proves the
// SAME full history, declared uniformly quarterly (Fase 0's shape, no
// cadence_segments), is now rejected by the corrected guards -- exactly
// the D4 consequence the spec accepts as correct, not a regression (spec
// editorial-config, "The uncorrected configuration now fails" / spec
// data-validation, "Fixing the guard may reject data that previously
// ingested").
func TestPopulationCadenceTriage_UncorrectedUniformConfigNowFails(t *testing.T) {
	_, err := runPopulationIngest(t, nil)
	if err == nil {
		t.Fatal("expected the uncorrected uniform-quarterly declaration to fail against the real mixed-cadence history")
	}
	if !strings.Contains(err.Error(), "quarterly") {
		t.Errorf("expected the failure to name the declared cadence, got: %v", err)
	}
}
