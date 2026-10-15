package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestMirrorPublicEmbeddedLibByteExact —— 内嵌引擎（base64url）解码后必须与原件**逐字节**同：
// sha256 与字节数两处锚（`mpPyLibSHA256` / `mpPyLibBytesLen`）现读必须相等。
func TestMirrorPublicEmbeddedLibByteExact(t *testing.T) {
	b := mpPyLibBytes()
	if b == nil {
		t.Fatalf("mpPyLibBytes() 解码失败（base64 常量坏了）")
	}
	if len(b) != mpPyLibBytesLen {
		t.Fatalf("内嵌引擎字节数 %d ≠ 锚 %d（与原件漂移）", len(b), mpPyLibBytesLen)
	}
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	if got != mpPyLibSHA256 {
		t.Fatalf("内嵌引擎 sha256 %s ≠ 锚 %s（与原件漂移）", got, mpPyLibSHA256)
	}
	// ★ F51：**双向校验**（✗ 不再只断言自身锚）—— 解码必须 == 盘上真源
	// `publish/mirror-public-lib.py` **逐字节**。上面那两格是「同语自证」（承载件与自身锚比）；
	// 本格才是「承载件 ⟷ 真源同步」那一刀（与生成器 `mirror_public_pyblocks_gen.go` 的 `--check` 同判据）。
	root := repoRoot()
	if root == "" {
		t.Fatalf("仓根解析不到 ⇒ 双向校验前置不成立（✗ 不留 Skip · fail-closed）")
	}
	pyPath := filepath.Join(root, "publish", "mirror-public-lib.py")
	src, err := os.ReadFile(pyPath)
	if err != nil {
		t.Fatalf("真源读不到 %s（✗ 不留 Skip · fail-closed）：%v", pyPath, err)
	}
	if !bytes.Equal(b, src) {
		t.Fatalf("内嵌引擎 == 自身锚，但 ⟷ 盘上真源**不是同一份**：解码 %d B sha256 %s ⟷ 真源 %d B sha256 %s（首个不同字节偏移 %d）",
			len(b), got, len(src), mpPygSum(src), mpPygFirstDiff(b, src))
	}
}

// TestMirrorPublicInlineBlockFifteenSameSource —— `_INL_BLOCK` **15 条**必须与 Go 半 `chsecBlockSpecs`
// **逐字同源同口径**（任务单点名对照 check_history_secrets.go 的 15 条）。
//
// 判据两腿：① 解码后的引擎里 `_INL_BLOCK = […]` 的元组行数 == 15 == len(chsecBlockSpecs)；
// ② chsecBlockSpecs 每一条的正则正文（拼接构造 · 源码里无连成一串的原串）都能在引擎里以 `r"<pat>"`
// 形态逐字命中 ⇒ 同一套 BLOCK 口径（顺序也照抄，逐条核）。
func TestMirrorPublicInlineBlockFifteenSameSource(t *testing.T) {
	text := string(mpPyLibBytes())
	if text == "" {
		t.Fatalf("内嵌引擎为空")
	}
	const marker = "_INL_BLOCK = ["
	i := strings.Index(text, marker)
	if i < 0 {
		t.Fatalf("内嵌引擎里找不到 %q", marker)
	}
	rest := text[i+len(marker):]
	j := strings.Index(rest, "\n]")
	if j < 0 {
		t.Fatalf("找不到 _INL_BLOCK 的收尾 `]`")
	}
	block := rest[:j]
	n := 0
	for _, ln := range strings.Split(block, "\n") {
		if strings.Contains(strings.TrimSpace(ln), `(r"`) {
			n++
		}
	}
	if n != 15 {
		t.Fatalf("_INL_BLOCK 元组行数 %d ≠ 15", n)
	}
	if len(chsecBlockSpecs) != 15 {
		t.Fatalf("chsecBlockSpecs 条数 %d ≠ 15", len(chsecBlockSpecs))
	}
	if n != len(chsecBlockSpecs) {
		t.Fatalf("_INL_BLOCK %d 条 ≠ chsecBlockSpecs %d 条", n, len(chsecBlockSpecs))
	}
	// ★ 口径说明（现读发现 · 记入回执）：Go 半 `chsecBlockSpecs[7]`（口令赋值）与 py 半同条目
	//   正文差**一处冗余转义** —— py 是 `[\"']`（字符类里转义引号），Go 半是 `["']`。RE2 里二者**语义同一**
	//   （引号非转义字符，`\'`/`\"` 与 `'`/`"` 等价）⇒ 判「同口径」用「去引号冗余转义」归一后比对。
	//   本件内嵌的是 py 原字节（逐字节同源）；此处只把这一处**已知等价形**归一，✗ 不放宽条数/顺序。
	norm := func(s string) string { return strings.ReplaceAll(s, "\\\"", "\"") }
	ntext := norm(text)
	for idx, spec := range chsecBlockSpecs {
		needle := `r"` + spec.pat + `"`
		if !strings.Contains(ntext, needle) {
			t.Fatalf("第 %d 条口径不同源：引擎（归一后）里找不到 %q（Go 半：%q）", idx+1, needle, spec.pat)
		}
	}
}

// TestMirrorPublicHelpAndUnknownArg —— 入口的无依赖两支：`-h` ⇒ rc=0 且出旧壳 -h 正文；
// 未知参数 ⇒ rc=2（照旧壳）。都不触仓、不触网。
func TestMirrorPublicHelpAndUnknownArg(t *testing.T) {
	var out, errb strings.Builder
	if rc := mirrorPublicNative("/nonexistent-root", &out, &errb, []string{"-h"}); rc != 0 {
		t.Fatalf("-h rc=%d ≠ 0", rc)
	}
	if !strings.Contains(out.String(), "publish/mirror-public.sh") {
		t.Fatalf("-h 正文里应含旧壳头注释（含件名），实得前 80 字：%q", firstN(out.String(), 80))
	}
	out.Reset()
	errb.Reset()
	if rc := mirrorPublicNative("/nonexistent-root", &out, &errb, []string{"--nope"}); rc != 2 {
		t.Fatalf("未知参数 rc=%d ≠ 2", rc)
	}
	if !strings.Contains(errb.String(), "未知参数: --nope") {
		t.Fatalf("未知参数 stderr 正文不同源：%q", errb.String())
	}
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// TestMirrorPublicListOnlyParityLive —— 两态对拍（**默认跳过** · 设 MIRROR_PARITY_LIVE=1 才跑）：
// 同输入（真仓 + `--list-only`）下 **旧壳** `bash publish/mirror-public.sh --list-only` ⟷ **Go 真身**
// `mirrorPublicNative` 的 rc/stdout/stderr 逐字同（掩时间戳/临时路径）。
func TestMirrorPublicListOnlyParityLive(t *testing.T) {
	if os.Getenv("MIRROR_PARITY_LIVE") != "1" {
		t.Skip("设 MIRROR_PARITY_LIVE=1 才跑两态对拍（要真仓 + python3）")
	}
	root := repoRoot()
	if root == "" {
		t.Fatalf("repoRoot() 解析不到仓根")
	}
	shell := filepath.Join(root, "publish", "mirror-public.sh")
	if _, err := os.Stat(shell); err != nil {
		t.Fatalf("哨兵失守：旧壳不在盘 %s（✗ 不留 Skip · fail-closed）：%v", shell, err)
	}
	argv := []string{"--list-only"}

	// 旧链
	oc := armChildGroup(exec.Command("bash", append([]string{shell}, argv...)...))
	oc.Dir = root
	var oOut, oErr strings.Builder
	oc.Stdout, oc.Stderr = &oOut, &oErr
	orc := mpRun(oc)

	// 新 Go 真身
	var gOut, gErr strings.Builder
	grc := mirrorPublicNative(root, &gOut, &gErr, argv)

	// 掩时间戳（默认 `--out`/报告名里带 `<TS>`；两跑不在同一秒 ⇒ 原样比会假红）。
	maskTS := func(s string) string {
		return regexp.MustCompile(`\d{8}-\d{6}`).ReplaceAllString(s, "<TS>")
	}
	if orc != grc {
		t.Fatalf("rc 不同：旧链 %d ⟷ 新 Go %d", orc, grc)
	}
	if maskTS(oOut.String()) != maskTS(gOut.String()) {
		t.Fatalf("stdout 不同：旧链 %d 字节 ⟷ 新 Go %d 字节", len(oOut.String()), len(gOut.String()))
	}
	if maskTS(oErr.String()) != maskTS(gErr.String()) {
		t.Fatalf("stderr 不同：旧链 %d 字节 ⟷ 新 Go %d 字节", len(oErr.String()), len(gErr.String()))
	}
}

// TestMirrorPublicToyParityLive —— 两态对拍（**默认跳过** · 设 MIRROR_TOY_LIVE=1 才跑）：**仓外靶**
// （`t.TempDir()` 里造的 toy git 仓），两格样例：
//
//	① 块命中样例：`core/x3demo.go` 含 `edge-node` 词、`core/mini-node-1.md` 含 `mini-node-1`（白名单 `core/` 收）
//	   ⇒ 引擎逐 blob 敏感扫描命中 `_INL_BLOCK` ⇒ 整批中止（两队都 rc=1）；
//	② 干净样例：只有 `core/clean.go` ⇒ 走完整写路径（镜像 + 私有面门禁 + 软门禁，两队都 rc=0）。
//
// 掩掉 `--out` 目录名与时间戳后，旧链 ⟷ 新 Go 的 rc/stdout/stderr **逐字同**。夹具落仓外（TempDir 自清）。
func TestMirrorPublicToyParityLive(t *testing.T) {
	if os.Getenv("MIRROR_TOY_LIVE") != "1" {
		t.Skip("设 MIRROR_TOY_LIVE=1 才跑（要造 toy 仓 + python3 + bin/zerg）")
	}
	root := repoRoot()
	if root == "" {
		t.Fatalf("repoRoot() 解析不到仓根")
	}
	shell := filepath.Join(root, "publish", "mirror-public.sh")

	git := func(dir string, args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Toy", "GIT_AUTHOR_EMAIL=toy@example.invalid",
			"GIT_COMMITTER_NAME=Toy", "GIT_COMMITTER_EMAIL=toy@example.invalid", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mk := func(files map[string]string) string {
		dir := t.TempDir()
		git(dir, "init", "-q", "-b", "main")
		for p, c := range files {
			fp := filepath.Join(dir, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fp, []byte(c), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		git(dir, "add", "-A")
		git(dir, "commit", "-q", "-m", "toy")
		return dir
	}
	// 掩时间戳（`<TS>`）；`--out` 名两侧不同 ⇒ 各自掩成 `<OUT>`（两侧用不同 out 目录）。
	mask := func(s, out string) string {
		s = strings.ReplaceAll(s, out, "<OUT>")
		return regexp.MustCompile(`\d{8}-\d{6}`).ReplaceAllString(s, "<TS>")
	}

	cases := []struct {
		name  string
		files map[string]string
	}{
		{"块命中(edge-node/mini-node-1)", map[string]string{
			"core/clean.go":  "package main\n// clean\n",
			"core/x3demo.go": "package main\n\nvar edge-node = 1\n",
			"core/mini-node-1.md":  "note: mini-node-1 here\n",
		}},
		{"干净", map[string]string{
			"core/clean.go": "package main\n// clean\n",
		}},
	}
	for _, tc := range cases {
		toy := mk(tc.files)
		outA := filepath.Join(t.TempDir(), "oA")
		outB := filepath.Join(t.TempDir(), "oB")
		oc := armChildGroup(exec.Command("bash", shell, "--repo", toy, "--out", outA, "--range", "HEAD"))
		var oOut, oErr strings.Builder
		oc.Stdout, oc.Stderr = &oOut, &oErr
		orc := mpRun(oc)
		var gOut, gErr strings.Builder
		grc := mirrorPublicNative(root, &gOut, &gErr,
			[]string{"--repo", toy, "--out", outB, "--range", "HEAD"})
		if orc != grc {
			t.Fatalf("[%s] rc 不同：旧链 %d ⟷ 新 Go %d\n--旧链 stdout--\n%s\n--新 Go stdout--\n%s",
				tc.name, orc, grc, mask(oOut.String(), outA), mask(gOut.String(), outB))
		}
		if mask(oOut.String(), outA) != mask(gOut.String(), outB) {
			t.Fatalf("[%s] stdout 不同\n--旧链--\n%s\n--新 Go--\n%s",
				tc.name, mask(oOut.String(), outA), mask(gOut.String(), outB))
		}
		if mask(oErr.String(), outA) != mask(gErr.String(), outB) {
			t.Fatalf("[%s] stderr 不同\n--旧链--\n%s\n--新 Go--\n%s",
				tc.name, mask(oErr.String(), outA), mask(gErr.String(), outB))
		}
	}
}

// TestMirrorPublicPyblocksGenFourStates —— 生成器/双向校验门 `gen-mirror-public-pyblocks` 的**四态自证**：
//
//	① 一致（盘上真源 ＋ 盘上真承载件）   ⇒ rc=0
//	② 真源被改一字节（仓外副本）         ⇒ rc=1（✗ 不得只因自身锚而绿 · 旧断言此处照绿）
//	③ 承载件被手改（仓外副本）           ⇒ rc=1
//	④ 真源取不到                         ⇒ rc=2（不给结论）
//
// ★ 「仓外副本」= 全在 t.TempDir() 里造；真仓两件**只读**（✗ 不碰）。
func TestMirrorPublicPyblocksGenFourStates(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Fatalf("仓根解析不到 ⇒ 四态前置不成立（✗ 不留 Skip · fail-closed）")
	}
	py := filepath.Join(root, "publish", "mirror-public-lib.py")
	carry := filepath.Join(root, "core", "cmd", "zerg", "family_mirror_public_pyblocks.go")
	src, err := os.ReadFile(py)
	if err != nil {
		t.Fatalf("真源读不到 %s：%v", py, err)
	}
	carryTxt, err := os.ReadFile(carry)
	if err != nil {
		t.Fatalf("承载件读不到 %s：%v", carry, err)
	}
	run := func(p, c string) (int, string, string) {
		var o, e strings.Builder
		rc := mpPyblocksGenGateMain(&o, &e, []string{"--check", "--py", p, "--carry", c})
		return rc, o.String(), e.String()
	}
	// ① 一致 ⇒ rc=0
	if rc, o, e := run(py, carry); rc != 0 {
		t.Fatalf("① 一致态 rc=%d ≠ 0\nstdout=%s\nstderr=%s", rc, o, e)
	}
	dir := t.TempDir()
	// ② 真源改一字节（仓外副本）⇒ rc=1 且点名「不是同一份」
	mod := append([]byte(nil), src...)
	mod[len(mod)/2] ^= 0x01
	modPy := filepath.Join(dir, "mirror-public-lib.py")
	if err := os.WriteFile(modPy, mod, 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, o, _ := run(modPy, carry); rc != 1 || !strings.Contains(o, "不是同一份") {
		t.Fatalf("② 真源改一字节 rc=%d（期望 1 且点名「不是同一份」）\nstdout=%s", rc, o)
	}
	// ③ 承载件被手改（仓外副本 · 翻 b64 里一个字符 ⇒ 解码内容变）
	s := string(carryTxt)
	at := strings.Index(s, "const mpPyLibB64 = \"")
	if at < 0 {
		t.Fatal("③ 前置不成立：承载件里定位不到 mpPyLibB64 常量")
	}
	pos := at + len("const mpPyLibB64 = \"") + 30
	rep := byte('A')
	if s[pos] == 'A' {
		rep = 'B'
	}
	bad := s[:pos] + string(rep) + s[pos+1:]
	if bad == s {
		t.Fatal("③ 前置不成立：承载件没被改动")
	}
	badCarry := filepath.Join(dir, "carry-hand-edited.go")
	if err := os.WriteFile(badCarry, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, o, _ := run(py, badCarry); rc != 1 {
		t.Fatalf("③ 承载件手改 rc=%d ≠ 1\nstdout=%s", rc, o)
	}
	// ④ 真源取不到 ⇒ rc=2（不给结论）
	if rc, _, e := run(filepath.Join(dir, "no-such.py"), carry); rc != 2 {
		t.Fatalf("④ 真源取不到 rc=%d ≠ 2（不给结论）\nstderr=%s", rc, e)
	}
}
