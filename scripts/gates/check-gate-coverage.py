#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-gate-coverage.py —— 门覆盖自检（治「假绿」本体的那一层）

一句话：**断言「该被门收进来的东西，真的被某一步收进来了」** —— 门禁守的是别人，
这一层守的是**门自己**（有没有漏掉一整个目录 / 一整个 module / 一整类脚本）。

为什么存在（《债务台账-20260918》§0 一整节 11 行 + §7 建议门①）
  ① **新建的构建树可以静默不在任何门里**：`ui/`（Rust crate · 27G 构建产物 · 16 处
     `#[cfg(test)]`）与 `shared/`、`scripts/exportnames/` 两个 Go module **今天就不在**
     `scripts/gates/precommit-gates.sh` 的任何一步里 —— 而 `precommit-gates.sh` 的 `precheck`
     只验 `ui/Cargo.toml` **在不在**、不验它**过不过** ⇒ **ui 可以编译失败而闸全绿**。
  ② **新建的脚本可以静默不接线**：`pub` scope 只查 `scripts/*.sh|*.py` 的**语法**，
     语法过 ≠ 接了门；一只刚写完的门脚本**不写进 add_step 就永远不会有人跑它**，而闸全绿。
  ③ **「没有门」这件事本身没有任何门在管**：全部判定都靠「脚本里手写的 add_step 步骤表」，
     没有一处能发现「一个新目录 / 一个新 module 没被任何 add_step 收进来」。
  ⇒ 本脚本就是那一处。**它是唯一一个「扫步骤表」的门**。

两条断言（A 阻断 · B 阻断但带基线棘轮）

  A. **构建清单目录必须被某步收进**
     扫全仓含 `go.mod` / `Cargo.toml` 的目录（排除 `.git`/`vendor`/`node_modules`/`target`/
     构建产物/venv…，名单见 config `exclude`），逐个问「有没有哪一步把这个目录收进来了」。
     命中口径（三条，任一条成立即算收进；逐条给「是哪一步」作证）：
       A-a **工作目录命中**：某步的工作目录 == 该构建目录
            例：`add_step go … rc "${REPO_ROOT}/core" "go build ./..."`
       A-b **命令串路径参数命中**：命令串里出现该目录（相对仓根或相对工作目录），
            按路径边界匹配（`shared` / `./shared` / `cd shared` / `shared/...` 都算；
            `ui` 不许被 `build`、`ruin` 之类的子串蹭到）
       A-c **通配命中**：命令串里的 shell 通配 token 经 fnmatch 后覆盖该目录
     ★ **口径写死一条**：上级目录的 `./...` **不收**下级嵌套 module（Go 的多 module 语义
       就是这样：`./...` 不跨 `go.mod` 边界）—— 所以「core 被收进」不能证明「shared 被收进」。
     ★ 今天应真红的三条（**不许填白名单洗干净**）：`ui/` · `shared/` · `scripts/exportnames/`。

  B. **scripts 脚本必须要么在步骤表、要么在白名单**（**带基线棘轮**）
     扫 `scripts/*.sh` · `scripts/*.py` · `scripts/` 下无后缀可执行件，逐个问「这个脚本
     在 `precommit-gates.sh` 里被点名了吗」，分三档（**三档之和恒等于候选总数**，计数自证）：
       ① **add_step 命令串里点名**（真被某步跑）—— 最强
       ② **闸文件里被提到、但不在任何 add_step 命令串里**（**弱挂载**）—— 再拆两档
          （★ `GAP-20260927-123`：名字「全文出现即算」= 静默洞，必须区分「真接线」与「只被提一下」）：
          ②a **真接线**（弱挂载）：名字落在闸文件的**命令位**上（启动器 `python3`/`bash`/`${PY}`/`./`
             + 路径 + 名，逐条与门③ `check-wired-scripts.py:38-41` 同口径）—— 它**真被闸体调用**，
             只是不走 add_step 命令串；
          ②b **只被提一下**：名字只在注释 / 文案 / 清单项里出现 —— **不算接线**（「写在注释里的门」）；
             ★ ②b 里**新件**（不在 git 索引）也落 ④「新件未挂门」⇒ rc=1（治「名字被提一下即永久静默」）
       ③ **闸文件里查无此名** —— **棘轮命中数**，与 config `baseline` 比
     ★ **通用语法步的 glob 兜底不计入 ①②③**：`scripts/*.sh 语法（bash -n）` 与
       `scripts/*.py 语法（ast.parse）` 用通配覆盖全部 82 条 —— **语法过 ≠ 接了门**，
       把 glob 当「在表里」就是本门要治的那个假绿。glob 命中单独打印。
     ★ **一次红 71 条会把提交闸锁死** ⇒ 起步用**基线棘轮**（同 D2「只报告」的先例）：
       `命中数 > baseline ⇒ rc=1`；`≤ baseline ⇒ rc=0`，但**必须打印剩余数**（不许悄悄绿）。

     ★ **③ 里哪几类本质挂不了门**（20260926 取证读数 · **只解释构成，不是豁免**）
       未挂门 77 条（存量 66 + 新件 11）里：**可分可挂 37 条 / 不可挂 40 条**
       （可分可挂 = 只读档 23 + 自检档 4 + 真跑档 10）。不可挂的 40 条按「为什么在提交闸里挂不上」归 **8 类**：
         ① 需模型面（跑真 Agent / 对话 / 靶子）**20** —— `evals/v25_*.py`×14 · `evals/v24_full_test.py` ·
            `evals/ab-campaigns.py` · `evals/chat-harness.py` · `calib/calib-local-model.sh` ·
            `calib/calib-task-budget.py` · `build/run_all.sh`
         ② 需真机 + 写盘（EDGE-NODE 真 GTT / 装机换装到前缀）**2** —— `calib/calib-x3-model.sh` · `svc/setup-zerg.sh`
         ③ 起常驻进程 / 碰 launchd / cron **2** —— `svc/zerg-core-daemon.sh` ·
            `svc/zerg-cron-submit.sh`
         ④ 需必填参数（无默认可跑档）**5** —— `gates/accept-no-probe.sh` · `gates/check-manifest-freshness.py` ·
            `gates/who-references.py` · `evals/make-manifest.py` · `evals/compare.py`
         ⑤ 需网络 / ssh 同步 **1** —— `svc/fleet-relay.sh`
         ⑥ 夹具 / 负控 / 变异（**是夹具不是工具**）**6** —— `build/negctl-build-wall.sh` ·
            `build/build-wall-mutate.py` · `gates/two-states-gate-mutate.py` ·
            `gates/probe-build-all-wall-wiring.sh` · `wall/wall-bridge-mutate.py` · `wall/wall-macos-mutate.py`
         ⑦ 元门 / 人用视图（跑一遍 = 全部门之和）**1** —— `gates/gate-report.py`
         ⑧ 需活主控（要被插件宿主加载才能跑）**1** —— `plugins/zerg-hello`
         ⇒ 20 + 2 + 4 + 5 + 1 + 6 + 1 + 1 = **40**（+ 可分可挂 37 = 77，与逐件表恒等 ✓）
        ★ **这 40 条照旧计入 ③ 命中数、照旧与 `baseline` 比** —— 本节只把「一堆无名债」变成「可读的八类」：
          本门**不为它们加白名单**、**不改任何计数 / 阀值 / 基线**（整类加白名单 = 判据面洗白，
          见 `verification-gate-engineering` §3）。
        ★ 上面 77 / 66 / 11 / 40 是 20260926 的**封板读数**；逐件清单见仓外
          `未挂门77条-分类表-20260926.tsv`（8 列 · 77 行），分批处方见
          `Zerg-内部文档/项目文档/v2.5.13/处方-未挂门77条分批-20260926.md`。**现读数一律以 `--baseline` /
          `--json` 为准**（新建件会继续落进 ③/④，命中数本来就会动）。

例外表（四档 · 逐档计数可见 · `--list-rules` 全列）
  | 档 | 名 | 进不进退码 | 口径 |
  |---|---|---|---|
  | 一 | **硬排除**（config `exclude`） | 不进 | 结构级：`.git`/`vendor`/`node_modules`/`target`/
  |   |   |   | 构建产物/venv… —— 它们**不在候选集里**，不是「放过」 |
  | 二 | **白名单**（config `whitelist`） | 不进（**计数可见**） | 逐条给 `reason` + `date`；
  |   |   |   | **缺 reason 或缺 date ⇒ rc=2**（不是红：写不清理由 = 判不了） |
  | 三 | **基线棘轮**（config `baseline`） | 不超基线不进 | 存量债的**已登记**额度；超一条即红；
  |   |   |   | **只许下调，永不上调**（上调 = 放宽口径 ⇒ 必须走拍板） |
  | 四 | **通用语法 glob 兜底** | 不进 | 只证明「语法过了」，**不计入 B 的 ①②③**；单列 |
  ★ 悬空白名单（写了 reason/date 但盘上已无此件）⇒ **照报，不改退码**（豁免面漂移看得见）。

基线棘轮怎么下调（**号令字句**，每批一次，照抄即可）
  「批 〈批次名〉：本批把下列 〈N〉 条从「闸文件查无此名」改成「步骤表里点名」：〈逐条脚本名〉。
    依据 = `python3 scripts/gates/check-gate-coverage.py --baseline` 的 **③ 实测命中数**（改前/改后各跑一次）；
    动作 = 把 `scripts/gates/gate-coverage.config.json` 的 `baseline` 由 〈旧〉 改成 〈新〉（**只改这一个数字**）；
    规矩 = **一次只许下调、永不上调**；若实测值比基线**高**（= 又冒出新脚本）⇒ 先接线或补白名单理由，
    **不许**把基线抬上去。」
  `--baseline` 会把这段与「今日实测命中数 / 当前基线 / 剩余 / 建议值」一起打印出来。

与既有门的接口（**本波不挂**；第二波由父代理统一挂进 `scripts/gates/precommit-gates.sh`）
  本门退码口径 = 仓内三档 **0/1/2**（`2` = 缺件 / 不可判 / 空转），⇒ 挂法用 **`tri`** 模式：
      add_step cov "门覆盖自检（构建清单目录 A + 脚本接线 B）" tri "${REPO_ROOT}" \\
               "python3 scripts/gates/check-gate-coverage.py"
  ★ 全程**不接管道**（不写 `| tee` / `| tail`），rc 由 `_judge` 按真退出码判（本门 rc=2 落 BLOCKED）。
  ★ 本门**自带 `--self-test`**（成对负控），门禁别传 `--no-self-test`：先自证「会红」再扫真目标。
  ★ 挂进来之后，`check-gate-coverage.py` **自身**就进了①档 ⇒ ③ 档命中数会由 72 回落到 71，
    届时按上面的号令把 `baseline` 由 72 下调成 71。

用法
    python3 scripts/gates/check-gate-coverage.py                  # 自检 + 两条断言扫真目标
    python3 scripts/gates/check-gate-coverage.py --json           # 机读：**stdout 只有 JSON**
    python3 scripts/gates/check-gate-coverage.py --list-rules     # 两条断言 + 例外表四档 + 退码口径
    python3 scripts/gates/check-gate-coverage.py --baseline       # 基线视图（命中/基线/剩余 + 下调号令）
    python3 scripts/gates/check-gate-coverage.py --self-test      # 只跑自检（合成夹具，不碰真目标）
    python3 scripts/gates/check-gate-coverage.py --no-self-test    # 内部子进程 / 迭代调试用（门禁别用）
    python3 scripts/gates/check-gate-coverage.py --root <目录> --config <文件>
    python3 scripts/gates/check-gate-coverage.py --max-examples N  # 每桶最多列几条（默认 40）

  ★ **`--json` ⇒ stdout 只有 JSON**（`json.loads(stdout)` 直接可用；**不必**再配 `--no-self-test`）：
    `--json` 时本门的**全部自检文本**（头行 + 逐格 + 结论行）**改走 stderr** —— 人读面（不传 `--json`
    的那一档）与 rc 口径**一字不动**；结论摘要 `rc=… · …` 也照旧只进 stderr。
  ★ `--json` 与「只产人读文本、不产 JSON」的两个开关**互斥**：`--json --self-test` · `--json --list-rules`
    ⇒ **用法错 rc=2**（同给 = 号称机读却没有机器面，判不了就不给结论）。`--json --baseline` 允许：
    基线三数本来就在 JSON 里（`baseline` / `b.hit`），人读的基线视图在 `--json` 下不打印。
  ★ `--no-self-test` 不再是为了 stdout 纯净（`--json` 已自带），只留给内部子进程 / 迭代调试用。
  ★ **唯一残余例外**：`--help`（argparse 自带）在 `--json --help` 时会先打印帮助、rc=0 ⇒ stdout 是人读
    帮助文本 —— CLI 通例，不属「污染」；要机读就别同时传 `--help`。

退出码（三档 · **优先级 2 > 1 > 0**，与 check-doc-meta/name/freshness 同口径）
    0 = 全绿（A 无未收进目录；B 命中数 ≤ 基线）
    1 = **有红**（A 有目录没被任何步收进 且 不在白名单；或 B 命中数 > 基线）
    2 = **不给结论**（用法错 / 缺件：config 或闸文件不在 / config 有未实现键 / 白名单条目缺 reason
        或 date / **空转**：候选集为 0 ⇒ 假覆盖 / 计数自证不过 / 自检未过 /
        **判不了**：仓根不是 git 仓 且 ③ 档非空 ⇒ 「新件 ⟷ 存量债」分不开（`git_tracked_known=False`））
    ★ 有「判不了」的项时**不给结论**，即便同时有红 —— 判不了的时候报绿报红都是猜。

纪律：**只读**（除 /tmp 夹具外不写盘）· 无三方依赖 · 不联网 · 不起后台进程 · 不改任何已有文件。
★ **面⑥ 机检约束（用法面登记 · 缺口 `GAP-20260928-138`）**
    处置文案（`HINT_ADD_STEP` / 第二档白名单那句）的**字面只许各出现一次** ——
    门 `scripts/gates/check-newfile-registry.py:335` 的 `load_wording()` 按「`处置`」块
    机检「白名单 + reason」两词是否都指到（面⑥）；每多引一处就多一个受判块
    （自检断言里复述一句 = 自己造一个红块）⇒ 统一走常量，调度/调用点**勿复述**这句字面。
"""
import argparse
import atexit
import fnmatch
import glob
import io
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

PY = sys.executable or "python3"
HERE = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT_DEFAULT = os.path.dirname(os.path.dirname(HERE))
LOCAL_CONFIG = os.path.join("scripts", "gates", "gate-coverage.config.json")
GATES_REL = os.path.join("scripts", "gates", "precommit-gates.sh")

EXIT_OK, EXIT_FAIL, EXIT_BLOCKED = 0, 1, 2

# ★ 缺件/不支持档的**结构化错误**封包 schema（`GAP-20260927-42` ②）：`--json` 档下，机器面必须
#   能区分「缺件（判不了 ⇒ rc=2）」与「零命中（rc=0）」—— 旧行为 stdout **0 字节**，两者同形。
ERR_SCHEMA = "zerg/gate-coverage-error/1"

# ── ④ 格指路的**分类真源**（唯一一处定义，见 `declares_out_of_scope`）──────────────
#   「永远不该进步骤表」那一类件的既有真源**不是本门发明的**：
#     · `scripts/gates/gate-coverage.config.json` 第二档白名单口径逐字 = 「只有「永远不该进
#       步骤表」的件才配进来（本机可用不等于普遍可用：如**会写盘**的工具、判据绑**仓外**
#       个人语料的只读检查）」；
#     · 其中「语料在仓外」那条的 reason **逐字引件头**（现读
#       `scripts/docs/check-outrepo-dead-refs.py:19`「★ 为什么本件**不进门禁步骤表**」）。
#   ⇒ 本门只**照读**这一句件头自述，把 ④ 格的指路分两路（普通件 ↔ 仓外/写盘类）。
#   ★ 写盘型件若无这句自述（现读 `scripts/dev/atomic-write.sh` / `writer-lock.sh` /
#     `scripts/gates/check-writer-guard.py` 三件的件头**都没有**这句，尽管
#     `scripts/公开标记.tsv:235` 声称有）⇒ 本门**不猜**，只把第二路**无条件**印出来让人自己对。
OOR_DECL = "不进门禁步骤表"
DECL_SCAN_LINES = 80          # 自述口径 = 件头（前 80 行）；读不到 ⇒ 不认（不猜）

# ── ④ 格**两条指路文案**（唯一一处定义）────────────────────────────────────────
#   ★ 为什么抽成常量：判据件 `scripts/gates/check-newfile-registry.py:335 load_wording()`
#     按「`处置`」块机检「白名单 + reason」两词是否都指到（面⑥）⇒ 这句字面**只许出现一次**，
#     否则每多引一处就多一个受判块（自检断言里复述一句 = 自己造一个红块）。
#   ★ 第一路 = 普通件（可入步骤表）：同批挂门 `add_step`；
#   ★ 第二路 = 语料在**仓外** / **会写盘**的件：**第二档白名单**（`whitelist.scripts` 写
#     `path`+`reason`+`date`，缺 reason 或缺 date ⇒ rc=2）—— 给这类件 add_step 是死路。
HINT_ADD_STEP = ("        ⇒ 处置：**真修** —— 该件与本批代码**同批挂门**"
                 "（`scripts/gates/precommit-gates.sh` 的 `add_step` 按名调用它）；"
                 "新件**不许**蹭基线棘轮（棘轮只装存量债）。")
#   ★ 受判块口径：判据件按「处置」块机检「**白名单** + **reason**」两词（`check-newfile-
#     registry.py:335 load_wording`）⇒ 本串字面**只许出现一次**（多处复述 = 自己造红块）。

# ── 自检里的「stdout 纯度探针」开关（**只有 import 本文件才够得着**：无 CLI 开关、无环境变量）──
#   探针要跑一次**真命令行**（`--json`）才看得见 stdout 纯不纯；而被探的子进程若跑满自检，
#   它自己又会再起探针 ⇒ 无限自嵌套。故探针子进程把本开关置 False：**只剪掉探针自己**，
#   其余断言与「自检文本的打印 + 分流」全是真代码路径（门禁命令行永远走 True）。
_SELFTEST_DEEP = True

DEFAULT_BUILD_MANIFESTS = ("go.mod", "Cargo.toml")
DEFAULT_SCRIPT_GLOBS = ("scripts/*.sh", "scripts/*.py")
DEFAULT_NOSUFFIX_DIR = "scripts"
# 无后缀面不下沉的子目录（= check-wired-scripts.py 的 EXCL_SUBDIRS，同口径）：
# 分域前就有的工作/实验子目录，不进候选面。
# ★ 2026-09-20 T-56 决议⑤：第三棵 `253` 已退役删除 ⇒ 两处名单同批收成两棵
#   （只改一处 = 两个门打架，见 check-wired-scripts.py 头注释）。
NOSUFFIX_EXCL = ("exportnames", "sandbox-probes")
# ── 硬排除（第一档）：结构级，不是「放过」——它们压根不进候选集 ──────────────
DEFAULT_EXCLUDE_COMPONENTS = (
    ".git", "vendor", "node_modules", "target", "__pycache__", ".history", ".obsidian",
    "dist", "bin", "data", "zerg-wt", ".zerg", "venv", ".cargo", ".venv", "htmlcov",
)
DEFAULT_EXCLUDE_PREFIXES = ("tools/ocr/venv",)

# ── config 键白名单：出现未实现的键 ⇒ rc=2（否则 config 加了规则、门照绿 = 假覆盖）──
ALLOWED_TOP = {"schema", "_comment", "_baseline_note", "baseline", "scan", "exclude", "whitelist"}
ALLOWED_SCAN = {"build_manifests", "script_globs", "nosuffix_dir"}
ALLOWED_EXCLUDE = {"path_components", "prefixes"}
ALLOWED_WL = {"build_dirs", "scripts"}
ALLOWED_WL_ITEM = {"path", "reason", "date"}

DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")


class Unsupported(Exception):
    """不可判定 ⇒ 一律 rc=2（不给结论），绝不猜。"""


# ══════════════════════════════════════════════════════════════════
# 0. 步骤表提取（**唯一来源 = precommit-gates.sh 自己**；不在这里抄第二份步骤表）
# ══════════════════════════════════════════════════════════════════
# 为什么这么取：命令串里有 heredoc / 嵌套 $( ) / 中文，文本正则解析会漏会错；
#   `--emit-cmd` 只给命令串、不给工作目录（A-a 要用），所以走 **source + EXIT trap**：
#   在闸脚本自己的 shell 里跑一遍 `main --list`（不跑任何真步骤），退出时把五个 indexed
#   array 原样吐出来。★ 闸脚本尾部是 `exit "${MAIN_RC}"` ⇒ 用 EXIT trap 抢在退出瞬间 dump。
#   ★ fd 3 在 source 的 stdout 重定向之前就复制好，免得 dump 被 `>/dev/null` 吃掉。
_EXTRACT_HELPER = r'''#!/usr/bin/env bash
set -u
cd "$1" || exit 9
exec 3>&1
dump() {
  local i=0
  while [ "$i" -lt "${#STEP_NAME[@]}" ]; do
    printf '%s\t%s\t%s\t%s\t%s\0' \
      "${STEP_SCOPE[$i]}" "${STEP_NAME[$i]}" "${STEP_MODE[$i]}" "${STEP_DIR[$i]}" "${STEP_CMD[$i]}" >&3
    i=$((i+1))
  done
}
trap dump EXIT
source scripts/gates/precommit-gates.sh --list >/dev/null 2>&1
'''


def extract_steps(root, tmpdir):
    """⇒ (steps, err)。steps = [(scope, name, mode, dir, cmd)]；err 非空 ⇒ rc=2。"""
    gates = os.path.join(root, GATES_REL)
    if not os.path.isfile(gates):
        return [], "闸文件不在：%s（缺件 ⇒ 不给结论）" % GATES_REL
    helper = os.path.join(tmpdir, "gc-extract.sh")
    with open(helper, "w", encoding="utf-8") as fh:
        fh.write(_EXTRACT_HELPER)
    try:
        proc = subprocess.run(["/bin/bash", helper, root], stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE, timeout=600)
    except Exception as exc:  # noqa: BLE001 —— 跑不起来 = 判不了
        return [], "步骤表提取失败（%s）" % exc
    out = proc.stdout.decode("utf-8", "replace")
    err = proc.stderr.decode("utf-8", "replace")
    if proc.returncode != 0:
        tail = err.strip().splitlines()[-1] if err.strip() else "（stderr 为空）"
        return [], ("闸脚本 `--list` 非零退出（rc=%d）⇒ **不给结论**（判不了 = 不报绿也不报红）。"
                    "常见原因：闸自己的自检不过、或 precheck 缺件（precheck 要 "
                    "core/go.mod · agent/go.mod · shared/go.mod · scripts/exportnames/go.mod · "
                    "wall/Cargo.toml · ui/Cargo.toml · publish/whitelist.txt 七件）、或仓不完整；stderr 末行：%s"
                    % (proc.returncode, tail))
    steps = []
    #   ★ 记录用 NUL 分隔（`GAP-20260927-127`）：命令串里可能含**换行**（多行 add_step）⇒
    #   用 `\n` 作分隔会把一条记录切成几行、读侧只取到首行（脚本名若在第 2 行 ⇒
    #   误判成 ② 弱挂载：既不进 ① 也不进 ③，永久静默）。NUL 不出现在 shell 命令里，
    #   故改用 `\0` 作**记录分隔符**，命令原样保留。
    for rec in out.split("\0"):
        if rec.count("\t") < 4:
            continue
        parts = rec.split("\t", 4)
        if parts[1].startswith("自检-"):  # 合成步骤（闸自己的负控）不算真目标
            continue
        steps.append(tuple(parts))
    if not steps:
        return [], "步骤表为空（0 步）⇒ 空转，不给结论（若闸脚本被重构，请同步本脚本的提取器）"
    return steps, ""


# ══════════════════════════════════════════════════════════════════
# 1. config
# ══════════════════════════════════════════════════════════════════
def load_config(path):
    if not os.path.isfile(path):
        raise Unsupported("config 不在：%s（缺件 ⇒ 不给结论；本门没有内建默认值，"
                          "因为「基线/白名单」必须显式落盘才可审计）" % path)
    try:
        with open(path, encoding="utf-8") as fh:
            cfg = json.load(fh)
    except Exception as exc:  # noqa: BLE001
        raise Unsupported("config 不是合法 JSON：%s（%s）" % (path, exc))
    if not isinstance(cfg, dict):
        raise Unsupported("config 顶层必须是对象")
    bad = sorted(k for k in cfg if not k.startswith("_") and k not in ALLOWED_TOP)
    if bad:
        raise Unsupported("config 有本脚本未实现的顶层键：%s ⇒ 不给结论（schema 加了键、"
                          "门照绿 = 假覆盖）" % " · ".join(bad))
    scan = cfg.get("scan", {})
    if not isinstance(scan, dict):
        raise Unsupported("config.scan 必须是对象")
    bad = sorted(k for k in scan if not k.startswith("_") and k not in ALLOWED_SCAN)
    if bad:
        raise Unsupported("config.scan 有未实现的键：%s" % " · ".join(bad))
    excl = cfg.get("exclude", {})
    if not isinstance(excl, dict):
        raise Unsupported("config.exclude 必须是对象")
    bad = sorted(k for k in excl if not k.startswith("_") and k not in ALLOWED_EXCLUDE)
    if bad:
        raise Unsupported("config.exclude 有未实现的键：%s" % " · ".join(bad))
    wl = cfg.get("whitelist", {})
    if not isinstance(wl, dict):
        raise Unsupported("config.whitelist 必须是对象")
    bad = sorted(k for k in wl if not k.startswith("_") and k not in ALLOWED_WL)
    if bad:
        raise Unsupported("config.whitelist 有未实现的键：%s" % " · ".join(bad))
    for bucket in ("build_dirs", "scripts"):
        items = wl.get(bucket, [])
        if not isinstance(items, list):
            raise Unsupported("config.whitelist.%s 必须是数组" % bucket)
        for i, it in enumerate(items):
            tag = "whitelist.%s[%d]" % (bucket, i)
            if not isinstance(it, dict):
                raise Unsupported("%s 必须是对象" % tag)
            bad = sorted(k for k in it if not k.startswith("_") and k not in ALLOWED_WL_ITEM)
            if bad:
                raise Unsupported("%s 有未实现的键：%s" % (tag, " · ".join(bad)))
            p = it.get("path")
            if not isinstance(p, str) or not p.strip():
                raise Unsupported("%s 缺 path" % tag)
            if not isinstance(it.get("reason"), str) or not it["reason"].strip():
                raise Unsupported("%s（%s）**缺 reason** ⇒ 不给结论（写不清理由的豁免 = 洗白）"
                                  % (tag, p))
            d = it.get("date")
            if not isinstance(d, str) or not DATE_RE.match(d):
                raise Unsupported("%s（%s）**缺 date 或格式不是 YYYY-MM-DD**（现 %r）⇒ 不给结论"
                                  % (tag, p, d))
    bl = cfg.get("baseline")
    if not isinstance(bl, int) or isinstance(bl, bool) or bl < 0:
        raise Unsupported("config.baseline 必须是非负整数（现 %r）" % (bl,))
    return {
        "path": path,
        "baseline": bl,
        "baseline_note": cfg.get("_baseline_note", ""),
        "build_manifests": tuple(scan.get("build_manifests", DEFAULT_BUILD_MANIFESTS)),
        "script_globs": tuple(scan.get("script_globs", DEFAULT_SCRIPT_GLOBS)),
        "nosuffix_dir": scan.get("nosuffix_dir", DEFAULT_NOSUFFIX_DIR),
        "exclude_components": tuple(excl.get("path_components", DEFAULT_EXCLUDE_COMPONENTS)),
        "exclude_prefixes": tuple(excl.get("prefixes", DEFAULT_EXCLUDE_PREFIXES)),
        "wl_build": list(wl.get("build_dirs", [])),
        "wl_scripts": list(wl.get("scripts", [])),
    }


# ══════════════════════════════════════════════════════════════════
# 2. 扫描（A 构建清单目录 · B scripts 候选）
# ══════════════════════════════════════════════════════════════════
def _rel(root, p):
    return os.path.relpath(p, root).replace(os.sep, "/")


def scan_build_dirs(root, cfg):
    """⇒ [(rel_dir, [manifest…])]，已按硬排除剪枝。"""
    excl_c = set(cfg["exclude_components"])
    excl_p = [p.strip("/") for p in cfg["exclude_prefixes"]]
    out = []
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        rel = _rel(root, dirpath)
        rel = "" if rel == "." else rel
        kept = []
        for d in sorted(dirnames):
            if d in excl_c:
                continue
            child = ("%s/%s" % (rel, d)).lstrip("/")
            if any(child == p or child.startswith(p + "/") for p in excl_p):
                continue
            kept.append(d)
        dirnames[:] = kept
        found = sorted(m for m in cfg["build_manifests"] if m in filenames)
        if found:
            out.append((rel, found))
    return sorted(out, key=lambda t: t[0])


def scan_scripts(root, cfg):
    """⇒ (cands, nosuffix)。cands = [(rel_path, kind)]；kind ∈ {sh, py, nosuffix}。"""
    cands, seen = [], set()
    for pat in cfg["script_globs"]:
        for p in sorted(glob.glob(os.path.join(root, pat))):
            if not os.path.isfile(p):
                continue
            rel = _rel(root, p)
            if rel in seen:
                continue
            seen.add(rel)
            cands.append((rel, os.path.splitext(p)[1].lstrip(".") or "nosuffix"))
    ns_dir = os.path.join(root, cfg["nosuffix_dir"])
    #   ★ 2026-09-20（脚本分域第二段）：无后缀脚本下沉到 `scripts/<域>/` ⇒ 无后缀面随之下沉一层。
    #   旧口径只 `os.listdir(顶层)`，下沉后 `scripts/docs/edit-assert` 与 `scripts/gates/mutate-scan`
    #   会**静默掉出候选面**（覆盖变窄却不报红）。此处补「顶层 + 一层子目录」，
    #   跳过 NOSUFFIX_EXCL（与 check-wired-scripts.py 的 EXCL_SUBDIRS 同口径）。
    scan_dirs = []
    if os.path.isdir(ns_dir):
        scan_dirs.append(ns_dir)
        for fn in sorted(os.listdir(ns_dir)):
            sub = os.path.join(ns_dir, fn)
            if (os.path.isdir(sub) and not fn.startswith(".") and fn not in NOSUFFIX_EXCL
                    and not os.path.islink(sub)):
                scan_dirs.append(sub)
    nosuffix = []
    for sd in scan_dirs:
        for fn in sorted(os.listdir(sd)):
            p = os.path.join(sd, fn)
            if not os.path.isfile(p) or "." in fn:
                continue
            rel = _rel(root, p)
            if rel in seen:
                continue
            head = ""
            try:
                with open(p, "rb") as fh:
                    head = fh.readline(200).decode("utf-8", "replace")
            except OSError:
                pass
            if not (os.access(p, os.X_OK) or head.startswith("#!")):
                continue  # 无后缀但既不可执行也无 shebang ⇒ 不是脚本
            seen.add(rel)
            nosuffix.append(rel)
            cands.append((rel, "nosuffix"))
    return cands, nosuffix


# ══════════════════════════════════════════════════════════════════
# 3. 命中判定
# ══════════════════════════════════════════════════════════════════
_SPLIT = re.compile(r"[\s;|&()<>]+")


def tokens(cmd):
    out = []
    for t in _SPLIT.split(cmd):
        t = t.strip("\"'`")
        if t:
            out.append(t)
    return out


def _norm_ref(tok):
    """把命令串里的一个 token 归一成「目录引用」形态：去尾 /、去 ./、去尾 /..."""
    t = tok.strip("\"'`")
    t = re.sub(r"/\.\.\.$", "", t)
    t = re.sub(r"/\.$", "", t)
    t = t.rstrip("/")
    if t.startswith("./"):
        t = t[2:]
    return t


def dir_ref_in_cmd(rel_dir, cmd):
    """A-b / A-c：命令串里有没有点名这个目录（按路径边界；通配按 fnmatch）。"""
    for tok in tokens(cmd):
        if _norm_ref(tok) == rel_dir:
            return True
    for tok in tokens(cmd):
        if "*" in tok or "?" in tok:
            if (fnmatch.fnmatch(rel_dir, tok) or fnmatch.fnmatch(rel_dir + "/x", tok)
                    or fnmatch.fnmatch(rel_dir + "/x.go", tok)):
                return True
    return False


def abs_ref_in_cmd(abs_dir, cmd):
    """命令串里直接写了绝对路径（如 `cd /…/shared`）：按边界匹配绝对形态。"""
    i = cmd.find(abs_dir)
    while i != -1:
        after = cmd[i + len(abs_dir):i + len(abs_dir) + 1]
        if after == "" or after in "/ \t\"';|&":
            return True
        i = cmd.find(abs_dir, i + 1)
    return False


def judge_build_dir(root, rel_dir, steps):
    """⇒ (命中?, 证据串)。命中口径 A-a/A-b/A-c，逐条留证。"""
    abs_dir = os.path.normpath(os.path.join(root, rel_dir))
    for scope, name, _mode, sdir, cmd in steps:
        if os.path.normpath(sdir) == abs_dir:  # A-a 工作目录命中
            return True, "[%s] %s ⌁ 工作目录 = %s" % (scope, name, rel_dir)
    for scope, name, _mode, sdir, cmd in steps:
        if dir_ref_in_cmd(rel_dir, cmd) or abs_ref_in_cmd(abs_dir, cmd):  # A-b 路径参数
            return True, "[%s] %s ⌁ 命令串点名 %s" % (scope, name, rel_dir)
    return False, ""


_NAME_CACHE = {}


def name_re(name):
    r = _NAME_CACHE.get(name)
    if r is None:
        r = re.compile(r"(?<![\w.\-])" + re.escape(name) + r"(?![\w.\-])")
        _NAME_CACHE[name] = r
    return r


# ── 「真接线」面（命令位调用）—— 与门③ `check-wired-scripts.py:38-41` 同口径 ──────────
#   名字在闸文件里被提到 **不等于** 它接了线（缺口 `GAP-20260927-123` ①·治静默）。
#   只有落在**命令位**上才算「真接线」；口径三条排除（逐字对齐门③）：
#     ① 纯注释行（`#` 开头）—— 「注释里提过」不算；
#     ② 纯清单项（整行就是一个路径字面量，如 `"scripts/gates/<名>",`）—— 名册/排除表里
#        被点名也不算（「记录不能充作引用」）；
#     ③ 不在命令位（只被 echo 文案 / 说明文字 / grep 模式串提到）—— **只被提一下**。
#   启动器闭集 + 变量解释器（`${PY}` / `$PY` / `./`）；可带引号与路径前缀。
_RUNNER_RE = (r"\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*"
              r"|python3|python|bash|sh|zsh|dash|node|env|exec|time|nice|\./")
_PATH_PREFIX = r"(?:[\w.${}/-]*/)"
_CALL_CACHE = {}
_BARE_ITEM_CACHE = {}


def call_re(name):
    """「命令位调用」正则：启动器 + （选项旗标）* + 路径 + 名（与门③ `call_re` 同口径）。

    两种形态都收：带引号（`python3 \"$Z/scripts/gates/<名>\" \"$D\"`）· 不带引号；
    名字后面必须是分隔符（空白 / 引号 / `;` / `&` / `|` / `)` / 行尾），防前缀误命中。
    """
    r = _CALL_CACHE.get(name)
    if r is None:
        n = re.escape(name)
        head = r'(?:^|[\s"\'=(;|&])(?:' + _RUNNER_RE + r')\s+(?:-[^\s]+\s+)*'
        r = re.compile(head + r'"' + _PATH_PREFIX + r'*' + n + r'"'
                       + r'|' + head + _PATH_PREFIX + r'*' + n + r'(?=[\s"\';&|)]|$)')
        _CALL_CACHE[name] = r
    return r


def bare_item_re(name):
    """「纯清单项」正则：整行就是一个路径字面量 —— 被名册/排除表点名，不算接线。"""
    r = _BARE_ITEM_CACHE.get(name)
    if r is None:
        r = re.compile(r'^["\']?(?:\./)?(?:scripts/)?(?:[A-Za-z0-9_.\-]+/)*'
                       + re.escape(name) + r'["\']?,?$')
        _BARE_ITEM_CACHE[name] = r
    return r


def wired_mention(name, gates_text):
    """闸文件里有没有「真接线·命令位调用」形态（②a）？⇒ (True, 证据行) / (False, "")。"""
    rx_call, rx_bare = call_re(name), bare_item_re(name)
    for line in gates_text.splitlines():
        if name not in line:
            continue
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        if rx_bare.match(s):
            continue
        if rx_call.search(line):
            return True, s[:160]
    return False, ""


def text_mention(name, gates_text):
    """闸文件里有没有「只被提一下」形态（②b）—— 非命令位的任意出现（含注释行）。"""
    rx_bare, rx_call, rx = bare_item_re(name), call_re(name), name_re(name)
    for line in gates_text.splitlines():
        if name not in line:
            continue
        s = line.strip()
        if not s or rx_bare.match(s):
            continue
        if s.startswith("#"):
            if rx.search(line):
                return True, s[:160]
            continue
        if rx_call.search(line):
            continue
        if rx.search(line):
            return True, s[:160]
    return False, ""


_ZERG_GATE_CACHE = {}


def zerg_gate_re(stem):
    """`zerg gate check <件名去扩展名>` 命令位形（`GAP-20261001-74`）。

    ★ 光把 `zerg` 加进 `_RUNNER_RE` **不解**：`gate check` 两词夹在启动器与名之间 ⇒
      现有 `call_re` 结构上吃不进 ⇒ 另写专用规则（`gate check` 不是旗标）。
    """
    r = _ZERG_GATE_CACHE.get(stem)
    if r is None:
        head = r'(?:^|[\s"\'=(;|&])(?:[^\s]*/)?zerg\s+gate\s+check\s+(?:[^\s]*/)*'
        r = re.compile(head + re.escape(stem) + r'(?=[\s"\';|)&]|$)')
        _ZERG_GATE_CACHE[stem] = r
    return r


def zerg_name_covers(base, cmds_text):
    """命令串里有没有 `zerg gate check <base 去扩展名>` 形态（① in_steps 的等价档）。

    `GAP-20261001-74`：接线改造后命令形从 `python3 …/check-x.py` 变成
    `zerg gate check check-x` ⇒ 用**去扩展名**比较；否则 basename（含 `.py`）永不在
    命令串里 ⇒ 该件假掉 ③ 档（自指死结）。
    """
    stem = os.path.splitext(base)[0]
    if not stem:
        return False
    for line in cmds_text.splitlines():
        if stem not in line:
            continue
        if zerg_gate_re(stem).search(line):
            return True
    return False


def script_tier(rel, gates_text, cmds_text):
    """① in_steps · ②a wired · ②b mention · ③ unmentioned。

    ★ **不再「闸文件全文出现即算」**（缺口 `GAP-20260927-123`）：② 拆成两档 ——
      · **②a wired（真接线·弱挂载）**：名字落在闸文件的**命令位**上（启动器 + 路径 + 名），
        即它**真被闸体调用**（只是不走 add_step 命令串）；
      · **②b mention（只被提一下）**：名字只在注释 / 文案 / 清单项里出现 —— **不算接线**。
    ★ 计数自证不变：① + ② + ③ ≡ 候选总数（其中 ② = ②a + ②b）。
    """
    name = os.path.basename(rel)
    if name_re(name).search(cmds_text) or zerg_name_covers(name, cmds_text):
        return "in_steps"
    if wired_mention(name, gates_text)[0]:
        return "wired"
    if text_mention(name, gates_text)[0]:
        return "mention"
    return "unmentioned"


def glob_hit(rel, steps):
    """第四档：只被通用语法步的通配兜住（语法过 ≠ 接了门 ⇒ 不计入 ①②③）。"""
    for _scope, _name, _mode, _sdir, cmd in steps:
        for tok in tokens(cmd):
            if ("*" in tok or "?" in tok) and fnmatch.fnmatch(rel, tok):
                return True
    return False


def wl_match(rel, entries):
    """白名单命中：路径相等，或位于该路径之下（目录型条目）。"""
    for e in entries:
        p = e["path"].strip("/")
        if rel == p or rel.startswith(p + "/"):
            return e
    return None


def tracked_files(root):
    """`git ls-files`（相对仓根）的集合 —— **新件 / 存量债** 的判据面。

    ★ 为什么用「在不在 git 索引里」当新件判据：本门开张前就在盘上的脚本 = **存量债**
      （已提交、走 `baseline` 棘轮，只许下调）；**本门开张后新建**的脚本一旦落 ③ 档，
      说明它**没有与代码同批挂门** —— 那条纪律（《缺口总账》`Q-175`）在面上必须是**红**，
      而不是「只要没超基线就绿」。
    ★ 判不了就当**判不了**：非 git 仓 / 无 git / 命令失败 ⇒ 返回 `None`
      （调用方**不按新件论处**，只在报告里照实记一句）—— 绝不把「判不了」读成「全部是存量」。
    ★ **判不了 ⇒ 不给结论**（`GAP-20260927-105`）：③ 档非空时，`None` 会把每一件都写成
      「存量债（已提交件 …）」—— 那句**事实断言是假的**，且「新件未挂门」恒 0 ⇒ 新件在基线余额内
      **静默洗白**（删一条存量债 + 加一条新件 ⇒ 命中数不变 ⇒ rc=0）。故 `main()` 里
      **③ 档非空 ∧ `git_tracked_known=False` ⇒ rc=2（不给结论）**；③ 档为空时 ④ 格子不承重，
      照旧给结论（判不了才停，不误报）。
    """
    try:
        proc = subprocess.run(["git", "-C", root, "ls-files", "-z"],
                              stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    except (OSError, ValueError):
        return None
    if proc.returncode != 0:
        return None
    out = proc.stdout.decode("utf-8", "replace")
    return {p for p in out.split("\0") if p}


# ══════════════════════════════════════════════════════════════════
# 4. 汇总 / 报告
# ══════════════════════════════════════════════════════════════════
def counts_certify(a_covered, a_wl, a_red, a_items, b_counts, cands):
    """计数自证 —— 两条恒等式：**三列表之和 == 总项数** ∧ **三档之和 == 候选数**。

    ★ 为什么必须是一个**独立函数**（`GAP-20260928-11`）：这两条恒等式原先只在
      `analyse()` 里就地算成一个 bool —— 判据与构造同源（三列表由 `:535-540` 三分支穷尽、
      三档由 `script_tier` 三个 `return` 穷尽）⇒ **自我重述、恒真**，没有任何输入能把它置假，
      `:706` 的「✗ ⇒ rc=2」成了死码、`:1021` 的自检只断言输出里有那两行字面串。
      抽成这一处**唯一口径**之后：`analyse()` 与 `--self-test` 的成对负控**同走本函数** ——
      自检里「三列表之和 ≠ 总项数 / 三档之和 ≠ 候选数」的合成输入置假的，就是真判据本身。
    ★ 口径一字未放宽（仍是这两条恒等式），也不另开第三份计数口径。
    """
    return (len(a_covered) + len(a_wl) + len(a_red) == len(a_items)
            and sum(b_counts.values()) == len(cands))


def baseline_note_restated(note):
    """从 `_baseline_note` 取**末次现值句**声称的现值（`下调成 <N>` 的最后一个 N）。
    取不到 ⇒ None（本门只认这一种可机读的重述形态，✗ 不猜别的写法）。"""
    hits = re.findall(r"下调成\s*(\d+)", note or "")
    return int(hits[-1]) if hits else None


def baseline_note_drift(baseline, note):
    """`_baseline_note` 的末次现值句必须 == `baseline`（同源 · `GAP-20260927-42`）。

    ★ 为什么要这条（旧洞）：`_baseline_note` 有多个消费点（人读报告 / `--json` / 文档引它当
      出处 / 契约注册表），但**全仓零处做 note 与 baseline 的同值校验** —— 唯一写入口
      `baseline-ratchet.py --write` 只改 `"baseline": <数字>`、自检还逐字节断言「只改那一个
      数字」⇒ note 落后一次下调也**没有任何门会响**。本函数就是那颗牙。
    返回 (ok, detail)。★ note 无「下调成 N」重述句 ⇒ 视为无重述（ok，不误红）。"""
    got = baseline_note_restated(note)
    if got is None:
        return True, "note 无现值重述句（不判）"
    if got == baseline:
        return True, "note 现值句 %d == baseline %d" % (got, baseline)
    return False, "note 现值句 %d ≠ baseline %d（%+d）" % (got, baseline, got - baseline)


def analyse(root, cfg, steps, gates_text, max_examples):
    cmds_text = "\n".join(s[4] for s in steps)

    # ── A ──────────────────────────────────────────────────────────
    a_items, a_red, a_wl, a_covered = [], [], [], []
    for rel_dir, charts in scan_build_dirs(root, cfg):
        hit, why = judge_build_dir(root, rel_dir, steps)
        wl = wl_match(rel_dir, cfg["wl_build"])
        item = {"dir": rel_dir, "manifests": charts, "covered": hit, "why": why,
                "whitelist": (wl["reason"] if wl else None), "whitelist_date": (wl["date"] if wl else None)}
        a_items.append(item)
        if hit:
            a_covered.append(item)
        elif wl:
            a_wl.append(item)
        else:
            a_red.append(item)

    # ── B ──────────────────────────────────────────────────────────
    cands, nosuffix = scan_scripts(root, cfg)
    tracked = tracked_files(root)                       # None = 判不了（非 git 仓）⇒ ③ 档非空时 rc=2
    b_items, b_red, b_wl = [], [], []
    buckets = {"in_steps": [], "mentioned": [], "unmentioned": []}
    wired_items, mention_items, new_mention = [], [], []
    glob_note = []
    for rel, kind in cands:
        tier = script_tier(rel, gates_text, cmds_text)
        wl = wl_match(rel, cfg["wl_scripts"])
        item = {"script": rel, "kind": kind, "tier": tier,
                "glob": glob_hit(rel, steps),
                "whitelist": (wl["reason"] if wl else None),
                "whitelist_date": (wl["date"] if wl else None),
                "new": False, "reason": ""}
        b_items.append(item)
        if tier == "in_steps":
            buckets["in_steps"].append(item)            # ① 真被某步跑（add_step 命令串）
        elif tier in ("wired", "mention"):
            buckets["mentioned"].append(item)           # ② = ②a wired + ②b mention（计数自证不变）
            if tier == "wired":
                wired_items.append(item)                # ②a 真接线（闸体命令位调用）
                item["reason"] = "真接线·弱挂载（闸文件命令位调用 · 未走 add_step 命令串）"
            else:
                mention_items.append(item)              # ②b 只被提一下（注释/文案/清单项）
                item["reason"] = "只被提一下（注释/文案/清单项 · **不算接线**）"
                # ★ 治静默（`GAP-20260927-123` ①）：**只被提一下**的**新件**（不在 git 索引）
                #   也属「新件未挂门」——旧口径里它落 ② ⇒ 既不进棘轮也不进 ④ ⇒ 永久静默。
                if tracked is not None and rel not in tracked:
                    item["new"] = True
                    item["reason"] = ("新件只被提一下、未真接线（不在 git 索引 ⇒ 新建后没与"
                                      "代码同批挂 add_step）")
                    new_mention.append(item)
        else:
            buckets["unmentioned"].append(item)         # ③ 闸文件里查无此名 = 棘轮命中来源
            if wl:
                b_wl.append(item)                       # 第二档白名单（不计入棘轮）
            else:
                # ④ 新格：**新件**（不在 git 索引 = 本门开张后新建、且未与代码同批挂门）
                #    与 **存量债**（已提交件 · 走 baseline 棘轮）**分开** —— 存量债是「还」，
                #    新件是「不许再欠」（《缺口总账》`Q-175` ① / ②）。
                item["new"] = bool(tracked is not None and rel not in tracked)
                item["reason"] = ("新件未挂门（不在 git 索引 ⇒ 新建后没与代码同批挂 add_step）"
                                  if item["new"] else
                                  "存量债（已提交件 · 未挂 add_step · 走基线棘轮，只许下调）")
                b_red.append(item)                      # ③ 棘轮命中数 = 查无此名 且 不在白名单
        if item["glob"]:
            glob_note.append(item)

    # ④ 新格（单列成格）：「新件」那一条 —— 与存量债分开点名、分开判。
    #   ★ 来源两处：③ 档里未挂门的新件 + **②b 只被提一下**的新件（治静默 · `GAP-20260927-123`）。
    new_ungated = [it for it in b_red if it["new"]] + new_mention
    legacy_ungated = [it for it in b_red if not it["new"]]

    # ── 例外面计数（第二档：白名单）────────────────────────────────
    stale = []
    for bucket, entries in (("build_dirs", cfg["wl_build"]), ("scripts", cfg["wl_scripts"])):
        for e in entries:
            p = e["path"].strip("/")
            if bucket == "build_dirs":
                exists = os.path.isfile(os.path.join(root, p, "go.mod")) or \
                         os.path.isfile(os.path.join(root, p, "Cargo.toml"))
            else:
                exists = os.path.isfile(os.path.join(root, p))
            if not exists:
                stale.append((bucket, p, e["reason"], e["date"]))

    rep = {
        "root": root, "config": cfg["path"], "steps": len(steps),
        "a": {"items": a_items, "covered": a_covered, "whitelisted": a_wl, "red": a_red},
        "b": {"cands": b_items, "counts": {k: len(v) for k, v in buckets.items()},
              "tier1": buckets["in_steps"], "tier2": buckets["mentioned"], "tier3": buckets["unmentioned"],
              "tier2_wired": wired_items, "tier2_mention": mention_items,
              "hit": len(b_red), "hit_items": b_red,
              "new_ungated": new_ungated, "legacy_ungated": legacy_ungated,
              "git_tracked_known": tracked is not None,
              "whitelisted": b_wl, "glob_only": glob_note, "nosuffix": nosuffix},
        "baseline": cfg["baseline"], "baseline_note": cfg["baseline_note"],
        "stale_whitelist": stale,
        "wl_counts": {"build_dirs": len(cfg["wl_build"]), "scripts": len(cfg["wl_scripts"])},
        "exclude_components": list(cfg["exclude_components"]),
        "exclude_prefixes": list(cfg["exclude_prefixes"]),
        "max_examples": max_examples,
    }
    rep["a"]["empty"] = len(a_items) == 0
    rep["b"]["empty"] = len(cands) == 0
    rep["certify"] = counts_certify(a_covered, a_wl, a_red, a_items, rep["b"]["counts"], cands)
    return rep


def declares_out_of_scope(root, rel):
    """件头自述「不进门禁步骤表」？⇒ True / False（读不到 ⇒ False：**不猜**）。

    ★ 分类**只此一处**（`OOR_DECL` + 本函数）。判据面 = 件自己的件头自述，本门只**照读**；
      不靠关键字猜「会不会写盘」（那是自造判据，必出误判）。
    ★ 只读前 `DECL_SCAN_LINES` 行：自述按体例都写在件头（`check-outrepo-dead-refs.py:19`），
      件体中段的引用（如该件 `:229` 的打印行）不算自述 ⇒ 上界就是「件头」这个口径。
    ★ **不进任何计数 / 不进退码 / 不动覆盖分母**：只决定 ④ 格那句**指路**怎么分两路。
    """
    try:
        with open(os.path.join(root, rel), encoding="utf-8", errors="replace") as fh:
            head = fh.read(200000)
    except OSError:
        return False
    return any(OOR_DECL in ln for ln in head.splitlines()[:DECL_SCAN_LINES])


def _list(items, key, max_examples, indent="     ", mark_wl=False, mark_reason=False):
    lines = []
    for it in items[:max_examples]:
        extra = ""
        if mark_wl and it.get("whitelist"):
            extra = "  ← 白名单豁免 %s（%s）" % (it["whitelist"], it.get("whitelist_date", ""))
        elif mark_reason and it.get("reason"):
            extra = "  ← %s" % it["reason"]
        lines.append("%s- %s%s" % (indent, it[key], extra))
    if len(items) > max_examples:
        lines.append("%s… 另 %d 条（--max-examples 调整）" % (indent, len(items) - max_examples))
    return lines


def print_report(rep):
    a, b = rep["a"], rep["b"]
    print("── 门覆盖自检 · 仓根 %s ──" % rep["root"])
    print("   步骤表来源：%s（%d 步，已剔除合成自检步）" % (GATES_REL, rep["steps"]))
    print("   config：%s（baseline=%d）" % (rep["config"], rep["baseline"]))
    print()

    # ── A ──
    print("== A 构建清单目录必须被某步收进（阻断）==")
    print("   候选（含 go.mod / Cargo.toml 的目录，已剪硬排除）：%d" % len(a["items"]))
    print("     ① 已被某步收进：%d" % len(a["covered"]))
    for ln in _list(a["covered"], "dir", min(rep["max_examples"], 12)):
        print(ln)
    print("     ② 白名单豁免（第二档 · 计数可见）：%d" % len(a["whitelisted"]))
    for ln in _list(a["whitelisted"], "dir", rep["max_examples"], mark_wl=True):
        print(ln)
    print("     ③ **未收进且不在白名单 = 红：%d**" % len(a["red"]))
    for ln in _list(a["red"], "dir", rep["max_examples"]):
        print(ln)
    if a["red"]:
        print("     ⇒ 处置：**真修**（给该目录补一步 add_step，工作目录指到它）"
              "或**白名单写清 reason+date**；不许为了让本门变绿而填白名单。")
    print()

    # ── B ──
    print("== B scripts 脚本必须在步骤表或白名单（阻断 · 基线棘轮）==")
    kinds = {}
    for it in b["cands"]:
        kinds[it["kind"]] = kinds.get(it["kind"], 0) + 1
    print("   候选总数：%d（%s）" % (len(b["cands"]),
                                   " · ".join("%s %d" % (k, v) for k, v in sorted(kinds.items()))))
    print("     ① add_step 命令串里点名：%d" % b["counts"]["in_steps"])
    for ln in _list(b["tier1"], "script", rep["max_examples"]):
        print(ln)
    print("     ② 闸文件里被提到、但不在任何 add_step 命令串里（弱挂载 · 单列）：%d"
          % b["counts"]["mentioned"])
    print("        · ②a **真接线**（闸体命令位调用 · 与门③同口径）：%d" % len(b["tier2_wired"]))
    for ln in _list(b["tier2_wired"], "script", rep["max_examples"], indent="          "):
        print(ln)
    print("        · ②b **只被提一下**（注释/文案/清单项 · **不算接线** · `GAP-20260927-123`）：%d"
          % len(b["tier2_mention"]))
    for ln in _list(b["tier2_mention"], "script", rep["max_examples"], indent="          ",
                    mark_reason=True):
        print(ln)
    print("     ③ 闸文件里查无此名：%d" % b["counts"]["unmentioned"])
    print("        · 其中白名单豁免（第二档 · 计数可见）：%d" % len(b["whitelisted"]))
    for ln in _list(b["whitelisted"], "script", rep["max_examples"], mark_wl=True):
        print(ln)
    print("        · 其中**棘轮命中数**（= ③ − 白名单，与 baseline 比）：%d" % b["hit"])
    rem = b["hit"] - rep["baseline"]
    if rem > 0:
        print("          ✗ **超过基线 %d 条**（命中 %d > 基线 %d）⇒ 红"
              % (rem, b["hit"], rep["baseline"]))
    else:
        print("          （命中 %d ≤ 基线 %d ⇒ 不红，**剩余 %d 条**；"
              "每批下调基线见 --baseline）" % (b["hit"], rep["baseline"], b["hit"]))
    # ★ ③ 档**逐件点名**（总数 + 名单 + 每件一行 reason）—— 存量债 / 新件分开写，
    #   不再只给一个数（数看不出「哪一件」⇒ 只能人开闸文件全文对，见 `Q-175`）。
    print("        · **③ 档逐件点名（%d 条 · 每件一行 reason）**：" % len(b["hit_items"]))
    for it in b["hit_items"]:
        print("          - %s  ← %s" % (it["script"], it["reason"]))
    print("     ④ **新件未挂门**（本门开张后新建、且没与代码同批挂 add_step）= **%d**"
          % len(b["new_ungated"]))
    if not b["git_tracked_known"]:
        print("        （★ 判不了：仓根不是 git 仓 / git 不可用 ⇒ **不按新件论处**、"
              "本格只作照实记录 —— 判不了的时候报绿报红都是猜）")
        print("        ⇒ **判不了 ⇒ 不给结论（rc=2）**：③ 档 %d 条分不开「新件 / 存量债」；"
              "③ 档为空（本格不承重）时才照旧给结论" % len(b["hit_items"]))
    for it in b["new_ungated"]:
        print("          ✗ %s  ← %s" % (it["script"], it["reason"]))
    if b["new_ungated"]:
        print(HINT_ADD_STEP)
        # ── 分流（排队项「面⑥ 失败文案仍红」）：上面那句只对「可入步骤表」的件成立。
        #   另一类件照它做是死路 —— 语料在仓外（本机个人语料）/ 会写盘的工具：挂 `add_step`
        #   会在「没这棵树」「每次提交都改工作树」的机器上恒红（门禁反向危害），而这类件的
        #   既有去处就是第二档白名单（config 口径 + 四条先例）。
        #   ★ 只加字：不改三档计数 / 不进退码 / 不动覆盖分母；普通件那句一字未动。
        oor = [it for it in b["new_ungated"]
               if declares_out_of_scope(rep["root"], it["script"])]
        print("        ⇒ **若不是「可入步骤表」的件，上面那路是死路**（原来只给这一条指路）："
              "语料在**仓外**（本机个人语料）或**会写盘**的件 —— 本机可用 ≠ 普遍可用 —— "
              "挂 `add_step` 会在「没有这棵树 / 每次提交都改工作树」的机器上**恒红**"
              "（门禁反向危害）⇒ 这类件改走**第二档白名单**（真走得通的那一路）：")
        print("          `scripts/gates/gate-coverage.config.json` 的 `whitelist.scripts` 加一条"
              " `{\"path\": \"<件仓内相对路径>\", \"reason\": \"<件头自述的理由>\", "
              "\"date\": \"<YYYY-MM-DD>\"}`（缺 reason 或缺 date ⇒ rc=2，不是绿）；"
              "先例四条 = `scripts/dev/atomic-write.sh` · `scripts/dev/writer-lock.sh` · "
              "`scripts/gates/check-writer-guard.py` · `scripts/docs/check-outrepo-dead-refs.py`"
              "（`whitelist.build_dirs` 同规）；**两路都不许抬 `baseline`**。")
        if oor:
            print("          ★ 本格**件头自述「%s」**那一类（照读件头 · 唯一分类处）"
                  "逐件点名（%d 条 ⇒ 走第二路）：" % (OOR_DECL, len(oor)))
            for it in oor:
                print("            - %s  ← 修法 = `whitelist.scripts` 写 `path`+`reason`+`date`，"
                      "**不许**挂 `add_step`（挂上去就是死路），也**不许**抬 `baseline`。"
                      % it["script"])
    print("     注：另 %d 条被通用语法步的 glob 兜住（`scripts/*.sh|*.py` 语法）——"
          "**语法过 ≠ 接了门，不计入 ①②③**（第四档）" % len(b["glob_only"]))
    print()

    # ── 自证 / 例外面 ──
    print("== 计数自证 ==")
    ok = rep["certify"]
    print("   A：收进 %d + 白名单 %d + 红 %d = %d（候选 %d）%s"
          % (len(a["covered"]), len(a["whitelisted"]), len(a["red"]),
             len(a["covered"]) + len(a["whitelisted"]) + len(a["red"]), len(a["items"]),
             "✓" if len(a["covered"]) + len(a["whitelisted"]) + len(a["red"]) == len(a["items"]) else "✗"))
    print("   B：① %d + ② %d + ③ %d = %d（候选 %d）%s"
          % (b["counts"]["in_steps"], b["counts"]["mentioned"], b["counts"]["unmentioned"],
             sum(b["counts"].values()), len(b["cands"]),
             "✓" if sum(b["counts"].values()) == len(b["cands"]) else "✗"))
    if not ok:
        print("   ✗ 三档之和 ≠ 候选总数 ⇒ **计数自证不过** ⇒ 不给结论（rc=2）")
    if rep["stale_whitelist"]:
        print("   ⚠ 悬空白名单（写了 reason/date 但盘上查无此件，**不影响退码**）：%d 条"
              % len(rep["stale_whitelist"]))
        for bucket, p, reason, date in rep["stale_whitelist"][:rep["max_examples"]]:
            print("     - [%s] %s（%s）" % (bucket, p, date))
    print("   硬排除（第一档，不进候选）：%s（前缀：%s）"
          % (" · ".join(sorted(rep["exclude_components"])), " · ".join(rep["exclude_prefixes"])))
    print()


def print_baseline(rep):
    a, b = rep["a"], rep["b"]
    n = b["hit"]
    print("── 基线视图（只读 · 不改任何文件）──")
    print("   ③ 实测命中数（闸文件里查无此名的脚本数） = %d" % n)
    print("   config baseline                          = %d" % rep["baseline"])
    print("   剩余（命中数）                            = %d" % n)
    if n > rep["baseline"]:
        print("   ⇒ **超额 %d 条 ⇒ rc=1**；处置：接线（写进 add_step）或补白名单 reason+date，"
              "**不许把基线抬上去**" % (n - rep["baseline"]))
    else:
        print("   ⇒ 不超额 ⇒ rc=0（但剩余数必须打印，不许悄悄绿）")
    print("   建议值 = %d（= 今日实测；若已接线若干条 ⇒ 实测值会自己降到该数）" % n)
    if rep["baseline_note"]:
        print("   基线自述 = %s" % rep["baseline_note"])
    print()
    print("号令字句（每批下调基线时照抄，一次一批，**只许下调、永不上调**）：")
    print("  「批 〈批次名〉：本批把下列 〈N〉 条从「闸文件查无此名」改成「步骤表里点名」："
          "〈逐条脚本名〉。")
    print("    依据 = `python3 scripts/gates/check-gate-coverage.py --baseline` 的 ③ 实测命中数"
          "（改前/改后各跑一次）；")
    print("    动作 = 把 `scripts/gates/gate-coverage.config.json` 的 `baseline` 由 %d 改成"
          " %d（**只改这一个数字**）；" % (rep["baseline"], n))
    print("    规矩 = 一次只许下调、永不上调；实测值比基线高（= 又冒出新脚本）⇒ 先接线或补白名单理由，"
          "不许抬基线。」")


def print_list_rules():
    print("check-gate-coverage.py 规则表（门覆盖自检）")
    print()
    print("A [FAIL·阻断] 构建清单目录必须被某步收进")
    print("   候选 = 全仓含 %s 的目录（剪掉第一档硬排除）" % " / ".join(DEFAULT_BUILD_MANIFESTS))
    print("   命中口径（三条，任一成立）：")
    print("     A-a 工作目录命中：某步的 add_step 工作目录 == 该构建目录")
    print("     A-b 命令串路径参数命中：命令串里点名该目录（按路径边界：shared / ./shared /")
    print("         cd shared / shared/... 都算；ui 不许被 build 之类的子串蹭到）")
    print("     A-c 通配命中：命令串里的 shell 通配 token 经 fnmatch 后覆盖该目录")
    print("   ★ 口径写死：上级目录的 ./... 不收下级嵌套 module（Go 的 ./... 不跨 go.mod 边界）")
    print("   ★ 今日应真红：ui/ · shared/ · scripts/exportnames/")
    print()
    print("B [FAIL·阻断·基线棘轮] scripts 脚本必须要么在步骤表、要么在白名单")
    print("   候选 = %s + %s/ 下无后缀可执行件（有执行位或 shebang）"
          % (" / ".join(DEFAULT_SCRIPT_GLOBS), DEFAULT_NOSUFFIX_DIR))
    print("   ① add_step 命令串里点名（真被某步跑） · ② 闸文件里被提到但不在任何 add_step 命令串里"
          "（弱挂载，单列） · ③ 闸文件查无此名 = 棘轮命中数")
    print("   ★ ② 拆两档（`GAP-20260927-123`）：**②a 真接线** = 名字落在闸文件**命令位**上"
          "（启动器 python3/bash/${PY}/./ + 路径 + 名；与门③ `check-wired-scripts.py` 同口径）；")
    print("     **②b 只被提一下** = 只在注释 / 文案 / 清单项里出现 —— **不算接线**；②b 里"
          "**新件**（不在 git 索引）落 ④「新件未挂门」⇒ rc=1（治静默）。")
    print("   ★ 计数自证：① + ② + ③ ≡ 候选总数")
    print("   ★ glob 兜底（scripts/*.sh|*.py 语法步）**不计入** ①②③ —— 语法过 ≠ 接了门")
    print("   ★ 命中数 > baseline ⇒ rc=1；≤ baseline ⇒ rc=0 但必须打印剩余数")
    print()
    print("例外表（四档）")
    print("   一 硬排除（config.exclude）   不进候选集（结构级，不是「放过」）")
    print("   二 白名单（config.whitelist） 逐条 reason + date；缺任一 ⇒ rc=2；计数可见")
    print("   三 基线棘轮（config.baseline）已登记存量额度；超一条即红；只许下调")
    print("   四 通用语法 glob 兜底        只证明语法过了；单列；不计入 B 的 ①②③")
    print()
    print("退码（三档 · 优先级 2 > 1 > 0）")
    print("   0 = 全绿（A 无红；B 命中 ≤ 基线）")
    print("   1 = 有红（A 有未收进目录 且 不在白名单；或 B 命中 > 基线）")
    print("   2 = 不给结论（用法错 / 缺件：config 或闸文件不在 / config 有未实现键 / 白名单条目")
    print("       缺 reason 或 date / 空转（候选集为 0 = 假覆盖）/ 计数自证不过 / 自检未过 /")
    print("       **判不了**：仓根不是 git 仓 且 ③ 档非空 ⇒ 「新件 ⟷ 存量债」分不开）")
    print()
    print("`--json` 契约（机读面）")
    print("   ★ `--json` ⇒ **stdout 只有 JSON**（可 json.loads）：自检文本全部改走 stderr，"
          "**不必**再配 --no-self-test")
    print("   ★ `--json` 与 `--self-test` / `--list-rules` 互斥（它们不产 JSON）⇒ 用法错 rc=2")
    print()
    print("与既有门的接口（第二波挂接）：")
    print('   add_step cov "门覆盖自检（构建清单目录 A + 脚本接线 B）" tri "${REPO_ROOT}" \\')
    print('            "python3 scripts/gates/check-gate-coverage.py"')


# ══════════════════════════════════════════════════════════════════
# 5. 自检（成对负控：漏一个目录必红 ↔ 补齐必绿 · 命中>基线必红 ↔ ≤基线必绿 ·
#          白名单缺 reason ⇒ rc=2 · 空转 ⇒ rc=2（绝不报绿））
# ══════════════════════════════════════════════════════════════════
SELF_PATH = os.path.abspath(__file__)
_TMPDIRS = []


def _mkroot(base):
    d = tempfile.mkdtemp(prefix=base, dir=base)
    _TMPDIRS.append(d)
    return d


def _write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)


def _fixture(base, name, *, steps=(), manifests=("core/go.mod",), scripts=("scripts/ok.py",),
             config=None, gates_extra="", no_gates=False, config_raw=None):
    """造一个最小仓：scripts/gates/precommit-gates.sh（合成步骤表）+ 构建清单 + 脚本 + config。"""
    root = os.path.join(base, name)
    os.makedirs(root, exist_ok=True)
    for m in manifests:
        _write(os.path.join(root, m), "module fixture\n" if m.endswith("go.mod") else "[package]\n")
    for s in scripts:
        _write(os.path.join(root, s), "#!/usr/bin/env python3\nprint('fixture')\n")
    if not no_gates:
        lines = ["#!/usr/bin/env bash", "set -u",
                 "# fixture：本文件 scripts/gates/precommit-gates.sh 是合成件"
                 "（仅供 check-gate-coverage.py 自检用）",
                 'REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"',
                 "STEP_SCOPE=(); STEP_NAME=(); STEP_MODE=(); STEP_DIR=(); STEP_CMD=()",
                 'add_step() { STEP_SCOPE+=("$1"); STEP_NAME+=("$2"); STEP_MODE+=("$3");'
                 ' STEP_DIR+=("$4"); STEP_CMD+=("$5"); }',
                 "clear_steps() { STEP_SCOPE=(); STEP_NAME=(); STEP_MODE=(); STEP_DIR=(); STEP_CMD=(); }",
                 "build_steps() {"]
        for d, cmd in steps:
            lines.append('  add_step fixture "步骤 %s" rc "${REPO_ROOT}/%s" "%s"' % (d or "root", d, cmd))
        lines += ["}", 'main() { clear_steps; build_steps; }', 'main "$@"', gates_extra]
        _write(os.path.join(root, "scripts", "gates", "precommit-gates.sh"), "\n".join(lines) + "\n")
    wl = {"build_dirs": [], "scripts": []}
    if config:
        wl = config
    if config_raw is not None:
        _write(os.path.join(root, "scripts", "gates", "gate-coverage.config.json"), config_raw)
    else:
        _write(os.path.join(root, "scripts", "gates", "gate-coverage.config.json"),
               json.dumps({"schema": "gate-coverage/1", "baseline": 0, "whitelist": wl},
                          ensure_ascii=False, indent=2))
    return root


def _gitify(root):
    """把夹具变成「索引已知」形态：真 `git init` + `git add -A`（**不提交**）。

    ★ ④/⑤/⑯/⑰ 判的是「新件 ⟷ 存量债」，前提是 `git ls-files` 读得到 ⇒ 夹具必须**有索引**；
      没有索引的夹具是 `GAP-20260927-105` 那条「`tracked=None` ⇒ 判不了」的形态，现在归 rc=2
      （见自检 ⑯b/⑯c 成对负控）。
    """
    subprocess.run(["git", "init", "-q"], cwd=root,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(["git", "add", "-A"], cwd=root,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def _run(root):
    proc = subprocess.run([PY, SELF_PATH, "--no-self-test", "--root", root],
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return proc.returncode, proc.stdout.decode("utf-8", "replace") + proc.stderr.decode("utf-8", "replace")


# ── stdout 纯度探针（⑱/⑲/⑳）：真命令行 + 真退出码，只看**流的归属** ──────────────
#   薄启动器：import 本门 → 剪掉探针自己（防自嵌套）→ 走真 main()。**不是**第二份实现：
#   参数解析、自检文本的打印与分流、扫描与退码，全是被测文件本身那一份。
_JSON_PROBE_LAUNCHER = r'''#!/usr/bin/env python3
import importlib.util, sys
spec = importlib.util.spec_from_file_location("gc_probe", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
m._SELFTEST_DEEP = False
sys.exit(m.main([a for a in sys.argv[2:]]))
'''


def _probe(base, argv):
    """跑一次**真命令行** ⇒ (rc, stdout_bytes, stderr_bytes)。"""
    launcher = os.path.join(base, "gc-probe-%d.py" % len(_TMPDIRS))
    _write(launcher, _JSON_PROBE_LAUNCHER)
    proc = subprocess.run([PY, launcher, SELF_PATH] + list(argv),
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=600)
    return proc.returncode, proc.stdout, proc.stderr


def _stdout_json_purity(raw):
    """纯度判据 ⇒ (ok, 点名)。ok=False 时**逐字点名** stdout 首行（前 60 字）作证。"""
    try:
        json.loads(raw.decode("utf-8"))
        return True, ""
    except Exception:  # noqa: BLE001 —— 解析不了就是「不是 JSON」，不猜
        lines = [l for l in raw.decode("utf-8", "replace").splitlines() if l.strip()]
        return False, (lines[0][:60] if lines else "（stdout 为空 ⇒ 也不是 JSON）")


def self_test(sink=None, deep=None):
    # ★ 参数名**不能叫 `out`**：函数体里的既有断言都用 `rc, out = _run(...)` ⇒ 会把流变量冲成 str。
    if sink is None:
        sink = sys.stdout
    if deep is None:
        deep = _SELFTEST_DEEP
    base = tempfile.mkdtemp(prefix="gc-selftest-")
    _TMPDIRS.append(base)
    cases = []

    def check(desc, ok, extra=""):
        cases.append((desc, bool(ok), extra))

    # ── 好件：core 被某步收进 + ok.py 在步骤表里 + baseline 0 ⇒ 必绿 ─────────
    #   ★ 2026-09-20（脚本分域第二段）：夹具的闸文件随真仓下沉到
    #   `scripts/gates/precommit-gates.sh` ⇒ 它不再落进 `scripts/*.sh` 候选面。
    #   原来靠它凑出的「② 1」没了，故**显式补一只弱挂载脚本**（weak.py：只在闸文件里被提到、
    #   不接进任何 add_step）——「计数自证 ① 1 + ② 1 + ③ 0 = 2」这条恒等式的**强度不变**
    #   （仍同时覆盖 ① 与 ② 两档），只是不再依赖夹具闸文件的落点。
    good = _fixture(base, "good", steps=[("core", "go build ./..."), ("", "python3 scripts/ok.py")],
                    scripts=("scripts/ok.py", "scripts/orphan.py"),
                    gates_extra="# 只提到 scripts/orphan.py，不接进任何一步")
    rc, out = _run(good)
    check("① 好件必绿（目录被收进 + 脚本在表里）⇒ rc=0", rc == 0, "rc=%d" % rc)

    # ── 成对负控 A：抽掉 core 那一步 ⇒ 必红且点名 core ─────────────────────
    miss = _fixture(base, "a-missing", steps=[("", "python3 scripts/ok.py")])
    rc, out = _run(miss)
    check("② A 漏掉一个目录 ⇒ rc=1", rc == 1, "rc=%d" % rc)
    check("② 且**点名**那个目录（core）", "core" in out and "未收进且不在白名单 = 红：1" in out)
    check("② 且提示不许填白名单洗干净", "不许为了让本门变绿而填白名单" in out)

    # ── 与 ② 成对：把 core 那一步补回来 ⇒ 必绿 ─────────────────────────────
    rc2, out2 = _run(good)
    hit2 = "③ 好件必绿 —— 与 ② 成对（补回一步即绿）"
    check(hit2 + "（rc=0）", rc2 == 0, "rc=%d" % rc2)
    check(hit2 + "（A 红数 0）", "未收进且不在白名单 = 红：0" in out2)

    # ── 成对负控 B：命中 ≤ 基线 ⇒ 绿且打印剩余；命中 > 基线 ⇒ 红 ────────────
    #    夹具：orphan.py 未点名 ⇒ 命中 1。baseline=1 ⇒ 绿；baseline=0 ⇒ 红。
    ok_wl = {"build_dirs": [], "scripts": [{"path": "scripts/ok.py", "reason": "夹具占位",
                                            "date": "2026-09-18"}]}
    rat_ok = _fixture(base, "b-ratchet-ok",
                      steps=[("core", "go build ./...")],
                      scripts=("scripts/ok.py", "scripts/orphan.py"),
                      config=ok_wl)
    _write(os.path.join(rat_ok, "scripts", "gates", "gate-coverage.config.json"),
           json.dumps({"schema": "gate-coverage/1", "baseline": 1, "whitelist": ok_wl},
                      ensure_ascii=False))
    _gitify(rat_ok)          # ★ 判「新件 ⟷ 存量债」要有索引（无索引 ⇒ rc=2，见 ⑯b/⑯c）
    rc, out = _run(rat_ok)
    check("④ B 命中(1) ≤ 基线(1) ⇒ rc=0", rc == 0, "rc=%d" % rc)
    check("④ 且**打印剩余数**（不许悄悄绿）", "剩余 1 条" in out)

    rat_bad = _fixture(base, "b-ratchet-exceeded",
                       steps=[("core", "go build ./...")],
                       scripts=("scripts/ok.py", "scripts/orphan.py"),
                       config=ok_wl)  # baseline=0
    _gitify(rat_bad)         # ★ 同上：无索引 ⇒ rc=2，会盖掉这条「棘轮超额必红」的判据
    rc, out = _run(rat_bad)
    check("⑤ B 命中(1) > 基线(0) ⇒ rc=1 —— 与 ④ 成对", rc == 1, "rc=%d" % rc)
    check("⑤ 且点名「超过基线 1 条」", "超过基线 1 条" in out)

    # ── 白名单缺 reason / 缺 date ⇒ rc=2（不是红：写不清理由 = 判不了）────────
    nor_eason = _fixture(base, "wl-no-reason",
                         steps=[("", "python3 scripts/ok.py")],
                         manifests=("core/go.mod",),
                         config={"build_dirs": [{"path": "core", "date": "2026-09-18"}],
                                 "scripts": []})
    rc, out = _run(nor_eason)
    check("⑥ 白名单缺 reason ⇒ rc=2（不是 rc=1）", rc == 2, "rc=%d" % rc)
    check("⑥ 且输出点名「缺 reason」", "缺 reason" in out)

    nodate = _fixture(base, "wl-no-date",
                      steps=[("", "python3 scripts/ok.py")],
                      config={"build_dirs": [{"path": "core", "reason": "夹具"}], "scripts": []})
    rc, out = _run(nodate)
    check("⑦ 白名单缺 date ⇒ rc=2", rc == 2, "rc=%d" % rc)
    check("⑦ 且输出点名「缺 date」", "缺 date" in out)

    # ── 空转 ⇒ rc=2（绝不报绿）──────────────────────────────────────────
    a_empty = _fixture(base, "a-empty", steps=[("", "python3 scripts/ok.py")], manifests=())
    rc, out = _run(a_empty)
    check("⑧ A 空转（0 个构建清单目录）⇒ rc=2，**绝不报绿**", rc == 2, "rc=%d" % rc)
    check("⑧ 且输出点名「空转」", "空转" in out)

    b_empty = _fixture(base, "b-empty", steps=[("core", "go build ./...")], scripts=(),
                       config_raw=json.dumps({"schema": "gate-coverage/1", "baseline": 0,
                                              "scan": {"script_globs": ["scripts/*.rb"],
                                                       "nosuffix_dir": "no-such-dir"},
                                              "whitelist": {}}, ensure_ascii=False))
    rc, out = _run(b_empty)
    check("⑨ B 空转（0 个脚本）⇒ rc=2，**绝不报绿**", rc == 2, "rc=%d" % rc)
    check("⑨ 且输出点名「空转」", "空转" in out)

    # ── 缺件 ⇒ rc=2 ─────────────────────────────────────────────────────
    no_cfg = _fixture(base, "no-config", steps=[("core", "go build ./...")],
                      config_raw="{}", )
    os.remove(os.path.join(no_cfg, "scripts", "gates", "gate-coverage.config.json"))
    rc, out = _run(no_cfg)
    check("⑩ config 缺件 ⇒ rc=2", rc == 2, "rc=%d" % rc)
    check("⑩ 且输出点名「config 不在」", "config 不在" in out)

    no_gates = _fixture(base, "no-gates", manifests=("core/go.mod",), scripts=("scripts/ok.py",),
                        no_gates=True)
    rc, out = _run(no_gates)
    check("⑪ 闸文件缺件 ⇒ rc=2", rc == 2, "rc=%d" % rc)
    check("⑪ 且输出点名「闸文件不在」", "闸文件不在" in out)

    # ── config 有未实现键 ⇒ rc=2（schema 加了规则、门照绿 = 假覆盖）─────────
    bad_key = _fixture(base, "cfg-bad-key", steps=[("core", "go build ./...")],
                       config_raw=json.dumps({"schema": "gate-coverage/1", "baseline": 0,
                                              "whitelist": {}, "new_rule": True}, ensure_ascii=False))
    rc, out = _run(bad_key)
    check("⑫ config 有未实现的键 ⇒ rc=2", rc == 2, "rc=%d" % rc)
    check("⑫ 且输出点名那个键（new_rule）", "new_rule" in out)

    # ── 悬空白名单：照报、不改退码 ───────────────────────────────────────
    stale = _fixture(base, "wl-stale", steps=[("core", "go build ./..."), ("", "python3 scripts/ok.py")],
                     config={"build_dirs": [{"path": "没了/这个目录", "reason": "夹具", "date": "2026-09-18"}],
                             "scripts": []})
    rc, out = _run(stale)
    check("⑬ 悬空白名单**不改退码**（rc=0）", rc == 0, "rc=%d" % rc)
    check("⑬ 且照报「悬空白名单」", "悬空白名单" in out)

    # ── 弱挂载档（②）真的分得出来：只在闸文件注释里提名的脚本不算 ① ─────────
    #   ★ 同上：夹具闸文件下沉后不再是候选 ⇒ **显式补第二只弱挂载脚本**，
    #   让「② = 2」这条判据的区分度保持不变。
    weak = _fixture(base, "weak",
                    steps=[("core", "go build ./...")],
                    scripts=("scripts/ok.py", "scripts/orphan.py"),
                    config={"build_dirs": [], "scripts": [{"path": "scripts/ok.py", "reason": "夹具",
                                                           "date": "2026-09-18"}]},
                    gates_extra="# 这里只为「提到」scripts/ok.py 与 scripts/orphan.py，不接进任何一步")
    rc, out = _run(weak)
    check("⑭ 弱挂载档可分辨（② 单列，不并进 ①）", rc == 0 and "弱挂载" in out
          and "① add_step 命令串里点名：0" in out
          and "② 闸文件里被提到、但不在任何 add_step 命令串里（弱挂载 · 单列）：2" in out
          and "- scripts/ok.py" in out)
    check("⑭ 且②b「只被提一下」单列（注释里的门不算接线）",
          "②b **只被提一下**" in out and "②b **只被提一下**（注释/文案/清单项 · **不算接线** · `GAP-20260927-123`）：2" in out)

    # ── ⑭b/⑭c ② 拆两档（真接线 vs 只被提一下）成对负控（`GAP-20260927-123`）────────────
    #   判据面 = `script_tier` 的 ②a/②b 分类（命令位调用 = 真接线；注释/文案 = 只被提一下）。
    #   ⑭b 正控：闸体里 `python3 scripts/side.py`（命令位、非 add_step）⇒ 判 ②a；
    #   ⑭c 负控（同一夹具、只把命令位那行改成注释）⇒ 回 ②b（分类不是恒真）。
    wiredfix = _fixture(base, "c-tier2-wired",
                        steps=[("core", "go build ./..."), ("", "python3 scripts/ok.py")],
                        scripts=("scripts/ok.py", "scripts/side.py"),
                        gates_extra="if true; then python3 scripts/side.py; fi")
    rc, out = _run(wiredfix)
    check("⑭b 命令位调用（非 add_step）⇒ 判 ②a 真接线（旧口径只给一个 ②）",
          rc == 0 and "②a **真接线**（闸体命令位调用 · 与门③同口径）：1" in out
          and "②b **只被提一下**（注释/文案/清单项 · **不算接线** · `GAP-20260927-123`）：0" in out)
    _write(os.path.join(wiredfix, "scripts", "gates", "precommit-gates.sh"),
           open(os.path.join(wiredfix, "scripts", "gates", "precommit-gates.sh"),
                encoding="utf-8").read().replace(
                     "if true; then python3 scripts/side.py; fi",
                     "# 只提到 scripts/side.py，不是命令位调用"))
    rc, out = _run(wiredfix)
    check("⑭c 负控（同一夹具 · 把那行改成注释）⇒ 回 ②b（分类不是恒真）",
          rc == 0 and "②a **真接线**（闸体命令位调用 · 与门③同口径）：0" in out
          and "②b **只被提一下**（注释/文案/清单项 · **不算接线** · `GAP-20260927-123`）：1" in out)

    # ── ⑭f/⑭g ① 的 `zerg gate check <去扩展名>` 等价档（`GAP-20261001-74`）─────────────
    #   判据面 = `script_tier` 的 ① in_steps：命令串 `zerg gate check check-x` 里 basename
    #   （含 `.py`）不在 ⇒ 旧口径掉 ③；等价档按**去扩展名**判 ⇒ 回 ①。
    #   ⑭f 正控：命令位 `zerg gate check check-x` ⇒ ① 命中 1、rc=0。
    zfix = _fixture(base, "z-zerg-insteps",
                    steps=[("core", "go build ./..."), ("", "zerg gate check check-x")],
                    scripts=("scripts/check-x.py",))
    rc, out = _run(zfix)
    check("⑭f `zerg gate check <去扩展名>` 命令位 ⇒ 判 ① in_steps 等价档（GAP-20261001-74）",
          rc == 0 and "① add_step 命令串里点名：1" in out, "rc=%d" % rc)
    #   ⑭g 负控（与 ⑭f 成对 · 方向性）：缺 `gate` 子命令（`zerg check check-x`）⇒ 不认 ⇒ 落 ③、rc=1
    nfix = _fixture(base, "z-zerg-nosubcmd",
                    steps=[("core", "go build ./..."), ("", "zerg check check-x")],
                    scripts=("scripts/check-x.py",))
    _gitify(nfix)                                 # 有索引 ⇒ 「新件 ⟷ 存量债」分得开（否则 rc=2「判不了」）
    rc, out = _run(nfix)
    check("⑭g 负控：`zerg check check-x`（缺 gate）⇒ 不判 ①（防「只加 zerg 就放行」）",
          rc == 1 and "① add_step 命令串里点名：0" in out, "rc=%d" % rc)

    # ── ⑭d/⑭e 治静默：**只被提一下的新件**必须落 ④（rc=1）—— 成对负控 ──────────────
    #   判据面 = 「②b 只被提一下 ∧ 不在 git 索引 ⇒ 新件未挂门」。旧口径里它落 ② ⇒ 永久静默
    #   （既不进棘轮也不进 ④），这条正是 `GAP-20260927-123` ① 的洞。
    mn = _fixture(base, "d-mention-new-ungated",
                  steps=[("core", "go build ./..."), ("", "python3 scripts/ok.py")],
                  scripts=("scripts/ok.py",))
    _gitify(mn)                                   # 有索引 ⇒ 「新件 ⟷ 存量债」分得开
    _write(os.path.join(mn, "scripts", "mentiononly.py"),
           "#!/usr/bin/env python3\nprint('mention only fixture')\n")
    gf = os.path.join(mn, "scripts", "gates", "precommit-gates.sh")
    _write(gf, open(gf, encoding="utf-8").read().replace(
        'main "$@"', '# 只提到 scripts/mentiononly.py，不接进任何一步\nmain "$@"'))
    rc, out = _run(mn)
    check("⑭d **只被提一下的新件** ⇒ rc=1 + 点名（旧口径落 ② ⇒ 永久静默/rc=0）",
          rc == 1 and "新件只被提一下、未真接线" in out, "rc=%d" % rc)
    #   负控（与 ⑭d 成对）：把它真接进 add_step ⇒ 落 ① ⇒ 回绿
    _write(gf, open(gf, encoding="utf-8").read().replace(
        '  add_step fixture "步骤 core"',
        '  add_step fixture "步骤 mentiononly" rc "${REPO_ROOT}"'
        ' "python3 scripts/mentiononly.py"\n'
        '  add_step fixture "步骤 core"'))
    rc, out = _run(mn)
    check("⑭e 同批真接线（写进 add_step）⇒ 回绿（rc=0）—— 与 ⑭d 成对", rc == 0, "rc=%d" % rc)

    # ── 计数自证行真的在 ────────────────────────────────────────────────
    check("⑮ 报告含「计数自证」两条恒等式", "计数自证" in out2 and "① 1 + ② 1 + ③ 0 = 2（候选 2）" in out2)

    # ── ⑮b–⑮d 计数自证**成对负控**（`GAP-20260928-11`）：恒等式必须能被输入置假 ────────
    #   判据面 = `counts_certify`（analyse 与这里**同一份**口径）⇒ 自检里置假 = 置假真判据。
    #   ⑮b 正控（真仓形态：三列表 1+1+1 == 3 · 三档 1+1+1 == 3）⇒ ✓；
    #   ⑮c/⑮d 负控（分别只砸两条恒等式中的一条）⇒ 必 ✗。**合起来才证明它有牙**。
    _c3 = [{"script": "x"}, {"script": "y"}, {"script": "z"}]
    _c_ok3 = {"in_steps": 1, "mentioned": 1, "unmentioned": 1}
    check("⑮b 计数自证 正控：三列表 1+1+1 == 总项数 3 · 三档 1+1+1 == 候选 3 ⇒ ✓",
          counts_certify([1], [1], [1], _c3, _c_ok3, _c3) is True)
    check("⑮c 计数自证 负控：三列表 1+1+0 == 2 ≠ 总项数 3 ⇒ **必 ✗**（三列表那一半有牙）",
          counts_certify([1], [1], [], _c3, _c_ok3, _c3) is False)
    check("⑮d 计数自证 负控：三档 1+1+0 == 2 ≠ 候选数 3 ⇒ **必 ✗**（三档那一半有牙）",
          counts_certify([1], [1], [1], _c3,
                         {"in_steps": 1, "mentioned": 1, "unmentioned": 0}, _c3) is False)
    #   ★ 端到端成对负控：判据被置假 ⇒ **真退 2**（证明 `:706`/`:1178` 那条路不是死码）；
    #     复位后**同一夹具**回绿。两次都走真 `main()`（只把判据与 stdout 换掉，别的一字不动）。
    _sv_cert, _sv_out = globals()["counts_certify"], sys.stdout
    _buf_bad, _buf_ok = io.StringIO(), io.StringIO()
    try:
        globals()["counts_certify"] = lambda *a, **k: False
        sys.stdout = _buf_bad
        rc_bad = main(["--no-self-test", "--root", good])
        globals()["counts_certify"] = _sv_cert
        sys.stdout = _buf_ok
        rc_ok = main(["--no-self-test", "--root", good])
    finally:
        globals()["counts_certify"] = _sv_cert
        sys.stdout = _sv_out
    check("⑮e 端到端负控：计数自证被置假 ⇒ rc=2（不给结论，不是死码）", rc_bad == 2, "rc=%d" % rc_bad)
    check("⑮f 且逐字点名「计数自证不过」", "计数自证不过" in _buf_bad.getvalue())
    check("⑮g 正控：判据复位后**同一夹具**⇒ rc=0（与 ⑮e 成对）", rc_ok == 0, "rc=%d" % rc_ok)

    # ── ④ 新格：**新件未挂门**（与存量债分开）—— 成对负控 ─────────────────
    #   判据面 = 「不在 git 索引里」⇒ 夹具必须先真 `git init` + `git add`（不提交）。
    #   这一对证明的是 `Q-175` ② 那半：**新件落 ③ ⇒ rc≠0 且逐字点名该件**（今天 ≤ 基线即绿 ✗）。
    ng = _fixture(base, "c-new-ungated",
                  steps=[("core", "go build ./..."), ("", "python3 scripts/ok.py")],
                  scripts=("scripts/ok.py",))
    subprocess.run(["git", "init", "-q"], cwd=ng,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(["git", "add", "-A"], cwd=ng,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    _write(os.path.join(ng, "scripts", "newgate.py"),
           "#!/usr/bin/env python3\nprint('new gate fixture')\n")
    rc, out = _run(ng)
    check("⑯ 新件未挂门 ⇒ rc=1（存量 0 条、基线 0 ⇒ 旧口径必绿）", rc == 1, "rc=%d" % rc)
    check("⑯ 且新格单列 + 逐字点名该件",
          "新件未挂门" in out and "④ **新件未挂门**" in out
          and "scripts/newgate.py" in out
          and "不在 git 索引" in out)
    # 负控（与 ⑯ 成对）：把该新件**同批挂进 add_step** ⇒ 回绿
    gate_fixture = os.path.join(ng, "scripts", "gates", "precommit-gates.sh")
    _write(gate_fixture, open(gate_fixture, encoding="utf-8").read().replace(
        '  add_step fixture "步骤 root"',
        '  add_step fixture "步骤 newgate" rc "${REPO_ROOT}" "python3 scripts/newgate.py"\n'
        '  add_step fixture "步骤 root"'))
    rc, out = _run(ng)
    check("⑰ 同批挂门（写进 add_step）⇒ 回绿（rc=0）—— 与 ⑯ 成对", rc == 0, "rc=%d" % rc)
    check("⑰ 且新格归零", "④ **新件未挂门**（本门开张后新建、且没与代码同批挂 add_step）= **0**" in out)

    # ── ⑰b 多行 add_step 命令：脚本名在第 2 行 ⇒ 必判 ①（`GAP-20260927-127` 成对负控）──────
    #   判据面 = 步骤抽取器：记录改 NUL 分隔 ⇒ 多行命令原样进 cmds_text。
    #   负控（真退码）：命令拆两行、脚本名放第 2 行 ⇒ 旧口径切成 ② 弱挂载（不进 ①/③）；
    #   本口径必须判 ①、② 档为 0 ⇒ 该件被认定「真挂门」⇒ 不进 ③/④ ⇒ rc=0（baseline 0）。
    ml = _fixture(base, "multiline-step",
                  steps=[("core", "go build ./..."),
                         ("", "true\npython3 scripts/multiline.py")],
                  scripts=("scripts/multiline.py",))
    rc, out = _run(ml)
    check("⑰b 多行 add_step（脚本名在第 2 行）⇒ 判 ①（不被首行截断）", rc == 0, "rc=%d" % rc)
    check("⑰b 且 ① 档点名 1 条", "① add_step 命令串里点名：1" in out)
    check("⑰b 且 ② 档为 0（旧口径会把这条切成弱挂载）",
          "② 闸文件里被提到、但不在任何 add_step 命令串里（弱挂载 · 单列）：0" in out)

    # ── ⑯d/⑯e ④ 格**第二路指路**（语料在仓外 / 写盘型 ⇒ `add_step` 是死路）成对负控 ──────
    #   判据面 = `declares_out_of_scope`（分类**只此一处**）—— 真源是**件头自述**
    #   「不进门禁步骤表」（config 第二档白名单口径逐字引 `check-outrepo-dead-refs.py:19`）。
    #   这一对证明：① 自述件被**逐件点名**指到第二路（`whitelist.scripts`）；
    #   ② 普通件那句（`add_step`）**逐字照旧**、一字未动 —— 两路并存不互相吞。
    oor_fix = _fixture(base, "c-new-ungated-oor",
                       steps=[("core", "go build ./..."), ("", "python3 scripts/ok.py")],
                       scripts=("scripts/ok.py",))
    _gitify(oor_fix)
    _write(os.path.join(oor_fix, "scripts", "outrepo.py"),
           "#!/usr/bin/env python3\n"
           "# -*- coding: utf-8 -*-\n"
           '"""夹具：★ 为什么本件**不进门禁步骤表** —— 语料在仓外（夹具自称，与真仓同体例）。\n"""\n'
           "print('outrepo fixture')\n")
    rc, out = _run(oor_fix)
    check("⑯d 件头自述「不进门禁步骤表」⇒ 逐件点名走第二路（第二档白名单）",
          rc == 1 and "- scripts/outrepo.py" in out and "whitelist.scripts" in out
          and "**不许**挂 `add_step`（挂上去就是死路）" in out, "rc=%d" % rc)
    check("⑯e 普通件那一路**逐字照旧**（同一字面串 · 复用 `HINT_ADD_STEP` 常量，不另抄一份）",
          HINT_ADD_STEP in out)
    #   ★ 负控（与 ⑯d 成对）：把件头那句自述删掉（同一夹具、别的一字不动）⇒ 不再点名第二路
    _write(os.path.join(oor_fix, "scripts", "outrepo.py"),
           "#!/usr/bin/env python3\nprint('outrepo fixture')\n")
    rc, out = _run(oor_fix)
    check("⑯d- 负控：删掉件头自述 ⇒ 该件不再进第二路点名（分流不是恒真）",
          rc == 1 and "- scripts/outrepo.py  ← 修法" not in out, "rc=%d" % rc)

    # ── ⑯b/⑯c **索引读不到 ⇒ 判不了** 成对负控（`GAP-20260927-105`）────────────────
    #   判据面 = 「③ 档非空 ∧ git 索引读不到 ⇒ 不给结论 rc=2」（与 ⑧/⑨ 空转同口径）。
    #   ⑯b 负控：无 `.git` 的物化树上，③ 档那件**退码面必须不再静默当存量债** ⇒ rc=2 且点名；
    #   ⑯c 正控：**同一夹具**补真 `git init` + `git add -A` ⇒ 该件=存量债（≤ 基线）⇒ 回绿 rc=0
    #   —— 与 ⑯b 成对，证明这条判据不是「恒 rc=2」。
    nu = _fixture(base, "d-tracked-unknown",
                  steps=[("core", "go build ./...")],
                  scripts=("scripts/ok.py", "scripts/orphan.py"),
                  config_raw=json.dumps({"schema": "gate-coverage/1", "baseline": 2,
                                         "whitelist": {}}, ensure_ascii=False))
    rc, out = _run(nu)
    check("⑯b 无 .git 且 ③ 档非空 ⇒ rc=2（判不了，不再静默当存量债 / 不再 rc=0）",
          rc == 2, "rc=%d" % rc)
    check("⑯b 且逐字点名「判不了：仓根不是 git 仓」与「不给结论」",
          "判不了：仓根不是 git 仓" in out and "不给结论" in out)
    _gitify(nu)
    rc, out = _run(nu)
    check("⑯c 同一夹具补真 git init + add ⇒ 该件=存量债 ⇒ rc=0 —— 与 ⑯b 成对（非恒 rc=2）",
          rc == 0, "rc=%d" % rc)
    check("⑯c 且输出里不再有「判不了：仓根不是 git 仓」", "判不了：仓根不是 git 仓" not in out)

    # ── ⑱/⑲/⑳/㉑/㉒ `--json` 的 **stdout 纯净度**（本门刚治的病：自检文本混进 stdout ⇒ json.loads 直接失败）──
    #   真命令行 · 真退出码 · 真流；判据只问「stdout 能不能 json.loads」⇒ 与实现无关（换实现也拦得住）。
    #   ★ 这一组只在 `deep` 档跑：探针子进程必须剪掉探针自己（否则无限自嵌套，见 _SELFTEST_DEEP）。
    if deep:
        rcA, outA, errA = _probe(base, ["--json", "--root", good])
        sA = outA.decode("utf-8", "replace")
        okA, offA = _stdout_json_purity(outA)
        check("⑱ `--json`（自检开着）⇒ stdout 是纯 JSON（json.loads 过 · 首字符 `{`）",
              okA and sA.lstrip().startswith("{"), "点名=%s" % offA)
        check("⑱ 且自检文本**改走 stderr**（stdout 里一个「自检」都不许有）",
              "── 自检（成对负控）──" in errA.decode("utf-8", "replace") and "自检" not in sA)
        check("⑱ 且退码与人读档同源（夹具 good ⇒ rc=0）", rcA == 0, "rc=%d" % rcA)

        # 成对负控：**默认档**（自检文本 + 人读面照旧全落 stdout）⇒ 上面那条判据必须拓出来并点名
        rcB, outB, _errB = _probe(base, ["--root", good])
        sB = outB.decode("utf-8", "replace")
        okB, offB = _stdout_json_purity(outB)
        check("⑲ 负控：自检文本回写 stdout（默认档）⇒ 纯度判据必拓出来", not okB, "点名=%s" % offB)
        check("⑲ 且**逐字点名**首行 `── 自检（成对负控）──`",
              offB.startswith("── 自检（成对负控）──"), "点名=%s" % offB)
        check("⑲ 且负控档 stdout 确实含自检文本（负控本身不是空转）",
              "自检结论" in sB and "── 自检（成对负控）──" in sB)

        # 与 ⑱ 成对：`--json --no-self-test`（等价档）同样纯
        rcC, outC, _errC = _probe(base, ["--json", "--no-self-test", "--root", good])
        okC, offC = _stdout_json_purity(outC)
        check("⑳ 成对：`--json --no-self-test` 亦纯 JSON", rcC == 0 and okC,
              "rc=%d 点名=%s" % (rcC, offC))

        # 互斥：只产人读文本的开关 + `--json` ⇒ 用法错 rc=2（否则「`--json` ⇒ stdout 只有 JSON」有例外）
        rcD, _outD, errD = _probe(base, ["--json", "--self-test"])
        check("㉑ `--json --self-test` 互斥 ⇒ rc=2（不是「stdout 空着号称机读」）", rcD == 2, "rc=%d" % rcD)
        check("㉑ 且理由点名在 stderr（含「互斥」）", "互斥" in errD.decode("utf-8", "replace"))
        rcE, outE, _errE = _probe(base, ["--json", "--list-rules"])
        check("㉒ `--json --list-rules` 互斥 ⇒ rc=2", rcE == 2, "rc=%d" % rcE)
        check("㉒ 且 stdout 不落人读规则表", "规则表" not in outE.decode("utf-8", "replace"))

    # ── ㉓ `_baseline_note` ↔ `baseline` 同源（`GAP-20260927-42` ①）成对负控 ────────────
    #   判据面 = `baseline_note_drift`（真跑路径与这里**同一份**口径）⇒ 置假 = 置假真判据。
    check("㉓a note 末次现值句 == baseline ⇒ 不漂（正控）",
          baseline_note_drift(55, "…本行由 68 下调成 56、再下调成 55。")[0] is True)
    check("㉓b note 末次现值句 ≠ baseline ⇒ **必漂**（负控 · 与 ㉓a 成对）",
          baseline_note_drift(55, "…本行由 68 下调成 56。")[0] is False)
    check("㉓c note 无现值重述句 ⇒ 不误红（只认可机读「下调成 N」形态）",
          baseline_note_drift(55, "只有散文、无重述句")[0] is True)

    print("── check-gate-coverage 自检（成对负控 · 真命令行 + 真退出码）──", file=sink)
    bad = 0
    for desc, ok, extra in cases:
        print("   %s %s%s" % ("✓" if ok else "✗", desc, ("（%s）" % extra) if extra and not ok else ""),
              file=sink)
        if not ok:
            bad += 1
    print("自检结论: %s（断言 %d 条 · 失败 %d 条）"
          % ("全过" if bad == 0 else "**不过**", len(cases), bad), file=sink)
    return bad == 0


def cleanup():
    for d in _TMPDIRS:
        shutil.rmtree(d, ignore_errors=True)


atexit.register(cleanup)


# ══════════════════════════════════════════════════════════════════
# 6. main
# ══════════════════════════════════════════════════════════════════
def main(argv):
    ap = argparse.ArgumentParser(add_help=True, description="门覆盖自检（check-gate-coverage.py）")
    ap.add_argument("--root", default=REPO_ROOT_DEFAULT)
    ap.add_argument("--config", default="")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--baseline", action="store_true")
    ap.add_argument("--list-rules", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--no-self-test", action="store_true")
    ap.add_argument("--max-examples", type=int, default=40)
    args = ap.parse_args(argv)

    # ── `--json` 的契约 = **stdout 只有 JSON**（机读面可直接 json.loads）──────────────────
    #   两个「只产人读文本、不产 JSON」的开关与 `--json` 同给 ⇒ **用法错 rc=2**（不是静默忽略）：
    #   静默忽略 = 号称机读而 stdout 空着 / 脏着 —— 那正是本门刚治的病（判不了就不给结论）。
    if args.json and (args.self_test or args.list_rules):
        which = "--self-test" if args.self_test else "--list-rules"
        print("✗ 用法错（rc=2）：`--json` 与 `%s` 互斥 —— `%s` 只产人读文本、不产 JSON，"
              "而 `--json` 的契约是「stdout 只有 JSON（可 json.loads）」⇒ 不给结论。"
              % (which, which), file=sys.stderr)
        return EXIT_BLOCKED

    if args.list_rules:
        print_list_rules()
        return EXIT_OK

    if args.self_test:
        print("── check-gate-coverage 自检（成对负控：漏一个目录必红 ↔ 补齐必绿；"
              "命中>基线必红 ↔ ≤基线必绿；白名单缺 reason/date ⇒ rc=2；空转 ⇒ rc=2）──")
        return EXIT_OK if self_test() else EXIT_BLOCKED

    if not args.no_self_test:
        # ★ `--json` 时**全部自检文本改走 stderr** ⇒ stdout 只剩那一份 JSON（人读面一字不动）
        st = sys.stderr if args.json else sys.stdout
        print("── 自检（成对负控）──", file=st)
        if not self_test(sink=st):
            print("自检未过 ⇒ 拒绝扫真目标（rc=2 不给结论）", file=st)
            return EXIT_BLOCKED
        print(file=st)

    root = os.path.abspath(args.root)
    cfg_path = args.config or os.path.join(root, LOCAL_CONFIG)
    try:
        cfg = load_config(cfg_path)
        if not os.path.isdir(root):
            raise Unsupported("仓根不存在：%s" % root)
        tmpdir = tempfile.mkdtemp(prefix="gc-steps-")
        _TMPDIRS.append(tmpdir)
        steps, err = extract_steps(root, tmpdir)
        if err:
            raise Unsupported(err)
        gates_text = open(os.path.join(root, GATES_REL), encoding="utf-8").read()
        rep = analyse(root, cfg, steps, gates_text, args.max_examples)
    except Unsupported as exc:
        print("✗ 不给结论（rc=2）：%s" % exc, file=sys.stderr)
        # ★ `GAP-20260927-42` ②：`--json` 档给**结构化错误**封包（stdout 只有一份可 json.loads 的 JSON）
        #   ⇒「缺件（判不了）」与「零命中（rc=0）」在机器面可分；非 `--json` 档一字不动。
        if args.json:
            print(json.dumps({
                "schema": ERR_SCHEMA,
                "rc": EXIT_BLOCKED,
                "verdict": {"rc": EXIT_BLOCKED, "code": "UNSUPPORTED", "reason": str(exc)},
                "root": root,
                "config": cfg_path,
            }, ensure_ascii=False, indent=2, sort_keys=True))
        return EXIT_BLOCKED

    # ★ `_baseline_note` ↔ `baseline` 同源校验（`GAP-20260927-42` ①）：末次现值句必须 == baseline，
    #   否则人读报告说一套、`baseline` 是另一套 ⇒ 读者按错数算余量。红档并入 rc=1（不是 rc=2）。
    _note_ok, _note_why = baseline_note_drift(rep["baseline"], rep["baseline_note"])
    rep["note_drift"] = {"ok": _note_ok, "detail": _note_why}

    # ── 退出码（优先级 2 > 1 > 0）—— **先算结论，再输出**（--json 的 stdout 必须是纯 JSON）──
    a, b = rep["a"], rep["b"]
    n_red_a = len(a["red"])
    n_over_b = b["hit"] - rep["baseline"]
    n_new_b = len(b["new_ungated"])
    if a["empty"] or b["empty"]:
        which = ("A（构建清单目录）" if a["empty"] else "") + ("B（scripts 脚本）" if b["empty"] else "")
        rep["verdict"] = {"rc": EXIT_BLOCKED, "code": "EMPTY_SCAN",
                          "reason": "空转：%s 候选集为 0 ⇒ 假覆盖（假绿的一种）⇒ 不给结论" % which}
        rep["rc"] = EXIT_BLOCKED
    elif not rep["certify"]:
        rep["verdict"] = {"rc": EXIT_BLOCKED, "code": "CERTIFY_FAILED",
                          "reason": "计数自证不过（三档之和 ≠ 候选总数）⇒ 不给结论"}
        rep["rc"] = EXIT_BLOCKED
    elif b["hit_items"] and not b["git_tracked_known"]:
        # ④ 格子**判不了** ⇒ 不给结论（口径照抄同门 ⑧/⑨ 空转 · ⑥/⑦ 白名单缺理由：判不了就 rc=2，
        #   「判不了的时候报绿报红都是猜」）。
        #   ★ 为什么只在「③ 档非空」时判不了：③ 档为空 ⇒ ④ 格子恒 0、「新件 ⟷ 存量债」这条判据
        #     **不承重** ⇒ 结论仍确定（判不了才停，不误报 —— 同族纪律）。
        #   ★ 为什么必须 fail-closed：非 git 仓里每一件都被写成「存量债（已提交件 …）」，那句
        #     事实断言是假的，且「新件未挂门」恒 0 ⇒ 新件在基线余额内静默洗白（`GAP-20260927-105`）。
        rep["verdict"] = {"rc": EXIT_BLOCKED, "code": "TRACKED_UNKNOWN",
                          "reason": "判不了：仓根不是 git 仓 / git 不可用 ⇒ ③ 档 %d 条分不开"
                                    "「新件 / 存量债」⇒ 不给结论"
                                    % len(b["hit_items"])}
        rep["rc"] = EXIT_BLOCKED
    elif n_red_a or n_over_b > 0 or n_new_b > 0 or not _note_ok:
        rep["verdict"] = {"rc": EXIT_FAIL, "code": "RED",
                          "reason": "A 未收进 %d 条；B 棘轮命中 %d > 基线 %d（超 %d）；"
                                    "新件未挂门 %d 条；_baseline_note 漂移 %d 条"
                                    % (n_red_a, b["hit"], rep["baseline"], max(n_over_b, 0), n_new_b,
                                       0 if _note_ok else 1)}
        rep["rc"] = EXIT_FAIL
    else:
        rep["verdict"] = {"rc": EXIT_OK, "code": "OK",
                          "reason": "A 无未收进目录；B 命中 %d ≤ 基线 %d（剩余 %d）；新件未挂门 0"
                                    % (b["hit"], rep["baseline"], b["hit"])}
        rep["rc"] = EXIT_OK

    if args.json:
        print(json.dumps(rep, ensure_ascii=False, indent=2, sort_keys=True))
        print("rc=%d · %s" % (rep["rc"], rep["verdict"]["reason"]), file=sys.stderr)
    else:
        print_report(rep)
        if args.baseline:
            print_baseline(rep)
        print("== 结论 ==")
        print("   A 未收进：%d 条 · B 棘轮命中 %d / 基线 %d（超 %d）· ④ 新件未挂门 %d 条"
              % (n_red_a, b["hit"], rep["baseline"], max(n_over_b, 0), n_new_b))
        print("   _baseline_note：%s（%s）"
              % ("不漂" if _note_ok else "**漂移**（note 与 baseline 不同源）", _note_why))
        if rep["rc"] == EXIT_BLOCKED:
            print("   ⇒ **不给结论（rc=2）**：%s" % rep["verdict"]["reason"], file=sys.stderr)
        elif rep["rc"] == EXIT_FAIL:
            print("   ⇒ 红（rc=1）：诊断见上；处置 = **真修**（补 add_step）或白名单写清 reason+date")
        else:
            print("   ⇒ 绿（rc=0）：A 无未收进目录；B 命中 %d ≤ 基线 %d（剩余 %d）"
                  % (b["hit"], rep["baseline"], b["hit"]))
    return rep["rc"]


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
