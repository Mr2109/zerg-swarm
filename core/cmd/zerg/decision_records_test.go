// decision_records_test.go —— 决策记录与经验落库的**判据机检**（§20.1 步 9 · §十二 `P-134`–`P-138` · 开工单 T-63）。
//
// 真源 = `core/internal/contract/decision-records.json`（定稿已给出的取值原样落形状）；
// 被检面 = `Zerg-内部文档/01-设计/决策记录/`（`P-134` 的落点 + 同目录 INDEX）。
//
// 判据逐条：
//
//	① 最小留痕三格（`DR-NNNN` + `who_decided` + `when_decided`）**每份件都有且非空**（`P-135`）
//	② 落点：件只在 `Zerg-内部文档/01-设计/决策记录/` 下（`P-134` 的闭集一个值）
//	③ 经验落点闭集 **4 类**（`P-138`：`Zerg-内部文档` / `AGENTS.md` / `tools/` / 状态目录 —— 不新造）
//	④ **换会话 / 换 AI 读得到**：INDEX 列出的 DR 号集 === 目录里的件集（漂了即红）
//	⑤ `P-136`「一物两态」的落判在真源里**照实记**（含「未收口」三字）—— 不许悄悄写成已合
//
// 另有 8 必填 + 4 收口（`P-137`）的整表判据与「`who_decided` 只人」的安全格（AI 身份写入 ⇒ 红）。
// 负控：判定口 judgeDecisionRecords 是唯一判定口，负控逐格喂坏件（缺格 / 缺三格 / AI 拍板 /
// 提者=批者 / status 越界 / accepted 无证据 / INDEX 漂 / 经验落点 3 类）⇒ 每格都必须报错。
package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// drDirRel —— 决策记录的落点（相对 Zerg-内部文档 仓根）。
const drDirRel = "01-设计/决策记录"

// drBlockRe 抓正文里的 ```decision-record 围栏块（字段表那一块）。
var drBlockRe = regexp.MustCompile("(?s)```decision-record\\n(.*?)```")

// parseDecisionRecord 解一份 DR 件：frontmatter 之后的 ```decision-record 块 ⇒ key/value。
func parseDecisionRecord(text string) (map[string]string, error) {
	m := drBlockRe.FindStringSubmatch(text)
	if m == nil {
		return nil, fmt.Errorf("件里没有 ```decision-record 围栏块（字段表的落点）")
	}
	out := map[string]string{}
	for _, line := range strings.Split(m[1], "\n") {
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		if k != "" {
			out[k] = v
		}
	}
	return out, nil
}

// drLive —— 一份被检件的实况（供判定口用，负控可以不碰盘直接喂）。
type drLive struct {
	Name   string
	Fields map[string]string
}

// judgeDecisionRecords 是 T-63 全部判据的**唯一判定口**。
// files = 目录里的件（不含 INDEX）；index = INDEX.md 的正文。
func judgeDecisionRecords(spec *contract.DecisionRecordSpec, files []drLive, index string) []error {
	var errs []error
	// 真源自身的三格（空真源 = 判据没有对象，不许当绿）
	if spec == nil || len(spec.RequiredFields) == 0 || len(spec.StatusSet) == 0 {
		return []error{fmt.Errorf("真源不完整（required_fields / status_set 为空）⇒ 不给结论")}
	}
	// ③ 经验落点闭集 4 类（`P-138` 逐字，不新造）
	if len(spec.ExperienceDirs) != 4 {
		errs = append(errs, fmt.Errorf("判据③ 破：经验落点闭集 = %d 类（要 4 类：Zerg-内部文档 / AGENTS.md / tools/ / 状态目录）",
			len(spec.ExperienceDirs)))
	}
	for _, want := range []string{"Zerg-内部文档", "AGENTS.md", "tools/"} {
		found := false
		for _, d := range spec.ExperienceDirs {
			if strings.Contains(d, want) {
				found = true
			}
		}
		if !found {
			errs = append(errs, fmt.Errorf("判据③ 破：经验落点闭集里没有 %q（照 SD5 的既有落点，不新造）", want))
		}
	}
	// ⑤ 一物两态（`P-136`）的落判必须照实记
	if !strings.Contains(spec.OneThingTwoStates, "未收口") {
		errs = append(errs, fmt.Errorf("判据⑤ 破：`P-136` 一物两态的落判里没有「未收口」三字 —— "+
			"提案件在状态目录、决策记录在 Zerg-内部文档，两处**不是一个目录**，不许写成已合"))
	}
	if len(files) == 0 {
		return append(errs, fmt.Errorf("落点下一份决策记录都没有（空转 = 假覆盖）"))
	}
	ids := []string{}
	for _, f := range files {
		get := func(k string) string { return strings.TrimSpace(f.Fields[k]) }
		// ② 落点：件名必须是 `NNNN-<标题>.md` 形态
		if !regexp.MustCompile(`^\d{4}-\S.*\.md$`).MatchString(f.Name) {
			errs = append(errs, fmt.Errorf("判据② 破：件名 %q 不是 `NNNN-<标题>.md` 形态（P-134 的落点）", f.Name))
		}
		// ① 三格（`P-135`）
		if !regexp.MustCompile(`^DR-\d{4}$`).MatchString(get("id")) {
			errs = append(errs, fmt.Errorf("判据① 破：%s 的记录号 %q 不是 `DR-NNNN` 形态", f.Name, get("id")))
		}
		if get("who_decided") == "" {
			errs = append(errs, fmt.Errorf("判据① 破：%s 缺 `who_decided`（最小留痕三格之一）", f.Name))
		}
		if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(get("when_decided")) {
			errs = append(errs, fmt.Errorf("判据① 破：%s 的 `when_decided` %q 不是 YYYY-MM-DD", f.Name, get("when_decided")))
		}
		ids = append(ids, get("id"))
		// 8 必填（`P-137`）
		for _, k := range spec.RequiredFields {
			if get(k) == "" {
				errs = append(errs, fmt.Errorf("%s 缺必填格 `%s`（P-137 的 8 必填）", f.Name, k))
			}
		}
		// why 两段
		for _, seg := range spec.WhySegments {
			if !strings.Contains(get("why"), seg) {
				errs = append(errs, fmt.Errorf("%s 的 `why` 缺 %q 段（照字段表：context + decision 两段，decision 用主动语态）", f.Name, seg))
			}
		}
		// `who_decided` 只人（AI 身份写入 ⇒ 红）
		for _, mark := range spec.AIDeniedMarkers {
			if mark != "" && strings.Contains(get("who_decided"), mark) {
				errs = append(errs, fmt.Errorf("%s 的 `who_decided` 命中 AI 标识 %q ⇒ 红（字段表逐字：who_decided 只人；AI 提的不得由 AI 批）", f.Name, mark))
			}
		}
		// 提 ≠ 批
		if get("who_proposed") != "" && get("who_proposed") == get("who_decided") {
			errs = append(errs, fmt.Errorf("%s 的 `who_proposed` == `who_decided` ⇒ 红（H3 提 ≠ 批：AI 提的提案不得由 AI 批准）", f.Name))
		}
		// status 闭集
		ok := false
		for _, s := range spec.StatusSet {
			if get("status") == s {
				ok = true
			}
		}
		if !ok {
			errs = append(errs, fmt.Errorf("%s 的 `status` %q 不在闭集 %v 里", f.Name, get("status"), spec.StatusSet))
		}
		// accepted ⇒ criteria + evidence 必填（收口规则）
		if get("status") == "accepted" {
			if get("criteria") == "" {
				errs = append(errs, fmt.Errorf("%s 是 accepted 却没有 `criteria`（收口规则：status: accepted 时必填）", f.Name))
			}
			if get("evidence") == "" {
				errs = append(errs, fmt.Errorf("%s 是 accepted 却没有 `evidence`（证据为空 ⇒ 不许给结论）", f.Name))
			}
		}
	}
	// ④ INDEX ↔ 件集一致（换会话读得到 · 不会漂）
	sort.Strings(ids)
	listed := []string{}
	for _, m := range regexp.MustCompile("`(DR-\\d{4})`").FindAllStringSubmatch(index, -1) {
		listed = append(listed, m[1])
	}
	listed = dedupeSorted(listed)
	if strings.Join(ids, ",") != strings.Join(listed, ",") {
		errs = append(errs, fmt.Errorf("判据④ 破：INDEX 列的 DR 号 %v ≠ 目录里的件 %v —— 换会话/换 AI 读到的索引已经漂了", listed, ids))
	}
	return errs
}

func dedupeSorted(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// zergDocsRoot 找同级的 Zerg-内部文档 仓根（两仓分家 · 部署形态固定）。找不到 ⇒ 交给调用方 Skip。
func zergDocsRoot(t *testing.T) string {
	t.Helper()
	if v := strings.TrimSpace(os.Getenv("ZERG_DOCS")); v != "" {
		return v
	}
	p := filepath.Join(repoRootFromCLI(t), "..", "Zerg-内部文档")
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		t.Skipf("同级没有 Zerg-内部文档（两仓分家 · 本层不自己造它）：%s", abs)
	}
	return abs
}

// ---- 判据 ①②③④⑤ 跑真件 ----

func TestDecisionRecordsLandTheShape(t *testing.T) {
	spec, err := contract.DecisionRecords()
	if err != nil {
		t.Fatalf("决策记录真源读不出来：%v", err)
	}
	if spec.LandingDir != "Zerg-内部文档/01-设计/决策记录/NNNN-<标题>.md" {
		t.Errorf("落点 = %q（要 `Zerg-内部文档/01-设计/决策记录/NNNN-<标题>.md` · P-134 逐字）", spec.LandingDir)
	}
	docs := zergDocsRoot(t)
	dir := filepath.Join(docs, drDirRel)
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("落点目录读不出来（%s）：%v —— P-134 的落点必须先立起来", dir, err)
	}
	var files []drLive
	index := ""
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if e.Name() == "INDEX.md" {
			index = string(b)
			continue
		}
		f, err := parseDecisionRecord(string(b))
		if err != nil {
			t.Errorf("%s：%v", e.Name(), err)
			continue
		}
		files = append(files, drLive{Name: e.Name(), Fields: f})
	}
	for _, e := range judgeDecisionRecords(spec, files, index) {
		t.Error(e)
	}
	t.Logf("判据③ 经验落点闭集 %d 类：%s", len(spec.ExperienceDirs), strings.Join(spec.ExperienceDirs, " / "))
	t.Logf("判据④ INDEX ↔ 件集：%d 份件 · %d 份件都被索引点到", len(files), len(files))
	for _, f := range files {
		t.Logf("   %s · %s 拍 · %s · status=%s", f.Fields["id"], f.Fields["who_decided"], f.Fields["when_decided"], f.Fields["status"])
	}
	t.Logf("判据⑤ 一物两态落判（P-136）：%s", spec.OneThingTwoStates[:min(60, len(spec.OneThingTwoStates))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---- 负控：判定口**真的有牙**（每格一条） ----

func TestDecisionRecordJudgeHasTeeth(t *testing.T) {
	spec, err := contract.DecisionRecords()
	if err != nil {
		t.Fatalf("真源读不出来：%v", err)
	}
	good := drLive{Name: "0001-好件.md", Fields: map[string]string{
		"id": "DR-0001", "title": "t", "who_proposed": "Hermes 会话（AI）", "who_decided": "Mr2109",
		"when_decided": "2026-09-20", "why": "context=c ; decision=We will d", "impact": "i",
		"status": "accepted", "criteria": "c", "related": "r", "supersedes": "（无）", "evidence": "e",
	}}
	goodIndex := "| `DR-0001` | t | Mr2109 | 2026-09-20 | accepted |"
	if errs := judgeDecisionRecords(spec, []drLive{good}, goodIndex); len(errs) != 0 {
		t.Fatalf("正控失败：好件被误判红：%v", errs)
	}
	mutate := func(k, v string) []drLive {
		f := map[string]string{}
		for kk, vv := range good.Fields {
			f[kk] = vv
		}
		if v == "" {
			delete(f, k)
		} else {
			f[k] = v
		}
		return []drLive{{Name: good.Name, Fields: f}}
	}
	cases := []struct {
		name  string
		files []drLive
		index string
		why   string
	}{
		{"① 缺 who_decided", mutate("who_decided", ""), goodIndex, "最小留痕三格之一"},
		{"① 缺 when_decided", mutate("when_decided", ""), goodIndex, "最小留痕三格之一"},
		{"① 记录号不合法", mutate("id", "DR-1"), goodIndex, "DR-NNNN 形态"},
		{"8 必填缺 why", mutate("why", ""), goodIndex, "P-137 的 8 必填"},
		{"why 缺 decision 段", mutate("why", "context=c"), goodIndex, "两段（context + decision）"},
		{"who_decided 是 AI", mutate("who_decided", "Hermes 会话（AI）"), goodIndex, "who_decided 只人"},
		{"提者=批者", mutate("who_proposed", "Mr2109"), goodIndex, "H3 提 ≠ 批"},
		{"status 越界", mutate("status", "provisional"), goodIndex, "status 闭集"},
		{"accepted 无证据", mutate("evidence", ""), goodIndex, "证据为空 ⇒ 不许给结论"},
		{"② 件名不成形", []drLive{{Name: "决策一.md", Fields: good.Fields}}, goodIndex, "NNNN-<标题>.md"},
		{"④ INDEX 漂", []drLive{good}, "| `DR-0001` | t |\n| `DR-0002` | 幽灵 |", "INDEX ↔ 件集一致"},
		{"空转：一件都没有", nil, goodIndex, "空转 = 假覆盖"},
	}
	for _, c := range cases {
		if errs := judgeDecisionRecords(spec, c.files, c.index); len(errs) == 0 {
			t.Errorf("负控失败（%s）：没有被抓到 —— 该格要的是「%s」", c.name, c.why)
		}
	}
	// ③ 经验落点闭集少一类 ⇒ 红
	bad := *spec
	bad.ExperienceDirs = []string{"Zerg-内部文档", "AGENTS.md", "tools/"}
	if errs := judgeDecisionRecords(&bad, []drLive{good}, goodIndex); len(errs) == 0 {
		t.Error("负控失败（③ 经验落点 3 类）：没有被抓到")
	}
	// ⑤ 一物两态被写成已合 ⇒ 红
	bad = *spec
	bad.OneThingTwoStates = "一物两态：提案件就是 status: proposed 的决策记录（已合）"
	if errs := judgeDecisionRecords(&bad, []drLive{good}, goodIndex); len(errs) == 0 {
		t.Error("负控失败（⑤ 一物两态写成已合）：没有被抓到")
	}
	// 真源空 ⇒ 不给结论
	if errs := judgeDecisionRecords(&contract.DecisionRecordSpec{}, []drLive{good}, goodIndex); len(errs) == 0 {
		t.Error("负控失败（真源空）：空真源被当成绿")
	}
}
