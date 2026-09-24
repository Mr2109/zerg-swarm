// cli_repo_status_shared_test.go —— `zerg repo status` 的**身份两格 + 共享件提示**成对判据
// （波19 序137 · 组4 §二.4 `W-56`（`研-禁:211` §六 栗②）· 与 `W-46` 同族（并行会话））。
//
// 判据栏逐字：**`repo status` 逐件出 `mtime` + `sha256`；共享件出提示行；
// 负控：件被别人改 ⇒ `sha256` 变且提示出现**。
//
// 本件把这条判据的每一格都钉住（夹具自建自管 · 全在 `t.TempDir()` 里 · 不碰真仓 / 不碰状态目录）：
//
//	① 正控 · 逐件身份两格：脏件 + 未跟踪件**逐条**带 `mtime`（RFC3339）与 `sha256`（64 位 hex），
//	   且与**另算一遍**的 `sha256` 逐字同（Go `crypto/sha256` 一路 ⟷ `shasum -a 256` 一路 = 两器对拍）；
//	② 正控 · 「件被别人改 ⇒ `sha256` 变」：改一件再跑 ⇒ 该件 `sha256` 与上一跑**不同**（成对负控见 ①b：
//	   不动的件两跑**逐字同**）；
//	③ 正控 · **共享件提示**（判据栏那半）：同一相对路径在两棵工作树里都脏 ⇒ **一行提示**
//	   （件名 + 各面 + **两枚各不相同的 `sha256`**）—— 「多枚会话共享的件」的机械形态；
//	④ 成对负控 · **不许恒出**：只有一棵树 / 两棵树脏的是**不同**路径 ⇒ 逐字「无」；
//	⑤ 负控 · **读不到的会话面不许当「无」**：点名 + 明写「可能漏」（最重的假绿就是把它读成「没有共享件」）；
//	⑥ 对界 · **字段面总得通**：全字段 `--json` 在**单仓面 · 多仓面 · 干净仓**三处都通（不许「未知字段」）；
//	⑦ 只读自证 · 另外**一棵**树的 `index` 与工作树一个字节未动（其余会话面走 `--no-optional-locks`）。
//
// 落点：`package main_test` ＋ `zerg.RunForTest` —— 跑的是**当前源码**的行为（不是盘上旧制品）；
// 与 `cli_repo_status_test.go` / `cli_repo_status_multi_test.go` 同一条路（`gitRepo` 夹具也取自那里）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// rsAllFields —— 本命令命令树登记的**全字段表**（判据⑥ 用它一把全请求）。
const rsAllFields = "head,branch,path,status,untracked,mtime,sha256"

// rsHex64 —— `sha256` 的**形态判据**（64 位小写 hex ⇒ 不许是「短哈希」「空串」「0 顶替」）。
var rsHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// rsHex64Any —— 从一行读数里**取**每一枚 64 位 hex（形态判据仍走上面那条带锚的）。
var rsHex64Any = regexp.MustCompile(`[0-9a-f]{64}`)

// rsSha 算一件的 sha256（**另算一遍**：与被测实现不同一路代码）。
func rsSha(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s：%v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// rsShaShasum 用**另一个器**（`shasum -a 256`）再算一遍 —— 两器逐字同才算数（判据①那一半）。
func rsShaShasum(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command("shasum", "-a", "256", path).Output()
	if err != nil {
		t.Skipf("本机没有 shasum（另一器拿不到 ⇒ 这一半跳过 · 不当绿）：%v", err)
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		t.Fatalf("`shasum -a 256 %s` 没给读数：%q", path, out)
	}
	return f[0]
}

// rsWithWorktree 建一个真 git 仓 + **一棵链接工作树**（一仓两「会话面」）。
// 会话面口径 = 本仓 + `git worktree list` 的其余工作树（一枚会话 = 一棵工作树）。
func rsWithWorktree(t *testing.T) (root, wt string) {
	t.Helper()
	root = gitRepo(t)
	wt = filepath.Join(t.TempDir(), "wt2")
	mustGit(t, root, "worktree", "add", "-q", "--detach", wt, "HEAD")
	return root, wt
}

// rsItems 跑一条 `repo status --json <全字段>` ⇒ (rc, items, stderr)。
func rsItems(t *testing.T, argv ...string) (int, []map[string]string, string) {
	t.Helper()
	return repoStatusJSON(t, argv...)
}

// ---- ① 正控 · 逐件身份两格（含两器对拍）------------------------------------------------------

func TestRepoStatusIdentity_PerFileMtimeAndSha256(t *testing.T) {
	root := gitRepo(t)
	mustWrite(t, filepath.Join(root, "README.md"), "changed\n")
	mustWrite(t, filepath.Join(root, "new.go"), "package main\n")

	rc, items, errb := rsItems(t, "repo", "status", "--root", root, "--json", rsAllFields)
	if rc != 1 {
		t.Fatalf("有脏件 ⇒ rc=1，得到 %d · stderr=%s", rc, errb)
	}
	want := map[string]string{
		"README.md": rsSha(t, filepath.Join(root, "README.md")),
		"new.go":    rsSha(t, filepath.Join(root, "new.go")),
	}
	seen := map[string]bool{}
	for _, it := range items {
		p := it["path"]
		if _, ok := want[p]; !ok {
			continue
		}
		seen[p] = true
		if !rsHex64.MatchString(it["sha256"]) {
			t.Errorf("%s 的 sha256 不是 64 位小写 hex：%q", p, it["sha256"])
		}
		if it["sha256"] != want[p] {
			t.Errorf("%s 的 sha256 与另算一遍不同：命令面 %q ⟷ 另算 %q", p, it["sha256"], want[p])
		}
		if got := rsShaShasum(t, filepath.Join(root, p)); got != it["sha256"] {
			t.Errorf("%s 的 sha256 与 `shasum -a 256` 不同：命令面 %q ⟷ shasum %q", p, it["sha256"], got)
		}
		if _, err := time.Parse(time.RFC3339, it["mtime"]); err != nil {
			t.Errorf("%s 的 mtime 不是 RFC3339：%q（%v）", p, it["mtime"], err)
		}
	}
	for p := range want {
		if !seen[p] {
			t.Errorf("脏件 %s 那一行没出现在 `--json` 的 items 里：%v", p, items)
		}
	}
}

// ---- ② 正控 + 成对负控 · 「件被别人改 ⇒ sha256 变」 / 「没动的件 ⇒ 两跑逐字同」-------------------

func TestRepoStatusIdentity_Sha256ChangesWhenFileIsChanged(t *testing.T) {
	root := gitRepo(t)
	mustWrite(t, filepath.Join(root, "README.md"), "v1\n")
	mustWrite(t, filepath.Join(root, "other.md"), "same\n")

	first := rsShaOf(t, root, "README.md")
	firstOther := rsShaOf(t, root, "other.md")

	// 「别人」改了同一件（同一棵树里的同一路径 —— 这正是要能看出来的一幕）。
	mustWrite(t, filepath.Join(root, "README.md"), "v2-by-someone-else\n")

	second := rsShaOf(t, root, "README.md")
	secondOther := rsShaOf(t, root, "other.md")

	if first == second {
		t.Errorf("件被改过 ⇒ sha256 **必变**，两跑却都是 %q（读数没跟着盘走）", first)
	}
	if firstOther != secondOther {
		t.Errorf("没被动过的件 ⇒ 两跑 sha256 应逐字同：%q ⟷ %q", firstOther, secondOther)
	}
	if second != rsSha(t, filepath.Join(root, "README.md")) {
		t.Errorf("第二跑的 sha256 与另算一遍不同：%q", second)
	}
}

// rsShaOf 跑一次 `repo status` 并取某件的 sha256（找不到即 Fatal —— 夹具坏了不许静默）。
func rsShaOf(t *testing.T, root, rel string) string {
	t.Helper()
	_, items, errb := rsItems(t, "repo", "status", "--root", root, "--json", rsAllFields)
	for _, it := range items {
		if it["path"] == rel {
			return it["sha256"]
		}
	}
	t.Fatalf("items 里没有 %s：%v（stderr=%s）", rel, items, errb)
	return ""
}

// ---- ③ 正控 · 共享件提示（判据栏那半）---------------------------------------------------------

func TestRepoStatusShared_OneLineWhenSamePathDirtyInTwoTrees(t *testing.T) {
	root, wt := rsWithWorktree(t)

	// 只读自证（判据⑦）要用的两样：另**一棵**树的 index 与那一件的内容。
	wtGitDir := strings.TrimSpace(mustGit(t, wt, "rev-parse", "--absolute-git-dir"))
	idx := filepath.Join(wtGitDir, "index")
	idxBefore, err := os.ReadFile(idx)
	if err != nil {
		t.Fatalf("读另一棵树的 index：%v", err)
	}

	mustWrite(t, filepath.Join(root, "README.md"), "changed-by-session-a\n")
	mustWrite(t, filepath.Join(wt, "README.md"), "changed-by-session-b\n")

	rc, _, errb := rsItems(t, "repo", "status", "--root", root, "--json", rsAllFields)
	if rc != 1 {
		t.Fatalf("两处未提交 ⇒ rc=1，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "会话面 2 个") {
		t.Errorf("会话面数不对（要 2：本仓 + 那一棵工作树）：stderr=%s", errb)
	}
	if !strings.Contains(errb, "★ 共享件提示：README.md") {
		t.Fatalf("同一件在两棵树里都脏 ⇒ **必须出一行共享件提示**：stderr=%s", errb)
	}
	// 提示行里那两枚 sha256：**各不相同**（这就是「件被别人改」在一条读数里的形态）。
	hint := ""
	for _, ln := range strings.Split(errb, "\n") {
		if strings.Contains(ln, "各面 sha256") {
			hint = ln
			break
		}
	}
	got := rsHex64Any.FindAllString(hint, -1)
	if len(got) != 2 {
		t.Fatalf("提示行应给**两枚** sha256，得到 %d 枚：%q", len(got), hint)
	}
	if got[0] == got[1] {
		t.Errorf("两棵树里的同一件内容不同 ⇒ 两枚 sha256 应不同：%q", hint)
	}
	a, b := rsSha(t, filepath.Join(root, "README.md")), rsSha(t, filepath.Join(wt, "README.md"))
	if !(got[0] == a && got[1] == b) {
		t.Errorf("提示行的两枚 sha256 与两棵树现算的对应不上：提示 %v · 本仓 %s · 另一棵 %s", got, a, b)
	}
	// 单仓面那一行也要有本仓这一件的身份（判据栏前半）。
	if rowsha := rsShaOf(t, root, "README.md"); rowsha != a {
		t.Errorf("单仓面那一行的 sha256 与本仓现算不同：%q ⟷ %q", rowsha, a)
	}
	// 只读自证：另一棵树的 index 与工作树一个字节未动（其余会话面走 `--no-optional-locks`）。
	idxAfter, err := os.ReadFile(idx)
	if err != nil {
		t.Fatalf("复读另一棵树的 index：%v", err)
	}
	if string(idxBefore) != string(idxAfter) {
		t.Errorf("另一棵树的 `.git/index` 被改了（其余会话面**必须**零副作用 ⇒ `--no-optional-locks` 没生效）")
	}
	if got := rsSha(t, filepath.Join(wt, "README.md")); got != b {
		t.Errorf("另一棵树那一件被改了：%q ⟷ %q", got, b)
	}
}

// ---- ④ 成对负控 · 不许恒出 ----------------------------------------------------------------

func TestRepoStatusShared_NegativesNoHint(t *testing.T) {
	// ④a 只有一棵树（没有别的会话面）⇒ 逐字「无」。
	solo := gitRepo(t)
	mustWrite(t, filepath.Join(solo, "README.md"), "x\n")
	_, _, errb := rsItems(t, "repo", "status", "--root", solo, "--json", rsAllFields)
	if !strings.Contains(errb, "会话面 1 个") || !strings.Contains(errb, "共享件提示：无") {
		t.Errorf("单会话面 ⇒ 应逐字「共享件提示：无」（不许恒出）：stderr=%s", errb)
	}
	if strings.Contains(errb, "★ 共享件提示") {
		t.Errorf("单会话面却出了逐件提示行：stderr=%s", errb)
	}

	// ④b 两棵树脏的是**不同**路径 ⇒ 也不是「共享件」。
	root, wt := rsWithWorktree(t)
	mustWrite(t, filepath.Join(root, "a-only-main.go"), "package main\n")
	mustWrite(t, filepath.Join(wt, "b-only-other.go"), "package main\n")
	_, _, errb = rsItems(t, "repo", "status", "--root", root, "--json", rsAllFields)
	if strings.Contains(errb, "★ 共享件提示") {
		t.Errorf("两棵树脏的是不同路径 ⇒ 不许出共享件提示（恒出即空转）：stderr=%s", errb)
	}
	if !strings.Contains(errb, "共享件提示：无") {
		t.Errorf("这一跑应逐字给「无」：stderr=%s", errb)
	}
}

// ---- ⑤ 负控 · 读不到的会话面不许当「无」 -----------------------------------------------------

func TestRepoStatusShared_UnreadableFaceIsNotGreen(t *testing.T) {
	root, wt := rsWithWorktree(t)
	mustWrite(t, filepath.Join(root, "README.md"), "main-side\n")
	// 把那一棵树挪走：`git worktree list` 还列着它，但读不到 ⇒ 照实印、**不当绿**。
	if err := os.RemoveAll(wt); err != nil {
		t.Fatalf("挪走夹具里那棵树：%v", err)
	}
	_, _, errb := rsItems(t, "repo", "status", "--root", root, "--json", rsAllFields)
	if !strings.Contains(errb, "会话面读不到") {
		t.Errorf("读不到的会话面**必须**点名（照实）：stderr=%s", errb)
	}
	if !strings.Contains(errb, "可能漏") {
		t.Errorf("读不到时**必须**明写「提示行可能漏」（不许静默当「无」）：stderr=%s", errb)
	}
}

// ---- ⑥ 对界 · 字段面总得通（单仓面 / 多仓面 / 干净仓三处）-----------------------------------

func TestRepoStatusFieldsFaceIsTotal(t *testing.T) {
	dirty := gitRepo(t)
	mustWrite(t, filepath.Join(dirty, "README.md"), "changed\n")
	clean := gitRepo(t)

	cases := []struct {
		name    string
		argv    []string
		wantRC  int
		wantLen int
	}{
		{"单仓面（脏）", []string{"repo", "status", "--root", dirty, "--json", rsAllFields}, 1, 1},
		{"干净仓", []string{"repo", "status", "--root", clean, "--json", rsAllFields}, 0, 1},
		{"多仓面", []string{"repo", "status", "--root", dirty, "--root", clean, "--json", rsAllFields}, 1, 2},
	}
	for _, c := range cases {
		rc, items, errb := rsItems(t, c.argv...)
		if rc != c.wantRC {
			t.Errorf("%s：rc=%d（要 %d）· stderr=%s", c.name, rc, c.wantRC, errb)
		}
		if len(items) != c.wantLen {
			t.Errorf("%s：items=%d 条（要 %d）：%v", c.name, len(items), c.wantLen, items)
		}
		for _, it := range items {
			for _, f := range strings.Split(rsAllFields, ",") {
				if _, ok := it[f]; !ok {
					t.Errorf("%s：某一行缺字段 %q（字段表是命令级的 ⇒ 哪一面都得通得了）：%v", c.name, f, it)
				}
			}
			if strings.Contains(errb, "未知字段") {
				t.Errorf("%s：退 2「未知字段」= 字段面时通时不通：stderr=%s", c.name, errb)
			}
		}
	}

	// 只作用面那半：多仓面**不是逐件的面**（一行一仓）⇒ 身份两格给 `—`（判据⑥ 的边界写死）。
	_, items, _ := rsItems(t, "repo", "status", "--root", dirty, "--root", clean, "--json", rsAllFields)
	for _, it := range items {
		if it["mtime"] != "—" || it["sha256"] != "—" {
			t.Errorf("多仓面的身份两格应为 `—`（逐件身份在单仓面）：%v", it)
		}
	}
}
