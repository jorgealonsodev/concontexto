# Branch protection settings (manual GitHub configuration)

`.github/CODEOWNERS` alone does not enforce a review *count* — GitHub only
uses it to determine which reviewers are eligible/required once "Require
review from Code Owners" is enabled. The two-approval, four-eyes control
on `/config/**` (PRD §9.6 / §15.2) requires the following branch protection
rule to be applied manually on GitHub (Settings → Branches → Branch
protection rules → `main`), since this cannot be expressed in a
repository file:

- **Require a pull request before merging**: enabled.
- **Require approvals**: `2`.
- **Require review from Code Owners**: enabled — combined with the
  `Require approvals: 2` setting above, this guarantees that a pull
  request touching `/config/**` needs at least two approving reviews,
  at least one of which is a listed code owner.
- **Dismiss stale pull request approvals when new commits are pushed**:
  enabled — prevents an approval from surviving an unreviewed late change
  to `/config/**`.
- **Require status checks to pass before merging**: enabled, with the
  `ci` workflow (`.github/workflows/ci.yml`) as a required check.
- **Do not allow bypassing the above settings**: enabled for
  administrators too — a four-eyes control that admins can bypass is not
  a four-eyes control.
- **Restrict who can push to matching branches**: enabled, direct pushes
  to `main` disabled for everyone (all changes go through PRs).

These settings apply repository-wide (GitHub branch protection is not
natively path-scoped), so every PR to `main` needs 2 approvals; the
CODEOWNERS entry ensures a `/config/**` change specifically needs a code
owner among those two.

Operational verification is task 7.14 of `phase-0-data-foundations`: after
these settings are applied, confirm that a single-approval PR touching
`rupturas.yaml` is blocked from merging.
