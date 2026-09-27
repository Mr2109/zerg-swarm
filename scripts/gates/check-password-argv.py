#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-password-argv.py —— 脚本/单元面口径：**口令三面**（明文不入 argv · 档权限与环境入口处置 · 正控）

病象（逐字见 `Zerg-内部文档/项目文档/v2.5.12/缺口总账-20260923.md` 本批条目）：
    「部署四件（`gateway/deploy_x3.sh` · `gateway/deploy_mini.sh` ·
     `gateway/deploy/transfer_mini_light.sh` · `gateway/deploy/transfer_mini_models.sh`）的口令入口」
    原先是**命令行明文**（`--password <明文>`）与 `sshpass -p <值>`（`git show HEAD:` 可读到的改前面），
    外加环境变量 `X3_PASS` / `MINI_PASS` ⇒ 口令**同时进本机 `ps`（同 uid 进程可见）与 shell 历史**。
拍板：口令一律走 —— ① `--password-file <600档>`（argv 上只见**档路径**）· ② `--password-stdin`（值经 stdin）
    · ③ 不填 ⇒ ssh key 免密；裸 `--password <明文>` **拒收 · 退 2**（兼容臂：值若确实指向**已存在的档** ⇒ 按 ① 接）；
    环境变量入口标**已弃用**（仍能跑 + stderr 一次性警告）。
★ **本门为何新开**：现读 `scripts/gates/` 里 `--password` **零命中** ⇒ 这套口径**没有任何门对拍** ✗
    （同族门 `check-token-argv.py` 只管令牌面）⇒ 照旧会漏下一处。本门照它的做法与文件风格写（不创式样）。

**判红档（四档 · 每档都有独立用例与真退出码）**：
    · ① `PW-LITERAL-ARGV`  = 判红面非注释行出现**裸 `--password`/`--passwd`/`--pass`/`--pw` + 实值**
      （`--password <值>` / `--password=<值>`）。「值」若是**已存在的档** ⇒ **兼容臂**（按 `--password-file` 接）
      ⇒ 只登记；否则（字面值**或变量展开** —— 命令行上变量照样展开进 argv）⇒ **判红**。
    · ② `PW-SHORTOPT-ARGV` = 同上，短选项形：`-p <值>` **喂给吃口令的工具**
      （`sshpass`/`ssh`/`scp`/`sftp`/`ssh-copy-id`/`rsync`/`mysql`/`mysqldump`/`psql`/`smbclient`/`htpasswd`）。
      ★ **工具名挡一道**的理由（防误报）：`mkdir -p <目录>` / `cp -p <档>` / `tar -p` 那类同形选项满地都是，
      本门**不**把它们读成口令 ⇒ 首词不在工具表里 ⇒ 不判红。端口（纯数字）与路径（`/` / `~` / `.` 起头）形态
      **只登记**（`PW-MENTION`）。
    · ③ `ENV-NOWARN-ARGV`  = **读了** `X3_PASS`/`MINI_PASS`（`${X3_PASS…}` 形态）但**全文没有**那行已弃用警告
      （同一行同时含「弃用」+ 变量名 + `>&2`）⇒ 判红（弃用档的处置是「仍能跑，但必须警告」⇒ 没警告 = 没处置）。
    · ④ `ENV-TAUGHT-USAGE` = **usage/echo 行仍在教**环境变量入口（提到 `X3_PASS`/`MINI_PASS` 却没有
      `弃用`/`deprecated` 标记）⇒ 判红（**教** = 把已弃用入口写成正门）。
    ★ **判红面 = 脚本面 + 单元面**（`.sh`/`.bash`/`.zsh` · `.service`/`.socket`/`.timer`/`.plist`）；
      其余面（`.md`/`.py`/`.go`/`.json`/`.yml`…）与**注释行**只登记、不判红 —— 与兄弟门
      `check-token-argv.py` / `check-curl-timeout.py` 的「夹具/注释只登记、不判红」同口径（不判 ≠ 不静默）。
      plist/unit 里的 `<!-- … -->`（可跨行）按注释算 ⇒ 不判红。
    · **口令档权限（为何不判红 · 现读四件真实现后定）**：四件都是**运行期**读 argv 给的那只档的 9 位权限串
      （`ls -ld` 取第 1 字段前 9 位、裁掉 macOS 的 `@`/`+`）并要求属组/其他位全 0（`"${_pm:3:6}" == "------"`）
      ⇒「档权限」这个值**在仓外**、由调用者当场给 ⇒ **静态无从判合规** ✗；静态能看的只是「有没有那道校验式样」，
      而**缺校验式样 ≠ 口令进了 argv** ⇒ 只登记（`PASSFILE-PERMCHECK-OK` / `PASSFILE-PERM-NOCHECK`），不判红。
    **登记档（只打印、不判红）** = 注释行 · 非判红面里的同形 · 裸选项提及（usage 里的 `--password <明文>`）·
      兼容臂（值指向已存在档）· 端口/路径形态的 `-p` · `PASSFILE-SITE` · `PASSSTDIN-SITE` · `ENVFILE-SITE`
      （正门形态：`EnvironmentFile=` / `EnvironmentVariables`）· 已弃用 env 的提及。
    **点名登记豁免表**（`EXEMPT_PW` · 精确文件名 + 形状匹配，不是路径前缀）**现读零条**（仓内已清 ⇒ 不需要）
      —— ★ 表长硬钳：**多于 1 条 ⇒ rc=2 不给结论**（防把「摘掉 scope」当治本 ✗）。
    · 缺件 / 扫描根读不到 / 扫到 0 个脚本或单元件 / `--root` 值以 `-` 开头（拼错旗标）⇒
      **rc=2 不给结论**（BLOCKED · 不许当绿 ✗）。

用法：
    python3 scripts/gates/check-password-argv.py [--root <目录>] [--json]
    python3 scripts/gates/check-password-argv.py --self-test
    # 自检四档：逐档负控（四型逐型 rc=1）＋ 正控（三正门 rc=0）＋ 真目标（现读仓内计数）＋ 退码 0-1-2 钳
    #   ＋ **防空绿档**（旧档同形 ⇒ 本门 rc=1 ∧ 兄弟门 check-token-argv.py rc=0）
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
)
EXCLUDE_DIRS = {".git", "node_modules", "__pycache__", "target", "dist", "build", "bin",
                "vendor", "data", "zerg-wt", ".zerg", "_history"}
SH_SUFFIX = (".sh", ".bash", ".zsh")
UNIT_SUFFIX = (".service", ".socket", ".timer", ".plist")     # systemd / launchd 单元面
JUDGED_SUFFIX = SH_SUFFIX + UNIT_SUFFIX                       # 判红面 = 脚本 + 单元
REG_SUFFIX = (".py", ".md", ".go", ".json", ".txt", ".yml", ".yaml")

# ① 裸口令选项（`--password-file` / `--password-stdin` **不算**：后紧跟着 `-` ⇒ 由 `(?![-\w])` 挡住）
PWOPT = r"(?:--password|--passwd|--pass|--pw)"
# 「值」必须是**实值**：排除空、`<占位>`（`--password <明文>` 那种说明行不许误报）、`)`（shell case 臂
#   `--password) X3_PASS="$2"` 不许误报）、逗号（`add_argument("--password", …)` 那种 python 面不许误报）
OPT_PW_JUDGE = re.compile(r"(?<![\w-])" + PWOPT + r"(?![-\w])\s*=?\s*(?P<q>['\"]?)"
                          r"(?P<v>[A-Za-z0-9_./${}~@:%+-][^\s)<>|;&,]*)", re.I)
OPT_MENTION = re.compile(r"(?<![\w-])" + PWOPT + r"(?![-\w])", re.I)
# ② 短选项喂口令：**首词必须是吃口令的工具**（否则 `mkdir -p`/`cp -p`/`tar -p` 满地误报 ✗）
PW_TOOLS = (r"(?:sshpass|ssh-copy-id|ssh|scp|sftp|rsync|mysqladmin|mysqldump|mysql|"
            r"psql|pg_dump|smbclient|htpasswd)")
SHORT_PW = re.compile(r"(?<![\w./-])" + PW_TOOLS + r"\b[^\n|;&]*?(?<![\w-])-p\s*"
                      r"(?P<q>['\"]?)(?P<v>[^\s'\"]+)")
# ③ env 入口（已弃用）：**读**（`${X3_PASS…}` / `$X3_PASS`）· 警告行（弃用 + 变量名 + `>&2`）· 教学行
#   ★ 「读环境」的可判形态（防把**内部变量**读成 env ⇒ pos2 那类误报 ✗）：
#     ① 默认展开形 `${X3_PASS:-…}` / `${X3_PASS-…}` / `${X3_PASS:=…}` —— 现读四件与改前档都是这一形；
#     ② 本件内**从未**给这个名字赋值（`X3_PASS=` / `read -r X3_PASS`）⇒ 裸引用只可能来自环境。
ENV_DEFAULT = re.compile(r"\$\{?(?:EDGE-NODE|MINI)_PASS(?![A-Za-z0-9_])\s*:?[-=]")
ENV_REF = re.compile(r"\$\{?(?:EDGE-NODE|MINI)_PASS(?![A-Za-z0-9_])")
ENV_ASSIGN = re.compile(r"(?:^|[\s;|&(])(?:EDGE-NODE|MINI)_PASS\s*=|read\s[^\n]*(?:EDGE-NODE|MINI)_PASS")
ENV_NAME = re.compile(r"(?:EDGE-NODE|MINI)_PASS(?![A-Za-z0-9_])")
# 引号里的**字面**名字（教学/警告文字）—— `"${X3_PASS}"` 那种**展开**不算（`printf '%s' "${X3_PASS}"` 不许当 usage ✗）
ENV_LITERAL = re.compile(r"(?<![\w${])(?:EDGE-NODE|MINI)_PASS(?![A-Za-z0-9_])")
DEPRECATED_MARK = re.compile(r"弃用|deprecated|不再教", re.I)
USAGE_TEXT = re.compile(r"\b(?:echo|printf)\b")
# 档权限的**校验式样**（运行期）：四件现读都是 `"${_pm:3:6}" == "------"` + `chmod 600` 提示
PERMCHECK_RX = re.compile(r"3:6|权限过宽|rw-------|chmod\s+600")
ENVFILE_RX = re.compile(r"EnvironmentFile\s*=|EnvironmentVariables")
# 点名登记豁免（精确文件名 + 形状）—— 本批现读**零条**（仓内已清 ⇒ 不需要）；**表长 >1 ⇒ rc=2**
EXEMPT_PW = []


def _walk(root):
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]
        for fn in sorted(filenames):
            yield os.path.join(dirpath, fn)


def _xml_comment_lines(text):
    """plist / unit 里的 `<!-- … -->`（可跨行）⇒ 落在其中的**行号**集合（判红面按注释排除 · 防误报）"""
    spans = [(m.start(), m.end()) for m in re.finditer(r"<!--.*?-->", text, re.S)]
    head = text.rfind("<!--")
    if head != -1 and (not spans or head > spans[-1][0]):
        spans.append((head, len(text)))          # 未闭合的 `<!--` ⇒ 直到文末都算注释（宁可不判红）
    if not spans:
        return set()
    out, pos = set(), 0
    for i, ln in enumerate(text.splitlines(), 1):
        for s, e in spans:
            if s <= pos < e:
                out.add(i)
                break
        pos += len(ln) + 1
    return out


def _is_existing_path(v, root):
    """值是否**已存在的档**（兼容臂判据）：纯变量/含 `$` ⇒ 不是档；否则按相对/绝对各试一次"""
    if not v or "$" in v or v.startswith("-"):
        return False
    return os.path.isfile(v) or os.path.isfile(os.path.join(root, v))


def scan(root):
    """回 (judged, registered, n_face)；root 不存在 ⇒ (None, None, None)"""
    if not os.path.isdir(root):
        return None, None, None
    judged, registered, n_face = [], [], 0
    for p in _walk(root):
        rel = os.path.relpath(p, root)
        suf = os.path.splitext(p)[1].lower()
        on_face = suf in JUDGED_SUFFIX
        if on_face:
            n_face += 1
        elif suf not in REG_SUFFIX:
            continue
        try:
            text = open(p, encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        xmlc = _xml_comment_lines(text)
        exempt_shapes = {shape for (f, shape) in EXEMPT_PW if rel == f}
        lines = text.splitlines()
        # 弃用档的处置「仍能跑 + 警告」⇒ 全件级判据：有没有那一行警告（弃用 + 变量名 + stderr）
        env_warn_ok = any(("弃用" in ln or "deprecated" in ln.lower())
                          and ENV_LITERAL.search(ln) and ">&2" in ln for ln in lines)
        # 本件有没有给这个名字**赋值**（有 ⇒ 裸引用是内部变量，不算「读环境」）
        env_assigned = any(ENV_ASSIGN.search(ln) for ln in lines)
        passfile_line = None
        done = set()

        def rec(i, tag, why_soft, soft=False, comment=False):
            """按「在不在判红面 + 是不是注释 + 是不是只登记形态」定红档还是登记档"""
            done.add(i)
            if on_face and i not in xmlc and not comment and not soft:
                judged.append((rel, i, tag))
            else:
                registered.append((rel, i, tag,
                                   "注释行" if (comment or i in xmlc) else why_soft))

        for i, ln in enumerate(lines, 1):
            s = ln.strip()
            comment = s.startswith("#") or s.startswith("//") or i in xmlc
            if comment:
                if ENV_NAME.search(ln) or OPT_MENTION.search(ln) or "--password-file" in ln:
                    rec(i, "PW-MENTION", "注释行", soft=True, comment=True)
                continue
            if passfile_line is None and "--password-file" in ln:
                passfile_line = i
            is_usage = bool(USAGE_TEXT.search(ln))
            hard = None
            m = OPT_PW_JUDGE.search(ln)
            if m:
                v = m.group("v").strip()
                if not ("$" in v) and _is_existing_path(v, root) and v.lower() not in exempt_shapes:
                    # 兼容臂：值确实指向**已存在的档** ⇒ 按 `--password-file` 接（旧调用形态）⇒ 只登记
                    rec(i, "PW-COMPAT-PASSFILE", "值指向已存在档（兼容臂 ⇒ 按 --password-file 接）", soft=True)
                    continue
                hard = "PW-LITERAL-ARGV"
            if hard is None:
                ms = SHORT_PW.search(ln)
                if ms:
                    v = ms.group("v").strip()
                    if re.fullmatch(r"\d+", v) or v.startswith(("/", "~", ".")):
                        rec(i, "PW-MENTION", "端口/路径形态的 `-p`（非口令）", soft=True)
                        continue
                    hard = "PW-SHORTOPT-ARGV"
            env_read = bool(ENV_DEFAULT.search(ln)) or bool(ENV_REF.search(ln) and not env_assigned)
            if hard is None and env_read and not env_warn_ok:
                hard = "ENV-NOWARN-ARGV"
            if hard is None and is_usage and ENV_LITERAL.search(ln) and not DEPRECATED_MARK.search(ln):
                hard = "ENV-TAUGHT-USAGE"
            if hard:
                rec(i, hard, "非判红面")
                continue
            # ── 登记档（只打印、不判红）：正门形态 / 裸提及 / 已弃用提及 ──
            if is_usage and ENV_LITERAL.search(ln) and DEPRECATED_MARK.search(ln):
                rec(i, "ENV-MENTION-DEPRECATED", "已弃用入口的用法文字（带弃用标记 ⇒ 只登记）", soft=True)
            elif ENV_NAME.search(ln):
                rec(i, "ENV-MENTION", "已弃用入口的内部提及/赋值（非「读入」形态）", soft=True)
            elif OPT_MENTION.search(ln):
                rec(i, "PW-MENTION", "非判红面", soft=True)
            elif "--password-file" in ln:
                rec(i, "PASSFILE-SITE", "非判红面", soft=True)
            elif "--password-stdin" in ln:
                rec(i, "PASSSTDIN-SITE", "非判红面", soft=True)
            elif ENVFILE_RX.search(ln):
                rec(i, "ENVFILE-SITE", "非判红面", soft=True)

        # ── 件级登记：收 `--password-file` 的件，**有没有**那道运行期权限校验式样（档权限本身静态判不了 ✗）──
        if passfile_line is not None:
            ok = bool(PERMCHECK_RX.search(text))
            registered.append((rel, passfile_line,
                               "PASSFILE-PERMCHECK-OK" if ok else "PASSFILE-PERM-NOCHECK",
                               "收 --password-file 且见运行期权限校验式样（档权限本身在仓外 · 静态判不了 ⇒ 只登记）"
                               if ok else
                               "收 --password-file 但**未见**运行期权限校验式样 ⇒ 档可能非 600（静态判不了档权限 ⇒ 只登记）"))
    return judged, registered, n_face


def render(root):
    if len(EXEMPT_PW) > 1:
        print("BLOCKED —— 点名登记豁免表长 %d > 1 ⇒ 本门自废（rc=2 · 不许用豁免把门藏起来）"
              % len(EXEMPT_PW), file=sys.stderr)
        return 2, None
    if str(root).startswith("-"):
        print("BLOCKED —— 防呆：`--root` 值不得以 `-` 开头（%r 是拼错的旗标）⇒ 判不了（rc=2 · 不许当绿）"
              % root, file=sys.stderr)
        return 2, None
    judged, registered, n_face = scan(root)
    if judged is None:
        print("BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）" % root, file=sys.stderr)
        return 2, None
    if n_face == 0:
        print("BLOCKED —— 扫描根下 0 个脚本/单元件（%s）⇒ 判不了（rc=2 · 不许当绿）" % root, file=sys.stderr)
        return 2, None
    return (1 if judged else 0), (judged, registered, n_face)


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--root", default=REPO_ROOT)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()

    if a.self_test:
        return self_test()

    rc, data = render(a.root)
    if data is None:
        return rc
    judged, registered, n_face = data
    if a.json:
        print(json.dumps({
            "schema": "zerg/v1", "kind": "PasswordArgvScan", "root": a.root,
            "face_files": n_face, "sh_files": n_face,
            "judged": len(judged), "registered": len(registered),
            "exempt_pw": [f for (f, _) in EXEMPT_PW],
            "judged_sites": [{"file": f, "line": i, "shape": t} for (f, i, t) in judged],
        }, ensure_ascii=False))
        return rc
    print("扫描根：%s" % a.root)
    print("判红面件 %d 个（脚本 + 单元）· **判红 %d 处** · 登记档 %d 处（只登记、不判红）· 点名登记豁免 %d 条"
          % (n_face, len(judged), len(registered), len(EXEMPT_PW)))
    if judged:
        print("⇒ 口令进了命令行实参 / 弃用入口没处置（逐处点名 · 逐档）：")
        for f, i, t in judged:
            print("  ✗ %s:%d  [%s]" % (f, i, t))
        print("⇒ 口径：口令走 **①`--password-file <600档>`·argv 只见档路径 / ②`--password-stdin`·值经 stdin /"
              " ③ 不填 ⇒ ssh key 免密**；裸 `--password <明文>` 拒收退 2；env `X3_PASS`/`MINI_PASS` 已弃用"
              " ⇒ 仍能跑但**必须**打一次性警告（`>&2`）· 用法里不许再教它 ⇒ rc=1")
        return rc
    print("⇒ 判红面**零处**口令进 argv / 弃用入口未处置（四档口径成立）—— rc=0")
    print("★ 登记档照实列出（不判红 · 供复核）：")
    for f, i, t, why in registered:
        print("  [登记/%s] %s:%d  %s" % (why, f, i, t))
    return rc


# 自检夹具：(档, 形态说明, 用例名, 文件名, 正文, 期望判红档)
NEG_CASES = [
    ("①", "明文口令进 argv（单引号实值）", "neg1-literal-password", "probe.sh",
     '#!/usr/bin/env bash\ngateway/deploy_x3.sh --host g01@<cluster-ip> --password \'S3cr3t-Pw!\'\n',
     "PW-LITERAL-ARGV"),
    ("①", "--password=<变量>（命令行上照样展开进 argv）", "neg2-var-password", "probe.sh",
     '#!/usr/bin/env bash\ndeploy_mini.sh --password="$MINI_PW" --host Mr2109@<cluster-ip>\n',
     "PW-LITERAL-ARGV"),
    ("②", "短选项喂口令给吃口令的工具（字面值）", "neg3-sshpass-p-literal", "probe.sh",
     '#!/usr/bin/env bash\nsshpass -p \'Passw0rd!\' ssh root@<cluster-ip> true\n',
     "PW-SHORTOPT-ARGV"),
    ("②", "短选项喂口令（变量 · 旧部署四件同形）", "neg4-sshpass-p-var", "deploy_x3.sh",
     '#!/usr/bin/env bash\nsshpass -p "${X3_PW}" scp -o StrictHostKeyChecking=no "$F" host:/tmp/\n',
     "PW-SHORTOPT-ARGV"),
    ("③", "读了弃用 env（`:-` 默认展开）但全文没有任何警告", "neg5-env-no-warn", "deploy_mini.sh",
     '#!/usr/bin/env bash\nMINI_PASS="${MINI_PASS:-}"\nscp "$F" mini:/tmp/\n',
     "ENV-NOWARN-ARGV"),
    ("④", "usage 仍在教弃用 env 入口", "neg6-env-taught-usage", "probe.sh",
     '#!/usr/bin/env bash\nusage() { echo "  口令入口: export X3_PASS=<口令> 后再跑"; }\n',
     "ENV-TAUGHT-USAGE"),
]

# 正控夹具：(用例名, 文件名, 正文, 期望登记 ≥ N)
POS_CASES = [
    ("pos1-password-file-600", "deploy_x3.sh",
     '#!/usr/bin/env bash\nX3_PASSFILE=""\n'
     'case "$1" in --password-file) X3_PASSFILE="$2"; shift 2 ;; esac\n'
     '_pm="$(ls -ld -- "${X3_PASSFILE}" | awk \'{print $1}\')"\n'
     '[[ "${_pm:3:6}" == "------" ]] || { echo "口档权限过宽" >&2; exit 2; }\n'
     'X3_PASS="$(cat "${X3_PASSFILE}")"\n'
     'SSHPASS="${X3_PASS}" sshpass -e ssh g01@<cluster-ip> true\n', 2),
    ("pos2-password-stdin", "deploy_x3.sh",
     '#!/usr/bin/env bash\nif [[ "${PW_STDIN}" == "1" ]]; then\n'
     '    IFS= read -r X3_PASS || true\n'
     '    [[ -n "${X3_PASS}" ]] || { echo "stdin 空行" >&2; exit 2; }\nfi\n', 1),
    ("pos3-unit-envfile", "edge-node-agent.service",
     '[Service]\nUser=g01\nEnvironmentFile=-/home/<user>/agent/edge-node-agent.env\n'
     'ExecStart=/usr/bin/python3 /home/<user>/agent/agent.py --host 0.0.0.0\n', 1),
    ("pos4-deprecated-env-warned", "deploy_x3.sh",
     '#!/usr/bin/env bash\nX3_PASS_ENV="${X3_PASS:-}"\n'
     'usage() { echo "  已弃用: X3_PASS 环境变量（仍能跑 · 会打一次性警告；文档/用法不再教）"; }\n'
     'if [[ -n "${X3_PASS_ENV}" ]]; then\n'
     '    echo "⚠️  已弃用入口：环境变量 X3_PASS 仍能跑（本批不删），但环境变量对同 uid 进程可见。" >&2\n'
     '    X3_PASS="${X3_PASS_ENV}"\nfi\n', 2),
]

# 防空绿夹具：**改前同形**（旧 usage 教 `--password <明文>` · 调用点照抄）⇒ 本门必判红、兄弟门必绿
OLD_STYLE = ('#!/usr/bin/env bash\n'
             'echo "用法: $0 [--controller http://<主控IP>:8580] [--password <pw>]"\n'
             'gateway/deploy_x3.sh --host g01@<worker-host> --password \'S3cr3t-Pw!\'\n')
SIBLING = os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-token-argv.py")


def _write(d, name, fn, body):
    sub = os.path.join(d, name)
    os.makedirs(sub, exist_ok=True)
    open(os.path.join(sub, fn), "w", encoding="utf-8").write(body)
    return sub


def self_test():
    """四档自检：逐档负控（rc=1）＋ 正控（rc=0）＋ 真目标（现读计数）＋ 退码 0-1-2 钳 ＋ 防空绿档

    自带工作区隔离（只用临时目录 · 仓外夹具）· 负控格逐格贴真退出码与档名。
    """
    d = tempfile.mkdtemp(prefix="pw-argv-selftest-")
    try:
        bad = 0
        print("── 自检（合成夹具 · 临时目录 · 不碰真目标）──")
        print("[逐档负控] 四型各一个用例 ⇒ 每型必须 rc=1 且**恰好判红 1 处**、档名相符")
        for tier, desc, name, fn, body, tag in NEG_CASES:
            sub = _write(d, name, fn, body)
            rc, data = render(sub)
            judged = data[0] if data else []
            got = judged[0][2] if judged else "-"
            ok = rc == 1 and len(judged) == 1 and got == tag
            bad += 0 if ok else 1
            print("  %s %-26s %-28s rc=%d · 判红 %d 处 [%s] · %s"
                  % (tier, desc, name, rc, len(judged), got, "✓" if ok else "✗ 断言不过"))

        print("[正控] 三正门（`--password-file <600档>`+权限校验 / `--password-stdin` / `EnvironmentFile=`）"
              "＋ 已弃用 env 带警告 ⇒ rc=0")
        for name, fn, body, reg_min in POS_CASES:
            sub = _write(d, name, fn, body)
            rc, data = render(sub)
            judged, reg = (data[0], data[1]) if data else ([], [])
            ok = rc == 0 and not judged and len(reg) >= reg_min
            bad += 0 if ok else 1
            print("  %-30s rc=%d · 判红 %d 处 · 登记 %d 处 · %s"
                  % (name, rc, len(judged), len(reg), "✓" if ok else "✗ 断言不过"))

        print("[真目标] 现读仓内（`$ZERG_REPO_ROOT` 或本件推得的仓根）⇒ 结构断言（判红面件 > 0 · rc ∈ {0,1}）"
              "＋ 计数照实报")
        rc_real, data_real = render(REPO_ROOT)
        n_face = data_real[2] if data_real else 0
        n_j = len(data_real[0]) if data_real else -1
        ok = rc_real in (0, 1) and n_face > 0
        bad += 0 if ok else 1
        print("  真目标 %s ⇒ rc=%d · 判红面件 %d · 判红 %d 处 · %s"
              % (REPO_ROOT, rc_real, n_face, n_j, "✓" if ok else "✗ 断言不过"))
        if n_j > 0:
            for f, i, t in data_real[0]:
                print("     ← 真目标现红（不阻塞自检 · 但仓内应清）：%s:%d [%s]" % (f, i, t))

        print("[退码 0-1-2 钳] 缺件 / 空转 / 豁免表长 2 / `--root` 以 `-` 开头 ⇒ 一律 rc=2（判不了 · 不许当绿）")
        rc_missing = render(os.path.join(d, "nope"))[0]
        empty = os.path.join(d, "empty")
        os.makedirs(empty, exist_ok=True)
        open(os.path.join(empty, "a.md"), "w", encoding="utf-8").write("# 只有登记面件\n")
        rc_empty = render(empty)[0]
        EXEMPT_PW.extend([("x/y.sh", "pw"), ("x/z.sh", "pw")])
        rc_exempt = render(empty)[0]
        EXEMPT_PW.clear()
        rc_flag = render("-oops")[0]
        ok = rc_missing == 2 and rc_empty == 2 and rc_exempt == 2 and rc_flag == 2
        bad += 0 if ok else 1
        print("  扫描根不存在 rc=%d · 零脚本/单元件 rc=%d · 豁免表长 2 rc=%d · `--root -oops` rc=%d · %s"
              % (rc_missing, rc_empty, rc_exempt, rc_flag, "✓" if ok else "✗ 断言不过"))

        print("[防空绿档] 旧档同形（改前 usage 教 `--password <明文>` + 同样调用点）⇒ 本门 rc=1，"
              "且**兄弟门 check-token-argv.py 同夹具 rc=0**（= 本批之前这形态**没有任何门对拍**）")
        old = _write(d, "old-style", "deploy_x3.sh", OLD_STYLE)
        rc_own, data_own = render(old)
        rc_sib = None
        if os.path.isfile(SIBLING):
            try:
                r = subprocess.run([sys.executable, SIBLING, "--root", old],
                                   capture_output=True, text=True, timeout=120)
                rc_sib = r.returncode
            except (OSError, subprocess.SubprocessError):
                rc_sib = None
        ok = rc_own == 1 and rc_sib == 0
        bad += 0 if ok else 1
        print("  本门 rc=%d（判红 %d 处 [%s]）· 兄弟门 rc=%s · %s"
              % (rc_own, len(data_own[0]) if data_own else -1,
                 (data_own[0][0][2] if data_own and data_own[0] else "-"),
                 "NA（缺件 ⇒ 不判）" if rc_sib is None else str(rc_sib),
                 "✓" if ok else "✗ 断言不过"))

        print("⇒ 自检 %s" % ("全过（rc=0）" if bad == 0 else "有 %d 条断言不过（rc=1）" % bad))
        return 0 if bad == 0 else 1
    finally:
        shutil.rmtree(d, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
