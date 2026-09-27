#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Go 日志/横幅中文字面量审计（多语言 L4——决策③「日志=英文」，分批推进的进度尺）。

用法：
  python3 scripts/gates/audit_go_logs.py           # 汇总（总数 + 优先批 + 文件 Top）
  python3 scripts/gates/audit_go_logs.py --list    # 逐条列出（文件:行 + 内容）
  python3 scripts/gates/audit_go_logs.py --batch 启动   # 只看某优先批
  python3 scripts/gates/audit_go_logs.py --self-test    # 成对负控自检（仓外夹具 · 不碰真目标）

优先批（设计稿 §4.4 顺序）：启动 / 认证 / 模型加载 / 任务生命周期 / 熔断 —— 优先英文化；
纯内部诊断可晚改；**注释保持中文**（不在此脚本口径内）。

退码（三档 · `--self-test` 那一档是本门**唯一能变红**的地方）
------------------------------------------------------------
  0 = 真跑（**恒 0**：本门是「分批推进的进度尺」，只报计数、不判红）· 也是 `--self-test` 全过
  1 = `--self-test` 有格不过（判据被弄哑 / 夹具改坏 ⇒ 自检当场红）
  2 = 判不了（自检夹具搭不出来 / 自检用例条数对不上 ⇒ 不许当绿）
★ 为何给一只「恒 0」的门补自检面（缺口 `Q-176` 第六批已入账的那一格）：`all.sh` 的逐门自检档里，
  文件里没有 `--self-test` 字面的门只能记 `NA` ⇒ **它永远不会让总 rc 变红** ✗。本门真跑既不红、
  也不该红（进度尺），所以把「能变红」那根杆立在**自检**上 —— 自检自己会退 1。
  判据与真跑退码**一字未改**（只加 `--self-test` 这一档）。
"""
import argparse
import collections
import os
import re
import shutil
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
CORE = os.path.join(ROOT, "core")
CJK = re.compile(r'[\u4e00-\u9fff]')
LOG = re.compile(r'(log\.Printf|log\.Println|log\.Print|fmt\.Printf|fmt\.Println|fmt\.Print|'
                 r'fmt\.Fprintf|fmt\.Fprintln|println\(|eprintln\(|slog\.|Fatalf\()')

# 优先批关键词 → 批名
BATCHES = [
    ("启动", ["启动", "监听", "横幅", "初始化", "就绪"]),
    ("认证", ["令牌", "认证", "auth", "token", "AUTH"]),
    ("模型加载", ["模型", "model", "llama", "适配器", "adapter"]),
    ("任务生命周期", ["任务", "task", "派单", "worktree", "调度"]),
    ("熔断", ["熔断", "冷却", "失败"]),
]


def batch_of(line):
    for name, kws in BATCHES:
        for kw in kws:
            if kw in line:
                return name
    return "其他/内部诊断"


def scan():
    rows = []
    for root, dirs, names in os.walk(CORE):
        dirs[:] = [d for d in dirs if d not in ("vendor", "testdata")]
        for n in sorted(names):
            if not n.endswith(".go") or n.endswith("_test.go"):
                continue
            path = os.path.join(root, n)
            rel = os.path.relpath(path, ROOT)
            for i, line in enumerate(open(path, encoding="utf-8").read().split("\n"), 1):
                s = line.strip()
                if s.startswith("//") or not CJK.search(s) or not LOG.search(s):
                    continue
                rows.append((rel, i, batch_of(s), s[:150]))
    return rows


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
                     % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                                          else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser()
    ap.add_argument("--list", action="store_true", help="逐条列出")
    ap.add_argument("--batch", help="只看某优先批")
    # ★ `Q-176` 第六批：拼法统一 —— **准拼法 `--self-test`**，旧拼法 `--selftest` 作别名
    ap.add_argument("--self-test", "--selftest", dest="selftest", action="store_true",
                    help="只跑成对负控自检（合成夹具 · 仓外临时目录 · 不碰真目标）")
    args = ap.parse_args()

    if args.selftest:
        import atexit
        atexit.register(cleanup)
        print("── audit_go_logs.py 自证（成对正负控 + 防空绿档 · 仓外夹具 · 真退出码）")
        return self_test()

    rows = scan()
    by_batch = collections.Counter(r[2] for r in rows)
    by_file = collections.Counter(r[0] for r in rows)

    print("=== Go 含中文的日志/横幅语句：%d 条 ===" % len(rows))
    print("\n按优先批（设计稿 §4.4 顺序）：")
    for name, _ in BATCHES + [("其他/内部诊断", [])]:
        print("  %-14s %d" % (name, by_batch.get(name, 0)))
    print("\n文件 Top 15：")
    for f, n in by_file.most_common(15):
        print("  %-52s %d" % (f, n))

    if args.list or args.batch:
        sel = [r for r in rows if not args.batch or r[2] == args.batch]
        print("\n=== 明细（%d 条）===" % len(sel))
        for rel, i, b, s in sel:
            print("  [%s] %s:%d %s" % (b, rel, i, s))


# ══════════════════════════════════════════════════════════════════
# 自检面（`--self-test`）：成对正负控 + 防空绿档 · 仓外夹具（$TMPDIR）· 不碰真目标
# ══════════════════════════════════════════════════════════════════
# 设计真源（式样照同目录 `check-password-argv.py` / `check-marks-coverage-parity.py` /
# `check-no-symlink-farm.py` 的 `--self-test`）：docstring 写清真源 / 判据 / 退码三档 ·
# `WANT_CASES` 写死（少跑一格即断言失败）· 夹具全落**仓外**临时目录 · 成对正负控（把夹具改坏
# ⇒ 判红；复原 ⇒ 回绿）· 防空绿（把判据真源弄哑 ⇒ 读数必须跟着变：弄哑而不变 ⇒ 判据本来就是
# 死的 ⇒ 自检当场红）。
#   ① 判据真源**只有两处**：`CJK`（这一行有没有中文）与 `LOG`（这是不是日志/横幅调用）；
#      批名真源 = `BATCHES`。三处之外没有第二份实现 ⇒ 自检只换这两个正则 / 两个输入口。
#   ② 判据：夹具命中集合**逐条**（相对仓根 · 行号 · 批名）相符 = 正控绿；把一行改成触发形 = 负控红。
#   ③ 退码：见文件头「退码」那一段（本门真跑恒 0 ⇒ 「能变红」那根杆立在自检上）。
SELF_PATH = os.path.abspath(__file__)
_TMPDIRS = []
WANT_CASES = 9                     # 自证用例条数（**写死**：少跑一格即断言失败）

# 夹具甲（**全树** · 逐条对拍用）：相对仓根路径 → 正文。规格表写死在这里 = 期望值那一份
FIX_FILES = {
    "core/app.go": (
        "package app\n"
        "// 启动：中文注释不算（判据只看日志调用的实参）\n"
        'func boot() { log.Printf("启动：监听端口失败") }\n'
        'func ascii() { log.Printf("plain ascii line") }\n'
        'var msg = "认证：这不是日志调用"\n'
        'func load() { fmt.Fprintln(os.Stderr, "模型加载：适配器缺失") }\n'
        'func fuse() { slog.Error("熔断：冷却中") }\n'
        'func misc() { log.Println("目录已建") }\n'
    ),
    "core/app_test.go": 'func t() { log.Printf("任务：测试件不算") }\n',
    "core/vendor/dep/dep.go": 'func d() { log.Println("任务：vendor 不算") }\n',
    "core/testdata/x.go": 'func x() { log.Println("任务：testdata 不算") }\n',
}
# 夹具乙（**刚好全干净**）：注释行 / 非日志调用 / 纯 ASCII / 测试件 / vendor / testdata ⇒ 一条都不该命中
CLEAN_FILES = {
    "core/clean.go": (
        "package clean\n"
        "// 启动：注释里的中文不算\n"
        'var s = "认证：非日志调用的中文字面量"\n'
        'func p() { log.Printf("plain ascii") }\n'
    ),
    "core/clean_test.go": 'func t() { log.Println("任务：测试件不算") }\n',
    "core/vendor/v.go": 'func v() { log.Println("任务：vendor 不算") }\n',
    "core/testdata/d.go": 'func d() { log.Println("任务：testdata 不算") }\n',
}
# 夹具甲应有的**全部**命中（相对仓根 · 行号 · 优先批）—— 逐条写死，多一条少一条都断言不过
WANT_HITS = (
    ("core/app.go", 3, "启动"),
    ("core/app.go", 6, "模型加载"),
    ("core/app.go", 7, "熔断"),
    ("core/app.go", 8, "其他/内部诊断"),
)
BREAK_FILE = "core/added.go"                       # ② 负控：往夹具乙里加一件触发形
BREAK_LINE = 'func bad() { log.Println("任务：派单失败") }\n'
BREAK_HIT = ("core/added.go", 1, "任务生命周期")
EXTRA_LINE = 'func extra() { log.Printf("启动：横幅已就绪") }\n'   # ⑤ 负控：往夹具甲里加一行触发形
EXTRA_HIT = ("core/app.go", 9, "启动")


def _write_fixture(base, name, files):
    """在**仓外**临时目录里现搭一棵最小仓树（`core/` 下摆 .go）—— 不碰真仓一个字节。"""
    root = tempfile.mkdtemp(prefix=name + "-", dir=base)
    _TMPDIRS.append(root)
    for rel, body in files.items():
        path = os.path.join(root, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(body)
    return root


def _rows_of(root):
    """在夹具上现跑判据：**只换 `ROOT`/`CORE` 两个输入口**（判据字节一个不动），跑完必复原。"""
    global ROOT, CORE
    saved = (ROOT, CORE)
    ROOT, CORE = root, os.path.join(root, "core")
    try:
        return scan()
    finally:
        ROOT, CORE = saved


def _rows_muted(root, src):
    """把判据真源（`LOG` / `CJK`）换成**永假**正则 ⇒ 同一棵树必须查不出东西。

    防空绿：判据**活着**才可能出现「弄哑 ⇒ 归零」这个差；判据本来就是死的 ⇒ 前面那几格先红。
    """
    global LOG, CJK
    saved = (LOG, CJK)
    dead = re.compile(r"(?!)")
    if src == "LOG":
        LOG = dead
    else:
        CJK = dead
    try:
        return _rows_of(root)
    finally:
        LOG, CJK = saved


def cleanup():
    for d in _TMPDIRS:
        shutil.rmtree(d, ignore_errors=True)


def self_test():
    """成对正负控自检：②③ 改坏 ⇒ 判红 / 复原 ⇒ 回绿 · ⑦⑧ 防空绿 · ⑨ 真目标计数（仓外夹具）。"""
    d = tempfile.mkdtemp(prefix="agl-selftest-")
    _TMPDIRS.append(d)
    bad = 0
    total = 0

    def case(tag, desc, ok, detail):
        nonlocal bad, total
        total += 1
        bad += 0 if ok else 1
        print("  %s %s %s\n       %s" % ("✓" if ok else "✗", tag, desc, detail))

    def hits(root):
        return [(r[0], r[1], r[2]) for r in _rows_of(root)]

    print("── audit_go_logs.py 自检（成对正负控 · 防空绿档 · 仓外夹具 · 不碰真目标）──")
    print("[正控/负控] 夹具乙（干净）⇒ 0 条；同一棵树加一件触发形 ⇒ 判红；复原 ⇒ 回绿")

    clean = _write_fixture(d, "clean", CLEAN_FILES)
    got = hits(clean)
    case("①正控", "干净夹具（注释 / 非日志调用 / 纯 ASCII / 测试件 / vendor / testdata）⇒ 0 条",
         got == [], "命中 %d 条 %s" % (len(got), got or ""))

    with open(os.path.join(clean, BREAK_FILE), "w", encoding="utf-8") as fh:
        fh.write(BREAK_LINE)
    got = hits(clean)
    case("②负控", "同一棵树加一件触发形（%s）⇒ 恰 1 条并点名 路径:行 / 批名" % BREAK_FILE,
         got == [BREAK_HIT], "命中 %d 条 %s" % (len(got), got))

    os.remove(os.path.join(clean, BREAK_FILE))
    got = hits(clean)
    case("③正控", "复原（把那件拿掉）⇒ 回 0 条（与 ② 成对：改坏 ⇒ 判红 / 复原 ⇒ 回绿）",
         got == [], "命中 %d 条 %s" % (len(got), got or ""))

    print("[逐条档] 夹具甲 ⇒ 每条命中的 路径·行号·批名 一字不差（`WANT_HITS` 写死）")
    per = _write_fixture(d, "per-line", FIX_FILES)
    got = hits(per)
    case("④负控", "夹具甲 ⇒ 恰 %d 条，逐条 = WANT_HITS" % len(WANT_HITS),
         got == list(WANT_HITS), "命中 %d 条 %s" % (len(got), got))

    app = os.path.join(per, "core", "app.go")
    with open(app, "a", encoding="utf-8") as fh:
        fh.write(EXTRA_LINE)
    got = hits(per)
    case("⑤负控", "把夹具甲改坏一行（core/app.go 末尾追加触发形）⇒ 多出 1 条 %s" % (EXTRA_HIT,),
         got == list(WANT_HITS) + [EXTRA_HIT], "命中 %d 条 %s" % (len(got), got))

    with open(app, "w", encoding="utf-8") as fh:
        fh.write(FIX_FILES["core/app.go"])
    got = hits(per)
    case("⑥正控", "复原（把追加那行裁掉）⇒ 回 %d 条（与 ⑤ 成对）" % len(WANT_HITS),
         got == list(WANT_HITS), "命中 %d 条 %s" % (len(got), got))

    print("[防空绿档] 把判据真源弄哑 ⇒ 同一棵树必须查不出东西（弄哑而不变 = 判据本来就是死的）")
    n_log = len(_rows_muted(per, "LOG"))
    case("⑦防空绿", "弄哑判据真源 `LOG`（日志调用识别）⇒ 夹具甲命中归零", n_log == 0,
         "命中 %d 条（判据活着时应 %d 条）" % (n_log, len(WANT_HITS)))
    n_cjk = len(_rows_muted(per, "CJK"))
    case("⑧防空绿", "弄哑判据真源 `CJK`（中文识别）⇒ 夹具甲命中同样归零", n_cjk == 0,
         "命中 %d 条（判据活着时应 %d 条）" % (n_cjk, len(WANT_HITS)))

    print("[真目标] 现读真仓（本件推得的仓根）⇒ 结构断言（命中 > 0）＋ 计数照实报")
    real = hits(ROOT)
    case("⑨真目标", "真仓 Go 树 ⇒ 命中 > 0（防真目标空转当绿）", len(real) > 0,
         "%s ⇒ 命中 %d 条" % (ROOT, len(real)))
    print("  ⚠ 缺口（只报 · 不当绿）：扫描根不在盘（真仓缺 `core/`）时本门**静默 0 条**、退码仍 0 ——")
    print("     「空转」与「干净」同形；本批只补自检面（判据 / 退码一字未改）⇒ 逐字登记，留给门面口径拍板。")

    print("  自证用例：%d 条 · 断言条数 %d（少跑一格即失效）" % (total, WANT_CASES))
    if total != WANT_CASES:
        print("✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2（不许当绿）"
              % (WANT_CASES, total), file=sys.stderr)
        return 2
    print("⇒ 自检 %s" % ("全过（rc=0）" if bad == 0 else "有 %d 条断言不过（rc=1）" % bad))
    return 0 if bad == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
