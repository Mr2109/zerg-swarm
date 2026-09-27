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

判据（A–F，逐条能独立变红）
--------------------------
  A. 名册可读且 `nodes:` 非空（否则 rc=2 不给结论 —— 真源不在，判不了）
  B. `gateway/fleet.yaml` 的 `fleet:` 段与 `updates:` 段**各自**带投影源声明（段内出现
     `~/.zerg/contexts/`）——「建而不收」的直接判据
  C. `fleet:` 键集 == 名册键集（**两向**）；每台 host/port/os 与名册逐字相同
  D. `updates.nodes[]`：name ∈ 名册；ssh/root/prefix/api/components 与名册逐字相同
  E. `models:` 段里每个 host ∈ 名册键（模型驻留在哪台 —— 第 4 处，顺带核）
  F. 子端机器名旗标 ∈ 名册键：**盘上 plist**（`deploy/com.zerg.agent-Mr2109.plist`）与
     **在跑的 `zerg-agentd`**（`ps` 读，只读）两处都核；没有在跑的进程 ⇒ 打印跳过（不是红）

退码（与仓内既有门一致的三档）
------------------------------
  0 = 全对（PASS）
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
"""
import argparse
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


def check_fleet(fleet_path, nodes):
    """→ (fails, checks) —— B/C/D/E 四条。"""
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


def check_subend(plist_path, nodes):
    """F：盘上 plist 与在跑的 agentd 两处的机器名都要在名册里。"""
    fails, checks = [], []
    val = None
    if os.path.isfile(plist_path):
        try:
            import plistlib
            with open(plist_path, "rb") as f:
                args = plistlib.load(f).get("ProgramArguments") or []
            for i, a in enumerate(args):
                if a.endswith("machine") and i + 1 < len(args):
                    val = args[i + 1]
        except Exception as e:
            fails.append("F plist 读不了（%s）：%s" % (plist_path, e))
    else:
        checks.append("F 盘上 plist 不在（%s）⇒ 跳过该处" % plist_path)
    if val is not None:
        checks.append("F 盘上 plist 机器名 = %s" % val)
        if val not in nodes:
            fails.append("F plist 的机器名 `%s` 不在名册（改机器名要先改名册）" % val)
    run = running_agentd_machine()
    if run is None:
        checks.append("F 在跑的 zerg-agentd：没找到（跳过该处）")
    else:
        checks.append("F 在跑的 zerg-agentd 机器名 = %s" % run)
        if run not in nodes:
            fails.append("F 在跑的 zerg-agentd 机器名 `%s` 不在名册（子端自报和真源对不上）" % run)
    return fails, checks


def run_checks(roster, fleet, plist, verbose=True):
    nodes, err = load_roster(roster)
    if err:
        print("⛔ 不给结论：%s" % err)
        return 2
    assert nodes is not None
    print("真源名册 = %s（节点 %d 个：%s）" % (roster, len(nodes), " · ".join(sorted(nodes))))
    fails, checks = check_fleet(fleet, nodes)
    f2, c2 = check_subend(plist, nodes)
    fails += f2
    checks += c2
    if verbose:
        for c in checks:
            print("   · " + c)
    if fails:
        print("✗ 名册与投影不一致：%d 处" % len(fails))
        for f in fails:
            print("   ✗ " + f)
        return 1
    print("✓ 三处旧来源与名册逐字一致（fleet: · updates.nodes · 子端机器名；models: 的 host 也各在册）")
    return 0


# ══════════════════════════════════════════════════════════════════════════
# 自检（合成夹具 · 成对负控 · 不碰真目标）
# ══════════════════════════════════════════════════════════════════════════
ROSTER_FIX = """# 夹具名册
context: default
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

        bad = 0
        for name, r, f, p, want in cases:
            rc = run_checks(r, f, p, verbose=False)
            mark = "✓" if rc == want else "✗"
            if rc != want:
                bad += 1
            print("   %s %-40s 期望 rc=%d 实得 rc=%d" % (mark, name, want, rc))
    print("自检结论：%s（%d 条，失败 %d）" % ("全过" if bad == 0 else "**不过**", len(cases), bad))
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
    return run_checks(a.roster, a.fleet, a.plist)


if __name__ == "__main__":
    sys.exit(main())
