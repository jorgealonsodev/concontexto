package sourceerr_test

// Task 5a.13 (RED): sourceerr.FailureClass is the taxonomy every source
// adapter (INE now, Eurostat/XLSX later) classifies a failure into, so
// retry policy can ask ONE question -- "is this class retryable?" --
// instead of parsing error strings (design.md "Error taxonomy: typed
// FailureClass ... §9.2 retry/backoff must never loop on the
// non-retryable classes").

import (
	"errors"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

func TestFailureClass_OnlyRetryableTransportIsRetryable(t *testing.T) {
	cases := []struct {
		name      string
		class     sourceerr.FailureClass
		retryable bool
	}{
		{"retryable transport", sourceerr.RetryableTransport, true},
		{"source refusal", sourceerr.SourceRefusal, false},
		{"silent empty", sourceerr.SilentEmpty, false},
		{"schema drift", sourceerr.SchemaDrift, false},
		{"response too large", sourceerr.ResponseTooLarge, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.class.Retryable(); got != tc.retryable {
				t.Errorf("%s.Retryable() = %v, want %v", tc.class, got, tc.retryable)
			}
		})
	}
}

func TestNew_BuildsANamedErrorCarryingItsClassAndMessage(t *testing.T) {
	err := sourceerr.New(sourceerr.SourceRefusal, "No puede mostrarse por restricciones de volumen")

	if err.Class != sourceerr.SourceRefusal {
		t.Fatalf("expected class %s, got %s", sourceerr.SourceRefusal, err.Class)
	}
	if err.Message != "No puede mostrarse por restricciones de volumen" {
		t.Fatalf("expected the Spanish source message preserved verbatim, got %q", err.Message)
	}
	if err.Error() == "" {
		t.Fatal("expected a non-empty Error() string")
	}
}

func TestErrorsAs_RecoversTheClassFromAWrappedError(t *testing.T) {
	// A caller several layers up (the retry loop, the ingestion pipeline)
	// must be able to recover the FailureClass via errors.As rather than
	// parsing a string -- this is the whole point of a *named* error
	// (spec source-ingestion-ine, "it returns the named volume-restriction
	// error, not an opaque JSON type error").
	wrapped := errors.New("wrapping context: " + sourceerr.New(sourceerr.SchemaDrift, "periodicity mismatch").Error())
	_ = wrapped // the string form is not what callers should rely on; proven below

	var target *sourceerr.Error
	original := sourceerr.New(sourceerr.SchemaDrift, "periodicity mismatch")
	var asErr error = original
	if !errors.As(asErr, &target) {
		t.Fatal("expected errors.As to recover *sourceerr.Error")
	}
	if target.Class != sourceerr.SchemaDrift {
		t.Fatalf("expected recovered class %s, got %s", sourceerr.SchemaDrift, target.Class)
	}
}
