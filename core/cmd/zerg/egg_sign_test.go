// egg_sign_test.go —— `egg_sign.go`（卵开工 · 卵5：cosign 调用形态 / bundle 槽位 / deny-list / 展开区权限）的定向用例。
//
// 夹具一律落 `t.TempDir()`（**自回收** · 跑后不留件 · 不碰真目标）；
// 仓外假 cosign 桩由用例**运行时生成**（✗ 不进仓 · ✗ 不真签 · ✗ 不联网 · ✗ 不装 cosign）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// eggSignStubBin —— 在临时目录里造一枚名为 `cosign` 的仓外桩件，返回 bin 目录与夹具路径。
func eggSignStubBin(t *testing.T, version string) (binDir, egg, key, bundle, sig string) {
	t.Helper()
	root := t.TempDir()
	binDir = filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("造桩目录失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "cosign"), []byte(eggSignStubScript), 0o755); err != nil {
		t.Fatalf("写桩件失败：%v", err)
	}
	if version != "" {
		t.Setenv("FAKE_COSIGN_VERSION", version)
	}
	egg = filepath.Join(root, "demo.egg")
	if err := os.WriteFile(egg, []byte("ZEGG-demo-egg-bytes"), 0o644); err != nil {
		t.Fatalf("写卵夹具失败：%v", err)
	}
	key = filepath.Join(root, "sub-key.pem")
	if err := os.WriteFile(key, []byte("PRIVKEY-STUB"), 0o600); err != nil {
		t.Fatalf("写子钥失败：%v", err)
	}
	bundle = filepath.Join(root, "demo.bundle")
	sig = filepath.Join(root, "demo.sig")
	return binDir, egg, key, bundle, sig
}

func TestEggSignSelfTest(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := eggSignSelfTest(&out, &errb); rc != exitOK {
		t.Fatalf("自检应 rc=0，实得 rc=%d\nstdout:\n%s\nstderr:\n%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "self-test 通过") {
		t.Fatalf("自检未印通过行：%s", out.String())
	}
}

func TestEggSignArgvVerbatim(t *testing.T) {
	got := strings.Join(eggSignBlobArgv("/k.pem", "/b.json", "/x.egg"), "\x1f")
	want := "sign-blob\x1f--key\x1f/k.pem\x1f--bundle\x1f/b.json\x1f--yes\x1f/x.egg"
	if got != want {
		t.Fatalf("sign-blob argv 不符：\n实得 %q\n期望 %q", got, want)
	}
	got = strings.Join(eggSignBlobStdinArgv("/k.pem", "/b.json"), "\x1f")
	want = "sign-blob\x1f--key\x1f/k.pem\x1f--bundle\x1f/b.json\x1f--yes\x1f-"
	if got != want {
		t.Fatalf("sign-blob(stdin) argv 不符：\n实得 %q\n期望 %q", got, want)
	}
	got = strings.Join(eggSignVerifyBlobArgv("/root.pub", "/x.sig", "/b.json", "/x.egg"), "\x1f")
	want = "verify-blob\x1f--key\x1f/root.pub\x1f--signature\x1f/x.sig\x1f--bundle\x1f/b.json\x1f/x.egg"
	if got != want {
		t.Fatalf("verify-blob argv 不符：\n实得 %q\n期望 %q", got, want)
	}
}

func TestEggSignToolMissingIsNoConclusion(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty) // 确保 PATH 里没有 cosign
	res := eggSignRunCosign(nil, eggSignVerifyBlobArgv("/k", "/s", "/b", "/e")...)
	if res.RC != eggSignRCNoConclusion {
		t.Fatalf("工具缺失应 rc=%d，实得 rc=%d", eggSignRCNoConclusion, res.RC)
	}
	if res.Conclusion {
		t.Fatalf("工具缺失竟给了结论（假绿）")
	}
	if !strings.Contains(string(res.Stderr), "不给结论") {
		t.Fatalf("工具缺失未明说「不给结论」：%s", res.Stderr)
	}
}

func TestEggSignStubSawVerbatimArgv(t *testing.T) {
	binDir, egg, key, bundle, _ := eggSignStubBin(t, "")
	t.Setenv("PATH", binDir)
	res := eggSignRunCosign(nil, eggSignBlobArgv(key, bundle, egg)...)
	if !res.Conclusion || res.RC != exitOK {
		t.Fatalf("桩在 PATH 时调用应成立：rc=%d conclusion=%v why=%s", res.RC, res.Conclusion, res.Why)
	}
	want := eggSignArgvLinePrefix + " " + strings.Join(eggSignBlobArgv(key, bundle, egg), " ")
	if got := eggSignStubArgvLine(res.Stdout); got != want {
		t.Fatalf("桩见 argv 不符：\n实得 %q\n期望 %q", got, want)
	}
}

func TestEggSignVersionMismatchIsNoConclusion(t *testing.T) {
	binDir, egg, key, bundle, sig := eggSignStubBin(t, "v9.9.9")
	t.Setenv("PATH", binDir)
	res := eggSignRunCosign(nil, eggSignVerifyBlobArgv(key, sig, bundle, egg)...)
	if res.RC != eggSignRCNoConclusion || res.Conclusion {
		t.Fatalf("版本不符应 rc=%d 不给结论，实得 rc=%d conclusion=%v", eggSignRCNoConclusion, res.RC, res.Conclusion)
	}
}

func TestEggSlotBundleSlotSemantics(t *testing.T) {
	slot := eggSlotEncodeBundle([]byte(`{"mediaType":"application/vnd.dev.sigstore.bundle+json"}`))
	if k := eggSlotClassify(slot); k != eggSlotBundleK {
		t.Fatalf("槽位类别应为 bundle，实得 %q", k)
	}
	if b, ok := eggSlotDecodeBundle(slot); !ok || !bytes.Contains(b, []byte("sigstore")) {
		t.Fatalf("bundle 槽解码失败")
	}
	if _, ok := eggSlotDecodeBundle(make([]byte, 8)); ok {
		t.Fatalf("空槽不该解出 bundle")
	}
	if rc, _ := eggSlotOldReaderVerdict(1, slot); rc == exitOK {
		t.Fatalf("旧读端读到 bundle 槽必拒，实得放行")
	}
	if rc, _ := eggSlotOldReaderVerdict(eggSlotV2Major, nil); rc == exitOK {
		t.Fatalf("旧读端读到主号 2 必拒，实得放行")
	}
	if rc, _ := eggSlotOldReaderVerdict(1, make([]byte, 8)); rc != exitOK {
		t.Fatalf("空槽旧读端应放行")
	}
	if rc, msg := eggSlotNewReaderVerdict(slot); rc != exitOK {
		t.Fatalf("新读端读 bundle 槽应放行：rc=%d %s", rc, msg)
	}
}

func TestEggDenyListVerdicts(t *testing.T) {
	dir := t.TempDir()
	sha := strings.Repeat("ab", 32)
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	clean := filepath.Join(dir, "clean.json")
	write := func(p, body string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("写清单失败：%v", err)
		}
	}
	write(clean, `{"version":1,"not_after":"2026-12-31T00:00:00Z","revoked":[],"sig":"s"}`)
	if d := eggDenyCheckBeforeHatch(clean, sha, now); d.RC != exitOK || d.Verdict != "allow" {
		t.Fatalf("未命中应放行，实得 %+v", d)
	}

	hit := filepath.Join(dir, "hit.json")
	write(hit, `{"version":1,"not_after":"2026-12-31T00:00:00Z","revoked":["`+sha+`"],"sig":"s"}`)
	d := eggDenyCheckBeforeHatch(hit, sha, now)
	if d.RC != eggDenyRCRevoked || !strings.Contains(d.Line, sha) {
		t.Fatalf("命中应 rc=%d 且点名，实得 %+v", eggDenyRCRevoked, d)
	}

	expired := filepath.Join(dir, "expired.json")
	write(expired, `{"version":1,"not_after":"2026-02-01T00:00:00Z","revoked":[],"sig":"s"}`)
	d = eggDenyCheckBeforeHatch(expired, sha, now)
	if d.RC != eggDenyRCFailClosed || !strings.Contains(d.Line, eggDenyDegradeLine) {
		t.Fatalf("过期应 fail-closed 且明写「%s」，实得 %+v", eggDenyDegradeLine, d)
	}

	// 负控：清单缺失 ⇒ fail-closed（✗ 不静默放行）
	if d := eggDenyCheckBeforeHatch(filepath.Join(dir, "absent.json"), sha, now); d.RC != eggDenyRCFailClosed {
		t.Fatalf("清单缺失应 fail-closed rc=%d，实得 %+v", eggDenyRCFailClosed, d)
	}
	// 负控：带外人工已更新 ⇒ 放行但仍出声
	t.Setenv(eggDenyManualEnv, "1")
	if d := eggDenyCheckBeforeHatch(expired, sha, now); d.RC != exitOK || !strings.Contains(d.Line, eggDenyDegradeLine) {
		t.Fatalf("带外人工授权应放行且出声，实得 %+v", d)
	}
}

func TestEggExpandPermThreeFaces(t *testing.T) {
	base := t.TempDir()
	// 自建 0700 根（✗ 不依赖 t.TempDir() 的父目录 mode —— 正控不该受环境摆布）。
	root := filepath.Join(base, "root0700")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("造 0700 根失败：%v", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatalf("chmod 根失败：%v", err)
	}
	exp := filepath.Join(root, "expand")
	if err := os.MkdirAll(exp, 0o700); err != nil {
		t.Fatalf("造展开区失败：%v", err)
	}
	uid := uint32(os.Getuid())
	if ok, probs := eggExpandPermCheck(exp, uid); !ok {
		t.Fatalf("0700 ＋ 父 0700 应绿：%v", probs)
	}
	// 负控①：mode 放宽 ⇒ 红
	if err := os.Chmod(exp, 0o755); err != nil {
		t.Fatalf("chmod 失败：%v", err)
	}
	ok, probs := eggExpandPermCheck(exp, uid)
	if ok || len(probs) == 0 {
		t.Fatalf("展开区 0755 竟判绿")
	}
	hit := false
	for _, p := range probs {
		if strings.Contains(p, "0700") {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("0755 未点名权限面：%v", probs)
	}
	// 负控②：属主不符 ⇒ 红
	if err := os.Chmod(exp, 0o700); err != nil {
		t.Fatalf("chmod 失败：%v", err)
	}
	if ok, probs := eggExpandPermCheck(exp, uid+1); ok {
		t.Fatalf("属主不符竟判绿：%v", probs)
	}
	// 负控③：父目录对 group/other 可达 ⇒ 红
	parent := filepath.Join(root, "parent")
	child := filepath.Join(parent, "expand")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("造父子目录失败：%v", err)
	}
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatalf("chmod 父目录失败：%v", err)
	}
	if ok, probs := eggExpandPermCheck(child, uid); ok {
		t.Fatalf("父目录可下钻竟判绿：%v", probs)
	}
}
