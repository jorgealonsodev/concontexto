package xlsx

// Task 8.5/8.9: the Social Security affiliation workbook's period column
// carries a source-specific label shape indicators.NormalizePeriodLabel
// does not recognise: "<Spanish month name> <year>" ("Enero 2001",
// "Junio 2026" -- verified live 2026-07-28, task 8.1's resolution). This
// is deliberately its own small parser in the xlsx package, not an
// addition to indicators/period.go: it is genuinely a new label SHAPE
// (a month NAME, not a "<code> <year>" pair like INE's "T1 2026"), and
// keeping it here confines the blast radius of a new, adapter-specific
// parsing rule to the one package that needs it -- indicators/period.go
// stays untouched and its own extensive existing coverage stays exactly
// as it was.
//
// Task 8.1's verified trap #2: twelve rows carry a double space in the
// period label ("Febrero  2001" through "Febrero  2006"). Spec
// source-ingestion-xlsx, "Period labels ... are normalised before
// matching": internal whitespace MUST be collapsed and the label trimmed
// before the month name is matched.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// spanishMonths maps every lowercase Spanish month name to its 1-12
// ordinal. The workbook's own labels are title-cased ("Enero", "Junio");
// matching is done case-insensitively so a publisher casing change alone
// would not break parsing.
var spanishMonths = map[string]int{
	"enero": 1, "febrero": 2, "marzo": 3, "abril": 4, "mayo": 5, "junio": 6,
	"julio": 7, "agosto": 8, "septiembre": 9, "octubre": 10, "noviembre": 11, "diciembre": 12,
}

// reSpanishMonthYear matches "<month name> <year>" AFTER whitespace has
// already been collapsed by collapseInternalWhitespace -- it never sees
// the raw double-spaced label itself.
var reSpanishMonthYear = regexp.MustCompile(`^(\p{L}+) (\d{4})$`)

// footnoteRe matches the Social Security workbook's footnote rows, which
// begin immediately after the last data row and mark the terminator: "(1)
// No incluye ...", "(2) Vigente desde ...". Parsing MUST stop the moment
// column A stops matching a period label AND starts matching this shape
// (spec source-ingestion-xlsx's own verified terminator: "footnote rows
// begin at row 310, matching ^\(\d+\) in column A").
var footnoteRe = regexp.MustCompile(`^\(\d+\)`)

// collapseInternalWhitespace trims raw and collapses every internal run
// of whitespace to a single space (spec's own trap #2: "Febrero  2001"
// with a double space MUST still parse as February 2001).
func collapseInternalWhitespace(raw string) string {
	return strings.Join(strings.Fields(raw), " ")
}

// isFootnoteRow reports whether raw (already trimmed) is one of the
// workbook's trailing footnote rows -- the parsing terminator.
func isFootnoteRow(raw string) bool {
	return footnoteRe.MatchString(strings.TrimSpace(raw))
}

// parsePeriodoLabel parses one Social Security "Periodo" cell ("Enero
// 2001", "Febrero  2001" with the verified double space, "Junio 2026")
// into the canonical monthly indicators.Period. An unrecognised shape
// (not a footnote either -- the caller checks that first) is reported by
// name, never silently skipped or guessed (spec source-ingestion-xlsx,
// "The parser targets underlying data ... if the requested month cannot
// be resolved ... the run fails and writes nothing").
func parsePeriodoLabel(raw string) (indicators.Period, error) {
	normalized := collapseInternalWhitespace(raw)
	m := reSpanishMonthYear.FindStringSubmatch(normalized)
	if m == nil {
		return indicators.Period{}, fmt.Errorf("unrecognised period label %q", raw)
	}
	month, ok := spanishMonths[strings.ToLower(m[1])]
	if !ok {
		return indicators.Period{}, fmt.Errorf("unrecognised Spanish month name %q in period label %q", m[1], raw)
	}
	year, err := strconv.Atoi(m[2])
	if err != nil {
		return indicators.Period{}, fmt.Errorf("parsing year in period label %q: %w", raw, err)
	}
	return indicators.Period{Frequency: indicators.FrequencyMonthly, Year: year, Ordinal: month}, nil
}
