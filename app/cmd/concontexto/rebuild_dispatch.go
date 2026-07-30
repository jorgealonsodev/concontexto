package main

// Rebuild-dispatch composition (verify-report CRITICAL-28, link 2: "The
// dispatcher is never composed in the deployed stack").
//
// THE DEFECT THIS FILE EXISTS TO CLOSE. buildDispatcher used to read
// GITHUB_DISPATCH_REPO/GITHUB_DISPATCH_TOKEN and return a bare nil when
// either was empty; publishing.Publish treats a nil Dispatcher as "not
// configured, skip dispatch" and returns silently. docker-compose.yml
// passed neither variable to the `app` service, so every deployed publish
// cycle exported the artifact, dispatched nothing, and said nothing --
// while the spec's own alert set (pipeline-operations, "Operational
// alerts") names "a failed or UNDISPATCHED site rebuild". The pages stayed
// frozen at whatever the image was built from while /data-derived moved on
// every cycle, and nothing anywhere raised a sound.
//
// THE DISTINCTION THAT FIXES IT. "Not configured" is a legitimate state for
// local development, for `go test`, and for a hermetic stack that must come
// up without reaching GitHub. It is NOT a legitimate state for a deployment
// whose whole purpose is to publish. The difference is not something this
// process can infer -- it is whether the operator ever ASKED for
// dispatching -- so it is made explicit, once, by APP_REBUILD_DISPATCH:
//
//	off (or unset)  dispatch is deliberately off. Nil dispatcher, one INFO
//	                record, no alerts ever. The default for a bare binary.
//	required        a rebuild dispatch is expected. Configured means
//	                dispatch; UNCONFIGURED means BROKEN -- an immediately
//	                failing dispatcher, so publishing.Publish takes its
//	                existing dispatch-failure branch and raises
//	                alerting.DispatchFailed naming the missing variable.
//
// docker-compose.yml defaults the `app` service to `required`, so a
// DEPLOYED stack alarms when the dispatch wiring is missing, and a bare
// `go run ./app/cmd/concontexto ingest` on a laptop stays quiet. The
// deployed-vs-local distinction therefore needs no operator memory: it is
// the compose file, which is exactly what separates the two situations.
//
// Both states reach the STRUCTURED log (slog, the same slog.Default()
// destination alerting.LogSink already writes to), tagged
// component=rebuild-dispatch with a state of disabled / enabled /
// unconfigured -- so "deliberately off" and "broken" are two different
// records at two different levels, greppable, not two readings of the same
// silence.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/github"
	"github.com/jorgealonsodev/concontexto/app/internal/publishing"
)

// rebuildDispatchComponent tags every structured record this file emits, so
// an operator can select the whole rebuild-dispatch story out of the
// container log with one filter.
const rebuildDispatchComponent = "rebuild-dispatch"

// rebuildDispatchExpected reports whether this deployment expects a rebuild
// dispatch, from APP_REBUILD_DISPATCH.
//
// It fails LOUD, not open, and that is the opposite of scheduleInterval /
// scheduleDisabled / publishLatencyBudget's own fail-open convention in
// schedule.go. The asymmetry is deliberate: those knobs fail open BECAUSE
// the loud outcome (keep serving, keep scheduling) is the safe one. Here
// the quiet outcome is the unsafe one -- silently disabling the publish
// loop over a typo is precisely the failure CRITICAL-28 recorded -- so an
// unrecognised value is treated as `required` and reported, never as `off`.
// "false" and "0" are accepted alongside "off" for one concrete reason, not
// for generality: an unquoted `off` in a compose file is YAML 1.1's boolean
// FALSE, so `APP_REBUILD_DISPATCH: off` (no quotes) reaches this process as
// the string "false". Rejecting that would turn an operator's clear
// intention into the loud path over a quoting rule they cannot see, which
// is noise, not honesty. Every OTHER unrecognised value still fails loud.
func rebuildDispatchExpected() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_REBUILD_DISPATCH"))) {
	case "", "off", "false", "0":
		return false
	case "required":
		return true
	default:
		return true
	}
}

// buildDispatcher resolves the rebuild-trigger adapter (design D-2:
// "adapter adapters/github/ POSTs a repository_dispatch ... with a
// fine-grained token from env") from GITHUB_DISPATCH_REPO ("owner/repo")
// and GITHUB_DISPATCH_TOKEN, under the APP_REBUILD_DISPATCH mode above.
//
// GITHUB_DISPATCH_* is deliberately named apart from the
// GitHub-Actions-reserved GITHUB_* variable family (e.g. GITHUB_REPOSITORY,
// auto-set inside every Actions runner) so this production-side config can
// never be shadowed by a value Actions itself injects.
//
// It returns nil ONLY in the deliberately-off case. When a dispatch is
// expected but unconfigured it returns a dispatcher that fails immediately:
// nil would take publishing.Publish's silent-skip path, and that silence is
// the whole defect. A failing dispatcher instead travels the already-built,
// already-tested dispatch-failure branch -- alert, no retry, never a failed
// ingest (design D-2) -- so the undispatched case reuses the exact
// machinery the failed case has always used, with a message naming which
// variable is missing.
func buildDispatcher() publishing.Dispatcher {
	repo := strings.TrimSpace(os.Getenv("GITHUB_DISPATCH_REPO"))
	token := os.Getenv("GITHUB_DISPATCH_TOKEN")
	expected := rebuildDispatchExpected()

	if !expected {
		slog.Info("rebuild dispatch is disabled: no site rebuild will be triggered by a publish cycle",
			slog.String("component", rebuildDispatchComponent),
			slog.String("state", "disabled"),
			slog.String("reason", "APP_REBUILD_DISPATCH is unset or off"))
		return nil
	}

	if missing := missingDispatchVars(repo, token); len(missing) > 0 {
		verb := "is"
		if len(missing) > 1 {
			verb = "are"
		}
		reason := fmt.Sprintf("a rebuild dispatch is expected (APP_REBUILD_DISPATCH) but %s %s not set", strings.Join(missing, " and "), verb)
		slog.Error("rebuild dispatch is expected but not configured: every publish cycle will raise a dispatch-failed alert until this is fixed",
			slog.String("component", rebuildDispatchComponent),
			slog.String("state", "unconfigured"),
			slog.String("reason", reason))
		return func(context.Context, time.Time, string) error {
			return fmt.Errorf("github: no rebuild dispatch was sent: %s", reason)
		}
	}

	slog.Info("rebuild dispatch is enabled: each publish cycle will trigger a site rebuild",
		slog.String("component", rebuildDispatchComponent),
		slog.String("state", "enabled"),
		slog.String("repo", repo))
	client := github.NewClient(repo, token, nil)
	return client.Dispatch
}

// missingDispatchVars names the dispatch variables that are still empty, in
// a stable order, so the alert and the log record both say exactly what an
// operator has to set. The token's VALUE is never included anywhere -- only
// its name.
func missingDispatchVars(repo, token string) []string {
	var missing []string
	if repo == "" {
		missing = append(missing, "GITHUB_DISPATCH_REPO")
	}
	if token == "" {
		missing = append(missing, "GITHUB_DISPATCH_TOKEN")
	}
	return missing
}

// publishOutcomeMessage renders one publish cycle's log line, naming which
// of publishing.PublishResult's three dispatch states the cycle actually
// ended in, and reports whether that state is a FAULT (so the caller can
// choose stdout or stderr).
//
// runIngest used to print "exported and dispatched" for all three -- in
// particular for the deployed stack, where the dispatcher was nil and
// nothing at all was dispatched (verify-report CRITICAL-28, link 2). A log
// line that claims a rebuild was triggered when none was is worse than no
// line: it is the evidence an operator would consult to rule the dispatch
// OUT as the cause of a frozen site.
//
// A skipped dispatch is deliberately NOT a fault: an operator who set
// APP_REBUILD_DISPATCH off (or who is running the binary on a laptop) asked
// for exactly this, and reporting it on stderr would train them to ignore
// stderr. A FAILED dispatch is a fault, and has additionally already raised
// alerting.DispatchFailed from inside publishing.Publish.
func publishOutcomeMessage(outDir, reason string, result publishing.PublishResult) (message string, fault bool) {
	switch {
	case result.DispatchedAt != nil:
		return fmt.Sprintf("ingest: publish: exported and dispatched to %s (%s)", outDir, reason), false
	case result.DispatchSkipped:
		return fmt.Sprintf("ingest: publish: exported to %s (%s); rebuild dispatch is disabled, so the deployed pages will NOT be rebuilt from this artifact", outDir, reason), false
	default:
		return fmt.Sprintf("ingest: publish: exported to %s (%s); rebuild dispatch failed (alerted), so the deployed pages have NOT been rebuilt from this artifact", outDir, reason), true
	}
}
