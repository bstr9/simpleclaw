---
id: REQ-060
title: "Config 默认值去重"
status: active
level: task
priority: P1
cluster: config
created_at: "2026-04-26T16:00:00"
updated_at: "2026-04-26T16:00:00"
relations:
  depends_on: [REQ-054]
  related_to: [REQ-056]
versions:
  - version: 1
    date: "2026-04-26T16:00:00"
    author: ai
    context: "架构评审发现 defaults.go 中 getDefaultConfig() 和 setDefaults(viper) 双重维护同一组默认值"
    reason: "架构评审 P1 级发现"
    snapshot: "消除 Config 默认值的双重维护，统一为单一来源"
---

# Config 默认值去重

## 描述
`pkg/config/defaults.go` 定义了两组默认值：
1. `getDefaultConfig()` — 返回 `*Config` 结构体形式的默认值（用于 `Get()` 自动创建）
2. `setDefaults(viper.Viper)` — 设置 viper 默认值（用于配置加载流程）

这两组默认值必须手动保持同步，否则会出现不一致。例如 `setDefaults` 包含 `stream_output: true`、`voice_reply_voice: false`、`admin.*` 等字段，而 `getDefaultConfig()` 中缺失。

**推荐修复方案**: 保留 `setDefaults(viper.Viper)` 作为唯一默认值来源（它覆盖更完整），`getDefaultConfig()` 改为通过 viper 生成，或直接调用 `setDefaults` 后 Unmarshal。

## 验收标准
- [ ] 默认值只在一处定义
- [ ] `getDefaultConfig()` 和 `setDefaults()` 不再需要手动同步
- [ ] 新增配置字段的默认值只需在一处添加
- [ ] 现有配置加载行为不变
