package xlsx_test

// Companion to adapters/ine's own useragent_test.go, which carries the
// full evidence for why an explicit User-Agent is load-bearing (INE's
// edge blackholes Go's default "Go-http-client/1.1" token outright --
// see app/internal/useragent for the measured table).
//
// This adapter fetches a published workbook straight off a source's web
// server rather than an API. That makes it MORE exposed to the class of
// failure INE demonstrated, not less: a plain web server sits behind
// whatever generic bot/WAF filtering its operator turned on, and those
// filters are exactly what score a default library User-Agent. The
// assertion is on the header AS RECEIVED by the server, through a real
// request the Client issues.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx"
	"github.com/jorgealonsodev/concontexto/app/internal/useragent"
)

func TestFetchRaw_SendsTheSharedUserAgentOnTheWire(t *testing.T) {
	var got string
	var seen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		seen = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("workbook-bytes"))
	}))
	defer srv.Close()

	// FetchRaw archives the response bytes without parsing them, so an
	// empty schema is sufficient here -- decoding is decode_test.go's job.
	client := xlsx.NewClient(config.XLSXSchemaConfig{}, srv.Client())
	if _, err := client.FetchRaw(context.Background(), srv.URL+"/workbook.xlsx"); err != nil {
		t.Fatalf("FetchRaw against a healthy stub returned an error: %v", err)
	}

	if !seen {
		t.Fatal("the stub server was never called, so no header was observed")
	}
	if got != useragent.UserAgent {
		t.Fatalf("User-Agent on the wire = %q, want %q", got, useragent.UserAgent)
	}
}
