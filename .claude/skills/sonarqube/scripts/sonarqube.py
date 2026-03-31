#!/usr/bin/env python3
"""
SonarQube CLI - 代码质量和安全分析工具

用法:
    sonarqube.py issues          # 获取问题列表
    sonarqube.py metrics         # 获取度量指标
    sonarqube.py scan            # 运行扫描
    sonarqube.py hotspots        # 获取安全热点
    sonarqube.py summary         # 获取问题汇总
"""

import argparse
import json
import subprocess
import sys
import urllib.request
import urllib.error
import base64
from pathlib import Path

# 配置
SONAR_URL = "http://localhost:9000"
SONAR_TOKEN = "squ_1248d3428b95ccd727f0d9f09b02af73df36901a"
SCAN_TOKEN = "sqb_f0854dce1cbe1615a27f946aa68b5ac8b720cc84"

# 从 sonar-project.properties 读取项目 key
def get_project_key():
    props_file = Path("sonar-project.properties")
    if props_file.exists():
        for line in props_file.read_text().splitlines():
            if line.startswith("sonar.projectKey="):
                return line.split("=", 1)[1].strip()
    return "uai-claw"

PROJECT_KEY = get_project_key()


def api_get(endpoint: str, params: dict | None = None) -> dict:
    """调用 SonarQube API"""
    url = f"{SONAR_URL}/api/{endpoint}"
    if params:
        url += "?" + "&".join(f"{k}={v}" for k, v in params.items())
    
    req = urllib.request.Request(url)
    credentials = base64.b64encode(f"{SONAR_TOKEN}:".encode()).decode()
    req.add_header("Authorization", f"Basic {credentials}")
    
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        print(f"API 错误: {e.code} {e.reason}", file=sys.stderr)
        return {}


def print_table(data: list, columns: list):
    """打印表格"""
    if not data:
        print("无数据")
        return
    
    # 计算列宽
    widths = {col: len(col) for col in columns}
    for row in data:
        for col in columns:
            widths[col] = max(widths[col], len(str(row.get(col, ""))))
    
    # 打印表头
    header = " | ".join(col.ljust(widths[col]) for col in columns)
    print(header)
    print("-" * len(header))
    
    # 打印数据
    for row in data:
        print(" | ".join(str(row.get(col, "")).ljust(widths[col]) for col in columns))


def cmd_issues(args):
    page_size = 500 if args.limit == 0 else args.limit
    params = {
        "componentKeys": PROJECT_KEY,
        "ps": page_size,
    }
    
    if args.severity:
        params["severities"] = args.severity
    if args.type:
        params["types"] = args.type
    
    all_issues = []
    total = 0
    page = 1
    
    while True:
        params["p"] = page
        result = api_get("issues/search", params)
        issues = result.get("issues", [])
        total = result.get("total", 0)
        
        all_issues.extend(issues)
        
        if args.limit > 0 or len(all_issues) >= total:
            break
        page += 1
    
    if args.json:
        print(json.dumps({"total": total, "issues": all_issues}, indent=2, ensure_ascii=False))
        return
    
    data = []
    for issue in all_issues:
        data.append({
            "severity": issue.get("severity", ""),
            "type": issue.get("type", ""),
            "message": issue.get("message", "")[:60],
            "file": issue.get("component", "").split(":")[-1],
            "line": issue.get("line", ""),
        })
    
    shown = len(all_issues)
    if total > shown:
        print(f"\n找到 {total} 个问题 (显示前 {shown} 个)\n")
    else:
        print(f"\n找到 {total} 个问题\n")
    print_table(data, ["severity", "type", "file", "line", "message"])


def cmd_metrics(args):
    """获取度量指标"""
    metric_keys = "ncloc,complexity,cognitive_complexity,coverage,duplicated_lines_density,bugs,vulnerabilities,code_smells,sqale_rating,security_rating,reliability_rating"
    
    result = api_get("measures/component", {
        "component": PROJECT_KEY,
        "metricKeys": metric_keys
    })
    
    measures = result.get("component", {}).get("measures", [])
    
    if args.json:
        print(json.dumps(measures, indent=2, ensure_ascii=False))
        return
    
    print(f"\n项目: {PROJECT_KEY}\n")
    for m in measures:
        metric = m.get("metric", "")
        value = m.get("value", "N/A")
        best = m.get("bestValue", False)
        status = "✓" if best else "✗"
        print(f"  {status} {metric}: {value}")


def cmd_hotspots(args):
    """获取安全热点"""
    result = api_get("hotspots/search", {
        "projectKey": PROJECT_KEY,
        "ps": args.limit
    })
    
    hotspots = result.get("hotspots", [])
    
    if args.json:
        print(json.dumps(hotspots, indent=2, ensure_ascii=False))
        return
    
    data = []
    for h in hotspots:
        data.append({
            "status": h.get("status", ""),
            "severity": h.get("severity", ""),
            "file": h.get("component", "").split(":")[-1],
            "line": h.get("line", ""),
            "message": h.get("message", "")[:50],
        })
    
    print(f"\n找到 {result.get('paging', {}).get('total', 0)} 个安全热点\n")
    print_table(data, ["severity", "status", "file", "line", "message"])


def cmd_summary(args):
    """获取问题汇总"""
    result = api_get("issues/search", {
        "componentKeys": PROJECT_KEY,
        "facets": "severities,types,rules",
        "ps": 1
    })
    
    facets = {f["property"]: f["values"] for f in result.get("facets", [])}
    
    print(f"\n{'='*50}")
    print(f" SonarQube 分析报告: {PROJECT_KEY}")
    print(f"{'='*50}")
    
    # 严重级别统计
    print("\n按严重级别:")
    for v in facets.get("severities", []):
        print(f"  {v['val']:12} {v['count']:5}")
    
    # 类型统计
    print("\n按类型:")
    for v in facets.get("types", []):
        print(f"  {v['val']:12} {v['count']:5}")
    
    # 规则统计 (前5)
    print("\n按规则 (Top 5):")
    for v in facets.get("rules", [])[:5]:
        print(f"  {v['val']:12} {v['count']:5}")
    
    print(f"\n总计问题: {result.get('total', 0)}")
    print(f"查看详情: {SONAR_URL}/dashboard?id={PROJECT_KEY}\n")


def cmd_scan(args):
    """运行扫描"""
    import os
    
    # 检查覆盖率文件
    if not Path("coverage.out").exists():
        print("生成覆盖率报告...")
        subprocess.run(["go", "test", "./...", "-coverprofile=coverage.out", "-covermode=atomic"], check=True)
    
    print(f"运行 SonarQube 扫描...")
    print(f"项目: {PROJECT_KEY}")
    print(f"URL: {SONAR_URL}")
    
    cmd = ["/opt/sonar-scanner/bin/sonar-scanner", f"-Dsonar.token={SCAN_TOKEN}"]
    
    if args.dry_run:
        print(f"命令: {' '.join(cmd)}")
        return
    
    result = subprocess.run(cmd)
    if result.returncode == 0:
        print(f"\n扫描完成!")
        print(f"查看结果: {SONAR_URL}/dashboard?id={PROJECT_KEY}")
    else:
        print(f"\n扫描失败: {result.returncode}")


def cmd_projects(args):
    """列出所有项目"""
    result = api_get("projects/search")
    components = result.get("components", [])
    
    if args.json:
        print(json.dumps(components, indent=2, ensure_ascii=False))
        return
    
    data = []
    for c in components:
        data.append({
            "key": c.get("key", ""),
            "name": c.get("name", ""),
            "lastAnalysis": c.get("lastAnalysisDate", "")[:10] if c.get("lastAnalysisDate") else "",
        })
    
    print_table(data, ["key", "name", "lastAnalysis"])


def main():
    parser = argparse.ArgumentParser(
        description="SonarQube CLI - 代码质量和安全分析工具",
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    subparsers = parser.add_subparsers(dest="command", help="命令")
    
    # issues 命令
    issues_parser = subparsers.add_parser("issues", help="获取问题列表")
    issues_parser.add_argument("-s", "--severity", help="严重级别: CRITICAL,MAJOR,BLOCKER")
    issues_parser.add_argument("-t", "--type", help="问题类型: BUG,VULNERABILITY,CODE_SMELL")
    issues_parser.add_argument("-l", "--limit", type=int, default=20, help="限制数量 (0=全部)")
    issues_parser.add_argument("--json", action="store_true", help="JSON 输出")
    issues_parser.set_defaults(func=cmd_issues)
    
    # metrics 命令
    metrics_parser = subparsers.add_parser("metrics", help="获取度量指标")
    metrics_parser.add_argument("--json", action="store_true", help="JSON 输出")
    metrics_parser.set_defaults(func=cmd_metrics)
    
    # hotspots 命令
    hotspots_parser = subparsers.add_parser("hotspots", help="获取安全热点")
    hotspots_parser.add_argument("-l", "--limit", type=int, default=20, help="限制数量")
    hotspots_parser.add_argument("--json", action="store_true", help="JSON 输出")
    hotspots_parser.set_defaults(func=cmd_hotspots)
    
    # summary 命令
    summary_parser = subparsers.add_parser("summary", help="获取问题汇总")
    summary_parser.set_defaults(func=cmd_summary)
    
    # scan 命令
    scan_parser = subparsers.add_parser("scan", help="运行扫描")
    scan_parser.add_argument("--dry-run", action="store_true", help="仅打印命令")
    scan_parser.set_defaults(func=cmd_scan)
    
    # projects 命令
    projects_parser = subparsers.add_parser("projects", help="列出所有项目")
    projects_parser.add_argument("--json", action="store_true", help="JSON 输出")
    projects_parser.set_defaults(func=cmd_projects)
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return
    
    args.func(args)


if __name__ == "__main__":
    main()
