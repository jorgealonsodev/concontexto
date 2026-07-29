package main

// Remediation batch (verify-report CRITICAL C8/M7): cmdIngestRun
// (ingest_cmd.go) is cmdIngest's non-reconcile composition core -- where
// archiveHashPath/publicHashPath get resolved and handed to runIngest.
// Before this batch that line was never executed by any test: existing
// cmdIngest tests only reach the early-return branches, and every
// runIngest test calls runIngest directly with already-resolved paths.
// Setting publicHashPath to "" there silently disables PRD §14.2's
// public hash listing (ingest.go:192 gates on both paths non-empty) with
// the whole suite green.
//
// This test exercises cmdIngestRun end to end -- real Postgres, the real
// embedded config, one configured series against a checked-in fixture
// served offline -- and asserts the public hash listing actually lands
// at STATIC_ROOT/transparencia/raw-files.sha256.

import (
	"bytes"
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestCmdIngestRun_PublishesThePublicHashListingAtStaticRootTransparencia(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_ingest_composition_test"),
		tcpostgres.WithUsername("concontexto_ingest_composition_test"),
		tcpostgres.WithPassword("concontexto_ingest_composition_test"),
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
		t.Fatalf("shipped config's series %q references COD %q with no checked-in fixture at %s: %v",
			slug, ref.Ref, fixturePath, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	src, ok := cfg.Sources[seriesCfg.Source]
	if !ok {
		t.Fatalf("shipped series %q references unknown source %q", slug, seriesCfg.Source)
	}
	src.API = &config.APIConfig{BaseURL: server.URL, MaxResponseBytes: src.API.MaxResponseBytes}
	cfg.Sources[seriesCfg.Source] = src

	appDataDir := t.TempDir()
	staticDir := t.TempDir()
	t.Setenv("APP_DATA_ROOT", appDataDir)
	t.Setenv("STATIC_ROOT", staticDir)

	var stdout, stderr bytes.Buffer
	code := cmdIngestRun(ctx, pool, cfg, slug, "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cmdIngestRun: exit %d, stderr=%q", code, stderr.String())
	}

	publicPath := filepath.Join(staticDir, "transparencia", "raw-files.sha256")
	data, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatalf("expected cmdIngestRun's own publicHashPath resolution to publish %s, got: %v "+
			"(verify-report CRITICAL C8/M7: an empty publicHashPath silently skips this file)", publicPath, err)
	}
	if len(data) == 0 {
		t.Fatalf("expected a non-empty public hash listing at %s", publicPath)
	}
}

// TestResolveIngestPaths_ReturnsNonEmptyArchiveAndPublicPaths is a fast,
// Docker-free unit test on the exact formula cmdIngestRun and
// startSchedulerLoop both delegate to, complementing the slower
// integration proof above.
func TestResolveIngestPaths_ReturnsNonEmptyArchiveAndPublicPaths(t *testing.T) {
	archive, public := resolveIngestPaths("/tmp/root", "/tmp/static")

	if archive == "" {
		t.Fatal("expected a non-empty archive hash-listing path")
	}
	if public == "" {
		t.Fatal("expected a non-empty public hash-listing path")
	}
	if want := filepath.Join("/tmp/root", "raw_files.sha256"); archive != want {
		t.Fatalf("archive hash-listing path = %q, want %q", archive, want)
	}
	if want := filepath.Join("/tmp/static", "transparencia", "raw-files.sha256"); public != want {
		t.Fatalf("public hash-listing path = %q, want %q", public, want)
	}
}

// TestAppDataRoot_DefaultsAndHonoursOverride covers appDataRoot's
// env-reading branches cheaply (no Docker): a documented,
// check-env-example-enforced variable whose parse/fallback path was
// otherwise never exercised.
func TestAppDataRoot_DefaultsAndHonoursOverride(t *testing.T) {
	t.Run("unset falls back to the container default", func(t *testing.T) {
		t.Setenv("APP_DATA_ROOT", "")
		if got := appDataRoot(); got != "/app_data" {
			t.Fatalf("appDataRoot() = %q, want /app_data", got)
		}
	})
	t.Run("set overrides the default", func(t *testing.T) {
		t.Setenv("APP_DATA_ROOT", "/tmp/custom-app-data-root")
		if got := appDataRoot(); got != "/tmp/custom-app-data-root" {
			t.Fatalf("appDataRoot() = %q, want /tmp/custom-app-data-root", got)
		}
	})
}
