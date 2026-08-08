#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
from pathlib import Path
import unittest


MODULE_PATH = Path(__file__).with_name("sub2api-host-health-collector.py")
spec = importlib.util.spec_from_file_location("sub2api_host_health_collector", MODULE_PATH)
collector = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(collector)


class HostHealthDiagnosisTest(unittest.TestCase):
    def test_diagnoses_dominant_container_in_chinese(self) -> None:
        diagnosis = collector.diagnose(
            95,
            [{"name": "sub2api-postgres", "cpu_percent": 45}],
            [{"command": "postgres", "cpu_percent": 40}],
        )

        self.assertEqual(diagnosis, "CPU 压力主要来自容器 sub2api-postgres")

    def test_diagnoses_dominant_process_in_chinese(self) -> None:
        diagnosis = collector.diagnose(
            95,
            [{"name": "sub2api", "cpu_percent": 5}],
            [{"command": "postgres", "cpu_percent": 40}],
        )

        self.assertEqual(diagnosis, "CPU 压力主要来自进程 postgres")

    def test_diagnoses_high_cpu_without_dominant_source_in_chinese(self) -> None:
        diagnosis = collector.diagnose(
            95,
            [{"name": "sub2api", "cpu_percent": 5}],
            [{"command": "postgres", "cpu_percent": 5}],
        )

        self.assertEqual(diagnosis, "CPU 压力较高，但未识别到单个主要容器")


if __name__ == "__main__":
    unittest.main()
