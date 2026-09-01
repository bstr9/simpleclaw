#!/usr/bin/env python3
"""Local helper: generate BATCH_REQ, push, wait for result, download.

Usage:
  python3 ts_batch.py --tasks tasks.jsonl
  # or inline:
  python3 ts_batch.py --api daily --params '{"ts_code":"000001.SZ","start_date":"20250101","end_date":"20250201"}'
"""
import argparse, json, subprocess, sys, time, os

REPO = "bstr9/simpleclaw"
WORKDIR = "/tmp/simpleclaw"
GIT_SSH = "ssh -o StrictHostKeyChecking=no -p 443"


def run(cmd, **kw):
    return subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=kw.pop("timeout", 120), **kw)


def build_tasks(args):
    """Build task list from --tasks file or --api/--params."""
    if args.tasks:
        with open(args.tasks) as f:
            return [json.loads(l) for l in f if l.strip() and not l.startswith("#")]
    task = {"api": args.api, "params": json.loads(args.params or "{}")}
    if args.fields:
        task["fields"] = args.fields
    return [task]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tasks", help="jsonl task file: {'api':..,'params':{}} per line")
    ap.add_argument("--api", help="single request API name")
    ap.add_argument("--params", help="single request params JSON")
    ap.add_argument("--fields", help="fields filter")
    ap.add_argument("--out", default="/tmp/batch_result.json", help="local output path")
    ap.add_argument("--timeout", type=int, default=900, help="max wait seconds")
    args = ap.parse_args()

    tasks = build_tasks(args)
    print(f"Tasks: {len(tasks)}")

    # sync repo
    run(f"cd {WORKDIR} && git pull -q --rebase origin main")

    # write BATCH_REQ
    with open(f"{WORKDIR}/BATCH_REQ", "w") as f:
        for t in tasks:
            f.write(json.dumps(t, ensure_ascii=False) + "\n")

    # commit + push to trigger
    run(f"cd {WORKDIR} && git add BATCH_REQ")
    c = run(f"cd {WORKDIR} && git -c user.name=jiwei.bao -c user.email=jiwei.bao@uii-ai.com commit -q -m 'batch {len(tasks)} tasks'")
    if "nothing to commit" not in c.stdout + c.stderr:
        p = run(f"cd {WORKDIR} && GIT_SSH_COMMAND='{GIT_SSH}' git push -q origin main", timeout=90)
        if p.returncode != 0:
            print("push failed:", p.stderr[-300:]); sys.exit(1)
    print("Pushed, waiting for Actions...")

    # find the run triggered by our push
    time.sleep(10)
    deadline = time.time() + args.timeout
    run_id = None
    while time.time() < deadline and not run_id:
        r = run(f"gh run list -R {REPO} --workflow=tushare-batch.yml --limit 1 --json databaseId,createdAt --jq '.[0].databaseId'")
        run_id = r.stdout.strip()
        if not run_id:
            time.sleep(10)
    print(f"Run: {run_id}")

    # wait for completion
    while time.time() < deadline:
        r = run(f"gh run view {run_id} -R {REPO} --json status,conclusion --jq '\"\\(.status)/\\(.conclusion)\"'")
        status = r.stdout.strip()
        print(f"  {status}")
        if "completed/" in status:
            break
        time.sleep(20)

    if "completed/success" not in status:
        print("FAILED:", status)
        r = run(f"gh run view {run_id} -R {REPO} --log-failed", timeout=60)
        print(r.stdout[-2000:])
        sys.exit(1)

    # find result file from the commit message
    r = run(f"cd {WORKDIR} && git pull -q --rebase origin main && ls batch_results/result_*.json | tail -1")
    result_file = r.stdout.strip()
    print(f"Result: {result_file}")

    # copy to local output
    with open(f"{WORKDIR}/{result_file}") as f:
        data = json.load(f)
    with open(args.out, "w") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
    print(f"Summary: total={data['total']} success={data['success']} failed={data['failed']}")
    print(f"Saved to {args.out}")


if __name__ == "__main__":
    main()
