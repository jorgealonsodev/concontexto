package xlsx

// Task 8.5/8.9: the header fingerprint validation.Rule1Schema compares
// (indicators.ObservedSchema.HeaderFingerprint vs
// config.XLSXSchemaConfig.HeaderFingerprint, already wired in PR 4a) must
// be STABLE across two runs of the exact same, unchanged workbook. Task
// 8.1's verified trap #3: header cells carry a trailing newline
// ("REGIMEN GENERAL\n", "Régimen General (1) \n"). Without normalising
// it away, the fingerprint would never be reproducible and every
// unchanged run would falsely alert as schema drift (spec
// source-ingestion-xlsx, "Header fingerprinting MUST normalise line
// endings and trim trailing whitespace, so that an unchanged workbook
// never reports a schema change").

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// normalizeHeaderCell trims and collapses internal whitespace exactly
// like collapseInternalWhitespace, which also normalises the CRLF/LF
// trailing-newline trap: strings.Fields splits on any run of Unicode
// whitespace, \r and \n included, so a trailing "\n" or "\r\n" simply
// disappears along with any other trailing space.
func normalizeHeaderCell(raw string) string {
	return collapseInternalWhitespace(raw)
}

// computeFingerprint hashes the normalised header cells, in the given
// order, into a stable hex digest. Order matters (it is part of the
// declared shape), so the caller passes cells in a deterministic order
// (period anchor, then total column, then every component column, in
// config declaration order) rather than a set.
func computeFingerprint(headerCells []string) string {
	normalized := make([]string, len(headerCells))
	for i, c := range headerCells {
		normalized[i] = normalizeHeaderCell(c)
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "|")))
	return hex.EncodeToString(sum[:])
}
