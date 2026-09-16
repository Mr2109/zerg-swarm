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

用法：python3 scripts/check-zh-en.py [--selftest]
退出码：0 = 无命中；1 = 有命中；2 = 环境/输入异常（不静默）
"""
import os
import re
import sys

ROOTS = ["core", "agent", "gateway", "scripts", "deploy", "docs/skills"]
SKIP_DIRS = ("node_modules", "target", "vendor", ".git", "dist", "build", ".venv")
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
    "scripts/publish-preflight.sh",
    "scripts/publish-public.sh",
    "scripts/test-publish-parity.sh",
    "docs/skills/code-quality.md",
    "scripts/check-zh-en.py",
}
ROLE_PAT = re.compile(r"""(host\s*:\s*['"]?local\b|Machine\s*:\s*['"]local['"]|MachineSnapshot\(['"]local['"]\))""")
ROLE_EXEMPT_HINT = ("_test.go", "selfupdate", "toolchain")

# 已判定的假红（精确到「文件:行」+ 理由）。仍会在报告中单独列出——豁免不等于隐藏。
KNOWN_FP = {}  # 跨行引号已建模（2026-09-16）⇒ 原 publish-public.sh:420 豁免撤销

RE_TRIPLE = re.compile(r"""(\"\"\"|''')[\s\S]*?\1""")
RE_CMDSUB = re.compile(r"""\$\((?:[^()]|\([^()]*\))*\)""")
RE_HEREDOC_START = re.compile(r"""<<-?\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?""")


def strip_blocks(text):
    """剥掉跨行三引号块（等量换行替换，保住行号）。"""
    return RE_TRIPLE.sub(lambda m: "\n" * m.group(0).count("\n"), text)


def mask_heredocs(text):
    """屏蔽 heredoc 段：`<<[-]?['\"]?TAG['\"]?` 起始行之后、直到只含 TAG 的那一行。

    正文属字符串（非代码）。用空行替换以保住行号。
    行式实现 —— 正则版实测匹配不上真实文件（2026-09-16）。
    """
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


def mask_multiline_quotes(text):
    """屏蔽 shell **跨行引号字符串**的内部（如 git commit -m "…\n…" 的多行提交信息体）。

    逐行状态机看不见跨行引号 ⇒ 把正文当代码（实测 publish-public.sh:420 假红）。
    只有"跨行未闭合"才屏蔽中间行（空行替换保住行号）；行内成对的仍交给 strip_noncode。
    """
    out, quote = [], None
    for l in text.split("\n"):
        if quote:
            idx = l.find(quote)
            if idx < 0:
                out.append("")
                continue
            out.append(" " * (idx + 1) + l[idx + 1:])
            quote = None
            continue
        out.append(l)
        # 注释行**永不**开引号：中文注释里常有撇号（如「批 2'.6」），否则会把后续注释行
        # 也当成跨行字符串吞掉，连 "#" 一起屏蔽 ⇒ 注释正文被误判为代码（实测 build-wall.sh:5）。
        quote = None if l.lstrip().startswith("#") else _open_quote(l)
    return "\n".join(out)


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
                    body = strip_blocks(raw)
                elif ext in (".sh", ".plist", ".conf"):
                    body = mask_multiline_quotes(mask_heredocs(raw))
                else:
                    body = raw
                for i, l in enumerate(body.split("\n"), 1):
                    if p not in WHITELIST:
                        for w, why in FORBID.items():
                            if w in l:
                                hits["禁用词"].append("%s:%d  [%s]  %s" % (p, i, why, l.strip()[:70]))
                    code = strip_noncode(l, ext)
                    if ext in FULLW_EXT:
                        if (p, i) in KNOWN_FP:
                            hits["已登记假红(豁免)"].append("%s:%d  [%s]  %s" % (p, i, KNOWN_FP[(p, i)], l.strip()[:60]))
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

    条数必须为 13 —— 少跑一条即判失败（防"用例被静默截断还打印通过"）。
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
    print("  自证：%s（14 条单行 + 4 条跨行结构）" % ("通过 ✓" if ok else "**失败** ✗"))
    return ok


def main():
    if "--selftest" in sys.argv:
        return 0 if selftest() else 1
    if not os.path.isdir("core") and not os.path.isdir("agent"):
        print("✗ 不在仓库根目录（core/ 与 agent/ 都不存在）——拒绝静默空转", file=sys.stderr)
        return 2
    files, hits = scan()
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
    print("\n合计命中 = %d" % total)
    return 0 if total == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
