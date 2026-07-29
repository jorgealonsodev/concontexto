package validation_test

// Task 4.3 (RED): rule 1 (schema). Spec data-validation, "Rule 1 —
// schema": expected fields present with correct types before
// normalisation; for XLSX sources the check also covers sheet name,
// header row index, column anchors and the header fingerprint.
//
// Series/source codes in these fixtures are deliberately synthetic
// (TESTCOD001-style) per the origin-identifier guard (task 3.3/3.4) —
// none of these values are real INE/Eurostat identifiers.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

func TestRule1Schema_RenamedValueColumnFailsNamingIt(t *testing.T) {
	ctx := validation.SeriesContext{
		Schema: config.SchemaConfig{ExpectedFields: []string{"period", "value"}},
		ObservedSchema: indicators.ObservedSchema{
			Fields: []string{"period", "valor"}, // "value" renamed to "valor"
		},
	}

	findings := validation.Rule1Schema(ctx, nil)

	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != validation.SeverityBlock {
		t.Errorf("expected SeverityBlock, got %v", findings[0].Severity)
	}
	if !containsSubstring(findings[0].Message, `"value"`) {
		t.Errorf("expected the failure to name the missing field %q, got message %q", "value", findings[0].Message)
	}
}

func TestRule1Schema_AllExpectedFieldsPresentPasses(t *testing.T) {
	ctx := validation.SeriesContext{
		Schema: config.SchemaConfig{ExpectedFields: []string{"period", "value"}},
		ObservedSchema: indicators.ObservedSchema{
			Fields: []string{"period", "value"},
		},
	}

	findings := validation.Rule1Schema(ctx, nil)

	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestRule1Schema_XLSXHeaderFingerprintMismatchFails(t *testing.T) {
	ctx := validation.SeriesContext{
		Schema: config.SchemaConfig{
			XLSX: &config.XLSXSchemaConfig{
				SheetName:         "TESTCOD001-sheet",
				HeaderRow:         5,
				HeaderFingerprint: "abc123",
				ColumnAnchors:     map[string]string{"value": "C"},
			},
		},
		ObservedSchema: indicators.ObservedSchema{
			SheetName:         "TESTCOD001-sheet",
			HeaderRow:         5,
			HeaderFingerprint: "def456", // header row changed shape
			ColumnAnchors:     map[string]string{"value": "C"},
		},
	}

	findings := validation.Rule1Schema(ctx, nil)

	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding, got %d: %+v", len(findings), findings)
	}
	if !containsSubstring(findings[0].Message, "fingerprint") {
		t.Errorf("expected the failure to mention the header fingerprint, got message %q", findings[0].Message)
	}
}

func TestRule1Schema_XLSXColumnAnchorMovedFailsNamingIt(t *testing.T) {
	ctx := validation.SeriesContext{
		Schema: config.SchemaConfig{
			XLSX: &config.XLSXSchemaConfig{
				ColumnAnchors: map[string]string{"total": "F"},
			},
		},
		ObservedSchema: indicators.ObservedSchema{
			ColumnAnchors: map[string]string{"total": "G"}, // column shifted
		},
	}

	findings := validation.Rule1Schema(ctx, nil)

	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding, got %d: %+v", len(findings), findings)
	}
	if !containsSubstring(findings[0].Message, `"total"`) {
		t.Errorf("expected the failure to name the moved column %q, got message %q", "total", findings[0].Message)
	}
}

func TestRule1Schema_MatchingXLSXSchemaPasses(t *testing.T) {
	ctx := validation.SeriesContext{
		Schema: config.SchemaConfig{
			ExpectedFields: []string{"period", "value"},
			XLSX: &config.XLSXSchemaConfig{
				SheetName:         "TESTCOD001-sheet",
				HeaderRow:         5,
				HeaderFingerprint: "abc123",
				ColumnAnchors:     map[string]string{"value": "C"},
			},
		},
		ObservedSchema: indicators.ObservedSchema{
			Fields:            []string{"period", "value"},
			SheetName:         "TESTCOD001-sheet",
			HeaderRow:         5,
			HeaderFingerprint: "abc123",
			ColumnAnchors:     map[string]string{"value": "C"},
		},
	}

	findings := validation.Rule1Schema(ctx, nil)

	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
