#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""稳定性统计口径工具（T7.3）：**分歧率 + Clopper-Pearson 单侧 95% 上界**。

它存在的理由（一句话）：**"跑两次一样"没有任何统计意义**。
0 次分歧时，单侧 95% 上界 ≈ 3/n（rule of three）：N=6 ⇒ 只能声称分歧率 <50%；
N=20 ⇒ <15%；N=59 ⇒ <5%；**N=300 才够说 <1%**。反过来，要在 95% 置信下捕到
至少一次分歧，真率 5% 需 N≈59。任何"稳定/可复现"的宣传句都必须带 N 与上界。

输入（JSONL，每行一条观测；空行忽略）：
  ① {"group": "A", "vector_hash": "<十六进制/字符串指纹>"}   —— 按组内**众数**当参考向量
  ② {"group": "A", "diverged": true|false}                    —— 直接给定"是否分歧"
  ③ {"group": "A", "reference": "<指纹>"}                     —— 显式声明该组参考向量
     （同一组只能用一种形式；②与①混用时两种都算，见输出）
字段名可用 `--group-field/--hash-field/--diverged-field` 改写；默认 group /
vector_hash（也自动认 sig / hash / sha256 / fingerprint / vector）。

输出（每组）：
  N · 唯一值数 · 参考向量 · 分歧次数 k · 分歧率点估计 ·
  **CP 单侧 95% 上界**（k=0 时同时给 rule of three 3/n） · CP 单侧 95% 下界 ·
  一句话结论：**这个 N 能声称什么 / 不能声称什么**（含"要声称 <X% 还需多少样本"）

用法：
    python3 scripts/stability-stats.py --self-test            # 已知答案自测（rc=0/2）
    python3 scripts/stability-stats.py obs.jsonl              # 分析（先跑自检，失败拒跑）
    python3 scripts/stability-stats.py obs.jsonl --json       # 机器可读
    python3 scripts/stability-stats.py obs.jsonl --reference A=1f3c...   # 显式参考向量

退出码：`0` 正常 · `2` 硬失败（自检红 / 输入非法）——**不静默降级**。
纯标准库、python3.9 兼容（不依赖 scipy）。
"""

import argparse
import json
import math
import sys

ALPHA = 0.05                     # 单侧 95%（上界用 alpha，下界用 alpha）
TIERS = (0.50, 0.15, 0.10, 0.05, 0.02, 0.01, 0.005)   # 对外可用的档位
HASH_FIELDS = ("vector_hash", "sig", "hash", "sha256", "fingerprint", "vector")
DIVERGED_FIELDS = ("diverged", "changed", "unstable")
REVERSED_FIELDS = ("same", "identical", "same_as_reference")   # True = 未分歧


# --------------------------------------------------------------------------- #
# 核心统计：二项 CDF / Clopper-Pearson 单侧界
# --------------------------------------------------------------------------- #

def binom_cdf(k, n, p):
    """P(X <= k)，X ~ Binomial(n, p)。用 lgamma 取对数再做 fsum，避免中间溢出。"""
    if n < 0:
        raise ValueError("n must be >= 0")
    if k < 0:
        return 0.0
    if k >= n:
        return 1.0
    if p <= 0.0:
        return 1.0
    if p >= 1.0:
        return 0.0
    lp, lq = math.log(p), math.log1p(-p)
    lg = math.lgamma
    terms = []
    for i in range(0, k + 1):
        terms.append(lg(n + 1) - lg(i + 1) - lg(n - i + 1) + i * lp + (n - i) * lq)
    m = max(terms)
    val = math.exp(m) * math.fsum(math.exp(t - m) for t in terms)
    return min(1.0, max(0.0, val))


def cp_upper(k, n, alpha=ALPHA):
    """分歧率 p 的单侧 1-alpha **上界**（Clopper-Pearson 精确）。

    解 P(X <= k; n, p) = alpha。k=0 有闭式 1-alpha^(1/n)；k=n 时为 1。
    """
    _check_kn(k, n)
    if k >= n:
        return 1.0
    if k == 0:
        return 1.0 - alpha ** (1.0 / n)
    lo, hi = 0.0, 1.0                      # cdf 随 p 单调递减
    for _ in range(200):
        mid = 0.5 * (lo + hi)
        if binom_cdf(k, n, mid) > alpha:
            lo = mid
        else:
            hi = mid
    return 0.5 * (lo + hi)


def cp_lower(k, n, alpha=ALPHA):
    """分歧率 p 的单侧 1-alpha **下界**（k=n 时 = alpha^(1/n)）。"""
    _check_kn(k, n)
    if k <= 0:
        return 0.0
    lo, hi = 0.0, 1.0                      # P(X >= k) 随 p 单调递增
    for _ in range(200):
        mid = 0.5 * (lo + hi)
        if 1.0 - binom_cdf(k - 1, n, mid) > alpha:
            hi = mid
        else:
            lo = mid
    return 0.5 * (lo + hi)


def _check_kn(k, n):
    if n <= 0 or not isinstance(n, int):
        raise ValueError("n 必须是正整数")
    if k < 0 or k > n:
        raise ValueError("k 必须满足 0 <= k <= n")


def rule_of_three(n):
    """0/n 时的 rule of three：上界 ~ 3/n（保守近似，精确值略小）。"""
    return min(1.0, 3.0 / n)


def tier_for(u):
    """能对外声称的最强档位：**最小的**档位 X 满足 X > u（严格）。没有则 None。"""
    for t in sorted(TIERS):
        if t > u:
            return t
    return None


def min_n_for_upper_bound(x, alpha=ALPHA):
    """0/n 时，要声称"分歧率 < x"至少要多少**全一致**样本。"""
    if not (0.0 < x < 1.0):
        raise ValueError("x 必须在 (0,1)")
    return int(math.ceil(math.log(alpha) / math.log(1.0 - x)))


def min_n_to_detect(p_true, conf=0.95):
    """真率 p_true 时，要以 conf 概率**至少捕到一次分歧**所需的最小 N。"""
    if not (0.0 < p_true < 1.0):
        raise ValueError("p_true 必须在 (0,1)")
    return int(math.ceil(math.log(1.0 - conf) / math.log(1.0 - p_true)))


# --------------------------------------------------------------------------- #
# 输入解析
# --------------------------------------------------------------------------- #

def _pick(rec, names):
    for nm in names:
        if nm in rec:
            return nm, rec[nm]
    return None, None


def load_observations(path, group_field="group", hash_field=None, diverged_field=None):
    """读 JSONL ⇒ {group: {"hashes": [...], "flags": [bool|None...], "reference": str|None}}"""
    if path == "-":
        text = sys.stdin.read()
    else:
        with open(path, "r", encoding="utf-8") as fh:
            text = fh.read()
    groups = {}
    for lineno, line in enumerate(text.splitlines(), 1):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        try:
            rec = json.loads(line)
        except ValueError as exc:
            raise ValueError("%s:%d 不是合法 JSON：%s" % (path, lineno, exc))
        if not isinstance(rec, dict):
            raise ValueError("%s:%d 必须是 JSON 对象" % (path, lineno))
        g = rec.get(group_field, "_all")
        g = str(g)
        slot = groups.setdefault(g, {"hashes": [], "flags": [], "reference": None})
        if "reference" in rec and rec["reference"] is not None:
            if not _has_observation(rec, hash_field, diverged_field):
                slot["reference"] = str(rec["reference"])
                continue
        hname = hash_field or None
        if hname is not None:
            h = rec.get(hname)
        else:
            _, h = _pick(rec, HASH_FIELDS)
        dname = diverged_field or None
        d = rec.get(dname) if dname else None
        rev = None
        if dname is None:
            _, d = _pick(rec, DIVERGED_FIELDS)
            _, rev = _pick(rec, REVERSED_FIELDS)
        if d is not None:
            slot["flags"].append(bool(d))
            continue
        if rev is not None:
            slot["flags"].append(not bool(rev))
            continue
        if h is None:
            raise ValueError("%s:%d 既没有可识别的向量指纹字段，也没有 diverged 字段" % (path, lineno))
        slot["hashes"].append(str(h))
    if not groups:
        raise ValueError("%s 里没有可用观测" % path)
    return groups


def _has_observation(rec, hash_field, diverged_field):
    if diverged_field and diverged_field in rec:
        return True
    if hash_field and hash_field in rec:
        return True
    for nm in HASH_FIELDS + DIVERGED_FIELDS + REVERSED_FIELDS:
        if nm in rec:
            return True
    return False


# --------------------------------------------------------------------------- #
# 单组分析
# --------------------------------------------------------------------------- #

def analyze_group(name, slot, alpha=ALPHA, reference_override=None):
    """⇒ dict：N / 唯一值 / 参考向量 / k / 上界下界 / 结论句"""
    hashes = slot["hashes"]
    flags = slot["flags"]
    ref = reference_override or slot["reference"]
    counts = {}
    for h in hashes:
        counts[h] = counts.get(h, 0) + 1
    if ref is None and counts:
        # 组内众数（并列取字典序最小，保证可复现）
        best = max(counts.values())
        ref = sorted(h for h, c in counts.items() if c == best)[0]
    k = sum(1 for h in hashes if h != ref) if ref is not None else 0
    k += sum(1 for f in flags if f)
    n = len(hashes) + len(flags)
    if n == 0:
        raise ValueError("组 %s 是空的" % name)
    if n > 1 and math.log(n) > 200:
        raise ValueError("组 %s 的 N 过大" % name)
    k = min(k, n)
    u = cp_upper(k, n, alpha)
    l = cp_lower(k, n, alpha)
    rate = float(k) / n
    res = {
        "group": name, "n": n, "k": k, "unique": len(counts) if counts else None,
        "reference": ref, "rate": rate, "cp_upper": u, "cp_lower": l,
        "rule_of_three": rule_of_three(n) if k == 0 else None,
        "alpha": alpha,
    }
    if counts:
        top = sorted(counts.items(), key=lambda kv: (-kv[1], kv[0]))
        res["top_values"] = [{"hash": h, "count": c} for h, c in top[:3]]
        res["mode_count"] = top[0][1]
    res["verdict"] = _verdict(res)
    return res


def _pct(x):
    return "%.2f%%" % (100.0 * x)


def _pct_tier(x):
    """档位百分数（0.5% 这种小于 1% 的档位要保留一位小数，别被 round 成 0）。"""
    return "%.1f%%" % (100.0 * x) if x < 0.01 else "%d%%" % int(round(100.0 * x))


def _verdict(res):
    n, k, u, l = res["n"], res["k"], res["cp_upper"], res["cp_lower"]
    one = ("单侧 95%%（alpha=%.2f）" % res["alpha"])
    lines = []
    if k == 0:
        t = tier_for(u)
        lines.append(
            "N=%d、0/%d 次分歧 ⇒ 分歧率 95%% 上界 = %s（精确 CP；rule of three 3/%d = %s）"
            % (n, n, _pct(u), n, _pct(rule_of_three(n))))
        if t is None:
            lines.append(
                "结论：**本 N 连「分歧率 <50%%」都撑不住**（要 <50%% 需 N≥%d 精确 / N≥%d rule-of-three）"
                " ⇒ 等于没做统计。" % (min_n_for_upper_bound(0.50), int(math.ceil(3.0 / 0.50)) + 1))
        else:
            forbidden = [x for x in TIERS if x <= u]
            need = min_n_for_upper_bound(max(forbidden)) if forbidden else None
            s = "结论：只能声称「分歧率 < %s」（由精确上界 %s 定档）" % (_pct_tier(t), _pct(u))
            if forbidden:
                s += "；**不能**声称「< %s」（需 N≥%d，本组还差 %d 个全一致样本）" % (
                    _pct_tier(max(forbidden)), need, max(0, need - n))
            else:
                s += "；比最严档位（<0.5%）还强，已是本工具最强档"
            s += "。**不能**声称「分歧率 = 0」。"
            lines.append(s)
        return "\n".join(lines)
    if k == n:
        lines.append(
            "N=%d、%d/%d 次**全部分歧** ⇒ 分歧率 95%% 下界 = %s（精确 CP，= 0.05^(1/%d)）；"
            "同时把「一致率」压到 < %s" % (n, k, n, _pct(l), n, _pct(1.0 - l)))
        lines.append(
            "结论：可声称「分歧率 ≥ %s」（%s）；不可对分歧率给上界（N 越大下界越高，N=59 才 ≥2.5%%）"
            % (_pct(l), one))
        return "\n".join(lines)
    lines.append(
        "N=%d、%d 次分歧 ⇒ 点估计 %s；CP %s 上界 = %s、下界 = %s"
        % (n, k, _pct(res["rate"]), one, _pct(u), _pct(l)))
    t = tier_for(u)
    lines.append(
        "结论：可声称「分歧率 < %s」%s；点估计 %s **不是**可宣传的口径（N=%d 太薄）"
        % (_pct(u), ("（对外档位「< %s」）" % _pct_tier(t)) if t else "", _pct(res["rate"]), n))
    return "\n".join(lines)


# --------------------------------------------------------------------------- #
# 自测（已知答案；独立参考值来自 scipy.stats.beta.ppf，容差 1e-9）
# --------------------------------------------------------------------------- #

UP_CASES = [                     # (k, n, 期望 CP 单侧 95% 上界)
    (0, 6, 0.39303776899708265),
    (0, 20, 0.13910834066826516),
    (0, 21, 0.13294591102652337),
    (0, 25, 0.11292814500684320),
    (0, 40, 0.07215752450551455),
    (0, 55, 0.05301105495125376),
    (0, 59, 0.04950760988822693),
    (0, 80, 0.03675419777998230),
    (0, 300, 0.009936081944457708),
    (1, 20, 0.21610616420684730),
    (2, 20, 0.28261852488586087),
    (5, 59, 0.16996255552575096),
    (20, 40, 0.63890834000166220),
]
LO_CASES = [                     # (k, n, 期望 CP 单侧 95% 下界)
    (1, 20, 0.0025613787765302806),
    (5, 59, 0.033987176725468960),
    (21, 21, 0.86705408897347660),
    (25, 25, 0.88707185499315680),
    (25, 30, 0.68102881805268490),
    (12, 12, 0.77907780805444420),
    (20, 40, 0.36109165999833780),
]
TIER_CASES = [                   # (上界, 期望对外档位) —— 对齐任务书口径
    (0.39303777, 0.50),          # N=6  ⇒ <50%
    (0.13910834, 0.15),          # N=20 ⇒ <15%
    (0.04950761, 0.05),          # N=59 ⇒ <5%
    (0.00993608, 0.01),          # N=300 ⇒ <1%
    (0.07215753, 0.10),
    (0.77639320, None),          # N=2  ⇒ 无档位（"跑两次一样"没意义）
]
TOL = 1e-9


def self_test():
    """已知答案自测。返回 (通过数, 失败列表)。"""
    passed, failed = 0, []

    def check(name, got, want, tol=TOL):
        nonlocal passed
        ok = False
        if want is None or got is None:
            ok = got is want
        elif isinstance(want, float):
            ok = abs(float(got) - want) <= tol
        else:
            ok = got == want
        if ok:
            passed += 1
        else:
            failed.append("%s：期望 %r，实得 %r" % (name, want, got))
        return ok

    # 1) CP 上界：与 scipy.stats.beta.ppf(0.95, k+1, n-k) 对齐
    for k, n, want in UP_CASES:
        check("cp_upper(%d/%d)" % (k, n), cp_upper(k, n), want, 1e-9)
    # 2) CP 下界：与 scipy.stats.beta.ppf(0.05, k, n-k+1) 对齐
    for k, n, want in LO_CASES:
        check("cp_lower(%d/%d)" % (k, n), cp_lower(k, n), want, 1e-9)
    # 3) k=0 闭式与数值解必须一致（两条独立代码路径互证）
    for n in (1, 2, 6, 20, 59, 300):
        closed = 1.0 - ALPHA ** (1.0 / n)
        check("0/%d 闭式 vs 数值解" % n, cp_upper(0, n), closed, 1e-12)
    # 4) rule of three 是保守近似：3/n >= 精确上界
    worst = float("inf")
    for n in range(1, 401):
        gap = rule_of_three(n) - cp_upper(0, n)
        worst = min(worst, gap)
    check("rule of three 全程 >= 精确上界", worst >= -1e-12, True)
    # 5) 退化情形
    check("cp_upper(n/n)=1", cp_upper(7, 7), 1.0)
    check("cp_lower(0/n)=0", cp_lower(0, 6), 0.0)
    check("cp_lower(n/n)=0.05^(1/n)", cp_lower(21, 21), ALPHA ** (1.0 / 21), 1e-12)
    check("binom_cdf 归一", binom_cdf(20, 20, 0.3), 1.0)
    check("binom_cdf(-1)=0", binom_cdf(-1, 20, 0.3), 0.0)
    check("binom_cdf(0,20,0.139108)=alpha", binom_cdf(0, 20, 0.13910834066826516), ALPHA, 1e-9)
    # 6) 单调性
    check("上界随 k 单调增", cp_upper(1, 20) > cp_upper(0, 20), True)
    check("下界随 n 单调增(k=n)", cp_lower(25, 25) > cp_lower(21, 21), True)
    # 7) 档位映射（任务书口径：N=6⇒<50%，N=20⇒<15%，N=59⇒<5%，N=300⇒<1%）
    for u, want in TIER_CASES:
        check("tier_for(%.8f)" % u, tier_for(u), want)
    check("档位显示 0.5% 不被 round 成 0%", _pct_tier(0.005), "0.5%")
    check("档位显示 15%", _pct_tier(0.15), "15%")
    # 8) "要声称 <X% 需多少样本" / "真率 p 要多少样本才捕得到"
    for x, want in ((0.50, 5), (0.15, 19), (0.10, 29), (0.05, 59), (0.01, 299)):
        check("min_n_for_upper_bound(%g)" % x, min_n_for_upper_bound(x), want)
    for p, want in ((0.50, 5), (0.15, 19), (0.05, 59), (0.01, 299)):
        check("min_n_to_detect(%g)" % p, min_n_to_detect(p), want)
    # 9) 端到端：0/6 与 1/20 的两条输入形式
    obs = "\n".join('{"group":"g","vector_hash":"aa"}' for _ in range(6))
    g = load_observations_text(obs)
    r = analyze_group("g", g["g"])
    check("0/6 端到端 n", r["n"], 6)
    check("0/6 端到端 k", r["k"], 0)
    check("0/6 端到端 uniq", r["unique"], 1)
    check("0/6 端到端上界", r["cp_upper"], UP_CASES[0][2], 1e-9)
    check("0/6 端到端 rule of three", r["rule_of_three"], 0.5)
    obs = "\n".join('{"group":"g","diverged":%s}' % ("false" if i else "true") for i in range(20))
    g = load_observations_text(obs)
    r = analyze_group("g", g["g"])
    check("1/20(diverged) n", r["n"], 20)
    check("1/20(diverged) k", r["k"], 1)
    check("1/20(diverged) 上界", r["cp_upper"], UP_CASES[9][2], 1e-9)
    # 10) 显式参考向量：与参考不同的比例 ⇒ CP 下界（含 k=n 的极端）
    obs = "\n".join('{"group":"g","vector_hash":"solo"}' for _ in range(5))
    obs += "\n" + "\n".join('{"group":"g","vector_hash":"h%d"}' % i for i in range(25))
    g = load_observations_text(obs)
    r = analyze_group("g", g["g"], reference_override="solo")
    check("25/30 对显式参考 n", r["n"], 30)
    check("25/30 对显式参考 k", r["k"], 25)
    check("25/30 对显式参考下界", r["cp_lower"], LO_CASES[4][2], 1e-9)
    obs = "\n".join('{"group":"g","vector_hash":"u%d"}' % i for i in range(25))
    g = load_observations_text(obs)
    r = analyze_group("g", g["g"], reference_override="solo")
    check("25/25 全分歧 n", r["n"], 25)
    check("25/25 全分歧 k", r["k"], 25)
    check("25/25 全分歧下界", r["cp_lower"], LO_CASES[3][2], 1e-9)
    # 11) 非法输入必须抛错，不得静默
    for bad, tag in (("", "空文件"), ("{\n", "坏 JSON"), ('{"group":"g"}', "无可用字段")):
        try:
            load_observations_text(bad)
            failed.append("非法输入未抛错：%s" % tag)
        except ValueError:
            passed += 1
    for bad in ((0, 0), (3, 2)):
        try:
            cp_upper(*bad)
            failed.append("非法 k/n 未抛错：%r" % (bad,))
        except ValueError:
            passed += 1
    return passed, failed


def _run_self_test_safe():
    """自检的**崩溃也算红**：任何异常都折算成"1 项失败 + rc=2"，绝不静默放行。"""
    try:
        return self_test()
    except Exception as exc:                      # noqa: BLE001 —— 故意兜住一切
        return 0, ["自检自身抛异常（视为红）：%s: %s" % (type(exc).__name__, exc)]


def load_observations_text(text):
    """自测用的内存版输入解析。"""
    import tempfile, os
    fd, p = tempfile.mkstemp(suffix=".jsonl")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as fh:
            fh.write(text)
        return load_observations(p)
    finally:
        os.unlink(p)


# --------------------------------------------------------------------------- #
# 渲染
# --------------------------------------------------------------------------- #

def render_text(results, alpha=ALPHA, show_table=True):
    out = []
    out.append("=" * 78)
    out.append("稳定性统计口径 · 分歧率 + Clopper-Pearson 单侧 %.0f%% 上界（alpha=%.2f）"
               % (100 * (1 - alpha), alpha))
    out.append("=" * 78)
    for r in results:
        out.append("")
        out.append("组：%s" % r["group"])
        bits = ["N=%d" % r["n"], "分歧 k=%d" % r["k"], "点估计 %s" % _pct(r["rate"]),
                "CP 上界 %s" % _pct(r["cp_upper"]), "CP 下界 %s" % _pct(r["cp_lower"])]
        if r["unique"] is not None:
            bits.append("唯一值 %d" % r["unique"])
        if r["reference"]:
            bits.append("参考向量 %s" % r["reference"])
        if r["rule_of_three"] is not None:
            bits.append("rule of three %s" % _pct(r["rule_of_three"]))
        out.append("  " + " · ".join(bits))
        if r.get("top_values"):
            out.append("  取值分布：" + " | ".join(
                "%s×%d" % (v["hash"][:24], v["count"]) for v in r["top_values"]))
        for line in r["verdict"].splitlines():
            out.append("  " + line)
    if show_table:
        out.append("")
        out.append("-" * 78)
        out.append("样本量对照表（0 分歧时能声称什么 / 要多少样本才能捕到分歧）")
        out.append("-" * 78)
        out.append("  要声称「分歧率 <X%」(0 分歧)： " + " · ".join(
            "<%g%% 需 N≥%d" % (100 * x, min_n_for_upper_bound(x))
            for x in (0.15, 0.10, 0.05, 0.02, 0.01, 0.005)))
        out.append("  rule of three 3/n 近似：      " + " · ".join(
            "<%g%% 需 N≥%d" % (100 * x, int(math.ceil(3.0 / x))) for x in (0.05, 0.01)))
        out.append("  真率 p 要 ≥95% 概率捕到一次： " + " · ".join(
            "p=%g%% 需 N≥%d" % (100 * p, min_n_to_detect(p)) for p in (0.10, 0.05, 0.01)))
        out.append("  口径铁律：N=2 的「两次一样」上界 ≈ 78%——**没有任何统计意义**。")
    return "\n".join(out)


def main(argv=None):
    ap = argparse.ArgumentParser(
        description="分歧率 + Clopper-Pearson 单侧 95% 上界（T7.3 可复现性口径）")
    ap.add_argument("inputs", nargs="*", help="JSONL 观测文件（- 表示 stdin）")
    ap.add_argument("--self-test", action="store_true", help="只跑已知答案自测")
    ap.add_argument("--group-field", default="group")
    ap.add_argument("--hash-field", default=None)
    ap.add_argument("--diverged-field", default=None)
    ap.add_argument("--reference", action="append", default=[],
                    metavar="GROUP=HASH", help="显式声明某组的参考向量")
    ap.add_argument("--alpha", type=float, default=ALPHA)
    ap.add_argument("--json", action="store_true", help="输出机器可读 JSON")
    ap.add_argument("--no-preflight", action="store_true", help="跳过分析前自检（不建议）")
    ap.add_argument("--no-table", action="store_true", help="不打印样本量对照表")
    args = ap.parse_args(argv)

    if args.self_test:
        passed, failed = _run_self_test_safe()
        print("自测：%d 项通过，%d 项失败" % (passed, len(failed)))
        for f in failed:
            print("  FAIL " + f)
        if failed:
            print("自测红 ⇒ 拒跑真目标（rc=2）")
            return 2
        print("自测全绿（已知答案：0/6=39.30%、0/20=13.91%、1/20=21.61%、5/59=17.00%、"
              "0/59=4.95%、0/300=0.99%、21/21 下界=86.71%、25/25 下界=88.71%）")
        return 0

    if not args.inputs:
        ap.print_help()
        return 2

    if not args.no_preflight:
        passed, failed = _run_self_test_safe()
        if failed:
            print("预检自测失败（%d 项）⇒ 拒跑真目标（rc=2）" % len(failed), file=sys.stderr)
            for f in failed:
                print("  FAIL " + f, file=sys.stderr)
            return 2
        print("预检自检：%d 项全绿（本工具自己的正确性先过检）" % passed, file=sys.stderr)

    refs = {}
    for item in args.reference:
        if "=" not in item:
            print("--reference 需要 GROUP=HASH 形式：%s" % item, file=sys.stderr)
            return 2
        g, h = item.split("=", 1)
        refs[g] = h

    try:
        slots = {}
        for p in args.inputs:
            for g, slot in load_observations(
                    p, args.group_field, args.hash_field, args.diverged_field).items():
                tgt = slots.setdefault(g, {"hashes": [], "flags": [], "reference": None})
                tgt["hashes"].extend(slot["hashes"])
                tgt["flags"].extend(slot["flags"])
                if slot["reference"] and not tgt["reference"]:
                    tgt["reference"] = slot["reference"]
        results = [analyze_group(g, s, args.alpha, refs.get(g))
                   for g, s in sorted(slots.items())]
    except (ValueError, OSError) as exc:
        print("输入错误 ⇒ 硬失败（rc=2）：%s" % exc, file=sys.stderr)
        return 2

    if args.json:
        print(json.dumps({"alpha": args.alpha, "groups": results}, ensure_ascii=False, indent=2))
    else:
        print(render_text(results, args.alpha, not args.no_table))
    return 0


if __name__ == "__main__":
    sys.exit(main())
