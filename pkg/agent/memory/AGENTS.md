# AGENTS.md — 记忆系统

**目录:** pkg/agent/memory/ | **文件:** 9 | **深度:** 4

---

## OVERVIEW

三层记忆架构：短期、长期、会话。支持向量搜索和关键词搜索。

---

## STRUCTURE

```
pkg/agent/memory/
├── memory.go              # 接口定义
├── storage.go             # SQLite 存储层
├── short_term.go          # 短期记忆
├── long_term.go           # 长期记忆 (向量搜索)
├── conversation_store.go  # 对话持久化
├── chunker.go             # 文本分块
├── summarizer.go          # 摘要生成
└── embedding.go           # 向量嵌入
```

---

## SCOPES

| 作用域 | 常量 | 说明 |
|--------|------|------|
| 共享 | `ScopeShared` | 全局可见 |
| 用户 | `ScopeUser` | 用户隔离 |
| 会话 | `ScopeSession` | 会话隔离 |

---

## STORAGE INTERFACE

```go
type Storage interface {
    Save(ctx context.Context, memory *Memory) error
    Search(ctx context.Context, query string, opts ...SearchOption) ([]*Memory, error)
    Delete(ctx context.Context, id string) error
}
```

---

## SEARCH MODES

- **向量搜索**: `SearchModeVector` - 语义相似
- **关键词搜索**: `SearchModeKeyword` - 精确匹配
- **混合搜索**: `SearchModeHybrid` - 综合

---

## HOTSPOT

`long_term.go` (1011行) — 建议拆分为:
- `sqlite_store.go` - SQLite 操作
- `vector_search.go` - 向量索引
- `indexer.go` - 索引管理
