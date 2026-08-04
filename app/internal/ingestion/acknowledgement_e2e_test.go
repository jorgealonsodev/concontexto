package ingestion_test

// RED for the acknowledgement registry END TO END (spec data-validation,
// "Acknowledged findings" / "An acknowledged publish is distinguishable
// from a clean one"): the real editorial YAML shape -> the real
// ReconcileEditorialConfig -> validation_acknowledgement -> a real
// IngestSeries against a real Postgres, a real INE client and the real
// six-rule set.
//
// The scenario is the portal's actual live problem, not a synthetic one.
// ocupados-epa is blocked on every real ingest by rule3-plausibility: a
// period-over-period fall of 1074.1 at 2020-Q2 against a configured
// max_delta_abs of 1000, with no covering break. The jump is real -- it is
// the COVID-19 lockdown quarter -- and neither of the two remedies that
// existed before this registry was honest:
//
//   - raising max_delta_abs blinds the guard permanently (exactly ONE of
//     the series' 97 historical deltas breaches 1000; the median is 152.4
//     and the second largest is 770.9, the 2009 financial crisis), and
//   - recording it in config/rupturas.yaml falsifies a registry whose own
//     header declares it holds "rupturas metodológicas". The employment
//     collapse was real economics, not a change of method.
//
// Reuses ingest_test.go's fixtures/helpers (newTx, sixSeries,
// seedDimensions, ineIngestConfig, loadFixture) and
// logging_alerting_test.go's capturingHandler rather than duplicating
// them.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// covidPinnedValue is INE's published 2020-Q2 figure for ocupados-epa, in
// thousands of persons -- the exact number config/reconocimientos.yaml
// pins and the fixture below delivers.
const covidPinnedValue = 18607.2

// ocupadosCovidFixture reads testdata/datos_serie/ocupados-epa-covid.json
// (real INE data, 2019-Q4 / 2020-Q1 / 2020-Q2; see that directory's
// source.txt) and returns its bytes plus the COD its own "COD" field
// carries -- the same no-Go-string-literal discipline loadFixture uses.
func ocupadosCovidFixture(t *testing.T) (cod string, body []byte) {
	t.Helper()
	path := filepath.Join("testdata", "datos_serie", "ocupados-epa-covid.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	var wire struct {
		COD string `json:"COD"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("parsing COD out of fixture %s: %v", path, err)
	}
	return wire.COD, body
}

// ocupadosCovidCase is the series identity for the run below. Its
// thresholds are NOT stated here: ineIngestConfig reads them out of
// config/series/ocupados-epa.yaml itself (realValidationConfig).
//
// They used to be restated as Go literals right here -- `maxDelta :=
// 1000.0`, matching the YAML by hand. That made every test in this file
// insensitive to the very file they are about: raising max_delta_abs in
// config/series/ocupados-epa.yaml above 1074.1 would blind the production
// guard permanently and leave TestIngestSeries_WithoutAnAcknowledgement-
// TheCovidQuarterStillBlocks green, still "proving" a block that no longer
// happens. A restated threshold is a second source of truth, and this
// whole file exists because of what that one threshold decides.
func ocupadosCovidCase() sixSeriesCase {
	return sixSeriesCase{
		slug: "ocupados-epa", datasetID: "ine-epa",
		unit: "miles de personas", frequency: indicators.FrequencyQuarterly, decimals: 1,
	}
}

// covidAcknowledgementConfig is the editorial entry in its SIGNED shape,
// which is the shape config/reconocimientos.yaml now ships (the record was
// reviewed and signed by the repository's maintainer; see
// TestRealAcknowledgementRegistry_ShipsExactlyOneRecordSignedByARealHuman).
// The signer here is a fixture name and deliberately not the real one: this
// file tests the MECHANISM, and pinning the live signer's name into a unit
// test would make an editorial fact into a test dependency.
func covidAcknowledgementConfig() config.AcknowledgementConfig {
	a := covidDraftConfig()
	on := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	a.SignatureStatus, a.Todo = "", ""
	a.AcknowledgedBy, a.AcknowledgedOn = "Ada Lovelace", &on
	return a
}

// covidDraftConfig is the entry in the shape it had while it was still
// awaiting review: researched and drafted by an agent, unsigned. The shipped
// registry has moved past this state, but the state itself must stay
// exercised -- a draft is a legitimate thing to check in, and what must
// remain impossible is a draft resolving anything.
func covidDraftConfig() config.AcknowledgementConfig {
	value := covidPinnedValue
	return config.AcknowledgementConfig{
		ID: "ocupados-epa-2020-q2-covid", Series: "ocupados-epa", Period: "2020-Q2",
		Rule: "rule3-plausibility", Value: &value,
		SignatureStatus: "unsigned",
		DraftedBy:       "Claude (agent), under delegated authority",
		Todo:            "Revisar la nota de prensa del INE citada y firmar el registro.",
		NoteMD:          "Confinamiento por la COVID-19; caída económica real, no un cambio metodológico.",
		SourceURL:       "https://www.ine.es/daco/daco42/daco4211/epa0220.pdf",
		FilePath:        "reconocimientos.yaml",
	}
}

// runCovidIngest wires the whole pipeline: optional editorial reconcile,
// then a real IngestSeries against a real INE-shaped server, capturing the
// structured log.
func runCovidIngest(t *testing.T, ctx context.Context, tx pgx.Tx, acks []config.AcknowledgementConfig) (ingestion.Result, *capturingHandler) {
	t.Helper()

	sc := ocupadosCovidCase()
	cod, fixture := ocupadosCovidFixture(t)
	seedDimensions(t, ctx, tx, sc, cod)

	if _, err := ingestion.ReconcileEditorialConfig(ctx, tx, config.Config{Acknowledgements: acks}); err != nil {
		t.Fatalf("ReconcileEditorialConfig: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	icfg := ineIngestConfig(t, sc, cod)

	handler := &capturingHandler{}
	prior := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(prior)

	result, err := ingestion.IngestSeries(ctx, tx, filestore.NewStore(t.TempDir()),
		ine.NewClient(server.URL, server.Client()), icfg,
		time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("IngestSeries: %v", err)
	}
	return result, handler
}

// TestIngestSeries_WithoutAnAcknowledgementTheCovidQuarterStillBlocks is
// the control, and it is what makes every assertion in the next test mean
// something: with an empty registry the real pipeline really does block
// this real series on this real quarter, exactly as it does in production
// today.
func TestIngestSeries_WithoutAnAcknowledgementTheCovidQuarterStillBlocks(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	result, handler := runCovidIngest(t, ctx, tx, nil)

	if result.Outcome != validation.GateBlock {
		t.Fatalf("expected the un-acknowledged COVID quarter to block, got %v", result.Outcome)
	}
	if len(result.Published) != 0 {
		t.Errorf("a blocked run must write nothing, got %d observations", len(result.Published))
	}
	assertRunOutcome(t, ctx, tx, result.RunID, string(postgres.RunOutcomeValidationFailed))

	attrs := handler.attrValues(t, 0)
	if _, present := attrs["acknowledgements"]; present {
		t.Errorf("a run with no acknowledgements must not log an acknowledgements attribute, got %v", attrs["acknowledgements"])
	}
}

// TestIngestSeries_AnAcknowledgementPublishesTheCovidQuarterAsAnOverride
// is the whole mechanism, end to end, including every auditability
// property spec data-validation requires: the series publishes, the
// recorded outcome says it was overridden rather than validated, and the
// structured log says so too, naming the record and the human.
func TestIngestSeries_AnAcknowledgementPublishesTheCovidQuarterAsAnOverride(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	result, handler := runCovidIngest(t, ctx, tx, []config.AcknowledgementConfig{covidAcknowledgementConfig()})

	if result.Outcome != validation.GatePublishOverridden {
		t.Fatalf("expected the acknowledged run to publish as an override, got %v", result.Outcome)
	}
	if len(result.Published) != 3 {
		t.Fatalf("expected all three fixture observations to be published, got %d", len(result.Published))
	}

	// AUDITABILITY, half 1: the recorded outcome. Querying ingestion_run
	// alone -- months later, with no log retention -- must distinguish this
	// from a clean pass.
	assertRunOutcome(t, ctx, tx, result.RunID, string(postgres.RunOutcomeSucceededWithAcknowledgement))

	// AUDITABILITY, half 2: the structured log.
	attrs := handler.attrValues(t, 0)
	if attrs["outcome"] != string(validation.GatePublishOverridden) {
		t.Errorf("expected the log outcome to be %q, not a plain publish, got %v",
			validation.GatePublishOverridden, attrs["outcome"])
	}
	acks, _ := attrs["acknowledgements"].([]string)
	if len(acks) != 1 {
		t.Fatalf("expected exactly one logged acknowledgement, got %v", attrs["acknowledgements"])
	}
	for _, want := range []string{"overridden", "ocupados-epa-2020-q2-covid", "Ada Lovelace", "rule3-plausibility", "2020-Q2"} {
		if !strings.Contains(acks[0], want) {
			t.Errorf("expected the logged override to contain %q, got %q", want, acks[0])
		}
	}
	// The rule that fired must NOT be reported as a failed rule of a run
	// that published -- that would make the log contradict its own outcome.
	if failed, present := attrs["failed_rules"]; present {
		t.Errorf("an acknowledged publish has no failed rules, got %v", failed)
	}
	// ...but the original finding, with its magnitude, must still be there.
	// "1074.0" rather than "1074.1": rule 3 reports the float subtraction
	// verbatim (1074.0999999999985), which is exactly the point -- the log
	// carries the magnitude the machine actually computed, not a rounded
	// retelling. The acknowledgement's own pin is compared against the
	// OBSERVED VALUE (18607.2), never against a derived delta, which is why
	// exact equality is safe there and would not have been here.
	verdicts, _ := attrs["verdicts"].([]string)
	var sawMagnitude bool
	for _, v := range verdicts {
		if strings.Contains(v, "rule3-plausibility") && strings.Contains(v, "1074.0") {
			sawMagnitude = true
		}
	}
	if !sawMagnitude {
		t.Errorf("expected the acknowledged finding to stay reported verbatim, with its magnitude, got %v", verdicts)
	}

	// The datum itself really is published and current.
	var value float64
	if err := tx.QueryRow(ctx,
		`SELECT value FROM observation WHERE series_id='ocupados-epa' AND period='2020-Q2' AND is_current`).Scan(&value); err != nil {
		t.Fatalf("expected the acknowledged quarter to be current: %v", err)
	}
	if value != covidPinnedValue {
		t.Errorf("expected the published value to be %v, got %v", covidPinnedValue, value)
	}
}

// TestIngestSeries_AnUnsignedDraftLeavesTheCovidQuarterBlocked proves
// inertness against the REAL pipeline, not only in the pure gate: a record
// with all its research and its verified citation changes nothing at all
// until a human signs it.
//
// The shipped registry is now signed, so this is no longer the repository's
// live state -- which is precisely why the assertion has to stay. The
// property under test is that the RESEARCH never overrides anything on its
// own; if it ever did, the signature would have become decoration and the
// next drafted record would go live unreviewed.
func TestIngestSeries_AnUnsignedDraftLeavesTheCovidQuarterBlocked(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	result, handler := runCovidIngest(t, ctx, tx, []config.AcknowledgementConfig{covidDraftConfig()})

	if result.Outcome != validation.GateBlock {
		t.Fatalf("an unsigned draft must leave the series blocked, got %v", result.Outcome)
	}
	if len(result.Published) != 0 {
		t.Errorf("a blocked run must write nothing, got %d observations", len(result.Published))
	}
	if len(result.Overridden) != 0 {
		t.Errorf("an unsigned draft must override nothing, got %+v", result.Overridden)
	}
	assertRunOutcome(t, ctx, tx, result.RunID, string(postgres.RunOutcomeValidationFailed))

	// It must never reach the database at all -- the same discipline that
	// keeps an unconfirmed break date out of series_break.
	rows, err := postgres.ListAcknowledgements(ctx, tx)
	if err != nil {
		t.Fatalf("ListAcknowledgements: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("an unsigned draft must never be projected into validation_acknowledgement, got %+v", rows)
	}

	// And the log must not suggest anything was overridden.
	attrs := handler.attrValues(t, 0)
	if _, present := attrs["acknowledgements"]; present {
		t.Errorf("an unsigned draft must not appear as an override in the log, got %v", attrs["acknowledgements"])
	}
}

// TestReconcileEditorialConfig_ReportsAnUnsignedDraftAsPendingRatherThanDroppingIt:
// skipping the projection must be VISIBLE, exactly as an unconfirmed break
// date is reported in PendingBreakIDs rather than silently ignored. An
// operator has to be able to see that a record is waiting on a person.
func TestReconcileEditorialConfig_ReportsAnUnsignedDraftAsPendingRatherThanDroppingIt(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	result, err := ingestion.ReconcileEditorialConfig(ctx, tx, config.Config{
		Acknowledgements: []config.AcknowledgementConfig{covidDraftConfig()},
	})
	if err != nil {
		t.Fatalf("ReconcileEditorialConfig: %v", err)
	}
	if result.Acknowledgements != (postgres.ReconcileCounts{}) {
		t.Errorf("an unsigned draft must reconcile nothing, got %+v", result.Acknowledgements)
	}
	if len(result.PendingAcknowledgementIDs) != 1 || result.PendingAcknowledgementIDs[0] != "ocupados-epa-2020-q2-covid" {
		t.Fatalf("expected the unsigned draft reported as pending a signature, got %v", result.PendingAcknowledgementIDs)
	}
}

// TestReconcileEditorialConfig_RefusesAnUnsignedRecordEvenWhenOtherwiseProjectable
// pins the signature guard ON ITS OWN, independently of every other reason
// a record might be skipped.
//
// This test exists because of a mutation experiment. Removing the signature
// check from the reconcile did NOT fail the ordinary unsigned end-to-end
// test, because a well-formed draft also has no acknowledged_on and was
// skipped by the nil-date guard instead. Inertness was therefore resting on
// a guard that has nothing to do with signatures — if acknowledged_on ever
// became optional, unsigned records would silently go live and only the
// pure gate would still object.
//
// So the fixture here is deliberately a shape validate-config REJECTS: a
// record declared unsigned that nonetheless carries a date. The reconcile
// must refuse it anyway. That is the actual property worth pinning — the
// reconcile does not trust that the config gate ran, because "the schema
// would have caught it" is exactly the assumption that turns one skipped
// validation step into a published override nobody approved.
func TestReconcileEditorialConfig_RefusesAnUnsignedRecordEvenWhenOtherwiseProjectable(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	on := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	malformed := covidDraftConfig()
	malformed.AcknowledgedOn = &on // present, so only the SIGNATURE check can stop it

	// Guard the guard: this shape must genuinely be one validate-config
	// rejects, otherwise the scenario below is not the one it claims to be.
	cfg := config.Config{
		Series:           []config.SeriesConfig{{Slug: "ocupados-epa", Frequency: "Q", Dataset: "ine-epa", Source: "ine"}},
		Sources:          map[string]config.SourceConfig{"ine": {ID: "ine"}},
		Acknowledgements: []config.AcknowledgementConfig{malformed},
	}
	if len(config.Validate(&cfg)) == 0 {
		t.Fatal("fixture must be a shape validate-config rejects (unsigned AND dated)")
	}

	result, err := ingestion.ReconcileEditorialConfig(ctx, tx, cfg)
	if err != nil {
		t.Fatalf("ReconcileEditorialConfig: %v", err)
	}
	if result.Acknowledgements != (postgres.ReconcileCounts{}) {
		t.Errorf("an unsigned record must reconcile nothing even when otherwise well-formed, got %+v", result.Acknowledgements)
	}
	rows, err := postgres.ListAcknowledgements(ctx, tx)
	if err != nil {
		t.Fatalf("ListAcknowledgements: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("an unsigned record must never reach validation_acknowledgement, got %+v", rows)
	}
	if len(result.PendingAcknowledgementIDs) != 1 {
		t.Errorf("expected it reported as pending a signature, got %v", result.PendingAcknowledgementIDs)
	}
}

// TestIngestSeries_AStaleAcknowledgementBlocksAgainAndSaysWhy is the
// staleness guard proven through the whole stack: an acknowledgement
// pinned to a value the source no longer reports must not carry a human's
// old approval onto a number nobody reviewed, and the operator must be
// told that is what happened rather than watching the original finding
// reappear unexplained.
func TestIngestSeries_AStaleAcknowledgementBlocksAgainAndSaysWhy(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	stale := covidAcknowledgementConfig()
	revised := covidPinnedValue - 100 // the value a reviewer saw, since revised
	stale.Value = &revised

	result, handler := runCovidIngest(t, ctx, tx, []config.AcknowledgementConfig{stale})

	if result.Outcome != validation.GateBlock {
		t.Fatalf("a stale acknowledgement must fail closed, got %v", result.Outcome)
	}
	if len(result.Published) != 0 {
		t.Errorf("a blocked run must write nothing, got %d observations", len(result.Published))
	}
	assertRunOutcome(t, ctx, tx, result.RunID, string(postgres.RunOutcomeValidationFailed))

	attrs := handler.attrValues(t, 0)
	failed, _ := attrs["failed_rules"].([]string)
	var sawStale bool
	for _, r := range failed {
		if r == validation.RuleAcknowledgementStale {
			sawStale = true
		}
	}
	if !sawStale {
		t.Fatalf("expected %q among the failed rules so an operator learns the approval went stale, got %v",
			validation.RuleAcknowledgementStale, failed)
	}
	verdicts, _ := attrs["verdicts"].([]string)
	var explained bool
	for _, v := range verdicts {
		if strings.Contains(v, validation.RuleAcknowledgementStale) &&
			strings.Contains(v, "ocupados-epa-2020-q2-covid") &&
			strings.Contains(v, "18607.2") {
			explained = true
		}
	}
	if !explained {
		t.Errorf("expected the stale verdict to name the record and both values, got %v", verdicts)
	}
}

// TestIngestSeries_AnAcknowledgementNeverAppliesToAnotherSeries is the
// scope guarantee proven at the database boundary rather than only in the
// pure gate: even with the acknowledgement present and reconciled, a
// DIFFERENT series' identical finding is untouched by it.
func TestIngestSeries_AnAcknowledgementNeverAppliesToAnotherSeries(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	foreign := covidAcknowledgementConfig()
	foreign.ID, foreign.Series = "tasa-de-paro-epa-2020-q2-covid", "tasa-de-paro-epa"

	result, _ := runCovidIngest(t, ctx, tx, []config.AcknowledgementConfig{foreign})

	if result.Outcome != validation.GateBlock {
		t.Fatalf("an acknowledgement scoped to another series must not resolve ocupados-epa's finding, got %v", result.Outcome)
	}
	if len(result.Overridden) != 0 {
		t.Errorf("expected zero overrides, got %+v", result.Overridden)
	}
}

func assertRunOutcome(t *testing.T, ctx context.Context, tx pgx.Tx, runID int64, want string) {
	t.Helper()
	var got string
	if err := tx.QueryRow(ctx, `SELECT outcome FROM ingestion_run WHERE id=$1`, runID).Scan(&got); err != nil {
		t.Fatalf("reading run %d outcome: %v", runID, err)
	}
	if got != want {
		t.Errorf("expected ingestion_run.outcome=%q, got %q", want, got)
	}
}
