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
#     bash scripts/precommit-gates.sh                  # 默认跑全部（go + rust + pub + tags）
#     bash scripts/precommit-gates.sh --scope go       # 只跑 Go 侧（可重复：--scope go --scope rust）
#     bash scripts/precommit-gates.sh --scope tags     # 只跑双构建工程门禁（T6.3）
#     bash scripts/precommit-gates.sh --scope docs     # 只跑文档面门禁（meta/name/freshness D1–D3）
#     bash scripts/precommit-gates.sh --outdir /tmp/gates-14   # 指定日志目录
#     bash scripts/precommit-gates.sh --list           # 只看步骤清单，不跑
#     bash scripts/precommit-gates.sh --self-test      # 只跑自检（合成步骤，不碰真目标）
#     bash scripts/precommit-gates.sh --emit-cmd 无后缀  # 只打印匹配步骤的命令串（负控/复核用；不跑）
#
# 退出码：0 全绿 · 1 有失败项 · 2 **不给结论**（用法错/前置缺件/自检不过/**有步骤报 BLOCKED**）
#
# 三档（步骤级）：0=PASS · 1=FAIL（失败项）· 2=BLOCKED（**不给结论**）。
#   ★ BLOCKED **不计入失败项数** —— 「没结论」不是「错」，两者各有各的计数与打印位；
#     但 BLOCKED 也不许当绿（有 BLOCKED 且无 FAIL ⇒ 整脚本 rc=2，不是 rc=0）。
#   用三档判定的步骤，模式写 `tri`（现有 rc / empty 两模式的语义**一字未改**）。
#
# scope 说明（2026-09-17 加 tags · 2026-09-18 加 docs）：
#   go   = gofmt/build/vet/test（**单侧**：默认 tag 配置）
#   rust = wall 的 fmt/clippy/test
#   pub  = 公开面两侧都有的脚本静态检查
#   tags = 双构建工程门禁（脚本自带正反用例自检；它自己会在两种 tag 配置下成对跑 build/vet）
#   docs = 文档面只读门禁：check-doc-meta(--scope formal --missing=fail) · check-doc-name(--scope repo)
#          · check-doc-freshness 的 D1/D2/D3（D4 生成式 drift 归发布面，不在此）。
#          它们是**步骤表里的一等步骤**（不是本脚本尾部那种软检查位），rc 一律取真退出码、不接管道。
#          docs **不在默认 scope 集**里（默认仍是 go+rust+pub+tags）：是否纳入默认 / 各门是否阻断另行拍板。
#
# 自检不通过 ⇒ 拒绝跑真目标（项目口径：门禁自己先能被证明「会红」）。

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── 步骤表（indexed arrays，bash 3.2 可用）──────────────────────────
STEP_SCOPE=()
STEP_NAME=()
STEP_MODE=()   # rc    = 退出码为 0 即通过（**非 0 一律算失败**，含 2 —— 现有 20 步语义不变）
               # empty = 退出码为 0 且输出为空才通过
               # tri   = 三档：0=PASS · 1=FAIL · 2=BLOCKED（不给结论，**不计入失败项数**）
               #         非 0/1/2 的**异常码**（3/127/…）一律按 FAIL —— 不许把「跑不起来」当「没结论」
               #         ★ 只有自己申报 tri 的步骤才走三档；rc/empty 两模式未被这一档改动（2026-09-18）
               #         ★ 模式名不在 {rc,empty,tri} 内 ⇒ 该步硬红（拼错模式名不许静默按 rc 处理）
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

# ── 三档判定：**唯一的判据出口**（只看真退出码；rc 模式的语义与以前一字不差）──────
#   返回 PASS / FAIL / BLOCKED 三值之一。判据独立成函数，是为了让自检能直接钉住三格。
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

# ── 失败项数 / 不给结论数：**只从结果表数出来**（纯读文件，不在任何地方累加）──────
#    ★ 两个数是**分开的**：BLOCKED（没结论）不许计进失败项数，也不许被失败项数吞掉。
count_fail() {  # count_fail <结果表>
  grep -c '^FAIL' "$1" 2>/dev/null || true
}

count_blocked() {  # count_blocked <结果表> —— 第三档：rc=2「不给结论」的步数
  grep -c '^BLOCKED' "$1" 2>/dev/null || true
}

count_pass() {  # count_pass <结果表>
  grep -c '^PASS' "$1" 2>/dev/null || true
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
    fi
  done <"${results}"
  printf '── 状态计数：PASS %s · FAIL %s · BLOCKED(不给结论) %s ──\n' \
    "$(count_pass "${results}")" "$(count_fail "${results}")" "$(count_blocked "${results}")"
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
        ;;
      rust)
        add_step rust "cargo fmt --check"              rc "${REPO_ROOT}/wall" "cargo fmt --check"
        add_step rust "cargo clippy -- -D warnings"    rc "${REPO_ROOT}/wall" "cargo clippy -- -D warnings"
        add_step rust "cargo test"                     rc "${REPO_ROOT}/wall" "cargo test"
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
        ;;
      tags)
        # 双构建工程门禁（任务表 T6.3 / 设计稿 §〇 A3–A6）：tag 命名 · 两个构建都过 · vet 成对跑 ·
        # 导出面奇偶校验 · GOFLAGS 断言。**单独一个 scope**：它比 go scope 里的单侧检查慢
        # （每个模块两套配置），且口径不同（它守的是「两个构建配置等价」），不该混进现有 scope 语义。
        # --scope go 跳不到它，要单跑：bash scripts/precommit-gates.sh --scope tags
        add_step tags "双构建工程门禁（tag/构建/vet/导出面/GOFLAGS）" rc "${REPO_ROOT}" "python3 scripts/check-build-tags.py"
        ;;
      docs)
        # 文档面只读门禁（2026-09-18 路 D 挂接 · 模式一律 `tri` 三档 0/1/2）──────────────
        # 位置：**在 run_suite 的步骤表里**（一等步骤），不是脚本尾部那个软检查位 ——
        #   尾部软门禁的 rc 会被最后一句 `exit "${MAIN_RC}"` 之外的东西吃掉（本脚本 2026-09-17
        #   实测过「报红却退 0」），所以新门禁一律走步骤表，rc 由 _judge 按真退出码判。
        # 判定：**不接管道**（不写 `| tee` / `| tail`），rc 直接取进程退出码。
        # 三档落点：这三只门脚本的退码口径本身就是 0/1/2（2 = 不给结论/空转），
        #   所以用 `tri` ⇒ rc=2 记 BLOCKED、**不计入失败项数**；rc>=3（跑不起来）仍按红算。
        # D4（生成式参考 drift）**不挂**：它是发布面的事（重建 + 逐字节比对），不是提交闸。
        # 每只门脚本自带 --self-test，本 scope **不传 --no-self-test**：先自证「会红」再扫真目标。
        add_step docs "docs: meta --scope formal --missing=fail" tri "${REPO_ROOT}" "python3 scripts/check-doc-meta.py --scope formal --missing=fail"
        add_step docs "docs: name --scope repo"                  tri "${REPO_ROOT}" "python3 scripts/check-doc-name.py --scope repo"
        add_step docs "docs: freshness D1 引用路径存在"           tri "${REPO_ROOT}" "python3 scripts/check-doc-freshness.py d1"
        add_step docs "docs: freshness D2 引用 文件:行 有效"       tri "${REPO_ROOT}" "python3 scripts/check-doc-freshness.py d2"
        add_step docs "docs: freshness D3 断链断锚"               tri "${REPO_ROOT}" "python3 scripts/check-doc-freshness.py d3"
        ;;
      *)
        printf '✗ 未知 scope: %s（可用: go / rust / pub / tags / docs）\n' "${s}" >&2
        return 2
        ;;
    esac
  done
  return 0
}

# ── 前置检查：必须真在 Zerg 仓里，且三棵构建树的入口都在 ────────────
precheck() {
  local miss=0 p
  for p in core/go.mod agent/go.mod wall/Cargo.toml ui/Cargo.toml publish/whitelist.txt; do
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
      -h|--help) sed -n '19,48p' "${BASH_SOURCE[0]}"; return 0 ;;
      *) printf '✗ 未知参数: %s\n' "$1" >&2; return 2 ;;
    esac
  done

  if [ "${#scopes[@]}" -eq 0 ]; then
    # 默认 scope 集**保持原样**（go rust pub tags）—— 现有 20 步的名字与语义、默认覆盖面一字未改。
    # ★ docs **暂不进默认集**（2026-09-18 待拍）：它今天 D1/D2 有红、D3 报 BLOCKED，直接并进默认
    #   会让每一次提交都变红/变「没结论」。要不要进默认、各门谁阻断 —— 见 scripts/precommit-gates.md
    #   §docs scope 的「待拍」一句。进默认 = 这一行改成 (go rust pub tags docs)。
    scopes=(go rust pub tags)
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

  local nfail nblock npass
  nfail="$(count_fail "${results}")"
  nblock="$(count_blocked "${results}")"
  npass="$(count_pass "${results}")"
  printf '\n步骤总数: %d · 通过: %s · 失败项数: %s · 不给结论(BLOCKED): %s\n' \
    "${#STEP_NAME[@]}" "${npass}" "${nfail}" "${nblock}"
  if [ "${nfail}" -ne 0 ]; then
    printf '⇒ 门禁红灯（rc=1）：逐条看上面的日志路径重跑\n'
    return 1
  fi
  if [ "${nblock}" -ne 0 ]; then
    printf '⇒ 无失败项，但有 %s 步「不给结论」（BLOCKED · rc=2）：**不许当绿**、也不按「错」计\n' "${nblock}"
    return 2
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
