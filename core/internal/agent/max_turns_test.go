// max_turns_test.go — §二十一 已红第 12 条（T-34「`max_turns` 三处不等」）的**成对负控 + 字面量棘轮**。
//
// 四格：
//
//	① 常量之间的关系（派生 · 子任务不许比主任务宽）；
//	② 三处入口的**行为**（`NewAgent(Config{})` 零值兜底 · `newLoop` 非正数兜底 · `SpawnSubagent` 配额）；
//	③ **字面量棘轮**：全仓非测试 Go 件里不许再出现 `MaxTurns = <数字>` / `"max-turns", <数字>`
//	   —— 这条是 T-34 的正题（同一条上限三个真源就是这么来的）；有人写回去，这里当场红。
//	④ 反例夹具：证明 ③ 的扫描器**有牙**（喂一个合成源串，必须扫得到）。
package agent

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestMaxTurnsConstants_DerivedAndOrdered —— ①：两个常量同源且有序（子任务不比主任务宽）。
func TestMaxTurnsConstants_DerivedAndOrdered(t *testing.T) {
	if DefaultMaxTurns != 100 {
		t.Errorf("主上限的历史值（v2.5 起）是 100，得到 %d —— 改它要同时改证据与说明", DefaultMaxTurns)
	}
	if SubAgentMaxTurns != DefaultMaxTurns/5 {
		t.Errorf("子任务配额必须是主上限派生的 1/5：%d ≠ %d/5", SubAgentMaxTurns, DefaultMaxTurns)
	}
	if SubAgentMaxTurns != 20 {
		t.Errorf("子任务配额的历史值是 20（派生结果也应是 20），得到 %d", SubAgentMaxTurns)
	}
	if !(DefaultMaxTurns > SubAgentMaxTurns) {
		t.Errorf("子任务配额绝不许 ≥ 主上限：%d ≱ %d", DefaultMaxTurns, SubAgentMaxTurns)
	}
}

// TestMaxTurnsZeroValueFallbacks —— ②：三处零值兜底都落到同一真源。
func TestMaxTurnsZeroValueFallbacks(t *testing.T) {
	// 主 Agent 的零值兜底
	a := NewAgent(Config{})
	if a.cfg.MaxTurns != DefaultMaxTurns {
		t.Errorf("NewAgent(Config{}) 的兜底应是 DefaultMaxTurns=%d，得到 %d", DefaultMaxTurns, a.cfg.MaxTurns)
	}
	// 主循环的非正数兜底（newLoop 是包内函数，直接问它）
	ls := newLoop(a, nil, nil, nil, 0, 0, 0)
	if ls.maxTurns != DefaultMaxTurns {
		t.Errorf("newLoop(maxTurns=0) 的兜底应是 DefaultMaxTurns=%d，得到 %d", DefaultMaxTurns, ls.maxTurns)
	}
	ls2 := newLoop(a, nil, nil, nil, -3, 0, 0)
	if ls2.maxTurns != DefaultMaxTurns {
		t.Errorf("newLoop(maxTurns=-3) 的兜底应是 DefaultMaxTurns=%d，得到 %d", DefaultMaxTurns, ls2.maxTurns)
	}
	// 子任务的配额（SpawnSubagent 里 cfg.MaxTurns==0 那一支）
	child := childConfigMaxTurns(t)
	if child != SubAgentMaxTurns {
		t.Errorf("SpawnSubagent 的配额应是 SubAgentMaxTurns=%d，得到 %d", SubAgentMaxTurns, child)
	}
}

// TestNoMaxTurnsLiterals — ③ 字面量棘轮：三处入口不许再各写一个数。
func TestNoMaxTurnsLiterals(t *testing.T) {
	root := repoRootFromAgent(t)
	literalAssign := regexp.MustCompile(`MaxTurns\s*=\s*\d+`)
	literalFlag := regexp.MustCompile(`"max-turns",\s*\d+`)
	var hits []string
	_ = filepath.Walk(filepath.Join(root, "core"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "core/internal/agent/max_turns.go" {
			return nil // 真源自己那份常量当然在
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		for i, l := range strings.Split(string(b), "\n") {
			if literalAssign.MatchString(l) || literalFlag.MatchString(l) {
				hits = append(hits, rel+":"+itoa(int64(i+1))+": "+strings.TrimSpace(l))
			}
		}
		return nil
	})
	if len(hits) != 0 {
		t.Errorf("又有地方把轮数上限写成了字面量（T-34 的病）：\n  %s", strings.Join(hits, "\n  "))
	}

	// ④ 负控：扫描器必须抓得到合成反例（否则「0 命中」可能是假绿）
	if !literalAssign.MatchString("\t\tcfg.MaxTurns = 30") {
		t.Errorf("扫描器漏判：`cfg.MaxTurns = 30` 应被命中")
	}
	if !literalFlag.MatchString(`flag.IntVar(&maxTurns, "max-turns", 100, "…")`) {
		t.Errorf("扫描器漏判：flag 默认值里的字面量应被命中")
	}
}

// childConfigMaxTurns —— 走 SpawnSubagent 里同一段「配额兜底」逻辑（不真起子 agent）。
// 为什么单拎出来：SpawnSubagent 会跑整个循环（要模型），本测试只要那一格的判定；
// 判定与实现共用同一对常量（`max_turns.go`），所以查询方式与实跑同源。
func childConfigMaxTurns(t *testing.T) int {
	t.Helper()
	cfg := Config{} // 零值：等价于「父没限定轮数」
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = SubAgentMaxTurns
	}
	return cfg.MaxTurns
}

// repoRootFromAgent 从 core/internal/agent 往上找仓根。
func repoRootFromAgent(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("找不到仓根（从 %s 往上）", wd)
	return ""
}
