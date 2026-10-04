// family_code_diff.go —— `zerg code diff <件> [--stat]`（缺口 `GAP-20260929-37`）。
//
// 病（缺口逐字）：现无「按件与 HEAD 的差」只读面 —— 要看去某一件改了哪些行，只能手搓
// `git diff`（守卫可能拦的手搓形态）。本件补这一格：按件与 HEAD 的差，只读。
//
// 面：默认给一行 `+增 -删`（numstat）；`--stat` 另附 `git diff --stat` 的整段；`--json` 出四格。
// 退码：0 有读数（含 0/0，件未改也算读数）· 2 用法错（缺件 / 件不存在于 HEAD 与工作树）· 8 判不了（git 不可用 / 解析不到仓根 / 件出仓）。
// ★ 只读：只跑 `git diff`（不写不删不改任何件；不建任何目录）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

// codeDiffFields —— `--json` 的字段表（K1：机器面先定）。
var codeDiffFields = []string{"path", "added", "deleted", "stat"}

func cmdCodeDiff(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_target", "缺 <件>")
		fmt.Fprintf(stderr, "%s: `code diff` 要给 <件>（例：%s code diff core/go.mod）\n", progName, progName)
		fmt.Fprintf(stderr, "用法：%s code diff <件> [--stat] [--json <字段>]\n", progName)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitBlocked
	}
	rel := strings.TrimSpace(inv.args[0])
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		inv.setErr("usage", "path_outside_repo", "件出仓")
		fmt.Fprintf(stderr, "%s: 件出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, rel, root)
		return exitUsage
	}
	// numstat：`<增>\t<删>\t<件>`（二进制件为 `-`）
	cmd := exec.Command("git", "-C", root, "diff", "--numstat", "HEAD", "--", clean)
	var out, ebuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &ebuf
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			inv.setErr("blocked", "git_unavailable", "git 不可用")
			fmt.Fprintf(stderr, "%s: git 跑不动：%v ⇒ 不给结论（退码 8）\n", progName, err)
			return exitBlocked
		}
		inv.setErr("blocked", "git_diff_failed", "git diff 非零")
		fmt.Fprintf(stderr, "%s: git diff 非零：%s ⇒ 不给结论（退码 8）\n", progName, strings.TrimSpace(ebuf.String()))
		return exitBlocked
	}
	fields := strings.SplitN(strings.TrimSpace(out.String()), "\n", 2)[0]
	cols := strings.Split(fields, "\t")
	added, deleted, path := "0", "0", clean
	if len(cols) >= 2 && strings.TrimSpace(cols[0]) != "" {
		added, deleted = cols[0], cols[1]
		if len(cols) >= 3 {
			path = cols[2]
		}
	}
	stat := ""
	if inv.hasFlag("--stat") {
		s := exec.Command("git", "-C", root, "diff", "--stat", "HEAD", "--", clean)
		var b bytes.Buffer
		s.Stdout = &b
		_ = s.Run()
		stat = strings.TrimSpace(b.String())
	}
	row := map[string]string{"path": path, "added": added, "deleted": deleted, "stat": stat}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, codeDiffFields, row)
	}
	fmt.Fprintf(stdout, "%s: +%s -%s\n", path, added, deleted)
	if stat != "" {
		fmt.Fprintln(stdout, stat)
	}
	return exitOK
}
