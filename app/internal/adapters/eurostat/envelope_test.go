package eurostat_test

// Task 2b.1-2b.5 (RED/GREEN): the decoder MUST read the JSON-stat
// "status" map at the same linear pos it already computes for "value"
// (spec source-ingestion-eurostat, "JSON-stat status is read at the
// computed position") and classify each flag per design D-3's mapping
// table -- "p" -> provisional, absence -> definitive (never a schema
// drift), "b"/"d" -> break/definition metadata routed beside
// series_break (never into the status enum), anything else -> fail
// closed as sourceerr.SchemaDrift naming the dataset and the flag.
//
// No real recorded Eurostat response verified live in this project
// (testdata/source.txt) happens to carry a "status" map (confirmed:
// none of the three checked-in fixtures declares one) -- this file
// therefore INJECTS a synthetic "status" key into the real, live-
// verified fixtures at test time (withInjectedStatus below), rather than
// hand-typing a whole synthetic JSON-stat document: every dimension,
// size, id-ordering and value this test exercises is still the real
// recorded shape, only the "status" map itself is synthetic. posForLabel
// independently recomputes the linear position from the fixture's own
// id/size/dimension content -- the same generic Horner formula
// envelope.go's Decode uses -- so this test's own expectation is not
// just "the decoder agrees with itself" (matching this file's sibling
// client_test.go's own stated testing philosophy).

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

type wireDimForTest struct {
	Category struct {
		Index map[string]int `json:"index"`
	} `json:"category"`
}

// posForLabel independently recomputes the linear JSON-stat position for
// the observation carrying time label label, using only raw's own
// id/size/dimension content -- never a value read from the production
// decoder.
func posForLabel(t *testing.T, raw []byte, label string) int {
	t.Helper()
	var wire struct {
		ID        []string                  `json:"id"`
		Size      []int                     `json:"size"`
		Dimension map[string]wireDimForTest `json:"dimension"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("posForLabel: parsing fixture: %v", err)
	}
	sizeByDim := make(map[string]int, len(wire.ID))
	for i, dim := range wire.ID {
		if i < len(wire.Size) {
			sizeByDim[dim] = wire.Size[i]
		}
	}
	timeIdx, ok := wire.Dimension["time"].Category.Index[label]
	if !ok {
		t.Fatalf("posForLabel: fixture carries no time label %q", label)
	}
	pos := 0
	for _, dim := range wire.ID {
		idx := timeIdx
		if dim != "time" {
			cat := wire.Dimension[dim].Category.Index
			if len(cat) != 1 {
				t.Fatalf("posForLabel: dimension %q does not carry exactly one pinned category", dim)
			}
			for _, i := range cat {
				idx = i
			}
		}
		pos = pos*sizeByDim[dim] + idx
	}
	return pos
}

// withInjectedStatus returns raw with a top-level "status" key added
// (or replaced), carrying entries keyed by linear position exactly like
// JSON-stat 2.0's own "value" map -- everything else in raw is
// untouched.
func withInjectedStatus(t *testing.T, raw []byte, entries map[string]string) []byte {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("withInjectedStatus: parsing fixture: %v", err)
	}
	statusBytes, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("withInjectedStatus: marshalling injected status map: %v", err)
	}
	doc["status"] = statusBytes
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("withInjectedStatus: re-marshalling fixture: %v", err)
	}
	return out
}

func observationAt(t *testing.T, result indicators.SourceResult, period string) indicators.Observation {
	t.Helper()
	for _, o := range result.Observations {
		if o.Period.String() == period {
			return o
		}
	}
	t.Fatalf("expected an observation at period %s, got %+v", period, result.Observations)
	return indicators.Observation{}
}

// TestDecode_StatusFlagsAlignWithTheirComputedPositionAcrossNonContiguousEntries
// is 2b.1/2b.2's RED/GREEN proof: a "p" flag at one position and a "b"
// flag at a non-adjacent position (skipping the position in between,
// which carries no entry) each land on exactly the observation the same
// linear position identifies -- never off by one, never bleeding onto a
// neighbour.
func TestDecode_StatusFlagsAlignWithTheirComputedPositionAcrossNonContiguousEntries(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	quarterly := fixtureWithFreq(t, fixtures, "Q")

	posFirst := posForLabel(t, quarterly.body, "2025-Q3")
	posLast := posForLabel(t, quarterly.body, "2026-Q1")
	raw := withInjectedStatus(t, quarterly.body, map[string]string{
		strconv.Itoa(posFirst): "p",
		strconv.Itoa(posLast):  "b",
	})

	result, err := eurostat.Decode(raw, quarterly.ref, indicators.FrequencyQuarterly)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(result.Observations) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(result.Observations))
	}

	first := observationAt(t, result, "2025-Q3")
	if first.Status != indicators.ObservationStatusProvisional {
		t.Errorf("expected 2025-Q3 status Provisional, got %s", first.Status)
	}
	if first.SourceStatus != "p" {
		t.Errorf("expected 2025-Q3 source_status %q, got %q", "p", first.SourceStatus)
	}

	middle := observationAt(t, result, "2025-Q4")
	if middle.Status != indicators.ObservationStatusDefinitive {
		t.Errorf("expected the unflagged 2025-Q4 status Definitive, got %s", middle.Status)
	}
	if middle.SourceStatus != "" {
		t.Errorf("expected the unflagged 2025-Q4 source_status empty, got %q", middle.SourceStatus)
	}

	last := observationAt(t, result, "2026-Q1")
	if last.Status != indicators.ObservationStatusDefinitive {
		t.Errorf("expected the break-flagged 2026-Q1 status to stay Definitive (b is not a status), got %s", last.Status)
	}
	if last.SourceStatus != "b" {
		t.Errorf("expected 2026-Q1 source_status %q, got %q", "b", last.SourceStatus)
	}
}

// TestDecode_AbsentStatusEntryIsDefinitiveWithNullSourceStatus is 2b.3's
// RED proof (spec "Absence of a flag means definitive"): a fixture with
// no "status" map at all -- the real, unmodified recorded shape -- must
// classify every observation definitive with an empty (NULL-bound)
// source_status, never a schema-drift rejection.
func TestDecode_AbsentStatusEntryIsDefinitiveWithNullSourceStatus(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	annual := fixtureWithFreq(t, fixtures, "A")

	result, err := eurostat.Decode(annual.body, annual.ref, indicators.FrequencyAnnual)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for _, o := range result.Observations {
		if o.Status != indicators.ObservationStatusDefinitive {
			t.Errorf("observation %s: expected status Definitive absent any status map, got %s", o.Period, o.Status)
		}
		if o.SourceStatus != "" {
			t.Errorf("observation %s: expected empty source_status absent any status map, got %q", o.Period, o.SourceStatus)
		}
	}
}

// TestDecode_EmptyStatusMapIsValidAndEveryObservationIsDefinitive is
// 2b.3's other RED scenario: a response that DOES carry a "status" key,
// but an empty one, is equally valid -- not a schema-drift rejection.
func TestDecode_EmptyStatusMapIsValidAndEveryObservationIsDefinitive(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	monthly := fixtureWithFreq(t, fixtures, "M")

	raw := withInjectedStatus(t, monthly.body, map[string]string{})

	result, err := eurostat.Decode(raw, monthly.ref, indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(result.Observations) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(result.Observations))
	}
	for _, o := range result.Observations {
		if o.Status != indicators.ObservationStatusDefinitive {
			t.Errorf("observation %s: expected status Definitive under an empty status map, got %s", o.Period, o.Status)
		}
	}
}

// TestDecode_BreakFlagRoutesToBreakSignalsNotStatusEnum is 2b.5's RED
// proof for "b": a break-in-time-series flag becomes a
// SourceResult.BreakSignal (period + flag), and the observation's own
// domain status is NOT derived from it -- it stays whatever absence
// would have made it (definitive).
func TestDecode_BreakFlagRoutesToBreakSignalsNotStatusEnum(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	quarterly := fixtureWithFreq(t, fixtures, "Q")
	pos := posForLabel(t, quarterly.body, "2025-Q4")
	raw := withInjectedStatus(t, quarterly.body, map[string]string{strconv.Itoa(pos): "b"})

	result, err := eurostat.Decode(raw, quarterly.ref, indicators.FrequencyQuarterly)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	obs := observationAt(t, result, "2025-Q4")
	if obs.Status != indicators.ObservationStatusDefinitive {
		t.Errorf("expected the b-flagged observation's status to stay Definitive, got %s", obs.Status)
	}
	if obs.SourceStatus != "b" {
		t.Errorf("expected source_status %q, got %q", "b", obs.SourceStatus)
	}
	if len(result.BreakSignals) != 1 {
		t.Fatalf("expected exactly 1 break signal, got %d: %+v", len(result.BreakSignals), result.BreakSignals)
	}
	if result.BreakSignals[0].Period.String() != "2025-Q4" || result.BreakSignals[0].Flag != "b" {
		t.Errorf("expected break signal {2025-Q4, b}, got %+v", result.BreakSignals[0])
	}
}

// TestDecode_DefinitionFlagRoutesToBreakSignalsNotStatusEnum is 2b.5's
// RED proof for "d" (definition differs): same routing, different flag.
func TestDecode_DefinitionFlagRoutesToBreakSignalsNotStatusEnum(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	annual := fixtureWithFreq(t, fixtures, "A")
	pos := posForLabel(t, annual.body, "2024")
	raw := withInjectedStatus(t, annual.body, map[string]string{strconv.Itoa(pos): "d"})

	result, err := eurostat.Decode(raw, annual.ref, indicators.FrequencyAnnual)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	obs := observationAt(t, result, "2024")
	if obs.Status != indicators.ObservationStatusDefinitive {
		t.Errorf("expected the d-flagged observation's status to stay Definitive, got %s", obs.Status)
	}
	if len(result.BreakSignals) != 1 || result.BreakSignals[0].Flag != "d" || result.BreakSignals[0].Period.String() != "2024" {
		t.Errorf("expected break signal {2024, d}, got %+v", result.BreakSignals)
	}
}

// TestDecode_UnrecognisedStatusFlagFailsClosedNamingDatasetAndFlag is
// 2b.5's fail-closed RED proof: any flag outside {p, b, d} (an absent
// entry already means definitive, handled separately) fails the whole
// run as sourceerr.SchemaDrift, naming both the dataset and the flag --
// nothing is returned.
func TestDecode_UnrecognisedStatusFlagFailsClosedNamingDatasetAndFlag(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	monthly := fixtureWithFreq(t, fixtures, "M")
	pos := posForLabel(t, monthly.body, "2026-06")
	raw := withInjectedStatus(t, monthly.body, map[string]string{strconv.Itoa(pos): "e"})

	result, err := eurostat.Decode(raw, monthly.ref, indicators.FrequencyMonthly)
	if err == nil {
		t.Fatal("expected an unrecognised status flag to fail decoding")
	}
	if len(result.Observations) != 0 {
		t.Errorf("expected nothing returned on a rejected flag, got %+v", result)
	}
	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SchemaDrift {
		t.Errorf("expected class %s, got %s", sourceerr.SchemaDrift, classified.Class)
	}
	if !strings.Contains(err.Error(), monthly.ref) || !strings.Contains(err.Error(), strconv.Quote("e")) {
		t.Errorf("expected the error to name both the dataset %q and the flag %q, got: %v", monthly.ref, "e", err)
	}
}
