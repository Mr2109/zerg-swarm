// cli_model_add_cmd_test.go —— `zerg model add --cmd`（缺口 GAP-20260925-26）的判据机检。
//
// 正控：`--cmd '<JSON 数组>'` 可判/可读 —— 干跑 rc=0 且计划件的**新增行**逐字含 `cmd: [...]`；
// 真写 rc=0 且名册件里那一行逐字含 cmd（计划件与人面**同形**）。
// 负控①（非法/缺格 ⇒ 按既有档拒并点名）：`--cmd` 非 JSON / 空数组 / 非串数组 ⇒ rc=2 且点名 `--cmd`；
// `--arch example-moe-36b`（主线不支持）**缺 `--cmd`** ⇒ rc=2 且点名（V016 校验前移到写入前）—— 且文件逐字节没动。
// 负控②（取不到 ⇒ 不给结论 · 非 0）：名册件读不到 ⇒ rc=8（✗ 不许静默当空）。
//
// 夹具纪律：全在 `t.TempDir()` 里合成（`fleetFixtureAt` / `fileSHA` / `writeFixture` 与该包既有件同源），绝不碰真 `gateway/fleet.yaml`。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modelAddCmdJSON —— 一条子端自定义启动命令（专用 fork 包装脚本形态）的 JSON 数组字面量。
const modelAddCmdJSON = `["/opt/run-fork.sh","-m","{file}","-c","16384"]`

// modelAddCmdLine —— 计划件/名册件里那一行该**逐字**含的 cmd（yamlStr 逐元素加引号）。
const modelAddCmdLine = `cmd: ["/opt/run-fork.sh", "-m", "{file}", "-c", "16384"]`

// TestModelAddCmdPositive —— 正控：`--cmd` 可判/可读（干跑零字节改动 · 真写落进名册件 · 逐字同形）。
func TestModelAddCmdPositive(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, out, errb := runCapture("model", "add", "--path", path, "--model", "probe-cmd",
		"--host", "node-a", "--file", "/models/cmd.gguf", "--arch", "example-moe-36b",
		"--cmd", modelAddCmdJSON, "--dry-run")
	if rc != 0 {
		t.Fatalf("干跑 rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, modelAddCmdLine) {
		t.Fatalf("计划件的新增行里没有逐字 cmd（人面同形）：%s", out)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("干跑改了文件：%s → %s", before, after)
	}
	rc2, _, errb2 := runCapture("model", "add", "--path", path, "--model", "probe-cmd",
		"--host", "node-a", "--file", "/models/cmd.gguf", "--arch", "example-moe-36b",
		"--cmd", modelAddCmdJSON, "--confirm", "probe-cmd", "--yes")
	if rc2 != 0 {
		t.Fatalf("真写 rc=%d（要 0）· stderr=%s", rc2, errb2)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回不过：%v", err)
	}
	if !strings.Contains(string(got), modelAddCmdLine) {
		t.Fatalf("名册件里没写进逐字 cmd：%s", got)
	}
}

// TestModelAddCmdV016RequiresCmd —— 缺格：主线不支持的架构**缺 `--cmd`** ⇒ 2 且点名（V016 前移到写入前）。
func TestModelAddCmdV016RequiresCmd(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	rc, out, errb := runCapture("model", "add", "--path", path, "--model", "probe-v016",
		"--host", "node-a", "--file", "/models/v.gguf", "--arch", "example-moe-36b", "--dry-run")
	if rc != 2 {
		t.Fatalf("example-moe-36b 缺 --cmd：rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "--cmd") {
		t.Fatalf("判词没点名缺的是 --cmd：%s", errb)
	}
	if out != "" {
		t.Fatalf("用法错时 stdout 该空，得到 %q", out)
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("被拒却改了文件")
	}
}

// TestModelAddCmdNegativeBadValue —— 非法形态：`--cmd` 非 JSON / 非串数组 / 空数组 ⇒ 2 且点名 `--cmd`。
func TestModelAddCmdNegativeBadValue(t *testing.T) {
	_, path := fleetFixtureAt(t)
	before := fileSHA(t, path)
	for _, bad := range []string{"not-json", `["a",`, `{"a":1}`, `[1,2,3]`, `[]`} {
		rc, out, errb := runCapture("model", "add", "--path", path, "--model", "probe-bad",
			"--host", "node-a", "--file", "/models/bad.gguf", "--cmd", bad, "--dry-run")
		if rc != 2 {
			t.Fatalf("`--cmd %s`：rc=%d（要 2）· stderr=%s", bad, rc, errb)
		}
		if !strings.Contains(errb, "--cmd") {
			t.Fatalf("`--cmd %s` 判词没点名 --cmd：%s", bad, errb)
		}
		if out != "" {
			t.Fatalf("`--cmd %s` 用法错时 stdout 该空，得到 %q", bad, out)
		}
	}
	if after := fileSHA(t, path); after != before {
		t.Fatalf("被拒却改了文件")
	}
}

// TestModelAddCmdRosterUnreadable —— 取不到（名册件读不到）⇒ 不给结论（退 8），✗ 不许静默当空。
func TestModelAddCmdRosterUnreadable(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.yaml")
	rc, _, errb := runCapture("model", "add", "--path", p, "--model", "probe-x",
		"--host", "node-a", "--file", "/models/x.gguf", "--cmd", `["/a"]`, "--dry-run")
	if rc != 8 {
		t.Fatalf("名册件读不到 ⇒ 要 8（不给结论），得 rc=%d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "8") {
		t.Fatalf("判词没点明不给结论（8）：%s", errb)
	}
}
