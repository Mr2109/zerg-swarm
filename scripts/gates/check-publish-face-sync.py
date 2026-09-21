#!/usr/bin/env python3
"""发布面同步门禁：白名单导出面 × 两器的排除口径 × 私有面黑名单 —— 三者不许漂移。

**为什么需要它（2026-09-16 批 2'.6 实测撞上的形态，不是假想）**：
`publish/private-paths.txt` 黑名单里后来收进的三个「发布机制自身」脚本
（`scripts/build/publish-preflight.sh` / `scripts/gates/check-public-tree-hazards.py` /
`scripts/gates/check-placeholder-residue.py`）只进了镜像器自带的 `DROP_EXACT`，
**没有**同步进 `scripts/build/publish-public.sh` 的 `EXCLUDES=( … )`（B8 声明的「单一真源」，
镜像器解析的就是这个数组）⇒ **旧器（应急后备通道）真跑时把这三个文件导出，随后被自己的
私有面门禁抓住而整批自中止**（一个字节都不推）；两器口径静默漂移，而两器一致性验收
`scripts/build/test-publish-parity.sh` 要跑约 6 分钟才看得见。本门禁是**秒级**的那一面镜子。

**判据（任一不满足 ⇒ rc=1；用法/缺件/无区分度 ⇒ rc=2）**：
  ① 漂移 —— 白名单选中的路径里，镜像器**丢**（`map_path` 为 None）而旧器**不排**（不在
     `EXCLUDES`）⇒ 红。旧器真跑会把该文件导进快照 ⇒ 被私有面门禁抓住 ⇒ 整批自中止。
  ② 两边都漏 —— 命中私有面黑名单、且**两器都没处理**（旧器不排 + 镜像器不丢）⇒ 红。
     两器都会把私有面路径带进公开面（旧器门禁 / 镜像器末树门禁各自中止）。
  ③ 有区分度（守卫）—— 黑名单解析为空 / 白名单为空 / 白名单里没有 `wall/` / 候选集里
     没有一条 `wall/` 路径 / EXCLUDES 一条都解析不出来 ⇒ **rc=2 拒绝给结论**
     （「配置没了所以全绿」是最难发现的假绿）。
  ④ `--self-test` 必须先过（本门禁**自检不过就拒绝扫真目标**，与
     `scripts/scan-replacement-residue.py` 同一纪律）。
  ⑤ 清单自身语法 —— 白名单 / 私有面黑名单 / 豁免清单 / 替换规则表 四件里，**末行未闭合**
     （句子没写完 · 括号不配平 · 没有结尾换行）或**条目格式不合格**（少一列 / 带空白 / 绝对路径
     ⇒ 解析器**静默跳过**那一条）⇒ **rc=2 拒绝给结论**（真源读不全 ⇒ 绿就是假绿）。逐条口径见
     下面 `list_syntax_problems` 的注释。
  ⑥ **标记表 × 两器**（2026-09-21 公开线⑤ · 设计稿 §2.2 **方案 b** 的第四张清单 · §4.5）——
     `scripts/公开标记.tsv` 里 `public=no` 的行，链上**仍会发它**（不在 `EXCLUDES` —— 两器共用那把闸，
     旧器会把它导进快照；镜像器另叠的 `map_path` 也没丢 ⇒ 这一件还会**进产出树**）⇒ **红**，逐条点名。
     为什么单立这一格：方案 b 让**两套真源并存**（表的意图 vs 链的清单），本判据把「表 ↔ 两器」的
     漂移从「要跑约 6 分钟的两器一致性验收才看得见」压成**秒级**。★ 判据① 只管「镜像器丢、旧器不排」
     那一向，② 只管私有面黑名单那一维 ⇒ 「表说 no、链照发」这一维此前没人在管，由本判据补上。
     **fail-closed**：表读不到 / 表不合规（`load_table` 抛 `TableError`）/ 表里一条 `public=no` 都没有
     ⇒ **rc=2 不给结论**（「表全放行」在视觉上与正常表同形 —— 那是最难发现的一类假绿）。
     ★ 处置只有两条（照设计稿 §3.3 纪律 3）：收进 `EXCLUDES` + `DROP_EXACT`，或**改表**；
       **不许**放宽本判据来凑绿 ✗。
     ★ 2026-09-21 补（前一批的已知缺口 ①）：候选集必须**看得见非 ASCII 名件** —— `tracked_files` 给
       `git ls-files` 加 `-c core.quotepath=false`。不加时这类件被转义成 `"scripts/\345…"` ⇒ 白名单
       失配 ⇒ 它们在候选集里缺席 ⇒ 判据⑥ 对它们**静默无牙**（表说 no、链上照发，而门禁全绿）。

**单一真源（一律复用，绝不另抄一份解析器）**：
  · 白名单解析 / EXCLUDES 解析 —— `scripts/gates/check-history-secrets.py`
    （`load_whitelist` / `published_path` / `load_excludes` / `excluded`；镜像器解析的也是它）
  · 私有面黑名单解析 + `!` 豁免 —— `scripts/gates/check-public-tree-private.py`
  · 镜像器的丢弃口径 —— `publish/mirror-public-lib.py` 的 `map_path`（它内含
    `DROP_EXACT` / `DROP_PREFIX`）

**口径（为什么 `publish/` 下的路径不在本门禁内）**：旧器对 `publish/` 是**结构性**处置
（先把整个 `publish/` 导进快照、再 `rm -rf` 掉，只留 overlay 拷出来的那几件），不是
`EXCLUDES` 驱动的 ⇒ 拿 `EXCLUDES` 去比它没有意义。故候选集**排除 `publish/`**，
并在输出里明写这条边界；`publish/` 的映射一致性由 `publish/parity-contract.tsv` 与
`publish/parity-compare.py` 负责。

用法：
    python3 scripts/gates/check-publish-face-sync.py                 # 查本仓（自动先跑自检）
    python3 scripts/gates/check-publish-face-sync.py --repo <仓根>    # 查别的仓（夹具/演练）
    python3 scripts/gates/check-publish-face-sync.py --self-test      # 只跑合成夹具自检
可选参数（默认取 `--repo` 下的标准位置）：`--whitelist` / `--private-paths` /
`--publish-sh` / `--mirror-lib`。
"""
import argparse
import importlib.util
import os
import shutil
import subprocess
import sys
import tempfile

# 本门禁**自己**所在的仓（复用的门禁脚本一律从这里加载，不随 --repo 走）
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

WALL_ENTRY = "wall/"


def load_module(name, path):
    if not os.path.exists(path):
        print("✗ 缺少要复用的资产：%s（本门禁不接受缺件）" % path, file=sys.stderr)
        raise SystemExit(2)
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        print("✗ 无法加载 %s" % path, file=sys.stderr)
        raise SystemExit(2)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


HSEC = load_module("face_sync_hsec", os.path.join(REPO, "scripts", "gates", "check-history-secrets.py"))
PRIV = load_module("face_sync_priv", os.path.join(REPO, "scripts", "gates", "check-public-tree-private.py"))
# 判据⑥（表 × 两器）复用**表自己的加载器**（`check-public-marks.py` 的 `load_table` / `TableError`）——
# 单一真源，绝不另抄第二份表解析器（那张表的合规口径由提交闸上的 B15 管，本门只读它的判定结果）。
MARKS = load_module("face_sync_marks", os.path.join(REPO, "scripts", "gates", "check-public-marks.py"))
MARKS_TABLE_REL = "scripts/公开标记.tsv"


def tracked_files(repo):
    # ★ 2026-09-21（公开线⑥ · 补前一批的已知缺口 ①）：`-c core.quotepath=false` **必须给** ——
    #   `git ls-files` 默认（`core.quotepath=true`）把非 ASCII 文件名输出成 `"scripts/\345\205\254…"`
    #   这种**带引号的转义形态** ⇒ 白名单前缀匹配当场失配 ⇒ 这**一整类件在候选集里根本不存在**：
    #   表把它标 `no`、链上照发，判据⑥ 也一声不响（**假绿** —— 最难发现的那一类）。
    #   牙齿：`--self-test` 的 `marks-no-nonascii-file-sent` 那一格（修前 rc=0 · 修后必须 rc=1）。
    r = subprocess.run(["git", "-C", repo, "-c", "core.quotepath=false", "ls-files"],
                       capture_output=True, text=True)
    if r.returncode != 0:
        print("✗ git ls-files 失败：%s（rc=%d）" % (repo, r.returncode), file=sys.stderr)
        raise SystemExit(2)
    return [ln for ln in r.stdout.splitlines() if ln]


def require_paths(paths):
    """返回缺件的键列表（缺件一律 rc=2，绝不静默跳过）。"""
    return [k for k, v in paths.items() if not os.path.exists(v)]


def classify(repo, paths):
    """→ (candidates, mirror_dropped, private_hits, wall, entries, excludes, marks, mirror)

    `marks` = 表的逐行读回（`[(行号, 仓相对路径, public, 理由)]`）；表不合规 ⇒ 抛 `MARKS.TableError`，
    由调用方按 **rc=2 不给结论** 处理（照 `check-public-marks.py` 的 A 档口径）。
    """
    entries = HSEC.load_whitelist(paths["whitelist"])
    excludes = HSEC.load_excludes(paths["publish_sh"])
    pdirs, pfiles, pallowed = PRIV.load_rules(paths["private_paths"])
    mirror = load_module("face_sync_mirror", paths["mirror_lib"])
    marks = MARKS.load_table(paths["marks_table"])

    def private_hit(rel):
        if PRIV.is_allowed(rel, pallowed):
            return None
        for d in pdirs:
            if rel.startswith(d):
                return d
        if rel in pfiles:
            return rel
        return None

    candidates, dropped, hits, wall = [], [], [], []
    for rel in tracked_files(repo):
        if not HSEC.published_path(rel, entries):
            continue
        if rel.startswith("publish/"):      # 结构性处置，不在本门禁口径内（见文件头注释）
            continue
        candidates.append(rel)
        if rel.startswith(WALL_ENTRY):
            wall.append(rel)
        m = private_hit(rel)
        if m:
            hits.append((rel, m))
        if mirror.map_path(rel) is None:    # 镜像器自带清单丢弃
            dropped.append(rel)
    return candidates, dropped, hits, wall, entries, excludes, marks, mirror


# ── 判据⑤：清单自身语法（末行未闭合 / 条目格式不合格 ⇒ 不给结论）────────────────────────────
# 为什么单立一格：本门禁读的四件（白名单 · 私有面黑名单 · 豁免清单 · 替换规则表）都是**纯文本清单**，
# 而解析器对不合格行一律**静默跳过**（`check-history-secrets.py` 的 `load_whitelist` 只收非空非注释行、
# `load_rules` 少于 2 个 TAB 字段即 `continue`）⇒ 一句写了一半的注释、少一列的行**不会报错，只会悄悄少
# 一条**，门禁照绿。现成的形态（2026-09-21）：`publish/whitelist.txt` 最后一句以「；」结尾、全角括号
# 11 开 / 10 闭 —— 那句话没写完，而整条发布链一声不响地照跑。
#
# 口径（**清单类** .txt 三件）：
#   R1 必须以换行结尾（末行被截断 ⇒ 报错）
#   R2 末条内容行不得以「句子未完」的收尾字符结尾（；;、，,：:），且该行全角括号必须配平
#   R3 整件全角括号「（）」计数必须配平（注释可跨行 ⇒ 按**整件**配平，不逐行）
#   R4 条目格式：白名单/黑名单条目必须是**仓相对路径**（不以 / 或 ~ 开头 · 不含空白 · 无 `..` 段）；
#      黑名单允许 `!` 前缀（显式豁免，见它自己的语法说明）；豁免清单每行必须 `镜像路径<TAB>理由`
# 口径（**规则表** .tsv 一件）：只判 R1 + R5（≥2 个 TAB 字段 —— 少一列会被 `load_rules` 静默跳过），
#   **不判**括号与收尾标点：它的值列本来就允许任意字符（含正则里的 `（`）。
# 不合格 ⇒ **rc=2 不给结论**（「真源读不全」；与「两器漂移」的 rc=1 分开）。
UNCLOSED_TAIL = "；;、，,：:"
LIST_FILES = (("白名单", "whitelist.txt"), ("私有面黑名单", "private-paths.txt"),
              ("豁免清单", "mirror-allow.txt"))
TSV_FILES = (("替换规则表", "replace-rules.tsv"),)


def is_repo_rel(rel):
    """条目的机械形状：非空 · 不以 / 或 ~ 开头 · 不含空白 · 没有 `..` 段。"""
    if not rel or rel.startswith("/") or rel.startswith("~"):
        return False
    if any(ch.isspace() for ch in rel):
        return False
    return ".." not in rel.split("/")


def list_syntax_problems(paths):
    """清单类文件的语法问题 → [(文件名, 行号(0=整件), 说明)]。只读，不落盘。"""
    pubdir = os.path.dirname(paths["whitelist"])
    problems = []

    def read_bytes(p):
        try:
            with open(p, "rb") as fh:
                return fh.read()
        except OSError:
            return None

    for label, name in LIST_FILES + TSV_FILES:
        raw = read_bytes(os.path.join(pubdir, name))
        if raw is None:
            problems.append((name, 0, "%s 读不到（缺件 ≠ 绿）" % label))
            continue
        text = raw.decode("utf-8", "replace")
        if not text.strip():
            problems.append((name, 0, "%s 是空的（空清单 = 什么都没配 ⇒ 不给结论）" % label))
            continue
        if not raw.endswith(b"\n"):
            problems.append((name, text.count("\n") + 1, "末行没有结尾换行 ⇒ 末行可能被截断（未闭合）"))
        rows = [(i, ln) for i, ln in enumerate(text.split("\n"), 1) if ln.strip()]
        last_i, last = rows[-1]
        if name in ("whitelist.txt", "private-paths.txt", "mirror-allow.txt"):
            tail = last.rstrip()[-1]
            if tail in tuple(UNCLOSED_TAIL):
                problems.append((name, last_i, "末条内容行以 %s 结尾（句子没写完 ⇒ 未闭合）" % tail))
            if last.count("（") != last.count("）"):
                problems.append((name, last_i, "末条内容行全角括号不配平（%d 开 / %d 闭 ⇒ 未闭合）"
                                 % (last.count("（"), last.count("）"))))
            if text.count("（") != text.count("）"):
                problems.append((name, 0, "整件全角括号不配平（%d 开 / %d 闭 ⇒ 有注释没写完）"
                                 % (text.count("（"), text.count("）"))))
        for i, ln in rows:
            ln = ln.strip()
            if ln.startswith("#"):
                continue
            if name == "mirror-allow.txt":
                key, _, why = ln.partition("\t")
                if not why.strip():
                    problems.append((name, i, "豁免条目不是『镜像路径<TAB>理由』（理由空 = 命中了也不说为什么）：%r" % ln[:60]))
                elif not is_repo_rel(key.strip()):
                    problems.append((name, i, "豁免条目的键不是仓相对路径：%r" % key.strip()[:60]))
            elif name == "replace-rules.tsv":
                if len(ln.split("\t")) < 2:
                    problems.append((name, i, "规则行少于 2 个 TAB 字段（解析器会**静默跳过**这一条）：%r" % ln[:60]))
            else:
                body = ln[1:].strip() if ln.startswith("!") else ln
                if not body:
                    problems.append((name, i, "`!` 后面没有路径（空豁免）"))
                elif not is_repo_rel(body):
                    problems.append((name, i, "条目不是仓相对路径（解析器只当它不存在）：%r" % ln[:60]))
    return problems


def evaluate(repo, paths):
    """→ (rc, lines)。本函数只做判定，不落盘、不 exec。"""
    missing = require_paths(paths)
    if missing:
        return 2, ["  ✗ 缺件（%s）⇒ 拒绝给结论（rc=2）" % "、".join(sorted(missing))]
    try:
        cand, dropped, hits, wall, entries, excludes, marks, mirror = classify(repo, paths)
    except MARKS.TableError as e:
        return 2, ["  ✗ 标记表不合规（%s）⇒ 拒绝给结论（rc=2）—— 用坏表判出的绿没有意义（A 档口径）" % e]
    pdirs, pfiles, pallowed = PRIV.load_rules(paths["private_paths"])
    no_marks = [rel for _i, rel, pub, _why in marks if pub == "no"]
    lines = []
    lines.append("发布面同步门禁 —— 候选 %d 条（白名单 %d 条 · 黑名单 %d 目录+%d 文件 · EXCLUDES %d 条 · "
                 "标记表 %d 行/`no` %d 条）"
                 % (len(cand), len(entries), len(pdirs), len(pfiles), len(excludes),
                    len(marks), len(no_marks)))
    lines.append("  候选里的 wall/ 路径：%d 条" % len(wall))
    lines.append("  镜像器自带清单丢弃（map_path=None）：%d 条" % len(dropped))
    lines.append("  命中私有面黑名单：%d 条" % len(hits))

    # ③ 有区分度守卫（先判，避免「配置没了所以全绿」）
    if not cand:
        return 2, lines + ["  ✗ 守卫：候选集为空 ⇒ 拒绝给结论（rc=2）"]
    if not entries:
        return 2, lines + ["  ✗ 守卫：白名单解析为空 ⇒ 拒绝给结论（rc=2）"]
    if not excludes:
        return 2, lines + ["  ✗ 守卫：EXCLUDES 一条都解析不出来 ⇒ 拒绝给结论（rc=2）"]
    if not pdirs and not pfiles:
        return 2, lines + ["  ✗ 守卫：私有面黑名单解析为空 ⇒ 拒绝给结论（rc=2）"]
    if WALL_ENTRY not in entries and not any(e.rstrip("/") == WALL_ENTRY.rstrip("/") for e in entries):
        return 2, lines + ["  ✗ 守卫：白名单里没有 %s ⇒ 拒绝给结论（rc=2）" % WALL_ENTRY]
    if not wall:
        return 2, lines + ["  ✗ 守卫：候选集里没有一条 %s 路径 ⇒ 拒绝给结论（rc=2）" % WALL_ENTRY]
    if not no_marks:
        return 2, lines + ["  ✗ 守卫：标记表里一条 `public=no` 都没有 ⇒ 判据⑥ 无区分度（空转 = 假覆盖）"
                           "⇒ 拒绝给结论（rc=2）"]

    # ⑤ 清单自身语法（末行未闭合 / 条目格式不合格 ⇒ **不给结论**；见本函数上面那一段注释）
    syn = list_syntax_problems(paths)
    lines.append("  清单语法（白名单/私有面黑名单/豁免清单/替换规则表 四件）：%d 处不合格" % len(syn))
    if syn:
        for name, ln, why in syn[:40]:
            lines.append("  ✗ 判据⑤ %s:%s —— %s" % (name, str(ln) if ln else "整件", why))
        lines.append("  ⇒ 真源读不全（不合格行会被解析器**静默跳过** ⇒ 门禁假绿）⇒ 不给结论（rc=2）")
        return 2, lines

    bad = 0
    # ① 漂移：镜像器丢、旧器不排 ⇒ 旧器真跑会导出它（自中止）
    drift = [p for p in dropped if not HSEC.excluded(p, excludes)]
    if drift:
        bad = 1
        lines.append("  ✗ 判据① 漂移 %d 条 —— 镜像器丢、旧器不排（旧器真跑会导出 ⇒ 被私有面门禁抓住而整批自中止）：" % len(drift))
        for p in drift[:40]:
            lines.append("      %s" % p)
    else:
        lines.append("  ✅ 判据① 漂移：0 条（镜像器丢弃的路径旧器全都排除了）")

    # ② 两边都漏：命中黑名单且两器都没处理
    leak = [(p, m) for p, m in hits if not HSEC.excluded(p, excludes) and p not in dropped]
    if leak:
        bad = 1
        lines.append("  ✗ 判据② 两边都漏 %d 条 —— 命中私有面黑名单且两器都没处理（两器都会把私有面路径带进公开面）：" % len(leak))
        for p, m in leak[:40]:
            lines.append("      %-60s ← 规则 %s" % (p, m))
    else:
        lines.append("  ✅ 判据② 两边都漏：0 条（命中黑名单的路径两器都已处理）")

    # ⑥ 标记表 × 两器（设计稿 §2.2 方案 b 的第四张清单 · §4.5「表说 no、而两器都发 ⇒ 红」）
    # 「两器会发」取的是**有效判定**，不是单看某一层清单：两器共用的那把闸是 EXCLUDES
    # （镜像器的 `Assets.excluded` 解析的也是它 —— 故 EXCLUDES 收了的件，两器都不会发）；
    # 镜像器另叠一层 `map_path`（映射失败 / 自带丢弃清单）⇒ **旧器会发 ⊇ 镜像器会发**。
    # 判红那一格取「旧器会发」：它一发，应急快照通道就有它；镜像器会发则**一定会进产出树** ⇒ 逐条打印。
    # 判据① 只管「镜像器丢、旧器不排」那一向 ⇒ 本判据补的是「表 ↔ 链」这一维（表说 no 而链照发）。
    no_set = set(no_marks)
    marks_cand = [p for p in cand if p in no_set]
    unhandled = [p for p in marks_cand if not HSEC.excluded(p, excludes)]
    if unhandled:
        bad = 1
        n_mirror = sum(1 for p in unhandled if mirror.map_path(p) is not None)
        lines.append("  ✗ 判据⑥ 表说 no、链上却仍会发 %d 条（其中镜像器仍发 %d 条 ⇒ 会进产出树）："
                     % (len(unhandled), n_mirror))
        for p in unhandled[:40]:
            lines.append("      %s（%s）"
                         % (p, "镜像器仍发 ⇒ 会进产出树" if mirror.map_path(p) is not None
                            else "旧器映射层丢，但 EXCLUDES 没收 ⇒ 快照通道会带上"))
        lines.append("      ⇒ 处置只有两条（照设计稿 §3.3 纪律 3）：收进 EXCLUDES + DROP_EXACT，或**改表**；"
                     "**不许**放宽本判据来凑绿 ✗")
    else:
        lines.append("  ✅ 判据⑥ 表说 no 的件两器都已排除：%d 条（表 `no` %d 行 · 候选集内 %d 条）"
                     % (len(marks_cand), len(no_marks), len(marks_cand)))
    return bad, lines


# ── 自检：合成夹具（每条都能红）────────────────────────────────────────────────────────
FIXTURE_LIB = '''# 夹具用的最小 mirror lib（只提供本门禁读的三个名字）
DROP_PREFIX = ()
DROP_EXACT = {%s}


def map_path(rel):
    if rel in DROP_EXACT or any(rel.startswith(p) for p in DROP_PREFIX):
        return None
    return rel
'''

FIXTURES = [
    # (名字, whitelist, private_paths, excludes, drop_exact, 期望 rc[, 选项])
    ("ok-both-dropped", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 0),
    ("drift-mirror-only", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/unrelated-but-real.py"], ["scripts/mech.py"], 1),
    ("both-leak", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/unrelated-but-real.py"], [], 1),
    ("no-private-rules", ["scripts/", "wall/"], [],
     ["scripts/mech.py"], ["scripts/mech.py"], 2),
    ("whitelist-without-wall", ["scripts/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 2),
    ("candidate-without-wall", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 2, {"no_wall_files": True}),
    ("missing-whitelist", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 2, {"delete_whitelist": True}),
    # 判据⑤ 清单语法（三格都必须 rc=2：末行未闭合 / 条目不是仓相对路径 / 豁免条目没有 TAB+理由）
    ("list-lastline-unclosed", ["scripts/", "wall/", "# v2.5.9 残件：某共享模块（裁决逻辑；"],
     ["scripts/mech.py"], ["scripts/mech.py"], ["scripts/mech.py"], 2),
    ("list-bad-entry", ["scripts/", "wall/", "/abs/path"],
     ["scripts/mech.py"], ["scripts/mech.py"], ["scripts/mech.py"], 2),
    ("list-allow-no-reason", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 2,
     {"allow_text": "# 夹具豁免清单\nscripts/mech.py 夹具理由（没有 TAB）\n"}),
    # 判据⑥（表 × 两器 · 2026-09-21 公开线⑤）四格：正控 / 层边界（EXCLUDES 覆盖）/ 两器都发 / 表缺件 / 无 no 行
    ("marks-no-both-handled", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 0),
    # 层边界那一格：EXCLUDES 收了、镜像器的 map_path 那层没收 ⇒ 两器**都不发**（EXCLUDES 是两器共用那把闸）
    # ⇒ 绿 —— 钉住「判据⑥ 判的是**有效判定**，不是单看某一层清单」
    ("marks-no-excludes-covers-mirror-layer", ["scripts/", "wall/"], ["gateway/fleet.yaml"],
     ["scripts/mech.py"], [], 0),
    # 「两器都发」那一格：EXCLUDES 不收 + 镜像器 map_path 也不丢 ⇒ 判据① 与判据② 都沉默
    # （dropped/hits 都空），**只有判据⑥ 会红** ⇒ 这一格专钉 ⑥ 的牙齿（不是靠邻判据顺手红的）
    ("marks-no-both-send", ["scripts/", "wall/"], ["gateway/fleet.yaml"],
     ["scripts/unrelated-but-real.py"], [], 1),
    ("marks-table-missing", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 2, {"delete_marks": True}),
    ("marks-no-rows-empty", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 2,
     {"marks_text": "文件\tpublic\t理由\nscripts/keep.py\tyes\t夹具：只有 yes 行\n"}),
    # 缺口 ① 那一格（2026-09-21 公开线⑥ 补）：**非 ASCII 名件**被表标 `no`、两器却都发 ⇒ **必须 rc=1**。
    # 修 `tracked_files` 之前这一格是 **rc=0（假绿）**：`git ls-files` 默认把非 ASCII 名转义成
    # `"scripts/\345…"` ⇒ 白名单前缀失配 ⇒ 该件在候选集里不存在 ⇒ 判据⑥ 根本看不见它。
    # （夹具里 `scripts/公开标记.tsv` 就是表自身那个非 ASCII 名 —— 它另证「这类名现在确实进候选集」。）
    ("marks-no-nonascii-file-sent", ["scripts/", "wall/"], ["scripts/mech.py"],
     ["scripts/mech.py"], ["scripts/mech.py"], 1,
     {"tracked_extra": ["scripts/公开核对.tsv"],
      "marks_text": "文件\tpublic\t理由\n"
                    "scripts/mech.py\tno\t夹具：B 类（合成 · 判据⑥ 的被判对象）\n"
                    "scripts/公开核对.tsv\tno\t夹具：非 ASCII 名 + 表说 no + 两器都发（缺口 ① 的那一格）\n"
                    "scripts/keep.py\tyes\t夹具：A 类（合成）\n"}),
]

# 夹具里被跟踪的件（另可逐格用 opts["tracked_extra"] 加 —— 缺口 ① 那一格要一枚**非 ASCII 名**件）。
FIXTURE_TRACKED = ["scripts/mech.py", "scripts/keep.py", "wall/src/lib.rs", "wall/Cargo.toml"]

# 判据⑥ 的夹具表（默认值）：`scripts/mech.py` 判 `no` —— 各格的正控要求「两器都不发它」
DEFAULT_MARKS = ("文件\tpublic\t理由\n"
                 "scripts/mech.py\tno\t夹具：B 类（合成 · 判据⑥ 的被判对象）\n"
                 "scripts/keep.py\tyes\t夹具：A 类（合成）\n")


def make_fixture(root, wl, priv, excl, drop, opts):
    os.makedirs(os.path.join(root, "publish"), exist_ok=True)
    os.makedirs(os.path.join(root, "scripts", "build"), exist_ok=True)
    os.makedirs(os.path.join(root, "wall/src"), exist_ok=True)
    with open(os.path.join(root, "publish", "whitelist.txt"), "w", encoding="utf-8") as fh:
        fh.write("# 夹具白名单\n" + "\n".join(wl) + "\n")
    with open(os.path.join(root, "publish", "private-paths.txt"), "w", encoding="utf-8") as fh:
        fh.write("# 夹具黑名单\n" + "\n".join(priv) + "\n")
    ex_lines = "\n".join('  "%s"' % e for e in excl)
    with open(os.path.join(root, "scripts", "build", "publish-public.sh"), "w", encoding="utf-8") as fh:
        fh.write("#!/usr/bin/env bash\nEXCLUDES=(\n%s\n)\n" % ex_lines)
    with open(os.path.join(root, "publish", "mirror-public-lib.py"), "w", encoding="utf-8") as fh:
        fh.write(FIXTURE_LIB % ", ".join('"%s"' % d for d in drop))
    # 判据⑤ 要读的四件：另两件（规则表 / 豁免清单）也给夹具默认值（合法值 ⇒ 不干扰原有各格）
    with open(os.path.join(root, "publish", "replace-rules.tsv"), "w", encoding="utf-8") as fh:
        fh.write("# 夹具替换规则表\nfoo\tbar\t夹具\n")
    with open(os.path.join(root, "publish", "mirror-allow.txt"), "w", encoding="utf-8") as fh:
        fh.write(opts.get("allow_text", "# 夹具豁免清单\nscripts/mech.py\t夹具理由\n"))
    # 判据⑥ 要读的表（`marks_text` 换内容；`delete_marks` 造「表缺件」那一格）
    marks_path = os.path.join(root, MARKS_TABLE_REL)
    with open(marks_path, "w", encoding="utf-8") as fh:
        fh.write(opts.get("marks_text", DEFAULT_MARKS))
    tracked = list(FIXTURE_TRACKED) + list(opts.get("tracked_extra", []))
    if opts.get("no_wall_files"):
        tracked = [t for t in tracked if not t.startswith("wall/")]
    for rel in tracked:
        p = os.path.join(root, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        open(p, "w", encoding="utf-8").write("x\n")
    subprocess.run(["git", "init", "-q", "-b", "main", root], check=True)
    subprocess.run(["git", "-C", root, "add", "-A"], check=True,
                   env=dict(os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@e",
                            GIT_COMMITTER_NAME="t", GIT_COMMITTER_EMAIL="t@e"))
    if opts.get("delete_whitelist"):
        os.unlink(os.path.join(root, "publish", "whitelist.txt"))
    if opts.get("delete_marks"):
        os.unlink(marks_path)


def paths_for(repo):
    return {
        "whitelist": os.path.join(repo, "publish", "whitelist.txt"),
        "private_paths": os.path.join(repo, "publish", "private-paths.txt"),
        "publish_sh": os.path.join(repo, "scripts", "build", "publish-public.sh"),
        "mirror_lib": os.path.join(repo, "publish", "mirror-public-lib.py"),
        "marks_table": os.path.join(repo, MARKS_TABLE_REL),      # 判据⑥ 的表（缺件 ⇒ rc=2，见 require_paths）
    }


def self_test():
    base = tempfile.mkdtemp(prefix="face-sync-selftest-")
    print("自检工作区：%s" % base)
    pass_n = fail_n = 0
    for fx in FIXTURES:
        name, wl, priv, excl, drop, want = fx[0], fx[1], fx[2], fx[3], fx[4], fx[5]
        opts = fx[6] if len(fx) > 6 else {}
        root = os.path.join(base, name)
        os.makedirs(root, exist_ok=True)
        make_fixture(root, wl, priv, excl, drop, opts)
        try:
            rc, _lines = evaluate(root, paths_for(root))
        except SystemExit as exc:            # 缺件 ⇒ 2（与真跑同语义）
            rc = exc.code if isinstance(exc.code, int) else 2
        okc = (rc == want)
        if okc:
            pass_n += 1
            print("  ✅ %-24s rc=%d（期望 %d）" % (name, rc, want))
        else:
            fail_n += 1
            print("  ❌ %-24s rc=%d（期望 %d）" % (name, rc, want))
    shutil.rmtree(base, ignore_errors=True)
    print("自检：%d 过 / %d 红（%d 条用例）" % (pass_n, fail_n, pass_n + fail_n))
    return 0 if fail_n == 0 else 1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", default=REPO)
    ap.add_argument("--self-test", dest="self_test", action="store_true")
    ap.add_argument("--whitelist")
    ap.add_argument("--private-paths", dest="private_paths")
    ap.add_argument("--publish-sh", dest="publish_sh")
    ap.add_argument("--mirror-lib", dest="mirror_lib")
    args = ap.parse_args()

    if args.self_test:
        return self_test()

    if self_test() != 0:
        print("\n✗ 自检未过 ⇒ 拒绝扫真目标（rc=2）")
        return 2

    repo = os.path.abspath(args.repo)
    paths = paths_for(repo)
    for k, v in (("whitelist", args.whitelist), ("private_paths", args.private_paths),
                 ("publish_sh", args.publish_sh), ("mirror_lib", args.mirror_lib)):
        if v:
            paths[k] = os.path.abspath(v)
    for k, v in paths.items():
        if not os.path.exists(v):
            print("✗ 缺件（%s）：%s ⇒ 拒绝给结论（rc=2）" % (k, v))
            return 2

    print("仓：%s" % repo)
    print("口径：候选集排除 `publish/`（旧器对它是结构性处置，不由 EXCLUDES 驱动）\n")
    rc, lines = evaluate(repo, paths)
    for ln in lines:
        print(ln)
    if rc == 0:
        print("\n结果: 通过 —— 两器排除口径一致，私有面命中两器都已处理")
    elif rc == 1:
        print("\n结果: ✗ 发布面不同步（详见上面 ✗ 行）—— 真跑发布器会自中止/或两器漂移")
    else:
        print("\n结果: ✗ 拒绝给结论（无区分度或用法问题，rc=2）")
    return rc


if __name__ == "__main__":
    sys.exit(main())
