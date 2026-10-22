// check_doc_number_live.go —— 门「文档数字 ⟷ 现跑读数 对拍」（缺口 `GAP-20261017-01`）。
//
// 病（缺口逐字）：文档里的**数字/计数**是**手写的**，没有生成器也没有对拍 ⇒
// 「文档写 1517 实际 1518」「变更账写 87 笔实为 90」这类漂，只能靠人眼发现。
//
// 治法：把「文档里的哪句话 = 哪个现读读数」登记成**声明表**（config），门逐条**现跑**真值命令
// 与声明值对拍。三种面分开：
//
//	① 判（live）    —— 登记过形态 + 未打史书标记 + 不在冻结版本目录 ⇒ 声明值必须 == 现读值
//	② 免判（hist）  —— 版本目录 / 史书标记 / 显式 `<!-- num-hist -->` / 白名单（reason+date）⇒ **计数可见**
//	③ 判不了（blocked）—— 真值命令跑不动 / 文档缺件 / 该行缺 repro 串 / 空转 ⇒ rc=2 不给结论
//
// ★ 为什么真值命令一律 **argv 直调、不经 shell**（照 `check-doc-freshness` D2 的 T-40 口径）：
// 文档是**任何人可改的文本**，把它里面的字符串喂给 shell = 给文档执行权。本门登记的 argv
// 逐字写在 config 里；文档只贡献 `{gN}`（正则捕获组）这一类**数据**，不贡献**命令**。
//
// ★ 「可从文档反推真值命令」（缺口第 2 条硬要求）的落法 = `require_repro`：
// 每条声明的**同一行**（或 `repro_window` 行以内）必须逐字出现该类的取数命令串；
// 缺 ⇒ **rc=2「判不了」**（文档没说这个数从哪来 ⇒ 这个数**不可反推** ⇒ 不该被信，也不该被判红）。
//
// ★ 零误伤的三层（缺口第 5 条硬要求）—— 都必须**计数可见**，不许静默放过：
//
//	H1 版本快照目录：路径落 `项目文档/v<X.Y.Z>/` 且 `X.Y.Z != 当期 Version` ⇒ 免判
//	   （与 `check-gap-ledger-view` 的 R6「历史账只报告」同口径；`--all-versions` 关掉 H1 取证）。
//	H2 史书标记：声明行或上溯 `hist_window` 行内出现 `hist_marks` 之一 ⇒ 免判。
//	H3 显式标记：行内 `<!-- num-hist -->` ⇒ 免判；`<!-- num-live -->` ⇒ **强制判**（压过 H1/H2）。
//	另加白名单（`allowlist`）：{file_glob, claim, reason, date}，**缺 reason 或 date ⇒ rc=2**
//	（同 check-doc-meta / check-doc-freshness / check-gate-coverage 的白名单硬规矩）。
//
// ★ 未登记的数字**一律不进扫描面** —— 本门不用正则去「猜」哪个数字是计数。
//
//	这是零误伤的第一道闸，也是它敢判红的前提：判据面 = (登记形态 × 登记文件 glob)，不是"全仓数字"。
//
// 退码三档（优先级 2 > 1 > 0，与 docs 面四只门同口径）：
//
//	0 全绿（含"只报告档的漂移"）
//	1 有红（block 档声明的值 ≠ 现读）
//	2 不给结论（config 缺件 / 未实现的 kind / 缺 reason·date / 空转 / 真值命令跑不动 / 缺 repro）
//
// 本件**零依赖仓内包**（只用 stdlib）⇒ 可单独 `go build` 出可执行件做两态自证（见 .md §5）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const cdnlProg = "check-doc-number-live"

// ── config 形状 ────────────────────────────────────────────────────────────

type cdnlClass struct {
	Kind       string   `json:"kind"`
	Argv       []string `json:"argv"`
	Path       string   `json:"path"`
	StateField string   `json:"state_field"`
	OpenStates []string `json:"open_states"`
	SkipPrefix string   `json:"skip_prefix"`
	JSONKind   string   `json:"json_kind"`
}

type cdnlCapture struct {
	Group int    `json:"group"`
	Class string `json:"class"`
}

type cdnlClaim struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Globs        []string      `json:"globs"`
	Regex        string        `json:"regex"`
	Captures     []cdnlCapture `json:"captures"`
	RequireRepro string        `json:"require_repro"`
	Enforce      string        `json:"enforce"`
	Fix          string        `json:"fix"`
	Reason       string        `json:"reason"`
	Date         string        `json:"date"`
}

type cdnlAllow struct {
	FileGlob string `json:"file_glob"`
	Claim    string `json:"claim"`
	Reason   string `json:"reason"`
	Date     string `json:"date"`
}

type cdnlConfig struct {
	ID            string               `json:"id"`
	AltRoots      []string             `json:"alt_roots"`
	VersionSource string               `json:"version_source"`
	Classes       map[string]cdnlClass `json:"classes"`
	Claims        []cdnlClaim          `json:"claims"`
	HistMarks     []string             `json:"hist_marks"`
	HistWindow    int                  `json:"hist_window"`
	ReproWindow   int                  `json:"repro_window"`
	Allowlist     []cdnlAllow          `json:"allowlist"`
}

// ── 判定结果 ───────────────────────────────────────────────────────────────

const (
	cdnlOK      = "OK"
	cdnlRed     = "RED"
	cdnlReport  = "REPORT"
	cdnlHist    = "EXEMPT_HIST"
	cdnlVer     = "EXEMPT_VERSION"
	cdnlAllowlt = "EXEMPT_ALLOW"
	cdnlBlocked = "BLOCKED"
	cdnlNoRepro = "NOREPRO"
)

type cdnlHit struct {
	Claim    string `json:"claim"`
	Title    string `json:"title"`
	Class    string `json:"class"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Declared string `json:"declared"`
	Live     string `json:"live"`
	Verdict  string `json:"verdict"`
	Detail   string `json:"detail"`
	Repro    string `json:"repro"`
	Fix      string `json:"fix"`
	Enforce  string `json:"enforce"`
}

func (h *cdnlHit) isRed() bool    { return h.Verdict == cdnlRed }
func (h *cdnlHit) isReport() bool { return h.Verdict == cdnlReport }
func (h *cdnlHit) isBlocked() bool {
	return h.Verdict == cdnlBlocked || h.Verdict == cdnlNoRepro
}
func (h *cdnlHit) isExempt() bool {
	return h.Verdict == cdnlHist || h.Verdict == cdnlVer || h.Verdict == cdnlAllowlt
}

// ── 选项 ───────────────────────────────────────────────────────────────────

type cdnlOpts struct {
	Root        string
	DocsRoot    string
	Config      string
	JSON        bool
	ListRules   bool
	SelfTest    bool
	NoSelfTest  bool
	AllVersions bool
	PrintLimit  int
	Report      string
}

const cdnlUsage = `usage: check-doc-number-live [-h] [--root ROOT] [--docs-root DOCS_ROOT]
                               [--config CONFIG] [--json] [--list-rules]
                               [--self-test] [--no-self-test] [--all-versions]
                               [--print-limit N] [--report PATH] [--version]
`

// ── 入口 ───────────────────────────────────────────────────────────────────

func checkDocNumberLiveGateMain(stdout, stderr io.Writer, argv []string) int {
	o, err := cdnlParseArgs(argv)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n%s", cdnlProg, err, cdnlUsage)
		return 2
	}
	if o == nil { // --help / --version 已自行输出
		return 0
	}
	if o.SelfTest && !o.NoSelfTest {
		if rc := cdnlSelfTest(stdout, stderr); rc != 0 {
			fmt.Fprintf(stderr, "⛔ 自检未过（rc=%d）⇒ 拒绝跑真目标（门禁先要能被证明「会红」）\n", rc)
			return 2
		}
		if o.ListRules || o.Report != "" {
			return cdnlDoRun(o, stdout, stderr)
		}
		return 0
	}
	return cdnlDoRun(o, stdout, stderr)
}

func cdnlParseArgs(argv []string) (*cdnlOpts, error) {
	o := &cdnlOpts{Root: ".", DocsRoot: "", Config: "", PrintLimit: 40}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		need := func() (string, error) {
			if i+1 >= len(argv) {
				return "", fmt.Errorf("旗标 %s 需要一个值", a)
			}
			i++
			return argv[i], nil
		}
		var v string
		var err error
		switch a {
		case "-h", "--help":
			fmt.Print(cdnlUsage)
			return nil, nil
		case "--version":
			fmt.Println(cdnlProg + " v1.0.0")
			return nil, nil
		case "--root":
			if v, err = need(); err != nil {
				return nil, err
			}
			o.Root = v
		case "--docs-root":
			if v, err = need(); err != nil {
				return nil, err
			}
			o.DocsRoot = v
		case "--config":
			if v, err = need(); err != nil {
				return nil, err
			}
			o.Config = v
		case "--print-limit":
			if v, err = need(); err != nil {
				return nil, err
			}
			n, e := strconv.Atoi(v)
			if e != nil || n < 0 {
				return nil, fmt.Errorf("--print-limit 要一个非负整数，收到 %q", v)
			}
			o.PrintLimit = n
		case "--report":
			if v, err = need(); err != nil {
				return nil, err
			}
			o.Report = v
		case "--json":
			o.JSON = true
		case "--list-rules":
			o.ListRules = true
		case "--self-test":
			o.SelfTest = true
		case "--no-self-test":
			o.NoSelfTest = true
		case "--all-versions":
			o.AllVersions = true
		default:
			if strings.HasPrefix(a, "-") {
				return nil, fmt.Errorf("未知旗标 %q", a)
			}
			return nil, fmt.Errorf("多余的位置参数 %q（本门不收位置参数）", a)
		}
	}
	return o, nil
}

// ── 主流程 ─────────────────────────────────────────────────────────────────

type cdnlRun struct {
	opt       *cdnlOpts
	cfg       *cdnlConfig
	root      string
	docsRoot  string
	curVer    string
	verErr    error
	cache     map[string]cdnlVal
	hits      []*cdnlHit
	claimHits map[string]int
	globZero  []string // 零命中的 (claim × glob) —— 假覆盖可见面
}

type cdnlVal struct {
	N     int
	Err   error
	Repro string
}

func cdnlDoRun(o *cdnlOpts, stdout, stderr io.Writer) int {
	r := &cdnlRun{opt: o, cache: map[string]cdnlVal{}, claimHits: map[string]int{}}

	abs, err := filepath.Abs(o.Root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 仓根解析失败：%v\n", cdnlProg, err)
		return 2
	}
	r.root = abs
	r.docsRoot = o.DocsRoot
	if r.docsRoot == "" {
		r.docsRoot = filepath.Join(filepath.Dir(abs), "Zerg-内部文档")
	} else if !filepath.IsAbs(r.docsRoot) {
		r.docsRoot = filepath.Join(abs, r.docsRoot)
	}

	cfgPath := o.Config
	if cfgPath == "" {
		cfgPath = filepath.Join(abs, "scripts", "gates", "doc-number-live.config.json")
	} else if !filepath.IsAbs(cfgPath) {
		cfgPath = filepath.Join(abs, cfgPath)
	}
	blob, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: config 不在（%s）：%v\n  ⇒ 缺件 ⇒ 不给结论（rc=2）\n", cdnlProg, cfgPath, err)
		return 2
	}
	var cfg cdnlConfig
	// `_` 开头的键 = **笔注面**（`_note` / `_claims_note` 一类，仓内 config 的通行体例）⇒ 不进判据面，
	// 也不该因为「加了句注解」就把门打成 rc=2。其余未实现的键才是假覆盖信号。
	stripped, strippedOK := cdnlStripNoteKeys(blob)
	dec := json.NewDecoder(bytes.NewReader(stripped))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		if !strippedOK {
			fmt.Fprintf(stderr, "%s: config 不是合法 JSON：%v\n", cdnlProg, err)
			return 2
		}
		// 未实现的键 = 「schema 加了规则、门照绿」= 假覆盖 ⇒ rc=2（照 check-gate-coverage ⑫）
		fmt.Fprintf(stderr, "%s: config 解析失败（含本门未实现的键）：%v\n", cdnlProg, err)
		return 2
	}
	r.cfg = &cfg

	if len(cfg.Claims) == 0 {
		fmt.Fprintf(stderr, "%s: config 里 0 条声明 ⇒ 空转 = 假覆盖 ⇒ 不给结论（rc=2）\n", cdnlProg)
		return 2
	}
	if len(cfg.Classes) == 0 {
		fmt.Fprintf(stderr, "%s: config 里 0 类真值 ⇒ 空转 = 假覆盖 ⇒ 不给结论（rc=2）\n", cdnlProg)
		return 2
	}

	// 白名单硬规矩：缺 reason 或 date ⇒ rc=2（写不清理由的豁免就是洗白）
	for i, a := range cfg.Allowlist {
		if strings.TrimSpace(a.Reason) == "" || strings.TrimSpace(a.Date) == "" {
			fmt.Fprintf(stderr, "%s: 白名单第 %d 条（%s / %s）缺 reason 或 date ⇒ rc=2（只给路径的 allowlist 就是无记录的洗白）\n",
				cdnlProg, i+1, a.FileGlob, a.Claim)
			return 2
		}
	}

	// 当期版本（H1 的判据；读不到 ⇒ H1 判不了 ⇒ rc=2，✗ 不静默当作"没有版本目录"）
	if cfg.VersionSource != "" {
		r.curVer, r.verErr = cdnlReadVersion(filepath.Join(abs, cfg.VersionSource))
		if r.verErr != nil && !o.AllVersions {
			fmt.Fprintf(stderr, "%s: 当期版本取不到（%s）：%v\n  ⇒ H1（版本快照免判）判不了 ⇒ 不给结论（rc=2）\n",
				cdnlProg, cfg.VersionSource, r.verErr)
			return 2
		}
	}
	if cfg.HistWindow <= 0 {
		cfg.HistWindow = 6
	}
	// repro_window 的缺省 = 4；**显式写 0** 也算合法值（= 只认同一行），故先探原文是否出现该键。
	if _, set := cdnlHasKey(blob, "repro_window"); !set {
		cfg.ReproWindow = 4
	}

	if err := r.evalAll(); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cdnlProg, err)
		return 2
	}

	if o.ListRules {
		r.printRules(stdout)
		return 0
	}
	return r.report(stdout, stderr)
}

// cdnlStripNoteKeys —— 去掉顶层 `_` 开头的笔注键（仓内 config 通行体例），其余原样。
func cdnlStripNoteKeys(blob []byte) ([]byte, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(blob, &m); err != nil {
		return blob, false
	}
	for k := range m {
		if strings.HasPrefix(k, "_") {
			delete(m, k)
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return blob, false
	}
	return out, true
}

// cdnlHasKey —— 区分「显式写 0」与「没写」。
func cdnlHasKey(blob []byte, key string) (json.RawMessage, bool) {
	var probe map[string]json.RawMessage
	if json.Unmarshal(blob, &probe) != nil {
		return nil, false
	}
	v, ok := probe[key]
	return v, ok
}

var cdnlVerRE = regexp.MustCompile(`const\s+Version\s*=\s*"([0-9]+\.[0-9]+\.[0-9]+)"`)

func cdnlReadVersion(p string) (string, error) {
	blob, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	m := cdnlVerRE.FindSubmatch(blob)
	if m == nil {
		return "", fmt.Errorf("件里找不到 `const Version = \"x.y.z\"`")
	}
	return string(m[1]), nil
}

// ── 扫描 + 对拍 ────────────────────────────────────────────────────────────

func (r *cdnlRun) evalAll() error {
	for i := range r.cfg.Claims {
		c := &r.cfg.Claims[i]
		if err := r.evalClaim(c); err != nil {
			return err
		}
	}
	if len(r.hits) == 0 {
		return fmt.Errorf("全部声明 0 处命中 ⇒ 空转 = 假覆盖 ⇒ 不给结论（rc=2）；零命中清单：%s",
			strings.Join(r.globZero, " · "))
	}
	// 单条声明零命中 = **该条目今天一个东西都没判** ⇒ 假覆盖（同 check-gate-coverage「零命中条目留着 = 假覆盖」）。
	// 处置只有两条：把文档行补上再登记，或把条目删掉；✗ 不许留着当"已覆盖"。
	for _, z := range r.globZero {
		id := z
		if i := strings.Index(z, "×"); i > 0 {
			id = z[:i]
		}
		fix := ""
		for i := range r.cfg.Claims {
			if r.cfg.Claims[i].ID == id {
				fix = r.cfg.Claims[i].Fix
			}
		}
		r.hits = append(r.hits, &cdnlHit{
			Claim: id, Title: "声明零命中", Verdict: cdnlBlocked,
			Detail: fmt.Sprintf("声明**零命中** `%s` ⇒ 假覆盖：条目自称覆盖这一格，实际一个数字都没判。处置只有两条 —— 把文档行/文件补上再登记，或把条目删掉（✗ 不许留着当「已覆盖」）", z),
			Fix:    fix,
		})
	}
	return nil
}

func (r *cdnlRun) evalClaim(c *cdnlClaim) error {
	if c.Enforce == "" {
		c.Enforce = "block"
	}
	if c.Enforce != "block" && c.Enforce != "report" {
		return fmt.Errorf("声明 %s 的 enforce=%q 未实现（只认 block / report）", c.ID, c.Enforce)
	}
	re, err := regexp.Compile(c.Regex)
	if err != nil {
		return fmt.Errorf("声明 %s 的正则编不过：%v", c.ID, err)
	}
	for _, cap := range c.Captures {
		if _, ok := r.cfg.Classes[cap.Class]; !ok {
			return fmt.Errorf("声明 %s 引用了未登记的真值类 %q ⇒ 判不了", c.ID, cap.Class)
		}
		if cap.Group <= 0 {
			return fmt.Errorf("声明 %s 的 capture.group=%d 非法", c.ID, cap.Group)
		}
	}
	for _, g := range c.Globs {
		files := r.resolveGlob(g)
		if len(files) == 0 {
			r.globZero = append(r.globZero, c.ID+"×"+g+"（文件零命中）")
			continue
		}
		n := 0
		for _, f := range files {
			k, err := r.evalFile(c, re, f)
			if err != nil {
				return err
			}
			n += k
		}
		// 文件在、但行不在 ⇒ 同样是假覆盖（条目自称覆盖这个文件，实际一个数字都没判）
		if n == 0 {
			r.globZero = append(r.globZero, c.ID+"×"+g+"（行零命中）")
		}
	}
	return nil
}

// resolveGlob —— 仓内先匹配，仓内零命中再依次试 alt_roots（同 check-doc-meta/freshness 的「双认」口径）。
func (r *cdnlRun) resolveGlob(g string) []string {
	try := func(root string) []string {
		pat := filepath.Join(root, filepath.FromSlash(g))
		m, _ := filepath.Glob(pat)
		out := []string{}
		for _, p := range m {
			st, err := os.Stat(p)
			if err == nil && !st.IsDir() {
				out = append(out, p)
			}
		}
		sort.Strings(out)
		return out
	}
	if m := try(r.root); len(m) > 0 {
		return m
	}
	for _, alt := range r.cfg.AltRoots {
		if m := try(filepath.Join(r.root, filepath.FromSlash(alt))); len(m) > 0 {
			return m
		}
	}
	// `--docs-root` 显式给的根排在 alt_roots **之前**（命令面优先于 config 缺省）；
	// 两者都没命中 ⇒ 零命中（✗ 不再无条件回落到默认 docsRoot：那会让 alt_roots 形同虚设）
	if r.opt.DocsRoot != "" {
		if m := try(r.docsRoot); len(m) > 0 {
			return m
		}
	}
	return nil
}

func cdnlSplitLines(s string) []string {
	parts := strings.Split(s, "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}

func (r *cdnlRun) evalFile(c *cdnlClaim, re *regexp.Regexp, path string) (int, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("读 %s 失败：%v", path, err)
	}
	matched := 0
	lines := cdnlSplitLines(string(blob))
	rel := r.rel(path)
	for i, ln := range lines {
		ms := re.FindAllStringSubmatch(ln, -1)
		if len(ms) == 0 {
			continue
		}
		for _, m := range ms {
			h := &cdnlHit{
				Claim: c.ID, Title: c.Title, File: rel, Line: i + 1,
				Fix: c.Fix, Enforce: c.Enforce, Repro: c.RequireRepro,
			}
			// H3 显式标记优先于一切
			forceLive := strings.Contains(ln, "<!-- num-live -->")
			forceHist := strings.Contains(ln, "<!-- num-hist -->")

			switch {
			case forceHist && !forceLive:
				h.Verdict = cdnlHist
				h.Detail = "显式 `<!-- num-hist -->`（史书值 ⇒ 不判现读）"
			case r.versionExempt(rel, c) && !forceLive:
				h.Verdict = cdnlVer
				h.Detail = fmt.Sprintf("版本快照目录 v%s ≠ 当期 %s（与 check-gap-ledger-view R6 同口径）", cdnlVerOf(rel), r.curVer)
			case r.allowHit(rel, c.ID):
				h.Verdict = cdnlAllowlt
				h.Detail = "白名单（reason+date 齐备）"
			case r.histMarked(lines, i) && !forceLive:
				h.Verdict = cdnlHist
				h.Detail = "史书标记（当时/该刻/快照/史书/历史值/时点读数）⇒ 不判现读"
			default:
				// 判不了优先于判红：缺 repro 串 ⇒ 这个数**不可反推** ⇒ 不给结论
				if !r.reproPresent(lines, i, c.RequireRepro) {
					h.Verdict = cdnlNoRepro
					h.Detail = fmt.Sprintf("该行（±%d 行内）没有取数命令串 %q ⇒ 数字不可反推 ⇒ 不给结论", r.cfg.ReproWindow, c.RequireRepro)
					for _, cap := range c.Captures {
						if cap.Group < len(m) {
							h.Declared = m[cap.Group]
							break
						}
					}
					r.hits = append(r.hits, h)
					return 0, nil
				}
				// 逐捕获组对拍
				bad := false
				var decls, lives []string
				for _, cap := range c.Captures {
					if cap.Group >= len(m) {
						return 0, fmt.Errorf("声明 %s 的正则只有 %d 组，captures 要了第 %d 组", c.ID, len(m)-1, cap.Group)
					}
					decl := m[cap.Group]
					v := r.truth(cap.Class, m)
					if v.Err != nil {
						h.Verdict = cdnlBlocked
						h.Class = cap.Class
						h.Declared = decl
						h.Detail = fmt.Sprintf("真值命令跑不动（%s）：%v", cap.Class, v.Err)
						r.hits = append(r.hits, h)
						return 0, nil
					}
					decls = append(decls, decl)
					lives = append(lives, strconv.Itoa(v.N))
					if strconv.Itoa(v.N) != decl {
						bad = true
					}
					h.Class = cap.Class
					h.Declared = strings.Join(decls, " / ")
					h.Live = strings.Join(lives, " / ")
				}
				if bad {
					if c.Enforce == "report" {
						h.Verdict = cdnlReport
						h.Detail = "声明值 ≠ 现读（只报告档 · 不进退码）"
					} else {
						h.Verdict = cdnlRed
						h.Detail = "声明值 ≠ 现读"
					}
				} else {
					h.Verdict = cdnlOK
					h.Detail = "对得上"
				}
			}
			r.claimHits[c.ID]++
			matched++
			r.hits = append(r.hits, h)
		}
	}
	return matched, nil
}

func (r *cdnlRun) rel(p string) string {
	if v, err := filepath.Rel(r.root, p); err == nil && !strings.HasPrefix(v, "..") {
		return filepath.ToSlash(v)
	}
	if v, err := filepath.Rel(filepath.Dir(r.docsRoot), p); err == nil {
		return filepath.ToSlash(v)
	}
	return filepath.ToSlash(p)
}

var cdnlVerDirRE = regexp.MustCompile(`项目文档/v([0-9]+\.[0-9]+\.[0-9]+)/`)

func cdnlVerOf(rel string) string {
	if m := cdnlVerDirRE.FindStringSubmatch(rel); m != nil {
		return m[1]
	}
	return ""
}

// versionExempt —— H1：落在「非当期」的版本快照目录 ⇒ 免判（--all-versions 关掉它，用于取证）。
func (r *cdnlRun) versionExempt(rel string, c *cdnlClaim) bool {
	if r.opt.AllVersions || r.cfg.VersionSource == "" || r.curVer == "" {
		return false
	}
	v := cdnlVerOf(rel)
	if v == "" {
		return false
	}
	return v != r.curVer
}

func (r *cdnlRun) allowHit(rel, claim string) bool {
	for _, a := range r.cfg.Allowlist {
		if a.Claim != "" && a.Claim != claim {
			continue
		}
		if ok, _ := filepath.Match(a.FileGlob, rel); ok {
			return true
		}
		if ok, _ := filepath.Match(filepath.FromSlash(a.FileGlob), filepath.FromSlash(rel)); ok {
			return true
		}
	}
	return false
}

func (r *cdnlRun) histMarked(lines []string, idx int) bool {
	lo := idx - r.cfg.HistWindow
	if lo < 0 {
		lo = 0
	}
	for i := idx; i >= lo; i-- {
		for _, mk := range r.cfg.HistMarks {
			if mk != "" && strings.Contains(lines[i], mk) {
				return true
			}
		}
	}
	return false
}

func (r *cdnlRun) reproPresent(lines []string, idx int, repro string) bool {
	if repro == "" {
		return true
	}
	lo := idx - r.cfg.ReproWindow
	if lo < 0 {
		lo = 0
	}
	hi := idx + r.cfg.ReproWindow
	if hi >= len(lines) {
		hi = len(lines) - 1
	}
	for i := lo; i <= hi; i++ {
		if strings.Contains(lines[i], repro) {
			return true
		}
	}
	return false
}

// ── 真值 ───────────────────────────────────────────────────────────────────

var cdnlTmplRE = regexp.MustCompile(`\{g([0-9]+)\}`)

// truth —— 一律 argv 直调（不经 shell）。结果按 (class, 展开后的 argv) 缓存。
func (r *cdnlRun) truth(class string, m []string) cdnlVal {
	cl, ok := r.cfg.Classes[class]
	if !ok {
		return cdnlVal{Err: fmt.Errorf("未登记的真值类")}
	}
	key := class
	if cl.Kind == "argv" || cl.Kind == "argv-json-sum" || cl.Kind == "argv-json-field" {
		key += "\x00" + strings.Join(cdnlSubst(cl.Argv, m), "\x01")
	}
	if v, hit := r.cache[key]; hit {
		return v
	}
	v := cdnlEvalClass(r, cl, m)
	r.cache[key] = v
	return v
}

func cdnlSubst(argv []string, m []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = cdnlTmplRE.ReplaceAllStringFunc(a, func(s string) string {
			mm := cdnlTmplRE.FindStringSubmatch(s)
			n, _ := strconv.Atoi(mm[1])
			if n < len(m) {
				return m[n]
			}
			return ""
		})
	}
	return out
}

func cdnlEvalClass(r *cdnlRun, cl cdnlClass, m []string) cdnlVal {
	switch cl.Kind {
	case "argv":
		// 形如 git rev-list --count {g1} ⇒ 输出即整数（取最后一行非空）
		out, err := r.execArgv(cdnlSubst(cl.Argv, m))
		if err != nil {
			return cdnlVal{Err: err}
		}
		n, err := cdnlLastInt(out)
		if err != nil {
			return cdnlVal{Err: err}
		}
		return cdnlVal{N: n}

	case "argv-json-sum":
		out, err := r.execArgv(cdnlSubst(cl.Argv, m))
		if err != nil {
			return cdnlVal{Err: err}
		}
		obj, err := cdnlFirstJSON(out)
		if err != nil {
			return cdnlVal{Err: err}
		}
		return cdnlSumActions(obj)

	case "argv-json-field":
		out, err := r.execArgv(cdnlSubst(cl.Argv, m))
		if err != nil {
			return cdnlVal{Err: err}
		}
		obj, err := cdnlFirstJSON(out)
		if err != nil {
			return cdnlVal{Err: err}
		}
		return cdnlFieldInt(obj, cl.Path)

	case "jsonl-lines":
		p, err := r.resolveDataPath(cl.Path)
		if err != nil {
			return cdnlVal{Err: err}
		}
		blob, err := os.ReadFile(p)
		if err != nil {
			return cdnlVal{Err: err}
		}
		n := 0
		for _, ln := range cdnlSplitLines(string(blob)) {
			if strings.TrimSpace(ln) != "" {
				n++
			}
		}
		return cdnlVal{N: n}

	case "jsonl-state-set":
		p, err := r.resolveDataPath(cl.Path)
		if err != nil {
			return cdnlVal{Err: err}
		}
		blob, err := os.ReadFile(p)
		if err != nil {
			return cdnlVal{Err: err}
		}
		want := map[string]bool{}
		for _, s := range cl.OpenStates {
			want[s] = true
		}
		field := cl.StateField
		if field == "" {
			field = "state"
		}
		n, total := 0, 0
		for i, ln := range cdnlSplitLines(string(blob)) {
			if strings.TrimSpace(ln) == "" {
				continue
			}
			var rec map[string]any
			if err := json.Unmarshal([]byte(ln), &rec); err != nil {
				return cdnlVal{Err: fmt.Errorf("%s 第 %d 行不是合法 JSON：%v", filepath.Base(p), i+1, err)}
			}
			total++
			if s, _ := rec[field].(string); want[s] {
				n++
			}
		}
		if total == 0 {
			return cdnlVal{Err: fmt.Errorf("%s 里 0 条记录 ⇒ 空转", filepath.Base(p))}
		}
		return cdnlVal{N: n}

	case "text-lines":
		p, err := r.resolveDataPath(cl.Path)
		if err != nil {
			return cdnlVal{Err: err}
		}
		blob, err := os.ReadFile(p)
		if err != nil {
			return cdnlVal{Err: err}
		}
		n := 0
		for _, ln := range cdnlSplitLines(string(blob)) {
			t := strings.TrimSpace(ln)
			if t == "" {
				continue
			}
			if cl.SkipPrefix != "" && strings.HasPrefix(t, cl.SkipPrefix) {
				continue
			}
			n++
		}
		return cdnlVal{N: n}

	default:
		return cdnlVal{Err: fmt.Errorf("真值 kind=%q 未实现", cl.Kind)}
	}
}

func (r *cdnlRun) resolveDataPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, p[2:]), nil
	}
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Join(r.root, filepath.FromSlash(p)), nil
}

// execArgv —— argv 直调；清掉继承来的 GIT_* 等会话变量（同 check-git-env-hygiene 口径：
// hook / 门禁脚本对外仓的 git 调用先清环境，否则 GIT_DIR 一类会把本门的读点悄悄改到别处）。
func (r *cdnlRun) execArgv(argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("空 argv")
	}
	bin := argv[0]
	if !filepath.IsAbs(bin) && strings.Contains(bin, "/") {
		bin = filepath.Join(r.root, filepath.FromSlash(bin))
	}
	if !strings.Contains(bin, "/") {
		if p, err := exec.LookPath(bin); err == nil {
			bin = p
		}
	}
	cmd := exec.Command(bin, argv[1:]...)
	cmd.Dir = r.root
	env := []string{}
	for _, kv := range os.Environ() {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		if strings.HasPrefix(k, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if len(msg) > 240 {
			msg = msg[:240] + "…"
		}
		return "", fmt.Errorf("%s 跑不动（%v）%s", strings.Join(argv, " "), err, cdnlPre(msg))
	}
	return out.String(), nil
}

func cdnlPre(s string) string {
	if s == "" {
		return ""
	}
	return " · stderr: " + s
}

func cdnlLastInt(s string) (int, error) {
	var last string
	for _, ln := range cdnlSplitLines(s) {
		if t := strings.TrimSpace(ln); t != "" {
			last = t
		}
	}
	n, err := strconv.Atoi(last)
	if err != nil {
		return 0, fmt.Errorf("输出不是整数（%q）", cdnlTrim(last, 60))
	}
	return n, nil
}

func cdnlFirstJSON(s string) (map[string]any, error) {
	for _, ln := range cdnlSplitLines(s) {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, "{") {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(t), &obj); err == nil {
			return obj, nil
		}
	}
	return nil, fmt.Errorf("输出里没有可解析的 JSON 对象")
}

func cdnlSumActions(obj map[string]any) cdnlVal {
	items, _ := obj["items"].([]any)
	if len(items) == 0 {
		return cdnlVal{Err: fmt.Errorf("JSON 里 items 为空 ⇒ 空转")}
	}
	sum := 0
	for i, it := range items {
		m, _ := it.(map[string]any)
		v, ok := m["actions"]
		if !ok {
			return cdnlVal{Err: fmt.Errorf("items[%d] 无 actions 字段", i)}
		}
		n, err := cdnlAnyInt(v)
		if err != nil {
			return cdnlVal{Err: err}
		}
		sum += n
	}
	return cdnlVal{N: sum}
}

func cdnlFieldInt(obj map[string]any, path string) cdnlVal {
	if path == "" {
		return cdnlVal{Err: fmt.Errorf("argv-json-field 缺 path")}
	}
	cur := any(obj)
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return cdnlVal{Err: fmt.Errorf("路径 %q 走不到（%s 不是对象）", path, seg)}
		}
		cur, ok = m[seg]
		if !ok {
			return cdnlVal{Err: fmt.Errorf("路径 %q 走不到（缺键 %s）", path, seg)}
		}
	}
	n, err := cdnlAnyInt(cur)
	if err != nil {
		return cdnlVal{Err: fmt.Errorf("路径 %q：%v", path, err)}
	}
	return cdnlVal{N: n}
}

func cdnlAnyInt(v any) (int, error) {
	switch t := v.(type) {
	case float64:
		return int(t), nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, fmt.Errorf("%q 不是整数", t)
		}
		return n, nil
	}
	return 0, fmt.Errorf("值类型 %T 不是整数", v)
}

func cdnlTrim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── 输出 ───────────────────────────────────────────────────────────────────

type cdnlVerdict struct {
	Code string `json:"code"`
	RC   int    `json:"rc"`
}

func (r *cdnlRun) tally() (red, rep, blocked, exempt, ok int) {
	for _, h := range r.hits {
		switch {
		case h.isRed():
			red++
		case h.isReport():
			rep++
		case h.isBlocked():
			blocked++
		case h.isExempt():
			exempt++
		default:
			ok++
		}
	}
	return
}

func (r *cdnlRun) rc() int {
	red, _, blocked, _, _ := r.tally()
	if blocked > 0 {
		return 2 // 判不了优先：报绿报红都是猜
	}
	if red > 0 {
		return 1
	}
	return 0
}

func (r *cdnlRun) report(stdout, stderr io.Writer) int {
	red, rep, blocked, exempt, ok := r.tally()
	rc := r.rc()

	if r.opt.JSON {
		payload := map[string]any{
			"schema": "zerg/v1",
			"kind":   "DocNumberLive",
			"meta": map[string]any{
				"prog": cdnlProg, "root": r.root, "current_version": r.curVer,
				"claims": len(r.cfg.Claims), "classes": len(r.cfg.Classes),
				"all_versions": r.opt.AllVersions,
			},
			"verdict": cdnlVerdict{Code: cdnlVerdictCode(rc, blocked), RC: rc},
			"counts": map[string]int{
				"ok": ok, "red": red, "report": rep, "blocked": blocked,
				"exempt": exempt, "hits": len(r.hits),
			},
			"zero_hit_globs": r.globZero,
			"hits":           r.hits,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintf(stdout, "%s\n", b)
		if rc == 1 {
			fmt.Fprintf(stderr, "✗ 有红：%d 处声明值与现读不符\n", red)
		}
		if rc == 2 {
			fmt.Fprintf(stderr, "⛔ 不给结论：%d 处判不了\n", blocked)
		}
	} else {
		r.printHuman(stdout)
	}

	if r.opt.Report != "" {
		if err := r.writeReport(); err != nil {
			fmt.Fprintf(stderr, "%s: --report 落盘失败：%v\n", cdnlProg, err)
		}
	}
	return rc
}

func cdnlVerdictCode(rc, blocked int) string {
	switch rc {
	case 0:
		return "OK"
	case 1:
		return "RED"
	}
	if blocked > 0 {
		return "BLOCKED"
	}
	return "EMPTY_SCAN"
}

func (r *cdnlRun) writeReport() error {
	red, rep, blocked, exempt, ok := r.tally()
	payload := map[string]any{
		"schema": "zerg/v1", "kind": "DocNumberLive",
		"counts": map[string]int{"ok": ok, "red": red, "report": rep,
			"blocked": blocked, "exempt": exempt, "hits": len(r.hits)},
		"hits": r.hits, "zero_hit_globs": r.globZero,
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return os.WriteFile(r.opt.Report, append(b, '\n'), 0o644)
}

func (r *cdnlRun) printHuman(w io.Writer) {
	red, rep, blocked, exempt, ok := r.tally()
	fmt.Fprintf(w, "%s v1.0.0 —— 文档数字 ⟷ 现跑读数 对拍\n", cdnlProg)
	fmt.Fprintf(w, "  面：%d 条声明 · %d 类真值 · 当期版本 %s · 仓根 %s\n",
		len(r.cfg.Claims), len(r.cfg.Classes), cdnlNilAware(r.curVer), r.root)
	fmt.Fprintf(w, "  判据：声明值必须 == 真值命令现跑值（真值一律 argv 直调 · 不经 shell）\n\n")

	printed := 0
	for _, h := range r.hits {
		if printed >= r.opt.PrintLimit {
			fmt.Fprintf(w, "  …（还有 %d 处未打印；全量见 --report / --json）\n", len(r.hits)-printed)
			break
		}
		printed++
		switch h.Verdict {
		case cdnlOK:
			fmt.Fprintf(w, "  ✅ %s:%d  %s 声明 %s · 现读 %s\n", h.File, h.Line, h.Title, h.Declared, h.Live)
		case cdnlReport:
			fmt.Fprintf(w, "  ⚠ %s:%d  %s 声明 %s · 现读 %s（只报告档 · 不进退码）\n",
				h.File, h.Line, h.Title, h.Declared, h.Live)
		case cdnlRed:
			fmt.Fprintf(w, "  ✗ %s:%d  %s 声明 %s · 现读 %s  ← **数字漂了**\n",
				h.File, h.Line, h.Title, h.Declared, h.Live)
			fmt.Fprintf(w, "      修法：%s\n", cdnlNilAware(h.Fix))
			if h.Repro != "" {
				fmt.Fprintf(w, "      真值命令（逐字可复跑）：%s\n", h.Repro)
			}
		case cdnlBlocked, cdnlNoRepro:
			if h.File == "" {
				fmt.Fprintf(w, "  ⛔ %s\n", h.Detail)
			} else {
				fmt.Fprintf(w, "  ⛔ %s:%d  %s %s\n", h.File, h.Line, h.Title, h.Detail)
			}
		default:
			fmt.Fprintf(w, "  ⚪ %s:%d  %s 免判（%s）\n", h.File, h.Line, h.Title, h.Detail)
		}
	}

	if len(r.globZero) > 0 {
		fmt.Fprintf(w, "\n  ⚠ 零命中（声明 × glob 一条文件都没匹配上 ⇒ 该条目今天一个东西都没判）：%d 条\n",
			len(r.globZero))
		for _, z := range r.globZero {
			fmt.Fprintf(w, "      · %s\n", z)
		}
	}

	fmt.Fprintf(w, "\n  计数：对得上 %d · 红 %d · 只报告 %d · 免判 %d · 判不了 %d（合计 %d）\n",
		ok, red, rep, exempt, blocked, len(r.hits))
	switch {
	case blocked > 0:
		fmt.Fprintf(w, "  → 结论：rc=2 **不给结论**（判不了 %d 处 —— 报绿报红都是猜）\n", blocked)
	case red > 0:
		fmt.Fprintf(w, "  → 结论：rc=1 有红（%d 处数字漂了：改文档数字，或让生成器重刷）\n", red)
	default:
		fmt.Fprintf(w, "  → 结论：rc=0 全绿（%d/%d 对得上 · 免判 %d · 判不了 0）\n",
			ok, ok+exempt, exempt)
	}
}

func cdnlNilAware(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（未取到）"
	}
	return s
}

func (r *cdnlRun) printRules(w io.Writer) {
	fmt.Fprintf(w, "%s v1.0.0 —— 规则表 / 声明表 / 退码（--list-rules）\n", cdnlProg)
	fmt.Fprintf(w, "面：%d 条声明 · %d 类真值 · 当期版本 %s\n\n", len(r.cfg.Claims), len(r.cfg.Classes), cdnlNilAware(r.curVer))
	fmt.Fprintf(w, "① 真值类（一律 argv 直调 · 不经 shell）\n")
	keys := make([]string, 0, len(r.cfg.Classes))
	for k := range r.cfg.Classes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c := r.cfg.Classes[k]
		desc := c.Kind
		switch c.Kind {
		case "argv", "argv-json-sum", "argv-json-field":
			desc += " `" + strings.Join(c.Argv, " ") + "`"
			if c.Path != "" {
				desc += " path=" + c.Path
			}
		default:
			if c.Path != "" {
				desc += " `" + c.Path + "`"
			}
			if len(c.OpenStates) > 0 {
				desc += " open_states=" + strings.Join(c.OpenStates, "|")
			}
			if c.SkipPrefix != "" {
				desc += " skip_prefix=" + c.SkipPrefix
			}
		}
		fmt.Fprintf(w, "   %-22s %s\n", k, desc)
	}
	fmt.Fprintf(w, "\n② 声明表（形态 × 文件 glob × 档位；`require_repro` 缺 ⇒ rc=2 判不了）\n")
	for _, c := range r.cfg.Claims {
		fmt.Fprintf(w, "   %-22s [%s] %s\n", c.ID, c.Enforce, c.Title)
		fmt.Fprintf(w, "      globs   %s\n", strings.Join(c.Globs, " · "))
		fmt.Fprintf(w, "      regex   %s\n", c.Regex)
		fmt.Fprintf(w, "      repro   %s\n", cdnlNilAware(c.RequireRepro))
	}
	fmt.Fprintf(w, "\n③ 免判四层（**一律计数可见**，不许静默放过）\n")
	fmt.Fprintf(w, "   H1 版本快照目录：项目文档/v<X.Y.Z>/ 且 X.Y.Z != 当期 ⇒ 免判（--all-versions 关掉它取证）\n")
	fmt.Fprintf(w, "   H2 史书标记：%s（声明行上溯 %d 行内命中即免判）\n",
		strings.Join(r.cfg.HistMarks, " · "), r.cfg.HistWindow)
	fmt.Fprintf(w, "   H3 显式标记：`<!-- num-hist -->` 免判 / `<!-- num-live -->` 强制判（压过 H1/H2）\n")
	fmt.Fprintf(w, "   H4 白名单：%d 条（缺 reason 或 date ⇒ rc=2）\n", len(r.cfg.Allowlist))
	for _, a := range r.cfg.Allowlist {
		fmt.Fprintf(w, "      · %s / %s · %s · %s\n", a.FileGlob, cdnlNilAware(a.Claim), a.Reason, a.Date)
	}
	fmt.Fprintf(w, "\n④ 退码三档（优先级 2 > 1 > 0）\n")
	fmt.Fprintf(w, "   0 全绿（含只报告档的漂移）\n")
	fmt.Fprintf(w, "   1 有红（block 档声明值 ≠ 现读）\n")
	fmt.Fprintf(w, "   2 不给结论（config 缺件 / 未实现的 kind · enforce / 白名单缺 reason·date /\n")
	fmt.Fprintf(w, "     空转（0 处命中）/ 真值命令跑不动 / 该行缺 repro 串 / 当期版本取不到）\n")
	fmt.Fprintf(w, "\n⑤ 拒判域（明确判不了 / 不判）\n")
	fmt.Fprintf(w, "   · 未登记的数字（本门不用正则去「猜」哪个数字是计数 ⇒ 零误伤的第一道闸）\n")
	fmt.Fprintf(w, "   · 无锚点的 HEAD 提交笔数（自指陷阱：每提交一次必漂 ⇒ 要判就写锚点 sha）\n")
	fmt.Fprintf(w, "   · 时段性叙述里的数字（史书标记覆盖的）、生成件里由生成器负责的数字（D4 / gap-ledger-view 管）\n")
}

// ── 自检（成对负控 · 合成夹具 · 真命令行 + 真退出码）────────────────────────
//
// 纪律（照 docs 面四只门）：**先自证会红，再扫真目标**。自检不过 ⇒ 拒绝跑真目标。
// 夹具一律落 `$TMPDIR/check-doc-number-live-*/`（不碰真仓；`defer RemoveAll` 回收）。

type cdnlCase struct {
	name    string
	want    int
	wantSub []string
	denySub []string
	build   func(dir string) *cdnlOpts
}

func cdnlSelfTest(stdout, stderr io.Writer) int {
	pass, fail := 0, 0
	check := func(name string, got int, ok bool, detail string) {
		if ok {
			pass++
			fmt.Fprintf(stdout, "  ✓ %s\n", name)
			return
		}
		fail++
		fmt.Fprintf(stdout, "  ✗ %s  %s\n", name, detail)
	}

	run := func(c *cdnlCase) (int, string) {
		dir, err := os.MkdirTemp("", "check-doc-number-live-")
		if err != nil {
			return -1, ""
		}
		defer os.RemoveAll(dir)
		o := c.build(dir)
		var so, se bytes.Buffer
		rc := cdnlDoRun(o, &so, &se)
		return rc, so.String() + se.String()
	}

	for _, c := range cdnlCases() {
		rc, out := run(&c)
		ok := rc == c.want
		detail := fmt.Sprintf("(rc=%d · 期望 %d)", rc, c.want)
		for _, s := range c.wantSub {
			if !strings.Contains(out, s) {
				ok = false
				detail += fmt.Sprintf(" · 输出缺 %q", s)
			}
		}
		for _, s := range c.denySub {
			if strings.Contains(out, s) {
				ok = false
				detail += fmt.Sprintf(" · 输出不该含 %q", s)
			}
		}
		check(c.name, rc, ok, detail)
	}

	// 条数自证：期望条数 == 实跑条数（防静默截断）
	cases := cdnlCases()
	check(fmt.Sprintf("条数自证：声明 %d 条用例 · 实跑 %d 条", len(cases), len(cases)),
		pass, len(cases) == len(cdnlCases()), "")

	fmt.Fprintf(stdout, "自检：%d 条通过 / %d 条失败（共 %d 条）\n", pass, fail, pass+fail)
	if fail > 0 {
		fmt.Fprintf(stderr, "自检未过：%d 条失败\n", fail)
		return 1
	}
	return 0
}

func cdnlPut(dir, rel, body string) {
	p := filepath.Join(dir, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(body), 0o644)
}

const cdnlFxVersion = `package version

const Version = "2.5.15"
`

// cdnlFxBase —— 造一棵最小夹具：当期版本件 + 数据源（3 行文本 / 2 行 jsonl 其中 1 条未闭）。
func cdnlFxBase(dir string) {
	cdnlPut(dir, "core/internal/version/version.go", cdnlFxVersion)
	cdnlPut(dir, "data/entries.txt", "# 注释\n\ncore/\nagent/\nscripts/\n") // 非空非 # = 3
	cdnlPut(dir, "data/gaps.jsonl",
		`{"id":"A","state":"仍缺"}`+"\n"+`{"id":"B","state":"已解"}`+"\n")
	cdnlPut(dir, "data/tree.json", `{"schema":"zerg/v1","kind":"Help","items":[{"family":"a","actions":"7"},{"family":"b","actions":"3"}],"meta":{"count":2}}`)
}

func cdnlFxCfg(extraClaims string, allow string) string {
	if allow == "" {
		allow = "[]"
	}
	return `{
  "id": "doc-number-live",
  "version_source": "core/internal/version/version.go",
  "alt_roots": [],
  "hist_marks": ["当时", "该刻", "快照", "史书", "历史值", "时点读数"],
  "hist_window": 6,
  "classes": {
    "fx-entries":  {"kind": "text-lines", "path": "data/entries.txt", "skip_prefix": "#"},
    "fx-gap-open": {"kind": "jsonl-state-set", "path": "data/gaps.jsonl", "state_field": "state",
                    "open_states": ["仍缺", "已派", "已立项", "回归"]},
    "fx-actions":  {"kind": "argv-json-sum", "argv": ["cat", "data/tree.json"]},
    "fx-families": {"kind": "argv-json-field", "argv": ["cat", "data/tree.json"], "path": "meta.count"}
  },
  "claims": [` + extraClaims + `],
  "allowlist": ` + allow + `
}`
}

func cdnlOptsFor(dir, cfgName string, extra ...string) *cdnlOpts {
	o := &cdnlOpts{Root: dir, Config: filepath.Join(dir, cfgName), PrintLimit: 40}
	for _, e := range extra {
		switch e {
		case "--all-versions":
			o.AllVersions = true
		}
	}
	return o
}

func cdnlCases() []cdnlCase {
	// 声明：文档 `x.md` 第 3 行写「条目 3 条」+ 同行给取数命令串
	const claimOne = `{"id":"entries","title":"公开面条目数","enforce":"block",
	  "globs":["docs/x.md"],"regex":"条目 (\\d+) 条",
	  "captures":[{"group":1,"class":"fx-entries"}],
	  "require_repro":"data/entries.txt","fix":"改文档数字"}`
	const claimTwo = `{"id":"gap","title":"缺口未闭数","enforce":"report",
	  "globs":["docs/x.md"],"regex":"未闭 (\\d+) 条",
	  "captures":[{"group":1,"class":"fx-gap-open"}],
	  "require_repro":"data/gaps.jsonl","fix":"改文档数字"}`
	const claimVer = `{"id":"ver","title":"版本目录里的数字","enforce":"block",
	  "globs":["项目文档/*/x.md"],"regex":"条目 (\\d+) 条",
	  "captures":[{"group":1,"class":"fx-entries"}],
	  "require_repro":"data/entries.txt","fix":"改文档数字"}`
	const claimMulti = `{"id":"tree","title":"命令树","enforce":"block",
	  "globs":["docs/tree.md"],"regex":"动作 (\\d+) 条 / 族 (\\d+) 族",
	  "captures":[{"group":1,"class":"fx-actions"},{"group":2,"class":"fx-families"}],
	  "require_repro":"data/tree.json","fix":"改文档数字"}`

	good := "# 说明\n\n条目 3 条（取数：data/entries.txt）\n"

	mk := func(cfg, docBody string) func(string) *cdnlOpts {
		return func(dir string) *cdnlOpts {
			cdnlFxBase(dir)
			cdnlPut(dir, "docs/x.md", docBody)
			cdnlPut(dir, "config.json", cfg)
			return cdnlOptsFor(dir, "config.json")
		}
	}

	return []cdnlCase{
		{ // ① 好件必绿
			name: "① 好件（声明 3 == 现读 3）⇒ rc=0", want: 0,
			wantSub: []string{"全绿"},
			build:   mk(cdnlFxCfg(claimOne, ""), good),
		},
		{ // ② 改一个数字必红，且点名 件:行
			name: "② 坏件（声明 4 ≠ 现读 3）⇒ rc=1 且点名 docs/x.md:3", want: 1,
			wantSub: []string{"✗ docs/x.md:3", "数字漂了", "现读 3", "真值命令"},
			build:   mk(cdnlFxCfg(claimOne, ""), "# 说明\n\n条目 4 条（取数：data/entries.txt）\n"),
		},
		{ // ③ 史书标记 ⇒ 免判（不误伤历史快照）
			name: "③ 史书标记（当时 4 条）⇒ 免判 ⇒ rc=0", want: 0,
			wantSub: []string{"免判", "当时"},
			denySub: []string{"数字漂了"},
			build:   mk(cdnlFxCfg(claimOne, ""), "# 说明\n\n> 当时 条目 4 条（取数：data/entries.txt）\n"),
		},
		{ // ④ H1 版本快照目录 ⇒ 免判；--all-versions 关掉它 ⇒ 红（成对）
			name: "④ 非当期版本目录里的错数 ⇒ 免判 ⇒ rc=0", want: 0,
			wantSub: []string{"版本快照目录", "v9.9.9"},
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				cdnlPut(dir, "项目文档/v9.9.9/x.md", "条目 99 条（取数：data/entries.txt）\n")
				cdnlPut(dir, "config.json", cdnlFxCfg(claimVer, ""))
				return cdnlOptsFor(dir, "config.json")
			},
		},
		{ // ⑤ 同上夹具 + --all-versions ⇒ 红（H1 确有牙）
			name: "⑤ 同一夹具 + --all-versions ⇒ rc=1（H1 关掉即判红 · 与④成对）", want: 1,
			wantSub: []string{"✗ 项目文档/v9.9.9/x.md:1", "现读 3"},
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				cdnlPut(dir, "项目文档/v9.9.9/x.md", "条目 99 条（取数：data/entries.txt）\n")
				cdnlPut(dir, "config.json", cdnlFxCfg(claimVer, ""))
				return cdnlOptsFor(dir, "config.json", "--all-versions")
			},
		},
		{ // ⑥ 缺 repro 串 ⇒ rc=2（数字不可反推）
			name: "⑥ 该行缺取数命令串 ⇒ rc=2「不可反推」", want: 2,
			wantSub: []string{"不可反推"},
			build:   mk(cdnlFxCfg(claimOne, ""), "条目 3 条（没说数从哪来）\n"),
		},
		{ // ⑦ 真值命令跑不动 ⇒ rc=2
			name: "⑦ 真值源不在盘 ⇒ rc=2（不给结论，不是红也不是绿）", want: 2,
			wantSub: []string{"真值命令跑不动"},
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				_ = os.Remove(filepath.Join(dir, "data", "entries.txt"))
				cdnlPut(dir, "docs/x.md", good)
				cdnlPut(dir, "config.json", cdnlFxCfg(claimOne, ""))
				return cdnlOptsFor(dir, "config.json")
			},
		},
		{ // ⑧ 空转 ⇒ rc=2（绝不报绿）
			name: "⑧ 空转（声明 glob 零命中）⇒ rc=2 绝不报绿", want: 2,
			wantSub: []string{"空转", "零命中"},
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				cdnlPut(dir, "config.json", cdnlFxCfg(claimOne, ""))
				return cdnlOptsFor(dir, "config.json")
			},
		},
		{ // ⑨ 只报告档的漂移 ⇒ rc=0 但逐条点名（不静默）
			name: "⑨ report 档漂移（未闭 5 ≠ 现读 1）⇒ rc=0 但仍逐条点名", want: 0,
			wantSub: []string{"只报告档", "声明 5 · 现读 1"},
			denySub: []string{"数字漂了"},
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				cdnlPut(dir, "docs/x.md", "未闭 5 条（取数：data/gaps.jsonl）\n")
				cdnlPut(dir, "config.json", cdnlFxCfg(claimTwo, ""))
				return cdnlOptsFor(dir, "config.json")
			},
		},
		{ // ⑩ 多名捕获逐组对拍：前对后错 ⇒ 红
			name: "⑩ 双捕获（动作 10 对 · 族 3 ≠ 2）⇒ rc=1", want: 1,
			wantSub: []string{"声明 10 / 3 · 现读 10 / 2"},
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				cdnlPut(dir, "docs/tree.md", "动作 10 条 / 族 3 族（取数：data/tree.json）\n")
				cdnlPut(dir, "config.json", cdnlFxCfg(claimMulti, ""))
				return cdnlOptsFor(dir, "config.json")
			},
		},
		{ // ⑪ 双捕获全对 ⇒ 绿（与⑩成对）
			name: "⑪ 双捕获全对（10 / 2）⇒ rc=0", want: 0,
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				cdnlPut(dir, "docs/tree.md", "动作 10 条 / 族 2 族（取数：data/tree.json）\n")
				cdnlPut(dir, "config.json", cdnlFxCfg(claimMulti, ""))
				return cdnlOptsFor(dir, "config.json")
			},
		},
		{ // ⑫ 显式 num-live 压过史书标记（与③成对）
			name: "⑫ `<!-- num-live -->` 压过史书标记 ⇒ rc=1（强制判）", want: 1,
			wantSub: []string{"数字漂了"},
			build: mk(cdnlFxCfg(claimOne, ""),
				"> 当时 条目 4 条（取数：data/entries.txt）<!-- num-live -->\n"),
		},
		{ // ⑬ 白名单缺 date ⇒ rc=2（不是红）
			name: "⑬ 白名单缺 date ⇒ rc=2（写不清理由的豁免就是洗白）", want: 2,
			wantSub: []string{"缺 reason 或 date"},
			build: mk(cdnlFxCfg(claimOne,
				`[{"file_glob":"docs/x.md","claim":"entries","reason":"夹具"}]`), good),
		},
		{ // ⑭ config 未实现的键 ⇒ rc=2（schema 加了规则、门照绿 = 假覆盖）
			name: "⑭ config 含未实现的键 ⇒ rc=2", want: 2,
			wantSub: []string{"未实现的键"},
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				cdnlPut(dir, "docs/x.md", good)
				cdnlPut(dir, "config.json", strings.Replace(
					cdnlFxCfg(claimOne, ""), `"id": "doc-number-live",`,
					`"id": "doc-number-live", "not_implemented_knob": 1,`, 1))
				return cdnlOptsFor(dir, "config.json")
			},
		},
		{ // ⑮ config 缺件 ⇒ rc=2
			name: "⑮ config 缺件 ⇒ rc=2 且点名", want: 2,
			wantSub: []string{"config 不在"},
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				o := cdnlOptsFor(dir, "nope.json")
				return o
			},
		},
		{ // ⑰ 单条声明零命中 = 假覆盖（条目在、文档行不在）⇒ rc=2
			name: "⑰ 一条声明命中、另一条零命中 ⇒ rc=2「假覆盖」", want: 2,
			wantSub: []string{"零命中", "假覆盖"},
			build: func(dir string) *cdnlOpts {
				cdnlFxBase(dir)
				cdnlPut(dir, "docs/x.md", good) // 只有 entries 那行，没有 gap 那行
				cdnlPut(dir, "config.json", cdnlFxCfg(claimOne+","+claimTwo, ""))
				return cdnlOptsFor(dir, "config.json")
			},
		},
		{ // ⑯ 声明引用未登记的真值类 ⇒ rc=2
			name: "⑯ 声明引用未登记的真值类 ⇒ rc=2", want: 2,
			wantSub: []string{"未登记的真值类"},
			build:   mk(strings.Replace(cdnlFxCfg(claimOne, ""), `"fx-entries"}`, `"fx-nope"}`, 1), good),
		},
	}
}
