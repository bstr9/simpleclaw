---
id: REQ-012
title: "长期记忆与向量搜索"
status: active
level: story
priority: P0
cluster: agent-memory
created_at: "2026-04-23T10:10:00"
updated_at: "2026-04-23T18:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-003]
  merged_from: []
  depends_on: [REQ-005]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T10:10:00"
    author: ai
    context: "从 REQ-003 拆分，来源: pkg/agent/memory/long_term.go"
    reason: "Epic 拆分为 Story"
    snapshot: "长期记忆：向量嵌入、cosine 相似度搜索、SQLite 存储、文本分块与索引"
  - version: 2
    date: "2026-04-23T18:00:00"
    author: ai
    context: "代码逆向分析，扩展验收标准"
    reason: "基于代码分析扩展验收标准至25条"
    snapshot: "长期记忆系统：SQLite WAL模式、混合搜索(0.7/0.3权重)、时间衰减、3种分块策略、EmbeddingCache、非致命降级、4表结构"
source_code:
  - pkg/agent/memory/memory.go
  - pkg/agent/memory/storage.go
  - pkg/agent/memory/long_term.go
  - pkg/agent/memory/manager.go
  - pkg/agent/memory/embedding.go
  - pkg/agent/memory/short_term.go
  - pkg/agent/memory/conversation_store.go
  - pkg/agent/memory/chunker.go
  - pkg/agent/memory/summarizer.go
---

# 长期记忆与向量搜索

## 描述
Agent 长期记忆子系统，将文本分块后生成向量嵌入，存储到 SQLite。支持向量相似度搜索（cosine）、关键词搜索和混合搜索。文件变更时自动更新索引。使用纯Go SQLite驱动（modernc.org/sqlite，无CGO依赖），WAL模式提升并发读性能。嵌入失败时非致命降级为关键词搜索。

## 验收标准
- [x] MemoryScope三种作用域：ScopeShared="shared"（全局共享）, ScopeUser="user"（用户级别）, ScopeSession="session"（会话级别）
- [x] MemorySource两种来源：SourceMemory="memory"（持久记忆）, SourceSession="session"（会话记忆）
- [x] 四个核心接口：Memory(Add/Get/Clear/Summarize/Close), Searcher(Search/SearchVector/SearchKeyword), Embedder(Embed/EmbedBatch/Dimensions), Summarizer(Summarize/Flush)
- [x] MemoryChunk结构体：ID(content-hash生成), UserID, Scope, Source, Path, StartLine, EndLine, Text, Embedding([]float64), Hash(SHA256), Metadata, CreatedAt, UpdatedAt
- [x] SearchResult结构体：Path, StartLine, EndLine, Score(float64), Snippet(截断500字符), Source, UserID
- [x] SearchOptions结构体：UserID, Scopes([]MemoryScope), MaxResults(默认10), MinScore(默认0.1), IncludeShared(bool)
- [x] Config默认值：EmbeddingModel="text-embedding-3-small", EmbeddingDim=1536, ChunkMaxTokens=500, ChunkOverlapTokens=50, MaxResults=10, MinScore=0.1, VectorWeight=0.7, KeywordWeight=0.3, EnableAutoSync=true, SyncOnSearch=true, MaxAgeDays=30
- [x] SQLite存储：使用modernc.org/sqlite（纯Go，无CGO），WAL模式，5000ms busy_timeout，单连接池(SetMaxOpenConns(1))
- [x] 4张SQLite表：chunks, files, sessions, messages
- [x] 混合搜索权重：VectorWeight=0.7, KeywordWeight=0.3，按key去重合并结果
- [x] 时间衰减计算：exp(-ln(2)/30 * ageDays)，仅对YYYY-MM-DD.md格式文件应用；非日期格式文件(evergreen)衰减系数固定1.0
- [x] Cosine相似度：标准dot-product / (norm1 * norm2)计算，维度不匹配或零范数时返回0.0
- [x] 关键词搜索：正则[\p{L}\p{N}]+提取关键词（最少2字符），LIKE逐词查询，启发式评分(基础0.5 + 每匹配0.1, 上限1.0)
- [x] 嵌入失败非致命：记忆存储时向量生成失败不影响存储，搜索时降级为纯关键词搜索
- [x] 长期记忆初始化失败非致命：Manager中LongTermMemory初始化失败仅记录警告，继续运行
- [x] Manager.AddMessage：仅assistant消息(msg.Role == RoleAssistant)进入长期记忆，user消息仅存入conversation store
- [x] 文本分块3种策略：ChunkText(行级分块)、ChunkMarkdown(结构感知，保留代码块完整性)、ChunkByParagraph(段落分块)
- [x] 分块默认参数：500 tokens上限, 50 tokens重叠, 4 chars/token换算比
- [x] EmbeddingCache：MD5键, 最大10000条, 满时随机淘汰一半（非LRU策略）
- [x] OpenAIEmbeddingProvider：默认text-embedding-3-small/1536维度, 30秒超时, API Key验证（拒绝空字符串、"YOUR API KEY"、"YOUR_API_KEY"）
- [x] MemoryFlushManager：LLM摘要生成+规则降级回退, 每日记忆文件, flush触发条件：trim/overflow/daily_summary
- [x] ConversationStore：SQLite存储, sessions+messages表, LoadMessages(maxTurns默认30), 分页LoadHistoryPage含DisplayTurn渲染
- [x] Schema重复定义：storage.go和long_term.go独立定义chunks+files表，Storage接口未被LongTermMemory使用
- [x] 无chunk级TTL：仅messages表有CleanupOldMessages，chunks无过期清理机制
- [x] 文件变更检测：通过hash/mtime/size检测文件变更，自动重新索引

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| MemoryScope三种作用域 | memory.go:MemoryScope常量定义 |
| MemorySource两种来源 | memory.go:MemorySource常量定义 |
| 四个核心接口 | memory.go:Memory/Searcher/Embedder/Summarizer接口 |
| MemoryChunk结构体 | memory.go:MemoryChunk结构体 |
| SearchResult结构体 | memory.go:SearchResult结构体 |
| SearchOptions默认值 | memory.go:SearchOptions结构体及默认值 |
| Config默认值 | memory.go:Config结构体及默认值 |
| SQLite WAL/单连接 | long_term.go:LongTermMemory初始化 |
| 4张SQLite表 | long_term.go:initDB() / storage.go:initDB() |
| 混合搜索权重 | long_term.go:Search()混合合并逻辑 |
| 时间衰减计算 | long_term.go:时间衰减函数 |
| Cosine相似度 | long_term.go:cosineSimilarity() |
| 关键词搜索 | long_term.go:SearchKeyword() |
| 嵌入失败非致命 | long_term.go:AddFile()/索引逻辑 |
| 初始化失败非致命 | manager.go:NewManager() |
| 仅assistant入长期记忆 | manager.go:AddMessage() |
| 3种分块策略 | chunker.go:ChunkText/ChunkMarkdown/ChunkByParagraph |
| 分块默认参数 | chunker.go:TextChunker默认配置 |
| EmbeddingCache | embedding.go:EmbeddingCache结构体 |
| OpenAIEmbeddingProvider | embedding.go:OpenAIEmbeddingProvider |
| MemoryFlushManager | summarizer.go:MemoryFlushManager |
| ConversationStore | conversation_store.go:ConversationStore |
| Schema重复定义 | storage.go + long_term.go:chunks/files表定义 |
| 无chunk级TTL | long_term.go:无chunk过期清理逻辑 |
| 文件变更检测 | long_term.go:AddFile()/索引更新逻辑 |
