package probe_test

// odd/tasks/probe-transport-vs-churn.md T1 (RED): probe.Classify is the
// single decision point that separates "the source never answered" from
// "the source answered and the answer was wrong" -- the distinction
// live_test.go itself cannot make today (verified problem #2 in the task
// doc: both cases print the identical "possible identifier churn"
// message). This test does not invent a second taxonomy; it proves
// Classify defers entirely to sourceerr.FailureClass.Retryable(), which
// ingestion/sourceerr already owns (sourceerr.go's own doc comment: "Only
// RetryableTransport is ever retried... every other class means the
// source DID answer and the answer was wrong").

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
	"github.com/jorgealonsodev/concontexto/app/internal/probe"
)

func TestClassify_NilErrorIsHealthy(t *testing.T) {
	if got := probe.Classify(nil); got != probe.OutcomeHealthy {
		t.Fatalf("Classify(nil) = %s, want %s", got, probe.OutcomeHealthy)
	}
}

func TestClassify_RetryableTransportIsUnreachable(t *testing.T) {
	// A RetryableTransport error means the source never answered at all
	// (5xx, timeout, network error, see sourceerr.go) -- exactly the
	// runner-vs-INE egress case the task doc verified (problem #1): INE
	// refused the connection from the same runner egress that Eurostat
	// reached fine, so no identifier churned.
	err := sourceerr.New(sourceerr.RetryableTransport, "connect: connection refused")
	if got := probe.Classify(err); got != probe.OutcomeUnreachable {
		t.Fatalf("Classify(RetryableTransport) = %s, want %s", got, probe.OutcomeUnreachable)
	}
}

func TestClassify_WrappedRetryableTransportIsStillUnreachable(t *testing.T) {
	// fetchWithRetry's caller (this probe's own live_test.go, and every
	// other consumer of an adapter error) wraps the classified error with
	// %w while adding request context -- Classify MUST recover the class
	// through errors.As, never a type assertion, or a single fmt.Errorf
	// wrap anywhere in the call chain silently turns Unreachable into
	// Broken.
	base := sourceerr.New(sourceerr.RetryableTransport, "dial tcp: i/o timeout")
	wrapped := fmt.Errorf("probe request for fake-ine-series (source ref FAKE001) failed: %w", base)
	if got := probe.Classify(wrapped); got != probe.OutcomeUnreachable {
		t.Fatalf("Classify(wrapped RetryableTransport) = %s, want %s", got, probe.OutcomeUnreachable)
	}
}

func TestClassify_NonRetryableClassesAreBroken(t *testing.T) {
	// Every class besides RetryableTransport means the source DID answer
	// and the answer itself is what is wrong -- an HTTP 200 volume
	// restriction envelope, a structurally valid but empty payload, a
	// renamed field or periodicity mismatch, or an oversized response.
	// None of these recover by retrying (sourceerr.go: "permanent for the
	// current attempt"), so all four must alert exactly like the
	// probe already alerts today.
	cases := []struct {
		name  string
		class sourceerr.FailureClass
	}{
		{"source refusal", sourceerr.SourceRefusal},
		{"silent empty", sourceerr.SilentEmpty},
		{"schema drift", sourceerr.SchemaDrift},
		{"response too large", sourceerr.ResponseTooLarge},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := sourceerr.New(tc.class, "boom")
			if got := probe.Classify(err); got != probe.OutcomeBroken {
				t.Fatalf("Classify(%s) = %s, want %s", tc.class, got, probe.OutcomeBroken)
			}
		})
	}
}

func TestClassify_UnclassifiedErrorFailsClosedToBroken(t *testing.T) {
	// An error this package does not recognise (not a *sourceerr.Error at
	// all -- a bug in a future adapter, a panic recovered into a plain
	// error, an errors.New typo) must still alert. The probe exists to
	// catch identifier churn; silently swallowing an error this package
	// cannot classify would recreate exactly the muted-alarm failure mode
	// the task doc's "Why" section describes, just one layer down.
	err := errors.New("boom")
	if got := probe.Classify(err); got != probe.OutcomeBroken {
		t.Fatalf("Classify(unclassified) = %s, want %s (fail closed)", got, probe.OutcomeBroken)
	}
}
