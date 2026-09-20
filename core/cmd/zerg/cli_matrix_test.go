// cli_matrix_test.go —— 命令面自身的**契约测试 · 第一/二层**（§九 M17 · 开工单 T-42）。
//
// 落点：**外部测试包 `package main_test`**（判据②）—— 它只能看见导出标识符，
// 命令树/`run`/退码表这些未导出面由 `export_test.go` 那座**只读桥**开出来
// （桥直接指真源，不另抄副本）。
//
// 三层测试的落点（§九 M17「三层测试落点」）：
//
//	层① **进程内 · 契约面**（本文件）：`RunForTest` 直接跑一条命令，判退码与 stdout 字节数。
//	层② **进程内 · 静态面**（本文件）：矩阵覆盖（无空白/无幽灵/每条命令 ≥1 must-fail）·
//	     退码表唯一（同数字两义 ⇒ 红）· 包封形状。
//	层③ **进程外 · 合成夹具**（cli_exec_test.go）：纯 Go `os/exec` + 自写夹具，**不引依赖**。
//
// 一条硬纪律：**每条口令都必须是 must-fail**（`want_rc != 0`）。矩阵里若混进一条 rc=0 的，
// 本测试当场红 —— 「must-fail 矩阵」不许被稀释成「随便跑几条」。
package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

type matrixCase struct {
	ID              string   `json:"id"`
	Command         string   `json:"command"`
	Argv            []string `json:"argv"`
	WantRC          int      `json:"want_rc"`
	WantStdoutBytes int      `json:"want_stdout_bytes"`
	Why             string   `json:"why"`
}

type matrixFile struct {
	ID         string `json:"id"`
	Exemptions []struct {
		Command string `json:"command"`
		Reason  string `json:"reason"`
	} `json:"exemptions"`
	Cases []matrixCase `json:"cases"`
}

func loadMatrix(t *testing.T) matrixFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "cli-matrix.json"))
	if err != nil {
		t.Fatalf("读不到矩阵（testdata/cli-matrix.json）：%v", err)
	}
	var m matrixFile
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("矩阵不是合法 JSON：%v", err)
	}
	if m.ID != "cli-contract.v1" {
		t.Fatalf("矩阵 id = %q（要 cli-contract.v1）", m.ID)
	}
	if len(m.Cases) == 0 {
		t.Fatal("矩阵一个 case 都没有（空转 = 假覆盖）")
	}
	return m
}

// judgeCase 是本测试的**唯一判定口**（判据①「真的红」的落点）。
// 抽成函数是为了让负控能直接喂一个错的期望值进来（见 TestCLIContractHarnessNegativeControl）。
func judgeCase(c matrixCase, rc, stdoutBytes int) error {
	if rc != c.WantRC {
		return fmt.Errorf("退码 %d ≠ 期望 %d", rc, c.WantRC)
	}
	if stdoutBytes != c.WantStdoutBytes {
		return fmt.Errorf("stdout %d 字节 ≠ 期望 %d 字节", stdoutBytes, c.WantStdoutBytes)
	}
	return nil
}

// runCase 跑一条口令（进程内 · 层①）。stdin 一律为空：**无 TTY 零提示词**（K6）。
func runCase(c matrixCase) (int, int) {
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(c.Argv, &out, &errb)
	return rc, out.Len()
}

// TestCLIContractMustFailMatrix —— 判据①：每条口令**真的红**（退码 + stdout 字节数逐条对）。
func TestCLIContractMustFailMatrix(t *testing.T) {
	m := loadMatrix(t)
	bad, n := 0, 0
	for _, c := range m.Cases {
		if c.WantRC == 0 {
			t.Errorf("case %s 的 want_rc = 0 —— must-fail 矩阵不许有 rc=0 的条目（判据①）", c.ID)
			bad++
			continue
		}
		rc, outBytes := runCase(c)
		if err := judgeCase(c, rc, outBytes); err != nil {
			t.Errorf("case %s（%v · %s）不红/形状变了：%v", c.ID, c.Argv, c.Why, err)
			bad++
		}
		n++
	}
	t.Logf("矩阵：%d 条 case 跑完 · 不符 %d 条 · 命令覆盖 %d 条", n, bad, len(commandsInMatrix(m)))
}

func commandsInMatrix(m matrixFile) map[string]bool {
	set := map[string]bool{}
	for _, c := range m.Cases {
		set[c.Command] = true
	}
	return set
}

// TestCLIContractMatrixNoBlank —— 判据④「矩阵不留空白」（§九 M17 `T5` 的进程内一半）：
//
//	① 命令树里**每条**命令都要有 ≥1 条 case，或一条**带 reason** 的豁免；
//	   没有 case 又没有 reason 的豁免 ⇒ 空白（红）；有豁免但 reason 空 ⇒ **不给结论**（测试直接 Fatal）。
//	② 矩阵里**不许有幽灵**（case 点名的命令在命令树里不存在）。
func TestCLIContractMatrixNoBlank(t *testing.T) {
	m := loadMatrix(t)
	have := map[string]int{}
	for _, c := range m.Cases {
		have[c.Command]++
	}
	exempt := map[string]string{}
	for _, e := range m.Exemptions {
		if strings.TrimSpace(e.Reason) == "" {
			t.Fatalf("豁免 %q **没有 reason** —— 写不清理由的豁免就是洗白（§九 M17 T5：reason 缺 ⇒ 不给结论 2）", e.Command)
		}
		exempt[e.Command] = e.Reason
	}
	tree := zerg.CommandPathsForTest()
	if len(tree) == 0 {
		t.Fatal("命令树读出来是空的（空转 = 假覆盖）")
	}
	blanks := []string{}
	for _, p := range tree {
		if have[p] == 0 && exempt[p] == "" {
			blanks = append(blanks, p)
		}
	}
	if len(blanks) > 0 {
		sort.Strings(blanks)
		t.Errorf("矩阵有 %d 条**空白**（命令没有 must-fail case 也没有带 reason 的豁免）：%s",
			len(blanks), strings.Join(blanks, " · "))
	}
	ghosts := []string{}
	for _, p := range sortedKeys(have) {
		if _, ok := zerg.CommandInfoOfForTest(p); !ok {
			ghosts = append(ghosts, p)
		}
	}
	if len(ghosts) > 0 {
		t.Errorf("矩阵里有 %d 条**幽灵条目**（命令树里没有这条命令）：%s", len(ghosts), strings.Join(ghosts, " · "))
	}
	t.Logf("命令树 %d 条 · 有 case 的 %d 条 · 带 reason 的豁免 %d 条 · 空白 %d 条",
		len(tree), len(have), len(exempt), len(blanks))
}

// TestCLIContractHarnessNegativeControl —— **成对负控**：先证明这台机器能把红判出来，
// 再谈「全过」。喂一条改正过的期望值，judgeCase 必须报错（否则本测试就是恒绿装置）。
func TestCLIContractHarnessNegativeControl(t *testing.T) {
	m := loadMatrix(t)
	c := m.Cases[0]
	rc, outBytes := runCase(c)
	if err := judgeCase(c, rc, outBytes); err != nil {
		t.Fatalf("负控第 0 步就不成立：真值都判不过（%v）", err)
	}
	wrong := c
	wrong.WantRC = c.WantRC + 1
	if err := judgeCase(wrong, rc, outBytes); err == nil {
		t.Error("负控失败：期望值改错后**没有**报错 —— 这台机器判不出红（恒绿装置，不许当覆盖）")
	}
	wrong2 := c
	wrong2.WantStdoutBytes = c.WantStdoutBytes + 1
	if err := judgeCase(wrong2, rc, outBytes); err == nil {
		t.Error("负控失败：stdout 字节数改错后**没有**报错")
	}
}

// TestCLIContractExitCodeTableUnique —— 判据③「退码表唯一」的**进程内**一半：
// 表内数字不许重复（同数字两义 ⇒ 红），每格三要素齐（号/名/语义）。
func TestCLIContractExitCodeTableUnique(t *testing.T) {
	rows := zerg.ExitCodeTableForTest()
	if len(rows) == 0 {
		t.Fatal("退码表读出来是空的（空转 = 假覆盖）")
	}
	seen := map[int]string{}
	dups := []string{}
	for _, r := range rows {
		if r.Name == "" || r.Meaning == "" {
			t.Errorf("退码 %d 的三要素不齐（名=%q 语义=%q）", r.Code, r.Name, r.Meaning)
		}
		if prev, ok := seen[r.Code]; ok {
			dups = append(dups, fmt.Sprintf("%d 同时是 %q 与 %q", r.Code, prev, r.Name))
			continue
		}
		seen[r.Code] = r.Name
	}
	if len(dups) > 0 {
		t.Errorf("退码表内**同数字两义** %d 处：%s（§十二 P-013 占号纪律：禁止同数字两义）",
			len(dups), strings.Join(dups, " · "))
	}
	t.Logf("退码表：%d 格 · 数字唯一 ✓（%v）", len(rows), sortedCodes(rows))
}

func sortedCodes(rows []zerg.ExitCodeRowForTest) []int {
	out := []int{}
	for _, r := range rows {
		out = append(out, r.Code)
	}
	sort.Ints(out)
	return out
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestCLIContractJSONShape —— 判据⑥/`T4`「`--json` 形状守卫（运行时只读）」（§九 M6）：
// 跑一条**离线**命令，逐键核包封六键 + `schema` 取值 + `items` 恒数组。
// 门 `check-cli-contract.py` 的 T4 就是调这一条（`-run TestCLIContractJSONShape`）：
// 它读的是**当前源码**的运行期行为，不是 `bin/` 里的旧制品。
func TestCLIContractJSONShape(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := zerg.RunForTest([]string{"version", "--json", "name"}, &out, &errb); rc != 0 {
		t.Fatalf("version --json name 退码 = %d（要 0）· stderr=%s", rc, errb.String())
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("--json 输出不是对象：%v", err)
	}
	for _, k := range zerg.EnvelopeKeysForTest() {
		if _, ok := doc[k]; !ok {
			t.Errorf("包封缺键 %q（§九 M6 I1：六键恒在）", k)
		}
	}
	for k := range doc {
		found := false
		for _, want := range zerg.EnvelopeKeysForTest() {
			if k == want {
				found = true
			}
		}
		if !found {
			t.Errorf("包封多了未声明键 %q（真源 = EnvelopeKeysForTest）", k)
		}
	}
	var schema string
	if err := json.Unmarshal(doc["schema"], &schema); err != nil || schema != zerg.ContractSchemaForTest() {
		t.Errorf("schema = %s（要 %q）", string(doc["schema"]), zerg.ContractSchemaForTest())
	}
	if !bytes.HasPrefix(bytes.TrimSpace(doc["items"]), []byte("[")) {
		t.Errorf("items 不是数组：%s（§九 M6 I3：恒数组、永不为 null）", string(doc["items"]))
	}
}
