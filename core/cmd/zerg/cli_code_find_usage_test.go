// cli_code_find_usage_test.go —— 缺口 `GAP-20260927-301` 的真二进制判据：
// `code find` 的**两处兜底用法串**（缺正则 / 聚合档互斥）必须**复用树真源**
// （命令树里 `code find` 那一条的 `usage`），✗ 不许各写一份字面量。
//
// 判据（主判据 · 逐字节）：运行期 stderr 里那行 `用法：…` 去掉前缀后，与命令树 usage **逐字节相同**。
// 两处各判一次：① 缺正则 ⇒ `family_code.go` 那一处兜底串；② `--count --files-only` 同给 ⇒ 另一处。
//
// ★ 本组从源码现编（`zergBinaryFromSource`）—— 本单硬约束禁重编 `bin/zerg`；走旧制品会把
//
//	「本面未落制品」误读成「本面不存在」（同法见 `cli_code_find_test.go` 的
//	`TestCodeFind_BackslashPipeIsNotAlternation`）。
package main_test

import (
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// TestCodeFind_RuntimeUsageReusesTreeSource —— 缺口 `GAP-20260927-301` 的真判据（主判据 = 逐字节）。
func TestCodeFind_RuntimeUsageReusesTreeSource(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := syntheticRepo(t, "exit 0\n")

	// 树真源 = 命令树里 `code find` 那一条的 usage（经只读桥现读 —— 与 `help code` / 导出面同一格）。
	info, ok := zerg.CommandInfoOfForTest("code find")
	if !ok {
		t.Fatal("命令树里没有 `code find` ⇒ 树真源取不到（判不了，不当绿）")
	}
	if strings.TrimSpace(info.Usage) == "" {
		t.Fatal("命令树里 `code find` 的 usage 是空的 ⇒ 树真源取不到（判不了，不当绿）")
	}
	want := "用法：" + info.Usage

	usageLine := func(s string) string {
		for _, ln := range strings.Split(s, "\n") {
			if strings.HasPrefix(ln, "用法：") {
				return ln
			}
		}
		return ""
	}

	// ① 缺正则 ⇒ 走 `family_code.go` 那一处兜底串。
	rc, _, errb := execCase(t, bin, root, "code", "find")
	if rc != 2 {
		t.Fatalf("缺正则 ⇒ 退 2，得到 %d · stderr=%q", rc, errb)
	}
	if got := usageLine(errb); got != want {
		t.Errorf("兜底用法串（缺正则那处）与树真源不同形：\n  运行期：%q\n  树真源：%q", got, want)
	}

	// ② `--count` 与 `--files-only` 同给 ⇒ 走另一处兜底串。
	rc, _, errb = execCase(t, bin, root, "code", "find", "needle", "--count", "--files-only")
	if rc != 2 {
		t.Fatalf("`--count --files-only` 同给 ⇒ 退 2，得到 %d · stderr=%q", rc, errb)
	}
	if got := usageLine(errb); got != want {
		t.Errorf("兜底用法串（聚合档互斥那处）与树真源不同形：\n  运行期：%q\n  树真源：%q", got, want)
	}
}
