// family_impact_del.go —— `C1` 删面（**只做「不碰标记」那一半**）。
//
// 依据（逐字）：`设计-变更影响面-v1.6` §6.6（删面的**文档联动**：恒带一行「文档面：<N> 篇（口径 B）」·
// `N = 0` 写「面内未见」）+ §十 `R19` / `R20` / `R31` / `R37`（照做四条）与 `R28`（**已拍**：
// 「能找回的只报，找不回的必拦」）；任务单-影响面实施-20260922 §四 `C1` 八字段判据①–⑥；
// 契约真源 `core/internal/contract/impact-del.json`（登记表 `S-m` · 本件只读它）。
//
// 本件**只做**这半（`R18` / `R21` 未拍 ⇒ 一个字都不碰）：
//
//	① 「**面内未见引用**」**候选**清单（恒带「候选」措辞 · **不许读成「可删」**）；
//	② **五类盲区字段**（反射 / 注册表 / 配置串 / 序列化 / 外部 API · **缺一不出结论**）；
//	③ **两件可找回证据**（被删对象 `sha256` + 一条找回路径 · **缺一不出**）；
//	④ **「整件搬出」与「符号级删」分行报**（`R37`：`git --diff-filter` 的 A/D/M 当分类器）；
//	⑤ **零**自动化删除****（无第二条路径 · 本件里没有任何删除动作）；
//	⑥ **不可机检**：删面误报率（`H9`）**未跑** ⇒ 「候选」只作提示（照实标）。
//
// 两器（都只读、都消费、都不改）：`deadcode`（**必带 `-test`**：不带 ⇒ 85.1% 是「只被测试引用」·
// `R19`）+ `staticcheck` 的 `U1000`（第二判据 · `R20`）；`go vet` 无此项 ⇒ **不进删面**。
//
// 红线（逐条）：**不许有第二条删路径** ✗ · **不许 `deadcode` 不带 `-test`** ✗ ·
// **不许把两器的数写成「可删 N 条」** ✗ · **不许批准件 / 门禁判据随动** ✗ ·
// **不许写标记 / 不许批量动作**（`R18` / `R21` 未拍）✗ · **不许动归档区** ✗。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// impactDelContractRel —— 删面契约真源（登记表 `S-m` · **只读**）。
const impactDelContractRel = "core/internal/contract/impact-del.json"

// impactDelSchema —— 契约件形状号（不认 ⇒ 不给结论、不猜口径）。
const impactDelSchema = "zerg/impact-del/1"

// impactDelBlindKeys —— 五类盲区的**固定五格**（顺序即报告顺序；缺一 ⇒ 不出结论）。
var impactDelBlindKeys = []string{"反射", "注册表", "配置串", "序列化", "外部API"}

// impactDelContract —— 契约件（字段名与契约件键一一对应）。
type impactDelContract struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Tools  struct {
		Main struct {
			Bin   string `json:"件"`
			Must  string `json:"必带"`
			Why   string `json:"为什么必带"`
			Known string `json:"在册值"`
		} `json:"主判据"`
		Second struct {
			Bin   string `json:"件"`
			Known string `json:"在册值"`
			Why   string `json:"为什么当第二判据"`
		} `json:"第二判据"`
		Never map[string]string `json:"不进删面"`
	} `json:"两器"`
	Blind map[string]string `json:"五类盲区"`
	Proof map[string]string `json:"两件可找回证据"`
	Class struct {
		Bin   string `json:"件"`
		Tiers string `json:"两档"`
		Known string `json:"在册实测"`
		Rule  string `json:"纪律"`
	} `json:"分类器"`
	ZeroAuto string   `json:"零**自动化删除**"`
	NotMech  []string `json:"不可机检"`
	DocLink  struct {
		Caliber string `json:"口径"`
		Line    string `json:"行形"`
		Zero    string `json:"零命中"`
	} `json:"文档联动"`
	Redlines []string `json:"红线"`
}

// impactDelContractOf 现读契约件（只读真源；读不到 / 形状不认 / 五类盲区少一格 ⇒ 明写原因、不给结论）。
func impactDelContractOf(root string) (impactDelContract, string) {
	var c impactDelContract
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactDelContractRel)))
	if err != nil {
		return c, "契约件读不到（" + err.Error() + "）"
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, "契约件解不开（" + err.Error() + "）"
	}
	if c.Schema != impactDelSchema {
		return c, fmt.Sprintf("形状号不认（`schema`=%q ≠ %q ⇒ 不给结论）", c.Schema, impactDelSchema)
	}
	for _, k := range impactDelBlindKeys {
		if _, ok := c.Blind[k]; !ok {
			return c, fmt.Sprintf("五类盲区少一格（`%s`）⇒ **缺一不出结论**（判据①）", k)
		}
	}
	if strings.TrimSpace(c.Tools.Main.Must) == "" || !strings.Contains(c.Tools.Main.Must, "-test") {
		return c, "契约件没把 `-test` 写成主判据的必带项 ⇒ 不给结论（`R19`）"
	}
	return c, ""
}

// impactDelCandidate 一条候选（两器之一给的）。
type impactDelCandidate struct {
	Tool string // deadcode / staticcheck
	File string // 相对 module 目录
	Line int
	Sym  string
}

// impactDelBin 找两器（PATH 优先 → `~/go/bin/<名>`）。找不到 ⇒ 空串（照实标「未装」，不造第二套算法）。
func impactDelBin(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, "go", "bin", name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// impactDelParseRows 解析两器的输出行（`<件>:<行>:<列>: …`）——**两器共用一份切法**（不另造第二套）。
func impactDelParseRows(tool, out string) []impactDelCandidate {
	rows := []impactDelCandidate{}
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		parts := strings.SplitN(ln, ":", 4)
		if len(parts) < 4 {
			continue
		}
		line, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			continue
		}
		sym := strings.TrimSpace(parts[3])
		rows = append(rows, impactDelCandidate{Tool: tool, File: filepath.ToSlash(parts[0]), Line: line, Sym: sym})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Tool != rows[j].Tool {
			return rows[i].Tool < rows[j].Tool
		}
		if rows[i].File != rows[j].File {
			return rows[i].File < rows[j].File
		}
		return rows[i].Line < rows[j].Line
	})
	return rows
}

// impactDelRun 跑一只器（**只读** · 工作时 = module 目录）：返回 (候选, rc, 首行, 耗时, err)。
//
// 纪律（本件自己的）：`deadcode` **必须带 `-test`**（不带就不许跑）；两器都只给**候选**，
// 谁都不许把输出写成「可删 N 条」。
func impactDelRun(modAbs, bin string, args ...string) ([]impactDelCandidate, int, string, time.Duration, error) {
	start := time.Now()
	cmd := exec.Command(bin, args...)
	cmd.Dir = modAbs
	var sb, eb strings.Builder
	cmd.Stdout, cmd.Stderr = &sb, &eb
	err := cmd.Run()
	dur := time.Since(start)
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
			err = nil
		} else {
			return nil, -1, impactFirstLine(eb.String()), dur, err
		}
	}
	return impactDelParseRows(filepath.Base(bin), sb.String()), code, impactFirstLine(eb.String()), dur, nil
}

// impactDelEvidence 两件可找回证据之一/之二（判据③）：
//
//	① 被删对象的 `sha256`（**现算**）；
//	② 一条找回路径：`git log -1 --format=%H -- <件>` 那一笔提交（可 `git show <sha>:<件>` 取回）·
//	   或**归档副本**的现读路径。
//
// **缺一 ⇒ 不出**（返回的 `Missing` 非空即「缺」）。
func impactDelEvidence(root, rel string) (sha string, path string, missing string) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if st, err := os.Stat(abs); err == nil && !st.IsDir() {
		f, err := os.Open(abs)
		if err == nil {
			h := sha256.New()
			if _, err := io.Copy(h, f); err == nil {
				sha = hex.EncodeToString(h.Sum(nil))
			}
			f.Close()
		}
	}
	sha = strings.TrimSpace(sha)
	// 找回路径：git 那一笔（**只在仓内件上**）。
	if out, _, code, err := impactRunIn(root, "git", "log", "-1", "--format=%H", "--", rel); err == nil && code == 0 {
		if s := strings.TrimSpace(out); s != "" {
			path = "git 提交 " + s + "（`git show " + s[:minInt(12, len(s))] + ":" + rel + "` 可取回）"
		}
	}
	// 归档副本面（**只读**：只 `test -e`，绝不写归档区）。
	if path == "" {
		if p := impactDelArchiveCopy(root, rel); p != "" {
			path = "归档副本 " + p + "（现读 `test -e` 通过）"
		}
	}
	if sha == "" {
		missing = "`sha256` 取不到（件不在盘上 / 读不了）"
	}
	if path == "" {
		if missing != "" {
			missing += " · "
		}
		missing += "找回路径取不到（既没有可指名的提交、也没有归档副本 ⇒ **找不回的必拦** · `R28`）"
	}
	return sha, path, missing
}

// impactDelArchiveCopy 在**归档区**里按**同相对路径**找那份副本（只读 · `test -e`）。
func impactDelArchiveCopy(root, rel string) string {
	base := strings.TrimSpace(os.Getenv("ZERG_ARCHIVE_ROOT"))
	if base == "" {
		base = filepath.Join(filepath.Dir(root), "Zerg-归档")
	}
	cands := []string{filepath.Join(base, filepath.FromSlash(rel))}
	if ents, err := os.ReadDir(base); err == nil {
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			cands = append(cands, filepath.Join(base, e.Name(), filepath.Base(rel)))
		}
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

// impactDelClassify 分类器（`R37`）：按「该件的**顶层符号**是不是**全都**在候选里」分两档 ——
//
//	**整件搬出**（该件全部顶层符号都判为未引用 ⇒ 判「只报」+ 两件证据）
//	**符号级删**（只剩部分符号 ⇒ 判「必拦」侧）
//
// 两档**分行报**，不许合成一个数。
func impactDelClassify(root, fileRel string, cand map[string]bool) (tier string, total int, hit int) {
	syms := impactGoFileSymbols(filepath.Join(root, filepath.FromSlash(fileRel)))
	total = len(syms)
	if total == 0 {
		return "判不了（该件解析不出顶层符号 ⇒ 只记不判）", 0, 0
	}
	for n := range syms {
		if cand[fileRel+"\x00"+n] {
			hit++
		}
	}
	if hit == total {
		return "整件搬出（该件顶层符号全在候选里 ⇒ 判「只报」+ 两件可找回证据）", total, hit
	}
	return fmt.Sprintf("符号级删（候选只覆盖 %d/%d 个顶层符号 ⇒ 判「必拦」侧 —— `R28`）", hit, total), total, hit
}

// emitImpactDelBlock 打删面那一段（**只报候选** · 零**自动化删除** · 不写标记）。
//
// `runTools=false`（默认档）时**一个字都不许少**：五类盲区**恒带**、两器状态照实写「未跑（贵面现跑 --deep）」、
// 并明说「未跑 ≠ 没有候选」。
func emitImpactDelBlock(w io.Writer, root string, tgt *impactTarget, layout impactStateLayout,
	layoutOK bool, runTools bool) {
	c, why := impactDelContractOf(root)
	fmt.Fprintf(w, "%s: `C1` 删面（**只出候选** · 一次删除动作都没有 · 不写标记 —— `R18`/`R21` 未拍 ⇒ 不碰）\n", progName)
	if why != "" {
		fmt.Fprintf(w, "  契约件 `%s`：%s ⇒ 本面**不给结论**（缺一不出）\n", impactDelContractRel, why)
		return
	}
	// 五类盲区（**恒带五格** ⇒ 缺一不出结论的那一半就落在这里）。
	fmt.Fprintf(w, "  五类盲区（**恒带字段 · 缺一不出结论** · 判据①）：")
	for _, k := range impactDelBlindKeys {
		fmt.Fprintf(w, " %s=%s；", k, c.Blind[k])
	}
	fmt.Fprintf(w, "\n")
	// 两器（照实：跑没跑、带没带 `-test`、域是什么）。
	modDir, modPath := "", ""
	if tgt != nil && tgt.Kind == impactKindFile {
		modDir, modPath = impactModuleDir(root, tgt.Rel)
	}
	if modDir == "" {
		fmt.Fprintf(w, "  两器：**未跑**（解析不到目标件所属 module ⇒ 不知道跑哪棵树）\n")
	} else {
		fmt.Fprintf(w, "  域：module `%s`（%s）—— 两器都在这个域上跑（换域 = 换一把尺）\n", modDir, modPath)
	}
	dead, sc := impactDelBin("deadcode"), impactDelBin("staticcheck")
	// 文档联动行（`C3` 倒排 · 口径 B）——**恒带**（判据：删面恒带这一行）。
	if tgt != nil && tgt.Kind == impactKindFile {
		doc := impactDocFaceLookup(root, tgt, layout, layoutOK, false, "B")
		if doc.Status == "取值" {
			line := fmt.Sprintf("文档面：%d 篇（口径 B · 反引号包住的符号名 —— `C3` 倒排现读）", doc.Docs)
			if doc.Docs == 0 {
				line = "文档面：**面内未见**（`N = 0` · 口径 B）—— 说的是**这一片里没人提**，不是「无文档受影响」✗"
			}
			fmt.Fprintf(w, "  %s\n", line)
		} else {
			fmt.Fprintf(w, "  文档面：**未取数**（%s —— 索引不在 / 读不到；口径 B）\n", doc.Status)
		}
	} else {
		fmt.Fprintf(w, "  文档面：**本行未适用**（目标不是件）\n")
	}
	if !runTools {
		fmt.Fprintf(w, "  两器：**本跑未跑**（贵面 ⇒ 归**贵面现跑** `--deep`）：`%s`（必带 `-test`）· `%s`（`U1000`）\n",
			c.Tools.Main.Bin, c.Tools.Second.Bin)
		fmt.Fprintf(w, "  ★ 候选清单：**本跑没跑 ⇒ 不给候选** —— **未跑 ≠ 没有候选**（宁少报不猜报）\n")
	} else {
		modAbs := filepath.Join(root, filepath.FromSlash(modDir))
		all := []impactDelCandidate{}
		if dead == "" {
			fmt.Fprintf(w, "  主判据 `deadcode`：**未装**（PATH 与 `~/go/bin` 都没有）⇒ 本面**不给候选**\n")
		} else {
			// `-test` **必带**（契约件写死；缺了就报「口径不成形」而不是照跑）。
			rows, rc, first, dur, err := impactDelRun(modAbs, dead, "-test", "./...")
			if err != nil {
				fmt.Fprintf(w, "  主判据 `deadcode -test ./...`：**起不来**（%v）⇒ 不给候选\n", err)
			} else {
				all = append(all, rows...)
				fmt.Fprintf(w, "  主判据 `deadcode %s ./...`（**`-test` 必带** · rc=%d · 耗时 %s）：**候选 %d 条**%s\n",
					c.Tools.Main.Must, rc, dur.Round(time.Millisecond), len(rows),
					ternary(first == "", "", " · 首行: "+first))
			}
		}
		if sc == "" {
			fmt.Fprintf(w, "  第二判据 `staticcheck`：**未装** ⇒ 只有一条判据（照实标）\n")
		} else {
			rows, rc, first, dur, err := impactDelRun(modAbs, sc, "-checks=U1000", "./...")
			if err != nil {
				fmt.Fprintf(w, "  第二判据 `staticcheck -checks=U1000 ./...`：**起不来**（%v）\n", err)
			} else {
				all = append(all, rows...)
				fmt.Fprintf(w, "  第二判据 `staticcheck -checks=U1000 ./...`（rc=%d · 耗时 %s）：**候选 %d 条**%s\n",
					rc, dur.Round(time.Millisecond), len(rows), ternary(first == "", "", " · 首行: "+first))
			}
		}
		if dead == "" && sc == "" {
			fmt.Fprintf(w, "  ★ 候选清单：**两器都没得跑 ⇒ 不给候选**（`go vet` 无「未使用」检查项 ⇒ 不进删面 · `R20`）\n")
		} else {
			emitImpactDelCandidates(w, root, tgt, all)
		}
	}
	// 分类器口径 + 判断不了的那两条（照实标）。
	fmt.Fprintf(w, "  分类器（`%s`）：%s · 在册实测 %s · %s\n", c.Class.Bin, c.Class.Tiers, c.Class.Known, c.Class.Rule)
	fmt.Fprintf(w, "  删除面：%s\n", c.ZeroAuto)
	for _, n := range c.NotMech {
		fmt.Fprintf(w, "  ★ 不可机检（照实标）：%s\n", n)
	}
	fmt.Fprintf(w, "  红线（照实打）：%s\n", strings.Join(c.Redlines, " · "))
}

// emitImpactDelCandidates 逐条候选（**恒带「候选」措辞** + 两件可找回证据 + 分行报两档）。
func emitImpactDelCandidates(w io.Writer, root string, tgt *impactTarget, rows []impactDelCandidate) {
	if len(rows) == 0 {
		fmt.Fprintf(w, "  ★ 候选清单：**0 条**（两器现跑都没有未引用项）—— 这一格说的是「**本域内没发现**」，不是「没有死代码」\n")
		return
	}
	byFile := map[string]map[string]bool{}
	for _, r := range rows {
		if byFile[r.File] == nil {
			byFile[r.File] = map[string]bool{}
		}
		byFile[r.File][strings.TrimSpace(strings.TrimPrefix(r.Sym, "unreachable func:"))] = true
	}
	files := []string{}
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	fmt.Fprintf(w, "  ★ 候选清单（**候选** = 「本域内没找到引用」的**提示**，**不是「可删」**· 判据⑥：误报率 `H9` 未跑）：**%d 件 / %d 条**\n",
		len(files), len(rows))
	whole, sym := 0, 0
	for _, f := range files {
		rel := strings.TrimPrefix(f, "./")
		// 两档分行（`R37`）：整件搬出 vs 符号级删。
		// 判据口径里的「件」是**仓内相对路径**：这里的 `f` 是 module 内的相对路径 ⇒ 拼回仓内路径。
		relInRepo := rel
		if tgt != nil && tgt.Kind == impactKindFile {
			if md, _ := impactModuleDir(root, tgt.Rel); md != "" && md != "." {
				relInRepo = filepath.ToSlash(filepath.Join(md, rel))
			}
		}
		tier, _, _ := impactDelClassify(root, relInRepo, byFile[f])
		if strings.HasPrefix(tier, "整件搬出") {
			whole++
		} else {
			sym++
		}
		sha, path, missing := impactDelEvidence(root, relInRepo)
		fmt.Fprintf(w, "    · [%s] %s：%d 条\n", tier, relInRepo, len(byFile[f]))
		if missing != "" {
			fmt.Fprintf(w, "      两件可找回证据：**缺**（%s）⇒ **找不回的必拦**（`R28`）\n", missing)
		} else {
			fmt.Fprintf(w, "      两件可找回证据（缺一不出 · `R31`）：① `sha256`=%s ② %s\n", sha, path)
		}
	}
	fmt.Fprintf(w, "  两档计数（分行报 · 不许合成一个数）：**整件搬出 %d 件** / **符号级删 %d 件**（在册实测 95.6%% : 4.4%%）\n",
		whole, sym)
}

// minInt 小值（取 sha 前 12 位时的边界）。
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
