// cli_model_add_test.go —— `model add`（`Q-099`）+ `config reload`（`Q-100`）的判据机检（波① `T1a`）。
//
// 判据（`任务清单-缺口收口-20260923.md` §自排补遗 `S1` 的两条形状 · 逐条落成断言）：
//
//	`model add`     含 `--dry-run` ✓ 先校验后写 ✓ ⇒ **干跑零字节改动**（现跑 sha256 对拍）·
//	                真写只有**一行**新增（其余逐字节不变）· 写完读回再校 · 任一步不过 ⇒ **回滚**（原文逐字节不变）
//	`config reload` 照 `nginx -s reload`：**先校验、失败回滚** ⇒ 名册件解析不过时 **一个请求都不发**
//
// 成对负控（每条判据都有「探针不红 ⇒ 判红」的那一半）：重复 file / 主机不在册 / 坏档 /
// `--json` 不给字段 / 缺 `--yes` —— 逐格断言「退码对 **且** 文件逐字节没动」。
//
// 夹具纪律：全在 `t.TempDir()` 里合成，**绝不碰真 `gateway/fleet.yaml`**（它身上有 Mr2109 的在途改动）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fleetFixture = `# 合成夹具（判据件不碰真名册）
auth:
  token: ""

models:
  Mr2109:
    - { host: Mr2109, backend: llama-server, file: "/models/old-a.gguf", mem_gb: 6, ctx_window: 8192 }
    - { host: Mr2109, backend: llama-server, file: "/models/old-b.gguf", mem_gb: 8, ctx_window: 16384, added: "2026-09-01" }
  x3:
    - { host: x3, backend: llama-server, file: "/data/models/old-c.gguf", mem_gb: 18, ctx_window: 131072 }

aliases:
  zerg-a: old-a

fleet:
  Mr2109:
    host: Mr2109
    port: 8580
`

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s 不过：%v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func fleetFixtureAt(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "fleet.yaml")
	if err := os.WriteFile(path, []byte(fleetFixture), 0o644); err != nil {
		t.Fatalf("写夹具不过：%v", err)
	}
	return dir, path
}

// 判据面 · ① 干跑零字节改动 + 计划件里有那一行。
func TestModelAddDryRunZeroSideEffect(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, out, errb := runCapture("model", "add", "--path", path, "--host", "Mr2109",
		"--model", "new-1", "--file", "/models/new-1.gguf", "--mem-gb", "6", "--ctx", "16384",
		"--arch", "qwen3", "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "零副作用") {
		t.Fatalf("干跑 stderr 里没有「零副作用」判词：%s", errb)
	}
	if !strings.Contains(out, `/models/new-1.gguf`) {
		t.Fatalf("计划件里没有那一行：%s", out)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("干跑改了文件：%s → %s", before, after)
	}
}

// 判据面 · ② 真写：只有一行新增（其余逐字节不变）+ 写完解析过 + 读回再校。
func TestModelAddRealWriteOneLineAdded(t *testing.T) {
	_, path := fleetFixtureAt(t)
	rc, out, errb := runCapture("model", "add", "--path", path, "--host", "Mr2109",
		"--model", "new-1", "--file", "/models/new-1.gguf", "--mem-gb", "6", "--ctx", "16384",
		"--arch", "qwen3", "--desc", "夹具新条", "--added", "2026-09-23", "--yes")
	if rc != 0 {
		t.Fatalf("真写 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "new-1") {
		t.Fatalf("行式面里没有模型名：%s", out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回不过：%v", err)
	}
	oldLines := strings.Split(strings.TrimRight(fleetFixture, "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(string(got), "\n"), "\n")
	if len(newLines) != len(oldLines)+1 {
		t.Fatalf("行数 %d → %d（要 +1）", len(oldLines), len(newLines))
	}
	if !strings.Contains(string(got), `/models/new-1.gguf`) {
		t.Fatalf("新条没写进去：%s", got)
	}
	// 逐字节：删掉新插入的那一行，其余必须与原档**逐字相同**（新增式判据）
	at := -1
	for i, l := range newLines {
		if strings.Contains(l, "/models/new-1.gguf") {
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("找不到新行的位置")
	}
	restored := append(append([]string{}, newLines[:at]...), newLines[at+1:]...)
	if strings.Join(restored, "\n") != strings.Join(oldLines, "\n") {
		t.Fatalf("除了新增那一行，其余**不是**逐字节相同 ⇒ 不是新增式")
	}
}

// 判据面 · ③ 缺 `--yes` ⇒ 2 且**文件逐字节没动**（fail-closed）。
func TestModelAddRequiresYes(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, out, errb := runCapture("model", "add", "--path", path, "--host", "Mr2109",
		"--model", "new-1", "--file", "/models/new-1.gguf")
	if rc != 2 {
		t.Fatalf("缺 --yes rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Fatalf("缺 --yes 时 stdout 该是空的，得到 %q", out)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("缺 --yes 却改了文件：%s → %s", before, after)
	}
}

// 反例探针一 · 重复 `file` ⇒ 2 且不动（不覆盖别人的条）。
func TestModelAddNegativeDuplicateFile(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, _, errb := runCapture("model", "add", "--path", path, "--host", "Mr2109",
		"--model", "dup", "--file", "/models/old-a.gguf", "--yes")
	if rc != 2 || !strings.Contains(errb, "不覆盖") {
		t.Fatalf("重复 file：rc=%d stderr=%s（要 2 + 「不覆盖」）", rc, errb)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("被拒了却改了文件")
	}
}

// 反例探针二 · 主机不在 `models:` 段 ⇒ 2 且不动（新开一块要人签）。
func TestModelAddNegativeHostNotDeclared(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, _, errb := runCapture("model", "add", "--path", path, "--host", "mini9",
		"--model", "m", "--file", "/models/m.gguf", "--yes")
	if rc != 2 || !strings.Contains(errb, "models:") {
		t.Fatalf("主机不在册：rc=%d stderr=%s（要 2 + 点名 models: 段）", rc, errb)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("被拒了却改了文件")
	}
}

// 反例探针三 · 现档就是坏 YAML ⇒ 1 且**不写**（先校验后写的用处）。
func TestModelAddNegativeBadYAMLStaysUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fleet.yaml")
	bad := "models:\n  Mr2109:\n    - { host: Mr2109, file: \"/x.gguf\"\n" // 少一个 `}`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	before := fileSHA(t, path)
	rc, out, _ := runCapture("model", "add", "--path", path, "--host", "Mr2109",
		"--model", "m", "--file", "/models/m.gguf", "--yes")
	if rc != 1 {
		t.Fatalf("坏档 rc=%d（要 1）", rc)
	}
	if out != "" {
		t.Fatalf("坏档时 stdout 该空，得到 %q", out)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("坏档被改了（先校验后写破功）")
	}
}

// 反例探针四 · `--json` 不给字段 ⇒ 1 且 stdout 0 字节（K2 四件套）。
func TestModelAddJSONNoFields(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, out, _ := runCapture("model", "add", "--path", path, "--host", "Mr2109",
		"--model", "m", "--file", "/models/m.gguf", "--json")
	if rc != 1 || out != "" {
		t.Fatalf("`--json` 不给字段：rc=%d stdout=%q（要 1 + 空）", rc, out)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("只读用法面却改了文件")
	}
}

// `config reload` · ① 干跑：只校验、**一个请求都不发**（连主控都不用起 ⇒ 就是这个判据的牙）。
func TestConfigReloadDryRunSendsNoRequest(t *testing.T) {
	_, path := fleetFixtureAt(t)
	rc, out, errb := runCapture("config", "reload", "--path", path, "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "POST") || !strings.Contains(errb, "零副作用") {
		t.Fatalf("干跑计划件不完整：out=%s err=%s", out, errb)
	}
	if !strings.Contains(out, "Mr2109") {
		t.Fatalf("干跑没报在册 host：%s", out)
	}
}

// `config reload` · ② 先校验、失败回滚：坏档 ⇒ 1 且不发请求。
func TestConfigReloadBadConfigRefusesBeforeRequest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fleet.yaml")
	if err := os.WriteFile(path, []byte("models: [这 不是 映射]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := runCapture("config", "reload", "--path", path, "--yes")
	if rc != 1 {
		t.Fatalf("坏档 rc=%d（要 1）", rc)
	}
	if out != "" {
		t.Fatalf("坏档 stdout 该空，得到 %q", out)
	}
	if !strings.Contains(errb, "不发请求") {
		t.Fatalf("判词里没有「不发请求」：%s", errb)
	}
}

// `config reload` · ③ 缺 `--yes` ⇒ 2（D2 档 fail-closed，且**没打主控**）。
func TestConfigReloadRequiresYes(t *testing.T) {
	_, path := fleetFixtureAt(t)
	rc, out, errb := runCapture("config", "reload", "--path", path)
	if rc != 2 || out != "" {
		t.Fatalf("缺 --yes：rc=%d stdout=%q（要 2 + 空）· stderr=%s", rc, out, errb)
	}
}
