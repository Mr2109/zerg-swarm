#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-public-marks.py —— 「脚本公开面」标记表门禁（2026-09-20 承接项）

设计真源：`Zerg-内部文档/项目文档/v2.5.10/变更-v2.5.10.md` §「脚本公开面：标记表 + 机械门禁」
          （Mr2109 2026-09-20 拍板「好」）
表：`scripts/公开标记.tsv`（列 = 文件 · public · 理由；风格对齐 `docs/术语表.tsv`）

判据不是语言（Python 与开源无关 ✗），而是两条问句（写死在一处，本脚本据此判）：
  ① 别人复现产品**离得开它吗** —— 构建/安装/启动链 ⇒ 离开不开 ✗ ⇒ **A 类应当开源**
     （不开源 = 产品缺一块：外部人拿源码造不出能跑的东西）。
  ② 是否**暴露私有面** —— 门禁/文档审计（规则与私有文档体系绑死）· 标定（机型路径/端口/显存口径）·
     发布（Gitee 账号与仓库布局）· 换件（生产流程 + 签名身份）· 内部体检/统计 ⇒ 暴露 ⇒ **B 类不应开源**。
  为什么不能照抄业界：六家整目录公开 `scripts/` 是因为它们**从第一天就是公开项目**；我们是
  「私有主体 + 选择性公开面」⇒ 照抄必漏私有面。

本脚本判三条（三档退码，与仓内其余门脚本同口径）：
  A 表自身不合规 ⇒ **rc=2 不给结论**（缺表/空表/表头不符/字段数不符/文件列空/路径写法不是仓相对路径/
    public 非 yes|no/理由空/同一件重复登记）
  B 违规 ⇒ **rc=1 红**：
    B1 悬空登记：表里点名的件在盘上（`scripts/<域>/` 面）不存在；
    B2 漏登记：`scripts/<域>/` 面里有件**没进表**（域 = 分域第二段的 9 个域：build · calib · docs ·
       evals · gates · sandbox · svc · wall · edge-node；口径同分域地图 `清单-脚本分域-20260920.tsv`）；
    B3 私有面：**public=yes 的件**里出现私有 IP / 绝对路径 / 账户 / 令牌模式 ⇒ 逐条点名（模式 + 行号 + 片段）。
       ★ **只判 public=yes 的面**：public=no 的件里出现同样的文本**不是本门的判据**（那是全仓其它门的活）——
         这条「区分度」由自检用例 ⑨ 钉住（同一段坏文本放进 no 面必须放行），否则本门会退化成「全扫一遍」。
       ★ **不给注释豁免**：公开面是**整件字节**，注释里的私有路径会随件一起开源。

「导出公开面时按表过滤」的三条动作（**说明**；不搬物理文件，沿用既有「白名单 + 路径映射」机制）：
  1. 导出器（`scripts/build/publish-public.sh` + `publish/whitelist.txt` + `publish/mirror-public-lib.py`）
     在 scripts/ 面**只取本表 `public=yes` 的行**；public=no 与未登记的件一律不进导出。
  2. 路径映射照旧（`map_path()`）：公开树里脚本仍落 `scripts/<域>/<名>`，表里的路径**直接可用**。
  3. 本门挂在提交闸（`scripts/gates/precommit-gates.sh` 的 scope `public` · 模式 `tri`）⇒ 表与件一旦不一致
     （悬空/漏登记/public=yes 面有私有面）**在提交前就被拦下**，不再靠「记得导出前跑一遍」。

用法：
    python3 scripts/gates/check-public-marks.py                    # 判真目标（仓根 + scripts/公开标记.tsv）
    python3 scripts/gates/check-public-marks.py --root <树> --table <表>   # 判别的树（自检夹具用同一入口）
    python3 scripts/gates/check-public-marks.py --max-examples 40
    python3 scripts/gates/check-public-marks.py --selftest         # 成对负控自检（含 must-fail，合成夹具、不碰真目标）
退出码：0 全绿 · 1 有违规（B1/B2/B3）· 2 表不合规或空转（**不给结论** —— 不静默放绿）
"""
import argparse
import os
import re
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))                    # scripts/gates/ → 仓根
TABLE_REL = "scripts/公开标记.tsv"
COLS = ["文件", "public", "理由"]

# 扫描面 = 分域第二段的 9 个域（地图 `清单-脚本分域-20260920.tsv` 的同一批）；其它顶层目录
# （253 · exportnames · sandbox-probes）是分域前就有的工作/实验目录 ⇒ **不在**本门的面里
# （口径同 `check-wired-scripts.py` / `check-doc-name.py` 的「排除域」）。
DOMAINS = ("build", "calib", "docs", "evals", "gates", "sandbox", "svc", "wall", "edge-node")
SKIP_DIRS = {"__pycache__", ".git", ".history", ".mypy_cache", ".pytest_cache",
             "node_modules", "target", "dist", "venv", ".venv", "build"}
SKIP_SUFFIX = ("~", ".bak", ".orig", ".rej", ".swp", ".pyc", ".DS_Store")

# 私有面模式（**与既有门同一批口径**，逐条给「为什么」）——
#   来源：`scripts/gates/check-history-secrets.py` 的 BLOCK 表（全历史敏感审计，同一份判据）
#       + `scripts/gates/check-hardcoded-private-paths.py` 的 NEEDLES（私有卷/家目录）
#       + 私有标识（Gitee 镜像账号 · 机型名 · 私有项目名），对齐 `publish/replace-rules.tsv` 的脱敏对象。
PRIVATE_PATTERNS = (
    (r"/Volumes/(AHZ|BXC)\b", "私有卷绝对路径"),
    (r"/Users/[A-Za-z0-9_.-]+/", "用户家目录绝对路径"),
    (r"/home/[A-Za-z0-9_.-]+/", "Linux 家目录绝对路径"),
    (r"(?<![0-9.])192\.168\.\d{1,3}\.\d{1,3}", "内网地址 192.168.x"),
    (r"(?<![0-9.])10\.\d{1,3}\.\d{1,3}\.\d{1,3}", "内网地址 10.x"),
    (r"(?<![0-9.])172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}", "内网地址 172.16–31.x"),
    (r"\bA72109\b", "账户：Gitee 镜像账号"),
    (r"\bMr2109\b", "机群机型名 Mr2109"),
    (r"(?<![A-Za-z0-9_.-])edge-node(?![A-Za-z0-9_.-])", "机群机型名 edge-node"),
    (r"<project-a>|<project-b>", "私有项目名"),
    (r"ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}", "GitHub 令牌"),
    (r"sk-[A-Za-z0-9]{20,}", "API 密钥（sk-）"),
    (r"AKIA[0-9A-Z]{16}", "AWS Access Key"),
    (r"xox[baprs]-[A-Za-z0-9-]{10,}", "Slack 令牌"),
    (r"-----BEGIN [A-Z ]*PRIVATE KEY-----", "私钥块"),
    (r"(?i)\b(password|passwd|secret|api[_-]?key|access[_-]?token)\s*[:=]\s*[\"']?[A-Za-z0-9._\-]{12,}",
     "明文口令/密钥赋值"),
    (r"\bBearer\s+[A-Za-z0-9._\-]{20,}", "Bearer 令牌"),
)


class TableError(Exception):
    """表自身不合规 ⇒ rc=2（不给结论）。"""


def load_table(path):
    """读标记表 ⇒ [(行号, 文件, public, 理由)]；任何形态问题抛 TableError。"""
    if not os.path.isfile(path):
        raise TableError("标记表不存在：%s" % path)
    rows, dup = [], {}
    for i, line in enumerate(open(path, encoding="utf-8").read().split("\n"), 1):
        if i == 1:
            header = line.rstrip("\r").split("\t")
            if header != COLS:
                raise TableError("表头不符：%s（应为 %s）" % (header, "\t".join(COLS)))
            continue
        if not line.strip():
            continue
        f = line.rstrip("\r").split("\t")
        if len(f) != len(COLS):
            raise TableError("第 %d 行字段数 %d ≠ %d（列口径：%s）" % (i, len(f), len(COLS), "\t".join(COLS)))
        rel, pub, why = f[0].strip(), f[1].strip(), f[2].strip()
        if not rel:
            raise TableError("第 %d 行「文件」列为空" % i)
        if rel.startswith("/") or rel.startswith("./") or ".." in rel.split("/"):
            raise TableError("第 %d 行的路径不是仓相对路径（禁用绝对/`./`/`..`）：%s" % (i, rel))
        if pub not in ("yes", "no"):
            raise TableError("第 %d 行 public 不是 yes|no：%r" % (i, pub))
        if not why:
            raise TableError("第 %d 行「理由」为空（登记必须说清为什么 —— 写不清理由的标记就是洗白）" % i)
        if rel in dup:
            raise TableError("同一件重复登记：%s（第 %d 行与第 %d 行）" % (rel, dup[rel], i))
        dup[rel] = i
        rows.append((i, rel, pub, why))
    if not rows:
        raise TableError("表零数据行（只有表头）—— 空表不许当绿")
    return rows


def scan_face(root):
    """`scripts/<域>/` 面里的件（仓相对路径，字典序）。0 件 ⇒ 调用方判空转。"""
    out = []
    for d in DOMAINS:
        base = os.path.join(root, "scripts", d)
        if not os.path.isdir(base):
            continue
        for dp, dns, fns in os.walk(base):
            dns[:] = sorted(x for x in dns if x not in SKIP_DIRS and not x.startswith("."))
            for fn in sorted(fns):
                if fn.startswith(".") or fn.endswith(SKIP_SUFFIX):
                    continue
                out.append(os.path.relpath(os.path.join(dp, fn), root).replace(os.sep, "/"))
    return sorted(out)


def scan_private(path):
    """逐模式扫**整件字节**（不给注释豁免）⇒ [(为什么, 行号, 命中片段)]。"""
    try:
        txt = open(path, encoding="utf-8", errors="replace").read()
    except OSError as e:
        return [("读不了文件：%s" % e, 0, "")]
    hits = []
    for pat, why in PRIVATE_PATTERNS:
        for m in re.finditer(pat, txt):
            ln = txt.count("\n", 0, m.start()) + 1
            hits.append((why, ln, m.group(0)[:60]))
    return hits


def run(root=ROOT, table=None, max_examples=20, out=None):
    """判一遍 ⇒ 退出码（0 全绿 · 1 违规 · 2 表不合规/空转）。"""
    out = out or sys.stdout
    if table is None:
        table = os.path.join(root, TABLE_REL)
    try:
        rows = load_table(table)
    except TableError as e:
        out.write("✗ %s\n" % e)
        out.write("⇒ 不给结论（rc=2）：表不合规时**不判**真目标 —— 用坏表判出的绿没有意义\n")
        return 2
    face = scan_face(root)
    if not face:
        out.write("✗ scripts/<域>/ 面里 0 件（域 = %s）⇒ 空转，拒绝给结论（rc=2）\n" % " · ".join(DOMAINS))
        return 2

    registered = {rel: (i, pub, why) for i, rel, pub, why in rows}
    yes = [rel for rel in registered if registered[rel][1] == "yes"]
    dangling = [rel for rel in registered if not os.path.isfile(os.path.join(root, rel))]
    missing = [rel for rel in face if rel not in registered]
    private = []
    for rel in sorted(yes):
        for why, ln, sample in scan_private(os.path.join(root, rel)):
            private.append((rel, ln, why, sample))

    out.write("表：%s（%d 行 · public=yes %d · public=no %d）· 面：scripts/<域>/ %d 件（%s）\n"
              % (table, len(rows), len(yes), len(rows) - len(yes), len(face), " · ".join(DOMAINS)))

    def dump(title, items):
        out.write("\n── %s：%d 处\n" % (title, len(items)))
        for x in items[:max_examples]:
            out.write("   %s\n" % (x if isinstance(x, str) else " · ".join(str(v) for v in x)))
        if len(items) > max_examples:
            out.write("   …另 %d 处\n" % (len(items) - max_examples))

    dump("B1 悬空登记（表里点名、盘上无此件）", ["%s（表第 %d 行）" % (r, registered[r][0]) for r in dangling])
    dump("B2 漏登记（盘上有、表里没有）", missing)
    dump("B3 public=yes 面里的私有面", ["%s:%d  [%s]  %s" % (r, ln, why, s) for r, ln, why, s in private])

    bad = len(dangling) + len(missing) + len(private)
    out.write("\n合计：悬空 %d · 漏登记 %d · 私有面 %d\n" % (len(dangling), len(missing), len(private)))
    if bad:
        out.write("⇒ 违规 %d 处（rc=1）：修表或修件 —— ① 悬空 ⇒ 改表；② 漏登记 ⇒ 补表（附理由）；"
                  "③ 私有面 ⇒ 把该件标 public=no，或去掉私有面（**不许**放宽本门的模式表来凑绿）\n" % bad)
        return 1
    out.write("⇒ 全绿（rc=0）：%d 件已登记 · public=yes 面 %d 件未发现私有面\n" % (len(rows), len(yes)))
    return 0


# ── 自检：成对负控（含 **must-fail**：故意喂错 ⇒ 必红；以及「不判 no 面」「关掉扫描必退回绿」两个反向控制）──
def _fixture(tmp, table_text, files):
    """搭一棵最小树：scripts/公开标记.tsv + files{相对路径: 内容} ⇒ 返回 (root, table 路径)。"""
    root = tempfile.mkdtemp(dir=tmp)
    for rel, txt in files.items():
        p = os.path.join(root, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as fh:
            fh.write(txt)
    tpath = os.path.join(root, TABLE_REL)
    os.makedirs(os.path.dirname(tpath), exist_ok=True)
    with open(tpath, "w", encoding="utf-8") as fh:
        fh.write(table_text)
    return root, tpath


def _run_quiet(root, table):
    import io
    buf = io.StringIO()
    rc = run(root=root, table=table, out=buf)
    return rc, buf.getvalue()


def selftest():
    """成对负控 16 条：每条都要「该红的红、该绿的绿」，且断言用例条数（少跑即失效）。"""
    ok = 0
    total = 0

    def case(name, want_rc, want_text, mk):
        nonlocal ok, total
        total += 1
        rc, text = mk()
        good = (rc == want_rc) and all(s in text for s in want_text)
        ok += 1 if good else 0
        print("  %s %s（期望 rc=%s / 实测 rc=%s%s）"
              % ("✓" if good else "✗", name, want_rc, rc,
                 "" if good else "；缺字样 %s" % [s for s in want_text if s not in text]))

    GOOD = ("文件\tpublic\t理由\n"
            "scripts/build/good.sh\tyes\tA 类：构建/启动链（合成夹具）\n"
            "scripts/gates/gate.sh\tno\tB 类：门禁（合成夹具）\n")
    DIRTY_NO = ("#!/bin/sh\n# 私有面样例（public=no 面 —— 本门**不判**）\n"
                "# <worker-ip> ~/x ghp_ABCDEFGHIJKLMNOPQRSTUVWX\n")
    table_tmp = [tempfile.mkdtemp(prefix="pubmarks-selftest-")]

    def base(files_extra=None, table_text=None, table_path_override=None):
        files = {"scripts/build/good.sh": "#!/bin/sh\necho ok\n",
                 "scripts/gates/gate.sh": DIRTY_NO}
        if files_extra:
            files.update(files_extra)
        return _fixture(table_tmp[0], table_text if table_text is not None else GOOD, files)

    def mk(root, table=None):
        return lambda: _run_quiet(root, table if table else os.path.join(root, TABLE_REL))

    # ① 正控：yes 面干净 + no 面带私有面 ⇒ 绿（同时钉「不判 no 面」）
    r, t = base()
    case("① 正控：yes 面干净、no 面带私有面 ⇒ 绿（不判 no 面）", 0, ["全绿", "public=yes 1"],
         mk(r, t))

    # ② must-fail：public=yes 面塞内网 IP ⇒ 必红
    r2, t2 = base({"scripts/build/good.sh": "#!/bin/sh\ncurl http://<worker-ip>:8100/api\n"})
    case("② must-fail：public=yes 面塞内网 IP ⇒ 红且点名模式与行号", 1,
         ["私有面 1", "<worker-ip>", "内网地址 192.168.x", "good.sh:2"], mk(r2, t2))

    # ③ must-fail：public=yes 面塞私有绝对路径 ⇒ 必红
    r3, t3 = base({"scripts/build/good.sh": "#!/bin/sh\ncd <repo>\n"})
    case("③ must-fail：public=yes 面塞私有卷绝对路径 ⇒ 红", 1,
         ["私有卷绝对路径", "<volume-path>"], mk(r3, t3))

    # ④ must-fail：public=yes 面塞账户（Gitee 账号 / 机型名）⇒ 必红
    r4, t4 = base({"scripts/build/good.sh": "#!/bin/sh\ngit clone https://gitee.com/A72109/zerg-swarm\n"
                                           "ssh Mr2109 'true'\n"})
    case("④ must-fail：public=yes 面塞账户（A72109）与机型名（Mr2109）⇒ 红两条", 1,
         ["Gitee 镜像账号", "机群机型名 Mr2109"], mk(r4, t4))

    # ⑤ must-fail：public=yes 面塞令牌 ⇒ 必红
    r5, t5 = base({"scripts/build/good.sh": "#!/bin/sh\nexport T=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123\n"})
    case("⑤ must-fail：public=yes 面塞 GitHub 令牌 ⇒ 红", 1, ["GitHub 令牌", "ghp_"], mk(r5, t5))

    # ⑥ 悬空登记 ⇒ 红
    r6, t6 = base(table_text=GOOD + "scripts/gates/不存在.sh\tno\tB 类：合成夹具\n")
    case("⑥ 悬空登记（表里点名、盘上无此件）⇒ 红并点名", 1, ["悬空登记", "不存在.sh"], mk(r6, t6))

    # ⑦ 漏登记（盘上有、表里没有）⇒ 红
    r7, t7 = base({"scripts/build/newbie.sh": "#!/bin/sh\ntrue\n"})
    case("⑦ 漏登记（盘上有、表里没有）⇒ 红并点名", 1, ["漏登记", "newbie.sh"], mk(r7, t7))

    # ⑧ 表头不符 ⇒ 不给结论 rc=2
    r8, t8 = base(table_text="文件\tpublic\t理由\t备注\nscripts/build/good.sh\tyes\tA\n")
    case("⑧ 表头不符 ⇒ rc=2（不给结论，不判真目标）", 2, ["表头不符"], mk(r8, t8))

    # ⑨ public 值非法 ⇒ rc=2
    r9, t9 = base(table_text="文件\tpublic\t理由\nscripts/build/good.sh\tYES\tA\n")
    case("⑨ public 值非法（YES）⇒ rc=2", 2, ["public 不是 yes|no"], mk(r9, t9))

    # ⑩ 理由空 ⇒ rc=2
    r10, t10 = base(table_text="文件\tpublic\t理由\nscripts/build/good.sh\tyes\t\n")
    case("⑩ 理由空 ⇒ rc=2（写不清理由的标记 = 洗白）", 2, ["理由」为空"], mk(r10, t10))

    # ⑪ 重复登记 ⇒ rc=2
    r11, t11 = base(table_text=GOOD + "scripts/build/good.sh\tno\t重复\n")
    case("⑪ 同一件重复登记 ⇒ rc=2", 2, ["重复登记"], mk(r11, t11))

    # ⑫ 表缺件 ⇒ rc=2
    r12, _ = base()
    case("⑫ 表缺件 ⇒ rc=2", 2, ["标记表不存在"],
         mk(r12, os.path.join(r12, "scripts/没有这个表.tsv")))

    # ⑬ 空表（只有表头）⇒ rc=2
    r13, t13 = base(table_text="文件\tpublic\t理由\n")
    case("⑬ 空表（只有表头）⇒ rc=2（空表不许当绿）", 2, ["零数据行"], mk(r13, t13))

    # ⑭ 空转（面里 0 件）⇒ rc=2
    r14, t14 = _fixture(table_tmp[0], GOOD, {"scripts/calib/占位.txt": "x\n"})
    import shutil
    shutil.rmtree(os.path.join(r14, "scripts", "calib"))          # 面里一个件都不剩
    case("⑭ 面里 0 件 ⇒ rc=2（空转 = 假覆盖，拒绝给结论）", 2, ["空转"], mk(r14, t14))

    # ⑮ 绝对路径写法 ⇒ rc=2（路径口径：表里一律仓相对）
    r15, t15 = base(table_text="文件\tpublic\t理由\n/abs/good.sh\tyes\tA\n")
    case("⑮ 表里写绝对路径 ⇒ rc=2", 2, ["不是仓相对路径"], mk(r15, t15))

    # ⑯ 反向控制：关掉私有面扫描 ⇒ ② 那棵树必须**退回绿**（证明 ② 的断言有牙齿）
    global PRIVATE_PATTERNS
    saved = PRIVATE_PATTERNS
    PRIVATE_PATTERNS = ()
    rc16, text16 = _run_quiet(r2, t2)
    PRIVATE_PATTERNS = saved
    total += 1
    good = (rc16 == 0 and "全绿" in text16)
    ok += 1 if good else 0
    print("  %s ⑯ 反向控制：关掉私有面模式表 ⇒ ② 那棵树退回绿（rc=%s）—— 证明 ② 不是恒红也不是恒绿"
          % ("✓" if good else "✗", rc16))

    print("  自证用例：%d 条 · 通过 %d 条（断言用例条数 %d，少跑即失效）"
          % (total, ok, 16))
    assert total == 16, "自证用例数必须为 16，实际 %d" % total
    return 0 if ok == total else 1


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=ROOT, help="树根（默认 = 仓根）")
    ap.add_argument("--table", default=None, help="标记表路径（默认 = <root>/scripts/公开标记.tsv）")
    ap.add_argument("--max-examples", type=int, default=20)
    ap.add_argument("--selftest", action="store_true", help="只跑成对负控自检（合成夹具，不碰真目标）")
    args = ap.parse_args()
    if args.selftest:
        print("── check-public-marks.py 自证（含 must-fail 负控 · 合成夹具）")
        return selftest()
    return run(root=args.root, table=args.table, max_examples=args.max_examples)


if __name__ == "__main__":
    sys.exit(main())
