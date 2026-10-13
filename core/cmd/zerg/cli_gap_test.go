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
// ★ `--repro-cmd` / `--verify-cmd` 都取**非样板值** —— 写侧样板值闸 `GAP-20260927-246`
// 会拒收 `zerg task ls` / `zerg gap ls` 一类通用命令闭集，故形如 `zerg gap ls` 的占位不再合法。
func gapAddArgv(handmade, symptom string, extra ...string) []string {
	a := []string{"gap", "add", "--symptom", symptom, "--handmade", handmade,
		"--impact", "命令面", "--want-family", "gap", "--want-action", "ls",
		"--repro-cmd", "zerg gap verify-one --dry-run", "--verify-cmd", "zerg version --json name"}
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
	nonEmpty := 0
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		nonEmpty++
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("审计件第 %d 行不是 JSON：%v", len(rows)+1, err)
		}
		rows = append(rows, m)
	}
	// ★ ② 冻结下限 = 1（棘轮 · 只许增）：审计件非空即须至少解析出 1 条记录 ——
	//   结构静默变小（解析面漏行 / 抽到 0 条）⇒ 本格判红，✗ 不许静默绿。
	gN := len(rows)
	if nonEmpty > 0 && gN < 1 {
		t.Fatalf("② 冻结下限 1 被破：审计件 %d 行非空却只解析出 %d 条记录（棘轮 · 只许增）", nonEmpty, gN)
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
	// ★ 缺口 `GAP-20260928-195`（本枚）：判据批量面销案同走**共用两态闸** —— 新账未带两态读数
	//   ⇒ **拒写**（退 2 · 一字不写）；带成对读数 ⇒ 放行（下面正控）。此前该面绕道无闸。
	bsha := gapSHAOf(t, ledger)
	if rc, _, errb := gapRun(t, "gap", "verify", id, "--yes"); rc != 2 {
		t.Errorf("verify 销案缺两态读数 ⇒ 2（共用闸 · 负控要真红），得到 %d · %s", rc, errb)
	}
	if gapSHAOf(t, ledger) != bsha {
		t.Errorf("verify 负控（缺读数拒写）动了真源")
	}
	if rc, out, errb := gapRun(t, "gap", "verify", id, "--yes",
		"--before-read", "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前该号列于仍缺",
		"--after-read", "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后该号列于已解"); rc != 0 {
		t.Errorf("verify --yes（带两态）⇒ 0，得到 %d · %s%s", rc, out, errb)
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

// ── 判据 ⑩ 写侧样板值闸（缺口 GAP-20260927-246 靶心 ＋ GAP-20260926-55 写侧拒绝）──────────────
//
// 两态（拒收）：`--verify-cmd` / `--repro-cmd` 填通用命令闭集（`zerg task ls` / `zerg gap ls`）
// ⇒ rc=2、点名「样板值」、真源不动。负控（放行）：真判据（`zerg gate check … --dry-run` ＋
// 非样板 repro）⇒ rc=0 能进。判的是**进程退码**与其自己那档的拒绝理由（✗ 不卡死整句汉字）。
func TestGapFamily_BoilerplateRejected(t *testing.T) {
	_, ledger, _ := gapEnv(t)

	base := func(repro, verify string) []string {
		return []string{"gap", "add", "--symptom", "样板值闸夹具", "--handmade", "手搓 h",
			"--impact", "命令面", "--want-family", "gap", "--want-action", "ls",
			"--repro-cmd", repro, "--verify-cmd", verify, "--yes"}
	}

	// 态①：`--verify-cmd` 样板值（`zerg task ls` ⇒ 通用命令闭集）⇒ 2 并点名「样板值」
	rc, _, errb := gapRun(t, base("go test ./... -run X", "zerg task ls")...)
	if rc != 2 {
		t.Errorf("--verify-cmd 样板值 ⇒ 2，得到 %d · %s", rc, errb)
	}
	if !strings.Contains(errb, "样板值") || !strings.Contains(errb, "--verify-cmd") {
		t.Errorf("拒收判词要点名「样板值」与 --verify-cmd，得到 %q", errb)
	}
	// 态①′：`--repro-cmd` 样板值（`zerg gap ls` ⇒ 通用命令闭集）⇒ 2 并点名「样板值」
	rc, _, errb = gapRun(t, base("zerg gap ls", "zerg version --json name")...)
	if rc != 2 {
		t.Errorf("--repro-cmd 样板值 ⇒ 2，得到 %d · %s", rc, errb)
	}
	if !strings.Contains(errb, "样板值") || !strings.Contains(errb, "--repro-cmd") {
		t.Errorf("拒收判词要点名「样板值」与 --repro-cmd，得到 %q", errb)
	}
	// 两态都在任何盘面动作之前拒 —— 真源一个字节都不许建
	if _, err := os.Stat(ledger); !os.IsNotExist(err) {
		t.Errorf("样板值拒收不许动真源（%v）", err)
	}

	// 负控：真判据放行 ⇒ 0 能进（非样板 verify ＋ 非样板 repro）
	rc, out, errb := gapRun(t, base("go test ./... -run X", "zerg gate check check-verify-cmd-real --dry-run")...)
	if rc != 0 {
		t.Fatalf("真判据要放行 ⇒ 0，得到 %d · %s · %s", rc, out, errb)
	}
	if n := len(gapReadLines(t, ledger)); n != 1 {
		t.Errorf("真判据要落 1 行，得到 %d", n)
	}
}

// ── 判据 ⑪ 写侧现跑预检闸（缺口 GAP-20261013-14 靶心）───────────────────────────────────────────
//
// 病（修前）：`--verify-cmd` 只有静态闸（样板值 ＋ 解析面）、**不真跑** ⇒ 「命令树里解析得到、
// 真跑却恒退 2」的判据照样入库、且永远变不绿。治：写侧**真跑一次取 rc** —— rc=2 且错文点名
// 未知旗标 / 未知字段 / 缺必填子命令 ⇒ 拒收；其余 rc（0 / 1 / 8 一类有鉴别力的读数）⇒ 收。
//
// 两态（拒收）：① 未知 `--json` 字段（`zerg gate results --json scope` 真跑 rc=2 · `未知字段 "scope"`）
// ② 未知 `--json` 字段（另一条件 · `zerg version --json nosuchfield-zz` 真跑 rc=2 · `未知字段`）
// ⇒ 各 rc=2、点名「现跑预检」、真源不动。负控（放行）：真跑得动的判据（`zerg version --json name`
// rc=0）⇒ rc=0 能进。判的是**进程退码**与其自己那档的拒绝理由（✗ 不卡死整句汉字）。
//
// ★ 缺口 `GAP-20261005-34`（本单 DEV-2742）改了**闸的次序**：`gapWriteFaceWhy` 现在对
// **原样透传型**（`cmd.passthrough`）也有分支 ⇒ 透传型判据在**静态闸**（`gapJudgeArgv`）就被拒，
// **到不了**本档「现跑预检」（这是**加严**：本档要**真跑**才出判词，而透传型真跑＝拉起被包脚本）。
// ⇒ 本件的态①夹具由 `zerg fleet logs`（**透传型** · 旧时靠本档拒）换成 `zerg gate results --json scope`
// （**非透传** · 同样 rc=2 且点名「未知字段」⇒ 仍走本档）；旧夹具另立一格「态①′」钉住**新**归属：
// 它现在由透传闸拒（rc=2 且点名「原样透传型」）—— 覆盖不丢、归属如实。
func TestGapFamily_PreflightRejected(t *testing.T) {
	_, ledger, _ := gapEnv(t)

	base := func(symptom, verify string) []string {
		return []string{"gap", "add", "--symptom", symptom, "--handmade", "手搓 h",
			"--impact", "命令面", "--want-family", "gap", "--want-action", "ls",
			"--repro-cmd", "go test ./... -run X", "--verify-cmd", verify, "--yes"}
	}

	// 态①：未知 `--json` 字段（`zerg gate results --json scope` 真跑 rc=2 · `未知字段`）⇒ 2 并点名「现跑预检」
	rc, _, errb := gapRun(t, base("现跑预检-未知字段", "zerg gate results --json scope")...)
	if rc != 2 {
		t.Errorf("未知 json 字段 ⇒ 2，得到 %d · %s", rc, errb)
	}
	if !strings.Contains(errb, "现跑预检") || !strings.Contains(errb, "--verify-cmd") {
		t.Errorf("拒收判词要点名「现跑预检」与 --verify-cmd，得到 %q", errb)
	}
	// 态①′（本单 DEV-2742 新归属）：同一条旧夹具 `zerg fleet logs` 是**原样透传型** ⇒ 现在由
	// **静态闸**拒（在透传闸那一档），仍 rc=2、点名「原样透传型」与 `--verify-cmd`（✗ 不放行）。
	rc, _, errb = gapRun(t, base("透传型判据", "zerg fleet logs")...)
	if rc != 2 {
		t.Errorf("透传型判据 ⇒ 2，得到 %d · %s", rc, errb)
	}
	if !strings.Contains(errb, "原样透传型") || !strings.Contains(errb, "--verify-cmd") {
		t.Errorf("透传型拒收判词要点名「原样透传型」与 --verify-cmd，得到 %q", errb)
	}
	// 态②：未知 `--json` 字段（`zerg version --json nosuchfield-zz` 真跑 rc=2 · `未知字段`）⇒ 2
	rc, _, errb = gapRun(t, base("现跑预检-未知字段", "zerg version --json nosuchfield-zz")...)
	if rc != 2 {
		t.Errorf("未知 json 字段 ⇒ 2，得到 %d · %s", rc, errb)
	}
	if !strings.Contains(errb, "现跑预检") {
		t.Errorf("未知 json 字段拒收要点名「现跑预检」，得到 %q", errb)
	}
	// 两态都在任何盘面动作之前拒 —— 真源一个字节都不许建
	if _, err := os.Stat(ledger); !os.IsNotExist(err) {
		t.Errorf("现跑预检拒收不许动真源（%v）", err)
	}

	// 负控：真跑得动的判据放行 ⇒ 0 能进（非样板 verify ＋ 非样板 repro）
	rc, out, errb := gapRun(t, base("现跑预检-放行", "zerg version --json name")...)
	if rc != 0 {
		t.Fatalf("真跑得动的判据要放行 ⇒ 0，得到 %d · %s · %s", rc, out, errb)
	}
	if n := len(gapReadLines(t, ledger)); n != 1 {
		t.Errorf("真判据要落 1 行，得到 %d", n)
	}
}

// ── 判据 ⑧ 判决成对（过 ⇒ 已解 + 证据；判据改坏 ⇒ 回归 + rc=1）──────────────────────────────────

func TestGapFamily_VerifyJudgePairs(t *testing.T) {
	state, ledger, _ := gapEnv(t)
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-判决", "判决"), "--yes")...); rc != 0 {
		t.Fatalf("打底失败：rc=%d %s", rc, errb)
	}
	id := gapFirstID(t)

	// ★ 缺口 `GAP-20260928-195`（本枚）：新账销案须带两态读数（共用闸）—— 带成对读数才放行。
	if rc, out, errb := gapRun(t, "gap", "verify", id, "--yes",
		"--before-read", "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前该号列于仍缺",
		"--after-read", "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后该号列于已解"); rc != 0 {
		t.Fatalf("判据过 ⇒ verify --yes（带两态）要 0，得到 %d · %s%s", rc, out, errb)
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

// ── 卵闭案回执的**生产者** + 检查面**错族点名**（缺口账 `GAP-20260928-184` 第三面 · 2026-10-10）─────
//
// 三格（逐格对派单）：
//
//	正控 —— `zerg gap receipt new` 产出的件 ⇒ `zerg gap receipt check` rc=0（生产者↔检查面同一把尺）；
//	负控① —— 真缺栏的本族件 ⇒ 检查面**仍须红**（判据侧硬必填不动）；
//	负控② —— `dev receipt` 族件（签名键 `round_id`）⇒ 检查面**点名为错族**（「本面不适用」），**不许**报「缺 4 栏」。

// gapReceiptEnv 合成 receipts 目录（`ZERG_RECEIPT_DIR`）+ 合成真源（`gapEnv`）。
func gapReceiptEnv(t *testing.T) (receipts, ledger string) {
	t.Helper()
	receipts = t.TempDir()
	t.Setenv("ZERG_RECEIPT_DIR", receipts)
	_, ledger, _ = gapEnv(t)
	return
}

// gapReceiptAssignedLine 一条 `state=已派` 的真源行（闭案对账要「号在账里且当前 state=已派」）。
func gapReceiptAssignedLine(id string) string {
	return `{"id":"` + id + `","fp":"x","symptom":"s","handmade":"h","impact":"命令面","prio":"P1",` +
		`"state":"已派","want_family":"gap","want_action":"ls","repro_cmd":"x",` +
		`"verify_cmd":"zerg version","found_at":"2026-10-10T00:00:00+08:00"}` + "\n"
}

func TestGapReceiptNewRoundTripThenCheckPasses(t *testing.T) {
	_, ledger := gapReceiptEnv(t)
	const id = "GAP-20261010-900"
	if err := os.WriteFile(ledger, []byte(gapReceiptAssignedLine(id)), 0o644); err != nil {
		t.Fatalf("夹具写真源失败：%v", err)
	}
	// 缺 `--yes` ⇒ fail-closed 2（零写 · 同族 `dev receipt new` 一条纪律）
	if rc, out, _ := gapRun(t, "gap", "receipt", "new", "--egg", "EGG-A", "--reading", id+"=ok", "--conclusion", "收口"); rc != 2 || out != "" {
		t.Fatalf("缺 --yes ⇒ 要 rc=2 + stdout 0 字节，实测 rc=%d out=%q", rc, out)
	}
	// 缺必需旗标 ⇒ 2 且点名
	if rc, _, errb := gapRun(t, "gap", "receipt", "new", "--yes", "--egg", "EGG-A"); rc != 2 || !strings.Contains(errb, "--reading") {
		t.Fatalf("缺 --reading/--conclusion ⇒ 要 rc=2 且点名，实测 rc=%d err=%q", rc, errb)
	}
	// `--reading` 形态不对 ⇒ 2
	if rc, _, _ := gapRun(t, "gap", "receipt", "new", "--yes", "--egg", "E", "--reading", "没有等号", "--conclusion", "c"); rc != 2 {
		t.Fatalf("`--reading` 无 `=` ⇒ 要 rc=2，实测 rc=%d", rc)
	}
	// 真写（`--yes`）
	rc, out, errb := gapRun(t, "gap", "receipt", "new", "--yes",
		"--egg", "EGG-A", "--reading", id+"=ok", "--conclusion", "收口")
	if rc != 0 {
		t.Fatalf("写回执 ⇒ rc=0，实测 rc=%d err=%s", rc, errb)
	}
	p := firstLine(out)
	if !strings.HasPrefix(filepath.Base(p), "GR-") {
		t.Fatalf("文件名前缀应为 `GR-`（与 `dev receipt` 的 `R-` 族区分），实测 %q", p)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("回执没落盘到 %s：%v", p, err)
	}
	// 正控：新生产者产物 ⇒ check rc=0
	rc, out, errb = gapRun(t, "gap", "receipt", "check", p)
	if rc != 0 {
		t.Fatalf("check 新生产者产物 ⇒ rc=0，实测 rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(out, "EGG-A") || !strings.Contains(out, "一一对应") {
		t.Errorf("check 人面没读出卵号 / 对账：%q", out)
	}
	// 逆：`--reading` 空值那一半也不收（`=` 在尾 ⇒ 2）
	if rc, _, _ := gapRun(t, "gap", "receipt", "new", "--yes", "--egg", "E", "--reading", id+"=", "--conclusion", "c"); rc != 2 {
		t.Errorf("`--reading` 结果半为空 ⇒ 要 rc=2，实测 rc=%d", rc)
	}
}

func TestGapReceiptCheckNegativeControls(t *testing.T) {
	receipts, ledger := gapReceiptEnv(t)
	const id = "GAP-20261010-901"
	if err := os.WriteFile(ledger, []byte(gapReceiptAssignedLine(id)), 0o644); err != nil {
		t.Fatalf("夹具写真源失败：%v", err)
	}
	// 负控① 真缺栏（缺 `conclusion` / `closed_ids`）⇒ 仍须红（判据侧硬必填不动）
	missingPath := filepath.Join(receipts, "GR-bad.json")
	if err := os.WriteFile(missingPath, []byte(`{"egg":"E","readings":[{"gap_id":"`+id+`","result":"ok"}]}`+"\n"), 0o644); err != nil {
		t.Fatalf("夹具写缺栏件失败：%v", err)
	}
	rc, out, errb := gapRun(t, "gap", "receipt", "check", missingPath)
	if rc != 2 || out != "" {
		t.Fatalf("负控①：真缺栏 ⇒ 要 rc=2 + stdout 0 字节，实测 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(errb, "缺栏拒收") {
		t.Errorf("负控①：stderr 没报缺栏：%q", errb)
	}
	// 负控② 错族（`dev receipt` 族 · 签名键 `round_id`）⇒ **点名错族**，**不许**报缺栏
	roundPath := filepath.Join(receipts, "R-20261010T000000.json")
	if err := os.WriteFile(roundPath, []byte(`{"round_id":"R-20261010T000000","trace_id":"TR-000000000000","what":"w","next":"n"}`+"\n"), 0o644); err != nil {
		t.Fatalf("夹具写错族件失败：%v", err)
	}
	rc, out, errb = gapRun(t, "gap", "receipt", "check", roundPath)
	if rc != 2 || out != "" {
		t.Fatalf("负控②：错族 ⇒ 要 rc=2 + stdout 0 字节，实测 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(errb, "本面不适用") || !strings.Contains(errb, "轮级交接回执族") {
		t.Errorf("负控②：错族未点名（要「本面不适用」+「轮级交接回执族」）：%q", errb)
	}
	if strings.Contains(errb, "缺栏拒收") || strings.Contains(errb, "缺 4 栏") {
		t.Errorf("负控②：错族被误报成缺栏（本单第三面禁止）：%q", errb)
	}
}

// ── 判据 〔本枚〕GAP-20260927-259：`gap add` 建条时顺带落注（`--note`/`--notes`/`--evidence` 同义）──
//
// 正控：给了旗标 ⇒ 注**逐字**落进真源那一行的 `notes`（机器面 `gap show --json notes` 读得到），
// 且 `state` 一字不动（仍是 `仍缺`）。负控①：裸给（给了旗标没给值）⇒ rc=2 并点名那枚旗标。
// 负控②：加了注旗标**也不放宽**七必填（缺必填仍按既有档拒）。
func TestGapFamily_AddNoteFlags(t *testing.T) {
	_, ledger, _ := gapEnv(t)

	// 正控甲：`--note` ⇒ 一条注 · 逐字落 `notes` · 形态 `<时刻> · <谁>：<文本>`
	rc, out, errb := gapRun(t, append(gapAddArgv("h-注甲", "注甲", "--note", "现读证据 探针读数 rc=0"), "--yes")...)
	if rc != 0 {
		t.Fatalf("`--note` 建条 ⇒ 0，得到 %d · %s%s", rc, out, errb)
	}
	idA := gapIDOf(t, out)
	recA := gapRecordOf(t, ledger, idA)
	nA := gapNotesOf(t, recA)
	if len(nA) != 1 || !strings.Contains(nA[0], "现读证据 探针读数 rc=0") {
		t.Errorf("`--note` 要逐字落进 `notes`，得到 %v", nA)
	}
	if len(nA) == 1 && !strings.Contains(nA[0], "合成夹具") {
		t.Errorf("注形态要含 `<时刻> · <谁>：`（那一格 = ZERG_BY），得到 %q", nA[0])
	}
	if recA["state"] != "仍缺" {
		t.Errorf("落注不改 state，得到 %v", recA["state"])
	}

	// 正控乙：`--notes` 与 `--evidence` 同义 —— 各落一条，机器面读得到
	rc, out, errb = gapRun(t, append(gapAddArgv("h-注乙", "注乙", "--notes", "注二是也", "--evidence", "证据二是也"), "--yes")...)
	if rc != 0 {
		t.Fatalf("`--notes`+`--evidence` 建条 ⇒ 0，得到 %d · %s%s", rc, out, errb)
	}
	idB := gapIDOf(t, out)
	nB := gapNotesOf(t, gapRecordOf(t, ledger, idB))
	if len(nB) != 2 || !strings.Contains(nB[0], "注二是也") || !strings.Contains(nB[1], "证据二是也") {
		t.Errorf("`--notes`/`--evidence` 要各落一条（按固定次序），得到 %v", nB)
	}
	if rc, out, _ := gapRun(t, "gap", "show", idB, "--json", "notes"); rc != 0 ||
		!strings.Contains(out, "注二是也") || !strings.Contains(out, "证据二是也") {
		t.Errorf("机器面 `gap show --json notes` 要读得到两条注，rc=%d out=%q", rc, out)
	}

	// 负控①：裸给 `--note`（给了旗标没给值）⇒ 2 并点名那枚旗标（不许静默降级）
	before := len(gapReadLines(t, ledger))
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-裸给", "裸给", "--note"), "--yes")...); rc != 2 {
		t.Errorf("裸给 `--note` ⇒ 2，得到 %d · %s", rc, errb)
	} else if !strings.Contains(errb, "--note") {
		t.Errorf("裸给要点名那枚旗标 `--note`，得到 %q", errb)
	}
	if n := len(gapReadLines(t, ledger)); n != before {
		t.Errorf("裸给拒收不许动真源：%d → %d", before, n)
	}

	// 负控②：加了注旗标也不放宽七必填（缺 --verify-cmd）⇒ 仍按既有档拒 2
	if rc, _, errb := gapRun(t, "gap", "add", "--symptom", "s", "--handmade", "h", "--impact", "命令面",
		"--want-family", "gap", "--want-action", "ls", "--repro-cmd", "go test ./... -run X",
		"--note", "加了注也不放宽", "--yes"); rc != 2 {
		t.Errorf("缺 --verify-cmd（即使带 --note）⇒ 2（防呆⑤），得到 %d · %s", rc, errb)
	}
}

// gapNotesOf 取真源那一行的 `notes` 数组（原始账行 ⇒ `notes` 是数组，不是渲染串）。
func gapNotesOf(t *testing.T, rec map[string]any) []string {
	t.Helper()
	raw, ok := rec["notes"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, _ := v.(string)
		out = append(out, s)
	}
	return out
}
