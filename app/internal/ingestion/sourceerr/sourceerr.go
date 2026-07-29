// Package sourceerr is the FailureClass taxonomy shared by every source
// adapter (design.md package layout, "sourceerr/ # FailureClass
// taxonomy shared by all source adapters"). It exists because §9.2's
// retry/backoff policy must ask exactly one question about a failure --
// "may retrying ever help?" -- and every adapter (INE now, Eurostat and
// XLSX later) must answer it the same, typed way instead of each
// inventing its own ad-hoc error strings that retry code would then
// have to parse.
//
// The taxonomy mirrors postgres.DownloadOutcome's documented value set
// (migration 0001's comment; app/internal/adapters/postgres/download_attempt.go)
// exactly, so a classified sourceerr.Error maps to a download_attempt
// outcome with no translation table.
package sourceerr

import "fmt"

// FailureClass distinguishes WHY a source interaction failed.
//
// Only RetryableTransport is ever retried. The other four classes are
// permanent for the current attempt -- a source that has actively
// refused a request (SourceRefusal), returned a structurally valid but
// empty payload (SilentEmpty), drifted its schema or periodicity
// (SchemaDrift), or exceeded the configured response ceiling
// (ResponseTooLarge) will not start succeeding because the client waited
// and asked again. Retrying any of those four is the exact failure
// Finding A (Engram #4690) identified: the INE volume-restriction
// envelope is HTTP 200, so a client that does not classify it separately
// from a transport error would retry forever.
type FailureClass string

const (
	// RetryableTransport covers 5xx responses, timeouts, and network-level
	// errors -- conditions where the source or the network may recover on
	// its own before the retry budget is exhausted.
	RetryableTransport FailureClass = "retryable-transport"
	// SourceRefusal covers a source actively declining to serve the
	// request (the INE volume-restriction envelope is the first-class
	// example). HTTP 200 does not imply success.
	SourceRefusal FailureClass = "source-refusal"
	// SilentEmpty covers a structurally valid response carrying zero
	// observations -- the source did not error, it just had nothing to
	// say, and treating that as ordinary success would publish silently
	// wrong data (rule 1's non-emptiness backstop exists for the same
	// reason at the validation layer; this classification exists so the
	// adapter and download_attempt can already tell the two apart).
	SilentEmpty FailureClass = "silent-empty"
	// SchemaDrift covers a response whose shape no longer matches what
	// the adapter expects: renamed fields, and -- for INE specifically --
	// a periodicity mismatch against the configured assertion (design.md
	// "SchemaDrift covers renamed dimensions ... and periodicity
	// mismatch").
	SchemaDrift FailureClass = "schema-drift"
	// ResponseTooLarge covers a body that exceeded the configured
	// response ceiling (io.LimitReader, design.md "Response ceiling"
	// decision). Not produced by the INE client in this batch -- INE's
	// DATOS_SERIE responses are naturally small (ADR-2) -- but the class
	// exists here so every adapter shares one taxonomy.
	ResponseTooLarge FailureClass = "response-too-large"
)

// Retryable reports whether backoff/retry may ever help for c. Only
// RetryableTransport answers true; every other class fails fast.
func (c FailureClass) Retryable() bool {
	return c == RetryableTransport
}

// Error is a source-adapter failure carrying its FailureClass alongside
// a human-readable message. Adapters return *Error (never a bare
// fmt.Errorf) for any classified failure, so callers recover the class
// with errors.As instead of parsing a string to decide whether to retry
// (spec source-ingestion-ine, "it returns the named volume-restriction
// error, not an opaque JSON type error").
type Error struct {
	Class   FailureClass
	Message string
}

// New builds a classified *Error. Message is preserved verbatim --
// notably, the INE volume-restriction envelope's own Spanish text
// ("No puede mostrarse por restricciones de volumen") is data returned
// by the API and stays exactly as received, never translated or
// reworded.
func New(class FailureClass, message string) *Error {
	return &Error{Class: class, Message: message}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Class, e.Message)
}
