// cli_repo_status_multi_test.go —— `zerg repo status --root <仓根>…（≥2 次）` 的**成对判据**
// （波12 序111 · 组4 §二.2 `W-28` · `Q-028`（`研-影:94`）· `A7`（`研-归:49`）· `R-10`（`研-禁:123`））。
//
// 事项（任务单 §一 序111 逐字）：**两仓 `git status` 只读汇总** —— 一条命令出两仓 `HEAD + 脏件数`，
// **任何写旗标一律拒 `rc=2`**（留痕那半已在册 `G-42`/`G-43` ⇒ 本条只开「两仓汇总」这一半）。
// 判据（任务单该行「判据」栏逐字）：**一条只读命令出两仓 `HEAD + 脏件数`**；**带写旗标 ⇒ 拒（rc=2）**；
// **负控：两仓中一仓脏 ⇒ 该仓数 ≠ 0**。
//
// 本条钉五格（三正控 + 两负控）：
//
//	① 正控 · 两仓一行一仓：`--json` 的 `items` **恰好 2 条**（一仓一条），各自带 `head` / `branch`；
//	② 负控 · 一仓脏 ⇒ rc=1 **且那一仓的数 ≠ 0**（干净那一仓仍是 `clean` —— 数不是拍脑袋写死的）；
//	③ 正控 · 两仓全干净 ⇒ rc=0（同一个式子、换输入换输出 ⇒ 不是恒 1 也不是恒 0）；
//	④ 负控 · 写旗标一律拒 rc=2（七种形态 · 不看位置）**且仓一个字节没动**（HEAD 与 `--porcelain` 前后同值）；
//	⑤ 负控 · 任一 `--root` 不是 git 工作树 ⇒ rc=8（**读不到不许当绿**，也不拿它当 0）。
//
// 落点：`package main_test` ＋ `zerg.RunForTest` —— 跑的是**当前源码**的行为（不是盘上旧制品 `bin/zerg`），
// 与 `cli_help_all_test.go` / `repo_commit_test.go` 的 `TestRepoStatusRootFlag` 同一条路。
// 全程只读：夹具用 `t.TempDir()` 自管；不碰真仓、不碰状态目录、不写任何件。
package main_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// repoStatusJSON 起 `repo status` 抓 `--json` 的 items（两仓面用）。
func repoStatusJSON(t *testing.T, argv ...string) (int, []map[string]string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	var doc struct {
		Items []map[string]string `json:"items"`
	}
	body := out.String()
	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		if err := json.Unmarshal([]byte(body), &doc); err != nil {
			t.Fatalf("`--json` 的 items 解不开（%v）：%q", err, body)
		}
	}
	return rc, doc.Items, errb.String()
}

// itemByPath 按仓根取那一行（找不到即 Fatal —— 夹具坏了不许静默）。
func itemByPath(t *testing.T, items []map[string]string, root string) map[string]string {
	t.Helper()
	for _, it := range items {
		if it["path"] == root {
			return it
		}
	}
	t.Fatalf("两仓面里没有仓 %q 的那一行：%v", root, items)
	return nil
}

// TestRepoStatusMulti_TwoReposOneDirty —— ① 正控两行 + ② 负控「一仓脏 ⇒ 该仓数 ≠ 0」。
func TestRepoStatusMulti_TwoReposOneDirty(t *testing.T) {
	clean := gitRepo(t)
	dirty := gitRepo(t)
	// 弄脏第二仓：改一件（提交面）+ 加一件（未跟踪）⇒ 该仓未提交 **2** 处。
	mustWrite(t, filepath.Join(dirty, "README.md"), "changed\n")
	mustWrite(t, filepath.Join(dirty, "new.go"), "package main\n")

	rc, items, errb := repoStatusJSON(t, "repo", "status",
		"--root", clean, "--root", dirty, "--json", "head,branch,path,status,untracked")
	if rc != 1 {
		t.Fatalf("两仓里有一仓脏 ⇒ rc=1（「脏」是状态不是错），得到 %d · stderr=%s", rc, errb)
	}
	if len(items) != 2 {
		t.Fatalf("两仓面应**一行一仓**（2 条），得到 %d 条：%v", len(items), items)
	}
	for _, root := range []string{clean, dirty} {
		it := itemByPath(t, items, root)
		if strings.TrimSpace(it["head"]) == "" || strings.TrimSpace(it["branch"]) == "" {
			t.Errorf("仓 %s 那一行没带 head/branch：%v", root, it)
		}
	}
	// 负控的那一半：**脏的那仓数 ≠ 0**，干净的那仓仍是 `clean`（证明数是从盘上现算的）。
	if got := itemByPath(t, items, dirty)["status"]; got != "dirty 2" {
		t.Errorf("脏仓的 status 应为 `dirty 2`（1 改 + 1 未跟踪），得到 %q", got)
	}
	if got := itemByPath(t, items, clean)["status"]; got != "clean" {
		t.Errorf("干净仓的 status 应为 `clean`，得到 %q", got)
	}
}

// TestRepoStatusMulti_BothCleanZero —— ③ 正控：两仓全干净 ⇒ rc=0（不是恒 1）。
func TestRepoStatusMulti_BothCleanZero(t *testing.T) {
	a := gitRepo(t)
	b := gitRepo(t)
	rc, items, errb := repoStatusJSON(t, "repo", "status",
		"--root", a, "--root", b, "--json", "head,branch,path,status,untracked")
	if rc != 0 {
		t.Fatalf("两仓全干净 ⇒ rc=0，得到 %d · stderr=%s", rc, errb)
	}
	if len(items) != 2 {
		t.Fatalf("两仓面应 2 条，得到 %d 条：%v", len(items), items)
	}
	for _, root := range []string{a, b} {
		if got := itemByPath(t, items, root)["status"]; got != "clean" {
			t.Errorf("干净仓 %s 的 status 应为 `clean`，得到 %q", root, got)
		}
	}
}

// TestRepoStatusMulti_WriteFlagsRefused —— ④ 负控：写旗标一律拒 rc=2，**且仓真没动**。
//
// ★ **两层牙**（照实记 · 不是一道）：
//
//	第一道（解析器）：**未知旗标 / 多余位置参数** ⟹ 命令根本没跑就能退 2
//	  （`--push` · `--add` · `-a` · `--push=origin` · 裸词 `commit` / `add` 都落这一道）；
//	第二道（本命令自己的写词闭集 `repoWriteWords`）：**写词里有些是真旗标名**
//	  （`--commit` 就是 `repo commit` 的旗标）—— 解析器认得它、于是**静默收下不报错**，
//	  只有这一道抓得住。两道都在 ⇒ 「带写旗标 ⇒ 拒 2」这句话在**任何形态**下都成立。
func TestRepoStatusMulti_WriteFlagsRefused(t *testing.T) {
	root := gitRepo(t)
	headBefore := strings.TrimSpace(mustGit(t, root, "rev-parse", "HEAD"))
	porBefore := mustGit(t, root, "status", "--porcelain=v1")

	refuse := func(bad string) string {
		t.Helper()
		var out, errb strings.Builder
		rc := zerg.RunForTest([]string{"repo", "status", "--root", root, bad}, &out, &errb)
		if rc != 2 {
			t.Errorf("写词 %q ⇒ 应拒 rc=2（本命令只读 · 长在仓外仓上），得到 %d · stderr=%s",
				bad, rc, errb.String())
		}
		return errb.String()
	}
	// 第一道（解析器）：退 2 即可（判词是「未知旗标 / 多余位置参数」—— 照实，不硬要它说「拒」）。
	for _, bad := range []string{"--push", "--add", "-a", "--push=origin", "commit", "add"} {
		refuse(bad)
	}
	// 第二道（本命令的写词闭集）：**真旗标名的写词** ⇒ 退 2 **且**判词必须给「拒」字。
	for _, bad := range []string{"--commit", "--commit=x", "--reset"} {
		if msg := refuse(bad); !strings.Contains(msg, "拒") {
			t.Errorf("写词 %q 走到本命令的写词闭集 ⇒ 判词必须给「拒」字，得到 %q", bad, msg)
		}
	}
	// 真拒（不是拒了又干）：HEAD 与 porcelain 前后同值。
	if got := strings.TrimSpace(mustGit(t, root, "rev-parse", "HEAD")); got != headBefore {
		t.Errorf("拒写后 HEAD 变了：%s ⇒ %s", headBefore, got)
	}
	if got := mustGit(t, root, "status", "--porcelain=v1"); got != porBefore {
		t.Errorf("拒写后工作树变了：%q ⇒ %q", porBefore, got)
	}
}

// TestRepoStatusMulti_NotAGitRepo —— ⑤ 负控：任一 `--root` 不是 git 工作树 ⇒ rc=8（不给结论）。
func TestRepoStatusMulti_NotAGitRepo(t *testing.T) {
	root := gitRepo(t)
	plain := t.TempDir()
	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"repo", "status", "--root", root, "--root", plain}, &out, &errb)
	if rc != 8 {
		t.Errorf("两仓里有一个不是 git 树 ⇒ rc=8（读不到不许当绿），得到 %d · stderr=%s", rc, errb.String())
	}
}

// TestRepoStatusMulti_SingleRootFaceUnchanged —— 对界：`--root` 给 **1 次** ⇒ 仍是**单仓面**（`path` 是仓内相对路径）。
func TestRepoStatusMulti_SingleRootFaceUnchanged(t *testing.T) {
	root := gitRepo(t)
	mustWrite(t, filepath.Join(root, "README.md"), "changed\n")
	rc, items, errb := repoStatusJSON(t, "repo", "status",
		"--root", root, "--json", "head,branch,path,status,untracked")
	if rc != 1 {
		t.Fatalf("单仓（有脏件）⇒ rc=1，得到 %d · stderr=%s", rc, errb)
	}
	if len(items) != 1 {
		t.Fatalf("单仓面应有 1 条（一件一行），得到 %d 条：%v", len(items), items)
	}
	if got := items[0]["path"]; got != "README.md" {
		t.Errorf("单仓面的 path 应是**仓内相对路径**（面一个字未改），得到 %q", got)
	}
}
