#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-public-marks.py —— 「脚本公开面」标记表门禁（2026-09-20 承接项）

设计真源：`Zerg-内部文档/项目文档/v2.5.10/变更-v2.5.10.md` §「脚本公开面：标记表 + 机械门禁」
          （Mr2109 2026-09-20 拍板「好」）
表：`scripts/公开标记.tsv`（列 = 文件 · public · 理由；风格对齐 `docs/术语表.tsv`）

判据不是语言（Python 与开源无关 ✗），而是两条问句（写死在一处，本脚本据此判）：
  ① 别人复现产品**离得开它吗** —— 构建/安装/启动链 ⇒ 离开不开 ✗ ⇒ **A 类应当开源**
     （不开源 = 产品缺一块：外部人拿源码造不出能跑的东西）。
  ② 是否**暴露私有面** —— 门禁/文档审计（规则与私有文档体系绑死）· 标定（机型路径/端口/显存口径）·
     发布（Gitee 账号与仓库布局）· 换件（生产流程 + 签名身份）· 内部体检/统计 ⇒ 暴露 ⇒ **B 类不应开源**。
  为什么不能照抄业界：六家整目录公开 `scripts/` 是因为它们**从第一天就是公开项目**；我们是
  「私有主体 + 选择性公开面」⇒ 照抄必漏私有面。

本脚本判三条（三档退码，与仓内其余门脚本同口径）：
  A 表自身不合规 ⇒ **rc=2 不给结论**（缺表/空表/表头不符/字段数不符/文件列空/路径写法不是仓相对路径/
    public 非 yes|no/理由空/同一件重复登记）
  B 违规 ⇒ **rc=1 红**：
    B1 悬空登记：表里点名的件在盘上（`scripts/<域>/` 面）不存在；
    B2 漏登记：`scripts/<域>/` 面里有件**没进表**（域 = **现读派生**：表里出现过的 `scripts/<域>/…`
       第二段 ∪ 盘上 `scripts/` 下实际存在的域目录，另加「顶层」一档 · `scripts/*.sh|*.py`；
       **面外域** exportnames / sandbox-probes / plugins 不进面 —— 理由见下面 `OFF_FACE_DOMAINS`）。
       ★ 2026-09-26（本批）：域面**不再写死**。此前是 9 域常量，与门侧
         （`gate-coverage.config.json` 的 `script_globs`）静默漂移：`dev` 域在门侧却不在常量里
         ⇒ B2 对 `scripts/dev/**` 天生瞎（由 `check-marks-coverage-parity.py` 的「覆盖缺口」格报出）；
    B3 私有面：**public=yes 的件**里出现私有 IP / 绝对路径 / 账户 / 令牌模式 ⇒ 逐条点名（模式 + 行号 + 片段）。
       ★ **只判 public=yes 的面**：public=no 的件里出现同样的文本**不是本门的判据**（那是全仓其它门的活）——
         这条「区分度」由自检用例 ⑨ 钉住（同一段坏文本放进 no 面必须放行），否则本门会退化成「全扫一遍」。
       ★ **不给注释豁免**：公开面是**整件字节**，注释里的私有路径会随件一起开源。
    B3e 编码变形（★ 2026-09-26 本批新增 · 治真漏 F-16）：**public=yes 的件**里出现**编码形态**的私有面
       —— 判据**只有一条**：串**能解码**（base64 / base64url / 百分号转义）**且解码出来的明文命中
       B3 那张同一张 `PRIVATE_PATTERNS`**（**复用，不另写白名单**）⇒ 报；解不出 / 解出来不命中 ⇒ **不报**。
       为什么必须补这一格：B3 只收**明文形态** ⇒ 一行 base64（`publish/replace-rules.tsv:21` 的
       `L1VzZXJzL21zMDEv…`，解开 = `~/.config/zerg/token` = 真家目录路径）里躺着真路径却谁都拦不住 ✗。

「导出公开面时按表过滤」的三条动作（**说明**；不搬物理文件，沿用既有「白名单 + 路径映射」机制）：
  1. 导出器（`scripts/build/publish-public.sh` + `publish/whitelist.txt` + `publish/mirror-public-lib.py`）
     在 scripts/ 面**只取本表 `public=yes` 的行**；public=no 与未登记的件一律不进导出。
  2. 路径映射照旧（`map_path()`）：公开树里脚本仍落 `scripts/<域>/<名>`，表里的路径**直接可用**。
  3. 本门挂在提交闸（`scripts/gates/precommit-gates.sh` 的 scope `public` · 模式 `tri`）⇒ 表与件一旦不一致
     （悬空/漏登记/public=yes 面有私有面）**在提交前就被拦下**，不再靠「记得导出前跑一遍」。
  ★ 2026-09-21（公开线⑤ · 攒批修）：上面那条**已落地**（此前只有说明、链上一行没接）——
    `public=no` 的件 + 表自身（条数以 `scripts/公开标记.tsv` 的 `public=no` 行数 + 表自身现读为准）逐条收进 `scripts/build/publish-public.sh` 的 `EXCLUDES`
    与 `publish/mirror-public-lib.py` 的 `DROP_EXACT`（走「**不进树**」那一套；**不是** `publish/private-paths.txt`
    的整批中止语义）；表↔两器的漂移由 `scripts/gates/check-publish-face-sync.py` 的**判据⑥**
    （表说 no、链上仍会发 ⇒ 红）**秒级**兜住。表自身也立了一行（`public=no`：判据的真源不许跟着出公开面）。
    下游可核对的读数 = 本脚本 D 档：`--tree <产出树>` 的 `表 no 件进产出树`（本批开工基线 99）。

另有 **D 档（`--tree <产出树根>` · 只报告 · 2026-09-21 公开线④）**：判「**产出树 ⨯ 标记表**」——
  D1 表说 `no` 的件**却进了产出树**（逐件点名：表行号 + 表内路径 + 理由原样）；
  D2 表说 `yes` 的件**却不在产出树**（反向冲突：表说该发、链没发）；
  D3 **表自身**在产出树里（判据的真源跟着一起出公开面 ⇒ 判据与输入同时被发出去）。
  为什么单立这一档：B9/B12/B15 全在判**私有树**（白名单两器口径 / 私有面黑名单 / 表自身合规），
  **没有一只闸把「产出树 × 标记表」对起来判** ✗ ⇒ 表说 `no` 的件照旧躺在公开产物树里，却谁都看不见。
  口径见 `Zerg-内部文档/项目文档/v2.5.11/设计-公开标记接过滤链-v1.0.md` §2.2 方案 b + §5 步 1（**先只报告**）。

用法：
    python3 scripts/gates/check-public-marks.py                    # 判真目标（仓根 + scripts/公开标记.tsv）
    python3 scripts/gates/check-public-marks.py --root <树> --table <表>   # 判别的树（自检夹具用同一入口）
    python3 scripts/gates/check-public-marks.py --tree <产出树根>   # D 档：产出树 ⨯ 标记表（只报告）
    python3 scripts/gates/check-public-marks.py --max-examples 40
    python3 scripts/gates/check-public-marks.py --selftest         # 成对负控自检（含 must-fail，合成夹具、不碰真目标）
退出码：0 全绿 · 1 有违规（B1/B2/B3，或 D 档命中）· 2 表不合规/空转/产出树缺件（**不给结论** —— 不静默放绿）
        ★ D 档是**只报告档**：它自己的 rc 判据一字未改（0/1/2 三档分明）；「只报告」是**挂它的那一步**
          在步骤表里申报的档位（`tri-report` ⇒ rc=1 落 REPORT，不计失败项、不改退出码）。
"""
import argparse
import base64
import os
import re
import sys
import tempfile
import urllib.parse

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))                    # scripts/gates/ → 仓根
TABLE_REL = "scripts/公开标记.tsv"
COLS = ["文件", "public", "理由"]

# ── 域面：**从标记表现读派生**（2026-09-26 · 不再写死域清单）──────────────────────────
# 域 = `scripts/<域>/…` 的第二段；`scripts/<件>`（两段行）归到**「顶层」一档**（顶层的面 = 门侧
# 同一批 glob：`scripts/*.sh` + `scripts/*.py`）。
# 为什么改成派生：写死的 9 域常量与门侧（`gate-coverage.config.json` 的 `scan.script_globs`）会
# **静默漂移** —— 实测（`check-marks-coverage-parity.py` 的「覆盖缺口」格）：门侧有 `dev` 域
# （`scripts/dev/*.sh|*.py`）而本门的 9 域常量里没有它 ⇒ B2「漏登记」对 `scripts/dev/**` **天生瞎**
# （一件都不在面上的件，怎么漏登记都不会红）。派生后两边同口径：**表里出现过的域自动进面**，
# 门侧多域/少域由 parity 门的 R1 对拍兜住（本门不再需要人同步改常量）。
# 两个来源（都在**跑的时候**现读，任一处出现新域都进面）：
#   ① 表里出现过的域（`scripts/<域>/…` 第二段 ⇒ 域；两段行 ⇒ 顶层）；
#   ② 盘上 `scripts/` 下**实际存在的域目录**（给了 `root` 时才算）—— 保证「盘上冒出一个表里
#      一行都没有的新域」照样进面（否则面就等于「表说自己是面」= 自我循环，B2 永远追不上新域）。
# ★ **面外域**（分域前就有的工作/实验目录）**不进面**：它们本就不在门覆盖面里（口径同
#   `check-wired-scripts.py` 的 `EXCL_SUBDIRS` + `plugins`，也同 parity 门 `EXEMPT_DOMAINS` 登记
#   的那三域）⇒ 拉进来就是**改判据**（会给面外件造出一批假红）。逐条理由见下。
OFF_FACE_DOMAINS = (
    "exportnames",     # 分域前的工作目录（Go module · 非门覆盖面）：19 件全是 public=no
    "sandbox-probes",  # 分域前的实验目录（18 件探针）：表逐件登记 public=no
    "plugins",         # 插件示例件（scripts/plugins/zerg-hello）：不属「过门禁的脚本」那一批
)
TOP = "顶层"           # 顶层一档的域名（`scripts/*.sh|*.py`，无第二段）
SKIP_DIRS = {"__pycache__", ".git", ".history", ".mypy_cache", ".pytest_cache",
             "node_modules", "target", "dist", "venv", ".venv", "build"}
SKIP_SUFFIX = ("~", ".bak", ".orig", ".rej", ".swp", ".pyc", ".DS_Store")

# 私有面模式（**与既有门同一批口径**，逐条给「为什么」）——
#   来源：`scripts/gates/check-history-secrets.py` 的 BLOCK 表（全历史敏感审计，同一份判据）
#       + `scripts/gates/check-hardcoded-private-paths.py` 的 NEEDLES（私有卷/家目录）
#       + 私有标识（Gitee 镜像账号 · 机型名 · 私有项目名），对齐 `publish/replace-rules.tsv` 的脱敏对象。
PRIVATE_PATTERNS = (
    (r"/Volumes/(AHZ|BXC)\b", "私有卷绝对路径"),
    (r"/Users/[A-Za-z0-9_.-]+/", "用户家目录绝对路径"),
    (r"/home/[A-Za-z0-9_.-]+/", "Linux 家目录绝对路径"),
    (r"(?<![0-9.])192\.168\.\d{1,3}\.\d{1,3}", "内网地址 192.168.x"),
    (r"(?<![0-9.])10\.\d{1,3}\.\d{1,3}\.\d{1,3}", "内网地址 10.x"),
    (r"(?<![0-9.])172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}", "内网地址 172.16–31.x"),
    (r"\bA72109\b", "账户：Gitee 镜像账号"),
    (r"\bMr2109\b", "机群机型名 Mr2109"),
    (r"(?<![A-Za-z0-9_.-])edge-node(?![A-Za-z0-9_.-])", "机群机型名 edge-node"),
    (r"<project-a>|<project-b>", "私有项目名"),
    (r"ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}", "GitHub 令牌"),
    (r"sk-[A-Za-z0-9]{20,}", "API 密钥（sk-）"),
    (r"AKIA[0-9A-Z]{16}", "AWS Access Key"),
    (r"xox[baprs]-[A-Za-z0-9-]{10,}", "Slack 令牌"),
    (r"-----BEGIN [A-Z ]*PRIVATE KEY-----", "私钥块"),
    (r"(?i)\b(password|passwd|secret|api[_-]?key|access[_-]?token)\s*[:=]\s*[\"']?[A-Za-z0-9._\-]{12,}",
     "明文口令/密钥赋值"),
    (r"\bBearer\s+[A-Za-z0-9._\-]{20,}", "Bearer 令牌"),
)

# ── 编码变形（★ 2026-09-26 本批 · 治真漏 F-16）────────────────────────────────────────────
# 真漏（另一枚卵实测坐实）：上面那张表**只收明文形态** ⇒ `publish/replace-rules.tsv:21` 那一行 base64
#   （`L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=`，解开 = `~/.config/zerg/token` = **真家目录路径**）
#   里躺着真私有面却谁都拦不住。本批补这一格，**判据只有一条**：
#   ① 候选 = 文本里的长串（base64 / base64url / 百分号转义）—— 候选只是**被试解码**的串，不是命中；
#   ② 候选必须**解码成功**（非法字母表 / 长度补不上填充 / 解出的字节不是 utf-8 ⇒ **不报** —— 防空绿也防洪水）；
#   ③ 解码出来的**明文**必须命中上面**同一张 `PRIVATE_PATTERNS`**（**复用，不另写白名单**）⇒ 才报。
# 阈值（`{12,}` / 至少一个 `%XX`）只是「试解码」的门槛，**不收窄也不放宽命中判据**：短串解码意义弱
#   （多为标识符 / hex 片段）⇒ 试它只会添噪声，命中与否仍 100% 交给 `PRIVATE_PATTERNS`。
# 已知**不覆盖**（写明，不假装没漏）：全角（`／Users／ｆｕｚｚ０１`）与零宽空格（`~`）是
#   **归一化等价形态**、不是「解码」形态 —— 本门不收（`publish/replace-rules.tsv:22–24` 有对应规则字面量，
#   形态枚举归 `check-private-path-forms.py` 的形态表）。
ENCODED_MARK_ON = True            # **只给自检的反向控制用**（同 ⑯ 关私有面模式表那一手）：真跑路径不写它
B64_CAND = re.compile(r"[A-Za-z0-9+/]{12,}={0,2}")
B64URL_CAND = re.compile(r"[A-Za-z0-9_\-]{12,}")
PCT_ESC = re.compile(r"%[0-9A-Fa-f]{2}")


def _decode_b64(tok, url=False):
    """**严格**解码 ⇒ 明文；解不出（非法/填充补不上/字节非 utf-8）一律 `None`（= 不报）。"""
    s = tok.replace("-", "+").replace("_", "/") if url else tok
    if len(s) % 4 == 1:               # 4k+1 ⇒ 任何填充都补不上 ⇒ 非法
        return None
    if len(s) % 4:
        s += "=" * (4 - len(s) % 4)
    try:
        raw = base64.b64decode(s, validate=True)
    except Exception:
        return None
    try:
        return raw.decode("utf-8")
    except UnicodeDecodeError:
        return None


def _plain_match(text):
    """这段**明文**命中 `PRIVATE_PATTERNS` 里的哪一条 ⇒ (为什么, 命中片段)；不命中 ⇒ (None, None)。

    ★ 这是编码变形判据的**唯一命中真源** —— 解码后一律回这张表判，**不许另立一套编码专用白名单**。
    """
    for pat, why in PRIVATE_PATTERNS:
        m = re.search(pat, text)
        if m:
            return why, m.group(0)[:50]
    return None, None


class TableError(Exception):
    """表自身不合规 ⇒ rc=2（不给结论）。"""


def load_table(path):
    """读标记表 ⇒ [(行号, 文件, public, 理由)]；任何形态问题抛 TableError。"""
    if not os.path.isfile(path):
        raise TableError("标记表不存在：%s" % path)
    rows, dup = [], {}
    for i, line in enumerate(open(path, encoding="utf-8").read().split("\n"), 1):
        if i == 1:
            header = line.rstrip("\r").split("\t")
            if header != COLS:
                raise TableError("表头不符：%s（应为 %s）" % (header, "\t".join(COLS)))
            continue
        if not line.strip():
            continue
        f = line.rstrip("\r").split("\t")
        if len(f) != len(COLS):
            raise TableError("第 %d 行字段数 %d ≠ %d（列口径：%s）" % (i, len(f), len(COLS), "\t".join(COLS)))
        rel, pub, why = f[0].strip(), f[1].strip(), f[2].strip()
        if not rel:
            raise TableError("第 %d 行「文件」列为空" % i)
        if rel.startswith("/") or rel.startswith("./") or ".." in rel.split("/"):
            raise TableError("第 %d 行的路径不是仓相对路径（禁用绝对/`./`/`..`）：%s" % (i, rel))
        if pub not in ("yes", "no"):
            raise TableError("第 %d 行 public 不是 yes|no：%r" % (i, pub))
        if not why:
            raise TableError("第 %d 行「理由」为空（登记必须说清为什么 —— 写不清理由的标记就是洗白）" % i)
        if rel in dup:
            raise TableError("同一件重复登记：%s（第 %d 行与第 %d 行）" % (rel, dup[rel], i))
        dup[rel] = i
        rows.append((i, rel, pub, why))
    if not rows:
        raise TableError("表零数据行（只有表头）—— 空表不许当绿")
    return rows


def face_domains(rows, root=None):
    """**现读派生**域面 ⇒ 元组（含 `TOP` 一档）；面外域（`OFF_FACE_DOMAINS`）一律不进面。

    ① 表里出现过的域（`scripts/<域>/…` 的第二段；两段行 ⇒ `TOP`）；
    ② 给了 `root` 时，另算盘上 `scripts/` 下**实际存在的域目录**与顶层件（`.sh|.py`）。
    ⇒ 表与盘**任一处**出现新域，面就跟着走（不必改本件任何常量）。空元组 ⇒ 调用方判空转。
    """
    doms = set()
    for _i, rel, _pub, _why in rows:
        p = rel.split("/")
        if len(p) >= 2 and p[0] == "scripts":
            doms.add(p[1] if len(p) >= 3 else TOP)
    if root:
        sdir = os.path.join(root, "scripts")
        if os.path.isdir(sdir):
            for name in os.listdir(sdir):
                if name.startswith(".") or name in SKIP_DIRS or name.endswith(SKIP_SUFFIX):
                    continue
                if os.path.isdir(os.path.join(sdir, name)):
                    doms.add(name)
                elif name.endswith((".sh", ".py")):
                    doms.add(TOP)                  # 顶层件（同门侧 glob `scripts/*.sh|*.py`）
    return tuple(sorted(doms - set(OFF_FACE_DOMAINS)))


# `DOMAINS` = 在**本仓**（`ROOT`）上现读派生的域面 —— 只为一处外部读者而留：
#   `check-marks-coverage-parity.py` 按 `getattr(模块, "DOMAINS")` 取「标记面域集」算**覆盖缺口**
#   （那一格只报数、不入退码）。派生不出来（表缺件/不合规）⇒ 空元组：那一格自动跳过，
#   而本门自己跑起来会 **rc=2**（fail-closed，缺件 ≠ 绿）⇒ 不许在这里硬兜一个常量。
def _derive_domains():
    try:
        return face_domains(load_table(os.path.join(ROOT, TABLE_REL)), ROOT)
    except Exception:                      # 表缺件/不合规/读不了 ⇒ 空元组（本门自己会 rc=2 报出来）
        return ()


DOMAINS = _derive_domains()


def scan_face(root, face):
    """`face` 里的件（仓相对路径，字典序）。0 件 ⇒ 调用方判空转。

    · 域目录（`scripts/<域>/…`）**递归**取件（任意后缀；`SKIP_DIRS` / 隐藏项 / `SKIP_SUFFIX` 不取）；
    · `TOP`（顶层）只取 `scripts/*.sh|scripts/*.py` —— **逐字同门侧 glob**（顶层不许下沉一层）。
    """
    out = []
    for d in face:
        if d == TOP:
            sdir = os.path.join(root, "scripts")
            if not os.path.isdir(sdir):
                continue
            for fn in sorted(os.listdir(sdir)):
                if fn.startswith(".") or fn.endswith(SKIP_SUFFIX) or not fn.endswith((".sh", ".py")):
                    continue
                if os.path.isfile(os.path.join(sdir, fn)):
                    out.append("scripts/" + fn)
            continue
        base = os.path.join(root, "scripts", d)
        if not os.path.isdir(base):
            continue
        for dp, dns, fns in os.walk(base):
            dns[:] = sorted(x for x in dns if x not in SKIP_DIRS and not x.startswith("."))
            for fn in sorted(fns):
                if fn.startswith(".") or fn.endswith(SKIP_SUFFIX):
                    continue
                out.append(os.path.relpath(os.path.join(dp, fn), root).replace(os.sep, "/"))
    return sorted(out)


def scan_private(path):
    """逐模式扫**整件字节**（不给注释豁免）⇒ [(为什么, 行号, 命中片段)]。"""
    try:
        txt = open(path, encoding="utf-8", errors="replace").read()
    except OSError as e:
        return [("读不了文件：%s" % e, 0, "")]
    hits = []
    for pat, why in PRIVATE_PATTERNS:
        for m in re.finditer(pat, txt):
            ln = txt.count("\n", 0, m.start()) + 1
            hits.append((why, ln, m.group(0)[:60]))
    return hits


def scan_encoded(path):
    """**编码变形**扫描（★ 2026-09-26 本批 · 口径同 `scan_private`：**只对 public=yes 面**调用）。

    ⇒ [(为什么, 行号, 片段)]，三式（**都只报「解码成功且解出命中 `PRIVATE_PATTERNS`」的串**）：
      · base64（标准字母表，长串）；
      · base64url（含 `-`/`_` 的长串 —— 纯字母数字的串由标准那一遍收，**不重复报**）；
      · 百分号转义（URL 编码）：**逐行**判 —— 该行有 `%XX` ⇒ 整行 unquote ⇒ 解出命中才报；
        该行**明文**本来就已命中的**不重复报**（明文那一格已由 `scan_private` 点名）。
    解不出 / 解出来不命中 ⇒ **一条都不报**（防空绿也防洪水）。
    """
    try:
        txt = open(path, encoding="utf-8", errors="replace").read()
    except OSError as e:
        return [("读不了文件：%s" % e, 0, "")]
    hits, url_spans = [], []
    for m in B64URL_CAND.finditer(txt):
        tok = m.group(0)
        if "-" not in tok and "_" not in tok:
            continue                          # 无 url 字母表字符 ⇒ 归标准那遍（避免同一串报两次）
        url_spans.append(m.span())
        dec = _decode_b64(tok, url=True)
        why, frag = _plain_match(dec) if dec else (None, None)
        if why:
            hits.append(("编码变形 base64url（解开后命中「%s」）" % why,
                         txt.count("\n", 0, m.start()) + 1, "%s… ⇒ %s" % (tok[:40], frag)))
    for m in B64_CAND.finditer(txt):
        if any(a <= m.start() and m.end() <= b for a, b in url_spans):
            continue                          # 落在 url 串里 ⇒ 上面那遍已判过
        tok = m.group(0)
        dec = _decode_b64(tok)
        why, frag = _plain_match(dec) if dec else (None, None)
        if why:
            hits.append(("编码变形 base64（解开后命中「%s」）" % why,
                         txt.count("\n", 0, m.start()) + 1, "%s… ⇒ %s" % (tok[:40], frag)))
    if PCT_ESC.search(txt):
        for i, line in enumerate(txt.split("\n"), 1):
            if not PCT_ESC.search(line):
                continue
            dec = urllib.parse.unquote(line)
            if dec == line:                   # 一个转义都没真解开 ⇒ 不报
                continue
            why, frag = _plain_match(dec)
            if why and not _plain_match(line)[0]:
                hits.append(("编码变形 URL（百分号转义 · 解开后命中「%s」）" % why, i,
                             "%s… ⇒ %s" % (line.strip()[:40], frag)))
    return sorted(hits, key=lambda x: x[1])


def run(root=ROOT, table=None, max_examples=20, out=None):
    """判一遍 ⇒ 退出码（0 全绿 · 1 违规 · 2 表不合规/空转）。"""
    out = out or sys.stdout
    if table is None:
        table = os.path.join(root, TABLE_REL)
    try:
        rows = load_table(table)
    except TableError as e:
        out.write("✗ %s\n" % e)
        out.write("⇒ 不给结论（rc=2）：表不合规时**不判**真目标 —— 用坏表判出的绿没有意义\n")
        return 2
    face_doms = face_domains(rows, root)
    if not face_doms:
        out.write("✗ 表里没有一条 `scripts/<域>/…` 行，且盘上 `scripts/` 下无域目录 ⇒ 域面派生为空，"
                  "拒绝给结论（rc=2）\n")
        return 2
    face = scan_face(root, face_doms)
    if not face:
        out.write("✗ scripts/<域>/ 面里 0 件（域 = %s · **现读派生**）⇒ 空转，拒绝给结论（rc=2）\n"
                  % " · ".join(face_doms))
        return 2

    registered = {rel: (i, pub, why) for i, rel, pub, why in rows}
    yes = [rel for rel in registered if registered[rel][1] == "yes"]
    dangling = [rel for rel in registered if not os.path.isfile(os.path.join(root, rel))]
    missing = [rel for rel in face if rel not in registered]
    private, encoded = [], []
    for rel in sorted(yes):
        p = os.path.join(root, rel)
        for why, ln, sample in scan_private(p):
            private.append((rel, ln, why, sample))
        if ENCODED_MARK_ON:                      # ★ 编码变形（只判 public=yes 面 —— 口径同 B3）
            for why, ln, sample in scan_encoded(p):
                encoded.append((rel, ln, why, sample))

    out.write("表：%s（%d 行 · public=yes %d · public=no %d）· 面：scripts/<域>/ %d 件（域 %d 个 · %s）\n"
              % (table, len(rows), len(yes), len(rows) - len(yes), len(face), len(face_doms),
                 " · ".join(face_doms)))

    def dump(title, items):
        out.write("\n── %s：%d 处\n" % (title, len(items)))
        for x in items[:max_examples]:
            out.write("   %s\n" % (x if isinstance(x, str) else " · ".join(str(v) for v in x)))
        if len(items) > max_examples:
            out.write("   …另 %d 处\n" % (len(items) - max_examples))

    dump("B1 悬空登记（表里点名、盘上无此件）", ["%s（表第 %d 行）" % (r, registered[r][0]) for r in dangling])
    dump("B2 漏登记（盘上有、表里没有）", missing)
    dump("B3 public=yes 面里的私有面", ["%s:%d  [%s]  %s" % (r, ln, why, s) for r, ln, why, s in private])
    dump("B3e public=yes 面里的**编码变形**（解码成功且解出命中上面同一张 `PRIVATE_PATTERNS`）",
         ["%s:%d  [%s]  %s" % (r, ln, why, s) for r, ln, why, s in encoded])

    bad = len(dangling) + len(missing) + len(private) + len(encoded)
    out.write("\n合计：悬空 %d · 漏登记 %d · 私有面 %d · 编码变形 %d\n"
              % (len(dangling), len(missing), len(private), len(encoded)))
    if bad:
        out.write("⇒ 违规 %d 处（rc=1）：修表或修件 —— ① 悬空 ⇒ 改表；② 漏登记 ⇒ 补表（附理由）；"
                  "③ 私有面 ⇒ 把该件标 public=no，或去掉私有面（**不许**放宽本门的模式表来凑绿）；"
                  "④ 编码变形 ⇒ 同上，**不许**给编码形态加豁免白名单来凑绿\n" % bad)
        return 1
    out.write("⇒ 全绿（rc=0）：%d 件已登记 · public=yes 面 %d 件未发现私有面 · 编码变形 0 处\n"
              % (len(rows), len(yes)))
    return 0


# ── D 档：产出树 ⨯ 标记表（**只报告** · 2026-09-21 公开线④）──────────────────────────────
# 三问（写死在这里，不另抄一份规则）：
#   ① 表说 `no` 的件，**却出现在产出树里** ⇒ 逐件点名（表行号 · 表内路径 → 树内路径 · 理由原样）；
#   ② 表说 `yes` 的件，**却不在产出树里** ⇒ 逐件点名（反向冲突：表说该发、链没发）；
#   ③ **表自身**在产出树里 ⇒ 单独一条（判据的真源跟着出公开面 ⇒ 判据与输入同时被发出去）。
# 边界（**必须写明，否则这条判据会被读歪** —— 设计稿 §4.1「边界写法」）：
#   · 本档逐行问「表登记过的这一件在不在产出树里」；**不**新增「表没登记的就不许发」这一条 ✗
#     （那是把面外全判红，与「表不是全仓真源」自相矛盾）⇒ 未登记件只**报数**（D4 段 · 不进命中），
#     漏登记仍由提交闸上的 B2 管 ✓。
#   · **面外件**（不在 9 域内：`scripts/plugins/**` · `core/**` 等）照判并**标出「面外件」**，
#     好让读的人把这一格与面内件分开 —— 现跑基线 99 里含 `core/cmd/compressor_demo/main.go` 1 件
#     （设计稿 §六 P6 待拍：收进排除清单；本批**只报** ✗ 不动它）。
# 档位（只报告）：命中 ⇒ rc=1（挂它的那一步在步骤表里申报 `tri-report` ⇒ 落 REPORT，**不改退出码**）；
#   读不到 / 判不了（产出树缺件 · 表不合规 · 树里 `scripts/` 面 0 件空转）⇒ rc=2（**不给结论 ≠ 绿**）。
#   ★ 三条纪律照设计稿 §3.3 写进本档：**不许**放宽判据来凑绿 ✗ · 收敛到 0 之后再**单独一笔**升阻断 ✗ ·
#     `rc=2` 在发布档一律按「拦」处理（不给结论 = 不发）。
TREE_JUDGE_ON = True   # **只给自检的反向控制用**（同 ⑯ 关私有面模式表那一手）：真跑路径不写它

# 产出树扫描的排除面（见 `scan_tree_scripts` 的为什么）：**只排 VCS / 缓存目录** ——
# 别把 `SKIP_DIRS` 里的 `build` / `dist` / `target` 借过来（那会吃掉产出树的 `scripts/build/`）。
TREE_SKIP_DIRS = {".git", ".history", "__pycache__", ".mypy_cache", ".pytest_cache", "node_modules"}


def scan_tree(tree):
    """产出树里的件 ⇒ `(全部件, scripts/ 面件)`（树根相对路径 · 字典序）。

    ★ 为什么两者都要：表里的行**不都在 `scripts/` 下**（现读 2 件面外：`scripts/plugins/zerg-hello` ·
      `core/cmd/compressor_demo/main.go`）⇒ 判「这一件在不在树里」必须看**整棵产出树**；
      而「未登记」只数 `scripts/` 面（口径 = `scripts/` 面件数 − 已登记件数 · 两值一律现读为准）。
    ★ 排除面**不借** `SKIP_DIRS`：那一份是为仓内 `scripts/<域>/` 面定的，里面有 `build`/`dist`/`target`
      —— 拿来扫产出树会把 `scripts/build/` **整段吃掉**（现跑实测：8 件凭空消失 ⇒ 表里 yes 件被误判成
      「缺产出树」= 假红，同时 no 件被误判成「不在树里」= **假绿**）。产出树只排 VCS / 缓存目录。
    """
    allf, scripts = [], []
    for dp, dns, fns in os.walk(tree):
        dns[:] = sorted(x for x in dns if x not in TREE_SKIP_DIRS and not x.startswith("."))
        for fn in sorted(fns):
            if fn.startswith(".") or fn.endswith(SKIP_SUFFIX):
                continue
            rel = os.path.relpath(os.path.join(dp, fn), tree).replace(os.sep, "/")
            allf.append(rel)
            if rel.startswith("scripts/"):
                scripts.append(rel)
    return sorted(allf), sorted(scripts)


def in_domains(rel, face):
    """这一件在**面内**（`scripts/<域>/…` 且域 ∈ 派生出来的域面）吗？面外件在输出里会被标出来。

    ★ D 档这一格的面用**表侧派生**（`face_domains(rows)`，不掺产出树自己的目录）：判的是
      「这一件在不在**标记表的域**里」，产出树多出来的域不改变这个判断。
    """
    p = rel.split("/")
    return len(p) >= 3 and p[0] == "scripts" and p[1] in face


def run_tree(tree=None, table=None, max_examples=20, out=None):
    """D 档：判一遍「产出树 ⨯ 标记表」⇒ 退出码（0 全绿 · 1 命中 · 2 判不了）。**先打基线再列件**。"""
    out = out or sys.stdout
    if table is None:
        table = os.path.join(ROOT, TABLE_REL)
    if not tree or not os.path.isdir(tree):
        out.write("✗ 产出树不存在或不是目录：%s（缺件 ≠ 绿）\n" % (tree if tree else "（没给 --tree）"))
        out.write("⇒ 不给结论（rc=2）：判不了产出树时**不判** —— 这类判据在发布档一律按「拦」处理\n")
        return 2
    try:
        rows = load_table(table)
    except TableError as e:
        out.write("✗ %s\n" % e)
        out.write("⇒ 不给结论（rc=2）：表不合规时**不判**产出树 —— 用坏表判出的绿没有意义\n")
        return 2
    tree_files_all, tree_files = scan_tree(tree)
    face = face_domains(rows)               # 面内/面外那一格只看**表的域**（产出树自身的目录不参与）
    if not tree_files:
        out.write("✗ 产出树 %s 的 scripts/ 面里 0 件 ⇒ 空转，拒绝给结论（rc=2）\n" % tree)
        out.write("   （这类树判不了「表 ⨯ scripts/」：整树 %d 件，但一件脚本都没在里面 —— 不要拿它当绿）\n"
                  % len(tree_files_all))
        return 2
    tree_all = set(tree_files_all)

    no_in, no_absent, yes_in, yes_absent = [], [], [], []
    for i, rel, pub, why in rows:
        inside = rel in tree_all
        if pub == "no":
            (no_in if inside else no_absent).append((i, rel, why))
        else:
            (yes_in if inside else yes_absent).append((i, rel, why))
    self_in = TABLE_REL in tree_files
    registered = set(r[1] for r in rows)
    unregistered = sorted(p for p in tree_files if p not in registered)
    if not TREE_JUDGE_ON:          # 自检的反向控制：关掉命中判定 ⇒ **同一棵树必须退回绿**
        no_in, yes_absent, self_in = [], [], False

    out.write("产出树：%s · 表：%s（%d 行 · public=yes %d · public=no %d）\n"
              % (tree, table, len(rows), len(yes_in) + len(yes_absent), len(no_in) + len(no_absent)))
    # ★ 基线数字**打在最前**（这一档的规矩：先给三个数，再列件 —— 读的人不必翻到末尾才知道规模）
    out.write("基线（现跑 · 产出树 ⨯ 标记表）：表 no 件进产出树 = %d · 表 no 件缺产出树 = %d · 树内未登记件 = %d\n"
              % (len(no_in), len(no_absent), len(unregistered)))
    out.write("反向（现跑）：表 yes 件在产出树 = %d · 表 yes 件缺产出树 = %d\n"
              % (len(yes_in), len(yes_absent)))

    def dump(title, items):
        out.write("\n── %s：%d 处\n" % (title, len(items)))
        for x in items[:max_examples]:
            out.write("   %s\n" % x)
        if len(items) > max_examples:
            out.write("   …另 %d 处\n" % (len(items) - max_examples))

    dump("D1 表说 no、却进了产出树（只报告：本档不改退出码）",
         ["%s（表第 %d 行）→ 产出树 %s%s\n       ← 表判 public=no：%s"
          % (rel, i, rel, "" if in_domains(rel, face) else "  ★面外件（不在派生的域面内 · 设计稿 §六 P6 待拍）", why)
          for i, rel, why in no_in])
    dump("D2 表说 yes、却不在产出树（反向冲突）",
         ["%s（表第 %d 行）← 表判 public=yes：%s" % (rel, i, why) for i, rel, why in yes_absent])
    dump("D3 表自身在产出树里（判据的真源跟着出公开面）",
         ["%s → 产出树 %s（表自己不在表内 ⇒ 也是下面 D4 未登记件之一）" % (TABLE_REL, TABLE_REL)]
         if self_in else [])
    # D4 只报数、不进命中（设计稿 §4.1：**不新增**「表没登记的就不许发」这一条 ✗）
    dump("D4 树内未登记件（只报数 · **不进命中** —— 处置建议见设计稿 §六 P9：先不扩，只登记为缺口，由人拍）",
         unregistered)

    hits = len(no_in) + len(yes_absent) + (1 if self_in else 0)
    out.write("\n合计：D1 %d 处 · D2 %d 处 · D3 %d 处 · D4 未登记 %d 件（只报数）\n"
              % (len(no_in), len(yes_absent), 1 if self_in else 0, len(unregistered)))
    if hits:
        out.write("⇒ 命中 %d 处（rc=1 · **本档只报告**：挂它的那一步申报 `tri-report` ⇒ 落 REPORT，\n"
                  "   不计失败项、不改退出码；逐域收敛到 0 之后再**单独一笔**升 `tri`）\n"
                  "   命中之后只有两条路：① 改表（承认它该发）② 改处置（收进排除清单 / EXCLUDES）\n"
                  "   —— **不许**放宽本判据来凑绿 ✗（照设计稿 §3.3 纪律 3）\n" % hits)
        return 1
    out.write("⇒ 全绿（rc=0）：表说 no 的件一件都不在产出树里 · 表说 yes 的件全在 · 表自身不在产出树里\n")
    return 0


# ── 自检：成对负控（含 **must-fail**：故意喂错 ⇒ 必红；以及「不判 no 面」「关掉扫描必退回绿」两个反向控制）──
def _fixture(tmp, table_text, files):
    """搭一棵最小树：scripts/公开标记.tsv + files{相对路径: 内容} ⇒ 返回 (root, table 路径)。"""
    root = tempfile.mkdtemp(dir=tmp)
    for rel, txt in files.items():
        p = os.path.join(root, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as fh:
            fh.write(txt)
    tpath = os.path.join(root, TABLE_REL)
    os.makedirs(os.path.dirname(tpath), exist_ok=True)
    with open(tpath, "w", encoding="utf-8") as fh:
        fh.write(table_text)
    return root, tpath


def _run_quiet(root, table):
    import io
    buf = io.StringIO()
    rc = run(root=root, table=table, out=buf)
    return rc, buf.getvalue()


def _fixture_tree(tmp, table_text, tree_files):
    """搭「一个仓根（放表）+ 一棵产出树」⇒ (仓根, 表路径, 产出树根)。

    ★ 表与产出树**分开搭**：本档要判的正是「表（私有树里的真源）对产出树有没有约束力」，
    合成时把两者放在一棵树里会让那件事看不出来。
    """
    root, tpath = _fixture(tmp, table_text, {})
    tree = tempfile.mkdtemp(dir=tmp)
    for rel, txt in tree_files.items():
        p = os.path.join(tree, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as fh:
            fh.write(txt)
    return root, tpath, tree


def _run_tree_quiet(tree, table):
    import io
    buf = io.StringIO()
    rc = run_tree(tree=tree, table=table, out=buf)
    return rc, buf.getvalue()


def selftest():
    """成对负控（条数以该件自检自报为准 · 不在此写死）：每条都要「该红的红、该绿的绿」，且断言用例条数（少跑即失效）。

    ①–⑯ = A/B/C 三判（表自身 · 私有面）；⑰–㉓ = **D 档**（产出树 ⨯ 标记表 · 2026-09-21 公开线④）；
    ㉔–㉘ = **域面派生档**（新域 / dev / 顶层进面 · 面外域不进面 · 2026-09-26）；
    ㉙–㉝ = **编码变形档**（base64 真家目录 ⇒ 红 · 无害 base64 / 随机乱码 ⇒ 绿 · URL 转义 ⇒ 红 ·
    关掉这一格 ⇒ 退回绿 · 2026-09-26 治真漏 F-16）。
    """
    ok = 0
    total = 0

    def case(name, want_rc, want_text, mk):
        nonlocal ok, total
        total += 1
        rc, text = mk()
        good = (rc == want_rc) and all(s in text for s in want_text)
        ok += 1 if good else 0
        print("  %s %s（期望 rc=%s / 实测 rc=%s%s）"
              % ("✓" if good else "✗", name, want_rc, rc,
                 "" if good else "；缺字样 %s" % [s for s in want_text if s not in text]))

    GOOD = ("文件\tpublic\t理由\n"
            "scripts/build/good.sh\tyes\tA 类：构建/启动链（合成夹具）\n"
            "scripts/gates/gate.sh\tno\tB 类：门禁（合成夹具）\n")
    DIRTY_NO = ("#!/bin/sh\n# 私有面样例（public=no 面 —— 本门**不判**）\n"
                "# <worker-ip> ~/x ghp_ABCDEFGHIJKLMNOPQRSTUVWX\n")
    table_tmp = [tempfile.mkdtemp(prefix="pubmarks-selftest-")]

    def base(files_extra=None, table_text=None, table_path_override=None):
        files = {"scripts/build/good.sh": "#!/bin/sh\necho ok\n",
                 "scripts/gates/gate.sh": DIRTY_NO}
        if files_extra:
            files.update(files_extra)
        return _fixture(table_tmp[0], table_text if table_text is not None else GOOD, files)

    def mk(root, table=None):
        return lambda: _run_quiet(root, table if table else os.path.join(root, TABLE_REL))

    # ① 正控：yes 面干净 + no 面带私有面 ⇒ 绿（同时钉「不判 no 面」）
    r, t = base()
    case("① 正控：yes 面干净、no 面带私有面 ⇒ 绿（不判 no 面）", 0, ["全绿", "public=yes 1"],
         mk(r, t))

    # ② must-fail：public=yes 面塞内网 IP ⇒ 必红
    r2, t2 = base({"scripts/build/good.sh": "#!/bin/sh\ncurl http://<worker-ip>:8100/api\n"})
    case("② must-fail：public=yes 面塞内网 IP ⇒ 红且点名模式与行号", 1,
         ["私有面 1", "<worker-ip>", "内网地址 192.168.x", "good.sh:2"], mk(r2, t2))

    # ③ must-fail：public=yes 面塞私有绝对路径 ⇒ 必红
    r3, t3 = base({"scripts/build/good.sh": "#!/bin/sh\ncd <repo>\n"})
    case("③ must-fail：public=yes 面塞私有卷绝对路径 ⇒ 红", 1,
         ["私有卷绝对路径", "<volume-path>"], mk(r3, t3))

    # ④ must-fail：public=yes 面塞账户（Gitee 账号 / 机型名）⇒ 必红
    r4, t4 = base({"scripts/build/good.sh": "#!/bin/sh\ngit clone https://gitee.com/A72109/zerg-swarm\n"
                                           "ssh Mr2109 'true'\n"})
    case("④ must-fail：public=yes 面塞账户（A72109）与机型名（Mr2109）⇒ 红两条", 1,
         ["Gitee 镜像账号", "机群机型名 Mr2109"], mk(r4, t4))

    # ⑤ must-fail：public=yes 面塞令牌 ⇒ 必红
    r5, t5 = base({"scripts/build/good.sh": "#!/bin/sh\nexport T=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123\n"})
    case("⑤ must-fail：public=yes 面塞 GitHub 令牌 ⇒ 红", 1, ["GitHub 令牌", "ghp_"], mk(r5, t5))

    # ⑥ 悬空登记 ⇒ 红
    r6, t6 = base(table_text=GOOD + "scripts/gates/不存在.sh\tno\tB 类：合成夹具\n")
    case("⑥ 悬空登记（表里点名、盘上无此件）⇒ 红并点名", 1, ["悬空登记", "不存在.sh"], mk(r6, t6))

    # ⑦ 漏登记（盘上有、表里没有）⇒ 红
    r7, t7 = base({"scripts/build/newbie.sh": "#!/bin/sh\ntrue\n"})
    case("⑦ 漏登记（盘上有、表里没有）⇒ 红并点名", 1, ["漏登记", "newbie.sh"], mk(r7, t7))

    # ⑧ 表头不符 ⇒ 不给结论 rc=2
    r8, t8 = base(table_text="文件\tpublic\t理由\t备注\nscripts/build/good.sh\tyes\tA\n")
    case("⑧ 表头不符 ⇒ rc=2（不给结论，不判真目标）", 2, ["表头不符"], mk(r8, t8))

    # ⑨ public 值非法 ⇒ rc=2
    r9, t9 = base(table_text="文件\tpublic\t理由\nscripts/build/good.sh\tYES\tA\n")
    case("⑨ public 值非法（YES）⇒ rc=2", 2, ["public 不是 yes|no"], mk(r9, t9))

    # ⑩ 理由空 ⇒ rc=2
    r10, t10 = base(table_text="文件\tpublic\t理由\nscripts/build/good.sh\tyes\t\n")
    case("⑩ 理由空 ⇒ rc=2（写不清理由的标记 = 洗白）", 2, ["理由」为空"], mk(r10, t10))

    # ⑪ 重复登记 ⇒ rc=2
    r11, t11 = base(table_text=GOOD + "scripts/build/good.sh\tno\t重复\n")
    case("⑪ 同一件重复登记 ⇒ rc=2", 2, ["重复登记"], mk(r11, t11))

    # ⑫ 表缺件 ⇒ rc=2
    r12, _ = base()
    case("⑫ 表缺件 ⇒ rc=2", 2, ["标记表不存在"],
         mk(r12, os.path.join(r12, "scripts/没有这个表.tsv")))

    # ⑬ 空表（只有表头）⇒ rc=2
    r13, t13 = base(table_text="文件\tpublic\t理由\n")
    case("⑬ 空表（只有表头）⇒ rc=2（空表不许当绿）", 2, ["零数据行"], mk(r13, t13))

    # ⑭ 空转（面里 0 件）⇒ rc=2
    r14, t14 = _fixture(table_tmp[0], GOOD, {"scripts/calib/占位.txt": "x\n"})
    import shutil
    shutil.rmtree(os.path.join(r14, "scripts", "calib"))          # 面里一个件都不剩
    case("⑭ 面里 0 件 ⇒ rc=2（空转 = 假覆盖，拒绝给结论）", 2, ["空转"], mk(r14, t14))

    # ⑮ 绝对路径写法 ⇒ rc=2（路径口径：表里一律仓相对）
    r15, t15 = base(table_text="文件\tpublic\t理由\n/abs/good.sh\tyes\tA\n")
    case("⑮ 表里写绝对路径 ⇒ rc=2", 2, ["不是仓相对路径"], mk(r15, t15))

    # ⑯ 反向控制：关掉私有面扫描 ⇒ ② 那棵树必须**退回绿**（证明 ② 的断言有牙齿）
    global PRIVATE_PATTERNS
    saved = PRIVATE_PATTERNS
    PRIVATE_PATTERNS = ()
    rc16, text16 = _run_quiet(r2, t2)
    PRIVATE_PATTERNS = saved
    total += 1
    good = (rc16 == 0 and "全绿" in text16)
    ok += 1 if good else 0
    print("  %s ⑯ 反向控制：关掉私有面模式表 ⇒ ② 那棵树退回绿（rc=%s）—— 证明 ② 不是恒红也不是恒绿"
          % ("✓" if good else "✗", rc16))

    # ── ⑰–㉓：**D 档**（产出树 ⨯ 标记表 · 只报告档 · 公开线④）────────────────────────────
    # 正控 / 三条 must-fail（no 件进树 · yes 件缺树 · 表自身在树）/ 两格「不给结论」/ 一条反向控制。
    # 表与产出树**分开搭**（合成仓根放表 + 合成产出树放货）—— 这正是本档要判的那件事。
    GOOD_TREE = {"scripts/build/good.sh": "#!/bin/sh\necho ok\n"}                       # yes 件（表第 2 行）
    DIRTY_TREE = dict(GOOD_TREE, **{"scripts/gates/gate.sh": "#!/bin/sh\ntrue\n"})     # 多一件 no（表第 3 行）

    # ⑰ 正控：产出树里只有 yes 件 ⇒ 绿（同时钉「未登记件不进命中」：树里放一件没登记的）
    r17, t17, tr17 = _fixture_tree(table_tmp[0], GOOD, dict(GOOD_TREE, **{"scripts/sandbox-probes/x.sb": "x\n"}))
    case("⑰ 正控 D：产出树里只有 yes 件（另有一件未登记）⇒ 绿（未登记只报数、不进命中）", 0,
         ["全绿", "表 no 件进产出树 = 0", "树内未登记件 = 1"], lambda: _run_tree_quiet(tr17, t17))

    # ⑱ must-fail：表说 no 的件进了产出树 ⇒ 必红且点名（表行号 + 理由原样）
    r18, t18, tr18 = _fixture_tree(table_tmp[0], GOOD, DIRTY_TREE)
    case("⑱ must-fail D1：表说 no 的件进了产出树 ⇒ 红且点名（件:行 + 理由原样）", 1,
         ["表 no 件进产出树 = 1", "scripts/gates/gate.sh（表第 3 行）", "B 类：门禁（合成夹具）"],
         lambda: _run_tree_quiet(tr18, t18))

    # ⑲ must-fail：表说 yes 的件**不在**产出树 ⇒ 反向冲突，必红
    r19, t19, tr19 = _fixture_tree(table_tmp[0], GOOD, {"scripts/gates/gate.sh": "#!/bin/sh\ntrue\n"})
    case("⑲ must-fail D2：表说 yes 的件不在产出树 ⇒ 红且点名（反向冲突）", 1,
         ["表 yes 件缺产出树 = 1", "scripts/build/good.sh（表第 2 行）"], lambda: _run_tree_quiet(tr19, t19))

    # ⑳ must-fail：**表自身**在产出树里 ⇒ 单独一条 + 红
    r20, t20, tr20 = _fixture_tree(table_tmp[0], GOOD,
                                   dict(GOOD_TREE, **{"scripts/公开标记.tsv": "文件\tpublic\t理由\n"}))
    case("⑳ must-fail D3：表自身在产出树里 ⇒ 单独一条（判据的真源跟着出公开面）", 1,
         ["D3 表自身在产出树里", "scripts/公开标记.tsv → 产出树"], lambda: _run_tree_quiet(tr20, t20))

    # ㉑ must-fail：产出树缺件 ⇒ rc=2（缺件 ≠ 绿，不许被「只报告」洗掉）
    r21, t21, _ = _fixture_tree(table_tmp[0], GOOD, GOOD_TREE)
    case("㉑ must-fail D：产出树不存在 ⇒ rc=2（缺件 ≠ 绿 · 判不了 ≠ 绿）", 2,
         ["缺件 ≠ 绿", "不给结论"], lambda: _run_tree_quiet(os.path.join(r21, "没有这棵树"), t21))

    # ㉒ must-fail：产出树里 scripts/ 面 0 件 ⇒ rc=2（空转 = 假覆盖）
    r22, t22, tr22 = _fixture_tree(table_tmp[0], GOOD, {"docs/a.md": "x\n"})
    case("㉒ must-fail D：产出树 scripts/ 面 0 件 ⇒ rc=2（空转 = 假覆盖，拒绝给结论）", 2,
         ["空转"], lambda: _run_tree_quiet(tr22, t22))

    # ㉓ 反向控制：**关掉命中判定** ⇒ ⑱ 那棵必红的树必须**退回绿**（证明 ⑱ 不是恒红）
    global TREE_JUDGE_ON
    saved_tree = TREE_JUDGE_ON
    TREE_JUDGE_ON = False
    rc23, text23 = _run_tree_quiet(tr18, t18)
    TREE_JUDGE_ON = saved_tree
    total += 1
    good = (rc23 == 0 and "全绿" in text23)
    ok += 1 if good else 0
    print("  %s ㉓ 反向控制：关掉 D 档命中判定 ⇒ ⑱ 那棵树退回绿（rc=%s）—— 证明 ⑱ 不是恒红也不是恒绿"
          % ("✓" if good else "✗", rc23))

    # ── ㉔–㉘：**域面派生**（2026-09-26 · 真缺口：`dev` 域在门侧却不在旧写死的 9 域里 ⇒ B2 对
    #    `scripts/dev/**` 天生瞎）。逐格钉四件事：新域进面 · dev 进面 · 顶层进面 · 面外域**不**进面。
    #    改前的行为（旧件对这四格分别=瞎/瞎/无此档/同）由本批的「仓外 toy 现跑」对照给出（见回执），
    #    这里钉的是改后必须成立的四条。
    print("[域面派生档] 表 ∪ 盘派生域面（含「顶层」一档）· 面外域不进面")

    # ㉔ must-fail：盘上冒出一个**表里一行都没有的新域** ⇒ 面必须跟着走（否则面就是「表说自己是面」）
    r24, t24 = base(files_extra={"scripts/zzz/lone.py": "#!/bin/sh\ntrue\n"})
    case("㉔ must-fail · 域面派生：盘上新域 `scripts/zzz/`（表里一行都没有）⇒ 进面且点名", 1,
         ["漏登记", "scripts/zzz/lone.py"], mk(r24, t24))

    # ㉕ 成对：把那件补进表 ⇒ 回绿（证明 ㉔ 不是恒红）
    r25, t25 = base(files_extra={"scripts/zzz/lone.py": "#!/bin/sh\ntrue\n"},
                    table_text=GOOD + "scripts/zzz/lone.py\tno\tB 类：合成夹具（新域）\n")
    case("㉕ 成对：同一棵树把该件补进表 ⇒ 回绿（与 ㉔ 成对）", 0, ["全绿"], mk(r25, t25))

    # ㉖ must-fail：**dev 域**（本批真缺口的形态）—— 表里有 dev 行 ⇒ 面含 dev；同域里另一件未登记 ⇒ 点名
    r26, t26 = base(files_extra={"scripts/dev/isolate-ro.sh": "#!/bin/sh\ntrue\n",
                                 "scripts/dev/lone.py": "#!/bin/sh\ntrue\n"},
                    table_text=GOOD + "scripts/dev/isolate-ro.sh\tno\tB 类·开发工具（合成夹具 · 缺口 GAP-20260926-20）\n")
    case("㉖ must-fail · dev 口径纳入：表里有 dev 行 ⇒ 同域未登记件被点名（旧 9 域常量对 dev 全瞎）", 1,
         ["漏登记", "scripts/dev/lone.py"], mk(r26, t26))

    # ㉘ must-fail：**顶层**一档（门侧同样的两条 glob `scripts/*.sh|*.py`）未登记 ⇒ 点名
    r28, t28 = base(files_extra={"scripts/top.sh": "#!/bin/sh\ntrue\n"})
    case("㉘ must-fail · 顶层一档：`scripts/top.sh` 未登记 ⇒ 点名（口径同门侧 `scripts/*.sh`）", 1,
         ["漏登记", "scripts/top.sh"], mk(r28, t28))

    # ㉗ 反向控制：**面外域**里有未登记件 ⇒ 必须**放绿**（面外域拉进面 = 改判据 ⇒ 钉住「不拉进来」）
    r27, t27 = base(files_extra={"scripts/sandbox-probes/x.sb": "x\n"})
    rc27, text27 = _run_quiet(r27, t27)
    total += 1
    good = (rc27 == 0 and "全绿" in text27 and "sandbox-probes" not in text27)
    ok += 1 if good else 0
    print("  %s ㉗ 反向控制：面外域（`scripts/sandbox-probes/`）有未登记件 ⇒ 放绿且不点名（rc=%s）"
          "—— 证明派生**没有**把不可比的面拉进来" % ("✓" if good else "✗", rc27))

    # ── ㉙–㉝：**编码变形**（★ 2026-09-26 本批 · 治真漏 F-16：明文形态表对 base64/URL 编码天生瞎）──
    #    真漏形态 = `publish/replace-rules.tsv:21` 那一行（解开 = 真家目录的凭据路径）。
    #    逐格钉五件事：真家目录 base64 ⇒ **必红并点名**（㉙）· 无害 base64（解开不是私有形态）⇒ 放绿（㉚）·
    #    随机长串（解开不是 utf-8 = 乱码）⇒ 放绿（㉛）· URL 百分号转义家目录 ⇒ 必红（㉜）·
    #    关掉这一格 ⇒ ㉙ 那棵树退回绿（㉝ · 证明断言有牙齿）。**明文形态仍必报**由 ②③④⑤ 继续钉着。
    print("[编码变形档] 只收「**能解码**且**解出来命中 `PRIVATE_PATTERNS`**」的串（复用同一张表 · 不另写白名单）")

    # ㉙ must-fail：public=yes 面塞**真家目录路径的 base64**（F-16 那个真漏形态）⇒ 必红并点名（件:行 + 解出片段）
    B64_HOME = "L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4="        # base64 解开 = ~/.config/zerg/token
    r29, t29 = base({"scripts/build/good.sh": "#!/bin/sh\nTOKEN_FILE=$(echo %s | base64 -d)\n" % B64_HOME})
    case("㉙ must-fail · 编码变形：public=yes 面塞 base64 的真家目录路径 ⇒ 红且点名（件:行 + 解出片段）", 1,
         ["编码变形 1", "good.sh:2", "编码变形 base64", "用户家目录绝对路径"], mk(r29, t29))

    # ㉚ 成对：同一面塞**无害 base64**（解开 = 一句普通英文，不命中任何私有形态）⇒ 必须放绿（不许洪水）
    B64_OK = base64.b64encode(b"hello zerg swarm public fixture ok").decode()
    r30, t30 = base({"scripts/build/good.sh": "#!/bin/sh\necho %s\n" % B64_OK})
    rc30, text30 = _run_quiet(r30, t30)
    total += 1
    good = (rc30 == 0 and "全绿" in text30 and "编码变形 0" in text30)
    ok += 1 if good else 0
    print("  %s ㉚ 成对 · 编码变形：无害 base64（解开 = 普通英文、不命中形态表）⇒ 放绿（rc=%s）"
          "—— 证明这一格**不是**「看到长串就报」" % ("✓" if good else "✗", rc30))

    # ㉛ 成对：塞**随机长串**（解开不是 utf-8 = 乱码）⇒ 必须放绿（解不出 ⇒ 不报）
    B64_JUNK = base64.b64encode(os.urandom(24) + b"\xff\xfe").decode()
    r31, t31 = base({"scripts/build/good.sh": "#!/bin/sh\necho %s\n" % B64_JUNK})
    rc31, text31 = _run_quiet(r31, t31)
    total += 1
    good = (rc31 == 0 and "全绿" in text31 and "编码变形 0" in text31)
    ok += 1 if good else 0
    print("  %s ㉛ 成对 · 编码变形：随机长串（解开不是 utf-8 = 乱码）⇒ 放绿（rc=%s）"
          "—— 证明「解不出 ⇒ 不报」" % ("✓" if good else "✗", rc31))

    # ㉜ must-fail：**URL 百分号转义**的家目录（另一式）⇒ 必红并点名
    r32, t32 = base({"scripts/build/good.sh": "#!/bin/sh\nP=%2FUsers%2FMr2109%2F.config%2Fzerg%2Ftoken\n"})
    case("㉜ must-fail · 编码变形：public=yes 面塞 URL 百分号转义的真家目录路径 ⇒ 红且点名", 1,
         ["编码变形 1", "good.sh:2", "编码变形 URL", "用户家目录绝对路径"], mk(r32, t32))

    # ㉝ 反向控制：**关掉编码变形这一格** ⇒ ㉙ 那棵必红的树必须**退回绿**（证明 ㉙ 不是恒红）
    global ENCODED_MARK_ON
    saved_enc = ENCODED_MARK_ON
    ENCODED_MARK_ON = False
    rc33, text33 = _run_quiet(r29, t29)
    ENCODED_MARK_ON = saved_enc
    total += 1
    good = (rc33 == 0 and "全绿" in text33)
    ok += 1 if good else 0
    print("  %s ㉝ 反向控制：关掉编码变形格 ⇒ ㉙ 那棵树退回绿（rc=%s）—— 证明 ㉙ 不是恒红也不是恒绿"
          % ("✓" if good else "✗", rc33))

    print("  自证用例：%d 条 · 通过 %d 条（断言用例条数 %d，少跑即失效）"
          % (total, ok, 33))
    assert total == 33, "自证用例数必须为 33，实际 %d" % total
    return 0 if ok == total else 1


# ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
#    口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
#    退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
JSON_FIELDS = []          # 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）


def refuse_json_without_machine_face(argv=None):
    """给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后返 `rc=2`；没给 ⇒ `None`（原路照走）。"""
    argv = list(sys.argv[1:] if argv is None else argv)
    hit = [x for x in argv if x == "--json" or x.startswith("--json=")]
    if not hit:
        return None
    print("⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
                     % " · ".join("`%s`" % h for h in hit), file=sys.stderr)
    print("   字段表：%s" % (" · ".join("`%s`" % f for f in JSON_FIELDS) if JSON_FIELDS
                                          else "无（本门只出人读面）"), file=sys.stderr)
    print("⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形", file=sys.stderr)
    return 2


def main():
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=ROOT, help="树根（默认 = 仓根）")
    ap.add_argument("--table", default=None, help="标记表路径（默认 = <root>/scripts/公开标记.tsv）")
    ap.add_argument("--tree", default=None,
                    help="**D 档**：产出树根 ⇒ 判「产出树 ⨯ 标记表」（只报告 · 命中 rc=1）")
    ap.add_argument("--max-examples", type=int, default=20)
    # ★ `Q-176` 第六批：拼法统一 —— **准拼法 `--self-test`**，旧拼法 `--selftest` 作别名（两个都认；新门只写准的）
    ap.add_argument("--self-test", "--selftest", dest="selftest", action="store_true",
                    help="只跑成对负控自检（合成夹具，不碰真目标）")
    args = ap.parse_args()
    if args.selftest:
        print("── check-public-marks.py 自证（含 must-fail 负控 · 合成夹具）")
        return selftest()
    if args.tree:
        print("── check-public-marks.py D 档（产出树 ⨯ 标记表 · **只报告**：命中 rc=1 不改退出码）")
        return run_tree(tree=args.tree, table=args.table, max_examples=args.max_examples)
    return run(root=args.root, table=args.table, max_examples=args.max_examples)


if __name__ == "__main__":
    sys.exit(main())
