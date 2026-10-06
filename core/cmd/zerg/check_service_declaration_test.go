// check_service_declaration_test.go —— `check_service_declaration.go` 的**成对负控**单测
// （M2 一枚 · 承接 GAP-20261001-13 · 入口 `checkServiceDeclarationGateMain`）。
//
// 口径：✗ 禁恒绿 —— 每格都断言**判据面**（rc ＋ 关键正文行），不是只断言「跑起来了」。
// 成对：正控（声明齐 / 已登记幽灵 / 闭集外标签 ⇒ rc=0）与负控（未登记幽灵 / 悬空声明 / 无处置 /
// 空转 / 缺件 ⇒ rc=1 或 2）成对，**空值三态逐态各一格**（取不到 · 显式空集合 · 缺键各成格，
// 并各自断言「不出现对方文案」）。帮助面把 py 3.9.6 `-h` 的 sha256 钉成 golden 逐字比。
//
// 有牙实证（仓外副本哑化 ⇒ 本件用例 FAIL）见交付回执：① 把主件里「未登记幽灵」那条 problem 的
// 追加动作掐掉 ⇒ 「负控:未登记幽灵」当场 FAIL（带行号）；② 把主件 `csdWordAnchor` 换成子串包含
// （`strings.Contains`）⇒ 「负控:词界锚 兄弟名不假绿」与「负控:R4a 标签↔pid↔程序不一致」两格 FAIL
// （`bin/zerg-core` 是 `bin/zerg-core-test` 的子串 ⇒ 假绿、rc 由 1 变 0）。
// ③ 2026-10-07（A2 收尾）：把 `searx.webapp` 从 `csdProcHints` 去掉 ⇒ 「负控:searxng 进程无落点」一格
//
//	rc 由 1 变 0（假绿）⇒ 该用例 FAIL ⇒ 证 `searx.webapp` 是 R4b 覆盖 searxng 的牙。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// csdTestRoot —— 造一棵含 `core/cmd/zerg` 的**仓外夹具根**（find_root 才认它）。
func csdTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// csdRun —— 进程内取 rc / stdout / stderr 三面。
func csdRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkServiceDeclarationGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// csdSetEnv —— 四旋钮逐字指向仓外夹具。
func csdSetEnv(t *testing.T, dp, pp, lp, home string) {
	t.Helper()
	t.Setenv("ZERG_SVCDECL_DECL", dp)
	t.Setenv("ZERG_SVCDECL_PS", pp)
	t.Setenv("ZERG_SVCDECL_LAUNCHCTL", lp)
	t.Setenv("ZERG_SVCDECL_HOME", home)
}

// csdWrite —— 把一张夹具落盘（子测里改盘用）。
func csdWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckServiceDeclarationGatePort(t *testing.T) {
	goodPlists := []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist"}

	t.Run("正控:好件 rc=0 双面取证在场", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, se := csdRun(t, root)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（stderr=%q）", rc, se)
		}
		if !strings.Contains(so, "⇒ 全绿（rc=0）") {
			t.Fatalf("未走全绿档：%q", so)
		}
		if !strings.Contains(so, "取证:   ps=fixture · launchctl=fixture") {
			t.Fatalf("取证面不对：%q", so)
		}
		if !strings.Contains(so, "声明 name 2 个（行数 相等）") {
			t.Fatalf("集合对拍行不对：%q", so)
		}
	})

	t.Run("负控:未登记幽灵 rc=1 且逐条点名", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		csdWrite(t, lp, csdGoodLC+"55555\t0\tcom.zerg.ghost-unregistered\n")
		rc, so, _ := csdRun(t, root)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "**未登记幽灵**：launchctl 有标签 com.zerg.ghost-unregistered") {
			t.Fatalf("未点名未登记幽灵：%q", so)
		}
		if !strings.Contains(so, "⇒ 门禁红（rc=1）") {
			t.Fatalf("未走红档：%q", so)
		}
	})

	t.Run("负控:ps 多一件未登记进程 rc=1（R4）", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		csdWrite(t, pp, csdGoodPS+"  99999     1       01:00 /repo/bin/zerg-wall\n")
		rc, so, _ := csdRun(t, root)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "FAIL R4 现值进程没有落点") {
			t.Fatalf("R4 未点名：%q", so)
		}
	})

	t.Run("负控:ghost 无处置 rc=1（R5）", func(t *testing.T) {
		root := csdTestRoot(t)
		noDisp := strings.ReplaceAll(csdGoodDecl, "进 reap 候选 · 待 Mr2109 拍", "-")
		dp, pp, lp, home := csdWriteFixture(root, noDisp, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "FAIL R5 行 5 的 ghost（cocoon-docs-service）没有处置") {
			t.Fatalf("R5 未点名：%q", so)
		}
	})

	t.Run("负控:声明件缺件 rc=1（R3）", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, []string{"com.zerg.core.plist"})
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "FAIL R3 行 4 的声明件不存在") {
			t.Fatalf("R3 未点名：%q", so)
		}
	})

	t.Run("负控:数目同·名字不同 双向点名 rc=1", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		csdWrite(t, lp, "68939\t0\tcom.zerg.core\n86298\t0\tcom.zerg.tunnel.mini-node-1\n")
		rc, so, _ := csdRun(t, root)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "`A∖B`") || !strings.Contains(so, "`B∖A`") {
			t.Fatalf("两侧未分开点名：%q", so)
		}
		if !strings.Contains(so, "com.zerg.tunnel.mini-node-1") || !strings.Contains(so, "com.zerg.agent-Mr2109") {
			t.Fatalf("两侧名字未逐字出现：%q", so)
		}
	})

	t.Run("正控:闭集外标签不进集合 rc=0", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		csdWrite(t, lp, csdGoodLC+"77777\t0\tcom.example.zerg-probe\n")
		rc, so, _ := csdRun(t, root)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0", rc)
		}
		if !strings.Contains(so, "R2 闭集外 1 行（未覆盖形态 · 不进集合）：com.example.zerg-probe") {
			t.Fatalf("闭集外未另印：%q", so)
		}
	})

	t.Run("正控:已登记幽灵不算幽灵 rc=0", func(t *testing.T) {
		root := csdTestRoot(t)
		decl := csdGoodDecl + "ghost\tcom.zerg.ghost-registered\t-\t-\t-\t测试\t待拍\n"
		dp, pp, lp, home := csdWriteFixture(root, decl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		csdWrite(t, lp, csdGoodLC+"88888\t0\tcom.zerg.ghost-registered\n")
		rc, so, _ := csdRun(t, root)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（已登记幽灵是人签设计）", rc)
		}
		if !strings.Contains(so, "R2 `A∖B` 已登记：com.zerg.ghost-registered") {
			t.Fatalf("未走已登记档：%q", so)
		}
	})

	// ── 空值三态：取不到 / 显式空集合 / 缺件，逐态成格并断言「不出现对方文案」──────────────
	t.Run("三态:声明树取不到 rc=2 且不出空转文案", func(t *testing.T) {
		root := csdTestRoot(t)
		_, pp, lp, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, filepath.Join(root, "不存在.tsv"), pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(so, "缺件：声明树不存在") {
			t.Fatalf("未走取不到档：%q", so)
		}
		if strings.Contains(so, "声明树里 0 条 launchd 声明") {
			t.Fatalf("取不到态混进了空集合文案：%q", so)
		}
	})

	t.Run("三态:声明树显式空集合 rc=2 且不出缺件文案", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, lp, home := csdWriteFixture(root, "# 只有注释\nkind\tname\t声明件\t程序\t进程特征\t归属\t处置\ngaunt\tx\t-\t-\t-\t-\t-\n", csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(so, "空转：声明树里 0 条 launchd 声明") {
			t.Fatalf("未走空集合档：%q", so)
		}
		if strings.Contains(so, "缺件：声明树不存在") {
			t.Fatalf("空集合态混进了取不到文案：%q", so)
		}
	})

	t.Run("三态:ps 夹具缺件 rc=2 且不出空转文案", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, _, lp, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, filepath.Join(root, "no-such-ps.txt"), lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(so, "不可判：ps 取不到（夹具文件不存在：") {
			t.Fatalf("未走夹具缺件档：%q", so)
		}
		if strings.Contains(so, "空转：ps 里一件自研件") {
			t.Fatalf("缺件态混进了空集合文案：%q", so)
		}
	})

	t.Run("三态:ps 显式空集合 rc=2 且不出夹具缺件文案", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, "    1     0  00:00 /sbin/launchd\n", csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(so, "空转：ps 里一件自研件（bin/zerg-…）都没扫到（不许当绿）") {
			t.Fatalf("未走 ps 空集合档：%q", so)
		}
		if strings.Contains(so, "夹具文件不存在") {
			t.Fatalf("空集合态混进了夹具缺件文案：%q", so)
		}
	})

	t.Run("负控:launchctl 夹具缺件 rc=2", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, _, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, filepath.Join(root, "no-such-lc.txt"), home)
		rc, so, _ := csdRun(t, root)
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(so, "不可判：launchctl 取不到（夹具文件不存在：") {
			t.Fatalf("未走 launchctl 缺件档：%q", so)
		}
	})

	t.Run("负控:根上找不到 core/cmd/zerg rc=2", func(t *testing.T) {
		dir := t.TempDir()
		rc, so, _ := csdRun(t, dir)
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(so, "不给结论：找不到 core/cmd/zerg") {
			t.Fatalf("未走找不到根档：%q", so)
		}
	})

	t.Run("--self-test rc=0 二十格全过", func(t *testing.T) {
		rc, so, se := csdRun(t, "--self-test")
		if rc != 0 {
			t.Fatalf("self-test rc=%d 期望 0（stderr=%q）", rc, se)
		}
		if !strings.Contains(so, "自检结论: 全过") {
			t.Fatalf("自检结论不对：%q", so)
		}
		if n := strings.Count(so, "（rc="); n != 20 {
			t.Fatalf("自检格数 = %d 期望 20：%q", n, so)
		}
		if !strings.Contains(so, "✓ 点名断言（`A∖B` + `B∖A` 两侧逐字出现）") {
			t.Fatalf("点名断言行不对：%q", so)
		}
	})

	t.Run("-h rc=0 且 sha256 逐字命中 golden", func(t *testing.T) {
		rc, so, se := csdRun(t, "-h")
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0", rc)
		}
		if se != "" {
			t.Fatalf("帮助面 stderr 应为空：%q", se)
		}
		sum := sha256.Sum256([]byte(so))
		got := hex.EncodeToString(sum[:])
		if got != csdHelpGoldenSHA256 {
			t.Fatalf("-h stdout sha256 = %s 期望 golden %s（正文=%q）", got, csdHelpGoldenSHA256, so)
		}
	})

	t.Run("负控:未知旗标 rc=2 走 stderr", func(t *testing.T) {
		rc, so, se := csdRun(t, "--nope")
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if so != "" {
			t.Fatalf("用法错不应出 stdout：%q", so)
		}
		if !strings.Contains(se, "check-service-declaration.py: error: unrecognized arguments: --nope") {
			t.Fatalf("未报未知旗标：%q", se)
		}
	})

	t.Run("负控:--json rc=2 机器面缺明说", func(t *testing.T) {
		rc, so, se := csdRun(t, "--json")
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if so != "" {
			t.Fatalf("--json 拒收不应出 stdout：%q", so)
		}
		if !strings.Contains(se, "缺口 Q-176") || !strings.Contains(se, "字段表：无（本门只出人读面）") {
			t.Fatalf("机器面缺未明说：%q", se)
		}
		if !strings.Contains(se, "**机器面缺 ≠ 零命中**") {
			t.Fatalf("两态未分开：%q", se)
		}
	})

	t.Run("负控:--json=form 也拒收 rc=2", func(t *testing.T) {
		rc, _, se := csdRun(t, "--json=out")
		if rc != 2 {
			t.Fatalf("rc=%d 期望 2", rc)
		}
		if !strings.Contains(se, "`--json=out`") {
			t.Fatalf("未逐字回显旗标：%q", se)
		}
	})

	// ── R4a / 词界锚 / 层限（GAP-20260927-100 · GAP-20260927-168）· 带牙面 ─────────────────────
	// ★ 牙齿：把主件 `csdWordAnchor` 哑化成子串包含（`strings.Contains`）⇒ 下面「词界锚」与「R4a 兄弟名」
	//   两格必 FAIL（`bin/zerg-core` 是 `bin/zerg-core-test` 的子串 ⇒ 假绿、rc 由 1 变 0）。
	t.Run("负控:词界锚 兄弟名不假绿 rc=1（R4a+R4b）", func(t *testing.T) {
		root := csdTestRoot(t)
		badPS := "  68939     1       56:36 /repo/bin/zerg-core-test\n" +
			"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
			"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, badPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1（子串锚会假绿成 0）", rc)
		}
		if !strings.Contains(so, "标签↔pid↔程序不一致") {
			t.Fatalf("R4a 未判「标签↔pid↔程序不一致」：%q", so)
		}
		if !strings.Contains(so, "FAIL R4 现值进程没有落点") {
			t.Fatalf("R4b 未判未落点：%q", so)
		}
	})

	t.Run("负控:R4a 标签↔pid↔程序不一致 rc=1", func(t *testing.T) {
		root := csdTestRoot(t)
		badPS := "  68939     1       56:36 /repo/bin/zerg-agentd\n" + // com.zerg.core 的 pid 上跑别件
			"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
			"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, badPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "FAIL R4 声明 com.zerg.core（第 3 行）的作业 pid=68939") {
			t.Fatalf("R4a 未逐条点名：%q", so)
		}
	})

	t.Run("负控:R4a 瞬态工人（ps 已无该 pid）rc=1", func(t *testing.T) {
		root := csdTestRoot(t)
		badPS := "  68939     1       56:36 /repo/bin/zerg-core\n" +
			"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n" // 14149 消失
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, badPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1", rc)
		}
		if !strings.Contains(so, "在 ps 现值里取不到命令行 ⇒ 锚不了") {
			t.Fatalf("R4a 未判瞬态：%q", so)
		}
	})

	t.Run("正控:词界锚 shell 包装仍命中 rc=0", func(t *testing.T) {
		root := csdTestRoot(t)
		shPS := "  68939     1       56:36 /bin/sh -c exec bin/zerg-core\n" +
			"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
			"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, shPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（词界锚对行尾命中）：%q", rc, so)
		}
	})

	t.Run("正控:searxng 外置进程有落点 rc=0（R4b 覆盖）", func(t *testing.T) {
		root := csdTestRoot(t)
		decl := csdGoodDecl + "launchd\tcom.zerg.searxng\t~/Library/LaunchAgents/com.zerg.searxng.plist\t" +
			"~/zerg-searxng/src/.venv/bin/python -m searx.webapp\tsearx.webapp\t主控\t-\thost\n"
		ps := csdGoodPS + "  59975     1       11:25 ~/zerg-searxng/src/.venv/bin/python -m searx.webapp\n"
		dp, pp, lp, home := csdWriteFixture(root, decl, ps, csdGoodLC+"59975\t0\tcom.zerg.searxng\n",
			[]string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist", "com.zerg.searxng.plist"})
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（searxng 进程被声明面覆盖）: %q", rc, so)
		}
		if !strings.Contains(so, "⇒ 全绿（rc=0）") {
			t.Fatalf("未走全绿档：%q", so)
		}
	})

	t.Run("负控:searxng 进程无落点 rc=1（R4b · searx.webapp 是 hint 的牙）", func(t *testing.T) {
		root := csdTestRoot(t)
		ps := csdGoodPS + "  59975     1       11:25 ~/zerg-searxng/src/.venv/bin/python -m searx.webapp\n"
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, ps, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 1 {
			t.Fatalf("rc=%d 期望 1（去掉 searx.webapp hint 会假绿成 0）", rc)
		}
		if !strings.Contains(so, "FAIL R4 现值进程没有落点") || !strings.Contains(so, "searx.webapp") {
			t.Fatalf("R4b 未点名 searxng 进程行：%q", so)
		}
	})

	t.Run("正控:R0 层限注记在场（8 列 host）rc=0", func(t *testing.T) {
		root := csdTestRoot(t)
		dp, pp, lp, home := csdWriteFixture(root, csdGoodDecl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0", rc)
		}
		if !strings.Contains(so, "R0 层限：本门判面 = `layer=host`（**本机层**）· 本层 3 行 · 非本层 0 行") {
			t.Fatalf("R0 层限注记不对：%q", so)
		}
	})

	t.Run("正控:层限 非本层行不由本门判 rc=0", func(t *testing.T) {
		root := csdTestRoot(t)
		decl := csdGoodDecl + "launchd	edge-node-agent.service	/home/<user>/edge-node-agent.service	" +
			"/home/<user>/zerg/bin/zerg-agentd	bin/zerg-agentd	节点(edge-node)	-	node:edge-node\n"
		dp, pp, lp, home := csdWriteFixture(root, decl, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0（非本层行不由本门判）", rc)
		}
		if !strings.Contains(so, "本层 3 行 · 非本层 1 行") {
			t.Fatalf("层计数不对：%q", so)
		}
	})

	t.Run("负控:无 layer 列（旧 7 列件）出补列注记", func(t *testing.T) {
		root := csdTestRoot(t)
		old7 := strings.ReplaceAll(csdGoodDecl, "	layer\n", "\n")
		old7 = strings.ReplaceAll(old7, "	host\n", "\n")
		dp, pp, lp, home := csdWriteFixture(root, old7, csdGoodPS, csdGoodLC, goodPlists)
		csdSetEnv(t, dp, pp, lp, home)
		rc, so, _ := csdRun(t, root)
		if rc != 0 {
			t.Fatalf("rc=%d 期望 0", rc)
		}
		if !strings.Contains(so, "层限：声明树无 `layer` 列（旧 7 列件）") {
			t.Fatalf("缺 `layer` 列未出补列注记：%q", so)
		}
	})
}
