// check_newcmd_followup_test.go —— 成对负控（本件与 check_newcmd_followup.go 同批）
//
// 牙口说明：下列子用例逐条钉住九个面 / 三档退码 / 遍历序。任意一条被**哑化**
// （例：把面判定恒置「齐」、把 rc=2 档改成 rc=0、把实现件排序读法换成 readdir 原序），
// 对应子用例立刻 FAIL 并给出**行号**（见回执的哑化实证）。
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func cnfT(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkNewcmdFollowupGateMain(&out, &errb, append([]string{"--root", root}, args...))
	return rc, out.String(), errb.String()
}

func TestCnfFaces_正控与负控成对(t *testing.T) {
	t.Run("①正控_无新增_rc0", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		rc, out, _ := cnfT(t, d)
		if rc != 0 {
			t.Fatalf("期望 rc=0 实得 %d\n%s", rc, out)
		}
		if !strings.Contains(out, "新增 0 条") {
			t.Fatalf("空转读数缺失（0 条与没跑分不开）:\n%s", out)
		}
	})

	t.Run("②负控_面①dryrun未登记", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, true, true, false, "zerg nu", nil)
		cnfAppendFile(cnfPyJoin(d, "core", "cmd", "zerg", "cmdNu.go"), cnfSelfDrySite)
		rc, out, _ := cnfT(t, d)
		if rc != 1 || !strings.Contains(out, "面① dryrun 落点：缺") {
			t.Fatalf("期望 rc=1 且点名面① 实得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("③负控_面②矩阵缺", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, true, false, false, "zerg nu", nil)
		rc, out, _ := cnfT(t, d)
		if rc != 1 || !strings.Contains(out, "面② cli-matrix must-fail：缺") {
			t.Fatalf("期望 rc=1 且点名面② 实得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("④负控_面④public=no缺侧甲", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, true, true, false, "zerg nu", nil)
		mk := cnfPyJoin(d, cnfMarksRel)
		src, _ := cnfReadText(mk)
		cnfWrite(mk, strings.Replace(src, "core/cmd/zerg/cmdNu.go\tyes\t", "core/cmd/zerg/cmdNu.go\tno\t", 1))
		rc, out, _ := cnfT(t, d)
		if rc != 1 || !strings.Contains(out, "两器都发") {
			t.Fatalf("期望 rc=1 且点名两器都发 实得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("⑤负控_面⑤取值旗标不认", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, true, true, false, "zerg nu --ghost <值>", nil)
		rc, out, _ := cnfT(t, d)
		if rc != 1 || !strings.Contains(out, "面⑤ 门73 旗标名表：缺") {
			t.Fatalf("期望 rc=1 且点名面⑤ 实得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("⑥正控半边_旗标已登记_rc0", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, true, true, false, "zerg nu --ghost <值>", []string{"--ghost"})
		rc, out, _ := cnfT(t, d)
		if rc != 0 {
			t.Fatalf("期望 rc=0 实得 %d\n%s", rc, out)
		}
	})

	t.Run("⑦负控_面⑥闭集外kind", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, true, true, false, "zerg nu", nil)
		cnfAppendFile(cnfPyJoin(d, "core", "cmd", "zerg", "cmdNu.go"), cnfSelfSetErr)
		rc, out, _ := cnfT(t, d)
		if rc != 1 || !strings.Contains(out, "面⑥ 门⑤ error.kind 闭集：缺") {
			t.Fatalf("期望 rc=1 且点名面⑥ 实得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("⑧负控_面⑦未登记退码", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, true, true, false, "zerg nu", nil)
		cnfAppendFile(cnfPyJoin(d, "core", "cmd", "zerg", "cmdNu.go"), cnfSelfExit99)
		rc, out, _ := cnfT(t, d)
		if rc != 1 || !strings.Contains(out, "面⑦ 门⑨ 退码表+命名空间：缺") {
			t.Fatalf("期望 rc=1 且点名面⑦ 实得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("⑨正控半边_退码已登记_rc0", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, true, true, false, "zerg nu", nil)
		cnfAppendFile(cnfPyJoin(d, "core", "cmd", "zerg", "cmdNu.go"), cnfSelfExit14)
		rc, out, _ := cnfT(t, d)
		if rc != 0 {
			t.Fatalf("期望 rc=0 实得 %d\n%s", rc, out)
		}
	})

	t.Run("⑩负控_面③计数漂移", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, true, "zerg nu", nil)
		rc, out, _ := cnfT(t, d)
		if rc != 1 || !strings.Contains(out, "面③ cli-contract-baseline 计数：缺") {
			t.Fatalf("期望 rc=1 且点名面③ 实得 rc=%d\n%s", rc, out)
		}
	})
}

func TestCnfFaces_不给结论档(t *testing.T) {
	t.Run("解析漏读_rc2", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		mp := cnfPyJoin(d, cnfMainRel)
		src, _ := cnfReadText(mp)
		cnfWrite(mp, strings.ReplaceAll(src, "\t}\n}\n",
			"\t\t{\n\t\t\tpath: []string{},\n\t\t\trun: cmdAlpha,\n\t\t},\n\t}\n}\n"))
		rc, out, _ := cnfT(t, d)
		if rc != 2 || !strings.Contains(out, "解析漏读") {
			t.Fatalf("期望 rc=2 解析漏读 实得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("缺基线件_rc2", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		_ = os.Remove(cnfPyJoin(d, cnfOwnBaselineRel))
		rc, out, _ := cnfT(t, d)
		if rc != 2 || !strings.Contains(out, "基线件缺失") {
			t.Fatalf("期望 rc=2 缺基线 实得 rc=%d\n%s", rc, out)
		}
	})

	t.Run("取不到命令树块_rc2", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		_ = os.Remove(cnfPyJoin(d, cnfMainRel))
		rc, out, _ := cnfT(t, d)
		if rc != 2 || !strings.Contains(out, "取不到命令树块") {
			t.Fatalf("期望 rc=2 取不到树 实得 rc=%d\n%s", rc, out)
		}
	})
}

func TestCnfFaces_遍历序_子目录夹中间(t *testing.T) {
	d := t.TempDir()
	cnfMiniRoot(d, true, true, false, "zerg nu", nil)
	pkg := cnfPyJoin(d, "core", "cmd", "zerg")
	// 按**非字典序**创建 + 子目录夹在中间（readdir 原序 ≠ sorted）
	cnfWrite(cnfPyJoin(pkg, "zz.go"), "package main\n\nfunc cmdNu() {}\n")
	cnfWrite(cnfPyJoin(pkg, "sub", "mm.go"), "package main\n\nfunc cmdNu() {}\n")
	cnfWrite(cnfPyJoin(pkg, "mm.go"), "package main\n\nfunc cmdNu() {}\n")
	rc, out, _ := cnfT(t, d)
	want := "实现件 core/cmd/zerg/cmdNu.go、core/cmd/zerg/mm.go、core/cmd/zerg/zz.go）"
	if !strings.Contains(out, want) {
		t.Fatalf("实现件序非 sorted（早钻/原序读法会分叉）rc=%d\nwant 含: %s\n%s", rc, want, out)
	}
	if strings.Contains(out, "sub/mm.go") || strings.Contains(out, "sub\\mm.go") {
		t.Fatalf("子目录被下钻（py os.listdir 不下钻）:\n%s", out)
	}
}

func TestCnfFaces_读不了的件静默跳过(t *testing.T) {
	d := t.TempDir()
	cnfMiniRoot(d, true, true, false, "zerg nu", nil)
	bad := cnfPyJoin(d, "core", "cmd", "zerg", "cmdNu.go")
	if err := os.Chmod(bad, 0); err != nil {
		t.Skip("chmod 不支持")
	}
	defer os.Chmod(bad, 0o644)
	rc, out, _ := cnfT(t, d)
	if rc != 1 || !strings.Contains(out, "实现件未定位 ⇒ 缺（判不了）") {
		t.Fatalf("期望读不了件后 rc=1 且面①「判不了」 实得 rc=%d\n%s", rc, out)
	}
}

func TestCnfFaces_显式null档(t *testing.T) {
	// 成对负控：**显式 null ⇒ rc=2 + 具名文案**；**缺键 ⇒ 原默认（与 py 逐字同）**。
	// 哑化（把 null 判定归零 ⇒ 走缺键默认）时下列 null_* 子用例必须 FAIL 并给出本件行号。
	t.Run("null_paths_rc2", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		cnfWrite(cnfPyJoin(d, cnfOwnBaselineRel),
			`{"schema": "`+cnfSchema+`", "head": "fixture", "count": 0, "paths": null}`)
		rc, out, _ := cnfT(t, d)
		if rc != 2 || !strings.Contains(out, "`paths` 为显式 null") {
			t.Fatalf("显式 null `paths` 期望 rc=2 且具名 实得 rc=%d\n%s", rc, out)
		}
	})
	t.Run("missing_paths_原默认", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		cnfWrite(cnfPyJoin(d, cnfOwnBaselineRel),
			`{"schema": "`+cnfSchema+`", "head": "fixture", "count": 0}`)
		rc, out, _ := cnfT(t, d)
		if rc != 0 {
			t.Fatalf("缺键 `paths` 期望原默认 rc=0 实得 %d\n%s", rc, out)
		}
	})
	t.Run("null_cases_rc2", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		cnfWriteJSON(cnfPyJoin(d, cnfContractBaseline), map[string]interface{}{
			"id": "cli-contract-baseline.v1", "blanks": 0, "mustfail_cases": 0, "commands": 2})
		cnfWrite(cnfPyJoin(d, cnfMatrixRel), `{"id": "cli-contract.v1", "cases": null}`)
		rc, out, _ := cnfT(t, d)
		if rc != 2 || !strings.Contains(out, "`cases` 为显式 null") {
			t.Fatalf("显式 null `cases` 期望 rc=2 且具名 实得 rc=%d\n%s", rc, out)
		}
	})
	t.Run("missing_cases_原默认", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		cnfWriteJSON(cnfPyJoin(d, cnfContractBaseline), map[string]interface{}{
			"id": "cli-contract-baseline.v1", "blanks": 0, "mustfail_cases": 0, "commands": 2})
		cnfWrite(cnfPyJoin(d, cnfMatrixRel), `{"id": "cli-contract.v1"}`)
		rc, out, _ := cnfT(t, d)
		if rc != 0 {
			t.Fatalf("缺键 `cases` 期望原默认 rc=0 实得 %d\n%s", rc, out)
		}
	})
	t.Run("null_件_rc2", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		cnfWrite(cnfPyJoin(d, cnfDryrunRegRel),
			`{"schema": "zerg/dryrun-semantics/v1", "件": null}`)
		rc, out, _ := cnfT(t, d)
		if rc != 2 || !strings.Contains(out, "`件` 为显式 null") {
			t.Fatalf("显式 null `件` 期望 rc=2 且具名 实得 rc=%d\n%s", rc, out)
		}
	})
	t.Run("missing_件_原默认", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		cnfWrite(cnfPyJoin(d, cnfDryrunRegRel), `{"schema": "zerg/dryrun-semantics/v1"}`)
		rc, out, _ := cnfT(t, d)
		if rc != 0 {
			t.Fatalf("缺键 `件` 期望原默认 rc=0 实得 %d\n%s", rc, out)
		}
	})
	t.Run("null_namespaces_rc2", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		cnfWrite(cnfPyJoin(d, cnfNamespacesRel), `{"schema": "x", "namespaces": null}`)
		rc, out, _ := cnfT(t, d)
		if rc != 2 || !strings.Contains(out, "为显式 null") {
			t.Fatalf("显式 null `namespaces` 期望 rc=2 且具名 实得 rc=%d\n%s", rc, out)
		}
	})
	t.Run("null_ns_table_rc2", func(t *testing.T) {
		d := t.TempDir()
		cnfMiniRoot(d, false, true, false, "zerg nu", nil)
		cnfWrite(cnfPyJoin(d, cnfNamespacesRel),
			`{"schema": "x", "namespaces": [{"id": "cli", "table": null}]}`)
		rc, out, _ := cnfT(t, d)
		if rc != 2 || !strings.Contains(out, "为显式 null") {
			t.Fatalf("显式 null `namespaces[].table` 期望 rc=2 且具名 实得 rc=%d\n%s", rc, out)
		}
	})
}

func TestCnfFaces_自检15控全过(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkNewcmdFollowupGateMain(&out, &errb, []string{"--self-test"})
	if rc != 0 || !strings.Contains(out.String(), "自检通过：15 控") {
		t.Fatalf("自检期望 rc=0 实得 %d\n%s%s", rc, out.String(), errb.String())
	}
}
