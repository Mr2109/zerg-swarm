#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
i18n 审计工具（P0-2 文案口径分拣 + P0-3 键健康度审计）

设计稿：docs/01-设计/设计-多语言开源版-20260911.md（§2.4 口径分拣 / §4.2 G1-G3 门禁 / P3 键审计）

用法：
    python3 ui/scripts/i18n_audit.py classify   # P0-2：分拣 CJK 字面量 → TSV 清单
    python3 ui/scripts/i18n_audit.py keys       # P0-3：46 键健康度（未定义/未使用/一键多义）
    python3 ui/scripts/i18n_audit.py all        # 两者都跑

输出目录：<repo>/docs/项目文档/v2.5.9/i18n-audit/
"""
import os
import re
import sys
import glob
import collections

CJK = re.compile(r'[\u4e00-\u9fff]')
# 判定"用户可见 UI 文案"的 egui 调用点特征
UI_CALL = re.compile(
    r'\b(?:text|label|heading|Button::new|button|title|tooltip|on_hover_text|'
    r'RichText::new|SelectableLabel::new|TextEdit::singleline|TextEdit::multiline|'
    r'ComboBox::from_label|CollapsingHeader::new|Window::new|menu_button|checkbox)\s*\('
)
LOG_CALL = re.compile(r'\b(?:println!|eprintln!|log::|tracing::|dbg!)')
ERR_CALL = re.compile(r'\b(?:panic!|unreachable!|todo!|expect\(|unwrap_or_else\(|Err\()')
T_MACRO = re.compile(r't!\(')


def repo_root(start):
    d = os.path.abspath(start)
    while d != "/":
        if os.path.isdir(os.path.join(d, "ui")) and os.path.isdir(os.path.join(d, "core")):
            return d
        d = os.path.dirname(d)
    return os.path.abspath(start)


def rust_files(root):
    out = []
    for p in glob.glob(os.path.join(root, "ui", "src", "**", "*.rs"), recursive=True):
        if "/target/" in p:
            continue
        out.append(p)
    return sorted(out)


def literals_in_line(line):
    """返回该行所有双引号字符串字面量（简化：不处理转义引号内嵌）"""
    return re.findall(r'"((?:[^"\\]|\\.)*)"', line)


def classify(root):
    rows = []
    for f in rust_files(root):
        rel = os.path.relpath(f, root)
        for i, line in enumerate(open(f, encoding="utf-8", errors="replace"), 1):
            stripped = line.strip()
            if not CJK.search(line):
                continue
            if stripped.startswith("//") or stripped.startswith("*") or stripped.startswith("/*"):
                rows.append((rel, i, "注释", "(comment)", stripped[:100]))
                continue
            for lit in literals_in_line(line):
                if not CJK.search(lit):
                    continue
                if T_MACRO.search(line) and lit == "":  # 保守：t! 的键不算法文案
                    continue
                # 非 UI 的排除项（顺序即优先级）
                if LOG_CALL.search(line):
                    cls = "日志"
                elif ERR_CALL.search(line):
                    cls = "错误/panic"
                elif re.match(r'^\s*/', lit) or re.search(r'^https?://', lit):
                    cls = "接口路径/URL"
                elif re.fullmatch(r'[\s{}:.<>\d%\-]*', lit):            # 纯格式符号串（注意：不可含 \w，Python 的 \w 匹配中文）
                    cls = "格式串"
                elif re.search(r'\.(?:ttf|ttc|png|svg|json|md|db|log|so|dylib)$', lit, re.I):
                    cls = "资产路径"
                elif re.search(r'\b(?:const|static)\b', line) or re.search(r'Keywords|\[\s*$', line):
                    cls = "数据/常量"
                elif UI_CALL.search(line) or T_MACRO.search(line):
                    cls = "UI文案"
                else:
                    cls = "UI文案(待确认)"
                rows.append((rel, i, cls, lit[:120], stripped[:100]))
    return rows


def keys_audit(root):
    loc = os.path.join(root, "ui", "locales")
    defs = {}
    for name in ("zh-CN", "en"):
        p = os.path.join(loc, name + ".yml")
        ks = set()
        for line in open(p, encoding="utf-8"):
            m = re.match(r"^([A-Za-z_][\w]*):\s*", line)
            if m:
                ks.add(m.group(1))
        defs[name] = ks
    used = collections.defaultdict(list)   # key -> [(file, line, ctx)]
    # 前置边界：避免误匹配 format!() / writeln!() 尾部的 "t!("
    tlit = re.compile(r'(?<![A-Za-z_])t!\(\s*"((?:[^"\\]|\\.)*)"')
    for f in rust_files(root):
        rel = os.path.relpath(f, root)
        for i, line in enumerate(open(f, encoding="utf-8", errors="replace"), 1):
            for k in tlit.findall(line):
                used[k].append((rel, i, line.strip()[:90]))
    return defs, used


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else "all"
    root = repo_root(os.path.dirname(os.path.abspath(__file__)))
    outdir = os.path.join(root, "docs", "项目文档", "v2.5.9", "i18n-audit")
    os.makedirs(outdir, exist_ok=True)
    print("repo =", root)

    if mode in ("classify", "all"):
        rows = classify(root)
        p = os.path.join(outdir, "P0-2-文案分拣.tsv")
        with open(p, "w", encoding="utf-8") as fh:
            fh.write("文件\t行\t分类\t字面量\t该行代码\n")
            for r in rows:
                fh.write("\t".join(str(x).replace("\t", " ") for x in r) + "\n")
        cnt = collections.Counter(r[2] for r in rows)
        UI_LIKE = {"UI文案", "UI文案(待确认)"}
        per_file_ui = collections.Counter(r[0] for r in rows if r[2] in UI_LIKE)
        print("\n=== P0-2 分拣结果（%d 条含 CJK 行）===" % len(rows))
        for k, v in cnt.most_common():
            print("   %-12s %5d" % (k, v))
        print("   --- UI 文案候选 top 文件（含待确认）---")
        for k, v in per_file_ui.most_common(10):
            print("   %-45s %4d" % (k, v))
        print("   写出:", os.path.relpath(p, root))

    if mode in ("keys", "all"):
        defs, used = keys_audit(root)
        zh, en = defs["zh-CN"], defs["en"]
        p = os.path.join(outdir, "P0-3-键审计.md")
        lines = ["# P0-3 i18n 键健康度审计", "",
                 "| 检查 | 结果 |", "|---|---|",
                 "| zh-CN 键数 | %d |" % len(zh),
                 "| en 键数 | %d |" % len(en),
                 "| 仅 zh 有 | %s |" % (sorted(zh - en) or "无"),
                 "| 仅 en 有 | %s |" % (sorted(en - zh) or "无"),
                 "| 代码引用但未定义 | %s |" % (sorted(set(used) - zh) or "无"),
                 "| 定义了但未被引用 | %s |" % (sorted(zh - set(used)) or "无"), ""]
        lines += ["## 复用点（疑似一键多义；同一键跨文件/跨语义使用）", "",
                  "| 键 | 引用次数 | 涉及文件数 | 引用点 |", "|---|---|---|---|"]
        multi = 0
        for k in sorted(used, key=lambda x: -len(used[x])):
            sites = used[k]
            files = {s[0] for s in sites}
            if len(sites) > 1:
                multi += 1
                lines.append("| `%s` | %d | %d | %s |" % (
                    k, len(sites), len(files),
                    "<br>".join("%s:%d" % (s[0], s[1]) for s in sites[:6])))
        lines += ["", "> 判读：同一键出现在**语义不同**的位置即为一键多义（GPUI 官方明令禁止）。"
                      "同一文件内相邻行的重复调用通常正常（列表逐行渲染）。", ""]
        open(p, "w", encoding="utf-8").write("\n".join(lines))
        print("\n=== P0-3 键审计 ===")
        print("   zh=%d en=%d 仅zh=%s 仅en=%s" % (len(zh), len(en), sorted(zh - en) or "无", sorted(en - zh) or "无"))
        print("   引用未定义:", sorted(set(used) - zh) or "无")
        print("   定义未引用(%d):" % len(zh - set(used)), sorted(zh - set(used)))
        print("   多次引用的键: %d 个（详见报告）" % multi)
        print("   写出:", os.path.relpath(p, root))


if __name__ == "__main__":
    main()
