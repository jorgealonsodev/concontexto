package postgres_test

// Task 7.10 (RED) / 7.11 (GREEN): principle P4's "series breaks are
// non-dismissible" guarantee is strongest when "dismiss this break" is
// not REPRESENTABLE in the type system at all, not merely disallowed by
// convention (spec editorial-config, "Breaks are stored as
// non-dismissible" / "The break model exposes no dismiss affordance").
// This test inspects, by reflection, every Go field name and struct tag
// on the series_break schema's Go-side types (SeriesBreakInput,
// SeriesBreak — postgres/editorial.go) AND the domain type
// (indicators.Break) and fails if ANY field looks like a dismissible,
// optional-visibility or default-hidden attribute — so a future PR
// cannot silently add one without this test catching it, without
// needing to trust code review alone.
//
// Mutation-tested (RED confirmed, see PR 7b's apply-progress for the
// transcript): a temporary `Dismissed bool` field was added to
// SeriesBreakInput, this test was re-run and failed naming the exact
// field, then the field was reverted and the test re-confirmed passing.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// forbiddenDismissSubstrings names every wording a dismissible,
// optional-visibility or default-hidden field could plausibly carry,
// matched case-insensitively against both the Go field name and its
// struct tag (so a yaml/db tag like `db:"is_hidden"` is caught even if
// the Go field name itself were innocuous).
var forbiddenDismissSubstrings = []string{
	"dismiss", "hidden", "hide", "optional", "defaultoff", "default_off",
}

func assertNoDismissibleField(t *testing.T, v any) {
	t.Helper()
	typ := reflect.TypeOf(v)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		haystack := strings.ToLower(f.Name + " " + string(f.Tag))
		for _, forbidden := range forbiddenDismissSubstrings {
			if strings.Contains(haystack, forbidden) {
				t.Errorf("%s.%s looks dismissible/optional-visibility/default-hidden (matched %q) — principle P4 requires series_break to expose no such attribute in its type", typ.Name(), f.Name, forbidden)
			}
		}
	}
}

func TestSeriesBreakTypes_ExposeNoDismissibleAttribute(t *testing.T) {
	assertNoDismissibleField(t, postgres.SeriesBreakInput{})
	assertNoDismissibleField(t, postgres.SeriesBreak{})
	assertNoDismissibleField(t, indicators.Break{})
}
