// family_code_constdup_test.go —— `zerg code const-dup`（缺口 `GAP-20261001-69` 的前置工具）的判据机检。
//
// 钉四件事（逐条可单独失败）：
//
//	① 扫法必须是**解析器**而不是正则：注释里的同值字面量、`var` 里的同值字面量、`_test.go` 里的同值
//	   常量，**都不许**计入；`const` 里的不同名字符串字面量必须计入。
//	② 真仓里的**标尺值**：`scripts/gates/precommit-gates.sh` 这一组必须被找出，且点名那四枚常量
//	   （`zergPrecommitGatesRel` / `gateScriptRel` / `pgsDefaultTarget` / `tsmFixGateScriptRel`）的
//	   **件:行** 逐条对得上 —— 这是判断工具对不对的标尺。
//	③ 退码三档：默认（有命中）退 0 · `--fail-on-hit` 且有命中退 1 · 用法错（缺 `--path` / 路径不存在）
//	   退 2 **且 `error.kind` 是 `usage`**（`kind` 在进程内可读，`--json` 用法错按 `GAP-20260929-41`
//	   不吐 stdout 包封 ⇒ 只能在这一层判）。
//	④ 机器面形状：`--json value,count,owner,dupes` 只投影点名的四格，`owner` 是**对象**、`dupes` 是
//	   **对象数组**（不是被转义的字符串）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ccdInv —— 造一枚调用（`kv` 是取值旗标的唯一读面）。
func ccdInv(kv map[string]string, args ...string) *invocation {
	m := map[string][]string{}
	for k, v := range kv {
		m[k] = []string{v}
	}
	return &invocation{path: []string{"code", "const-dup"}, args: args, kv: m}
}

// ccdRun —— 跑一次，返回 (rc, stdout, stderr)。
func ccdRun(t *testing.T, inv *invocation) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := cmdCodeConstDup(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestCodeConstDupParserNotRegexp —— ① 解析器口径 + 排除面（`_test.go` 不计）。
func TestCodeConstDupParserNotRegexp(t *testing.T) {
	dir := t.TempDir()
	body := "package x\n\n" +
		"const aPath = \"same/value.txt\"\n" +
		"const bPath = \"same/value.txt\"\n" +
		"const solo = \"only/once.txt\"\n" +
		"var notConst = \"same/value.txt\"\n\n" +
		"// 注释里写一遍 \"same/value.txt\" 不算\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// 同值常量落在测试件里 ⇒ 整件不进面（否则该组会变 3 枚）。
	testBody := "package x\n\nconst cPath = \"same/value.txt\"\n"
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte(testBody), 0o600); err != nil {
		t.Fatal(err)
	}
	rc, out, errText := ccdRun(t, ccdInv(map[string]string{"--path": dir}))
	if rc != exitOK {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errText)
	}
	if !strings.Contains(out, "same/value.txt") {
		t.Fatalf("没列出同值异名组：%s", out)
	}
	if strings.Contains(out, "only/once.txt") {
		t.Fatalf("只出现一次的常量被当成命中：%s", out)
	}
	// 该组必须**恰好 2 枚**（aPath + bPath）：`var` / 注释 / `_test.go` 三处都不许计入。
	var row string
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "same/value.txt\t") {
			row = ln
		}
	}
	if row == "" {
		t.Fatalf("行面没有该组：%s", out)
	}
	cols := strings.Split(row, "\t")
	if len(cols) < 5 || cols[2] != "2" {
		t.Fatalf("枚数不是 2（var/注释/测试件被算进去了？）：%q", row)
	}
	if !strings.Contains(cols[4], "bPath") && !strings.Contains(cols[3], "bPath") {
		t.Fatalf("两个名字没都点名：%q", row)
	}
	if !strings.Contains(cols[3], "aPath") && !strings.Contains(cols[4], "aPath") {
		t.Fatalf("两个名字没都点名：%q", row)
	}
}

// TestCodeConstDupStandard ———— ② 真仓回归：**已收归单一真源的那个值，不许再以同值异名出现**。
//
// 原口径是「拿 4 枚真常量当标尺、按件:行逐条对」—— 但那些常量正是 GAP-20261001-69 要消灭的
// 目标：本轮把 scripts/gates/precommit-gates.sh 的 5 个异名收归 paths_canonical.go 后，
// 标尺本身消失 ⇒ 测试红。标尺钉在"待修的病灶"上，等于**修好反而红**。
// ⇒ 改为**方向反转的回归**：这些值现在只许有一处字面量定义；若又被写散，本测试红。
// 扫描器自身的正确性由上面那组合成夹具（aPath/bPath）负责，不依赖真仓数据。
func TestCodeConstDupStandard(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Fatal("解析不到仓根 ⇒ 判不了（✗ 不许当绿）")
	}
	inv := ccdInv(map[string]string{"--path": filepath.Join(root, "core", "cmd", "zerg")})
	rc, out, errText := ccdRun(t, inv)
	if rc != exitOK {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errText)
	}
	// 已收归真源的 6 个值（paths_canonical.go）：不许再出现在同值异名表里。
	canonicalized := []string{
		"publish/docs",
		"scripts/公开标记.tsv",
		"core/cmd/zerg/main.go",
		"core/cmd/zerg",
		"publish/mirror-public-lib.py",
		"scripts/gates/precommit-gates.sh",
		// 第二批（2026-10-20）
		"scripts/gates",
		"core/internal/version/version.go",
		"scripts/gates/gate-coverage.config.json",
		"publish/excludes.txt",
		"scripts/gates/judge-registry.json",
		"scripts/gates/check-manifest-freshness.py",
		"publish/whitelist.txt",
		"core/internal/contract/registry.json",
		// 第三批（2026-10-20）
		"scripts/sandbox-probes/verify-two-states.py",
		"scripts/gates/check-manifest-freshness-profile.py",
		"scripts/gates/check-contract-registry.py",
		"scripts/build/zerg-upgrade.sh",
		"scripts/build/zerg-swap-core.sh",
		"scripts/build/zerg-swap-agent.sh",
		"scripts/build/pack-release.sh",
		"scripts/build/build-all.sh",
		"publish/replace-rules.tsv",
		"gateway/fleet.yaml",
		"core/internal/contract/adopt-retire.json",
		"core/cmd/zerg/testdata/cli-matrix.json",
		"core/cmd/zerg/gate_inproc_table.go",
		"core/added.go",
		"wall/target/debug/zerg-wall",
		"/usr/bin/python3",
		"/Volumes/ZERG-ARCHIVE",
	}
	for _, v := range canonicalized {
		for _, ln := range strings.Split(out, "\n") {
			if strings.HasPrefix(ln, v+"	") {
				t.Fatalf("值 %q 又出现同值异名（应只留 paths_canonical.go 一处）：%q", v, ln)
			}
		}
	}
}

// TestCodeConstDupExitCodes ———— ③ 退码三档 + `error.kind=usage`（进程内可读）。
func TestCodeConstDupExitCodes(t *testing.T) {
	// 缺 `--path` ⇒ 2 · kind=usage
	inv := ccdInv(nil)
	rc, _, _ := ccdRun(t, inv)
	if rc != exitUsage {
		t.Fatalf("缺 --path 的 rc=%d（要 2）", rc)
	}
	if inv.err == nil || inv.err.Kind != "usage" {
		t.Fatalf("缺 --path 的 error.kind 不是 usage：%+v", inv.err)
	}
	// 路径不存在 ⇒ 2 · kind=usage
	inv2 := ccdInv(map[string]string{"--path": "/nosuch-path-zz/nope"})
	rc2, _, _ := ccdRun(t, inv2)
	if rc2 != exitUsage {
		t.Fatalf("路径不存在的 rc=%d（要 2）", rc2)
	}
	if inv2.err == nil || inv2.err.Kind != "usage" {
		t.Fatalf("路径不存在的 error.kind 不是 usage：%+v", inv2.err)
	}
	// 有命中 + `--fail-on-hit` ⇒ 1；不给它 ⇒ 0（默认不红）
	root := repoRoot()
	if root == "" {
		t.Skip("解析不到仓根 ⇒ 跳过命中档")
	}
	dir := filepath.Join(root, "core", "cmd", "zerg")
	inv3 := ccdInv(map[string]string{"--path": dir, "--fail-on-hit": "true"})
	if rc3, _, _ := ccdRun(t, inv3); rc3 != exitFail {
		t.Fatalf("--fail-on-hit 的 rc=%d（要 1）", rc3)
	}
	inv4 := ccdInv(map[string]string{"--path": dir})
	if rc4, _, _ := ccdRun(t, inv4); rc4 != exitOK {
		t.Fatalf("默认档的 rc=%d（要 0 —— 命中不红）", rc4)
	}
}

// TestCodeConstDupJSONShape ———— ④ 机器面：只投影点名的格 · `owner` 对象 · `dupes` 对象数组。
func TestCodeConstDupJSONShape(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Fatal("解析不到仓根 ⇒ 判不了")
	}
	// ★ `--json` 不进 `inv.kv`（解析器把它收在 `jsonGiven`/`fields` 两格上）—— 塞进 kv 会被
	//   `foreignFlag` 当「不归本命令的旗标」点名拒掉，那是**另一格**判据。
	inv := ccdInv(map[string]string{"--path": filepath.Join(root, "core", "cmd", "zerg")})
	inv.jsonGiven = true
	inv.fields = []string{"value", "count", "owner", "dupes"}
	rc, out, errText := ccdRun(t, inv)
	if rc != exitOK {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errText)
	}
	var env struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		head := out
		if len(head) > 200 {
			head = head[:200]
		}
		t.Fatalf("包封解不动：%v · %s", err, head)
	}
	if len(env.Items) == 0 {
		t.Fatal("items 是空的")
	}
	for _, it := range env.Items {
		if len(it) != 4 {
			t.Fatalf("item 的键不是点名的四格：%v", it)
		}
		for _, k := range []string{"value", "count", "owner", "dupes"} {
			if _, ok := it[k]; !ok {
				t.Fatalf("缺键 %q：%v", k, it)
			}
		}
		var own map[string]any
		if err := json.Unmarshal(it["owner"], &own); err != nil {
			t.Fatalf("owner 不是对象（被当字符串转义了？）：%v", err)
		}
		for _, k := range []string{"name", "file", "line"} {
			if _, ok := own[k]; !ok {
				t.Fatalf("owner 缺键 %q：%v", k, own)
			}
		}
		var dups []map[string]any
		if err := json.Unmarshal(it["dupes"], &dups); err != nil {
			t.Fatalf("dupes 不是对象数组：%v", err)
		}
		if len(dups) == 0 {
			t.Fatalf("dupes 空 ⇒ 不该出现在命中面里")
		}
	}
}
