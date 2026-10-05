#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-model-size-anchor.py — 名册 `size_bytes` 锚 ⟷ 盘上现读（本机层 · 只读）
                          ＋ 机群行 ⟷ 能力快照记录（registry）接线（棘轮）

为什么存在
----------
GAP-20260927-178 / -186 / -193：`gateway/fleet.yaml` 的每条模型候选可带 `size_bytes` 锚
（名册自带 · 秒级可判），但此前**没有任何消费点**去 stat 它 —— 锚写了没人看就等于没写。
本门就是那条「看」：逐条把 `size_bytes` 与盘上现读 `stat` 对拍（先用大小当廉价代理，
不符即早退；不符再来 sha256 —— 同 `core/internal/modelreg/integrity.go:118-124` 的次序）。

★ 与 `check-nodes-roster.py`（G 支 `--verify-sha`）分工：那条**读满整件**算 sha256（分钟级）；
  本门只 `stat`（秒级）。两条互补、不重叠：本门先挡「近名陷阱」（盘上 `21166757760` ≠
  example-35b-v2-1.5 的 `21725437056` 那类），roster 门再核内容。

★ 判据 E（fleet ⟷ registry 接线 · 棘轮）是本门第二面：路由表（`/api/fleet/models` ·
  读 `gateway/fleet.yaml` 的 `models:`）与能力快照（`/api/models/registry` · 读模型目录
  `<root>/manifests/`）的 id **规范化接线**此前无任何机读判据；缺一格 ⇒ 路由表里的
  `registry_id` 可指向**不存在的**快照记录而无人拦。真源 = `contract.ModelIDMap()`
  （go:embed `core/internal/contract/model-id-map.json` · `entries`），逐条点名。

判据（A–D）
-----------
  A. `models:` 段可解析（否则 rc=2 不给结论）。
  B. 带 `size_bytes` 的条目、其 `file:` 是**本机绝对路径且在盘** ⇒ stat 对拍（逐字相等）。
  C. 带 `size_bytes` 的条目、其 `file:` 读不到（远端 host / 件不在盘）⇒ **只报告「跳过」**——
     本机读不到 ⇒ 不给结论，不算红（照 `check-nodes-roster.py` 的断层口径）。
  D. 一条 `size_bytes` 都没有 ⇒ rc=2（**空转假覆盖** —— 锚面根本没落地，不许当绿）。
     有锚但**本机一条都核不到**（全为远端/缺件）⇒ 同样 rc=2（本机层给不出结论）。

判据（E · fleet ⟷ registry 接线 · 棘轮）
----------------------------------------
  真源 = `core/internal/contract/model-id-map.json` 的 `entries`（每条 `registry_id` ↔
  `fleet_ids`）；registry 记录 id = 模型目录 `<root>/manifests/<id>/` 的**子目录名**
  （同 `core/internal/modelreg/store.go` 的 List 口径：`id := idEnt.Name()`）。逐条判：
    E① **已带 registry_id 的 fleet 行 ⇒ 该 registry_id 必 ∈ registry 记录 id 集**
        （硬判 · 一条对不上即 rc=1 并点名「行 · registry_id · 现记录集」）。
    E①' 反向：真源 `fleet_ids` 必落在 fleet.yaml 模型册里（不引不存在的行）。
    E② **棘轮**：**带 registry_id 的行数不得低于** `REGISTRY_WIRING_RATCHET`
        （接线只许升不许降 —— 现读只有一部分行接了线，一上硬判「全接」必红）。
  ★ 棘轮现读（2026-10-05 逐字 · 收口重编+重启主控后）= **17**：`/api/fleet/models` = 30 行（逐候选展开），
    其中带 `registry_id` 的 17 行 = 真源 7 条 entries 逐条展开：gemma-4-26B×2 + example-26b-review×1 +
    example-35b-v2×2 + example-35b-v2-1.5-9b×3 + example-35b×2 + Qwen3.8-27B×2 + Ternary-Bonsai-2-27B×3 +
    Nemotron-3.5-Lightning×2（原 5 行 ⇒ 收口重编+重启主控后接线上 live）。
  ★ registry 记录目录读不到 ⇒ E① **判不了**（不给结论）· E② 仍判（棘轮只吃 fleet.yaml+真源）。
  ★ 本门**只读**：读 fleet.yaml / 真源 json / manifests 目录列举 —— 不起子进程、不写盘、不联网。

结论范围（read_layer × verdict_scope）
--------------------------------------
  本门全部读取面 = **本机**（本机盘上的 `gateway/fleet.yaml` + 本机 `stat` ＋ 本机
  `core/internal/contract/model-id-map.json` ＋ 本机模型目录 `manifests/`）⇒ 判词一律
  `verdict_scope=本机`，并**逐条列出本机读不到的远端条目**；远端节点的锚须由**各机自己**跑本门回答。

退码（与仓内既有门一致的三档）
------------------------------
  0 = 本机可读的锚全对 ＆ 接线判据全过（PASS · 本机层）
  1 = 有漂移（FAIL · 逐条给 现值 vs 锚值）／有非整数锚／接线缺陷（E①/E①'/E② 任一命中）
  2 = 不给结论（fleet.yaml 不在 / 解析不出 models / 无锚 / 本机一条都核不到 /
      真源读不到 / registry 记录目录读不到（E① 判不了）/ 自检不过）

用法
----
    python3 scripts/gates/check-model-size-anchor.py
    python3 scripts/gates/check-model-size-anchor.py --fleet <路径>
    python3 scripts/gates/check-model-size-anchor.py --idmap <model-id-map.json> --registry-dir <manifests 目录>
    python3 scripts/gates/check-model-size-anchor.py --list-rules
    python3 scripts/gates/check-model-size-anchor.py --self-test
    python3 scripts/gates/check-model-size-anchor.py --json
"""
import argparse
import json
import os
import sys
import tempfile

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
DEFAULT_FLEET = os.path.join(REPO_ROOT, "gateway", "fleet.yaml")

# 判据 E 的真源（go:embed `core/internal/contract/model-id-map.json` 的仓内路径）。
DEFAULT_IDMAP = os.path.join(REPO_ROOT, "core", "internal", "contract", "model-id-map.json")

# 判据 E② 的棘轮：带 registry_id 的 fleet 行数**下限**（只许升不许降）。
# ★ 逐字现读（2026-10-05 · 收口重编+重启主控后）= 17（/api/fleet/models 30 行里带
#   registry_id 的 17 行 = 真源 model-id-map.json 7 条 entries 逐条展开：gemma-4-26B×2 +
#   example-26b-review×1 + example-35b-v2×2 + example-35b-v2-1.5-9b×3 + example-35b×2 +
#   Qwen3.8-27B×2 + Ternary-Bonsai-2-27B×3 + Nemotron-3.5-Lightning×2）。
#   现读只有一部分行接了线 ⇒ 一上来就要求「全行必带 registry_id」必红；故取现读数当棘轮，
#   只机检「不得回退」。真源新增条目令行数上升时，须同批上调本值（写明是哪一批）。
#   ★ 上调沿革：5（DEV-1627 首版）→ 17（DEV-1663/1664 名册接线落 live 后 · GAP-20261005-67）。
REGISTRY_WIRING_RATCHET = 17


# ── 极简 YAML 读取（只解 models: 段的 flow-map / block-map，不引第三方库）──

def _split_top(s):
    """按顶层逗号切分（跳过引号内与 []{} 内的逗号）。"""
    out, buf, depth, q = [], [], 0, None
    for ch in s:
        if q:
            buf.append(ch)
            if ch == q:
                q = None
            continue
        if ch in "\"'":
            q = ch
            buf.append(ch)
            continue
        if ch in "[{":
            depth += 1
        elif ch in "]}":
            depth -= 1
        if ch == "," and depth == 0:
            out.append("".join(buf))
            buf = []
            continue
        buf.append(ch)
    if buf:
        out.append("".join(buf))
    return out


def _kv(s):
    i = s.find(":")
    if i < 0:
        return None
    k = s[:i].strip()
    v = s[i + 1:].strip()
    if len(v) >= 2 and v[0] == v[-1] and v[0] in "\"'":
        v = v[1:-1]
    return k, v


def _parse_flow(body):
    inner = body.strip()
    if inner.startswith("{"):
        inner = inner[1:]
    if inner.endswith("}"):
        inner = inner[:-1]
    d = {}
    for part in _split_top(inner):
        kv = _kv(part)
        if kv:
            d[kv[0]] = kv[1]
    return d


def parse_models(text):
    """返回 {模型名: [候选dict, ...]}。只认 `models:` 段。"""
    models = {}
    cur_name = None
    cur_cand = None
    in_models = False
    for raw in text.splitlines():
        line = raw.rstrip("\n")
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if not line.startswith(" "):  # 顶层键
            in_models = stripped.startswith("models:")
            cur_name = None
            cur_cand = None
            continue
        if not in_models:
            continue
        indent = len(line) - len(line.lstrip(" "))
        if indent == 2:
            if stripped.endswith(":"):
                cur_name = stripped[:-1].strip()
                models.setdefault(cur_name, [])
                cur_cand = None
            else:
                nm, _, rest = stripped.partition(":")
                cur_name = nm.strip()
                models.setdefault(cur_name, [])
                rest = rest.strip()
                models[cur_name].append(_parse_flow(rest) if rest.startswith("{") else {})
                cur_cand = models[cur_name][-1]
        elif indent >= 4 and cur_name is not None:
            if stripped.startswith("- "):
                body = stripped[2:].strip()
                if body.startswith("{"):
                    models[cur_name].append(_parse_flow(body))
                    cur_cand = models[cur_name][-1]
                else:
                    cur_cand = {}
                    models[cur_name].append(cur_cand)
                    kv = _kv(body)
                    if kv:
                        cur_cand[kv[0]] = kv[1]
            elif cur_cand is not None:
                kv = _kv(stripped)
                if kv:
                    cur_cand[kv[0]] = kv[1]
    return models


def check(models, base_dir=""):
    """逐条对拍；返回 [(kind, name, idx, host, file, anchor, actual, note)]。"""
    results = []
    for name in sorted(models):
        for idx, cand in enumerate(models[name]):
            raw = cand.get("size_bytes")
            if raw is None:
                continue
            host = cand.get("host", "")
            f = cand.get("file", "")
            try:
                anchor = int(str(raw).strip())
            except ValueError:
                results.append(("BAD", name, idx, host, f, str(raw), None, "size_bytes 非整数"))
                continue
            path = f if os.path.isabs(f) else os.path.join(base_dir, f)
            if not path or not os.path.exists(path):
                results.append(("SKIP", name, idx, host, f, anchor, None,
                                "本机读不到（远端 host 或件不在盘）"))
                continue
            actual = os.path.getsize(path)
            if actual == anchor:
                results.append(("OK", name, idx, host, f, anchor, actual, ""))
            else:
                results.append(("MISMATCH", name, idx, host, f, anchor, actual, ""))
    return results


def verdict(results):
    if not results:
        return 2
    if any(r[0] in ("MISMATCH", "BAD") for r in results):
        return 1
    if not any(r[0] == "OK" for r in results):
        return 2
    return 0


# ══════════════════════════════════════════════════════════════════════════
# 判据 E：fleet 行 ⟷ registry 记录 id 的接线（棘轮 · 只读）
# ══════════════════════════════════════════════════════════════════════════
def default_registry_dir():
    """模型目录的 manifests 目录：ZERG_MODELS_DIR 优先，缺省 ~/.zerg/models/manifests
    （同 core/internal/modelreg/store.go 的 DefaultModelsDir + ManifestsDir 口径）。"""
    root = (os.environ.get("ZERG_MODELS_DIR") or "").strip()
    if not root:
        root = os.path.join(os.path.expanduser("~"), ".zerg", "models")
    return os.path.join(root, "manifests")


def load_registry_entries(path):
    """真源 `model-id-map.json` → ({fleet_id: registry_id}, {registry_id: [fleet_id…]})。
       读不到 / 解析不出 / 无 entries ⇒ (None, None)（调用方按「不给结论」处置）。"""
    try:
        with open(path, encoding="utf-8") as fh:
            doc = json.load(fh)
    except (OSError, ValueError):
        return None, None
    ents = doc.get("entries")
    if not isinstance(ents, list):
        return None, None
    f2r, r2f = {}, {}
    for e in ents:
        if not isinstance(e, dict):
            continue
        rid = str(e.get("registry_id", "")).strip()
        if not rid:
            continue
        fids = [str(x) for x in (e.get("fleet_ids") or [])]
        r2f[rid] = fids
        for fid in fids:
            f2r[fid] = rid
    return f2r, r2f


def registry_record_ids(registry_dir):
    """manifests 目录下的**子目录名** = registry 记录 id 集（同 store.go List 口径）。
       目录不在 ⇒ None（判不了）；在 ⇒ 集合（可能为空）。"""
    if not os.path.isdir(registry_dir):
        return None
    ids = set()
    for name in os.listdir(registry_dir):
        if name.startswith("."):
            continue
        if os.path.isdir(os.path.join(registry_dir, name)):
            ids.add(name)
    return ids


def check_registry_wiring(models, f2r, r2f, reg_ids, ratchet):
    """判据 E 核心：→ (rc, findings, n_with)。findings = [文字, ...]。
       E①  已带 registry_id 的行 ⇒ registry_id ∈ reg_ids（reg_ids=None ⇒ 判不了 ⇒ rc=2）
       E①' 真源 fleet_ids ⊆ fleet.yaml 模型册
       E②  带 registry_id 的行数 ≥ ratchet（棘轮）"""
    findings = []
    defect = False

    # E①' 反向：真源里的 fleet_id 必在 fleet.yaml 模型册里
    for fid in sorted(f2r):
        if fid not in models:
            findings.append("E①' 真源 fleet_id `%s`（→ `%s`）不在 fleet.yaml 模型册里（引了不存在的行）"
                            % (fid, f2r[fid]))
            defect = True

    # 数「带 registry_id 的行」（逐候选展开，同 /api/fleet/models 一行一候选）
    rows_with = []
    for name in sorted(models):
        rid = f2r.get(name)
        if rid:
            for idx in range(len(models[name])):
                rows_with.append((name, idx, rid))
    n_with = len(rows_with)

    # E① 硬判：registry_id ∈ registry 记录 id 集
    if reg_ids is None:
        findings.append("E① registry 记录目录读不到 ⇒ 接线判不了（不给结论）：%s" % registry_dir_note)
    else:
        for name, idx, rid in rows_with:
            if rid not in reg_ids:
                findings.append("E① %s[#%d]：registry_id `%s` 不在 registry 记录 id 集 %s 里（引了不存在的快照记录）"
                                % (name, idx, rid, sorted(reg_ids)))
                defect = True

    # E② 棘轮
    if n_with < ratchet:
        findings.append("E② 带 registry_id 的行数 %d < 棘轮 %d（接线回退 —— 只许升不许降）" % (n_with, ratchet))
        defect = True

    if defect:
        return 1, findings, n_with
    if reg_ids is None:
        return 2, findings, n_with
    return 0, findings, n_with


# registry_dir_note 在一次主运行里被设成实际路径，供 E① 判不了时的点名（只读展示用）。
registry_dir_note = ""


def combine_rc(a, b):
    """两腿退码合并：红优先，其次不给结论，最后全绿。"""
    if a == 1 or b == 1:
        return 1
    if a == 2 or b == 2:
        return 2
    return 0


def render(results):
    lines = []
    for kind, name, idx, host, f, anchor, actual, note in results:
        if kind == "OK":
            lines.append("  ✅ %s[%d] host=%s size_bytes=%s == 现读 %s  %s"
                         % (name, idx, host, anchor, actual, f))
        elif kind == "MISMATCH":
            lines.append("  ❌ %s[%d] host=%s 锚=%s ≠ 现读 %s  %s（漂移）"
                         % (name, idx, host, anchor, actual, f))
        elif kind == "BAD":
            lines.append("  ❌ %s[%d] host=%s size_bytes=%r 不是整数  %s"
                         % (name, idx, host, anchor, f))
        else:
            lines.append("  ⚪ %s[%d] host=%s 锚=%s 跳过：%s  %s"
                         % (name, idx, host, anchor, note, f))
    return "\n".join(lines)


LIST_RULES = """check-model-size-anchor.py 判据 / 路径来源 / 退码
  A  可解析 `models:` 段（否则 rc=2）
  B  本机绝对路径的 `size_bytes` 锚 ⇒ stat 对拍（相等才算过）
  C  本机读不到的锚 ⇒ 只报告「跳过」（不给结论、不算红）
  D  无锚 ⇒ rc=2（空转假覆盖）；有锚但本机一条都核不到 ⇒ rc=2
  E  fleet 行 ⟷ registry 记录接线（棘轮）：① 已带 registry_id 的行 ⇒ registry_id ∈ 记录 id 集（硬判）；
     ①' 真源 fleet_ids ⊆ fleet.yaml 模型册；② 带 registry_id 的行数 ≥ 棘轮（现读 17 · 只许升不许降）
  路径：gateway/fleet.yaml（--fleet 可覆盖）· core/internal/contract/model-id-map.json（--idmap）·
       ~/.zerg/models/manifests（--registry-dir 或 ZERG_MODELS_DIR）
  退码：0 本机层全对＋接线全过 · 1 有漂移/非整数锚/接线缺陷 · 2 不给结论
"""


def _wiring_self_test(fails):
    """判据 E 的成对负控：**两层**都钉 —— ① 内部分析函数层（check_registry_wiring 的 rc）；
       ② 主入口层（main() 真吃的**进程退码** —— 防「牙只长在内部函数上」：只断言内部函数，
       即便 main() 把 leg_rc 丢了自检仍会全绿）。合成夹具 · tempfile · 不碰真数据/真接口。"""
    idmap_good = [{"registry_id": "R1", "fleet_ids": ["M"]}]
    models_txt = "models:\n  M:\n    - { host: Mr2109, file: /x.gguf, mem_gb: 1 }\n" \
                 "    - { host: edge-node, file: /y.gguf, mem_gb: 1 }\n"

    def run(label, entries, reg_subdirs, ratchet, want_rc):
        with tempfile.TemporaryDirectory() as td:
            idmap = os.path.join(td, "model-id-map.json")
            with open(idmap, "w", encoding="utf-8") as fh:
                json.dump({"schema": "z/test", "entries": entries}, fh, ensure_ascii=False)
            regdir = os.path.join(td, "manifests")
            if reg_subdirs is not None:
                os.makedirs(regdir)
                for sd in reg_subdirs:
                    os.makedirs(os.path.join(regdir, sd))
            global registry_dir_note
            registry_dir_note = regdir
            models = parse_models(models_txt)
            f2r, r2f = load_registry_entries(idmap)
            reg_ids = registry_record_ids(regdir)
            rc, findings, _n = check_registry_wiring(models, f2r, r2f, reg_ids, ratchet)
            status = "PASS" if rc == want_rc else "FAIL"
            if rc != want_rc:
                fails.append("wiring/%s：期望 rc=%d 实得 rc=%d（findings=%s）" % (label, want_rc, rc, findings))
            print("  [%s] wiring/%-16s 期望 rc=%d 实得 rc=%d" % (status, label, want_rc, rc))

    run("good", idmap_good, ["R1"], 2, 0)
    run("false-registry-id", [{"registry_id": "R2", "fleet_ids": ["M"]}], ["R1"], 2, 1)   # 负控①：假 registry_id 必红
    run("ratchet-bump", idmap_good, ["R1"], 3, 1)                                          # 负控②：棘轮+1 必红
    run("dangling-fleet-id", [{"registry_id": "R1", "fleet_ids": ["NOPE"]}], ["R1"], 0, 1)  # E①' 反向
    run("no-registry-dir", idmap_good, None, 2, 2)                                         # 记录目录不在 ⇒ 判不了
    registry_dir_note = ""

    # ── 主入口层（进程退码）：把两条成对负控钉在 main() 真吃的那个退码上 ──
    #    ★ 夹具造 17 行都映射到 R1 且锚都对 ⇒ 正控 anchor_rc=0、E①过、E②过(17==棘轮) ⇒ 进程 rc=0；
    #      再由「假 registry_id / 行数少 1」两条负控验证 leg_rc 真被 main() 并进进程退码。
    import io as _io
    import contextlib as _ctx
    with tempfile.TemporaryDirectory() as td:
        real = os.path.join(td, "m.gguf")
        with open(real, "wb") as fh:
            fh.write(b"z" * 321)
        fleet = os.path.join(td, "fleet.yaml")

        def write_fleet(n):
            ls = ["models:", "  M:"]
            for _ in range(n):
                ls.append("    - { host: Mr2109, file: %s, mem_gb: 1, size_bytes: 321 }" % real)
            with open(fleet, "w", encoding="utf-8") as fh:
                fh.write("\n".join(ls) + "\n")

        regdir = os.path.join(td, "manifests")
        os.makedirs(os.path.join(regdir, "R1"))
        idmap = os.path.join(td, "idmap.json")

        def write_idmap(rid):
            with open(idmap, "w", encoding="utf-8") as fh:
                json.dump({"schema": "z/t", "entries": [{"registry_id": rid, "fleet_ids": ["M"]}]},
                          fh, ensure_ascii=False)

        def proc(label, n, rid, want):
            write_fleet(n)
            write_idmap(rid)
            with _ctx.redirect_stdout(_io.StringIO()):
                rc = main(["--fleet", fleet, "--idmap", idmap, "--registry-dir", regdir])
            status = "PASS" if rc == want else "FAIL"
            if rc != want:
                fails.append("proc/%s：期望进程 rc=%d 实得 rc=%d" % (label, want, rc))
            print("  [%s] proc/%-20s 期望 rc=%d 实得 rc=%d" % (status, label, want, rc))

        proc("good(17行=棘轮)", 17, "R1", 0)
        proc("false-registry-id", 17, "R2", 1)   # 负控①：假 registry_id ⇒ **进程**必红
        proc("ratchet-short(16<17)", 16, "R1", 1)  # 负控②：棘轮不变、行数少 1 ⇒ **进程**必红


def self_test():
    ok = True
    with tempfile.TemporaryDirectory() as td:
        real = os.path.join(td, "w.gguf")
        with open(real, "wb") as fh:
            fh.write(b"x" * 1234)
        base = "models:\n  m1:\n    - { host: Mr2109, file: %s, mem_gb: 1, %%s }\n" % real
        cases = [
            ("good", base % "size_bytes: 1234", 0),
            ("bad", base % "size_bytes: 9999", 1),
            ("badd", base % "size_bytes: abc", 1),
            ("skip", "models:\n  m1:\n    - { host: edge-node, file: /nope/x.gguf, mem_gb: 1, size_bytes: 42 }\n", 2),
            ("none", base % "ctx_window: 8", 2),
            # block 形态（多行）
            ("block", "models:\n  m2:\n    - host: Mr2109\n      file: %s\n      size_bytes: 1234\n" % real, 0),
        ]
        for label, text, want in cases:
            rc = verdict(check(parse_models(text)))
            status = "PASS" if rc == want else "FAIL"
            if rc != want:
                ok = False
            print("  [%s] %-6s 期望 rc=%d 实得 rc=%d" % (status, label, want, rc))
    # 判据 E 的成对负控
    wiring_fails = []
    _wiring_self_test(wiring_fails)
    if wiring_fails:
        ok = False
        for f in wiring_fails:
            print("  [FAIL] " + f)
    print("自检 %s" % ("全绿" if ok else "有红"))
    return 0 if ok else 2


def main(argv=None):
    ap = argparse.ArgumentParser(description="名册 size_bytes 锚 ⟷ 盘上现读 ＋ fleet ⟷ registry 接线（本机层）")
    ap.add_argument("--fleet", default=DEFAULT_FLEET)
    ap.add_argument("--idmap", default=DEFAULT_IDMAP,
                    help="判据 E 真源 model-id-map.json（默认 %s）" % DEFAULT_IDMAP)
    ap.add_argument("--registry-dir", default=None,
                    help="registry 记录目录（manifests 根；默认 ZERG_MODELS_DIR 或 ~/.zerg/models 下的 manifests）")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args(argv)

    if args.list_rules:
        sys.stdout.write(LIST_RULES)
        return 0
    if args.self_test:
        return self_test()

    if not os.path.exists(args.fleet):
        sys.stderr.write("BLOCKED：fleet.yaml 不在盘上：%s（缺件 ⇒ 不给结论）\n" % args.fleet)
        return 2
    try:
        with open(args.fleet, encoding="utf-8") as fh:
            text = fh.read()
        models = parse_models(text)
    except Exception as e:  # noqa: BLE001
        sys.stderr.write("BLOCKED：解析失败：%s\n" % e)
        return 2
    if not models:
        sys.stderr.write("BLOCKED：解析不出 models: 段（不给结论）\n")
        return 2

    results = check(models)
    anchor_rc = verdict(results)

    # ── 判据 E：fleet ⟷ registry 接线（棘轮）──
    global registry_dir_note
    registry_dir = args.registry_dir if args.registry_dir else default_registry_dir()
    registry_dir_note = registry_dir
    f2r, r2f = load_registry_entries(args.idmap)
    n_with = 0
    if f2r is None:
        leg_rc, leg_findings = 2, ["E 真源 model-id-map 读不到/解析不出：%s ⇒ 接线判不了（不给结论）" % args.idmap]
    else:
        reg_ids = registry_record_ids(registry_dir)
        leg_rc, leg_findings, n_with = check_registry_wiring(
            models, f2r, r2f, reg_ids, REGISTRY_WIRING_RATCHET)

    rc = combine_rc(anchor_rc, leg_rc)

    n_ok = sum(1 for r in results if r[0] == "OK")
    n_bad = sum(1 for r in results if r[0] in ("MISMATCH", "BAD"))
    n_skip = sum(1 for r in results if r[0] == "SKIP")

    if args.json:
        print(json.dumps({
            "verdict_scope": "本机",
            "rc": rc,
            "anchors": len(results),
            "ok": n_ok, "mismatch": n_bad, "skipped": n_skip,
            "wiring": {"rc": leg_rc, "ratchet": REGISTRY_WIRING_RATCHET,
                       "rows_with_registry_id": n_with,
                       "registry_dir": registry_dir,
                       "findings": leg_findings},
            "items": [{"name": r[1], "idx": r[2], "host": r[3], "file": r[4],
                       "anchor": r[5], "actual": r[6], "kind": r[0]} for r in results],
        }, ensure_ascii=False, indent=2))
        return rc

    print("── 名册 size_bytes 锚 ⟷ 盘上现读 ＋ fleet ⟷ registry 接线（本机层 · 只读）──")
    print("取源：%s" % args.fleet)
    if results:
        print(render(results))
    print("锚 %d 条：对 %d · 漂移 %d · 跳过 %d" % (len(results), n_ok, n_bad, n_skip))
    print("接线（E）：带 registry_id 的行 %d · 棘轮 %d · registry 目录 %s"
          % (n_with, REGISTRY_WIRING_RATCHET, registry_dir))
    if leg_findings:
        for msg in leg_findings:
            print("  [%s] %s" % ("✗" if leg_rc == 1 else "⚪", msg))
    if rc == 0:
        print("PASS：本机可读的锚全对 ＋ 接线全过（verdict_scope=本机）")
    elif rc == 1:
        print("FAIL：有漂移/接线缺陷（逐条见上）")
    else:
        print("BLOCKED：本机层给不出结论（无锚 / 一条都核不到 / 真源或 registry 目录读不到 ⇒ 缺件 ≠ 绿）")
    return rc


if __name__ == "__main__":
    sys.exit(main())
