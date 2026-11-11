#!/usr/bin/env python3
"""Check local Markdown links without network access."""

from __future__ import annotations

import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlparse


LINK_PATTERN = re.compile(r"(?<!!)\[[^\]]+\]\((?P<target>[^)]+)\)")
FENCED_CODE_PATTERN = re.compile(r"```.*?```", re.DOTALL)
SKIP_PARTS = {".git", "artifacts", "node_modules", ".venv", "venv"}


def iter_markdown_files(root: Path):
    for path in sorted(root.rglob("*.md")):
        if not any(part in SKIP_PARTS for part in path.parts):
            yield path


def normalize_target(raw: str) -> str:
    target = raw.strip()
    if target.startswith("<") and ">" in target:
        target = target[1 : target.index(">")]
    else:
        target = target.split(maxsplit=1)[0]
    return target


def check_file(markdown_file: Path) -> list[str]:
    failures: list[str] = []
    text = FENCED_CODE_PATTERN.sub("", markdown_file.read_text(encoding="utf-8"))
    for match in LINK_PATTERN.finditer(text):
        target = normalize_target(match.group("target"))
        if not target or target.startswith("#"):
            continue
        parsed = urlparse(target)
        if parsed.scheme in {"http", "https", "mailto", "tel"}:
            continue
        path_text = unquote(parsed.path)
        if not path_text:
            continue
        resolved = (markdown_file.parent / path_text).resolve()
        if not resolved.exists():
            failures.append(f"{markdown_file}: missing local target: {target}")
    return failures


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    files = list(iter_markdown_files(root))
    failures: list[str] = []
    link_count = 0
    for markdown_file in files:
        link_count += len(LINK_PATTERN.findall(FENCED_CODE_PATTERN.sub("", markdown_file.read_text(encoding="utf-8"))))
        failures.extend(check_file(markdown_file))
    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1
    print(f"Markdown links: OK ({len(files)} files, {link_count} links)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
