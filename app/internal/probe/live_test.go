//go:build live

package probe_test

// Task 9.3/9.4: the synthetic daily probe itself. This file carries the
// `live` build tag SPECIFICALLY so `go test ./...` never compiles it,
// let alone runs it or opens a socket (spec "The probe never runs in the
// default test suite": "no probe test executes and no network call is
// made"). It is wired as its own scheduled workflow
// (.github/workflows/probe.yml), separate from ci.yml, per this batch's
// own instruction.
//
// Every identifier this file touches is resolved through probe.Targets
// from the real embedded /config tree -- never a Go literal -- matching
// the origin-identifier guard's own convention and this batch's explicit
// constraint ("the probe must read its endpoints from the embedded
// config, not from hard-coded strings").
//
// Shape, never values (spec "The probe asserts shape, not values"): each
// target's response is decoded through the SAME adapter Decode function
// production ingestion uses (ine.DecodeSeries / eurostat.Decode), which
// asserts structural shape and periodicity but inspects no specific
// numeric value -- exactly the property that makes a legitimately
// updated latest value a non-failure, while a renamed field, a changed
// periodicity, or the INE volume-restriction/Eurostat oversized-response
// envelopes still fail the probe (spec "The probe detects a broken
// identifier").
import (
	"context"
	"io/fs"
	"testing"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/probe"
)

func TestLiveProbe_EveryConfiguredEndpointRespondsWithValidShape(t *testing.T) {
	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	targets := probe.Targets(cfg)
	if len(targets) == 0 {
		t.Fatal("expected at least one probe target resolved from the embedded config")
	}

	ctx := context.Background()
	for _, tgt := range targets {
		tgt := tgt
		t.Run(tgt.SeriesSlug, func(t *testing.T) {
			freq := indicators.Frequency(tgt.Frequency)

			switch tgt.Kind {
			case "ine-series-cod":
				client := ine.NewClient(tgt.BaseURL, nil)
				raw, err := client.FetchProbe(ctx, tgt.Ref, 1)
				if err != nil {
					t.Fatalf("probe request for %s (source ref %s) failed -- possible identifier churn: %v", tgt.SeriesSlug, tgt.Ref, err)
				}
				if _, err := ine.DecodeSeries(raw, tgt.Ref, freq); err != nil {
					t.Fatalf("probe response for %s (source ref %s) failed shape/periodicity decode -- possible identifier churn: %v", tgt.SeriesSlug, tgt.Ref, err)
				}
			case "eurostat-dataset":
				client := eurostat.NewClient(tgt.BaseURL, tgt.Filters, nil)
				raw, err := client.FetchProbe(ctx, tgt.Ref, 1)
				if err != nil {
					t.Fatalf("probe request for %s (source ref %s) failed -- possible identifier churn: %v", tgt.SeriesSlug, tgt.Ref, err)
				}
				if _, err := eurostat.Decode(raw, tgt.Ref, freq); err != nil {
					t.Fatalf("probe response for %s (source ref %s) failed shape decode -- possible identifier churn: %v", tgt.SeriesSlug, tgt.Ref, err)
				}
			default:
				t.Fatalf("unexpected probe target kind %q", tgt.Kind)
			}
		})
	}
}
