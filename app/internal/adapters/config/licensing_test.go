package config_test

// Task 3.9 (RED) / 3.10 (GREEN): "Eurostat restrictions are recorded,
// not flattened" — this scenario is about the AUTHORED
// config/sources/eurostat.yaml content, not the generic schema rules
// validate_test.go covers, so it reads the real embedded config tree
// through configdata.FS + fs.Sub (the same wiring
// app/cmd/concontexto/validate_config_cmd.go uses), proving the checked-in
// file itself — not just a synthetic fixture — satisfies the spec.

import (
	"io/fs"
	"strings"
	"testing"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func realConfig(t *testing.T) *config.Config {
	t.Helper()
	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestRealConfig_PassesValidate(t *testing.T) {
	cfg := realConfig(t)
	if violations := config.Validate(cfg); len(violations) != 0 {
		t.Fatalf("the real checked-in /config tree failed validate-config: %v", violations)
	}
}

func TestRealEurostatSource_RecordsAcknowledgementOnlyThirdPartyAndCommercialRestrictions(t *testing.T) {
	cfg := realConfig(t)
	src, ok := cfg.Sources["eurostat"]
	if !ok {
		t.Fatal("expected config/sources/eurostat.yaml to be present and resolve id=eurostat")
	}

	redist := src.Licence.Redistribution
	if !redist.AcknowledgementRequired {
		t.Error("expected redistribution.acknowledgement_required = true (Commission Decision 2011/833/EU is acknowledgement-only)")
	}
	if !redist.ThirdPartyExcluded {
		t.Error("expected redistribution.third_party_excluded = true (the permission does not extend to third-party material)")
	}
	if !strings.Contains(redist.ConditionsMD, "2011/833/EU") {
		t.Errorf("expected conditions_md to cite Commission Decision 2011/833/EU, got %q", redist.ConditionsMD)
	}
	if !strings.Contains(strings.ToLower(redist.CommercialRestrictionsMD), "commercial") {
		t.Errorf("expected commercial_restrictions_md to record a commercial redissemination restriction, got %q", redist.CommercialRestrictionsMD)
	}
}

func TestRealIneSource_HasCompleteLicensingFields(t *testing.T) {
	cfg := realConfig(t)
	src, ok := cfg.Sources["ine"]
	if !ok {
		t.Fatal("expected config/sources/ine.yaml to be present and resolve id=ine")
	}
	if src.Licence.AttributionText == "" {
		t.Error("expected a non-empty attribution_text")
	}
	if src.AccessType == "" {
		t.Error("expected a non-empty access_type")
	}
	if !src.Licence.Redistribution.Allowed {
		t.Error("expected redistribution.allowed = true for INE")
	}
}

// Task 9.9 (RED) / 9.10 (GREEN): milestone 0.7 closure. INE is a Spanish
// public-sector body: its reuse terms are not just "whatever its own
// aviso_legal happens to say" in isolation, they are an INSTANCE of the
// Spanish public-sector information reuse framework (Ley 37/2007, as
// amended, transposing the EU PSI Directive) -- the same framework
// seg-social's terms cite explicitly below. Recording that framework by
// name, not only the source-specific legal notice, is what "record each
// source's exact attribution formula" (this batch's own instruction)
// means for a Spanish public-sector source: the formula is traceable to
// the LAW, not merely to a webpage that could be edited unilaterally.
func TestClosure_IneRecordsSpanishPublicSectorReuseFramework(t *testing.T) {
	cfg := realConfig(t)
	src, ok := cfg.Sources["ine"]
	if !ok {
		t.Fatal("expected config/sources/ine.yaml to be present and resolve id=ine")
	}
	if !strings.Contains(src.Licence.Redistribution.ConditionsMD, "37/2007") {
		t.Errorf("expected conditions_md to cite the Spanish public-sector information reuse "+
			"framework (Ley 37/2007), got %q", src.Licence.Redistribution.ConditionsMD)
	}
}

// Task 9.9 (RED) / 9.10 (GREEN): closes milestone 0.7 for the third and
// final source. Two distinct gaps existed before this batch:
//
//  1. redistribution.acknowledgement_required was never set in
//     config/sources/seg-social.yaml, so it defaulted to the Go zero
//     value (false) -- silently CONTRADICTING the source's own
//     conditions_md, which already said reuse is "permitida citando la
//     fuente" (permitted CITING the source, i.e. acknowledgement IS
//     required). A structured field disagreeing with its own prose is
//     exactly the "flattened, unverifiable claim" spec
//     source-attribution-licensing's Eurostat scenario already warns
//     against -- the same discipline applies here.
//  2. The source's own file comment claimed "the §9.4 synthetic probe
//     ... is the early warning when [the URL] rotates" -- true when that
//     comment was written (slice 9 was "not built yet"), false now:
//     app/internal/probe.Targets deliberately excludes every xlsx-url ref
//     (see that package's own doc comment -- there is no "last period
//     only" query parameter to probe an XLSX download with, so probing
//     it would mean downloading the full workbook, defeating the probe's
//     own purpose). The ONLY early-warning mechanism that actually exists
//     today is the source_ref's own valid_from/valid_to validity range,
//     checked administratively -- not an automated live probe. GREEN
//     corrects the comment; this test only asserts the structured field.
func TestClosure_SegSocialRecordsSpanishPublicSectorReuseFrameworkWithAttributionRequired(t *testing.T) {
	cfg := realConfig(t)
	src, ok := cfg.Sources["seg-social"]
	if !ok {
		t.Fatal("expected config/sources/seg-social.yaml to be present and resolve id=seg-social")
	}
	if src.AccessType != "xlsx-download" {
		t.Errorf("expected access_type = %q, got %q", "xlsx-download", src.AccessType)
	}
	if src.Licence.AttributionText == "" {
		t.Error("expected a non-empty attribution_text")
	}
	redist := src.Licence.Redistribution
	if !redist.Allowed {
		t.Error("expected redistribution.allowed = true for Seguridad Social")
	}
	if !redist.AcknowledgementRequired {
		t.Error("expected redistribution.acknowledgement_required = true -- the source's own " +
			"conditions_md already requires citing the source, the structured field must agree")
	}
	if !strings.Contains(redist.ConditionsMD, "37/2007") {
		t.Errorf("expected conditions_md to cite the Spanish public-sector information reuse "+
			"framework (Ley 37/2007), got %q", redist.ConditionsMD)
	}
	if redist.CommercialRestrictionsMD == "" {
		t.Error("expected a non-empty commercial_restrictions_md")
	}
}

// Task 9.9 (RED) / 9.10 (GREEN): the literal spec scenario "The
// attribution table is closed before anything is published" --
// EVERY source referenced by ANY real configured series resolves to a
// COMPLETE licence, walked from the series side (not just "every loaded
// source file happens to be complete", which validateSource already
// checks unconditionally). This is also the closure proof for spec
// requirement "Attribution chains from the original source": it builds
// the exact series -> source -> attribution_text projection production
// ingestion needs (indicators.Series.Licence, per that type's own doc
// comment -- "whichever caller builds SeriesContext ... is responsible
// for populating them") using the REAL config, and confirms it never
// yields an empty Licence for rule 5 to block on. Wiring this
// projection into the CLI ingest orchestrator itself remains explicitly
// out of Fase 0 scope (app/cmd/concontexto/ingest_cmd.go's own doc
// comment: "per-source orchestration is later-phase territory") --
// this test proves the DATA is resolvable, not that a not-yet-built
// command resolves it.
func TestClosure_EveryConfiguredSeriesResolvesACompleteLicenceAndPassesRule5(t *testing.T) {
	cfg := realConfig(t)
	if violations := config.Validate(cfg); len(violations) != 0 {
		t.Fatalf("real config fails validate-config, cannot be closed: %v", violations)
	}

	seenSources := map[string]bool{}
	for _, s := range cfg.Series {
		if s.Source == "" {
			t.Errorf("series %q has no source reference", s.Slug)
			continue
		}
		src, ok := cfg.Sources[s.Source]
		if !ok {
			t.Errorf("series %q references unknown source %q", s.Slug, s.Source)
			continue
		}
		seenSources[s.Source] = true

		resolved := indicators.Series{
			Slug: s.Slug, Unit: s.Unit, Frequency: indicators.Frequency(s.Frequency),
			Source: s.Source, Licence: src.Licence.Name,
		}
		findings := validation.Rule5MetadataCompleteness(validation.SeriesContext{Series: resolved}, nil)
		if len(findings) != 0 {
			t.Errorf("series %q: expected rule 5 to pass with the real config's resolved licence, got findings: %v", s.Slug, findings)
		}
	}

	wantSources := []string{"ine", "eurostat", "seg-social"}
	for _, id := range wantSources {
		if !seenSources[id] {
			t.Errorf("expected at least one configured series to reference source %q", id)
		}
	}
	if len(seenSources) != len(wantSources) {
		t.Errorf("expected exactly %d distinct referenced sources %v, got %d: %v", len(wantSources), wantSources, len(seenSources), seenSources)
	}
}
