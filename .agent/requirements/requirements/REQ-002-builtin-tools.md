---
id: REQ-002
title: "内置工具系统"
status: active
level: epic
priority: P0
cluster: agent-tools
created_at: "2026-04-23T10:00:00"
updated_at: "2026-04-26T10:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: [REQ-001]
  refined_by: [REQ-013]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T10:00:00"
    author: ai
    context: "从代码逆向分析提取需求，来源: pkg/agent/tools/"
    reason: "逆向代码生成需求"
    snapshot: "14 个内置工具：文件操作、Web 搜索/抓取、Bash、浏览器、记忆、定时任务、视觉、环境配置"
  - version: 2
    date: "2026-04-22T16:13:03"
    author: ai
    context: "元数据自动同步"
    reason: "自动补充反向关系: refined_by"
    snapshot: "自动同步元数据"
  - version: 3
    date: "2026-04-26T10:00:00"
    author: ai
    context: "需求审查发现工具名称不匹配和缺失实现"
    reason: "添加已知问题章节：scheduler Name() 返回 'cron'、memory_search Name() 返回 'memory'、grep/find/terminal 无实现"
    snapshot: "内置工具系统，标注工具名称不匹配和缺失实现问题"
---

# 内置工具系统

## 描述
Agent 可调用的 14 个内置工具，统一实现 Tool 接口（Name/Description/Parameters/Execute），通过 ToolRegistry 注册和管理。工具覆盖文件操作、网络请求、浏览器控制、记忆管理、定时任务等场景。

## 验收标准
- [x] Tool 接口定义：Name()、Description()、Parameters()（JSON Schema）、Execute()
- [x] ToolResult 统一返回格式：Success/Output/Error
- [x] 文件操作工具：read（读取文件）、write（创建/覆盖，限 10KB）、edit（精确字符串替换）、ls（目录列表）
- [x] 网络工具：web_search（多提供商搜索：Brave/Gemini/Grok/Kimi/Perplexity）、web_fetch（网页内容获取）
- [x] 实用工具：bash（Shell 命令执行，含禁止列表）、time（时间查询）
- [x] 代理工具：browser（浏览器自动化）、memory（记忆读写）、scheduler（定时任务管理）
- [x] 辅助工具：vision（图像识别）、env_config（环境变量和 API 密钥管理）、send（发送文件给用户）
- [x] 工具配置化启用/禁用（web_search、web_fetch 通过 config 控制）
- [x] Bash 工具安全控制（WithBashDenyList 禁止危险命令）

## 已知问题
- scheduler 工具 Name() 返回 'cron' 而非 'scheduler'，与配置键名不一致
- memory_search 工具 Name() 返回 'memory'，与 memory_write 工具共用名称
- grep/find/terminal 工具在 AGENTS.md 中列出但代码中无实现
