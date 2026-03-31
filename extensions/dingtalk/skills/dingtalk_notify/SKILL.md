# Dingtalk Approval Skill

钉钉审批流程操作技能。

## Capabilities

- 发送工作通知消息
- 获取用户信息
- 获取部门列表

## Usage

当用户需要操作钉钉审批或发送通知时使用。

### Examples

```
用户: 给张三发送钉钉通知"明天开会"
助手: [调用 dingtalk 工具: {"action": "send_message", "user_id": "zhangsan", "message": "明天开会"}]
```
