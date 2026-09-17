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
#     bash scripts/precommit-gates.sh --outdir /tmp/gates-14   # 指定日志目录
#     bash scripts/precommit-gates.sh --list           # 只看步骤清单，不跑
#     bash scripts/precommit-gates.sh --self-test      # 只跑自检（合成步骤，不碰真目标）
#
# 退出码：0 全绿 · 1 有失败项 · 2 用法错/前置缺件/自检不过（**不给结论**）
#
# scope 说明（2026-09-17 加 tags）：
#   go   = gofmt/build/vet/test（**单侧**：默认 tag 配置）
#   rust = wall 的 fmt/clippy/test
#   pub  = 公开面两侧都有的脚本静态检查
#   tags = 双构建工程门禁（脚本自带正反用例自检；它自己会在两种 tag 配置下成对跑 build/vet）
#
# 自检不通过 ⇒ 拒绝跑真目标（项目口径：门禁自己先能被证明「会红」）。

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── 步骤表（indexed arrays，bash 3.2 可用）──────────────────────────
STEP_SCOPE=()
STEP_NAME=()
STEP_MODE=()   # rc    = 退出码为 0 即通过
               # empty = 退出码为 0 且输出为空才通过
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
  status="PASS"
  [ "${rc}" -eq 0 ] || status="FAIL"
  if [ "${mode}" = "empty" ] && [ "${rc}" -eq 0 ] && [ -s "${log}" ]; then
    status="FAIL"   # 判据是「输出必须为空」，非空 ⇒ 红
  fi
  printf '%s\t%s\t%s\t%s\t%s\n' "${status}" "${name}" "${rc}" "$((t1 - t0))s" "${log}" >>"${results}"
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

# ── 失败项数：**只从结果表数出来**（纯读文件，不在任何地方累加）──────
count_fail() {  # count_fail <结果表>
  grep -c '^FAIL' "$1" 2>/dev/null || true
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
  printf '%-5s %-50s %-5s %s\n' "状态" "步骤" "rc" "耗时"
  while IFS=$'\t' read -r status name rc secs log; do
    [ -n "${status}" ] || continue
    printf '%-5s %-50s %-5s %s\n' "${status}" "${name}" "${rc}" "${secs}"
    if [ "${status}" = "FAIL" ]; then
      printf '      ↳ 日志: %s\n' "${log}"
      tail -12 "${log}" | sed 's/^/      | /'
    fi
  done <"${results}"
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

  printf '自检结论: %s（断言失败 %d 条）\n' "$([ "${SELF_BAD}" -eq 0 ] && echo 全过 || echo 不过)" "${SELF_BAD}"
  rm -rf "${t}"
  if [ "${SELF_BAD}" -eq 0 ]; then
    return 0
  fi
  return 2
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
        add_step pub "check-shell-unicode-vars.py --check"         rc "${REPO_ROOT}" "python3 scripts/check-shell-unicode-vars.py --check scripts/*.sh"
        ;;
      tags)
        # 双构建工程门禁（任务表 T6.3 / 设计稿 §〇 A3–A6）：tag 命名 · 两个构建都过 · vet 成对跑 ·
        # 导出面奇偶校验 · GOFLAGS 断言。**单独一个 scope**：它比 go scope 里的单侧检查慢
        # （每个模块两套配置），且口径不同（它守的是「两个构建配置等价」），不该混进现有 scope 语义。
        # --scope go 跳不到它，要单跑：bash scripts/precommit-gates.sh --scope tags
        add_step tags "双构建工程门禁（tag/构建/vet/导出面/GOFLAGS）" rc "${REPO_ROOT}" "python3 scripts/check-build-tags.py"
        ;;
      *)
        printf '✗ 未知 scope: %s（可用: go / rust / pub / tags）\n' "${s}" >&2
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
  local scopes=() outdir="" list_only=0 self_only=0 t
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --scope) scopes+=("$2"); shift 2 ;;
      --outdir) outdir="$2"; shift 2 ;;
      --list) list_only=1; shift ;;
      --self-test) self_only=1; shift ;;
      -h|--help) sed -n '19,36p' "${BASH_SOURCE[0]}"; return 0 ;;
      *) printf '✗ 未知参数: %s\n' "$1" >&2; return 2 ;;
    esac
  done

  if [ "${#scopes[@]}" -eq 0 ]; then
    scopes=(go rust pub tags)
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

  local nfail
  nfail="$(count_fail "${results}")"
  printf '\n步骤总数: %d · 失败项数: %s\n' "${#STEP_NAME[@]}" "${nfail}"
  if [ "${nfail}" -ne 0 ]; then
    printf '⇒ 门禁红灯（rc=1）：逐条看上面的日志路径重跑\n'
    return 1
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
