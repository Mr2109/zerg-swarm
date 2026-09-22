#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-trustroot-witness.py v1.0.0 —— 信任根可追溯：**只读一致性对拍件**（2.5.11 人签 F 批 · `F1`）

为什么存在（`F1` 的任务原文 + 已拍的落点）
--------------------------------------------------
病灶（`v1.3 §2 S2` · `L-7`）：在册公钥表 `~/.zerg/state/approvals/operator.pub` **不在任何 git 面**
⇒ 改它一个字节，事后**看不出**（无版本、无审计）。`P17` 已拍 = **甲**（= 照设计稿建议）：
**状态目录那枚仍是运行真源** ✓ · **主仓另放一枚已人签的审计副本**（公钥本体）· **换钥时新旧两枚都签**。

本件只出**对拍**：把「运行真源 ↔ 审计副本」两份的 `key_id` / 字节**逐字对拍**，并判三条红线：
① 运行真源**仍不在任何 git 面**（`L-7`）· ② 副本**不许进公开面**（两器显式排除）·
③ **任何运行路径不许读副本**（副本只作审计用）。

★ 本件**只判、不改**：读状态目录、读主仓件、读 `git ls-files`；**一个字节都不写** ✗
（不写副本、不写状态目录、不写公开面）。

判据（W1–W5 · 每条都能单独失败）
--------------------------------------------------
  W1 **运行真源可读且形状对**：`operator.pub` 是 JSON，键集合**逐字**等于四字段闭集
     `alg/pub/key_id/at`，`key_id` 是 **16 位小写十六进制**，`alg` 非空，`at` 带时区偏移。
     ★ 字节数（批前现读 **145 B**）与 `key_id` 具体值 **只作读数、不作判据** ——
       `P17` 明写**换钥会发生**（新旧双签）⇒ 把这两个数钉死，会在换钥当天变成**假红**。
  W1b **在册件认这把钥**（`M-13` 只读那一半 · `D3` 的口）：状态目录 `approvals/*.json` 里
     **至少一枚**件的 `key_id` == 运行真源的 `key_id`；一枚都不认它 ⇒ **红**。
     与真源不一致的**旧件**只交代（`E3`：换钥后旧件作废 + 归档位 ⇒ 不许因它判红）。
  W2 **运行真源 ↔ 审计副本一致**（本件的正题）：
     · 副本**不在** ⇒ **明写「未落地」且 rc≠0**（rc=2）—— **不许**拿「文件不存在」当「没问题」；
     · 副本在且**逐字节相同** ⇒ 绿；
     · 副本在且**不一致** ⇒ **判红**，除非「**已补一枚人签件**」（存在在册件，其 `key_id` ==
       运行真源 `key_id`，且 `approved_at` **晚于**副本在 git 里的最后提交时刻）⇒ 那叫**已人签的变更**
       （`I5`：改信任根要有件），只交代不判红。
  W3 **运行真源仍不在任何 git 面**（`L-7` 现跑）：`git ls-files` 里 `approvals/operator.pub` 命中 **0**。
     （`operator.pub` 的总命中数只作读数 —— 副本落地后它会变成 1，那不是红。）
  W4 **副本不许进公开面**（`P17` 红线 2）：副本**已落地** ⇒ 两器（`publish-public.sh` 的 `EXCLUDES`
     与 `mirror-public-lib.py` 的 `DROP_EXACT`）**都必须显式排除它**，缺一 ⇒ **红**；
     副本**未落地** ⇒ 这一格只交代（无该件可排；**落地那一步必须同批补两器 + 现跑负控**）。
  W5 **任何运行路径不许读副本**（`P17` 红线 1）：已跟踪的源码面（`core/**/*.go` · `agent/**/*.go` ·
     `scripts/**` · `publish/**`）里出现副本路径的字面串 ⇒ **红**（副本只作审计 ⇒ 不许被读）。

退码
--------------------------------------------------
  0 = 全绿 · 1 = 有红（不一致 / L-7 破 / 运行路径读副本 / 已落地却未排除）· 2 = **不给结论**
      （运行真源读不到 **或** 副本**未落地** —— 两种都**明写**，且都 rc≠0）

用法
--------------------------------------------------
  python3 scripts/gates/check-trustroot-witness.py                # 先自检，再扫真目标
  python3 scripts/gates/check-trustroot-witness.py --self-test    # 只跑自检（合成夹具 · 不碰真目标）
  python3 scripts/gates/check-trustroot-witness.py --root <仓根> --state <状态目录> --copy <副本路径>
"""

import argparse
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_ROOT = os.path.dirname(os.path.dirname(HERE))
VERSION = "check-trustroot-witness.py v1.0.0"

RUNNING_REL = os.path.join("approvals", "operator.pub")     # 运行真源（相对状态目录）
# ★ 「审计副本」的**落点占位**（声明式 · `P17` = 甲）：主仓里那一枚**已人签的审计副本**。
#   目录名自带角色（`trustroot-audit-copy`）⇒ 读者一眼知道「这是审计副本，不是运行源」。
#   本任务**不许**把副本本体写进仓 ✗（`I5`：副本进仓是**不可逆档**，真写那一步另需一枚人签件）
#   ⇒ 这里只**声明落点**，并给出「未落地」的判词与「落地时必须同批做的三件事」。
AUDIT_COPY_REL = os.path.join("core", "internal", "contract", "trustroot-audit-copy", "operator.pub")

FIELD_CLOSED = ["alg", "pub", "key_id", "at"]               # 四字段**闭集**（`v1.3 §9.3 I-1` 的现状形状）
KEYID_RE = re.compile(r"^[0-9a-f]{16}$")
AT_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$")
# 两条批前登记读数（**只印不判** —— 见头部 W1 的说明）
BASE_BYTES = 145
BASE_KEY_ID = "8b69ad95939df5cf"
PUBLISH_EXCLUDES_SH = os.path.join("scripts", "build", "publish-public.sh")
PUBLISH_DROP_PY = os.path.join("publish", "mirror-public-lib.py")
SOURCE_GLOBS = ("core", "agent", "scripts", "publish")      # 「运行路径」的扫描面（已跟踪件）


# ── 判定口（纯函数：自检与真目标**共用同一套**）──────────────────────────────────────

def parse_pub(raw):
    """运行真源 → (读数 dict, 问题 list)。"""
    problems = []
    try:
        d = json.loads(raw.decode("utf-8"))
    except Exception as exc:  # noqa: BLE001 —— 解不动 = 判不了
        return None, ["W1 破：运行真源不是合法 JSON（%s）" % exc]
    if not isinstance(d, dict):
        return None, ["W1 破：运行真源不是 JSON 对象"]
    keys = list(d.keys())
    if keys != FIELD_CLOSED:
        problems.append("W1 破：键集合 = %s ≠ 四字段闭集 %s（顺序也照今天那一份）"
                        % (",".join(keys), ",".join(FIELD_CLOSED)))
    kid = d.get("key_id")
    if not isinstance(kid, str) or not KEYID_RE.match(kid):
        problems.append("W1 破：`key_id` = %r 不是 16 位小写十六进制（只读面排不出身份）" % (kid,))
    if not isinstance(d.get("alg"), str) or not d.get("alg"):
        problems.append("W1 破：`alg` 空/非串")
    if not isinstance(d.get("at"), str) or not AT_RE.match(d.get("at", "")):
        problems.append("W1 破：`at` = %r 不带时区偏移（排不出序）" % (d.get("at"),))
    if not isinstance(d.get("pub"), str) or not d.get("pub"):
        problems.append("W1 破：`pub` 空/非串")
    return d, problems


def judge_tickets(tickets, run_kid):
    """W1b：在册件里**至少一枚**认这把钥。tickets = [(path, key_id, approved_at)]。
    ⇒ (reds, nocalls, others)：一枚件都没有 = **未取数**（同门⑰「读不到 ⇒ rc=2」的口径）；
    有件但一枚都不认 = **红**。"""
    if not tickets:
        return [], ["W1b **未取数**：状态目录 `approvals/` 下**一枚在册件都没有** ⇒ 没人在册认这把钥"
                    "（缺件 ≠ 没问题）"], []
    mine = [t for t in tickets if t[1] == run_kid]
    others = [t for t in tickets if t[1] != run_kid]
    reds = []
    if not mine:
        reds.append("W1b 破：在册件 %d 枚，**没有一枚**的 `key_id` == 运行真源的 `%s`"
                    % (len(tickets), run_kid))
    return reds, [], others


def judge_pair(run_raw, copy_raw, copy_stamp, tickets, run_kid):
    """W2：运行真源 ↔ 副本。copy_raw = None 表示**未落地** ⇒ (reds, nocalls, 状态词)。"""
    if copy_raw is None:
        return [], ["W2 **未落地**：审计副本（%s）**不在** ⇒ 本格**不给结论**（rc=2）—— 明写「未落地」，"
                    "**不许**拿「文件不存在」当「没问题」（`P17` 已拍落点；副本本体真写那一步另需一枚人签件）"
                    % AUDIT_COPY_REL], "not_landed"
    if copy_raw == run_raw:
        return [], [], "landed"
    # 不一致 ⇒ 判红，**除非**有一枚晚于副本的人签件（= 已人签的变更，`I5`）
    excused = [t for t in tickets
               if t[1] == run_kid and copy_stamp and str(t[2]) > str(copy_stamp)]
    if excused:
        return [], [], "landed-signed-change"
    return ["W2 破：运行真源与审计副本**不一致**，且**没有**一枚晚于副本、且在册认这把钥的人签件"
            " ⇒ 判红（`I5`：改信任根要有件；副本待随批更新）。副本的在树提交时刻 = %s"
            % (copy_stamp if copy_stamp else "**读不到**（副本未入库 ⇒ 判不了先后 ⇒ 按「无补签」办）")], [], "landed-differ"


def judge_l7(tracked):
    """W3：运行真源仍不在任何 git 面。"""
    hit = [p for p in tracked if p.endswith(RUNNING_REL.replace(os.sep, "/"))]
    return (["W3 破：运行真源**进了 git 面**（`%s`）⇒ `L-7` 不成立" % p for p in hit]
            if hit else []), len([p for p in tracked if p.endswith("operator.pub")])


def judge_public_face(copy_landed, excludes_text, drop_text, copy_rel=None):
    """W4：副本已落地 ⇒ 两器必须显式排除。未落地 ⇒ 只交代。
    `copy_rel` 不是默认落点（`--copy` 夹具 / 仓外）⇒ **只交代不判**（两器的排除表只对默认落点生效）。"""
    if not copy_landed:
        return [], "未落地 ⇒ 无该件可排除（落地那一步必须同批补两器 + 现跑负控）"
    rel = (copy_rel or AUDIT_COPY_REL).replace(os.sep, "/")
    if rel != AUDIT_COPY_REL.replace(os.sep, "/"):
        return [], "副本不在默认落点（%s）⇒ 本格**只交代不判**" % rel
    problems = []
    if rel not in excludes_text:
        problems.append("W4 破：副本已落地，但 `%s` 的 `EXCLUDES` 里**没有**它（旧器会把它发出去）" % PUBLISH_EXCLUDES_SH)
    if rel not in drop_text:
        problems.append("W4 破：副本已落地，但 `%s` 的 `DROP_EXACT` 里**没有**它（镜像器会把它发出去）" % PUBLISH_DROP_PY)
    return problems, "已落地 ⇒ 两器都已显式排除"


def judge_readers(hits):
    """W5：任何运行路径不许读副本。"""
    return ["W5 破：运行路径里出现副本路径的字面串（%s）⇒ 副本被当运行源读了" % h for h in hits]


# ── 取数（只读）───────────────────────────────────────────────────────────────────

def state_dir_of(given=None):
    if given:
        return os.path.abspath(given)
    env = os.environ.get("ZERG_STATE_DIR")
    if env:
        return os.path.abspath(env)
    return os.path.join(os.path.expanduser("~"), ".zerg", "state")


def git_tracked(repo):
    try:
        p = subprocess.run(["git", "-C", repo, "ls-files"], stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE, timeout=120)
    except Exception as exc:  # noqa: BLE001
        return None, "跑 `git ls-files` 失败：%s" % exc
    if p.returncode != 0:
        return None, "`git ls-files` rc=%d" % p.returncode
    return p.stdout.decode("utf-8", "replace").splitlines(), ""


def copy_stamp_of(repo, copy_rel):
    """副本在 git 里的最后提交时刻（未跟踪/未提交 ⇒ None）。"""
    try:
        p = subprocess.run(["git", "-C", repo, "log", "-1", "--format=%cI", "--", copy_rel],
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=120)
        if p.returncode == 0:
            s = p.stdout.decode("utf-8", "replace").strip()
            return s or None
    except Exception:  # noqa: BLE001 —— 取不到时刻 ⇒ 只是当不了「已人签」的例外，不判红
        pass
    return None


def read_all(repo, state, copy_rel=None):
    copy_rel = copy_rel if copy_rel is not None else AUDIT_COPY_REL
    run_path = os.path.join(state, RUNNING_REL)
    if not os.path.isdir(state):
        return None, "状态目录不在（%s）" % state
    if not os.path.exists(run_path):
        return None, "运行真源不在（%s）" % run_path
    with open(run_path, "rb") as f:
        run_raw = f.read()
    approvals = os.path.join(state, "approvals")
    tickets = []
    if os.path.isdir(approvals):
        for name in sorted(os.listdir(approvals)):
            if not name.endswith(".json"):
                continue
            try:
                with open(os.path.join(approvals, name), encoding="utf-8") as f:
                    t = json.load(f)
            except Exception:  # noqa: BLE001 —— 解不动的件不当读数（也不当红）
                continue
            if isinstance(t, dict) and isinstance(t.get("key_id"), str):
                tickets.append((name, t.get("key_id"), t.get("approved_at", "")))
    tracked, why = git_tracked(repo)
    if tracked is None:
        return None, why
    copy_path = os.path.join(repo, copy_rel)
    copy_raw = None
    if os.path.exists(copy_path):
        with open(copy_path, "rb") as f:
            copy_raw = f.read()
    return {"run_path": run_path, "run_raw": run_raw, "tickets": tickets, "tracked": tracked,
            "copy_path": copy_path, "copy_raw": copy_raw, "copy_rel": copy_rel,
            "copy_stamp": copy_stamp_of(repo, copy_rel) if copy_raw is not None else None, "repo": repo}, ""


def scan_readers(repo, tracked, copy_rel):
    """W5：已跟踪的源码面里，副本路径的字面串命中（本件自身排除）。"""
    rel = copy_rel.replace(os.sep, "/")
    d = os.path.dirname(rel)
    hits = []
    self_rel = os.path.relpath(os.path.abspath(__file__), repo).replace(os.sep, "/")
    for p in tracked:
        if p == self_rel:
            continue
        if not p.startswith(tuple(g + "/" for g in SOURCE_GLOBS)):
            continue
        if not p.endswith((".go", ".py", ".sh", ".md", ".tsv", ".json", ".txt", ".yml", ".yaml")):
            continue
        full = os.path.join(repo, p)
        try:
            with open(full, encoding="utf-8", errors="replace") as f:
                txt = f.read()
        except Exception:  # noqa: BLE001
            continue
        if rel in txt or (d and d in txt):
            hits.append(p)
    return hits


def judge_all(r):
    reds, nocalls = [], []
    d, p1 = parse_pub(r["run_raw"])
    reds += p1
    run_kid = (d or {}).get("key_id", "")
    p1b, n1b, others = judge_tickets(r["tickets"], run_kid)
    reds += p1b
    nocalls += n1b
    p2, n2, state2 = judge_pair(r["run_raw"], r["copy_raw"], r["copy_stamp"], r["tickets"], run_kid)
    reds += p2
    nocalls += n2
    p3, pub_hits = judge_l7(r["tracked"])
    reds += p3
    ex_txt = drop_txt = ""
    for rel, key in ((PUBLISH_EXCLUDES_SH, "ex"), (PUBLISH_DROP_PY, "drop")):
        full = os.path.join(r["repo"], rel)
        try:
            with open(full, encoding="utf-8") as f:
                t = f.read()
        except Exception:  # noqa: BLE001
            t = ""
        if key == "ex":
            ex_txt = t
        else:
            drop_txt = t
    p4, why4 = judge_public_face(r["copy_raw"] is not None,
                                 ex_txt, drop_txt, r.get("copy_rel"))
    reds += p4
    reds += judge_readers(r["readers"])
    return reds, nocalls, {"run_kid": run_kid, "state2": state2, "why4": why4,
                           "pub_hits": pub_hits, "others": others, "d": d}


# ── 自检（先自证会红，再扫真目标）────────────────────────────────────────────────

def self_test():
    ok = bad = 0

    def check(cond, msg_ok, msg_bad):
        nonlocal ok, bad
        if cond:
            print("   ✓ %s" % msg_ok)
            ok += 1
        else:
            print("   ✗ %s" % msg_bad)
            bad += 1

    good = json.dumps({"alg": "ed25519", "pub": "AAAA", "key_id": "8b69ad95939df5cf",
                       "at": "2026-09-21T12:43:11+08:00"}).encode()
    d, p = parse_pub(good)
    check(d is not None and not p, "① 正控：四字段闭集 + 16 位小写 hex + 带偏移 ⇒ 不判红（W1）",
          "① 正控失败：好件被判红（%s）" % p)
    bad_keys = json.dumps({"alg": "ed25519", "pub": "AAAA", "key_id": "8b69ad95939df5cf",
                           "at": "2026-09-21T12:43:11+08:00", "extra": 1}).encode()
    check(bool(parse_pub(bad_keys)[1]), "② 负控：多一格（五键）⇒ 判红（闭集破）",
          "② 负控失败：多一格还判绿")
    check(bool(parse_pub(good.replace(b"8b69ad95", b"ZZ69ad95"))[1]),
          "③ 负控：`key_id` 不是 16 位小写 hex ⇒ 判红", "③ 负控失败：坏 key_id 还判绿")

    p, n, others = judge_tickets([("dev_edit.json", "8b69ad95939df5cf", "2026-09-21T12:44:00+08:00")],
                                 "8b69ad95939df5cf")
    check(not p and not n, "④ 正控：在册件认这把钥 ⇒ 不判红（W1b）", "④ 正控失败：认了还判红")
    p2, n2, others2 = judge_tickets([("old.json", "0" * 16, "2026-01-01T00:00:00+08:00")],
                                    "8b69ad95939df5cf")
    check(bool(p2) and len(others2) == 1,
          "⑤ 负控：一枚都不认运行真源 ⇒ 判红；旧钥件只落「交代」不当红（`E3` 换钥）",
          "⑤ 负控失败：没人认还判绿 / 旧钥件被当红")
    p2b, n2b, _ = judge_tickets([], "8b69ad95939df5cf")
    check(bool(n2b) and not p2b, "⑤b 缺件：**一枚在册件都没有** ⇒ 未取数（rc=2），不是判绿",
          "⑤b 失败：缺件被当绿")

    p3, n3, s3 = judge_pair(good, None, None, [], "8b69ad95939df5cf")
    check(bool(n3) and s3 == "not_landed" and "未落地" in n3[0],
          "⑥ 正控（本件的正题）：副本**缺** ⇒ **明写「未落地」且 rc≠0**（不许当「没问题」）",
          "⑥ 正控失败：缺副本还判绿（那就是拿「文件不存在」当「没问题」）")
    p4, n4, s4 = judge_pair(good, good, None, [], "8b69ad95939df5cf")
    check(not p4 and not n4 and s4 == "landed", "⑦ 正控：两份**逐字相同** ⇒ 绿",
          "⑦ 正控失败：一致还判红")
    p5, n5, s5 = judge_pair(good, good.replace(b"AAAA", b"BBBB"), "2026-09-22T00:00:00+08:00",
                            [], "8b69ad95939df5cf")
    check(bool(p5) and s5 == "landed-differ",
          "⑧ **负控（`M-13` 的正控）**：两份**不一致**且没补人签件 ⇒ **判红**",
          "⑧ 负控失败：不一致还判绿（「两份 key_id 不一致却当成一致」就是这条要压的）")
    p6, n6, s6 = judge_pair(good, good.replace(b"AAAA", b"BBBB"), "2026-09-22T00:00:00+08:00",
                            [("new.json", "8b69ad95939df5cf", "2026-09-22T01:00:00+08:00")],
                            "8b69ad95939df5cf")
    check(not p6 and not n6 and s6 == "landed-signed-change",
          "⑨ 正控：不一致但**补了一枚晚于副本的人签件** ⇒ 不判红（`I5` 的「补件 ⇒ 过」）",
          "⑨ 正控失败：补了件还判红")

    p7, n7 = judge_l7(["docs/x.md", "core/a.go"])
    check(not p7 and n7 == 0, "⑩ 正控：git 面里没有 `approvals/operator.pub` ⇒ 不判红（W3 · `L-7`）",
          "⑩ 正控失败：干净还判红")
    check(bool(judge_l7(["core/internal/contract/trustroot-audit-copy/operator.pub"])[0]) is False,
          "⑪ 正控：**副本**在 git 面里**不算**破 `L-7`（只有运行真源进 git 才是破）",
          "⑪ 正控失败：把副本当运行真源判红")
    check(bool(judge_l7(["state/approvals/operator.pub"])[0]),
          "⑫ 负控：`approvals/operator.pub` 进了 git 面 ⇒ 判红", "⑫ 负控失败：进 git 面还判绿")

    p8, w8 = judge_public_face(False, "", "")
    check(not p8 and "未落地" in w8, "⑬ 正控：副本**未落地** ⇒ W4 只交代（不判红）",
          "⑬ 正控失败：未落地还判红")
    p9, w9 = judge_public_face(True, "x\n", "y\n")
    check(len(p9) == 2, "⑭ 负控：副本**已落地**而两器**都没排** ⇒ 判红（两条）",
          "⑭ 负控失败：落地了没排还判绿")
    p10, w10 = judge_public_face(True, AUDIT_COPY_REL.replace(os.sep, "/") + "\n",
                                 AUDIT_COPY_REL.replace(os.sep, "/") + "\n")
    check(not p10, "⑮ 正控：副本已落地且**两器都显式排除** ⇒ 不判红（`P17` 红线 2）",
          "⑮ 正控失败：两器都排了还判红")

    check(not judge_readers([]), "⑯ 正控：运行路径里没有副本路径 ⇒ 不判红（W5）",
          "⑯ 正控失败：干净还判红")
    check(bool(judge_readers(["core/cmd/zerg/x.go"])),
          "⑰ 负控：运行路径里读了副本 ⇒ 判红（`P17` 红线 1）", "⑰ 负控失败：读了还判绿")

    print("   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节）" % (ok, bad))
    return bad == 0


# ── 主 ───────────────────────────────────────────────────────────────────────────

def scan(repo, state, copy_rel):
    r, why = read_all(repo, state, copy_rel)
    if r is None:
        print("⛔ 未取数：%s ⇒ **不出结论**（rc=2）" % why)
        return 2
    r["readers"] = scan_readers(repo, r["tracked"], r["copy_rel"])
    reds, nocalls, info = judge_all(r)
    rel = copy_rel.replace(os.sep, "/")
    print("── 读数（只读 · 每处带口径）")
    print("   ① 运行真源（%s）= **%d B** · sha256 = %s" % (
        r["run_path"], len(r["run_raw"]), __import__("hashlib").sha256(r["run_raw"]).hexdigest()))
    print("   ①-补 批前登记读数：**%d B** · `key_id=%s`（**只印不判** —— `P17` 明写换钥会发生）"
          % (BASE_BYTES, BASE_KEY_ID))
    d = info["d"] or {}
    print("   ①-形 `alg=%s` · `key_id=%s` · `at=%s`（四字段闭集 %s）"
          % (d.get("alg"), d.get("key_id"), d.get("at"), ",".join(FIELD_CLOSED)))
    print("   ② 审计副本落点（%s）：%s" % (rel, "**未落地**" if r["copy_raw"] is None
                                       else "%d B · %s" % (len(r["copy_raw"]), info["state2"])))
    print("   ②-补 两器排除口径（W4）：%s" % info["why4"])
    print("   ③ `git ls-files` 里 `approvals/operator.pub` 命中 = %d（`L-7` 要求 0）；"
          "`operator.pub` 总命中 = %d（**只印不判**：副本落地后为 1）"
          % (len([p for p in r["tracked"] if p.endswith(RUNNING_REL.replace(os.sep, "/"))]),
             info["pub_hits"]))
    print("   ④ 在册件 %d 枚；其中认运行真源的 %d 枚 · 不认的（旧钥件 · 只交代）%d 枚"
          % (len(r["tickets"]), len(r["tickets"]) - len(info["others"]), len(info["others"])))
    if info["others"]:
        print("   ·  交代：与运行真源不一致的在册件 = %s（`E3`：换钥后旧件作废 + 归档位 ⇒ 不当红）"
              % ", ".join(o[0] for o in info["others"]))
    print("   ⑤ 运行路径里副本路径的字面串命中 = %d（W5 要求 0）%s"
          % (len(r["readers"]), ("：%s" % ", ".join(r["readers"])) if r["readers"] else ""))
    print("   ⑥ 不可机检（如实标 · 不许写成命令判据）：**副本该放哪个仓面谁签它**是人定的（`P17`）；"
          "**两份 `key_id` 一致 ≠ 信任根历史可信**（历史面靠副本逐次人签）")
    if reds or nocalls:
        rc = 1 if reds else 2
        print("⇒ 门禁%s（rc=%d）：红 %d 条 · 不给结论 %d 条"
              % ("红" if rc == 1 else "不给结论", rc, len(reds), len(nocalls)))
        for p in reds:
            print("   ✗ %s" % p)
        for p in nocalls:
            print("   ⛔ %s" % p)
        return rc
    print("⇒ 全绿（rc=0）：运行真源形状对 · 在册件认它 · 副本逐字一致 · `L-7` 仍成立 · "
          "副本未进公开面 · 无运行路径读副本")
    return 0


def main():
    ap = argparse.ArgumentParser(description=VERSION)
    ap.add_argument("--root", default=DEFAULT_ROOT, help="仓根（默认 = 本脚本的上两级）")
    ap.add_argument("--state", default=None, help="状态目录（默认 = `ZERG_STATE_DIR` > `~/.zerg/state`）")
    ap.add_argument("--copy", default=None, help="审计副本路径（默认 = `P17` 甲 声明的落点占位）")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（只有已自证过的调用方才该用）")
    args = ap.parse_args()
    repo = os.path.abspath(args.root)
    state = state_dir_of(args.state)
    print(VERSION)
    if not args.no_self_test and not self_test():
        return 1
    if args.self_test:
        return 0
    print("── 真目标：%s（状态目录 %s）" % (repo, state))
    if args.state is None and os.environ.get("ZERG_STATE_DIR"):
        print("★★ 状态目录是**环境变量 `ZERG_STATE_DIR` 给的**（%s）⇒ 本读数是**沙箱**读数，"
              "**不是**真状态目录 `~/.zerg/state`（闸里不设这个变量；要读真源请显式 `--state`）"
              % os.environ["ZERG_STATE_DIR"])
    return scan(repo, state, args.copy if args.copy is not None else AUDIT_COPY_REL)


if __name__ == "__main__":
    sys.exit(main())
