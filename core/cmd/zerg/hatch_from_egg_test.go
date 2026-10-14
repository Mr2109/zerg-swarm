package main

import (
	"bytes"
	"testing"
)

// TestHatchFromEggSelfTest —— 「从单文件孵」自检口（四态 ＋ 两条负控 ＋ 逐格读数）。
//
// ★ 本件**无 CLI 开关**（✗ 不新造旗标、✗ 不动命令树）⇒ 自检走测试件面，同
// `eggstore.SelfTestNote`「自检口 = SelfTest()（测试件面跑）」的既有先例。
func TestHatchFromEggSelfTest(t *testing.T) {
	var out, errb bytes.Buffer
	rc := hfeSelfTest(&out, &errb)
	if rc != 0 {
		t.Fatalf("自检 rc=%d\nstdout:\n%s\nstderr:\n%s", rc, out.String(), errb.String())
	}
	t.Logf("自检档：\n%s", out.String())
}
