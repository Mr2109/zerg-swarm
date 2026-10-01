// check_nodes_roster.go —— 判据件 `check-nodes-roster.py`（559 行）的 **Go 真搬**
// （甲案 C 类 · M2 · 入口 `checkNodesRosterGateMain`）。
//
// 为什么有这件（M2 = 按价值真搬 Go · 落点 core/cmd/zerg/）：
//
//	验收判据 = 对同一 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码三面逐字相同
//	（除下列**显式声明**的口径差面与未覆盖面）。本件**只落码、不接线**（✗ 不注册命令 ·
//	✗ 不删 py 原件 · 入口暂无调用点）。
//
// 本门 = 「机器名册真源唯一」的核对门：读本机名册（~/.zerg 下，不入仓）+ 仓内 gateway 的
// fleet 件 + deploy 的 plist + 在跑的 agentd（`ps` 只读），判 B/C/D/E/F 五条。
//
// ═══════════════════ 出厂声明行 (a) 默认根定位差异 ═══════════════════
//
//	py 从 `__file__` 上溯两层得 REPO_ROOT（= 仓根）；Go 无 `__file__` 语境 ⇒ 默认根 = cwd
//	（`os.Getwd()`，同 check-curl-timeout / check-ai-direct-edits 的既有口径）。⇒ 「脚本位置 ≠ cwd」
//	时 `--fleet` / `--plist` 的默认推导面不同（✗ 不声称逐字对齐）。对拍甲档显式传三路径；
//	乙档把 py 副本放进 `<fx>/scripts/gates/` 且 Go 以 cwd=`<fx>` 跑 ⇒ 两面同根。
//
// ═══════════════════ 出厂声明行 (b) 自指检查 ═══════════════════
//
//	真判：本件是否落在它自己扫描的面内？本门扫描面 = **三件具体文件**（本机名册 · 仓内 fleet 件 ·
//	盘上 plist）＋ 一条 `ps` 读数；**不含** core/**、不含任何目录遍历、不含 `git ls-files` 面
//	⇒ 本件**不在自己的扫描面内**（依据：py 全件零 `walk`/`listdir`/`glob`/`scandir` 调用，
//	读面逐条点名为具体路径）。⇒ 本单**不做影子根**，理由与命中数见回执 selfref 栏。
//	第四形态（落在他门扫描面内）：本件正文以**描述式**复述被扫形态（如真源路径前缀、机器名
//	旗标名），✗ 不作独立可解析字面量；夹具串按 py 原样保留（它们是**夹具输入**、不是扫描面命中）。
//
// ═══════════════════ 输出行序的决定者 ═══════════════════
//
//	py 全件**零目录遍历**（无 walk/listdir/glob）⇒ 不存在「遍历序」面。输出行序由三处决定：
//	 ① 判据/检查项的**代码追加序**（B fleet → B updates → C → D → E → 再 F plist / F ps）；
//	 ② py **明写 `sorted(...)`** 的序：名册行 `sorted(nodes)` · `sorted(set(nodes)-set(fleet))` ·
//	    `sorted(set(fleet)-set(nodes))` · `sorted(fleet.items())`（按**键**）· `sorted(hosts-set(nodes))`
//	    ⇒ Go 侧一律 `sort.Strings`（UTF-8 字节序 == py code point 序）；
//	 ③ 列表面 `updates.nodes[]` 按 **parse_seq 产出序**（= 行序）。
//	★ 零 Go map 迭代进输出：凡 py 迭代 dict 的点，Go 侧都走显式排序的键切片。
//
// ═══════════════════ 正则面（本件只有 1 条）═══════════════════
//
//	py E：`re.search(r"\bhost\s*:\s*([A-Za-z0-9_.\-]+)", l)`。语义差两条：
//	 ① `\s` = py Unicode 空白 ⇒ 本件 `cnrIsSpace` 按码点展开（含 \x1c-\x1f / \x85 / \xa0 …）；
//	 ② `\b` = py 的 **Unicode** 词边界（CJK 亦算词字符）⟷ Go RE2 的 `\b` 是 ASCII
//	    ⇒ 本件**手写**扫描器 `cnrModelsHost`：用 py 口径的词字符判定复刻 `\b`，再逐字匹配
//	    `host\s*:\s*([A-Za-z0-9_.\-]+)`（值类**原样 ASCII**，✗ 不换成 \p{Nd}）。
//	`re.escape` 只用于 `section_range` 的**常量键**（全 ASCII 字母）⇒ 无差。
//
// ═══════════════════ py 现读硬伤（逐字沿用 · 交回入账 · ✗ 本单不修 py）═══════════════════
//
//	① **C 类**：py 无最外层兜底 —— `check_fleet` 里读 fleet 件**未捕异常**（件在盘但读不了 ⇒
//	   回溯 + rc=1）。本件 fail-closed **rc=2**（写进未覆盖面）。
//	② **B 类**：`load_roster` 的 `except Exception`（名册读不了 ⇒ 落 rc=2 文案）—— 逐字沿用。
//	③ **子进程面**：`running_agentd_machine()` 真跑 `ps -Ao command=`（只读 · 不碰进程）—— 逐字沿用。
//	④ **A 类**：`tempfile.TemporaryDirectory()` **有**回收（with 块）⇒ 本件 `defer os.RemoveAll` 对齐
//	   （自检前后同前缀目录计数 delta=0）。
//	⑤ **D 类**：`--list-rules` 真打印 `__doc__`（非死旗标）⇒ 本件逐字复刻其正文。
//
// ═══════════════════ 未覆盖面（✗ 不声称逐字对齐）═══════════════════
//
//	① **C 类兜底档**：见硬伤①（py 回溯 rc=1 ⟷ 本件 BLOCKED rc=2）。
//	② **argparse 极端内部形态**：`--` 分隔符 / 缩写歧义报话 / `--flag=v` 打在 flag 上 / 短旗标簇
//	   / 位置参数 —— 本件按「未知 ⇒ unrecognized / 缺值 ⇒ expected one argument」处理，深角正文不同。
//	③ **默认根定位**：见声明行 (a)。py 的 prog 名来自 argv[0] 基名；本件**定名**（同 py 常规调用）。
//	④ **非 UTF-8 角落**：py `open(encoding="utf-8")` 严格解码（非法序列抛异常）；本件 utf8.Valid 判否
//	   ⇒ 走「读不了」档。真仓三件现读全为合法 UTF-8 ⇒ 不触发。
//	⑤ **universal newlines**：py 文本模式把 `\r\n` / 裸 `\r` 折成 `\n`；本件显式同折。
//	⑥ **异常正文**：py `"名册读不了：%s" % e` / `"F plist 读不了（%s）：%s"` 里是 **py 异常 repr**；
//	   本件为 Go error 文本 ⇒ 极端读失败档的**正文字节**不同（rc 面一致）。
//	⑦ **`ps` 分列**：py `str.split()` 与 Go `strings.Fields` 对 `\x1c-\x1f` 一类判法略异 ⇒ 极端行不同。
//	⑧ **嵌套块取值**：py `str(dict)` 于「字段值竟是子块」时产出 `{...}`；本件走 `%v` ⇒ 该档不同
//	   （真仓三件的相关字段全是标量 ⇒ 不触发）。
//	⑨ **二进制 plist**：py `plistlib` 认 binary plist；本件只解 XML plist。
package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ─────────────────────────── py 出处的常量串（生成器注入 · 逐字节取自 py 原件）───────────────────────────

// cnrDoc —— py 模块 `__doc__`（`--list-rules` 的首段输出）。
const cnrDoc = "check-nodes-roster.py — 机器名册的**唯一真源核对**（契约 §九 M13 `Z2` / `P-069`）\n\n为什么存在\n----------\n「名册有三处并存、一个真源都没有」（`调研-M13` §4.1 实测）：`gateway/fleet.yaml` 的 `fleet:`\n（子端寻址）· 同文件的 `updates.nodes`（换件 ssh 目标）· 子端的机器名旗标（子端自报）。\n三处各自更新 ⇒ 任何「按名找到一台机器」的行为都建立在一张会漂的表上。\n\n定案（契约 §九 `Z2`）**名册真源唯一** = `~/.zerg/contexts/<档>.yaml` 的 `nodes:`；\n`fleet:` 与 `updates.nodes` 降为**投影源** —— 处置只给了两条路：① **由名册生成**，或\n② **被门禁逐项核对**。本门走 ②（①要动主控的配置加载面，属写面，不在本批）。\n\n⇒ **建而不收 = 又多一处**：只新建名册文件、不把三处旧来源绑到它，名册就只是第 4 份拷贝。\n本门就是那条「绑」——它同时判两件事：\n  ① 两段投影**有没有声明自己在投影**（段内必须写明真源路径）——没有声明 ⇒ rc=1（这才是「收」）；\n  ② 声明了之后**值面是否逐字相同**（host/port/os/ssh/root/prefix/api/components 逐字段）。\n\n判据（A–F，逐条能独立变红）\n--------------------------\n  A. 名册可读且 `nodes:` 非空（否则 rc=2 不给结论 —— 真源不在，判不了）\n  B. `gateway/fleet.yaml` 的 `fleet:` 段与 `updates:` 段**各自**带投影源声明（段内出现\n     `~/.zerg/contexts/`）——「建而不收」的直接判据\n  C. `fleet:` 键集 == 名册键集（**两向**）；每台 host/port/os 与名册逐字相同\n  D. `updates.nodes[]`：name ∈ 名册；ssh/root/prefix/api/components 与名册逐字相同\n  E. `models:` 段里每个 host ∈ 名册键（模型驻留在哪台 —— 第 4 处，顺带核）\n  F. 子端机器名旗标 ∈ 名册键：**盘上 plist**（`deploy/com.zerg.agent-Mr2109.plist`）与\n     **在跑的 `zerg-agentd`**（`ps` 读，只读）两处都核；没有在跑的进程 ⇒ 打印跳过（不是红）\n\n退码（与仓内既有门一致的三档）\n------------------------------\n  0 = 全对（PASS）\n  1 = 有漂移（FAIL）—— 逐条给「哪一处 · 哪个字段 · 现值 vs 名册值」\n  2 = **不给结论**（缺件 / 解析不出 `nodes:` / `fleet.yaml` 不在 / 自检不过 / 空转假覆盖）\n\n★ 本机物件：名册住在 `~/.zerg/`（**不入任何仓**）⇒ 在没建名册的机器上本门**必然** rc=2\n  （「名册不在、判不了」是事实，不是「绿」）。所以它**不进提交闸的快速档**（`FAST_STEPS`）：\n  否则每台没建名册的机器**每次提交**都会被拦一格 —— 那不叫严格，是把闸废掉（同 `check-wired-scripts`\n  升档时的先例）。它挂在**默认（全量）档** 的 `gates` scope 里。\n\n用法\n----\n    python3 scripts/gates/check-nodes-roster.py                 # 核真目标（仓内 fleet.yaml + 本机名册）\n    python3 scripts/gates/check-nodes-roster.py --roster P      # 换名册路径（默认 ~/.zerg/contexts/default.yaml）\n    python3 scripts/gates/check-nodes-roster.py --list-rules    # 打判据 / 路径来源 / 退码\n    python3 scripts/gates/check-nodes-roster.py --self-test     # 只跑自检（合成夹具，不碰真目标）\n"

// cnrHelpText —— py argparse `-h/--help` 的 stdout **逐字**（Python 3.9 用 `optional arguments:`）。
const cnrHelpText = "usage: check-nodes-roster.py [-h] [--roster ROSTER] [--fleet FLEET]\n                             [--plist PLIST] [--self-test] [--list-rules]\n\noptional arguments:\n  -h, --help       show this help message and exit\n  --roster ROSTER\n  --fleet FLEET\n  --plist PLIST\n  --self-test\n  --list-rules\n"

// cnrUsageLine —— argparse 出错时的 `usage:` 两行（与 help 首两行同）。
const cnrUsageLine = "usage: check-nodes-roster.py [-h] [--roster ROSTER] [--fleet FLEET]\n                             [--plist PLIST] [--self-test] [--list-rules]\n"

// cnrRosterFix / cnrFleetFix / cnrPlistFix —— py 自检夹具（逐字节取自 py 原件）。
const cnrRosterFix = "# 夹具名册\ncontext: default\nnodes:\n  edge-node:\n    host: <worker-ip>\n    port: 8100\n    os: ubuntu\n    ssh: g01@<worker-ip>\n    root: /home/<user>/zerg\n    prefix: /home/<user>/zerg/bin\n    api: \"http://127.0.0.1:8580\"\n    components: \"core,agentd\"\n  Mr2109:\n    host: 127.0.0.1\n    port: 8100\n    os: macos\n"

const cnrFleetFix = "# 夹具 fleet.yaml\nupdates:\n  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n  check: true\n  nodes:\n    - { name: edge-node, ssh: g01@<worker-ip>, root: /home/<user>/zerg, prefix: /home/<user>/zerg/bin, api: \"http://127.0.0.1:8580\", components: \"core,agentd\" }\nmodels:\n  m1: { host: edge-node, backend: llama-server }\nfleet:\n  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n  edge-node: { host: <worker-ip>, port: 8100, os: ubuntu }\n  Mr2109: { host: 127.0.0.1, port: 8100, os: macos }\n"

const cnrPlistFix = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict>\n  <key>Label</key><string>com.zerg.agent-Mr2109</string>\n  <key>ProgramArguments</key><array>\n    <string>/tmp/zerg-agentd</string>\n    <string>--machine</string><string>Mr2109</string>\n  </array>\n</dict></plist>\n"

// cnrValue —— py 无类型值的近似（string | map[string]cnrValue | []cnrValue）。
type cnrValue = interface{}

// ══════════════════════════ 路径 / 文件原语 ══════════════════════════

// cnrPyJoin —— 照 CPython posixpath.join 的逐段规则（✗ 不用 filepath.Join，见标准声明行）。
func cnrPyJoin(parts ...string) string {
	out := ""
	have := false
	for _, p := range parts {
		if !have {
			if p != "" {
				out = p
				have = true
			}
			continue
		}
		if len(p) > 0 && p[0] == '/' {
			out = p
			continue
		}
		if out == "" || out[len(out)-1] == '/' {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// cnrHome —— py `os.path.expanduser("~")`。
func cnrHome() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

// cnrIsFile —— py `os.path.isfile`（stat 跟随符号链接）。
func cnrIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// cnrDefaultRoster / Fleet / Plist —— py 三个缺省路径（REPO_ROOT 见声明行 (a)）。
func cnrRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func cnrDefaultRoster() string { return cnrPyJoin(cnrHome(), ".zerg", "contexts", "default.yaml") }
func cnrDefaultFleet() string  { return cnrPyJoin(cnrRepoRoot(), "gateway", "fleet.yaml") }
func cnrDefaultPlist() string {
	return cnrPyJoin(cnrRepoRoot(), "deploy", "com.zerg.agent-Mr2109.plist")
}

// cnrReadTextLines —— py `read_lines`：→ (raw, stripped)。文本模式折行同 py。
func cnrReadTextLines(path string) ([]string, []string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if !utf8.Valid(b) {
		return nil, nil, fmt.Errorf("invalid utf-8")
	}
	s := string(b)
	s = cnrReplaceAll(s, "\r\n", "\n")
	s = cnrReplaceAll(s, "\r", "\n")
	raw := cnrSplitStr(s, "\n")
	stripped := make([]string, len(raw))
	for i, l := range raw {
		stripped[i] = cnrStripComment(l)
	}
	return raw, stripped, nil
}

// ══════════════════════════ py 空白 / 字符串原语 ══════════════════════════

// cnrIsSpace —— py `\s`（Unicode · 含 \x1c-\x1f / \x85 / \xa0）。
func cnrIsSpace(r rune) bool {
	switch r {
	case 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// cnrPyStrip —— py `str.strip()`（无参 · Unicode 空白）。
func cnrPyStrip(s string) string {
	s = cnrTrimLeftFunc(s, cnrIsSpace)
	s = cnrTrimRightFunc(s, cnrIsSpace)
	return s
}

// cnrLstripSpaces —— py `s.lstrip(" ")`（**只剥 ASCII 空格**）。
func cnrLstripSpaces(s string) string {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return s[i:]
}

// cnrSkipSpace —— 跳过前导 py 空白（`\s*`）。
func cnrSkipSpace(s string) string {
	for len(s) > 0 {
		r, sz := utf8.DecodeRuneInString(s)
		if !cnrIsSpace(r) {
			break
		}
		s = s[sz:]
	}
	return s
}

// cnrStripComment —— py `strip_comment`。
func cnrStripComment(line string) string {
	var b []rune
	quote := rune(0)
	depth := 0
	prev := rune(0)
	first := true
	for _, ch := range line {
		if quote != 0 {
			b = append(b, ch)
			if ch == quote {
				quote = 0
			}
			prev = ch
			first = false
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			b = append(b, ch)
			prev = ch
			first = false
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			if depth > 0 {
				depth--
			}
		}
		if ch == '#' && depth == 0 && (first || prev == ' ' || prev == '\t') {
			break
		}
		b = append(b, ch)
		prev = ch
		first = false
	}
	return cnrPyRstrip(string(b))
}

// cnrPyRstrip —— py `str.rstrip()`（无参 · Unicode 空白）。
func cnrPyRstrip(s string) string { return cnrTrimRightFunc(s, cnrIsSpace) }

// cnrSplitTop —— py `split_top`（忽略引号与花括号内的 sep）。
func cnrSplitTop(text string, sep rune) []string {
	var parts []string
	var buf []rune
	quote := rune(0)
	depth := 0
	for _, ch := range text {
		if quote != 0 {
			buf = append(buf, ch)
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			buf = append(buf, ch)
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
		}
		if ch == sep && depth == 0 {
			parts = append(parts, string(buf))
			buf = nil
			continue
		}
		buf = append(buf, ch)
	}
	parts = append(parts, string(buf))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if cnrPyStrip(p) != "" {
			out = append(out, cnrPyStrip(p))
		}
	}
	return out
}

// cnrUnquote —— py `unquote`（剥外层成对引号）。
func cnrUnquote(v string) string {
	v = cnrPyStrip(v)
	r := []rune(v)
	if len(r) >= 2 && r[0] == r[len(r)-1] && (r[0] == '"' || r[0] == '\'') {
		return string(r[1 : len(r)-1])
	}
	return v
}

// cnrKVMatch —— py `_KV.match(s)`：`^([^:\s#][^:]*):\s*(.*)$` → (g1, g2, ok)。
func cnrKVMatch(s string) (string, string, bool) {
	idx := cnrIndexByte(s, ':')
	if idx < 0 {
		return "", "", false
	}
	key := s[:idx]
	if key == "" {
		return "", "", false
	}
	r, _ := utf8.DecodeRuneInString(key)
	if r == ':' || cnrIsSpace(r) || r == '#' {
		return "", "", false
	}
	return key, cnrPyStrip(s[idx+1:]), true
}

// cnrParseFlow —— py `parse_flow`：`{ k: v, ... }` → map（键值皆字符串）。
func cnrParseFlow(text string) map[string]cnrValue {
	inner := cnrPyStrip(text)
	if len(inner) > 0 && inner[0] == '{' {
		inner = inner[1:]
	}
	if len(inner) > 0 && inner[len(inner)-1] == '}' {
		inner = inner[:len(inner)-1]
	}
	out := map[string]cnrValue{}
	for _, part := range cnrSplitTop(inner, ',') {
		k, _, v := cnrPartition(part, ':')
		out[cnrUnquote(k)] = cnrUnquote(v)
	}
	return out
}

// cnrParseBlock —— py `parse_block`。
func cnrParseBlock(lines []string) map[string]cnrValue {
	out := map[string]cnrValue{}
	i := 0
	for i < len(lines) {
		raw := lines[i]
		if cnrPyStrip(raw) == "" {
			i++
			continue
		}
		ind := len(raw) - len(cnrLstripSpaces(raw))
		s := cnrPyStrip(raw)
		if len(s) > 0 && s[0] == '-' {
			i++
			continue
		}
		g1, val, ok := cnrKVMatch(s)
		if !ok {
			i++
			continue
		}
		key := cnrUnquote(g1)
		if val == "" {
			j := i + 1
			var sub []string
			for j < len(lines) {
				l2 := lines[j]
				if cnrPyStrip(l2) == "" {
					j++
					continue
				}
				if len(l2)-len(cnrLstripSpaces(l2)) <= ind {
					break
				}
				sub = append(sub, l2)
				j++
			}
			subStartDash := len(sub) > 0 && len(cnrPyStrip(sub[0])) > 0 && cnrPyStrip(sub[0])[0] == '-'
			if subStartDash {
				out[key] = cnrParseSeq(sub)
			} else {
				out[key] = cnrParseBlock(sub)
			}
			i = j
			continue
		}
		if len(val) > 0 && val[0] == '{' {
			out[key] = cnrParseFlow(val)
		} else {
			out[key] = cnrUnquote(val)
		}
		i++
	}
	return out
}

// cnrParseSeq —— py `parse_seq`。
func cnrParseSeq(lines []string) []cnrValue {
	var items []cnrValue
	var cur map[string]cnrValue
	hasCur := false
	i := 0
	for i < len(lines) {
		raw := lines[i]
		if cnrPyStrip(raw) == "" {
			i++
			continue
		}
		s := cnrPyStrip(raw)
		if len(s) > 0 && s[0] == '-' {
			ind := len(raw) - len(cnrLstripSpaces(raw))
			body := cnrPyStrip(s[1:])
			if hasCur {
				items = append(items, cur)
			}
			cur = map[string]cnrValue{}
			hasCur = true
			if len(body) > 0 && body[0] == '{' {
				for k, v := range cnrParseFlow(body) {
					cur[k] = v
				}
				i++
				continue
			}
			if g1, g2, ok := cnrKVMatch(body); ok {
				k := cnrUnquote(g1)
				if len(g2) > 0 && g2[0] == '{' {
					cur[k] = cnrParseFlow(g2)
				} else {
					cur[k] = cnrUnquote(g2)
				}
			}
			i++
			for i < len(lines) && cnrPyStrip(lines[i]) != "" &&
				(len(lines[i])-len(cnrLstripSpaces(lines[i]))) > ind {
				if g1, g2, ok := cnrKVMatch(cnrPyStrip(lines[i])); ok {
					k := cnrUnquote(g1)
					if len(g2) > 0 && g2[0] == '{' {
						cur[k] = cnrParseFlow(g2)
					} else {
						cur[k] = cnrUnquote(g2)
					}
				}
				i++
			}
			continue
		}
		i++
	}
	if hasCur {
		items = append(items, cur)
	}
	return items
}

// cnrSectionRange —— py `section_range`：顶层键 key 的半开区间。
func cnrSectionRange(lines []string, key string) (int, int, bool) {
	start := -1
	for i, l := range lines {
		if cnrHasPrefix(l, key) {
			rest := cnrSkipSpace(l[len(key):])
			if len(rest) > 0 && rest[0] == ':' {
				start = i + 1
				break
			}
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	end := len(lines)
	for j := start; j < len(lines); j++ {
		l := lines[j]
		if cnrPyStrip(l) != "" && !cnrHasPrefix(l, " ") && !cnrHasPrefix(l, "\t") && !cnrHasPrefix(l, "#") {
			end = j
			break
		}
	}
	return start, end, true
}

// cnrSectionLines —— py `section_lines`。
func cnrSectionLines(lines []string, key string) []string {
	s, e, ok := cnrSectionRange(lines, key)
	if !ok {
		return nil
	}
	return lines[s:e]
}

// ══════════════════════════ 取值 / 输出原语 ══════════════════════════

// cnrToString —— py `str(x)` 的近似（缺键 ⇒ 调用方已落 ""）。
func cnrToString(v cnrValue) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

// cnrStripQuotes —— py `str(x).strip('"')`。
func cnrStripQuotes(s string) string { return cnrTrimChars(s, '"') }

// cnrRepr —— py `repr(str)` 的近似（可读面 · 见未覆盖面）。
func cnrRepr(s string) string {
	var b []rune
	b = append(b, '\'')
	for _, r := range s {
		switch r {
		case '\\':
			b = append(b, '\\', '\\')
		case '\'':
			b = append(b, '\\', '\'')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			if r < 0x20 || r == 0x7f {
				b = append(b, []rune(fmt.Sprintf("\\x%02x", r))...)
			} else {
				b = append(b, r)
			}
		}
	}
	b = append(b, '\'')
	return string(b)
}

// cnrLjust —— py `"%-40s"`（**按码点**补齐，✗ 非字节）。
func cnrLjust(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return s
	}
	return s + cnrRepeat(" ", n-len(r))
}

// cnrSortedKeys —— 排好序的键切片（唯一进输出的 dict 序来源）。
func cnrSortedKeys(m map[string]cnrValue) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// ══════════════════════════ 判据 A–F ══════════════════════════

// cnrLoadRoster —— py `load_roster`：→ (nodes, 错误文案)。
func cnrLoadRoster(path string) (map[string]cnrValue, string) {
	if !cnrIsFile(path) {
		return nil, fmt.Sprintf("名册不在：%s（本机物件；建它 = 决议⑥ 的第 0 步）", path)
	}
	_, lines, err := cnrReadTextLines(path)
	if err != nil {
		return nil, fmt.Sprintf("名册读不了：%s", err)
	}
	sec := cnrSectionLines(lines, "nodes")
	if sec == nil {
		return nil, "名册里没有顶层键 nodes:"
	}
	all := cnrParseBlock(sec)
	out := map[string]cnrValue{}
	for k, v := range all {
		if m, ok := v.(map[string]cnrValue); ok && len(m) > 0 {
			out[k] = m
		}
	}
	if len(out) == 0 {
		return nil, "名册的 nodes: 解析不出任何机器（空转 = 假覆盖）"
	}
	return out, ""
}

// cnrModelsHost —— py E 的 `re.search` 手写复刻（见件头「正则面」）。
func cnrModelsHost(line string) (string, bool) {
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		if !cnrWordBoundary(rs, i) {
			continue
		}
		if i+4 > len(rs) || string(rs[i:i+4]) != "host" {
			continue
		}
		j := i + 4
		for j < len(rs) && cnrIsSpace(rs[j]) {
			j++
		}
		if j >= len(rs) || rs[j] != ':' {
			continue
		}
		j++
		for j < len(rs) && cnrIsSpace(rs[j]) {
			j++
		}
		k := j
		for k < len(rs) && cnrIsHostChar(rs[k]) {
			k++
		}
		if k > j {
			return string(rs[j:k]), true
		}
	}
	return "", false
}

func cnrIsWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func cnrWordBoundary(rs []rune, i int) bool {
	left := i > 0 && cnrIsWord(rs[i-1])
	right := i < len(rs) && cnrIsWord(rs[i])
	return left != right
}

func cnrIsHostChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
		r == '_' || r == '.' || r == '-'
}

// cnrCheckFleet —— py `check_fleet`：→ (fails, checks, hardErr)。
func cnrCheckFleet(fleetPath string, nodes map[string]cnrValue) ([]string, []string, string) {
	var fails, checks []string
	if !cnrIsFile(fleetPath) {
		return []string{fmt.Sprintf("fleet.yaml 不在：%s", fleetPath)}, checks, ""
	}
	raw, lines, err := cnrReadTextLines(fleetPath)
	if err != nil {
		return nil, nil, fmt.Sprintf("fleet.yaml 读不了：%s", err)
	}

	// B. 两段各自声明「我是投影源」——看 **raw（含注释）**。
	for _, key := range []string{"fleet", "updates"} {
		s, e, ok := cnrSectionRange(lines, key)
		if !ok {
			fails = append(fails, fmt.Sprintf("fleet.yaml 里没有 %s: 段", key))
			continue
		}
		decl := false
		for _, l := range raw[s:e] {
			if cnrContains(l, "~/.zerg/contexts/") {
				decl = true
				break
			}
		}
		mark := "有"
		if !decl {
			mark = "**缺**"
		}
		checks = append(checks, fmt.Sprintf("B %s: 段投影源声明：%s", key, mark))
		if !decl {
			fails = append(fails, fmt.Sprintf(
				"%s: 段没有投影源声明（段内应出现真源路径 ~/.zerg/contexts/…）——「建而不收」就是不写这句", key))
		}
	}

	// C. fleet: 键集（两向）＋ 每台 host/port/os 逐字。
	if s, e, ok := cnrSectionRange(lines, "fleet"); ok {
		all := cnrParseBlock(lines[s:e])
		fleet := map[string]cnrValue{}
		for k, v := range all {
			if m, ok := v.(map[string]cnrValue); ok {
				fleet[k] = m
			}
		}
		onlyRoster := cnrSortedDiff(nodes, fleet)
		onlyFleet := cnrSortedDiff(fleet, nodes)
		checks = append(checks, fmt.Sprintf("C fleet: 键 %d 个 · 名册节点 %d 个", len(fleet), len(nodes)))
		for _, k := range onlyFleet {
			fails = append(fails, fmt.Sprintf("C fleet: 里的 `%s` 不在名册（子端名册没登记它）", k))
		}
		for _, k := range onlyRoster {
			fails = append(fails, fmt.Sprintf("C 名册里的 `%s` 不在 fleet: 段（名册有、投影没有）", k))
		}
		for _, name := range cnrSortedKeys(fleet) {
			if _, ok := nodes[name]; !ok {
				continue
			}
			ent := fleet[name].(map[string]cnrValue)
			nEnt := nodes[name].(map[string]cnrValue)
			for _, f := range []string{"host", "port", "os"} {
				want := cnrStripQuotes(cnrToString(nEnt[f]))
				got := cnrStripQuotes(cnrToString(ent[f]))
				if got != want {
					fails = append(fails, fmt.Sprintf("C `%s` 的 %s 漂了：fleet=%s 名册=%s",
						name, f, cnrRepr(got), cnrRepr(want)))
				}
			}
		}
	}

	// D. updates.nodes 逐字段。
	if s, e, ok := cnrSectionRange(lines, "updates"); ok {
		ups := cnrParseBlock(lines[s:e])
		var items []cnrValue
		if v, ok := ups["nodes"]; ok {
			if lst, ok := v.([]cnrValue); ok {
				items = lst
			}
		}
		checks = append(checks, fmt.Sprintf("D updates.nodes 条目 %d 个", len(items)))
		for _, itv := range items {
			it, ok := itv.(map[string]cnrValue)
			if !ok {
				continue
			}
			name := cnrStripQuotes(cnrToString(it["name"]))
			if name == "" {
				fails = append(fails, "D updates.nodes 有一项没有 name")
				continue
			}
			if _, ok := nodes[name]; !ok {
				fails = append(fails, fmt.Sprintf("D updates.nodes 里的 `%s` 不在名册", name))
				continue
			}
			nEnt := nodes[name].(map[string]cnrValue)
			for _, f := range []string{"ssh", "root", "prefix", "api", "components"} {
				want := cnrStripQuotes(cnrToString(nEnt[f]))
				got := cnrStripQuotes(cnrToString(it[f]))
				if got != want {
					fails = append(fails, fmt.Sprintf("D `%s` 的 %s 漂了：updates.nodes=%s 名册=%s",
						name, f, cnrRepr(got), cnrRepr(want)))
				}
			}
		}
	}

	// E. models: 里的 host 必须在名册。
	if s, e, ok := cnrSectionRange(lines, "models"); ok {
		hosts := map[string]bool{}
		for _, l := range lines[s:e] {
			if h, ok := cnrModelsHost(l); ok {
				hosts[h] = true
			}
		}
		checks = append(checks, fmt.Sprintf("E models: 段出现 %d 个 host", len(hosts)))
		var extra []string
		for h := range hosts {
			if _, ok := nodes[h]; !ok {
				extra = append(extra, h)
			}
		}
		sort.Strings(extra)
		for _, h := range extra {
			fails = append(fails, fmt.Sprintf("E models: 里的 host `%s` 不在名册（模型驻留在哪台必须有条目）", h))
		}
	}
	return fails, checks, ""
}

// cnrRunningAgentdMachine —— py `running_agentd_machine`（只读 ps · 不碰进程）。
func cnrRunningAgentdMachine() (string, bool) {
	out, err := exec.Command("ps", "-Ao", "command=").Output()
	if err != nil {
		return "", false
	}
	for _, line := range cnrSplitStr(string(out), "\n") {
		if !cnrContains(line, "zerg-agentd") || cnrContains(line, "grep") {
			continue
		}
		parts := cnrFields(line)
		for i, p := range parts {
			if cnrHasSuffix(p, "machine") && i+1 < len(parts) {
				return parts[i+1], true
			}
		}
	}
	return "", false
}

// cnrPlistArgs —— py `plistlib.load(f).get("ProgramArguments") or []`（XML plist）。
func cnrPlistArgs(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	var args []string
	inPA := false
	pendingKey := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "key":
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return nil, err
				}
				pendingKey = s
			case "array":
				if pendingKey == "ProgramArguments" {
					inPA = true
				}
			case "string":
				if inPA {
					var s string
					if err := dec.DecodeElement(&s, &t); err != nil {
						return nil, err
					}
					args = append(args, s)
				}
			}
		case xml.EndElement:
			if t.Name.Local == "array" {
				inPA = false
			}
		}
	}
	return args, nil
}

// cnrCheckSubend —— py `check_subend`（F）。
func cnrCheckSubend(plistPath string, nodes map[string]cnrValue) ([]string, []string) {
	var fails, checks []string
	val := ""
	hasVal := false
	if cnrIsFile(plistPath) {
		args, err := cnrPlistArgs(plistPath)
		if err != nil {
			fails = append(fails, fmt.Sprintf("F plist 读不了（%s）：%s", plistPath, err))
		} else {
			for i, a := range args {
				if cnrHasSuffix(a, "machine") && i+1 < len(args) {
					val = args[i+1]
					hasVal = true
				}
			}
		}
	} else {
		checks = append(checks, fmt.Sprintf("F 盘上 plist 不在（%s）⇒ 跳过该处", plistPath))
	}
	if hasVal {
		checks = append(checks, fmt.Sprintf("F 盘上 plist 机器名 = %s", val))
		if _, ok := nodes[val]; !ok {
			fails = append(fails, fmt.Sprintf("F plist 的机器名 `%s` 不在名册（改机器名要先改名册）", val))
		}
	}
	run, ok := cnrRunningAgentdMachine()
	if !ok {
		checks = append(checks, "F 在跑的 zerg-agentd：没找到（跳过该处）")
	} else {
		checks = append(checks, fmt.Sprintf("F 在跑的 zerg-agentd 机器名 = %s", run))
		if _, ok := nodes[run]; !ok {
			fails = append(fails, fmt.Sprintf("F 在跑的 zerg-agentd 机器名 `%s` 不在名册（子端自报和真源对不上）", run))
		}
	}
	return fails, checks
}

// cnrRunChecks —— py `run_checks`。
func cnrRunChecks(stdout io.Writer, roster, fleet, plist string, verbose bool) int {
	nodes, errMsg := cnrLoadRoster(roster)
	if errMsg != "" {
		fmt.Fprintf(stdout, "⛔ 不给结论：%s\n", errMsg)
		return 2
	}
	fmt.Fprintf(stdout, "真源名册 = %s（节点 %d 个：%s）\n", roster, len(nodes),
		cnrJoinStr(cnrSortedKeys(nodes), " · "))
	fails, checks, hard := cnrCheckFleet(fleet, nodes)
	if hard != "" {
		fmt.Fprintf(stdout, "⛔ 不给结论：%s\n", hard)
		return 2
	}
	f2, c2 := cnrCheckSubend(plist, nodes)
	fails = append(fails, f2...)
	checks = append(checks, c2...)
	if verbose {
		for _, c := range checks {
			fmt.Fprintf(stdout, "   · %s\n", c)
		}
	}
	if len(fails) > 0 {
		fmt.Fprintf(stdout, "✗ 名册与投影不一致：%d 处\n", len(fails))
		for _, f := range fails {
			fmt.Fprintf(stdout, "   ✗ %s\n", f)
		}
		return 1
	}
	fmt.Fprint(stdout, "✓ 三处旧来源与名册逐字一致（fleet: · updates.nodes · 子端机器名；models: 的 host 也各在册）\n")
	return 0
}

// ══════════════════════════ 自检（合成夹具 · 成对负控 · 不碰真目标）══════════════════════════

// cnrSelfTest —— py `self_test()`。
func cnrSelfTest(stdout, stderr io.Writer) int {
	bad := 0
	d, err := os.MkdirTemp("", "tmp")
	if err != nil {
		fmt.Fprintf(stdout, "自检跑不了：造不出临时目录（%v）\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(d) }() // A 类对齐：py `with TemporaryDirectory()` 有回收
	w := func(name, text string) string {
		p := cnrPyJoin(d, name)
		_ = os.WriteFile(p, []byte(text), 0o644)
		return p
	}
	roster := w("roster.yaml", cnrRosterFix)
	fleet := w("fleet.yaml", cnrFleetFix)
	plist := w("a.plist", cnrPlistFix)

	type kase struct {
		name                 string
		roster, fleet, plist string
		want                 int
	}
	cases := []kase{
		{"全对 ⇒ 0", roster, fleet, plist, 0},
		{"fleet: 少一台 ⇒ 1", roster, w("f2.yaml",
			cnrReplaceAll(cnrFleetFix, "  Mr2109: { host: 127.0.0.1, port: 8100, os: macos }\n", "")), plist, 1},
		{"fleet: host 改一位 ⇒ 1", roster, w("f3.yaml",
			cnrReplaceAll(cnrFleetFix, "edge-node: { host: <worker-ip>", "edge-node: { host: <worker-ip>")), plist, 1},
		{"fleet: 投影源声明删掉 ⇒ 1（建而不收）", roster, w("f4.yaml",
			cnrReplaceN(cnrFleetFix, "  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n", "", 1)), plist, 1},
		{"updates.nodes 的 ssh 改 ⇒ 1", roster, w("f5.yaml",
			cnrReplaceAll(cnrFleetFix, "ssh: g01@<worker-ip>", "ssh: root@<worker-ip>")), plist, 1},
		{"models: host 未登记 ⇒ 1", roster, w("f6.yaml",
			cnrReplaceAll(cnrFleetFix, "m1: { host: edge-node", "m1: { host: ghost")), plist, 1},
		{"updates 段声明删掉 ⇒ 1", roster, w("f7.yaml",
			cnrReplaceN(cnrFleetFix, "  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n", "", 2)), plist, 1},
		{"plist 机器名未登记 ⇒ 1", roster, fleet, w("b.plist",
			cnrReplaceAll(cnrPlistFix, "<string>Mr2109</string>", "<string>ghost</string>")), 1},
		{"名册不在 ⇒ 2（不给结论）", cnrPyJoin(d, "nope.yaml"), fleet, plist, 2},
		{"名册 nodes 空 ⇒ 2", w("r2.yaml", "nodes:\n"), fleet, plist, 2},
	}

	for _, c := range cases {
		rc := cnrRunChecks(stdout, c.roster, c.fleet, c.plist, false)
		mark := "✓"
		if rc != c.want {
			mark = "✗"
			bad++
		}
		fmt.Fprintf(stdout, "   %s %s 期望 rc=%d 实得 rc=%d\n", mark, cnrLjust(c.name, 40), c.want, rc)
	}
	concl := "全过"
	if bad != 0 {
		concl = "**不过**"
	}
	fmt.Fprintf(stdout, "自检结论：%s（%d 条，失败 %d）\n", concl, len(cases), bad)
	if bad == 0 {
		return 0
	}
	return 2
}

// ══════════════════════════ 缺口 Q-176：本门无机器读面 ⇒ 给了 `--json` 不许沉默 ══════════════════════════

// cnrJSONFields —— 本门机器读面字段表（**唯一真源**；空表 ⇒ 印「无」）。
var cnrJSONFields = []string{}

// cnrRefuseJSON —— py `refuse_json_without_machine_face`。
func cnrRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	var hit []string
	for _, x := range argv {
		if x == "--json" || cnrHasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	quoted := make([]string, len(hit))
	for i, h := range hit {
		quoted[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", cnrJoinStr(quoted, " · "))
	fs := "无（本门只出人读面）"
	if len(cnrJSONFields) > 0 {
		qs := make([]string, len(cnrJSONFields))
		for i, f := range cnrJSONFields {
			qs[i] = "`" + f + "`"
		}
		fs = cnrJoinStr(qs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fs)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2, true
}

// ══════════════════════════ argparse 复刻（子集）══════════════════════════

type cnrArgs struct {
	roster    string
	fleet     string
	plist     string
	selfTest  bool
	listRules bool
}

var cnrOptSpecs = []struct {
	name  string
	takes bool
}{
	{"--roster", true},
	{"--fleet", true},
	{"--plist", true},
	{"--self-test", false},
	{"--list-rules", false},
	{"--help", false},
}

// cnrMatchOpt —— argparse 长旗标精确 / 唯一前缀匹配。
func cnrMatchOpt(name string) (string, bool, bool) {
	for _, s := range cnrOptSpecs {
		if s.name == name {
			return s.name, s.takes, true
		}
	}
	var cands []string
	for _, s := range cnrOptSpecs {
		if len(name) < len(s.name) && cnrHasPrefix(s.name, name) {
			cands = append(cands, s.name)
		}
	}
	if len(cands) == 1 {
		for _, s := range cnrOptSpecs {
			if s.name == cands[0] {
				return s.name, s.takes, true
			}
		}
	}
	return "", false, false
}

// cnrParseArgs —— 复刻六键 argparse 面。rc<0 ⇒ 成功；rc=0 ⇒ 打帮助；rc=2 ⇒ 用法错。
func cnrParseArgs(argv []string) (cnrArgs, int, string) {
	a := cnrArgs{roster: cnrDefaultRoster(), fleet: cnrDefaultFleet(), plist: cnrDefaultPlist()}
	var unknown []string
	i := 0
	for i < len(argv) {
		arg := argv[i]
		if arg == "--" {
			unknown = append(unknown, arg)
			unknown = append(unknown, argv[i+1:]...)
			break
		}
		if arg == "-h" || arg == "--help" {
			return cnrArgs{}, 0, ""
		}
		if cnrHasPrefix(arg, "--") {
			name, val, hasVal := arg, "", false
			if eq := cnrIndexByte(arg, '='); eq >= 0 {
				name, val, hasVal = arg[:eq], arg[eq+1:], true
			}
			canon, takes, ok := cnrMatchOpt(name)
			if !ok {
				unknown = append(unknown, arg)
				i++
				continue
			}
			if canon == "--help" {
				return cnrArgs{}, 0, ""
			}
			if takes {
				if !hasVal {
					if i+1 >= len(argv) {
						return cnrArgs{}, 2, "argument " + canon + ": expected one argument"
					}
					i++
					val = argv[i]
				}
				switch canon {
				case "--roster":
					a.roster = val
				case "--fleet":
					a.fleet = val
				case "--plist":
					a.plist = val
				}
			} else {
				if hasVal {
					return cnrArgs{}, 2, "argument " + canon + ": ignored explicit argument '" + val + "'"
				}
				switch canon {
				case "--self-test":
					a.selfTest = true
				case "--list-rules":
					a.listRules = true
				}
			}
			i++
			continue
		}
		unknown = append(unknown, arg)
		i++
	}
	if len(unknown) > 0 {
		return cnrArgs{}, 2, "unrecognized arguments: " + cnrJoinStr(unknown, " ")
	}
	return a, -1, ""
}

// ══════════════════════════ 入口 ══════════════════════════

// checkNodesRosterGateMain —— 入口（= py `main()`）。
func checkNodesRosterGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, hit := cnrRefuseJSON(stderr, argv); hit {
		return rc
	}
	a, rc, msg := cnrParseArgs(argv)
	if rc == 0 {
		fmt.Fprint(stdout, cnrHelpText)
		return 0
	}
	if rc > 0 {
		fmt.Fprint(stderr, cnrUsageLine)
		fmt.Fprintf(stderr, "check-nodes-roster.py: error: %s\n", msg)
		return 2
	}
	if a.listRules {
		fmt.Fprint(stdout, cnrDoc)
		fmt.Fprint(stdout, "\n")
		fmt.Fprintf(stdout, "路径来源：名册 %s · fleet %s · plist %s（都可覆盖；自检用夹具）\n",
			a.roster, a.fleet, a.plist)
		return 0
	}
	if a.selfTest {
		return cnrSelfTest(stdout, stderr)
	}
	rc2 := cnrSelfTest(stdout, stderr)
	if rc2 != 0 {
		fmt.Fprint(stderr, "⛔ 自检不过 ⇒ 拒绝核真目标\n")
		return 2
	}
	fmt.Fprint(stdout, "── 自检通过，开始核真目标 ──\n")
	return cnrRunChecks(stdout, a.roster, a.fleet, a.plist, true)
}

// ══════════════════════════ 薄封装（件名前缀 · 便于并发路不撞名）══════════════════════════

func cnrReplaceAll(s, old, nw string) string { return strings.ReplaceAll(s, old, nw) }

func cnrReplaceN(s, old, nw string, n int) string { return strings.Replace(s, old, nw, n) }

func cnrSplitStr(s, sep string) []string { return strings.Split(s, sep) }

func cnrJoinStr(parts []string, sep string) string { return strings.Join(parts, sep) }

func cnrFields(s string) []string { return strings.Fields(s) }

func cnrContains(s, sub string) bool { return strings.Contains(s, sub) }

func cnrHasPrefix(s, p string) bool { return strings.HasPrefix(s, p) }

func cnrHasSuffix(s, suf string) bool { return strings.HasSuffix(s, suf) }

func cnrIndexByte(s string, b byte) int { return strings.IndexByte(s, b) }

func cnrPartition(s string, sep byte) (string, string, string) {
	i := strings.IndexByte(s, sep)
	if i < 0 {
		return s, "", ""
	}
	return s[:i], string(sep), s[i+1:]
}

func cnrRepeat(s string, n int) string { return strings.Repeat(s, n) }

func cnrTrimChars(s string, cut rune) string { return strings.Trim(s, string(cut)) }

func cnrTrimLeftFunc(s string, f func(rune) bool) string { return strings.TrimLeftFunc(s, f) }

func cnrTrimRightFunc(s string, f func(rune) bool) string { return strings.TrimRightFunc(s, f) }

func cnrSortedDiff(a, b map[string]cnrValue) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
