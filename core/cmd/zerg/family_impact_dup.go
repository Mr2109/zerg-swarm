// family_impact_dup.go —— `C4` 重复面与归位。
//
// 依据（逐字）：`设计-变更影响面-v1.6` §6.5（重复面：**只报数**；官方健康分半衰期是
// **参照值**不是门槛）· 任务单-影响面实施-20260922 §四 `C4` 八字段判据①–④ 与四条红线 ·
// 契约真源 `core/internal/contract/impact-dup.json`（登记表 `S-l` · 本件只读它）。
//
// 四格判据的落点：
//
//	① **现跑值可复现**：`jscpd` 报数档 —— 处 / 行 / 行% / token / token% / 件 / 耗时，
//	   两个参数**成对写死**（`--min-tokens 50` · `--min-lines 5` ⇒ 只写一个就等于换尺）；
//	② **标定曲线同报**：20/30/**50**/70/100/150 六档逐档报「处」（同一棵树、只换 `--min-tokens`）；
//	③ **不设门禁阈值**：本仓 50 档远低于官方参照值 ⇒ 本批**不设阈值**（读数不进「会红」、
//	   不进任何退码；实现里也**没有**阈值分支）；
//	④ **茧壁门 rc=0**（既有面保持绿）+ **归位正面判据**：某目录「只许白名单 import」逐条报
//	   （标准库 / 本仓 `core/internal` / 本仓其它包；落到圈外**只报不拦** —— `R27`）。
//
// 关于「删除动作」：本件**唯一**一处删除 = 清掉**自己造的临时报告目录**（`os.MkdirTemp` 那一枚；
// jscpd 的报告只落在那儿）—— 仓内件 / 状态目录件 / 归档区 **一处都不碰**（红线）。
//
// 红线（逐条）：**不许设门禁阈值** ✗ · **不许引那两个外挂** ✗ ·
// **不许把重复面读数判红绿** ✗ · **不许为了让门禁变绿改既有文件或改 schema** ✗。
// 本件只读 + 只渲染 stderr（唯一落盘动作在 jscpd 自己的临时报告上，跑完即弃、不落仓、不落状态目录）。
package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// impactDupContractRel —— 重复面契约真源（登记表 `S-l` · **只读**）。
const impactDupContractRel = "core/internal/contract/impact-dup.json"

// impactDupSchema —— 契约件形状号（不认 ⇒ 不给数、不猜参数）。
const impactDupSchema = "zerg/impact-dup/1"

// impactDupContract —— 契约件（字段名与契约件键一一对应）。
type impactDupContract struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Tools  struct {
		Report struct {
			Bin      string   `json:"件"`
			VerCal   string   `json:"版本口径"`
			Pair     []string `json:"成对参数写死"`
			PairWhy  string   `json:"为什么成对"`
			NotAllow []string `json:"不许引"`
		} `json:"报数档"`
		Home struct {
			Bin     string `json:"件"`
			Positiv string `json:"正面判据"`
		} `json:"归位判据"`
	} `json:"两器"`
	Domain struct {
		Path    string `json:"路径"`
		Caliber string `json:"口径"`
		NoWhole string `json:"为什么不是整仓"`
		Known   string `json:"与在册值的关系"`
	} `json:"域"`
	Fields []string `json:"报数字段"`
	Curve  struct {
		Tiers   []int  `json:"档位"`
		Caliber string `json:"口径"`
		Known   string `json:"在册值"`
	} `json:"标定曲线"`
	Reference struct {
		HalfLife string `json:"官方健康分半衰期"`
		Caliber  string `json:"口径"`
	} `json:"参照值"`
	Discipline []string `json:"纪律"`
	NoActions  []string `json:"不许接的动作"`
}

// impactDupContractOf 现读契约件（只读真源；读不到 / 形状不认 / 成对参数缺一 ⇒ 明写原因、不给数）。
func impactDupContractOf(root string) (impactDupContract, string) {
	var c impactDupContract
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactDupContractRel)))
	if err != nil {
		return c, "契约件读不到（" + err.Error() + "）"
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, "契约件解不开（" + err.Error() + "）"
	}
	if c.Schema != impactDupSchema {
		return c, fmt.Sprintf("形状号不认（`schema`=%q ≠ %q ⇒ 不给数）", c.Schema, impactDupSchema)
	}
	if len(c.Tools.Report.Pair) != 2 || len(c.Curve.Tiers) == 0 {
		return c, "键不成形（`成对参数写死` 必须两格 / `标定曲线.档位` 非空 ⇒ 缺一不给数）"
	}
	return c, ""
}

// impactDupPair 成对参数（**必须两格** ⇒ 只写一个就等于换尺：契约件里也是这么写的）。
func impactDupPair(c impactDupContract) (minTokens, minLines int, ok bool) {
	mt, ml := 0, 0
	for _, p := range c.Tools.Report.Pair {
		fs := strings.Fields(strings.TrimSpace(p))
		if len(fs) != 2 {
			return 0, 0, false
		}
		n, err := strconv.Atoi(fs[1])
		if err != nil {
			return 0, 0, false
		}
		switch {
		case strings.Contains(fs[0], "min-tokens"):
			mt = n
		case strings.Contains(fs[0], "min-lines"):
			ml = n
		}
	}
	if mt <= 0 || ml <= 0 {
		return 0, 0, false
	}
	return mt, ml, true
}

// impactDupBin 找 `jscpd`（PATH 优先；本机只装在 npx 缓存里 ⇒ 再找 `~/.npm/_npx/*/node_modules/jscpd/bin/jscpd`）。
// 找不到 ⇒ 空串（本层标「未装」—— 宁少报不猜报，**不自己造一个重复度算法**）。
func impactDupBin() (string, string) {
	if p, err := exec.LookPath("jscpd"); err == nil {
		return p, "PATH"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	npx := filepath.Join(home, ".npm", "_npx")
	ents, err := os.ReadDir(npx)
	if err != nil {
		return "", ""
	}
	cands := []string{}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(npx, e.Name(), "node_modules", "jscpd", "bin", "jscpd")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			cands = append(cands, p)
		}
	}
	sort.Strings(cands)
	if len(cands) == 0 {
		return "", ""
	}
	return cands[len(cands)-1], "npx 缓存"
}

// impactDupReport 一次 jscpd 报数（**只报数**：处 / 行 / 行% / token / token% / 件 / 耗时）。
type impactDupReport struct {
	OK       bool
	Why      string
	Clones   int
	Lines    int
	LinePct  float64
	Tokens   int
	TokenPct float64
	Files    int
	DurMS    int64
	RC       int
	Out      string // 现跑输出末几行（人读；照原样）
}

// impactDupRunOnce 跑一次 jscpd（临时目录出报告 ⇒ 跑完即弃；**不在仓里、不在状态目录里留件**）。
func impactDupRunOnce(root, bin, domain string, minTokens, minLines int) impactDupReport {
	rep := impactDupReport{}
	tmp, err := os.MkdirTemp("", "zerg-jscpd-")
	if err != nil {
		rep.Why = "临时目录造不出来（" + err.Error() + "）"
		return rep
	}
	defer os.RemoveAll(tmp)
	start := time.Now()
	cmd := exec.Command(bin, "--reporters", "json", "--output", tmp,
		"--min-tokens", strconv.Itoa(minTokens), "--min-lines", strconv.Itoa(minLines),
		"--ignore", "**/node_modules/**,**/.git/**,**/vendor/**,**/target/**", domain)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	rep.DurMS = time.Since(start).Milliseconds()
	rep.Out = impactDupTail(string(out), 6)
	if ee, ok := err.(*exec.ExitError); ok {
		rep.RC = ee.ExitCode()
	} else if err != nil {
		rep.Why = "`jscpd` 起不来（" + err.Error() + "）"
		return rep
	}
	// 报告件名由 jscpd 定（`jscpd-report.json`）；找不到 ⇒ 只报耗时与 rc，不给数（**不猜**）。
	// 为什么走 walk 不走 `**` 通配：`filepath.Glob` 不认 `**`（那是 shell 的扩展）⇒ 自己找一遍。
	found := ""
	filepath.Walk(tmp, func(p string, info os.FileInfo, werr error) error {
		if werr != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Name() == "jscpd-report.json" && found == "" {
			found = p
		}
		return nil
	})
	if found == "" {
		rep.Why = "报告件没落下来（只给 rc 与耗时，不给数 —— 宁少报不猜报）"
		return rep
	}
	b, err := os.ReadFile(found)
	if err != nil {
		rep.Why = "报告件读不到（" + err.Error() + "）"
		return rep
	}
	// 报告形状（现读实测 · jscpd 4.3.0）：`statistics.total` 那七格才是报数档要的 ——
	// 处 = `clones` · 行 = `duplicatedLines` · 行% = `percentage` · token = `duplicatedTokens` ·
	// token% = `percentageTokens` · 件 = `sources`（扫描到的件数）。**百分比读工具自己给的**，
	// 不许自己拿两个数现算一个分母出来（口径：数字纪律）。
	var doc struct {
		Statistics struct {
			Total struct {
				Lines            int     `json:"lines"`
				Tokens           int     `json:"tokens"`
				Sources          int     `json:"sources"`
				Clones           int     `json:"clones"`
				DuplicatedLines  int     `json:"duplicatedLines"`
				DuplicatedTokens int     `json:"duplicatedTokens"`
				Percentage       float64 `json:"percentage"`
				PercentageTokens float64 `json:"percentageTokens"`
			} `json:"total"`
		} `json:"statistics"`
		Duplicates []struct {
			Lines  int `json:"lines"`
			Tokens int `json:"tokens"`
		} `json:"duplicates"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		rep.Why = "报告件解不开（" + err.Error() + "）"
		return rep
	}
	rep.OK = true
	rep.Clones = doc.Statistics.Total.Clones
	rep.Lines = doc.Statistics.Total.DuplicatedLines
	rep.Tokens = doc.Statistics.Total.DuplicatedTokens
	rep.Files = doc.Statistics.Total.Sources
	rep.LinePct = doc.Statistics.Total.Percentage
	rep.TokenPct = doc.Statistics.Total.PercentageTokens
	return rep
}

// impactDupTail 取现跑输出的末 N 行（照原样，不加工）。
func impactDupTail(s string, n int) string {
	ls := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(ls) > n {
		ls = ls[len(ls)-n:]
	}
	return strings.Join(ls, " ⏎ ")
}

// ---- 归位正面判据（某目录只许白名单 import · **只报不拦**）-----------------------------------

// impactImportRow 一条 import 的分类结果。
type impactImportRow struct {
	File string
	Path string
	Kind string // 标准库 / 本仓 internal / 本仓其它包 / 圈外
}

// impactImportClassify import path 三档（**正面判据的口径**：`R27`）。
//
//	① **标准库**：import path 第一段无 `.`（Go 的判据就是这一条）；
//	② **本仓 `core/internal/*`**：Go `internal` 语义面（同仓内可见、仓外不可见）；
//	③ **本仓其它包**：模块路径开头但不是 `internal/`；
//	④ 其余 ⇒ **圈外**（逐条列名 —— 只报不拦）。
func impactImportClassify(path string) string {
	if strings.HasPrefix(path, "github.com/Mr2109/zerg-swarm/core/internal/") {
		return "本仓 internal"
	}
	if strings.HasPrefix(path, "github.com/Mr2109/zerg-swarm/") {
		return "本仓其它包"
	}
	first := path
	if i := strings.Index(path, "/"); i > 0 {
		first = path[:i]
	}
	if !strings.Contains(first, ".") {
		return "标准库"
	}
	return "圈外"
}

// impactImportWhitelistDir 扫一个目录（**只看这一层** `.go` 件）的 import 逐条分类。
// 读不到目录 / 一件 `.go` 都没有 ⇒ 返回错（不给结论，不当「全白名单」）。
func impactImportWhitelistDir(root, rel string) ([]impactImportRow, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	ents, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	rows := []impactImportRow{}
	srcFiles := 0
	names := []string{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, nm := range names {
		f, err := parser.ParseFile(fset, filepath.Join(abs, nm), nil, parser.ImportsOnly)
		if err != nil {
			continue
		}
		srcFiles++
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			rows = append(rows, impactImportRow{File: rel + "/" + nm, Path: p, Kind: impactImportClassify(p)})
		}
	}
	if srcFiles == 0 {
		return nil, fmt.Errorf("%s 下没有可解析的 `.go` 件 ⇒ 不给结论（空与「全白名单」不是一回事）", rel)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		if rows[i].File != rows[j].File {
			return rows[i].File < rows[j].File
		}
		return rows[i].Path < rows[j].Path
	})
	return rows, nil
}

// ---- 茧壁门（既有面 · 判据④的现跑读数）------------------------------------------------------

// impactDupWallGate 现跑既有茧壁门（`scripts/gates/check-wall-layer.py`）—— 只读、只报 rc。
func impactDupWallGate(root string) (int, string, string) {
	rel := "scripts/gates/check-wall-layer.py"
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		return -1, "读不到", "茧壁门脚本不在盘上（" + rel + "）"
	}
	out, errOut, code, err := impactRunIn(root, "python3", rel)
	if err != nil {
		return -1, "读不到", "茧壁门起不来：" + err.Error()
	}
	return code, impactFirstLine(out), impactFirstLine(errOut)
}

// ---- 渲染（stderr · 报数档 · 两档跑法都由调用方定）-------------------------------------------

// emitImpactDupBlock 打重复面那一段（**只报数** · 不判红绿 · 不进退码）。
//
// `curve=true` 时另跑标定曲线的六档（贵：每档一次 jscpd ⇒ 只在**贵面现跑** `--deep` 走）。
func emitImpactDupBlock(w io.Writer, root string, curve bool) {
	c, why := impactDupContractOf(root)
	fmt.Fprintf(w, "%s: `C4` 重复面与归位（§6.5：**只报数不判红** · 不设门禁阈值）\n", progName)
	if why != "" {
		fmt.Fprintf(w, "  契约件 `%s`：%s ⇒ 本跑**不给数**（宁少报不猜报）\n", impactDupContractRel, why)
		return
	}
	mt, ml, ok := impactDupPair(c)
	if !ok {
		fmt.Fprintf(w, "  成对参数不成形（契约件 `%s` 的 `成对参数写死` 必须**两格**：只写一个就等于换尺）⇒ 不给数\n",
			impactDupContractRel)
		return
	}
	bin, from := impactDupBin()
	if bin == "" {
		fmt.Fprintf(w, "  报数档：`jscpd` **未装**（PATH 与 npx 缓存都没有）⇒ 本层**不取数**"+
			"（宁少报不猜报：不自己造一个重复度算法）\n")
	} else {
		rep := impactDupRunOnce(root, bin, c.Domain.Path, mt, ml)
		fmt.Fprintf(w, "  报数档 `jscpd`（%s · 来源 %s · 成对参数 `--min-tokens %d` + `--min-lines %d` **写死** · 域 = %q）\n",
			bin, from, mt, ml, c.Domain.Path)
		fmt.Fprintf(w, "    域口径：%s\n", c.Domain.Caliber)
		if strings.TrimSpace(c.Domain.NoWhole) != "" {
			fmt.Fprintf(w, "    整仓域为什么不给数：%s\n", c.Domain.NoWhole)
		}
		if !rep.OK {
			fmt.Fprintf(w, "    本跑**不给数**（%s · rc=%d · 耗时 %d ms）—— 只报能报的那几格：%s\n",
				rep.Why, rep.RC, rep.DurMS, rep.Out)
		} else {
			fmt.Fprintf(w, "    现跑值：**%d 处**克隆 · **%d 行**（%.2f%%）· **%d token**（%.2f%%）· **%d 件** · **耗时 %.3fs**\n",
				rep.Clones, rep.Lines, rep.LinePct, rep.Tokens, rep.TokenPct, rep.Files, float64(rep.DurMS)/1000.0)
			fmt.Fprintf(w, "    口径：两个参数成对写死（%s）；**只报数** —— 判红绿/设阈值都不在本面（读数不进「会红」、不进退码）\n",
				c.Tools.Report.PairWhy)
		}
	}
	// ② 标定曲线（同报 · 只换 `--min-tokens`）。
	if bin == "" {
		fmt.Fprintf(w, "  标定曲线：**未跑**（报数档未装）⇒ 在册值并留：%s\n", c.Curve.Known)
	} else if !curve {
		fmt.Fprintf(w, "  标定曲线：**本跑未跑**（六档 = 六次 jscpd ⇒ 归**贵面现跑** `--deep`）· 在册值并留：%s\n", c.Curve.Known)
	} else {
		parts := []string{}
		for _, t := range c.Curve.Tiers {
			r := impactDupRunOnce(root, bin, c.Domain.Path, t, ml)
			if !r.OK {
				parts = append(parts, fmt.Sprintf("%d 档=取不到", t))
				continue
			}
			parts = append(parts, fmt.Sprintf("%d 档=%d 处", t, r.Clones))
		}
		fmt.Fprintf(w, "  标定曲线（同报 · 只换 `--min-tokens`）：%s\n", strings.Join(parts, " · "))
		fmt.Fprintf(w, "    在册值并留：%s\n", c.Curve.Known)
		fmt.Fprintf(w, "    与在册值的关系：%s\n", c.Domain.Known)
	}
	// ③ 参照值 ≠ 门槛（照实打）。
	fmt.Fprintf(w, "  参照值（**不是门槛**）：官方健康分半衰期 %s —— %s\n",
		c.Reference.HalfLife, c.Reference.Caliber)
	// ④ 茧壁门 rc（既有面保持绿）+ 归位正面判据（只报不拦）。
	rc, first, efirst := impactDupWallGate(root)
	if rc < 0 {
		fmt.Fprintf(w, "  茧壁门（既有面）：**读不到**（%s）⇒ 不给结论\n", first)
	} else {
		fmt.Fprintf(w, "  茧壁门（既有面 · %s）：rc=%d（判据④：既有面保持绿）· %s%s\n",
			c.Tools.Home.Bin, rc, first, ternary(efirst != "", " · stderr: "+efirst, ""))
	}
	fmt.Fprintf(w, "  归位正面判据（只报不拦）：%s\n", c.Tools.Home.Positiv)
	// 归位口径落在**目标件所在目录**？本面是「新件归位」的通用判据 ⇒ 拿本族自己的目录当样例现读：
	// `core/cmd/zerg`（命令面）与 `core/internal/contract`（契约面）各报一份。
	for _, dir := range []string{"core/cmd/zerg", "core/internal/contract"} {
		rows, err := impactImportWhitelistDir(root, dir)
		if err != nil {
			fmt.Fprintf(w, "    %s：**不给结论**（%v）\n", dir, err)
			continue
		}
		cnt := map[string]int{}
		outside := []string{}
		for _, r := range rows {
			cnt[r.Kind]++
			if r.Kind == "圈外" {
				outside = append(outside, r.File+" ← "+r.Path)
			}
		}
		verdict := "是（**只许白名单 import** ⇒ 全在圈内）"
		if len(outside) > 0 {
			verdict = fmt.Sprintf("**否**（%d 条圈外）", len(outside))
		}
		fmt.Fprintf(w, "    %s：%s —— import %d 条 = 标准库 %d / 本仓 internal %d / 本仓其它包 %d / 圈外 %d\n",
			dir, verdict, len(rows), cnt["标准库"], cnt["本仓 internal"], cnt["本仓其它包"], cnt["圈外"])
		for _, o := range outside {
			fmt.Fprintf(w, "      · 圈外：%s（**只报不拦**：不新增阻断性闸 · `R27`）\n", o)
		}
	}
	fmt.Fprintf(w, "  纪律（照实打）：只报数不判红绿（读数**不进「会红」、不进退码**）· **不设门禁阈值** · "+
		"**不引** %s · **不为凑绿**改既有文件或改 schema · 不接的动作：%s\n",
		strings.Join(c.Tools.Report.NotAllow, " / "), strings.Join(c.NoActions, " / "))
}
