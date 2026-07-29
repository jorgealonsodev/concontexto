// Package freshness holds the PURE freshness decision that drives the
// amber semaphore (spec raw-file-archive, "Freshness state is tracked
// per source"; PRD §9.2, §6.1.1). Resolve never reads the system clock
// itself — the reference point is always an explicit parameter, the
// same pattern PR 4a established for the six validation rules
// (design.md "Rule func(ctx SeriesContext, incoming []Observation)
// []Finding") — so the 24h window is testable without sleeping or
// faking time.Now(). The database query that supplies the one fact this
// package needs (a source's last successful download_attempt) lives in
// the postgres adapter (postgres.SourceFreshness / SeriesFreshness);
// this package only decides, given that fact, what state a source is
// in.
package freshness

import "time"

// Window is the maximum time a source may go without a successful
// download_attempt before it is considered failed (spec "Twenty-four
// hours without success marks the source stale").
const Window = 24 * time.Hour

// State is a source's freshness verdict.
type State string

const (
	StateFresh  State = "fresh"
	StateFailed State = "failed"
)

// RaisesIncident reports whether state is incident-worthy: only
// StateFailed is (spec raw-file-archive's amber-semaphore escalation;
// task 6.12/6.13's Eurostat maintenance-window scenario — a 2-hour-old
// success during a scheduled maintenance window resolves StateFresh and
// therefore raises no incident, it simply retries on the next scheduled
// attempt; unavailability that outlasts the 24h Window resolves
// StateFailed and DOES raise one). This method exists so a future
// alert-emission step (task 9.7/9.8, not built yet) has one named,
// tested boundary to call instead of re-deriving "which State means
// page someone" from State's two string values by hand.
//
// Deliberately NOT a per-source or per-maintenance-window concept:
// Resolve already takes no source identity, and Eurostat's own
// scheduled-maintenance windows need no special-casing here — the same
// 24h rule that already governs INE outages governs them too (design.md
// "extend PR 5a-i's freshness handling, do not fork it").
func (s State) RaisesIncident() bool {
	return s == StateFailed
}

// Resolve decides a source's freshness state as of asOf, given the time
// of its last successful download_attempt (nil if it has never
// succeeded). Resolve carries no state across calls, so two sources
// evaluated independently — one stale, one healthy — never influence
// each other (spec "One failing source does not affect another").
func Resolve(lastSuccess *time.Time, asOf time.Time) State {
	if lastSuccess == nil {
		return StateFailed
	}
	if asOf.Sub(*lastSuccess) > Window {
		return StateFailed
	}
	return StateFresh
}
