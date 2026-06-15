#!/usr/bin/env python3
"""Collect lightweight host health for Sub2API Ops dashboard.

The application containers read the JSON file produced by this script. Keep
host shelling out here, not in the request path.
"""

from __future__ import annotations

import json
import os
import subprocess
import time
from datetime import datetime, timezone
from pathlib import Path


OUT_PATH = Path(os.environ.get("SUB2API_HOST_HEALTH_OUTPUT", "/run/sub2api-ops/host-health.json"))
CPU_HIGH_THRESHOLD = float(os.environ.get("SUB2API_HOST_HEALTH_CPU_HIGH_THRESHOLD", "90"))


def run(cmd: list[str], timeout: float = 4.0) -> str:
    try:
        return subprocess.check_output(cmd, stderr=subprocess.DEVNULL, text=True, timeout=timeout)
    except Exception:
        return ""


def read_load_average() -> dict[str, float]:
    try:
        parts = Path("/proc/loadavg").read_text(encoding="utf-8").split()
        return {"one": float(parts[0]), "five": float(parts[1]), "fifteen": float(parts[2])}
    except Exception:
        return {"one": 0.0, "five": 0.0, "fifteen": 0.0}


def read_memory() -> dict[str, int]:
    values: dict[str, int] = {}
    try:
        for line in Path("/proc/meminfo").read_text(encoding="utf-8").splitlines():
            key, raw = line.split(":", 1)
            values[key] = int(raw.strip().split()[0])
    except Exception:
        return {"available_mb": 0, "swap_used_mb": 0}

    available_mb = values.get("MemAvailable", 0) // 1024
    swap_total = values.get("SwapTotal", 0)
    swap_free = values.get("SwapFree", 0)
    swap_used_mb = max(swap_total - swap_free, 0) // 1024
    return {"available_mb": available_mb, "swap_used_mb": swap_used_mb}


def read_cpu_usage_percent() -> float:
    def sample() -> tuple[int, int]:
        parts = Path("/proc/stat").read_text(encoding="utf-8").splitlines()[0].split()[1:]
        numbers = [int(p) for p in parts]
        idle = numbers[3] + numbers[4]
        total = sum(numbers)
        return total, idle

    try:
        total1, idle1 = sample()
        time.sleep(0.2)
        total2, idle2 = sample()
        total_delta = total2 - total1
        idle_delta = idle2 - idle1
        if total_delta <= 0:
            return 0.0
        return round((1 - idle_delta / total_delta) * 100, 2)
    except Exception:
        return 0.0


def parse_percent(raw: str) -> float:
    try:
        return float(raw.strip().rstrip("%"))
    except Exception:
        return 0.0


def read_top_containers() -> list[dict[str, object]]:
    output = run(["docker", "stats", "--no-stream", "--format", "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.PIDs}}"])
    rows: list[dict[str, object]] = []
    for line in output.splitlines():
        parts = line.split("\t")
        if len(parts) != 4:
            continue
        name, cpu, memory, pids = parts
        try:
            pid_count = int(pids.strip())
        except Exception:
            pid_count = 0
        rows.append({
            "name": name.strip(),
            "cpu_percent": parse_percent(cpu),
            "memory": memory.strip(),
            "pids": pid_count,
        })
    rows.sort(key=lambda item: float(item.get("cpu_percent", 0)), reverse=True)
    return rows[:8]


def read_top_processes() -> list[dict[str, object]]:
    output = run(["ps", "-eo", "pid=,comm=,pcpu=,rss=", "--sort=-pcpu"], timeout=2.0)
    rows: list[dict[str, object]] = []
    for line in output.splitlines()[:8]:
        parts = line.split(None, 3)
        if len(parts) != 4:
            continue
        pid, command, cpu, rss = parts
        if command == "ps":
            continue
        try:
            rss_mb = int(int(rss) / 1024)
        except Exception:
            rss_mb = 0
        rows.append({
            "pid": int(pid),
            "command": command,
            "cpu_percent": parse_percent(cpu),
            "rss_mb": rss_mb,
        })
    return rows


def diagnose(cpu_percent: float, containers: list[dict[str, object]], processes: list[dict[str, object]]) -> str:
    if cpu_percent < CPU_HIGH_THRESHOLD:
        return ""
    if containers and float(containers[0].get("cpu_percent", 0)) >= 30:
        name = str(containers[0].get("name", "unknown"))
        return f"CPU 压力主要来自容器 {name}"
    if processes and float(processes[0].get("cpu_percent", 0)) >= 30:
        command = str(processes[0].get("command", "unknown"))
        return f"CPU 压力主要来自进程 {command}"
    return "CPU 压力较高，但未识别到单个主要容器"


def main() -> None:
    cpu_percent = read_cpu_usage_percent()
    containers = read_top_containers()
    processes = read_top_processes()
    payload = {
        "collected_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "load_average": read_load_average(),
        "cpu": {"usage_percent": cpu_percent, "high": cpu_percent >= CPU_HIGH_THRESHOLD},
        "memory": read_memory(),
        "top_containers": containers,
        "top_processes": processes,
        "diagnosis": diagnose(cpu_percent, containers, processes),
    }

    OUT_PATH.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = OUT_PATH.with_suffix(".json.tmp")
    tmp_path.write_text(json.dumps(payload, ensure_ascii=False, separators=(",", ":")) + "\n", encoding="utf-8")
    tmp_path.replace(OUT_PATH)


if __name__ == "__main__":
    main()
