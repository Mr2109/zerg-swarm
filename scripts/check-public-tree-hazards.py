#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""发布期门禁：「危险路径」与「令牌形态」。任一命中 ⇒ 不得发布。

退出码：0 = 干净；1 = 有命中（不得发布）；2 = 脚本自测未过（拒绝出结论）。

原由（2026-09-13 审计报告 §8.2）：公开仓历史里曾出现 6 个 **CA 任务日志**
`events.jsonl`（合计约 854KB，含令牌形态与 bearer 字样），而当时的发布清单只按
「识别词」（私有称谓/卷路径/家机名）扫描，**没有任何「危险路径」闸** —— 换个文件名
就不会被任何一道检查看见。同时，项目令牌前缀形态曾随文档/脚本进入公开历史。

判据来源（均为实测，不是拍脑袋）：
  · 路径判据取「存在性」——末树 706 个文件上 12 个模式**全部 0 命中**（零假阳）。
  · 形态判据只保留**项目特有前缀**与**收窄的 bearer 形态**：
      - `x3gw-` 前缀 + 足够长的值：末树 0 命中、历史 127 笔命中（能红）。
      - `Bearer ` + 长值（≥20）：末树 0 命中。
    刻意**不**纳入通用 `api_key=<长串>`：`core/internal/memory/memory_test.go` 里
    存在**合成的假凭据**（用于测试敏感信息扫描），纳入即假阳。
    也**不**纳入「超长 base64 样串」：末树 881 处命中（哈希/内嵌资源），纯噪声。

用法: python3 scripts/check-public-tree-hazards.py <git目录> [<commit>|--all]

模式语义（重要）：
  · **发布闸用 HEAD**（或指定 commit）—— 判的是「即将发布的这棵树」里有没有危险物；
    危险物一旦进了末树，闸即红，发布被拦在推送之前。
  · `--all` 扫全史对象，**供人工审计**用。本仓历史上确实存在过的日志与旧令牌残留
    （见 Zerg-内部文档/issues/审计报告-公开面私有串残留-20260913.md §8.2 —— 该报告 2026-09-19 已随开发文档
    分家整目录迁出工作树，原路径 docs/issues/；Mr2109 已决定
    不重写历史）会让 `--all` **预期为红** ⇒ 它不得直接当发布闸，否则永久阻塞发布。

注意：本文件自身不得出现真实令牌值或完整可命中的形态（自测串一律拼接构造）。
"""
import re
import subprocess
import sys

# ── 判据 A：危险路径（存在性；按仓库根相对路径匹配）────────────────
DANGER_PATH = [
    (r"(^|/)\.zerg/",              "运行期任务日志/状态目录"),
    (r"\.jsonl$",                  "事件/日志流文件"),
    (r"\.db$",                     "SQLite 数据库"),
    (r"\.db-(wal|shm)$",           "SQLite WAL/SHM"),
    (r"\.sqlite$",                 "SQLite 数据库"),
    (r"(^|/)logs/",                "日志目录"),
    (r"\.log$",                    "日志文件"),
    (r"(^|/)\.env$",               "真配置文件（示例应叫 .env.example）"),
    (r"\.env\.local$",             "本地覆盖配置"),
    (r"(^|/)target/",              "构建产物目录"),
    (r"(^|/)node_modules/",        "依赖目录"),
    (r"\.DS_Store$",               "macOS 目录元数据"),
]

# ── 判据 B：令牌形态（内容）──────────────────────────────────────
TOKEN_FORM = [
    (r"x3gw-[A-Za-z0-9_.\-]{6,}",        "项目网关令牌形态"),
    (r"Bearer [A-Za-z0-9_.\-]{20,}",     "bearer + 长值（真凭据）"),
]

PATH_RES = [(re.compile(rx), why) for rx, why in DANGER_PATH]
FORM_RES = [(re.compile(rx), why) for rx, why in TOKEN_FORM]

# 拼接构造自测串，避免本文件出现完整可命中的形态
T = "x3gw-"
B = "Bearer "


def selftest():
    cases_path = [
        ("core/.zerg/logs/2026-01-01T00-00-00/events.jsonl", True, "运行期日志（本次事故的形态）"),
        ("data/knowledge.db", True, "SQLite 库"),
        ("data/knowledge.db-wal", True, "WAL"),
        ("var/logs/app.log", True, "日志目录/文件"),
        ("docs/logs.md", False, "文件名含 logs，但不是日志目录"),
        ("docs/guide.md", False, "正常文档"),
        (".env", True, "真配置"),
        (".env.example", False, "示例配置（正当）"),
        ("src/main.go", False, "正常源码"),
        ("ui/target/debug/x", True, "构建产物"),
    ]
    cases_form = [
        ("Authorization: " + B + "a1b2c3d4e5f6g7h8i9j0k1l2", True, "真 bearer 值"),
        ("Authorization: " + B + "<token>", False, "占位写法（正当）"),
        ("令牌以 " + T + " 开头", False, "说明文字（无实际值）"),
        ("X-Auth-Token: " + T + "shared0123456789", True, "实际令牌形态"),
        ("curl -H 'X-Auth-Token: $TOKEN' http://x", False, "变量引用"),
    ]
    bad = 0
    for text, want, why in cases_path:
        got = any(rx.search(text) for rx, _ in PATH_RES)
        ok = (got == want)
        bad += (not ok)
        print("   %s want=%-5s got=%-5s | %s" % ("✓" if ok else "✗", want, got, why))
    for text, want, why in cases_form:
        got = any(rx.search(text) for rx, _ in FORM_RES)
        ok = (got == want)
        bad += (not ok)
        print("   %s want=%-5s got=%-5s | %s" % ("✓" if ok else "✗", want, got, why))
    print("   自测: %s" % ("全过" if bad == 0 else "%d 条失败" % bad))
    return bad == 0


def all_paths(gitdir, commit):
    """返回该提交（或全史对象）里的仓库根相对路径集合。"""
    out = subprocess.run(["git", "-C", gitdir, "ls-tree", "-r", commit, "--name-only"],
                         capture_output=True, text=True, encoding="utf-8", errors="replace").stdout
    return [p for p in out.split("\n") if p.strip()]


def scan_paths(gitdir, commits_all):
    hits = []
    if commits_all:
        out = subprocess.run(["git", "-C", gitdir, "rev-list", "--objects", "--all"],
                             capture_output=True, text=True, encoding="utf-8", errors="replace").stdout
        paths = sorted({ln.split(" ", 1)[1] for ln in out.split("\n") if " " in ln})
        for rx, why in PATH_RES:
            hits.extend((p, why) for p in paths if rx.search(p))
    else:
        for p in all_paths(gitdir, "HEAD"):
            for rx, why in PATH_RES:
                if rx.search(p):
                    hits.append((p, why))
    return hits


def scan_forms(gitdir, commit, commits_all):
    hits = []
    if commits_all:
        commits = subprocess.run(["git", "-C", gitdir, "rev-list", "main"],
                                 capture_output=True, text=True).stdout.split()
        seen = set()
        for c in commits:
            o = subprocess.run(["git", "-C", gitdir, "grep", "-I", "-E", "-l",
                                "-e", "|".join(rx.pattern for rx, _ in FORM_RES), c, "--", "."],
                               capture_output=True, text=True, encoding="utf-8", errors="replace").stdout
            for ln in o.split("\n"):
                if not ln.strip():
                    continue
                key = ln.split(":", 1)[-1]
                if key not in seen:
                    seen.add(key)
                    hits.append((key, "令牌形态（提交 %s）" % c[:8]))
    else:
        for rx, why in FORM_RES:
            o = subprocess.run(["git", "-C", gitdir, "grep", "-I", "-E", "-l", "-e", rx.pattern,
                                commit, "--", "."],
                               capture_output=True, text=True, encoding="utf-8", errors="replace").stdout
            for ln in o.split("\n"):
                if ln.strip():
                    hits.append((ln.split(":", 1)[-1], why))
    return hits


if __name__ == "__main__":
    if not selftest():
        print("自测未过 ⇒ 脚本本身有问题，拒绝出结论", file=sys.stderr)
        sys.exit(2)
    target = sys.argv[1] if len(sys.argv) > 1 else "."
    mode = sys.argv[2] if len(sys.argv) > 2 else "HEAD"
    commits_all = (mode == "--all")

    ph = scan_paths(target, commits_all)
    fh = scan_forms(target, mode if not commits_all else "main", commits_all)
    total = len(ph) + len(fh)

    if total == 0:
        print("结果: 通过——未发现危险路径与令牌形态（扫 %s mode=%s）" % (target, mode))
        sys.exit(0)

    print("结果: 不通过——危险路径 %d 项、令牌形态 %d 项（%s mode=%s）：" % (len(ph), len(fh), target, mode))
    for p, why in ph[:15]:
        print("   路径 %s   ← %s" % (p, why))
    for p, why in fh[:15]:
        print("   形态 %s   ← %s" % (p, why))
    sys.exit(1)
