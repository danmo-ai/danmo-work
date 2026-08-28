#!/usr/bin/env python3
"""Deterministic novel write-gate. Stdlib only. Invoked by novel skills via exec_shell.

python3 novel_gate.py --action doctor|preflight|precommit|postcommit \\
  --workdir PROJECT [--book-id SLUG] [--chapter N] [--json]
Exit 0 PASS, 1 FAIL, 2 usage/error.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path

HOOK_TYPES = {
    "信息缺口", "未兑现承诺", "高代价选择",
    "身份反转", "关系临界", "倒计时",
    "未完成动作", "突然揭示", "回声/意象",
}
MAX_OPEN_DEBTS = 5
TOXIC = [
    re.compile(r"不是.{1,20}而是"),
    re.compile(r"带着一丝|带着一抹"),
    re.compile(r"声音不大[，,]却带着"),
    re.compile(r"(他|她)知道|终于明白"),
    re.compile(r"眼中闪过|嘴角勾起|心中涌起"),
    re.compile(r"他不知道的是"),
]
LEVEL_ONE = [
    "仿佛", "犹如", "宛若", "不禁", "深吸一口气", "目光深邃",
    "不容置疑", "前所未有", "话锋一转", "心中暗道", "指节泛白", "瞳孔微缩",
]
SOUP_ENDINGS = [
    "或许，这只是个开始", "或许这只是个开始",
    "反击才刚刚开始", "这只是个开始",
]
CH_FILE_RE = re.compile(r"^ch(\d+)\.md$")
CH_CONTRACT_RE = re.compile(r"^ch(\d+)-contract\.yaml$")
UNIT_ID_RE = re.compile(r"^v\d+-U\d+$")
VOLUME_UNIT_ROW = re.compile(r"\|\s*U(\d+)\s*\|")
OPEN_STATUS = re.compile(r"(?i)\|\s*open\s*\|")
ADVANCED_STATUS = re.compile(r"(?i)\|\s*advanced\s*\|")


def _unquote(s: str) -> str:
    s = s.strip()
    if len(s) >= 2 and s[0] == s[-1] and s[0] in "\"'":
        return s[1:-1]
    return s


def _parse_flow_list(raw: str) -> list[str]:
    raw = raw.strip()
    if raw in ("[]", ""):
        return []
    if raw.startswith("[") and raw.endswith("]"):
        inner = raw[1:-1].strip()
        if not inner:
            return []
        parts, buf, q = [], "", None
        for ch in inner:
            if q:
                buf += ch
                if ch == q:
                    q = None
                continue
            if ch in "\"'":
                q = ch
                buf += ch
                continue
            if ch == ",":
                parts.append(_unquote(buf))
                buf = ""
                continue
            buf += ch
        if buf.strip():
            parts.append(_unquote(buf))
        return [p for p in parts if p]
    return [_unquote(raw)] if raw else []


def load_yaml_map(text: str) -> dict:
    try:
        import yaml  # type: ignore

        data = yaml.safe_load(text)
        return data if isinstance(data, dict) else {}
    except Exception:
        pass
    root: dict = {}
    stack: list[tuple[int, dict]] = [(0, root)]
    pending_list_key: str | None = None
    pending_list_indent = 0
    for raw in text.splitlines():
        if "#" in raw:
            hash_i = raw.find("#")
            if hash_i == 0 or (hash_i > 0 and raw[hash_i - 1].isspace()):
                raw = raw[:hash_i]
        if not raw.strip():
            continue
        indent = len(raw) - len(raw.lstrip(" "))
        line = raw.strip()
        while stack and indent < stack[-1][0]:
            stack.pop()
            pending_list_key = None
        cur = stack[-1][1]
        if line.startswith("- ") and pending_list_key is not None and indent >= pending_list_indent:
            cur.setdefault(pending_list_key, [])
            if not isinstance(cur[pending_list_key], list):
                cur[pending_list_key] = []
            cur[pending_list_key].append(_unquote(line[2:]))
            continue
        if ":" not in line:
            continue
        key, _, rest = line.partition(":")
        key, rest = key.strip(), rest.strip()
        if rest == "":
            nxt: dict = {}
            cur[key] = nxt
            stack.append((indent + 2, nxt))
            pending_list_key = key
            pending_list_indent = indent + 2
            continue
        pending_list_key = None
        if rest.startswith("["):
            cur[key] = _parse_flow_list(rest)
        else:
            val = _unquote(rest)
            if re.fullmatch(r"-?\d+", val):
                cur[key] = int(val)
            else:
                cur[key] = val
    return root


def contract_rel(ch: int) -> str:
    return f"chapters/ch{ch:03d}-contract.yaml"


def chapter_rel(ch: int) -> str:
    return f"chapters/ch{ch:03d}.md"


def parse_chapter_num(name: str) -> int | None:
    m = CH_FILE_RE.match(name) or CH_CONTRACT_RE.match(name)
    if not m:
        return None
    n = int(m.group(1))
    return n if n > 0 else None


def file_exists(root: Path, rel: str) -> bool:
    return (root / rel).exists()


def read_text(root: Path, rel: str) -> str:
    return (root / rel).read_text(encoding="utf-8")


def is_blank(s) -> bool:
    return s is None or str(s).strip() == ""


def nonempty_list(v) -> list[str]:
    if not isinstance(v, list):
        return []
    return [str(x).strip() for x in v if str(x).strip()]


def writing_stage(stage: str) -> bool:
    return str(stage).strip() in {"outline", "writing", "review"}


def tomato_profile(profile: str) -> bool:
    return str(profile).strip() in {"male_power", "female_emotion"}


def load_state(path: Path) -> dict:
    st = load_yaml_map(path.read_text(encoding="utf-8"))
    if not st.get("book_id"):
        st["book_id"] = path.parent.name
    return st


def load_contract(book_root: Path, chapter: int) -> tuple[dict, str]:
    rel = contract_rel(chapter)
    path = book_root / rel
    data = load_yaml_map(path.read_text(encoding="utf-8"))
    return data, rel


def resolve_book(workdir: str, book_id: str) -> tuple[Path, dict]:
    work = Path(workdir).resolve()
    if not workdir:
        raise SystemExit("workdir is required")
    state_path = work / "novel-state.yaml"
    if state_path.is_file():
        st = load_state(state_path)
        if not book_id or not st.get("book_id") or st.get("book_id") == book_id:
            return work, st
    novel_dir = work / "novel"
    if book_id:
        root = novel_dir / book_id
        return root, load_state(root / "novel-state.yaml")
    if not novel_dir.is_dir():
        raise FileNotFoundError(f"no novel-state.yaml in workdir and cannot list novel/: {work}")
    found: list[tuple[Path, dict]] = []
    for child in novel_dir.iterdir():
        if not child.is_dir():
            continue
        sp = child / "novel-state.yaml"
        if not sp.is_file():
            continue
        found.append((child, load_state(sp)))
    if not found:
        raise FileNotFoundError(f"no novel/<book-id>/novel-state.yaml under {work}")
    if len(found) > 1:
        ids = [st.get("book_id") or p.name for p, st in found]
        raise FileNotFoundError(f"multiple books {ids}; pass --book-id")
    return found[0]


class Report:
    def __init__(self, action: str, book_id: str, book_root: Path, chapter: int = 0):
        self.action = action
        self.book_id = book_id
        self.book_root = str(book_root)
        self.chapter = chapter
        self.verdict = "PASS"
        self.findings: list[dict] = []

    def blocking(self, check: str, msg: str) -> None:
        self.findings.append({"severity": "blocking", "check": check, "message": msg})

    def advisory(self, check: str, msg: str) -> None:
        self.findings.append({"severity": "advisory", "check": check, "message": msg})

    def finalize(self) -> None:
        self.verdict = "FAIL" if any(f["severity"] == "blocking" for f in self.findings) else "PASS"

    def format(self) -> str:
        lines = ["### VERDICT", self.verdict, "", "### ACTION", self.action]
        if self.book_id:
            lines += ["", "### BOOK", self.book_id]
        if self.chapter:
            lines += ["", "### CHAPTER", str(self.chapter)]
        lines += ["", "### BLOCKING"]
        blocks = [f for f in self.findings if f["severity"] == "blocking"]
        if not blocks:
            lines.append("None.")
        else:
            for f in blocks:
                lines.append(f"- [{f['check']}] {f['message']}")
        lines += ["", "### ADVISORY"]
        adv = [f for f in self.findings if f["severity"] == "advisory"]
        if not adv:
            lines.append("None.")
        else:
            for f in adv:
                lines.append(f"- [{f['check']}] {f['message']}")
        return "\n".join(lines) + "\n"

    def as_dict(self) -> dict:
        return {
            "action": self.action,
            "book_id": self.book_id,
            "book_root": self.book_root,
            "chapter": self.chapter,
            "verdict": self.verdict,
            "findings": self.findings,
        }


def last_runes(s: str, n: int) -> str:
    if n <= 0:
        return ""
    return s if len(s) <= n else s[-n:]


def rune_count(s: str) -> int:
    return sum(1 for r in s if r not in " \n\t\r")


def scan_deslop(prose: str) -> tuple[list[str], int, bool]:
    toxic = [p.pattern for p in TOXIC if p.search(prose)]
    n1 = sum(prose.count(w) for w in LEVEL_ONE)
    soup = any(x in last_runes(prose, 200) for x in SOUP_ENDINGS)
    return toxic, n1, soup


def count_open_foreshadows(tracker: str) -> int:
    n = 0
    for line in tracker.splitlines():
        if "|" not in line or "---" in line:
            continue
        low = line.lower()
        if "status" in low and "summary" in low:
            continue
        if OPEN_STATUS.search(line) or ADVANCED_STATUS.search(line):
            n += 1
    return n


def unit_listed(outline_root: Path, unit_id: str) -> bool:
    unit_id = unit_id.strip()
    if not unit_id or not outline_root.is_dir():
        return False
    u_short = ""
    vol = ""
    i = unit_id.rfind("-U")
    if i >= 0:
        vol = unit_id[:i]
        u_short = unit_id[i + 1 :]
    for path in outline_root.rglob("*.md"):
        text = path.read_text(encoding="utf-8", errors="replace")
        if unit_id in text:
            return True
        if u_short and VOLUME_UNIT_ROW.search(text) and f"| {u_short} |" in text:
            base = path.stem
            if base.lower() == vol.lower() or vol in text:
                return True
    return False


def open_debt_count(book_root: Path, contract: dict) -> int:
    n = len(nonempty_list(contract.get("reader_debt")))
    tracker = book_root / "continuity/foreshadow-tracker.md"
    if tracker.is_file():
        n += count_open_foreshadows(tracker.read_text(encoding="utf-8", errors="replace"))
    return n


def list_chapter_nums(book_root: Path, kind: str) -> list[int]:
    d = book_root / "chapters"
    if not d.is_dir():
        return []
    out = []
    for e in d.iterdir():
        if e.is_dir():
            continue
        n = parse_chapter_num(e.name)
        if n is None:
            continue
        if kind == "md" and CH_FILE_RE.match(e.name):
            out.append(n)
        elif kind == "contract" and CH_CONTRACT_RE.match(e.name):
            out.append(n)
    return out


def has_summary(summaries: str, ch: int) -> bool:
    return f"## ch{ch:03d}" in summaries


def hook_of(contract: dict) -> tuple[str, str]:
    hook = contract.get("hook") or {}
    if not isinstance(hook, dict):
        return "", ""
    return str(hook.get("type") or "").strip(), str(hook.get("out") or "").strip()


def check_doctor(book_root: Path, st: dict, r: Report) -> None:
    for rel in ("novel-state.yaml", "book-bible.md", "canon/world.md", "canon/glossary.md"):
        if not file_exists(book_root, rel):
            r.blocking("layout", "missing " + rel)
    for d in ("canon", "canon/cast", "outline", "outline/volumes", "chapters", "continuity", "reviews"):
        if not file_exists(book_root, d):
            r.blocking("layout", "missing directory " + d + "/")
    for rel in ("canon/author-lore.md", "continuity/public-lore.md", "continuity/tracking.md"):
        if file_exists(book_root, rel):
            continue
        if writing_stage(str(st.get("stage") or "")):
            r.blocking("lore-tracks", f"missing {rel} (required from outline/writing onward)")
        else:
            r.advisory("lore-tracks", f"missing {rel} — seed at setup")
    contracts = {}
    for n in list_chapter_nums(book_root, "contract"):
        contracts[n] = True
        try:
            c, rel = load_contract(book_root, n)
        except OSError as e:
            r.blocking("orphan-contract", f"{contract_rel(n)}: {e}")
            continue
        ch_field = int(c.get("chapter") or 0)
        if ch_field and ch_field != n:
            r.blocking("orphan-contract", f"{rel} chapter field={ch_field} filename={n}")
        if str(c.get("status") or "").strip() in ("drafted", "reviewed"):
            if not file_exists(book_root, chapter_rel(n)):
                r.blocking("orphan-contract", f"{rel} status={c.get('status')} but {chapter_rel(n)} missing")
    for n in list_chapter_nums(book_root, "md"):
        if n not in contracts:
            r.blocking("orphan-prose", f"{chapter_rel(n)} has no {contract_rel(n)}")
    last = int(st.get("last_committed_ch") or 0)
    if last > 0 and not file_exists(book_root, chapter_rel(last)):
        r.blocking("state", f"last_committed_ch={last} but {chapter_rel(last)} missing")


def check_preflight(book_root: Path, st: dict, ch: int, r: Report) -> None:
    if not file_exists(book_root, "novel-state.yaml"):
        r.blocking("state", "missing novel-state.yaml")
        return
    for rel in ("continuity/public-lore.md", "continuity/tracking.md"):
        if not file_exists(book_root, rel):
            r.blocking("lore-tracks", f"missing {rel} — write prose from public-lore + tracking only")
    if file_exists(book_root, "canon/author-lore.md"):
        r.advisory("lore-tracks", "do not load canon/author-lore.md into the draft context")
    elif writing_stage(str(st.get("stage") or "")):
        r.blocking("lore-tracks", "missing canon/author-lore.md")
    try:
        c, rel = load_contract(book_root, ch)
    except OSError as e:
        r.blocking("contract", f"{contract_rel(ch)}: {e}")
        return
    ch_field = int(c.get("chapter") or 0)
    if ch_field and ch_field != ch:
        r.blocking("contract", f"{rel} chapter={ch_field} want {ch}")
    status = str(c.get("status") or "").strip()
    if status not in ("accepted", "drafted"):
        r.blocking("contract", f"{rel} status={status} (need accepted before prose)")
    unit = str(c.get("unit_id") or "").strip()
    if not unit:
        r.blocking("unit_id", f"{rel} unit_id empty — return to novel-plan")
    elif not UNIT_ID_RE.match(unit):
        r.blocking("unit_id", f"{rel} unit_id={unit} must match vNN-U#")
    elif not unit_listed(book_root / "outline", unit):
        r.blocking("unit_id", f"{unit} not found in outline/ — return to novel-plan")
    if is_blank(c.get("purpose")):
        r.blocking("contract", "purpose empty")
    if not nonempty_list(c.get("beats")):
        r.blocking("contract", "beats empty")
    htype, hout = hook_of(c)
    if htype not in HOOK_TYPES:
        r.blocking("hook", f"hook.type must be one of KB 爽点与追读 types, got {htype}")
    if not hout:
        r.blocking("hook", "hook.out empty (need a concrete event)")
    if tomato_profile(str(st.get("qc_profile") or "")) and is_blank(c.get("pleasure_point")):
        r.blocking("pleasure_point", f"qc_profile={st.get('qc_profile')} requires pleasure_point")
    debts = open_debt_count(book_root, c)
    if debts > MAX_OPEN_DEBTS:
        r.blocking("reader_debt", f"open foreshadows+reader_debt={debts} exceeds {MAX_OPEN_DEBTS}")


def check_precommit(book_root: Path, st: dict, ch: int, r: Report) -> None:
    try:
        c, rel = load_contract(book_root, ch)
    except OSError as e:
        r.blocking("contract", f"{contract_rel(ch)}: {e}")
        return
    prose_rel = chapter_rel(ch)
    try:
        prose = read_text(book_root, prose_rel)
    except OSError:
        r.blocking("prose", f"{prose_rel} missing")
        return
    status = str(c.get("status") or "").strip()
    if status not in ("drafted", "accepted", "reviewed"):
        r.advisory("contract", f"{rel} status={status} expected drafted before review")
    toxic, n1, soup = scan_deslop(prose)
    if toxic:
        r.blocking("deslop", f"P0 毒句式 ×{len(toxic)} in {prose_rel}")
    if n1 >= 3:
        r.blocking("deslop", f"一级词 dense ({n1} hits) in {prose_rel}")
    elif n1 > 0:
        r.advisory("deslop", f"一级词 ×{n1} (blocking at ≥3)")
    if soup:
        r.blocking("deslop", "chicken-soup ending (P0)")
    _, hout = hook_of(c)
    if hout and hout not in prose and rune_count(hout) >= 8:
        r.advisory("hook", "hook.out not found verbatim in prose — confirm the event landed")
    wt = int(c.get("word_target") or 0)
    if wt > 0:
        n = rune_count(prose)
        if n < wt * 6 // 10:
            r.advisory("length", f"prose ~{n} runes vs word_target={wt}")
    debts = open_debt_count(book_root, c)
    if debts > MAX_OPEN_DEBTS:
        r.blocking("reader_debt", f"open foreshadows+reader_debt={debts} exceeds {MAX_OPEN_DEBTS}")


def check_postcommit(book_root: Path, st: dict, ch: int, r: Report) -> None:
    try:
        c, rel = load_contract(book_root, ch)
    except OSError as e:
        r.blocking("contract", f"{contract_rel(ch)}: {e}")
        return
    if str(c.get("status") or "").strip() != "reviewed":
        r.blocking("contract", f"{rel} status={c.get('status')} (Commit requires reviewed)")
    if not file_exists(book_root, chapter_rel(ch)):
        r.blocking("prose", f"{chapter_rel(ch)} missing")
    summaries = book_root / "continuity/chapter_summaries.md"
    if not summaries.is_file():
        r.blocking("commit", "missing continuity/chapter_summaries.md")
    elif not has_summary(summaries.read_text(encoding="utf-8", errors="replace"), ch):
        r.blocking("commit", f"no ## ch{ch:03d} block in continuity/chapter_summaries.md")
    last = int(st.get("last_committed_ch") or 0)
    if last < ch:
        r.blocking("state", f"last_committed_ch={last} want ≥{ch} after Commit")
    if not file_exists(book_root, "continuity/public-lore.md"):
        r.blocking("lore-tracks", "Commit must refresh continuity/public-lore.md from accepted prose")
    if not file_exists(book_root, "continuity/tracking.md"):
        r.blocking("lore-tracks", "Commit must refresh continuity/tracking.md")


def run(workdir: str, book_id: str, action: str, chapter: int) -> Report:
    action = (action or "").strip().lower()
    if action not in {"preflight", "precommit", "postcommit", "doctor"}:
        raise ValueError(f"unknown action {action!r} (preflight|precommit|postcommit|doctor)")
    root, st = resolve_book(workdir, book_id)
    bid = str(st.get("book_id") or root.name)
    r = Report(action, bid, root, chapter)
    if action == "doctor":
        check_doctor(root, st, r)
    else:
        if chapter <= 0:
            raise ValueError(f"chapter is required for {action}")
        if action == "preflight":
            check_preflight(root, st, chapter, r)
        elif action == "precommit":
            check_precommit(root, st, chapter, r)
        else:
            check_postcommit(root, st, chapter, r)
    r.finalize()
    return r


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="Novel write-gate / doctor (skill script)")
    p.add_argument("--action", default="doctor", help="preflight | precommit | postcommit | doctor")
    p.add_argument("--workdir", default=".", help="project root or book root")
    p.add_argument("--book-id", default="", help="slug under novel/<book-id>/")
    p.add_argument("--chapter", type=int, default=0)
    p.add_argument("--json", action="store_true")
    args = p.parse_args(argv)
    try:
        rep = run(os.path.abspath(args.workdir), args.book_id, args.action, args.chapter)
    except ValueError as e:
        print(e, file=sys.stderr)
        return 2
    except FileNotFoundError as e:
        print(e, file=sys.stderr)
        return 2
    except OSError as e:
        print(e, file=sys.stderr)
        return 2
    if args.json:
        json.dump(rep.as_dict(), sys.stdout, ensure_ascii=False, indent=2)
        sys.stdout.write("\n")
    else:
        sys.stdout.write(rep.format())
    return 0 if rep.verdict == "PASS" else 1


if __name__ == "__main__":
    sys.exit(main())
