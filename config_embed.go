// Package configdata embeds the repository's /config directory into the
// binary (ADR-1, ADR-5). go:embed patterns are resolved relative to the
// directory of the declaring file and cannot traverse upward — a module
// rooted at /app could never embed the root-level /config — so this shim
// must live at the repository root, alongside /config, while the rest of
// the Go sources stay under /app/... (PRD §14.2).
//
// config/sources/*.yaml (this package's PR 3) is the first real schema;
// config/series/*.yaml and the editorial files (rupturas.yaml,
// eventos.yaml, gobiernos.yaml) arrive in later phases.
package configdata

import "embed"

// FS is the embedded /config tree. Adapters under app/internal/adapters/config
// read from this fs.FS instead of the filesystem, so config version is
// pinned to binary version atomically (ADR-1).
//
//go:embed config
var FS embed.FS
