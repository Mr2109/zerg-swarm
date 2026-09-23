#!/usr/bin/env python3
# check-contract-registry.py — 门禁：契约登记表「表 ↔ 件」一致性（任务单 v2.5.12 `波8` · 组4 §二.1 `W-14`）。
#
# 为什么有这只门（缺口的原话）：`core/internal/contract/registry.json` 第 3 行自陈
# 「门禁 scripts/gates/check-contract-registry.py 读同一份做「表 ↔ 件」一致性检查」，
# 而**该件当初不存在** —— 「表自称有门在读它、而门不在」= 表自己在说谎（`W-14`）。
# 本件把它补上：门在盘，且**真的读这一份**。
#
# 八条规则（任一不通过 ⇒ rc=1；读不到表 / 解析不到根 ⇒ rc=2 **不给结论**）：
#   R1 表可读 + 顶层结构齐（`schema` / `entries` 非空 / `note` / `authority` 三键）
#   R2 每条条目**八字段**齐且非空（id / name / truth / truth_shape / version_field / gate / change_class / change_note）
#   R3 `id` 唯一 + 形状 `^S-[a-z]$`（沿用本表现有的号段形态；新号段要同批改这一条正则）
#   R4 `change_class` ∈ 闭集 {A, B}
#   R5 `truth` 件在盘（`truth` 里的路径部分 —— `（…）` 之前的、或路径形状的那一段）
#   R6 `gate` 面**不许空口**：路径型 token 逐条在盘；非路径型必须写明 `go test`（那是「由 core/agent 的单测覆盖」这一档的**唯一合法形态**）
#      ★ 这就是负控要打的那条：**表加一条无门的 entry ⇒ 判红**
#   R7 表自称的门**在盘且就是本脚本**（`note` 点名的 `scripts/gates/*.py` ⇒ 必须等于本件；防「表说 A、跑的是 B」）
#   R8 Go 侧声明与表**同一路径**（`core/cmd/zerg/family_impact_contract.go` 的 `impactContractRegistryGateRel`）——
#      两处声明同一真源，任一处漂了都红
#
# 纪律：
#   · 本门**只读**（不写表 / 不写状态目录 / 不落审计 / 不改件）；
#   · 自带 `--self-test`（成对：正控 1 组 + 负控 7 组）——「永远报红」与「永远不报红」一样没用；
#   · 缺读数一律 rc=2（不给结论），**不许**把「读不到」当绿。
#
# 用法：
#   python3 scripts/gates/check-contract-registry.py                # 默认在仓内自动定位根
#   python3 scripts/gates/check-contract-registry.py --root <仓根>
#   python3 scripts/gates/check-contract-registry.py --registry <表路径>   # 夹具用（负控造一份改坏的表）
#   python3 scripts/gates/check-contract-registry.py --json <字段,逗号分隔>  # 字段闭集：entries,reds,gate,wired
#   python3 scripts/gates/check-contract-registry.py --self-test     # 自检（成对负控）
import argparse
import json
import os
import re
import shutil
import sys
import tempfile

REGISTRY_REL = os.path.join("core", "internal", "contract", "registry.json")
GO_DECL_REL = os.path.join("core", "cmd", "zerg", "family_impact_contract.go")
GO_DECL_CONST = "impactContractRegistryGateRel"
SCHEMA_ID = "zerg-contract-registry/1"
REQUIRED_ENTRY_FIELDS = ["id", "name", "truth", "truth_shape",
                         "version_field", "gate", "change_class", "change_note"]
CHANGE_CLASSES = ("A", "B")
ID_RE = re.compile(r"^S-[a-z]$")
# 路径形状 token：带目录分隔或已知后缀的一段（表里现用的两种写法都在面内）。
TOKEN_RE = re.compile(r"[A-Za-z0-9_./\u4e00-\u9fff-]*[A-Za-z0-9_/-]+\.(?:py|sh|go|json|md|ts|rs|toml)")
ROOT_ANCHOR = os.path.join("core", "internal", "version", "version.go")
JSON_FIELDS = ("entries", "reds", "gate", "wired")


def find_root(start):
    d = os.path.abspath(start)
    for _ in range(8):
        if os.path.isfile(os.path.join(d, ROOT_ANCHOR)):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return None


def path_part(truth):
    """从 `truth` 里取出可判在盘的那一段路径（表里两种写法：纯路径 / 路径 + `（…）` 说明）。"""
    t = (truth or "").strip()
    t = t.split("（")[0].strip()
    t = t.split("(")[0].strip()
    return t


def gate_tokens(gate):
    """`gate` 面里的路径型 token（含 `·` 分隔的多道门）。"""
    return [t for t in TOKEN_RE.findall(gate or "") if t]


def check(root, registry_path):
    """⇒ (rc, lines, info)。rc=2 = 读不到 / 根解析不到（不给结论）。"""
    lines = []
    info = {"entries": 0, "reds": 0, "gate": "", "wired": False}
    if root is None:
        return 2, ["⛔ 不给结论：解析不到仓根（找 %s 失败）—— 设 ZERG_REPO=<仓根>" % ROOT_ANCHOR], info
    if not os.path.isfile(registry_path):
        return 2, ["⛔ 不给结论：契约登记表读不到（%s）—— 表不在盘上 ⇒ 「表 ↔ 件」无法判" % registry_path], info
    try:
        with open(registry_path, encoding="utf-8") as f:
            doc = json.load(f)
    except Exception as e:  # noqa: BLE001 —— 读不了就是不给结论，不猜
        return 2, ["⛔ 不给结论：契约登记表解析失败（%s）：%s" % (registry_path, e)], info

    reds = []

    # ── R1 顶层结构 ─────────────────────────────────────────────────────────
    if doc.get("schema") != SCHEMA_ID:
        reds.append("R1 顶层 `schema` 不是 %r（实得 %r）" % (SCHEMA_ID, doc.get("schema")))
    for k in ("note", "authority"):
        if not doc.get(k):
            reds.append("R1 顶层缺 `%s`（登记表要写清「真源 / 导出物 / 规则」三键）" % k)
    entries = doc.get("entries")
    if not isinstance(entries, list) or not entries:
        reds.append("R1 `entries` 不是非空数组（空表 = 空转 = 假覆盖）")
        return 1, reds, info
    info["entries"] = len(entries)

    # ── R2/R3/R4/R5/R6 逐条 ─────────────────────────────────────────────────
    seen = {}
    for i, e in enumerate(entries):
        eid = e.get("id", "<无 id>")
        for f in REQUIRED_ENTRY_FIELDS:
            if not str(e.get(f, "")).strip():
                reds.append("R2 %s 缺字段 `%s`" % (eid, f))
        # R3
        if not ID_RE.match(str(e.get("id", ""))):
            reds.append("R3 %s 的 id 形状不合 `^S-[a-z]$`" % eid)
        if eid in seen:
            reds.append("R3 id 重复：%s（第 %d 条与第 %d 条）" % (eid, seen[eid] + 1, i + 1))
        seen[eid] = i
        # R4
        if e.get("change_class") not in CHANGE_CLASSES:
            reds.append("R4 %s 的 change_class=%r 不在闭集 %s" % (eid, e.get("change_class"), list(CHANGE_CLASSES)))
        # R5
        tp = path_part(e.get("truth", ""))
        if tp and not os.path.exists(os.path.join(root, tp)):
            reds.append("R5 %s 的 truth 件不在盘：%s" % (eid, tp))
        # R6 —— ★ 负控面：「无门的 entry」在这里被判红
        gate = str(e.get("gate", "")).strip()
        toks = gate_tokens(gate)
        if not gate:
            reds.append("R6 %s 的 `gate` 是空的 ⇒ 无门的 entry（表指向一个没人判的契约）" % eid)
        elif toks:
            for t in toks:
                if not os.path.isfile(os.path.join(root, t)):
                    reds.append("R6 %s 的 gate 件不在盘：%s" % (eid, t))
        elif "go test" not in gate:
            reds.append("R6 %s 的 `gate` 既不是路径型、也不含 `go test`：%r ⇒ 判不出「谁在读它」" % (eid, gate))

    # ── R7 表自称的门在盘且就是本脚本 ────────────────────────────────────────
    note = str(doc.get("note", ""))
    self_rel = "scripts/gates/check-contract-registry.py"
    named = [t for t in gate_tokens(note) if t.endswith("check-contract-registry.py")]
    if not named:
        reds.append("R7 表的 `note` 没点名 `scripts/gates/check-contract-registry.py`（表自称有门这一句不见了）")
    else:
        info["gate"] = named[0]
        if named[0] != self_rel:
            reds.append("R7 表自称的门是 %s，本件是 %s（表 ↔ 件不是同一份）" % (named[0], self_rel))
        if not os.path.isfile(os.path.join(root, self_rel)):
            reds.append("R7 表自称的门件不在盘：%s" % self_rel)
        else:
            info["wired"] = True

    # ── R8 Go 侧声明同一路径 ────────────────────────────────────────────────
    gp = os.path.join(root, GO_DECL_REL)
    if not os.path.isfile(gp):
        reds.append("R8 Go 侧声明件不在盘：%s" % GO_DECL_REL)
    else:
        src = open(gp, encoding="utf-8").read()
        m = re.search(re.escape(GO_DECL_CONST) + r'\s*=\s*"([^"]+)"', src)
        if not m:
            reds.append("R8 Go 侧 %s 的 %s 声明读不到" % (GO_DECL_REL, GO_DECL_CONST))
        elif m.group(1) != self_rel:
            reds.append("R8 Go 侧声明 %s，表自称 %s（两处漂了）" % (m.group(1), self_rel))

    info["reds"] = len(reds)
    if reds:
        lines = reds
        return 1, lines, info
    lines = ["契约登记表 OK：%d 条条目 · 八字段齐 · id 唯一 · truth 件全在盘 · gate 面全可判（无门 0 条）"
             % info["entries"],
             "表自称的门在盘：%s（= 本件）· Go 侧声明同一路径" % info["gate"]]
    return 0, lines, info


def self_test(root):
    """成对自检：1 组正控（真表）+ 7 组负控（逐条打坏 ⇒ 必须判红 / 不给结论）。"""
    bad, n = [], 0

    def case(name, want_rc, got_rc, want_red=None, lines=None):
        nonlocal n
        n += 1
        if got_rc != want_rc:
            bad.append("✗ %s：want rc=%d got rc=%d" % (name, want_rc, got_rc))
        elif want_red and not any(want_red in l for l in (lines or [])):
            bad.append("✗ %s：rc 对了但判词里没有 %r" % (name, want_red))

    real = os.path.join(root, REGISTRY_REL)
    rc, lines, _ = check(root, real)
    case("① 正控：真表 ⇒ rc=0", 0, rc, None, lines)
    if rc == 0 and not any("无门 0 条" in l for l in lines):
        bad.append("✗ ① 正控：该印「无门 0 条」")

    tmp = tempfile.mkdtemp(prefix="zerg-cr-selftest-")
    try:
        with open(real, encoding="utf-8") as f:
            good = json.load(f)

        def fixture(name, mutate):
            doc = json.loads(json.dumps(good, ensure_ascii=False))
            mutate(doc)
            p = os.path.join(tmp, name)
            with open(p, "w", encoding="utf-8") as fh:
                json.dump(doc, fh, ensure_ascii=False, indent=1)
            return check(root, p)

        rc, lines, _ = fixture("no-gate.json", lambda d: d["entries"].append(
            {"id": "S-z", "name": "无门的条目", "truth": "core/internal/contract/reap.json",
             "truth_shape": "x", "version_field": "schema", "gate": "",
             "change_class": "B", "change_note": "x"}))
        case("② 负控：加一条**无门**的 entry ⇒ rc=1", 1, rc, "S-z", lines)

        rc, lines, _ = fixture("ghost-gate.json", lambda d: d["entries"][0].update(
            {"gate": "scripts/gates/check-nonexistent-xyz.py"}))
        case("③ 负控：gate 指向不存在的件 ⇒ rc=1", 1, rc, "不在盘", lines)

        rc, lines, _ = fixture("missing-field.json", lambda d: d["entries"][1].pop("change_note"))
        case("④ 负控：缺一个字段 ⇒ rc=1", 1, rc, "缺字段", lines)

        rc, lines, _ = fixture("dup-id.json", lambda d: d["entries"].append(
            dict(d["entries"][0])))
        case("⑤ 负控：id 重复 ⇒ rc=1", 1, rc, "id 重复", lines)

        rc, lines, _ = fixture("ghost-truth.json", lambda d: d["entries"][0].update(
            {"truth": "core/internal/contract/这件的真源不在盘.json"}))
        case("⑥ 负控：truth 件不在盘 ⇒ rc=1", 1, rc, "truth 件不在盘", lines)

        rc, lines, _ = fixture("gate-blank-word.json", lambda d: d["entries"][1].update(
            {"gate": "由某个没人知道的东西覆盖"}))
        case("⑦ 负控：gate 既非路径也不含 `go test` ⇒ rc=1", 1, rc, "判不出", lines)

        rc, lines, _ = fixture("bad-schema.json", lambda d: d.update({"schema": "别的号"}))
        case("⑧ 负控：schema 号不对 ⇒ rc=1", 1, rc, "schema", lines)

        rc, lines, _ = check(root, os.path.join(tmp, "根本没这份表.json"))
        case("⑨ 负控：表读不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)

        rc, lines, _ = check(None, real)
        case("⑩ 负控：根解析不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    if bad:
        print("✗ 自检失败（%d 组断言未过 / 共 %d 组）：" % (len(bad), n))
        for b in bad:
            print("   " + b)
        return 1
    print("✓ 自检全过（%d 组：正控 1 + 负控 9 · 成对）" % n)
    return 0


def main():
    ap = argparse.ArgumentParser(description="契约登记表「表 ↔ 件」一致性门（W-14）")
    ap.add_argument("--root", default=None, help="仓库根（默认自动定位：找 %s）" % ROOT_ANCHOR)
    ap.add_argument("--registry", default=None, help="登记表路径（默认 <根>/%s；夹具用）" % REGISTRY_REL)
    ap.add_argument("--json", default=None, help="机器面字段（逗号分隔 · 闭集：%s）" % ",".join(JSON_FIELDS))
    ap.add_argument("--self-test", "--selftest", dest="self_test", action="store_true",
                    help="只跑自检（合成表 · 不碰真目标）")
    a = ap.parse_args()

    root = find_root(a.root or os.environ.get("ZERG_REPO") or os.path.dirname(os.path.abspath(__file__)))
    if a.json is not None:
        want = [f for f in a.json.split(",") if f]
        unknown = [f for f in want if f not in JSON_FIELDS]
        if not want or unknown:
            print("✗ `--json` 字段表外：%s（可选字段：%s）" % (",".join(unknown) or "（空）", ",".join(JSON_FIELDS)),
                  file=sys.stderr)
            return 2

    reg = a.registry or (os.path.join(root, REGISTRY_REL) if root else REGISTRY_REL)
    if a.self_test:
        print("门 · 契约登记表「表 ↔ 件」一致性（W-14）· 自检档（正控真表只读 + 合成负控 · 不碰真目标）")
        return self_test(root)
    rc, lines, info = check(root, reg)
    if a.json is not None:
        print(json.dumps({f: info[f] for f in a.json.split(",") if f in JSON_FIELDS},
                         ensure_ascii=False, sort_keys=True))
        return rc
    print("门 · 契约登记表「表 ↔ 件」一致性（W-14）· 仓根 %s" % root)
    print("  表：%s" % reg)
    for l in lines:
        print("  " + l)
    if rc == 0:
        print("⇒ 表 ↔ 件一致（rc=0）")
    elif rc == 1:
        print("⇒ 表 ↔ 件不一致（rc=1）：逐条见上")
    return rc


if __name__ == "__main__":
    sys.exit(main())
