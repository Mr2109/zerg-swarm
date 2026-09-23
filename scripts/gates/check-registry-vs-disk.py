#!/usr/bin/env python3
# check-registry-vs-disk.py —— 门禁：**登记表 ↔ 盘上真物对拍**（空指 / 悬空 / 文案与实际不符）。
#
# 出处（逐条私有编号 · 本件判据绑私有面 ⇒ 本件**不进公开面**）：
#   · 组4 `W-21`（跨件合并）：`研-影:122`（`G-15` 条件性）· `研-禁:131`（`R-18`）
#     —— 「登记表 ↔ 盘上真物对拍（空指 / 悬空 / 文案与实际不符）—— **只认内容不认名字**」。
#   · `R-18` 的可核条件逐字：「登记表 ↔ 盘上实物：逐条给**两边的内容身份**（`sha256` / `digest`），
#     名字只作显示」·「正控：空指（`^app/` 命中 0 行）**必须报**；负控：悬空引用 ⇒ **逐条点名**而非只报数」。
#   · 组3 §一 序38 / 账 `Q-004` 同面（「文案 / 判据 vs 实际无人判」那一半的登记表侧）。
#
# 为什么要有它（本仓现成的形态，不是假想）：
#   两份「登记表」各自管着公开面的一半，而**没有一层在看它们与盘上真物对不对**：
#     ① `publish/whitelist.txt`（导出面白名单）：条目点名的东西**必须**盘上有物 ——
#        否则「本条永远选不中任何件」（`app/` 就是现成的例子：现读 `^app/` 命中 0 行 ⇒ 空指）。
#     ② `scripts/build/publish-public.sh` 的 `EXCLUDES=( … )`（B8 声明的**单一真源**）：
#        条目若**盘上无物** ⇒ 一条也排除不掉；若**盘上有物、却没有任何白名单条目会把它带上导出面**
#        ⇒ 这条排除**指向一个不存在的面**（悬空）——两者都会让读者以为「这条在管事」。
#     ③ 两处的**注释文案**是**现在时声明**：说明块逐字说「这些在脚本 `EXCLUDES` 中以模式表达」，
#        而 `EXCLUDES` 现读是**清一色字面路径**（带 glob 的条目 **0** 条）⇒ 这句话对哪些 token 成立、
#        对哪些不成立，此前**没有任何一层会报**。
#
# 三格（各一格 · 逐条点名 · 各带**盘上真物的内容身份**）：
#
#   ① **空指** —— 条目在盘上**无物**。域（写死两条）：白名单**活跃条目** + `EXCLUDES` **活跃条目**。
#   ② **悬空** —— `EXCLUDES` 条目**盘上有物**，但**没有任何白名单条目覆盖它**
#      （覆盖口径**复用** `published_path()`：目录条目按前缀、文件条目按等值）
#      ⇒ 这条排除排不掉任何东西（它排的对象根本不在导出面内）。
#   ③ **文案与实际不符** —— 登记件注释里的**现在时声明**与现读不符。域（写死两条规则）：
#        C1 **注释化条目 ↔ 盘上真物**：形如 `# <路径> ← ★ …空指…` 的注释条目，逐条核
#           「盘上**是否仍无**此物」；盘上**有** ⇒ 该注释断言的现状已过时 ⇒ 不符（点名）。
#        C2 **排除说明块点名的 token ↔ `EXCLUDES`**：说明块里**受判形状**的 token（见下）
#           逐条要求 `EXCLUDES` 里有**能覆盖它的字面条目**（同 token · 或它的**父目录**在 `EXCLUDES`）
#           ⇒ 查不到 ⇒ 不符（点名 + 列出 `EXCLUDES` 里最像的条目）。
#      ★ **未判 token 逐条列出并计数**（口径外的 token 不静默 —— 口径 `R-19`：「跳过 / 丢件必须计数」）。
#
# **边界（写死 · 防假红 · 一条不漏地写在这里）**：
#   · `publish/private-paths.txt`（私有面**黑名单**）**不进**任何一格：黑名单条目的职责是
#     「该路径若出现就拦」，条目对应的东西**本来就可能不在盘上**（旧名 / 历史路径）⇒ 判它 = 纯假红。
#   · `publish/mirror-allow.txt`（**全史**镜像豁免清单）**不进**任何一格：它的条目指向的是
#     **历史提交树**里的路径，与工作树在不在无关 ⇒ 同上。
#   · 只判**活跃条目**：注释掉的行（`#` 开头）不进① ②；它们只在 ③ C1 里作为**文案**被核。
#   · 只判**登记件自己写下的东西**：本件不判 `publish/docs/**` 的内容、不判公开产物树、不判外链。
#   · `EXCLUDES` 里**带 glob 的条目**：本件**不**据此报「形状不符」（现读 0 条 ⇒ 该分支今天不走；
#     走到时按「字面条目形状」口径判，逐条点名）。
#   · **不写一字节**：本件只读登记件与盘上真物，**不改**任何登记件（改文案 / 清空指是**另一笔**的活，
#     且 `publish/` 生效面有写死的「一字不许改」条件 ⇒ 本件只点名、不代拍）。
#
# 退码（默认 **报告档** · 与 `check-docs-orphans-links.py` / `check-dangling-refs.py` 同规）：
#   0 = 判到了（三格**逐格给数**；命中照数照点名 —— 报告档**不阻断**）
#   1 = 只有 `--strict` 档才可能：任一格 > 0（逐条点名）· `--self-test` 不过
#   2 = **不给结论**：白名单不在 / `EXCLUDES` 解析出 0 条（空转 = 假绿）/ 说明块找不到
#       （真源读不全 ⇒ 绿就是假绿）· 扫面 0 件 · 自检不过
#
# 用法：
#   python3 scripts/gates/check-registry-vs-disk.py               # 查本仓（报告档）
#   python3 scripts/gates/check-registry-vs-disk.py --strict      # 阻断档（任一格 > 0 ⇒ rc=1）
#   python3 scripts/gates/check-registry-vs-disk.py --json schema,dangling,uncovered,claims,not_judged
#   python3 scripts/gates/check-registry-vs-disk.py --repo <仓根>
#   python3 scripts/gates/check-registry-vs-disk.py --self-test   # 合成夹具成对自检（不扫真仓）
#
# 单一真源（一律复用，绝不另抄一份解析器）：
#   · 白名单解析 `load_whitelist` / `EXCLUDES` 解析 `load_excludes` / 覆盖口径 `published_path`
#     —— 全部取自 `scripts/gates/check-history-secrets.py`（镜像器解析的也是它）。
import argparse
import hashlib
import importlib.util
import json
import os
import re
import shutil
import sys
import tempfile

# 本门禁**自己**所在的仓（复用的门禁脚本一律从这里加载，不随 --repo 走）
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

WHITELIST_REL = os.path.join("publish", "whitelist.txt")
PUBLISH_SH_REL = os.path.join("scripts", "build", "publish-public.sh")

# 说明块的起始句（现读逐字；找不到这一句 ⇒ rc=2 —— 不许拿「没有说明块」当「文案全对」）
CLAIM_HEAD = "这些在脚本 EXCLUDES 中以模式表达"
# C1：注释化条目行形如  `# app/  ← ★ 空指：盘上无此目录（…）`
COMMENTED_ENTRY_RE = re.compile(r"^\s*#\s*([A-Za-z0-9_./-]+/?)\s*[←<]")
# C2：受判 token 形状（写死）：带 glob、或尾斜杠、或「路径.后缀」；其余 token 逐条列进「未判」
TOKEN_RE = re.compile(r"^[ -~]+$")
GLOB_SHAPE_RE = re.compile(r"^[A-Za-z0-9_.*/-]*[*?][A-Za-z0-9_.*/-]*$")
DIR_SHAPE_RE = re.compile(r"^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*/$")
EXT_SHAPE_RE = re.compile(r"^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*\.[A-Za-z0-9]{1,6}$")
PAREN_RE = re.compile(r"（[^）]*）")

# 目录内容身份的口径（写死）：件数 + 名字集 sha256 前 12 位；名字集封顶件数（超过 ⇒ partial）
DIR_CAP = 5000


def load_module(name, path):
    if not os.path.exists(path):
        print("✗ 缺少要复用的资产：%s（本门禁不接受缺件）" % path, file=sys.stderr)
        raise SystemExit(2)
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        print("✗ 复用资产加载不了：%s" % path, file=sys.stderr)
        raise SystemExit(2)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def load_shared():
    """复用既有解析器（单一真源纪律；本件不另抄一份白名单 / EXCLUDES 解析）。"""
    return load_module("_crs_shared", os.path.join(REPO, "scripts", "gates", "check-history-secrets.py"))


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def content_identity(root, rel):
    """盘上真物的**内容身份**（`R-18`：名字只作显示）。文件 ⇒ sha256；目录 ⇒ 件数 + 名字集 sha256。"""
    p = os.path.join(root, rel)
    if os.path.isdir(p):
        names, over = [], False
        for dirpath, dirnames, filenames in os.walk(p):
            dirnames.sort()
            for n in sorted(filenames):
                rp = os.path.relpath(os.path.join(dirpath, n), p)
                names.append(rp)
                if len(names) >= DIR_CAP:
                    over = True
                    break
            if over:
                break
        names.sort()
        h = hashlib.sha256("\n".join(names).encode("utf-8")).hexdigest()
        return {"shape": "dir", "items": len(names), "names_sha256_12": h[:12],
                "partial": over, "rule": "名字集 sha256（前 12 位 · 封顶 %d 件）" % DIR_CAP}
    if os.path.isfile(p):
        return {"shape": "file", "bytes": os.path.getsize(p), "sha256_12": sha256_file(p)[:12]}
    return {"shape": "absent"}


def read_whitelist_comments(root):
    """白名单里注释化条目（C1）与说明块 token（C2）——只读，不解析语义。"""
    path = os.path.join(root, WHITELIST_REL)
    commented, block_lines = [], []
    in_block = False
    with open(path, encoding="utf-8") as fh:
        for i, raw in enumerate(fh, 1):
            line = raw.rstrip("\n")
            m = COMMENTED_ENTRY_RE.match(line)
            if m:
                commented.append({"line": i, "entry": m.group(1), "text": line.strip()})
            if CLAIM_HEAD in line:
                in_block = True
                continue
            if in_block:
                if line.strip().startswith("#") and line.startswith("#   "):
                    block_lines.append({"line": i, "text": line.strip()})
                else:
                    in_block = False
    return commented, block_lines


def claim_tokens(block_lines):
    """说明块 token：先剥括注，再按空白/顿号切；受判形状见文件头，其余进「未判」。

    ★ 句子边界（写死）：白名单自己在块内写着「★ **本节到此为止**」—— 该标记**之后**的文字
      （现读是 `shared/` 那一句「它是白名单项，不是排除项」）**不进** token 集（取了就是假红）。
    """
    judged, not_judged = [], []
    for bl in block_lines:
        text = bl["text"].lstrip("#").strip()
        text = text.split("本节到此为止")[0]
        text = PAREN_RE.sub(" ", text)
        for tok in re.split(r"[\s、，]+", text):
            tok = tok.strip().strip("`'\"")
            if not tok or not TOKEN_RE.match(tok):
                continue
            if not re.search(r"[A-Za-z0-9]", tok):  # 纯标点（`**` / `——` / `★` 一类）不进 token 集
                continue
            if GLOB_SHAPE_RE.match(tok) or DIR_SHAPE_RE.match(tok) or EXT_SHAPE_RE.match(tok):
                judged.append({"line": bl["line"], "token": tok})
            else:
                not_judged.append({"line": bl["line"], "token": tok})
    return judged, not_judged


def covered_by_excludes(token, excludes):
    """C2 的「能找到能覆盖它的字面条目」口径：同 token · 或其父目录在 EXCLUDES。"""
    if token in excludes:
        return token
    parent = os.path.dirname(token.rstrip("/"))
    if parent and parent in excludes:
        return parent
    return None


def audit(root):
    """三格 + 未判 + 判不了。只读。"""
    shared = load_shared()
    wl_path = os.path.join(root, WHITELIST_REL)
    sh_path = os.path.join(root, PUBLISH_SH_REL)
    problems = []

    if not os.path.exists(wl_path):
        problems.append("白名单不在盘：%s" % wl_path)
    if not os.path.exists(sh_path):
        problems.append("`EXCLUDES` 单一真源不在盘：%s" % sh_path)
    if problems:
        return {"problems": problems, "dangling": [], "uncovered": [],
                "claims": [], "not_judged": [], "counts": None}

    whitelist = shared.load_whitelist(wl_path)
    excludes = shared.load_excludes(sh_path)
    if not whitelist:
        problems.append("白名单解析出 0 条活跃条目（空转 = 假绿）")
    if not excludes:
        problems.append("`EXCLUDES` 解析出 0 条活跃条目（空转 = 假绿）")
    commented, block_lines = read_whitelist_comments(root)
    if not block_lines:
        problems.append("白名单里找不到说明块起始句「%s」（真源读不全 ⇒ 不给结论）" % CLAIM_HEAD)
    if problems:
        return {"problems": problems, "dangling": [], "uncovered": [],
                "claims": [], "not_judged": [], "counts": None}

    # ① 空指 —— 白名单活跃条目 + EXCLUDES 活跃条目，逐条问「盘上有没有物」
    dangling = []
    for e in whitelist:
        if not os.path.exists(os.path.join(root, e)):
            dangling.append({"registry": "whitelist", "entry": e,
                             "why": "导出面白名单条目在盘上无物（本条永远选不中任何件）",
                             "identity": content_identity(root, e)})
    for e in excludes:
        if not os.path.exists(os.path.join(root, e)):
            dangling.append({"registry": "EXCLUDES", "entry": e,
                             "why": "排除清单条目在盘上无物（一条也排除不掉）",
                             "identity": content_identity(root, e)})

    # ② 悬空 —— EXCLUDES 条目盘上有物，却没有任何白名单条目覆盖它
    uncovered = []
    for e in excludes:
        if not os.path.exists(os.path.join(root, e)):
            continue  # 无物的那一条已进①（不重报）
        if not shared.published_path(e, whitelist):
            uncovered.append({"entry": e, "why": "盘上有物，但没有任何白名单条目覆盖它 ⇒ 这条排除排不掉任何东西",
                              "identity": content_identity(root, e)})

    # ③ 文案与实际不符 —— C1 注释化条目 ↔ 盘上真物；C2 说明块 token ↔ EXCLUDES
    claims = []
    for c in commented:
        if os.path.exists(os.path.join(root, c["entry"])):
            claims.append({"rule": "C1", "line": c["line"], "token": c["entry"],
                           "why": "注释说它盘上无此物，而现读盘上**有** ⇒ 该注释断言的现状已过时",
                           "identity": content_identity(root, c["entry"])})
    judged, not_judged = claim_tokens(block_lines)
    for j in judged:
        if covered_by_excludes(j["token"], excludes) is None:
            near = [e for e in excludes if j["token"].split("/")[0] in e][:3]
            ident = ({"shape": "glob", "note": "glob 形状（不是盘上真物，无内容身份）"}
                     if ("*" in j["token"] or "?" in j["token"]) else content_identity(root, j["token"]))
            claims.append({"rule": "C2", "line": j["line"], "token": j["token"],
                           "why": "说明块说「以模式表达」，而 `EXCLUDES` 里找不到能覆盖它的字面条目"
                                  "（同 token 或它的父目录）",
                           "nearest": near, "identity": ident})

    counts = {"dangling": len(dangling), "uncovered": len(uncovered),
              "claims": len(claims), "not_judged": len(not_judged),
              "whitelist_entries": len(whitelist), "excludes_entries": len(excludes),
              "claim_tokens_judged": len(judged)}
    return {"problems": [], "dangling": dangling, "uncovered": uncovered,
            "claims": claims, "not_judged": not_judged, "counts": counts,
            "whitelist": whitelist, "excludes": excludes}


def identity_str(ident):
    if not ident:
        return "—"
    if ident.get("shape") == "glob":
        return "glob 形状（无内容身份：它不是盘上真物）"
    if ident.get("shape") == "file":
        return "file · %d B · sha256:%.12s…" % (ident["bytes"], ident["sha256_12"])
    if ident.get("shape") == "dir":
        return "dir · %d 件 · names-sha256:%.12s…%s" % (
            ident["items"], ident["names_sha256_12"], "（封顶，partial）" if ident.get("partial") else "")
    return "不在盘"


def print_report(root, res, strict):
    c = res["counts"]
    print("登记表 ↔ 盘上真物对拍 —— 域：`publish/whitelist.txt` × `scripts/build/publish-public.sh` 的 `EXCLUDES=( … )`")
    print("仓：%s" % root)
    if c is None:
        for p in res["problems"]:
            print("  ✗ %s" % p)
        print("⇒ **不给结论**（rc=2）：真源读不全 ⇒ 绿就是假绿")
        return 2
    print("两册规模：白名单活跃 %d 条 · `EXCLUDES` 活跃 %d 条" % (c["whitelist_entries"], c["excludes_entries"]))
    print("")
    print("── ① 空指（条目在盘上无物）：%d 处" % c["dangling"])
    for d in res["dangling"]:
        print("   · [%s] %s —— %s · 内容身份：%s" % (d["registry"], d["entry"], d["why"], identity_str(d["identity"])))
    print("")
    print("── ② 悬空（`EXCLUDES` 条目有物、却不在导出面内）：%d 处" % c["uncovered"])
    for u in res["uncovered"]:
        print("   · %s —— %s · 内容身份：%s" % (u["entry"], u["why"], identity_str(u["identity"])))
    print("")
    print("── ③ 文案与实际不符：%d 处" % c["claims"])
    for cl in res["claims"]:
        near = ("· `EXCLUDES` 里最像的：%s" % " / ".join(cl["nearest"])) if cl.get("nearest") else ""
        print("   · [%s] :%d `%s` —— %s %s" % (cl["rule"], cl["line"], cl["token"], cl["why"], near))
    print("")
    print("── 未判 token（口径外 · **逐条列出**不静默）：%d 个（受判 token 形状 = 带 glob / 尾斜杠 / 路径.后缀）" % c["not_judged"])
    for nj in res["not_judged"]:
        print("   · :%d `%s`" % (nj["line"], nj["token"]))
    print("")
    hits = c["dangling"] + c["uncovered"] + c["claims"]
    if hits == 0:
        print("⇒ 三格全 0（rc=0）：登记表 ↔ 盘上真物一致")
        return 0
    if strict:
        print("⇒ **阻断档（--strict）**：三格合计 %d 处 ⇒ rc=1（逐条点名如上；报告档不阻断）" % hits)
        return 1
    print("⇒ 报告档（rc=0）：三格合计 %d 处**只报不拦** —— 零化要改登记件（`publish/whitelist.txt` 的注释 /"
          " `EXCLUDES` 的条目），属**另一笔**的活（本件只点名，不代拍）" % hits)
    return 0


def emit_json(res, fields):
    c = res["counts"] or {}
    payload = {
        "schema": "registry-vs-disk/1",
        "dangling": c.get("dangling"), "uncovered": c.get("uncovered"), "claims": c.get("claims"),
        "not_judged": c.get("not_judged"),
        "whitelist_entries": c.get("whitelist_entries"), "excludes_entries": c.get("excludes_entries"),
        "detail": {"dangling": res["dangling"], "uncovered": res["uncovered"],
                   "claims": res["claims"], "not_judged": res["not_judged"],
                   "problems": res["problems"]},
    }
    if fields:
        keys = [f.strip() for f in fields.split(",") if f.strip()]
        payload = {k: payload.get(k) for k in keys}
    print(json.dumps(payload, ensure_ascii=False))


# ── 自检（合成夹具 · 成对）────────────────────────────────────────────────────────
FIXTURE_WHITELIST = """# 合成白名单（自检夹具 · 不是真仓件）
# 说明：这些在脚本 EXCLUDES 中以模式表达 —— ★ **本节只解释 EXCLUDES 的模式**（不是「本节之后的行都排除」）
#   gateway/fleet.yaml  部署痕迹
#   sub/drop.txt  夹具排除件
sub/
gateway/
keep.txt
# gone/ ← ★ 空指：盘上无此目录（夹具）
"""

FIXTURE_PUBLISH_SH = """#!/usr/bin/env bash
# 合成 EXCLUDES（自检夹具）
EXCLUDES=(
  "gateway/fleet.yaml"
  "sub/drop.txt"
)
"""


def build_fixture(tmp):
    os.makedirs(os.path.join(tmp, "publish"))
    os.makedirs(os.path.join(tmp, "scripts", "build"))
    os.makedirs(os.path.join(tmp, "sub"))
    os.makedirs(os.path.join(tmp, "gateway"))
    open(os.path.join(tmp, "keep.txt"), "w").write("keep\n")
    open(os.path.join(tmp, "sub", "drop.txt"), "w").write("drop\n")
    open(os.path.join(tmp, "gateway", "fleet.yaml"), "w").write("fleet\n")
    open(os.path.join(tmp, "publish", "whitelist.txt"), "w").write(FIXTURE_WHITELIST)
    open(os.path.join(tmp, "scripts", "build", "publish-public.sh"), "w").write(FIXTURE_PUBLISH_SH)


def run_self_test():
    """合成夹具 · 成对：正控 2 + 负控 4（含「造一空指行 ⇒ 判红」）+ 判不了 1。"""
    checks = []

    def add_exclude(tmp, entry):
        """把一条**数组内**条目塞进合成 `EXCLUDES`（`load_excludes` 只认 `EXCLUDES=( … )` 里的行）。"""
        p = os.path.join(tmp, "scripts", "build", "publish-public.sh")
        text = open(p, encoding="utf-8").read()
        open(p, "w", encoding="utf-8").write(text.replace("\n)", '\n  "%s"\n)' % entry, 1))

    def add_block_token(tmp, token):
        """把一枚 token 塞进**说明块内**（紧跟起始句那几行 —— 块到第一个非 `#   ` 行为止）。"""
        p = os.path.join(tmp, "publish", "whitelist.txt")
        lines = open(p, encoding="utf-8").read().split("\n")
        out, done = [], False
        for ln in lines:
            out.append(ln)
            if not done and CLAIM_HEAD in ln:
                out.append("#   %s  夹具点名的模式" % token)
                done = True
        open(p, "w", encoding="utf-8").write("\n".join(out))
        return done

    def case(name, want_rc, strict, mutate=None, drop=None):
        tmp = tempfile.mkdtemp(prefix="crs-selftest-")
        try:
            build_fixture(tmp)
            if mutate == "dangling":
                add_exclude(tmp, "never-existed/")
            if mutate == "uncovered":
                open(os.path.join(tmp, "loose.txt"), "w").write("loose\n")
                add_exclude(tmp, "loose.txt")
            if mutate == "claim":
                add_block_token(tmp, "*.rej")
            if mutate == "stale_comment":
                os.makedirs(os.path.join(tmp, "gone"))
                open(os.path.join(tmp, "gone", "x.txt"), "w").write("x\n")
            if drop:
                os.remove(os.path.join(tmp, drop))
            res = audit(tmp)
            rc = 2 if res["counts"] is None else (
                0 if not (res["counts"]["dangling"] + res["counts"]["uncovered"] + res["counts"]["claims"])
                else (1 if strict else 0))
            ok = rc == want_rc
            extra = ""
            if res["counts"]:
                c = res["counts"]
                extra = "（空指 %d · 悬空 %d · 文案 %d）" % (c["dangling"], c["uncovered"], c["claims"])
            checks.append((name, want_rc, rc, ok, extra))
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    case("正控 · 干净夹具（报告档）⇒ 0", 0, False)
    case("正控 · 干净夹具（阻断档）⇒ 0", 0, True)
    case("负控② · 造一**空指行** ⇒ 阻断档判红", 1, True, mutate="dangling")
    case("负控③ · 造一**悬空行** ⇒ 阻断档判红", 1, True, mutate="uncovered")
    case("负控④ · 说明块点一个 `EXCLUDES` 里没有的 token ⇒ 阻断档判红", 1, True, mutate="claim")
    case("负控⑤ · 注释说「盘上无」而盘上有 ⇒ 阻断档判红", 1, True, mutate="stale_comment")
    case("判不了 · 白名单不在 ⇒ 2（不给结论）", 2, False, drop=os.path.join("publish", "whitelist.txt"))

    print("── 自检（合成夹具 · 成对）──")
    bad = 0
    for name, want, got, ok, extra in checks:
        print("  %s %s —— 要 %d · 得 %d %s" % ("✓" if ok else "✗", name, want, got, extra))
        bad += 0 if ok else 1
    print("自检结论: %s（断言 %d 条 · 失败 %d）" % ("全过" if bad == 0 else "**不过**", len(checks), bad))
    return 0 if bad == 0 else 1


def main():
    ap = argparse.ArgumentParser(description="登记表 ↔ 盘上真物对拍（空指 / 悬空 / 文案与实际不符）")
    ap.add_argument("--repo", default=REPO, help="仓库路径（默认本仓；自检 / 演练可指向别处）")
    ap.add_argument("--strict", action="store_true", help="阻断档：任一格 > 0 ⇒ rc=1（默认报告档 rc=0）")
    ap.add_argument("--json", dest="json_fields", default=None,
                    help="机器面：逗号分隔字段（dangling,uncovered,claims,not_judged,detail,schema…）")
    ap.add_argument("--self-test", dest="self_test", action="store_true", help="只跑合成夹具自检")
    args = ap.parse_args()

    if args.self_test:
        return run_self_test()

    root = os.path.abspath(args.repo)
    res = audit(root)
    rc = print_report(root, res, args.strict)
    if args.json_fields is not None:
        emit_json(res, args.json_fields)
    return rc


if __name__ == "__main__":
    sys.exit(main())
