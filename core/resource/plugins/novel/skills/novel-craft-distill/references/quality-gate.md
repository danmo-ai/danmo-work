# Quality gate（产物自检）

在宣称完成前逐项检查。任一项失败 → `edit` 输出文件，勿只在聊天里道歉。

## Checklist

- [ ] 磁盘上**仅**用户指定（或默认）那一份 md；未写入 `novel/<book-id>/canon/`
- [ ] 文首含 `control_scope: craft-only` 与消费方式说明
- [ ] `one_sentence_craft` + 四系统 + Dials 表 + Must keep/avoid + Evidence 均非空
- [ ] 无情节摘要、无人名地名专有设定、无金手指/伏笔/时间线
- [ ] 无空话标签（「文笔流畅」「节奏明快」等须改写成可执行规则）
- [ ] 示例句均 ≤80 字且已匿名化
- [ ] 短样本时 `confidence: low`
- [ ] Dial 均有 1–5 **且** Meaning 列解释两端
- [ ] 与通用去 AI P0 不强制冲突说明：若源文破折号/排比偏多，写在 Must keep，并注明「本书技法优先于通用禁令」

## 剧情泄漏快检

对产物 `grep`（或通读）可疑模式：章号剧情句、「主角名叫」、「反派是」、「结局」、「伏笔」。命中则删除或改成纯手法表述。
