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

// eggStubBackends 把**子端真后端探测口**换成合成回据（`GAP-20260926-08`）。
//
// 为什么判据件必须换口：子端地址来自 `gateway/fleet.yaml`（真机地址），而本机 `Mr2109` 的子端
// **恰好活着** ⇒ 走真口测出来的绿是「今天恰好是这样」的绿（不 hermetic）。换成合成回据之后，
// 三态**每一态都能稳定造出来**（真后端在跑 / 不在 / 取不到），判据才钉得住。
func eggStubBackends(t *testing.T, byHost map[string]zerg.EggBackendFactForTest) {
	t.Helper()
	t.Cleanup(zerg.SetEggBackendProbeForTest(byHost))
}

// eggStubOnMS01 —— 最常用的一份合成回据：`Mr2109` 答得动（`Egg-A` 在跑 · 端口 9001），`x3` 答得动但没有卵。
// 名册里没有的机 ⇒ 不在表里 ⇒ 「取不到」（与真路径同形）。
func eggStubOnMS01(t *testing.T) {
	t.Helper()
	eggStubBackends(t, map[string]zerg.EggBackendFactForTest{
		"Mr2109": {Reachable: true, Running: map[string]int{"Egg-A": 9001}},
		"x3":   {Reachable: true},
	})
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
	eggStubOnMS01(t) // 判据件不连真网：子端真后端回据换成合成件

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

// 判据②（改版 · `GAP-20260926-08`）：`state` 值域是**卵面四词闭集**（`已加载` / `未加载` /
// `已声明但后端不在` / `取不到`）—— 闭集外的词一个都不许出现。
//
// 前身是「`state` 只有主控那两个词」：那条判据在**同一条链的两处读数**之间判一致
// （主控 `status` ⟷ 心跳快照），而**两处的源头是同一个子端声明**（`RegistryNames()`）
// ⇒ 它判不出「声明 ≠ 真后端」，正是本缺口的成因。判据改成「闭集收口 + 三态可造」。
func TestEggStateValueDomainIsClosedSet(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"未加载"}`,
	})
	defer srv.Close()
	eggStubOnMS01(t)

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	closed := map[string]bool{}
	for _, w := range zerg.EggStateClosedSetForTest() {
		closed[w] = true
	}
	seen := map[string]bool{}
	for _, ln := range strings.Split(strings.TrimRight(stdout, "\n"), "\n")[1:] {
		f := strings.Split(ln, "	")
		seen[f[2]] = true
	}
	// 正例：真后端在跑那枚 ⇒ `已加载`；没声也没跑那两枚 ⇒ `未加载`。
	if !seen["已加载"] || !seen["未加载"] {
		t.Fatalf("`已加载` / `未加载` 两个词都要出得来：%v", seen)
	}
	for w := range seen {
		if !closed[w] {
			t.Errorf("`state` 出现了闭集外的词 %q（闭集 = %v）", w, zerg.EggStateClosedSetForTest())
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

// ★ `GAP-20260926-08` 的**正题**（本判据件的前身逐字钉住的正是那个 bug，故此处整条重写）：
//
//	主控说 `已加载`，而子端 `/eggs` 说**没这枚卵**（引擎进程已经没了）
//	⇒ `state` 必须落 `已声明但后端不在`，**不许**逐字照抄主控的 `已加载`。
//
// 前身 `TestEggAuthoritativeCellIsMasterWordVerbatim` 的判据是「主控点名那台机上那一格**逐字取
// 主控原话**」—— 它把「声明」当成了真值，所以**卸载之后引擎已无、`egg ls` 仍报「已加载」**
// 这个 P0 现象在它眼里是**绿**的（实测：`Mr2109` 注册表 7 条 ⇒ 报 7 枚已加载，而 `ps` 上只有 1 个
// `llama-server`）。修法是把那一格改由**子端真后端回据**落定；本判据钉住新语义。
//
// 同时钉住**取不到**这一档：读不到就只是读不到 —— 既不报「已加载」（本缺口），也不并进
// 「未加载」（那是**编一个数**）。
func TestEggStateFollowsChildBackendNotMasterDeclaration(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON, // Mr2109 的快照里只有 Egg-A
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		// 主控说 Egg-B 在 Mr2109 上是 `已加载` —— 但子端 `/eggs` 里**没有它**（真后端不在）。
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"已加载"}`,
	})
	defer srv.Close()
	// 合成子端：Mr2109 答得动、只有 Egg-A 在跑（端口 9001）；x3 答得动、没有卵。
	eggStubOnMS01(t)

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	// ① 声明与回据一致 ⇒ 真 `已加载`。
	if !strings.Contains(stdout, "Mr2109	Egg-A	已加载") {
		t.Errorf("子端 `/eggs` 里在跑的那一枚要落 `已加载`：%q", stdout)
	}
	// ② 声明说「已加载」而真后端不在 ⇒ **必须**是 `已声明但后端不在`（不许照抄声明）。
	if !strings.Contains(stdout, "Mr2109	Egg-B	已声明但后端不在") {
		t.Errorf("主控声「已加载」而子端回据里没有它 ⇒ 必须落 `已声明但后端不在`：%q", stdout)
	}
	if strings.Contains(stdout, "Mr2109	Egg-B	已加载") {
		t.Errorf("**本缺口的原现象**：不许把主控的声明当真值（`Egg-B@Mr2109` 照抄成了 `已加载`）：%q", stdout)
	}
	// ③ 非主控点名的那台机：主控没声、子端也没有 ⇒ `未加载`（两处一致）。
	if !strings.Contains(stdout, "x3	Egg-B	未加载") {
		t.Errorf("两处一致地「没在跑」那一格要落 `未加载`：%q", stdout)
	}
}

// `取不到` 必须是**独立一格**：子端 `/eggs` 打不到 ⇒ `state` 落 `取不到`，且**整张表都不给结论**
// （退码 8 · stdout 0 字节）—— 不许把它写进 `已加载`（本缺口）或 `未加载`（编一个数）。
func TestEggBackendUnreadableIsItsOwnState(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"已加载"}`,
	})
	defer srv.Close()
	// 合成子端：**两台机都答不动** ⇒ 全表都是 `取不到`。
	eggStubBackends(t, map[string]zerg.EggBackendFactForTest{
		"Mr2109": {Reachable: false},
		"x3":   {Reachable: false},
	})

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 8 {
		t.Fatalf("整张表的真后端回据都取不到 ⇒ 退码 8（blocked · 前置拿不到），实际 rc=%d · stdout=%q", rc, stdout)
	}
	// ★ 表格**照旧打出来**：`取不到` 是逐枚卵的结论，机器/人都要看得见它（清空 stdout 等于让
	//   「取不到」这一态消失）。人面三格那一格里逐字写着 `取不到` ⇒ 不会被当成「已加载」。
	if !strings.Contains(stdout, "	取不到") {
		t.Errorf("人面也要逐字说白 `取不到`（不许清空 stdout、更不许并进 `已加载`）：%q", stdout)
	}
	if strings.Contains(stdout, "已加载") || strings.Contains(stdout, "未加载") {
		t.Errorf("取不到**不许**塌成 `已加载` / `未加载`：%q", stdout)
	}
	if !strings.Contains(stderr, "取不到") {
		t.Errorf("判词要点名 `取不到` 这一档：stderr=%s", stderr)
	}
	if !strings.Contains(stderr, "backend_unreadable") {
		t.Errorf("机器面要留 detail=backend_unreadable：stderr=%s", stderr)
	}

	// 机器面：三态照样分得开（`取不到` 不许塌成 `未加载`）。
	rc, stdout, stderr = runEggCmd(t, "egg", "ls", "--json", "host,model,state,declared,backend_port")
	if rc != 8 {
		t.Fatalf("有卵取不到 ⇒ 退码 8，实际 rc=%d · stderr=%s", rc, stderr)
	}
	if !strings.Contains(stdout, `"state":"取不到"`) {
		t.Errorf("机器面上 `取不到` 必须逐字出得来：%q", stdout)
	}
	if strings.Contains(stdout, `"state":"未加载"`) || strings.Contains(stdout, `"state":"已加载"`) {
		t.Errorf("取不到**不许**塌成 `已加载` / `未加载`：%q", stdout)
	}
}

// 三态在同一跑里**分得开**：真在跑 / 已声明而后端不在 / 取不到，三行三词（机器面逐字可辨）。
func TestEggThreeStatesDistinguishable(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggFleetJSON,
		"/api/fleet/status": eggStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"已加载"}`,
	})
	defer srv.Close()
	// Mr2109 答得动（Egg-A 在跑）· x3 **答不动**（⇒ 那枚卵取不到）。
	eggStubBackends(t, map[string]zerg.EggBackendFactForTest{
		"Mr2109": {Reachable: true, Running: map[string]int{"Egg-A": 9001}},
		"x3":   {Reachable: false},
	})

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--json", "host,model,state,declared,backend_port")
	if rc != 8 {
		t.Fatalf("有机取不到 ⇒ 退码 8（部分不给结论），实际 rc=%d · stderr=%s", rc, stderr)
	}
	var doc struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("--json 输出不是合法对象：%v · stdout=%s", err, stdout)
	}
	got := map[string]map[string]string{}
	for _, it := range doc.Items {
		got[it["host"]+"	"+it["model"]] = it
	}
	// ① 真 `已加载`：声明与子端回据一致，且 `backend_port` 逐字带出端口。
	a := got["Mr2109	Egg-A"]
	if a["state"] != "已加载" || a["declared"] != "已加载" || a["backend_port"] != "9001" {
		t.Errorf("真 `已加载` 那一格：state/declared/backend_port = %v（要 已加载/已加载/9001）", a)
	}
	// ② `已声明但后端不在`：声明 `已加载`、回据里没有它、端口空格。
	b := got["Mr2109	Egg-B"]
	if b["state"] != "已声明但后端不在" || b["declared"] != "已加载" || b["backend_port"] != "" {
		t.Errorf("`已声明但后端不在` 那一格：%v（要 已声明但后端不在/已加载/空）", b)
	}
	// ③ `取不到`：子端答不动那一台机上那一枚。
	c := got["x3	Egg-B"]
	if c["state"] != "取不到" {
		t.Errorf("`取不到` 那一格：%v", c)
	}
	// 三态互不相同（不许塌成一格）。
	words := map[string]bool{a["state"]: true, b["state"]: true, c["state"]: true}
	if len(words) != 3 {
		t.Errorf("三态必须分得开，实得 %d 种：%v", len(words), words)
	}
	// `已加载` 与 `未加载` 两个旧词**逐字未动**（不许换字）。
	if a["state"] != "已加载" || b["state"] == "已加载" {
		t.Errorf("旧词逐字未动这条不成立：%v", words)
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
	eggStubOnMS01(t)

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
	eggStubOnMS01(t)

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
