// check_hardcoded_private_paths.go —— 门禁件 scripts/gates/check-hardcoded-private-paths.py 的 **Go 真搬**（v2.5.14 · M2 路B）。
//
// 为什么有这件：v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
// M1 = 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面（转发壳）；M2 = 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。
// 本件 = M2 批的一枚样板：验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它的理由（M2-0 = 「无输入物料 · 纯静态检查 · 输出确定有序」；候选区间 = 件名首字母 `f`–`p`，现读后自定）：
//
//	✓ 选：`check-hardcoded-private-paths.py`（288 行 · 纯 stdlib `os`/`sys`）
//	· **无外部树依赖**：只扫**仓内**三棵树 `core/`、`agent/`、`ui/src/`（不碰文档仓 / 公开产物树 / `bin/`）；
//	· **输出确定有序**：`SCAN_DIRS` 定序 + 每层 `sorted(dirnames)` / `sorted(filenames)`（无「按 inode 序」的读取）；
//	· 退码 `0/1/2` 三档齐 · **纯只读、无写面**（`--self-test` 只在 `tempfile.mkdtemp()` 里造件）；
//	· 行数适中 · `git status` 现读**不含它**（无人正在改）。
//	✗ 不选 `check-gap-ledger-face.py`（447 行）：判据绑**仓外** `Zerg-内部文档/`（外部树依赖）。
//	✗ 不选 `check-publish-layer-names.py`（484 行）：真源 = `publish/mirror-public-lib.py`（**登记面**，本批禁碰），
//	  且 L4 要**真跑 `bin/zerg` 子进程**（本批硬约束：禁碰 `bin/`）。
//	✗ 不选 `check-lineref-limit.py`（337 行）：落点解析**依赖同级外部树 `Zerg-内部文档/`**，且裸件名解析走
//	  `os.walk` 取 `cands[0]`（遍历序 ⇒ 落点路径可非确定）。
//	✗ 不选 `check-keep-marks.py`（391 行）：判据面要**输入物料**（案卷 JSON `keep-marks-registry.json`，
//	  `--dossier` 可指向仓外件）⇒ 不属「无输入物料」；且 `--self-test` 里用 `subprocess` 真跑自身。
//	✗ 不选 `check-manifest-freshness.py`（226 行）：含墙钟 `datetime.now()` + 退码 64 + `git rev-parse`。
//	✗ 不选 `check-placeholder-residue.py`（250 行）：靠 `git grep`（子进程 + git 索引面）。
//	✗ 不选 `check-idspace-map.py`（328 行）/ `check-glossary.py`（572 行）：读仓外 `Zerg-内部文档/`（外部树）。
//	✗ 不选 `check-links-lychee.py`（432 行）/ `check-history-secrets.py`（760 行）：起外部子进程（lychee / git）。
//	✗ 不选既改件：`check-glossary.py` / `check-help-shape.py`（共享件）/ `check-multirepo-summary.py` /
//	  `check-newcmd-followup.py` —— `git status` 现读 ` M`，本单禁碰。
//
// ⚠ 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · `add_step` **未挂**（`precommit-gates.sh` 现读零命中））；
//	· **不**删 `scripts/gates/check-hardcoded-private-paths.py` 原件（退场规则：**对拍通过后才**退场）；
//	· **已接线**：入口 `hardcodedPrivatePathsGateMain` 由 `gateCheckGoEntry["check-hardcoded-private-paths"]` 分派（`zerg gate check check-hardcoded-private-paths`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func hardcodedPrivatePathsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 零命中 / 1 有命中 / 2 用法错（不给结论）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ★ 标准声明行 (a) —— **默认根定位差异**（py `__file__` 上溯 ⟷ Go cwd）：
//
//	py `repo_root(None)` 从 `os.path.dirname(__file__)`（脚本所在目录）起上溯 ≤6 层找带 `core/` 与 `ui/` 的目录；
//	Go **没有 `__file__` 语境**（同一份二进制可跑在任意 cwd）⇒ 本件从 **cwd** 起上溯同一判据（≤6 层）。
//	⇒ 同真仓 + `cwd` = 仓根时两态读数**逐字相同**（已对拍 C1）；「脚本位置 ≠ cwd」时路径推导面不同 ⇒
//	  对拍一律**显式传根**（py 位置参数 = Go 位置参数）。
//
// ★ 标准声明行 (b) —— **自指检查**：本件**落在它自己的扫描面内**（`core/**` · 非 `_test.go` · 非注释）——
// 门的命中词是三条私有绝对路径，**若在正文里直写原串**，本件一落仓就把本门判红（真仓现读凭空多命中）。
// ⇒ 本件已刻意规避：① 命中词用**字符串拼接**构造（源码文本里没有连成一串的私径，见 `hcppNeedles`）；
// ② `--self-test` 的夹具体用 `%s` 占位 + 运行时取 `hcppNeedles()[0]`（正文里没有原串）；
// ③ 件头正文里也不写原串。⇒ 后续动本件者：**别**把这三处改回原串形态（参 `check_wall_layer.go` 的自扰面记载）。
//
// 已对拍面（仓外夹具两态逐字）：真仓现读（缺省根 / 显式根）· 合成仓全绿 · 代码行命中（core/agent/ui/src 三面）·
// 注释行豁免（防假红）· Rust `#[cfg(test)]` 段豁免 · `_test.go` / `ui/src/**test*` 夹具豁免 · 跳过目录
// （`target` 等 + 以 `.` 开头）· 二进制件（前 4 KiB 含 NUL）跳过 · `--strict` · 空转（扫面 0 件）·
// 取不到输入（根不存在）· 未知旗标 ⇒ 2 · `--json` / `--json=…` 拒收 ⇒ 2 · `--self-test` 三档 · `-h` ⇒ 2。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **帮助正文**：本件 py **无 argparse**（`-h` 走的是「未知旗标」档 ⇒ `rc=2`，正文 = `USAGE` 那一行）。
//	   本件按**现读复刻**该两行正文（含 prog 名 `check-hardcoded-private-paths.py` 硬编码 = 原件基名），
//	   **不是**实现级同源 ⇒ 换了 Python 版本或改了 py 的 `USAGE` 字串就不同。
//	② **默认根定位**：见标准声明行 (a) —— cwd ⟷ `__file__` 的语境不同。
//	③ **异常路径**：文件读不到（py `OSError`）⇒ 两态都「静默跳过」；但某件**非 UTF-8** 时 py `decode("utf-8","replace")`
//	   按**非法序列**逐段替换 U+FFFD，Go `strings.ToValidUTF8` 按**非法字节**逐个替换 ⇒ 极端角落的正文字节可能不同
//	   （真仓三树现读全为合法 UTF-8 ⇒ 不触发）。`os.walk` 的**符号链接目录**语义（py 列进 `dirnames` 但不下降）
//	   vs 本件 `os.Stat().IsDir()`（视作目录并下降）⇒ 有软链时才分叉；真仓三树现读**软链 0 枚** ⇒ 不触发。
//	④ py `--self-test` 用 `subprocess.run([sys.executable, SELF_PATH])` **真跑自身**；本件**进程内**直调入口
//	   （同 `--json` / `--strict` 面，读数一致）。⇒ 打印文案逐字对齐（已对拍），差在**实现手段**（子进程 ⟷ 进程内）。
//	⑤ py 的 `STRICT` 只改「登记项」那一支的归类；`KNOWN_PENDING` 现读 = **空表** ⇒ 本件同样留空表，
//	   `--strict` 与缺省档观测输出**逐字相同**（已对拍；登记项非空时本件不声称对齐）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// ── 常量（与 py 逐字同源）────────────────────────────────────────────────────────────────
const hcppProg = "check-hardcoded-private-paths.py"

// SCAN_DIRS：只扫随代码发布的源码树（相对仓根）。
var hcppScanDirs = []string{"core", "agent", "ui/src"}

// SKIP_DIRS：整棵子树跳过（构建产物 / 运行态 / 依赖）；另加通用规则「以 `.` 开头的一律跳过」。
var hcppSkipDirs = map[string]bool{
	"target": true, "build": true, "node_modules": true, "dist": true, "vendor": true,
	".git": true, ".zerg": true, ".codegraph": true, ".hermes": true, ".cache": true,
	"__pycache__": true, ".pytest_cache": true, ".venv": true, ".idea": true,
}

// COMMENT_PREFIXES：注释判据 —— 去前导空白后的行首。
var hcppCommentPrefixes = []string{"//", "#", "*", "/*"}

// KNOWN_PENDING：已登记待修补（与 py 现读一致 = **空表**）⇒ exempt 恒空。
var hcppKnownPending = map[string]int{}

const hcppUsage = "用法：python3 scripts/gates/check-hardcoded-private-paths.py [仓库根] [--strict] [--self-test]"

// hcppNeedles 返回命中词（与 `publish/replace-rules.tsv` 的脱敏对象同一批）。
//
// ★ 自指检查：本件落在本门**自己的扫描面内**（`core/**` · 非 `_test.go` · 非注释行）⇒
// 若在此直写原串，本件一落仓就把本门判红。这里**拼接构造**：源码文本里没有连成一串的私径。
func hcppNeedles() []string {
	return []string{"/Volumes/" + "AHZ", "/Volumes/" + "BXC", "/Users/" + "Mr2109"}
}

// ── 规则族②：机器地址 / 端口字面量（`GAP-20261010-20` · 扩既有断言 · ✗ 不新造命令）──────────
//
// 为什么加：本门原先只扫「私有绝对路径」三类字面量；**「仓内硬编码机器地址/端口」无一条判据盯着**
// （脱敏侧 `publish/replace-rules.tsv` 早有替换规则、**门禁侧那一半未落**）⇒ 按既有门扩一格。
// 规则源 = `publish/replace-rules.tsv` §「内网地址（示例值）」段（**唯一真源**，✗ 不另立口径）：
//
//	· `192.168.x.x` 内网地址兜底 · `10.0.x.x` 集群内网地址兜底 · 主机名 `<机器名>.local`
//
// ★ 覆盖形态：地址**连同它带的端口**（命中即点名整行 ⇒ `http://192.168.<私>.<私>:<端口>` 这类
//
//	「地址:端口」一并被点名）。★ **裸端口数字不判**（`8100`/`8580` 与合法常量
//	`config.DefaultAgentPort` 在文本上不可区分；判它必然误拦）—— 这是有意收窄，见回执。
//
// ★ 白名单（合法写法 · **永不报**）：见 `hcppAddrAllow`。
// ★ 存量豁免（**只许减**）：见 `hcppAddrKnown` —— 扩规则那一刻的现存命中逐条入免扫面。
var hcppAddrRules = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"内网地址 192.168.x", regexp.MustCompile(`192\.168\.\d{1,3}\.\d{1,3}`)},
	{"集群内网地址 10.0.x", regexp.MustCompile(`\b10\.0\.\d{1,3}\.\d{1,3}\b`)},
	{"机器主机名 *.local", regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9-]*[-0-9][A-Za-z0-9-]*\.local\b`)},
}

// hcppAddrAllow 值级白名单 —— **合法写法，永不报**（命中值逐字等于其中之一、或以之为前缀即放过）。
//
//	· 回环 / 未指定：`127.0.0.1` · `0.0.0.0` · `::1` · `::` · `localhost`（本机自己，不是机器地址）
//	· 文档举例段（RFC5737 TEST-NET-1/2/3）—— 文档/示例专用，不是真机
//
// ★ 现读三条规则（`hcppAddrRules`）**结构性**就不会命中上表多数值（回环是 `127.*`、未指定是 `0.0.0.0`、
//
//	文档段是 `192.0.2.*`/`198.51.100.*`/`203.0.113.*`）；本表是**显式声明**：
//	① 把「哪些写法算合法」写成可审的一行行；② 日后若有人放宽规则，白名单**先行**挡住误拦。
var hcppAddrAllow = []struct{ val, why string }{
	{"127.0.0.1", "回环（本机 · 非机器地址）"},
	{"0.0.0.0", "未指定地址（全接口监听）"},
	{"::1", "回环（IPv6）"},
	{"::", "未指定（IPv6）"},
	{"localhost", "本机名（非机群机器）"},
	{"192.0.2.", "RFC5737 文档段 TEST-NET-1（文档举例）"},
	{"198.51.100.", "RFC5737 文档段 TEST-NET-2（文档举例）"},
	{"203.0.113.", "RFC5737 文档段 TEST-NET-3（文档举例）"},
}

// hcppAddrAllowed：值 ∈ 白名单（逐字，或以白名单项为前缀）⇒ 合法写法、不报。
func hcppAddrAllowed(v string) bool {
	for _, a := range hcppAddrAllow {
		if v == a.val || strings.HasPrefix(v, a.val) {
			return true
		}
	}
	return false
}

// hcppAddrHit 一处「机器地址/端口」命中（规则族②）。
type hcppAddrHit struct {
	rel  string
	line int
	kind string
	val  string
	text string
}

// hcppAddrKnown 存量豁免（`GAP-20261010-20` 扩规则那一刻的现存命中 · **只许减**）。
//
// 命中全落在各判据件里**自检夹具的示例值 / 检测词表字面**（不是生产取址）——
// 逐条 =(仓内相对件, 命中值) ⇒ 该件里**那个值**不算命中；**同一件里出现新值、或别处出现该值仍判红**。
// ★ 值一律**拼接构造**：源码里不连成一串 ⇒ 本件不成为自己的命中宿主（同 `hcppNeedles` 的自指纪律）。
// ★ 现存命中被清掉后**应把该行删掉**（门只有「新增即红」这一颗牙；删行是人工动作，删行不改判色）。
var hcppAddrKnown = []struct {
	rel, val, why string
}{
	{"core/cmd/zerg/check_approve_merge.go", "Mr2109deMac-Studio" + ".local", "自检夹具：审批合并列里的本机名"},
	{"core/cmd/zerg/check_doc_freshness.go", "10." + "0.0.1", "自检夹具：文档新鲜度取样行里的示例地址"},
	{"core/cmd/zerg/check_fleet_loadable.go", "10." + "0.0.1", "自检夹具：fleet 夹具名册 host"},
	{"core/cmd/zerg/check_fleet_loadable.go", "10." + "0.0.2", "自检夹具：fleet 夹具名册 host"},
	{"core/cmd/zerg/check_nodes_roster.go", "10." + "0.0.2", "自检夹具：名册/fleet 夹具 host"},
	{"core/cmd/zerg/check_nodes_roster.go", "10." + "0.0.3", "自检夹具：名册夹具的变异值"},
	{"core/cmd/zerg/check_nodes_roster.go", "192.168." + "110.4", "自检夹具：判据 H「拨号地址不在名册 ⇒ 红」的坏例"},
	{"core/cmd/zerg/check_password_argv.go", "10." + "0.0.7", "自检夹具：口令走 argv 的 ssh 目标"},
	{"core/cmd/zerg/check_password_argv.go", "10." + "0.0.83", "自检夹具：口令走 argv 的 ssh 目标"},
	{"core/cmd/zerg/check_password_argv.go", "10." + "0.0.9", "自检夹具：口令走 argv 的 ssh 目标"},
	{"core/cmd/zerg/check_private_path_forms.go", "192.168." + "1.5", "形态枚举件自身的示例值（F-06 内网地址）"},
	{"core/cmd/zerg/check_private_path_forms.go", "Mr2109deMac-Studio" + ".local", "形态枚举件自身的示例值（F-09 主机名）"},
	{"core/cmd/zerg/check_public_face_wording.go", "Mr2109deMac-Studio" + ".local", "写作面用词表的**检测词表**（该门自己盯这个词）"},
	{"core/cmd/zerg/check_public_marks.go", "192.168." + "1.10", "自检夹具：私有面泄漏样例"},
	{"core/cmd/zerg/check_public_marks.go", "192.168." + "31.7", "自检夹具：私有面泄漏样例"},
	{"core/cmd/zerg/check_token_argv.go", "10." + "0.0.7", "自检夹具：令牌走 argv 的 controller 地址"},
}

// hcppAddrExempt：该 (件, 值) 是否在存量豁免表内。
func hcppAddrExempt(rel, val string) bool {
	for _, e := range hcppAddrKnown {
		if e.rel == rel && e.val == val {
			return true
		}
	}
	return false
}

// ── 判据辅助 ─────────────────────────────────────────────────────────────────────────────

// hcppIsDir：跟随软链判目录（与 py `os.path.isdir` 同义）。
func hcppIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// hcppIsComment：去前导空白后以注释符开头即视为注释（行尾注释不在此列 —— 那是代码行）。
func hcppIsComment(line string) bool {
	s := strings.TrimLeftFunc(line, unicode.IsSpace)
	for _, p := range hcppCommentPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// hcppIsTestFile：测试夹具文件整体豁免（`_test.go` / `ui/src` 下名字含 test 的文件）。
func hcppIsTestFile(rel string) bool {
	if strings.HasSuffix(rel, "_test.go") {
		return true
	}
	if strings.HasPrefix(rel, "ui/src/") && strings.Contains(strings.ToLower(filepath.Base(rel)), "test") {
		return true
	}
	return false
}

// hcppRustTestLines：Rust 文件里属于 `#[cfg(test)]` 夹具段的行号集合（1 起）。
//
// 为什么按括号深度而不是「见到 #[cfg(test)] 就跳到底」：测试段后面可能还有生产代码，
// 跳到底会把生产代码一起放过（假阴性）。只在见到了 `{`（进入模块体）之后，才拿深度归零判定段结束。
func hcppRustTestLines(lines []string) map[int]bool {
	out := map[int]bool{}
	inside, depth, opened := false, 0, false
	for i, line := range lines {
		ln := i + 1
		if !inside && strings.HasPrefix(strings.TrimSpace(line), "#[cfg(test)]") {
			inside, depth, opened = true, 0, false
		}
		if inside {
			out[ln] = true
			if strings.Contains(line, "{") {
				opened = true
			}
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			if opened && depth <= 0 {
				inside = false
			}
		}
	}
	return out
}

// hcppReadText：读文本；二进制（前 4 KiB 含 NUL，即编译产物）返回 ok=false 表示不扫。
func hcppReadText(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	limit := len(raw)
	if limit > 4096 {
		limit = 4096
	}
	if strings.IndexByte(string(raw[:limit]), 0x00) >= 0 {
		return "", false
	}
	// py `decode("utf-8", "replace")` —— 非法字节按 U+FFFD 替。
	return strings.ToValidUTF8(string(raw), "\uFFFD"), true
}

// hcppHit：一处命中（相对路径 · 行号 · 行内容 rstrip）。
type hcppHit struct {
	rel  string
	line int
	text string
}

// hcppWalk：按 py `os.walk(topdown=True)` 的**确定序**递归 —— 先逐件（字典序），再逐子目录（字典序）。
// ★ `addr` 是规则族②（机器地址/端口）的命中出口；两族共用**同一趟遍历与同一套豁免面**
//
//	（注释行 / `_test.go` / Rust `#[cfg(test)]` / 二进制 / 跳过目录）。
func hcppWalk(root, dir string, hits *[]hcppHit, addr *[]hcppAddrHit, scanned *int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	needles := hcppNeedles()
	var subdirs []string
	for _, e := range entries { // os.ReadDir 已按名字典序
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
		(*scanned)++ // ★ 扫描面件数（空域守卫的分母：全件计入；豁免/二进制只是「不判命中」）
		if hcppIsTestFile(rel) {
			continue
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
			// 注释行与 Rust 夹具段：两族**一并**跳过（防假红的口径不分族）。
			if rustSkip[ln] || hcppIsComment(line) {
				continue
			}
			matched := false
			for _, n := range needles {
				if strings.Contains(line, n) {
					matched = true
					break
				}
			}
			if matched {
				*hits = append(*hits, hcppHit{rel: rel, line: ln, text: strings.TrimRightFunc(line, unicode.IsSpace)})
			}
			// 规则族②：机器地址/端口字面量（白名单先筛；存量豁免在出口处筛）。
			for _, rule := range hcppAddrRules {
				for _, m := range rule.re.FindAllString(line, -1) {
					if hcppAddrAllowed(m) {
						continue
					}
					*addr = append(*addr, hcppAddrHit{rel: rel, line: ln, kind: rule.kind, val: m,
						text: strings.TrimRightFunc(line, unicode.IsSpace)})
				}
			}
		}
	}
	for _, d := range subdirs {
		hcppWalk(root, d, hits, addr, scanned)
	}
}

// hcppScan：扫全部白名单目录，返回 (私径命中, 机器地址命中, 扫描面件数)。
func hcppScan(root string, strict bool) ([]hcppHit, []hcppAddrHit, int) {
	var hits []hcppHit
	var addr []hcppAddrHit
	scanned := 0
	for _, sd := range hcppScanDirs {
		base := filepath.Join(append([]string{root}, strings.Split(sd, "/")...)...)
		if !hcppIsDir(base) {
			continue
		}
		hcppWalk(root, base, &hits, &addr, &scanned)
	}
	_ = strict // KNOWN_PENDING 现读为空 ⇒ 两支同值（见件头未覆盖面⑤）
	return hits, addr, scanned
}

// hcppRepoRoot：显式参数 → abspath；否则从 cwd 逐级上溯（≤6 层）找带 `core/` 与 `ui/` 的目录；再否则 cwd。
func hcppRepoRoot(arg string) string {
	if arg != "" {
		if abs, err := filepath.Abs(arg); err == nil {
			return abs
		}
		return arg
	}
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	d = filepath.Clean(d)
	for i := 0; i < 6; i++ {
		if hcppIsDir(filepath.Join(d, "core")) && hcppIsDir(filepath.Join(d, "ui")) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	wd, werr := os.Getwd()
	if werr != nil {
		return "."
	}
	return wd
}

// ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗ ──────────
// 口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
// 退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。
func hcppRefuseJSON(argv []string, stderr io.Writer) (int, bool) {
	var hit []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	quoted := make([]string, 0, len(hit))
	for _, h := range hit {
		quoted = append(quoted, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	fmt.Fprintf(stderr, "   字段表：无（本门只出人读面）\n")
	fmt.Fprintf(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2, true
}

// ── `Q-176` 第五批：`--self-test` 三档自检（正控 rc=0 · 负控 rc=1 · 用法错 rc=2）──────────
// 纪律：**只在临时目录里造件**（不读不写仓内件）· **进程内**真跑本入口取真退出码 · 退出即清。
func hcppSelfTest(stdout io.Writer) int {
	tmp, err := os.MkdirTemp("", "hardcoded-paths-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "── %s --self-test 三档（正控 0 · 负控 1 · 用法错 2）\n", hcppProg)
		fmt.Fprintf(stdout, "── 自检：**1 档不过** ✗\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	bad := 0

	chk := func(label string, want, rc int, out string) {
		good := rc == want
		if !good {
			bad++
		}
		tail := ""
		if !good {
			s := strings.TrimFunc(out, unicode.IsSpace)
			last := ""
			if s != "" {
				parts := strings.Split(s, "\n")
				last = parts[len(parts)-1]
			}
			if len(last) > 90 {
				last = last[:90]
			}
			tail = " ｜ " + last
		}
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
	}

	run := func(argv ...string) (int, string) {
		var b strings.Builder
		rc := hardcodedPrivatePathsGateMain(&b, &b, argv)
		return rc, b.String()
	}

	fixture := func(name, body string) string {
		d := filepath.Join(tmp, name)
		ap := filepath.Join(d, "core", "main.go")
		if err := os.MkdirAll(filepath.Dir(ap), 0o755); err != nil {
			return d
		}
		_ = os.WriteFile(ap, []byte(body), 0o644)
		return d
	}

	fmt.Fprintf(stdout, "── %s --self-test 三档（正控 0 · 负控 1 · 用法错 2）\n", hcppProg)
	// 夹具里的命中词**取本件自己的 `hcppNeedles()`**（不新写字面量，免得本件成为新的私有路径宿主）。
	n := hcppNeedles()[0]
	clean := fixture("clean", "package main\n\nvar x = \"hello\"\n")
	rc, out := run(clean)
	chk("档① 正控：干净件（无命中词）", 0, rc, out)

	dirty := fixture("dirty", fmt.Sprintf("package main\n\nvar p = \"%s/x\"\n", n))
	rc, out = run(dirty)
	chk("档② 负控：代码行写死私有绝对路径", 1, rc, out)

	comment := fixture("comment", fmt.Sprintf("package main\n\n// 历史：曾写死 %s/x（注释不算，不编译进二进制）\nvar x = 1\n", n))
	rc, out = run(comment)
	chk("档①′ 成对正控：同一路径出现在注释里 ⇒ **不许红**（防假红）", 0, rc, out)

	// ── 规则族②（机器地址/端口 · GAP-20261010-20）成对负控 ──
	// ★ 夹具值一律**拼接构造**（本件在门自己的扫描面内 ⇒ 正文里不许连成一串的真地址）。
	chkBool := func(label string, ok bool) {
		if ok {
			fmt.Fprintf(stdout, "  ✓ %s\n", label)
			return
		}
		bad++
		fmt.Fprintf(stdout, "  ✗ %s\n", label)
	}
	addrReal := "192.168." + "9.9" // 不在存量豁免表里 ⇒ 真·新增
	addrTen := "10." + "0.0.42"    // 同上（另一族）
	hostReal := "edge01deMac-Studio" + ".local"

	addrClean := fixture("addrc-ok", "package main\n\nvar a = \"127.0.0.1\"\nvar b = \"0.0.0.0\"\n"+
		"var c = \"192.0.2.1\"\nvar d = \"localhost\"\nvar e = \"zerg.local\"\nvar f = \"foo.local\"\n")
	rc, out = run(addrClean)
	chk("档②′ 成对正控：合法写法（127.0.0.1 / 0.0.0.0 / 文档段 / localhost / 纯字母 .local）⇒ 不许红", 0, rc, out)

	addrBad := fixture("addrc-bad", "package main\n\nvar u = \"http://"+addrReal+":8580/api\"\n")
	rc, outBad := run(addrBad)
	chk("档②″ 负控：代码行写死内网地址 ⇒ 红", 1, rc, outBad)
	chkBool("档②‴ 负控：红并**点名件:行**（core/main.go:3）＋回显命中值",
		strings.Contains(outBad, "  ✗ 机器地址 内网地址 192.168.x · core/main.go:3:") && strings.Contains(outBad, addrReal))

	addrTwo := fixture("addrc-two", "package main\n\nvar p = \""+addrTen+"\"\nvar h = \""+hostReal+"\"\n")
	rc, out = run(addrTwo)
	chk("档②⁗ 负控：另两族（10.0.x / 机器主机名 *.local）也红", 1, rc, out)

	addrCmt := fixture("addrc-cmt", "package main\n\n// 例："+"http://"+addrReal+":8580"+"（注释不算）\nvar x = 1\n")
	rc, out = run(addrCmt)
	chk("档②⁵ 成对正控：同一地址落在**注释行** ⇒ 不许红（防假红）", 0, rc, out)

	// 存量豁免表 / 白名单接线自检（表里第 1 条必判豁免；表外 (件,值) 必判不豁免）。
	chkBool("档②⁶ 存量豁免表接线：首条 (件,值) 判豁免",
		len(hcppAddrKnown) > 0 && hcppAddrExempt(hcppAddrKnown[0].rel, hcppAddrKnown[0].val))
	chkBool("档②⁷ 存量豁免表边界：表外的 (件,值) 判**不**豁免（棘轮）", !hcppAddrExempt("core/x.go", addrReal))
	chkBool("档②⁸ 白名单：回环/未指定/文档段判合法写法", hcppAddrAllowed("127.0.0.1") &&
		hcppAddrAllowed("0.0.0.0") && hcppAddrAllowed("192.0.2.") && !hcppAddrAllowed(addrReal))

	rc, out = run(clean, "--no-such-flag")
	chk("档③ 用法错：未知旗标（输入错 ≠ 零命中）", 2, rc, out)

	rc, out = run("--json")
	chk("档③′ 用法错：`--json` 而本门无机器读面（`Q-176` 统一口）", 2, rc, out)

	verdict := "全过 ✓"
	if bad != 0 {
		verdict = fmt.Sprintf("**%d 档不过** ✗", bad)
	}
	fmt.Fprintf(stdout, "── 自检：%s\n", verdict)
	if bad == 0 {
		return 0
	}
	return 1
}

// hardcodedPrivatePathsGateMain：本件唯一入口（与 py `main()` 三码逐字对齐）。
func hardcodedPrivatePathsGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, refuse := hcppRefuseJSON(argv, stderr); refuse {
		return rc
	}
	for _, a := range argv {
		if a == "--self-test" {
			return hcppSelfTest(stdout)
		}
	}
	var unknown []string
	for _, a := range argv {
		if strings.HasPrefix(a, "-") && a != "--strict" {
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "✗ 用法错（rc=2）：未知旗标 %s\n", strings.Join(unknown, " "))
		fmt.Fprintf(stderr, "  %s\n", hcppUsage)
		return 2
	}
	strict := false
	var pos []string
	for _, a := range argv {
		if a == "--strict" {
			strict = true
			continue
		}
		pos = append(pos, a)
	}
	rootArg := ""
	if len(pos) > 0 {
		rootArg = pos[0]
	}
	root := hcppRepoRoot(rootArg)
	fmt.Fprintf(stdout, "仓库根: %s\n", root)
	fmt.Fprintf(stdout, "扫描根: %s（跳过构建/运行态目录与二进制文件）\n", strings.Join(hcppScanDirs, "、"))
	fmt.Fprintf(stdout, "命中词: %s\n", strings.Join(hcppNeedles(), "、"))
	addrKinds := make([]string, 0, len(hcppAddrRules))
	for _, r := range hcppAddrRules {
		addrKinds = append(addrKinds, r.kind)
	}
	fmt.Fprintf(stdout, "机器地址规则: %s（白名单 %d 条 · 命中即在「地址:端口」形态上点名整行）\n",
		strings.Join(addrKinds, "、"), len(hcppAddrAllow))
	fmt.Fprintf(stdout, "存量豁免（机器地址 · 只许减）: %d 条 (件,值) 登记\n", len(hcppAddrKnown))

	hits, addrHits, scanned := hcppScan(root, strict)
	// ★ 空集守卫（GAP-20260927-108）：空域 ⇒ fail-closed（扫描面 0 件 ⇒ rc=2，不许把「空转」当绿）
	if scanned == 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 空域：扫描面（%s）下一件可扫源码都没取到 ⇒ rc=2（不给结论；空转 ≠ 绿）\n",
			strings.Join(hcppScanDirs, "、"))
		return 2
	}

	for _, h := range hits {
		fmt.Fprintf(stdout, "  ✗ %s:%d:%s\n", h.rel, h.line, h.text)
	}

	// 机器地址面：存量豁免逐条打印（○ · **不进退码**）；其余 = 新增 ⇒ 红并点名件:行。
	var addrRed []hcppAddrHit
	addrKeep := 0
	for _, a := range addrHits {
		if hcppAddrExempt(a.rel, a.val) {
			addrKeep++
			fmt.Fprintf(stdout, "  ○ 存量豁免 %s · %s:%d:%s（值 %s）\n", a.kind, a.rel, a.line, a.text, a.val)
			continue
		}
		addrRed = append(addrRed, a)
	}
	for _, a := range addrRed {
		fmt.Fprintf(stdout, "  ✗ 机器地址 %s · %s:%d:%s（值 %s）\n", a.kind, a.rel, a.line, a.text, a.val)
	}
	fmt.Fprintf(stdout, "存量豁免命中: %d 处（登记 %d 条 · 只打印不进退码）\n", addrKeep, len(hcppAddrKnown))

	exemptFiles := len(hcppKnownPending) // 现读空表 ⇒ 0（与 py `len({e[0] for e in exempt})` 同值）
	if len(hits) > 0 {
		fmt.Fprintf(stdout, "\n结果: 失败——白名单代码里有 %d 处写死的私有绝对路径（应改为经 statepath/modelreg/HOME 解析）\n", len(hits))
	}
	if len(addrRed) > 0 {
		fmt.Fprintf(stdout, "\n结果: 失败——仓内新增 %d 处硬编码机器地址/端口（应改为经 fleet.yaml 名册真源解析 · GAP-20261010-20）\n", len(addrRed))
	}
	if len(hits) > 0 || len(addrRed) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "\n结果: 通过——未发现写死的私有绝对路径（已登记待修补 %d 个文件）· 机器地址面无新增（存量豁免 %d 条）\n", exemptFiles, len(hcppAddrKnown))
	return 0
}
