#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-wallclock-ratchet.py —— 墙钟棘轮（三档：日常 / 中 / 盖章）· **只许下降**

口径照抄仓内既有棘轮（`scripts/gates/baseline-ratchet.py` 的「唯一写通道 + append-only 审计 + 事后对账」）：

  · 不带写旗标 ⇒ **只报差**（现值 / 现测 / 将升将降几条），仓内件 sha256 一字不变；
  · 带写旗标（`--write`）⇒ 改数 **并且**往 append-only 台账追一条「件 · 旧值 → 新值 · 理由 · 时间」；
  · **上调 = 放宽判据** ⇒ 即使给了 `--reason` **还要**一枚显式 `--allow-raise`，否则 fail-closed rc=2 不写；
  · **无记录 ⇒ 判红**（现值 ≠ 台账最后一条 `new` ⇒ 有人手改）。

三档（件④ 定义）：
  daily  = 日常档（`zerg gate bench --fast` · 现读 31–37 步 · 父代理实测 31.4s）
  medium = 中档（默认全量 `--scope` 子集或 `zerg gate bench --scope go`，可自定义）
  stamp  = 盖章档（**全量 154 步 + 全冷跑**：断言 `CACHED` 计数 == 0；见件④ 正文）

现测值从哪来（**现跑**、不是常量）：
  `--from-bench <json>`：读 `zerg gate bench [--fast] --json real_ms` 的**现跑输出**（真形状：
  `{"schema":"zerg/v1","kind":"GateBench","items":[{"real_ms":"24763"},…],…}`）——
  `items[].real_ms` 是**毫秒字符串** ⇒ **/1000 得秒**（本件口径一律「秒」）；多趟取**最差**（max，拧紧）。
  （旧口径「取 `secs` 一栏」在现闭集里**不存在** —— `--json` 闭集真源 = `core/cmd/zerg/main.go`
   `GateBench` 条的 `fields` = `run,real_ms,user_ms,sys_ms,rc,log`，喂 `secs` 逐字报「未知字段」rc=2。）
  或 `--measured <秒>`：由调用方直接给（CI/人工复核用）。
  两样都不给 ⇒ **rc=2 不给结论**（不许拿记忆充数）。
  ★ 裸数字形态（整件就是个数字）**一律按秒读**（历史口径，未改）：要毫秒请给真 envelope。

基线件 `scripts/gates/wallclock-baseline.json`（schema `wallclock-ratchet/1`）：
  { "tiers": { "daily": {"secs": <num>|null, "_unmeasured": bool, "note": "…"}, … },
    "audit": "scripts/gates/wallclock-audit.jsonl" }
  ★ `_unmeasured=true` / `secs=null` ⇒ 本档**未标定** ⇒ 本件 rc=2（**不给结论**，不许悄悄绿）。
"""
import argparse, datetime, json, os, sys

HERE = os.path.dirname(os.path.abspath(__file__))
BASE = os.path.join(HERE, "wallclock-baseline.json")
TIERS = ("daily", "medium", "stamp")


def die(msg, rc=2):
    print(msg, file=sys.stderr)
    sys.exit(rc)


def load():
    if not os.path.isfile(BASE):
        die(f"✗ 缺基线件 {BASE} ⇒ 不给结论(rc=2)")
    with open(BASE, encoding="utf-8") as f:
        return json.load(f)


def audit_path(cfg):
    return os.path.join(HERE, os.path.basename(cfg.get("audit", "wallclock-audit.jsonl")))


def last_audit(path):
    if not os.path.isfile(path):
        return None
    last = None
    with open(path, encoding="utf-8") as f:
        for ln in f:
            ln = ln.strip()
            if ln:
                try:
                    last = json.loads(ln)
                except json.JSONDecodeError:
                    pass
    return last


def measured_secs(cfg, tier, args):
    if args.measured is not None:
        return float(args.measured)
    if args.from_bench:
        try:
            with open(args.from_bench, encoding="utf-8") as f:
                raw = f.read()
            try:
                data = json.loads(raw)
            except json.JSONDecodeError:
                # 允许 `zerg gate bench --json secs` 的裸数字形态
                data = float(raw.strip())
            if isinstance(data, dict):
                # ── 2026-09-29 收口：`zerg gate bench` 的**真**输出形状 ──────────────────
                #   {"schema":"zerg/v1","kind":"GateBench","items":[{"real_ms":"24763"},…]}
                #   `items[].real_ms` = **毫秒字符串** ⇒ /1000 得秒；多趟取 max（拧紧，非中位）。
                #   （`--json real_ms` 是闭集里唯一给墙钟的字段；`secs` 不在闭集里。）
                items = data.get("items")
                if isinstance(items, list) and items:
                    ms = []
                    for it in items:
                        if isinstance(it, dict) and it.get("real_ms") not in (None, ""):
                            try:
                                ms.append(float(it["real_ms"]))
                            except (TypeError, ValueError):
                                die(f"✗ --from-bench 的 items[].real_ms 读不出数：{it.get('real_ms')!r} ⇒ 不给结论(2)")
                    if not ms:
                        die("✗ --from-bench 的 items 里一条可用 real_ms 都没有 ⇒ 不给结论(2)")
                    return max(ms) / 1000.0
                if data.get("real_ms") not in (None, ""):
                    try:
                        return float(data["real_ms"]) / 1000.0
                    except (TypeError, ValueError):
                        die(f"✗ --from-bench 的 real_ms 读不出数：{data.get('real_ms')!r} ⇒ 不给结论(2)")
                for k in ("secs", "seconds", "wall"):
                    if k in data:
                        return float(data[k])
                # `steps=[{secs:…}]` 形态 ⇒ 求和（分片前的口径）
                if "steps" in data:
                    return float(sum(float(s.get("secs", 0)) for s in data["steps"]))
                die(f"✗ --from-bench 的 json 里找不到 secs/seconds/wall/steps ⇒ 不给结论(2)")
            return float(data)
        except (OSError, ValueError, TypeError) as e:
            die(f"✗ 读不了 --from-bench {args.from_bench}: {e} ⇒ 不给结论(2)")
    die("✗ 既没给 --measured 也没给 --from-bench ⇒ 不给结论(2)（不许拿记忆充数）")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tier", choices=TIERS, default="daily")
    ap.add_argument("--measured", type=float, default=None)
    ap.add_argument("--from-bench", default=None)
    ap.add_argument("--diff", action="store_true")
    ap.add_argument("--check", action="store_true")
    ap.add_argument("--write", action="store_true")
    ap.add_argument("--reason", default="")
    ap.add_argument("--allow-raise", action="store_true")
    ap.add_argument("--json", default="")
    args = ap.parse_args()

    cfg = load()
    tiers = cfg.get("tiers", {})
    if args.tier not in tiers:
        die(f"✗ 基线件里没有 `{args.tier}` 档 ⇒ 不给结论(2)")
    t = tiers[args.tier]
    cur = t.get("secs")
    unmeasured = bool(t.get("_unmeasured")) or cur is None

    # ── 事后对账（--check）：现值必须 == 台账最后一条 new（否则「有人手改」）──────
    last = last_audit(audit_path(cfg))
    if args.check:
        if last is None:
            die(f"✗ --check：台账 {audit_path(cfg)} 无记录，而基线件有值 ⇒ 判红(1)", rc=1)
        if last.get("tier") != args.tier:
            die(f"✗ --check：台账最后一条是 `{last.get('tier')}` 档，与 --tier {args.tier} 不符 ⇒ 判红(1)", rc=1)
        if float(last.get("new")) != float(cur):
            die(f"✗ --check：现值 {cur} ≠ 台账最后一条 new {last.get('new')} ⇒ **有人手改** ⇒ 判红(1)", rc=1)
        print(f"✓ --check 过：{args.tier} 现值 {cur} == 台账最后一条 new")
        return 0

    if unmeasured:
        die(f"✗ `{args.tier}` 档**未标定**（secs={cur} · _unmeasured={bool(t.get('_unmeasured'))}）"
            f" ⇒ 不给结论(rc=2)：先在有门跑窗口实测并 `--write` 标定，不许静默当绿")

    ms = measured_secs(cfg, args.tier, args)
    print(f"[{args.tier}] 现值 {cur}s · 现测 {ms}s")
    if args.diff:
        d = ms - float(cur)
        print(f"差 {d:+.2f}s —— {'将升（放宽判据，需 --allow-raise）' if d > 0 else '将降/持平（棘轮允许）'}")
        if args.json:
            print(json.dumps({"tier": args.tier, "cur": cur, "measured": ms, "delta": d}, ensure_ascii=False))
        return 0

    raising = ms > float(cur)
    if raising and not args.write:
        die(f"✗ 现测 {ms}s > 现值 {cur}s ⇒ **棘轮只许下降**（rc=1）", rc=1)
    if raising and not args.allow_raise:
        die(f"✗ 上调 = 放宽判据：给了 --write 还要 --allow-raise 才写（rc=2 · fail-closed，不写）")
    if raising and not args.reason:
        die("✗ 上调必须带 --reason（留痕）⇒ 不给结论(rc=2)")
    if not args.write:
        if ms <= float(cur):
            print(f"✓ 现测 {ms}s ≤ 现值 {cur}s（棘轮 OK）")
            return 0
        return 0

    # ── 写通道：改数 + append-only 台账 ─────────────────────────────────
    if not args.reason and not raising:
        die("✗ --write 必须带 --reason（唯一写通道要留痕）⇒ 不给结论(rc=2)")
    new = ms
    t["secs"] = round(new, 3)
    t.pop("_unmeasured", None)
    t["updated"] = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    with open(BASE, "w", encoding="utf-8") as f:
        json.dump(cfg, f, ensure_ascii=False, indent=1)
        f.write("\n")
    rec = {"t": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
           "tier": args.tier, "old": cur, "new": t["secs"], "raise": raising, "reason": args.reason,
           "by": os.environ.get("USER", "?")}
    with open(audit_path(cfg), "a", encoding="utf-8") as f:
        f.write(json.dumps(rec, ensure_ascii=False) + "\n")
    print(f"✓ 已写 {args.tier}: {cur} → {t['secs']} · 台账 +1 行（{audit_path(cfg)}）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
