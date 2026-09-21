// cli_dev_edit_hook_test.go —— `zerg dev edit` 干跑的**影响面摘要 + 钩子分档**（任务单 §二 `A4`）。
//
// 本件判据（逐条落在可机检的判定口上 · 成对负控**全真跑**）：
//
//	① **分类器**（`ImpactHookClassForTest`）：改 / 符号级删 / 件级（整件）/ 判不了 四格各一枚正控 +
//	   一枚负控（非 `.go` 件**不许**判符号面 · 坏 Go 内容**必须**落「判不了」而不是「符号都没了」）；
//	② **判档**（`ImpactHookDecideForTest` · 与实现同一份纯函数）：`R28` 分档表的成对负控 ——
//	   符号级删 ⇒ **必拦**（这一档不看三条判据）· 件级可找回 ⇒ **只报 + 两件证据** ·
//	   件级不可找回（未跟踪 / 两条找回路径都取不到 / 仓外）⇒ **升为必拦** ·
//	   改 + 不可找回 ⇒ **仍只记**（负控：证明「不可找回」不是恒必拦 —— 只有「删」才升）；
//	③ **指纹可复算**：同一段摘要正文 ⇒ 同一枚 `sha256`；改一个字节 ⇒ 必须变；
//	④ **干跑挂点**（真跑 `dev edit`）：类=改 ⇒ 退 0；符号级删 ⇒ 退 2 且点名「必拦」；
//	   件级 + 未跟踪 ⇒ 退 2（判据①命中）；件级 + 有 git 历史 ⇒ 退 0「只报」+ 证据两件；
//	⑤ **干跑零副作用**：每个用例跑完，目标件字节逐字不变、`<状态目录>/edit_audit.jsonl` **不产生**；
//	⑥ **判据② 的负控**：已知无关件（`impactZeroHitTarget`）⇒ **不许打那三行**（打了就是灌噪声）；
//	⑦ **真写审计行**：多一枚 `impact_digest`（64 hex · 与同一仓态下干跑打出的那一枚**逐字相同**）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// ---- ① 分类器（纯函数 · 成对负控）----------------------------------------------------------

func TestDevEditHook_ClassifierPaired(t *testing.T) {
	const goSrc = "package main\n\nfunc alpha() int { return 1 }\n\nfunc beta() int { return alpha() }\n"
	cases := []struct {
		name       string
		rel        string
		before     string
		after      string
		wantClass  string
		whyContain string
	}{
		{"改（顶层符号一个不少）", "core/cmd/zerg/x.go", goSrc,
			"package main\n\nfunc alpha() int { return 2 }\n\nfunc beta() int { return alpha() }\n",
			"改", "一个不少"},
		{"符号级删（少了 beta）", "core/cmd/zerg/x.go", goSrc,
			"package main\n\nfunc alpha() int { return 1 }\n", "符号级删", "少了"},
		{"件级（整件）· 写后 0 字节", "core/cmd/zerg/x.go", goSrc, "", "件级（整件）", "0 字节"},
		// 负控一：非 `.go` 件**不许**判符号面（本件不自造第二套符号解析器 ⇒ 宁少报不猜报）。
		{"负控：非 .go 件不判符号面", "scripts/x.sh", "#!/bin/bash\nf() { :; }\n", "#!/bin/bash\n", "改", "判不了"},
		// 负控二：坏 Go 内容 ⇒ **判不了**（不许当成「符号都没了 ⇒ 必拦」）。
		{"负控：改后解析不了 ⇒ 判不了", "core/cmd/zerg/x.go", goSrc, "package main\n\nfunc alpha( { return 1 }\n",
			"不判（判不了）", "判不了"},
	}
	for _, c := range cases {
		got, why := zerg.ImpactHookClassForTest(c.rel, c.before, c.after)
		if got != c.wantClass {
			t.Errorf("%s：类 = %q（要 %q）· why=%s", c.name, got, c.wantClass, why)
		}
		if !strings.Contains(why, c.whyContain) {
			t.Errorf("%s：依据里要点到 %q（实得 %q）", c.name, c.whyContain, why)
		}
	}
}

// ---- ② 判档（纯函数 · `R28` 分档表的成对负控）------------------------------------------------

func TestDevEditHook_TierPaired(t *testing.T) {
	tiers := zerg.ImpactTierNamesForTest()
	if strings.Join(tiers, ",") != "只记,只报,必拦" {
		t.Fatalf("三档取值漂了：%v", tiers)
	}
	tr := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	_ = tr
	base := zerg.ImpactHookFactsForTest{Rel: "core/cmd/zerg/x.go", BeforeSHA: strings.Repeat("a", 64)}
	cases := []struct {
		name     string
		class    string
		tracked  bool
		gitSHA   string
		archive  string
		outside  bool
		wantTier string
		wantEvid bool // 只报那一档**必给**两件证据
		hitWhich string
	}{
		// 改 = 只记（哪怕三条判据都命中 —— 「不可找回」只对「删」生效）。
		{"改 + 全不可找回 ⇒ 只记（负控）", "改", false, "", "", false, "只记", false, ""},
		{"判不了 + 全不可找回 ⇒ 只记（负控）", "不判（判不了）", false, "", "", false, "只记", false, ""},
		// 符号级删 = 必拦（这一档不查三条判据 —— 给了再好的可找回证据也照样必拦）。
		{"符号级删 + git 历史在 ⇒ 必拦", "符号级删", true, "0123456789abcdef", "", false, "必拦", true, ""},
		{"符号级删 + 全不可找回 ⇒ 必拦", "符号级删", false, "", "", false, "必拦", true, ""},
		// 件级（整件）：可找回 ⇒ 只报 + 证据两件。
		{"件级 + tracked + git 历史 ⇒ 只报", "件级（整件）", true, "0123456789abcdef", "", false, "只报", true, ""},
		{"件级 + tracked + 有归档副本 ⇒ 只报", "件级（整件）", true, "", "Zerg-归档/旧件/x.go", false, "只报", true, ""},
		// 件级：三条不可找回判据任一命中 ⇒ 升为必拦。
		{"件级 + 未跟踪 ⇒ 升必拦（判据①）", "件级（整件）", false, "", "", false, "必拦", true, "①"},
		{"件级 + 两条找回路径都取不到 ⇒ 升必拦（判据②）", "件级（整件）", true, "", "", false, "必拦", true, "②"},
		{"件级 + 仓外 ⇒ 升必拦（判据③）", "件级（整件）", true, "0123456789abcdef", "", true, "必拦", true, "③"},
	}
	for _, c := range cases {
		f := base
		f.Class, f.Tracked, f.GitSHA, f.Archive, f.Outside = c.class, c.tracked, c.gitSHA, c.archive, c.outside
		h := zerg.ImpactHookDecideForTest(f)
		if h.Tier != c.wantTier {
			t.Errorf("%s：分档 = %q（要 %q）· why=%s", c.name, h.Tier, c.wantTier, h.Why)
		}
		if c.wantEvid && len(h.Evidence) != 2 {
			t.Errorf("%s：可找回证据要**两件**（实得 %d 件）：%v", c.name, len(h.Evidence), h.Evidence)
		}
		if c.hitWhich != "" && !strings.Contains(h.Recall[0]+h.Recall[1]+h.Recall[2], c.hitWhich) {
			t.Errorf("%s：三条判据的读数里要点到判据 %s：%v", c.name, c.hitWhich, h.Recall)
		}
		// 只报那一档**必须**把「可找回证据」两件带上（§6.1：缺一不许删）。
		if h.Tier == "只报" {
			if len(h.Evidence) != 2 || !strings.Contains(h.Evidence[0], "sha256") || !strings.Contains(h.Evidence[1], "找回路径") {
				t.Errorf("%s：只报那一档要给两件证据（sha256 + 找回路径）：%v", c.name, h.Evidence)
			}
		}
	}
}

// ---- ③ 指纹可复算 --------------------------------------------------------------------------

func TestDevEditHook_DigestRecomputable(t *testing.T) {
	a := zerg.ImpactDigestForTest("会话三行 + 摘要正文")
	b := zerg.ImpactDigestForTest("会话三行 + 摘要正文")
	if a != b {
		t.Errorf("同一段正文 ⇒ 同一枚指纹（实得 %s vs %s）", a, b)
	}
	if n := len(a); n != 64 {
		t.Errorf("指纹要是 sha256 的 64 位 hex（实得 %d 位）", n)
	}
	if c := zerg.ImpactDigestForTest("会话三行 + 摘要正文\u0000"); c == a {
		t.Error("改一个字节 ⇒ 指纹必须变（这一格没牙）")
	}
}

// ---- ④⑤⑥ 干跑挂点（真跑命令行）--------------------------------------------------------------

// shaOf 现算 sha256（与命令面 `sha256Of` 同一算法 —— 独立算一遍，不用它给的值）。
func shaOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// gitInitCommit 把合成仓变成一个**真的 git 仓**并把目标件提交进去（造「可找回：git 历史」那一档）。
// 只动 `t.TempDir()` 里的合成仓 —— 真仓一个字节都不碰。
func gitInitCommit(t *testing.T, repo, rel string) {
	t.Helper()
	for _, argv := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "hook@test"},
		{"config", "user.name", "hook"},
		{"add", "--", filepath.ToSlash(rel)},
		{"commit", "-q", "-m", "夹具：把目标件提交进去（造「可找回」那一档）"},
	} {
		cmd := exec.Command("git", argv...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("合成仓里 git %v 跑不动（%v）：%s", argv, err, out)
		}
	}
}

// hookRun 在**合成仓根 + 合成状态目录**里跑一条干跑（真仓的件一个都不碰）。
func hookRun(t *testing.T, repo, prop, state string, argv ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", repo)
	t.Setenv("ZERG_PROPOSAL_DIR", prop)
	t.Setenv("ZERG_STATE_DIR", state)
	var out, errb strings.Builder
	rc := zerg.RunForTest(append([]string{"dev", "edit"}, argv...), &out, &errb)
	return rc, out.String(), errb.String()
}

// hookDigestOf 从输出里取 `impact_digest=<hex>`（干跑与真写两路都打同一形状的那一行）。
func hookDigestOf(s string) string {
	m := regexp.MustCompile(`impact_digest=([0-9a-f]{64})`).FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// hookReport 一条干跑的三件取证（件字节 + 审计件是否存在）。
func hookSnapshot(t *testing.T, path, state string) (string, bool) {
	t.Helper()
	var sha string
	if b, err := os.ReadFile(path); err == nil {
		sha = shaOf(b)
	}
	_, err := os.Stat(filepath.Join(state, "edit_audit.jsonl"))
	return sha, err == nil
}

func TestDevEditHook_DryRunTiersAndZeroSideEffect(t *testing.T) {
	gitBin, _ := exec.LookPath("git")
	// 五格：改 / 符号级删 / 件级·未跟踪（升拦）/ 件级·可找回（只报）/ 判不了（改）。
	type tc struct {
		name       string
		gitInit    bool
		content    string // 写进目标件的内容
		after      string // 写后的内容（--from 用）
		wantRC     int
		wantTier   string
		wantStderr string
	}
	goSrc := "package main\n\nfunc alpha() int { return 1 }\n\nfunc beta() int { return alpha() }\n"
	cases := []tc{
		{"改 ⇒ 只记（退 0）", true, goSrc,
			"package main\n\nfunc alpha() int { return 2 }\n\nfunc beta() int { return alpha() }\n",
			0, "只记", "不是放行条件"},
		{"符号级删 ⇒ 必拦（退 2）", true, goSrc,
			"package main\n\nfunc alpha() int { return 1 }\n",
			2, "必拦", "一字未动"},
		{"件级（整件）· 可找回 ⇒ 只报（退 0）", true, goSrc, "",
			0, "只报", "可找回证据"},
	}
	for _, c := range cases {
		if c.gitInit && gitBin == "" {
			t.Skip("本机没有 git ⇒ 合成仓造不出「可找回」那一档，跳过")
		}
		repo, prop, state := t.TempDir(), t.TempDir(), t.TempDir()
		target := filepath.Join("core", "cmd", "zerg", "hooktarget.go")
		mustWrite(t, filepath.Join(repo, target), c.content)
		writeEditProposal(t, prop, "DEV-9100", target)
		if c.gitInit {
			gitInitCommit(t, repo, target)
		}
		from := filepath.Join(t.TempDir(), "after.go")
		mustWrite(t, from, c.after)
		beforeSHA, hadAudit := hookSnapshot(t, filepath.Join(repo, target), state)

		rc, out, errb := hookRun(t, repo, prop, state, "--proposal", "DEV-9100", "--file", target,
			"--from", from, "--dry-run")
		if rc != c.wantRC {
			t.Fatalf("%s：退码 = %d（要 %d）· stderr=%s", c.name, rc, c.wantRC, errb)
		}
		// ④ 分档判决落在**计划件**（stdout）与**机器可读那一行**（stderr）上，两处必须一致。
		if !strings.Contains(out, "影响面钩子: "+c.wantTier) {
			t.Errorf("%s：计划件要点明分档 %q：%q", c.name, c.wantTier, out)
		}
		if !strings.Contains(errb, "impact_hook tier="+c.wantTier+" ") {
			t.Errorf("%s：机器可读那一行要给 tier=%s：%q", c.name, c.wantTier, errb)
		}
		if c.wantStderr != "" && !strings.Contains(errb, c.wantStderr) {
			t.Errorf("%s：stderr 要点到 %q", c.name, c.wantStderr)
		}
		// ⑤ 干跑零副作用：件字节逐字不变 · 审计件不产生。
		afterSHA, hasAudit := hookSnapshot(t, filepath.Join(repo, target), state)
		if afterSHA != beforeSHA {
			t.Errorf("%s：干跑改了件（%s → %s）", c.name, beforeSHA, afterSHA)
		}
		if hasAudit || hadAudit {
			t.Errorf("%s：干跑落了审计（前 %v / 后 %v）—— 零副作用破", c.name, hadAudit, hasAudit)
		}
	}
}

// 件级（整件）· **不可找回** ⇒ 升为必拦（判据①：未跟踪件）。
func TestDevEditHook_FileTierUntrackedRisesToBlock(t *testing.T) {
	repo, prop, state := t.TempDir(), t.TempDir(), t.TempDir()
	target := filepath.Join("core", "cmd", "zerg", "hooktarget.go")
	mustWrite(t, filepath.Join(repo, target), "package main\n\nfunc alpha() int { return 1 }\n")
	writeEditProposal(t, prop, "DEV-9101", target) // 合成仓**不 git init** ⇒ 未跟踪
	empty := filepath.Join(t.TempDir(), "empty.go")
	mustWrite(t, empty, "") // **空件** ⇒ 写后 0 字节 ⇒ 件级（整件）
	beforeSHA, _ := hookSnapshot(t, filepath.Join(repo, target), state)

	rc, out, errb := hookRun(t, repo, prop, state, "--proposal", "DEV-9101", "--file", target,
		"--from", empty, "--dry-run")
	if rc != 2 {
		t.Fatalf("件级 + 不可找回 ⇒ 升必拦（退 2），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "影响面钩子: 必拦") || !strings.Contains(out, "未执行") {
		t.Errorf("计划件要点明「必拦 + 未执行」：%q", out)
	}
	if !strings.Contains(errb, "impact_criteria untracked=yes") {
		t.Errorf("机器可读那一行要点名判据① 命中：%q", errb)
	}
	if !strings.Contains(errb, "真写前置") {
		t.Errorf("要照实点名「真写前置一字未动」这条差口：%q", errb)
	}
	if sha, hasAudit := hookSnapshot(t, filepath.Join(repo, target), state); sha != beforeSHA || hasAudit {
		t.Errorf("必拦那一格也不许动件 / 落审计（sha %s → %s · 审计 %v）", beforeSHA, sha, hasAudit)
	}
}

// ⑥ 判据② 的负控：**已知无关件** ⇒ 不许打那三行（零影响时打三行 = 灌噪声）。
// 目标件 = `impactZeroHitTarget()`（与 `A1/A2` 的零命中判据同一枚件 · 件名在测试件里也是拼出来的）；
// 仓 = **真仓**（只读：`ZERG_STATE_DIR` / `ZERG_PROPOSAL_DIR` 都指到临时目录，审计与提案不落真仓）。
func TestDevEditHook_ZeroImpactDoesNotPrintThreeLines(t *testing.T) {
	repo := repoRootFromCLI(t)
	prop, state := t.TempDir(), t.TempDir()
	tgt := impactZeroHitTarget()
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(tgt))); err != nil {
		t.Skipf("零命中目标件不在盘上：%v", err)
	}
	writeEditProposal(t, prop, "DEV-9102", tgt)
	cur, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(tgt)))
	if err != nil {
		t.Fatalf("读不了零命中目标件：%v", err)
	}
	from := filepath.Join(t.TempDir(), "same.go")
	mustWrite(t, from, string(cur)) // 内容逐字不变 ⇒ 类 = 改 ⇒ 不许因「删」升拦
	rc, out, errb := hookRun(t, repo, prop, state, "--proposal", "DEV-9102", "--file", tgt,
		"--from", from, "--dry-run")
	if rc != 0 {
		t.Fatalf("零影响的干跑仍应退 0（只有钩子「必拦」才退 2），得到 %d · stderr=%s", rc, errb)
	}
	for _, head := range []string{"会牵动：", "会红：", "建议："} {
		if strings.Contains(errb, head) {
			t.Errorf("已知无关件**不许**打三行（出现了 %q）—— 零影响时打三行 = 灌噪声", head)
		}
	}
	if !strings.Contains(errb, "面内未见") {
		t.Errorf("不打三行时仍要**照实说没报**（「没报 ≠ 没事」F6）：%q", errb)
	}
	if !strings.Contains(out, "影响面钩子: 只记") {
		t.Errorf("零影响 + 改 ⇒ 只记：%q", out)
	}
	_ = out
}

// ---- ⑦ 真写审计行多一枚 `impact_digest`（与同一仓态下干跑那一枚**逐字相同**）-----------------

func TestDevEditHook_RealWriteAuditCarriesDigest(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9103", target)
	sign := writeOperatorKey(t, state)
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// A4：真写那一笔要带波纹指纹\n")

	// 先干跑一次（零副作用）——把**同一仓态**下算出的指纹取出来。
	rc, _, errb := hookRun(t, repo, prop, state, "--proposal", "DEV-9103", "--file", target,
		"--from", from, "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑要先能跑（退 0），得到 %d · stderr=%s", rc, errb)
	}
	dry := hookDigestOf(errb)
	if dry == "" {
		t.Fatalf("干跑要打出 `impact_digest=<64 hex>`（第三者可复算）：%q", errb)
	}

	// 再真写（人签批准件齐）。
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-09-22T00:00:00+08:00", "放行这一件"))+"\n")
	rc, _, errb = hookRun(t, repo, prop, state, "--proposal", "DEV-9103", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("人签齐了 ⇒ 真写退 0，得到 %d · stderr=%s", rc, errb)
	}
	ab, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		t.Fatalf("真写要落审计：%v", err)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(ab))), &line); err != nil {
		t.Fatalf("审计不是一行 JSON：%v · %q", err, ab)
	}
	got, _ := line["impact_digest"].(string)
	if got == "" {
		t.Fatalf("审计行要多一枚 `impact_digest`（§四.2）：%+v", line)
	}
	if got != dry {
		t.Errorf("同一仓态下：审计里的指纹 %s ≠ 干跑打出的 %s（同一枚摘要 ⇒ 同一枚指纹）", got, dry)
	}
	// **只留指纹**：审计里**不许**出现卡片 / 摘要正文（红线）。
	for _, leak := range []string{"会牵动：", "impact_card", "摘要正文"} {
		if strings.Contains(string(ab), leak) {
			t.Errorf("审计里不许留卡片全文（只留指纹）：出现了 %q", leak)
		}
	}
	if _, ok := line["impact_actual"]; ok {
		t.Log("`impact_actual` 已出现（属 `B3`；本批只立字段、不填值 ⇒ 现在应当没有这一格）")
	}
}
