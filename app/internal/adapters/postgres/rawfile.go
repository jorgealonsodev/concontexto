package postgres

// Task 5a.1/5a.2 (GREEN, DB half): raw_file is the database half of the
// raw-file archive (spec raw-file-archive, "Raw downloads are stored
// immutably keyed by SHA-256"). filestore.Store (a sibling, DB-free
// adapter) owns the bytes-on-disk half; this file owns the row that
// makes the archive queryable and lets provenance.go's chain
// (observation -> ingestion_run -> raw_file -> source) resolve.
// ArchiveRawFile composes the two: the disk write and the row insert
// both complete -- disk first, then a committed row -- before this
// function returns, so a caller that only invokes a parser AFTER
// ArchiveRawFile returns automatically satisfies "the archive write
// happens before any parsing": if the parser panics or errors, the
// archive already exists independently of that outcome.
//
// raw_file.hash is the table's PRIMARY KEY, so "an identical redownload
// does not duplicate storage" is a database-enforced invariant here too,
// not just filestore.Store's own dedup at the file level: INSERT ... ON
// CONFLICT (hash) DO NOTHING makes a second archive of the same bytes a
// no-op at the row level.

import (
	"context"
	"fmt"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore"
)

// RawFile mirrors one raw_file row.
type RawFile struct {
	Hash         string
	SourceID     string
	URL          string
	DownloadedAt time.Time
	StoragePath  string
	SizeBytes    int64
}

// RecordRawFile inserts a raw_file row for an already-archived payload
// (filestore.Store.Put has already run). A hash collision -- the same
// bytes downloaded again -- is not an error: the pre-existing row is
// returned unchanged and isNew reports false (spec "An identical
// redownload does not duplicate storage").
func RecordRawFile(ctx context.Context, db DBTX, in RawFile) (RawFile, bool, error) {
	tag, err := db.Exec(ctx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (hash) DO NOTHING`,
		in.Hash, in.SourceID, in.URL, in.DownloadedAt, in.StoragePath, in.SizeBytes)
	if err != nil {
		return RawFile{}, false, fmt.Errorf("postgres: recording raw_file %s: %w", in.Hash, err)
	}
	if tag.RowsAffected() > 0 {
		return in, true, nil
	}

	existing, err := readRawFile(ctx, db, in.Hash)
	if err != nil {
		return RawFile{}, false, fmt.Errorf("postgres: reading pre-existing raw_file %s: %w", in.Hash, err)
	}
	return existing, false, nil
}

func readRawFile(ctx context.Context, db DBTX, hash string) (RawFile, error) {
	row := db.QueryRow(ctx, `SELECT hash, source_id, url, downloaded_at, storage_path, size_bytes
		FROM raw_file WHERE hash=$1`, hash)
	var rf RawFile
	if err := row.Scan(&rf.Hash, &rf.SourceID, &rf.URL, &rf.DownloadedAt, &rf.StoragePath, &rf.SizeBytes); err != nil {
		return RawFile{}, err
	}
	return rf, nil
}

// ArchiveRawFile is the ONE function ingestion code must call to turn a
// downloaded payload into a durably archived raw file: it writes the
// bytes to disk via store (content-addressed, deduplicated by
// filestore.Store itself) and then records the raw_file row (also
// deduplicated, by the database). Documented at length above.
func ArchiveRawFile(ctx context.Context, db DBTX, store *filestore.Store, sourceID, url string, payload []byte, downloadedAt time.Time) (RawFile, bool, error) {
	stored, err := store.Put(payload)
	if err != nil {
		return RawFile{}, false, fmt.Errorf("postgres: archiving payload from %s to disk: %w", url, err)
	}

	return RecordRawFile(ctx, db, RawFile{
		Hash:         stored.Hash,
		SourceID:     sourceID,
		URL:          url,
		DownloadedAt: downloadedAt,
		StoragePath:  stored.StoragePath,
		SizeBytes:    stored.SizeBytes,
	})
}
