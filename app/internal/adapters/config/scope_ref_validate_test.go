package config_test

// Remediation batch (verify-report WARNING W2): design.md line 287 says
// validate-config "checks: ... every scope.ref resolves", but Validate
// never enforced it — four dangling refs shipped in config/rupturas.yaml
// (source: aeat, source: igae, dataset: ine-ipc-subclases, dataset:
// eurostat-hicp-subclases), the same failure shape as the ECOICOP entry
// that resolves for zero configured series (verify-report WARNING W3).
// This is the RED->GREEN proof that a dangling scope.ref is now caught,
// a resolving one still passes, and the ref_status: pending escape hatch
// (mirroring date_status: unconfirmed) lets a genuinely-not-yet-
// configured reference ship honestly instead of either failing CI or
// silently passing.

import (
	"testing"
	"testing/fstest"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func TestValidate_BreakScopeRefResolution(t *testing.T) {
	tests := []struct {
		name          string
		rupturasYAML  string
		extraFiles    fstest.MapFS
		wantViolation string // "" means no scope-related violation is expected
	}{
		{
			name: "dangling source scope ref fails naming it",
			rupturasYAML: `
- id: b1
  date: 2021-01-01
  kind: methodology
  scope: { kind: source, ref: does-not-exist }
  note_md: "..."
`,
			wantViolation: "scope.ref",
		},
		{
			name: "resolving source scope ref passes",
			rupturasYAML: `
- id: b1
  date: 2021-01-01
  kind: methodology
  scope: { kind: source, ref: ine }
  note_md: "..."
`,
			extraFiles: fstest.MapFS{"sources/ine.yaml": &fstest.MapFile{Data: []byte(completeSourceYAML())}},
		},
		{
			name: "dangling dataset scope ref fails naming it",
			rupturasYAML: `
- id: b1
  date: 2021-01-01
  kind: methodology
  scope: { kind: dataset, ref: does-not-exist }
  note_md: "..."
`,
			wantViolation: "scope.ref",
		},
		{
			name: "resolving dataset scope ref passes",
			rupturasYAML: `
- id: b1
  date: 2021-01-01
  kind: methodology
  scope: { kind: dataset, ref: ine-epa }
  note_md: "..."
`,
			extraFiles: fstest.MapFS{
				"sources/ine.yaml":             &fstest.MapFile{Data: []byte(completeSourceYAML())},
				"series/tasa-de-paro-epa.yaml": &fstest.MapFile{Data: []byte(completeSeriesYAML())},
			},
		},
		{
			name: "dangling series scope ref fails naming it",
			rupturasYAML: `
- id: b1
  date: 2021-01-01
  kind: methodology
  scope: { kind: series, ref: does-not-exist }
  note_md: "..."
`,
			wantViolation: "scope.ref",
		},
		{
			name: "resolving series scope ref passes",
			rupturasYAML: `
- id: b1
  date: 2021-01-01
  kind: methodology
  scope: { kind: series, ref: tasa-de-paro-epa }
  note_md: "..."
`,
			extraFiles: fstest.MapFS{
				"sources/ine.yaml":             &fstest.MapFile{Data: []byte(completeSourceYAML())},
				"series/tasa-de-paro-epa.yaml": &fstest.MapFile{Data: []byte(completeSeriesYAML())},
			},
		},
		{
			name: "dangling ref marked ref_status pending with todo passes",
			rupturasYAML: `
- id: b1
  date: 2021-01-01
  kind: methodology
  scope: { kind: dataset, ref: does-not-exist-yet, ref_status: pending }
  todo: "Configure the ine-ipc-subclases dataset in a future phase."
  note_md: "..."
`,
		},
		{
			name: "ref_status pending without todo fails naming it",
			rupturasYAML: `
- id: b1
  date: 2021-01-01
  kind: methodology
  scope: { kind: dataset, ref: does-not-exist-yet, ref_status: pending }
  note_md: "..."
`,
			wantViolation: "todo",
		},
		{
			name: "invalid ref_status value fails naming it",
			rupturasYAML: `
- id: b1
  date: 2021-01-01
  kind: methodology
  scope: { kind: dataset, ref: ine-epa, ref_status: bogus }
  note_md: "..."
`,
			extraFiles: fstest.MapFS{
				"sources/ine.yaml":             &fstest.MapFile{Data: []byte(completeSourceYAML())},
				"series/tasa-de-paro-epa.yaml": &fstest.MapFile{Data: []byte(completeSeriesYAML())},
			},
			wantViolation: "scope.ref_status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{"rupturas.yaml": &fstest.MapFile{Data: []byte(tt.rupturasYAML)}}
			for name, f := range tt.extraFiles {
				fsys[name] = f
			}
			cfg, err := config.Load(fsys)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			violations := config.Validate(cfg)

			gotScopeViolation := hasViolation(violations, "rupturas.yaml", "scope.ref") ||
				hasViolation(violations, "rupturas.yaml", "scope.ref_status") ||
				hasViolation(violations, "rupturas.yaml", "todo")

			if tt.wantViolation == "" {
				if gotScopeViolation {
					t.Fatalf("expected no scope-ref-related violation, got %v", violations)
				}
				return
			}
			if !hasViolation(violations, "rupturas.yaml", tt.wantViolation) {
				t.Fatalf("expected a violation naming rupturas.yaml + %s, got %v", tt.wantViolation, violations)
			}
		})
	}
}
