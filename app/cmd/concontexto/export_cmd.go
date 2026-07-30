package main

// Task 3.11 (RED/GREEN): the standalone `concontexto export` subcommand
// (design D-2, "A standalone concontexto export subcommand runs the
// same path for recovery, boot-time self-heal, and fixture generation"
// -- boot-time self-heal itself is a future slice's wiring, this slice
// only builds the command every one of those callers will share). Task
// 3.12 wires `export --fixture` to the exact invocation the golden
// anti-drift fixture is regenerated from: `go run ./app/cmd/concontexto
// export --fixture`.
//
// runExport is the testable core (mirrors runIngest/runValidateConfig's
// established pattern in this package): it takes an already-built
// publishing.Deps and an already-resolved output directory instead of
// resolving DATABASE_URL/STATIC_ROOT itself, so a test can drive it
// against fakes or a real testcontainers pool without a live
// environment.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

// fixtureExportDir is where `export --fixture` writes the golden
// anti-drift fixture (design D-1: "Go exports a golden artifact fixture
// ... and the web unit suite validates it with the same Zod schemas the
// real build uses"; slice 5 is the first consumer of this directory).
const fixtureExportDir = "web/test/fixtures/export"

// buildExportDeps binds publishing.Deps' five function-typed ports to the
// real postgres package functions against db -- the ONE production
// composition point every publishing.Export/Publish caller shares (design
// D-2: "the command layer ... calls Publish"). db is postgres.DBTX (not
// *pgxpool.Pool) so runIngest (schedule.go/ingest_cmd.go, slice 4) can
// reuse this exact function against its own already-open db handle --
// *pgxpool.Pool (this function's original, still its only real caller)
// satisfies DBTX structurally, so this widening changes no behaviour.
func buildExportDeps(db postgres.DBTX) publishing.Deps {
	return publishing.Deps{
		ListPublishedSeries: func(ctx context.Context) ([]postgres.PublishedSeries, error) {
			return postgres.ListPublishedSeries(ctx, db)
		},
		ListObservations: func(ctx context.Context, seriesID string) ([]postgres.PublishedObservation, error) {
			return postgres.ListPublishedObservations(ctx, db, seriesID)
		},
		SeriesFreshness: func(ctx context.Context, seriesID string, asOf time.Time) (freshness.State, error) {
			return postgres.SeriesFreshness(ctx, db, seriesID, asOf)
		},
		ResolveActiveBreaksForSeries: func(ctx context.Context, seriesID string) ([]postgres.SeriesBreak, error) {
			return postgres.ResolveActiveBreaksForSeries(ctx, db, seriesID)
		},
		ListActiveEvents: func(ctx context.Context, seriesID string) ([]postgres.Event, error) {
			return postgres.ListActiveEvents(ctx, db, seriesID)
		},
		// Remediation B (CRITICAL-4): the validation half of the
		// artifact's page state. Bound here, at the ONE production
		// composition point, so `export`, `export --fixture` and the
		// scheduler's in-cycle export all carry it -- an artifact
		// written without this port would silently report every series
		// as "fresh".
		SeriesValidationOutcome: func(ctx context.Context, seriesID string) (postgres.ValidationOutcome, error) {
			return postgres.SeriesValidationOutcome(ctx, db, seriesID)
		},
	}
}

// runExport calls publishing.Export against deps and reports the
// resulting series count and manifest header on stdout, or the error on
// stderr (spec "the export fails with an error naming the violated
// constraint" -- that error string is exactly what reaches stderr here,
// unmodified).
func runExport(ctx context.Context, deps publishing.Deps, outDir string, now time.Time, stdout, stderr io.Writer) int {
	artifact, err := publishing.Export(ctx, deps, now, outDir)
	if err != nil {
		fmt.Fprintln(stderr, "export:", err)
		return 1
	}
	fmt.Fprintf(stdout, "export: wrote %d series to %s (schema_version=%d, generated_at=%s)\n",
		len(artifact.Series), outDir, artifact.Manifest.SchemaVersion, artifact.Manifest.GeneratedAt.Format(time.RFC3339))
	return 0
}

// exportOutputDir resolves `export`'s output directory: the fixed
// golden-fixture path under `--fixture` (task 3.12), or the LIVE
// STATIC_ROOT/data-derived otherwise (design D-1: "the artifact IS
// /data-derived/" -- immediately servable, no Astro build required to
// exist first). Extracted so the directory-selection decision is
// testable without DATABASE_URL or Docker.
func exportOutputDir(fixture bool) string {
	if fixture {
		return fixtureExportDir
	}
	return filepath.Join(staticAssetRoot(), "data-derived")
}

// cmdExport dispatches `export` (writes the LIVE artifact) and
// `export --fixture` (writes the golden anti-drift fixture instead)
// against a real DATABASE_URL, mirroring cmdIngest's established
// resolution pattern in this same package.
func cmdExport(args []string, stdout, stderr io.Writer) int {
	fixture := false
	for _, a := range args {
		if a == "--fixture" {
			fixture = true
		}
	}

	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(stderr, "export: DATABASE_URL is not set")
		return 1
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(stderr, "export:", err)
		return 1
	}
	defer pool.Close()

	return runExport(ctx, buildExportDeps(pool), exportOutputDir(fixture), time.Now().UTC(), stdout, stderr)
}
