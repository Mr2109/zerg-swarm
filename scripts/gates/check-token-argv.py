#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-token-argv.py —— 脚本/单元面口径：**令牌不进命令行实参**（缺口 Q-207 · Mr2109 2026-09-26 拍板）

病象（逐字见 `Zerg-内部文档/项目文档/v2.5.12/缺口总账-20260923.md` §附录四十五 表内 `Q-207`）：
    「`curl` 的 `X-Auth-Token` 是**命令行实参** ⇒ 在目标机**进程表里瞬时可见**」
    ⇒ 现读本仓**同形者不止一处**（换件/升级链上一串 `curl` 探针）⇒ 下一处照样会漏。
拍板：**令牌一律不走实参** —— 值经 **stdin**（`curl -H @-`）交给 curl；命令行上既不许出现
    令牌的**字面值**，也不许出现**变量展开**。

**判红档（五档 · 每档都有独立用例与真退出码）**：
    · ① `HDR-ARGV`      = 判红面非注释行出现「`-H`/`--header`/`--header=` 紧跟令牌头名
      （`X-Auth-Token` / `Authorization` / `x-api-key`）」—— 「令牌进 argv」的本形。
    · ② `QS-ARGV`       = 同面 URL **查询串**带令牌（`?token=` / `?access_token=` / `?api_key=` 非空值）。
    · ③ `UNIT-OPT-ARGV` = **单元面的「选项名 + 值」**：`ExecStart=… --token <值>` / `--token=<值>`
      （`--auth-token` / `--access-token` / `--api-key` / `--secret-key` 同族）。
      这就是 `/home/<user>/agent/edge-node-agent.service` 那类真漏点（20 位令牌进单元正文 + argv）的**同形**，
      也包括**卵里生成单元的 heredoc**（`deploy_x3.sh` 那种 `.sh` 内的单元正文 —— 同样在判红面）。
    · ④ `PLIST-OPT-ARGV`= **launchd plist 面的「选项名 + 值」**：`<string>--token</string>` 紧邻
      `<string><值></string>`（同行或邻行），以及单串形态 `<string>--token=<值></string>`。
    · ⑤ `PW-SUDO-ARGV`  = **口令经 argv 喂给提权工具**：`sudo -S` / `sudo --stdin` 与**同一条命令行**上的
      喂入口（`echo '<值>' |` / `printf '<值>' |` / `sudo … <<< '<值>'`）⇒ 口令照进进程表。
      同族免密面**两支同判红**：`sshpass -p <字面值>`（`PW-SSHPASS-ARGV`）与 `sshpass -p <纯变量>`
      （`PW-SSHPASS-VAR` —— 变量在命令行上照样展开进 argv、进程表照看见值；仓内该形已实体清零 ⇒ 由
      「只登记」提为**判红**，棘轮转到完全阻断）。
    ★ **判红面 = 脚本面 + 单元面**（`.sh`/`.bash`/`.zsh` · `.service`/`.socket`/`.timer`/`.plist`）；
      其余面（`.md`/`.py`/`.go`/`.json`/`.yml`…）与**注释行**只登记、不判红 ——
      与兄弟门 `check-curl-timeout.py` 的「夹具/注释只登记、不判红」同口径（不判 ≠ 不静默）。
      plist/unit 里的 `<!-- … -->`（可跨行）**按注释算** ⇒ 不判红（单元正文里那些「禁式:」说明行不许误报 ✗）。
    **点名登记豁免**只许有一条（`EXEMPT_QS`，精确文件名 + 形状匹配，不是路径前缀）——
      现读唯一一条在 `publish/mirror-cn-init.sh`（本批**禁改 `publish/`** ⇒ 只点名、不动 ✗）。
      ★ 豁免表长度硬钳：**多于 1 条 ⇒ rc=2 不给结论**（防把「摘掉 scope」当治本 ✗）。
    **登记档（只打印、不判红）** = 注释行 · 非判红面（`.md`/`.go`/`.py` 夹具串）里的同形出现 ·
      裸选项提及（`--token <占位>` / 本件自己的 CLI 收 `--token`，如 `--token) TOKEN="$2"`）。
      ★ 旧「`sshpass -p <变量>` 只登记」一条**已作废**：该形态本批已提为判红档 `PW-SSHPASS-VAR`，
      登记档自此**不含口令形态** ✗。
    · 缺件 / 扫描根读不到 / 扫到 0 个脚本或单元件 ⇒ **rc=2 不给结论**（BLOCKED · 不许当绿 ✗）。

用法：
    python3 scripts/gates/check-token-argv.py [--root <目录>] [--json]
    python3 scripts/gates/check-token-argv.py --self-test     # 正控（干净件 ⇒ rc=0）+ 逐型负控（五型逐型 rc=1）

★ 环境旋钮登记（本件现读的**这一枚** · 只登记、不改行为）—— `ZERG_REPO_ROOT` = **脚本侧仓根**：
    谁读 / 读什么：`REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(...)`（本模块
    常量区那行）—— 设了就**无条件采信**（不校验是不是真仓根、不回落）；**不设** ⇒ 按本件位置上溯三级推得。
    它唯一的用处是给 `--root` 定默认值（`--root` 不给 ⇒ 扫 `REPO_ROOT`）；给了 `--root` ⇒ 本旋钮被绕开。
    ★ 与 CLI 侧**同名族** `ZERG_REPO`（`core/cmd/zerg/root.go` 件头登记的那一枚）是**两枚、互不读取**：
    脚本面**不认识** `ZERG_REPO`、CLI 面**不认识** `ZERG_REPO_ROOT` ⇒ 要用哪一面就在**哪一面显式设**。
    为何要设 / 正确用法：只用来把**默认扫描根**指到仓外的合成假仓根（负控用法）或另一份仓副本；
    日常跑门**不要设**（设错 ⇒ 扫的是别处；扫到 0 个脚本/单元件 ⇒ rc=2 不给结论，不当绿 ✓）。
"""
import argparse
import json
import os
import re
import shutil
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

# ① 令牌头名（大小写不敏感）：`-H` 后紧跟令牌头名那一族
HDR_SITE = re.compile(r'''(-H|--header)\s*=?\s*["']?\s*(X-Auth-Token|Authorization|x-api-key)\s*:''', re.I)
# ② URL 查询串带令牌
QS_SITE = re.compile(r'''[?&](token|access_token|api_key)=([^"'\s&]*)''')
# ③ 单元面「选项名 + 值」：`--token <值>` / `--token=<值>`（`--token-stdin` 这类**不算**：后面还有 `-`）
TOKOPT = r"(?:--(?:auth-|access-)?token|--api-key|--secret-key)"
# 「值」必须是**实值**：排除空、`<占位>`、`)`（shell case 臂 `--token) TOKEN="$2"` 不许误报）、
#   逗号（`add_argument("--token", …)` 那种 python 面不许误报）
VALUE = r"['\"]?[A-Za-z0-9_./${}~@:%+-][^\s)<>|;&,]*"
OPT_JUDGE = re.compile(r"(?<![\w-])" + TOKOPT + r"(?![\w-])\s*=?\s*" + VALUE, re.I)
OPT_MENTION = re.compile(r"(?<![\w-])" + TOKOPT + r"(?![\w-])", re.I)
# ④ launchd plist：`<string>--token</string>` + 取值（邻行/同行）· 或单串 `<string>--token=<值></string>`
PLIST_NEAR = re.compile(r"<string>\s*" + TOKOPT + r"(?![\w-])\s*</string>\s*<string>\s*([^<>\s][^<>]*?)\s*</string>",
                        re.I | re.S)
PLIST_INLINE = re.compile(r"<string>\s*" + TOKOPT + r"(?![\w-])\s*=\s*([^<>\s][^<>]*?)\s*</string>", re.I | re.S)
# ⑤ 口令经 argv：提权面（`sudo -S` / `--stdin` + 同命令行喂入口）· 免密面（`sshpass -p <值>`）
SUDO_OPT = r"(?:-[A-Za-z]*S[A-Za-z]*|--stdin)"
PW_PIPE_TO_SUDO = re.compile(
    r"\b(?:echo|printf)\s+(?:-{1,2}[^\s]+\s+)*(?P<q>['\"])(?P<v>[^'\"]+)(?P=q)\s*\|\s*"
    r"(?:[A-Za-z_][\w.-]*\s+)*sudo\b[^\n]*?(?<![\w-])" + SUDO_OPT + r"(?![\w-])")
PW_HERE_TO_SUDO = re.compile(
    r"(?<![\w-])sudo\b[^\n|]*?(?<![\w-])" + SUDO_OPT + r"(?![\w-])[^\n]*?<<<\s*(?P<q>['\"])(?P<v>[^'\"]+)(?P=q)")
SSHPASS_P = re.compile(r"(?<![\w-])sshpass\b[^\n|]*?(?<![\w-])-p\s*(?P<q>['\"]?)(?P<v>[^\s'\"]+)(?P=q)")
# 「纯变量引用」（`$PASS` / `${X3_PASS}` / `$1`）—— 命令行上照样展开进 argv ⇒ 与字面值**同判红**
PURE_VAR = re.compile(r"\$(?:\{[A-Za-z_][A-Za-z0-9_]*\}|[A-Za-z_][A-Za-z0-9_]*|[0-9]+)")
# 点名登记豁免（精确文件名 + 形状）——本批禁改 `publish/`，故只点名、不判红；**表长 >1 ⇒ rc=2**
EXEMPT_QS = [("publish/mirror-cn-init.sh", "access_token")]


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


def _line_of(text, pos):
    return text.count("\n", 0, pos) + 1


def _qs_hit(ln, exempt_shapes):
    for m in QS_SITE.finditer(ln):
        if m.group(2) and m.group(1).lower() not in exempt_shapes:
            return True
    return False


def _pw_hit(ln):
    """口令经 argv 的三种喂法 ⇒ 档名；不是这一族 ⇒ None（三支**一律判红**）

    提权面（`sudo -S`/`--stdin` + 同命令行喂入口）：**变量展开与字面值同判红**（Q-207 口径）。
    免密面（`sshpass -p <值>`）：字面值判红（`PW-SSHPASS-ARGV`）；**纯变量引用同样判红**
    （`PW-SSHPASS-VAR` —— 变量在命令行上照样展开进 argv；仓内已实体清零 ⇒ 棘轮转完全阻断）。
    """
    if PW_PIPE_TO_SUDO.search(ln) or PW_HERE_TO_SUDO.search(ln):
        return "PW-SUDO-ARGV"
    m = SSHPASS_P.search(ln)
    if m:
        if PURE_VAR.fullmatch(m.group("v").strip()):
            return "PW-SSHPASS-VAR"
        return "PW-SSHPASS-ARGV"
    return None


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
        exempt_shapes = {shape for (f, shape) in EXEMPT_QS if rel == f}
        done = set()

        def rec(i, tag, why_soft, soft=False, comment=False):
            """按「在不在判红面 + 是不是注释 + 是不是只登记形态」定红档还是登记档"""
            done.add(i)
            if on_face and i not in xmlc and not comment and not soft:
                judged.append((rel, i, tag))
            else:
                registered.append((rel, i, tag,
                                   "注释行" if (comment or i in xmlc) else why_soft))

        # ④ **先**扫单元面的 `<string>` 对（可跨行）⇒ 报选项那一行的行号
        #    （先扫 ⇒ `<string>--token=<值></string>` 落 ④ 档名，而不是落成 ③/裸提及）
        if "<string>" in text:
            for rx in (PLIST_NEAR, PLIST_INLINE):
                for m in rx.finditer(text):
                    i = _line_of(text, m.start())
                    if i not in done:
                        rec(i, "PLIST-OPT-ARGV", "非判红面")

        for i, ln in enumerate(text.splitlines(), 1):
            if i in done:
                continue
            s = ln.strip()
            comment = s.startswith("#") or s.startswith("//") or i in xmlc
            hard = None
            if HDR_SITE.search(ln):
                hard = "HDR-ARGV"
            elif _qs_hit(ln, exempt_shapes):
                hard = "QS-ARGV"
            else:
                # 口令经 argv 的三支**一律判红**（免密面纯变量形态已由「只登记」提为判红 ⇒ 无软档）
                hard = _pw_hit(ln)
            if hard:
                rec(i, hard, "非判红面", comment=comment)
                continue
            if OPT_JUDGE.search(ln):
                rec(i, "UNIT-OPT-ARGV", "非判红面", comment=comment)
            elif OPT_MENTION.search(ln):
                # 裸选项提及（`--token <占位>` · 本件自己的 CLI 收 `--token`）：**只登记，不判红** ✗
                rec(i, "OPT-MENTION", "非判红面", soft=True, comment=comment)
    return judged, registered, n_face


def render(root):
    if len(EXEMPT_QS) > 1:
        print("BLOCKED —— 点名登记豁免表长 %d > 1 ⇒ 本门自废（rc=2 · 不许用豁免把门藏起来）"
              % len(EXEMPT_QS), file=sys.stderr)
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
            "schema": "zerg/v1", "kind": "TokenArgvScan", "root": a.root,
            "face_files": n_face, "sh_files": n_face,
            "judged": len(judged), "registered": len(registered),
            "exempt_qs": [f for (f, _) in EXEMPT_QS],
            "judged_sites": [{"file": f, "line": i, "shape": t} for (f, i, t) in judged],
        }, ensure_ascii=False))
        return rc
    print("扫描根：%s" % a.root)
    print("判红面件 %d 个（脚本 + 单元）· **判红 %d 处** · 登记档 %d 处（只登记、不判红）· 点名登记豁免 %d 条"
          % (n_face, len(judged), len(registered), len(EXEMPT_QS)))
    if judged:
        print("⇒ 令牌/口令进了命令行实参（逐处点名 · 逐档）：")
        for f, i, t in judged:
            print("  ✗ %s:%d  [%s]" % (f, i, t))
        print("⇒ 口径：值走 **stdin**（`curl -H @-` 那一形态）· 单元面只许 `EnvironmentFile=` 指向 600 文件"
              "· plist 面只许 `EnvironmentVariables` —— 命令行上不许出现令牌/口令的字面值或变量展开 ⇒ rc=1")
        return rc
    print("⇒ 判红面**零处**令牌/口令进 argv（五档口径成立）—— rc=0")
    print("★ 登记档照实列出（不判红 · 供复核）：")
    for f, i, t, why in registered:
        print("  [登记/%s] %s:%d  %s" % (why, f, i, t))
    return rc


# 自检夹具：(档, 形态说明, 用例名, 文件名, 正文, 期望判红档)
NEG_CASES = [
    ("①", "变量展开（curl -H 令牌头）", "neg1-curl-header-var", "probe.sh",
     '#!/usr/bin/env bash\ncurl -s -m 4 "$U" -H "X-Auth-Token: ${TOKEN}"\n', "HDR-ARGV"),
    ("②", "URL 查询串带令牌", "neg2-url-querystring", "probe.sh",
     '#!/usr/bin/env bash\ncurl -s -m 4 "$API/user?access_token=$T"\n', "QS-ARGV"),
    ("③", "systemd ExecStart「选项名+值」", "neg3-systemd-execstart", "edge-node-agent.service",
     '[Service]\nEnvironmentFile=-/home/<user>/agent/edge-node-agent.env\n'
     'ExecStart=/usr/bin/python3 /home/<user>/agent/agent.py --controller http://<cluster-ip>:8580 '
     '--token 9f3c1d7a5b2e4f60\n', "UNIT-OPT-ARGV"),
    ("③", "单元正文在 .sh 的 heredoc 里（卵里生成）", "neg3b-unit-in-heredoc", "deploy_x3.sh",
     '#!/usr/bin/env bash\ncat > ${D}/edge-node-agent.service << UNIT_EOF\n[Service]\n'
     'ExecStart=/usr/bin/python3 ${D}/agent.py --host 0.0.0.0 --token=9f3c1d7a5b2e4f60\n'
     'UNIT_EOF\n', "UNIT-OPT-ARGV"),
    ("④", "launchd plist「选项名+值」（两串）", "neg4-launchd-plist", "com.x3gw.agent.plist",
     '<?xml version="1.0"?>\n<array>\n  <string>/usr/bin/python3</string>\n'
     '  <string>--token</string>\n  <string>9f3c1d7a5b2e4f60</string>\n</array>\n', "PLIST-OPT-ARGV"),
    ("④", "launchd plist 单串形态", "neg4b-launchd-plist-inline", "com.x3gw.agent.plist",
     '<?xml version="1.0"?>\n<array>\n  <string>--api-key=9f3c1d7a5b2e4f60</string>\n</array>\n',
     "PLIST-OPT-ARGV"),
    ("⑤", "sudo -S 口令经 argv（echo|管道）", "neg5-sudo-stdin-value", "probe.sh",
     '#!/usr/bin/env bash\necho \'S3cr3t-Pw!\' | sudo -S systemctl restart edge-node-agent\n', "PW-SUDO-ARGV"),
    ("⑤", "sshpass -p 字面口令", "neg6-sshpass-literal", "probe.sh",
     '#!/usr/bin/env bash\nsshpass -p \'Passw0rd!\' ssh root@<cluster-ip> true\n', "PW-SSHPASS-ARGV"),
    ("⑤", "sshpass -p 纯变量（命令行展开）", "neg7-sshpass-pure-variable", "probe.sh",
     '#!/usr/bin/env bash\nSP="sshpass -p $PASS"\n$SP ssh root@<cluster-ip> true\n', "PW-SSHPASS-VAR"),
]

# 正控夹具：(用例名, 文件名, 正文, 期望登记 ≥ N)
POS_CASES = [
    ("pos1-curl-stdin-form", "probe.sh",
     '#!/usr/bin/env bash\nTOKEN_FILE="$HOME/.zerg/token"\n'
     'curl_tok() { printf \'X-Auth-Token: *** %s\\n\' "$(cat "$TOKEN_FILE")" | curl -H @- "$@"; }\n'
     'curl_tok -s -m 4 "$U"\n', 0),
    ("pos2-unit-envfile-clean", "edge-node-agent.service",
     '[Service]\nUser=g01\nEnvironmentFile=-/home/<user>/agent/edge-node-agent.env\n'
     'ExecStart=/usr/bin/python3 /home/<user>/agent/agent.py --host 0.0.0.0 '
     '--registry /home/<user>/agent/agent_models.yaml\n', 0),
    ("pos3-plist-token-in-xml-comment", "com.x3gw.agent.plist",
     '<?xml version="1.0"?>\n<!-- 禁式: <string>--token</string><string>9f3c1d7a5b2e4f60</string> -->\n'
     '<dict>\n  <key>EnvironmentVariables</key>\n  <dict>\n    <key>X3GW_TOKEN</key>\n'
     '    <string>${TOKEN}</string>\n  </dict>\n</dict>\n', 1),
    ("pos4-sshpass-env-e", "probe.sh",
     '#!/usr/bin/env bash\nSSHPASS="${PASS}" sshpass -e scp "$F" mini:/tmp/\n', 0),
]


def self_test():
    """逐型负控（五型 · 逐型一个用例 ⇒ 逐型 rc=1）＋ 正控（干净件 ⇒ rc=0）· 自带工作区隔离（只用临时目录）"""
    d = tempfile.mkdtemp(prefix="q207-selftest-")
    try:
        bad = 0
        print("── 自检（合成夹具 · 临时目录 · 不碰真目标）──")
        print("[逐型负控] 五型各一个用例 ⇒ 每型必须 rc=1 且**恰好判红 1 处**、档名相符")
        for tier, desc, name, fn, body, tag in NEG_CASES:
            sub = os.path.join(d, name)
            os.makedirs(sub)
            open(os.path.join(sub, fn), "w", encoding="utf-8").write(body)
            rc, data = render(sub)
            judged = data[0] if data else []
            got = judged[0][2] if judged else "-"
            ok = rc == 1 and len(judged) == 1 and got == tag
            bad += 0 if ok else 1
            print("  %s %-24s %-30s rc=%d · 判红 %d 处 [%s] · %s"
                  % (tier, desc, name, rc, len(judged), got, "✓" if ok else "✗ 断言不过"))

        print("[正控] 干净件（`-H @-` stdin 形态 / `EnvironmentFile=` / 注释里的同形 /"
              " 合法 `sshpass -e` 免密形态）⇒ rc=0")
        for name, fn, body, reg_min in POS_CASES:
            sub = os.path.join(d, name)
            os.makedirs(sub)
            open(os.path.join(sub, fn), "w", encoding="utf-8").write(body)
            rc, data = render(sub)
            judged, reg = (data[0], data[1]) if data else ([], [])
            ok = rc == 0 and not judged and len(reg) >= reg_min
            bad += 0 if ok else 1
            print("  %-32s rc=%d · 判红 %d 处 · 登记 %d 处 · %s"
                  % (name, rc, len(judged), len(reg), "✓" if ok else "✗ 断言不过"))

        print("[缺件/空转/豁免钳] ⇒ 一律 rc=2（判不了 · 不许当绿）")
        rc_missing = render(os.path.join(d, "nope"))[0]
        empty = os.path.join(d, "empty")
        os.makedirs(empty)
        open(os.path.join(empty, "a.md"), "w", encoding="utf-8").write("# 只有登记面件\n")
        rc_empty = render(empty)[0]
        EXEMPT_QS.append(("x/y.sh", "token"))
        rc_exempt = render(empty)[0]
        EXEMPT_QS.pop()
        ok = rc_missing == 2 and rc_empty == 2 and rc_exempt == 2
        bad += 0 if ok else 1
        print("  扫描根不存在 rc=%d · 零脚本/单元件 rc=%d · 豁免表长 2 rc=%d · %s"
              % (rc_missing, rc_empty, rc_exempt, "✓" if ok else "✗ 断言不过"))

        print("⇒ 自检 %s" % ("全过（rc=0）" if bad == 0 else "有 %d 条断言不过（rc=1）" % bad))
        return 0 if bad == 0 else 1
    finally:
        shutil.rmtree(d, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
