// family_build_show.go —— `zerg build show`（§一 A3/A5 · 缺口-命令面-20260921 §十一 P0-6）。
//
// 为什么它排 P0：「换件后必须验**在跑的件 == 盘上件**」这条血泪判据今天**机器判不了** ——
// `build ls` 给 sha256 但**不给 mtime 与 inode**，记录里靠手敲 `stat -f` + `lsof -p <pid>` 看 inode。
// 命令化之后：换件档的就绪判据可机械化、`dev release`/`dev rollback` 的「换上了没」有真判据、
// `E2 验签` 与它共用同一份件身份。
//
// 口径（照 §十一 P0-6 的形态，不自造）：
//
//	形态 `zerg build show <件> | --all [--json <字段>]`
//	输出 `name/sha256/bytes/mtime/inode/type/arch/签名态`
//	退码 `0` / `1` 件不存在 / `2` 用法错
//
// 三条真源各归各的（**不自己造第二套**）：身份 = `file`（type/arch）· 签名 = `codesign` ·
// 文件系统 = `stat`（mtime/inode）· 内容 = sha256（内存现算，与 `build ls` 同一算法）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

// artifactArchRE —— `file` 输出尾部的架构词（认得出就单列 `arch`，认不出留原串、不猜）。
var artifactArchRE = regexp.MustCompile(`\b(arm64e|arm64|x86_64|i386|ppc64le|riscv64)\b`)

func cmdBuildShow(inv *invocation, stdout, stderr io.Writer) int {
	all := inv.all
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if !all && name == "" {
		inv.setErr("usage", "missing_artifact", "缺件名")
		fmt.Fprintf(stderr, "%s: `build show` 要给件名（例：zerg build show zerg-core），或 `--all`\n", progName)
		return exitUsage
	}
	if all && name != "" {
		inv.setErr("usage", "artifact_and_all", "件名与 --all 互斥")
		fmt.Fprintf(stderr, "%s: 件名与 `--all` 互斥（二选一 · 退码 2）\n", progName)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 找不到 bin/（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}

	var targets []string
	if all {
		binDir := filepath.Join(root, "bin")
		ents, err := os.ReadDir(binDir)
		if err != nil {
			inv.setErr("blocked", "bin_absent", "bin/ 读不到")
			fmt.Fprintf(stderr, "%s: `bin/` 读不到（%v）⇒ 不给结论（退码 8）\n", progName, err)
			return exitBlocked
		}
		for _, e := range ents {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			p := filepath.Join(binDir, e.Name())
			if fi, err := os.Stat(p); err == nil && fi.Mode()&0o111 != 0 {
				targets = append(targets, p)
			}
		}
		sort.Strings(targets)
	} else {
		p := name
		if !strings.Contains(name, "/") {
			p = filepath.Join(root, "bin", name)
		}
		targets = []string{p}
	}

	rows := []map[string]string{}
	missing := []string{}
	for _, p := range targets {
		fi, err := os.Stat(p)
		if err != nil {
			missing = append(missing, p)
			continue
		}
		if fi.IsDir() {
			missing = append(missing, p+"（是目录）")
			continue
		}
		inode := "（读不到）"
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			inode = fmt.Sprintf("%d", st.Ino)
		}
		ftype, arch := fileTypeArch(p)
		// `name` 一律给**件名**（`bin/` 下的基名）；用户给的是路径（含 `/`）就照抄路径 ——
		// 「这一行是哪一件」不许让人从绝对路径里自己抠。
		rowName := filepath.Base(p)
		if name != "" && strings.Contains(name, "/") {
			rowName = p
		}
		rows = append(rows, map[string]string{
			"name":   rowName,
			"sha256": fileSHA256(p),
			"bytes":  fmt.Sprintf("%d", fi.Size()),
			"mtime":  fi.ModTime().Format(time.RFC3339),
			"inode":  inode,
			"type":   ftype,
			"arch":   arch,
			"signed": codeSignState(p),
		})
	}
	if len(rows) == 0 {
		inv.setErr("failed", "artifact_absent", "件不存在")
		fmt.Fprintf(stderr, "%s: 件不存在 ⇒ 退 1（**不是错**：本机没有这件）· 找不到：%v\n", progName, missing)
		fmt.Fprintf(stderr, "下一步：`zerg build ls` 列现存的件，或 `zerg build show --all`\n")
		return exitFail
	}
	fmt.Fprintf(stderr, "%s: 逐件身份现读（`file`/`codesign`/`stat` 各是真源；sha256 内存现算 · 不缓存）\n", progName)
	for _, m := range missing {
		fmt.Fprintf(stderr, "%s: 找不到：%s（退 1 —— 与本机没有这件同义）\n", progName, m)
	}
	// CLI 收尾（工作日志的教训）：**指定名查不到就不再顺路输出别的件** —— 退 1 的含义是
	// 「没有你要的那一件」，而「--all 里有几件」这件事与它无关。
	if rc := listCmd(inv, stdout, stderr,
		[]string{"name", "sha256", "bytes", "mtime", "inode", "type", "arch", "signed"}, rows); rc != exitOK {
		return rc
	}
	if len(missing) > 0 {
		return exitFail
	}
	return exitOK
}

// fileTypeArch —— 文件类型与架构（真源 = `file -b`；认不出的架构留给人读的原串，**不猜**）。
func fileTypeArch(p string) (ftype, arch string) {
	if _, err := exec.LookPath("file"); err != nil {
		return "（本机没有 file）", "（读不到）"
	}
	out, err := exec.Command("file", "-b", p).Output()
	if err != nil {
		return "（读不到）", "（读不到）"
	}
	s := strings.TrimSpace(string(out))
	arch = "（未标出）"
	if m := artifactArchRE.FindStringSubmatch(s); m != nil {
		arch = m[1]
	}
	return s, arch
}

// codeSignState —— 签名态（真源 = `codesign`）。**四种情形分开报**，不许把读不到当通过：
// 通过 / 不通过（带原文首行）/ 这件根本没签 / 本机没 codesign（读不到）。
func codeSignState(p string) string {
	if _, err := exec.LookPath("codesign"); err != nil {
		return "读不到（本机没有 codesign）"
	}
	if out, err := exec.Command("codesign", "--verify", "--verbose=1", p).CombinedOutput(); err != nil {
		s := strings.TrimSpace(string(out))
		if strings.Contains(s, "not signed at all") {
			return "无签名（codesign：code object is not signed at all）"
		}
		first := s
		if i := strings.Index(first, "\n"); i >= 0 {
			first = first[:i]
		}
		return "不通过（" + first + "）"
	}
	signer := ""
	if out, err := exec.Command("codesign", "-dv", p).CombinedOutput(); err == nil {
		for _, ln := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(strings.TrimSpace(ln), "Authority=") {
				signer = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ln), "Authority="))
				break
			}
		}
	}
	if signer == "" {
		return "通过（签名者读不到）"
	}
	return "通过（" + signer + "）"
}
