// Package probe resolves which live endpoints the synthetic daily probe
// (spec pipeline-operations, "Synthetic daily probe against every
// endpoint": "A synthetic daily probe MUST issue a minimal request to
// every configured source endpoint ... and MUST assert response SHAPE
// only, never values, so identifier churn is detected before the
// ingestion window") must call.
//
// Targets is the pure, offline-testable half (task 9.3): it reads
// EXCLUSIVELY from an already-loaded *config.Config -- never a Go
// literal -- so a genuinely broken origin identifier (a churned INE COD,
// a discontinued Eurostat dataset code) is caught by resolving the SAME
// configuration production ingestion resolves, not a second, potentially
// stale copy (this batch's own constraint: "the probe must read its
// endpoints from the embedded config, not from hard-coded strings" --
// which is also what makes it a genuine churn detector, not a
// tautology). The live-tagged suite that actually calls each Target
// (app/internal/probe/live_test.go, `//go:build live`) is a separate
// file specifically so the default `go test ./...` never compiles it,
// let alone runs it (task 9.3's own "never runs in the default test
// suite" scenario).
package probe

import "github.com/jorgealonsodev/concontexto/app/internal/adapters/config"

// Target is one endpoint the probe must call.
type Target struct {
	SeriesSlug string
	SourceID   string
	Kind       string // "ine-series-cod" | "eurostat-dataset"
	Ref        string
	BaseURL    string
	Filters    map[string]string // eurostat-dataset only; nil for ine-series-cod
	Frequency  string
}

// Targets resolves every ACTIVE (ValidTo == nil) ine-series-cod and
// eurostat-dataset source_ref across cfg's series into a probe Target.
// xlsx-url refs are not probed here: the spec's own probe requirement
// names INE nult=1 and Eurostat lastTimePeriod=1 specifically, and an
// xlsx-url has no equivalent "last period only" query parameter to probe
// with -- probing it would mean downloading the full workbook, which is
// exactly the cost the probe exists to avoid.
func Targets(cfg *config.Config) []Target {
	var out []Target
	for _, s := range cfg.Series {
		src, ok := cfg.Sources[s.Source]
		if !ok || src.API == nil {
			continue
		}
		for _, ref := range s.SourceRefs {
			if ref.ValidTo != nil {
				continue
			}
			switch ref.Kind {
			case "ine-series-cod", "eurostat-dataset":
				out = append(out, Target{
					SeriesSlug: s.Slug,
					SourceID:   s.Source,
					Kind:       ref.Kind,
					Ref:        ref.Ref,
					BaseURL:    src.API.BaseURL,
					Filters:    ref.Filters,
					Frequency:  s.Frequency,
				})
			}
		}
	}
	return out
}
