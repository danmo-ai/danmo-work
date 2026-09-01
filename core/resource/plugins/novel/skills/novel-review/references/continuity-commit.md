# Continuity Commit

Commit = tools landed. Prefer **one** `apply_patch` covering ledger + contract + novel-state.

## When

After review PASS (and optional polish), before starting the next chapter. PASS 不要求 `reviews/` 文件。

## Snapshot（写入 ledger，勿拆多文件）

Update `continuity/ledger.md` in one pass:

1. **Public facts** — new shown_fact / inference from this chapter only  
2. **Tracking** — cursor, cast snapshot deltas, cannot-rewind  
3. **Open loops** — plant / advance / pay off（≤5 open）；合同 `info_control.foreshadowing` 的 FS-id 必须出现在表中  
4. **Chapter summaries** — append fixed 5-line block（五要素齐全，gate 硬检）:

```markdown
## chNNN {{title}}
- 事件: 一两句主干剧情
- 状态变化: 谁从X→Y（位置/伤势/资源/关系）
- 伏笔: FS-id plant/advance/payoff
- 钩子: 章末留下的具体悬念
- 下章指向: 接钩的第一拍
```

合同每条 `state_deltas` 的「谁」必须出现在 Cast snapshot。

## Tool actions（少交互）

1. Ensure final text in `chapters/chNNN.md`.  
2. Set `chapters/chNNN-contract.yaml` `status=reviewed`.  
3. Patch `continuity/ledger.md` (facts + tracking + loops + summary).  
4. Update `novel-state.yaml` (`last_committed_ch`, stage, gates).  
5. Optional: `memory_update` / `table_upsert` — **默认不做**.  
6. Gate 脚本 `--action postcommit --chapter N`. FAIL → do not claim Commit.

## Resume

Cold start: gate `--action doctor` → `novel-state` → `ledger.md` tail → next contract.  
If legacy continuity files exist without ledger → merge into `ledger.md` then archive.
