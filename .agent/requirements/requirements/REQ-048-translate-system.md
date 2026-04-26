---
id: REQ-048
title: "翻译系统"
status: active
level: story
priority: P2
cluster: bridge
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-008]
  merged_from: []
  depends_on: []
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "多提供商翻译服务，支持百度翻译和 Google/DeepL/有道占位"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，扩展验收标准至完整覆盖"
    reason: "从现有代码逆向补充详细验收标准"
    snapshot: "翻译系统：Translator接口、工厂注册、Builder构建、实例缓存、百度翻译完整实现（MD5签名/重试/超时）、4提供商占位注册"
source_code:
  - pkg/translate/translator.go
  - pkg/translate/factory.go
  - pkg/translate/baidu/baidu.go
---

# 翻译系统

## 描述
多提供商翻译服务，支持百度翻译（已实现）和 Google/DeepL/有道（占位）。工厂模式注册，Builder 模式构建，支持缓存和别名。Translator 接口定义统一的翻译方法，各提供商通过工厂注册，Builder 模式简化构建过程，实例缓存避免重复创建。百度翻译实现了完整的 API 对接流程，包括 MD5 签名验证、线性退避重试和超时控制。

## 验收标准

### 核心接口
- [x] Translator 接口定义 Translate(text, from, to) (string, error) 方法
- [x] from 参数为空字符串时表示自动检测源语言
- [x] 语言代码遵循 ISO 639-1 标准

### 工厂注册模式
- [x] TranslatorCreator 类型定义：func() (Translator, error)
- [x] translatorRegistry 结构体：sync.RWMutex 互斥锁 + creators map 并发安全
- [x] RegisterTranslator(name, creator) 注册翻译器，重复注册时输出警告日志
- [x] RegisterTranslator 注册成功时输出 debug 级别日志
- [x] CreateTranslator(translatorType) 根据类型名创建翻译器实例，未注册时返回错误
- [x] GetRegisteredTranslators() 返回所有已注册的翻译器类型列表
- [x] IsTranslatorRegistered(name) 检查指定翻译器是否已注册

### 预定义提供商常量
- [x] TranslatorBaidu="baidu" 常量定义
- [x] TranslatorGoogle="google" 常量定义
- [x] TranslatorDeepL="deepl" 常量定义
- [x] TranslatorYoudao="youdao" 常量定义

### Builder 模式
- [x] TranslatorBuilder 结构体：translatorType + aliases + creator
- [x] NewTranslatorBuilder(translatorType) 创建 Builder 实例
- [x] WithAliases(aliases...) 设置翻译器别名
- [x] WithCreator(creator) 设置创建函数
- [x] Register() 将 Builder 配置注册到工厂

### 实例缓存
- [x] translatorCache 结构体：sync.RWMutex + instances map 并发安全
- [x] GetCachedTranslator(name) 获取已缓存的翻译器实例
- [x] SetCachedTranslator(name, translator) 缓存翻译器实例
- [x] ClearTranslatorCache(name) 清除指定名称的缓存实例
- [x] ClearAllTranslatorCache() 清除所有缓存实例

### 百度翻译实现
- [x] baiduEndpoint="https://fanyi-api.baidu.com" + baiduPath="/api/trans/vip/translate" 端点常量
- [x] requestTimeout=10s 请求超时配置
- [x] maxRetries=3 最大重试次数
- [x] 错误码常量定义：ErrCodeSuccess/Timeout/SystemError/Unauthorized/InvalidParam
- [x] BaiduTranslator 结构体：appID + appKey + client *http.Client
- [x] baiduResponse 结构体解析：From/To/TransResult[]{Src,Dst}/ErrorCode/ErrorMsg
- [x] Config 结构体：AppID + AppKey 配置
- [x] NewBaiduTranslator(cfg) 校验 AppID 和 AppKey 非空
- [x] Translate 方法：校验参数，from 为空时自动设为 "auto"
- [x] generateSign(query, salt) 生成 MD5(appid+query+salt+appkey) 签名
- [x] generateSalt() 生成 [32768, 65535] 范围随机数
- [x] 重试机制：线性退避 100ms * retry 次数
- [x] isRetryableError(err) 判断错误是否可重试（当前始终返回 true）
- [x] init() 注册到工厂，返回建议直接构造的错误信息

### 占位提供商
- [x] Google/DeepL/有道：init() 中占位注册，调用时返回"尚未实现"错误

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| Translator 接口 | `pkg/translate/translator.go` |
| 工厂注册/创建/查询 | `pkg/translate/factory.go` RegisterTranslator/CreateTranslator/GetRegisteredTranslators/IsTranslatorRegistered |
| 预定义常量 | `pkg/translate/factory.go` TranslatorBaidu/Google/DeepL/Youdao |
| Builder 模式 | `pkg/translate/factory.go` TranslatorBuilder/NewTranslatorBuilder/WithAliases/WithCreator/Register |
| 实例缓存 | `pkg/translate/factory.go` translatorCache/GetCachedTranslator/SetCachedTranslator/ClearTranslatorCache/ClearAllTranslatorCache |
| 百度翻译端点/超时/重试 | `pkg/translate/baidu/baidu.go` 常量定义区 |
| 百度错误码 | `pkg/translate/baidu/baidu.go` ErrCodeSuccess/Timeout/SystemError/Unauthorized/InvalidParam |
| 百度翻译结构体 | `pkg/translate/baidu/baidu.go` BaiduTranslator/baiduResponse/Config |
| 百度签名/盐值 | `pkg/translate/baidu/baidu.go` generateSign/generateSalt |
| 百度重试逻辑 | `pkg/translate/baidu/baidu.go` Translate 重试循环 + isRetryableError |
| 占位注册 | `pkg/translate/factory.go` init() 函数 |
