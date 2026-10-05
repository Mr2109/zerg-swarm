#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-fields-face-parity.py —— 命令级 `fields` ⟷ 面级 `--json` 键集 对拍门（缺口 `Q-178`）。

为什么要有它（缺口账 `Q-178` · 2026-09-26 本枚现读复现）
  `check-cli-contract.py:113` 逐字 `'fields': bool(FIELDS_RE.search(block))` —— 它把
  `fields: []string{…}` 抓到的**名单折成 bool**，只判「**有没有**字段表」、**不判对不对** ✗。
  ⇒ 「命令树里声明的字段面」⟷「该命令**真吐**的 `--json` 键集」**无一条对拍**：
  两处名不同（声明多一名 / 少一名 / 拼错），**没有任何一层会报** —— 门全绿而消费方按声明拿键必空。

判据（任一不满足 ⇒ rc=1；判不了 / 空转 ⇒ rc=2，**不给结论**）
  ① **声明侧**：`core/cmd/zerg/main.go` 命令树里每条 `path: []string{…}` 带字段面的命令
     ⇒ 该命令的**声明字段集**（`FIELDS_RE` 同一形态，本件自己解析、不复用 bool）。
     ★ 拼接变量形态（缺口 `GAP-20260928-159` · 本件与 `check-cli-contract.py` **同源改**）：
       `fields: <标识符>`（如 `main.go` 的 `fields: devEditFields`，定义在 `family_dev_edit.go`）
       —— 只认 `[]string{…}` 字面量会把这类命令读成「**没有字段表**」= 字段面凭空消失（既漏判、
       也漏进 ① 的对拍面）。⇒ 本件按**与 `check-cli-contract.py` 同一口径**跨件解析该标识符
       （扫 `core/cmd/zerg` 包内全部非测试 `.go` 源，递归 ≤ 4 层展开 `[]string{…}` 字面量与
       `append` 拼接链，取**名集**——`compare()` 按集合比、不比序 ⇒ 次序无关）。
  ② **真身侧**：现跑 `<bin> <path…> --json zzz-nonexistent-zz`（**故意给一个不在表里的名**）
     ⇒ 字段面**执行前判** ⇒ rc=2 + stderr 逐字 `合法字段: a,b,c` = **真身闭集**。
     ★ 「不给字段」那一格**不是**本门的判据（那是 `K2` 甲/乙档 · 见 `Q-119`）—— 本门只对**名**。
  ③ **逐名对拍**：声明集 == 真身集 ⇒ 过；多一名 / 少一名 / 抄错一名 ⇒ 红并**逐名点名**。
  ④ **不打靶白名单 ＋ 杀进程组**（腿② 的两刀 · 自伤洞 · `GAP-20261002-11` / `-20261003-17`）：
     ★ **治本**：对「**会起整套门 / 长任务 / 阻塞**」的命令（`gate bench`（探针不带 `--fast`/`--scope`
       ⇒ 必是全量）· `gate run` · `gate wait` · `gap reread`）**一格不打靶**（记 `not_probed` + 为何）
       —— 它们的登记字段会过字段闸、随后命令**真跑一整趟全量门** ⇒ 探针超时只杀直接子 ⇒ 孙趟门跑
       成孤儿占闸（判据「宁少打不可误跑」）。
     ★ **纵深**：探针子进程一律 `start_new_session=True`（独立进程组）＋ 超时 `os.killpg` 收**整棵**
       （✗ 不留孙进程）。两刀同批。
  ④′ **公告 == 可选**（腿② · 本枚加 · `GAP-20261005-45` 的判据化 · 与 ③ 同门不另立）：
     对**两集无差**的命令，**逐枚**跑 `<bin> <path…> --json <f>`（f 走该命令**登记的每一格**）
     ⇒ 该格**不许**被判「未知字段 <f>」= **登记了却选不出**（红并逐格点名）；且**不许**出现
     「退非 0 却把 f 列在**同一条**的 `合法字段:` 清单里」= **自相矛盾**（同红）。
     ★ 判「被接受」**不**苛求 rc==0：报话**不是**「未知字段 <f>」即当该格过了字段面
       （如 `agent bench` 少位置参数 ⇒ rc=2 但字段面已过 ⇒ 仍算**可选**；苛求 rc==0 会把
       「要活服务 / 要位置参数」的命令错杀成红）。
     ★ 覆盖 = **可断言的格数**（两集无差命令的登记格），受**棘轮**约束（`--min-selectable`
       默认 92）= 「今天能逐枚打靶的格数下限」，**只许上调、永不下调**。
  ⑤ **不给静默**：真身表取不到的命令（前置校验先于字段面 / 要活服务 / 重命令）**逐条点名 + 计数**，
     并受**棘轮**约束（`--min-covered` 默认 29）= 「今天能对拍的下限」，**只许上调、永不下调**。

用法
  python3 scripts/gates/check-fields-face-parity.py [仓库根]
  python3 scripts/gates/check-fields-face-parity.py --self-test   # 成对负控（合成声明/真身）
  python3 scripts/gates/check-fields-face-parity.py --json        # 机器读面

退码：0 全对拍且覆盖 ≥ 棘轮 · 1 有逐名差异 / 有「登记却选不出」/ 有自相矛盾 ·
      2 判不了（缺件 / 制品不在 / 覆盖不够 ⇒ 不许当绿）。
纪律：**只读**（不写仓）· 不联网 · 命令一律 `--json <坏名|登记字段>` 走**用法错**这一格
      （不落任何写路径 —— 腿① 用坏名、腿② 用登记字段，**都**只触字段面校验、不触写路径）。
"""
import argparse
import json
import os
import re
import signal
import subprocess
import sys

CMD_RE = re.compile(r'path:\s*\[\]string\{([^}]*)\}')
PATH_SEG_RE = re.compile(r'"([^"]+)"')
FIELDS_RE = re.compile(r'fields:\s*\[\]string\{([^}]*)\}')
# 拼接变量形态（缺口 `GAP-20260928-159` · 与 `check-cli-contract.py` 的 `FIELDS_IDENT_RE` 同一口径）：
# `fields: <标识符>` ⇒ 跨件解析该标识符的名字集（`devEditFields` 那一族）。
FIELDS_IDENT_RE = re.compile(r'fields:\s*([A-Za-z_][A-Za-z0-9_]*)\s*,')
# `[]string{…}` 里那批字面量（本件用；与上面 PATH_SEG_RE 同形，但只吃某个 group 段）。
STR_LIT_RE = re.compile(r'"((?:[^"\\]|\\.)*)"')
IDENT_RE = re.compile(r'\b([A-Za-z_][A-Za-z0-9_]*)\b')
# 递归展开深度上限（够 `devEditFields` 那一族用；不够就明说「判不了」而不是硬编一个名单）。
IDENT_DEPTH_MAX = 4
# 展开时要跳过的 Go 内建/无意义标识符（避免把 `append` / `string` 这类当变量名递归）。
IDENT_SKIP = frozenset(('append', 'string', 'nil', 'true', 'false'))
MAIN_REL = os.path.join("core", "cmd", "zerg", "main.go")
PKG_REL = os.path.join("core", "cmd", "zerg")
LEGAL_FIELDS_PREFIX = "合法字段:"
# 腿②「公告 == 可选」：一枚字段被判**选不出**时制品逐字打的标记（`zerg: 未知字段 "<f>"`）。
UNKNOWN_FIELD_MARK = "未知字段"
BOGUS = "zzz-nonexistent-zz"
PER_CMD_TIMEOUT = 15
MIN_COVERED_DEFAULT = 29
# ★ 腿②「会起整套门 / 长任务 / 阻塞」⇒ **不打靶**（自伤洞：孤儿门跑占闸 · `GAP-20261002-11` /
#   `-20261003-17`）：这些命令的**登记字段会过字段闸**，随后命令**真干活**；而探针超时只杀**
#   直接子** ⇒ 孙趟门跑成孤儿、继承锁 fd、占闸烧 CPU（实测一跑六棵全量门 · load 63.8 / 16 核）。
#   判据 =「**宁少打不可误跑**」：宁可少打几格靶，也绝不让探针真起整套门。
#   ★ `gate bench`：探针 argv 恒为 `<bin> <path…> --json <field>`、**永不带** `--fast`/`--scope`
#     ⇒ 打到它时必是**默认全量档**（重活）⇒ 整条排除（不做「带 --fast 才排除」那种细分）。
NOT_PROBED_PREFIXES = (
    "gate bench",   # 默认全量档（探针不带 --fast/--scope ⇒ 必是全量）
    "gate run",     # 整套门跑
    "gate wait",    # 有界阻塞档（会挂到一门跑收工）
    "gap reread",   # 长任务（重读全账）
)
NOT_PROBED_REASON = "会起整套门 / 长任务 / 阻塞 ⇒ 不打靶（宁少打不可误跑 · 自伤洞 GAP-20261002-11）"
# ★ 腿② 棘轮（`--min-selectable`）：今天能逐枚打靶的**格数**下限（只许上调、永不下调）。
MIN_SELECTABLE_DEFAULT = 92


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def package_sources(root):
    """`core/cmd/zerg` 包内全部非测试 .go 源（拼接）—— 供 `fields: <标识符>` 跨件解析
    （缺口 `GAP-20260928-159` · 与 `check-cli-contract.py` 的 `package_sources` 同一口径）。"""
    pkg = os.path.join(root, PKG_REL)
    try:
        names = sorted(os.listdir(pkg))
    except OSError:
        return ""
    out = []
    for n in names:
        if n.endswith(".go") and not n.endswith("_test.go"):
            try:
                out.append(read(os.path.join(pkg, n)))
            except OSError:
                pass
    return "\n".join(out)


def _strip_comments(src):
    """剥 `//` 注释（与 `check-newcmd-followup.py` 同口径）——注释里写着 `X = []string{…}` 一类
    示例会被当**真定义**读进来（实测：一句注释 `approveFields = \\[\\]string{…}', src)` 把无关标识符
    `src` 拖进解析链、汇出一大坨假字段名）。"""
    return "\n".join(line.split("//", 1)[0] for line in src.splitlines())


def _ident_defs(pkg_src):
    """字段表标识符的定义表（缺口 `GAP-20260928-159`）⇒ `(lit, refs)`。

    · `lit`  = {标识符: [字面量名单, …]}：`X = []string{…}` 形态，**`…` 允许跨行**
      （`[^{}]*` 吃换行）——★ 这是关键：字段表几乎都是**多行**字面量，只吃「`=` 到行尾」
      会把字面量截断（实测截断后递归会去啃别的标识符、把无关字面量当成字段名）。
    · `refs` = {标识符: [[被引用标识符, …], …]}：`X = append(…)` 形态里**以展开形（`Ident...`）
      引用**的标识符（`append(append([]string{}, A...), B...)` 那一族）——★ 只收**展开形**：
      裸 `Ident` 引用会顺着通用变量名（`src`/`b`/`v` 这类到处都在赋值的名字）汇出一大坨无关字面量。

    ★ 先**剥注释**再扫；只做**两趟**正则（不按标识符逐个重扫全文 —— 后者在 6MB 源上会退化 / 超时）。
    """
    code = _strip_comments(pkg_src)
    lit, refs = {}, {}
    for mt in re.finditer(r'\b([A-Za-z_][A-Za-z0-9_]*)\b\s*=\s*\[\]string\{([^{}]*)\}', code, re.S):
        body = mt.group(2)
        names = STR_LIT_RE.findall(body)
        # ★ 缺口 `GAP-20261005-45`（本枚补）：`[]string{…, <标识符>}` 里的**裸标识符** ——
        #   它多半是「某格字段名」的常量（如 `eggFields` 尾上的 `eggFieldHasProfile`，定义为
        #   `const eggFieldHasProfile = "has_profile"`）。只认引号字面量会把这一格读丢
        #   ⇒ 声明集少一名 ⇒ 与公告集差一名 = **假红**（实测 `egg ls`）。
        #   取法：把引号字面量**挖空**，对剩下来的每个裸标识符去全源里找它贴切的字符串标量定义。
        for ref in IDENT_RE.findall(STR_LIT_RE.sub(" ", body)):
            sm = re.search(r'\b%s\b\s*=\s*"((?:[^"\\]|\\.)*)"' % re.escape(ref), code)
            if sm:
                names.append(sm.group(1))
        lit.setdefault(mt.group(1), []).append(names)
    for mt in re.finditer(r'\b([A-Za-z_][A-Za-z0-9_]*)\b\s*=\s*append\s*\(([^\n]*)', code):
        ident, expr = mt.group(1), mt.group(2)
        ids = [r for r in re.findall(r'\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\.\.', expr) if r != ident]
        if ids:
            refs.setdefault(ident, []).append(ids)
    return lit, refs


def _resolve_ident_names(lit, refs, ident, memo, depth=0):
    """拼接变量的字段表 ⇒ 名字**集**（缺口 `GAP-20260928-159`）。

    取法：`ident` 的 `[]string{…}` 字面量名 ∪ 其引用、且在 `lit`/`refs` 里定义过的标识符的
    **递归展开**（`append` 拼接链那一族）—— 汇成名字集（`compare()` 按集合比、不比序）。
    ★ 用 **set** 收集（不是 list 拼接）：同一子表被多处置用时不会指数膨胀（实测 list 拼接到
    3.8 亿项 / 超时；set 有界）。`memo` 按标识符缓存；`depth` 上限封死环。
    展开不完（标识符没定义 / 深度超限）⇒ 返回已取到的部分（可能为空），由调用方处置。
    """
    if ident in memo:
        return memo[ident]
    if depth > IDENT_DEPTH_MAX:
        return set()
    memo[ident] = set()  # 环保护：解析途中再遇到自己 ⇒ 取空（不递归）
    names = set()
    for ns in lit.get(ident, []):
        names.update(ns)
    for ids in refs.get(ident, []):
        for ref in ids:
            if ref == ident:
                continue
            if ref in lit or ref in refs:
                names |= _resolve_ident_names(lit, refs, ref, memo, depth + 1)
    memo[ident] = names
    return names


def declared_faces(root):
    """main.go 命令树 → [(path, [声明字段…])]（只列**有**字段表的命令）。

    ★ 字段表两形态：字面量 `fields: []string{…}` · 拼接变量 `fields: <标识符>`
      （缺口 `GAP-20260928-159`）——后者跨件解析（见 `_resolve_ident_names`）。
    """
    try:
        src = read(os.path.join(root, MAIN_REL))
    except OSError:
        return None
    pkg_src = package_sources(root)
    ident_lit, ident_refs = _ident_defs(pkg_src)
    ident_memo = {}
    try:
        start = src.index("commands = []*command{")
        end = src.index("\n\t}\n", start)
    except ValueError:
        return None
    seg = src[start:end]
    out, ms = [], list(CMD_RE.finditer(seg))
    for i, m in enumerate(ms):
        parts = PATH_SEG_RE.findall(m.group(1))
        if not parts:
            continue
        nxt = ms[i + 1].start() if i + 1 < len(ms) else len(seg)
        window = seg[m.end():nxt]
        fm = FIELDS_RE.search(window)
        if fm:
            names = [s.strip() for s in PATH_SEG_RE.findall(fm.group(1))]
        else:
            im = FIELDS_IDENT_RE.search(window)
            got = _resolve_ident_names(ident_lit, ident_refs, im.group(1), ident_memo) if im else set()
            # 标识符解析不到名字（没定义 / 展开为空）⇒ 本门不把它当「有字段面」条目
            # （既不给空名单去对拍、也不静默当成「无字段表」⇒ 与解析前同形，不出假红）。
            names = sorted(got) or None
        if names is not None:
            out.append((" ".join(parts), sorted(set(names))))
    return out


def _kill_probe_group(proc, sig):
    """杀**整棵**探针子树（自伤洞：孤儿门跑占闸 · `GAP-20261002-11`）。

    `start_new_session=True` ⇒ 子进程自成会话/进程组、`pgid == proc.pid` ⇒ `killpg` 一次收
    **整组**（✗ 只 `proc.kill()` 杀直接子 ⇒ 孙趟门跑 reparent 到 pid 1 继续跑、还继承锁 fd）。
    """
    try:
        os.killpg(proc.pid, sig)
    except (ProcessLookupError, PermissionError, OSError):
        pass


def _probe_run(argv, timeout):
    """起一枚探针子进程 ⇒ (rc | None, stdout, stderr, timed_out, err_msg)。（自伤洞两刀之一 · 纵深）

    ★ 纵深刀：`start_new_session=True`（= `os.setsid` ⇒ 独立进程组）＋ 超时后 `os.killpg` 收
    **整棵**子树（先 TERM、宽限 2s 再 KILL；✗ 不用 `subprocess.run(timeout=)` —— 它只杀**直接子**，
    孙进程会成孤儿并继承父趟锁 fd）。
    ★ 治本刀在**调用方**：`probe_excluded()` 把「会起整套门」的命令挡在打靶面外 —— 两刀同批。
    """
    try:
        proc = subprocess.Popen(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                stdin=subprocess.DEVNULL, text=True, start_new_session=True)
    except OSError as e:
        return None, "", "", False, "跑不起来：%s" % e
    try:
        out, err = proc.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        _kill_probe_group(proc, signal.SIGTERM)
        try:
            out, err = proc.communicate(timeout=2)
        except subprocess.TimeoutExpired:
            _kill_probe_group(proc, signal.SIGKILL)
            try:
                out, err = proc.communicate(timeout=5)
            except subprocess.TimeoutExpired:
                out, err = "", ""   # 兜底：整组拒不退出（几乎不可能；已 KILL 过）
        return None, out or "", err or "", True, ""
    return proc.returncode, out or "", err or "", False, ""


def runtime_face(binpath, path, timeout=PER_CMD_TIMEOUT):
    """现跑一条命令的**真身**字段表 → (字段列表 | None, 原因)。

    只走 `--json <坏名>` 这一格（字段面**执行前判** ⇒ 坏名被当场拒、重活一步不起）：
    rc=2 且 stderr 有「合法字段: …」⇒ 真身拿到。其它 rc / 无表 ⇒ 给原因（**逐条点名，不许静默**）。
    """
    argv = [binpath] + path.split() + ["--json", BOGUS]
    rc, _out, err, timed_out, err_msg = _probe_run(argv, timeout)
    if err_msg:
        return None, err_msg
    if timed_out:
        return None, "超时（>{0}s ⇒ 不算对拍面）".format(timeout)
    for line in err.splitlines():
        if line.strip().startswith(LEGAL_FIELDS_PREFIX):
            return [s.strip() for s in line.split(":", 1)[1].split(",") if s.strip()], ""
    return None, "无字段表（rc=%d ⇒ 前置校验先于字段面 / 要活服务）" % rc


def legal_fields_of(err):
    """stderr 里 `合法字段: a,b,c` ⇒ 真身闭集（没有这一行 ⇒ None）。"""
    for line in err.splitlines():
        if line.strip().startswith(LEGAL_FIELDS_PREFIX):
            return [s.strip() for s in line.split(":", 1)[1].split(",") if s.strip()]
    return None


def classify_field(rc, err, field):
    """腿②「公告 == 可选」：一枚 `--json <field>` 读数 ⇒ 三档之一。

    · `ok`            —— 字段**被接受**：rc==0，或退非 0 但报话**不是**「未知字段 <field>」
                        （= 字段面已过、卡在**别的**档：要位置参数 / 要活服务 …）。
    · `rejected`      —— 报话逐字「未知字段 <field>」= **登记了却选不出**（腿② 必红）。
    · `contradiction` —— 同 `rejected`，**且** field 又出现在**同一条**的 `合法字段:` 清单里
                        = **自相矛盾**（`GAP-20261005-45` 的原症状 · 必红）。
    """
    legal = legal_fields_of(err)
    unknown = ('%s "%s"' % (UNKNOWN_FIELD_MARK, field)) in err
    if not unknown:
        return "ok"
    if legal is not None and field in legal:
        return "contradiction"
    return "rejected"


def probe_excluded(path):
    """腿② 不打靶白名单（自伤洞：孤儿门跑占闸 · `GAP-20261002-11` / `-20261003-17`）。

    True ⇒ 该命令**一格不打靶**（记 `not_probed` + 为何），✗ 不让探针把它真跑起来。
    """
    return any(path == p or path.startswith(p + " ") for p in NOT_PROBED_PREFIXES)


def selectable_probe(binpath, path, field, timeout=PER_CMD_TIMEOUT):
    """现跑一枚 `--json <field>`（f 走该命令**登记的**一格）⇒ (档 | None, 原因)。

    只走**字段面**那一格（登记的字段名 ⇒ 字段面过闸后命令**可能真干活** ⇒ 调用方须先用
    `probe_excluded()` 把「会起整套门 / 长任务 / 阻塞」的命令挡在**打靶面外**）。超时 / 跑不起来 ⇒ 档 = None。
    """
    argv = [binpath] + path.split() + ["--json", field]
    rc, _out, err, timed_out, err_msg = _probe_run(argv, timeout)
    if err_msg:
        return None, err_msg
    if timed_out:
        return None, "超时（>{0}s ⇒ 不算打靶格）".format(timeout)
    return classify_field(rc, err, field), ""


def compare(decl, real):
    """→ (extra, missing)：声明多出来的 / 真身多出来的（**逐名**）。"""
    ds, rs = set(decl), set(real)
    return sorted(ds - rs), sorted(rs - ds)


def run(root, binpath, min_covered, timeout, min_selectable=MIN_SELECTABLE_DEFAULT):
    decl = declared_faces(root)
    if decl is None:
        return 2, {"error": "读不到命令树（%s）⇒ 判不了" % MAIN_REL}
    if not os.path.exists(binpath):
        return 2, {"error": "缺制品 %s ⇒ 判不了（不许拿声明自己跟自己比）" % binpath}
    ok, mism, nope, covered = [], [], [], 0
    sel_ok, sel_rejected, sel_contra, sel_unreach, sel_covered = [], [], [], [], 0
    sel_notprobed = []
    for path, fields in decl:
        real, why = runtime_face(binpath, path, timeout)
        if real is None:
            # 真身表取不到 ⇒ 字段面这一格**够不着** ⇒ 腿② 这一条整条不打靶（已在 `nope` 逐条点名）。
            nope.append({"path": path, "reason": why})
            continue
        covered += 1
        extra, missing = compare(fields, real)
        if extra or missing:
            mism.append({"path": path, "declared_extra": extra, "runtime_extra": missing})
        else:
            ok.append(path)
        # ★ 腿② 不打靶白名单（自伤洞两刀之一 · 治本）：会起整套门的命令**一格不打靶** ⇒ 记 `not_probed`。
        if probe_excluded(path):
            for f in fields:
                sel_notprobed.append({"path": path, "field": f, "reason": NOT_PROBED_REASON})
            continue
        # ── 腿②「公告 == 可选」（本枚加）：对**两集无差**的命令逐枚跑登记的每一格 ──────────
        good = True
        for f in fields:
            verdict, w2 = selectable_probe(binpath, path, f, timeout)
            if verdict is None:
                sel_unreach.append({"path": path, "field": f, "reason": w2})
                good = False
                continue
            sel_covered += 1
            if verdict == "ok":
                continue
            good = False
            rec = {"path": path, "field": f}
            (sel_contra if verdict == "contradiction" else sel_rejected).append(rec)
        if good and fields:
            sel_ok.append(path)
    rc = 0
    if mism or sel_rejected or sel_contra:
        rc = 1
    elif covered < min_covered or sel_covered < min_selectable:
        rc = 2  # 覆盖不够 = 空转（「配置没了所以全绿」那一类假绿）
    return rc, {"declared": len(decl), "covered": covered, "min_covered": min_covered,
                "matched": ok, "mismatched": mism, "not_comparable": nope,
                "selectable_covered": sel_covered, "min_selectable": min_selectable,
                "selectable_matched": sel_ok, "selectable_rejected": sel_rejected,
                "selectable_contradiction": sel_contra, "selectable_unreachable": sel_unreach,
                "selectable_not_probed": sel_notprobed}


def selftest():
    ok = True

    def chk(name, cond, why=""):
        nonlocal ok
        print(("   ✓ " if cond else "   ✗ ") + name + (("（%s）" % why) if (why and not cond) else ""))
        ok = ok and bool(cond)

    # 正控：逐名相等 ⇒ 无差
    chk("正控：声明 == 真身 ⇒ 无差", compare(["a", "b"], ["a", "b"]) == ([], []))
    # 负控①：声明多一名 ⇒ 必须点名
    e, m = compare(["a", "b", "c"], ["a", "b"])
    chk("负控①：声明多一名 ⇒ 点名 extra=['c']", e == ["c"] and m == [], str((e, m)))
    # 负控②：声明少一名 ⇒ 必须点名
    e, m = compare(["a"], ["a", "b"])
    chk("负控②：声明少一名 ⇒ 点名 runtime_extra=['b']", e == [] and m == ["b"], str((e, m)))
    # 负控③：抄错一名（同数不同名）⇒ 两边都点名（**同数不是对拍**）
    e, m = compare(["a", "z"], ["a", "b"])
    chk("负控③：同数不同名 ⇒ 两边都点名", e == ["z"] and m == ["b"], str((e, m)))
    # 负控④：顺序不同**不算**差（名集对拍，不比序）
    chk("负控④：仅顺序不同 ⇒ 无差（不比序）", compare(["b", "a"], ["a", "b"]) == ([], []))
    # ── 腿②「公告 == 可选」（本枚加）：成对负控（纯函数 · 合成 stderr · 不碰真目标）────────
    # 正控：rc=0 ⇒ 该格被接受
    chk("腿②正控：rc=0 ⇒ ok", classify_field(0, "", "model") == "ok")
    # 负控⑦：退非 0 但报话**不是**「未知字段 <本格>」（字段面已过、卡在别的档）⇒ 仍算可选
    chk("腿②负控⑦：rc=2 报话非「未知字段 model」（卡在位置参数/活服务）⇒ ok",
        classify_field(2, "zerg: `agent bench` 要一台机器名（`zerg agent ls` 看名册）\n", "model") == "ok")
    # 负控⑧：报话逐字「未知字段 model」且合法清单**不含** model ⇒ **登记了却选不出**（必红）
    chk("腿②负控⑧：「未知字段 model」且同消息清单不含 model ⇒ rejected",
        classify_field(2, 'zerg: 未知字段 "model"\n合法字段: host,state\n', "model") == "rejected")
    # ★ 负控⑨（成对 · 造「公告 ≠ 可选」的合成夹具 ⇒ 必红）：退 2 + 「未知字段 model」
    #   **却**把 model 列进**同一条**合法清单 = 自相矛盾（`GAP-20261005-45` 的原症状）。
    chk("腿②负控⑨（公告≠可选 ⇒ 必红）：rc=2「未知字段 model」却把 model 列进同条清单 ⇒ contradiction",
        classify_field(2, 'zerg: 未知字段 "model"\n合法字段: host,model,state\n', "model") == "contradiction")
    # 负控⑩：报话点名的是**别的**字段（不是本格）⇒ 本格不算被拒
    chk("腿②负控⑩：「未知字段 other」不牵累本格 model ⇒ ok",
        classify_field(2, 'zerg: 未知字段 "other"\n合法字段: host,model\n', "model") == "ok")
    # 解析面：合成命令树 ⇒ 抓得到 path + 声明名单（不是 bool）
    import tempfile
    with tempfile.TemporaryDirectory() as d:
        os.makedirs(os.path.join(d, "core", "cmd", "zerg"))
        with open(os.path.join(d, MAIN_REL), "w", encoding="utf-8") as f:
            f.write("\tcommands = []*command{\n"
                    "\t\t{path: []string{\"zz\", \"ls\"}, fields: []string{\"name\", \"verdict\"}},\n"
                    "\t\t{path: []string{\"zz\", \"ed\"}, fields: zzFields, run: x},\n"
                    "\t\t{path: []string{\"zz\", \"cf\"}, fields: zzConst, run: x},\n"
                    "\t}\n")
        # 拼接变量：**多行**字面量 + `append` 链（缺口 GAP-20260928-159 —— 多行是实测踩到的坑）
        with open(os.path.join(d, "core", "cmd", "zerg", "zz_defs.go"), "w", encoding="utf-8") as f:
            f.write("package main\n\n"
                    "var zzA = []string{\n\t\"alpha\", \"beta\",\n}\n\n"
                    "var zzB = []string{\"gamma\",\n\t\"delta\"}\n\n"
                    "var zzNoise = []string{\"--not-a-field\"}\n\n"
                    "var zzFields = append(append([]string{}, zzA...), zzB...)\n\n"
                    # ★ 缺口 `GAP-20261005-45`：`[]string{…, <标识符>}` 里的裸标识符 = 一格字段名常量
                    #   （照 `eggFields` 尾上的 `eggFieldHasProfile` / `const eggFieldHasProfile = "has_profile"`）
                    "var zzConst = []string{\"kappa\", zzConstField}\n\n"
                    "const zzConstField = \"has_profile\"\n")
        got = declared_faces(d)
        chk("解析面：字面量 ⇒ 名单逐字（不是 bool）",
            ("zz ls", ["name", "verdict"]) in got, str(got))
        chk("解析面：拼接变量 `fields: zzFields` ⇒ 跨件展开名集（多行字面量 · 缺口 GAP-20260928-159）",
            ("zz ed", ["alpha", "beta", "delta", "gamma"]) in got, str(got))
        chk("解析面：`[]string{…, <常量标识符>}` 的裸标识符 ⇒ 按字符串标量取值（缺口 GAP-20261005-45）",
            ("zz cf", ["has_profile", "kappa"]) in got, str(got))
        chk("解析面：未被子表引用的 zzNoise 不被误收（不啃无关标识符）",
            all("--not-a-field" not in ns for _, ns in got), str(got))
    # ★★ 腿② 端到端成对负控：合成制品（假 `bin/zerg`）⇒ 真跑 run()，成对判绿/红。
    with tempfile.TemporaryDirectory() as d:
        os.makedirs(os.path.join(d, "core", "cmd", "zerg"))
        with open(os.path.join(d, MAIN_REL), "w", encoding="utf-8") as f:
            f.write("\tcommands = []*command{\n"
                    "\t\t{path: []string{\"zz\", \"ffp\"}, fields: []string{\"model\", \"host\"}},\n\t}\n")

        def _fake_bin(body_lines):
            p = os.path.join(d, "fakerz")
            with open(p, "w", encoding="utf-8") as fh:
                fh.write("#!/bin/sh\n" + "\n".join(body_lines) + "\n")
            os.chmod(p, 0o755)
            return p

        # 绿档：坏名报「未知字段 + 清单」；登记的两格都退 0 ⇒ 声明==公告 且 公告==可选
        green = _fake_bin([
            'f=""',
            'for a in "$@"; do f="$a"; done',
            'if [ "$f" = "' + BOGUS + '" ]; then',
            '  printf \'%s\\n\' \'zerg: ' + UNKNOWN_FIELD_MARK + ' "' + BOGUS + '"\' \'' + LEGAL_FIELDS_PREFIX + ' host,model\' >&2',
            '  exit 2',
            'fi',
            'exit 0',
        ])
        rc_g, _ = run(d, green, 1, 2, 1)
        chk("腿②端到端正控：假制品全接受 ⇒ run() rc=0", rc_g == 0, "rc=%d" % rc_g)
        # ★ 红档（成对）：假制品对登记格 `model` 打「未知字段 model」**却**把 model 列进同条清单
        #   = 「公告 ≠ 可选」自相矛盾 ⇒ run() 必红（rc=1）
        red = _fake_bin([
            'f=""',
            'for a in "$@"; do f="$a"; done',
            'printf \'%s\\n\' \'zerg: ' + UNKNOWN_FIELD_MARK + ' "\'"$f"\'"\' \'' + LEGAL_FIELDS_PREFIX + ' host,model\' >&2',
            'exit 2',
        ])
        rc_r, res_r = run(d, red, 1, 2, 1)
        chk("腿②端到端负控（公告≠可选 ⇒ 必红）：run() rc=1 且逐格点名",
            rc_r == 1 and {"path": "zz ffp", "field": "model"} in res_r.get("selectable_contradiction", []),
            "rc=%d res=%s" % (rc_r, str(res_r.get("selectable_contradiction"))))
    # ★ 腿② 不打靶白名单（自伤洞两刀之一 · 治本）：会起整套门的目标 ⇒ 记 not_probed 且**零执行**（假制品留痕缺席）
    with tempfile.TemporaryDirectory() as d:
        os.makedirs(os.path.join(d, "core", "cmd", "zerg"))
        with open(os.path.join(d, MAIN_REL), "w", encoding="utf-8") as f:
            f.write("\tcommands = []*command{\n"
                    "\t\t{path: []string{\"gate\", \"bench\"}, fields: []string{\"log\", \"rc\"}},\n"
                    "\t\t{path: []string{\"zz\", \"show\"}, fields: []string{\"alpha\"}},\n\t}\n")
        heavy_mark = os.path.join(d, "HEAVY_RAN")
        normal_mark = os.path.join(d, "NORMAL_RAN")
        p = os.path.join(d, "fakerz")
        with open(p, "w", encoding="utf-8") as fh:
            fh.write("#!/bin/sh\ncase \"$*\" in\n"
                     "  'gate bench --json " + BOGUS + "') printf '%s\\n' '" + LEGAL_FIELDS_PREFIX +
                     " log,rc' >&2; exit 2;;\n"
                     "  'zz show --json " + BOGUS + "') printf '%s\\n' '" + LEGAL_FIELDS_PREFIX +
                     " alpha' >&2; exit 2;;\n"
                     "  'gate bench --json '*) : > '" + heavy_mark + "'; exit 0;;\n"
                     "  'zz show --json alpha') : > '" + normal_mark + "'; exit 0;;\n"
                     "  *) exit 0;;\nesac\n")
        os.chmod(p, 0o755)
        rc_np, res_np = run(d, p, 2, 3, 1)
        np_cells = res_np.get("selectable_not_probed", [])
        np_hit = sorted(c["field"] for c in np_cells if c.get("path") == "gate bench")
        chk("腿②不打靶（自伤洞治本）：会起整套门的目标（gate bench）记 not_probed 且**零执行**",
            (not os.path.exists(heavy_mark)) and np_hit == ["log", "rc"],
            "rc=%d np_hit=%s heavy_ran=%s" % (rc_np, str(np_hit), os.path.exists(heavy_mark)))
        chk("腿②不打靶对照：正常命令（zz show）照打靶（假制品真被跑到）",
            os.path.exists(normal_mark) and "zz show" in res_np.get("selectable_matched", []),
            "matched=%s normal_ran=%s" % (str(res_np.get("selectable_matched")),
                                          os.path.exists(normal_mark)))
    # 负控⑥：拼接变量未定义 ⇒ 不当「有字段表」（不出假红、也不静默漏判成空名单）
    with tempfile.TemporaryDirectory() as d:
        os.makedirs(os.path.join(d, "core", "cmd", "zerg"))
        with open(os.path.join(d, MAIN_REL), "w", encoding="utf-8") as f:
            f.write("\tcommands = []*command{\n"
                    "\t\t{path: []string{\"zz\", \"gh\"}, fields: zzGhost, run: x},\n\t}\n")
        chk("负控⑥：标识符未定义 ⇒ 不进声明面",
            declared_faces(d) == [], str(declared_faces(d)))
    # 负控⑤：制品不在 ⇒ rc=2（不许拿声明自己跟自己比）
    rc, _ = run("/nonexistent-zerg-root-zz", "/nonexistent-bin-zz", 1, 2)
    chk("负控⑤：读不到命令树 ⇒ rc=2", rc == 2, "rc=%d" % rc)
    print("── " + ("全过" if ok else "有红") + " ──")
    return 0 if ok else 1


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("root", nargs="?", default=None)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--bin", default=None)
    ap.add_argument("--min-covered", type=int, default=MIN_COVERED_DEFAULT)
    ap.add_argument("--min-selectable", type=int, default=MIN_SELECTABLE_DEFAULT)
    ap.add_argument("--timeout", type=float, default=PER_CMD_TIMEOUT)
    a = ap.parse_args()
    if a.self_test:
        print("── check-fields-face-parity 自检（成对负控 · 合成夹具 · 不碰真目标）──")
        return selftest()
    root = os.path.abspath(a.root) if a.root else os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    binpath = a.bin or os.path.join(root, "bin", "zerg")
    rc, res = run(root, binpath, a.min_covered, a.timeout, a.min_selectable)
    if a.json:
        print(json.dumps({"schema": "zerg/v1", "kind": "FieldsFaceParity", "items": [res],
                          "meta": {"count": 1, "source": "local（本机）"},
                          "warnings": [], "truncated": False}, ensure_ascii=False))
        return rc
    if "error" in res:
        print("✗ " + res["error"])
        return rc
    print("== 命令级 `fields` ⟷ 面级 `--json` 键集 对拍（缺口 `Q-178`）· 公告 == 可选（`GAP-20261005-45`）==")
    print("   声明侧（%s）：%d 条带字段表的命令" % (MAIN_REL, res["declared"]))
    print("   真身侧（现跑 `--json %s` ⇒ `合法字段:` 表）：可对拍 %d 条 · 不可对拍 %d 条"
          % (BOGUS, res["covered"], len(res["not_comparable"])))
    print("   可选侧（逐枚跑登记字段 `--json <f>` · %d 条命令可断言）：可打靶 %d 格 · 不打靶 %d 格 · 棘轮 %d"
          % (res["covered"], res["selectable_covered"], len(res["selectable_not_probed"]),
             res["min_selectable"]))
    for x in res["not_comparable"]:
        print("   ○ 不给静默：%s —— %s" % (x["path"], x["reason"]))
    if res["mismatched"]:
        for x in res["mismatched"]:
            print("   ✗ %s：声明多 %s · 真身多 %s" % (x["path"], x["declared_extra"], x["runtime_extra"]))
    for x in res["selectable_rejected"]:
        print("   ✗ 登记却选不出：%s --json %s" % (x["path"], x["field"]))
    for x in res["selectable_contradiction"]:
        print("   ✗ 自相矛盾（同条清单列了却被判未知）：%s --json %s" % (x["path"], x["field"]))
    for x in res["selectable_unreachable"]:
        print("   ○ 打靶格够不着：%s --json %s —— %s" % (x["path"], x["field"], x["reason"]))
    for x in res["selectable_not_probed"]:
        print("   ○ 不打靶（会起整套门 / 长任务）：%s --json %s —— %s" % (x["path"], x["field"], x["reason"]))
    print("== 结论 ==")
    if res["mismatched"]:
        print("   ⇒ 红（rc=1）：声明集 ⟷ 真身集有逐名差异（%d 条）" % len(res["mismatched"]))
    elif res["selectable_contradiction"]:
        print("   ⇒ 红（rc=1）：有「公告 ≠ 可选」的自相矛盾（%d 格）" % len(res["selectable_contradiction"]))
    elif res["selectable_rejected"]:
        print("   ⇒ 红（rc=1）：有「登记却选不出」的格（%d 格）" % len(res["selectable_rejected"]))
    elif res["covered"] < res["min_covered"]:
        print("   ⇒ 判不了（rc=2）：可对拍 %d < 棘轮 %d（**只许上调** —— 覆盖掉了就是空转）"
              % (res["covered"], res["min_covered"]))
    elif res["selectable_covered"] < res["min_selectable"]:
        print("   ⇒ 判不了（rc=2）：可打靶 %d 格 < 棘轮 %d（**只许上调** —— 覆盖掉了就是空转）"
              % (res["selectable_covered"], res["min_selectable"]))
    else:
        print("   ⇒ 绿（rc=0）：%d 条逐名相等 · 可对拍 %d ≥ 棘轮 %d ｜ 公告 == 可选：可打靶 %d 格 ≥ 棘轮 %d"
              % (len(res["matched"]), res["covered"], res["min_covered"],
                 res["selectable_covered"], res["min_selectable"]))
    return rc


if __name__ == "__main__":
    sys.exit(main())
