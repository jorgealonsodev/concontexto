package ine

// decodeAndNormalize turns one DATOS_SERIE HTTP 200 body into a Result:
// decode -- discriminating the volume-restriction refusal envelope from
// a real success body BEFORE trying to read either one's fields
// (5a.13/5a.14, Finding A/Engram #4690) -- then assert periodicity
// BEFORE normalising a single period (5a.11/5a.12, periodicity.go), then
// normalise every observation (period.go). A structurally valid,
// zero-observation success is classified sourceerr.SilentEmpty rather
// than treated as an ordinary (empty) success, so nothing downstream
// mistakes "the source had nothing to say" for "nothing changed."
//
// Normalising an observation also REFUSES one whose Valor is an explicit
// null (verify-report WARNING-30): the project has no decided meaning for
// that, and the row carries no marker that would supply one, so the
// decode fails closed rather than emitting a nil-valued observation. See
// the branch's own comment in the loop below for the full argument and
// for why it deliberately does not mirror the Eurostat sparse-position
// fix (befa81f).

import (
	"encoding/json"
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// wireResponse is the union of DATOS_SERIE's two real response shapes:
// a successful series body (Nombre + Data) and the volume-restriction
// refusal envelope (Status). Checking Status FIRST means the refusal
// case is a NAMED sourceerr.SourceRefusal, never an opaque
// encoding/json error from trying to read Data out of a body that never
// had a Data field to begin with (spec source-ingestion-ine, "it
// returns the named volume-restriction error, not an opaque JSON type
// error").
type wireResponse struct {
	Status *string           `json:"status"`
	Nombre string            `json:"Nombre"`
	Data   []wireObservation `json:"Data"`
}

type wireObservation struct {
	Anyo     int      `json:"Anyo"`
	Periodo  string   `json:"T3_Periodo"`
	Valor    *float64 `json:"Valor"`
	TipoDato string   `json:"T3_TipoDato"`
}

func decodeAndNormalize(body []byte, cod string, expectedFrequency indicators.Frequency, segments []indicators.CadenceSegment) (Result, error) {
	var wr wireResponse
	if err := json.Unmarshal(body, &wr); err != nil {
		return Result{}, fmt.Errorf("ine: decoding DATOS_SERIE/%s response: %w", cod, err)
	}

	if wr.Status != nil {
		// The refusal envelope's own Spanish text is data returned by
		// the API -- preserved verbatim, never translated or reworded.
		return Result{}, sourceerr.New(sourceerr.SourceRefusal, *wr.Status)
	}

	if len(wr.Data) == 0 {
		return Result{}, sourceerr.New(sourceerr.SilentEmpty, fmt.Sprintf("DATOS_SERIE/%s returned zero observations", cod))
	}

	// Task 1.2 (GREEN): periodicity is classified over the WHOLE payload,
	// not Data[0] alone (spec source-ingestion-ine, "Periodicity MUST be
	// detected over the whole payload"). Every row's raw shape must match
	// expectedFrequency's shape -- a genuinely different SHAPE (e.g.
	// annual "A" returned for a quarterly-configured series) is a
	// mismatch regardless of where in the payload it appears.
	for _, d := range wr.Data {
		if actual := detectPeriodicity(d.Periodo); actual != expectedFrequency {
			return Result{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf(
				"DATOS_SERIE/%s periodicity mismatch: expected %s, got %s",
				cod, expectedFrequency, periodicityLabel(actual, d.Periodo)))
		}
	}

	observations := make([]Observation, 0, len(wr.Data))
	periods := make([]indicators.Period, 0, len(wr.Data))
	for _, d := range wr.Data {
		period, err := normalizeINEPeriod(d.Anyo, d.Periodo)
		if err != nil {
			return Result{}, fmt.Errorf("ine: DATOS_SERIE/%s: %w", cod, err)
		}
		// verify-report WARNING-30 (GREEN): an explicit "Valor": null
		// fails the decode CLOSED here, rather than being carried through
		// as an observation with a nil value (spec source-ingestion-ine,
		// "A null Valor has no decided meaning and fails closed").
		//
		// This is deliberately NOT the shape of the Eurostat fix
		// (befa81f, adapters/eurostat): a sparse JSON-stat value map
		// simply has no entry at a position, which is the ABSENCE of a
		// datum and is decidable -- the source published nothing for that
		// period, so no observation is emitted. INE is the opposite: a row
		// EXISTS and carries an explicit null. That is a positive act by
		// the source, not an absence, and silently discarding it would
		// throw away something INE chose to publish.
		//
		// Why refuse instead of projecting it onto SOME status: a
		// DATOS_SERIE row carries exactly five fields (Fecha, Anyo,
		// T3_Periodo, T3_TipoDato, Valor -- verified live 2026-07-30
		// across all six configured series, 1,032 rows). There is no
		// secrecy marker, no not-applicable flag, no annotation of any
		// kind. A null therefore arrives carrying NO information about
		// why, so statistical secrecy, a not-applicable period and a
		// genuine gap are indistinguishable from the payload alone, and
		// every available interpretation would be an invention. Marking it
		// withdrawn to satisfy migration 0001's
		// CHECK (value IS NOT NULL OR status = 'W') would be the worst of
		// them: data-model-vintages defines withdrawal as a source
		// STOPPING publication of a period it previously published, so a
		// period the source is publishing RIGHT NOW, as a null, was never
		// withdrawn. This mirrors ingestion.firstUnclassifiedStatus, which
		// fails a whole run closed rather than default an unclassified
		// status.
		//
		// Latent, not live: those same 1,032 rows carry zero nulls. What
		// this buys is that the day INE publishes one, the run stops HERE
		// with a legible refusal naming the series, the period and the
		// row, instead of the cryptic
		// `violates check constraint "observation_check" (SQLSTATE 23514)`
		// the publish gate produces from deep inside the observation
		// writer.
		//
		// Placed in decodeAndNormalize, BELOW Client.Decode's own
		// classifyTipoDato, which means a row that is surprising twice (a
		// null AND an unrecognised token) reports the null first -- the
		// opposite of befa81f's ordering, where Eurostat judges flag
		// vocabulary before value presence. The asymmetry is intentional:
		// DecodeSeries and FetchSeries are exported and never run
		// classifyTipoDato at all, so a check placed one layer up would
		// leave two entry points able to return a nil-valued
		// ine.Observation. Containment of the invariant beats symmetry of
		// the diagnostic, and the token surprise is not lost -- it
		// surfaces on the next run once the null is decided.
		if d.Valor == nil {
			return Result{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf(
				"DATOS_SERIE/%s: %s carries a null Valor (row: Anyo=%d, T3_Periodo=%q, T3_TipoDato=%q, Valor=null). "+
					"This project has no decided meaning for a null Valor and will not publish a guess: a DATOS_SERIE "+
					"row carries no secrecy marker, no not-applicable flag and no annotation, so statistical secrecy, "+
					"a not-applicable period and a genuine gap are indistinguishable from this payload alone. To "+
					"resolve this, decide the projection in spec source-ingestion-ine -- which domain status a null "+
					"Valor carries, or that the row yields no observation -- and record why; this run's archived raw "+
					"file holds the verbatim payload to decide against.",
				cod, period, d.Anyo, d.Periodo, d.TipoDato))
		}

		// Task 2a.1/2a.2 (GREEN): T3_TipoDato is carried through VERBATIM
		// and UNVALIDATED here -- classification and fail-closed rejection
		// of an unrecognised/missing token happens one layer up, in
		// Client.Decode (classifyTipoDato below), so this package's own
		// periodicity/cadence unit tests can keep exercising DecodeSeries
		// in isolation without also supplying a status token (spec
		// source-ingestion-ine, "The source's data-type token is carried
		// through to the domain").
		observations = append(observations, Observation{Period: period, Value: d.Valor, SourceStatus: d.TipoDato})
		periods = append(periods, period)
	}

	// Task 1.2 (GREEN): a declared cadence that contradicts the observed
	// history (a uniform declaration against a mixed-cadence payload, or
	// an invented period inside a declared segment) fails closed here,
	// BEFORE the run reaches validation (design D-4).
	if err := indicators.AssertCadence(periods, expectedFrequency, segments); err != nil {
		return Result{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("DATOS_SERIE/%s: %v", cod, err))
	}

	return Result{COD: cod, Name: wr.Nombre, Observations: observations}, nil
}

// classifyTipoDato maps INE's T3_TipoDato token to the shared domain
// observation status (design D-3). It is called from Client.Decode, not
// from decodeAndNormalize above -- see that function's own comment for
// why the split matters to this package's unit tests.
//
// INE's own OpenAPI schema (TiposDatosJSON) declares Id, Nombre and
// Codigo with NO enum for this field, so the domain is not enumerated by
// the source: any token this switch does not recognise -- including an
// empty string, when a tip=A response omits the field entirely -- fails
// closed instead of being silently coerced to definitive (spec
// source-ingestion-ine, "An unknown data-type token fails closed";
// mirrors detectPeriodicity's own unrecognised-shape handling in
// periodicity.go).
func classifyTipoDato(token string) (indicators.ObservationStatus, error) {
	switch token {
	case "Definitivo":
		return indicators.ObservationStatusDefinitive, nil
	case "Provisional":
		return indicators.ObservationStatusProvisional, nil
	default:
		return "", fmt.Errorf("unrecognised T3_TipoDato %q", token)
	}
}
