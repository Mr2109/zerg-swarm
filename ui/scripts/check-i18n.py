#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""i18n 门禁（多语言决策⑥ 2026-09-11）——CI 与发布导出都会跑，失败即中止。

四道门：
  G1 键对称     zh-CN.yml 与 en.yml 的键集合必须完全一致
  G2 引用完整   代码里静态写死的 t!("…") 键必须都在 yml 里定义
  G3 占位符一致 同一键在两版 yml 的 %{name} 集合必须相同（翻译不许漏掉变量）
  G4 中文门禁   非测试、非注释的含中文 UI 字面量不得超过基线白名单（结构 error / 未使用 warn）

用法：
  python3 ui/scripts/check-i18n.py            # 门禁（退出码 0 通过 / 1 失败）
  python3 ui/scripts/check-i18n.py --list     # 只列出当前剩余的中文字面量（不改基线）
"""
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
CJK = re.compile(r'[\u4e00-\u9fff]')
KEY_LINE = re.compile(r'^([A-Za-z][\w.]*):\s*(.*)$')
# 静态键引用：t!("k") / t!("k", …)；排除 t!(变量) 这类动态键
T_STATIC = re.compile(r'(?<![A-Za-z_])t!\(\s*"([^"]+)"')
# 测试函数里的断言消息不译；语言自称（English/中文）属专名；AI 提示词与数据值按决策保留
BASELINE_FILE = os.path.join(ROOT, "ui", "scripts", "i18n-baseline.json")


def docs_out_root():
    """「项目文档」取源根（**仓内优先、缺则仓外** —— 与 2026-09-19「开发文档分家」同口径）。

    契约（与 tools/kb_docs_sync.py:32-34 · docs/site/export-and-build.sh:48 · ui/scripts/i18n_audit.py 一致）：
      ① 仓内 <repo>/docs/项目文档 在盘上 ⇒ 用它（未分家的机器 / 公开树行为一字不变）；
      ② 缺 ⇒ 仓外 <ZERG_DOCS_ALT|../Zerg-内部文档>/项目文档（分家后本机的真身）；
      ③ **两处都缺 ⇒ 返回 (None, 缺件说明)** —— 缺件不静默：调用方 rc=2 不给结论，
         **绝不在仓内 makedirs 造目录**（原写法写死 <repo>/docs/项目文档/… ⇒ 分家后一跑
         就把空目录造回工作树，让「取源根还在仓内」这个错误结论看起来成立）。
    """
    r = ROOT
    alt = os.environ.get("ZERG_DOCS_ALT") or os.path.join(os.path.dirname(r), "Zerg-内部文档")
    cands = [os.path.join(r, "docs", "项目文档"), os.path.join(alt, "项目文档")]
    for p in cands:
        if os.path.isdir(p):
            return p, None
    return None, "项目文档取源根未找到（仓内根 %s ✗ · 仓外根 %s ✗）" % (cands[0], cands[1])


def read_lines(path):
    with open(path, encoding="utf-8") as f:
        return f.read().split("\n")


def parse_yml(path):
    """返回 {键: 值}（本项目 yml 为扁平结构，逐行解析足够）。"""
    out = {}
    for line in read_lines(path):
        m = KEY_LINE.match(line)
        if m:
            out[m.group(1)] = m.group(2).strip()
    return out


def placeholders(value):
    return set(re.findall(r'%\{(\w+)[^}]*\}', value))


def rust_files():
    for dirpath, _, names in os.walk(os.path.join(ROOT, "ui", "src")):
        for n in sorted(names):
            if n.endswith(".rs"):
                yield os.path.join(dirpath, n)


def chinese_literals():
    """非测试、非注释的含中文行 → [(相对路径, 行号, 函数名, 代码)]。"""
    rows = []
    for path in rust_files():
        rel = os.path.relpath(path, ROOT)
        fn, in_test = "?", False
        for i, line in enumerate(read_lines(path), 1):
            m = re.match(r'\s*(?:pub )?fn (\w+)', line)
            if m:
                fn = m.group(1)
            if re.match(r'\s*#\[cfg\(test\)\]', line):
                in_test = True
            code = re.sub(r'//.*$', '', line)
            if not CJK.search(code) or '"' not in code:
                continue
            if in_test or fn.startswith(("test_", "parses", "bench", "sample")):
                continue
            rows.append((rel, i, fn, code.strip()))
    return rows


def load_baseline():
    if os.path.exists(BASELINE_FILE):
        with open(BASELINE_FILE, encoding="utf-8") as f:
            return json.load(f)
    return {"allowed": []}


def main():
    list_only = "--list" in sys.argv
    errors, warns = [], []

    zh = parse_yml(os.path.join(ROOT, "ui", "locales", "zh-CN.yml"))
    en = parse_yml(os.path.join(ROOT, "ui", "locales", "en.yml"))

    # G1 键对称
    only_zh, only_en = sorted(set(zh) - set(en)), sorted(set(en) - set(zh))
    if only_zh:
        errors.append("G1 仅 zh-CN 有（%d）：%s" % (len(only_zh), ", ".join(only_zh[:8])))
    if only_en:
        errors.append("G1 仅 en 有（%d）：%s" % (len(only_en), ", ".join(only_en[:8])))

    # G2 引用未定义
    used = set()
    for path in rust_files():
        for line in read_lines(path):
            used.update(T_STATIC.findall(re.sub(r'//.*$', '', line)))
    missing = sorted(k for k in used if k not in zh or k not in en)
    if missing:
        errors.append("G2 代码引用但未定义（%d）：%s" % (len(missing), ", ".join(missing[:8])))

    # G3 占位符一致
    diff = []
    for k in sorted(set(zh) & set(en)):
        pz, pe = placeholders(zh[k]), placeholders(en[k])
        if pz != pe:
            diff.append("%s（zh=%s en=%s）" % (k, sorted(pz) or "-", sorted(pe) or "-"))
    if diff:
        errors.append("G3 占位符不一致（%d）：%s" % (len(diff), "; ".join(diff[:5])))

    # G4 中文门禁（基线白名单）
    baseline = load_baseline()
    allowed = baseline.get("allowed", [])
    rows = chinese_literals()

    def is_allowed(rel, code):
        for a in allowed:
            if a.get("file") == rel and a.get("contains", "") in code:
                return True
        return False

    extra = [r for r in rows if not is_allowed(r[0], r[3])]
    stale = [
        a for a in allowed
        if not any(r[0] == a.get("file") and a.get("contains", "") in r[3] for r in rows)
    ]

    extra_pre = [r for r in rows if not is_allowed(r[0], r[3])]
    extra = extra_pre

    if list_only:
        print("剩余中文字面量 %d 行（基线允许 %d 行）：" % (len(rows), len(allowed)))
        for rel, i, fn, code in rows:
            mark = "允许" if is_allowed(rel, code) else "★新增"
            print("  [%s] %s:%d %s | %s" % (mark, rel, i, fn, code[:90]))
        return 0

    if "--tsv" in sys.argv:
        # 现状快照（不覆盖 P0 基线——那是历史记录）
        # ★ 2026-09-19「开发文档分家」：输出目录不再写死仓内（原写法一跑就把空目录造回工作树）——
        #   ZERG_I18N_AUDIT_OUT 显式优先 → 缺则 <项目文档取源根>/v2.5.9/i18n-audit；
        #   取源根=仓内优先、缺则仓外；两处都缺 ⇒ rc=2 不给结论（缺件不静默）。
        tsv_dir = os.environ.get("ZERG_I18N_AUDIT_OUT", "").strip()
        if not tsv_dir:
            base, missing = docs_out_root()
            if base is None:
                print("check-i18n: " + (missing or "项目文档取源根未找到"))
                print("⇒ 输出目录不可判（不给结论）· rc=2")
                return 2
            tsv_dir = os.path.join(base, "v2.5.9", "i18n-audit")
        out = os.path.join(tsv_dir, "L2-现状-中文字面量.tsv")
        os.makedirs(os.path.dirname(out), exist_ok=True)
        with open(out, "w", encoding="utf-8") as f:
            f.write("状态\t文件\t行号\t函数\t代码\n")
            for rel, i, fn, code in rows:
                f.write("%s\t%s\t%d\t%s\t%s\n"
                        % ("允许(基线)" if is_allowed(rel, code) else "★未登记", rel, i, fn, code))
        print("写出: " + out + "（%d 行）" % len(rows))
        return 1 if extra else 0

    if extra:
        errors.append("G4 出现未登记的中文 UI 字面量（%d 行，须抽成 i18n 键或登记进基线）：" % len(extra))
        for rel, i, fn, code in extra:
            errors.append("      %s:%d %s | %s" % (rel, i, fn, code[:90]))
    if stale:
        warns.append(
            "G4 基线里有 %d 条已不存在（可收紧基线）：%s"
            % (len(stale), ", ".join("%s|%s" % (a.get("file"), a.get("contains", "")[:24]) for a in stale[:5]))
        )

    # G5 错误码 ↔ UI 键（多语言 L4——服务端 46 个 code 应有对应 apierr.* 键）
    # 强度：warn（未收录的 code 会回退服务端 message——客户端不改即可运行，故不阻断）
    api_dir = os.path.join(ROOT, "core", "internal", "api")
    go_codes = set()
    if os.path.isdir(api_dir):
        for fn in sorted(os.listdir(api_dir)):
            if fn.endswith(".go") and not fn.endswith("_test.go"):
                src = open(os.path.join(api_dir, fn), encoding="utf-8").read()
                go_codes.update(re.findall(r'writeErrorCode\(w,\s*[^,]+,\s*"([A-Z0-9_]+)"', src))
    ui_codes = {k[len("apierr."):].upper() for k in zh if k.startswith("apierr.")}
    missing_codes = sorted(go_codes - ui_codes)
    extra_codes = sorted(ui_codes - go_codes)
    if missing_codes:
        warns.append("G5 服务端错误码缺 UI 本地化键（%d）：%s" % (len(missing_codes), ", ".join(missing_codes[:8])))
    if extra_codes:
        warns.append("G5 UI 有 apierr.* 键但服务端已无该码（%d）：%s" % (len(extra_codes), ", ".join(extra_codes[:8])))

    # 未引用键（提示级）
    # apierr.* 由 UI 动态拼键（format!("apierr.{code}")）→ 静态扫描看不见，不算未引用
    unused = sorted(k for k in zh if k not in used and "[" not in k and not k.startswith("apierr."))
    if unused:
        warns.append("未引用键 %d 个（可能被动态引用 t!(变量)，据此删键前先确认）：%s"
                     % (len(unused), ", ".join(unused[:8])))

    print("=== i18n 门禁 ===")
    print("  键数 zh=%d en=%d ｜ 静态引用 %d ｜ 剩余中文字面量 %d 行（基线 %d）"
          % (len(zh), len(en), len(used), len(rows), len(allowed)))
    for w in warns:
        print("  [warn] " + w)
    for e in errors:
        print("  [ERROR] " + e)
    if errors:
        print("→ 门禁失败（%d 项）" % len(errors))
        return 1
    print("→ 门禁通过")
    return 0


if __name__ == "__main__":
    sys.exit(main())
