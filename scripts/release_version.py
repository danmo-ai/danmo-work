#!/usr/bin/env python3
"""Bump Danmo Work version, summarize commits, optionally commit/tag/push/release.

Version rules:
  - No version given  → latest vX.Y.Z tag, increment patch (Z+1)
  - Version given     → use as specified (with or without leading v)

Release notes = commit subject summary since the previous vX.Y.Z tag.

Examples:
  python3 scripts/release_version.py --dry-run
  python3 scripts/release_version.py
  python3 scripts/release_version.py 0.10.0
  python3 scripts/release_version.py --push
  python3 scripts/release_version.py 0.9.2 --push
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TAG_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")

CARGO_TOML = ROOT / "desktop" / "src-tauri" / "Cargo.toml"
CARGO_LOCK = ROOT / "desktop" / "src-tauri" / "Cargo.lock"
TAURI_CONF = ROOT / "desktop" / "src-tauri" / "tauri.conf.json"


def run(
    args: list[str],
    *,
    check: bool = True,
    capture: bool = True,
    input_text: str | None = None,
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        args,
        cwd=ROOT,
        check=check,
        text=True,
        input=input_text,
        capture_output=capture,
    )


def git_out(*args: str) -> str:
    return run(["git", *args]).stdout.strip()


def die(msg: str, code: int = 1) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    raise SystemExit(code)


def normalize_version(raw: str) -> str:
    v = raw.strip()
    if v.startswith("v") or v.startswith("V"):
        v = v[1:]
    if not re.fullmatch(r"\d+\.\d+\.\d+", v):
        die(f"invalid version '{raw}' (want X.Y.Z)")
    return v


def list_semver_tags() -> list[tuple[int, int, int, str]]:
    out = git_out("tag", "-l", "v*")
    tags: list[tuple[int, int, int, str]] = []
    for line in out.splitlines():
        m = TAG_RE.match(line.strip())
        if m:
            tags.append((int(m.group(1)), int(m.group(2)), int(m.group(3)), m.group(0)))
    tags.sort()
    return tags


def resolve_version(requested: str | None) -> tuple[str, str | None]:
    """Return (version without v, previous tag or None)."""
    tags = list_semver_tags()
    prev_tag = tags[-1][3] if tags else None

    if requested:
        version = normalize_version(requested)
        if any(t[3] == f"v{version}" for t in tags):
            die(f"tag v{version} already exists")
        return version, prev_tag

    if not tags:
        return "0.1.0", None

    major, minor, patch, prev = tags[-1]
    return f"{major}.{minor}.{patch + 1}", prev


def commit_summary(version: str, prev_tag: str | None) -> str:
    range_spec = f"{prev_tag}..HEAD" if prev_tag else "HEAD"
    log = git_out("log", range_spec, "--pretty=format:%s", "--no-merges")
    subjects = [s.strip() for s in log.splitlines() if s.strip()]
    # Drop pure version-bump commits like "v0.9.1"
    subjects = [s for s in subjects if not TAG_RE.match(s)]
    if not subjects:
        return f"Release v{version}."
    bullets = "\n".join(f"- {s}" for s in subjects)
    if prev_tag:
        return f"Changes since {prev_tag}:\n\n{bullets}"
    return f"Changes:\n\n{bullets}"


def bump_files(version: str) -> list[Path]:
    changed: list[Path] = []

    text = CARGO_TOML.read_text(encoding="utf-8")
    new, n = re.subn(
        r'(?m)^version = "[^"]*"',
        f'version = "{version}"',
        text,
        count=1,
    )
    if n != 1:
        die(f"failed to bump version in {CARGO_TOML}")
    if new != text:
        CARGO_TOML.write_text(new, encoding="utf-8")
        changed.append(CARGO_TOML)

    text = CARGO_LOCK.read_text(encoding="utf-8")
    new, n = re.subn(
        r'(name = "danmo-work-desktop"\n)version = "[^"]*"',
        rf'\1version = "{version}"',
        text,
        count=1,
    )
    if n != 1:
        die(f"failed to bump danmo-work-desktop in {CARGO_LOCK}")
    if new != text:
        CARGO_LOCK.write_text(new, encoding="utf-8")
        changed.append(CARGO_LOCK)

    conf = json.loads(TAURI_CONF.read_text(encoding="utf-8"))
    if conf.get("version") != version:
        conf["version"] = version
        TAURI_CONF.write_text(json.dumps(conf, indent=2) + "\n", encoding="utf-8")
        changed.append(TAURI_CONF)

    return changed


def ensure_gh() -> None:
    try:
        run(["gh", "--version"])
    except (FileNotFoundError, subprocess.CalledProcessError):
        die("gh CLI required for --push (create/update GitHub release notes)")


def git_commit_all(version: str, notes: str) -> None:
    run(["git", "add", "-A"], capture=False)
    status = git_out("status", "--porcelain")
    if not status:
        die("nothing to commit (version already at target and tree clean?)")
    msg = f"v{version}\n\n{notes}\n"
    run(["git", "commit", "-m", msg], capture=False)


def git_tag_and_push(version: str, notes: str) -> None:
    tag = f"v{version}"
    run(["git", "tag", "-a", tag, "-m", notes], capture=False)
    run(["git", "push", "origin", "HEAD"], capture=False)
    run(["git", "push", "origin", tag], capture=False)


def publish_release_notes(version: str, notes: str) -> None:
    tag = f"v{version}"
    ensure_gh()
    # Create release early so CI softprops attaches assets to an existing release.
    # If it already exists, edit notes.
    view = run(["gh", "release", "view", tag], check=False)
    if view.returncode == 0:
        run(["gh", "release", "edit", tag, "--notes", notes], capture=False)
        print(f"Updated GitHub release notes for {tag}")
    else:
        run(
            [
                "gh",
                "release",
                "create",
                tag,
                "--title",
                tag,
                "--notes",
                notes,
                "--verify-tag",
            ],
            capture=False,
        )
        print(f"Created GitHub release {tag} (assets will attach from CI)")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description="Bump Danmo Work version and prepare a tagged release.",
    )
    p.add_argument(
        "version",
        nargs="?",
        help="X.Y.Z (optional). If omitted, bump patch of latest vX.Y.Z tag.",
    )
    p.add_argument(
        "--dry-run",
        action="store_true",
        help="Print resolved version and notes; do not write or push.",
    )
    p.add_argument(
        "--push",
        action="store_true",
        help="Commit, annotated-tag, push, and set GitHub release notes.",
    )
    p.add_argument(
        "--no-release",
        action="store_true",
        help="With --push, skip gh release create/edit (tag push only).",
    )
    return p.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if args.push and args.dry_run:
        die("use either --dry-run or --push, not both")

    version, prev_tag = resolve_version(args.version)
    notes = commit_summary(version, prev_tag)

    print(f"version: {version}")
    print(f"tag:     v{version}")
    print(f"prev:    {prev_tag or '(none)'}")
    print()
    print("--- release notes ---")
    print(notes)
    print("---")

    if args.dry_run:
        return 0

    changed = bump_files(version)
    for path in changed:
        print(f"bumped: {path.relative_to(ROOT)}")
    if not changed:
        print("version files already at target")

    if not args.push:
        print()
        print("Files updated. Commit/tag/push yourself, or re-run with --push.")
        return 0

    git_commit_all(version, notes)
    git_tag_and_push(version, notes)
    if not args.no_release:
        publish_release_notes(version, notes)

    print()
    print(f"Done. CI release build should start for v{version}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
