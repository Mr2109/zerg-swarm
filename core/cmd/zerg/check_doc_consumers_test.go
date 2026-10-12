package main

// check_doc_consumers_test.go —— 判据 `check-doc-consumers` 的成对正负控（✗ 禁恒绿）。
//
// 夹具全落**仓外**（t.TempDir）· 经 `ZERG_DOCS_ALT` 指过去（不碰真机 Zerg-内部文档）。
// ★ 版本目录基名**不钉死**：从唯一真源 `version.Tag`（= `v<当前发布版>`）现算，换版自动跟随。
// 旧病：曾钉死 `v2.5.13` ⇒ 换版静默过期 ⇒ 同 `check_doc_cmds.go` 记的 `GAP-20260926-209` 一类病）。
// 正控：四消费者全落在含发布件的 `version.Tag` 目录 ⇒ rc=0。
// 负控：抽掉该目录的发布件 ⇒ 该格由「符」翻「不符」⇒ rc=1。

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// dcFixture 造一棵仓外文档树：<alt>/Zerg-内部文档/项目文档/<version.Tag>（按需带发布件）。
func dcFixture(t *testing.T, withArtifact bool) {
	t.Helper()
	alt := filepath.Join(t.TempDir(), "Zerg-内部文档")
	d := filepath.Join(alt, "项目文档", version.Tag)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"00-架构-虫族项目详解-x.md", "01-模块-主控core-x.md", "INDEX.md"} {
		if err := os.WriteFile(filepath.Join(d, n), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if withArtifact {
		if err := os.WriteFile(filepath.Join(d, "使用-虫族指南-20260902.md"), []byte("g\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ZERG_DOCS_ALT", alt)
}

// TestDocConsumersGateAllConform — 正控：四消费者同落 version.Tag 且含发布件 ⇒ rc=0。
func TestDocConsumersGateAllConform(t *testing.T) {
	dcFixture(t, true)
	var out, errb bytes.Buffer
	rc := checkDocConsumersGateMain(&out, &errb, nil)
	if rc != exitOK {
		t.Fatalf("期望 rc=0（四消费者全符）· 实得 rc=%d\nout=%s\nerr=%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), version.Tag) {
		t.Fatalf("out 未打印消费者目录 %s：%s", version.Tag, out.String())
	}
	if !strings.Contains(out.String(), "不符 0/4") {
		t.Fatalf("out 结论不是「不符 0/4」：%s", out.String())
	}
}

// TestDocConsumersGateNegativeControl — 负控：抽掉 version.Tag 的发布件 ⇒ 该格变不符 ⇒ rc=1。
func TestDocConsumersGateNegativeControl(t *testing.T) {
	dcFixture(t, false)
	var out, errb bytes.Buffer
	rc := checkDocConsumersGateMain(&out, &errb, nil)
	if rc != dcRCReport {
		t.Fatalf("负控期望 rc=1（无发布件 ⇒ 不符）· 实得 rc=%d\nout=%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "不符") {
		t.Fatalf("负控未报「不符」：%s", out.String())
	}
}

// TestDocConsumersSelfTest — `--self-test` 自身必绿（含成对负控格）。
func TestDocConsumersSelfTest(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkDocConsumersGateMain(&out, &errb, []string{"--self-test"})
	if rc != exitOK {
		t.Fatalf("自检应 rc=0 · 实得 rc=%d\n%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "0 失败") {
		t.Fatalf("自检未全绿：%s", out.String())
	}
}

// TestDocConsumersDivergenceReported — 跨消费者不一致必须**报**（✗ 不静默）。
// ★ 四消费者同源收敛后（doc_search 亦经 currentVersionDir()）不再靠 version.Tag 分叉，故分叉由
// 「**现行指针指向无发布件目录**」造：zerg_overview 取现行指针**不看发布件**（定格该目录）；
// help export / doc meta fill / doc_search 经 devDocsConsumable 要求含发布件 ⇒ 判不了 ⇒ 必报分叉。
func TestDocConsumersDivergenceReported(t *testing.T) {
	alt := filepath.Join(t.TempDir(), "Zerg-内部文档")
	base := filepath.Join(alt, "项目文档")
	// 「现行」指针所指 = version.Tag 目录（**无发布件**）；另造一枚含发布件的 v1.0.0 作旁证（均非分叉源）。
	dOld := filepath.Join(base, "v1.0.0")
	dCur := filepath.Join(base, version.Tag)
	for _, d := range []string{dOld, dCur} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(d, "INDEX.md"), []byte("x\n"), 0o644)
	}
	_ = os.WriteFile(filepath.Join(dOld, "使用-虫族指南-20260902.md"), []byte("g\n"), 0o644)
	// ★ 现行指针（文本件首行 = 目录名）⇒ currentVersionDir() 定格 version.Tag（无发布件）。
	if err := os.WriteFile(filepath.Join(base, "现行"), []byte(version.Tag+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_DOCS_ALT", alt)

	div, seq := dcDiverge(dcMustRead(t))
	if !div {
		t.Fatalf("期望报分叉 · 实得不分叉（%s）", seq)
	}
	var out, errb bytes.Buffer
	_ = checkDocConsumersGateMain(&out, &errb, nil)
	if !strings.Contains(out.String(), "跨消费者不一致") {
		t.Fatalf("out 未报分叉：%s", out.String())
	}
}

func dcMustRead(t *testing.T) []dcReading {
	t.Helper()
	rs, why := dcReadAll()
	if rs == nil {
		t.Fatalf("取源根判不了：%s", why)
	}
	return rs
}
