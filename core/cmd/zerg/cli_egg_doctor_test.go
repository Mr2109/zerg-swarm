// cli_egg_doctor_test.go —— 缺口 `GAP-20261018-13` 的**成对负控 + 三退码**（命令面一侧）。
//
// 为什么这一件必须存在（不是「顺手补测」）：本缺口的病是「两种完全不同的结论被写成同一个词」——
// 只跑真机永远只能看到**今天恰好**那一态。⇒ 三态各造一枚合成件，逐态钉住：
//
//	① 「装过、进程没了」   ⇒ `egg ls` 必须是 `声明在但进程没了`（**新态**，不许并进旧态）
//	② 「声明已加载、从未装上」⇒ `egg ls` 仍是 `已声明但后端不在`（旧态**一字不动** —— 本笔只加不改）
//	③ 「进程在跑、名册没有」⇒ `egg doctor` 必须判不一致并**点名**（第二类负控）
//
// 另外三格钉 `egg doctor` 的**退码约定**：全一致 0 / 有不一致 1 / 判不了 8（三种各真跑一次）。
//
// 口径：全部走 `SetEggBackendProbeLiveForTest`（本进程内的 `httptest` 合成子端）⇒ **不连真网**、
// 不碰任何真服务；主控面用本包既有的 `newSyntheticMaster`（一个真源，不另起夹具）。
package main_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// eggChildStub 起一个**只答 `/eggs`** 的合成子端，并把探测口接到它（真解析路径）——
// 判据件因此能喂**原始载荷**（含 `residency[]`）而不碰真机。
func eggChildStub(t *testing.T, payload string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eggs" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(srv.Close)
	_, portStr, ok := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	if !ok {
		t.Fatalf("合成子端地址形状变了：%s", srv.URL)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("合成子端端口解析不了（%s）：%v", portStr, err)
	}
	t.Cleanup(zerg.SetEggBackendProbeLiveForTest("127.0.0.1", port))
}

// eggOneHostMaster 一台机（Mr2109）一枚卵的合成主控：`status` 由调用方给（这就是「声明」）。
func eggOneHostMaster(t *testing.T, status string) {
	t.Helper()
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models": `{"count":1,"models":[{"id":"Egg-A","host":"Mr2109","backend":"llama-server","mem_gb":6}]}`,
		"/api/fleet/status": `{"machines":{"Mr2109":{"machine":"Mr2109","models":[]}}}`,
		"/api/models/Egg-A": fmt.Sprintf(`{"name":"Egg-A","host":"Mr2109","status":%q}`, status),
	})
	t.Cleanup(srv.Close)
}

// ── ① 新态：账本记 vanished ⇒ `egg ls` 必须出 `声明在但进程没了` ───────────────────────────

func TestEggLsProcessVanishedIsItsOwnState(t *testing.T) {
	eggOneHostMaster(t, "已加载")
	// 子端：**没有**在跑的卵（`eggs` 空），但驻留账本记着它曾经装上、pid 已不在活。
	eggChildStub(t, `{"eggs":[],"residency":[
	  {"egg_id":"Egg-A","model":"Egg-A","pid":4242,"port":9001,"unit":"zerg-egg-a",
	   "state":"vanished","loaded_at":"2026-10-18T01:00:00Z","last_seen":"2026-10-18T02:00:00Z",
	   "detected_at":"2026-10-18T02:00:02Z","reason":"process_vanished","port_live":false}]}`)

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	if !strings.Contains(stdout, "Mr2109\tEgg-A\t声明在但进程没了") {
		t.Fatalf("账本记 vanished 的卵必须落 `声明在但进程没了`（**新态**）：stdout=%q", stdout)
	}
	if strings.Contains(stdout, "Mr2109\tEgg-A\t已声明但后端不在") {
		t.Fatalf("**本缺口的原现象**：装过而进程没了的卵被并进「已声明但后端不在」（两种不同结论又混成一个词）：%q", stdout)
	}
}

// ── ② 旧态不动：账本里没有这一枚 ⇒ 仍是 `已声明但后端不在`（本笔只加不改）─────────────────

func TestEggLsDeclaredNeverInstalledKeepsOldState(t *testing.T) {
	eggOneHostMaster(t, "已加载")
	// 子端答得动、但 `eggs` 与 `residency` 都空（= 从未装上 / 已显式卸载）。
	eggChildStub(t, `{"eggs":[],"residency":[]}`)

	rc, stdout, stderr := runEggCmd(t, "egg", "ls", "--plain")
	if rc != 0 {
		t.Fatalf("egg ls 退码 = %d（要 0）· stderr=%s", rc, stderr)
	}
	if !strings.Contains(stdout, "Mr2109\tEgg-A\t已声明但后端不在") {
		t.Fatalf("没有装载证据的卵仍要落旧态 `已声明但后端不在`（不许冒充「进程没了」）：stdout=%q", stdout)
	}
	if strings.Contains(stdout, "声明在但进程没了") {
		t.Fatalf("账本里没有这一枚 ⇒ 不许判「进程没了」（那是拿缺席冒充结论））：%q", stdout)
	}
}

// ── ③ 体检退码：有不一致 ⇒ 1（并点名「进程没了」＋ 最后见到活着的时刻）────────────────────

func TestEggDoctorProcessGoneIsInconsistentExitOne(t *testing.T) {
	eggOneHostMaster(t, "已加载")
	eggChildStub(t, `{"eggs":[],"residency":[
	  {"egg_id":"Egg-A","model":"Egg-A","pid":4242,"port":9001,"state":"vanished",
	   "loaded_at":"2026-10-18T01:00:00Z","last_seen":"2026-10-18T02:00:00Z","reason":"process_vanished"}]}`)

	rc, stdout, stderr := runEggCmd(t, "egg", "doctor", "--machine", "Mr2109", "--plain")
	if rc != 1 {
		t.Fatalf("有不一致 ⇒ 退码必须是 1，实得 %d · stdout=%q stderr=%s", rc, stdout, stderr)
	}
	for _, want := range []string{"不一致", "进程没了", "2026-10-18T02:00:00Z", "判词：不一致（退码 1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("体检面缺一格 %q（要能一句话说清「服务掉了」）· stdout=%q", want, stdout)
		}
	}
}

// ── ③' 第二类负控：进程在跑而名册里没有它 ⇒ 也必须判不一致 ───────────────────────────────

func TestEggDoctorRunningButNotInRosterIsInconsistent(t *testing.T) {
	// 名册只有 Egg-A（主控声明**未加载**）；子端却在 9002 上跑着 `Ghost-X`。
	eggOneHostMaster(t, "未加载")
	eggChildStub(t, `{"eggs":[
	  {"egg_id":"Ghost-X","model":"Ghost-X","state":"ready","port":9002,"pid":7777,"managed":true,"gtt_gb":12.5}],
	  "residency":[{"egg_id":"Ghost-X","model":"Ghost-X","pid":7777,"port":9002,"state":"loaded",
	   "loaded_at":"2026-10-18T03:00:00Z","last_seen":"2026-10-18T03:10:00Z","port_live":true}]}`)

	rc, stdout, stderr := runEggCmd(t, "egg", "doctor", "--machine", "Mr2109", "--plain")
	if rc != 1 {
		t.Fatalf("进程在跑而名册没有 ⇒ 退码必须是 1，实得 %d · stdout=%q stderr=%s", rc, stdout, stderr)
	}
	if !strings.Contains(stdout, "名册里没有这一枚") {
		t.Fatalf("必须**点名**「进程在跑而名册没有」（声明面漏项）：stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "Ghost-X") {
		t.Fatalf("点名要落到卵名 `Ghost-X`：stdout=%q", stdout)
	}
}

// ── ③'' 全一致 ⇒ 0（正控：三面对得上时不许判红）────────────────────────────────────────

func TestEggDoctorAllConsistentExitsZero(t *testing.T) {
	eggOneHostMaster(t, "已加载")
	eggChildStub(t, `{"eggs":[
	  {"egg_id":"Egg-A","model":"Egg-A","state":"ready","port":9001,"pid":1111,"managed":true,"gtt_gb":7.5}],
	  "residency":[{"egg_id":"Egg-A","model":"Egg-A","pid":1111,"port":9001,"state":"loaded",
	   "loaded_at":"2026-10-18T04:00:00Z","last_seen":"2026-10-18T04:10:00Z","port_live":true}]}`)

	rc, stdout, stderr := runEggCmd(t, "egg", "doctor", "--machine", "Mr2109", "--plain")
	if rc != 0 {
		t.Fatalf("全一致 ⇒ 退码必须是 0，实得 %d · stdout=%q stderr=%s", rc, stdout, stderr)
	}
	if !strings.Contains(stdout, "不一致 0 项") || !strings.Contains(stdout, "判词：一致（退码 0") {
		t.Fatalf("全一致时人面要给出「不一致 0 项 + 判词 一致」：stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "在(pid=1111)") {
		t.Fatalf("进程态那一格要能看见 pid（体检面「凭什么判」）：stdout=%q", stdout)
	}
}

// ── ③''' 判不了 ⇒ 8（子端打不到 ⇒ 进程/端口两面没有任何证据）────────────────────────────

func TestEggDoctorUnreadableChildExitsEight(t *testing.T) {
	eggOneHostMaster(t, "已加载")
	// 不给合成子端：探测口回落真实现，而 Mr2109 的子端地址在名册里 ⇒ 打不到 ⇒ `取不到`。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	_, portStr, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	t.Cleanup(zerg.SetEggBackendProbeLiveForTest("127.0.0.1", port))

	rc, stdout, stderr := runEggCmd(t, "egg", "doctor", "--machine", "Mr2109", "--plain")
	if rc != 8 {
		t.Fatalf("子端读不到 ⇒ 退码必须是 8（判不了），实得 %d · stdout=%q stderr=%s", rc, stdout, stderr)
	}
	if !strings.Contains(stdout, "判不了") {
		t.Fatalf("人面要逐行落 `判不了`（不许把读不到说成一致）：stdout=%q", stdout)
	}
}
