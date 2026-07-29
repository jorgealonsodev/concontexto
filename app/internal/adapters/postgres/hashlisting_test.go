package postgres_test

// Task 5a.5 (RED) / 5a.6 (GREEN): the public hash listing (spec
// raw-file-archive, "Raw-file hashes are listed in the repository";
// PRD §14.2). This is the concrete answer to an accusation of
// manipulation: anyone can recompute the SHA-256 of an archived file and
// compare it against this listing.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestWriteRawFileHashListing_ListsEveryArchivedFileAndRecomputedHashMatches(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "test-source")

	store := filestore.NewStore(t.TempDir())
	payloadA := []byte("first archived payload")
	payloadB := []byte("second archived payload, different bytes")

	rfA, _, err := postgres.ArchiveRawFile(ctx, tx, store, "test-source", "https://example.test/a", payloadA, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ArchiveRawFile A: %v", err)
	}
	rfB, _, err := postgres.ArchiveRawFile(ctx, tx, store, "test-source", "https://example.test/b", payloadB, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ArchiveRawFile B: %v", err)
	}

	var buf bytes.Buffer
	if err := postgres.WriteRawFileHashListing(ctx, tx, &buf); err != nil {
		t.Fatalf("WriteRawFileHashListing: %v", err)
	}
	listing := buf.String()

	for _, rf := range []postgres.RawFile{rfA, rfB} {
		if !strings.Contains(listing, rf.Hash) {
			t.Fatalf("expected the listing to contain hash %s, got:\n%s", rf.Hash, listing)
		}
		if !strings.Contains(listing, "test-source") {
			t.Fatalf("expected the listing to contain the source id, got:\n%s", listing)
		}

		onDisk, err := os.ReadFile(rf.StoragePath)
		if err != nil {
			t.Fatalf("reading archived file %s: %v", rf.StoragePath, err)
		}
		sum := sha256.Sum256(onDisk)
		if hex.EncodeToString(sum[:]) != rf.Hash {
			t.Fatalf("recomputed hash of the archived bytes does not match the listed hash for %s", rf.Hash)
		}
	}

	lines := strings.Split(strings.TrimRight(listing, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected exactly 2 listing lines (one per archived file), got %d:\n%s", len(lines), listing)
	}
}

func TestPublishRawFileHashListing_WritesBothTheArchiveAndPublicCopies(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "test-source")

	store := filestore.NewStore(t.TempDir())
	payload := []byte("published listing payload")
	rf, _, err := postgres.ArchiveRawFile(ctx, tx, store, "test-source", "https://example.test/data", payload, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ArchiveRawFile: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "app_data", "raw_files.sha256")
	publicPath := filepath.Join(t.TempDir(), "public", "transparencia", "raw-files.sha256")

	if err := postgres.PublishRawFileHashListing(ctx, tx, archivePath, publicPath); err != nil {
		t.Fatalf("PublishRawFileHashListing: %v", err)
	}

	archived, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("reading the app_data copy: %v", err)
	}
	published, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatalf("reading the /public copy: %v", err)
	}
	if string(archived) != string(published) {
		t.Fatal("expected the app_data and /public copies to be byte-identical")
	}
	if !strings.Contains(string(published), rf.Hash) {
		t.Fatalf("expected the published listing to contain %s, got:\n%s", rf.Hash, string(published))
	}
}

// TestPublishRawFileHashListing_ARequestAlreadyReadingThePublicCopyIsUnaffectedByALaterPublish
// covers verify-report WARNING W11 with a DETERMINISTIC proof (not a
// timing-dependent race, which would be flaky for a listing this small):
// before this batch, publicPath was written via a truncate-then-write
// (os.WriteFile), which reuses the EXISTING inode when the file already
// exists. httpserver serves that exact path directly from disk via
// http.FileServer/os.File; a request already reading the file when a
// new publish lands would therefore observe the NEW, in-progress
// content mixed with its own read position -- exactly the truncated/
// torn read W11 describes. C4 made this reachable in production by
// putting ingestion and serving in the same process for the first time.
//
// The fix writes to a temp file in publicPath's own directory and
// os.Rename's it into place. POSIX rename() is atomic AND never affects
// a file descriptor opened before the rename: that descriptor keeps
// pointing at the OLD (now unlinked, but still valid) inode until
// closed. This test exploits exactly that guarantee, deterministically:
// it opens publicPath for reading BEFORE a second publish, republishes
// with different content, and asserts the ALREADY-OPEN handle still
// reads the FIRST listing byte-for-byte -- the exact contract an
// in-flight http.FileServer response depends on.
func TestPublishRawFileHashListing_ARequestAlreadyReadingThePublicCopyIsUnaffectedByALaterPublish(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSource(t, ctx, tx, "test-source")
	store := filestore.NewStore(t.TempDir())

	archivePath := filepath.Join(t.TempDir(), "app_data", "raw_files.sha256")
	publicPath := filepath.Join(t.TempDir(), "public", "transparencia", "raw-files.sha256")

	rfA, _, err := postgres.ArchiveRawFile(ctx, tx, store, "test-source", "https://example.test/a", []byte("payload A"), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ArchiveRawFile A: %v", err)
	}
	if err := postgres.PublishRawFileHashListing(ctx, tx, archivePath, publicPath); err != nil {
		t.Fatalf("PublishRawFileHashListing (first publish): %v", err)
	}

	// Simulates an in-flight read of publicPath (an http.FileServer
	// response already streaming this exact file) started BEFORE the
	// second publish below.
	reader, err := os.Open(publicPath)
	if err != nil {
		t.Fatalf("opening publicPath before the second publish: %v", err)
	}
	defer reader.Close()

	rfB, _, err := postgres.ArchiveRawFile(ctx, tx, store, "test-source", "https://example.test/b", []byte("payload B, a deliberately different length"), time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ArchiveRawFile B: %v", err)
	}
	if err := postgres.PublishRawFileHashListing(ctx, tx, archivePath, publicPath); err != nil {
		t.Fatalf("PublishRawFileHashListing (second publish): %v", err)
	}

	observed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading from the handle opened before the second publish: %v", err)
	}

	if !strings.Contains(string(observed), rfA.Hash) {
		t.Fatalf("expected the pre-opened reader to still observe the FIRST listing (containing %s), got:\n%s", rfA.Hash, string(observed))
	}
	if strings.Contains(string(observed), rfB.Hash) {
		t.Fatalf("the pre-opened reader observed the SECOND publish's content (%s) -- publicPath was mutated in place instead of atomically replaced, exactly the W11 defect", rfB.Hash)
	}
}
