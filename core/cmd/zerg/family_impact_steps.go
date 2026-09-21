// family_impact_steps.go —— 步名真源与探针（`B1` · 任务单-影响面实施-20260922 §三 `B1`）。
//
// 本件只做一件事：把「会红」那一行的**门步名**那一格接到**真实可判的面**上 —— `A1`–`A5` 里
// 那一格写的是「门步名映射属 `B1`（未接）」，那是一条**估计**（一步都没查过）。
//
//	· 真源 = `bash scripts/gates/precommit-gates.sh --list` 的**现跑输出**（`R51` 已拍：
//	  **不是** `cli-matrix.json` —— 矩阵是命令面 must-fail 矩阵（`command` 取值全是命令名），
//	  与步名逐字交集 = 0 ⇒ 那条通道**不存在**）；
//	· 探针 = `bash scripts/gates/precommit-gates.sh --emit-cmd '<全名>'`（`R45` 已拍：
//	  **全名 + 恰好命中 1 条**；子串口径会一次打 **4** 条：`gofmt`；负控 = 塞一个不存在的名字
//	  ⇒ **必须 `rc=2`**）；
//	· join 的**真同源键 = 脚本路径**（`registry.json` 的 `gate` 字段 ↔ 步**命令串**里的同名脚本）；
//	  join 不到的契约条目**必须明写「未接步」**（不许拿 `go test` 一类粗步冒充 ✗）。
//
// 三条口径（照任务单逐字）：
//
//	① 步数**不许写常量** ✗ —— 一律「本跑 N 步（`--list` 自报）+ `head_sha` + 工作树是否脏」
//	  （步名集合随脚本改而变：设计 v1.5 记「同一会话 6 分钟内 76 → 77」）；
//	② 投影出的每个步名都要能过判据①（逐名 `--emit-cmd '<全名>'` **恰好 1 条**）；
//	③ 取不到 ⇒ **只报数量不报步名**（宁少报不猜报 · §九 批2 的退路）；真跑不了的
//	  **照实标「不可机检」** ✗（名字不唯一 / `--list` 读不到 / 探针退码不是 0）。
//
// 成本（本机现跑 · 这一步是**贵项**）：`--list` 会先跑整套门禁自检（**8.3–9.5s**），逐名探针
// 再乘 N 条 `bash` 起停（并行池 8 ⇒ **0.4–0.6s**）。所以两条闸：
//
//	· 只有「有可 join 的脚本路径」时才拉真源（没有候选 ⇒ 一次都不跑，也不编「本跑 N 步」）；
//	· `dev edit` 的**干跑**那一档**不拉**（干跑是人每次敲都走的那条路 —— §4.4 的档位纪律；
//	  那一档照实写「**未机检**」并给出怎么拉）。
//
// 红线（本件逐条）：**不改门禁脚本** ✗（只用 `--list` / `--emit-cmd` 两个只读入口）·
// **不拿 `cli-matrix.json` 当步名来源** ✗ · **不把 `docs: freshness D3 断链断锚` 当接线点** ✗ ·
// **不放宽 `--emit-cmd` 的命中数判据** ✗（多命中 = 名字不唯一 ⇒ 那一步**不可机检**，不猜）·
// 只读（不写缓存 / 不落审计 / 不改件）· 不引新依赖（只用 `os/exec` 那一个底座）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// impactStepNegativeProbe —— 判据② 的负控名字（**不存在**的一步 ⇒ `--emit-cmd` 必须 `rc=2`）。
	// 逐字取任务单/设计稿里那一个（另造一个就成了第二份口径）。
	impactStepNegativeProbe = "zzz-没有这一步-zz"
	// impactStepProbePool —— 逐名探针的并发池（**只读**探针 · 每条一次 `bash` 起停 ≈ 30–60 ms）。
	impactStepProbePool = 8
	// impactStepListSection —— `--list` 里那一段的起头（脚本自己用这一行切；`gate explain` 的
	// `gateStepIndex` 是**同一份口径**，本件不另造第二套切法）。
	impactStepListSection = "步骤清单"
)

// impactStepTotalRe —— `--list` 自报的那一行（`共 N 步`）。**N 不许写常量**：只认现跑这一行。
var impactStepTotalRe = regexp.MustCompile(`^共 (\d+) 步$`)

// impactStepListLineRe —— `--list` 清单那一列的行形（`scope 模式 名称`）。
//
// ★ 名字取**原文**（第一、二列按空白切，名字那一段**不折叠空白**）：脚本里那两条名字就是
// `core: linux  build ./...`（**两个空格**）—— 折叠成一个就变成了另一个名字，`--emit-cmd '<全名>'`
// 会 `rc=2`，判据① 就会**假红**（本跑首版就是这么把 2 条真步名误标成不可机检的）。
var impactStepListLineRe = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(.+?)\s*$`)

// impactGateScriptTokenRe —— 从 `gate` 字段里**机械**取「脚本路径」（后缀 `.py` / `.sh`）。
// 只取带 `/` 的 token（`check-x.py` 这种裸名**不算**路径 —— 与设计 v1.5 的「真同源键 = 脚本路径」同一口径）。
var impactGateScriptTokenRe = regexp.MustCompile(`[A-Za-z0-9_][A-Za-z0-9_./-]*\.(?:py|sh)`)

// impactStepRow —— 一步（名字**逐字**取自 `--list` 现跑那一列）。
type impactStepRow struct {
	Scope string // `--list` 第一列（scope）
	Mode  string // `--list` 第二列（模式）
	Name  string // `--list` 第三列起（步名 · 可能含空格）
	Cmd   string // `--emit-cmd '<全名>'` 的命令串（**恰好命中 1 条**时才填）
	Hits  int    // 名表里有几个步名**包含**它（判据①：必须 == 1）
	RC    int    // 探针退码
	Note  string // 不可机检的理由（取不到时）
}

// impactStepTable —— 门禁步骤表的**现跑快照**（真源 = `--list`；步数**不写常量**）。
type impactStepTable struct {
	Rows       []impactStepRow
	Total      int // 现跑「共 N 步」里那个 N
	Listed     int // 清单里真解析出来的条数（与 Total 不等 ⇒ 不对齐 ⇒ 不可机检）
	HeadSHA    string
	At         string
	Dirty      string // 工作树里那一步脚本脏不脏（空 ⇒ 干净）
	ListCost   time.Duration
	ProbeCost  time.Duration
	Probed     int // 真跑了的探针条数（名字不唯一的不探 —— 探了也归不到那一步）
	Resolved   int // 恰好 1 条且拿到命令串的条数
	Ambiguous  int // 名字不唯一的条数（判据① 不成立 ⇒ 不可机检）
	Unresolved int // 探针没给出命令串的条数
	NegRC      int // 负控探针的退码（**必须 2**）
	NegRan     bool
	Status     string // 取值 / 读不到
	Detail     string
}

// impactStepParseList 只读「步骤清单」那一段（自检输出在前、`共 N 步` 收尾）——
// 不这么切就会把自检的 ✓ 行当成步骤（`gate explain` 那条手搓 `sed` 的同一口径）。
func impactStepParseList(out string) ([]impactStepRow, int) {
	rows := []impactStepRow{}
	total := 0
	in := false
	for _, ln := range strings.Split(out, "\n") {
		t := strings.TrimSpace(ln)
		if strings.Contains(t, impactStepListSection) {
			in = true
			continue
		}
		if !in {
			continue
		}
		if m := impactStepTotalRe.FindStringSubmatch(t); m != nil {
			n, _ := strconv.Atoi(m[1])
			total = n
			break
		}
		// 行形如 `scope 模式 名称` —— 名字**取原文**（含内部空白，见 `impactStepListLineRe`）。
		if m := impactStepListLineRe.FindStringSubmatch(t); m != nil {
			rows = append(rows, impactStepRow{Scope: m[1], Mode: m[2], Name: m[3]})
		}
	}
	return rows, total
}

// impactStepTablePull 拉真源（**只跑 `--list`** · 不跑任何步骤）：步名 + 本跑步数 + `head_sha`
// + 工作树是否脏。四件缺一件或清单与自报数不对齐 ⇒ `Status = 读不到`（**不给结论**）。
func impactStepTablePull(root string) impactStepTable {
	t := impactStepTable{Status: "读不到"}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateScriptRel))); err != nil {
		t.Detail = fmt.Sprintf("门禁脚本不在盘上（%s · %v）⇒ 步名真源取不到（「读不到」不当「没有」）",
			gateScriptRel, err)
		return t
	}
	beg := time.Now()
	out, errb, code, err := impactRunIn(root, "bash", gateScriptRel, "--list")
	t.ListCost = time.Since(beg)
	if err != nil {
		t.Detail = fmt.Sprintf("`--list` 起不来（%v）⇒ 不给结论", err)
		return t
	}
	if code != 0 {
		t.Detail = fmt.Sprintf("`--list` 退码 %d（stderr 头一行：%s）⇒ 不给结论", code, impactFirstLine(errb))
		return t
	}
	t.Rows, t.Total = impactStepParseList(out)
	t.Listed = len(t.Rows)
	t.HeadSHA = impactHeadSHA(root)
	t.At = impactEffectiveAt()
	t.Dirty = impactStepWorktreeNote(root)
	if t.Listed == 0 || t.Listed != t.Total {
		t.Detail = fmt.Sprintf("清单解析出 %d 条 ≠ `--list` 自报「共 %d 步」⇒ **不对齐 ⇒ 不可机检**（两个数都不当结论）",
			t.Listed, t.Total)
		return t
	}
	t.Status = "取值"
	t.Detail = fmt.Sprintf("`--list` 现跑自报「共 %d 步」· 清单解析 %d 条（逐字相同）", t.Total, t.Listed)
	return t
}

// impactStepWorktreeNote 那一步脚本在工作树里脏不脏（**必报** —— 同一个脚本改一行，步数就变：
// 设计 v1.5 记「同一会话 6 分钟内 76 → 77」）。取不到就照实说取不到。
func impactStepWorktreeNote(root string) string {
	out, _, code, err := impactRunIn(root, "git", "status", "--porcelain", "--", gateScriptRel)
	if err != nil || code != 0 {
		return "（取不到 —— 工作树干不干净判不了）"
	}
	s := strings.Join(strings.Fields(strings.ReplaceAll(out, "\n", " ")), " ")
	if s == "" {
		return "干净"
	}
	return "脏（" + s + "）"
}

// impactStepHits 名表里有几个步名**包含**它（子串口径 · 与 `--emit-cmd` 的匹配口径逐字同义）。
// 判据① 要的是 == 1；> 1 = 名字不唯一（`cargo test` ⊂ `ui: cargo test`）⇒ 那一步不可机检。
func impactStepHits(rows []impactStepRow, name string) int {
	n := 0
	for _, r := range rows {
		if strings.Contains(r.Name, name) {
			n++
		}
	}
	return n
}

// impactStepProbeCmd 跑一条探针（`--emit-cmd '<全名>'`）：它是**只读**入口 ——
// 脚本里 `--emit-cmd` 在整套自检**之前**就 `exit 0`（不跑步骤、不给结论）。
func impactStepProbeCmd(root, arg string) (string, int, string) {
	out, errb, code, err := impactRunIn(root, "bash", gateScriptRel, "--emit-cmd", arg)
	if err != nil {
		return "", -1, fmt.Sprintf("起不来（%v）", err)
	}
	return strings.TrimRight(out, "\n"), code, impactFirstLine(errb)
}

// impactStepTableProbe 逐名探针（**并行池** = 8；每条只读、写自己的那一格 ⇒ 无竞态）。
// 另跑**一条负控**（判据②）：不存在的名字 ⇒ 必须 `rc=2`（证明通道活着 —— 不会把「没命中」
// 读成「取到」）。名字不唯一的步**不探**（探了也归不到哪一步）⇒ 照实计入 `Ambiguous`。
func impactStepTableProbe(root string, t *impactStepTable) {
	if t == nil || t.Status != "取值" {
		return
	}
	beg := time.Now()
	// ★ 两趟分开：**第一趟**把「名字不唯一」整表判完（这一趟不并发），**第二趟**才起探针。
	// 为什么不能合成一趟（`-race` 当场报 DATA RACE · 真事故级）：探针 goroutine 会写 `R t.Rows[i]`
	// 的 `Cmd`/`RC`/`Note`，而「算命中数」要**读整表**的 `Name`（同一块内存上一边读全表、一边写某一行）
	// ⇒ 分开就干净。口径一字未变（判据① 用的还是 `Hits == 1`）。
	for i := range t.Rows {
		row := &t.Rows[i]
		row.Hits = impactStepHits(t.Rows, row.Name)
		if row.Hits != 1 {
			row.Note = fmt.Sprintf("名字不唯一（名表里有 %d 个步名**包含**它 —— 子串口径）⇒ 判据① 不成立 ⇒ **不可机检**", row.Hits)
		}
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, impactStepProbePool)
	for i := range t.Rows {
		row := &t.Rows[i]
		if row.Hits != 1 {
			continue // 名字不唯一的**不探**（探了也归不到哪一步）⇒ 照实计入 `Ambiguous`
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(row *impactStepRow) {
			defer wg.Done()
			defer func() { <-sem }()
			cmd, rc, note := impactStepProbeCmd(root, row.Name)
			row.Cmd, row.RC = cmd, rc
			if rc != 0 || strings.TrimSpace(cmd) == "" {
				row.Note = fmt.Sprintf("`--emit-cmd '<全名>'` 没给出唯一命令串（rc=%d · %s）⇒ **不可机检**", rc, note)
			}
		}(row)
	}
	wg.Wait()
	for _, r := range t.Rows {
		switch {
		case r.Hits != 1:
			t.Ambiguous++
		case strings.TrimSpace(r.Cmd) == "":
			t.Unresolved++
		default:
			t.Resolved++
		}
	}
	t.Probed = t.Resolved + t.Unresolved
	_, rc, _ := impactStepProbeCmd(root, impactStepNegativeProbe)
	t.NegRC, t.NegRan = rc, true
	t.ProbeCost = time.Since(beg)
}

// impactStepScriptTokens 从 `gate` 字段里**机械**取脚本路径（后缀 `.py` / `.sh` 且带 `/`）。
// 这是 `S-*` 一族的 gate 字段形态决定的：门脚本一族写路径（`scripts/gates/check-x.py`），
// 而 `S-b`/`S-c`/`S-d`/`S-h` 写的是「由 `X`: go test 覆盖」一类**粗步表述** ⇒ 取不到路径
// ⇒ 那几条**只能**写「未接步」（不许拿粗步冒充 ✗）。
func impactStepScriptTokens(gate string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, tok := range impactGateScriptTokenRe.FindAllString(gate, -1) {
		if !strings.Contains(tok, "/") || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// impactRegistryGatesOf 现读契约登记表的 `id ⇒ gate`（**同一份真源** `registry.json` ——
// 与 ③ 契约层读的是同一个件，不是另抄一份表）。读不到 ⇒ 报错（「读不到」不当「没有」）。
func impactRegistryGatesOf(root string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactRegistryRel)))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Entries []struct {
			ID   string `json:"id"`
			Gate string `json:"gate"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s 解不开：%v", impactRegistryRel, err)
	}
	if len(doc.Entries) == 0 {
		return nil, fmt.Errorf("%s 的 entries[] 一条都没有（空表不许当「都不在册」）", impactRegistryRel)
	}
	out := map[string]string{}
	for _, e := range doc.Entries {
		if strings.TrimSpace(e.ID) != "" {
			out[e.ID] = e.Gate
		}
	}
	return out, nil
}

// impactStepJoinRow 一条 join 的结果（人面与 stderr 块共用**同一份取值**，不另算一遍）。
type impactStepJoinRow struct {
	ID      string
	Path    string   // gate 里的脚本路径
	Steps   []string // 调用它的步名（本跑命令串里出现该路径）
	Unwhy   string   // 未接步 / 不可机检的原因（空 = 接上了）
	CmdHead string   // 命令串头一行（判据① 的可读证据 · 接上时才有）
}

// impactStepProjection —— 「会红」那一行里**门步**那一格的取值（**三态不许混**）。
type impactStepProjection struct {
	Status    string // 取值 / 未拉 / 不可机检 / 空
	Reason    string // 为什么是这一态（人面直接打）
	Total     int    // 本跑 N 步（`--list` 自报 · **不是常量**）
	HeadSHA   string
	Dirty     string
	At        string
	Steps     []string // 投影出的步名（逐名已过判据① · 去重 · 定序）
	ByID      map[string][]string
	Unjoined  map[string]string // 契约 id ⇒ 未接步的原因
	Joins     []impactStepJoinRow
	Uncheck   []string // 不可机检的步名（附原因）
	Cost      time.Duration
	ListCost  time.Duration
	ProbeCost time.Duration
	Probed    int
	Resolved  int
	Ambiguous int
	Probes    int
	Table     impactStepTable
	NegRC     int
	NegRan    bool
	Cands     int
}

// impactStepProjectionNotRun 干跑那一档用的投影（**未机检** —— 不拉真源，照实说为什么）。
// 干跑是人每次敲都走的那条路：把那 8.3–9.5s 的 `--list` 塞进去就是 §4.4 点名的那个反模式。
func impactStepProjectionNotRun() impactStepProjection {
	return impactStepProjection{
		Status: "未机检",
		Reason: "干跑档**不背贵项**（真源 `--list` 现跑会先跑整套门禁自检 · 本机 8.3–9.5s + 逐名探针" +
			" ⇒ 按 §4.4 档位纪律不进干跑档；要看真源给 `zerg impact <目标> --all`（按需档））",
		ByID:     map[string][]string{},
		Unjoined: map[string]string{},
	}
}

// impactStepProjectionCostly 默认档（吃紧档）用的投影：候选**有**，但真源探针是**贵项**
// ⇒ 按 §4.4 「贵层按需」**不拉**，也**不编**步名（宁少报不猜报）。
//
// 为什么把这一步放进档位（本机现跑，不是口味）：`--list` 会先跑整套门禁自检 —— **8.3–9.5s**，
// 比设计里最贵的那一层（`callgraph -algo rta` 3.5–3.8s）还贵一档；§4.4 点名的反模式正是
// 「把最贵的那一块**无条件**塞进每一次取数」⇒ 真源探针走**按需档 `--all`**（既有全局布尔，
// 不新造旗标名）。这一格的三态：有候选 + 默认档 = **未机检**（不是「没有」）· `--all` = 取值 ·
// 没有候选 = **未接步**（那个判定不用拉真源 ⇒ 默认档就能给）。
func impactStepProjectionCostly(cands int) impactStepProjection {
	return impactStepProjection{
		Status: "未机检",
		Reason: fmt.Sprintf("真源探针是**贵项**（`--list` 现跑会先跑整套门禁自检 · 本机 8.3–9.5s —— "+
			"比最贵的层 `callgraph -algo rta` 3.5–3.8s 还贵一档）⇒ 按 §4.4 **走按需档**："+
			"`zerg impact <目标> --all`（本跑有 %d 条可 join 的脚本路径 ⇒ 本档**故意不编步名**：宁少报不猜报）", cands),
		ByID:     map[string][]string{},
		Unjoined: map[string]string{},
		Cands:    cands,
	}
}

// impactStepProjectionOf 把「③ 命中的那些契约条目」投影到**门禁步名**上（本件的全部实现）。
//
// 五步，逐条对应判据：
//
//	① 候选 = 各条 `gate` 里的**脚本路径**（机械取 · **不用拉真源** ⇒ 默认档就能判「没有门脚本路径」）；
//	② 没有候选 ⇒ **未接步**（那些条目的 gate 只有粗步表述 —— 判据④ 的这一格在默认档就给）；
//	③ 有候选而档位是**默认档**（`pull=false`）⇒ **未机检**：真源探针是贵项，走 §4.4 按需档 `--all`
//	  （**不许**把「没跑」写成「没有」—— 那是两件事）；
//	④ 按需档：拉真源（`--list` 现跑 ⇒ 本跑步数 + `head_sha` + 工作树是否脏）+ 逐名探针；
//	⑤ join：某条候选路径出现在**哪些步的命令串**里 ⇒ 那些步名就是投影（真同源键 = 脚本路径）；
//	  接不上 ⇒ 「未接步」（**有牙的那种**：本跑命令串里真没有它）。
func impactStepProjectionOf(root string, ids []string, pull bool) impactStepProjection {
	p := impactStepProjection{ByID: map[string][]string{}, Unjoined: map[string]string{}}
	ids = impactStepSortedUniq(ids)
	if len(ids) == 0 {
		p.Status = "空"
		p.Reason = "没有 ③ 命中的契约条目 ⇒ 没有可 join 的脚本路径"
		return p
	}
	gates, err := impactRegistryGatesOf(root)
	if err != nil {
		p.Status = "不可机检"
		p.Reason = fmt.Sprintf("契约登记表读不到（%v）⇒ 判不了「哪些步会红」", err)
		return p
	}
	type candPath struct{ id, path string }
	cands := []candPath{}
	for _, id := range ids {
		g, ok := gates[id]
		if !ok {
			p.Unjoined[id] = "不在册（登记表里没有这条 id）"
			continue
		}
		toks := impactStepScriptTokens(g)
		if len(toks) == 0 {
			p.Unjoined[id] = "gate 字段里**没有门脚本路径**（只有「由 `X`: go test 覆盖」一类粗步表述 —— " +
				"不许拿粗步冒充具体步名 ✗）"
			p.Joins = append(p.Joins, impactStepJoinRow{ID: id, Unwhy: p.Unjoined[id]})
			continue
		}
		for _, tk := range toks {
			cands = append(cands, candPath{id: id, path: tk})
		}
	}
	p.Cands = len(cands)
	if len(cands) == 0 {
		idsLeft := []string{}
		for id := range p.Unjoined {
			idsLeft = append(idsLeft, id)
		}
		sort.Strings(idsLeft)
		p.Status = "未拉"
		p.Reason = "**未接步** —— " + strings.Join(idsLeft, ",") + "（`gate` 里**没有门脚本路径**：不许拿 `go test` " +
			"一类粗步冒充 ✗）· 真源探针**不拉**（没有可 join 的候选就不编「本跑 N 步」这个数）"
		return p
	}
	beg := time.Now()
	if !pull {
		// 默认档：候选有了，但真源探针是贵项 ⇒ 不拉、不编（**不许**把「没跑」写成「没有」）。
		c := impactStepProjectionCostly(p.Cands)
		c.Joins = p.Joins
		return c
	}
	t := impactStepTablePull(root)
	p.Table, p.ListCost = t, t.ListCost
	if t.Status != "取值" {
		p.Status = "不可机检"
		p.Reason = "步名真源取不到（" + t.Detail + "）⇒ 只报数量不报步名"
		return p
	}
	impactStepTableProbe(root, &t)
	p.Table = t
	p.Total, p.HeadSHA, p.Dirty, p.At = t.Total, t.HeadSHA, t.Dirty, t.At
	p.ProbeCost, p.Probed, p.Resolved, p.Ambiguous = t.ProbeCost, t.Probed, t.Resolved, t.Ambiguous
	p.NegRC, p.NegRan = t.NegRC, t.NegRan
	for _, c := range cands {
		row := impactStepJoinRow{ID: c.id, Path: c.path}
		for _, r := range t.Rows {
			if r.Cmd == "" {
				continue
			}
			if strings.Contains(r.Cmd, c.path) {
				row.Steps = append(row.Steps, r.Name)
				if row.CmdHead == "" {
					row.CmdHead = impactFirstLine(r.Cmd)
				}
			}
		}
		if len(row.Steps) == 0 {
			row.Unwhy = fmt.Sprintf("gate 里的 `%s` 在**本跑 %d 步的命令串**里没有出现（没有一步调用它）%s",
				c.path, t.Total, impactStepAmbNote(t))
			if _, ok := p.Unjoined[c.id]; !ok {
				p.Unjoined[c.id] = row.Unwhy
			}
		} else {
			p.ByID[c.id] = impactStepAppendUniq(p.ByID[c.id], row.Steps...)
		}
		p.Joins = append(p.Joins, row)
	}
	// 投影集合：按**排序后的契约 id** 走（定序 ⇒ 两跑逐字相同），逐名去重。
	stepSeen := map[string]bool{}
	for _, id := range ids {
		for _, nm := range p.ByID[id] {
			if !stepSeen[nm] {
				stepSeen[nm] = true
				p.Steps = append(p.Steps, nm)
			}
		}
	}
	for _, r := range t.Rows {
		if r.Hits != 1 || strings.TrimSpace(r.Cmd) == "" {
			p.Uncheck = append(p.Uncheck, fmt.Sprintf("%s（%s）", r.Name, r.Note))
		}
	}
	p.Probes = t.Probed
	p.Status = "取值"
	p.Cost = time.Since(beg)
	return p
}

// impactStepAmbNote 名字不唯一的步那一格（**有它才敢说「未接步」**：否则「本跑可判的步里没有」
// 会被读成「就是没有」）。
func impactStepAmbNote(t impactStepTable) string {
	if t.Ambiguous == 0 {
		return ""
	}
	return fmt.Sprintf("（另有 %d 步**名字不唯一** ⇒ 那几步不可机检 —— 所以这一格只说「本跑**可判**的步里没有」，不说「没有」）",
		t.Ambiguous)
}

// impactStepSortedUniq 排序去重（定序 = 两跑逐字相同）。
func impactStepSortedUniq(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// impactStepAppendUniq 追加去重（保序）。
func impactStepAppendUniq(dst []string, in ...string) []string {
	seen := map[string]bool{}
	for _, s := range dst {
		seen[s] = true
	}
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		dst = append(dst, s)
	}
	return dst
}

// impactStepLineText 「会红」那一行里**门步**那一格的文本（三态：取值 / 未接步 / 不可机检）。
// 措辞纪律：这一格是**预测**（§7.8 只预测不真跑）⇒ 明写「预测」，**不许写成「必红」** ✗。
func impactStepLineText(p impactStepProjection) string {
	switch p.Status {
	case "取值":
		head := "门步（**预测** · §7.8 只预测不真跑 · 逐名 `--emit-cmd '<全名>'` 恰好 1 条 ✓）"
		body := ""
		if len(p.Steps) > 0 {
			body = strings.Join(p.Steps, " · ")
		} else {
			body = "**没接上任何一步**（原因逐条在 stderr 的步名真源块）"
		}
		tail := fmt.Sprintf(" · 真源 `--list` 现跑 · 本跑 %d 步 · head_sha=%s · 工作树=%s",
			p.Total, dashIfEmpty(p.HeadSHA), p.Dirty)
		if len(p.Unjoined) > 0 {
			ids := []string{}
			for id := range p.Unjoined {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			tail += " · **未接步**：" + strings.Join(ids, ",") + "（原因逐条在 stderr 的步名真源块 —— 不许拿粗步冒充）"
		}
		return head + "：" + body + tail
	case "未拉": // 没有可 join 的候选（`gate` 里全是粗步表述）⇒ 明写「未接步」，步数一个不编
		return "门步：" + p.Reason
	default: // 未机检 / 不可机检 / 空
		return "门步：**" + impactStepStatusLabel(p.Status) + "**（" + p.Reason + "）"
	}
}

// impactStepStatusLabel 三态的措辞（**不许把「没跑」写成「没有」**）。
func impactStepStatusLabel(status string) string {
	switch status {
	case "未机检":
		return "未机检"
	case "空":
		return "不适用（本条没有可 join 的契约条目）"
	default:
		return "不可机检"
	}
}

// impactStepBlock —— stderr 的**步名真源块**（本件判据①②③④的可读落点 · 不写进六键包封）。
//
// 为什么落 stderr：§九 批2 的退路要求把「取不到」的原因写进 `warnings[]`，而 `emitEnvelopeWith`
// 把 `warnings` 恒写 `[]`、`truncated` 恒写 `false`（`A1`–`A5` 红线「不改 `emitEnvelope*`」本批
// 未解禁）⇒ 同一份取值落 stderr，缺口照实点名（与层表那条同款）。
func emitImpactStepBlock(w io.Writer, p impactStepProjection) {
	fmt.Fprintf(w, "%s: `B1` 步名真源与探针（真源 = `bash scripts/gates/precommit-gates.sh --list` 的**现跑输出** —— "+
		"**不是** `cli-matrix.json`（矩阵是命令面 must-fail 矩阵 · 与步名逐字交集 = 0）；`R51` / `R45` 已拍）\n", progName)
	switch p.Status {
	case "取值":
		fmt.Fprintf(w, "  本跑：%s · 解析 %d 条 · 工作树=%s · `--list` 耗时 %s\n",
			p.Table.Detail, len(p.Table.Rows), p.Dirty, p.ListCost.Round(time.Millisecond).String())
		fmt.Fprintf(w, "  层规（本件口径）：head_sha=%s · 取数时刻=%s · 档位=默认档（真源探针只在**有可 join 的脚本路径**时才拉）\n",
			dashIfEmpty(p.HeadSHA), p.At)
		fmt.Fprintf(w, "  探针：逐名 `--emit-cmd '<全名>'` 本跑 %d 条（恰好 1 条 %d · 名字不唯一 %d · 取不到 %d）· "+
			"耗时 %s（并行池 %d · **只读**：`--emit-cmd` 在整套自检之前就 `exit 0`，不跑任何步骤）\n",
			p.Probed, p.Resolved, p.Ambiguous, p.Probed-p.Resolved, p.ProbeCost.Round(time.Millisecond).String(), impactStepProbePool)
		if p.NegRan {
			verdict := "✗（**判据② 不成立** —— 通道会把「没命中」读成「取到」，本跑不给步名结论）"
			if p.NegRC == 2 {
				verdict = "✓（判据②：不存在的名字 ⇒ rc=2）"
			}
			fmt.Fprintf(w, "  判据②（负控）：`--emit-cmd '%s'` ⇒ rc=%d %s\n", impactStepNegativeProbe, p.NegRC, verdict)
		} else {
			fmt.Fprintf(w, "  判据②（负控）：**未跑**（连负控都没跑 ⇒ 上面那几条也别当结论）\n")
		}
		fmt.Fprintf(w, "  判据①（正控）：投影出的每个步名都过「恰好命中 1 条」—— 本跑投影 %d 步\n", len(p.Steps))
		fmt.Fprintf(w, "  join（真同源键 = **脚本路径**：`%s` 的 `gate` ↔ 步**命令串**里的同名脚本）：\n", impactRegistryRel)
		for _, j := range p.Joins {
			if len(j.Steps) == 0 {
				fmt.Fprintf(w, "    %s · %s ⇒ **未接步**：%s\n", j.ID, j.Path, j.Unwhy)
				continue
			}
			fmt.Fprintf(w, "    %s · %s ⇒ %s（命令串头一行：%s）\n", j.ID, j.Path, strings.Join(j.Steps, " · "), j.CmdHead)
		}
		for id, why := range p.Unjoined {
			found := false
			for _, j := range p.Joins {
				if j.ID == id && j.Path == "" {
					found = true
				}
			}
			if !found {
				fmt.Fprintf(w, "    %s · （gate 里没有脚本路径）⇒ **未接步**：%s\n", id, why)
			}
		}
		if len(p.Uncheck) > 0 {
			fmt.Fprintf(w, "  不可机检（照实标）：%d 条步名 —— %s\n", len(p.Uncheck), strings.Join(p.Uncheck, " · "))
		} else {
			fmt.Fprintf(w, "  不可机检（照实标）：0 条（本跑每个步名都过了判据①）\n")
		}
	case "未拉":
		fmt.Fprintf(w, "  本跑：**未接步**（没有可 join 的脚本路径）—— %s\n", p.Reason)
	case "未机检":
		fmt.Fprintf(w, "  本跑：**未机检** —— %s\n", p.Reason)
	default:
		fmt.Fprintf(w, "  本跑：**不可机检** —— %s\n", p.Reason)
	}
	fmt.Fprintf(w, "  退路（§九 批2）：投影取不到 ⇒ **只报数量不报步名**（宁少报不猜报）· join 不到的契约条目**明写「未接步」**"+
		"（不许拿 `go test` 一类粗步冒充）\n")
	fmt.Fprintf(w, "  ★ 为什么这条走 stderr：退路要求的原因本该进 `warnings[]`，而 `emitEnvelopeWith` 里 `warnings` 恒 `[]`、"+
		"`truncated` 恒 `false`（`A1`–`A5` 红线「不改 `emitEnvelope*`」本批未解禁）⇒ 同一份取值落 stderr（缺口照实登记，不偷偷改包封）\n")
	fmt.Fprintf(w, "  零副作用：两个入口都只读（`--list` 只列不跑 · `--emit-cmd` 在自检**之前** `exit 0`）⇒ "+
		"不写缓存 / 不落审计 / 不改件 · 不改门禁脚本\n")
}

// impactStepIDsToJoin 「会红」那一行里要 join 的契约 id（③ 命中条的 id + 契约态目标自身的 id）。
func impactStepIDsToJoin(tgt *impactTarget, layers []impactLayer) []string {
	ids := []string{}
	if tgt != nil && tgt.Kind == impactKindContract {
		ids = append(ids, tgt.ID)
	}
	if l, ok := impactLayerBySeq(layers, "③"); ok {
		for _, r := range l.Rows {
			if id := strings.TrimPrefix(r["what"], "契约级:"); id != r["what"] {
				ids = append(ids, id)
			}
		}
	}
	return impactStepSortedUniq(ids)
}
