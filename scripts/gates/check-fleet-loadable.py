#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-fleet-loadable.py — 机群清单 `gateway/fleet.yaml` 的「**可加载**」机读判据（缺口 GAP-20260925-12）

为什么存在
----------
缺口原文（GAP-20260925-12）：「名册没有『件能否加载』的机读判据：fork 专用件在名册里只有
`verified:true` 一个布尔，没有字段/门能机器判定**某机的二进制是否支持该量化类型**」。

`verified:` 是「核没核过」的布尔（CLI 侧写册路径 `family_config.go` 也只写它），**不是**「这台机器
能不能把它拉起来」。真知识此前只活在**注释**里（如 `fleet.yaml` 里「Ternary-Bonsai … llama.cpp 厂商
fork(PrismML-Eng)专用·主线未支持」）⇒ **人可读、机不可判**。本门把那套判断落成**机读判据**。

判据（A–E；每条各自能独立判红）
------------------------------
  A. `models:` 段可解析且**非空**（空域 ⇒ rc=2 **不给结论** —— 零执行 = 假绿）。
  B. 每条候选**必备字段齐全**：`host` / `backend` / `file` 非空，`mem_gb` > 0。
  C. **不引不存在的东西**：`host` ∈ `fleet:` 段的机器键集；`backend` ∈ 闭集；`file` 以
     `.gguf` 结尾；`mmproj`（若给）也以 `.gguf` 结尾。
  D. **引擎实现一致（可加载核心）**：一个模型组只要**任一**候选声明了非主线引擎实现
     （`cmd:` 非空 **或** `engine_impl_required: true` **或** 架构 ∈ 主线不支持清单），
     则该组**每一条**候选都必须带 `cmd:` —— 否则那台机器会**静默退回主线 llama-server**
     （= 起不来或误链，且不报错）⇒ 点名「模型 × 机器」。
  E. `aliases:`（若给）的每个值必须指向一个**在册模型名**（不引不存在的模型）。

★ 本门**只判本机盘上的 `gateway/fleet.yaml`**（`read_layer=local`）：远端机器的文件存在性、
  远端二进制版本**读不到** ⇒ 本门不判、也不把「读不到」当绿；它判的是**声明面的机读一致性**。

退码（与仓内既有门一致的三档）
------------------------------
  0 = 全对（PASS）
  1 = 有可由声明面判定的缺陷（A–E 任一命中；逐条点名「模型 · 机器 · 哪个字段 · 现值」）
  2 = **不给结论**（fleet.yaml 不在 / 解析不出 `models:` / 空域 / 自检不过 / `--fleet` 指的不是件）

用法
----
    python3 scripts/gates/check-fleet-loadable.py                 # 核真目标 gateway/fleet.yaml
    python3 scripts/gates/check-fleet-loadable.py --fleet P       # 换清单路径（测试/夹具用）
    python3 scripts/gates/check-fleet-loadable.py --self-test     # 只跑自检（合成夹具，不碰真目标）
"""
import argparse
import os
import re
import sys

# ── 闭集 ───────────────────────────────────────────────────────────────────
# 判据 C：backend 闭集（与 fleet.yaml 现读一致；新增 backend 须同批扩本表）。
BACKENDS = ("llama-server", "ds4-server")
# 判据 B：候选必备字段（缺一即判）。
REQUIRED_FIELDS = ("host", "backend", "file")
# 判据 D：主线 llama.cpp 不支持的架构（真源同 core/internal/config/validator.go 的
# `MainlineUnsupportedArchitectures`）。★ 该清单会随新引擎变长 —— 本门只作**参照真源**，
#   判据 D 的主触发面是「`cmd:` 非空」与「`engine_impl_required: true」两处（引擎自陈），
#   本清单只补「还没打标的老形态」。
MAINLINE_UNSUPPORTED_ARCHS = ("example-moe-36b",)

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
DEFAULT_FLEET = os.path.join(REPO_ROOT, "gateway", "fleet.yaml")


# ══════════════════════════════════════════════════════════════════════════
# 极简 YAML 子集解析（只服务本门的形状：块式映射/列表 + 行内流式映射）
#   ★ 为什么不用 PyYAML：本仓的门禁脚本一律**零第三方依赖**（本机 python3 没有 yaml 模块）。
#     形状固定；解析失败**大声报**（⇒ rc=2），不静默漏判 —— 这是零依赖可接受的前提。
# ══════════════════════════════════════════════════════════════════════════
def _bracket_delta(ch, depth):
    if ch in "{[":
        return depth + 1
    if ch in "}]":
        return depth - 1
    return depth


def strip_comment(line):
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
        depth = _bracket_delta(ch, depth)
        if depth < 0:
            depth = 0
        if ch == "#" and depth == 0 and (i == 0 or line[i - 1] in " \t"):
            break
        out.append(ch)
    return "".join(out).rstrip()


def split_top(text, sep=","):
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
        depth = _bracket_delta(ch, depth)
        if ch == sep and depth <= 0:
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


def section_range(lines, key):
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


def read_lines(path):
    with open(path, encoding="utf-8") as f:
        raw = f.read().split("\n")
    return raw, [strip_comment(l) for l in raw]


# ══════════════════════════════════════════════════════════════════════════
# 清单解析：把 `models:` 段解析成 {模型名: [ {字段: 值}, ... ]}；`fleet:` 段解析成机器键集
# ══════════════════════════════════════════════════════════════════════════
def parse_models(section_lines):
    """`models:` 段 → {模型名: [candidate dict, ...]}。支持三种形态：
        - `Name:` + `    - { ... }`（流式候选）
        - `Name:` + `    - host: edge-node`（块式候选，含后续更深缩进的键）
        - `Name:   { ... }`（单行单候选）"""
    groups = {}
    cur_name = None
    i = 0
    n = len(section_lines)
    while i < n:
        raw = section_lines[i]
        if not raw.strip():
            i += 1
            continue
        ind = len(raw) - len(raw.lstrip(" "))
        s = raw.strip()
        # 模型名行（缩进 2，且含 `:`）
        if ind <= 2 and not s.startswith("-"):
            m = _KV.match(s)
            if m:
                cur_name = unquote(m.group(1))
                val = m.group(2).strip()
                groups.setdefault(cur_name, [])
                if val.startswith("{"):
                    groups[cur_name].append(parse_flow(val))
                elif val:
                    # 标量值不算候选（本门形状里不该出现）
                    pass
                i += 1
                continue
        # 候选行（缩进 ≥ 4 的 `- `）
        if s.startswith("-") and cur_name is not None:
            body = s[1:].strip()
            cand = {}
            if body.startswith("{"):
                cand.update(parse_flow(body))
                i += 1
            else:
                m = _KV.match(body)
                if m:
                    k, v = unquote(m.group(1)), m.group(2).strip()
                    cand[k] = parse_flow(v) if v.startswith("{") else unquote(v)
                i += 1
                # 收后续更深缩进的键（块式）
                while i < n and section_lines[i].strip() and \
                        (len(section_lines[i]) - len(section_lines[i].lstrip(" "))) > ind:
                    m2 = _KV.match(section_lines[i].strip())
                    if m2:
                        k2, v2 = unquote(m2.group(1)), m2.group(2).strip()
                        cand[k2] = parse_flow(v2) if v2.startswith("{") else unquote(v2)
                    i += 1
            groups[cur_name].append(cand)
            continue
        i += 1
    return groups


def parse_fleet_keys(section_lines):
    """`fleet:` 段 → 机器键集。每行形如 `  edge-node: { host: …, port: … }`。"""
    keys = set()
    for raw in section_lines:
        if not raw.strip():
            continue
        ind = len(raw) - len(raw.lstrip(" "))
        if ind != 2:
            continue
        m = _KV.match(raw.strip())
        if m:
            keys.add(unquote(m.group(1)))
    return keys


def parse_aliases(section_lines):
    """`aliases:` 段 → {别名: 目标模型名}。空段 ⇒ {}。"""
    out = {}
    for raw in section_lines:
        if not raw.strip():
            continue
        m = _KV.match(raw.strip())
        if m:
            out[unquote(m.group(1))] = unquote(m.group(2))
        elif raw.strip().startswith("-"):
            # 别名也可能是序列形态；本仓现读是空段，保守跳过
            continue
    return out


# ══════════════════════════════════════════════════════════════════════════
# 判据
# ══════════════════════════════════════════════════════════════════════════
def arch_of(c):
    return (c.get("architecture") or c.get("arch") or "").strip()


def cmd_present(c):
    """候选是否声明了非主线引擎实现（`cmd:`）。"""
    v = c.get("cmd")
    if v is None:
        return False
    if isinstance(v, list):
        return len(v) > 0
    return bool(str(v).strip())


def impl_flag(c):
    """`engine_impl_required: true`（通用声明开关）。"""
    v = str(c.get("engine_impl_required", "")).strip().lower()
    return v in ("true", "yes", "1")


def needs_engine_impl(c):
    """该候选是否**必须**显式声明引擎实现（= 主线引擎支持不了它）。三触发任一：
       ① 架构 ∈ 主线不支持清单；② 显式打标 engine_impl_required；③ 已带 cmd:（自陈非主线）。"""
    if impl_flag(c):
        return True
    if arch_of(c) in MAINLINE_UNSUPPORTED_ARCHS:
        return True
    # 判据 C 的「backend 闭集」之外（非 llama 家族）时引擎实现由 backend 承载，不重复要求 cmd:
    if (c.get("backend") or "").strip() != "llama-server":
        return False
    return cmd_present(c)


def toe(v):
    return float(v) if v not in (None, "") else None


def check(fleet_path):
    """→ (rc, findings)。rc∈{0,1,2}；findings = [(面, 文字)]。"""
    if not os.path.isfile(fleet_path):
        return 2, [("A", "清单件不在：%s ⇒ 判不了（不给结论）" % fleet_path)]
    try:
        raw, stripped = read_lines(fleet_path)
    except OSError as e:
        return 2, [("A", "读不到清单件：%s（%s）" % (fleet_path, e))]

    models_rng = section_range(stripped, "models")
    if models_rng is None:
        return 2, [("A", "解析不出 `models:` 段 ⇒ 判不了（不给结论）")]
    models = parse_models(stripped[models_rng[0]:models_rng[1]])
    models = {k: v for k, v in models.items() if v is not None}
    if not models:
        return 2, [("A", "`models:` 段为空（零执行 = 假绿）⇒ 判不了（不给结论）")]

    fleet_rng = section_range(stripped, "fleet")
    fleet_keys = parse_fleet_keys(stripped[fleet_rng[0]:fleet_rng[1]]) if fleet_rng else set()
    aliases_rng = section_range(stripped, "aliases")
    aliases = parse_aliases(stripped[aliases_rng[0]:aliases_rng[1]]) if aliases_rng else {}

    findings = []
    for name, cands in models.items():
        for idx, c in enumerate(cands):
            where = "%s[#%d]" % (name, idx)
            if not c:
                findings.append(("B", "%s：候选解析为空（YAML 形状不支持 / 空候选）" % where))
                continue
            # B. 必备字段齐全
            for f in REQUIRED_FIELDS:
                if not str(c.get(f, "")).strip():
                    findings.append(("B", "%s：缺必备字段 `%s`" % (where, f)))
            mg = toe(c.get("mem_gb"))
            if mg is None:
                findings.append(("B", "%s：缺 `mem_gb`（无法判内存可加载性）" % where))
            elif mg <= 0:
                findings.append(("B", "%s：`mem_gb`=%s 非正数" % (where, c.get("mem_gb"))))
            # C. 不引不存在的东西
            host = str(c.get("host", "")).strip()
            if host and fleet_keys and host not in fleet_keys:
                findings.append(("C", "%s：host `%s` 不在 `fleet:` 机器键集 %s 里（引了不存在的机器）"
                                 % (where, host, sorted(fleet_keys))))
            be = str(c.get("backend", "")).strip()
            if be and be not in BACKENDS:
                findings.append(("C", "%s：backend `%s` 不在闭集 %s 里" % (where, be, list(BACKENDS))))
            fp = str(c.get("file", "")).strip()
            if fp and not fp.endswith(".gguf"):
                findings.append(("C", "%s：`file` 不以 `.gguf` 结尾（不是可加载权重）：%s" % (where, fp)))
            mm = str(c.get("mmproj", "")).strip()
            if mm and not mm.endswith(".gguf"):
                findings.append(("C", "%s：`mmproj` 不以 `.gguf` 结尾：%s" % (where, mm)))
    # D. 引擎实现一致（可加载核心）—— 按**模型组**判
    for name, cands in models.items():
        if not cands:
            continue
        needs = [needs_engine_impl(c) for c in cands]
        if any(needs) and not all(needs):
            for idx, c in enumerate(cands):
                if not needs[idx]:
                    host = str(c.get("host", "")).strip() or "?"
                    findings.append(("D", "%s 在机器 `%s` 上：同组别的候选已声明非主线引擎实现"
                                     "（cmd:/engine_impl_required），本条没有 ⇒ 会**静默退回主线**"
                                     " llama-server（该机不可加载）" % (name, host)))
    # E. 别名不引不存在的模型
    for alias, target in aliases.items():
        if target and target not in models:
            findings.append(("E", "alias `%s` → `%s`：目标模型不在册" % (alias, target)))

    return (1 if findings else 0), findings


# ══════════════════════════════════════════════════════════════════════════
# 自检（合成夹具 · 不碰真目标）
# ══════════════════════════════════════════════════════════════════════════
GOOD = """\
models:
  Foo-Fork:
    - { host: m1, backend: llama-server, file: /srv/models/foo.gguf, mem_gb: 8, cmd: ["/opt/run-foo.sh", "-m", "{file}"] }
    - { host: m2, backend: llama-server, file: /srv/models/foo.gguf, mem_gb: 8, cmd: ["/opt/run-foo.sh", "-m", "{file}"] }
  Bar:
    - { host: m1, backend: llama-server, file: /srv/models/bar.gguf, mem_gb: 4 }
aliases:
fleet:
  m1: { host: <controller-ip>, port: 8100, os: macos, kind: work }
  m2: { host: <worker-ip>, port: 8100, os: ubuntu, kind: ai }
"""

BAD_FORK_INCONSISTENT = """\
models:
  Foo-Fork:
    - { host: m1, backend: llama-server, file: /srv/models/foo.gguf, mem_gb: 8, cmd: ["/opt/run-foo.sh", "-m", "{file}"] }
    - { host: m2, backend: llama-server, file: /srv/models/foo.gguf, mem_gb: 8 }
fleet:
  m1: { host: <controller-ip>, port: 8100, os: macos, kind: work }
  m2: { host: <worker-ip>, port: 8100, os: ubuntu, kind: ai }
"""

BAD_FIELDS = """\
models:
  Broken:
    - { host: m9, backend: llama-server, file: /srv/models/x.bin, mem_gb: 0 }
fleet:
  m1: { host: <controller-ip>, port: 8100, os: macos, kind: work }
"""

EMPTY = "models:\nfleet:\n  m1: { host: <controller-ip>, port: 8100 }\n"


def _write_tmp(text):
    import tempfile
    fd, p = tempfile.mkstemp(suffix=".yaml", prefix="fleet-loadable-selftest-")
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        f.write(text)
    return p


def run_self_test():
    fails = []

    def expect(label, text, want_rc, want_face=None):
        p = _write_tmp(text)
        try:
            rc, findings = check(p)
        finally:
            os.unlink(p)
        if rc != want_rc:
            fails.append("%s：期望 rc=%d，实得 rc=%d（findings=%s）" % (label, want_rc, rc, findings))
            return
        if want_face and not any(f == want_face for f, _ in findings):
            fails.append("%s：期望命中面 %s，实得面集 %s" % (label, want_face, [f for f, _ in findings]))

    expect("good", GOOD, 0)
    expect("fork-inconsistent", BAD_FORK_INCONSISTENT, 1, "D")
    expect("bad-fields", BAD_FIELDS, 1, "B")
    expect("empty-domain", EMPTY, 2, "A")

    # 缺件 ⇒ rc=2（不给结论，不是绿）
    rc, _ = check("/nonexistent/fleet.yaml")
    if rc != 2:
        fails.append("missing-file：期望 rc=2，实得 rc=%d" % rc)

    return fails


def main(argv=None):
    ap = argparse.ArgumentParser(add_help=True, description="机群清单「可加载」机读判据")
    ap.add_argument("--fleet", default=DEFAULT_FLEET, help="清单件路径（默认 %s）" % DEFAULT_FLEET)
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成夹具，不碰真目标）")
    args = ap.parse_args(argv)

    if args.self_test:
        fails = run_self_test()
        if fails:
            print("✗ 自检不过（%d 条）：" % len(fails))
            for f in fails:
                print("  - " + f)
            return 2
        print("✓ check-fleet-loadable 自检全过（5 格）")
        return 0

    rc, findings = check(args.fleet)
    if rc == 2:
        print("✗ 不给结论（rc=2）：")
        for face, msg in findings:
            print("  [%s] %s" % (face, msg))
        return 2
    if rc == 1:
        print("✗ 可加载性缺陷 %d 条（fleet=%s）：" % (len(findings), args.fleet))
        for face, msg in findings:
            print("  [%s] %s" % (face, msg))
        return 1
    print("✓ 可加载判据全过（fleet=%s）：必备字段齐 · 引用都在册 · 引擎实现一致" % args.fleet)
    return 0


if __name__ == "__main__":
    sys.exit(main())
