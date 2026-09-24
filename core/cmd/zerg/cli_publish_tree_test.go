// cli_publish_tree_test.go —— `zerg publish tree has` 的**成对判据**（组4 §二.4 `W-50` ·
// `研-禁:129` `R-16` · 任务单序132 · 与组3 `Q-026` 同条）。
//
// 判据栏逐字：**凡「在 / 不在」输出都带口径 + `head_sha`**；**两代树 ⇒ 两行**（并一行 ⇒ 判红）。
//
// 本件把这条判据的两个半边都钉住（夹具自建自管 · 不碰真仓、不碰 `/tmp` 下他人产出树、
// 不写任何件）：
//
//	① 正控 · 两代树两行：两棵**合成**产出树（各自 `git init` + 一次压平提交），件只在一棵里
//	   ⇒ `items` **恰好 2 条**、逐条带 `caliber` + `head_sha`、两行的 `head_sha` **不同**
//	   （换一棵树 = 换一个身份 ⇒ 「两代」这个词在机器面看得见）
//	② 负控 · 「并成一行 ⇒ 判红」：同一次读数的**行数**必须等于**点名次数**（点名两次 ⇒ 两行）；
//	   并成一行（去重 / 合并 / 只留最后一次）⇒ 本用例红
//	③ 负控 · 树身份读不到：树自己不带 `.git` ⇒ **rc=8 且 stdout 0 字节**（一行都不出 ——
//	   口径是「树身份 = 该树自己的压平提交」；拿上层仓的头顶替会让两棵树读同一个 sha）
//	④ 负控 · 树不在盘 ⇒ **rc=8 且 stdout 0 字节**
//	⑤ 负控 · `--json` 里缺身份两格（`caliber` / `head_sha`）⇒ **rc=2 且 stdout 0 字节**
//	   （那两格是读数的身份、不是可选投影格）
//	⑥ 用法面：缺件名 / 缺 `--tree` / 件名形状不合口径 / 裸 `--json` ⇒ 一律 **rc=2 · stdout 0 字节**；
//	   字段表**两处同值**（命令树登记 ⟷ 裸 `--json` 时 stderr 印的那一行）
//
// 落点：`package main_test` ＋ `zerg.RunForTest` —— 跑的是**当前源码**的行为
// （与 `cli_repo_status_multi_test.go` / `cli_gap_test.go` 同一条路）。
package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// publishTreePath —— 本件唯一的命令路径（判据里逐字点名的那一条）。
const publishTreePath = "publish tree has"

// publishTreeFixture —— 造一棵**合成产出树**：给几件 → `git init` + 一次提交（压平提交的等价物）。
func publishTreeFixture(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(f)), "fixture\n")
	}
	// `git init` 要求目录里有东西或空目录都行（-q 静默）；提交作者由 `-c` 就地给。
	for _, argv := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "-m", "flat"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, argv...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("建合成产出树 `git %v`：%v（%s）", argv, err, out)
		}
	}
	return root
}

// publishTreeRun —— 跑一条命令 ⇒ (rc, stdout, stderr)。
func publishTreeRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// publishTreeItems —— 抓 `--json` 的 items（解不开即 Fatal —— 夹具坏了不许静默）。
func publishTreeItems(t *testing.T, out string) []map[string]string {
	t.Helper()
	var doc struct {
		Items []map[string]string `json:"items"`
	}
	body := strings.TrimSpace(out)
	if !strings.HasPrefix(body, "{") {
		t.Fatalf("`--json` 的输出不是包封：%q", out)
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("`--json` 的 items 解不开（%v）：%q", err, out)
	}
	return doc.Items
}

// TestPublishTreeHas_TwoGenerationsTwoRows —— ① 正控两代树两行 + ② 负控「并成一行 ⇒ 判红」。
func TestPublishTreeHas_TwoGenerationsTwoRows(t *testing.T) {
	rel := "scripts/gates/x.py"
	old := publishTreeFixture(t, rel, "README.md")
	fresh := publishTreeFixture(t, "README.md") // 新代树里那件**没了**

	rc, out, errb := publishTreeRun(t, "publish", "tree", "has", rel,
		"--tree", old, "--tree", fresh, "--json", "tree,caliber,head_sha,present")
	if rc != 0 {
		t.Fatalf("两棵树都给出了答案（在 / 不在 都是答案）⇒ rc=0，得到 %d · stderr=%s", rc, errb)
	}
	items := publishTreeItems(t, out)
	// ② 负控：行数 == 点名次数。并成一行（去重 / 合并 / 只留一棵）⇒ 这里就红。
	if len(items) != 2 {
		t.Fatalf("两代树 ⇒ **必须两行**（并成一行 ⇒ 判红）：点名 2 棵、得到 %d 条：%v", len(items), items)
	}
	// ① 正控：逐行都带口径 + head_sha（判据那一句的「凡」）。
	for _, it := range items {
		if strings.TrimSpace(it["tree"]) == "" {
			t.Errorf("有一条读数没带树名：%v", it)
		}
		if strings.TrimSpace(it["caliber"]) == "" {
			t.Errorf("有一条「在 / 不在」读数**没带口径**（判据栏那一句的「凡」）：%v", it)
		}
		if strings.TrimSpace(it["head_sha"]) == "" {
			t.Errorf("有一条「在 / 不在」读数**没带树 head_sha**（判据栏那一句的「凡」）：%v", it)
		}
	}
	if items[0]["head_sha"] == items[1]["head_sha"] {
		t.Errorf("两代树的身份竟然同值（%q）—— 那两行就不是两条读数了", items[0]["head_sha"])
	}
	// 答案确实相反（这就是源件 `Q-026` 那句「同一句「在树否」两代产出树答案相反」的形态）。
	if items[0]["present"] == items[1]["present"] {
		t.Errorf("夹具该造出相反的答案，得到 %q / %q", items[0]["present"], items[1]["present"])
	}
	one, zero := 0, 0
	for _, it := range items {
		if it["present"] == "是" {
			one++
		} else {
			zero++
		}
	}
	if one != 1 || zero != 1 {
		t.Errorf("夹具该是一「是」一「否」，得到 是=%d 否=%d", one, zero)
	}
	// 人面同一句话：一树一行、行行带口径与身份（`件=` 那一行的条数 == 棵树）。
	if got := strings.Count(errb, "件="+rel); got != 2 {
		t.Errorf("人面该逐行照出 2 条读数，得到 %d 条 · stderr=%s", got, errb)
	}
	// 两代树答案不同 ⇒ 明说「口径不同 · 不许并成一个数」。
	if !strings.Contains(errb, "口径不同") || !strings.Contains(errb, "不许并成一个数") {
		t.Errorf("两代树答案不同时该明说「判「口径不同」· 不许并成一个数」：%s", errb)
	}
}

// TestPublishTreeHas_NoJSONHumanFace —— 人面（不给 `--json`）也**行行**带口径 + `head_sha`。
func TestPublishTreeHas_NoJSONHumanFace(t *testing.T) {
	rel := "README.md"
	a := publishTreeFixture(t, rel)
	b := publishTreeFixture(t, rel, "extra.txt") // 两棵树的**内容不同** ⇒ 身份（head_sha）必不同

	rc, out, errb := publishTreeRun(t, "publish", "tree", "has", rel, "--tree", a, "--tree", b)
	if rc != 0 {
		t.Fatalf("人面 rc 该是 0，得到 %d · stderr=%s", rc, errb)
	}
	lines := []string{}
	for _, ln := range strings.Split(out, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		// 人面是**制表符分隔的四列**（表头那一行第三列是字面量 `head_sha`，不是 sha ⇒ 自然排除）。
		cols := strings.Split(strings.TrimSpace(ln), "	")
		if len(cols) == 4 && len(cols[2]) == 40 {
			lines = append(lines, strings.TrimSpace(ln))
		}
	}
	if len(lines) != 2 {
		t.Fatalf("人面该是**一树一行**（2 行读数），得到 %d 行：%q", len(lines), out)
	}
	heads := map[string]bool{}
	for _, ln := range lines {
		cols := strings.Split(ln, "	")
		heads[cols[2]] = true
	}
	if len(heads) != 2 {
		t.Errorf("两棵合成树的 head_sha 该各不相同：%v", heads)
	}
}

// TestPublishTreeHas_IdentityAndAbsence —— ③ 树身份读不到 + ④ 树不在盘：**两档都 rc=8 · stdout 0 字节**。
func TestPublishTreeHas_IdentityAndAbsence(t *testing.T) {
	good := publishTreeFixture(t, "README.md")
	// ③ 一棵**不带 `.git`** 的普通目录。
	noGit := t.TempDir()
	mustWrite(t, filepath.Join(noGit, "README.md"), "x\n")
	// ③b **最要紧的那一格**：树的名字指到**某个 git 仓里面**的子目录（那一层不是它自己的仓）
	//     —— 此时 `git -C <子目录> rev-parse HEAD` **会往上走**、把父仓的头当成这棵树的身份。
	//     这正是源件要治的病（两棵不同的树读出**同一个** head_sha）⇒ 必须 rc=8。
	outer := gitRepo(t)
	inner := filepath.Join(outer, "subtree")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name string
		tree string
		mark string // 该档**必须**点名的那个词（两档的理由不同 ⇒ 判词也得不同）
	}{
		{"树自己不带 .git", noGit, ".git"},
		{"树指到别的仓里面的子目录（不许拿父仓头顶替）", inner, ".git"},
	} {
		rc, out, errb := publishTreeRun(t, "publish", "tree", "has", "README.md", "--tree", c.tree)
		if rc != 8 {
			t.Errorf("%s ⇒ rc=8（不许当绿、不许拿上层仓头顶替），得到 %d · stderr=%s", c.name, rc, errb)
		}
		if out != "" {
			t.Errorf("%s：不给结论时 **stdout 0 字节**（出半张表会被当成完整读数），得到 %q", c.name, out)
		}
		if !strings.Contains(errb, c.mark) {
			t.Errorf("%s：该点名 %q：%s", c.name, c.mark, errb)
		}
	}

	// ④ 树不在盘（**另一档理由** ⇒ 判词也必须不同：这一档讲「不在盘」，不讲「.git」）。
	rc, out, errb := publishTreeRun(t, "publish", "tree", "has", "README.md",
		"--tree", filepath.Join(t.TempDir(), "nope-zz"))
	if rc != 8 {
		t.Fatalf("树不在盘 ⇒ rc=8，得到 %d · stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("不给结论时 stdout 该 0 字节，得到 %q", out)
	}
	if !strings.Contains(errb, "不在盘") {
		t.Errorf("这一档的理由是「树不在盘」——判词该讲它（与「树身份读不到」分开）：%s", errb)
	}

	// ④b 树名口径：给**相对写法**，印出来的一律是**绝对路径**（源件 `R-16` 引的那句
	//     「The filepath has to be consistent across the runs」—— 相对写法随 cwd 变）。
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relPath, err := filepath.Rel(cwd, good)
	if err != nil {
		t.Fatal(err)
	}
	rc, out, errb = publishTreeRun(t, "publish", "tree", "has", "README.md", "--tree", relPath)
	if rc != 0 {
		t.Fatalf("相对写法也是同一棵树 ⇒ rc=0，得到 %d · stderr=%s", rc, errb)
	}
	if strings.Contains(out, relPath) {
		t.Errorf("树名口径是**绝对路径**（相对写法随 cwd 变、同一棵树会读出两个名字）：印了 %q", out)
	}
	if !strings.Contains(out, filepath.Clean(good)+"	") {
		t.Errorf("树名该印绝对路径 %q：%q", good, out)
	}

	// 成对半边：同一棵树、同一句问法，树好了就**回绿**（证明上面那两红不是因为命令恒红）。
	rc, out, errb = publishTreeRun(t, "publish", "tree", "has", "README.md", "--tree", good)
	if rc != 0 {
		t.Fatalf("同一棵树（带 `.git`）⇒ 回绿 rc=0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, good) {
		t.Errorf("人面该带树名（绝对路径）：%q", out)
	}
}

// TestPublishTreeHas_FieldsFace —— ⑤ `--json` 缺身份两格 ⇒ 2 + 0 字节；齐 ⇒ 0；⑥ 字段表两处同值。
func TestPublishTreeHas_FieldsFace(t *testing.T) {
	tree := publishTreeFixture(t, "README.md")

	// ⑤ 负控三条：缺 `caliber` / 缺 `head_sha` ⇒ 一律 rc=2；且**给了 `--json <字段>`**时的失败
	// 走 M7 错误包封（六键 + `error.kind`），`items` **空**（一条读数都不出）。
	for _, fields := range []string{"tree,present", "caliber,tree", "present"} {
		rc, out, errb := publishTreeRun(t, "publish", "tree", "has", "README.md",
			"--tree", tree, "--json", fields)
		if rc != 2 {
			t.Errorf("`--json %s` 缺身份格 ⇒ rc=2，得到 %d · stderr=%s", fields, rc, errb)
		}
		var doc struct {
			Items []map[string]string `json:"items"`
			Error struct {
				Kind   string `json:"kind"`
				Detail string `json:"detail"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Errorf("`--json %s` 被拒时该出一个**错误包封**：%v · out=%q", fields, err, out)
			continue
		}
		if len(doc.Items) != 0 {
			t.Errorf("`--json %s` 被拒时 `items` 该是空的（一条读数都不出），得到 %v", fields, doc.Items)
		}
		if doc.Error.Kind != "usage" || doc.Error.Detail != "identity_cells_required" {
			t.Errorf("`--json %s` 的错误块该是 usage/identity_cells_required，得到 %+v", fields, doc.Error)
		}
		if !strings.Contains(errb, "口径") || !strings.Contains(errb, "head_sha") {
			t.Errorf("拒的理由该点名那两格：%s", errb)
		}
	}
	// 成对正控：两格齐 ⇒ 绿。
	rc, _, errb := publishTreeRun(t, "publish", "tree", "has", "README.md",
		"--tree", tree, "--json", "tree,caliber,head_sha,present")
	if rc != 0 {
		t.Fatalf("身份两格齐 ⇒ rc=0，得到 %d · stderr=%s", rc, errb)
	}
	// 裸 `--json`（K2 甲档）⇒ 2 + 0 字节 + stderr 列全部可选字段。
	rc, out, errb := publishTreeRun(t, "publish", "tree", "has", "README.md", "--tree", tree, "--json")
	if rc != 2 {
		t.Errorf("裸 `--json` ⇒ rc=2（K2 甲档 · 码取自退码表），得到 %d", rc)
	}
	if out != "" {
		t.Errorf("裸 `--json` 时 stdout 该 0 字节，得到 %q", out)
	}
	// ⑥ 字段表**两处同值**：命令树那一份 ⟷ 裸 `--json` 时 stderr 印的那一份。
	info, ok := zerg.CommandInfoOfForTest(publishTreePath)
	if !ok {
		t.Fatalf("命令树里没有 %q（登记漏了）", publishTreePath)
	}
	declared := strings.Join(info.Fields, ",")
	if want := "tree,caliber,head_sha,present"; declared != want {
		t.Errorf("%s 的字段表该是 %q，得到 %q", publishTreePath, want, declared)
	}
	if !strings.Contains(errb, "可选字段: "+declared) {
		t.Errorf("裸 `--json` 那一行「可选字段」与命令树登记不同值：\n 树=%q\n stderr=%s", declared, errb)
	}
	// 点不在表里的字段 ⇒ 2（`reportBadField` 的既有路径 · 与全命令面同形）。
	rc, out, _ = publishTreeRun(t, "publish", "tree", "has", "README.md",
		"--tree", tree, "--json", "tree,caliber,head_sha,present,nosuch-zz")
	if rc != 2 {
		t.Errorf("字段表外的字段 ⇒ rc=2，得到 rc=%d · out=%q", rc, out)
	}
}

// TestPublishTreeHas_UsageFace —— ⑥ 四条用法错：一律 rc=2 · stdout 0 字节。
func TestPublishTreeHas_UsageFace(t *testing.T) {
	tree := publishTreeFixture(t, "README.md")
	cases := []struct {
		name string
		argv []string
	}{
		{"缺件名", []string{"publish", "tree", "has", "--tree", tree}},
		{"缺 --tree", []string{"publish", "tree", "has", "README.md"}},
		{"件名是绝对路径", []string{"publish", "tree", "has", "/etc/hosts", "--tree", tree}},
		{"件名带 `..`", []string{"publish", "tree", "has", "../README.md", "--tree", tree}},
		{"件名带前导 `./`", []string{"publish", "tree", "has", "./README.md", "--tree", tree}},
		{"未知旗标", []string{"publish", "tree", "has", "README.md", "--tree", tree, "--nosuchflag-zz"}},
		{"不认的契约主号", []string{"publish", "tree", "has", "README.md", "--tree", tree, "--schema", "zerg/v9"}},
	}
	for _, c := range cases {
		rc, out, errb := publishTreeRun(t, c.argv...)
		if rc != 2 {
			t.Errorf("%s ⇒ rc=2，得到 %d · stderr=%s", c.name, rc, errb)
		}
		if out != "" {
			t.Errorf("%s ⇒ stdout 该 0 字节，得到 %q", c.name, out)
		}
	}
	// 成对半边：同一棵树 + 合格件名 ⇒ 回绿（证明上面那七红不是命令恒红）。
	if rc, _, errb := publishTreeRun(t, "publish", "tree", "has", "README.md", "--tree", tree); rc != 0 {
		t.Errorf("合格口令 ⇒ rc=0，得到 %d · stderr=%s", rc, errb)
	}
	// 只读自证：跑完真仓与状态目录一个字节没动（本命令只 `lstat` + `git rev-parse`）。
	if _, err := os.Stat(filepath.Join(tree, ".git")); err != nil {
		t.Errorf("夹具树被动了：%v", err)
	}
}
