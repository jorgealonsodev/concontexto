package ine_test

// Task 5a.9 (RED): ingestion uses DATOS_SERIE per canonical series (ADR-2/
// D5, spec source-ingestion-ine "Ingestion uses DATOS_SERIE per canonical
// series"). The COD in the checked-in fixture filename
// (testdata/datos_serie/EPA453100.json) is the series identity -- this
// test derives it from that filename rather than hard-coding it as a Go
// string literal, so the origin-identifier static-scan guard
// (app/internal/guard) stays satisfied without widening its allowlist:
// the real identifier lives only in testdata/, exactly as the guard
// already permits.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// loadDatosSerieFixture reads the one checked-in DATOS_SERIE success
// fixture and derives its series COD from the filename -- see the
// package-level doc comment on why this is not a Go string literal.
func loadDatosSerieFixture(t *testing.T) (cod string, body []byte) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join("testdata", "datos_serie", "*.json"))
	if err != nil {
		t.Fatalf("globbing DATOS_SERIE fixtures: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one DATOS_SERIE fixture, found %v", matches)
	}
	body, err = os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("reading fixture %s: %v", matches[0], err)
	}
	cod = strings.TrimSuffix(filepath.Base(matches[0]), ".json")
	return cod, body
}

func loadRefusalFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "volume_restriction", "refusal.json"))
	if err != nil {
		t.Fatalf("reading refusal fixture: %v", err)
	}
	return body
}

// requestCounter is a tiny httptest handler that classifies every
// request by its INE endpoint (DATOS_SERIE vs DATOS_TABLA) so tests can
// assert the ingestion path never touches the discovery-only endpoint
// (ADR-2/D5).
type requestCounter struct {
	datosSerie int
	datosTabla int
	respond    func(w http.ResponseWriter, r *http.Request)
}

func (c *requestCounter) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/DATOS_SERIE/"):
			c.datosSerie++
		case strings.HasPrefix(r.URL.Path, "/DATOS_TABLA/"):
			c.datosTabla++
		}
		c.respond(w, r)
	}
}

func TestFetchSeries_IssuesExactlyOneRequestToDatosSerieAndZeroToDatosTabla(t *testing.T) {
	cod, fixture := loadDatosSerieFixture(t)

	counter := &requestCounter{respond: func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}}
	server := httptest.NewServer(counter.handler())
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())

	result, err := client.FetchSeries(context.Background(), cod, indicators.FrequencyQuarterly)
	if err != nil {
		t.Fatalf("FetchSeries: %v", err)
	}

	if counter.datosSerie != 1 {
		t.Errorf("expected exactly 1 request to DATOS_SERIE, got %d", counter.datosSerie)
	}
	if counter.datosTabla != 0 {
		t.Errorf("expected 0 requests to DATOS_TABLA, got %d", counter.datosTabla)
	}

	// Real decoded content, not a smoke test: the fixture's three
	// observations (2025-Q4=9.93, 2026-Q1=10.83, 2026-Q2=9.87) must
	// round-trip through the client exactly.
	if result.COD != cod {
		t.Errorf("expected COD %s, got %s", cod, result.COD)
	}
	if len(result.Observations) != 3 {
		t.Fatalf("expected 3 observations, got %d: %+v", len(result.Observations), result.Observations)
	}
	want := []struct {
		period string
		value  float64
	}{
		{"2025-Q4", 9.93},
		{"2026-Q1", 10.83},
		{"2026-Q2", 9.87},
	}
	for i, w := range want {
		got := result.Observations[i]
		if got.Period.String() != w.period {
			t.Errorf("observation %d: expected period %s, got %s", i, w.period, got.Period.String())
		}
		if got.Value == nil || *got.Value != w.value {
			t.Errorf("observation %d: expected value %v, got %v", i, w.value, got.Value)
		}
	}
}
