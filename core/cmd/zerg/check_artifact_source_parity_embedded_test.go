// check_artifact_source_parity_embedded_test.go —— 门74 取身份**改走嵌入段**的回归牙（只读 · 合成夹具）。
//
// 背景（本单病征）：旧读法靠跑 `<制品> --version` 自报 ⇒ 「跑不起来 / 不可执行 / 不在盘」一律 rc=2；
// 交叉件（如 linux-amd64）在本机恒跑不起来 ⇒ 门74 对交叉件**永远只出 rc=2**，判定取决于架构偶然性。
// 新读法：**先读制品自身嵌入段**（`artifactEmbeddedIdentity` ⇒ `debug/buildinfo` **只读文件头** ⇒ 不执行该件）。
//
// 四条断言（任一条被哑化回旧路即整例 FAIL）：
//
//	① 同代可执行件           ⇒ rc=0（嵌入段取身份）
//	② 同代**不可执行**件      ⇒ 仍 rc=0（旧路此处 rc=2 —— 嵌入段只读文件头，不需执行位）
//	③ 同代**交叉件**(GOOS=linux) ⇒ 仍 rc=0（旧路此处 rc=2 —— exec format error）
//	④ 反误报（严格性未丢）：同代件但建构早于输入面 ⇒ 仍 rc=1 并点名（嵌入段喂的是真判据，不是永绿）
package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// aspBuildTaggedArtifact 造一件带嵌入段身份的真 Go 制品（`-tags commit=…,build_time=…`）。
// goos 空 ⇒ 本机平台；非空（如 linux）⇒ 交叉件（本机跑不起来，只能读文件头）。
func aspBuildTaggedArtifact(t *testing.T, dir, tag, commit, buildISO, goos string) string {
	t.Helper()
	sub := aspPyJoin(dir, "tagbuild-"+tag)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aspPyJoin(sub, "go.mod"), []byte("module tagbuild\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aspPyJoin(sub, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := aspPyJoin(sub, "art")
	env := append(os.Environ(), "GOFLAGS=")
	if goos != "" {
		env = append(env, "GOOS="+goos, "GOARCH=amd64")
	}
	cmd := exec.Command("go", "build", "-buildvcs=false",
		"-tags", "commit="+commit+",build_time="+buildISO, "-o", out, ".")
	cmd.Dir = sub
	cmd.Env = env
	var so bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &so
	if err := cmd.Run(); err != nil {
		t.Fatalf("夹具：go build 失败：%v\n%s", err, so.String())
	}
	return out
}

func TestAsp_EmbeddedIdentityPreferred(t *testing.T) {
	r, h := aspNewRepo(t, "embid")
	aspSetMtime(aspPyJoin(r, "core", "a.go"), "2026-09-28T09:00:00Z")
	commit := aspFirst16(h)

	// ① 同代可执行件 ⇒ rc=0
	a := aspBuildTaggedArtifact(t, r, "same", commit, "2026-09-28T10:00:00Z", "")
	rc, out, _ := aspRunEntry("--root", r, "--artifact", a)
	if rc != 0 {
		t.Fatalf("① 同代可执行件 rc=%d（期望 0 · 嵌入段取身份）\n%s", rc, out)
	}

	// ② 同代**不可执行**件（chmod 644）⇒ 仍 rc=0（旧路靠跑件 ⇒ 此处 rc=2）
	if err := os.Chmod(a, 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ = aspRunEntry("--root", r, "--artifact", a)
	if rc != 0 {
		t.Fatalf("② 不可执行件 rc=%d（期望 0 · 从嵌入段取身份；旧路靠跑件 ⇒ rc=2）\n%s", rc, out)
	}

	// ③ 同代**交叉件**（本机跑不起来）⇒ 仍 rc=0（旧路 exec format error ⇒ rc=2）
	ac := aspBuildTaggedArtifact(t, r, "cross", commit, "2026-09-28T10:00:00Z", "linux")
	rc, out, _ = aspRunEntry("--root", r, "--artifact", ac)
	if rc != 0 {
		t.Fatalf("③ 交叉件 rc=%d（期望 0 · 嵌入段取身份；旧路靠跑件 ⇒ rc=2）\n%s", rc, out)
	}

	// ④ 反误报：同代件但建构**早于**输入面 ⇒ 仍须 rc=1 并点名（嵌入段喂真判据 · 禁永绿）
	al := aspBuildTaggedArtifact(t, r, "lag", commit, "2026-09-27T00:00:00Z", "")
	rc, out, _ = aspRunEntry("--root", r, "--artifact", al)
	if rc != 1 {
		t.Fatalf("④ 建构早于输入面 rc=%d（期望 1 · 严格性未丢）\n%s", rc, out)
	}
	if aspCountContains(aspSplitLines(out), "· core/a.go（源码侧") != 1 {
		t.Fatalf("④ 未逐条点名 core/a.go：\n%s", out)
	}

	// ⑤ 自指：本件落在自门扫描面（core/**/*.go）内 ⇒ 零物理行匹配自门嵌入口径（不新增 embed 资产）。
	//    ★ 本件**落件后**才在盘；overlay 态（未落盘）该格无从谈起 ⇒ 明示跳过（不静默）。
	if data, err := os.ReadFile("check_artifact_source_parity_embedded_test.go"); err == nil {
		hits := 0
		for _, line := range aspIterLines(string(data)) {
			if aspEmbedRE.MatchString(line) {
				hits++
			}
		}
		if hits != 0 {
			t.Fatalf("本件有 %d 行匹配自门嵌入口径 ⇒ 会让自门凭空多出 embed 资产", hits)
		}
	} else {
		t.Logf("自指格：本件未在盘（%v）⇒ 跳过该格（落件后现读覆盖）", err)
	}
}
