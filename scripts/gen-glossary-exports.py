#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""gen-glossary-exports.py —— 从术语表主表生成四处导出（唯一真源 = docs/术语表.tsv）

设计真源：docs/01-设计/设计-文档体系-v1.0.md §5.6③（一份主表 + 四处导出 + 导出规则六条）
依据：docs/调研/调研-文档分块落地-检索与双语.md §2.2

四处导出（本脚本一次生成，全部**派生物、不得手改**）：
  ① docs/术语表.md                              人读对照（生成、只读）
  ② docs/术语表.jieba.txt                       jieba 用户词典（每行 `词语 词频 词性`）
  ③ 同上文件 + Material `plugins.search.jieba_dict_user`  ← 站内搜索词典复用同一份（不另出文件）
  ④ styles/config/vocabularies/Zerg/accept.txt    Vale vocab（accept，每行一条正则）
     styles/config/vocabularies/Zerg/reject.txt    Vale vocab（reject，每行一条正则）
     docs/术语表.forbid.txt                      门禁词表（`词<TAB>理由`，喂 check-zh-en.py 的 FORBID）

导出规则六条（逐条落在此脚本里）：
  ① 主表**保序**（导出顺序 = 表内顺序，便于 diff）
  ② 去重 + **禁空行**
  ③ 导出物**不得手改**（jieba / Vale 都不吃注释 ⇒ 用独立 README + 门禁代替抬头）
  ④ 生成后跑 `--check`（等价 `git diff --exit-code`）：与盘上不一致 ⇒ rc=1
  ⑤ **条数断言**（生成的行数与主表行数逐条对齐，缺一条即 rc≠0）
  ⑥ `en` 列为空 ⇒ 该词条**不得进译稿**（本脚本把它单列出来并计数；拦截在 check-glossary.py）

用法：
  python3 scripts/gen-glossary-exports.py            # 生成/更新四处导出
  python3 scripts/gen-glossary-exports.py --check    # 只比对，不写盘（门禁用；有漂移 rc=1）
  python3 scripts/gen-glossary-exports.py --selftest # 自证解析与生成规则（不读仓）

退出码：0 = 一致/已生成；1 = 漂移（--check）；2 = 环境/输入异常（不静默）
"""
import os
import re
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
MAIN = os.path.join(ROOT, "docs", "术语表.tsv")
OUT = {
    "md": os.path.join(ROOT, "docs", "术语表.md"),
    "jieba": os.path.join(ROOT, "docs", "术语表.jieba.txt"),
    "forbid": os.path.join(ROOT, "docs", "术语表.forbid.txt"),
    "accept": os.path.join(ROOT, "styles", "config", "vocabularies", "Zerg", "accept.txt"),
    "reject": os.path.join(ROOT, "styles", "config", "vocabularies", "Zerg", "reject.txt"),
}
COLS = ["zh", "en", "id", "do_not_translate", "forbid_zh", "forbid_en", "owner", "note"]
DEFAULT_FREQ = 10000          # jieba 词频：显式给比默认高的值，让术语优先成词（--freq 可调）
DEFAULT_TAG = "nz"            # jieba 词性：专名（其余取值由主表 owner/note 决定，不猜）

# Vale vocab 只收「能当正则用」的短条目：全 ASCII 词字符 + . _ - / 或纯 CJK
RE_OK_ASCII = re.compile(r"^[A-Za-z0-9_.\-/]+$")
RE_OK_CJK = re.compile(r"^[\u3400-\u9fff\u3000-\u303f]+$")


def read_main(path=MAIN):
    """读主表：表头校验 + 字段数校验 + 禁空行/空 zh + 去重（保序）。"""
    if not os.path.isfile(path):
        raise EnvironmentError("主表不存在：%s" % path)
    raw = open(path, encoding="utf-8").read().split("\n")
    rows, header = [], None
    for i, line in enumerate(raw, 1):
        if i == 1:
            header = line.rstrip("\r").split("\t")
            if header != COLS:
                raise EnvironmentError("表头不符：期望 %s，实际 %s" % (COLS, header))
            continue
        if line.strip() == "":
            continue                       # 规则②：禁空行（末尾换行不算行）
        f = line.rstrip("\r").split("\t")
        if len(f) != len(COLS):
            raise EnvironmentError("第 %d 行字段数 = %d（应为 %d）：%s" % (i, len(f), len(COLS), line[:60]))
        if not f[0].strip():
            raise EnvironmentError("第 %d 行 zh 为空" % i)
        rows.append(dict(zip(COLS, f)))
    if not rows:
        raise EnvironmentError("主表零数据行——拒绝静默空转（假绿）")
    seen, uniq = set(), []
    for r in rows:
        if r["zh"] in seen:                # 规则②：去重（保序）
            continue
        seen.add(r["zh"])
        uniq.append(r)
    return uniq


def split_multi(v):
    """forbid_zh / forbid_en 用 `|` 分隔多值 ⇒ 保序去重。"""
    out = []
    for x in v.split("|"):
        x = x.strip()
        if x and x not in out:
            out.append(x)
    return out


def gen_jieba(rows, freq=DEFAULT_FREQ):
    """jieba 用户词典：每行 `词语 词频 词性`（jieba 官方 README：三部分，后两部分可省略）。"""
    lines = []
    for r in rows:
        w = r["zh"].strip()
        if not w or " " in w or "\t" in w:   # 空行/含空白的条目不许混进词表
            continue
        lines.append("%s %d %s" % (w, freq, DEFAULT_TAG))
    return lines


def gen_forbid(rows):
    """门禁词表：`词<TAB>理由`。理由 = 该词条的正名（可判定、不含歧义）。"""
    lines = []
    for r in rows:
        for w in split_multi(r["forbid_zh"]) + split_multi(r["forbid_en"]):
            lines.append("%s\t术语统一：只有「%s」（docs/术语表.tsv）" % (w, r["zh"]))
    out, seen = [], set()
    for l in lines:
        if l.split("\t")[0] not in seen:
            seen.add(l.split("\t")[0])
            out.append(l)
    return out


def gen_vale(rows):
    """Vale vocab：accept = 正名（中文 + 英文 + 标识符）；reject = 禁用写法（逐条正则）。"""
    accept, reject = [], []
    def push(bag, v):
        v = v.strip()
        if not v or v in bag:
            return
        if RE_OK_ASCII.match(v) or RE_OK_CJK.match(v):
            bag.append(v)
        # 含其它正则元字符的条目（如 `{: #slug }`）**不收**——免得把非法正则写进词表
    for r in rows:
        push(accept, r["zh"])
        push(accept, r["en"])
        push(accept, r["do_not_translate"])
        push(accept, r["id"])
        for w in split_multi(r["forbid_zh"]) + split_multi(r["forbid_en"]):
            push(reject, w)
    return accept, reject


def gen_md(rows, empty_en, n_jieba, n_forbid, n_accept, n_reject):
    """人读对照（生成、只读）。

    抬头两段：① frontmatter（6 件套 —— 与 docs/site/frontmatter-schema.json 对齐，字段一个不多）
    ② 「请勿手改」注释。frontmatter 必须在文件**第一行**，所以生成顺序不能颠倒。
    """
    L = []
    L.append("---")
    L.append("title: 术语表（生成物 · 只读）")
    L.append("type: reference")
    L.append("status: draft")
    L.append("source_of_truth: docs/术语表.tsv")
    L.append("owner: Mr2109")
    L.append("updated_at: %s" % today())
    L.append("ai_assisted: ai")
    L.append("summary: 由 docs/术语表.tsv 生成的人读对照表；改字请改主表后重跑生成器。")
    L.append("---")
    L.append("")
    L.append("<!-- 本文件由 scripts/gen-glossary-exports.py 生成 —— 请勿手改 ✗")
    L.append("     唯一真源 = docs/术语表.tsv（改主表后重跑生成器）；门禁：生成后 --check 必须 rc=0 -->")
    L.append("")
    L.append("# 术语表（生成物 · 只读）")
    L.append("")
    L.append("> 唯一真源：`docs/术语表.tsv`（人读主表 + 四处导出，见 `docs/术语表-说明.md`）。")
    L.append("> 本页是**派生物**：改字请改主表，然后重跑 `python3 scripts/gen-glossary-exports.py`。")
    L.append("")
    L.append("条数：主表 %d 条 · jieba 词表 %d 行 · 门禁词表 %d 条 · Vale accept %d 条 / reject %d 条"
             % (len(rows), n_jieba, n_forbid, n_accept, n_reject))
    L.append("")
    L.append("| zh | en | id | do_not_translate | forbid_zh | forbid_en | owner | note |")
    L.append("|---|---|---|---|---|---|---|---|")
    for r in rows:
        L.append("| %s |" % " | ".join(r[c] for c in COLS))
    L.append("")
    L.append("## `en` 列为空的词条（**不得进译稿**，共 %d 条）" % len(empty_en))
    L.append("")
    for r in empty_en:
        L.append("- **%s**（id: %s）—— %s" % (r["zh"], r["id"] or "—", r["note"]))
    L.append("")
    return "\n".join(L) + "\n"


def today():
    """生成物 frontmatter 的 `updated_at` = **主表最后修改日**（不是「跑生成器的当天」）。

    为什么不用当天：生成物要能被 `--check` 逐字节比对（等价 `git diff --exit-code`）；
    写当天会让它**每天**报漂移。绑主表 mtime 的语义也对：主表动了，生成物才是真的更新了。
    """
    import datetime
    return datetime.date.fromtimestamp(os.path.getmtime(MAIN)).isoformat()


def build():
    rows = read_main()
    empty_en = [r for r in rows if not r["en"].strip()]
    jieba = gen_jieba(rows)
    forbid = gen_forbid(rows)
    accept, reject = gen_vale(rows)
    md = gen_md(rows, empty_en, len(jieba), len(forbid), len(accept), len(reject))
    files = {
        OUT["md"]: md,
        OUT["jieba"]: "".join(l + "\n" for l in jieba),
        OUT["forbid"]: "".join(l + "\n" for l in forbid),
        OUT["accept"]: "".join(l + "\n" for l in accept),
        OUT["reject"]: "".join(l + "\n" for l in reject),
    }
    # 规则⑤：条数断言（生成物行数 == 主表条数 或 与之逐条对齐）
    assert len(jieba) <= len(rows), "jieba 行数 > 主表条数"
    # 规则②：禁空行（人读 .md 是散文，允许空行；三份机器词表一律不许）
    for name in ("jieba", "forbid", "accept", "reject"):
        for i, l in enumerate(files[OUT[name]].split("\n")[:-1], 1):
            assert l.strip() != "", "导出物 %s 第 %d 行为空（规则②）" % (name, i)
    stats = dict(rows=len(rows), empty_en=len(empty_en), jieba=len(jieba),
                 forbid=len(forbid), accept=len(accept), reject=len(reject))
    assert stats["rows"] > 0 and stats["jieba"] > 0, "条数断言失败：主表或词表为空"
    return files, stats


def main():
    argv = sys.argv[1:]
    if "--selftest" in argv:
        return selftest()
    try:
        files, st = build()
    except (EnvironmentError, AssertionError) as e:
        print("✗ 术语表导出失败：%s" % e, file=sys.stderr)
        return 2
    check = "--check" in argv
    drift = []
    for p, txt in files.items():
        old = open(p, encoding="utf-8").read() if os.path.isfile(p) else None
        if old is None:
            drift.append("%s（缺文件）" % p)
        elif old != txt:
            drift.append("%s（内容不一致）" % p)
    print("主表 %d 条（en 空 %d 条 = 不得进译稿）· jieba %d 行 · 门禁词表 %d 条 · Vale accept %d / reject %d"
          % (st["rows"], st["empty_en"], st["jieba"], st["forbid"], st["accept"], st["reject"]))
    if check:
        if drift:
            print("✗ 导出物与主表不一致（改主表后须重跑生成器）：", file=sys.stderr)
            for d in drift:
                print("   %s" % d, file=sys.stderr)
            return 1
        print("✓ 四处导出与主表一致（等价 git diff --exit-code）")
        return 0
    if drift:
        for d in drift:
            print("   写 %s" % d.split("（")[0])
    for p, txt in files.items():
        d = os.path.dirname(p)
        if d and not os.path.isdir(d):
            os.makedirs(d)
        with open(p, "w", encoding="utf-8") as f:
            f.write(txt)
    print("✓ 已写 5 个文件（术语表.md / .jieba.txt / .forbid.txt / Zerg/accept.txt / Zerg/reject.txt）")
    print("  提示：生成物不得手改；手改即被本脚本 --check 抓出（规则③④）")
    return 0


def selftest():
    """自证：断言用例条数（防静默截断），逐条校验规则②⑤与正则过滤。"""
    print("── gen-glossary-exports.py 自证")
    ok = True
    cases = [
        ("split_multi 保序去重", split_multi("a|b|a"), ["a", "b"]),
        ("split_multi 去空白", split_multi(" a | b "), ["a", "b"]),
        ("split_multi 空串", split_multi(""), []),
        ("Vale 收 ASCII 词", bool(RE_OK_ASCII.match("jieba_dict_user")), True),
        ("Vale 收 CJK 词", bool(RE_OK_CJK.match("虫茧")), True),
        ("Vale 拒正则元字符", bool(RE_OK_ASCII.match("{: #slug }")), False),
        ("jieba 行 = 词语 词频 词性",
         gen_jieba([dict(zip(COLS, ["虫茧", "cocoon", "cocoon", "cocoon", "", "", "Mr2109", ""]))])[0].count(" "), 2),
        ("jieba 跳空 zh",
         gen_jieba([dict(zip(COLS, ["", "x", "", "", "", "", "", ""]))]), []),
        ("门禁词表用 TAB 分列",
         gen_forbid([dict(zip(COLS, ["虫茧", "cocoon", "", "", "虫巢", "pod", "Mr2109", ""]))])[1].split("\t")[0], "pod"),
        ("门禁词表带正名理由",
         "只有「虫茧」" in gen_forbid([dict(zip(COLS, ["虫茧", "cocoon", "", "", "虫巢", "", "Mr2109", ""]))])[0], True),
    ]
    assert len(cases) == 10, "自证用例数必须为 10，实际 %d（少跑即失效）" % len(cases)
    for name, got, want in cases:
        good = got == want
        ok = ok and good
        print("  %s %s" % ("✓" if good else "✗", name))
    print("  自证：%s（10 条）" % ("通过 ✓" if ok else "**失败** ✗"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
