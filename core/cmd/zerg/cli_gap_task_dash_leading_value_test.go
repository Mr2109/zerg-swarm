// cli_gap_task_dash_leading_value_test.go —— 取值以减号开头这一族的两态判据
// （缺口 `GAP-20260926-27` · `gap add --symptom` ／ `GAP-20260926-31` · `task submit --acceptance`）。
//
// 病（改前现读实测）：
//
//	zerg gap add --symptom -一句症状 …      ⇒ rc=2 `未知旗标 "-一句症状"`（值被判成旗标）
//	zerg task submit --acceptance -某判据   ⇒ rc=2 `未知旗标 "-某判据"`
//	zerg task submit --acceptance -- -某判据 ⇒ rc=0 但 `-某判据` 落进位置参数被**静默丢弃**
//	                                          （请求体 `"acceptance":null` —— 假绿，最坏的一态）
//
// 治法（本单）：`--` 分隔符档 `<旗标> -- <值…>`（仓内先例 = `fs run` 把 `--` 之后转给子命令）。
// 本件钉三样：
//
//	① 两态——改前拒 / 改后收（改前的读数用 `-overlay` 映回前态件现跑，见交付报告）；
//	② 三档并存——`--` 档 · `=` 档 · 普通空格值档都收，且**显式空**（`--acceptance=`）仍是 `null`；
//	③ 没有放宽——空格形态 `--acceptance -减号值` 照旧拒（`未知旗标`），裸给无 `--` 的
//	   `--symptom` 照旧「给过、值空 ⇒ 缺必填」，即别的旗标解析一字不动。
//
// 全部合成盘面（`gapEnv(t)` = `t.TempDir()` 当状态目录）⇒ 不碰真 `~/.zerg/state`。
package main_test

import (
	"strings"
	"testing"
)

const (
	dashLeadingSymptom = "-开头症状（减号开头）"
	dashLeadingJudge   = "-某判据（减号开头）"
)

// gapAddDashBase —— 除 `--symptom` 外一条形状合法的 `gap add`。
// ★ 判据取**非样板值**（`--repro-cmd` / `--verify-cmd` 不入通用命令闭集 · `GAP-20260927-246`）。
func gapAddDashBase() []string {
	return []string{"gap", "add",
		"--handmade", "手搓 h", "--impact", "命令面",
		"--want-family", "gap", "--want-action", "ls",
		"--repro-cmd", "zerg gap verify-one --dry-run",
		"--verify-cmd", "zerg version --json name"}
}

// TestGapAddDashLeadingSymptom —— `gap add` 收下以减号开头的症状（且只在这两种写形下收）。
func TestGapAddDashLeadingSymptom(t *testing.T) {
	// ① 本单的真修点：`--` 分隔符档。★ `--` 之后的 token 一律是值 ⇒ 其余旗标都在它之前。
	t.Run("--分隔符档", func(t *testing.T) {
		gapEnv(t)
		argv := append(gapAddDashBase(), "--dry-run", "--symptom", "--", dashLeadingSymptom)
		rc, out, errb := gapRun(t, argv...)
		if rc != 0 {
			t.Fatalf("rc=%d want 0（改前这里是 2「缺必填 --symptom」）· stderr=%s", rc, errb)
		}
		if !strings.Contains(out, "symptom   : "+dashLeadingSymptom) {
			t.Fatalf("计划件没把以减号开头的症状收进 symptom 格：\n%s", out)
		}
		if !strings.Contains(errb, "`--` 分隔符档") {
			t.Fatalf("收下时应当响亮点名 `--` 分隔符档（✗ 静默）：\n%s", errb)
		}
	})

	// ② 等号形态（同一缺口的另一半出口 · 改前也已能收）：钉住，防以后被改坏。
	t.Run("等号形态", func(t *testing.T) {
		gapEnv(t)
		argv := append(gapAddDashBase(), "--symptom="+dashLeadingSymptom, "--dry-run")
		rc, out, errb := gapRun(t, argv...)
		if rc != 0 {
			t.Fatalf("rc=%d want 0 · stderr=%s", rc, errb)
		}
		if !strings.Contains(out, "symptom   : "+dashLeadingSymptom) {
			t.Fatalf("等号形态没把值收进 symptom 格：\n%s", out)
		}
	})

	// ③ 普通值一字不动。
	t.Run("普通值不变", func(t *testing.T) {
		gapEnv(t)
		argv := append(gapAddDashBase(), "--symptom", "普通症状", "--dry-run")
		rc, out, errb := gapRun(t, argv...)
		if rc != 0 {
			t.Fatalf("rc=%d want 0 · stderr=%s", rc, errb)
		}
		if !strings.Contains(out, "symptom   : 普通症状") {
			t.Fatalf("普通值档被改坏了：\n%s", out)
		}
	})

	// ④ 负控：裸给且**无** `--` ⇒ 仍是「给过、值空」⇒ 缺必填（本单没动裸形语义）。
	t.Run("裸给无--仍按空收", func(t *testing.T) {
		gapEnv(t)
		argv := append(gapAddDashBase(), "--symptom", "--dry-run")
		rc, _, errb := gapRun(t, argv...)
		if rc != 2 {
			t.Fatalf("rc=%d want 2（裸形语义：给过、值空 ⇒ 缺必填）", rc)
		}
		if !strings.Contains(errb, "缺必填旗标") || !strings.Contains(errb, "--symptom") {
			t.Fatalf("应点名缺 --symptom：\n%s", errb)
		}
	})

	// ⑤ 负控：`--` 之后不止一个 token ⇒ 歧义 ⇒ 用法错 2（✗ 不猜、不静默取第一个）。
	t.Run("--后多值按歧义拒", func(t *testing.T) {
		gapEnv(t)
		argv := append(gapAddDashBase(), "--dry-run", "--symptom", "--", "-甲", "-乙")
		rc, _, errb := gapRun(t, argv...)
		if rc != 2 {
			t.Fatalf("rc=%d want 2", rc)
		}
		if !strings.Contains(errb, "只收**一个**值") {
			t.Fatalf("应报「只收一个值」：\n%s", errb)
		}
	})
}

// TestTaskSubmitDashLeadingAcceptance —— `task submit` 收下以减号开头的判据（三态语义不变）。
func TestTaskSubmitDashLeadingAcceptance(t *testing.T) {
	// ① 本单的真修点：`--` 分隔符档（改前这一发 rc=0 却把值静默丢成 `"acceptance":null`）。
	t.Run("--分隔符档", func(t *testing.T) {
		rc, out, errb := gapRun(t, "task", "submit", "--desc", "d", "--dry-run", "--acceptance", "--", dashLeadingJudge)
		if rc != 0 {
			t.Fatalf("rc=%d want 0 · stderr=%s", rc, errb)
		}
		if !strings.Contains(out, `"acceptance":["`+dashLeadingJudge+`"]`) {
			t.Fatalf("请求体没收到以减号开头的判据（改前是 \"acceptance\":null）：\n%s", out)
		}
		if !strings.Contains(errb, "`--` 分隔符档") {
			t.Fatalf("收下时应当响亮点名 `--` 分隔符档（✗ 静默）：\n%s", errb)
		}
	})

	// ② 可重复旗标：`--` 之后每个 token 各成一条判据（次序照给）。
	t.Run("--后多值各成一条", func(t *testing.T) {
		rc, out, errb := gapRun(t, "task", "submit", "--desc", "d", "--dry-run",
			"--acceptance", "--", "-甲判据", "-乙判据")
		if rc != 0 {
			t.Fatalf("rc=%d want 0 · stderr=%s", rc, errb)
		}
		if !strings.Contains(out, `"acceptance":["-甲判据","-乙判据"]`) {
			t.Fatalf("多值没按次序进请求体：\n%s", out)
		}
	})

	// ③ 等号形态（改前已能收）：钉住。
	t.Run("等号形态", func(t *testing.T) {
		rc, out, errb := gapRun(t, "task", "submit", "--desc", "d", "--dry-run", "--acceptance="+dashLeadingJudge)
		if rc != 0 {
			t.Fatalf("rc=%d want 0 · stderr=%s", rc, errb)
		}
		if !strings.Contains(out, `"acceptance":["`+dashLeadingJudge+`"]`) {
			t.Fatalf("等号形态没进请求体：\n%s", out)
		}
	})

	// ④ 三态里的「显式空」一字不动：`--acceptance=` ⇒ `null`，不是 `[]`、更不是本条的值。
	t.Run("显式空仍是null", func(t *testing.T) {
		rc, out, _ := gapRun(t, "task", "submit", "--desc", "d", "--dry-run", "--acceptance=")
		if rc != 0 {
			t.Fatalf("rc=%d want 0", rc)
		}
		if !strings.Contains(out, `"acceptance":null`) {
			t.Fatalf("显式空三态被改坏了（应是 null）：\n%s", out)
		}
	})

	// ⑤ 负控：空格形态照旧拒（本单只加 `--` / `=` 两条出口，✗ 没放宽解析器）。
	t.Run("空格形态仍拒", func(t *testing.T) {
		rc, _, errb := gapRun(t, "task", "submit", "--desc", "d", "--dry-run", "--acceptance", dashLeadingJudge)
		if rc != 2 {
			t.Fatalf("rc=%d want 2", rc)
		}
		if !strings.Contains(errb, "未知旗标") || !strings.Contains(errb, dashLeadingJudge) {
			t.Fatalf("应照旧报「未知旗标」并点名那个值：\n%s", errb)
		}
	})
}
