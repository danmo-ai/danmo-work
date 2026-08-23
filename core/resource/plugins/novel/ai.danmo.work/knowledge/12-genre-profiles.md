# 题材 QC Profile（审稿加权）

审稿时按本书 `novel-state.yaml` 的 `qc_profile` 选择权重；默认 `general`。

## Profile 表

| Profile | 适用 | 优先维度（10 分制，证据必填） |
|---------|------|------------------------------|
| `male_power` | 男频爽文、玄幻、都市逆袭 | 爽点兑现、升级反馈、节奏、期待链、钩子 |
| `female_emotion` | 女频情感、古言、甜虐 | 关系推进、情绪兑现、人物选择、对话张力 |
| `mystery` | 悬疑、灵异、规则怪谈 | 证据公平、信息差、反转可信度、氛围 |
| `general` | 未指定或混合 | 连贯、角色、节奏、钩子、语言 |

## male_power 门槛

- 爽点兑现 <6 → advisory；连续 2 章 <5 → blocking（开篇期）
- 升级/打脸无反馈 → blocking
- 钩子抽象口号 → P0

## female_emotion 门槛

- 关系无推进且章内无情绪高点 → advisory
- 人设崩（欲望/伤口违背）→ blocking
- 甜虐无因果 → P1

## mystery 门槛

- 主角获读者未知信息无铺垫 → blocking
- 线索回收不公平 → blocking
- 氛围全靠形容词无事件 → P1

## 与题材速览联动

写前 `search_kb` 题材速览 + 本 profile；合同 `pleasure_point` 对齐 profile 优先维度。

## Checklist

- [ ] `qc_profile` 已写入 novel-state
- [ ] 审稿报告 SCORES 段使用对应权重
- [ ] 每项分数附原文引用
