// adopt_retire_test.go —— T-56「收编与退役」归属真源的**判据机检**（§6.3 S7 · §5.6 汇总 · §7.1 P9）。
//
// 真源 = core/internal/contract/adopt-retire.json（同目录 go:embed 之外 · 与 unresolved-entries.json 同款台账）。
// 本件把 T-56 的四处判据落成断言：
//
//	① 算术自洽：273 = 41 + 145 + 4 + 83（稿 §5.6 行 388）与落回后的 4 类列和**逐格对得上**，
//	   且五行分面的行内和 = 行首的 objects（= 设计稿五行分面 76/13/46/125/13）；
//	② 10 条决议逐条有落点（`status` 只能取闭集），**没落的不许写成已落**；
//	③ ④ 的每一组都带 `why` + `next` + `承接` —— 「不许长期停在待定」落成「必须有人/有落点」；
//	④ ③ 退役台账与**盘上现状**一致（已删的必须不在 · 待删的必须还在）—— 台账不腐；
//	⑤ 决议⑤ 的判据：`scripts/253/` 不在盘上，两处排除域名单里零 `253`（只改一处 = 两个门打架）；
//	⑥ 决议② 的判据：`itask` 的 **7 条命令路径**与 **9 条端点**逐条现跑命中（命令树声明了自己的端点）。
//
// ★ 判定口一律**显式传入**（仓根 / 真源字节 / 命令树正文）—— 为的是让负控能喂合成件进来，不碰真仓。
// ★ 边界（照实记）：本机检比的是「登记的条文字面 ↔ 现跑件里有没有」；端点集合的**封闭性**归命令面
// 自己的契约矩阵（scripts/gates/check-cli-contract.py），不在本件重做。
package main_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ── 真源形状（只取机检要用的格） ───────────────────────────────────────────────

type arFace struct {
	Face     string `json:"face"`
	Objects  int    `json:"objects"`
	Adopt    int    `json:"adopt"`
	Internal int    `json:"internal"`
	Retire   int    `json:"retire"`
	Pending  int    `json:"pending"`
}

type arDoc struct {
	Schema string `json:"schema"`
	Was    struct {
		Tally map[string]int `json:"tally"`
		Faces map[string]int `json:"faces"`
	} `json:"was"`
	Faces []arFace `json:"faces"`
	Tally struct {
		Objects, Adopt, Internal, Retire, Pending int
		SevenGroups                               map[string]int `json:"seven_groups"`
		PendingRowVsSlot                          struct {
			ByRows            int `json:"by_rows"`
			BySevenGroupSlots int `json:"by_seven_group_slots"`
			Diff              int `json:"diff"`
		} `json:"pending_row_vs_slot"`
	} `json:"tally"`
	Decisions []struct {
		ID       int    `json:"id"`
		Verdict  string `json:"verdict"`
		Landing  string `json:"landing"`
		Status   string `json:"status"`
		Evidence string `json:"evidence"`
	} `json:"decisions"`
	Itask struct {
		Endpoints9    []string `json:"endpoints_9"`
		Commands7     []string `json:"commands_7"`
		Read4         []string `json:"read_4"`
		WriteGuarded3 []string `json:"write_guarded_3"`
		WriteNotOpen  []string `json:"write_registered_not_open"`
		Mapping       []struct {
			Endpoint, Command, State string
		} `json:"mapping"`
	} `json:"itask_criteria"`
	RetireLedger []struct {
		Object, State, Evidence string
	} `json:"retire_ledger"`
	PendingItems []struct {
		Object string `json:"object"`
		Count  int    `json:"count"`
		Why    string `json:"why"`
		Next   string `json:"next"`
		Follow string `json:"承接"`
	} `json:"pending_items"`
}

// 决议落点只许取这个闭集：已落 / 归属登记 / 待拍。
var arStatusClosed = map[string]bool{"已落": true, "归属登记": true, "待拍": true}

// 决议 ⑤ 必须删干净的三件 + 门禁两处名单（「同一提交」那条纪律的现跑判据）。
var arDeletedPaths = []string{
	"scripts/253/branch_system.sh",
	"scripts/253/docker_worker.go",
	"scripts/253/docker_worker_test.go",
}

// ★ 退役容忍（2026-10-07 退役批 · 判据⑤ 硬耦合解）：名单件（py 半）退役 ⇒ 不再读它；其「排除域
//
//	名单」由**同判据的 Go 半**（goHalf · 同包）承载 ⇒ Go 半在盘即视为一件合法退场，本格不过问。
//	两半皆不在盘 ⇒ fail-closed（台账撒谎：名单件凭空消失）。py 在盘时旧路逐字不变。
var arExclusionFiles = []struct{ path, linePrefix, list, goHalf string }{
	{"scripts/gates/check-wired-scripts.py", "EXCL_SUBDIRS", "253", "core/cmd/zerg/check_wired_scripts.go"},
	{"scripts/gates/check-gate-coverage.py", "NOSUFFIX_EXCL", "253", "core/cmd/zerg/check_gate_coverage.go"},
}

// ── 判定口（纯函数 · 负控喂合成件） ────────────────────────────────────────────

// judgeARArithmetic —— 判据①：算术自洽（稿的四类列和 + 落回后的四类列和 + 七组）。
func judgeARArithmetic(d arDoc) []error {
	var errs []error
	wasTally := map[string]int{"adopt": 41, "internal": 145, "retire": 4, "pending": 83, "total": 273}
	for k, want := range wasTally {
		if got := d.Was.Tally[k]; got != want {
			errs = append(errs, fmt.Errorf("判据① 破：稿 §5.6 的 %s 应是 %d，真源写的是 %d", k, want, got))
		}
	}
	sum := func(f func(arFace) int) int {
		n := 0
		for _, x := range d.Faces {
			n += f(x)
		}
		return n
	}
	if len(d.Faces) != 5 {
		errs = append(errs, fmt.Errorf("判据① 破：分面应是 5 行（HTTP/入口/门禁/脚本/UI），真源有 %d 行", len(d.Faces)))
	}
	for _, x := range d.Faces {
		if x.Objects != x.Adopt+x.Internal+x.Retire+x.Pending {
			errs = append(errs, fmt.Errorf("判据① 破：面「%s」行内和不对：%d ≠ %d+%d+%d+%d",
				x.Face, x.Objects, x.Adopt, x.Internal, x.Retire, x.Pending))
		}
	}
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"objects", sum(func(x arFace) int { return x.Objects }), d.Tally.Objects},
		{"①收编", sum(func(x arFace) int { return x.Adopt }), d.Tally.Adopt},
		{"②内部", sum(func(x arFace) int { return x.Internal }), d.Tally.Internal},
		{"③退役", sum(func(x arFace) int { return x.Retire }), d.Tally.Retire},
		{"④待定", sum(func(x arFace) int { return x.Pending }), d.Tally.Pending},
	} {
		if c.got != c.want {
			errs = append(errs, fmt.Errorf("判据① 破：列和 %s = %d，合计行写的是 %d", c.name, c.got, c.want))
		}
	}
	if d.Tally.Objects != 273 {
		errs = append(errs, fmt.Errorf("判据① 破：对象总数应是 273，真源写的是 %d", d.Tally.Objects))
	}
	if d.Tally.Adopt+d.Tally.Internal+d.Tally.Retire+d.Tally.Pending != d.Tally.Objects {
		errs = append(errs, errors.New("判据① 破：四类列和 ≠ 对象总数（273 = 41+145+4+83 的落回版对不上）"))
	}
	if got := d.Was.Faces["HTTP 路径"]; got != 76 {
		errs = append(errs, fmt.Errorf("判据① 破：稿的面分布 HTTP 应是 76，真源写 %d", got))
	}
	g := 0
	for k, v := range d.Tally.SevenGroups {
		if k == "sum" {
			continue
		}
		g += v
	}
	if g != 83 || d.Tally.SevenGroups["sum"] != 83 {
		errs = append(errs, fmt.Errorf("判据① 破：稿的七组应和到 83，现算 %d / 声明 %d", g, d.Tally.SevenGroups["sum"]))
	}
	// ④ 的「逐行读 vs 七组槽读」那条口径差必须**可见**：差 1 条，且逐行读数 = 合计行的 ④。
	ps := d.Tally.PendingRowVsSlot
	if ps.ByRows != d.Tally.Pending {
		errs = append(errs, fmt.Errorf("判据① 破：逐行读 ④ = %d，合计行的 ④ = %d（两个数必须同源）", ps.ByRows, d.Tally.Pending))
	}
	if ps.ByRows-ps.BySevenGroupSlots != ps.Diff || ps.Diff != 1 {
		errs = append(errs, fmt.Errorf("判据① 破：界面那 1 条口径差要显式登记（逐行 %d − 槽 %d ≠ %d）",
			ps.ByRows, ps.BySevenGroupSlots, ps.Diff))
	}
	return errs
}

// judgeARDecisions —— 判据②：10 条决议逐条有落点（闭集内）+ 有证据。
func judgeARDecisions(d arDoc) []error {
	var errs []error
	if len(d.Decisions) != 10 {
		errs = append(errs, fmt.Errorf("判据② 破：决议应是 10 条，真源有 %d 条", len(d.Decisions)))
	}
	for i, x := range d.Decisions {
		if x.ID != i+1 {
			errs = append(errs, fmt.Errorf("判据② 破：第 %d 条的 id 是 %d（决议要 1–10 逐条在册）", i+1, x.ID))
		}
		if !arStatusClosed[x.Status] {
			errs = append(errs, fmt.Errorf("判据② 破：决议 %d 的 status = %q 不在闭集 {已落|归属登记|待拍}", x.ID, x.Status))
		}
		for _, f := range []struct{ name, v string }{{"verdict", x.Verdict}, {"landing", x.Landing}, {"evidence", x.Evidence}} {
			if strings.TrimSpace(f.v) == "" {
				errs = append(errs, fmt.Errorf("判据② 破：决议 %d 的 %s 是空的（落点/证据不许留白）", x.ID, f.name))
			}
		}
	}
	return errs
}

// judgeARPending —— 判据③：④ 的每一组都有人/有落点（不许长期停在待定）。
func judgeARPending(d arDoc) []error {
	var errs []error
	if len(d.PendingItems) == 0 {
		errs = append(errs, errors.New("判据③ 破：④ 有 37 条却一组承接都没有（空转 = 假清零）"))
	}
	n := 0
	for i, x := range d.PendingItems {
		if x.Count <= 0 {
			errs = append(errs, fmt.Errorf("判据③ 破：第 %d 组的 count = %d", i+1, x.Count))
		}
		n += x.Count
		for _, f := range []struct{ name, v string }{{"object", x.Object}, {"why", x.Why}, {"next", x.Next}, {"承接", x.Follow}} {
			if strings.TrimSpace(f.v) == "" {
				errs = append(errs, fmt.Errorf("判据③ 破：第 %d 组（%s）的 %s 是空的 —— 待定必须带理由与承接项",
					i+1, x.Object, f.name))
			}
		}
	}
	if n != d.Tally.Pending {
		errs = append(errs, fmt.Errorf("判据③ 破：承接组的条数之和 %d ≠ ④ 的 %d（漏组或多组）", n, d.Tally.Pending))
	}
	return errs
}

// judgeARRetireLedger —— 判据④：③ 台账与盘上现状一致（台账不腐）。
//
// ★ 2026-10-07 随动（退役试点 #1 · `check-build-selfclean.py`）—— 两处去掉硬钉常量 / 纳入正门那一档：
//
//	① 条数**现读真源**（✗ 不硬钉 3）：退场正门 `zerg script retire` 每退一件就往 `retire_ledger`
//	   追加一条（`family_script_retire.go:595`）⇒ 台账随退场**正常增长**；硬钉「== 3」会把每一次
//	   正当退场误判成红。改为：台账**非空** ＋ 已知删除面（`arDeletedPaths` 三件）逐件在册。
//	② state 闭集**以代码为准**补一档：正门写进 ledger 的 state 是 `scriptRetireStateWord`
//	   ＝「已登记退场（<日期>）」（`family_script_retire.go:58/590`），与历史两档「已删」「待删」并列为合法。
func judgeARRetireLedger(root string, d arDoc) []error {
	var errs []error
	// ① 条数现读真源（✗ 不硬钉常量）：台账非空 ＋ 已知删除面逐件在册。
	if len(d.RetireLedger) == 0 {
		errs = append(errs, errors.New("判据④ 破：③ 台账为空（`scripts/253/` 三件已删，台账不许空）"))
	}
	inLedger := map[string]bool{}
	for _, e := range d.RetireLedger {
		inLedger[strings.TrimSpace(e.Object)] = true
	}
	for _, p := range arDeletedPaths {
		if !inLedger[p] {
			errs = append(errs, fmt.Errorf("判据④ 破：已知删除面 %s 不在台账里（删了就得有档）", p))
		}
	}
	for _, e := range d.RetireLedger {
		if strings.TrimSpace(e.Evidence) == "" {
			errs = append(errs, fmt.Errorf("判据④ 破：%s 没有证据列", e.Object))
		}
		_, err := os.Stat(filepath.Join(root, e.Object))
		switch {
		case strings.Contains(e.State, "已删"):
			if err == nil {
				errs = append(errs, fmt.Errorf("判据④ 破：台账写「已删」但盘上还在：%s", e.Object))
			}
		case strings.Contains(e.State, "待删"):
			if err != nil {
				errs = append(errs, fmt.Errorf("判据④ 破：台账写「待删」但盘上已经不在：%s（删了就要改台账）", e.Object))
			}
		case strings.Contains(e.State, "已登记退场"):
			// ② 正门那一档：`zerg script retire` 只做**登记面**退场（决-01 · ✗ 不代管 `git rm`）⇒
			//   真删交 git 历史＋收口批 ⇒ 登记之后件可能已删、也可能尚未删。这一档**不**以「在盘/不在盘」
			//   断言（两种都是正门语义内的合法态）；只认它是正门会写进 ledger 的合法取值。
		default:
			errs = append(errs, fmt.Errorf("判据④ 破：%s 的 state = %q 不在闭集 {已删|待删|已登记退场}", e.Object, e.State))
		}
	}
	return errs
}

// judgeARExclusions —— 判据⑤：`scripts/253/` 三件不在盘上，两处排除域名单里零 `253`。
func judgeARExclusions(root string) []error {
	var errs []error
	for _, p := range arDeletedPaths {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			errs = append(errs, fmt.Errorf("判据⑤ 破：应删的件还在：%s", p))
		}
	}
	if _, err := os.Stat(filepath.Join(root, "scripts", "253")); err == nil {
		errs = append(errs, errors.New("判据⑤ 破：scripts/253/ 目录还在"))
	}
	for _, f := range arExclusionFiles {
		b, err := os.ReadFile(filepath.Join(root, f.path))
		if err != nil {
			// ★ 退役容忍（2026-10-07 退役批）：名单件（py 半）退役（不在盘）∧ 其 Go 半在盘
			//   ⇒ 名单已随件入 Go 半（或不再是判据对象）⇒ 本格不过问（✗ 不判「读不到」）。
			//   两半皆不在盘 ⇒ 照旧 fail-closed（名单件凭空消失 = 台账撒谎）。
			if os.IsNotExist(err) {
				if _, gerr := os.Stat(filepath.Join(root, f.goHalf)); gerr == nil {
					continue
				}
			}
			errs = append(errs, fmt.Errorf("判据⑤ 破：名单件读不到：%s（%v）", f.path, err))
			continue
		}
		found := false
		for _, line := range strings.Split(string(b), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), f.linePrefix) {
				continue
			}
			found = true
			if strings.Contains(line, `"`+f.list+`"`) {
				errs = append(errs, fmt.Errorf("判据⑤ 破：%s 的名单里还留着 %q ⇒ 幽灵特判（目录已删）", f.path, f.list))
			}
		}
		if !found {
			errs = append(errs, fmt.Errorf("判据⑤ 破：%s 里找不到 %s 那一行（名单形状变了）", f.path, f.linePrefix))
		}
	}
	return errs
}

// judgeARItask —— 判据⑥：`itask` 的 **7 条命令路径**与 **9 条端点**逐条对上（且未开放的写面真未开放）。
func judgeARItask(root string, d arDoc, mainGo, familyItask string) []error {
	var errs []error
	if len(d.Itask.Commands7) != 7 || len(d.Itask.Endpoints9) != 9 {
		errs = append(errs, fmt.Errorf("判据⑥ 破：命令应 7 条、端点应 9 条（真源 %d / %d）",
			len(d.Itask.Commands7), len(d.Itask.Endpoints9)))
	}
	cmds := map[string]bool{}
	for _, c := range d.Itask.Commands7 {
		cmds[c] = true
	}
	// ① 9 条端点逐条有映射，且映射到的命令在 7 条闭集里。
	if len(d.Itask.Mapping) != 9 {
		errs = append(errs, fmt.Errorf("判据⑥ 破：9 条端点应有 9 条映射，真源有 %d 条", len(d.Itask.Mapping)))
	}
	mapped := map[string]bool{}
	for i, m := range d.Itask.Mapping {
		if !cmds[m.Command] {
			errs = append(errs, fmt.Errorf("判据⑥ 破：第 %d 条映射把 %s 挂到不存在的命令 %q 上", i+1, m.Endpoint, m.Command))
		}
		if strings.TrimSpace(m.State) == "" {
			errs = append(errs, fmt.Errorf("判据⑥ 破：端点 %s 的开放状态留白", m.Endpoint))
		}
		mapped[m.Endpoint] = true
	}
	for _, ep := range d.Itask.Endpoints9 {
		if !mapped[ep] {
			errs = append(errs, fmt.Errorf("判据⑥ 破：端点 %s 没有命令映射（收编面漏一条）", ep))
		}
	}
	// ② 7 条命令路径逐条在命令树里。
	for _, v := range d.Itask.Commands7 {
		pat := regexp.MustCompile(`\[\]string\{"itask",\s*"` + regexp.QuoteMeta(v) + `"\}`)
		if !pat.MatchString(mainGo) {
			errs = append(errs, fmt.Errorf("判据⑥ 破：命令树里没有 `zerg itask %s` 的注册行", v))
		}
	}
	// ③ 只读 4 条：实现件里有端点字面 + 命令树自己声明了 `endpoint:`（两处都要，缺一处就是挂了个名字）。
	readEP := map[string]string{
		"ls":       "GET /api/internal-tasks",
		"state":    "GET /api/internal-tasks/state",
		"mode":     "GET /api/internal-tasks/modes",
		"interval": "GET /api/internal-tasks/intervals",
	}
	for _, v := range d.Itask.Read4 {
		ep, ok := readEP[v]
		if !ok {
			errs = append(errs, fmt.Errorf("判据⑥ 破：只读面 %q 不在闭集（ls/state/mode/interval）", v))
			continue
		}
		if !strings.Contains(familyItask, strings.TrimPrefix(ep, "GET ")) {
			errs = append(errs, fmt.Errorf("判据⑥ 破：family_itask.go 里找不到端点 %s", ep))
		}
		if !strings.Contains(mainGo, `endpoint: "`+ep+`"`) {
			errs = append(errs, fmt.Errorf("判据⑥ 破：`zerg itask %s` 没把 %s 声明成自己的端点", v, ep))
		}
	}
	// ④ 危险 3 条：危险档点名了自己的 POST 端点 + 注册块走 `cmdGuarded`。
	guardedEP := map[string]string{
		"start": "POST /api/internal-tasks/start",
		"stop":  "POST /api/internal-tasks/stop",
		"run":   "POST /api/internal-tasks/{id}/run",
	}
	for _, v := range d.Itask.WriteGuarded3 {
		frag, ok := guardedEP[v]
		if !ok {
			errs = append(errs, fmt.Errorf("判据⑥ 破：写面 %q 不在闭集（start/stop/run）", v))
			continue
		}
		if !strings.Contains(mainGo, frag) {
			errs = append(errs, fmt.Errorf("判据⑥ 破：写面 `itask %s` 的危险档没点名端点 %s", v, frag))
		}
		anchor := `[]string{"itask", "` + v + `"}`
		i := strings.Index(mainGo, anchor)
		if i < 0 {
			errs = append(errs, fmt.Errorf("判据⑥ 破：命令树里找不到 %s", anchor))
			continue
		}
		end := i + 1200
		if end > len(mainGo) {
			end = len(mainGo)
		}
		if !strings.Contains(mainGo[i:end], "Guarded") {
			// ★ 2026-10-05：`itask stop` 已开放（写**持久化**用户意图 `stopped=true`）⇒ 走自家门
			//   `cmdItaskStopGuarded`；`start`/`run` 仍走通用守门 `cmdGuarded`。判据口径 = 「危险档
			//   必须走**一个确认档门**（`cmdGuarded` 或自家 `*Guarded`）」，✗ 不再是「必须逐字 cmdGuarded」。
			errs = append(errs, fmt.Errorf("判据⑥ 破：写面 `itask %s` 没走确认档门（`*Guarded` · 危险档失守）", v))
		}
	}
	// ⑤ 写面分两档（2026-10-08 · P0 #232 第三面 · ★ **加强**，✗ 不是删断言）：
	//   · 未开放的写面（真源 `write_registered_not_open` 现读 = ["mode"]）：命令树里**不许**出现 `itask mode set`；
	//   · **已开放**的写面（`interval` 的 POST）：命令树里**必须**出现 `itask interval set` 子命令 —— 由「负向
	//     （不许出现）」**加强**成「正向（必须出现）＋ 走确认档门」，四态真跑落在独立测试
	//     `TestAdoptRetireItaskIntervalSetFourStates`（裸跑 rc=2 / --dry-run rc=0 / --yes 真写 / 坏值 rc=2）。
	for _, v := range d.Itask.WriteNotOpen {
		pat := regexp.MustCompile(`\[\]string\{"itask",\s*"` + regexp.QuoteMeta(v) + `",\s*"set"\}`)
		if pat.MatchString(mainGo) {
			errs = append(errs, fmt.Errorf("判据⑥ 破：真源写 `itask %s` 写面未开放，命令树里却有 `%s set` 子命令", v, v))
		}
	}
	// ⑤′ 已开放的写面（interval set）**正控**：命令树里必须真有这条 + 走自家确认档门 + 危险档点名自己的 POST 端点。
	if !regexp.MustCompile(`\[\]string\{"itask",\s*"interval",\s*"set"\}`).MatchString(mainGo) {
		errs = append(errs, fmt.Errorf("判据⑥ 破：`itask interval` 的 POST 写面已开放，命令树里却没有 `itask interval set` 子命令（正控）"))
	}
	if !strings.Contains(mainGo, "POST /api/internal-tasks/{id}/interval") {
		errs = append(errs, fmt.Errorf("判据⑥ 破：已开放的写面 `itask interval set` 的危险档没点名端点 POST /api/internal-tasks/{id}/interval"))
	}
	if !strings.Contains(mainGo, "cmdItaskIntervalSetGuarded") {
		errs = append(errs, fmt.Errorf("判据⑥ 破：`itask interval set` 没走自家确认档门 cmdItaskIntervalSetGuarded（危险档失守）"))
	}
	if !strings.Contains(familyItask, "func cmdItaskIntervalSetGuarded") {
		errs = append(errs, fmt.Errorf("判据⑥ 破：family_itask.go 里没有 cmdItaskIntervalSetGuarded 的实现"))
	}
	return errs
}

// ── 装载 ─────────────────────────────────────────────────────────────────────

func loadAR(t *testing.T) (string, arDoc, []byte) {
	t.Helper()
	root := repoRootFromCLI(t)
	b, err := os.ReadFile(filepath.Join(root, "core", "internal", "contract", "adopt-retire.json"))
	if err != nil {
		t.Fatalf("归属真源读不到（%v）—— T-56 的落点必须先立起来", err)
	}
	var d arDoc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("归属真源解不动：%v", err)
	}
	return root, d, b
}

func readRepo(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("读不到 %s：%v", rel, err)
	}
	return string(b)
}

// ── 六条判据（真仓现跑） ─────────────────────────────────────────────────────

func TestAdoptRetireArithmetic(t *testing.T) {
	_, d, _ := loadAR(t)
	if errs := judgeARArithmetic(d); len(errs) > 0 {
		t.Errorf("判据① 破（%d 处）：\n  %s", len(errs), joinErrs(errs))
	} else {
		t.Logf("判据① ✓ 稿 %d = %d+%d+%d+%d ⇒ 落回后 %d = %d（①）+%d（②）+%d（③）+%d（④）",
			d.Was.Tally["total"], d.Was.Tally["adopt"], d.Was.Tally["internal"], d.Was.Tally["retire"], d.Was.Tally["pending"],
			d.Tally.Objects, d.Tally.Adopt, d.Tally.Internal, d.Tally.Retire, d.Tally.Pending)
	}
}

func TestAdoptRetireDecisionsAllLanded(t *testing.T) {
	_, d, _ := loadAR(t)
	if errs := judgeARDecisions(d); len(errs) > 0 {
		t.Errorf("判据② 破（%d 处）：\n  %s", len(errs), joinErrs(errs))
	} else {
		t.Logf("判据② ✓ 10 条决议逐条有落点（status 全在闭集 {已落|归属登记|待拍}）")
	}
}

func TestAdoptRetirePendingHasOwner(t *testing.T) {
	_, d, _ := loadAR(t)
	if errs := judgeARPending(d); len(errs) > 0 {
		t.Errorf("判据③ 破（%d 处）：\n  %s", len(errs), joinErrs(errs))
	} else {
		t.Logf("判据③ ✓ ④ 的 %d 条收敛成 %d 组承接项，逐组带 why + next + 承接",
			d.Tally.Pending, len(d.PendingItems))
	}
}

func TestAdoptRetireLedgerMatchesDisk(t *testing.T) {
	root, d, _ := loadAR(t)
	if errs := judgeARRetireLedger(root, d); len(errs) > 0 {
		t.Errorf("判据④ 破（%d 处）：\n  %s", len(errs), joinErrs(errs))
	} else {
		t.Logf("判据④ ✓ ③ 台账 %d 件与盘上一致（`scripts/253/` 三件均已删、不在盘；compressor_demo 已改判 ② ⇒ 不在退役台账；正门退场件 state=已登记退场 亦入闭集）", len(d.RetireLedger))
	}
}

func TestAdoptRetireExclusionsClean(t *testing.T) {
	root, _, _ := loadAR(t)
	if errs := judgeARExclusions(root); len(errs) > 0 {
		t.Errorf("判据⑤ 破（%d 处）：\n  %s", len(errs), joinErrs(errs))
	} else {
		t.Logf("判据⑤ ✓ `scripts/253/` 三件不在盘上 · 两处排除域名单（EXCL_SUBDIRS / NOSUFFIX_EXCL）里零 `253`")
	}
}

// TestAdoptRetireExclusionsRetiredTolerant —— 判据⑤ 的**退役容忍**（2026-10-07 退役批）。
//
//	① 退役态：名单件 py 半不在盘 ∧ 其 Go 半在盘 ⇒ 该格不过问（0 处）；
//	② fail-closed：两半皆不在盘 ⇒ 逐件点名（len(arExclusionFiles) 处）。
//	★ 哑化负控：抽掉 judgeARExclusions 里「退役容忍」那段（`if os.IsNotExist(err)` …）
//	  ⇒ ① 格当场 FAIL（判词「判据⑤ 破：名单件读不到」）。
//	★ 夹具全在仓外（t.TempDir）：只造 goHalf 或全空 ⇒ ✗ 不碰真仓任何件。
func TestAdoptRetireExclusionsRetiredTolerant(t *testing.T) {
	// ① 退役容忍态：只放 Go 半，py 半缺席。
	rootA := t.TempDir()
	for _, f := range arExclusionFiles {
		p := filepath.Join(rootA, filepath.FromSlash(f.goHalf))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if errs := judgeARExclusions(rootA); len(errs) != 0 {
		t.Errorf("退役容忍态该 0 处（py 退役 ∧ Go 半在盘 ⇒ 不过问），得到 %d 处：\n  %s", len(errs), joinErrs(errs))
	}
	// ② fail-closed：两半皆不在盘 ⇒ 逐件点名。
	rootB := t.TempDir()
	if errs := judgeARExclusions(rootB); len(errs) != len(arExclusionFiles) {
		t.Errorf("两半皆缺席该逐件点名 %d 处，得到 %d 处：\n  %s", len(arExclusionFiles), len(errs), joinErrs(errs))
	}
}

func TestAdoptRetireItaskCriteriaHold(t *testing.T) {
	root, d, _ := loadAR(t)
	mainGo := readRepo(t, root, "core/cmd/zerg/main.go")
	fam := readRepo(t, root, "core/cmd/zerg/family_itask.go")
	if errs := judgeARItask(root, d, mainGo, fam); len(errs) > 0 {
		t.Errorf("判据⑥ 破（%d 处）：\n  %s", len(errs), joinErrs(errs))
	} else {
		t.Logf("判据⑥ ✓ `itask` 7 条命令路径逐条在命令树里 · 9 条端点逐条有命令映射" +
			"（4 条只读在实现件里命中 · 3 条写面点名自己的 POST 端点且走确认档门 `*Guarded` · 1 条写面未开放（mode · 命令树里零 `mode set`）· `interval` 的 POST 已开放 ⇒ 命令树里必须有 `itask interval set`（正控））")
	}
}

// ── 负控：判定口必须**带牙**（喂改动过的合成件 ⇒ 每条判据都要报错） ─────────────

func TestAdoptRetireJudgeHasTeeth(t *testing.T) {
	root, d, raw := loadAR(t)
	mainGo := readRepo(t, root, "core/cmd/zerg/main.go")
	fam := readRepo(t, root, "core/cmd/zerg/family_itask.go")
	// 正控：真件全绿。
	for name, n := range map[string]int{
		"算术": len(judgeARArithmetic(d)), "决议": len(judgeARDecisions(d)), "承接": len(judgeARPending(d)),
		"台账": len(judgeARRetireLedger(root, d)), "名单": len(judgeARExclusions(root)),
		"itask": len(judgeARItask(root, d, mainGo, fam)),
	} {
		if n != 0 {
			t.Fatalf("负控前的正控就不绿：%s 判据报 %d 处", name, n)
		}
	}
	if len(raw) == 0 {
		t.Fatal("真源是空文件")
	}

	// 负控 ①：面列被改一格 ⇒ 算术判据必须报。
	bad := cloneAR(t, d)
	bad.Faces[0].Adopt++
	if len(judgeARArithmetic(bad)) == 0 {
		t.Errorf("负控① 失败：面里 ① 改一格，算术判据没报（假绿）")
	}
	// 负控 ②：把那 1 条口径差抹平 ⇒ 必须报（不许把口径差藏起来）。
	bad2 := cloneAR(t, d)
	bad2.Tally.PendingRowVsSlot.BySevenGroupSlots = bad2.Tally.PendingRowVsSlot.ByRows
	bad2.Tally.PendingRowVsSlot.Diff = 0
	if len(judgeARArithmetic(bad2)) == 0 {
		t.Errorf("负控② 失败：把那 1 条口径差抹平，算术判据没报")
	}
	// 负控 ③：决议 status 写个闭集外的值 ⇒ 必须报。
	bad3 := cloneAR(t, d)
	bad3.Decisions[0].Status = "大概是落了吧"
	if len(judgeARDecisions(bad3)) == 0 {
		t.Errorf("负控③ 失败：status 写闭集外的值，决议判据没报")
	}
	// 负控 ④：一组承接项把 next / 承接 留白 ⇒ 必须报（「不许长期停在待定」）。
	bad4 := cloneAR(t, d)
	bad4.PendingItems[0].Follow = ""
	if len(judgeARPending(bad4)) == 0 {
		t.Errorf("负控④ 失败：承接项留白，待定判据没报")
	}
	// 负控 ⑤：把已经删掉的件写成「待删」（台账撒谎）⇒ 必须报。
	bad5 := cloneAR(t, d)
	for i := range bad5.RetireLedger {
		if strings.Contains(bad5.RetireLedger[i].State, "已删") {
			bad5.RetireLedger[i].State = "待删（本枚未授权）"
			break
		}
	}
	if len(judgeARRetireLedger(root, bad5)) == 0 {
		t.Errorf("负控⑤ 失败：把已删件写成待删，台账判据没报")
	}
	// 负控 ⑥：命令清单塞一条不存在的 ⇒ 必须报。
	bad6 := cloneAR(t, d)
	bad6.Itask.Commands7 = append(bad6.Itask.Commands7, "zzz")
	if len(judgeARItask(root, bad6, mainGo, fam)) == 0 {
		t.Errorf("负控⑥ 失败：命令清单里有盘上没有的路径，itask 判据没报")
	}
	// 负控 ⑦：把一条端点映射删掉（收编面漏一条）⇒ 必须报。
	bad7 := cloneAR(t, d)
	bad7.Itask.Mapping = bad7.Itask.Mapping[1:]
	if len(judgeARItask(root, bad7, mainGo, fam)) == 0 {
		t.Errorf("负控⑦ 失败：端点映射少一条，itask 判据没报")
	}
	// 负控 ⑧：给未开放的写面塞一个 `itask mode set` 子命令（假开放）⇒ 必须报。
	bad8 := cloneAR(t, d)
	fake := mainGo + "\n		{path: []string{\"itask\", \"mode\", \"set\"}},\n"
	if len(judgeARItask(root, bad8, fake, fam)) == 0 {
		t.Errorf("负控⑧ 失败：未开放的写面被塞了子命令，itask 判据没报")
	}
	// 负控 ⑨（P0 #232 加强后的负控仍有牙）：把**已开放**的 `itask interval set` 子命令从命令树里删掉 ⇒
	//   ⑤′ 正控必须报（「必须出现」不是恒真断言）。
	cut := strings.Replace(mainGo, `[]string{"itask", "interval", "set"}`, `[]string{"itask", "interval"}`, 1)
	if cut == mainGo {
		t.Fatal("负控⑨ 前置不成立：mainGo 里找不到 `[]string{\"itask\", \"interval\", \"set\"}`（命令树没登记？）")
	}
	if len(judgeARItask(root, d, cut, fam)) == 0 {
		t.Errorf("负控⑨ 失败：删掉已开放的 `itask interval set` 子命令，⑤′ 正控没报（假绿）")
	}
}

// cloneAR —— 负控用：把真源过一遍 JSON 克隆（值语义复制，改副本不碰真件）。
func cloneAR(t *testing.T, d arDoc) arDoc {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("克隆真源失败：%v", err)
	}
	var out arDoc
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("克隆真源解不动：%v", err)
	}
	return out
}

// judgeARExport —— 判据⑦：导出物（Zerg-内部文档 的归属清单）与真源的四个总数/五个面数**逐格相同**。
// 导出物缺失时 Skip（两仓分家 · 本层不自己造它）—— 在时则必须逐字符串对上。
func judgeARExport(exportText string, d arDoc) []error {
	var errs []error
	if strings.TrimSpace(exportText) == "" {
		return []error{errors.New("判据⑦ 破：导出物是空的")}
	}
	want := []string{
		fmt.Sprintf("**273 = %d（①）+ %d（②）+ %d（③）+ %d（④）**", d.Was.Tally["adopt"], d.Was.Tally["internal"], d.Was.Tally["retire"], d.Was.Tally["pending"]),
		fmt.Sprintf("**273 = %d（①）+ %d（②）+ %d（③）+ %d（④）**", d.Tally.Adopt, d.Tally.Internal, d.Tally.Retire, d.Tally.Pending),
		fmt.Sprintf("| **合计** | **%d** | **%d** | **%d** | **%d** | **%d** | **41 / 145 / 4 / 83** |",
			d.Tally.Objects, d.Tally.Adopt, d.Tally.Internal, d.Tally.Retire, d.Tally.Pending),
	}
	for _, h := range []string{
		fmt.Sprintf("## ① 收编为命令（%d 条）", d.Tally.Adopt),
		fmt.Sprintf("## ② 保留为内部实现（%d 条）", d.Tally.Internal),
		fmt.Sprintf("## ③ 退役（%d 条）", d.Tally.Retire),
		fmt.Sprintf("## ④ 待定（%d 条）", d.Tally.Pending),
	} {
		want = append(want, h)
	}
	for _, w := range want {
		if !strings.Contains(exportText, w) {
			errs = append(errs, fmt.Errorf("判据⑦ 破：导出物里找不到 `%s`（真源与导出物不同源）", w))
		}
	}
	for _, f := range d.Faces {
		row := fmt.Sprintf("| %s | %d | %d | %d | %d | %d |", f.Face, f.Objects, f.Adopt, f.Internal, f.Retire, f.Pending)
		if !strings.Contains(exportText, row) {
			errs = append(errs, fmt.Errorf("判据⑦ 破：导出物的计数表里没有面行 `%s`", row))
		}
	}
	return errs
}

func TestAdoptRetireExportMatchesTruth(t *testing.T) {
	_, d, _ := loadAR(t)
	docs := zergDocsRoot(t) // 同级没有 Zerg-内部文档 时自动 Skip
	p := filepath.Join(docs, "项目文档", "v2.5.10", "归属-收编与退役-20260920.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("导出物不在（%v）—— 两仓分家时本层不自己造它", err)
	}
	if errs := judgeARExport(string(b), d); len(errs) > 0 {
		t.Errorf("判据⑦ 破（%d 处）：\n  %s", len(errs), joinErrs(errs))
	} else {
		t.Logf("判据⑦ ✓ 导出物与原真源逐格相同（稿 41/145/4/83 · 落回后 %d/%d/%d/%d · 五行分面逐行）",
			d.Tally.Adopt, d.Tally.Internal, d.Tally.Retire, d.Tally.Pending)
	}
}

func joinErrs(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "\n  ")
}

// TestAdoptRetireItaskIntervalSetFourStates —— 判据⑥ 正控加强的**四态真跑**（P0 #232 第三面 · 2026-10-08）：
// `zerg itask interval set` 这条**已开放**的 D2 写面，从**当前源码现编**（`zergBinaryFromSource` · ✗ 不碰 `bin/`）
// 逐态真跑四态：
//
//	① 裸跑（缺确认档）⇒ rc=2 · stdout 0 字节（**零写**：确认档前不落任何状态）；
//	② `--dry-run`    ⇒ rc=0 · 出计划件（零副作用）；
//	③ `--yes`（确认档齐 · 对**合成主控**）⇒ rc=0 · 主控收到 POST `/api/internal-tasks/{id}/interval` 逐字体
//	   `{"hours":6}` · 回读 GET `/api/internal-tasks/intervals` 报出**落地值**（落地值回读）；
//	④ 坏值 `--hours abc` ⇒ rc=2 · stdout 0 字节（**零写**）。
//
// ★ 本测试**不用活主控**（`ZERG_PORT` 指本机合成 server）· **不碰真状态目录**（真写那一态只打合成 server）。
func TestAdoptRetireItaskIntervalSetFourStates(t *testing.T) {
	bin := zergBinaryFromSource(t)

	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/internal-tasks/health-check/interval":
			b, _ := io.ReadAll(r.Body)
			gotPath, gotBody = r.URL.Path, string(b)
			w.WriteHeader(200)
			fmt.Fprint(w, `{"success":true,"id":"health-check","hours":6,"message":"ok","state":{}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/internal-tasks/intervals":
			w.WriteHeader(200)
			fmt.Fprint(w, `{"intervals":{"health-check":6},"note":"","state":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	port := strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port)

	run := func(argv ...string) (int, string, string) {
		cmd := exec.Command(bin, argv...)
		cmd.Env = append(os.Environ(), "ZERG_PORT="+port, "ZERG_HOST=testhost")
		cmd.Stdin = strings.NewReader("")
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		rc := 0
		if err := cmd.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				rc = ee.ExitCode()
			} else {
				t.Fatalf("起不了现编二进制：%v", err)
			}
		}
		return rc, out.String(), errb.String()
	}

	if rc, out, _ := run("itask", "interval", "set", "health-check", "--hours", "6"); rc != 2 || out != "" {
		t.Errorf("① 裸跑：rc=%d（要 2）· stdout=%q（要空 —— 零写）", rc, out)
	}
	if rc, out, _ := run("itask", "interval", "set", "health-check", "--hours", "6", "--dry-run"); rc != 0 || !strings.Contains(out, "计划件") {
		t.Errorf("② --dry-run：rc=%d（要 0）· stdout 里没计划件：%q", rc, out)
	}
	gotPath, gotBody = "", ""
	rc, out, errb := run("itask", "interval", "set", "health-check", "--hours", "6", "--confirm=testhost", "--yes")
	if rc != 0 {
		t.Errorf("③ --yes：rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if gotPath != "/api/internal-tasks/health-check/interval" || gotBody != `{"hours":6}` {
		t.Errorf("③ 合成主控收到的请求不对：path=%q body=%q（要 path=/api/internal-tasks/health-check/interval · body={\"hours\":6}）", gotPath, gotBody)
	}
	if !strings.Contains(out, "6") {
		t.Errorf("③ 落地值回读没报出 6：stdout=%q", out)
	}
	if rc, out, _ := run("itask", "interval", "set", "health-check", "--hours", "abc"); rc != 2 || out != "" {
		t.Errorf("④ 坏值：rc=%d（要 2）· stdout=%q（要空 —— 零写）", rc, out)
	}
}
