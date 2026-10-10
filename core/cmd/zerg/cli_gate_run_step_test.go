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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// init —— 本件的测试会**真调**门跑（`zerg gate run --step`），而门跑带**单实例闸**
// （缺口 `GAP-20260927-214`）。分片跑时门分片器**并发起 4 个进程跑同一个包**，每个进程
// 都去拿**同一把** `<TMPDIR>/zerg-gate-instance/gate-run.lock` ⇒ 组内测试撞上「门自己那一趟
// 持有的锁」⇒ 一律 rc=2（缺口 `GAP-20261002-05`）。
//
// 处置：把**本进程**的锁目录指到进程私有处 —— 用正门旋钮 `ZERG_GATE_LOCK_DIR`
// （真源自陈「探针/沙盒用」，见 `family_gate_run_step.go` 的单实例闸注释）。每个分片进程
// 各得一把锁 ⇒ 与门那一趟互不见面。
//
// **不关闸**：不用 `ZERG_GATE_NO_LOCK`（关掉会把「闸存在」这件事本身关出测试面）；
// 只换锁域，闸照跑。进程级临时目录随系统清理（`init` 里没有 `*testing.T` 可用）。
func init() {
	// ★ **只在没被显式指定时才换锁域**：本包的测试会起「探针子进程」（`jlkSelfExe` 换成测试
	// 二进制自己，见 judge_lock_test.go），而子进程启动时会**再跑一遍本 `init()`**。若无条件换，
	// 就会把子进程从父测试**继承来的** `ZERG_GATE_LOCK_DIR`（`t.Setenv` 设的）顶掉 ⇒ 探针那一半与
	// 持锁那一半**锁域不再相同** ⇒ 「同名竞争」不再互阻 ⇒ 跨进程三态测试假红。
	// 显式设了（探针/沙盒/上述测试）就尊重它；只有「没人指过」时才换私有域 —— 那正是
	// 分片跑的场景（门持锁 ＋ 分片器并发起 4 个进程 ⇒ GAP-20261002-05）。
	if strings.TrimSpace(os.Getenv("ZERG_GATE_LOCK_DIR")) != "" {
		return
	}
	d, err := os.MkdirTemp("", "zerg-grs-lock-")
	if err == nil {
		_ = os.Setenv("ZERG_GATE_LOCK_DIR", d)
	}
}

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
	// ★ 判据**不取包封里的字面量**（缺口 `GAP-20260927-430` 起 `log` 的**目录段已掩码**成 `<TMP>/…`），
	//   而是照夹具**自己知道的**真路径去读那一件，再与 `log_body` 对拍 —— 这样验的才是
	//   「命令面原样读回」，而不是「印了两份一样的东西」；同时钉住包封至少**保留件名**。
	if !strings.HasSuffix(row["log"], "00-合成步.log") {
		t.Fatalf("包封的 log 要保留件名（掩码只抹目录段）：%q", row["log"])
	}
	realLog := filepath.Join(od, "00-合成步.log")
	want, err := os.ReadFile(realLog)
	if err != nil {
		t.Fatalf("夹具的日志件应当存在（%s）：%v", realLog, err)
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
	// 不截断：末行哨兵仍在。只钉「在不在」。（缺口 `GAP-20260927-430` 起目录段已掩码，字节也可钉了。）
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
	// ★ 缺口 `GAP-20260927-430`：包封的 `log` 目录段已掩码 ⇒ 点名那件的**真路径**照夹具自算
	//   （`--outdir od` ＋ 合成脚本写死的件名 `00-日志缺失.log`）；顺带钉住包封**保留件名**。
	if !strings.HasSuffix(row["log"], "00-日志缺失.log") {
		t.Fatalf("包封的 log 要保留件名（掩码只抹目录段）：%+v", row)
	}
	missing := filepath.Join(od, "00-日志缺失.log")
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

// ── ⑦ 不给 `--outdir` 时：**同秒两次运行不共享目录** + **本轮没落表读不回** ──
//
// 缺口 `GAP-20260927-433`（GAP-20260927-432 源码面已修 · 判据面为空）：旧形态按**秒级**时间戳
// 自建日志目录 + `MkdirAll` **复用同名目录** ⇒ 同一秒内两次运行共用同一处 results.tsv，后一次
// 会把前一次的判决读成自己的（陈旧判决回读 ⇒ 脚本这一轮没落表时「不许当绿」被旧表绕开）。
// 修法已落（`family_gate_run_step.go` 改 `os.MkdirTemp` 原子新建 · 名字带**纳秒** + 随机后缀）；
// 本条补**在册负控**，钉住「刚修的行为不许被静默回退」。
//
// stepGateScriptNoTable —— 认识 `--step`/`--outdir`，但**不落 results.tsv**（验读不回那一档）。
const stepGateScriptNoTable = `case "$*" in *--json*) exit 2 ;; esac
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outdir) out="$2"; shift 2 ;;
    --step) shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "${out}"
echo "合成报告（无结果表）"
exit 0
`

// gateRunStepOutdir —— 从单步档 `--json outdir` 的包封里取那一格。
func gateRunStepOutdir(t *testing.T, tag, out string) string {
	t.Helper()
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || len(env.Items) != 1 {
		t.Fatalf("%s：要一行包封：%v · %q", tag, err, out)
	}
	return env.Items[0]["outdir"]
}

// ⑦① 同秒两次运行 ⇒ `outdir` **必不同**（否则就是陈旧判决回读的那条路）。
func TestGateRunStep_NoOutdirIsDistinctPerCall(t *testing.T) {
	// 私有 TMPDIR：本测试真起两条门跑（各原子新建一处目录），确保不往共享 TMPDIR 里漏。
	t.Setenv("TMPDIR", t.TempDir())
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScript)

	_, o1, _ := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--json", "outdir")
	_, o2, _ := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--json", "outdir")
	d1, d2 := gateRunStepOutdir(t, "第一次运行", o1), gateRunStepOutdir(t, "第二次运行", o2)
	if d1 == "" || d2 == "" {
		t.Fatalf("不给 --outdir ⇒ 命令面要自建并把 outdir 报出来，不许为空：%q / %q", d1, d2)
	}
	if d1 == d2 {
		t.Errorf("同一秒内两次运行**共用**了日志目录（%q）⇒ 陈旧判决回读（缺口 `GAP-20260927-432/433`）：两次必须各建一处", d1)
	}
}

// ⑦② 本轮**没落结果表** ⇒ 判决与正文一律「读不到」（不许拿任何旧表当自己的）。
func TestGateRunStep_NoOutdirWithoutResultsTableReadsBackNothing(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	bin := zergBinary(t)
	root := stepRepo(t, stepGateScriptNoTable)

	rc, out, _ := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--json", "verdict,rc,log,log_body")
	if rc != 0 {
		t.Fatalf("合成脚本退 0 ⇒ 命令面退 0（只转发、不翻译），得到 %d", rc)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || len(env.Items) != 1 {
		t.Fatalf("要一行包封：%v · %q", err, out)
	}
	row := env.Items[0]
	if !strings.HasPrefix(row["verdict"], "（读不到") {
		t.Errorf("本轮没落结果表 ⇒ verdict 必须「（读不到…）」（不许当绿）：%+v", row)
	}
	if row["log_body"] != "（读不到：脚本没落结果表）" {
		t.Errorf("本轮没落表 ⇒ log_body 走初值那一枚字面量：%+v", row)
	}
}

// ── ⑨ 缺口 `GAP-20260927-430`：机器面**不印可变路径** ⇒ stdout 对同参稳定 ──
//
// 病：`gate run --step <步名> --json <字段>` 的包封把日志目录（**调用方给的** `--outdir` 或命令面
// **自建**的临时目录）印进 `outdir`/`log` ⇒ 同一 argv 换个目录名 / 换个 TMPDIR 形态就换一串字节
// （血证：同参 3517 B vs 3516 B，差 1 = 目录名 hold vs hb3 差 1）⇒ 矩阵格/字节型判据钉不住它。
// 修法：把日志目录抹成稳定占位符 `<TMP>`。本组：
//
//	① 正控：同一 argv（`--outdir <本机 TMPDIR 下的同一处>`）两次跑 ⇒ stdout **逐字节相同**，
//	   且真目录**不再**出现在 stdout 可比面上（抹成 `<TMP>`）；
//	③ 正控：**换个目录名**（hold vs hb3）掩码生效时结果**仍逐字节相同**（名字长短不漏字节）；
//	④ 正控：**不给 `--outdir`**（命令面自建那一档）⇒ 本机 TMPDIR 段也抹成 `<TMP>/`；
//	② 负控·哑化（「可变路径仍在可比面」⇒ 须真红）：`ZERG_GATE_STEP_NOMASK` 关掉掩码 ⇒ 真目录**必出现**、
//	   **换个目录名必不同** ⇒ 证明 ①/③ 的断言能判红（负控有牙）。
//
// ★ 本组走**从源码现编**（`zergBinaryFromSource`）：行为面改了，而 `bin/zerg` 本批**禁重编**
// ⇒ 走旧制品会把「本面未落制品」误读成「本面不存在」（同 ⑧ 口径）。
func TestGateRunStep_MachineFaceHidesVarDir(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := stepRepo(t, stepGateScript)
	tmp := t.TempDir()
	odA := filepath.Join(tmp, "hold") // 名字长短就是要抹掉的可变量（血证：hold vs hb3）
	odB := filepath.Join(tmp, "hb3")
	argv := func(od string) []string {
		return []string{"gate", "run", "--step", "合成步", "--outdir", od,
			"--json", "step,verdict,rc,log,outdir"}
	}

	// ① 正控：同一 argv 两次跑 ⇒ 逐字节相同；真目录不在可比面。
	_, a1, _ := execCase(t, bin, root, argv(odA)...)
	_, a2, _ := execCase(t, bin, root, argv(odA)...)
	if a1 != a2 {
		t.Errorf("缺口 `GAP-20260927-430`：同参两次跑 stdout 必须逐字节相同：\n  ① %q\n  ② %q", a1, a2)
	}
	if strings.Contains(a1, odA) {
		t.Errorf("可变路径（--outdir=%s）仍印在 stdout 可比面上：%q", odA, a1)
	}
	// ★ `<TMP>` 在 JSON 里被 HTML-转义成 `\u003cTMP\u003e` ⇒ 断言看**解包后的解码值**。
	if got := gateRunStepOutdir(t, "改后-hold", a1); got != "<TMP>" {
		t.Errorf("outdir 应抹成稳定占位符 `<TMP>`（解码值），得 %q", got)
	}

	// ③ 正控：换目录名（hold vs hb3）⇒ 掩码生效时结果仍逐字节相同。
	_, b1, _ := execCase(t, bin, root, argv(odB)...)
	if a1 != b1 {
		t.Errorf("换个目录名（%s vs %s）stdout 仍须逐字节相同（掩码吃掉目录段）：\n  hold %q\n  hb3  %q",
			odA, odB, a1, b1)
	}

	// ④ 正控：不给 `--outdir`（命令面自建那一档）⇒ 本机 TMPDIR 段抹成 `<TMP>/`（运行期唯一后缀保留）。
	_, c1, _ := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--json", "log,outdir")
	if td := os.TempDir(); td != "" && strings.Contains(c1, td) {
		t.Errorf("自建目录的本机 TMPDIR 段仍印在可比面上（%s）：%q", td, c1)
	}
	if got := gateRunStepOutdir(t, "改后-自建", c1); !strings.HasPrefix(got, "<TMP>/zerg-gate-step-") {
		t.Errorf("自建目录的 outdir 应抹成 `<TMP>/zerg-gate-step-…`（解码值），得 %q", got)
	}

	// ② 负控·哑化：关掉掩码 ⇒ 真目录必出现、换个目录名必不同（证明上面几条断言能判红）。
	t.Setenv("ZERG_GATE_STEP_NOMASK", "1")
	_, m1, _ := execCase(t, bin, root, argv(odA)...)
	_, m2, _ := execCase(t, bin, root, argv(odB)...)
	if !strings.Contains(m1, odA) {
		t.Errorf("哑化后真目录应当漏回可比面（否则「不出现」那条断言判不了红）：%q", m1)
	}
	if m1 == m2 {
		t.Errorf("哑化后换个目录名结果却相同 ⇒ 掩码测试无区分度（负控没牙）：%q", m1)
	}
}

// ── ⑧ 缺口 `GAP-20260927-276`：`gate run --step X -- <只读旗标>` 的**显式透传门** ──
//
// 病：`gate run --step` 带不了脚本自有旗标（除哨兵认识的那几枚）—— `--self-test`/`--json`/
// `--root` 这类**只读**旗标经命令面传不下去 ⇒ rc=2 拒执（且与 `gate run --help` 的「参数: 脚本
// 旗标（原样透传）」措辞不符）。治法 = 开一条**显式**透传门 `--`：`--` 之后按**只读白名单**逐字
// 附到那一步的命令串（复用 `--verify-live` 那条直跑路径），逐字列举 · ✗ 不得通配。口径：
//
//	① 无 `--` ⇒ 未知旗标照旧 rc=2（本组③钉住 —— 默认档一个字节不改）；
//	② `--` 之后**只读**旗标 ⇒ 逐字附到那一步的命令串上（本组①正控）；
//	③ `--` 之后**写面/危险档**旗标 ⇒ 仍逐字拒执 rc=2 且**不 exec**（本组②负控）。
//
// ★ 本组为什么**从源码现编**（`zergBinaryFromSource`）而不走 `bin/zerg`：本单硬约束禁重编
//   `bin/zerg`（行为面待重编）⇒ 走旧制品会把「本面未落制品」误读成「本面不存在」。现编只落
//   临时目录、不碰 `bin/zerg`（先例：`check_approve_tier.go` 的探针也现编 `./cmd/zerg`）。

var (
	srcBinOnce sync.Once
	srcBinPath string
	srcBinErr  error
)

// zergBinaryFromSource —— 从**当前源码**现编一份 CLI 到临时目录（不碰 `bin/zerg`）。
func zergBinaryFromSource(t *testing.T) string {
	t.Helper()
	srcBinOnce.Do(func() {
		d, err := os.MkdirTemp("", "zerg-276-bin-")
		if err != nil {
			srcBinErr = err
			return
		}
		out := filepath.Join(d, "zerg")
		cmd := exec.Command("go", "build", "-buildvcs=false", "-o", out, "./cmd/zerg")
		cmd.Dir = filepath.Join("..", "..") // core/
		var eb strings.Builder
		cmd.Stderr = &eb
		if err := cmd.Run(); err != nil {
			srcBinErr = fmt.Errorf("go build ./cmd/zerg: %v · %s", err, eb.String())
			return
		}
		srcBinPath = out
	})
	if srcBinErr != nil {
		t.Skipf("从源码现编 CLI 失败（本格跳过，不算行为面证据）：%v", srcBinErr)
	}
	return srcBinPath
}

// stepScriptBody —— 合成「被跑步骤」的命令脚本：打印收到的 argv；含 `--self-test` ⇒ 打哨兵。
const stepScriptBody = `#!/usr/bin/env bash
echo "STEP-ARGV: $*"
case " $* " in
  *" --self-test "*) echo "SELFTEST-OK" ;;
esac
exit 0
`

// stepRepoWithScript —— 合成仓：`合成步` 的命令串 = `bash scripts/gates/synthetic-step.sh`
// （纯 argv ⇒ 走单步命令串透传路径），脚本落在仓内可跑。
func stepRepoWithScript(t *testing.T) string {
	t.Helper()
	root := syntheticRepo(t, "add_step gates \"合成步\" tri \"${REPO_ROOT}\" \"bash scripts/gates/synthetic-step.sh\"\n")
	p := filepath.Join(root, "scripts", "gates", "synthetic-step.sh")
	mustWrite(t, p, stepScriptBody)
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// ⑧① 正控：`-- <只读旗标>` 逐字透给那一步的命令串（脚本真收到那枚旗标）。
func TestGateRunStep_DashPassthroughReadonlyFlag(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := stepRepoWithScript(t)
	rc, out, errb := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--", "--self-test")
	if rc != 0 {
		t.Fatalf("`-- --self-test`（只读）⇒ 透传成功退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "STEP-ARGV: --self-test") {
		t.Errorf("那一步的脚本必须**逐字**收到 `--self-test`：%q", out)
	}
	if !strings.Contains(out, "SELFTEST-OK") {
		t.Errorf("只读旗标要真作用到脚本上（不是只回显）：%q", out)
	}
}

// ⑧② 负控：`-- <写面/危险档旗标>` 仍拒执（rc=2 + 理由明确）且**脚本一次都没被调用**；
// 同格再钉「无 `--` 的未知旗标仍 rc=2」（默认档一个字节不改）。
func TestGateRunStep_DashPassthroughRejectsWriteFlag(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := stepRepoWithScript(t)
	rc, out, errb := execCase(t, bin, root, "gate", "run", "--step", "合成步", "--", "--apply")
	if rc != 2 {
		t.Fatalf("`-- --apply`（写面）⇒ 必须仍 rc=2 拒执，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "非只读旗标: --apply") {
		t.Errorf("负控理由要点名那枚旗标：%q", errb)
	}
	if strings.Contains(out, "STEP-ARGV") {
		t.Errorf("拒执必须在 exec 之前 ⇒ 脚本不许被跑：%q", out)
	}
	rc, _, errb = execCase(t, bin, root, "gate", "run", "--step", "合成步", "--frobnicate")
	if rc != 2 || !strings.Contains(errb, "未知参数: --frobnicate") {
		t.Errorf("默认档：无 `--` 的未知旗标仍须 rc=2「未知参数」，得到 rc=%d · stderr=%q", rc, errb)
	}
}
