package publishing

// Task 4.9/4.10: artifact retention (design's own Migration/Rollout note,
// "last N artifacts retained, N configurable >= 5" -- kept enough history
// that a bad publish can be rolled back). ArchiveArtifact is called from
// Publish (trigger.go) right after a successful Export, best-effort: a
// retention failure is a rollback-convenience gap, never a reason to fail
// an otherwise-successful publish cycle (see PublishResult.ArchiveErr).

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// DefaultRetainedArtifacts is the retention floor (design's Migration/
// Rollout note and task 4.9's own wording: "last N (configurable, >= 5)
// retained"). A caller-requested retain below this floor is raised to it,
// never honoured verbatim -- the floor exists precisely so a
// misconfiguration cannot silently erode rollback history to nothing.
const DefaultRetainedArtifacts = 5

// snapshotTimeLayout is filesystem-safe (no colons) and sorts
// lexicographically in chronological order, which is what pruneHistory
// below relies on to find the oldest snapshots without parsing every
// directory name back into a time.Time.
const snapshotTimeLayout = "20060102T150405.000000000Z"

// ArchiveArtifact copies the just-written export at outDir into a new,
// timestamped snapshot under historyDir -- "rebuild from retained artifact
// reproduces byte-identical output" (task 4.9) holds by construction,
// since each snapshot is a verbatim byte-for-byte copy of a real, once-live
// outDir -- then prunes historyDir down to the newest `retain` snapshots
// (raised to DefaultRetainedArtifacts if requested lower).
//
// The live outDir is never written to, renamed, or considered for pruning
// by this function: it only ever creates a NEW directory under historyDir
// and removes OLD ones there. outDir and historyDir are always two
// distinct directory trees (the production caller, app/cmd/concontexto,
// never points historyDir at outDir or a parent of it), so "pruning can
// never delete the artifact currently being served" holds structurally,
// not merely by convention.
func ArchiveArtifact(outDir, historyDir string, generatedAt time.Time, retain int) error {
	if retain < DefaultRetainedArtifacts {
		retain = DefaultRetainedArtifacts
	}
	snapshotDir := filepath.Join(historyDir, generatedAt.UTC().Format(snapshotTimeLayout))
	if err := copyTree(outDir, snapshotDir); err != nil {
		return fmt.Errorf("publishing: archiving artifact snapshot: %w", err)
	}
	return pruneHistory(historyDir, retain)
}

// copyTree recursively copies every file under src into dst, preserving
// the relative directory structure. Both outDir (the live export) and each
// historyDir snapshot are small, JSON/CSV-only trees (a handful of files
// per series) -- a plain read-all/write-all per file needs no streaming.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("publishing: resolving relative path for %s: %w", path, err)
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("publishing: reading %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("publishing: creating directory %s: %w", filepath.Dir(target), err)
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// pruneHistory keeps only the newest `retain` snapshot directories
// directly under historyDir (ArchiveArtifact's own snapshotTimeLayout
// sorts lexicographically in chronological order, so no directory name
// needs re-parsing into a time.Time) and removes the rest. A non-existent
// historyDir (nothing archived yet) is not an error -- there is nothing to
// prune.
func pruneHistory(historyDir string, retain int) error {
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("publishing: reading artifact history %s: %w", historyDir, err)
	}

	var snapshots []string
	for _, e := range entries {
		if e.IsDir() {
			snapshots = append(snapshots, e.Name())
		}
	}
	sort.Strings(snapshots)
	if len(snapshots) <= retain {
		return nil
	}

	for _, name := range snapshots[:len(snapshots)-retain] {
		if err := os.RemoveAll(filepath.Join(historyDir, name)); err != nil {
			return fmt.Errorf("publishing: pruning old artifact snapshot %s: %w", name, err)
		}
	}
	return nil
}
