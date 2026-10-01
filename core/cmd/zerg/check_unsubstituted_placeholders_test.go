// check_unsubstituted_placeholders_test.go —— 门禁件 `check-unsubstituted-placeholders.py` Go 真搬的**成对负控**用例。
//
// 口径（照 M2-0 样板）：
//
//	· 只跑**进程内入口** `unsubstitutedPlaceholdersGateMain`（不接线、不起子进程、不碰真目标）；
//	· 夹具全落 `t.TempDir()`（仓外），**不写仓**；
//	· 每一类**正控 / 负控成对**（同一夹具只改一个变量 ⇒ 一绿一红），✗ 禁恒绿：
//	  正控不绿 ⇒ 门恒红；负控不红 ⇒ 门没牙 —— 两侧都要扎。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func upRunGate(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := unsubstitutedPlaceholdersGateMain(&out, &errb, args)
	return rc, out.String(), errb.String()
}

func upWrite(t *testing.T, root, rel, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		t.Fatalf("造夹具目录失败：%v", err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatalf("造夹具件失败：%v", err)
	}
}

func upMustContain(t *testing.T, what, hay, needle string) {
	t.Helper()
	if !strings.Contains(hay, needle) {
		t.Fatalf("%s：正文里找不到 %q\n---- 实得 ----\n%s", what, needle, hay)
	}
}

func TestUnsubstitutedPlaceholdersGatePort(t *testing.T) {
	// ── 夹具根（仓外 t.TempDir）──────────────────────────────────────────────
	root := t.TempDir()
	upWrite(t, root, "docs/zh/参考-命令行.md", "# 参考\n\n命令 `zerg help` 只读。\n")
	upWrite(t, root, "publish/docs/CLI.zh-CN.md", "> 本版共 118 条已开放命令。\n")

	// ① 正控：干净句面 ⇒ rc 0（门恒红的话这里就红）
	rc, out, errS := upRunGate("--root", root)
	if rc != 0 {
		t.Fatalf("正控-干净句面：期望 rc=0，实得 %d\nstdout=%q\nstderr=%q", rc, out, errS)
	}
	upMustContain(t, "正控-件数", out, "= **2 件**")
	upMustContain(t, "正控-零命中", out, "✓ 命中 **0 处**")

	// ② 负控：句面留一处 `{counts}` ⇒ rc 1 + 逐件逐行点名（门要真咬）
	upWrite(t, root, "publish/docs/CLI.zh-CN.md", "> 本版共 {counts} 条已开放命令。\n")
	rc, out, _ = upRunGate("--root", root)
	if rc != 1 {
		t.Fatalf("负控-句面占位：期望 rc=1，实得 %d\nstdout=%q", rc, out)
	}
	upMustContain(t, "负控-点名", out, "publish/docs/CLI.zh-CN.md:1: {counts}")

	// ③ 成对半边②：同一夹具删掉那一处 ⇒ 回绿（区分度：门不是恒红）
	upWrite(t, root, "publish/docs/CLI.zh-CN.md", "> 本版共 118 条已开放命令。\n")
	if rc, out, _ = upRunGate("--root", root); rc != 0 {
		t.Fatalf("成对半边②-删占位：期望 rc=0，实得 %d\nstdout=%q", rc, out)
	}

	// ④ 假红对照：裸路由模板（`{` 紧跟 `/`）⇒ 0
	upWrite(t, root, "docs/zh/模块-主控.md", "- **舰队**: `GET /api/models/{name}`（单枚模型）\n")
	upWrite(t, root, "docs/zh/模块-主控.md", "- **舰队**: `GET /api/models/{name}`（单枚模型）\n- 裸路由：GET /api/logs/{kind} 读日志\n")
	if rc, out, _ = upRunGate("--root", root); rc != 0 {
		t.Fatalf("假红对照-路由模板：期望 rc=0，实得 %d\nstdout=%q", rc, out)
	}

	// ⑤ 假红对照：围栏内 / 行内代码里的 `{counts}` ⇒ 0；成对半边①：句面再加一处 ⇒ 1
	upWrite(t, root, "docs/zh/code.md", "示例：\n\n```text\n{counts}\n```\n\n行内 `{counts}` 不算。\n")
	if rc, out, _ = upRunGate("--root", root); rc != 0 {
		t.Fatalf("假红对照-代码区：期望 rc=0，实得 %d\nstdout=%q", rc, out)
	}
	upWrite(t, root, "docs/zh/code.md", "示例：\n\n```text\n{counts}\n```\n\n行内 `{counts}` 不算。\n句面漏代：本版共 {counts} 条。\n")
	if rc, out, _ = upRunGate("--root", root); rc != 1 {
		t.Fatalf("成对半边①-句面加一处：期望 rc=1，实得 %d\nstdout=%q", rc, out)
	}

	// ⑥ 空转：判面 0 件 ⇒ 走「⛔ 判不了」分支（**照 py 实测**：judge 返 1、report 丢弃该 rc ⇒ 进程 rc=0）
	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, "docs"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(empty, "publish"), 0o777); err != nil {
		t.Fatal(err)
	}
	rc, out, _ = upRunGate("--root", empty)
	if rc != 0 {
		t.Fatalf("空转-空判面：照 py 实测期望 rc=0（report 丢弃 judge 的 1），实得 %d\nstdout=%q", rc, out)
	}
	upMustContain(t, "空转-判不了行", out, "⛔ 判不了：扫面 **0 件**")

	// ⑦ 取不到输入：根不存在 ⇒ rc 1 + 「找不到仓根」
	rc, out, _ = upRunGate("--root", filepath.Join(root, "no-such-root"))
	if rc != 1 {
		t.Fatalf("取不到输入-缺根：期望 rc=1，实得 %d\nstdout=%q", rc, out)
	}
	upMustContain(t, "取不到输入-正文", out, "⛔ 判不了：找不到仓根")

	// ⑧ 未知旗标 ⇒ rc 2（stderr；stdout 空）
	rc, out, errS = upRunGate("--bogus")
	if rc != 2 || out != "" {
		t.Fatalf("未知旗标：期望 rc=2 且 stdout 空，实得 rc=%d stdout=%q", rc, out)
	}
	upMustContain(t, "未知旗标-stderr", errS, "unrecognized arguments: --bogus")

	// ⑨ `--json`（本门无机器读面）⇒ rc 2 + stderr 逐字
	rc, out, errS = upRunGate("--json")
	if rc != 2 || out != "" {
		t.Fatalf("--json：期望 rc=2 且 stdout 空，实得 rc=%d stdout=%q", rc, out)
	}
	upMustContain(t, "--json-stderr", errS, "⛔ 本门无机器读面：`--json` 在本门**未实现**（缺口 Q-176）")

	// ⑩ `--list-rules` ⇒ rc 0 + 首行 = VERSION
	rc, out, _ = upRunGate("--list-rules")
	if rc != 0 || !strings.HasPrefix(out, upVersion+"\n") {
		t.Fatalf("--list-rules：期望 rc=0 且首行=版本串，实得 rc=%d stdout=%q", rc, out)
	}

	// ⑪ `-h` ⇒ rc 0 + argparse 用法串（逐字，含续行缩进）
	rc, out, _ = upRunGate("-h")
	if rc != 0 || out != upHelp() {
		t.Fatalf("-h：期望 rc=0 且正文逐字=upHelp()，实得 rc=%d stdout=%q", rc, out)
	}

	// ⑫ `--self-test` ⇒ rc 0 + 全过（格 1–7 断言 + 格 8 真树现跑）
	rc, out, _ = upRunGate("--self-test")
	if rc != 0 {
		t.Fatalf("--self-test：期望 rc=0，实得 %d\nstdout=%q", rc, out)
	}
	upMustContain(t, "--self-test-结论", out, "⇒ 自检全过（断言 11 格 · 失败 0 格）")
}
