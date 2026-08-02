#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import re
import shlex
import subprocess
import sys
from datetime import datetime
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
FORK_DOC = ROOT / "docs" / "FORK_MAINTENANCE_CN.md"
STATE_DIR = ROOT / "tools" / "fork-maintenance" / "production-state"
LOGIN_AGREEMENT_STATE = STATE_DIR / "login-agreement.json"


FORK_DOC_REL = "docs/FORK_MAINTENANCE_CN.md"

IGNORED_RECORD_PATTERNS = (
    FORK_DOC_REL,
    "backend/internal/web/dist/",
    "tmp/",
)

VERIFY_SEARCHES = (
    ("favicon helper", "applySiteIcons|resolveIconMimeType", "frontend/src"),
    ("static logo icon", 'rel="icon".*/logo\\.svg|/logo\\.svg.*rel="icon"', "frontend/index.html"),
    ("fork record", "2026-05-31 线上 OAuth|favicon-20260531210858|bg-20260531", "docs/FORK_MAINTENANCE_CN.md"),
    ("account runtime-state refresh binding", '@runtime-state-updated="handleAccountRuntimeStateUpdated"', "frontend/src/views/admin/AccountsView.vue"),
    ("admin usage user notes mapping", "UserFromServiceAdmin\\(l\\.User\\)", "backend/internal/handler/dto/mappers.go"),
)

LOCAL_PATCH_RECORD_HEADING = "## 本地补丁记录"
POST_LOCAL_PATCH_RECORD_HEADING = "## 同步官方版本后的复查流程"
LOCAL_PATCH_TABLE_HEADER = "| 日期 | 问题 | 本地修复 | 验证 | 后续同步复查 |"
LOCAL_PATCH_TABLE_SEPARATOR = "| --- | --- | --- | --- | --- |"


class CommandError(RuntimeError):
    pass


def run(args: list[str], *, cwd: Path = ROOT, check: bool = True, capture: bool = True) -> subprocess.CompletedProcess:
    proc = subprocess.run(
        args,
        cwd=cwd,
        check=False,
        text=True,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.STDOUT if capture else None,
    )
    if check and proc.returncode != 0:
        output = proc.stdout or ""
        raise CommandError(f"command failed ({proc.returncode}): {' '.join(args)}\n{output}")
    return proc


def git(args: list[str], *, check: bool = True) -> str:
    return run(["git", *args], check=check).stdout or ""


def ref_exists(ref: str) -> bool:
    return run(["git", "rev-parse", "--verify", "--quiet", ref], check=False).returncode == 0


def default_base() -> str | None:
    for ref in ("upstream/main", "upstream/master", "origin/main", "origin/master"):
        if ref_exists(ref):
            return ref
    return None


def resolve_base(explicit: str | None) -> str:
    base = explicit or default_base()
    if not base:
        raise CommandError(
            "No upstream base ref found. Add the official upstream remote or pass --base <ref>.\n"
            "Example: git remote add upstream https://github.com/Wei-Shaw/sub2api.git"
        )
    if not ref_exists(base):
        raise CommandError(f"Base ref does not exist: {base}")
    return base


def changed_files_from_status() -> set[str]:
    output = git(["status", "--porcelain=v1", "--untracked-files=all"])
    files: set[str] = set()
    for line in output.splitlines():
        if not line:
            continue
        path = line[3:]
        if " -> " in path:
            path = path.split(" -> ", 1)[1]
        files.add(path)
    return files


def changed_files_from_index() -> set[str]:
    output = git(["diff", "--cached", "--name-only", "--diff-filter=ACMRTD"])
    return {line.strip() for line in output.splitlines() if line.strip()}


def changed_files_from_base(base: str) -> set[str]:
    output = git(["diff", "--name-only", f"{base}...HEAD"])
    files = {line.strip() for line in output.splitlines() if line.strip()}
    files.update(changed_files_from_status())
    return files


def path_matches(path: str, pattern: str) -> bool:
    return path == pattern or path.startswith(pattern)


def is_record_candidate(path: str) -> bool:
    return not any(path_matches(path, pattern) for pattern in IGNORED_RECORD_PATTERNS)


def doc_changed() -> bool:
    return FORK_DOC_REL in changed_files_from_status()


def doc_staged() -> bool:
    return FORK_DOC_REL in changed_files_from_index()


def upstream_sync_in_progress() -> bool:
    git_dir = Path(git(["rev-parse", "--git-dir"]).strip())
    if not git_dir.is_absolute():
        git_dir = ROOT / git_dir
    return any(
        (git_dir / marker).exists()
        for marker in ("MERGE_HEAD", "REBASE_HEAD", "CHERRY_PICK_HEAD", "rebase-merge", "rebase-apply")
    )


def print_table(rows: list[tuple[str, str, str]]) -> None:
    if not rows:
        print("No fork-maintenance candidates found.")
        return
    widths = [max(len(row[i]) for row in rows + [("Path", "Kind", "Action")]) for i in range(3)]
    header = ("Path", "Kind", "Action")
    print("  ".join(header[i].ljust(widths[i]) for i in range(3)))
    print("  ".join("-" * widths[i] for i in range(3)))
    for row in rows:
        print("  ".join(row[i].ljust(widths[i]) for i in range(3)))


def cmd_inventory(args: argparse.Namespace) -> int:
    base = resolve_base(args.base)
    files = sorted(changed_files_from_base(base))
    rows = []
    for path in files:
        kind = "record-candidate" if is_record_candidate(path) else "ignored"
        action = "record in docs/FORK_MAINTENANCE_CN.md" if kind == "record-candidate" else "no record required"
        rows.append((path, kind, action))
    print(f"Base: {base}")
    print_table(rows)
    return 0


def build_record_text(files: list[str], title: str, *, auto: bool) -> str:
    today = datetime.now().date().isoformat()
    if auto:
        lines = [
            "",
            f"### {today}: {title}",
            "",
            "**自动记录：**",
            "",
            "- 本条由 pre-commit 护栏根据本次 staged 文件自动生成。",
            "- 提交后请补充业务目的、验证结果和同步官方后的复查方式；不要长期保留空泛记录。",
            "",
            "**涉及文件：**",
            "",
        ]
    else:
        lines = [
            "",
            f"### {today}: {title}",
            "",
            "**现象：**",
            "",
            "- TODO: 描述用户看到的问题、页面、接口或日志。",
            "",
            "**原因：**",
            "",
            "- TODO: 描述定位到的根因，尽量引用具体文件和 key。",
            "",
            "**修改：**",
            "",
        ]
    lines.extend(f"- `{path}`" for path in files)
    lines.extend(
        [
            "",
            "**验证：**",
            "",
            "```bash",
            "TODO: 填写验证命令",
            "```",
            "",
            "**同步官方后的复查：**",
            "",
            "- TODO: 说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。",
            "",
        ]
    )
    return "\n".join(lines)


def sort_local_patch_table(text: str) -> str:
    start_marker = f"\n{LOCAL_PATCH_RECORD_HEADING}\n"
    end_marker = f"\n{POST_LOCAL_PATCH_RECORD_HEADING}"
    start = text.find(start_marker)
    end = text.find(end_marker, start + len(start_marker)) if start != -1 else -1
    if start == -1 or end == -1:
        return text

    section = text[start:end]
    lines = section.splitlines()
    header_idx = -1
    separator_idx = -1
    for idx, line in enumerate(lines):
        if line.strip() == LOCAL_PATCH_TABLE_HEADER:
            header_idx = idx
            if idx + 1 < len(lines) and lines[idx + 1].strip() == LOCAL_PATCH_TABLE_SEPARATOR:
                separator_idx = idx + 1
            break
    if header_idx == -1 or separator_idx == -1:
        return text

    row_pattern = re.compile(r"^\| (?P<date>\d{4}-\d{2}-\d{2}) \|")
    rows: list[tuple[str, int, str]] = []
    row_indexes: list[int] = []
    for idx in range(separator_idx + 1, len(lines)):
        match = row_pattern.match(lines[idx])
        if match:
            rows.append((match.group("date"), len(rows), lines[idx]))
            row_indexes.append(idx)

    if len(rows) < 2:
        return text

    sorted_rows = [row for _, _, row in sorted(rows, key=lambda item: (item[0], item[1]))]
    for idx, row in zip(row_indexes, sorted_rows):
        lines[idx] = row

    sorted_section = "\n".join(lines)
    return text[:start] + sorted_section + "\n" + text[end:]


def sort_fork_doc() -> None:
    text = FORK_DOC.read_text(encoding="utf-8")
    sorted_text = sort_local_patch_table(text)
    if sorted_text != text:
        FORK_DOC.write_text(sorted_text, encoding="utf-8")


def append_record(files: list[str], title: str, *, auto: bool) -> None:
    text = FORK_DOC.read_text(encoding="utf-8")
    marker = f"\n{POST_LOCAL_PATCH_RECORD_HEADING}"
    insert = build_record_text(files, title, auto=auto)
    if marker not in text:
        raise CommandError(f"cannot find insertion marker in {FORK_DOC}: {marker.strip()}")
    FORK_DOC.write_text(sort_local_patch_table(text.replace(marker, insert + marker, 1)), encoding="utf-8")


def cmd_check_doc(args: argparse.Namespace) -> int:
    if upstream_sync_in_progress():
        print("Upstream sync in progress; fork maintenance auto-record skipped.")
        return 0
    candidates = sorted(path for path in changed_files_from_index() if is_record_candidate(path))
    if not candidates:
        print("No fork-maintenance record candidates staged.")
        return 0
    if doc_staged():
        print("Fork maintenance doc staged with local changes.")
        for path in candidates:
            print(f"  - {path}")
        return 0
    append_record(candidates, "自动记录本地改动", auto=True)
    git(["add", FORK_DOC_REL])
    print("Auto-recorded fork maintenance entry for staged local changes:")
    for path in candidates:
        print(f"  - {path}")
    return 0


def cmd_record(args: argparse.Namespace) -> int:
    files = sorted(path for path in changed_files_from_status() if is_record_candidate(path))
    if not files:
        print("No fork-maintenance record candidates changed; nothing to record.")
        return 0
    title = args.title or "待补充 fork 本地改动"
    if args.dry_run:
        print(build_record_text(files, title, auto=False))
        return 0
    append_record(files, title, auto=False)
    print(f"Appended fork maintenance record template to {FORK_DOC}")
    return 0


def cmd_sort_doc(args: argparse.Namespace) -> int:
    sort_fork_doc()
    print(f"Sorted local patch records in {FORK_DOC}")
    return 0


def cmd_snapshot(args: argparse.Namespace) -> int:
    base = resolve_base(args.base)
    timestamp = datetime.now().strftime("%Y%m%d%H%M%S")
    out_dir = Path(args.output or ROOT / "tmp" / "fork-maintenance" / timestamp)
    out_dir.mkdir(parents=True, exist_ok=True)
    patch_path = out_dir / "fork-diff.patch"
    files_path = out_dir / "fork-files.txt"
    patch = git(["diff", "--binary", f"{base}...HEAD"])
    worktree = git(["diff", "--binary"])
    staged = git(["diff", "--binary", "--cached"])
    patch_path.write_text(
        "\n".join(
            part for part in (
                f"# base: {base}\n# created_at: {datetime.now().isoformat(timespec='seconds')}\n",
                patch,
                "\n# --- staged worktree diff ---\n",
                staged,
                "\n# --- unstaged worktree diff ---\n",
                worktree,
            )
            if part
        ),
        encoding="utf-8",
    )
    files = sorted(changed_files_from_base(base))
    files_path.write_text("\n".join(files) + "\n", encoding="utf-8")
    print(f"Snapshot written to {out_dir}")
    print(f"  patch: {patch_path}")
    print(f"  files: {files_path}")
    return 0


def cmd_verify_after_upstream(args: argparse.Namespace) -> int:
    failures = 0
    for label, pattern, path in VERIFY_SEARCHES:
        proc = run(["rg", "-n", pattern, path], check=False)
        if proc.returncode == 0:
            print(f"[ok] {label}")
        else:
            failures += 1
            print(f"[fail] {label}: pattern not found: {pattern} in {path}")
    frontend_dir = ROOT / "frontend"
    vitest = frontend_dir / "node_modules" / ".bin" / "vitest"
    vite = frontend_dir / "node_modules" / ".bin" / "vite"
    test_cmds = []
    vitest_cmd = [str(vitest)] if vitest.exists() else ["pnpm", "exec", "vitest"]
    favicon_spec = ROOT / "frontend" / "src" / "__tests__" / "app-favicon.spec.ts"
    if favicon_spec.exists():
        test_cmds.append([*vitest_cmd, "run", "src/__tests__/app-favicon.spec.ts"])
    else:
        print("[skip] favicon test: frontend/src/__tests__/app-favicon.spec.ts not found")
    native_dialog_spec = ROOT / "frontend" / "src" / "components" / "common" / "__tests__" / "nativeDialogUsage.spec.ts"
    if native_dialog_spec.exists():
        test_cmds.append([*vitest_cmd, "run", "src/components/common/__tests__/nativeDialogUsage.spec.ts"])
    if not args.skip_build:
        test_cmds.append([str(vite), "build"] if vite.exists() else ["pnpm", "run", "build"])
    for cmd in test_cmds:
        print(f"[run] {' '.join(cmd)}")
        proc = run(cmd, cwd=frontend_dir, check=False, capture=False)
        if proc.returncode != 0:
            failures += 1
            print(f"[fail] {' '.join(cmd)}")
        else:
            print(f"[ok] {' '.join(cmd)}")
    return 1 if failures else 0


def quote_remote(command: str) -> str:
    return shlex.quote(command)


def remote(host: str, command: str, *, apply: bool) -> None:
    ssh_cmd = f"ssh -o BatchMode=yes -o ConnectTimeout=10 {shlex.quote(host)} {quote_remote(command)}"
    if not apply:
        print(f"[dry-run] {ssh_cmd}")
        return
    print(f"[apply] {ssh_cmd}")
    run(["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", host, command], check=True, capture=False)


def scp_to_remote(host: str, local: Path, remote_path: str, *, apply: bool) -> None:
    scp_cmd = f"scp -q -o BatchMode=yes -o ConnectTimeout=10 {shlex.quote(str(local))} {shlex.quote(host + ':' + remote_path)}"
    if not apply:
        print(f"[dry-run] {scp_cmd}")
        return
    print(f"[apply] {scp_cmd}")
    run(["scp", "-q", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", str(local), f"{host}:{remote_path}"], check=True, capture=False)


def prepare_static_restore_assets() -> dict[str, Path]:
    work_dir = ROOT / "tmp" / "fork-maintenance" / "restore-assets"
    work_dir.mkdir(parents=True, exist_ok=True)
    logo_png = ROOT / "frontend" / "public" / "logo.png"
    logo_svg = ROOT / "frontend" / "public" / "logo.svg"
    favicon_ico = work_dir / "favicon.ico"
    if not logo_png.exists():
        raise CommandError(f"missing logo asset: {logo_png}")
    if not logo_svg.exists():
        raise CommandError(f"missing logo asset: {logo_svg}")
    try:
        from PIL import Image
    except ImportError as exc:
        raise CommandError(
            "Pillow is required to regenerate favicon.ico from the current logo.png. "
            "Install Pillow or run from the bundled workspace where PIL is available."
        ) from exc
    src = Image.open(logo_png).convert("RGBA")
    src.save(favicon_ico, sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (180, 180)])
    return {
        "logo.png": logo_png,
        "logo.svg": logo_svg,
        "favicon.ico": favicon_ico,
    }


def render_login_agreement_sql(state: dict) -> str:
    docs = json.dumps(state["login_agreement_documents"], ensure_ascii=False)
    enabled = "true" if state.get("login_agreement_enabled") else "false"
    mode = str(state.get("login_agreement_mode") or "modal")
    updated_at = str(state.get("login_agreement_updated_at") or datetime.now().date().isoformat())
    values = {
        "login_agreement_enabled": enabled,
        "login_agreement_mode": mode,
        "login_agreement_updated_at": updated_at,
        "login_agreement_documents": docs,
    }
    statements = []
    for key, value in values.items():
        statements.append(
            "INSERT INTO settings (key, value) "
            f"VALUES ({sql_literal(key)}, {sql_literal(value)}) "
            "ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;"
        )
    return "\n".join(statements) + "\n"


def sql_literal(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def cmd_reapply_production_state(args: argparse.Namespace) -> int:
    apply = bool(args.apply)
    host = args.host
    version = args.version
    if not LOGIN_AGREEMENT_STATE.exists():
        raise CommandError(f"missing state file: {LOGIN_AGREEMENT_STATE}")
    state = json.loads(LOGIN_AGREEMENT_STATE.read_text(encoding="utf-8"))

    print("Production restore plan:")
    print(f"  host: {host}")
    print(f"  mode: {'apply' if apply else 'dry-run'}")
    print(f"  static version: {version}")
    assets = prepare_static_restore_assets()
    print(f"  favicon source: regenerated from {assets['logo.png']}")

    remote(host, "mkdir -p /tmp/fork-maintenance", apply=apply)
    for name, local in assets.items():
        if not local.exists():
            raise CommandError(f"missing local asset: {local}")
        scp_to_remote(host, local, f"/tmp/fork-maintenance/{name}", apply=apply)

    static_cmd = f"""set -eu
cd /opt/51token-home
TS=$(date +%Y%m%d%H%M%S)
cp index.html "index.html.bak-fork-maint-$TS"
cp favicon.ico "favicon.ico.bak-fork-maint-$TS" 2>/dev/null || true
cp logo.png "logo.png.bak-fork-maint-$TS" 2>/dev/null || true
cp logo.svg "logo.svg.bak-fork-maint-$TS" 2>/dev/null || true
cp /tmp/fork-maintenance/logo.png ./logo.png
cp /tmp/fork-maintenance/logo.svg ./logo.svg
cp /tmp/fork-maintenance/favicon.ico ./favicon.ico
python3 - <<'PY'
from pathlib import Path
import re
p = Path("index.html")
s = p.read_text()
s = re.sub(r'\\s*<link\\b[^>]*rel="(?:icon|alternate icon|apple-touch-icon)"[^>]*>\\s*', '\\n', s)
icons = '    <link rel="icon" type="image/svg+xml" href="/logo.svg?v={version}" />\\n    <link rel="apple-touch-icon" href="/logo.png?v={version}" />'
marker = '<meta charset="UTF-8" />'
if marker in s:
    s = s.replace(marker, marker + '\\n' + icons, 1)
else:
    s = s.replace('</head>', icons + '\\n</head>', 1)
p.write_text(s)
PY
docker restart cf-origin-ssl >/dev/null
docker exec cf-origin-ssl sh -lc "grep -o '<link rel=\\"[^\\"]*\\"[^>]*>' /srv/51token-home/index.html | grep -E 'icon|apple-touch' || true"
"""
    remote(host, static_cmd, apply=apply)

    sql = render_login_agreement_sql(state)
    sql_path = ROOT / "tmp" / "fork-maintenance" / "login-agreement.sql"
    sql_path.parent.mkdir(parents=True, exist_ok=True)
    sql_path.write_text(sql, encoding="utf-8")
    scp_to_remote(host, sql_path, "/tmp/fork-maintenance/login-agreement.sql", apply=apply)
    db_cmd = """set -eu
docker cp /tmp/fork-maintenance/login-agreement.sql sub2api-postgres:/tmp/login-agreement.sql
for db in sub2api sub2api_ap1 sub2api_test; do
  docker exec sub2api-postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$1" -v ON_ERROR_STOP=1 -f /tmp/login-agreement.sql' sh "$db"
done
"""
    remote(host, db_cmd, apply=apply)

    verify_cmd = f"""set -eu
curl -fsS https://ai.upit.top/health
curl -k -fsS 'https://ai.upit.top/?v={version}' -o /tmp/fork-maintenance/home.html
if grep -q 'favicon.ico\\|alternate icon' /tmp/fork-maintenance/home.html; then
  echo 'old favicon reference still present' >&2
  exit 1
fi
grep -o '<link rel="[^"]*"[^>]*>' /tmp/fork-maintenance/home.html | grep -E 'icon|apple-touch'
curl -fsS http://127.0.0.1:8081/api/v1/settings/public | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; print("primary_docs", len(d.get("login_agreement_documents") or []), d.get("login_agreement_enabled"), d.get("login_agreement_mode"))'
curl -fsS http://127.0.0.1:8082/api/v1/settings/public | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; print("ap1_docs", len(d.get("login_agreement_documents") or []), d.get("login_agreement_enabled"), d.get("login_agreement_mode"))'
"""
    remote(host, verify_cmd, apply=apply)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Fork maintenance inventory, checks, snapshots, and production restore helpers.")
    sub = parser.add_subparsers(dest="command", required=True)

    inventory = sub.add_parser("inventory", help="List fork-maintenance candidate changes relative to a base ref.")
    inventory.add_argument("--base", help="Base ref, defaults to upstream/main when available.")
    inventory.set_defaults(func=cmd_inventory)

    check_doc = sub.add_parser("check-doc", help="Auto-record staged local changes in the maintenance doc.")
    check_doc.set_defaults(func=cmd_check_doc)

    record = sub.add_parser("record", help="Append a maintenance record template for current record-candidate changes.")
    record.add_argument("--title", help="Record title.")
    record.add_argument("--dry-run", action="store_true", help="Print the generated record without editing the doc.")
    record.set_defaults(func=cmd_record)

    sort_doc = sub.add_parser("sort-doc", help="Sort local patch records in the maintenance doc by date.")
    sort_doc.set_defaults(func=cmd_sort_doc)

    snapshot = sub.add_parser("snapshot", help="Export fork diff and file list for review or manual reapplication.")
    snapshot.add_argument("--base", help="Base ref, defaults to upstream/main when available.")
    snapshot.add_argument("--output", help="Output directory, defaults to tmp/fork-maintenance/<timestamp>.")
    snapshot.set_defaults(func=cmd_snapshot)

    verify = sub.add_parser("verify-after-upstream", help="Run fork-specific searches and tests after merging/rebasing upstream.")
    verify.add_argument("--skip-build", action="store_true", help="Skip frontend production build.")
    verify.set_defaults(func=cmd_verify_after_upstream)

    restore = sub.add_parser("reapply-production-state", help="Reapply non-Git production state. Dry-run by default.")
    restore.add_argument("--host", default="51tokens", help="SSH host alias.")
    restore.add_argument("--version", default="bg-20260531", help="Static asset cache-busting version.")
    restore.add_argument("--apply", action="store_true", help="Actually modify the remote host.")
    restore.set_defaults(func=cmd_reapply_production_state)

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        return args.func(args)
    except CommandError as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
