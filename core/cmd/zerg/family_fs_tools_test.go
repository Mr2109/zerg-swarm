// family_fs_tools_test.go —— `GAP-20261018-07`：`fs read` / `fs write` 的**兄弟仓正门**
// `--allow-sibling-repo <仓名>`（默认仍 fail-closed）。
//
// 落点：`package main_test` ＋ `zerg.RunForTest`（跑**当前源码**的 `run` —— 连旗标解析面一起过，
// 所以「旗标真被命令面收下」这件事也在判据里，不是只测一个内部函数）。夹具全在 `t.TempDir()` 里：
//
//	<tmp>/Zerg/          —— 合成主仓根（`ZERG_REPO`）
//	<tmp>/Zerg-内部文档/     —— 合成兄弟仓根（= 仓根的兄弟目录 · 闭集里唯一的那个名字）
//
// 成对负控（✗ 禁恒绿 —— 每一格都有相反的那一格）：
//
//	① **默认态逐字不变**：不给旗标读 `../Zerg-内部文档/…` ⇒ rc=2 + 旧文案「件出仓了 … ✗ 只碰本仓工作树」；
//	② 给旗标读真件（三种写法：兄弟仓相对 / `../Zerg-内部文档/…` / 绝对路径）⇒ rc=0 且**内容真读到**；
//	③ 越界：旗标 + `../../etc/passwd` / 仓外绝对路径 ⇒ rc=2（✗ 从兄弟仓再绕出去）；
//	④ 非白名单：旗标值给路径形态 / 别家名 / 空名 ⇒ rc=2 且具名「非白名单兄弟仓」；
//	⑤ 兄弟仓根不在盘 ⇒ rc=8（不给结论）；
//	⑥ 写面：给旗标写临时件 ⇒ rc=0 · **读回字节一致** · 审计行带 `sibling_repo`/`sibling_root` ·
//	   stderr 明写「写了兄弟仓 X 的件 Y」；不给旗标写同一落点 ⇒ rc=2 且那一件**一个字节不动**；
//	⑦ 兄弟仓写面仍受原约束：父目录不存在 ⇒ rc=2（✗ 不自建）、目标件是符号链接 ⇒ rc=2。
package main_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// fsRunTest —— 在合成仓根/状态目录上跑一条 `zerg fs …`（与 `runDevEdit` 同形）。
func fsRunTest(t *testing.T, repo, state string, argv ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", repo)
	t.Setenv("ZERG_STATE_DIR", state)
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(append([]string{"fs"}, argv...), &out, &errb)
	return rc, out.String(), errb.String()
}

// fsuoSynthRepoWithSibling —— 合成「主仓 + 兄弟仓 Zerg-内部文档」两棵树；返回 (仓根, 兄弟仓根, 兄弟仓内相对路径)。
func fsuoSynthRepoWithSibling(t *testing.T) (string, string, string) {
	t.Helper()
	base := t.TempDir()
	repo := filepath.Join(base, "Zerg")
	sib := filepath.Join(base, "Zerg-内部文档")
	mustWrite(t, filepath.Join(repo, "core", "internal", "version", "version.go"), "package version\n")
	rel := filepath.Join("项目文档", "v2.5.15", "变更-v2.5.15.md")
	mustWrite(t, filepath.Join(sib, rel), "第一行：文档面\n第二行：原文\n")
	return repo, sib, rel
}

// TestFSRead_SiblingRepoGate —— 判据 ①–⑤（读面）。
func TestFSRead_SiblingRepoGate(t *testing.T) {
	repo, sib, rel := fsuoSynthRepoWithSibling(t)
	state := t.TempDir()

	// ① 默认态（不给旗标）⇒ rc=2 · **旧文案逐字**（fail-closed 一格）
	rc, out, errb := fsRunTest(t, repo, state, "read", filepath.Join("..", "Zerg-内部文档", rel))
	if rc != 2 {
		t.Fatalf("默认态读兄弟仓件 ⇒ rc=2（fail-closed）：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}
	if !strings.Contains(errb, "件出仓了") || !strings.Contains(errb, "✗ 只碰本仓工作树") {
		t.Errorf("默认态文案必须与改前逐字同（`件出仓了 … ✗ 只碰本仓工作树`）：%q", errb)
	}

	// ② 给旗标读真件 —— 三种写法都要能读（内容真读到，不是「rc=0 但空」）
	for _, p := range []string{
		rel,                                   // 相对**兄弟仓根**（主用法 · 与 dev edit --file 同口径）
		filepath.Join("..", "Zerg-内部文档", rel), // 相对**主仓根**的旧写法
		filepath.Join(sib, rel),               // 绝对路径（含空格也能用 —— argv 是一枚实参）
	} {
		rc, out, errb = fsRunTest(t, repo, state, "read", "--allow-sibling-repo", "Zerg-内部文档", p)
		if rc != 0 {
			t.Fatalf("给旗标读 %q ⇒ rc=0：rc=%d stdout=%q stderr=%q", p, rc, out, errb)
		}
		if !strings.Contains(out, "第一行：文档面") || !strings.Contains(out, "第二行：原文") {
			t.Errorf("给旗标读 %q ⇒ 内容要真读到：stdout=%q", p, out)
		}
	}

	// ②b 机器面 `path` = **兄弟仓内相对**（与 `dev edit` 的 `File` 同口径 ⇒ 回读时原样喂回）
	rc, out, errb = fsRunTest(t, repo, state, "read", rel, "--allow-sibling-repo", "Zerg-内部文档", "--json", "path")
	if rc != 0 || !strings.Contains(out, "项目文档/v2.5.15/变更-v2.5.15.md") {
		t.Errorf("机器面 path 要出兄弟仓内相对路径：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}

	// ③ 越界防绕：给了旗标也不许从兄弟仓再 `..` 出去
	for _, p := range []string{
		filepath.Join("..", "..", "etc", "passwd"),                                     // 相对兄弟仓根 ⇒ 两层 `..`
		filepath.Join("..", "..", "Zerg", "core", "internal", "version", "version.go"), // 绕回主仓也不行（只开兄弟树）
		"/etc/passwd", // 绝对路径出界
	} {
		rc, _, errb = fsRunTest(t, repo, state, "read", "--allow-sibling-repo", "Zerg-内部文档", p)
		if rc != 2 || !strings.Contains(errb, "越出") {
			t.Errorf("越界路径 %q ⇒ rc=2 · 逐字「越出」：rc=%d stderr=%q", p, rc, errb)
		}
	}

	// ④ 非白名单兄弟仓 ⇒ rc=2 且**具名**拒（路径形态 / 大小写别名 / 陌生名 / 空名）
	for _, bad := range []string{filepath.Join("..", "Zerg-内部文档"), "../whatever", "ZergDocs", "foo", ""} {
		rc, _, errb = fsRunTest(t, repo, state, "read", "--allow-sibling-repo", bad, rel)
		if rc != 2 || !strings.Contains(errb, "非白名单兄弟仓") {
			t.Errorf("旗标值 %q ⇒ rc=2 · 具名「非白名单兄弟仓」：rc=%d stderr=%q", bad, rc, errb)
		}
	}

	// ⑤ 闭集名但兄弟仓根不在盘 ⇒ rc=8（不给结论 · 不猜）
	repo2 := t.TempDir()
	mustWrite(t, filepath.Join(repo2, "core", "internal", "version", "version.go"), "package version\n")
	rc, _, errb = fsRunTest(t, repo2, t.TempDir(), "read", "--allow-sibling-repo", "Zerg-内部文档", rel)
	if rc != 8 || !strings.Contains(errb, "不在盘上") {
		t.Errorf("兄弟仓根不在盘 ⇒ rc=8 · 逐字「不在盘上」：rc=%d stderr=%q", rc, errb)
	}

	// ⑥ 件不存在 ⇒ rc=2（给旗标也一样 · ✗ 不冒充空件）
	rc, _, errb = fsRunTest(t, repo, state, "read", "--allow-sibling-repo", "Zerg-内部文档", "没有这件.md")
	if rc != 2 || !strings.Contains(errb, "件不存在") {
		t.Errorf("件不存在 ⇒ rc=2 · 逐字「件不存在」：rc=%d stderr=%q", rc, errb)
	}
}

// TestFSWrite_SiblingRepoGate —— 判据 ⑥–⑦（写面）。
func TestFSWrite_SiblingRepoGate(t *testing.T) {
	repo, sib, rel := fsuoSynthRepoWithSibling(t)
	state := t.TempDir()
	tmpDir := filepath.Join("临时-兄弟仓写面", "20261018")
	if err := os.MkdirAll(filepath.Join(sib, tmpDir), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmpDir, "探针件.md")
	body := "兄弟仓写面探针\n第二行\n"

	// ① 不给旗标 ⇒ rc=2（**默认仍 fail-closed**）· 件一个字节都不许出现
	rc, out, errb := fsRunTest(t, repo, state, "write", filepath.Join("..", "Zerg-内部文档", target),
		"--content", body, "--yes")
	if rc != 2 || !strings.Contains(errb, "件出仓了") {
		t.Fatalf("不给旗标写兄弟仓 ⇒ rc=2 · 旧文案：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}
	if _, err := os.Stat(filepath.Join(sib, target)); err == nil {
		t.Fatalf("被拒的那一格：兄弟仓件**不许**被建出来")
	}

	// ② 给旗标 + 兄弟仓相对路径 ⇒ rc=0 · 读回字节一致 · stderr 留痕 · 审计两格
	rc, out, errb = fsRunTest(t, repo, state, "write", target, "--content", body, "--yes",
		"--allow-sibling-repo", "Zerg-内部文档")
	if rc != 0 {
		t.Fatalf("给旗标写兄弟仓 ⇒ rc=0：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}
	got, rerr := os.ReadFile(filepath.Join(sib, target))
	if rerr != nil || string(got) != body {
		t.Fatalf("写后读回要逐字节一致：err=%v got=%q want=%q", rerr, got, body)
	}
	if _, err := os.Stat(filepath.Join(repo, target)); err == nil {
		t.Errorf("写了兄弟仓 ⇒ ✗ 不许在主仓同名路径留下副本")
	}
	if !strings.Contains(errb, "写了兄弟仓 Zerg-内部文档 的件") {
		t.Errorf("写兄弟仓必须留痕（stderr 一行）：%q", errb)
	}
	ab, aerr := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if aerr != nil {
		t.Fatalf("真写要落审计：%v", aerr)
	}
	lines := strings.Split(strings.TrimSpace(string(ab)), "\n")
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		t.Fatalf("审计不是一行 JSON：%v", err)
	}
	if row["sibling_repo"] != "Zerg-内部文档" || row["sibling_root"] != sib {
		t.Errorf("审计行要带「兄弟仓名 + 它的根」：%+v（want repo=Zerg-内部文档 root=%s）", row, sib)
	}
	if row["fs_path"] != filepath.ToSlash(target) {
		t.Errorf("审计 fs_path 要出兄弟仓内相对路径：%v", row["fs_path"])
	}

	// ③ 兄弟仓写面仍受原约束：父目录不存在 ⇒ rc=2（✗ 不自建）
	rc, _, errb = fsRunTest(t, repo, state, "write", filepath.Join("没有这层", "件.md"),
		"--content", body, "--yes", "--allow-sibling-repo", "Zerg-内部文档")
	if rc != 2 || !strings.Contains(errb, "父目录不存在") {
		t.Errorf("兄弟仓父目录不存在 ⇒ rc=2（✗ 不自建）：rc=%d stderr=%q", rc, errb)
	}
	if _, err := os.Stat(filepath.Join(sib, "没有这层")); err == nil {
		t.Errorf("✗ 不许自建目录")
	}

	// ④ 兄弟仓写面仍受原约束：目标件是符号链接 ⇒ rc=2
	link := filepath.Join(tmpDir, "链接件.md")
	if err := os.Symlink(filepath.Join(sib, rel), filepath.Join(sib, link)); err != nil {
		t.Fatalf("夹具建符号链接失败：%v", err)
	}
	rc, _, errb = fsRunTest(t, repo, state, "write", link, "--content", body, "--yes",
		"--allow-sibling-repo", "Zerg-内部文档")
	if rc != 2 || !strings.Contains(errb, "符号链接") {
		t.Errorf("兄弟仓目标件是符号链接 ⇒ rc=2：rc=%d stderr=%q", rc, errb)
	}
	if b, _ := os.ReadFile(filepath.Join(sib, rel)); string(b) != "第一行：文档面\n第二行：原文\n" {
		t.Errorf("符号链接那一格：被指件一个字节都不许动：%q", b)
	}

	// ⑤ 写面越界 / 非白名单同样拒（写面不是「读面松、写面跟着松」）
	rc, _, errb = fsRunTest(t, repo, state, "write", filepath.Join("..", "..", "etc", "zerg-probe"),
		"--content", body, "--yes", "--allow-sibling-repo", "Zerg-内部文档")
	if rc != 2 || !strings.Contains(errb, "越出") {
		t.Errorf("写面越界 ⇒ rc=2：rc=%d stderr=%q", rc, errb)
	}
	rc, _, errb = fsRunTest(t, repo, state, "write", target, "--content", body, "--yes",
		"--allow-sibling-repo", "../whatever")
	if rc != 2 || !strings.Contains(errb, "非白名单兄弟仓") {
		t.Errorf("写面非白名单 ⇒ rc=2 · 具名：rc=%d stderr=%q", rc, errb)
	}
}

// ── GAP-20261018-08：`fs read` 缺省窗口收口（2000 行 · 不静默截断 · `--offset` 续读拼回整件）──────────
//
// 病灶（对话里真撞）：缺省「读整件」⇒ 一篇 400+ 行 md 就把模型 ctx 顶爆（`system_warning: Input tokens
// have exceeded the limit`），而模型看不到窗口边界 ⇒ 同一调用重发两次、文档一篇未改。
//
// 两态 ＋ 一条负控（缺一不算完）：
//
//	态① 4200 行件缺省读 ⇒ 行数 == 2000（≤ 上限）· **末行逐字**含「已截断：显示 1–2000 / 共 4200 行，
//	     续读用 --offset 2001 …」；`--limit 3000` 显式给值 ⇒ 按 3000（显式即授权 · 不悄悄再夹）；
//	态② `--offset 2001` / `--offset 4001` 续读到尾 ⇒ 三段 `--json content` 拼起来与整件 **sha256 相等**
//	     （内容连续 · 不重不漏 —— 这是「收口没收坏内容」的唯一硬判据）；
//	负控 420 行件（< 上限）⇒ **不报**截断（✗ 不许把没截的说成截断），且 420 行全出。
//
// 夹具全在 `t.TempDir()` 里（`ZERG_REPO` 指合成仓根 ⇒ 一个字节都不落真仓）。

// fsReadWindowLines —— 人面输出的「数据行」与「截断末行」分开取（首行是表头 `line/text`）。
func fsReadWindowLines(t *testing.T, out string) (rows []string, notice string) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, ""
	}
	body := lines[1:]
	if n := len(body); n > 0 && strings.HasPrefix(body[n-1], "已截断：") {
		notice = body[n-1]
		body = body[:n-1]
	}
	return body, notice
}

// fsReadBigRepo —— 合成仓 ＋ 一件 n 行的文本件；返回 (仓根, 仓内相对路径, 整件字节)。
func fsReadBigRepo(t *testing.T, n int) (string, string, string) {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "Zerg")
	mustWrite(t, filepath.Join(repo, "core", "internal", "version", "version.go"), "package version\n")
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "第%04d行：读量收口夹具 filler-%d\n", i, i)
	}
	rel := filepath.Join("docs", "big.md")
	mustWrite(t, filepath.Join(repo, rel), b.String())
	return repo, rel, b.String()
}

// fsReadSHA —— 整串 sha256（十六进制）。
func fsReadSHA(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestFSRead_DefaultWindowCap — 态①：缺省 2000 行夹口 ＋ 末行逐字提示（含总行数与续读起点）。
func TestFSRead_DefaultWindowCap(t *testing.T) {
	const total = 4200
	repo, rel, _ := fsReadBigRepo(t, total)
	state := t.TempDir()

	rc, out, errb := fsRunTest(t, repo, state, "read", rel)
	if rc != 0 {
		t.Fatalf("缺省读 ⇒ rc=0：rc=%d stderr=%q", rc, errb)
	}
	rows, notice := fsReadWindowLines(t, out)
	if len(rows) != 2000 {
		t.Fatalf("缺省窗口应恰 2000 行（≤ 上限）：得 %d 行", len(rows))
	}
	wantNotice := "已截断：显示 1–2000 / 共 4200 行，续读用 --offset 2001（显式 `--limit N` 可放宽窗口）"
	if notice != wantNotice {
		t.Fatalf("末行提示必须逐字（范围 ＋ 总行数 ＋ 续读命令）：\n得 %q\n要 %q", notice, wantNotice)
	}
	if !strings.HasPrefix(rows[0], "1	") || !strings.HasSuffix(rows[0], "第0001行：读量收口夹具 filler-1") {
		t.Errorf("首行不是第 1 行：%q", rows[0])
	}
	if !strings.HasPrefix(rows[len(rows)-1], "2000	") || !strings.HasSuffix(rows[len(rows)-1], "filler-2000") {
		t.Errorf("末条数据行应是第 2000 行：%q", rows[len(rows)-1])
	}
	if !strings.Contains(errb, "已截断") {
		t.Errorf("stderr 也要点明截断（人机两面同一判据）：%q", errb)
	}

	// 机器面：`limit` 报**实际生效**的窗口（旧口径报 0 = 到件尾 ⇒ 调用方看不出被夹了）
	rc, jout, _ := fsRunTest(t, repo, state, "read", rel, "--json", "path,content,total_lines,offset,limit")
	if rc != 0 {
		t.Fatalf("--json 读 ⇒ rc=0：rc=%d", rc)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(jout)), &env); err != nil || len(env.Items) != 1 {
		t.Fatalf("机器面解析失败：err=%v out=%q", err, jout)
	}
	if env.Items[0]["limit"] != "2000" || env.Items[0]["total_lines"] != "4200" || env.Items[0]["offset"] != "1" {
		t.Errorf("机器面三格应是 limit=2000 / total_lines=4200 / offset=1：%v", env.Items[0])
	}
	if strings.Contains(jout, "已截断") {
		t.Errorf("机器面 stdout 必须是纯 JSON（截断只在 stderr 与三格读数里）：%q", jout)
	}

	// 显式 `--limit` 即授权：按 3000（不封顶、也不拿缺省去二次夹）
	rc, out, _ = fsRunTest(t, repo, state, "read", rel, "--limit", "3000")
	if rc != 0 {
		t.Fatalf("--limit 3000 ⇒ rc=0：rc=%d", rc)
	}
	rows, notice = fsReadWindowLines(t, out)
	if len(rows) != 3000 {
		t.Fatalf("`--limit 3000` 应出 3000 行：得 %d 行", len(rows))
	}
	if want := "已截断：显示 1–3000 / 共 4200 行，续读用 --offset 3001（显式 `--limit N` 可放宽窗口）"; notice != want {
		t.Errorf("--limit 档末行提示不逐字：\n得 %q\n要 %q", notice, want)
	}
}

// TestFSRead_OffsetContinuationSHA — 态②：续读到尾后拼接 == 整件（sha256 相等 ⇒ 不重不漏）。
func TestFSRead_OffsetContinuationSHA(t *testing.T) {
	const total = 4200
	repo, rel, body := fsReadBigRepo(t, total)
	state := t.TempDir()

	type win struct {
		offset string // "" = 不传 `--offset`
		rows   int
	}
	var windows []string
	for _, w := range []win{{"", 2000}, {"2001", 2000}, {"4001", 200}} {
		argv := []string{"read", rel, "--json", "content,total_lines,offset,limit"}
		if w.offset != "" {
			argv = append(argv, "--offset", w.offset)
		}
		rc, jout, errb := fsRunTest(t, repo, state, argv...)
		if rc != 0 {
			t.Fatalf("续读 offset=%q ⇒ rc=0：rc=%d stderr=%q", w.offset, rc, errb)
		}
		var env struct {
			Items []map[string]string `json:"items"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(jout)), &env); err != nil || len(env.Items) != 1 {
			t.Fatalf("offset=%q 机器面解析失败：err=%v out=%q", w.offset, err, jout)
		}
		it := env.Items[0]
		// `limit` 报**窗口请求**（缺省 2000）· 实得行数由 content 自己数 —— 两格各管一件事。
		if it["limit"] != "2000" || it["total_lines"] != strconv.Itoa(total) {
			t.Fatalf("offset=%q 窗口读数错（limit=%q total_lines=%q）：要 limit=2000 / total=%d",
				w.offset, it["limit"], it["total_lines"], total)
		}
		first := 1
		if w.offset != "" {
			first, _ = strconv.Atoi(w.offset)
		}
		got := strings.Split(it["content"], "\n")
		if len(got) != w.rows {
			t.Fatalf("offset=%q 实得 %d 行（要 %d 行）", w.offset, len(got), w.rows)
		}
		wantHead := fmt.Sprintf("第%04d行：读量收口夹具 filler-%d", first, first)
		wantTail := fmt.Sprintf("第%04d行：读量收口夹具 filler-%d", first+w.rows-1, first+w.rows-1)
		if !strings.HasPrefix(got[0], wantHead) || !strings.HasPrefix(got[len(got)-1], wantTail) {
			t.Fatalf("offset=%q 窗口首尾行对不上：首 %q 尾 %q", w.offset, got[0], got[len(got)-1])
		}
		if w.offset == "" {
			windows = windows[:0]
		}
		windows = append(windows, it["content"])
	}
	// 三段按顺序拼回整件：窗口之间补回被 `strings.Join` 吃掉的那个换行，末尾补回件尾换行。
	got := strings.Join(windows, "\n") + "\n"
	if got != body {
		t.Fatalf("续读拼接与整件**不是逐字节同**（长度 got=%d want=%d · 首差 %d）",
			len(got), len(body), fsReadFirstDiff(got, body))
	}
	if a, b := fsReadSHA(got), fsReadSHA(body); a != b {
		t.Fatalf("sha256 不等：拼接=%s 整件=%s", a, b)
	} else {
		t.Logf("续读三段拼接 sha256 == 整件 sha256 == %s（共 %d 行 / %d 字节）", a, total, len(body))
	}
}

// fsReadFirstDiff —— 首个不同字节的下标（-1 = 前缀相同）。
func fsReadFirstDiff(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return -1
}

// TestFSRead_SmallFileNoTruncationNotice — 负控：没超窗口 ⇒ ✗ 不许报「已截断」（400+ 行件也要全出）。
func TestFSRead_SmallFileNoTruncationNotice(t *testing.T) {
	const total = 420
	repo, rel, _ := fsReadBigRepo(t, total)
	state := t.TempDir()

	rc, out, errb := fsRunTest(t, repo, state, "read", rel)
	if rc != 0 {
		t.Fatalf("读 420 行件 ⇒ rc=0：rc=%d stderr=%q", rc, errb)
	}
	rows, notice := fsReadWindowLines(t, out)
	if notice != "" {
		t.Fatalf("420 行 < 缺省上限 2000 ⇒ **不许**报截断（假截断会把模型骗去续读）：%q", notice)
	}
	if len(rows) != total {
		t.Fatalf("420 行件应全出：得 %d 行", len(rows))
	}
	if len(rows) > 2000 {
		t.Errorf("行数必须 ≤ 缺省上限 2000：得 %d", len(rows))
	}
	if !strings.HasSuffix(rows[len(rows)-1], "第0420行：读量收口夹具 filler-420") {
		t.Errorf("末行应是第 420 行：%q", rows[len(rows)-1])
	}
	if strings.Contains(errb, "已截断") {
		t.Errorf("stderr 不许报截断：%q", errb)
	}
}

// ── GAP-20261018-09：`fs write --replace <补丁件>`（**定点替换** · 不必先把全文读进上下文）───────
//
// 病灶（对话里真撞 · 已亲核）：`fs write` 是**整件覆盖** ⇒ 改一行必须先拿全文；而读面有两层截断
// （`fs read` 缺省 2000 行 ＋ 对话面回执 8000 字符）⇒ 19000 字节量级的真件只取得到约 8KB，
// 模型判定对、`--dry-run` 也说可写，却**拼不出「完整原文 ＋ 只改一行」** ⇒ 停手不写。
//
// 判据（与 `dev edit --replace` **同一把尺** —— 直接复用 `applyReplacements`，✗ 不另造语法）：
//
//	① 正控：> 8000 字节的件上只改**一行**（补丁件走**仓内相对**路径）⇒ rc=0 ·
//	   读回与「原文只换那一行」**逐字节相同** · 与原文的**差异行数 == 1**（其余字节一个不动）·
//	   审计 `fs_bytes` == 写后大小 · `fs_append` == false；
//	② 零副作用两档：`--dry-run` / 缺 `--yes` ⇒ 件逐字节不变、审计行数不变；
//	③ 负控三态：`old` 命中 **0 次** ⇒ 2 具名 · 命中**多次** ⇒ 2 具名 ·
//	   `--replace` 与整件写（`--content`）/`--append` **同给** ⇒ 2 具名（互斥）；
//	④ 靶件不存在 ⇒ 2（定点替换只在**既有件**上做）；
//	⑤ 兄弟仓：给 `--allow-sibling-repo` 改真件 ⇒ 0；不给旗标 ⇒ 2（默认仍 fail-closed）。
//
// 夹具全在 `t.TempDir()` 里（`ZERG_REPO` 指合成仓根 ⇒ 一个字节都不落真仓）。

// fsuoReplaceFixture —— 造一件 > 8000 字节的文本件：返回 (仓根, 仓内相对路径, 原文)。
// 中间那一行带 `type=record` 且**全件唯一** ⇒ 正好用来验「只改一处」。
func fsuoReplaceFixture(t *testing.T) (string, string, string) {
	t.Helper()
	// ★ 仓根取**解析后**的真实路径：`TMPDIR=/tmp` 时 macOS 的 `/tmp -> /private/tmp`，
	//   而 `fs write` 的「父目录经符号链接出仓」判据在默认态（无兄弟仓）拿未解析的仓根比
	//   已解析的父目录 ⇒ 会误判成「出仓」（那是既有判据，不是本单的面）⇒ 夹具给真路径。
	repo := t.TempDir()
	if r, err := filepath.EvalSymlinks(repo); err == nil {
		repo = r
	}
	rel := filepath.Join("项目文档", "探针-v2.5.15.md")
	lines := make([]string, 0, 900)
	for i := 1; i <= 900; i++ {
		lines = append(lines, fmt.Sprintf("第%04d行：定点替换夹具 filler-%d —— 这一行只为把件撑到 8000 字节以上", i, i))
	}
	lines[449] = "type=record（本行待改：old 必须在件里恰好命中 1 次）"
	orig := strings.Join(lines, "\n") + "\n"
	if len(orig) <= 8000 {
		t.Fatalf("夹具要 > 8000 字节（对话面回执上限）：得 %d 字节", len(orig))
	}
	mustWrite(t, filepath.Join(repo, rel), orig)
	return repo, rel, orig
}

// fsuoWritePatch —— 落一枚补丁件（`[{"old":…,"new":…}]` · 与 `dev edit --replace` 同形）。
func fsuoWritePatch(t *testing.T, path, old, new string) {
	t.Helper()
	body, err := json.Marshal([]map[string]string{{"old": old, "new": new}})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, string(body))
}

// fsuoAuditLines —— 审计件的非空行数（零副作用两档要拿它比「有没有多一行」）。
func fsuoAuditLines(t *testing.T, state string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	return len(fsToolSplitNonEmpty(string(b)))
}

func fsToolSplitNonEmpty(s string) []string {
	out := []string{}
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

// fsuoDiffLines —— 两份文本的**差异行数**（「其余字节一个不动」的硬判据）。
func fsuoDiffLines(a, b string) int {
	al := strings.Split(strings.TrimSuffix(a, "\n"), "\n")
	bl := strings.Split(strings.TrimSuffix(b, "\n"), "\n")
	if len(al) != len(bl) {
		return -1
	}
	n := 0
	for i := range al {
		if al[i] != bl[i] {
			n++
		}
	}
	return n
}

// TestFSWrite_ReplaceInPlace —— 判据 ①–④。
func TestFSWrite_ReplaceInPlace(t *testing.T) {
	repo, rel, orig := fsuoReplaceFixture(t)
	abs := filepath.Join(repo, rel)
	state := t.TempDir()
	oldLine := "type=record（本行待改：old 必须在件里恰好命中 1 次）"
	newLine := "type=record（本行已改：定点替换只动这一行 · 2026-10-18）"

	// 补丁件放**仓内相对**路径（↔ `dev edit --replace` 的「仓内相对按仓根」同一套口径）
	if err := os.MkdirAll(filepath.Join(repo, "补丁"), 0o755); err != nil {
		t.Fatal(err)
	}
	fsuoWritePatch(t, filepath.Join(repo, "补丁", "换一行.json"), oldLine, newLine)

	// ② 干跑：零副作用（件逐字节不变 · 审计零行）
	audit0 := fsuoAuditLines(t, state)
	rc, out, errb := fsRunTest(t, repo, state, "write", rel, "--replace", "补丁/换一行.json", "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 ⇒ rc=0：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}
	if !strings.Contains(out, "定点替换") {
		t.Errorf("计划件要点明模式是定点替换：%q", out)
	}
	if b, _ := os.ReadFile(abs); string(b) != orig {
		t.Fatalf("干跑零副作用：件被改了")
	}
	if n := fsuoAuditLines(t, state); n != audit0 {
		t.Errorf("干跑零副作用：审计多出 %d 行", n-audit0)
	}

	// ② 缺 --yes：rc=2 · 一个字节都不动
	rc, _, errb = fsRunTest(t, repo, state, "write", rel, "--replace", "补丁/换一行.json")
	if rc != 2 || !strings.Contains(errb, "缺 --yes") {
		t.Fatalf("缺 --yes ⇒ rc=2：rc=%d stderr=%q", rc, errb)
	}
	if b, _ := os.ReadFile(abs); string(b) != orig {
		t.Fatalf("缺 --yes 那一档：件被改了")
	}

	// ① 正控：真写（**调用方从未读过全文** —— 这一格只喂了 old/new 两行）
	rc, out, errb = fsRunTest(t, repo, state, "write", rel, "--replace", "补丁/换一行.json", "--yes")
	if rc != 0 {
		t.Fatalf("定点替换真写 ⇒ rc=0：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}
	want := strings.Replace(orig, oldLine, newLine, 1)
	gotB, rerr := os.ReadFile(abs)
	if rerr != nil {
		t.Fatal(rerr)
	}
	got := string(gotB)
	if got != want {
		t.Fatalf("读回必须 == 「原文只换那一行」（逐字节）：got %d 字节 / want %d 字节", len(got), len(want))
	}
	if n := fsuoDiffLines(orig, got); n != 1 {
		t.Fatalf("与原文的差异行数必须 == 1（其余字节一个不动）：得 %d", n)
	}
	if !strings.Contains(got, newLine) || strings.Contains(got, oldLine) {
		t.Errorf("目标行没被换掉（或旧行还在）")
	}
	if !strings.Contains(out, "定点替换") {
		t.Errorf("写成功行要点明模式（定点替换）：%q", out)
	}
	// 审计：大小 == 写后件大小 · append == false
	ab, aerr := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if aerr != nil {
		t.Fatalf("真写要落审计：%v", aerr)
	}
	rows := fsToolSplitNonEmpty(string(ab))
	var row map[string]any
	if err := json.Unmarshal([]byte(rows[len(rows)-1]), &row); err != nil {
		t.Fatalf("审计最后一行不是 JSON：%v", err)
	}
	if int(row["fs_bytes"].(float64)) != len(want) {
		t.Errorf("审计 fs_bytes 要 == 写后大小 %d：得 %v", len(want), row["fs_bytes"])
	}
	if row["fs_append"] != false {
		t.Errorf("定点替换不是追加：fs_append=%v", row["fs_append"])
	}

	// ③ 负控 a：`old` 命中 0 次 ⇒ 2 具名
	fsuoWritePatch(t, filepath.Join(repo, "补丁", "零命中.json"), "这一串在件里根本不存在-ZZ", "ZZZ9-NOT-IN-FILE-QQ")
	before0, _ := os.ReadFile(abs)
	rc, _, errb = fsRunTest(t, repo, state, "write", rel, "--replace", "补丁/零命中.json", "--yes")
	if rc != 2 || !strings.Contains(errb, "命中 0 次") {
		t.Errorf("old 命中 0 次 ⇒ rc=2 · 具名「命中 0 次」：rc=%d stderr=%q", rc, errb)
	}
	if after0, _ := os.ReadFile(abs); string(after0) != string(before0) {
		t.Errorf("被拒的那一格：件一个字节都不许动")
	}

	// ③ 负控 b：`old` 命中多次 ⇒ 2 具名（`第0` 在 900 行里出现多次）
	fsuoWritePatch(t, filepath.Join(repo, "补丁", "多命中.json"), "filler-1", "filler-X")
	rc, _, errb = fsRunTest(t, repo, state, "write", rel, "--replace", "补丁/多命中.json", "--yes")
	if rc != 2 || !strings.Contains(errb, "恰好 1 次") || !strings.Contains(errb, "命中") {
		t.Errorf("old 命中多次 ⇒ rc=2 · 具名「恰好 1 次」：rc=%d stderr=%q", rc, errb)
	}
	if after1, _ := os.ReadFile(abs); string(after1) != string(before0) {
		t.Errorf("被拒的那一格：件一个字节都不许动")
	}

	// ③ 负控 c：与整件写 / `--append` 同给 ⇒ 2（互斥）
	rc, _, errb = fsRunTest(t, repo, state, "write", rel, "--replace", "补丁/换一行.json", "--content", "x", "--yes")
	if rc != 2 || !strings.Contains(errb, "互斥") {
		t.Errorf("`--replace` 与 `--content` 同给 ⇒ rc=2 · 具名「互斥」：rc=%d stderr=%q", rc, errb)
	}
	rc, _, errb = fsRunTest(t, repo, state, "write", rel, "--replace", "补丁/换一行.json", "--append", "--yes")
	if rc != 2 || !strings.Contains(errb, "互斥") {
		t.Errorf("`--replace` 与 `--append` 同给 ⇒ rc=2 · 具名「互斥」：rc=%d stderr=%q", rc, errb)
	}

	// ④ 靶件不存在 ⇒ 2（定点替换只在既有件上做）
	rc, _, errb = fsRunTest(t, repo, state, "write", filepath.Join("项目文档", "没有这件.md"),
		"--replace", "补丁/换一行.json", "--yes")
	if rc != 2 || !strings.Contains(errb, "靶件不存在") {
		t.Errorf("靶件不存在 ⇒ rc=2 · 具名「靶件不存在」：rc=%d stderr=%q", rc, errb)
	}
	if _, err := os.Stat(filepath.Join(repo, "项目文档", "没有这件.md")); err == nil {
		t.Errorf("被拒的那一格：✗ 不许把靶件建出来")
	}
}

// TestFSWrite_ReplaceSiblingRepo —— 判据 ⑤：兄弟仓 ＋ `--replace`（给旗标能改真件；不给仍 2）。
func TestFSWrite_ReplaceSiblingRepo(t *testing.T) {
	repo, sib, rel := fsuoSynthRepoWithSibling(t)
	state := t.TempDir()
	patchDir := t.TempDir()
	orig := "第一行：文档面\n第二行：原文\n"
	fsuoWritePatch(t, filepath.Join(patchDir, "换一行.json"), "第二行：原文", "第二行：已改（定点替换）")

	// 不给旗标 ⇒ rc=2（默认仍 fail-closed）· 件一个字节都不动
	rc, _, errb := fsRunTest(t, repo, state, "write", filepath.Join("..", "Zerg-内部文档", rel),
		"--replace", filepath.Join(patchDir, "换一行.json"), "--yes")
	if rc != 2 || !strings.Contains(errb, "件出仓了") {
		t.Fatalf("不给旗标写兄弟仓 ⇒ rc=2 · 旧文案：rc=%d stderr=%q", rc, errb)
	}
	if b, _ := os.ReadFile(filepath.Join(sib, rel)); string(b) != orig {
		t.Fatalf("被拒的那一格：兄弟仓件不许动")
	}

	// 给旗标 ⇒ rc=0 · 只改那一行
	rc, out, errb := fsRunTest(t, repo, state, "write", rel, "--replace", filepath.Join(patchDir, "换一行.json"),
		"--yes", "--allow-sibling-repo", "Zerg-内部文档")
	if rc != 0 {
		t.Fatalf("给旗标 ＋ --replace 改兄弟仓 ⇒ rc=0：rc=%d stdout=%q stderr=%q", rc, out, errb)
	}
	want := "第一行：文档面\n第二行：已改（定点替换）\n"
	if b, _ := os.ReadFile(filepath.Join(sib, rel)); string(b) != want {
		t.Errorf("兄弟仓读回要 == 期望（只改一行）：got=%q want=%q", b, want)
	}
	if !strings.Contains(errb, "写了兄弟仓 Zerg-内部文档 的件") {
		t.Errorf("写兄弟仓必须留痕（stderr 一行）：%q", errb)
	}
}
