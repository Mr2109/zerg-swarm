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
	"os/exec"
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

// ── 缺口 `GAP-20260929-22`：`gate ls` 的**可直接粘贴命令**表 ＋ `--step` 未命中提示（成对自检）──
//
// 两态都读在**源码编出的测试进程**里（`runZergRepo` ⇒ `zerg.RunForTest`）：本单禁重编 `bin/zerg`
// ⇒ 「改后」读数是**解析层**（源码面），✗ 不是制品读数。
//
// 合成门禁脚本一次备两样：① `--list` 出口（供 `gate ls` 出清单）；② 一条 `add_step` 声明
// （供 `parseAddSteps` 读步名 —— `gate run --step … --dry-run` 只读脚本、**不起任何子进程**）。

// glqStepName —— 合成步名：空白 ＋ 单引号 ＋ 反引号（三种「该怎么引用」的坑一次全带）。
const glqStepName = "合 成'步`x`"

// glqListBody —— `--list` 的**人面正文**（落成一件，脚本 `cat` 它 ⇒ 反引号/单引号不经 shell 展开）。
const glqListBody = "── 步骤清单（scope 模式 名称）──\n" +
	"   go    empty  gofmt -l core\n" +
	"   gates tri    " + glqStepName + "\n" +
	"共 2 步\n"

// glqGateBody —— 合成门禁脚本：`--list` 出口 ＋ 一条 `add_step` 声明（声明放在 `exit 2` 之后 =
// **不可达** ⇒ 脚本跑起来不会去执行它，解析器照样读得到）。
const glqGateBody = "case \"${1:-}\" in\n" +
	"  --list) cat \"$(dirname \"$0\")/.glq-list\"; exit 0 ;;\n" +
	"esac\n" +
	"printf '✗ 未知参数: %s\\n' \"${1:-}\" >&2\n" +
	"exit 2\n" +
	"add_step gates \"" + glqStepName + "\" tri \".\" \"true\"\n"

func glqRepo(t *testing.T) string {
	t.Helper()
	root := syntheticRepo(t, glqGateBody)
	mustWrite(t, root+"/scripts/gates/.glq-list", glqListBody)
	return root
}

// glqQuotedFromFooter —— 从 `gate ls` 的 stderr 追加段里**逐字**取出那一步的引号形态
// （= 调用者照抄时要搬走的那个字节串）。取不到 ⇒ 停手（不许拿别的形态顶替）。
func glqQuotedFromFooter(t *testing.T, errb string) string {
	t.Helper()
	const head = "gate run --step "
	const tail = " --only-step"
	for _, ln := range strings.Split(errb, "\n") {
		i := strings.Index(ln, head)
		if i < 0 {
			continue
		}
		rest := ln[i+len(head):]
		j := strings.LastIndex(rest, tail)
		if j < 0 || !strings.Contains(rest[:j], "合 成") {
			continue
		}
		return rest[:j]
	}
	t.Fatalf("`gate ls` 的粘贴表里取不到那一步的引号形态：%q", errb)
	return ""
}

// glqShellUnquote —— 让 **shell** 按它的规矩把引号形态还原成 argv 里的那一个串
// （= 粘贴那一刻 shell 实际交给程序的字节 · 这一步**就是**「粘贴」本身）。
func glqShellUnquote(t *testing.T, quoted string) string {
	t.Helper()
	out, err := exec.Command("bash", "-c", "printf '%s' "+quoted).Output()
	if err != nil {
		t.Fatalf("shell 解不开这段引号形态 %q：%v", quoted, err)
	}
	return string(out)
}

// ① 表在：`gate ls`（默认面）⇒ stdout **逐字不变**（仍是脚本 `--list`），stderr 追加
// 「可直接粘贴的整条命令」，且步名按「闭引号 ＋ 反斜杠单引号 ＋ 重开引号」的写法闭合（含单引号的合成步名）。
func TestGateLsPasteFooterQuotesStepName(t *testing.T) {
	root := glqRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "ls")
	if rc != 0 {
		t.Fatalf("`gate ls` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, glqStepName) {
		t.Fatalf("stdout 该逐字带出脚本 `--list` 的步名：%q", out)
	}
	if strings.Contains(out, "可直接粘贴的整条命令") {
		t.Fatalf("✗ 粘贴表不许进 stdout（stdout 是脚本 `--list` 的逐字节转出）：%q", out)
	}
	if !strings.Contains(errb, "可直接粘贴的整条命令") {
		t.Fatalf("stderr 该带「可直接粘贴的整条命令」表：%q", errb)
	}
	quoted := glqQuotedFromFooter(t, errb)
	if !strings.Contains(quoted, `'\''`) {
		t.Fatalf("含单引号的步名该按 `'\\''` 闭合，实得 %q", quoted)
	}
	if want := "'合 成'\\''步`x`'"; quoted != want {
		t.Fatalf("引号形态 = %q（要 %q）", quoted, want)
	}
}

// ② 粘贴即解析到该步（正控）：把表里那一条**照抄**过 shell 解引号 ⇒ `--step` **命中**该步。
// `--dry-run` 是只读形态（零子进程、不跑任何步骤）—— 本单 ✗ 不真跑门。
func TestGateLsPasteCmdResolvesStep(t *testing.T) {
	root := glqRepo(t)
	rc, _, errb := runZergRepo(t, root, "gate", "ls")
	if rc != 0 {
		t.Fatalf("`gate ls` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	quoted := glqQuotedFromFooter(t, errb)
	name := glqShellUnquote(t, quoted)
	if name != glqStepName {
		t.Fatalf("shell 按表里的引号形态还原出 %q（要 %q）⇒ 该形态没做到逐字", name, glqStepName)
	}
	rc2, out2, errb2 := runZergRepo(t, root, "gate", "run", "--step", name, "--only-step", "--dry-run")
	if rc2 != 0 {
		t.Fatalf("粘贴表里那条命令 ⇒ 该解析到该步（退 0），得到 %d · stdout=%q · stderr=%s", rc2, out2, errb2)
	}
	if !strings.Contains(out2+errb2, glqStepName) {
		t.Fatalf("解析到的那一步该是 %q：stdout=%q stderr=%q", glqStepName, out2, errb2)
	}
}

// ③ 成对自检 —— 未命中 ⇒ 提示「可能是引号/展开所致，试试单引号」出现；命中 ⇒ **不出现**。
func TestGateStepMissHintPaired(t *testing.T) {
	root := glqRepo(t)
	rcMiss, _, errMiss := runZergRepo(t, root, "gate", "run", "--step", "合 成'步", "--only-step", "--dry-run")
	if rcMiss != 2 {
		t.Fatalf("未命中 ⇒ 退 2，得到 %d · stderr=%s", rcMiss, errMiss)
	}
	if !strings.Contains(errMiss, "可能是引号/展开所致") {
		t.Fatalf("未命中该点明「可能是引号/展开所致」：%q", errMiss)
	}
	if !strings.Contains(errMiss, "单引号") {
		t.Fatalf("未命中该提示「试试单引号」：%q", errMiss)
	}
	rcHit, _, errHit := runZergRepo(t, root, "gate", "run", "--step", glqStepName, "--only-step", "--dry-run")
	if rcHit != 0 {
		t.Fatalf("命中 ⇒ 退 0，得到 %d · stderr=%s", rcHit, errHit)
	}
	if strings.Contains(errHit, "可能是引号/展开所致") {
		t.Fatalf("命中时**不许**出现未命中提示：%q", errHit)
	}
}
