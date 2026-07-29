// Package filestore is the disk-only half of the raw-file archive (spec
// raw-file-archive, "Raw downloads are stored immutably keyed by
// SHA-256"). It knows nothing about the database, sources, URLs or
// timestamps — it is a content-addressable store: bytes in, and the SAME
// bytes always resolve to the SAME path, keyed by their SHA-256 hex
// digest. Store never overwrites or deletes a file it has already
// written, so an identical redownload is deduplicated by construction,
// and a rollback elsewhere in the pipeline (which never calls into this
// package at all — postgres.ObservationWriter.RollbackRun only touches
// the observation table) cannot possibly touch an archived file. The
// database half (the raw_file row, source/URL/timestamp) lives in
// package postgres, which composes a Store with its own DB writes (see
// postgres.ArchiveRawFile).
package filestore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Store archives payloads on disk under basePath, content-addressed by
// their SHA-256 hex digest.
type Store struct {
	basePath string
}

// NewStore builds a Store rooted at basePath. The path is always an
// explicit constructor parameter, never read from the environment
// inside this adapter — the caller (production wiring or a test)
// decides where raw bytes live, so tests can point anywhere with
// t.TempDir() and production can point at the app_data volume mount
// without this package ever knowing an environment variable's name.
// basePath is created, including any missing parent directories, on the
// first successful Put; NewStore itself performs no I/O.
func NewStore(basePath string) *Store {
	return &Store{basePath: basePath}
}

// Stored describes one archived payload: its content hash, the path it
// was written to, its size, and whether THIS call is the one that
// actually wrote it (false when the bytes were already archived by a
// prior call — spec "An identical redownload does not duplicate
// storage").
type Stored struct {
	Hash           string
	StoragePath    string
	SizeBytes      int64
	AlreadyExisted bool
}

// Put archives payload under its SHA-256 hex digest. Calling Put twice
// with byte-identical payloads is a no-op the second time: the file is
// never rewritten, and AlreadyExisted reports true so a caller (e.g.
// postgres.ArchiveRawFile) can avoid recording a duplicate raw_file row
// too.
func (s *Store) Put(payload []byte) (Stored, error) {
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	path := filepath.Join(s.basePath, hash)

	if info, err := os.Stat(path); err == nil {
		return Stored{Hash: hash, StoragePath: path, SizeBytes: info.Size(), AlreadyExisted: true}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Stored{}, fmt.Errorf("filestore: checking %s: %w", path, err)
	}

	if err := os.MkdirAll(s.basePath, 0o755); err != nil {
		return Stored{}, fmt.Errorf("filestore: creating %s: %w", s.basePath, err)
	}

	tmp, err := os.CreateTemp(s.basePath, "tmp-*")
	if err != nil {
		return Stored{}, fmt.Errorf("filestore: creating a temp file in %s: %w", s.basePath, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return Stored{}, fmt.Errorf("filestore: writing %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return Stored{}, fmt.Errorf("filestore: syncing %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return Stored{}, fmt.Errorf("filestore: closing %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		// A concurrent Put for the identical content may have already
		// renamed its own temp file into place first; that is success,
		// not a race to report — the content-addressed path is either
		// there with our bytes or it isn't there at all.
		if info, statErr := os.Stat(path); statErr == nil {
			return Stored{Hash: hash, StoragePath: path, SizeBytes: info.Size(), AlreadyExisted: true}, nil
		}
		return Stored{}, fmt.Errorf("filestore: archiving %s: %w", path, err)
	}

	return Stored{Hash: hash, StoragePath: path, SizeBytes: int64(len(payload)), AlreadyExisted: false}, nil
}

// Read returns the archived bytes for hash — e.g. to recompute and
// independently verify a listed hash (spec raw-file-archive,
// "recomputing the SHA-256 of the archived bytes reproduces the listed
// hash").
func (s *Store) Read(hash string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(s.basePath, hash))
	if err != nil {
		return nil, fmt.Errorf("filestore: reading %s: %w", hash, err)
	}
	return data, nil
}
