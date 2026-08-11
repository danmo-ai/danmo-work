# Continuity Commit

Commit = tools landed. No prose claim without updates.

## When

After review PASS (and optional polish), before starting the next chapter.

## Snapshot fields (update tables + short continuity notes)

1. Protagonist state: location, goal, injuries/timers, inventory deltas  
2. Supporting cast deltas only  
3. Relationship deltas  
4. World/rule deltas (or change requests)  
5. Foreshadow table `FS-xx`: planted / advanced / paid off  
6. Knowledge-state (who knows what) if relevant  
7. Resource / 金手指 uses  

## Tool actions

1. Ensure final text in `chapters/chNNN.md`.  
2. Set `chapters/chNNN-contract.yaml` `status=reviewed`; `table_upsert` timeline_events, foreshadows, characters, resources, chapter_contracts mirror.  
3. Append `continuity/chapter_summaries.md` — fixed 5-line block per chapter:

   ```markdown
   ## chNNN {{title}}
   - 事件: 一两句主干剧情
   - 状态变化: 谁从X→Y（位置/伤势/资源/关系）
   - 伏笔: FS-id plant/advance/payoff
   - 钩子: 章末留下的具体悬念
   - 下章指向: 接钩的第一拍
   ```

   Update `continuity/foreshadow-tracker.md` / `decision-log.md` as needed.  
4. `memory_update` agent: `last_committed_ch`, rolling summary pointer; project if promise changed.  
5. Update `novel-state.yaml` next chapter / stage.  
6. Close fixed `continuity_issues`.

## Phase compression

Every **10** committed chapters: write `continuity/phase-NN.md` synthesizing the arc; trim detailed mid summaries from active memory (keep file history).

## Resume

On cold start: `read_file` novel-state → `memory_read` → `table_query` open foreshadows → continue from next contract.
