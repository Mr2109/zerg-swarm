// cli_gate_run_step_test.go —— `zerg gate run --step <步名>` 的**真二进制**判据（缺口-命令面 §三 `C3` ·
// 2026-09-21 · P0 之外的那条头条）。
//
// 钉住五件（全部走**合成门禁脚本**，不碰真门禁、不碰真目标）：
//
//	① 单步档的 `--json` 出**契约形状**的包封，且判决**逐格来自脚本自己落的结果表**（不是命令面自己判的）；
//	② `--json` **不透传给被包的脚本**（脚本见到它必红 —— 这正是「命令面旗标」与「脚本旗标」的分界）；
//	③ 步名未知名 ⇒ 退 2（**执行前判**：脚本一次都没被调用）+ 给出「最像的合法输入」（K14 第三件）；
//	④ `--json` 不给字段 ⇒ 退码取自退码表（`usage` · 归一后 = 2）且 stdout 0 字节（K2）；`--json` 不在单步档 ⇒ 退 2；
//	⑤ **脚本的退码原样转出**（合成脚本退 2 ⇒ 命令面退 2，不是「非 0 一律 1」）。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stepGateScript —— 合成门禁脚本：认识 `--step`/`--outdir`，落一行结果表，遇到 `--json` **必红**
// （② 的判据：命令面的旗标不许漏给脚本）。`$1` 是它收到的第一枚参数。
const stepGateScript = `if [ "${1:-}" = "--list" ]; then
  echo "── 步骤清单（scope 模式 名称）──"
  echo "  gates tri    合成步"
  echo "共 1 步"
  exit 0
fi
case "$*" in *--json*) echo "合成脚本收到了 --json（命令面的旗标漏下来了）"; exit 2 ;; esac
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outdir) out="$2"; shift 2 ;;
    --step) shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "${out}"
printf 'PASS\t合成步\t0\t0s\t%s/00-合成步.log\n' "${out}" > "${out}/results.tsv"
echo "合成报告（人面走 stderr）"
exit 0
`

// stepGateScriptFails —— 同一形状，但结果表里是 FAIL/rc=2（验「脚本退码原样转出」）。
const stepGateScriptFails = `case "$*" in *--json*) exit 2 ;; esac
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outdir) out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "${out}"
printf 'BLOCKED\t合成步\t2\t0s\t%s/00-合成步.log\n' "${out}" > "${out}/results.tsv"
exit 2
`

// stepRepo 建一个带 `add_step` 声明的合成仓（步名逐字 = `合成步`）。
func stepRepo(t *testing.T, body string) string {
	t.Helper()
	root := syntheticRepo(t, body)
	appendToFile(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		"\nadd_step gates \"合成步\" tri \"${REPO_ROOT}\" \"python3 scripts/gates/synthetic.py\"\n")
	return root
}

func TestGateRunStep_JSONComesFromScriptResults(t *testing.T) {
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScript)
	rc, out, errb := execCase(t, bin, root, "gate", "run", "--step", "合成步",
		"--json", "step,scope,mode,verdict,rc,secs,log,outdir")
	if rc != 0 {
		t.Fatalf("单步档正控 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	if len(env.Items) != 1 {
		t.Fatalf("要一行（这一步的判决），得到 %d 行：%s", len(env.Items), out)
	}
	row := env.Items[0]
	if row["step"] != "合成步" || row["scope"] != "gates" || row["mode"] != "tri" {
		t.Errorf("步身份读错：%+v", row)
	}
	// ① 判决来自脚本自己落的结果表 —— 命令面不自己判 PASS/FAIL。
	if row["verdict"] != "PASS" || row["rc"] != "0" || row["secs"] != "0s" {
		t.Errorf("判决必须逐格来自脚本的 results.tsv：%+v", row)
	}
	if !strings.HasSuffix(row["log"], "00-合成步.log") {
		t.Errorf("日志路径要来自结果表：%q", row["log"])
	}
	if row["outdir"] == "" {
		t.Errorf("outdir 要给出（结果表的读回点）：%+v", row)
	}
	// 人面（脚本的报告）走 stderr，不与机器面抢 stdout。
	if !strings.Contains(errb, "合成报告") {
		t.Errorf("脚本的人面报告应当转出到 stderr：%q", errb)
	}
	if strings.Contains(out, "合成报告") {
		t.Errorf("stdout 只许有机器面：%q", out)
	}
}

// ⑤ 脚本退码原样转出（**不翻译**）：合成脚本退 2 ⇒ 命令面退 2。
func TestGateRunStep_CarriesScriptExitCode(t *testing.T) {
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScriptFails)
	rc, out, _ := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--json", "step,verdict,rc")
	if rc != 2 {
		t.Fatalf("合成脚本退 2 ⇒ 命令面退 2（只转发、不翻译），得到 %d", rc)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || len(env.Items) != 1 {
		t.Fatalf("失败那一档也要有包封（判决 = 脚本给的 BLOCKED）：%v · %q", err, out)
	}
	if env.Items[0]["verdict"] != "BLOCKED" || env.Items[0]["rc"] != "2" {
		t.Errorf("判决要照抄脚本结果表（BLOCKED/2）：%+v", env.Items[0])
	}
}

// ③ 未知名 ⇒ 退 2（**执行前判**）+ 「最像的合法输入」。
func TestGateRunStep_UnknownStepIsUsageError(t *testing.T) {
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScript)
	rc, out, errb := execCase(t, bin, root, "gate", "run", "--step", "没有这一步-zz", "--json", "step")
	if rc != 2 {
		t.Fatalf("未知名 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "最像的合法输入: ") {
		t.Errorf("未知名要给「最像的合法输入」（K14 第三件 · 与 agent 族同一方言）：%q", errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
		Err   map[string]any      `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("失败那一档也要是合法包封：%v · %q", err, out)
	}
	if len(env.Items) != 0 {
		t.Errorf("执行前判就没过 ⇒ items 必须空（不许给半份）：%+v", env.Items)
	}
	if env.Err == nil || env.Err["kind"] != "usage" {
		t.Errorf("error.kind 要是 usage（AI 自愈靠它）：%+v", env.Err)
	}
}

// ④ K2：给了 `--json` 不给字段 ⇒ 退码**取自退码表**（`usage` · 归一后 = 2）且 stdout **0 字节**；`--json` 不在单步档 ⇒ 退 2。
func TestGateRunStep_JSONFieldDiscipline(t *testing.T) {
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScript)
	want := usageCodeFromTable(t)
	rc, out, _ := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--json")
	if rc != want {
		t.Errorf("--json 不给字段 ⇒ 退 %d（K2 那一档 · 取自退码表 `usage`），得到 %d", want, rc)
	}
	if out != "" {
		t.Errorf("--json 不给字段 ⇒ stdout 必须 0 字节：%q", out)
	}
	rc, out, errb := execCase(t, bin, root, "gate", "run", "--json", "step")
	if rc != 2 {
		t.Errorf("--json 不在单步档（缺 --step）⇒ 退 2，得到 %d", rc)
	}
	if !strings.Contains(errb, "只在**单步档**") {
		t.Errorf("要明说这条面只挂在单步档上：%q", errb)
	}
	if out == "" {
		t.Errorf("这一档要有错误包封（items 空 + error 块）：%q", out)
	}
}

// 域面（合成夹具的清理）：本文件不建真仓、不碰真门禁。
func TestGateRunStep_FixtureIsSynthetic(t *testing.T) {
	root := stepRepo(t, stepGateScript)
	if _, err := os.Stat(filepath.Join(root, "core", "internal", "version", "version.go")); err != nil {
		t.Fatalf("合成仓根判据件不在：%v", err)
	}
}
