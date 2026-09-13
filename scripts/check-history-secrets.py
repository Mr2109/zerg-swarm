#!/usr/bin/env python3
"""全历史敏感审计（B0 / G1）—— 镜像私有历史**之前**的只读前置审计。

为什么必须先做：逐提交镜像会把**过去的每一笔**也过滤后公开 ⇒ 若历史上某笔曾提交过凭据/私有路径，
那笔会**永久**进入公开历史（压平快照恰好"顺手掩盖"了这个风险）。

本脚本**只读**：遍历指定范围的每一笔提交，按**发布白名单**取出"真会被公开的文件集"，
逐 blob 扫描敏感模式，并扫描提交信息本身；命中即逐条打印（敏感值一律**打码**），exit 1。

用法：
    python3 scripts/check-history-secrets.py                     # 默认：--all（全历史）
    python3 scripts/check-history-secrets.py --range A..B        # 只审区间（B1 复用）
    python3 scripts/check-history-secrets.py --json /tmp/audit.json
"""
import argparse
import json
import os
import re
import subprocess
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
RULES_FILE = os.path.join(REPO, "publish", "replace-rules.tsv")
WHITELIST = os.path.join(REPO, "publish", "whitelist.txt")

# 阻断级（命中即不应公开）—— 逐条给"为什么"
BLOCK = [
    (r"/Volumes/(AHZ|BXC)", "私有卷绝对路径"),
    (r"~", "用户家目录路径"),
    (r"ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}", "GitHub 令牌"),
    (r"sk-[A-Za-z0-9]{20,}", "API 密钥（sk-）"),
    (r"AKIA[0-9A-Z]{16}", "AWS Access Key"),
    (r"xox[baprs]-[A-Za-z0-9-]{10,}", "Slack 令牌"),
    (r"-----BEGIN [A-Z ]*PRIVATE KEY-----", "私钥块"),
    (r"(?i)\b(password|passwd|secret|api[_-]?key|access[_-]?token)\s*[:=]\s*[\"']?[A-Za-z0-9._\-]{12,}", "疑似明文口令/密钥赋值"),
    (r"\bBearer\s+[A-Za-z0-9._\-]{20,}", "Bearer 令牌"),
    (r"192\.168\.\d{1,3}\.\d{1,3}", "内网地址 192.168.x"),
    (r"\b10\.0\.0\.\d{1,3}", "内网地址 10.0.0.x"),
]
# 提示级（会被替换规则过滤掉，但仍值得知道）
INFO = [
    (r"(?i)\.zerg/token|auth\.json", "提到私有凭据文件路径（路径引用，非密钥值）"),
    (r"<project-a>|<project-b>", "私有项目名（替换规则会过滤）"),
    (r"(?i)Mr2109", "人物名（替换规则会过滤）"),
]
BINARY_SUFFIX = (".png", ".jpg", ".jpeg", ".gif", ".pdf", ".zip", ".gz", ".tgz", ".bundle",
                 ".dylib", ".so", ".a", ".o", ".ttf", ".ttc", ".otf", ".woff", ".woff2",
                 ".mp3", ".mp4", ".mov", ".webp", ".ico", ".icns", ".lock")
MAX_BLOB = 2 * 1024 * 1024


def sh(args, cwd=REPO, binary=False):
    r = subprocess.run(args, cwd=cwd, capture_output=True,
                       text=not binary, encoding=None if binary else "utf-8",
                       errors=None if binary else "replace")
    return r.stdout


def load_whitelist(path=WHITELIST):
    entries = []
    with open(path, encoding="utf-8") as fh:
        for raw in fh:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            entries.append(line)
    return entries


PUBLISH_SH = os.path.join(REPO, "scripts", "publish-public.sh")
NOISE_SUFFIX = (".bak", ".orig", ".rej")
NOISE_DIRS = ("__pycache__",)


def load_excludes(path=PUBLISH_SH):
    """从 publish-public.sh 的 EXCLUDES=( ... ) 数组里解析排除项——单一真源，避免两处漂移。"""
    ex = []
    if not os.path.exists(path):
        return ex
    text = open(path, encoding="utf-8").read()
    m = re.search(r"EXCLUDES=\((.*?)\n\)", text, re.S)
    if not m:
        return ex
    for raw in m.group(1).split("\n"):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        line = line.split("#")[0].strip().strip('"').strip("'")
        if line:
            ex.append(line)
    return ex


def excluded(rel, ex):
    if rel.endswith(NOISE_SUFFIX) or any(part in NOISE_DIRS for part in rel.split("/")):
        return True
    for e in ex:
        if rel == e or rel.startswith(e.rstrip("/") + "/"):
            return True
    return False


def load_rules(path=RULES_FILE):
    """发布替换规则：默认字面量，`regex:` 前缀为正则。必须先套规则再扫描——
    否则会把"本来就会被改写成占位符"的内容误报成泄露（第一版审计的硬伤）。"""
    literals, regexes = [], []
    if not os.path.exists(path):
        return literals, regexes
    with open(path, encoding="utf-8") as fh:
        for raw in fh:
            line = raw.rstrip("\n")
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            parts = line.split("\t")
            if len(parts) < 2:
                continue
            pat, rep = parts[0], parts[1]
            if pat.startswith("regex:"):
                try:
                    regexes.append((re.compile(pat[6:]), rep))
                except re.error:
                    pass
            else:
                literals.append((pat, rep))
    return literals, regexes


def apply_rules(text, literals, regexes):
    for pat, rep in literals:
        if pat in text:
            text = text.replace(pat, rep)
    for rx, rep in regexes:
        text = rx.sub(rep, text)
    return text


def published_path(rel, entries):
    for e in entries:
        if e.endswith("/"):
            if rel.startswith(e):
                return True
        elif rel == e:
            return True
    return False


def mask(s):
    s = s.strip()
    return s[:6] + "…" + ("（%d 字符）" % len(s)) if len(s) > 8 else s


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", default=REPO, help="仓库路径（默认本仓；自测/B1 复用时可指向别处）")
    ap.add_argument("--range", dest="rng", default=None, help="提交区间，如 A..B；默认 --all")
    ap.add_argument("--json", dest="json_out", default=None)
    ap.add_argument("--max-hits", type=int, default=200)
    args = ap.parse_args()

    repo = os.path.abspath(args.repo)
    if not os.path.isdir(os.path.join(repo, ".git")):
        print("✗ 不是 git 仓库：%s" % repo)
        return 2

    literals, regexes = load_rules()
    ex_list = load_excludes()
    print("排除项: %d 条（从 publish-public.sh 的 EXCLUDES 解析）+ 噪声后缀 %s" % (len(ex_list), "/".join(NOISE_SUFFIX)))
    print("替换规则: %d 条字面 + %d 条正则（扫描前先套用——审计的是『真会公开的形状』）" % (len(literals), len(regexes)))
    entries = load_whitelist(os.path.join(repo, "publish", "whitelist.txt") if os.path.exists(
        os.path.join(repo, "publish", "whitelist.txt")) else WHITELIST)
    if args.rng:
        revs = sh(["git", "rev-list", args.rng], cwd=repo).split()
        scope = args.rng
    else:
        revs = sh(["git", "rev-list", "--all"], cwd=repo).split()
        scope = "--all"
    print("审计范围: %s（%d 笔提交）" % (scope, len(revs)))
    print("白名单条目: %d（只审『真会被公开』的那些路径）" % len(entries))

    blob_cache = {}  # blob sha → (kind, why, sample)
    hits = []
    info_hits = []
    scanned_files = 0
    scanned_bytes = 0

    blk = [(re.compile(rx), why) for rx, why in BLOCK]
    inf = [(re.compile(rx), why) for rx, why in INFO]

    for idx, sha in enumerate(revs, 1):
        # ① 提交信息本身也会被镜像 ⇒ 一起审
        msg = apply_rules(sh(["git", "log", "-1", "--format=%B", sha], cwd=repo), literals, regexes)
        for rx, why in blk:
            m = rx.search(msg)
            if m:
                hits.append({"commit": sha[:10], "path": "<commit-message>", "why": why,
                             "match": mask(m.group(0))})
        # ② 该提交里"会被公开"的文件
        # ls-tree 带 blob sha ⇒ 同一内容只扫一次（历史上重复出现的文件不重复读）
        listing = sh(["git", "ls-tree", "-r", sha], cwd=repo)
        for line in listing.split("\n"):
            parts = line.split(None, 3)
            if len(parts) != 4:
                continue
            _mode, otype, osha, rel = parts
            if otype != "blob" or not published_path(rel, entries) or excluded(rel, ex_list):
                continue
            if rel.lower().endswith(BINARY_SUFFIX):
                continue
            verdict = blob_cache.get(osha)
            if verdict is None:
                blob = sh(["git", "cat-file", "blob", osha], cwd=repo, binary=True)
                if blob is None or len(blob) > MAX_BLOB or b"\x00" in blob[:8192]:
                    blob_cache[osha] = ("skip", None, None)
                    continue
                scanned_files += 1
                scanned_bytes += len(blob)
                try:
                    text = blob.decode("utf-8")
                except Exception:
                    blob_cache[osha] = ("skip", None, None)
                    continue
                text = apply_rules(text, literals, regexes)  # 关键：先变换，再扫描
                verdict = ("clean", None, None)
                for rx, why in blk:
                    m = rx.search(text)
                    if m:
                        verdict = ("block", why, mask(m.group(0)))
                        break
                else:
                    for rx, why in inf:
                        if rx.search(text):
                            verdict = ("info", why, None)
                            break
                blob_cache[osha] = verdict
            kind, why, sample = verdict
            if kind == "block":
                hits.append({"commit": sha[:10], "path": rel, "why": why, "match": sample})
            elif kind == "info":
                info_hits.append({"commit": sha[:10], "path": rel, "why": why})
        if idx % 200 == 0:
            print("  …已审 %d/%d 笔提交（文件 %d，%.1f MB）" % (idx, len(revs), scanned_files,
                                                          scanned_bytes / 1048576))

    print("\n扫描完成：提交 %d 笔 · 文件版本 %d 个 · %.1f MB 文本" % (len(revs), scanned_files,
                                                              scanned_bytes / 1048576))
    if info_hits:
        print("\n· 提示级（%d 处，替换规则会过滤，不影响公开）:" % len(info_hits))
        for h in info_hits[:10]:
            print("    %s  %-52s %s" % (h["commit"], h["path"][:52], h["why"]))
    if hits:
        from collections import Counter, OrderedDict
        bypath = OrderedDict()
        for h in hits:
            bypath.setdefault((h["path"], h["why"]), [0, h["commit"], h["match"]])
            bypath[(h["path"], h["why"])][0] += 1
        print("\n✗ 阻断级命中 %d 处，涉及 %d 个文件（按文件归并）：" % (len(hits), len(bypath)))
        for (path, why), (cnt, first_commit, sample) in sorted(bypath.items(), key=lambda kv: -kv[1][0])[:30]:
            print("    %-56s %-20s ×%-4d 首见 %s  %s" % (path[:56], why, cnt, first_commit, sample))
        print("\n  逐条明细（前 %d）:" % min(args.max_hits, len(hits)))
        for h in hits[: args.max_hits]:
            print("    %s  %-52s %-22s %s" % (h["commit"], h["path"][:52], h["why"], h["match"]))
        if len(hits) > args.max_hits:
            print("    …（其余 %d 处省略）" % (len(hits) - args.max_hits))
    else:
        print("\n结果: 通过——全历史未发现阻断级敏感内容 ✓")

    if args.json_out:
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump({"scope": scope, "commits": len(revs), "files": scanned_files,
                       "bytes": scanned_bytes, "block_hits": hits, "info_hits": info_hits},
                      fh, ensure_ascii=False, indent=2)
        print("  JSON 报告: %s" % args.json_out)
    return 1 if hits else 0


if __name__ == "__main__":
    sys.exit(main())
