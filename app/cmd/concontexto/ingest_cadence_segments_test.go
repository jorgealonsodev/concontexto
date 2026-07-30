package main

// Task 1.2/1.6 (RED/GREEN companion): seriesCadenceSegments is the one
// place config.SeriesConfig.CadenceSegments (plain YAML strings/ints)
// becomes the domain indicators.CadenceSegment values IngestSeries'
// SourceClient.Decode and validation.Rule2Continuity both consume
// (design D-4). An empty config declaration must stay the ordinary,
// unaffected uniform-cadence case for five of the six milestone-0.2
// series.

import (
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func TestSeriesCadenceSegments_EmptyConfigYieldsNilSegments(t *testing.T) {
	segs, err := seriesCadenceSegments(config.SeriesConfig{Slug: "tasa-de-paro-epa", Frequency: "Q"})
	if err != nil {
		t.Fatalf("seriesCadenceSegments: %v", err)
	}
	if segs != nil {
		t.Errorf("expected nil segments for an unsegmented series, got %+v", segs)
	}
}

func TestSeriesCadenceSegments_ParsesDeclaredSegments(t *testing.T) {
	cfg := config.SeriesConfig{
		Slug: "poblacion-residente", Frequency: "Q",
		CadenceSegments: []config.CadenceSegmentConfig{
			{From: "1977-Q1", To: "2023-Q2", Cadence: "semiannual", Present: []int{1, 3}},
			{From: "2023-Q3", Cadence: "quarterly"},
		},
	}
	segs, err := seriesCadenceSegments(cfg)
	if err != nil {
		t.Fatalf("seriesCadenceSegments: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if segs[0].Cadence != "semiannual" || segs[0].OpenEnded {
		t.Errorf("expected segs[0] to be a closed semiannual segment, got %+v", segs[0])
	}
	if segs[1].Cadence != "quarterly" || !segs[1].OpenEnded {
		t.Errorf("expected segs[1] to be an open-ended quarterly segment, got %+v", segs[1])
	}
}

func TestSeriesCadenceSegments_UnparseableLabelErrorsNamingTheSeries(t *testing.T) {
	cfg := config.SeriesConfig{
		Slug: "poblacion-residente", Frequency: "Q",
		CadenceSegments: []config.CadenceSegmentConfig{{From: "not-a-period", Cadence: "quarterly"}},
	}
	_, err := seriesCadenceSegments(cfg)
	if err == nil {
		t.Fatal("expected an unparseable cadence segment label to error")
	}
	if !strings.Contains(err.Error(), "poblacion-residente") {
		t.Errorf("expected the error to name the series, got: %v", err)
	}
}
