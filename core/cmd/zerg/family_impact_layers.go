// family_impact_layers.go —— `zerg impact <目标>` 的**六层取数**（`A2` 件）。
//
// 依据（逐字）：`设计-变更影响面-v1.6` §二（六层表 + **层规**：任何一层的产物/输出都必须带
// `head_sha` + `layer` + 该层口径值；**② 层必带 `algo`** —— 同仓同时刻模块内边 `cha` 6,049 vs
// `rta` 3,869 = **1.56 倍**，不带算法名的边数不可比）· §四.4（分层预算：**默认档只吃毫秒层 +
// 编译器层**，贵层按需）· §七.4（时效三件 + **不新增第七键**）· §九 判据①⑥·
// 任务单-影响面实施-20260922 §二 `A2` 判据①–④。
//
// 六层与顺序（§二「五层的顺序就是命令里的顺序」· v1.4 把顺序立成三条判据：**确定性递减 ·
// 前层不依赖后层 · 单层成本可单独量**，并明说**不按墙钟递增排**——④ 两层 0.055s 比 ① 编译层
// 2.2s 便宜两个数量级却排在后面）：
//
//	① 编译器层（`go build ./...` + `go list` 反向包）—— **包级**
//	② 符号层（`callgraph -algo rta`）—— **符号级**（贵层：现跑 3.9–5.8s ⇒ 走 `--quick` 档可跳）
//	③ 契约层（`core/internal/contract/registry.json`）—— **契约级**
//	④ 词法 + 形近层（`git grep -w` 词边界 / `git grep -E` 正则面 / `ast-grep` 结构同形）—— **文件级 + 名字级**
//	⑤ 语义层（**按需**：在位判据可测 + 索引未建即不取数）—— 件级
//	⑥ 公开面（`v1.6` 新增 · `R55`）—— 件级（**唯一产物不在本仓的一层**）
//
// 诚实边界（§十一 · `F6`「把『没报』读成『没影响』」）：
//   - 每一层的「未跑 / 未适用 / 未装 / 未建索引 / 读不到」都**逐层点名**（`--quick` 跳过的层
//     写「未跑」，**不冒充「没影响」**）；层表恒在 stderr，人面三行里不塞这些。
//   - **`meta.layers_not_run[]` / `meta.head_sha` / `meta.effective_at` 今天没有落点** ——
//     `emitEnvelopeWith`（`main.go`）把六键里的 `warnings` 恒写 `[]`、`meta` 只写
//     `count/source/changed`（+`node`/`idempotency_key`）⇒ 层规三件只能落在**既有位置**：
//     人面条件行 + stderr 层表 + `items[]` 的四字段；本件**不改 `emitEnvelope*`**（`A1` 红线）
//     ⇒ **这一格照实记为 CLI 缺口**（回执里的「CLI 缺口一栗」）。
//   - 运行期面（反射 / 配置串 / HTTP 路由 / 序列化）标 **「未查」**（§十一：静态图天生看不见）；
//     `R34` 的「`zerg doctor` 名册四项 verdict 摘要进 `warnings[]` 一条」**本批未接**（同一处缺口：
//     `warnings[]` 是死的）—— **照实标未做**，不许假装全覆盖。
//   - ④a 只用 `git grep`（**只搜 tracked**）并在口径里写死；仓里另有一条正则面命令
//     `zerg code find`（**全盘口径**：含未跟踪件）—— 两层口径不同、数不同源，本件**不另抄它的口径**
//     （`A1` 红线：不动 `code find` 的扫码口径与排除表）。
//
// 红线（任务单 §二 `A2` 逐条）：不引 `gopls` / `rust-analyzer` ✗ · 不加 `go.work` ✗ ·
// 不动 `publish/` 那七件生效面里的任何一行 ✗（第 ⑥ 层**只消费** `check-publish-face-sync.py`
// 的输出）· 不拿私有树件数冒充产出树件数 ✗ · **不为凑绿放宽任何既有判据** ✗ ·
// 本件只读：不写缓存、不落审计、不改件（缓存与毫秒档属 `A5` · 挂干跑属 `A4`）。
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---- 层规三件（§二 层规 · `A2` 判据①）-----------------------------------------------------

// impactLayer —— 一层的取数结果（**层规三件**：`HeadSHA` + `Seq/Name`（layer）+ `Caliber`（该层口径值））。
type impactLayer struct {
	Seq     string // ①–⑥（层序 = §二 的表序 = 命令里的顺序）
	Name    string // 层名
	Grane   string // **粒度**四档之一（`R40`：文件级 / 包级 / 符号级 / 契约级 —— 四档不可相加）
	Caliber string // **该层口径值**（② 必带 `algo`；④b 给 ast-grep 版本；③给 registry 行数与条目数…）
	Status  string // 取值 / 未跑 / 未适用 / 未装 / 未建索引 / 读不到
	Detail  string // 该层现跑的读数（人面层表用 · **原样**，不做心算）
	Cost    time.Duration
	Rows    []map[string]string // 该层给出的条目（四字段 what/why/how/red · 粒度前缀在 what 里）
	HeadSHA string
	At      string

	// 第 ⑥ 层的条件行三件（§3.7 口径：N / M · 产出树路径 + 扫的时刻 · head_sha）——
	// 只有命中生效面时才有值。
	PublicDelta string
	PublicTree  string
	PublicAt    string
}

// impactRow —— 条目四字段（§3.1 · `A1` 的字段表一字不改）。
func impactRow(what, why, how, red string) map[string]string {
	return map[string]string{"what": what, "why": why, "how": how, "red": red}
}

// impactRowPage —— 一页最多列多少条（**明说**上限，不静默截；`truncated` 那一格今天在
// `emitEnvelopeWith` 里是死的 ⇒ 上限走 stderr 明说）。
const impactRowPage = 40

// 层 ② 的算法名（`R13` v1.4 起为**必填**）。取 `rta`：波纹要的是**反向边**（谁真的调我），
// `rta` 是可达性分析、边数最少口径最窄（对照现跑：`cha` 最宽）—— 换算法就必须换这一格。
const impactSymbolAlgo = "rta"

// 层 ⑥ 的生效面（**七件** · §3.7 判据）。本件只**点名**它们（读七件是 `publish/` 的红线面：
// 一行都不许改），命中判定 = 逐字相等 ∈ 七件 · 或 目标在 `scripts/公开标记.tsv` 里有行 ·
// 或 目标在 `publish/` 的清单里逐字出现（**成员检查**，不是链上判定 —— 链上判定一律以
// `scripts/gates/check-publish-face-sync.py` 的现跑输出为准）。
var impactPublishFace = []string{
	"scripts/公开标记.tsv",
	"scripts/build/publish-public.sh",
	"publish/mirror-public-lib.py",
	"publish/whitelist.txt",
	"publish/mirror-allow.txt",
	"publish/private-paths.txt",
	"publish/replace-rules.tsv",
}

// impactPubTreeDefault —— 产出树路径（§3.7 口径三件之一：**产出树路径 + 扫的时刻 + `head_sha`**）。
// 环境变量优先（同一口径可指向别的快照）；不写死私有绝对路径（门 `check-hardcoded-private-paths.py`）。
const impactPubTreeDefault = "/tmp/zerg-pub-final-20260921"

// impactPublishFaceSyncRel —— 第 ⑥ 层的**判据面**（`A2` 产物：**复用 · 不改它**，只消费它的输出）。
const impactPublishFaceSyncRel = "scripts/gates/check-publish-face-sync.py"

// ---- 现跑底座 ------------------------------------------------------------------------------

// impactRunIn 在 dir 里跑一条外部命令（**只读**用途；stdout/stderr 分开收）。
// 起不来 ⇒ err 非空（调用方按「读不到」处置 —— **不给结论**，不许当成「没有」）。
func impactRunIn(dir, name string, args ...string) (string, string, int, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
			err = nil // 起得来、退码非 0：是「结果」，不是「起不来」
		} else {
			return out.String(), errb.String(), -1, err
		}
	}
	return out.String(), errb.String(), code, nil
}

// impactHeadSHA 取数时那笔仓的提交（形状出处 §二：`head_sha=%s`）。
func impactHeadSHA(root string) string {
	out, _, code, err := impactRunIn(root, "git", "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}

// impactEffectiveAt —— 取数时刻（§7.4 的 `effective_at`；带时刻是附二第 10 行的血泪：计数自己会变）。
func impactEffectiveAt() string { return time.Now().Format(time.RFC3339) }

// impactModuleDir 从目标件往上找 module（有 `go.mod` 的最近祖先）⇒ (module 相对仓根的目录, 模块路径)。
func impactModuleDir(root, rel string) (string, string) {
	dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
	for {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			modRel, rerr := filepath.Rel(root, dir)
			if rerr != nil {
				return "", ""
			}
			return filepath.ToSlash(modRel), impactGoModPath(string(b))
		}
		parent := filepath.Dir(dir)
		if parent == dir || len(parent) < len(root) {
			return "", ""
		}
		dir = parent
	}
}

// impactGoModPath 逐行读 `go.mod` 的 `module` 行（**不引解析库**：只要一个模块路径）。
func impactGoModPath(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(ln, "module "))
		}
		if strings.HasPrefix(ln, "module\t") {
			return strings.TrimSpace(strings.TrimPrefix(ln, "module\t"))
		}
	}
	return ""
}

// impactGoFileSymbols 目标件里的**顶层函数 / 方法 / 类型名**（`go/parser` 标准库 ⇒ 不引依赖）。
// 返回名 → 行号（行号只为人面定位用）。
func impactGoFileSymbols(absPath string) map[string]int {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	out := map[string]int{}
	add := func(n *ast.Ident) {
		if n == nil || n.Name == "" || n.Name == "_" {
			return
		}
		out[n.Name] = fset.Position(n.Pos()).Line
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			nm := fn.Name.Name
			if fn.Recv != nil {
				nm = fn.Name.Name // 方法：只留方法名（callgraph 的口径里方法名是最后一段）
			}
			add(&ast.Ident{Name: nm, NamePos: fn.Name.NamePos})
			continue
		}
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			for _, sp := range gd.Specs {
				if ts, ok := sp.(*ast.TypeSpec); ok {
					add(ts.Name)
				}
			}
		}
	}
	return out
}

// impactCallgraphBin 找 `callgraph`（本机 `~/go/bin/callgraph` · 10,115,714 B ≈ 10.1 MB · §二 ②）。
// 找不到 ⇒ 空串（层 ② 标「未装」—— 宁少报不猜报，**不许**自己造一份符号面）。
func impactCallgraphBin() string {
	if p, err := exec.LookPath("callgraph"); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, "go", "bin", "callgraph")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ---- 六层取数一条链（§二 顺序即命令里的顺序）------------------------------------------------

// impactPullLayers 跑六层（`cheap` = §4.4 的默认档：贵层**跳过并点名**：贵层**跳过并点名**，不静默少给）。
func impactPullLayers(root string, tgt *impactTarget, cheap bool) []impactLayer {
	layers := []impactLayer{
		impactLayerCompiler(root, tgt),
		impactLayerSymbol(root, tgt, cheap),
		impactLayerContract(root, tgt),
		impactLayerLexical(root, tgt),
		impactLayerSemantic(root, tgt),
		impactLayerPublic(root, tgt, cheap),
	}
	head, at := impactHeadSHA(root), impactEffectiveAt()
	for i := range layers {
		layers[i].HeadSHA = head
		layers[i].At = at
	}
	return layers
}

// -------- ① 编译器层（包级）----------------------------------------------------------------

func impactLayerCompiler(root string, tgt *impactTarget) impactLayer {
	lay := impactLayer{
		Seq: "①", Name: "编译器层", Grane: "包级",
		Caliber: "go build ./...（目标件所属 module）+ go list -f '{{.ImportPath}} {{join .Imports \" \"}}' ./... 逐包筛反向（Go 没反向开关 ⇒ 两段式拼 · 设计 §二 免费层）",
	}
	if tgt.Kind != impactKindFile {
		lay.Status = "未适用"
		lay.Detail = "目标不是件（契约 id）⇒ 无编译面 / 无反向包面 —— **不是 0，是未适用**"
		return lay
	}
	modDir, modPath := impactModuleDir(root, tgt.Rel)
	if modDir == "" {
		lay.Status = "读不到"
		lay.Detail = "目标件往上找不到 go.mod ⇒ 解析不到 module ⇒ 编译面**不给结论**"
		return lay
	}
	lay.Caliber += fmt.Sprintf(" · module=%s（%s）", modDir, modPath)
	modAbs := filepath.Join(root, filepath.FromSlash(modDir))

	start := time.Now()
	out, errOut, code, err := impactRunIn(modAbs, "go", "build", "./...")
	lay.Cost = time.Since(start)
	if err != nil {
		lay.Status = "读不到"
		lay.Detail = fmt.Sprintf("`go build` 起不来：%v ⇒ 编译面不给结论", err)
		return lay
	}
	// 缓存态（`R35` 口径三件套之一）：Go build cache 的命中情况**命令面看不见**
	// ⇒ 照实标「未测」（缺档位的耗时**不许进预算裁决**）。
	compileLines := impactCountNonEmpty(out + "\n" + errOut)
	lay.Caliber += " · 缓存态=未测（go build cache 命中命令面不可见 ⇒ 本层耗时只作参考，不进预算裁决）"

	// 反向包（两段式）：目标包的 import path。
	pkgSuffix := path.Dir(filepath.ToSlash(tgt.Rel))
	if i := strings.Index(pkgSuffix, "/"); i >= 0 && modDir != "." {
		pkgSuffix = strings.TrimPrefix(pkgSuffix, modDir+"/")
	}
	pkgPath := modPath
	if pkgSuffix != "." && pkgSuffix != "" {
		pkgPath = modPath + "/" + pkgSuffix
	}
	rev := []string{}
	lout, lerr, lcode, lerrRaw := impactRunIn(modAbs, "go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...")
	if lerrRaw != nil || lcode != 0 {
		lay.Status = "读不到"
		lay.Detail = fmt.Sprintf("`go list ./...` rc=%d ⇒ 反向包面不给结论（stderr: %s）", lcode, impactFirstLine(lerr))
		return lay
	}
	for _, ln := range strings.Split(lout, "\n") {
		fs := strings.Fields(strings.TrimSpace(ln))
		if len(fs) < 2 {
			continue
		}
		for _, imp := range fs[1:] {
			if imp == pkgPath {
				rev = append(rev, fs[0])
				break
			}
		}
	}
	sort.Strings(rev)
	for _, p := range rev {
		lay.Rows = append(lay.Rows, impactRow("包级:"+p, "包反向",
			"这一包 import 了 "+pkgPath+" —— 改签名会先在这一包报编译错", ""))
	}
	lay.Status = "取值"
	lay.Detail = fmt.Sprintf("`go build ./...` rc=%d · 输出 %d 行 · 反向包 %d 个（依赖 %s 的包）· 目标包=%s",
		code, compileLines, len(rev), pkgPath, pkgPath)
	if code != 0 {
		lay.Detail += fmt.Sprintf(" · ★ 编译面已红（rc=%d）", code)
	}
	return lay
}

// -------- ② 符号层（符号级 · 贵层）---------------------------------------------------------

func impactLayerSymbol(root string, tgt *impactTarget, cheap bool) impactLayer {
	lay := impactLayer{
		Seq: "②", Name: "符号层", Grane: "符号级",
		Caliber: "algo=" + impactSymbolAlgo + "（`callgraph` 文本口径 · 不带算法名的边数不可比 —— 同仓同时刻模块内边 cha 6,049 vs rta 3,869 = 1.56 倍）",
	}
	if cheap {
		lay.Status = "未跑"
		lay.Detail = "**默认档**（§4.4：贵层按需）—— 这是**没跑**，不是「没影响」；要看「谁真的调我」给 `--all`（按需档）"
		return lay
	}
	if tgt.Kind != impactKindFile || !strings.HasSuffix(tgt.Rel, ".go") {
		lay.Status = "未适用"
		lay.Detail = "② 只对 `.go` 件目标取值（符号面）；契约 id / 非 Go 件没有符号边"
		return lay
	}
	bin := impactCallgraphBin()
	if bin == "" {
		lay.Status = "未装"
		lay.Detail = "`callgraph` 既不在 PATH、也不在 `~/go/bin` ⇒ 符号层**不取值**（宁少报不猜报；不许自造第二份符号面）"
		return lay
	}
	lay.Caliber += " · " + bin
	modDir, modPath := impactModuleDir(root, tgt.Rel)
	if modDir == "" || modPath == "" {
		lay.Status = "读不到"
		lay.Detail = "解析不到目标件所属 module ⇒ 符号面不给结论"
		return lay
	}
	syms := impactGoFileSymbols(filepath.Join(root, filepath.FromSlash(tgt.Rel)))
	if len(syms) == 0 {
		lay.Status = "未适用"
		lay.Detail = "目标件里没有顶层函数 / 方法 / 类型（或解析不了）⇒ 没有可查的符号"
		return lay
	}
	names := make([]string, 0, len(syms))
	for n := range syms {
		names = append(names, n)
	}
	sort.Strings(names)

	start := time.Now()
	cmd := exec.Command(bin, "-algo", impactSymbolAlgo, "./...")
	cmd.Dir = filepath.Join(root, filepath.FromSlash(modDir))
	var errb bytes.Buffer
	cmd.Stderr = &errb
	pipe, perr := cmd.StdoutPipe()
	if perr != nil {
		lay.Status = "读不到"
		lay.Detail = fmt.Sprintf("取 callgraph 输出管道失败：%v ⇒ 不给结论", perr)
		return lay
	}
	if serr := cmd.Start(); serr != nil {
		lay.Status = "读不到"
		lay.Detail = fmt.Sprintf("`callgraph` 起不来：%v ⇒ 不给结论", serr)
		return lay
	}
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	lines, modEdges := 0, 0
	dLine := map[string]bool{}
	dPair := map[string]bool{}
	rev := []map[string]string{}
	for sc.Scan() {
		ln := sc.Text()
		lines++
		dLine[ln] = true
		parts := strings.Split(ln, "\t")
		if len(parts) < 3 {
			continue
		}
		caller, edge, callee := parts[0], parts[1], parts[2]
		dPair[caller+"\x00"+callee] = true
		if strings.HasPrefix(caller, modPath) && strings.HasPrefix(callee, modPath) {
			modEdges++
		}
		if nm, ok := impactCalleeIsTarget(callee, modPath, pkgPathOf(modPath, modDir, tgt.Rel), syms); ok {
			callSite := strings.TrimSuffix(strings.TrimPrefix(edge, "--"), "-->")
			rev = append(rev, impactRow(
				"符号级:"+caller,
				"调用边",
				"它调 "+nm+"（调用点 "+callSite+"）—— "+impactSymbolAlgo+" 口径的反向边",
				""))
		}
	}
	serr := cmd.Wait()
	lay.Cost = time.Since(start)
	if serr != nil {
		lay.Status = "读不到"
		lay.Detail = fmt.Sprintf("`callgraph` 不退 0（%v · stderr: %s）⇒ 边集不完整，不给结论", serr, impactFirstLine(errb.String()))
		return lay
	}
	sort.Slice(rev, func(i, j int) bool { return rev[i]["what"] < rev[j]["what"] })
	if len(rev) > impactRowPage {
		rev = rev[:impactRowPage]
	}
	lay.Rows = rev
	lay.Status = "取值"
	// **三个数各归各位**（§7.6 判据 2 的 v1.4 纠错）：行 / 去重行 / 去重对 不许混着说。
	lay.Detail = fmt.Sprintf(
		"`callgraph -algo %s ./...` rc=0 · 全图 **行 %d** / **去重行 %d** / **去重对 %d** / **模块内边 %d**（双端都以 %s 开头）· 目标件符号 %d 个 · 反向边命中 %d 条 · 耗时 %s（热 Go cache）",
		impactSymbolAlgo, lines, len(dLine), len(dPair), modEdges, modPath, len(names), len(rev), lay.Cost.Round(time.Millisecond))
	return lay
}

// pkgPathOf 目标件的包 import path（与 ① 层同一口径）。
func pkgPathOf(modPath, modDir, rel string) string {
	dir := path.Dir(filepath.ToSlash(rel))
	if modDir != "" && modDir != "." {
		dir = strings.TrimPrefix(dir, modDir+"/")
	}
	if dir == "." || dir == "" {
		return modPath
	}
	return modPath + "/" + dir
}

// impactCalleeIsTarget 判一条边的**被调端**是不是目标件里的符号。
// `callgraph` 的符号形状：函数 `包路径.名`、方法 `包路径.类型.名` 或 `(*包路径.类型).名`
// （现跑样例逐字：`github.com/Mr2109/zerg-swarm/core/cmd/zerg.emitEnvelopeWith`）。
func impactCalleeIsTarget(callee, modPath, pkgPath string, syms map[string]int) (string, bool) {
	c := strings.TrimPrefix(callee, "(*")
	c = strings.TrimPrefix(c, "*")
	if pkgPath != "" && !strings.HasPrefix(c, pkgPath+".") {
		return "", false
	}
	rest := strings.TrimPrefix(c, pkgPath+".")
	rest = strings.TrimSuffix(rest, ")")
	seg := rest
	if i := strings.LastIndex(rest, "."); i >= 0 {
		seg = rest[i+1:]
	}
	if _, ok := syms[seg]; ok {
		return seg, true
	}
	return "", false
}

// -------- ③ 契约层（契约级）----------------------------------------------------------------

func impactLayerContract(root string, tgt *impactTarget) impactLayer {
	lay := impactLayer{
		Seq: "③", Name: "契约层", Grane: "契约级",
		Caliber: "`core/internal/contract/registry.json` 现读（**取数与跑它的门必须分开报** · `R35`）",
	}
	abs := filepath.Join(root, filepath.FromSlash(impactRegistryRel))
	b, err := os.ReadFile(abs)
	if err != nil {
		lay.Status = "读不到"
		lay.Detail = fmt.Sprintf("契约登记表读不到（%s）⇒ 「读不到」不当「没有」· 不给结论", impactRegistryRel)
		return lay
	}
	var doc struct {
		Entries []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Truth       string `json:"truth"`
			TruthShape  string `json:"truth_shape"`
			VersionF    string `json:"version_field"`
			Gate        string `json:"gate"`
			ChangeClass string `json:"change_class"`
			ChangeNote  string `json:"change_note"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		lay.Status = "读不到"
		lay.Detail = fmt.Sprintf("契约登记表解不开（%v）⇒ 不给结论", err)
		return lay
	}
	nLines := len(strings.Split(strings.TrimRight(string(b), "\n"), "\n"))
	lay.Caliber += fmt.Sprintf(" · registry.json %d 行 · %d 条 entries", nLines, len(doc.Entries))
	if len(doc.Entries) == 0 {
		lay.Status = "读不到"
		lay.Detail = "`entries[]` 一条都没有（**空表不许当「都不在册」**）"
		return lay
	}
	hits := 0
	for _, e := range doc.Entries {
		match := false
		if tgt.Kind == impactKindContract {
			match = e.ID == tgt.ID
		} else {
			base := path.Base(tgt.Rel)
			dir := path.Dir(tgt.Rel)
			match = strings.Contains(e.Truth, tgt.Rel) ||
				strings.Contains(e.Truth, dir+"/") ||
				(e.Gate != "" && strings.Contains(e.Gate, base))
		}
		if !match {
			continue
		}
		hits++
		red := ""
		if e.ChangeClass == "B" {
			red = e.ID // 「会红」闭集之一：契约 id（§3.5 铁律：语义级条目永不进「会红」行）
		}
		how := e.ChangeNote
		if how == "" {
			how = "改它要同时改登记表指向的真源（" + e.Truth + "）"
		}
		lay.Rows = append(lay.Rows, impactRow("契约级:"+e.ID, "契约", how, red))
	}
	lay.Status = "取值"
	lay.Detail = fmt.Sprintf("registry.json %d 行 / %d 条 · 目标命中 %d 条（口径：件的路径出现在 `truth`/`gate` 上，契约 id 逐字相等）· 取数毫秒",
		nLines, len(doc.Entries), hits)
	return lay
}

// -------- ④ 词法 + 形近层（文件级 + 名字级）-------------------------------------------------

func impactLayerLexical(root string, tgt *impactTarget) impactLayer {
	lay := impactLayer{
		Seq: "④", Name: "词法 + 形近层", Grane: "文件级 + 名字级",
		Caliber: "④a `git grep -w`（**词边界** · **只搜 tracked**）+ `git grep -E`（正则面 · 同一件的另一档）· ④b `ast-grep`（结构同形）",
	}
	word := tgt.ID
	if tgt.Kind == impactKindFile {
		word = path.Base(tgt.Rel)
	}
	// ④a-1 词边界（`R41`：默认口径写「词边界」，命中数必须带口径标记）。
	w1, _, c1, e1 := impactRunIn(root, "git", "grep", "-n", "-w", "--no-color", "--", word)
	// ④a-2 正则面（`-E`）：与 `-w` 差一个词边界（现读对照：同一查询 7 vs 3）。
	w2, _, c2, e2 := impactRunIn(root, "git", "grep", "-n", "-E", "--no-color", "--", regexpQuote(word))
	if e1 != nil || e2 != nil || c1 > 2 || c2 > 2 {
		lay.Status = "读不到"
		lay.Detail = fmt.Sprintf("`git grep` 起不来或报错（词边界 rc=%d / 正则 rc=%d）⇒ 词法面不给结论", c1, c2)
		return lay
	}
	lay.Caliber += " · 词=" + word + " · 口径标记：词边界=只搜 tracked（未跟踪件不在面内 ⇒ 与 `zerg code find` 的全盘口径**不同源**）"
	rows := []map[string]string{}
	add := func(out, why, how string, cap int) int {
		n := 0
		for _, ln := range strings.Split(out, "\n") {
			if strings.TrimSpace(ln) == "" {
				continue
			}
			n++
			if len(rows) >= cap {
				continue
			}
			file, line, text := impactSplitGrepLine(ln)
			rows = append(rows, impactRow("文件级:"+file+":"+line, why, how+"（"+impactTruncText(text, 80)+"）", ""))
		}
		return n
	}
	hits := add(w1, "词法", "词边界命中（`git grep -w`）", impactRowPage)
	hitsR := add(w2, "词法", "正则面命中（`git grep -E`）", impactRowPage)

	// ④b 形近（`ast-grep` · `R11` 已拍）：目标件的**顶层符号**的**调用形状**（`名($$$A)`）——
	// 结构面与词法面的分野：注释里提到 `foo` 只有词法面看得见，`foo(` 这种**调用形状**才是形近面。
	sg := impactAstGrepBin()
	sgRows, sgNote := 0, ""
	if sg == "" {
		sgNote = "④b 未装（`ast-grep` 不在 PATH）"
	} else if tgt.Kind != impactKindFile || !strings.HasSuffix(tgt.Rel, ".go") {
		sgNote = "④b 未适用（只对 `.go` 件取结构面）"
	} else {
		syms := impactGoFileSymbols(filepath.Join(root, filepath.FromSlash(tgt.Rel)))
		names := []string{}
		for n := range syms {
			names = append(names, n)
		}
		sort.Strings(names)
		if len(names) > 8 {
			names = names[:8] // 逐符号起进程：明说上限（前 8 个符号，字典序）
		}
		var verOut string
		if v, _, _, verr := impactRunIn(root, sg, "--version"); verr == nil {
			verOut = strings.TrimSpace(v)
		}
		modDir, _ := impactModuleDir(root, tgt.Rel)
		scanDir := root
		if modDir != "" {
			scanDir = filepath.Join(root, filepath.FromSlash(modDir))
		}
		for _, n := range names {
			if len(rows) >= impactRowPage*3 {
				break
			}
			out, _, code, err := impactRunIn(root, sg, "run", "--lang", "go", "--json",
				"-p", n+"($$$A)", scanDir)
			if err != nil || code != 0 || strings.TrimSpace(out) == "" {
				continue
			}
			var ms []struct {
				Text  string `json:"text"`
				Range struct {
					Start struct {
						Line int `json:"line"`
					} `json:"start"`
				} `json:"range"`
				File string `json:"file"`
			}
			if json.Unmarshal([]byte(out), &ms) != nil {
				continue
			}
			for _, m := range ms {
				sgRows++
				if len(rows) >= impactRowPage*3 {
					continue
				}
				rel := strings.TrimPrefix(filepath.ToSlash(m.File), filepath.ToSlash(root)+"/")
				rows = append(rows, impactRow("文件级:"+rel+":"+strconv.Itoa(m.Range.Start.Line+1), "形近",
					"结构同形（`ast-grep` 调用形状 "+n+"($$$A)）："+impactTruncText(m.Text, 80), ""))
			}
		}
		lay.Caliber += fmt.Sprintf(" · ④b 版本=%s · 符号上限 8（字典序前 8）", verOut)
		sgNote = fmt.Sprintf("④b 结构面命中 %d 处", sgRows)
	}
	lay.Rows = rows
	lay.Status = "取值"
	lay.Detail = fmt.Sprintf("④a 词边界 `git grep -w` %d 条 · 正则面 `git grep -E` %d 条（**两个数不同源**，别混用）· %s · 条数上限 %d/档（`truncated` 那一格今天在 `emitEnvelopeWith` 里是死的 ⇒ 上限走这里明说）",
		hits, hitsR, sgNote, len(rows))
	return lay
}

func regexpQuote(s string) string {
	return regexp.QuoteMeta(s)
}

// impactAstGrepBin 找 `ast-grep`（本机 0.45.3 · `R11` 已拍）。
func impactAstGrepBin() string {
	if p, err := exec.LookPath("ast-grep"); err == nil {
		return p
	}
	if p, err := exec.LookPath("sg"); err == nil {
		return p
	}
	return ""
}

func impactSplitGrepLine(ln string) (string, string, string) {
	parts := strings.SplitN(ln, ":", 3)
	if len(parts) < 3 {
		return ln, "", ""
	}
	return parts[0], parts[1], strings.TrimSpace(parts[2])
}

func impactCountLines(s string) int {
	n := 0
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
}

func impactCountNonEmpty(s string) int { return impactCountLines(s) }

func impactFirstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			return impactTruncText(strings.TrimSpace(ln), 160)
		}
	}
	return ""
}

func impactTruncText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// -------- ⑤ 语义层（按需 · 在位判据可测）---------------------------------------------------

// impactSemanticGate —— 第 ⑤ 层的**在位判据**（任务单 §二 `A2` 判据② · 逐字三条子句）：
// `/api/tags` 非空 **且** `capabilities` 含 `embedding` **且** `embedding_length` 有值。
// 抽成纯函数 ⇒ 成对负控能直接喂坏输入（不连真端点也能判红）。
func impactSemanticGate(tagsJSON []byte) (bool, string, string) {
	var doc struct {
		Models []struct {
			Name         string   `json:"name"`
			Capabilities []string `json:"capabilities"`
			Details      struct {
				EmbeddingLength int `json:"embedding_length"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.Unmarshal(tagsJSON, &doc); err != nil {
		return false, "", "「/api/tags」解不开：" + impactTruncText(err.Error(), 80)
	}
	if len(doc.Models) == 0 {
		return false, "", "子句① /api/tags 非空 ⇒ **否**（0 个模型）"
	}
	caps, dim, names := []string{}, 0, []string{}
	for _, m := range doc.Models {
		names = append(names, m.Name)
		caps = append(caps, m.Capabilities...)
		if m.Details.EmbeddingLength > 0 {
			dim = m.Details.EmbeddingLength
		}
	}
	hasEmb := false
	for _, c := range caps {
		if c == "embedding" {
			hasEmb = true
		}
	}
	detail := fmt.Sprintf("子句① /api/tags 非空 ⇒ **是**（%d 个模型：%s）· 子句② capabilities 含 embedding ⇒ **%s** · 子句③ embedding_length 有值 ⇒ **%s**（%d）",
		len(doc.Models), strings.Join(names, ","), yesno(hasEmb), yesno(dim > 0), dim)
	if !hasEmb || dim <= 0 {
		return false, "", detail
	}
	return true, names[0], detail
}

func yesno(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

// impactOllamaTags 现读本机 ollama 的 `/api/tags`（**只读** · 短超时 ⇒ 不在位就照实报不在位）。
func impactOllamaTags() ([]byte, error) {
	addr := os.Getenv("ZERG_OLLAMA")
	if addr == "" {
		addr = "http://127.0.0.1:11434"
	}
	cl := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := cl.Get(addr + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func impactLayerSemantic(root string, tgt *impactTarget) impactLayer {
	lay := impactLayer{
		Seq: "⑤", Name: "语义层（按需）", Grane: "件级",
		Caliber: "在位判据（可测三条子句）：/api/tags 非空 + capabilities 含 embedding + embedding_length 有值",
	}
	tags, err := impactOllamaTags()
	if err != nil {
		lay.Status = "未在位"
		lay.Detail = fmt.Sprintf("在位判据三子句**一条都没过**：连不上本机 ollama（%v）—— 照实报「未在位」，不猜", err)
	} else {
		ok, model, detail := impactSemanticGate(tags)
		lay.Detail = "在位判据现读：" + detail
		if !ok {
			lay.Status = "未在位"
		} else {
			lay.Caliber += " · model_id=" + model
			// 索引面（`R13`：`<状态目录>/impact-index/` 名**待 `A5` 拍板**）—— 建索引是低频批处理，
			// **不在查询路径上现建** ⇒ 索引不在就明说「未建索引」，**不拿模型在位冒充有召回**。
			idx := filepath.Join(stateDirOf(), "impact-index")
			if st, serr := os.Stat(idx); serr == nil && st.IsDir() {
				lay.Status = "取值"
				lay.Detail += " · 索引目录在盘上（" + idx + "）—— **取数实现属 `A5`**（本批不读索引、不落缓存）"
			} else {
				lay.Status = "未建索引"
				lay.Detail += " · 索引目录不在盘上（" + idx + "，目录名待 `A5` 拍板）⇒ **未建索引 ⇒ 不取数**" +
					"（v1.5 纠错⑦：中英同义对余弦 −0.0338 ⇒「有模型」≠「有召回」）"
			}
		}
	}
	// 运行期面（§十一 + `R34`）：**未查**，逐条点名四类盲区 —— 静态图天生看不见。
	lay.Detail += " · 运行期面（反射 / 配置串 / HTTP 路由 / 序列化）：**未查**" +
		"（`R34` 的「`zerg doctor` 名册四项 verdict 摘要进 `warnings[]` 一条」**本批未接**：六键包封的 `warnings` 今天恒 `[]` ⇒ 无落点）"
	return lay
}

// -------- ⑥ 公开面（件级 · 唯一产物不在本仓的一层）------------------------------------------

func impactLayerPublic(root string, tgt *impactTarget, cheap bool) impactLayer {
	lay := impactLayer{
		Seq: "⑥", Name: "公开面", Grane: "件级（件不是行）",
		Caliber: "判据面 = `" + impactPublishFaceSyncRel + "`（**只消费、不改它**）+ 产出树路径 + 扫的时刻 + `head_sha`",
	}
	// 命中判定（七件生效面 · 逐字相等 / 成员检查）。
	hit, why := impactOnPublishFace(root, tgt)
	lay.Detail = why
	if !hit {
		lay.Status = "未命中"
		lay.Detail += " ⇒ **不打「公开产出树 +N / −M 件」这一行**（判据③的负控面：这一行不是恒返回一行）"
		return lay
	}
	// 判据面现跑（`A2` 产物：复用、不改）。`--all` 之外（默认档）**不跑**它（秒级档 ⇒ 归按需），
	// 但 ±N / −M 的口径三件（产出树路径 + 扫的时刻 + `head_sha`）**照样取** —— 缺一才写「未取数」。
	if cheap {
		lay.Detail += " · 默认档：判据面 `" + impactPublishFaceSyncRel + "` **未跑**（秒级档 ⇒ 归按需档 `--all`）；±N/−M 的口径三件仍照取（下面逐件给）"
	} else {
		absScript := filepath.Join(root, filepath.FromSlash(impactPublishFaceSyncRel))
		out, errOut, code, err := impactRunIn(root, "python3", absScript)
		if err != nil {
			lay.Status = "读不到"
			lay.Detail += " · 判据面起不来 ⇒ **公开面：未取数**（口径三件缺一 ⇒ 只许写未取数，不编数字）"
			return lay
		}
		merged := out + "\n" + errOut
		lay.Caliber += fmt.Sprintf(" · 现跑 rc=%d · 候选 %s · 判据① 漂移 %s / 判据② 两边都漏 %s / 判据⑥ 表说 no %s",
			code, impactScriptNum(merged, "候选"), impactScriptNum(merged, "判据①"), impactScriptNum(merged, "判据②"), impactScriptNum(merged, "判据⑥"))
		lay.Detail += fmt.Sprintf(" · 判据面现跑 rc=%d（**只消费不改**）", code)
	}
	tree := os.Getenv("ZERG_PUB_TREE")
	if tree == "" {
		tree = impactPubTreeDefault
	}
	cnt, walkErr := impactCountTreeFiles(tree)
	at := impactEffectiveAt()
	if walkErr != nil {
		lay.Status = "命中（产出树读不到）"
		lay.Detail += fmt.Sprintf(" · 产出树路径 %s 读不到（%v）⇒ **公开面：未取数**（缺「产出树路径 + 扫的时刻 + head_sha」三件之一就不许出数）", tree, walkErr)
		return lay
	}
	// ±N / −M：**件级**口径 —— 这一改的**件**进出各算 1 件；方向由「今天在不在那棵树里」定
	// （第三者可复算：同一个 `find` 口径 + 同一个相对路径 `test -e`）。
	n, m := 0, 1
	if _, err := os.Stat(filepath.Join(tree, filepath.FromSlash(tgt.Rel))); err != nil {
		n, m = 1, 0
	}
	lay.Status = "命中"
	lay.Detail += fmt.Sprintf(" · 产出树 %s 现读 %d 件（`-type f` · 排 .git/vendor）· 扫的时刻 %s",
		tree, cnt, at)
	lay.Rows = append(lay.Rows, impactRow(
		"件级:"+tgt.Rel, "公开面",
		fmt.Sprintf("公开产出树 +%d / −%d 件（件级口径：这一件的进出各 1 件；整棵树的位移要重跑链，本层不拿单值冒充）", n, m),
		""))
	lay.PublicDelta = fmt.Sprintf("+%d / −%d", n, m)
	lay.PublicTree = fmt.Sprintf("%s（现读 %d 件 · -type f · 排 .git/vendor）", tree, cnt)
	lay.PublicAt = at
	return lay
}

// impactOnPublishFace 命中判定（成员检查 —— **链上判定**一律以判据面现跑输出为准）。
func impactOnPublishFace(root string, tgt *impactTarget) (bool, string) {
	if tgt.Kind != impactKindFile {
		return false, "目标不是件（契约 id）⇒ 没有「件」可落到生效面上"
	}
	for _, f := range impactPublishFace {
		if tgt.Rel == f {
			return true, "命中：目标**就是**七件生效面之一（" + f + "）"
		}
	}
	// 表里有行 / 清单里有它（逐行成员检查）。
	for _, f := range []string{"scripts/公开标记.tsv", "publish/whitelist.txt", "publish/mirror-allow.txt", "publish/private-paths.txt"} {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		for _, ln := range strings.Split(string(b), "\n") {
			ln = strings.TrimSpace(ln)
			if ln == "" || strings.HasPrefix(ln, "#") {
				continue
			}
			first := strings.FieldsFunc(ln, func(r rune) bool { return r == '\t' || r == ' ' })[0]
			if strings.TrimPrefix(first, "!") == tgt.Rel || strings.TrimPrefix(first, "./") == tgt.Rel {
				if f == "scripts/公开标记.tsv" {
					return true, "命中：目标在 `scripts/公开标记.tsv` 里有行（表判：" + ln + "）"
				}
				return true, "命中：目标在两器清单（`" + f + "`）里逐字出现"
			}
		}
	}
	return false, "未命中：目标不在七件生效面上、也不在标记表 / 两器清单里"
}

// impactCountTreeFiles 产出树件数（口径：`-type f` · 排 `.git` 与 `vendor` —— 与设计 §3.7 同一口径，
// **件不是行**）。读不到 ⇒ 报错（不许拿私有树件数冒充产出树件数）。
func impactCountTreeFiles(tree string) (int, error) {
	st, err := os.Stat(tree)
	if err != nil {
		return 0, err
	}
	if !st.IsDir() {
		return 0, fmt.Errorf("%s 不是目录", tree)
	}
	n := 0
	werr := filepath.Walk(tree, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		n++
		return nil
	})
	return n, werr
}

// impactScriptNum 从判据面现跑输出里取一个数（**只读它的输出**，不另抄一份解析器）。
// 口径：取含该关键字的那一行里的**第一个整数**（判据面的行形如 `判据⑥ 表说 no 的件两器都已排除：127 条`）。
func impactScriptNum(s, key string) string {
	for _, ln := range strings.Split(s, "\n") {
		if !strings.Contains(ln, key) {
			continue
		}
		for _, f := range strings.Fields(ln) {
			if n, err := strconv.Atoi(strings.Trim(f, "，,。:：")); err == nil {
				return strconv.Itoa(n)
			}
		}
	}
	return "—"
}
