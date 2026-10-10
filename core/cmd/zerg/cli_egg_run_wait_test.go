// cli_egg_run_wait_test.go —— 缺口 `GAP-20260925-44`（命令面 · 「就绪等待 / 异步查询」）的契约测试。
//
// 落点：外部测试包 `package main_test`（与命令面其余契约测试同一层）；合成主控复用本包既有的
// `newSyntheticMaster`（`fusion_test.go`）；子端真后端回据复用 `eggStubBackends`（`cli_egg_projection_test.go`）。
//
// 背景：`POST /api/control/load` 是**异步受理**（回 `accepted:true + task_id`，冷装在后台跑）⇒
// 只读那枚 `ok:true` **分不清**「引擎还在冷装」与「引擎起不来」。两段落点：
//
//	(A) `egg run --wait`：拿 `task_id` 轮询只读面 `GET /api/core/status` 的 `load_tasks` 给**结论**；
//	(B) `egg ls`：真后端不在、但主控有一枚在装 ⇒ `state` 落第五档 `装载中`。
//
// 判据：正控（正常形态可判/可读、逐字相符）＋ 负控（缺格 ⇒ 按既有档拒并点名 · 取不到 ⇒ 不给结论非 0）。
package main_test

import (
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// eggWaitFleetJSON / eggWaitStatusJSON —— 本判据件的最小名册：Egg-A 单机候选 Mr2109。
// （不复用 `eggFleetJSON` —— 那份有三枚卵，会把这几个用例的断言搅浑。）
const eggWaitFleetJSON = `{"count":1,"models":[{"id":"Egg-A","host":"Mr2109","backend":"llama-server","mem_gb":6}]}`
const eggWaitStatusJSON = `{"machines":{"Mr2109":{"machine":"Mr2109","models":[]}}}`

// eggWaitLoadAccepted —— 一份「异步受理」的 `POST /api/control/load` 响应（任务号 T1）。
const eggWaitLoadAccepted = `{"ok":true,"accepted":true,"task_id":"T1","started_at":"2026-10-10T12:00:00Z","machine":"Mr2109","model":"Egg-A"}`

// ---- (A) `egg run --wait`：就绪等待面 ----------------------------------------------------------

// 正控：受理体给了 `task_id`，只读面报 `phase=ok` ⇒ `egg run --wait` 必须落**就绪**（退 0），
// 而不是停在「受理体回了 ok」那一档（那正是本缺口的症状）。
func TestEggRunWaitReadyOK(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggWaitFleetJSON,
		"/api/control/load": eggWaitLoadAccepted,
		"/api/core/status":  `{"ok":true,"load_tasks":[{"task_id":"T1","machine":"Mr2109","model":"Egg-A","phase":"ok","backend":"llama-server","port":9400}]}`,
	})
	defer srv.Close()

	rc, _, stderr := runEggCmd(t, "egg", "run", "Egg-A@Mr2109", "--wait", "--timeout", "30s", "--yes")
	if rc != 0 {
		t.Fatalf("`egg run --wait`（phase=ok）退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	if !strings.Contains(stderr, "就绪") {
		t.Errorf("`--wait` 拿到 phase=ok ⇒ 必须给「就绪」结论：stderr=%s", stderr)
	}
}

// 正控：`--no-wait` ⇒ **拿句柄就走**（受理体的 `task_id` 打进 stderr），不阻塞、不给「就绪」。
func TestEggRunNoWaitReturnsHandle(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggWaitFleetJSON,
		"/api/control/load": eggWaitLoadAccepted,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"未加载"}`,
		"/api/core/status":  `{"ok":true,"load_tasks":[{"task_id":"T1","machine":"Mr2109","model":"Egg-A","phase":"accepted"}]}`,
	})
	defer srv.Close()

	rc, _, stderr := runEggCmd(t, "egg", "run", "Egg-A@Mr2109", "--no-wait", "--yes")
	if rc != 0 {
		t.Fatalf("`egg run --no-wait` 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	if !strings.Contains(stderr, "task_id") && !strings.Contains(stderr, "任务号") {
		t.Errorf("`--no-wait` 要回**句柄**（task_id）：stderr=%s", stderr)
	}
	if strings.Contains(stderr, "就绪") {
		t.Errorf("`--no-wait` **不许**落「就绪」（它没等）：stderr=%s", stderr)
	}
}

// 负控①（缺格 ⇒ 按既有档拒并点名）：`--wait` 缺 `--timeout` ⇒ **先于任何网络**退 2，点名那枚旗标。
func TestEggRunWaitRequiresTimeout(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggWaitFleetJSON,
		"/api/control/load": eggWaitLoadAccepted,
	})
	defer srv.Close()

	rc, stdout, stderr := runEggCmd(t, "egg", "run", "Egg-A@Mr2109", "--wait", "--yes")
	if rc != 2 {
		t.Fatalf("`--wait` 缺 `--timeout` ⇒ 退码 2（用法错），实际 rc=%d · stderr=%s", rc, stderr)
	}
	if stdout != "" {
		t.Errorf("用法错时 stdout 必须 0 字节：%q", stdout)
	}
	if !strings.Contains(stderr, "--timeout") {
		t.Errorf("判词要**点名**缺的那枚旗标 `--timeout`：stderr=%s", stderr)
	}
}

// 负控②（坏结果 ⇒ 非 0）：受理体给了 `task_id`，只读面报 `phase=failed` ⇒ `egg run --wait`
// 落**装载失败**（非 0），✗ 不当成功（本缺口：起不来不许读成起来了）。
func TestEggRunWaitFailedIsNotSuccess(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggWaitFleetJSON,
		"/api/control/load": eggWaitLoadAccepted,
		"/api/core/status":  `{"ok":true,"load_tasks":[{"task_id":"T1","machine":"Mr2109","model":"Egg-A","phase":"failed","error":"oom"}]}`,
	})
	defer srv.Close()

	rc, _, stderr := runEggCmd(t, "egg", "run", "Egg-A@Mr2109", "--wait", "--timeout", "30s", "--yes")
	if rc == 0 {
		t.Fatalf("`--wait` 拿到 phase=failed ⇒ **不许**退 0（起不来不许读成起来了）· stderr=%s", stderr)
	}
	if !strings.Contains(stderr, "失败") {
		t.Errorf("要逐字说白 `装载失败`：stderr=%s", stderr)
	}
}

// 负控③（取不到 ⇒ 不给结论，非 0）：受理体给了 `task_id`，但只读面**查不到**这枚任务号
// ⇒ 退 8，✗ 不许静默当「装好了」、也不许当「还在装」空转。
func TestEggRunWaitTaskAbsentIsNoConclusion(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggWaitFleetJSON,
		"/api/control/load": eggWaitLoadAccepted,
		"/api/core/status":  `{"ok":true,"load_tasks":[]}`,
	})
	defer srv.Close()

	rc, stdout, stderr := runEggCmd(t, "egg", "run", "Egg-A@Mr2109", "--wait", "--timeout", "30s", "--yes")
	if rc != 8 {
		t.Fatalf("任务号查不到 ⇒ 退码 8（不给结论），实际 rc=%d · stdout=%q · stderr=%s", rc, stdout, stderr)
	}
	if !strings.Contains(stderr, "不给结论") {
		t.Errorf("要逐字说白 `不给结论`：stderr=%s", stderr)
	}
}

// 负控④（取不到 ⇒ 不给结论）：受理体里**没有** `task_id`（旧主控）⇒ 就绪等待拿不到跟谁 ⇒ 退 8。
func TestEggRunWaitNoHandleInAcceptedIsNoConclusion(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggWaitFleetJSON,
		"/api/control/load": `{"ok":true,"accepted":true}`,
	})
	defer srv.Close()

	rc, _, stderr := runEggCmd(t, "egg", "run", "Egg-A@Mr2109", "--wait", "--timeout", "30s", "--yes")
	if rc != 8 {
		t.Fatalf("受理体没有 task_id ⇒ 退码 8（不给结论），实际 rc=%d · stderr=%s", rc, stderr)
	}
	if !strings.Contains(stderr, "task_id") {
		t.Errorf("判词要点名缺的是 `task_id`：stderr=%s", stderr)
	}
}

// ---- (B) `egg ls`：第五档 `装载中` ------------------------------------------------------------

// 正控：真后端不在（子端 `/eggs` 没有它）、主控也**没**声「已加载」，但只读面有一枚
// **在装**（`phase=accepted`）的 Egg-A@Mr2109 ⇒ 那一格必须是第五档 `装载中`
// （✗ 不许落 `未加载` ——「还在冷装」与「没在跑」必须分得开，本缺口的复现面）。
func TestEggLsShowsLoadingFifthState(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggWaitFleetJSON,
		"/api/fleet/status": eggWaitStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"未加载"}`,
		"/api/core/status":  `{"ok":true,"load_tasks":[{"task_id":"T1","machine":"Mr2109","model":"Egg-A","phase":"accepted"}]}`,
	})
	defer srv.Close()
	// 子端答得动、但 Egg-A 没在跑（真后端不在）⇒ 本来会落 `未加载`；有在装的任务 ⇒ 必须落 `装载中`。
	eggStubBackends(t, map[string]zerg.EggBackendFactForTest{"Mr2109": {Reachable: true}})

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	if !strings.Contains(stdout, "Mr2109\tEgg-A\t装载中") {
		t.Errorf("真后端不在而主控有一枚在装 ⇒ 必须落第五档 `装载中`：%q", stdout)
	}
	if strings.Contains(stdout, "Mr2109\tEgg-A\t未加载") {
		t.Errorf("「装载中」**不许**塌成 `未加载`（本缺口的复现面）：%q", stdout)
	}
}

// 负控：只读面里那枚在装的任务是**别的模型** ⇒ Egg-A@Mr2109 那一格**不许**落 `装载中`
// （避免「按机名粗配」把不相干的在装任务也算到这一格）。
func TestEggLsLoadingScopedToMachineAndModel(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": eggWaitFleetJSON,
		"/api/fleet/status": eggWaitStatusJSON,
		"/api/models/Egg-A": `{"name":"Egg-A","host":"Mr2109","status":"未加载"}`,
		// 在装的不是 Egg-A（是 Egg-B @ Mr2109）⇒ 不许算到 Egg-A 头上。
		"/api/core/status": `{"ok":true,"load_tasks":[{"task_id":"T9","machine":"Mr2109","model":"Egg-B","phase":"accepted"}]}`,
	})
	defer srv.Close()
	eggStubBackends(t, map[string]zerg.EggBackendFactForTest{"Mr2109": {Reachable: true}})

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	if strings.Contains(stdout, "装载中") {
		t.Errorf("在装的是别的模型 ⇒ 这一格**不许**落 `装载中`：%q", stdout)
	}
	if !strings.Contains(stdout, "Mr2109\tEgg-A\t未加载") {
		t.Errorf("没有在装那一枚要照旧落 `未加载`：%q", stdout)
	}
}
