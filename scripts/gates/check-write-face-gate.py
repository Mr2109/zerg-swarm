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

WRITE_FLAG_RE = re.compile(r"--(yes|confirm|out)\b")
USAGE_RE = re.compile(r'usage:\s*"([^"]*)"')
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


def catalog_entries(main_go):
    """粗解析 main.go 的 catalog 条目 {path, usage, danger?, run}。"""
    text = open(main_go, "r", encoding="utf-8").read()
    ents = []
    for block in re.findall(r"\{(?:[^{}]|\{[^{}]*\})*?\}", text):
        um = USAGE_RE.search(block)
        if not um:
            continue
        pm = PATH_RE.search(block)
        dm = DANGER_RE.search(block)
        rm = RUN_RE.search(block)
        path = ""
        if pm:
            path = " ".join(re.findall(r'"([^"]*)"', pm.group(1)))
        ents.append({"path": path, "usage": um.group(1),
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
    """校「写面登记」每条 ⇒ 回 (valid, fake)。

    valid = {命令: 授权类}（条目完整 ⇒ 充作白名单）。
    fake  = [(命令, 原因)]（假条目 = 逐条能指出的牙：命令不存在 / 授权类越集 / 理由·出处空 / 无闸不真）。
    """
    valid, fake, seen = {}, [], set()
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
        if cls_v == "无闸" and cat is not None and WRITE_FLAG_RE.search(cat["usage"]):
            probs.append("无闸声明不真：usage 已声明写旗标 ⇒ 该写面有闸，不得洗成无闸")
        if cls_v in ("D2", "D3") and cat is not None and not cat["has_danger"]:
            probs.append("%s 声明不真：catalog() 无 danger 档" % cls_v)
        if cmd:
            seen.add(cmd)
        if probs:
            fake.append((who, "；".join(probs)))
        else:
            valid[cmd] = cls_v
    return valid, fake


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
    entries = catalog_entries(main_go)
    if not entries:
        print("BLOCKED —— catalog() 解析出 0 条（空域）⇒ rc=2 不给结论", file=sys.stderr)
        return 2
    cat_by_path = {e["path"]: e for e in entries if e["path"]}
    valid, fake = validate_registry(reg or [], cls, cat_by_path)
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
        declares = bool(WRITE_FLAG_RE.search(e["usage"]))
        if cmd in waived:
            continue  # 已显式白名单（无闸）⇒ 不再是「未声明/未白名单」
        if writes and not declares and not e["has_danger"]:
            hits.append((cmd, "写面但 usage 未声明写旗标且无 danger（缺白名单）", e["usage"]))
        elif e["has_danger"] and not declares:
            hits.append((cmd, "已登记 danger 但 usage 未声明对应旗标", e["usage"]))
    rc = 1 if (hits or fake) else 0
    if as_json:
        print(json.dumps({
            "rc": rc,
            "registered": [{"cmd": c, "cls": k} for c, k in registered],
            "registered_by_class": by_cls,
            "unregistered": [{"cmd": c, "why": w} for c, w, _ in hits],
            "fake_entries": [{"cmd": c, "why": w} for c, w in fake],
        }, ensure_ascii=False))
    else:
        print("命令面写面「声明↔实现」两面判（GAP-20260927-90 · authority-map 写面登记接入）")
        print("catalog 条目 %d · 写面函数 %d · 已登记写面 = %d（%s）· 未声明/未白名单 = %d 条 · 假条目 = %d 条"
              % (len(entries), len(wfuncs), len(registered),
                 " · ".join("%s %d" % (k, len(v)) for k, v in sorted(by_cls.items())) or "无",
                 len(hits), len(fake)))
        for c, k in registered:
            print("  · [%s] %s" % (k, c))
        for c, w, u in hits:
            print("  ✗ %s：%s ⇒ usage=「%s」" % (c, w, u))
        for c, w in fake:
            print("  ✗[假条目] %s：%s" % (c, w))
        print("  ★ 登记面 = `core/internal/contract/authority-map.json` 的「写面登记」（白名单）；"
              "未登记而会写盘者仍判「未声明/未白名单」；表里假条目（命令不存在 / 授权类越集 /")
        print("    理由·出处空 / 无闸不真）逐条点名。")
    return rc


# ── 自检夹具 ──────────────────────────────────────────────────────────────
_MAIN_GO = (
    'package main\n'
    'var cat = []*command{\n'
    ' {path: []string{"help", "export"}, usage: "zerg help export", run: cmdHelpExport},\n'
    ' {path: []string{"impact"}, usage: "zerg impact <目标>", danger: &dangerSpec{...}, run: cmdImpact},\n'
    ' {path: []string{"guarded"}, usage: "zerg guarded [--yes]", danger: &dangerSpec{...}, run: cmdGuarded},\n'
    '}\n')
_WRITERS = {
    "family_help.go":
        'func cmdHelpExport(inv *invocation, o, e io.Writer) int {\n\tos.WriteFile("x", b, 0o644)\n\treturn 0\n}\n',
    "family_impact.go":
        'func cmdImpact(inv *invocation, o, e io.Writer) int {\n\tos.WriteFile("y", b, 0o644)\n\treturn 0\n}\n',
    "family_guarded.go":
        'func cmdGuarded(inv *invocation, o, e io.Writer) int {\n\tos.WriteFile("z", b, 0o644)\n\treturn 0\n}\n',
}


def _write_reg(td, entries):
    d = os.path.join(td, "core", "internal", "contract")
    os.makedirs(d, exist_ok=True)
    data = {"授权类闭集": [{"值": "D2"}, {"值": "D3"}, {"值": "无闸"}], "写面登记": entries}
    with open(os.path.join(d, "authority-map.json"), "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False)


def self_test():
    import tempfile
    ok = True
    with tempfile.TemporaryDirectory() as td:
        d = os.path.join(td, "core", "cmd", "zerg")
        os.makedirs(d)
        with open(os.path.join(d, "main.go"), "w", encoding="utf-8") as f:
            f.write(_MAIN_GO)
        for name, body in _WRITERS.items():
            with open(os.path.join(d, name), "w", encoding="utf-8") as f:
                f.write(body)
        reg_ok = {"命令": "help export", "授权类": "无闸", "理由": "无闸 —— 声明面无闸", "出处": "自检"}

        # 格1 正控：空登记 ⇒ help export + impact 两条未声明/未白名单 ⇒ rc=1
        _write_reg(td, [])
        if run(td, False) != 1:
            print("  ✗ 格1失败：空登记应回 rc=1"); ok = False

        # 格2 登记 help export（无闸）⇒ 只 impact 命中 ⇒ rc=1
        _write_reg(td, [dict(reg_ok)])
        if run(td, False) != 1:
            print("  ✗ 格2失败：登记一条后应回 rc=1（impact 仍未登记）"); ok = False

        # 格3 负控：登记 help export + impact ⇒ 0 未声明/未白名单、0 假条目 ⇒ rc=0
        _write_reg(td, [dict(reg_ok), {"命令": "impact", "授权类": "无闸",
                                       "理由": "无闸 —— 声明面无闸", "出处": "自检"}])
        if run(td, False) != 0:
            print("  ✗ 格3失败：两条都登记后应回 rc=0"); ok = False

        # 格4 负控：塞一条瞎写命令的假条目 ⇒ 假条目被指出 ⇒ rc=1
        _write_reg(td, [dict(reg_ok), {"命令": "impact", "授权类": "无闸",
                                       "理由": "无闸 —— 声明面无闸", "出处": "自检"},
                        {"命令": "bogus no-such cmd", "授权类": "无闸",
                         "理由": "无闸 —— 瞎写", "出处": "自检"}])
        if run(td, False) != 1:
            print("  ✗ 格4失败：假条目（瞎写命令）应回 rc=1"); ok = False
        _, _, rerr = load_write_registry(td)
        v, fk = validate_registry([{"命令": "bogus no-such cmd", "授权类": "无闸",
                                    "理由": "无闸", "出处": "x"}], {"无闸"}, {})
        if len(fk) != 1 or "不存在" not in fk[0][1]:
            print("  ✗ 格4失败：假条目未被点名（fk=%r）" % (fk,)); ok = False

        # 格5 负控：把一条 usage 已声明 --yes 的命令（guarded）洗成「无闸」⇒ 无闸不真 ⇒ rc=1 且点名
        _write_reg(td, [{"命令": "guarded", "授权类": "无闸",
                         "理由": "无闸 —— 洗白试试", "出处": "自检"}])
        if run(td, False) != 1:
            print("  ✗ 格5失败：无闸不真应回 rc=1"); ok = False
        v, fk = validate_registry([{"命令": "guarded", "授权类": "无闸",
                                    "理由": "无闸", "出处": "x"}], {"无闸"}, {"guarded": {"usage": "zerg guarded [--yes]", "has_danger": True}})
        if len(fk) != 1 or "不真" not in fk[0][1]:
            print("  ✗ 格5失败：无闸不真未被点名（fk=%r）" % (fk,)); ok = False

        # 格6 负控：缺 main.go ⇒ rc=2
        if run(os.path.join(td, "nope"), False) != 2:
            print("  ✗ 格6失败：缺语料未回 rc=2"); ok = False
    print("自检 %s（6 格）" % ("PASS" if ok else "FAIL"))
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
