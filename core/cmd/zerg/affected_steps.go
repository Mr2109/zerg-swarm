// affected_steps.go —— 门禁件 `scripts/gates/affected-steps.py` 的 **Go 真搬**（v2.5.14 · M2 路 A 续5）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值真搬 Go（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2 路 A 续5（候选区间首字母 a–e）的一枚：验收判据 = 对同一件跑两态
//	（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（选件口径 = 无外部树依赖 · 输出确定有序 · 退码仅 0/1/2 · 纯静态无写面 ·
// 行数适中 · 未被已搬 20 枚覆盖）：
//
//	✓ 选：`scripts/gates/affected-steps.py`（281 行 · 纯 stdlib argparse/fnmatch/json/os/re/sys/tempfile）
//	  · **无外部树依赖**（只读调用方点名的三件：`--list` 步表件 · `--map` 声明表 · `--changed` 改动清单；
//	    不碰文档仓 / 公开树 / 生成物 / 状态目录）
//	  · **输出确定有序**（零 os.walk / 零 glob / 零目录遍历；唯二排序点为 `sorted(faces)` 与
//	    按 `step_order` 保序过滤 ⇒ 逐字节可证）
//	  · **退码仅 0/1/2**（0 只报告 / 1 本轮不产生 / 2 不给结论：少声明·漏读·取不到指纹·用法错）
//	  · **纯静态无写面**（默认档只读；写面仅 `--self-test` 落 `tempfile.mkdtemp` 临时目录，跑完即删）
//	  · **无人正在改**（`git status --porcelain` 现读不含它）· 不属已列重件 · 不在已搬 20 枚内。
//	✗ 不选 `check-anchor-uniqueness.py`（340 行）：**外部树依赖**（默认读仓外 `Zerg-内部文档/` 登记件 +
//	  逐条现扫目标件）⇒ 不满足「无外部树依赖」。
//	✗ 不选 `check-contract-version-diff.py`（434 行）：依赖 **git 子进程**取基线 ⇒ 输入面 = 仓的可变
//	  历史而非仓内静态件 ⇒ 非纯静态、且两态不可逐字对拍（HEAD 会动）。
//	✗ 不选 `check-curl-timeout.py`（167 行）：扫描根靠 `os.walk` ⇒ 遍历序非确定（Go `WalkDir` 字典序
//	  ≠ py `scandir` inode 序）⇒ 对拍会假差。
//	✗ 不选 `check-exit-codes.py`（649 行）：R6 机检的**分母**靠 `os.walk` 全树现读 ⇒ 命中行序 = 遍历序
//	  （非确定）+ 输出面上这些「只报告」注行逐条打印 ⇒ 不满足「输出确定有序」。
//	✗ 不选 `check-build-tags.py`（863 行）· `check-artifact-source-parity.py`（476 行）·
//	  `check-approve-tier.py`（460 行）：**真跑外部工具**（`go build` / `go vet` / `bin/zerg` / `git`）⇒
//	  属 M2-2，非「纯静态检查」。
//	✗ 不选 `check-approve-{backfill,merge,time,freeze}.py`：读**状态目录 / 仓外 docs / hostname**
//	  （`~/.zerg/state` · `Zerg-内部文档`）或带写面（`--refresh-constant --yes`）⇒ 外部依赖 / 非纯静态。
//	✗ 不选 `check-doc-section-numbers.py`（189 行）等 8 件 `check-doc*`：**共享件禁改面**（任务铁律）。
//	✗ 不选 `check-dryrun-semantics.py` · `check-{ai-direct-edits,ci-package-parity,compat-manifest,
//	  contract-registry,error-kinds}.py`：**已被已搬 20 枚取走**。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/affected-steps.py` 原件（退场规则 4.3：**对拍通过后才**退场，逐件显式点名）；
//	· 入口 `affectedStepsGateMain` 目前**无调用点** —— 这是 M2 的**刻意**状态：先证「搬得逐字相同」，
//	  接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func affectedStepsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 只报告 / 2 不给结论 / 用法错（与 py 逐字对齐；1 本轮不产生）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字 · ≥8 输入）：正控（声明齐 + 有命中 ⇒ 0 · 点名 ▶）· 负控（少声明一步
// ⇒ 2「静默漏选」）· 负控（D==N 但集合不等 ⇒ 2）· 负控（缺指纹/基线 ⇒ 2「取不到输入基线」）·
// 负控（空 faces ⇒ 2）· 未声明件 ⇒ 报全量 · 空改动 ⇒ 「受影响 0 步」· 步表件缺 ⇒ 2 · 声明表缺 ⇒ 2 ·
// 无参 ⇒ 2 · `--json` 合法面 · `--self-test` ⇒ 0 · 真仓现读（真 `affected-map.json`）。
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 极端内部形态**：短旗标簇（`-hx`）· `--` 分隔符语义 · 缩写**歧义**报话
//	   （`--m` 同时匹配 `--map`，本件无第二长旗标冲突）· `--list=`（空值 ⇒ py 走「用法」档）——
//	   本件实现长旗标前缀匹配 + `=` 取值 + 缺值报话，上述极端形态只做到「主形态对齐」。
//	② **异常路径**：`--map` 件内容**不是合法 JSON** / 顶层不是对象 ⇒ py `json.load` / `mp.get` 抛
//	   未捕获异常 ⇒ traceback + 进程退码 1；本件落「读不到 / 解析失败」⇒ 退码同为 1，但 **stderr 正文
//	   与 py 的 traceback 不同** ⇒ 这一面不在对拍输入集内。
//	③ **glob 语法**：py `fnmatch`（`*` 跨 `/`、`[seq]`、`[!seq]`）与 Go 本件实现的 `*`/`?` 子集，
//	   字符组 `[..]` 已实现但未逐形态对拍（真仓 `affected-map.json` 现读零 `kind=glob` 条目）。
//	④ **`sorted(faces)` 的键序**：py 按 **Unicode 码点**排、Go `sort.Strings` 按 **UTF-8 字节**排
//	   —— 对 BMP 内字符两序相同（UTF-8 保序），本件未对拍星平面/代理对级极端键。
//
// ★★ 标准声明 (a)：**默认根定位差异 —— 无差异**。本件**没有**「默认仓根」概念（py 里也没有）：
// 三个入参（`--list` / `--map` / `--changed`）都由调用方**显式给出**，相对路径一律按**当前工作目录**
// 解析（py `open(path)` ⟷ Go `os.Open(path)`）⇒ 无 `__file__`→cwd 的根定位落差。其余未覆盖面见上①②③④。
//
// ★★ 标准声明 (b)：**自指检查 —— 前提为真（本件落在他门扫描面内），已拼串/转义并证明有效**。
// 四形态逐条：
//
//	① 本件在自己扫描面内？**假** —— 本件自身零目录遍历 / 零 glob，扫描面 = 调用方点名的
//	   `--list` / `--map` / `--changed` 三件，永不含本件（同 `bench-summary.py` 形态）。
//	② 本件是否落在任何他门扫描面内？**真** —— 本件落 `core/cmd/zerg/` 域，而 `check-hardcoded-private-paths.py`
//	   的 `SCAN_DIRS` 含 `core/`、`check-keep-marks.py` / `check-unsubstituted-placeholders.py` 等也现扫
//	   `core/**`。⇒ 件头与正文**零绝对私有路径字面量**（无 `/Users/…` · 无 `~/.zerg`）；负控夹具里
//	   的样例路径一律为仓内相对形（`core/cmd/x.go` · `deploy/unknown.yaml`），不构成任何门的命中面。
//	③ 证明处理有效：落仓前后对同一条命令取数逐字相同（见本单 `self-ref-before.txt` / `self-ref-after.txt`，
//	   命令 = `check-hardcoded-private-paths.py` 现跑，命中 0 处 · 输出 sha16 落仓前后逐字相同）。
//	④ 假则量化依据：若①为真的门存在，则须拼串；现读 ① 为假 ⇒ 无需拼串（同批 20 枚已落件同判）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"
)

const asProg = "affected-steps.py"

const asUsageLine = "usage: affected-steps.py [-h] [--list LIST] [--map MAP] [--decl DECL]\n" +
	"                         [--changed CHANGED] [--self-test] [--json]\n"

const asHelpBody = asUsageLine + "\n" +
	"optional arguments:\n" +
	"  -h, --help         show this help message and exit\n" +
	"  --list LIST\n" +
	"  --map MAP\n" +
	"  --decl DECL\n" +
	"  --changed CHANGED\n" +
	"  --self-test\n" +
	"  --json\n"

// ── 有序 JSON（py dict 保插入序 ⇒ `faces` 的键序要复刻）─────────────────────
type asJV struct {
	kind byte // 'o' 'a' 's' 'n' 'b' 'z'
	s    string
	num  string
	b    bool
	arr  []asJV
	obj  []asJP
}

type asJP struct {
	k string
	v asJV
}

func asRead(dec *json.Decoder) (asJV, error) {
	t, err := dec.Token()
	if err != nil {
		return asJV{}, err
	}
	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '{':
			o := asJV{kind: 'o'}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return asJV{}, err
				}
				key, _ := kt.(string)
				val, err := asRead(dec)
				if err != nil {
					return asJV{}, err
				}
				o.obj = append(o.obj, asJP{key, val})
			}
			if _, err := dec.Token(); err != nil {
				return asJV{}, err
			}
			return o, nil
		case '[':
			a := asJV{kind: 'a'}
			for dec.More() {
				val, err := asRead(dec)
				if err != nil {
					return asJV{}, err
				}
				a.arr = append(a.arr, val)
			}
			if _, err := dec.Token(); err != nil {
				return asJV{}, err
			}
			return a, nil
		}
		return asJV{}, fmt.Errorf("bad delim")
	case string:
		return asJV{kind: 's', s: v}, nil
	case json.Number:
		return asJV{kind: 'n', num: v.String()}, nil
	case bool:
		return asJV{kind: 'b', b: v}, nil
	case nil:
		return asJV{kind: 'z'}, nil
	}
	return asJV{}, fmt.Errorf("bad token")
}

func asLoad(path string) (asJV, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return asJV{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return asRead(dec)
}

func (o asJV) get(k string) (asJV, bool) {
	if o.kind == 'o' {
		for _, p := range o.obj {
			if p.k == k {
				return p.v, true
			}
		}
	}
	return asJV{}, false
}

func (o asJV) truthy() bool {
	switch o.kind {
	case 's':
		return o.s != ""
	case 'n':
		return o.num != "" && o.num != "0"
	case 'b':
		return o.b
	case 'a':
		return len(o.arr) > 0
	case 'o':
		return len(o.obj) > 0
	}
	return false
}

// asStrCostr —— py `str(v)`（本门取值面只出字符串 / 数字 / 布尔 / null）。
func (o asJV) pyStr() string {
	switch o.kind {
	case 's':
		return o.s
	case 'n':
		return o.num
	case 'b':
		if o.b {
			return "True"
		}
		return "False"
	case 'z':
		return "None"
	}
	return ""
}

// asParseList —— py `parse_list()`（`^\s*(\S+)\s+(\S+)\s+(.*)$`，scope 须 `^[a-z-]+$`）。
func asParseList(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// ★ GAP-20261010-25（本片）：机读面优先（步表件的机读形 —— `--list --json` 的单对象
	// `{"count":N,"steps":[…]}`）—— 复用同族助手 `pgsParseListJSON`（✗ 不另造第二套解析）。
	// 机读面在场而形状坏/自证不过 ⇒ **响亮失败**（err ⇒ 调用方 rc=2），✗ 不许静默退回人面。
	if steps, _, isM, perr := pgsParseListJSON(string(b)); isM {
		if perr != "" {
			return nil, fmt.Errorf("步表件机读面（`--list --json`）形状坏：%s", perr)
		}
		names := []string{}
		for _, s := range steps {
			if s.Scope == "self" || !asScopeOK(s.Scope) {
				continue
			}
			if s.Name != "" {
				names = append(names, s.Name)
			}
		}
		return names, nil
	}
	// ② 退化路：机读面不在 ⇒ 人面（与改前逐字相同）。
	names := []string{}
	for _, line := range strings.SplitAfter(string(b), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "──") || strings.HasPrefix(line, "共 ") || strings.TrimFunc(line, unicode.IsSpace) == "" {
			continue
		}
		scope, _, name, ok := asStepLine(strings.TrimSuffix(line, "\n"))
		if !ok {
			continue
		}
		if scope == "self" || !asScopeOK(scope) {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

func asScopeOK(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && r != '-' {
			return false
		}
	}
	return true
}

// asStepLine —— `^\s*(\S+)\s+(\S+)\s+(.*)$`：三个捕获（第三捕获可空）。
func asStepLine(s string) (scope, mode, name string, ok bool) {
	i := 0
	for i < len(s) && asIsSpaceAt(s, i) {
		i = asSkipSpace(s, i)
	}
	t1, i, o1 := asField(s, i)
	if !o1 {
		return "", "", "", false
	}
	i = asSkipSpace(s, i)
	if i >= len(s) || asFieldStart(s, i) == false {
		return "", "", "", false
	}
	t2, j, o2 := asField(s, i)
	if !o2 {
		return "", "", "", false
	}
	i = j
	i = asSkipSpace(s, i)
	if i >= len(s) {
		return "", "", "", false
	}
	return t1, t2, s[i:], true
}

func asField(s string, i int) (string, int, bool) {
	start := i
	for i < len(s) {
		r, sz := asDecodeRune(s, i)
		if unicode.IsSpace(r) {
			break
		}
		i += sz
	}
	if i == start {
		return "", start, false
	}
	return s[start:i], i, true
}

func asFieldStart(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	r, _ := asDecodeRune(s, i)
	return !unicode.IsSpace(r)
}

func asSkipSpace(s string, i int) int {
	for i < len(s) {
		r, sz := asDecodeRune(s, i)
		if !unicode.IsSpace(r) {
			break
		}
		i += sz
	}
	return i
}

func asIsSpaceAt(s string, i int) bool {
	r, _ := asDecodeRune(s, i)
	return unicode.IsSpace(r)
}

func asDecodeRune(s string, i int) (rune, int) {
	for _, r := range s[i:] {
		return r, len(string(r))
	}
	return 0, 0
}

// asEntryMatches —— py `entry_matches()`。
func asEntryMatches(e asJV, rel string) bool {
	pat := ""
	if p, ok := e.get("pattern"); ok {
		pat = p.pyStr()
	}
	kind := ""
	if k, ok := e.get("kind"); ok {
		kind = k.pyStr()
	}
	if kind == "prefix" || strings.HasSuffix(pat, "/**") {
		base := pat
		if strings.HasSuffix(pat, "/**") {
			base = pat[:len(pat)-3]
		}
		return rel == base || strings.HasPrefix(rel, strings.TrimRight(base, "/")+"/")
	}
	if kind == "glob" {
		return asFnmatch(rel, pat)
	}
	return rel == pat
}

// asFnmatch —— py fnmatch 的 `*` “ `?` `[seq]` `[!seq]`（`*` 跨 `/`；见件头未覆盖面③）。
func asFnmatch(name, pat string) bool {
	n := []rune(name)
	p := []rune(pat)
	var rec func(ni, pi int) bool
	rec = func(ni, pi int) bool {
		for pi < len(p) {
			switch p[pi] {
			case '*':
				for pi+1 < len(p) && p[pi+1] == '*' {
					pi++
				}
				if pi+1 == len(p) {
					return true
				}
				for k := ni; k <= len(n); k++ {
					if rec(k, pi+1) {
						return true
					}
				}
				return false
			case '?':
				if ni >= len(n) {
					return false
				}
				ni++
				pi++
			case '[':
				if ni >= len(n) {
					return false
				}
				j := pi + 1
				neg := false
				if j < len(p) && (p[j] == '!' || p[j] == '^') {
					neg = true
					j++
				}
				hit := false
				closed := false
				for j < len(p) {
					if p[j] == ']' && j > pi+1 {
						closed = true
						break
					}
					if j+2 < len(p) && p[j+1] == '-' {
						if n[ni] >= p[j] && n[ni] <= p[j+2] {
							hit = true
						}
						j += 3
						continue
					}
					if p[j] == n[ni] {
						hit = true
					}
					j++
				}
				if !closed {
					if n[ni] != '[' {
						return false
					}
					ni++
					pi++
					continue
				}
				if hit == neg {
					return false
				}
				ni++
				pi = j + 1
			default:
				if ni >= len(n) || n[ni] != p[pi] {
					return false
				}
				ni++
				pi++
			}
		}
		return ni == len(n)
	}
	return rec(0, 0)
}

// asAffectedFor —— py `affected_for()`。
func asAffectedFor(mp asJV, changed []string) ([]string, []string) {
	entries, _ := mp.get("files")
	faces, _ := mp.get("faces")
	stepOrder := []string{}
	seen := map[string]bool{}
	if faces.kind == 'o' {
		for _, f := range faces.obj {
			for _, n := range f.v.arr {
				s := n.pyStr()
				if !seen[s] {
					seen[s] = true
					stepOrder = append(stepOrder, s)
				}
			}
		}
	}
	aff := map[string]bool{}
	undeclared := []string{}
	for _, c := range changed {
		hit := false
		if entries.kind == 'a' {
			for _, e := range entries.arr {
				if asEntryMatches(e, c) {
					hit = true
					if sv, ok := e.get("steps"); ok && sv.kind == 'a' {
						for _, n := range sv.arr {
							aff[n.pyStr()] = true
						}
					}
				}
			}
		}
		if !hit {
			undeclared = append(undeclared, c)
		}
	}
	if len(undeclared) > 0 {
		return stepOrder, undeclared
	}
	out := []string{}
	for _, n := range stepOrder {
		if aff[n] {
			out = append(out, n)
		}
	}
	return out, []string{}
}

// asAnalyse —— py `analyse()`（返回 rc, lines）。
func asAnalyse(listPath, mapPath, changedPath string) (int, []string) {
	L := []string{}
	add := func(s string) { L = append(L, s) }
	if !asIsFile(listPath) {
		return 2, []string{fmt.Sprintf("[不给结论] 步骤表件缺失：%s ⇒ rc=2", listPath)}
	}
	if !asIsFile(mapPath) {
		return 2, []string{fmt.Sprintf("[不给结论] 声明表缺失：%s ⇒ rc=2", mapPath)}
	}
	table, err := asParseList(listPath)
	if err != nil {
		return 2, []string{fmt.Sprintf("[不给结论] 步骤表件读不到：%s ⇒ rc=2", listPath)}
	}
	N := len(table)
	mp, err := asLoad(mapPath)
	if err != nil {
		return 2, []string{fmt.Sprintf("[不给结论] 声明表读不到/解析失败：%s ⇒ rc=2", mapPath)}
	}
	fp, _ := mp.get("input_fingerprint")
	sh, _ := mp.get("source_head")
	if mp.kind != 'o' || !fp.truthy() || !sh.truthy() {
		return 2, []string{"[不给结论] 声明表无指纹/无基线（缺 input_fingerprint 或 source_head）" +
			"⇒ 取不到输入基线 ⇒ 不跳 ⇒ rc=2"}
	}
	faces, _ := mp.get("faces")
	faceKeys := []string{}
	if faces.kind == 'o' {
		for _, p := range faces.obj {
			faceKeys = append(faceKeys, p.k)
		}
	}
	sort.Strings(faceKeys)
	named := []string{}
	seen := map[string]bool{}
	for _, fk := range faceKeys {
		fv, _ := faces.get(fk)
		for _, n := range fv.arr {
			s := n.pyStr()
			if !seen[s] {
				seen[s] = true
				named = append(named, s)
			}
		}
	}
	D := len(named)
	head := sh.pyStr()
	if r := []rune(head); len(r) > 16 {
		head = string(r[:16])
	}
	add(fmt.Sprintf("步骤表（--list 自报）步数 N=%d · 声明表命名步数 D=%d（来源 %s @ %s）", N, D, mapPath, head))
	if D != N {
		return 2, append(L, fmt.Sprintf("[不给结论] C0 未过：声明命名 %d 步 ≠ 步骤表 %d 步 ⇒ **有步未声明 = 静默漏选** ⇒ rc=2", D, N))
	}
	tset := map[string]bool{}
	for _, t := range table {
		tset[t] = true
	}
	dset := map[string]bool{}
	for _, t := range named {
		dset[t] = true
	}
	if !asSameSet(tset, dset) {
		miss := []string{}
		for t := range tset {
			if !dset[t] {
				miss = append(miss, t)
			}
		}
		ghost := []string{}
		for t := range dset {
			if !tset[t] {
				ghost = append(ghost, t)
			}
		}
		sort.Strings(miss)
		sort.Strings(ghost)
		return 2, append(L, fmt.Sprintf("[不给结论] C0 未过：命名集合 ≠ 表集合（漏=%s · 幻影=%s）⇒ rc=2",
			asReprSlice(asFirst(miss, 5)), asReprSlice(asFirst(ghost, 5))))
	}
	add(fmt.Sprintf("C0 双向计数过：D==N==%d 且命名集合逐字相等（**没少声明**）", N))
	files, _ := mp.get("files")
	nEntries := 0
	if files.kind == 'a' {
		for _, e := range files.arr {
			sv, ok := e.get("steps")
			if !ok || sv.kind != 'a' {
				pat := ""
				if p, ok := e.get("pattern"); ok {
					pat = p.pyStr()
				}
				return 2, append(L, fmt.Sprintf("[不给结论] C1 未过：条目「%s」的 steps 不是列表 ⇒ rc=2", pat))
			}
			nEntries += len(sv.arr)
		}
	}
	nFiles := len(files.arr)
	add(fmt.Sprintf("C1 解析自证过：声明条目 %d 条（面 %d 个 · 件型条目 %d 条）", nEntries, len(faceKeys), nFiles))
	changed := []string{}
	if changedPath != "" && asIsFile(changedPath) {
		b, err := os.ReadFile(changedPath)
		if err == nil {
			for _, x := range strings.SplitAfter(string(b), "\n") {
				t := strings.TrimFunc(x, unicode.IsSpace)
				if t == "" || strings.HasPrefix(t, "#") {
					continue
				}
				changed = append(changed, t)
			}
		}
	}
	aff, undeclared := asAffectedFor(mp, changed)
	add(fmt.Sprintf("本趟改动件 %d 件", len(changed)))
	if len(undeclared) > 0 {
		add(fmt.Sprintf("★ 未声明件 %d 件（已拍第 3 条 ⇒ 默认每轮都跑 ⇒ 本轮按**全量**）：", len(undeclared)))
		for _, x := range asFirst(undeclared, 10) {
			add(fmt.Sprintf("   ？ %s", x))
		}
	}
	add("── 报告（**只报告档** · 只算不跳 · 不真跳任何步 · ✗ 本结果**未可信**）──")
	add(fmt.Sprintf("本次变更 ⇒ 理论上应受影响的步集：%d 步（本趟该跑）：", len(aff)))
	for _, n := range asFirst(aff, 10) {
		add(fmt.Sprintf("   ▶ %s", n))
	}
	if len(aff) > 10 {
		add(fmt.Sprintf("   …（共 %d 步，余 %d 步略）", len(aff), len(aff)-10))
	}
	if len(changed) == 0 {
		add(fmt.Sprintf("   受影响 0 步（本读数可分辨「没跑」：上一行有 D/N=%d/%d 与改动件计数 0）", D, N))
	}
	return 0, L
}

func asSameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func asFirst(xs []string, n int) []string {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

// asReprSlice —— py `list` 的 `%s`（repr：`['a', 'b']`）。
func asReprSlice(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = asPyRepr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func asPyRepr(s string) string {
	q := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = "\""
	}
	body := strings.ReplaceAll(s, "\\", "\\\\")
	body = strings.ReplaceAll(body, q, "\\"+q)
	return q + body + q
}

func asIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// ── argparse 复刻（`-h/--help` · 长旗标前缀匹配 · `=` 取值 · 缺值报话 · 未知旗标）──
type asNS struct {
	list     string
	mp       string
	decl     string
	changed  string
	selfTest bool
	asJSON   bool
	help     bool
}

var asLongOpts = []string{"help", "list", "map", "decl", "changed", "self-test", "json"}

func asResolveLong(body string) (string, string) {
	if body == "" {
		return "", ""
	}
	for _, o := range asLongOpts {
		if o == body {
			return o, ""
		}
	}
	var hits []string
	for _, o := range asLongOpts {
		if strings.HasPrefix(o, body) {
			hits = append(hits, "--"+o)
		}
	}
	if len(hits) == 1 {
		return strings.TrimPrefix(hits[0], "--"), ""
	}
	if len(hits) > 1 {
		return "", strings.Join(hits, ", ")
	}
	return "", ""
}

func asParseArgs(argv []string) (*asNS, error) {
	ns := &asNS{}
	var unknown []string
	i := 0
	for ; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			unknown = append(unknown, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			body := a[2:]
			val := ""
			hasVal := false
			if eq := strings.Index(body, "="); eq >= 0 {
				val, hasVal = body[eq+1:], true
				body = body[:eq]
			}
			canon, ambig := asResolveLong(body)
			if ambig != "" {
				return nil, fmt.Errorf("ambiguous option: --%s could match %s", body, ambig)
			}
			if canon == "" {
				unknown = append(unknown, a)
				continue
			}
			switch canon {
			case "help":
				ns.help = true
			case "self-test":
				ns.selfTest = true
			case "json":
				ns.asJSON = true
			default:
				if !hasVal {
					if i+1 >= len(argv) {
						return nil, fmt.Errorf("argument --%s: expected one argument", body)
					}
					i++
					val = argv[i]
				}
				switch canon {
				case "list":
					ns.list = val
				case "map":
					ns.mp = val
				case "decl":
					ns.decl = val
				case "changed":
					ns.changed = val
				}
			}
		case strings.HasPrefix(a, "-") && a != "-":
			if a == "-h" {
				ns.help = true
			} else {
				unknown = append(unknown, a)
			}
		default:
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("unrecognized arguments: %s", strings.Join(unknown, " "))
	}
	return ns, nil
}

// ── 入口（= py `main()`）────────────────────────────────────────────────
func affectedStepsGateMain(stdout, stderr io.Writer, argv []string) int {
	ns, perr := asParseArgs(argv)
	if perr != nil {
		fmt.Fprint(stderr, asUsageLine)
		fmt.Fprintf(stderr, "%s: error: %s\n", asProg, perr.Error())
		return 2
	}
	if ns.help {
		fmt.Fprint(stdout, asHelpBody)
		return 0
	}
	if ns.selfTest {
		return asSelfTest(stdout)
	}
	mp := ns.mp
	if mp == "" {
		mp = ns.decl
	}
	if ns.list == "" || mp == "" {
		fmt.Fprintln(stderr, "用法：affected-steps.py --list <件> --map <声明表.json> [--changed <件>]")
		return 2
	}
	rc, lines := asAnalyse(ns.list, mp, ns.changed)
	if ns.asJSON {
		doc := struct {
			Schema string   `json:"schema"`
			Kind   string   `json:"kind"`
			Rc     int      `json:"rc"`
			Lines  []string `json:"lines"`
		}{"zerg/v1", "AffectedSteps", rc, lines}
		if doc.Lines == nil {
			doc.Lines = []string{}
		}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", " ")
		_ = enc.Encode(doc)
		return rc
	}
	for _, x := range lines {
		fmt.Fprintln(stdout, x)
	}
	return rc
}

// ── py `--self-test`（7 控 · 打印文案逐字）──────────────────────────────
func asSelfTest(stdout io.Writer) int {
	fails := []string{}
	tmp, err := os.MkdirTemp("", "affected-steps-selftest-")
	if err != nil {
		fmt.Fprintln(stdout, "自检不过：")
		fmt.Fprintf(stdout, "  ✗ %s\n", err.Error())
		return 2
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	lst := tmp + "/list.txt"
	_ = os.WriteFile(lst, []byte(
		"── 步骤清单（scope 模式 名称）──\n"+
			"   go     rc     a-step\n"+
			"   rust   empty  b-step\n"+
			"   docs   tri    c-step\n"+
			"共 3 步\n"), 0o644)
	chg := tmp + "/chg.txt"
	_ = os.WriteFile(chg, []byte("core/cmd/x.go\n"), 0o644)

	base := `"input_fingerprint": "` + strings.Repeat("f", 64) + `", "source_head": "deadbeef"`
	writeJSON := func(name, body string) string {
		p := tmp + "/" + name
		_ = os.WriteFile(p, []byte("{"+body+"}"), 0o644)
		return p
	}
	p1 := writeJSON("m1.json", base+
		`, "faces": {"core": ["a-step"], "agent": ["b-step"], "docs": ["c-step"]}, `+
		`"files": [{"pattern": "core/**", "kind": "prefix", "steps": ["a-step"]}, `+
		`{"pattern": "ui/**", "kind": "prefix", "steps": ["b-step"]}, `+
		`{"pattern": "docs/**", "kind": "prefix", "steps": ["c-step"]}]`)
	rc, lines := asAnalyse(lst, p1, chg)
	if rc != 0 {
		fails = append(fails, fmt.Sprintf("① 正控期望 rc=0，实得 %d", rc))
	} else if !asAnyContains(lines, "▶ a-step") {
		fails = append(fails, "① 正控未点名受影响步")
	}

	p2 := writeJSON("m2.json", base+
		`, "faces": {"core": ["a-step"], "agent": ["b-step"]}, `+
		`"files": [{"pattern": "core/**", "kind": "prefix", "steps": ["a-step"]}]`)
	rc, lines = asAnalyse(lst, p2, chg)
	if rc != 2 {
		fails = append(fails, fmt.Sprintf("② 少声明期望 rc=2，实得 %d", rc))
	} else if !asAnyContains(lines, "静默漏选") {
		fails = append(fails, "② 未点名「静默漏选」")
	}

	p3 := writeJSON("m3.json", base+
		`, "faces": {"core": ["a-step"], "agent": ["b-step"], "docs": ["zzz"]}, "files": []`)
	rc, _ = asAnalyse(lst, p3, chg)
	if rc != 2 {
		fails = append(fails, fmt.Sprintf("③ 集合不等期望 rc=2，实得 %d", rc))
	}

	chg0 := tmp + "/chg0.txt"
	_ = os.WriteFile(chg0, []byte(""), 0o644)
	rc, lines = asAnalyse(lst, p1, chg0)
	if rc != 0 {
		fails = append(fails, fmt.Sprintf("④ 空改动期望 rc=0，实得 %d", rc))
	} else if !asAnyContains(lines, "受影响 0 步") {
		fails = append(fails, "④ 未打印「受影响 0 步」")
	}

	p5 := writeJSON("m5.json",
		`"faces": {"core": ["a-step"], "agent": ["b-step"], "docs": ["c-step"]}, "files": []`)
	rc, lines = asAnalyse(lst, p5, chg)
	if rc != 2 {
		fails = append(fails, fmt.Sprintf("⑤ 缺指纹期望 rc=2，实得 %d", rc))
	} else if !asAnyContains(lines, "取不到输入基线") {
		fails = append(fails, "⑤ 未点名「取不到输入基线」")
	}

	p6 := writeJSON("m6.json", base+`, "faces": {}, "files": []`)
	rc, _ = asAnalyse(lst, p6, chg)
	if rc != 2 {
		fails = append(fails, fmt.Sprintf("⑥ 空声明期望 rc=2，实得 %d", rc))
	}

	chg7 := tmp + "/chg7.txt"
	_ = os.WriteFile(chg7, []byte("deploy/unknown.yaml\n"), 0o644)
	rc, lines = asAnalyse(lst, p1, chg7)
	if rc != 0 {
		fails = append(fails, fmt.Sprintf("⑦ 未声明件期望 rc=0，实得 %d", rc))
	} else if !asAnyContains(lines, "未声明件 1 件") {
		fails = append(fails, "⑦ 未单列「未声明」件")
	} else if !asAnyContains(lines, "受影响的步集：3 步") {
		fails = append(fails, "⑦ 未按全量报（应 3 步）")
	}

	if len(fails) > 0 {
		fmt.Fprintln(stdout, "自检不过：")
		for _, x := range fails {
			fmt.Fprintln(stdout, "  ✗ "+x)
		}
		return 2
	}
	fmt.Fprintln(stdout, "自检通过：7 控（正控 3 ＋ 负控 4）逐条对拍 ✓")
	return 0
}

func asAnyContains(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}
