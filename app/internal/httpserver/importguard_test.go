package httpserver_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestImportGraph_HttpserverNeverImportsPostgresOrPgx makes the golden
// rule (PRD §14.2 — zero DB queries and zero computation at request
// time) falsifiable by the import graph, not a convention (design.md).
// If httpserver ever transitively imports the postgres adapter, the pgx
// driver, or a source-client adapter, this test fails and CI blocks the
// merge (task 1.7 wires it as a CI step).
//
// Remediation batch (verify-report WARNING W1): the forbidden-prefix
// list originally covered only the postgres/pgx half of design.md's
// golden-rule paragraph. The other half — PRD §9.2, "no external source
// is ever called at page-request time" — was true today (`go list
// -deps` verified) but unguarded: nothing here stopped a future handler
// from importing adapters/ine, adapters/eurostat or adapters/xlsx. Adding
// them closes that gap the same way the postgres/pgx prefixes already do.
func TestImportGraph_HttpserverNeverImportsPostgresOrPgx(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/jorgealonsodev/concontexto/app/internal/httpserver").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}

	deps := strings.Fields(string(out))
	forbiddenPrefixes := []string{
		"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres",
		"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine",
		"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat",
		"github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx",
		"github.com/jackc/pgx",
	}

	for _, dep := range deps {
		for _, forbidden := range forbiddenPrefixes {
			if strings.HasPrefix(dep, forbidden) {
				t.Fatalf("golden rule violated: httpserver transitively imports %q (forbidden prefix %q)", dep, forbidden)
			}
		}
	}
}
