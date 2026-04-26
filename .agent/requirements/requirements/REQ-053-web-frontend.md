---
id: REQ-053
title: "Web 前端 SPA"
status: active
level: story
priority: P1
cluster: bridge
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-004]
  merged_from: []
  depends_on: [REQ-016]
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "基于 Vue 3 + TypeScript 的 Web 前端 SPA，提供聊天界面、管理后台、设置向导"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "细化验收标准，从源代码逆向补充实现细节"
    reason: "验收标准从6项扩展至25项，覆盖应用入口、路由守卫、API层、状态管理、聊天组件、管理后台、认证系统等"
    snapshot: "Vue3+TS SPA，Pinia状态管理(auth/chat/config stores)，ElementPlus zh-CN，4路由(/setup /login / /admin)，SSE流式聊天，JWT认证，明暗主题切换，管理后台(Dashboard+Config)，设置向导"
source_code:
  - web/
---

# Web 前端 SPA

## 描述
基于 Vue 3 + TypeScript 的 Web 前端 SPA，提供聊天界面、管理后台、设置向导等功能。应用入口 main.ts 注册 Pinia 状态管理、Vue Router 路由、Element Plus（zh-CN 本地化+图标）。App.vue 支持明暗主题切换（localStorage 持久化）+ ElConfigProvider。路由 4 条：/setup（初始化向导）、/login（登录）、/（Chat 首页）、/admin（管理后台含 Dashboard+Config），beforeEach 守卫检查认证状态。API 层基于 axios，Bearer token 认证，401 自动跳转登录。3 个 Pinia Store：auth（token+user 持久化）、chat（sessions/messages Map、SSE 流式通信、工具调用事件）、config（系统配置/状态/渠道/模型提供商）。聊天组件 4 个：InputBox、MessageItem、MessageList、Sidebar。

## 代码参考

| 功能 | 文件 | 行号 |
|------|------|------|
| 应用入口 | web/src/main.ts | - |
| 根组件+主题切换 | web/src/App.vue | - |
| 路由配置+守卫 | web/src/router/index.ts | - |
| API 层 (axios+SSE) | web/src/api/index.ts | - |
| auth Store | web/src/stores/auth.ts | - |
| chat Store | web/src/stores/chat.ts | - |
| config Store | web/src/stores/config.ts | - |
| 类型定义 | web/src/types/index.ts | - |
| 聊天首页 | web/src/views/Chat/Index.vue | - |
| 管理后台布局 | web/src/views/Admin/Layout.vue | - |
| 仪表盘 | web/src/views/Admin/Dashboard.vue | - |
| 配置页 | web/src/views/Admin/Config.vue | - |
| 登录页 | web/src/views/Login.vue | - |
| 设置向导 | web/src/views/SetupWizard.vue | - |
| 输入框组件 | web/src/components/chat/InputBox.vue | - |
| 消息项组件 | web/src/components/chat/MessageItem.vue | - |
| 消息列表组件 | web/src/components/chat/MessageList.vue | - |
| 侧边栏组件 | web/src/components/chat/Sidebar.vue | - |
| 设计令牌 | web/src/styles/design-tokens.css | - |
| Element 覆盖样式 | web/src/styles/element-override.css | - |
| 主样式 | web/src/styles/main.css | - |

## 验收标准
- [x] Vue 3 + TypeScript SPA 架构，Vite 构建
- [x] main.ts 注册 createApp + Pinia + Router + ElementPlus（zh-CN 本地化 + 图标注册）
- [x] App.vue 明暗主题切换，localStorage 持久化主题偏好，ElConfigProvider 包裹
- [x] 4 条路由：/setup（设置向导）、/login（登录）、/（Chat 首页）、/admin（管理后台）
- [x] /admin 路由含嵌套 Dashboard + Config 子页面，Layout 布局组件
- [x] Router beforeEach 守卫检查认证状态，未登录跳转 /login
- [x] API 层基于 axios，baseURL 为 /admin/api，Bearer token 请求头注入
- [x] API 层 401 响应拦截，自动跳转登录页
- [x] authApi：login/logout 接口
- [x] configApi：setup/getStatus/getConfig/updateConfig/getChannels/getProviders/testLlm 接口
- [x] chatApi：sendMessage/upload/getConfig/getProviders 接口
- [x] createSSEConnection 实现 SSE 流式通信
- [x] auth Store：token + user 持久化到 localStorage，isAuthenticated/isAdmin 计算属性，login/logout/clearAuth 方法
- [x] chat Store：sessions Map + messages Map，createSession/selectSession/deleteSession 方法
- [x] chat Store：addUserMessage/addAssistantMessage，sendMessage 方法集成 SSE
- [x] chat Store：handleSSEEvent 处理 delta/done/error/tool_start/tool_end 事件类型
- [x] chat Store：cancelRequest 取消请求，clearMessages 清空消息，saveToStorage/loadFromStorage 持久化
- [x] config Store：config/status/channels/providers 状态管理，fetch/update/testLlm/setup 方法
- [x] 类型定义：User, LoginRequest, SystemStatus, Channel, LLMProvider, Message, ToolCall, Session, ChatConfig, SSEEvent, UploadResponse, ApiResponse
- [x] 聊天组件：InputBox（消息输入）、MessageItem（单条消息渲染）、MessageList（消息列表）、Sidebar（会话侧边栏）
- [x] Chat/Index.vue 聊天首页视图
- [x] Admin/Layout.vue 管理后台布局 + Dashboard.vue 仪表盘 + Config.vue 配置页
- [x] Login.vue 登录页面
- [x] SetupWizard.vue 初始设置向导
- [x] 样式系统：design-tokens.css（设计令牌）、element-override.css（Element 样式覆盖）、main.css（全局样式）
- [x] Admin Server 反向代理集成，前端 API 请求代理至后端 /admin/api
