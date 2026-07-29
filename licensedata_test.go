package configdata_test

// Task 3.11: LICENSE-DATA defers to per-source terms; LICENSE stays MIT;
// no statement anywhere asserts one licence over all derived data (spec
// source-attribution-licensing, "No blanket data-licence claim exists in
// the repository", settled decision D2).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLicenseFiles_NoBlanketDataLicenceClaim(t *testing.T) {
	root := repoRoot(t)

	license := mustReadFile(t, filepath.Join(root, "LICENSE"))
	if !strings.Contains(license, "MIT License") {
		t.Error("LICENSE does not read as the MIT License")
	}

	licenseData := mustReadFile(t, filepath.Join(root, "LICENSE-DATA"))
	if !strings.Contains(licenseData, "config/sources/") {
		t.Error("LICENSE-DATA does not point to config/sources/{source}.yaml as the authoritative per-source terms")
	}

	blanketPhrases := []string{
		"all derived data is licensed under",
		"all data is licensed under",
		"the entirety of this repository's data",
		"every series is licensed under cc by 4.0",
	}
	lower := strings.ToLower(licenseData)
	for _, phrase := range blanketPhrases {
		if strings.Contains(lower, phrase) {
			t.Errorf("LICENSE-DATA appears to assert a blanket licence over all derived data: contains %q", phrase)
		}
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}
