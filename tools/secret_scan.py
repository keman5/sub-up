#!/usr/bin/env python3
"""Lightweight secret scan for CI and local checks."""

from __future__ import annotations

import argparse
import hashlib
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, Sequence


@dataclass(frozen=True)
class Rule:
    name: str
    pattern: re.Pattern[str]
    allowlist: Sequence[re.Pattern[str]]


RULES: list[Rule] = [
    Rule(
        name="google_oauth_client_secret",
        pattern=re.compile(r"GOCSPX-[0-9A-Za-z_-]{24,}"),
        allowlist=(
            re.compile(r"GOCSPX-your-"),
            re.compile(r"GOCSPX-REDACTED"),
        ),
    ),
    Rule(
        name="google_api_key",
        pattern=re.compile(r"AIza[0-9A-Za-z_-]{35}"),
        allowlist=(
            re.compile(r"AIza\.{3}"),
            re.compile(r"AIza-your-"),
            re.compile(r"AIza-REDACTED"),
        ),
    ),
]

# Existing upstream OAuth client configuration is intentionally versioned. Keep
# the exception content-addressed so any replacement value is still reported.
TRUSTED_MATCH_FINGERPRINTS = {
    "1d2f041093fd95aa8995a038c711d50a7960da09a505381c09a745d6ad0ecc60",
    "6a5f78b8b99dd4025e41ba11bf54c304c6af29f924c5569cc7865b2428ce03a9",
}


def iter_git_files(repo_root: Path) -> list[Path]:
    try:
        out = subprocess.check_output(
            ["git", "ls-files"], cwd=repo_root, stderr=subprocess.DEVNULL, text=True
        )
    except Exception:
        return []
    return [
        path
        for line in out.splitlines()
        if (path := (repo_root / line).resolve()).is_file()
    ]


def iter_walk_files(repo_root: Path) -> Iterable[Path]:
    for dirpath, _dirnames, filenames in os.walk(repo_root):
        if "/.git/" in dirpath.replace("\\", "/"):
            continue
        for name in filenames:
            yield Path(dirpath) / name


def should_skip(path: Path, repo_root: Path) -> bool:
    rel = path.relative_to(repo_root).as_posix()
    if any(rel.endswith(suffix) for suffix in (".png", ".jpg", ".jpeg", ".gif", ".pdf", ".zip")):
        return True
    return rel.startswith("backend/bin/")


def scan_file(path: Path, repo_root: Path) -> list[str]:
    try:
        text = path.read_bytes().decode("utf-8")
    except (OSError, UnicodeDecodeError):
        return []

    findings: list[str] = []
    for line_number, line in enumerate(text.splitlines(), start=1):
        for rule in RULES:
            matches = rule.pattern.findall(line)
            if not matches or any(item.search(line) for item in rule.allowlist):
                continue
            for match in matches:
                fingerprint = hashlib.sha256(match.encode("utf-8")).hexdigest()
                if fingerprint in TRUSTED_MATCH_FINGERPRINTS:
                    continue
                rel = path.relative_to(repo_root).as_posix()
                findings.append(f"{rel}:{line_number} ({rule.name})")
    return findings


def main(argv: Sequence[str]) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    args = parser.parse_args(argv)

    repo_root = Path(args.repo_root).resolve()
    files = iter_git_files(repo_root) or list(iter_walk_files(repo_root))
    findings = [
        finding
        for path in files
        if not should_skip(path, repo_root)
        for finding in scan_file(path, repo_root)
    ]
    if findings:
        sys.stderr.write("Secret scan FAILED. Potential secrets detected:\n")
        sys.stderr.write("\n".join(f"- {finding}" for finding in findings) + "\n")
        return 1

    print("Secret scan OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
