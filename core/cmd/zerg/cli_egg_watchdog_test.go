// cli_egg_watchdog_test.go —— `GAP-20260925-15`（Face A · 字段面投影 · `egg ls` / `egg show`）。
//
// 病（改前现读）：子端 `/eggs` **已经透出**看门狗读数（`watchdog` 子对象 = `backend.WatchdogObservation`：
// `last_output_ago_s` + `verdict`），而命令面此前**只解析** `port/managed/state/model`、把这子对象**丢掉**
// ⇒ 「这枚卵多久没出声、看门狗怎么判的」在卵面上**看不见**（一枚本地 35B 卵卡死 813.6s、停在原地 93 分钟
// 无人打断，父代理只能人肉盯日志）。
//
// 治法（本笔）：探测保留 `watchdog` 子对象 ⇒ 卵面增两格 `last_output_ago_s` / `watchdog_verdict`，**逐字照子端**。
//
// 三条判据（都在**合成**面上跑 · ✗ 不连真集群 · ✗ 不改生产状态）：
//
//	正控：正常载荷 ⇒ 两格读得到、数值**逐字相符**；`0` / `ok` 也**逐字照报**（不许当「没值」）。
//	负控一：喂 `verdict:"stuck_no_progress"` / `last_output_ago_s:813.6` ⇒ 须**如实报该值**
//	        （不是恒 `ok`、不是恒 `0`）。
//	负控二：不给 `/eggs` 读数（子端答不动）⇒ 按既有一档 fail-closed（rc=8 · `state`=`取不到`），
//	        两格**留空** —— ✗ 不许假装成 `ok`/`0`。
//
// ★ 诚实边界（本判据件钉的硬线）：`watchdog` 是**上一次请求的快照** ⇒ 看门狗没跑时子端报 `ok`/`0`；
// 本面**逐字照报**，✗ 不加工成「实时静默 N 秒」、✗ 不把 `0` 说成「未卡」（值域照子端原样）。
package main_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// ---- 合成载荷（只装判据要的那几格 · 字段名逐字照子端）----------------------------------------

// eggWatchdogFleetJSON —— 合成名册：两枚卵都在 Mr2109。
const eggWatchdogFleetJSON = `{"count":2,"models":[
 {"id":"Egg-A","host":"Mr2109","backend":"llama-server","mem_gb":6},
 {"id":"Egg-B","host":"Mr2109","backend":"llama-server","mem_gb":7}]}`

// eggWatchdogStatusJSON —— 合成各机驻留：两枚都在 Mr2109 的 `models[]` 里。
const eggWatchdogStatusJSON = `{"machines":{"Mr2109":{"machine":"Mr2109","models":["Egg-A","Egg-B"]}}}`

// eggChildEgg 合成子端 `/eggs` 的一枚卵条目（含 `watchdog` 子对象）。`wdLastOut` 用**原始 JSON 数值文本**
// 拼（`813.6` 不是 `"813.6"`）—— 判「逐字照子端」必须让子端真的发数值，而不是字符串。
func eggChildEgg(eggID string, port int, wdVerdict, wdLastOut string) string {
	return `{"egg_id":"` + eggID + `","model":"` + eggID + `","state":"ready","port":` + strconv.Itoa(port) +
		`,"managed":true,"watchdog":{"window_idx":1,"extensions":0,"last_output_ago_s":` + wdLastOut +
		`,"cpu_usage_usec_delta":900000,"verdict":"` + wdVerdict + `","reason":"合成载荷"}}`
}

// eggChildEggsJSON 拼 `/eggs` 的整份载荷。
func eggChildEggsJSON(items ...string) string {
	return `{"eggs":[` + strings.Join(items, ",") + `],"generated_at":"2026-10-10T00:00:00Z"}`
}

// eggLiveChild 起一个**本进程内**的合成子端（只答 `/eggs` · 喂**原始 JSON 载荷**），并把**真**探测口
// （`eggBackendProbeLive`）接到它上 ⇒ 判据走的是**真解析路径**（✗ 不连真集群 · 地址与口令都不真）。
func eggLiveChild(t *testing.T, eggsJSON string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eggs" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, eggsJSON)
	}))
	t.Cleanup(srv.Close)
	port, err := strconv.Atoi(strings.TrimPrefix(srv.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatalf("合成子端端口读不出：%v（%s）", err, srv.URL)
	}
	t.Cleanup(zerg.SetEggBackendProbeLiveForTest("127.0.0.1", port))
}

// eggWatchdogMaster 起合成主控（两枚卵都在 Mr2109 · 都是 `已加载`），返回它的关闭函数。
func eggWatchdogMaster(t *testing.T) *httptest.Server {
	t.Helper()
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggWatchdogFleetJSON,
		"/api/fleet/status": eggWatchdogStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"已加载"}`,
		"/api/models/Egg-B": `{"name":"Egg-B","host":"Mr2109","status":"已加载"}`,
	})
	t.Cleanup(srv.Close)
	return srv
}

// eggWatchdogRead 跑一条命，把 `host\tmodel` ⇒ 行 取回来（`--json` 面）。
func eggWatchdogRead(t *testing.T, argv ...string) (int, map[string]map[string]string, string) {
	t.Helper()
	rc, stdout, stderr := runEggCmd(t, argv...)
	got := map[string]map[string]string{}
	if strings.TrimSpace(stdout) != "" {
		var doc struct {
			Items []map[string]string `json:"items"`
		}
		if err := json.Unmarshal([]byte(stdout), &doc); err == nil {
			for _, it := range doc.Items {
				got[it["host"]+"\t"+it["model"]] = it
			}
		}
	}
	return rc, got, stderr
}

// ---- 正控：正常载荷 ⇒ 两格逐字 ---------------------------------------------------------------

func TestEggWatchdogProjectionVerbatim(t *testing.T) {
	eggWatchdogMaster(t)
	// Egg-A 看门狗判「延长」（12.5s 无输出但工时在动）；Egg-B 看门狗没跑 ⇒ 子端报 `ok` / `0`。
	eggLiveChild(t, eggChildEggsJSON(
		eggChildEgg("Egg-A", 9001, "extended", "12.5"),
		eggChildEgg("Egg-B", 9002, "ok", "0"),
	))

	rc, got, stderr := eggWatchdogRead(t, "egg", "ls", "--json",
		"host,model,state,last_output_ago_s,watchdog_verdict")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	a, b := got["Mr2109\tEgg-A"], got["Mr2109\tEgg-B"]
	if a == nil || b == nil {
		t.Fatalf("两枚卵的行缺格：%v", got)
	}
	if a["last_output_ago_s"] != "12.5" || a["watchdog_verdict"] != "extended" {
		t.Errorf("正控 Egg-A 两格要逐字 `12.5`/`extended`，实得 %q/%q",
			a["last_output_ago_s"], a["watchdog_verdict"])
	}
	// ★ 诚实边界：子端报 `0` / `ok` ⇒ **逐字照报**（不把 `0` 当「没值」，也不把 `ok` 加工成别的）。
	if b["last_output_ago_s"] != "0" || b["watchdog_verdict"] != "ok" {
		t.Errorf("正控 Egg-B（看门狗没跑）要逐字 `0`/`ok`，实得 %q/%q",
			b["last_output_ago_s"], b["watchdog_verdict"])
	}

	// `egg show` 与 `egg ls` **同源**：同一枚卵同一份真源 ⇒ 两格一样。
	rc, got, stderr = eggWatchdogRead(t, "egg", "show", "Egg-A@Mr2109", "--json",
		"host,model,last_output_ago_s,watchdog_verdict")
	if rc != 0 {
		t.Fatalf("egg show 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	if s := got["Mr2109\tEgg-A"]; s == nil || s["last_output_ago_s"] != "12.5" || s["watchdog_verdict"] != "extended" {
		t.Errorf("egg show 与 egg ls 不同源：%v", got)
	}
}

// ---- 负控一：卡住的卵 ⇒ 如实报该值（不是恒 `ok`/`0`）-----------------------------------------

func TestEggWatchdogStuckMustReportVerbatim(t *testing.T) {
	eggWatchdogMaster(t)
	// 这正是 `GAP-20260925-15` 的现场载荷：判词 `stuck_no_progress` · 静默 813.6s。
	eggLiveChild(t, eggChildEggsJSON(
		eggChildEgg("Egg-A", 9001, "stuck_no_progress", "813.6"),
	))

	rc, got, stderr := eggWatchdogRead(t, "egg", "ls", "--json",
		"host,model,last_output_ago_s,watchdog_verdict")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	a := got["Mr2109\tEgg-A"]
	if a == nil {
		t.Fatalf("行缺格：%v", got)
	}
	if a["watchdog_verdict"] != "stuck_no_progress" {
		t.Errorf("负控一：判词要**如实报** `stuck_no_progress`（不许恒 `ok`），实得 %q", a["watchdog_verdict"])
	}
	if a["last_output_ago_s"] != "813.6" {
		t.Errorf("负控一：静默读数要**逐字** `813.6`，实得 %q", a["last_output_ago_s"])
	}
	if a["watchdog_verdict"] == "ok" || a["last_output_ago_s"] == "0" {
		t.Errorf("负控一：把卡住读成 `ok`/`0` = 本缺口的原现象：%v", a)
	}
}

// ---- 负控二：不给 `/eggs` 读数 ⇒ 按既有档 fail-closed，两格留空（不编 ok/0）------------------

func TestEggWatchdogNoReadingMustNotFabricate(t *testing.T) {
	eggWatchdogMaster(t)
	// 子端答不动（合成回据 Reachable=false ⇒ 与真路径的失败面同形）。
	eeggStubBackendsMS01Down(t)

	rc, got, stderr := eggWatchdogRead(t, "egg", "ls", "--json",
		"host,model,state,last_output_ago_s,watchdog_verdict")
	if rc != 8 {
		t.Fatalf("整张表的真后端回据都取不到 ⇒ 退码 8（blocked），实际 rc=%d · stderr=%s", rc, stderr)
	}
	if !strings.Contains(stderr, "backend_unreadable") {
		t.Errorf("机器面要留 detail=backend_unreadable：stderr=%s", stderr)
	}
	a := got["Mr2109\tEgg-A"]
	if a == nil {
		t.Fatalf("行缺格（`取不到` 是逐枚卵的结论、必须看得见）：%v", got)
	}
	if a["state"] != "取不到" {
		t.Errorf("取不到要逐字落 `取不到`（不许塌成 `已加载`/`未加载`）：%q", a["state"])
	}
	// ★ 核心负控：没有读数 ⇒ 两格**留空** —— ✗ 不许假装成 `ok`/`0`（那是**编一个数**）。
	if a["last_output_ago_s"] != "" {
		t.Errorf("负控二：没有读数时 `last_output_ago_s` 要留空，实得 %q（不许编 `0`）", a["last_output_ago_s"])
	}
	if a["watchdog_verdict"] != "" {
		t.Errorf("负控二：没有读数时 `watchdog_verdict` 要留空，实得 %q（不许编 `ok`）", a["watchdog_verdict"])
	}
}

// eeggStubBackendsMS01Down —— 子端答不动那一档（合成回据 · 与既有 `eggStubBackends` 同一入口）。
func eeggStubBackendsMS01Down(t *testing.T) {
	t.Helper()
	eggStubBackends(t, map[string]zerg.EggBackendFactForTest{
		"Mr2109": {Reachable: false},
	})
}
