#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-glossary.py —— 术语表门禁（可执行版 · 三档退码）

设计真源：Zerg-内部文档/01-设计/设计-文档体系-v1.0.md
            §0.6 P6 ④（正文出现术语表外的译法 ⇒ 告警，**先非阻塞、收敛后阻塞**，节奏同 §6 的 G-3）
            §5.6②（双语目录 · 译页字段）· §5.6③（导出规则⑥「en 列为空 ⇒ 不得进译稿」）
            §14.3（判据③ 代码标识符零改动）· §2.4（漂移三件套③ 源文注释）
唯一真源：docs/术语表.tsv（本脚本只读它，不另存词表；导出物见 scripts/docs/gen-glossary-exports.py）

三档判据（**这就是「三档退码」的三档**，退码见文末）：
  T1 阻塞档 —— 与术语收敛进度无关，第一天就能阻塞的三条：
       B3  `en` 列为空的词条，其中文原串出现在**译稿正文**（导出规则⑥「不得进译稿」）
       B4  英文页缺译页三必填 frontmatter：`translation_of` / `ai_assisted` / `translation_status`
       B5  译页**代码标识符零改动**：正文的行内代码集合 ≠ 该页 HTML 注释里源文的集合
           （判据 = §14.3 ③；集合相等 ⇒ 可过人；不等 ⇒ 必须人重写。无源文注释 ⇒ 降为 T3 告警）
  T2 术语档 —— 「术语表外的译法 / 禁写词」，按 G-3 节奏**先非阻塞**（rc=1）：
       `forbid_zh` 词出现在中文页 · `forbid_en` 词出现在英文页 · `forbid_en` 词出现在中文页
       ⇒ 加 `--enforce` 才升为阻塞（rc=2）。**收敛后再把默认值翻过来**（这一步要一句令）
  T3 告警档 —— 恒非阻塞：
       W2 英文页只出现 `en` 列的大小写变体、未见正名 · W3 `translation_of` 指向的源页不存在
       · W4 译页没有 HTML 注释（未按「源文注释」写 —— 漂移三件套③ 先非阻塞）
       · W5 正文私建术语表（2026-09-20 承接项「术语表只有一份」）：
         全局术语**只有一份主表** `docs/术语表.tsv`，正文**只引用、不复制**；本文专有的一组词
         可留在本文，但**不得叫「术语表」**。判据 = 1–4 级标题的**标题文本**含「术语表」或恰为「术语」
         ⇒ 告警并提示（全局 ⇒ 进主表；本文专有 ⇒ 改名，如「本文概念/字段说明」）。
         三条防假红：① 先剥 `{: #锚 }` —— 锚点里带「术语表」是**引用**不是私建
         （本仓实证：`docs/en/_drafts/设计-文档体系-v1.0.md` 的 `# Glossary: … {: #术语表：… }`）；
         ② 围栏代码块里的行不算正文；③ 每行可用 `<!-- glossary-allow -->` 单条豁免。

扫描口径（三条，写死防假红；每条都是被咬过的地方）
  ① **剥 HTML 注释与围栏代码块**后才查 B3（源文注释是规定动作；注释里的中文不是漏译）
  ② 每行可用 `<!-- glossary-allow -->`（可跟 `: 理由`）单条豁免 —— **引述原话 / 文件名 / 旧称取证**是法定例外
     （`设计-子端沙箱化` 用词纪律：仅在引述原话处保留原词）；豁免不隐藏：命中计入「单条豁免」计数
  ③ 词表文件自身与两个脚本在 WHITELIST 里（否则规则文本自己就红）

用法：
  python3 scripts/gates/check-glossary.py [--lang zh,en] [--paths P …] [--enforce] [--no-identifiers] [--selftest]
  python3 scripts/gates/check-glossary.py --scope devdocs [--root <目录>]   # D3①：扫开发文档面（Zerg-内部文档）
     --enforce        把 T2 术语档升为阻塞（收敛后的口径）
     --no-identifiers 关掉 B5/W4（大型树上想只看词表命中时用）
     --scope devdocs  换到**开发文档面**（D3 第一步 · 默认根见 devdocs_root()`；语料一律按**中文面**判）
     --root <目录>    换扫描根（缺省 = 本脚本的上一级 = 主仓根；**默认行为一字不变**）
退出码：0 = 干净 · 1 = 仅 T2/T3 命中（非阻塞）· 2 = **不给结论 / 阻塞**
        （T1 命中 · `--enforce` 下 T2 命中 · **环境/输入异常与空转**）
        ★ 2026-09-20 承接项「术语表只有一份」第 4 条：本脚本原 `rc=3` 一律**统一成 `rc=2`** ——
          与仓内其余门脚本（check-doc-name/meta/freshness、precommit-gates.sh）的 0/1/2 三档同一口径；
          对外效果 = 挂在公开树（无 `docs/` 面）时落 **BLOCKED（不给结论）**，不会被读成 FAIL。

D3 · 开发文档门禁（2026-09-21 第一步 · 换根与档位）
------------------------------------------------
  命题（《待办-20260920.md》D3 · 设计稿 `设计-开发文档门禁-v1.0.md`）：`Zerg-内部文档/`（开发文档侧）
  同样受命名/文件头/术语/引用存在性/新鲜度约束，但**原先不受任何门禁管辖** ⇒ 全靠人记。
  本步只做「**换根 + 加档位**」两件近零成本的事，**默认行为一字不变**：
    ① `--root`（缺省仍是主仓根 ⇒ 不传本旗标时输出与从前逐字节相同）；
    ② `--scope devdocs` —— 扫描根换成开发文档面（与门⑫ `check-doc-cmds.py` **同根同默认**：
       `ZERG_DEVDOCS_ROOT` > `<仓根 的上一级>/Zerg-内部文档/项目文档/v2.5.10`）。
  为什么开发文档面**一律按中文面判**（`lang_of()`）：Zerg-内部文档 没有 `docs/zh` + `docs/en` 的双语分家，
  正文是中文稿；照默认口径（第二段是 `zh` 才算中文）93 篇会被全判成「英文译页」⇒ 每篇都吃
  B4「译页三必填」= **93 处假红**。这不是放宽判据，是**把面认对**（判据本身一字未改）。
  为什么主表仍是主仓的 `docs/术语表.tsv`（不随 `--root` 走）：承接项「**术语表只有一份**」——
  换根换的是**被扫的语料**，不是真源；开发文档面的禁写词照样照这份主表判。
  根不在（仓外不可达）⇒ `rc=2` **不给结论**（设计稿 §五：不静默当绿 ✗）。
"""
import os
import re
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
MAIN = os.path.join(ROOT, "docs", "术语表.tsv")
COLS = ["zh", "en", "id", "do_not_translate", "forbid_zh", "forbid_en", "owner", "note"]
ALLOW = "<!-- glossary-allow -->"          # 单条豁免标记（后面可跟 `: 理由`）
RE_ALLOW = re.compile(r"<!--\s*glossary-allow\b")
WHITELIST = ("docs/术语表.tsv", "docs/术语表.md", "docs/术语表.jieba.txt", "docs/术语表.forbid.txt",
             "docs/术语表-说明.md", "scripts/gates/check-glossary.py", "scripts/docs/gen-glossary-exports.py")
RE_COMMENT = re.compile(r"<!--.*?-->", re.S)
RE_FENCE = re.compile(r"^```.*?^```", re.S | re.M)
RE_CODE = re.compile(r"`([^`\n]+)`")
RE_FM = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.S)
CJK = re.compile(r"[\u3400-\u9fff]")
# ── W5 正文私建术语表（2026-09-20 承接项「术语表只有一份」）：判据见文件头 T3 段 ─────────────
RE_ANCHOR = re.compile(r"\{:\s*#[^}]*\}")                        # `{: #锚 }` 先剥：锚点是引用不是私建
RE_HEADING = re.compile(r"^\s{0,3}(#{1,4})\s*(.+?)\s*#*\s*$")   # 1–4 级标题（ATX 形态）
RE_PRIVATE_GLOSSARY = re.compile(r"术语表")                        # 标题文本含「术语表」
PRIVATE_GLOSSARY_EXACT = "术语"                                    # 或标题文本恰为「术语」（去尾标点后）


def split_multi(v):
    out = []
    for x in v.split("|"):
        x = x.strip()
        if x and x not in out:
            out.append(x)
    return out


def devdocs_root(repo_root=ROOT):
    """开发文档面根（D3① · 与门⑫ `check-doc-cmds.py` **同根同默认**）。

    优先级：`ZERG_DEVDOCS_ROOT` > `<仓根 的上一级>/Zerg-内部文档/项目文档/v2.5.10`。
    为什么是**仓外**路径：`Zerg-内部文档/` 与主仓并列（2026-09-19 分家）、且它不开源 ⇒ 不进公开树；
    D3 只把它当**被管对象**读，不搬进仓。根不在 ⇒ 调用方按 `rc=2` 处理（不静默当绿）。
    """
    env = os.environ.get("ZERG_DEVDOCS_ROOT")
    if env:
        return os.path.abspath(env)
    # 同级根归一（GAP-20260927-82 批1）：接现成同级根旋钮 ZERG_DOCS_ALT
    # （与 core/internal/statepath.DocsAltRoot() 同根同默认）/ ZERG_DOCS
    # （同 scriptInvDocsRoot）> <仓根 的上一级>/Zerg-内部文档 —— 全族「读外部树」判据件共用一个取根规则。
    sibling = (os.environ.get("ZERG_DOCS_ALT") or os.environ.get("ZERG_DOCS")
               or os.path.join(os.path.dirname(os.path.abspath(repo_root)), "Zerg-内部文档"))
    return os.path.abspath(os.path.join(sibling, "项目文档", "v2.5.10"))


def lang_of(rel, scope=None):
    """一页归中文面还是英文面。

    默认口径（scope != devdocs）**一字未改**：相对路径第二段是 `zh` ⇒ 中文，否则英文。
    `devdocs`（开发文档面）⇒ **一律中文**：Zerg-内部文档 没有 `docs/zh` + `docs/en` 双语分家，
    正文是中文稿；照默认口径会把这 93 篇全判成「英文译页」⇒ 每篇都吃 B4「译页三必填」= 假红。
    """
    if scope == "devdocs":
        return "zh"
    return "zh" if rel.split(os.sep)[1:2] == ["zh"] else "en"


def load_glossary(path=None):
    # `Q-176` 第五批：给 `--self-test` 留的**唯一**换表口（不设此环境变量 ⇒ 一字不变，仍是 MAIN）。
    # 为什么需要：自检**不许读写仓内件** ⇒ 合成术语表落在临时目录，由这个变量指过去。
    path = path or os.environ.get("ZERG_GLOSSARY_TSV") or MAIN
    if not os.path.isfile(path):
        raise EnvironmentError("术语表主表不存在：%s" % path)
    rows, header = [], None
    for i, line in enumerate(open(path, encoding="utf-8").read().split("\n"), 1):
        if i == 1:
            header = line.rstrip("\r").split("\t")
            if header != COLS:
                raise EnvironmentError("表头不符：%s" % header)
            continue
        if not line.strip():
            continue
        f = line.rstrip("\r").split("\t")
        if len(f) != len(COLS) or not f[0].strip():
            raise EnvironmentError("第 %d 行不合法（字段数 %d / zh 空）" % (i, len(f)))
        rows.append(dict(zip(COLS, f)))
    if not rows:
        raise EnvironmentError("主表零数据行")
    return rows


def strip_comments(text):
    return RE_COMMENT.sub(lambda m: "\n" * m.group(0).count("\n"), text)


def strip_fences(text):
    return RE_FENCE.sub(lambda m: "\n" * m.group(0).count("\n"), text)


def parse_frontmatter(text):
    m = RE_FM.match(text)
    if not m:
        return {}
    fm = {}
    for l in m.group(1).split("\n"):
        if ":" in l and not l.lstrip().startswith("#"):
            k, v = l.split(":", 1)
            fm[k.strip()] = v.strip().strip('"').strip("'")
    return fm


def wb(word):
    """ASCII 词用词界匹配：防 `pod` 命中 `podcast`，也防命**路径里的同名段**（`/usr/local/bin`）。

    左侧边界除词字符外再排除 `/` `.`（路径/扩展名分隔）—— 假红一次就够了（本仓踩过三轮）。
    CJK 词无词界概念，直接子串匹配。
    """
    if CJK.search(word):
        return re.compile(re.escape(word))
    return re.compile(r"(?<![\w\-/.])" + re.escape(word) + r"(?![A-Za-z0-9_\-])")


def scan_page(rel, text, gloss, lang, identifiers=True, exists=lambda p: True):
    """单页判定：返回 (blocking=T1, terms=T2, warnings=T3, exempted)。纯函数 —— 自证直接调它。"""
    blocking, terms, warnings = [], [], []
    exempted = 0
    lines = text.split("\n")
    # 英文页：**只扫剥掉 HTML 注释后的文本** —— 注释块按 §2.4③ 是源文引述（源文自己会写出禁写词，
    # 例如术语表样例行里的 pod|container），拿它当译法违规是假红。中文页不剥（注释不是契约）。
    scan_lines = strip_comments(text).split("\n") if lang == "en" else lines
    body = strip_fences(strip_comments(text))
    fm = parse_frontmatter(text)
    pairs_zh, pairs_en = [], []
    for r in gloss:                       # 同一禁写词被多行登记时只报一次（取首个正名）
        for w in split_multi(r["forbid_zh"]):
            if w not in [p[0] for p in pairs_zh]:
                pairs_zh.append((w, r["zh"]))
        for w in split_multi(r["forbid_en"]):
            if w not in [p[0] for p in pairs_en]:
                pairs_en.append((w, r["zh"]))
    forbid_zh, forbid_en = pairs_zh, pairs_en
    en_empty = [r for r in gloss if not r["en"].strip()]

    def hits(pairs, target, tag):
        n = 0
        for i, l in enumerate(lines, 1):
            target_line = scan_lines[i - 1] if i - 1 < len(scan_lines) else ""
            for w, canon in pairs:
                if wb(w).search(target_line):
                    if RE_ALLOW.search(l):   # 豁免标记永远在**原行**上找（剥注释后它就没了）
                        n += 1
                        continue
                    target("%s:%d  [%s]  %s  ← 正名「%s」" % (rel, i, tag, target_line.strip()[:60], canon))
        return n

    if lang == "zh":
        hits(forbid_zh, terms.append, "T2 禁用中文写法")
        hits(forbid_en, terms.append, "T2 中英混写")
    else:
        hits(forbid_en, terms.append, "T2 禁用英文写法")
        for r in en_empty:
            if wb(r["zh"]).search(body):
                blocking.append("%s  [T1 B3 en 列为空不得进译稿]  「%s」  ← %s"
                                % (rel, r["zh"], r["note"][:40]))
        is_translation = not rel.endswith("README.md")
        if is_translation:
            miss = [k for k in ("translation_of", "ai_assisted", "translation_status") if not fm.get(k)]
            if miss:
                blocking.append("%s  [T1 B4 译页三必填缺 %s]" % (rel, "/".join(miss)))
            if fm.get("translation_of") and not exists(fm["translation_of"]):
                warnings.append("%s  [T3 W3 translation_of 指向不存在的源页]  %s"
                                % (rel, fm["translation_of"]))
        if identifiers and is_translation:
            body_ids = set(RE_CODE.findall(strip_fences(strip_comments(text))))
            src_ids = set()
            for m in RE_COMMENT.finditer(text):
                src_ids |= set(RE_CODE.findall(m.group(0)))
            if not src_ids:
                warnings.append("%s  [T3 W4 无源文注释，零改动无法机械判]" % rel)
            elif body_ids != src_ids:
                missing = sorted(src_ids - body_ids)
                extra = sorted(body_ids - src_ids)
                if missing:
                    blocking.append("%s  [T1 B5 标识符被改动/丢失 %d 个]  %s"
                                    % (rel, len(missing), "、".join(missing[:5])))
                if extra:
                    blocking.append("%s  [T1 B5 正文多出源页没有的代码标识符 %d 个]  %s"
                                    % (rel, len(extra), "、".join(extra[:5])))
        for r in gloss:                       # W2：只有大小写变体、未见正名
            e = r["en"].strip()
            if e and e.isascii() and e.islower() and " " not in e and e.isalpha():
                if wb(e.capitalize()).search(body) and not wb(e).search(body):
                    warnings.append("%s  [T3 W2 只有大小写变体 %s，未见正名 %s]" % (rel, e.capitalize(), e))
    # W5 正文私建术语表（T3 · 恒非阻塞 · 2026-09-20 承接项）：标题文本含「术语表」或恰为「术语」。
    #   为什么扫 `body`（已剥围栏与注释）：围栏里的 `## 术语表` 是**示例**不是正文；
    #   为什么用 `lines` 找豁免标记：剥注释后标记本身也没了（同 T2 `hits()` 的纪律）。
    for i, l in enumerate(body.split("\n"), 1):
        m = RE_HEADING.match(l)
        if not m:
            continue
        title = RE_ANCHOR.sub("", m.group(2)).strip().strip("*_").strip().rstrip("：:。.、，,")
        if not (RE_PRIVATE_GLOSSARY.search(title) or title == PRIVATE_GLOSSARY_EXACT):
            continue
        if RE_ALLOW.search(lines[i - 1] if i - 1 < len(lines) else ""):
            continue                    # 单条豁免（标记本身计入文末「单条豁免」，与 T2 同一账）
        warnings.append("%s:%d  [T3 W5 正文私建术语表]  %s  ← 全局术语只有一份（docs/术语表.tsv）："
                        "全局 ⇒ 进主表；本文专有 ⇒ 改名（如「本文概念/字段说明」）"
                        % (rel, i, RE_ANCHOR.sub("", l).strip()[:60]))
    exempted += sum(1 for l in lines if RE_ALLOW.search(l))
    return blocking, terms, warnings, exempted


def walk_files(paths, root, langs, scope=None):
    out = []
    for p in paths:
        ap = p if os.path.isabs(p) else os.path.join(root, p)
        if os.path.isfile(ap):
            out.append((os.path.relpath(ap, root), ap))
            continue
        if not os.path.isdir(ap):
            continue
        for dp, dn, fn in os.walk(ap):
            dn[:] = [d for d in dn if d not in ("node_modules", ".git")]
            for f in fn:
                if not f.endswith(".md"):
                    continue
                rel = os.path.relpath(os.path.join(dp, f), root)
                lang = lang_of(rel, scope)
                if lang in langs:
                    out.append((rel, os.path.join(dp, f)))
    return out


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


def usage_help():
    """`--help` 只印这一段：本件 docstring 的「用法」块（**唯一真源** —— 不另存第二份文案，防漂）。"""
    m = re.search(r"^用法：.*?(?=\n\S)", __doc__ or "", re.S | re.M)
    return (m.group(0) if m else USAGE).rstrip()


def main():
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        return rc_q176
    argv = sys.argv[1:]
    # ── 缺口 `GAP-20260926-168`：**旗标面**（只动这一层 —— 扫描判据/词表/输出行文案一字未改）
    #    ① `--help` / `-h` ⇒ 只印用法 + `rc=0`（**不进扫描**）；口径照同族
    #       `scripts/docs/gen-glossary-exports.py` 与 `scripts/gates/check-build-tags.py`
    #       （`--help`/`-h` 只读印用法 · 未知参数 `rc=2`）。
    #    ② 闭集外的旗标 ⇒ `✗ 未知参数: …` + `rc=2`（**逐字点名 · 不静默吞**），同样不进扫描。
    #    闭集 = 本件 docstring「用法」段逐字（唯一真源）⟷ 下面这一处；两处不许各说各话。
    #    `--json` 在闭集里只是为了「用法与实现同一份名单」—— 它先被 `Q-176` 那一支拦下（口径未动）。
    known = {"--root", "--scope", "--lang", "--paths", "--self-test", "--selftest",
             "--enforce", "--json", "--no-identifiers", "--help", "-h"}
    unknown = [a for a in argv if a.startswith("-") and a not in known]
    if unknown:
        print("✗ 未知参数: %s" % " ".join(unknown), file=sys.stderr)
        return 2
    if "--help" in argv or "-h" in argv:
        print(usage_help())
        return 0
    if "--self-test" in argv or "--selftest" in argv:
        return self_test()
    root = ROOT
    scope = None
    langs, paths, i = set(), [], 0
    while i < len(argv):
        if argv[i] == "--root" and i + 1 < len(argv):
            root = os.path.abspath(argv[i + 1])
            i += 2
            continue
        if argv[i] == "--scope" and i + 1 < len(argv):
            scope = argv[i + 1]
            if scope not in ("docs", "devdocs"):
                print("✗ 未知 --scope %r（表内只有 docs / devdocs）⇒ 不给结论" % scope, file=sys.stderr)
                return 2
            i += 2
            continue
        if argv[i] == "--lang" and i + 1 < len(argv):
            langs = set(argv[i + 1].split(",")) & {"zh", "en"}
            i += 2
            continue
        if argv[i] == "--paths":
            i += 1
            while i < len(argv) and not argv[i].startswith("--"):
                paths.append(argv[i])
                i += 1
            continue
        i += 1
    if scope == "devdocs" and "--root" not in argv:
        # ── D3① 换根：开发文档面带自己的默认根（与门⑫ 同根同默认），不借用主仓根
        root = devdocs_root(ROOT)
        if not os.path.isdir(root):
            print("✗ 开发文档面根不在（%s）⇒ 不给结论（rc=2 · 仓外不可达不许静默当绿）" % root,
                  file=sys.stderr)
            return 2
        langs = {"zh"}          # 开发文档面没有 zh/en 双语分家 ⇒ 一律中文面（见 lang_of()）
        if not paths:
            paths = [root]
    if not langs:
        langs = {"zh", "en"}
    if not paths:
        paths = [os.path.join("docs", l) for l in sorted(langs)]
    try:
        gloss = load_glossary()
        files = walk_files(paths, root, langs, scope)
    except EnvironmentError as e:
        print("✗ %s" % e, file=sys.stderr)
        return 2
    if not files:
        print("✗ 扫到 0 个 .md —— 拒绝静默空转（假绿）。检查 --paths 与 docs/zh|docs/en 是否存在",
              file=sys.stderr)
        return 2
    blocking, terms, warnings, exempt = [], [], [], 0
    for rel, ap in files:
        if rel in WHITELIST:
            continue
        try:
            text = open(ap, encoding="utf-8", errors="replace").read()
        except Exception as e:
            print("✗ 读不了 %s：%s" % (ap, e), file=sys.stderr)
            return 2
        lang = lang_of(rel, scope)
        b, t, w, x = scan_page(rel, text, gloss, lang,
                               identifiers="--no-identifiers" not in argv,
                               exists=lambda p: os.path.isfile(os.path.join(root, p)))
        blocking += b
        terms += t
        warnings += w
        exempt += x
    print("术语表 %d 条（en 空 %d 条）· 扫描 %d 篇（%s；白名单 %d 篇）· enforce=%s"
          % (len(gloss), sum(1 for r in gloss if not r["en"].strip()), len(files),
             "+".join(sorted(langs)), len(WHITELIST), "--enforce" in argv))
    for title, bag in (("T1 阻塞档（rc=2）", blocking),
                       ("T2 术语档（非阻塞 · --enforce 时 rc=2）", terms),
                       ("T3 告警档（非阻塞）", warnings)):
        print("\n── %s：%d 处" % (title, len(bag)))
        for x in bag[:20]:
            print("   ", x)
        if len(bag) > 20:
            print("    …另 %d 处" % (len(bag) - 20))
    print("\n合计：T1 %d · T2 %d · T3 %d · 单条豁免 %d" % (len(blocking), len(terms), len(warnings), exempt))
    if blocking or ("--enforce" in argv and terms):
        return 2
    if terms or warnings:
        return 1
    return 0


def selftest():
    """自证：断言用例条数（防「用例被静默截断还打印通过」），逐条验证三档判据。"""
    gloss = [
        dict(zip(COLS, ["虫茧", "cocoon", "cocoon", "cocoon", "虫巢|集装箱", "pod", "Mr2109", "只此一译"])),
        dict(zip(COLS, ["承接项", "", "", "", "", "", "Mr2109", "英文待拍"])),
        dict(zip(COLS, ["虫卵", "egg", "", "", "沙箱|容器", "sandbox|container", "Mr2109", ""])),
        dict(zip(COLS, ["主控", "core", "core", "core", "", "", "Mr2109", ""])),
    ]
    fm_ok = ("---\ntitle: T\ntranslation_of: docs/x.md\nai_assisted: ai\n"
             "translation_status: machine_draft\n---\n")
    cases = [
        ("T2 中文页禁写 虫巢", lambda: scan_page("docs/zh/a.md", "这里叫虫巢。", gloss, "zh")[1], 1),
        ("T2 正名不命中", lambda: scan_page("docs/zh/a.md", "这里叫虫茧。", gloss, "zh")[1], 0),
        ("T2 单条豁免生效", lambda: scan_page("docs/zh/a.md", "旧称虫巢。<!-- glossary-allow -->", gloss, "zh")[1], 0),
        ("T2 英文页禁写 pod", lambda: scan_page("docs/en/a.md", fm_ok + "the pod runs\n", gloss, "en")[1], 1),
        ("T2 英文页注释里的 pod 不算", lambda: scan_page("docs/en/a.md", fm_ok + "x\n<!-- the pod runs -->\n", gloss, "en")[1], 0),
        ("T2 中文页注释里的 pod 仍算", lambda: scan_page("docs/zh/a.md", "x\n<!-- the pod runs -->\n", gloss, "zh")[1], 1),
        ("T2 同词多行登记只报一次", lambda: scan_page("docs/en/a.md", fm_ok + "container leak\n", gloss, "en")[1], 1),
        ("T2 词界防误伤 podcast", lambda: scan_page("docs/en/a.md", fm_ok + "a podcast\n", gloss, "en")[1], 0),
        ("T2 路径里的同名段不算（/pod/x）", lambda: scan_page("docs/zh/a.md", "路径 `/pod/x`", gloss, "zh")[1], 0),
        ("T2 中文页混写 pod", lambda: scan_page("docs/zh/a.md", "一个 pod", gloss, "zh")[1], 1),
        ("T1 B3 en 空词条进译稿", lambda: scan_page("docs/en/a.md", fm_ok + "承接项 list\n", gloss, "en")[0], 1),
        ("T1 B3 注释里的中文不算", lambda: scan_page("docs/en/a.md", fm_ok + "x\n<!-- 承接项 -->\n", gloss, "en")[0], 0),
        ("T1 B4 译页三必填", lambda: scan_page("docs/en/a.md", "---\ntitle: T\n---\nbody\n", gloss, "en")[0], 1),
        ("T1 B4 README 豁免", lambda: scan_page("docs/en/README.md", "body\n", gloss, "en")[0], 0),
        ("T1 B5 标识符集合相等", lambda: scan_page(
            "docs/en/a.md", fm_ok + "run `doc_search`\n<!-- 跑 `doc_search` -->\n", gloss, "en")[0], 0),
        ("T1 B5 标识符被译（丢 1 多 1）", lambda: scan_page(
            "docs/en/a.md", fm_ok + "run `docSearch`\n<!-- 跑 `doc_search` -->\n", gloss, "en")[0], 2),
        ("T3 W2 只有大小写变体", lambda: scan_page("docs/en/a.md", fm_ok + "Cocoon is here\n", gloss, "en",
                                                  identifiers=False)[2], 1),
        ("T3 W3 translation_of 悬空", lambda: scan_page("docs/en/a.md", fm_ok + "x\n", gloss, "en",
                                                       identifiers=False, exists=lambda p: False)[2], 1),
        ("T3 W4 无源文注释", lambda: scan_page("docs/en/a.md", fm_ok + "x\n", gloss, "en")[2], 1),
        # ── W5 正文私建术语表（2026-09-20 承接项）：命中和**不命中**两侧都有，防「把判据改宽成恒报」
        ("T3 W5 正文私建术语表（## 术语表）",
         lambda: scan_page("docs/zh/a.md", "## 术语表\n", gloss, "zh")[2], 1),
        ("T3 W5 现成落点 ## 术语 {: #术语 }（改名前的形态）",
         lambda: scan_page("docs/zh/a.md", "## 术语 {: #术语 }\n", gloss, "zh")[2], 1),
        ("T3 W5 带前缀的术语表标题（### 中英术语表）",
         lambda: scan_page("docs/zh/a.md", "### 中英术语表\n", gloss, "zh")[2], 1),
        ("T3 W5 改名后不命中（## 本文概念/字段说明 {: #术语 }）",
         lambda: scan_page("docs/zh/a.md", "## 本文概念/字段说明 {: #术语 }\n", gloss, "zh")[2], 0),
        ("T3 W5 锚点里的「术语表」不算（引用，不是私建）",
         lambda: scan_page("docs/en/a.md", fm_ok + "# Glossary: one master table {: #术语表：一份主表 }\n",
                           gloss, "en", identifiers=False)[2], 0),
        ("T3 W5 围栏代码块里的 ## 术语表 不算（示例）",
         lambda: scan_page("docs/zh/a.md", "```\n## 术语表\n```\n", gloss, "zh")[2], 0),
        ("T3 W5 单条豁免生效（<!-- glossary-allow -->）",
         lambda: scan_page("docs/zh/a.md", "## 术语表 <!-- glossary-allow -->\n", gloss, "zh")[2], 0),
        ("T3 W5 含「术语」但不恰为（## 术语与字段说明）不命中",
         lambda: scan_page("docs/zh/a.md", "## 术语与字段说明\n", gloss, "zh")[2], 0),
        # ── D3① 换根与档位（2026-09-21）：**成对**钉住「面认对」——
        #    左格 = 开发文档面的中文稿文件（`01-设计/x.md`，第二段不是 zh）⇒ 必须按**中文面**判；
        #    右格 = 同一路径在**默认口径**下会被判成英文面（这正是 93 处假红的来源）。
        #    自证框架按 `len(fn())` 计分 ⇒ 两格都回**列表**（真 = 命中 1 条，假 = 空表）。
        ("D3① devdocs：`01-设计/x.md` 按中文面判（禁写中文写法要命中）",
         lambda: [lang_of("01-设计/x.md", "devdocs")] if lang_of("01-设计/x.md", "devdocs") == "zh" else [], 1),
        ("D3① 默认口径：同路径第二段不是 zh ⇒ 英文面（**这是那 93 处假红的来源**）",
         lambda: [1] if lang_of("01-设计/x.md") == "en" else [], 1),
    ]
    assert len(cases) == 29, "自证用例数必须为 29，实际 %d（少跑即失效）" % len(cases)
    ok = True
    print("── check-glossary.py 自证（29 条）")
    for name, fn, want in cases:
        try:
            got = len(fn())
        except Exception as e:                # noqa: BLE001 —— 自证里任何异常都算失败并打印原因
            got = "异常:%s" % e
        good = got == want
        ok = ok and good
        print("  %s %s（期望 %s 实际 %s）" % ("✓" if good else "✗", name, want, got))
    print("  自证：%s" % ("通过 ✓" if ok else "**失败** ✗"))
    return 0 if ok else 1


# ── `Q-176` 第五批：`--self-test` 三档自检（正控 rc=0 · 负控 rc=1 · 用法错 rc=2）──────────
#    两层：① 纯函数 29 例（判据本身的成对正负控）② 子进程**真跑本件**取真退出码。
#    纪律：合成术语表 + 合成页面**全在临时目录**（`ZERG_GLOSSARY_TSV` 指到临时表）⇒ 不读不写仓内件 ·
#          子进程夹具退出即清。`--selftest` 是本旗标的旧写法，两个拼法同一档。
USAGE = ("用法：python3 scripts/gates/check-glossary.py [--lang zh,en] [--paths P …] [--enforce] "
         "[--no-identifiers] [--self-test]")
SELF_PATH = os.path.abspath(__file__)


def self_test():
    """三档自检：只写正名 ⇒ rc=0；中文页出现 `forbid_zh` 词 ⇒ rc=1；未知 `--scope`/`--json` ⇒ rc=2。"""
    import shutil
    import subprocess
    import tempfile
    bad = [0]

    def chk(label, want, rc, out):
        good = (rc == want)
        if not good:
            bad[0] += 1
        tail = "" if good else " ｜ " + (out.strip().splitlines() or [""])[-1][:90]
        print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))

    print("── check-glossary.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
    print("   ＜层① 纯函数 29 例（判据成对正负控 · 不碰任何目录）＞")
    if selftest() != 0:        # ★ 返回值是 rc（0 = 过）⇒ 不能写 `if not selftest()`
        bad[0] += 1
    tmp = tempfile.mkdtemp(prefix="glossary-selftest-")
    try:
        tsv = os.path.join(tmp, "术语表.tsv")
        with open(tsv, "w", encoding="utf-8") as f:
            f.write("\t".join(COLS) + "\n")
            f.write("虫茧\tcocoon\tcocoon\tcocoon\t\t\tMr2109\t\n")
            f.write("虫巢\t\t\t\t虫巢\t\tMr2109\t英文待拍\n")
        docs = os.path.join(tmp, "docs", "zh")
        os.makedirs(docs)
        clean = os.path.join(docs, "clean.md")
        with open(clean, "w", encoding="utf-8") as f:
            f.write("本文只写正名：这里叫虫茧。\n")
        dirty = os.path.join(docs, "dirty.md")
        with open(dirty, "w", encoding="utf-8") as f:
            f.write("旧称：这里叫虫巢。\n")
        env = dict(os.environ, ZERG_GLOSSARY_TSV=tsv)

        def run(*a):
            p = subprocess.run([sys.executable, SELF_PATH, "--root", tmp, "--lang", "zh"] + list(a),
                               capture_output=True, text=True, env=env)
            return p.returncode, p.stdout + p.stderr

        print("   ＜层② 子进程真跑本件（合成根 + 合成主表 · 临时目录）＞")
        rc, out = run("--paths", clean)
        chk("档① 正控：中文页只写正名 ⇒ rc=0", 0, rc, out)

        rc, out = run("--paths", dirty)
        chk("档② 负控：中文页出现 `forbid_zh` 词（虫巢）⇒ T2 非阻塞 rc=1", 1, rc, out)

        rc, out = run("--scope", "no-such-scope", "--paths", clean)
        chk("档③ 用法错：未知 `--scope` ⇒ rc=2", 2, rc, out)

        rc, out = run("--paths", clean, "--json")
        chk("档③′ 用法错：`--json` 而本门无机器读面（`Q-176` 统一口）", 2, rc, out)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
