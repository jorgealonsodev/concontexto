//go:build live

package probe_test

// Task 9.3/9.4, rewired by odd/tasks/probe-transport-vs-churn.md: the
// synthetic daily probe itself. This file carries the `live` build tag
// SPECIFICALLY so `go test ./...` never compiles it, let alone runs it or
// opens a socket (spec "The probe never runs in the default test suite":
// "no probe test executes and no network call is made"). It is wired as
// its own scheduled workflow (.github/workflows/probe.yml), separate
// from ci.yml, per this batch's own instruction.
//
// Every identifier this file touches is resolved through probe.Targets
// from the real embedded /config tree -- never a Go literal -- matching
// the origin-identifier guard's own convention and this batch's explicit
// constraint ("the probe must read its endpoints from the embedded
// config, not from hard-coded strings").
//
// Shape, never values (spec "The probe asserts shape, not values"): each
// target's response is decoded through the SAME adapter Decode function
// production ingestion uses (ine.DecodeSeries / eurostat.Decode), which
// asserts structural shape and periodicity but inspects no specific
// numeric value -- exactly the property that makes a legitimately
// updated latest value a non-failure, while a renamed field, a changed
// periodicity, or the INE volume-restriction/Eurostat oversized-response
// envelopes still fail the probe (spec "The probe detects a broken
// identifier").
//
// Transport vs. shape (odd/tasks/probe-transport-vs-churn.md, verified
// 2026-09-17): the ORIGINAL version of this file could not tell "the
// source never answered" from "the source answered and the answer was
// wrong" -- both printed the identical "possible identifier churn"
// message. That is false alarm material: servicios.ine.es answers HTTP
// 200 in 0.32s from a normal host but refuses the connection from a
// GitHub-hosted runner's egress, while Eurostat passes fine on that SAME
// runner in the same run -- runner egress is healthy, no identifier
// churned, INE's own edge simply blocks this runner's IP. A probe that
// cannot distinguish that from a genuinely renamed field or a churned
// COD trains whoever reads the daily alert to ignore it, and the failure
// mode this probe exists to catch (real identifier churn) goes unnoticed
// the day it actually happens.
//
// Every fetch and decode error below now routes through the SAME single
// decision point, probe.Classify (app/internal/probe/outcome.go), which
// defers to sourceerr.FailureClass.Retryable() -- the taxonomy every
// source adapter already shares. probe.OutcomeUnreachable (the source
// never answered: network error, timeout, 5xx) skips the subtest with a
// message that says plainly this is a transport problem, NOT churn.
// probe.OutcomeBroken (the source answered and the shape/identifier was
// wrong, or an error this package does not even recognise -- fail
// closed) still fails the subtest with the original "possible identifier
// churn" wording, so a real churn keeps alerting exactly as before. A
// decode failure is Broken by construction (a body that DID arrive and
// failed to parse is never RetryableTransport), so routing it through
// the identical Classify call is correct, not a special case: one
// decision point, not two copies that could silently drift apart.
//
// Each subtest previously shared ONE deadline-less context.Background()
// and a 5-attempt retry budget per adapter default (30s timeout each) --
// the corrected version of verify-report WARNING W6's remediation
// comment in .github/workflows/probe.yml explains why that combination
// let a run of dead INE targets outlive Go's own default 10-minute
// `-timeout` and panic the binary before the job-level ceiling ever
// engaged. Every subtest now gets its OWN bounded context
// (perTargetTimeout) and both clients are constructed with a short probe
// HTTP timeout and a reduced retry budget (probeHTTPTimeout,
// probeMaxAttempts below) sized for the tiny nult=1 / lastTimePeriod=1
// request this probe actually issues, not the full-history request
// production ingestion issues.
import (
	"context"
	"io/fs"
	"net/http"
	"testing"
	"time"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/probe"
)

// probeHTTPTimeout bounds a SINGLE request this probe issues. A healthy
// source answers this probe's minimal nult=1 / lastTimePeriod=1 query in
// well under a second (verified problem #1: 0.32s for INE, 0.2-1.7s for
// Eurostat, both from a normal host); 10s is generous headroom for that
// tiny request while still failing a genuinely unresponsive source fast
// instead of riding out the adapter's own 30s production default
// (adapters/{ine,eurostat}.defaultHTTPTimeout), which exists to tolerate
// a full-history DATOS_SERIE fetch this probe never makes.
const probeHTTPTimeout = 10 * time.Second

// probeMaxAttempts bounds the RetryableTransport retry budget per target
// (both adapters' own WithMaxAttempts option). The adapter default (5)
// is sized for production ingestion, where a transient 5xx is worth
// riding out; this probe runs daily specifically to catch a broken
// identifier before that window, so 2 attempts is enough to rule out a
// single flaky response without letting one dead-from-this-runner target
// (problem #3: 6 INE targets x the OLD 5-attempt/30s/5s-backoff budget
// was ~155s each, ~15.5 minutes total) burn most of the job's timeout
// budget on a target this file will skip anyway.
const probeMaxAttempts = 2

// perTargetTimeout bounds one subtest's ENTIRE fetch-plus-retry attempt,
// replacing the shared, deadline-less context.Background() the pre-fix
// version of this file passed to every target. Worst case with
// probeMaxAttempts=2 and probeHTTPTimeout=10s is two full timeouts plus
// one backoff wait between them (both adapters' defaultBackoff is
// attempt*500ms, so 1*500ms after the first failed attempt) -- 20.5s;
// perTargetTimeout leaves comfortable headroom above that for scheduling
// jitter and the (negligible) decode step, without coming close to the
// -timeout budget set on the `go test -tags live` invocation in
// .github/workflows/probe.yml.
const perTargetTimeout = 25 * time.Second

func TestLiveProbe_EveryConfiguredEndpointRespondsWithValidShape(t *testing.T) {
	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	targets := probe.Targets(cfg)
	if len(targets) == 0 {
		t.Fatal("expected at least one probe target resolved from the embedded config")
	}

	for _, tgt := range targets {
		tgt := tgt
		t.Run(tgt.SeriesSlug, func(t *testing.T) {
			freq := indicators.Frequency(tgt.Frequency)

			ctx, cancel := context.WithTimeout(context.Background(), perTargetTimeout)
			defer cancel()

			probeHTTPClient := &http.Client{Timeout: probeHTTPTimeout}

			switch tgt.Kind {
			case "ine-series-cod":
				client := ine.NewClient(tgt.BaseURL, probeHTTPClient, ine.WithMaxAttempts(probeMaxAttempts))
				raw, fetchErr := client.FetchProbe(ctx, tgt.Ref, 1)
				if reportOutcome(t, tgt.SeriesSlug, tgt.Ref, "request", fetchErr) {
					_, decodeErr := ine.DecodeSeries(raw, tgt.Ref, freq)
					reportOutcome(t, tgt.SeriesSlug, tgt.Ref, "response shape/periodicity decode", decodeErr)
				}
			case "eurostat-dataset":
				client := eurostat.NewClient(tgt.BaseURL, tgt.Filters, probeHTTPClient, eurostat.WithMaxAttempts(probeMaxAttempts))
				raw, fetchErr := client.FetchProbe(ctx, tgt.Ref, 1)
				if reportOutcome(t, tgt.SeriesSlug, tgt.Ref, "request", fetchErr) {
					_, decodeErr := eurostat.Decode(raw, tgt.Ref, freq)
					reportOutcome(t, tgt.SeriesSlug, tgt.Ref, "response shape decode", decodeErr)
				}
			default:
				t.Fatalf("unexpected probe target kind %q", tgt.Kind)
			}
		})
	}
}

// reportOutcome is the ONE place a fetch or decode error becomes a skip,
// a failure, or a pass-through, for every target kind and both the
// request step and the decode step -- written once rather than copied
// per kind/step, so the skip-vs-fail decision cannot silently drift
// between an INE and a Eurostat subtest, or between the request and
// decode halves of the same subtest. It reports true when the caller
// should proceed (err was nil, probe.OutcomeHealthy) and false when it
// already ended the subtest (Skipf/Fatalf, both of which stop the
// current goroutine via runtime.Goexit -- the false return only matters
// so the caller's own "if" reads naturally; execution never actually
// continues past a false return in the same call frame).
func reportOutcome(t *testing.T, slug, ref, stage string, err error) bool {
	t.Helper()

	switch probe.Classify(err) {
	case probe.OutcomeUnreachable:
		// The source never answered this attempt (network error,
		// timeout, or 5xx -- sourceerr.RetryableTransport). Skip, not
		// fail: this is a transport problem, NOT identifier churn (the
		// exact distinction odd/tasks/probe-transport-vs-churn.md
		// exists to draw), and the daily job must stay GREEN so a real
		// churn elsewhere in the same run keeps its alert meaningful.
		t.Skipf("probe %s for %s (source ref %s) found the source UNREACHABLE, not identifier churn: %v", stage, slug, ref, err)
		return false
	case probe.OutcomeBroken:
		// The source DID answer and the answer itself is wrong (a
		// refusal, an empty payload, a renamed field, a periodicity
		// mismatch, an oversized response) -- or this is an error class
		// probe.Classify does not even recognise, which fails closed to
		// Broken rather than passing silently. Either way, this is the
		// original alarm this probe exists to raise.
		t.Fatalf("probe %s for %s (source ref %s) failed -- possible identifier churn: %v", stage, slug, ref, err)
		return false
	default: // probe.OutcomeHealthy
		return true
	}
}
