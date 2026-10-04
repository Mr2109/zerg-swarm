#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""发布期门禁：「危险路径」与「令牌形态」。任一命中 ⇒ 不得发布。

退出码：0 = 干净；1 = 有命中（不得发布）；2 = 脚本自测未过**或目标不是公开产物树**（拒绝出结论）。

档位（自述与退码同向 · 缺口 `Q-177`，病灶见下方「前置判据」一节）：
  · **发布档**（默认 · 目标**必须是公开产物树**）：判「即将发布的这棵树」。
      目标不是产物树 / 判不出是不是 ⇒ **rc=2 拒绝出结论**（缺语料铁律：判不了不给结论，
      **既不得 FAIL、更不得静默绿**）。私有/开发树请改用审计档。
  · **审计档**（`--all`）：扫全史对象，**供人工审计**，**预期为红**，不作发布结论
      （rc=1 只说「审计有命中」，与档位自述同向）。

原由（2026-09-13 审计报告 §8.2）：公开仓历史里曾出现过一批 **CA 任务日志**
`events.jsonl`（**件数与合计体积一律现读 · 禁手写**：审计档 `--all` 的对象面按 `.jsonl`
数件 · 逐件 `git cat-file -s` 求和；含令牌形态与 bearer 字样），而当时的发布清单只按
「识别词」（私有称谓/卷路径/家机名）扫描，**没有任何「危险路径」闸** —— 换个文件名
就不会被任何一道检查看见。同时，项目令牌前缀形态曾随文档/脚本进入公开历史。

判据来源（均为实测 —— **下面凡带读数处一律现读 · 禁手写**）：
  · 路径判据取「存在性」——**末树件数**（= `git -C <产物目录> ls-tree -r --name-only HEAD`
    的行数）上 `DANGER_PATH` 的**全部模式**（条数 = 本件该列表现读）**全部 0 命中**（零假阳）。
  · 形态判据只保留**项目特有前缀**与**收窄的 bearer 形态**：
      - `x3gw-` 前缀 + 足够长的值：末树 0 命中、**全史命中笔数现读 · 禁手写**
        （跑审计档 `--all` 取；能红）。
      - `Bearer ` + 长值（≥20）：末树 0 命中。
    刻意**不**纳入通用 `api_key=<长串>`：`core/internal/memory/memory_test.go` 里
    存在**合成的假凭据**（用于测试敏感信息扫描），纳入即假阳。
    也**不**纳入「超长 base64 样串」：末树命中**处数现读 · 禁手写**（逐文件 `git grep -c`
    求和取），命中物是哈希/内嵌资源，纯噪声。

用法: python3 scripts/gates/check-public-tree-hazards.py <git目录> [<commit>|--all]

目标语义（缺口 `Q-177` 治「档位自述与退码不同向」）：
  · **不给目录不再默认判 `.`** —— `.` 是私有权威仓（**规模现读 · 禁手写**：提交数 =
    `git rev-list --count HEAD` · 文件数 = `git ls-tree -r --name-only HEAD` 的行数），而私有树里
    本就留着发布机制自己的脚本与运行期日志 ⇒ 默认调用**恒红**，真红与「判错了树」在机器上同形。
  · 发布档要先过**前置判据**（`tree_class`：单提交 + 快照提交信息）；判不出 ⇒ `rc=2`。

模式语义（重要）：
  · **发布闸用 HEAD**（或指定 commit）—— 判的是「即将发布的这棵树」里有没有危险物；
    危险物一旦进了末树，闸即红，发布被拦在推送之前。
  · `--all` 扫全史对象，**供人工审计**用。本仓历史上确实存在过的日志与旧令牌残留
    （见 Zerg-内部文档/issues/审计报告-公开面私有串残留-20260913.md §8.2 —— 该报告 2026-09-19 已随开发文档
    分家整目录迁出工作树，原路径 docs/issues/；Mr2109 已决定
    不重写历史）会让 `--all` **预期为红** ⇒ 它不得直接当发布闸，否则永久阻塞发布。

注意：本文件自身不得出现真实令牌值或完整可命中的形态（自测串一律拼接构造）。
"""
import re
import subprocess
import sys

# ── vendor/ 出局（GAP-20260927-147 · 2026-10-04 现读）：为什么本闸不把 vendor 的 `eval` 当「随件出去」──
#   现读三条（命令行现取 · 禁手写）：
#     · `git ls-files -s vendor/` ⇒ **2 条 gitlink**（mode 160000）：`vendor/rtk` · `vendor/searxng`；
#     · `publish/whitelist.txt`（发布面唯一真源 = **allow-list**，`rsync --files-from` 导出）里 `vendor` **0 命中**；
#     · `scripts/build/publish-public.sh` 的 `EXCLUDES=( … )`（白名单内排除项）里 `vendor` **0 命中**。
#   ⇒ vendor 既**不在白名单**（进不去）也不在 EXCLUDES（不是「白名单内被排」）——**结构性不进产物树**。
#   故 `vendor/searxng/utils/lib.sh:400/469` 与 `vendor/rtk/scripts/benchmark.sh:44/635` 的那两处 `eval`
#   （「执行位 + 仓外输入」形态）**不随件出去**；本闸的判面（危险路径 / 令牌形态）**不涉及 vendor**。
#   ★ 本闸**不改扫描面**（DANGER_PATH / TOKEN_FORM 一条都不动）。
#   ★ 负控自检格（见 `selftest()` 的 `cases_vendor`）：
#       v1 = vendor 现读的两处 `eval` 件**不是**本闸判据形态 ⇒ 本闸对它**保持沉默是对的**（不误报）；
#       v2 = **有牙**：若哪天 vendor 进了白名单、且里面有危险物（如 `.jsonl`），本闸的**路径判据照抓**
#            —— 证明判面不因 vendor 而失明（不是「vendor 一律不看」，是「vendor 的结构性出局经现读坐实」）。

# ── 判据 A：危险路径（存在性；按仓库根相对路径匹配）────────────────
DANGER_PATH = [
    (r"(^|/)\.zerg/",              "运行期任务日志/状态目录"),
    (r"\.jsonl$",                  "事件/日志流文件"),
    (r"\.db$",                     "SQLite 数据库"),
    (r"\.db-(wal|shm)$",           "SQLite WAL/SHM"),
    (r"\.sqlite$",                 "SQLite 数据库"),
    (r"(^|/)logs/",                "日志目录"),
    (r"\.log$",                    "日志文件"),
    (r"(^|/)\.env$",               "真配置文件（示例应叫 .env.example）"),
    (r"\.env\.local$",             "本地覆盖配置"),
    (r"(^|/)target/",              "构建产物目录"),
    (r"(^|/)node_modules/",        "依赖目录"),
    (r"\.DS_Store$",               "macOS 目录元数据"),
]

# ── 判据 B：令牌形态（内容）──────────────────────────────────────
TOKEN_FORM = [
    (r"x3gw-[A-Za-z0-9_.\-]{6,}",        "项目网关令牌形态"),
    (r"Bearer [A-Za-z0-9_.\-]{20,}",     "bearer + 长值（真凭据）"),
]

PATH_RES = [(re.compile(rx), why) for rx, why in DANGER_PATH]
FORM_RES = [(re.compile(rx), why) for rx, why in TOKEN_FORM]

# 拼接构造自测串，避免本文件出现完整可命中的形态
T = "x3gw-"
B = "Bearer "

# ── 前置判据（缺口 `Q-177`）：传入目录是不是「公开产物树」──────────────
#   为什么加（2026-09-26 现读坐实 · 另一枚卵）：本件默认参数是 `.` ⇒ **无参调用判的是
#   私有权威仓 HEAD 全树**（规模**现读 · 禁手写**：提交数 = `git rev-list --count HEAD` ·
#   文件数 = `git ls-tree -r --name-only HEAD` 的行数），而私有树里本就留着发布机制
#   自己的脚本与运行期日志 ⇒ **默认调用恒红**（现读 rc=1：`scripts/gates/baseline-audit.jsonl`
#   路径命中 + `scripts/build/publish-public.sh` 令牌形态命中）；发布闸
#   （`scripts/build/publish-preflight.sh:85`）传的是 `$D` = **产物目录**（规模**现读 · 禁手写**：
#   以 `git -C "$D" ls-tree -r --name-only HEAD` 的行数为准 ⇒ 0 命中 rc=0）。
#   两处**自述同形、退码反色** ⇒ 机器上分不开「真有危险物」与「判错了树」。
#   治法选**乙**：给发布档加**前置判据**；**判不出「这是产物树」⇒ rc=2 拒绝出结论**
#   （缺语料铁律：判不了不给结论 —— 既不得 FAIL、也不得静默绿）。
SNAPSHOT_SUBJECT = "Zerg Swarm (虫族) — 公开快照"   # 压平提交首行（scripts/build/publish-public.sh:954）
SNAPSHOT_BODY_MARK = "GitOrigin-RevId:"            # 压平提交正文（同件 :958，第二道独立信号）


def tree_class(commit_count, subject, body):
    """产物树判据（**纯函数** · 无子进程 ⇒ 可自测）：单提交 + 快照提交信息。
    返回 `(是产物树, 判据说明)`；判不出 ⇒ 由调用侧退 `rc=2`，绝不当成「命中」。"""
    if commit_count is None:
        return False, "读不到提交数（不是 git 仓 / 空仓？）"
    if commit_count != 1:
        return False, "提交数 %s ≠ 1（产物树由压平器收成单提交）" % commit_count
    if not (subject or "").startswith(SNAPSHOT_SUBJECT):
        return False, "提交首行不是快照首行（现读 %r）" % (subject or "")
    if SNAPSHOT_BODY_MARK not in (body or ""):
        return False, "提交正文缺 `%s`（第二道独立信号）" % SNAPSHOT_BODY_MARK
    return True, "单提交 + 快照提交信息（压平产物树）"


def probe_product_tree(gitdir):
    """读目标仓实况交给 `tree_class`；读不到 ⇒ `(False, 说清为何判不了)`。"""
    try:
        log = subprocess.run(["git", "-C", gitdir, "log", "-1", "--format=%s%x00%b"],
                             capture_output=True, text=True, encoding="utf-8", errors="replace")
    except (OSError, subprocess.SubprocessError):
        return False, "调不起 git（本机无 git 可执行或权限不足）"
    if log.returncode != 0 or "\x00" not in log.stdout:
        return False, "读不到 HEAD 提交信息（非 git 目录 / 空仓：`%s`）" % gitdir
    subject, body = log.stdout.rstrip("\n").split("\x00", 1)
    cnt = subprocess.run(["git", "-C", gitdir, "rev-list", "--count", "HEAD"],
                         capture_output=True, text=True).stdout.strip()
    return tree_class(int(cnt) if cnt.isdigit() else None, subject, body)


def selftest():
    cases_path = [
        ("core/.zerg/logs/2026-01-01T00-00-00/events.jsonl", True, "运行期日志（本次事故的形态）"),
        ("data/knowledge.db", True, "SQLite 库"),
        ("data/knowledge.db-wal", True, "WAL"),
        ("var/logs/app.log", True, "日志目录/文件"),
        ("docs/logs.md", False, "文件名含 logs，但不是日志目录"),
        ("docs/guide.md", False, "正常文档"),
        (".env", True, "真配置"),
        (".env.example", False, "示例配置（正当）"),
        ("src/main.go", False, "正常源码"),
        ("ui/target/debug/x", True, "构建产物"),
    ]
    # ── ★ 负控自检格（GAP-20260927-147 · vendor 出局）：钉死「vendor 结构性出局」这条现读结论 ──
    #   v1：vendor 现读的两处 `eval` 件（searxng lib.sh / rtk benchmark.sh）**不是**本闸的判据形态
    #       ⇒ 本闸对它**保持沉默是对的**（不误报）。v2：**有牙** —— 若 vendor 进了产物树且含危险物
    #       （`.jsonl`），路径判据**照抓**，证明判面不因 vendor 而失明。
    cases_vendor = [
        ("vendor/searxng/utils/lib.sh", False, "v1 vendor 出局：lib.sh 的 eval 非本闸判据形态（不误报）"),
        ("vendor/rtk/scripts/benchmark.sh", False, "v1 vendor 出局：benchmark.sh 的 eval 非本闸判据形态（不误报）"),
        ("vendor/searxng/events.jsonl", True, "v2 有牙：vendor 若含危险物，路径判据照抓（判面不因 vendor 失明）"),
    ]
    cases_form = [
        ("Authorization: " + B + "a1b2c3d4e5f6g7h8i9j0k1l2", True, "真 bearer 值"),
        ("Authorization: " + B + "<token>", False, "占位写法（正当）"),
        ("令牌以 " + T + " 开头", False, "说明文字（无实际值）"),
        ("X-Auth-Token: " + T + "shared0123456789", True, "实际令牌形态"),
        ("curl -H 'X-Auth-Token: $TOKEN' http://x", False, "变量引用"),
    ]
    # `Q-177` 前置判据格（**纯函数**格：输入 → 是否产物树；判不出 ⇒ 调用侧 rc=2）
    cases_tree = [
        (1, SNAPSHOT_SUBJECT, "源码快照，非开发历史。" + SNAPSHOT_BODY_MARK + " abc", True,
         "压平产物树（发布档正控）"),
        (1, "feat: 私有开发提交", SNAPSHOT_BODY_MARK + " abc", False,
         "单提交但不是快照 ⇒ 判不了（不给结论）"),
        (1, SNAPSHOT_SUBJECT, "正文无第二道信号", False, "缺 GitOrigin-RevId ⇒ 判不了"),
        (1332, SNAPSHOT_SUBJECT, SNAPSHOT_BODY_MARK + " abc", False, "私有权威仓（多提交）⇒ 判不了"),
        (None, SNAPSHOT_SUBJECT, SNAPSHOT_BODY_MARK + " abc", False, "读不到提交数 ⇒ 判不了"),
    ]
    bad = 0
    for text, want, why in cases_path:
        got = any(rx.search(text) for rx, _ in PATH_RES)
        ok = (got == want)
        bad += (not ok)
        print("   %s want=%-5s got=%-5s | %s" % ("✓" if ok else "✗", want, got, why))
    for text, want, why in cases_vendor:
        got = any(rx.search(text) for rx, _ in PATH_RES)
        ok = (got == want)
        bad += (not ok)
        print("   %s want=%-5s got=%-5s | %s" % ("✓" if ok else "✗", want, got, why))
    for text, want, why in cases_form:
        got = any(rx.search(text) for rx, _ in FORM_RES)
        ok = (got == want)
        bad += (not ok)
        print("   %s want=%-5s got=%-5s | %s" % ("✓" if ok else "✗", want, got, why))
    for cnt, subj, body, want, why in cases_tree:
        got = tree_class(cnt, subj, body)[0]
        ok = (got == want)
        bad += (not ok)
        print("   %s want=%-5s got=%-5s | %s" % ("✓" if ok else "✗", want, got, why))
    # ── ★ 回归自测格（缺口 `GAP-20260927-216` · 2026-09-27 补）：**非 ASCII（CJK）件名**不许失明 ──
    #   病灶（今天已修，本格是牙齿）：`git ls-tree --name-only` 默认 `core.quotePath=true`，把
    #   非 ASCII 件名输出成 `"子目录/\344\270\255…"`（**首尾双引号 + 反斜杠八进制**）⇒ 拿它去
    #   比对 / 拼路径 / `os.stat` 全失效（该件在清单里“查不到”，闸却照样绿）。
    #   `all_paths()` 的治法就是那一处 `-c core.quotepath=false`；本格把它钉死 —— 谁把那处 `-c`
    #   拿掉，这一格立刻变红（负控已在**仓外副本**上实测：去 `-c` ⇒ 本自测 rc=2）。
    #   夹具：**仓外**临时 git 仓（`tempfile.mkdtemp`）里建一枚中文名件 `子目录/中文名件.md`
    #   ＋一枚 ASCII 件作对照；跑的是本件真取清单那条路径 `all_paths()`。
    import os as _os
    import shutil as _shutil
    import tempfile as _tempfile
    n_extra = 0
    _fx_root = _tempfile.mkdtemp(prefix="hazards-selftest-cjk-")
    _fx_repo = _os.path.join(_fx_root, "repo")
    _cjk_rel = _os.path.join("子目录", "中文名件.md")
    _git_env = dict(_os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@e",
                    GIT_COMMITTER_NAME="t", GIT_COMMITTER_EMAIL="t@e")
    try:
        _os.makedirs(_os.path.join(_fx_repo, "子目录"), exist_ok=True)
        with open(_os.path.join(_fx_repo, _cjk_rel), "w", encoding="utf-8") as fh:
            fh.write("x\n")
        with open(_os.path.join(_fx_repo, "docs-ascii.md"), "w", encoding="utf-8") as fh:
            fh.write("x\n")
        subprocess.run(["git", "init", "-q", "-b", "main", _fx_repo], check=True, env=_git_env)
        subprocess.run(["git", "-C", _fx_repo, "add", "-A"], check=True, env=_git_env)
        subprocess.run(["git", "-C", _fx_repo, "commit", "-q", "-m", "fixture"], check=True, env=_git_env)
        _listed = all_paths(_fx_repo, "HEAD")
        _naive = subprocess.run(["git", "-C", _fx_repo, "-c", "core.quotepath=true",
                                 "ls-tree", "-r", "HEAD", "--name-only"],
                                capture_output=True, text=True, encoding="utf-8",
                                errors="replace").stdout
        _naive_escaped = any(('"' in p) or re.search(r"\\[0-7]{3}", p)
                             for p in _naive.split("\n") if p.strip())
        _stat_ok = True
        try:
            _os.stat(_os.path.join(_fx_repo, _cjk_rel))
        except OSError:
            _stat_ok = False
        # 格①：清单里那枚中文名件**原样**在（不是被转义/加引号的形态），且按此名能 `os.stat` 到
        _case1 = [
            (_cjk_rel in _listed, "中文名件在清单里（原样命中）"),
            (not any(p.startswith('"') or p.endswith('"') for p in _listed),
             "清单条目首尾无双引号"),
            (not any(re.search(r"\\[0-7]{3}", p) for p in _listed),
             "清单条目无反斜杠八进制转义"),
            (_stat_ok, "按清单里的名字能 os.stat 到该件"),
            (all(p in ("docs-ascii.md", _cjk_rel) for p in _listed),
             "夹具清单只含这两件（现读 %r）" % (_listed,)),
        ]
        for _ok, _why in _case1:
            bad += (not _ok)
            n_extra += 1
            print("   %s %-32s | %s" % ("✓" if _ok else "✗", "cjk-name-clean-from-list", _why))
        # 格②：**这一格有牙齿** —— 同仓同件，默认 `quotePath` 下 git 确实把它转义出去
        #      （⇒ 一旦 `all_paths()` 丢了 `-c core.quotepath=false`，格① 必红，不是恒绿摆设）
        _teeth = _naive_escaped and ("docs-ascii.md" in _naive)
        bad += (not _teeth)
        n_extra += 1
        print("   %s %-32s | %s" % ("✓" if _teeth else "✗", "cjk-name-quotepath-is-real",
                                    "默认 quotePath 下 git 确实转义该名（⇒ 丢 `-c` 必红）"))
    except (OSError, subprocess.SubprocessError) as exc:
        # 建不起夹具（无 git / 临时目录不可写）⇒ **红**：缺语料不许静默绿
        bad += 1
        n_extra += 1
        print("   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿" % ("cjk-name-clean-from-list", exc))
    finally:
        _shutil.rmtree(_fx_root, ignore_errors=True)
    # ── ★ 回归自测格（缺口 `GAP-20260926-216` · 2026-09-28 补）：**真扫描面**不许哑化 ──
    #   病灶（今天已治，本格是牙齿）：本自测此前只喂**分类面**（`PATH_RES`/`FORM_RES` 逐条正则
    #   ＋`tree_class()` 纯函数＋`all_paths()` 取清单），**真判据 `scan_paths()` / `scan_forms()`
    #   一个都不进自检** ⇒ 把 `scan_paths()` 体首行后插一行 `return []` 哑化后 `--self-test`
    #   仍 `rc=0`（假安全；2026-09-28 **仓外硬拷贝**实测复现：哑化后 `py_compile` rc=0、自测 rc=0）。
    #   治法：本格拿一艘**含真危险物**的合成树（仓外 `tempfile.mkdtemp`）喂给**真扫描面**，
    #   断言它**真判红** —— 谁把 `scan_paths()` / `scan_forms()` 哑化成空结果，本格立刻变红
    #   （负控已在**仓外副本**上实测：`scan_paths()` 体首行后插 `return []` ⇒ 本自测 rc=2、点名本格）。
    _fx2_root = _tempfile.mkdtemp(prefix="hazards-selftest-scan-")
    _fx2_repo = _os.path.join(_fx2_root, "repo")
    _danger_path = ".zerg/logs/events.jsonl"                     # 命中 `.zerg/`・`logs/`・`.jsonl` 三条危险路径判据
    _danger_body = "note: " + T + "shared0123456789\n"          # 命中项目令牌形态（拼接构造 · 本文件不含完整形态）
    try:
        _os.makedirs(_os.path.join(_fx2_repo, ".zerg", "logs"), exist_ok=True)
        with open(_os.path.join(_fx2_repo, _danger_path), "w", encoding="utf-8") as fh:
            fh.write(_danger_body)
        with open(_os.path.join(_fx2_repo, "docs-clean.md"), "w", encoding="utf-8") as fh:
            fh.write("正常文档\n")
        subprocess.run(["git", "init", "-q", "-b", "main", _fx2_repo], check=True, env=_git_env)
        subprocess.run(["git", "-C", _fx2_repo, "add", "-A"], check=True, env=_git_env)
        subprocess.run(["git", "-C", _fx2_repo, "commit", "-q", "-m", "fixture"], check=True, env=_git_env)
        # 格③：真**路径**扫描面**真判红**（正控 —— 哑化 `scan_paths()` ⇒ 本格必红）
        _ph = scan_paths(_fx2_repo, False)
        _case3 = [
            (len(_ph) >= 1, "scan_paths 对含危险路径的树真判红（现读 %d 项）" % len(_ph)),
            (any(_danger_path == p for p, _ in _ph), "命中项逐字点名 `%s`（现读 %r）" % (_danger_path, _ph)),
        ]
        for _ok, _why in _case3:
            bad += (not _ok)
            n_extra += 1
            print("   %s %-32s | %s" % ("✓" if _ok else "✗", "scan-paths-catches-hazard", _why))
        # 格④：真**形态**扫描面**真判红**（正控 —— 哑化 `scan_forms()` ⇒ 本格必红）
        _fh = scan_forms(_fx2_repo, "HEAD", False)
        _case4 = [
            (len(_fh) >= 1, "scan_forms 对含令牌形态的树真判红（现读 %d 项）" % len(_fh)),
            (any(_danger_path in h for h, _ in _fh), "命中项逐字点名 `%s`（现读 %r）" % (_danger_path, _fh)),
        ]
        for _ok, _why in _case4:
            bad += (not _ok)
            n_extra += 1
            print("   %s %-32s | %s" % ("✓" if _ok else "✗", "scan-forms-catches-hazard", _why))
    except (OSError, subprocess.SubprocessError) as exc:
        # 建不起夹具（无 git / 临时目录不可写）⇒ **红**：缺语料不许静默绿
        for _lbl in ("scan-paths-catches-hazard", "scan-forms-catches-hazard"):
            bad += 1
            n_extra += 1
            print("   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿" % (_lbl, exc))
    finally:
        _shutil.rmtree(_fx2_root, ignore_errors=True)
    print("   自测: %s（%d 格）" % ("全过" if bad == 0 else "%d 条失败" % bad,
                                   len(cases_path) + len(cases_vendor) + len(cases_form) + len(cases_tree) + n_extra))
    return bad == 0


def all_paths(gitdir, commit):
    """返回该提交（或全史对象）里的仓库根相对路径集合。"""
    out = subprocess.run(["git", "-C", gitdir, "-c", "core.quotepath=false", "ls-tree", "-r", commit, "--name-only"],
                         capture_output=True, text=True, encoding="utf-8", errors="replace").stdout
    return [p for p in out.split("\n") if p.strip()]


def scan_paths(gitdir, commits_all):
    """⇒ 命中列表；**取不到域 ⇒ None**（`GAP-20260927-114`：`--all` 审计档 rev-list 退码未查）。"""
    hits = []
    if commits_all:
        _r = subprocess.run(["git", "-C", gitdir, "-c", "core.quotepath=false", "rev-list", "--objects", "--all"],
                            capture_output=True, text=True, encoding="utf-8", errors="replace")
        if _r.returncode != 0:
            return None
        out = _r.stdout
        paths = sorted({ln.split(" ", 1)[1] for ln in out.split("\n") if " " in ln})
        for rx, why in PATH_RES:
            hits.extend((p, why) for p in paths if rx.search(p))
    else:
        for p in all_paths(gitdir, "HEAD"):
            for rx, why in PATH_RES:
                if rx.search(p):
                    hits.append((p, why))
    return hits


def scan_forms(gitdir, commit, commits_all):
    """⇒ 命中列表；**取不到域 ⇒ None**（`GAP-20260927-114`）。"""
    hits = []
    if commits_all:
        _r = subprocess.run(["git", "-C", gitdir, "rev-list", "main"],
                            capture_output=True, text=True, encoding="utf-8", errors="replace")
        if _r.returncode != 0:
            return None
        commits = _r.stdout.split()
        seen = set()
        for c in commits:
            o = subprocess.run(["git", "-C", gitdir, "grep", "-I", "-E", "-l",
                                "-e", "|".join(rx.pattern for rx, _ in FORM_RES), c, "--", "."],
                               capture_output=True, text=True, encoding="utf-8", errors="replace").stdout
            for ln in o.split("\n"):
                if not ln.strip():
                    continue
                key = ln.split(":", 1)[-1]
                if key not in seen:
                    seen.add(key)
                    hits.append((key, "令牌形态（提交 %s）" % c[:8]))
    else:
        for rx, why in FORM_RES:
            o = subprocess.run(["git", "-C", gitdir, "grep", "-I", "-E", "-l", "-e", rx.pattern,
                                commit, "--", "."],
                               capture_output=True, text=True, encoding="utf-8", errors="replace").stdout
            for ln in o.split("\n"):
                if ln.strip():
                    hits.append((ln.split(":", 1)[-1], why))
    return hits


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


if __name__ == "__main__":
    rc_q176 = refuse_json_without_machine_face()
    if rc_q176 is not None:
        sys.exit(rc_q176)

    # ★ `Q-176` 第六批：拼法统一 —— 准拼法 `--self-test`（旧拼法 `--selftest` 作别名，两个都认）。
    #   给旗标 ⇒ **只跑自检**（退出码即自检结论：0 = 过 · 2 = 不过）；不给 ⇒ 照旧「自检不过 ⇒ 拒绝出结论」。
    argv = [a for a in sys.argv[1:] if a not in ("--self-test", "--selftest")]
    if len(argv) != len(sys.argv) - 1:
        ok = selftest()
        print("── check-public-tree-hazards.py --self-test（旧拼法 --selftest 同档）：%s"
              % ("全过 ✓" if ok else "**不过** ✗"))
        sys.exit(0 if ok else 2)

    if not selftest():
        print("自测未过 ⇒ 脚本本身有问题，拒绝出结论", file=sys.stderr)
        sys.exit(2)
    target = argv[0] if len(argv) > 0 else "."
    mode = argv[1] if len(argv) > 1 else "HEAD"
    commits_all = (mode == "--all")

    # ── 档位（缺口 `Q-177`）：**自述与退码必须同向** ────────────────────
    if commits_all:
        print("档位: 审计档（`--all` 扫全史对象）—— 供人工审计，**预期为红**；"
              "rc=1 只说「审计有命中」，不作发布结论")
    else:
        is_product, why = probe_product_tree(target)
        if not is_product:
            print("⛔ 拒绝出结论（rc=2）：目标不是「公开产物树」—— %s" % why, file=sys.stderr)
            print("   目标: `%s` · 发布档只判**即将发布的产物树**（判不了就不给结论）" % target,
                  file=sys.stderr)
            print("   口径: 私有/开发树请用审计档 `--all`；「真有危险物」与「判错了树」不许同形",
                  file=sys.stderr)
            sys.exit(2)
        print("档位: 发布档（`%s` mode=%s）—— %s" % (target, mode, why))

    ph = scan_paths(target, commits_all)
    fh = scan_forms(target, mode if not commits_all else "main", commits_all)
    if ph is None or fh is None:
        print("⛔ 不给结论（rc=2）：域取不到（`git rev-list` 非零退出）——"
              "「取不到」与「零命中」不许同形（GAP-20260927-114）", file=sys.stderr)
        sys.exit(2)
    total = len(ph) + len(fh)

    if total == 0:
        print("结果: 通过——未发现危险路径与令牌形态（扫 %s mode=%s）" % (target, mode))
        sys.exit(0)

    print("结果: 不通过——危险路径 %d 项、令牌形态 %d 项（%s mode=%s）：" % (len(ph), len(fh), target, mode))
    for p, why in ph[:15]:
        print("   路径 %s   ← %s" % (p, why))
    for p, why in fh[:15]:
        print("   形态 %s   ← %s" % (p, why))
    sys.exit(1)
