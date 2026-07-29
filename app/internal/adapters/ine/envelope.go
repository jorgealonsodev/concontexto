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
	Anyo    int      `json:"Anyo"`
	Periodo string   `json:"T3_Periodo"`
	Valor   *float64 `json:"Valor"`
	Secreto bool     `json:"Secreto"`
}

func decodeAndNormalize(body []byte, cod string, expectedFrequency indicators.Frequency) (Result, error) {
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

	if actual := detectPeriodicity(wr.Data[0].Periodo); actual != expectedFrequency {
		return Result{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf(
			"DATOS_SERIE/%s periodicity mismatch: expected %s, got %s",
			cod, expectedFrequency, periodicityLabel(actual, wr.Data[0].Periodo)))
	}

	observations := make([]Observation, 0, len(wr.Data))
	for _, d := range wr.Data {
		period, err := normalizeINEPeriod(d.Anyo, d.Periodo)
		if err != nil {
			return Result{}, fmt.Errorf("ine: DATOS_SERIE/%s: %w", cod, err)
		}
		observations = append(observations, Observation{Period: period, Value: d.Valor})
	}

	return Result{COD: cod, Name: wr.Nombre, Observations: observations}, nil
}
