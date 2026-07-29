package main

// Remediation batch (verify-report Priority 2 / risk 3 the previous
// batch disclosed): postgres.ReconcileDimensions and runIngest were only
// ever proven against SYNTHETIC config (ingest_run_cmd_test.go). No test
// ran `ingest --series <slug>` against a REAL series slug from the
// shipped embedded /config tree with ReconcileDimensions reconciling the
// REAL config -- the exact class of gap this whole verification pass has
// been finding (tested in isolation, never exercised against reality).
//
// This test loads the real embedded configdata.FS (the SAME tree
// `validate-config` and every production `ingest` invocation loads),
// reconciles ALL of it via runIngest's own ReconcileDimensions call (not
// a hand-seeded subset), then ingests exactly one real configured series
// ("tasa-de-paro-epa", INE ref EPA453100) against its checked-in,
// real-response fixture (app/internal/adapters/ine/testdata/datos_serie,
// source.txt-documented) served over a local httptest.Server -- offline,
// deterministic, but genuinely the shipped config end to end. The ONLY
// thing overridden is the source's api.base_url, so the request is
// captured instead of leaving this machine; every other field (licence,
// id, redistribution terms, all nine other series/sources) is exactly
// what ships in /config.
import (
	"bytes"
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestRunIngest_RealEmbeddedConfigReconcilesAndIngestsARealConfiguredSeriesOffline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_real_config_test"),
		tcpostgres.WithUsername("concontexto_real_config_test"),
		tcpostgres.WithPassword("concontexto_real_config_test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	var migrateOut, migrateErr bytes.Buffer
	t.Setenv("DATABASE_URL", dsn)
	if code := cmdMigrate([]string{"up"}, &migrateOut, &migrateErr); code != 0 {
		t.Fatalf("migrate up: exit %d, stderr=%q", code, migrateErr.String())
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	const slug = "tasa-de-paro-epa"
	var seriesCfg config.SeriesConfig
	found := false
	for _, s := range cfg.Series {
		if s.Slug == slug {
			seriesCfg = s
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("shipped config no longer defines series %q -- update this test's target slug", slug)
	}
	ref, ok := postgres.ActiveSourceRef(seriesCfg)
	if !ok {
		t.Fatalf("shipped series %q has no active source_ref", slug)
	}

	fixturePath := filepath.Join("..", "..", "internal", "adapters", "ine", "testdata", "datos_serie", ref.Ref+".json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("shipped config's series %q references COD %q with no checked-in fixture at %s: %v "+
			"(a real configured series with no recorded fixture is a genuine defect, not a test-fixture gap)",
			slug, ref.Ref, fixturePath, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	// The ONLY override: point this one source's endpoint at the local
	// fixture server so the run stays offline. Every other field of the
	// real shipped source config (licence, redistribution terms, id)
	// stays exactly as configured.
	src, ok := cfg.Sources[seriesCfg.Source]
	if !ok {
		t.Fatalf("shipped series %q references unknown source %q", slug, seriesCfg.Source)
	}
	src.API = &config.APIConfig{BaseURL: server.URL, MaxResponseBytes: src.API.MaxResponseBytes}
	cfg.Sources[seriesCfg.Source] = src

	root := t.TempDir()
	store := filestore.NewStore(filepath.Join(root, "raw"))
	archiveHashPath := filepath.Join(root, "app_data", "raw_files.sha256")
	publicHashPath := filepath.Join(root, "public", "transparencia", "raw-files.sha256")
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	var stdout, stderr bytes.Buffer
	// runIngest itself calls postgres.ReconcileDimensions against the
	// WHOLE real cfg (every one of the ten shipped series, not just this
	// one) before ingesting -- this is the exact prerequisite step the
	// previous batch's own apply-progress flagged as never proven
	// against reality.
	code := runIngest(ctx, pool, cfg, store, archiveHashPath, publicHashPath, now, slug, "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runIngest against the real embedded config: exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "outcome=publish") {
		t.Errorf("expected a publish outcome for the real series %q, got %q (stderr=%q)", slug, stdout.String(), stderr.String())
	}

	var obsCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM observation WHERE series_id=$1`, slug).Scan(&obsCount); err != nil {
		t.Fatalf("counting observation: %v", err)
	}
	if obsCount == 0 {
		t.Errorf("expected at least 1 published observation for the real series %q, got %d", slug, obsCount)
	}

	// The whole real config's dimensions were reconciled, not only this
	// one series' -- assert every shipped series now has an identity row.
	var seriesRowCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM series`).Scan(&seriesRowCount); err != nil {
		t.Fatalf("counting series rows: %v", err)
	}
	if seriesRowCount != len(cfg.Series) {
		t.Errorf("expected ReconcileDimensions to have reconciled all %d shipped series, found %d rows", len(cfg.Series), seriesRowCount)
	}
}
