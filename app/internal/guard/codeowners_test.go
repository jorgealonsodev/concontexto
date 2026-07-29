package guard

// Task 7.14: branch protection (task 1.16) against real /config/**
// paths. .github/CODEOWNERS already carries a `/config/**` rule (PR 1a),
// so this test proves what IS statically checkable from the repository
// itself: the rule's path pattern and that it names at least two
// distinct reviewer handles (a single owner cannot satisfy a
// two-approval requirement, whatever GitHub's own branch-protection
// setting says separately).
//
// What this test deliberately does NOT and CANNOT verify (disclosed,
// not silently skipped — same discipline as PR 7a's honest gaps):
//  1. Whether GitHub's own branch-protection SETTINGS (2 required
//     approvals, "Require review from Code Owners", administrators not
//     exempt — see .github/BRANCH_PROTECTION.md) are actually applied on
//     the remote. That is a GitHub repository setting, not a file in
//     this tree, and cannot be observed by a Go test running offline.
//  2. Whether the second listed handle, `@TODO-second-config-reviewer`,
//     is a real GitHub account. It is a documented placeholder (see
//     CODEOWNERS's own TODO comment and BRANCH_PROTECTION.md) pending a
//     maintainer decision (PRD §18: four-eyes on /config/** cannot rest
//     on one person). This test intentionally does not fail on the
//     placeholder text, since replacing it is a human, non-code action.
//  3. That "a single-approval PR touching rupturas.yaml is blocked from
//     merging" (spec's own operational scenario) — this repository has
//     zero commits and no remote, so there is no PR to open and no
//     merge to observe. That operational proof remains blocked until
//     both the real second reviewer and at least one push/PR exist.

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// codeownersRuleFor returns the space-separated owner tokens for the
// first CODEOWNERS line whose path pattern equals want, or nil if no
// such line exists.
func codeownersRuleFor(t *testing.T, path, want string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == want {
			return fields[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning %s: %v", path, err)
	}
	return nil
}

func TestCODEOWNERS_ConfigPathRequiresAtLeastTwoDistinctOwners(t *testing.T) {
	path := filepath.Join(repoRoot(t), ".github", "CODEOWNERS")
	owners := codeownersRuleFor(t, path, "/config/**")
	if owners == nil {
		t.Fatalf("no CODEOWNERS rule found for /config/** in %s — branch protection cannot require code-owner review on editorial config without one", path)
	}

	seen := map[string]bool{}
	for _, o := range owners {
		if !strings.HasPrefix(o, "@") {
			t.Errorf("CODEOWNERS entry %q for /config/** does not look like a GitHub handle (missing @)", o)
			continue
		}
		seen[o] = true
	}
	if len(seen) < 2 {
		t.Fatalf("/config/** lists %d distinct owner(s) (%v), want at least 2 — a single-owner CODEOWNERS entry cannot satisfy PRD §9.6/§15.2's four-eyes (two-approval) requirement", len(seen), owners)
	}
}

func TestBRANCH_PROTECTION_DocumentsTheTwoApprovalConfigOwnerSettings(t *testing.T) {
	path := filepath.Join(repoRoot(t), ".github", "BRANCH_PROTECTION.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	content := string(data)

	for _, want := range []string{
		"Require approvals", "2",
		"Require review from Code Owners",
		"Do not allow bypassing",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("%s does not mention %q — the documented branch-protection settings must cover the two-approval, code-owner-required, no-admin-bypass rule (PRD §9.6/§15.2)", path, want)
		}
	}
}
