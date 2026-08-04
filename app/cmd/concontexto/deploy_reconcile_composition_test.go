package main

// THE EIGHTH "BUILT, TESTED, NEVER CONNECTED" INSTANCE, AND ITS GUARD.
//
// ingestion.ReconcileEditorialConfig had exactly ONE production call site
// -- the `ingest --reconcile` flag path in ingest_cmd.go. Nothing in a
// DEPLOYED stack ever issued that invocation: docker-compose.yml ran
// `migrate up` and then `serve`, and `serve`'s in-process scheduler only
// ever calls runIngest (per-source ingestion), never the editorial
// reconcile. Measured on the live stack before this batch:
//
//   SELECT count(*) FROM event;        -> 0
//   SELECT count(*) FROM series_break; -> 0
//
// config/rupturas.yaml, config/eventos.yaml and config/gobiernos.yaml
// therefore never reached the database at all, which silently disabled
// three separately-built features at once: the break band (BreakBand.astro
// and the chart's shaded geometry render from artifact `breaks`, which
// publishing.Export reads back out of series_break), event annotations, and
// validation rule 3's break exemption (rule3_plausibility.go's breakAt
// always saw an empty slice, so a jump at a genuinely recorded
// methodological break blocked exactly as if no break existed).
//
// WHY THESE TWO TESTS, AND WHY NOT A GREP.
// The wiring that closes the gap lives in docker-compose.yml, so a Go test
// asserting "ingest_cmd.go contains ReconcileEditorialConfig" would pass on
// the exact broken stack measured above -- that call site already existed.
// What has to be pinned is the DEPLOY composition:
//
//   1. TestDockerComposeReconcile_RunsAfterMigrationsAndGatesTheApp reads
//      the committed compose file and asserts the ordering EXPLICITLY --
//      reconciliation writes to tables migrations create, and `app` must
//      not start until the editorial rows exist, because the scheduler's
//      first ingest cycle (immediate at boot, see schedulerTicks) runs
//      rule 3 against them and then exports them into the artifact.
//   2. TestDockerComposeReconcile_TheCommittedCommandProjectsEditorialRows
//      takes the `command:` ARRAY OUT OF THAT SAME FILE -- never a literal
//      copy of it -- and runs it through the binary's real dispatch table
//      against a real migrated Postgres, then asserts the rows actually
//      land. Deleting the service, renaming it, or pointing its command at
//      something that does not reconcile fails one or both.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gopkg.in/yaml.v3"
)

// composeFile decodes only what these tests assert against. Compose's own
// `x-app-image`/`x-app-build` anchors resolve during YAML parsing, so the
// aliased `image:`/`build:` keys need no special handling here and are
// simply not decoded.
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Command   []string                     `yaml:"command"`
	Restart   string                       `yaml:"restart"`
	DependsOn map[string]composeDependency `yaml:"depends_on"`
}

type composeDependency struct {
	Condition string `yaml:"condition"`
}

// reconcileServiceName is the one-shot service docker-compose.yml runs
// between `migrate` and `app`. Named as a constant because both tests
// below look it up and a rename must fail loudly in one place.
const reconcileServiceName = "reconcile"

func loadComposeFile(t *testing.T) composeFile {
	t.Helper()
	path := filepath.Join(closureRepoRoot(t), "docker-compose.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var parsed composeFile
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return parsed
}

func TestDockerComposeReconcile_RunsAfterMigrationsAndGatesTheApp(t *testing.T) {
	compose := loadComposeFile(t)

	reconcile, ok := compose.Services[reconcileServiceName]
	if !ok {
		t.Fatalf("docker-compose.yml declares no %q service. Without it a deployed stack runs "+
			"`migrate up` and then `serve`, and NOTHING ever calls "+
			"ingestion.ReconcileEditorialConfig: series_break and event stay empty forever, so "+
			"no break band, no annotation and no rule-3 break exemption can ever fire. "+
			"Services present: %v", reconcileServiceName, serviceNames(compose))
	}

	// A one-shot job, not a service: `app` gates on its exit being 0, and a
	// restart loop here would block the whole stack rather than surface the
	// failure -- the same reasoning `migrate` already carries.
	if reconcile.Restart != "no" {
		t.Errorf("%s.restart = %q, want \"no\": it is a one-shot job `app` gates on, not a long-running service",
			reconcileServiceName, reconcile.Restart)
	}

	// THE ORDERING, MADE EXPLICIT RATHER THAN LEFT TO TIMING. Reconciliation
	// INSERTs into series_break/event, which app/migrations creates. Gating
	// on the migration having EXITED 0 -- not merely started -- is the same
	// discipline `app` already applies to `migrate`.
	if got := reconcile.DependsOn["migrate"].Condition; got != "service_completed_successfully" {
		t.Errorf("%s.depends_on.migrate.condition = %q, want \"service_completed_successfully\": "+
			"reconciliation writes to tables the migrations create, so the dependency must be on the "+
			"migration having finished, never on it merely having been started",
			reconcileServiceName, got)
	}

	// `app`'s scheduler runs its first ingest cycle AT BOOT (schedulerTicks),
	// and that cycle both evaluates rule 3 against series_break and exports
	// whatever breaks/events exist into the artifact the site renders. A
	// reconcile that lands after that cycle would leave the first published
	// artifact carrying empty breaks -- exactly the state measured on the
	// live stack -- until the next cycle 24h later.
	if got := compose.Services["app"].DependsOn[reconcileServiceName].Condition; got != "service_completed_successfully" {
		t.Errorf("app.depends_on.%s.condition = %q, want \"service_completed_successfully\": "+
			"the scheduler's first cycle runs at boot and both validates against and exports the "+
			"editorial rows, so they must already exist when `app` starts",
			reconcileServiceName, got)
	}
}

func serviceNames(c composeFile) []string {
	names := make([]string, 0, len(c.Services))
	for name := range c.Services {
		names = append(names, name)
	}
	return names
}

// TestDockerComposeReconcile_TheCommittedCommandProjectsEditorialRows runs
// the compose file's OWN command array through the binary's real dispatch
// table (realCommands -- the same table main() uses) against a real,
// migrated Postgres, and asserts the editorial rows land.
//
// The command is read out of the YAML rather than written here on purpose:
// a literal `[]string{"ingest", "--reconcile"}` would keep passing while
// the deployed stack ran something else entirely, which is the precise
// failure this whole file exists to make impossible.
func TestDockerComposeReconcile_TheCommittedCommandProjectsEditorialRows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}

	compose := loadComposeFile(t)
	reconcile, ok := compose.Services[reconcileServiceName]
	if !ok {
		t.Fatalf("docker-compose.yml declares no %q service (see the sibling ordering test for what that costs)", reconcileServiceName)
	}
	if len(reconcile.Command) == 0 {
		t.Fatalf("%s declares no command, so the deployed stack would run the image's default entrypoint arguments, not a reconcile", reconcileServiceName)
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_deploy_reconcile_test"),
		tcpostgres.WithUsername("concontexto_deploy_reconcile_test"),
		tcpostgres.WithPassword("concontexto_deploy_reconcile_test"),
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
	t.Setenv("DATABASE_URL", dsn)

	// Step 2 of the compose sequence, run exactly as the stack runs it.
	var stdout, stderr bytes.Buffer
	if code := dispatch([]string{"migrate", "up"}, realCommands(), &stdout, &stderr); code != 0 {
		t.Fatalf("migrate up: exit %d, stderr=%q", code, stderr.String())
	}

	// Step 3: the committed command, through the same dispatch table main()
	// builds, with the real embedded config/*.yaml behind it.
	stdout.Reset()
	stderr.Reset()
	if code := dispatch(reconcile.Command, realCommands(), &stdout, &stderr); code != 0 {
		t.Fatalf("%v (docker-compose.yml %s.command): exit %d, stderr=%q",
			reconcile.Command, reconcileServiceName, code, stderr.String())
	}
	out := stdout.String()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	for _, table := range []string{"series_break", "event"} {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("counting %s: %v", table, err)
		}
		if count == 0 {
			t.Errorf("%s is EMPTY after running the deployed reconcile command %v. "+
				"This is the exact state measured on the live stack before this batch.", table, reconcile.Command)
		}
	}

	// epa-metodologia-2021 is scoped to dataset ine-epa, which both
	// tasa-de-paro-epa and ocupados-epa belong to, so it is the break that
	// makes a band renderable at all. Asserting the row by key (not merely a
	// non-zero count) pins the one entry the indicator page depends on.
	var breakRows int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM series_break WHERE break_key = $1 AND retired_at IS NULL",
		"epa-metodologia-2021").Scan(&breakRows); err != nil {
		t.Fatalf("looking up epa-metodologia-2021: %v", err)
	}
	if breakRows != 1 {
		t.Errorf("expected exactly 1 active series_break row for epa-metodologia-2021, got %d: "+
			"without it publishing.Export writes an empty `breaks` array and BreakBand.astro renders nothing",
			breakRows)
	}

	// The signed acknowledgement, by key. It used to be asserted as a PENDING
	// identifier in the command's output, because the record shipped unsigned
	// and a record waiting on a human had to read as waiting rather than as
	// absent. It has since been signed, so it projects: the thing worth
	// pinning is now the row itself, since it is what unblocks ocupados-epa's
	// ingest and therefore what puts the slug in the export artifact at all.
	var ackRows int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM validation_acknowledgement WHERE ack_key = $1 AND retired_at IS NULL",
		"ocupados-epa-2020-q2-covid").Scan(&ackRows); err != nil {
		t.Fatalf("looking up ocupados-epa-2020-q2-covid: %v", err)
	}
	if ackRows != 1 {
		t.Errorf("expected exactly 1 active validation_acknowledgement row for ocupados-epa-2020-q2-covid, got %d: "+
			"without it every real ingest of ocupados-epa blocks on the COVID quarter and the slug never reaches the artifact",
			ackRows)
	}

	// THE OPERATOR-VISIBLE HALF (spec editorial-config, "Reconciliation MUST
	// report the count and identifiers of unprojected entries to operators
	// through the run's structured output"). This output is what reaches the
	// container log, so what is not printed here is invisible in production.
	for _, want := range []string{
		// Counts, not only identifiers -- the spec scenario asks for both.
		"pending=",
		// A government whose date is genuinely unconfirmed today.
		"gobierno-suarez-1976",
		// The acknowledgement counts, so an operator can see the registry
		// took effect rather than inferring it from a series that published.
		"acknowledgements inserted=1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the reconcile command's output does not contain %q, so an operator reading the "+
				"container log cannot see it. Got:\n%s", want, out)
		}
	}
}
