#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
REMOTE="${FORK_SYNC_REMOTE:-origin}"
MAIN_BRANCH="${FORK_SYNC_MAIN_BRANCH:-main}"
TARGET_BRANCH="${FORK_SYNC_TARGET_BRANCH:-subapi}"
MODE=""
APPLY=0
REVIEW=""
EVIDENCE_DIR=""

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

collect_requirement_records() {
  rg --with-filename --line-number \
    '^### [0-9]+\.|^### 20[0-9]{2}-[0-9]{2}|^\| 20[0-9]{2}-[0-9]{2}' \
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
  local timestamp evidence_dir source_ref target_ref merge_commit upstream_base
  timestamp="$(date +%Y%m%d%H%M%S)"
  evidence_dir="$ROOT/tmp/fork-maintenance/reviews/${timestamp}-${label}"
  EVIDENCE_DIR="$evidence_dir"
  source_ref="$REMOTE/$MAIN_BRANCH"
  target_ref="HEAD"
  mkdir -p "$evidence_dir"
  git -C "$ROOT" rev-parse HEAD > "$evidence_dir/review-head.txt"
  git -C "$ROOT" rev-parse "$source_ref" > "$evidence_dir/review-source.txt"
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
  local requirement_index=0 record_path record_line record_summary
  while IFS=: read -r record_path record_line record_summary; do
    [[ -n "$record_path" && -n "$record_line" ]] || continue
    requirement_index=$((requirement_index + 1))
    record_path="${record_path#"$ROOT/"}"
    printf 'NF-%03d\t%s:%s\t%s\tTODO\tTODO\tTODO\tTODO\tTODO\tTODO\n' \
      "$requirement_index" "$record_path" "$record_line" "$record_summary"
  done < <(collect_requirement_records) >> "$evidence_dir/non-fork-requirements.tsv"

  merge_commit="$(find_latest_main_merge || true)"
  if [[ -n "$merge_commit" ]]; then
    upstream_base="$(git -C "$ROOT" merge-base "$merge_commit^1" "$merge_commit^2")"
    printf '%s\n' "$merge_commit" > "$evidence_dir/merge-commit.txt"
    printf '%s\n' "$upstream_base" > "$evidence_dir/upstream-base.txt"
    git -C "$ROOT" log --reverse --format='%H%x09%s' "$upstream_base..$merge_commit^2" \
      > "$evidence_dir/upstream-commits.tsv"
    git -C "$ROOT" diff --name-status "$upstream_base..$merge_commit^2" \
      > "$evidence_dir/upstream-changed-paths.tsv"
    git -C "$ROOT" diff --name-only "$upstream_base..$merge_commit^2" \
      > "$evidence_dir/upstream-changed-path-list.txt"
    git -C "$ROOT" diff --diff-filter=D --name-status "$upstream_base..$merge_commit^2" \
      > "$evidence_dir/upstream-deleted-paths.tsv"
    git -C "$ROOT" diff --diff-filter=D --name-status "$merge_commit^2..HEAD" \
      > "$evidence_dir/final-deleted-upstream-paths.tsv"
    git -C "$ROOT" show --remerge-diff --format=fuller "$merge_commit" \
      > "$evidence_dir/merge-remerge-diff.patch"
    git -C "$ROOT" diff --name-status "$merge_commit^2..HEAD" -- \
      $(<"$evidence_dir/upstream-changed-path-list.txt") \
      > "$evidence_dir/final-delta-on-upstream-paths.tsv"
    git -C "$ROOT" log --reverse --format='%H%x09%s' "$merge_commit..HEAD" -- \
      $(<"$evidence_dir/upstream-changed-path-list.txt") \
      > "$evidence_dir/post-merge-upstream-overrides.tsv"
    printf '提交SHA\t上游提交\t涉及路径或能力\t当前fork等价实现\t差异依据\t验证命令与结果\t结论\n' \
      > "$evidence_dir/upstream-capabilities.tsv"
    while IFS=$'\t' read -r commit_sha subject; do
      local commit_paths deviating_paths commit_path
      commit_paths="$(git -C "$ROOT" show --format= --name-only "$commit_sha" | sed '/^$/d' | sort -u | paste -sd ',' -)"
      deviating_paths=""
      while IFS= read -r commit_path; do
        [[ -n "$commit_path" ]] || continue
        if ! git -C "$ROOT" diff --quiet "$merge_commit^2..HEAD" -- "$commit_path"; then
          if [[ -n "$deviating_paths" ]]; then
            deviating_paths="$deviating_paths,$commit_path"
          else
            deviating_paths="$commit_path"
          fi
        fi
      done < <(git -C "$ROOT" show --format= --name-only "$commit_sha" | sed '/^$/d' | sort -u)
      if [[ -z "$deviating_paths" ]]; then
        printf '%s\t%s\t%s\t与origin/main逐字一致\t提交涉及路径最终无fork偏离\t逐路径git diff --quiet: 通过\t通过\n' \
          "$commit_sha" "$subject" "${commit_paths:-无路径}"
      else
        printf '%s\t%s\t%s\tTODO\t偏离路径:%s\tTODO\tTODO\n' \
          "$commit_sha" "$subject" "${commit_paths:-无路径}" "$deviating_paths"
      fi
    done < "$evidence_dir/upstream-commits.tsv" >> "$evidence_dir/upstream-capabilities.tsv"
    printf '上游路径\t上游变更类型\t最终相对origin/main差异\t偏离依据记录\t入口与行为等价性\t专项验证命令与结果\t结论\n' \
      > "$evidence_dir/upstream-paths-review.tsv"
    while IFS= read -r changed_path; do
      [[ -n "$changed_path" ]] || continue
      local change_type final_delta
      change_type="$(git -C "$ROOT" diff --name-status "$upstream_base..$merge_commit^2" -- "$changed_path" | cut -f1 | paste -sd ',' -)"
      final_delta="$(git -C "$ROOT" diff --name-status "$merge_commit^2..HEAD" -- "$changed_path" | tr '\t' ':' | paste -sd ',' -)"
      if [[ -z "$final_delta" ]]; then
        printf '%s\t%s\t无差异\t不适用\t与origin/main逐字一致\tgit diff --quiet %s..HEAD -- %s: 通过\t通过\n' \
          "$changed_path" "$change_type" "$merge_commit^2" "$changed_path"
      else
        printf '%s\t%s\t%s\tTODO\tTODO\tTODO\tTODO\n' \
          "$changed_path" "$change_type" "$final_delta"
      fi
    done < "$evidence_dir/upstream-changed-path-list.txt" >> "$evidence_dir/upstream-paths-review.tsv"
    printf '提交SHA\t提交说明\t覆盖的上游路径\t维护记录依据\t为何不丢失上游行为\t专项验证命令与结果\t结论\n' \
      > "$evidence_dir/post-merge-overrides-review.tsv"
    while IFS=$'\t' read -r override_sha override_subject; do
      [[ -n "$override_sha" ]] || continue
      while IFS= read -r override_path; do
        [[ -n "$override_path" ]] || continue
        if git -C "$ROOT" diff --quiet "$merge_commit^2..HEAD" -- "$override_path"; then
          printf '%s\t%s\t%s\t不适用\t最终已恢复为origin/main\tgit diff --quiet %s..HEAD -- %s: 通过\t通过\n' \
            "$override_sha" "$override_subject" "$override_path" "$merge_commit^2" "$override_path"
        else
          printf '%s\t%s\t%s\tTODO\tTODO\tTODO\tTODO\n' \
            "$override_sha" "$override_subject" "$override_path"
        fi
      done < <(git -C "$ROOT" diff-tree --no-commit-id --name-only -r "$override_sha" -- \
        $(<"$evidence_dir/upstream-changed-path-list.txt"))
    done < "$evidence_dir/post-merge-upstream-overrides.tsv" >> "$evidence_dir/post-merge-overrides-review.tsv"
    printf '上游提交\t候选产品面\t入口或涉及路径\t产品决定\t用户决定或维护记录依据\t实现方式\t专项验证命令与结果\t结论\n' \
      > "$evidence_dir/product-surface-decisions.tsv"
    while IFS=$'\t' read -r surface_sha surface_subject; do
      local surface_paths
      surface_paths="$(git -C "$ROOT" show --format= --name-only "$surface_sha" -- \
        'frontend/src/**/*.vue' 'frontend/src/router/**' 'frontend/src/i18n/**' \
        'backend/internal/handler/dto/**' 'backend/ent/schema/**' 'backend/migrations/**' \
        | sed '/^$/d' | sort -u | paste -sd ',' -)"
      [[ -n "$surface_paths" ]] || continue
      printf '%s\t%s\t%s\tTODO\tTODO\tTODO\tTODO\tTODO\n' \
        "$surface_sha" "$surface_subject" "$surface_paths"
    done < "$evidence_dir/upstream-commits.tsv" >> "$evidence_dir/product-surface-decisions.tsv"
  fi
  printf '[fork-sync-deploy] Fork 文件数：%s；证据目录：%s\n' \
    "$(wc -l < "$evidence_dir/fork-delta-name-status.txt" | tr -d ' ')" "$evidence_dir"
}

find_reusable_review_evidence() {
  local label="$1" candidate expected_head expected_source
  expected_head="$(git -C "$ROOT" rev-parse HEAD)"
  expected_source="$(git -C "$ROOT" rev-parse "$REMOTE/$MAIN_BRANCH")"
  while IFS= read -r candidate; do
    [[ -f "$candidate/review-head.txt" && -f "$candidate/review-source.txt" ]] || continue
    [[ "$(<"$candidate/review-head.txt")" = "$expected_head" ]] || continue
    [[ "$(<"$candidate/review-source.txt")" = "$expected_source" ]] || continue
    printf '%s\n' "$candidate"
    return 0
  done < <(find "$ROOT/tmp/fork-maintenance/reviews" -maxdepth 1 -type d -name "*-$label" -print | sort -r)
  return 1
}

find_latest_main_merge() {
  local candidate second_parent
  while IFS= read -r candidate; do
    second_parent="$(git -C "$ROOT" rev-parse "$candidate^2" 2>/dev/null || true)"
    [[ -n "$second_parent" ]] || continue
    if git -C "$ROOT" merge-base --is-ancestor "$second_parent" "$REMOTE/$MAIN_BRANCH"; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done < <(git -C "$ROOT" rev-list --first-parent --merges HEAD)
  return 1
}

validate_upstream_review() {
  local evidence_dir="$1" expected reviewed
  [[ -s "$evidence_dir/upstream-commits.tsv" ]] || die "无法建立本轮 origin/main 上游提交清单"
  expected="$(wc -l < "$evidence_dir/upstream-commits.tsv" | tr -d ' ')"
  reviewed="$(( $(wc -l < "$evidence_dir/upstream-capabilities.tsv" | tr -d ' ') - 1 ))"
  [[ "$reviewed" -eq "$expected" ]] || die "上游能力审查覆盖不完整：应审 $expected 项，实际 $reviewed 项"
  if rg -n $'\tTODO(\t|$)|\t(未审查|待确认|失败)(\t|$)' "$evidence_dir/upstream-capabilities.tsv"; then
    die "上游能力审查表存在未完成或失败条目；必须逐提交核对 origin/main 能力后才能通过"
  fi
  [[ -s "$evidence_dir/upstream-paths-review.tsv" ]] || die "缺少上游变更路径逐文件审查表"
  [[ -s "$evidence_dir/post-merge-overrides-review.tsv" ]] || die "缺少 merge 后上游路径覆盖审查表"
  awk -F '\t' 'NF != 7 { exit 1 }' "$evidence_dir/upstream-paths-review.tsv" \
    || die "上游路径审查表列数错误；检查字段中是否混入制表符"
  awk -F '\t' 'NF != 7 { exit 1 }' "$evidence_dir/post-merge-overrides-review.tsv" \
    || die "merge 后覆盖审查表列数错误；检查字段中是否混入制表符"
  awk -F '\t' 'NF != 8 { exit 1 }' "$evidence_dir/product-surface-decisions.tsv" \
    || die "可见产品面决策表列数错误；检查字段中是否混入制表符"
  if [[ -s "$evidence_dir/final-deleted-upstream-paths.tsv" ]]; then
    cat "$evidence_dir/final-deleted-upstream-paths.tsv" >&2
    die "最终 fork 删除了 origin/main 文件；必须恢复上游文件，不能用文档或测试豁免整文件删除"
  fi
  if rg -n $'\tTODO(\t|$)|\t(未审查|待确认|失败)(\t|$)' \
    "$evidence_dir/upstream-paths-review.tsv" "$evidence_dir/post-merge-overrides-review.tsv"; then
    die "上游路径或 merge 后覆盖审查存在未完成项；每个最终偏离都必须关联维护记录并专项验证"
  fi
  [[ -s "$evidence_dir/product-surface-decisions.tsv" ]] || die "缺少可见产品面决策表"
  if rg -n $'\tTODO(\t|$)|\t(未审查|待确认|失败)(\t|$)' "$evidence_dir/product-surface-decisions.tsv"; then
    die "可见产品面仍有未判定项；不得默认接受上游新增 UI、路由、设置或运营配置"
  fi
}

validate_non_fork_review() {
  local evidence_dir="$1" expected reviewed
  expected="$(collect_requirement_records | wc -l | tr -d ' ')"
  reviewed="$(( $(wc -l < "$evidence_dir/non-fork-requirements.tsv" | tr -d ' ') - 1 ))"
  [[ "$reviewed" -eq "$expected" ]] || die "非 fork 审查覆盖不完整：文档记录 $expected 项，实际审查 $reviewed 项"
  if rg -n $'\tTODO(\t|$)|\t(未审查|待确认|失败)(\t|$)' "$evidence_dir/non-fork-requirements.tsv"; then
    die "非 fork 审查表存在未完成或失败条目；必须逐条核对维护记录后才能通过"
  fi
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

  local reusable_evidence_dir
  reusable_evidence_dir="$(find_reusable_review_evidence "review-$REVIEW" || true)"
  if [[ -n "$reusable_evidence_dir" ]]; then
    EVIDENCE_DIR="$reusable_evidence_dir"
    printf '[fork-sync-deploy] 复用同一提交的审查目录：%s\n' "$EVIDENCE_DIR"
  else
    write_review_evidence "review-$REVIEW"
  fi
  validate_upstream_review "$EVIDENCE_DIR"
  validate_non_fork_review "$EVIDENCE_DIR"
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
