// mirror_public_pyblocks_gen.go —— 镜像引擎承载件的**生成器 ＋ 双向校验门**（收口遗留 F51 的根因件）。
//
// 承载件 = `core/cmd/zerg/family_mirror_public_pyblocks.go`：把真源 `publish/mirror-public-lib.py`
// **逐字节**内嵌成 base64url 常量（`mpPyLibSHA256` / `mpPyLibBytesLen` / `mpPyLibB64`）；运行期由
// Go 真身 `family_mirror_public_native.go` 解码后经 `python3 -c` 交 stdin ⇒ **承载件是运行期真源**。
//
// 病的根因（本件要治的）：该承载件**仓内历来无生成器** ⇒ 只能**手改**；而仓内唯一的机检
// （`family_mirror_public_native_test.go` 的 `TestMirrorPublicEmbeddedLibByteExact`）只断言
// 「解码 == 自身锚（`mpPyLibSHA256` / `mpPyLibBytesLen`）」—— 那是**同语自证**：承载件与盘上 py
// 一旦分头漂移，自身锚照样自洽 ⇒ 机检报绿而两件早已不是同一份（历史事实照实：解码 139507 B
// ⟷ 盘上 144675 B ⟷ HEAD 版 144500 B 曾三者互不相等）。
//
// 本件补两样：
//
//	① **生成器**（默认档）：读真源 `publish/mirror-public-lib.py` ⇒ 逐字节重生承载件那三格
//	   （sha256 / 字节数 / b64 块），✗ 不再手改；
//	② **双向校验**（`--check` 档）：**解码 == 盘上真源逐字节** ⇒ 漂移必红且逐格点名
//	   （✗ 不再只比自身锚）。
//
// 形态照仓内同类生成器先例（`gen_glossary_exports.go` / `gen_cli_reference.go` / `gen_gap_ledger.go` /
// `doc_index_gen.go` / `gen_tools_md.go`，逐件同形）：入口签名 `func <名>GateMain(stdout, stderr io.Writer, argv []string) int`
// （argv 不含程序名），名下挂进 `gate_inproc_table.go` 的 inproc 表 ⇒ 一条命令打得着
// `zerg gate check gen-mirror-public-pyblocks`（重生）与 `… --check`（对拍）。★ 不改 `main.go`
// 命令树（同族先例同：生成器只走 inproc 路由，不动命令面）。
//
// 退码（生成器族同形）：0 一致／已重生；1 漂移（含「自身锚与解码不符」）；2 取数面拿不到
// （真源 / 承载件缺 · 读不动 · 定位不到三格 · 用法错 ⇒ **不给结论**，✗ 不静默绿）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// mpPygCarryRel —— 承载件（生成物）仓内相对路径。
	mpPygCarryRel = "core/cmd/zerg/family_mirror_public_pyblocks.go"
	// mpPygSourceRel —— 真源（本生成器的唯一输入）仓内相对路径。
	mpPygSourceRel = pathMirrorLibPy
	// mpPygChunk —— b64 常量每行字符数（与承载件既有形态逐字一致 · 122）。
	mpPygChunk = 122
)

const mpPygUsage = `zerg gate check gen-mirror-public-pyblocks [--check] [--root <仓根>] [--py <真源件>] [--carry <承载件>]

  默认档  读真源 ⇒ 逐字节重生承载件三格（mpPyLibSHA256 / mpPyLibBytesLen / mpPyLibB64）
  --check 只读对拍：承载件 mpPyLibB64 解码 == 盘上真源逐字节？（✗ 不再只比自身锚）

退码：0 一致／已重生；1 漂移（逐格点名）；2 取数面拿不到（不给结论）
`

// mpPyblocksGenGateMain —— 本件入口（inproc 口径：argv **不含程序名**）。
func mpPyblocksGenGateMain(stdout, stderr io.Writer, argv []string) int {
	check := false
	root := ""
	pyPath := ""
	carryPath := ""
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch a {
		case "--check":
			check = true
		case "-h", "--help":
			fmt.Fprint(stdout, mpPygUsage)
			return 0
		case "--root", "--py", "--carry":
			if i+1 >= len(argv) {
				fmt.Fprintf(stderr, "gen-mirror-public-pyblocks: %s 缺值 ⇒ 不给结论\n%s", a, mpPygUsage)
				return 2
			}
			i++
			switch a {
			case "--root":
				root = argv[i]
			case "--py":
				pyPath = argv[i]
			case "--carry":
				carryPath = argv[i]
			}
		default:
			fmt.Fprintf(stderr, "gen-mirror-public-pyblocks: 未知参数 %s ⇒ 不给结论\n%s", a, mpPygUsage)
			return 2
		}
	}
	// 仓根只在需要按仓内相对路径定位时才求（给了 --py 与 --carry 就与仓根无关）。
	if pyPath == "" || carryPath == "" {
		if root == "" {
			root = repoRoot()
		}
		if root == "" {
			fmt.Fprintln(stderr, "gen-mirror-public-pyblocks: 仓根取不到 ⇒ 不给结论")
			return 2
		}
	}
	if pyPath == "" {
		pyPath = filepath.Join(root, filepath.FromSlash(mpPygSourceRel))
	}
	if carryPath == "" {
		carryPath = filepath.Join(root, filepath.FromSlash(mpPygCarryRel))
	}
	if check {
		return mpPygCheck(stdout, stderr, pyPath, carryPath)
	}
	return mpPygRegen(stdout, stderr, pyPath, carryPath)
}

// mpPygCheck —— 双向校验：承载件解码 ⟷ 盘上真源**逐字节**（主判据）＋承载件自身锚自洽（次级）。
func mpPygCheck(stdout, stderr io.Writer, pyPath, carryPath string) int {
	src, rc := mpPygReadSource(stderr, pyPath)
	if rc != 0 {
		return rc
	}
	carry, err := os.ReadFile(carryPath)
	if err != nil {
		fmt.Fprintf(stderr, "gen-mirror-public-pyblocks: 承载件读不到 %s：%v ⇒ 不给结论\n", carryPath, err)
		return 2
	}
	b64, ok := mpPygParseB64(string(carry))
	if !ok {
		fmt.Fprintf(stderr, "gen-mirror-public-pyblocks: 承载件 %s 里定位不到 mpPyLibB64 常量 ⇒ 不给结论\n", carryPath)
		return 2
	}
	dec, derr := base64.RawURLEncoding.DecodeString(b64)
	srcSum := mpPygSum(src)
	// 自身锚（承载件自报的两格）—— **次级**结构校验，✗ 不是本门的判据。
	anchorOK := derr == nil && mpPygSum(dec) == mpPyLibSHA256 && len(dec) == mpPyLibBytesLen
	// 主判据：解码 == 盘上真源逐字节。
	drift := ""
	switch {
	case derr != nil:
		drift = fmt.Sprintf("承载件 mpPyLibB64 解码失败（坏串）：%v", derr)
	case !bytes.Equal(dec, src):
		drift = "承载件解码 ⟷ 盘上真源**不是同一份**"
	case !anchorOK:
		drift = "解码 == 盘上真源，但承载件自身锚（mpPyLibSHA256 / mpPyLibBytesLen）与解码不符（被手改）"
	}
	if drift == "" {
		fmt.Fprintf(stdout, "一致：承载件解码 == 盘上真源逐字节\n  真源 : %s  %d B  sha256 %s\n  解码 : mpPyLibB64       %d B  sha256 %s\n",
			pyPath, len(src), srcSum, len(dec), mpPygSum(dec))
		return 0
	}
	fmt.Fprintf(stdout, "✗ 漂移：%s\n", drift)
	fmt.Fprintf(stdout, "  真源   : %s  %d B  sha256 %s\n", pyPath, len(src), srcSum)
	if derr != nil {
		fmt.Fprintf(stdout, "  承载件 : mpPyLibB64 **解不开**（坏串）\n")
	} else {
		fmt.Fprintf(stdout, "  承载件 : mpPyLibB64 解码  %d B  sha256 %s（首个不同字节偏移 %d）\n",
			len(dec), mpPygSum(dec), mpPygFirstDiff(dec, src))
	}
	fmt.Fprintf(stdout, "  自身锚 : mpPyLibSHA256=%s · mpPyLibBytesLen=%d（★ 同语自证格 · ✗ 不是本门判据）\n",
		mpPyLibSHA256, mpPyLibBytesLen)
	fmt.Fprintf(stdout, "  ⇒ 重生：zerg gate check gen-mirror-public-pyblocks\n")
	return 1
}

// mpPygRegen —— 默认档：读真源 ⇒ 逐字节重生承载件三格（其余逐字节不动）。
func mpPygRegen(stdout, stderr io.Writer, pyPath, carryPath string) int {
	src, rc := mpPygReadSource(stderr, pyPath)
	if rc != 0 {
		return rc
	}
	carry, err := os.ReadFile(carryPath)
	if err != nil {
		fmt.Fprintf(stderr, "gen-mirror-public-pyblocks: 承载件读不到 %s：%v ⇒ 不给结论\n", carryPath, err)
		return 2
	}
	sum := mpPygSum(src)
	out, ok := mpPygRebuild(string(carry), src, sum, len(src))
	if !ok {
		fmt.Fprintf(stderr, "gen-mirror-public-pyblocks: 承载件 %s 里三格常量定位不到（形态不合）⇒ 不给结论\n", carryPath)
		return 2
	}
	if out == string(carry) {
		fmt.Fprintf(stdout, "已是最新（逐字节相同）：%s\n  真源 %s  %d B  sha256 %s\n",
			carryPath, pyPath, len(src), sum)
		return 0
	}
	before := mpPygSum([]byte(carry))
	if err := os.WriteFile(carryPath, []byte(out), 0o644); err != nil {
		fmt.Fprintf(stderr, "gen-mirror-public-pyblocks: 写承载件失败 %s：%v ⇒ 不给结论\n", carryPath, err)
		return 2
	}
	fmt.Fprintf(stdout, "已重生：%s\n  sha256 %s → %s（%d → %d B · 真源 sha256 %s）\n",
		carryPath, before, mpPygSum([]byte(out)), len(carry), len(out), sum)
	return 0
}

// ── 底座（纯函数）────────────────────────────────────────────────────

// mpPygReadSource —— 读真源件。拿不到 / 空件 ⇒ rc=2（不给结论 · ✗ 不静默绿）。
func mpPygReadSource(stderr io.Writer, pyPath string) ([]byte, int) {
	src, err := os.ReadFile(pyPath)
	if err != nil {
		fmt.Fprintf(stderr, "gen-mirror-public-pyblocks: 真源读不到 %s：%v ⇒ 不给结论\n", pyPath, err)
		return nil, 2
	}
	if len(src) == 0 {
		fmt.Fprintf(stderr, "gen-mirror-public-pyblocks: 真源 %s 是空件 ⇒ 不给结论\n", pyPath)
		return nil, 2
	}
	return src, 0
}

// mpPygSum —— 内容的 sha256 十六进制小写（与承载件 `mpPyLibSHA256` 同一口径）。
func mpPygSum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// mpPygFirstDiff —— 两份内容的**首个不同字节偏移**（全等 ⇒ 较短者长度）。
func mpPygFirstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// mpPygWrap —— 把 b64 串按每行 mpPygChunk 字符切块（b64 是 ASCII ⇒ 按字节切即按字符切）。
func mpPygWrap(s string) []string {
	var out []string
	for len(s) > mpPygChunk {
		out = append(out, s[:mpPygChunk])
		s = s[mpPygChunk:]
	}
	out = append(out, s)
	return out
}

// mpPygB64Block —— 生成 `mpPyLibB64` 那一段（首行带 `const … = "` 前缀、续行制表符引号、
// 非末行尾随 `" +`）· 与承载件既有形态逐字同（✗ 不带尾换行，由调用方按行拼回）。
func mpPygB64Block(b64 string) string {
	chunks := mpPygWrap(b64)
	var b strings.Builder
	for i, c := range chunks {
		if i == 0 {
			b.WriteString("const mpPyLibB64 = \"")
		} else {
			b.WriteString("\t\"")
		}
		b.WriteString(c)
		b.WriteString("\"")
		if i < len(chunks)-1 {
			b.WriteString(" +")
		}
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// mpPygParseB64 —— 从承载件文本里取出 `mpPyLibB64` 常量的**拼接后**字符串
// （逐行解析 · ✗ 不依赖行号 · 行界定沿用承载件既有形态：非末行以 `" +` 收尾）。
func mpPygParseB64(carry string) (string, bool) {
	lines := strings.Split(carry, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "const mpPyLibB64 = ") {
			start = i
			break
		}
	}
	if start < 0 {
		return "", false
	}
	var b strings.Builder
	for i := start; i < len(lines); i++ {
		ln := lines[i]
		q1 := strings.Index(ln, "\"")
		q2 := strings.LastIndex(ln, "\"")
		if q1 < 0 || q2 <= q1 {
			return "", false
		}
		b.WriteString(ln[q1+1 : q2])
		if !strings.HasSuffix(strings.TrimRight(ln, " \t"), "+") {
			return b.String(), true
		}
	}
	return "", false
}

// mpPygRebuild —— 三格**精确替换**（`mpPyLibSHA256` / `mpPyLibBytesLen` / 整块 `mpPyLibB64`）；
// 其余字节（含抬头注释 · 两个 helper · `mpHelpBody`）逐字节不动。定位不到任一格 ⇒ false。
func mpPygRebuild(carry string, src []byte, sha string, n int) (string, bool) {
	lines := strings.Split(carry, "\n")
	shaIdx, lenIdx, b64Start := -1, -1, -1
	for i, ln := range lines {
		switch {
		case strings.HasPrefix(ln, "const mpPyLibSHA256 = "):
			shaIdx = i
		case strings.HasPrefix(ln, "const mpPyLibBytesLen = "):
			lenIdx = i
		case strings.HasPrefix(ln, "const mpPyLibB64 = "):
			b64Start = i
		}
	}
	if shaIdx < 0 || lenIdx < 0 || b64Start < 0 {
		return "", false
	}
	b64End := -1
	for i := b64Start; i < len(lines); i++ {
		if !strings.HasSuffix(strings.TrimRight(lines[i], " \t"), "+") {
			b64End = i
			break
		}
	}
	if b64End < b64Start {
		return "", false
	}
	b64 := base64.RawURLEncoding.EncodeToString(src)
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		switch i {
		case shaIdx:
			out = append(out, "const mpPyLibSHA256 = \""+sha+"\"")
			i++
		case lenIdx:
			out = append(out, fmt.Sprintf("const mpPyLibBytesLen = %d", n))
			i++
		case b64Start:
			out = append(out, strings.Split(mpPygB64Block(b64), "\n")...)
			i = b64End + 1
		default:
			out = append(out, lines[i])
			i++
		}
	}
	return strings.Join(out, "\n"), true
}
