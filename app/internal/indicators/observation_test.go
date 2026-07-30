package indicators_test

// Task 2a.1 (RED, partial): indicators.Observation must be able to carry
// the source's data-type classification through the domain (design D-3,
// spec data-model-vintages "The source's verbatim status token is
// preserved"). ObservationStatus mirrors postgres.ObservationStatus's own
// P/D/W value set but lives here, as its own type, because indicators
// MUST NOT import the postgres adapter (validation/purity_test.go's
// import-graph guard).

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func TestObservation_CarriesStatusAndSourceStatus(t *testing.T) {
	value := 10.5
	obs := indicators.Observation{
		Period:       indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1},
		Value:        &value,
		Status:       indicators.ObservationStatusProvisional,
		SourceStatus: "Provisional",
	}
	if obs.Status != indicators.ObservationStatusProvisional {
		t.Errorf("expected Status to round-trip as Provisional, got %q", obs.Status)
	}
	if obs.SourceStatus != "Provisional" {
		t.Errorf("expected SourceStatus to round-trip verbatim, got %q", obs.SourceStatus)
	}
}

func TestObservationStatus_ValueSet(t *testing.T) {
	cases := map[indicators.ObservationStatus]string{
		indicators.ObservationStatusProvisional: "P",
		indicators.ObservationStatusDefinitive:  "D",
		indicators.ObservationStatusWithdrawn:   "W",
	}
	for status, want := range cases {
		if string(status) != want {
			t.Errorf("expected %v to equal %q, got %q", status, want, string(status))
		}
	}
}
