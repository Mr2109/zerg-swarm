// cli_envelope_truth_test.go —— 包封「**只让已有键有值**」这一批的判据（源件 = 缺口账
// `Zerg-内部文档/项目文档/v2.5.11/缺口-命令面-20260922.md` 的 `G-08`）。
//
// 本批做什么（一句话）：顶层**仍是那六键**（一个不多一个不少 ✗），变的是 `warnings[]` /
// `truncated` 两格的**真值**与 `meta` 的**按需子键**；拿不到真值就保持 `[]`/`false`/缺席，
// **不许编造**。
//
// 六条判据（可机检 · **每条配正控 + 成对负控**；判定口抽出来就是为了能喂坏期望）：
//
//	判据① 六键一个不少一个不多     正控 = 真跑 + 合成两路都判 · 负控 = 多一键 / 少一键 ⇒ 必红
//	判据② `truncated` 真值         正控 = 真裁条 ⇒ true · 成对负控 = 没裁 ⇒ false
//	判据③ `warnings[]` 真值        正控 = 真有事 ⇒ 非空 · 成对负控 = 无事 ⇒ 逐字 `[]`
//	判据④ 不编造信号（字面）       负控 = 空白项不算信号 / 包里 0 处「已裁」「未跑」这类词
//	判据⑤ `meta` 按需子键 + 旧子键语义不变  负控 = 同名子键不许覆盖 / 空值不写
//	判据⑥ 契约里**逐条列举**了哪几类真事才进 `warnings[]`（契约先于实现的机检）
//
// ★ 落点：`package main_test`（外部测试包）—— `RunForTest` 跑**当前源码**，不是盘上旧制品。
package main_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// ---- 判定口（抽出来 ⇒ 负控能直接喂坏期望）---------------------------------------------------

// judgeEnvelopeSixKeys 判据①的**唯一判定口**：顶层键**一个不少一个不多**。
func judgeEnvelopeSixKeys(doc map[string]json.RawMessage) error {
	want := zerg.EnvelopeKeysForTest()
	for _, k := range want {
		if _, ok := doc[k]; !ok {
			return errStr("包封缺键 " + k)
		}
	}
	for k := range doc {
		found := false
		for _, w := range want {
			if k == w {
				found = true
				break
			}
		}
		if !found {
			return errStr("包封多了未声明键 " + k + "（顶层只有那六键 ⇒ 这就是第七键）")
		}
	}
	return nil
}

// judgeEnvelopeWarningsNotFabricated 判据④的**唯一判定口**（静态面）：包封实现件里不许出现
// 「信号词」—— `warnings[]` 的内容一律来自**调用方给出的真值**，实现件自己不许造词。
//
// 为什么这条能判「不编造」：一个会造词的实现，必然在源码里写死至少一条告警文案；本件里一个都
// 不许有（真值由 `zerg impact` / `dev edit` 这些命令传进来 —— 词在它们那儿，不在这儿）。
func judgeEnvelopeWarningsNotFabricated(srcCode string) error {
	for _, word := range []string{"已裁 ", "未跑", "未取数", "读不到"} {
		if strings.Contains(srcCode, word) {
			return errStr("包封实现件（代码行）里出现了信号词 " + word + " —— 包封只搬运真值，不许自己造")
		}
	}
	return nil
}

// judgeContractEnumeratesWarnings 判据⑥的**唯一判定口**：契约正文里必须**逐条列举**哪几类真事
// 才进 `warnings[]`（不许只写「有告警就写进来」这种没牙的话）。
func judgeContractEnumeratesWarnings(help string) error {
	for _, must := range []string{"① 预算裁条", "② 层/面没跑", "③ 取不到真源", "④ 降级放行"} {
		if !strings.Contains(help, must) {
			return errStr("契约正文里没有逐条列举 " + must)
		}
	}
	if !strings.Contains(help, "一个不多一个不少") {
		return errStr("契约正文里没有写死「顶层键一个不多一个不少」")
	}
	return nil
}

// envelopeDoc 跑一条命令并把包封解析出来（正控用）。
func envelopeDoc(t *testing.T, argv ...string) (map[string]json.RawMessage, string) {
	t.Helper()
	rc, out, errb := runCapture(argv...)
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v 的输出不是对象（rc=%d）：%v · stdout=%q · stderr=%s", argv, rc, err, out, tail(errb, 400))
	}
	return doc, errb
}

// ---- 判据①：六键一个不少一个不多 ------------------------------------------------------------

func TestEnvelopeTruth_SixKeysExactly(t *testing.T) {
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// 正控 A：真跑三条命令（只读 · 三种 kind）+ 合成一路 ⇒ 逐条判六键。
	cases := [][]string{
		{"version", "--json", "name"},
		{"repo", "status", "--json", "head"},
		{"impact", impactZeroHitTarget(), "--json", "what"},
	}
	for _, argv := range cases {
		doc, _ := envelopeDoc(t, argv...)
		if err := judgeEnvelopeSixKeys(doc); err != nil {
			t.Errorf("正控失败 %v：%v", argv, err)
		}
	}
	// 正控 B：合成一路（判定口与真跑读**同一个** emitEnvelopeWith）。
	synth := zerg.EnvelopeRenderForTest("[]", 0, nil, false, nil)
	var sdoc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(synth), &sdoc); err != nil {
		t.Fatalf("合成包封解析失败：%v · %q", err, synth)
	}
	if err := judgeEnvelopeSixKeys(sdoc); err != nil {
		t.Errorf("正控失败（合成）：%v", err)
	}

	// 负控①：**多一键**（第七键）⇒ 判定口必红 —— 「不新增第七键」这条红线就靠这一格守。
	more := map[string]json.RawMessage{}
	for k, v := range sdoc {
		more[k] = v
	}
	more["envelope_ext"] = json.RawMessage(`{}`)
	if err := judgeEnvelopeSixKeys(more); err == nil {
		t.Error("负控①失败：多了第七键竟判过（这台机器判不出红）")
	}
	// 负控②：**少一键**（抽掉 warnings）⇒ 判定口必红。
	fewer := map[string]json.RawMessage{}
	for k, v := range sdoc {
		if k != "warnings" {
			fewer[k] = v
		}
	}
	if err := judgeEnvelopeSixKeys(fewer); err == nil {
		t.Error("负控②失败：少了 `warnings` 竟判过")
	}
	// 负控③：真跑的包封里**恰好**六键（不是「至少六键」）—— 直接数顶层键数。
	for _, argv := range cases {
		rc, out, _ := runCapture(argv...)
		if rc != 0 && rc != 1 {
			t.Errorf("%v 退码 = %d（本判据要 0/1 两档内的真跑）", argv, rc)
		}
		var d map[string]json.RawMessage
		_ = json.Unmarshal([]byte(out), &d)
		if len(d) != len(zerg.EnvelopeKeysForTest()) {
			t.Errorf("%v 顶层键数 = %d（要 %d）：%s", argv, len(d), len(zerg.EnvelopeKeysForTest()), out)
		}
	}
}

// ---- 判据②③：truncated / warnings 的真值与成对负控 ------------------------------------------

func TestEnvelopeTruth_TruncatedAndWarningsPair(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// 正控：`impact` 打在**真会被裁**的大件上（六层全量 > 卡片上限 ⇒ 卡片块自己报 truncated=true）。
	doc, errb := envelopeDoc(t, "impact", "core/cmd/zerg/main.go", "--json", "what,why")
	if !strings.Contains(errb, "本跑 truncated=true") {
		t.Fatalf("准备态不对：卡片块没报真裁（stderr 里找不到「本跑 truncated=true」）—— 换一个更大的件再判")
	}
	if string(doc["truncated"]) != "true" {
		t.Errorf("正控②失败：卡片真裁了，包封 `truncated` = %s（要 true）", string(doc["truncated"]))
	}
	var warns []string
	if err := json.Unmarshal(doc["warnings"], &warns); err != nil {
		t.Fatalf("正控③失败：`warnings` 不是字符串数组：%v", err)
	}
	if len(warns) == 0 {
		t.Fatal("正控③失败：真裁了却 `warnings[]` 空（§3.3 三件之二没落进包封）")
	}
	cut := false
	for _, w := range warns {
		if strings.HasPrefix(w, "已裁 ") {
			cut = true
		}
	}
	if !cut {
		t.Errorf("正控③失败：`warnings[]` 里没有「已裁 N 条」那条：%q", warns)
	}
	// `meta.layers_not_run[]` 与 `warnings[]` 同源（同一次取值 ⇒ 两处不会漂）。
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(doc["meta"], &meta); err != nil {
		t.Fatalf("`meta` 不是对象：%v", err)
	}
	var notRun []string
	if err := json.Unmarshal(meta["layers_not_run"], &notRun); err != nil {
		t.Fatalf("`meta.layers_not_run` 不是字符串数组：%v（%s）", err, string(meta["layers_not_run"]))
	}
	if len(notRun) == 0 || !strings.Contains(notRun[0], "runtime") {
		t.Errorf("`meta.layers_not_run[]` 首项该是恒在的 `runtime` 那一项，实得 %q", notRun)
	}
	for _, ln := range notRun[1:] { // 首项 = runtime（恒在 · 不是「没跑」那一类）
		hit := false
		for _, w := range warns {
			if w == ln {
				hit = true
			}
		}
		if !hit {
			t.Errorf("`meta.layers_not_run[]` 的「%s」没在 `warnings[]` 里（同一份取值 ⇒ 必须同源）", tail(ln, 80))
		}
	}

	// 成对负控（**没裁**）：零命中件 ⇒ 卡片一条不裁。
	neg, _ := envelopeDoc(t, "impact", impactZeroHitTarget(), "--json", "what")
	if string(neg["truncated"]) != "false" {
		t.Errorf("负控②失败：一条都没裁，包封 `truncated` = %s（要 false）", string(neg["truncated"]))
	}
	var nwarns []string
	_ = json.Unmarshal(neg["warnings"], &nwarns)
	for _, w := range nwarns {
		if strings.HasPrefix(w, "已裁 ") {
			t.Errorf("负控③失败：没裁却写了「已裁 N 条」：%q —— 「余量不是裁了」", w)
		}
	}
	var nmeta map[string]json.RawMessage
	_ = json.Unmarshal(neg["meta"], &nmeta)
	if _, ok := nmeta["how_to_restore"]; ok {
		t.Error("负控⑤失败：没裁却写了 `meta.how_to_restore`（§3.3 三件之三**只在真裁时**才有）")
	}
	if _, ok := meta["how_to_restore"]; !ok {
		t.Error("正控⑤失败：真裁了却没有 `meta.how_to_restore`（§3.3 三件之三没有落点）")
	}

	// 成对负控（**无事**）：这条命令没有任何降级/裁条 ⇒ 逐字 `[]` / `false`（不编造）。
	plain, _ := envelopeDoc(t, "version", "--json", "name")
	if string(plain["warnings"]) != "[]" {
		t.Errorf("负控③失败：无事却 `warnings` = %s（要逐字 []）", string(plain["warnings"]))
	}
	if string(plain["truncated"]) != "false" {
		t.Errorf("负控②失败：无事却 `truncated` = %s（要 false）", string(plain["truncated"]))
	}
	// 判据③正控的另一半：`impact` 默认档里**贵层没跑**是**真事** ⇒ 必须非空（宁少报不猜报）。
	if len(nwarns) == 0 {
		t.Error("判据③：默认档贵层没跑（真事）却 `warnings[]` 空 —— 降级不许静默")
	}
}

// ---- 判据④：不编造信号（字面）---------------------------------------------------------------

func TestEnvelopeTruth_NoFabrication(t *testing.T) {
	// 正控：两枚真值渲染器的**空侧**逐字给出（无事 ⇒ []/false）。
	w, tr := zerg.EnvelopeTruthJSONForTest(nil, false)
	if w != "[]" || tr != "false" {
		t.Errorf("正控失败：空侧渲染 = (%s, %s)，要 ([], false)", w, tr)
	}
	w, tr = zerg.EnvelopeTruthJSONForTest([]string{"真事一条"}, true)
	if w != `["真事一条"]` || tr != "true" {
		t.Errorf("正控失败：有事侧渲染 = (%s, %s)", w, tr)
	}

	// 负控①：空白项**不算信号**（不许拿空格凑一条告警）。
	w, tr = zerg.EnvelopeTruthJSONForTest([]string{"", "   ", "\t\n"}, true)
	if w != "[]" {
		t.Errorf("负控①失败：只有空白的那些竟渲染成 %s（要 []）", w)
	}
	if tr != "true" {
		t.Errorf("负控①失败：`truncated` 被空 warnings 带跑了（要 true）：%s", tr)
	}
	// 负控②：合成包封里塞空白告警 ⇒ 输出仍逐字 `"warnings":[]`（与无事那一跑**逐字节同形**）。
	blank := zerg.EnvelopeRenderForTest("[]", 0, []string{"", "  "}, false, nil)
	if !strings.Contains(blank, `"warnings":[]`) {
		t.Errorf("负控②失败：空白告警进了包封：%s", blank)
	}
	if blank != zerg.EnvelopeRenderForTest("[]", 0, nil, false, nil) {
		t.Errorf("负控②失败：带空白告警与不带告警的包封**不同形**（空白被当信号了）")
	}

	// 负控③（静态）：实现件（代码行）里 0 处信号词 —— 包封只搬运真值，不许自己造词。
	src, err := zerg.MainSourceForTest()
	if err != nil {
		t.Fatalf("读包封实现件失败：%v", err)
	}
	if err := judgeEnvelopeWarningsNotFabricated(stripComments(src)); err != nil {
		t.Errorf("负控③失败：%v", err)
	}
	// 负控③′：判定口本身有区分度 —— 一段**会造词**的合成源码喂进去必红。
	if err := judgeEnvelopeWarningsNotFabricated(`warns = append(warns, "已裁 3 条")`); err == nil {
		t.Error("负控③′失败：会造词的合成源码竟判过（判定口是空的）")
	}
	// 负控③″：注释里出现信号词**不算**（判据只看代码行 —— 注释是记录，不是实现）。
	if err := judgeEnvelopeWarningsNotFabricated(stripComments("// 注释里写了「未跑」这两个字\nx := 1\n")); err != nil {
		t.Errorf("负控③″失败：注释里的词被当成实现：%v", err)
	}
}

// ---- 判据⑤：meta 按需子键 + 旧子键语义不变 --------------------------------------------------

func TestEnvelopeTruth_MetaSubkeysAndOldKeys(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// 正控：`meta` 的三个**旧子键**取法一字未动（count == items 条数 · source 非空 · changed 布尔字面量）。
	doc, _ := envelopeDoc(t, "impact", "core/cmd/zerg/main.go", "--json", "what,why")
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(doc["meta"], &meta); err != nil {
		t.Fatalf("`meta` 不是对象：%v", err)
	}
	var items []map[string]string
	if err := json.Unmarshal(doc["items"], &items); err != nil {
		t.Fatalf("`items` 不是对象数组：%v", err)
	}
	var count int
	if err := json.Unmarshal(meta["count"], &count); err != nil {
		t.Fatalf("`meta.count` 不是整数：%v", err)
	}
	if count != len(items) {
		t.Errorf("旧子键语义破了：`meta.count` = %d，`items` = %d 条", count, len(items))
	}
	var src string
	_ = json.Unmarshal(meta["source"], &src)
	if strings.TrimSpace(src) == "" {
		t.Error("旧子键语义破了：`meta.source` 空")
	}
	if string(meta["changed"]) != "false" {
		t.Errorf("旧子键语义破了：只读命令的 `meta.changed` = %s（要 false）", string(meta["changed"]))
	}
	// 新增子键**逐条**在册（本批两处 + 干跑那一处见下一条）：`layers_not_run[]` / `how_to_restore`。
	for _, k := range []string{"layers_not_run", "how_to_restore"} {
		if _, ok := meta[k]; !ok {
			t.Errorf("本批新增的 `meta.%s` 不在（真值没落进去）", k)
		}
	}
	// 负控①：同名**旧子键**不许从按需口覆盖 —— 塞五个旧子键名 ⇒ 值仍是旧取法、且各只出现一次。
	over := zerg.EnvelopeRenderForTest("[{\"a\":\"1\"}]", 7, nil, false, [][2]string{
		{"count", "999"}, {"source", "假源"}, {"changed", "true"},
		{"node", "假节点"}, {"idempotency_key", "假键"},
	})
	if !strings.Contains(over, `"count":7`) || strings.Contains(over, `999`) {
		t.Errorf("负控①失败：`meta.count` 被按需口覆盖了：%s", over)
	}
	if strings.Contains(over, "假源") || strings.Contains(over, "假节点") || strings.Contains(over, "假键") {
		t.Errorf("负控①失败：旧子键被按需口写进去了：%s", over)
	}
	for _, k := range zerg.EnvelopeMetaReservedForTest() {
		if n := strings.Count(over, `"`+k+`"`); n > 1 {
			t.Errorf("负控①失败：`%s` 出现 %d 次（旧子键不许有第二份）", k, n)
		}
	}
	// 负控②：空值**不写**（缺席 ≠ 空值）—— 空串 / 空白都不许造出一格。
	empty := zerg.EnvelopeRenderForTest("[]", 0, nil, false, [][2]string{{"how_to_restore", ""}, {"dry_run", "  "}})
	if strings.Contains(empty, "how_to_restore") || strings.Contains(empty, "dry_run") {
		t.Errorf("负控②失败：空值被写进 `meta`：%s", empty)
	}
	// 负控③：就算按需口塞满了，**顶层仍是六键**（meta 里加子键 ≠ 加键）。
	var edoc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(over), &edoc); err != nil {
		t.Fatalf("合成包封解析失败：%v", err)
	}
	if err := judgeEnvelopeSixKeys(edoc); err != nil {
		t.Errorf("负控③失败：%v", err)
	}
}

// ---- 判据⑥：契约先于实现（正文逐条列举）-----------------------------------------------------

func TestEnvelopeTruth_ContractEnumeratesEvents(t *testing.T) {
	help := zerg.ContractHelpForTest()
	if err := judgeContractEnumeratesWarnings(help); err != nil {
		t.Errorf("判据⑥失败：%v", err)
	}
	// 负控：抽掉一条列举 ⇒ 判定口必红（证明这条判据不是「只要正文里有字就算过」）。
	if err := judgeContractEnumeratesWarnings(strings.Replace(help, "② 层/面没跑", "（抽掉）", 1)); err == nil {
		t.Error("负控失败：契约里少了一条列举竟判过")
	}
	if err := judgeContractEnumeratesWarnings(strings.ReplaceAll(help, "一个不多一个不少", "不作承诺")); err == nil {
		t.Error("负控失败：契约里没写死「顶层键一个不多一个不少」竟判过")
	}
}

// ---- 判据⑦（回归）：无信号时**逐字节等于改前的包封** ----------------------------------------

// envelopeKeyRe —— `meta.idempotency_key` 的取值（**改前就在**的派生值：由调用面派生 ⇒ 与本次
// 改动无关）。判「与改前逐字同形」时把它归一成一个占位符 —— 归一的只有它的**值**，键名、位置、
// 引号一个都不动。
var envelopeKeyRe = regexp.MustCompile(`"idempotency_key":"[^"]*"`)

func envelopeNormKey(s string) string {
	return envelopeKeyRe.ReplaceAllString(s, `"idempotency_key":"K"`)
}

// TestEnvelopeTruth_OldShapeByteIdentical —— 改前 `emitEnvelopeWith` 在无真值时逐字写的就是下面
// 这一份（六键 + 五个旧子键；`idempotency_key` 是既有派生值，归一后比对）：本判据把那一份**旧形态**
// 写成常量喂进合成判定口 —— 无真值 ⇒ 归一后**逐字节相同**（老字段一个字没动）；有真值 ⇒ 只在
// **新增真值那两格**上不同（成对负控证明这一格真在动）。
func TestEnvelopeTruth_OldShapeByteIdentical(t *testing.T) {
	const oldShape = `{"schema":"zerg/v1","kind":"Probe","items":[{"a":"1"}],"meta":{"count":1,"source":"local（本机）","changed":false,"idempotency_key":"K"},"warnings":[],"truncated":false}` + "\n"
	got := envelopeNormKey(zerg.EnvelopeRenderForTest(`[{"a":"1"}]`, 1, nil, false, nil))
	if got != oldShape {
		t.Errorf("无真值时包封与改前**不同形**：\n实得 %s要得 %s", got, oldShape)
	}
	// 成对负控：给一条真值 ⇒ 只在 warnings/truncated 两格上不同（归一后其余逐字相同）。
	cut := envelopeNormKey(zerg.EnvelopeRenderForTest(`[{"a":"1"}]`, 1, []string{"已裁 1 条"}, true, nil))
	if cut == got {
		t.Error("负控失败：给了真值却与无真值那跑逐字节相同（这一格没接上）")
	}
	wantCut := `{"schema":"zerg/v1","kind":"Probe","items":[{"a":"1"}],"meta":{"count":1,"source":"local（本机）","changed":false,"idempotency_key":"K"},"warnings":["已裁 1 条"],"truncated":true}` + "\n"
	if cut != wantCut {
		t.Errorf("负控失败：有真值那一跑除新增两格外还变了：\n实得 %s要得 %s", cut, wantCut)
	}
}
