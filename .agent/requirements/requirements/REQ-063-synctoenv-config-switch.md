---
id: REQ-063
title: "syncToEnv() 添加配置开关"
status: active
level: task
priority: P1
cluster: architecture
created_at: "2026-04-26T19:00:00"
updated_at: "2026-04-26T19:00:00"
relations:
  depends_on: [REQ-008]
  related_to: [REQ-056]
versions:
  - version: 1
    date: "2026-04-26T19:00:00"
    author: ai
    context: "第二轮架构修复：syncToEnv() 将 API 密钥写入环境变量，存在安全风险"
    reason: "第二轮架构修复"
    snapshot: "syncToEnv() 添加 SyncToEnv 配置开关，可关闭 API 密钥泄漏到子进程"
---

# syncToEnv() 添加配置开关

## 描述
`syncToEnv()` 将 API 密钥等敏感信息通过 `os.Setenv()` 写入环境变量，导致所有子进程和 goroutine 均可读取，存在安全风险。

**修复方案**:
- Config 结构体新增 `SyncToEnv bool` 字段
- `syncToEnv()` 开头检查该开关，`false` 时跳过
- 默认值 `true`（向后兼容）
- 配置项: `sync_to_env`

## 验收标准
- [ ] Config 新增 `SyncToEnv bool` 字段
- [ ] `syncToEnv()` 检查开关，`false` 时跳过
- [ ] 默认值为 `true`（向后兼容）
- [ ] `defaults.go` 添加 `sync_to_env` 默认值
