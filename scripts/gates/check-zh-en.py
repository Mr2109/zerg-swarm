#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-zh-en.py —— 中英文与术语检查（code-quality §8 的可执行版）

设计要点（2026-09-16 三轮假红的教训）：
  ① 检查对象按文件类型分流：
     - 「全角标点」只扫**没有编译器兜底**的文件（.sh/.yaml/.json/.toml/.py/.plist）
       —— .go 里全角标点编译即报错，扫它只会制造假红（第一轮 5,725 处）
     - 「术语/旧角色词」对 .go 也要扫（与编译器无关，只有人眼会滑过）
  ② 剥注释/字符串必须覆盖**跨行结构**：
     - Python 三引号文档字符串（第二轮 312 处假红）
     - shell heredoc 正文、$( … ) 命令替换内部（第三轮 2 处假红）
  ③ 白名单：防泄漏词表（发布拦截脚本）与规则文本本身必须能写出禁用词
  ④ 零输入即报错 —— 绝不静默空转（假绿）
  ⑤ 自带自证，且**断言用例条数** —— 用例被静默截断时不得打印"通过"

用法：python3 scripts/gates/check-zh-en.py [--selftest]
退出码：0 = 无命中；1 = 有命中；2 = 环境/输入异常（不静默）
"""
import os
import re
import sys

ROOTS = ["core", "agent", "gateway", "scripts", "deploy", "docs/skills"]
SKIP_DIRS = ("node_modules", "target", "vendor", ".git", "dist", ".venv")
FULLW_EXT = (".sh", ".yaml", ".yml", ".json", ".toml", ".py", ".plist", ".conf")
TERM_EXT = FULLW_EXT + (".go",)
FULLW = "，。：；（）！？“”‘’　"

FORBID = {
    "虫巢": "术语铁律：只有「虫茧」",
    "集装箱": "术语铁律：只有「虫茧」",
    "Mr2109": "私有称谓：署名只用 GitHub 用户名 Mr2109",
    "Mr2109": "私有称谓：署名只用 GitHub 用户名 Mr2109",
    "Mr2109": "私有称谓：署名只用 GitHub 用户名 Mr2109",
}
WHITELIST = {
    # ③（本件自己的口径逐字）：**防泄漏词表（发布拦截脚本）与规则文本本身必须能写出禁用词**。
    "scripts/build/publish-preflight.sh",
    "scripts/build/publish-public.sh",
    "scripts/build/test-publish-parity.sh",
    "docs/skills/code-quality.md",
    "scripts/gates/check-zh-en.py",
    # 2026-09-23（`T-03` 门㉝ · 公开面写作红线自检件）：它的**内置甲档词表**逐条写着哪些词属私有面
    #   （`AHZ`/`Zerg-内部文档`/缺口号/私有编写角色称谓/私有称谓 …）⇒ 正落在上面那一条口径里（**规则文本本身**）。
    #   加之前：本件在禁用词面 6 处命中（词表行 1 + 理由引用 3 + 合成夹具 2）⇒ 尾巴软检查由 6 涨到 12；
    #   加之后：回到 6（**不是**放宽判据：本件仍受「全角标点(代码区)」那一半管辖，仍是 0 命中）。
    "scripts/gates/check-public-face-wording.py",
    # 2026-09-24（**波15 · 序136** · 门㊷ `check-private-path-forms.py`）：同一条口径（③「**规则文本本身**」）——
    #   它的**形态表**必须逐字写出私有称谓（`Mr2109`）等禁用词，否则**认领正则匹配不上** `publish/replace-rules.tsv`
    #   的规则原文，判据就退化成空转；⇒ 与上一件同规（**不是**放宽判据：本件仍受「全角标点(代码区)」那一半管辖）。
    #   加之前：本件在禁用词面 4 处命中（认领正则 1 + 出处探针 1 + 门禁侧探针 1 + 夹具规则行 1）⇒ 尾巴软检查由 7 涨到 11；
    #   加之后：回到 7（另 7 处为 `check-glossary.py` / `gen-glossary-exports.py` 的**存量**，未动）。
    "scripts/gates/check-private-path-forms.py",
}
ROLE_PAT = re.compile(r"""(host\s*:\s*['"]?local\b|Machine\s*:\s*['"]local['"]|MachineSnapshot\(['"]local['"]\))""")
ROLE_EXEMPT_HINT = ("_test.go", "selfupdate", "toolchain")

# 已判定的假红（精确到「件 + 内容子串」+ 理由）。仍会在报告中单独列出——豁免不等于隐藏。
#
# ★ 2026-09-26（`Q-176` 第五批后的教训 · 行号登记的普查推荐 (B)）：本表**原来按行号登记**（键 `(件, 行号)`）。
#   `check-glossary.py` 上方插 26 行 ⇒ 旧键 379/387/389 **整体失指**（有的指空、有的指到**别的行**）
#   ⇒ 一次假红 7 处，而登记表**自己一声不吭** ✗。「行号锚点本身就是病」（`W-45` / `R-17`）⇒ 键改成
#   **(件, 该行逐字片段)**：登记是否有效由**内容**决定、与行号无关（件内挪行不再失指；子串必须能在
#   它那件里 `grep -F` **恰命 1 行**）。
#   **不是放宽判据**：豁免范围、理由、逐条打印**一字未变**，只换「定位方式」（行号 → 子串）；
#   子串命中 0 行 / 多于 1 行 ⇒ `registry_stale()` 判「**登记过期**」并逐字点名 ✗
#   （旧式样里登记失效是**静默**的——这正是本改动要治的病）。
#   式样同仓内三条生产先例（**零新机制**）：`check-dangling-refs.py` 的 `ref`（件 + 引文逐处登记）·
#   `check-keep-marks.py` 的 `anchor`（`M5`：内容锚恰好命中 1 次）· `check-anchor-uniqueness.py`（内容锚唯一性）。
#   条目里的 `# 导航 :NNN` 只作**导航**用（判据不吃行号 ⇒ 件挪行时它过期不影响判定）。
#
# ★ 覆盖范围（`Q-120` · 2026-09-26 修）：这一张表是**逐行**豁免口 —— 命中登记 ⇒ 该行的
#   **两半判定（禁用词 / 全角标点）** 一律改记为「已登记假红(豁免)」，不计入「合计命中」，
#   但照样逐条打印。修前只有「全角标点」那一半会读它 ⇒ 禁用词那一半的已知假红（门禁 /
#   生成器**自己的词表字面量**）永远算命中，尾巴软检查长期挂着 6 处噪声。
#   负控：登记表里没有这一行 ⇒ 两半照旧判红（`--selftest` 里 `[豁免口]` 那两条）。
KNOWN_FP = {
    # 2026-09-26（`Q-120`）：以下 4 行全是**规则文本 / 词表字面量本身**，不是被检对象 —
    #   与 WHITELIST 同一条口径（本件文件头 ③「规则文本本身必须能写出禁用词」），只是
    #   它们落在**别的件**里，而 WHITELIST 是**整件**级的 ⇒ 用逐行豁免口精确到行。
    #   `check-glossary.py` 一行同时含 `虫巢` 与 `集装箱`（那 6 处里占 2）⇒ 一行一条豁免。
    # ★ 2026-09-26（`Q-176` 第五批 · 44 件同秒批）：`check-glossary.py` 在**上面那三行之上**插了 26 行
    #   （`--self-test` 三档自检）⇒ 三行**行号整体下移 26**（379→405 · 387→413 · 389→415）。
    #   ★ 本笔（2026-09-26 · 普查推荐 (B)）：键已从**行号**换成**逐字片段** ⇒ 上面那次「挪行」从今往后
    #   不再需要改登记表（内容逐字未动 ⇒ 键照样命中）；豁免集合**一字不变**。负控：把某键的子串改成
    #   不存在的串 ⇒ 该键判「登记过期」并逐字点名（不再静默）。
    #   同批又在文件尾部新写 3 行合成夹具 / 用例标题（511 · 519 · 532），同属「规则文本本身」⇒ 按同一式样补登。
    ("scripts/gates/check-glossary.py", '"虫巢|集装箱", "pod"'):  # 导航 :405（判据不吃行号）
        "术语词表字面量（`COLS` 里逐字写着 `虫巢|集装箱` 用作**被检词**，与 `docs/术语表.forbid.txt` 同源）",
    ("scripts/gates/check-glossary.py", 'scan_page("docs/zh/a.md", "这里叫虫巢。", gloss, "zh")[1], 1)'):  # 导航 :413（判据不吃行号）
        "自证夹具：断言「T2 中文页禁写 虫巢」的那条**用例正文**（`\"这里叫虫巢。\"`）",
    ("scripts/gates/check-glossary.py", '旧称虫巢。<!-- glossary-allow -->'):  # 导航 :415（判据不吃行号）
        "自证夹具：断言「T2 单条豁免生效」的**用例正文**（`\"旧称虫巢。<!-- glossary-allow -->\"`）",
    ("scripts/gates/check-glossary.py", 'f.write("虫巢\\t\\t\\t\\t虫巢\\t\\tMr2109\\t英文待拍\\n")'):  # 导航 :511（判据不吃行号）
        "`Q-176` 第五批合成夹具：喂 `load_glossary` 的**词表行**（`虫巢` 当 `forbid_zh` 列 ⇒ 验 T2 抓得到）",
    ("scripts/gates/check-glossary.py", 'f.write("旧称：这里叫虫巢。\\n")'):  # 导航 :519（判据不吃行号）
        "`Q-176` 第五批合成夹具：中文页的**用例正文**（`\"旧称：这里叫虫巢。\"` ⇒ 验 T2 负控 rc=1）",
    ("scripts/gates/check-glossary.py", '中文页出现 `forbid_zh` 词（虫巢）⇒ T2 非阻塞 rc=1'):  # 导航 :532（判据不吃行号）
        "`Q-176` 第五批自证**用例标题串**（`chk(\"档② 负控：…（虫巢）…\")`；与 413 同种：标题里点名被检词）",
    ("scripts/docs/gen-glossary-exports.py", '"虫巢", "pod", "Mr2109", ""]))])[1].split("\\t")[0], "pod")'):  # 导航 :274（判据不吃行号）
        "生成器自证夹具：喂 `gen_forbid(...)` 的合成词表行（验它把 `pod` 翻出来）",
    ("scripts/docs/gen-glossary-exports.py", '"只有「虫茧」" in gen_forbid([dict(zip(COLS, ["虫茧", "cocoon", "", "", "虫巢", "", "Mr2109", ""]))])[0], True)'):  # 导航 :276（判据不吃行号）
        "生成器自证夹具：喂 `gen_forbid(...)` 的合成词表行（验理由串「只有「虫茧」」在）",
}

RE_TRIPLE = re.compile(r"""(\"\"\"|''')[\s\S]*?\1""")
RE_CMDSUB = re.compile(r"""\$\((?:[^()]|\([^()]*\))*\)""")
RE_HEREDOC_START = re.compile(r"""<<-?\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?""")


def strip_blocks(text):
    """剥掉跨行三引号块（等量换行替换，保住行号）。"""
    return RE_TRIPLE.sub(lambda m: "\n" * m.group(0).count("\n"), text)


def _hd_quote_cols(l, st):
    """S 变体（GAP-20260927-17）：行内字符级状态机 —— 返回「位于引号内」的列集合，并就地推进 st。

    规则：`'…'` 原样（内不认 `$(`、不认转义）；`"…"` 认 `\\` 转义、认 `$(`；
    遇 `$(` **挂起**外层引号态（压栈、内层从「无引号」重开），遇配对 `)` 弹栈复原；
    行内注释（`#` 在行首或前面是空白）之后整行不参与状态 —— 注释里的撇号不再开幻影引号 ✗✓。
    """
    q, stack, n, i, cols = st.get("q"), st.get("stack", []), len(l), 0, set()
    while i < n:
        c = l[i]
        if q is None and not stack and c == "#" and (i == 0 or l[i - 1] in " \t"):
            break
        if q == '"' and c == "\\":
            i += 2
            continue
        if q is None and c == ")" and stack:
            q = stack.pop()
        elif (q is None or q == '"') and c == "$" and i + 1 < n and l[i + 1] == "(":
            stack.append(q)          # `$(` 在双引号内**也要认**（挂起外层引号态），单引号内不认
            q = None
            i += 2
            continue
        elif q is None:
            if c in "'\"":
                q = c
        elif c == q:
            q = None
        if q is not None:
            cols.add(i)
        i += 1
    st["q"], st["stack"] = q, stack
    return cols


MASK_UNCLOSED = []   # [(件, [未闭合起始行…])]；scan() 前清空（GAP-20260927-128/-129）
MASK_MLQ_UNCLOSED = []  # [(件, [跨行引号未收尾起始行…])]；scan() 前清空（GAP-20260926-128 余项）
MASK_PYQ_UNCLOSED = []  # [(件, [三引号层配不齐而被掩行…])]；scan() 前清空（GAP-20260928-74 余项）


def mask_heredocs_chk(text):
    """同 `mask_heredocs`，但**回报未闭合段**：返回 `(遮蔽后正文, 起始行号列表)`。

    GAP-20260927-129：起始标记出现在**引号内**（如 `printf '…<<PYEOF…'`）时，收尾条件
    （只含 TAG 的那一行）永不成立 ⇒ 其后直到文件末全被遮蔽 = 门整段失明。旧行为静默通过。
    """
    out, tag = [], None
    starts = []
    hst = {"q": None, "stack": []}
    for lineno, l in enumerate(text.split("\n"), 1):
        qcols = _hd_quote_cols(l, hst)          # 每行都推进状态（跨行持态）
        if tag is None:
            out.append(l)
            m = RE_HEREDOC_START.search(l)
            if m and m.start() not in qcols:    # GAP-20260927-17：引号内的 <<TAG 不算起始
                tag = m.group(1)
                starts.append((lineno, tag))
        else:
            if l.strip() == tag:
                out.append(l)
                tag = None
                starts.pop()
            else:
                out.append("")
    return "\n".join(out), [s[0] for s in starts]


def mask_heredocs(text):
    """屏蔽 heredoc 段：`<<[-]?['\"]?TAG['\"]?` 起始行之后、直到只含 TAG 的那一行。

    正文属字符串（非代码）。用空行替换以保住行号。
    行式实现 —— 正则版实测匹配不上真实文件（2026-09-16）。
    """
    return mask_heredocs_chk(text)[0]


def _mask_heredocs_old(text):
    out, tag = [], None
    for l in text.split("\n"):
        if tag is None:
            out.append(l)
            m = RE_HEREDOC_START.search(l)
            tag = m.group(1) if m else None
        else:
            if l.strip() == tag:
                out.append(l)
                tag = None
            else:
                out.append("")
    return "\n".join(out)


def _open_quote(line):
    """返回该行**留下的未闭合引号字符**（None = 行内成对）。转义与单双引号交替的简化处理。"""
    q, i = None, 0
    while i < len(line):
        c = line[i]
        if c == "\\":
            i += 2
            continue
        if q:
            if c == q:
                q = None
        elif c in "\"'":
            q = c
        i += 1
    return q


def mask_multiline_quotes_chk(text):
    """同 `mask_multiline_quotes`，但**回报未收尾段**：返回 `(遮蔽后正文, 起始行号列表)`。

    `GAP-20260926-128` 余项：本层与 `mask_heredocs` 同病 —— 收尾条件是 `l.find(quote)`
    （同一引号字符再次出现）。引号起于**命令头之后**而本件内无收尾时，后半件全被当字符串
    掩掉 = 门在这些行上失明，而旧行为**静默**通过（可 rc=0 假绿）。
    与 `mask_heredocs_chk` 同式样（零新机制）：遮蔽输出**一字不变**，只多一条自报口。
    """
    out, quote, starts = [], None, []
    for lineno, l in enumerate(text.split("\n"), 1):
        if quote:
            idx = l.find(quote)
            if idx < 0:
                out.append("")
                continue
            out.append(" " * (idx + 1) + l[idx + 1:])
            quote = None
            starts.pop()
            continue
        out.append(l)
        # 注释行**永不**开引号：中文注释里常有撇号（如「批 2'.6」），否则会把后续注释行
        # 也当成跨行字符串吞掉，连 "#" 一起屏蔽 ⇒ 注释正文被误判为代码（实测 build-wall.sh:5）。
        quote = None if l.lstrip().startswith("#") else _open_quote(l)
        if quote:
            starts.append((lineno, quote))
    return "\n".join(out), [s[0] for s in starts]


def mask_multiline_quotes(text):
    """屏蔽 shell **跨行引号字符串**的内部（如 git commit -m "…\n…" 的多行提交信息体）。

    逐行状态机看不见跨行引号 ⇒ 把正文当代码（实测 publish-public.sh:420 假红）。
    只有"跨行未闭合"才屏蔽中间行（空行替换保住行号）；行内成对的仍交给 strip_noncode。
    """
    return mask_multiline_quotes_chk(text)[0]


MASK_PYQ_JUDGE = "python-tokenize（Python 自己的词法器）"   # 本层「判不了」的对拍源，非启发式


def _py_quote_blind_lines(text, masked):
    """`.py` 三引号层的**鉴别力探针**：列出「被正则掩成空行、而词法器说这行不是字符串」的行号。

    判据不吃启发式 —— 对拍源是 **Python 自己的词法器**（`tokenize`）：
      字符级真值取 `STRING` 记号的行区间；被 `RE_TRIPLE` 掩成空行、落在字符串区间**之外**、
      且**本来就是命中**（代码区全角标点 / 禁用词）的行 ⇒ 正则把**真命中**掩掉了（门在这行失明）。
      掩掉的段里本来无命中 ⇒ 无损失，不报（本层不靠启发式猜，不扩判据面）。
    件本身词法读不通（未收尾多行字符串、缩进错等）⇒ 本层同样**无结论**，整片被掩行如实回报。
    """
    raw_l, msk_l = text.split("\n"), masked.split("\n")
    blanked = [i for i, (a, b) in enumerate(zip(raw_l, msk_l), 1) if a.strip() and not b.strip()]
    if not blanked:
        return []
    try:
        import io
        import tokenize
        toks = list(tokenize.generate_tokens(io.StringIO(text).readline))
    except Exception:
        return blanked                      # 读不通 ⇒ 判不了（绝不当绿：同 accept-no-probe.sh 的口径）
    strlines = set()
    for tk in toks:
        if tk.type == tokenize.STRING and tk.start and tk.end:
            strlines.update(range(tk.start[0], tk.end[0] + 1))
    # 只报「掩掉的这一行本来就是命中」的那些行 —— 本层判不了的**充分理由**（不是判据扩面）：
    # 被掩的真代码里本来有命中 ⇒ 「掩掉后零命中」不能当绿。掩掉的段里本来没命中 ⇒ 无损失，不报。
    out = []
    for l in blanked:
        if l in strlines:
            continue
        src = raw_l[l - 1]
        code = strip_noncode(src, ".py")
        if any(ch in code for ch in FULLW) or any(w in src for w in FORBID):
            out.append(l)
    return out


def strip_blocks_chk(text):
    """同 `strip_blocks`，但**回报 `.py` 三引号层的盲区行**：返回 `(遮蔽后正文, 判不了行号列表)`。

    `GAP-20260928-74`（余项）：本层此前**没有自报口** —— `RE_TRIPLE` 是「就近配对」式正则，
    件里出现**配不齐**的三引号（尤其 \"\"\" 落在注释或普通字符串里）时，成对的两颗定界符会把
    **两者之间的真代码整段掩掉** ⇒ 门在这段失明，而旧行为照样 rc=0（**假绿**）。
    与 `mask_heredocs_chk` / `mask_multiline_quotes_chk` **同式样**：遮蔽输出**一字不变**，
    只多一条自报口（判不了 ⇒ main() 给 rc=2，不当绿）。
    """
    masked = strip_blocks(text)
    return masked, _py_quote_blind_lines(text, masked)


def strip_noncode(line, ext):
    """剥掉单行注释与单行字符串，返回「代码区」子串。

    先屏蔽 $( … )（命令替换内部等价于字符串；shell 嵌套引号会打乱引号配平）。
    """
    if "$(" in line:
        line = RE_CMDSUB.sub(lambda m: " " * len(m.group(0)), line)
    out, i, n, quote = [], 0, len(line), None
    hash_comment = ext in (".sh", ".py", ".yaml", ".yml", ".toml", ".conf", ".plist")
    while i < n:
        c = line[i]
        if quote:
            if c == "\\":
                i += 2
                continue
            if c == quote:
                quote = None
            i += 1
            continue
        if c in "\"'":
            quote = c
            i += 1
            continue
        if line.startswith("//", i) or (hash_comment and c == "#"):
            break
        out.append(c)
        i += 1
    return "".join(out)


def _body_of(path, ext):
    """与 `scan()` **同一条遮蔽管线**：子串唯一性只在「扫描域」里判（被掩掉的段不算命中）。"""
    raw = open(path, encoding="utf-8", errors="replace").read()
    if ext == ".py":
        return strip_blocks(raw)
    if ext in (".sh", ".plist", ".conf"):
        return mask_multiline_quotes(mask_heredocs(raw))
    return raw


def fp_hit(p, line):
    """逐行豁免口：键 = `(件, 内容子串)` ⇒ 命中 ⇔ 登记的那条子串在**本行**逐字出现。返回理由（无则 None）。"""
    for (path, sub), why in KNOWN_FP.items():
        if path == p and sub and sub in line:
            return why
    return None


def registry_stale():
    """**登记表体检**（唯一性校验 · 本改动的牙）：每条 `(件, 子串)` 在**它那件**里必须恰好命中 1 行。

    0 行 ⇒ 子串已不在件里（内容被改 / 落进被掩掉的段）；>1 行 ⇒ 子串不唯一（指不到一处）。
    两个方向都判「**登记过期**」并逐字点名 —— 旧「行号登记」的病正在这里：行号失指时登记
    **静默**失效（豁免悄悄掉，或悄悄指到**另一行**）。返回 (过期表, 读不到的件表)；
    后者是**输入异常**（rc=2），不当作「登记过期」。
    """
    stale, missing = [], []
    for (path, sub) in KNOWN_FP:
        if not sub:
            stale.append((path, "", "登记子串为空", []))
            continue
        full = os.path.join(os.getcwd(), path)
        if not os.path.isfile(full):
            missing.append(path)
            continue
        body = _body_of(full, os.path.splitext(path)[1].lower())
        hit = [i for i, ln in enumerate(body.splitlines(), 1) if sub in ln]
        if not hit:
            stale.append((path, sub, "命中 0 行（该内容已不在件里）", hit))
        elif len(hit) > 1:
            stale.append((path, sub, "命中 %d 行（子串不唯一 ⇒ 指不到一处）" % len(hit), hit))
    return stale, missing


def _fp_row(hits, p, i, line, why):
    """登记行 → 「已登记假红(豁免)」桶（一行只记一次）。

    禁用词 / 全角标点两半**共用**同一张 `KNOWN_FP` ⇒ 同一行可能两半都命中：
    这里按「渲染后的整条串」去重，保证报告里一行只出现一次（豁免 ≠ 隐藏，但 ≠ 重复刷屏）。
    """
    ent = "%s:%d  [%s]  %s" % (p, i, why, line.strip()[:60])
    if ent not in hits["已登记假红(豁免)"]:
        hits["已登记假红(豁免)"].append(ent)


def scan():
    files = 0
    hits = {"禁用词": [], "全角标点(代码区)": [], "旧角色词": [], "已登记假红(豁免)": []}
    for root in ROOTS:
        if not os.path.isdir(root):
            continue
        for dp, dn, fn in os.walk(root):
            dn[:] = [d for d in dn if d not in SKIP_DIRS]
            for f in fn:
                p = os.path.join(dp, f)
                ext = os.path.splitext(f)[1].lower()
                if ext not in TERM_EXT and not f.endswith(".env"):
                    continue
                files += 1
                try:
                    raw = open(p, encoding="utf-8", errors="replace").read()
                except Exception as e:
                    print("  ⚠ 读不了 %s：%s" % (p, e), file=sys.stderr)
                    continue
                if ext == ".py":
                    body, _pyq = strip_blocks_chk(raw)
                    if _pyq:
                        # `GAP-20260928-74`（余项）：三引号层掩掉了真代码 ⇒ 本层判不了，由 main() 给 rc=2
                        MASK_PYQ_UNCLOSED.append((p, _pyq))
                elif ext in (".sh", ".plist", ".conf"):
                    masked, _unclosed = mask_heredocs_chk(raw)
                    if _unclosed:
                        # GAP-20260927-128/-129：遮蔽未闭合 = 门在这段失明 ⇒ 登记，由 main() 判 rc=2
                        MASK_UNCLOSED.append((p, _unclosed))
                    body, _mlq = mask_multiline_quotes_chk(masked)
                    if _mlq:
                        # `GAP-20260926-128` 余项：跨行引号未收尾 ⇒ 本层失明，由 main() 判「判不了」
                        MASK_MLQ_UNCLOSED.append((p, _mlq))
                else:
                    body = raw
                for i, l in enumerate(body.split("\n"), 1):
                    # 逐行豁免口（`KNOWN_FP`）：**键 = `(件, 内容子串)`**，登记的那条子串在本行逐字
                    # 出现即命中 ⇒ 该行的**两半判定**（禁用词 / 全角标点）一律改记「已登记假红(豁免)」
                    # ——照样逐条打印、但**不计**「合计命中」。**不吃行号**（件挪行不失指）；
                    # 子串在件里命中 ≠1 ⇒ `registry_stale()` 判「登记过期」并点名（正/负控见 `selftest`）。
                    fp = fp_hit(p, l)
                    if p not in WHITELIST:
                        for w, why in FORBID.items():
                            if w in l:
                                if fp:
                                    _fp_row(hits, p, i, l, fp)
                                else:
                                    hits["禁用词"].append("%s:%d  [%s]  %s" % (p, i, why, l.strip()[:70]))
                    code = strip_noncode(l, ext)
                    if ext in FULLW_EXT:
                        if fp:
                            _fp_row(hits, p, i, l, fp)
                        else:
                            for ch in FULLW:
                                if ch in code:
                                    hits["全角标点(代码区)"].append("%s:%d  全角「%s」  %s" % (p, i, ch, l.strip()[:70]))
                                    break
                    m = ROLE_PAT.search(code)
                    if m and not any(h in p for h in ROLE_EXEMPT_HINT):
                        hits["旧角色词"].append("%s:%d  %s  %s" % (p, i, m.group(1), l.strip()[:70]))
    return files, hits


def selftest():
    """校验器先自证：样本取自真实文件形态（注释 / 字符串 / 三引号 / heredoc / $( )）。

    条数：`cases` **必须为 14**（`:410` 自断言，少跑一条即判失败，防"用例被静默截断还打印通过"）；
    另有跨行结构 4 格（三引号 / heredoc / 跨行引号 / 注释撇号）、遮蔽起始族 6 格（`GAP-20260927-17`）、
    豁免口 3 格（`selftest_fp`：正控 / 负控 / 去重）。
    """
    ok = True
    cases = [
        (".sh", "x=1  # 注释里的全角逗号，不该抓", False),
        (".sh", "echo '字符串里的全角逗号，不该抓'", False),
        (".sh", "if [ $a = 1，]; then", True),
        (".sh", 'say "x: $( [ "$A" = "1" ] && echo \'不触碰（a）\')"', False),
        (".yaml", "cmd: run  --flag，x", True),
        (".yaml", "# 注释，不抓", False),
        (".py", "s = '，'  # 字符串，不抓", False),
        (".py", "x = 1，2", True),
        (".go", 'if s == "，" {', False),
        (".go", "// 注释里的（全角）不抓", False),
        (".json", '{"a": "，"}', False),
        (".json", '{"a": 1，}', True),
        (".toml", "k = '，'", False),
        (".sh", "echo x='，'", False),
    ]
    assert len(cases) == 14, "自证用例数必须为 14，实际 %d（少跑即失效）" % len(cases)
    for ext, line, want in cases:
        got = any(ch in strip_noncode(line, ext) for ch in FULLW)
        ok = ok and (got == want)
        print("  %s [%s] 期望=%s 实际=%s  « %s" % ("✓" if got == want else "✗", ext, want, got, line[:56]))
    py = '"""\n模块说明（含全角括号）\n"""\nx = 1，2\ny = "，"\n'
    pl = strip_blocks(py).split("\n")
    d = any(ch in strip_noncode(l, ".py") for ch in FULLW for l in pl[:2])
    c = any(ch in strip_noncode(l, ".py") for ch in FULLW for l in pl[3:4])
    ok = ok and (not d) and c
    print("  %s [三引号] 段落不抓=%s（期望 False）· 段外真代码抓=%s（期望 True）" % ("✓" if (not d and c) else "✗", d, c))
    hd = "cat <<'EOF' > x\n不触碰（body）\nEOF\necho ，真代码\n"
    hl = mask_heredocs(hd).split("\n")
    hb = any(ch in strip_noncode(l, ".sh") for ch in FULLW for l in hl[1:2])
    hc = any(ch in strip_noncode(l, ".sh") for ch in FULLW for l in hl[3:])
    ok = ok and (not hb) and hc
    print("  %s [heredoc] 正文不抓=%s（期望 False）· 段外真代码抓=%s（期望 True）" % ("✓" if (not hb and hc) else "✗", hb, hc))
    # GAP-20260927-17：**引号内的** `<<TAG` 不算 heredoc 起始（S 变体：行内字符级状态机 +
    # 跨行持态 + 注释行护）。修前 5/5 误判（引号内起始被当起始 ⇒ 掩到文件末 = 门整段失明）。
    hcases = [
        ("C1 单引号内", "printf '%s\\n' '…<<PYEOF…'\n", 0, 0),
        ("C2 双引号内(跨行串)", "printf \"…<<'PYEOF'\\n%s\\nPYEOF\" \"$1\" \"$X\"\n", 0, 0),
        ("C3 add_step 串内", "add_step pub \"python3 - <<'PYEOF'\"\n", 0, 0),
        ("C4 引号内起始不得吞其后真起始（被掩 1；修前掩到末）", "printf '%s' '<<PYEOF'\nx=$(cat <<'EOF'\nbody\nEOF\n", 0, 1),
        ("C5 双引号内 $( 里是真起始（防误杀锁·被掩 1）", "FAST_NAMES=\"$(cat <<'EOF'\nbody\nEOF\n)\"\n", 0, 1),
        ("C6 真起始体内含括号（被掩 1）", "$(python3 - \"$PLIST\" <<'PY'\nprint(1)\nPY\n", 0, 1),
    ]
    for hname, hsrc, hn_want, hm_want in hcases:
        _ht, _hst = mask_heredocs_chk(hsrc)
        _hm = sum(1 for _a, _b in zip(hsrc.split("\n"), _ht.split("\n")) if _b == "" and _a.strip() != "")
        _hgood = (len(_hst) == hn_want) and (_hm == hm_want)
        ok = ok and _hgood
        print("  %s [%s] 起始=%d（期望 %d）· 被掩=%d（期望 %d）" % ("✓" if _hgood else "✗", hname, len(_hst), hn_want, _hm, hm_want))
    mq = chr(10).join(['git commit -q -m "第一行（全角）',
                       '第二行，仍属字符串',
                       '第三行"',
                       'echo ，真代码',
                       ''])
    ml = mask_multiline_quotes(mq).split("\n")
    mb = any(ch in strip_noncode(l, ".sh") for ch in FULLW for l in ml[1:3])
    mc = any(ch in strip_noncode(l, ".sh") for ch in FULLW for l in ml[3:])
    ok = ok and (not mb) and mc
    print("  %s [跨行引号] 串内不抓=%s（期望 False）· 串外真代码抓=%s（期望 True）" % ("✓" if (not mb and mc) else "✗", mb, mc))
    cm = "# 批 2'.6 注释里带撇号（全角括号）" + chr(10) + "echo ，真代码" + chr(10)
    cl = mask_multiline_quotes(cm).split(chr(10))
    cb = any(ch in strip_noncode(l, ".sh") for ch in FULLW for l in cl[:1])
    cc = any(ch in strip_noncode(l, ".sh") for ch in FULLW for l in cl[1:])
    ok = ok and (not cb) and cc
    print("  %s [注释撇号] 注释内不抓=%s（期望 False）· 下一行真代码抓=%s（期望 True）" % ("✓" if (not cb and cc) else "✗", cb, cc))
    ok = ok and selftest_fp()
    print("  自证：%s（14 条单行 + 4 条跨行结构 + 6 条遮蔽起始族 + 3 条豁免口）" % ("通过 ✓" if ok else "**失败** ✗"))
    return ok


def selftest_fp():
    """`[豁免口]` 正/负控 —— 证明 `KNOWN_FP` 真的覆盖**禁用词**那一半（`Q-120` 的修法有牙）。

    做法 = 端到端：临时树里放两行同名禁用词，只登记第 1 行 ⇒
      正控：第 1 行改记豁免、第 2 行仍判 `禁用词`（豁免只挡登记那一行）；
      负控：清空登记表 ⇒ 两行都判 `禁用词`（证明上面那 1 处是**豁免口**挡的，不是判据失灵）。
    另测去重：同一行既含禁用词又含全角标点 ⇒ 豁免桶里只出现一次。
    """
    import tempfile

    global ROOTS, WHITELIST, KNOWN_FP
    saved = (ROOTS, WHITELIST, KNOWN_FP, os.getcwd())
    ok = True
    try:
        tmp = tempfile.mkdtemp(prefix="zh-en-fp-selftest-")
        os.makedirs(os.path.join(tmp, "core"))
        probe = os.path.join(tmp, "core", "probe_fp.py")
        with open(probe, "w", encoding="utf-8") as fh:
            fh.write('a = "虫巢"\n'
                     'b = "虫巢"\n'
                     'c = 1，2  # 虫巢 同行两种命中\n')
        os.chdir(tmp)
        ROOTS = ["core"]
        WHITELIST = set()
        # 登记第 1 行（纯禁用词）与第 3 行（禁用词 + 全角标点**同行** ⇒ 考去重）。
        # 键 = `(件, 内容子串)`（新式样 · 不吃行号）。
        KNOWN_FP = {("core/probe_fp.py", 'a = "虫巢"'): "自证登记行①",
                    ("core/probe_fp.py", 'c = 1，2  # 虫巢 同行两种命中'): "自证登记行③（同行两种命中 ⇒ 只许列一次）"}

        _, h = scan()
        got_fp, got_fb, got_fw = len(h["已登记假红(豁免)"]), len(h["禁用词"]), len(h["全角标点(代码区)"])
        c1 = (got_fp == 2 and got_fb == 1 and got_fw == 0)
        print("  %s [豁免口·正控] 登记 2 行（第 3 行同行两种命中）⇒ 豁免=%d（期望 2·去重生效）· 禁用词=%d（期望 1，只剩未登记的第 2 行）· 全角=%d（期望 0）"
              % ("✓" if c1 else "✗", got_fp, got_fb, got_fw))
        ok = ok and c1

        KNOWN_FP = {}
        _, h2 = scan()
        d_fp, d_fb, d_fw = len(h2["已登记假红(豁免)"]), len(h2["禁用词"]), len(h2["全角标点(代码区)"])
        c2 = (d_fp == 0 and d_fb == 3 and d_fw == 1)
        # ★ 本改动的新牙（**折进同一格**带过，不另开格）：登记过期 = 子串失配（0 命中）/ 多处命中（>1）
        #   ⇒ 必须判「登记过期」并**点名**（旧式样里登记失效是**静默**的，那正是要治的病）。
        KNOWN_FP = {("core/probe_fp.py", 'a = "压根不在件里的串"'): "自证登记行①"}
        st0, ms0 = registry_stale()
        c3 = (len(st0) == 1 and not ms0 and st0[0][0] == "core/probe_fp.py"
              and st0[0][1] == 'a = "压根不在件里的串"' and "命中 0 行" in st0[0][2])
        KNOWN_FP = {("core/probe_fp.py", "虫巢"): "自证登记行①"}
        st1, _ = registry_stale()
        c4 = (len(st1) == 1 and "命中 3 行" in st1[0][2])
        print("  %s [豁免口·负控] 清空登记 ⇒ 豁免=%d（期望 0）· 禁用词=%d（期望 3）· 全角=%d（期望 1）—— 上面那两处是**登记表**挡的，不是判据失灵；"
              "登记过期：子串失配 %d 条（期望 1 · 点名）· 多处命中 %d 条（期望 1 · 点名）"
              % ("✓" if (c2 and c3 and c4) else "✗", d_fp, d_fb, d_fw, len(st0), len(st1)))
        ok = ok and c2 and c3 and c4
        # GAP-20260927-128/-129：遮蔽未闭合必须**可被检出**（成对：未闭合 / 闭合）
        _m1, _u1 = mask_heredocs_chk("echo hi <<PYEOF\nnot closed\n")
        _m2, _u2 = mask_heredocs_chk("cat <<PYEOF\nbody\nPYEOF\ntail\n")
        c5 = (_u1 == [1]) and (_u2 == []) and ("not closed" not in _m1) and ("tail" in _m2)
        # `GAP-20260926-128` 余项：**跨行引号层**同样必须可被检出（成对：未收尾 / 收尾）
        _x1, _xu1 = mask_multiline_quotes_chk('PY_CMD="python3 - \'\'\'\nbody without closer\n')
        _x2, _xu2 = mask_multiline_quotes_chk('git commit -q -m "第一行\n第二行"\ntail\n')
        c6 = (_xu1 == [1]) and (_xu2 == []) and ("body without closer" not in _x1) and ("tail" in _x2)
        # `GAP-20260928-74` 余项：**三引号层**同样必须可被检出（成对：配不齐 / 成对）——
        # 折进同一格带过，不另开格（族体例）。
        _q3 = '"' * 3     # 本件自己也受本层管辖：夹具串里**不得出现**字面三引号（否则自咬）
        _y1, _yu1 = strip_blocks_chk("x = 1\n# 说明：用 " + _q3 + " 包块\ny = 2，3\n" + _q3 + "\n中文，文档\n" + _q3 + "\n")
        _y2, _yu2 = strip_blocks_chk("x = 1\n" + _q3 + "\n中文，文档\n" + _q3 + "\ny = 2，3\n")
        c7 = (_yu1 == [3]) and (_yu2 == [])
        print("  %s [遮蔽未闭合] 未闭合起始行=%s（期望 [1]）· 闭合件=%s（期望 []）· 闭合件尾行保留=%s"
              " ｜ 跨行引号层：未收尾起始行=%s（期望 [1]）· 收尾件=%s（期望 []）· 尾行保留=%s"
              " ｜ 三引号层：配不齐被掩行=%s（期望 [3]）· 成对件=%s（期望 []）"
              % ("✓" if (c5 and c6 and c7) else "✗", _u1, _u2, "tail" in _m2, _xu1, _xu2, "tail" in _x2, _yu1, _yu2))
        ok = ok and c5 and c6 and c7
    finally:
        ROOTS, WHITELIST, KNOWN_FP, cwd = saved
        os.chdir(cwd)
    return ok


def main():
    # ★ `Q-176` 第六批：准拼法 `--self-test`；旧拼法 `--selftest` 作别名（两个都认）
    if "--self-test" in sys.argv or "--selftest" in sys.argv:
        return 0 if selftest() else 1
    if not os.path.isdir("core") and not os.path.isdir("agent"):
        print("✗ 不在仓库根目录（core/ 与 agent/ 都不存在）——拒绝静默空转", file=sys.stderr)
        return 2
    MASK_UNCLOSED.clear()
    MASK_MLQ_UNCLOSED.clear()
    MASK_PYQ_UNCLOSED.clear()
    files, hits = scan()
    if MASK_UNCLOSED:
        # ★ 判不了档（GAP-20260927-128/-129）：遮蔽段没闭合 ⇒ 门在那些行上**失明**，
        #   此时「零命中」不能当绿 ⇒ 一律 rc=2（BLOCKED：不阻塞提交，但不许当绿）。
        print("✗ 遮蔽面有未闭合的 heredoc 段 —— 门在这些行上整段失明，本轮**不给结论**", file=sys.stderr)
        for pf, starts in MASK_UNCLOSED[:10]:
            print("   · %s：起始行 %s（其后直到文件末皆被遮蔽）"
                  % (pf, ", ".join(str(x) for x in starts[:5])), file=sys.stderr)
        print("   ★ 处置：收尾标记必须**独占一行**（现读 `l.strip()==TAG`）；"
              "起始标记出现在引号内时不算真 heredoc（根因档待办）。", file=sys.stderr)
        return 2
    if MASK_MLQ_UNCLOSED:
        # ★ 判不了档（GAP-20260926-128 余项）：跨行引号层没收回尾 ⇒ 门在后半件整段失明，
        #   此时「零命中」不能当绿 ⇒ 一律 rc=2（不给结论：不阻塞提交，但不许当绿）。
        print("⛔ 不给结论（rc=2）：跨行引号层有未收尾的段 —— 门在这些行上整段失明，本轮**判不了**",
              file=sys.stderr)
        for pf, starts in MASK_MLQ_UNCLOSED[:10]:
            print("   · %s：起始行 %s（该行起的引号在本件内无收尾 ⇒ 其后直到文件末皆被当字符串掩掉）"
                  % (pf, ", ".join(str(x) for x in starts[:5])), file=sys.stderr)
        print("   ★ 处置：跨行引号必须有配对收尾（现读 `l.find(quote)`）；"
              "引号起于命令头之后而本件内无收尾时，本层判不了（不把「掩掉后零命中」当绿）。",
              file=sys.stderr)
        return 2
    if MASK_PYQ_UNCLOSED:
        # ★ 判不了档（GAP-20260928-74 余项）：`.py` 三引号层把真代码掩掉了（配不齐的三引号）⇒
        #   门在这些行上失明，此时「零命中」不能当绿 ⇒ 一律 rc=2（不给结论：不阻塞提交，但不许当绿）。
        print("⛔ 不给结论（rc=2）：三引号层有配不齐的段 —— 门在这些行上失明，本轮**判不了**",
              file=sys.stderr)
        for pf, lines in MASK_PYQ_UNCLOSED[:10]:
            print("   · %s：被掩行 %s（对拍源 = %s；该段里正则掩掉的是真代码）"
                  % (pf, ", ".join(str(x) for x in lines[:5]), MASK_PYQ_JUDGE), file=sys.stderr)
        print("   ★ 处置：件里 \"\"\" / ''' 必须成对，且不得出现在注释或普通字符串里"
              "（现读 RE_TRIPLE 就近配对）；配不齐时本层判不了（不把「掩掉后零命中」当绿）。",
              file=sys.stderr)
        return 2
    if files == 0:
        print("✗ 扫到 0 个文件——拒绝静默空转（假绿）", file=sys.stderr)
        return 2
    total = sum(len(hits[k]) for k in ("禁用词", "全角标点(代码区)", "旧角色词"))
    print("扫描文件数 = %d（全角标点只扫无编译器兜底的文件；.go 仅扫术语/旧角色词）" % files)
    for k, v in hits.items():
        print("\n── %s：%d 处" % (k, len(v)))
        for x in v[:15]:
            print("   ", x)
        if len(v) > 15:
            print("    …另 %d 处" % (len(v) - 15))
    # ── 登记表体检：每条 `(件, 子串)` 在它那件里必须**恰好命中 1 行** ⇒ 否则判「登记过期」──────
    stale, missing = registry_stale()
    if missing:
        for m in sorted(set(missing)):
            print("✗ 登记表指向的件不在盘：%s —— 判不了「登记过期」，拒绝静默放过" % m, file=sys.stderr)
        return 2
    print("\n── 登记过期（键失效 ⇒ 必报）：%d 条" % len(stale))
    for path, sub, why, hit in stale:
        print("   · %s  «%s» ⇒ %s%s" % (path, sub[:70], why,
              ("（命中行 %s）" % ", ".join(str(h) for h in hit)) if hit else ""))
    if stale:
        print("   ★ 处置：登记过期 ⇒ **修登记**（换一段逐字片段）或**改件**——不得当作豁免成功")
    print("\n合计命中 = %d" % total)
    return 0 if (total == 0 and not stale) else 1


if __name__ == "__main__":
    sys.exit(main())
