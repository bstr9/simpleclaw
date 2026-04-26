---
id: REQ-065
title: "Extension.ShutdownAll 错误格式结构化"
status: active
level: task
priority: P2
cluster: architecture
created_at: "2026-04-26T19:00:00"
updated_at: "2026-04-26T19:00:00"
relations:
  depends_on: [REQ-008]
  related_to: [REQ-061]
versions:
  - version: 1
    date: "2026-04-26T19:00:00"
    author: ai
    context: "第二轮架构修复：ShutdownAll 错误格式不规范，多个错误直接拼接"
    reason: "第二轮架构修复"
    snapshot: "ShutdownAll 错误格式改为结构化，包含关闭数量和分号分隔"
---

# Extension.ShutdownAll 错误格式结构化

## 描述
`Extension.ShutdownAll()` 中多个扩展关闭失败时，错误信息直接拼接，格式不规范，难以解析和排查。

**修复方案**:
- 错误格式改为: `关闭 N 个扩展失败: err1; err2; err3`
- 包含失败数量和分号分隔
- 添加 `strings` 导入

## 验收标准
- [ ] 错误格式包含关闭失败数量
- [ ] 多个错误用分号分隔
- [ ] 格式可解析、可读性好
