package xlsx

// Task 8.9: wires this package's Decode into indicators.SourceClient, the
// same port adapters/ine.Client and adapters/eurostat.Client satisfy, so
// app/internal/ingestion.IngestSeries works with this adapter unmodified
// (spec source-ingestion-xlsx, "using the same domain types, writer and
// validation harness as the API-backed sources").
//
// Unlike adapters/ine (baseURL + a per-call COD) or adapters/eurostat
// (baseURL + baked-in filters, dataset code per call), an xlsx-url
// source_ref's `ref` IS the full, already-resolved download URL itself
// (config/series/afiliacion-ss.yaml's source_refs[].kind: xlsx-url). This
// Client therefore carries no baseURL at all -- RequestURL/FetchRaw issue
// the request against ref directly.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// defaultMaxAttempts mirrors adapters/ine and adapters/eurostat's shared
// retry budget for RetryableTransport failures.
const defaultMaxAttempts = 5

// defaultMaxResponseBytes mirrors design.md's "Response ceiling" default
// (8 MiB). The real workbook is 54 KB -- comfortably under -- but every
// adapter carries the same defence-in-depth ceiling regardless of how
// small its own source happens to be today.
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
func WithMaxAttempts(n int) Option { return func(c *Client) { c.maxAttempts = n } }

// WithBackoff overrides the delay-before-retry function of the attempt
// number (1-indexed). Tests inject a fast function.
func WithBackoff(f func(attempt int) time.Duration) Option {
	return func(c *Client) { c.backoff = f }
}

// WithSleep overrides the function the retry loop calls to wait between
// attempts (default time.Sleep).
func WithSleep(f func(time.Duration)) Option { return func(c *Client) { c.sleep = f } }

// WithMaxResponseBytes overrides the response-size ceiling (default
// defaultMaxResponseBytes).
func WithMaxResponseBytes(n int64) Option { return func(c *Client) { c.maxResponseBytes = n } }

// Client is the XLSX SourceClient for one configured series: its
// workbook structure (schema) is fixed at construction, exactly like
// adapters/eurostat.Client bakes in its dimension filters, so the same
// Client type satisfies indicators.SourceClient exactly like the other
// two adapters do.
type Client struct {
	schema           config.XLSXSchemaConfig
	httpClient       *http.Client
	maxAttempts      int
	backoff          func(attempt int) time.Duration
	sleep            func(time.Duration)
	maxResponseBytes int64
}

// NewClient builds a Client that will parse every fetched workbook
// according to schema (config/series/{slug}.yaml's schema.xlsx block --
// PRD §9.4, structure lives in configuration, never in Go). httpClient
// may be nil, in which case http.DefaultClient is used.
func NewClient(schema config.XLSXSchemaConfig, httpClient *http.Client, opts ...Option) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	c := &Client{
		schema:           schema,
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

// RequestURL satisfies indicators.SourceClient: for an xlsx-url series,
// ref already IS the exact request URL (unlike INE/Eurostat, which build
// one from a baseURL + a code/dataset id) -- see the package doc comment.
func (c *Client) RequestURL(ref string) string { return ref }

// FetchRaw fetches ref's raw, undecoded workbook bytes -- no parsing, so
// a caller (the ingestion orchestrator) can archive the payload BEFORE
// any parsing happens, exactly like the other two adapters.
func (c *Client) FetchRaw(ctx context.Context, ref string) ([]byte, error) {
	return c.fetchWithRetry(ctx, ref)
}

// Decode satisfies indicators.SourceClient by delegating to the
// package-level Decode with this Client's baked-in schema.
func (c *Client) Decode(raw []byte, ref string, expectedFrequency indicators.Frequency) (indicators.SourceResult, error) {
	return Decode(raw, c.schema, expectedFrequency)
}

var _ indicators.SourceClient = (*Client)(nil)

// fetchWithRetry mirrors adapters/ine and adapters/eurostat's own:
// RetryableTransport is retried with backoff; every other classified
// failure (a non-200/non-5xx status, or the response ceiling) returns
// immediately, exactly one request issued.
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

// readWithCeiling mirrors adapters/eurostat's own: read at most
// ceiling+1 bytes via io.LimitReader, so a response larger than the
// configured ceiling is detected without ever asking the underlying
// reader for the full oversized body (design.md "Response ceiling").
func readWithCeiling(r io.Reader, ceiling int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, ceiling+1))
	if err != nil {
		return nil, fmt.Errorf("xlsx: reading response body: %w", err)
	}
	if int64(len(body)) > ceiling {
		return nil, sourceerr.New(sourceerr.ResponseTooLarge, fmt.Sprintf("response exceeded the %d-byte ceiling", ceiling))
	}
	return body, nil
}
