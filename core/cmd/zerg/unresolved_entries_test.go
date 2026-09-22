// unresolved_entries_test.go —— 七个未定入口的**归属取证判据机检**（§7.1 `P13` · §7.2 `U21` · 开工单 T-53）。
//
// 本条立的是「**取证**」这条判据，不是归属 —— 归属要人拍（`P13`）。
//
// ★ 口径 v2（2026-09-22 · 治「同一类回归第二次犯」）：证据**不再按字面行号钉**。
//
//	  v1 的口径是「`file` + `line` + 那一行必须含入口名」，它已被同一类漂移打红**两次**
//	  （70→80 · 96→97；158→168 · 173→183；本轮 compat 两条真身再漂 +74：168→242 · 183→257 ——
//	  元凶 `b7c1d471`(G-05 · +6) 与 `e71d07a5`(G-06 · +60) 都只是在 `build-all.sh` **前面插行**）。
//	  ⇒ 只要有人在证据件前面插行，判据就打红，而**证据本身一个字没变**（假红）。
//	  现在三条判据逐条落成断言：
//
//		① **7 件各有一行「谁在调我」**，且每条证据是**可复核的内容锚**：`file` + `anchor`。
//		  锚必须在件里**逐字命中且唯一**（`strings.Count` 口径）：
//		    · 命中 **0 处** ⇒ 红，错因逐字「锚不存在」（证据挂在空气上）
//		    · 命中 **>1 处** ⇒ 红，错因逐字「锚不唯一（命中 N 处：行 …）」—— **不许静默取第一处**；
//		      要么收紧锚串，要么让锚跨行写成**上下文窗**（锚串里带 `\n`，例：compat 那条 = 241 行宣告 + 242 行构建）
//		  `line_hint` 只是**提示**（= 锚命中处的起始行）：漂了**只记一行警告、不判红** ——
//		  这一条就是「在证据件头部插一行，判据仍绿」那个性质（真实件上的正控见
//		  unresolved_entries_anchor_drift_test.go）。
//		② **拍板前 7 件一个都不许删**（`P13` 逐字）：7 个入口目录 + 各自的 main.go 必须在盘上。
//		③ **归属未定者不许立命令名**（判据③ 逐字「`zerg trace` 这类名字**先不立命令**」）：
//		  命令树里不许出现以 `compat` / `review` / `scheduler` / `trace` 开头的命令路径。
//
// 负控：判定口 `judgeUnresolved` 是三格判据的**唯一判定口**，负控直接喂它坏台账
// （锚空 / 锚不存在 / 锚不唯一 / 证据件读不到 / 入口目录没了 / 命令树里冒出 `trace` / 空表）⇒ 每格都必须报错。
// 另有一条**对偶正控**：锚对而行号提示写错 ⇒ **不判红**，但必须留下警告（否则「行号不再是判据」这句话没有机检）。
package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// repoRootFromCLI —— 仓根：`core/cmd/zerg` 往上三级（与 cli_exec_test.go 找 bin/ 的口径同源）。
func repoRootFromCLI(t *testing.T) string {
	t.Helper()
	if v := strings.TrimSpace(os.Getenv("ZERG_REPO")); v != "" {
		return v
	}
	abs, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("仓根解析不了：%v", err)
	}
	return abs
}

// anchorHit —— 一条证据在真实件上的**现跑命中**（判据① 的现读面：供清单/漂移告警/复现打印）。
type anchorHit struct {
	EntryID   string
	File      string
	Anchor    string
	Count     int // 锚在件内的命中次数：1 = 合法 · 0 = 锚不存在 · >1 = 锚不唯一
	StartLine int // 锚命中处的起始行（1 基；命中 0 处 ⇒ 0）
	SpanLines int // 锚跨几行（1 = 单行锚；>1 = 上下文窗）
	LineHint  int // 台账里的行号提示（0 = 没写）
	HintDrift int // 现跑锚起行 − 提示行（0 = 准；≠0 = 提示漂了，只警告）
}

// unresolvedVerdict —— 判定口的输出：Errs 判红 · Warns 只记（不判红）· Hits 现跑命中清单。
type unresolvedVerdict struct {
	Errs  []error
	Warns []error
	Hits  []anchorHit
}

func (v unresolvedVerdict) ok() bool { return len(v.Errs) == 0 }

// clip —— 把锚/原文缩到一行里可读（只影响报错措辞，不影响判定）。
func clip(s string) string {
	s = strings.ReplaceAll(s, "\n", "⏎")
	r := []rune(s)
	if len(r) > 46 {
		return string(r[:46]) + "…"
	}
	return s
}

// hitLines —— 锚的全部命中起行（「锚不唯一」的错因要能点名，不能只说「不唯一」）。
func hitLines(txt, anchor string) string {
	var at []string
	off := 0
	for {
		i := strings.Index(txt[off:], anchor)
		if i < 0 {
			break
		}
		at = append(at, fmt.Sprint(strings.Count(txt[:off+i], "\n")+1))
		off += i + len(anchor)
	}
	return strings.Join(at, ", ")
}

// isPublicFaceTree —— 判的是不是**公开树**（§五 第四批的「按所判的那棵树分列判」）：
//
//	· 私有树：`publish/replace-rules.tsv` 在场（发布机制自身**不进公开面** —— publish/private-paths.txt 逐条登记）；
//	· 公开树：`publish/` 整棵不在，而 `publish/ci/release-agent.yml` 被 publish-public.sh 的 MAPPINGS
//	  铺成公开树的 `.github/workflows/release-agent.yml`。
//
// 两条都要求 ⇒ 判据夹具的**合成小树**（既无 publish/ 也无那个 workflow）被当**私有面**判 ——
// 宁可多判、不许少判：声明式不判只发生在**认得出的**公开树里。
func isPublicFaceTree(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "publish", "replace-rules.tsv")); err == nil {
		return false
	}
	_, err := os.Stat(filepath.Join(root, ".github", "workflows", "release-agent.yml"))
	return err == nil
}

// judgeUnresolved 是本票三格判据的**唯一判定口**。
// 参数一律显式传入（仓根 / 台账 / 命令树）—— 为的是让负控能喂合成件与 /tmp 副本进来，不碰真仓。
func judgeUnresolved(root string, led contract.UnresolvedLedger, cmdPaths []string) unresolvedVerdict {
	var v unresolvedVerdict
	if len(led.Entries) == 0 {
		v.Errs = append(v.Errs, fmt.Errorf("台账一件都没有（空转 = 假覆盖）"))
		return v
	}
	forbidden := map[string]bool{}
	for _, s := range led.ForbiddenCommandSegment {
		forbidden[s] = true
	}
	for _, e := range led.Entries {
		// ② 入口目录 + main.go 都还在（拍板前一个都不许删）
		if st, err := os.Stat(filepath.Join(root, e.Dir)); err != nil || !st.IsDir() {
			v.Errs = append(v.Errs, fmt.Errorf("判据② 破：入口 %s 的目录 %s 不在了（P13 逐字「拍板前一个都不许删」）",
				e.ID, e.Dir))
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Main)); err != nil {
			v.Errs = append(v.Errs, fmt.Errorf("判据② 破：入口 %s 的入口件 %s 不在了", e.ID, e.Main))
		}
		// ① 「谁在调我」必须有一行**可复核的内容锚**证据
		if strings.TrimSpace(e.WhoCalls) == "" {
			v.Errs = append(v.Errs, fmt.Errorf("判据① 破：入口 %s 没有写「谁在调我」那一行", e.ID))
		}
		if len(e.Callers) == 0 {
			v.Errs = append(v.Errs, fmt.Errorf("判据① 破：入口 %s 一条证据都没有（取证 = 至少一条 `file` + `anchor`）", e.ID))
		}
		for _, c := range e.Callers {
			// ④ 公开面对应件：**缺件不许留空**（§五 第四批 —— 公开面-only / 私有面-only 必须显式二选一）
			if strings.TrimSpace(c.PublicCounterpart) == "" {
				v.Errs = append(v.Errs, fmt.Errorf("判据④ 破：入口 %s 的证据 %s 没写「公开面对应件」—— "+
					"缺件不许留空（件在公开面同路径同在 ⇒ 写那个路径；不进公开面 ⇒ 写 %q）",
					e.ID, c.File, contract.UnresolvedPublicOnly))
				continue
			}
			if strings.TrimSpace(c.Anchor) == "" {
				v.Errs = append(v.Errs, fmt.Errorf("判据① 破：入口 %s 的证据 %s 没有内容锚（`anchor` 空）—— "+
					"v2 起行号只是提示（`line_hint`），锚才是判据", e.ID, c.File))
				continue
			}
			// 判哪一件：按**所判的那棵树**分列判（§五 第四批）——
			//   · 声明私有面-only 且判的是公开树 ⇒ 按声明**不判**（要留痕，不许静默少判）
			//   · 公开面对应件在场 ⇒ 判它；否则判 `file`（私有树就是这一支）
			if c.PublicCounterpart == contract.UnresolvedPublicOnly && isPublicFaceTree(root) {
				v.Warns = append(v.Warns, fmt.Errorf("判据④（声明式不判 · 不判红）：入口 %s 的证据 %s 声明「%s」"+
					"⇒ 公开树里按台账声明不判（私有面照判；这一条留痕是为了「少判」看得见）",
					e.ID, c.File, contract.UnresolvedPublicOnly))
				continue
			}
			target := c.File
			if c.PublicCounterpart != contract.UnresolvedPublicOnly && c.PublicCounterpart != c.File {
				if _, err := os.Stat(filepath.Join(root, c.PublicCounterpart)); err == nil {
					target = c.PublicCounterpart
				}
			}
			b, err := os.ReadFile(filepath.Join(root, target))
			if err != nil {
				v.Errs = append(v.Errs, fmt.Errorf("判据① 破：入口 %s 的证据件读不到 %s（%v）", e.ID, target, err))
				continue
			}
			txt := string(b)
			n := strings.Count(txt, c.Anchor)
			h := anchorHit{EntryID: e.ID, File: target, Anchor: c.Anchor, Count: n,
				SpanLines: strings.Count(c.Anchor, "\n") + 1, LineHint: c.LineHint}
			switch {
			case n == 0:
				v.Errs = append(v.Errs, fmt.Errorf("判据① 破：入口 %s 的锚不存在 —— %s 里一处都没有 %q（证据挂在空气上了）",
					e.ID, target, clip(c.Anchor)))
			case n > 1:
				v.Errs = append(v.Errs, fmt.Errorf("判据① 破：入口 %s 的锚不唯一 —— %s 里 %q 命中 %d 处（行 %s）⇒ "+
					"判据不许静默取第一处：收紧锚串，或让锚跨行写成上下文窗", e.ID, target, clip(c.Anchor), n,
					hitLines(txt, c.Anchor)))
			default:
				h.StartLine = strings.Count(txt[:strings.Index(txt, c.Anchor)], "\n") + 1
				if c.LineHint > 0 && c.LineHint != h.StartLine {
					h.HintDrift = h.StartLine - c.LineHint
					v.Warns = append(v.Warns, fmt.Errorf("提示漂（不判红）：入口 %s 的证据 %s 的行号提示 %d 已不是现跑锚起行 %d"+
						"（差 %+d）—— `line_hint` 只是提示，判据看锚；重取提示的命令见契约件的 `verify_command_hints`",
						e.ID, target, c.LineHint, h.StartLine, h.HintDrift))
				}
			}
			v.Hits = append(v.Hits, h)
		}
	}
	// ③ 归属未定者不许立命令名
	for _, p := range cmdPaths {
		seg := strings.SplitN(p, " ", 2)[0]
		if forbidden[seg] {
			v.Errs = append(v.Errs, fmt.Errorf("判据③ 破：命令树里有 `zerg %s …`（路径 %q）—— 归属未定（P13）前不许立命令名；"+
				"`zerg trace` 这类名字只能先不立", seg, p))
		}
	}
	return v
}

// ---- 判据 ①②③ 跑真件 ----

func TestUnresolvedEntriesHaveVerifiableEvidence(t *testing.T) {
	root := repoRootFromCLI(t)
	led, err := contract.Unresolved()
	if err != nil {
		t.Fatalf("取出台账读不出来：%v", err)
	}
	if led.Schema != contract.UnresolvedSchema {
		t.Errorf("台账 schema = %q（要 %q）—— v2 起证据必须带内容锚，v1 的行号口径已废",
			led.Schema, contract.UnresolvedSchema)
	}
	if len(led.Entries) != 7 {
		t.Errorf("台账 %d 条（要 7 条：`agent`/`api`/`compat`/`model`/`review`/`scheduler`/`trace`）", len(led.Entries))
	}
	// 口径与**可复现命令**必须在件内（不许只住在谁脑子里）：缺一条 ⇒ 红（「规则写了没人接电」的反面）
	for _, k := range []struct{ name, v string }{
		{"evidence_rule", led.EvidenceRule},
		{"hint_drift_policy", led.HintDriftPolicy},
		{"verify_command", led.VerifyCommand},
		{"verify_command_hints", led.VerifyCommandHints},
		{"verify_command_drift", led.VerifyCommandDrift},
	} {
		if strings.TrimSpace(k.v) == "" {
			t.Errorf("契约件缺 `%s`（口径条文与可复现命令必须在件内，详见 unresolved-entries.json）", k.name)
		}
	}
	v := judgeUnresolved(root, *led, zerg.CommandPathsForTest())
	for _, e := range v.Errs {
		t.Error(e)
	}
	for _, w := range v.Warns {
		t.Log(w) // 只记不判红（行号提示漂了不拦人）
	}
	// 逐条把「谁在调我」打成一行 + 每条证据的现跑口径（供记录引用）
	for _, e := range led.Entries {
		t.Logf("%s · %s ⇒ 证据 %d 条 · %s", e.ID, e.Bin, len(e.Callers), e.WhoCalls)
	}
	for _, h := range v.Hits {
		shape := "内容锚"
		if h.SpanLines > 1 {
			shape = fmt.Sprintf("内容锚（%d 行上下文窗）", h.SpanLines)
		}
		hint := "无"
		if h.LineHint > 0 {
			hint = fmt.Sprintf("行号提示 %d", h.LineHint)
			if h.HintDrift != 0 {
				hint = fmt.Sprintf("行号提示 %d（**已漂 %+d**）", h.LineHint, h.HintDrift)
			}
			shape += " + " + hint
		}
		t.Logf("  口径 · %s · %s · 命中 %d 处 · 锚起行 %d · %s", h.EntryID, h.File, h.Count, h.StartLine, shape)
	}
	t.Logf("复现：%s", led.VerifyCommand)
}

// TestUnresolvedLedgerHasNoLinePinnedResidue —— **静态自检**：台账里不许再有「按行号钉」的残迹。
//
// 为什么单独一条：口径换了，残留的旧字段会**静默**留在 json 里（谁也不会读它），
// 下一个改台账的人照着旧字段抄回来就又是一次漂移。这一条把它钉住：
//
//	· 每条证据都带 `anchor`（条数 == `file` 条数 == `line_hint` 条数）
//	· 一条 `"line":` 旧字段都不许有
//
// 负控在合成件上打：抹掉一个 `anchor` / 加回一个 `"line":` ⇒ 判定口必须报错。
func TestUnresolvedLedgerHasNoLinePinnedResidue(t *testing.T) {
	root := repoRootFromCLI(t)
	p := filepath.Join(root, "core/internal/contract/unresolved-entries.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("台账件读不到：%v", err)
	}
	raw := string(b)
	for _, e := range checkLedgerShape(raw) {
		t.Error(e)
	}
	// 负控（合成件）：判定口真的有牙
	mutants := []struct {
		name string
		raw  string
		why  string
	}{
		{"抹掉一条锚", strings.Replace(raw, `"anchor":`, `"anchorX":`, 1), "`anchor` 少一条"},
		{"旧行号字段回潮", strings.Replace(raw, `"anchor":`, "\"anchor\": \"x\", \"line\": 1,", 1), "`\"line\":` 回潮"},
	}
	for _, m := range mutants {
		if len(checkLedgerShape(m.raw)) == 0 {
			t.Errorf("负控（%s）失败：%s 没有被抓到", m.name, m.why)
		}
	}
	// 空转：空台账 ⇒ 不给结论（空转 = 假覆盖）
	if len(checkLedgerShape("{}")) == 0 {
		t.Error("负控（空台账）失败：空件被当成绿")
	}
}

// checkLedgerShape —— 台账**形状**的判定口（只读文本，不碰盘）：供真件自检与负控共用。
func checkLedgerShape(raw string) []error {
	var errs []error
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return []error{fmt.Errorf("台账是空的（空转 = 假覆盖）")}
	}
	if !strings.Contains(raw, `"schema": "`+contract.UnresolvedSchema+`"`) {
		errs = append(errs, fmt.Errorf("台账 schema 不是 %q（v2 起证据必须带内容锚）", contract.UnresolvedSchema))
	}
	nFile := strings.Count(raw, `"file":`)
	nAnchor := strings.Count(raw, `"anchor":`)
	nHint := strings.Count(raw, `"line_hint":`)
	nPub := strings.Count(raw, `"公开面对应件":`)
	if nFile == 0 {
		errs = append(errs, fmt.Errorf("台账里一条 `\"file\":` 都没有（空转 = 假覆盖）"))
	}
	if nAnchor != nFile {
		errs = append(errs, fmt.Errorf("`\"anchor\":` %d 条 ≠ `\"file\":` %d 条 —— 有证据没带内容锚", nAnchor, nFile))
	}
	if nHint != nFile {
		errs = append(errs, fmt.Errorf("`\"line_hint\":` %d 条 ≠ `\"file\":` %d 条 —— 有证据没给行号提示（提示是给人看的，缺了不影响判绿但台账不完整）", nHint, nFile))
	}
	if nPub != nFile {
		errs = append(errs, fmt.Errorf("`\"公开面对应件\":` %d 条 ≠ `\"file\":` %d 条 —— 有证据没写好「公开面对应件」（§五 第四批：缺件不许留空）", nPub, nFile))
	}
	if nEmpty := strings.Count(raw, `"公开面对应件": ""`); nEmpty != 0 {
		errs = append(errs, fmt.Errorf("有 %d 条「公开面对应件」是**空值** —— 缺件不许留空：件不进公开面就写字死的「%s」", nEmpty, contract.UnresolvedPublicOnly))
	}
	if n := strings.Count(raw, `"line":`); n != 0 {
		errs = append(errs, fmt.Errorf("台账里还有 %d 条旧字段 `\"line\":` —— v1 的按行号钉已废（会再漂一次）；行号只能写 `line_hint`", n))
	}
	return errs
}

// TestUnresolvedEvidenceHintRefresh —— **重取行号提示的工具**（不是判据：不判红绿）。
// 只在 ZERG_REFRESH_HINTS=1 时跑（默认 skip，免得每次 go test 都刷一屏）。
// 它按现跑逐条打印 `file` / `anchor` / 现跑锚起行 —— 照它改台账里的 `line_hint` 即可（锚一个字都不用动）。
func TestUnresolvedEvidenceHintRefresh(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ZERG_REFRESH_HINTS")) == "" {
		t.Skip("本测试是重取提示的工具（不是判据）：ZERG_REFRESH_HINTS=1 才跑")
	}
	led, err := contract.Unresolved()
	if err != nil {
		t.Fatalf("取出台账读不出来：%v", err)
	}
	v := judgeUnresolved(repoRootFromCLI(t), *led, nil)
	t.Logf("行号提示现读（共 %d 条证据 · %d 条提示已漂）：", len(v.Hits), len(v.Warns))
	for _, h := range v.Hits {
		flag := ""
		if h.HintDrift != 0 {
			flag = fmt.Sprintf(" ← 漂 %+d（旧提示 %d）", h.HintDrift, h.LineHint)
		}
		t.Logf("  %s\t%s\tline_hint=%d%s", h.EntryID, h.File, h.StartLine, flag)
	}
}

// ---- 负控：判定口**真的有牙**（每格一条 + 一条对偶正控） ----

func TestUnresolvedJudgeHasTeeth(t *testing.T) {
	root := t.TempDir()
	// 合成一个「入口件在、证据件在」的最小仓：先证它**不报错**（正控），再逐格破坏它
	mustWrite(t, filepath.Join(root, "core/cmd/zerg-trace/main.go"), "package main\n")
	mustWrite(t, filepath.Join(root, "README.md"), "第一行\n| zerg-trace | 追踪工具 |\n第三行\n")
	good := contract.UnresolvedLedger{
		ForbiddenCommandSegment: []string{"trace"},
		Entries: []contract.UnresolvedEntry{{
			ID: "trace", Dir: "core/cmd/zerg-trace", Main: "core/cmd/zerg-trace/main.go",
			Bin: "zerg-trace", WhoCalls: "文档一行", Verdict: "待拍",
			Callers: []contract.UnresolvedCaller{{File: "README.md", Anchor: "| zerg-trace | 追踪工具 |", LineHint: 2, PublicCounterpart: "README.md", What: "文档登记行"}},
		}},
	}
	if v := judgeUnresolved(root, good, []string{"task ls"}); !v.ok() {
		t.Fatalf("正控失败：好台账被误判红：%v", v.Errs)
	}
	// 克隆一件「带一条证据的单件台账」——负控逐格在克隆上改一个字段
	clone := func(caller contract.UnresolvedCaller, dir string) contract.UnresolvedLedger {
		e := good.Entries[0]
		e.Dir = dir
		e.Callers = []contract.UnresolvedCaller{caller}
		return contract.UnresolvedLedger{ForbiddenCommandSegment: good.ForbiddenCommandSegment,
			Entries: []contract.UnresolvedEntry{e}}
	}

	// ①-a 锚不存在（把锚串改成件里没有的东西 —— 正是「证据件被改掉/搬走」那一格）
	{
		v := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "README.md", Anchor: "zerg-trace 的这一句不在件里", PublicCounterpart: "README.md"}, good.Entries[0].Dir), nil)
		if v.ok() {
			t.Error("负控①-a 失败：锚不存在没有被抓到")
		} else if !strings.Contains(fmt.Sprint(v.Errs), "锚不存在") {
			t.Errorf("负控①-a 失败：报错没写清错因「锚不存在」：%v", v.Errs)
		}
	}

	// ①-b 锚不唯一（同一串在件里出现两次 ⇒ 不许静默取第一处）
	{
		mustWrite(t, filepath.Join(root, "DUP.md"), "| zerg-trace | 追踪工具 |\n别的\n| zerg-trace | 追踪工具 |\n")
		v := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "DUP.md", Anchor: "| zerg-trace | 追踪工具 |", PublicCounterpart: "DUP.md"}, good.Entries[0].Dir), nil)
		if v.ok() {
			t.Error("负控①-b 失败：锚不唯一没有被抓到（静默取了第一处）")
		} else if !strings.Contains(fmt.Sprint(v.Errs), "锚不唯一") || !strings.Contains(fmt.Sprint(v.Errs), "1, 3") {
			t.Errorf("负控①-b 失败：报错没写清错因「锚不唯一」+ 命中行号：%v", v.Errs)
		}
	}

	// ①-c 锚是空的（还想按行号判 ⇒ 直接红：行号不是判据）
	{
		v := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "README.md", LineHint: 2, PublicCounterpart: "README.md"}, good.Entries[0].Dir), nil)
		if v.ok() || !strings.Contains(fmt.Sprint(v.Errs), "没有内容锚") {
			t.Errorf("负控①-c 失败：空锚没有被抓到（或错因不清）：%v", v.Errs)
		}
	}

	// ①-d 证据件读不到
	{
		v := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "没有这件.md", Anchor: "x", PublicCounterpart: "没有这件.md"}, good.Entries[0].Dir), nil)
		if v.ok() {
			t.Error("负控①-d 失败：证据件读不到没有被抓到")
		}
	}

	// ①-e 对偶正控：**锚对、行号提示写错** ⇒ 不判红，但必须留警告（「行号不是判据」的机检）
	{
		v := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "README.md", Anchor: "| zerg-trace | 追踪工具 |", LineHint: 99, PublicCounterpart: "README.md"}, good.Entries[0].Dir), nil)
		if !v.ok() {
			t.Errorf("对偶正控失败：行号提示写错（锚是对的）被判红了 —— 行号不许当判据：%v", v.Errs)
		}
		if len(v.Warns) == 0 {
			t.Error("对偶正控失败：行号提示漂了却一条警告都没有（等于悄悄失效）")
		}
	}

	// ② 入口目录没了（拍板前删了件）
	if v := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "README.md", Anchor: "| zerg-trace | 追踪工具 |"}, "core/cmd/zerg-已删"), nil); v.ok() {
		t.Error("负控② 失败：入口目录被删没有被抓到（P13 逐字「拍板前一个都不许删」）")
	}

	// ③ 命令树里冒出未定名的命令
	if v := judgeUnresolved(root, good, []string{"trace", "task ls"}); v.ok() {
		t.Error("负控③ 失败：命令树里 `zerg trace` 没有被抓到")
	}

	// ④ 公开面对应件（§五 第四批）：缺件不许留空 · 分列判 · 声明式不判 —— 三格都要有牙
	{
		// ④-a 列是空的 ⇒ 必红（「缺件不许留空」那条判据）
		v := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "README.md",
			Anchor: "| zerg-trace | 追踪工具 |"}, good.Entries[0].Dir), nil)
		if v.ok() || !strings.Contains(fmt.Sprint(v.Errs), "公开面对应件") ||
			!strings.Contains(fmt.Sprint(v.Errs), "缺件不许留空") {
			t.Errorf("负控④-a 失败：空「公开面对应件」没有被抓到（或错因不清）：%v", v.Errs)
		}
	}
	{
		// ④-b 声明「只在私有面成立」**不是豁免**：在（认得出的）私有面树里照样判锚 ⇒ 锚错必红
		v := judgeUnresolved(root, clone(contract.UnresolvedCaller{File: "README.md",
			Anchor: "这一句不在件里", PublicCounterpart: contract.UnresolvedPublicOnly}, good.Entries[0].Dir), nil)
		if v.ok() || !strings.Contains(fmt.Sprint(v.Errs), "锚不存在") {
			t.Errorf("负控④-b 失败：声明「只在私有面成立」在私有面树里被当成了豁免：%v", v.Errs)
		}
	}
	// ④-c/④-d 公开树：合成一棵**认得出是公开树**的小树（`publish/` 不在 · 但有 MAPPINGS 铺出来的 workflow）
	{
		pubRoot := t.TempDir()
		mustWrite(t, filepath.Join(pubRoot, "core/cmd/zerg-trace/main.go"), "package main\n")
		mustWrite(t, filepath.Join(pubRoot, ".github/workflows/release-agent.yml"),
			"jobs:\n  go:\n    steps:\n      - run: GOOS=linux GOARCH=amd64 go build -o zerg-agent-$GOOS-$GOARCH ./cmd/zerg-agent\n")
		led2 := contract.UnresolvedLedger{
			ForbiddenCommandSegment: []string{"trace"},
			Entries: []contract.UnresolvedEntry{{
				ID: "trace", Dir: "core/cmd/zerg-trace", Main: "core/cmd/zerg-trace/main.go",
				Bin: "zerg-trace", WhoCalls: "CI 工作流出它 + 文档登记", Verdict: "待拍",
				Callers: []contract.UnresolvedCaller{
					{File: "publish/ci/release-agent.yml", Anchor: "zerg-agent-$GOOS-$GOARCH", LineHint: 1,
						PublicCounterpart: ".github/workflows/release-agent.yml",
						What:              "CI 交叉编译（公开面对应件在场 ⇒ 判它）"},
					{File: "docs/skills/zerg-overview.md", Anchor: "| zerg-trace | 追踪工具 |", LineHint: 1,
						PublicCounterpart: contract.UnresolvedPublicOnly, What: "只在私有面成立的文档行"},
				},
			}},
		}
		v := judgeUnresolved(pubRoot, led2, nil)
		if !v.ok() {
			t.Errorf("正控④-c 失败：公开树里「对应件在场 ⇒ 判它」本该绿，实测红：%v", v.Errs)
		}
		hitPub := false
		for _, h := range v.Hits {
			if h.File == ".github/workflows/release-agent.yml" && h.Count == 1 {
				hitPub = true
			}
		}
		if !hitPub {
			t.Errorf("正控④-c 失败：判的不是公开面对应件（Hits=%v）", v.Hits)
		}
		// ④-d 声明私有面-only 的证据在公开树里**不判**，但必须留痕（少判要看得见，不许静默）
		noted := false
		for _, w := range v.Warns {
			if strings.Contains(fmt.Sprint(w), contract.UnresolvedPublicOnly) {
				noted = true
			}
		}
		if !noted {
			t.Errorf("正控④-d 失败：公开树里声明私有面-only 的证据既没判也没留痕（静默少判）：warns=%v", v.Warns)
		}
		if len(v.Hits) != 1 {
			t.Errorf("正控④-d 失败：公开树该只判 1 条（对应件那条），实测 %d 条", len(v.Hits))
		}
	}

	// 空转：空台账 ⇒ 不给结论（不许当绿）
	if v := judgeUnresolved(root, contract.UnresolvedLedger{}, nil); v.ok() {
		t.Error("负控-空转 失败：空台账被当成绿（空转 = 假覆盖）")
	}
}
