---
name: nonfork-sync-deploy
description: Use when deploying a non-fork Sub2API repository by pulling the origin subapi branch, merging Wei-Shaw/sub2api main into subapi, reapplying non-fork maintenance requirements, and performing a gated production rollout.
---

# Non-fork Sync and Deploy

Use this skill only from the repository root. Do not create a worktree. The deployment target is the local `subapi` branch. This is a non-fork workflow: never call GitHub's Fork `merge-upstream` API and never assume `origin/main` is the upstream source.

## Required remotes and references

- `origin`: the user's non-fork repository; pull `origin/subapi` first.
- `upstream`: `https://github.com/Wei-Shaw/sub2api.git`; merge `upstream/main`.
- target: local `subapi`; deploy the verified local merge commit.

Resolve the actual URLs before mutating Git. If either remote points somewhere else, stop and report it. Do not print credentials, `.env` values, database dumps, tokens or private host details.

## Hard gates

Do not deploy, force-push, reset, discard changes or abort the merge after a gate fails.

1. Confirm repository root, clean worktree, no merge/rebase/cherry-pick state, and the expected remotes.
2. Pull `subapi` first:

   ```bash
   git pull --ff-only origin subapi
   ```

   Then save a backup ref and fetch the exact upstream branch:

   ```bash
   stamp="$(date +%Y%m%d%H%M%S)"
   git update-ref "refs/nonfork-sync-backups/$stamp/before-merge" HEAD
   git fetch --prune upstream main
   git rev-parse upstream/main
   ```

3. Merge upstream into `subapi`:

   ```bash
   git merge --no-ff --no-edit upstream/main
   ```

   If conflicts occur, list every unmerged path with `git diff --name-only --diff-filter=U`. Before resolving each path, read the related entry in `docs/FORK_MAINTENANCE_CN.md`, `docs/fork-maintenance/*.md`, `docs/VPS_DEPLOY_NOTES.md`, `deploy/README.md` and the relevant deployment script. Resolve behavior, not conflict markers: retain upstream capability unless a documented non-fork decision says to hide or reject the visible surface while preserving backend compatibility. Never bulk-select `ours` or `theirs`.

   After resolution:

   ```bash
   git add <resolved-paths>
   git diff --cached --check
   git commit
   git diff --check
   git ls-files -u
   ```

4. Establish evidence in `tmp/nonfork-maintenance/reviews/<timestamp>-first/` (ignored local evidence is preferred). Save at least:

   - `source-and-target.txt`: `origin/subapi`, `upstream/main`, merge commit and both parent SHAs.
   - `deployment-record-sources.txt`: exact record files and line references used.
   - `non-fork-requirements.tsv`: one row per unretired requirement.
   - `upstream-commits.tsv`, `upstream-changed-paths.tsv`, `upstream-deleted-paths.tsv`.
   - `upstream-capabilities.tsv`: one row per upstream commit, with behavior, current implementation, rationale, validation and result.
   - `upstream-paths-review.tsv`: one row per upstream-changed path.
   - `post-merge-overrides-review.tsv`: every post-merge deviation from `upstream/main`.
   - `product-surface-decisions.tsv`: every changed page, route, menu, form field, setting, i18n surface, DTO/schema and migration that can affect operators or users.
   - `final-deleted-upstream-paths.tsv`: must be empty unless the user explicitly approved a deletion.

   A row with `TODO`, blank validation, unknown status, or missing evidence blocks deployment. Do not collapse several independent behaviors into one row.

5. Reapply the non-fork records one by one. The baseline is the current records, not filenames or an assumption that the upstream implementation is equivalent. For each row, check:

   - real entry point and affected files;
   - API request/response, persistence, runtime state and UI behavior where applicable;
   - upstream equivalent and the exact remaining difference;
   - whether the product surface is `accept`, `hide-but-keep-backend-compatible`, or `reject` with a dated record basis;
   - the stated focused test, typecheck/build or endpoint check and its result.

   Specifically preserve the recorded behavior for account usage refresh and runtime-state writeback, Codex Spark request compatibility, routing/billing/failover semantics, Pages and VPS topology, and any later monthly maintenance entry. A visible upstream feature is not automatically accepted; a hidden surface must retain compatible API, schema and migration behavior where the records require it.

6. Perform the first independent review before full tests. Re-read all record sources and rebuild the tables from the current `upstream/main` and merge parents. Re-check every row and every conflict file. Do not use “file exists”, “no conflict markers” or a broad test suite as evidence of behavior equivalence.

7. Run complete verification, then perform a second independent review from the records again. The second review must not copy the first review's conclusions or tables. At minimum run the repository's prescribed checks when applicable:

   ```bash
   git diff --check
   git diff --check upstream/main...HEAD
   git ls-files -u
   make test
   make secret-scan
   pnpm --dir frontend run typecheck
   pnpm --dir frontend run build
   ```

   Also run every focused command named by each changed record, including backend Go tests, frontend Vitest tests, maintenance guards and deployment-script syntax checks. If a command is unavailable, record the exact blocker and stop before deployment.

8. Only after both reviews and all required checks pass, read the current deployment instructions in `docs/VPS_DEPLOY_NOTES.md`, `deploy/README.md` and `deploy/local-gzip-binary-deploy.sh`. Confirm whether frontend assets changed: use `--skip-frontend-build` only for backend/API-only changes; otherwise build and publish the documented Pages artifacts as well.

## Deployment gates

Do not expose secrets in output. Default SSH alias is `51tokens`; use an explicit host only when the user supplied one.

First run both dry-runs and inspect the printed target, image, environment order and recovery behavior:

```bash
tools/fork-maintenance/fork-maintenance.sh reapply-production-state --host 51tokens
deploy/local-gzip-binary-deploy.sh --host 51tokens [--skip-frontend-build]
```

Then deploy the verified local commit in the documented order. The script must gate `test` before `ap1`, and `ap1` before `primary`; a failed environment stops the rollout:

```bash
deploy/local-gzip-binary-deploy.sh --apply --deploy --host 51tokens [--skip-frontend-build]
tools/fork-maintenance/fork-maintenance.sh reapply-production-state --host 51tokens --apply
```

After each environment, verify its container, local port and public health endpoints. After the full rollout, verify all six documented health endpoints (`test/a2t`, `a1/ap1`, `ai/api`) and the relevant public settings/runtime-state checks. Restore recorded non-Git state only after the new code is healthy, then re-check the endpoints. Keep generated remote backup paths in the final report.

Do not push `subapi` unless explicitly requested. If deployment changes remote state but a later check fails, stop, report the exact stage and preserved backup information, and do not continue to another environment.

## Final report

Report the pull source, upstream SHA, merge commit and parents, conflicts and resolutions, each review table result, every focused/full verification command, deployment image tag, test -> a1 -> primary results, state restoration result and health endpoint results. State explicitly whether `subapi` was pushed.
