package postgres

// Task 5a.5/5a.6 (GREEN): the public hash listing (spec raw-file-archive,
// "Raw-file hashes are listed in the repository"; PRD §14.2). This is
// the concrete answer to an accusation of manipulation: anyone can
// recompute the SHA-256 of an archived file and compare it against this
// listing. design.md: "filestore appends app_data/raw_files.sha256,
// copied at ingest to /public/transparencia/raw-files.sha256, served
// statically" -- both paths are always explicit parameters here, never
// read from the environment inside this adapter (same constraint as
// filestore.Store's basePath), so production wiring (the real app_data
// and STATIC_ROOT-relative paths) is the ingest orchestrator's job, not
// this file's -- that orchestrator does not exist yet (phase 5b/9).

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// hashListingEntry is one line of the published listing.
type hashListingEntry struct {
	Hash         string
	SourceID     string
	DownloadedAt time.Time
	URL          string
}

// WriteRawFileHashListing writes every archived raw_file's hash, source
// and download timestamp to w, one entry per line, sorted by hash for a
// deterministic, diffable listing (spec "every archived file appears in
// the listing with its SHA-256, source and download timestamp").
func WriteRawFileHashListing(ctx context.Context, db DBTX, w io.Writer) error {
	rows, err := db.Query(ctx, `SELECT hash, source_id, downloaded_at, url FROM raw_file`)
	if err != nil {
		return fmt.Errorf("postgres: querying raw_file for the hash listing: %w", err)
	}
	defer rows.Close()

	var entries []hashListingEntry
	for rows.Next() {
		var e hashListingEntry
		if err := rows.Scan(&e.Hash, &e.SourceID, &e.DownloadedAt, &e.URL); err != nil {
			return fmt.Errorf("postgres: scanning a raw_file row for the hash listing: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("postgres: iterating raw_file for the hash listing: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Hash < entries[j].Hash })

	for _, e := range entries {
		if _, err := fmt.Fprintf(w, "%s  %s  %s  %s\n", e.Hash, e.SourceID, e.DownloadedAt.UTC().Format(time.RFC3339), e.URL); err != nil {
			return fmt.Errorf("postgres: writing hash listing entry for %s: %w", e.Hash, err)
		}
	}
	return nil
}

// PublishRawFileHashListing writes the listing to archivePath (the
// app_data-rooted copy) and then copies the same bytes to publicPath
// (the statically-served copy design.md describes) -- "refreshed as
// part of the ingestion pipeline" (spec) means calling this after every
// run that may have archived a new file, not on a fixed schedule. Both
// paths' parent directories are created if missing.
//
// Remediation batch (verify-report WARNING W11): publicPath is written
// via writeFileAtomically, never a truncate-then-write, because
// httpserver serves this exact path directly from disk
// (http.FileServer/os.File) and -- since CRITICAL C4 put ingestion and
// serving in the SAME process -- a request already reading publicPath
// when a new publish lands must keep observing the listing it opened,
// never a torn mix of old and new bytes (PRD §14.2: this file's entire
// purpose is letting a reader verify archive integrity). archivePath is
// not served by anything and keeps its simple os.Create; only publicPath
// needs the atomic-replace guarantee.
func PublishRawFileHashListing(ctx context.Context, db DBTX, archivePath, publicPath string) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return fmt.Errorf("postgres: creating the directory for %s: %w", archivePath, err)
	}
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("postgres: creating the hash listing at %s: %w", archivePath, err)
	}
	if err := WriteRawFileHashListing(ctx, db, archiveFile); err != nil {
		archiveFile.Close()
		return err
	}
	if err := archiveFile.Close(); err != nil {
		return fmt.Errorf("postgres: closing the hash listing at %s: %w", archivePath, err)
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		return fmt.Errorf("postgres: reading back the hash listing at %s: %w", archivePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(publicPath), 0o755); err != nil {
		return fmt.Errorf("postgres: creating the directory for %s: %w", publicPath, err)
	}
	if err := writeFileAtomically(publicPath, data, 0o644); err != nil {
		return fmt.Errorf("postgres: publishing the hash listing to %s: %w", publicPath, err)
	}
	return nil
}

// writeFileAtomically replaces path's content with data without ever
// exposing a partially-written or truncated intermediate state to a
// concurrent reader: it writes to a temp file in path's own directory
// (guaranteeing the later rename stays on the same filesystem, a
// prerequisite for os.Rename's atomicity) and renames it into place.
// POSIX rename(2) is a single atomic operation that swaps the directory
// entry -- any file descriptor a reader already opened against the OLD
// content keeps working unaffected against that now-unlinked inode until
// it is closed (see hashlisting_test.go's
// ...ARequestAlreadyReadingThePublicCopyIsUnaffectedByALaterPublish for
// the exact property this exists to guarantee).
func writeFileAtomically(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("creating a temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing the temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing the temp file for %s: %w", path, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("setting permissions on the temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming the temp file into place at %s: %w", path, err)
	}
	return nil
}
