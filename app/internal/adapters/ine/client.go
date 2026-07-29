// Package ine is the SourceClient for INE (Instituto Nacional de
// Estadística). It implements ADR-2/D5: ingestion goes through
// DATOS_SERIE/{COD}, one request per canonical series, and never
// through DATOS_TABLA or SERIES_TABLA -- those two remain discovery-only
// tools a human uses while pinning a config/series/*.yaml entry, never
// something this client calls at runtime.
//
// Why DATOS_SERIE and not DATOS_TABLA (verified live 2026-07-28, Engram
// #4690, design.md ADR-2): the series COD is the STABLE identifier; the
// table Id is only a container around it -- ECP320 resolves to the
// identical national-total population series in BOTH table 56934
// (annual) and table 59238 (quarterly). Fetching by table Id also risks
// INE's undocumented volume-restriction failure mode on wide tables
// (HTTP 200 with a JSON object body where success is an array --
// classified as sourceerr.SourceRefusal, see envelope.go); DATOS_SERIE
// was verified NOT subject to that restriction for every series checked.
// One request per series also keeps the blast radius of any single
// failure to one series, and keeps fixtures small enough to review
// (DATOS_SERIE responses are naturally small, unlike a wide DATOS_TABLA
// dump).
package ine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// defaultMaxAttempts bounds the retry loop so a permanent, non-retryable
// failure -- or an exhausted retry budget on a persistent 5xx -- cannot
// spin forever (spec source-ingestion-ine, "Retry and backoff MUST NOT
// loop on this condition").
const defaultMaxAttempts = 5

// defaultHTTPTimeout bounds every request this client issues when the
// caller passes a nil httpClient to NewClient (verify-report WARNING W6):
// http.DefaultClient carries Timeout: 0 (unbounded) -- a live probe
// attempt against this exact fallback hung until killed at 180s rather
// than failing, defeating §9.2's retry/backoff design (a call that never
// returns can never be backed off from). 30s is generous for INE's own
// small DATOS_SERIE responses while still bounding a genuinely
// unresponsive source to a single, failing attempt per retry.
const defaultHTTPTimeout = 30 * time.Second

// defaultNult bounds how many periods DATOS_SERIE returns. Verified live
// 2026-07-28: a bare "?tip=A" query with no "nult" 404s against the real
// INE endpoint (DATOS_SERIE/EPA453100?tip=A -> HTTP 404) -- INE requires
// an explicit nult, it does not default to "everything" the way this
// adapter's very first version assumed. "Full history" (spec
// source-ingestion-ine, "each MUST load its full history") therefore
// means a generous EXPLICIT cap, not an omitted parameter: the longest of
// the six milestone-0.2 series (IPC, monthly, 2002-2026) holds 294
// periods, so 9999 leaves comfortable headroom for decades of future
// growth.
const defaultNult = 9999

// defaultMaxResponseBytes mirrors adapters/eurostat and adapters/xlsx's
// own default (design.md "Response ceiling", 8 MiB) and
// config/sources/ine.yaml's own configured max_response_bytes value
// (8388608). INE's documented failure mode is the opposite of
// Eurostat's -- it REFUSES an oversized query with the volume-restriction
// envelope rather than serving it (see this package's doc comment) -- so
// this ceiling is defence-in-depth for a misconfigured request, a
// source-side error page, or a proxy interposing, not a response to any
// observed INE behaviour. Every adapter satisfying indicators.SourceClient
// carries the same ceiling regardless of how low-probability its own
// oversized-response case is.
const defaultMaxResponseBytes int64 = 8 * 1024 * 1024

// Observation is one normalized INE data point: a canonical
// indicators.Period (never a source-specific label -- see period.go) and
// its value.
type Observation struct {
	Period indicators.Period
	Value  *float64
}

// Result is FetchSeries's return value: the series' live descriptive
// name plus its normalized, periodicity-checked observations.
type Result struct {
	COD          string
	Name         string
	Observations []Observation
}

// Option configures a Client at construction time.
type Option func(*Client)

// WithMaxAttempts overrides the retry budget for RetryableTransport
// failures (default 5). Every other sourceerr.FailureClass never
// consults this budget at all -- it returns after exactly one attempt
// regardless of maxAttempts (spec "Backoff does not loop on the
// restriction envelope").
func WithMaxAttempts(n int) Option {
	return func(c *Client) { c.maxAttempts = n }
}

// WithBackoff overrides the delay-before-retry function of the attempt
// number (1-indexed, the attempt that just failed). Tests inject a fast
// function to prove the retry cycle consults backoff without waiting on
// real durations.
func WithBackoff(f func(attempt int) time.Duration) Option {
	return func(c *Client) { c.backoff = f }
}

// WithSleep overrides the function the retry loop calls to wait between
// attempts (default time.Sleep). Tests inject a no-op so the retry cycle
// runs instantly instead of waiting on real backoff durations.
func WithSleep(f func(time.Duration)) Option {
	return func(c *Client) { c.sleep = f }
}

// WithMaxResponseBytes overrides the response-size ceiling (default
// defaultMaxResponseBytes, 8 MiB). A response body larger than this
// aborts the fetch with a sourceerr.ResponseTooLarge before it is fully
// buffered -- see readWithCeiling. Mirrors
// adapters/eurostat.WithMaxResponseBytes and adapters/xlsx.WithMaxResponseBytes.
func WithMaxResponseBytes(n int64) Option {
	return func(c *Client) { c.maxResponseBytes = n }
}

// Client is the INE DATOS_SERIE client. baseURL is always an explicit
// constructor parameter, never read from the environment inside this
// adapter -- production wiring resolves it from config/sources/ine.yaml
// (config.SourceConfig.API.BaseURL), tests point it at an
// httptest.Server, matching the pattern PR 5a-i's filestore.NewStore
// already established.
type Client struct {
	baseURL          string
	httpClient       *http.Client
	maxAttempts      int
	backoff          func(attempt int) time.Duration
	sleep            func(time.Duration)
	maxResponseBytes int64
}

// NewClient builds a Client against baseURL (e.g.
// "https://servicios.ine.es/wstempus/js/ES" in production, an
// httptest.Server's URL in tests). httpClient may be nil, in which case
// http.DefaultClient is used.
func NewClient(baseURL string, httpClient *http.Client, opts ...Option) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	c := &Client{
		baseURL:          strings.TrimRight(baseURL, "/"),
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
	// Tuning the real production backoff curve is §9.2's job (the
	// scheduler, task 9.1/9.2); this adapter only needs A backoff so the
	// "503-then-success retries and eventually succeeds" contract holds.
	return time.Duration(attempt) * 500 * time.Millisecond
}

// SeriesURL returns the exact DATOS_SERIE/{COD} request URL this client
// issues for cod -- the one place that URL is built, so FetchRaw,
// FetchSeries and any caller that needs it for its own bookkeeping (the
// ingestion orchestrator's download_attempt/raw_file records, task 5b.6)
// never risk constructing a second, possibly-diverging copy.
func (c *Client) SeriesURL(cod string) string {
	return fmt.Sprintf("%s/DATOS_SERIE/%s?nult=%d&tip=A", c.baseURL, cod, defaultNult)
}

// ProbeURL returns the exact request URL a synthetic probe issues for cod
// (spec pipeline-operations, "Synthetic daily probe against every
// endpoint": "INE nult=1") -- the smallest possible DATOS_SERIE request,
// deliberately built through the same query shape SeriesURL uses (nult
// explicit, tip=A) rather than a second, divergent URL-construction path.
// Task 9.3/9.4; the shared live-tagged probe suite that calls this
// (app/internal/probe, not built here) is a separate, not-yet-built
// batch -- this is only the adapter-level primitive, mirroring
// adapters/eurostat.Client.ProbeURL exactly.
func (c *Client) ProbeURL(cod string, nult int) string {
	return fmt.Sprintf("%s/DATOS_SERIE/%s?nult=%d&tip=A", c.baseURL, cod, nult)
}

// FetchProbe issues a synthetic daily probe request for cod: the same
// retry/classification path as FetchRaw (fetchWithRetry), against
// ProbeURL's nult-narrowed request instead of SeriesURL's full-history
// one -- a non-retryable classified failure (sourceerr.SourceRefusal,
// the volume-restriction envelope) still returns after exactly one
// request, because fetchWithRetry itself decides that, not this method
// (this package's own doc comment: "the clients already classify
// correctly").
func (c *Client) FetchProbe(ctx context.Context, cod string, nult int) ([]byte, error) {
	return c.fetchWithRetry(ctx, c.ProbeURL(cod, nult))
}

// fetchWithRetry issues one GET against url, retrying a RetryableTransport
// failure (5xx, network error) with backoff up to maxAttempts. Every other
// failure -- a non-5xx, non-200 HTTP status, or doRequest's own
// ResponseTooLarge classification -- returns immediately without
// retrying (spec source-ingestion-ine, "Backoff does not loop on the
// restriction envelope"): a body over the configured ceiling will not
// become smaller on a second attempt, mirroring
// adapters/eurostat.Client.fetchWithRetry's identical errors.As check.
// Envelope-level classification (SourceRefusal, SilentEmpty, a
// periodicity SchemaDrift) only exists once decodeAndNormalize inspects
// a 200 body, which is why this transport-only function never produces
// those three classes itself.
func (c *Client) fetchWithRetry(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		body, status, err := c.doRequest(ctx, url)
		if err != nil {
			var classified *sourceerr.Error
			if errors.As(err, &classified) && !classified.Class.Retryable() {
				return nil, err
			}
			lastErr = sourceerr.New(sourceerr.RetryableTransport, fmt.Sprintf("requesting %s: %v", url, err))
			c.waitBeforeRetry(attempt)
			continue
		}
		if status >= http.StatusInternalServerError {
			lastErr = sourceerr.New(sourceerr.RetryableTransport, fmt.Sprintf("%s returned HTTP %d", url, status))
			c.waitBeforeRetry(attempt)
			continue
		}
		if status != http.StatusOK {
			return nil, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("%s returned unexpected HTTP %d", url, status))
		}
		return body, nil
	}
	return nil, lastErr
}

// FetchRaw fetches a canonical series' full history through
// DATOS_SERIE/{COD} (never DATOS_TABLA -- see the package doc comment)
// and returns the exact, undecoded response bytes -- no periodicity
// assertion, no normalization. It exists so a caller (the ingestion
// orchestrator, task 5b.6) can archive the raw payload BEFORE any parsing
// happens (spec raw-file-archive's ordering guarantee: the archive
// exists independently of whatever DecodeSeries later does with it), then
// call DecodeSeries separately on the exact same bytes.
func (c *Client) FetchRaw(ctx context.Context, cod string) ([]byte, error) {
	return c.fetchWithRetry(ctx, c.SeriesURL(cod))
}

// DecodeSeries decodes and normalizes an already-fetched DATOS_SERIE
// response body (see FetchRaw) -- no HTTP, no I/O, the exact same
// decode/periodicity/normalize logic FetchSeries runs in one call, split
// out so a caller can archive raw bytes between fetch and decode.
func DecodeSeries(body []byte, cod string, expectedFrequency indicators.Frequency) (Result, error) {
	return decodeAndNormalize(body, cod, expectedFrequency)
}

// RequestURL satisfies indicators.SourceClient: the exact DATOS_SERIE
// request URL for ref (an INE COD). It is a thin alias for SeriesURL,
// added so *Client structurally implements the same port
// adapters/eurostat.Client does (design.md package layout, "ports.go:
// ... SourceClient" -- built in slice 6 because it is the first caller
// that needs IngestSeries to work with more than one adapter).
func (c *Client) RequestURL(ref string) string {
	return c.SeriesURL(ref)
}

// Decode satisfies indicators.SourceClient: it wraps the package-level
// DecodeSeries (unchanged, still used directly by this package's own
// tests) and converts ine.Observation into the shared
// indicators.Observation shape -- the two are field-identical, so this
// is a pure re-labelling, never a lossy conversion.
func (c *Client) Decode(raw []byte, ref string, expectedFrequency indicators.Frequency) (indicators.SourceResult, error) {
	result, err := DecodeSeries(raw, ref, expectedFrequency)
	if err != nil {
		return indicators.SourceResult{}, err
	}
	observations := make([]indicators.Observation, 0, len(result.Observations))
	for _, o := range result.Observations {
		observations = append(observations, indicators.Observation{Period: o.Period, Value: o.Value})
	}
	return indicators.SourceResult{Name: result.Name, Observations: observations}, nil
}

var _ indicators.SourceClient = (*Client)(nil)

// FetchSeries fetches a canonical series by its COD through
// DATOS_SERIE/{COD} (never DATOS_TABLA -- see the package doc comment),
// asserts its returned periodicity matches expectedFrequency, and
// normalizes every observation's period into the canonical
// indicators.Period. It is FetchRaw + DecodeSeries composed into one
// call, for callers that have no need to see the raw bytes separately.
//
// A RetryableTransport failure (5xx, network error) is retried with
// backoff up to maxAttempts. Every other classified failure --
// SourceRefusal (the volume-restriction envelope), SilentEmpty (zero
// observations), SchemaDrift (periodicity mismatch or a decode failure)
// -- returns immediately without retrying: exactly one request is
// issued for those classes, by construction, because fetchWithRetry
// already returns before decode ever runs (spec source-ingestion-ine,
// "Backoff does not loop on the restriction envelope").
func (c *Client) FetchSeries(ctx context.Context, cod string, expectedFrequency indicators.Frequency) (Result, error) {
	body, err := c.fetchWithRetry(ctx, c.SeriesURL(cod))
	if err != nil {
		return Result{}, err
	}
	return decodeAndNormalize(body, cod, expectedFrequency)
}

func (c *Client) waitBeforeRetry(attempt int) {
	if attempt < c.maxAttempts {
		c.sleep(c.backoff(attempt))
	}
}

func (c *Client) doRequest(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
// io.LimitReader, so a body larger than ceiling is detected WITHOUT
// ever asking the underlying reader for more than ceiling+1 bytes --
// mirrors adapters/eurostat.readWithCeiling and
// adapters/xlsx.readWithCeiling exactly (design.md "Response ceiling").
// INE's own documented failure mode REFUSES an oversized query outright
// (HTTP 200 with a volume-restriction envelope, see this package's doc
// comment) rather than serving it the way Eurostat does, so this
// ceiling is defence-in-depth against a misconfigured request, a
// source-side error page, or an interposing proxy -- not a response to
// any observed INE behaviour.
func readWithCeiling(r io.Reader, ceiling int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, ceiling+1))
	if err != nil {
		return nil, fmt.Errorf("ine: reading response body: %w", err)
	}
	if int64(len(body)) > ceiling {
		return nil, sourceerr.New(sourceerr.ResponseTooLarge, fmt.Sprintf("response exceeded the %d-byte ceiling", ceiling))
	}
	return body, nil
}
