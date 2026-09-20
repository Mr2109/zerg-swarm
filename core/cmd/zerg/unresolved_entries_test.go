// unresolved_entries_test.go —— 七个未定入口的**归属取证判据机检**（§7.1 `P13` · §7.2 `U21` · 开工单 T-53）。
//
// 本条立的是「**取证**」这条判据，不是归属 —— 归属要人拍（`P13`）。三条判据逐条落成断言：
//
//	① **7 件各有一行「谁在调我」**，且那一行是**可复核的证据**：每条证据带 `文件:行`，
//	   该行**必须逐字含**入口名 —— 文件没了 / 行号漂了 / 那行不再提这个名字 ⇒ **红**。
//	   （只有「写过一次台账」不叫取证：挂在空气上的台账会随文件漂移悄悄失效。）
//	② **拍板前 7 件一个都不许删**（`P13` 逐字）：7 个入口目录 + 各自的 main.go 必须在盘上。
//	③ **归属未定者不许立命令名**（判据③ 逐字「`zerg trace` 这类名字**先不立命令**」）：
//	   命令树里不许出现以 `compat` / `review` / `scheduler` / `trace` 开头的命令路径。
//
// 负控：判定口 `judgeUnresolved` 是三格判据的**唯一判定口**，负控直接喂它坏台账
// （行号越界 / 那行不含名字 / 入口目录没了 / 命令树里冒出 `trace`）⇒ 每格都必须报错。
package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// repoRootFromCLI —— 仓根：`core/cmd/zerg` 往上三级（与 cli_exec_test.go 找 bin/ 的口径同源）。
func repoRootFromCLI(t *testing.T) string {
	t.Helper()
	if v := strings.TrimSpace(os.Getenv("ZERG_REPO")); v != "" {
		return v
	}
	abs, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("仓根解析不了：%v", err)
	}
	return abs
}

// judgeUnresolved 是本票三格判据的**唯一判定口**。
// 参数一律显式传入（仓根 / 台账 / 命令树）—— 为的是让负控能喂合成件进来，不碰真仓。
func judgeUnresolved(root string, led contract.UnresolvedLedger, cmdPaths []string) []error {
	var errs []error
	if len(led.Entries) == 0 {
		return []error{fmt.Errorf("台账一件都没有（空转 = 假覆盖）")}
	}
	forbidden := map[string]bool{}
	for _, s := range led.ForbiddenCommandSegment {
		forbidden[s] = true
	}
	for _, e := range led.Entries {
		// ② 入口目录 + main.go 都还在（拍板前一个都不许删）
		if st, err := os.Stat(filepath.Join(root, e.Dir)); err != nil || !st.IsDir() {
			errs = append(errs, fmt.Errorf("判据② 破：入口 %s 的目录 %s 不在了（P13 逐字「拍板前一个都不许删」）",
				e.ID, e.Dir))
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Main)); err != nil {
			errs = append(errs, fmt.Errorf("判据② 破：入口 %s 的入口件 %s 不在了", e.ID, e.Main))
		}
		// ① 「谁在调我」必须有一行可复核的证据
		if strings.TrimSpace(e.WhoCalls) == "" {
			errs = append(errs, fmt.Errorf("判据① 破：入口 %s 没有写「谁在调我」那一行", e.ID))
		}
		if len(e.Callers) == 0 {
			errs = append(errs, fmt.Errorf("判据① 破：入口 %s 一条证据行都没有（取证 = 至少一条 文件:行）", e.ID))
		}
		for _, c := range e.Callers {
			b, err := os.ReadFile(filepath.Join(root, c.File))
			if err != nil {
				errs = append(errs, fmt.Errorf("判据① 破：入口 %s 的证据件读不到 %s:%d（%v）", e.ID, c.File, c.Line, err))
				continue
			}
			lines := strings.Split(string(b), "\n")
			if c.Line < 1 || c.Line > len(lines) {
				errs = append(errs, fmt.Errorf("判据① 破：入口 %s 的证据行号越界 %s:%d（该件 %d 行）—— 行号漂了就得重取",
					e.ID, c.File, c.Line, len(lines)))
				continue
			}
			got := lines[c.Line-1]
			if !strings.Contains(got, e.Bin) {
				errs = append(errs, fmt.Errorf("判据① 破：入口 %s 的证据行 %s:%d 里没有 %q（那一行现在是 %q）—— 证据挂在空气上了",
					e.ID, c.File, c.Line, e.Bin, strings.TrimSpace(got)))
			}
		}
	}
	// ③ 归属未定者不许立命令名
	for _, p := range cmdPaths {
		seg := strings.SplitN(p, " ", 2)[0]
		if forbidden[seg] {
			errs = append(errs, fmt.Errorf("判据③ 破：命令树里有 `zerg %s …`（路径 %q）—— 归属未定（P13）前不许立命令名；"+
				"`zerg trace` 这类名字只能先不立", seg, p))
		}
	}
	return errs
}

// ---- 判据 ①②③ 跑真件 ----

func TestUnresolvedEntriesHaveVerifiableEvidence(t *testing.T) {
	root := repoRootFromCLI(t)
	led, err := contract.Unresolved()
	if err != nil {
		t.Fatalf("取出台账读不出来：%v", err)
	}
	if len(led.Entries) != 7 {
		t.Errorf("台账 %d 条（要 7 条：`agent`/`api`/`compat`/`model`/`review`/`scheduler`/`trace`）", len(led.Entries))
	}
	for _, e := range judgeUnresolved(root, *led, zerg.CommandPathsForTest()) {
		t.Error(e)
	}
	// 逐条把「谁在调我」打成一行 —— 这一行就是判据① 的证据面（供记录引用）
	for _, e := range led.Entries {
		t.Logf("%s · %s ⇒ 证据 %d 条 · %s", e.ID, e.Bin, len(e.Callers), e.WhoCalls)
	}
}

// ---- 负控：判定口**真的有牙**（每格一条） ----

func TestUnresolvedJudgeHasTeeth(t *testing.T) {
	root := t.TempDir()
	// 合成一个「入口件在、证据件在」的最小仓：先证它**不报错**（正控），再逐格破坏它
	mustWrite(t, filepath.Join(root, "core/cmd/zerg-trace/main.go"), "package main\n")
	mustWrite(t, filepath.Join(root, "README.md"), "第一行\n| zerg-trace | 追踪 |\n第三行\n")
	good := contract.UnresolvedLedger{
		ForbiddenCommandSegment: []string{"trace"},
		Entries: []contract.UnresolvedEntry{{
			ID: "trace", Dir: "core/cmd/zerg-trace", Main: "core/cmd/zerg-trace/main.go",
			Bin: "zerg-trace", WhoCalls: "文档一行", Verdict: "待拍",
			Callers: []contract.UnresolvedCaller{{File: "README.md", Line: 2, What: "文档登记行"}},
		}},
	}
	if errs := judgeUnresolved(root, good, []string{"task ls"}); len(errs) != 0 {
		t.Fatalf("正控失败：好台账被误判红：%v", errs)
	}
	// 克隆一件「带一条证据行的单件台账」——负控逐格在克隆上改一个字段
	clone := func(caller contract.UnresolvedCaller, dir string) contract.UnresolvedLedger {
		e := good.Entries[0]
		e.Dir = dir
		e.Callers = []contract.UnresolvedCaller{caller}
		return contract.UnresolvedLedger{ForbiddenCommandSegment: good.ForbiddenCommandSegment,
			Entries: []contract.UnresolvedEntry{e}}
	}

	// ①-a 行号越界
	if errs := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "README.md", Line: 99}, good.Entries[0].Dir), nil); len(errs) == 0 {
		t.Error("负控①-a 失败：证据行号越界没有被抓到")
	}

	// ①-b 那一行不含入口名（证据挂在空气上）
	if errs := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "README.md", Line: 1}, good.Entries[0].Dir), nil); len(errs) == 0 {
		t.Error("负控①-b 失败：证据行里没有入口名，没有被抓到")
	}

	// ② 入口目录没了（拍板前删了件）
	if errs := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "README.md", Line: 2}, "core/cmd/zerg-已删"), nil); len(errs) == 0 {
		t.Error("负控② 失败：入口目录被删没有被抓到（P13 逐字「拍板前一个都不许删」）")
	}

	// ③ 命令树里冒出未定名的命令
	if errs := judgeUnresolved(root, good, []string{"trace", "task ls"}); len(errs) == 0 {
		t.Error("负控③ 失败：命令树里 `zerg trace` 没有被抓到")
	}

	// 空转：空台账 ⇒ 不给结论（不许当绿）
	if errs := judgeUnresolved(root, contract.UnresolvedLedger{}, nil); len(errs) == 0 {
		t.Error("负控-空转 失败：空台账被当成绿（空转 = 假覆盖）")
	}
}
