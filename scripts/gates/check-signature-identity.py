#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-signature-identity.py —— 「换件档的产出件必须是**稳定身份签名**，不是 Go linker 的 ad-hoc 件」那一格

一句话：**读被测件的签名档三格**（`Identifier` / `flags` / `Signature`），
`Identifier` 不许是 `a.out`、`flags` 不许含 `adhoc` —— 过不了就判红。

为什么存在（缺口 `Q-210` · 脚本面 · ★★★）
  `scripts/build/build-all.sh` 的**命令面 zerg 段**（留档 + `go build -o "$OUT/zerg"`）此前位于
  `if [ "$ONLY_CORE" = "1" ]` **之前** ⇒ `--only-core`（换主控档）与 `--only-compat` **会顺手重编
  `bin/zerg` 却不重签** —— 产出件只剩 Go linker 的 ad-hoc 签名（实读 `Identifier=a.out` ·
  `flags=0x20002(adhoc,linker-signed)` · `Signature=adhoc`）。
  那不是「报红」：那是 **macOS TCC 的本地网络授权随 cdhash 静默失效**（本仓 `G-05` 那一档，
  症状与 `--only-cli` 当年那次逐字同源）。脚本面的根因已同批修（两条「只出一件」收工支上移到
  命令面段之前 ⇒ 该两档**一个字都不碰** `bin/zerg`）；**这一格是它的牙**：
  脚本面修好后如果谁再把它改回去（或谁绕过脚本手搓 `go build` 覆盖 `bin/`），
  换出来的件当场判红 —— 本门之前 `grep -rln 'adhoc|linker-signed' scripts/gates/` 现读 **0 命中**
  （这就是「无门盯」的实据：那种静默失效此前没有任何门看得见）。

判据（**三格** · 逐件独立判）
  G1 `Identifier` ≠ `a.out`                —— Go linker ad-hoc 签名的固定形态
  G2 `flags` 不含 `adhoc`                  —— `0x2(adhoc)`（`codesign -s -`）与
                                              `0x20002(adhoc,linker-signed)`（linker）都算
  G3 `Signature` ≠ `adhoc`                 —— `codesign -dvvv` 给出该行时判（不给该行 = 无视这一格）
  ★ 三格都不看**签名证书是不是本机的稳定身份**（那要看钥匙串，别的机器上必判不了）——
    本门只判「**是不是那个会静默失效的 ad-hoc 形态**」：可判、与机器无关、退码确定。

被判件（默认）
  `bin/zerg` · `bin/zerg-core` —— 正是 `Q-210` 点名的两件（命令面换件档误伤的那件 + 换主控档的靶件）。
  另可用 `--piece <路径>`（可重复）追加/替换被测集。**不扫 `bin/` 全目录**：
  件集写死，免得「谁往 `bin/` 扔了个第三方件」把这一格判红（本门判的是**本仓换件档的产出件**）。

退码（三档 · 与仓内 `check-*` 同口径）
  0 = 全绿（每件三格都过）
  1 = **有红**（至少一件三格之一不过，或该件**根本没有签名**）
  2 = **不给结论**（`codesign` 不可用 / 被测件不在 / 空转 / 自检不过）
  ★ 件**不在** ⇒ **2（缺件）**、件在但**未签** ⇒ **1（红）** —— 两件事不许混成同一个码。

用法
    python3 scripts/gates/check-signature-identity.py                 # 自检 + 判真目标两件
    python3 scripts/gates/check-signature-identity.py --json          # 机读
    python3 scripts/gates/check-signature-identity.py --list-rules    # 判据三格 + 退码 + 夹具
    python3 scripts/gates/check-signature-identity.py --self-test     # 只跑自检（合成夹具，不碰真目标）
    python3 scripts/gates/check-signature-identity.py --no-self-test  # 内部调试用（门禁别用）
    python3 scripts/gates/check-signature-identity.py --piece bin/zerg --piece /tmp/adhoc
    python3 scripts/gates/check-signature-identity.py --root <仓根>

自检（成对负控 · 4 格 · 全在 `/tmp` 合成夹具里，**不碰**真目标）
  ① ad-hoc 件（`cp /bin/echo` + `codesign -s - --force`）⇒ **必判红**（G2 命中）
  ② 原样苹果签名的件（`/bin/echo`，`flags=0x0(none)`）⇒ **必判绿**（证明门不是恒红）
  ③ 未签件（`cp /bin/echo` + `codesign --remove-signature`）⇒ **必判红**
  ④ 不存在的路径 ⇒ **必 rc=2**（缺件不给结论）
  ★ ②③ 是**成对半边**：只改「签 / 不签」这一个变量 ⇒ 一绿一红，门才叫有区分度。

纪律：**只读**（除 `/tmp` 夹具外不写盘）· 无三方依赖 · 不联网 · 不起后台进程 · 不改任何已有文件。
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT_DEFAULT = os.path.dirname(os.path.dirname(HERE))

EXIT_OK, EXIT_FAIL, EXIT_BLOCKED = 0, 1, 2

DEFAULT_PIECES = ("bin/zerg", "bin/zerg-core")
BAD_IDENTIFIER = "a.out"
BAD_FLAGS_TOKEN = "adhoc"

CODE_DIR_RE = re.compile(r"^CodeDirectory .*?\bflags=0x([0-9a-fA-F]+)\(([^)]*)\)")

RULES = [
    ("G1", "`Identifier` 不得为 `a.out`（Go linker ad-hoc 形态）"),
    ("G2", "`flags` 不得含 `adhoc`（`0x2(adhoc)` / `0x20002(adhoc,linker-signed)`）"),
    ("G3", "`Signature` 不得为 `adhoc`（该行不出现时这一格不判）"),
]


def read_dvvv(path):
    """`codesign -dvvv <件>` → (字段 dict, 报错串)。**不接管道**（rc 由子进程真退出码取）。"""
    try:
        p = subprocess.run(["codesign", "-dvvv", path], capture_output=True, text=True)
    except FileNotFoundError:
        return None, "`codesign` 不在（本机无 codesign ⇒ 判不了）"
    out = (p.stdout or "") + (p.stderr or "")
    fields = {}
    for line in out.splitlines():
        s = line.strip()
        m = CODE_DIR_RE.match(s)
        if m:
            fields["flags"] = "0x%s(%s)" % (m.group(1), m.group(2))
            fields["flags_raw"] = m.group(2)
            continue
        for key in ("Identifier", "Signature", "CDHash", "Authority", "Executable"):
            if s.startswith(key + "="):
                fields[key.lower()] = s.split("=", 1)[1].strip()
                break
    if p.returncode != 0 and "not signed at all" in out:
        return {"unsigned": True, "raw": out.strip().splitlines()[:1]}, ""
    if p.returncode != 0:
        return None, "`codesign -dvvv` 退码 %d：%s" % (p.returncode, out.strip()[:200])
    return fields, ""


def judge(fields):
    """三格判决 → (pass?, 逐条理由)。"""
    bad = []
    ident = fields.get("identifier", "")
    if ident == BAD_IDENTIFIER:
        bad.append("G1：`Identifier=%s`（Go linker 的 ad-hoc 形态 ⇒ TCC 本地网络授权随 cdhash 静默失效）"
                   % ident)
    flags = fields.get("flags", "")
    if BAD_FLAGS_TOKEN in flags.lower():
        bad.append("G2：`flags=%s` 含 `adhoc`（该件不是稳定身份签名）" % flags)
    if fields.get("signature", "").lower() == BAD_FLAGS_TOKEN:
        bad.append("G3：`Signature=adhoc`")
    return (not bad), bad


def check_piece(root, rel):
    """判一件 → (verdict, 行 dict)；verdict ∈ ok / 红 / blocked。"""
    p = rel if os.path.isabs(rel) else os.path.join(root, rel)
    row = {"piece": rel, "path": p}
    if not os.path.isfile(p):
        row["reason"] = "被测件不在（缺件 ⇒ 不给结论）"
        return "blocked", row
    fields, err = read_dvvv(p)
    if err:
        row["reason"] = err
        return "blocked", row
    if fields.get("unsigned"):
        row["identifier"], row["flags"] = "", ""
        row["reason"] = "该件**没有签名**（`code object is not signed at all`）"
        return "red", row
    ok, bad = judge(fields)
    row["identifier"] = fields.get("identifier", "")
    row["flags"] = fields.get("flags", "")
    row["signature"] = fields.get("signature", "")
    row["cdhash"] = fields.get("cdhash", "")
    if ok:
        row["reason"] = "三格全过"
        return "ok", row
    row["reason"] = " · ".join(bad)
    return "red", row


# ── 自检（成对负控 · 全在 /tmp 夹具里）─────────────────────────────────────────────
def self_test(verbose=True):
    """→ (断言数, 失败列表)。夹具用完即清。"""
    n, fails = 0, []
    def assert_eq(name, got, want):
        nonlocal n
        n += 1
        if got != want:
            fails.append("%s：得 %r · 期望 %r" % (name, got, want))
            if verbose:
                print("  ✗ %s：得 %r · 期望 %r" % (name, got, want))
        elif verbose:
            print("  ✓ %s" % name)

    if not shutil.which("codesign"):
        print("  ⛔ 自检跑不了：本机无 `codesign`")
        return 0, ["`codesign` 不在 ⇒ 自检不可判"]
    d = tempfile.mkdtemp(prefix="zerg-sigident-")
    try:
        f1 = os.path.join(d, "adhoc")
        #  ★ 不用 `shutil.copy2`：苹果系统件的 st_flags 复制会被 `chflags` 拒（`Operation not permitted`
        #    —— `copy2` 连元数据一起搬）⇒ 只要**字节**，签名在 Mach-O 里、复制字节就带过来。
        shutil.copy("/bin/echo", f1)
        p = subprocess.run(["codesign", "-s", "-", "--force", f1], capture_output=True, text=True)
        assert_eq("夹具① 造成 ad-hoc（`codesign -s - --force`）rc", p.returncode, 0)
        v, row = check_piece("/", f1)
        assert_eq("夹具① ad-hoc 件 ⇒ **必判红**（负控半边）", v, "red")
        assert_eq("夹具① 红理由落在 G2（flags 含 adhoc）",
                  any(b.startswith("G2") for b in row.get("reason", "").split(" · ")), True)

        f2 = "/bin/echo"
        v2, row2 = check_piece("/", f2)
        assert_eq("夹具② 原样苹果签名件 ⇒ **必判绿**（正控半边）", v2, "ok")

        f3 = os.path.join(d, "unsigned")
        shutil.copy("/bin/echo", f3)
        subprocess.run(["codesign", "--remove-signature", f3], capture_output=True, text=True)
        v3, _ = check_piece("/", f3)
        assert_eq("夹具③ 未签件 ⇒ **必判红**（与② 只差「签 / 不签」一个变量）", v3, "red")

        v4, _ = check_piece("/", os.path.join(d, "不存在这件"))
        assert_eq("夹具④ 件不在 ⇒ **必 rc=2（不给结论）**，不许与「未签」混成同一个码", v4, "blocked")

        # G1 那一格单独证（真造 `Identifier=a.out` 要 Go linker；此处用合成字段面证判据本身）
        n += 1
        ok, bad = judge({"identifier": BAD_IDENTIFIER, "flags": "0x20002(adhoc,linker-signed)",
                         "signature": "adhoc"})
        if ok or len(bad) != 3:
            fails.append("G1/G2/G3 合成字段面 ⇒ 期望三格全红，得 %r / %r" % (ok, bad))
        elif verbose:
            print("  ✓ G1+G2+G3 合成字段面（`a.out` + linker-signed + `Signature=adhoc`）⇒ 三格全红")
    finally:
        shutil.rmtree(d, ignore_errors=True)
    return n, fails


def main():
    ap = argparse.ArgumentParser(add_help=True,
                                 description="签名档三格门（Identifier / flags / Signature · 缺口 Q-210 的牙）")
    ap.add_argument("--root", default=None, help="仓根（默认 = 本脚本上一级的上一级）")
    ap.add_argument("--piece", action="append", default=[], metavar="路径",
                    help="被测件（仓内相对路径或绝对路径 · 可重复）；不给 ⇒ 默认 bin/zerg · bin/zerg-core")
    ap.add_argument("--json", action="store_true", help="机读输出")
    ap.add_argument("--list-rules", action="store_true", help="只列判据三格 + 退码口径 + 夹具")
    ap.add_argument("--self-test", action="store_true", help="只跑自检（合成夹具，不碰真目标）")
    ap.add_argument("--no-self-test", action="store_true", help="跳过自检（内部调试用；门禁别用）")
    a = ap.parse_args()

    if a.list_rules:
        print("判据（三格 · 逐件独立判）：")
        for k, v in RULES:
            print("  %s %s" % (k, v))
        print("被测件（默认）：%s" % " · ".join(DEFAULT_PIECES))
        print("退码：0 全绿 · 1 有红（三格之一不过 / 未签）· 2 不给结论（缺件 / 无 codesign / 自检不过）")
        print("自检：① ad-hoc 件必红 ② 苹果签名件必绿 ③ 未签件必红 ④ 缺件必 2")
        return EXIT_OK

    root = os.path.abspath(a.root) if a.root else REPO_ROOT_DEFAULT

    if not a.no_self_test:
        if not a.json:
            print("自检（成对负控 · 合成夹具 · 不碰真目标）：")
        n, fails = self_test(verbose=not a.json)
        if fails:
            print("⛔ 不给结论：自检未过（%d 条失败）⇒ 先修门自己" % len(fails))
            for f in fails:
                print("  ✗ " + f)
            return EXIT_BLOCKED
        if a.self_test:
            print("自检结论: 全过（断言 %d 条 · 失败 0 条）" % n)
            return EXIT_OK
        if not a.json:
            print("自检结论: 全过（断言 %d 条 · 失败 0 条）\n" % n)
    elif a.self_test:
        print("⛔ --self-test 与 --no-self-test 同给 ⇒ 用法错")
        return EXIT_BLOCKED

    if not shutil.which("codesign"):
        print("⛔ 不给结论（rc=2）：本机无 `codesign` ⇒ 签名档读不了（门侧口径：缺语料不给结论，不许当绿）")
        return EXIT_BLOCKED

    rels = a.piece or list(DEFAULT_PIECES)
    rows, reds, blockeds, oks = [], [], [], []
    for rel in rels:
        v, row = check_piece(root, rel)
        row["verdict"] = v
        rows.append(row)
        (oks if v == "ok" else reds if v == "red" else blockeds).append(row)

    if a.json:
        print(json.dumps({"root": root, "rows": rows, "ok": len(oks), "red": len(reds),
                          "blocked": len(blockeds)}, ensure_ascii=False, indent=2))
    else:
        print("签名档三格门 · 仓根 %s" % root)
        print("  被测件 %d 件：%s" % (len(rels), " · ".join(rels)))
        for row in rows:
            mark = {"ok": "✓", "red": "✗", "blocked": "⛔"}[row["verdict"]]
            print("  %s %-16s Identifier=%s · flags=%s · Signature=%s"
                  % (mark, row["piece"], row.get("identifier", "—"), row.get("flags", "—"),
                     row.get("signature", "—")))
            if row.get("cdhash"):
                print("       CDHash=%s" % row["cdhash"])
            print("       %s" % row["reason"])
        print("  ⇒ 绿 %d · 红 %d · 不给结论 %d" % (len(oks), len(reds), len(blockeds)))

    if reds:
        print("⇒ 判红（rc=1）：上面带 ✗ 的件不是稳定身份签名（ad-hoc = TCC 本地网络授权会随 cdhash 静默失效）")
        return EXIT_FAIL
    if blockeds:
        print("⇒ 不给结论（rc=2）：上面带 ⛔ 的件读不到（缺件 / 工具不在）")
        return EXIT_BLOCKED
    print("⇒ 全过（rc=0）：被测件都不是 ad-hoc 件（`Identifier` ≠ `a.out` · `flags` 不含 `adhoc`）")
    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())
