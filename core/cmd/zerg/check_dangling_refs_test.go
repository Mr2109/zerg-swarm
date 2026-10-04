// check_dangling_refs_test.go —— `core/cmd/zerg/check_dangling_refs.go` 的**成对负控**测试件
// （M2 真搬验收 · 入口 `checkDanglingRefsGateMain`）。
//
// 口径（照 skill「成对负控 · ✗ 禁恒绿」）：
//
//	① **正控**：`--self-test` 必须绿（十五格成对负控 —— 必红×2 / 豁免×1 / 拼接×1 / 前缀×1 /
//	   无输入×1 / 登记表×1 / 整件豁免×1 / 前缀越界×1 / 缺语料×1 / 跨仓×1 / GAP悬空×1 /
//	   GAP全在账×1 / GAP负控×1 / GAP账缺×1）。
//	② **成对**：每一格判据都配一枚反例（哑化本件任一判据 ⇒ 本测试件必 FAIL）。
//	③ **自回收**：`--self-test` 跑完，`TMPDIR` 里 `cdr-selftest-*` 目录数增量必须 = 0
//	   （py 原件 `mkdtemp` 从不回收 = 真口子；本件自回收，此处把该性质钉成用例）。
//	④ 全程**不碰真目标**（`--self-test` 只造 `TMPDIR` 下的合成夹具；`t.TempDir()` 自动回收）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cdrCountSelfTestDirs —— TMPDIR 里 `cdr-selftest-*` 目录数（自回收的度量）。
func cdrCountSelfTestDirs() int {
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		return -1
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "cdr-selftest-") {
			n++
		}
	}
	return n
}

// cdrQ —— 夹具串拼（✗ 不让被扫形态以**连续原串**落进本件正文 —— 自指形态②）。
func cdrQ(s string) string { return "\"" + s + "\"" }

func TestCheckDanglingRefsGatePort(t *testing.T) {
	t.Run("自检十五格绿", func(t *testing.T) {
		var out, errb bytes.Buffer
		rc := checkDanglingRefsGateMain(&out, &errb, []string{"--self-test"})
		if rc != 0 {
			t.Fatalf("`--self-test` rc=%d（期望 0）\nstdout:\n%s\nstderr:\n%s", rc, out.String(), errb.String())
		}
		if !strings.Contains(out.String(), "自检：通过 ✓") {
			t.Fatalf("`--self-test` 未报「通过 ✓」（十格里有格不过）：\n%s", out.String())
		}
		// 成对负控：十格的名必须都在（少跑即失效 —— py 有 `assert cases == 10`）。
		for _, want := range []string{"[必红夹具]", "[必红夹具·有落点]", "[豁免夹具]", "[拼接夹具]",
			"[前缀夹具]", "[无输入夹具]", "[登记表夹具]", "[整件豁免夹具]", "[前缀登记夹具]", "[缺语料夹具]",
			"[跨仓夹具]", "[GAP悬空夹具]", "[GAP全在账夹具]", "[GAP负控夹具]", "[GAP账缺夹具]"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("`--self-test` 少跑一格：缺 %s\n%s", want, out.String())
			}
		}
	})

	t.Run("自回收_delta0", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
		before := cdrCountSelfTestDirs()
		if before < 0 {
			t.Skipf("TMPDIR 读不了 ⇒ 本格不适用")
		}
		var out, errb bytes.Buffer
		if rc := checkDanglingRefsGateMain(&out, &errb, []string{"--selftest"}); rc != 0 {
			t.Fatalf("`--selftest` rc=%d（期望 0）\n%s", rc, errb.String())
		}
		after := cdrCountSelfTestDirs()
		if after != before {
			t.Fatalf("自检夹具未自回收：TMPDIR 里 `cdr-selftest-*` 由 %d 变 %d（delta 必须 = 0 —— py 原件 mkdtemp 从不回收 = 真口子，本件自回收）",
				before, after)
		}
	})

	t.Run("pyJoin_三语义", func(t *testing.T) {
		// 三类语义差逐条钉：① 空段 ② 后段绝对丢前段 ③ 不吃中间 `..`
		cases := [][3]string{
			{"a/b", "", "a/b/"},         // py: join('a/b','') == 'a/b/'（✗ 不是 filepath.Join 的 'a/b'）
			{"a/b/", "c", "a/b/c"},      // 空段跳过
			{"a/b", "/c", "/c"},         // 后段绝对 ⇒ 丢前段
			{"a/../b", "c", "a/../b/c"}, // ✗ 不归一 `..`（filepath.Join 会得 b/c）
			{"", "a", "a"},              // py: join('','a') == 'a'
		}
		for _, c := range cases {
			if got := cdrPyJoin(c[0], c[1]); got != c[2] {
				t.Fatalf("cdrPyJoin(%q,%q)=%q（期望 %q）—— os.path.join 三语义差", c[0], c[1], got, c[2])
			}
		}
	})

	t.Run("直书_左后顾", func(t *testing.T) {
		st := &cdrStats{}
		got := cdrDirectRefs("# 见 docs"+"/01-设计/x.md 与 notdocs"+"/a", st)
		if len(got) != 1 || got[0].ref != "docs"+"/01-设计/x.md" {
			t.Fatalf("直书抽取错：%+v", got)
		}
		// 成对：前面粘着字母/斜杠的**不算**（那正是「仓内相对」与「URL/跨仓」的分界）
		st2 := &cdrStats{}
		got2 := cdrDirectRefs("https://x/docs"+"/a.md 与 别仓/docs"+"/b.md", st2)
		if len(got2) != 0 {
			t.Fatalf("左后顾没挡住前面的字符：%+v", got2)
		}
		// 散文误吞（`docs` 后接**单段纯中文** —— 不判）记数，不静默（成对：另起一枚统计袋）
		st3 := &cdrStats{}
		if got3 := cdrDirectRefs("**docs"+"/的目录**（散文）", st3); len(got3) != 0 {
			t.Fatalf("单段纯中文散文被当引文：%+v", got3)
		}
		if st3.refsProseSkipped == 0 {
			t.Fatalf("散文误吞计数为 0（应记数不静默）")
		}
	})

	t.Run("Join_拼接", func(t *testing.T) {
		got := cdrJoinRefs("x := filepath." + "Join(workDir, " + cdrQ("docs") + ", " + cdrQ("issues") + ")")
		if len(got) != 1 || got[0].ref != "docs"+"/issues" || !strings.Contains(got[0].note, "变量") {
			t.Fatalf("filepath.Join 首段变量档错：%+v", got)
		}
		got2 := cdrJoinRefs("y := path." + "Join(" + cdrQ("docs") + ", " + cdrQ("01-设计") + ", " + cdrQ("x.md") + ")")
		if len(got2) != 1 || got2[0].ref != "docs"+"/01-设计/x.md" || !strings.Contains(got2[0].note, "字面量") {
			t.Fatalf("path.Join 字面量档错：%+v", got2)
		}
		// 成对负控：遇变量即止（✗ 不许把 `docs`+`/issues/.md` 拼成假的）
		got3 := cdrJoinRefs("z := filepath." + "Join(" + cdrQ("docs") + ", " + cdrQ("issues") + ", id+" + cdrQ(".md") + ")")
		if len(got3) != 1 || got3[0].ref != "docs"+"/issues" {
			t.Fatalf("遇变量未即止：%+v", got3)
		}
		// 成对：前面粘着标识符字符的 `.Join` 不认
		if got4 := cdrJoinRefs(`myfilepath.Join("docs","a")`); len(got4) != 0 {
			t.Fatalf("左后顾没挡住 `.Join` 前缀：%+v", got4)
		}
	})

	t.Run("concat_前缀", func(t *testing.T) {
		g1 := cdrExtract(`name = "docs`+`/" + base`, &cdrStats{})
		g2 := cdrExtract(`p = "docs" + "/x"`, &cdrStats{})
		n1, n2 := 0, 0
		for _, r := range g1 {
			if r.form == "字符串拼接" {
				n1++
			}
		}
		for _, r := range g2 {
			if r.form == "字符串拼接" {
				n2++
			}
		}
		if n1 != 1 || n2 != 1 {
			t.Fatalf("字符串拼接形态漏认：%d / %d（期望各 1）", n1, n2)
		}
		// 成对负控：普通字符串不误认
		g3 := cdrExtract(`p = "docsx" + base`, &cdrStats{})
		for _, r := range g3 {
			if r.form == "字符串拼接" {
				t.Fatalf("误认非 `docs"+"/` 前缀形态：%+v", r)
			}
		}
	})

	t.Run("分类_优先级", func(t *testing.T) {
		ctx := &cdrCtx{root: "/nonexistent-root", registry: &cdrOM{}}
		// 机读件 ⇒ ledger（后缀容器判据）
		cls, why := cdrClassify(ctx, "scripts/x.json", "docs"+"/gone.md", `{"p": "docs`+`/gone.md"}`)
		if cls != "ledger" {
			t.Fatalf("机读件未判 ledger：%s / %s", cls, why)
		}
		// 记录措辞 ⇒ ledger
		if cls, _ := cdrClassify(ctx, "scripts/x.py", "docs"+"/gone.md", "# 台账：docs"+"/gone.md"); cls != "ledger" {
			t.Fatalf("记录措辞未判 ledger：%s", cls)
		}
		// 指路措辞 ⇒ live
		if cls, _ := cdrClassify(ctx, "scripts/x.py", "docs"+"/gone.md", "# 真源：docs"+"/gone.md"); cls != "live" {
			t.Fatalf("指路措辞未判 live：%s", cls)
		}
		// 默认 ⇒ live
		if cls, _ := cdrClassify(ctx, "scripts/x.py", "docs"+"/gone.md", "# 见 docs"+"/gone.md 吧"); cls != "live" {
			t.Fatalf("默认档未判 live：%s", cls)
		}
		// 成对负控：夹具词 ⇒ ledger（默认档的反例）
		if cls, _ := cdrClassify(ctx, "scripts/x.py", "docs"+"/gone.md", "# 负控夹具 docs"+"/gone.md"); cls != "ledger" {
			t.Fatalf("夹具词未判 ledger：%s", cls)
		}
	})

	t.Run("登记表_整件豁免拒", func(t *testing.T) {
		d := t.TempDir()
		p := filepath.Join(d, "blanket.json")
		if err := os.WriteFile(p, []byte(`{"entries":[{"file":"a.py","class":"ledger","reason":"整件免了","date":"2026-09-22"}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		doc, bad := cdrLoadRegistry(p)
		// py 语义：bad 非空 ⇒ 调用方 rc=2（**doc 本身非 nil** —— 只有 JSON 坏/缺 entries 才 nil）
		if doc == nil || len(bad) != 1 || !strings.Contains(bad[0], "洗白") {
			t.Fatalf("整件豁免未被拒：doc=%v bad=%v", doc, bad)
		}
		// 成对：缺 reason + date ⇒ ≥2 条
		p2 := filepath.Join(d, "bad.json")
		if err := os.WriteFile(p2, []byte(`{"entries":[{"file":"a","ref":"docs`+`/x"}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		doc2, bad2 := cdrLoadRegistry(p2)
		if doc2 == nil || len(bad2) < 2 {
			t.Fatalf("缺 reason+date 未报 ≥2 条：doc=%v bad=%v", doc2, bad2)
		}
		// 成对：合格表 ⇒ 0 条
		p3 := filepath.Join(d, "ok.json")
		if err := os.WriteFile(p3, []byte(`{"entries":[{"file":"a.py","ref_prefix":"docs`+`/","class":"ledger","reason":"有意保留","date":"2026-09-22"}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, bad3 := cdrLoadRegistry(p3); len(bad3) != 0 {
			t.Fatalf("合格表被误判：%v", bad3)
		}
	})

	t.Run("缺语料退2", func(t *testing.T) {
		// 成对：语料树不在盘 ⇒ 报 1 条（翻 rc=2 的档）；在盘 ⇒ 报 0 条；归档树缺席**不翻退码**
		d := t.TempDir()
		miss := cdrAbsentCorpusTrees(filepath.Join(d, "no-such"), "")
		if len(miss) != 1 {
			t.Fatalf("语料树缺席未报 1 条：%v", miss)
		}
		have := cdrAbsentCorpusTrees(d, filepath.Join(d, "no-such-arch"))
		if len(have) != 0 {
			t.Fatalf("语料树在盘（归档缺席）应报 0 条：%v", have)
		}
		// 退码边界：scanned==0 ⇒ rc=2（拒绝静默空转）
		var out, errb bytes.Buffer
		empty := filepath.Join(d, "empty")
		if err := os.MkdirAll(empty, 0o755); err != nil {
			t.Fatal(err)
		}
		if rc := cdrRealExitCode(empty, "", []string{}, true, nil, &errb); rc != 2 {
			t.Fatalf("空树 rc=%d（期望 2 —— 拒绝静默空转）", rc)
		}
		_ = out
	})

	t.Run("未知旗标退2", func(t *testing.T) {
		var out, errb bytes.Buffer
		if rc := checkDanglingRefsGateMain(&out, &errb, []string{"--zzz"}); rc != 2 {
			t.Fatalf("未知旗标 rc=%d（期望 2）", rc)
		}
		if !strings.Contains(errb.String(), "'--zzz'") {
			t.Fatalf("未知旗标未按 py repr 打引号：%q", errb.String())
		}
	})
	// 牙 读失败不再静默（仓外牙 · 回归）：待扫件不可读 ⇒ 登记 `cdrStats.readFails` ⇒ rc=2，
	//    BLOCKED 头落**本件实际通道 stderr**（`checkDanglingRefsGateMain` 处 `fmt.Fprint(stderr, …)`）
	//    + 逐件点名那一件。
	t.Run("读失败-BLOCKED落stderr", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 下 chmod 000 仍可读 ⇒ 该格无鉴别力")
		}
		root := t.TempDir()
		// 入口守卫要 `docs/` 或 `core/` 在盘（空目录即够）。
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		notes := filepath.Join(root, "notes")
		if err := os.MkdirAll(notes, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(notes, "ok.md"), []byte("干净件\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		blockedRel := "notes" + "/blocked.md"
		blocked := filepath.Join(root, filepath.FromSlash(blockedRel))
		if err := os.WriteFile(blocked, []byte("干净件\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if o, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
			t.Fatalf("git init 夹具失败：%v %s", err, o)
		}
		if o, err := exec.Command("git", "-C", root, "add", "-A").CombinedOutput(); err != nil {
			t.Fatalf("git add 夹具失败：%v %s", err, o)
		}
		if err := os.Chmod(blocked, 0o000); err != nil {
			t.Fatalf("chmod 000 失败：%v", err)
		}
		defer os.Chmod(blocked, 0o644)
		var out, errb bytes.Buffer
		if rc := checkDanglingRefsGateMain(&out, &errb, []string{"--root", root}); rc != 2 {
			t.Fatalf("读失败应 rc=2（不给结论）：得 rc=%d out=%q err=%q", rc, out.String(), errb.String())
		}
		if !strings.Contains(errb.String(), "⛔ BLOCKED：以下件读失败，覆盖面不完整 —— 不给结论（假覆盖）\n") {
			t.Fatalf("BLOCKED 头未落 stderr（本件实际通道）：%q", errb.String())
		}
		if !strings.Contains(errb.String(), "   · "+blockedRel+"（") {
			t.Fatalf("未逐件点名读失败件：%q", errb.String())
		}
		if strings.Contains(out.String(), "BLOCKED") {
			t.Fatalf("BLOCKED 头不该落 stdout（本件通道是 stderr）：%q", out.String())
		}
	})

	// GAP 号引用类：分类复用本门 cdrClassify（成对负控 —— live 命中 / 数据字串例外面 / 夹具件 / 本件自身）。
	t.Run("GAP号_分类", func(t *testing.T) {
		ctx := &cdrCtx{root: "/nonexistent-root", registry: &cdrOM{}}
		g := "GAP-" + "20260928-999"
		if cls, _ := cdrClassify(ctx, "scripts/x.py", g, "# 见缺口 `"+g+"`"); cls != "live" {
			t.Fatalf("GAP 注释引用未判 live：%s", cls)
		}
		if cls, _ := cdrClassify(ctx, "scripts/x.py", g, `fx = {"id": "`+g+`"}`); cls != "ledger" {
			t.Fatalf("GAP 数据字串未判 ledger：%s", cls)
		}
		if !cdrIsGapTestContainer("scripts/x_test.go") || !cdrIsGapTestContainer("a/testdata/b.txt") ||
			!cdrIsGapTestContainer("pkg/tests/c.py") || cdrIsGapTestContainer("scripts/x.go") {
			t.Fatalf("cdrIsGapTestContainer 判定错")
		}
		if !cdrIsGapSelfFile(ctx, "scripts/gates/check-dangling-refs.py") ||
			cdrIsGapSelfFile(ctx, "scripts/gates/g1.py") {
			t.Fatalf("cdrIsGapSelfFile 判定错")
		}
		if !cdrGapRE.MatchString("GAP-" + "20261005-54") {
			t.Fatalf("cdrGapRE 未匹配")
		}
	})
}
