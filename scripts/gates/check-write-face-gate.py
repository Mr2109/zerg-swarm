#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-write-face-gate.py —— 命令面「声明 ↔ 实现」两面判（缺口 `GAP-20260927-90`）

题面（缺口 `want_action` 逐字）：
    按该枚 C 节设计落地：`authority-map.json` 新增「写面登记」（授权类闭集 + 白名单理由逐字写
    「无闸」，照现有「无对应」一格）；新增判据件 `cli_write_face_gate_test.go` 与门禁步
    `check-write-face-gate.py`；**两面判**（声明面 `usage`/`writeSpec` ↔ 实现面 AST 认
    `if !inv.yes`／`!inv.confirmGiven`／唯一门调用／验签口令四类形状）；
    三条判定（旗标必须声明 · 无闸必须显式白名单 · 强闸必须进危险清单）＋ 6 负控 1 空转 ＋
    5 防误报硬约束（证据落 file:line、白名单空即红、探针只跑零副作用档且在隔离副本、
    深度>2 判 2、自检不过不许报绿）。

病象（缺口 `GAP-20260927-90`）：
    11 条命令会写盘却一条授权都不要（`help export` · `gate matrix --out` · `dev proposal new` ·
    `dev verify` · `dev receipt new` · `plan --out` · `impact <目标>`（缓存路）· `isolate-ro new` ·
    `gate run` · `publish preflight` · `gate bench`），且审计面全仓只有三处 ⇒ 写动作不留授权记录。

── 本候件的**范围声明**（诚实边界）──
    真源设计里的 `writeSpec` 字段**尚未落 `catalog()`**（本刻现读 `writeSpec` 在 `core/cmd/zerg`
    0 命中）⇒ 本件落**声明面用 `usage` 串 + `danger` 现况、实现面用写调用现读**的**可用子集**：
        · 「旗标必须声明」（写面命令的 usage 必须含 `--yes`/`--confirm`/`--out`）
          ＋「无闸必须显式白名单」（无 danger 又无写旗标的写面命令 ⇒ 缺白名单即红）。
        · 第三「强闸必须进危险清单」仍待 `writeSpec` 落地。
    2026-10-05 接入：**本轮消费 `core/internal/contract/authority-map.json` 的「写面登记」**（体例见其
    `写面登记_说明`）—— 登记面即白名单，按 `授权类`（`D2`/`D3`/`无闸`）分类报；表里**未登记而会写盘**的写面
    **仍判「未声明/未白名单」**（✗ 不得因为加了表就全放行）；表里**假条目**（命令面不存在 / 授权类不在闭集 /
    理由·出处空 / 无闸声明不真 = 洗白）逐条点名（含负控）。
    这**不是**把整条缺口判「已解」—— 只是把其中可现算的两条摊在亮处、逐条点名。

用法：
    python3 scripts/gates/check-write-face-gate.py [--root <dir>] [--json] [--self-test]

★ 接线状态：**已挂门100**（tri-report 只报告档）· 2026-10-04 现读更正（原「仓外候件 / 未挂门步」已过期）。
★ 2026-10-05：接入 `authority-map.json`「写面登记」（假条目可点名 · 含负控）。
★ 2026-10-06 修两缺陷（隔离副本负控实据）：
    ① **认常量 usage**：命令树里 `usage` 允许写成包内常量（现读 3 条：`impactUsageLine` · `lsFaceUsage` ·
       `auditCoverUsage`）—— 旧解析只认字面量，把这些条目**整条跳过** ⇒ 登记它会被假判「命令面不存在」。
       现回源件按常量名查字面量（同源先例 `scripts/docs/gen-cli-reference.py` 的 `const_usages`）；
       查不到 ⇒ **不静默当不存在**，而是报「判不了」（条目仍进表、按常量名点名）。
    ② **授权旗标 / 落点旗标分两档**：旧 `WRITE_FLAG_RE` 把 `--yes`/`--confirm`（授权旗标 ⇒ 构成闸）与
       `--out`（落点旗标 ⇒ 不构成闸）揉成一档 ⇒ `help export`/`plan` 这类「有 `--out` 但无需授权」的
       写面被判「无闸不真」（登记即被假拒），且在跑面上被当成「已声明旗标」而逃过缺白名单点名。
       判据：**带落点旗标但无授权旗标的写面 ⇒ 属「无闸」档（应可进登记 · 未登记则缺白名单点名）**。
"""
import argparse
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
try:
    import importlib.util as _u
    _spec = _u.spec_from_file_location(
        "cwa", os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-write-authz.py"))
    _cwa = _u.module_from_spec(_spec); _spec.loader.exec_module(_cwa)
except Exception:  # noqa: BLE001
    _cwa = None

# ★ 写面旗标分两档（2026-10-06 订正 · 缺陷②）：
#   · **授权旗标**（构成「闸」）：`--yes`（D2 执行许可）/ `--confirm=<目标>`（D3 人点名目标）
#   · **落点旗标**（只定「写到哪」，**不**构成闸）：`--out <件|目录>`
#   旧版 `WRITE_FLAG_RE` 把两类揉成一档 ⇒ `help export`/`plan` 这类「有 `--out` 但无需授权」的写面
#   被误判「无闸不真」（登记即被假拒），跑面上又被当成「已声明旗标」而逃过缺白名单点名。
AUTH_FLAG_RE = re.compile(r"--(yes|confirm)\b")   # 授权旗标 ⇒ 构成闸
OUT_FLAG_RE = re.compile(r"--out\b")              # 落点旗标 ⇒ 不构成闸
WRITE_FLAG_RE = re.compile(r"--(yes|confirm|out)\b")  # 任一写面旗标（仅作「有无写面旗标」的粗判）
USAGE_RE = re.compile(r'usage:\s*"([^"]*)"')
# `usage: <标识符>` —— 命令树里 `usage` 可写成包内常量（缺陷①），须回源件查它的字面量。
USAGE_CONST_RE = re.compile(r'usage:\s*([A-Za-z_][A-Za-z0-9_]*)\s*,')
# 包内 `名字 = "字面量"` 表（同源先例 = `scripts/docs/gen-cli-reference.py:281` 的 `CONST_RE`）。
CONST_RE = re.compile(r'^\s*(?:const\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*"([^"]*)"\s*$', re.M)
PATH_RE = re.compile(r'path:\s*\[\]string\{([^}]*)\}')
DANGER_RE = re.compile(r"danger:\s*(nil|&dangerSpec\{)")
RUN_RE = re.compile(r"run:\s*([A-Za-z_][\w]*)")
# 写面登记白名单真源 = `core/internal/contract/authority-map.json` 的「写面登记」（登记面即白名单）。
AUTH_MAP_REL = os.path.join("core", "internal", "contract", "authority-map.json")
# 授权类闭集回落（真源 = 表里 `授权类闭集`；缺或空 ⇒ 回落此三枚）。
AUTH_CLASS_FALLBACK = ("D2", "D3", "无闸")


def impl_write_funcs(scopedir):
    """回 {func: [(file,line)]} —— 含写调用的函数。"""
    out = {}
    if _cwa is None:
        return out
    for name in sorted(os.listdir(scopedir)):
        if not name.endswith(".go") or name.endswith("_test.go"):
            continue
        for lineno, func, call, _ in _cwa.scan_file(os.path.join(scopedir, name)):
            out.setdefault(func, []).append((name, lineno))
    return out


def const_usage_map(scopedir):
    """扫包内 `*.go`（非测试）取 {标识符: "字面量"} —— 供 `usage: <常量>` 回源解析（缺陷①）。

    同源先例 `scripts/docs/gen-cli-reference.py` 的 `const_usages`：命令树 usage 与实现件常量
    「唯一真源、本件不旁写第二份」⇒ 只认字面量的老解析器会把这些条目读出空串/整条跳过。
    """
    out = {}
    try:
        names = sorted(os.listdir(scopedir))
    except OSError:
        return out
    for name in names:
        if not name.endswith(".go") or name.endswith("_test.go"):
            continue
        try:
            text = open(os.path.join(scopedir, name), "r", encoding="utf-8").read()
        except OSError:
            continue
        for m in CONST_RE.finditer(text):
            out.setdefault(m.group(1), m.group(2))
    return out


def catalog_entries(main_go, consts=None):
    """粗解析 main.go 的 catalog 条目 {path, usage, usage_known, danger?, run}。

    `usage` 可为**字面量**或**包内常量引用**（缺陷①）：
      · 字面量 ⇒ usage_known=True；
      · 常量引用且回源件查得到字面量 ⇒ 解出后 usage_known=True；
      · 常量引用但查不到 ⇒ usage_known=False —— **不静默跳过**（条目仍进表、登记它不会被假判「不存在」，
        上层按常量名报「判不了」）。
    """
    consts = consts or {}
    text = open(main_go, "r", encoding="utf-8").read()
    ents = []
    for block in re.findall(r"\{(?:[^{}]|\{[^{}]*\})*?\}", text):
        um = USAGE_RE.search(block)
        cm = USAGE_CONST_RE.search(block)
        if not um and not cm:
            continue  # 该块没有 usage 字段 ⇒ 非命令条目
        if um:
            usage, known = um.group(1), True
        elif cm.group(1) in consts:
            usage, known = consts[cm.group(1)], True
        else:
            usage, known = "", False  # 常量引用但回源件查不到字面量 ⇒ 判不了
        pm = PATH_RE.search(block)
        dm = DANGER_RE.search(block)
        rm = RUN_RE.search(block)
        path = ""
        if pm:
            path = " ".join(re.findall(r'"([^"]*)"', pm.group(1)))
        ents.append({"path": path, "usage": usage, "usage_known": known,
                     "has_danger": bool(dm and dm.group(1) != "nil"),
                     "run": rm.group(1) if rm else ""})
    return ents


def load_write_registry(root):
    """读 `authority-map.json` 的「写面登记」。回 (entries, cls, err)。

    entries=None ⇒ 契约件不存在（按「无登记」处理 ⇒ 不退 BLOCKED；写面照判未声明/未白名单）。
    err!="" ⇒ 契约件在、但「写面登记」不可用 ⇒ 调用方 rc=2 不给结论（fail-closed）。
    """
    path = os.path.join(root, AUTH_MAP_REL)
    if not os.path.isfile(path):
        return None, set(AUTH_CLASS_FALLBACK), ""
    try:
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except Exception as e:  # noqa: BLE001
        return None, set(AUTH_CLASS_FALLBACK), "authority-map.json 解析失败：%s" % e
    cls = {c.get("值") for c in (data.get("授权类闭集") or [])
           if isinstance(c, dict) and c.get("值")}
    if not cls:
        cls = set(AUTH_CLASS_FALLBACK)
    reg = data.get("写面登记")
    if not isinstance(reg, list):
        return None, cls, "authority-map.json 缺「写面登记」表（非 list）"
    return reg, cls, ""


def validate_registry(reg, cls, cat_by_path):
    """校「写面登记」每条 ⇒ 回 (valid, fake, undecided)。

    valid = {命令: 授权类}（条目完整 ⇒ 充作白名单）。
    fake  = [(命令, 原因)]（假条目 = 逐条能指出的牙：命令不存在 / 授权类越集 / 理由·出处空 /
            **无闸不真**（usage 已声明授权旗标却洗成无闸））。
    undecided = [(命令, 原因)]（**判不了**：usage 是常量引用但回源件查不到字面量 ⇒ 不静默、不假判「不存在」）。
    """
    valid, fake, undecided, seen = {}, [], [], set()
    for ent in reg:
        if not isinstance(ent, dict):
            fake.append(("<非对象>", "条目不是 JSON 对象"))
            continue
        cmd = str(ent.get("命令", "")).strip()
        cls_v = str(ent.get("授权类", "")).strip()
        reason = str(ent.get("理由", "")).strip()
        source = str(ent.get("出处", "")).strip()
        who = cmd or "<命令空>"
        cat = cat_by_path.get(cmd) if cmd else None
        probs = []
        if not cmd:
            probs.append("命令空")
        elif cmd in seen:
            probs.append("命令重复登记")
        elif cat is None:
            probs.append("命令面不存在（catalog() 无此 path ⇒ 瞎写的命令）")
        if not cls_v:
            probs.append("授权类空")
        elif cls_v not in cls:
            probs.append("授权类「%s」不在授权类闭集 {%s}" % (cls_v, "/".join(sorted(cls))))
        if not reason:
            probs.append("理由空")
        if not source:
            probs.append("出处空")
        if cls_v == "无闸" and reason and "无闸" not in reason:
            probs.append("无闸条目理由未逐字写「无闸」（洗白）")
        # ★ 无闸不真：只认**授权旗标**（`--yes`/`--confirm`）—— 落点旗标 `--out` 不构成闸（缺陷②）。
        if cls_v == "无闸" and cat is not None and cat.get("usage_known", True) \
                and AUTH_FLAG_RE.search(cat["usage"]):
            probs.append("无闸声明不真：usage 已声明授权旗标（--yes/--confirm）⇒ 该写面有闸，不得洗成无闸")
        # ★ usage 判不了（常量引用查不到字面量）⇒ 无法确认其无闸 ⇒ 判不了，不假判、不静默放行（缺陷①）。
        if cls_v == "无闸" and cat is not None and not cat.get("usage_known", True):
            undecided.append((who, "判不了：usage 为常量引用但回源件查不到字面量 ⇒ 无法确认它无闸（✗ 不静默当无闸）"))
        if cls_v in ("D2", "D3") and cat is not None and not cat["has_danger"]:
            probs.append("%s 声明不真：catalog() 无 danger 档" % cls_v)
        if cmd:
            seen.add(cmd)
        if probs:
            fake.append((who, "；".join(probs)))
        else:
            valid[cmd] = cls_v
    return valid, fake, undecided


def run(root, as_json):
    scopedir = os.path.join(root, "core", "cmd", "zerg")
    main_go = os.path.join(scopedir, "main.go")
    if not os.path.isdir(scopedir) or not os.path.isfile(main_go):
        print("BLOCKED —— 缺 main.go / 扫描面 ⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    reg, cls, rerr = load_write_registry(root)
    if rerr:
        print("BLOCKED —— %s ⇒ rc=2 不给结论" % rerr, file=sys.stderr)
        return 2
    wfuncs = impl_write_funcs(scopedir)
    consts = const_usage_map(scopedir)  # 缺陷①：回源件取包内常量 usage 的字面量
    entries = catalog_entries(main_go, consts)
    if not entries:
        print("BLOCKED —— catalog() 解析出 0 条（空域）⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    cat_by_path = {e["path"]: e for e in entries if e["path"]}
    valid, fake, undecided = validate_registry(reg or [], cls, cat_by_path)
    undecided_cmds = {c for c, _ in undecided}
    # 登记面 = 白名单：仅「无闸」一档豁免写面缺白名单（D2/D3 仍需 usage 逐字声明旗标）。
    waived = {c for c, k in valid.items() if k == "无闸"}
    registered = sorted(valid.items(), key=lambda kv: (kv[1], kv[0]))
    by_cls = {}
    for c, k in registered:
        by_cls.setdefault(k, []).append(c)
    hits = []
    for e in entries:
        cmd = e["path"] or e["run"]
        writes = e["run"] in wfuncs
        if not e["usage_known"]:
            # ★ 缺陷①：usage 判不了 ⇒ 会写盘的**不静默当不存在/不虚报命中**，按常量引用点名「判不了」。
            if writes and cmd not in undecided_cmds:
                undecided.append((cmd, "判不了：usage 为常量引用且回源件查不到字面量 ⇒ 无法判它有没有闸"))
            continue
        # ★ 缺陷②：闸由**授权旗标**（--yes/--confirm）构成；落点旗标（--out）不算闸。
        declares_auth = bool(AUTH_FLAG_RE.search(e["usage"]))
        declares_out = bool(OUT_FLAG_RE.search(e["usage"]))
        if cmd in waived:
            continue  # 已显式白名单（无闸）⇒ 不再是「未声明/未白名单」
        if writes and not declares_auth and not e["has_danger"]:
            why = "写面但 usage 未声明授权旗标（--yes/--confirm）且无 danger（缺白名单）"
            if declares_out:
                # 判据：带落点旗标但无授权旗标的写面 ⇒ 属「无闸」档 ⇒ 必须进登记，未登记即缺白名单。
                why = "带落点旗标 --out 但无授权旗标（--yes/--confirm）⇒ 属「无闸」写面，缺白名单登记"
            hits.append((cmd, why, e["usage"]))
        elif e["has_danger"] and not declares_auth:
            hits.append((cmd, "已登记 danger 但 usage 未声明授权旗标", e["usage"]))
    rc = 1 if (hits or fake or undecided) else 0
    if as_json:
        print(json.dumps({
            "rc": rc,
            "registered": [{"cmd": c, "cls": k} for c, k in registered],
            "registered_by_class": by_cls,
            "unregistered": [{"cmd": c, "why": w} for c, w, _ in hits],
            "fake_entries": [{"cmd": c, "why": w} for c, w in fake],
            "undecided": [{"cmd": c, "why": w} for c, w in undecided],
        }, ensure_ascii=False))
    else:
        print("命令面写面「声明↔实现」两面判（GAP-20260927-90 · authority-map 写面登记接入）")
        print("catalog 条目 %d · 写面函数 %d · 已登记写面 = %d（%s）· 未声明/未白名单 = %d 条 · 假条目 = %d 条 · 判不了 = %d 条"
              % (len(entries), len(wfuncs), len(registered),
                 " · ".join("%s %d" % (k, len(v)) for k, v in sorted(by_cls.items())) or "无",
                 len(hits), len(fake), len(undecided)))
        for c, k in registered:
            print("  · [%s] %s" % (k, c))
        for c, w, u in hits:
            print("  ✗ %s：%s ⇒ usage=「%s」" % (c, w, u))
        for c, w in fake:
            print("  ✗[假条目] %s：%s" % (c, w))
        for c, w in undecided:
            print("  ✗[判不了] %s：%s" % (c, w))
        print("  ★ 登记面 = `core/internal/contract/authority-map.json` 的「写面登记」（白名单）；"
              "未登记而会写盘者仍判「未声明/未白名单」；表里假条目（命令不存在 / 授权类越集 /")
        print("    理由·出处空 / 无闸不真）逐条点名。闸由授权旗标（--yes/--confirm）构成，落点旗标（--out）不算闸。")
    return rc


# ── 自检夹具 ──────────────────────────────────────────────────────────────
# 三条命令：`help export`（**落点旗标 `--out` 无授权旗标** · 缺陷②）· `impact`（**usage 是常量引用**
# · 缺陷①）· `guarded`（usage 声明授权旗标 `--yes` + danger ⇒ 真闸，用于洗白负控）。
_MAIN_GO = (
    'package main\n'
    'var cat = []*command{\n'
    ' {path: []string{"help", "export"}, usage: "zerg help export [--out <目录>] [--dry-run]", run: cmdHelpExport},\n'
    ' {path: []string{"impact"}, usage: impactUsageLine, run: cmdImpact},\n'
    ' {path: []string{"guarded"}, usage: "zerg guarded [--yes]", danger: &dangerSpec{...}, run: cmdGuarded},\n'
    '}\n')
_WRITERS = {
    "family_help.go":
        'func cmdHelpExport(inv *invocation, o, e io.Writer) int {\n\tos.WriteFile("x", b, 0o644)\n\treturn 0\n}\n',
    "family_impact.go":
        'const impactUsageLine = "zerg impact <目标> [--json <字段>]"\n\n'
        'func cmdImpact(inv *invocation, o, e io.Writer) int {\n\tos.WriteFile("y", b, 0o644)\n\treturn 0\n}\n',
    "family_guarded.go":
        'func cmdGuarded(inv *invocation, o, e io.Writer) int {\n\tos.WriteFile("z", b, 0o644)\n\treturn 0\n}\n',
}
# 缺陷① 负控夹具：`usage` 是常量引用，但包内**查不到**它的字面量 ⇒ 应报「判不了」（不许静默当不存在）。
_MAIN_GO_GHOST = (
    'package main\n'
    'var cat = []*command{\n'
    ' {path: []string{"ghost"}, usage: ghostUsageLine, run: cmdGhost},\n'
    '}\n')
_MAIN_GO_EMPTY = 'package main\n\nvar cat = []*command{}\n'
_WRITER_GHOST = {
    "family_ghost.go":
        'func cmdGhost(inv *invocation, o, e io.Writer) int {\n\tos.WriteFile("g", b, 0o644)\n\treturn 0\n}\n',
}


def _write_reg(td, entries):
    d = os.path.join(td, "core", "internal", "contract")
    os.makedirs(d, exist_ok=True)
    data = {"授权类闭集": [{"值": "D2"}, {"值": "D3"}, {"值": "无闸"}], "写面登记": entries}
    with open(os.path.join(d, "authority-map.json"), "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False)


def _write_reg_raw(td, data):
    d = os.path.join(td, "core", "internal", "contract")
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "authority-map.json"), "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False)


def _mkfixture(td, main_src, writers):
    d = os.path.join(td, "core", "cmd", "zerg")
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "main.go"), "w", encoding="utf-8") as f:
        f.write(main_src)
    for name, body in writers.items():
        with open(os.path.join(d, name), "w", encoding="utf-8") as f:
            f.write(body)


def _run_capture(td):
    import io as _io
    import contextlib as _cl
    buf = _io.StringIO()
    with _cl.redirect_stdout(buf), _cl.redirect_stderr(buf):
        rc = run(td, False)
    return rc, buf.getvalue()


def self_test():
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as td:
        _mkfixture(td, _MAIN_GO, _WRITERS)
        reg_ok = {"命令": "help export", "授权类": "无闸",
                  "理由": "无闸 —— usage 只有落点旗标 --out、无授权旗标", "出处": "自检"}
        reg_impact = {"命令": "impact", "授权类": "无闸",
                      "理由": "无闸 —— usage 无授权旗标", "出处": "自检"}

        # 格1 正控：空登记 ⇒ help export + impact 会写盘却无闸又无白名单 ⇒ rc=1
        _write_reg(td, [])
        if run(td, False) != 1:
            print("  ✗ 格1失败：空登记应回 rc=1"); ok = False

        # 格2 ★缺陷①②负控：help export（只有落点旗标 --out）⇒ 登记「无闸」**不得**判「无闸不真」；
        #      impact（usage 是常量引用）⇒ 登记「无闸」**不得**判「命令面不存在」⇒ 全绿 rc=0
        _write_reg(td, [dict(reg_ok), dict(reg_impact)])
        rc2, out2 = _run_capture(td)
        if rc2 != 0:
            print("  ✗ 格2失败：两条都登记（--out 落点旗标 + 常量 usage）后应回 rc=0（实得 rc=%d）" % rc2); ok = False
        if "✗[假条目]" in out2:
            print("  ✗ 格2失败：出现假条目假判（out=%r）" % out2); ok = False

        # 格3 ★缺陷①负控（细）：只登记 impact ⇒ 不得出「命令面不存在」假条目 ⇒ rc=1 只余 help export
        _write_reg(td, [dict(reg_impact)])
        _v, fk, _u = validate_registry([{"命令": "impact", "授权类": "无闸", "理由": "无闸", "出处": "x"}],
                                       {"无闸"},
                                       {"impact": {"usage": "zerg impact <目标>", "usage_known": True, "has_danger": False}})
        if len(fk) != 0:
            print("  ✗ 格3失败：常量 usage 的 impact 被假判假条目（fk=%r）" % (fk,)); ok = False
        if run(td, False) != 1:
            print("  ✗ 格3失败：只登记 impact 应回 rc=1（help export 仍缺白名单）"); ok = False

        # 格4 负控：塞一条瞎写命令的假条目 ⇒ 仍被点名 ⇒ rc=1
        _write_reg(td, [dict(reg_ok), dict(reg_impact),
                        {"命令": "bogus no-such cmd", "授权类": "无闸", "理由": "无闸 —— 瞎写", "出处": "自检"}])
        if run(td, False) != 1:
            print("  ✗ 格4失败：假条目（瞎写命令）应回 rc=1"); ok = False
        _v, fk, _u = validate_registry([{"命令": "bogus no-such cmd", "授权类": "无闸",
                                         "理由": "无闸", "出处": "x"}], {"无闸"}, {})
        if len(fk) != 1 or "不存在" not in fk[0][1]:
            print("  ✗ 格4失败：假条目未被点名（fk=%r）" % (fk,)); ok = False

        # 格5 负控：把 usage 已声明**授权旗标** --yes 的 guarded 洗成「无闸」⇒ 无闸不真 ⇒ rc=1 且点名
        _write_reg(td, [{"命令": "guarded", "授权类": "无闸", "理由": "无闸 —— 洗白试试", "出处": "自检"}])
        if run(td, False) != 1:
            print("  ✗ 格5失败：无闸不真应回 rc=1"); ok = False
        _v, fk, _u = validate_registry([{"命令": "guarded", "授权类": "无闸", "理由": "无闸", "出处": "x"}],
                                       {"无闸"},
                                       {"guarded": {"usage": "zerg guarded [--yes]", "usage_known": True, "has_danger": True}})
        if len(fk) != 1 or "不真" not in fk[0][1]:
            print("  ✗ 格5失败：无闸不真未被点名（fk=%r）" % (fk,)); ok = False

        # 格6 ★缺陷②负控（单点）：usage 只有落点旗标 --out 的写面登记「无闸」⇒ **不判假**
        _v, fk, _u = validate_registry([{"命令": "help export", "授权类": "无闸", "理由": "无闸", "出处": "x"}],
                                       {"无闸"},
                                       {"help export": {"usage": "zerg help export [--out <目录>] [--dry-run]",
                                                        "usage_known": True, "has_danger": False}})
        if fk:
            print("  ✗ 格6失败：只有落点旗标 --out 的写面被判假（fk=%r）" % (fk,)); ok = False

        # 格7 空域：缺 main.go ⇒ rc=2
        if run(os.path.join(td, "nope"), False) != 2:
            print("  ✗ 格7失败：缺 main.go 未回 rc=2"); ok = False

    # 格8 ★缺陷①负控：usage 常量引用**查不到字面量** ⇒ 报「判不了」，**不得**假判「命令面不存在」
    with tempfile.TemporaryDirectory() as td2:
        _mkfixture(td2, _MAIN_GO_GHOST, _WRITER_GHOST)
        _write_reg(td2, [])
        rc8, out8 = _run_capture(td2)
        if rc8 != 1:
            print("  ✗ 格8失败：常量查不到的写面应回 rc=1（判不了）"); ok = False
        if "判不了" not in out8 or "命令面不存在" in out8:
            print("  ✗ 格8失败：应报「判不了」且不得假判「不存在」（out=%r）" % out8); ok = False

    # 格9 空域：登记表读不到（缺「写面登记」表）⇒ rc=2
    with tempfile.TemporaryDirectory() as td3:
        _mkfixture(td3, _MAIN_GO, _WRITERS)
        _write_reg_raw(td3, {"授权类闭集": [{"值": "无闸"}]})
        if run(td3, False) != 2:
            print("  ✗ 格9失败：登记表读不到未回 rc=2"); ok = False

    # 格10 空域：catalog() 解析出 0 条 ⇒ rc=2
    with tempfile.TemporaryDirectory() as td4:
        _mkfixture(td4, _MAIN_GO_EMPTY, {})
        _write_reg(td4, [])
        if run(td4, False) != 2:
            print("  ✗ 格10失败：catalog 0 条未回 rc=2"); ok = False

    print("自检 %s（10 格）" % ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


def main(argv=None):
    ap = argparse.ArgumentParser(description="命令面写面两面判（GAP-20260927-90 · authority-map 写面登记接入）")
    ap.add_argument("--root", default=os.environ.get("ZERG_ROOT", os.getcwd()))
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return run(args.root, args.json)


if __name__ == "__main__":
    sys.exit(main())
