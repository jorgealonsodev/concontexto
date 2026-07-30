package main

// Remediation batch (verify-report CRITICAL C4): design.md's Go layout
// names "serve = static server + /healthz + in-process scheduler" and
// spec pipeline-operations requires "The scheduler MUST run one job per
// source". Before this batch, scheduler.Runner was never instantiated
// outside its own package's tests: `serve.go` started only the HTTP
// server, so the 24h amber-escalation incident path (scheduler.Run ->
// alerting.SourceDown) could never fire in production.
//
// This is wiring, not redesign: scheduler.Runner (task 9.1/9.2) and its
// backoff/incident contract are unchanged. cmdServe now ALSO launches a
// scheduler loop, in its own goroutine, sharing the same shutdown ctx as
// runServe -- but neither package imports the other. httpserver still
// imports no repository port, no adapters/postgres, no pgx and (per the
// same batch's W1 fix) no source-client adapter either -- the golden
// rule (PRD §14.2, "zero DB queries at request time") and PRD §9.2 ("no
// external source call at page-request time") both hold exactly as they
// did before this file existed, because the scheduler never becomes
// reachable FROM httpserver's handler; it is composed strictly BESIDE it,
// only inside cmdServe.
//
// runScheduler is the testable core (same pattern runIngest/runServe
// already established): it takes an already-built runners map and an
// already-resolved op factory instead of resolving DATABASE_URL/the
// embedded config itself, so a test can drive it with a fake op and a
// manually-fed tick channel without Docker, and a second, real-Postgres
// test can prove one genuine end-to-end cycle.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
	"github.com/jorgealonsodev/concontexto/app/internal/scheduler"
)

// defaultScheduleInterval is how long a source waits after a SUCCESSFUL
// scheduled cycle before its next one (overridable via
// APP_SCHEDULE_INTERVAL; a failed cycle instead uses the Runner's own
// backoff via Attempt.NextAttemptAt -- see runScheduler). 24h matches
// PRD §9.2's own publication-cadence language; per-source
// dataset.refresh_calendar (an existing, still-unconsumed DDL column) is
// a finer-grained Fase 1+ refinement, not built in this compose-only
// batch.
const defaultScheduleInterval = 24 * time.Hour

// scheduleCheckInterval is how often the driving loop wakes up to check
// whether any source is due (production only; tests feed their own tick
// channel and control every tick explicitly). 15 minutes bounds the
// worst-case delay between a source becoming due and its cycle actually
// starting without polling so tightly it matters for load.
const scheduleCheckInterval = 15 * time.Minute

// scheduleInterval resolves APP_SCHEDULE_INTERVAL (a Go duration string,
// e.g. "24h"), falling back to defaultScheduleInterval when unset or
// unparsable -- unparsable is logged, never fatal: a malformed override
// must not stop `serve` from serving pages (the same resilience choice
// cmdServe itself makes for a missing DATABASE_URL, below).
func scheduleInterval(logs io.Writer) time.Duration {
	raw := os.Getenv("APP_SCHEDULE_INTERVAL")
	if raw == "" {
		return defaultScheduleInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		fmt.Fprintf(logs, "serve: scheduler: APP_SCHEDULE_INTERVAL=%q is not a valid duration, using default %s: %v\n", raw, defaultScheduleInterval, err)
		return defaultScheduleInterval
	}
	return d
}

// scheduleDisabled resolves APP_SCHEDULE_DISABLED, the opt-out that turns
// `serve` back into a pure static file server.
//
// It exists because schedulerTicks now delivers the scheduler's first tick
// AT BOOT rather than 15 minutes later (verify-report WARNING-24). That is
// required for a real deployment -- a fresh `docker compose up` must reach
// a site with real data -- but it also means any short-lived stack built
// from this compose file starts fetching from the live INE/Eurostat APIs
// the moment it comes up. A hermetic test stack (CI's container smoke
// test) needs a way to say "serve, do not ingest" that does not involve
// withholding DATABASE_URL, which the stack needs for `migrate` and for
// `healthcheck --deep`.
//
// Fails OPEN, mirroring scheduleInterval and publishLatencyBudget: an
// unparsable value is logged and treated as "not disabled", because
// silently switching the whole pipeline off over a typo is strictly worse
// than ignoring the typo.
func scheduleDisabled(logs io.Writer) bool {
	raw := os.Getenv("APP_SCHEDULE_DISABLED")
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		fmt.Fprintf(logs, "serve: scheduler: APP_SCHEDULE_DISABLED=%q is not a valid boolean, leaving the scheduler ENABLED: %v\n", raw, err)
		return false
	}
	return v
}

// scheduleSourceOp builds the op scheduler.Runner.Run needs for
// sourceID: a one-shot ingest of every series configured under that
// source, reusing runIngest's own "--source" path so a scheduled cycle
// and a manual `ingest --source=X` never diverge (C2 and C4 share one
// wiring, two callers). Its own error already names the failing series
// (runIngest's stderr); this only re-surfaces that text as the error
// scheduler.Runner.Run reports on the returned Attempt.
func scheduleSourceOp(db postgres.TxBeginner, cfg *config.Config, store *filestore.Store, archiveHashPath, publicHashPath, sourceID string, logs io.Writer) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var stdout, stderrBuf strings.Builder
		if code := runIngest(ctx, db, cfg, store, archiveHashPath, publicHashPath, time.Now().UTC(), "", sourceID,
			exportOutputDir(false), buildDispatcher(), &stdout, &stderrBuf); code != 0 {
			return fmt.Errorf("scheduled ingest for source %s: %s", sourceID, strings.TrimSpace(stderrBuf.String()))
		}
		if stdout.Len() > 0 && logs != nil {
			fmt.Fprint(logs, stdout.String())
		}
		return nil
	}
}

// runScheduler drives one scheduler.Runner per entry in runners until ctx
// is cancelled. newOp(sourceID) builds that source's op EACH cycle
// (production: scheduleSourceOp, capturing the shared db/cfg/store;
// tests: a fake closure, so backoff/interval behaviour is provable
// without Docker). tick is the driving clock -- a real *time.Ticker's
// channel in production, a manually-fed channel in tests, matching the
// same injected-clock discipline scheduler.Runner.Run itself already
// requires of its own now parameter.
//
// Every source runs on the very first tick (next seeded at zero time, so
// clock().After(zero) is always true) and is then rescheduled: interval
// after now on success, Attempt.NextAttemptAt (the Runner's own backoff)
// on failure -- exactly Attempt's documented contract, never re-derived
// here.
//
// Remediation batch (verify-report CRITICAL C6): lastSuccess used to
// start as an empty in-memory map on EVERY call to runScheduler, i.e. on
// every process start. freshness.Resolve(nil, asOf) treats "no in-memory
// success yet" identically to "never succeeded" and returns StateFailed,
// so the very first failed cycle after any restart raised a source-down
// incident claiming the source "has been down for over 24h0m0s" -- false,
// and a regression of the 24h rule the scheduler exists to implement,
// because `serve` (and therefore this loop) restarts on every deploy.
//
// seedLastSuccess breaks that conflation: the first time (this process)
// a source is actually about to run, and only then, its lastSuccess is
// seeded from seedLastSuccess (production: postgres.
// LastSuccessfulDownloadAttempt, which survives restarts because it
// reads download_attempt, not process memory) instead of defaulting to
// nil. From that point on the in-process value (updated on every real
// success, exactly as before) takes over -- seedLastSuccess is consulted
// AT MOST ONCE per source per runScheduler call, not every tick, so a
// source that keeps failing does not re-query the seed on every cycle.
// A nil seedLastSuccess (every existing test that does not care about
// this distinction) preserves the prior "unknown means nil" behaviour
// exactly.
// watchdog (task 4.11/4.12) is invoked once per tick for every source that
// already has a known lastSuccess -- deliberately DECOUPLED from next[id]'s
// own due-gating below, because a stalled rebuild has to be caught on a
// regular cadence (this loop's own scheduleCheckInterval, in production),
// not only when the source's next 24h ingest cycle happens to come back
// around. A nil watchdog (every test that does not care about this
// distinction) preserves prior behaviour exactly, mirroring
// seedLastSuccess's own nil-safe convention.
func runScheduler(ctx context.Context, runners map[string]*scheduler.Runner, newOp func(sourceID string) func(context.Context) error, interval time.Duration, tick <-chan time.Time, seedLastSuccess func(ctx context.Context, sourceID string) *time.Time, watchdog func(sourceID string, now, lastSuccess time.Time)) {
	lastSuccess := make(map[string]*time.Time, len(runners))
	seeded := make(map[string]bool, len(runners))
	next := make(map[string]time.Time, len(runners))

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick:
			for id := range runners {
				if watchdog != nil {
					if last := lastSuccess[id]; last != nil {
						watchdog(id, now, *last)
					}
				}
			}
			for id, runner := range runners {
				if now.Before(next[id]) {
					continue
				}
				if !seeded[id] {
					seeded[id] = true
					if seedLastSuccess != nil {
						lastSuccess[id] = seedLastSuccess(ctx, id)
					}
				}
				attempt := runner.Run(ctx, lastSuccess[id], now, newOp(id))
				if attempt.Err == nil {
					success := now
					lastSuccess[id] = &success
					next[id] = now.Add(interval)
				} else {
					next[id] = attempt.NextAttemptAt
				}
			}
		}
	}
}

// startScheduler wires runScheduler for production use: it connects its
// OWN pgxpool (independent of any pool runServe/the HTTP path might use
// -- there is none, by the golden rule) and loads the same embedded
// config every other subcommand loads, mirroring cmdIngest's own
// resolution of DATABASE_URL/config/APP_DATA_ROOT exactly, then launches
// the driving loop in a goroutine tied to ctx. A missing or unusable
// DATABASE_URL is reported on logs and leaves the scheduler disabled --
// static page serving must survive a missing ingestion prerequisite, the
// same resilience choice runServe already makes for a missing
// STATIC_ROOT directory.
//
// Remediation batch (verify-report CRITICAL C8/M2): the composition past
// "connect and load" used to live inline here and was executed by NO
// test (see startSchedulerLoop's doc comment). This is now a thin
// wrapper: resolve DATABASE_URL/config/root, then delegate to
// startSchedulerLoop.
func startScheduler(ctx context.Context, logs io.Writer) {
	if scheduleDisabled(logs) {
		fmt.Fprintln(logs, "serve: scheduler: APP_SCHEDULE_DISABLED is set, scheduler disabled (serving static content only)")
		return
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(logs, "serve: scheduler: DATABASE_URL not set, scheduler disabled (serving static content only)")
		return
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(logs, "serve: scheduler: connecting:", err)
		return
	}

	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		fmt.Fprintln(logs, "serve: scheduler:", err)
		pool.Close()
		return
	}
	cfg, err := config.Load(sub)
	if err != nil {
		fmt.Fprintln(logs, "serve: scheduler: loading config:", err)
		pool.Close()
		return
	}

	root := appDataRoot()
	tick, stopTicks := schedulerTicks(ctx)
	go func() {
		defer stopTicks()
		defer pool.Close()
		startSchedulerLoop(ctx, pool, cfg, root, staticAssetRoot(), tick, logs)
	}()
}

// schedulerTicks builds the PRODUCTION tick source for
// startSchedulerLoop: one tick delivered immediately, then one every
// scheduleCheckInterval until ctx is cancelled. The returned func stops
// the underlying ticker and must always be called.
//
// The immediate first tick is the whole point (verify-report
// WARNING-24). runScheduler already documents and implements "every
// source runs on the very first tick" -- next[id] starts at the zero
// time, so nothing is gated on the first pass. What was missing is that
// production handed it a bare `time.NewTicker(scheduleCheckInterval).C`,
// and a ticker's FIRST send lands one full interval after creation. The
// consequence on a clean `docker compose up` is not a slow start, it is a
// broken site: the pages ship pre-rendered in the image, but the database
// is empty and `/web/dist/data-derived` holds only whatever the image
// seeded, so every CSV/JSON download link 404s until the pipeline has run
// once -- 15 minutes of a deployment that looks up and serves nothing
// real.
//
// This deliberately changes ONLY when the first tick arrives, never the
// loop's due-gating. In particular it does not seed next[id] from the
// persisted last success: `runScheduler`'s cold-start contract is that a
// restart re-reads freshness from `download_attempt` and still runs the
// source (schedule_freshness_test.go asserts exactly that, and would be
// made vacuous by such a change). The cost is honest and bounded: a
// container restart triggers one cycle per source instead of waiting up
// to 15 minutes for one, which is the same number of third-party fetches
// per restart either way.
func schedulerTicks(ctx context.Context) (<-chan time.Time, func()) {
	ticker := time.NewTicker(scheduleCheckInterval)

	// Buffered so the immediate tick can be queued before any receiver
	// exists -- startScheduler builds the channel before the goroutine
	// that reads it.
	out := make(chan time.Time, 1)
	out <- time.Now().UTC()

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				select {
				case out <- t:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, ticker.Stop
}

// startSchedulerLoop is startScheduler's composition core, extracted for
// testability (verify-report CRITICAL C8/M2): an already-connected pool,
// an already-loaded cfg and an injected tick channel replace
// startScheduler's own DATABASE_URL/config resolution and real 15-minute
// *time.Ticker, mirroring runIngest/runServe's own "testable core"
// pattern (this file's package doc comment). Before this extraction,
// startScheduler -- including the seedLastSuccess closure that is the
// entire C6 fix -- had 0.0% statement coverage: a mutation replacing
// seedLastSuccess with nil left the whole suite green, silently
// restoring the pass-2 C6 defect. schedule_composition_test.go drives
// this function with a real Postgres pool and asserts NO incident fires
// on a cold-start failure when a recent success is genuinely persisted --
// proving seedLastSuccess reaches runScheduler non-nil, against the REAL
// wiring, not a hand-built stand-in.
func startSchedulerLoop(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, root, staticRoot string, tick <-chan time.Time, logs io.Writer) {
	store := filestore.NewStore(filepath.Join(root, "raw"))
	archiveHashPath, publicHashPath := resolveIngestPaths(root, staticRoot)

	runners := make(map[string]*scheduler.Runner, len(cfg.Sources))
	for id := range cfg.Sources {
		runners[id] = scheduler.NewRunner(id)
	}
	newOp := func(sourceID string) func(context.Context) error {
		return scheduleSourceOp(pool, cfg, store, archiveHashPath, publicHashPath, sourceID, logs)
	}

	// seedLastSuccess (verify-report CRITICAL C6, re-pinned as C8/M2) is
	// the production answer to runScheduler's cold-start question:
	// postgres.LastSuccessfulDownloadAttempt reads the real last
	// successful download_attempt for sourceID, which survives this
	// exact restart -- unlike the in-process lastSuccess map runScheduler
	// itself owns. A query failure is logged and treated as nil (the
	// same fail-safe resilience choice startScheduler already makes for
	// a missing DATABASE_URL or an unparsable APP_SCHEDULE_INTERVAL): the
	// scheduler must keep running even if one freshness lookup fails, and
	// nil is exactly what the pre-C6 code always passed, so a lookup
	// failure is never worse than the defect that batch closed.
	seedLastSuccess := func(ctx context.Context, sourceID string) *time.Time {
		last, err := postgres.LastSuccessfulDownloadAttempt(ctx, pool, sourceID)
		if err != nil {
			fmt.Fprintln(logs, "serve: scheduler: resolving last success for", sourceID, ":", err)
			return nil
		}
		return last
	}

	runScheduler(ctx, runners, newOp, scheduleInterval(logs), tick, seedLastSuccess, publishLatencyWatchdog(ctx, publishLatencyBudget(logs), logs))
}

// publishLatencyWatchdog builds runScheduler's watchdog callback (task
// 4.11/4.12, spec pipeline-operations "An ingestion not followed by a
// rebuild alerts operators"): for each tick it re-reads the newest LOCAL
// export's manifest.json (publishing.ReadManifest, the best signal this
// codebase can observe for "did the rebuild follow" without a live
// deployed URL -- PORTAINER_WEBHOOK_URL/VPS provisioning remains a
// separate, disclosed dependency, design.md's own Open Questions, blocking
// end-to-end DEPLOY verification only, never this budget check) and
// compares it against sourceID's last known successful ingestion via
// scheduler.PublishLatencyBreached -- the exact same pure decision
// app/internal/scheduler/watchdog.go's own tests exercise offline. A
// breach raises alerting.PublishLatencyBreach, source-level (empty
// series), the same established convention Alert.Series' own doc comment
// already documents for KindSourceDown -- this watchdog observes one
// export cycle covering every series at once, not a single series in
// isolation.
//
// A manifest.json that has NEVER been written (os.ErrNotExist) is itself a
// meaningful, comparable signal -- the zero time.Time it resolves to is
// "before" any real success, so PublishLatencyBreached correctly treats
// "the export has never run at all" as a breach once budget has elapsed,
// exactly like a stale one. A genuinely UNREADABLE manifest (corrupt JSON,
// a permission error) is different: this codebase's own best-effort
// convention for every other watchdog/alert path is to log and skip that
// tick rather than risk raising or suppressing an alert off garbage data.
// THE SECOND COMPARISON (verify-report CRITICAL-28, link 3: "No
// deploy-completed instant exists anywhere"). The check described above can
// only ever observe the EXPORT step, because the manifest it reads is the
// one publishing.Publish wrote seconds earlier in the same call -- a
// dispatch that was never sent, a rebuild that failed and a deploy that
// never landed are all invisible to it. The live stack sat in exactly that
// state: /data-derived advanced every cycle while the pre-rendered pages
// stayed frozen at the image build, and nothing raised a sound.
//
// deployedManifestPath (the image's own build manifest, written by the
// Dockerfile from the artifact the pages were RENDERED from) supplies the
// missing instant. It cannot change without a new image, and a new image
// cannot arrive without the dispatch, the CI rebuild and the Portainer
// redeploy all having succeeded -- so one comparison covers every remaining
// link at once, from inside the container, with no call to GitHub, to
// Portainer or to the public site.
//
// It declines to fire in two cases, both deliberate:
//
//   - Rebuild dispatch is deliberately off (APP_REBUILD_DISPATCH, see
//     rebuild_dispatch.go). Divergence is then the operator's own stated
//     intent, not a fault, and paging for it would train them to ignore
//     this alert.
//   - The deployed artifact is UNKNOWN -- no build manifest, i.e. a bare
//     binary or an image built before the stamp existed. Unknown is not
//     stale, and reporting a deploy failure this process cannot observe
//     would be the same fabricated verdict the whole remediation exists to
//     avoid. Which of the two the process is in is recorded once, at
//     construction, so an operator can see whether the check is live rather
//     than having to infer it from silence.
//
// The two comparisons never double-page: a stale EXPORT returns before the
// deploy comparison runs, because until the export catches up there is no
// current artifact for a rebuild to be late for.
func publishLatencyWatchdog(ctx context.Context, budget time.Duration, logs io.Writer) func(sourceID string, now, lastSuccess time.Time) {
	manifestPath := filepath.Join(exportOutputDir(false), "manifest.json")
	deployedPath := deployedManifestPath()
	rebuildExpected := rebuildDispatchExpected()
	logDeployWatchdogState(deployedPath, rebuildExpected)

	return func(sourceID string, now, lastSuccess time.Time) {
		var generatedAt time.Time
		if m, err := publishing.ReadManifest(manifestPath); err == nil {
			generatedAt = m.GeneratedAt
		} else if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(logs, "serve: scheduler: reading the local manifest for the publish-latency watchdog:", err)
			return
		}
		if breached, elapsed := scheduler.PublishLatencyBreached(now, lastSuccess, generatedAt, budget); breached {
			if err := alerting.PublishLatencyBreach(ctx, alerting.DefaultSink(), sourceID, "", elapsed); err != nil {
				fmt.Fprintln(logs, "serve: scheduler: raising publish-latency-breach alert for", sourceID, ":", err)
			}
			return
		}

		if !rebuildExpected {
			return
		}
		deployed, err := publishing.ReadManifest(deployedPath)
		if err != nil {
			// Unknown, never stale. Only a genuinely unreadable (as opposed
			// to absent) build manifest is worth a line, matching the
			// best-effort convention the live-manifest read above already
			// establishes.
			if !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintln(logs, "serve: scheduler: reading the deployed build manifest for the rebuild watchdog:", err)
			}
			return
		}
		if breached, elapsed := scheduler.RebuildLatencyBreached(now, generatedAt, deployed.GeneratedAt, budget); breached {
			if err := alerting.PublishLatencyBreach(ctx, alerting.DefaultSink(), sourceID, "", elapsed); err != nil {
				fmt.Fprintln(logs, "serve: scheduler: raising publish-latency-breach alert for", sourceID, ":", err)
			}
		}
	}
}

// deployedManifestPath resolves where the running image records the
// artifact its pre-rendered pages were BUILT from (APP_BUILD_MANIFEST,
// defaulting to the path the Dockerfile writes).
//
// /web/build-manifest.json is deliberately NOT under STATIC_ROOT
// (/web/dist): docker-compose.yml mounts the export_artifact volume over
// /web/dist/data-derived, so a stamp written there would either be shadowed
// by the volume or served to readers as though it were part of the
// published artifact. Outside dist it is shipped by the image, replaced only
// by a deploy, and reachable by no HTTP route.
func deployedManifestPath() string {
	if p := os.Getenv("APP_BUILD_MANIFEST"); p != "" {
		return p
	}
	return "/web/build-manifest.json"
}

// logDeployWatchdogState records once, at start-up, whether the
// deploy-completed comparison is actually live -- so "no rebuild alert has
// ever fired" can be read as "the deploy is keeping up" rather than "the
// check was never running", which are the two readings CRITICAL-28 showed
// are impossible to tell apart from silence alone.
func logDeployWatchdogState(deployedPath string, rebuildExpected bool) {
	switch {
	case !rebuildExpected:
		slog.Info("deploy-completed watchdog is inactive: rebuild dispatch is off, so divergence between the published artifact and the deployed pages is expected and will not be alerted",
			slog.String("component", deployWatchdogComponent),
			slog.String("state", "inactive"),
			slog.String("reason", "APP_REBUILD_DISPATCH is unset or off"))
	case !fileExists(deployedPath):
		slog.Warn("deploy-completed watchdog is inactive: this build carries no build manifest, so the artifact the deployed pages were rendered from is unknown and a stalled rebuild cannot be detected",
			slog.String("component", deployWatchdogComponent),
			slog.String("state", "inactive"),
			slog.String("reason", "no build manifest at "+deployedPath))
	default:
		slog.Info("deploy-completed watchdog is active: the published artifact will be compared against the artifact the deployed pages were rendered from",
			slog.String("component", deployWatchdogComponent),
			slog.String("state", "active"),
			slog.String("build_manifest", deployedPath))
	}
}

// deployWatchdogComponent tags the records above, mirroring
// rebuildDispatchComponent's own convention in rebuild_dispatch.go.
const deployWatchdogComponent = "deploy-watchdog"

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// publishLatencyBudget resolves APP_PUBLISH_LATENCY_BUDGET (a Go duration
// string, e.g. "30m"), falling back to scheduler.DefaultPublishLatencyBudget
// when unset or unparsable -- mirrors scheduleInterval's own resilience
// convention exactly (spec pipeline-operations: "The budget is
// configuration, not a constant ... resolves from configuration with a
// documented default of 30 minutes").
func publishLatencyBudget(logs io.Writer) time.Duration {
	raw := os.Getenv("APP_PUBLISH_LATENCY_BUDGET")
	if raw == "" {
		return scheduler.DefaultPublishLatencyBudget
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		fmt.Fprintf(logs, "serve: scheduler: APP_PUBLISH_LATENCY_BUDGET=%q is not a valid duration, using default %s: %v\n", raw, scheduler.DefaultPublishLatencyBudget, err)
		return scheduler.DefaultPublishLatencyBudget
	}
	return d
}
