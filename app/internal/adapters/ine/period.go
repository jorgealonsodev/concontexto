package ine

// normalizeINEPeriod joins INE's separate Anyo (year) and T3_Periodo
// (cadence code) wire fields into the single "<code> <year>" label
// indicators.NormalizePeriodLabel already parses ("T1 2026", "M06
// 2026" -- see indicators/period.go's doc comment there for why the
// monthly INE shape carries an explicit year), and normalises it into
// the canonical indicators.Period. Normalisation itself is not
// reinvented here: indicators.Period already owns the parser (built in
// PR 4a), so this function's whole job is the join (spec
// source-ingestion-ine, "INE period labels ... MUST be normalised to
// the canonical domain period before validation, so that continuity
// checks are source-independent").

import (
	"fmt"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func normalizeINEPeriod(anyo int, periodo string) (indicators.Period, error) {
	// The population series' (ECP) quarter-start-date phrase ("1 de
	// enero de", ...) is translated to the same "T<n>" code EPA/CNTR
	// already use BEFORE joining, so this function has exactly one join
	// path regardless of which of INE's two quarterly period-label
	// shapes T3_Periodo carries (periodicity.go's quarterStartLabels;
	// verified live 2026-07-28 against DATOS_SERIE/ECP320).
	if ordinal, ok := quarterStartLabels[periodo]; ok {
		periodo = fmt.Sprintf("T%d", ordinal)
	}
	label := fmt.Sprintf("%s %d", periodo, anyo)
	period, err := indicators.NormalizePeriodLabel(label)
	if err != nil {
		return indicators.Period{}, fmt.Errorf("normalizing INE period Anyo=%d T3_Periodo=%q: %w", anyo, periodo, err)
	}
	return period, nil
}
