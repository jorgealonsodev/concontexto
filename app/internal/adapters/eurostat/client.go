// Package eurostat is the SourceClient (indicators.SourceClient) for
// Eurostat (European Statistical Office). It implements spec
// source-ingestion-eurostat: one call per configured dataset against the
// JSON-stat 2.0 dissemination API, reusing the same domain types,
// observation writer and validation harness as the INE adapter — only
// the transport and decode differ (indicators.SourceClient is the
// interface both adapters implement so app/internal/ingestion.IngestSeries
// never needs to know which one it was handed).
//
// Why a Client is constructed with its dimension filters already baked
// in, rather than taking them per call the way ine.Client takes a COD
// per FetchSeries: Eurostat's request identity is dataset+filters
// together (design.md "/config file schemas", "Eurostat refs add
// dimensions[] + filters{}"), and this batch (slice 6a) wires exactly
// one eurostat.Client per configured series — the same one-client-per-
// series shape config/series/*.yaml's source_refs already describes.
// ref (the dataset code) is still an explicit per-call argument on
// RequestURL/FetchRaw/Decode, matching indicators.SourceClient's shared
// signature and giving the ingestion orchestrator its
// download_attempt/raw_file bookkeeping identity.
//
// Verified live 2026-07-28 (spec source-ingestion-eurostat's own
// endpoint-behaviour note): unlike INE, Eurostat does not refuse an
// oversized query — an unpinned prc_hicp_minr request returns HTTP 200
// and 157,513,570 bytes against a 256 MB container limit. This batch's
// guard against that has two layers: config/series/*.yaml pinning every
// dimension except time (task 6.3) plus the config-validation-time
// enforcement already built in PR 3 (config.validateEurostatPinning) is
// the FIRST layer (catches the mistake before deploy); the runtime
// io.LimitReader response ceiling (task 6.8/6.9, spec "Response size
// ceiling") built in this file is the SECOND layer — a backstop for any
// case config-time pinning cannot see (a dataset growing a new
// dimension between review and ingest, an operator error that somehow
// still passes validate-config). Defence in depth, not a redundant
// duplicate: the two layers catch the failure at two different times.
package eurostat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// defaultMaxAttempts mirrors adapters/ine's retry budget (spec
// source-ingestion-ine's "Retry and backoff MUST NOT loop on this
// condition" applies identically here — sourceerr.FailureClass is the
// shared taxonomy every adapter answers the same way).
const defaultMaxAttempts = 5

// defaultMaxResponseBytes is the ceiling applied when WithMaxResponseBytes
// is not set at construction, matching design.md's "Response ceiling"
// default and config/sources/eurostat.yaml's own configured
// max_response_bytes value (8388608). Production wiring reads
// config.APIConfig.MaxResponseBytes into WithMaxResponseBytes in
// app/cmd/concontexto/ingest_cmd.go's buildSourceClient; this constant
// keeps every adapter test and any direct NewClient caller safe by
// default when no override is supplied.
const defaultMaxResponseBytes int64 = 8 * 1024 * 1024

// defaultHTTPTimeout bounds every request this client issues when the
// caller passes a nil httpClient to NewClient (verify-report WARNING W6):
// see adapters/ine's identical constant for the full rationale -- an
// unbounded http.DefaultClient hung the live probe until killed at 180s.
const defaultHTTPTimeout = 30 * time.Second

// Option configures a Client at construction time.
type Option func(*Client)

// WithMaxAttempts overrides the retry budget for RetryableTransport
// failures (default 5).
func WithMaxAttempts(n int) Option {
	return func(c *Client) { c.maxAttempts = n }
}

// WithBackoff overrides the delay-before-retry function of the attempt
// number (1-indexed, the attempt that just failed). Tests inject a fast
// function so the retry cycle runs instantly instead of waiting on real
// backoff durations.
func WithBackoff(f func(attempt int) time.Duration) Option {
	return func(c *Client) { c.backoff = f }
}

// WithSleep overrides the function the retry loop calls to wait between
// attempts (default time.Sleep).
func WithSleep(f func(time.Duration)) Option {
	return func(c *Client) { c.sleep = f }
}

// WithMaxResponseBytes overrides the response-size ceiling (default
// defaultMaxResponseBytes, 8 MiB). A response body larger than this
// aborts the fetch with a sourceerr.ResponseTooLarge before it is fully
// buffered (task 6.8/6.9, design.md "Response ceiling") — see
// readWithCeiling.
func WithMaxResponseBytes(n int64) Option {
	return func(c *Client) { c.maxResponseBytes = n }
}

// Client is the Eurostat dissemination-API client for one configured
// series: baseURL and filters are fixed at construction (baseURL from
// config/sources/eurostat.yaml's api.base_url, filters from the series'
// own source_refs[].filters), and the dataset code (ref) is supplied per
// call so the same Client type satisfies indicators.SourceClient exactly
// like adapters/ine.Client does.
type Client struct {
	baseURL          string
	filters          map[string]string
	httpClient       *http.Client
	maxAttempts      int
	backoff          func(attempt int) time.Duration
	sleep            func(time.Duration)
	maxResponseBytes int64
}

// NewClient builds a Client against baseURL (e.g.
// "https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data"
// in production, an httptest.Server's URL in tests) with filters already
// pinned for the one series this Client serves. httpClient may be nil,
// in which case http.DefaultClient is used.
func NewClient(baseURL string, filters map[string]string, httpClient *http.Client, opts ...Option) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	c := &Client{
		baseURL:          strings.TrimRight(baseURL, "/"),
		filters:          filters,
		httpClient:       httpClient,
		maxAttempts:      defaultMaxAttempts,
		backoff:          defaultBackoff,
		sleep:            time.Sleep,
		maxResponseBytes: defaultMaxResponseBytes,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func defaultBackoff(attempt int) time.Duration {
	return time.Duration(attempt) * 500 * time.Millisecond
}

// RequestURL returns the exact request URL this client issues for ref
// (the dataset code): {baseURL}/{ref}?format=JSON&lang=EN&{filters...},
// filters sorted by dimension name for a deterministic, reviewable URL
// (spec source-ingestion-eurostat's own request shape).
func (c *Client) RequestURL(ref string) string {
	return c.buildURL(ref, nil)
}

// ProbeURL returns the exact request URL a synthetic daily probe (spec
// §9.4) issues for ref: the same pinned filters RequestURL carries PLUS
// lastTimePeriod, requesting only the n most recent periods instead of
// full history — the Eurostat analogue of INE's nult=1 (design.md
// "Tier 2: build tag live, shape-only (INE nult=1, Eurostat
// lastTimePeriod=1)"). Verified live 2026-07-28 against all three
// milestone-0.3 datasets (prc_hicp_minr/une_rt_q/nama_10_gdp) with
// lastTimePeriod=1 plus every pinned filter: each returned HTTP 200 with
// exactly one period. Task 6.14/6.15; the shared live-tagged probe suite
// that will CALL this method (spec §9.4, tasks 9.3/9.4) is a separate,
// not-yet-built batch — this is only the adapter-level primitive.
func (c *Client) ProbeURL(ref string, lastTimePeriod int) string {
	return c.buildURL(ref, map[string]string{"lastTimePeriod": strconv.Itoa(lastTimePeriod)})
}

// buildURL is RequestURL/ProbeURL's shared query construction: every
// pinned filter, sorted by dimension name for a deterministic,
// reviewable URL, plus whatever extra query parameters the caller adds
// (nil for a full-history request, {"lastTimePeriod": "N"} for a probe).
func (c *Client) buildURL(ref string, extra map[string]string) string {
	q := url.Values{}
	q.Set("format", "JSON")
	q.Set("lang", "EN")
	dims := make([]string, 0, len(c.filters))
	for dim := range c.filters {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	for _, dim := range dims {
		q.Set(dim, c.filters[dim])
	}
	for k, v := range extra {
		q.Set(k, v)
	}
	return fmt.Sprintf("%s/%s?%s", c.baseURL, ref, q.Encode())
}

// fetchWithRetry mirrors adapters/ine.Client.fetchWithRetry: only a
// RetryableTransport failure (5xx, network error) is retried with
// backoff. Any OTHER classified failure — a non-200 non-5xx status
// (SchemaDrift) or doRequest's own ResponseTooLarge classification —
// returns immediately without retrying: the body will be exactly as
// oversized, or the status exactly as wrong, on a second attempt, so
// looping would only waste the retry budget (spec source-ingestion-ine's
// "Backoff does not loop on the restriction envelope", the same
// contract this adapter shares via sourceerr's taxonomy). Eurostat's own
// envelope-level failure classes (zero-observation SilentEmpty) only
// exist once Decode inspects a 200 body, so they never appear here.
func (c *Client) fetchWithRetry(ctx context.Context, requestURL string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		body, status, err := c.doRequest(ctx, requestURL)
		if err != nil {
			var classified *sourceerr.Error
			if errors.As(err, &classified) && !classified.Class.Retryable() {
				return nil, err
			}
			lastErr = sourceerr.New(sourceerr.RetryableTransport, fmt.Sprintf("requesting %s: %v", requestURL, err))
			c.waitBeforeRetry(attempt)
			continue
		}
		if status >= http.StatusInternalServerError {
			lastErr = sourceerr.New(sourceerr.RetryableTransport, fmt.Sprintf("%s returned HTTP %d", requestURL, status))
			c.waitBeforeRetry(attempt)
			continue
		}
		if status != http.StatusOK {
			return nil, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("%s returned unexpected HTTP %d", requestURL, status))
		}
		return body, nil
	}
	return nil, lastErr
}

// FetchRaw fetches ref's raw, undecoded JSON-stat response bytes — no
// periodicity assertion, no normalization — so a caller (the ingestion
// orchestrator) can archive the raw payload BEFORE any parsing happens,
// exactly like adapters/ine.Client.FetchRaw.
func (c *Client) FetchRaw(ctx context.Context, ref string) ([]byte, error) {
	return c.fetchWithRetry(ctx, c.RequestURL(ref))
}

// FetchProbe issues a synthetic daily probe request for ref (spec
// §9.4): the same retry/classification path as FetchRaw, against
// ProbeURL's lastTimePeriod-narrowed request instead of a full-history
// one.
func (c *Client) FetchProbe(ctx context.Context, ref string, lastTimePeriod int) ([]byte, error) {
	return c.fetchWithRetry(ctx, c.ProbeURL(ref, lastTimePeriod))
}

// Decode decodes and normalizes an already-fetched JSON-stat response
// body (see FetchRaw) into indicators.SourceResult, satisfying
// indicators.SourceClient. It delegates to the package-level Decode
// function so a caller with no need for a live Client (this package's
// own tests, task 6.1) can decode a fixture directly.
func (c *Client) Decode(raw []byte, ref string, expectedFrequency indicators.Frequency, segments ...indicators.CadenceSegment) (indicators.SourceResult, error) {
	return Decode(raw, ref, expectedFrequency, segments...)
}

var _ indicators.SourceClient = (*Client)(nil)

func (c *Client) waitBeforeRetry(attempt int) {
	if attempt < c.maxAttempts {
		c.sleep(c.backoff(attempt))
	}
}

func (c *Client) doRequest(ctx context.Context, requestURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := readWithCeiling(resp.Body, c.maxResponseBytes)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// readWithCeiling reads at most ceiling+1 bytes from r via
// io.LimitReader (task 6.8/6.9), so a body larger than ceiling is
// detected WITHOUT ever asking the underlying reader for more than
// ceiling+1 bytes — Eurostat served an unfiltered prc_hicp_minr request
// as HTTP 200 with 157,513,570 bytes against a 256 MB container
// (verified live 2026-07-28, design.md "Response ceiling"), so the check
// must happen DURING the read, not after a full io.ReadAll has already
// spent the very memory budget it was meant to protect. io.ReadAll's own
// amortized buffer growth means actual peak memory can exceed ceiling+1
// by a constant factor while filling that final buffer — "memory stays
// within ceiling+buffer", never within the full oversized body.
func readWithCeiling(r io.Reader, ceiling int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, ceiling+1))
	if err != nil {
		return nil, fmt.Errorf("eurostat: reading response body: %w", err)
	}
	if int64(len(body)) > ceiling {
		return nil, sourceerr.New(sourceerr.ResponseTooLarge, fmt.Sprintf("response exceeded the %d-byte ceiling", ceiling))
	}
	return body, nil
}
