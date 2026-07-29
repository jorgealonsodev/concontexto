package ine

// Remediation batch 4 (W12 follow-up): adapters/ine had NO
// response-size ceiling mechanism at all — doRequest called
// io.ReadAll(resp.Body) unbounded — despite ine.yaml declaring the
// identical max_response_bytes key adapters/eurostat and adapters/xlsx
// already enforce. This file mirrors
// adapters/eurostat/ceiling_test.go exactly: readWithCeiling must abort
// BEFORE fully buffering a body larger than its ceiling, not merely
// reject it AFTER io.ReadAll has already read the whole thing.
// boundedFailReader is a structural proof: it fails the test outright
// if asked to produce even one byte beyond ceiling+1, so a passing test
// means io.LimitReader — not the source's willingness to stop sending —
// is what bounds memory here.
//
// This is a white-box (package ine, not ine_test) file because
// readWithCeiling is deliberately unexported: it is doRequest's internal
// implementation detail, not part of the adapter's public surface
// (RequestURL/FetchRaw/Decode/FetchSeries already cover that, per
// indicators.SourceClient).

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// boundedFailReader never returns io.EOF on its own; it fails t
// immediately if a single Read call would push total bytes served past
// max. The property under test is exactly "the caller never requests
// more than ceiling+1 bytes in total" — a violation of that IS the test
// failure, not something checked afterwards from a length that already
// grew unbounded.
type boundedFailReader struct {
	t   *testing.T
	max int64
	n   int64
}

func (r *boundedFailReader) Read(p []byte) (int, error) {
	if r.n+int64(len(p)) > r.max {
		r.t.Fatalf("readWithCeiling requested %d more byte(s) with %d of %d already served — it must never ask its source for more than ceiling+1 bytes total", len(p), r.n, r.max)
	}
	for i := range p {
		p[i] = 'x'
	}
	r.n += int64(len(p))
	return len(p), nil
}

func TestReadWithCeiling_BodyOverCeilingFailsWithoutFullyBuffering(t *testing.T) {
	const ceiling = 1024
	// r can, in principle, produce unlimited data (it never returns
	// EOF) — exactly the shape of a real oversized response streaming
	// far more than the configured ceiling. max is set to precisely
	// ceiling+1: any Read call requesting bytes beyond that fails the
	// test immediately, proving the abort happens strictly before a
	// full buffer of the oversized body would ever exist.
	r := &boundedFailReader{t: t, max: ceiling + 1}

	_, err := readWithCeiling(r, ceiling)
	if err == nil {
		t.Fatal("expected a response-too-large error for a source larger than the ceiling")
	}
	var classified *sourceerr.Error
	if !errors.As(err, &classified) || classified.Class != sourceerr.ResponseTooLarge {
		t.Fatalf("expected sourceerr.ResponseTooLarge, got %v", err)
	}
	if r.n > ceiling+1 {
		t.Fatalf("readWithCeiling consumed %d byte(s) from its source, want at most %d (ceiling+1)", r.n, ceiling+1)
	}
}

func TestReadWithCeiling_BodyUnderCeilingPasses(t *testing.T) {
	const ceiling = 1024
	body, err := readWithCeiling(bytes.NewReader(make([]byte, ceiling-10)), ceiling)
	if err != nil {
		t.Fatalf("readWithCeiling: %v", err)
	}
	if len(body) != ceiling-10 {
		t.Fatalf("expected %d bytes, got %d", ceiling-10, len(body))
	}
}

func TestReadWithCeiling_BodyExactlyAtCeilingPasses(t *testing.T) {
	const ceiling = 1024
	body, err := readWithCeiling(bytes.NewReader(make([]byte, ceiling)), ceiling)
	if err != nil {
		t.Fatalf("readWithCeiling: %v", err)
	}
	if len(body) != ceiling {
		t.Fatalf("expected exactly %d bytes (at, not over, the ceiling must pass), got %d", ceiling, len(body))
	}
}

func TestReadWithCeiling_TransportErrorIsNotMisclassifiedAsResponseTooLarge(t *testing.T) {
	wantErr := errors.New("connection reset by peer")
	_, err := readWithCeiling(&erroringReader{err: wantErr}, 1024)
	if err == nil {
		t.Fatal("expected an error")
	}
	var classified *sourceerr.Error
	if errors.As(err, &classified) {
		t.Fatalf("expected an ordinary wrapped transport error, not a classified sourceerr.Error (that classification belongs to fetchWithRetry's retry decision, not to this helper), got %v", err)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the underlying transport error to be recoverable via errors.Is, got %v", err)
	}
}

type erroringReader struct{ err error }

func (r *erroringReader) Read([]byte) (int, error) { return 0, r.err }

var _ io.Reader = (*erroringReader)(nil)
