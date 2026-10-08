// check_build_selfclean_test.go —— `check_build_selfclean.go`（M2 · 搬运粒20）的成对负控测试件。
//
// 覆盖：
//
//	· 正控（四格全过 ⇒ rc=0）＋ 四种负控（K1/K2/K3/K4）＋ 缺件/空件（rc=2）＋ 坏件（chmod 000）
//	· argparse 面（-h / 未知旗标 / --root 缺参 / 前缀缩写）
//	· `--json` 空值三形态之一：空集**显式在场**（`"reds": []`，✗ 不是 Nil→null/吞键）
//	· A 类回归牙：`--self-test` 的临时夹具目录**回收**（同前缀目录增量 == 0）；删 `defer os.RemoveAll` ⇒ 本用例必红
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

const bscTestGreen = "#!/usr/bin/env bash\n" +
	"# 自净化\nunset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR\n" +
	"TMPDIR=${ZERG_BUILD_TMP:-/tmp/zerg-build}\n" +
	"if ! mkdir -p \"$TMPDIR\"; then echo \"自净化失败：仓外 TMPDIR 建不起来 ⇒ 拒绝重编\" >&2; exit 1; fi\n" +
	"export TMPDIR=\"$ZERG_BUILD_TMP\"\n" +
	"# 解析参数\nwhile [ $# -gt 0 ]; do case \"$1\" in -*) shift;; esac; done\n"

// bscRunGate —— 进程内取 rc/stdout/stderr。
func bscRunGate(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := buildSelfcleanGateMain(&out, &errb, args)
	return rc, out.String(), errb.String()
}

// bscWriteFile —— 夹具写入（仓外 t.TempDir）。
func bscWriteFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := dir + "/" + name
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
	return p
}

func TestBuildSelfcleanCheck(t *testing.T) {
	d := t.TempDir()
	green := bscWriteFile(t, d, "green.sh", bscTestGreen)

	type cs struct {
		label   string
		file    string
		wantRC  int
		wantMsg string
	}
	noK1 := strings.ReplaceAll(bscTestGreen, "ZERG_REPO ", "")
	noK2 := strings.ReplaceAll(bscTestGreen, "export TMPDIR=\"$ZERG_BUILD_TMP\"\n", "")
	k3 := strings.ReplaceAll(strings.ReplaceAll(bscTestGreen, "unset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR\n", ""),
		"while [ $# -gt 0 ]; do case \"$1\" in -*) shift;; esac; done\n",
		"while [ $# -gt 0 ]; do case \"$1\" in -*) shift;; esac; done\nunset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR\n")
	noK4 := strings.ReplaceAll(bscTestGreen, " echo \"自净化失败：仓外 TMPDIR 建不起来 ⇒ 拒绝重编\" >&2; exit 1; ", " : ")

	cases := []cs{
		{"正控(四格全过)", green, 0, "⇒ 全绿（K1/K2/K3/K4 四格全过）"},
		{"负控K1(漏一枚旋钮)", bscWriteFile(t, d, "k1.sh", noK1), 1, "K1 未在一条 `unset` 上同时清 ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR"},
		{"负控K2(无 export TMPDIR)", bscWriteFile(t, d, "k2.sh", noK2), 1, "K2 无 `export TMPDIR=`（TMPDIR 未做仓外自建）"},
		{"负控K3(自净化挪到解析之后)", bscWriteFile(t, d, "k3.sh", k3), 1, "K3 自净化("},
		{"负控K4(无 fail-closed)", bscWriteFile(t, d, "k4.sh", noK4), 1, "K4 无「仓外 TMPDIR 建不起来 ⇒ 拒编」的 fail-closed 话术锚"},
		{"缺件(不给结论)", d + "/没有这个.sh", 2, "⇒ 不给结论（缺件/空/读不了）"},
		{"空件(空转不给结论)", bscWriteFile(t, d, "empty.sh", ""), 2, "脚本为空："},
	}
	for _, c := range cases {
		rc, out, errs := bscRunGate("--file", c.file)
		if rc != c.wantRC {
			t.Errorf("%s：rc=%d 期望 %d\nout=%s\nerr=%s", c.label, rc, c.wantRC, out, errs)
			continue
		}
		if !strings.Contains(out, c.wantMsg) {
			t.Errorf("%s：stdout 未含 %q\nout=%s", c.label, c.wantMsg, out)
		}
		if errs != "" {
			t.Errorf("%s：stderr 非空：%s", c.label, errs)
		}
	}

	// 空集显式在场：正控 --json ⇒ `"reds": []`（✗ 不是 null、✗ 不是缺键）
	rc, out, _ := bscRunGate("--json", "--file", green)
	if rc != 0 || !strings.Contains(out, "\"reds\": []") || !strings.Contains(out, "\"rc\": 0") {
		t.Errorf("正控 --json 空集面不符：rc=%d\nout=%s", rc, out)
	}
	// 非空集面：K1 负控 --json ⇒ `"reds": [`
	rc, out, _ = bscRunGate("--json", "--file", bscWriteFile(t, d, "k1b.sh", noK1))
	if rc != 1 || !strings.Contains(out, "\"reds\": [") || !strings.Contains(out, "\"rc\": 1") {
		t.Errorf("负控 --json 非空集面不符：rc=%d\nout=%s", rc, out)
	}
	// 坏件（读不了）⇒ rc=2
	bad := bscWriteFile(t, d, "bad.sh", bscTestGreen)
	if err := os.Chmod(bad, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(bad, 0o644) }()
	if rc, _, _ := bscRunGate("--file", bad); rc != 2 {
		t.Errorf("坏件(不可读) ⇒ 期望 rc=2，得 %d", rc)
	}
}

func TestBuildSelfcleanArgparse(t *testing.T) {
	// -h ⇒ rc=0 ＋ 逐字用法首行
	rc, out, errs := bscRunGate("-h")
	if rc != 0 || !strings.HasPrefix(out, "usage: check-build-selfclean.py [-h] [--root ROOT] [--file FILE] [--json]\n") || errs != "" {
		t.Errorf("-h 面不符：rc=%d out=%q err=%q", rc, out, errs)
	}
	// 未知旗标 ⇒ rc=2 ＋ stderr 具名
	rc, out, errs = bscRunGate("--zzz")
	if rc != 2 || out != "" || !strings.Contains(errs, "unrecognized arguments: --zzz") {
		t.Errorf("未知旗标面不符：rc=%d out=%q err=%q", rc, out, errs)
	}
	// --root 缺参 ⇒ rc=2
	rc, _, errs = bscRunGate("--root")
	if rc != 2 || !strings.Contains(errs, "argument --root: expected one argument") {
		t.Errorf("--root 缺参面不符：rc=%d err=%q", rc, errs)
	}
	// 前缀缩写：--jso ⇒ --json（唯一前缀）
	rc, out, _ = bscRunGate("--jso", "--file", bscWriteFile(t, t.TempDir(), "g.sh", bscTestGreen))
	if rc != 0 || !strings.Contains(out, "\"schema\": \"zerg/build-selfclean/1\"") {
		t.Errorf("前缀缩写 --jso 面不符：rc=%d out=%q", rc, out)
	}
}

// TestBuildSelfcleanSelfTestNoLeak —— A 类回归牙：`--self-test` 建的临时夹具目录必须**回收**。
// ★ 牙的机制：把 TMPDIR 收进私有域（t.TempDir），跑前后数「bsc-selfclean-」前缀目录增量，须为 0。
// 把主件的 `defer os.RemoveAll(d)` 删掉 ⇒ 本用例必红（增量 1）。
func TestBuildSelfcleanSelfTestNoLeak(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	count := func() int {
		ents, err := os.ReadDir(tmp)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "bsc-selfclean-") {
				n++
			}
		}
		return n
	}
	before := count()
	rc, out, errs := bscRunGate("--self-test")
	after := count()
	if rc != 0 {
		t.Fatalf("--self-test ⇒ 期望 rc=0，得 %d（out=%s err=%s）", rc, out, errs)
	}
	if !strings.Contains(out, "（5 用例 · 0 败）") {
		t.Errorf("--self-test 结论行不符：%s", out)
	}
	if after != before {
		t.Errorf("临时夹具未回收：before=%d after=%d（同前缀目录增量须为 0）", before, after)
	}
}

// TestBuildSelfcleanRetireFallback —— 退役态回落的成对负控（同族先例 check_exit_codes_test.go 的
// TestCheckExitCodesRetireFallback）：
//
//	① 默认靶件缺席 ∧ Go 半在盘 ⇒ 退役态：四格判在**合成副本**上（rc=0 · ✗ 不判缺件）；
//	② Go 半在盘但**自净化语义缺** ⇒ 合成副本照红（rc=1 · ✗ 不静默通过 · 牙）；
//	③ 两半皆缺（真消失）⇒ fail-closed（rc=2 · 点「脚本读不到」）；
//	④ 显式 `--file` 缺席 ⇒ 不受回落腿影响（照旧 rc=2）。
func TestBuildSelfcleanRetireFallback(t *testing.T) {
	writeFixture := func(t *testing.T, rel, body string, root string) {
		t.Helper()
		dir := root
		for _, seg := range strings.Split(rel, "/")[:strings.Count(rel, "/")] {
			dir += "/" + seg
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(root+"/"+rel, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	presentGoHalf := func(t *testing.T, d string) {
		t.Helper()
		writeFixture(t, "scripts/build/cmd/zerg-build-all/main.go",
			"package main\n\nfunc main() {}\n", d)
		writeFixture(t, "scripts/build/internal/zgb/zgb.go",
			"package zgb\n\n"+
				"func SelfClean() {\n"+
				"\tenv := EnvDrop(os.Environ(), \"ZERG_STATE_DIR\", \"ZERG_REPO\", \"ZERG_PROPOSAL_DIR\")\n"+
				"\tenv = EnvWith(env, \"TMPDIR\", base)\n"+
				"\t_ = env\n"+
				"}\n\n"+
				"const msg = \"══ 自净化失败：仓外 TMPDIR 建不起来 ⇒ 拒绝在未净化的环境里重编\"\n", d)
	}

	t.Run("退役态_壳件缺席Go半在盘_合成副本判四格_rc0", func(t *testing.T) {
		d := t.TempDir()
		presentGoHalf(t, d)
		rc, out, errs := bscRunGate("--root", d)
		if rc != 0 {
			t.Fatalf("退役态 rc=%d 应 0（合成副本判四格）\nout=%s\nerr=%s", rc, out, errs)
		}
		if !strings.Contains(out, "⇒ 全绿（K1/K2/K3/K4 四格全过）") {
			t.Errorf("退役态须在合成副本上判四格（全绿）：%s", out)
		}
		if strings.Contains(out, "脚本读不到") {
			t.Errorf("退役态 ✗ 不许判缺件：%s", out)
		}
	})

	t.Run("Go半缺语义_合成副本照红_rc1", func(t *testing.T) {
		d := t.TempDir()
		// Go 半在盘，但**自净化语义缺**（无三旋钮一处清 · 无 TMPDIR · 无 fail-closed 话术）。
		writeFixture(t, "scripts/build/cmd/zerg-build-all/main.go", "package main\n\nfunc main() {}\n", d)
		rc, out, _ := bscRunGate("--root", d)
		if rc != 1 {
			t.Fatalf("Go 半缺语义 ⇒ 合成副本须照红（✗ 不静默通过），得 rc=%d\nout=%s", rc, out)
		}
	})

	t.Run("两半皆缺_真消失_fail_closed_rc2", func(t *testing.T) {
		d := t.TempDir()
		rc, out, _ := bscRunGate("--root", d)
		if rc != 2 || !strings.Contains(out, "脚本读不到：") {
			t.Fatalf("两半皆缺 ⇒ 期望 rc=2 点缺件，得 rc=%d\nout=%s", rc, out)
		}
	})

	t.Run("显式file缺席_不受回落腿影响_rc2", func(t *testing.T) {
		d := t.TempDir()
		presentGoHalf(t, d)
		rc, out, _ := bscRunGate("--root", d, "--file", d+"/没有这个.sh")
		if rc != 2 || !strings.Contains(out, "脚本读不到：") {
			t.Fatalf("显式 --file 缺席 ⇒ 期望 rc=2，得 rc=%d\nout=%s", rc, out)
		}
	})
}
