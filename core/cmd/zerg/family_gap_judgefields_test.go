// family_gap_judgefields_test.go —— 缺口 `GAP-20261010-37` 的**回归牙**（本单新增件）。
//
// 面：`gap` 判据执行面的分词器（`gapCmdFields`）必须**剥 shell 引号**（引号只作**分组**），
// 而**不带引号者切词逐字不变**（本仓绝大多数账条不带引号 ⇒ 改动面必须零行为差）。
//
// 病（修前）：四处解析面都走 `strings.Fields` ⇒ 不剥引号 ⇒ 账内带引号的 `verify_cmd` 正则被吃成
// 字面撇号（`'doc consumers'` ⇒ `'doc` ＋ `consumers'`）⇒ 恒零命中 rc=1 **假红**
// （实测 `GAP-20261008-14` / `GAP-20261008-09`）。
//
// 三格：〔甲〕不带引号 ⇒ 与 `strings.Fields` 逐字同；〔乙〕带引号 ⇒ 剥引号后仍**一个参数**；
// 〔端到端〕带引号 `verify_cmd` 经 `gap regress`（⇒ `gapJudgeArgv` ⇒ `run`）真跑 ⇒ rc=0。
// 负控（哑化 `gapCmdFields` 为 `strings.Fields` ⇒ 〔乙〕/〔端到端〕应红）在件外以 `go test -overlay` 跑。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 〔甲〕不带引号（亦无反引号）⇒ 分词与旧切词 `strings.Fields` **逐字同**。
func TestGapCmdFieldsUnquotedIdenticalToFields(t *testing.T) {
	for _, c := range []string{
		"zerg code find doc consumers --path core/cmd/zerg",
		"zerg gate run --step gofmt -l core --only-step",
		"zerg gap ls --json id",
		"zerg code find func --path core/cmd/zerg --json path,line,text",
		"zerg doctor",
	} {
		got := gapCmdFields(c)
		want := strings.Fields(c)
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("不带引号串分词必须逐字同 strings.Fields\n in  = %q\n got = %q\n want= %q", c, got, want)
		}
	}
}

// ── 〔乙〕带引号的多词参数 ⇒ 引号只作**分组**：剥一对引号后仍**一个参数**（`doc consumers` 不被切碎）。
func TestGapCmdFieldsQuotedGroupingSingleArg(t *testing.T) {
	want := []string{"zerg", "code", "find", "doc consumers", "--path", "publish/docs/CLI.zh-CN.md"}
	for _, c := range []string{
		"zerg code find 'doc consumers' --path publish/docs/CLI.zh-CN.md",
		"zerg code find \"doc consumers\" --path publish/docs/CLI.zh-CN.md",
	} {
		got := gapCmdFields(c)
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("带引号串必须按分组剥引号（仍一个参数）\n in  = %q\n got = %q\n want= %q", c, got, want)
		}
	}
}

// ── 〔端到端〕一条**带引号**的 `verify_cmd` 经 `gap regress`（⇒ `gapJudgeArgv` ⇒ `run`）真跑：
// 修前该型恒零命中 rc=1（假红）· 修后真判 rc=0。夹具 = 合成账 + 合成状态目录（零副作用）。
func TestGapRegressQuotedVerifyCmdReallyJudges(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	unit := "core/cmd/zerg/family_gap.go" // 必须是仓内真件（gap regress 先做件存在性检查）
	rec := map[string]any{
		"id": "GAP-20261010-37-FIXTURE", "fp": "fp-quoted-cmd", "symptom": "夹具（带引号判据）",
		"handmade": "夹具", "impact": "命令面", "prio": "P1", "state": "仍缺",
		"want_family": "gap", "want_action": "ls",
		"repro_cmd":  "zerg gap ls",
		"verify_cmd": "zerg code find 'gapJudgeArgv' --path core/cmd/zerg/family_gap.go",
		"found_at":   "2026-10-10T00:00:00+08:00", "unit": unit,
	}
	line, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("夹具行序列化失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zerg-cli-gaps.jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Fatalf("写夹具真源失败：%v", err)
	}
	var out, errb bytes.Buffer
	rc := RunForTest([]string{"gap", "regress", unit}, &out, &errb)
	if rc != 0 {
		t.Fatalf("带引号 verify_cmd 经 gap regress 应 rc=0（修前假红 rc=1）\n rc=%d\n out=%s\n err=%s",
			rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "通过 1") {
		t.Fatalf("应「通过 1」（= 真判，不是拒跑/零条）：%s", out.String())
	}
}
