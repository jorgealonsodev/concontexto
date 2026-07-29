package filestore_test

// Task 5a.1 (RED): the pure filesystem half of the raw-file archive
// (spec raw-file-archive, "Raw downloads are stored immutably keyed by
// SHA-256"). No database, no network — a content-addressable store keyed
// by SHA-256, proven with t.TempDir() the same way every other
// file-operation test in this repo is (go-testing skill decision gate).

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
)

func TestPut_ArchivesUnderItsSHA256Hash(t *testing.T) {
	dir := t.TempDir()
	store := filestore.NewStore(dir)

	payload := []byte("payload bytes for a raw file archive test")
	sum := sha256.Sum256(payload)
	wantHash := hex.EncodeToString(sum[:])

	stored, err := store.Put(payload)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if stored.Hash != wantHash {
		t.Fatalf("expected hash %s, got %s", wantHash, stored.Hash)
	}
	if stored.AlreadyExisted {
		t.Fatal("expected the first Put to report AlreadyExisted=false")
	}
	if stored.SizeBytes != int64(len(payload)) {
		t.Fatalf("expected size %d, got %d", len(payload), stored.SizeBytes)
	}

	onDisk, err := os.ReadFile(filepath.Join(dir, wantHash))
	if err != nil {
		t.Fatalf("reading archived file: %v", err)
	}
	if string(onDisk) != string(payload) {
		t.Fatal("archived bytes do not match the payload")
	}
	if stored.StoragePath != filepath.Join(dir, wantHash) {
		t.Fatalf("expected storage path %s, got %s", filepath.Join(dir, wantHash), stored.StoragePath)
	}
}

func TestPut_IdenticalPayloadDoesNotDuplicateStorage(t *testing.T) {
	dir := t.TempDir()
	store := filestore.NewStore(dir)
	payload := []byte("same bytes every time")

	first, err := store.Put(payload)
	if err != nil {
		t.Fatalf("Put (1st): %v", err)
	}
	if first.AlreadyExisted {
		t.Fatal("the first Put must not report AlreadyExisted")
	}

	second, err := store.Put(payload)
	if err != nil {
		t.Fatalf("Put (2nd): %v", err)
	}
	if !second.AlreadyExisted {
		t.Fatal("an identical redownload must report AlreadyExisted=true — no second copy stored")
	}
	if second.StoragePath != first.StoragePath || second.Hash != first.Hash {
		t.Fatalf("expected the identical redownload to resolve to the same file, got %+v vs %+v", first, second)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one archived file after two Puts of identical bytes, found %d: %v", len(entries), entries)
	}
}

func TestPut_DifferentPayloadsArchiveUnderDifferentHashes(t *testing.T) {
	dir := t.TempDir()
	store := filestore.NewStore(dir)

	a, err := store.Put([]byte("payload A"))
	if err != nil {
		t.Fatalf("Put A: %v", err)
	}
	b, err := store.Put([]byte("payload B"))
	if err != nil {
		t.Fatalf("Put B: %v", err)
	}
	if a.Hash == b.Hash {
		t.Fatal("expected different payloads to hash — and archive — differently")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected two distinct archived files, found %d", len(entries))
	}
}

func TestRead_ReturnsTheArchivedBytesForIndependentVerification(t *testing.T) {
	dir := t.TempDir()
	store := filestore.NewStore(dir)
	payload := []byte("verify me")

	stored, err := store.Put(payload)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Read(stored.Hash)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatal("Read did not return the archived bytes")
	}

	sum := sha256.Sum256(got)
	if hex.EncodeToString(sum[:]) != stored.Hash {
		t.Fatal("recomputed hash of the archived bytes does not match the stored hash — the concrete verification spec raw-file-archive requires")
	}
}

func TestNewStore_CreatesBasePathOnFirstPutIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "raw")
	store := filestore.NewStore(dir)

	if _, err := store.Put([]byte("x")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("expected basePath %s to be created on first Put, err=%v", dir, err)
	}
}
