#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-two-pass-identity.py v1.0.0 —— `C3`（`Q-009`）**同一判据跑两遍 · 逐字对拍**（防假绿）
   ＋ `C4`（`Q-011`）**对象口径**：只对「只读 / 已封」基准出结论；可变件只报「无可比基准 · rc=8」

真源（**逐字** · 不转述）
--------------------------
`Zerg-内部文档/项目文档/v2.5.12/承接自-v2.5.11/调研-缺口-归档台面-20260923.md:70`（`C3` 行）逐字：
  「**同一判据跑两遍、逐字对拍（防假绿）** —— 今天手搓 `diff /tmp/st-before.txt /tmp/st-after.txt
   && echo \"git status: 逐字不变\"`」；同格「可核」逐字：「两跑的**逐字**对拍（两次输出
   `shasum -a 256` **同值**）」。
同件 `:71`（`C4` 行 · 本机 `man hdiutil` 口径）逐字：「**条件 = 只对「只读 / 已封」的基准出零副作用证明**」·
  「对可变的件（今天的工作树）**不许假装证明**，只许报「无可比基准 · rc=8」」。
施工条逐字（`Zerg-内部文档/项目文档/v2.5.12/任务清单-本版全做-v2.5.12-20260924.md` §一 序120 ·
  组4 §二.3 `W-37`）：「① 同一判据跑两遍逐字对拍（两次输出 `shasum -a 256` 同值）
  ② 只对只读 / 已封的基准出证明；对可变的件不许假装证明，只许报「无可比基准 · rc=8」」。

判据（三条 · 每条能单独失败）
------------------------------
  J1 **两跑逐字对拍**：同一条判据命令跑**两遍** ⇒ 剥掉**抖动登记表**里的字段 / 行之后，两次输出
     **逐字**同值（`sha256` 同值）；不同 ⇒ **rc=1** 并点名首个差异行。★ **原始**（未剥离）两值也照实
     打印 —— 一个字节都不藏 ✗。
  J2 **对象口径（只读 / 已封才出结论）**：判据的机器读面（`--json`）逐件给出对象档位 ——
     全部「可比（自带基准）」才给绿；有**判红** ⇒ rc=1；有**缺件** ⇒ rc=2（不给结论）。
  J3 **可变件不许假装**：只要对象集里有**可变件**（工作树：dirty / 未跟踪 / 与 `HEAD` blob 不等）
     ⇒ 逐条打印逐字「**无可比基准**」+ 最终 **rc=8**（**不许给绿**）——「两跑一样」对**本来就会变**的件
     证不出零副作用（`C4` 的病根逐字：「可变的件无法校验」）。

单一口径（**不另写一份判据** ✗）
--------------------------------
本件**不自己判**「哪些件算已封 / 可比」：默认判据 = 门㉓ `scripts/gates/check-zero-side-effect.py`
（`C4` 零副作用证明件）的**机器读档**（`--json`）⇒ 对象集与档位**现调同一件**（同门㉔ / 门㉕ 口径）。
`--cmd` 可换判据（自检与取证用），但**对象口径仍取自那条判据自己的 `items[]`**。

抖动登记表（**唯一豁免面** · 逐条带理由 · 只许「本次运行的真实时刻」这一类）
------------------------------------------------------------------------------
  · `meta.at`（JSON 档）—— 每次运行的真实时刻（门㉓ 打印的「时刻: …」那一格）
  · `^\\s*时刻: <ISO8601>$`（文本档）—— 同上（门㉓ / 门㉔ 的 `时刻:` 行）
  ★ **表外任何一处差异 ⇒ 一律判红**（自检 ③ 的牙）；表为空也合法（更严，不报错）。

退码（三档 + 一档不给结论 · 优先级 **1 > 2 > 8 > 0**）
------------------------------------------------------
  0 = 两跑逐字同值（剥离抖动登记后）**且**对象集全部可比出结论
  1 = 两跑不同（差异在登记表外）/ 对象集有**判红**
  2 = **不给结论**：判据跑不出 · 判据没给机器读面（不是 JSON）· 对象集**空转** · 自检不过 · 命令行含写旗标
  8 = **对象集有可变件**（无可比基准 ⇒ 不假装）

用法
----
    python3 scripts/gates/check-two-pass-identity.py                    # 真目标（判据 = 门㉓ 的 --json 档）
    python3 scripts/gates/check-two-pass-identity.py --cmd "<判据命令>"  # 换判据（仍按它自己的 items[] 判对象）
    python3 scripts/gates/check-two-pass-identity.py --json             # 机器读面
    python3 scripts/gates/check-two-pass-identity.py --list-rules
    python3 scripts/gates/check-two-pass-identity.py --self-test        # 成对正/负夹具 · 不碰真目标
"""

import argparse
import hashlib
import json
import os
import re
import shlex
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))
SELF_NAME = os.path.basename(__file__)
VERSION = "check-two-pass-identity.py v1.0.0"

RC_OK, RC_RED, RC_NOCONC, RC_NOBASE = 0, 1, 2, 8

# 默认判据 = 门㉓（`C4` 零副作用证明件）的机器读档 —— 「单一口径」：不另写一份「哪些件算已封」
DEFAULT_CMD = "python3 scripts/gates/check-zero-side-effect.py --json"

# ── 抖动登记表（**唯一豁免面**）────────────────────────────────────────────────
VOLATILE_JSON_KEYS = [("meta.at", "每次运行的真实时刻（门㉓ 的「时刻: …」那一格）")]
VOLATILE_LINE_RES = [
    (re.compile(r"^\s*时刻: \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4}\s*$"),
     "每次运行的真实时刻行（门㉓ / 门㉔ 的 `时刻:` 行）"),
]
WRITE_FLAGS = ("--chmod", "--seal", "--fix", "--record", "commit", "add", "rm", "-w")


# ─────────────────────────────── 基础 ───────────────────────────────
def sha256_bytes(b: bytes) -> str:
    return hashlib.sha256(b).hexdigest()


def run_once(cmd_str: str, timeout=300):
    """跑一遍判据 ⇒ (rc, stdout_bytes, stderr_bytes)。**只读**：不落盘、不接管道。"""
    argv = shlex.split(cmd_str)
    try:
        p = subprocess.run(argv, cwd=REPO_ROOT, capture_output=True, timeout=timeout)
    except FileNotFoundError:
        return None, b"", ("判据命令跑不起来（找不到可执行件）：%s" % argv[0]).encode("utf-8")
    except subprocess.TimeoutExpired:
        return None, b"", ("判据命令超时（%ss）：%s" % (timeout, cmd_str)).encode("utf-8")
    return p.returncode, p.stdout, p.stderr


def _canon_json_line(line: str):
    """一行能当 JSON 解析 ⇒ (规范化后的行, 剥离的抖动项)；否则 (None, [])。"""
    try:
        obj = json.loads(line)
    except Exception:                              # noqa: BLE001
        return None, []
    if not isinstance(obj, dict):
        return None, []
    dropped = []
    for path, why in VOLATILE_JSON_KEYS:
        parts = path.split(".")
        cur = obj
        for p in parts[:-1]:
            cur = cur.get(p) if isinstance(cur, dict) else None
            if cur is None:
                break
        if isinstance(cur, dict) and parts[-1] in cur:
            dropped.append("%s=%r（%s）" % (path, cur.pop(parts[-1]), why))
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":")), dropped


def normalize(out: bytes):
    """⇒ (档名, 归一后的字节, 被剥离的抖动项清单)。

    规则（**逐行** · 与判据是 JSON 档还是文本档无关）：
      · 该行命中**抖动登记表**（文本形态）⇒ 剥离并登记
      · 该行是 JSON ⇒ 规范化（键排序）并**剥掉登记表里的字段**（如 `meta.at`）
      · 其余行**原样逐字**保留 ⇒ 「登记表外的任何差异一律判红」
    """
    kept, dropped, mode = [], [], "text"
    for ln in out.decode("utf-8", "replace").split("\n"):
        if any(rx.search(ln) for rx, _why in VOLATILE_LINE_RES):
            dropped.append("%s（文本档抖动行）" % ln.strip())
            continue
        canon, dr = _canon_json_line(ln)
        if canon is not None:
            mode = "json"
            kept.append(canon)
            dropped.extend(dr)
        else:
            kept.append(ln)
    return mode, "\n".join(kept).encode("utf-8"), dropped


def first_diff_line(a: bytes, b: bytes):
    la, lb = a.decode("utf-8", "replace").split("\n"), b.decode("utf-8", "replace").split("\n")
    for i in range(max(len(la), len(lb))):
        x = la[i] if i < len(la) else "<（无此行）>"
        y = lb[i] if i < len(lb) else "<（无此行）>"
        if x != y:
            return i + 1, x.strip()[:120], y.strip()[:120]
    return None


def objects_of(out: bytes):
    """从判据输出里取机器读面 ⇒ items 或 None（没有 JSON 行 / `items` 空 ⇒ None ⇒ 不给结论）。

    判据的 `--json` 档后面可能还跟一行人读小结（门㉓ 的 `⇒ rc=…`）⇒ **逐行找**那一条 JSON。
    """
    for ln in out.decode("utf-8", "replace").split("\n"):
        try:
            obj = json.loads(ln)
        except Exception:                          # noqa: BLE001
            continue
        if isinstance(obj, dict) and isinstance(obj.get("items"), list) and obj["items"]:
            return obj["items"]
    return None


# ─────────────────────────────── 判 ───────────────────────────────
def judge(cmd_str: str, json_out: bool, quiet=False):
    line = (lambda s: None) if quiet else print
    line("── `C3` 同一判据跑两遍 · 逐字对拍（**防假绿**）＋ `C4` 对象口径（只对只读 / 已封）──")
    line("   判据命令: %s" % cmd_str)
    line("   抖动登记表（唯一豁免面）: %d 项 —— %s"
         % (len(VOLATILE_JSON_KEYS) + len(VOLATILE_LINE_RES),
            " · ".join([k for k, _ in VOLATILE_JSON_KEYS] +
                       [rx.pattern for rx, _ in VOLATILE_LINE_RES])))

    rc1, o1, e1 = run_once(cmd_str)
    rc2, o2, e2 = run_once(cmd_str)
    if rc1 is None or rc2 is None:
        line("⛔ 不给结论（rc=2）：判据跑不起来 —— %s" % (e1 or e2).decode("utf-8", "replace")[:200])
        return RC_NOCONC, {"reason": "cmd_unrunnable"}

    mode1, n1, d1 = normalize(o1)
    mode2, n2, d2 = normalize(o2)
    s_raw1, s_raw2 = sha256_bytes(o1), sha256_bytes(o2)
    s_n1, s_n2 = sha256_bytes(n1), sha256_bytes(n2)
    same = s_n1 == s_n2
    fd = None if same else first_diff_line(n1, n2)

    line("   J1 两跑逐字对拍（`shasum -a 256`）:")
    line("      ① **原始**输出（**一个字节都不藏** · 照实打印两值）:")
    line("         跑1 rc=%s · sha256 %s（%d 字节）" % (rc1, s_raw1, len(o1)))
    line("         跑2 rc=%s · sha256 %s（%d 字节）" % (rc2, s_raw2, len(o2)))
    if s_raw1 == s_raw2:
        line("         ⇒ 原始两值 **同值** ✓（没动用登记表）")
    else:
        line("         ⇒ 原始两值**不同**（差在已登记抖动项上：%s）" % ("；".join(d2) or "（无）"))
    line("      ② **剥离抖动登记项后**（%s 档 · 剥离 %d 项）:" % (mode1, len(d2)))
    line("         跑1 sha256 %s" % s_n1)
    line("         跑2 sha256 %s" % s_n2)
    line("         ⇒ %s" % ("**逐字同值** ✓（登记表外的差异 = 0）" if same else "**不同** ✗"))
    if fd:
        n_, x, y = fd
        line("         首个差异行（归一后 · 第 %d 行）: 跑1=%r / 跑2=%r" % (n_, x, y))
    line("   （对照：`shasum -a 256` 的算法与这里一致 —— 自检 ⑤ 现跑真 `shasum` 对拍同值）")

    items = objects_of(o1)
    line("   J2 / J3 对象口径（现调同一件判据的 `items[]`）:")
    if items is None:
        line("         ⛔ 判据**没给机器读面**（不是 JSON 或 `items[]` 空）⇒ 对象集**不可判** ⇒ 不给结论")
        line("⇒ rc=2：不给结论（判据没给机器读面 ⇒ 不许当绿）")
        return RC_NOCONC, {"reason": "no_machine_face"}

    by = {}
    for it in items:
        by.setdefault(int(it.get("rc", -1)), []).append(it)
    base_from = sorted({str(it.get("baseline_from")) for it in by.get(RC_OK, [])})
    line("         对象 %d 件：可比出结论 %d · 判红 %d · 缺件 %d · **可变件（无可比基准）%d**"
         % (len(items), len(by.get(RC_OK, [])), len(by.get(RC_RED, [])),
            len(by.get(RC_NOCONC, [])), len(by.get(RC_NOBASE, []))))
    kinds = {}
    for b in base_from:
        kinds[b.split(":", 1)[0]] = kinds.get(b.split(":", 1)[0], 0) + 1
    line("         出结论那批的基准来源（自带可比基准）: %s%s"
         % (" · ".join("%s×%d" % (k, v) for k, v in sorted(kinds.items())) or "（无）",
            ("" if len(base_from) <= 3 else "（例：%s）" % "；".join(base_from[:3]))))
    for it in by.get(RC_NOBASE, []):
        line("         ⊘ 无可比基准（rc=8）%s" % it.get("path"))
        line("             类别=%s · %s" % (it.get("class"), it.get("note")))
    for it in by.get(RC_RED, []):
        line("         ✗ 判红（rc=1）%s · %s" % (it.get("path"), it.get("note")))
    for it in by.get(RC_NOCONC, []):
        line("         ⛔ 缺件（不给结论）%s · %s" % (it.get("path"), it.get("note")))

    rep = {"mode": mode1, "sha_raw": [s_raw1, s_raw2], "sha_norm": [s_n1, s_n2],
           "same": same, "objects": len(items), "no_baseline": len(by.get(RC_NOBASE, [])),
           "red": len(by.get(RC_RED, [])), "missing": len(by.get(RC_NOCONC, [])),
           "baseline_from": base_from}

    if by.get(RC_RED):
        line("⇒ rc=1：两跑对拍%s · **对象集有判红**（逐条见上）" % ("" if same else "**不同**（差异在登记表外）"))
        return RC_RED, rep
    if not same:
        line("⇒ rc=1：**两跑不同**（差异落在抖动登记表**之外** ⇒ 不许拿登记表洗白）")
        return RC_RED, rep
    if by.get(RC_NOCONC):
        line("⇒ rc=2：不给结论（有 %d 条件不在盘）" % len(by.get(RC_NOCONC, [])))
        return RC_NOCONC, rep
    if by.get(RC_NOBASE):
        line("⇒ **rc=8**：对象集有 %d 条**可变件（工作树）无可比基准** ⇒ **不许假装证明**（逐条已列）"
             % len(by.get(RC_NOBASE, [])))
        return RC_NOBASE, rep
    line("⇒ rc=0：两跑**逐字同值**（登记表外 0 差异）· 对象集 %d 件**全部**自带可比基准（只读 / 已封）"
         % len(items))
    return RC_OK, rep


# ─────────────────────────────── 自检 ───────────────────────────────
def _mk_dirty_repo(d):
    """合成仓夹具：`clean.txt` 已封 · `dirty.txt` 提交后被改（= 可变件）。**每次 git 调用先清环境**。"""
    repo = os.path.join(d, "repo")
    os.makedirs(repo, exist_ok=True)
    env = {"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t",
           "GIT_COMMITTER_EMAIL": "t@t", "HOME": d, "PATH": os.environ.get("PATH", "")}

    def g(*args):
        return subprocess.run(["git", "-C", repo] + list(args), capture_output=True, text=True,
                              env=env, timeout=60)
    g("init", "-q", "-b", "main")
    for name in ("clean.txt", "dirty.txt"):
        with open(os.path.join(repo, name), "w", encoding="utf-8") as f:
            f.write("已封件\n")
    g("add", "--", "clean.txt", "dirty.txt")
    g("commit", "-qm", "init")
    with open(os.path.join(repo, "dirty.txt"), "a", encoding="utf-8") as f:
        f.write("改过了\n")
    return repo


def self_test():
    import tempfile
    ok = True

    def case(label, expect, got, note=""):
        nonlocal ok
        good = expect == got
        ok = ok and good
        print("   %s %s（实得 %r · 期望 %r）%s" % ("✓" if good else "✗", label, got, expect, note))

    with tempfile.TemporaryDirectory(prefix="zerg-twopass-") as d:
        py = sys.executable or "python3"
        # 合成判据：一律写进本次自建的临时目录（**不落仓、不写 `/tmp`**），都给机器读面（`items[]`）
        def probe(name, body):
            p = os.path.join(d, name)
            with open(p, "w", encoding="utf-8") as f:
                f.write(body)
            return "%s %s" % (py, shlex.quote(p))

        ok_items = ("{'items':[{'path':'a.md','rc':0,'baseline_from':'head-blob:a.md'}],"
                    "'meta':%s}")
        stable = probe("stable.py", "import json;print(json.dumps(%s,sort_keys=True))"
                       % (ok_items % "{}"))
        clocked = probe("clocked.py",
                        "import json,time;print(json.dumps(%s,sort_keys=True))"
                        % (ok_items % "{'at':time.strftime('%Y-%m-%dT%H:%M:%S')"
                                      " + '.%09d' % (time.time_ns() % 1000000000)"
                                      " + time.strftime('%z')}"))
        unreg = probe("unreg.py", "import json,os;print(json.dumps(%s,sort_keys=True))"
                      % (ok_items % "{'pid':os.getpid()}"))
        prose = probe("prose.py", "print('这不是 JSON')")

        # ① 稳定判据 ⇒ rc=0
        rc, _ = judge(stable, False, quiet=True)
        case("① 正控：稳定判据（带 items[]）两跑逐字同值 ⇒ rc=0", RC_OK, rc)
        # ② 已登记抖动（meta.at）⇒ 剥离后同值 ⇒ rc=0
        rc, r2 = judge(clocked, False, quiet=True)
        case("② 正控：只差「已登记抖动字段」`meta.at` ⇒ 剥离后同值 ⇒ rc=0", RC_OK, rc,
             note="  原始两值不同 = %s" % (r2["sha_raw"][0] != r2["sha_raw"][1]))
        case("②b 且**照实打印**原始两值（不藏）", True, r2["sha_raw"][0] != r2["sha_raw"][1])
        # ③ 未登记抖动 ⇒ rc=1（登记表外一律判红）
        rc, r3 = judge(unreg, False, quiet=True)
        case("③ 负控：差异**在登记表外**（pid）⇒ rc=1", RC_RED, rc)
        case("③b 且两值照实不同", True, r3["sha_norm"][0] != r3["sha_norm"][1])
        # ④ 真话负控：把 ③ 那条改回固定 ⇒ 回绿
        rc, _ = judge(stable, False, quiet=True)
        case("④ 真话负控：同一判据去掉未登记抖动 ⇒ 回绿", RC_OK, rc)

        # ⑤ 本件的 sha256 与真 `shasum -a 256` 同算法（现跑对拍）
        blob = "对拍夹具\n".encode("utf-8")
        with open(os.path.join(d, "x.txt"), "wb") as f:
            f.write(blob)
        p = subprocess.run(["shasum", "-a", "256", os.path.join(d, "x.txt")],
                           capture_output=True, text=True, timeout=60)
        case("⑤ hashlib sha256 == 现跑 `shasum -a 256`", True,
             p.stdout.split()[0] == sha256_bytes(blob))

        # ⑥ 判据不给机器读面 ⇒ rc=2（空转不许当绿）
        rc, _ = judge(prose, False, quiet=True)
        case("⑥ 负控：判据没给机器读面（不是 JSON）⇒ rc=2", RC_NOCONC, rc)
        # ⑦ 判据跑不起来 ⇒ rc=2
        rc, _ = judge("definitely-not-a-command-zz --json", False, quiet=True)
        case("⑦ 负控：判据跑不起来 ⇒ rc=2", RC_NOCONC, rc)
        # ⑧ 写旗标 ⇒ rc=2
        case("⑧ 负控：写旗标 %r ⇒ 拒（rc=2）" % "--record", RC_NOCONC, main(["--record"]))
        # ⑨ 可变件档：合成仓的 dirty 件 ⇒ 门㉓ 给 rc=8 ⇒ 本件**不许给绿**
        repo = _mk_dirty_repo(d)
        dirty = os.path.join(repo, "dirty.txt")
        cmd = ('%s scripts/gates/check-zero-side-effect.py --json --root %s --sealed %s'
               % (py, shlex.quote(repo), shlex.quote(dirty)))
        rc, r9 = judge(cmd, False, quiet=True)
        case("⑨ 负控：对象集含**可变件** ⇒ 本件 rc=8（不许给绿）", RC_NOBASE, rc)
        case("⑨b 且它确实报了 1 条无可比基准", 1, r9.get("no_baseline"))
        # ⑩ 同一夹具里那件**已封**的件 ⇒ rc=0（成对 · 证明 ⑨ 不是恒 8）
        clean = os.path.join(repo, "clean.txt")
        cmd10 = ('%s scripts/gates/check-zero-side-effect.py --json --root %s --sealed %s'
                 % (py, shlex.quote(repo), shlex.quote(clean)))
        rc, r10 = judge(cmd10, False, quiet=True)
        case("⑩ 正控（与 ⑨ 成对）：同一夹具的已封件 ⇒ rc=0", RC_OK, rc)
        case("⑩b 且基准来源是 HEAD blob（自带基准）", True,
             any("head-blob" in b for b in r10.get("baseline_from", [])))
        # ⑪ 本件真目标段不含写调用形态（结构性自证 · 走 /tmp 写盘那两条形态）
        src = open(os.path.abspath(__file__), encoding="utf-8").read()
        body = src.split("SELFTEST_MARKER")[0]
        case("⑪ 正例：本件真目标段 0 处 shell-重定向写盘形态", 0,
             len(re.findall(r"(?<![A-Za-z<=])>{1,2}\s*/tmp/", body)))

    print("自检结论: " + ("全过" if ok else "**有断言失败**"))
    return 0 if ok else 1


def list_rules():
    print("── %s 判据 ──" % VERSION)
    print("J1 同一判据跑两遍 ⇒ 剥离**抖动登记表**后**逐字**同值（`sha256` 同值）；不同 ⇒ rc=1")
    print("   · 原始（未剥离）两值**照实打印**（不藏）· 登记表外任何差异 ⇒ 一律判红")
    print("J2 对象口径：只对**只读 / 已封**（自带可比基准）的件出结论；判红 ⇒ rc=1 · 缺件 ⇒ rc=2")
    print("J3 **可变件不许假装**：对象集里有可变件 ⇒ 逐条「**无可比基准**」+ rc=8（**不许给绿**）")
    print("抖 登记表（唯一豁免面）: %s" % " · ".join(
        [k for k, _ in VOLATILE_JSON_KEYS] + [rx.pattern for rx, _ in VOLATILE_LINE_RES]))
    print("退码: 0 两跑同值且全可比 / 1 两跑不同或判红 / 2 不给结论 / 8 对象集有可变件（不假装）")
    print("单一口径: 默认判据 = 门㉓ `check-zero-side-effect.py --json`（对象集与档位现调同一件）")
    return RC_OK


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    hits = [w for w in WRITE_FLAGS if w in argv]
    if hits:
        print("⇒ 拒（rc=2 · 不给结论）：命令行里出现写旗标 %s" % hits)
        print("   理由：本件**只出判据**，一个字节都不改（只读两跑 + 只读对象口径）。")
        return RC_NOCONC

    ap = argparse.ArgumentParser(description="C3 同一判据跑两遍逐字对拍 + C4 对象口径（可变件 rc=8）")
    ap.add_argument("--cmd", default=DEFAULT_CMD, help="判据命令（默认 = 门㉓ 的 --json 档）")
    ap.add_argument("--json", action="store_true", help="本件的机器读面")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--version", action="version", version=VERSION)
    a = ap.parse_args(argv)

    if a.self_test:
        return self_test()
    if a.list_rules:
        return list_rules()

    if a.json:
        import io
        import contextlib
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            rc, rep = judge(a.cmd, True)
        rep = dict(rep, cmd=a.cmd, tool=VERSION)
        print(json.dumps({"schema": "zerg/v1", "kind": "TwoPassIdentity",
                          "items": [rep], "meta": {"count": 1, "rc": rc},
                          "warnings": [], "truncated": False}, ensure_ascii=False))
        return rc
    rc, _rep = judge(a.cmd, False)
    return rc


SELFTEST_MARKER = "# ─────────────────────────── " + "自检"

if __name__ == "__main__":
    sys.exit(main())
