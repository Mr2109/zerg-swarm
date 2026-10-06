// decl_label.go —— 件 scripts/svc/decl-label.sh（76 行 · sha16 15ae896d85eaa680）的 Go 真搬
// （非 gates 面 · 零 py/sh 普查表 #64 · 入口 declLabelGateMain）。
//
// 为什么有这件：普查表 #64 现读为「静态只读（声明件）」——
//
//	· 形态 = 纯静态只读：只以 r 模式读一枚 tsv（<仓根>/deploy/服务声明.tsv）并 print；
//	          零 subprocess / 零联网 / 零真 TTY / 零写面；
//	· 退码 = 恰 0/1/2（命中恰好一条=0 · 命中数不为 1=1 · 用法错或缺声明件=2）。
//
// ⇒ 属普查表里现读**零 subprocess** 的纯静态只读件，适格 M2-0 搬运（同档另有 compare.py 已由
// eval_compare.go 真搬 ⇒ 本件不重选；见回执的候选排查表）。
//
// 本件真身速览（逐处现读 scripts/svc/decl-label.sh 现值 · 非按件名推断）：
//
//	· 参数面（手写 case，非 argparse ⇒ 无缩写面）：
//	    --decl <件>（其后无参 ⇒ usage+rc2）· --all · -h|--help（usage+rc0）· --
//	    （其后参数一律忽略）· 其余以 `-` 起头者（含裸 `-`）⇒ usage+rc2 · 其它 ⇒ 末者胜作查询。
//	· 声明件解析面 = `while IFS=$'\t' read -r k n decl prog proc owner disp rest`：
//	    ★ bash 的 IFS 仅含制表符（属 whitespace IFS）⇒ 行首/行尾制表符被剥、连续制表符折叠成
//	      单个分隔 ⇒ 本文所用三列 = 列0 kind · 列1 name · 列4 proc（>8 列的余项进末变量、本件不读）；
//	    ★ 只处理**以 \n 结尾**的行 —— 末行无换行时 bash read 于 EOF 返非零、循环体不跑 ⇒ 该行**丢弃**；
//	    ★ 不做 \r 折叠（bash read -r 不折叠 CR）；空行/注释行/表头行的 kind 自然不等于 launchd ⇒ 跳过。
//	· 判面 = kind=="launchd" 且（name==查询 或 proc==查询）⇒ 收 name（**按文件行序**、不排序）。
//	· 输出面：
//	    --all：命中≥1 ⇒ stdout = 各 name 逐行连写、rc0；命中 0 ⇒ stderr「零命中」、rc1。
//	    非 --all：命中==1 ⇒ stdout = name、rc0；否则 stderr「命中 N 条」（0 与 >1 走同一句）、rc1。
//
// 出厂声明 (a) 默认根定位差异：原件以 `dirname($BASH_SOURCE)/../..` 定仓根（脚本位置上溯 · 物理路径）；
// 本 Go 件无 __file__ 语境 ⇒ 走同包 repoRoot()（ZERG_REPO → 可执行件目录上溯 → cwd 上溯，判据 =
// 仓根有 core/internal/version/version.go）⇒「脚本位置 ≠ cwd/可执行件」时默认根不同（✗ 不声称逐字对齐）。
// ★ 显式 --decl 或 env ZERG_SVCDECL_DECL 时两侧同值、该差异面**不适用**。
//
// 出厂声明 (b) 自指检查：本件确落在 core/cmd/zerg/*.go 全件扫描面内 ⇒ 逐条消解：
//
//	· 零私有绝对路径字面量（只写 `deploy/服务声明.tsv` 这类仓根相对串）；
//	· 零「引号 + 协议键名 + 引号」的连续原串（本件不写 JSON 键名；kind 面只与字面值比较）；
//	· 零会悬空的仓内相对路径字面量（唯一仓内相对串 = deploy/服务声明.tsv，真仓在盘）；
//	· 复核 0/0（self-ref sweep 见回执）。
//
// 本单边界（照实）：只落码、不接线 ——
//
//	· 不在 core/cmd/zerg/main.go 注册命令（✗ 不进命令树 · ✗ 不改 gate_inproc_table.go）；
//	· 不删 scripts/svc/decl-label.sh 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 declLabelGateMain 目前无调用点（M2 的刻意状态）；
//	· ✗ 不改任何登记面 / family_*.go / 已在仓旧件 / scripts/**。
//
// 入口签名（与原件 `$@` 对齐）：
//
//	func declLabelGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与原件位置参数同形（不含脚本名）
//	  · 退码  —— 0 命中恰好一条 · 1 命中数不为 1 · 2 用法错 / 声明件不存在
//	  · 输出  —— stdout/stderr 与原件逐字节对齐（见「已对拍面 / 未覆盖面」）
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// dlKindLaunchd = 声明件只认的 kind 字面（原件 `[ "$_kind" = "launchd" ]`）。
// ★ 拆成具名常量，避免件内出现「引号 + 键名 + 引号」式连续原串（自指消解 ②）。
const dlKindLaunchd = "launchd"

// dlUsage 复刻原件 usage()（`cat >&2 <<'USAGE' … USAGE` 的逐字正文 + 尾换行）。
const dlUsage = "用法: decl-label.sh <查询> [--decl <声明件>] [--all]\n" +
	"  <查询> 精确匹配声明件第 2 列(name) 或 第 5 列(进程特征)，印出第 2 列 label。\n" +
	"  --all  列出全部命中（默认要求恰好 1 条）。\n"

// dlSplitIFSTab 照 bash `IFS=$'\t' read` 的字段切分：
// IFS 仅含制表符（属 whitespace IFS）⇒ 行首/行尾制表符被剥、连续制表符折叠为单个分隔、整行制表符 = 零字段。
// ★ 按**字节**切分：制表符 0x09 不会出现在 UTF-8 多字节序列内部 ⇒ 安全。
func dlSplitIFSTab(line string) []string {
	line = strings.Trim(line, "\t")
	if line == "" {
		return nil
	}
	fields := []string{}
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] == '\t' {
			fields = append(fields, b.String())
			b.Reset()
			for i+1 < len(line) && line[i+1] == '\t' {
				i++
			}
			continue
		}
		b.WriteByte(line[i])
	}
	fields = append(fields, b.String())
	return fields
}

// dlField 取第 i 列；越界 ⇒ 空串（与 bash 未赋值的变量同形）。
func dlField(fields []string, i int) string {
	if i < len(fields) {
		return fields[i]
	}
	return ""
}

// dlScan 逐行解析并收集命中 name（**文件行序**）。返回 (names, ok)。
// ★ 只处理以 \n 结尾的行 —— 末行无换行 ⇒ 丢弃（bash `read` 于 EOF 返非零、循环体不跑）。
func dlScan(content, query string) []string {
	out := []string{}
	rest := content
	for {
		idx := strings.IndexByte(rest, '\n')
		if idx < 0 {
			break
		}
		line := rest[:idx]
		rest = rest[idx+1:]
		f := dlSplitIFSTab(line)
		if dlField(f, 0) != dlKindLaunchd {
			continue
		}
		name := dlField(f, 1)
		if name == query || dlField(f, 4) == query {
			out = append(out, name)
		}
	}
	return out
}

// declLabelGateMain = 件入口（argv 与原件位置参数同形）。
func declLabelGateMain(stdout, stderr io.Writer, argv []string) int {
	decl := os.Getenv("ZERG_SVCDECL_DECL")
	if decl == "" {
		// 原件缺省 = `$REPO_ROOT/deploy/服务声明.tsv`（REPO_ROOT 由脚本位置上溯；Go 侧走 repoRoot()，见出厂声明 (a)）。
		decl = repoRoot() + "/deploy/服务声明.tsv"
	}
	query := ""
	all := false

parseLoop:
	for i := 0; i < len(argv); {
		a := argv[i]
		switch {
		case a == "--decl":
			if i+1 >= len(argv) {
				io.WriteString(stderr, dlUsage)
				return 2
			}
			decl = argv[i+1]
			i += 2
		case a == "--all":
			all = true
			i++
		case a == "-h" || a == "--help":
			io.WriteString(stderr, dlUsage)
			return 0
		case a == "--":
			break parseLoop
		case strings.HasPrefix(a, "-"):
			io.WriteString(stderr, dlUsage)
			return 2
		default:
			query = a
			i++
		}
	}

	if query == "" {
		io.WriteString(stderr, dlUsage)
		return 2
	}
	if fi, err := os.Stat(decl); err != nil || !fi.Mode().IsRegular() {
		fmt.Fprintf(stderr, "decl-label: 声明件不存在：%s\n", decl)
		return 2
	}

	b, err := os.ReadFile(decl)
	if err != nil {
		// 已过 -f 关（存在且为常规件）⇒ 此路只余权限/IO 异常（bash 于此因 set -e 退 1）。
		fmt.Fprintf(stderr, "decl-label: %s: %v\n", decl, err)
		return 1
	}
	hits := dlScan(string(b), query)

	if all {
		if len(hits) >= 1 {
			for _, n := range hits {
				io.WriteString(stdout, n+"\n")
			}
			return 0
		}
		fmt.Fprintf(stderr, "decl-label: 零命中：%s（声明件 %s）\n", query, decl)
		return 1
	}
	if len(hits) == 1 {
		io.WriteString(stdout, hits[0]+"\n")
		return 0
	}
	fmt.Fprintf(stderr, "decl-label: 命中 %d 条（要恰好 1 条）——查询 '%s'（声明件 %s）\n", len(hits), query, decl)
	return 1
}
