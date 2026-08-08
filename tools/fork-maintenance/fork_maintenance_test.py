#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
from pathlib import Path
import unittest


SCRIPT_PATH = Path(__file__).with_name("fork_maintenance.py")
SPEC = importlib.util.spec_from_file_location("fork_maintenance", SCRIPT_PATH)
assert SPEC is not None
fork_maintenance = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(fork_maintenance)


class ForkMaintenanceDocSortTest(unittest.TestCase):
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
