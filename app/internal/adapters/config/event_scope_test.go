package config_test

// The SCOPE and the CITATION an event registry entry may declare.
//
// WHAT THE SCOPE IS FOR. postgres.ListActiveEvents used to accept a seriesID
// it did not read, and said so in its own doc comment: migration 0001's
// `event` table carried no scope columns, so every active event was global
// by construction, and the parameter existed "to leave room for a real
// per-series scope in a future slice". Migration 0007 added the columns and
// closed that gap. Every entry the registry holds today is still global — a
// change of government and a worldwide shock are facts about the calendar
// and apply wherever the calendar does — but "global" is now a value an
// entry STATES, and an entry that applies to one series, dataset or source
// has somewhere to say so.
//
// The widening rule is the break registry's, reused rather than re-decided:
// series ⊂ dataset ⊂ source (config.scopeRefResolves,
// postgres.ResolveActiveBreaksForSeries).

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

// A scoped, cited entry round-trips through the loader with both fields
// intact. No file in config/ exercises either field today, which is exactly
// why it is tested here: a field nothing populates that silently stopped
// parsing would be caught by no other gate.
func TestLoad_EventCarriesAnExplicitScopeAndCitation(t *testing.T) {
	fsys := fstest.MapFS{"eventos.yaml": &fstest.MapFile{Data: []byte(`
- id: cambio-metodologico-ecoicop
  group: milestones
  name: "Cambio metodológico ECOICOP"
  scope: { kind: dataset, ref: ine-epa }
  date_start: 2021-12-31
  source_url: "https://www.ine.es/metodologia"
  note_md: "Nota metodológica publicada por la fuente."
`)}}

	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(cfg.Events), cfg.Events)
	}

	ev := cfg.Events[0]
	if ev.Scope.Kind != "dataset" || ev.Scope.Ref != "ine-epa" {
		t.Errorf("Events[0].Scope = %+v, want dataset/ine-epa", ev.Scope)
	}
	if ev.SourceURL != "https://www.ine.es/metodologia" {
		t.Errorf("Events[0].SourceURL = %q, want the methodology URL", ev.SourceURL)
	}
	wantStart := time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC)
	if ev.DateStart == nil || !ev.DateStart.Equal(wantStart) {
		t.Errorf("Events[0].DateStart = %v, want %v", ev.DateStart, wantStart)
	}
	if ev.FilePath != "eventos.yaml" {
		t.Errorf("Events[0].FilePath = %q, want eventos.yaml", ev.FilePath)
	}
}

// An entry that declares no scope at all is GLOBAL, and says so as a real
// value rather than as an empty string. That normalisation is the loader's,
// not the reader's: every consumer downstream (the digest, the reconcile,
// the SQL predicate) would otherwise have to re-decide what "" means, and
// the one that decided differently would be the one a reader met.
func TestLoad_EventWithoutScopeIsGlobal(t *testing.T) {
	fsys := fstest.MapFS{"gobiernos.yaml": &fstest.MapFile{Data: []byte(`
- id: gobierno-x
  name: "X"
  date_start: 1996-05-05
`)}}

	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(cfg.Events))
	}
	if cfg.Events[0].Scope.Kind != "global" {
		t.Errorf("Events[0].Scope.Kind = %q, want global", cfg.Events[0].Scope.Kind)
	}
	if cfg.Events[0].Scope.Ref != "" {
		t.Errorf("Events[0].Scope.Ref = %q, want empty for a global scope", cfg.Events[0].Scope.Ref)
	}
}

func TestValidate_EventScopeCoherence(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		wantViolation string // "" means the entry must validate cleanly
	}{
		{
			name: "an entry scoped to a configured dataset validates",
			yaml: `
- id: e1
  group: milestones
  name: "Hito"
  scope: { kind: dataset, ref: ine-epa }
  date_start: 2021-12-31
`,
		},
		{
			name: "an entry scoped to a configured series validates",
			yaml: `
- id: e1
  group: milestones
  name: "Hito"
  scope: { kind: series, ref: tasa-de-paro-epa }
  date_start: 2021-12-31
`,
		},
		{
			// The default, and what every entry authored today takes: a bare
			// entry means "applies wherever the calendar does".
			name: "an entry with no scope at all validates as global",
			yaml: `
- id: e1
  group: exogenous
  name: "Shock"
  date_start: 2022-02-24
`,
		},
		{
			name: "a scope ref that resolves to no configured series is rejected",
			yaml: `
- id: e1
  group: milestones
  name: "Hito"
  scope: { kind: series, ref: no-such-series }
  date_start: 2021-12-31
`,
			wantViolation: "scope.ref",
		},
		{
			name: "a narrow scope kind with no ref names nothing and is rejected",
			yaml: `
- id: e1
  group: milestones
  name: "Hito"
  scope: { kind: dataset }
  date_start: 2021-12-31
`,
			wantViolation: "scope.ref",
		},
		{
			name: "an unrecognised scope kind is rejected",
			yaml: `
- id: e1
  group: milestones
  name: "Hito"
  scope: { kind: continente, ref: europa }
  date_start: 2021-12-31
`,
			wantViolation: "scope.kind",
		},
		{
			// A global scope names nothing, so a ref beside it is a
			// contradiction the reader would never see resolved.
			name: "a global scope carrying a ref is rejected",
			yaml: `
- id: e1
  group: exogenous
  name: "Shock"
  scope: { kind: global, ref: ine-epa }
  date_start: 2022-02-24
`,
			wantViolation: "scope.ref",
		},
		{
			name: "an unrecognised group is rejected",
			yaml: `
- id: e1
  group: opiniones
  name: "..."
  date_start: 2022-02-24
`,
			wantViolation: "group",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"eventos.yaml":                 &fstest.MapFile{Data: []byte(tc.yaml)},
				"sources/ine.yaml":             &fstest.MapFile{Data: []byte(completeSourceYAML())},
				"series/tasa-de-paro-epa.yaml": &fstest.MapFile{Data: []byte(epaSeriesYAML())},
			}
			cfg, err := config.Load(fsys)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			violations := config.Validate(cfg)

			found := ""
			for _, v := range violations {
				if v.Field == tc.wantViolation {
					found = v.Field
				}
			}
			if tc.wantViolation == "" {
				for _, v := range violations {
					if v.File == "eventos.yaml" {
						t.Fatalf("expected no violation for eventos.yaml, got %v", v)
					}
				}
				return
			}
			if found == "" {
				t.Fatalf("expected a violation on field %q, got %v", tc.wantViolation, violations)
			}
		})
	}
}

// epaSeriesYAML is a minimal, schema-complete series config whose only job
// here is to give `dataset: ine-epa` and `slug: tasa-de-paro-epa` something
// real to resolve against.
func epaSeriesYAML() string {
	return `
slug: tasa-de-paro-epa
name: "Tasa de paro (EPA)"
source: ine
dataset: ine-epa
unit: "%"
frequency: Q
decimals: 2
geo: ES
source_refs:
  - kind: ine-series-cod
    ref: EPA453100
    valid_from: 2002-01-01
`
}
