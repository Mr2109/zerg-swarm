// check_flag_registry_parity_test.go —— Go 真搬的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckFlagRegistryParityGatePort（定向：go test -run TestCheckFlagRegistryParityGatePort）·
// TestFlagRegistryParityGoldenInternalized（内化金标：旧 py 对拍退场 ⇒ 逐格金标收成 Go 常量 · ✗ 不 Skip）·
// TestFlagRegistryParityGoldenTwoStates（两态逐字同：夹具无 py ⟷ 合成旧态有 py）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frpFixtureTree —— 在 t.TempDir() 下造一棵含 `core/cmd/zerg/main.go` 的夹具（仓外）。
func frpFixtureTree(t *testing.T, mainBody, extraBody string, withExtra bool) string {
	t.Helper()
	root := t.TempDir()
	d := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "main.go"), []byte(mainBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if withExtra {
		if err := os.WriteFile(filepath.Join(d, "family_probe.go"), []byte(extraBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func frpRunEntry(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := flagRegistryParityGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckFlagRegistryParityGatePort(t *testing.T) {
	// ── 正控（必然判绿）：命令树两面（除存量豁免）对齐 ─────────────────────
	green := frpFixtureTree(t, frpFixtureMain, frpFixtureExtra, true)
	rcGreen, outGreen, _ := frpRunEntry(t, "--root", green)
	if rcGreen != 0 || !strings.Contains(outGreen, "新分家项 0 项") {
		t.Fatalf("正控失败：对齐件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控①（必然判红）：名字表删 --desc ⇒ 「缺一处」点名 ────────────────
	missingBody := strings.Replace(frpFixtureMain,
		"case \"--desc\", \"--slice-id\":", "case \"--slice-id\":", 1)
	rcMiss, outMiss, _ := frpRunEntry(t, "--root",
		frpFixtureTree(t, missingBody, frpFixtureExtra, true))
	if rcMiss != 1 || !strings.Contains(outMiss, "缺一处") || !strings.Contains(outMiss, "--desc") {
		t.Fatalf("负控-缺一处 失败：得 rc=%d out=%q", rcMiss, outMiss)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcMiss {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcMiss)
	}

	// ── 负控②（必然判红）：名字表加一枚无人写的名 ⇒ 「多一处」 ─────────────
	extraBody := strings.Replace(frpFixtureMain,
		"case \"--desc\", \"--slice-id\":", "case \"--desc\", \"--slice-id\", \"--zz-new\":", 1)
	rcGhost, outGhost, _ := frpRunEntry(t, "--root",
		frpFixtureTree(t, extraBody, frpFixtureExtra, true))
	if rcGhost != 1 || !strings.Contains(outGhost, "多一处") || !strings.Contains(outGhost, "--zz-new") {
		t.Fatalf("负控-多一处 失败：得 rc=%d out=%q", rcGhost, outGhost)
	}

	// ── 负控③（必然判红）：用法串取值改裸形 ⇒ 「形态」 ────────────────────
	formBody := strings.Replace(frpFixtureMain, "--slice-id <片>", "--slice-id", 1)
	rcForm, outForm, _ := frpRunEntry(t, "--root",
		frpFixtureTree(t, formBody, frpFixtureExtra, true))
	if rcForm != 1 || !strings.Contains(outForm, "形态") || !strings.Contains(outForm, "--slice-id") {
		t.Fatalf("负控-形态 失败：得 rc=%d out=%q", rcForm, outForm)
	}

	// ── 负控④（必然判红 · 面⑦ 声明面 ⇔ 用法串 · **进退码**）：合成一条「声明了机器目标、
	//    而用法串不写 `--node`」的命令 ⇒ 必红且**点名 --node**（任务单 波1 片2 的主病那一类）。
	declBody := strings.Replace(frpFixtureMain,
		"			passthrough: true,\n		},\n	}\n}",
		"			passthrough: true,\n		},\n"+
			"		{\n			path:  []string{\"probe\", \"decl\"},\n"+
			"			usage: \"zerg probe decl\",\n"+
			"			machine: machineNodeName,\n"+
			"			run:   cmdProbeDecl,\n		},\n	}\n}", 1)
	rcDecl, outDecl, _ := frpRunEntry(t, "--root",
		frpFixtureTree(t, declBody, frpFixtureExtra, true))
	if rcDecl != 1 || !strings.Contains(outDecl, "声明未列") || !strings.Contains(outDecl, "--node") {
		t.Fatalf("负控-声明未列(机器面) 失败：得 rc=%d out=%q", rcDecl, outDecl)
	}
	if rcGreen == rcDecl {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控⑦ rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcDecl)
	}

	// ── 不给结论（≠ 绿）：命令树零条 ⇒ rc=2 ──────────────────────────────
	rcBail, outBail, _ := frpRunEntry(t, "--root",
		frpFixtureTree(t, frpEmptyMain, "", false))
	if rcBail != 2 || !strings.Contains(outBail, "判不了") {
		t.Fatalf("空转/零条失败：得 rc=%d out=%q", rcBail, outBail)
	}

	// ── 取不到输入：根路径不存在 ⇒ rc=2 ──────────────────────────────────
	rcNoRoot, outNoRoot, _ := frpRunEntry(t, "--root", filepath.Join(t.TempDir(), "nope"))
	if rcNoRoot != 2 || !strings.Contains(outNoRoot, "包目录不在盘") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNoRoot, outNoRoot)
	}

	// ── 未知旗标（argparse 用法错）⇒ rc=2 · stderr 明说 ──────────────────
	rcFlag, _, errFlag := frpRunEntry(t, "--no-such-flag")
	if rcFlag != 2 || !strings.Contains(errFlag, "unrecognized arguments: --no-such-flag") {
		t.Fatalf("未知旗标失败：得 rc=%d stderr=%q", rcFlag, errFlag)
	}

	// ── `--json`（真仓机器面一行）⇒ rc=0 且尾行是 JSON ────────────────────
	repo := filepath.Join(t.TempDir(), "repo")
	_ = os.MkdirAll(filepath.Join(repo, "core", "cmd", "zerg"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, "core", "cmd", "zerg", "main.go"), []byte(frpFixtureMain), 0o644)
	rcJSON, outJSON, _ := frpRunEntry(t, "--root", repo, "--json")
	if rcJSON != 0 || !strings.Contains(outJSON, "\"gate\": \"flag-registry-parity\"") {
		t.Fatalf("--json 失败：得 rc=%d out=%q", rcJSON, outJSON)
	}

	// ── `-h`（argparse 帮助正文）⇒ rc=0 ──────────────────────────────────
	rcHelp, outHelp, _ := frpRunEntry(t, "-h")
	if rcHelp != 0 || !strings.Contains(outHelp, "usage: check-flag-registry-parity.py") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcHelp, outHelp)
	}

	// ── `--self-test` 十二格自身必须全过（防恒绿/恒红） ───────────────────
	rcST, outST, _ := frpRunEntry(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "失败 0 条") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
	// ── 面④ 读者面（按命令 · 缺口账 `GAP-20260927-325`）在报告里可见（报告档 · 不进退码） ──
	if !strings.Contains(outST, "④读者面") {
		t.Fatalf("--self-test 输出里没有读者面两格：%q", outST)
	}
	// ── 面⑥ 运行期用法行面（按命令 · 缺口账 `GAP-20260928-183`）在报告里可见（报告档 · 不进退码） ──
	if !strings.Contains(outST, "⑥运行期用法行面") {
		t.Fatalf("--self-test 输出里没有运行期用法行面两格：%q", outST)
	}
}

// ── 哨兵（锚 Go 半 · fail-closed）─────────────────────────────────────────────

// frpGoHalfRel —— 哨兵锚：Go 半（✗ 不锚已退场的 py —— 一退役即失锚）。
const frpGoHalfRel = "core/cmd/zerg/check_flag_registry_parity.go"

// frpTestRepoRoot —— 哨兵：由 cwd 上溯找含 Go 半的仓根；找不到 ⇒ fail-closed（缺件 ≠ 绿 · ✗ 不留 Skip）。
// 旧版锚 `scripts/gates/check-flag-registry-parity.py`（对拍真身）：该 py 已退役 ⇒ 上溯恒找不到 ⇒
// 旧 `TestFlagRegistryParityGoPyDifferential` 恒 Skip（假绿、零保护）；跳因「不在真仓内」名不副实
// （整测零断言）。改锚 Go 半后它在盘（与本测试同包）⇒ 真跑断言。
func frpTestRepoRoot(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if st, err := os.Stat(filepath.Join(d, filepath.FromSlash(frpGoHalfRel))); err == nil && st.Mode().IsRegular() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	t.Fatalf("哨兵失守：上溯未见 Go 半 %s（不是真仓树）⇒ fail-closed（✗ 不留 Skip）", frpGoHalfRel)
	return ""
}

// ── 内化金标：旧 py 对拍的逐格金标收成 Go 常量（现算现冻 · ✗ 不臆造）────────────────
//
// 为什么这么测（M2 口径 · 与 `atomic_write_test.go` 的 `TestAwTwoStatesGoldenInternalized` /
// `check_manifest_freshness_test.go` 的 `TestCheckManifestFreshnessGoldenInternalized` /
// `check_verify_cmd_real_test.go` 的 `TestCheckVerifyCmdRealGoldenInternalized` 同族）：本件原
// 「py 真身 ⟷ Go 真搬」逐字对拍（旧名 `TestFlagRegistryParityGoPyDifferential`）的对拍源
// `scripts/gates/check-flag-registry-parity.py` 已退场（M2 批）⇒ 对拍源消失、断言恒 Skip（假绿、
// 零保护）。照 `atomic_write_test.go`（体例最新）＋ `check_verify_cmd_real_test.go` 把金标真源
// **收回 Go 半现役行为本身**：
//
//	★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：
//	  逐格「合成夹具 + argv ⇒ (rc, stdout, stderr)」由 `flagRegistryParityGateMain` 现跑捕获
//	  （一次性现算 · 只读夹具 · ✗ 不落仓），冻成下表 wantRC/wantOut/wantErr；**本机位置类量**在
//	  比对前归一化（✗ 不冻结 —— 因其本非常量）：
//	    ① 夹具根 `t.TempDir()`（随机路径 + macOS /var↔/private/var）⇒ `<FX>`；
//	    ② `$TMPDIR` ⇒ `<TMP>`。
//
// 两态（✗ 不许 Skip）：A 态 = 夹具无 py（真仓现态）；B 态 = 合成「旧态」夹具（把退役 py 路径
// `scripts/gates/check-flag-registry-parity.py` 放回）⇒ 两态对同一套金标逐字同命中，证 py
// 在/不在盘都不改判据结论（内化彻底 · 将来若有人让 Go 半重新去读 py，B 态当场分家）。
//
// ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 `check_flag_registry_parity.go` 的一臂哑化
// ⇒ 金标用例必 FAIL 且带行号（✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。

type frpGoldenCase struct {
	name    string
	argv    func(fx string) []string
	wantRC  int
	wantOut string // 归一化后（frpNormGolden 现算自证 · ✗ 手写）
	wantErr string
}

// frpGoldenCases —— 金标真源（6 格：原对拍六组 argv 逐格同 · want* 由现跑捕获）。
func frpGoldenCases() []frpGoldenCase {
	return []frpGoldenCase{
		{name: "self_test", argv: func(string) []string { return []string{"--self-test"} }, wantRC: 0, wantOut: "门73 自检（成对负控 · 合成夹具）：断言 24 条 · 失败 0 条\n  PASS ①对齐 ⇒ 无分家项\n  PASS ①' 透传命令的旗标不进判红（--scope 不在解析面）\n  PASS ②缺一处（表里删 --desc）⇒ 判红且**点名 --desc**\n  PASS ③多一处（表里加一枚没人写的名）⇒ 判红且**点名 --zz-new**\n  PASS ④形态（用法串取值改裸形）⇒ 判红且**点名 --slice-id**\n  PASS ⑤命令树真源不在盘 ⇒ 判不了（Bail ⇒ rc=2）\n  PASS ⑥命令树零条 ⇒ 判不了（不给结论）\n  PASS ⑦注释里的旗标名不被读成登记（面② 不含 --zzz-注释）\n  PASS ⑧豁免行必须带原因（EXEMPT 逐行有值）\n  PASS ⑨判据只有一处定义（本件内 RULES 与三档退码常量在位）\n  PASS ④读者面（负控）：run 指不到实现件 ⇒ 非全局 usage 旗标列缺读者\n  PASS ④读者面（正控）：实现件读全 ⇒ 该命令不再列缺读者\n  PASS ⑤反读面（负控）：实现按取值读但 usage 没列 ⇒ 列出该命令\n  PASS ⑤反读面（正控）：usage 列了该旗标 ⇒ 该命令不再列\n  PASS ⑥运行期用法行面（负控）：运行期用法行漏列命令树旗标（--zz-b）⇒ 点名该命令\n  PASS ⑥运行期用法行面（正控）：运行期用法行把命令树旗标列全 ⇒ 该命令不再列\n  PASS ⑦声明面（负控 · 机器面）：`machine:` 声明了而用法串没写 `--node` ⇒ 判红且**点名 --node**\n  PASS ⑦声明面（正控 · 机器面）：用法串写了 `--node` ⇒ 不再列\n  PASS ⑦声明面（负控 · 字段面）：`fields:` 非空而用法串没写 `--json` ⇒ 判红且**点名 --json**\n  PASS ⑦声明面（正控 · 字段面）：用法串写了 `--json` ⇒ 不再列\n  PASS ⑦声明面（负控 · 危险档面）：`danger:` 非空而用法串两枚都没有 ⇒ 判红且**点名 --dry-run|--yes**\n  PASS ⑦声明面（正控 · 危险档面）：用法串写了 `--dry-run` ⇒ 不再列\n  PASS ⑩棘轮（正控 · 面④读者面）：读者面项全在冻结表里 ⇒ 不判红、计入存量豁免\n  PASS ⑪棘轮（负控 · 面④读者面）：冻结表漏收一项（--slice-id）⇒ 判红且**点名**\n"},
		{name: "list_rules", argv: func(string) []string { return []string{"--list-rules"} }, wantRC: 0, wantOut: "面① 用法串 = 命令树 usage:（非字面量按包内常量名现读）；面② 名字表 = valueFlagName 体；面③ 解析面 = valueFlagName ∪ parseInvocation 专用 case。甲 缺一处 = 用法串取值形态的旗标 ∉ 面③（透传型命令不计）；乙 多一处 = 面② 的名字在全部用法串里零提及；丙 形态 = 面② 的名字在用法串里只写裸形。 存量豁免表逐条带原因（棘轮 · 只许下调）；新增分家项 ⇒ rc=1（面④/⑤ 同规：现读存量逐条带 reason 冻表 · 新增即红且点名 · 删即绿）。面④ 读者面（按命令 · 棘轮 · 进退码）= 某命令 usage 串里的旗标 ⇒ 该命令的实现件（定义其 run: 处理函数的包内 .go 件 · 先剥注释）必读它（字面 \"--F\" 或类型化字段 inv.<field>）—— 缺口账 `GAP-20260927-325`（全局「零读者」是错的判据）。面⑤ 反读面（按命令 · 棘轮 · 进退码）= 某命令实现按**取值**读了（`inv.flagVal`）某旗标 ⇒ 该命令的 usage 串必列它（面④ 的另一向 —— 「实现读 ⇒ 用法必列」，缺口 `GAP-20261005-15`）；拒执守卫（同一函数里 `inv.flagVals`/`inv.hasFlag` 存在性检查）与别处已列 不计。面⑥ 运行期用法行面（按命令 · 报告档 · 不进退码）= 件内运行期 stderr 用法行的旗标集 ⟷ 面① 命令树同命令用法串旗标集（双向对拍 · 全局选项不计 · 命令需精确同名）—— 差集非空逐条列（缺口 `GAP-20260928-183`）。面⑦ 声明面 ⇔ 用法串（按命令 · **进退码** · **零豁免** · 双向）= 把命令树当**单一真源**，取该命令**声明面**（`machine:` / `fields:` / `danger:`）里由实现**证明了的**消费旗标 ⇒ 断言「声明面推得的必列旗标 ⇔ 该命令用法串的旗标」双向零差集：甲 **声明未列** = `machine:` 声明对象是某台机、用法串又没给位置参数机器名 ⇒ 必列 `--node <机>`（`remote.go` 的 `enforceNodeRules` 真跑无目标 ⇒ rc=2 · 治「`build all --only cli` 必带 `--node` 而用法串未列」这一类）；`fields:` 非空 ⇒ 必列 `--json`（`main.go` 收 `--json` 的真条件）；`danger:` 非空 ⇒ 必列 `--dry-run` 或 `--yes`（危险档三态：`--dry-run` 出计划件 / 真跑要 `--yes`＋`--confirm`）；乙 **用法未注册** = 面甲的同一向（用法串取值形态的旗标 ∉ 解析面）。★ 零豁免表（✗ 不放宽）：真漂就逐条报红、**不许**借豁免转绿 ⇒ rc=1。\n退码：0=对齐 · 1=有新的分家项（逐条点名）· 2=取数面拿不到（不给结论）\n存量豁免 66 项：④读者面/--allow-stale-gates@repo/commit · ④读者面/--by@gap/assign · ④读者面/--by@gap/bulk/set-state · ④读者面/--by@gap/note · ④读者面/--by@gap/project · ④读者面/--by@gap/set-state · ④读者面/--by@gap/set-unit · ④读者面/--by@gap/unproject · ④读者面/--by@gap/verify-one · ④读者面/--candidate@dev/release · ④读者面/--context@fleet/render · ④读者面/--dir@gate/results · ④读者面/--dist@dev/build · ④读者面/--human-approval@dev/verify · ④读者面/--last@gate/results · ④读者面/--no-fast@repo/commit · ④读者面/--node@agent/reap · ④读者面/--node@build/all · ④读者面/--node@build/release · ④读者面/--node@core/restart · ④读者面/--node@core/start · ④读者面/--node@core/stop · ④读者面/--node@core/update · ④读者面/--node@publish/run · ④读者面/--node@update · ④读者面/--outdir@dev/test · ④读者面/--path@fleet/render · ④读者面/--scope@dev/build · ④读者面/--scope@dev/test · ④读者面/--set@model/opts · ④读者面/--set@model/set · ④读者面/--timeout@ask · ④读者面/--timeout@egg/run · ④读者面/--timeout@task/submit · ④读者面/--tool@dev/edit · ④读者面/--ttl@route/pin · ⑤反读面/--approval@dev/edit · ⑤反读面/--approval-tool@dev/edit · ⑤反读面/--by@agent/restart · ⑤反读面/--by@core/restart · ⑤反读面/--by@gap/backfill-unit · ⑤反读面/--by@gap/idea/promote · ⑤反读面/--docs-root@script/inventory/verify · ⑤反读面/--limit@gap/ls · ⑤反读面/--limit@repo/status · ⑤反读面/--path@config/reload · ⑤反读面/--path@core/daemon · ⑤反读面/--path@core/reload · ⑤反读面/--path@core/restart · ⑤反读面/--path@doctor · ⑤反读面/--path@model/add · ⑤反读面/--path@model/rm · ⑤反读面/--path@model/set · ⑤反读面/--remote@publish/run · ⑤反读面/--root@config/reload · ⑤反读面/--root@core/daemon · ⑤反读面/--root@core/reload · ⑤反读面/--root@core/restart · ⑤反读面/--root@doctor · ⑤反读面/--root@fleet/render · ⑤反读面/--root@model/add · ⑤反读面/--root@model/rm · ⑤反读面/--root@model/set · ⑤反读面/--state@dev/proposal · ⑤反读面/--symptom@gap/idea/promote · ⑤反读面/--target@plan\n面④ 读者面（按命令 · 棘轮 · 进退码）：RULES 末句 —— 「某命令 usage 串里的旗标 ⇒ 该命令的实现件必读」（缺口账 `GAP-20260927-325`）\n面⑤ 反读面（按命令 · 棘轮 · 进退码 · 缺口账 `GAP-20261005-15`）：RULES 末句 —— 「某命令实现按取值读了某旗标 ⇒ 该命令的 usage 串必列它」（面④ 的另一向）\n面⑥ 运行期用法行面（按命令 · 报告档 · 不进退码 · 缺口账 `GAP-20260928-183`）：件内运行期 stderr 用法行的旗标集 ⟷ 面① 命令树同命令用法串旗标集（双向对拍）\n"},
		{name: "help", argv: func(string) []string { return []string{"-h"} }, wantRC: 0, wantOut: "usage: check-flag-registry-parity.py [-h] [--root ROOT_OPT] [--json]\n                                     [--list-rules] [--self-test]\n                                     [root]\n\n门73 旗标两处登记面对拍（只读）\n\npositional arguments:\n  root             仓根（默认当前目录）\n\noptional arguments:\n  -h, --help       show this help message and exit\n  --root ROOT_OPT  同位置参数（显式档）\n  --json           末尾追加机器面一行\n  --list-rules     打判据 / 取数口径 / 退码\n  --self-test      只跑自检（合成夹具 · 不碰真目标）\n"},
		{name: "unknown", argv: func(string) []string { return []string{"--no-such-flag"} }, wantRC: 2, wantErr: "usage: check-flag-registry-parity.py [-h] [--root ROOT_OPT] [--json]\n                                     [--list-rules] [--self-test]\n                                     [root]\ncheck-flag-registry-parity.py: error: unrecognized arguments: --no-such-flag\n"},
		{name: "root", argv: func(fx string) []string { return []string{"--root", fx} }, wantRC: 0, wantOut: "门73 旗标两处登记面对拍（用法串 ⟷ valueFlagName 名字表 + 实现件读者面）· v1.2.0\n仓根：<FX>\n取数：命令树条数=2 · 名字表项数=2 · 解析面项数=2 · 用法串旗标数=6 · 透传命令=['gate/run']\n判据：面① 用法串 = 命令树 usage:（非字面量按包内常量名现读）；面② 名字表 = valueFlagName 体；面③ 解析面 = valueFlagName ∪ parseInvocation 专用 case。甲 缺一处 = 用法串取值形态的旗标 ∉ 面③（透传型命令不计）；乙 多一处 = 面② 的名字在全部用法串里零提及；丙 形态 = 面② 的名字在用法串里只写裸形。 存量豁免表逐条带原因（棘轮 · 只许下调）；新增分家项 ⇒ rc=1（面④/⑤ 同规：现读存量逐条带 reason 冻表 · 新增即红且点名 · 删即绿）。面④ 读者面（按命令 · 棘轮 · 进退码）= 某命令 usage 串里的旗标 ⇒ 该命令的实现件（定义其 run: 处理函数的包内 .go 件 · 先剥注释）必读它（字面 \"--F\" 或类型化字段 inv.<field>）—— 缺口账 `GAP-20260927-325`（全局「零读者」是错的判据）。面⑤ 反读面（按命令 · 棘轮 · 进退码）= 某命令实现按**取值**读了（`inv.flagVal`）某旗标 ⇒ 该命令的 usage 串必列它（面④ 的另一向 —— 「实现读 ⇒ 用法必列」，缺口 `GAP-20261005-15`）；拒执守卫（同一函数里 `inv.flagVals`/`inv.hasFlag` 存在性检查）与别处已列 不计。面⑥ 运行期用法行面（按命令 · 报告档 · 不进退码）= 件内运行期 stderr 用法行的旗标集 ⟷ 面① 命令树同命令用法串旗标集（双向对拍 · 全局选项不计 · 命令需精确同名）—— 差集非空逐条列（缺口 `GAP-20260928-183`）。面⑦ 声明面 ⇔ 用法串（按命令 · **进退码** · **零豁免** · 双向）= 把命令树当**单一真源**，取该命令**声明面**（`machine:` / `fields:` / `danger:`）里由实现**证明了的**消费旗标 ⇒ 断言「声明面推得的必列旗标 ⇔ 该命令用法串的旗标」双向零差集：甲 **声明未列** = `machine:` 声明对象是某台机、用法串又没给位置参数机器名 ⇒ 必列 `--node <机>`（`remote.go` 的 `enforceNodeRules` 真跑无目标 ⇒ rc=2 · 治「`build all --only cli` 必带 `--node` 而用法串未列」这一类）；`fields:` 非空 ⇒ 必列 `--json`（`main.go` 收 `--json` 的真条件）；`danger:` 非空 ⇒ 必列 `--dry-run` 或 `--yes`（危险档三态：`--dry-run` 出计划件 / 真跑要 `--yes`＋`--confirm`）；乙 **用法未注册** = 面甲的同一向（用法串取值形态的旗标 ∉ 解析面）。★ 零豁免表（✗ 不放宽）：真漂就逐条报红、**不许**借豁免转绿 ⇒ rc=1。\n新分家项 0 项：两面（除存量豁免）对齐。\n读者面（按命令 · 棘轮 · 进退码 · 缺口账 `GAP-20260927-325`）：「某命令 usage 串里的旗标 ⇒ 该命令的实现件必读」——本命令实现件不读的 3 项：\n  · zerg task submit · --all · 实现件 （取不到实现件）\n  · zerg task submit · --desc · 实现件 （取不到实现件）\n  · zerg task submit · --slice-id · 实现件 （取不到实现件）\n反读面（按命令 · 棘轮 · 进退码 · 缺口账 `GAP-20261005-15`）：「某命令实现按取值读（`inv.flagVal`）了某旗标 ⇒ 该命令的 usage 串必列它」（面④ 的另一向；拒执守卫 / 别处已列 不计）——本命令实现读了而 usage 没列的 0 项：\n运行期用法行面（按命令 · 报告档 · 不进退码 · 缺口 `GAP-20260928-183`）：「件内运行期 stderr 用法行的旗标集 ⟷ 面① 命令树同命令用法串旗标集（双向对拍 · 全局选项不计 · 命令精确同名）」—— 差集非空的 0 项：\n存量豁免 0 项（棘轮 · 逐条打印 · 不进退码）：\n退码：0=对齐 · 1=有新的分家项（逐条点名）· 2=取数面拿不到（不给结论）\n"},
		{name: "root_json", argv: func(fx string) []string { return []string{"--root", fx, "--json"} }, wantRC: 0, wantOut: "门73 旗标两处登记面对拍（用法串 ⟷ valueFlagName 名字表 + 实现件读者面）· v1.2.0\n仓根：<FX>\n取数：命令树条数=2 · 名字表项数=2 · 解析面项数=2 · 用法串旗标数=6 · 透传命令=['gate/run']\n判据：面① 用法串 = 命令树 usage:（非字面量按包内常量名现读）；面② 名字表 = valueFlagName 体；面③ 解析面 = valueFlagName ∪ parseInvocation 专用 case。甲 缺一处 = 用法串取值形态的旗标 ∉ 面③（透传型命令不计）；乙 多一处 = 面② 的名字在全部用法串里零提及；丙 形态 = 面② 的名字在用法串里只写裸形。 存量豁免表逐条带原因（棘轮 · 只许下调）；新增分家项 ⇒ rc=1（面④/⑤ 同规：现读存量逐条带 reason 冻表 · 新增即红且点名 · 删即绿）。面④ 读者面（按命令 · 棘轮 · 进退码）= 某命令 usage 串里的旗标 ⇒ 该命令的实现件（定义其 run: 处理函数的包内 .go 件 · 先剥注释）必读它（字面 \"--F\" 或类型化字段 inv.<field>）—— 缺口账 `GAP-20260927-325`（全局「零读者」是错的判据）。面⑤ 反读面（按命令 · 棘轮 · 进退码）= 某命令实现按**取值**读了（`inv.flagVal`）某旗标 ⇒ 该命令的 usage 串必列它（面④ 的另一向 —— 「实现读 ⇒ 用法必列」，缺口 `GAP-20261005-15`）；拒执守卫（同一函数里 `inv.flagVals`/`inv.hasFlag` 存在性检查）与别处已列 不计。面⑥ 运行期用法行面（按命令 · 报告档 · 不进退码）= 件内运行期 stderr 用法行的旗标集 ⟷ 面① 命令树同命令用法串旗标集（双向对拍 · 全局选项不计 · 命令需精确同名）—— 差集非空逐条列（缺口 `GAP-20260928-183`）。面⑦ 声明面 ⇔ 用法串（按命令 · **进退码** · **零豁免** · 双向）= 把命令树当**单一真源**，取该命令**声明面**（`machine:` / `fields:` / `danger:`）里由实现**证明了的**消费旗标 ⇒ 断言「声明面推得的必列旗标 ⇔ 该命令用法串的旗标」双向零差集：甲 **声明未列** = `machine:` 声明对象是某台机、用法串又没给位置参数机器名 ⇒ 必列 `--node <机>`（`remote.go` 的 `enforceNodeRules` 真跑无目标 ⇒ rc=2 · 治「`build all --only cli` 必带 `--node` 而用法串未列」这一类）；`fields:` 非空 ⇒ 必列 `--json`（`main.go` 收 `--json` 的真条件）；`danger:` 非空 ⇒ 必列 `--dry-run` 或 `--yes`（危险档三态：`--dry-run` 出计划件 / 真跑要 `--yes`＋`--confirm`）；乙 **用法未注册** = 面甲的同一向（用法串取值形态的旗标 ∉ 解析面）。★ 零豁免表（✗ 不放宽）：真漂就逐条报红、**不许**借豁免转绿 ⇒ rc=1。\n新分家项 0 项：两面（除存量豁免）对齐。\n读者面（按命令 · 棘轮 · 进退码 · 缺口账 `GAP-20260927-325`）：「某命令 usage 串里的旗标 ⇒ 该命令的实现件必读」——本命令实现件不读的 3 项：\n  · zerg task submit · --all · 实现件 （取不到实现件）\n  · zerg task submit · --desc · 实现件 （取不到实现件）\n  · zerg task submit · --slice-id · 实现件 （取不到实现件）\n反读面（按命令 · 棘轮 · 进退码 · 缺口账 `GAP-20261005-15`）：「某命令实现按取值读（`inv.flagVal`）了某旗标 ⇒ 该命令的 usage 串必列它」（面④ 的另一向；拒执守卫 / 别处已列 不计）——本命令实现读了而 usage 没列的 0 项：\n运行期用法行面（按命令 · 报告档 · 不进退码 · 缺口 `GAP-20260928-183`）：「件内运行期 stderr 用法行的旗标集 ⟷ 面① 命令树同命令用法串旗标集（双向对拍 · 全局选项不计 · 命令精确同名）」—— 差集非空的 0 项：\n存量豁免 0 项（棘轮 · 逐条打印 · 不进退码）：\n{\"gate\": \"flag-registry-parity\", \"version\": \"1.2.0\", \"violations\": [], \"exempt\": [], \"stale_exempt\": [], \"exempt_without_reason\": [], \"counts\": {\"命令树条数\": 2, \"名字表项数\": 2, \"解析面项数\": 2, \"用法串旗标数\": 6, \"透传命令\": [\"gate/run\"]}, \"no_reader\": [{\"command\": \"task/submit\", \"flag\": \"--all\", \"impl\": \"（取不到实现件）\"}, {\"command\": \"task/submit\", \"flag\": \"--desc\", \"impl\": \"（取不到实现件）\"}, {\"command\": \"task/submit\", \"flag\": \"--slice-id\", \"impl\": \"（取不到实现件）\"}], \"no_usage\": [], \"runtime_drift\": [], \"runtime_unmatched\": []}\n退码：0=对齐 · 1=有新的分家项（逐条点名）· 2=取数面拿不到（不给结论）\n"},
	}
}

// frpNormGolden —— 归一夹具根（t.TempDir() 随机路径 + macOS /var↔/private/var）⇒ 金标跨机可复现。
func frpNormGolden(s, fx string) string {
	if fx != "" {
		s = strings.ReplaceAll(s, fx, "<FX>")
	}
	if td := os.TempDir(); td != "" && td != fx {
		s = strings.ReplaceAll(s, td, "<TMP>")
	}
	return s
}

// frpRunGolden —— 逐格现建夹具、现跑 Go 半、归一后与冻值逐字比；返回失配格数。
// plant 非 nil 时在跑前往夹具落「旧态」py 件（两态用）。
func frpRunGolden(t *testing.T, tag string, plant func(fx string)) int {
	t.Helper()
	mism := 0
	for _, c := range frpGoldenCases() {
		fx := frpMkFixture(filepath.Join(t.TempDir(), "diffroot"))
		if plant != nil {
			plant(fx)
		}
		var ob, eb bytes.Buffer
		rc := flagRegistryParityGateMain(&ob, &eb, c.argv(fx))
		gotOut := frpNormGolden(ob.String(), fx)
		gotErr := frpNormGolden(eb.String(), fx)
		if rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, rc, c.wantRC)
		}
		if gotOut != c.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, c.name, gotOut, c.wantOut)
		}
		if gotErr != c.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, c.name, gotErr, c.wantErr)
		}
	}
	return mism
}

// TestFlagRegistryParityGoldenInternalized —— A 态（真仓 · 退役 py 不在盘）金标自证（✗ 不 Skip ·
// 逐格真断言 · ✗ 不放宽为「只判非空」）。
func TestFlagRegistryParityGoldenInternalized(t *testing.T) {
	repo := frpTestRepoRoot(t)
	py := filepath.Join(repo, "scripts", "gates", "check-flag-registry-parity.py")
	if st, err := os.Stat(py); err == nil && !st.IsDir() {
		t.Fatalf("A 态前提破：退役 py 竟在盘 %s（本件前提 = 内化后无 py）", py)
	}
	if n := frpRunGolden(t, "A真仓无py", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestFlagRegistryParityGoldenTwoStates —— 两态逐字同：A 态 = 夹具无 py（真仓现态）；B 态 = 合成
// 「旧态」夹具（把退役 py 路径 `scripts/gates/check-flag-registry-parity.py` 放回）⇒ 两态对同一套
// 金标逐字同命中（证 py 在/不在盘都不改判据结论 · ✗ 不 Skip）。
func TestFlagRegistryParityGoldenTwoStates(t *testing.T) {
	_ = frpTestRepoRoot(t) // 哨兵：锚 Go 半在盘 ⇒ fail-closed（✗ 不 Skip）
	if n := frpRunGolden(t, "A夹具无py", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
	plant := func(fx string) {
		p := filepath.Join(fx, "scripts", "gates", "check-flag-registry-parity.py")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/usr/bin/env python3\n# stub：造「旧态」——退役 py 在位（本件只证 py 在/不在盘不改判据结论）\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if n := frpRunGolden(t, "B旧态有py", plant); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（py 在盘不得改判据结论）", n)
	}
}
