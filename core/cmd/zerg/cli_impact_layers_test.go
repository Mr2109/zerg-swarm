// cli_impact_layers_test.go —— `zerg impact` 的**六层取数**（`A2`）判据
// （任务单-影响面实施-20260922 §二 `A2` 判据①–③ · 设计-变更影响面-v1.6 §二/§3.7/§九 判据⑥）。
//
// 落在 `package main_test`（§九 M17 三层测试落点的层①②）：`RunForTest` 跑的是**当前源码**的
// 运行期行为，不是盘上旧制品。
//
// 四条判据的机检口（**都在本文件里真跑**）：
//
//	① 层规三件(`head_sha` + `layer` + `该层口径值`) + 层序三判据 —— 判定口 `judgeImpactLayerLines`，
//	   带成对负控（少 `口径=` / 少 `head_sha=` / ② 层少 `algo=` ⇒ **必须报错**）；
//	② 第 ⑤ 层在位判据三条子句可测 —— 判定口 `impactSemanticGate`（**纯函数** ⇒ 三枚坏输入直接喂）；
//	③ 第 ⑥ 层正控 + 负控（`v1.6` §九 判据⑥）—— 命中生效面 ⇒ **必带**那一行（口径三件齐）；
//	   不在 ⇒ **必须不打**（证明它不是恒返回一行）；
//	④ 第 ⑥ 层的「该不该公开」**判不了**（人判）—— 不可机检，照实标（写在注释里，不假装有判据）。
package main_test

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// impactZeroHitTarget —— **零命中**的件目标（现跑 rc=1）：`core/internal/plugin/runtime/` 下那一件
// （该包无人 import · 件名在 tracked 面里无人提 · 不命中任何契约条目 —— 三层都空才是真零命中）。
//
// ★ **它的件名在源码里用拼的，不许逐字写出来** —— 写出来它就落进**词法面**（`git grep -w <件名>`），
// 于是它自己把「零命中」变成「有命中」。这不是洁癖：A2 落地时真撞到过这一格（见回执）。
func impactZeroHitTarget() string {
	return "core/internal/plugin/runtime/" + "lla" + "ma.go"
}

// impactLayerLineRe —— 层表一行的形状（判据①：`layer=` + `口径=` + `head_sha=` 三件**必在**）。
// 层名与粒度里都可能有空格（例：`④词法 + 形近层` · `文件级 + 名字级`）⇒ 非贪婪取值。
var impactLayerLineRe = regexp.MustCompile(`^  层([①-⑥]) layer=([①-⑥])(.+?) 粒度=(.+?) 口径=(.+) 状态=(\S+) 耗时=(\S+) head_sha=(\S+) 时刻=(\S+)$`)

// judgeImpactLayerLines —— 判据①的**唯一判定口**：层表六行，三件必在、层序固定、② 层必带 `algo`。
// 抽出来的目的：负控能直接喂坏行（把「三件」里任一件摘掉 ⇒ 必须报错）。
func judgeImpactLayerLines(stderr string) error {
	lines := []string{}
	for _, ln := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(ln, "  层") && strings.Contains(ln, "layer=") {
			lines = append(lines, ln)
		}
	}
	if len(lines) != 6 {
		return errStr("层表不是六层（实得 " + itoaSafe(len(lines)) + " 行）")
	}
	wantSeq := []string{"①", "②", "③", "④", "⑤", "⑥"}
	for i, ln := range lines {
		m := impactLayerLineRe.FindStringSubmatch(ln)
		if m == nil {
			return errStr("层表第 " + itoaSafe(i+1) + " 行缺三件之一（layer=/口径=/head_sha=）：" + ln)
		}
		if m[1] != wantSeq[i] || m[2] != wantSeq[i] {
			return errStr("层序不对：第 " + itoaSafe(i+1) + " 行是 " + m[1] + "（要 " + wantSeq[i] + "）")
		}
		if m[8] == "" || m[8] == "（取不到）" {
			return errStr("层 " + m[1] + " 缺 head_sha")
		}
		if m[5] == "" {
			return errStr("层 " + m[1] + " 缺该层口径值")
		}
		if m[2] == "②" && !strings.Contains(m[5], "algo=") {
			return errStr("② 符号层的口径里没带 algo —— 不带算法名的边数不可比（`R13` 必填）")
		}
		if m[7] == "" {
			return errStr("层 " + m[1] + " 缺耗时（单层成本要能单独量）")
		}
	}
	return nil
}

func itoaSafe(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// TestImpactLayers_LayerTableAndTier —— 判据① 正控 + 成对负控：
//
//	正控：默认档跑一件 ⇒ stderr 层表**六行**、三件齐、层序 ①–⑥、② 层带 `algo=rta`；
//	判据（b）**前层不依赖后层**：② 在默认档**未跑**，而 ③④⑤⑥ 仍然**取值**（一层没跑不拖累别层）；
//	负控：三枚坏行（摘 `口径=` / 摘 `head_sha=` / ② 层摘 `algo=`）喂进判定口 ⇒ **必须报错**。
func TestImpactLayers_LayerTableAndTier(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))
	// `A5`：`zerg impact` 自本批起**会落盘缓存**（落点在状态目录）⇒ 测试一律把状态目录改到合成目录，
	// **不碰真状态目录**（本件判据一个字不动；不这么做就是「测试有副作用」）。
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	tgt := "core/cmd/zerg/main.go"

	rc, out, errb := runCapture("impact", tgt)
	if rc != 0 {
		t.Fatalf("有影响面的件 ⇒ 期望 rc=0，得到 %d · stderr=%s", rc, errb)
	}
	if err := judgeImpactLayerLines(errb); err != nil {
		t.Fatalf("判据① 破：%v", err)
	}
	if !strings.Contains(errb, "结果随仓变而变") || !strings.Contains(errb, "head_sha=") {
		t.Errorf("时效声明没打（§3.1「没带 head_sha 的卡片不许出」）：%s", tail(errb, 400))
	}
	// （b）前层不依赖后层：② 未跑，③④⑤⑥ 仍取值。
	if !strings.Contains(errb, "状态=未跑") {
		t.Errorf("默认档里贵层应写「未跑」（不许悄悄少给几层）：%s", tail(errb, 600))
	}
	for _, seq := range []string{"③", "④", "⑤", "⑥"} {
		if !strings.Contains(errb, "层"+seq+" layer="+seq) || strings.Contains(lineOf(errb, "  层"+seq+" "), "状态=未跑") {
			t.Errorf("层 %s 在前层未跑时也应取值（前层不依赖后层）：%s", seq, lineOf(errb, "  层"+seq+" "))
		}
	}
	// 负控：三枚坏行必红。
	good := lineOf(errb, "  层① layer=")
	if good == "" {
		t.Fatalf("拿不到 ① 层行（准备态不对）")
	}
	noCaliber := strings.Replace(good, "口径=", "口径X=", 1)
	noSHA := strings.Replace(good, "head_sha=", "headX_sha=", 1)
	l2 := lineOf(errb, "  层② layer=")
	noAlgo := strings.Replace(l2, "algo=", "algox=", 1)
	if err := judgeImpactLayerLines(strings.Replace(errb, good, noCaliber, 1)); err == nil {
		t.Error("负控失败：① 层摘掉 `口径=` 竟判过 —— 「三件」这一格没牙")
	}
	if err := judgeImpactLayerLines(strings.Replace(errb, good, noSHA, 1)); err == nil {
		t.Error("负控失败：① 层摘掉 `head_sha=` 竟判过")
	}
	if l2 != "" {
		if err := judgeImpactLayerLines(strings.Replace(errb, l2, noAlgo, 1)); err == nil {
			t.Error("负控失败：② 层摘掉 `algo=` 竟判过 —— 「不带算法名的边数不可比」这一格没牙")
		}
	}
	// 判据③（负控）：main.go **不在**七件生效面上 ⇒ 人面**必须不打**「公开面」那一行。
	if strings.Contains(out, "公开面：") {
		t.Errorf("负控失败：不在生效面上的件竟打了公开面行 —— 它是恒返回一行就废了：%s", out)
	}
	if n := countNonEmpty(out); n != 3 {
		t.Errorf("人面不是三行（实得 %d 行 —— 没命中就不许多带行）：%s", n, out)
	}
}

// TestImpactLayers_SemanticGateThreeClauses —— 判据②（第 ⑤ 层**在位判据可测**）：
// 三条子句（`/api/tags` 非空 · `capabilities` 含 `embedding` · `embedding_length` 有值）
// 由纯函数判 —— 正控一 + 负控三（各摘一条子句 ⇒ 必须判「不在位」）。
func TestImpactLayers_SemanticGateThreeClauses(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	// `A5`：本用例也要**现跑**一次 `impact`（判据③的现读面）⇒ 状态目录同样改到合成目录。
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	ok := []byte(`{"models":[{"name":"all-minilm:latest","capabilities":["embedding"],"details":{"embedding_length":384}}]}`)
	if pass, model, detail := zerg.ImpactSemanticGateForTest(ok); !pass || model != "all-minilm:latest" {
		t.Errorf("正控破：在位判据竟判「不在位」（model=%q · %s）", model, detail)
	}
	empty := []byte(`{"models":[]}`)
	if pass, _, _ := zerg.ImpactSemanticGateForTest(empty); pass {
		t.Error("负控①失败：`/api/tags` 空（0 个模型）竟判「在位」")
	}
	noCap := []byte(`{"models":[{"name":"m","capabilities":["completion"],"details":{"embedding_length":384}}]}`)
	if pass, _, _ := zerg.ImpactSemanticGateForTest(noCap); pass {
		t.Error("负控②失败：`capabilities` 不含 `embedding` 竟判「在位」")
	}
	noDim := []byte(`{"models":[{"name":"m","capabilities":["embedding"],"details":{"embedding_length":0}}]}`)
	if pass, _, _ := zerg.ImpactSemanticGateForTest(noDim); pass {
		t.Error("负控③失败：`embedding_length` 无值竟判「在位」")
	}
	if pass, _, _ := zerg.ImpactSemanticGateForTest([]byte(`{`)); pass {
		t.Error("负控④失败：解不开的响应竟判「在位」")
	}
	// 现跑（真端点 · 只读）：层表 ⑤ 行**三子句**都要打出来（判不了就照实说不在位）。
	rc, _, errb := runCapture("impact", "core/cmd/zerg/main.go")
	if rc != 0 {
		t.Fatalf("准备态不对：rc=%d", rc)
	}
	if !strings.Contains(errb, "子句①") || !strings.Contains(errb, "子句②") || !strings.Contains(errb, "子句③") {
		t.Errorf("层表 ⑤ 行的在位判据三子句没逐条打：%s", lineOf(errb, "    读数：在位判据"))
	}
}

// TestImpactLayers_PublicFacePositiveAndNegative —— 判据③（`v1.6` §九 判据⑥）成对：
//
//	正控：改一个**在生效面上**的件（`scripts/公开标记.tsv` 就是七件之一）⇒ 卡片**必带**
//	      「公开面：此改动会改变公开产出树 +N / −M 件」行，且**口径三件齐**（件计数口径 ·
//	      产出树路径 + 扫的时刻 · `head_sha`）；
//	负控：不在生效面上的件 ⇒ **必须不打**（在另一件里已判，这里再判一次「打没打」的成对）。
func TestImpactLayers_PublicFacePositiveAndNegative(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))
	// `A5`：`zerg impact` 自本批起**会落盘缓存**（落点在状态目录）⇒ 测试一律把状态目录改到合成目录，
	// **不碰真状态目录**（本件判据一个字不动；不这么做就是「测试有副作用」）。
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	rc, out, errb := runCapture("impact", "scripts/公开标记.tsv")
	if rc != 0 {
		t.Fatalf("生效面上的件 ⇒ 期望 rc=0，得到 %d · stderr=%s", rc, errb)
	}
	line := lineOf(out, "公开面：")
	if line == "" {
		t.Fatalf("正控破：命中七件生效面却没打公开面行 · out=%s", out)
	}
	if !strings.Contains(line, "此改动会改变公开产出树") || !strings.Contains(line, "+") || !strings.Contains(line, "−") {
		t.Errorf("公开面行的字面不合 §3.7（`+N / −M 件`）：%s", line)
	}
	// 口径三件齐：件计数口径（-type f · 排 .git/vendor）· 产出树路径 + 扫的时刻 · head_sha。
	for _, want := range []string{"-type f", "扫的时刻", "head_sha", "件（口径"} {
		if !strings.Contains(line, want) {
			t.Errorf("公开面行缺口径三件之一（%q）：%s", want, line)
		}
	}
	if len(strings.Split(line, "公开面：")) != 2 {
		t.Errorf("公开面行不止一行：%s", line)
	}
	// 负控（成对 · 真跑）：普通件 ⇒ 一行都不许有。
	_, out2, _ := runCapture("impact", "core/cmd/zerg/main.go")
	if strings.Contains(out2, "公开面：") {
		t.Errorf("负控破：普通件也打了公开面行：%s", out2)
	}
}

// TestImpactLayers_PublicLineThreePieces —— `C5` 判据①②③（公开面产出树行）：
//
//	① 正控（真仓现跑）：在生效面上的件 ⇒ **必出**这一行，且**三件齐** + 第三方可复算的
//	   同一条命令在行里（`find … | wc -l` + 同相对路径 `test -e`）；
//	② 负控甲（纯函数喂坏输入）：三件**缺一**（`head_sha` / 产出树路径 / 扫的时刻）⇒ 只许写
//	   「公开面：未取数」，且**不许出现数字那一句**；
//	③ 负控乙（纯函数）：不在生效面 ⇒ **空串**（一行都不许打）；
//	④ 负控丙（真跑 · 端到端）：产出树读不到（`ZERG_PUB_TREE` 指一个不存在的目录）⇒ 人面那一行
//	   必须是「未取数」，**不是**去打一个 0 或拿旧数顶上。
func TestImpactLayers_PublicLineThreePieces(t *testing.T) {
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	rc, out, errb := runCapture("impact", "scripts/公开标记.tsv")
	if rc != 0 {
		t.Fatalf("生效面上的件 ⇒ 期望 rc=0，得到 %d · stderr=%s", rc, tail(errb, 300))
	}
	line := lineOf(out, "公开面：")
	if line == "" {
		t.Fatalf("正控破：命中生效面却没打公开面行 · out=%s", out)
	}
	for _, want := range []string{"此改动会改变公开产出树", "-type f", "排 .git/vendor", "扫的时刻", "head_sha",
		"复算（第三方 · 同一口径）", "find ", "-not -path '*/.git/*'", "test -e ", "不合并"} {
		if !strings.Contains(line, want) {
			t.Errorf("`C5` 正控破：公开面行缺 %q：%s", want, line)
		}
	}

	// ② 三件缺一 ⇒ 未取数（三种缺法各喂一次 —— **纯函数**，不连真仓）。
	for _, tc := range []struct{ name, tree, path, at, head string }{
		{"缺 head_sha", "树（1 件）", "/tmp/t", "2026-09-22T00:00:00+08:00", ""},
		{"缺产出树路径", "树（1 件）", "", "2026-09-22T00:00:00+08:00", "abc123"},
		{"缺扫的时刻", "树（1 件）", "/tmp/t", "", "abc123"},
	} {
		got := zerg.ImpactPublicLineTextForTest(true, "+1 / −0", tc.tree, tc.path, tc.at, tc.head, "")
		if !strings.HasPrefix(got, "公开面：未取数") {
			t.Errorf("负控甲（%s）：该写「未取数」，实得 %q", tc.name, got)
		}
		if strings.Contains(got, "此改动会改变公开产出树") {
			t.Errorf("负控甲（%s）：三件不齐却打出了数字那一句：%q", tc.name, got)
		}
	}
	// 命中但带缺件原因（层里那一路）⇒ 未取数 + 原因逐字带上。
	if got := zerg.ImpactPublicLineTextForTest(true, "", "", "", "", "", "产出树路径读不到（/tmp/没这一棵）"); !strings.Contains(got, "未取数") || !strings.Contains(got, "读不到") {
		t.Errorf("负控甲（层给的原因）：该写未取数并把原因逐字带上，实得 %q", got)
	}
	// ③ 不在生效面 ⇒ 空串。
	if got := zerg.ImpactPublicLineTextForTest(false, "+1 / −0", "树（1 件）", "/tmp/t", "2026-09-22T00:00:00+08:00", "abc123", ""); got != "" {
		t.Errorf("负控乙：不在生效面竟打了一行（这一行不是恒返回一行）：%q", got)
	}
	// ④ 端到端：产出树读不到 ⇒ 未取数（不是 0，也不是不打）。
	t.Setenv("ZERG_PUB_TREE", filepath.Join(t.TempDir(), "没这一棵产出树"))
	rc2, out2, errb2 := runCapture("impact", "scripts/公开标记.tsv")
	if rc2 != 0 {
		t.Fatalf("负控丙：退码该仍是 0（这一行不是闸），得到 %d · stderr=%s", rc2, tail(errb2, 300))
	}
	line2 := lineOf(out2, "公开面：")
	if line2 == "" {
		t.Error("负控丙：命中生效面时这一行**必须在**（三件不齐 ⇒ 写未取数，而不是不打）")
	} else {
		if !strings.Contains(line2, "未取数") {
			t.Errorf("负控丙：产出树读不到 ⇒ 该写未取数，实得 %s", line2)
		}
		if strings.Contains(line2, "此改动会改变公开产出树") {
			t.Errorf("负控丙：三件不齐却打出了数字那一句：%s", line2)
		}
	}
}

// TestImpactLayers_MachineFaceItemsCarryLayerWhy —— 判据①在**机器面**上的那一半：
// `--json` 的 `items[]` 每条都要有 `why`，且取值落在**层名闭集**里（来路不明的东西进不来）。
func TestImpactLayers_MachineFaceItemsCarryLayerWhy(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))
	// `A5`：`zerg impact` 自本批起**会落盘缓存**（落点在状态目录）⇒ 测试一律把状态目录改到合成目录，
	// **不碰真状态目录**（本件判据一个字不动；不这么做就是「测试有副作用」）。
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	rc, out, errb := runCapture("impact", "core/cmd/zerg/main.go", "--json", "what,why,how,red")
	if rc != 0 {
		t.Fatalf("期望 rc=0，得到 %d · stderr=%s", rc, errb)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("机器面不是对象：%v", err)
	}
	for _, k := range impactEnvelopeKeys {
		if _, ok := doc[k]; !ok {
			t.Fatalf("六键形状破：缺 %s", k)
		}
	}
	var items []map[string]string
	if err := json.Unmarshal(doc["items"], &items); err != nil {
		t.Fatalf("items 不是数组：%v", err)
	}
	if len(items) == 0 {
		t.Fatal("有影响面的件 ⇒ items 不该空（这正是 `A2` 要接的：骨架期的三个 0 是「未取数」）")
	}
	closed := map[string]bool{"包反向": true, "调用边": true, "契约": true, "词法": true, "形近": true, "义近": true, "公开面": true}
	for i, it := range items {
		if it["why"] == "" {
			t.Errorf("第 %d 条没有 why（「没有 why 的条目不进卡片」）：%+v", i, it)
			continue
		}
		if !closed[it["why"]] {
			t.Errorf("第 %d 条的 why = %q 不在闭集里：%+v", i, it["why"], it)
		}
		if !strings.Contains(it["what"], ":") {
			t.Errorf("第 %d 条的 what 没带粒度前缀（`R40`：`what` 恒带前缀）：%+v", i, it)
		}
	}
}

// ---- 小工具（只在本文件用）-----------------------------------------------------------------

func countNonEmpty(s string) int {
	n := 0
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
}

func lineOf(s, prefix string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.Contains(ln, prefix) {
			return ln
		}
	}
	return ""
}

func tail(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n:])
}
