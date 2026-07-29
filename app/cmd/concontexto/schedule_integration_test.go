package main

// Remediation batch (verify-report CRITICAL C4): a genuine end-to-end
// scheduled cycle -- real Postgres (testcontainers), a real
// scheduler.Runner, and scheduleSourceOp's own runIngest("--source")
// path -- proving the wiring this batch adds actually reaches a
// published observation, not only the fake-op scheduling behaviour
// schedule_test.go covers.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/scheduler"
)

func TestRunScheduler_TickDrivesARealCycleThatPublishesAnObservation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_schedule_test"),
		tcpostgres.WithUsername("concontexto_schedule_test"),
		tcpostgres.WithPassword("concontexto_schedule_test"),
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

	fixture := []byte(`{"COD":"TESTSCHED01", "Nombre":"test", "T3_Unidad":"index", "T3_Escala":" ", "Data":[` +
		`{"Fecha":"2026-06-01T00:00:00.000+02:00", "T3_TipoDato":"Definitivo", "T3_Periodo":"M06", "Anyo":2026, "Valor":42}]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	cfg := &config.Config{
		Sources: map[string]config.SourceConfig{
			"test-sched-src": {ID: "test-sched-src", Name: "Test", URL: "https://example.test", AccessType: "api-json",
				API:     &config.APIConfig{BaseURL: server.URL},
				Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
		},
		Series: []config.SeriesConfig{
			{Slug: "test-sched-series", Name: "Test", Source: "test-sched-src", Dataset: "test-sched-dataset",
				Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
				SourceRefs: []config.SourceRef{{Kind: "ine-series-cod", Ref: "TESTSCHED01", ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}},
		},
	}

	root := t.TempDir()
	store := filestore.NewStore(filepath.Join(root, "raw"))
	archiveHashPath := filepath.Join(root, "app_data", "raw_files.sha256")
	publicHashPath := filepath.Join(root, "public", "transparencia", "raw-files.sha256")

	var logs bytes.Buffer
	newOp := func(sourceID string) func(context.Context) error {
		return scheduleSourceOp(pool, cfg, store, archiveHashPath, publicHashPath, sourceID, &logs)
	}
	runners := map[string]*scheduler.Runner{"test-sched-src": scheduler.NewRunner("test-sched-src")}

	schedCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tick := make(chan time.Time)
	go runScheduler(schedCtx, runners, newOp, time.Hour, tick, nil)

	tick <- time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	deadline := time.Now().Add(5 * time.Second)
	var obsCount int
	for time.Now().Before(deadline) {
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM observation WHERE series_id='test-sched-series'`).Scan(&obsCount); err == nil && obsCount == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if obsCount != 1 {
		t.Fatalf("expected the scheduled cycle to publish exactly 1 observation, got %d (op log: %s)", obsCount, logs.String())
	}
}
