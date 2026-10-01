// check_compat_manifest_test.go —— `compatManifestGateMain` 的定向用例（**成对负控** ⇒ 断言必须能失败）。
//
// 纪律：
//
//	· 只用 `t.TempDir()` 造夹具（仓内件一个字节不写 · 不碰 `~/.zerg`）；
//	· 每个用例都断言 **rc + 关键词**：绿档断言「必须绿」，红档断言「必须红到**那一条规则**」
//	  —— 「永远报红」与「永远不报绿」都过不了本用例（正控 + 反控成对）。
//	· py ⟷ Go 的**逐字节**对拍不在这里（那是仓外对拍台的事）；本件判「判据有没有牙」。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmTestFixture —— 一棵**能全绿**的最小清单仓（六条规则都有落点）。
func cmTestFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "core", "internal", "compat")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	api := filepath.Join(root, "core", "internal", "api")
	if err := os.MkdirAll(api, 0o755); err != nil {
		t.Fatal(err)
	}
	// R2 迁移入口面：migrate_func=AddSchemaField ⇒ 必须有 func migrateAddSchemaField(
	migrations := "package compat\n\nfunc migrateAddSchemaField() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "migrations.go"), []byte(migrations), 0o644); err != nil {
		t.Fatal(err)
	}
	// R3 锚点面 + R6 目录声明面（env / resolver 都要能在 core/**/*.go 里找到）
	anchor := "package api\n\n" +
		"func engineStatePath() string { return \"\" }\n\n" +
		"func Dir(d string) string { return d }\n\n" +
		"func ReceiptsDir() string { return \"\" }\n\n" +
		"var _ = \"ZERG_STATE_DIR\"\n" +
		"var _ = \"ZERG_RECEIPTS_DIR\"\n"
	if err := os.WriteFile(filepath.Join(api, "internal_engine.go"), []byte(anchor), 0o644); err != nil {
		t.Fatal(err)
	}
	cmWriteManifest(t, root, cmBaseManifest())
	return root
}

// cmBaseManifest —— 全绿清单（JSON 原文 · 保 `dirs` 文档序）。
func cmBaseManifest() string {
	return `{
  "schema": 1,
  "dirs": {
    "state": {"resolver": "statepath.Dir", "env": "ZERG_STATE_DIR", "default": "~/.zerg/state"},
    "receipts": {"resolver": "compat.ReceiptsDir", "env": "ZERG_RECEIPTS_DIR", "default": "~/.zerg/update_receipts"}
  },
  "entries": [
    {
      "name": "internal_engine",
      "file": "internal_engine.json",
      "dir": "state",
      "owner": "go",
      "kind": "file",
      "envelope": "inband",
      "schema_field": "schema",
      "current_schema": 1,
      "min_readable": 0,
      "migrate_func": "AddSchemaField",
      "stamps_schema": "layer",
      "anchor_file": "core/internal/api/internal_engine.go",
      "anchor": "func engineStatePath()"
    }
  ],
  "excluded": [
    {"file": "x.jsonl", "reason": "夹具：不登记"}
  ]
}
`
}

func cmWriteManifest(t *testing.T, root, body string) {
	t.Helper()
	p := filepath.Join(root, "core", "internal", "compat", "compat.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cmMutate —— 在基准清单上改一处（走 JSON 往返 · 夹具落 `t.TempDir()` 内）。
func cmMutate(t *testing.T, root string, fn func(doc map[string]interface{})) {
	t.Helper()
	p := filepath.Join(root, "core", "internal", "compat", "compat.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	fn(doc)
	out, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func cmRun(t *testing.T, argv []string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := compatManifestGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func cmEntry0(doc map[string]interface{}) map[string]interface{} {
	es, _ := doc["entries"].([]interface{})
	if len(es) == 0 {
		return map[string]interface{}{}
	}
	m, _ := es[0].(map[string]interface{})
	return m
}

// TestCheckCompatManifestGatePort —— 正控 1 + 负控 7 + 空转 + 取不到输入 + 用法错 + 真仓自检（成对）。
func TestCheckCompatManifestGatePort(t *testing.T) {
	// ① 正控：全绿夹具 ⇒ rc=0（守卫不能永远报红）
	green := cmTestFixture(t)
	rc, out, errs := cmRun(t, []string{green})
	if rc != 0 || !strings.Contains(out, "✅ 六条规则全过") || errs != "" {
		t.Fatalf("① 正控失败：得 rc=%d stderr=%q out=%q", rc, errs, out)
	}
	if !strings.Contains(out, "internal_engine") || !strings.Contains(out, "schema=1 min=0 inband/layer") {
		t.Fatalf("① 正控：绿档条目表没印全：%q", out)
	}

	// ② 负控 R1：current_schema < min_readable ⇒ rc=1 且**那一条**点名
	r1 := cmTestFixture(t)
	cmMutate(t, r1, func(doc map[string]interface{}) { cmEntry0(doc)["min_readable"] = float64(9) })
	rc, out, _ = cmRun(t, []string{r1})
	if rc != 1 || !strings.Contains(out, cmR1) || !strings.Contains(out, "current_schema=1 < min_readable=9") {
		t.Fatalf("② 负控-R1 失败：得 rc=%d out=%q", rc, out)
	}

	// ③ 负控 R2：migrate_func 指向不存在的函数 ⇒ rc=1
	r2 := cmTestFixture(t)
	cmMutate(t, r2, func(doc map[string]interface{}) { cmEntry0(doc)["migrate_func"] = "NoSuchMigrationXyz" })
	rc, out, _ = cmRun(t, []string{r2})
	if rc != 1 || !strings.Contains(out, cmR2) || !strings.Contains(out, "找不到 func migrateNoSuchMigrationXyz") {
		t.Fatalf("③ 负控-R2 失败：得 rc=%d out=%q", rc, out)
	}

	// ④ 负控 R3：anchor 指向不存在的符号 ⇒ rc=1
	r3 := cmTestFixture(t)
	cmMutate(t, r3, func(doc map[string]interface{}) {
		cmEntry0(doc)["anchor"] = "func no_such_symbol_xyz()"
	})
	rc, out, _ = cmRun(t, []string{r3})
	if rc != 1 || !strings.Contains(out, cmR3) || !strings.Contains(out, "可能没有真正的读写实现") {
		t.Fatalf("④ 负控-R3 失败：得 rc=%d out=%q", rc, out)
	}

	// ⑤ 负控 R4：源码里出现未登记的状态件 ⇒ rc=1（且逐条点名到件）
	r4 := cmTestFixture(t)
	probeDir := filepath.Join(r4, "core", "internal", "zzprobe")
	if err := os.MkdirAll(probeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(probeDir, "probe.go"), []byte(cmProbeGoFlat), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ = cmRun(t, []string{r4})
	if rc != 1 || !strings.Contains(out, cmR4) || !strings.Contains(out, "'brand_new_state.json'") ||
		!strings.Contains(out, "core/internal/zzprobe/probe.go") {
		t.Fatalf("⑤ 负控-R4 失败：得 rc=%d out=%q", rc, out)
	}

	// ⑥ 负控 R5：excluded 缺理由 ⇒ rc=1
	r5 := cmTestFixture(t)
	cmMutate(t, r5, func(doc map[string]interface{}) {
		xs, _ := doc["excluded"].([]interface{})
		if len(xs) > 0 {
			if m, ok := xs[0].(map[string]interface{}); ok {
				m["reason"] = "   "
			}
		}
	})
	rc, out, _ = cmRun(t, []string{r5})
	if rc != 1 || !strings.Contains(out, cmR5) || !strings.Contains(out, "不许无理由排除") {
		t.Fatalf("⑥ 负控-R5 失败：得 rc=%d out=%q", rc, out)
	}

	// ⑦ 负控 R6：目录声明的环境变量与代码脱节 ⇒ rc=1
	//    ⚠ 自指反制：本件**在 R6 的扫描面内**（`core/**/*.go`）⇒ 这一格用的魔法串**不许**以**连续字面**
	//    落在本件里（否则 R6 的 `env not in joined` 会被本件自己满足 ⇒ 真仓 `--self-test` 的那一格
	//    抓不住反例 —— 本单第一发踩过：`--self-test` 由 rc=0 变 rc=1）。故此处拼串落地。
	bogusEnv := "ZERG_NO_SUCH_ENV_" + "XYZ"
	r6 := cmTestFixture(t)
	cmMutate(t, r6, func(doc map[string]interface{}) {
		if dirs, ok := doc["dirs"].(map[string]interface{}); ok {
			if d, ok := dirs["state"].(map[string]interface{}); ok {
				d["env"] = bogusEnv
			}
		}
	})
	rc, out, _ = cmRun(t, []string{r6})
	if rc != 1 || !strings.Contains(out, cmR6) || !strings.Contains(out, bogusEnv) {
		t.Fatalf("⑦ 负控-R6 失败：得 rc=%d out=%q", rc, out)
	}

	// ⑧ 空转 ≠ 绿：清单是空壳（schema=0 · 无 entries/dirs）⇒ rc=1，**不许**当 0
	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, "core", "internal", "compat"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmWriteManifest(t, empty, `{"schema": 0, "entries": [], "excluded": [], "dirs": {}}`)
	rc, out, _ = cmRun(t, []string{empty})
	if rc != 1 || !strings.Contains(out, "门禁未通过") || !strings.Contains(out, "清单缺 dirs 段") {
		t.Fatalf("⑧ 空转失败：得 rc=%d out=%q", rc, out)
	}

	// ⑨ 取不到输入：根定位不到 ⇒ rc=1 + stderr 明说（不许当绿）
	rc, out, errs = cmRun(t, []string{filepath.Join(os.TempDir(), "zerg-cm-no-such-root-xyz")})
	if rc != 1 || out != "" || !strings.Contains(errs, "找不到仓库根（缺 core/internal/compat/compat.json）") {
		t.Fatalf("⑨ 取不到输入失败：得 rc=%d out=%q stderr=%q", rc, out, errs)
	}

	// ⑩ 未知旗标 ⇒ rc=2（用法错 · 不给结论）
	rc, out, errs = cmRun(t, []string{"--bogus"})
	if rc != 2 || out != "" || !strings.Contains(errs, "unrecognized arguments: --bogus") {
		t.Fatalf("⑩ 未知旗标失败：得 rc=%d out=%q stderr=%q", rc, out, errs)
	}

	// ⑪ `--json` 拒收（本门无机器读面）⇒ rc=2
	rc, out, errs = cmRun(t, []string{"--json"})
	if rc != 2 || out != "" || !strings.Contains(errs, "本门无机器读面") {
		t.Fatalf("⑪ --json 拒收失败：得 rc=%d out=%q stderr=%q", rc, out, errs)
	}

	// ⑫ 真仓自检：cwd = 包目录（core/cmd/zerg）⇒ 上溯到仓根，`--self-test` 必须 rc=0
	rc, out, errs = cmRun(t, []string{"--self-test"})
	if rc != 0 || !strings.Contains(out, "门禁自测通过") || !strings.Contains(out, "R4 嵌套落点必须登记全路径") {
		t.Fatalf("⑫ --self-test 失败：得 rc=%d stderr=%q out=%q", rc, errs, out)
	}
}
