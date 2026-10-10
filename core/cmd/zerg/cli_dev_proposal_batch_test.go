// cli_dev_proposal_batch_test.go —— `dev proposal new --batch`（可机读批量档）的机检。
// 缺口 GAP-20260927-413：`dev proposal new` 在「多旗标 + for 循环 / 管道」拼命令的形态下会卡
// **调用侧**的人工应答（调用侧审批层判「无法解析的嵌套执行体」⇒ BLOCKED · 零写入）⇒ 批量建提案
// 时子代理必停手。本件钉住缺口要的那条出口（**可机读**的批量档）与两条负控。
//
// 落点：`package main_test` + `zerg.RunForTest`（跑**当前源码**的行为，不是盘上旧制品）。
// 全部合成盘面：`ZERG_PROPOSAL_DIR` 指 `t.TempDir()` ⇒ 真 `~/.zerg/proposals` **一个字节不碰**。
//
// 钉住的格（逐条可单独失败）：
//
//	① 正控 · 单片档逐字不变：一枚合法单片 ⇒ rc=0 · 落一件（单枚形态照旧通过）。
//	② 正控 · 批量档建多枚：两格合法批量件 ⇒ rc=0 · **落两件**（DEV-0001/0002 · 机器面 `--json` 可读）。
//	③ 负控 · 批量不挂住（快失败 + 点名 + 零写）：第 2 格缺 `target` ⇒ rc=2 · stderr 点名「第 2 格」·
//	   目录里**一枚未写**（不是「跳过坏的、写好第 1 格」= 不许把挂住换成静默部分成功）。
//	④ 负控 · 批量形状：件不是 JSON / 空数组 / 认不得的键 / 与单片旗标同给 ⇒ 各自 rc=2 且点名。
//	⑤ 负控 · 审批语义未放宽：批量档也**要 `--yes`**（缺 ⇒ rc=2 · 零写）；单片旧档位（缺 --yes ·
//	   坏 target）逐字不变。
package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// pnbEnv 合成提案目录 + 一枚「现读可得」的退点件（真状态目录一个字节不碰）。
func pnbEnv(t *testing.T) (dir, rollback string) {
	t.Helper()
	dir = t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)
	rb := filepath.Join(t.TempDir(), "family_dev.go.before")
	if err := os.WriteFile(rb, []byte("（退点夹具 · 现读可得）\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, rb
}

func pnbRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// pnbItem 一枚形状合法的批量格（各测试自换坏格）。
func pnbItem(rollback, title string) string {
	return `{"title":"` + title + `","target":"D3b","goal":"目标一句话",` +
		`"evidence":["描述性出处（无前缀自由文本 ⇒ 不走在册闸）"],` +
		`"rollback":"` + rollback + `",` +
		`"criterion":"zerg code find proposalNewBatch",` +
		`"file":["core/cmd/zerg/family_dev.go"]}`
}

func pnbSpecFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "batch.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// pnbSingleArgv 一枚合法单片档的旗标（各测试自换坏格）。
func pnbSingleArgv(rollback string) []string {
	return []string{"dev", "proposal", "new",
		"--title", "单片夹具", "--target", "D3b", "--goal", "目标一句话",
		"--evidence", "描述性出处（无前缀自由文本）",
		"--rollback", rollback,
		"--criterion", "zerg code find proposalNewBatch",
		"--file", "core/cmd/zerg/family_dev.go"}
}

// pnbProposalIDs 读合成提案目录里的全部提案 id（`*.json`）。
func pnbProposalIDs(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	ids := []string{}
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return ids
}

// ---- ① 正控 · 单片档逐字不变 ----
func TestPnb_SingleUnchanged(t *testing.T) {
	dir, rb := pnbEnv(t)
	rc, out, errb := pnbRun(t, append(pnbSingleArgv(rb), "--yes")...)
	if rc != 0 {
		t.Fatalf("① 破：单片合法档 rc=%d（要 0）\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, "DEV-0001") {
		t.Errorf("① 破：单片档 stdout 未见提案号\n%s", out)
	}
	if ids := pnbProposalIDs(t, dir); len(ids) != 1 {
		t.Errorf("① 破：单片档应落 1 件，实落 %d 件（%v）", len(ids), ids)
	}
}

// ---- ② 正控 · 批量档一次建多枚 ----
func TestPnb_BatchWritesMultiple(t *testing.T) {
	dir, rb := pnbEnv(t)
	sp := pnbSpecFile(t, "["+pnbItem(rb, "甲")+","+pnbItem(rb, "乙")+"]")
	rc, out, errb := pnbRun(t, "dev", "proposal", "new", "--batch", sp, "--yes")
	if rc != 0 {
		t.Fatalf("② 破：批量档 rc=%d（要 0）\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	if ids := pnbProposalIDs(t, dir); len(ids) != 2 {
		t.Fatalf("② 破：批量档应落 2 件，实落 %d 件（%v）", len(ids), ids)
	}
	for _, want := range []string{"DEV-0001", "DEV-0002"} {
		if !strings.Contains(out, want) {
			t.Errorf("② 破：批量档 stdout 未见 %s\n%s", want, out)
		}
	}
	// 机器面：`--json` 与 `dev proposal list` 同形（可机读）
	rcJ, outJ, errJ := pnbRun(t, "dev", "proposal", "new", "--batch",
		pnbSpecFile(t, "["+pnbItem(rb, "丙")+"]"), "--yes", "--json", "id,title")
	if rcJ != 0 {
		t.Fatalf("② 破：批量档 `--json` rc=%d（要 0）\nstderr=%s", rcJ, errJ)
	}
	var env struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(outJ)), &env); err != nil {
		t.Fatalf("② 破：批量档 `--json` 不是合法 JSON：%v\n%s", err, outJ)
	}
	if len(env.Items) != 1 {
		t.Errorf("② 破：批量档 `--json` items=%d（要 1）", len(env.Items))
	}
}

// ---- ③ 负控 · 批量不挂住：快失败 + 点名 + 零写 ----
func TestPnb_BatchBadItemFastFailsZeroWrite(t *testing.T) {
	dir, rb := pnbEnv(t)
	// 第 2 格缺 `target`（其余合法）
	bad := `{"title":"坏格","goal":"目标","evidence":["描述性出处"],"rollback":"` + rb +
		`","criterion":"zerg code find proposalNewBatch","file":["core/cmd/zerg/family_dev.go"]}`
	sp := pnbSpecFile(t, "["+pnbItem(rb, "甲")+","+bad+"]")
	rc, out, errb := pnbRun(t, "dev", "proposal", "new", "--batch", sp, "--yes")
	if rc == 0 {
		t.Fatalf("③ 破：第 2 格缺 target 竟 rc=0（挂住 / 静默成功都算破）\nstdout=%s", out)
	}
	if rc != 2 {
		t.Errorf("③ 破：rc=%d（要 2 = 用法错 · 快速失败）", rc)
	}
	if !strings.Contains(errb, "第 2 格") {
		t.Errorf("③ 破：失败没**点名到格**（要出现「第 2 格」）\nstderr=%s", errb)
	}
	if ids := pnbProposalIDs(t, dir); len(ids) != 0 {
		t.Errorf("③ 破：先全验后全写被破 —— 落 %d 件（%v）· 应一枚未写", len(ids), ids)
	}
}

// ---- ④ 负控 · 批量形状各档快失败 ----
func TestPnb_BatchShapes(t *testing.T) {
	_, rb := pnbEnv(t)
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"件不是 JSON", []string{"dev", "proposal", "new", "--batch", pnbSpecFile(t, "not json"), "--yes"}, "解不动"},
		{"空数组", []string{"dev", "proposal", "new", "--batch", pnbSpecFile(t, "[]"), "--yes"}, "0 条"},
		{"认不得的键", []string{"dev", "proposal", "new", "--batch", pnbSpecFile(t, `[{"nope":"x"}]`), "--yes"}, "认不得的键"},
		{"与单片旗标互斥", []string{"dev", "proposal", "new", "--batch", pnbSpecFile(t, "["+pnbItem(rb, "甲")+"]"),
			"--title", "混着给", "--yes"}, "互斥"},
		{"件读不到", []string{"dev", "proposal", "new", "--batch", filepath.Join(t.TempDir(), "nope.json"), "--yes"}, "读不到"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc, _, errb := pnbRun(t, c.argv...)
			if rc != 2 {
				t.Errorf("④ 破：%s rc=%d（要 2）\nstderr=%s", c.name, rc, errb)
			}
			if !strings.Contains(errb, c.want) {
				t.Errorf("④ 破：%s 的判词没点名 %q\nstderr=%s", c.name, c.want, errb)
			}
		})
	}
}

// ---- ⑤ 负控 · 审批语义未放宽 ----
func TestPnb_ApprovalNotLoosened(t *testing.T) {
	dir, rb := pnbEnv(t)
	// 批量档缺 `--yes` ⇒ 退 2 · 零写（真写仍要 `--yes` —— 与单片档同一条 fail-closed 门）
	sp := pnbSpecFile(t, "["+pnbItem(rb, "甲")+"]")
	rc, _, errb := pnbRun(t, "dev", "proposal", "new", "--batch", sp)
	if rc != 2 || !strings.Contains(errb, "--yes") {
		t.Errorf("⑤ 破：批量档缺 --yes rc=%d（要 2 且点名 --yes）\nstderr=%s", rc, errb)
	}
	if ids := pnbProposalIDs(t, dir); len(ids) != 0 {
		t.Errorf("⑤ 破：缺 --yes 竟落了 %d 件", len(ids))
	}
	// 单片旧档位：缺 `--yes` ⇒ 2（逐字不变）
	rc2, _, errb2 := pnbRun(t, pnbSingleArgv(rb)...)
	if rc2 != 2 || !strings.Contains(errb2, "--yes") {
		t.Errorf("⑤ 破：单片缺 --yes rc=%d（旧档位行为应逐字不变 · 要 2）\nstderr=%s", rc2, errb2)
	}
	// 单片旧档位：坏 target ⇒ 2 且点名（逐字不变）
	badT := []string{"dev", "proposal", "new", "--title", "t", "--target", "Z9", "--goal", "g",
		"--evidence", "描述性出处", "--rollback", rb, "--criterion", "zerg code find proposalNewBatch",
		"--file", "core/cmd/zerg/family_dev.go", "--yes"}
	rc3, _, errb3 := pnbRun(t, badT...)
	if rc3 != 2 || !strings.Contains(errb3, "Z9") {
		t.Errorf("⑤ 破：单片坏 target rc=%d（要 2 且点名 Z9）\nstderr=%s", rc3, errb3)
	}
	if ids := pnbProposalIDs(t, dir); len(ids) != 0 {
		t.Errorf("⑤ 破：被拒的那几发竟落了 %d 件", len(ids))
	}
}
