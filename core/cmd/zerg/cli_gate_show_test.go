// cli_gate_show_test.go —— `zerg gate show <步名> --json` 的**闭集四格**判据（缺口 `Q-061` / `B-3`）。
//
// 为什么落在这里（不是真二进制那一层）：本面读的是**当前源码**（`RunForTest`）——
// `bin/zerg` 是按批重建的制品，本批**不重建**（换件要另走七步）⇒ 新面的棘轮必须落进程内。
//
// 钉住七件（全走**合成门禁脚本** · 不碰真门禁、不跑任何步骤）：
//
//	① 正控：`--json`（**不给字段**）⇒ rc=0 且 `items[0]` 是**闭集四格**（scope/mode/criterion/log
//	   一个不多一个不少 —— `Q-061` 的可核条件逐字）；
//	② `--json <子集>` ⇒ 只投影点名的那几格（值来自同一份计算，不另算一套）；
//	③ `--json <闭集外的字段>` ⇒ rc=2 + stderr 列四格合法字段（`I5`）；
//	④ 未知名 ⇒ rc=2 + 「最像的合法输入」（**不许静默放过** —— 可核条件的负控）；
//	⑤ 重名（歧义）⇒ rc=2（判据要一个答案，不是一坨）；
//	⑥ **默认面一字不改**（可核条件 ② 的负控）：`gate show <步名>` 的输出 == 脚本自己
//	   `--emit-cmd <步名>` 的**逐字节**输出（`bash -c "$(…)"` 的直取口不许被动）；
//	⑦ **只读**（可核条件 ①）：跑前 / 跑后整个合成仓的逐件 `sha256` + 目录清单**逐字相同**。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// showGateScript —— 合成门禁脚本：认识 `--list` / `--emit-cmd`，其余参数一律 rc=2
// （「命令面的旗标不许漏给脚本」这一条与真脚本同方言）。
const showGateScript = `case "${1:-}" in
  --list)
    echo "── 步骤清单（scope 模式 名称）──"
    echo "  go    empty  gofmt -l core"
    echo "  gates tri    合成步"
    echo "共 2 步"
    exit 0 ;;
  --emit-cmd)
    case "${2:-}" in
      "gofmt -l core") echo "gofmt -l ."; exit 0 ;;
      "合成步") echo "python3 scripts/gates/synthetic.py"; exit 0 ;;
    esac
    printf '✗ 步骤名里没有匹配 %s 的\n' "${2:-}" >&2
    exit 2 ;;
esac
printf '✗ 未知参数: %s\n' "${1:-}" >&2
exit 2
`

// showRepo —— 合成仓：脚本 + 两行 `add_step`（步名逐字 = `合成步` / `gofmt -l core`）。
func showRepo(t *testing.T) string {
	t.Helper()
	root := syntheticRepo(t, showGateScript)
	appendToFile(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		"\nadd_step go \"gofmt -l core\" empty \"${REPO_ROOT}/core\" \"gofmt -l .\"\n"+
			"add_step gates \"合成步\" tri \"${REPO_ROOT}\" \"python3 scripts/gates/synthetic.py\"\n")
	return root
}

// gateShowItems —— 跑一次 `gate show <步名> --json …` 并把 `items[0]` 取回来（键序无关）。
func gateShowItems(t *testing.T, root string, argv ...string) (int, string, string, map[string]string) {
	t.Helper()
	rc, out, errb := runZergRepo(t, root, append([]string{"gate", "show"}, argv...)...)
	if rc != 0 {
		return rc, out, errb, nil
	}
	items := envelopeItems(t, out)
	if len(items) != 1 {
		t.Fatalf("要 1 条 items（这一步的四格），得到 %d 条：%q", len(items), out)
	}
	return rc, out, errb, items[0]
}

func TestGateShowJSON_FourCellsClosedSet(t *testing.T) {
	root := showRepo(t)
	rc, out, errb, row := gateShowItems(t, root, "合成步", "--json")
	if rc != 0 {
		t.Fatalf("`gate show 合成步 --json`（不给字段）⇒ 退 0（闭集全量），得到 %d · stderr=%s", rc, errb)
	}
	// ① 闭集四格：一个不多一个不少。
	want := []string{"scope", "mode", "criterion", "log"}
	got := make([]string, 0, len(row))
	for k := range row {
		got = append(got, k)
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("四格集合 = %v（要 %v —— 闭集，一个不多一个不少）", got, want)
	}
	// scope 逐字来自脚本那行 `add_step`；mode = 四档闭集值 + 档位词。
	if row["scope"] != "gates" {
		t.Errorf("scope = %q（要 gates —— 脚本那一行 add_step 的 scope）", row["scope"])
	}
	if !strings.HasPrefix(row["mode"], "tri（") || !strings.Contains(row["mode"], "阻断") {
		t.Errorf("mode = %q（要 `tri（阻断…）`：闭集值在前 + 档位词 —— `Q-061` 的「mode（阻断/只报告）」）", row["mode"])
	}
	// 判据：模式语义 + 命令串（与 `gate explain` 同一处 `gateStepCriterion`）。
	if !strings.Contains(row["criterion"], "tri 模式") || !strings.Contains(row["criterion"], "python3 scripts/gates/synthetic.py") {
		t.Errorf("criterion = %q（要模式语义 + 命令串）", row["criterion"])
	}
	// 日志路径：序号以脚本自己的 `--list` 为准（合成清单里是第 2 步）。
	if !strings.Contains(row["log"], "<outdir>/02-") || !strings.Contains(row["log"], "序号 2") {
		t.Errorf("log = %q（序号应当来自脚本 --list：合成清单里是第 2 步）", row["log"])
	}
	// 六键包封（顶层形状不许自造第七键）。
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("--json 不是对象：%v", err)
	}
	for _, k := range []string{"schema", "kind", "items", "meta", "warnings", "truncated"} {
		if _, ok := doc[k]; !ok {
			t.Errorf("包封缺键 %q", k)
		}
	}
	var kind string
	_ = json.Unmarshal(doc["kind"], &kind)
	if kind != "GateShow" {
		t.Errorf("kind = %q（要与命令一一对应 · 单数 CamelCase）", kind)
	}
}

// ② `--json <子集>`：只投影点名的字段（同一份值）。
func TestGateShowJSON_SubsetProjects(t *testing.T) {
	root := showRepo(t)
	_, _, errb, sub := gateShowItems(t, root, "合成步", "--json", "scope,mode")
	if len(sub) != 2 || sub["scope"] != "gates" || !strings.HasPrefix(sub["mode"], "tri（") {
		t.Fatalf("子集投影 = %+v（要只有 scope/mode 两格）· stderr=%s", sub, errb)
	}
	_, _, _, full := gateShowItems(t, root, "合成步", "--json")
	if sub["scope"] != full["scope"] || sub["mode"] != full["mode"] {
		t.Errorf("子集与全量的值必须同源：子集 %+v · 全量 %+v", sub, full)
	}
	// 旗标在位置参数之前也给得进（解析器口径：位置参数与旗标互不牵连）。
	_, _, _, pre := gateShowItems(t, root, "--json", "scope", "合成步")
	if pre["scope"] != "gates" {
		t.Errorf("`--json` 写在步名之前也应当成立：%+v", pre)
	}
}

// ③ 闭集外的字段 ⇒ 2 + 列四格合法字段（`I5`）。
func TestGateShowJSON_BadFieldListsLegalFour(t *testing.T) {
	root := showRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "show", "合成步", "--json", "nosuchfield-zz")
	if rc != 2 {
		t.Errorf("闭集外的字段 ⇒ 退 2，得到 %d", rc)
	}
	if !strings.Contains(errb, "scope,mode,criterion,log") {
		t.Errorf("stderr 要列出四格合法字段（自描述面）：%q", errb)
	}
	if strings.Contains(out, "nosuchfield-zz") {
		t.Errorf("用法错那一档 stdout 不许出半份包封：%q", out)
	}
	// `error.kind` 是 AI 自愈的入口：用法错必须是 usage（不许落到「按退码兜底」那条路 ——
	// 实读会变成 kind=failed/exit_code=1，与 rc=2 自相矛盾）。
	var env struct {
		Err map[string]any `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("失败那一档也要是合法包封：%v · %q", err, out)
	}
	if env.Err == nil || env.Err["kind"] != "usage" || env.Err["exit_code"] != float64(2) {
		t.Errorf("闭集外字段的 error 块 = %+v（要 kind=usage · exit_code=2）", env.Err)
	}
}

// ④ 未知名 ⇒ 2 + 「最像的合法输入」（**不许静默放过**）；⑤ 重名 ⇒ 2。
func TestGateShowJSON_UnknownAndAmbiguousAreUsageErrors(t *testing.T) {
	root := showRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "show", "zzz-没有这一步-zz", "--json", "scope")
	if rc != 2 {
		t.Fatalf("未知名 ⇒ 退 2（不许静默放过），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "最像的合法输入: ") {
		t.Errorf("未知名要给「最像的合法输入」（K14 第三件）：%q", errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
		Err   map[string]any      `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("失败那一档也要是合法包封：%v · %q", err, out)
	}
	if len(env.Items) != 0 {
		t.Errorf("执行前判没过 ⇒ items 必须空（不许给半份）：%+v", env.Items)
	}
	if env.Err == nil || env.Err["kind"] != "usage" {
		t.Errorf("error.kind 要是 usage（AI 自愈靠它）：%+v", env.Err)
	}
	// 子串不算命中（`B-4` 的对面）：`合成` 不是逐字名。
	if rc2, _, _ := runZergRepo(t, root, "gate", "show", "合成", "--json"); rc2 != 2 {
		t.Errorf("非逐字名 ⇒ 退 2（**不许**子串匹配给一坨），得到 %d", rc2)
	}
	// 重名 ⇒ 歧义 ⇒ 2。
	ambigu := showRepo(t)
	appendToFile(t, filepath.Join(ambigu, "scripts", "gates", "precommit-gates.sh"),
		"\nadd_step docs \"重名步\" rc \"${REPO_ROOT}\" \"true\"\n"+
			"add_step pub \"重名步\" tri \"${REPO_ROOT}\" \"true\"\n")
	rc3, _, errb3 := runZergRepo(t, ambigu, "gate", "show", "重名步", "--json")
	if rc3 != 2 || !strings.Contains(errb3, "歧义") {
		t.Errorf("重名 ⇒ 退 2 + 明说「歧义」，得到 %d · stderr=%q", rc3, errb3)
	}
	// 缺步名 ⇒ 2（`--json` 那一路也不许把缺目标当成功）。
	if rc4, _, _ := runZergRepo(t, root, "gate", "show", "--json"); rc4 != 2 {
		t.Errorf("缺步名 ⇒ 退 2，得到 %d", rc4)
	}
}

// ⑥ 默认面一字不改（可核条件 ② 的负控）：`gate show <步名>` == 脚本自己 `--emit-cmd <步名>` 逐字节。
func TestGateShowDefaultFaceIsVerbatimEmitCmd(t *testing.T) {
	root := showRepo(t)
	script := filepath.Join(root, "scripts", "gates", "precommit-gates.sh")
	for _, name := range []string{"合成步", "gofmt -l core"} {
		direct, err := exec.Command("bash", script, "--emit-cmd", name).Output()
		if err != nil {
			t.Fatalf("脚本直跑 --emit-cmd %q 失败：%v", name, err)
		}
		rc, out, errb := runZergRepo(t, root, "gate", "show", name)
		if rc != 0 {
			t.Fatalf("`gate show %q` ⇒ 0，得到 %d · stderr=%s", name, rc, errb)
		}
		if out != string(direct) {
			t.Errorf("默认面与脚本 `--emit-cmd` **逐字节不同**（直取口被动了）：\n命令面=%q\n脚本  =%q", out, string(direct))
		}
	}
	// 未知名那一档：默认面退码由脚本定（合成脚本 ⇒ 2），命令面不翻译。
	rc, _, _ := runZergRepo(t, root, "gate", "show", "zzz-没有这一步-zz")
	if rc != 2 {
		t.Errorf("未知名（默认面）⇒ 脚本退 2 ⇒ 命令面退 2，得到 %d", rc)
	}
	// `--json` 那一跑不改默认面：同一句再跑一次仍逐字节相同。
	before, _ := exec.Command("bash", script, "--emit-cmd", "合成步").Output()
	_, _, _, _ = gateShowItems(t, root, "合成步", "--json")
	after, _ := exec.Command("bash", script, "--emit-cmd", "合成步").Output()
	if string(before) != string(after) {
		t.Errorf("`--json` 那一跑改动了 `--emit-cmd` 的输出：%q → %q", string(before), string(after))
	}
}

// ⑦ 只读（可核条件 ①）：跑前 / 跑后整个合成仓逐件 sha256 + 目录清单逐字相同。
func TestGateShowJSON_IsReadOnly(t *testing.T) {
	root := showRepo(t)
	before := treeFingerprint(t, root)
	for _, argv := range [][]string{
		{"gate", "show", "合成步", "--json"},
		{"gate", "show", "合成步"},
		{"gate", "show", "zzz-没有这一步-zz", "--json", "scope"},
	} {
		_, _, _ = runZergRepo(t, root, argv...)
	}
	if after := treeFingerprint(t, root); after != before {
		t.Errorf("只读面跑完仓内件变了（`git status --porcelain` 那一条口径的进程内版）：\n前=%s\n后=%s", before, after)
	}
}

// treeFingerprint —— 合成仓的逐件 sha256 + 目录清单（跑前/跑后对拍用；不碰 git）。
func treeFingerprint(t *testing.T, root string) string {
	t.Helper()
	entries := []string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			entries = append(entries, "d "+rel)
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(b)
		entries = append(entries, fmt.Sprintf("f %s %s %d", rel, hex.EncodeToString(sum[:]), len(b)))
		return nil
	})
	if err != nil {
		t.Fatalf("扫合成仓失败：%v", err)
	}
	sort.Strings(entries)
	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(sum[:])
}
