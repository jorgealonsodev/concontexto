package main

// Remediation batch (verify-report WARNING W12 -- the sixth instance of
// the "parsed and ignored" pattern): config.APIConfig.MaxResponseBytes
// is set to 8388608 in every source YAML and validated at
// validate-config time, but before this batch buildSourceClient built
// every eurostat/xlsx client with NO options, so the value editorial
// config declares was silently discarded and each client instead fell
// back to its OWN unconfigurable hardcoded defaultMaxResponseBytes
// constant. Behaviour happened to be correct only because the constant
// and the configured value coincide (8 MiB in both places) -- editing
// the YAML would have zero effect.
//
// These tests prove the configured value is genuinely threaded through
// by using a ceiling that DIFFERS from the hardcoded default: a response
// body larger than the small configured ceiling but comfortably smaller
// than the 8 MiB default. If buildSourceClient ever regresses to
// ignoring config.APIConfig.MaxResponseBytes, the response fits under
// the default and FetchRaw succeeds instead of failing --
// sourceerr.ResponseTooLarge never fires, and these tests fail.
//
// Remediation batch 4: adapters/ine had NO response-ceiling mechanism
// at all (no WithMaxResponseBytes option, no maxResponseBytes field --
// its doRequest called io.ReadAll(resp.Body) unbounded) despite
// ine.yaml declaring the identical max_response_bytes: 8388608 key.
// That gap is closed in this batch: adapters/ine now carries the same
// WithMaxResponseBytes/readWithCeiling mechanism as adapters/eurostat
// and adapters/xlsx, and buildSourceClient wires it here exactly like
// the other two.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func TestBuildSourceClient_INEUsesTheConfiguredMaxResponseBytesNotTheHardcodedDefault(t *testing.T) {
	// 300 bytes, comfortably under ine's 8 MiB default but well over the
	// 100-byte ceiling configured below -- proves the CONFIGURED value
	// is consulted, not merely that some ceiling exists.
	const configuredCeiling = 100
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes.Repeat([]byte("x"), 300))
	}))
	defer server.Close()

	src := config.SourceConfig{ID: "ine", API: &config.APIConfig{BaseURL: server.URL, MaxResponseBytes: configuredCeiling}}
	s := config.SeriesConfig{Slug: "test-series"}
	ref := config.SourceRef{Kind: "ine-series-cod", Ref: "any-cod"}

	client, err := buildSourceClient(src, s, ref)
	if err != nil {
		t.Fatalf("buildSourceClient: %v", err)
	}

	_, err = client.FetchRaw(context.Background(), ref.Ref)
	if err == nil {
		t.Fatal("expected FetchRaw to fail with a response-too-large error using the configured 100-byte ceiling; it succeeded, meaning the hardcoded default was used instead")
	}
	if !strings.Contains(err.Error(), "100") {
		t.Errorf("expected the error to name the CONFIGURED 100-byte ceiling (not the 8 MiB default), got: %v", err)
	}
}

func TestBuildSourceClient_EurostatUsesTheConfiguredMaxResponseBytesNotTheHardcodedDefault(t *testing.T) {
	// 300 bytes, comfortably under eurostat's 8 MiB default but well
	// over the 100-byte ceiling configured below -- proves the CONFIGURED
	// value is consulted, not merely that some ceiling exists.
	const configuredCeiling = 100
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes.Repeat([]byte("x"), 300))
	}))
	defer server.Close()

	src := config.SourceConfig{ID: "eurostat", API: &config.APIConfig{BaseURL: server.URL, MaxResponseBytes: configuredCeiling}}
	s := config.SeriesConfig{Slug: "test-series"}
	ref := config.SourceRef{Kind: "eurostat-dataset", Ref: "any-dataset"}

	client, err := buildSourceClient(src, s, ref)
	if err != nil {
		t.Fatalf("buildSourceClient: %v", err)
	}

	_, err = client.FetchRaw(context.Background(), ref.Ref)
	if err == nil {
		t.Fatal("expected FetchRaw to fail with a response-too-large error using the configured 100-byte ceiling; it succeeded, meaning the hardcoded default was used instead")
	}
	if !strings.Contains(err.Error(), "100") {
		t.Errorf("expected the error to name the CONFIGURED 100-byte ceiling (not the 8 MiB default), got: %v", err)
	}
}

func TestBuildSourceClient_XLSXUsesTheConfiguredMaxResponseBytesNotTheHardcodedDefault(t *testing.T) {
	const configuredCeiling = 100
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(bytes.Repeat([]byte("x"), 300))
	}))
	defer server.Close()

	src := config.SourceConfig{ID: "seg-social", API: &config.APIConfig{MaxResponseBytes: configuredCeiling}}
	s := config.SeriesConfig{Slug: "test-series", Schema: config.SchemaConfig{XLSX: &config.XLSXSchemaConfig{SheetName: "Sheet1"}}}
	ref := config.SourceRef{Kind: "xlsx-url", Ref: server.URL}

	client, err := buildSourceClient(src, s, ref)
	if err != nil {
		t.Fatalf("buildSourceClient: %v", err)
	}

	_, err = client.FetchRaw(context.Background(), ref.Ref)
	if err == nil {
		t.Fatal("expected FetchRaw to fail with a response-too-large error using the configured 100-byte ceiling; it succeeded, meaning the hardcoded default was used instead")
	}
	if !strings.Contains(err.Error(), "100") {
		t.Errorf("expected the error to name the CONFIGURED 100-byte ceiling (not the 8 MiB default), got: %v", err)
	}
}
