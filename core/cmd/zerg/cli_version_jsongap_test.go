// cli_version_jsongap_test.go —— `version` 的 `--json` 用法错**两态**（缺口 `Q-146` · 组「退码与自家表不同向」）。
//
// 判据（缺口账 `项目文档/v2.5.12/缺口总账-20260923.md` §附录三十三 `Q-146` 治② ·
// 退码表 `core/cmd/zerg/exitcodes.go:33`＝`{2, "usage", "用法错 / **不给结论**", …}`）：
//
//	① 坏用：`zerg version --json`（**不给字段**）⇒ 退 **2**（usage）· stdout **0 字节** ·
//	   stderr 首行逐字 `zerg: --json 需要逗号分隔的字段列表` ＋ 第二行 `可选字段: <该条字段表>`；
//	② 合法：`zerg version --json name,version,commit` ⇒ 退 **0** ＋ 包封（`items[0]` 三格值齐）；
//	③ 对照：`zerg version`（不给 `--json`）⇒ 退 **0**（合法档没被顺手改坏）。
//
// 为什么三格必须成对钉：`Q-146` 的病根是「**同一句用法错的退码与自家退码表不同向**」（改前 ① 退 `1`
// ＝ `exitFail`，而表说用法错 = `2`）—— 只钉①证明不了合法档还在（改宽了也能绿），
// 只钉②证明不了用法错归了位（改前 ② 就是绿的）。
//
// 判据值**不写死**：从退码表真源（`zerg.ExitCodeTableForTest()`）现读 `usage` 那一格 ——
// 表若被改成别的数，这条测试当场说话（「代码合自己的表」这句话才可复算）。
// 字段表同理：从命令树真源（`zerg.CommandInfoOfForTest("version")`）现读，不另抄一份九格清单。
//
// 落点：`package main_test` ＋ `zerg.RunForTest` —— 跑的是**当前源码**的行为（不是 `bin/` 里的旧制品），
// 与 `cli_help_twostate_test.go` / `cli_matrix_test.go` 同一条路；全程只读，不起子进程、不碰状态目录。
package main_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// usageCodeFromTable 现读退码表里 `usage` 那一格（判据值唯一来源＝退码表真源）。
func usageCodeFromTable(t *testing.T) int {
	t.Helper()
	for _, r := range zerg.ExitCodeTableForTest() {
		if r.Name == "usage" {
			return r.Code
		}
	}
	t.Fatal("退码表里没有 `usage` 这一格（表被人改坏了 ⇒ 判据不可判，不给结论）")
	return 0
}

// TestCLIVersionJSONGap_MissingFieldsIsUsageError —— ① 坏用档：`--json` 不给字段 ⇒ 退码表那个 number。
func TestCLIVersionJSONGap_MissingFieldsIsUsageError(t *testing.T) {
	want := usageCodeFromTable(t)
	if want != 2 {
		t.Fatalf("退码表里 `usage` = %d（本测试按 `Q-146` 的口径钉 2 —— 表变了就得同批重贴现读）", want)
	}

	var out, errb bytes.Buffer
	rc := zerg.RunForTest([]string{"version", "--json"}, &out, &errb)

	if rc != want {
		t.Errorf("`zerg version --json`（不给字段）⇒ 应退用法错 %d（退码表 `usage` · `Q-146`），得到 %d", want, rc)
	}
	if out.Len() != 0 {
		t.Errorf("`zerg version --json`（不给字段）⇒ stdout 必须 **0 字节**（提示面走 stderr），得到 %d 字节：%q",
			out.Len(), out.String())
	}

	lines := strings.Split(strings.TrimRight(errb.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("stderr 应有两行（提示 + 可选字段），得到 %d 行：%q", len(lines), errb.String())
	}
	if want1 := "zerg: --json 需要逗号分隔的字段列表"; lines[0] != want1 {
		t.Errorf("stderr 首行应逐字 %q，得到 %q", want1, lines[0])
	}

	info, ok := zerg.CommandInfoOfForTest("version")
	if !ok {
		t.Fatal("命令树里找不到 `version`（判据不可判）")
	}
	if len(info.Fields) == 0 {
		t.Fatal("`version` 的字段表是空的（空转 ⇒ 判据不可判）")
	}
	if want2 := "可选字段: " + strings.Join(info.Fields, ","); lines[1] != want2 {
		t.Errorf("stderr 第二行应是**该条字段表**（真源自命令树）%q，得到 %q", want2, lines[1])
	}
}

// TestCLIVersionJSONGap_LegalFormsStayZero —— ②／③ 合法档对照：给字段 ⇒ 0（真出包封）；不给 `--json` ⇒ 0。
//
// 两格一起钉的理由：`Q-146` 只动「缺字段」那一支的退码 —— 合法档若被顺手改坏（或改成要更多前置），
// 这条会红；反过来只钉坏用档，改宽成「任何 `--json` 都给 2」也能绿。
func TestCLIVersionJSONGap_LegalFormsStayZero(t *testing.T) {
	// ② 给字段 ⇒ 0 + 包封（六键形状 + items[0] 三格）。
	var out, errb bytes.Buffer
	rc := zerg.RunForTest([]string{"version", "--json", "name,version,commit"}, &out, &errb)
	if rc != 0 {
		t.Fatalf("`zerg version --json name,version,commit` ⇒ 应退 0，得到 %d · stderr=%q", rc, errb.String())
	}
	var env struct {
		Schema string              `json:"schema"`
		Kind   string              `json:"kind"`
		Items  []map[string]string `json:"items"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("合法档的 stdout 不是合法 JSON（包封坏了）：%v · %q", err, out.String())
	}
	if env.Schema != zerg.ContractSchemaForTest() {
		t.Errorf("包封 `schema` 应是 %q，得到 %q", zerg.ContractSchemaForTest(), env.Schema)
	}
	if len(env.Items) != 1 {
		t.Fatalf("包封 `items` 应恰 1 条，得到 %d 条：%q", len(env.Items), out.String())
	}
	for _, k := range []string{"name", "version", "commit"} {
		if v, ok := env.Items[0][k]; !ok || v == "" {
			t.Errorf("包封 `items[0].%s` 应非空（点了名就得给值），得到 %q（在不在＝%v）", k, v, ok)
		}
	}
	ver := env.Items[0]["version"]

	// ③ 不给 `--json`（人面档）⇒ 0，且首行是版本身份行 —— 判据不写死前缀：拿 ② 里机器面报的
	// 那个版本值当锚（`T4`「同一进程只有一个版本号」：两条问法同一次构建同值）。
	out.Reset()
	errb.Reset()
	rc = zerg.RunForTest([]string{"version"}, &out, &errb)
	if rc != 0 {
		t.Fatalf("`zerg version`（不给 --json）⇒ 应退 0，得到 %d · stderr=%q", rc, errb.String())
	}
	first := strings.SplitN(out.String(), "\n", 2)[0]
	if !strings.Contains(first, ver) {
		t.Errorf("`zerg version` 首行应含版本身份（机器面报的 %q），得到 %q", ver, first)
	}
}

// ---- `Q-155`（本批 · 2026-09-24）：包封内 `error.exit_code` 与**进程退码同源** ------------------
//
// 病灶（修前现读实据）：`./bin/zerg version --json bogusfield` 退 **rc=2**（用法错），而包封内
// `error.exit_code` 写 **1** —— 「命令未报出 kind」那一格把兜底 kind 写死成 `exitFail`
// （`Q-155` 逐字：**两个码面打架**）。治法：兜底 kind 由**本次真退码**派生 ⇒ `exit_code`
// （= `codeOfKind(kind)` · `E1` 的唯一真源）与 `rc` 同源。**不新造机制**：不新增 kind、
// 不新增顶层键、不改「kind → 退码」的单向口径。
//
// 判据值**不写死**：期望的那一档从退码表真源（`usageCodeFromTable`）现读 —— 表变了这条当场说话。

// TestCLIQ155_EnvelopeExitCodeSameSourceAsRC —— 正控：包封内 `exit_code` == 进程退码 `rc`，
// 且用法错那一档的兜底 kind 归位 `usage`（`E1` 单向口径仍成立）。
func TestCLIQ155_EnvelopeExitCodeSameSourceAsRC(t *testing.T) {
	want := usageCodeFromTable(t)

	for _, argv := range [][]string{
		{"version", "--json", "bogusfield"},
		{"context", "ls", "--json", "bogusfield"},
		{"approve", "ls", "--json", "name"},
	} {
		var out, errb bytes.Buffer
		rc := zerg.RunForTest(argv, &out, &errb)
		var env struct {
			Error *struct {
				Kind     string `json:"kind"`
				ExitCode int    `json:"exit_code"`
				Message  string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(out.Bytes(), &env); err != nil {
			t.Fatalf("`zerg %s` 的 stdout 不是合法 JSON（失败面包封坏了）：%v · %q",
				strings.Join(argv, " "), err, out.String())
		}
		if env.Error == nil {
			t.Fatalf("`zerg %s` 的失败面要带 `error` 块（§九 M7），得到 %q · stderr=%q",
				strings.Join(argv, " "), out.String(), errb.String())
		}
		if env.Error.ExitCode != rc {
			t.Errorf("`zerg %s`：包封内 exit_code=%d 与进程退码 rc=%d **不同源**（`Q-155` 的病灶）：%q",
				strings.Join(argv, " "), env.Error.ExitCode, rc, out.String())
		}
		if rc == want && env.Error.Kind != "usage" {
			t.Errorf("`zerg %s`：用法错的兜底 kind 要 `usage`（`E1` 单向口径），得到 %q",
				strings.Join(argv, " "), env.Error.Kind)
		}
	}
}

// TestCLIQ155_NegativeControl_SuccessHasNoErrorBlock —— 成对负控：**成功面不许挂 `error` 块**
// （同源那条纪律只管失败面；成功面挂了就是「编造信号」—— 与 `G-08`「不编造」同口径）。
func TestCLIQ155_NegativeControl_SuccessHasNoErrorBlock(t *testing.T) {
	var out, errb bytes.Buffer
	rc := zerg.RunForTest([]string{"version", "--json", "name,version"}, &out, &errb)
	if rc != 0 {
		t.Fatalf("`zerg version --json name,version` 要退 0，得到 %d · stderr=%q", rc, errb.String())
	}
	if strings.Contains(out.String(), "\"error\":") {
		t.Errorf("成功面不许带 `error` 块：%q", out.String())
	}
}
