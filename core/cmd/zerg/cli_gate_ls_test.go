// cli_gate_ls_test.go —— `zerg gate ls --json [<字段>]` 的**字段面**判据（缺口 `GAP-20261005-65`）。
//
// 病（改前 · 现读）：`--json <字段>` 的**实参被整枚吞掉** —— 既不校名也不投影：
// `gate ls --json zzz` 与裸 `--json` 逐字节相同、静默 `rc=0`（fail-open，同族 `gate show`/`gate find`
// 一律 `rc=2` + 「合法字段: …」）。
//
// 钉三件（全走**合成门禁脚本** · 不碰真门禁、不跑任何步骤）：
//
//	① `--json <闭集外>` ⇒ `rc=2` + stderr 逐字「未知字段 + 合法字段: total,scope,mode,step」（同族出口
//	   `reportBadField` 的原话 · 复用不新造）；
//	② `--json total` ⇒ 只出顶层那一格；`--json scope,mode` ⇒ 行内只留点名的键、且不点名 `total` 就不出它；
//	③ 负控：裸 `--json`（不给字段）= 闭集全量 `{steps,total}`，一个字节不改。
//
// 为什么落在进程内（不是真二进制那一层）：`bin/zerg` 是按批重建的制品，本批**不重建**（换件另走七步）
// ⇒ 新面的棘轮必须落进程内（与 `cli_gate_show_test.go` 同规）。
package main_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// lsGateScript —— 合成门禁脚本：只认 `--list`（两行清单 + 「共 2 步」），其余参数一律 rc=2。
const lsGateScript = `case "${1:-}" in
  --list)
    echo "── 步骤清单（scope 模式 名称）──"
    echo "  go    empty  gofmt -l core"
    echo "  gates tri    合成步"
    echo "共 2 步"
    exit 0 ;;
esac
printf '✗ 未知参数: %s\n' "${1:-}" >&2
exit 2
`

func lsRepo(t *testing.T) string {
	t.Helper()
	return syntheticRepo(t, lsGateScript)
}

// ① 缺口复现：闭集外的字段**不许被吞** —— rc=2 + 同族出口的「合法字段:」清单。
func TestGateLsJSON_BadFieldRejected(t *testing.T) {
	root := lsRepo(t)
	rc, _, errb := runZergRepo(t, root, "gate", "ls", "--json", "zzz")
	if rc != 2 {
		t.Fatalf("`gate ls --json zzz` ⇒ 退 2（同族 gate show/find），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "未知字段") || !strings.Contains(errb, "合法字段: total,scope,mode,step") {
		t.Errorf("stderr 要「未知字段 + 合法字段: total,scope,mode,step」：%q", errb)
	}
}

// ②a `--json total` ⇒ 只出顶层那一格（闭集里的顶层键）。
func TestGateLsJSON_TotalProjects(t *testing.T) {
	root := lsRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json", "total")
	if rc != 0 {
		t.Fatalf("`gate ls --json total` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var doc map[string]int
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("`--json total` 要是一个 {\"total\":N} 对象：%v · %q", err, out)
	}
	if doc["total"] != 2 {
		t.Errorf("total = %d（合成清单 2 步）· %q", doc["total"], out)
	}
	if strings.Contains(out, "steps") {
		t.Errorf("只点名 total ⇒ 不许出 steps：%q", out)
	}
}

// ②b `--json scope,mode` ⇒ 行内只留点名的键；没点名 total ⇒ 不出 total。
func TestGateLsJSON_NestedSubsetProjects(t *testing.T) {
	root := lsRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json", "scope,mode")
	if rc != 0 {
		t.Fatalf("`gate ls --json scope,mode` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var doc struct {
		Total *int                     `json:"total"`
		Steps []map[string]interface{} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("解不动：%v · %q", err, out)
	}
	if doc.Total != nil {
		t.Errorf("没点名 total ⇒ 不许出 total：%q", out)
	}
	if len(doc.Steps) != 2 {
		t.Fatalf("steps 行数 = %d（要 2）· %q", len(doc.Steps), out)
	}
	for i, r := range doc.Steps {
		if len(r) != 2 || r["scope"] == nil || r["mode"] == nil {
			t.Errorf("第 %d 行 = %+v（只许 scope/mode 两键）", i, r)
		}
		if _, ok := r["step"]; ok {
			t.Errorf("第 %d 行不该有 step 键：%+v", i, r)
		}
	}
}

// ③ 负控：裸 `--json`（不给字段）= 闭集全量 `{steps,total}` —— 默认面一字不改。
func TestGateLsJSON_BareIsFullFace(t *testing.T) {
	root := lsRepo(t)
	rc, bare, errb := runZergRepo(t, root, "gate", "ls", "--json")
	if rc != 0 {
		t.Fatalf("裸 `--json` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var doc struct {
		Total int                      `json:"total"`
		Steps []map[string]interface{} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(bare), &doc); err != nil {
		t.Fatalf("裸 `--json` 要出 {steps,total}：%v · %q", err, bare)
	}
	if doc.Total != 2 || len(doc.Steps) != 2 {
		t.Errorf("裸面 total=%d steps=%d（要 2/2）· %q", doc.Total, len(doc.Steps), bare)
	}
}

// ④ 正控（缺口 `GAP-20260927-226` want①）：`name` 走**可选字段层**（`optFields`）⇒ `--json name` 收下、逐行出步名的第二个键 `name`，值与既有冻结键 `--json step` **同值**（同一步名两种拼法）。
func TestGateLsJSON_NameOptionalFieldProjects(t *testing.T) {
	root := lsRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json", "name")
	if rc != 0 {
		t.Fatalf("`gate ls --json name` ⇒ 退 0（可选字段层收下），得到 %d · stderr=%s", rc, errb)
	}
	var doc struct {
		Steps []map[string]string `json:"steps"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("解不动：%v · %q", err, out)
	}
	if len(doc.Steps) != 2 {
		t.Fatalf("steps 行数 = %d（要 2）· %q", len(doc.Steps), out)
	}
	want := []string{"gofmt -l core", "合成步"}
	for i, r := range doc.Steps {
		if len(r) != 1 {
			t.Errorf("第 %d 行 = %+v（只许 name 一键）", i, r)
		}
		if r["name"] != want[i] {
			t.Errorf("第 %d 行 name = %q（要 %q）", i, r["name"], want[i])
		}
	}
}

// ⑤ 负控甲：`name` 只是**可选层**，不进冻结字段表 —— 闭集外字段的报话仍是既有的 `合法字段: total,scope,mode,step`，且那行里**不出现** `name`（与 `code find` 的 `count` 同规）。
func TestGateLsJSON_NameNotInFrozenFieldList(t *testing.T) {
	root := lsRepo(t)
	rc, _, errb := runZergRepo(t, root, "gate", "ls", "--json", "totally-bogus")
	if rc != 2 {
		t.Fatalf("`--json <闭集外>` ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "合法字段: total,scope,mode,step") {
		t.Errorf("冻结字段表须逐字不变：%q", errb)
	}
	if strings.Contains(errb, "name") {
		t.Errorf("`合法字段:` 那行不许出现 name（可选层不进冻结表）：%q", errb)
	}
}

// lsGateScriptDup —— 合成门禁脚本：**两步同名**（缺口 `GAP-20260928-68` 的反例面 —— 同名步下
// 一个短 id 会落在两个步上 ⇒ 必须响亮判冲突，✗ 不许静默取首）。
const lsGateScriptDup = `case "${1:-}" in
  --list)
    echo "── 步骤清单（scope 模式 名称）──"
    echo "  go    empty  gofmt -l core"
    echo "  gates tri    gofmt -l core"
    echo "共 2 步"
    exit 0 ;;
esac
printf '✗ 未知参数: %s\n' "${1:-}" >&2
exit 2
`

// ⑦ 缺口 `GAP-20260928-68`：`id` 走**可选字段层** ⇒ `--json id` 每行给该步的**稳定短 id**
// （`s` ＋ sha256(归一后步名) 前 4 字节 hex · 8 个十六进制字符）。步名本身是人读串（含空格/CJK/反引号），
// 而 `gate run --step`/`--fast`/脚本 `filter_step` 一律按名逐字精确匹配 ⇒ 工具与脚本没有短而稳的句柄。
func TestGateLsJSON_IDOptionalFieldProjects(t *testing.T) {
	root := lsRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json", "id")
	if rc != 0 {
		t.Fatalf("`gate ls --json id` ⇒ 退 0（可选字段层收下），得到 %d · stderr=%s", rc, errb)
	}
	var doc struct {
		Steps []map[string]string `json:"steps"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("解不动：%v · %q", err, out)
	}
	if len(doc.Steps) != 2 {
		t.Fatalf("steps 行数 = %d（要 2）· %q", len(doc.Steps), out)
	}
	seen := map[string]bool{}
	for i, r := range doc.Steps {
		if len(r) != 1 {
			t.Errorf("第 %d 行 = %+v（只许 id 一键）", i, r)
		}
		id := r["id"]
		if !regexp.MustCompile(`^s[0-9a-f]{8}$`).MatchString(id) {
			t.Errorf("第 %d 行 id = %q（要 `s` + 8 位十六进制）", i, id)
		}
		if seen[id] {
			t.Errorf("第 %d 行 id = %q 与前面的步撞了（异名步不该撞）", i, id)
		}
		seen[id] = true
	}
}

// ⑧ 负控：`id` 只是**可选层** —— 闭集外字段的报话仍是既有的「合法字段: total,scope,mode,step」且那行里
// **不出现** `id`；裸 `--json`（不点名）逐行也**不长** `id` 键（默认面一字不改）。
func TestGateLsJSON_IDNotInFrozenFace(t *testing.T) {
	root := lsRepo(t)
	rc, _, errb := runZergRepo(t, root, "gate", "ls", "--json", "totally-bogus")
	if rc != 2 {
		t.Fatalf("`--json <闭集外>` ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "合法字段: total,scope,mode,step") {
		t.Errorf("冻结字段表须逐字不变：%q", errb)
	}
	if strings.Contains(errb, "id") {
		t.Errorf("`合法字段:` 那行不许出现 id（可选层不进冻结表）：%q", errb)
	}
	rc2, bare, errb2 := runZergRepo(t, root, "gate", "ls", "--json")
	if rc2 != 0 {
		t.Fatalf("裸 `--json` ⇒ 退 0，得到 %d · stderr=%s", rc2, errb2)
	}
	var doc struct {
		Steps []map[string]interface{} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(bare), &doc); err != nil {
		t.Fatalf("解不动：%v · %q", err, bare)
	}
	for i, r := range doc.Steps {
		if _, ok := r["id"]; ok {
			t.Errorf("第 %d 行不该有 id 键（裸面＝冻结全量）：%+v", i, r)
		}
	}
}

// ⑨ 反例（缺口 `GAP-20260928-68`）：**同名步** ⇒ 一个短 id 落在两个步上 ⇒ 判不了、**响亮报冲突**
// （退 2 · 逐条点名），✗ **不静默取首**（也不许吐一坨 id 当答案）。
func TestGateLsJSON_IDAmbiguousStepLoud(t *testing.T) {
	root := syntheticRepo(t, lsGateScriptDup)
	rc, out, errb := runZergRepo(t, root, "gate", "ls", "--json", "id")
	if rc != 2 {
		t.Fatalf("同名步 ⇒ 退 2（不给结论），得到 %d · stdout=%q · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "撞车") {
		t.Errorf("stderr 要说清是短 id 撞车：%q", errb)
	}
	if !strings.Contains(errb, "gofmt -l core") {
		t.Errorf("撞车那条要点名撞在一起的步名：%q", errb)
	}
	if strings.Contains(out, "\"id\"") {
		t.Errorf("判不了时不许吐 id（✗ 静默取首）：%q", out)
	}
}

// ⑥ 负控乙：裸 `--json`（不给字段）＝冻结全量 `{steps,total}` —— 逐行**不许**长出 `name` 键（默认面一字不改；want 形状的 `name` 只在显式点名时落）。
func TestGateLsJSON_BareHasNoNameKey(t *testing.T) {
	root := lsRepo(t)
	rc, bare, errb := runZergRepo(t, root, "gate", "ls", "--json")
	if rc != 0 {
		t.Fatalf("裸 `--json` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var doc struct {
		Steps []map[string]interface{} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(bare), &doc); err != nil {
		t.Fatalf("解不动：%v · %q", err, bare)
	}
	for i, r := range doc.Steps {
		if _, ok := r["name"]; ok {
			t.Errorf("第 %d 行不该有 name 键（裸面＝冻结全量）：%+v", i, r)
		}
	}
}
