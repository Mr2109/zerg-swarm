// input_fingerprint_test.go —— input_fingerprint.go 的成对测试（非恒绿 · 逐面留牙）。
//
// 组织原则：每条「已对拍面」都配一条会红的钉子；「取不到 ⟷ 空集合」三态各成一子用例；
// CACHED 硬线（命中不得渲染成绿）单独成牙。✗ 本件不定义 TestMain（该符号已被包内他件占用）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ifptHelpSHA256 = "b682873728365bf39a091906a2a1af364b06b2ad5d3a223cae3a3fd6bedc29d5"

func ifptRun(args ...string) (int, string, string) {
	var so, se bytes.Buffer
	rc := inputFingerprintGateMain(&so, &se, args)
	return rc, so.String(), se.String()
}

func ifptWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("夹具写件失败 %s: %v", p, err)
	}
}

// ① 帮助面 golden —— py -h stdout 的 sha256 钉死（GAP-20261001-61）。
func TestInputFingerprintHelpGolden(t *testing.T) {
	rc, out, errs := ifptRun("-h")
	if rc != 0 {
		t.Fatalf("rc=%d 期望 0", rc)
	}
	if errs != "" {
		t.Fatalf("stderr 非空: %q", errs)
	}
	sum := sha256.Sum256([]byte(out))
	if got := hex.EncodeToString(sum[:]); got != ifptHelpSHA256 {
		t.Fatalf("帮助面 sha256 不符\n got=%s\nwant=%s\n正文=%q", got, ifptHelpSHA256, out)
	}
	if !strings.Contains(out, "optional arguments:") {
		t.Fatalf("帮助面缺 3.9 措辞段标题 optional arguments:（正文=%q）", out)
	}
	rc2, out2, _ := ifptRun("--help")
	if rc2 != 0 || out2 != out {
		t.Fatalf("--help 与 -h 不等: rc=%d 等=%v", rc2, out2 == out)
	}
}

// ② 用法错面 —— usage + <prog>: error: 行，退 2（argparse 语义）。
func TestInputFingerprintUsageErrFaces(t *testing.T) {
	cases := []struct {
		name string
		args []string
		msg  string
	}{
		{"未知旗标", []string{"--bogus"}, "input-fingerprint.py: error: unrecognized arguments: --bogus\n"},
		{"缺值", []string{"--tier"}, "input-fingerprint.py: error: argument --tier: expected one argument\n"},
		{"值被他旗标吃掉", []string{"--tier", "--cache", "x"}, "input-fingerprint.py: error: argument --tier: expected one argument\n"},
		{"多余位置参数", []string{"xtra"}, "input-fingerprint.py: error: unrecognized arguments: xtra\n"},
	}
	for _, c := range cases {
		rc, out, errs := ifptRun(c.args...)
		if rc != 2 {
			t.Fatalf("%s: rc=%d 期望 2", c.name, rc)
		}
		if out != "" {
			t.Fatalf("%s: stdout 非空 %q", c.name, out)
		}
		if errs != ifpUsageText+c.msg {
			t.Fatalf("%s: stderr 不符\n got=%q\nwant=%q", c.name, errs, ifpUsageText+c.msg)
		}
	}
	// argparse 的 help action 解析途中即 fire ⇒ 未知旗标在前也照样打帮助退 0。
	rc, out, _ := ifptRun("--bogus", "-h")
	if rc != 0 || out != ifpHelpText {
		t.Fatalf("--bogus -h: rc=%d（期望 0）· 正文等=%v", rc, out == ifpHelpText)
	}
}

// ③ MISS → CACHED 两步 + 硬线「命中不得渲染成 PASS」。
func TestInputFingerprintMissThenCachedNoPass(t *testing.T) {
	dir := t.TempDir()
	inp := filepath.Join(dir, "a.txt")
	cache := filepath.Join(dir, "c.json")
	ifptWrite(t, inp, "v1\n")

	rc, out, errs := ifptRun("--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "true")
	if rc != 0 || errs != "" {
		t.Fatalf("首跑 rc=%d stderr=%q 期望 0/空", rc, errs)
	}
	if !strings.HasPrefix(out, "MISS ") || !strings.Contains(out, "（未命中 ⇒ 真跑并记缓存）") {
		t.Fatalf("首跑 out=%q 期望 MISS 行", out)
	}
	rc, out, errs = ifptRun("--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "true")
	if rc != 0 || errs != "" {
		t.Fatalf("二跑 rc=%d stderr=%q 期望 0/空", rc, errs)
	}
	if !strings.HasPrefix(out, "CACHED ") {
		t.Fatalf("二跑 out=%q 期望 CACHED 行", out)
	}
	// 硬线：命中档 stdout 一个 PASS 字节都不许有。
	if strings.Contains(out, "PASS") {
		t.Fatalf("命中却出现 PASS（硬线：命中不得渲染成绿）: %q", out)
	}
	if !strings.Contains(out, "本行**不是通过**") {
		t.Fatalf("缺「本行不是通过」说明行: %q", out)
	}
}

// ④ --compare 两态一致 ⟷ 不一致两面（后者退 2）。
func TestInputFingerprintCompareFaces(t *testing.T) {
	dir := t.TempDir()
	inp := filepath.Join(dir, "a.txt")
	cache := filepath.Join(dir, "c.json")
	ifptWrite(t, inp, "v1\n")
	if rc, _, _ := ifptRun("--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "true"); rc != 0 {
		t.Fatalf("造缓存 rc=%d", rc)
	}
	rc, out, errs := ifptRun("--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "true", "--compare")
	if rc != 0 {
		t.Fatalf("一致档 rc=%d 期望 0（stderr=%q）", rc, errs)
	}
	if !strings.Contains(out, "── 冷跑对拍：缓存 rc=0 == 冷跑 rc=0 ✓") {
		t.Fatalf("一致档缺对拍行: %q", out)
	}
	rc, _, errs = ifptRun("--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "false", "--compare")
	if rc != 2 {
		t.Fatalf("不一致档 rc=%d 期望 2", rc)
	}
	if errs != "✗ 冷跑对拍不等：缓存 rc=0 ⟷ 冷跑 rc=1 ⇒ rc=2\n" {
		t.Fatalf("不一致档 stderr=%q", errs)
	}
}

// ⑤ 档位闸：seal/stamp 禁缓存退 2 · 未知档退 2（带 py repr 引号）· medium 走缓存。
func TestInputFingerprintTierFaces(t *testing.T) {
	dir := t.TempDir()
	inp := filepath.Join(dir, "a.txt")
	cache := filepath.Join(dir, "c.json")
	ifptWrite(t, inp, "v1\n")
	for _, tier := range []string{"seal", "stamp"} {
		rc, out, errs := ifptRun("--tier", tier, "--cache", cache, "--inputs", inp)
		if rc != 2 || out != "" {
			t.Fatalf("档 %s rc=%d out=%q 期望 2/空", tier, rc, out)
		}
		want := fmt.Sprintf("✗ 盖章档（%s）**禁用缓存** ⇒ 必全冷跑 ⇒ rc=2（不给结论）\n", tier)
		if errs != want {
			t.Fatalf("档 %s stderr=%q want=%q", tier, errs, want)
		}
	}
	rc, _, errs := ifptRun("--tier", "weird", "--cache", cache, "--inputs", inp)
	if rc != 2 || errs != "✗ 未知档位 'weird'（闭集 = daily/medium 走缓存 · seal/stamp 禁用）⇒ rc=2\n" {
		t.Fatalf("未知档 rc=%d stderr=%q", rc, errs)
	}
	if rc, _, _ := ifptRun("--tier", "medium", "--cache", cache, "--inputs", inp); rc != 0 {
		t.Fatalf("medium 档 rc=%d 期望 0", rc)
	}
}

// ⑥ 空转 / 缺 --cache 两档退 2。
func TestInputFingerprintEmptyAndNoCache(t *testing.T) {
	dir := t.TempDir()
	inp := filepath.Join(dir, "a.txt")
	cache := filepath.Join(dir, "c.json")
	ifptWrite(t, inp, "v1\n")
	if rc, _, errs := ifptRun("--tier", "daily", "--cache", cache); rc != 2 || errs != "✗ 无输入件 ⇒ 空转 ⇒ rc=2\n" {
		t.Fatalf("空转 rc=%d stderr=%q", rc, errs)
	}
	if rc, _, errs := ifptRun("--tier", "daily", "--inputs", inp); rc != 2 || errs != "✗ 缺 --cache 件 ⇒ rc=2\n" {
		t.Fatalf("缺 cache rc=%d stderr=%q", rc, errs)
	}
	// --inputs 后紧跟旗标 ⇒ 收 0 件 ⇒ 空转（argparse nargs='*' 语义）
	if rc, _, errs := ifptRun("--tier", "daily", "--cache", cache, "--inputs", "--cmd", "true"); rc != 2 || errs != "✗ 无输入件 ⇒ 空转 ⇒ rc=2\n" {
		t.Fatalf("--inputs 空收 rc=%d stderr=%q", rc, errs)
	}
}

// ⑦ 缺件 ⇒ 字面 MISSING 参与指纹（不静默当 0）；且指纹稳定可比。
func TestInputFingerprintMissingFileFingerprint(t *testing.T) {
	dir := t.TempDir()
	have := filepath.Join(dir, "have.txt")
	gone := filepath.Join(dir, "gone.txt")
	ifptWrite(t, have, "x")
	if got := ifpFileDigest(gone); got != "MISSING" {
		t.Fatalf("缺件摘要=%q 期望 MISSING", got)
	}
	fp1 := ifpFingerprint([]string{have, gone})
	fp2 := ifpFingerprint([]string{gone, have})
	if fp1 != fp2 {
		t.Fatalf("排序不稳：%s / %s", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Fatalf("指纹长度=%d 期望 64", len(fp1))
	}
	// 件出现 ⇒ 指纹必变（MISSING 是真参与，不是被当作不存在）。
	ifptWrite(t, gone, "x")
	if ifpFingerprint([]string{have, gone}) == fp1 {
		t.Fatalf("缺件补齐后指纹未变 ⇒ MISSING 未参与指纹")
	}
	// 目录不是普通件 ⇒ 亦记 MISSING。
	if got := ifpFileDigest(dir); got != "MISSING" {
		t.Fatalf("目录摘要=%q 期望 MISSING", got)
	}
}

// ⑧ 缓存件 JSON 逐字节形状（indent=1 · ensure_ascii=False · 无尾换行 · 键序=插入序）。
func TestInputFingerprintCacheJSONBytes(t *testing.T) {
	dir := t.TempDir()
	inp := filepath.Join(dir, "a.txt")
	cache := filepath.Join(dir, "c.json")
	ifptWrite(t, inp, "v1\n")
	if rc, _, _ := ifptRun("--tier", "daily", "--cache", cache, "--key", "k1", "--inputs", inp); rc != 0 {
		t.Fatalf("造缓存失败")
	}
	fp := ifpFingerprint([]string{inp})
	want := "{\n \"k1\": {\n  \"fp\": \"" + fp + "\",\n  \"rc\": 0,\n  \"inputs\": [\n   \"" + inp + "\"\n  ]\n }\n}"
	got, err := os.ReadFile(cache)
	if err != nil {
		t.Fatalf("读缓存失败: %v", err)
	}
	if string(got) != want {
		t.Fatalf("缓存字节不符\n got=%q\nwant=%q", string(got), want)
	}
}

// ⑨ 三态：取不到（顶层非对象）⟷ 空集合（{}）⟷ 键缺 ⟷ 显式 null —— 各成一格、各断言「不出现对方文案」。
func TestInputFingerprintNullMissingEmptyKeyThreeStates(t *testing.T) {
	dir := t.TempDir()
	inp := filepath.Join(dir, "a.txt")
	cache := filepath.Join(dir, "c.json")
	ifptWrite(t, inp, "v1\n")
	if rc, _, _ := ifptRun("--tier", "daily", "--cache", cache, "--key", "k1", "--inputs", inp); rc != 0 {
		t.Fatalf("造缓存失败")
	}
	raw, _ := os.ReadFile(cache)
	// 追加 k2=null、k3={}，保序。
	body := strings.TrimSuffix(string(raw), "\n}")
	body += ",\n \"k2\": null,\n \"k3\": {}\n}"
	ifptWrite(t, cache, body)

	// 真命中格（对照）——防「三态全被干掉」把命中面一起改死。
	if rc, out, _ := ifptRun("--tier", "daily", "--cache", cache, "--key", "k1", "--inputs", inp); rc != 0 || !strings.HasPrefix(out, "CACHED ") {
		t.Fatalf("k1 真命中被破坏: rc=%d out=%q", rc, out)
	}
	for _, key := range []string{"k2", "k3", "k9"} {
		rc, out, _ := ifptRun("--tier", "daily", "--cache", cache, "--key", key, "--inputs", inp)
		if rc != 0 {
			t.Fatalf("key=%s rc=%d 期望 0（应走 MISS 冷跑）", key, rc)
		}
		if strings.Contains(out, "CACHED") {
			t.Fatalf("key=%s 误判为命中: %q", key, out)
		}
		if !strings.HasPrefix(out, "MISS ") {
			t.Fatalf("key=%s out=%q 期望 MISS", key, out)
		}
	}
	// 取不到面：顶层非对象 ⇒ fail-closed rc=2 具名（≠ 空集合的空对象 {} 走 MISS）。
	ifptWrite(t, cache, "[]")
	rc, out, errs := ifptRun("--tier", "daily", "--cache", cache, "--key", "k1", "--inputs", inp)
	if rc != 2 {
		t.Fatalf("顶层非对象 rc=%d 期望 2", rc)
	}
	if out != "" || !strings.Contains(errs, "缓存件顶层不是 JSON 对象") {
		t.Fatalf("顶层非对象 out=%q stderr=%q", out, errs)
	}
	// 件缺 ⇒ 空集合语义（走 MISS），✗ 不许被当成取不到。
	if err := os.Remove(cache); err != nil {
		t.Fatal(err)
	}
	if rc, out, _ := ifptRun("--tier", "daily", "--cache", cache, "--key", "k1", "--inputs", inp); rc != 0 || !strings.HasPrefix(out, "MISS ") {
		t.Fatalf("件缺 rc=%d out=%q 期望走 MISS", rc, out)
	}
}

// ⑩ 子进程 stdout 序（复刻 CPython 块缓冲）：--cmd 正文先落、本门正文后 flush。
func TestInputFingerprintCmdOutputOrdering(t *testing.T) {
	dir := t.TempDir()
	inp := filepath.Join(dir, "a.txt")
	cache := filepath.Join(dir, "c.json")
	ifptWrite(t, inp, "v1\n")
	rc, out, _ := ifptRun("--tier", "daily", "--cache", cache, "--inputs", inp, "--cmd", "printf hi")
	if rc != 0 {
		t.Fatalf("rc=%d 期望 0", rc)
	}
	if !strings.HasPrefix(out, "hi") {
		t.Fatalf("子进程正文未先落: %q", out)
	}
	if i := strings.Index(out, "MISS "); i <= 0 {
		t.Fatalf("本门正文序不对: %q", out)
	}
	// --cmd 跑不存在的命令 ⇒ 逐字回传 127。
	rc, out, _ = ifptRun("--tier", "daily", "--cache", cache, "--key", "zz", "--inputs", inp, "--cmd", "definitely_not_a_cmd_zzz")
	if rc != 127 {
		t.Fatalf("命令不存在 rc=%d 期望 127（out=%q）", rc, out)
	}
}

// ⑪ --self-test：7 控逐条过 ⇒ rc=0 + 逐字文案。
func TestInputFingerprintSelfTest(t *testing.T) {
	rc, out, errs := ifptRun("--self-test")
	if rc != 0 {
		t.Fatalf("自检 rc=%d 期望 0（stderr=%q out=%q）", rc, errs, out)
	}
	if out != "自检通过：7 控（正控 3 + 负控 4）逐条对拍 ✓\n" {
		t.Fatalf("自检文案不符: %q", out)
	}
}
