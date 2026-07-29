package config

// Task 3.1 (RED) / 3.2 (GREEN): the typed config loader. fsys is
// expected to already be rooted at the config/ directory (the caller
// does `fs.Sub(configdata.FS, "config")` — ADR-1 keeps the embed shim
// itself agnostic of this package's internal layout), so "sources/*.yaml"
// and "series/*.yaml" are direct children of fsys.

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load parses every config/sources/*.yaml and config/series/*.yaml file
// reachable from fsys into a typed Config. A tree missing the sources/
// or series/ subdirectory is not an error — Phase 3 ships sources/ only,
// series/ arrives in Phase 5b/6 (TestLoad_EmptyTreeIsNotAnError).
func Load(fsys fs.FS) (*Config, error) {
	sources, err := loadSources(fsys)
	if err != nil {
		return nil, err
	}
	series, err := loadSeries(fsys)
	if err != nil {
		return nil, err
	}
	breaks, err := loadBreaks(fsys)
	if err != nil {
		return nil, err
	}
	events, err := loadEvents(fsys)
	if err != nil {
		return nil, err
	}
	return &Config{Sources: sources, Series: series, Breaks: breaks, Events: events}, nil
}

func loadSources(fsys fs.FS) (map[string]SourceConfig, error) {
	out := map[string]SourceConfig{}
	names, err := yamlFilesIn(fsys, "sources")
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		filePath := path.Join("sources", name)
		var sc SourceConfig
		if err := decodeYAML(fsys, filePath, &sc); err != nil {
			return nil, err
		}
		sc.FilePath = filePath
		out[sc.ID] = sc
	}
	return out, nil
}

func loadSeries(fsys fs.FS) ([]SeriesConfig, error) {
	var out []SeriesConfig
	names, err := yamlFilesIn(fsys, "series")
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		filePath := path.Join("series", name)
		var sec SeriesConfig
		if err := decodeYAML(fsys, filePath, &sec); err != nil {
			return nil, err
		}
		sec.FilePath = filePath
		out = append(out, sec)
	}
	return out, nil
}

// loadBreaks parses config/rupturas.yaml (PRD §9.6 filename, fixed and
// kept in Spanish) — a bare top-level YAML sequence of BreakConfig
// entries, not present at all until Phase 7 ships it, so a missing file
// is not an error (same convention as loadSources/loadSeries for a
// not-yet-shipped directory).
func loadBreaks(fsys fs.FS) ([]BreakConfig, error) {
	const filePath = "rupturas.yaml"
	present, err := fileExists(fsys, filePath)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	var out []BreakConfig
	if err := decodeYAML(fsys, filePath, &out); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].FilePath = filePath
	}
	return out, nil
}

// loadEvents parses config/eventos.yaml (Group read per-entry from the
// YAML: "exogenous"|"milestones") and config/gobiernos.yaml (Group
// always "governments", assigned here rather than repeated in every
// entry — the whole file is exactly one group, PRD §9.6).
func loadEvents(fsys fs.FS) ([]EventConfig, error) {
	exogenousAndMilestones, err := loadEventFile(fsys, "eventos.yaml", "")
	if err != nil {
		return nil, err
	}
	governments, err := loadEventFile(fsys, "gobiernos.yaml", "governments")
	if err != nil {
		return nil, err
	}
	return append(exogenousAndMilestones, governments...), nil
}

// loadEventFile parses one editorial event file. defaultGroup is applied
// to any entry that omits its own `group:` field; passing "" for
// eventos.yaml means every entry there MUST declare its own group
// explicitly (exogenous vs milestones), while gobiernos.yaml entries
// never need to.
func loadEventFile(fsys fs.FS, filePath, defaultGroup string) ([]EventConfig, error) {
	present, err := fileExists(fsys, filePath)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	var out []EventConfig
	if err := decodeYAML(fsys, filePath, &out); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].FilePath = filePath
		if out[i].Group == "" {
			out[i].Group = defaultGroup
		}
	}
	return out, nil
}

// fileExists reports whether filePath exists directly inside fsys,
// treating fs.ErrNotExist as "not present" rather than an error (mirrors
// yamlFilesIn's own not-shipped-yet convention below).
func fileExists(fsys fs.FS, filePath string) (bool, error) {
	if _, err := fs.Stat(fsys, filePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("config: checking %s: %w", filePath, err)
	}
	return true, nil
}

// yamlFilesIn returns the sorted, deterministic list of *.yaml/*.yml
// file names directly inside dir, or an empty (non-error) list when dir
// itself does not exist yet.
func yamlFilesIn(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: reading %s/: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !isYAML(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func decodeYAML(fsys fs.FS, filePath string, v any) error {
	data, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return fmt.Errorf("config: reading %s: %w", filePath, err)
	}
	if err := yaml.Unmarshal(data, v); err != nil {
		return fmt.Errorf("config: parsing %s: %w", filePath, err)
	}
	return nil
}

func isYAML(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}
