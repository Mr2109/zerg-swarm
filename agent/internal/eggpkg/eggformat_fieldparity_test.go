// eggformat_fieldparity_test.go —— F24（GAP-20261015-13）的**对拍判据**：同一份 `*.egg` 格式的
// **字段面 / 键面 / 判据面**在两侧读端必须一致；不一致的只许是**本表已具名登记的分叉**（ledger 格）。
//
// 为什么要它（本单取证结论）：格式在两处各有一份 Go 实现 ——
//
//	A 侧：agent/internal/eggpkg（`eggpkg.go` 的 v1 读端 ＋ `expand.go` 的 v2 读端/展开器）
//	B 侧：core/cmd/zerg（`zerg_egg.go` 的 v1/v2 读端 ＋ `zerg_egg_verify.go` 的校验/重建
//	     ＋ `zerg_egg_version.go` 的面分类/版本闸）
//
// 两份**不完全重叠**（B 独有写端/校验缓存/版本闸/闭包；A 独有落盘展开/进度/幂等凭据）⇒ 强行合并会掉功能。
// 故本单**不合**，改为把「加字段只加一边」这件病钉成判据：任何一侧新增头槽字段 / 段 JSON 键 / 索引键 /
// path 规则而不改另一侧（或不同步更新下面的 ledger）⇒ 本判据当场红并点名该字段。
//
// 判据形态（照仓内冻结表惯例）：
//
//   - kind=equal 的格：两侧现读值必须**逐字相同**；
//   - kind=ledger 的格：两侧现读值必须等于**已登记分叉**的冻结值（任一侧变了 ⇒ 红，逼重新登记）。
//
// ★ 本件只读源码（不写仓 · 不改两侧任何行为）；键规格真源仍是 B 侧两张表（`EggSpecGate` 管它们）。
package eggpkg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// eggFieldCell —— 一格对拍读数。equal：A 必须 == B；ledger：A==WantA 且 B==WantB。
type eggFieldCell struct {
	No    int
	Name  string
	Kind  string // "equal" | "ledger"
	A     string // A 侧（eggpkg）现读值
	B     string // B 侧（zerg_egg）现读值
	WantA string // ledger 格的冻结值（equal 格留空）
	WantB string
	OK    bool
	Why   string
}

// eggSrcFacts —— 一件 Go 源码里本判据要用的几类事实（全部机械抽取 · ✗ 不手打）。
type eggSrcFacts struct {
	Consts   map[string]ast.Expr
	Structs  map[string][]string // 类型名 → 字段名（顺序）
	Tags     map[string][]string // 类型名 → json 键名（顺序）
	VarStrs  map[string][]string // 变量名 → 复合字面量里 `Name: "<串>"` 的值
	VarInts  map[string][]int64  // 变量名 → 复合字面量里整数字面量键（map 键）
	Funcs    map[string]*ast.FuncDecl
	FuncText map[string]string // 函数名 → 体内逐字源码
	Strings  map[string][]string
}

func eggFactsOf(path string) (*eggSrcFacts, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读不到 %s：%w", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("解析 %s：%w", path, err)
	}
	fa := &eggSrcFacts{
		Consts:   map[string]ast.Expr{},
		Structs:  map[string][]string{},
		Tags:     map[string][]string{},
		VarStrs:  map[string][]string{},
		VarInts:  map[string][]int64{},
		Funcs:    map[string]*ast.FuncDecl{},
		FuncText: map[string]string{},
		Strings:  map[string][]string{},
	}
	for _, d := range f.Decls {
		switch dd := d.(type) {
		case *ast.GenDecl:
			for _, sp := range dd.Specs {
				switch s := sp.(type) {
				case *ast.ValueSpec:
					for i, n := range s.Names {
						if dd.Tok == token.CONST && len(s.Values) > 0 {
							v := s.Values[len(s.Values)-1]
							if i < len(s.Values) {
								v = s.Values[i]
							}
							fa.Consts[n.Name] = v
						}
						if dd.Tok == token.VAR && len(s.Values) > 0 {
							eggScanComposite(n.Name, s.Values[0], fa)
						}
					}
				case *ast.TypeSpec:
					st, ok := s.Type.(*ast.StructType)
					if !ok || st.Fields == nil {
						continue
					}
					var names, tags []string
					for _, fd := range st.Fields.List {
						for _, n := range fd.Names {
							names = append(names, n.Name)
						}
						if fd.Tag != nil {
							raw, err := strconv.Unquote(fd.Tag.Value)
							if err != nil {
								continue
							}
							if k := eggJSONKeyOf(raw); k != "" {
								tags = append(tags, k)
							}
						}
					}
					fa.Structs[s.Name.Name] = names
					fa.Tags[s.Name.Name] = tags
				}
			}
		case *ast.FuncDecl:
			fa.Funcs[dd.Name.Name] = dd
			if dd.Body != nil {
				if lo, hi := fset.Position(dd.Body.Lbrace).Offset, fset.Position(dd.Body.Rbrace).Offset; lo >= 0 && hi > lo && hi <= len(src) {
					fa.FuncText[dd.Name.Name] = string(src[lo:hi])
				}
				ast.Inspect(dd.Body, func(n ast.Node) bool {
					if bl, ok := n.(*ast.BasicLit); ok && bl.Kind == token.STRING {
						if v, err := strconv.Unquote(bl.Value); err == nil {
							fa.Strings[dd.Name.Name] = append(fa.Strings[dd.Name.Name], v)
						}
					}
					return true
				})
			}
		}
	}
	return fa, nil
}

// eggScanComposite —— 记 `var x = []T{{Name:"…"},…}` / `map[int]bool{0o644:true,…}` 的元素。
func eggScanComposite(name string, e ast.Expr, fa *eggSrcFacts) {
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return
	}
	var walk func(el ast.Expr)
	walk = func(el ast.Expr) {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			// 元素本身可能是一枚**无类型**结构字面量（`[]T{{Name:"…"},…}`）⇒ 下钻一层。
			if inner, ok := el.(*ast.CompositeLit); ok {
				for _, x := range inner.Elts {
					walk(x)
				}
			}
			return
		}
		if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Name" {
			if bl, ok := kv.Value.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				if v, err := strconv.Unquote(bl.Value); err == nil {
					fa.VarStrs[name] = append(fa.VarStrs[name], v)
				}
			}
			return
		}
		if bl, ok := kv.Key.(*ast.BasicLit); ok && bl.Kind == token.INT {
			if n, ok := eggEvalInt(bl, nil, 0); ok {
				fa.VarInts[name] = append(fa.VarInts[name], n)
			}
		}
	}
	for _, el := range cl.Elts {
		walk(el)
	}
}

func eggJSONKeyOf(tag string) string {
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "json:") {
			v := strings.TrimPrefix(part, "json:")
			if i := strings.IndexByte(v, ','); i >= 0 {
				v = v[:i]
			}
			v = strings.Trim(v, "\"")
			if v == "" || v == "-" {
				return ""
			}
			return v
		}
	}
	return ""
}

// eggEvalInt —— 求常量整数值（支持 INT 字面量 / 括号 / 标识符 / `<<` `|` `+` `*`）。
func eggEvalInt(e ast.Expr, env map[string]ast.Expr, depth int) (int64, bool) {
	if depth > 16 {
		return 0, false
	}
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.INT {
			return 0, false
		}
		n, err := strconv.ParseInt(v.Value, 0, 64)
		return n, err == nil
	case *ast.ParenExpr:
		return eggEvalInt(v.X, env, depth+1)
	case *ast.Ident:
		if env != nil {
			if x, ok := env[v.Name]; ok {
				return eggEvalInt(x, env, depth+1)
			}
		}
		return 0, false
	case *ast.BinaryExpr:
		a, ok := eggEvalInt(v.X, env, depth+1)
		if !ok {
			return 0, false
		}
		b, ok := eggEvalInt(v.Y, env, depth+1)
		if !ok {
			return 0, false
		}
		switch v.Op {
		case token.SHL:
			return a << uint(b), true
		case token.ADD:
			return a + b, true
		case token.OR:
			return a | b, true
		case token.MUL:
			return a * b, true
		}
	}
	return 0, false
}

func eggConstInt(fa *eggSrcFacts, name string) (int64, bool) {
	e, ok := fa.Consts[name]
	if !ok {
		return 0, false
	}
	return eggEvalInt(e, fa.Consts, 0)
}

func eggConstStr(fa *eggSrcFacts, name string) (string, bool) {
	e, ok := fa.Consts[name]
	if !ok {
		return "", false
	}
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	return s, err == nil
}

// eggMustInt / eggMustStr —— 抽不到 ⇒ fail-closed 报错（✗ 不静默当 0/空）。
func eggMustInt(fa *eggSrcFacts, where, name string) (int64, error) {
	n, ok := eggConstInt(fa, name)
	if !ok {
		return 0, fmt.Errorf("%s 里抽不到常量 %s（判据抽空 ⇒ 不给结论）", where, name)
	}
	return n, nil
}

func eggMustStr(fa *eggSrcFacts, where, name string) (string, error) {
	s, ok := eggConstStr(fa, name)
	if !ok {
		return "", fmt.Errorf("%s 里抽不到字符串常量 %s（判据抽空 ⇒ 不给结论）", where, name)
	}
	return s, nil
}

// eggOffsetsOf —— 抽 `<prefix>Name = N` 形状的偏移常量表（name 去前缀后小写化）。
func eggOffsetsOf(fa *eggSrcFacts, prefix string) (map[string]int64, error) {
	out := map[string]int64{}
	for name, e := range fa.Consts {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		n, ok := eggEvalInt(e, fa.Consts, 0)
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimPrefix(name, prefix))] = n
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("抽不到 %s* 偏移常量表（判据抽空 ⇒ 不给结论）", prefix)
	}
	return out, nil
}

func eggMapStr(m map[string]int64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(" · ")
		}
		fmt.Fprintf(&b, "%s=%d", k, m[k])
	}
	return b.String()
}

func eggSortedStr(xs []string) string {
	c := append([]string(nil), xs...)
	sort.Strings(c)
	return strings.Join(c, " · ")
}

func eggSortedInt(xs []int64) string {
	c := append([]int64(nil), xs...)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	parts := make([]string, len(c))
	for i, n := range c {
		parts[i] = fmt.Sprintf("0o%o", n)
	}
	return strings.Join(parts, " · ")
}

// eggPathRulesA —— A 侧（`eggRefusePath`）的规则名集（从报话里的 `命中规则 <name>` 抽）。
func eggPathRulesA(fa *eggSrcFacts) ([]string, error) {
	var out []string
	for _, s := range fa.Strings["eggRefusePath"] {
		if i := strings.Index(s, "命中规则 "); i >= 0 {
			rest := s[i+len("命中规则 "):]
			if j := strings.IndexAny(rest, "（("); j >= 0 {
				rest = rest[:j]
			}
			if rest = strings.TrimSpace(rest); rest != "" {
				out = append(out, rest)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("A 侧 eggRefusePath 里抽不到规则名（判据抽空 ⇒ 不给结论）")
	}
	return out, nil
}

// eggFuncInts —— 一个函数体内全部整数字面量（去重排序）；用于把「掩码式判据」钉成读数。
func eggFuncInts(fa *eggSrcFacts, fn string) (string, error) {
	fd, ok := fa.Funcs[fn]
	if !ok || fd.Body == nil {
		return "", fmt.Errorf("抽不到函数 %s（判据抽空 ⇒ 不给结论）", fn)
	}
	seen := map[int64]bool{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if bl, ok := n.(*ast.BasicLit); ok && bl.Kind == token.INT {
			if v, ok := eggEvalInt(bl, nil, 0); ok && v != 0 {
				seen[v] = true
			}
		}
		return true
	})
	if len(seen) == 0 {
		return "", fmt.Errorf("函数 %s 里抽不到整数字面量（判据抽空 ⇒ 不给结论）", fn)
	}
	xs := make([]int64, 0, len(seen))
	for v := range seen {
		xs = append(xs, v)
	}
	return "禁位/掩码 " + eggSortedInt(xs), nil
}

// eggFuncMarkers —— 函数体内是否真含这几种判据串（逐条点名 · 按函数体内逐字源码判）。
func eggFuncMarkers(fa *eggSrcFacts, fns []string, probes []string) string {
	var sb strings.Builder
	for _, fn := range fns {
		body, ok := fa.FuncText[fn]
		if !ok {
			return "函数缺失"
		}
		sb.WriteString(body)
		sb.WriteByte('\n')
	}
	body := sb.String()
	var have []string
	for _, p := range probes {
		if strings.Contains(body, p) {
			have = append(have, p)
		}
	}
	if len(have) == 0 {
		return "无（0 项）"
	}
	return fmt.Sprintf("%d 项：%s", len(have), strings.Join(have, " | "))
}

// eggCountFuncs —— 这些函数名在件里**有定义**的个数（机械读数）。
func eggCountFuncs(fa *eggSrcFacts, names []string) int {
	n := 0
	for _, name := range names {
		if _, ok := fa.Funcs[name]; ok {
			n++
		}
	}
	return n
}

// eggFieldParityCells —— 现读两侧源码，产出对拍格（root = 仓根）。
func eggFieldParityCells(root string) ([]eggFieldCell, error) {
	faV1, err := eggFactsOf(filepath.Join(root, "agent", "internal", "eggpkg", "eggpkg.go"))
	if err != nil {
		return nil, err
	}
	faV2, err := eggFactsOf(filepath.Join(root, "agent", "internal", "eggpkg", "expand.go"))
	if err != nil {
		return nil, err
	}
	fb, err := eggFactsOf(filepath.Join(root, "core", "cmd", "zerg", "zerg_egg.go"))
	if err != nil {
		return nil, err
	}
	fbVer, err := eggFactsOf(filepath.Join(root, "core", "cmd", "zerg", "zerg_egg_version.go"))
	if err != nil {
		return nil, err
	}

	var cells []eggFieldCell
	add := func(name, a, b string) {
		cells = append(cells, eggFieldCell{
			No: len(cells) + 1, Name: name, Kind: "equal", A: a, B: b, OK: a == b,
		})
	}
	addLedger := func(name, a, b, wantA, wantB, why string) {
		cells = append(cells, eggFieldCell{
			No: len(cells) + 1, Name: name, Kind: "ledger",
			A: a, B: b, WantA: wantA, WantB: wantB, Why: why,
			OK: a == wantA && b == wantB,
		})
	}
	mi := eggMustInt
	ms := eggMustStr

	magicA, err := ms(faV1, "eggpkg.go", "Magic")
	if err != nil {
		return nil, err
	}
	magicB, err := ms(fb, "zerg_egg.go", "zegMagic")
	if err != nil {
		return nil, err
	}
	add("magic 取值", magicA, magicB)

	prefA, err := mi(faV1, "eggpkg.go", "HeaderPrefix")
	if err != nil {
		return nil, err
	}
	prefB, err := mi(fb, "zerg_egg.go", "zegHeaderPrefix")
	if err != nil {
		return nil, err
	}
	add("v1 固定前缀字节数", strconv.FormatInt(prefA, 10), strconv.FormatInt(prefB, 10))

	v1A, err := mi(faV1, "eggpkg.go", "FormatVersion")
	if err != nil {
		return nil, err
	}
	v1B, err := mi(fb, "zerg_egg.go", "zegFormatVerV1")
	if err != nil {
		return nil, err
	}
	add("v1 布局主号", strconv.FormatInt(v1A, 10), strconv.FormatInt(v1B, 10))

	v2A, err := mi(faV2, "expand.go", "eggLayoutV2")
	if err != nil {
		return nil, err
	}
	v2B, err := mi(fb, "zerg_egg.go", "zegV2LayoutVer")
	if err != nil {
		return nil, err
	}
	add("v2 布局主号", strconv.FormatInt(v2A, 10), strconv.FormatInt(v2B, 10))

	// 五枚布局尺寸/上界：任一偏一处即两侧读同一枚蛋会得到不同结论。
	pairs := [][3]string{
		{"固定头字节数", "eggHeaderBytes", "zegV2HeaderBytes"},
		{"头槽字节数", "eggSlotBytes", "zegV2SlotBytes"},
		{"段表对齐", "eggTableAlign", "zegV2TableAlign"},
		{"段表上界", "eggTableMaxBytes", "zegV2TableMaxBytes"},
		{"段对齐下限", "eggMinAlign", "zegMinAlign"},
	}
	for _, p := range pairs {
		a, err := mi(faV2, "expand.go", p[1])
		if err != nil {
			return nil, err
		}
		b, err := mi(fb, "zerg_egg.go", p[2])
		if err != nil {
			return nil, err
		}
		add(p[0], strconv.FormatInt(a, 10), strconv.FormatInt(b, 10))
	}

	// ★ 加字段的第一现场：头槽偏移表。B 侧新增 `zegV2OffXxx` 而 A 侧没跟 ⇒ 本格红并点名。
	offA, err := eggOffsetsOf(faV2, "eggOff")
	if err != nil {
		return nil, err
	}
	offB, err := eggOffsetsOf(fb, "zegV2Off")
	if err != nil {
		return nil, err
	}
	add("v2 头槽字段偏移表", eggMapStr(offA), eggMapStr(offB))

	// ★ 加字段的第二现场：槽结构（解码进内存的字段名集）。
	slotA, ok := faV2.Structs["eggSlot"]
	if !ok {
		return nil, fmt.Errorf("expand.go 里找不到结构 eggSlot（判据抽空）")
	}
	slotB, ok := fb.Structs["zegV2Slot"]
	if !ok {
		return nil, fmt.Errorf("zerg_egg.go 里找不到结构 zegV2Slot（判据抽空）")
	}
	add("v2 头槽结构字段名集", eggSortedStr(slotA), eggSortedStr(slotB))

	// ★ 加字段的第三现场：段 JSON 键（写端加键而读端不认 ⇒ 静默丢字段）。
	segA, ok := faV2.Tags["EggSegment"]
	if !ok {
		return nil, fmt.Errorf("expand.go 里找不到 EggSegment（判据抽空）")
	}
	segB, ok := fb.Tags["zegEntryJSON"]
	if !ok {
		return nil, fmt.Errorf("zerg_egg.go 里找不到 zegEntryJSON（判据抽空）")
	}
	add("段 JSON 键集", eggSortedStr(segA), eggSortedStr(segB))

	// 第四现场：索引/段表顶层键（v1 的 Index 与 v2 的 eggTableJSON 都算）。
	topA1, ok := faV1.Tags["Index"]
	if !ok {
		return nil, fmt.Errorf("eggpkg.go 里找不到 Index（判据抽空）")
	}
	topA2, ok := faV2.Tags["eggTableJSON"]
	if !ok {
		return nil, fmt.Errorf("expand.go 里找不到 eggTableJSON（判据抽空）")
	}
	if eggSortedStr(topA1) != eggSortedStr(topA2) {
		return nil, fmt.Errorf("A 侧 v1 索引键集（%s）与 v2 段表键集（%s）已自相分叉",
			eggSortedStr(topA1), eggSortedStr(topA2))
	}
	topB, ok := fb.Tags["zegIndexJSON"]
	if !ok {
		return nil, fmt.Errorf("zerg_egg.go 里找不到 zegIndexJSON（判据抽空）")
	}
	add("索引/段表顶层 JSON 键集", eggSortedStr(topA1), eggSortedStr(topB))

	// path 规则集（A 侧 v2 读端已接满 12 条；任一增删而不对称 ⇒ 红）。
	pA, err := eggPathRulesA(faV2)
	if err != nil {
		return nil, err
	}
	pB := fb.VarStrs["zegPathRules"]
	if len(pB) == 0 {
		return nil, fmt.Errorf("zerg_egg.go 里抽不到 zegPathRules 的 Name 表（判据抽空）")
	}
	add("path 规则名集（v2 读端）", eggSortedStr(pA), eggSortedStr(pB))

	// ---- ledger：已登记的**真分叉**（不合，且不许无声扩大）-------------------------------------
	litA, err := eggFuncInts(faV2, "eggModeOf")
	if err != nil {
		return nil, err
	}
	modeB := eggSortedInt(fb.VarInts["zegModeSafeSet"])
	if modeB == "" {
		return nil, fmt.Errorf("zerg_egg.go 里抽不到 zegModeSafeSet（判据抽空）")
	}
	addLedger("段 mode 值域", litA, modeB,
		"禁位/掩码 0o444 · 0o777 · 0o7000", "0o644 · 0o700 · 0o755",
		"A 更松：只禁 setuid/setgid/sticky 且要求有读位 ⇒ 0666/0777 这类 B 拒、A 收（A 展开时会写出 0777）；"+
			"B 是安全子集闭集 {0644,0755,0700}。★ 本单只登记、不改行为；收敛方向＝A 对齐 B 的闭集（另单）")

	// v1 读端校验面：两侧各有一半（A 的读端独有 4 项 / B 不做交叉核对，B 的内容比对 A 不做）。
	probes := []string{"Align != align", "len(e.SHA256) != 64", "e.Off%ix.Align != 0", "e.Off+e.Size > fileSize"}
	mA := eggFuncMarkers(faV1, []string{"ReadIndex", "validate"}, probes)
	mB := eggFuncMarkers(fb, []string{"zegParseIndexV1"}, probes)
	addLedger("v1 读端校验面", mA, mB,
		"4 项：Align != align | len(e.SHA256) != 64 | e.Off%ix.Align != 0 | e.Off+e.Size > fileSize",
		"无（0 项）",
		"A(v1 validate) 独有 4 项：align 交叉核对 / sha 长度 64 / off 对齐 / 越界；B(v1) 这 4 项都不在解析期做"+
			"（off 对齐与越界落 verify 的 problems 面、sha 走内容比对）⇒ 两份各有一半，合并会掉功能")

	// 版本兼容闸：B 有三道具名闸（D2 前向 / D3 过旧 / min_readable），A 无。
	gateNames := []string{"zegCompatGateMainVersion", "zegCompatGateMinReadable", "zegCompatGateLegacy"}
	gatesB := eggCountFuncs(fbVer, gateNames)
	gatesA := eggCountFuncs(faV2, gateNames) + eggCountFuncs(faV1, gateNames)
	addLedger("版本兼容闸面", strconv.Itoa(gatesA), strconv.Itoa(gatesB), "0", "3",
		"B 有三道具名闸（主号前向 D2 / 过旧 D3 / 卵声明 min_readable）；A 读端只有一句泛化「格式版本不支持」"+
			"⇒ 新主号能拒但点不出档位。登记为已知分叉（A 侧补闸＝改行为，另单）")

	return cells, nil
}

// ---- 主判据 ------------------------------------------------------------------------------

// TestEggFieldParity —— 现读仓内两侧源码：equal 格必须逐字相等，ledger 格必须等于已登记分叉。
func TestEggFieldParity(t *testing.T) {
	root := f24RepoRoot(t)
	if root == "" {
		t.Skip("找不到仓根（合成仓外跑）")
	}
	cells, err := eggFieldParityCells(root)
	if err != nil {
		t.Fatalf("对拍判据抽不到事实（fail-closed · 不给结论）：%v", err)
	}
	if len(cells) < 14 {
		t.Fatalf("对拍格数 %d 低于下限 14（格表被抽空 ⇒ 判据失效）", len(cells))
	}
	t.Logf("对拍格数 N=%d（equal/ledger 各 %d/%d）", len(cells), eggCountKind(cells, "equal"), eggCountKind(cells, "ledger"))
	var red []string
	for _, c := range cells {
		t.Logf("格 %2d [%s] %s｜A=%s｜B=%s｜ok=%v%s", c.No, c.Kind, c.Name, c.A, c.B, c.OK, eggCellWhy(c))
		if !c.OK {
			red = append(red, fmt.Sprintf("%s（A=%q B=%q 期望 A=%q B=%q）", c.Name, c.A, c.B, c.WantA, c.WantB))
		}
	}
	if len(red) > 0 {
		t.Fatalf("两侧实现已长歪 %d 格（加字段只加一边 / 未经登记的分叉）：\n  - %s",
			len(red), strings.Join(red, "\n  - "))
	}
}

func eggCountKind(cells []eggFieldCell, kind string) int {
	n := 0
	for _, c := range cells {
		if c.Kind == kind {
			n++
		}
	}
	return n
}

func eggCellWhy(c eggFieldCell) string {
	if c.Why == "" {
		return ""
	}
	return "｜" + c.Why
}

// ---- 负控：判据必须真拦「只加一边」 ---------------------------------------------------------

// eggSynthRepo —— 把两侧四个真源件拷进临时仓，按 edits 逐件做**字符串替换**（模拟「只加一边」）。
func eggSynthRepo(t *testing.T, name string, edits map[string][][2]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	files := map[string]string{
		"agent/internal/eggpkg/eggpkg.go":   filepath.Join("agent", "internal", "eggpkg", "eggpkg.go"),
		"agent/internal/eggpkg/expand.go":   filepath.Join("agent", "internal", "eggpkg", "expand.go"),
		"core/cmd/zerg/zerg_egg.go":         filepath.Join("core", "cmd", "zerg", "zerg_egg.go"),
		"core/cmd/zerg/zerg_egg_version.go": filepath.Join("core", "cmd", "zerg", "zerg_egg_version.go"),
	}
	srcRoot := f24RepoRoot(t)
	if srcRoot == "" {
		t.Skip("找不到仓根")
	}
	for rel, fp := range files {
		b, err := os.ReadFile(filepath.Join(srcRoot, fp))
		if err != nil {
			t.Fatalf("读真源 %s：%v", fp, err)
		}
		s := string(b)
		for _, ed := range edits[rel] {
			if strings.Count(s, ed[0]) != 1 {
				t.Fatalf("负控锚在 %s 里命中 %d 次（要恰好 1 次）：%q", rel, strings.Count(s, ed[0]), ed[0])
			}
			s = strings.Replace(s, ed[0], ed[1], 1)
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func eggRedCells(t *testing.T, root string) map[string]eggFieldCell {
	t.Helper()
	cells, err := eggFieldParityCells(root)
	if err != nil {
		t.Fatalf("合成仓对拍抽不到事实：%v", err)
	}
	out := map[string]eggFieldCell{}
	for _, c := range cells {
		if !c.OK {
			out[c.Name] = c
		}
	}
	return out
}

// 负控①：B 侧**只**新增一个头槽字段（A 侧不动）⇒ 偏移表格必红并点名该字段。
func TestEggFieldParityNeg_BHeaderFieldOnly(t *testing.T) {
	root := eggSynthRepo(t, "nc-off", map[string][][2]string{
		"core/cmd/zerg/zerg_egg.go": {
			{"\tzegV2OffCRC      = 96", "\tzegV2OffXattr    = 104\n\tzegV2OffCRC      = 96"},
		},
	})
	red := eggRedCells(t, root)
	c, ok := red["v2 头槽字段偏移表"]
	if !ok {
		t.Fatalf("负控① 失效：B 只加头槽字段，偏移表格竟未红（红格=%v）", eggCellNames(red))
	}
	t.Logf("负控① 命中：%s｜B=%s", c.Name, c.B)
	if !strings.Contains(c.B, "xattr=104") || strings.Contains(c.A, "xattr") {
		t.Fatalf("负控① 未点名该字段：A=%q B=%q", c.A, c.B)
	}
}

// 负控②：B 侧**只**给段 JSON 加一个键（A 侧不动）⇒ 段键集格必红。
func TestEggFieldParityNeg_BEntryKeyOnly(t *testing.T) {
	root := eggSynthRepo(t, "nc-seg", map[string][][2]string{
		"core/cmd/zerg/zerg_egg.go": {
			{"\tLinkTarget string `json:\"linkTarget,omitempty\"`\n}\n\ntype zegIndexJSON struct {",
				"\tLinkTarget string `json:\"linkTarget,omitempty\"`\n\tXattr      string `json:\"xattr,omitempty\"`\n}\n\ntype zegIndexJSON struct {"},
		},
	})
	red := eggRedCells(t, root)
	if _, ok := red["段 JSON 键集"]; !ok {
		t.Fatalf("负控② 失效：B 只加段键，段键集格竟未红（红格=%v）", eggCellNames(red))
	}
}

// 负控③：A 侧**只**新增一条 path 规则（B 侧不动）⇒ 规则名集格必红。
func TestEggFieldParityNeg_APathRuleOnly(t *testing.T) {
	root := eggSynthRepo(t, "nc-path", map[string][][2]string{
		"agent/internal/eggpkg/expand.go": {
			{"func eggRefusePath(rel string) error {\n", "func eggRefusePath(rel string) error {\n\t_ = \"拒：段 path —— 命中规则 path_synth\"\n"},
		},
	})
	red := eggRedCells(t, root)
	if _, ok := red["path 规则名集（v2 读端）"]; !ok {
		t.Fatalf("负控③ 失效：A 只加 path 规则，规则名集格竟未红（红格=%v）", eggCellNames(red))
	}
}

func eggCellNames(m map[string]eggFieldCell) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---- 活体对拍（opt-in）：同一枚真蛋在两份实现下的读数必须一致 ---------------------------------

// TestEggLiveABParityOnRealEgg —— 同一枚真蛋，A 侧（进程内 ReadIndex/ReadEggIndex ＋ 逐段 sha）⟷
// B 侧（现场编 `core/cmd/zerg` 后跑 `cocoon info` / `cocoon verify`）读数必须一致。
//
// ★ 默认 skip：本判据要在测试里编 core 模块（≈4 s 且依赖 core 当前可编）。常开的是上面的
// `TestEggFieldParity`（只读源码、零构建）；本格是**取证面**，一条命令即可跑：
//
//	ZERG_EGG_PARITY_LIVE=1 go test ./internal/eggpkg -run TestEggLiveABParityOnRealEgg -v
func TestEggLiveABParityOnRealEgg(t *testing.T) {
	if os.Getenv("ZERG_EGG_PARITY_LIVE") != "1" {
		t.Skip("活体双读数对拍需 ZERG_EGG_PARITY_LIVE=1（会在测试里编 core 模块）")
	}
	root := f24RepoRoot(t)
	if root == "" {
		t.Skip("找不到仓根")
	}
	egg := filepath.Join(root, "agent", "internal", "eggpkg", "testdata", "demo.egg")
	if _, err := os.Stat(egg); err != nil {
		t.Fatalf("真蛋取不到（%s）：%v", egg, err)
	}

	// B 侧：`go run` 现编现跑（✗ 不用 bin/zerg —— 制品可能落后于源码）。
	// ★ 首参一律写带引号的字面量「go」（门103 P3 只报 `exec.Command(<变量>` 形态）。
	runB := func(args ...string) ([]byte, error) {
		cmd := exec.Command("go", append([]string{"run", "-mod=mod", "./cmd/zerg"}, args...)...)
		cmd.Dir = filepath.Join(root, "core")
		return cmd.CombinedOutput()
	}
	infoOut, err := runB("cocoon", "info", egg)
	if err != nil {
		t.Fatalf("B 侧 cocoon info 失败：%v\n%s", err, infoOut)
	}
	verOut, err := runB("cocoon", "verify", egg)
	if err != nil {
		t.Fatalf("B 侧 cocoon verify 失败：%v\n%s", err, verOut)
	}

	bMagic, bVer, bAlign, bSegs := eggParseCocoonInfo(string(infoOut))
	if bMagic == "" || bSegs < 0 {
		t.Fatalf("B 侧 info 输出解析不出读数（格式已变 · 判据抽空 ⇒ 不给结论）：\n%s", infoOut)
	}
	bVerifyOK := strings.Contains(string(verOut), "校验通过") && strings.Contains(string(verOut), fmt.Sprintf("（%d 段", bSegs))
	t.Logf("B 侧读数：magic=%s version=%d align=%d 段数=%d 校验=%v", bMagic, bVer, bAlign, bSegs, bVerifyOK)

	// A 侧：v1 走 ReadIndex，v2 走 ReadEggIndex（按 B 报的版本分派，与 B 自己的分派同口径）。
	aMagic, aVer, aAlign, aSegs, aSHAOK, err := eggReadSideA(egg, bVer)
	if err != nil {
		t.Fatalf("A 侧读端拒了同一枚蛋（两侧结论不一致）：%v", err)
	}
	t.Logf("A 侧读数：magic=%s version=%d align=%d 段数=%d 逐段sha=%d/%d", aMagic, aVer, aAlign, aSegs, aSHAOK, aSegs)

	if aMagic != bMagic || aVer != bVer || aAlign != bAlign || aSegs != bSegs {
		t.Fatalf("同一枚蛋两侧读数不一致：A(magic=%s ver=%d align=%d segs=%d) ≠ B(magic=%s ver=%d align=%d segs=%d)",
			aMagic, aVer, aAlign, aSegs, bMagic, bVer, bAlign, bSegs)
	}
	if !bVerifyOK || aSHAOK != aSegs {
		t.Fatalf("校验结论不一致：B verify=%v · A 逐段 sha %d/%d", bVerifyOK, aSHAOK, aSegs)
	}
}

// eggParseCocoonInfo —— 从 `cocoon info` 人面输出抽四格（magic / version / align / 段数）。
func eggParseCocoonInfo(out string) (magic string, ver int, align int64, segs int) {
	segs = -1
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		for _, tok := range f {
			switch {
			case strings.HasPrefix(tok, "magic="):
				magic = strings.TrimPrefix(tok, "magic=")
			case strings.HasPrefix(tok, "version="):
				ver, _ = strconv.Atoi(strings.TrimPrefix(tok, "version="))
			case strings.HasPrefix(tok, "align="):
				v := strings.TrimPrefix(tok, "align=")
				if i := strings.IndexAny(v, "（("); i >= 0 {
					v = v[:i]
				}
				n, _ := strconv.ParseInt(v, 10, 64)
				align = n
			}
			if i := strings.Index(line, "段数="); i >= 0 {
				rest := line[i+len("段数="):]
				j := 0
				for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
					j++
				}
				segs, _ = strconv.Atoi(rest[:j])
			}
		}
	}
	return magic, ver, align, segs
}

// eggReadSideA —— A 侧读数（按 B 报的布局主号分派；逐段现算 sha 得到 A 的校验结论）。
func eggReadSideA(egg string, layoutVer int) (string, int, int64, int, int, error) {
	f, err := os.Open(egg)
	if err != nil {
		return "", 0, 0, 0, 0, err
	}
	defer f.Close()
	var (
		magic string
		ver   int
		align int64
		segs  []EggSegment
	)
	switch layoutVer {
	case 1:
		ix, err := ReadIndex(egg)
		if err != nil {
			return "", 0, 0, 0, 0, err
		}
		magic, ver, align = ix.Magic, ix.Version, ix.Align
		for _, e := range ix.Entries {
			segs = append(segs, EggSegment{Path: e.Path, Off: e.Off, Size: e.Size, SHA256: e.SHA256})
		}
	case 2:
		ix, err := ReadEggIndex(egg)
		if err != nil {
			return "", 0, 0, 0, 0, err
		}
		magic, ver, align = eggMagicV2, eggLayoutV2, ix.Align
		segs = ix.Entries
	default:
		return "", 0, 0, 0, 0, fmt.Errorf("B 报了本判据不认识的布局主号 %d", layoutVer)
	}
	ok := 0
	h := sha256.New()
	for _, e := range segs {
		h.Reset()
		if _, err := io.Copy(h, io.NewSectionReader(f, e.Off, e.Size)); err != nil {
			return "", 0, 0, 0, 0, err
		}
		if hex.EncodeToString(h.Sum(nil)) == e.SHA256 {
			ok++
		}
	}
	return magic, ver, align, len(segs), ok, nil
}
