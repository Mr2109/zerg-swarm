// family_code_constdup.go —— `zerg code const-dup`（缺口 `GAP-20261001-69` 的前置工具）。
//
// 病（缺口逐字）：同一个字符串值被**多个常量各抄一份**（同值异名）。改一处、漏一处 ⇒ 两处静默分叉，
// 且没有任何判据看着它。本命令把这类点**列出来**供人工归一。
//
// ★ 本单只造工具：本命令**不改**任何常量、**不改**任何调用点（缺口原话）。全件**零写盘** —— 不建目录、
// 不落状态、不改件，归 L0 只读族。
//
// 面（`zerg code const-dup --path <件或目录> [--glob <模式>] [--root <仓根>] [--json <字段>] [--fail-on-hit]`）：
//
//	· 扫法走 **Go 自己的解析器**（`go/parser` + `go/ast` + `go/token`），✗ 不用正则 ——
//	  正则会把**注释里的**字面量与**字符串里的**字面量一起误判。本命令只取 **`const` 声明里的字符串
//	  字面量**：`*ast.GenDecl{Ttok: token.CONST}` → `*ast.ValueSpec` 的取值是 `*ast.BasicLit` 且
//	  `Kind == token.STRING`（其余形态 —— 拼接、引用别的常量、数字 —— 一律**不**计入）。
//	· 排除面：`_test.go` · `vendor/` · `node_modules/` · `.git/` · `>2MB` 的件（目录闸 / 件闸直接复用
//	  `code find` 那一套：`codeScanSkipDirs` ＋ 点项规则 ＋ `codeScanSkipFile`，✗ 不另抄第二份 —— 两处
//	  口径一旦分家就必然漂）。
//	· 聚合口径：按**值**分组；同一个值被 **≥2 个不同常量名**指向 ⇒ 判「同值异名」。
//	  保留名（owner）= 定义点按 `(件, 行)` 字典序**最靠前**的那一枚（确定性口径 —— ✗ 猜「谁更该留」；
//	  要谁留是人拍的事，本命令只把候选摊开）。
//	· 分类按值的形状：含 `/` ⇒ `path` · 以 `.go/.json/.yaml/.md/.sh` 结尾 ⇒ `file` · 纯小写单词 ⇒
//	  `token` · 其余 ⇒ `other`。
//
// 退码：`0` 有读数（**含命中** —— 默认不红）· `1` 给了 `--fail-on-hit` 且命中 >0（供 CI 用）·
// `2` 用法错（缺 `--path` / 路径不存在 / 出仓 / `--glob` 非法）· `8` 判不了（仓根或目录读不到）。
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ccdFields —— `--json` 的字段表（K1：机器面先定）。与命令树 `fields:` 逐名同源（门58 对拍面）。
var ccdFields = []string{"value", "category", "count", "owner", "dupes"}

// ccdCategoryOrder —— 人面「按类分组」的固定次序（✗ 不按 map 序：那会随语言实现抖）。
var ccdCategoryOrder = []string{"path", "file", "token", "other"}

// ccdSite —— 一处常量定义点。
type ccdSite struct {
	Name  string
	Value string // 该常量指向的字符串字面量（已去掉引号 —— 聚合的键）
	File  string // 仓相对路径（`/` 分隔）
	Line  int
}

// ccdGroup —— 一个「值」的聚合结果（= 输出的一行）。
type ccdGroup struct {
	Value    string
	Category string
	Owner    ccdSite
	Dupes    []ccdSite
}

// ccdCount —— 该值的常量名枚数（含 owner）。
func (g ccdGroup) ccdCount() int { return 1 + len(g.Dupes) }

// ccdCategory —— 按值的**形状**归类（口径逐条见件头）。
func ccdCategory(v string) string {
	if strings.Contains(v, "/") {
		return "path"
	}
	for _, ext := range []string{".go", ".json", ".yaml", ".md", ".sh"} {
		if strings.HasSuffix(v, ext) {
			return "file"
		}
	}
	if ccdLowerWord(v) {
		return "token"
	}
	return "other"
}

// ccdLowerWord —— 纯小写单词：非空、全 ASCII 小写字母（`token` 类只此一档，不扩到数字/连字符 ——
// 扩了就把 `2kb`、`foo-bar` 一类混进来，分类面失去意义）。
func ccdLowerWord(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// ccdSiteText —— 人面 `名@件:行`。
func ccdSiteText(s ccdSite) string {
	return s.Name + "@" + s.File + ":" + strconv.Itoa(s.Line)
}

// ccdJSONObj —— 一行的机器面（★ `owner` / `dupes` 是**真对象 / 真数组**：`listCmd` 的行值全是字符串，
// 装不下嵌套结构 ⇒ 本命令自建对象，字段白名单仍走 `reportBadField` 那一套同一句判词）。
func (g ccdGroup) ccdJSONObj(fields []string) string {
	var b strings.Builder
	b.WriteString("{")
	n := 0
	put := func(k, raw string) {
		if n > 0 {
			b.WriteString(",")
		}
		b.WriteString(ccdJstr(k))
		b.WriteString(":")
		b.WriteString(raw)
		n++
	}
	for _, f := range fields {
		switch f {
		case "value":
			put("value", ccdJstr(g.Value))
		case "category":
			put("category", ccdJstr(g.Category))
		case "count":
			put("count", strconv.Itoa(g.ccdCount()))
		case "owner":
			put("owner", ccdSiteJSON(g.Owner))
		case "dupes":
			var a strings.Builder
			a.WriteString("[")
			for i, d := range g.Dupes {
				if i > 0 {
					a.WriteString(",")
				}
				a.WriteString(ccdSiteJSON(d))
			}
			a.WriteString("]")
			put("dupes", a.String())
		}
	}
	b.WriteString("}")
	return b.String()
}

func ccdSiteJSON(s ccdSite) string {
	return `{"name":` + ccdJstr(s.Name) + `,"file":` + ccdJstr(s.File) + `,"line":` + strconv.Itoa(s.Line) + `}`
}

func ccdJstr(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// ccdCollectFromFile —— 解析一件，把 `const` 里的字符串字面量追加进 out。
// 返回：是否真扫成（parse 失败 ⇒ false，交调用方计数，✗ 不静默当「零命中」）。
func ccdCollectFromFile(fset *token.FileSet, root, p string, out *[]ccdSite) (bool, error) {
	src, err := os.ReadFile(p)
	if err != nil {
		return false, err
	}
	af, perr := parser.ParseFile(fset, p, src, parser.SkipObjectResolution)
	if perr != nil || af == nil {
		return false, perr
	}
	rel, rerr := filepath.Rel(root, p)
	if rerr != nil {
		rel = p
	}
	rel = filepath.ToSlash(rel)
	for _, d := range af.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, sp := range gd.Specs {
			vs, ok := sp.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, nm := range vs.Names {
				// 取值与名字按下标对齐；给了值的那一格才判（`const ( a = "x"; b )` 的 b 沿用上一行 ⇒
				// 本命令**不**替它推 —— 没有自己的字面量就不是本命令的靶子）。
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				val, uerr := strconv.Unquote(lit.Value)
				if uerr != nil {
					continue
				}
				*out = append(*out, ccdSite{Name: nm.Name, Value: val, File: rel, Line: fset.Position(lit.Pos()).Line})
			}
		}
	}
	return true, nil
}

// ccdResolveRoot —— `--root` 显式给了就用它（必须是目录）；否则走 `codeRepoRoot()`（与同族六条只读
// 子命令同一条根判据：cwd 在并排仓里时读那棵树）。
//
// ★ 它**只算根**、不报错：退码与 `error.detail` 一律由调用点用**字面量**报 —— `setErr` 的第二参不许传变量（门⑤ 的 detail 取值口径把「非字面量形态」也当登记面，传变量会造出一个叫 `kind` 的幽灵形态）。
func ccdResolveRoot(inv *invocation) string {
	if r := strings.TrimSpace(inv.flagVal("--root")); r != "" {
		abs := r
		if !filepath.IsAbs(abs) {
			if wd, err := os.Getwd(); err == nil {
				abs = filepath.Join(wd, filepath.FromSlash(r))
			}
		}
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			return ""
		}
		return filepath.Clean(abs)
	}
	return codeRepoRoot()
}

func cmdCodeConstDup(inv *invocation, stdout, stderr io.Writer) int {
	sub := strings.TrimSpace(inv.flagVal("--path"))
	if sub == "" {
		inv.setErr("usage", "missing_path", "缺 --path")
		fmt.Fprintf(stderr, "%s: `code const-dup` 要给 --path <件或目录>（例：%s code const-dup --path core/cmd/zerg）⇒ 退码 2\n", progName, progName)
		fmt.Fprintf(stderr, "用法：%s code const-dup --path <件或目录> [--glob <模式>] [--root <仓根>] [--json <字段>] [--fail-on-hit]\n", progName)
		return exitUsage
	}
	if bad := foreignFlag(inv, "--path", "--glob", "--root", "--fail-on-hit"); bad != "" {
		// ★ 与同族 `code find` / `code audit-readings` 逐字同一种拒法：只打判词、**不** setErr
		//   （`--json` 用法错的机器面由 dispatch 兜 —— 本处不新造 detail 字面量）。
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, bad)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	// ★ 两种「根取不到」分开报：`--root` 给了却不是目录 ⇒ **用法错**（2）；不给 `--root` 又解析不到
	//   仓根 ⇒ **判不了**（8）。两者的 detail 都是仓内既有字面量（✗ 不新造）。
	root := ccdResolveRoot(inv)
	if root == "" {
		if strings.TrimSpace(inv.flagVal("--root")) != "" {
			inv.setErr("usage", "path_not_dir", "--root 不是目录")
			fmt.Fprintf(stderr, "%s: `--root` 给的不是一个目录 ⇒ 退码 2\n", progName)
			return exitUsage
		}
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不了码（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	base := filepath.Clean(filepath.FromSlash(sub))
	if !filepath.IsAbs(base) {
		base = filepath.Join(root, filepath.FromSlash(sub))
		if !pathWithinDir(root, base) {
			inv.setErr("usage", "path_outside_repo", "--path 出仓")
			fmt.Fprintf(stderr, "%s: --path 出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, sub, root)
			return exitUsage
		}
	}
	if st, serr := os.Stat(base); serr != nil || (!st.IsDir() && !st.Mode().IsRegular()) {
		inv.setErr("usage", "path_not_dir", "--path 既不是件也不是目录")
		fmt.Fprintf(stderr, "%s: --path 指的既不是件也不是目录：%s ⇒ 退码 2\n", progName, sub)
		return exitUsage
	}
	glob := strings.TrimSpace(inv.flagVal("--glob"))
	if glob != "" {
		if _, gerr := filepath.Match(glob, ""); gerr != nil {
			inv.setErr("usage", "bad_glob", "--glob 模式非法")
			fmt.Fprintf(stderr, "%s: --glob 模式非法（模式 %q）：%v ⇒ 退码 2\n", progName, glob, gerr)
			return exitUsage
		}
	}

	sites := []ccdSite{}
	scanned, skippedTest, skippedBig, skippedNonGo, parseFailed := 0, 0, 0, 0, 0
	parseErrText := ""
	fset := token.NewFileSet()
	walkErr := filepath.Walk(base, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			return nil
		}
		if info.IsDir() {
			dotDir := strings.HasPrefix(info.Name(), ".") && !codeScanKeepDots[info.Name()]
			if codeScanSkipDirs[info.Name()] || dotDir {
				return filepath.SkipDir
			}
			return nil
		}
		if codeScanSkipFile(root, p) {
			return nil
		}
		baseName := filepath.Base(p)
		if !strings.HasSuffix(baseName, ".go") {
			skippedNonGo++
			return nil
		}
		if strings.HasSuffix(baseName, "_test.go") {
			skippedTest++
			return nil
		}
		if glob != "" {
			if ok, _ := filepath.Match(glob, baseName); !ok {
				return nil
			}
		}
		if info.Size() > codeFindMaxFileBytes {
			skippedBig++
			return nil
		}
		scanned++
		ok, derr := ccdCollectFromFile(fset, root, p, &sites)
		if !ok {
			parseFailed++
			if parseErrText == "" && derr != nil {
				parseErrText = fmt.Sprintf("%s：%v", filepath.Base(p), derr)
			}
		}
		return nil
	})
	if walkErr != nil {
		inv.setErr("blocked", "walk_failed", "扫不动")
		fmt.Fprintf(stderr, "%s: 扫不动（%v）⇒ 不给结论（退码 8）\n", progName, walkErr)
		return exitBlocked
	}
	if scanned > 0 && scanned == parseFailed {
		inv.setErr("blocked", "walk_failed", "全都解析不了")
		fmt.Fprintf(stderr, "%s: %d 件 Go 件一件都没解析成（首件：%s）⇒ 不给结论（退码 8）\n", progName, parseFailed, parseErrText)
		return exitBlocked
	}

	groups := ccdAggregate(sites)

	fmt.Fprintf(stderr, "%s: 扫了 %d 件非测试 Go 件", progName, scanned)
	if skippedTest > 0 {
		fmt.Fprintf(stderr, " · 跳过 _test.go %d 件", skippedTest)
	}
	if skippedBig > 0 {
		fmt.Fprintf(stderr, " · 跳过 >%dMB 的件 %d 个", codeFindMaxFileBytes>>20, skippedBig)
	}
	if skippedNonGo > 0 {
		fmt.Fprintf(stderr, " · 非 Go 件 %d 个不进面", skippedNonGo)
	}
	if parseFailed > 0 {
		fmt.Fprintf(stderr, " · 解析不了 %d 件（不计入，✗ 不当零命中：%s）", parseFailed, parseErrText)
	}
	fmt.Fprintf(stderr, " · 字符串常量定义 %d 处 · 同值异名 %d 组（覆盖常量 %d 枚）\n",
		len(sites), len(groups), ccdCovered(groups))
	if len(groups) > 0 {
		fmt.Fprintf(stderr, "按类分组：%s\n", ccdCategorySummary(groups))
	}

	if inv.jsonGiven {
		if r := requireFields(inv, stderr); r != exitOK {
			return r
		}
		for _, f := range inv.fields {
			if !ccdFieldKnown(f) {
				inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
				return reportBadField(stderr, inv.path, f)
			}
		}
		cmd := find(inv.path)
		if cmd == nil {
			return exitUsage
		}
		objs := make([]string, 0, len(groups))
		for _, g := range groups {
			objs = append(objs, g.ccdJSONObj(inv.fields))
		}
		emitEnvelopeWith(stdout, cmd, "["+strings.Join(objs, ",")+"]", len(groups), inv)
	} else {
		fmt.Fprintln(stdout, "value\tcategory\tcount\towner\tdupes")
		for _, g := range groups {
			dupText := make([]string, 0, len(g.Dupes))
			for _, d := range g.Dupes {
				dupText = append(dupText, ccdSiteText(d))
			}
			fmt.Fprintf(stdout, "%s\t%s\t%d\t%s\t%s\n",
				ccdCell(g.Value), g.Category, g.ccdCount(), ccdCell(ccdSiteText(g.Owner)), ccdCell(strings.Join(dupText, ",")))
		}
	}
	if len(groups) == 0 {
		fmt.Fprintf(stderr, "%s: 该面没有同值异名 —— 这不是错，是「没有」（退码 0）\n", progName)
		return exitOK
	}
	if inv.hasFlag("--fail-on-hit") {
		fmt.Fprintf(stderr, "%s: 命中 %d 组 ⇒ `--fail-on-hit` 生效（退码 1 · 供 CI）\n", progName, len(groups))
		return exitFail
	}
	return exitOK
}

// ccdCell —— 人面单元格：制表符/换行按 TSV 规矩转义（照同族 `codeTextCell` 的处置面）。
func ccdCell(s string) string {
	r := strings.NewReplacer("\t", "\\t", "\n", "\\n", "\r", "\\r")
	return r.Replace(s)
}

// ccdFieldKnown —— `--json` 字段是否在登记面（与命令树 `fields:` 同一份 `ccdFields`）。
func ccdFieldKnown(f string) bool {
	for _, x := range ccdFields {
		if x == f {
			return true
		}
	}
	return false
}

// ccdAggregate —— 按值聚合并判「同值异名」：同一个值被 **≥2 个不同常量名**指向才算命中
// （同一个名字在两个包里各定义一次 ⇒ 只算一个名字 ⇒ **不**算异名）。
func ccdAggregate(sites []ccdSite) []ccdGroup {
	byValue := map[string][]ccdSite{}
	for _, s := range sites {
		byValue[s.Value] = append(byValue[s.Value], s)
	}
	out := []ccdGroup{}
	for v, ss := range byValue {
		sort.Slice(ss, func(i, j int) bool {
			if ss[i].File != ss[j].File {
				return ss[i].File < ss[j].File
			}
			if ss[i].Line != ss[j].Line {
				return ss[i].Line < ss[j].Line
			}
			return ss[i].Name < ss[j].Name
		})
		seen := map[string]bool{}
		uniq := make([]ccdSite, 0, len(ss))
		for _, s := range ss {
			if seen[s.Name] {
				continue
			}
			seen[s.Name] = true
			uniq = append(uniq, s)
		}
		if len(uniq) < 2 {
			continue
		}
		out = append(out, ccdGroup{Value: v, Category: ccdCategory(v), Owner: uniq[0], Dupes: uniq[1:]})
	}
	// 定序：先按类（`path`→`file`→`token`→`other`），再按值（字节序）。
	rank := map[string]int{}
	for i, c := range ccdCategoryOrder {
		rank[c] = i
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return rank[out[i].Category] < rank[out[j].Category]
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// ccdCovered —— 命中覆盖的常量枚数（每组 = owner + dupes）。
func ccdCovered(gs []ccdGroup) int {
	n := 0
	for _, g := range gs {
		n += g.ccdCount()
	}
	return n
}

// ccdCategorySummary —— 「path 2 组 · file 1 组 · …」（零组的类也报，✗ 不省 —— 省了读者会以为没扫那一类）。
func ccdCategorySummary(gs []ccdGroup) string {
	cnt := map[string]int{}
	for _, g := range gs {
		cnt[g.Category]++
	}
	parts := make([]string, 0, len(ccdCategoryOrder))
	for _, c := range ccdCategoryOrder {
		parts = append(parts, fmt.Sprintf("%s %d 组", c, cnt[c]))
	}
	return strings.Join(parts, " · ")
}
