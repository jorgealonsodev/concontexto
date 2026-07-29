// Package migrations embeds the Fase 0 SQL migration files so the
// postgres adapter (app/internal/adapters/postgres) can apply them
// without touching the filesystem at runtime. Migration files cannot be
// embedded directly by the postgres adapter package because Go's
// //go:embed patterns are resolved relative to the directory of the
// declaring file and cannot traverse to a sibling directory (design.md,
// "Go package layout"; same constraint as ADR-1's config embed).
//
// Every migration in this package is applied ONLY by the explicit
// `migrate` subcommand (app/internal/migrate), never on boot.
package migrations

import "embed"

// FS holds every {NNNN_name.up.sql, NNNN_name.down.sql} pair in this
// directory. File names carry the ordering: migrations are applied in
// ascending lexical order of their numeric prefix.
//
//go:embed *.sql
var FS embed.FS
