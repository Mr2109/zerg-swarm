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
	"os"
	"path/filepath"
	"regexp"
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

var arExclusionFiles = []struct{ path, linePrefix, list string }{
	{"scripts/gates/check-wired-scripts.py", "EXCL_SUBDIRS", "253"},
	{"scripts/gates/check-gate-coverage.py", "NOSUFFIX_EXCL", "253"},
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
func judgeARRetireLedger(root string, d arDoc) []error {
	var errs []error
	if len(d.RetireLedger) != 4 {
		errs = append(errs, fmt.Errorf("判据④ 破：③ 应 4 件（3 件 253 + compressor_demo），台账有 %d 件", len(d.RetireLedger)))
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
		default:
			errs = append(errs, fmt.Errorf("判据④ 破：%s 的 state = %q 既不是「已删」也不是「待删」", e.Object, e.State))
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
		if !strings.Contains(mainGo[i:end], "cmdGuarded") {
			errs = append(errs, fmt.Errorf("判据⑥ 破：写面 `itask %s` 没走 cmdGuarded（危险档失守）", v))
		}
	}
	// ⑤ 未开放的写面（mode/interval 的 POST）：命令树里**不许**出现 `itask mode set` 这种子命令。
	for _, v := range d.Itask.WriteNotOpen {
		pat := regexp.MustCompile(`\[\]string\{"itask",\s*"` + regexp.QuoteMeta(v) + `",\s*"set"\}`)
		if pat.MatchString(mainGo) {
			errs = append(errs, fmt.Errorf("判据⑥ 破：真源写 `itask %s` 写面未开放，命令树里却有 `%s set` 子命令", v, v))
		}
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
		t.Logf("判据④ ✓ ③ 台账 4 件与盘上一致（3 件已删不在盘 · 1 件待删仍在盘）")
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

func TestAdoptRetireItaskCriteriaHold(t *testing.T) {
	root, d, _ := loadAR(t)
	mainGo := readRepo(t, root, "core/cmd/zerg/main.go")
	fam := readRepo(t, root, "core/cmd/zerg/family_itask.go")
	if errs := judgeARItask(root, d, mainGo, fam); len(errs) > 0 {
		t.Errorf("判据⑥ 破（%d 处）：\n  %s", len(errs), joinErrs(errs))
	} else {
		t.Logf("判据⑥ ✓ `itask` 7 条命令路径逐条在命令树里 · 9 条端点逐条有命令映射" +
			"（4 条只读在实现件里命中 · 3 条写面点名自己的 POST 端点且走 cmdGuarded · 2 条写面未开放且命令树里零 `set` 子命令）")
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
