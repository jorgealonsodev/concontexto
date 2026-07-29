package main

// Task 9.11: the Fase-0 closure gate. This is the single, always-run
// (`go test ./...`) entry point that walks all seven PRD §17 milestone
// exit criteria and, for each one, either asserts it directly, points to
// the exact existing test that already proves it, or states plainly that
// it cannot be asserted here.
//
// Design choice: most of Fase 0 was built test-first, milestone by
// milestone, so the real proof for several criteria already exists as an
// expensive fixture- or container-backed test elsewhere in the suite
// (design.md's own testing strategy: "Repository/SQL ... testcontainers-go
// postgres:17-alpine, TestMain container per package"). This gate does
// NOT re-run those containers/fixtures a second time under a new name --
// that would be exactly the "manufactured ceremony test" design.md's own
// Milestone-0.1 row explicitly warns against, and it would double the
// Docker cost of `go test ./...` for zero additional proof. Instead this
// test names the exact proving test next to each criterion it does not
// re-assert, and treats "go test ./..." itself (which already runs those
// named tests) as part of the same closure gate this file's own doc
// comment for TestFase0ClosureGate describes. Where a criterion has no
// automated proof at all (0.1's deploy check), this test says so
// explicitly via t.Log rather than fabricating a passing assertion or
// silently skipping it.
import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func closureRealConfig(t *testing.T) *config.Config {
	t.Helper()
	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

// closureRepoRoot resolves the repository root from this package's own
// directory (app/cmd/concontexto -- three levels below root), so the 0.1
// subtest can check deploy-artifact presence without a hard-coded
// absolute path.
func closureRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..", "..")
}

// TestFase0ClosureGate is the milestone-by-milestone verification PRD
// §17 asks for. Run it with `go test ./app/cmd/concontexto/... -run
// TestFase0ClosureGate -v` for a standalone report, or as part of the
// full `go test ./...` (its own natural home, since it is itself a plain
// Go test with zero external dependencies -- no Docker, no network).
func TestFase0ClosureGate(t *testing.T) {
	cfg := closureRealConfig(t)
	root := closureRepoRoot(t)

	// --- 0.1 Repository + CI + environments -----------------------------
	// PRD §17's exit criterion is "automated deploy of a hello world".
	// design.md's own testing-strategy table is explicit that this has
	// "almost no unit-testable surface -- the test is the CI job + a
	// container smoke test"; running the smoke test itself is CI's job
	// (.github/workflows/ci.yml's own container-smoke-test job), not this
	// offline `go test` gate's. What THIS subtest asserts is the wiring:
	// the smoke-test script exists, is executable, and ci.yml actually
	// invokes it -- so task 1.21 (and everything tasks 1.14-1.19 defer to
	// it) cannot regress to "marked complete but never wired in" again
	// without this gate failing the build.
	//
	// History: task 1.21 was marked [x] in tasks.md after a human ran this
	// exact sequence by hand once, in a PR review session, and reported
	// the real output -- but never automated it. This subtest previously
	// only t.Log'd that gap without failing the build. scripts/smoke-test.sh
	// plus the ci.yml container-smoke-test job (corrective batch) close it;
	// the assertions below are the regression guard.
	t.Run("0.1_repository_ci_environments", func(t *testing.T) {
		for _, rel := range []string{
			"Dockerfile", "docker-compose.yml",
			".github/workflows/ci.yml", "web/package.json",
			"scripts/smoke-test.sh",
		} {
			if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
				t.Errorf("expected deploy artifact %s to exist: %v", rel, err)
			}
		}

		smokeInfo, err := os.Stat(filepath.Join(root, "scripts/smoke-test.sh"))
		if err != nil {
			t.Fatalf("scripts/smoke-test.sh must exist: %v", err)
		}
		if smokeInfo.Mode()&0o111 == 0 {
			t.Error("scripts/smoke-test.sh must be executable (chmod +x)")
		}

		ci, err := os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
		if err != nil {
			t.Fatalf("reading .github/workflows/ci.yml: %v", err)
		}
		if !strings.Contains(string(ci), "scripts/smoke-test.sh") {
			t.Error("tasks.md marks task 1.21 (\"CI smoke test: build image, run compose, curl " +
				"/healthz, exercise healthcheck inside the container\") complete, but " +
				".github/workflows/ci.yml does not invoke scripts/smoke-test.sh. Without this, " +
				"milestone 0.1's deploy exit criterion and the \"proven by 1.21's smoke test\" " +
				"claim made by tasks 1.14-1.19 have no ongoing CI verification.")
		}
	})

	// --- 0.1 deploy step (remediation batch, sdd-verify CRITICAL C5) -----
	// task 1.17 originally claimed a "+ deploy step" that did not exist
	// anywhere in the repository -- the same "marked [x], deliverable
	// absent" failure mode 1.21 already disclosed once. This subtest
	// asserts the HONEST version of that fix: a real, correctly wired
	// deploy workflow exists and is explicitly gated on the one secret a
	// real Portainer redeploy needs, rather than either (a) a fabricated
	// deploy that cannot run in an environment with no VPS/secrets, or
	// (b) silently leaving the false claim in tasks.md. It deliberately
	// does NOT assert that a deploy actually happened -- it cannot,
	// offline, with zero configured secrets (see docs/deploy.md) -- and
	// says so via t.Log, matching this same file's own established
	// convention for a criterion with no automated proof surface.
	t.Run("0.1_deploy_step_gated_on_secrets_and_honestly_disclosed", func(t *testing.T) {
		for _, rel := range []string{".github/workflows/deploy.yml", "docs/deploy.md"} {
			if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
				t.Errorf("expected %s to exist: %v", rel, err)
			}
		}
		deploy, err := os.ReadFile(filepath.Join(root, ".github/workflows/deploy.yml"))
		if err != nil {
			t.Fatalf("reading .github/workflows/deploy.yml: %v", err)
		}
		if !strings.Contains(string(deploy), "PORTAINER_WEBHOOK_URL") {
			t.Error("expected deploy.yml to gate on secrets.PORTAINER_WEBHOOK_URL, the one secret " +
				"a real Portainer redeploy needs -- see docs/deploy.md")
		}
		t.Log("PRD §17's milestone-0.1 exit criterion (\"automated deploy of a hello world\") " +
			"remains genuinely UNMET: this environment has no VPS/Portainer target and no " +
			"PORTAINER_WEBHOOK_URL secret configured, so deploy.yml's own gate step no-ops with " +
			"a ::warning:: annotation rather than reporting a fake green deploy. The workflow " +
			"itself is correctly built and reviewable; only the target infrastructure is missing.")
	})

	// --- 0.2 INE ingestion -----------------------------------------------
	t.Run("0.2_ine_six_series_full_history_loaded_and_validated", func(t *testing.T) {
		var slugs []string
		for _, s := range cfg.Series {
			if s.Source == "ine" {
				slugs = append(slugs, s.Slug)
			}
		}
		if len(slugs) != 6 {
			t.Errorf("expected exactly 6 INE series configured, got %d: %v", len(slugs), slugs)
		}
		if violations := config.Validate(cfg); len(violations) != 0 {
			t.Errorf("real config fails validate-config: %v", violations)
		}
		t.Log("Trimmed-fixture load + all six validation rules are proven end-to-end by " +
			"TestIngestSeries_AllSixSeriesLoadTheirTrimmedFixtureHistoryAndValidate " +
			"(app/internal/ingestion/ingest_test.go, renamed W5: the name previously said " +
			"\"FullHistory\" but every assertion is against the 3-period trimmed fixture), " +
			"part of this same `go test ./...` run; not re-executed here to avoid a second " +
			"real-fixture pass. A genuinely full-history real-config ingest is separately " +
			"proven, offline, by TestRunIngest_RealEmbeddedConfigReconcilesAndIngestsA" +
			"RealConfiguredSeriesOffline (app/cmd/concontexto).")
	})

	// --- 0.3 Eurostat ingestion -------------------------------------------
	t.Run("0.3_eurostat_three_harmonized_datasets_loaded", func(t *testing.T) {
		var slugs []string
		for _, s := range cfg.Series {
			if s.Source == "eurostat" {
				slugs = append(slugs, s.Slug)
				if !s.Harmonized {
					t.Errorf("Eurostat series %q expected harmonized=true", s.Slug)
				}
			}
		}
		if len(slugs) != 3 {
			t.Errorf("expected exactly 3 Eurostat series configured, got %d: %v", len(slugs), slugs)
		}
		t.Log("Load + validation is proven end-to-end by " +
			"TestIngestSeries_AllThreeEurostatDatasetsLoadAndValidate " +
			"(app/internal/ingestion/ingest_eurostat_test.go), part of this same `go test ./...` run.")
	})

	// --- 0.4 Data model + vintages -----------------------------------------
	t.Run("0.4_revision_creates_new_version_without_overwriting", func(t *testing.T) {
		t.Log("NOT re-asserted here: design.md explicitly rejects a fake/in-memory proof for this " +
			"exit criterion (\"only a real PostgreSQL 17 can assert that the database itself rejects " +
			"a second current row\"); duplicating that testcontainers container in this package would " +
			"double Docker cost for zero new proof. Proven by " +
			"TestObservationWriter_RevisionCreatesNewVersionWithoutOverwriting and " +
			"TestMigrationUp_DatabaseRejectsSecondCurrentRow " +
			"(app/internal/adapters/postgres/observation_writer_test.go, migration_test.go), both " +
			"real-Postgres, both skipped under -short, both part of the plain `go test ./...` run " +
			"this closure gate is itself part of.")
	})

	// --- 0.5 Break/event registry v1 ---------------------------------------
	t.Run("0.5_break_event_registry_populated", func(t *testing.T) {
		if len(cfg.Breaks) == 0 {
			t.Error("expected config/rupturas.yaml to contain at least one break entry")
		}
		if len(cfg.Events) == 0 {
			t.Error("expected config/eventos.yaml + config/gobiernos.yaml to contain at least one event entry")
		}
		var confirmed, unconfirmed int
		for _, b := range cfg.Breaks {
			if b.DateStatus == "unconfirmed" {
				unconfirmed++
			} else {
				confirmed++
			}
		}
		for _, e := range cfg.Events {
			if e.DateStatus == "unconfirmed" {
				unconfirmed++
			} else {
				confirmed++
			}
		}
		t.Logf("registry has %d confirmed-date and %d unconfirmed-date entries", confirmed, unconfirmed)
		if unconfirmed > 0 {
			t.Logf("INCOMPLETE, not failing: %d of %d entries carry date_status=unconfirmed. "+
				"ReconcileEditorialConfig never projects a nil date into the database "+
				"(TestReconcileEditorialConfig_ProjectsConfirmedEntriesAndSkipsUnconfirmedDates, "+
				"app/internal/ingestion), so nothing wrong reaches series_break/event -- but the "+
				"registry itself is not yet fully populated pending those confirmations. Milestone "+
				"0.5 is genuinely incomplete on this count, not unconditionally closed.",
				unconfirmed, confirmed+unconfirmed)
		}
	})

	// --- 0.6 XLSX parser -----------------------------------------------------
	t.Run("0.6_xlsx_real_ingestion_and_malformed_suite", func(t *testing.T) {
		var xlsxRefs int
		for _, s := range cfg.Series {
			for _, ref := range s.SourceRefs {
				if ref.Kind == "xlsx-url" {
					xlsxRefs++
				}
			}
		}
		if xlsxRefs == 0 {
			t.Error("expected at least one xlsx-url series configured (Social Security affiliation)")
		}
		t.Log("Real monthly ingestion is proven by TestDecode_HappyPath and " +
			"TestDecode_FullHistoryEveryRowSatisfiesArithmeticInvariant " +
			"(app/internal/adapters/xlsx/decode_test.go); the malformed-file suite that correctly " +
			"fails is proven by TestDecode_MalformedFileSuite (same package); the end-to-end " +
			"'writes nothing, prior datum preserved' path is proven by " +
			"TestIngestSeries_XLSXPartialSuccessWithinOneWorkbookWritesNothingAndPreservesThePublishedDatum " +
			"(app/internal/ingestion/malformed_xlsx_test.go). All part of this same `go test ./...` run.")
	})

	// --- 0.7 Licence resolution -----------------------------------------------
	t.Run("0.7_licence_resolution_per_source_attribution_closed", func(t *testing.T) {
		if violations := config.Validate(cfg); len(violations) != 0 {
			t.Errorf("attribution table is not closed -- validate-config violations: %v", violations)
		}
		seen := map[string]bool{}
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
			seen[s.Source] = true
			if src.Licence.Name == "" || src.Licence.AttributionText == "" ||
				src.Licence.Redistribution.ConditionsMD == "" ||
				src.Licence.Redistribution.CommercialRestrictionsMD == "" {
				t.Errorf("source %q (referenced by series %q) has incomplete licensing", s.Source, s.Slug)
			}
		}
		want := []string{"ine", "eurostat", "seg-social"}
		for _, id := range want {
			if !seen[id] {
				t.Errorf("expected at least one configured series to reference source %q", id)
			}
		}
		if len(seen) != len(want) {
			t.Errorf("expected exactly %d distinct referenced sources %v, got %d: %v", len(want), want, len(seen), seen)
		}
		t.Log("Per-source completeness (INE Ley 37/2007 citation, Eurostat 2011/833/EU acknowledgement-" +
			"only/third-party-excluded scope, Seguridad Social Ley 37/2007 citation + acknowledgement " +
			"required) is proven in detail by app/internal/adapters/config/licensing_test.go's " +
			"TestClosure_* and TestReal* tests, part of this same `go test ./...` run.")
	})
}
