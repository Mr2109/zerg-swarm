package main

// check_script_registration_test.go —— 门135 的正控/反控（缺口 `GAP-20261020-03` 的**两态自证**）。
//
// 夹具全部落 `t.TempDir()`：合成仓根（`scripts/` + `scripts/公开标记.tsv`），一个字节不落真仓；
// 入口显式给 `--root`（✗ 不靠 cwd 猜），进进程内分派（✗ 不起子进程）。

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// csrFixture —— 造一棵最小合成仓根：scripts/<文件…> + 登记表行。
func csrFixture(t *testing.T, scripts map[string]string, marks string, withMarks bool) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range scripts {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("建目录失败：%v", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("写件失败：%v", err)
		}
	}
	if withMarks {
		p := filepath.Join(root, filepath.FromSlash(csrMarksRel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("建目录失败：%v", err)
		}
		if err := os.WriteFile(p, []byte(marks), 0o644); err != nil {
			t.Fatalf("写登记表失败：%v", err)
		}
	}
	return root
}

func csrCall(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkScriptRegistrationGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// TestScriptRegistrationTwoStates —— 正控（全登记 ⇒ 0）/ 反控（漏一件 ⇒ 1 且点名）/
// fail-closed 三档（登记表缺 · 空域 · 用法错）。
func TestScriptRegistrationTwoStates(t *testing.T) {
	allRegistered := "scripts/gates/check-a.sh\tno\tB 类·门禁\nscripts/ops/run.py\tno\tB 类·运维\n"
	root := csrFixture(t, map[string]string{
		"scripts/gates/check-a.sh": "# a\n",
		"scripts/ops/run.py":       "# b\n",
	}, allRegistered, true)

	rc, out, errs := csrCall(t, "--root", root)
	if rc != 0 {
		t.Fatalf("正控：已登记应放行 rc=0，实测 rc=%d\n%s%s", rc, out, errs)
	}
	if !strings.Contains(out, "未登记 0 件") {
		t.Fatalf("正控：读数应含「未登记 0 件」，实测：%s", out)
	}

	// 反控：盘上多一件未登记的脚本 ⇒ 必须被拦，且点名这一件。
	if err := os.WriteFile(filepath.Join(root, "scripts", "ops", "new_one.sh"), []byte("# new\n"), 0o644); err != nil {
		t.Fatalf("加反控件失败：%v", err)
	}
	rc, out, errs = csrCall(t, "--root", root)
	if rc != 1 {
		t.Fatalf("反控：未登记脚本应被拦 rc=1，实测 rc=%d\n%s%s", rc, out, errs)
	}
	if !strings.Contains(out, "✗ 未登记：scripts/ops/new_one.sh") {
		t.Fatalf("反控：应点名 scripts/ops/new_one.sh，实测：%s", out)
	}
	if !strings.Contains(out, "未登记 1 件") || !strings.Contains(out, csrSyncHint) {
		t.Fatalf("反控：读数应含「未登记 1 件」且给出治法 `%s`，实测：%s", csrSyncHint, out)
	}

	// 补一行登记（治法落地）⇒ 回绿：同一棵树、只差登记表一行。
	f, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(csrMarksRel)), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("打开登记表失败：%v", err)
	}
	if _, err := f.WriteString("scripts/ops/new_one.sh\tno\t反控补登\n"); err != nil {
		t.Fatalf("补登失败：%v", err)
	}
	_ = f.Close()
	if rc, out, errs = csrCall(t, "--root", root); rc != 0 {
		t.Fatalf("补登后应回绿 rc=0，实测 rc=%d\n%s%s", rc, out, errs)
	}

	// fail-closed ①：登记表不在盘 ⇒ rc=2（「没有」不许当「全登记」）。
	rootNoMarks := csrFixture(t, map[string]string{"scripts/a.sh": "# a\n"}, "", false)
	if rc, _, _ = csrCall(t, "--root", rootNoMarks); rc != 2 {
		t.Fatalf("登记表缺应 rc=2，实测 rc=%d", rc)
	}

	// fail-closed ②：空域（scripts/ 在、0 件脚本）⇒ rc=2。
	rootEmpty := csrFixture(t, map[string]string{}, "scripts/gates/check-a.sh\tno\tx\n", true)
	if err := os.MkdirAll(filepath.Join(rootEmpty, "scripts"), 0o755); err != nil {
		t.Fatalf("建 scripts 失败：%v", err)
	}
	if rc, _, _ = csrCall(t, "--root", rootEmpty); rc != 2 {
		t.Fatalf("空域应 rc=2，实测 rc=%d", rc)
	}

	// fail-closed ③：仓根不在盘 ⇒ rc=2（✗ 不许「取不到」当绿）。
	if rc, _, _ = csrCall(t, "--root", filepath.Join(root, "no-such-dir")); rc != 2 {
		t.Fatalf("仓根不在盘应 rc=2，实测 rc=%d", rc)
	}

	// 用法错 ⇒ rc=2（未知旗标）。
	if rc, _, _ = csrCall(t, "--root", root, "--nope"); rc != 2 {
		t.Fatalf("未知旗标应 rc=2，实测 rc=%d", rc)
	}
}

// TestScriptRegistrationRealRepoIsClean —— 真仓现读：`scripts/` 下每一件都有登记行。
// 这条是「正门在真仓上真的通」的机检（新增一件没登记 ⇒ 本测试与提交闸同步红）。
func TestScriptRegistrationRealRepoIsClean(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("定位仓根失败：%v", err)
	}
	rc, out, errs := csrCall(t, "--root", root)
	if rc != 0 {
		t.Fatalf("真仓应 rc=0（未登记 0 件），实测 rc=%d\n%s%s", rc, out, errs)
	}
	if !strings.Contains(out, "未登记 0 件") {
		t.Fatalf("真仓读数应含「未登记 0 件」，实测：%s", out)
	}
}
