package configdata_test

// Task 3.5 (RED) / 3.6 (GREEN): the embedded fs.FS serves /config, and
// mutating the config files on disk after the binary containing them was
// compiled does NOT change what the running process reads (spec
// editorial-config, "Runtime configuration cannot drift from the
// binary"). ADR-1's whole point is pinning config version to binary
// version atomically.
//
// Proof shape: this test BINARY (the `go test` binary for this package)
// already finished compiling — and therefore already captured
// config/embed-marker.txt's bytes into configdata.FS — before this test
// function starts running. Mutating the on-disk marker file DURING the
// test and re-reading configdata.FS is a real, falsifiable proof: if
// go:embed captured a live filesystem reference instead of compiled-in
// bytes, the second read would observe the mutation. It doesn't, because
// go:embed doesn't work that way — that is exactly the property ADR-1
// depends on for "a git revert of a bad editorial edit reverts data and
// code together".

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	configdata "github.com/jorgealonsodev/concontexto"
)

func TestEmbeddedFS_UnaffectedByOnDiskMutationAfterCompile(t *testing.T) {
	before, err := configdata.FS.ReadFile("config/embed-marker.txt")
	if err != nil {
		t.Fatalf("reading embedded marker: %v", err)
	}

	markerPath := filepath.Join(repoRoot(t), "config", "embed-marker.txt")
	original, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("reading on-disk marker: %v", err)
	}
	if string(original) != string(before) {
		t.Fatalf("test fixture drift: embedded marker %q != on-disk marker %q before any mutation — the embed and the disk file should start identical", before, original)
	}
	t.Cleanup(func() {
		if err := os.WriteFile(markerPath, original, 0o644); err != nil {
			t.Fatalf("restoring on-disk marker after mutation: %v", err)
		}
	})

	if err := os.WriteFile(markerPath, []byte("MUTATED-AFTER-COMPILE\n"), 0o644); err != nil {
		t.Fatalf("mutating on-disk marker: %v", err)
	}

	after, err := configdata.FS.ReadFile("config/embed-marker.txt")
	if err != nil {
		t.Fatalf("re-reading embedded marker: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("embedded config changed after an on-disk mutation: got %q, want unchanged %q — go:embed must pin content at compile time, not read the live filesystem", after, before)
	}

	// Confirm the mutation genuinely happened on disk (so the assertion
	// above is not vacuously true because the write silently failed).
	mutatedOnDisk, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("reading mutated on-disk marker: %v", err)
	}
	if string(mutatedOnDisk) == string(before) {
		t.Fatal("test bug: on-disk mutation did not actually happen, so this test proves nothing")
	}
}

// TestEmbeddedFS_ServesConfigTree is the minimal "the embedded fs.FS
// serves /config" half of the requirement: a well-known file under
// /config is reachable through configdata.FS with the "config/" prefix
// (the //go:embed pattern is "config", not "config/*", so paths keep
// the directory name — this is also why app/internal/adapters/config's
// Load takes an already fs.Sub'd tree, see loader.go).
func TestEmbeddedFS_ServesConfigTree(t *testing.T) {
	entries, err := configdata.FS.ReadDir("config")
	if err != nil {
		t.Fatalf("reading embedded config/ directory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry under embedded config/, got none")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// This file lives at the repository root, alongside config_embed.go.
	return filepath.Dir(thisFile)
}
