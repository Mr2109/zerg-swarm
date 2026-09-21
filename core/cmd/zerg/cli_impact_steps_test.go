// cli_impact_steps_test.go —— `B1` 步名真源与探针的判据（任务单-影响面实施-20260922 §三 `B1` 判据 ①–④）。
//
// 落点：`package main_test`（`RunForTest` 跑真命令 + 只读桥读真源 —— 判的是**当前源码**的行为）。
//
// 成对负控（每条判据都配一枚反面 · 防自欺）：
//
//	· 判据①（全名 + **恰好命中 1 条**）：**合成门禁**里放一对子串撞车（`cargo test` ⊂ `ui: cargo test`）
//	  ⇒ 短的那个一次打 **2** 条、长的那个 **1** 条 ⇒ 判据① 不成立的那个**不许**被投影；
//	· 判据②（负控）：塞一个不存在的名字 ⇒ `--emit-cmd` **必须 `rc=2`**（真门禁 + 合成门禁各一枚）；
//	· 判据③（步数**不许写常量**）：合成门禁自报 **4** 步 ⇒ 报出来的就是 **4**（不是真门禁那个 N）；
//	· 判据④（join 不到 ⇒ 「未接步」）：合成里 `S-y` 的 gate 只有「由 `X`: go test 覆盖」⇒ 必须未接步；
//	  真仓里 `S-a` 的脚本在盘上但**没有一步调用它** ⇒ 必须未接步（**有牙的那种**：本跑命令串里真没有）；
//	· 「不许把估计写成必红」：那一格必须出现「预测」，且**不许**出现「必红」。
package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// stepCountRe —— 「本跑 N 步」的形态（判据③：那个 N 只能来自**现跑** ⇒ 测试不许自己写它）。
var stepCountRe = regexp.MustCompile(`本跑 \d+ 步`)

// stepFixtureScript —— **合成门禁**（只为判据用：只实现 `--list` / `--emit-cmd` 两条只读入口）。
// 它故意与真门禁**不同源**：步数 4、名字里带一对子串撞车、其中一步的命令串里没有那条脚本路径。
const stepFixtureScript = `#!/usr/bin/env bash
# 合成门禁（B1 判据夹具）：4 步 · 只读两入口。
names=("门S 合成门一（阻断）" "gofmt -l core" "cargo test" "ui: cargo test")
cmds=("python3 scripts/gates/check-x.py" "gofmt -l ." "cargo test" "cd ui && cargo test")
case "${1:-}" in
  --list)
    echo "── 步骤清单（scope 模式 名称）──"
    echo "   gates tri    门S 合成门一（阻断）"
    echo "   go    empty  gofmt -l core"
    echo "   ui    rc     cargo test"
    echo "   ui    rc     ui: cargo test"
    echo "共 4 步"
    ;;
  --emit-cmd)
    hit=0
    for i in "${!names[@]}"; do
      case "${names[$i]}" in *"$2"*) echo "${cmds[$i]}"; hit=$((hit + 1)) ;; esac
    done
    if [ "${hit}" -eq 0 ]; then echo "✗ 步骤名里没有匹配 $2 的" >&2; exit 2; fi
    ;;
  *) echo "✗ 合成夹具只认 --list / --emit-cmd" >&2; exit 2 ;;
esac
`

// stepFixtureRegistry —— 合成契约登记表：`S-x` 的 gate 指脚本（能 join 上）· `S-y` 只有粗步表述。
const stepFixtureRegistry = `{
  "schema": "zerg-contract-registry/1",
  "entries": [
    {"id": "S-x", "name": "合成契约一", "truth": "core/x.go", "gate": "scripts/gates/check-x.py",
     "change_class": "B", "change_note": "合成"},
    {"id": "S-y", "name": "合成契约二", "truth": "core/y.go", "gate": "（由 core: go test 覆盖）",
     "change_class": "B", "change_note": "合成"}
  ]
}`

// stepFixtureRoot 造一棵**合成仓**（只放门禁脚本 + 契约登记表 ⇒ 判据全在这两件上）。
func stepFixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"scripts/gates", "core/internal/contract"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatalf("合成目录造不出来：%v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "gates", "precommit-gates.sh"), []byte(stepFixtureScript), 0o755); err != nil {
		t.Fatalf("合成门禁脚本写不出来：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "contract", "registry.json"), []byte(stepFixtureRegistry), 0o644); err != nil {
		t.Fatalf("合成契约登记表写不出来：%v", err)
	}
	return root
}

// stepProbeLines 探针输出有几条（空输出 ⇒ 0）。
func stepProbeLines(out string) int {
	if strings.TrimSpace(out) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimRight(out, "\n"), "\n"))
}

// TestImpactSteps_SyntheticFixtureJudgesAllFour —— **合成夹具**上把判据①–④ 逐条判掉。
//
// 为什么用合成件而不是真门禁：判据③ 要证的是「步数**随真源**」—— 只有把真源换成另一份
// **条数不同**的表，才能把「写常量」这件事判出来（真门禁那一个 N 天天在变 ⇒ 拿它当常量
// 反而看不出来）。判据①②④ 同理：夹具能把「子串撞车」「没有一步调用它」摆成定局。
func TestImpactSteps_SyntheticFixtureJudgesAllFour(t *testing.T) {
	root := stepFixtureRoot(t)

	// 判据①②：三枚探针（正控 1 条 · 子串撞车 2 条 · 不存在 ⇒ rc=2）。
	out, rc := zerg.ImpactStepProbeForTest(root, "门S 合成门一（阻断）")
	if rc != 0 || stepProbeLines(out) != 1 || !strings.Contains(out, "scripts/gates/check-x.py") {
		t.Errorf("判据①（正控）：全名 `门S 合成门一（阻断）` 该恰好 1 条命令串，实得 rc=%d · %d 条 · %q",
			rc, stepProbeLines(out), out)
	}
	outShort, rcShort := zerg.ImpactStepProbeForTest(root, "cargo test")
	outLong, rcLong := zerg.ImpactStepProbeForTest(root, "ui: cargo test")
	if rcShort != 0 || rcLong != 0 {
		t.Fatalf("探针退码不该非 0：短=%d · 长=%d", rcShort, rcLong)
	}
	if stepProbeLines(outShort) != 2 || stepProbeLines(outLong) != 1 {
		t.Errorf("判据①（成对负控）：子串撞车那一对应该是 短=%d 条 / 长=%d 条（实得 短=%d / 长=%d）—— "+
			"这就是「必须按全名 + 命中数判」的那一格",
			2, 1, stepProbeLines(outShort), stepProbeLines(outLong))
	}
	outNeg, rcNeg := zerg.ImpactStepProbeForTest(root, zerg.ImpactStepNegativeProbeForTest())
	if rcNeg != 2 || stepProbeLines(outNeg) != 0 {
		t.Errorf("判据②（负控）：不存在的名字 ⇒ 必须 rc=2 且 0 条，实得 rc=%d · %d 条", rcNeg, stepProbeLines(outNeg))
	}

	// 判据③④：把两条契约 id 投影到步名（按需档 = 真拉真源）。
	p := zerg.ImpactStepProjectionForTest(root, true, "S-x", "S-y")
	if p.Status != "取值" {
		t.Fatalf("合成件上该取到数，实得 status=%s · reason=%s", p.Status, p.Reason)
	}
	if p.Total != 4 {
		t.Errorf("判据③：步数该**随真源**（合成门禁自报 4 步），实得 %d —— 写常量就会在这里露出来", p.Total)
	}
	if strings.Join(p.Steps, ",") != "门S 合成门一（阻断）" {
		t.Errorf("join（真同源键 = 脚本路径）：该只接上那一步，实得 %v", p.Steps)
	}
	if got := strings.Join(p.ByID["S-x"], ","); got != "门S 合成门一（阻断）" {
		t.Errorf("S-x 该接到合成门一，实得 %q", got)
	}
	if len(p.ByID["S-y"]) != 0 {
		t.Errorf("S-y 的 gate 只有粗步表述 ⇒ 不该接上任何一步，实得 %v", p.ByID["S-y"])
	}
	if why := p.Unjoined["S-y"]; !strings.Contains(why, "没有门脚本路径") || !strings.Contains(why, "粗步") {
		t.Errorf("判据④：S-y 该写「未接步（gate 里没有门脚本路径 · 不许拿粗步冒充）」，实得 %q", why)
	}
	if p.NegRC != 2 {
		t.Errorf("判据②：投影里那条负控退码该是 2，实得 %d", p.NegRC)
	}
	// 判据①：名字不唯一的那一步**必须**标不可机检（不许猜成哪一步）。
	if len(p.Uncheck) != 1 || !strings.Contains(p.Uncheck[0], "名字不唯一") {
		t.Errorf("判据①：名字不唯一的那一步该进「不可机检」清单，实得 %v", p.Uncheck)
	}
	// 措辞：这一格是**预测**，不许写成「必红」。
	if !strings.Contains(p.Line, "预测") || strings.Contains(p.Line, "必红") {
		t.Errorf("措辞判据：门步那一格必须写「预测」、不许写「必红」，实得 %q", p.Line)
	}

	// 档位成对负控（§4.4「贵项按需」）：**同一条 id** 换档 ⇒ 两个不同的态，不许混成一个
	// —— ① 有候选 + 默认档 = **未机检**（不是「没有」）② 没有候选 + 默认档 = **未接步**（照样能判）。
	cheapX := zerg.ImpactStepProjectionForTest(root, false, "S-x")
	if cheapX.Status != "未机检" || !strings.Contains(cheapX.Line, "--all") {
		t.Errorf("默认档遇上候选该写「未机检 + 怎么拉（--all）」，实得 status=%s · %q", cheapX.Status, cheapX.Line)
	}
	if stepCountRe.MatchString(cheapX.Line) {
		t.Errorf("默认档**不拉**真源 ⇒ 不许出现「本跑 N 步」（不许把没跑的编成数），实得 %q", cheapX.Line)
	}
	cheapY := zerg.ImpactStepProjectionForTest(root, false, "S-y")
	if cheapY.Status != "未拉" || !strings.Contains(cheapY.Line, "未接步") {
		t.Errorf("没有候选的那一条在默认档也该判「未接步」（不用拉真源），实得 status=%s · %q", cheapY.Status, cheapY.Line)
	}
}

// TestImpactSteps_LiveTruthSourceAndRedLine —— **真门禁**上的判据③ + 「会红」接真面（现跑）。
//
// 三跑（目标各不相同，判的是不同一格）：
//
//	① `S-g`（契约态 · 4 条 gate 路径全接得上）⇒ 人面「会红」那一行里出现**现跑的 N**；
//	② `agent/internal/enclosure/contract.go`（③ 命中 `S-b`：gate 里只有粗步表述）⇒ **不用拉真源**
//	  就写「未接步」（没有候选就不跑 —— 也不编步数）；
//	③ `scripts/gates/check-compat-manifest.py`（③ 命中 `S-a`：脚本在盘上但**没有一步调用它**）
//	   ⇒ 必须写「未接步」，且理由是**有牙的那种**（本跑命令串里真没有它）。
func TestImpactSteps_LiveTruthSourceAndRedLine(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)

	// 真源现跑（含逐名探针）：本跑步数 / 解析条数 / 负控退码 —— 三样都不许由测试写常量。
	snap := zerg.ImpactStepTableOfForTest(root, true)
	if snap.Status != "取值" {
		t.Fatalf("步名真源取不到（%s）：%s", snap.Status, snap.Detail)
	}
	if snap.Total <= 0 || snap.Listed != snap.Total {
		t.Fatalf("`--list` 现跑自报 %d 步 / 解析出 %d 条 —— 不对称就不许拿来当结论", snap.Total, snap.Listed)
	}
	if snap.HeadSHA == "" {
		t.Error("`head_sha` 取不到 ⇒ 按 §7.4 只许当参考（本跑该取得到）")
	}
	if snap.NegRC != 2 {
		t.Errorf("判据②：真门禁上的负控该 rc=2，实得 %d", snap.NegRC)
	}
	if snap.Resolved+snap.Ambiguous+snap.Unresolv != snap.Listed {
		t.Errorf("逐名分类不齐：恰好 1 条 %d + 名字不唯一 %d + 取不到 %d ≠ 清单 %d",
			snap.Resolved, snap.Ambiguous, snap.Unresolv, snap.Listed)
	}
	if len(snap.AmbigNames) != snap.Ambiguous {
		t.Errorf("名字不唯一清单与计数不符：%v vs %d", snap.AmbigNames, snap.Ambiguous)
	}

	// ⓪ 默认档（吃紧档）那一格：有候选但**不拉贵项** ⇒ 必须写「未机检」并给出 `--all`
	//   （同一目标换档 ⇒ 两个不同的态；这一跑**不拉真源**，所以它也是「档位纪律」的现跑证据）。
	rc, out, errb := runCapture("impact", "core/cmd/zerg/main.go")
	if rc != 0 {
		t.Fatalf("`impact core/cmd/zerg/main.go`（默认档）该退 0，实得 %d · stderr=%s", rc, errb)
	}
	red := lineOf(out, "会红：")
	if !strings.Contains(red, "未机检") || !strings.Contains(red, "--all") {
		t.Errorf("默认档遇上候选该写「未机检 + 按需档 --all」，实得 %q", red)
	}
	if stepCountRe.MatchString(red) {
		t.Errorf("默认档不拉真源 ⇒ 不许出现「本跑 N 步」，实得 %q", red)
	}

	// ① 契约目标（**按需档 `--all`**）：人面「会红」那一行必须带**现跑的 N** + 门步（预测）。
	rc, out, errb = runCapture("impact", "S-g", "--all")
	if rc != 0 {
		t.Fatalf("`impact S-g` 该退 0，实得 %d · stderr=%s", rc, errb)
	}
	if err := judgeImpactHuman(out, impactHeads); err != nil {
		t.Errorf("人面仍须恒三行：%v · stdout=%q", err, out)
	}
	red = lineOf(out, "会红：")
	if !strings.Contains(red, fmt.Sprintf("本跑 %d 步", snap.Total)) {
		t.Errorf("判据③：「会红」行该带**本跑 %d 步**（现跑 · 不许写常量），实得 %q", snap.Total, red)
	}
	if !strings.Contains(red, "门步") || !strings.Contains(red, "预测") || strings.Contains(red, "必红") {
		t.Errorf("「会红」行的门步那一格：要写「预测」、不许写「必红」，实得 %q", red)
	}
	if !strings.Contains(red, "`--list` 现跑") {
		t.Errorf("那一格要明写真源（`--list` 现跑），实得 %q", red)
	}
	// stderr 的步名真源块：判据①②③④ 的可读落点（`warnings[]` 那一格今天落不进去 —— 缺口照实点名）。
	for _, want := range []string{"步名真源与探针", "判据②（负控）", "rc=2", "未接步", "只报数量不报步名", "零副作用"} {
		if !strings.Contains(errb, want) {
			t.Errorf("stderr 的步名真源块里没有 %q", want)
		}
	}

	// ② 粗步表述那一条（`S-b`）：没有可 join 的脚本路径 ⇒ 未接步，且**不拉真源**（默认档即可判）。
	rc, out, errb = runCapture("impact", "agent/internal/enclosure/contract.go")
	if rc != 0 {
		t.Fatalf("`impact agent/internal/enclosure/contract.go` 该退 0，实得 %d · stderr=%s", rc, errb)
	}
	red = lineOf(out, "会红：")
	if !strings.Contains(red, "未接步") || !strings.Contains(red, "S-b") {
		t.Errorf("粗步表述那一条该写「未接步 S-b」，实得 %q", red)
	}
	if stepCountRe.MatchString(red) {
		t.Errorf("没有候选就不许编「本跑 N 步」（那一格该只写「未接步」的理由），实得 %q", red)
	}

	// ③ 脚本在盘上但没一步调用它（`S-a` · 按需档）：未接步的理由必须是**有牙的那种**。
	rc, out, errb = runCapture("impact", "scripts/gates/check-compat-manifest.py", "--all")
	if rc != 0 {
		t.Fatalf("`impact scripts/gates/check-compat-manifest.py` 该退 0，实得 %d · stderr=%s", rc, errb)
	}
	red = lineOf(out, "会红：")
	if !strings.Contains(red, "未接步") || !strings.Contains(red, "S-a") {
		t.Errorf("`S-a` 该写「未接步」，实得 %q", red)
	}
	if !strings.Contains(errb, "没有一步调用它") {
		t.Errorf("未接步的理由该是「本跑 %d 步的命令串里没有出现它」（有牙的那种），stderr=%s", snap.Total, errb)
	}
	if !strings.Contains(red, fmt.Sprintf("本跑 %d 步", snap.Total)) {
		t.Errorf("这一跑拉过真源 ⇒ 那一格该带本跑 %d 步，实得 %q", snap.Total, red)
	}
	// 零副作用：三跑都不许在状态目录里落审计件（只读探针 · 不落审计）。
	assertNoAuditWrites(t, state)
}

// TestImpactSteps_SameOutputAndNoSideEffects —— `M8`：同一目标两跑 ⇒ 人面与 `items` **逐字节相同**；
// 且两跑都不许落任何件（干跑那一条 discipline 的四条在 `B1` 这一档同样成立）。
func TestImpactSteps_SameOutputAndNoSideEffects(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)

	rc1, out1, _ := runCapture("impact", "S-g", "--all")
	rc2, out2, errb2 := runCapture("impact", "S-g", "--all")
	if rc1 != rc2 {
		t.Fatalf("同一口令两跑退码不同：%d vs %d", rc1, rc2)
	}
	if out1 != out2 {
		t.Errorf("同一目标两跑人面不逐字相同（`M8`）：\n① %q\n② %q", out1, out2)
	}
	if !strings.Contains(errb2, "步名真源与探针") {
		t.Errorf("第二跑没有步名真源块 ⇒ 两跑不是同一条路：%s", errb2)
	}
	rc3, out3, _ := runCapture("impact", "S-g", "--json", "what,why,how,red")
	rc4, out4, _ := runCapture("impact", "S-g", "--json", "what,why,how,red")
	if rc3 != rc4 || out3 != out4 {
		t.Errorf("机器面两跑不逐字相同（`M8`）：rc %d/%d", rc3, rc4)
	}
	if !strings.Contains(out3, "\"red\"") {
		t.Errorf("`items` 里该有四字段（`red` 是「会红」闭集那一格）：%q", out3)
	}
	assertNoAuditWrites(t, state)
}

// assertNoAuditWrites 状态目录里**不许落审计/新建的旁路件**（`zerg impact` 只读：探针也只读）。
// 注意：`A5` 的落盘缓存（`impact-cache/`）是**设计内的写**（那一档本就落盘）⇒ 不在这条判据里。
func assertNoAuditWrites(t *testing.T, state string) {
	t.Helper()
	ents, err := os.ReadDir(state)
	if err != nil {
		t.Fatalf("状态目录读不到：%v", err)
	}
	for _, e := range ents {
		name := e.Name()
		if strings.Contains(name, "audit") || strings.Contains(name, "backfill") {
			t.Errorf("只读面落了审计/回填件：%s（`impact` 不落审计 · 回填属 `B3`）", name)
		}
	}
}
