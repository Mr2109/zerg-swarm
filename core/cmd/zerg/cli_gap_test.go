// cli_gap_test.go —— `gap` 族（`ls` / `add` / `verify`）的**机检**（设计稿 §六 判据 10 条 · §七 负控 8 枚）。
//
// 落点：`package main_test` + `zerg.RunForTest` —— 跑的是**当前源码**的行为（不是盘上旧制品），
// 与 `cli_matrix_test.go` 的层①同一条路。全部合成盘面（`t.TempDir()` 当状态目录）：
// **不碰真 `~/.zerg/state`**、不碰任何仓内件、不真跑任何危险动作（判据用 `version` / `gap ls` 这类只读面）。
//
// 本件钉住的九格（逐格对设计稿）：
//
//	① 三态齐（`--dry-run=0` / 缺 `--yes`=2 / `--yes=0`；`ls` 取到 0/1/2 与 8）
//	② 干跑零副作用（真源 sha256 逐字不变 + 审计行数不变）
//	③ 真写「变」+ 读回（行数 +1、sha 变、审计尾行的 after_sha256 == 现算值）
//	④ 幂等（同 fp 同内容 ⇒ 0 且不新增行）
//	⑤ 冲突（同 fp 内容不同 ⇒ 14 且给既有 id）
//	⑥ 必填两件（缺 `--handmade` / 缺 `--verify-cmd` ⇒ 各 2 且真源行数不变）
//	⑦ 判据一把尺（`--verify-cmd` 不在树里 ⇒ 2）
//	⑧ 判决成对（过 ⇒ 已解 + `solved_evidence` + `solved_at` 晚于 `found_at`；判据改坏 ⇒ `回归` + rc=1）
//	⑨ 审计不撞名（`gap_ledger_written` 的字段名 ∩ 现网那六格 = ∅ · 必需格齐）
//
// 负控（§七）：① 真源读不到 ⇒ 8 · ② 真源在但 0 行 ⇒ `ls` 1 / `verify --all` 2 ·
// ③ 未知旗标与未知命令 ⇒ 2 · ④ 人面写 `state=已解` ⇒ 2 · ⑥ 审计写不进 ⇒ 真源不写 + 8 ·
// ⑦ 干跑不落审计行 · ⑧ 只读面不写审计行。（⑤ 闸② `L3` 的牙在 `ls` 侧的「账内越界」判红上，同在本件。）
package main_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// gapEnv 合成状态目录：真源与审计都落在这里（审计与 `dev edit` 同一件 —— 本件就钉这一点）。
func gapEnv(t *testing.T) (state, ledger, audit string) {
	t.Helper()
	state = t.TempDir()
	ledger = filepath.Join(state, "zerg-cli-gaps.jsonl")
	audit = filepath.Join(state, "edit_audit.jsonl")
	t.Setenv("ZERG_STATE_DIR", state)
	t.Setenv("ZERG_EDIT_AUDIT", audit)
	t.Setenv("ZERG_BY", "合成夹具")
	return
}

func gapRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// gapAddArgv 一条**形状合法**的 `add`（各测试自换 `--handmade` / `--symptom` 造不同内容）。
func gapAddArgv(handmade, symptom string, extra ...string) []string {
	a := []string{"gap", "add", "--symptom", symptom, "--handmade", handmade,
		"--impact", "命令面", "--want-family", "gap", "--want-action", "ls",
		"--repro-cmd", "zerg gap ls", "--verify-cmd", "zerg version --json name"}
	return append(a, extra...)
}

func gapReadLines(t *testing.T, p string) []string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读不了 %s：%v", p, err)
	}
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

func gapAuditLines(t *testing.T, p string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return rows
		}
		t.Fatalf("读不了审计件：%v", err)
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("审计件第 %d 行不是 JSON：%v", len(rows)+1, err)
		}
		rows = append(rows, m)
	}
	return rows
}

// gapIDOf 从 add 的人面输出里取 id（`已落账 <id> · …`）。
func gapIDOf(t *testing.T, out string) string {
	t.Helper()
	f := strings.Fields(out)
	for i, w := range f {
		if w == "已落账" && i+1 < len(f) {
			return f[i+1]
		}
	}
	t.Fatalf("输出里没找到 id：%q", out)
	return ""
}

// gapFirstID 取账里第一条的 id（走 `--json`，不让测试去猜人面格式）。
func gapFirstID(t *testing.T) string {
	t.Helper()
	rc, out, errb := gapRun(t, "gap", "ls", "--json", "id")
	if rc != 0 {
		t.Fatalf("gap ls --json id 要 0：rc=%d · %s", rc, errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || len(env.Items) == 0 {
		t.Fatalf("解析不了 `gap ls --json id`：%v · %q", err, out)
	}
	return env.Items[0]["id"]
}

func gapSHAOf(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "（不在盘上）"
		}
		t.Fatalf("读不了 %s：%v", p, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ── 判据 ① 三态齐（`ls` 0/1/2/8 · `add` 与 `verify` 各三态）────────────────────────────────────

func TestGapFamily_ThreeStates(t *testing.T) {
	_, ledger, _ := gapEnv(t)

	// `ls` 的第一个码：真源读不到 ⇒ 8（负控①）
	if rc, _, errb := gapRun(t, "gap", "ls"); rc != 8 {
		t.Errorf("负控①：真源不在 ⇒ ls 要 8，得到 %d · stderr=%s", rc, errb)
	}
	// `ls` 的第二个码：用法错 ⇒ 2
	if rc, _, _ := gapRun(t, "gap", "ls", "--prio", "P3"); rc != 2 {
		t.Errorf("`--prio P3` 在闭集外 ⇒ 2，得到 %d", rc)
	}
	// `add` 三态
	if rc, out, _ := gapRun(t, append(gapAddArgv("h-三态", "三态"), "--dry-run")...); rc != 0 {
		t.Errorf("add --dry-run ⇒ 0，得到 %d · %s", rc, out)
	}
	if _, err := os.Stat(ledger); !os.IsNotExist(err) {
		t.Errorf("干跑后真源不该出现（%v）", err)
	}
	if rc, out, errb := gapRun(t, gapAddArgv("h-三态", "三态")...); rc != 2 {
		t.Errorf("add 缺 --yes ⇒ 2，得到 %d · out=%q err=%s", rc, out, errb)
	} else if out != "" {
		t.Errorf("缺 --yes 那一态 stdout 要 0 字节（H-5：本族格只进退码面），得到 %q", out)
	}
	if rc, out, errb := gapRun(t, append(gapAddArgv("h-三态", "三态"), "--yes")...); rc != 0 {
		t.Fatalf("add --yes ⇒ 0，得到 %d · out=%s err=%s", rc, out, errb)
	}
	// `ls` 的第三个码：有命中 ⇒ 0
	if rc, out, errb := gapRun(t, "gap", "ls"); rc != 0 {
		t.Errorf("ls 有命中 ⇒ 0，得到 %d · out=%s err=%s", rc, out, errb)
	}
	// `verify` 三态（判据 = `version`，必过）
	id := gapFirstID(t)
	if rc, out, errb := gapRun(t, "gap", "verify", id, "--dry-run"); rc != 0 {
		t.Errorf("verify --dry-run ⇒ 0，得到 %d · %s%s", rc, out, errb)
	}
	if rc, out, _ := gapRun(t, "gap", "verify", id); rc != 2 {
		t.Errorf("verify 缺 --yes ⇒ 2，得到 %d · %q", rc, out)
	}
	if rc, out, errb := gapRun(t, "gap", "verify", id, "--yes"); rc != 0 {
		t.Errorf("verify --yes ⇒ 0，得到 %d · %s%s", rc, out, errb)
	}
}

// ── 判据 ② 干跑零副作用 + 负控 ⑦ 干跑不落审计行 ────────────────────────────────────────────────

func TestGapFamily_DryRunZeroSideEffect(t *testing.T) {
	_, ledger, audit := gapEnv(t)
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-干跑", "干跑"), "--yes")...); rc != 0 {
		t.Fatalf("打底那次 --yes 没成：rc=%d · %s", rc, errb)
	}
	before := gapSHAOf(t, ledger)
	beforeAudit := len(gapAuditLines(t, audit))

	if rc, out, _ := gapRun(t, append(gapAddArgv("h-干跑-新", "干跑新行"), "--dry-run")...); rc != 0 {
		t.Fatalf("干跑要 0：rc=%d · %s", rc, out)
	}
	if after := gapSHAOf(t, ledger); after != before {
		t.Errorf("干跑不许动真源：sha256 %s → %s（判据②）", before, after)
	}
	if after := len(gapAuditLines(t, audit)); after != beforeAudit {
		t.Errorf("干跑不许落审计行：%d → %d（负控⑦）", beforeAudit, after)
	}
}

// ── 判据 ③ 真写「变」+ 读回（审计尾行的 after_sha256 == 现算值）────────────────────────────────

func TestGapFamily_RealWriteThenReadback(t *testing.T) {
	_, ledger, audit := gapEnv(t)
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-读回", "读回"), "--yes")...); rc != 0 {
		t.Fatalf("add --yes 要 0：rc=%d · %s", rc, errb)
	}
	rows := gapReadLines(t, ledger)
	if len(rows) != 1 {
		t.Fatalf("真源要 1 行，得到 %d", len(rows))
	}
	lines := gapAuditLines(t, audit)
	if len(lines) != 1 {
		t.Fatalf("审计要 1 行，得到 %d", len(lines))
	}
	last := lines[len(lines)-1]
	if last["event"] != "gap_ledger_written" {
		t.Errorf("事件名要是 gap_ledger_written，得到 %v", last["event"])
	}
	if got, want := last["gap_ledger_after_sha256"], gapSHAOf(t, ledger); got != want {
		t.Errorf("审计里的 after_sha256 = %v（现算 %s）—— 写后读回对不上", got, want)
	}
	if n, ok := last["gap_ledger_after_lines"].(float64); !ok || int(n) != 1 {
		t.Errorf("gap_ledger_after_lines 要 1，得到 %v", last["gap_ledger_after_lines"])
	}
	if last["confirm"] != "--yes" {
		t.Errorf("confirm 要 --yes，得到 %v", last["confirm"])
	}
	if last["gap_cmd"] != "add" {
		t.Errorf("gap_cmd 要 add，得到 %v", last["gap_cmd"])
	}
}

// ── 判据 ④ 幂等 + ⑤ 冲突（防呆②）────────────────────────────────────────────────────────────

func TestGapFamily_IdempotentAndConflict(t *testing.T) {
	_, ledger, audit := gapEnv(t)
	argv := gapAddArgv("h-幂等", "幂等")
	if rc, _, errb := gapRun(t, append(argv, "--yes")...); rc != 0 {
		t.Fatalf("第一次 add --yes 要 0：rc=%d · %s", rc, errb)
	}
	id1 := gapFirstID(t)
	rows1 := len(gapReadLines(t, ledger))
	audit1 := len(gapAuditLines(t, audit))

	// ④ 同 fp 同内容 ⇒ 0 且不新增行（也不写审计：没写真源）
	if rc, out, errb := gapRun(t, append(argv, "--yes")...); rc != 0 {
		t.Errorf("幂等命中要 0，得到 %d · %s · %s", rc, out, errb)
	} else if !strings.Contains(out, "幂等命中") {
		t.Errorf("幂等命中要在输出里明说，得到 %q", out)
	}
	if n := len(gapReadLines(t, ledger)); n != rows1 {
		t.Errorf("幂等命中不许新增行：%d → %d（防呆②）", rows1, n)
	}
	if n := len(gapAuditLines(t, audit)); n != audit1 {
		t.Errorf("幂等命中没写真源 ⇒ 也不写审计：%d → %d", audit1, n)
	}

	// ⑤ 同 fp 而内容不同 ⇒ 14 且给既有 id
	rc, out, errb := gapRun(t, append(gapAddArgv("h-幂等", "换一句症状"), "--yes")...)
	if rc != 14 {
		t.Errorf("同 fp 内容不同要 14（防呆②），得到 %d · %s%s", rc, out, errb)
	}
	if !strings.Contains(errb, id1) {
		t.Errorf("冲突判词要给出既有 id %q，得到 %q", id1, errb)
	}
	if n := len(gapReadLines(t, ledger)); n != rows1 {
		t.Errorf("冲突不许落行：%d → %d", rows1, n)
	}
}

// ── 判据 ⑥ 必填两件 + ⑦ 判据一把尺 + 负控 ④ 人面写 state ──────────────────────────────────────

func TestGapFamily_RequiredAndJudgeRuler(t *testing.T) {
	_, ledger, _ := gapEnv(t)
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-打底", "打底"), "--yes")...); rc != 0 {
		t.Fatalf("打底失败：rc=%d %s", rc, errb)
	}
	n0 := len(gapReadLines(t, ledger))

	// ⑥ 缺 `--handmade`
	argv := []string{"gap", "add", "--symptom", "s", "--impact", "命令面", "--want-family", "gap",
		"--want-action", "ls", "--repro-cmd", "zerg gap ls", "--verify-cmd", "zerg version --json name", "--yes"}
	if rc, _, _ := gapRun(t, argv...); rc != 2 {
		t.Errorf("缺 --handmade ⇒ 2（防呆⑤），得到 %d", rc)
	}
	// ⑥ 缺 `--verify-cmd`
	argv2 := []string{"gap", "add", "--symptom", "s", "--handmade", "h", "--impact", "命令面",
		"--want-family", "gap", "--want-action", "ls", "--repro-cmd", "zerg gap ls", "--yes"}
	if rc, _, _ := gapRun(t, argv2...); rc != 2 {
		t.Errorf("缺 --verify-cmd ⇒ 2（防呆⑤），得到 %d", rc)
	}
	if n := len(gapReadLines(t, ledger)); n != n0 {
		t.Errorf("拒收不许动真源：%d → %d", n0, n)
	}
	// ⑦ 判据不在命令树 ⇒ 2
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-尺", "尺", "--verify-cmd", "zerg 无此命令-zz"), "--yes")...); rc != 2 {
		t.Errorf("--verify-cmd 不在树里 ⇒ 2（v1.3 §七 规矩 2），得到 %d · %s", rc, errb)
	}
	// ⑦′ 判据是本族自己的写命令 ⇒ 2（自递归闸）
	if rc, _, _ := gapRun(t, append(gapAddArgv("h-尺2", "尺2", "--verify-cmd", "zerg gap verify"), "--yes")...); rc != 2 {
		t.Errorf("判据 = 本族写命令 ⇒ 2（自递归），得到 %d", rc)
	}
	// ④ 人面写 `已解` ⇒ 2（防呆④）
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-已解", "已解", "--state", "已解"), "--yes")...); rc != 2 {
		t.Errorf("人面 --state 已解 ⇒ 2（防呆④），得到 %d · %s", rc, errb)
	}
	if rc, _, _ := gapRun(t, "gap", "verify", "--all", "--state", "已解", "--yes"); rc != 2 {
		t.Errorf("verify 收 --state ⇒ 2，得到 %d", rc)
	}
}

// ── 判据 ⑧ 判决成对（过 ⇒ 已解 + 证据；判据改坏 ⇒ 回归 + rc=1）──────────────────────────────────

func TestGapFamily_VerifyJudgePairs(t *testing.T) {
	state, ledger, _ := gapEnv(t)
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-判决", "判决"), "--yes")...); rc != 0 {
		t.Fatalf("打底失败：rc=%d %s", rc, errb)
	}
	id := gapFirstID(t)

	if rc, out, errb := gapRun(t, "gap", "verify", id, "--yes"); rc != 0 {
		t.Fatalf("判据过 ⇒ verify --yes 要 0，得到 %d · %s%s", rc, out, errb)
	}
	rec := gapRecordOf(t, ledger, id)
	if rec["state"] != "已解" {
		t.Fatalf("判据过 ⇒ state=已解，得到 %v", rec["state"])
	}
	if rec["solved_evidence"] == nil || strings.TrimSpace(rec["solved_evidence"].(string)) == "" {
		t.Errorf("已解必须带 solved_evidence，得到 %v", rec["solved_evidence"])
	}
	found, err1 := time.Parse(time.RFC3339Nano, rec["found_at"].(string))
	solved, err2 := time.Parse(time.RFC3339Nano, rec["solved_at"].(string))
	err := err1
	if err == nil {
		err = err2
	}
	if err != nil || !solved.After(found) {
		t.Errorf("solved_at 要晚于 found_at（found=%s solved=%v err=%v）", rec["found_at"], rec["solved_at"], err)
	}

	// 「把那条命令改坏」：把该条的 `verify_cmd` 换一条**能解析但会失败**的命令 ⇒ 已解现缺 ⇒ 回归 + rc=1
	bad := swapGapVerifyCmd(t, ledger, id, "zerg version --json nosuchfield-zz")
	if !bad {
		t.Fatalf("夹具没换成（ledger=%s）", ledger)
	}
	rc, out, errb := gapRun(t, "gap", "verify", id, "--yes")
	if rc != 1 {
		t.Errorf("已解现缺 ⇒ rc=1（唯一能让 verify 退 1 的东西），得到 %d · %s%s", rc, out, errb)
	}
	if rec := gapRecordOf(t, ledger, id); rec["state"] != "回归" {
		t.Errorf("已解现缺 ⇒ state=回归，得到 %v", rec["state"])
	}

	// `--dry-run` 恒 0：同一条（已回归）再干跑 ⇒ 0，且 state 一个字节不变
	before := gapSHAOf(t, ledger)
	if rc, _, _ := gapRun(t, "gap", "verify", id, "--dry-run"); rc != 0 {
		t.Errorf("verify --dry-run 恒 0，得到 %d", rc)
	}
	if after := gapSHAOf(t, ledger); after != before {
		t.Errorf("干跑不许动真源：%s → %s", before, after)
	}
	_ = state
}

// gapRecordOf 取真源里某 id 那一行（解析成 map，方便逐格断言）。
func gapRecordOf(t *testing.T, ledger, id string) map[string]any {
	t.Helper()
	for _, ln := range gapReadLines(t, ledger) {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("真源行不是 JSON：%v", err)
		}
		if m["id"] == id {
			return m
		}
	}
	t.Fatalf("真源里没有 %s", id)
	return nil
}

// swapGapVerifyCmd 把某条的 `verify_cmd` 换掉（**手改真源**：模拟「判据那条命令被改坏」）。
func swapGapVerifyCmd(t *testing.T, ledger, id, cmd string) bool {
	t.Helper()
	var out []string
	hit := false
	for _, ln := range gapReadLines(t, ledger) {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("真源行不是 JSON：%v", err)
		}
		if m["id"] == id {
			m["verify_cmd"] = cmd
			hit = true
		}
		enc, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("序列化不过：%v", err)
		}
		out = append(out, string(enc))
	}
	if err := os.WriteFile(ledger, []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("写回真源失败：%v", err)
	}
	return hit
}

// ── 判据 ⑨ 审计不撞名（拿**落地当天**的字段并集现算，不是抄设计稿）──────────────────────────────

func TestGapFamily_AuditNoNameClash(t *testing.T) {
	_, _, audit := gapEnv(t)
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-审计", "审计"), "--yes")...); rc != 0 {
		t.Fatalf("add --yes 要 0：rc=%d %s", rc, errb)
	}
	rows := gapAuditLines(t, audit)
	if len(rows) != 1 {
		t.Fatalf("要 1 行审计，得到 %d", len(rows))
	}
	line := rows[0]
	// 必需格（缺任一 ⇒ 这一行不许写 —— 这是它落下来的前提，故这里断言它齐）
	for _, k := range []string{"at", "event", "gap_cmd", "gap_ledger_path", "gap_ledger_before_sha256", "gap_ledger_after_sha256"} {
		if v, ok := line[k]; !ok || strings.TrimSpace(toStr(v)) == "" {
			t.Errorf("必需格 %q 缺或空（设计稿 §四）", k)
		}
	}
	// 判据 9：本族字段名 ∩ 现网那六格 = ∅
	six := []string{"before_sha256", "after_sha256", "before_bytes", "after_bytes", "approval", "approver"}
	for k := range line {
		for _, s := range six {
			if k == s {
				t.Errorf("审计字段 %q 与现网六格撞名（判据 9）", k)
			}
		}
	}
	// 本稿新引入的 10 格逐格在
	for _, k := range []string{"gap_cmd", "gap_id", "gap_fp", "gap_state_before", "gap_state_after",
		"gap_ledger_path", "gap_ledger_before_sha256", "gap_ledger_after_sha256",
		"gap_ledger_before_lines", "gap_ledger_after_lines"} {
		if _, ok := line[k]; !ok {
			t.Errorf("设计稿 §四 的 10 格少了 %q", k)
		}
	}
}

func toStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// ── 负控（§七）：② 0 行 · ① 读不到 · ③ 未知旗标/命令 · ⑥ 审计写不进 · ⑧ 只读面不写审计 ──────────

func TestGapFamily_LsWritesNoAudit(t *testing.T) {
	_, ledger, audit := gapEnv(t)
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-只读", "只读"), "--yes")...); rc != 0 {
		t.Fatalf("打底失败：rc=%d %s", rc, errb)
	}
	// 模拟「已有过别的写面写过审计」：先塞一行，再跑 `ls`，行数必须逐字不变
	if err := os.WriteFile(audit, []byte("{\"at\":\"x\",\"event\":\"edit\"}\n"), 0o644); err != nil {
		t.Fatalf("夹具写审计失败：%v", err)
	}
	n0 := len(gapAuditLines(t, audit))
	if rc, _, errb := gapRun(t, "gap", "ls"); rc != 0 {
		t.Fatalf("ls 有命中要 0：rc=%d %s", rc, errb)
	}
	if n := len(gapAuditLines(t, audit)); n != n0 {
		t.Errorf("负控⑧：只读面不许写审计（%d → %d）", n0, n)
	}
	_ = ledger
}

func TestGapFamily_NegativeControls(t *testing.T) {
	state, ledger, audit := gapEnv(t)

	// ② 真源在但 0 行：`ls` ⇒ 1（零命中）· `verify --all` ⇒ 2（不给结论 · 闸② L5）
	if err := os.WriteFile(ledger, nil, 0o644); err != nil {
		t.Fatalf("夹具建空真源失败：%v", err)
	}
	if rc, _, errb := gapRun(t, "gap", "ls"); rc != 1 {
		t.Errorf("负控②：0 行 ⇒ ls 要 1，得到 %d · %s", rc, errb)
	}
	if rc, _, errb := gapRun(t, "gap", "verify", "--all", "--yes"); rc != 2 {
		t.Errorf("负控②：0 行 + --all ⇒ 2（不给结论），得到 %d · %s", rc, errb)
	}
	// ③ 未知旗标 / 未知命令
	if rc, _, _ := gapRun(t, "gap", "ls", "--nope-zz"); rc != 2 {
		t.Errorf("负控③：未知旗标 ⇒ 2，得到 %d", rc)
	}
	if rc, _, _ := gapRun(t, "gapls"); rc != 2 {
		t.Errorf("负控③：未知命令 ⇒ 2，得到 %d", rc)
	}
	// ① 真源读不到 ⇒ `ls` / `verify` 都 8（**不许当绿**）
	if err := os.Remove(ledger); err != nil {
		t.Fatalf("夹具删真源失败：%v", err)
	}
	if rc, _, _ := gapRun(t, "gap", "ls"); rc != 8 {
		t.Errorf("负控①：ls ⇒ 8，得到 %d", rc)
	}
	if rc, _, _ := gapRun(t, "gap", "verify", "--all", "--yes"); rc != 8 {
		t.Errorf("负控①：verify ⇒ 8，得到 %d", rc)
	}
	if rc, _, _ := gapRun(t, "gap", "verify", "--all", "--dry-run"); rc != 8 {
		t.Errorf("负控①：verify --dry-run 也 ⇒ 8（真源读不到不许当绿），得到 %d", rc)
	}
	// ⑤ 账内越界（手改真源塞一行 state=乱）⇒ 判红 1 且判词第一行逐字 `账内越界：<行号> <字段>`
	bad := "{\"id\":\"GAP-20260923-98\",\"fp\":\"x\",\"symptom\":\"s\",\"handmade\":\"h\",\"impact\":\"命令面\"," +
		"\"prio\":\"P1\",\"state\":\"乱\",\"want_family\":\"gap\",\"want_action\":\"ls\",\"repro_cmd\":\"x\"," +
		"\"verify_cmd\":\"zerg version\",\"found_at\":\"2026-09-23T00:00:00+08:00\"}\n"
	if err := os.WriteFile(ledger, []byte(bad), 0o644); err != nil {
		t.Fatalf("夹具写坏账失败：%v", err)
	}
	rc, out, errb := gapRun(t, "gap", "ls")
	if rc != 1 {
		t.Errorf("账内越界 ⇒ 判红 1，得到 %d", rc)
	}
	if out != "" {
		t.Errorf("判红那一跑 stdout 要 0 字节，得到 %q", out)
	}
	if !strings.HasPrefix(errb, "账内越界：1 state\n") {
		t.Errorf("判词第一行要逐字 `账内越界：1 state`，得到 %q", gapFirstLine(errb))
	}
	// 零命中那一条判词的第一行必须**不同**（不许把两条判词印成同一行）
	if err := os.Remove(ledger); err != nil {
		t.Fatalf("夹具删真源失败：%v", err)
	}
	if err := os.WriteFile(ledger, nil, 0o644); err != nil {
		t.Fatalf("夹具建空真源失败：%v", err)
	}
	_, _, zero := gapRun(t, "gap", "ls")
	if gapFirstLine(zero) == gapFirstLine(errb) {
		t.Errorf("「零命中」与「账坏了」判词第一行不许相同：%q", gapFirstLine(zero))
	}
	// ⑥ 审计写不进 ⇒ 真源也不写 + 8
	t.Setenv("ZERG_EDIT_AUDIT", "/dev/null/不可以")
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-审计写不进", "审计写不进"), "--yes")...); rc != 8 {
		t.Errorf("负控⑥：审计写不进 ⇒ 8，得到 %d · %s", rc, errb)
	}
	if b, err := os.ReadFile(ledger); err != nil || len(b) != 0 {
		t.Errorf("负控⑥：审计写不进 ⇒ 真源一个字节都不写（len=%d err=%v）", len(b), err)
	}
	_ = audit
	_ = state
}

func gapFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
