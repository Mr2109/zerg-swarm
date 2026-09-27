// Q-218（2026-09-26 Mr2109 拍板「合并而非覆盖」）的用例：
//   - 正控：两侧**异名**参数 ⇒ 都出现在 argv ✓
//   - 负控：两侧**同名**参数 ⇒ 只留一个 ✓ 且留的是**卵清单**那一侧 ✓
//   - 三枚真模型的前后对照（读本机卵清单现读；清单不在 ⇒ 跳过，绝不编造）
//
// 「前」= 改前的语义（**整段覆盖**），逐字复刻 `.bak-q218-20260926` 的 hatch_spec.go:787–797：
// 卵清单非空 ⇒ cmdArgs = strings.Fields(entry.Cmd)（适配器参数整段丢弃）。
package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/modeladapter"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// oldCoverArgv 改前语义（整段覆盖）的最小复刻——只用于本文件的**对照表**，不参与生产路径。
func oldCoverArgv(entry *registry.ModelEntry, port int) []string {
	if entry.Cmd == "" {
		return nil
	}
	args := strings.Fields(string(entry.Cmd))
	replaceArgvPlaceholders(args, entry.File, port)
	return args
}

func countToken(args []string, tok string) int {
	n := 0
	for _, a := range args {
		if argKey(a) == tok {
			n++
		}
	}
	return n
}

func flagValue(args []string, flag string) (string, bool) {
	for i, a := range args {
		if argKey(a) == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// TestQ218_MergePositiveControl 两侧**异名**参数 ⇒ 两侧都在（合并 = 并存，不是覆盖）。
func TestQ218_MergePositiveControl(t *testing.T) {
	adapter := []string{"-m", "/w/x.gguf", "-c", "0", "-ngl", "999", "-ctk", "q8_0", "-fa", "on", "--cache-prompt", "-cb"}
	egg := []string{"-m", "/w/x.gguf", "-c", "65536", "-ngl", "999", "--reasoning-budget", "256", "--host", "127.0.0.1"}
	got := mergeEngineArgs(adapter, egg)
	joined := strings.Join(got, " ")
	// 适配器独有（卵清单没发）⇒ 必须补缺进来
	for _, want := range []string{"-ctk q8_0", "-fa on", "--cache-prompt", "-cb"} {
		if !strings.Contains(joined, want) {
			t.Errorf("正控：适配器独有参数 %q 丢了（合并应补缺）：%v", want, got)
		}
	}
	// 卵清单独有 ⇒ 必须在
	for _, want := range []string{"--reasoning-budget 256", "--host 127.0.0.1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("正控：卵清单独有参数 %q 丢了：%v", want, got)
		}
	}
}

// TestQ218_MergeNegativeControl 两侧**同名**参数 ⇒ 只留一个，且是**卵清单**那一侧（写死的优先级）。
func TestQ218_MergeNegativeControl(t *testing.T) {
	adapter := []string{"-m", "/w/x.gguf", "-c", "32768", "-ngl", "999"}
	egg := []string{"-m", "/w/x.gguf", "-c", "131072"}
	got := mergeEngineArgs(adapter, egg)

	if n := countToken(got, "-c"); n != 1 {
		t.Errorf("负控：同名键 -c 应只留一个，实得 %d 个：%v", n, got)
	}
	if v, ok := flagValue(got, "-c"); !ok || v != "131072" {
		t.Errorf("负控：同名键 -c 应取**卵清单**侧的 131072，实得 %q（ok=%v）：%v", v, ok, got)
	}
	if strings.Contains(strings.Join(got, " "), "32768") {
		t.Errorf("负控：适配器侧的 -c 32768 不该出现：%v", got)
	}
	// 异名键照旧保留（负控只对同名键收紧）
	if v, ok := flagValue(got, "-ngl"); !ok || v != "999" {
		t.Errorf("负控：异名键 -ngl 999 应保留，实得 %q（ok=%v）：%v", v, ok, got)
	}
}

// TestQ218_ThreeRealModelsBeforeAfter 三枚入库模型的 argv 前/后对照（卵清单现读）。
// 判据（每条都可判定）：
//
//		① 可执行文件 = 卵清单首词（wrapper 不许被适配器改写）；
//		② 同名键（-c/-ngl/-m/--host/--port）取卵清单值 —— 即「运行参数不动」；
//		③ 适配器独有键补缺进来（-ctk/-ctv/-fa/--cache-prompt/-cb）；
//	 ④ `-np` 的来源必须**可判定**：example-35b-v2 系两侧都不发 ⇒ 不出现（槽数维持 auto／实测 4 槽 ✓）；
//	    gemma 系适配器**发** `-np 1`，但卵清单没显式写 ⇒ 按 Q-218 `-np` 例外**摘掉**，同样不出现
//	    ✓（槽数维持 auto／实测 4 槽）。「适配器发过」这件事由下面三条 `-np` 例外用例另行佐证。
func TestQ218_ThreeRealModelsBeforeAfter(t *testing.T) {
	regPath := filepath.Join(os.Getenv("HOME"), "agent", "agent_models.yaml")
	if _, err := os.Stat(regPath); err != nil {
		t.Skipf("本机卵清单不在（%s）：跳过真模型对照（不编造）", regPath)
	}
	reg, err := registry.New(regPath)
	if err != nil {
		t.Fatalf("加载卵清单失败: %v", err)
	}
	const port = 9400
	// 按现读（2026-09-26 · `-np` 例外落地后）：三枚卵都没显式写 `-np` ⇒ argv 里**都不出现**
	// （example-35b-v2 适配器本就不发；gemma 适配器发的 `-np 1` 被例外摘掉）。「适配器发过」由本文件
	// TestQ218_AdapterNP* 三条用例佐证 —— 若 gemma 适配器哪天不再发 `-np`，那三条会先红。
	wantNP := map[string]int{"example-35b-v2-1.5-9b": 0, "example-35b": 0, "gemma-4-26B": 0}
	for _, name := range []string{"example-35b-v2-1.5-9b", "example-35b", "gemma-4-26B"} {
		entry, ok := reg.Get(name)
		if !ok {
			t.Errorf("卵清单里没有 %q —— 对照表要的模型缺了（清单形状变了？）", name)
			continue
		}
		before := oldCoverArgv(entry, port)
		exe, after := buildEngineArgv(name, entry, port)
		t.Logf("\n── %s\n  前(整段覆盖) : %s\n  后(合并)     : %s", name, strings.Join(before, " "), strings.Join(append([]string{exe}, after...), " "))

		eggExe := strings.Fields(string(entry.Cmd))[0]
		if exe != eggExe {
			t.Errorf("[%s] 可执行文件应由卵清单定（%q），实得 %q", name, eggExe, exe)
		}
		eggFields := strings.Fields(string(entry.Cmd))
		replaceArgvPlaceholders(eggFields, entry.File, port) // 期望值按同一口径（占位符已替换）
		for _, flag := range []string{"-c", "-ngl", "-m", "--host", "--port"} {
			want, wok := flagValue(eggFields, flag)
			got, gok := flagValue(after, flag)
			if !wok {
				continue
			}
			if !gok || got != want {
				t.Errorf("[%s] 同名键 %s 应取卵清单值 %q，实得 %q（ok=%v）", name, flag, want, got, gok)
			}
			if n := countToken(after, flag); n != 1 {
				t.Errorf("[%s] 同名键 %s 应只留一个，实得 %d 个：%v", name, flag, n, after)
			}
		}
		for _, flag := range []string{"-ctk", "-ctv", "-fa", "--cache-prompt", "-cb"} {
			if countToken(after, flag) == 0 {
				t.Errorf("[%s] 适配器独有键 %s 应补缺进来：%v", name, flag, after)
			}
		}
		if n := countToken(after, "-np"); n != wantNP[name] {
			t.Errorf("[%s] -np 出现 %d 次，按现读应为 %d 次（卵清单没显式写 -np ⇒ 适配器那份被例外摘掉）：%v",
				name, n, wantNP[name], after)
		}
	}
}

// ═══ Q-218 `-np` 例外（2026-09-26 父代理拍：`-np` 只认卵清单**显式声明**）═══════════════
//
// 三条用例全部走 **buildEngineArgv**（判据落在真 argv 上，不是 mergeEngineArgs 的中间态）；
// 模型名固定 `gemma-4-26B` —— gemma 适配器现读**确实**发 `-np 1`（每条用例开头先验证这一点，
// 否则例外测的就是空气），而 example-35b-v2 适配器压根不发 `-np` ⇒ 用它测不出摘除动作。

// newNPTestEntry 造一枚 llama 系卵（只为 `-np` 例外服务：file 是占位符落点，backend 走主线引擎）。
func newNPTestEntry(cmd string) *registry.ModelEntry {
	return &registry.ModelEntry{
		File:    "/w/np-test.gguf",
		Backend: "llama.cpp",
		Cmd:     registry.CmdString(cmd),
	}
}

// requireAdapterEmitsNP 佐证「gemma 适配器确实发 `-np 1`」—— 否则下面的「argv 里没有 -np」可能
// 只是适配器本来就没发（假绿）。适配器哪天改了这份默认，本断言先红，提醒回来重审本例外。
func requireAdapterEmitsNP(t *testing.T, entry *registry.ModelEntry) {
	t.Helper()
	adapter := modeladapter.Dispatch("gemma-4-26B").BuildArgs(entry, 9400)
	if v, ok := flagValue(adapter, "-np"); !ok || v != "1" {
		t.Fatalf("前置失效：gemma 适配器现读应发 `-np 1`，实得 %q（ok=%v）—— `-np` 例外的前提变了", v, ok)
	}
}

// TestQ218_AdapterNPNotInjectedWhenEggOmitsIt 正控①：卵清单写了 `cmd:` 但**没写** `-np`
// ⇒ 合并后 argv 里**不得**出现 `-np`（适配器的单槽铁律不许注入；槽数维持引擎缺省／实测 4 槽）。
func TestQ218_AdapterNPNotInjectedWhenEggOmitsIt(t *testing.T) {
	entry := newNPTestEntry("llama-server -m {file} -c 131072 -ngl 999 --host 127.0.0.1 --port {port}")
	requireAdapterEmitsNP(t, entry)

	_, argv := buildEngineArgv("gemma-4-26B", entry, 9400)
	if n := countToken(argv, "-np"); n != 0 {
		t.Errorf("正控①：卵清单没写 -np ⇒ argv 里不该有 -np，实得 %d 次：%v", n, argv)
	}
	for _, a := range argv {
		if strings.HasPrefix(a, "-np") {
			t.Errorf("正控①：argv 里出现 -np 形态的 token %q：%v", a, argv)
		}
	}
	// 例外**只**作用于 `-np` 一个键：适配器其余参数照旧补缺（否则这条就变成「合并失效」的假修法）
	if v, ok := flagValue(argv, "-ctk"); !ok || v != "q8_0" {
		t.Errorf("正控①：-np 之外适配器参数应照旧补缺，-ctk 实得 %q（ok=%v）：%v", v, ok, argv)
	}
	if countToken(argv, "-fa") == 0 || countToken(argv, "-cb") == 0 {
		t.Errorf("正控①：适配器独有键 -fa/-cb 应补缺进来：%v", argv)
	}
	// 卵清单侧的槽数声明缺失 = 沿用引擎缺省 ⇒ 也不该由别的旗标替它说话
	if v, ok := flagValue(argv, "-c"); !ok || v != "131072" {
		t.Errorf("正控①：同名键 -c 应取卵清单值 131072，实得 %q（ok=%v）：%v", v, ok, argv)
	}
}

// TestQ218_AdapterNPKeptWhenEggCmdEmpty 正控②：卵清单 `cmd:` 为空 ⇒ 适配器是**唯一真源**
// ⇒ argv 里必须出现 `-np 1`（例外不许把这一情形也摘了）。
func TestQ218_AdapterNPKeptWhenEggCmdEmpty(t *testing.T) {
	entry := newNPTestEntry("") // cmd: 为空 ⇒ 不经过 mergeEngineArgs，走适配器参数
	requireAdapterEmitsNP(t, entry)

	_, argv := buildEngineArgv("gemma-4-26B", entry, 9400)
	if n := countToken(argv, "-np"); n != 1 {
		t.Fatalf("正控②：cmd: 为空 ⇒ argv 里应恰好 1 个 -np，实得 %d 次：%v", n, argv)
	}
	if v, ok := flagValue(argv, "-np"); !ok || v != "1" {
		t.Errorf("正控②：cmd: 为空 ⇒ -np 应为适配器的 1，实得 %q（ok=%v）：%v", v, ok, argv)
	}
}

// TestQ218_EggExplicitNPSurvivesAdapter 负控③：卵清单**显式**写 `-np 4` ⇒ argv 里只 1 个 `-np`
// 且值 = 4（适配器的 `-np 1` 不许覆盖它）—— 即「槽数只认卵清单显式声明」的另一半。
func TestQ218_EggExplicitNPSurvivesAdapter(t *testing.T) {
	entry := newNPTestEntry("llama-server -m {file} -c 131072 -np 4 --host 127.0.0.1 --port {port}")
	requireAdapterEmitsNP(t, entry)

	_, argv := buildEngineArgv("gemma-4-26B", entry, 9400)
	if n := countToken(argv, "-np"); n != 1 {
		t.Errorf("负控③：卵清单显式 -np 4 ⇒ 应只留 1 个 -np，实得 %d 次：%v", n, argv)
	}
	if v, ok := flagValue(argv, "-np"); !ok || v != "4" {
		t.Errorf("负控③：卵清单显式值 4 不许被适配器覆盖，实得 %q（ok=%v）：%v", v, ok, argv)
	}
	if strings.Contains(strings.Join(argv, " "), "-np 1") {
		t.Errorf("负控③：适配器的 `-np 1` 不该出现：%v", argv)
	}
	// 等号形态（`-np=4`）同样算「显式声明」⇒ 不该再补一个 `-np 1` 进来（hasExplicitNP 归一口径）
	entryEq := newNPTestEntry("llama-server -m {file} -c 131072 -np=4 --host 127.0.0.1 --port {port}")
	_, argvEq := buildEngineArgv("gemma-4-26B", entryEq, 9400)
	if n := countToken(argvEq, "-np"); n != 1 {
		t.Errorf("负控③：`-np=4` 也算显式声明 ⇒ 应只 1 个 -np，实得 %d 次：%v", n, argvEq)
	}
	if !strings.Contains(strings.Join(argvEq, " "), "-np=4") {
		t.Errorf("负控③：`-np=4` 原文应保留：%v", argvEq)
	}
	if countToken(argvEq, "-fa") == 0 {
		t.Errorf("负控③：适配器其余参数仍应补缺：%v", argvEq)
	}
}
