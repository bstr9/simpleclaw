# Feishu Doc Skill

操作飞书文档的技能。

## Capabilities

- 读取飞书文档内容
- 写入/更新飞书文档
- 创建新文档
- 追加内容到文档

## Usage

当用户需要操作飞书文档时，使用 `feishu_doc` 工具。

### Examples

```
用户: 帮我读取飞书文档 ABC123
助手: 我来读取该文档内容。
[调用 feishu_doc 工具: {"action": "read", "doc_token": "ABC123"}]
```

```
用户: 创建一个新文档，标题是"会议纪要"
助手: [调用 feishu_doc 工具: {"action": "create", "title": "会议纪要"}]
```
