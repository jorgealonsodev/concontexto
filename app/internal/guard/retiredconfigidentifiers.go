package guard

// Task 5b.2/5b.3: scanConfigForRetiredIdentifiers backs
// TestNoRetiredIdentifiersInEmbeddedConfig -- see that file's doc
// comment for the full rationale. This scans YAML CONTENT (not Go
// source), so it lives beside, not inside, originidentifiers_test.go's
// Go-literal scanner.

import (
	"io/fs"
	"regexp"
	"strings"
)

// retiredConfigIdentifierPatterns is the exact set spec source-
// ingestion-ine's "No retired identifier remains in configuration" and
// spec source-ingestion-eurostat's "Discontinued dataset codes are
// absent from configuration" scenarios name: table 4247, table 50902,
// operation 72, code CP (INE), plus prc_hicp_manr, prc_hicp_midx and the
// dead v1 dimension name "coicop" (Eurostat, task 6.4). Each pattern is
// word-bounded so a substring occurrence inside an unrelated real
// identifier -- e.g. CNTR6721's digits contain "72", and "coicop18" (the
// LIVE dimension name) contains "coicop" -- is never a false positive
// (the same exact-match discipline originidentifiers_test.go's
// isForbidden already applies to Go literals).
var retiredConfigIdentifierPatterns = map[string]*regexp.Regexp{
	"table 4247":            regexp.MustCompile(`\b4247\b`),
	"table 50902":           regexp.MustCompile(`\b50902\b`),
	"operation 72":          regexp.MustCompile(`\b72\b`),
	"code CP":               regexp.MustCompile(`\bCP\b`),
	"dataset prc_hicp_manr": regexp.MustCompile(`\bprc_hicp_manr\b`),
	"dataset prc_hicp_midx": regexp.MustCompile(`\bprc_hicp_midx\b`),
	"dimension name coicop": regexp.MustCompile(`\bcoicop\b`),
}

// scanConfigForRetiredIdentifiers walks fsys (an already fs.Sub'd
// /config tree, the same convention config.Load's fsys parameter uses)
// and returns one violation string per retired-identifier match found in
// any *.yaml/*.yml file, formatted "path: retired identifier <name>".
func scanConfigForRetiredIdentifiers(fsys fs.FS) ([]string, error) {
	var violations []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")) {
			return nil
		}
		data, readErr := fs.ReadFile(fsys, p)
		if readErr != nil {
			return readErr
		}
		content := stripYAMLComments(data)
		for name, re := range retiredConfigIdentifierPatterns {
			if re.Match(content) {
				violations = append(violations, p+": retired identifier "+name)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

// stripYAMLComments removes every YAML comment (an unquoted "#" through
// end of line) from data, the same "comments are prose, not values"
// exemption originidentifiers_test.go's go/scanner-based Go-literal scan
// already applies -- a retired identifier explained in a doc comment
// (exactly what this batch's own config/series/*.yaml files do, to
// document WHY it must not be used) must not itself trip the guard it is
// documenting.
func stripYAMLComments(data []byte) []byte {
	var out []byte
	inSingle, inDouble := false, false
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '#' && !inSingle && !inDouble:
			// Skip to end of line, keeping the newline itself so line
			// numbers/violations stay meaningful if ever surfaced.
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i >= len(data) {
				return out
			}
			c = data[i]
		}
		out = append(out, c)
	}
	return out
}
