package config_test

// The policy-measures registry (config/medidas.yaml) and the SCOPE the
// event registry gained in order to carry it.
//
// WHY THESE TWO THINGS ARE ONE TEST FILE. A measure is, structurally, an
// event: a stable id, a name, a date, a note. What it is NOT is transversal
// — a labour-market reform belongs on the EPA charts and is noise on an IPC
// chart — and until now `event` carried no scope columns at all, which
// postgres.ListActiveEvents' own package comment states plainly ("every
// currently active event is, by the schema this change inherited, global").
// So the registry and the scope arrive together: neither is useful alone.
//
// NOTHING HERE ASSERTS AN EFFECT. A measure entry carries an instrument, a
// date of entry into force, a citation and a scope. There is no field for an
// outcome, a direction, a magnitude or an evaluation, and none is derivable
// from what is stored — the same structural refusal `EventConfig` already
// applies to party colour (PRD §12.1).

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

// measuresYAML is one well-formed medidas.yaml: a scoped, cited measure and
// an unconfirmed one, mirroring the confirmed/unconfirmed pair every other
// editorial registry's loader test already exercises.
const measuresYAML = `
- id: rdl-32-2021-reforma-laboral
  name: "Real Decreto-ley 32/2021, de 28 de diciembre"
  scope: { kind: dataset, ref: ine-epa }
  date_start: 2021-12-31
  source_url: "https://www.boe.es/buscar/act.php?id=BOE-A-2021-21788"
  note_md: "Instrumento publicado en el BOE."
- id: medida-pendiente
  name: "Instrumento por confirmar"
  scope: { kind: dataset, ref: ine-epa }
  date_status: unconfirmed
  todo: "Confirmar la fecha de entrada en vigor en el BOE."
  source_url: "https://www.boe.es/"
  note_md: "Pendiente."
`

func TestLoad_MedidasYAMLParsesAsScopedMeasureEvents(t *testing.T) {
	fsys := fstest.MapFS{"medidas.yaml": &fstest.MapFile{Data: []byte(measuresYAML)}}

	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Events) != 2 {
		t.Fatalf("expected 2 events from medidas.yaml, got %d: %+v", len(cfg.Events), cfg.Events)
	}

	measure := cfg.Events[0]
	// The group is assigned by the LOADER, never repeated per entry — the
	// exact convention config/gobiernos.yaml already established: the whole
	// file is exactly one group.
	if measure.Group != "measures" {
		t.Errorf("Events[0].Group = %q, want measures", measure.Group)
	}
	if measure.Scope.Kind != "dataset" || measure.Scope.Ref != "ine-epa" {
		t.Errorf("Events[0].Scope = %+v, want dataset/ine-epa", measure.Scope)
	}
	if measure.SourceURL != "https://www.boe.es/buscar/act.php?id=BOE-A-2021-21788" {
		t.Errorf("Events[0].SourceURL = %q, want the BOE consolidated-text URL", measure.SourceURL)
	}
	wantStart := time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC)
	if measure.DateStart == nil || !measure.DateStart.Equal(wantStart) {
		t.Errorf("Events[0].DateStart = %v, want %v", measure.DateStart, wantStart)
	}
	if measure.FilePath != "medidas.yaml" {
		t.Errorf("Events[0].FilePath = %q, want medidas.yaml", measure.FilePath)
	}

	if cfg.Events[1].DateStatus != "unconfirmed" || cfg.Events[1].DateStart != nil {
		t.Errorf("Events[1] = %+v, want an unconfirmed entry carrying no date", cfg.Events[1])
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

func TestValidate_EventScopeAndMeasureRequirements(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		yaml          string
		wantViolation string // "" means the entry must validate cleanly
	}{
		{
			name: "a measure scoped to a configured dataset validates",
			file: "medidas.yaml",
			yaml: `
- id: m1
  name: "Instrumento"
  scope: { kind: dataset, ref: ine-epa }
  date_start: 2021-12-31
  source_url: "https://www.boe.es/x"
  note_md: "..."
`,
		},
		{
			// The whole reason the registry exists. A measure with no scope
			// would be published on every chart in the portal, which is the
			// noise this design set out to avoid.
			name: "a measure with no scope is rejected",
			file: "medidas.yaml",
			yaml: `
- id: m1
  name: "Instrumento"
  date_start: 2021-12-31
  source_url: "https://www.boe.es/x"
  note_md: "..."
`,
			wantViolation: "scope.kind",
		},
		{
			// Every measure is a legal instrument with a real citation. An
			// entry nobody can check is an editorial assertion, which is
			// exactly what this portal refuses to publish.
			name: "a measure with no source_url is rejected",
			file: "medidas.yaml",
			yaml: `
- id: m1
  name: "Instrumento"
  scope: { kind: dataset, ref: ine-epa }
  date_start: 2021-12-31
  note_md: "..."
`,
			wantViolation: "source_url",
		},
		{
			name: "a scope ref that resolves to no configured series is rejected",
			file: "medidas.yaml",
			yaml: `
- id: m1
  name: "Instrumento"
  scope: { kind: series, ref: no-such-series }
  date_start: 2021-12-31
  source_url: "https://www.boe.es/x"
  note_md: "..."
`,
			wantViolation: "scope.ref",
		},
		{
			name: "an unrecognised scope kind is rejected",
			file: "medidas.yaml",
			yaml: `
- id: m1
  name: "Instrumento"
  scope: { kind: continente, ref: europa }
  date_start: 2021-12-31
  source_url: "https://www.boe.es/x"
  note_md: "..."
`,
			wantViolation: "scope.kind",
		},
		{
			// A global scope names nothing, so a ref beside it is a
			// contradiction the reader would never see resolved.
			name: "a global scope carrying a ref is rejected",
			file: "eventos.yaml",
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
			file: "eventos.yaml",
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
				tc.file:                        &fstest.MapFile{Data: []byte(tc.yaml)},
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
					if v.File == tc.file {
						t.Fatalf("expected no violation for %s, got %v", tc.file, v)
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
