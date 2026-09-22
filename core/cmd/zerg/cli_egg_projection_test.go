// cli_egg_projection_test.go —— 卵面**只读投影**的契约测试（波② · `Q-101` · 任务清单 `T4`）。
//
// 落点：外部测试包 `package main_test`（与命令面其余契约测试同一层）；合成主控用本包既有的
// `newSyntheticMaster`（`fusion_test.go` —— 一个真源，不另起一个夹具）。
//
// 三条判据在这里**机检**（四判据里能脱离真机跑的那三条）：
//
//	① 每行三格齐（`host` / `model` / `state`）：合成主的**候选缺 host** ⇒ **必红**（判据④ 的负控 ·
//	   真实跑一次「抹掉 host 列」的动作在合成面上做，不靠人眼）
//	② `state` 值域与主控 `GET /api/models/{name}` 的 `status` **逐字同源**：正例两字都出得来；
//	   反例（主控 `status` 与快照算的那一格不一致）⇒ **必红**（不许挑一个当答案）
//	③ 只读：本文件只 GET；不写任何件、不改任何状态（合成主的 handler 只写响应）
package main_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// eggFleetJSON —— 合成名册：三种形状各一枚（单机候选 · 双机候选 · 缺 host 的那一枚由调用方拼）。
const eggFleetJSON = `{"count":3,"models":[
 {"id":"Egg-A","host":"Mr2109","backend":"llama-server","mem_gb":6},
 {"id":"Egg-B","host":"Mr2109","backend":"llama-server","mem_gb":7},
 {"id":"Egg-B","host":"x3","backend":"llama-server","mem_gb":7}]}`

// eggStatusJSON —— 合成各机驻留：Egg-A 在 Mr2109 上**已加载**，Egg-B 两台都**未加载**。
const eggStatusJSON = `{"machines":{
 "Mr2109":{"machine":"Mr2109","models":["Egg-A"]},
 "x3":{"machine":"x3","models":[]}}}`

func runEggCmd(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// 判据①：每行三格齐 · 表头就是三格 · rc=0（今天那条 rc=8 的 fail-closed 不许留）。
func TestEggLsMatrixThreeCells(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"未加载"}`,
	})
	defer srv.Close()

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 4 { // 表头 + 3 枚卵
		t.Fatalf("egg ls 行数 = %d（要 4 = 表头 + 3 枚卵）· stdout=%q", len(lines), stdout)
	}
	if lines[0] != "host\tmodel\tstate" {
		t.Errorf("表头 = %q（要 `host\\tmodel\\tstate` · 三格逐字）", lines[0])
	}
	for i, ln := range lines[1:] {
		if got := len(strings.Split(ln, "\t")); got != 3 {
			t.Errorf("第 %d 行格数 = %d（要 3 = host/model/state 三格齐）· 行=%q", i+1, got, ln)
		}
	}
	// 排序固定（矩阵要能逐次对拍）：先 host 再 model。
	if !(strings.HasPrefix(lines[1], "Mr2109\tEgg-A\t") && strings.HasPrefix(lines[2], "Mr2109\tEgg-B\t") &&
		strings.HasPrefix(lines[3], "x3\tEgg-B\t")) {
		t.Errorf("次序不是（host, model）升序 ⇒ 矩阵不可对拍：%q", stdout)
	}
}

// 判据②：`state` 值域**只有主控的两个词** —— 已加载 / 未加载，逐字（正例两字都出得来）。
func TestEggStateVerbatimFromMasterStatus(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"未加载"}`,
	})
	defer srv.Close()

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	seen := map[string]bool{}
	for _, ln := range strings.Split(strings.TrimRight(stdout, "\n"), "\n")[1:] {
		f := strings.Split(ln, "\t")
		seen[f[2]] = true
	}
	if !seen["已加载"] || !seen["未加载"] {
		t.Fatalf("两个词都要出得来（主控 `GET /api/models/{name}` 的 `status` 就是这个闭集）：%v", seen)
	}
	for w := range seen {
		if w != "已加载" && w != "未加载" {
			t.Errorf("`state` 出现了闭集外的词 %q（判据②：同一格不许两套词）", w)
		}
	}
}

// 判据④ 负控（**真跑**）：名册候选**缺 host** ⇒ 必红（不许静默打一行空格）。
// 这正是「抹掉 host 列」那一次变异在合成面上的复现 —— 与真实二进制上的变异自证成对。
func TestEggLsHostAbsentMustGoRed(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": `{"count":1,"models":[{"id":"Egg-A","backend":"llama-server","mem_gb":6}]}`,
		"/api/fleet/status": eggStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
	})
	defer srv.Close()

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc == 0 {
		t.Fatalf("缺 host 列的卵 ⇒ 必须判红，实际 rc=0 · stdout=%q", stdout)
	}
	if stdout != "" {
		t.Errorf("判红时 stdout 必须 0 字节（不许先打半张表）：%q", stdout)
	}
	if !strings.Contains(stderr, "host") {
		t.Errorf("判红要**点名缺的是哪一格**（`host`）：stderr=%s", stderr)
	}
}

// 判据② 正面：主控点名那台机上那一格**逐字取主控原话** —— 哪怕快照（同一台机）与它不一致，
// 也不许自己算一个、更不许因此判红（只读投影要**可重复**；「两次读数的时序差」不是语义分歧）。
func TestEggAuthoritativeCellIsMasterWordVerbatim(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON, // Mr2109 的快照里只有 Egg-A
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		// 主控说 Egg-B 在 Mr2109 上是 `已加载`（快照里没有它）⇒ 这一格必须照主控的原文出。
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"已加载"}`,
	})
	defer srv.Close()

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 0 {
		t.Fatalf("主控原话与快照有差拍 ⇒ 只读投影**不许**判红：rc=%d · stderr=%s", rc, stderr)
	}
	if !strings.Contains(stdout, "Mr2109	Egg-B	已加载") {
		t.Errorf("主控点名的那一格必须**逐字**取主控 `status`：%q", stdout)
	}
	// 另一台机（快照里确实没驻留）那一格，按同一闭集填 `未加载`。
	if !strings.Contains(stdout, "x3	Egg-B	未加载") {
		t.Errorf("非主控点名的那台机按快照填同一闭集的词：%q", stdout)
	}
}

// 判据② 负控（**真跑**）：主控把 `status` **扩出闭集**（此处编成 `加载中`）⇒ 必红。
// 为什么这是「两套词」的机检：卵面的 `state` 只认 `已加载`/`未加载` 两个词，主控一旦扩词，
// 要么跟着漂（两套词）、要么原样打出去假装没事 —— 两条都不许 ⇒ 判红交人定夺。
func TestEggStateOutOfClosedSetMustGoRed(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"加载中"}`,
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"未加载"}`,
	})
	defer srv.Close()

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc == 0 {
		t.Fatalf("主控 `status` 出闭集 ⇒ 必须判红，实际 rc=0 · stdout=%q", stdout)
	}
	if stdout != "" {
		t.Errorf("判红时 stdout 必须 0 字节：%q", stdout)
	}
	if !strings.Contains(stderr, "不在卵面 `state` 闭集") {
		t.Errorf("判红要点名「出闭集」并把闭集两个词报出来：stderr=%s", stderr)
	}
}

// `egg show` 三种输入形态：`模型@机`（正例） · 单机模型的裸模型名（代为定位） ·
// 多机模型的裸模型名（**歧义 ⇒ 退码 2，不替人挑**）。
func TestEggShowTargets(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"未加载"}`,
	})
	defer srv.Close()

	rc, stdout, stderr := runEggCmd(t, "egg", "show", "Egg-A@Mr2109", "--plain")
	if rc != 0 {
		t.Fatalf("egg show Egg-A@Mr2109 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	if !strings.Contains(stdout, "Mr2109\tEgg-A\t已加载") {
		t.Errorf("单枚卵的三格没出齐：%q", stdout)
	}

	// 只有一台候选机 ⇒ 裸模型名可代为定位。
	if rc, _, stderr := runEggCmd(t, "egg", "show", "Egg-A", "--plain"); rc != 0 {
		t.Errorf("单机模型的裸名应能定位：rc=%d · stderr=%s", rc, stderr)
	}

	// 两台候选机 ⇒ 歧义，退码 2 且**列出**合法的卵 id。
	rc, stdout, stderr = runEggCmd(t, "egg", "show", "Egg-B", "--plain")
	if rc != 2 {
		t.Fatalf("多机模型的裸名 ⇒ 退码 2（用法错），实际 rc=%d · stdout=%q", rc, stdout)
	}
	if stdout != "" {
		t.Errorf("用法错时 stdout 必须 0 字节：%q", stdout)
	}
	for _, want := range []string{"Egg-B@Mr2109", "Egg-B@x3"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("歧义面要列出合法的卵 id（缺 %s）：stderr=%s", want, stderr)
		}
	}

	// 名册里没有 ⇒ 退码 2 + 「最像的合法输入」。
	rc, _, stderr = runEggCmd(t, "egg", "show", "Egg-Z@Mr2109", "--plain")
	if rc != 2 || !strings.Contains(stderr, "最像的合法输入") {
		t.Errorf("名册里没有的卵 id ⇒ 2 + 提示最像的合法输入：rc=%d · stderr=%s", rc, stderr)
	}
}

// `--json` 面：字段名逐字（三格 + egg_id），且三格在机器面上一个不少。
func TestEggLsJSONFields(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"未加载"}`,
	})
	defer srv.Close()

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--json", "egg_id,host,model,state")
	if rc != 0 {
		t.Fatalf("egg ls --json 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	var doc struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("--json 输出不是合法对象：%v · stdout=%s", err, stdout)
	}
	if len(doc.Items) != 3 {
		t.Fatalf("--json 条目数 = %d（要 3）", len(doc.Items))
	}
	for _, it := range doc.Items {
		if it["egg_id"] != it["model"]+"@"+it["host"] {
			t.Errorf("egg_id 与（模型, 机）这一对不一致：%v", it)
		}
		if it["state"] == "" {
			t.Errorf("机器面上 state 缺格：%v", it)
		}
	}
}
