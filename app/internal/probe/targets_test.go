package probe_test

// Task 9.3 (RED): Targets must resolve every probe-able endpoint FROM
// config, never from a Go literal (task constraint: "the probe must read
// its endpoints from the embedded config, not from hard-coded strings").
// This test builds its own synthetic *config.Config in memory -- no real
// origin identifier appears anywhere in this file, matching the guard's
// own convention (app/internal/guard/originidentifiers_test.go) even
// though this package is not itself exempted from the scan.

import (
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/probe"
)

func synthCfg() *config.Config {
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	return &config.Config{
		Sources: map[string]config.SourceConfig{
			"fake-api":         {ID: "fake-api", API: &config.APIConfig{BaseURL: "https://fake-api.invalid/api"}},
			"fake-xlsx-source": {ID: "fake-xlsx-source"}, // no API block: xlsx-url series
		},
		Series: []config.SeriesConfig{
			{
				Slug: "fake-ine-series", Source: "fake-api", Frequency: "Q",
				SourceRefs: []config.SourceRef{
					{Kind: "ine-series-cod", Ref: "FAKE001", ValidFrom: past, ValidTo: nil},
				},
			},
			{
				Slug: "fake-eurostat-series", Source: "fake-api", Frequency: "M",
				SourceRefs: []config.SourceRef{
					{Kind: "eurostat-dataset", Ref: "fake_dataset", Filters: map[string]string{"geo": "ZZ"}, ValidFrom: past, ValidTo: nil},
				},
			},
			{
				Slug: "fake-xlsx-series", Source: "fake-xlsx-source", Frequency: "M",
				SourceRefs: []config.SourceRef{
					{Kind: "xlsx-url", Ref: "https://fake.invalid/x.xlsx", ValidFrom: past, ValidTo: nil},
				},
			},
			{
				Slug: "fake-retired-ref-series", Source: "fake-api", Frequency: "Q",
				SourceRefs: []config.SourceRef{
					{Kind: "ine-series-cod", Ref: "RETIRED001", ValidFrom: past, ValidTo: &past},
				},
			},
		},
	}
}

func TestTargets_ResolvesEveryActiveAPIBackedSourceRefFromConfig(t *testing.T) {
	targets := probe.Targets(synthCfg())

	if len(targets) != 2 {
		t.Fatalf("expected exactly 2 probe targets (INE + Eurostat, xlsx and the retired ref excluded), got %d: %+v", len(targets), targets)
	}

	byKind := map[string]probe.Target{}
	for _, tgt := range targets {
		byKind[tgt.Kind] = tgt
	}

	ineTgt, ok := byKind["ine-series-cod"]
	if !ok {
		t.Fatal("expected an ine-series-cod target")
	}
	if ineTgt.Ref != "FAKE001" || ineTgt.BaseURL != "https://fake-api.invalid/api" || ineTgt.SeriesSlug != "fake-ine-series" {
		t.Errorf("unexpected INE target: %+v", ineTgt)
	}

	eurostatTgt, ok := byKind["eurostat-dataset"]
	if !ok {
		t.Fatal("expected a eurostat-dataset target")
	}
	if eurostatTgt.Ref != "fake_dataset" || eurostatTgt.Filters["geo"] != "ZZ" {
		t.Errorf("unexpected Eurostat target: %+v", eurostatTgt)
	}
}

func TestTargets_EmptyConfigResolvesNoTargets(t *testing.T) {
	targets := probe.Targets(&config.Config{})
	if len(targets) != 0 {
		t.Fatalf("expected zero targets for an empty config, got %d", len(targets))
	}
}
