package ine_test

// Task 5a.11 (RED): periodicity is asserted when resolving an identifier
// (spec source-ingestion-ine, "Every pinned identifier MUST declare its
// expected periodicity, and ingestion MUST fail if the returned
// periodicity differs"). Tables 65962/72982 carry the same Nombre as
// 65109 but hold annual averages while 65109 is quarterly -- name alone
// cannot identify a series, so this assertion is the guard that catches
// a mis-pinned identifier.
//
// This test proves the mismatch using a verified MONTHLY response
// (IPC290751's real shape, Engram #4690: T3_Periodo "M06", Anyo 2026)
// against a QUARTERLY configuration, rather than a guessed "annual"
// shape: Engram #4690's live verification only confirmed INE's
// quarterly and monthly DATOS_SERIE shapes, never an annual one, and
// fabricating an unverified "real" annual fixture would risk asserting
// behaviour against fictional data. The mismatch-detection mechanism
// itself is periodicity-shape-generic (it compares whatever shape the
// response carries against the configured expectation), so a
// monthly-vs-quarterly mismatch is an equally strong proof of the
// requirement.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// monthlyFixtureBody is a small, real-shaped monthly DATOS_SERIE
// response (single period, matching the verified live value for
// IPC290751 2026-M06 = 103.598, Engram #4690). It is deliberately
// inline rather than a testdata/ fixture because its role here is a
// synthetic mismatch case (a quarterly-configured client receiving it),
// not a claim that this exact trimmed body was independently re-fetched
// for this test; the single verified real IPC290751 fixture with its
// own source.txt lives in testdata/datos_serie/ (5a.9's success path).
const monthlyFixtureBody = `{"Nombre": "Índice general nacional",
 "Data": [{"Anyo": 2026, "T3_Periodo": "M06", "Valor": 103.598, "Secreto": false}]}`

func TestFetchSeries_PeriodicityMismatchFailsAndNamesExpectedAndActual(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(monthlyFixtureBody))
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())

	// Configured as quarterly; the response is monthly.
	result, err := client.FetchSeries(context.Background(), "test-mismatch", indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected a periodicity-mismatch error, got nil")
	}
	if len(result.Observations) != 0 {
		t.Fatalf("expected zero observations on a periodicity-mismatch failure, got %d", len(result.Observations))
	}

	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SchemaDrift {
		t.Errorf("expected class %s, got %s", sourceerr.SchemaDrift, classified.Class)
	}
	if !strings.Contains(err.Error(), string(indicators.FrequencyQuarterly)) {
		t.Errorf("expected the error to name the expected periodicity %q, got %q", indicators.FrequencyQuarterly, err.Error())
	}
	if !strings.Contains(err.Error(), string(indicators.FrequencyMonthly)) {
		t.Errorf("expected the error to name the actual periodicity %q, got %q", indicators.FrequencyMonthly, err.Error())
	}
}

// annualFixtureBody is a small, real-shaped annual DATOS_SERIE response
// (two periods, matching the verified live values for EPA634676 --
// 21653.9 for 2024, 22221.1 for 2025, Engram-worthy verification done
// 2026-07-28: GET .../DATOS_SERIE/EPA634676?nult=2&tip=A). INE's annual
// cadence code is the single letter "A". EPA634676 is the annual-average
// Ocupados series from table 65962 -- the very table that shares its
// Nombre with the quarterly 65109 (task 5a.11's disambiguation gotcha).
// Deliberately inline rather than a testdata/ fixture, same rationale as
// monthlyFixtureBody above: this is a synthetic mismatch case (a
// quarterly-configured client receiving it), not an independent
// same-batch re-fetch claim.
const annualFixtureBody = `{"Nombre": "Total Nacional. Ambos sexos. Total. Ocupados. Valor absoluto. ",
 "Data": [{"Anyo": 2024, "T3_Periodo": "A", "Valor": 21653.9, "Secreto": false}
 ,{"Anyo": 2025, "T3_Periodo": "A", "Valor": 22221.1, "Secreto": false}]}`

// TestFetchSeries_UnrecognisedPeriodicityNamesTheRawCode proves the
// periodicity-mismatch error surfaces the RAW T3_Periodo code as the
// "actual" periodicity when detectPeriodicity cannot classify it (INE's
// annual code "A" is the live-verified example -- see annualFixtureBody),
// rather than an empty string. Before this fix, an operator saw
// "expected Q, got " with the actual cadence invisible; the spec's own
// scenario requires the failure to name BOTH the expected AND the actual
// periodicity (spec source-ingestion-ine, "A periodicity mismatch fails
// the run").
func TestFetchSeries_UnrecognisedPeriodicityNamesTheRawCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(annualFixtureBody))
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())

	// Configured as quarterly; the response is annual (unrecognised
	// shape -- FrequencyAnnual is out of Fase 0 scope, see periodicity.go).
	_, err := client.FetchSeries(context.Background(), "test-annual-mismatch", indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected a periodicity-mismatch error, got nil")
	}

	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SchemaDrift {
		t.Errorf("expected class %s, got %s", sourceerr.SchemaDrift, classified.Class)
	}
	if !strings.Contains(err.Error(), string(indicators.FrequencyQuarterly)) {
		t.Errorf("expected the error to name the expected periodicity %q, got %q", indicators.FrequencyQuarterly, err.Error())
	}
	if !strings.Contains(err.Error(), "got A") {
		t.Errorf("expected the error to surface the raw T3_Periodo code %q as the actual periodicity, got %q", "A", err.Error())
	}
}

func TestFetchSeries_MatchingPeriodicityProceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(monthlyFixtureBody))
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())

	// Configured as monthly; the response is monthly -- must proceed.
	result, err := client.FetchSeries(context.Background(), "test-match", indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("expected a matching periodicity to proceed, got error: %v", err)
	}
	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(result.Observations))
	}
	if got := result.Observations[0].Period.String(); got != "2026-06" {
		t.Errorf("expected normalized period 2026-06, got %s", got)
	}
	if result.Observations[0].Value == nil || *result.Observations[0].Value != 103.598 {
		t.Errorf("expected value 103.598, got %v", result.Observations[0].Value)
	}
}
