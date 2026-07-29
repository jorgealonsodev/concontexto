package eurostat_test

// Task 6.1 (RED): a recorded JSON-stat 2.0 fixture decodes into
// canonical observations with normalised periods (spec
// source-ingestion-eurostat, "A JSON-stat response becomes canonical
// observations" / "reuse the same domain types ... as the INE
// adapter"). This test derives every dataset code from the checked-in
// fixture FILENAME (never a Go string literal), and derives which
// fixture is which by its own JSON-stat "freq" dimension content ("M"/
// "Q"/"A" — the fixture-file-order-coupling bug PR 5b hit and fixed for
// INE's six fixtures is avoided here by never depending on glob order at
// all), exactly matching app/internal/guard's origin-identifier
// deny-list, which already forbids "prc_hicp_minr", "une_rt_q" and
// "nama_10_gdp" (the three real dataset codes) as Go literals anywhere
// outside /config and testdata/.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// freqCodeToFrequency translates JSON-stat's own single-letter freq
// dimension code into the canonical indicators.Frequency -- read from
// each fixture's own content, never hard-coded per dataset.
var freqCodeToFrequency = map[string]indicators.Frequency{
	"M": indicators.FrequencyMonthly,
	"Q": indicators.FrequencyQuarterly,
	"A": indicators.FrequencyAnnual,
}

// jsonStatFixture is the minimal shape this test reads directly off a
// fixture file, independently of the production decoder, so the test's
// own expectations are not just "my decoder agrees with itself".
type jsonStatFixture struct {
	ref  string // derived from the filename, never a Go literal
	body []byte
	freq string // "M" | "Q" | "A", read from the fixture's own dimension.freq.category.index
}

func loadEurostatFixtures(t *testing.T) []jsonStatFixture {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		t.Fatalf("globbing eurostat fixtures: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected exactly 3 eurostat fixtures, found %v", matches)
	}
	var out []jsonStatFixture
	for _, m := range matches {
		body, err := os.ReadFile(m)
		if err != nil {
			t.Fatalf("reading fixture %s: %v", m, err)
		}
		var wire struct {
			Dimension map[string]struct {
				Category struct {
					Index map[string]int `json:"index"`
				} `json:"category"`
			} `json:"dimension"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Fatalf("parsing fixture %s to read its freq dimension: %v", m, err)
		}
		var freq string
		for code := range wire.Dimension["freq"].Category.Index {
			freq = code
		}
		if freq == "" {
			t.Fatalf("fixture %s carries no freq dimension category", m)
		}
		out = append(out, jsonStatFixture{
			ref:  strings.TrimSuffix(filepath.Base(m), ".json"),
			body: body,
			freq: freq,
		})
	}
	return out
}

// fixtureWithFreq returns the one loaded fixture whose freq code matches
// want, identifying "the monthly one"/"the quarterly one"/"the annual
// one" by content rather than by dataset name or glob order.
func fixtureWithFreq(t *testing.T, fixtures []jsonStatFixture, want string) jsonStatFixture {
	t.Helper()
	for _, f := range fixtures {
		if f.freq == want {
			return f
		}
	}
	t.Fatalf("no fixture found with freq=%q among %d fixtures", want, len(fixtures))
	return jsonStatFixture{}
}

func TestDecode_EveryFixtureYieldsThreeChronologicalObservations(t *testing.T) {
	fixtures := loadEurostatFixtures(t)

	for _, f := range fixtures {
		f := f
		t.Run(f.ref, func(t *testing.T) {
			expected, ok := freqCodeToFrequency[f.freq]
			if !ok {
				t.Fatalf("fixture %s carries unrecognised freq code %q", f.ref, f.freq)
			}

			result, err := eurostat.Decode(f.body, f.ref, expected)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if result.Name == "" {
				t.Error("expected a non-empty dataset label")
			}
			if len(result.Observations) != 3 {
				t.Fatalf("expected 3 observations, got %d: %+v", len(result.Observations), result.Observations)
			}
			for i := 0; i < len(result.Observations)-1; i++ {
				if !result.Observations[i].Period.Before(result.Observations[i+1].Period) {
					t.Errorf("observation %d (%s) is not strictly before observation %d (%s)",
						i, result.Observations[i].Period, i+1, result.Observations[i+1].Period)
				}
				if result.Observations[i].Value == nil {
					t.Errorf("observation %d: expected a non-nil value", i)
				}
			}
			if result.Observations[2].Value == nil {
				t.Error("expected the last observation's value to be non-nil")
			}
		})
	}
}

// TestDecode_RealVerifiedValues anchors the decoder to the exact values
// verified live 2026-07-28 (see testdata/source.txt), identifying each
// fixture by its own freq content rather than by dataset name.
func TestDecode_RealVerifiedValues(t *testing.T) {
	fixtures := loadEurostatFixtures(t)

	monthly := fixtureWithFreq(t, fixtures, "M")
	result, err := eurostat.Decode(monthly.body, monthly.ref, indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("Decode(monthly): %v", err)
	}
	last := result.Observations[len(result.Observations)-1]
	if last.Period.String() != "2026-06" || last.Value == nil || *last.Value != 3.6 {
		t.Errorf("expected the monthly fixture's last observation to be 2026-06=3.6, got %s=%v", last.Period, last.Value)
	}

	quarterly := fixtureWithFreq(t, fixtures, "Q")
	result, err = eurostat.Decode(quarterly.body, quarterly.ref, indicators.FrequencyQuarterly)
	if err != nil {
		t.Fatalf("Decode(quarterly): %v", err)
	}
	last = result.Observations[len(result.Observations)-1]
	if last.Period.String() != "2026-Q1" || last.Value == nil || *last.Value != 10.3 {
		t.Errorf("expected the quarterly fixture's last observation to be 2026-Q1=10.3, got %s=%v", last.Period, last.Value)
	}

	annual := fixtureWithFreq(t, fixtures, "A")
	result, err = eurostat.Decode(annual.body, annual.ref, indicators.FrequencyAnnual)
	if err != nil {
		t.Fatalf("Decode(annual): %v", err)
	}
	last = result.Observations[len(result.Observations)-1]
	if last.Period.String() != "2025" || last.Value == nil || *last.Value != 1318439.0 {
		t.Errorf("expected the annual fixture's last observation to be 2025=1318439.0, got %s=%v", last.Period, last.Value)
	}
}

// TestDecode_DeadDimensionCodeCP00FailsAsSilentEmptyNotAsATransportError
// is task 6.10/6.11's decode-level proof: a real, live-verified HTTP 200
// JSON-stat response whose coicop18 dimension carries ZERO categories
// (the ECOICOP v1 all-items code "CP00", dead since ver.2 — spec
// source-ingestion-eurostat, "In ECOICOP ver.2 the all-items code is
// TOTAL; CP00 is the dead v1 code") must classify as the DISTINCT
// zero-observation class (sourceerr.SilentEmpty), not an ordinary
// decode error and not a transport failure — rule 6 (slice 4)'s own
// non-emptiness backstop exists at the validation layer for exactly the
// same reason this classification exists at the adapter layer.
func TestDecode_DeadDimensionCodeCP00FailsAsSilentEmptyNotAsATransportError(t *testing.T) {
	fixturePath := filepath.Join("testdata", "deadcode", "prc_hicp_minr-coicop18-cp00.json")
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading the dead-code fixture: %v", err)
	}

	// The empty dimension's own name is read from the fixture itself,
	// never hard-coded as a Go literal (app/internal/guard's
	// origin-identifier deny-list already forbids "coicop18" as a Go
	// literal outside /config and testdata/) -- this also makes the
	// assertion below correct for whichever dimension the fixture
	// actually carries as empty, not an assumption about this one file.
	emptyDim := emptyCategoryDimension(t, body)
	// ref is likewise never a Go literal for the same reason; it is only
	// used for this decode call's diagnostic message, never compared
	// against anything, so a non-identifier placeholder is exact and
	// correct.
	ref := strings.TrimSuffix(filepath.Base(fixturePath), ".json")

	_, decodeErr := eurostat.Decode(body, ref, indicators.FrequencyMonthly)
	if decodeErr == nil {
		t.Fatal("expected the dead-dimension-code response to fail decoding")
	}

	var classified *sourceerr.Error
	if !errors.As(decodeErr, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", decodeErr, decodeErr)
	}
	if classified.Class != sourceerr.SilentEmpty {
		t.Errorf("expected sourceerr.SilentEmpty (the distinct zero-observation class), got %s", classified.Class)
	}
	if !strings.Contains(decodeErr.Error(), emptyDim) {
		t.Errorf("expected the error to name the empty %q dimension, got %v", emptyDim, decodeErr)
	}
}

// emptyCategoryDimension reads raw's own dimension map and returns the
// name of the one non-time dimension whose category index is empty --
// the fixture's own defining shape (task 6.10/6.11's dead-code case),
// read from data rather than assumed as a Go literal.
func emptyCategoryDimension(t *testing.T, raw []byte) string {
	t.Helper()
	var wire struct {
		Dimension map[string]struct {
			Category struct {
				Index map[string]int `json:"index"`
			} `json:"category"`
		} `json:"dimension"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("parsing fixture to find its empty dimension: %v", err)
	}
	for name, dim := range wire.Dimension {
		// "time" is never on the origin-identifier deny-list (it is a
		// structural JSON-stat dimension name common to every dataset,
		// not an origin-specific identifier -- see
		// originidentifiers_test.go's own documented exclusion), so it
		// is safe to compare directly.
		if name == "time" {
			continue
		}
		if len(dim.Category.Index) == 0 {
			return name
		}
	}
	t.Fatal("expected the fixture to carry exactly one empty non-time dimension")
	return ""
}

// TestDecode_PeriodicityMismatchFailsNamingBoth proves rule parity with
// INE (spec source-ingestion-eurostat, "reuse the same ... validation
// harness"): asserting the WRONG expectedFrequency against a real
// fixture fails with a named error, not a silent misclassification.
func TestDecode_PeriodicityMismatchFailsNamingBoth(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	monthly := fixtureWithFreq(t, fixtures, "M")

	_, err := eurostat.Decode(monthly.body, monthly.ref, indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected a periodicity mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "Q") || !strings.Contains(err.Error(), "M") {
		t.Errorf("expected the error to name both the expected (Q) and actual (M) periodicity, got %v", err)
	}
}

// TestFetchRaw_IssuesOneRequestCarryingEveryPinnedFilter proves the
// client-level fetch path (RequestURL/FetchRaw), not just decode: the
// exact same round trip IngestSeries drives.
func TestFetchRaw_IssuesOneRequestCarryingEveryPinnedFilter(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	f := fixtureWithFreq(t, fixtures, "M")

	filters := map[string]string{"freq": "M", "unit": "RCH_A", "geo": "ES"}
	var requestCount int
	var gotQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(f.body)
	}))
	defer server.Close()

	client := eurostat.NewClient(server.URL, filters, server.Client())
	raw, err := client.FetchRaw(context.Background(), f.ref)
	if err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
	if string(raw) != string(f.body) {
		t.Error("expected FetchRaw to return the exact undecoded response bytes")
	}
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request, got %d", requestCount)
	}
	for dim, code := range filters {
		if got := gotQuery.Get(dim); got != code {
			t.Errorf("expected filter %s=%s on the request, got %q", dim, code, got)
		}
	}
	if gotQuery.Get("format") != "JSON" || gotQuery.Get("lang") != "EN" {
		t.Errorf("expected format=JSON&lang=EN on every request, got format=%q lang=%q", gotQuery.Get("format"), gotQuery.Get("lang"))
	}
}

// TestFetchRaw_ResponseOverCeilingFailsWithoutRetryingAndNamesTheCeiling
// is task 6.8/6.9's end-to-end proof at the client level: a response
// larger than the configured ceiling fails with a named
// sourceerr.ResponseTooLarge error, and — because that classification
// is permanent (retrying will not shrink the body) — exactly ONE
// request is issued, never a retry loop (mirrors adapters/ine's own
// "Backoff does not loop on the restriction envelope" contract for
// SourceRefusal). The body here is only a few hundred bytes: proving
// the WIRING (client -> doRequest -> readWithCeiling -> classification
// -> no-retry) needs no real oversized payload — readWithCeiling's own
// no-full-buffering property is proven separately and more rigorously
// in ceiling_test.go's white-box tests.
func TestFetchRaw_ResponseOverCeilingFailsWithoutRetryingAndNamesTheCeiling(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes.Repeat([]byte("x"), 500))
	}))
	defer server.Close()

	client := eurostat.NewClient(server.URL, map[string]string{"geo": "ES"}, server.Client(),
		eurostat.WithMaxResponseBytes(100),
		eurostat.WithSleep(func(time.Duration) {}))
	_, err := client.FetchRaw(context.Background(), "any-ref")
	if err == nil {
		t.Fatal("expected a response-too-large error")
	}
	if !strings.Contains(err.Error(), "100") {
		t.Errorf("expected the error to name the configured 100-byte ceiling, got %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request (a permanent classification must not be retried), got %d", requestCount)
	}
}

// TestFetchRaw_ResponseUnderCeilingSucceeds triangulates the ceiling
// check's other branch: a real fixture body, comfortably under a
// realistic ceiling, must decode successfully — proving
// WithMaxResponseBytes's default (8 MiB) and any explicit override
// never reject a legitimately-sized, fully-pinned response.
func TestFetchRaw_ResponseUnderCeilingSucceeds(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	f := fixtureWithFreq(t, fixtures, "A")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(f.body)
	}))
	defer server.Close()

	client := eurostat.NewClient(server.URL, map[string]string{"geo": "ES"}, server.Client(),
		eurostat.WithMaxResponseBytes(int64(len(f.body))+1))
	raw, err := client.FetchRaw(context.Background(), f.ref)
	if err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
	if len(raw) != len(f.body) {
		t.Errorf("expected the full %d-byte fixture body, got %d bytes", len(f.body), len(raw))
	}
}

// TestFetchProbe_IssuesOneRequestCarryingLastTimePeriodPlusEveryPinnedFilter
// is task 6.14/6.15's proof: a probe request (spec §9.4's synthetic
// daily probe, the Eurostat analogue of INE's nult=1) carries
// lastTimePeriod=1 PLUS every pinned filter — never lastTimePeriod
// alone, which would still return the full dimension cartesian product
// for whichever periods it did include.
func TestFetchProbe_IssuesOneRequestCarryingLastTimePeriodPlusEveryPinnedFilter(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	f := fixtureWithFreq(t, fixtures, "Q")

	filters := map[string]string{"freq": "Q", "s_adj": "SA", "age": "Y15-74", "unit": "PC_ACT", "sex": "T", "geo": "ES"}
	var requestCount int
	var gotQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(f.body)
	}))
	defer server.Close()

	client := eurostat.NewClient(server.URL, filters, server.Client())
	raw, err := client.FetchProbe(context.Background(), f.ref, 1)
	if err != nil {
		t.Fatalf("FetchProbe: %v", err)
	}
	if string(raw) != string(f.body) {
		t.Error("expected FetchProbe to return the exact undecoded response bytes")
	}
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request, got %d", requestCount)
	}
	if got := gotQuery.Get("lastTimePeriod"); got != "1" {
		t.Errorf("expected lastTimePeriod=1, got %q", got)
	}
	for dim, code := range filters {
		if got := gotQuery.Get(dim); got != code {
			t.Errorf("expected pinned filter %s=%s to survive on the probe request, got %q", dim, code, got)
		}
	}
	if gotQuery.Get("format") != "JSON" || gotQuery.Get("lang") != "EN" {
		t.Errorf("expected format=JSON&lang=EN on the probe request too, got format=%q lang=%q", gotQuery.Get("format"), gotQuery.Get("lang"))
	}
}

// TestProbeURL_NamesLastTimePeriodInTheURLItself triangulates
// FetchProbe's own test with the string-level shape ProbeURL builds,
// independent of a real round trip -- proving the parameter name is
// exactly "lastTimePeriod" (Eurostat's own documented query parameter),
// not a lookalike.
func TestProbeURL_NamesLastTimePeriodInTheURLItself(t *testing.T) {
	client := eurostat.NewClient("http://example.invalid", map[string]string{"geo": "ES"}, nil)
	got := client.ProbeURL("any-ref", 1)
	if !strings.Contains(got, "lastTimePeriod=1") {
		t.Errorf("expected ProbeURL to carry lastTimePeriod=1, got %q", got)
	}
	if !strings.Contains(got, "geo=ES") {
		t.Errorf("expected ProbeURL to still carry the pinned geo filter, got %q", got)
	}
}

// TestFetchRaw_TransportErrorRetriesWithBackoffAndSucceeds proves the
// client shares INE's RetryableTransport resilience (sourceerr's shared
// taxonomy, spec source-ingestion-eurostat "reuse the same ... validation
// harness"): a 503-then-200 sequence eventually succeeds without the
// caller ever seeing the transient failure.
func TestFetchRaw_TransportErrorRetriesWithBackoffAndSucceeds(t *testing.T) {
	fixtures := loadEurostatFixtures(t)
	f := fixtureWithFreq(t, fixtures, "Q")

	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(f.body)
	}))
	defer server.Close()

	client := eurostat.NewClient(server.URL, map[string]string{"geo": "ES"}, server.Client(),
		eurostat.WithSleep(func(time.Duration) {}))
	raw, err := client.FetchRaw(context.Background(), f.ref)
	if err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
	if string(raw) != string(f.body) {
		t.Error("expected the eventual successful response body")
	}
	if attempts != 2 {
		t.Errorf("expected exactly 2 attempts (1 failure + 1 success), got %d", attempts)
	}
}
