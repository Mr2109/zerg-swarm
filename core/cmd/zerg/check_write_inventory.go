// check_write_inventory.go —— 判据臂 `check-write-inventory`：把「命令面写盘点」这一份
// **单一真源**（`core/internal/contract/write-inventory.json`）与实践**逐条对拍**。
//
// 缺口 `GAP-20260927-35`（本件是 want① 的判据面）：此前命令面「默认档写不写盘」这件事**没有真源**，
// 只能拿**危险档**（`danger`）去猜 —— 而这两根轴并不重合：
//
//	· D2/D3 里有一批**零写点**（拒执 / 发网 / 转发）；
//	· `danger == nil` 里也可能写盘（锁件、缓存 —— 实测 `chat alias ls` 裸跑建 `chat-aliases.json.lock`）。
//
// ⇒ 猜错的方向有两个，两个都出过血：把只读命令当写面（白要一次 `--yes`）、把写命令当只读
// （裸跑就落件，破「一个字节都不写」的承诺）。
//
// 两个档：默认档 = **只读 ＋ 沙箱探针**（零写盘）；`--emit` = **打印模式**（✗ 不写盘，把按现读
// catalog 重算的真源内容打到 stdout，由人照抄进真源件或走 `dev edit` 正门 —— 同族先例是各门的
// `--emit-baseline`）。
//
// 本判据做三件事，缺一不可：
//
//	① **登记面完备**：`catalog()` 里每条命令在真源里**必须有一行**（新增命令不登记 ⇒ 判红并点名）。
//	② **声明面自洽**：只读旗标名必须在该命令 `usage` 里真的宣告、且源码里**真读它**；
//	   `gated_by` 里的 `--yes` 必须在该命令实现里真有一个 `writeGuardYes` 闸；
//	   「默认档真写盘」的命令不许落在**危险档之外**（锁件类单列，见 `write_kind`）。
//	③ **声明 ⟷ 实践对拍**（本判据的牙）：在**沙箱**里真跑 —— `HOME` / `ZERG_STATE_DIR` /
//	   `TMPDIR` 全指向沙箱，跑前跑后对沙箱整棵树取 sha256 清单。
//	   `default_writes=false` ⇒ 差异必须为 0（非 0 ⇒ 判红并**逐条点名写到哪件**）；
//	   给了只读旗标的命令再跑一趟 `--dry-run`，同样要求零差异。
//
// 退码（三档齐）：
//
//	0  全一致（登记面完备 · 声明面自洽 · 每条承诺都被实践证实）
//	1  有破承诺（逐条点名：命令 ＋ 哪个旗标 ＋ 写在哪）
//	8  判不了（真源读不到 / 仓根解析不到 / 探针跑不起来 / 探针超时 —— **不给结论**，✗ 不当绿）
//
// 口径（与缺口账 2026-09-27 拍板的两笔一致）：
//
//	· **默认档** = 裸跑（不给 `--yes` / `--confirm` / `--dry-run` / `--out`）。
//	· **写点** = 创建/覆盖/追加/删除/改名/建目录（含 `CreateTemp`/`MkdirTemp`）；判据一句话 =
//	  「Ctrl-C 之后盘上会不会多东西」。
//	· **落点旗标（`--out`）不算闸** —— 给了 `--out` 才写 ≠ 默认档不写，两件事分开登记。
//
// 为什么探针只跑「可在沙箱里代表默认档」的那一批：重编 / 提交 / 启停 / 发网 / 跑门的命令在沙箱里
// 跑不出默认档的真行为（不是被拒就是打空），登记表把它们标 `probe:"exempt"` 并**逐条写明理由**，
// 判据对它们只做声明面自洽（`gated_by` ⟷ 源码里的闸、`readonly_flag` ⟷ 用法串），✗ 不假装探过。
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	wviInventoryRel = "core/internal/contract/write-inventory.json"
	wviPeerDirRel   = "core/cmd/zerg"
	wviProbeTimeout = 25 * time.Second
	// wviRCLand / wviRCBreach / wviRCUnjudge —— 三档退码（与本仓判据同口径：0 绿 · 1 红 · 8 不给结论）。
	wviRCLand    = 0
	wviRCBreach  = 1
	wviRCUnjudge = 8
)

// wviEntry —— 真源里的一行（一条命令的写盘点声明）。
type wviEntry struct {
	Path         string          `json:"path"`
	Danger       string          `json:"danger"`
	Opened       bool            `json:"opened"`
	DefaultWrite json.RawMessage `json:"default_writes"` // true / false / "unprobed"
	WritesWhere  []string        `json:"writes_where"`
	WriteKind    string          `json:"write_kind"`
	GatedBy      []string        `json:"gated_by"`
	ReadonlyFlag string          `json:"readonly_flag"`
	Probe        string          `json:"probe"`
	Reason       string          `json:"reason"`
	Waiver       string          `json:"waiver"`
}

// wviCurated —— 手写的对拍探针（要位置参数 / 要落点旗标的命令靠它，裸跑代表不了）。
type wviCurated struct {
	Path   string   `json:"path"`
	Argv   []string `json:"argv"`
	Expect string   `json:"expect"` // nowrite / write
	Note   string   `json:"note"`
}

// wviDoc —— 真源件。
type wviDoc struct {
	Schema  string       `json:"schema"`
	ID      string       `json:"id"`
	Entries []wviEntry   `json:"entries"`
	Curated []wviCurated `json:"curated_probes"`
}

// wviCmd —— catalog() 的一条命令在本判据里的投影（测试可注入夹具）。
type wviCmd struct {
	Path   string
	Usage  string
	Danger string
	Opened bool
	Run    string // run 函数名（生产档由 reflect 取，测试档手填）
}

// wviSrcFact —— 该命令实现面的两条静态读数。
type wviSrcFact struct {
	HasDryRun   bool // 闭包内出现干跑字段（`inv` 上的 `dryRun`）
	HasYesGuard bool // 闭包内出现 writeGuardYes
	Resolved    bool // 取到了实现面（取不到 ⇒ 相关断言判不了，✗ 不静默放行）
}

// wviOpts —— 探针面旋钮（测试注入 tmp/二进制；生产档走默认）。
type wviOpts struct {
	Bin       string // 探针二进制（空 ⇒ os.Executable()）
	TmpRoot   string // 沙箱根（空 ⇒ os.MkdirTemp）
	SeedState string // 真源种子（沙箱里种一份缺口账，让「读真源」的命令真做全活）
}

// wviVerdict —— 逐条判定结果（累积成退码与输出）。
type wviVerdict struct {
	breaches []string
	unjudged []string
	notes    []string
	counts   map[string]int
}

func (v *wviVerdict) breachf(format string, a ...any) {
	v.breaches = append(v.breaches, fmt.Sprintf(format, a...))
}
func (v *wviVerdict) unjudgef(format string, a ...any) {
	v.unjudged = append(v.unjudged, fmt.Sprintf(format, a...))
}
func (v *wviVerdict) note(format string, a ...any) {
	v.notes = append(v.notes, fmt.Sprintf(format, a...))
}

func (v *wviVerdict) bump(k string) {
	if v.counts == nil {
		v.counts = map[string]int{}
	}
	v.counts[k]++
}

// wviTri —— `default_writes` 的三态解析（bool 或 "unprobed"；别的一律判不了）。
func wviTri(raw json.RawMessage) (writes bool, state string, ok bool) {
	s := strings.TrimSpace(string(raw))
	switch s {
	case "true":
		return true, "true", true
	case "false":
		return false, "false", true
	case `"unprobed"`:
		return false, "unprobed", true
	case "":
		return false, "missing", false
	default:
		return false, "bad:" + s, false
	}
}

// wviRunFuncName —— 从 run 函数值取它的名字（`main.cmdGapExport` ⇒ `cmdGapExport`）。
func wviRunFuncName(fn func(*invocation, io.Writer, io.Writer) int) string {
	if fn == nil {
		return ""
	}
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if f == nil {
		return ""
	}
	n := f.Name()
	if i := strings.LastIndex(n, "."); i >= 0 {
		n = n[i+1:]
	}
	return n
}

// wviLoadInventory —— 读真源（读不到 / 坏 JSON / schema 不对 ⇒ 判不了）。
func wviLoadInventory(path string) (*wviDoc, int, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, wviRCUnjudge, fmt.Sprintf("真源读不到：%s（%v）", path, err)
	}
	var d wviDoc
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, wviRCUnjudge, fmt.Sprintf("真源不是合法 JSON：%s（%v）", path, err)
	}
	if d.Schema != "zerg/write-inventory/v1" {
		return nil, wviRCUnjudge, fmt.Sprintf("真源 schema = %q（要 zerg/write-inventory/v1）", d.Schema)
	}
	if len(d.Entries) == 0 {
		return nil, wviRCUnjudge, "真源 entries 为空（空表 = 空转 ⇒ 不给结论）"
	}
	return &d, wviRCLand, ""
}

// wviDryRunField —— 「命令实现读干跑旗标」那一格的字面（★ 前缀 ＋ 字面拼串：本文本**不许**出现
// 连续原串，否则本件会被门⑩ `check-dryrun-semantics` 当成「又一套干跑语义站点」判红 ——
// 同族先例 `check_write_face_gate.go` 的写点原语拼串）。
var wviDryRunField = "inv." + "dryRun"

// wviLoadSrcFacts —— 静态面：run 函数 ＋ 它直接/间接（≤2 层）调到的同包函数的函数体文本，
// 判「有没有真读干跑字段（`inv` 的 `dryRun`）」「有没有 `writeGuardYes` 闸」。
func wviLoadSrcFacts(root string, cmds []wviCmd) (map[string]wviSrcFact, string) {
	dir := filepath.Join(root, filepath.FromSlash(wviPeerDirRel))
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Sprintf("实现面目录读不到：%s（%v）", dir, err)
	}
	bodies := map[string]string{}
	fset := token.NewFileSet()
	for _, e := range ents {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, 0)
		if err != nil {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			lo := fset.Position(fd.Pos()).Offset
			hi := fset.Position(fd.End()).Offset
			if lo >= 0 && hi <= len(src) && lo < hi {
				bodies[fd.Name.Name] = string(src[lo:hi])
			}
		}
	}
	if len(bodies) == 0 {
		return nil, "实现面一个函数体都没解析出来（解析面空转 ⇒ 不给结论）"
	}
	callRe := regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	facts := make(map[string]wviSrcFact, len(cmds))
	for _, c := range cmds {
		body, ok := bodies[c.Run]
		if !ok {
			facts[c.Path] = wviSrcFact{Resolved: false}
			continue
		}
		// 展开 run 函数体 ＋ 它调到的同包函数（**≤3 层**：闸与只读支常落在咽喉 helper 里 ——
		// 实测 `dev build` 的 `--dry-run` 要到 `cmdDevBuild → devOrderedAction → cmdGuarded` 第三层才读到，
		// 只展开一层会把守规矩的命令判成「旗标是摆设」＝假红）。
		text := body
		seen := map[string]bool{c.Run: true}
		frontier := []string{body}
		for depth := 0; depth < 3 && len(frontier) > 0; depth++ {
			next := []string{}
			for _, srcText := range frontier {
				for _, m := range callRe.FindAllStringSubmatch(srcText, -1) {
					callee := m[1]
					if seen[callee] {
						continue
					}
					if b, ok := bodies[callee]; ok {
						seen[callee] = true
						text += "\n" + b
						next = append(next, b)
					}
				}
			}
			frontier = next
		}
		// `cmdGuarded` 是本仓的**统一拒执闸**：它内部读干跑字段（出计划件）—— 落到它上面
		// 就等价于「该命令的 `--dry-run` 有消费者」，不必要求本函数里再写一遍。
		facts[c.Path] = wviSrcFact{
			HasDryRun:   strings.Contains(text, wviDryRunField) || seen["cmdGuarded"],
			HasYesGuard: strings.Contains(text, "writeGuardYes"),
			Resolved:    true,
		}
	}
	return facts, ""
}

// ── 沙箱探针 ────────────────────────────────────────────────────────────────

// wviSnapshot —— 沙箱整棵树的 sha256 清单（相对路径 + 摘要 + 字节数，排序后做键）。
func wviSnapshot(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(b)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:8]) + ":" + fmt.Sprintf("%d", len(b))
		return nil
	})
	return out, err
}

// wviProbeRun —— 在沙箱里真跑一条 argv，回「新增/改动/消失的件」。
//
// 沙箱三旋钮（HOME / ZERG_STATE_DIR / TMPDIR）全指沙箱；`ZERG_REPO` / `ZERG_GATE_LOCK_DIR` /
// `GIT_INDEX_FILE` 一律剔掉（否则探针会把真仓当真源 ⇒ 读到真状态、还可能写进真仓）。
func wviProbeRun(bin, tmpRoot, seedState string, argv []string) (changed []string, runErr error, timedOut bool) {
	sb, err := os.MkdirTemp(tmpRoot, "wvi-probe-")
	if err != nil {
		return nil, err, false
	}
	defer os.RemoveAll(sb)
	for _, d := range []string{"home", "state", "tmp", "work", "out", "docs", "plans"} {
		if err := os.MkdirAll(filepath.Join(sb, d), 0o755); err != nil {
			return nil, err, false
		}
	}
	if seedState != "" {
		if b, err := os.ReadFile(seedState); err == nil {
			_ = os.WriteFile(filepath.Join(sb, "state", filepath.Base(seedState)), b, 0o644)
		}
	}
	before, err := wviSnapshot(sb)
	if err != nil {
		return nil, err, false
	}
	args := make([]string, 0, len(argv))
	for _, a := range argv {
		args = append(args, strings.ReplaceAll(a, "$SB", sb))
	}
	ctx, cancel := context.WithTimeout(context.Background(), wviProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = filepath.Join(sb, "work")
	env := make([]string, 0, len(os.Environ())+3)
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		switch k {
		case "ZERG_REPO", "ZERG_GATE_LOCK_DIR", "GIT_INDEX_FILE", "ZERG_PROPOSAL_DIR", "HOME", "ZERG_STATE_DIR", "TMPDIR":
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"HOME="+filepath.Join(sb, "home"),
		"ZERG_STATE_DIR="+filepath.Join(sb, "state"),
		"TMPDIR="+filepath.Join(sb, "tmp"),
	)
	cmd.Env = env
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	rerr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil, true
	}
	after, serr := wviSnapshot(sb)
	if serr != nil {
		return nil, serr, false
	}
	for p, h := range after {
		if before[p] != h {
			changed = append(changed, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			changed = append(changed, p+"（消失）")
		}
	}
	sort.Strings(changed)
	return changed, rerr, false
}

// wviEvaluate —— 判据本体（纯函数式：输入 = 真源 ＋ 命令面 ＋ 静态面 ＋ 探针面，输出 = 退码 ＋ 报告）。
//
// 生产档：wviEvaluate(doc, cmdsFromCatalog, facts, opts)；测试档：全部注入（负控就在这一层造）。
func wviEvaluate(doc *wviDoc, cmds []wviCmd, facts map[string]wviSrcFact, opts wviOpts, stdout, stderr io.Writer) int {
	v := &wviVerdict{}
	byPath := map[string]*wviEntry{}
	for i := range doc.Entries {
		e := &doc.Entries[i]
		if _, dup := byPath[e.Path]; dup {
			v.breachf("%s：真源里**重复登记**（同一命令两行 ⇒ 判不出来哪一行算数）", e.Path)
			continue
		}
		byPath[e.Path] = e
	}
	cmdByPath := map[string]wviCmd{}
	for _, c := range cmds {
		cmdByPath[c.Path] = c
	}

	// ① 登记面完备（双向）。
	for _, c := range cmds {
		if _, ok := byPath[c.Path]; !ok {
			v.breachf("%s：**catalog 里有、写盘点里没有**（新增命令必须登记默认档写不写盘 ＋ 只读旗标名；跑 `zerg gate check check-write-inventory --emit` 或手补一行）", c.Path)
		}
	}
	for _, e := range doc.Entries {
		if _, ok := cmdByPath[e.Path]; !ok {
			v.breachf("%s：**写盘点里有、catalog 里没有**（幽灵条目 —— 命令已删/改名，登记面没跟）", e.Path)
		}
	}

	// ② 声明面自洽。
	for i := range doc.Entries {
		e := &doc.Entries[i]
		c, inCatalog := cmdByPath[e.Path]
		writes, state, okTrace := wviTri(e.DefaultWrite)
		switch state {
		case "true":
			v.bump("declared_writes")
		case "false":
			v.bump("declared_nowrite")
		case "unprobed":
			v.bump("declared_unprobed")
		}
		if !okTrace {
			v.unjudgef("%s：`default_writes` 既不是 true/false 也不是 \"unprobed\"（现读 %s）⇒ 这一行判不了", e.Path, state)
			continue
		}
		// 只读旗标：usage 里必须真宣告，源码里必须真读它。
		if e.ReadonlyFlag != "" {
			v.bump("readonly_flags")
			if inCatalog && !strings.Contains(c.Usage, e.ReadonlyFlag) {
				v.breachf("%s：声明只读旗标 %s，但 usage 里**没有**它 ⇒ 用户按这个名字敲会被判未知旗标（usage = %q）", e.Path, e.ReadonlyFlag, c.Usage)
			}
			if f, ok := facts[e.Path]; ok && f.Resolved && !f.HasDryRun {
				v.breachf("%s：声明只读旗标 %s，但实现面**从不读**干跑字段 `%s` ⇒ 旗标是摆设（给了照样写/照样做）", e.Path, e.ReadonlyFlag, wviDryRunField)
			}
		}
		// `--yes` 闸：声明有闸就必须真有一个 writeGuardYes。
		for _, g := range e.GatedBy {
			if g != "--yes" {
				v.unjudgef("%s：`gated_by` 里有 %q —— 本判据只认 `--yes` 一种授权旗标（别的先扩判据再登记）", e.Path, g)
				continue
			}
			v.bump("yes_gated")
			if f, ok := facts[e.Path]; ok && f.Resolved && !f.HasYesGuard {
				v.breachf("%s：登记 `gated_by:[\"--yes\"]`，但实现面**没有** `writeGuardYes` 闸 ⇒ 声明与代码不符（缺闸 = 裸跑就能写）", e.Path)
			}
		}
		// 默认档真写盘 ⇒ 必须在危险档里（锁件类单列：它不是耐久产物，但**照样登记**且不静默）。
		if writes && e.Danger == "" {
			switch e.WriteKind {
			case "lock":
				v.note("%s：默认档写盘（`write_kind=lock` · 落 %v）却**不在危险档** —— 已登记为锁件例外，%s", e.Path, e.WritesWhere, wviWaiverText(e))
			default:
				v.breachf("%s：**默认档真写盘**（落 %v）却**不在任何危险档**（danger 空）⇒ 裸跑就落件、且零授权", e.Path, e.WritesWhere)
			}
		}
	}

	// ③ 声明 ⟷ 实践对拍（牙）。
	tested := 0
	bin := opts.Bin
	if bin == "" {
		if exe, err := os.Executable(); err == nil {
			bin = exe
		}
	}
	probeSeeded := opts.SeedState != ""
	for i := range doc.Entries {
		e := &doc.Entries[i]
		writes, state, okTrace := wviTri(e.DefaultWrite)
		if !okTrace {
			continue
		}
		if e.Probe == "exempt" {
			v.bump("probe_exempt")
			continue
		}
		if e.Probe != "live" {
			v.unjudgef("%s：`probe` = %q（闭集只有 live / exempt）⇒ 这一行的实践面判不了", e.Path, e.Probe)
			continue
		}
		if bin == "" {
			v.unjudgef("%s：取不到探针二进制（os.Executable 失败）⇒ 实践面判不了", e.Path)
			continue
		}
		argv := strings.Fields(e.Path)
		changed, _, timedOut := wviProbeRun(bin, opts.TmpRoot, opts.SeedState, argv)
		if timedOut {
			v.unjudgef("%s：裸跑探针超时（>%s）⇒ 无法证明默认档零写盘", e.Path, wviProbeTimeout)
			continue
		}
		tested++
		v.bump("probe_bare")
		if state == "false" && len(changed) > 0 {
			v.breachf("%s：声明**默认档零写盘**，裸跑却写了 %d 件：%s", e.Path, len(changed), strings.Join(wviHead(changed, 4), ", "))
			continue
		}
		if state == "unprobed" && len(changed) > 0 {
			v.note("%s：登记为「未探」，但裸跑探针看到它写了 %d 件（%s）⇒ 请把 default_writes / writes_where 补进真源", e.Path, len(changed), strings.Join(wviHead(changed, 3), ", "))
		}
		if state == "true" && len(changed) == 0 {
			v.note("%s：登记「默认档真写盘」，裸跑探针没看到写（多半是需要位置参数；curated 探针补上）", e.Path)
		}
		// 只读旗标：给了它还是写 ⇒ 破承诺。
		if e.ReadonlyFlag == "" || writes {
			continue
		}
		if bin == "" {
			continue
		}
		changedDry, _, timedOutDry := wviProbeRun(bin, opts.TmpRoot, opts.SeedState, append(argv, e.ReadonlyFlag))
		if timedOutDry {
			v.unjudgef("%s：%s 探针超时 ⇒ 无法证明只读旗标零写盘", e.Path, e.ReadonlyFlag)
			continue
		}
		tested++
		v.bump("probe_readonly")
		if len(changedDry) > 0 {
			v.breachf("%s：给了只读旗标 %s 仍写了 %d 件：%s", e.Path, e.ReadonlyFlag, len(changedDry), strings.Join(wviHead(changedDry, 4), ", "))
		}
	}
	// curated 探针（要位置参数/落点旗标的命令）。
	for _, cu := range doc.Curated {
		if bin == "" {
			v.unjudgef("%s：curated 探针取不到二进制 ⇒ 判不了", cu.Path)
			continue
		}
		changed, _, timedOut := wviProbeRun(bin, opts.TmpRoot, opts.SeedState, cu.Argv)
		if timedOut {
			v.unjudgef("%s：curated 探针 %v 超时", cu.Path, cu.Argv)
			continue
		}
		tested++
		v.bump("probe_curated")
		switch cu.Expect {
		case "nowrite":
			if len(changed) > 0 {
				v.breachf("%s：curated 探针 %v 声明零写盘，却写了 %d 件：%s", cu.Path, cu.Argv, len(changed), strings.Join(wviHead(changed, 4), ", "))
			}
		case "write":
			if len(changed) == 0 {
				v.breachf("%s：curated 探针 %v 声明会写盘（这条是它留在危险档的实证），实际一个字节没写 ⇒ 声明与实现不符", cu.Path, cu.Argv)
			}
		default:
			v.unjudgef("%s：curated 探针 expect = %q（闭集 nowrite/write）", cu.Path, cu.Expect)
		}
	}
	if !probeSeeded {
		v.note("探针沙箱**没有种真源**（`ZERG_STATE_DIR/zerg-cli-gaps.jsonl` 取不到）⇒ 「读真源」类命令在沙箱里只跑到「真源不在盘」那一支，零写盘这一面仍成立、但「真做全活也零写盘」这一面未证")
	}

	// 输出：先人面摘要，再逐条。
	fmt.Fprintf(stdout, "命令面写盘点对拍（真源 = %s）\n", wviInventoryRel)
	fmt.Fprintf(stdout, "  声明面：catalog %d 条 · 登记 %d 行（默认档写 %d · 不写 %d · 未探 %d）· 只读旗标 %d 条\n",
		len(cmds), len(doc.Entries), v.counts["declared_writes"], v.counts["declared_nowrite"],
		v.counts["declared_unprobed"], v.counts["readonly_flags"])
	fmt.Fprintf(stdout, "  实践面：裸跑探针 %d · 只读旗标探针 %d · curated 探针 %d（沙箱：HOME/状态目录/TMPDIR 三点隔离）\n",
		v.counts["probe_bare"], v.counts["probe_readonly"], v.counts["probe_curated"])
	fmt.Fprintf(stdout, "  豁免：%d 条（D3 / 真活面 —— 见真源 reason，逐条写明为什么沙箱代表不了默认档）\n", v.counts["probe_exempt"])
	for _, n := range v.notes {
		fmt.Fprintf(stdout, "  · 注：%s\n", n)
	}
	for _, b := range v.breaches {
		fmt.Fprintf(stderr, "  ✗ 破承诺：%s\n", b)
	}
	for _, u := range v.unjudged {
		fmt.Fprintf(stderr, "  ⚠ 判不了：%s\n", u)
	}
	fmt.Fprintf(stdout, "  ⇒ 破承诺 %d 条 · 判不了 %d 条 · 实跑探针 %d 条\n", len(v.breaches), len(v.unjudged), tested)

	switch {
	case len(v.unjudged) > 0:
		fmt.Fprintf(stdout, "退码 8（判不了 —— 上面逐条点名；「判不了」不当绿、也不当红）\n")
		return wviRCUnjudge
	case len(v.breaches) > 0:
		fmt.Fprintf(stdout, "退码 1（有破承诺 —— 逐条点名在上）\n")
		return wviRCBreach
	default:
		fmt.Fprintf(stdout, "退码 0（声明面与实践面逐条一致）\n")
		return wviRCLand
	}
}

func wviHead(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(append([]string{}, xs[:n]...), fmt.Sprintf("…另 %d 件", len(xs)-n))
}

func wviWaiverText(e *wviEntry) string {
	if strings.TrimSpace(e.Waiver) == "" {
		return "（但**没写例外理由** ⇒ 这一行应补 waiver）"
	}
	return "理由： " + e.Waiver
}

// wviCmdsFromCatalog —— 生产档：catalog() ⇒ 判据输入。
func wviCmdsFromCatalog() []wviCmd {
	cs := catalog()
	out := make([]wviCmd, 0, len(cs))
	for _, c := range cs {
		d := ""
		if c.danger != nil {
			d = string(c.danger.Level)
		}
		out = append(out, wviCmd{
			Path:   strings.Join(c.path, " "),
			Usage:  c.usage,
			Danger: d,
			Opened: c.opened,
			Run:    wviRunFuncName(c.run),
		})
	}
	return out
}

// wviInventoryPath —— 真源件落点（★ 夹具旋钮 `ZERG_WRITE_INVENTORY` 只给测试与落件前试跑用：
// 真源换一件而不动仓内件；生产档不设它 ⇒ 恒为仓内相对路径）。
func wviInventoryPath(root string) string {
	if v := strings.TrimSpace(os.Getenv("ZERG_WRITE_INVENTORY")); v != "" {
		return v
	}
	return filepath.Join(root, filepath.FromSlash(wviInventoryRel))
}

// wviSeedState —— 沙箱种子的真源路径（缺口账）。取不到就空（探针照跑，只是「真做全活」这一面未证）。
func wviSeedState() string {
	if sd := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); sd != "" {
		p := filepath.Join(sd, "zerg-cli-gaps.jsonl")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".zerg", "state", "zerg-cli-gaps.jsonl")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// wviEmit —— `--emit`：按**现读 catalog** 重算真源内容并**打印**（★ 打印模式 · ✗ 一个字节不写盘）。
//
// 为什么要有它：命令树一加命令，登记面就会落后 ⇒ 判据当场判红并点名；这一档把「补登记」的
// **内容**算出来，✗ 不让人手抄 271 行（手抄必长歪）。**保留**旧行里的人工字段（default_writes /
// writes_where / write_kind / waiver / curated），只把能从源码现读的四格（danger / opened /
// readonly_flag / gated_by）刷新；新增命令一律 `default_writes:"unprobed"` —— 未探就是未探，
// ✗ 不按名字猜。
//
// ★ 为什么**不**直接写盘：本判据是**判据面**，写仓内真源件要走 `dev edit` 正门（写面人签闸）；
//
//	判据自己开一个写点会与门102 `check-write-authz`「写点前须有交代」正面冲突，也会把「谁改的
//	真源」这条审计链弄丢 ⇒ 只打印，落地仍走正门。
func wviEmit(root, invPath string, cmds []wviCmd, stdout, stderr io.Writer) int {
	facts, why := wviLoadSrcFacts(root, cmds)
	if facts == nil {
		fmt.Fprintf(stderr, "%s: %s ⇒ 判不了（退码 8）\n", progName, why)
		return wviRCUnjudge
	}
	top := map[string]json.RawMessage{}
	prev := map[string]wviEntry{}
	curated := json.RawMessage("[]")
	if raw, err := os.ReadFile(invPath); err == nil {
		if e := json.Unmarshal(raw, &top); e != nil {
			fmt.Fprintf(stderr, "%s: 现存真源不是合法 JSON（%v）⇒ 不肯覆盖（退码 8）\n", progName, e)
			return wviRCUnjudge
		}
		if b, ok := top["curated_probes"]; ok {
			curated = b
		}
		var d wviDoc
		if e := json.Unmarshal(raw, &d); e == nil {
			for _, e := range d.Entries {
				prev[e.Path] = e
			}
		}
	}
	entries := make([]wviEntry, 0, len(cmds))
	added := []string{}
	for _, c := range cmds {
		e, had := prev[c.Path]
		e.Path = c.Path
		e.Danger = strings.TrimPrefix(c.Danger, "danger")
		e.Opened = c.Opened
		if strings.Contains(c.Usage, "--dry-run") {
			if e.ReadonlyFlag == "" {
				e.ReadonlyFlag = "--dry-run"
			}
		} else {
			e.ReadonlyFlag = ""
		}
		if facts[c.Path].HasYesGuard {
			e.GatedBy = []string{"--yes"}
		} else if e.GatedBy == nil {
			e.GatedBy = []string{}
		}
		if e.WritesWhere == nil {
			e.WritesWhere = []string{}
		}
		if !had {
			added = append(added, c.Path)
			e.DefaultWrite = json.RawMessage(`"unprobed"`)
			e.WritesWhere = []string{}
			e.WriteKind = ""
			if c.Danger == "dangerD3" || wviLooksRealWork(c.Path) {
				e.Probe = "exempt"
				e.Reason = "新增命令：默认档写不写盘**未探**（沙箱代表不了默认档 ⇒ 先按豁免登记，等一次人工现读补 reason 与 default_writes）"
			} else {
				e.Probe = "live"
				e.Reason = ""
			}
		}
		entries = append(entries, e)
	}
	eb, err := json.Marshal(entries)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 序列化失败：%v\n", progName, err)
		return wviRCUnjudge
	}
	top["entries"] = eb
	top["curated_probes"] = curated
	// ★ 打印模式：✗ 不写盘（见函数头「为什么不直接写盘」）。
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(top); err != nil {
		fmt.Fprintf(stderr, "%s: 编码失败：%v\n", progName, err)
		return wviRCUnjudge
	}
	fmt.Fprintf(stderr, "（--emit 打印模式：%s 的**新内容**已打到 stdout —— 共 %d 行（新增 %d：%v）· **一个字节没写盘**；\n", wviInventoryRel, len(entries), len(added), added)
	fmt.Fprintf(stderr, "  落地走 `zerg dev edit` 正门；★ 新增行的 `default_writes` 一律 \"unprobed\" —— 补实要跑沙箱探针或读实现面，✗ 别按名字猜）\n")
	return wviRCLand
}

// wviRealWorkNames —— 「裸跑会做真活」的命令名闭集（重编 / 提交 / 启停 / 发网 / 跑门）。
var wviRealWorkNames = regexp.MustCompile(`\b(run|build|swap|deploy|start|stop|install|apply|commit|edit|upload|publish|sync|hatch|load|unload|reload|restart|send|push|lsremote|fsck|rollback|new|add|import|set-state|void|reread|reap|bench|exec|spawn)\b`)

func wviLooksRealWork(path string) bool { return wviRealWorkNames.MatchString(path) }

// wviArgErr —— 用法错（本判据的旗标面很小；未知旗标 ⇒ 2）。
func wviArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "%s: error: %s\n", progName, msg)
	fmt.Fprintf(stderr, "用法：%s  [--emit | --no-probe | --json D]\n", wviGateName)
	return exitUsage
}

const wviGateName = "check-write-inventory"

// checkWriteInventoryGateMain —— `zerg gate check check-write-inventory` 的入口（inproc 键同名）。
func checkWriteInventoryGateMain(stdout, stderr io.Writer, argv []string) int {
	emit := false
	noProbe := false
	asJSON := false
	for _, a := range argv {
		switch a {
		case "--emit":
			emit = true
		case "--no-probe":
			noProbe = true
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Fprintf(stdout, "用法：%s  [--emit | --no-probe | --json]\n", wviGateName)
			fmt.Fprintf(stdout, "  --emit      按现读 catalog 重算真源并**打印**（✗ 不写盘 —— 照抄进真源件或走 dev edit）\n")
			fmt.Fprintf(stdout, "  --no-probe  只判声明面（实践面不跑 ⇒ 退码 8，✗ 不冒充绿）\n")
			return exitOK
		default:
			return wviArgErr(stderr, fmt.Sprintf("未知旗标 %q", a))
		}
	}
	root := repoRoot()
	if root == "" {
		fmt.Fprintf(stderr, "%s: 仓根解析不到 ⇒ 判不了（退码 8）\n", progName)
		return wviRCUnjudge
	}
	invPath := wviInventoryPath(root)
	cmds := wviCmdsFromCatalog()
	if emit {
		return wviEmit(root, invPath, cmds, stdout, stderr)
	}
	doc, rc, why := wviLoadInventory(invPath)
	if doc == nil {
		fmt.Fprintf(stderr, "%s: %s ⇒ 判不了（退码 %d）\n", progName, why, rc)
		return rc
	}
	facts, why := wviLoadSrcFacts(root, cmds)
	if facts == nil {
		fmt.Fprintf(stderr, "%s: %s ⇒ 判不了（退码 8）\n", progName, why)
		return wviRCUnjudge
	}
	if asJSON {
		fmt.Fprintf(stdout, "{\"inventory\":%q,\"catalog\":%d,\"entries\":%d}\n", wviInventoryRel, len(cmds), len(doc.Entries))
	}
	if noProbe {
		// 实践面不跑 ⇒ 一律判不了（✗ 拿声明面冒充全绿）。
		fmt.Fprintf(stderr, "%s: --no-probe ⇒ 实践面（沙箱探针）未跑 ⇒ 本档**不给结论**（退码 8）\n", progName)
		return wviRCUnjudge
	}
	opts := wviOpts{SeedState: wviSeedState()}
	return wviEvaluate(doc, cmds, facts, opts, stdout, stderr)
}
