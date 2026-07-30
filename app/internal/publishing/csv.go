package publishing

// Task 4.3/4.4: the CSV projection under /data-derived/csv/{slug}.csv
// (design's supporting decisions table, "one file per series:
// period,value,status,source_status,version + header comment with
// licence/attribution -- attribution travels with the file (D2
// licensing)"). BuildSeriesCSV is a pure function of the exact SeriesDoc
// Export already built for the JSON side -- spec publishing-export's "CSV
// and artifact agree value by value" is true by construction: there is
// only one in-memory model, projected into two on-disk shapes, never two
// independently-derived views that could silently diverge.

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// BuildSeriesCSV renders doc's header comment (attribution, then licence
// name and URL -- both taken verbatim from config-derived fields, never
// invented prose) followed by the column header and one row per point, in
// doc.Points' existing order (buildSeriesDoc already sorts by period).
// Withdrawn periods are intentionally absent -- the CSV mirrors
// SeriesDoc.Points exactly, the same "current published data only"
// contract the JSON side carries (spec "Only published data enters the
// artifact").
func BuildSeriesCSV(doc SeriesDoc) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", doc.Source.Attribution)
	fmt.Fprintf(&b, "# %s (%s)\n", doc.Source.LicenceName, doc.Source.LicenceURL)
	b.WriteString("period,value,status,source_status,version\n")
	for _, p := range doc.Points {
		sourceStatus := doc.SourceStatus[p.Period]
		fmt.Fprintf(&b, "%s,%s,%s,%s,%d\n", p.Period, strconv.FormatFloat(p.Value, 'f', -1, 64), p.Status, sourceStatus, p.Version)
	}
	return []byte(b.String())
}

// writeSeriesCSV writes doc's CSV projection under outDir/csv/{slug}.csv,
// atomically (writeFileAtomic -- the same temp-file-plus-rename swap
// Export already uses for series/{slug}.json and manifest.json).
func writeSeriesCSV(outDir string, doc SeriesDoc) error {
	path := filepath.Join(outDir, "csv", doc.Slug+".csv")
	return writeFileAtomic(path, BuildSeriesCSV(doc))
}
