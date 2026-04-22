---
id: REQ-006
title: "插件系统"
status: active
level: epic
priority: P1
cluster: plugins
created_at: "2026-04-23T10:00:00"
updated_at: "2026-04-23T10:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: [REQ-004]
  related_to: [REQ-001, REQ-002]
  refined_by: [REQ-025, REQ-026, REQ-027, REQ-028, REQ-029]
versions:
  - version: 1
    date: "2026-04-23T10:00:00"
    author: ai
    context: "从代码逆向分析提取需求，来源: pkg/plugin/"
    reason: "逆向代码生成需求"
    snapshot: "事件驱动插件架构，9 个插件，支持消息处理、敏感词过滤、命令扩展、工具调用"
---

# 插件系统

## 描述
事件驱动的插件架构，支持消息生命周期拦截和扩展。插件通过 OnInit/OnLoad/OnUnload/OnEvent 生命周期管理，通过 EventBus 发布订阅事件。支持优先级排序、依赖管理、配置加载。

## 验收标准
- [x] Plugin 接口：Name()、Version()、OnInit(ctx)、OnLoad(ctx)、OnUnload(ctx)、OnEvent(event, ctx)
- [x] BasePlugin 基础实现：默认生命周期、事件处理器注册/注销、元数据管理
- [x] PluginContext 运行时上下文：Config、PluginDir、DataDir、LogDir、EventBus
- [x] EventBus 事件总线：Subscribe/Publish，线程安全
- [x] 事件流：OnReceiveMessage → OnHandleContext → OnDecorateReply → OnSendReply
- [x] 事件动作：ActionContinue（继续）、ActionBreak（停止并执行默认）、ActionBreakPass（停止并跳过默认）
- [x] 插件元数据：Name、NameCN、Version、Description、Priority、Hidden、Enabled、Dependencies
- [x] 插件配置：全局配置 + 插件目录配置，JSON 格式，LoadConfig/SaveConfig
- [x] tool 插件：工具调用框架（1654 行），桥接 Agent Tool 系统
- [x] linkai 插件：LinkAI 集成（1945 行），知识库/Midjourney/摘要
- [x] godcmd 插件：管理员命令（768 行），权限管理
- [x] banwords 插件：敏感词过滤
- [x] keyword 插件：关键词自动回复
- [x] hello 插件：示例/欢迎插件
- [x] dungeon 插件：文字冒险游戏
- [x] agent 插件：Agent 模式封装
- [x] finish 插件：结束处理
