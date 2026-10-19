// cli_repo_commit_unknown_flag_test.go —— 缺口 GAP-20260928-13 的判据件。
//
// 病（改前实测）：`zerg repo commit --confirm=x` 里的 `--confirm` 是 `dev edit` 的旗标，
// `repo commit` 根本不认识；解析层把它记进 inv.unknown（main.go:4805 逐字「非透传命令要据此报错」），
// 但 cmdRepoCommit 从不看它 ⇒ 静默吞掉，一路走到「要一枚 --message」，把调用方引到无关的错处。
//
// 治法：入口处先判 inv.unknown，非空即具名报「未知旗标 <首个>` 并退 exitUsage（2）。
// 两态：① 带未知旗标 ⇒ 报未知旗标（不是报缺 --message）；② 不带 ⇒ 仍报缺 --message（零回归）。
package main

import (
	"strings"
	"testing"
)

func TestRepoCommitUnknownFlagNamedFirst(t *testing.T) {
	var out, errb strings.Builder
	inv := &invocation{path: []string{"repo", "commit"}, unknown: []string{"--confirm=x"}}
	rc := cmdRepoCommit(inv, &out, &errb)

	if rc != exitUsage {
		t.Fatalf("未知旗标应退 %d，实得 %d（stderr=%q）", exitUsage, rc, errb.String())
	}
	msg := errb.String()
	if !strings.Contains(msg, "未知旗标") {
		t.Fatalf("报话未点名「未知旗标」：%q", msg)
	}
	if !strings.Contains(msg, "--confirm") {
		t.Fatalf("报话未点名具体旗标 `--confirm`：%q", msg)
	}
	// 反向断言：不能再把调用方引到 --message 上（那正是本缺口要治的错处）
	if strings.Contains(msg, "要一枚") {
		t.Fatalf("未知旗标被吞掉后仍报缺 --message（旧病复发）：%q", msg)
	}
}

func TestRepoCommitNoMessageUnchanged(t *testing.T) {
	var out, errb strings.Builder
	inv := &invocation{path: []string{"repo", "commit"}}
	rc := cmdRepoCommit(inv, &out, &errb)

	if rc != exitUsage {
		t.Fatalf("缺 --message 应退 %d，实得 %d", exitUsage, rc)
	}
	msg := errb.String()
	if !strings.Contains(msg, "--message") {
		t.Fatalf("缺 --message 的报话被改坏了：%q", msg)
	}
	if strings.Contains(msg, "未知旗标") {
		t.Fatalf("没有未知旗标却报未知旗标（假阳性）：%q", msg)
	}
}
