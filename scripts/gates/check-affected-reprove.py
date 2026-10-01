#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-affected-reprove.py —— 「受影响面声明表是否漏声明」的**反证体检**（主法丙·成对扰动式 · 首件）

病象（承 `GAP-20261001-02`）：静态差集（`check-affected-map.py`）只能证「表里写了盘上没有」与
「盘上有表里没写」两向，**证不了**「一条已写的声明是不是真的」；而「漏声明」更静态发现不了。
方案（已定 · 见 `方案-反证体检.md`）：**主法 = 丙 成对扰动式** —— 对一件做**语义扰动**
⇒ **依赖它的那一步必须变红**（真依赖被反证出来）；**仍全绿 = 该步并不真读它**（真无依赖）。
甲（快照 diff）只作隔离前置（对「读」是瞎的）· 乙（syscall 跟踪）本机不可得（已在账 · ✗ 不再试）。

判据（**只读**（除仓外副本与受控回滚）· 四档退码 0/1/2/3）
  C0 **成对自证**：每个样本**必须**同时给出「扰动后 sha ≠ 扰动前 sha」（扰动真生效）**且**
     「复原后 sha == 扰动前 sha」（复原逐字节）。任一不成立 ⇒ **不给结论**（C0 失败 ⇒ 2 / 复原失败 ⇒ 3）。
  C1 **判据先绿前置**：样本名下的每一步，**扰动前**必须 rc=0。不绿 ⇒ 该步**不给结论**
     （✗ 不许把「本来就红」读成「扰动引起的红」）。
  C2 **成对判读**：扰动后跑同一步 —— rc≠0 ⇒ **真依赖**；rc=0 ⇒ **真无依赖**。两态**都必须可给**。
  C3 **无步引用 ⇒ 不给结论**：样本件不在任何一步的命令位里 ⇒ rc=2（✗ 不许读成「无依赖」）。
  C4 **空转可分辨**：报告必须同时打印 `样本 K 件 · 步 M 条 · 真依赖 A · 真无依赖 B · 不给结论 C`
     —— 「0 处」与「没跑」在读数上可分辨（照 `check-newcmd-followup.py` C5 / `affected-steps.py` C3）。
  C5 **纯只读**：默认档 **✗ 不写盘**（只在仓外 $TMPDIR 落备份 + 受控回滚；跑前跑后 sha256 逐字节对拍）。

  C6 **手法按步档可选**（`--mutation`：`insert` 默认 · `semantic` rc=1 语义词族 · `auto` 按步档选）：
     `tri`（阻断档）与 `tri-report`（只报档）**只差 rc=1 那一格** —— 旧手法 `insert`（前置 `exit(97)`）
     在两档下都落 **FAIL**（异常码 `97` 是异常档）⇒ 「只报档（tri-report）⇒ 真无依赖」这一态
     **结构上取不到**（26 件样本 0 例）；`semantic`（rc=1 语义词族：删函数体 / 改常量 / 输出格式错 ·
     兜底 rc1-exit）把「有」档落回**常态 rc=1** —— 阻断档 rc=1 ⇒ FAIL（真依赖）· 只报档 rc=1 ⇒
     REPORT（**不影响门退出码** ⇒ 真无依赖）。**默认 `insert` 一字不改**（旧样本结论不得变）。

安全铁律（照 `mutation-smoke.sh` / `two-states-gate-mutate.py` 四手 · 一条不放宽）
  · 备份**落仓外**（`$TMPDIR/affected-reprove.<pid>`）· ✗ 绝不在仓内留备份；
  · `atexit` + `SIGINT/SIGTERM` 双保险复原 ⇒ 被 Ctrl-C / 被杀也把件放回；
  · 跑前跑后 **sha256 逐字节比对** ⇒ 复原不成 ⇒ **rc=3**（硬失败 · 不许静默）；
  · 扰动**只发生在仓内该样本件上**（受控回滚档），跑完**必须**与开跑前逐字节同值。

用法
  python3 scripts/gates/check-affected-reprove.py --list                 # 只列候选样本（不跑、不写盘）
  python3 scripts/gates/check-affected-reprove.py --sample 6             # 抽 6 件真跑
  python3 scripts/gates/check-affected-reprove.py --files a.py,b.py      # 点名取样
  python3 scripts/gates/check-affected-reprove.py --sample 6 --json
  python3 scripts/gates/check-affected-reprove.py --self-test            # 成对负控（仓外夹具硬拷贝 · 不碰真目标）
  python3 scripts/gates/check-affected-reprove.py --sample 6 --mutation semantic   # rc=1 语义词族手法
  python3 scripts/gates/check-affected-reprove.py --sample 6 --mutation auto       # 按步档（tri / tri-report）选手法

退出码：0 全部样本都给了确定结论且复原逐字节（且至少见一条「真依赖」）· 1 样本全落「真无依赖」（判据可能
        没牙 ⇒ 交回人）· 2 不给结论（判据先绿不成立 / 无步引用 / 扰动未生效 / 空转）· 3 复原失败
"""
import argparse
import atexit
import hashlib
import json
import os
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import types

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))

GATES_DIR_REL = os.path.join("scripts", "gates")
PRECOMMIT_REL = os.path.join("scripts", "gates", "precommit-gates.sh")
SELF_REL = os.path.join("scripts", "gates", os.path.basename(os.path.abspath(__file__)))

# 步表解析：唯一真源 = `precommit-gates.sh` 的 `add_step <scope> "<名>" <模式> "<目录>" "<命令>"`
ADD_STEP_RE = re.compile(r'add_step\s+(\S+)\s+"([^"]*)"\s+(\S+)\s+"([^"]*)"\s+"([^"]*)"', re.S)
GATE_SCRIPT_RE = re.compile(r'scripts/gates/([A-Za-z0-9_.\-]+)')

EXIT_OK, EXIT_UNRESOLVED, EXIT_HARD, EXIT_RESTORE = 0, 1, 2, 3

# 扰动体（**前置插入** ⇒ 语义确定、无语法风险；只在样本件上，跑完必复原）
MUT_PY = "import sys as _reprove_sys; _reprove_sys.exit(97)  # 反证扰动（体检用 · 必复原）\n"
MUT_SH = "exit 97  # 反证扰动（体检用 · 必复原）\n"

# ── 手法档（`--mutation`）──────────────────────────────────────────────────────────
#  insert  前置插入 `exit(97)`：旧手法（默认）—— 异常码 97 在 tri/tri-report 两档**都落 FAIL**，
#          ⇒ 只报档那条「REPORT / 门退码不变」的态取不到（结构盲区 · 本单要补的就是它）。
#  semantic rc=1 语义词族：改动**件自身的语义**，让子件以**常态 rc=1**（=「有」档）退出 ——
#          阻断档 rc=1 ⇒ FAIL · 只报档 rc=1 ⇒ REPORT（不影响门退出码 ⇒ 真无依赖）。
#          三种形态（上批实测有效）：删函数体 / 改常量 / 输出格式错；都不适用时兜底 `rc1-exit`。
#  auto    按步档选：样本被**只报档**（tri-report）任一引用 ⇒ `semantic`；否则（阻断档 / 其他）⇒ `insert`。
MUT_INSERT, MUT_SEMANTIC, MUT_AUTO = "insert", "semantic", "auto"
SEMANTIC_FORMS = ("gut-body", "flip-constant", "break-output", "rc1-exit")
REPORT_ONLY_MODES = ("tri-report",)

# 删函数体：只认**顶层** `def` / shell `name() {`（✗ 不碰缩进内的嵌套函数 —— 免得改坏闭包语义）
_DEF_RE_PY = re.compile(r"^def\s+[A-Za-z_]\w*\s*\(.*\)\s*(->[^:]*)?:\s*$")
_DEF_RE_SH = re.compile(r"^[A-Za-z_]\w*\s*\(\s*\)\s*\{")
# 改常量：顶层 `NAME = <字面量>`（全大写名 · 只收纯字面量行，免得碰复杂表达式）
_CONST_RE_PY = re.compile(r"^([A-Z][A-Z0-9_]{2,})\s*=\s*(.+?)\s*$")


def _gut_body_py(src):
    """删函数体：取**最后一个顶层 def**（惯例 = 入口 `main`），把它的体换成 `raise SystemExit(1)`。
    该函数一旦被跑（如 `sys.exit(main())`）⇒ 子件以 rc=1 退出（常态「有」档，不是异常码）。"""
    lines = src.split("\n")
    idx = None
    for i, ln in enumerate(lines):
        if _DEF_RE_PY.match(ln):
            idx = i
    if idx is None:
        return None
    j = idx + 1
    has_body = False
    while j < len(lines):
        ln = lines[j]
        if ln.strip() == "" or re.match(r"^\s", ln):
            if ln.strip() != "":
                has_body = True
            j += 1
            continue
        break
    if not has_body:
        return None
    return "\n".join(lines[:idx + 1] + ["    raise SystemExit(1)"] + lines[j:])


def _gut_body_sh(src):
    """删函数体（shell）：最后一个顶层 `name() {` 之后插一行 `return 1`。"""
    lines = src.split("\n")
    idx = None
    for i, ln in enumerate(lines):
        if _DEF_RE_SH.match(ln):
            idx = i
    if idx is None:
        return None
    return "\n".join(lines[:idx + 1] + ["  return 1"] + lines[idx + 1:])


def _flip_constant_py(src):
    """改常量：顶层大写常量行里的字面量换个值（字符串 ⇒ 换内容；整数 ⇒ +1）。"""
    lines = src.split("\n")
    for i, ln in enumerate(lines):
        m = _CONST_RE_PY.match(ln)
        if not m:
            continue
        rhs = m.group(2)
        if len(rhs) >= 2 and rhs[0] in "\"'" and rhs[-1] == rhs[0]:
            new = m.group(1) + " = " + rhs[:1] + "reprove-x" + rhs[-1:]       # 字符串常量换内容
        elif re.match(r"^-?\d+$", rhs):
            new = "%s = %d" % (m.group(1), int(rhs) + 1)                      # 整数常量 +1
        elif re.match(r"^[\(\[]", rhs) and re.match(r"[\)\]]$", rhs):
            new = m.group(1) + " = " + rhs[:-1] + (", " if rhs[:-1].rstrip().endswith("\"") or
                                                   rhs[:-1].rstrip().endswith("'") else "") + rhs[-1:]
        else:
            continue
        lines[i] = new
        return "\n".join(lines)
    return None


def _break_output_py(src):
    """输出格式错：把**第一个** `print(` 调用换成未定义名 ⇒ 运行到那句即 NameError（rc=1）。"""
    m = re.search(r"(?<![\w.])print\s*\(", src)
    if not m:
        return None
    return src[:m.start()] + "_reprove_out(" + src[m.end():]


def _rc1_exit(rel, src):
    """兜底：前置插入**常态 rc=1**（与 insert 同一插入点，只把异常码 97 换成语义码 1）。"""
    lines = src.split("\n")
    ins = ("exit 1  # 反证扰动（rc=1 语义词族 · 体检用 · 必复原）\n" if rel.endswith(".sh")
           else "import sys as _reprove_sys; _reprove_sys.exit(1)  # 反证扰动（rc=1 语义词族 · 必复原）\n")
    at = 1
    if rel.endswith(".py") and len(lines) > 1 and "coding" in lines[1]:
        at = 2
    lines.insert(at, ins.rstrip("\n"))
    return "\n".join(lines)


def semantic_mutate(rel, src):
    """rc=1 语义词族：⇒ (新正文, 手法名)；取不到可用形态 ⇒ 落兜底 `rc1-exit`。"""
    forms = (("gut-body", _gut_body_sh), ("rc1-exit", None)) if rel.endswith(".sh") else \
        (("gut-body", _gut_body_py), ("flip-constant", _flip_constant_py),
         ("break-output", _break_output_py), ("rc1-exit", None))
    for name, fn in forms:
        if fn is None:
            return _rc1_exit(rel, src), name
        try:
            out = fn(src)
        except Exception:                      # noqa: BLE001 —— 形态不适用 ⇒ 继续试下一形态
            out = None
        if out and out != src:
            return out, name
    return _rc1_exit(rel, src), "rc1-exit"


def choose_mutation(refs, mutation):
    """`--mutation auto` ⇒ 按步档选：被**只报档**任一引用 ⇒ semantic；否则 ⇒ insert。"""
    if mutation == MUT_SEMANTIC:
        return MUT_SEMANTIC
    if mutation == MUT_AUTO:
        return MUT_SEMANTIC if any(m in REPORT_ONLY_MODES for _n, m in refs) else MUT_INSERT
    return MUT_INSERT


def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def read(p):
    with open(p, encoding="utf-8", errors="replace") as f:
        return f.read()


def parse_steps(root):
    """→ [(scope, name, mode, dir, cmd)]；取不到件 ⇒ None。"""
    fp = os.path.join(root, PRECOMMIT_REL)
    if not os.path.isfile(fp):
        return None
    out = []
    for m in ADD_STEP_RE.finditer(read(fp)):
        out.append(m.groups())
    return out


def steps_by_file(steps):
    """gate 脚本 basename → [(步名, 模式)]（唯一真源 = 步表命令位里的 `scripts/gates/<件>`）。"""
    d = {}
    for _scope, name, mode, _dir, cmd in steps:
        for mt in GATE_SCRIPT_RE.finditer(cmd):
            d.setdefault(mt.group(1), [])
            if (name, mode) not in d[mt.group(1)]:
                d[mt.group(1)].append((name, mode))
    return d


def candidates(root, steps):
    """候选样本 = 步表**命令位**里按名点到的 `scripts/gates/` 件、且盘上真在、且**不是本件自己**。"""
    by = steps_by_file(steps)
    out = []
    self_base = os.path.basename(SELF_REL)
    for base in sorted(by):
        if base == self_base:
            continue
        rel = os.path.join(GATES_DIR_REL, base)
        if os.path.isfile(os.path.join(root, rel)):
            out.append((rel.replace(os.sep, "/"), by[base]))
    return out


def pick(samples, n):
    """确定性抽样：在排序表上等距取 n 件（首尾都取得到 · 不随机 ⇒ 可复跑）。"""
    if n >= len(samples):
        return list(samples)
    if n <= 1:
        return [samples[0]]
    idx = [round(i * (len(samples) - 1) / (n - 1)) for i in range(n)]
    seen, out = set(), []
    for i in idx:
        if i not in seen:
            seen.add(i)
            out.append(samples[i])
    return out


def run_step(root, name, timeout=600):
    """跑一步（只跑这一道门）⇒ rc。环境清面照派单铁律。"""
    env = dict(os.environ)
    for k in ("ZERG_STATE_DIR", "ZERG_REPO", "ZERG_REPO_ROOT", "ZERG_PROPOSAL_DIR"):
        env.pop(k, None)
    argv = ["bash", PRECOMMIT_REL, "--step", name, "--only-step"]
    try:
        p = subprocess.run(argv, cwd=root, env=env, stdout=subprocess.PIPE,
                           stderr=subprocess.STDOUT, timeout=timeout)
    except subprocess.TimeoutExpired:
        return 124, "[超时 %ss]" % timeout
    out = p.stdout.decode("utf-8", "replace")
    return p.returncode, out[-400:]


# ── 受控回滚（备份落**仓外** + atexit/signal 复原 + sha 逐字节核验）─────────────────────
class Guard:
    """一枚样本件的「改动→复原」护栏。备份在**仓外**；任何出口都把它放回。"""

    def __init__(self, root, rel, backup_dir):
        self.root = root
        self.rel = rel
        self.abs = os.path.join(root, rel)
        self.backup_dir = backup_dir
        self.bak = os.path.join(backup_dir, rel.replace("/", "__")) if backup_dir else ""
        self.pre = ""
        self.active = False

    def arm(self):
        self.pre = sha(self.abs)
        if self.backup_dir:
            os.makedirs(self.backup_dir, exist_ok=True)
            shutil.copy2(self.abs, self.bak)
        self.active = True
        return self.pre

    def perturb(self, technique=MUT_INSERT):
        """扰动 ⇒ (ok, note, form)。ok=False 表示扰动**没生效**（硬失败）。
        `insert`（默认）= 旧手法一字不改；`semantic` = rc=1 语义词族。"""
        src = read(self.abs)
        form = "前置插入"
        if technique == MUT_SEMANTIC:
            new, form = semantic_mutate(self.rel, src)
            with open(self.abs, "w", encoding="utf-8") as f:
                f.write(new)
        else:
            lines = src.split("\n")
            ins = MUT_SH if self.rel.endswith(".sh") else MUT_PY
            # 插在 shebang（+ coding 行）之后：PEP 263 只认前两行
            at = 1
            if self.rel.endswith(".py") and len(lines) > 1 and "coding" in lines[1]:
                at = 2
            lines.insert(at, ins.rstrip("\n"))
            with open(self.abs, "w", encoding="utf-8") as f:
                f.write("\n".join(lines))
        now = sha(self.abs)
        if now == self.pre:
            return False, "扰动后 sha 未变（扰动没生效）⇒ 不给结论", form
        return True, now[:12], form

    def restore(self):
        """复原并**逐字节核验** ⇒ (ok, note)。"""
        if not self.active:
            return True, "未扰动"
        if self.backup_dir and os.path.isfile(self.bak):
            shutil.copy2(self.bak, self.abs)
        now = sha(self.abs)
        self.active = False
        if now != self.pre:
            return False, "复原后 sha 不一致（%s ≠ %s）⇒ 仓内可能残留扰动体" % (now[:12], self.pre[:12])
        return True, "复原逐字节一致 sha=%s" % now[:12]


_GUARDS = []


def _emergency_restore(*_a):
    for g in _GUARDS:
        try:
            g.restore()
        except Exception:
            pass


def install_traps():
    atexit.register(_emergency_restore)
    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            signal.signal(sig, lambda *a: (_emergency_restore(), sys.exit(EXIT_RESTORE)))
        except Exception:
            pass


def analyse(root, samples, backup_dir, verbose=False, mutation=MUT_INSERT):
    """→ (rc, lines, tally)。rc: 0/1/2/3。"""
    L = []
    add = L.append
    rc_global = EXIT_OK
    tally = {"samples": 0, "steps": 0, "dep": 0, "nodep": 0, "unresolved": 0}
    for rel, refs in samples:
        tally["samples"] += 1
        tech = choose_mutation(refs, mutation)
        add("── 样本 %s（被 %d 步按名点到：%s）"
            % (rel, len(refs), " · ".join("%s[%s]" % (n, m) for n, m in refs)))
        g = Guard(root, rel, backup_dir)
        pre = g.arm()
        _GUARDS.append(g)

        greens = []
        for name, mode in refs:
            tally["steps"] += 1
            rc, tail = run_step(root, name)
            if rc == 0:
                greens.append((name, mode))
                add("   判据先绿 ✓ %s rc=0" % name)
            else:
                tally["unresolved"] += 1
                rc_global = EXIT_HARD            # 照方案 §五.3 逐字：判据先跑不绿 ⇒ rc=2「不给结论」
                add("   **不给结论**：判据先绿不成立 —— `%s` 扰动前就 rc=%d（✗ 不许把「本来就红」"
                    "读成「扰动引起的红」）" % (name, rc))
                if verbose:
                    add("       尾部：%s" % tail.replace("\n", " ⏎ ")[:300])
        if not greens:
            _GUARDS.remove(g)
            g.restore()
            continue

        ok, note, form = g.perturb(tech)
        if not ok:
            add("   **不给结论**：%s" % note)
            _GUARDS.remove(g)
            g.restore()
            rc_global = EXIT_HARD
            continue
        if tech == MUT_SEMANTIC:
            add("   扰动生效 ✓ %s：sha %s → %s（rc=1 语义词族 · %s · 必复原）"
                % (rel, pre[:12], note, form))
        else:
            add("   扰动生效 ✓ %s：sha %s → %s（前置插入 · 必复原）" % (rel, pre[:12], note))

        for name, mode in greens:
            rc2, tail = run_step(root, name)
            if rc2 != 0:
                tally["dep"] += 1
                add("   **真依赖** ✓ %s 扰动后 rc=%d（该步确实读了 %s）" % (name, rc2, rel))
            else:
                tally["nodep"] += 1
                extra = ""
                if tech == MUT_SEMANTIC and mode in REPORT_ONLY_MODES:
                    extra = ("（只报档：rc=1 落 REPORT ⇒ **不影响门退出码** · 「只报档＝真无依赖」这一态"
                             "正是 exit(97) 结构上取不到的那格）")
                add("   **真无依赖** ✓ %s 扰动后仍 rc=0（该步**并不真读** %s —— 可正当留在未声明）%s"
                    % (name, rel, extra))
            if verbose:
                add("       尾部：%s" % tail.replace("\n", " ⏎ ")[:300])

        rok, rnote = g.restore()
        _GUARDS.remove(g)
        if not rok:
            add("   **硬失败（rc=3）**：%s" % rnote)
            return EXIT_RESTORE, L, tally
        add("   %s" % rnote)

    add("== 读数：样本 %d 件 · 步 %d 条 · 真依赖 %d · 真无依赖 %d · 不给结论 %d"
        % (tally["samples"], tally["steps"], tally["dep"], tally["nodep"], tally["unresolved"]))
    if tally["samples"] == 0:
        add("⇒ **不给结论（rc=2）**：样本集为空（空转 ≠ 全绿）")
        return EXIT_HARD, L, tally
    if tally["dep"] == 0 and tally["nodep"] == 0:
        add("⇒ **不给结论（rc=2）**：一条成对判读都没跑出来（空转 ≠ 全绿）")
        return EXIT_HARD, L, tally
    if rc_global == EXIT_OK and tally["dep"] == 0:
        add("⇒ rc=1：样本**全部**落「真无依赖」（一次红都没见）⇒ 判据可能没牙，✗ 不许读成「无漏声明」；"
            "交回人复核样本选取")
        return EXIT_UNRESOLVED, L, tally
    if rc_global == EXIT_OK:
        add("⇒ rc=0：逐件成对判读完成 · 复原逐字节一致 · 「0 处」与「没跑」已由上行读数分辨")
    else:
        add("⇒ rc=2：有样本落「不给结论」（上列逐条点名 · fail-closed）")
    return rc_global, L, tally


# ══════════════════════════ 自检（成对正负控 · 仓外夹具硬拷贝）══════════════════════════
_FIX_STUB = """#!/usr/bin/env bash
# 夹具步表（自检专用 · 仓外）：add_step 行是驱动解析的**唯一真源**；
# 下面的 runner 是这棵树自己的「闸」，按步档（tri / tri-report）把子件 rc 映射成门的出口码
#   —— 照真门 `_judge()` 的四档口径（tri-report 只差「rc=1 ⇒ 不影响退出码」那一格）。
FIX_ROOT="__FIXROOT__"
# add_step gates "夹具步A" tri "${REPO_ROOT}" "python3 scripts/gates/gate-a.py"
# add_step gates "夹具步B" tri "${REPO_ROOT}" "python3 scripts/gates/gate-b.py"
# add_step gates "夹具步C(吞rc)" tri "${REPO_ROOT}" "python3 scripts/gates/gate-c.py"
# add_step gates "夹具步D(只报)" tri-report "${REPO_ROOT}" "python3 scripts/gates/gate-d.py"
run() { ( cd "${FIX_ROOT}" && bash -c "$1" ); }
_j() {  # _j <模式> <子件 rc> ⇒ 门出口码（0 全绿 · 1 有失败项 · 2 不给结论）
  case "$1" in
    tri)        case "$2" in 0) return 0;; 1) return 1;; 2) return 2;; *) return 1;; esac ;;
    tri-report) case "$2" in 0) return 0;; 1) return 0;; 2) return 2;; *) return 1;; esac ;;
    rc)         case "$2" in 0) return 0;; *) return 1;; esac ;;
    *)          return 1 ;;
  esac
}
step=""; for a in "$@"; do case "$a" in --step) shift; step="$1"; break;; esac; done
case "$step" in
  "夹具步A") run "python3 scripts/gates/gate-a.py"; r=$?; _j tri "$r"; exit $?;;
  "夹具步B") run "python3 scripts/gates/gate-b.py"; r=$?; _j tri "$r"; exit $?;;
  "夹具步C(吞rc)") run "python3 scripts/gates/gate-c.py"; echo "夹具：步C 吞掉 rc ⇒ 恒 0"; _j tri 0; exit $?;;
  "夹具步D(只报)") run "python3 scripts/gates/gate-d.py"; r=$?; _j tri-report "$r"; exit $?;;
  *) echo "夹具：没有这一步「$step」" >&2; exit 2;;
esac
"""

# 夹具靶件：带**顶层 def**（入口 `main`）—— `semantic` 的「删函数体」形态要能落上去。
_FIX_GATE_A = """#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import os, sys


def main():
    p = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data.txt")
    return 0 if open(p).read().strip() == "OK" else 1


if __name__ == "__main__":
    sys.exit(main())
"""
_FIX_GATE_D = """#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import os, sys


def main():
    p = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data-d.txt")
    return 0 if open(p).read().strip() == "OK" else 1


if __name__ == "__main__":
    sys.exit(main())
"""
_FIX_GATE_B = "#!/usr/bin/env python3\nimport sys\nsys.exit(0)\n"
_FIX_C = "#!/usr/bin/env python3\nimport sys\nsys.exit(0)\n"


def _mk_fixture(base, name, gate_c_swallows=True):
    fix = tempfile.mkdtemp(prefix=name + "-", dir=base)
    g = os.path.join(fix, "scripts", "gates")
    os.makedirs(g)
    shutil.copy2(os.path.abspath(__file__), os.path.join(fix, SELF_REL))   # **硬拷贝**（禁符号链接农场）
    with open(os.path.join(g, "precommit-gates.sh"), "w", encoding="utf-8") as f:
        f.write(_FIX_STUB.replace("__FIXROOT__", fix))
    for n, body in (("gate-a.py", _FIX_GATE_A), ("gate-b.py", _FIX_GATE_B),
                    ("gate-c.py", _FIX_C), ("gate-d.py", _FIX_GATE_D)):
        with open(os.path.join(g, n), "w", encoding="utf-8") as f:
            f.write(body)
    for d in ("data.txt", "data-d.txt"):
        with open(os.path.join(g, d), "w", encoding="utf-8") as f:
            f.write("OK\n")
    return fix


def _run_fixture(fix, argv):
    env = dict(os.environ)
    for k in ("ZERG_STATE_DIR", "ZERG_REPO", "ZERG_REPO_ROOT", "ZERG_PROPOSAL_DIR"):
        env.pop(k, None)
    p = subprocess.run([sys.executable, os.path.join(fix, SELF_REL)] + list(argv),
                       cwd=fix, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return p.returncode, p.stdout.decode("utf-8", "replace")


_SELF_MODS = {}


def _fixture_module(fix):
    if fix not in _SELF_MODS:
        import importlib.util
        spec = importlib.util.spec_from_file_location("affected_reprove_defs", os.path.join(fix, SELF_REL))
        if spec is None or spec.loader is None:
            raise RuntimeError("装不上夹具模块：%s" % fix)
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        _SELF_MODS[fix] = mod
    return _SELF_MODS[fix]


def _inproc(fix, swap=None, argv=()):
    import contextlib
    import io
    mod = _fixture_module(fix)
    saved = {k: getattr(mod, k) for k in (swap or {})}
    for k, v in (swap or {}).items():
        setattr(mod, k, v)
    ar = sys.argv
    sys.argv = [mod.__file__] + list(argv)
    out = io.StringIO()
    try:
        with contextlib.redirect_stdout(out):
            rc = mod.main()
    finally:
        sys.argv = ar
        for k, v in saved.items():
            setattr(mod, k, v)
    return rc, out.getvalue()


WANT_CASES = 14


def self_test():
    fails = []
    total = [0]

    def case(tag, desc, ok, detail):
        total[0] += 1
        print("  %s %s %s\n       %s" % ("✓" if ok else "✗", tag, desc, detail))
        if not ok:
            fails.append("%s %s" % (tag, desc))

    print("── check-affected-reprove.py 自检（成对正负控 · 仓外夹具硬拷贝 · 不碰真目标）──")
    base = tempfile.mkdtemp(prefix="affected-reprove-selftest-")
    try:
        # ① 正控：夹具里 gate-a.py 被「夹具步A」按名点到 ⇒ 扰动必红 ⇒ 真依赖
        fix = _mk_fixture(base, "healthy")
        gsha = sha(os.path.join(fix, SELF_REL))
        tsha = sha(os.path.join(fix, "scripts", "gates", "gate-a.py"))
        rc, out = _run_fixture(fix, ["--files", "scripts/gates/gate-a.py"])
        case("①正控", "扰动 gate-a.py ⇒ 夹具步A 变红 ⇒ 判「真依赖」· rc=0 · 复原逐字节",
             rc == 0 and "真依赖" in out and "复原逐字节一致" in out and "· 真依赖 1 ·" in out,
             "rc=%d · 真依赖=%s · 复原=%s" % (rc, "真依赖" in out, "复原逐字节一致" in out))
        case("①b真目标只读", "自检全程只动仓外夹具：夹具驱动件与靶件 sha 与开跑前同值",
             sha(os.path.join(fix, SELF_REL)) == gsha and
             sha(os.path.join(fix, "scripts", "gates", "gate-a.py")) == tsha,
             "驱动件/靶件 sha %s" % ("同值" if sha(os.path.join(fix, SELF_REL)) == gsha else "**变了**"))

        # ② 成对负控：同一棵树、把「夹具步C」的 rc 吞掉 ⇒ 扰动后**仍绿** ⇒ 判「真无依赖」
        #    （成对：① 判红 / ② 判绿 ⇒ 判读**不是恒红**，两态都给得出来）
        rc, out = _run_fixture(fix, ["--files", "scripts/gates/gate-c.py"])
        case("②成对负控", "吞 rc 的步 ⇒ 扰动后仍 rc=0 ⇒ 判「真无依赖」· rc=1（一次红都没见 ⇒ 不许读成绿）",
             rc == 1 and "真无依赖" in out and "**真依赖** ✓" not in out,
             "rc=%d（期望 1）· 真无依赖=%s" % (rc, "真无依赖" in out))

        # ③ 负控：判据先绿不成立（夹具步A 基线本就红）⇒ 该步「不给结论」· rc=1 · 点名
        fix_bad = _mk_fixture(base, "redbaseline")
        with open(os.path.join(fix_bad, "scripts", "gates", "data.txt"), "w") as f:
            f.write("BROKEN\n")
        rc, out = _run_fixture(fix_bad, ["--files", "scripts/gates/gate-a.py"])
        case("③负控", "基线本就红 ⇒ 不给结论（rc=2）且逐字点名「判据先绿不成立」· 不报绿",
             rc == 2 and "判据先绿不成立" in out and "**真依赖** ✓" not in out,
             "rc=%d（期望 2）· 点名=%s" % (rc, "判据先绿不成立" in out))

        # ④ 负控：**复原失败** ⇒ rc=3（弄哑「还原」那一次 copy2：目标在 scripts/gates/ 下的落地一律吞掉）
        fix_nr = _mk_fixture(base, "no-restore")
        real_copy2 = shutil.copy2

        def _no_restore(src_, dst_, *a, **k):
            if os.sep + "scripts" + os.sep + "gates" + os.sep in os.path.abspath(dst_):
                return dst_                      # 故意不落地（备份那一次 dst 在仓外 TMPDIR ⇒ 照做）
            return real_copy2(src_, dst_, *a, **k)

        rc, out = _inproc(fix_nr, {"shutil": types.SimpleNamespace(
            copy2=_no_restore, rmtree=shutil.rmtree, copy=shutil.copy, copyfile=shutil.copyfile)},
            ["--files", "scripts/gates/gate-a.py"])
        still = "exit(97)" in read(os.path.join(fix_nr, "scripts", "gates", "gate-a.py"))
        case("④负控", "复原不落地 ⇒ 驱动 rc=3 且逐字点名「复原后 sha 不一致」（复原核验是活的，不是摆设）",
             rc == 3 and "复原后 sha 不一致" in out,
             "rc=%d（期望 3）· 扰动仍在盘上=%s（仓外夹具 · 随夹具清掉）" % (rc, still))

        # ⑤ 负控：样本件**不在任何一步的命令位**里 ⇒ rc=2 · 逐字「无步引用」
        fix_orphan = _mk_fixture(base, "orphan")
        with open(os.path.join(fix_orphan, "scripts", "gates", "gate-z.py"), "w") as f:
            f.write("#!/usr/bin/env python3\nimport sys\nsys.exit(0)\n")
        rc, out = _run_fixture(fix_orphan, ["--files", "scripts/gates/gate-z.py"])
        case("⑤负控", "件不在任何一步命令位 ⇒ rc=2「无步引用」（✗ 不许读成「无依赖」）",
             rc == 2 and "无步引用" in out,
             "rc=%d · 点名=%s" % (rc, "无步引用" in out))

        # ⑥ 防空绿：把 `sha()` 弄哑（恒返同值 ⇒ 「扰动生效」断言会瞎）⇒ 驱动必 rc=2（不许静默当绿）
        fix_mute = _mk_fixture(base, "mute-sha")
        rc, out = _inproc(fix_mute, {"sha": lambda p: "MUTE"},
                          ["--files", "scripts/gates/gate-a.py"])
        case("⑥防空绿", "弄哑 sha（扰动生效断言瞎）⇒ 驱动 rc=2 且点名「扰动没生效」（弄哑而不变=真源本来是死的）",
             rc == 2 and "扰动没生效" in out,
             "rc=%d · 点名=%s" % (rc, "扰动没生效" in out))

        # ⑦ 正控成对：镜像 ⑥ —— 把 sha 修回 ⇒ rc=0（判不了不是恒判不了）
        rc, out = _run_fixture(fix, ["--files", "scripts/gates/gate-a.py"])
        case("⑦正控成对", "与 ⑥ 成对：sha 真源在位 ⇒ 同一棵树回 rc=0（不是恒判不了）",
             rc == 0, "rc=%d" % rc)

        # ⑧ rc=1 语义词族 · 正控（**阻断档** tri）：删函数体 ⇒ 子件 rc=1 ⇒ FAIL ⇒ 判「真依赖」
        rc, out = _run_fixture(fix, ["--mutation", "semantic", "--files", "scripts/gates/gate-a.py"])
        case("⑧语义词族正控", "阻断档 + semantic（删函数体）⇒ 子件 rc=1 落 FAIL ⇒ 判「真依赖」· rc=0 · 复原",
             rc == 0 and "真依赖" in out and "gut-body" in out and "复原逐字节一致" in out,
             "rc=%d · 真依赖=%s · 手法=%s" % (rc, "真依赖" in out, "gut-body" in out))

        # ⑨ rc=1 语义词族 · 负控（**只报档** tri-report）：rc=1 落 REPORT ⇒ **不影响门退出码**
        #    ⇒ 判「真无依赖」· rc=1（一次红都没见 ⇒ 不许读成绿）—— 这一态 exit(97) 结构上取不到
        rc, out = _run_fixture(fix, ["--mutation", "semantic", "--files", "scripts/gates/gate-d.py"])
        case("⑨语义词族负控", "只报档 + semantic ⇒ rc=1 落 REPORT（门退码不变）⇒ 判「真无依赖」· rc=1",
             rc == 1 and "**真无依赖** ✓" in out and "**真依赖** ✓" not in out and "不影响门退出码" in out,
             "rc=%d（期望 1）· 真无依赖=%s · 不报真依赖=%s"
             % (rc, "**真无依赖** ✓" in out, "**真依赖** ✓" not in out))

        # ⑩ rc=1 语义词族 · 负控（**本来就红**）：基线红 ⇒ 仍「不给结论」（✗ 不许读成任一结论）
        rc, out = _run_fixture(fix_bad, ["--mutation", "semantic", "--files", "scripts/gates/gate-a.py"])
        case("⑩语义词族红基线", "基线本就红 + semantic ⇒ 仍「不给结论」（rc=2 · 逐字点名）· 不报绿不报红",
             rc == 2 and "判据先绿不成立" in out and "**真依赖** ✓" not in out and "**真无依赖** ✓" not in out,
             "rc=%d（期望 2）· 点名=%s" % (rc, "判据先绿不成立" in out))

        # ⑪ 按步档选（auto）：**只报档**样本 ⇒ auto 选 semantic（rc=1 语义词族）
        rc, out = _run_fixture(fix, ["--mutation", "auto", "--files", "scripts/gates/gate-d.py"])
        case("⑪auto按档选", "auto：只报档样本 ⇒ 选 rc=1 语义词族（真无依赖 · rc=1；不是前置插入）",
             rc == 1 and "**真无依赖** ✓" in out and "rc=1 语义词族" in out and "前置插入" not in out,
             "rc=%d（期望 1）· 语义词族=%s" % (rc, "rc=1 语义词族" in out))

        # ⑫ 按步档选（auto）：**阻断档**样本 ⇒ auto 仍选旧手法 insert（前置插入 · 与默认档同结论）
        rc, out = _run_fixture(fix, ["--mutation", "auto", "--files", "scripts/gates/gate-a.py"])
        case("⑫auto阻断档", "auto：阻断档样本 ⇒ 仍选旧手法「前置插入」⇒ 判「真依赖」· rc=0",
             rc == 0 and "真依赖" in out and "前置插入" in out,
             "rc=%d · 前置插入=%s" % (rc, "前置插入" in out))

        # ⑬ 默认档向后兼容：不给 `--mutation` ⇒ 与 ⑫ 同结论（一字不改的旧路径）· 复原逐字节
        gsha2 = sha(os.path.join(fix, "scripts", "gates", "gate-a.py"))
        rc, out = _run_fixture(fix, ["--files", "scripts/gates/gate-a.py"])
        case("⑬默认档不变", "不给 --mutation ⇒ 旧路径（前置插入）· 结论与 ⑫ 逐字同 · 靶件 sha 与开跑前同",
             rc == 0 and "真依赖" in out and "前置插入" in out and
             sha(os.path.join(fix, "scripts", "gates", "gate-a.py")) == gsha2,
             "rc=%d · 还原=%s" % (rc, sha(os.path.join(fix, "scripts", "gates", "gate-a.py")) == gsha2))
    finally:
        shutil.rmtree(base, ignore_errors=True)
    print("  自证用例：%d 条 · 断言条数 %d（少跑一格即失效）" % (total[0], WANT_CASES))
    if total[0] != WANT_CASES:
        print("✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2" % (WANT_CASES, total[0]),
              file=sys.stderr)
        return EXIT_HARD
    if fails:
        print("✗ 自检不过：")
        for x in fails:
            print("   " + x)
        return EXIT_UNRESOLVED
    print("⇒ 自检全过（rc=0）：正控 / 成对负控 / 判据先绿 / 无步引用 / 防空绿 / rc=1 语义词族（按档） 逐条对拍 ✓")
    return EXIT_OK


def main():
    ap = argparse.ArgumentParser(description="反证体检（主法丙·成对扰动式）：扰动一件，看哪一步真红")
    ap.add_argument("--root", default=DEFAULT_ROOT)
    ap.add_argument("--list", action="store_true", help="只列候选样本（不跑、不写盘）")
    ap.add_argument("--sample", type=int, default=0, help="等距抽 N 件真跑")
    ap.add_argument("--files", default="", help="点名样本（逗号分隔 · 仓内相对路径或 basename）")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--verbose", action="store_true")
    ap.add_argument("--mutation", choices=(MUT_INSERT, MUT_SEMANTIC, MUT_AUTO), default=argparse.SUPPRESS,
                    help="扰动手法：insert（默认 · 旧手法 exit(97)）· semantic（rc=1 语义词族）· auto（按步档选）")
    ap.add_argument("--self-test", action="store_true")
    ns = ap.parse_args()
    # 默认档（不给 `--mutation`）⇒ 手法档串**一字不打印**：默认档 stdout 与改前**逐字节**同。
    # 手法档串只在**显式给 `--mutation`** 时打印（新手法能力照旧 · 只是默认档不显示）。
    mutation_given = hasattr(ns, "mutation")
    mutation = getattr(ns, "mutation", MUT_INSERT)

    if ns.self_test:
        return self_test()

    root = os.path.abspath(ns.root)
    steps = parse_steps(root)
    if steps is None:
        print("[不给结论] 取不到步表 %s ⇒ rc=2（判不了 ≠ 绿）" % PRECOMMIT_REL, file=sys.stderr)
        return EXIT_HARD
    cands = candidates(root, steps)
    by = dict(cands)
    if ns.list:
        print("候选样本 %d 件（步表命令位按名点到 · 排序）：" % len(cands))
        for rel, refs in cands:
            print("  %-46s ← %d 步：%s" % (rel, len(refs), " · ".join("%s[%s]" % (n, m) for n, m in refs)))
        return EXIT_OK

    if ns.files:
        want = [x.strip() for x in ns.files.split(",") if x.strip()]
        samples = []
        for w in want:
            rel = ("scripts/gates/" + w) if os.sep not in w and "/" not in w else w
            refs = by.get(rel)
            if refs is None:
                print("[不给结论] 样本 %s 不在步表命令位里（无步引用）⇒ rc=2" % rel, file=sys.stderr)
                return EXIT_HARD
            samples.append((rel, refs))
    elif ns.sample > 0:
        samples = pick(cands, ns.sample)
    else:
        print("用法：--list | --sample N | --files a,b | --self-test（至少给一个 ⇒ 否则 rc=2）")
        return EXIT_HARD

    backup_dir = os.path.join(os.environ.get("TMPDIR", "/tmp"), "affected-reprove.%d" % os.getpid())
    # 备份根安全断言：仓外（照 mutation-smoke A10_SANDBOX_NO_REPO 口径）
    bd = os.path.abspath(backup_dir)
    if bd == root or bd.startswith(root + os.sep):
        print("[不给结论] 备份根落在真仓内（%s）⇒ 绝不在仓里留备份 ⇒ rc=2" % bd, file=sys.stderr)
        return EXIT_HARD
    install_traps()
    if mutation_given:
        print("== 反证体检（主法丙·成对扰动式）· 仓根 %s · 手法档 %s" % (root, mutation))
    else:
        print("== 反证体检（主法丙·成对扰动式）· 仓根 %s" % root)
    print("== 备份落仓外 %s（受控回滚 · atexit+signal 双保险 · 跑前跑后 sha256 逐字节）" % bd)
    try:
        rc, lines, tally = analyse(root, samples, backup_dir, ns.verbose, mutation)
    finally:
        _emergency_restore()
        shutil.rmtree(backup_dir, ignore_errors=True)
    if ns.json:
        payload = {"schema": "zerg/v1", "kind": "AffectedReprove", "rc": rc}
        if mutation_given:
            payload["mutation"] = mutation
        payload["tally"] = tally
        payload["lines"] = lines
        print(json.dumps(payload, ensure_ascii=False, indent=1))
    else:
        for x in lines:
            print(x)
    return rc


if __name__ == "__main__":
    sys.exit(main())
