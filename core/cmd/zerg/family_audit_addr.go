// family_audit_addr.go —— `zerg audit hardcoded-addr`：仓内「硬编码机器地址」的**扇盘清单**
// （只读 · L0 · 零写盘 · 默认不红）。标的缺口 `GAP-20261010-20`。
//
// 为什么**另开一条命令**而不是扩既有判据门（拍定 · 父代理 2026-10-17）：**职责不同** ——
//
//	· 判据门 `check-hardcoded-private-paths` 是**门**（判红用：仓内**新增**一处即 rc=1，存量豁免表只许减）；
//	· 本命令是**扇盘工具**（给人看清单：`件:行:内容` + 总数 + 按目录分组计）。默认 rc=0、不判红；
//	  只有显式 `--fail-on-hit` 才把「命中 > 0」透成 rc=1（供门/CI/收口用）。
//
// ⇒ 判据**口径**不另立第二份：规则表 `hcppAddrRules` 与值级白名单 `hcppAddrAllow` **直接复用**
//
//	同一个包里的判据门那两张表（`check_hardcoded_private_paths.go` 规则族② —— 真源仍是
//	`publish/replace-rules.tsv` §「内网地址（示例值）」）。豁免机制本命令只做一档：**件 + 子串**
//	（豁免件 `scripts/audit-addr-allow.txt` · 一行一条 `路径:子串` · `#` 开头为注释）。
//
// 什么叫「硬编码机器地址」（三族 · 与判据门逐字同源）：
//
//	① `192.168.x.x` 内网地址兜底 · ② 集群内网地址 `10.0.x` 兜底 · ③ 机器主机名 `*.local`
//	形态上「地址**连同它带的端口**」一并被点名（命中即回显整行 ⇒ `http://<地址>:<端口>` 这类整条看得见）。
//	★ **裸端口数字不判**（`8100`/`8580` 与合法常量 `config.DefaultAgentPort` 在文本上不可区分；
//	  判它必然误拦 —— 这是与判据门同一处有意收窄）。
//	★ **回环 / 未指定 / 文档段**（`127.0.0.1` · `0.0.0.0` · `::1` · `localhost` · RFC5737 三段）
//	  是 `hcppAddrAllow` 里声明的**合法写法**（本机自己 / 全接口 / 文档举例，**不是机群机器地址**）
//	  ⇒ 永不报。★ 本命令**不自造**第 4 张规则表或第 2 张白名单（那样两处口径就会开始漂）。
//
// 扫描面（默认）：`core/` · `agent/` · `shared/` · `scripts/` —— **不含** `ui/`（`ui/src` 由判据门管）、
// 依赖树、构建产物与点目录（复用判据门的 `hcppSkipDirs` + 「点开头一律跳过」）。
// 排除：`*_test.go`（夹具地址）· 注释行 · Rust `#[cfg(test)]` 段 · 二进制件（前 4 KiB 含 NUL）。
//
// 退码（真源 `zerg help exit-codes` / `exitcodes.go`）：
//
//	0 = 跑通（**含「有命中」** —— 本命令默认是清单档，不是判据）
//	1 = **只有** `--fail-on-hit` 且命中 > 0 时（供门/CI 用；默认档永不给 1）
//	2 = 用法错（写旗标 / 未知字段 / 豁免件某行不成形）
//	8 = 不给结论（仓根解析不到 / 扫描面 0 件 —— 空转 ≠ 干净）
//
// ★ 本件落在**自己的扫描面内**（`core/**` · 非 `_test.go` · 非注释）⇒ 正文里**不许**出现那三族的字面量：
// 规则与白名单一律经 `hcppAddrRules` / `hcppAddrAllow` 读，不在此复写（后续动本件者请守这一条）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// auditAddrUsage —— 本条命令的**完整用法串**（唯一真源：命令树登记条目与各条用法错路径都读它）。
const auditAddrUsage = "zerg audit hardcoded-addr [--root <仓根>] [--json <字段>] [--fail-on-hit]"

// auditAddrFields —— `--json` 字段**闭集**（登记条目的 `fields` 与本表同源一处）。
var auditAddrFields = []string{"path", "line", "text", "kind", "value", "dir"}

// auditAddrScanDirs —— 默认扫描面（仓根相对）。★ 不含 `ui/`（依赖树 + 前端源码另有判据面）。
var auditAddrScanDirs = []string{"core", "agent", "shared", "scripts"}

// auditAddrAllowRel —— 豁免件（仓内相对）。形如 `路径:子串`，一行一条；`#` 开头为注释。
const auditAddrAllowRel = "scripts/audit-addr-allow.txt"

// auditAddrAllowEntry —— 一条豁免：命中的件 == rel 且该行文本含 sub ⇒ 不算命中。
type auditAddrAllowEntry struct {
	rel   string
	sub   string
	where string // 豁免件里的行号（给人回指）
}

// auditAddrAllowLoad 读豁免件。件不在 ⇒ (nil, "件不在") **不算错**（照实印「0 条」）；
// 某行不成形（没有 `:` / 路径空 / 子串空）⇒ 返回错误 —— 「写坏的豁免行静默不生效」会造出假绿。
func auditAddrAllowLoad(root string) ([]auditAddrAllowEntry, string, error) {
	ap := filepath.Join(root, filepath.FromSlash(auditAddrAllowRel))
	raw, err := os.ReadFile(ap)
	if err != nil {
		return nil, "件不在（0 条豁免）", nil
	}
	var out []auditAddrAllowEntry
	for i, ln := range strings.Split(strings.ToValidUTF8(string(raw), "\uFFFD"), "\n") {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		rel, sub, ok := strings.Cut(s, ":")
		rel, sub = strings.TrimSpace(rel), strings.TrimSpace(sub)
		if !ok || rel == "" || sub == "" {
			return nil, "", fmt.Errorf("%s:%d 不成形（要 `路径:子串`）：%s", auditAddrAllowRel, i+1, s)
		}
		out = append(out, auditAddrAllowEntry{rel: filepath.ToSlash(rel), sub: sub, where: auditAddrAllowRel + ":" + strconv.Itoa(i+1)})
	}
	return out, fmt.Sprintf("%d 条（%s）", len(out), auditAddrAllowRel), nil
}

// auditAddrExemptBy —— 该 (件, 整行文本) 是否被豁免件放过。
func auditAddrExemptBy(allow []auditAddrAllowEntry, rel, text string) (auditAddrAllowEntry, bool) {
	for _, e := range allow {
		if e.rel == rel && strings.Contains(text, e.sub) {
			return e, true
		}
	}
	return auditAddrAllowEntry{}, false
}

// auditAddrHitRow —— 一处命中（人面按 `件:行:内容` 印；机器面按 auditAddrFields 摊格）。
type auditAddrRow struct {
	rel, kind, val, text, dir string
	line                      int
	allow                     string // 命中的豁免行（空 = 未豁免）
}

// auditAddrSweep 扫默认四树。返回：(命中行, 扫描面件数, 豁免条数)。
//
// ★ 遍历序**确定**：`os.ReadDir` 已按名字典序；子目录在件之后下降（同判据门 `hcppWalk`）。
func auditAddrSweep(root string, allow []auditAddrAllowEntry) (rows []auditAddrRow, scanned, exempt int) {
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		var subdirs []string
		for _, e := range entries {
			name := e.Name()
			full := filepath.Join(dir, name)
			if hcppIsDir(full) {
				if hcppSkipDirs[name] || strings.HasPrefix(name, ".") {
					continue
				}
				subdirs = append(subdirs, full)
				continue
			}
			rel, rerr := filepath.Rel(root, full)
			if rerr != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			scanned++
			if hcppIsTestFile(rel) {
				continue // 夹具地址：整件不进清单
			}
			if rel == auditAddrAllowRel {
				continue // 豁免件本身是**登记面**（里面的地址是「要豁免谁」的指针，不是代码）
			}
			text, ok := hcppReadText(full)
			if !ok {
				continue
			}
			lines := strings.Split(text, "\n")
			var rustSkip map[int]bool
			if strings.HasSuffix(rel, ".rs") {
				rustSkip = hcppRustTestLines(lines)
			}
			for i, line := range lines {
				ln := i + 1
				if rustSkip[ln] || hcppIsComment(line) {
					continue
				}
				trimmed := strings.TrimRight(line, " \t\r\u00a0")
				for _, rule := range hcppAddrRules {
					for _, m := range rule.re.FindAllString(line, -1) {
						if hcppAddrAllowed(m) {
							continue // 合法写法：白名单先行挡住误拦
						}
						row := auditAddrRow{rel: rel, line: ln, kind: rule.kind, val: m, text: trimmed,
							dir: filepath.ToSlash(filepath.Dir(rel))}
						if ae, hit := auditAddrExemptBy(allow, rel, trimmed); hit {
							row.allow = ae.where
							exempt++
						}
						rows = append(rows, row)
					}
				}
			}
		}
		for _, d := range subdirs {
			walk(d)
		}
	}
	for _, sd := range auditAddrScanDirs {
		base := filepath.Join(append([]string{root}, strings.Split(sd, "/")...)...)
		if !hcppIsDir(base) {
			continue
		}
		walk(base)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].rel != rows[j].rel {
			return rows[i].rel < rows[j].rel
		}
		if rows[i].line != rows[j].line {
			return rows[i].line < rows[j].line
		}
		return rows[i].kind < rows[j].kind
	})
	return rows, scanned, exempt
}

// cmdAuditHardcodedAddr —— 清单档入口（只读 · 零写盘）。
func cmdAuditHardcodedAddr(inv *invocation, stdout, stderr io.Writer) int {
	// 第一道牙：写旗标一律拒（复用 `family_repo.go` 的写词闭集 —— 不另立第二份）。
	if hits := repoWriteHits(inv.orig); len(hits) > 0 {
		inv.setErr("usage", "write_flag_refused", "本命令是只读扇盘 · 写旗标一律拒")
		fmt.Fprintf(stderr, "%s: 拒（退码 2 · 不给结论）：命令行里出现写旗标 %v\n", progName, hits)
		fmt.Fprintf(stderr, "%s: `audit hardcoded-addr` **纯只读**（不写件、不起进程、不改仓内状态）· 用法 `%s`\n",
			progName, auditAddrUsage)
		return exitUsage
	}
	root := ""
	for _, r := range inv.flagVals("--root") {
		if s := strings.TrimSpace(r); s != "" {
			root = s
			break
		}
	}
	if root == "" {
		root = repoRoot()
	}
	if root == "" || !hcppIsDir(root) {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 扫不了（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "%s: 在仓内跑，或 `--root <仓根>`\n", progName)
		return exitBlocked
	}

	allow, allowWord, aerr := auditAddrAllowLoad(root)
	if aerr != nil {
		inv.setErr("usage", "allow_line_malformed", aerr.Error())
		fmt.Fprintf(stderr, "%s: 豁免件不成形 ⇒ 拒跑（退码 2 · 不给结论）：%v\n", progName, aerr)
		fmt.Fprintf(stderr, "%s: 形状 = 一行一条 `路径:子串`（`#` 开头为注释）；写坏的行会静默不生效 = 假绿\n", progName)
		return exitUsage
	}

	rules := make([]string, 0, len(hcppAddrRules))
	for _, r := range hcppAddrRules {
		rules = append(rules, r.kind)
	}
	fmt.Fprintf(stderr, "%s: 仓根 %s\n", progName, root)
	fmt.Fprintf(stderr, "%s: 扫描面 %s（跳过构建/运行态目录与点目录 · 排除 `*_test.go` · 注释行 · Rust 夹具段 · 二进制件）\n",
		progName, strings.Join(auditAddrScanDirs, "、"))
	fmt.Fprintf(stderr, "%s: 规则 %s（值白名单 %d 条 · 裸端口数字不判）\n", progName,
		strings.Join(rules, "、"), len(hcppAddrAllow))
	fmt.Fprintf(stderr, "%s: 豁免件 %s · %s\n", progName, auditAddrAllowRel, allowWord)

	rows, scanned, exempt := auditAddrSweep(root, allow)
	if scanned == 0 {
		inv.setErr("blocked", "empty_scan_face", "扫描面 0 件")
		fmt.Fprintf(stderr, "%s: **扫描面 0 件 ⇒ 判不了**（不给结论 · 退码 8）—— 「真的没有可扫源码」与「目录读不到」两态不许同形\n", progName)
		return exitBlocked
	}

	jsonRows := make([]map[string]string, 0, len(rows))
	live := 0
	dirCount := map[string]int{}
	dirFiles := map[string]map[string]bool{}
	for _, r := range rows {
		if r.allow != "" {
			// 豁免件放过的：**不进清单**（stdout / `--json` items 里都没有它）—— 这就是「白名单生效」的形态；
			// 但**照实印在 stderr 的旁栏**（谁被放过 · 由豁免件哪一行放过），✗ 不静默吞。
			fmt.Fprintf(stderr, "%s: ○ 豁免放过 %s:%d:%s（%s · %s）\n", progName, r.rel, r.line, r.text, r.val, r.allow)
			continue
		}
		live++
		dirCount[r.dir]++
		if dirFiles[r.dir] == nil {
			dirFiles[r.dir] = map[string]bool{}
		}
		dirFiles[r.dir][r.rel] = true
		jsonRows = append(jsonRows, map[string]string{
			"path":  r.rel,
			"line":  strconv.Itoa(r.line),
			"text":  r.text,
			"kind":  r.kind,
			"value": r.val,
			"dir":   r.dir,
		})
	}

	if inv.jsonGiven {
		if rc := listCmd(inv, stdout, stderr, auditAddrFields, jsonRows); rc != exitOK {
			return rc
		}
	} else {
		if live == 0 {
			fmt.Fprintf(stdout, "  （无命中）\n")
		}
		for _, r := range rows {
			if r.allow != "" {
				continue // 豁免的只在 stderr 旁栏出现
			}
			fmt.Fprintf(stdout, "%s:%d:%s\n", r.rel, r.line, r.text)
		}
	}

	// 分组计（按**所在目录** · 只计未豁免命中）＋ 总数。
	fmt.Fprintf(stderr, "%s: 命中 **%d** 处 · 件 **%d** 件 · 扫描 %d 件 · 豁免放过 %d 处\n",
		progName, live, auditAddrCountFiles(dirFiles), scanned, exempt)
	dirs := make([]string, 0, len(dirCount))
	for d := range dirCount {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		if dirCount[dirs[i]] != dirCount[dirs[j]] {
			return dirCount[dirs[i]] > dirCount[dirs[j]]
		}
		return dirs[i] < dirs[j]
	})
	if len(dirs) > 0 {
		fmt.Fprintf(stderr, "%s: 按目录分组计（处 · 件）：\n", progName)
	}
	for _, d := range dirs {
		fmt.Fprintf(stderr, "%s:   %s → %d 处 · %d 件\n", progName, d, dirCount[d], len(dirFiles[d]))
	}

	failOnHit := inv.hasFlag("--fail-on-hit")
	switch {
	case live > 0 && failOnHit:
		fmt.Fprintf(stderr, "%s: `--fail-on-hit` 档：命中 %d 处 > 0 ⇒ 退码 1（供门/CI 用）\n", progName, live)
		return 1
	case live > 0:
		fmt.Fprintf(stderr, "%s: 清单档（默认）：有命中**仍退 0** —— 这是扇盘工具，判红是判据门 `check-hardcoded-private-paths` 的活儿；要把它当闸用就加 `--fail-on-hit`\n", progName)
	default:
		fmt.Fprintf(stderr, "%s: 命中 0 处 —— 默认扫描面里没有「硬编码机器地址」字面量（豁免放过 %d 处）\n", progName, exempt)
	}
	return exitOK
}

// auditAddrCountFiles —— 分组里的件数合计（**去重**：同一件落两组不可能，这里只做一次跨组去重兜底）。
func auditAddrCountFiles(dirFiles map[string]map[string]bool) int {
	seen := map[string]bool{}
	for _, fs := range dirFiles {
		for f := range fs {
			seen[f] = true
		}
	}
	return len(seen)
}
