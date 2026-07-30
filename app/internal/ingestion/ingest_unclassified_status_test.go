package ingestion_test

// Task 2b (closing slice 2a's disclosed compatibility shim): slice 2a's
// mapObservationStatus defaulted a never-classified (zero-value) Status
// to postgres.StatusDefinitive, scoped explicitly to adapters/eurostat
// -- whose own status decoding was not yet built. This slice builds it
// (envelope.go); every observation adapters/ine and adapters/eurostat
// now hand IngestSeries always carries an already-classified Status.
//
// This file proves the CLOSURE directly, independent of either real
// adapter: IngestSeries must fail the whole run closed -- never
// silently default to Definitive -- if ANY indicators.SourceClient
// (present or future) ever hands it an observation with a still-
// unclassified Status. A fake SourceClient is used deliberately, not a
// real one, because no currently-wired real adapter can still exercise
// this path (that IS the point: the fallback is provably unreachable
// for both governed sources now) -- see this slice's own apply-progress
// for the disclosed adapters/xlsx implication (out of scope here: xlsx
// carries no status concept at all yet, a pre-existing, disclosed gap
// this slice does not touch).

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// fakeUnclassifiedStatusClient satisfies indicators.SourceClient and
// returns one observation whose Status is left at its domain zero value
// -- simulating a SourceClient implementation that does not classify
// status at all, the only shape that can still reach the closed
// fallback this file proves.
type fakeUnclassifiedStatusClient struct{ body []byte }

func (fakeUnclassifiedStatusClient) RequestURL(ref string) string {
	return "https://fake.example.test/" + ref
}

func (c fakeUnclassifiedStatusClient) FetchRaw(context.Context, string) ([]byte, error) {
	return c.body, nil
}

func (fakeUnclassifiedStatusClient) Decode(raw []byte, ref string, expectedFrequency indicators.Frequency, segments ...indicators.CadenceSegment) (indicators.SourceResult, error) {
	period, err := indicators.NormalizePeriodLabel("2026-04")
	if err != nil {
		return indicators.SourceResult{}, err
	}
	value := 42.0
	return indicators.SourceResult{
		Name: "fake",
		// Status/SourceStatus deliberately left at their zero value.
		Observations: []indicators.Observation{{Period: period, Value: &value}},
	}, nil
}

var _ indicators.SourceClient = fakeUnclassifiedStatusClient{}

func TestIngestSeries_UnclassifiedStatusFailsClosedRatherThanDefaultingToDefinitive(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	const sourceID = "fake-unclassified-source"
	const datasetID = "fake-unclassified-dataset"
	const slug = "fake-unclassified-series"
	const cod = "FAKE001"

	mustExecT(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ($1, 'Fake', 'https://fake.example.test', 'lic', 'attr', 'api-json', 'digest1')`, sourceID)
	mustExecT(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest)
		VALUES ($1, $2, $1, 'digest1')`, datasetID, sourceID)
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ($1, $2, $1, 'indice', 'M', 'ES', 1, false, 'digest1')`, slug, datasetID)

	client := fakeUnclassifiedStatusClient{body: []byte(`{}`)}
	store := filestore.NewStore(t.TempDir())
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	cfg := ingestion.SeriesIngestConfig{
		SourceID: sourceID, DatasetID: datasetID, SeriesID: slug, COD: cod,
		Series: indicators.Series{Slug: slug, Unit: "indice", Frequency: indicators.FrequencyMonthly, Decimals: 1, Source: sourceID, Licence: "test"},
	}

	result, err := ingestion.IngestSeries(ctx, tx, store, client, cfg, now)
	if err == nil {
		t.Fatal("expected an unclassified status to fail IngestSeries rather than defaulting to definitive")
	}
	if !strings.Contains(err.Error(), slug) {
		t.Errorf("expected the error to name the series %q, got: %v", slug, err)
	}
	if result.Outcome != validation.GateBlock {
		t.Errorf("expected GateBlock (recorded via the same converging path a decode failure takes), got %v", result.Outcome)
	}

	var obsCount, runCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM observation`).Scan(&obsCount); err != nil {
		t.Fatalf("counting observation rows: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM ingestion_run`).Scan(&runCount); err != nil {
		t.Fatalf("counting ingestion_run rows: %v", err)
	}
	if obsCount != 0 {
		t.Errorf("expected zero observation rows on an unclassified status, got %d", obsCount)
	}
	if runCount != 1 {
		t.Errorf("expected the run to still be recorded (same ordering guarantee as a decode failure), got %d", runCount)
	}
}
