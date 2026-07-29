package config_test

// Task 7.2 (RED) / 7.3 (GREEN): duplicate stable ids in rupturas.yaml or
// across eventos.yaml+gobiernos.yaml fail validate-config naming the
// duplicate (spec editorial-config, "Duplicate ids are rejected"). Events
// are checked across BOTH files together, not per-file: event.id is a
// single PRIMARY KEY (migration 0001) shared by both files' rows once
// reconciled, so an id reused across eventos.yaml and gobiernos.yaml
// would silently collide at reconcile time — checking only within one
// file would miss exactly that case.
//
// Also covers the required-field and date_status/todo rules a break/event
// entry must satisfy (design.md "validate-config checks: YAML schema,
// unique stable ids ...").

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func TestValidate_DuplicateBreakIDInRupturasYAMLFailsNamingIt(t *testing.T) {
	fsys := fstest.MapFS{
		"rupturas.yaml": &fstest.MapFile{Data: []byte(`
- id: dup-break
  date: 2021-01-01
  kind: methodology
  scope: { kind: dataset, ref: ine-epa }
  note_md: "Primera entrada."
- id: dup-break
  date: 2022-01-01
  kind: methodology
  scope: { kind: dataset, ref: ine-ipc }
  note_md: "Segunda entrada, mismo id por error."
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	found := false
	for _, v := range violations {
		if v.File == "rupturas.yaml" && strings.Contains(v.Message, "dup-break") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a violation naming the duplicated id dup-break, got %v", violations)
	}
}

func TestValidate_DuplicateEventIDAcrossEventosAndGobiernosFailsNamingIt(t *testing.T) {
	fsys := fstest.MapFS{
		"eventos.yaml": &fstest.MapFile{Data: []byte(`
- id: dup-event
  group: exogenous
  name: "Shock exógeno"
  date_start: 2008-01-01
  note_md: "..."
`)},
		"gobiernos.yaml": &fstest.MapFile{Data: []byte(`
- id: dup-event
  name: "Gobierno"
  date_start: 2018-06-02
  note_md: "..."
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	found := false
	for _, v := range violations {
		if strings.Contains(v.Message, "dup-event") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a violation naming the duplicated id dup-event across eventos.yaml/gobiernos.yaml, got %v", violations)
	}
}

func TestValidate_BreakMissingRequiredFieldsFailsNamingThem(t *testing.T) {
	fsys := fstest.MapFS{
		"rupturas.yaml": &fstest.MapFile{Data: []byte(`
- id: incomplete-break
  date: 2021-01-01
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	if !hasViolation(violations, "rupturas.yaml", "kind") {
		t.Errorf("expected a violation naming rupturas.yaml + kind, got %v", violations)
	}
	if !hasViolation(violations, "rupturas.yaml", "scope.kind") {
		t.Errorf("expected a violation naming rupturas.yaml + scope.kind, got %v", violations)
	}
	if !hasViolation(violations, "rupturas.yaml", "note_md") {
		t.Errorf("expected a violation naming rupturas.yaml + note_md, got %v", violations)
	}
}

func TestValidate_BreakWithNeitherDateNorUnconfirmedStatusFails(t *testing.T) {
	fsys := fstest.MapFS{
		"rupturas.yaml": &fstest.MapFile{Data: []byte(`
- id: no-date-no-status
  kind: methodology
  scope: { kind: dataset, ref: ine-epa }
  note_md: "Falta la fecha y no está marcada como pendiente de confirmar."
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	if !hasViolation(violations, "rupturas.yaml", "date") {
		t.Fatalf("expected a violation naming rupturas.yaml + date, got %v", violations)
	}
}

func TestValidate_UnconfirmedBreakWithoutTodoFailsNamingIt(t *testing.T) {
	fsys := fstest.MapFS{
		"rupturas.yaml": &fstest.MapFile{Data: []byte(`
- id: unconfirmed-no-todo
  date_status: unconfirmed
  kind: methodology
  scope: { kind: dataset, ref: ine-epa }
  note_md: "Pendiente de confirmar pero sin decir qué documento consultar."
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	if !hasViolation(violations, "rupturas.yaml", "todo") {
		t.Fatalf("expected a violation naming rupturas.yaml + todo, got %v", violations)
	}
}

func TestValidate_UnconfirmedBreakWithTodoAndNoDatePasses(t *testing.T) {
	fsys := fstest.MapFS{
		"rupturas.yaml": &fstest.MapFile{Data: []byte(`
- id: unconfirmed-with-todo
  date_status: unconfirmed
  todo: "Confirmar contra la nota metodológica de la fuente."
  kind: methodology
  scope: { kind: dataset, ref: ine-epa, ref_status: pending }
  note_md: "Ruptura confirmada en su existencia; fecha pendiente."
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if violations := config.Validate(cfg); len(violations) != 0 {
		t.Fatalf("expected an unconfirmed-with-todo break to pass validate-config, got %v", violations)
	}
}

func TestValidate_BreakWithInvalidDateStatusFailsNamingIt(t *testing.T) {
	fsys := fstest.MapFS{
		"rupturas.yaml": &fstest.MapFile{Data: []byte(`
- id: bogus-status
  date_status: probably
  kind: methodology
  scope: { kind: dataset, ref: ine-epa }
  note_md: "..."
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	if !hasViolation(violations, "rupturas.yaml", "date_status") {
		t.Fatalf("expected a violation naming rupturas.yaml + date_status, got %v", violations)
	}
}

func TestValidate_EventMissingRequiredFieldsFailsNamingThem(t *testing.T) {
	fsys := fstest.MapFS{
		"eventos.yaml": &fstest.MapFile{Data: []byte(`
- id: incomplete-event
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	if !hasViolation(violations, "eventos.yaml", "group") {
		t.Errorf("expected a violation naming eventos.yaml + group, got %v", violations)
	}
	if !hasViolation(violations, "eventos.yaml", "name") {
		t.Errorf("expected a violation naming eventos.yaml + name, got %v", violations)
	}
}

func TestValidate_CompleteEditorialFilesPass(t *testing.T) {
	fsys := fstest.MapFS{
		// A resolving dataset scope (remediation batch, W2) needs a real
		// configured series naming that dataset — reusing
		// validate_test.go's own completeSourceYAML/completeSeriesYAML
		// helpers (source "ine", dataset "ine-epa") keeps this in sync
		// with the same fixtures TestValidate_CompleteTreePasses uses.
		"sources/ine.yaml":             &fstest.MapFile{Data: []byte(completeSourceYAML())},
		"series/tasa-de-paro-epa.yaml": &fstest.MapFile{Data: []byte(completeSeriesYAML())},
		"rupturas.yaml": &fstest.MapFile{Data: []byte(`
- id: epa-metodologia-2021
  date: 2021-01-01
  kind: methodology
  scope: { kind: dataset, ref: ine-epa }
  note_md: "Cambio metodológico de la EPA (base 2021)."
`)},
		"eventos.yaml": &fstest.MapFile{Data: []byte(`
- id: crisis-financiera-2008-2013
  group: exogenous
  name: "Crisis financiera global"
  date_start: 2008-01-01
  date_end: 2013-12-31
  note_md: "..."
`)},
		"gobiernos.yaml": &fstest.MapFile{Data: []byte(`
- id: gobierno-sanchez-2018
  name: "Pedro Sánchez (I)"
  date_start: 2018-06-02
  note_md: "..."
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if violations := config.Validate(cfg); len(violations) != 0 {
		t.Fatalf("expected a complete editorial config to pass, got %v", violations)
	}
}
