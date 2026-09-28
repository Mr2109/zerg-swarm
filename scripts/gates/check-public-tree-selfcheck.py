#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-public-tree-selfcheck.py —— ⑭「公开树自洽检查」（③+⑦ 合并 · 只报告档起步）

设计真源：`Zerg-内部文档/项目文档/v2.5.11/设计-CI适配-v1.1.md`
  §三 交叉验证点 2（公开面自洽交叉）与 5（公开面快照当本机一份 checkout 跑一遍）**合并成这一条**；
  §五 第一批 ③（立闸 · **先只报告**）。

为什么存在（本仓实测，不是设想）
--------------------------------
公开树是 `whitelist ∪ replace-rules` 的**产物**，它跟私有全树**不是同一份东西** —— 而本机
（macOS / Mr2109）**永远看不见**只在公开树上成立的红。2026-09-21 run `35575915598` 那一批红里，
有两类**本机全绿、公开树一 clone 就红**：

  ① **替换打穿夹具**：`publish/replace-rules.tsv` 的 `~` → `~` 规则把公开树里的
     测试夹具 `home = "~/projects/zerg/vault.key"` 改成了 `home = "~/projects/zerg/vault.key"`，
     而 `core/internal/obs/redact/redact.go` 的脱敏**必要条件门**只认两个字面量（`/Users/`、`/home/`）
     ⇒ `~/…` 形态整段跳过脱敏 ⇒ 公开树 `TestFailClosed_OverBudget` 报
     `LEAK: 秘密 "~/projects/zerg/vault.key" 以投影 "~/projects/zerg/vault.key" 残留在 …`。
  ② **夹具长度漂**：同一条规则把 `Mr2109` 换成 `Mr2109`（**多 2 字节**）⇒ 夹具
     `secret := "peer Mr2109 run here ok"`（21 字节）变成 `"peer Mr2109 run here ok"`（**23 字节**），
     base64 变 32 字符且带 `=` 填充，**打穿测试自带的长度守卫**（`len(blob) < 24 || strings.ContainsAny(blob, "=")`）
     ⇒ 公开树 `TestGateReverseControlOnEscapeArtifact` 报 `夹具失效：blob="cGVlciBNcjIxMDkgcnVuIGhlcmUgb2s="（len=32）`。

两类都**不是环境问题**（本机 clone 即复现）⇒ 只有一条尺接得住：**拿公开树当本机一份 checkout 跑一遍**。
本脚本就是那把尺：`git clone --depth 1` 公开树到临时目录 ⇒ 跑「关键测试子集」⇒ 逐条点名。

两个探针（关键测试子集 · 都是「不依赖本机状态」的纯源码判据）
--------------------------------------------------------------
  P1 `redact 组`      `cd <树>/core && go test ./internal/obs/redact/ -count=1`
                      判据 = **rc=0 且输出不含具名红因**（`LEAK:` / `夹具失效` / `--- FAIL`）——
                      ①②两类红都落在这里，且这个包是**公开面替换的直接受害者**（必要条件门 + 夹具长度假设）。
  P2 `ui i18n 门禁`   `cd <树> && python3 ui/scripts/check-i18n.py`
                      判据 = **rc=0 且输出不含 `[ERROR]`** —— 公开面文案铁律的机械尺，本次唯一的「真红」就在它手上。

判据不是「跑起来就行」：**每个探针都配一组成对的 must-fail / 正控 / 空转**（见 `--self-test`），
其中 must-fail 用的就是**今天那两类红的原样文本** —— 自检不过 ⇒ **拒绝扫真目标**（rc=2，仓内口径）。

退码（三档 · 与仓内其余门脚本同口径）
--------------------------------------
  0 = 全绿（两个探针都 rc=0 且零具名红因）
  1 = **有命中**（任一探针红 —— 逐条给出探针 / rc / 具名红因 / 原样行）
  2 = **不给结论**：clone 失败 / 树缺件（没有 `core/` 或 `ui/`）/ 探针工具不在（go 不在）/
      探针零输出（空转）/ 一个探针都没跑成 / `--self-test` 不过 —— **缺件与空转一律不当绿**

用法
----
    python3 scripts/gates/check-public-tree-selfcheck.py                 # 真目标（clone 公开树到临时目录）
    python3 scripts/gates/check-public-tree-selfcheck.py --tree <目录>    # 复用已物料化的公开树（离线/CI 用）
    python3 scripts/gates/check-public-tree-selfcheck.py --keep          # 留下临时树（取证用 · 默认删）
    python3 scripts/gates/check-public-tree-selfcheck.py --json a,b      # 机器面
    python3 scripts/gates/check-public-tree-selfcheck.py --self-test     # 成对负控自检（合成夹具 · 不碰真目标 · 不走网）
    python3 scripts/gates/check-public-tree-selfcheck.py --list-rules    # 打判据 / 探针 / 档位

挂闸：`scripts/gates/precommit-gates.sh` 的 scope `public` · 模式 `tri-report`（**只报告档起步**）·
      **不进 `FAST_STEPS`**（要 clone + 跑一个 Go 包，既不便宜也不离线可用）。

档位口径（与仓内其它「只报告」步一字不差）：门脚本自己的 rc 判据 0/1/2 **三档分明、一字未改**；
「只报告」是**挂它的那一步**在步骤表里申报的档位 —— rc=1 落 REPORT（不计失败项、不计 BLOCKED、不改退出码），
rc=2 仍是 BLOCKED（**不给结论 ≠ 绿**）。

双方 HEAD sha（设计-CI适配-v1.1 §十三 `M4` · 波A `A3`）
----------------------------------------------------
**缺漏**（`M4` 逐字）：「§十 的闸 `git clone --depth 1 …/zerg-swarm.git` —— clone 的是**当前公开树**，
不是**本次要比的快照** ⇒ 公开树落后/领先本机时，跑的根本不是同一份 ⇒ 读数不对应任何一次发布」。
处置（`M4` 逐字）：「闸必须打印并记录**双方 HEAD sha**；**不相等 ⇒ 明确输出「不对应」** ✗（不许沉默地跑 ✓）」。

⇒ 本闸**每次跑**都先打一块「双方 HEAD sha + 对应判据」，并把两枚 sha 记进 `--json` 面（可再落 `--sha-file`）：
    · 本机 HEAD = `git -C <仓根> rev-parse HEAD`（`--local-head` 可显式覆盖 —— 供**成对负控**用）；
    · 公开树 HEAD = 物料化后的那棵树的 `rev-parse HEAD`（clone 或 `--tree` 复用）；
    · 两枚**都有值**且相等 ⇒ 「**对应**」；都有值但不等 ⇒ 「**不对应**」（**明确打出来**，且**绝不当绿**：
      `rc` 至少 1 —— 哪怕两个探针全绿）；任一读不到 ⇒ 「**未证**」（不给结论 · rc=2）。
      口径：**不对应时探针的读数不作数**（它测的不是本次要比的那棵树）—— 所以打印块里逐条写明。
    · `--shas-only`：**只**核双方 HEAD sha（不跑探针）—— 正控/负控与日常对账的最短路径。
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))  # scripts/gates → 仓根

DEFAULT_REPO = "https://github.com/Mr2109/zerg-swarm.git"
DEFAULT_TIMEOUT = 900  # 秒：clone（耗时以该件自检自报为准 · 不在此写死）+ go 包编译 + 跑测试（耗时以该件自检自报为准 · 不在此写死，宽给）

# ── 探针表（**判据写在一处**：命令串、具名红因、必须存在的树内件）────────────────────────
PROBES = [
    {
        "id": "P1",
        "name": "redact 组（脱敏必要条件门 ↔ 公开面替换）",
        "cwd": "core",
        "argv": ["go", "test", "./internal/obs/redact/", "-count=1"],
        "needs_argv": ["go"],
        "needs_tree": ["core"],
        # 具名红因：命中即「这一类的红」，逐条打印原样行（不是只报 rc）
        "marks": ["LEAK:", "夹具失效"],
        "fail_lines": ["--- FAIL", "FAIL\t"],
        "why": "①②两类红都在这里：替换把 `~` 改成 `~/` ⇒ 必要条件门只认 /Users/、/home/ ⇒ LEAK；"
               "`Mr2109`→`Mr2109` 多 2 字节 ⇒ 夹具长度假设被打穿 ⇒ 夹具失效。",
    },
    {
        "id": "P2",
        "name": "ui i18n 门禁（公开面文案）",
        "cwd": ".",
        "argv": [sys.executable or "python3", "ui/scripts/check-i18n.py"],
        "needs_argv": [],
        "needs_tree": ["ui/scripts/check-i18n.py", "ui/locales", "ui/src"],
        "marks": ["[ERROR]"],
        "fail_lines": [],
        "why": "公开面文案铁律的机械尺；本次 run 唯一的「真红」（G4 两条中文字面量）就是它抓到的。",
    },
]

# ── 自检夹具：must-fail 用的是**今天那两类红的原样文本**；正控/空转成对给区分度 ──────────────
MF1_TEXT = (
    "--- FAIL: TestFailClosed_OverBudget (0.00s)\n"
    '    failclosed_test.go:71: LEAK: 秘密 "~/projects/zerg/vault.key" 以投影 "~/projects/zerg/vault.key" '
    '残留在 {"err":"open ~/projects/zerg/vault.key failed"}\n'
)
MF2_TEXT = (
    "--- FAIL: TestGateReverseControlOnEscapeArtifact (0.00s)\n"
    '    limits_test.go:464: 夹具失效：blob="cGVlciBNcjIxMDkgcnVuIGhlcmUgb2s="（len=32）\n'
)
MF3_TEXT = (
    "=== i18n 门禁 ===\n"
    "  键数 zh=570 en=570 ｜ 静态引用 411 ｜ 剩余中文字面量 11 行（基线 9）\n"
    '  [ERROR] G4 出现未登记的中文 UI 字面量（2 行，须抽成 i18n 键或登记进基线）：\n'
    '  [ERROR]       ui/src/app.rs:1829 main_view | } else if trust == "正式" {\n'
    "→ 门禁失败（3 项）\n"
)
PC1_TEXT = "ok  \tgithub.com/Mr2109/zerg-swarm/core/internal/obs/redact\t0.571s\n"
PC2_TEXT = "=== i18n 门禁 ===\n  键数 zh=570 en=570 ｜ 静态引用 411 ｜ 剩余中文字面量 9 行（基线 9）\n→ 门禁通过\n"

SELF_TEST_CASES = [
    # (用例名, 喂哪个探针, 文本, rc, 期望判决)
    ("MF1 替换打穿夹具（redact 必要条件门只认 /Users/、/home/ ⇒ ~/ 形态整段跳过）", "P1", MF1_TEXT, 1, "hit"),
    ("MF2 夹具长度漂（Mr2109→Mr2109 多 2 字节 ⇒ base64 带 = 打穿 len<24 守卫）", "P1", MF2_TEXT, 1, "hit"),
    ("PC1 正控：干净输出", "P1", PC1_TEXT, 0, "green"),
    ("VAC1 空转：零输出 + rc=0（**不许当绿**）", "P1", "", 0, "vacuous"),
    ("MF3 i18n 真红（今日 run 原样）", "P2", MF3_TEXT, 1, "hit"),
    ("PC2 正控：i18n 通过输出", "P2", PC2_TEXT, 0, "green"),
    ("VAC2 空转：零输出 + rc=0", "P2", "", 0, "vacuous"),
]


def probe_by_id(pid):
    for p in PROBES:
        if p["id"] == pid:
            return p
    raise KeyError(pid)


# ── 判决：唯一的判据出口（自检直接钉它，真目标也走它）────────────────────────────
def judge(text, rc, probe):
    """返回 (verdict, named_hits, fail_hits)。

    verdict 三值：green（rc=0 且零具名红因）· hit（有红）· vacuous（**不给结论** —— 空转）。
    口径：**零输出不算绿**（跑了个没产出的东西，等于没跑），与仓内「空转 ⇒ rc=2」一致。
    """
    named = [ln for ln in text.splitlines() if any(m in ln for m in probe["marks"])]
    fails = [ln for ln in text.splitlines() if any(m in ln for m in probe["fail_lines"])]
    if not text.strip():
        return "vacuous", named, fails
    if named or fails or rc != 0:
        return "hit", named, fails
    return "green", named, fails



# ── 双方 HEAD sha（§十三 `M4`）：判据写在一处，自检直接钉它 ──────────────────────────
CORRESPOND = "对应"
UNRESPOND = "不对应"
UNPROVEN = "未证"


def read_head(tree, full=True):
    """读一棵树的 HEAD sha（读不到 ⇒ 空串 —— **缺件 ≠ 绿**，由 judge_heads 判「未证」）。"""
    if not tree or shutil.which("git") is None:
        return ""
    argv = ["git", "-C", tree, "rev-parse"]
    if not full:
        argv.append("--short")
    argv.append("HEAD")
    try:
        p = subprocess.run(argv, capture_output=True, text=True, timeout=60, errors="replace")
    except (OSError, subprocess.TimeoutExpired):
        return ""
    if p.returncode != 0:
        return ""
    return (p.stdout or "").strip()


def judge_heads(local, public):
    """双方 HEAD sha 的对应判据 —— 唯一出口（自检直接钉它）。

    返回 (verdict, why)：`对应` / `不对应` / `未证`。
    ★ 为什么「不对应」不能只是打印：clone 的是**当前公开树**，不是本次要比的快照 ⇒ 不对应时
      探针读数**不作数**（测的不是同一份东西）。所以它必须进判决，不许沉默地跑。
    """
    local, public = (local or "").strip(), (public or "").strip()
    if not local or not public:
        lack = "本机 HEAD 读不到" if not local else "公开树 HEAD 读不到"
        return UNPROVEN, "%s ⇒ 两枚 sha 不齐，**给不出「对应/不对应」**（缺件 ≠ 绿）" % lack
    if local == public:
        return CORRESPOND, "双方 HEAD sha 逐字相同"
    return UNRESPOND, "本机 HEAD %s ≠ 公开树 HEAD %s —— clone 的是**当前公开树**，不是本次要比的快照" % (
        local[:12], public[:12])


def dedupe(lines, cap):
    """按「file:line」去重（同一处红会重复几十行）⇒ [(重复数, 原样行)]，取前 cap 组。"""
    order, seen = [], {}
    for ln in lines:
        m = re.match(r"\s*([^\s:]+:\d+):", ln)
        key = m.group(1) if m else ln.strip()[:80]
        if key not in seen:
            seen[key] = [0, ln.strip()[:200]]
            order.append(key)
        seen[key][0] += 1
    return [tuple(seen[k]) for k in order[:cap]], len(order)

def run_probe(tree, probe, timeout):
    """在公开树里跑一个探针 → (rc, text, seconds, note)。跑不起来 ⇒ rc=None + note。"""
    for a in probe["needs_argv"]:
        if shutil.which(a) is None:
            return None, "", 0.0, "工具不在 PATH：%s" % a
    for rel in probe["needs_tree"]:
        if not os.path.exists(os.path.join(tree, rel)):
            return None, "", 0.0, "树里缺件：%s" % rel
    cwd = os.path.normpath(os.path.join(tree, probe["cwd"]))
    t0 = time.time()
    try:
        p = subprocess.run(probe["argv"], cwd=cwd, capture_output=True, text=True,
                           timeout=timeout, errors="replace")
    except subprocess.TimeoutExpired:
        return None, "", time.time() - t0, "超时（>%ds）" % timeout
    except OSError as e:
        return None, "", time.time() - t0, "起不来：%s" % e
    return p.returncode, (p.stdout or "") + (p.stderr or ""), time.time() - t0, ""


# ── 公开树来源 ────────────────────────────────────────────────────────────
def materialize(repo, dest, timeout):
    if shutil.which("git") is None:
        return None, "git 不在 PATH ⇒ 给不出公开树"
    try:
        p = subprocess.run(["git", "clone", "--depth", "1", "-q", repo, dest],
                           capture_output=True, text=True, timeout=timeout, errors="replace")
    except subprocess.TimeoutExpired:
        return None, "clone 超时（>%ds）：%s" % (timeout, repo)
    except OSError as e:
        return None, "clone 起不来：%s" % e
    if p.returncode != 0:
        return None, "clone 失败（rc=%d）：%s" % (p.returncode, (p.stderr or "").strip()[:300])
    head = ""
    try:
        # §十三 M4：读**全 sha**（短 sha 比不出「对应」的确定性 —— 要逐字比 40 位）。
        h = subprocess.run(["git", "-C", dest, "rev-parse", "HEAD"],
                           capture_output=True, text=True, timeout=60)
        head = (h.stdout or "").strip()
    except Exception:
        pass
    return head, ""


# ── 自检（成对负控 · 含今天那两类红的 must-fail）──────────────────────────────────
def self_test():
    ok, bad = 0, []
    seen = {}
    for name, pid, text, rc, want in SELF_TEST_CASES:
        probe = probe_by_id(pid)
        got, named, fails = judge(text, rc, probe)
        if got == want:
            ok += 1
            print("   ✓ %s ⇒ %s" % (name, got))
        else:
            bad.append(name)
            print("   ✗ %s ⇒ 期望 %s，实得 %s（具名红因 %d 条 · FAIL 行 %d 条）"
                  % (name, want, got, len(named), len(fails)))
    # 区分度：同一段文本在 must-fail 用例里必须红、在正控里必须绿（否则判据没牙）
    mf1 = judge(MF1_TEXT, 1, probe_by_id("P1"))[0]
    pc1 = judge(PC1_TEXT, 0, probe_by_id("P1"))[0]
    if mf1 == "hit" and pc1 == "green":
        ok += 1
        print("   ✓ 区分度：同一探针在 MF1 ⇒ hit、在 PC1 ⇒ green（判据有牙，不是恒绿/恒红）")
    else:
        bad.append("区分度")
        print("   ✗ 区分度：MF1 ⇒ %s、PC1 ⇒ %s（判据没有证明自己有区分能力）" % (mf1, pc1))
    # 具名红因必须与探针绑定：P1 认「夹具失效」，P2 认「[ERROR]」，互换必须不命中
    if ("夹具失效" in probe_by_id("P1")["marks"]) and ("夹具失效" not in probe_by_id("P2")["marks"]):
        ok += 1
        print("   ✓ 具名红因与探针绑定：『夹具失效』只在 P1 的 marks 里（换到 P2 不命中）")
    else:
        bad.append("具名红因绑定")
    # 两类红的 must-fail 各自点名（不许用一条把两类都糊过去）
    kinds = {n: judge(t, rc, probe_by_id(p))[0] for n, p, t, rc, _w in SELF_TEST_CASES[:2]}
    if all(v == "hit" for v in kinds.values()):
        ok += 1
        print("   ✓ must-fail 两类分列：替换打穿夹具 / 夹具长度漂 各自被点名（都不是靠 rc 蹭红）")
    else:
        bad.append("两类 must-fail 分列")
    # ── 双方 HEAD sha 判据（§十三 `M4` · 波A `A3`）：正控（同 sha）· **负控（喂一个假 HEAD ⇒ 必报不对应）**
    #    · 缺一侧 ⇒ 未证（不给结论）三条成对给区分度 —— 判据没牙的话这三条里至少有两条过不去。
    v_pos = judge_heads("a" * 40, "a" * 40)
    v_neg = judge_heads("a" * 40, "b" * 40)
    v_lack = judge_heads("", "b" * 40)
    if v_pos[0] == CORRESPOND:
        ok += 1
        print("   ✓ 正控：双方同 sha ⇒ **对应**（本机 aaaa… = 公开树 aaaa…）")
    else:
        bad.append("双方 HEAD sha 正控（同 sha）")
        print("   ✗ 正控：同 sha 却判成 %s" % v_pos[0])
    if v_neg[0] == UNRESPOND:
        ok += 1
        print("   ✓ 负控：**喂一个假 HEAD**（公开树 bbbb… ≠ 本机 aaaa…）⇒ **不对应**（%s）" % v_neg[1])
    else:
        bad.append("双方 HEAD sha 负控（假 HEAD）")
        print("   ✗ 负控：假 HEAD 却判成 %s —— 「不对应」没进判决（判据没牙）" % v_neg[0])
    if v_lack[0] == UNPROVEN:
        ok += 1
        print("   ✓ 缺一侧 sha（本机读不到）⇒ **未证**（不给结论，不许当绿）：%s" % v_lack[1])
    else:
        bad.append("双方 HEAD sha 缺一侧 ⇒ 未证")
    # ── ★ 回归自测格（缺口 `GAP-20260928-51` · 2026-09-28 补）：**真探针执行面 `run_probe()` / 取数面 `read_head()`** 不许哑化 ──
    #   病灶（本格是牙齿 · 现状实测）：本自测此前只喂**判决面**（`judge()` / `judge_heads()` 的合成文本），
    #   而**真探针执行面 `run_probe()` 与取数面 `read_head()` 一个都不进自检** ⇒ 把 `run_probe()` 体
    #   首行后插一行 `return []` 哑化后 `--self-test` 仍 `rc=0`「13 条全过」（假安全；2026-09-28
    #   仓外硬拷贝实测复现：哑化后 `py_compile` rc=0、自测 rc=0 —— 因为自检从不调 `run_probe()`）。
    #   治法：拿一个**受控探针**（合成命令，输出里带具名红因）喂给**真执行面 `run_probe()`**，断言它
    #   真跑、真回传文本、且 `judge()` 对这段**真输出**真判 `hit`；再拿一枚真 git 夹具喂 **`read_head()`**
    #   断言它真读出 40 位 HEAD。谁把这两处哑化成空/异常，本格立刻变红（不靠异常崩，逐格点名 rc=2）。
    _sx_probe = {"id": "SX", "name": "自检受控探针", "cwd": ".",
                 "argv": [sys.executable or "python3", "-c", "print('LEAK: selftest-synthetic')"],
                 "needs_argv": [], "needs_tree": [], "marks": ["LEAK:"],
                 "fail_lines": ["--- FAIL"], "why": "自检受控探针（合成输出 · 不碰真目标/不走网）"}
    _sx_root = tempfile.mkdtemp(prefix="selfcheck-selftest-")
    _sx_cases = []
    try:
        _rc, _text, _secs, _note = run_probe(_sx_root, _sx_probe, 60)
        _v = judge(_text or "", _rc if _rc is not None else 1, _sx_probe)[0]
        _sx_cases = [
            (_rc == 0, "run_probe 真跑合成命令 ⇒ rc=0（现读 %r）" % (_rc,)),
            ("LEAK:" in (_text or ""), "run_probe 真回传子进程输出（现读 %r）" % ((_text or "")[:80],)),
            (_v == "hit", "判决面对**真执行面**输出真判 `hit`（现读 %s）" % _v),
        ]
    except Exception as _exc:
        _sx_cases = [(False, "真执行面 `run_probe()` 哑化/异常（%s: %s）⇒ 不许静默绿"
                      % (type(_exc).__name__, _exc))]
    try:
        _sx_env = dict(os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@e",
                       GIT_COMMITTER_NAME="t", GIT_COMMITTER_EMAIL="t@e")
        subprocess.run(["git", "init", "-q", "-b", "main", _sx_root], check=True, env=_sx_env)
        subprocess.run(["git", "-C", _sx_root, "commit", "-q", "--allow-empty", "-m", "fx"],
                       check=True, env=_sx_env)
        _sx_want = subprocess.run(["git", "-C", _sx_root, "rev-parse", "HEAD"],
                                  capture_output=True, text=True).stdout.strip()
        _sx_got = read_head(_sx_root, full=True)
        _sx_cases.append((_sx_got == _sx_want and len(_sx_got) == 40,
                          "read_head 真读出 40 位 HEAD（现读 %r）" % (_sx_got or "")))
    except Exception as _exc2:
        _sx_cases.append((False, "取数面 `read_head()` 哑化/异常（%s: %s）⇒ 不许静默绿"
                          % (type(_exc2).__name__, _exc2)))
    finally:
        shutil.rmtree(_sx_root, ignore_errors=True)
    for _okx, _whyx in _sx_cases:
        if _okx:
            ok += 1
            print("   ✓ %-32s | %s" % ("runprobe/readhead-truly-live", _whyx))
        else:
            bad.append("runprobe/readhead-truly-live")
            print("   ✗ %-32s | %s" % ("runprobe/readhead-truly-live", _whyx))
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if not bad else "**不过**", ok + len(bad), len(bad)))
    return 0 if not bad else 2


# ── 主流程 ────────────────────────────────────────────────────────────────
# ── 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的**字段名打错**不许静默 / 不许炸堆栈 ✗
#    口径照 `check-replace-rule-fixtures.py`（`:94` / `:395` / `:401-410`）：**用法错 `rc=2` ＋ 印可选字段表** ——
#    「字段名打错 ≠ 零命中」两态在机器面上必须分得开；**合法字段照旧走命中面**（业务判据与退码一字未动）。
JSON_FIELDS = ['schema', 'tree', 'head', 'local_head', 'correspondence', 'correspondence_why', 'note', 'rows']          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_unknown_json_fields(argv=None):
    """`--json <字段>` 里有**不认的名** ⇒ rc=2 ＋ 印可选字段表；没给 / 全合法 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    want = []
    for i, x in enumerate(argv):
        if x == "--json":
            want += (argv[i + 1] if i + 1 < len(argv) else "").split(",")
        elif x.startswith("--json="):
            want += x.split("=", 1)[1].split(",")
    want = [f.strip() for f in want if f.strip()]
    bad = [f for f in want if f not in JSON_FIELDS]
    if not bad:
        return None
    print("⛔ `--json` 不认的字段名：%s" % " · ".join("`%s`" % b for b in bad), file=sys.stderr)
    print("   可选字段表：%s" % " · ".join("`%s`" % f for f in JSON_FIELDS), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_unknown_json_fields()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--repo", default=DEFAULT_REPO, help="公开树来源（默认 %s）" % DEFAULT_REPO)
    ap.add_argument("--tree", default="", help="复用已物料化的公开树（给了就不 clone）")
    ap.add_argument("--dest", default="", help="clone 落点（默认 mktemp）")
    ap.add_argument("--keep", action="store_true", help="留下临时树（取证用；默认删）")
    ap.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT, help="每个探针/整次 clone 的超时秒数")
    ap.add_argument("--max-hits", type=int, default=12, help="具名红因最多列几组（去重后 · 默认 12）")
    ap.add_argument("--max-fail-lines", type=int, default=8, help="FAIL 行最多列几组（去重后 · 默认 8）")
    ap.add_argument("--json", default="", help="机器面字段（逗号分隔）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成夹具 · 不碰真目标 · 不走网）")
    # §十三 M4（波A A3）：双方 HEAD sha 的取证面 —— 两个覆盖开关给**离线/合成骨架**用（写法同仓内
    # 其余「显式覆盖」：写错即不给结论，不许默认成某一侧）；`--shas-only` 只核 sha（不跑探针）。
    ap.add_argument("--local-head", default="", help="显式给本机 HEAD sha（缺省现读 `git -C <仓根> rev-parse HEAD`）")
    ap.add_argument("--public-head", default="", help="显式给公开树 HEAD sha（缺省现读物料化那棵树的 HEAD）")
    ap.add_argument("--shas-only", action="store_true", help="**只**核双方 HEAD sha（不跑探针）")
    ap.add_argument("--sha-file", default="", help="把双方 sha + 对应判据落成一份 JSON（记录用 · 缺省只打印）")
    ap.add_argument("--list-rules", action="store_true", help="打判据 / 探针 / 档位")
    args = ap.parse_args()

    if args.list_rules:
        print("⑭ 公开树自洽检查（③+⑦ 合并）· 只报告档起步 · 不进 FAST_STEPS")
        print("真源：Zerg-内部文档/项目文档/v2.5.11/设计-CI适配-v1.1.md §三 交叉验证点 2+5 · §五 第一批 ③")
        for p in PROBES:
            print("  %s %s —— cwd=%s · %s" % (p["id"], p["name"], p["cwd"], " ".join(p["argv"])))
            print("     具名红因：%s" % " · ".join(p["marks"]))
            print("     为什么：%s" % p["why"])
        print("退码：0 全绿 · 1 有命中 · 2 不给结论（clone 失败/缺件/空转/自检不过）")
        print("双方 HEAD sha（§十三 M4 · A3）：打印 + 记录两枚 sha；**不相等 ⇒ 明确输出「不对应」**；"
              "探针全绿也**不当绿**（rc≥1）；任一读不到 ⇒ 「未证」· rc=2；`--shas-only` 只核 sha")
        return 0

    if args.self_test:
        print("── 公开树自洽检查 · 自检（成对负控 · 合成夹具）──")
        return self_test()

    t_start = time.time()
    tree, note, head, cleanup = "", "", "", None
    if args.tree:
        tree = os.path.abspath(args.tree)
        if not os.path.isdir(tree):
            print("缺件：--tree %s 不是目录 ⇒ 不给结论" % tree)
            print("⇒ rc=2")
            return 2
        note = "复用已物料化的公开树（未 clone）"
    else:
        dest = args.dest or tempfile.mkdtemp(prefix="zerg-selfcheck-")
        cleanup = dest
        head, err = materialize(args.repo, dest, args.timeout)
        if head is None:
            print("公开树拿不到：%s" % err)
            print("⇒ 缺件 ≠ 绿：本门不给结论（rc=2）—— 不许把「clone 失败」当通过")
            return 2
        tree = dest
        note = "git clone --depth 1 %s" % args.repo

    # ── 双方 HEAD sha（设计-CI适配-v1.1 §十三 `M4` · 波A `A3`）────────────────────────
    # 为什么必须先打这一块：clone 的是**当前公开树**，不是本次要比的快照 ⇒ 公开树落后/领先本机时
    # 跑的根本不是同一份 ⇒ 读数不对应任何一次发布。⇒ 两枚 sha 都打印 + 记录，不等就**明确说不对应**，
    # 并且**不许沉默地当绿**（探针全绿 + 不对应 ⇒ rc≥1）。
    local_head = (args.local_head or "").strip() or read_head(REPO_ROOT, full=True)
    if (args.public_head or "").strip():
        public_head, pub_src = (args.public_head or "").strip(), "显式 --public-head"
    elif args.tree:
        public_head, pub_src = read_head(tree, full=True), "现读 git -C %s rev-parse HEAD" % tree
    else:
        public_head, pub_src = head, "现读 clone 落地树的 HEAD"
    corr, corr_why = judge_heads(local_head, public_head)
    print("── 双方 HEAD sha（§十三 M4 · 波A A3）──")
    print("   本机 HEAD  ：%s   ← %s" % (local_head or "（读不到）",
          "显式 --local-head" if args.local_head else "现读 git -C %s rev-parse HEAD" % REPO_ROOT))
    print("   公开树 HEAD：%s   ← %s" % (public_head or "（读不到）", pub_src))
    print("   对应判据   ：%s —— %s" % (corr, corr_why))
    if corr == UNRESPOND:
        print("   ⚠ **不对应** ⇒ 本次物料化的**不是「要和本机比的那一份」**：下面探针的读数**不作数**（只当旁证）。")
    elif corr == UNPROVEN:
        print("   ⚠ **未证** ⇒ 两枚 sha 不齐 ⇒ 给不出「对应/不对应」（缺件 ≠ 绿）。")
    if args.sha_file:
        try:
            with open(args.sha_file, "w", encoding="utf-8") as f:
                json.dump({"schema": "zerg-public-tree-selfcheck-shas/1", "local_head": local_head,
                           "public_head": public_head, "correspondence": corr, "why": corr_why,
                           "tree": tree, "note": note, "when": time.strftime("%Y-%m-%dT%H:%M:%S%z")},
                          f, ensure_ascii=False, indent=1)
            print("   已记录双方 sha → %s" % args.sha_file)
        except OSError as e:
            print("   记录写不进去（%s）⇒ 不给结论 · rc=2" % e)
            return 2
    if args.shas_only:
        rc = {CORRESPOND: 0, UNRESPOND: 1, UNPROVEN: 2}[corr]
        print("⇒ --shas-only：**只核双方 HEAD sha**（不跑探针）⇒ %s · rc=%d" % (corr, rc))
        return rc

    rows, verdicts = [], []
    try:
        for p in PROBES:
            rc, text, secs, why = run_probe(tree, p, args.timeout)
            if rc is None:
                rows.append({"probe": p["id"], "name": p["name"], "verdict": "vacuous",
                             "rc": "", "seconds": "%.1f" % secs, "hits": 0, "note": why})
                verdicts.append("vacuous")
                continue
            v, named, fails = judge(text, rc, p)
            verdicts.append(v)
            rows.append({"probe": p["id"], "name": p["name"], "verdict": v,
                         "rc": str(rc), "seconds": "%.1f" % secs, "hits": len(named) + len(fails),
                         "note": ""})
            print("── %s %s ──" % (p["id"], p["name"]))
            print("   命令：%s（cwd=%s）· rc=%d · %.1fs" % (" ".join(p["argv"]), p["cwd"], rc, secs))
            grp, total = dedupe(named, args.max_hits)
            if grp:
                print("   ✗ 具名红因：%d 行 → 去重 %d 组（列前 %d 组）"
                      % (len(named), total, len(grp)))
                for n, ln in grp:
                    print("      [%d×] %s" % (n, ln))
            gf, tf = dedupe(fails, args.max_fail_lines)
            if gf:
                print("   ✗ FAIL 行：%d 行 → 去重 %d 组（列前 %d 组）"
                      % (len(fails), tf, len(gf)))
                for n, ln in gf:
                    print("      [%d×] %s" % (n, ln))
            if v == "green":
                print("   ✓ 无具名红因 · rc=0")
            elif v == "vacuous":
                print("   ⚠ 零输出 ⇒ 空转：**不给结论**（不当绿）")
            print("   为什么这条探针在：%s" % p["why"])
    finally:
        if cleanup and not args.keep:
            shutil.rmtree(cleanup, ignore_errors=True)

    if args.json:
        print(json.dumps({"schema": "zerg-public-tree-selfcheck/1", "tree": tree, "head": public_head,
                          "local_head": local_head, "correspondence": corr, "correspondence_why": corr_why,
                          "note": note, "rows": rows}, ensure_ascii=False))

    green = verdicts.count("green")
    hit = verdicts.count("hit")
    vac = verdicts.count("vacuous")
    print("── 公开树自洽检查 · 小结 ──")
    print("   树：%s（%s）" % (tree, note))
    print("   双方 HEAD：本机 %s ｜ 公开树 %s ｜ %s" % (local_head or "（读不到）", public_head or "（读不到）", corr))
    print("   探针 %d 个：绿 %d · 命中 %d · 不给结论 %d · 全程 %.1fs"
          % (len(verdicts), green, hit, vac, time.time() - t_start))
    if green == 0 and hit == 0:
        print("⇒ 一个探针都没跑成 ⇒ 不给结论 · rc=2")
        return 2
    if vac:
        print("⇒ 有探针空转/跑不起来（不给结论 ⇒ 不当绿）· rc=2")
        return 2
    if hit:
        print("⇒ 有命中（只报告档：挂闸那一步按 tri-report 落 REPORT）· rc=1")
        return 1
    if corr != CORRESPOND:
        rc = 2 if corr == UNPROVEN else 1
        print("⇒ 探针%s，但双方 HEAD **%s** ⇒ 本轮**不当绿**（读数不对应任何一次发布）· rc=%d"
              % ("全绿" if not hit else "有命中", corr, rc))
        return rc
    print("⇒ 全绿 · rc=0（双方 HEAD %s ⇒ 读数对应）" % CORRESPOND)
    return 0


if __name__ == "__main__":
    sys.exit(main())
