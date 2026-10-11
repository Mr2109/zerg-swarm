// cli_gate_run_step_gapfix_test.go —— 本片两条缺口的**真二进制**判据（全部走**合成门禁脚本**，
// 不碰真门禁、不碰真目标）：
//
//	① `GAP-20261001-35`：`gate run --step <步名>` **不带 `--only-step`** ⇒ stdout 首行必须是
//	   逐字声明「以下为合成自检输出，不是你要跑的那一步」（措辞取账内原文）；带了 `--only-step`
//	   （跳过整套合成自检）**不许**打这行。
//	② `GAP-20260927-286`：`gate run --step <步名> --json per_rule,examples` 从**那一步自己的日志
//	   正文**解析出 `per_rule`（规则号 ⇒ 命中数）与 `examples`（规则号 ⇒ 样例行）；日志里没有这
//	   一形态 ⇒ 两格写「（读不到…）」哨兵（**不许编数**）。
//
// ★ 为什么两格都走 `zergBinaryFromSource`（从源码现编到临时目录）：本单**禁重编 `bin/zerg`** ⇒
// 走旧制品会把「本面未落制品」误读成「本面不存在」（同 `TestGateRunStep_MachineFaceHidesVarDir` 的口径）。
//
// ★ 哑化负控（判据有牙的证据）：两格各挂一枚**现成口子** ——
//
//	`ZERG_GATE_STEP_NONOTICE`  非空 ⇒ ① 那行声明不打 ⇒ ① 的「首行必须出现」断言**真红**；
//	`ZERG_GATE_STEP_NOPERRULE` 非空 ⇒ ② 跳过解析 ⇒ ② 的「per_rule.N2 == 8」断言**真红**。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── ① 的夹具：不带 `--only-step` 时「先自检、后真跑」的混流（首行是自检段）──────
const g2SelfTestStep = `if [ "${1:-}" = "--list" ]; then
  echo "  gates tri    合成步"
  echo "共 1 步"
  exit 0
fi
echo "── 合成自检段 第 1 行（本行以下全是自检，不是你要跑的那一步）──"
echo "  SYNTH-SELFTEST glossary 脚本名示例"
echo "── 单步档（--step）：合成步 ──"
echo "SYNTH-STEP-RAN"
exit 0
`

// ── ② 的夹具 A：日志里带逐规则/样例段（行形逐字取 check-doc-name / check-doc-meta 方言）──
const g2StepLogPerRule = `case "$*" in *--json*) echo "合成脚本收到了 --json（命令面的旗标漏下来了）"; exit 2 ;; esac
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outdir) out="$2"; shift 2 ;;
    --step) shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "${out}"
log="${out}/00-合成步.log"
printf '%s\n' \
  'check-doc-name —— 合成夹具（本件不碰真仓）' \
  '  不合规 51（冻结区 0）· 告警 6（冻结区 0）' \
  '  N2 [FAIL] ②空格/全角：名称含 U+0020 —— 命中 8（可改面 8 · 冻结区 0）' \
  '      · 设计-夹具A.md → 名称含 U+0020' \
  '      · …另 3 条（--max-examples 调整）' \
  '  N3 [FAIL] ③日期后缀：词干含 -YYYYMMDD —— 命中 43（可改面 43 · 冻结区 0）' \
  '      · 事故-夹具B-20261004.md → 词干含日期后缀' \
  'FAIL：51 个不合规命中（只报告不改）' > "${log}"
printf 'FAIL\t合成步\t1\t0s\t%s\n' "${log}" > "${out}/results.tsv"
exit 1
`

// ── ② 的夹具 B：日志里**没有**逐规则/样例段（走哨兵那一档）──
const g2StepLogPlain = `out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outdir) out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "${out}"
log="${out}/00-合成步.log"
printf '%s\n' '别的门（没有逐规则段）' 'OK：合规 3 篇' > "${log}"
printf 'PASS\t合成步\t0\t0s\t%s\n' "${log}" > "${out}/results.tsv"
exit 0
`

// g2NoticeText —— ① 的**逐字**声明（缺口账 `GAP-20261001-35` 的 want 原文）。
const g2NoticeText = "以下为合成自检输出，不是你要跑的那一步"

// g2JSONRow —— 取单步档包封里唯一那一行。
func g2JSONRow(t *testing.T, out string) map[string]string {
	t.Helper()
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	if len(env.Items) != 1 {
		t.Fatalf("要一行（这一步的判决），得到 %d 行：%s", len(env.Items), out)
	}
	return env.Items[0]
}

// ① 不带 `--only-step` ⇒ stdout 首行 == 逐字声明；带 `--only-step` ⇒ 不打这行。
func TestGateRunStep_NoticeHeadForMixedSelfTest(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := stepRepo(t, g2SelfTestStep)

	rc, out, errb := execCase(t, bin, root, "gate", "run", "--step", "合成步")
	if rc != 0 {
		t.Fatalf("合成脚本退 0 ⇒ 命令面退 0，得到 %d · stderr=%s", rc, errb)
	}
	first, _, _ := strings.Cut(out, "\n")
	if !strings.HasPrefix(first, g2NoticeText) {
		t.Errorf("缺口 GAP-20261001-35：单步档**不带 `--only-step`** ⇒ stdout **首行**必须是逐字声明 %q；\n  首行 = %q\n  全量 = %q", g2NoticeText, first, out)
	}
	// 它同时是「自检段」的**可机检分隔标记**：整段 stdout 里恰好出现一次，且在首行。
	if n := strings.Count(out, g2NoticeText); n != 1 {
		t.Errorf("声明应恰好出现一次（它是自检段的分隔标记），得到 %d 次：%q", n, out)
	}
	// 声明不许把真那一步的输出吞掉（本夹具的真跑行仍在）。
	if !strings.Contains(out, "SYNTH-STEP-RAN") {
		t.Errorf("补声明不许吞掉脚本的真跑输出：%q", out)
	}
	// 声明必须在自检段之前（首行）—— 不许排到后面去。
	if strings.Index(out, g2NoticeText) > strings.Index(out, "SYNTH-SELFTEST") {
		t.Errorf("声明必须在自检段**之前**（它是首行）：%q", out)
	}

	// 排除面：带 `--only-step`（跳过整套合成自检）⇒ 不产合成段 ⇒ **不许**打这行。
	rc2, out2, errb2 := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--only-step")
	if rc2 != 0 {
		t.Fatalf("带 --only-step 的合成脚本退 0 ⇒ 命令面退 0，得到 %d · stderr=%s", rc2, errb2)
	}
	if strings.Contains(out2, g2NoticeText) {
		t.Errorf("带 `--only-step`（跳过整套合成自检）⇒ 声明失真 ⇒ 不许打这行：%q", out2)
	}
}

// ② `--json per_rule,examples` 从日志正文解析出两格；无该形态 ⇒ 哨兵；字段面同批登记。
func TestGateRunStep_PerRuleAndExamplesFields(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := stepRepo(t, g2StepLogPerRule)
	od := filepath.Join(t.TempDir(), "gate-out")

	rc, out, errb := execCase(t, bin, root, "gate", "run", "--step", "合成步",
		"--outdir", od, "--json", "step,verdict,rc,per_rule,examples")
	if rc != 1 {
		t.Fatalf("合成脚本退 1 ⇒ 命令面退 1（只转发、不翻译），得到 %d · stderr=%s", rc, errb)
	}
	row := g2JSONRow(t, out)

	// per_rule：值格是**字符串**（信封的值格都是字符串）⇒ 内容是逐字 JSON 对象串。
	perRule := map[string]int{}
	if err := json.Unmarshal([]byte(row["per_rule"]), &perRule); err != nil {
		t.Fatalf("缺口 GAP-20260927-286：`per_rule` 必须是 JSON 对象串（规则号 ⇒ 命中数）：%q · %v", row["per_rule"], err)
	}
	if perRule["N2"] != 8 || perRule["N3"] != 43 {
		t.Errorf("`per_rule` 要逐格取日志里的「命中 N」：得 %v（期望 N2=8 · N3=43）", perRule)
	}
	if len(perRule) != 2 {
		t.Errorf("`per_rule` 只许两条规则（本夹具）：%v", perRule)
	}
	// examples：规则号 ⇒ 样例行（前缀 `· ` 已剥；截断标记 `…另 N 条` 不算一条样例）。
	examples := map[string][]string{}
	if err := json.Unmarshal([]byte(row["examples"]), &examples); err != nil {
		t.Fatalf("`examples` 必须是 JSON 对象串（规则号 ⇒ 样例行）：%q · %v", row["examples"], err)
	}
	if got := examples["N2"]; len(got) != 1 || got[0] != "设计-夹具A.md → 名称含 U+0020" {
		t.Errorf("`examples.N2` 要**逐字**取那一条样例（截断标记不算）：%q", got)
	}
	if got := examples["N3"]; len(got) != 1 || !strings.Contains(got[0], "事故-夹具B-20261004.md") {
		t.Errorf("`examples.N3` 要取到那一条样例：%q", got)
	}
	for _, lines := range examples {
		for _, ln := range lines {
			if strings.HasPrefix(ln, "…另 ") || strings.HasPrefix(ln, "· ") {
				t.Errorf("样例不许带前缀、也不许把截断标记当样例：%q", ln)
			}
		}
	}

	// 哨兵档：日志里没有逐规则/样例段 ⇒ 两格写「（读不到…）」（**不许编数**）。
	rootPlain := stepRepo(t, g2StepLogPlain)
	rc2, out2, _ := execCase(t, bin, rootPlain, "gate", "run", "--step", "合成步",
		"--outdir", filepath.Join(t.TempDir(), "gate-out2"), "--json", "per_rule,examples")
	if rc2 != 0 {
		t.Fatalf("合成脚本退 0 ⇒ 命令面退 0，得到 %d", rc2)
	}
	row2 := g2JSONRow(t, out2)
	if !strings.HasPrefix(row2["per_rule"], "（读不到") || !strings.HasPrefix(row2["examples"], "（读不到") {
		t.Errorf("没有逐规则段 ⇒ 两格必须写「（读不到…）」哨兵（不许编数）：per_rule=%q · examples=%q",
			row2["per_rule"], row2["examples"])
	}

	// 字段面（合同面）：`--json` 不给字段时 stderr 现列的清单里必须有这两格。
	rc3, _, errb3 := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--json")
	if rc3 != 2 {
		t.Fatalf("--json 不给字段 ⇒ 退 2，得到 %d", rc3)
	}
	list := ""
	for _, ln := range strings.Split(errb3, "\n") {
		if strings.HasPrefix(ln, "可选字段: ") {
			list = strings.TrimPrefix(ln, "可选字段: ")
			break
		}
	}
	if list == "" {
		t.Fatalf("「--json 不给字段」要给字段清单：%q", errb3)
	}
	for _, f := range []string{"per_rule", "examples"} {
		found := false
		for _, x := range strings.Split(list, ",") {
			if strings.TrimSpace(x) == f {
				found = true
			}
		}
		if !found {
			t.Errorf("字段表里必须有 %s（合同面不许静默缩边）：%q", f, list)
		}
	}
}

// 断言前提：夹具脚本真落在那（防「夹具破了却看着像绿」）。
func TestGateRunStep_GapFixFixturesPresent(t *testing.T) {
	root := stepRepo(t, g2SelfTestStep)
	if _, err := os.Stat(filepath.Join(root, "scripts", "gates", "precommit-gates.sh")); err != nil {
		t.Fatalf("合成门禁脚本必须在：%v", err)
	}
}
