// gateway_build_test.go —— `gateway_build.go`（`gateway/install.sh` 的 Go 真身）的**两态夹具**。
//
// 钉什么：原脚本四条腿的输出面**逐字**（✗ 不加行 · ✗ 不改文案）＋ 子进程退码原样转出。
//
//	甲 两变量都缺席 ⇒ 逐字「!! 目录不存在: 」（空串 · 原脚本 `[ -d "" ]` 亦假）· rc=0
//	乙 两目录在场（绝对路径）⇒ 逐字含子进程回显与「构建完成」两行 · rc=0
//	丙 子进程起身但退 3 ⇒ 本件 rc=3（等价 `set -e` 打断 · ✗ 归 0、✗ 归 1）
//	丁 成对负控：把甲档的期望改一个字节 ⇒ 断言必红（证本测试不是恒绿装置）
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gwbGoldenAbsent —— 甲档（两变量缺席）的原脚本逐字读数（`bash gateway/install.sh` 现跑抄下）。
const gwbGoldenAbsent = `==> [1/2] 构建模型管理器 App
    !! 目录不存在: 
==> [2/2] 构建 X3Monitor
    !! 目录不存在: 

==> 全部完成。子端部署请使用: ./deploy_x3.sh / ./deploy_mini.sh
`

// gwbRunForTest —— 跑一次 Go 真身，返回 (stdout, stderr, rc)。
func gwbRunForTest(t *testing.T) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := cmdGatewayBuild(&invocation{path: []string{"gateway", "build"}}, &out, &errb)
	return out.String(), errb.String(), rc
}

// TestGatewayBuildAbsentFolders —— 甲档 · 乙档 · 丙档 三读。
func TestGatewayBuildAbsentFolders(t *testing.T) {
	t.Setenv("MODEL_MANAGER_DIR", "")
	t.Setenv("X3_MONITOR_DIR", "")
	out, errs, rc := gwbRunForTest(t)
	if out != gwbGoldenAbsent {
		t.Fatalf("甲档 stdout 与原件不逐字：\n得 %q\n要 %q", out, gwbGoldenAbsent)
	}
	if errs != "" {
		t.Fatalf("甲档不该有 stderr，得 %q", errs)
	}
	if rc != 0 {
		t.Fatalf("甲档 rc 要 0（原件 `if` 护住 ⇒ 不打断），得 %d", rc)
	}
}

// TestGatewayBuildPresentFolders —— 乙档 · 丙档（真起子进程 · 用 stub 钉回显）。
func TestGatewayBuildPresentFolders(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	mm := filepath.Join(tmp, "mm")
	x3Dir := filepath.Join(tmp, "edge-node")
	for _, d := range []string{bin, mm, x3Dir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// swift stub：回显 argv 与 cwd（cwd 是「在原目录里跑」这条腿的证物）。
	if err := os.WriteFile(filepath.Join(bin, "swift"),
		[]byte("#!/bin/bash\necho \"(stub) swift $*\"\necho \"(stub) build dir=$(pwd)\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bs := filepath.Join(x3Dir, "build.sh")
	if err := os.WriteFile(bs, []byte("#!/bin/bash\necho \"(stub) X3Monitor build.sh cwd=$(pwd)\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MODEL_MANAGER_DIR", mm)
	t.Setenv("X3_MONITOR_DIR", x3Dir)

	want := fmt.Sprintf(`==> [1/2] 构建模型管理器 App
(stub) swift build
(stub) build dir=%[1]s
    构建完成（swift build）。可运行: open %[1]s/模型管理器.app
==> [2/2] 构建 X3Monitor
(stub) X3Monitor build.sh cwd=%[2]s
    构建完成: %[2]s/X3Monitor.app

==> 全部完成。子端部署请使用: ./deploy_x3.sh / ./deploy_mini.sh
`, mm, x3Dir)
	out, errs, rc := gwbRunForTest(t)
	if out != want {
		t.Fatalf("乙档 stdout 与原件不逐字：\n得 %q\n要 %q", out, want)
	}
	if errs != "" {
		t.Fatalf("乙档不该有 stderr，得 %q", errs)
	}
	if rc != 0 {
		t.Fatalf("乙档 rc 要 0，得 %d", rc)
	}

	// 丙档：build.sh 退 3 ⇒ 原件 `set -e` 打断、退码原样 3 ⇒ 本件必须 3（✗ 归 0 ／ ✗ 归 1）
	if err := os.WriteFile(bs, []byte("#!/bin/bash\necho \"(stub) 故意失败\"\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, rc = gwbRunForTest(t)
	if rc != 3 {
		t.Fatalf("丙档 rc 要 3（子进程退码原样转出），得 %d", rc)
	}
}

// TestGatewayBuildGoldenIsNotTautology —— 丁档（成对负控）：改一个字节的期望值必须**判红**，
// 否则上面两条就是恒绿装置（照 `check_cli_ownership` 自检的成对口径）。
func TestGatewayBuildGoldenIsNotTautology(t *testing.T) {
	t.Setenv("MODEL_MANAGER_DIR", "")
	t.Setenv("X3_MONITOR_DIR", "")
	out, _, _ := gwbRunForTest(t)
	mutated := strings.Replace(gwbGoldenAbsent, "!! 目录不存在", "!! 目录在", 1)
	if out != mutated {
		return // 期望被改 ⇒ 现读 ≠ 期望 ⇒ 判红（本测试成立）
	}
	t.Fatal("改掉一个字节的期望仍然相等 ⇒ 甲乙两档无鉴别力（恒绿装置）")
}
