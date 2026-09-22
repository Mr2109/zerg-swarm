// cli_impact_contract_test.go —— `B2` **契约面**的判据（任务单-影响面实施-20260922 §三 `B2` 判据 ①–③）。
//
// 任务单原文三条（**逐字**，本文件按它判）：
//
//	① 每条目录项带 **`entry` 四字段**（`id` / `change_class` / `gate` / `change_note`）且 `id` 集合与
//	   `registry.json` **逐字一致**；
//	② **兼容级别只许报、不许据此红绿任何一步**（**今天没有基线**）；
//	③ **对拍原型可跑**：基线 = `git rev-parse` 取「上一次改动该件的提交」及其父（**不落新快照件** ✗），
//	   比 `entries[].id` 集合 + 每条 6 字段 + `change_flow` / `authority` 整体，零差异**明写「无变更」**，
//	   只存**两枚指纹**（`head_sha` + 基线 rev）—— 现跑原型 **0.062s** 出 **7v7 + 1 处字段差**。
//
// 落点：`package main_test`（`RunForTest` 跑真命令 + 只读桥读真源 —— 判的是**当前源码**的行为）。
//
// 成对负控（每条判据都配一枚反面 · 防自欺）：
//
//	· 判据①（四字段齐）：**合成件**里给一条**缺 `change_note`** ⇒ 必须报「缺 1 处 · `S-y 缺 change_note`」
//	  且 `FourOK=false`（**不许静默绿**）；再给一条**缺 `gate`** + **同一 id 两条** ⇒ 两个计数
//	  （`CatN` / `SelfN`）必须**都对不上**并被报出来；
//	· 判据②（只报不拦）：同一形状的**合成件**只改 `change_class`（`B` ↔ `A`）⇒ 「会红」那一行的
//	  契约 id 必须**跟着变**（`B` ⇒ 有 · 全 `A` ⇒ 一条都没有），而兼容级别那一行**两跑都恒打**
//	  （「只报」= 报了但不作判断）⇒ 红绿只由 `change_class` 决定；
//	· 判据③（两枚指纹）：**不是 git 仓的合成件**上 ⇒ 必须照实写「取不到」，且**不许编出一枚 rev**
//	  （块里不许出现 40 位十六进制指纹）；真仓上 ⇒ 两枚指纹都在、且基线 rev ≠ `head_sha`；
//	· 源件自检：实现件里**不许**有写件/删件调用与 `emitEnvelope*`（只读 · 不落新快照件 · 不改包封）。
package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// contractFixtureRegistry 合成契约登记表（判据①②③ 的夹具）。
const contractFixtureRegistry = `{
  "schema": "zerg-contract-registry/1",
  "authority": {"truth": "代码内 schema", "exported": "文档只是导出物", "rule": "改真源 ⇒ 必须同批改导出物"},
  "entries": [
    {"id": "S-x", "name": "合成契约一", "truth": "core/x.go", "truth_shape": "const X = 1",
     "version_field": "X", "gate": "scripts/gates/check-x.py", "change_class": "B", "change_note": "合成一"},
    {"id": "S-y", "name": "合成契约二", "truth": "core/x.go", "truth_shape": "const Y = 1",
     "version_field": "Y", "gate": "scripts/gates/check-y.py", "change_class": "B", "change_note": "合成二"},
    {"id": "S-z", "name": "合成契约三", "truth": "core/x.go", "truth_shape": "const Z = 1",
     "version_field": "Z", "gate": "scripts/gates/check-z.py", "change_class": "A", "change_note": "合成三"}
  ],
  "change_flow": {"carrier": "合成", "steps": ["V0 提案成文"], "author_not_approver": true}
}`

// contractFixtureRoot 造一棵**合成仓**（只放契约登记表 + 一枚目标件 —— 判据全在这两件上）。
// 特意**不** `git init`：判据③ 的负控要的就是「取不到基线」那一种局面。
func contractFixtureRoot(t *testing.T, registry string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"core/internal/contract", "core", "scripts/gates"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatalf("合成目录造不出来：%v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "contract", "registry.json"),
		[]byte(registry), 0o644); err != nil {
		t.Fatalf("合成契约登记表写不出来：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "x.go"), []byte("package core\n\n// 合成目标件\n"), 0o644); err != nil {
		t.Fatalf("合成目标件写不出来：%v", err)
	}
	return root
}

// contractRealRoot 真仓根（测试的工作目录是 `core/cmd/zerg` ⇒ `../../..`）。
func contractRealRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("仓根解析不了：%v", err)
	}
	return root
}

// contractJSONLineOf 从块文本里摘出机读行并解开（解不开 ⇒ 直接判失败）。
func contractJSONLineOf(t *testing.T, block string) map[string]any {
	t.Helper()
	i := strings.Index(block, "契约面机读行：")
	if i < 0 {
		t.Fatalf("块里没有机读行：\n%s", block)
	}
	line := strings.TrimSpace(block[i+len("契约面机读行："):])
	if j := strings.Index(line, "\n"); j >= 0 {
		line = line[:j]
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		t.Fatalf("机读行解不开：%v\n%s", err, line)
	}
	return doc
}

// TestImpactContract_CatalogFourFieldsAndIDSetPaired —— 判据①（两半都判 · 各配成对负控）。
func TestImpactContract_CatalogFourFieldsAndIDSetPaired(t *testing.T) {
	// 正控（真仓 · 现读）：四字段齐 + id 集合与真源逐字一致。
	real := zerg.ImpactContractFaceForTest(contractRealRoot(t))
	if real.Status != "取值" {
		t.Fatalf("真仓的契约登记表该「取值」，实得 %s（%s）", real.Status, real.Reason)
	}
	if len(real.Rows) < 7 {
		t.Errorf("在册条目该 ≥7 条（任务单逐字「先只对 7 条」），实得 %d 条", len(real.Rows))
	}
	if !real.FourOK || len(real.Missing) != 0 {
		t.Errorf("判据①（前半 · 正控）：真仓四字段该齐，实得 FourOK=%t · 缺 %v", real.FourOK, real.Missing)
	}
	want := strings.Join(zerg.ImpactContractFourFieldsForTest(), ",")
	if want != "id,change_class,gate,change_note" {
		t.Errorf("四字段逐字 = %s（任务单点名的就是这四个）", want)
	}
	for _, r := range real.Rows {
		for _, f := range []string{r.ID, r.ChangeClass, r.Gate, r.ChangeNote} {
			if strings.TrimSpace(f) == "" {
				t.Errorf("判据①（正控）：目录项 `%s` 有四字段里的空格（`%v`）", r.ID, r)
			}
		}
	}
	if !real.IDEqual || real.SelfN != real.CatN || len(real.OnlyInReg) != 0 || len(real.OnlyInCat) != 0 {
		t.Errorf("判据①（后半 · 正控）：id 集合该与真源逐字一致，实得 %dv%d · 只在真源=%v · 只在目录=%v",
			real.CatN, real.SelfN, real.OnlyInReg, real.OnlyInCat)
	}
	if !strings.Contains(real.Block, fmt.Sprintf("✓ %d/%d 条四字段齐", len(real.Rows), len(real.Rows))) {
		t.Errorf("判据①（正控）：块里该逐字报「四字段齐 N/N」：\n%s", real.Block)
	}
	if doc := contractJSONLineOf(t, real.Block); doc["id_set_equal"] != true {
		t.Errorf("判据①（机读行）：`id_set_equal` 该为 true，实得 %v", doc["id_set_equal"])
	}
	// 判据① 的四字段必须**逐条打出来**（不是只给一个数）—— 每条的 id 都要在块里出现。
	for _, r := range real.Rows {
		if !strings.Contains(real.Block, "id="+r.ID+" ") {
			t.Errorf("判据①（目录项逐条打）：块里没有 `%s` 那一行", r.ID)
		}
	}

	// 负控①（合成件 · 一条缺 `change_note`）⇒ 必须报缺、且 FourOK=false（**不许静默绿**）。
	缺字段 := strings.Replace(contractFixtureRegistry, `"change_note": "合成二"`, `"change_note": ""`, 1)
	neg1 := zerg.ImpactContractFaceForTest(contractFixtureRoot(t, 缺字段))
	if neg1.FourOK {
		t.Error("判据①（负控①）：合成件里 `S-y` 缺 `change_note` ⇒ `FourOK` 必须为 false")
	}
	if strings.Join(neg1.Missing, " · ") != "S-y 缺 change_note" {
		t.Errorf("判据①（负控①）：缺字段该逐格点名，实得 %v", neg1.Missing)
	}
	if !strings.Contains(neg1.Block, "缺 1 处：S-y 缺 change_note") {
		t.Errorf("判据①（负控①）：块里必须把「缺哪一格」逐字打出来：\n%s", neg1.Block)
	}
	if strings.Contains(neg1.Block, "条四字段齐（缺字段 0 条") {
		t.Error("判据①（负控①）：缺字段的合成件**不许**打出「缺字段 0 条」那种绿话")
	}

	// 负控②（合成件 · 缺 `gate` + **同一 id 两条**）⇒ 两个计数都要能看出对不上。
	dup := strings.Replace(contractFixtureRegistry, `"gate": "scripts/gates/check-y.py", `, `"gate": "", `, 1)
	dup = strings.Replace(dup, `{"id": "S-z", "name": "合成契约三"`, `{"id": "S-x", "name": "合成契约三"`, 1)
	neg2 := zerg.ImpactContractFaceForTest(contractFixtureRoot(t, dup))
	if neg2.FourOK || !strings.Contains(strings.Join(neg2.Missing, " · "), "S-y 缺 gate") {
		t.Errorf("判据①（负控②）：缺 `gate` 那条该被点名，实得 FourOK=%t · %v", neg2.FourOK, neg2.Missing)
	}
	if neg2.SelfN == neg2.CatN {
		t.Errorf("判据①（负控②）：同一 id 两条 ⇒ 真源 id 列(%d) 与目录 id 集合(%d) **该对不上**（两个数都要打出来）",
			neg2.SelfN, neg2.CatN)
	}
	if !strings.Contains(neg2.Block, fmt.Sprintf("%dv%d", neg2.CatN, neg2.SelfN)) {
		t.Errorf("判据①（负控②）：两个计数都要在块里逐字打出来：\n%s", neg2.Block)
	}
	if !strings.Contains(neg2.Block, "id 重复") {
		t.Errorf("判据①（负控②）：id 重复那一格该被报出来：\n%s", neg2.Block)
	}
}

// TestImpactContract_CompatLevelReportOnlyPaired —— 判据②（**只报** · 拿来判红绿的那条路与它无关）。
func TestImpactContract_CompatLevelReportOnlyPaired(t *testing.T) {
	root := contractRealRoot(t)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_STATE_DIR", t.TempDir()) // 判据件不碰真状态目录

	// 正控①（真命令）：块里恒打兼容级别那一行，且那一行**不参与**任何红绿判定。
	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"impact", "S-g"}, &out, &errb)
	if rc != 0 {
		t.Fatalf("`impact S-g` 该退 0（有影响面），实得 rc=%d", rc)
	}
	if !strings.Contains(errb.String(), "判据②（兼容级别**只报不拦**）") {
		t.Errorf("判据②（只报）：兼容级别那一行必须恒打（不打 = 连「报」都没做到）：\n%s", errb.String())
	}
	if !strings.Contains(errb.String(), zerg.ImpactContractCompatLevelForTest()) {
		t.Errorf("判据②：兼容级别取值该逐字出现（`%s`）", zerg.ImpactContractCompatLevelForTest())
	}
	会红 := ""
	for _, ln := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(ln, "会红：") {
			会红 = ln
		}
	}
	if 会红 == "" {
		t.Fatalf("人面没有「会红：」那一行：\n%s", out.String())
	}
	if strings.Contains(会红, "兼容级别") {
		t.Errorf("判据②（**不许据此红绿**）：兼容级别**不许**出现在「会红」那一行里：%s", 会红)
	}
	if !strings.Contains(会红, "契约 S-g") {
		t.Errorf("判据②（正控）：`S-g` 的 `change_class=B` ⇒ 该进「会红」闭集：%s", 会红)
	}
	if doc := contractJSONLineOf(t, errb.String()); doc["compat_level_affects_red"] != false ||
		doc["compat_level_report_only"] != true {
		t.Errorf("判据②（机读行）：`compat_level_affects_red` 该恒 false · `compat_level_report_only` 该恒 true，实得 %v / %v",
			doc["compat_level_affects_red"], doc["compat_level_report_only"])
	}

	// 成对负控（同一形状的合成件 · **只改 `change_class`**）⇒ 红绿跟着 `change_class` 变，
	// 而兼容级别那一行**两跑都恒打** —— 这就是「只报、不据此判红绿」的可判形态。
	rootB := contractFixtureRoot(t, contractFixtureRegistry)
	redB, err := zerg.ImpactRedLineForTest(rootB, "core/x.go")
	if err != nil {
		t.Fatalf("合成件（B 档）会红行算不出来：%v", err)
	}
	if !strings.Contains(redB, "契约 S-x,S-y") {
		t.Errorf("判据②（成对负控 · 正半边）：两条 `change_class=B` 该进「会红」，实得 %s", redB)
	}
	if strings.Contains(redB, "S-z") {
		t.Errorf("判据②（成对负控）：`change_class=A` 那条**不许**进「会红」，实得 %s", redB)
	}
	全A := strings.ReplaceAll(contractFixtureRegistry, `"change_class": "B"`, `"change_class": "A"`)
	rootA := contractFixtureRoot(t, 全A)
	redA, err := zerg.ImpactRedLineForTest(rootA, "core/x.go")
	if err != nil {
		t.Fatalf("合成件（全 A 档）会红行算不出来：%v", err)
	}
	if strings.Contains(redA, "契约 S-") {
		t.Errorf("判据②（成对负控 · 反半边）：全 `A` 档**一条契约 id 都不许**进「会红」，实得 %s", redA)
	}
	for name, r := range map[string]string{"B 档": rootB, "全 A 档": rootA} {
		blk := zerg.ImpactContractFaceForTest(r).Block
		if !strings.Contains(blk, zerg.ImpactContractCompatLevelForTest()) {
			t.Errorf("判据②（只报 · %s）：兼容级别那一行**与红绿无关、恒打**，实得：\n%s", name, blk)
		}
	}
}

// TestImpactContract_SnapshotCaliberFingerprintsAndReadOnlySource —— 判据③（口径 + 两枚指纹 + 不落实现）。
//
// 判据③ 的**原型本身**（`0.062s` 那一件）按任务单 §三 B2 目标「**只出设计、不落实现**」**不进代码**
// ⇒ 本测试判的是「**口径与两枚指纹在不在**」+「取不到时不许编」+「实现件里确实没有那个引擎」；
// 原型的现跑读数在 `项目文档/v2.5.11/设计-契约快照口径-v1.0.md` §四（复跑命令那一节）。
func TestImpactContract_SnapshotCaliberFingerprintsAndReadOnlySource(t *testing.T) {
	real := zerg.ImpactContractFaceForTest(contractRealRoot(t))
	// 判据③ 四条口径逐字（①②③④）都要在块里。
	for _, want := range []string{
		"只出设计",
		"不落实现",
		"基线 = `git log -1 --format=%H -- core/internal/contract/registry.json`",
		"零差异**明写「无变更」**",
		"只存两枚指纹",
		"不落新快照件",
		"不扩到全仓",
		"`change_flow` / `authority` 整体",
		"不许把「表 ↔ 件」的绿读成「承诺没破」",
	} {
		if !strings.Contains(real.Block, want) {
			t.Errorf("判据③：块里缺口径原文 %q：\n%s", want, real.Block)
		}
	}
	if want := strings.Join(zerg.ImpactContractSixFieldsForTest(), "` / `"); !strings.Contains(real.Block, want) {
		t.Errorf("判据③（比什么）：6 字段该逐字列出来（`%s`）", want)
	}
	// 两枚指纹（真仓上）：都在 · 且**基线 rev ≠ head_sha**（取成同一枚 = 比了个空）。
	if real.Head == "" || real.Rev == "" || real.Parent == "" || real.FpReason != "" {
		t.Fatalf("判据③（正控）：真仓该给两枚指纹，实得 head=%q · rev=%q · parent=%q · reason=%q",
			real.Head, real.Rev, real.Parent, real.FpReason)
	}
	if real.Rev == real.Head {
		t.Errorf("判据③（正控）：基线 rev 与 `head_sha` 取成同一枚（%s）⇒ 这一比是空的", real.Head)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(real.Rev) || !strings.Contains(real.Block, "基线 rev="+real.Rev) {
		t.Errorf("判据③：基线那一枚指纹该逐字落块（rev=%q）", real.Rev)
	}
	// `R43` 那一格（任务单 §三 B2 前置依赖点名 · 本件只点名不派实现）：块里必须出现那道门的路径。
	if !strings.Contains(real.Block, zerg.ImpactContractRegistryGateRelForTest()) {
		t.Errorf("`R43`：块里必须点名 `registry.json` 自陈的那道门（`%s`）",
			zerg.ImpactContractRegistryGateRelForTest())
	}

	// 负控（合成件 · **不是 git 仓**）⇒ 照实写「取不到」，且**不许编出一枚 rev**。
	neg := zerg.ImpactContractFaceForTest(contractFixtureRoot(t, contractFixtureRegistry))
	if neg.Rev != "" || neg.Parent != "" || neg.FpReason == "" {
		t.Errorf("判据③（负控）：合成件上取不到基线 ⇒ rev/parent 该为空且 reason 该非空，实得 %q/%q/%q",
			neg.Rev, neg.Parent, neg.FpReason)
	}
	if !strings.Contains(neg.Block, "本跑指纹：**取不到**") {
		t.Errorf("判据③（负控）：取不到时必须照实写（不许静默留空）：\n%s", neg.Block)
	}
	if loc := regexp.MustCompile(`[0-9a-f]{40}`).FindString(neg.Block); loc != "" {
		t.Errorf("判据③（负控）：取不到时**一枚 40 位指纹都不许出现**（编指纹 = 伪造取证），实得 %q", loc)
	}
	if doc := contractJSONLineOf(t, neg.Block); doc["baseline_rev"] != "" {
		t.Errorf("判据③（负控 · 机读行）：`baseline_rev` 该为空串，实得 %v", doc["baseline_rev"])
	}

	// 源件自检（两条禁令的可判形态）：
	//	① **不落实现那个对拍引擎**：实现件里不许有「旧版 ↔ 新版」的比较代码（`git show` 那一支）；
	//	② **只读 · 不落新快照件**：实现件里不许有写件 / 删件调用，也不许碰 `emitEnvelope*`。
	src, err := zerg.ImpactContractSourceForTest()
	if err != nil {
		t.Fatalf("读实现件源码失败：%v", err)
	}
	for _, banned := range []string{"WriteFile(", "Create(", "os.MkdirAll", "os.Remove", "git\", \"show", "emitEnvelope("} {
		if strings.Contains(src, banned) {
			t.Errorf("源件自检：实现件里不该出现 %q（只读 · 不落新快照件 · 不改包封 · 不落那个引擎）", banned)
		}
	}
	for _, want := range []string{"只出设计", "不落实现", "不落新快照件", "impactContractFingerprints"} {
		if !strings.Contains(src, want) {
			t.Errorf("源件自检：实现件里该写死 %q", want)
		}
	}
}
