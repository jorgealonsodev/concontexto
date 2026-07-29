package ingestion

// Task 7.4/7.5, 7.8/7.9: ReconcileEditorialConfig is design.md's third
// named ingestion/ function (package doc comment on ingest.go), built in
// this batch. It turns the parsed config.Config's Breaks/Events into
// digested inputs and delegates the actual transactional full reconcile
// to postgres.ReconcileBreaks/ReconcileEvents (see that file's package
// doc comment for the insert/update-in-place/soft-retire discipline).
//
// A break or event whose DateStatus is "unconfirmed" (Date/DateStart is
// nil) is NEVER projected into series_break/event: this function records
// its id in PendingBreakIDs/PendingEventIDs instead of guessing a date.
// A wrong break date silently corrupts every comparison across it — the
// portal's own worst failure mode — so an entry whose effective date is
// not yet confirmed against its source's methodological note stays
// documented in rupturas.yaml/eventos.yaml, visible as "pending", and
// reconciled once its date is confirmed and the YAML is edited
// accordingly (which itself goes through the normal in-place-update path,
// task 7.4/7.5, once Date stops being nil).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

// ReconcileResult is one ReconcileEditorialConfig call's combined effect.
type ReconcileResult struct {
	Breaks postgres.ReconcileCounts
	Events postgres.ReconcileCounts

	// PendingBreakIDs/PendingEventIDs list every entry this run declined
	// to project because its date is not yet confirmed (see the package
	// doc comment above) — never nil-but-silently-dropped.
	PendingBreakIDs []string
	PendingEventIDs []string
}

// ReconcileEditorialConfig reconciles cfg.Breaks/cfg.Events into
// series_break/event inside ONE transaction (spec editorial-config,
// "Editorial YAML is authoritative and reconciled transactionally" — "A
// failed reconcile leaves no partial state" is written against ONE
// reconcile call, not "the breaks half of one reconcile". Earlier PR 7a
// ran postgres.ReconcileBreaks/ReconcileEvents as two independent
// top-level transactions, disclosed as a gap: a failure in the events
// half could leave an already-committed breaks change durable. This now
// delegates to postgres.ReconcileEditorial, which shares one tx across
// both tables — see TestReconcileEditorialConfig_MidTransactionFailureAcrossBreaksAndEventsLeavesDatabaseByteIdentical).
func ReconcileEditorialConfig(ctx context.Context, db postgres.TxBeginner, cfg config.Config) (ReconcileResult, error) {
	var result ReconcileResult

	var breakInputs []postgres.SeriesBreakInput
	for _, b := range cfg.Breaks {
		if b.Date == nil {
			result.PendingBreakIDs = append(result.PendingBreakIDs, b.ID)
			continue
		}
		breakInputs = append(breakInputs, postgres.SeriesBreakInput{
			BreakKey: b.ID, ScopeKind: b.Scope.Kind, ScopeRef: b.Scope.Ref,
			Date: *b.Date, Kind: b.Kind, NoteMD: b.NoteMD, SourceURL: b.SourceURL,
			ConfigDigest: breakDigest(b),
		})
	}

	var eventInputs []postgres.EventInput
	for _, e := range cfg.Events {
		if e.DateStart == nil {
			result.PendingEventIDs = append(result.PendingEventIDs, e.ID)
			continue
		}
		eventInputs = append(eventInputs, postgres.EventInput{
			ID: e.ID, Group: e.Group, Name: e.Name,
			DateStart: *e.DateStart, DateEnd: e.DateEnd, NoteMD: e.NoteMD,
			ConfigDigest: eventDigest(e),
		})
	}

	breakCounts, eventCounts, err := postgres.ReconcileEditorial(ctx, db, breakInputs, eventInputs)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("ingestion: reconciling editorial config: %w", err)
	}
	result.Breaks = breakCounts
	result.Events = eventCounts

	return result, nil
}

// breakDigest/eventDigest compute the per-entry SHA-256 config_digest
// (spec "Every reconciled database row MUST record a config_digest
// (SHA-256 of its source YAML) so an edit is distinguishable from a
// delete plus insert") over exactly the fields that reach the database
// row. Scoped per entry, not per file: editing one break's description
// must change ONLY that break's digest (task 7.4), not every other
// entry's in the same rupturas.yaml file.
func breakDigest(b config.BreakConfig) string {
	h := sha256.New()
	fmt.Fprintf(h, "id=%s\nkind=%s\nscope.kind=%s\nscope.ref=%s\ndate=%s\nnote_md=%s\nsource_url=%s\n",
		b.ID, b.Kind, b.Scope.Kind, b.Scope.Ref, dateDigestString(b.Date), b.NoteMD, b.SourceURL)
	return hex.EncodeToString(h.Sum(nil))
}

func eventDigest(e config.EventConfig) string {
	h := sha256.New()
	fmt.Fprintf(h, "id=%s\ngroup=%s\nname=%s\ndate_start=%s\ndate_end=%s\nnote_md=%s\n",
		e.ID, e.Group, e.Name, dateDigestString(e.DateStart), dateDigestString(e.DateEnd), e.NoteMD)
	return hex.EncodeToString(h.Sum(nil))
}

func dateDigestString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}
