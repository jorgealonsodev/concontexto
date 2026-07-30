package eurostat_test

// A JSON-stat 2.0 response is SPARSE: its "time" dimension declares every
// period the DATASET spans, while its "value" map carries an entry only
// where the source actually published a figure. Every one of the three
// configured Eurostat series exhibits this against the live endpoint
// (verified 2026-07-30, see testdata/source.txt): nama_10_gdp declares
// 1975..2025 but publishes nothing before 1995; une_rt_q declares
// 2003-Q1..2026-Q1 but publishes nothing before 2009-Q1; prc_hicp_minr
// declares 1996-01..2026-06 but publishes nothing before 1997-01.
//
// The defect these tests pin: a position with no value used to become an
// observation anyway, with a nil Value and status Definitive, which the
// schema's own CHECK (value IS NOT NULL OR status = 'W') rejects at write
// time -- so NO Eurostat series could be ingested at all.
//
// Why dropping is the correct projection, and not "record it as W":
// status W is a WITHDRAWAL -- spec data-model-vintages, "Source
// withdrawal is representable": "A source withdrawing a period MUST be
// recorded as a new version with withdrawn status, never as a delete",
// whose own scenario is GIVEN a current observation ... WHEN the source
// STOPS PUBLISHING that period. A period the source never published in
// the first place was never withdrawn; coercing it to W would fabricate a
// retraction that never happened -- the same class of silent lie design
// D-3 exists to remove ("coercing unknown tokens to D is exactly the
// silent lie this change exists to remove"). The honest projection of "no
// value was published here" is no observation, exactly as the spec's own
// zero-observation requirement already assumes ("A zero-observation
// result fails the run": a response with "value": {} is a SilentEmpty
// run, not a run of null-valued observations).
//
// The fixture is a REAL, live-fetched full-history response (the exact
// payload that produced the reported pib-eurostat/1975 constraint
// violation), trimmed only of Eurostat's unparsed "extension" block --
// the same trimming convention testdata/source.txt already documents for
// the three 3-period fixtures. It is kept in its own testdata/sparse/
// subdirectory so it is never picked up by loadEurostatFixtures's
// "exactly 3 real datasets" glob, following testdata/deadcode/'s
// precedent.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// loadSparseFixture returns the single recorded full-history fixture in
// testdata/sparse/, deriving its dataset code from the FILENAME (never a
// Go string literal -- app/internal/guard's origin-identifier deny-list)
// and its frequency from its own "freq" dimension content, exactly as
// loadEurostatFixtures does for the three 3-period fixtures.
func loadSparseFixture(t *testing.T) jsonStatFixture {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join("testdata", "sparse", "*.json"))
	if err != nil {
		t.Fatalf("globbing the sparse fixture: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 sparse full-history fixture, found %v", matches)
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("reading fixture %s: %v", matches[0], err)
	}
	var wire struct {
		Dimension map[string]wireDimForTest `json:"dimension"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("parsing fixture %s to read its freq dimension: %v", matches[0], err)
	}
	var freq string
	for code := range wire.Dimension["freq"].Category.Index {
		freq = code
	}
	if freq == "" {
		t.Fatalf("fixture %s carries no freq dimension category", matches[0])
	}
	// "nama_10_gdp-full-history.json" -> "nama_10_gdp": the dataset code
	// is the filename up to the first descriptive suffix, mirroring
	// testdata/deadcode/'s "{dataset}-{what-makes-it-special}.json" shape.
	base := strings.TrimSuffix(filepath.Base(matches[0]), ".json")
	ref := base
	if i := strings.Index(base, "-full-history"); i > 0 {
		ref = base[:i]
	}
	return jsonStatFixture{ref: ref, body: body, freq: freq}
}

// valuedTimeLabels reads raw's OWN "value" map and "time" dimension
// directly -- never the production decoder -- and returns, in the time
// dimension's declared ordinal order, the labels whose linear position
// carries a value and those whose position does not. This is what makes
// the assertions below independent facts about the recorded payload
// rather than "the decoder agrees with itself".
func valuedTimeLabels(t *testing.T, raw []byte) (valued, valueless []string) {
	t.Helper()
	var wire struct {
		Value     map[string]json.RawMessage `json:"value"`
		Dimension map[string]wireDimForTest  `json:"dimension"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("valuedTimeLabels: parsing fixture: %v", err)
	}
	index := wire.Dimension["time"].Category.Index
	byOrdinal := make([]string, len(index))
	for label, idx := range index {
		if idx < 0 || idx >= len(byOrdinal) {
			t.Fatalf("valuedTimeLabels: time label %q carries out-of-range index %d", label, idx)
		}
		byOrdinal[idx] = label
	}
	for _, label := range byOrdinal {
		if _, ok := wire.Value[strconv.Itoa(posForLabel(t, raw, label))]; ok {
			valued = append(valued, label)
		} else {
			valueless = append(valueless, label)
		}
	}
	return valued, valueless
}

// withValueMap returns raw with its top-level "value" key replaced by
// entries; everything else -- id, size, every dimension, the time
// index -- is the untouched recorded shape.
func withValueMap(t *testing.T, raw []byte, entries map[string]float64) []byte {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("withValueMap: parsing fixture: %v", err)
	}
	valueBytes, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("withValueMap: marshalling replacement value map: %v", err)
	}
	doc["value"] = valueBytes
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("withValueMap: re-marshalling fixture: %v", err)
	}
	return out
}

// TestDecode_APositionWithNoValueYieldsNoObservationRatherThanANullValuedOne
// is the defect's own RED proof, against the real full-history payload
// that produced it: the decoder must emit exactly as many observations as
// the response has "value" entries, every one of them carrying a value.
func TestDecode_APositionWithNoValueYieldsNoObservationRatherThanANullValuedOne(t *testing.T) {
	fixture := loadSparseFixture(t)
	expectedFrequency, ok := freqCodeToFrequency[fixture.freq]
	if !ok {
		t.Fatalf("fixture %s carries unrecognised freq code %q", fixture.ref, fixture.freq)
	}
	valued, valueless := valuedTimeLabels(t, fixture.body)
	if len(valueless) == 0 {
		t.Fatalf("this fixture is not sparse (%d valued positions, 0 valueless) -- it cannot prove the defect", len(valued))
	}

	result, err := eurostat.Decode(fixture.body, fixture.ref, expectedFrequency)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(result.Observations) != len(valued) {
		t.Fatalf("expected exactly %d observations (one per recorded value entry), got %d",
			len(valued), len(result.Observations))
	}
	for _, o := range result.Observations {
		if o.Value == nil {
			t.Errorf("observation %s carries a nil value with status %q -- the schema's CHECK (value IS NOT NULL OR status = 'W') rejects exactly this row", o.Period, o.Status)
		}
	}

	decoded := make(map[string]bool, len(result.Observations))
	for _, o := range result.Observations {
		decoded[o.Period.String()] = true
	}
	for _, label := range valued {
		want := normalizedLabel(t, label)
		if !decoded[want] {
			t.Errorf("period %s carries a value in the payload but was not decoded", want)
		}
	}
	for _, label := range valueless {
		got := normalizedLabel(t, label)
		if decoded[got] {
			t.Errorf("period %s carries NO value in the payload but was decoded anyway", got)
		}
	}
}

// TestDecode_DroppingValuelessPositionsLeavesTheCadenceAuditIntact
// answers the question dropping observations raises: the periodicity /
// cadence audit runs over the WHOLE payload (design D-4), so a payload
// whose earliest declared periods vanish could in principle be classified
// at a different cadence. Against the real recorded history it is not:
// the valueless positions form one contiguous LEADING head, so what
// remains is still a dense, step-1 run at the declared frequency and
// indicators.AssertCadence still passes with no cadence_segments
// declared. This test states that as a checked fact rather than an
// assumption.
func TestDecode_DroppingValuelessPositionsLeavesTheCadenceAuditIntact(t *testing.T) {
	fixture := loadSparseFixture(t)
	expectedFrequency, ok := freqCodeToFrequency[fixture.freq]
	if !ok {
		t.Fatalf("fixture %s carries unrecognised freq code %q", fixture.ref, fixture.freq)
	}
	valued, valueless := valuedTimeLabels(t, fixture.body)

	// The recorded sparseness really is a leading head: no valueless
	// position sits after the first valued one. If Eurostat ever starts
	// publishing interior holes this assertion fails first, naming the
	// reason, instead of the cadence assertion below failing obscurely.
	firstValued := normalizedLabel(t, valued[0])
	for _, label := range valueless {
		if normalizedLabel(t, label) > firstValued {
			t.Fatalf("valueless period %s sits AFTER the first valued period %s -- the recorded sparseness is no longer a leading head, so this test's premise needs revisiting", label, valued[0])
		}
	}

	result, err := eurostat.Decode(fixture.body, fixture.ref, expectedFrequency)
	if err != nil {
		t.Fatalf("Decode: the cadence audit rejected a payload whose valueless leading positions were dropped: %v", err)
	}
	if len(result.Observations) < 3 {
		t.Fatalf("expected a payload long enough for AssertCadence's density check (at least 3 observations), got %d", len(result.Observations))
	}
	for i := 1; i < len(result.Observations); i++ {
		prev, cur := result.Observations[i-1].Period, result.Observations[i].Period
		if !prev.Next().Equal(cur) {
			t.Errorf("observations %s and %s are not one %s step apart -- dropping valueless positions perforated the surviving history", prev, cur, expectedFrequency)
		}
	}
}

// TestDecode_AValuelessPositionCarryingAStatusFlagFailsClosed pins the
// edge case the recorded payloads do NOT exhibit (verified 2026-07-30:
// across all three live full-history responses, zero positions carry a
// "status" entry without a "value" entry).
//
// A flag annotates an observation: the spec requires the decoder to
// "carry the verbatim flag through to the observation's source_status"
// (source-ingestion-eurostat, "JSON-stat status is read at the computed
// position"). With no observation to carry it, that MUST is
// unsatisfiable, and the only two alternatives are to silently discard
// source information or to fail closed. This adapter fails closed, for
// the same reason every other undecided token in it does ("An
// unrecognised flag fails closed"): a flag landing on a position the
// source published no value for is ALSO exactly what a bug in the linear
// position arithmetic would look like, and the spec has its own scenario
// protecting that alignment ("Status is aligned with the value it belongs
// to"). Failing closed turns a silent misalignment into a named
// SchemaDrift; ignoring the flag would hide it.
//
// A "b"/"d" flag on a valueless position therefore does NOT contribute a
// break signal either: the whole run fails, so nothing is emitted.
func TestDecode_AValuelessPositionCarryingAStatusFlagFailsClosed(t *testing.T) {
	fixture := loadSparseFixture(t)
	expectedFrequency, ok := freqCodeToFrequency[fixture.freq]
	if !ok {
		t.Fatalf("fixture %s carries unrecognised freq code %q", fixture.ref, fixture.freq)
	}
	_, valueless := valuedTimeLabels(t, fixture.body)
	if len(valueless) == 0 {
		t.Fatal("this fixture has no valueless position to flag")
	}
	orphan := valueless[0]

	for _, flag := range []string{"p", "b", "d"} {
		flag := flag
		t.Run("flag_"+flag, func(t *testing.T) {
			raw := withInjectedStatus(t, fixture.body, map[string]string{
				strconv.Itoa(posForLabel(t, fixture.body, orphan)): flag,
			})

			result, err := eurostat.Decode(raw, fixture.ref, expectedFrequency)
			if err == nil {
				t.Fatalf("expected a %q flag on the valueless position %s to fail the decode", flag, orphan)
			}
			if len(result.Observations) != 0 || len(result.BreakSignals) != 0 {
				t.Errorf("expected nothing returned on a rejected payload, got %+v", result)
			}
			var classified *sourceerr.Error
			if !errors.As(err, &classified) {
				t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
			}
			if classified.Class != sourceerr.SchemaDrift {
				t.Errorf("expected class %s, got %s", sourceerr.SchemaDrift, classified.Class)
			}
			for _, want := range []string{fixture.ref, normalizedLabel(t, orphan), strconv.Quote(flag)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected the error to name %q, got: %v", want, err)
				}
			}
		})
	}
}

// TestDecode_AnUnrecognisedFlagOnAValuelessPositionStillNamesTheFlag
// pins the precedence between the two fail-closed branches: the flag
// VOCABULARY is judged first, so a response whose undocumented flag also
// happens to sit on a valueless position keeps the diagnostic the spec's
// own scenario asks for ("the run fails with sourceerr.SchemaDrift naming
// the dataset and the unrecognised flag"), rather than being masked by
// the newer alignment check.
func TestDecode_AnUnrecognisedFlagOnAValuelessPositionStillNamesTheFlag(t *testing.T) {
	fixture := loadSparseFixture(t)
	expectedFrequency := freqCodeToFrequency[fixture.freq]
	_, valueless := valuedTimeLabels(t, fixture.body)
	orphan := valueless[0]

	raw := withInjectedStatus(t, fixture.body, map[string]string{
		strconv.Itoa(posForLabel(t, fixture.body, orphan)): "e",
	})

	_, err := eurostat.Decode(raw, fixture.ref, expectedFrequency)
	if err == nil {
		t.Fatal("expected an unrecognised flag to fail decoding")
	}
	if !strings.Contains(err.Error(), "unrecognised status flag") || !strings.Contains(err.Error(), strconv.Quote("e")) {
		t.Errorf("expected the unrecognised-flag diagnostic naming the flag, got: %v", err)
	}
}

// TestDecode_AResponseWhoseEveryPositionIsValuelessIsASilentEmptyResult
// closes the loop with the spec's own zero-observation requirement ("A
// zero-observation result fails the run": a dead dimension code returns
// HTTP 200 with valid JSON-stat and "value": {}). Once valueless
// positions stop becoming observations, a "value": {} response over a
// fully populated time dimension yields zero observations -- and that
// must be the NAMED SilentEmpty failure class, distinct from a transport
// error, exactly as ine/envelope.go classifies its own zero-observation
// body.
func TestDecode_AResponseWhoseEveryPositionIsValuelessIsASilentEmptyResult(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	quarterly := fixtureWithFreq(t, fixtures, "Q")

	raw := withValueMap(t, quarterly.body, map[string]float64{})

	result, err := eurostat.Decode(raw, quarterly.ref, indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected a response carrying no values at all to fail the run")
	}
	if len(result.Observations) != 0 {
		t.Errorf("expected nothing returned, got %+v", result)
	}
	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SilentEmpty {
		t.Errorf("expected class %s, got %s", sourceerr.SilentEmpty, classified.Class)
	}
	if !strings.Contains(err.Error(), quarterly.ref) {
		t.Errorf("expected the error to name the dataset %q, got: %v", quarterly.ref, err)
	}
}

// normalizedLabel renders a raw Eurostat time label in the canonical
// period shape the decoder reports, so expectations are written against
// the domain's own normalisation rather than a second, hand-maintained
// copy of it.
func normalizedLabel(t *testing.T, label string) string {
	t.Helper()
	p, err := indicators.NormalizePeriodLabel(label)
	if err != nil {
		t.Fatalf("normalising time label %q: %v", label, err)
	}
	return p.String()
}
