# Fork Maintenance Automation

This directory contains guardrails for preserving fork-only changes when syncing from upstream.

The scripts do not silently rewrite upstream merges. They inventory fork-only changes, auto-record staged local changes in the central maintenance document, export patch snapshots, run fork-specific verification, and reapply known non-Git production state when explicitly requested.

## Commands

```bash
tools/fork-maintenance/fork-maintenance.sh inventory --base upstream/main
tools/fork-maintenance/fork-maintenance.sh check-doc
tools/fork-maintenance/fork-maintenance.sh record --title "Describe the fork-only change"
tools/fork-maintenance/fork-maintenance.sh sort-doc
tools/fork-maintenance/fork-maintenance.sh snapshot --base upstream/main
tools/fork-maintenance/fork-maintenance.sh verify-after-upstream
tools/fork-maintenance/fork-maintenance.sh reapply-production-state
tools/fork-maintenance/fork-maintenance.sh reapply-production-state --apply
```

Equivalent Makefile shortcuts are available:

```bash
make fork-check
make fork-inventory FORK_BASE=upstream/main
make fork-snapshot FORK_BASE=upstream/main
make fork-verify
make fork-restore-dry-run
```

`reapply-production-state` is dry-run by default. It requires `--apply` before it touches the remote host.
It regenerates `favicon.ico` from the current `frontend/public/logo.png` before upload, so the fallback icon follows the logo.

`record` appends a TODO maintenance record template to `docs/FORK_MAINTENANCE_CN.md` for current record-candidate changes. Fill in the TODOs before committing.

`check-doc` and `record` keep the local patch table sorted by ascending date. Use `sort-doc` to normalize the order manually after editing old records.

Long records should be moved into `docs/fork-maintenance/YYYY-MM.md` before commit, with `docs/FORK_MAINTENANCE_CN.md` keeping only the main index and a short link to the monthly detail file. This keeps the central doc readable and reduces merge conflicts during upstream syncs.

For binary deployment over slow links, use the gzip transfer/decompress helper:

```bash
deploy/local-gzip-binary-deploy.sh
deploy/local-gzip-binary-deploy.sh --apply --deploy
```

## Optional Local Hook

```bash
tools/fork-maintenance/install-hooks.sh
```

The pre-commit hook auto-appends a pending entry to `docs/FORK_MAINTENANCE_CN.md` for staged local changes unless a merge, rebase, or cherry-pick is in progress.

The post-merge and post-rewrite hooks run fork-specific verification after merge/rebase and print a warning if fork changes need review. They do not automatically force-apply patches.
