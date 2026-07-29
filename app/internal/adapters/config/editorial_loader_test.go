package config_test

// Task 7.1: the loader half of authoring config/{rupturas,eventos,
// gobiernos}.yaml — proves the three editorial files parse into typed
// Breaks/Events (spec editorial-config, "Editorial entries carry stable
// identifiers and a digest"). Duplicate-id rejection is task 7.2/7.3,
// covered separately in editorial_validate_test.go.

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func TestLoad_RupturasYAMLParsesConfirmedAndUnconfirmedEntries(t *testing.T) {
	fsys := fstest.MapFS{
		"rupturas.yaml": &fstest.MapFile{Data: []byte(`
- id: epa-metodologia-2021
  date: 2021-01-01
  kind: methodology
  scope: { kind: dataset, ref: ine-epa }
  note_md: "Cambio metodológico de la EPA (base 2021)."
  source_url: "https://www.ine.es/dyngs/INEbase/es/operacion.htm?c=Estadistica_C&cid=1254736176918"
- id: epa-cnae2025-doble-codificacion
  date_status: unconfirmed
  todo: "Confirmar el primer periodo afectado en CLASIFICACIONES_OPERACION/293 (INE)."
  kind: methodology
  scope: { kind: dataset, ref: ine-epa }
  note_md: "Doble codificación CNAE 2025 en la EPA; fecha de efecto pendiente de confirmar."
`)},
	}

	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Breaks) != 2 {
		t.Fatalf("expected 2 breaks, got %d: %+v", len(cfg.Breaks), cfg.Breaks)
	}

	confirmed := cfg.Breaks[0]
	if confirmed.ID != "epa-metodologia-2021" {
		t.Errorf("Breaks[0].ID = %q, want epa-metodologia-2021", confirmed.ID)
	}
	wantDate := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	if confirmed.Date == nil || !confirmed.Date.Equal(wantDate) {
		t.Errorf("Breaks[0].Date = %v, want %v", confirmed.Date, wantDate)
	}
	if confirmed.Scope.Kind != "dataset" || confirmed.Scope.Ref != "ine-epa" {
		t.Errorf("Breaks[0].Scope = %+v, want dataset/ine-epa", confirmed.Scope)
	}
	if confirmed.FilePath != "rupturas.yaml" {
		t.Errorf("Breaks[0].FilePath = %q, want rupturas.yaml", confirmed.FilePath)
	}

	unconfirmed := cfg.Breaks[1]
	if unconfirmed.DateStatus != "unconfirmed" {
		t.Errorf("Breaks[1].DateStatus = %q, want unconfirmed", unconfirmed.DateStatus)
	}
	if unconfirmed.Date != nil {
		t.Errorf("Breaks[1].Date = %v, want nil (unconfirmed)", unconfirmed.Date)
	}
	if unconfirmed.Todo == "" {
		t.Errorf("Breaks[1].Todo is empty, want the document to consult")
	}
}

func TestLoad_EventosYAMLKeepsItsOwnPerEntryGroup(t *testing.T) {
	fsys := fstest.MapFS{
		"eventos.yaml": &fstest.MapFile{Data: []byte(`
- id: crisis-financiera-2008-2013
  group: exogenous
  name: "Crisis financiera global y de deuda soberana"
  date_start: 2008-01-01
  date_end: 2013-12-31
  note_md: "Crisis financiera global (2008) y crisis de deuda soberana europea (hasta 2013)."
`)},
	}

	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(cfg.Events), cfg.Events)
	}
	ev := cfg.Events[0]
	if ev.Group != "exogenous" {
		t.Errorf("Group = %q, want exogenous", ev.Group)
	}
	if ev.FilePath != "eventos.yaml" {
		t.Errorf("FilePath = %q, want eventos.yaml", ev.FilePath)
	}
	wantStart := time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)
	if ev.DateStart == nil || !ev.DateStart.Equal(wantStart) {
		t.Errorf("DateStart = %v, want %v", ev.DateStart, wantStart)
	}
}

func TestLoad_GobiernosYAMLEntriesAreAssignedTheGovernmentsGroupAndCarryNoColour(t *testing.T) {
	fsys := fstest.MapFS{
		"gobiernos.yaml": &fstest.MapFile{Data: []byte(`
- id: gobierno-sanchez-2018
  name: "Pedro Sánchez (I)"
  date_start: 2018-06-02
  note_md: "Investidura tras la moción de censura del 1 de junio de 2018."
`)},
	}

	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(cfg.Events), cfg.Events)
	}
	if cfg.Events[0].Group != "governments" {
		t.Errorf("Group = %q, want governments (assigned by the loader)", cfg.Events[0].Group)
	}
	if cfg.Events[0].FilePath != "gobiernos.yaml" {
		t.Errorf("FilePath = %q, want gobiernos.yaml", cfg.Events[0].FilePath)
	}
}

func TestLoad_MissingEditorialFilesIsNotAnError(t *testing.T) {
	cfg, err := config.Load(fstest.MapFS{})
	if err != nil {
		t.Fatalf("Load on a tree with no editorial files returned an error: %v", err)
	}
	if len(cfg.Breaks) != 0 || len(cfg.Events) != 0 {
		t.Errorf("expected empty Breaks/Events, got %+v", cfg)
	}
}
