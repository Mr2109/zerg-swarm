#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-nodes-roster.py — 机器名册的**唯一真源核对**（契约 §九 M13 `Z2` / `P-069`）

为什么存在
----------
「名册有三处并存、一个真源都没有」（`调研-M13` §4.1 实测）：`gateway/fleet.yaml` 的 `fleet:`
（子端寻址）· 同文件的 `updates.nodes`（换件 ssh 目标）· 子端的机器名旗标（子端自报）。
三处各自更新 ⇒ 任何「按名找到一台机器」的行为都建立在一张会漂的表上。

定案（契约 §九 `Z2`）**名册真源唯一** = `~/.zerg/contexts/<档>.yaml` 的 `nodes:`；
`fleet:` 与 `updates.nodes` 降为**投影源** —— 处置只给了两条路：① **由名册生成**，或
② **被门禁逐项核对**。本门走 ②（①要动主控的配置加载面，属写面，不在本批）。

⇒ **建而不收 = 又多一处**：只新建名册文件、不把三处旧来源绑到它，名册就只是第 4 份拷贝。
本门就是那条「绑」——它同时判两件事：
  ① 两段投影**有没有声明自己在投影**（段内必须写明真源路径）——没有声明 ⇒ rc=1（这才是「收」）；
  ② 声明了之后**值面是否逐字相同**（host/port/os/ssh/root/prefix/api/components 逐字段）。

判据（A–G；A–F 与 G 的「闭集 / reported」支逐条能独立变红，G 的「true 须带 64hex 锚」支**降级为只报告**）
--------------------------
  A. 名册可读且 `nodes:` 非空（否则 rc=2 不给结论 —— 真源不在，判不了）
  B. `gateway/fleet.yaml` 的 `fleet:` 段与 `updates:` 段**各自**带投影源声明（段内出现
     `~/.zerg/contexts/`）——「建而不收」的直接判据
  C. `fleet:` 键集 == 名册键集（**两向**）；每台 host/port/os 与名册逐字相同
  D. `updates.nodes[]`：name ∈ 名册；ssh/root/prefix/api/components 与名册逐字相同
  E. `models:` 段里每个 host ∈ 名册键（模型驻留在哪台 —— 第 4 处，顺带核）
  F. **本机**子端机器名旗标 ∈ 名册键：**本机盘上 plist**（`deploy/com.zerg.agent-Mr2109.plist`）
     与**本机在跑的 `zerg-agentd`**（`ps` 读，只读）两处都核；没有在跑的进程 ⇒ 打印跳过（不是红）。
     ★ 读面 = 本机（local）—— 远端子端本机读不到，见「结论范围」。
  G. `models:` 段里每条的 `verified` 是**闭集值**，「实测级」带**可核的内容锚**为**报告项**：
       · `verified: true`（实测级）—— **必须**带 `sha256:`/`digest:`/`sha:` 之一，且值 = 64 位
         十六进制；缺锚 / 锚格式不对 ⇒ **降级为只报告**（逐条列名，**不进 rc**）——
         GAP-20261003-21：`verified` 的消费者 schema 是 bool ⇒ 该锚恒缺、原「rc=1 点名」恒红且修不了。
         给了 `--verify-sha` 且该条 host 是本机、`file:` 在盘上 ⇒ 现算 sha256 逐字比对
         （内容寻址真核）；本机读不到（远端 host / 件不在 / 未开开关）⇒ 该锚记为**声明级**。
       · `verified: reported`（声明级）—— 必须带 `verified_where:`（∈ 名册键），表示「由哪台自报」。
       · `verified: false` ⇒ 放过（就是「没实测」）；闭集外的取值 ⇒ rc=1 点名。
       · 没写 `verified` 字段的条目 ⇒ 本判据不管（E 已核它的 host 在册）。
  H. **拨号地址（`--controller`）由名册可推导**（`GAP-20260927-03`）：仓内每个子端 plist 的
     `--controller` 值（`${VAR:-默认}` 变量形态取**默认值**）必须**能由名册推出来** ——
       host 必须 ∈ 可推导集 = { 回环 `127.0.0.1`/`::1`/`localhost` } ∪ { `nodes:` 每台的 host }；
       端口必须 == 名册顶层 `core:` 的端口。
     host 落在名册**没有的网段**（如 `192.168.110.x`）或端口不符 ⇒ **rc=1**，**逐字点名 plist 路径与该值**
     （换网/改 IP 时 plist 与名册各说各话的那种漂移）。plist 不在 / 无 `--controller` ⇒ 打印跳过（不是红）。

结论范围（read_layer × verdict_scope —— 本门**只用本机读数**，故判词必须带层限）
------------------------------------------------------------------------------
  本门的全部读取面都是**本机**：本机盘上的 `gateway/fleet.yaml`、本机 `~/.zerg/contexts/`
  名册、本机 `deploy/…plist`、本机 `ps -Ao command=`（`read_layer=local`）。⇒ 判词一律
  `verdict_scope=本机`，并**逐台列出本门读不到、因而不给结论的远端节点**；★ 严禁把本机
  读数冒称「机群级/全对」。远端节点的一致性须由**各机自己**跑本门回答（或该机自报 +
  `verified: reported` + `verified_where`）。

退码（与仓内既有门一致的三档）
------------------------------
  0 = **本机层**全对（PASS · verdict_scope=本机）—— ★ 不表示远端节点也对（见「结论范围」）
  1 = 有漂移（FAIL）—— 逐条给「哪一处 · 哪个字段 · 现值 vs 名册值」
  2 = **不给结论**（缺件 / 解析不出 `nodes:` / `fleet.yaml` 不在 / 自检不过 / 空转假覆盖）

★ 本机物件：名册住在 `~/.zerg/`（**不入任何仓**）⇒ 在没建名册的机器上本门**必然** rc=2
  （「名册不在、判不了」是事实，不是「绿」）。所以它**不进提交闸的快速档**（`FAST_STEPS`）：
  否则每台没建名册的机器**每次提交**都会被拦一格 —— 那不叫严格，是把闸废掉（同 `check-wired-scripts`
  升档时的先例）。它挂在**默认（全量）档** 的 `gates` scope 里。

用法
----
    python3 scripts/gates/check-nodes-roster.py                 # 核真目标（仓内 fleet.yaml + 本机名册）
    python3 scripts/gates/check-nodes-roster.py --roster P      # 换名册路径（默认 ~/.zerg/contexts/default.yaml）
    python3 scripts/gates/check-nodes-roster.py --list-rules    # 打判据 / 路径来源 / 退码
    python3 scripts/gates/check-nodes-roster.py --self-test     # 只跑自检（合成夹具，不碰真目标）
    python3 scripts/gates/check-nodes-roster.py --verify-sha    # 另对「本机可读的 verified: true」现算 sha256 真核
python3 scripts/gates/check-nodes-roster.py --ctrl-plist P  # 换判据 H「拨号地址对账」要核的 plist（可重复）
"""
import argparse
import contextlib
import hashlib
import io
import os
import re
import subprocess
import sys
import tempfile

# ── 目标路径（都可以被 --roster / --fleet / --plist 覆盖；自检用夹具走这些开关）──
REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
DEFAULT_FLEET = os.path.join(REPO_ROOT, "gateway", "fleet.yaml")
DEFAULT_PLIST = os.path.join(REPO_ROOT, "deploy", "com.zerg.agent-Mr2109.plist")
DEFAULT_ROSTER = os.path.join(os.path.expanduser("~"), ".zerg", "contexts", "default.yaml")

# 名册里参与「逐字比对」的字段（fleet: 段与 updates.nodes 段各自的字段集）
FLEET_FIELDS = ("host", "port", "os")
UPDATE_FIELDS = ("ssh", "root", "prefix", "api", "components")

# ── 判据 G：`models:` 条目 `verified` 的闭集值与内容锚（GAP-20260927-167/-171）──
#   `true`     = 实测级 —— 必须带 64 位十六进制内容锚（sha256/digest/sha）
#   `reported` = 声明级 —— 必须带 verified_where（哪台自报，∈ 名册键）
#   `false`    = 没实测（放过；真值是「未核」，不是「已核」）
VERIFIED_VALUES = ("true", "false", "reported")
ANCHOR_FIELDS = ("sha256", "digest", "sha")
ANCHOR_RE = re.compile(r"^[0-9a-fA-F]{64}$")
# 本机回环寻址：`fleet:` 段里 host 落在这些值上的节点名 = 本机（read_layer=local 的身份源）
LOCAL_HOSTS = ("127.0.0.1", "::1", "localhost")

# ── 判据 H：仓内子端 plist 的拨号地址（`--controller`）**由名册可推导**（GAP-20260927-03）──
#   默认要核的仓内件（可被 --ctrl-plist 覆盖）：两枚子端 plist。
CONTROLLER_PLISTS = (
    os.path.join(REPO_ROOT, "deploy", "com.zerg.agent-Mr2109.plist"),
    os.path.join(REPO_ROOT, "gateway", "deploy", "com.x3gw.agent.mini.plist"),
)
CTRL_VAR_RE = re.compile(r"^\$\{([A-Za-z_][A-Za-z0-9_]*):-(.*)\}$", re.S)   # ${VAR:-默认}
CTRL_URL_RE = re.compile(r"^(https?)://([^/:]+)(?::(\d+))?(/.*)?$")


# ══════════════════════════════════════════════════════════════════════════
# 极简 YAML 子集解析（只服务本门的两种形状：块式映射/列表 + 行内流式映射）
#   ★ 为什么不用 PyYAML：本仓的门禁脚本一律**零第三方依赖**（本机 python3 没有 yaml 模块），
#     而这里要读的形状是固定的（fleet.yaml 与名册）；解析失败会**大声报**（空转 ⇒ rc=2），
#     不会静默漏判 —— 这是零依赖可接受的前提。
# ══════════════════════════════════════════════════════════════════════════
def strip_comment(line):
    """去掉行尾注释：只认「前面是空白或行首」的 `#`，且不在引号/花括号里。"""
    out, quote, depth = [], None, 0
    for i, ch in enumerate(line):
        if quote:
            out.append(ch)
            if ch == quote:
                quote = None
            continue
        if ch in "\"'":
            quote = ch
            out.append(ch)
            continue
        if ch == "{":
            depth += 1
        elif ch == "}":
            depth = max(0, depth - 1)
        if ch == "#" and depth == 0 and (i == 0 or line[i - 1] in " \t"):
            break
        out.append(ch)
    return "".join(out).rstrip()


def split_top(text, sep=","):
    """按 sep 切分，但忽略引号与花括号内部的 sep。"""
    parts, buf, quote, depth = [], [], None, 0
    for ch in text:
        if quote:
            buf.append(ch)
            if ch == quote:
                quote = None
            continue
        if ch in "\"'":
            quote = ch
            buf.append(ch)
            continue
        if ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
        if ch == sep and depth == 0:
            parts.append("".join(buf))
            buf = []
            continue
        buf.append(ch)
    parts.append("".join(buf))
    return [p.strip() for p in parts if p.strip()]


def unquote(v):
    v = v.strip()
    if len(v) >= 2 and v[0] == v[-1] and v[0] in "\"'":
        v = v[1:-1]
    return v


def parse_flow(text):
    """`{ k: v, k: v }` → dict（键值都按字符串，去掉外层引号）。"""
    inner = text.strip()
    if inner.startswith("{"):
        inner = inner[1:]
    if inner.endswith("}"):
        inner = inner[:-1]
    out = {}
    for part in split_top(inner):
        k, _, v = part.partition(":")
        out[unquote(k)] = unquote(v)
    return out


_KV = re.compile(r"^([^:\s#][^:]*):\s*(.*)$")


def parse_block(lines):
    """把一段（已切好、不含父键那一行）解析成 {键: 字符串 | 子字典 | 列表}。"""
    out = {}
    i = 0
    while i < len(lines):
        raw = lines[i]
        if not raw.strip():
            i += 1
            continue
        ind = len(raw) - len(raw.lstrip(" "))
        s = raw.strip()
        if s.startswith("-"):
            i += 1
            continue  # 列表项由 parse_seq 处理；这里跳过（本门不用嵌套列表的映射形态）
        m = _KV.match(s)
        if not m:
            i += 1
            continue
        key, val = unquote(m.group(1)), m.group(2).strip()
        if val == "":
            # 子块：吃到下一个缩进 ≤ ind 的非空行
            j, sub = i + 1, []
            while j < len(lines):
                l2 = lines[j]
                if not l2.strip():
                    j += 1
                    continue
                if len(l2) - len(l2.lstrip(" ")) <= ind:
                    break
                sub.append(l2)
                j += 1
            out[key] = parse_seq(sub) if (sub and sub[0].strip().startswith("-")) else parse_block(sub)
            i = j
            continue
        out[key] = parse_flow(val) if val.startswith("{") else unquote(val)
        i += 1
    return out


def parse_seq(lines):
    """把 `- { k: v }` / `- k: v`（+ 后续更深的 k: v）解析成 [dict, ...]。"""
    items, cur, i = [], None, 0
    while i < len(lines):
        raw = lines[i]
        if not raw.strip():
            i += 1
            continue
        s = raw.strip()
        if s.startswith("-"):
            ind = len(raw) - len(raw.lstrip(" "))
            body = s[1:].strip()
            if cur is not None:
                items.append(cur)
            cur = {}
            if body.startswith("{"):
                cur.update(parse_flow(body))
                i += 1
                continue
            m = _KV.match(body)
            if m:
                k, _, v = m.group(1), None, m.group(2).strip()
                cur[unquote(k)] = parse_flow(v) if v.startswith("{") else unquote(v)
            i += 1
            # 收后续更深缩进的键
            while i < len(lines) and lines[i].strip() and (len(lines[i]) - len(lines[i].lstrip(" "))) > ind:
                m2 = _KV.match(lines[i].strip())
                if m2:
                    k2, v2 = unquote(m2.group(1)), m2.group(2).strip()
                    cur[k2] = parse_flow(v2) if v2.startswith("{") else unquote(v2)
                i += 1
            continue
        i += 1
    if cur is not None:
        items.append(cur)
    return items


def section_range(lines, key):
    """顶层键 key 那一段的半开区间 (start, end)：从 key 行之后到下一个列首非空行为止。"""
    start = None
    for i, l in enumerate(lines):
        if re.match(r"^" + re.escape(key) + r"\s*:", l):
            start = i + 1
            break
    if start is None:
        return None
    end = len(lines)
    for j in range(start, len(lines)):
        if lines[j].strip() and not lines[j].startswith((" ", "\t", "#")):
            end = j
            break
    return (start, end)


def section_lines(lines, key):
    """取顶层键 key 的那一段（已去注释的形态；结构解析用）。"""
    r = section_range(lines, key)
    return None if r is None else lines[r[0]:r[1]]


def read_lines(path):
    """→ (raw, stripped)：raw 保留注释（判「有没有声明投影源」用），stripped 供结构解析。"""
    with open(path, encoding="utf-8") as f:
        raw = f.read().split("\n")
    return raw, [strip_comment(l) for l in raw]


# ══════════════════════════════════════════════════════════════════════════
# 判据
# ══════════════════════════════════════════════════════════════════════════
def load_roster(path):
    """→ (nodes: dict, 错误信息 or None)"""
    if not os.path.isfile(path):
        return None, "名册不在：%s（本机物件；建它 = 决议⑥ 的第 0 步）" % path
    try:
        _raw, lines = read_lines(path)
    except Exception as e:  # pragma: no cover
        return None, "名册读不了：%s" % e
    sec = section_lines(lines, "nodes")
    if sec is None:
        return None, "名册里没有顶层键 nodes:"
    nodes = parse_block(sec)
    nodes = {k: v for k, v in nodes.items() if isinstance(v, dict) and v}
    if not nodes:
        return None, "名册的 nodes: 解析不出任何机器（空转 = 假覆盖）"
    return nodes, None


def check_fleet(fleet_path, nodes, local_names=None, verify_sha=False):
    """→ (fails, checks) —— B/C/D/E/G 五条。"""
    fails, checks = [], []
    if not os.path.isfile(fleet_path):
        return ["fleet.yaml 不在：%s" % fleet_path], checks
    raw, lines = read_lines(fleet_path)

    # B. 两段各自声明「我是投影源」——注意这里看的是 **raw（含注释）**：
    #    声明本身写在注释里，看「去注释后的行」等于永远看不到它（本脚本第一版就踩过这个）
    for key in ("fleet", "updates"):
        rng = section_range(lines, key)
        if rng is None:
            fails.append("fleet.yaml 里没有 %s: 段" % key)
            continue
        has_decl = any("~/.zerg/contexts/" in l for l in raw[rng[0]:rng[1]])
        checks.append("B %s: 段投影源声明：%s" % (key, "有" if has_decl else "**缺**"))
        if not has_decl:
            fails.append(
                "%s: 段没有投影源声明（段内应出现真源路径 ~/.zerg/contexts/…）——「建而不收」就是不写这句"
                % key)

    # C. fleet: 键集与逐字段
    sec = section_lines(lines, "fleet")
    if sec is not None:
        fleet = parse_block(sec)
        fleet = {k: v for k, v in fleet.items() if isinstance(v, dict)}
        only_roster = sorted(set(nodes) - set(fleet))
        only_fleet = sorted(set(fleet) - set(nodes))
        checks.append("C fleet: 键 %d 个 · 名册节点 %d 个" % (len(fleet), len(nodes)))
        for k in only_fleet:
            fails.append("C fleet: 里的 `%s` 不在名册（子端名册没登记它）" % k)
        for k in only_roster:
            fails.append("C 名册里的 `%s` 不在 fleet: 段（名册有、投影没有）" % k)
        for name, ent in sorted(fleet.items()):
            if name not in nodes:
                continue
            for f in FLEET_FIELDS:
                want = str(nodes[name].get(f, "")).strip('"')
                got = str(ent.get(f, "")).strip('"')
                if got != want:
                    fails.append("C `%s` 的 %s 漂了：fleet=%r 名册=%r" % (name, f, got, want))

    # D. updates.nodes 逐字段
    sec = section_lines(lines, "updates")
    if sec is not None:
        ups = parse_block(sec)
        items = ups.get("nodes") or []
        if not isinstance(items, list):
            items = []
        checks.append("D updates.nodes 条目 %d 个" % len(items))
        for it in items:
            name = str(it.get("name", "")).strip('"')
            if not name:
                fails.append("D updates.nodes 有一项没有 name")
                continue
            if name not in nodes:
                fails.append("D updates.nodes 里的 `%s` 不在名册" % name)
                continue
            for f in UPDATE_FIELDS:
                want = str(nodes[name].get(f, "")).strip('"')
                got = str(it.get(f, "")).strip('"')
                if got != want:
                    fails.append("D `%s` 的 %s 漂了：updates.nodes=%r 名册=%r" % (name, f, got, want))

    # E. models: 里的 host 必须在名册里
    sec = section_lines(lines, "models")
    if sec is not None:
        hosts = set()
        for l in sec:
            m = re.search(r"\bhost\s*:\s*([A-Za-z0-9_.\-]+)", l)
            if m:
                hosts.add(m.group(1))
        checks.append("E models: 段出现 %d 个 host" % len(hosts))
        for h in sorted(hosts - set(nodes)):
            fails.append("E models: 里的 host `%s` 不在名册（模型驻留在哪台必须有条目）" % h)

    # G. models: 段的 `verified` 闭集 + 「实测级必须有可核内容锚」
    fg, cg = check_models_verified(lines, nodes, local_names or set(), verify_sha)
    fails += fg
    checks += cg
    return fails, checks


def running_agentd_machine():
    """在跑的 zerg-agentd 的机器名旗标值（没有 ⇒ None）。只读 ps，不碰进程。"""
    try:
        out = subprocess.run(["ps", "-Ao", "command="], capture_output=True, text=True, timeout=15).stdout
    except Exception:
        return None
    for line in out.split("\n"):
        if "zerg-agentd" not in line or "grep" in line:
            continue
        parts = line.split()
        for i, p in enumerate(parts):
            if p.endswith("machine") and i + 1 < len(parts):
                return parts[i + 1]
    return None


def sha256_file(path, chunk=1 << 20):
    """现算件的 sha256（内容寻址真核用；只读）。"""
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            b = f.read(chunk)
            if not b:
                break
            h.update(b)
    return h.hexdigest()


def plist_program_argv(plist_path):
    """→ (tokens, err)：plist 的『逻辑命令行』token 表（只读）。

    ★ 两种形态都收：① 直给 argv 数组 —— 原样返回；② shell 包裹
      （`ProgramArguments = [/bin/sh, -c, "<脚本串>", …]`，照 `deploy_mini.sh` / `zerg-swap-agent.sh`
      的 `${VAR:-默认}` 壳形态）—— 用 shlex 把脚本串切成 token。
    """
    if not os.path.isfile(plist_path):
        return None, None
    try:
        import plistlib
        with open(plist_path, "rb") as f:
            args = plistlib.load(f).get("ProgramArguments") or []
    except Exception as e:
        return None, "plist 读不了（%s）：%s" % (plist_path, e)
    args = [str(a) for a in args]
    if len(args) >= 3 and os.path.basename(args[0]) in ("sh", "bash", "zsh") and args[1] == "-c":
        import shlex
        try:
            return shlex.split(args[2]), None
        except ValueError as e:
            return None, "plist 脚本串切不开（%s）：%s" % (plist_path, e)
    return args, None


def find_flag_value(tokens, flag):
    """命令行 `--flag <值>` / `--flag=<值>` 的取值（找不到 ⇒ None）。"""
    if not tokens:
        return None
    for i, a in enumerate(tokens):
        if a == flag and i + 1 < len(tokens):
            return tokens[i + 1]
        if a.startswith(flag + "="):
            return a[len(flag) + 1:]
    return None


def plist_machine(plist_path):
    """→ (机器名 or None, 错误信息 or None)：读 plist 的 `--machine` 旗标值（只读）。"""
    tokens, err = plist_program_argv(plist_path)
    if err:
        return None, err
    if tokens is None:
        return None, None
    val = find_flag_value(tokens, "--machine")
    if val is None:                       # 兼容旧口径：任何以 machine 收尾的旗标
        for i, a in enumerate(tokens):
            if a.endswith("machine") and i + 1 < len(tokens):
                val = tokens[i + 1]
                break
    return val, None


def plist_controller(plist_path):
    """→ (有效值 or None, 变量名 or None, 错误信息 or None)：读 `--controller` 的**有效值**（只读）。

    `${VAR:-默认}` 变量形态 ⇒ 取**默认值**（= 未设该变量时真正拨出去的地址；照 `zerg-swap-agent.sh`
    的 `${VAR:-默认}` 体例）；直给字面值 ⇒ 原样。
    """
    tokens, err = plist_program_argv(plist_path)
    if err:
        return None, None, err
    if tokens is None:
        return None, None, None
    raw = find_flag_value(tokens, "--controller")
    if raw is None:
        return None, None, None
    raw = raw.strip()
    m = CTRL_VAR_RE.match(raw)
    if m:
        return m.group(2).strip(), m.group(1), None
    return raw, None, None


def derive_local_names(fleet_path, plist_path):
    """→ (本机节点名集合, checks)。

    真源：本门只读本机（read_layer=local）。「本机是哪台」由两处现读合并：
      ① `fleet:` 段里 host 落回环（127.0.0.1 / ::1 / localhost）的节点名；
      ② 本机盘上 plist 的 `--machine` 旗标值。
    两处都读不到 ⇒ 空集（判词如实写「本机身份读不到」，一切远端条目按「本机读不到」处理）。
    """
    names, checks = set(), []
    sec = None
    try:
        _raw, lines = read_lines(fleet_path)
        sec = section_lines(lines, "fleet")
    except Exception:
        sec = None
    if sec is not None:
        for k, v in parse_block(sec).items():
            if isinstance(v, dict) and str(v.get("host", "")).strip('"') in LOCAL_HOSTS:
                names.add(k)
    pm, _perr = plist_machine(plist_path)
    if pm:
        names.add(pm)
    checks.append("结论层（read_layer=local）本机身份 = %s（fleet: 回环 host ∪ 本机 plist 机器名）"
                  % (" · ".join(sorted(names)) if names else "**读不到**"))
    return names, checks


def models_entries(lines):
    """→ [(模型名, 条目dict), ...]：把 `models:` 段展平成条目列表（flow / 块式都收）。

    ★ 只用现成的极简解析器：flow 条目里的 `cmd: [...]`（方括号内逗号不被跟踪）会溢出成
      无害的杂键；本判据只读 `verified` / `sha256|digest|sha` / `host` / `file` / `verified_where`
      这几个**具名标量**，杂键不与之同名（真仓现读核过）。
    """
    sec = section_lines(lines, "models")
    if sec is None:
        return []
    out = []
    for name, v in parse_block(sec).items():
        if isinstance(v, list):
            for e in v:
                if isinstance(e, dict):
                    out.append((name, e))
        elif isinstance(v, dict):
            out.append((name, v))
    return out


def check_models_verified(lines, nodes, local_names, verify_sha=False):
    """G：`models:` 条目的 `verified` 闭集 + 「实测级必须有可核内容锚」（GAP-…-167/-171）。

    ★ 层级纪律：`verified: true` 是**实测级**声明。本门只会**本机**读数 ⇒
        · host 是本机 且 件在盘上 且 开了 --verify-sha ⇒ 现算 sha256 逐字比对（真核）；
        · host 非本机（或件读不到 / 未开开关）⇒ 本机**无法复算**，该锚一律记**声明级**，
          绝不冒称「已核」；远端要「实测级」须由该机自己跑本门回答。
    """
    fails, checks = [], []
    ents = models_entries(lines)
    if not ents:
        checks.append("G models: 段解析不出条目 ⇒ 跳过（E 仍核 host）")
        return fails, checks
    n_true = n_rep = n_ok = 0
    n_nolabel = 0                 # ★ 降级（GAP-20261003-21）：无有效锚的 true —— **只报告 · 不进 rc**
    for name, e in ents:
        raw = str(e.get("verified", "")).strip().strip('"').lower()
        if raw == "":
            continue                      # 没写字段 ⇒ 本判据不管（不是「已核」也不是缺陷）
        host = str(e.get("host", "")).strip('"')
        tag = "%s（host=%s）" % (name, host or "?")
        if raw not in VERIFIED_VALUES:
            fails.append("G `%s` 的 verified 取值 `%s` 不在闭集 %s"
                         % (tag, raw, "/".join(VERIFIED_VALUES)))
            continue
        if raw == "false":
            continue
        if raw == "true":
            n_true += 1
            anchor = ""
            for f in ANCHOR_FIELDS:
                if str(e.get(f, "")).strip().strip('"'):
                    anchor = str(e.get(f, "")).strip().strip('"')
                    break
            if not anchor:
                n_nolabel += 1
                # ★ 降级（GAP-20261003-21）：消费者 schema 是 bool ⇒ 该锚恒缺、原 rc=1 恒红且修不了
                #   ⇒ 改判**只报告**（逐条列名，让「无锚的 true」可见，但不作门红）。
                checks.append("G（报告·降级 GAP-20261003-21）`%s` 写 verified: true（实测级）却**没有内容锚**"
                              "（%s 之一，64 位十六进制）—— 只报告 · 不进 rc"
                              % (tag, "/".join(ANCHOR_FIELDS)))
                continue
            if not ANCHOR_RE.match(anchor):
                n_nolabel += 1
                checks.append("G（报告·降级 GAP-20261003-21）`%s` 的锚不是 64 位十六进制：%r —— 只报告 · 不进 rc"
                              % (tag, anchor))
                continue
            if host in local_names:
                fpath = str(e.get("file", "")).strip().strip('"')
                if not verify_sha:
                    checks.append("G `%s` 锚在且格式对；本机未复算（开 --verify-sha 即可真核）" % tag)
                elif not fpath:
                    checks.append("G `%s` 锚在且格式对；条目无 file: ⇒ 本机无法复算，记**声明级**" % tag)
                elif not os.path.isfile(fpath):
                    checks.append("G `%s` 本机读不到件（%s）⇒ 锚记**声明级**，未复算" % (tag, fpath))
                else:
                    got = sha256_file(fpath)
                    if got != anchor.lower():
                        fails.append("G `%s` 的 sha256 与盘上件不符：条目=%s… 盘上=%s…"
                                     % (tag, anchor.lower()[:12], got[:12]))
                    else:
                        n_ok += 1
                        checks.append("G `%s` 锚**本机现算逐字相符**（%s…）" % (tag, anchor[:12]))
            else:
                checks.append("G `%s` 锚在且格式对；host 非本机 ⇒ 本机**无法复算**，记**声明级**" % tag)
        elif raw == "reported":
            n_rep += 1
            where = str(e.get("verified_where", "")).strip().strip('"')
            if not where:
                fails.append("G `%s` 写 verified: reported（声明级）却缺 verified_where（哪台自报）"
                             % tag)
            elif where not in nodes:
                fails.append("G `%s` 的 verified_where `%s` 不在名册" % (tag, where))
            else:
                checks.append("G `%s` 声明级（自报机=%s）" % (tag, where))
    checks.insert(0, "G models: verified=true %d 条 · reported %d 条 · 本机现算相符 %d 条 · "
                     "**无有效锚的 true %d 处（降级只报告 · 不进 rc）**"
                  % (n_true, n_rep, n_ok, n_nolabel))
    return fails, checks


def check_subend(plist_path, nodes):
    """F：**本机**盘上 plist 与**本机**在跑的 agentd 两处的机器名都要在名册里。

    ★ 读面 = 本机（read_layer=local）：plist 是本机那一只、`ps` 是本机进程表 ⇒
      判词一律带「本机」二字；**远端子端不在本判据的结论范围内**（本机读不到）。
    """
    fails, checks = [], []
    val = None
    if not os.path.isfile(plist_path):
        checks.append("F 本机盘上 plist 不在（%s）⇒ 跳过该处" % plist_path)
    else:
        val, perr = plist_machine(plist_path)
        if perr:
            fails.append("F %s" % perr)
            val = None
    if val is not None:
        checks.append("F 本机盘上 plist 机器名 = %s" % val)
        if val not in nodes:
            fails.append("F plist 的机器名 `%s` 不在名册（改机器名要先改名册）" % val)
    run = running_agentd_machine()
    if run is None:
        checks.append("F 本机在跑的 zerg-agentd：没找到（跳过该处）")
    else:
        checks.append("F 本机在跑的 zerg-agentd 机器名 = %s" % run)
        if run not in nodes:
            fails.append("F 本机在跑的 zerg-agentd 机器名 `%s` 不在名册（子端自报和真源对不上）" % run)
    return fails, checks


def roster_core_port(lines):
    """名册顶层 `core:` 的端口（读不到 ⇒ None）。"""
    for l in lines:
        m = re.match(r"^core\s*:\s*(\S+)\s*$", l)
        if m:
            u = unquote(m.group(1))
            mm = re.search(r":(\d+)\s*$", u)
            return int(mm.group(1)) if mm else None
    return None


def derivable_controller_hosts(nodes):
    """名册能产出的拨号 host 集：回环 ∪ `nodes:` 里每台的 host。"""
    hosts = set(LOCAL_HOSTS)
    for ent in nodes.values():
        if isinstance(ent, dict):
            h = str(ent.get("host", "")).strip().strip('"')
            if h:
                hosts.add(h)
    return hosts


def check_controller(plist_paths, nodes, roster_lines):
    """H：`--controller` **由名册可推导**（host ∈ 可推导集 · port == 名册 core 端口）。

    推不出来 ⇒ 判红，**逐字点名 plist 路径与该值**（GAP-20260927-03）。
    """
    fails, checks = [], []
    core_port = roster_core_port(roster_lines)
    hosts = derivable_controller_hosts(nodes)
    checks.append("H 拨号可推导集（回环 ∪ 名册 host）= %s · 名册 core 端口 = %s"
                  % (" · ".join(sorted(hosts)), core_port if core_port is not None else "**读不到**"))
    for p in plist_paths:
        if not os.path.isfile(p):
            checks.append("H 拨号对账：plist 不在（%s）⇒ 跳过该件" % p)
            continue
        val, var, err = plist_controller(p)
        if err:
            fails.append("H %s" % err)
            continue
        if val is None:
            checks.append("H 拨号对账：%s 无 `--controller` ⇒ 跳过" % p)
            continue
        shown = "%s（变量 ${%s:-…} 的默认值）" % (p, var) if var else p
        m = CTRL_URL_RE.match(val)
        if not m:
            fails.append("H %s 的 `--controller` 不是合法 http(s) 地址：%r" % (shown, val))
            continue
        host, port = m.group(2), m.group(3)
        why = []
        if host not in hosts:
            why.append("host=%s 不在名册可推导集" % host)
        if core_port is not None:
            if port is None:
                why.append("缺端口（名册 core 端口=%d）" % core_port)
            elif int(port) != core_port:
                why.append("port=%s ≠ 名册 core 端口=%d" % (port, core_port))
        if why:
            fails.append("H %s 的拨号地址推不出来：现值 %r —— %s（改 plist 或先把该地址登记进名册 nodes）"
                         % (shown, val, "；".join(why)))
        else:
            checks.append("H 拨号对账 %s ⇒ 可推导（值 %r）" % (shown, val))
    return fails, checks


def run_checks(roster, fleet, plist, verbose=True, verify_sha=False,
               ctrl_plists=None, ctrl_check=True):
    nodes, err = load_roster(roster)
    if err:
        print("⛔ 不给结论：%s" % err)
        return 2
    assert nodes is not None
    print("真源名册 = %s（节点 %d 个：%s）" % (roster, len(nodes), " · ".join(sorted(nodes))))
    local_names, lchecks = derive_local_names(fleet, plist)
    fails, checks = check_fleet(fleet, nodes, local_names, verify_sha)
    f2, c2 = check_subend(plist, nodes)
    fails += f2
    # H 拨号地址（`--controller`）由名册可推导（GAP-20260927-03）—— 负控可哑化（ctrl_check=False）
    if ctrl_check:
        try:
            _raw_r, roster_lines = read_lines(roster)
        except Exception:
            roster_lines = []
        plist_list = list(ctrl_plists) if ctrl_plists is not None else list(CONTROLLER_PLISTS)
        f3, c3 = check_controller(plist_list, nodes, roster_lines)
        fails += f3
    else:
        c3 = ["H 拨号对账：断言哑化（ctrl_check=False · 负控用）⇒ 不报"]
    checks = lchecks + checks + c2 + c3
    uncovered = sorted(set(nodes) - local_names)
    local_str = " · ".join(sorted(local_names)) if local_names else "**本机身份读不到**"
    uncov_str = " · ".join(uncovered) if uncovered else "无"
    if verbose:
        for c in checks:
            print("   · " + c)
        print("   ↳ read_layer=local · verdict_scope=**本机**（%s）—— 本门只读本机盘上的"
              "fleet.yaml / 名册 / plist / ps，**不用任何远端读数**" % local_str)
        print("   ↳ 本门**不覆盖**远端节点（本机读不到 ⇒ 本门不给结论）：%s"
              "（要它们的结论，须由各机自己跑本门回答）" % uncov_str)
    if fails:
        print("✗ **本机层**与名册有漂移：%d 处（verdict_scope=本机；远端不在结论内）" % len(fails))
        for f in fails:
            print("   ✗ " + f)
        return 1
    print("✓ **本机层**（read_layer=local）与名册逐字一致：fleet: · updates.nodes · "
          "models: host 各在册 · G verified 闭集（无锚 true 仅报告·GAP-20261003-21） · "
          "本机子端名册（plist/本机在跑 agentd） · H 拨号地址由名册可推导")
    print("   ↳ verdict_scope=**本机**（%s）—— 本门**不覆盖**远端：%s"
          % (local_str, uncov_str))
    return 0


# ══════════════════════════════════════════════════════════════════════════
# 自检（合成夹具 · 成对负控 · 不碰真目标）
# ══════════════════════════════════════════════════════════════════════════
ROSTER_FIX = """# 夹具名册
context: default
core: http://127.0.0.1:8580
nodes:
  edge-node:
    host: <worker-ip>
    port: 8100
    os: ubuntu
    ssh: g01@<worker-ip>
    root: /home/<user>/zerg
    prefix: /home/<user>/zerg/bin
    api: "http://127.0.0.1:8580"
    components: "core,agentd"
  Mr2109:
    host: 127.0.0.1
    port: 8100
    os: macos
"""

FLEET_FIX = """# 夹具 fleet.yaml
updates:
  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:
  check: true
  nodes:
    - { name: edge-node, ssh: g01@<worker-ip>, root: /home/<user>/zerg, prefix: /home/<user>/zerg/bin, api: "http://127.0.0.1:8580", components: "core,agentd" }
models:
  m1: { host: edge-node, backend: llama-server }
fleet:
  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:
  edge-node: { host: <worker-ip>, port: 8100, os: ubuntu }
  Mr2109: { host: 127.0.0.1, port: 8100, os: macos }
"""

PLIST_FIX = """<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.zerg.agent-Mr2109</string>
  <key>ProgramArguments</key><array>
    <string>/tmp/zerg-agentd</string>
    <string>--machine</string><string>Mr2109</string>
  </array>
</dict></plist>
"""

# 判据 H 夹具（拨号地址 · `__CTRL__` 由 self_test 代入）—— 变量形态（shell 包裹）与字面形态各一：
PLIST_CTRL_VAR_FIX = """<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.x3gw.agent</string>
  <key>ProgramArguments</key><array>
    <string>/bin/sh</string>
    <string>-c</string>
    <string>exec /usr/bin/python3 ~/agent/agent.py --controller "${ZERG_AGENT_CONTROLLER:-__CTRL__}" --machine mini-node-1</string>
  </array>
</dict></plist>
"""

PLIST_CTRL_LITERAL_FIX = """<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.x3gw.agent</string>
  <key>ProgramArguments</key><array>
    <string>/usr/bin/python3</string><string>~/agent/agent.py</string>
    <string>--controller</string><string>__CTRL__</string>
    <string>--machine</string><string>mini-node-1</string>
  </array>
</dict></plist>
"""

# 判据 G 夹具（`__HOST__` / `__FILE__` / `__VERIFIED__` 由 self_test 逐格代入）：
#   models: 段**单条** + 两段投影源声明齐 + fleet: 有本机(Mr2109 回环)与远端(edge-node) ⇒ 只让 G 变。
FLEET_VERIFIED_FIX = """# 夹具 fleet.yaml（判据 G）
updates:
  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:
  check: true
  nodes:
    - { name: edge-node, ssh: g01@<worker-ip>, root: /home/<user>/zerg, prefix: /home/<user>/zerg/bin, api: "http://127.0.0.1:8580", components: "core,agentd" }
models:
  mG:
    - { host: __HOST__, backend: llama-server, file: __FILE__, __VERIFIED__ }
fleet:
  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:
  edge-node: { host: <worker-ip>, port: 8100, os: ubuntu }
  Mr2109: { host: 127.0.0.1, port: 8100, os: macos }
"""


def self_test():
    """成对负控：每个夹具都断言**期望退码**，且成对（改一处 ⇒ 必红）。"""
    cases = []
    with tempfile.TemporaryDirectory() as d:
        def w(name, text):
            p = os.path.join(d, name)
            with open(p, "w", encoding="utf-8") as f:
                f.write(text)
            return p

        roster = w("roster.yaml", ROSTER_FIX)
        fleet = w("fleet.yaml", FLEET_FIX)
        plist = w("a.plist", PLIST_FIX)

        cases.append(("全对 ⇒ 0", roster, fleet, plist, 0))
        cases.append(("fleet: 少一台 ⇒ 1", roster, w("f2.yaml", FLEET_FIX.replace(
            "  Mr2109: { host: 127.0.0.1, port: 8100, os: macos }\n", "")), plist, 1))
        cases.append(("fleet: host 改一位 ⇒ 1", roster, w("f3.yaml", FLEET_FIX.replace(
            "edge-node: { host: <worker-ip>", "edge-node: { host: <worker-ip>")), plist, 1))
        cases.append(("fleet: 投影源声明删掉 ⇒ 1（建而不收）", roster, w("f4.yaml", FLEET_FIX.replace(
            "  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n", "", 1)), plist, 1))
        cases.append(("updates.nodes 的 ssh 改 ⇒ 1", roster, w("f5.yaml", FLEET_FIX.replace(
            "ssh: g01@<worker-ip>", "ssh: root@<worker-ip>")), plist, 1))
        cases.append(("models: host 未登记 ⇒ 1", roster, w("f6.yaml", FLEET_FIX.replace(
            "m1: { host: edge-node", "m1: { host: ghost")), plist, 1))
        cases.append(("updates 段声明删掉 ⇒ 1", roster, w("f7.yaml", FLEET_FIX.replace(
            "  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n", "", 2)), plist, 1))
        cases.append(("plist 机器名未登记 ⇒ 1", roster, fleet, w("b.plist", PLIST_FIX.replace(
            "<string>Mr2109</string>", "<string>ghost</string>")), 1))
        cases.append(("名册不在 ⇒ 2（不给结论）", os.path.join(d, "nope.yaml"), fleet, plist, 2))
        cases.append(("名册 nodes 空 ⇒ 2", w("r2.yaml", "nodes:\n"), fleet, plist, 2))

        # ── 判据 H：拨号地址由名册可推导（GAP-20260927-03；成对负控）──
        c_bad = w("ctrl_bad.plist", PLIST_CTRL_VAR_FIX.replace("__CTRL__", "http://<worker-host>:8580"))
        c_ok = w("ctrl_ok.plist", PLIST_CTRL_VAR_FIX.replace("__CTRL__", "http://<worker-ip>:8580"))
        c_port = w("ctrl_port.plist", PLIST_CTRL_LITERAL_FIX.replace("__CTRL__", "http://<worker-ip>:9"))
        cases.append(("H 拨号 host 不在名册（变量默认值）⇒ 1", roster, fleet, plist, 1, None, (c_bad,)))
        cases.append(("H 拨号可推导（变量默认值）⇒ 0", roster, fleet, plist, 0, None, (c_ok,)))
        cases.append(("H 拨号端口 ≠ core 端口 ⇒ 1", roster, fleet, plist, 1, None, (c_port,)))
        cases.append(("H 负控：断言哑化（同一 bad）⇒ 0（回不报）", roster, fleet, plist, 0, None, (c_bad,), False))

        # ── 判据 G：verified 闭集 + 「实测级必须有可核内容锚」（GAP-…-167/-171 成对负控）──
        wa = w("weights.bin", "zerg-verified-fixture\n")
        ahash = sha256_file(wa)                    # 本机件真 sha256
        bogus = "a" * 64                           # 格式对、内容不对的锚

        def gfix(vline, host="Mr2109", fpath=None):
            return (FLEET_VERIFIED_FIX
                    .replace("__HOST__", host)
                    .replace("__FILE__", fpath or "/nonexistent/none.gguf")
                    .replace("__VERIFIED__", vline))

        cases.append(("G true+锚+本机件相符+--verify-sha ⇒ 0", roster,
                      w("g1.yaml", gfix("verified: true, sha256: %s" % ahash, "Mr2109", wa)), plist, 0, True))
        cases.append(("G true+锚+本机件不符+--verify-sha ⇒ 1", roster,
                      w("g2.yaml", gfix("verified: true, sha256: %s" % bogus, "Mr2109", wa)), plist, 1, True))
        cases.append(("G true 无内容锚 ⇒ 0（降级：只报告·点名）", roster,
                      w("g3.yaml", gfix("verified: true")), plist, 0))
        cases.append(("G 锚格式非 64 位 ⇒ 0（降级：只报告）", roster,
                      w("g4.yaml", gfix("verified: true, sha256: deadbeef")), plist, 0))
        cases.append(("G true+锚+远端 host ⇒ 0（记声明级）", roster,
                      w("g5.yaml", gfix("verified: true, sha256: %s" % bogus, "edge-node")), plist, 0))
        cases.append(("G reported 缺 verified_where ⇒ 1", roster,
                      w("g6.yaml", gfix("verified: reported")), plist, 1))
        cases.append(("G reported+verified_where ∈ 名册 ⇒ 0", roster,
                      w("g7.yaml", gfix("verified: reported, verified_where: edge-node")), plist, 0))
        cases.append(("G 闭集外取值 ⇒ 1", roster,
                      w("g8.yaml", gfix("verified: maybe")), plist, 1))

        bad = 0
        for case in cases:
            name, r, f, p, want = case[:5]
            vsha = case[5] if len(case) > 5 else False
            ctrl = case[6] if len(case) > 6 else ()
            cchk = case[7] if len(case) > 7 else True
            rc = run_checks(r, f, p, verbose=False, verify_sha=vsha,
                            ctrl_plists=ctrl, ctrl_check=cchk)
            mark = "✓" if rc == want else "✗"
            if rc != want:
                bad += 1
            print("   %s %-48s 期望 rc=%d 实得 rc=%d" % (mark, name, want, rc))

        # ── 判词层限成对负控：判词必须带 read_layer / verdict_scope 与「不覆盖远端」声明 ──
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            run_checks(roster, fleet, plist, verbose=True, ctrl_plists=())
        out = buf.getvalue()
        for name, ok in (
            ("判词含 read_layer=local", "read_layer=local" in out),
            ("判词含 verdict_scope=**本机**", "verdict_scope=**本机**" in out),
            ("判词含「不覆盖」远端声明", "不覆盖" in out),
        ):
            mark = "✓" if ok else "✗"
            if not ok:
                bad += 1
            print("   %s %-48s %s" % (mark, name, "在" if ok else "**缺**"))
    print("自检结论：%s（%d 条，失败 %d）" % ("全过" if bad == 0 else "**不过**", len(cases) + 3, bad))
    return 0 if bad == 0 else 2


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
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--roster", default=DEFAULT_ROSTER)
    ap.add_argument("--fleet", default=DEFAULT_FLEET)
    ap.add_argument("--plist", default=DEFAULT_PLIST)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--verify-sha", action="store_true",
                    help="对本机可读的 verified: true 条目现算 sha256 真核（默认只查锚在不在/格式）")
    ap.add_argument("--ctrl-plist", dest="ctrl_plists", action="append", metavar="P",
                    help="判据 H 要核的 plist（可重复；默认 仓内 deploy/ 与 gateway/deploy/ 两件）")
    a = ap.parse_args()
    if a.list_rules:
        print(__doc__)
        print("路径来源：名册 %s · fleet %s · plist %s（都可覆盖；自检用夹具）"
              % (a.roster, a.fleet, a.plist))
        return 0
    if a.self_test:
        return self_test()
    # 先自证「会红」，再核真目标（仓内的既有口径）
    rc = self_test()
    if rc != 0:
        print("⛔ 自检不过 ⇒ 拒绝核真目标", file=sys.stderr)
        return 2
    print("── 自检通过，开始核真目标 ──")
    return run_checks(a.roster, a.fleet, a.plist, verify_sha=a.verify_sha,
                      ctrl_plists=a.ctrl_plists)


if __name__ == "__main__":
    sys.exit(main())
