#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""发布期门禁：扫「私有卷占位符后仍带实质余部」的**形态**（= 规则跨空格失配后的截断残留）。

退出码：0 = 干净；1 = 有残留（不得发布）；2 = 脚本自测未过。
原由：替换规则曾用「以空白为终止」的字符类匹配私有卷前缀 ⇒ 含空格的私有路径只被替掉前半段，
      余部（私有目录名）留在正文里；而彼时前缀已消失，任何「扫私有绝对路径」的检查都看不见它。
      故本门禁按**形态**扫：私有卷占位符 + 紧随其后的路径样余部。
用法: python3 scripts/check-placeholder-residue.py <git目录> [<commit>|--all]

注意：本文件自身**不得**出现真实私有目录字面量或完整占位符形态（否则会自己命中自己）。
"""
import re, subprocess, sys, os

# git grep -n <pat> [<commit>] 输出: [<commit>:]<path>:<lineno>:<content>
LINE_RE = re.compile(r"^(?:([0-9a-f]{7,40}):)?(.*?):(\d+):(.*)$", re.S)
# 余部判据：占位符之后（允许一个空白）紧跟「路径样」内容
PLACEHOLDER = "<volume-" + "path>"          # 拆开写，避免本文件出现完整形态
RESIDUE_RE = re.compile(re.escape(PLACEHOLDER) + r"[ \t]*([^\s\"'`\)]{1,})")
PATHY_RE = re.compile(r"/")   # 余部含斜杠 ⇒ 视为路径样（实测被截断的私有路径余部都带 /；普通中文正文不误报）


def content_of(line):
    m = LINE_RE.match(line)
    return (m.group(2), m.group(3), m.group(4)) if m else (None, None, None)


def is_residue(text):
    for m in RESIDUE_RE.finditer(text):
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
    commits = (subprocess.run(["git", "-C", gitdir, "rev-list", "main"], capture_output=True, text=True)
               .stdout.split() if all_commits else [commit or "HEAD"])
    total, files = 0, {}
    for c in commits:
        out = subprocess.run(["git", "-C", gitdir, "grep", "-n", "-I", PLACEHOLDER, c, "--", "."],
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


if __name__ == "__main__":
    if not selftest():
        print("自测未过 ⇒ 脚本本身有问题，拒绝出结论", file=sys.stderr)
        sys.exit(2)
    target = sys.argv[1] if len(sys.argv) > 1 else "."
    mode = sys.argv[2] if len(sys.argv) > 2 else "HEAD"
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
