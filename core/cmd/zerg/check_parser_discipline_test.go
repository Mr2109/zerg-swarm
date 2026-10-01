// check_parser_discipline_test.go —— `check_parser_discipline.go`（M2-0 路B续3 真搬）的定向单测。
//
// 判据形态：**成对负控**（正控绿 / must-fail 红 / 成对回绿 / 空转不给结论），
// 每条断言都在断言**入口读数**（rc + stdout 逐字），哑化任何一条判据 ⇒ 对应用例 FAIL。
//
// 纪律：夹具一律落**临时目录**（不碰真仓）· 只读真仓做现读格 · 无 subprocess。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pdRunGate —— 进程内复用入口取 (rc, stdout, stderr)。
func pdRunGate(argv ...string) (int, string, string) {
	var o, e bytes.Buffer
	rc := parserDisciplineGateMain(&o, &e, argv)
	return rc, o.String(), e.String()
}

// pdMkFace —— 在临时根下造 `<root>/scripts/…` 面，返回根。
func pdMkFace(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("夹具建目录失败：%v", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("夹具写件失败：%v", err)
		}
	}
	return root
}

func TestCheckParserDisciplineGatePort(t *testing.T) {
	// ── ① 正控：解析器形态（模式不以 `-` 起头）⇒ rc=0 ──────────────────────────
	clean := pdMkFace(t, map[string]string{
		"scripts/gates/ok.sh": "#!/bin/bash\nX=$(printf '%s' \"$S\" | " + "grep -oE '[0-9]+' | head -1)\n",
	})
	rc, out, errS := pdRunGate(clean)
	if rc != 0 || errS != "" {
		t.Fatalf("正控失败：期望 rc=0/stderr 空，实得 rc=%d stderr=%q out=%q", rc, errS, out)
	}
	if !strings.Contains(out, "⇒ 抓取/解析面全过（rc=0）：1 件里 0 处「破折号模式」0 处「贪婪剥标签」") {
		t.Fatalf("正控结论行不符：%q", out)
	}

	// ── ② 负控：破折号模式（不以 `-e`/`--` 起）⇒ rc=1 且逐条点名件:行 ─────────
	bad := pdMkFace(t, map[string]string{
		"scripts/gates/bad.sh": "#!/bin/bash\nN=$(" + "grep -o '--strict-report' \"$F\" | wc -l)\n",
	})
	rc, out, _ = pdRunGate(bad)
	if rc != 1 {
		t.Fatalf("负控①（破折号模式）失败：期望 rc=1，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "✗ scripts/gates/bad.sh:2  [P1]  --strict-report") {
		t.Fatalf("负控①未点名：%q", out)
	}

	// ── ②′ 成对回绿：同一模式加显式分界符 `-e` ⇒ 回绿 ─────────────────────────
	green := pdMkFace(t, map[string]string{
		"scripts/gates/ok2.sh": "#!/bin/bash\nN=$(" + "grep -o -e '--strict-report' \"$F\" | wc -l)\n",
	})
	rc, out, _ = pdRunGate(green)
	if rc != 0 {
		t.Fatalf("成对回绿失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	}

	// ── ③ 负控：贪婪剥标签 `sed 's/<[^>]*>//g'` ⇒ rc=1（P2）────────────────────
	bad2 := pdMkFace(t, map[string]string{
		"scripts/gates/bad2.sh": "#!/bin/bash\nT=$(sed 's/" + "<[^>]*>//g' \"$F\")\n",
	})
	rc, out, _ = pdRunGate(bad2)
	if rc != 1 || !strings.Contains(out, "[P2]  s/<[^>]*>//g") {
		t.Fatalf("负控②（贪婪剥标签）失败：期望 rc=1 且点 P2，实得 rc=%d out=%q", rc, out)
	}

	// ── ③′ 成对回绿：普通 sed 替换（非剥标签）⇒ rc=0 ─────────────────────────
	green2 := pdMkFace(t, map[string]string{
		"scripts/gates/ok3.sh": "#!/bin/bash\nT=$(sed 's/x/y/g' \"$F\")\n",
	})
	rc, out, _ = pdRunGate(green2)
	if rc != 0 {
		t.Fatalf("成对回绿②失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	}

	// ── ③″ 真话负控：把那一行**注释掉** ⇒ 回绿（命中的确实是那一行，不是噪声）──
	commented := pdMkFace(t, map[string]string{
		"scripts/gates/ok4.sh": "#!/bin/bash\n# N=$(" + "grep -o '--strict-report' \"$F\" | wc -l)\n",
	})
	rc, out, _ = pdRunGate(commented)
	if rc != 0 {
		t.Fatalf("真话负控失败：注释掉那行应回绿，实得 rc=%d out=%q", rc, out)
	}

	// ── ④ 负控：数据字面 vs 真命令位（证判据没被放宽）───────────────────────
	dataLit := pdMkFace(t, map[string]string{
		"scripts/gates/data.py": "#!/usr/bin/env python3\nCONSUMERS = (\n    \".MatchString(\", \"" + "grep -\" ,\n)\n",
	})
	rc, out, _ = pdRunGate(dataLit)
	if rc != 0 {
		t.Fatalf("负控③（数据字面不应判红）失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	}
	realPos := pdMkFace(t, map[string]string{
		"scripts/gates/real.sh": "#!/bin/bash\n" + "grep -o '--x' f\n",
	})
	rc, _, _ = pdRunGate(realPos)
	if rc != 1 {
		t.Fatalf("负控③′（同一形态落真命令位 ⇒ 照旧红）失败：期望 rc=1，实得 rc=%d", rc)
	}

	// ── ⑤ 空转（扫描面 0 件）⇒ rc=2（不给结论 · 不许当绿）──────────────────
	empty := t.TempDir()
	rc, out, _ = pdRunGate(empty)
	if rc != 2 || !strings.Contains(out, "⇒ 扫描面 0 件（不给结论 · 「没扫到」不等于「扫过且干净」）") {
		t.Fatalf("空转负控失败：期望 rc=2 且逐字理由，实得 rc=%d out=%q", rc, out)
	}

	// ── ⑥ 用法错：未知旗标 ⇒ rc=2 ＋ argparse usage/error 正文 ──────────────
	rc, out, errS = pdRunGate("--no-such-flag")
	if rc != 2 || !strings.Contains(errS, pdUsage) ||
		!strings.Contains(errS, "check-parser-discipline.py: error: unrecognized arguments: --no-such-flag\n") {
		t.Fatalf("用法错负控失败：rc=%d stderr=%q", rc, errS)
	}

	// ── ⑦ `--json` 机器读面（唯一真源：items/meta 键序与缩进）───────────────
	rc, out, _ = pdRunGate("--json", bad)
	if rc != 1 {
		t.Fatalf("--json 格失败：期望 rc=1（透传），实得 rc=%d", rc)
	}
	for _, want := range []string{"\"schema\": \"zerg-gate/1\"", "\"kind\": \"ParserDiscipline\"",
		"\"line\": 2", "\"kind\": \"P1\"", "\"scanned\": 1", "\"hits\": 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("--json 格缺 %q：%q", want, out)
		}
	}

	// ── ⑧ `--list-forms` / `--self-test` / `-h` 三档 ────────────────────────
	rc, out, _ = pdRunGate("--list-forms")
	if rc != 0 || !strings.Contains(out, "⇒ 用解析器（`html.parser` / `xml.etree`）或先划显式分界符") {
		t.Fatalf("--list-forms 格失败：rc=%d out=%q", rc, out)
	}
	rc, out, _ = pdRunGate("--self-test")
	if rc != 0 || !strings.Contains(out, "自检结论: 全过（断言 17 条 · 失败 0 条）") {
		t.Fatalf("--self-test 格失败：期望 17 条全过 rc=0，实得 rc=%d out=%q", rc, out)
	}
	rc, out, errS = pdRunGate("-h")
	if rc != 0 || errS != "" || !strings.Contains(out, "show this help message and exit") {
		t.Fatalf("-h 格失败：rc=%d stderr=%q", rc, errS)
	}

	// ── ⑨ 真仓现读（显式传根 · 只读）⇒ rc=0 且扫面件数 > 0 ──────────────────
	repo := "<repo>"
	if fi, err := os.Stat(repo); err == nil && fi.IsDir() {
		rc, out, errS = pdRunGate(repo)
		if rc != 0 || errS != "" {
			t.Fatalf("真仓现读失败：期望 rc=0/stderr 空，实得 rc=%d stderr=%q out=%q", rc, errS, out)
		}
		if !strings.Contains(out, "⇒ 抓取/解析面全过（rc=0）：") {
			t.Fatalf("真仓现读结论行不符：%q", out)
		}
	}
}
