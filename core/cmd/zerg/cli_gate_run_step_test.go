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
//	⑥ `log_body`（门步**自己 stdout 正文**的读回 · 2026-09-27 补）：正控逐字相等 + 日志件缺失档不编数。
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
	// ★ 旧断言「stdout 必须 0 字节」已过期：本批起发射闸扩到 `inv.err != nil` ⇒ 错误面必是机器可读
	//   包封走 stdout（旧行为下本断言会红）；**结果面**仍必须空（`items` 一条都不出）。
	if !strings.Contains(out, `"items":[]`) || !strings.Contains(out, `"error"`) {
		t.Errorf("--json 不给字段 ⇒ stdout 必是 `error` 包封且 `items` 空：%q", out)
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

// ── ⑥ `log_body`：门步**自己 stdout 正文**的读回 ──
//
// 病（2026-09-27）：`log_body` 随 `a6a884fa` 进场（结果表第 5 列 = 该步日志件的绝对路径，原样读回），
// 但**无任何测试件钉它** ⇒ 字段可被静默改掉 / 读回失效而无人报。本组三格把它钉成**结构性质**：
//
//	① 正控：日志件在 ⇒ `log_body` 逐字 == **判据自己现读那一件**的正文（并查末行哨兵 ⇒ 不截断）；
//	② 日志件缺 ⇒ `log_body` 逐字 ==「（读不到）」+ stderr 点名**那一件的路径**（不许编数）；
//	③ 结果表整个缺 ⇒ `log_body` 走初值「（读不到：脚本没落结果表）」+ stderr 点名结果表。
//
// ★ 为什么**不钉字节数**：包封里印了 `outdir`（入参/临时目录），同一门步换个目录跑字节数就变
//   （血证：报告表把 `--outdir` 印进输出）⇒ 只钉「非空 / 与日志件正文逐字相等 / 读不到时那两枚字面量 + stderr 点名」。

// stepGateScriptWithLog —— 同一形状，但**先把日志件落下**（结果表第 5 列指向它）。
// 正文末行放一枚哨兵：断言「原样读回、不截断」靠它（不靠字节数）。
const stepGateScriptWithLog = `case "$*" in *--json*) exit 2 ;; esac
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outdir) out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "${out}"
printf '%s\n' '门步正文第一行' '逐格读数：正控绿 · 负控红' '末行哨兵-ZB9' > "${out}/00-合成步.log"
printf 'PASS\t合成步\t0\t0s\t%s/00-合成步.log\n' "${out}" > "${out}/results.tsv"
exit 0
`

// stepGateScriptLogMissing —— 结果表照样落，但它指的那件日志**从不创建**（日志件缺失档）。
const stepGateScriptLogMissing = `case "$*" in *--json*) exit 2 ;; esac
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outdir) out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "${out}"
printf 'PASS\t合成步\t0\t0s\t%s/00-日志缺失.log\n' "${out}" > "${out}/results.tsv"
exit 0
`

// stepGateScriptNoResults —— 目录建了，但**结果表整个不落**（走 row 初值那一档）。
const stepGateScriptNoResults = `case "$*" in *--json*) exit 2 ;; esac
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outdir) out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "${out}"
exit 0
`

// ⑥① 正控：`log_body` == 判据**自己现读**那一件日志的正文（逐字）。
func TestGateRunStep_LogBodyIsStepLogVerbatim(t *testing.T) {
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScriptWithLog)
	// ★ 显式 `--outdir`（本测试自己的目录）：命令面不给时会按**秒级**时间戳建临时目录，
	//   同一秒内的两次运行会共用一处 ⇒ 上一轮落的 results.tsv 被这一轮读成自己的（跨测试串味）。
	od := filepath.Join(t.TempDir(), "gate-out")
	rc, out, errb := execCase(t, bin, root, "gate", "run", "--step", "合成步",
		"--outdir", od, "--json", "step,verdict,rc,log,outdir,log_body")
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
	if row["log"] == "" || strings.HasPrefix(row["log"], "（读不到") {
		t.Fatalf("结果表第 5 列的日志件路径要先读回来：%+v", row)
	}
	// ★ 判据**不取包封里的字面量**，而是自己去读那一件，再与 `log_body` 对拍
	//   —— 这样验的才是「命令面原样读回」，而不是「印了两份一样的东西」。
	want, err := os.ReadFile(row["log"])
	if err != nil {
		t.Fatalf("夹具的日志件应当存在（%s）：%v", row["log"], err)
	}
	if len(want) == 0 {
		t.Fatalf("夹具日志件是空的 ⇒ 本格验不出东西：%s", row["log"])
	}
	if row["log_body"] != string(want) {
		t.Errorf("log_body 必须与日志件正文**逐字**相等（%s）：\n 期望 %q\n 实得 %q",
			row["log"], string(want), row["log_body"])
	}
	if row["log_body"] == "" {
		t.Errorf("log_body 不许为空（读得到就要给正文）：%+v", row)
	}
	// 不截断：末行哨兵仍在。只钉「在不在」，**不钉字节数**（输出里印了 outdir，字节随目录变）。
	if !strings.Contains(row["log_body"], "末行哨兵-ZB9") {
		t.Errorf("log_body 被截断了（末行哨兵不在）：%q", row["log_body"])
	}
}

// ⑥② 日志件缺失档：写「（读不到）」+ stderr 点名那一件（**不许编数**、也不许留空）。
func TestGateRunStep_LogBodyMissingLogIsNamedNotFaked(t *testing.T) {
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScriptLogMissing)
	od := filepath.Join(t.TempDir(), "gate-out")
	rc, out, errb := execCase(t, bin, root, "gate", "run", "--step", "合成步",
		"--outdir", od, "--json", "step,verdict,rc,log,log_body")
	if rc != 0 {
		t.Fatalf("判决仍照抄结果表 ⇒ 日志件缺失不该改退码，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || len(env.Items) != 1 {
		t.Fatalf("要一行判决的包封：%v · %q", err, out)
	}
	row := env.Items[0]
	missing := row["log"]
	if missing == "" || strings.HasPrefix(missing, "（读不到") {
		t.Fatalf("结果表第 5 列的路径要先读回来（本档的前提）：%+v", row)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatalf("夹具前提破了：%s 不该存在（本档要的就是「点名了一件读不到的」）", missing)
	}
	if row["log_body"] != "（读不到）" {
		t.Errorf("日志件读不到 ⇒ log_body 逐字「（读不到）」（不许编数、不许留空）：%q", row["log_body"])
	}
	// 点名：stderr 必须说出**是哪一件**读不到（只说「读不到」= 没说）。
	if !strings.Contains(errb, "步骤日志件读不到（"+missing+"）") {
		t.Errorf("stderr 要点名那一件的路径（%s）：%q", missing, errb)
	}
	if !strings.Contains(errb, "不许编格数") {
		t.Errorf("「不许编数」的口径要在 stderr 里说出来：%q", errb)
	}
	if row["verdict"] != "PASS" || row["rc"] != "0" {
		t.Errorf("缺失只作用在 log_body 那一格，判决照旧来自结果表：%+v", row)
	}
}

// ⑥③ 结果表整个缺 ⇒ `log_body` 走初值字面量（另一条分支）+ stderr 点名结果表。
func TestGateRunStep_LogBodyWithoutResultsTable(t *testing.T) {
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScriptNoResults)
	od := filepath.Join(t.TempDir(), "gate-out")
	rc, out, errb := execCase(t, bin, root, "gate", "run", "--step", "合成步",
		"--outdir", od, "--json", "step,verdict,rc,log,log_body")
	if rc != 0 {
		t.Fatalf("脚本退 0 ⇒ 命令面退 0（判决不给结论也照转退码），得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || len(env.Items) != 1 {
		t.Fatalf("要一行判决的包封：%v · %q", err, out)
	}
	row := env.Items[0]
	if row["log_body"] != "（读不到：脚本没落结果表）" {
		t.Errorf("结果表读不到 ⇒ log_body 走初值那一枚字面量（不许编数）：%q", row["log_body"])
	}
	if row["log"] != "（读不到）" {
		t.Errorf("结果表读不到 ⇒ log/log_body 同源、都得说读不到：%+v", row)
	}
	if !strings.Contains(errb, "结果表读不到") || !strings.Contains(errb, "results.tsv") {
		t.Errorf("stderr 要点名读不到的结果表：%q", errb)
	}
}

// ⑥④ 合同面：`log_body` 必须逐字挂在**字段表**上（`--json` 不给字段时 stderr 现列那一行）。
// 为什么不能只钉读回：读回那三格只回答「值对不对」；字段表是**另一处面**（`--json` 不给字段时
// 现列的那张清单 · 发射面按**请求的字段名**去结果行取值 · 见 `emitSelected`/`marshalObject`），
// 它被静默缩边时前面几格不一定报 ⇒ 合同面单独一格钉。
func TestGateRunStep_LogBodyIsInFieldContract(t *testing.T) {
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScriptWithLog)
	od := filepath.Join(t.TempDir(), "gate-out")
	rc, _, errb := execCase(t, bin, root, "gate", "run", "--step", "合成步",
		"--outdir", od, "--json")
	if want := usageCodeFromTable(t); rc != want {
		t.Fatalf("--json 不给字段 ⇒ 退 %d（取自退码表 `usage`），得到 %d · stderr=%s", want, rc, errb)
	}
	// 逐格取字段名（不钉顺序、不钉整行字面 —— 顺序是字段表的自由，缩边不是）。
	line := ""
	for _, ln := range strings.Split(errb, "\n") {
		if strings.HasPrefix(ln, "可选字段: ") {
			line = strings.TrimPrefix(ln, "可选字段: ")
			break
		}
	}
	if line == "" {
		t.Fatalf("「--json 不给字段」要给字段清单（K2 的提示面）：%q", errb)
	}
	found := false
	for _, f := range strings.Split(line, ",") {
		if strings.TrimSpace(f) == "log_body" {
			found = true
		}
	}
	if !found {
		t.Errorf("字段表里必须有 log_body（合同面不许静默缩边）：%q", line)
	}
}
