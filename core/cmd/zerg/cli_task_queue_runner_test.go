// cli_task_queue_runner_test.go —— `task pause` / `task resume` / `task retry`（D2 危险档 · 与第一/二片
// 同形的真 runner）的**咽喉 helper** `taskQueueActGuarded` 单测（危险档「已开放」接线批 · 2026-10-09）。
//
// 只测**不触网**的三态（缺目标 / 缺 `--yes` / 干跑计划件）—— 真跑那一态要打主控（非 hermetic），
// 由**两态现读取证**（仓外新制品 + 不存在 id ⇒ 主控 400 码逐字透传）承担，不落本件。
// 咽喉 helper 一旦哑化（漏 missing_target / 漏 yes_required / 端点路径写错），本件当场红。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// tqrRun 直调咽喉 helper（与 `cmdTaskPauseGuarded` / `cmdTaskResumeGuarded` / `cmdTaskRetryGuarded`
// 同入口 · 同原型）。
func tqrRun(action, id string, dryRun, yes bool) (int, string, string) {
	var o, e bytes.Buffer
	inv := &invocation{path: []string{"task", action}, dryRun: dryRun, yes: yes}
	if id != "" {
		inv.args = []string{id}
	}
	rc := taskQueueActGuarded(inv, &o, &e, action)
	return rc, o.String(), e.String()
}

func TestTaskQueueRunnerGuarded(t *testing.T) {
	// 缺任务 id ⇒ 退码 2（fail-closed · 不猜目标）。
	for _, act := range []string{taskActPause, taskActResume, taskActRetry} {
		if rc, _, e := tqrRun(act, "", false, true); rc != 2 || !strings.Contains(e, "要一个任务 id") {
			t.Fatalf("%s 缺目标：rc=%d err=%q（要 rc=2 + 「要一个任务 id」）", act, rc, e)
		}
	}
	// 缺 `--yes` ⇒ 退码 2（D2 fail-closed · 零请求）。
	for _, act := range []string{taskActPause, taskActResume, taskActRetry} {
		if rc, _, e := tqrRun(act, "t-1", false, false); rc != 2 || !strings.Contains(e, "yes_required") {
			t.Fatalf("%s 缺 --yes：rc=%d err=%q（要 rc=2 + yes_required）", act, rc, e)
		}
	}
	// 干跑 ⇒ rc=0 计划件 · 零副作用 · 端点逐字（pause=true / resume=false 同一端点 · retry 另一条现役路由）。
	for _, c := range []struct{ action, want string }{
		{taskActPause, "/api/tasks/t-1/pause?pause=true"},
		{taskActResume, "/api/tasks/t-1/pause?pause=false"},
		{taskActRetry, "/api/tasks/t-1/retry"},
	} {
		rc, o, e := tqrRun(c.action, "t-1", true, false)
		if rc != 0 || !strings.Contains(o, c.want) || !strings.Contains(e, "零副作用") {
			t.Fatalf("%s 干跑：rc=%d out=%q err=%q（要 rc=0 + 端点 %q + 零副作用）", c.action, rc, o, e, c.want)
		}
	}
	// retry 干跑计划件不得混入 pause 端点（防「复用 helper 却抄错路径」）。
	if _, o, _ := tqrRun(taskActRetry, "t-1", true, false); strings.Contains(o, "/pause?") {
		t.Fatalf("retry 干跑端点错：计划件里出现了 pause 端点 ⇒ %q", o)
	}
}
