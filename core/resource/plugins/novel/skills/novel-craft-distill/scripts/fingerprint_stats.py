#!/usr/bin/env python3
"""Lightweight craft fingerprint stats for novel-craft-distill.

Reads local .txt/.md (file or directory). Prints JSON to stdout.
Never writes source prose to disk.

Method adapted from woanderingboy/novel-writing-pipeline fingerprint fields
(selective rewrite, MIT).
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

TEXT_SUFFIXES = {".txt", ".md", ".markdown", ".text"}

# Dialogue: Chinese quotes or Latin quotes wrapping a run.
_DIALOGUE_SPAN = re.compile(
    r"[「『“\"]([^」』”\"]{0,200})[」』”\"]"
)
_SENTENCE_END = re.compile(r"[。！？!?；;]+")
_EM_DASH = re.compile(r"—+|-{2,}")
# Common Chinese connective density signals (craft dials, not content).
_CONNECTIVES = (
    "然而",
    "但是",
    "于是",
    "接着",
    "然后",
    "因为",
    "所以",
    "如果",
    "虽然",
    "不过",
    "而且",
    "并且",
)


def collect_texts(path: Path) -> str:
    if path.is_file():
        return path.read_text(encoding="utf-8", errors="replace")
    if not path.is_dir():
        raise FileNotFoundError(path)
    parts: list[str] = []
    for p in sorted(path.rglob("*")):
        if p.is_file() and p.suffix.lower() in TEXT_SUFFIXES:
            parts.append(p.read_text(encoding="utf-8", errors="replace"))
    if not parts:
        raise FileNotFoundError(f"no text files under {path}")
    return "\n".join(parts)


def _rune_len(s: str) -> int:
    return len(s)


def analyze(text: str) -> dict:
    # Strip markdown headings markers lightly for length stats.
    body = re.sub(r"(?m)^#{1,6}\s+", "", text)
    body = body.strip()
    chars = _rune_len(body)
    if chars == 0:
        return {
            "chars": 0,
            "sentences": 0,
            "avg_sentence_len": 0.0,
            "dialogue_ratio": 0.0,
            "em_dash_per_1k": 0.0,
            "connective_per_1k": 0.0,
            "paragraph_avg_len": 0.0,
        }

    sentences = [s.strip() for s in _SENTENCE_END.split(body) if s.strip()]
    sent_n = len(sentences) or 1
    avg_sent = sum(_rune_len(s) for s in sentences) / sent_n

    dialogue_chars = sum(_rune_len(m.group(0)) for m in _DIALOGUE_SPAN.finditer(body))
    dialogue_ratio = dialogue_chars / chars

    em_n = len(_EM_DASH.findall(body))
    conn_n = sum(body.count(c) for c in _CONNECTIVES)
    per_1k = chars / 1000.0

    paras = [p.strip() for p in re.split(r"\n\s*\n", body) if p.strip()]
    para_avg = (sum(_rune_len(p) for p in paras) / len(paras)) if paras else 0.0

    return {
        "chars": chars,
        "sentences": len(sentences),
        "avg_sentence_len": round(avg_sent, 2),
        "dialogue_ratio": round(dialogue_ratio, 4),
        "em_dash_per_1k": round(em_n / per_1k, 2) if per_1k else 0.0,
        "connective_per_1k": round(conn_n / per_1k, 2) if per_1k else 0.0,
        "paragraph_avg_len": round(para_avg, 2),
    }


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="Craft fingerprint stats (JSON stdout, no prose dump)")
    p.add_argument("source", type=Path, help="File or directory of .txt/.md")
    args = p.parse_args(argv)
    try:
        text = collect_texts(args.source)
    except FileNotFoundError as e:
        print(json.dumps({"error": str(e)}, ensure_ascii=False), file=sys.stderr)
        return 1
    stats = analyze(text)
    stats["source"] = str(args.source)
    print(json.dumps(stats, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
