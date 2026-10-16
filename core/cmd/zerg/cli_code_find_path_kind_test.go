// cli_code_find_path_kind_test.go —— 缺口 `GAP-20260927-307`（第一半）的真二进制判据：
// `zerg code find --json path_kind` 的「扫的是目录还是单件」直读面。
//
// 病（改前现读）：`code find --path <单件>` 在 `--json` 面**分不出 dir/file** ——
// 合法字段集里没有 `path_kind`（给 `--json path_kind` 报 `未知字段 "path_kind"` · rc=2 ·
// detail=`json_field_unknown:path_kind`），机读脚本无从知道这一跑扫的是一件还是整棵子树。
//
// 治法（**只增 · 向后兼容**）：`--path` 指的**那一件**的形态（`dir` / `file`）落进**可选字段层**
// （`optFields`，与 `count` 同一体例：登在出口白名单、点得出来，**不进**文档/帮助导出面的冻结表
// ⇒ 无需重生成命令页）；值从**同一真源**现算（`--path` 分支那一处 `os.Stat(base)` 的 `st.IsDir()` ·
// ✗ 不另造第二套判定）。
//
// ★ 本件从源码现编（`zergBinaryFromSource`）—— 本单硬约束禁重编 `bin/zerg`。
//
// 成对五档：
//
//	① 正控：`--path <目录>` ⇒ `path_kind=dir`；
//	② 正控：`--path <单件>` ⇒ `path_kind=file`（缺口本体）；
//	③ 正控：不给 `--path` ⇒ 扫仓根（**目录**）⇒ `path_kind=dir`；
//	④ 兼容：只点旧字段（`--json path,line,text`）**不出** `path_kind`（旧读者逐字节不变）；
//	⑤ 负控：`--json path_kind,zzz_bogus` 混一坏字段仍按既有档拒（rc=2 · detail `json_field_unknown:zzz_bogus`）。
package main_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

type codeFindPathKindEnvelope struct {
	Items []struct {
		PathKind string `json:"path_kind"`
	} `json:"items"`
}

// codeFindPathKindOf —— 取包封里 `path_kind` 的跑级真值（逐条同值 · 取首条）。
func codeFindPathKindOf(t *testing.T, out string) string {
	t.Helper()
	var env codeFindPathKindEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("不是合法包封：%v · %q", err, out)
	}
	if len(env.Items) == 0 {
		t.Fatalf("夹具应至少 1 条命中（否则判不了 red/green）：%q", out)
	}
	for i, it := range env.Items {
		if it.PathKind != env.Items[0].PathKind {
			t.Errorf("path_kind 是**跑级真值** ⇒ 逐条同值：第 %d 条 %q ≠ 首条 %q", i, it.PathKind, env.Items[0].PathKind)
		}
	}
	return env.Items[0].PathKind
}

func TestCodeFind_JSONPathKind(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := syntheticRepo(t, "exit 0\n")
	mustWrite(t, filepath.Join(root, "aaa", "one.go"), "package aaa\n// needle here\nneedle()\n")

	// ① 正控：`--path <目录>` ⇒ dir
	rc, out, errb := execCase(t, bin, root, "code", "find", "needle", "--path", "aaa", "--json", "path_kind")
	if rc != 0 {
		t.Fatalf("`--path aaa --json path_kind` 要 rc=0（改前：未知字段 ⇒ rc=2）；得到 %d · stderr=%s", rc, errb)
	}
	if got := codeFindPathKindOf(t, out); got != "dir" {
		t.Errorf("`--path <目录>` ⇒ path_kind=dir，得到 %q", got)
	}

	// ② 正控（缺口本体）：`--path <单件>` ⇒ file
	rc, out, errb = execCase(t, bin, root, "code", "find", "needle", "--path", "aaa/one.go", "--json", "path_kind")
	if rc != 0 {
		t.Fatalf("`--path aaa/one.go --json path_kind` 要 rc=0；得到 %d · stderr=%s", rc, errb)
	}
	if got := codeFindPathKindOf(t, out); got != "file" {
		t.Errorf("`--path <单件>` ⇒ path_kind=file，得到 %q", got)
	}

	// ③ 正控：不给 `--path` ⇒ 扫仓根（目录）⇒ dir
	rc, out, errb = execCase(t, bin, root, "code", "find", "needle", "--json", "path_kind")
	if rc != 0 {
		t.Fatalf("不给 `--path` 的 `--json path_kind` 要 rc=0；得到 %d · stderr=%s", rc, errb)
	}
	if got := codeFindPathKindOf(t, out); got != "dir" {
		t.Errorf("不给 `--path`（扫仓根）⇒ path_kind=dir，得到 %q", got)
	}

	// ④ 兼容：只点旧字段 ⇒ **不出** path_kind（旧读者逐字节不变）
	rc, out, errb = execCase(t, bin, root, "code", "find", "needle", "--json", "path,line,text")
	if rc != 0 {
		t.Fatalf("旧字段面 `--json path,line,text` 要 rc=0；得到 %d · stderr=%s", rc, errb)
	}
	if strings.Contains(out, "path_kind") {
		t.Errorf("只点旧字段时不许带出 path_kind（向后兼容）：%q", out)
	}

	// ⑤ 负控：混一坏字段仍按既有档拒（rc=2）· ★ 缺口 `GAP-20260929-41`（本批）：用法错**不吐包封**
	//   ⇒ stdout **零字节**，坏字段名与合法字段表全在 stderr（旧断言「包封里 error.detail =
	//   json_field_unknown:zzz_bogus」已过期；本断言在旧行为下会红 ⇒ 有牙）。
	rc, out, errB2 := execCase(t, bin, root, "code", "find", "needle", "--json", "path_kind,zzz_bogus")
	if rc != 2 {
		t.Errorf("混一坏字段要 rc=2（既有档），得到 %d", rc)
	}
	if out != "" {
		t.Errorf("混一坏字段（用法错）⇒ stdout 必须零字节（GAP-20260929-41），得到 %q", out)
	}
	if !strings.Contains(errB2, `未知字段 "zzz_bogus"`) || !strings.Contains(errB2, "合法字段:") {
		t.Errorf("坏字段要在 stderr 点名 `zzz_bogus` 并列合法字段：%q", errB2)
	}
}
