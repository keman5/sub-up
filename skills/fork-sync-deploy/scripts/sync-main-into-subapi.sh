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
  sync-main-into-subapi.sh github-sync --apply [--remote NAME] [--main BRANCH]
  sync-main-into-subapi.sh sync --apply [--remote NAME] [--main BRANCH] [--target BRANCH]
  sync-main-into-subapi.sh audit --review first|second [--remote NAME] [--main BRANCH] [--target BRANCH]

github-sync 必须带 --apply。它会通过 GitHub API 同步 fork 的 main 到上游 main，
重新拉取远端并验证两个分支 SHA 一致。

sync 必须带 --apply。它会先执行 github-sync，再拉取目标引用、创建本地安全引用与
fork 补丁快照，快进目标分支，再将来源合并进去。发生冲突时会保留进行中的合并并以
非零状态退出，等待基于证据的人工解决。

audit 不会修改 Git 或源码。它先生成非 fork 功能审查表，再检查部署记录完整性、
合并状态、空白字符、冲突标记和 fork 护栏，并将详细复查证据保存到 tmp/fork-maintenance/。
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
  validate_main_ref
  git -C "$ROOT" rev-parse --verify --quiet "$REMOTE/$TARGET_BRANCH" >/dev/null || die "缺少目标引用：$REMOTE/$TARGET_BRANCH"
}

validate_main_ref() {
  git -C "$ROOT" rev-parse --verify --quiet "$REMOTE/$MAIN_BRANCH" >/dev/null || die "缺少来源引用：$REMOTE/$MAIN_BRANCH"
}

github_repo_from_remote() {
  local remote_url repo
  remote_url="$(git -C "$ROOT" remote get-url "$REMOTE")" || die "无法读取远端 $REMOTE 的 URL"
  case "$remote_url" in
    git@github.com:*) repo="${remote_url#git@github.com:}" ;;
    ssh://git@github.com/*) repo="${remote_url#ssh://git@github.com/}" ;;
    https://github.com/*|http://github.com/*) repo="${remote_url#*github.com/}" ;;
    *) die "远端 $REMOTE 不是 GitHub URL，无法执行 GitHub fork 上游同步：$remote_url" ;;
  esac
  repo="${repo%.git}"
  [[ "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || die "无法从远端 URL 解析 GitHub 仓库：$remote_url"
  printf '%s\n' "$repo"
}

github_sync() {
  [[ "$APPLY" -eq 1 ]] || die "github-sync 会更新 GitHub fork 的 ${MAIN_BRANCH}；请使用 --apply 重新运行"
  require_clean_worktree
  require_no_operation
  command -v gh >/dev/null || die "未安装 GitHub CLI (gh)，无法同步 GitHub fork"
  gh auth status -h github.com >/dev/null 2>&1 || die "GitHub CLI 未登录 github.com 或凭据不可用"

  local fork_repo upstream_repo upstream_sha fork_sha
  fork_repo="$(github_repo_from_remote)"
  upstream_repo="$(gh api "repos/$fork_repo" --jq 'if .fork and .parent then .parent.full_name else empty end')"
  [[ -n "$upstream_repo" ]] || die "$fork_repo 不是可同步的 GitHub fork"
  upstream_sha="$(gh api "repos/$upstream_repo/commits/$MAIN_BRANCH" --jq '.sha')"
  [[ -n "$upstream_sha" ]] || die "无法读取上游 $upstream_repo/$MAIN_BRANCH 的提交 SHA"

  gh api --method POST "repos/$fork_repo/merge-upstream" -f "branch=$MAIN_BRANCH" >/dev/null
  git -C "$ROOT" fetch --prune "$REMOTE" "$MAIN_BRANCH"
  validate_main_ref
  fork_sha="$(git -C "$ROOT" rev-parse "$REMOTE/$MAIN_BRANCH")"
  [[ "$fork_sha" = "$upstream_sha" ]] || die "GitHub fork $fork_repo/$MAIN_BRANCH ($fork_sha) 未与上游 $upstream_repo/$MAIN_BRANCH ($upstream_sha) 对齐；停止本地合并"
  printf '[fork-sync-deploy] GitHub fork 已同步：%s/%s = %s (%s)\n' \
    "$fork_repo" "$MAIN_BRANCH" "$fork_sha" "$upstream_repo"
}

collect_documented_items() {
  printf '%s\n' '[部署记录来源]'
  printf '%s\n' 'docs/FORK_MAINTENANCE_CN.md'
  printf '%s\n' 'docs/fork-maintenance/*.md'
  printf '%s\n' 'docs/VPS_DEPLOY_NOTES.md'
  printf '%s\n' 'deploy/README.md'
  printf '%s\n' 'deploy/local-gzip-binary-deploy.sh'
  printf '%s\n' '[主文档恢复清单]'
  sed -n '/^## Non-upstream feature recovery checklist$/,/^## Local patch records$/p; /^## 非上游功能恢复清单$/,/^## 本地补丁记录$/p' "$ROOT/docs/FORK_MAINTENANCE_CN.md" | rg '^### [0-9]+\.' || true
  printf '%s\n' '[主文档本地补丁]'
  rg -n '^### 20[0-9]{2}-[0-9]{2}|^\| 20[0-9]{2}-[0-9]{2}' "$ROOT/docs/FORK_MAINTENANCE_CN.md" || true
  printf '%s\n' '[月度维护记录]'
  rg -n '^### 20[0-9]{2}-[0-9]{2}|^\| 20[0-9]{2}-[0-9]{2}' "$ROOT"/docs/fork-maintenance/*.md || true
  printf '%s\n' '[明确标记的非 fork 需求]'
  rg -n -i '非[[:space:]-]*fork[[:space:]-]*需求|non[[:space:]-]*fork[[:space:]-]*requirement' \
    "$ROOT/docs/FORK_MAINTENANCE_CN.md" "$ROOT"/docs/fork-maintenance/*.md || true
}

print_checklist() {
  printf '\n[fork-sync-deploy] 必须逐项修复并复查的非 fork 功能和部署记录：\n'
  collect_documented_items
}

validate_deployment_records() {
  local record_files
  record_files=(
    "$ROOT/docs/FORK_MAINTENANCE_CN.md"
    "$ROOT/docs/fork-maintenance/"*.md
    "$ROOT/docs/VPS_DEPLOY_NOTES.md"
    "$ROOT/deploy/README.md"
  )
  if rg -n -i '^[[:space:]]*(?:[-*][[:space:]]*)?TODO:[[:space:]]|待填写验证命令' "${record_files[@]}"; then
    die "部署记录含未完成的 TODO 或缺失验证/复查说明；逐项补齐并修复对应非 fork 功能后再审查"
  fi
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
  collect_documented_items > "$evidence_dir/documented-fork-items.txt"
  printf '%s\n' \
    'docs/FORK_MAINTENANCE_CN.md' \
    'docs/fork-maintenance/*.md' \
    'docs/VPS_DEPLOY_NOTES.md' \
    'deploy/README.md' \
    'deploy/local-gzip-binary-deploy.sh' > "$evidence_dir/deployment-record-sources.txt"
  printf '编号\t记录位置\t需求摘要\t入口或涉及文件\t预期行为\t上游等价性结论\t修复状态\t验证命令与结果\t复查证据\n' \
    > "$evidence_dir/non-fork-requirements.tsv"
  printf '[fork-sync-deploy] Fork 文件数：%s；证据目录：%s\n' \
    "$(wc -l < "$evidence_dir/fork-delta-name-status.txt" | tr -d ' ')" "$evidence_dir"
}

sync() {
  [[ "$APPLY" -eq 1 ]] || die "sync 会修改本地 Git 状态；请使用 --apply 重新运行"
  require_clean_worktree
  require_no_operation

  github_sync
  git -C "$ROOT" fetch --prune "$REMOTE" "$TARGET_BRANCH"
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

  write_review_evidence "review-$REVIEW"
  validate_deployment_records
  git -C "$ROOT" diff --check "$REMOTE/$MAIN_BRANCH...HEAD"
  if git -C "$ROOT" grep -nE '^(<<<<<<< |=======($| )|>>>>>>> )' -- ':!docs/FORK_MAINTENANCE_CN.md'; then
    die "已跟踪文件中存在未解决的冲突标记"
  fi

  "$ROOT/tools/fork-maintenance/fork-maintenance.sh" verify-after-upstream --skip-build
  print_checklist
  printf '[fork-sync-deploy] 第 %s 次复查的自动护栏已通过。继续前必须完成维护文档规定的逐项人工复查。\n' "$REVIEW"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    github-sync|sync|audit)
      [[ -z "$MODE" ]] || die "只能选择一种模式：github-sync、sync 或 audit"
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
  github-sync) github_sync ;;
  sync) sync ;;
  audit) audit ;;
esac
