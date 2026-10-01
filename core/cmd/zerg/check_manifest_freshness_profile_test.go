// check_manifest_freshness_profile_test.go —— Go 真搬的**成对负控**定向单测（M2 批 · 甲案 C 类）。
//
// ✗ 禁恒绿：本件同时钉「必然判绿」「必然判红」「不给结论（rc=2）」三态，
// 任一态被写死（例如把 rc 恒置 0 或恒置 1）都必有断言失败。
// 用例名：TestCheckManifestFreshnessProfileGatePort（定向：go test -run TestCheckManifestFreshnessProfileGatePort）。
//
// 夹具策略：需要真仓的用例只在本测试的 t.TempDir()（**仓外**）里 `git init` + `commit`；
// ✗ 不碰真仓、✗ 不 git add/commit 到真仓、✗ 不写任何真实状态目录。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mfpTestRepoRoot —— 由 cwd 上溯找同族 py 原件所在仓根（与入口缺省推导同一枚标记）。
func mfpTestRepoRoot(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cur := d
	for i := 0; i < 32; i++ {
		if mfpFileExists(filepath.Join(cur, mfpSelfRel)) {
			return cur
		}
		p := filepath.Dir(cur)
		if p == cur {
			break
		}
		cur = p
	}
	t.Fatalf("找不到同族 py 原件所在仓根（由 %s 上溯）", d)
	return ""
}

// mfpTestFixture —— 在 t.TempDir() 下落一份真仓（仓外 git init），并返回 (repoRoot, manifestDir)。
// withNeighbor=false ⇒ 把邻件真源从夹具的 scripts/gates/ 里挪走（用来钉「真源不在场」那一支）。
func mfpTestFixture(t *testing.T, withNeighbor bool) (string, string) {
	t.Helper()
	src := mfpTestRepoRoot(t)
	root := t.TempDir()
	g := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"check-manifest-freshness-profile.py", "check-manifest-freshness.py"} {
		if name == "check-manifest-freshness.py" && !withNeighbor {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, "scripts", "gates", name))
		if err != nil {
			t.Fatalf("缺同族 py 原件 %s：%v", name, err)
		}
		if err := os.WriteFile(filepath.Join(g, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"commit", "-q", "-m", "seed"},
	} {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("夹具 git %v 失败：%v\n%s", args, err, out)
		}
	}
	mfDir := filepath.Join(root, "mf")
	if err := os.MkdirAll(mfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, mfDir
}

func mfpTestWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mfpTestRun —— 真调入口，返回 (rc, stdout, stderr)。
func mfpTestRun(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkManifestFreshnessProfileGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

func TestCheckManifestFreshnessProfileGatePort(t *testing.T) {
	repo, mfDir := mfpTestFixture(t, true)
	t.Chdir(repo) // 让入口的自根推导落在夹具上（同 py 的 HERE 面）

	// 夹具清单：source_sha 与夹具 HEAD 对齐 ⇒ 两条判据都「成立」。
	head := mfpGitTolerant(repo, []string{"rev-parse", "HEAD"})
	if head == "" {
		t.Fatal("夹具 HEAD 取不到")
	}
	fresh := filepath.Join(mfDir, "fresh.json")
	mfpTestWrite(t, fresh, `{"commit":"`+head+`","source_sha":"`+head+`","dirty":false,"generated_at":"2026-09-23T00:00:00Z"}`)
	mismatch := filepath.Join(mfDir, "mismatch.json")
	mfpTestWrite(t, mismatch, `{"commit":"`+head+`","source_sha":"ffffffffffff","dirty":false}`)
	dirty := filepath.Join(mfDir, "dirty.json")
	mfpTestWrite(t, dirty, `{"commit":"`+head+`","source_sha":"`+head+`","dirty":true}`)
	bad := filepath.Join(mfDir, "bad.json")
	mfpTestWrite(t, bad, "{not json")

	// ① 必然判绿：dev 档 + 新鲜清单（两条判据皆「成立」）⇒ rc=0。
	rc, out, _ := mfpTestRun([]string{"--profile", "dev", "--manifest", fresh, "--repo", "", "--release-head", head})
	if rc != 0 {
		t.Fatalf("① dev+新鲜 应 rc=0，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "成立") || !strings.Contains(out, "rc=0") {
		t.Fatalf("① 判定行不对：\n%s", out)
	}

	// ② 必然判红：public 档 + source_sha 不符 ⇒ rc=1（且必须点明「不是 stale 告警，是红」）。
	rc, out, _ = mfpTestRun([]string{"--profile", "public", "--manifest", mismatch, "--repo", "", "--release-head", head})
	if rc != 1 {
		t.Fatalf("② public+不符 应 rc=1，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "**阻断**") || !strings.Contains(out, "用 A 打包、声称是 B") {
		t.Fatalf("② 阻断判定行不对：\n%s", out)
	}

	// ③ 同一条命中在 dev 档只告警 ⇒ rc=0（档位真的改退码，不是写死）。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", mismatch, "--repo", "", "--release-head", head})
	if rc != 0 {
		t.Fatalf("③ dev+不符 应 rc=0（只告警），实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "**只告警**") {
		t.Fatalf("③ 告警判定行不对：\n%s", out)
	}

	// ④ public + dirty=true ⇒ 命中 J2 ⇒ rc=1（第二条判据也真的参与退码）。
	rc, out, _ = mfpTestRun([]string{"--profile", "public", "--manifest", dirty, "--repo", "", "--release-head", head})
	if rc != 1 {
		t.Fatalf("④ public+dirty 应 rc=1，实得 rc=%d\n%s", rc, out)
	}

	// ⑤ 不给结论：清单件读不到 ⇒ rc=2（且必须自陈「不给结论」）。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", filepath.Join(mfDir, "nope.json")})
	if rc != 2 {
		t.Fatalf("⑤ 缺件 应 rc=2，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "缺件") || !strings.Contains(out, "不许静默放绿") {
		t.Fatalf("⑤ 缺件文案不对：\n%s", out)
	}

	// ⑥ 不给结论：JSON 坏 ⇒ rc=2。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", bad})
	if rc != 2 {
		t.Fatalf("⑥ JSON 坏 应 rc=2，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "JSON 解析失败") {
		t.Fatalf("⑥ JSON 坏文案不对：\n%s", out)
	}

	// ⑦ 不给结论：给的仓与 --release-head 都取不到 ⇒ rc=2（「都取不到」才退 2）。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", fresh, "--repo", "", "--release-head", ""})
	if rc != 2 {
		t.Fatalf("⑦ 取不到 HEAD 应 rc=2，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "取不到发布提交 HEAD") {
		t.Fatalf("⑦ 取不到文案不对：\n%s", out)
	}

	// ⑧ 只给 --release-head（仓取不到）⇒ 不算「都取不到」⇒ 照走判据 rc=0。
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", fresh, "--repo", "", "--release-head", head})
	if rc != 0 {
		t.Fatalf("⑧ 显式 release-head 应 rc=0，实得 rc=%d\n%s", rc, out)
	}

	// ⑨ Q-176：--json 里有不认的字段名 ⇒ rc=2 ＋ 印可选字段表（stdout 为空）。
	rc, out, se := mfpTestRun([]string{"--profile", "dev", "--manifest", fresh, "--json", "hits,warnings"})
	if rc != 2 {
		t.Fatalf("⑨ 不认字段 应 rc=2，实得 rc=%d\nstdout=%s", rc, out)
	}
	if out != "" || !strings.Contains(se, "不认的字段名") || !strings.Contains(se, "可选字段表") {
		t.Fatalf("⑨ Q-176 面不对：stdout=%q stderr=%q", out, se)
	}

	// ⑩ 合法字段照旧走命中面：--json profile,rc ⇒ 出 JSON 行且 rc 照判。
	rc, out, _ = mfpTestRun([]string{"--profile", "public", "--manifest", fresh, "--repo", "", "--release-head", head, "--json", "profile,rc"})
	if rc != 0 {
		t.Fatalf("⑩ public+新鲜 应 rc=0，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, `{"profile": "public", "rc": 0}`) {
		t.Fatalf("⑩ JSON 行不对：\n%s", out)
	}

	// ⑪ --now 注入旋钮确定化：同一 --now 两次逐字节相同（且与不给 --now 同构）。
	old := filepath.Join(mfDir, "old.json")
	mfpTestWrite(t, old, `{"commit":"x","source_sha":"x","dirty":false,"generated_at":"2020-01-01T00:00:00Z"}`)
	a := []string{"--profile", "dev", "--manifest", old, "--repo", "", "--release-head", "x", "--max-age-hours", "1", "--now", "2026-10-01T00:00:00Z"}
	rcA, outA, _ := mfpTestRun(a)
	_, outB, _ := mfpTestRun(a)
	if outA != outB {
		t.Fatalf("⑪ --now 未确定化：两次输出不同\n%s\n---\n%s", outA, outB)
	}
	if rcA != 0 || !strings.Contains(outA, "> 上限 1h") || !strings.Contains(outA, "清单过旧") {
		t.Fatalf("⑪ --now 注入后的过龄判词不对（rc=%d）：\n%s", rcA, outA)
	}

	// ⑫ 真源不在场 ⇒ stale 面照实报「取不到」，不自己编一份（rc 仍由两条判据定）。
	repo2, mfDir2 := mfpTestFixture(t, false)
	t.Chdir(repo2)
	head2 := mfpGitTolerant(repo2, []string{"rev-parse", "HEAD"})
	fresh2 := filepath.Join(mfDir2, "fresh.json")
	mfpTestWrite(t, fresh2, `{"commit":"`+head2+`","source_sha":"`+head2+`","dirty":false}`)
	rc, out, _ = mfpTestRun([]string{"--profile", "dev", "--manifest", fresh2, "--repo", "", "--release-head", head2})
	if rc != 0 {
		t.Fatalf("⑫ 真源不在场应仍判绿 rc=0，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "取不到") {
		t.Fatalf("⑫ 真源不在场文案不对：\n%s", out)
	}

	// ⑬ --list-rules 走静态表，rc=0 且不含判定面（防把静态面写成判据面）。
	rc, out, _ = mfpTestRun([]string{"--list-rules"})
	if rc != 0 || !strings.Contains(out, "R-23") {
		t.Fatalf("⑬ --list-rules 不对（rc=%d）：\n%s", rc, out)
	}
}
