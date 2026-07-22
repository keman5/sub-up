#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
REMOTE="${FORK_SYNC_REMOTE:-origin}"
MAIN_BRANCH="${FORK_SYNC_MAIN_BRANCH:-main}"
TARGET_BRANCH="${FORK_SYNC_TARGET_BRANCH:-subapi}"
MODE=""
APPLY=0
REVIEW=""

usage() {
  cat <<'EOF'
Usage:
  sync-main-into-subapi.sh sync --apply [--remote NAME] [--main BRANCH] [--target BRANCH]
  sync-main-into-subapi.sh audit --review first|second [--remote NAME] [--main BRANCH] [--target BRANCH]

sync 必须带 --apply。它会拉取来源和目标引用，创建本地安全引用与 fork 补丁快照，
快进目标分支，再将来源合并进去。发生冲突时会保留进行中的合并并以非零状态退出，
等待基于证据的人工解决。

audit 不会修改 Git 或源码。它检查合并状态、空白字符、冲突标记和 fork 护栏，
并将详细复查证据保存到 tmp/fork-maintenance/。
EOF
}

die() {
  printf '[fork-sync-deploy] 错误：%s\n' "$*" >&2
  exit 1
}

require_clean_worktree() {
  [[ -z "$(git -C "$ROOT" status --porcelain=v1 --untracked-files=all)" ]] || die "工作区不干净；同步前请提交、暂存或以其他方式保留改动"
  git -C "$ROOT" diff --quiet || die "检测到未暂存差异"
  git -C "$ROOT" diff --cached --quiet || die "检测到已暂存差异"
}

require_no_operation() {
  local git_dir
  git_dir="$(git -C "$ROOT" rev-parse --git-dir)"
  [[ "$git_dir" = /* ]] || git_dir="$ROOT/$git_dir"
  for marker in MERGE_HEAD REBASE_HEAD CHERRY_PICK_HEAD rebase-merge rebase-apply; do
    [[ ! -e "$git_dir/$marker" ]] || die "检测到进行中的 Git 操作：$marker"
  done
}

validate_refs() {
  git -C "$ROOT" rev-parse --verify --quiet "$REMOTE/$MAIN_BRANCH" >/dev/null || die "缺少来源引用：$REMOTE/$MAIN_BRANCH"
  git -C "$ROOT" rev-parse --verify --quiet "$REMOTE/$TARGET_BRANCH" >/dev/null || die "缺少目标引用：$REMOTE/$TARGET_BRANCH"
}

print_checklist() {
  printf '\n[fork-sync-deploy] Fork 专属清单标题（必须逐项复查 docs/FORK_MAINTENANCE_CN.md）：\n'
  sed -n '/^## Non-upstream feature recovery checklist$/,/^## Local patch records$/p; /^## 非上游功能恢复清单$/,/^## 本地补丁记录$/p' "$ROOT/docs/FORK_MAINTENANCE_CN.md" | rg '^### [0-9]+\.' || true
}

write_review_evidence() {
  local label="$1"
  local timestamp evidence_dir source_ref target_ref
  timestamp="$(date +%Y%m%d%H%M%S)"
  evidence_dir="$ROOT/tmp/fork-maintenance/reviews/${timestamp}-${label}"
  source_ref="$REMOTE/$MAIN_BRANCH"
  target_ref="HEAD"
  mkdir -p "$evidence_dir"
  git -C "$ROOT" diff --name-status "$source_ref...$target_ref" > "$evidence_dir/fork-delta-name-status.txt"
  "$ROOT/tools/fork-maintenance/fork-maintenance.sh" inventory --base "$source_ref" > "$evidence_dir/fork-inventory.txt"
  printf '[fork-sync-deploy] Fork 文件数：%s；证据目录：%s\n' \
    "$(wc -l < "$evidence_dir/fork-delta-name-status.txt" | tr -d ' ')" "$evidence_dir"
}

sync() {
  [[ "$APPLY" -eq 1 ]] || die "sync 会修改本地 Git 状态；请使用 --apply 重新运行"
  require_clean_worktree
  require_no_operation

  git -C "$ROOT" fetch --prune "$REMOTE" "$MAIN_BRANCH" "$TARGET_BRANCH"
  validate_refs
  git -C "$ROOT" switch "$TARGET_BRANCH"
  git -C "$ROOT" pull --ff-only "$REMOTE" "$TARGET_BRANCH"

  local before timestamp backup_ref
  before="$(git -C "$ROOT" rev-parse HEAD)"
  timestamp="$(date +%Y%m%d%H%M%S)"
  backup_ref="refs/fork-sync-backups/${TARGET_BRANCH}-before-${timestamp}"
  git -C "$ROOT" update-ref "$backup_ref" "$before"
  printf '[fork-sync-deploy] 安全引用：%s -> %s\n' "$backup_ref" "$before"

  "$ROOT/tools/fork-maintenance/fork-maintenance.sh" snapshot --base "$REMOTE/$MAIN_BRANCH"
  write_review_evidence "pre-merge"
  print_checklist

  if ! git -C "$ROOT" merge --no-ff --no-edit "$REMOTE/$MAIN_BRANCH"; then
    printf '[fork-sync-deploy] 合并冲突需要按维护文档人工解决：\n' >&2
    git -C "$ROOT" diff --name-only --diff-filter=U >&2 || true
    exit 2
  fi

  printf '[fork-sync-deploy] 合并完成：%s\n' "$(git -C "$ROOT" rev-parse HEAD)"
  printf '[fork-sync-deploy] 接下来运行：%s audit --review first\n' "$0"
}

audit() {
  [[ "$REVIEW" = first || "$REVIEW" = second ]] || die "audit 必须指定 --review first 或 --review second"
  require_no_operation
  validate_refs
  [[ "$(git -C "$ROOT" branch --show-current)" = "$TARGET_BRANCH" ]] || die "当前分支不是 $TARGET_BRANCH"
  [[ -z "$(git -C "$ROOT" ls-files -u)" ]] || die "仍存在未解决的合并索引项"

  git -C "$ROOT" diff --check "$REMOTE/$MAIN_BRANCH...HEAD"
  if git -C "$ROOT" grep -nE '^(<<<<<<< |=======($| )|>>>>>>> )' -- ':!docs/FORK_MAINTENANCE_CN.md'; then
    die "已跟踪文件中存在未解决的冲突标记"
  fi

  write_review_evidence "review-$REVIEW"
  "$ROOT/tools/fork-maintenance/fork-maintenance.sh" verify-after-upstream --skip-build
  print_checklist
  printf '[fork-sync-deploy] 第 %s 次复查的自动护栏已通过。继续前必须完成维护文档规定的逐项人工复查。\n' "$REVIEW"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    sync|audit)
      [[ -z "$MODE" ]] || die "只能选择一种模式：sync 或 audit"
      MODE="$1"
      ;;
    --apply) APPLY=1 ;;
    --review)
      REVIEW="${2:-}"
      shift
      ;;
    --remote)
      REMOTE="${2:-}"
      shift
      ;;
    --main)
      MAIN_BRANCH="${2:-}"
      shift
      ;;
    --target)
      TARGET_BRANCH="${2:-}"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *) die "未知参数：$1" ;;
  esac
  shift
done

[[ -n "$MODE" ]] || { usage >&2; exit 1; }
case "$MODE" in
  sync) sync ;;
  audit) audit ;;
esac
