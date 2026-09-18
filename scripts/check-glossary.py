#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-glossary.py —— 术语表门禁（可执行版 · 三档退码）

设计真源：docs/01-设计/设计-文档体系-v1.0.md
            §0.6 P6 ④（正文出现术语表外的译法 ⇒ 告警，**先非阻塞、收敛后阻塞**，节奏同 §6 的 G-3）
            §5.6②（双语目录 · 译页字段）· §5.6③（导出规则⑥「en 列为空 ⇒ 不得进译稿」）
            §14.3（判据③ 代码标识符零改动）· §2.4（漂移三件套③ 源文注释）
唯一真源：docs/术语表.tsv（本脚本只读它，不另存词表；导出物见 scripts/gen-glossary-exports.py）

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

扫描口径（三条，写死防假红；每条都是被咬过的地方）
  ① **剥 HTML 注释与围栏代码块**后才查 B3（源文注释是规定动作；注释里的中文不是漏译）
  ② 每行可用 `<!-- glossary-allow -->`（可跟 `: 理由`）单条豁免 —— **引述原话 / 文件名 / 旧称取证**是法定例外
     （`设计-子端沙箱化` 用词纪律：仅在引述原话处保留原词）；豁免不隐藏：命中计入「单条豁免」计数
  ③ 词表文件自身与两个脚本在 WHITELIST 里（否则规则文本自己就红）

用法：
  python3 scripts/check-glossary.py [--lang zh,en] [--paths P …] [--enforce] [--no-identifiers] [--selftest]
     --enforce        把 T2 术语档升为阻塞（收敛后的口径）
     --no-identifiers 关掉 B5/W4（大型树上想只看词表命中时用）
退出码：0 = 干净 · 1 = 仅 T2/T3 命中（非阻塞）· 2 = T1 命中（或 --enforce 下 T2 命中）· 3 = 环境/输入异常（不静默空转）
"""
import os
import re
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
MAIN = os.path.join(ROOT, "docs", "术语表.tsv")
COLS = ["zh", "en", "id", "do_not_translate", "forbid_zh", "forbid_en", "owner", "note"]
ALLOW = "<!-- glossary-allow -->"          # 单条豁免标记（后面可跟 `: 理由`）
RE_ALLOW = re.compile(r"<!--\s*glossary-allow\b")
WHITELIST = ("docs/术语表.tsv", "docs/术语表.md", "docs/术语表.jieba.txt", "docs/术语表.forbid.txt",
             "docs/术语表-说明.md", "scripts/check-glossary.py", "scripts/gen-glossary-exports.py")
RE_COMMENT = re.compile(r"<!--.*?-->", re.S)
RE_FENCE = re.compile(r"^```.*?^```", re.S | re.M)
RE_CODE = re.compile(r"`([^`\n]+)`")
RE_FM = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.S)
CJK = re.compile(r"[\u3400-\u9fff]")


def split_multi(v):
    out = []
    for x in v.split("|"):
        x = x.strip()
        if x and x not in out:
            out.append(x)
    return out


def load_glossary(path=MAIN):
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
    exempted += sum(1 for l in lines if RE_ALLOW.search(l))
    return blocking, terms, warnings, exempted


def walk_files(paths, root, langs):
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
                lang = "zh" if rel.split(os.sep)[1:2] == ["zh"] else "en"
                if lang in langs:
                    out.append((rel, os.path.join(dp, f)))
    return out


def main():
    argv = sys.argv[1:]
    if "--selftest" in argv:
        return selftest()
    root = ROOT
    langs, paths, i = set(), [], 0
    while i < len(argv):
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
    if not langs:
        langs = {"zh", "en"}
    if not paths:
        paths = [os.path.join("docs", l) for l in sorted(langs)]
    try:
        gloss = load_glossary()
        files = walk_files(paths, root, langs)
    except EnvironmentError as e:
        print("✗ %s" % e, file=sys.stderr)
        return 3
    if not files:
        print("✗ 扫到 0 个 .md —— 拒绝静默空转（假绿）。检查 --paths 与 docs/zh|docs/en 是否存在",
              file=sys.stderr)
        return 3
    blocking, terms, warnings, exempt = [], [], [], 0
    for rel, ap in files:
        if rel in WHITELIST:
            continue
        try:
            text = open(ap, encoding="utf-8", errors="replace").read()
        except Exception as e:
            print("✗ 读不了 %s：%s" % (ap, e), file=sys.stderr)
            return 3
        lang = "zh" if rel.split(os.sep)[1:2] == ["zh"] else "en"
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
    ]
    assert len(cases) == 19, "自证用例数必须为 19，实际 %d（少跑即失效）" % len(cases)
    ok = True
    print("── check-glossary.py 自证（19 条）")
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


if __name__ == "__main__":
    sys.exit(main())
