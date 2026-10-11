// family_config_set_verified_test.go —— `model set`（`cmdModelSet`）的 verified 三态**加性**闭集自检
// （GAP-20261003-21 · 设计稿步2 后半 · `设计-verified三态与锚-v0.1-20261010.md` §五 步2）。
//
// 面：`model set --set k=v` 的**字段闭集**（`modelSetKind`）加两枚**加性**键 ——
// `verified_state`（值域闭集 true|false|reported）与 `verified_where`（串）。
// ★ 走闭集 ⇒ **不新增旗标、不碰 `main.go`**（`model add` 加旗标才要登 `main.go` 的旗标名表）；旧 bool 键
// `verified` 一字不动（零破坏：既有键语义与退码都不改）。
//
// 正控：`--set verified_state=reported --set verified_where=edge-node` ⇒ 真写落字段（仓外 `t.TempDir()` 名册）。
// 值域：`verified_state=maybe` ⇒ rc=2（值域不合）且文件**逐字节没动**；`true`/`false` 都收。
// 零破坏：既有 bool 键 `verified=true` 仍走旧路（裸 `true` · 渲染逐字不变）。
//
// ★ 负控（**真红**）：哑化两键（去掉 `modelSetKind` 的两个 case）⇒ 本文件正控 rc 从 0 变 2
// （`--set verified_state=` 不是可改字段）⇒ 用例 FAIL（做法见报告 `-overlay` 两态）。
package main_test

import (
	"os"
	"strings"
	"testing"
)

// 判据面 · ① 两枚加性键都收 · 真写落字段（其余字段逐字保留）。
func TestModelSetVerifiedStateAdditiveAccepted(t *testing.T) {
	_, path := fleetFixtureAt(t)
	rc, out, errb := runCapture("model", "set", "--path", path, "--model", "probe-bare", "--host", "node-a",
		"--set", "verified_state=reported", "--set", "verified_where=edge-node", "--confirm", "probe-bare", "--yes")
	if rc != 0 {
		t.Fatalf("真写 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回不过：%v", err)
	}
	txt := string(got)
	if !strings.Contains(txt, "verified_state: reported") {
		t.Fatalf("加性键 verified_state 没落字段：%s", txt)
	}
	if !strings.Contains(txt, `verified_where: "edge-node"`) {
		t.Fatalf("加性键 verified_where 没落字段（str 渲染应带引号）：%s", txt)
	}
	want := `  probe-bare: { host: node-a, backend: ds4-server, file: "/data/models/old-c.gguf", mem_gb: 18, ctx_window: 131072, verified_state: reported, verified_where: "edge-node" }`
	if !strings.Contains(txt, want) {
		t.Fatalf("原条其余字段没有逐字保留（保序原位改破功）——要：\n%s\n实得：\n%s", want, txt)
	}
	if !strings.Contains(out, "probe-bare") {
		t.Fatalf("行式面没报改的模型：%s", out)
	}
}

// 判据面 · ② 值域闭集：`maybe` ⇒ rc=2 且文件逐字节没动；`true`/`false` 都收（干跑）。
func TestModelSetVerifiedStateValueDomainClosed(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, _, errb := runCapture("model", "set", "--path", path, "--model", "probe-bare", "--host", "node-a",
		"--set", "verified_state=maybe", "--confirm", "probe-bare", "--yes")
	if rc != 2 {
		t.Fatalf("值域不合 rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "true / false / reported") {
		t.Fatalf("判词没点名三态闭集：%s", errb)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("值域不合却改了文件")
	}
	for _, v := range []string{"true", "false"} {
		rc2, _, errb2 := runCapture("model", "set", "--path", path, "--model", "probe-bare", "--host", "node-a",
			"--set", "verified_state="+v, "--dry-run")
		if rc2 != 0 {
			t.Fatalf("verified_state=%s 干跑 rc=%d（要 0）· stderr=%s", v, rc2, errb2)
		}
	}
}

// 判据面 · ③ 零破坏：既有 bool 键 `verified=true` 仍走旧路（渲染裸 `true`，与改前逐字同）。
func TestModelSetExistingBoolKeyUnchanged(t *testing.T) {
	_, path := fleetFixtureAt(t)
	rc, out, errb := runCapture("model", "set", "--path", path, "--model", "probe-bare", "--host", "node-a",
		"--set", "verified=true", "--dry-run")
	if rc != 0 {
		t.Fatalf("既有 bool 键 verified=true 干跑 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "verified: true }") {
		t.Fatalf("既有 bool 键渲染变了（应裸 `verified: true`）：%s", out)
	}
}
