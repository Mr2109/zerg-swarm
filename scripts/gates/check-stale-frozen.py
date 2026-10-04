#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-stale-frozen.py —— 只读：列出盘上「冻结件」的**冻结值 vs 现读** ＋ 刷新正门。

缺口 `GAP-20261005-31`（P1 · 门禁面）首版。它治的是「门间连锁面无面看着」：
改一个被冻结的源件（判据件 / 步骤表 / 命令树用法串）后，下游多处冻结值各自变红
（judge-registry 缓存键 · affected-map 指纹 · cli-contract 计数 · help 面 lines/bytes ·
公开标记表 · SH_GATES 清册 · DEV_EDIT_SHA256），但**没有任何一处能同批列出**
「改了 X ⇒ 会红 Y,Z」—— 只能靠人记忆或等全量门（~25 min）发现。本件把每一处冻结面
**现读**出来，逐条印「冻结点 · 冻结值 · 现读 · 状态 · 刷新正门」。

★★ 只报告，不进退码：本件**默认 rc=0**（存量陈旧不得把它变恒红）；
   只有「用法错 / 仓根取不到」才 rc=2。要接成阻断闸另加 `--check` 档（首版不做）。

★ 每一处冻结面「怎么算过期」**逐件读它自己的比较逻辑**（派单硬约束：✗ 不新立口径）：
   · `scripts/gates/judge-registry.json`       → 每件 `sha256` 缓存键（`check-judge-registry.py --check` 第 3 条）
   · `scripts/gates/affected-map.json`         → `input_fingerprint`（`check-affected-map.py` 的 `fingerprint()`）
   · `scripts/gates/cli-contract-baseline.json`→ `commands` / `mustfail_cases`（`check-cli-contract.py` T5b/T5c）
   · `scripts/gates/check-help-shape.py`       → `BASELINE` 六格 lines/bytes（本件 `--emit-baseline` 刷）
   · `scripts/gates/check-approve-backfill.py` → 常量 `DEV_EDIT_SHA256`（本件 `--emit-constant` / `--refresh-constant`）
   · `scripts/gates/real-gates.sh`             → 字面 `SH_GATES` 清册 ≡ `SH_TABLE` 键集（棘轮第 ④ 条）
   · `scripts/公开标记.tsv`                    → 门件（`scripts/gates/*.py|*.sh` 直下）⇕ 表第一列
   · 两器排除三表 / 步骤表自检冻结计数 / 门⑩⑫ 语义件 —— 首版只列「取数命令」（未代跑）。

用法
  python3 check-stale-frozen.py [--root <仓根>] [--json <字段>]
退出码：0 只报告 · 2 用法错 / 仓根取不到（fail-closed）
"""
import argparse
import hashlib
import importlib.util
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))  # scripts/gates/ -> <仓根>

REFRESH = {
    "judge-registry": "zerg gate check check-judge-registry --emit",
    "affected-map": "python3 scripts/gates/check-affected-map.py --emit",
    "cli-contract": "python3 scripts/gates/check-cli-contract.py --emit-matrix（★ count 两键按收口批随动 · 门78/门⑪ 现读对拍）",
    "help-shape": "python3 scripts/gates/check-help-shape.py --emit-baseline（现读打印 · 照抄 · 禁手写）",
    "approve-backfill": "python3 scripts/gates/check-approve-backfill.py --refresh-constant（先干跑 · 加 --yes 才落）",
    "real-gates": "把新 .sh 门件同批登记进 real-gates.sh 的 SH_TABLE 与字面 SH_GATES（两边不许漂移）",
    "public-marks": "在 scripts/公开标记.tsv 补一行（文件\\tpublic\\t理由）",
}


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        h.update(f.read())
    return h.hexdigest()


def load_module(path):
    name = "_sf_" + os.path.splitext(os.path.basename(path))[0].replace("-", "_")
    spec = importlib.util.spec_from_file_location(name, path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def face(件, 冻结形态, 刷新正门):
    return {"件": 件, "冻结形态": 冻结形态, "状态": "一致", "冻结点": "", "现读": "",
            "过期数": 0, "明细": [], "刷新正门": 刷新正门}


def _stale(f, label, frozen, live, note=""):
    f["过期数"] += 1
    f["状态"] = "过期"
    f["明细"].append((label, frozen, live, note))


# ── ① judge-registry.json ──────────────────────────────────────────────
def probe_judge_registry(root):
    rel = "scripts/gates/judge-registry.json"
    f = face(rel, "每件 sha256 缓存键（cache_key = sha256|tool_version）＋ count",
             REFRESH["judge-registry"])
    p = os.path.join(root, rel)
    try:
        reg = json.load(open(p, encoding="utf-8"))
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "注册表读不到：%s" % e
        return f
    entries = reg.get("entries", [])
    f["冻结点"] = "entries %d 条 · count=%s" % (len(entries), reg.get("count"))
    f["现读"] = "entries %d 条（盘上现读件名表）" % len(entries)
    if reg.get("count") != len(entries):
        _stale(f, "count ⟷ len(entries)", reg.get("count"), len(entries), "自洽")
    for e in entries:
        rel_e = e.get("path")
        frozen = e.get("sha256")
        fp = os.path.join(root, rel_e)
        if not os.path.isfile(fp):
            _stale(f, rel_e, frozen, "（盘上无此件）", "表里有件盘上无")
            continue
        live = sha256_file(fp)
        if live != frozen:
            _stale(f, rel_e, frozen, live, "缓存键过期 ⇒ 门79 红")
    return f


# ── ② affected-map.json ────────────────────────────────────────────────
def probe_affected_map(root):
    rel = "scripts/gates/affected-map.json"
    f = face(rel, "input_fingerprint（入键=步表文本＋点名件 sha256＋config＋newcmd＋HEAD）· source_head",
             REFRESH["affected-map"])
    p = os.path.join(root, rel)
    try:
        mp = json.load(open(p, encoding="utf-8"))
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "声明表读不到：%s" % e
        return f
    f["冻结点"] = "input_fingerprint=%s · source_head=%s" % (
        str(mp.get("input_fingerprint"))[:16], mp.get("source_head"))
    try:
        mod = load_module(os.path.join(root, "scripts/gates/check-affected-map.py"))
        fresh = mod.build_map(root)
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "重算失败（%s）⇒ 不给结论" % e
        return f
    live_fp = fresh.get("input_fingerprint")
    live_head = fresh.get("source_head")
    f["现读"] = "input_fingerprint=%s · source_head=%s" % (str(live_fp)[:16], live_head)
    if mp.get("input_fingerprint") != live_fp:
        _stale(f, "input_fingerprint", str(mp.get("input_fingerprint"))[:16], str(live_fp)[:16],
               "步表/点名件/config/HEAD 任一变 ⇒ 过期 ⇒ affected-steps 报不给结论")
    if mp.get("source_head") != live_head:
        _stale(f, "source_head", mp.get("source_head"), live_head, "HEAD 变 ⇒ 过期")
    return f


# ── ③ cli-contract-baseline.json ───────────────────────────────────────
def probe_cli_contract(root):
    rel = "scripts/gates/cli-contract-baseline.json"
    f = face(rel, "commands（树条数相等判 T5c）· mustfail_cases（格级棘轮 T5b · 只许上调）· blanks（棘轮）",
             REFRESH["cli-contract"])
    p = os.path.join(root, rel)
    try:
        base = json.load(open(p, encoding="utf-8"))
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "基线读不到：%s" % e
        return f
    try:
        mod = load_module(os.path.join(root, "scripts/gates/check-cli-contract.py"))
        tree = mod.parse_tree(root)
        matrix = mod.load_matrix(root)
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "现读重算失败（%s）⇒ 不给结论" % e
        return f
    cases = matrix.get("cases", [])
    n_cmd = len(tree)
    n_mf = 0
    for c in cases:
        try:
            if int(c.get("want_rc")) != 0:
                n_mf += 1
        except (TypeError, ValueError):
            pass
    f["冻结点"] = "commands=%s · mustfail_cases=%s · blanks=%s" % (
        base.get("commands"), base.get("mustfail_cases"), base.get("blanks"))
    f["现读"] = "commands=%d · mustfail_cases=%d" % (n_cmd, n_mf)
    if base.get("commands") != n_cmd:
        _stale(f, "commands（T5c 相等判）", base.get("commands"), n_cmd, "命令树条数漂了 ⇒ 门⑪ 红")
    if base.get("mustfail_cases") != n_mf:
        _stale(f, "mustfail_cases（T5b 棘轮）", base.get("mustfail_cases"), n_mf, "只许上调")
    return f


# ── ④ check-help-shape.py BASELINE ─────────────────────────────────────
def probe_help_shape(root):
    rel = "scripts/gates/check-help-shape.py"
    f = face(rel, "BASELINE：help --all / help / help dangerous / help config 的 lines·bytes（(+)rows）",
             REFRESH["help-shape"])
    try:
        mod = load_module(os.path.join(root, rel))
        baseline = mod.BASELINE
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "BASELINE 读不到：%s" % e
        return f
    zbin = os.path.join(root, "bin", "zerg")
    if not os.path.isfile(zbin):
        f["状态"] = "取不到"
        f["现读"] = "bin/zerg 不在（%s）⇒ 取不到 help 面读数" % zbin
        return f
    faces = [("help --all", ["help", "--all"]), ("help", ["help"]),
             ("help dangerous", ["help", "dangerous"]), ("help config", ["help", "config"])]
    got = {}
    for key, argv in faces:
        try:
            out = subprocess.run([zbin] + argv, capture_output=True)
        except Exception as e:
            f["状态"] = "取不到"
            f["现读"] = "跑 bin/zerg %s 失败：%s" % (" ".join(argv), e)
            return f
        text = out.stdout.decode("utf-8", "replace")
        got[key] = (len(text.splitlines()), len(text.encode("utf-8")))
    f["冻结点"] = " · ".join("%s=%s/%sB" % (k, baseline.get(k, {}).get("lines"),
                                          baseline.get(k, {}).get("bytes")) for k, _ in faces)
    f["现读"] = " · ".join("%s=%d/%dB" % (k, got[k][0], got[k][1]) for k, _ in faces)
    for k, _ in faces:
        fl = baseline.get(k, {}).get("lines")
        fb = baseline.get(k, {}).get("bytes")
        gl, gb = got[k]
        if (fl, fb) != (gl, gb):
            _stale(f, "%s lines/bytes" % k, "%s/%sB" % (fl, fb), "%d/%dB" % (gl, gb),
                   "改命令树用法串 ⇒ 门69 红")
    return f


# ── ⑤ check-approve-backfill.py DEV_EDIT_SHA256 ────────────────────────
def probe_approve_backfill(root):
    rel = "scripts/gates/check-approve-backfill.py"
    f = face(rel, "常量 DEV_EDIT_SHA256 = 在册批准件 approvals/dev_edit.json 整件 sha256",
             REFRESH["approve-backfill"])
    try:
        mod = load_module(os.path.join(root, rel))
        frozen = mod.DEV_EDIT_SHA256
        ticket_rel = mod.TICKET_REL
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "件读不到：%s" % e
        return f
    state = os.environ.get("ZERG_STATE_DIR") or os.path.expanduser("~/.zerg/state")
    ticket = os.path.join(state, ticket_rel)
    f["冻结点"] = "DEV_EDIT_SHA256=%s…" % frozen[:16]
    if not os.path.isfile(ticket):
        f["现读"] = "在册批准件不在（%s）⇒ 未取数" % ticket
        f["状态"] = "取不到"
        return f
    live = sha256_file(ticket)
    f["现读"] = "现读件 sha=%s…" % live[:16]
    if live != frozen:
        _stale(f, "DEV_EDIT_SHA256", frozen[:16] + "…", live[:16] + "…",
               "人签换签 ⇒ 门⑰ 红 ⇒ 走 --refresh-constant")
    return f


# ── ⑥ real-gates.sh SH_GATES 清册 ──────────────────────────────────────
def probe_real_gates(root):
    rel = "scripts/gates/real-gates.sh"
    f = face(rel, "字面 SH_GATES 清册键集 ≡ SH_TABLE 键集（棘轮第 ④ 条 · 外部消费方走字面正则）",
             REFRESH["real-gates"])
    p = os.path.join(root, rel)
    try:
        src = open(p, encoding="utf-8").read()
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "件读不到：%s" % e
        return f
    m = re.search(r"SH_GATES=\((.*?)\)", src, re.S)
    tbl = re.search(r"SH_TABLE=\"\$\(cat <<'EOF'\n(.*?)\nEOF", src, re.S)
    if not m or not tbl:
        f["状态"] = "取不到"
        f["现读"] = "字面 SH_GATES / SH_TABLE 抠不出来 ⇒ 不给结论"
        return f
    gates = sorted(t for t in re.split(r"\s+", m.group(1).replace("\\", " ")) if t.strip())
    table = sorted(l.split("\t")[0] for l in tbl.group(1).splitlines() if l.strip())
    f["冻结点"] = "SH_GATES %d 键 · SH_TABLE %d 键" % (len(gates), len(table))
    f["现读"] = "两边键集%s" % ("逐字相等" if gates == table else "不等")
    if gates != table:
        _stale(f, "SH_GATES ⟷ SH_TABLE 键集",
               ",".join(sorted(set(gates) - set(table))) or "-",
               ",".join(sorted(set(table) - set(gates))) or "-",
               "只在一边 ⇒ 门65 rc=8「判不了」")
    return f


# ── ⑦ scripts/公开标记.tsv ─────────────────────────────────────────────
def probe_public_marks(root):
    rel = "scripts/公开标记.tsv"
    f = face(rel, "门件（scripts/gates/*.py|*.sh 直下）⇕ 表第一列（门件未登记 ⇒ 门「悬空/漏登记」）",
             REFRESH["public-marks"])
    p = os.path.join(root, rel)
    try:
        rows = [l.split("\t")[0] for l in open(p, encoding="utf-8").read().splitlines()[1:] if l.strip()]
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "标记表读不到：%s" % e
        return f
    gdir = os.path.join(root, "scripts", "gates")
    try:
        ondisk = sorted(n for n in os.listdir(gdir)
                        if (n.endswith(".py") or n.endswith(".sh")) and os.path.isfile(os.path.join(gdir, n)))
    except Exception as e:
        f["状态"] = "取不到"
        f["现读"] = "scripts/gates/ 列举失败：%s" % e
        return f
    declared = set(rows)
    missing = [n for n in ondisk if ("scripts/gates/" + n) not in declared]
    f["冻结点"] = "表 %d 行 · scripts/gates 直下门件 %d 件" % (len(rows), len(ondisk))
    f["现读"] = "未登记门件 %d 件" % len(missing)
    for n in missing:
        _stale(f, "scripts/gates/" + n, "（表里无此行）", "（盘上有此件）",
               "未登记 ⇒ 悬空/漏登记面红（首版只报此直下门件面）")
    return f


# ── ⑧ 取数命令面（首版未代跑）───────────────────────────────────────────
def probe_ref_cmds(root):
    rows = [
        ("scripts/gates/real-gates.sh / precommit-gates.sh", "步骤表自检冻结计数（全表步数 / 档位三数 / scope 步数 / 单步档全表命中）",
         "改 precommit-gates.sh 步骤表 ⇒ 该件自检当场红（半改即红）", "该件自检（`bash scripts/gates/precommit-gates.sh --list` ＋ 自检段）"),
        ("scripts/build/publish-public.sh ＋ publish/mirror-public-lib.py", "两器排除三表（EXCLUDES / DROP_PREFIX / DROP_EXACT）",
         "新增件未进三表 ⇒ release 面「镜像器仍会发它」红", "两器同步登记 + `check-publish-face-sync.py`"),
        ("core/internal/contract/dryrun-semantics.json / dev-targets.json", "门⑩ 干跑语义登记 / 门⑫ 目标闭集",
         "改这两件 ⇒ 门⑩/门⑫ 现读红", "`zerg gate check check-newfile-registry` / `check-dev-targets.py`"),
    ]
    for 件, 形态, note, refresh in rows:
        f = face(件, 形态, refresh)
        f["冻结点"] = "（首版未代跑）"
        f["现读"] = note
        f["状态"] = "取数命令"
        yield f


def collect(root):
    faces = [probe_judge_registry(root), probe_affected_map(root), probe_cli_contract(root),
             probe_help_shape(root), probe_approve_backfill(root), probe_real_gates(root),
             probe_public_marks(root)]
    faces.extend(probe_ref_cmds(root))
    return faces


def render(faces, out):
    n_stale = sum(f["过期数"] for f in faces)
    n_face_stale = sum(1 for f in faces if f["过期数"] > 0)
    print("冻结件 stale 报告（只读 · 只报告不进退码 · GAP-20261005-31 首版）", file=out)
    for f in faces:
        mark = {"一致": "✓", "过期": "✗", "取不到": "···", "取数命令": "·"}.get(f["状态"], "?")
        print("", file=out)
        print("── %s ──  冻结形态：%s" % (f["件"], f["冻结形态"]), file=out)
        print("   冻结点：%s" % f["冻结点"], file=out)
        print("   现读  ：%s" % f["现读"], file=out)
        print("   %s 状态：%s（过期 %d 处）" % (mark, f["状态"], f["过期数"]), file=out)
        for label, frozen, live, note in f["明细"]:
            print("       · %s" % label, file=out)
            print("           冻结 %s  ⟶  现读 %s   [%s]" % (frozen, live, note), file=out)
        print("   刷新正门：%s" % f["刷新正门"], file=out)
    print("", file=out)
    print("合计：冻结面 %d 个 · 其中过期 %d 个 · 过期点 %d 处（rc=0 只报告）"
          % (len(faces), n_face_stale, n_stale), file=out)
    return n_stale, n_face_stale


def main(argv=None):
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=DEFAULT_ROOT, help="仓根（默认由本件位置上溯两级的仓根）")
    ap.add_argument("--json", metavar="字段", nargs="?", const="", help="机器面（逗号分隔字段；缺省全量）")
    a = ap.parse_args(argv)
    root = os.path.abspath(a.root)
    if not os.path.isdir(os.path.join(root, "scripts", "gates")):
        print("⛔ 用法错（rc=2）：仓根取不到 scripts/gates/（--root=%s）" % root, file=sys.stderr)
        return 2
    faces = collect(root)
    n_stale, n_face_stale = render(faces, sys.stdout)
    if a.json is not None:
        payload = {"root": root, "faces": faces, "stale_points": n_stale,
                   "stale_faces": n_face_stale, "rc": 0, "mode": "report-only"}
        print(json.dumps(payload, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
