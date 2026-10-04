#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-model-size-anchor.py — 名册 `size_bytes` 锚 ⟷ 盘上现读（本机层 · 只读）

为什么存在
----------
GAP-20260927-178 / -186 / -193：`gateway/fleet.yaml` 的每条模型候选可带 `size_bytes` 锚
（名册自带 · 秒级可判），但此前**没有任何消费点**去 stat 它 —— 锚写了没人看就等于没写。
本门就是那条「看」：逐条把 `size_bytes` 与盘上现读 `stat` 对拍（先用大小当廉价代理，
不符即早退；不符再来 sha256 —— 同 `core/internal/modelreg/integrity.go:118-124` 的次序）。

★ 与 `check-nodes-roster.py`（G 支 `--verify-sha`）分工：那条**读满整件**算 sha256（分钟级）；
  本门只 `stat`（秒级）。两条互补、不重叠：本门先挡「近名陷阱」（盘上 `21166757760` ≠
  example-35b-v2-1.5 的 `21725437056` 那类），roster 门再核内容。

判据（A–D）
-----------
  A. `models:` 段可解析（否则 rc=2 不给结论）。
  B. 带 `size_bytes` 的条目、其 `file:` 是**本机绝对路径且在盘** ⇒ stat 对拍（逐字相等）。
  C. 带 `size_bytes` 的条目、其 `file:` 读不到（远端 host / 件不在盘）⇒ **只报告「跳过」**——
     本机读不到 ⇒ 不给结论，不算红（照 `check-nodes-roster.py` 的断层口径）。
  D. 一条 `size_bytes` 都没有 ⇒ rc=2（**空转假覆盖** —— 锚面根本没落地，不许当绿）。
     有锚但**本机一条都核不到**（全为远端/缺件）⇒ 同样 rc=2（本机层给不出结论）。

结论范围（read_layer × verdict_scope）
--------------------------------------
  本门全部读取面 = **本机**（本机盘上的 `gateway/fleet.yaml` + 本机 `stat`）⇒ 判词一律
  `verdict_scope=本机`，并**逐条列出本机读不到的远端条目**；远端节点的锚须由**各机自己**跑本门回答。

退码（与仓内既有门一致的三档）
------------------------------
  0 = 本机可读的锚全对（PASS · 本机层）
  1 = 有漂移（FAIL · 逐条给 现值 vs 锚值）／有非整数锚
  2 = 不给结论（fleet.yaml 不在 / 解析不出 models / 无锚 / 本机一条都核不到 / 自检不过）

用法
----
    python3 scripts/gates/check-model-size-anchor.py
    python3 scripts/gates/check-model-size-anchor.py --fleet <路径>
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
  路径：gateway/fleet.yaml（--fleet 可覆盖）
  退码：0 本机层全对 · 1 有漂移/非整数锚 · 2 不给结论
"""


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
    print("自检 %s" % ("全绿" if ok else "有红"))
    return 0 if ok else 2


def main(argv=None):
    ap = argparse.ArgumentParser(description="名册 size_bytes 锚 ⟷ 盘上现读（本机层）")
    ap.add_argument("--fleet", default=DEFAULT_FLEET)
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
    rc = verdict(results)
    n_ok = sum(1 for r in results if r[0] == "OK")
    n_bad = sum(1 for r in results if r[0] in ("MISMATCH", "BAD"))
    n_skip = sum(1 for r in results if r[0] == "SKIP")

    if args.json:
        print(json.dumps({
            "verdict_scope": "本机",
            "rc": rc, "anchors": len(results),
            "ok": n_ok, "mismatch": n_bad, "skipped": n_skip,
            "items": [{"name": r[1], "idx": r[2], "host": r[3], "file": r[4],
                       "anchor": r[5], "actual": r[6], "kind": r[0]} for r in results],
        }, ensure_ascii=False, indent=2))
        return rc

    print("── 名册 size_bytes 锚 ⟷ 盘上现读（本机层 · 只读）──")
    print("取源：%s" % args.fleet)
    if results:
        print(render(results))
    print("锚 %d 条：对 %d · 漂移 %d · 跳过 %d" % (len(results), n_ok, n_bad, n_skip))
    if rc == 0:
        print("PASS：本机可读的锚全对（verdict_scope=本机）")
    elif rc == 1:
        print("FAIL：有漂移（逐条见上）")
    else:
        print("BLOCKED：本机层给不出结论（无锚 / 一条都核不到 ⇒ 缺件 ≠ 绿）")
    return rc


if __name__ == "__main__":
    sys.exit(main())
