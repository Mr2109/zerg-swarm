#!/usr/bin/env bash
# scripts/replay-fidelity.sh —— T4.4 保真度三指标 + 二分定位（E7）的**可执行验收**，
# 顺带打印 T4.5（非确定性源审计表）与 T4.6（trace 版本与失效）的取证。
#
# 为什么要有这个脚本（而不是只跑 go test）
# ----------------------------------------
# ① 判据要**跨实现**复核：Go 侧算出的产物 sha256，这里用 shasum 独立再算一遍。
#    两边都相等才叫"产物等价"；只用一侧的自述，等于让被测者自己给自己打分。
# ② 门禁自己必须先能被证明"会红"（负控，见设计稿 A8）：本脚本先对合成目录做自检，
#    自检不过 ⇒ exit 2，**拒绝出结论**（一个永远绿的比对器比没有比对器更坏）。
# ③ 输出给人看的是三行结论 + 四段证据，不是一堆 PASS 字样。
#
# 判据（每条都从结果文件数出来，不在子壳里累加计数器）
# ------------------------------------------------
#   ① F = 命中 / 尝试，missed > 0 即 FAIL；F 必须 = 1.0
#   ② 录制运行与回放运行的产物 sha256 相等（Go 侧摘要 + shell 侧独立摘要，两侧都要相等）
#   ③ replay(replay(x)) == replay(x)：独立两趟 + 叠加那趟，三份摘要全等；且夹具目录只读
#   ④ 二分定位：变异体与基准的最早分歧 seq = 4（夹具写死的期望值），前缀 4 条
#   ⑤ 审计：十类逐项声明，红项 0（自审整包）
#   ⑥ 作废判定：7 种指纹变化全部作废且判别词带两边指纹；"指纹相同 ⇒ 不作废"必须成立
#
# 用法
# ----
#   bash scripts/replay-fidelity.sh                 # 跑全部（夹具每次重建）
#   bash scripts/replay-fidelity.sh --fixture DIR   # 指定夹具目录（默认 $TMPDIR/zerg-replay-fidelity）
#   bash scripts/replay-fidelity.sh --selftest      # 只跑负控自检
#   bash scripts/replay-fidelity.sh --keep          # 保留中间件（默认就保留：证据要能被复核）
#
# 退出码：0 全过 · 1 有失败项 · 2 用法错/前置缺件/自检不过（不给结论）
set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORE_DIR="${REPO_ROOT}/core"
FIX="${TMPDIR:-/tmp}/zerg-replay-fidelity"
SELFTEST_ONLY=0

while [ $# -gt 0 ]; do
    case "$1" in
        --fixture)
            if [ $# -lt 2 ]; then echo "✗ --fixture 需要一个目录参数" >&2; exit 2; fi
            FIX="$2"; shift 2 ;;
        --selftest) SELFTEST_ONLY=1; shift ;;
        --keep) shift ;;
        -h|--help)
            sed -n '2,30p' "${BASH_SOURCE[0]}"; exit 0 ;;
        *) echo "✗ 未知参数：$1" >&2; exit 2 ;;
    esac
done

if [ ! -d "${CORE_DIR}" ]; then
    echo "✗ 找不到 core/（脚本按仓库根定位，实际在 ${REPO_ROOT}）" >&2
    exit 2
fi
if ! command -v python3 >/dev/null 2>&1; then
    echo "✗ 缺 python3（本脚本用它读 JSON 判据，不用输出字样判）" >&2
    exit 2
fi

rm -rf "${FIX}"
mkdir -p "${FIX}"
RESULTS="${FIX}/results.tsv"
: > "${RESULTS}"
LOG="${FIX}/00-go-test.log"

# ── 判定工具（判定一律立刻落盘：末了只数 results.tsv，避免子壳计数器假绿）────────
check() { # check <项> <0=通过 其余=失败> <说明>
    if [ "$2" = "0" ]; then
        printf 'PASS\t%s\t%s\n' "$1" "$3" >> "${RESULTS}"
        printf '  ✓ %-38s %s\n' "$1" "$3"
    else
        printf 'FAIL\t%s\t%s\n' "$1" "$3" >> "${RESULTS}"
        printf '  ✗ %-38s %s\n' "$1" "$3"
    fi
}

# dir_digest —— **独立**复算（与 Go 侧口径不同：这里对 `rel 路径 + 内容 sha256` 的文本流再取 sha256）
dir_digest() {
    local d="$1" n
    if [ ! -d "$d" ]; then printf 'MISSING'; return 0; fi
    n="$(find "$d" -type f | wc -l | tr -d ' ')"
    if [ "$n" = "0" ]; then printf 'EMPTY'; return 0; fi
    ( cd "$d" && find . -type f | LC_ALL=C sort | xargs shasum -a 256 | shasum -a 256 | awk '{print $1}' )
}

# json_get <文件> <点路径> —— 读一个字段（判据来自结构，不来自输出字样）
json_get() {
    python3 - "$1" "$2" <<'PY'
import json, sys
cur = json.load(open(sys.argv[1], encoding="utf-8"))
for k in sys.argv[2].split("."):
    cur = cur[int(k)] if isinstance(cur, list) else cur[k]
if isinstance(cur, bool):
    print("true" if cur else "false")
else:
    print(cur)
PY
}

# json_len <文件> <点路径> —— 取列表长度（判据来自结构，不来自输出字样）
json_len() {
    python3 - "$1" "$2" <<'PY'
import json, sys
cur = json.load(open(sys.argv[1], encoding="utf-8"))
for k in sys.argv[2].split("."):
    cur = cur[int(k)] if isinstance(cur, list) else cur[k]
print(len(cur))
PY
}

# 负控自检：比对器必须先能红
selftest() {
    local d="${FIX}/selftest" a b rc=0
    rm -rf "$d"; mkdir -p "$d/x" "$d/y"
    printf 'a\n' > "$d/x/f.txt"
    printf 'a\n' > "$d/y/f.txt"
    a="$(dir_digest "$d/x")"; b="$(dir_digest "$d/y")"
    if [ "$a" = "$b" ] && [ "$a" != "EMPTY" ]; then
        printf '  ✓ %-38s %s\n' "自检：同内容目录判相等" "摘要 ${a}"
    else
        printf '  ✗ %-38s %s\n' "自检：同内容目录判相等" "摘要 ${a} vs ${b}"; rc=1
    fi
    printf 'b\n' > "$d/y/f.txt"
    a="$(dir_digest "$d/x")"; b="$(dir_digest "$d/y")"
    if [ "$a" != "$b" ]; then
        printf '  ✓ %-38s %s\n' "自检：差一字节判不等" "摘要已变"
    else
        printf '  ✗ %-38s %s\n' "自检：差一字节判不等" "仍相等 ⇒ 比对器是假绿"; rc=1
    fi
    a="$(dir_digest "$d/x/nope")"; b="$(dir_digest "$d/x")"
    if [ "$a" != "$b" ]; then
        printf '  ✓ %-38s %s\n' "自检：缺目录不算相等" "缺目录得 ${a}"
    else
        printf '  ✗ %-38s %s\n' "自检：缺目录不算相等" "被当成了相等"; rc=1
    fi
    return $rc
}

echo "══ T4.4 保真度三指标（E7）· 夹具 ${FIX}"
echo "── 负控自检（比对器先要能被证明会红）"
if ! selftest; then
    echo "✗ 自检未过 ⇒ 脚本本身有问题，拒绝出结论" >&2
    exit 2
fi
check "负控自检" 0 "三条自检全过（同内容判等 / 差一字节判不等 / 缺目录不算相等）"

if [ "${SELFTEST_ONLY}" = "1" ]; then
    echo "（--selftest：只跑自检，到此为止）"
    exit 0
fi

echo "── 跑真夹具（Go 侧：录制 + 两趟回放 + 叠加回放 + 二分 + 审计 + 作废判定）"
( cd "${CORE_DIR}" && GOFLAGS=-mod=mod GOSUMDB=off GOPROXY=https://goproxy.cn,direct \
    ZERG_REPLAY_FIDELITY_DIR="${FIX}" go test ./internal/replay/ -count=1 -v ) > "${LOG}" 2>&1
rc=$?
check "go test ./internal/replay/" "${rc}" "$(grep -c '^--- PASS' "${LOG}" | tr -d ' ') 条用例通过（日志 ${LOG}）"
if [ "${rc}" != "0" ]; then
    echo "  ── 失败用例（节选）──"
    grep -E '^(--- FAIL|    )' "${LOG}" | head -20
fi

# 缺件即报错（不给结论）—— 报告不存在说明 Go 侧没能跑到写报告那一步
for f in fidelity.json audit.json invalidation.json; do
    if [ ! -f "${FIX}/${f}" ]; then echo "✗ 缺报告 ${FIX}/${f}（Go 侧没跑到出报告那一步）⇒ 不给结论" >&2; exit 2; fi
done

echo "── ① F = 命中录播事件 / 回放期尝试事件（missed>0 即 FAIL）"
ATT="$(json_get "${FIX}/fidelity.json" fidelity.attempted)"
HIT="$(json_get "${FIX}/fidelity.json" fidelity.hit)"
MISS="$(json_get "${FIX}/fidelity.json" fidelity.missed)"
FVAL="$(json_get "${FIX}/fidelity.json" fidelity.f)"
[ "${MISS}" = "0" ] && [ "${ATT}" = "${HIT}" ] && [ "${ATT}" -gt 0 ]; check "F=1.0 且 missed=0" $? "F=${FVAL}（命中 ${HIT} / 尝试 ${ATT}，未命中 ${MISS}）"
[ "${MISS}" = "0" ]; check "missed=0（未命中不是警告，是 FAIL）" $? "missed=${MISS}"

echo "── ② 输出等价性：录制产物 == 回放产物（Go 侧摘要 + shell 侧独立摘要）"
RD="$(json_get "${FIX}/fidelity.json" recorded_digest)"
AD="$(json_get "${FIX}/fidelity.json" replay_a_digest)"
EQ="$(json_get "${FIX}/fidelity.json" equivalence.equal)"
[ "${EQ}" = "true" ] && [ "${RD}" = "${AD}" ]; check "Go 侧产物摘要相等" $? "recorded == replay-a = ${RD#sha256:}"
SHR="$(dir_digest "${FIX}/recorded")"
SHA="$(dir_digest "${FIX}/replay-a")"
[ "${SHR}" = "${SHA}" ] && [ "${SHR}" != "EMPTY" ] && [ "${SHR}" != "MISSING" ]; check "shell 侧独立摘要相等" $? "recorded=${SHR:0:16} replay-a=${SHA:0:16}（与 Go 侧口径刻意不同）"
# 负控（用**真夹具数据**，不是合成目录）：把回放产物改一字节 ⇒ 同一个比对器必须判不等
NEG="${FIX}/negctl-replay-a"
rm -rf "${NEG}"; cp -R "${FIX}/replay-a" "${NEG}"
printf 'X' >> "${NEG}/answer.txt"
NEGD="$(dir_digest "${NEG}")"
[ "${NEGD}" != "${SHA}" ]; check "负控：真夹具产物改一字节 ⇒ 判不等" $? "改过的摘要 ${NEGD:0:16} ≠ 原摘要 ${SHA:0:16}"

echo "── ③ 确定性自检：replay(replay(x)) == replay(x)"
DET="$(json_get "${FIX}/fidelity.json" determinism.layer_stable)"
STB="$(json_get "${FIX}/fidelity.json" determinism.stable)"
UNT="$(json_get "${FIX}/fidelity.json" determinism.fixture_untouched)"
[ "${STB}" = "true" ] && [ "${DET}" = "true" ] && [ "${UNT}" = "true" ]; check "两趟独立 + 叠加 + 夹具只读" $? "stable=${STB} layer_stable=${DET} fixture_untouched=${UNT}"
SHB="$(dir_digest "${FIX}/replay-b")"
[ "${SHA}" = "${SHB}" ] && [ "${SHB}" != "EMPTY" ]; check "shell 侧 replay-a == replay-b" $? "replay-a=${SHA:0:16} replay-b=${SHB:0:16}"
LCALL="$(json_get "${FIX}/fidelity.json" live_calls)"
[ "${LCALL}" = "0" ]; check "回放期真发次数 = 0" $? "真发 ${LCALL} 次（回放不真发是结构性的，这里钉住它）"

echo "── ④ 二分定位最早分歧（夹具变异体：第 4 条录播的结果被改）"
ESEQ="$(json_get "${FIX}/fidelity.json" bisect.earliest_seq)"
PLEN="$(json_get "${FIX}/fidelity.json" bisect.prefix_len)"
PROBES="$(json_get "${FIX}/fidelity.json" bisect.probes)"
[ "${ESEQ}" = "4" ] && [ "${PLEN}" = "4" ]; check "最早分歧 seq=4 / 前缀 4 条" $? "探测 ${PROBES} 次（夹具 6 条；n 大时二分才省）"

echo "── ⑤ T4.5 非确定性源审计（十类逐项声明，红项必须为 0）"
RED="$(json_get "${FIX}/audit.json" red_count)"
ITEMS="$(json_len "${FIX}/audit.json" items)"
[ "${RED}" = "0" ]; check "整包自审红项 = 0" $? "红项 ${RED}"
[ "${ITEMS}" -ge 10 ]; check "十类逐项在表" $? "声明项 ${ITEMS} 类"
python3 - "${FIX}/audit.json" <<'PY'
import json, sys
rep = json.load(open(sys.argv[1], encoding="utf-8"))
bad = [r for r in rep["items"] if r["red"]]
print("  类别表（%d 类，红 %d）：" % (len(rep["items"]), len(bad)))
for i, r in enumerate(rep["items"], 1):
    print("    %2d. %-10s %-6s 命中=%d(豁免%d/计数%d) 声明=%d %s" % (
        i, r["id"], r["status"], r["hits"], r["allowed"], r["counted"], r["expect_hits"],
        "✗" if r["red"] else "✓"))
if rep.get("stale_allow"):
    print("    过期豁免：%d 条" % len(rep["stale_allow"]))
PY
check "审计表打印" 0 "已打印十类（脚本判据只看 red_count，表是给人看的）"

echo "── ⑥ T4.6 trace 版本与失效（指纹变 ⇒ 作废，附两边指纹；指纹同 ⇒ 不作废）"
ALLI="$(json_get "${FIX}/invalidation.json" all_invalidated)"
python3 - "${FIX}/invalidation.json" <<'PY'
import json, sys
ev = json.load(open(sys.argv[1], encoding="utf-8"))
print("  trace %s 头部 %s" % (ev["trace"], json.dumps(ev["header"], ensure_ascii=False)))
for c in ev["cases"]:
    print("    %-22s 作废=%-5s 两边指纹齐全=%-5s" % (c["name"], c["invalidated"], c["has_both_sides"]))
PY
[ "${ALLI}" = "true" ]; check "7 种指纹变化全部作废且带两边指纹" $? "all_invalidated=${ALLI}"
CTRL="$(json_get "${FIX}/invalidation.json" cases.0.invalidated)"
[ "${CTRL}" = "false" ]; check "反例：指纹相同不作废（防误报）" $? "对侧判定 invalidated=${CTRL}"

echo "── 结论"
FAILS="$(grep -c '^FAIL' "${RESULTS}" || true)"
PASSES="$(grep -c '^PASS' "${RESULTS}" || true)"
echo "  判定通过 ${PASSES} 项 / 失败 ${FAILS} 项（明细 ${RESULTS}）"
if [ "${FAILS}" != "0" ]; then
    echo "  ✗ 三条指标未全过（exit 1）—— 逐条看上面对应的 ✗ 行"
    exit 1
fi
echo "  ✓ 三条指标全过：F=1.0 且 missed=0 · 录制与回放产物摘要相等（两侧独立复算）· replay(replay(x))==replay(x)"
echo "  （夹具与证据留档：${FIX}）"
exit 0
