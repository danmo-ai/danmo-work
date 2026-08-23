# 强约束系统

每章写前/审稿勾选；合同字段 `constraint_checks` 镜像。

## 金手指代劳

- 金手指 **不得代劳** 主角的关键抉择与情感高潮；代劳比例建议 **≤30%** 章内情节推动力。
- 系统发任务 ≠ 主角无选择；每次使用记 `resources` 代价。
- 违反 → review **StrongConstraints** blocking。

## 时间线

- 章内时间单调或明确跳跃标注；禁止无说明时间倒流。
- `timeline_events` 与正文 `when` 一致。
- 多线并行时每线标注时间点。

## 叙事线

- 同时 **活跃主线 + 支线 ≤3** 条；新开线须有旧线收尾或降级为伏笔。
- 章合同 `forbidden` 列出本章不得推进的线。

## 每章自检表（写入合同或 review）

```yaml
constraint_checks:
  goldfinger_ratio_ok: true   # 金手指未代劳关键抉择
  timeline_monotonic: true
  active_plot_lines_le_3: true
```

## Checklist

- [ ] 金手指有代价记录
- [ ] 时间线无静默冲突
- [ ] 活跃叙事线 ≤3
- [ ] 合同 constraint_checks 已填
