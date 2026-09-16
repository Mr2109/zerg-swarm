#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-zh-en.py —— 中英文与术语检查（code-quality §8 的可执行版）

设计要点（2026-09-16 两轮假红的教训）：
  ① 检查对象按文件类型分流：
     - 「全角标点」只扫**没有编译器兜底**的文件（.sh/.yaml/.json/.toml/.py/.plist）
       —— .go 里全角标点编译即报错，扫它只会制造假红（第一轮 5,725 处假红）
     - 「术语/旧角色词」对 .go 也要扫（它跟编译器无关，只有人眼会滑过）
  ② 剥注释/字符串**必须跨行**：Python 三引号文档字符串整段是字符串，
     只处理单行引号会把它当代码（第二轮的 312 处假红）——故先按整文剥三引号块
     （用等量换行替换，保住行号）
  ③ 白名单：防泄漏词表（发布拦截脚本）与规则文本本身必须能写出禁用词
  ④ 零输入即报错 —— 绝不静默空转（假绿）
  ⑤ 自带自证 —— 样本取自**真实文件形态**（含三引号跨行），该抓的抓、不该抓的不抓

用法：python3 scripts/check-zh-en.py [--selftest]
退出码：0 = 无命中；1 = 有命中；2 = 环境/输入异常（不静默）
"""
import os
import re
import sys

ROOTS = ["core", "agent", "gateway", "scripts", "deploy", "docs/skills"]
SKIP_DIRS = ("node_modules", "target", "vendor", ".git", "dist", "build", ".venv")
# 全角标点检查范围（无编译器兜底者）
FULLW_EXT = (".sh", ".yaml", ".yml", ".json", ".toml", ".py", ".plist", ".conf")
# 术语/旧角色词检查范围（含 .go）
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
ROLE_PAT = re.compile(r"(host\s*:\s*['\"]?local\b|Machine\s*:\s*['\"]local['\"]|MachineSnapshot\(['\"]local['\"]\))")
ROLE_EXEMPT_HINT = ("_test.go", "selfupdate", "toolchain")

RE_TRIPLE = re.compile(r'(\"\"\"|\'\'\')(?:.|\n)*?\1', re.S)


def strip_blocks(text):
    """剥掉跨行三引号块（等量换行替换，保住行号）。分号/嵌套不追求完备。"""
    return RE_TRIPLE.sub(lambda m: "\n" * m.group(0).count("\n"), text)


def strip_noncode(line, ext):
    """剥掉单行注释与单行字符串，返回「代码区」子串。"""
    out, i, n, quote = [], 0, len(line), None
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
        if line.startswith("//", i) or (ext in (".sh", ".py", ".yaml", ".yml", ".toml", ".conf", ".plist") and c == "#"):
            break
        out.append(c)
        i += 1
    return "".join(out)


def scan():
    files = 0
    hits = {"禁用词": [], "全角标点(代码区)": [], "旧角色词": []}
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
                body = strip_blocks(raw) if ext == ".py" else raw
                for i, l in enumerate(body.split("\n"), 1):
                    if p not in WHITELIST:
                        for w, why in FORBID.items():
                            if w in l:
                                hits["禁用词"].append("%s:%d  [%s]  %s" % (p, i, why, l.strip()[:70]))
                    code = strip_noncode(l, ext)
                    if ext in FULLW_EXT:
                        for ch in FULLW:
                            if ch in code:
                                hits["全角标点(代码区)"].append("%s:%d  全角「%s」  %s" % (p, i, ch, l.strip()[:70]))
                                break
                    m = ROLE_PAT.search(code)
                    if m and not any(h in p for h in ROLE_EXEMPT_HINT):
                        hits["旧角色词"].append("%s:%d  %s  %s" % (p, i, m.group(1), l.strip()[:70]))
    return files, hits


def selftest():
    """校验器先自证：样本取自真实文件形态（含三引号跨行）。"""
    ok = True
    # ① 整文剥三引号：文档字符串里的全角不该抓
    py_src = '"""\n模块说明（含全角括号）\n"""\nx = 1，2\ny = "，"\n'
    stripped = strip_blocks(py_src).split("\n")
    doc_hit = any(ch in strip_noncode(l, ".py") for l in stripped[:2] for ch in FULLW)
    code_hit = any(ch in strip_noncode(l, ".py") for ch in FULLW for l in stripped)
    print("  %s 三引号段落（第 1~2 行）不抓：期望 False 实际 %s" % ("✓" if not doc_hit else "✗", doc_hit))
    ok = ok and not doc_hit
    print("  %s 三引号之后仍抓真代码（x = 1，2）：期望 True 实际 %s" % ("✓" if code_hit else "✗", code_hit))
    ok = ok and code_hit
    print("  %s 字符串内全角不抓（y = \"，\"）：期望 False 实际 %s" % ("✓" if not any("，" in strip_noncode(l, ".py") for l in stripped[4:]) else "✗", any("，" in strip_noncode(l, ".py") for l in stripped[4:])))
    ok = ok and not any("，" in strip_noncode(l, ".py") for l in stripped[4:])
    # ② 单行：注释/字符串不抓、真代码抓
    for ext, line, want in [
        (".sh", "x=1  # 注释里的全角逗号，不该抓", False),
        (".sh", "echo '字符串里的全角逗号，不该抓'", False),
        (".sh", "if [ $a = 1，]; then", True),
        (".yaml", "cmd: run  --flag，x", True),
        (".yaml", "# 注释，不抓", False),
        (".py", "s = '，'  # 字符串，不抓", False),
        (".py", "x = 1，2", True),
        (".go", 'if s == "，" {', False),
    ]:
        got = any(ch in strip_noncode(line, ext) for ch in FULLW)
        flag = "✓" if got == want else "✗"
        ok = ok and got == want
        print("  %s [%s] 期望命中=%s 实际=%s  « %s" % (flag, ext, want, got, line))
    print("  自证：%s" % ("通过 ✓" if ok else "**失败** ✗"))
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
    total = sum(len(v) for v in hits.values())
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
