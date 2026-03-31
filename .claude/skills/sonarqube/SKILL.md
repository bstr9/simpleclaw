---
name: sonarqube
description: SonarQube 代码质量和安全分析工具。用于查询代码问题、安全漏洞、覆盖率、代码重复等。当用户询问代码质量、安全扫描、SonarQube 分析结果、项目问题时使用此技能。触发词：sonar、代码质量、安全漏洞、代码问题、coverage、技术债务。
compatibility:
  - tools: [bash]
---

# SonarQube 技能

使用 CLI 脚本快速查询代码质量问题。

## 快速命令

```bash
# 查看问题汇总
python3 .claude/skills/sonarqube/scripts/sonarqube.py summary

# 获取问题列表
python3 .claude/skills/sonarqube/scripts/sonarqube.py issues

# 按严重级别过滤
python3 .claude/skills/sonarqube/scripts/sonarqube.py issues -s CRITICAL

# 按类型过滤
python3 .claude/skills/sonarqube/scripts/sonarqube.py issues -t BUG

# 获取度量指标
python3 .claude/skills/sonarqube/scripts/sonarqube.py metrics

# 获取安全热点
python3 .claude/skills/sonarqube/scripts/sonarqube.py hotspots

# 运行扫描
python3 .claude/skills/sonarqube/scripts/sonarqube.py scan
```

## 命令说明

| 命令 | 说明 |
|------|------|
| `summary` | 问题汇总统计 |
| `issues` | 问题列表 |
| `metrics` | 度量指标 |
| `hotspots` | 安全热点 |
| `scan` | 运行扫描 |
| `projects` | 列出项目 |

## 过滤选项

```bash
# 严重级别: CRITICAL, MAJOR, BLOCKER
python3 .claude/skills/sonarqube/scripts/sonarqube.py issues -s CRITICAL,MAJOR

# 问题类型: BUG, VULNERABILITY, CODE_SMELL
python3 .claude/skills/sonarqube/scripts/sonarqube.py issues -t BUG

# 限制数量
python3 .claude/skills/sonarqube/scripts/sonarqube.py issues -l 50

# JSON 输出
python3 .claude/skills/sonarqube/scripts/sonarqube.py issues --json
```

## SonarQube 信息

- **控制台**: http://localhost:9000
- **项目 Key**: `uai-claw`
