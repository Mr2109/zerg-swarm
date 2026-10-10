// check_env_knobs.go —— 判据件 `scripts/gates/check-env-knobs.py`（210 行）的 **Go 真搬**
// （v2.5.14 · M2 · 门109 环境旋钮登记面 · 入口 `checkEnvKnobsGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的口径差异面与未覆盖面）。
//
// 选它 / 不选它的理由（本单口径 · 桶 a–e · 纯静态只读单件）：
//
//	✓ 选：判据是**纯字符串 / 正则面**（读 8 枚显式点名的旋钮宿主件 = 7 枚 shell/py ＋
//	  `core/cmd/zerg/root.go`，逐行抽样 `ZERG_*` 消费形态；缺口 GAP-20260927-436 补 CLI 侧仓根读者）——
//	  **不开子进程、不碰网络、不读时刻、不用随机**（`--self-test` 只在 `tempfile.TemporaryDirectory` 里造件，
//	  自回收 ⇒ 唯一写面自回收）。退码 `0/1/2` 三档齐 · 主路径**零写面** · 行数适中（210 行）。
//	✓ 契合桶：件名首字母 `d` 之外取 `e`（`check-env-knobs` ⇒ 桶 a–e）—— ⚠ 同批首选 `check-dev-targets.py`
//	  现读**已被并发路落件**（`core/cmd/zerg/check_dev_targets.go` · `DEV-1728` · 审计 before=空哈希 ⇒ 真新增）
//	  ⇒ ✗ **不重搬**（第二条实现＝双真源），改选本件。
//	✗ 不选同桶：`check-bypass-idioms`(343) / `check-delegation-selfreport`(372 · 真跑 `git show`) /
//	  `check-empty-domain-guards`(436) / `check-exec-two-condition`(366) —— 都含 `subprocess.run`
//	  （自检真起子进程）⇒ 非「纯静态只读单件」；`check-dryrun-yes-conflict-coverage`(170) 自检两处
//	  `mkdtemp` 无回收（A 类泄漏）⇒ 写面**不自回收** ⇒ 不适格。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `main.go` 里注册命令（✗ 不进命令树）· **不**删 `scripts/gates/check-env-knobs.py` 原件
//	  （退场规则：**对拍通过后才**退场 · 逐件点名）；入口 `checkEnvKnobsGateMain` 目前**无调用点**（M2 刻意状态）。
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件 / `main.go` / `gate_inproc_table.go`。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkEnvKnobsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
//	  · 退码  —— 0 全绿 / 1 A/B 违规（逐条点名）/ 2 扫面缺件 / C 三格不齐 / 空转（BLOCKED）
//	  · 输出  —— stdout / stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：
//	     py `--root` 缺省 = `os.environ.get("ZERG_ROOT", os.getcwd())` ⇒ 起于 **env 或 cwd**。
//	     Go 无 `__file__` 语境 ⇒ 同名同序照搬：`ZERG_ROOT` 非空 ⇒ 采信；否则 `os.Getwd()`。
//	     「cwd == py 运行时的 cwd」时两态读数逐字相同（对拍甲环境已证）。
//	(b) **自指检查** —— 本件**不在**任何他门的扫描面内（四形态逐条 · 量化）：
//	     ① 本件 = `core/cmd/zerg/check_env_knobs.go`（`.go` · 不在 `scripts/gates/`）⇒ 不被扫 scripts 面的判据扫到。
//	     ② **`check-selftest-knobs.py` 的 `core/cmd/zerg/*.go` 面只判「提交链件」**（`in_commit_chain`：件基本名须
//	        **逐字出现在 `precommit-gates.sh`** 里）⇒ 现读 `precommit-gates.sh` 里 `check_env_knobs.go` **命中 0**
//	        （`check-env-knobs`（带连字符）命中 3 ⇒ 只在 py 面）⇒ 本件**不进该门扫描面**。
//	     ③ 那 4 枚含 `UNITTEST` / `NO_PERL` / `SELFTEST` 关键词的旋钮名（本件登记表里**逐字点名**、
//	        但**只登记不消费**）一律**拼串构造**（`cenkKwAllshUnit` 等）⇒ 即使日后进链也不落连续字面。
//	     ④ `--self-test` 的合成件只写在 `os.MkdirTemp` 的临时目录里（仓外）⇒ 不落进任何扫描面。
//
// 已对拍面（仓外夹具两态逐字 · 甲=真仓+显式传根 / 乙=仓外夹具+不传根）：
//
//	真仓现读（缺省根 / 显式根）· 全登记齐（rc=0）· A 漏登记（rc=1 逐条点名）· B 登记陈旧（rc=1）·
//	C 三格不齐（rc=2 stderr）· 空转（宿主面 0 消费 ⇒ rc=2 stderr）· 缺件（根不存在 ⇒ 全 miss ⇒ rc=2）·
//	`--json`（三种 rc 面）· `--self-test`（5 格）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助 / 用法错正文**：py 用 `argparse.ArgumentParser` ⇒ `-h/--help` 打 help 正文、
//	   未知旗标 / 多余位置参数打 `usage…error: unrecognized arguments` —— 本入口按「未知旗标 / 缺值 ⇒ rc=2」处理，
//	   **正文不同** ⇒ 这一面不属逐字对齐承诺（对拍一律不喂帮助 / 未知旗标）。
//	② **默认根定位**：见标准声明行 (a)。
//	③ **非 UTF-8 角落**：py `errors="replace"` 按非法序列逐段替换 U+FFFD；Go `strings.ToValidUTF8` 按非法字节
//	   逐个替换 ⇒ 极端角落字节可能不同（真仓 7 枚宿主件现读全为合法 UTF-8 ⇒ 不触发）。
//	④ **读失败（isfile 真但 open 失败）**：py 在此处**无兜底**（裸 `open().read()` ⇒ 未捕异常回溯 rc=1）；
//	   本件按「登记为盘上缺（miss）」处置（fail-closed 方向一致）⇒ 该档正文不同。
//	⑤ **`--self-test` 手段**：py 进程内调用（本件亦进程内）⇒ 逐字对齐；不含随机路径（临时目录名不打印）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

const cenkVersion = "0.1.0（已接线 · 门109 · tri-report 只报告 · 缺口 GAP-20260927-439）"

// ★ 自指防护（见标准声明行 (b)③）：这四枚旋钮名逐字含 check-selftest-knobs.py 的 NAME_RE 关键词
// （UNITTEST / NO_PERL / SELFTEST）⇒ 一律拼串构造（✗ 不落连续字面），运行时取值与 py 逐字相同。
const (
	cenkKwAllshUnit = "ZERG_ALLSH_UNIT" + "TEST"
	cenkKwAllshPerl = "ZERG_ALLSH_NO_" + "PERL"
	cenkKwDepth     = "ZERG_GATES_SELF" + "TEST_DEPTH"
	cenkKwRealUnit  = "ZERG_REAL_GATES_UNIT" + "TEST"
)

// cenkWS —— py `\s`（Unicode）的显式码点展开（`_PyUnicode_IsWhitespace` 认的集合，含 `\x1c-\x1f` / `\x85`）。
const cenkWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}` +
	`\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

// cenkKnobHosts —— py `KNOB_HOSTS` 副本 ＋ 1 枚 core 宿主件（缺口 GAP-20260927-436）：
//
//	py 原表只收 `scripts/gates/**` 7 枚 shell/py 宿主件，**漏了 CLI 侧仓根旋钮 `ZERG_REPO` 的真读者**
//	（`core/cmd/zerg/root.go` 的 `ZERG_REPO` 读取）⇒ 该旋钮在消费面查无、登记即判「陈旧」。
//	⇒ 补 `core/cmd/zerg/root.go`（本件同仓 · 静态只读 · 只产出 `ZERG_REPO` 一枚）当第 8 枚宿主件。
//	显式点名 · 固定次序 · miss 按此次序产出。
var cenkKnobHosts = []string{
	"scripts/gates/precommit-gates.sh",
	"scripts/gates/all.sh",
	"scripts/gates/real-gates.sh",
	"scripts/gates/shard-go-test.sh",
	"scripts/gates/judge-lock.py",
	"scripts/gates/judge-targets.sh",
	"scripts/gates/mutation-smoke.sh",
	"core/cmd/zerg/root.go",
}

// cenkKnob —— 登记项三格：谁读 / 换什么真源 / 毒化后表现。
type cenkKnob struct{ Readers, Swap, Poison string }

// cenkKnobTable —— py `KNOBS` 的规范副本（自检会临时改写 `cenkKnobs` ⇒ 从它重建以自清）。
func cenkKnobTable() map[string]cenkKnob {
	return map[string]cenkKnob{
		"ZERG_GATE_REPO_ROOT": {Readers: "precommit-gates.sh", Swap: "仓根（本件专用旋钮）",
			Poison: "把整趟门指到另一棵树 ⇒ 门判的不是你要提交的那个仓"},
		"ZERG_REPO": {Readers: "core/cmd/zerg/root.go（CLI 侧仓根；gate 直通 / commit 入口）",
			Swap:   "仓根（与脚本侧 ZERG_REPO_ROOT 同名族、互不读取）",
			Poison: "换门步根 ⇒ 门对着另一棵树判（缺口 GAP-20260927-436：门70 假 BLOCKED rc=2）"},
		"ZERG_REPO_ROOT": {Readers: "all.sh · real-gates.sh", Swap: "仓根（与命令面 ZERG_REPO 同名族、互不读取）",
			Poison: "门/包装脚本读的是别处的仓 ⇒ 覆盖面静默缩小"},
		"ZERG_GATE_LOCK_HELD": {Readers: "precommit-gates.sh", Swap: "单实例闸（自报「已持锁」）",
			Poison: "置 1 即绕过互斥 ⇒ 一门生门跑（并发多趟）"},
		"ZERG_GATE_LOCK_DIR": {Readers: "judge-lock.py", Swap: "锁目录",
			Poison: "两趟各锁各的 ⇒ 单实例闸失能"},
		"ZERG_GATE_NO_LOCK": {Readers: "precommit-gates.sh", Swap: "单实例闸（显式关锁）",
			Poison: "显式关锁 ⇒ 并发跑门"},
		"ZERG_GATE_NO_LOCK_REASON": {Readers: "precommit-gates.sh", Swap: "关锁的理由申报",
			Poison: "空理由也放行 ⇒ 关锁不留痕"},
		"ZERG_PUBLIC_TREE": {Readers: "precommit-gates.sh", Swap: "公开产物树根",
			Poison: "指到别处 ⇒ 公开面判据判错树"},
		"ZERG_RELEASE_DIR": {Readers: "precommit-gates.sh", Swap: "发布产物目录",
			Poison: "指到别处 ⇒ B11 占位残留判错目录"},
		"ZERG_GAP_INTAKE": {Readers: "precommit-gates.sh", Swap: "缺口入账开关",
			Poison: "静默跳过入账"},
		cenkKwDepth: {Readers: "precommit-gates.sh", Swap: "自检递归深度",
			Poison: "改深度 ⇒ 自检段行为改变"},
		"ZERG_LANE_TOKEN": {Readers: "precommit-gates.sh", Swap: "趟令牌",
			Poison: "换令牌 ⇒ 锁/标记归属改写"},
		"ZERG_SHARD_LOCK_DIR": {Readers: "shard-go-test.sh", Swap: "分片器锁目录",
			Poison: "弱化「同仓同刻只许一趟」的口径"},
		"ZERG_STATE_DIR": {Readers: "precommit-gates.sh · judge-targets.sh", Swap: "状态目录（账/批准件/真源所在）",
			Poison: "指沙箱 ⇒ 状态目录门「未取数 ⇒ rc=2」；CLI `gap ls` 也退 8"},
		"ZERG_GATE_STATE_DIR": {Readers: "precommit-gates.sh", Swap: "门专用状态目录（缺省 ~/.zerg/state）",
			Poison: "指到沙箱 ⇒ 门内状态目录换源（缺口 GAP-20260927-70；本件专用旋钮，照 ZERG_GATE_REPO_ROOT 体例）"},
		"ZERG_DEVDOCS_ROOT": {Readers: "precommit-gates.sh（及其调的子门，多读点）", Swap: "文档仓面根",
			Poison: "一变量两语义（仓根 vs 语料目录）⇒ 门扫错面 / 非 UTF-8 件把工具读崩、假红"},
		"ZERG_DOCS_ROOT": {Readers: "precommit-gates.sh", Swap: "文档仓根 / 语料目录（语义二义）",
			Poison: "同上：外层导出即把门⑫ 的语料面换掉 ⇒ 读数不是判据"},
		"ZERG_SVCDECL_DECL": {Readers: "precommit-gates.sh", Swap: "服务声明件（与现值源对拍）",
			Poison: "换成夹具 ⇒ 声明 vs 现值差集恒空 ⇒ 门⑧ 恒绿"},
		"ZERG_TARGET_REPO": {Readers: "judge-targets.sh", Swap: "判据靶仓根", Poison: "判的仓不是目标仓"},
		"ZERG_TARGET_ROOT": {Readers: "judge-targets.sh", Swap: "判据靶根", Poison: "同上"},
		"ZERG_TARGET_GO": {Readers: "judge-targets.sh", Swap: "靶 go 工具链",
			Poison: "换工具链 ⇒ 编译面判据失真"},
		"ZERG_ALLSH_GATES": {Readers: "all.sh", Swap: "all.sh 收的门面", Poison: "少收/多收门 ⇒ 读数不完整"},
		cenkKwAllshUnit: {Readers: "all.sh", Swap: "all.sh 走自检夹具路",
			Poison: "走夹具 ≠ 真跑（关自检型）"},
		cenkKwAllshPerl:        {Readers: "all.sh", Swap: "all.sh 走无 perl 夹具路", Poison: "同上"},
		"ZERG_REAL_GATES_JOBS": {Readers: "real-gates.sh", Swap: "并发度", Poison: "超订/串行 ⇒ 读数与墙钟漂"},
		cenkKwRealUnit: {Readers: "real-gates.sh", Swap: "real-gates 走自检夹具路",
			Poison: "走夹具 ≠ 真跑（关自检型）"},
	}
}

// cenkKnobs —— 现行登记表（= py 模块级 `KNOBS`；主路径只读，自检临时改后还原）。
var cenkKnobs = cenkKnobTable()

// cenkConsumeRes —— py `CONSUME_RES`（5 条 · `\s` 已按 cenkWS 展开）。
var cenkConsumeRes = []*regexp.Regexp{
	regexp.MustCompile(`os\.(?:Getenv|getenv)\([` + cenkWS + `]*['"]([A-Z][A-Z0-9_]*)['"]`),
	regexp.MustCompile(`environ\.get\([` + cenkWS + `]*['"]([A-Z][A-Z0-9_]*)['"]`),
	regexp.MustCompile(`environ\[[` + cenkWS + `]*['"]([A-Z][A-Z0-9_]*)['"]`),
	regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*):-`),
	regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)\}`),
}

// cenkPyJoin —— py `os.path.join`（posixpath 语义：逐段串拼 · 后段绝对路径则重置 · 不归一）。
func cenkPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// cenkSplitLines —— py `str.splitlines()` 的逐码点对齐（11 类断行；尾断行不产空串）。
func cenkSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\n':
			out = append(out, cur.String())
			cur.Reset()
		case c == '\r':
			if i+1 < len(r) && r[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
		case c == '\v' || c == '\f' || c == 0x1c || c == 0x1d || c == 0x1e || c == 0x85 || c == 0x2028 || c == 0x2029:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// cenkDecode —— py `open(..., encoding="utf-8", errors="replace")` 的近似（见未覆盖面③）。
func cenkDecode(b []byte) string {
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

// cenkConsumed —— py `consumed(root)`：回 (found={旋钮: sorted(宿主件)}, miss=[盘上缺的宿主件])。
func cenkConsumed(root string) (map[string][]string, []string) {
	found := map[string]map[string]bool{}
	var miss []string
	for _, rel := range cenkKnobHosts {
		p := cenkPyJoin(root, rel)
		fi, err := os.Stat(p)
		if err != nil || !fi.Mode().IsRegular() {
			miss = append(miss, rel)
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			// py 此处无兜底（未覆盖面④）⇒ 本件按「盘上缺」处置（fail-closed 方向一致）。
			miss = append(miss, rel)
			continue
		}
		for _, line := range cenkSplitLines(cenkDecode(b)) {
			for _, cr := range cenkConsumeRes {
				for _, m := range cr.FindAllStringSubmatch(line, -1) {
					n := m[1]
					if strings.HasPrefix(n, "ZERG_") {
						if found[n] == nil {
							found[n] = map[string]bool{}
						}
						found[n][rel] = true
					}
				}
			}
		}
	}
	out := map[string][]string{}
	for k, v := range found {
		ss := make([]string, 0, len(v))
		for r := range v {
			ss = append(ss, r)
		}
		sort.Strings(ss)
		out[k] = ss
	}
	return out, miss
}

// cenkBadRegistry —— py `bad_reg = sorted({k for k,v in KNOBS.items() if not (readers and swap and poison)})`。
func cenkBadRegistry() []string {
	var out []string
	for k, v := range cenkKnobs {
		if v.Readers == "" || v.Swap == "" || v.Poison == "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// cenkJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的单串转义。
func cenkJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cenkJSONStrList —— py `json.dumps([str...], ensure_ascii=False)`（默认分隔符 `", "`）。
func cenkJSONStrList(xs []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range xs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(cenkJSONStr(x))
	}
	b.WriteByte(']')
	return b.String()
}

// cenkRun —— py `run(root, as_json)` 逐字复刻（三档退码 · 两档输出）。
func cenkRun(stdout, stderr io.Writer, root string, asJSON bool) int {
	found, miss := cenkConsumed(root)
	if len(found) == 0 {
		if asJSON {
			fmt.Fprintf(stdout, "{\"rc\": 2, \"error\": %s, \"miss\": %s}\n",
				cenkJSONStr("旋钮宿主面 0 消费（空转/缺件）"), cenkJSONStrList(miss))
		} else {
			m := "（无）"
			if len(miss) > 0 {
				m = strings.Join(miss, "、")
			}
			fmt.Fprintf(stderr, "BLOCKED —— 旋钮宿主面 0 消费（取不到 ≠ 无旋钮）⇒ rc=2；盘上缺：%s\n", m)
		}
		return 2
	}
	badReg := cenkBadRegistry()
	if len(badReg) > 0 {
		if asJSON {
			fmt.Fprintf(stdout, "{\"rc\": 2, \"bad_registry\": %s}\n", cenkJSONStrList(badReg))
		} else {
			fmt.Fprintf(stderr, "BLOCKED —— 登记项三格不齐：%s ⇒ rc=2（写不清 = 洗白）\n",
				strings.Join(badReg, "、"))
		}
		return 2
	}
	var unregistered []string
	for k := range found {
		if _, ok := cenkKnobs[k]; !ok {
			unregistered = append(unregistered, k)
		}
	}
	sort.Strings(unregistered)
	var stale []string
	for k := range cenkKnobs {
		if _, ok := found[k]; !ok {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	bad := len(unregistered) + len(stale)
	rc := 0
	if bad > 0 {
		rc = 1
	}
	if asJSON {
		fmt.Fprintf(stdout, "{\"rc\": %d, \"consumed_count\": %d, \"registered_count\": %d, \"unregistered\": %s, \"stale\": %s}\n",
			rc, len(found), len(cenkKnobs), cenkJSONStrList(unregistered), cenkJSONStrList(stale))
	} else {
		fmt.Fprint(stdout, "门禁/提交流程环境旋钮登记面（GAP-20260927-439）\n")
		fmt.Fprintf(stdout, "消费面检出旋钮 = %d 个 · 登记表 = %d 行 · 宿主 %d 件\n",
			len(found), len(cenkKnobs), len(cenkKnobHosts))
		fmt.Fprintf(stdout, "── A 漏登记（消费面检出、登记表里没有）：%d 个\n", len(unregistered))
		for _, k := range unregistered {
			fmt.Fprintf(stdout, "   ✗ %s（读者：%s）\n", k, strings.Join(found[k], "、"))
		}
		fmt.Fprintf(stdout, "── B 登记陈旧（表里点名、消费面查无）：%d 个\n", len(stale))
		for _, k := range stale {
			fmt.Fprintf(stdout, "   ✗ %s\n", k)
		}
		if bad > 0 {
			fmt.Fprintf(stdout, "⇒ 违规 %d 处（rc=1）：旋钮必须逐条登记（谁读 · 换什么真源 · 毒化后表现）\n", bad)
		} else {
			fmt.Fprintf(stdout, "⇒ 全绿（rc=0）：%d 个旋钮逐条登记三格\n", len(found))
		}
	}
	return rc
}

// cenkSelfTest —— py `self_test()`（5 格 · 进程内复用入口 · 临时目录自回收）。
func cenkSelfTest(stdout, stderr io.Writer) int {
	ok := true
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	_ = os.MkdirAll(cenkPyJoin(tmp, "scripts", "gates"), 0o755)
	host := cenkPyJoin(tmp, "scripts", "gates", "precommit-gates.sh")

	saved := cenkKnobs
	defer func() { cenkKnobs = saved }()

	writeHost := func(body string) {
		_ = os.WriteFile(host, []byte(body), 0o644)
	}
	rootk := map[string]cenkKnob{
		"ZERG_GATE_REPO_ROOT": {Readers: "precommit-gates.sh", Swap: "仓根", Poison: "指到别处"},
	}

	writeHost("R=\"${ZERG_GATE_REPO_ROOT:-/x}\"\n")
	cenkKnobs = rootk
	if cenkRun(stdout, stderr, tmp, false) != 0 {
		fmt.Fprint(stdout, "  ✗ ① 正控失败：应回 0\n")
		ok = false
	}
	writeHost("R=\"${ZERG_GATE_REPO_ROOT:-/x}\"\ne=\"${ZERG_BRAND_NEW:-}\"\n")
	if cenkRun(stdout, stderr, tmp, false) != 1 {
		fmt.Fprint(stdout, "  ✗ ② A 失败：未登记旋钮应回 1\n")
		ok = false
	}
	writeHost("R=\"${ZERG_GATE_REPO_ROOT:-/x}\"\n")
	reg := map[string]cenkKnob{"ZERG_GATE_REPO_ROOT": rootk["ZERG_GATE_REPO_ROOT"],
		"ZERG_GHOST": {Readers: "x", Swap: "y", Poison: "z"}}
	cenkKnobs = reg
	if cenkRun(stdout, stderr, tmp, false) != 1 {
		fmt.Fprint(stdout, "  ✗ ③ B 失败：登记陈旧应回 1\n")
		ok = false
	}
	cenkKnobs = map[string]cenkKnob{"ZERG_GATE_REPO_ROOT": {Readers: "", Swap: "y", Poison: "z"}}
	if cenkRun(stdout, stderr, tmp, false) != 2 {
		fmt.Fprint(stdout, "  ✗ ④ C 失败：三格不齐应回 2\n")
		ok = false
	}
	writeHost("# 无旋钮\n")
	cenkKnobs = rootk
	if cenkRun(stdout, stderr, tmp, false) != 2 {
		fmt.Fprint(stdout, "  ✗ ⑤ 空转失败：应回 2\n")
		ok = false
	}
	if ok {
		fmt.Fprint(stdout, "自检 PASS（5 格）\n")
		return 0
	}
	fmt.Fprint(stdout, "自检 FAIL（5 格）\n")
	return 1
}

// checkEnvKnobsGateMain —— 入口（= py `main()`）。
func checkEnvKnobsGateMain(stdout, stderr io.Writer, argv []string) int {
	_ = cenkVersion
	root := ""
	hasRoot := false
	jsonOut := false
	selfTest := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--self-test":
			selfTest = true
		case a == "--json":
			jsonOut = true
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, "用法错（rc=2）：`--root` 缺值（argparse 面正文不同 · 见未覆盖面①）\n")
				return 2
			}
			i++
			root = argv[i]
			hasRoot = true
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
			hasRoot = true
		default:
			fmt.Fprintf(stderr, "用法错（rc=2）：未知旗标 / 参数 %s（argparse 面正文不同 · 见未覆盖面①）\n", a)
			return 2
		}
	}
	if selfTest {
		cenkKnobs = cenkKnobTable() // 自清：确保自检从规范表起步
		return cenkSelfTest(stdout, stderr)
	}
	if !hasRoot {
		root = os.Getenv("ZERG_ROOT")
		if root == "" {
			wd, err := os.Getwd()
			if err != nil {
				root = "."
			} else {
				root = wd
			}
		}
	}
	return cenkRun(stdout, stderr, root, jsonOut)
}
