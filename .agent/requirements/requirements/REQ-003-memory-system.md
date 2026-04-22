---
id: REQ-003
title: "记忆系统"
status: active
level: epic
priority: P0
cluster: agent-memory
created_at: "2026-04-23T10:00:00"
updated_at: "2026-04-22T16:13:03"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: [REQ-001]
  refined_by: [REQ-012]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T10:00:00"
    author: ai
    context: "从代码逆向分析提取需求，来源: pkg/agent/memory/"
    reason: "逆向代码生成需求"
    snapshot: "三层记忆架构（短期/长期/会话），支持向量搜索、关键词搜索、混合搜索，SQLite 后端存储"
  - version: 2
    date: "2026-04-22T16:13:03"
    author: ai
    context: "元数据自动同步"
    reason: "自动补充反向关系: refined_by"
    snapshot: "自动同步元数据"
---

# 记忆系统

## 描述
Agent 记忆管理系统，支持三层作用域（shared/user/session），长期记忆支持向量嵌入和语义搜索。SQLite 作为存储后端，支持文本分块、摘要生成、向量索引等能力。

## 验收标准
- [x] 三层记忆作用域：shared（全局共享）、user（用户隔离）、session（会话隔离）
- [x] Storage 接口：SaveChunk、SaveChunksBatch、GetChunk、DeleteByPath、SearchVector、SearchKeyword
- [x] SQLite 存储后端（WAL 模式，单连接并发控制）
- [x] 向量搜索：cosine 相似度计算，支持作用域和用户过滤
- [x] 关键词搜索：LIKE 查询 + 关键词提取 + 评分
- [x] 混合搜索模式（SearchModeHybrid）
- [x] 文本分块器（chunker）：将长文本按行数/字符数分块
- [x] 摘要生成器（summarizer）：调用 LLM 生成记忆摘要
- [x] 向量嵌入（embedding）：调用 LLM 生成文本向量
- [x] 文件元数据跟踪（hash/mtime/size 变更检测）
- [x] 对话持久化（conversation_store）：会话历史存储和检索
