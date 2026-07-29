# Deploy (Portainer, VPS)

Remediation batch (sdd-verify CRITICAL C5, 2026-07-29): task 1.17
originally claimed a "deploy step" that did not exist anywhere in the
repository — the same "marked `[x]`, deliverable absent" failure mode
task 1.21 already disclosed once (see `openspec/changes/phase-0-data-foundations/apply-progress.md`).

`.github/workflows/deploy.yml` is a real, correctly wired deploy workflow.
It is honest about its own status: **PRD §17's milestone-0.1 exit
criterion ("automated deploy of a hello world") is genuinely UNMET as of
this batch**, because this environment has no VPS running Portainer and
no secrets configured to reach one. The workflow gates on that fact
explicitly — when the one required secret below is absent, its gate step
prints a `::warning::` and every deploy step is skipped, rather than
reporting a fake green deploy.

## What must exist before this criterion can close

1. A VPS (or any host) running Portainer, reachable from the internet
   (or from GitHub Actions' runner IP range) over HTTPS.
2. A Portainer **stack** on that host, defined to pull
   `ghcr.io/jorgealonsodev/concontexto:latest` (the image `deploy.yml`
   builds and pushes to GitHub Container Registry — no extra registry
   secret is needed for the push half; it uses the workflow's own
   built-in `GITHUB_TOKEN`).
3. A **webhook** enabled on that stack (Portainer → stack → Webhooks),
   which redeploys the stack (re-pulling the `:latest` tag) whenever its
   URL receives an HTTP POST.

## Required GitHub Actions secret

| Secret | What it is | Where it comes from |
|---|---|---|
| `PORTAINER_WEBHOOK_URL` | The stack webhook URL from step 3 above | Portainer's own UI, once the stack exists |

No other secret is required: the image push to `ghcr.io` authenticates
with the workflow's automatically provided `GITHUB_TOKEN`, which already
has `packages: write` scope for a workflow running in this repository.

## What the workflow does once the secret is configured

1. Triggers after `ci.yml` completes successfully on `main` (or on
   manual `workflow_dispatch`).
2. Builds the repository's `Dockerfile` image and pushes it to
   `ghcr.io/jorgealonsodev/concontexto` tagged both `:latest` and with
   the triggering commit SHA (so a specific deployed build is always
   traceable).
3. POSTs to `PORTAINER_WEBHOOK_URL`, which tells the already-configured
   stack to redeploy — Portainer, not this workflow, decides how the
   running container is replaced.

This workflow never provisions the VPS or the Portainer stack itself:
that is a one-time manual infrastructure step (1 and 2 above), not
something a CI job should be trusted to do unattended against a
production host.
