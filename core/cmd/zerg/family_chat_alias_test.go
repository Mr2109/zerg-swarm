package main

// family_chat_alias_test.go —— `zerg chat` 机器正门与**会话别名**的三格牙（2026-10-17 本枚）：
//
//	① 别名表的**文件锁**：两条任务线并发写同一份共件 ⇒ 一条都不许丢（✗ 不许 last wins）；
//	② `--prompt-file`：整件读入 —— 中文与换行**逐字节**保留（✗ 不按行拼、✗ 不 Trim）；
//	③ 来源互斥：`--prompt-file` 与位置参数同给 ⇒ **用法错 2**（✗ 不许静默取一个）。
//
// 变异自证（删掉/改坏哪一处必红）：
//
//	· 把 `chatAliasMutate` 的排他锁去掉、改成裸「读 → 改 → 写」⇒ ① 必红（并发丢条目）；
//	· 把 `--prompt-file` 分支改成 `strings.Fields`/`TrimSpace` 拼串 ⇒ ② 必红（换行/前后空白丢）；
//	· 删掉 `fileGiven && len(inv.args) > 0` 那条判据 ⇒ ③ 必红（退码从 2 变 0）。
//
// 隔离纪律：状态目录一律指 `t.TempDir()`（`ZERG_STATE_DIR`）⇒ 不碰真机 `~/.zerg/state`。

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// 与 `chatNormalizeSessionRef` 的完整 id 形状逐字同形（夹具用 · ✗ 不依赖服务端）。
const testAliasSID = "chat_20261017_010203_abcdef"

// TestChatAliasMutateSerializesConcurrentWriters —— ①：两条任务线**同时**新建会话 ⇒ 两个别名都在。
//
// 这是别名表唯一的硬约束：文件是**共件**。去掉排他锁后，「读旧快照 → 改 → 整份写回」会让后写者
// 覆盖前写者 ⇒ 下面这条 `len(m) == 条目总数` 的断言必红（回调里那 1ms 睡眠就是把竞争窗口撑开）。
func TestChatAliasMutateSerializesConcurrentWriters(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const lines, per = 2, 20
	var wg sync.WaitGroup
	errs := make(chan error, lines)
	for i := 0; i < lines; i++ {
		name := "自测线" + strconv.Itoa(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < per; k++ {
				key := name + "_" + strconv.Itoa(k)
				err := chatAliasMutate(func(m map[string]string) error {
					// ★ 把「读 → 改 → 写」之间的窗口撑开 —— 无锁实现必在此处交错。
					time.Sleep(time.Millisecond)
					m[key] = testAliasSID
					return nil
				})
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("并发写别名表失败：%v", err)
	}

	m, err := chatAliasSnapshot()
	if err != nil {
		t.Fatalf("读回别名表失败：%v", err)
	}
	if len(m) != lines*per {
		t.Fatalf("并发写丢了条目：期望 %d 条，实得 %d 条（后写者覆盖前写者 = last wins）", lines*per, len(m))
	}
	for i := 0; i < lines; i++ {
		for k := 0; k < per; k++ {
			key := "自测线" + strconv.Itoa(i) + "_" + strconv.Itoa(k)
			if m[key] != testAliasSID {
				t.Fatalf("条目 %q 丢失或串了：%q", key, m[key])
			}
		}
	}

	// 落盘件必须是**完整可解析**的一份（无锁写的另一半坏法是半截 JSON）。
	b, err := os.ReadFile(chatAliasPath())
	if err != nil {
		t.Fatalf("读别名表件失败：%v", err)
	}
	var onDisk map[string]string
	if err := json.Unmarshal(b, &onDisk); err != nil {
		t.Fatalf("别名表件不是合法 JSON（并发写出半截）：%v\n%s", err, string(b))
	}
	if len(onDisk) != lines*per {
		t.Fatalf("盘上条目数 %d ≠ 期望 %d", len(onDisk), lines*per)
	}
	// 锁件必须真落在同一目录（有锁 ≠ 只在注释里说锁）。
	if _, err := os.Stat(chatAliasPath() + ".lock"); err != nil {
		t.Fatalf("别名锁件不存在（锁没落盘）：%v", err)
	}
}

// TestChatAliasSetLsRmRoundTrip —— 别名三条子命令的闭环：set ⇒ ls 看得到 ⇒ rm ⇒ ls 没了 ⇒ 再 rm 退 2。
func TestChatAliasSetLsRmRoundTrip(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	mk := func(args []string, kv map[string][]string) *invocation {
		return &invocation{path: []string{"chat", "alias"}, args: args, kv: kv}
	}

	var out, errb bytes.Buffer
	if rc := cmdChatAliasSet(mk([]string{"清缺口"}, map[string][]string{"--session": {testAliasSID}}), &out, &errb); rc != exitOK {
		t.Fatalf("alias set 应退 0，实得 %d（stderr=%s）", rc, errb.String())
	}
	out.Reset()
	errb.Reset()
	if rc := cmdChatAliasLs(mk(nil, nil), &out, &errb); rc != exitOK {
		t.Fatalf("alias ls 应退 0，实得 %d", rc)
	}
	if !strings.Contains(out.String(), "清缺口") || !strings.Contains(out.String(), testAliasSID) {
		t.Fatalf("alias ls 看不到刚登记的那一条：%q", out.String())
	}

	// 第二格：同一个名字改指另一个会话（改名/重指 = 覆盖，不是新增一条）。
	const other = "chat_20261017_020304_abcdef"
	out.Reset()
	errb.Reset()
	if rc := cmdChatAliasSet(mk([]string{"清缺口"}, map[string][]string{"--session": {other}}), &out, &errb); rc != exitOK {
		t.Fatalf("alias set 覆盖应退 0，实得 %d", rc)
	}
	snap, err := chatAliasSnapshot()
	if err != nil || len(snap) != 1 || snap["清缺口"] != other {
		t.Fatalf("覆盖语义不对：%v（err=%v）", snap, err)
	}

	out.Reset()
	errb.Reset()
	if rc := cmdChatAliasRm(mk([]string{"清缺口"}, nil), &out, &errb); rc != exitOK {
		t.Fatalf("alias rm 应退 0，实得 %d", rc)
	}
	snap, _ = chatAliasSnapshot()
	if len(snap) != 0 {
		t.Fatalf("alias rm 之后表里还有 %d 条：%v", len(snap), snap)
	}

	// ★ 负控：删不存在的名字 ⇒ **2**（✗ 不许「删成功了」骗人）。
	out.Reset()
	errb.Reset()
	if rc := cmdChatAliasRm(mk([]string{"清缺口"}, nil), &out, &errb); rc != exitCodeOf("usage") {
		t.Fatalf("rm 不存在名应退 %d，实得 %d", exitCodeOf("usage"), rc)
	}
	if !strings.Contains(errb.String(), "alias_not_found") {
		t.Fatalf("rm 不存在名没点名 detail=alias_not_found：%q", errb.String())
	}

	// 形状闸：带空白的别名一律 2（✗ 不许落一条 shell 拆不开的名字）。
	out.Reset()
	errb.Reset()
	if rc := cmdChatAliasSet(mk([]string{"两 个字"}, map[string][]string{"--session": {testAliasSID}}), &out, &errb); rc != exitCodeOf("usage") {
		t.Fatalf("含空白的别名应退 %d，实得 %d", exitCodeOf("usage"), rc)
	}
}

// TestChatPromptFileKeepsChineseAndNewlines —— ②：整件读入（中文 / 换行 / 行内反引号逐字节保留）。
func TestChatPromptFileKeepsChineseAndNewlines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "q.md")
	body := "第一行：清缺口 GAP-20261017-17\n\n第二行：含 ` 反引号 ` 与 $HOME 变量\n   缩进行\n末行（无结尾换行）"
	if err := os.WriteFile(p, []byte(body+"\n"), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
	inv := &invocation{path: []string{"chat", "send"}, kv: map[string][]string{"--prompt-file": {p}}}
	got, rc := chatPromptFromInv(inv, io.Discard)
	if rc != exitOK {
		t.Fatalf("`--prompt-file` 应退 0，实得 %d", rc)
	}
	if got != body {
		t.Fatalf("`--prompt-file` 读入不逐字节相同：\n 得=%q\n 期望=%q", got, body)
	}
	if !strings.Contains(got, "\n\n") || !strings.Contains(got, "\n   缩进行") {
		t.Fatalf("换行/缩进被处理掉了（✗ 不许按行拼）：%q", got)
	}
}

// TestChatPromptSourcesAreExclusive —— ③：三选一互斥 —— 同给 ⇒ 用法错 2；路径不存在 ⇒ 2。
func TestChatPromptSourcesAreExclusive(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "q.md")
	if err := os.WriteFile(p, []byte("题\n"), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
	// ① 与位置参数同给
	inv := &invocation{args: []string{"直接写在命令行上的题"}, kv: map[string][]string{"--prompt-file": {p}}}
	var errb bytes.Buffer
	if _, rc := chatPromptFromInv(inv, &errb); rc != exitCodeOf("usage") {
		t.Fatalf("`--prompt-file` 与位置参数同给应退 %d，实得 %d", exitCodeOf("usage"), rc)
	}
	if !strings.Contains(errb.String(), "prompt_source_conflict") {
		t.Fatalf("同给没点名 detail=prompt_source_conflict：%q", errb.String())
	}
	// ② 路径不存在（★ `error.kind=usage`，✗ 不落 8）
	errb.Reset()
	inv = &invocation{kv: map[string][]string{"--prompt-file": {filepath.Join(dir, "不存在.md")}}}
	if _, rc := chatPromptFromInv(inv, &errb); rc != exitCodeOf("usage") {
		t.Fatalf("路径不存在应退 %d，实得 %d", exitCodeOf("usage"), rc)
	}
	if !strings.Contains(errb.String(), "error.kind=usage") || !strings.Contains(errb.String(), "prompt_file_unreadable") {
		t.Fatalf("路径不存在没落 usage/prompt_file_unreadable：%q", errb.String())
	}
	// ③ 空路径
	errb.Reset()
	inv = &invocation{kv: map[string][]string{"--prompt-file": {""}}}
	if _, rc := chatPromptFromInv(inv, &errb); rc != exitCodeOf("usage") {
		t.Fatalf("`--prompt-file` 空值应退 %d，实得 %d", exitCodeOf("usage"), rc)
	}
	// ④ 空白件（只有换行）
	blank := filepath.Join(dir, "blank.md")
	if err := os.WriteFile(blank, []byte("\n\n\n"), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
	errb.Reset()
	inv = &invocation{kv: map[string][]string{"--prompt-file": {blank}}}
	if _, rc := chatPromptFromInv(inv, &errb); rc != exitCodeOf("usage") {
		t.Fatalf("空白提示应退 %d，实得 %d", exitCodeOf("usage"), rc)
	}
}

// TestChatAskRequiresAliasAndRejectsSessionMix —— `chat ask` 的两条用法闸（✗ 不碰网络：
// 两条判都发生在任何请求之前 ⇒ 闭端口也能判 · 与门⑪「零 HTTP 依赖」同口径）。
func TestChatAskRequiresAliasAndRejectsSessionMix(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	var out, errb bytes.Buffer

	// ① 缺 `--as` ⇒ 2（✗ 不回落 --last：那会把两条线接到同一个会话）
	inv := &invocation{path: []string{"chat", "ask"}, args: []string{"题"}}
	if rc := cmdChatAsk(inv, &out, &errb); rc != exitCodeOf("usage") {
		t.Fatalf("缺 --as 应退 %d，实得 %d", exitCodeOf("usage"), rc)
	}
	if !strings.Contains(errb.String(), "missing_alias") {
		t.Fatalf("缺 --as 没点名 detail=missing_alias：%q", errb.String())
	}

	// ② `--as` 与 `--session` 同给 ⇒ 2
	out.Reset()
	errb.Reset()
	inv = &invocation{
		path: []string{"chat", "ask"}, args: []string{"题"},
		kv: map[string][]string{"--as": {"自测线甲"}, "--session": {testAliasSID}},
	}
	if rc := cmdChatAsk(inv, &out, &errb); rc != exitCodeOf("usage") {
		t.Fatalf("--as 与 --session 同给应退 %d，实得 %d", exitCodeOf("usage"), rc)
	}
	if !strings.Contains(errb.String(), "alias_and_session_conflict") {
		t.Fatalf("同给没点名 detail=alias_and_session_conflict：%q", errb.String())
	}

	// ③ 别名形状不合法 ⇒ 2
	out.Reset()
	errb.Reset()
	inv = &invocation{path: []string{"chat", "ask"}, args: []string{"题"},
		kv: map[string][]string{"--as": {" "}}}
	if rc := cmdChatAsk(inv, &out, &errb); rc != exitCodeOf("usage") {
		t.Fatalf("空别名应退 %d，实得 %d", exitCodeOf("usage"), rc)
	}
}
