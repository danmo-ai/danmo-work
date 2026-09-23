# Craft techniques schema

输出文件必须覆盖下列节。可增小节，不可删 `control_scope` / 四系统 / dials / must_*。

## 必填结构

```markdown
# Craft techniques — <source-slug>

control_scope: craft-only
# 本文件只约束怎么写，不约束写什么情节/设定。
# 消费方式：导入知识库，或提示「参考本文件路径写作」。
# 不自动进入任何 novel/<book-id>/canon 或 preflight。

## Meta
- source: <路径或目录名>
- sampled: <短摘录 | 章首/中/末+对话密段+高潮段 | …>
- confidence: high | medium | low
- one_sentence_craft: <一句话概括这部作品的写法>

## Language system
…

## Narrative system
…

## Dialogue & interiority system
…

## Pacing & emotion system
…

## Dials
| Dial | 1–5 | Meaning |
|------|-----|---------|
| sentence_length | | |
| show_tell | | |
| dialogue_ratio_dial | | |
| interiority | | |
| pacing | | |
| emotion_temp | | |

## Must keep
- …

## Must avoid
- …

## Evidence
| Signal type | Note | Confidence |
|-------------|------|------------|
| sentence_rhythm | … | high |
| dialogue_shape | … | … |
```

## 字段约束

- `one_sentence_craft`：≤60 字；手法向（如「冷硬短句 + 高对话占比 + 章末决策钩」），禁剧情剧透。
- 四系统：每系统 3–8 条可执行规则；禁「文笔好」「引人入胜」等空话。
- `Evidence`：只记信号类型与观察，不贴长引文；形态示意句 ≤80 字且匿名化。
- `confidence: low` 当源 < ~8k 字或仅单章摘录时强制。
