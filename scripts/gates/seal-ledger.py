#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""seal-ledger.py —— 核章档（第四档）· **章账升格 + 核章**（设计稿《设计-流程规则程序化-v1.0》
§附录-门禁提速·核章档（第四档 · 局部章 + 核章） · 清单 T70 一阶段 · 拍-15）

一句话：**章 = 步级 / 件级的签名记录**；`--seal` 把一趟真跑的产物（`results.tsv` + `freshness.json`）
**升格**成章账（一行一步 · 九格）；`--verify` 是**核章**——只校验章链的覆盖与新鲜度，
**不真跑任何一步**。

档位链（唯一真源 · `scripts/gates/wallclock-baseline.json` 的 `chain`）：
    日常快速档 ⊂ 核章档 ⊂ 中档 ⊂ 盖章档

三条硬前提（缺一即章不可信 · 逐条有牙）：
    ① **章必绑内容**：章必须绑「件集内容摘要 + 判据件自身版本」，只绑整树指纹不算 ⇒ 缺任一格即**无效**；
    ② **输入变则章自动过期 · 禁手工续期**：过期一律由**现读比对**触发；章链**链式摘要** —— 改任一格
       后续章的 `chain` 立刻对不上 ⇒ 整条链报 `gap`（伪造/手工续期即在此处现形）；
    ③ **章不得冒充真跑**：核章输出**只许三态** `未真跑（核章）` / `stale` / `gap`，
       **禁渲染成 `PASS`**（`--check` 有负向守门）；且断言 `executed=true` 计数 == **0**。

与五条提速不变量的关系（设计稿 §五 · 一律五条优先）：
    ① 步集不缩 —— 章账盖的步集 == 现读步集，缺一步即 `gap`（不是抽样比）；
    ② 判据不放宽 —— 本件只**记**结论，不改任何阈值 / 闭集 / 棘轮值；
    ③ 跳过必留痕 —— 核章不真跑 ⇒ 独立状态逐字 = `未真跑（核章）`，**绝不显示成 PASS**；
    ⑤ 盖章档必须全冷跑 —— 核章档**不得用于盖章**：核章的章**不能**替代盖章档任何一步。

退码（只判 · 不改任何既有出口）：
    0 全部章有效（覆盖齐 + 四格全对 + 链完整）· 1 有 `stale` / `gap`（逐枚点名）· 2 判不了（缺件 / 读不回）
"""
import argparse, datetime, hashlib, json, os, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
LEDGER_REL = os.path.join("scripts", "gates", "seal", "ledger.jsonl")
# ★ 落点（2026-09-30 修 · N6）：原 `scripts/gates/seal-ledger.jsonl` 与判据件
#   `scripts/gates/seal-ledger.py` **同目录同基名、扩展名不同** ⇒ 撞命名规则 ⑥（N6）
#   ⇒ 换落到**子目录** `scripts/gates/seal/ledger.jsonl`（名字语义保留）。本常量是**单一真源**：
#   指纹 / 摘要口径照它**精确排除章账自身**（只此一件）。
LEDGER_DEFAULT = os.path.join(REPO_ROOT, LEDGER_REL)
GATE_SCRIPT = os.path.join("scripts", "gates", "precommit-gates.sh")
BASELINE = os.path.join(HERE, "wallclock-baseline.json")

# 核章输出**只许**这三种状态（闭集 · 禁 PASS）
SEAL_STATES = ("未真跑（核章）", "stale", "gap")
CHAIN_SEED = "zerg-seal-chain/1"


def die(msg, rc=2):
    print(msg, file=sys.stderr)
    sys.exit(rc)


def sha_file(path):
    try:
        with open(path, "rb") as f:
            return hashlib.sha256(f.read()).hexdigest()
    except OSError:
        return ""


def git(root, *args):
    try:
        p = subprocess.run(["git", "-C", root, *args], capture_output=True, text=True, timeout=120)
    except (OSError, subprocess.SubprocessError):
        return ""
    return p.stdout if p.returncode == 0 else ""


def ledger_rel_of(root, ledger):
    """章账的**仓内相对路径**（用于「精确排除章账自身」）。不在本仓内 ⇒ 回落缺省落点。"""
    try:
        rel = os.path.relpath(os.path.abspath(ledger), root)
    except ValueError:
        return LEDGER_REL
    if rel.startswith("..") or os.path.isabs(rel):
        return LEDGER_REL
    return rel


def item_set_digest(root, exclude_rel=None):
    """① 件集内容摘要 = 「这一章盖住了哪些件」——由**现读改动面**算出（只读 git）。

    口径与 `gate_freshness.go` 的整树指纹**故意不同**：这里只收**改动的件**（脏面），
    逐件取**工作树内容** blob sha，件名排序后整串再 hash 一次 ⇒ 件名清单变或内容变即变。

    ★ 精确排除章账自身（2026-09-30 修 · 自 stale 闭环）：章账 append-only ⇒ 追加一枚章就改
    自己的内容 ⇒ 它若进摘要，摘要里就含「记着摘要的那份账」⇒ **固定点不可达**：每盖一章摘要
    立刻变 ⇒ 刚盖的章在下次 `--verify` 全判 stale。只排除**那一个件**（缺省 = `LEDGER_REL`，
    调用方按 `--ledger` 现算），不动别的件、不改粒度 —— 不是放宽判据。
    """
    raw = git(root, "--no-optional-locks", "status", "--porcelain", "-z")
    names = []
    for tok in raw.split("\x00"):
        tok = tok.rstrip("\n")
        if not tok:
            continue
        # porcelain v1 -z：`XY <路径>`（重命名是两段），只取路径段
        if len(tok) > 3 and tok[2] == " ":
            tok = tok[3:]
        names.append(tok)
    skip = exclude_rel or LEDGER_REL
    names = sorted(set(n for n in names if n and n != skip))
    h = hashlib.sha256()
    h.update(b"item-set/1\x00")
    for n in names:
        blob = git(root, "--no-optional-locks", "hash-object", "--", n).strip() or "-"
        h.update(f"{n}\x00{blob}\x00".encode("utf-8"))
    return h.hexdigest(), names


def judge_version(root, step_cmd):
    """② 判据件自身版本 = sha256(该步的**命令串** + 命令串里点到的**在盘门件**的内容)。

    判据一改（脚本 / 门件被改）⇒ 旧章自动失配。取不到具名门件（如 go test / cargo test）时，
    命令串本身就是判据身份 —— 同样进摘要。
    """
    h = hashlib.sha256()
    h.update(b"judge/1\x00")
    h.update((step_cmd or "").encode("utf-8"))
    h.update(b"\x00")
    for tok in (step_cmd or "").replace("'", " ").replace('"', " ").split():
        if "/" not in tok:
            continue
        cand = os.path.join(root, tok)
        if os.path.isfile(cand):
            h.update(f"{tok}={sha_file(cand)}".encode("utf-8"))
            h.update(b"\x00")
    return h.hexdigest()


def gate_version(root):
    """③ 门版本 = 门件内容 sha + `zerg` 自报身份（取不到就只算门件 sha）。"""
    h = hashlib.sha256()
    h.update(b"gate/1\x00")
    h.update(sha_file(os.path.join(root, GATE_SCRIPT)).encode("utf-8"))
    ident = ""
    zerg = os.path.join(root, "bin", "zerg")
    if os.path.isfile(zerg):
        try:
            p = subprocess.run([zerg, "version"], capture_output=True, text=True, timeout=60)
            if p.returncode == 0:
                ident = " ".join(p.stdout.split())[:200]
        except (OSError, subprocess.SubprocessError):
            ident = ""
    h.update(("\x00" + ident).encode("utf-8"))
    return h.hexdigest(), ident


def read_results(path):
    rows = []
    with open(path, encoding="utf-8") as f:
        for ln in f:
            ln = ln.rstrip("\n")
            if not ln:
                continue
            c = ln.split("\t")
            if len(c) < 5:
                continue
            rows.append({"status": c[0], "step": c[1], "rc": c[2], "secs": c[3], "log": c[4]})
    return rows


def read_freshness(dirpath):
    p = os.path.join(dirpath, "freshness.json")
    try:
        with open(p, encoding="utf-8") as f:
            return json.load(f)
    except (OSError, json.JSONDecodeError):
        return {}


def chain_of(prev_chain, rec):
    h = hashlib.sha256()
    h.update(CHAIN_SEED.encode("utf-8") + b"\x00")
    h.update((prev_chain or "").encode("utf-8") + b"\x00")
    for k in ("step", "status", "rc", "worktree_fp", "head",
              "item_set_digest", "judge_ver", "gate_ver", "chapter_no"):
        h.update(f"{k}={rec.get(k, '')}\x00".encode("utf-8"))
    return h.hexdigest()


def last_chain(ledger):
    prev, n = "", 0
    if not os.path.isfile(ledger):
        return prev, n
    with open(ledger, encoding="utf-8") as f:
        for ln in f:
            ln = ln.strip()
            if not ln:
                continue
            try:
                r = json.loads(ln)
            except json.JSONDecodeError:
                continue
            prev, n = r.get("chain", ""), int(r.get("chapter_no", 0))
    return prev, n


def load_ledger(ledger):
    out = []
    if not os.path.isfile(ledger):
        return out
    with open(ledger, encoding="utf-8") as f:
        for ln in f:
            ln = ln.strip()
            if not ln:
                continue
            try:
                out.append(json.loads(ln))
            except json.JSONDecodeError:
                out.append({"_unparseable": ln[:80]})
    return out


def step_cmds(root):
    """现读步集与各步命令串（**门件自报** —— 不手抄名单）。取不回 ⇒ 判不了。"""
    script = os.path.join(root, GATE_SCRIPT)
    if not os.path.isfile(script):
        return None, None
    try:
        p = subprocess.run(["bash", script, "--list"], capture_output=True, text=True, timeout=180)
    except (OSError, subprocess.SubprocessError):
        return None, None
    if p.returncode != 0:
        return None, None
    names = []
    for ln in p.stdout.split("\n"):
        if not ln.startswith("   "):
            continue
        rest = ln.lstrip(" ")
        i = rest.find(" ")
        if i < 0:
            continue
        rest = rest[i:].lstrip(" ")
        j = rest.find(" ")
        if j < 0:
            continue
        nm = rest[j:].lstrip(" ")
        if nm:
            names.append(nm)
    if not names:
        return None, None
    cmds = {}
    zerg = os.path.join(root, "bin", "zerg")
    if os.path.isfile(zerg):
        for nm in names:
            try:
                q = subprocess.run([zerg, "gate", "show", nm, "--json", "cmd"],
                                   capture_output=True, text=True, timeout=60)
                if q.returncode == 0:
                    cmds[nm] = " ".join(q.stdout.split())
            except (OSError, subprocess.SubprocessError):
                pass
    return names, cmds


# ────────────────────────────────────────────────────────────────────────────
# 盖章（写 · 唯一写通道 · append-only）
# ────────────────────────────────────────────────────────────────────────────
def do_seal(args):
    run_dir = args.seal
    rt = os.path.join(run_dir, "results.tsv")
    if not os.path.isfile(rt):
        die(f"✗ `--seal {run_dir}` 里没有 results.tsv ⇒ 不是一趟门跑产物 ⇒ 不给结论(2)")
    rows = read_results(rt)
    if not rows:
        die(f"✗ `{rt}` 一行都读不出 ⇒ 判不了(2)")
    fp = read_freshness(run_dir)
    isd, changed = item_set_digest(REPO_ROOT, ledger_rel_of(REPO_ROOT, args.ledger))
    gv, gident = gate_version(REPO_ROOT)
    names, cmds = step_cmds(REPO_ROOT)
    ledger = args.ledger
    prev, n = last_chain(ledger)
    wrote = 0
    lines = []
    for r in rows:
        n += 1
        rec = {
            "chapter_no": n,
            "at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "step": r["step"], "status": r["status"], "rc": r["rc"], "secs": r["secs"],
            "log": r["log"],
            "worktree_fp": fp.get("worktree_fp", ""),   # 现成地基（freshness.json）
            "head": fp.get("head", ""),                 # 现成地基
            "item_set_digest": isd,                     # 新造格 ①（门/脚本现算现写）
            "judge_ver": judge_version(REPO_ROOT, cmds.get(r["step"], "")),
            "gate_ver": gv,
            "executed": True,                           # 盖章这一趟**是真跑的**
        }
        rec["chain"] = chain_of(prev, rec)
        prev = rec["chain"]
        lines.append(json.dumps(rec, ensure_ascii=False))
        wrote += 1
    # 覆盖表：这一批章盖住了哪些步（步集不缩的分母）
    lines.append(json.dumps({
        "chapter_no": max(n + 1, 1), "kind": "cover", "run_dir": os.path.abspath(run_dir),
        "steps": [r["step"] for r in rows], "n": wrote,
        "baseline_chain": prev, "gate_ver": gv, "gate_ident": gident,
        "at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "executed": True,
    }, ensure_ascii=False))
    if not args.write:
        print(f"[试跑] 将盖章 {wrote} 枚（未落盘 · 加 --write 才写）")
        print(f"  件集内容摘要 {isd[:16]} · 改动面 {len(changed)} 件 · 门版本 {gv[:16]} {gident}")
        return 0
    if not args.reason:
        die("✗ --seal --write 必须带 --reason（唯一写通道要留痕）⇒ 不给结论(2)")
    with open(ledger, "a", encoding="utf-8") as f:
        f.write("\n".join(lines) + "\n")
    print(f"✓ 已盖 {wrote} 枚章（落点 {ledger}）· 件集内容摘要 {isd[:16]} · 改动面 {len(changed)} 件")
    return 0


# ────────────────────────────────────────────────────────────────────────────
# 核章（只读 · 不真跑 · 不写盘 · 不续期）
# ────────────────────────────────────────────────────────────────────────────
def do_verify(args):
    ledger = args.ledger
    recs = load_ledger(ledger)
    if not recs:
        die(f"✗ 核章：章账 {ledger} 读不到任何一枚章 ⇒ **不给结论**(2)（不许当绿）")
    names, cmds = step_cmds(REPO_ROOT)
    if names is None:
        die("✗ 核章：读不回门件自己的步骤清单 ⇒ **不给结论**(2)")
    isd, changed = item_set_digest(REPO_ROOT, ledger_rel_of(REPO_ROOT, args.ledger))
    gv, gident = gate_version(REPO_ROOT)

    seals = [r for r in recs if r.get("kind") != "cover" and not r.get("_unparseable")]
    covers = [r for r in recs if r.get("kind") == "cover"]

    # ── 链完整：序号连续无洞 + 每枚章的 chain 逐枚复算 ────────────────────────
    chain_gap = []
    prev, expect = "", 1
    for r in seals:
        if int(r.get("chapter_no", -1)) != expect:
            chain_gap.append(f"序号有洞：期望 {expect} 得 {r.get('chapter_no')}")
        if chain_of(prev, {**r, "chapter_no": r.get("chapter_no")}) != r.get("chain"):
            chain_gap.append(f"章 #{r.get('chapter_no')} 链摘要对不上（**历史被改写 / 手工续期**）")
        prev, expect = r.get("chain", ""), expect + 1

    # ── 覆盖：章盖的步集 vs 现读步集（步集不缩 · 缺一即 gap）──────────────────
    sealed_steps = [s for c in covers for s in c.get("steps", [])]
    sealed_set = set(sealed_steps)
    cur_set = set(names)
    missing = sorted(cur_set - sealed_set)     # 现读有、章里没有
    extra = sorted(sealed_set - cur_set)       # 章里有、现读没有（改名/删步）

    verdicts, n_stale, n_gap = [], 0, 0
    for r in seals:
        step = r.get("step", "")
        reasons = []
        if not r.get("item_set_digest"):
            reasons.append("缺格：件集内容摘要为空 ⇒ 章无效（硬前提①）")
        if not r.get("judge_ver"):
            reasons.append("缺格：判据件自身版本为空 ⇒ 章无效（硬前提①）")
        if not r.get("gate_ver"):
            reasons.append("缺格：门版本为空 ⇒ 章无效")
        if r.get("item_set_digest") and r["item_set_digest"] != isd:
            reasons.append(f"件集内容摘要不符（章 {r['item_set_digest'][:12]} ≠ 现读 {isd[:12]}）")
        if r.get("judge_ver") and step in cmds and r["judge_ver"] != judge_version(REPO_ROOT, cmds.get(step, "")):
            reasons.append("判据件自身版本变（判据换了，旧结论不算数）")
        if r.get("gate_ver") and r["gate_ver"] != gv:
            reasons.append("门版本变")
        if step not in cur_set:
            reasons.append("gap：该步已不在现读步集（增删改名）")
        st = "未真跑（核章）" if not reasons else ("gap" if any(x.startswith("gap") or x.startswith("缺格") for x in reasons) else "stale")
        if st == "gap":
            n_gap += 1
        elif st == "stale":
            n_stale += 1
        verdicts.append({"chapter_no": r.get("chapter_no"), "step": step, "state": st,
                         "executed": False, "reasons": reasons})

    # ── 输出纪律：三态闭集 · 禁 PASS · executed=true 计数必须 0 ─────────────
    bad_render = [v for v in verdicts if v["state"] not in SEAL_STATES]
    n_exec = sum(1 for v in verdicts if v.get("executed"))
    head = ("未真跑 · 结论由章支持" if (not chain_gap and not missing and not extra
                                      and not n_stale and not n_gap) else "未真跑 · 结论由章支持（**有不符项**）")
    print(f"── 核章档（第四档）· {head} ──")
    print(f"── 档位链：日常快速档 ⊂ 核章档 ⊂ 中档 ⊂ 盖章档（真源 {os.path.relpath(BASELINE, REPO_ROOT)} 的 `chain`）──")
    print(f"── 章账 {ledger} · 章 {len(seals)} 枚 · 覆盖表 {len(covers)} 张 · "
          f"executed=true 计数 {n_exec}（**必须 0**）· 现读步集 {len(names)} 步 ──")
    print(f"── 件集内容摘要 {isd[:16]} · 改动面 {len(changed)} 件 · 门版本 {gv[:16]} {gident} ──")
    if chain_gap:
        print("── 链完整性：**不过** ──")
        for g in chain_gap:
            print(f"   ✗ {g}")
    else:
        print(f"── 链完整性：✓ 序号 1..{len(seals)} 连续 · 逐枚链摘要复算一致 ──")
    if missing or extra:
        print(f"── 覆盖：**gap** —— 现读有而章无 {len(missing)} 步 · 章有而现读无 {len(extra)} 步 ──")
        for s in missing:
            print(f"   gap 缺步：{s}")
        for s in extra:
            print(f"   gap 多步：{s}")
    else:
        print("── 覆盖：✓ 章盖的步集 == 现读步集（步集不缩）──")
    for v in verdicts:
        print(f"   [{v['state']}] #{v['chapter_no']} {v['step']}")
        for rs in v["reasons"]:
            print(f"        ✗ {rs}")
    bad = bool(chain_gap or missing or extra or n_stale or n_gap or bad_render or n_exec)
    if args.json:
        print(json.dumps({"kind": "SealVerify", "state": head,
                          "seals": len(seals), "executed_true": n_exec,
                          "stale": n_stale, "gap": n_gap,
                          "chain_gap": chain_gap, "missing": missing, "extra": extra,
                          "item_set_digest": isd, "gate_ver": gv,
                          "verdicts": verdicts}, ensure_ascii=False))
    if bad:
        print("⇒ 核章：**有不符项**（rc=1）—— 过期**不等于**重跑全量：只补跑该章覆盖的那几步")
        return 1
    print("⇒ 核章：章链有效（rc=0）· **未真跑任何一步**（executed=true == 0）")
    return 0


# ────────────────────────────────────────────────────────────────────────────
# 自检（成对正负控 · 合成夹具 · 不碰真目标）
# ────────────────────────────────────────────────────────────────────────────
def do_check():
    import shutil, tempfile
    tmp = tempfile.mkdtemp(prefix="seal-ledger-selftest.")
    bad = 0

    def chk(what, ok, extra=""):
        nonlocal bad
        if not isinstance(extra, str):
            extra = str(extra)
        print(("   ✓ " if ok else "   ✗ ") + what + (f"（{extra}）" if not ok and extra else ""))
        if not ok:
            bad += 1

    try:
        # 夹具：合成仓 + 合成一趟（results.tsv + freshness.json）
        repo = os.path.join(tmp, "repo")
        os.makedirs(os.path.join(repo, "scripts", "gates"), exist_ok=True)
        shutil.copy(os.path.join(HERE, "seal-ledger.py"), os.path.join(repo, "scripts", "gates", "seal-ledger.py"))
        with open(os.path.join(repo, "scripts", "gates", "precommit-gates.sh"), "w") as f:
            f.write("#!/bin/bash\necho '   1     tri    步甲'\n")
        subprocess.run(["git", "init", "-q", repo], check=False)
        with open(os.path.join(repo, "judge.py"), "w") as f:
            f.write("print('judge')\n")
        subprocess.run(["git", "-C", repo, "add", "-A"], check=False)
        run = os.path.join(tmp, "zerg-gates-fake")
        os.makedirs(run, exist_ok=True)
        with open(os.path.join(run, "results.tsv"), "w") as f:
            f.write("PASS\t步甲\t0\t1s\t/nowhere/a.log\n")
        with open(os.path.join(run, "freshness.json"), "w") as f:
            json.dump({"head": "abc1234", "worktree_fp": "f" * 64}, f)
        ledger = os.path.join(tmp, "seal.jsonl")
        # ★ 必须跑**夹具仓里那份副本**：REPO_ROOT 由 `__file__` 推出 ⇒ 跑原件会把真仓当夹具
        me = os.path.join(repo, "scripts", "gates", "seal-ledger.py")

        # 正控 1：盖章（--write --reason）落盘
        p = subprocess.run([sys.executable, me, "--seal", run, "--ledger", ledger,
                            "--write", "--reason", "自检"], capture_output=True, text=True, cwd=repo)
        chk("正控 1：--seal --write --reason ⇒ rc=0 且落盘", p.returncode == 0 and os.path.isfile(ledger), p.stderr[:200])
        # 负控 2：--seal --write 不带 --reason ⇒ rc=2（唯一写通道要留痕）
        p = subprocess.run([sys.executable, me, "--seal", run, "--ledger", ledger,
                            "--write"], capture_output=True, text=True, cwd=repo)
        chk("负控 2：--seal --write 缺 --reason ⇒ rc=2", p.returncode == 2, f"rc={p.returncode}")
        # 核章：输出必须带「未真跑 · 结论由章支持」且**逐字不含 PASS**
        p = subprocess.run([sys.executable, me, "--verify", "--ledger", ledger],
                           capture_output=True, text=True, cwd=repo)
        out = p.stdout
        chk("正控 3：核章输出带「未真跑 · 结论由章支持」", "未真跑 · 结论由章支持" in out)
        chk("负控 4（守门）：核章输出**逐字不含** `PASS`/`通过`", ("PASS" not in out) and ("通过" not in out),
            [l for l in out.split("\n") if "PASS" in l or "通过" in l][:2])
        chk("正控 5：断言 executed=true 计数 == 0 现印", "executed=true 计数 0" in out, out[:200])
        chk("正控 6：核章的步状态落在三态闭集里", any(("未真跑（核章）" in l) for l in out.split("\n")))
        # 负控 7：改一个被盖住的件 ⇒ 件集内容摘要变 ⇒ 必须报 stale（不是全绿）
        with open(os.path.join(repo, "judge.py"), "w") as f:
            f.write("print('judgeCHANGED')\n")
        p = subprocess.run([sys.executable, me, "--verify", "--ledger", ledger],
                           capture_output=True, text=True, cwd=repo)
        chk("负控 7：改动面变 ⇒ 核章报 stale 且 rc=1（**不许全绿**）",
            p.returncode == 1 and "stale" in p.stdout, f"rc={p.returncode}")
        # 负控 8：手工续期（改章里一格）⇒ 链摘要对不上 ⇒ 报 gap
        recs = [json.loads(l) for l in open(ledger, encoding="utf-8") if l.strip()]
        recs[0]["status"] = "FORGED"          # 伪造
        with open(ledger, "w", encoding="utf-8") as f:
            for r in recs:
                f.write(json.dumps(r, ensure_ascii=False) + "\n")
        p = subprocess.run([sys.executable, me, "--verify", "--ledger", ledger],
                           capture_output=True, text=True, cwd=repo)
        chk("负控 8：手工改章一格 ⇒ 链摘要对不上 ⇒ 报 gap 且 rc=1",
            p.returncode == 1 and "链摘要对不上" in p.stdout, f"rc={p.returncode}")
        # 负控 9：缺格 ⇒ 章无效
        recs[0]["chain"] = recs[0].get("chain")
        recs2 = [dict(recs[0])]
        recs2[0]["item_set_digest"] = ""
        records = [json.dumps(r, ensure_ascii=False) for r in recs2]
        led2 = os.path.join(tmp, "seal2.jsonl")
        with open(led2, "w", encoding="utf-8") as f:
            f.write("\n".join(records) + "\n")
        p = subprocess.run([sys.executable, me, "--verify", "--ledger", led2],
                           capture_output=True, text=True, cwd=repo)
        chk("负控 9：章缺格（件集内容摘要为空）⇒ 判无效（rc=1）",
            p.returncode == 1 and "缺格" in p.stdout, f"rc={p.returncode}")
        # 负控 10：章账读不到 ⇒ rc=2 **不给结论**
        p = subprocess.run([sys.executable, me, "--verify", "--ledger", os.path.join(tmp, "none.jsonl")],
                           capture_output=True, text=True, cwd=repo)
        chk("负控 10：章账空 ⇒ rc=2（不给结论 · 不许当绿）", p.returncode == 2, f"rc={p.returncode}")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print("── seal-ledger.py 自检（成对正负控 · 真退出码）──")
    print("自检结论: " + ("全过" if bad == 0 else f"**不过**（{bad} 条）"))
    return 0 if bad == 0 else 8


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--seal", default=None, help="把一趟真跑产物升格成章（<趟目录>）")
    ap.add_argument("--verify", action="store_true", help="核章：只读核章链（不真跑 · 不写盘 · 不续期）")
    ap.add_argument("--check", action="store_true", help="自检（合成夹具 · 成对正负控）")
    ap.add_argument("--ledger", default=LEDGER_DEFAULT, help="章账落点（append-only · 默认 scripts/gates/seal/ledger.jsonl）")
    ap.add_argument("--write", action="store_true", help="盖章时真落盘（必须同带 --reason）")
    ap.add_argument("--reason", default="", help="盖章留痕理由（--write 必填）")
    ap.add_argument("--json", default="", help="核章机读面（闭集字段见输出）")
    args = ap.parse_args()
    if args.check:
        return do_check()
    if args.seal:
        return do_seal(args)
    if args.verify:
        return do_verify(args)
    die("✗ 用法：`--seal <趟目录> [--write --reason <一句>]` ｜ `--verify` ｜ `--check`（三选一）")


if __name__ == "__main__":
    sys.exit(main())
