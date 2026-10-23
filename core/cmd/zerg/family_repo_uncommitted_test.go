// family_repo_uncommitted_test.go —— 缺口 `GAP-20260926-78` 的判据（**进程内直调** `cmdRepoUncommitted`）。
//
// 为什么进程内直调、不走真二进制：本件落地时 `main.go` 正被另一路改 ⇒ 本命令**未登记**在命令树里，
// 走 `bin/zerg` 必报「未知命令」——那验的是登记、不是本面。登记后这里一行不用改（仍验同一支入口）。
//
// 钉住的六面：
//
//	① **逐件口径**：4 档来源（本笔 / 他路 / 存量 / 来源不明）+ 副产物判词 + mtime 分桶（含「读不到」档）；
//	② **真源对拍**：本面件数 == `git status --porcelain=v1 -uall` 件数（`??` 件一个不丢 —— 这正是
//	   `git diff --stat` 数不出的那半边）；
//	③ **不给 `--me` 就不许有「本笔」**（不猜「我」是谁）；
//	④ **零写操作**：`.git/index` 的 sha256 前后一致（`--no-optional-locks` 的机械落点）+ 工作树读数不变；
//	⑤ **写旗标一律拒**（rc=2）且拒时零副作用；
//	⑥ 退码：读到 0 件 ⇒ 0（**并逐字印 0**，不拿空输出当干净）· 非 git 目录 ⇒ 8 · 坏 `--since` ⇒ 2。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rucGit 在 root 里跑一条 git（失败即 Fatal —— 夹具坏了不许静默跳过）。
func rucGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v：%v（%s）", args, err, out)
	}
	return string(out)
}

// rucWrite 写一件（自动建父目录）。
func rucWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// rucSHA —— 该件的 sha256（夹具里要造 `after_sha256`，让账能「对得上现内容」）。
func rucSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// rucFixture —— 合成仓 + 审计件，返回（仓根 · 审计件路径）。
//
// 四件未提交，正好盖四档来源：
//
//	README.md    已跟踪改动 · 审计 10 分钟前（窗口内）        ⇒ 本笔（给 --me 时）/ 他路（不给）
//	notes.md     未跟踪新件 · 无审计行                        ⇒ 来源不明
//	bin/dump     未跟踪副产物（`bin/` 前缀）                  ⇒ 来源不明 · artifact=是
//	legacy.md    已跟踪删除 · 审计 48 小时前（窗口外）        ⇒ 存量 · mtime 读不到 ⇒ 桶「未知」
//	stale.md     已跟踪改动（今夜）· 审计末行 3 小时前        ⇒ 来源不明（**末行解释不了现内容**）
func rucFixture(t *testing.T) (root, audit string) {
	t.Helper()
	root = t.TempDir()
	rucWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n")
	rucWrite(t, filepath.Join(root, "README.md"), "base\n")
	rucWrite(t, filepath.Join(root, "legacy.md"), "legacy\n")
	rucGit(t, root, "init", "-q", "-b", "main")
	rucGit(t, root, "add", "-A")
	rucGit(t, root, "-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "-m", "base")

	// 五件未提交
	rucWrite(t, filepath.Join(root, "README.md"), "changed\n")
	rucWrite(t, filepath.Join(root, "stale.md"), "stale-but-touched-tonight\n")
	rucWrite(t, filepath.Join(root, "notes.md"), "note\n")
	rucWrite(t, filepath.Join(root, "bin", "dump"), "binary\n")
	if err := os.Remove(filepath.Join(root, "legacy.md")); err != nil {
		t.Fatal(err)
	}
	// `stale.md` 要「件比账新」：先把它的 mtime 钉到 3 小时前的相反方向 —— 这里不造 mtime，
	// 靠审计行落在 3 小时前（件是刚写的）⇒ 末行必然早于 mtime。

	// 审计件：只认带 `file` 的行（口径①），末行 = `at` 最大那条（口径②）
	now := time.Now()
	audit = filepath.Join(t.TempDir(), "edit_audit.jsonl")
	var b strings.Builder
	readmeSHA := rucSHA(t, filepath.Join(root, "README.md"))
	for _, ln := range []map[string]string{
		{"at": now.Add(-48 * time.Hour).Format(time.RFC3339), "event": "edit", "file": "legacy.md",
			"proposal": "DEV-8000", "by": "Hermes (AI)", "mode": "replace", "after_sha256": "deadbeef"},
		// 同一件的**更早**一条（sha 与现内容**不同**）：判归属不能取它，即使它时刻也在窗口内
		{"at": now.Add(-30 * time.Hour).Format(time.RFC3339), "event": "edit", "file": "README.md",
			"proposal": "DEV-7000", "by": "Hermes (AI)", "mode": "replace", "after_sha256": "0badc0de"},
		// 与件同时、且 sha 对得上现内容（真 `dev edit` 的形态：先写件、后追加一行）
		{"at": now.Format(time.RFC3339), "event": "edit", "file": "README.md",
			"proposal": "DEV-9001", "by": "Hermes (AI)", "mode": "replace", "after_sha256": readmeSHA},
		// 件今夜被改过，而账里那条的 sha **对不上**现内容 ⇒ 账解释不了这堆字节
		{"at": now.Add(-3 * time.Hour).Format(time.RFC3339), "event": "edit", "file": "stale.md",
			"proposal": "DEV-6000", "by": "Hermes (AI)", "mode": "replace", "after_sha256": "feedface"},
		// 不带 `file` 的行（缺口账那批的形态）：不许被当成本工作树的证据
		{"at": now.Format(time.RFC3339), "event": "gap_ledger_written", "gap_id": "GAP-20260926-78",
			"gap_ledger_path": "scripts/gates/gap-ledger.jsonl"},
	} {
		raw, err := json.Marshal(ln)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(append(raw, '\n'))
	}
	rucWrite(t, audit, b.String())
	t.Setenv("ZERG_EDIT_AUDIT", audit)
	t.Setenv("ZERG_REPO", root)
	return root, audit
}

// rucRun 进程内跑一次本面，返回 rc / stdout / stderr / 解码后的 items（无 jsonGiven 时为 nil）。
func rucRun(t *testing.T, root string, jsonGiven bool, fields []string, tail ...string) (int, string, string, []map[string]string, map[string]any) {
	t.Helper()
	var out, errb bytes.Buffer
	inv := &invocation{
		path:      []string{"repo", "uncommitted"},
		orig:      append([]string{"repo", "uncommitted"}, tail...),
		jsonGiven: jsonGiven,
		fields:    fields,
		kv:        map[string][]string{},
	}
	// 只解析本面认的三枚值旗标（`--json` 的字段已由 fields 直接给）
	for i := 0; i < len(tail); i++ {
		switch tail[i] {
		case "--root", "--since", "--me", "--audit":
			if i+1 < len(tail) {
				inv.kv[tail[i]] = append(inv.kv[tail[i]], tail[i+1])
				i++
			}
		}
	}
	rc := cmdRepoUncommitted(inv, &out, &errb)
	if !jsonGiven {
		return rc, out.String(), errb.String(), nil, nil
	}
	var env struct {
		Schema string              `json:"schema"`
		Kind   string              `json:"kind"`
		Items  []map[string]string `json:"items"`
		Meta   map[string]any      `json:"meta"`
	}
	if s := strings.TrimSpace(out.String()); s != "" {
		if err := json.Unmarshal([]byte(s), &env); err != nil {
			t.Fatalf("`--json` 不是合法包封：%v（%s）", err, out.String())
		}
		if env.Schema != "zerg/v1" || env.Kind != "RepoUncommitted" {
			t.Fatalf("包封六键不对：schema=%q kind=%q", env.Schema, env.Kind)
		}
	}
	return rc, out.String(), errb.String(), env.Items, env.Meta
}

// rucByPath 把 items 摊成 路径 → 行。
func rucByPath(items []map[string]string) map[string]map[string]string {
	m := map[string]map[string]string{}
	for _, it := range items {
		m[it["path"]] = it
	}
	return m
}

// TestRepoUncommitted_ClassifyBySourceAndMtime —— 四档来源 + 副产物 + 分桶（含「读不到」档）。
func TestRepoUncommitted_ClassifyBySourceAndMtime(t *testing.T) {
	root, _ := rucFixture(t)
	rc, _, errb, items, _ := rucRun(t, root, true, repoUncommittedFields, "--root", root, "--since", "2h", "--me", "DEV-9001")
	if rc != 0 {
		t.Fatalf("读到就退 0（有没有未提交件都是读数，不是错）；得到 %d · stderr=%s", rc, errb)
	}
	got := rucByPath(items)
	if len(items) != 5 {
		t.Fatalf("应 5 件未提交，得到 %d：%v", len(items), items)
	}

	// ① 本笔：审计末行 10 分钟前（不是 30 小时前那条）+ --me 命中
	if r := got["README.md"]; r == nil {
		t.Fatalf("缺 README.md：%v", items)
	} else {
		if r["source"] != "本笔" {
			t.Errorf("README.md 来源 = %q（要 本笔 · 窗口内 + --me 命中）", r["source"])
		}
		if r["proposal"] != "DEV-9001" {
			t.Errorf("README.md proposal = %q（要**产出当前字节**那条 DEV-9001，不是更早的 DEV-7000）", r["proposal"])
		}
		if r["kind"] != "改" || r["status"] != " M" {
			t.Errorf("README.md kind/status = %q/%q（要 改/` M`）", r["kind"], r["status"])
		}
		if r["bucket"] != "<1h" {
			t.Errorf("README.md 桶 = %q（要 <1h）", r["bucket"])
		}
		if !strings.Contains(r["evidence"], "DEV-9001") || !strings.Contains(r["evidence"], "Hermes (AI)") {
			t.Errorf("README.md 证据没点名「谁」：%q", r["evidence"])
		}
	}

	// ② 来源不明：无审计行 + mtime 在窗口内
	if r := got["notes.md"]; r == nil {
		t.Fatalf("缺 notes.md：%v", items)
	} else if r["source"] != "来源不明" || r["kind"] != "未跟踪" {
		t.Errorf("notes.md = %q/%q（要 来源不明/未跟踪）", r["source"], r["kind"])
	}

	// ③ 副产物判词：`bin/` 前缀
	if r := got["bin/dump"]; r == nil {
		t.Fatalf("缺 bin/dump：%v", items)
	} else {
		if r["artifact"] != "是" {
			t.Errorf("bin/dump artifact = %q（要 是）", r["artifact"])
		}
		if !strings.Contains(r["kind"], "副产物") {
			t.Errorf("bin/dump kind = %q（要带 ·副产物）", r["kind"])
		}
	}

	// ③½ 账里有该件、但没有一条 sha 对得上现内容 ⇒ 不许落「存量」蒙过去
	if r := got["stale.md"]; r == nil {
		t.Fatalf("缺 stale.md：%v", items)
	} else {
		if r["source"] != "来源不明" {
			t.Errorf("stale.md 来源 = %q（要 来源不明 · 账解释不了现内容）", r["source"])
		}
		if !strings.Contains(r["evidence"], "一条都对不上") {
			t.Errorf("stale.md 证据没写清「账里没有一条产出当前字节」：%q", r["evidence"])
		}
	}

	// ④ 存量 + mtime 读不到（删除件）：不许拿 0 顶替
	if r := got["legacy.md"]; r == nil {
		t.Fatalf("缺 legacy.md：%v", items)
	} else {
		if r["source"] != "存量" {
			t.Errorf("legacy.md 来源 = %q（要 存量 · 审计在窗口外）", r["source"])
		}
		if r["bucket"] != "未知" {
			t.Errorf("legacy.md 桶 = %q（要 未知 · 已删除件无 mtime）", r["bucket"])
		}
		if r["mtime"] != "（读不到）" {
			t.Errorf("legacy.md mtime = %q（要逐字「（读不到）」）", r["mtime"])
		}
		if _, has := r["age_min"]; has {
			t.Errorf("legacy.md 不该有 age_min（缺值整键略去，不许落 0）：%v", r)
		}
	}
}

// TestRepoUncommitted_NoMeMeansNoSelf —— 不给 `--me` ⇒ `本笔` 档恒 0（不猜「我」是谁）。
func TestRepoUncommitted_NoMeMeansNoSelf(t *testing.T) {
	root, _ := rucFixture(t)
	rc, out, errb, items, _ := rucRun(t, root, true, []string{"path", "source"}, "--root", root, "--since", "2h")
	if rc != 0 {
		t.Fatalf("rc=%d · stderr=%s", rc, errb)
	}
	for _, it := range items {
		if it["source"] == "本笔" {
			t.Errorf("没给 --me 却出现「本笔」：%v", it)
		}
	}
	if got := rucByPath(items)["README.md"]["source"]; got != "他路" {
		t.Errorf("README.md 来源 = %q（无 --me ⇒ 窗口内审计行落 他路）", got)
	}
	if !strings.Contains(out, "他路") {
		t.Errorf("机器面没把 他路 落出来：%s", out)
	}
	// 人面要把这条口径明写出来（不然「本笔 0」会被读成「我没改过」）
	rc2, out2, _, _, _ := rucRun(t, root, false, nil, "--root", root, "--since", "2h")
	if rc2 != 0 {
		t.Fatalf("人面 rc=%d", rc2)
	}
	if !strings.Contains(out2, "未给 --me") {
		t.Errorf("人面没写「未给 --me ⇒ 本笔档恒 0」这条口径：\n%s", out2)
	}
}

// TestRepoUncommitted_CountMatchesGitTruth —— 真源对拍：件数 == `git status --porcelain=v1 -uall`。
//
// 这条就是缺口正文那半边病：`git diff --stat` 对 `??` 件恒空（本仓实测 29 件一件不列），
// 本面必须**一件不丢**。
func TestRepoUncommitted_CountMatchesGitTruth(t *testing.T) {
	root, _ := rucFixture(t)
	_, _, _, items, meta := rucRun(t, root, true, []string{"path", "source"}, "--root", root, "--since", "2h")
	truth := rucGit(t, root, "status", "--porcelain=v1", "-uall")
	want := 0
	for _, ln := range strings.Split(truth, "\n") {
		if strings.TrimSpace(ln) != "" {
			want++
		}
	}
	if len(items) != want {
		t.Fatalf("件数 %d ≠ `git status -uall` 的 %d（少报/多报都是病）", len(items), want)
	}
	// `git diff --stat` 的负控：未跟踪件它一件也数不出来（这正是要新面的理由）
	stat := rucGit(t, root, "diff", "--stat")
	if strings.Contains(stat, "notes.md") || strings.Contains(stat, "bin/dump") {
		t.Errorf("前提变了：`git diff --stat` 居然列了未跟踪件（%q）⇒ 缺口前提需重新取证", stat)
	}
	if _, ok := meta["by_source"]; !ok {
		t.Errorf("meta 缺 by_source 分桶读数：%v", meta)
	}
	if _, ok := meta["by_bucket"]; !ok {
		t.Errorf("meta 缺 by_bucket 分桶读数：%v", meta)
	}
}

// TestRepoUncommitted_ReadOnlyAndWriteRefused —— 零写操作（`.git/index` 不变）+ 写旗标拒。
func TestRepoUncommitted_ReadOnlyAndWriteRefused(t *testing.T) {
	root, _ := rucFixture(t)
	idx := filepath.Join(root, ".git", "index")
	rucGit(t, root, "status", "--porcelain=v1") // 先把可能的索引刷新做掉（否则测的是 git 自己）
	before, err := os.ReadFile(idx)
	if err != nil {
		t.Fatal(err)
	}
	beforePorcelain := rucGit(t, root, "status", "--porcelain=v1", "-uall")

	for _, tail := range [][]string{
		{"--root", root, "--since", "2h"},
		{"--root", root, "--since", "2h", "--me", "DEV-9001"},
	} {
		if rc, _, errb, _, _ := rucRun(t, root, true, []string{"path", "source"}, tail...); rc != 0 {
			t.Fatalf("%v rc=%d · stderr=%s", tail, rc, errb)
		}
		if rc, _, errb, _, _ := rucRun(t, root, false, nil, tail...); rc != 0 {
			t.Fatalf("%v（人面）rc=%d · stderr=%s", tail, rc, errb)
		}
	}

	after, err := os.ReadFile(idx)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Errorf("`.git/index` 被改了（%s → %s）⇒ 不是零写操作（`--no-optional-locks` 没落实）",
			hex.EncodeToString(before[:8]), hex.EncodeToString(after[:8]))
	}
	if p := rucGit(t, root, "status", "--porcelain=v1", "-uall"); p != beforePorcelain {
		t.Errorf("工作树读数变了：\n前 %q\n后 %q", beforePorcelain, p)
	}

	// 写旗标一律拒（与 repo status / repo diffstat 同守），且拒时一个字节都不改
	rc, out, errb, _, _ := rucRun(t, root, false, nil, "--root", root, "--commit", "-m", "x")
	if rc != 2 {
		t.Fatalf("写旗标 ⇒ 退 2，得到 %d · out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(errb, "拒") {
		t.Errorf("拒的时候没说明白：%q", errb)
	}
	if after2, _ := os.ReadFile(idx); sha256.Sum256(after2) != sha256.Sum256(before) {
		t.Errorf("拒的路径也动了 `.git/index`")
	}
}

// TestRepoUncommitted_ZeroAndUnreadable —— 0 件逐字印 0（空输出 ≠ 干净）· 非 git 目录退 8。
func TestRepoUncommitted_ZeroAndUnreadable(t *testing.T) {
	// 干净仓
	clean := t.TempDir()
	rucWrite(t, filepath.Join(clean, "core", "internal", "version", "version.go"), "package version\n")
	rucGit(t, clean, "init", "-q", "-b", "main")
	rucGit(t, clean, "add", "-A")
	rucGit(t, clean, "-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "-m", "base")
	t.Setenv("ZERG_REPO", clean)
	rc, _, errb, items, meta := rucRun(t, clean, true, []string{"path", "source"}, "--root", clean)
	if rc != 0 {
		t.Fatalf("0 件 ⇒ 退 0（不是错），得到 %d · stderr=%s", rc, errb)
	}
	if len(items) != 0 {
		t.Errorf("干净仓却列出 %d 件：%v", len(items), items)
	}
	if meta["count"] == nil {
		t.Errorf("包封 meta 缺 count（六键自带那一格）：%v", meta)
	}
	rc2, out2, _, _, _ := rucRun(t, clean, false, nil, "--root", clean)
	if rc2 != 0 {
		t.Fatalf("人面 rc=%d", rc2)
	}
	if !strings.Contains(out2, "0 件") {
		t.Errorf("0 件的读数没逐字印出来（空输出会被读成「干净」或「读不到」）：\n%s", out2)
	}

	// 非 git 目录 ⇒ 8（读不到既不是干净也不是 0 件）
	plain := t.TempDir()
	t.Setenv("ZERG_REPO", plain)
	rc3, out3, errb3, _, _ := rucRun(t, plain, false, nil, "--root", plain)
	if rc3 != 8 {
		t.Errorf("非 git 目录 ⇒ 退 8，得到 %d · out=%q err=%q", rc3, out3, errb3)
	}
	if strings.Contains(out3, "0 件") {
		t.Errorf("读不到却印了「0 件」⇒ 把「判不了」当绿：%q", out3)
	}

	// 坏 --since ⇒ 2；未知字段 ⇒ 2
	if rc4, _, _, _, _ := rucRun(t, clean, false, nil, "--root", clean, "--since", "zzz"); rc4 != 2 {
		t.Errorf("坏 --since ⇒ 退 2，得到 %d", rc4)
	}
	if rc5, _, _, _, _ := rucRun(t, clean, true, []string{"path", "bogus_field"}, "--root", clean); rc5 != 2 {
		t.Errorf("未知字段 ⇒ 退 2，得到 %d", rc5)
	}
}

// TestRepoUncommitted_DeterministicSelection —— 同一判据跑两遍逐字一致（§九 M11）。
//
// 只比**稳定字段**：`age_min` 会跨分钟变（那是真读数，不是不确定），故不进这条对拍。
func TestRepoUncommitted_DeterministicSelection(t *testing.T) {
	// ★ 本笔（2026-10-23 · 提交闸里稳定红、单跑绿）：本用例要求「两跑之间一切读数不变」，
	//   而 `repo uncommitted` 的来源判定会读**审计账**（ZERG_STATE_DIR 下）。提交闸跑时那个旋钮
	//   指向真状态目录，两跑之间账会变（闸自身在写）⇒ 读数必然不同。⇒ 本用例把状态目录也隔离掉。
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	root, _ := rucFixture(t)
	fields := []string{"path", "status", "kind", "artifact", "bucket", "source", "proposal", "evidence"}
	_, out1, _, _, _ := rucRun(t, root, true, fields, "--root", root, "--since", "2h", "--me", "DEV-9001")
	_, out2, _, _, _ := rucRun(t, root, true, fields, "--root", root, "--since", "2h", "--me", "DEV-9001")
	if out1 != out2 {
		t.Errorf("两跑 `--json` 逐字不同：\n① %s\n② %s", out1, out2)
	}
}

// TestRepoUncommitted_MultiLedgerMerge —— 多账合并：`--audit` 那枚账里更新的行赢（跨 lane 归位）。
//
// 为什么要有这一面：每枚 lane 把自己的 `edit_audit.jsonl` 落在 `<lane>/state/` 下 ⇒ 只看一枚账，
// 别的 lane 的改动必然全判「来源不明」。`--audit` 是让人**具名**把别处的账并进来的口子。
func TestRepoUncommitted_MultiLedgerMerge(t *testing.T) {
	root, _ := rucFixture(t)
	now := time.Now()
	other := filepath.Join(t.TempDir(), "lane-edit_audit.jsonl")
	raw, err := json.Marshal(map[string]string{
		"at": now.Format(time.RFC3339), "event": "edit", "file": "notes.md",
		"proposal": "DEV-9999", "by": "别的 lane", "mode": "replace",
		"after_sha256": rucSHA(t, filepath.Join(root, "notes.md")),
	})
	if err != nil {
		t.Fatal(err)
	}
	rucWrite(t, other, string(raw)+"\n")

	// 不认领：窗口内、但不是 --me ⇒ 他路（证据点名「谁的」）
	rc, _, errb, items, meta := rucRun(t, root, true, []string{"path", "source", "proposal", "by", "evidence"},
		"--root", root, "--since", "2h", "--audit", other)
	if rc != 0 {
		t.Fatalf("rc=%d · stderr=%s", rc, errb)
	}
	n := rucByPath(items)["notes.md"]
	if n["source"] != "他路" {
		t.Errorf("notes.md 来源 = %q（要 他路 · 第二枚账里窗口内的行）", n["source"])
	}
	if n["proposal"] != "DEV-9999" || !strings.Contains(n["evidence"], "别的 lane") {
		t.Errorf("notes.md 证据没取到第二枚账的末行：%v", n)
	}
	ledgers, _ := meta["audit_ledgers"].(map[string]any)
	if len(ledgers) != 2 {
		t.Errorf("meta.audit_ledgers 应 2 处，得到 %d：%v", len(ledgers), meta["audit_ledgers"])
	}

	// 认领之后 ⇒ 本笔
	_, _, _, items2, _ := rucRun(t, root, true, []string{"path", "source"},
		"--root", root, "--since", "2h", "--me", "DEV-9999", "--audit", other)
	if got := rucByPath(items2)["notes.md"]["source"]; got != "本笔" {
		t.Errorf("notes.md 来源 = %q（要 本笔 · --me 命中第二枚账的 proposal）", got)
	}
}

// TestRepoUncommitted_AuditUnreadableDegrades —— 账读不到 ⇒ 明说 + 降级（不许当绿、也不许硬编归属）。
func TestRepoUncommitted_AuditUnreadableDegrades(t *testing.T) {
	root, _ := rucFixture(t)
	t.Setenv("ZERG_EDIT_AUDIT", filepath.Join(t.TempDir(), "nope.jsonl"))
	rc, _, errb, items, meta := rucRun(t, root, true, []string{"path", "source", "evidence"},
		"--root", root, "--since", "2h", "--me", "DEV-9001")
	if rc != 0 {
		t.Fatalf("账读不到不是「读不到工作树」⇒ 仍退 0（附警告）；得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "⚠") || !strings.Contains(errb, "审计账") {
		t.Errorf("账读不到没在 stderr 明说：%q", errb)
	}
	for _, it := range items {
		if it["source"] == "本笔" || it["source"] == "他路" {
			t.Errorf("账读不到却给出了具体归属（%v）", it)
		}
		if !strings.Contains(it["evidence"], "审计读不到") {
			t.Errorf("降级时证据列没明写「审计读不到」：%v", it)
		}
	}
	if meta["audit_state"] == "读到" {
		t.Errorf("meta.audit_state 谎报「读到」：%v", meta["audit_state"])
	}
}
