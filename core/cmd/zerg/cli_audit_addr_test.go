// cli_audit_addr_test.go —— `zerg audit hardcoded-addr` 的成对判据（缺口 `GAP-20261010-20`）。
//
// 为什么不靠「跑一遍看输出对不对」：本命令是**清单档**（默认有命中仍退 0）⇒ 只测 rc 等于什么都没测。
// 这里逐条钉住**四件行为**（每条都有成对的正/负控）：
//
//	① 清单**只收该收的**：代码行里的机器地址进；`*_test.go` 夹具 / 注释行 / 回环（白名单合法写法）都**不进**；
//	② `--fail-on-hit` 是**唯一**把命中透成 rc=1 的档（默认档永不给 1）；
//	③ `--json <字段>` 的字段面照登记表出（每行一对象 · 字段齐）；
//	④ 豁免件 `路径:子串` **真生效**：加一行 ⇒ 该处从清单里消失且总数 -1（这一格是白名单的反面证据）。
//
// 落点与同族一致：`package main_test` ＋ `zerg.RunForTest`（跑**当前源码**行为）· 盘面全在 `t.TempDir()`。
// ★ 夹具地址一律**拼接构造**（同 `check_hardcoded_private_paths.go` 的写法）—— 本件虽是 `_test.go`
// （不在扫描面内），仍守同族纪律，免得日后有人把夹具挪进非测试件。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditHardcodedAddrSweep(t *testing.T) {
	root := t.TempDir()
	addr := "10." + "0.0.9" // 拼接构造（同族纪律）
	loop := "127." + "0.0.1:8580"

	// ① 生产码里的机器地址 ⇒ 进清单。
	mustWrite(t, filepath.Join(root, "core", "a.go"),
		"package a\n\nvar u = \"http://"+addr+":8580/x\"\n")
	// ② 测试夹具（整件排除）。
	mustWrite(t, filepath.Join(root, "core", "a_test.go"),
		"package a\n\nvar v = \""+addr+"\"\n")
	// ③ 注释行（不编译进二进制 ⇒ 排除）。
	mustWrite(t, filepath.Join(root, "core", "c.go"),
		"package a\n\n// 历史：曾写死 http://"+addr+":8580（注释不算）\nvar x = 1\n")
	// ④ 回环（值白名单里的合法写法 ⇒ 永不报）。
	mustWrite(t, filepath.Join(root, "core", "d.go"),
		"package a\n\nvar ready = \""+loop+"\"\n")
	// ⑤ 豁免件先落成**空表**（判 ④ 的负控要靠它起步）。
	allowPath := filepath.Join(root, "scripts", "audit-addr-allow.txt")
	mustWrite(t, allowPath, "# 空表\n")

	// ---- ① 清单内容：只有 core/a.go:3 那一处 ----
	rc, out, errs := runZergRepo(t, root, "audit", "hardcoded-addr")
	if rc != 0 {
		t.Fatalf("默认档 rc=%d（要 0 —— 清单档不判红）；stderr=%s", rc, errs)
	}
	if !strings.Contains(out, "core/a.go:3:") {
		t.Errorf("清单里没有真命中 `core/a.go:3:`；stdout=%q", out)
	}
	for _, unwanted := range []string{"a_test.go", "core/c.go", "core/d.go"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("%s 不该进清单（夹具 / 注释 / 回环），但出现了；stdout=%q", unwanted, out)
		}
	}
	if !strings.Contains(errs, "命中 **1** 处") {
		t.Errorf("总数读数不对（要「命中 **1** 处」）；stderr=%s", errs)
	}

	// ---- ② `--fail-on-hit`：命中 > 0 ⇒ 1（且只有它才给 1）----
	rc, _, errs = runZergRepo(t, root, "audit", "hardcoded-addr", "--fail-on-hit")
	if rc != 1 {
		t.Fatalf("`--fail-on-hit` rc=%d（要 1）；stderr=%s", rc, errs)
	}

	// ---- ③ `--json path,line,text`：每行一对象 · 字段齐 ----
	rc, out, errs = runZergRepo(t, root, "audit", "hardcoded-addr", "--json", "path,line,text")
	if rc != 0 {
		t.Fatalf("`--json` rc=%d（要 0）；stderr=%s", rc, errs)
	}
	items := envelopeItems(t, out)
	if len(items) != 1 {
		t.Fatalf("`--json` items=%d（要 1）；out=%s", len(items), out)
	}
	for _, f := range []string{"path", "line", "text"} {
		if items[0][f] == "" {
			t.Errorf("`--json` 第 1 个对象缺字段 %q；got=%v", f, items[0])
		}
	}
	if items[0]["path"] != "core/a.go" || items[0]["line"] != "3" {
		t.Errorf("`--json` 定位不对：path=%q line=%q（要 core/a.go / 3）", items[0]["path"], items[0]["line"])
	}
	if len(items[0]) != 3 {
		t.Errorf("`--json path,line,text` 只该出点的三格，实际 %d 格：%v", len(items[0]), items[0])
	}

	// ---- ④ 豁免件真生效：加一行 ⇒ 该处消失 · 总数 -1 ----
	if err := os.WriteFile(allowPath, []byte("# 豁免一条（判据 ④）\ncore/a.go:"+addr+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errs = runZergRepo(t, root, "audit", "hardcoded-addr", "--fail-on-hit")
	if rc != 0 {
		t.Fatalf("豁免生效后 `--fail-on-hit` rc=%d（要 0 —— 清单已空）；stderr=%s", rc, errs)
	}
	if strings.Contains(out, "core/a.go:3:") {
		t.Errorf("被豁免的那处仍在清单里（白名单没生效）；stdout=%q", out)
	}
	if !strings.Contains(errs, "命中 **0** 处") || !strings.Contains(errs, "豁免放过 1 处") {
		t.Errorf("豁免后的两格读数不对（要「命中 **0** 处」+「豁免放过 1 处」）；stderr=%s", errs)
	}

	// ---- ⑤ 用法错两格（机器面：不猜字形，只看退码）----
	if rc, _, _ = runZergRepo(t, root, "audit", "hardcoded-addr", "--json", "nosuchfield"); rc != 2 {
		t.Errorf("未知字段 rc=%d（要 2）", rc)
	}
	if rc, _, _ = runZergRepo(t, root, "audit", "hardcoded-addr", "--root", filepath.Join(root, "不存在-zz")); rc != 8 {
		t.Errorf("仓根不在 rc=%d（要 8 · 不给结论，不许当「零命中」）", rc)
	}
}
