package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ── G6：Go 工具链钉住 ────────────────────────────────────────────────────────
//
// 为什么必须钉：源码式更新 = **每台机器自己编**。若两台机器的 go 版本不同，
// 同一个 commit 会编出两个不同的二进制（优化/内联/标准库都在变），
// 「同 commit + 同平台 ⇒ 同 sha256」这条断言会静默失效，混版就查不出来。
//
// 纪律（四条）：
//  1. 单一真源 = `core/go.mod` 的 **`go` 指令**（如 `go 1.25.5`）。
//     ⚠️ 实测坑（2026-09-13）：`toolchain` 指令与同文件 `go` 指令**同值**时，go 命令会
//     把它当冗余**直接删掉**（构建期间改写 go.mod）⇒ "钉住"在文件里留不住。
//     故本仓把 pin 放在 go 指令上；`toolchain` 只在需要更高版本时显式写（两模块必须同值）。
//  2. `ZERG_GO_TOOLCHAIN_PIN` 可覆盖（**测试/应急**，不进正常路径）。
//  3. 构建一律 `GOTOOLCHAIN=local`：绝不让 go 在构建**中途**偷偷下载另一个工具链
//     （那样本机构建就不再可复现；`ZERG_GOTOOLCHAIN` 是显式逃生门）。
//  4. 本地工具链 **低于** 钉住值 ⇒ 明确报错、不构建（宁可不动，也不装一个身份可疑的件）。
//
// 钉住值本身也要能"被核对"：manifest 里同时写 pin 与 actual，`--fleet` 收进机群回执。
// 单一真源的机器可读断言见 scripts/check-gotoolchain.py（CI 门禁）。

const goToolchainDefaultPin = "go1.25.5"

var (
	toolchainRe   = regexp.MustCompile(`(?m)^toolchain[ \t]+(go[\d.]+)[ \t]*$`)
	goDirectiveRe = regexp.MustCompile(`(?m)^go[ \t]+([\d.]+)[ \t]*$`)
	goVerRe       = regexp.MustCompile(`^go(\d+)\.(\d+)(?:\.(\d+))?`)
)

// ToolchainPin —— 钉住的 Go 工具链版本（如 "go1.25.5"）。读不到 go.mod 时用内置默认值。
func ToolchainPin(root string) string {
	if v := strings.TrimSpace(os.Getenv("ZERG_GO_TOOLCHAIN_PIN")); v != "" {
		return v
	}
	if b, err := os.ReadFile(filepath.Join(root, "core", "go.mod")); err == nil {
		if m := toolchainRe.FindSubmatch(b); len(m) == 2 {
			return string(m[1])
		}
		if m := goDirectiveRe.FindSubmatch(b); len(m) == 2 {
			return "go" + string(m[1])
		}
	}
	return goToolchainDefaultPin
}

// parseGoVersion —— 解析 "go1.25.5" / "1.25.5" / "go1.26" 为三段数字。
func parseGoVersion(s string) ([3]int, bool) {
	var out [3]int
	m := goVerRe.FindStringSubmatch(strings.TrimSpace(s))
	if len(m) == 0 {
		return out, false
	}
	for i := 1; i <= 3; i++ {
		if m[i] == "" {
			continue
		}
		n := 0
		for _, c := range m[i] {
			n = n*10 + int(c-'0')
		}
		out[i-1] = n
	}
	return out, true
}

// CompareGoVersions —— 语义比较（**不能**用字符串比较："go1.9" > "go1.26" 会判错）。
// 返回 -1 / 0 / 1；任一侧解析不了返回 2（判不出）。
func CompareGoVersions(a, b string) int {
	va, ok1 := parseGoVersion(a)
	vb, ok2 := parseGoVersion(b)
	if !ok1 || !ok2 {
		return 2
	}
	for i := 0; i < 3; i++ {
		if va[i] != vb[i] {
			if va[i] < vb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// LocalGoVersion —— 本机 `go version` 报的版本（"go1.26.4"）。
func LocalGoVersion() (string, error) {
	out, err := exec.Command("go", "version").Output()
	if err != nil {
		return "", fmt.Errorf("读不到本机 go 版本（go version 失败：%w）", err)
	}
	f := strings.Fields(strings.TrimSpace(string(out)))
	if len(f) < 3 {
		return "", fmt.Errorf("go version 输出无法解析：%q", strings.TrimSpace(string(out)))
	}
	return f[2], nil
}

// VerifyToolchain —— 核对本机工具链是否满足钉住值。
// 返回 (pin, actual, err)；actual 高于或等于 pin ⇒ err == nil。
func VerifyToolchain(root string) (pin, actual string, err error) {
	pin = ToolchainPin(root)
	actual, verr := LocalGoVersion()
	if verr != nil {
		return pin, "", verr
	}
	switch CompareGoVersions(actual, pin) {
	case 0, 1:
		return pin, actual, nil
	case -1:
		return pin, actual, fmt.Errorf(
			"本机 Go 工具链 %s 低于钉住值 %s（core/go.mod 的 go 指令——单一真源）——拒绝构建；"+
				"请升级该节点的 Go，或显式改钉住值（不要静默降级：那样两台机器会编出不同二进制）", actual, pin)
	default:
		return pin, actual, fmt.Errorf("无法比较本机 Go 版本 %q 与钉住值 %q", actual, pin)
	}
}

// goToolchainEnv —— 构建时强制给 go 的环境：GOTOOLCHAIN=local（不让 go 中途换工具链）。
func goToolchainEnv() []string {
	policy := strings.TrimSpace(os.Getenv("ZERG_GOTOOLCHAIN"))
	if policy == "" {
		policy = "local"
	}
	return []string{"GOTOOLCHAIN=" + policy}
}
