package agent

// knobs_test.go — GAP-20260927-232：「新旋钮必须登记」的判据（白名单式 · 正向）。
//
// 判据：扫 `knobsFiles` 里每个件的**顶层** `const` 声明，凡名字**未在 knobs 表登记**、也不在
// `nonKnobConsts` 显式「不是旋钮」名单里的 ⇒ 报错（新增运行旋钮忘了登记 = 测试当场红）。
// ★ 正向白名单判据（未登记即红），不是「未命中就放行」的反向判据。
// ★ 只扫顶层声明（函数内 const 不入面）——本判据管的是「进程级运行旋钮」。
// ★ 覆盖扩展（GAP-20260927-232 leg③ · 第二批 · 2026-10-05）：扫描面由最初 6 件扩到
//   `core/internal/agent` 与 `core/internal/gateway` 两包里**带顶层运行旋钮的全部件**。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// knobsFiles — 扫描面：装有运行旋钮的件（相对 core/internal/agent 的路径；新增一件就加一行）。
var knobsFiles = []string{
	// agent 包
	"scheduler.go", "worker.go", "walkbudget.go", "recovery.go", "docker_worker.go",
	"bash_v101.go", "max_turns.go", "memory.go", "net_tools.go",
	// gateway 包
	"../gateway/gateway.go", "../gateway/compactor.go", "../gateway/ds4_route.go",
	"../gateway/egg_warmup.go", "../gateway/ledger_route.go", "../gateway/outbudget.go",
	"../gateway/prefix_cache.go", "../gateway/sse_usage_tee.go", "../gateway/tokencap.go",
}

// nonKnobConsts — 显式登记为「不是运行旋钮」的常量（枚举/静态定义/文案——改了不涉及运行调节）。
var nonKnobConsts = map[string]bool{
	// agent：优先级/退出码枚举
	"PriorityLow": true, "PriorityNormal": true, "PriorityHigh": true,
	"ExitSuccess": true, "ExitFailed": true, "ExitError": true,
	// agent：issue 状态机字符串常量（枚举语义）
	"statusOpen": true, "statusRunning": true, "statusFixing": true, "statusVerified": true,
	"statusDone": true, "statusFailed": true, "statusRetry": true, "statusQueued": true,
	// gateway：字符串常量/文案（非旋钮）
	"CompositeModelName": true, "breakerReasonUnspecified": true, "breakerNoReasonText": true,
	// agent：溢出目录/缓存文件名/遗留路径常量（非旋钮）
	"bashOverflowDirName": true, "bashOverflowLegacyDefaultDir": true,
	"searchCacheFileName": true, "searchCacheLegacyDefaultPath": true,
	// gateway：模型名/状态枚举/状态文件名/序列化版本常量（非旋钮）
	"ds4ModelName": true, "ds4NotLoaded": true, "ds4Idle": true, "ds4Busy": true,
	"prefixCacheStateFile": true, "persistedPrefixCacheVersion": true,
}

// knobNameRegistered — 旋钮名以 `<面>.<常量名>` 形态登记 ⇒ 后缀匹配常量名。
func knobNameRegistered(registered map[string]bool, name string) bool {
	for k := range registered {
		if k == name || strings.HasSuffix(k, "."+name) {
			return true
		}
	}
	return false
}

func TestKnobsRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, k := range knobs {
		registered[k.Name] = true
	}
	fset := token.NewFileSet()
	for _, f := range knobsFiles {
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", f, err)
		}
		for _, d := range af.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, s := range gd.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, n := range vs.Names {
					if nonKnobConsts[n.Name] {
						continue
					}
					if !knobNameRegistered(registered, n.Name) {
						t.Errorf("常量 %s（%s）未在 knobs.go 登记 —— 新旋钮必须登记（GAP-20260927-232）", n.Name, f)
					}
				}
			}
		}
	}
}
