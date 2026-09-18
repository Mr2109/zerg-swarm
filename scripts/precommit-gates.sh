#!/usr/bin/env bash
# 提交前门禁总控：一步一文件 · 计数从结果表读出 · 判据只看真实退出码
#
# 为什么存在（2026-09-16 第十三轮的真实事故）
# ------------------------------------------
# 我自己的门禁脚本是「假绿」的：① 每一步的输出都写进**同一个**临时文件 ⇒ 后一步覆盖前一步，
# 那次 `agent: go test -race ./... -count=1` 真红的输出当场丢失、无法取证；
# ② 失败计数在 `( cd core && step … )` 的**子壳里**自增 ⇒ 改的是子壳的副本，回到父壳
# 计数器仍是 0，末尾照样打印「门禁失败项数: 0」。
# 于是「门禁过了」这件事本身不可信 —— 与项目铁律点名的「拿输出字样/计数器当判据」同源。
#
# 本脚本的两条硬规矩
# ------------------
# ① **一步一个独立日志文件**：`<outdir>/NN-<步骤名>.log`，谁也不会覆盖谁；
# ② **失败数只从 `results.tsv` 数出来**（每次判定立刻落盘一行，末了 `grep -c '^FAIL'`）——
#    全脚本**不存在**「在子壳里累加的计数器」。判定一律用真实退出码，不看输出里有没有某个字样；
#    唯一例外是 `empty` 模式（判据本身就是「输出必须为空」，如 `gofmt -l` 不得列出任何文件）。
#
# 用法
# ----
#     bash scripts/precommit-gates.sh                  # 默认跑全部（go + rust + pub + tags + docs + gates + tools + slice ⇒ 45 步）
#     bash scripts/precommit-gates.sh --scope go       # 只跑 Go 侧（可重复：--scope go --scope rust）
#     bash scripts/precommit-gates.sh --scope tags     # 只跑双构建工程门禁（T6.3）
#     bash scripts/precommit-gates.sh --scope docs     # 只跑文档面门禁（meta/name/freshness D1–D3 + glossary + i18n-drift ⇒ 7 步）
#     bash scripts/precommit-gates.sh --scope gates    # 只跑「门自己的门」（覆盖/版本源/接线 ⇒ 3 步）
#     bash scripts/precommit-gates.sh --scope tools    # 只跑工具版本三处一致（⇒ 1 步）
#     bash scripts/precommit-gates.sh --scope slice    # 只跑切片合同探针集（⇒ 1 步）
#     bash scripts/precommit-gates.sh --outdir /tmp/gates-14   # 指定日志目录
#     bash scripts/precommit-gates.sh --list           # 只看步骤清单，不跑
#     bash scripts/precommit-gates.sh --self-test      # 只跑自检（合成步骤，不碰真目标）
#     bash scripts/precommit-gates.sh --emit-cmd 无后缀  # 只打印匹配步骤的命令串（负控/复核用；不跑）
#
# 退出码：0 全绿 · 1 有失败项 · 2 **不给结论**（用法错/前置缺件/自检不过/**有步骤报 BLOCKED**）
#   ★ 第四档 `REPORT`（只报告）**不参与**这个三值出口 —— 它既不是「错」也不是「没结论」。
#
# 四档（步骤级）：PASS · FAIL（失败项）· BLOCKED（**不给结论**）· **REPORT（只报告）**。
#   ★ BLOCKED **不计入失败项数** —— 「没结论」不是「错」，两者各有各的计数与打印位；
#     但 BLOCKED 也不许当绿（有 BLOCKED 且无 FAIL ⇒ 整脚本 rc=2，不是 rc=0）。
#   ★ REPORT 是**第四档「只报告」**（2026-09-18 拍板①，现用于 docs 的 D2）：命中**只入清单** ——
#     **不计失败项数、不计 BLOCKED、不影响退出码**（它只出现在计数与打印里）。
#   模式名的对应：`tri` ⇒ 三档（rc=0 PASS · rc=1 FAIL · rc=2 BLOCKED · 异常码 FAIL）；
#                 `tri-report` ⇒ 四档里的「rc=1 落 REPORT」那一格（其余三格与 `tri` 一字不差）。
#   rc / empty 两模式的语义**一字未改**（现有 20 步靠它们，自检 ⑧d 钉住）。
#
# scope 说明（2026-09-17 加 tags · 2026-09-18 加 docs · 2026-09-18 第二波加 gates ·
#            2026-09-18 ③批次加 tools 与 slice，五者**都进默认集**）
#   go   = gofmt/build/vet/test（**单侧**：默认 tag 配置）—— 2026-09-18 第二波由**两棵**扩到**四棵** Go module：
#          core · agent（原有 12 步）+ **shared**（build/vet/test 3 步）· **scripts/exportnames**（build/vet 2 步）。
#          ★ 2026-09-18 收尾（本批）：这两棵新 module 补上 `gofmt -l` 步（各 1 条，风格逐字对齐 core/agent
#            那两条）⇒ shared 4 步 · scripts/exportnames 3 步（go scope 17 → 19 步）。
#          为什么原来没有：`--scope go` 只 add_step core/agent 两棵（债务台账 §0 第 2 行）⇒ 两个 module
#          编译失败而闸全绿。★ exportnames 实测**无 `_test.go`** ⇒ 按「有测试就加」不加 test 步（不缝空转步）。
#          ★ 它是 main 包：`go build ./...` 会把二进制**写进当前目录**（实测冒出 7 MB `zerg-exportnames`）
#          ⇒ 该步用 `go build -buildvcs=false -o /dev/null ./...`（`-o` 在包模式**之前**，实测 rc=0 且无产物）。
#   rust = Rust 构建树 fmt/clippy/test —— 2026-09-18 第二波由**一棵**扩到**两棵**：wall（原有 3 步）+ **ui**（新增 3 步）。
#          为什么原来没有：`--scope rust` 只 add_step wall（债务台账 §0 第 1 行），而 precheck 只验
#          `ui/Cargo.toml` **在不在**、不验它**过不过** ⇒ ui 可以编译失败而闸全绿。
#          ★ ui 三步按 wall 的口径逐行对齐（同命令串、同工作目录形态、同 `rc` 模式）⇒ 如实反映现状：
#            实测 `cargo test` 绿（97 passed / 0 failed），而 `cargo fmt --check` 与
#            `cargo clippy -- -D warnings` **真红**（存量债：全树未 rustfmt 过 · 43 条告警）——
#            **不放宽、不填白名单**，红照实进清单（要不要按 D2 先例降档，见说明书 §待拍）。
#   pub  = 公开面两侧都有的脚本静态检查 —— 2026-09-18 第二波由 `scripts/` 扩到**四棵外围目录 + 顶层入口**：
#          新增一步「mcp/ · gateway/ · publish/ · tools/ 的 .py/.sh + 顶层 start-zerg-core.sh ·
#          start-zerg-ui.sh」语法（.py ⇒ ast.parse · .sh ⇒ bash -n）⇒ 债务台账 §0 第 3 行点名的那批脚本
#          第一次有门。口径：一条步里**遍历**（单来源函数 `ext_syntax_cmd`，与自检负控共用同一串命令）；
#          **零命中必红**（空转 = 假覆盖，同无后缀语法步）；排除 vendor/venv/构建产物。
#          ★ 2026-09-18 收尾（本批）：这一步的扫描面由四棵扩到**八棵**（`ROOTS += ["ui","zerg-evals","deploy","core"]`
#            一行）⇒ 实测被检查到 38 个（原 32 + 新 6：ui/2 · zerg-evals/2 · deploy/1 · core/1），逐条语法全过，
#            步数与档位都没动（仍是 `rc` 模式那一条步）。
#   tags = 双构建工程门禁（脚本自带正反用例自检；它自己会在两种 tag 配置下成对跑 build/vet）
#   docs = 文档面只读门禁：check-doc-meta(--scope formal --missing=fail) · check-doc-name(--scope repo)
#          · check-doc-freshness 的 D1/D2/D3（D4 生成式 drift 归发布面，不在此）。
#   gates = **门自己的门**（2026-09-18 第二波挂接；来源 = 债务台账 §7 的三条建议门）——
#          · 门① `check-gate-coverage.py`（模式 `tri` ⇒ **阻断**）：断言「含 go.mod/Cargo.toml 的目录都被某步收进」
#            ＋「scripts 脚本要么在步骤表、要么在白名单」的基线棘轮；
#          · 门② `check-version-sources.py`（模式 `tri` ⇒ **阻断**）：版本单一真源四处同版；
#          · 门③ `check-wired-scripts.py`（2026-09-18 ③批次：由 `tri-report` **升为 `tri` 阻断** +
#            `--strict-report`）：scripts 门脚本有没有被某个闸按名调用 / 零引用件有没有登记。
#          三条都**自带 `--self-test`**（成对负控），本 scope **不传 `--no-self-test`**：先自证「会红」再扫真目标。
#          为什么门③ 起初只报告、现在为什么能阻断：它首跑如实报「A 命中 8 · B 未登记 0」（8 只门脚本不在任何闸里）
#          = 存量债 ⇒ 先按 D2 先例**不锁死提交闸**；③批次把这 8 只**逐只挂了闸**（见 docs/pub/tools/slice 与发布闸）
#          ⇒ A 命中 8→0、B 未登记 0 ⇒ 按「清到 0 再升阻断」的拍板把它升成 `tri` + `--strict-report`
#          （**新增一只未挂/未登记就红**）。升档记录：`scripts/check-wired-scripts.md` §五（已执行）。
#   tools = 工具版本三处一致（2026-09-18 ③批次新 scope · 进默认集）——
#          · `check-tool-version-sync.sh`（模式 `tri` ⇒ 阻断）：台账 `tools/versions.json` / 工具文档抬头 /
#            履历首行三处同版；脚本自带前置自检（缺台账 / 台账 0 个工具 ⇒ rc=2），与 tri 三档一一对应。
#          为什么现在才挂：它从造出来那天起就只被手检清单（docs/skills/tool-upgrade-checklist.md）点名，
#          **从没被任何闸跑过**（门③ 断言 A 的第 3 只）；③批次给它 141 个工具的现读实跑 rc=0 ⇒ 直接阻断档。
#   slice = 切片合同探针集（2026-09-18 ③批次新 scope · 进默认集）——
#          · `check-slice.py --probe`（模式 `tri-report` ⇒ 只报告起步）：跑探针集 + 混淆矩阵，判据 = §3.5
#            质量门槛（假绿率必须 0 · 假红率 ≤ 10%）；`--selftest`（四条自证）现跑同样 rc=0。
#          为什么起步只报告：门③ 的现读建议写「体量大（109 KB + slice-probes.json）· 成本未标定 ⇒ 起步只报告」；
#          ③批次实测 `--probe` 0.05s、rc=0（假绿 0 / 假红 0.0%，探针 36 条）⇒ 可升 `tri`（留待拍，见 .md §待拍）。
#          ★ 它们都是**步骤表里的一等步骤**（不是本脚本尾部那种软检查位），rc 一律取真退出码、**不接管道**。
#          默认 scope 集 = go+rust+pub+tags+docs+**gates**（2026-09-18 第二波 ⇒ 默认全量 37 步；
#          2026-09-18 收尾再 +2 条 gofmt 步 ⇒ 39 步；2026-09-18 **③批次**再 +6 步
#          （docs +2 · pub +2 · 新 scope `tools` +1 · 新 scope `slice` +1）⇒ **45 步**）。
#   docs 的**阻断面**（拍板①；落地形态 = 每步的判定模式）：
#     阻断（rc=1 计失败项 · rc=2 计 BLOCKED）= `meta` · `name` · `D1` · `D3`（模式 `tri`）；
#     **只报告 = `D2`**（模式 `tri-report` ⇒ 红只入清单，不计失败项、不计 BLOCKED、不影响退出码）。
#     理由：D2 今天红 ≈235 条，几乎全是历史稿件里的陈旧 `文件:行` 引用（存量债），
#     直接阻断等于把提交闸锁死 ⇒ 先只报告，待存量债清到可接受再谈加严。
#     ★ 这是**改判定档位**，不是放宽 D2 的判据 —— check-doc-freshness.py 的判据一字未改。
#   ★ 2026-09-18 ③批次（门③ 存量清偿）：docs scope 再追加**两步**，**都起步 `tri-report`**（理由逐条附在步骤旁）：
#     · `docs: glossary 术语表（zh+en · 只报告）` —— `python3 scripts/check-glossary.py`（现跑 rc=1：T2 术语档 3 处；
#       T1 阻塞档 0 处 ⇒ 该脚本的 rc=1 就是「告警」不是「错」，正对 `tri-report` 那一格）；**不传 `--enforce`**：
#       术语档收敛进度未到（脚本自述「收敛后再把默认值翻过来」，那要一句令）。
#     · `docs: i18n-drift 双语漂移（只报告）` —— `bash scripts/check-i18n-drift.sh`（**现跑 rc=2**：相① 缺 en 侧 9 页 ·
#       相② 1 篇译页 source_commit 过期）。★ **如实说明**：它在 `tri-report`/`tri` 下都落 **BLOCKED**（rc=2 那一格），
#       而它自己的三档口径把 rc=2 用作「阻塞相失败」⇒ 「真红」与「不给结论」在这一点上语义错位，
#       属**待拍**（见 `scripts/precommit-gates.md` §待拍）：要么给它加一档 rc，要么把它换挂/换档。
#       本批次只按门③ 的现读建议挂上（不改它一个字），红照实进清单、不粉饰。
#
# 自检不通过 ⇒ 拒绝跑真目标（项目口径：门禁自己先能被证明「会红」）。

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── 默认 scope 集与「门自己的门」scope 名（**单来源**：main() 与自检 ⑩ 共用，不许各写一份）──────
#   ★ 2026-09-18 第二波：默认集追加 `gates`（门①②③），并把这一串提成常量 —— 自检 ⑩ 要能断言
#     「新 scope 真的在默认集里」，写两份字符串就会漂。
#   ★ 2026-09-18 ③批次（收尾二）：门③ 报的 8 只未挂门脚本逐只上岗 ⇒ 再追加两个新 scope
#     `tools`（check-tool-version-sync.sh · tri）与 `slice`（check-slice.py · tri-report），**都进默认集**
#     —— 新挂的步骤若不在默认集里跑，「上岗」就只是文件里的一行字（本仓最恨的那种假覆盖）。
#     同时把 `gates` 的门③ 由 `tri-report` 升成 `tri` + `--strict-report`（存量已清到 0）。
DEFAULT_SCOPES=(go rust pub tags docs gates tools slice)
GATE_SCOPE="gates"

# ── 步骤表（indexed arrays，bash 3.2 可用）──────────────────────────
STEP_SCOPE=()
STEP_NAME=()
STEP_MODE=()   # rc    = 退出码为 0 即通过（**非 0 一律算失败**，含 2 —— 现有 20 步语义不变）
               # empty = 退出码为 0 且输出为空才通过
               # tri   = 三档：0=PASS · 1=FAIL · 2=BLOCKED（不给结论，**不计入失败项数**）
               #         非 0/1/2 的**异常码**（3/127/…）一律按 FAIL —— 不许把「跑不起来」当「没结论」
               #         ★ 只有自己申报 tri 的步骤才走三档；rc/empty 两模式未被这一档改动（2026-09-18）
               # tri-report = **第四档「只报告」**（2026-09-18 拍板①，现用于 docs 的 D2）：
               #         与 tri **只差 rc=1 那一格** —— 那一格落 REPORT（红只入清单，**不计失败项数、
               #         不计 BLOCKED、不影响退出码**）；0 ⇒ PASS · 2 ⇒ BLOCKED · 异常码 ⇒ FAIL（三格与 tri 一致）。
               #         ★ 只报告 ≠ 放宽判据：门脚本自己的判据与 rc 一字未改，改的只是**本档位如何计账**。
               #         ★ 模式名不在 {rc,empty,tri,tri-report} 内 ⇒ 该步硬红（拼错模式名不许静默按 rc 处理）
STEP_DIR=()
STEP_CMD=()

add_step() {  # add_step <scope> <名> <模式> <工作目录> <命令>
  STEP_SCOPE+=("$1")
  STEP_NAME+=("$2")
  STEP_MODE+=("$3")
  STEP_DIR+=("$4")
  STEP_CMD+=("$5")
}

clear_steps() {
  STEP_SCOPE=(); STEP_NAME=(); STEP_MODE=(); STEP_DIR=(); STEP_CMD=()
}

# ── 运行一个步骤：输出落**自己的**文件，判定立刻追加进结果表 ────────
run_step() {  # run_step <序号> <名> <模式> <目录> <命令> <outdir> <结果表>
  local idx="$1" name="$2" mode="$3" dir="$4" cmd="$5" outdir="$6" results="$7"
  local slug log t0 t1 rc status
  slug="$(printf '%02d' "${idx}")-$(printf '%s' "${name}" | tr -c 'A-Za-z0-9._-' '_')"
  log="${outdir}/${slug}.log"
  t0="$(date +%s)"
  ( cd "${dir}" && bash -c "${cmd}" ) >"${log}" 2>&1
  rc=$?
  t1="$(date +%s)"
  status="$(_judge "${mode}" "${rc}" "${log}")"
  printf '%s\t%s\t%s\t%s\t%s\n' "${status}" "${name}" "${rc}" "$((t1 - t0))s" "${log}" >>"${results}"
  return 0
}

# ── 四档判定：**唯一的判据出口**（只看真退出码；rc 模式的语义与以前一字不差）──────
#   返回 PASS / FAIL / BLOCKED / REPORT 四值之一。判据独立成函数，是为了让自检能直接钉住每一格。
#   ★ 第四档 REPORT（只报告）只在 `tri-report` 模式下出现，且**只有 rc=1 那一格**与 `tri` 不同。
_judge() {  # _judge <模式> <rc> <日志文件>
  local mode="$1" rc="$2" log="$3"
  case "${mode}" in
    rc)
      # 老语义：rc=0 ⇒ PASS，其余（含 2）一律 FAIL —— 现有 20 步靠的就是这一句，**不许动**
      [ "${rc}" -eq 0 ] && { printf 'PASS'; return 0; }
      printf 'FAIL'
      ;;
    empty)
      # 判据是「输出必须为空」，非空 ⇒ 红
      if [ "${rc}" -ne 0 ]; then printf 'FAIL'; return 0; fi
      if [ -s "${log}" ]; then printf 'FAIL'; return 0; fi
      printf 'PASS'
      ;;
    tri)
      # 三档：0=PASS · 1=FAIL · 2=BLOCKED；**异常码（3/127/…）= FAIL**（跑不起来不是「没结论」）
      case "${rc}" in
        0) printf 'PASS' ;;
        1) printf 'FAIL' ;;
        2) printf 'BLOCKED' ;;
        *) printf 'FAIL' ;;
      esac
      ;;
    tri-report)
      # 四档之「只报告」：与 tri **只差 rc=1 那一格** ——
      #   rc=1 ⇒ REPORT（红只入清单：不计失败项数、不计 BLOCKED、不影响退出码）
      #   rc=0 ⇒ PASS · rc=2 ⇒ BLOCKED（判据不可判/空转 —— 不许当绿，与 tri 同口径）
      #   异常码 ⇒ FAIL（跑不起来不许借这一档洗成「只是报告一下」）
      case "${rc}" in
        0) printf 'PASS' ;;
        1) printf 'REPORT' ;;
        2) printf 'BLOCKED' ;;
        *) printf 'FAIL' ;;
      esac
      ;;
    *)
      # 模式名不在表内（拼错 / 漏写）⇒ 硬红：静默按 rc 处理会造出「没人判过的绿」
      printf 'FAIL'
      ;;
  esac
  return 0
}

run_suite() {  # run_suite <outdir> <结果表>  —— 遍历当前步骤表
  local outdir="$1" results="$2" i
  mkdir -p "${outdir}"
  : >"${results}"
  i=0
  while [ "${i}" -lt "${#STEP_NAME[@]}" ]; do
    run_step "${i}" "${STEP_NAME[${i}]}" "${STEP_MODE[${i}]}" "${STEP_DIR[${i}]}" "${STEP_CMD[${i}]}" "${outdir}" "${results}"
    i=$((i + 1))
  done
  return 0
}

# ── 失败项数 / 不给结论数 / 只报告数：**只从结果表数出来**（纯读文件，不在任何地方累加）──────
#    ★ 三个数是**分开的**：BLOCKED（没结论）不许计进失败项数，也不许被失败项数吞掉；
#      REPORT（只报告）两者都不进 —— 它只出现在计数与打印里，**不影响退出码**。
count_fail() {  # count_fail <结果表>
  grep -c '^FAIL' "$1" 2>/dev/null || true
}

count_blocked() {  # count_blocked <结果表> —— 第三档：rc=2「不给结论」的步数
  grep -c '^BLOCKED' "$1" 2>/dev/null || true
}

count_report() {  # count_report <结果表> —— 第四档「只报告」的步数（**不参与退出码**）
  grep -c '^REPORT' "$1" 2>/dev/null || true
}

count_pass() {  # count_pass <结果表>
  grep -c '^PASS' "$1" 2>/dev/null || true
}

# ── 退出码判定：**唯一的出口**（自检直接钉在它上面，才能断言「哪一种状态影响退出码」）──
#   0 全绿 · 1 有失败项 · 2 无失败项但有「不给结论」（BLOCKED 不许当绿）
#   ★ REPORT（只报告）**不进这三档**：它既不是「错」也不是「没结论」⇒ 不影响退出码。
#   ★ 判据只看结果表的状态列（与 count_* 同一口径、同一个文件）。
_exit_rc() {  # _exit_rc <结果表>  ⇒ 打印 0 / 1 / 2
  if [ "$(count_fail "$1")" -ne 0 ]; then printf '1'; return 0; fi
  if [ "$(count_blocked "$1")" -ne 0 ]; then printf '2'; return 0; fi
  printf '0'
}

# ── 旧写法（第十三轮那个假绿脚本的形态）—— 只用于自检里的区分度证明 ──
legacy_count() {  # 模拟：计数在子壳里自增 ⇒ 永远回到 0
  local fails=0
  (
    rc=1
    if [ "${rc}" -ne 0 ]; then
      fails=$((fails + 1))
    fi
  )
  printf '%s' "${fails}"
}

print_steps() {
  local i=0
  while [ "${i}" -lt "${#STEP_NAME[@]}" ]; do
    printf '   %-5s %-6s %s\n' "${STEP_SCOPE[${i}]}" "${STEP_MODE[${i}]}" "${STEP_NAME[${i}]}"
    i=$((i + 1))
  done
}

report() {  # report <结果表>
  local results="$1" status name rc secs log
  printf '%-7s %-50s %-5s %s\n' "状态" "步骤" "rc" "耗时"
  while IFS=$'\t' read -r status name rc secs log; do
    [ -n "${status}" ] || continue
    printf '%-7s %-50s %-5s %s\n' "${status}" "${name}" "${rc}" "${secs}"
    if [ "${status}" = "FAIL" ]; then
      printf '      ↳ 日志: %s\n' "${log}"
      tail -12 "${log}" | sed 's/^/      | /'
    elif [ "${status}" = "BLOCKED" ]; then
      # 第三档：**不是失败**，但也**不算绿** —— 照样要求人看原文（它就是「没结论」的取证）
      printf '      ↳ 不给结论（rc=2 · 不计入失败项数）· 日志: %s\n' "${log}"
      tail -6 "${log}" | sed 's/^/      ~ /'
    elif [ "${status}" = "REPORT" ]; then
      # 第四档「只报告」：**不计失败项数 · 不计 BLOCKED · 不影响退出码** —— 但清单必须可取证，
      # 所以照 FAIL/BLOCKED 一样打出日志路径与原文（前缀 `·`，措辞写明「只报告」）。
      printf '      ↳ 只报告（不计失败项数 · 不计 BLOCKED · 不影响退出码）· 日志: %s\n' "${log}"
      tail -12 "${log}" | sed 's/^/      · /'
    fi
  done <"${results}"
  printf '── 状态计数：PASS %s · FAIL %s · BLOCKED(不给结论) %s · REPORT(只报告) %s ──\n' \
    "$(count_pass "${results}")" "$(count_fail "${results}")" \
    "$(count_blocked "${results}")" "$(count_report "${results}")"
}

# ── 自检：证明这台镜子「会红」，且与旧写法有区分度 ──────────────────
SELF_OK=0
SELF_BAD=0
assert_eq() {  # assert_eq <描述> <实际> <期望>
  if [ "$2" = "$3" ]; then
    printf '   ✓ %s\n' "$1"
  else
    printf '   ✗ %s（实际=%s 期望=%s）\n' "$1" "$2" "$3"
    SELF_BAD=$((SELF_BAD + 1))
  fi
}

self_test() {
  local t
  t="$(mktemp -d "${TMPDIR:-/tmp}/zerg-gates-selftest-XXXXXX")"
  printf '── 门禁自检（合成步骤 · 不碰真目标）──\n'

  # ① 全过 ⇒ 计 0、结论 0
  clear_steps
  add_step self "自检-必过-1" rc "${t}" "true"
  add_step self "自检-必过-2" rc "${t}" "echo 正常输出"
  run_suite "${t}/a" "${t}/a.tsv" >/dev/null 2>&1
  assert_eq "① 全过：失败数" "$(count_fail "${t}/a.tsv")" "0"

  # ② 一条真红 ⇒ 计 1，且**点名到那条**、它的日志留着原文
  clear_steps
  add_step self "自检-必红" rc "${t}" "echo 这是红步骤的原文标记; exit 3"
  add_step self "自检-必过-3" rc "${t}" "echo 这是绿步骤的原文标记"
  run_suite "${t}/b" "${t}/b.tsv" >/dev/null 2>&1
  assert_eq "② 一条红：失败数" "$(count_fail "${t}/b.tsv")" "1"
  assert_eq "② 一条红：点名到红步骤" "$(grep '^FAIL' "${t}/b.tsv" | cut -f2)" "自检-必红"

  # ③ 覆盖 bug 的镜子：两个步骤的日志必须是**不同文件**且各自留着原文
  local logred loggreen
  logred="$(grep '^FAIL' "${t}/b.tsv" | cut -f5)"
  loggreen="$(grep '^PASS' "${t}/b.tsv" | cut -f5)"
  assert_eq "③ 日志文件各不相同" "$([ "${logred}" != "${loggreen}" ] && echo 不同 || echo 相同)" "不同"
  assert_eq "③ 红步骤原文仍在" "$(grep -c '这是红步骤的原文标记' "${logred}")" "1"
  assert_eq "③ 绿步骤原文仍在" "$(grep -c '这是绿步骤的原文标记' "${loggreen}")" "1"

  # ④ empty 模式：判据是「输出必须为空」
  clear_steps
  add_step self "自检-empty-非空" empty "${t}" "echo 有东西"
  run_suite "${t}/c" "${t}/c.tsv" >/dev/null 2>&1
  assert_eq "④ empty 模式遇到非空输出 ⇒ 红" "$(count_fail "${t}/c.tsv")" "1"
  clear_steps
  add_step self "自检-empty-空" empty "${t}" "true"
  run_suite "${t}/d" "${t}/d.tsv" >/dev/null 2>&1
  assert_eq "④ empty 模式遇到空输出 ⇒ 绿" "$(count_fail "${t}/d.tsv")" "0"

  # ⑤ 区分度：同一组「有一步真红」的步骤，旧写法（子壳里累加）报 0 = 假绿
  assert_eq "⑤ 旧写法在同一步骤集下假绿（复现第十三轮的病）" "$(legacy_count)" "0"
  clear_steps
  add_step self "自检-必红-2" rc "${t}" "exit 1"
  run_suite "${t}/e" "${t}/e.tsv" >/dev/null 2>&1
  assert_eq "⑤ 新写法同一步骤集下报真红（镜子有区分度）" "$(count_fail "${t}/e.tsv")" "1"

  # ⑥ 报错路径：工作目录不存在 ⇒ 该步必须红，不许静默算过
  clear_steps
  add_step self "自检-目录不存在" rc "${t}/不存在" "true"
  run_suite "${t}/f" "${t}/f.tsv" >/dev/null 2>&1
  assert_eq "⑥ 工作目录不存在 ⇒ 硬红（不静默跳过）" "$(count_fail "${t}/f.tsv")" "1"

  # ⑦ 新步骤（无后缀脚本语法，pub scope）的负控：**同一串命令**（nosuffix_syntax_cmd）对着合成件跑。
  #    三格都走真命令行 + 真退出码：坏件必红 · 好件必绿 · 空转（0 个被检查到）必红。
  #    **不碰仓内脚本**（新增一步的负控不许为了证明而改真目标）。
  mkdir -p "${t}/negctl/scripts" "${t}/negctl-ok/scripts" "${t}/negctl-empty/scripts"
  printf '#!/usr/bin/env python3\ndef broken(:\n'        > "${t}/negctl/scripts/broken-py"
  printf '#!/bin/bash\nif [ 1 -eq 1 ; then echo x\n'        > "${t}/negctl/scripts/broken-sh"
  clear_steps
  add_step self "自检-无后缀语法-坏件" rc "${t}/negctl" "$(nosuffix_syntax_cmd scripts)"
  run_suite "${t}/g" "${t}/g.tsv" >/dev/null 2>&1
  assert_eq "⑦ 无后缀语法（同一条命令串）：坏 python/sh 件 ⇒ 必红" "$(count_fail "${t}/g.tsv")" "1"
  printf '#!/usr/bin/env python3\ndef ok():\n    return 1\n' > "${t}/negctl-ok/scripts/ok-py"
  printf '#!/bin/bash\necho ok\n'                          > "${t}/negctl-ok/scripts/ok-sh"
  clear_steps
  add_step self "自检-无后缀语法-好件" rc "${t}/negctl-ok" "$(nosuffix_syntax_cmd scripts)"
  run_suite "${t}/h" "${t}/h.tsv" >/dev/null 2>&1
  assert_eq "⑦ 无后缀语法（同一条命令串）：好件 ⇒ 绿（与坏件有区分度）" "$(count_fail "${t}/h.tsv")" "0"
  clear_steps
  add_step self "自检-无后缀语法-空转" rc "${t}/negctl-empty" "$(nosuffix_syntax_cmd scripts)"
  run_suite "${t}/i" "${t}/i.tsv" >/dev/null 2>&1
  assert_eq "⑦ 无后缀语法：0 个被检查到 ⇒ 必红（空转 = 假覆盖）" "$(count_fail "${t}/i.tsv")" "1"

  # ⑦′ 第二波新步骤（四棵外围目录 + 顶层入口 脚本语法）的负控：**同一串命令**（ext_syntax_cmd）。
  #     三格都走真命令行 + 真退出码：坏件必红 · 好件必绿 · 空转（0 个被检查到）必红；
  #     外加第四格：**排除面要有牙** —— venv 里的坏件**不许**把好件树判红（否则排除名单是装饰）。
  #     **不碰仓内脚本**（负控不许为了证明而改真目标）。
  mkdir -p "${t}/ext/negctl/mcp" "${t}/ext/negctl-ok/mcp" "${t}/ext/negctl-ok/gateway" \
           "${t}/ext/negctl-ok/tools/ocr/venv" "${t}/ext/negctl-empty/mcp"
  printf '#!/usr/bin/env python3\ndef ok():\n    return 1\n'   > "${t}/ext/negctl/mcp/ok.py"
  printf '#!/usr/bin/env python3\ndef broken(:\n'                > "${t}/ext/negctl/mcp/broken.py"
  printf '#!/bin/bash\nif [ 1 -eq 1 ; then echo x\n'             > "${t}/ext/negctl/mcp/broken.sh"
  printf '#!/bin/bash\necho core\n'                              > "${t}/ext/negctl/start-zerg-core.sh"
  printf '#!/bin/bash\necho ui\n'                                > "${t}/ext/negctl/start-zerg-ui.sh"
  clear_steps
  add_step self "自检-外围语法-坏件" rc "${t}/ext/negctl" "$(ext_syntax_cmd .)"
  run_suite "${t}/ext/g1" "${t}/ext/g1.tsv" >/dev/null 2>&1
  assert_eq "⑦′ 外围脚本语法（同一条命令串）：坏 .py/.sh 件 ⇒ 必红" "$(count_fail "${t}/ext/g1.tsv")" "1"
  printf '#!/usr/bin/env python3\ndef ok():\n    return 1\n'     > "${t}/ext/negctl-ok/mcp/ok.py"
  printf '#!/bin/bash\necho ok\n'                                > "${t}/ext/negctl-ok/gateway/ok.sh"
  printf '#!/usr/bin/env python3\ndef broken_in_venv(:\n'        > "${t}/ext/negctl-ok/tools/ocr/venv/broken.py"
  printf '#!/bin/bash\necho core\n'                              > "${t}/ext/negctl-ok/start-zerg-core.sh"
  printf '#!/bin/bash\necho ui\n'                                > "${t}/ext/negctl-ok/start-zerg-ui.sh"
  clear_steps
  add_step self "自检-外围语法-好件" rc "${t}/ext/negctl-ok" "$(ext_syntax_cmd .)"
  run_suite "${t}/ext/g2" "${t}/ext/g2.tsv" >/dev/null 2>&1
  assert_eq "⑦′ 外围脚本语法：好件 ⇒ 绿（与坏件有区分度）" "$(count_fail "${t}/ext/g2.tsv")" "0"
  # 成对第四格：**排除面有牙** —— 上面那棵好件树里**故意塞了一个坏件**在 `tools/ocr/venv/`
  # （被排除）⇒ 它仍然绿；把同一个坏件挪到 `mcp/`（不被排除）⇒ 立刻红。
  # 没有这一对，「绿」就可能只是「压根没扫到 venv」蒙过去的（排除名单当装饰）。
  mkdir -p "${t}/ext/negctl-venv-moved"
  cp -R "${t}/ext/negctl-ok/." "${t}/ext/negctl-venv-moved/"
  mv "${t}/ext/negctl-venv-moved/tools/ocr/venv/broken.py" \
     "${t}/ext/negctl-venv-moved/mcp/broken-venv-moved.py"
  clear_steps
  add_step self "自检-外围语法-排除面" rc "${t}/ext/negctl-venv-moved" "$(ext_syntax_cmd .)"
  run_suite "${t}/ext/g2b" "${t}/ext/g2b.tsv" >/dev/null 2>&1
  assert_eq "⑦′ 排除面有牙（成对）：同一坏件在 venv 里 ⇒ 绿" "$(count_fail "${t}/ext/g2.tsv")" "0"
  assert_eq "⑦′ 排除面有牙（成对）：同一坏件挪到 mcp/ ⇒ 必红" "$(count_fail "${t}/ext/g2b.tsv")" "1"
  clear_steps
  add_step self "自检-外围语法-空转" rc "${t}/ext/negctl-empty" "$(ext_syntax_cmd .)"
  run_suite "${t}/ext/g3" "${t}/ext/g3.tsv" >/dev/null 2>&1
  assert_eq "⑦′ 外围脚本语法：0 个被检查到 ⇒ 必红（空转 = 假覆盖）" "$(count_fail "${t}/ext/g3.tsv")" "1"

  # ⑧ **三档（tri）**：0/1/2 三格各就各位 —— 2 = BLOCKED（不给结论）
  #    这是本路新增的那一档的镜子：**rc=2 不许计进失败项数**（「没结论」≠「错」），
  #    但也不许当绿；而**异常码（3/127/…）必须按红算**（跑不起来不许冒充「没结论」）。
  clear_steps
  add_step self "自检-三档-绿"       tri "${t}" "exit 0; echo 三档绿件原文"
  add_step self "自检-三档-红"       tri "${t}" "echo 三档红件原文; exit 1"
  add_step self "自检-三档-不给结论" tri "${t}" "echo 三档BLOCKED件原文; exit 2"
  run_suite "${t}/j" "${t}/j.tsv" >/dev/null 2>&1
  assert_eq "⑧ 三档：0/1/2 各就各位（三行状态依次 PASS/FAIL/BLOCKED）" \
    "$(cut -f1 "${t}/j.tsv" | tr '\n' ' ')" "PASS FAIL BLOCKED "
  assert_eq "⑧ 三档：PASS 数" "$(count_pass "${t}/j.tsv")" "1"
  assert_eq "⑧ 三档：**失败项数只数 rc=1**（rc=2 不许计进去）" "$(count_fail "${t}/j.tsv")" "1"
  assert_eq "⑧ 三档：BLOCKED 数 = 1" "$(count_blocked "${t}/j.tsv")" "1"
  assert_eq "⑧ 三档：BLOCKED 点名到那一步" "$(grep '^BLOCKED' "${t}/j.tsv" | cut -f2)" "自检-三档-不给结论"
  assert_eq "⑧ 三档：BLOCKED 那步的 rc 真是 2" "$(grep '^BLOCKED' "${t}/j.tsv" | cut -f3)" "2"
  assert_eq "⑧ 三档：BLOCKED 的原文仍留在它自己的日志里" \
    "$(grep -c '三档BLOCKED件原文' "$(grep '^BLOCKED' "${t}/j.tsv" | cut -f5)")" "1"

  # ⑧b **异常码不是「没结论」**：exit 3 / exit 127 ⇒ 红的红灯亮，BLOCKED 数必须是 0
  clear_steps
  add_step self "自检-三档-异常码3"   tri "${t}" "exit 3"
  add_step self "自检-三档-异常码127" tri "${t}" "exit 127"
  run_suite "${t}/k" "${t}/k.tsv" >/dev/null 2>&1
  assert_eq "⑧b 三档：非 0/1/2 的异常码 ⇒ 计失败项（不许被吞成「不给结论」）" "$(count_fail "${t}/k.tsv")" "2"
  assert_eq "⑧b 三档：异常码下 BLOCKED 数 = 0" "$(count_blocked "${t}/k.tsv")" "0"

  # ⑧c 反向：**只有 BLOCKED、没有 FAIL** ⇒ 失败项数为 0（这就是拍板口径「没结论不算错」的落点），
  #     但它也不许被当成绿 —— 整脚本的出口在 main 里按「无 FAIL 而有 BLOCKED ⇒ rc=2」处理。
  clear_steps
  add_step self "自检-三档-全是不给结论" tri "${t}" "exit 2"
  run_suite "${t}/l" "${t}/l.tsv" >/dev/null 2>&1
  assert_eq "⑧c 只有 BLOCKED：失败项数 = 0" "$(count_fail "${t}/l.tsv")" "0"
  assert_eq "⑧c 只有 BLOCKED：BLOCKED 数 = 1（没结论 ≠ 绿）" "$(count_blocked "${t}/l.tsv")" "1"

  # ⑧d **老语义未被三档改动**：同一串 rc=2，用 `rc` 模式 ⇒ 仍旧算失败项（现有 20 步靠这一句）
  clear_steps
  add_step self "自检-老rc模式遇rc2" rc "${t}" "exit 2"
  run_suite "${t}/m" "${t}/m.tsv" >/dev/null 2>&1
  assert_eq "⑧d rc 模式遇 rc=2 仍是失败项（老语义一字未改）" "$(count_fail "${t}/m.tsv")" "1"
  assert_eq "⑧d rc 模式下 BLOCKED 数 = 0" "$(count_blocked "${t}/m.tsv")" "0"

  # ⑧e 模式名打错 ⇒ 硬红（不静默按 rc 处理，也就不会造出没人判过的绿）
  clear_steps
  add_step self "自检-模式名打错" tri2 "${t}" "exit 0"
  run_suite "${t}/n" "${t}/n.tsv" >/dev/null 2>&1
  assert_eq "⑧e 未知模式名 ⇒ 硬红" "$(count_fail "${t}/n.tsv")" "1"

  # ⑨ **第四档「只报告」（tri-report）**：本批拍板①的落地格 —— 现用于 docs 的 D2。
  #    镜子要钉住的是「同一串 rc=1，走 `tri` 计失败项、走 `tri-report` 只入清单」这一对
  #    （= 「D1 影响退出码 / D2 不影响退出码」的机制本体），而不只是某个计数好看。
  clear_steps
  add_step self "自检-只报告-rc1" tri-report "${t}" "echo 只报告件原文; exit 1"
  add_step self "自检-阻断-rc1"   tri        "${t}" "echo 阻断件原文; exit 1"
  run_suite "${t}/o" "${t}/o.tsv" >/dev/null 2>&1
  assert_eq "⑨ 只报告档遇 rc=1 ⇒ 状态 REPORT（不是 FAIL）" "$(cut -f1 "${t}/o.tsv" | tr '\n' ' ')" "REPORT FAIL "
  assert_eq "⑨ 两步的 rc 都真是 1（区分度来自模式，不是 rc）" "$(cut -f3 "${t}/o.tsv" | tr '\n' ' ')" "1 1 "
  assert_eq "⑨ 只报告档：失败项数只数阻断那一步" "$(count_fail "${t}/o.tsv")" "1"
  assert_eq "⑨ 只报告档：REPORT 数 = 1" "$(count_report "${t}/o.tsv")" "1"
  assert_eq "⑨ 只报告档**不计入 BLOCKED**（「只报告」≠「没结论」）" "$(count_blocked "${t}/o.tsv")" "0"
  assert_eq "⑨ REPORT 那步的原文仍留在它自己的日志里" \
    "$(grep -c '只报告件原文' "$(grep '^REPORT' "${t}/o.tsv" | cut -f5)")" "1"

  # ⑨b **成对断言（本批的硬要求 · 有区分度）**：同样只有「一步 rc=1 的红」，
  #     走只报告档 ⇒ `_exit_rc` = 0（**不影响退出码**）；走阻断档 ⇒ `_exit_rc` = 1（**影响退出码**）。
  #     两格用的是 main() 同一条出口（`_exit_rc`），所以断言的就是整脚本退出码本身。
  clear_steps
  add_step self "自检-只报告-单独" tri-report "${t}" "exit 1"
  run_suite "${t}/p" "${t}/p.tsv" >/dev/null 2>&1
  assert_eq "⑨b 只有「只报告」红（= D2 型）⇒ 退出码 = 0（不影响）" "$(_exit_rc "${t}/p.tsv")" "0"
  assert_eq "⑨b 同上一格：失败项数 = 0 · REPORT = 1" \
    "$(count_fail "${t}/p.tsv")/$(count_report "${t}/p.tsv")" "0/1"
  clear_steps
  add_step self "自检-阻断-单独" tri "${t}" "exit 1"
  run_suite "${t}/q" "${t}/q.tsv" >/dev/null 2>&1
  assert_eq "⑨b 只有「阻断」红（= D1 型）⇒ 退出码 = 1（影响）—— 与上一格成对有区分度" \
    "$(_exit_rc "${t}/q.tsv")" "1"

  # ⑨c 只报告档的**另外三格与 `tri` 一字不差**：rc=2 ⇒ BLOCKED（判据不可判/空转 ⇒ 不许当绿）、
  #     异常码 ⇒ FAIL（跑不起来不许借这一档洗成「只是报告了一下」）。
  clear_steps
  add_step self "自检-只报告-不给结论" tri-report "${t}" "exit 2"
  add_step self "自检-只报告-异常码"   tri-report "${t}" "exit 127"
  run_suite "${t}/r" "${t}/r.tsv" >/dev/null 2>&1
  assert_eq "⑨c 只报告档：rc=2 仍是 BLOCKED、异常码仍按红" "$(cut -f1 "${t}/r.tsv" | tr '\n' ' ')" "BLOCKED FAIL "
  assert_eq "⑨c 只报告档：异常码计失败项" "$(count_fail "${t}/r.tsv")" "1"
  assert_eq "⑨c 只报告档：BLOCKED 数 = 1（不许当绿）" "$(count_blocked "${t}/r.tsv")" "1"
  assert_eq "⑨c 只报告档：这一组退出码仍是 1（失败优先于不给结论）" "$(_exit_rc "${t}/r.tsv")" "1"

  # ⑩ **「门自己的门」三步：档位申报 + 三档各就各位**（第二波挂接 · 2026-09-18 ③批次同步升档）
  #    镜子取的是**步骤表里那三步真申报的模式**（不是合成模式名）⇒ 一次钉两件事：
  #      ① 三步申报的档位对不对：`tri` × 3（**门①②③ 全阻断** —— ③批次把门③ 由 `tri-report` 升上来的，
  #         因为它的存量命中 8→0 已清；且它的命令串必须带 `--strict-report`，否则就是「恒绿的假阻断」）；
  #      ② 每一档在 0/1/2 三格上的计账（0 ⇒ PASS · 1 ⇒ 阻断进失败项 · 2 ⇒ BLOCKED 且**不计失败项**、
  #         也不许当绿）；「只报告 vs 阻断」的区分度由 ⑨/⑨b/⑨c 用**同一串 rc=1 的合成成对**继续钉住
  #         （本组末尾的成对断言改成跨档对照：门③（阻断）⇒ 退出码 1 · ⑨b 那步（只报告）⇒ REPORT 1 且退出码 0）。
  #    ★ 谁把门③ 改回只报告、或把门① 改成只报告、或给门③ 摘掉 `--strict-report`，本组立刻红；
  #      谁给新步骤接管道取 rc，也立刻红。
  clear_steps
  build_steps "${GATE_SCOPE}" >/dev/null 2>&1
  assert_eq "⑩ ${GATE_SCOPE} scope：步数" "${#STEP_NAME[@]}" "3"
  assert_eq "⑩ 三步申报的档位（门①②③ 全阻断）" \
    "$(printf '%s ' "${STEP_MODE[@]}")" "tri tri tri "
  assert_eq "⑩ 三步的命令串分别点名三只新门脚本" \
    "$(printf '%s\n' "${STEP_CMD[@]}" | grep -cE 'check-gate-coverage\.py|check-version-sources\.py|check-wired-scripts\.py')" "3"
  assert_eq "⑩ 门③ 升阻断后命令串**必须带 --strict-report**（不带 = 只挂了个恒绿的步）" \
    "$(printf '%s\n' "${STEP_CMD[2]}" | grep -c -- '--strict-report')" "1"
  assert_eq "⑩ 三步都不接管道取 rc（命令串里 0 个竖线）" \
    "$(printf '%s\n' "${STEP_CMD[@]}" | grep -c '|')" "0"
  assert_eq "⑩ 三步的工作目录都 = 仓根" \
    "$(printf '%s\n' "${STEP_DIR[@]}" | grep -c "^${REPO_ROOT}$")" "3"
  assert_eq "⑩ ${GATE_SCOPE} 已在默认集（DEFAULT_SCOPES）里" \
    "$(printf '%s\n' "${DEFAULT_SCOPES[@]}" | grep -c "^${GATE_SCOPE}$")" "1"
  local gm0 gm1 gm2 gmode gi
  gm0="${STEP_MODE[0]}"; gm1="${STEP_MODE[1]}"; gm2="${STEP_MODE[2]}"
  gi=1
  for gmode in "${gm0}" "${gm1}" "${gm2}"; do
    clear_steps; add_step self "自检-新门-rc0" "${gmode}" "${t}" "exit 0"
    run_suite "${t}/z${gi}a" "${t}/z${gi}a.tsv" >/dev/null 2>&1
    assert_eq "⑩ 门${gi}（${gmode}）rc=0 ⇒ PASS" "$(cut -f1 "${t}/z${gi}a.tsv")" "PASS"
    clear_steps; add_step self "自检-新门-rc1" "${gmode}" "${t}" "exit 1"
    run_suite "${t}/z${gi}b" "${t}/z${gi}b.tsv" >/dev/null 2>&1
    if [ "${gmode}" = "tri-report" ]; then
      assert_eq "⑩ 门${gi}（只报告）rc=1 ⇒ REPORT+1 且**不进失败项**" \
        "$(count_report "${t}/z${gi}b.tsv")/$(count_fail "${t}/z${gi}b.tsv")" "1/0"
      assert_eq "⑩ 门${gi}（只报告）rc=1 ⇒ **退出码不受影响**（= 0）" \
        "$(_exit_rc "${t}/z${gi}b.tsv")" "0"
    else
      assert_eq "⑩ 门${gi}（阻断）rc=1 ⇒ 失败项+1（不落 REPORT）" \
        "$(count_fail "${t}/z${gi}b.tsv")/$(count_report "${t}/z${gi}b.tsv")" "1/0"
      assert_eq "⑩ 门${gi}（阻断）rc=1 ⇒ 退出码 = 1" "$(_exit_rc "${t}/z${gi}b.tsv")" "1"
    fi
    clear_steps; add_step self "自检-新门-rc2" "${gmode}" "${t}" "exit 2"
    run_suite "${t}/z${gi}c" "${t}/z${gi}c.tsv" >/dev/null 2>&1
    assert_eq "⑩ 门${gi}（${gmode}）rc=2 ⇒ BLOCKED+1 且**不计失败项**" \
      "$(count_blocked "${t}/z${gi}c.tsv")/$(count_fail "${t}/z${gi}c.tsv")" "1/0"
    assert_eq "⑩ 门${gi}（${gmode}）rc=2 ⇒ 退出码 = 2（不许当绿）" \
      "$(_exit_rc "${t}/z${gi}c.tsv")" "2"
    gi=$((gi + 1))
  done
  assert_eq "⑩ 成对：同一串 rc=1 —— 门①（阻断）⇒ 退出码 1 · 门③（现同阻断）⇒ 失败项 1 且不落 REPORT" \
    "$(_exit_rc "${t}/z1b.tsv")/$(count_fail "${t}/z3b.tsv")/$(count_report "${t}/z3b.tsv")" "1/1/0"
  assert_eq "⑩ 成对（跨档）：同一串 rc=1 —— 阻断（门③，升档后）⇒ 退出码 1 · 只报告（⑨b 那步）⇒ REPORT 1 且退出码 0" \
    "$(_exit_rc "${t}/z3b.tsv")/$(count_report "${t}/p.tsv")/$(count_fail "${t}/p.tsv")" "1/1/0"

  # ⑪ **③批次新挂 6 步的挂接纪律**（本批硬要求逐条钉住；镜子取真步骤表，不取合成模式名）
  #    新挂 = docs +2（glossary · i18n-drift）· pub +2（edit-assert · mutate-scan 各自检）·
  #           新 scope `tools` +1 · 新 scope `slice` +1。钉这四条：
  #      ① 命令串**无管道**（接管道 = rc 变成管道尾的 rc，项目铁律）；
  #      ② 工作目录**都 = 仓根**（与上一批同口径）；
  #      ③ 档位申报都在表内（`rc`/`empty`/`tri`/`tri-report`），且**新挂这 6 步只用 tri / tri-report**
  #         （新步不借老档 `rc`/`empty` —— 那两档把 rc=2 也算失败/不看三档）；
  #      ④ **每一步都真的进结果表**（用同一套四档合成步骤跑一遍，四档计数之和必须 = 步数 ⇒
  #         「挂上了但没记账」这种假覆盖不可能通过）。
  #    ★ 本组不碰真目标（只用 `build_steps` 读步骤表 + 合成步骤），符合自检「不碰真目标」的口径。
  clear_steps
  build_steps docs pub tools slice >/dev/null 2>&1
  local nnew=${#STEP_NAME[@]}
  assert_eq "⑪ 新挂步所在四个 scope 的步数（docs 7 + pub 7 + tools 1 + slice 1）" "${nnew}" "16"
  assert_eq "⑪ 四只门脚本按名出现在命令串里（glossary · i18n-drift · tool-version-sync · slice）" \
    "$(printf '%s\n' "${STEP_CMD[@]}" | grep -cE 'check-glossary\.py|check-i18n-drift\.sh|check-tool-version-sync\.sh|check-slice\.py')" "4"
  assert_eq "⑪ 两只「说明书自称门」的自检按名上榜（edit-assert · mutate-scan）" \
    "$(printf '%s\n' "${STEP_CMD[@]}" | grep -cE 'edit-assert --self-test|mutate-scan --self-test')" "2"
  # 只挑本批这 6 步：**按命令串里的脚本名挑**（不按序号 —— 序号会随别人加步漂）
  local nm6="" ii=0
  while [ "${ii}" -lt "${nnew}" ]; do
    case "${STEP_CMD[${ii}]}" in
      *check-glossary.py*|*check-i18n-drift.sh*|*check-tool-version-sync.sh*|*check-slice.py*|*edit-assert*--self-test*|*mutate-scan*--self-test*)
        nm6="${nm6}${STEP_MODE[${ii}]}"$'\n' ;;
    esac
    ii=$((ii + 1))
  done
  assert_eq "⑪ 本批新挂 6 步被「点名命令」选中（不多不少）" "$(printf '%s' "${nm6}" | grep -c .)" "6"
  assert_eq "⑪ 本批新挂 6 步的档位只在 tri / tri-report 两档（新步不借 rc/empty 老档）" \
    "$(printf '%s' "${nm6}" | grep -vcE '^(tri|tri-report)$')" "0"
  assert_eq "⑪ 命令串里 0 个单竖线管道（不接管道取 rc；\`||\` 不算管道）" \
    "$(printf '%s\n' "${STEP_CMD[@]}" | grep -cE '(^|[^|])\|([^|]|$)')" "0"
  assert_eq "⑪ 工作目录全部 = 仓根" \
    "$(printf '%s\n' "${STEP_DIR[@]}" | grep -c "^${REPO_ROOT}$")" "${nnew}"
  assert_eq "⑪ 档位申报都在表内（rc / empty / tri / tri-report —— 表外模式名会被 _judge 硬红）" \
    "$(printf '%s\n' "${STEP_MODE[@]}" | grep -vcE '^(rc|empty|tri|tri-report)$')" "0"
  clear_steps
  add_step self "自检-记账-1" tri        "${t}" "exit 0"
  add_step self "自检-记账-2" tri        "${t}" "exit 1"
  add_step self "自检-记账-3" tri        "${t}" "exit 2"
  add_step self "自检-记账-4" tri-report "${t}" "exit 1"
  run_suite "${t}/acc" "${t}/acc.tsv" >/dev/null 2>&1
  assert_eq "⑪ 四档计数之和 = 步数（每一步都进结果表 ⇒ 新步不会被漏记）" \
    "$(( $(count_pass "${t}/acc.tsv") + $(count_fail "${t}/acc.tsv") + $(count_blocked "${t}/acc.tsv") + $(count_report "${t}/acc.tsv") ))" "4"

  printf '自检结论: %s（断言失败 %d 条）\n' "$([ "${SELF_BAD}" -eq 0 ] && echo 全过 || echo 不过)" "${SELF_BAD}"
  rm -rf "${t}"
  if [ "${SELF_BAD}" -eq 0 ]; then
    return 0
  fi
  return 2
}

# ── 无后缀脚本的语法检查命令（**唯一来源**：pub 真步骤与自检负控共用同一串文本）─────────────
# 为什么需要（2026-09-18 补牙 ①，实测）：pub scope 原来只收 `scripts/*.sh`（bash -n）与
# `scripts/*.py`（ast.parse）⇒ **无后缀**的门脚本（scripts/edit-assert · scripts/mutate-scan）
# **不在任何一步里** ⇒ 上千行的门脚本一个字节都没被语法检查过（假覆盖）。
# 判法：无扩展名 + 首行是 `#!` ⇒ 按 shebang 分派：python ⇒ ast.parse；sh/bash/dash/ksh/zsh ⇒ bash -n。
# 硬规矩：**一个都没检查到 ⇒ 红**（空转就是假覆盖，不许给绿）。
NOSUFFIX_CHECK_PY='
import ast, os, subprocess, sys
root = sys.argv[1] if len(sys.argv) > 1 else "scripts"
picked, bad, skipped = [], [], []
for name in sorted(os.listdir(root)):
    path = os.path.join(root, name)
    if not os.path.isfile(path) or "." in name:
        continue
    fh = open(path, "rb")
    head = fh.readline(200)
    fh.close()
    if not head.startswith(b"#!"):
        continue
    toks = head.decode("utf-8", "replace").strip().split()
    interp = os.path.basename(toks[0]) if toks else ""
    if interp == "env":
        # `#!/usr/bin/env python3` 这种：往后找第一个不以 - 开头的 token（跳过 -S / -u 之类别名）
        for t in toks[1:]:
            if not t.startswith("-"):
                interp = os.path.basename(t)
                break
    if "python" in interp:
        code = "import ast,sys;ast.parse(open(sys.argv[1],encoding=\"utf-8\").read(),filename=sys.argv[1])"
        argv = [sys.executable, "-c", code, path]
    elif interp in ("sh", "bash", "dash", "ksh", "zsh"):
        argv = ["bash", "-n", path]
    else:
        skipped.append("%s（%s）" % (path, interp))
        continue
    rc = subprocess.call(argv)
    if rc == 0:
        picked.append("%s（%s）" % (path, interp))
    else:
        bad.append("%s（%s，rc=%d）" % (path, interp, rc))
for s in skipped:
    print("⚠ 未覆盖的解释器（本步只查 bash/python）：%s" % s)
for b in bad:
    print("✗ 语法不过：%s" % b)
if not picked:
    print("✗ 这一类里 0 个被检查到 ⇒ 本步空转 = 假覆盖 ⇒ 红（若本树确实没有无后缀脚本，请改本步选择条件）")
    sys.exit(1)
print("✓ 无后缀脚本语法：%d 个（%s）%s" % (len(picked), " · ".join(picked),
                                        "· 未覆盖 %d 个" % len(skipped) if skipped else ""))
sys.exit(1 if bad else 0)
'
nosuffix_syntax_cmd() {  # nosuffix_syntax_cmd <相对目录> ⇒ 打印检查命令串（真步骤与自检负控共用）
  printf 'python3 - %s <<PYEOF\n%s\nPYEOF' "$1" "${NOSUFFIX_CHECK_PY}"
}

# ── 八棵外围目录 + 顶层入口的脚本语法检查命令（**唯一来源**：pub 真步骤与自检 ⑦′ 负控共用同一串文本）──
# 为什么需要（2026-09-18 第二波，债务台账 §0 第 3 行）：pub scope 原来只收 `scripts/*.sh|*.py` 与
# `scripts/` 无后缀件 ⇒ `mcp/`(9) · `gateway/`(9) · `publish/`(8) · `tools/`(4) 的 .py/.sh 与顶层两个
# 运行入口（`start-zerg-core.sh` / `start-zerg-ui.sh`）**不在任何一步里**（假覆盖）。
# ★ 2026-09-18 收尾（本批）：ROOTS **再加四棵**（只加一行 `ROOTS += [...]`）——
#   `ui/`(2：ui/scripts/check-i18n.py · i18n_audit.py) · `zerg-evals/`(2) · `deploy/`(1) · `core/`(1)
#   （实测 `cd <仓根> && <本串>` rc=0、被检查到 38 个 = 原 32 + 新 6，逐条语法全过 ⇒ 不需要改 tri-report）。
# 判法：一条步里**遍历**这八棵目录的 .py（`ast.parse`）与 .sh（`bash -n`），外加两个顶层入口（`bash -n`）；
#   排除 vendor/venv/node_modules/target/dist/__pycache__ 等构建与虚拟环境目录（同各门硬排除名单）。
# 硬规矩：**一个都没检查到 ⇒ 红**（空转就是假覆盖，不许给绿）；两个顶层入口缺件也 ⇒ 红。
# ★ 全篇**不用单引号**是为了能整段放进单引号 bash 字符串（与 NOSUFFIX_CHECK_PY 同法）。
EXT_SYNTAX_PY='
import ast, os, subprocess, sys
ROOTS = ["mcp", "gateway", "publish", "tools"]
ROOTS += ["ui", "zerg-evals", "deploy", "core"]   # 2026-09-18 收尾：再收四棵（同口径 .py ast.parse / .sh bash -n）
EXCL_DIRS = {"vendor", "node_modules", "__pycache__", ".git", "target", ".venv", "venv",
             "dist", "build", ".history", ".mypy_cache"}
EXCL_PREFIX = ("tools/ocr/venv",)
TOP_ENTRIES = ["start-zerg-core.sh", "start-zerg-ui.sh"]
root = sys.argv[1] if len(sys.argv) > 1 else "."


def excluded(rel):
    parts = rel.split("/")
    if any(p in EXCL_DIRS for p in parts):
        return True
    return any(rel == p or rel.startswith(p + "/") for p in EXCL_PREFIX)


picked, bad = [], []
for r in ROOTS:
    for dp, dns, fns in os.walk(os.path.join(root, r)):
        rel_dp = os.path.relpath(dp, root).replace(os.sep, "/")
        if rel_dp == ".":
            rel_dp = ""
        dns[:] = sorted(d for d in dns
                        if d not in EXCL_DIRS and not excluded(("%s/%s" % (rel_dp, d)).lstrip("/")))
        for fn in sorted(fns):
            rel = ("%s/%s" % (rel_dp, fn)).lstrip("/")
            if excluded(rel):
                continue
            ext = os.path.splitext(fn)[1]
            if ext not in (".py", ".sh"):
                continue
            path = os.path.join(root, rel)
            if ext == ".py":
                try:
                    ast.parse(open(path, encoding="utf-8").read(), filename=rel)
                    picked.append(rel)
                except SyntaxError as e:
                    bad.append("%s：%s" % (rel, e))
            else:
                rc = subprocess.call(["bash", "-n", path])
                if rc == 0:
                    picked.append(rel)
                else:
                    bad.append("%s：bash -n rc=%d" % (rel, rc))
for rel in TOP_ENTRIES:
    path = os.path.join(root, rel)
    if not os.path.isfile(path):
        bad.append("%s：顶层入口件不存在" % rel)
        continue
    rc = subprocess.call(["bash", "-n", path])
    if rc == 0:
        picked.append(rel)
    else:
        bad.append("%s：bash -n rc=%d" % (rel, rc))
for b in bad:
    print("✗ 语法不过：%s" % b)
if not picked:
    print("✗ 外围目录（mcp/ gateway/ publish/ tools/ ui/ zerg-evals/ deploy/ core/）+ 顶层入口里"
          " 0 个脚本被检查到 ⇒ 本步空转 = 假覆盖 ⇒ 红"
          "（若本树确实没有这些脚本，请改本步的选择条件）")
    sys.exit(1)
print("✓ 外围脚本语法：%d 个（mcp/ · gateway/ · publish/ · tools/ · ui/ · zerg-evals/ · deploy/ · core/"
      " + 顶层两个入口）" % len(picked))
sys.exit(1 if bad else 0)
'
ext_syntax_cmd() {  # ext_syntax_cmd <根> ⇒ 打印检查命令串（真步骤与自检 ⑦′ 负控共用同一串）
  printf "python3 - %s <<'PYEOF'\n%s\nPYEOF" "$1" "${EXT_SYNTAX_PY}"
}

# ── 真目标的步骤表 ────────────────────────────────────────────────
build_steps() {  # build_steps <scope…>
  local s
  for s in "$@"; do
    case "${s}" in
      go)
        add_step go "gofmt -l core"         empty "${REPO_ROOT}/core"  "gofmt -l ."
        add_step go "gofmt -l agent"        empty "${REPO_ROOT}/agent" "gofmt -l ."
        add_step go "core: darwin build ./..."  rc "${REPO_ROOT}/core"  "go build -buildvcs=false ./..."
        add_step go "core: linux  build ./..."  rc "${REPO_ROOT}/core"  "GOOS=linux GOARCH=amd64 go build -buildvcs=false ./..."
        add_step go "agent: darwin build ./..." rc "${REPO_ROOT}/agent" "go build -buildvcs=false ./..."
        add_step go "agent: linux  build ./..." rc "${REPO_ROOT}/agent" "GOOS=linux GOARCH=amd64 go build -buildvcs=false ./..."
        add_step go "core: go vet ./..."         rc "${REPO_ROOT}/core"  "go vet ./..."
        add_step go "agent: go vet ./..."        rc "${REPO_ROOT}/agent" "go vet ./..."
        add_step go "core: go test ./... -count=1"        rc "${REPO_ROOT}/core"  "go test ./... -count=1"
        add_step go "agent: go test ./... -count=1"       rc "${REPO_ROOT}/agent" "go test ./... -count=1"
        add_step go "core: go test -race ./... -count=1"  rc "${REPO_ROOT}/core"  "go test -race ./... -count=1"
        add_step go "agent: go test -race ./... -count=1" rc "${REPO_ROOT}/agent" "go test -race ./... -count=1"
        # ── 2026-09-18 第二波（债务台账 §0 第 2 行 / §7 建议门①）：两个**原先不在任何门里**的 Go module ──
        #   写法逐条对齐上面 core/agent 的老步骤（build/vet/test），只换工作目录 ⇒ A 段「工作目录命中」。
        # ── 2026-09-18 收尾（本批）：上面两个新 module **原先没有 gofmt 步**（只有 build/vet/test）⇒ 补齐。
        #   风格逐字对齐最上面 core/agent 的两条 gofmt 步：模式 `empty`（`gofmt -l .` 列出文件 ⇒ 非空 ⇒ 红；
        #   零命中 ⇒ 输出为空 ⇒ 绿）、工作目录 = 该 module 自己、命令串 `gofmt -l .`。
        #   实测（收尾时点 · 本机）：`cd shared && gofmt -l .` ⇒ 0 行；`cd scripts/exportnames && gofmt -l .` ⇒ 0 行。
        add_step go "gofmt -l shared"              empty "${REPO_ROOT}/shared"              "gofmt -l ."
        add_step go "gofmt -l scripts/exportnames" empty "${REPO_ROOT}/scripts/exportnames" "gofmt -l ."
        add_step go "shared: build ./..."            rc "${REPO_ROOT}/shared" "go build -buildvcs=false ./..."
        add_step go "shared: go vet ./..."           rc "${REPO_ROOT}/shared" "go vet ./..."
        add_step go "shared: go test ./... -count=1" rc "${REPO_ROOT}/shared" "go test ./... -count=1"
        # ★ scripts/exportnames **实测无 `_test.go`**（现读：`find scripts/exportnames -name '*_test.go'` = 0 条）
        #   ⇒ 按任务书「有测试就加」**不加 test 步**（加了只是 "no test files" 的空转步，不缝）。
        # ★ 它是 **main 包**：`go build ./...` 会把二进制**写进当前目录** —— 实测当场冒出
        #   7 MB `scripts/exportnames/zerg-exportnames`（未跟踪散件，已删）。⇒ 本步**必须**带 `-o`：
        #   `-o /dev/null`（`-o` 与它的值要在包模式 `./...` **之前** —— 旗标解析遇到第一个非旗标参数就停）。
        #   实测 rc=0 且仓内零新文件（同法在库模块 shared/ 上也实测 rc=0，没有 "no main packages" 报错）。
        add_step go "scripts/exportnames: build ./..." rc "${REPO_ROOT}/scripts/exportnames" "go build -buildvcs=false -o /dev/null ./..."
        add_step go "scripts/exportnames: go vet ./..." rc "${REPO_ROOT}/scripts/exportnames" "go vet ./..."
        ;;
      rust)
        add_step rust "cargo fmt --check"              rc "${REPO_ROOT}/wall" "cargo fmt --check"
        add_step rust "cargo clippy -- -D warnings"    rc "${REPO_ROOT}/wall" "cargo clippy -- -D warnings"
        add_step rust "cargo test"                     rc "${REPO_ROOT}/wall" "cargo test"
        # ── 2026-09-18 第二波（债务台账 §0 第 1 行）：ui/ 三步，与上面 wall 三步**逐行对齐** ──────────
        #   为什么必须补：`--scope rust` 原来只 add_step wall ⇒ ui（27G 构建产物 · 19 个 .rs · 16 处
        #   `#[cfg(test)]`）**不在任何 cargo 步里**，而 precheck 只验 `ui/Cargo.toml` 在不在、不验它过不过
        #   ⇒ **ui 可以编译失败而闸全绿**（本波要治的那个形态）。
        #   实测（2026-09-18 22:5x 本机 · cargo/rustc 1.98.1）：
        #     · `cargo test`  rc=0 —— 97 passed / 0 failed（121s 冷跑一次，之后 cargo 判新很快）
        #     · `cargo fmt --check` rc=1 —— ui/ 全树**未 rustfmt 过**（483 行 diff：build.rs / src/api.rs /
        #       src/modules/* 等），属**存量债**
        #     · `cargo clippy -- -D warnings` rc=101 —— 43 条（dead_code / unused import / deprecated 等）
        #   ⇒ 本波**按真退出码如实上报，不放宽、不填白名单、不缝**：三条一律 `rc` 模式（与 wall 逐行对齐），
        #     所以 fmt 与 clippy 今天会让默认全量多两个失败项 —— 这是把闸真正上岗的代价，不是判据被动过。
        #     （「要不要按 D2 先例降成只报告」是父代理的拍板项：本脚本说明书 §待拍 记了逐条数字与一行改法。）
        #   ★ 2026-09-18 收尾（本批）复测：`cargo fmt --check` 已转绿（全树 rustfmt 过）；
        #     `cargo clippy -- -D warnings` 由 **43 条 → 22 条**（语义不变的 21 条已直修：collapsible_if 5 ·
        #     useless_format 3 · manual_div_ceil 2 · op_ref 2 · unnecessary_to_owned 2 · doc_lazy_continuation 2 ·
        #     empty_line_after_doc_comments 1 · map_clone 1 · option_map_unit_fn 1 · or_insert_with→or_default 1 ·
        #     unused_import 1）；**余下 22 条全是 dead_code（17）/ 需人判（unused variable · inherent_to_string ·
        #     type_complexity · too_many_arguments ×2）——按口径只列不改、不用 `#[allow]` 盖债** ⇒ 这一步今天仍红。
        add_step rust "ui: cargo fmt --check"              rc "${REPO_ROOT}/ui" "cargo fmt --check"
        add_step rust "ui: cargo clippy -- -D warnings"    rc "${REPO_ROOT}/ui" "cargo clippy -- -D warnings"
        add_step rust "ui: cargo test"                     rc "${REPO_ROOT}/ui" "cargo test"
        ;;
      pub)
        # 本 scope 只放**公开面两侧都有**的检查：发布面专属的门禁脚本（EXCLUDES 里的
        # publish-preflight.sh / check-publish-face-sync.py / check-hardcoded-private-paths.py /
        # check-public-tree-hazards.py）在公开树里根本不存在，写进步骤表只会让公开树里的本脚本
        # 带两行「文件找不到」的红 —— 而发布面的那些闸有自己的家（scripts/publish-preflight.sh 的
        # 闸⓪~⑤，推之前跑），不必也不该塞进提交闸。
        add_step pub "scripts/*.sh 语法（bash -n）"               rc "${REPO_ROOT}" "for f in scripts/*.sh; do bash -n \"\$f\" || exit 1; done"
        add_step pub "scripts/*.py 语法（ast.parse）"              rc "${REPO_ROOT}" "python3 - <<'PYEOF'
import ast, glob, sys
bad = 0
for f in sorted(glob.glob('scripts/*.py')):
    try:
        ast.parse(open(f, encoding='utf-8').read(), filename=f)
    except SyntaxError as e:
        print('✗ %s: %s' % (f, e))
        bad += 1
sys.exit(1 if bad else 0)
PYEOF"
        # ★ 2026-09-18（补牙 ①）：无后缀脚本（scripts/edit-assert · scripts/mutate-scan）**原先不在任何
        #   一步里** —— pub scope 只收 *.sh / *.py ⇒ 这两个上千行的门脚本没被任何一步查过（假覆盖）。
        #   本步按 shebang 分派（python ⇒ ast.parse；sh ⇒ bash -n）；**空转即红**（0 个被检查到 ⇒ 不给绿）。
        #   负控在自检第 ⑦ 条：用**同一串命令**（nosuffix_syntax_cmd）对着 /tmp 的坏件/好件跑 ⇒ 必红/必绿。
        #   位置：在 run_suite 的步骤表里（**不是**脚本尾部那个 `MAIN_RC=$?` 之后的软检查位置）。
        add_step pub "scripts/*（无后缀 + 首行 #!）按 shebang 语法"  rc "${REPO_ROOT}" "$(nosuffix_syntax_cmd scripts)"
        add_step pub "check-shell-unicode-vars.py --check"         rc "${REPO_ROOT}" "python3 scripts/check-shell-unicode-vars.py --check scripts/*.sh"
        # ★ 2026-09-18 第二波（债务台账 §0 第 3 行）：**四棵外围目录 + 顶层运行入口**的脚本语法。
        #   为什么原来没有：pub scope 只收 `scripts/*.sh|*.py` + `scripts/` 无后缀件 ⇒ `mcp/`(9) ·
        #   `gateway/`(9) · `publish/`(8) · `tools/`(4) 的 .py/.sh 与顶层 `start-zerg-core.sh` /
        #   `start-zerg-ui.sh`（两个真运行入口）**一个字节都没被语法检查过**（假覆盖）。
        #   口径（与既有写法对齐）：**一条步里遍历** —— .py ⇒ `ast.parse`；.sh ⇒ `bash -n`；
        #   命令串**只有一个来源**（`ext_syntax_cmd`），自检 ⑦′ 的坏件/好件/空转负控用的是**同一串**。
        #   **零命中必红**（空转 = 假覆盖，同上面的无后缀语法步）；排除 vendor/venv/构建产物/隐藏缓存；
        #   两个顶层入口**必须存在**（缺件即红 —— 它们是 start-zerg-core/ui 的真身）。
        add_step pub "mcp/gateway/publish/tools + 顶层入口 脚本语法（ast.parse / bash -n）" rc "${REPO_ROOT}" "$(ext_syntax_cmd .)"
        # ── 2026-09-18 ③批次（门③ 断言 A 存量清偿第 ⑦⑧只）：两只「说明书自称门」的自检 ──────────
        # 为什么现在才挂：`scripts/edit-assert` 与 `scripts/mutate-scan` 是无后缀脚本、且是门（各自说明书自称门），
        #   但它们今天**只被上面那条无后缀语法步按 glob 覆盖语法**，**从没按名跑过自检** ⇒ 门③ 判它们「未挂」。
        # 挂法 = 各一步 `--self-test`（两只的自检都是「真命令行 + 真退出码」的成对负控：edit-assert 16 条 ·
        #   mutate-scan 26 条），自检不过 ⇒ 拒绝跑真目标（它们自己的口径）。
        # 档位 = `tri`（**阻断**）：两只现跑都 rc=0（16/16 · 26/26 全过）⇒ 直接上阻断档。
        # 为什么放 pub scope：本 scope 是「公开面两侧都有的脚本静态检查」，而这两只**在公开树里都在**
        #   （不在 publish/mirror-public-lib.py 的 DROP_EXACT 里）⇒ 与上面两条语法步同族、同工作目录（仓根）。
        # 硬规矩同上一批：一等步骤 · rc 取真退出码 · **命令串无管道** · 进 results.tsv 计数。
        add_step pub "edit-assert --self-test（16 条成对负控）"   tri "${REPO_ROOT}" "python3 scripts/edit-assert --self-test"
        add_step pub "mutate-scan --self-test（26 条成对负控）"   tri "${REPO_ROOT}" "python3 scripts/mutate-scan --self-test"
        ;;
      tags)
        # 双构建工程门禁（任务表 T6.3 / 设计稿 §〇 A3–A6）：tag 命名 · 两个构建都过 · vet 成对跑 ·
        # 导出面奇偶校验 · GOFLAGS 断言。**单独一个 scope**：它比 go scope 里的单侧检查慢
        # （每个模块两套配置），且口径不同（它守的是「两个构建配置等价」），不该混进现有 scope 语义。
        # --scope go 跳不到它，要单跑：bash scripts/precommit-gates.sh --scope tags
        add_step tags "双构建工程门禁（tag/构建/vet/导出面/GOFLAGS）" rc "${REPO_ROOT}" "python3 scripts/check-build-tags.py"
        ;;
      docs)
        # 文档面只读门禁（2026-09-18 路 D 挂接 · 2026-09-18 拍板① 定阻断面：D2 只报告）──────────
        # 位置：**在 run_suite 的步骤表里**（一等步骤），不是脚本尾部那个软检查位 ——
        #   尾部软门禁的 rc 会被最后一句 `exit "${MAIN_RC}"` 之外的东西吃掉（本脚本 2026-09-17
        #   实测过「报红却退 0」），所以新门禁一律走步骤表，rc 由 _judge 按真退出码判。
        # 判定：**不接管道**（不写 `| tee` / `| tail`），rc 直接取进程退出码。
        # 档位落点：这五只门脚本的退码口径本身就是 0/1/2（2 = 不给结论/空转），
        #   ⇒ 阻断的四步用 `tri`（rc=2 记 BLOCKED、**不计入失败项数**；rc>=3 跑不起来仍按红算）；
        #   ⇒ **D2 用 `tri-report`**（拍板①：它的 rc=1 落 REPORT ⇒ 红只入清单、不计失败项、
        #      不计 BLOCKED、**不影响退出码**；rc=2 依旧是 BLOCKED，不许当绿 —
        #      「判据不可判/空转」和「有红」是两回事，只报告的是后者）。
        #      为什么只 D2：它今天红 ≈235 条，几乎全是历史稿件的陈旧 `文件:行` 引用（存量债），
        #      阻断等于把提交闸锁死；**这是改档位，不是放宽 D2 的判据**（门脚本一字未改）。
        # D4（生成式参考 drift）**不挂**：它是发布面的事（重建 + 逐字节比对），不是提交闸。
        # 每只门脚本自带 --self-test，本 scope **不传 --no-self-test**：先自证「会红」再扫真目标。
        add_step docs "docs: meta --scope formal --missing=fail" tri "${REPO_ROOT}" "python3 scripts/check-doc-meta.py --scope formal --missing=fail"
        add_step docs "docs: name --scope repo"                  tri "${REPO_ROOT}" "python3 scripts/check-doc-name.py --scope repo"
        add_step docs "docs: freshness D1 引用路径存在"           tri "${REPO_ROOT}" "python3 scripts/check-doc-freshness.py d1"
        add_step docs "docs: freshness D2 引用 文件:行 有效（只报告）" tri-report "${REPO_ROOT}" "python3 scripts/check-doc-freshness.py d2"
        add_step docs "docs: freshness D3 断链断锚"               tri "${REPO_ROOT}" "python3 scripts/check-doc-freshness.py d3"
        # ── 2026-09-18 ③批次（门③ 断言 A 存量清偿第 ①②只）：术语表 + 双语漂移 ────────────────────
        # 为什么现在才挂：这两只门脚本写好了、跑起来绿过，但**从没被任何闸按名调用** ⇒ 提交时等于不存在
        #   （门③ 断言 A 的第 1/2 只）。归属 = 「与 check-doc-* 同族（文档/术语面）」· docs 已在默认集里。
        # 档位 = `tri-report`（起步）：两只的 rc=1 都表示「告警/存量债」而不是「错」——
        #   · glossary 现跑 rc=1（T2 术语档 3 处 · T1 阻塞档 0 处）
        #   · i18n-drift 现跑 rc=2（相① 缺 en 侧 9 页 + 相② 1 篇过期）⇒ 在 tri/tri-report 下都落 BLOCKED
        #     ★ 它的 rc=2 是「阻塞相失败」不是「不给结论」⇒ 语义错位已列待拍，本批次**不改它一个字**。
        # 硬规矩同上一批：一等步骤（**不是尾部软检查位**）· rc 取真退出码 · **命令串无管道** · 进 results.tsv 计数。
        add_step docs "docs: glossary 术语表 zh+en（只报告）"       tri-report "${REPO_ROOT}" "python3 scripts/check-glossary.py"
        add_step docs "docs: i18n-drift 双语漂移（只报告）"         tri-report "${REPO_ROOT}" "bash scripts/check-i18n-drift.sh"
        ;;
      gates)
        # ── 「门自己的门」（2026-09-18 第二波挂接 · 来源 = 债务台账 §7 的三条建议门）───────────────
        # 位置：**在 run_suite 的步骤表里**（一等步骤），不是脚本尾部那个软检查位 ——
        #   尾部软门禁的 rc 会被 `exit "${MAIN_RC}"` 之外的东西吃掉（本脚本 2026-09-17 实测过
        #   「报红却退 0」），所以新门禁一律走步骤表，rc 由 _judge 按真退出码判。
        # 判定：**不接管道**（不写 `| tee` / `| tail`），rc 直接取进程退出码。
        # 档位落点：三只门脚本的退码口径本身就是 0/1/2（2 = 缺件/不可判/空转）⇒
        #   ⇒ 门①/门② 用 `tri`（**阻断**：rc=1 进失败项、rc=2 记 BLOCKED 不计失败项也不当绿、
        #      rc>=3 跑不起来仍按红算）；
        #   ⇒ 门③ 用 `tri` + `--strict-report`（**阻断** · 2026-09-18 ③批次升档）：存量 8 只未挂门脚本
        #      **逐只挂完**（docs +2 · pub +2 · 新 scope tools +1 · 新 scope slice +1 · 发布闸 +2）⇒
        #      A 命中 8→0 · B 未登记 0 ⇒ 按「清到 0 再升阻断」的拍板升档：**从此新增一只未挂/未登记即红**。
        #      ★ 升档要**两处一起改**：模式 `tri-report`→`tri`，且命令串加 `--strict-report`
        #        （脚本自身默认仍只报告 rc=0；不带这个开关就等于只挂了个恒绿的步 —— 那是假阻断）。
        #      ★ 旧状态（已被事实取代，留档）：门③ 起初报「A 命中 8 · B 未登记 0」= 存量债，
        #        起步按 D2 先例只报告；升阻断路径见 scripts/check-wired-scripts.md §五（已执行）。
        # 每只门脚本自带 --self-test，本 scope **不传 --no-self-test**：先自证「会红」再扫真目标。
        # ★ 三条命令串里**没有管道** —— 自检 ⑩ 有一条断言直接钉住这一点（`grep -c '|'` = 0）。
        add_step gates "门① 覆盖：构建清单目录 + 脚本接线（阻断）" tri        "${REPO_ROOT}" "python3 scripts/check-gate-coverage.py"
        add_step gates "门② 版本源：四处同版（阻断）"              tri        "${REPO_ROOT}" "python3 scripts/check-version-sources.py"
        add_step gates "门③ 接线：scripts 门脚本有没有被闸调用（阻断）" tri "${REPO_ROOT}" "python3 scripts/check-wired-scripts.py --strict-report"
        ;;
      tools)
        # ── 2026-09-18 ③批次新 scope：工具版本三处一致（门③ 断言 A 存量清偿第 ③只）─────────────────
        # 位置：**在 run_suite 的步骤表里**（一等步骤），不是脚本尾部那个软检查位（同 docs/gates 两段的口径）。
        # 判定：**不接管道**，rc 直接取进程退出码；档位 `tri`（阻断）——脚本自带前置自检（缺 tools/versions.json
        #   或台账 0 个工具 ⇒ rc=2），0/1/2 与 tri 三档一一对应，所以**不需要**「先只报告」。
        # 现跑：台账工具数 = 141 · 抬头一致 = 141 · 抬头不一致 = 0 ⇒ rc=0（门③ 也点名它「早已自测/实跑绿」）。
        add_step tools "tools: 工具版本三处一致（台账/文档抬头/履历）" tri "${REPO_ROOT}" "bash scripts/check-tool-version-sync.sh"
        ;;
      slice)
        # ── 2026-09-18 ③批次新 scope：切片合同探针集 + 混淆矩阵门槛（门③ 断言 A 存量清偿第 ④只）──────
        # 判据 = 该脚本自己登记的 §3.5 质量门槛：**假绿率必须 0 · 假红率 ≤ 10%**，外加探针条数门槛；
        #   现跑 rc=0（探针 36 条：期望红 22 全拦住 / 期望绿 11 全放行 / 需递归 1 / 期望错 2；假绿 0 · 假红 0.0%）。
        # 档位 = `tri-report`（起步 · 门③ 的现读建议如此：它体量大、成本当时未标定）；实测 0.05s ⇒ 可升 `tri`（待拍）。
        # ★ 为什么用 `--probe` 而不是 `--selftest`：前者判的是**判据本身的质量门槛**（会红吗/会不会假红），
        #   后者只是四条自证；两个现跑都 rc=0，本批次挂前者（更强的那一个）。
        add_step slice "slice: 切片合同探针集 + 混淆矩阵门槛（只报告）" tri-report "${REPO_ROOT}" "python3 scripts/check-slice.py --probe"
        ;;
      *)
        printf '✗ 未知 scope: %s（可用: go / rust / pub / tags / docs / gates / tools / slice）\n' "${s}" >&2
        return 2
        ;;
    esac
  done
  return 0
}

# ── 前置检查：必须真在 Zerg 仓里，且构建树入口都在（四棵构建树 + 两个第二波 Go module + 发布白名单 = **7 件**）───
#   ★ 2026-09-18 收尾（本批）把 `shared/go.mod` 与 `scripts/exportnames/go.mod` 补进清单：
#     上一波给这两个 module 加了步骤，但**缺件检查没跟着补** ⇒ 那两个 module 目录整个不在时，
#     闸会照跑那几步（各步自己红、报错却是「不行的工作目录」），而不是**前置缺件 ⇒ rc=2 不给结论**。
#     同步改了 `scripts/check-gate-coverage.py` 里那句「precheck 要 …五件」的提示文字（改成七件并点名）。
precheck() {
  local miss=0 p
  for p in core/go.mod agent/go.mod shared/go.mod scripts/exportnames/go.mod wall/Cargo.toml ui/Cargo.toml publish/whitelist.txt; do
    if [ ! -f "${REPO_ROOT}/${p}" ]; then
      printf '✗ 缺少前置：%s\n' "${p}" >&2
      miss=$((miss + 1))
    fi
  done
  if [ "${miss}" -ne 0 ]; then
    printf '✗ 仓不完整（%d 件缺）⇒ 不给结论（rc=2）\n' "${miss}" >&2
    return 2
  fi
  return 0
}

main() {
  local scopes=() outdir="" list_only=0 self_only=0 emit_cmd="" t
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --scope) scopes+=("$2"); shift 2 ;;
      --outdir) outdir="$2"; shift 2 ;;
      --list) list_only=1; shift ;;
      --self-test) self_only=1; shift ;;
      --emit-cmd) emit_cmd="$2"; shift 2 ;;
      -h|--help) sed -n '19,114p' "${BASH_SOURCE[0]}"; return 0 ;;   # 打印文件头「用法…自检不过」整段（2026-09-18 ③批次：头变长 ⇒ 范围随之放宽到 19–114）
      *) printf '✗ 未知参数: %s\n' "$1" >&2; return 2 ;;
    esac
  done

  if [ "${#scopes[@]}" -eq 0 ]; then
    # 默认 scope 集 = `DEFAULT_SCOPES` 常量 = go rust pub tags docs **gates tools slice**
    #   （2026-09-18 第二波把 `gates` 追加进默认集 ⇒ 默认全量 37 步；
    #     2026-09-18 ③批次再把 `tools` 与 `slice` 追加进默认集）。
    # ★ 老步骤（go 12 · rust 3 · pub 4 · tags 1 · docs 5）的名字/命令串/模式/目录**一字未改**；
    #   第二波只**追加**：go +5（shared 3 · exportnames 2）· rust +3（ui）· pub +1（外围脚本语法）·
    #   新 scope `gates` 3 步。一条都没删、一条都没改语义。
    # ★ 2026-09-18 收尾（上一批）再**追加 2 步**（go：`gofmt -l shared` · `gofmt -l scripts/exportnames`，
    #   模式 `empty`，风格与 core/agent 那两条逐字一致）；pub 那一条步的扫描面由四棵扩到八棵（步数不变）。
    # ★ 2026-09-18 **③批次**（本批）再追加 **6 步**：docs +2（glossary · i18n-drift，`tri-report`）·
    #   pub +2（edit-assert / mutate-scan 各自检，`tri`）· 新 scope `tools` +1（check-tool-version-sync.sh，`tri`）·
    #   新 scope `slice` +1（check-slice.py --probe，`tri-report`）⇒ 默认全量 **45 步**。
    #   本批**唯一被改模式的既有步骤 = `gates` 的门③**（`tri-report` → `tri` + `--strict-report`，
    #   因为它的存量命中已被本批清到 0；名字里的档位标注同步由「只报告」改成「阻断」，
    #   命令串只**增加** `--strict-report` 一个开关 —— 步骤身份、scope、工作目录、门脚本判据都没动）。
    #   其余老步骤一条没删、一条没改语义。
    # ★ 默认跑法的退出码由阻断面决定（见文件头「docs 的阻断面」与 `gates` scope 段）：
    #   阻断步骤（含本批升档的门③）能把它拉成 rc=1 / rc=2；**只报告档**（docs 的 D2 · docs 的 glossary ·
    #   docs 的 i18n-drift · slice 的探针步）rc=1 时不参与退出码（rc=2 仍记 BLOCKED、不许当绿）。
    scopes=("${DEFAULT_SCOPES[@]}")
  fi

  # ── 探针（--emit-cmd <子串>）：只打印匹配步骤的**命令串**，不跑步骤、不给结论 ────────────────
  # 为什么在这里（自检**之前**）：它不跑任何真目标、也不产生任何判定 ⇒ 不需要先自证；
  #   输出**只有命令串本身**（供 `bash -c "$(…)"` 直接取用），所以必须躲开自检的输出。
  # 口径：命令串**只有一个来源**（nosuffix_syntax_cmd），探针不复制、不重写任何一步。
  # 用途（2026-09-18 补牙 ①：证明新增的无后缀语法那一步不是摆设）——
  #   D=/tmp/gate-neg && mkdir -p $D/scripts && cp scripts/edit-assert $D/scripts/ &&
  #   printf 'def broken(:\n' >> $D/scripts/edit-assert &&
  #   cd $D && bash -c "$( cd <仓根> && bash scripts/precommit-gates.sh --scope pub --emit-cmd 无后缀 )"
  #   ⇒ 必红（同一串命令、真退出码）
  if [ -n "${emit_cmd}" ]; then
    local es ei ehit
    ehit=0
    for es in "${scopes[@]}"; do
      clear_steps
      build_steps "${es}" || return 2
      ei=0
      while [ "${ei}" -lt "${#STEP_NAME[@]}" ]; do
        case "${STEP_NAME[${ei}]}" in
          *"${emit_cmd}"*)
            printf '%s\n' "${STEP_CMD[${ei}]}"
            ehit=$((ehit + 1))
            ;;
        esac
        ei=$((ei + 1))
      done
    done
    if [ "${ehit}" -eq 0 ]; then
      printf '✗ 步骤名里没有匹配 %s 的（scope %s）\n' "${emit_cmd}" "${scopes[*]}" >&2
      exit 2
    fi
    # **立刻出口**：本探针的 stdout 必须是**纯粹的命令串**（调用方要 `bash -c "$(…)"` 直接取用）
    # ⇒ 不能让它被「尾部软门禁」的输出（check-zh-en.py 那一段）污染。
    exit 0
  fi

  # 自检先跑（--self-test 时只跑自检）
  if ! self_test; then
    printf '✗ 门禁自检不过 ⇒ 拒绝跑真目标（rc=2）\n' >&2
    return 2
  fi
  if [ "${self_only}" -eq 1 ]; then
    return 0
  fi

  precheck || return 2

  export GOFLAGS="${GOFLAGS:--mod=mod}"
  export GOSUMDB="${GOSUMDB:-off}"
  export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
  export CARGO_TERM_COLOR="${CARGO_TERM_COLOR:-never}"

  clear_steps
  build_steps "${scopes[@]}" || return 2

  if [ "${list_only}" -eq 1 ]; then
    printf '── 步骤清单（scope 模式 名称）──\n'
    print_steps
    printf '共 %d 步\n' "${#STEP_NAME[@]}"
    return 0
  fi

  if [ -z "${outdir}" ]; then
    outdir="${TMPDIR:-/tmp}/zerg-gates-$(date +%Y%m%d-%H%M%S)"
  fi
  mkdir -p "${outdir}"
  local results="${outdir}/results.tsv"

  printf '── 提交前门禁 · 仓根 %s · scope %s ──\n' "${REPO_ROOT}" "${scopes[*]}"
  printf '── 日志目录 %s（一步一文件）──\n' "${outdir}"
  run_suite "${outdir}" "${results}"
  report "${results}" | tee "${outdir}/report.txt"

  local nfail nblock npass nreport rc
  nfail="$(count_fail "${results}")"
  nblock="$(count_blocked "${results}")"
  npass="$(count_pass "${results}")"
  nreport="$(count_report "${results}")"
  # ★ 退出码走**唯一的出口** `_exit_rc`：FAIL ⇒ 1 · （无 FAIL 而）BLOCKED ⇒ 2 · 其余 ⇒ 0。
  #   REPORT（只报告）**不在这三档里** —— 它只进上面的计数与清单，不影响退出码（拍板①）。
  rc="$(_exit_rc "${results}")"
  printf '\n步骤总数: %d · 通过: %s · 失败项数: %s · 不给结论(BLOCKED): %s · 只报告(REPORT): %s\n' \
    "${#STEP_NAME[@]}" "${npass}" "${nfail}" "${nblock}" "${nreport}"
  if [ "${rc}" = "1" ]; then
    printf '⇒ 门禁红灯（rc=1）：逐条看上面的日志路径重跑\n'
    return 1
  fi
  if [ "${rc}" = "2" ]; then
    printf '⇒ 无失败项，但有 %s 步「不给结论」（BLOCKED · rc=2）：**不许当绿**、也不按「错」计\n' "${nblock}"
    return 2
  fi
  if [ "${nreport}" -ne 0 ]; then
    printf '（另有 %s 步「只报告」（REPORT）：已在清单里 · 不计失败项数 · 不计 BLOCKED · 不影响退出码）\n' "${nreport}"
  fi
  printf '⇒ 门禁全绿（rc=0）\n'
  return 0
}

main "$@"
MAIN_RC=$?


# ── 中英文与术语检查（软门禁·报告不阻断；2026-09-16 Mr2109 拍板）────────────────────────
# 为什么是软的：该检查器本日连修四轮假红（见 docs/skills/code-quality.md §8），
# 若直接设为阻断，一个假红就会卡住提交——"门禁不可信"的伤害大于漏检。先观察。
# 转硬条件：连续多次提交无假红 ⇒ 把下面的 `|| echo …` 去掉、改为 `|| exit 1` 即可。
if [ -f scripts/check-zh-en.py ]; then
  echo "── 软门禁：中英文与术语检查（报告不阻断）──"
  python3 scripts/check-zh-en.py || echo "⚠ 中英文/术语检查有命中（不阻断提交）——见上；判定真伪后修正"
fi

# ── 总控退出码：必须由 main 决定，不许被尾部软门禁吃掉（2026-09-17 实测修）──────────────
# 现场：`--scope tags` 在 GOFLAGS 污染下报「⇒ 门禁红灯（rc=1）」，而**整脚本退出码是 0**
# —— 尾部那条软门禁的 rc 成了脚本的 rc，调用方（人、脚本、CI）一律读到绿。
# 这类「报红却退 0」正是本脚本开头点名的那类假绿，故在此显式出口。
exit "${MAIN_RC}"
