#!/usr/bin/env python3
"""Download Terminal-Bench 2.0 tasks via GitHub Contents API (when git is blocked).

Resumable: skips tasks that already have task.toml + environment/Dockerfile.
Uses raw download_url for file bodies (avoids huge base64 JSON IncompleteRead).

Env:
  TB2_API_REPO   default harbor-framework/terminal-bench-2
  TB2_API_REF    default main
  HARBOR_TASKS_DIR / TASKS_DIR destination
  GITHUB_TOKEN / GH_TOKEN  optional, raises rate limit
  TB2_API_MAX_TASKS  optional limit for smoke
  FORCE_RESYNC=1  re-download even if task.toml exists
"""

from __future__ import annotations

import json
import os
import re
import shutil
import sys
import time
import urllib.error
import urllib.request
from http.client import IncompleteRead
from pathlib import Path

REPO = os.environ.get("TB2_API_REPO", "harbor-framework/terminal-bench-2")
REF = os.environ.get("TB2_API_REF", "main")
BASE = f"https://api.github.com/repos/{REPO}/contents"
DEST = Path(
    os.environ.get("HARBOR_TASKS_DIR")
    or os.environ.get("TASKS_DIR")
    or Path(__file__).resolve().parent / "tasks"
)
BASE_IMAGE = os.environ.get("HARBOR_BASE_IMAGE", "dq-harbor-base:local")
MAX_TASKS = os.environ.get("TB2_API_MAX_TASKS", "").strip()
FORCE = os.environ.get("FORCE_RESYNC", "") == "1"
SKIP_NAMES = {".github", "docs", "scripts", ".git"}
RETRIES = 5


def headers() -> dict[str, str]:
    h = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "danmo-work-tb2-sync",
    }
    token = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN")
    if token:
        h["Authorization"] = f"Bearer {token}"
    return h


def _read_url(url: str, *, as_json: bool, timeout: int = 180):
    last: Exception | None = None
    for attempt in range(1, RETRIES + 1):
        try:
            req = urllib.request.Request(url, headers=headers())
            with urllib.request.urlopen(req, timeout=timeout) as r:
                data = r.read()
            if as_json:
                return json.loads(data.decode("utf-8"))
            return data
        except (IncompleteRead, TimeoutError, urllib.error.URLError, ConnectionError) as exc:
            last = exc
            wait = min(2**attempt, 30)
            print(f"  retry {attempt}/{RETRIES} after {exc!r} (sleep {wait}s)", flush=True)
            time.sleep(wait)
        except urllib.error.HTTPError as exc:
            if exc.code in (403, 429, 502, 503, 504) and attempt < RETRIES:
                wait = min(2**attempt, 60)
                print(f"  HTTP {exc.code}; retry in {wait}s", flush=True)
                time.sleep(wait)
                last = exc
                continue
            raise
    assert last is not None
    raise last


def api_get(url: str):
    return _read_url(url, as_json=True, timeout=120)


def download_bytes(url: str) -> bytes:
    return _read_url(url, as_json=False, timeout=300)


def rel_under_prefix(path: str, prefix: str) -> str:
    if prefix and path.startswith(prefix + "/"):
        return path[len(prefix) + 1 :]
    return Path(path).name


def download_path(api_path: str, dest_root: Path, prefix: str) -> None:
    url = f"{BASE}/{api_path}?ref={REF}" if api_path else f"{BASE}?ref={REF}"
    data = api_get(url)
    if isinstance(data, dict) and data.get("type") == "file":
        rel = rel_under_prefix(data["path"], prefix)
        local = dest_root / rel
        local.parent.mkdir(parents=True, exist_ok=True)
        dl = data.get("download_url")
        if not dl:
            raise RuntimeError(f"no download_url for {data['path']}")
        local.write_bytes(download_bytes(dl))
        print(f"  file {data['path']}", flush=True)
        return

    for e in data:
        if e["type"] == "dir":
            download_path(e["path"], dest_root, prefix)
            continue
        rel = rel_under_prefix(e["path"], prefix)
        local = dest_root / rel
        local.parent.mkdir(parents=True, exist_ok=True)
        dl = e.get("download_url")
        if not dl:
            # Rare: need metadata
            meta = api_get(e["url"])
            dl = meta.get("download_url")
        if not dl:
            raise RuntimeError(f"no download_url for {e['path']}")
        local.write_bytes(download_bytes(dl))
        print(f"  file {e['path']}", flush=True)


def patch_dockerfiles(task_dir: Path) -> int:
    n = 0
    pat = re.compile(
        r"^(FROM\s+)(?:--\S+\s+)*(\S+)(\s+AS\s+\S+)?\s*$",
        re.IGNORECASE | re.MULTILINE,
    )
    for df in task_dir.rglob("Dockerfile*"):
        if "environment" not in df.parts:
            continue
        text = df.read_text(encoding="utf-8")
        text2, c = pat.subn(rf"\g<1>{BASE_IMAGE}\g<3>", text, count=1)
        if c:
            df.write_text(text2, encoding="utf-8")
            n += 1
            print(f"patched {df}")
    return n


def clear_prebuilt_image(task_dir: Path) -> bool:
    tomls = list(task_dir.glob("task.toml"))
    if not tomls:
        return False
    p = tomls[0]
    text = p.read_text(encoding="utf-8")
    text2, n = re.subn(r"(?m)^docker_image\s*=\s*.*\n?", "", text)
    if n:
        p.write_text(text2, encoding="utf-8")
        print(f"cleared docker_image in {p}")
        return True
    return False


def task_complete(task_dir: Path) -> bool:
    return (task_dir / "task.toml").is_file() and any(
        task_dir.glob("environment/Dockerfile*")
    )


def main() -> int:
    print(f"API sync repo={REPO} ref={REF} → {DEST} (resume={not FORCE})")
    try:
        root = api_get(f"{BASE}?ref={REF}")
    except urllib.error.HTTPError as e:
        print(f"GitHub API error: {e}", file=sys.stderr)
        return 1

    tasks = sorted(
        e["name"]
        for e in root
        if e["type"] == "dir" and e["name"] not in SKIP_NAMES and not e["name"].startswith(".")
    )
    if MAX_TASKS:
        tasks = tasks[: int(MAX_TASKS)]

    DEST.mkdir(parents=True, exist_ok=True)

    patched = 0
    done = 0
    failed: list[str] = []
    for i, name in enumerate(tasks, 1):
        task_dest = DEST / name
        if not FORCE and task_complete(task_dest):
            # Still ensure patches applied on resume.
            patched += patch_dockerfiles(task_dest)
            clear_prebuilt_image(task_dest)
            print(f"[{i}/{len(tasks)}] {name} (skip existing)", flush=True)
            done += 1
            continue

        print(f"[{i}/{len(tasks)}] {name}", flush=True)
        if task_dest.exists():
            shutil.rmtree(task_dest)
        task_dest.mkdir(parents=True)
        try:
            download_path(name, task_dest, name)
            patched += patch_dockerfiles(task_dest)
            clear_prebuilt_image(task_dest)
            if not task_complete(task_dest):
                raise RuntimeError("missing task.toml or Dockerfile after download")
            done += 1
        except Exception as exc:
            print(f"FAIL {name}: {exc}", file=sys.stderr, flush=True)
            failed.append(name)
            if task_dest.exists():
                shutil.rmtree(task_dest, ignore_errors=True)

    marker = DEST / ".tb2-synced"
    marker.write_text(
        f"api:{REPO}@{REF}\ntasks={done}/{len(tasks)}\nfailed={','.join(failed)}\n",
        encoding="utf-8",
    )
    print(
        f"OK synced {done}/{len(tasks)} tasks "
        f"(patched_dockerfiles={patched} failed={len(failed)})"
    )
    if failed:
        print("failed:", ", ".join(failed), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
