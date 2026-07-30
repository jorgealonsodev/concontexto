package eurostat

// Decode turns one JSON-stat 2.0 response body into an
// indicators.SourceResult (task 6.1/6.2, spec source-ingestion-eurostat
// "A JSON-stat response becomes canonical observations"). Unlike INE,
// which joins two separate wire fields (Anyo/T3_Periodo) into a period
// label before normalising, Eurostat's own "time" dimension category
// labels ("2026-Q1", "2026-06", "2025") are ALREADY the canonical shape
// indicators.NormalizePeriodLabel parses — so this decoder has no join
// step, only JSON-stat's own flat-value addressing to invert.
//
// JSON-stat 2.0 stores one flat "value" map keyed by a linear index
// computed, in "id" order, as a row-major (Horner) flattening of every
// dimension's selected category position: pos = 0; for each dim in id
// order: pos = pos*size[dim] + selectedIndex[dim]. Because
// config/series/*.yaml pins every dimension except "time" to exactly one
// category (spec "Every dimension except time MUST be pinned"), the
// formula below computes it generically rather than assuming that
// shape structurally — a genuinely correct JSON-stat reader, not a
// three-dataset special case.
//
// JSON-stat is also SPARSE, and that is not a detail: the "time"
// dimension declares every period the DATASET spans, while "value"
// carries an entry only where the source actually PUBLISHED a figure.
// Every configured series shows it live (verified 2026-07-30) —
// nama_10_gdp declares 1975 and publishes from 1995, une_rt_q declares
// 2003-Q1 and publishes from 2009-Q1, prc_hicp_minr declares 1996-01 and
// publishes from 1997-01. A position with no value therefore yields NO
// observation at all (see the value-presence branch below for why the
// alternative — a nil-valued or withdrawn-status observation — is both
// unwritable and untrue).

import (
	"encoding/json"
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// wireJSONStat is the subset of a JSON-stat 2.0 dataset response this
// adapter reads. Eurostat's own descriptive/provenance metadata
// ("extension") is not modelled here -- only "status" (task 2b.1/2b.2,
// spec source-ingestion-eurostat "JSON-stat status is read at the
// computed position"), keyed by the exact same linear position "value"
// already uses.
type wireJSONStat struct {
	Label     string                   `json:"label"`
	ID        []string                 `json:"id"`
	Size      []int                    `json:"size"`
	Value     map[string]float64       `json:"value"`
	Status    map[string]string        `json:"status"`
	Dimension map[string]wireDimension `json:"dimension"`
}

type wireDimension struct {
	Category wireCategory `json:"category"`
}

type wireCategory struct {
	// Index maps a category's own id (e.g. a time label "2026-06", or a
	// pinned code like "ES") to its zero-based ordinal position within
	// that dimension. JSON-stat 2.0 allows this to be an id-ordered array
	// too, but every response verified live 2026-07-28 (testdata/source.txt)
	// carries the explicit object form, so no order assumption is needed.
	Index map[string]int `json:"index"`
}

// Decode decodes and normalizes raw into an indicators.SourceResult,
// asserting the response's actual periodicity (derived from each
// observation's own normalised Period, since Eurostat's time labels are
// already canonical) against expectedFrequency. ref is the dataset code,
// used only for diagnostic messages.
func Decode(raw []byte, ref string, expectedFrequency indicators.Frequency, segments ...indicators.CadenceSegment) (indicators.SourceResult, error) {
	var ws wireJSONStat
	if err := json.Unmarshal(raw, &ws); err != nil {
		return indicators.SourceResult{}, fmt.Errorf("eurostat: decoding %s response: %w", ref, err)
	}

	sizeByDim := make(map[string]int, len(ws.ID))
	for i, dim := range ws.ID {
		if i < len(ws.Size) {
			sizeByDim[dim] = ws.Size[i]
		}
	}

	timeDim, ok := ws.Dimension["time"]
	if !ok || len(timeDim.Category.Index) == 0 {
		return indicators.SourceResult{}, sourceerr.New(sourceerr.SilentEmpty, fmt.Sprintf("%s returned no time periods", ref))
	}

	// pinnedIndex holds, for every non-time dimension, the single
	// selected category's ordinal position -- required for every
	// non-time dimension to carry exactly one category (config-validation
	// time enforcement, spec "Every dimension except time MUST be
	// pinned"); a dimension with zero categories (the dead-code
	// coicop18=CP00 shape, spec "A dead dimension code is caught as an
	// empty result") or more than one is reported as SilentEmpty /
	// SchemaDrift respectively rather than silently guessing.
	pinnedIndex := make(map[string]int, len(ws.ID))
	for _, dim := range ws.ID {
		if dim == "time" {
			continue
		}
		cat, ok := ws.Dimension[dim]
		if !ok {
			return indicators.SourceResult{}, fmt.Errorf("eurostat: %s response has no dimension %q declared in its own id list", ref, dim)
		}
		switch len(cat.Category.Index) {
		case 0:
			return indicators.SourceResult{}, sourceerr.New(sourceerr.SilentEmpty, fmt.Sprintf("%s dimension %q has zero categories (a dead dimension code)", ref, dim))
		case 1:
			for _, idx := range cat.Category.Index {
				pinnedIndex[dim] = idx
			}
		default:
			return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("%s dimension %q carries %d categories, expected exactly 1 (every dimension except time must be pinned)", ref, dim, len(cat.Category.Index)))
		}
	}

	type timeSlot struct {
		label string
		index int
	}
	slots := make([]timeSlot, 0, len(timeDim.Category.Index))
	for label, idx := range timeDim.Category.Index {
		slots = append(slots, timeSlot{label: label, index: idx})
	}
	// Sort by the dimension's own declared ordinal (chronological order
	// in every verified Eurostat response, testdata/source.txt), not by
	// map iteration order, which Go deliberately randomises.
	for i := 1; i < len(slots); i++ {
		for j := i; j > 0 && slots[j].index < slots[j-1].index; j-- {
			slots[j], slots[j-1] = slots[j-1], slots[j]
		}
	}

	observations := make([]indicators.Observation, 0, len(slots))
	var breakSignals []indicators.BreakSignal
	for _, slot := range slots {
		period, err := indicators.NormalizePeriodLabel(slot.label)
		if err != nil {
			return indicators.SourceResult{}, fmt.Errorf("eurostat: %s: normalising time label %q: %w", ref, slot.label, err)
		}

		pos := 0
		for _, dim := range ws.ID {
			idx := slot.index
			if dim != "time" {
				idx = pinnedIndex[dim]
			}
			pos = pos*sizeByDim[dim] + idx
		}
		posKey := fmt.Sprintf("%d", pos)

		rawValue, hasValue := ws.Value[posKey]
		flag, hasFlag := ws.Status[posKey]

		// Task 2b.1-2b.5 (GREEN): status is read at the SAME posKey
		// "value" already uses (spec "JSON-stat status is read at the
		// computed position") and classified per design D-3's mapping
		// table. Eurostat publishes no definitive flag -- an absent entry
		// (the ordinary case; also true when ws.Status is nil, an empty
		// map, or simply carries no key for this pos) means definitive,
		// never a schema-drift rejection (spec "Absence of a flag means
		// definitive"). classifyEurostatFlag only runs when an entry
		// exists.
		//
		// The flag's VOCABULARY is judged here, before the value-presence
		// branch below, deliberately: an undocumented flag keeps the
		// diagnostic its own spec scenario asks for ("An unrecognised flag
		// fails closed" -- "naming the dataset and the unrecognised flag")
		// whether or not it also happens to sit on a position carrying no
		// value, rather than being masked by the newer alignment check.
		status := indicators.ObservationStatusDefinitive
		sourceStatus := ""
		breakOrDefinition := false
		if hasFlag {
			classified, isBreakOrDefinition, classifyErr := classifyEurostatFlag(flag)
			if classifyErr != nil {
				return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf(
					"%s: %v at period %s", ref, classifyErr, period))
			}
			status = classified
			sourceStatus = flag
			breakOrDefinition = isBreakOrDefinition
		}

		// A JSON-stat response is SPARSE: the "time" dimension declares
		// every period the DATASET spans, while "value" carries an entry
		// only where the source actually PUBLISHED a figure. All three
		// configured series exhibit this live (verified 2026-07-30,
		// testdata/source.txt): nama_10_gdp's time dimension reaches back
		// to 1975 while its earliest published value is 1995; une_rt_q
		// declares 2003-Q1 and publishes from 2009-Q1; prc_hicp_minr
		// declares 1996-01 and publishes from 1997-01.
		//
		// A position with no value is therefore "the source published
		// nothing here", and the honest projection of that is NO
		// observation -- not an observation with a nil Value. Two reasons,
		// both load-bearing:
		//
		//  1. The schema forbids the row outright: observation's own
		//     CHECK (value IS NOT NULL OR status = 'W') (migration
		//     0001_fase0_schema) rejects a null value under any status but
		//     withdrawn, so a nil-valued Definitive observation could
		//     never be written -- it failed at the publish gate, which is
		//     how this defect surfaced: NO Eurostat series could be
		//     ingested at all.
		//  2. Coercing it to 'W' to satisfy that CHECK would be a lie.
		//     Status W is a WITHDRAWAL: spec data-model-vintages, "Source
		//     withdrawal is representable" -- "A source withdrawing a
		//     period MUST be recorded as a new version with withdrawn
		//     status, never as a delete", whose scenario is GIVEN a
		//     current observation ... WHEN the source STOPS PUBLISHING
		//     that period. A period never published in the first place was
		//     never withdrawn, and there is a whole rollback/tombstone
		//     mechanism keyed to that meaning. Fabricating a retraction
		//     that never happened is precisely the silent lie design D-3
		//     exists to remove ("coercing unknown tokens to D is exactly
		//     the silent lie this change exists to remove").
		//
		// Dropping is also what the spec's own zero-observation
		// requirement already assumes: "A zero-observation result fails
		// the run" describes a dead dimension code answering HTTP 200 with
		// "value": {} as a run yielding ZERO observations -- not a run of
		// null-valued ones. That requirement is only actually satisfiable
		// once a valueless position stops becoming an observation (see the
		// SilentEmpty check after this loop).
		if !hasValue {
			// A flag with no value to annotate. The spec requires the
			// decoder to "carry the verbatim flag through to the
			// observation's source_status" (spec "JSON-stat status is read
			// at the computed position"); with no observation to carry it,
			// that MUST is unsatisfiable, leaving only two options --
			// silently discard published source information, or fail
			// closed. This adapter fails closed, for the same reason every
			// other undecided token in it does ("An unrecognised flag
			// fails closed", design D-3's "fails SchemaDrift until the
			// spec allowlists a token with a decided projection"), and for
			// one more that is specific to this shape: a flag landing on a
			// position the source published no value for is ALSO exactly
			// what a bug in the linear (Horner) position arithmetic above
			// would look like, and the spec has a scenario protecting that
			// very alignment ("Status is aligned with the value it belongs
			// to"). Failing closed turns a silent misalignment into a
			// named SchemaDrift naming dataset, period and flag; ignoring
			// the flag would hide it.
			//
			// No live response in this project exhibits the shape
			// (verified 2026-07-30 across all three full-history payloads:
			// zero positions carry a "status" entry without a "value"
			// entry), so failing closed costs nothing today and escalates
			// to a human -- with the dataset and period in the message --
			// if Eurostat ever starts publishing it.
			//
			// Consequently a "b"/"d" flag on a valueless position emits no
			// break signal either: the whole run fails, so nothing is
			// emitted at all. A break annotation beside series_break
			// (spec "Break and definition flags are metadata, not
			// statuses") is metadata ABOUT an observation this payload
			// does not contain.
			if hasFlag {
				return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf(
					"%s carries status flag %q at period %s but no value at that position -- a flag cannot annotate an observation the source did not publish",
					ref, flag, period))
			}
			continue
		}

		value := rawValue
		if breakOrDefinition {
			breakSignals = append(breakSignals, indicators.BreakSignal{Period: period, Flag: flag})
		}

		observations = append(observations, indicators.Observation{
			Period: period, Value: &value, Status: status, SourceStatus: sourceStatus,
		})
	}

	// Once valueless positions stop becoming observations, a structurally
	// valid response that published nothing at any position decodes to
	// zero observations -- the exact "value": {} shape a dead dimension
	// code returns under HTTP 200 (spec "A zero-observation result fails
	// the run": "it fails with the zero-observation failure class,
	// distinct from a transport error ... and nothing is published"). It
	// is classified here, in the adapter, exactly as ine/envelope.go
	// classifies its own zero-row body, so the two SourceClient
	// implementations answer "the source had nothing to say" the same
	// named way instead of one relying on Rule 6 downstream while the
	// other reports it at the source. Rule 6 (validation/rule6_nonempty)
	// remains the second, independent net for any path that does reach it.
	if len(observations) == 0 {
		return indicators.SourceResult{}, sourceerr.New(sourceerr.SilentEmpty, fmt.Sprintf(
			"%s returned no observed values across its %d declared time periods", ref, len(slots)))
	}

	// Task 1.2 (GREEN, "same principle in eurostat" -- design D-4):
	// periodicity is classified over the WHOLE payload, then audited
	// against the declared cadence segments (empty means the ordinary
	// uniform case). No eurostat-sourced series declares cadence_segments
	// today -- this generalises the same shared mechanism envelope.go's
	// INE adapter uses, rather than leaving eurostat on the single-
	// observation check the population defect showed was insufficient.
	for _, o := range observations {
		if actual := o.Period.Frequency; actual != expectedFrequency {
			return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf(
				"%s periodicity mismatch: expected %s, got %s", ref, expectedFrequency, actual))
		}
	}
	periods := make([]indicators.Period, 0, len(observations))
	for _, o := range observations {
		periods = append(periods, o.Period)
	}
	if err := indicators.AssertCadence(periods, expectedFrequency, segments); err != nil {
		return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("%s: %v", ref, err))
	}

	return indicators.SourceResult{Name: ws.Label, Observations: observations, BreakSignals: breakSignals}, nil
}

// classifyEurostatFlag maps one Eurostat JSON-stat status flag to the
// shared domain status (design D-3's mapping table), and reports whether
// the flag is break/definition metadata rather than a status ("b"/"d",
// spec "Break and definition flags are metadata, not statuses"). It is
// only called for a flag that IS present -- absence is the caller's own,
// separate "definitive, no entry needed" branch, never routed through
// here (unlike INE's classifyTipoDato, which fails closed on a MISSING
// token too: Eurostat's own vocabulary has no "definitive" flag at all,
// so a missing entry is not an unknown case to classify, it is the
// documented default).
//
// The flag vocabulary is "p e b d f u c n :" (spec's own list); only "p"
// (provisional) and "b"/"d" (break/definition metadata) have a decided
// projection today. Every other flag -- "e", "f", "u", "c", "n", ":", or
// anything undocumented -- fails closed as sourceerr.SchemaDrift by the
// caller (spec "An unrecognised flag fails closed"), mirroring
// adapters/ine.classifyTipoDato's own unrecognised-token handling: two
// adapters satisfying the same indicators.SourceClient contract answer
// "is this status token known?" the same fail-closed way, never one
// coercing silently while the other rejects.
func classifyEurostatFlag(flag string) (status indicators.ObservationStatus, isBreakOrDefinition bool, err error) {
	switch flag {
	case "p":
		return indicators.ObservationStatusProvisional, false, nil
	case "b", "d":
		return indicators.ObservationStatusDefinitive, true, nil
	default:
		return "", false, fmt.Errorf("unrecognised status flag %q", flag)
	}
}
