#!/usr/bin/env python3
from __future__ import annotations

import copy
import importlib.util
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import unittest
from unittest.mock import patch
from argparse import Namespace
import tempfile


SCRIPT_PATH = Path(__file__).with_name("fork_maintenance.py")
SPEC = importlib.util.spec_from_file_location("fork_maintenance", SCRIPT_PATH)
assert SPEC is not None
fork_maintenance = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(fork_maintenance)


class ForkMaintenanceDocSortTest(unittest.TestCase):
    def capture_restore_commands(self) -> list[str]:
        commands = []
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            asset = root / "logo.png"
            asset.touch()
            with patch.object(fork_maintenance, "ROOT", root), patch.object(
                fork_maintenance, "prepare_static_restore_assets", return_value={"logo.png": asset}
            ), patch.object(fork_maintenance, "scp_to_remote"), patch.object(
                fork_maintenance, "remote", side_effect=lambda host, command, **kwargs: commands.append(command)
            ):
                fork_maintenance.cmd_reapply_production_state(Namespace(apply=False, host="unused", version="test"))
        return commands

    def test_restore_targets_only_active_databases_and_public_sites(self) -> None:
        commands = self.capture_restore_commands()
        plan = "\n".join(commands)
        self.assertIn("for db in sub2api sub2api_ap1 sub2api_3dbooks;", plan)
        for site in ["ai.upit.top", "api.upit.top", "a1.upit.top", "ap1.upit.top", "ai.3dbooks.top", "api.3dbooks.top"]:
            self.assertIn("https://" + site + "/health", plan)
        for retired in ["sub2api_test", "test.upit.top", "a2t.upit.top"]:
            self.assertNotIn(retired, plan)

    def run_static_restore_fragment(
        self, command: str, *, fail_asset: str = "", existing_assets: tuple[str, ...]
    ) -> tuple[subprocess.CompletedProcess[str], list[str], dict[str, str]]:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            staging = root / "staging"
            staging.mkdir()
            for name in ("logo.png", "logo.svg", "favicon.ico"):
                (staging / name).write_text("replacement:" + name)
            originals = {name: "original:" + name for name in existing_assets}
            for name, content in originals.items():
                (root / name).write_text(content)
            stub = root / "bin"
            stub.mkdir()
            cp = stub / "cp"
            cp.write_text("""#!/bin/sh
kind=replace
case "$2" in *.bak-fork-maint-*) kind=backup;; esac
printf '%s|%s\n' "$kind" "$1" >> "$CP_LOG"
if [ "$kind" = backup ] && [ "$1" = "$FAIL_ASSET" ]; then
  exit 23
fi
exec /bin/cp "$@"
""")
            cp.chmod(0o755)
            log = root / "cp.log"
            fragment, separator, _ = command.partition("python3 - <<'PY'")
            self.assertTrue(separator, "extract the real static backup/replacement segment")
            self.assertIn("cd /opt/51token-home", fragment)
            fragment = fragment.replace("cd /opt/51token-home", "cd " + shlex.quote(str(root)))
            fragment = fragment.replace("/tmp/fork-maintenance/", str(staging) + "/")
            result = subprocess.run(
                ["bash", "-c", fragment],
                env={**os.environ, "PATH": str(stub) + os.pathsep + os.environ.get("PATH", ""),
                     "CP_LOG": str(log), "FAIL_ASSET": fail_asset},
                capture_output=True, text=True, timeout=10,
            )
            events = log.read_text().splitlines() if log.exists() else []
            contents = {
                name: (root / name).read_text()
                for name in ("index.html", "favicon.ico", "logo.png", "logo.svg")
                if (root / name).is_file()
            }
            return result, events, contents

    def test_static_restore_backup_failure_aborts_before_any_replacement(self) -> None:
        command = next(c for c in self.capture_restore_commands() if "cd /opt/51token-home" in c)
        assets = ("index.html", "favicon.ico", "logo.png", "logo.svg")
        for asset in assets:
            with self.subTest(failed_backup=asset):
                result, events, contents = self.run_static_restore_fragment(
                    command, fail_asset=asset, existing_assets=assets
                )
                self.assertNotEqual(result.returncode, 0, result.stderr)
                self.assertIn("backup|" + asset, events)
                self.assertFalse(any(e.startswith("replace|") for e in events), events)
                self.assertEqual(contents, {name: "original:" + name for name in assets})

    def test_static_restore_requires_index_but_allows_missing_optional_assets(self) -> None:
        command = next(c for c in self.capture_restore_commands() if "cd /opt/51token-home" in c)
        for existing in ((), ("index.html",), ("index.html", "favicon.ico", "logo.png", "logo.svg")):
            with self.subTest(existing=existing):
                result, events, contents = self.run_static_restore_fragment(command, existing_assets=existing)
                if not existing:
                    self.assertNotEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(events, [], "missing index must stop even before backup copies")
                    continue
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(
                    [e for e in events if e.startswith("backup|")],
                    ["backup|" + name for name in existing],
                )
                self.assertEqual(sum(e.startswith("replace|") for e in events), 3)
                self.assertEqual(contents["index.html"], "original:index.html")
                for name in ("favicon.ico", "logo.png", "logo.svg"):
                    self.assertEqual(contents[name], "replacement:" + name)

    def verification_programs(self) -> tuple[str, list[str]]:
        command = next(c for c in self.capture_restore_commands() if "for url in https://" in c)
        programs = []
        for line in command.splitlines():
            if "| python3 -c " in line:
                tokens = shlex.split(line)
                index = tokens.index("python3")
                self.assertEqual(tokens[index + 1], "-c")
                programs.append(tokens[index + 2])
        self.assertEqual(len(programs), 4, "one health check and checks for all three settings ports")
        return programs[0], programs[1:]

    def test_restored_settings_check_rejects_wrong_state(self) -> None:
        _, programs = self.verification_programs()
        state = json.loads(fork_maintenance.LOGIN_AGREEMENT_STATE.read_text())
        self.assertTrue(state["login_agreement_documents"], "fixture must exercise nonempty documents")
        for case in ("exact", "disabled", "wrong_mode", "empty_documents", "wrong_body"):
            data = copy.deepcopy(state)
            if case == "disabled":
                data["login_agreement_enabled"] = False
            elif case == "wrong_mode":
                data["login_agreement_mode"] = "wrong"
            elif case == "empty_documents":
                data["login_agreement_documents"] = []
            elif case == "wrong_body":
                data["login_agreement_documents"][0]["content_md"] += "\nchanged"
            for port, code in zip((8081, 8082, 8083), programs):
                with self.subTest(case=case, port=port):
                    result = subprocess.run(
                        [sys.executable, "-c", code], input=json.dumps({"data": data}),
                        capture_output=True, text=True, timeout=10,
                    )
                    if case == "exact":
                        self.assertEqual(result.returncode, 0, result.stderr)
                    else:
                        self.assertNotEqual(result.returncode, 0, "incorrect settings accepted")

    def test_health_check_rejects_http_200_with_unhealthy_body(self) -> None:
        code, _ = self.verification_programs()
        # Feed successful HTTP response bodies directly; curl -f alone cannot reject them.
        for status in ("ok", "error", "degraded", None):
            with self.subTest(status=status):
                body = {} if status is None else {"status": status}
                result = subprocess.run(
                    [sys.executable, "-c", code], input=json.dumps(body),
                    capture_output=True, text=True, timeout=10,
                )
                if status == "ok":
                    self.assertEqual(result.returncode, 0, result.stderr)
                else:
                    self.assertNotEqual(result.returncode, 0, "unhealthy HTTP 200 body accepted")

    def test_restore_health_curl_failure_aborts_even_with_valid_ok_body(self) -> None:
        command = next(c for c in self.capture_restore_commands() if "for url in https://" in c)
        health_loop, separator, _ = command.partition("\ndone\n")
        self.assertTrue(separator, "extract actual verification preamble and health loop")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            curl = root / "curl"
            curl.write_text("#!/bin/sh\nprintf 'called\\n' >> \"$CURL_LOG\"\nprintf '{\"status\":\"ok\"}\\n'\nexit 22\n")
            curl.chmod(0o755)
            marker = root / "continued"
            log = root / "curl.log"
            fragment = health_loop + separator + "touch " + shlex.quote(str(marker))
            result = subprocess.run(
                ["bash", "-c", fragment],
                env={**os.environ, "PATH": str(root) + os.pathsep + os.environ.get("PATH", ""),
                     "CURL_LOG": str(log)},
                capture_output=True, text=True, timeout=10,
            )
            self.assertEqual(result.returncode, 22, result.stderr)
            self.assertEqual(log.read_text().splitlines(), ["called"], "must stop on first failed curl")
            self.assertFalse(marker.exists(), "failed health pipeline must not reach later commands")

    def test_sort_local_patch_table_by_date_stably(self) -> None:
        text = """# Fork

## 本地补丁记录

| 日期 | 问题 | 本地修复 | 验证 | 后续同步复查 |
| --- | --- | --- | --- | --- |
| 2026-06-08 | b | fix | test | review |
| 2026-06-07 | a | fix | test | review |
| 2026-06-08 | c | fix | test | review |

## 同步官方版本后的复查流程
"""

        sorted_text = fork_maintenance.sort_local_patch_table(text)

        rows = [
            line
            for line in sorted_text.splitlines()
            if line.startswith("| 2026-")
        ]
        self.assertEqual(
            rows,
            [
                "| 2026-06-07 | a | fix | test | review |",
                "| 2026-06-08 | b | fix | test | review |",
                "| 2026-06-08 | c | fix | test | review |",
            ],
        )
        self.assertIn("\n\n## 同步官方版本后的复查流程\n", sorted_text)


if __name__ == "__main__":
    unittest.main()
