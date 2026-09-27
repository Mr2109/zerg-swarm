#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""发布期门禁：扫「私有卷占位符后仍带实质余部」的**形态**（= 规则跨空格失配后的截断残留）。

退出码：0 = 干净；1 = 有残留（不得发布）；2 = 脚本自测未过。
原由：替换规则曾用「以空白为终止」的字符类匹配私有卷前缀 ⇒ 含空格的私有路径只被替掉前半段，
      余部（私有目录名）留在正文里；而彼时前缀已消失，任何「扫私有绝对路径」的检查都看不见它。
      故本门禁按**形态**扫：私有卷占位符 + 紧随其后的路径样余部
      （占位符**自成列**的除外 —— 规则表里它是替换列，属模式文本，不是残留）。
用法: python3 scripts/gates/check-placeholder-residue.py <git目录> [<commit>|--all]

注意：本文件自身**不得**出现真实私有目录字面量或完整占位符形态（否则会自己命中自己）。
"""
import re, subprocess, sys, os

# git grep -n <pat> [<commit>] 输出: [<commit>:]<path>:<lineno>:<content>
LINE_RE = re.compile(r"^(?:([0-9a-f]{7,40}):)?(.*?):(\d+):(.*)$", re.S)
# 余部判据：占位符之后（允许一个空白）紧跟「路径样」内容
PLACEHOLDER = "<volume-" + "path>"          # 拆开写，避免本文件出现完整形态
RESIDUE_RE = re.compile(re.escape(PLACEHOLDER) + r"[ \t]*([^\s\"'`\)]{1,})")
PATHY_RE = re.compile(r"/")   # 余部含斜杠 ⇒ 视为路径样（实测被截断的私有路径余部都带 /；普通中文正文不误报）


def is_definition_field(text, m):
    """占位符**自成列**（前为行首/制表符，后为制表符/行尾）⇒ 定义面。

    现读病灶（假红）：`publish/replace-rules.tsv` 的行是 `pattern<TAB>替换列<TAB>说明` 三列，
    替换列 = 占位符；说明列的中文里带 `/`（`引号/反引号/换行`）⇒ 被判成「路径样余部」。
    与 `selftest()` 档①同一条口径（「规则表定义（模式文本，非残留）」），此处把它**落成判据**。
    为什么这是**收窄**而非放宽：真残留的余部是私有路径被截断后剩下的那段，恒**紧贴**占位符
    （空格或直连的目录名 —— 见档②/档⑤/档⑥），**绝不**以制表符起头 ⇒ 本判据只会放过定义列。
    """
    after = m.start() + len(PLACEHOLDER)
    return ((m.start() == 0 or text[m.start() - 1] == "\t")
            and (after >= len(text) or text[after] == "\t"))


def content_of(line):
    m = LINE_RE.match(line)
    return (m.group(2), m.group(3), m.group(4)) if m else (None, None, None)


def is_residue(text):
    for m in RESIDUE_RE.finditer(text):
        if is_definition_field(text, m):   # 定义面（规则表替换列等）⇒ 模式文本，不是残留
            continue
        if PATHY_RE.search(m.group(1)):
            return True, m.group(0)
    return False, ""


def selftest():
    V = PLACEHOLDER
    cases = [
        ("regex:" + V + r"\t<volume-path>\t说明", False, "规则表定义（模式文本，非残留）"),
        (V + " 中文字目录/示例.json", True, "截断残留（带空格续部）"),
        ("配置见 " + V + " 即可", False, "正常正文：后面是普通词"),
        (V, False, "纯占位符，无余部"),
        (V + "/中文字目录/示例", True, "无空格直连的续部"),
        (V + "中文字目录/示例.md", True, "无空格直连的中文目录（含斜杠，形态成立）"),
        (V + "\t私有卷路径（引号/反引号/换行）", False, "规则表定义列（占位符自成列 · 说明含斜杠）"),
    ]
    bad = 0
    for text, want, why in cases:
        got, frag = is_residue(text)
        ok = (got == want)
        if not ok:
            bad += 1
        print("   %s want=%-5s got=%-5s | %-42s | %s" % ("✓" if ok else "✗", want, got, why, frag[:40]))
    print("   自测: %s" % ("全过" if bad == 0 else "%d 条失败" % bad))
    return bad == 0


def scan(gitdir, all_commits=False, commit=None):
    commits = (subprocess.run(["git", "-C", gitdir, "-c", "core.quotepath=false", "rev-list", "main"], capture_output=True, text=True)
               .stdout.split() if all_commits else [commit or "HEAD"])
    total, files = 0, {}
    for c in commits:
        out = subprocess.run(["git", "-C", gitdir, "-c", "core.quotepath=false", "grep", "-n", "-I", PLACEHOLDER, c, "--", "."],
                             capture_output=True, text=True, encoding="utf-8", errors="replace").stdout
        for ln in out.split("\n"):
            if not ln.strip():
                continue
            path, lineno, text = content_of(ln)
            hit, frag = is_residue(text)
            if hit:
                total += 1
                files.setdefault(path, []).append("%s:%s" % (lineno, frag[:60]))
    return total, files


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


# ── `Q-176` 第五批：`--self-test` 三档自检（正控 rc=0 · 负控 rc=1 · 用法错 rc=2）──────────
#    纪律：**只在临时 git 仓里造件**（不读不写仓内件）· 子进程**真跑本件**取真退出码 · 退出即清。
USAGE = "用法：python3 scripts/gates/check-placeholder-residue.py <git目录> [<commit>|--all] [--self-test]"
SELF_PATH = os.path.abspath(__file__)


def self_test():
    """三档自检：无残留 ⇒ rc=0；有残部（占位符 + 路径样余部）⇒ rc=1；未知旗标 ⇒ rc=2。"""
    import shutil
    import subprocess
    import tempfile
    V = PLACEHOLDER          # 拆开写的形态，本件自己也不出现完整字面量
    tmp = tempfile.mkdtemp(prefix="placeholder-residue-selftest-")
    bad = [0]

    def chk(label, want, rc, out):
        good = (rc == want)
        if not good:
            bad[0] += 1
        tail = "" if good else " ｜ " + (out.strip().splitlines() or [""])[-1][:90]
        print("  %s %s（期望 rc=%d 实际 rc=%d）%s" % ("✓" if good else "✗", label, want, rc, tail))

    def git(d, *a):
        return subprocess.run(["git", "-c", "core.quotepath=false", "-C", d, "-c", "user.email=selftest@invalid",
                               "-c", "user.name=selftest", "-c", "commit.gpgsign=false"] + list(a),
                              capture_output=True, text=True)

    def repo(name, body):
        d = os.path.join(tmp, name)
        os.makedirs(d)
        git(d, "init", "-q")
        with open(os.path.join(d, "说明.md"), "w", encoding="utf-8") as f:
            f.write(body)
        git(d, "add", "-A")
        git(d, "commit", "-q", "-m", "fixture")
        return d

    def run(target, *a):
        p = subprocess.run([sys.executable, SELF_PATH, target] + list(a), capture_output=True, text=True)
        return p.returncode, p.stdout + p.stderr

    try:
        print("── check-placeholder-residue.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
        clean = repo("clean", "路径见 %s 即可（后面是普通词，无余部）\n" % V)
        rc, out = run(clean, "HEAD")
        chk("档① 正控：纯占位符、余部不是路径样 ⇒ 无残留", 0, rc, out)

        dirty = repo("dirty", "配置在 %s 中文字目录/示例.json 里\n" % V)
        rc, out = run(dirty, "HEAD")
        chk("档② 负控：占位符后仍带路径样余部（截断残留）", 1, rc, out)

        case = repo("case", "跑 `%s\\t%s\\t说明`（规则表定义，形态文本非残留）\n" % (V, V))
        rc, out = run(case, "HEAD")
        chk("档①′ 成对正控：规则表定义行 ⇒ **不许红**（防假红）", 0, rc, out)

        desc = repo("desc", "regex:/Volumes/(AHZ|BXC)*\t%s\t私有卷路径（停在引号/反引号/换行）\n" % V)
        rc, out = run(desc, "HEAD")
        chk("档①″ 成对正控：定义列说明**含斜杠** ⇒ 不许红（现读假红的病灶）", 0, rc, out)

        rc, out = run(clean, "--no-such-flag")
        chk("档③ 用法错：未知旗标（输入错 ≠ 零命中）", 2, rc, out)

        rc, out = run("--json")
        chk("档③′ 用法错：`--json` 而本门无机器读面（`Q-176` 统一口）", 2, rc, out)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("── 自检：%s" % ("全过 ✓" if bad[0] == 0 else "**%d 档不过** ✗" % bad[0]))
    return 0 if bad[0] == 0 else 1


if __name__ == "__main__":
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        sys.exit(rc_q176)
    _argv = sys.argv[1:]
    if "--self-test" in _argv:
        sys.exit(self_test())
    _unknown = [a for a in _argv if a.startswith("-") and a != "--all"]
    if _unknown:
        print("✗ 用法错（rc=2）：未知旗标 %s" % " ".join(_unknown), file=sys.stderr)
        print("  %s" % USAGE, file=sys.stderr)
        sys.exit(2)
    if not selftest():
        print("自测未过 ⇒ 脚本本身有问题，拒绝出结论", file=sys.stderr)
        sys.exit(2)
    target = _argv[0] if _argv else "."
    mode = _argv[1] if len(_argv) > 1 else "HEAD"
    total, files = scan(target, all_commits=(mode == "--all"))
    if total == 0:
        print("结果: 通过——未发现残留（扫 %s mode=%s）" % (target, mode))
        sys.exit(0)
    print("结果: 不通过——发现 %d 处残留（%d 个文件）：" % (total, len(files)))
    for f, hits in sorted(files.items(), key=lambda x: -len(x[1]))[:15]:
        print("   %s" % f)
        for h in hits[:3]:
            print("      %s" % h)
    sys.exit(1)
