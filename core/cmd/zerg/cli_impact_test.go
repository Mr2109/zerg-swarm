// cli_impact_test.go —— `zerg impact` 的**`A1` 骨架**判据（任务单-影响面实施-20260922 §二 `A1` 判据 ①–③）。
//
// 落点在 `package main_test`（§九 M17「三层测试落点」的层①②）：`RunForTest` 直接跑一条命令，
// 判退码 + 包封形状 + 人面三行 —— 读的是**当前源码**的运行期行为，不是盘上旧制品。
//
// 成对负控（防自欺条款 · 本文件里真跑）：
//
//	· 判据① 的判定口 = `judgeImpactEnvelope`；喂「少一键」与「多一键」两枚错期望 ⇒ **必须报错**；
//	· 判据② 的负控 = **目标解析不到 ⇒ 退 2**（不是 1）—— 证明这条命令不是「恒返回零命中」；
//	· 判据③ 的负控 = 把人面三行**打乱顺序**的夹具喂进判定口 ⇒ **必须报错**。
package main_test

import (
	"encoding/json"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// impactEnvelopeKeys —— §九 M6 的六键（判据①：**六键恒在**）。
// ★ 本表与真源**现跑对拍**（`TestImpact_CommandTreeAndEnvelopeSource`）—— 不另造第二套六键。
var impactEnvelopeKeys = []string{"schema", "kind", "items", "meta", "warnings", "truncated"}

// TestImpact_CommandTreeAndEnvelopeSource —— 契约面两处**现跑对拍**（防「测试另抄一份」）：
//
//	① 命令树里真登记了这条命令：只读（`danger` 空）· 字段表 = §3.1 的四字段；
//	② 本文件的两份期望值（六键字头）与桥开出来的真源**逐字相同**。
func TestImpact_CommandTreeAndEnvelopeSource(t *testing.T) {
	info, ok := zerg.CommandInfoOfForTest("impact")
	if !ok {
		t.Fatalf("命令树里没有 `impact` 这条命令（注册漏了）")
	}
	if info.DangerLevel != "" {
		t.Errorf("`impact` 是危险动作（%s）—— A1 是**只读**骨架（§7.1「只读、零副作用」）", info.DangerLevel)
	}
	if strings.Join(info.Fields, ",") != "what,why,how,red" {
		t.Errorf("字段表 = %v（要 §3.1 的四字段 what/why/how/red）", info.Fields)
	}
	got := strings.Join(zerg.EnvelopeKeysForTest(), ",")
	if got != strings.Join(impactEnvelopeKeys, ",") {
		t.Errorf("六键对拍不一致：真源 %q · 本文件 %q（本文件不许另造第二套）", got, strings.Join(impactEnvelopeKeys, ","))
	}
	if zerg.ContractSchemaForTest() != "zerg/v1" {
		t.Errorf("契约主号 = %q（要 zerg/v1）", zerg.ContractSchemaForTest())
	}
}

// judgeImpactEnvelope —— 判据①②的**唯一判定口**（抽出来的目的：让负控能直接喂坏期望）。
// 判三件：六键**一个不少**、**一个不多**、`items` 逐字是 `[]`（恒数组 · 空为 `[]` · 永不为 `null`）。
func judgeImpactEnvelope(doc map[string]json.RawMessage, itemsWant string) error {
	for _, k := range impactEnvelopeKeys {
		if _, ok := doc[k]; !ok {
			return errStr("包封缺键 " + k)
		}
	}
	for k := range doc {
		found := false
		for _, want := range impactEnvelopeKeys {
			if k == want {
				found = true
			}
		}
		if !found {
			return errStr("包封多了未声明键 " + k)
		}
	}
	if got := strings.TrimSpace(string(doc["items"])); got != itemsWant {
		return errStr("items = " + got + "（要 " + itemsWant + "）")
	}
	return nil
}

type errStr string

func (e errStr) Error() string { return string(e) }

// judgeImpactHuman —— 判据③的判定口：人面**恒三行**、字头与顺序固定（`会牵动：` / `会红：` / `建议：`）。
func judgeImpactHuman(stdout string, heads [3]string) error {
	lines := []string{}
	for _, ln := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		if strings.TrimSpace(ln) != "" {
			lines = append(lines, ln)
		}
	}
	if len(lines) != 3 {
		return errStr("人面不是三行（实得 N 行）")
	}
	for i, h := range heads {
		if !strings.HasPrefix(lines[i], h) {
			return errStr("第 " + string(rune('1'+i)) + " 行的字头不是 " + h)
		}
	}
	return nil
}

// impactHeads —— 判据③ 的字头（与实现同一份取值；测试**不另造**第二套字头）。
var impactHeads = [3]string{"会牵动：", "会红：", "建议："}

// TestImpact_MachineFaceSixKeysAndEmptyItems —— 判据① + 判据②（正控）：
// `A2` 取数接上之后，**零命中**要挑一件三层都空的件（① 反向包 0 · ③ 契约 0 · ④ 词法/形近 0）——
// 那一件见 `impactZeroHitTarget()`（**件名不许逐字写出**：写出来它就命中词法面）：
// ⇒ `--json` 出**六键**、`items` 逐字 `[]`、退码 **1**（零命中 · 不是 0、不是失败）。
func TestImpact_MachineFaceSixKeysAndEmptyItems(t *testing.T) {
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))
	tgt := impactZeroHitTarget()

	rc, out, errb := runCapture("impact", tgt, "--json", "what,why,how,red")
	if rc != 1 {
		t.Fatalf("目标有效但零命中 ⇒ 退码 1（§7.5「无影响面」· 不是 0、不是失败），得到 %d · stderr=%s", rc, errb)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("--json 输出不是对象：%v · out=%q", err, out)
	}
	if err := judgeImpactEnvelope(doc, "[]"); err != nil {
		t.Errorf("判据①/② 破：%v", err)
	}
	var kind, schema string
	_ = json.Unmarshal(doc["kind"], &kind)
	_ = json.Unmarshal(doc["schema"], &schema)
	if kind != "Impact" {
		t.Errorf("kind = %q（要单数 CamelCase `Impact` · §九 M6 I2）", kind)
	}
	if schema != "zerg/v1" {
		t.Errorf("schema = %q（要 zerg/v1）", schema)
	}

	// 判据③（正控）：不给 `--json` ⇒ 人面恒三行。
	rc, out, _ = runCapture("impact", tgt)
	if rc != 1 {
		t.Errorf("人面零命中 ⇒ 退码 1，得到 %d", rc)
	}
	if err := judgeImpactHuman(out, impactHeads); err != nil {
		t.Errorf("判据③ 破：%v · stdout=%q", err, out)
	}

	// 反面对照（`A2` 的正控）：有影响面的件 ⇒ 退码 **0**（不是 1）· 人面仍恒三行。
	rc, out, errb = runCapture("impact", "core/cmd/zerg/main.go")
	if rc != 0 {
		t.Errorf("有影响面的件 ⇒ 退码 0，得到 %d · stderr=%s", rc, errb)
	}
	if err := judgeImpactHuman(out, impactHeads); err != nil {
		t.Errorf("有影响面时人面也应恒三行：%v · stdout=%q", err, out)
	}

	// 契约 id 目标（第二态）也走同一条路（在册 id 现读自 registry.json）。
	rc, out, errb = runCapture("impact", "S-g", "--json", "what")
	if rc != 0 {
		t.Errorf("在册契约 id 目标（`S-g` 指向命令面目录）⇒ 退码 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("契约目标的机器面不是包封：%q", out)
	}
}

// TestImpact_NegativeControls —— 成对负控（全都**真跑**）：
//
//	① 判定口能把红判出来（少一键 / 多一键 / `items` 为 `null` 三枚错期望）；
//	② 目标解析不到 / 出仓 / 缺目标 ⇒ **退 2**（不是 1 —— 这条命令不是「恒返回零命中」）；
//	③ 人面三行**顺序**被打乱 ⇒ 判定口必红；
//	④ K2：给了 `--json` 不给字段 ⇒ 退 1 + stdout **0 字节**。
func TestImpact_NegativeControls(t *testing.T) {
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))

	// ① 判定口的负控：三枚坏期望，逐枚**必须**报错。
	rc, out, _ := runCapture("impact", impactZeroHitTarget(), "--json", "what")
	if rc != 1 {
		t.Fatalf("准备态不对：正控退码 = %d（要 1 —— 零命中件见 impactZeroHitTarget）", rc)
	}
	var good map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &good); err != nil {
		t.Fatalf("正控输出不是对象：%v", err)
	}
	if err := judgeImpactEnvelope(good, "[]"); err != nil {
		t.Fatalf("负控第 0 步就不成立：真值都判不过（%v）", err)
	}
	fewer := map[string]json.RawMessage{}
	for k, v := range good {
		if k != "warnings" {
			fewer[k] = v
		}
	}
	if err := judgeImpactEnvelope(fewer, "[]"); err == nil {
		t.Error("负控①失败：少一键（warnings）竟判过 —— 这台机器判不出红")
	}
	more := map[string]json.RawMessage{}
	for k, v := range good {
		more[k] = v
	}
	more["error"] = json.RawMessage(`{}`)
	if err := judgeImpactEnvelope(more, "[]"); err == nil {
		t.Error("负控①失败：**第七键**（error）竟判过 —— 「不新增第七键」这条红线就靠这一格守")
	}
	if err := judgeImpactEnvelope(good, "null"); err == nil {
		t.Error("负控①失败：`items` 期望写成 null 竟判过 —— 「永不为 null」这一格没牙")
	}

	// ② 退码成对：目标不合格 ⇒ 2（不给结论），**不许**与「零命中 1」混。
	for _, argv := range [][]string{
		{"impact"},
		{"impact", "zzz/never/there.go"},
		{"impact", "S-zz"},
		{"impact", "/tmp"},
		{"impact", "../Zerg-内部文档/项目文档"},
		{"impact", "core/cmd/zerg/main.go", "--nosuchflag-zz"},
	} {
		rc, _, _ := runCapture(argv...)
		if rc != 2 {
			t.Errorf("%v ⇒ 要退 2（用法错/目标解析不到/未知旗标），得到 %d", argv, rc)
		}
	}

	// ③ 人面顺序的负控：把三行对调 ⇒ 判定口必红。
	shuffled := impactHeads[2] + "x\n" + impactHeads[0] + "y\n" + impactHeads[1] + "z\n"
	if err := judgeImpactHuman(shuffled, impactHeads); err == nil {
		t.Error("负控③失败：人面三行顺序被对调竟判过 —— 判据③「顺序固定」这一格没牙")
	}

	// ④ K2：给了 `--json` 不给字段 ⇒ 退 1 + stdout 0 字节（与既有各命令同一条纪律）。
	rc, out, errb := runCapture("impact", "core/cmd/zerg/main.go", "--json")
	if rc != 1 || len(out) != 0 {
		t.Errorf("K2：`--json` 不给字段 ⇒ 退 1 + stdout 0 字节，实得 rc=%d · %d 字节 · stderr=%s",
			rc, len(out), errb)
	}
	if !strings.Contains(errb, "可选字段") {
		t.Errorf("K2：字段清单没走 stderr：%q", errb)
	}
}
