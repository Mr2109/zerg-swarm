// devdocs_test.go —— 「版本档案目录」取源的判据（缺口 `G-17` ③ + `G-19` 同根那处）。
//
// 四格 + 一条静态自证（都是**合成树**，不碰真 `Zerg-内部文档`；隔离靠 `ZERG_WORKSPACE`/`ZERG_DOCS_ALT`）：
//
//	① 正控：`v2.5.9`(3 篇) · `v2.5.10`(5 篇) · `v2.6`(2 篇 = 空壳) ⇒ 选 **v2.5.10**（版本号最大**且**非空壳）；
//	② 成对负控：把 `v2.5.10` 也弄成空壳（抽到 2 篇）⇒ 选 `v2.5.9` —— 两条判据（版本号最大 / 非空壳）都在咬；
//	③ 显式钉版：`pinVersion` 给了 ⇒ 就是它（**兼容「钉死在某一版」的旧行为**），空壳也认（人说了算）；
//	④ 缺件不静默：取源根不在盘上 ⇒ 返回 `""` + 点名**两个候选绝对路径**（不许猜一个目录出来）；
//	⑤ 静态自证：解析面三个文件里**零版本号字面量**（注释不计）+ `"项目文档"` 字面量**只在一处**（取源唯一）。
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// devDocsFixture —— 造一棵假的「项目文档」取源根：返回它的绝对路径。
// dirs 里每个版本目录按篇数造 `*.md`（内容无所谓，取源只看篇数）。
func devDocsFixture(t *testing.T, dirs map[string]int) string {
	t.Helper()
	root := t.TempDir()
	alt := t.TempDir()
	t.Setenv("ZERG_WORKSPACE", root) // 仓内根：没有 docs/项目文档 ⇒ 落到仓外根
	t.Setenv("ZERG_DOCS_ALT", alt)
	base := filepath.Join(alt, "项目文档")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	for d, n := range dirs {
		p := filepath.Join(base, d)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < n; i++ {
			f := filepath.Join(p, "件"+strings.Repeat("x", i+1)+".md")
			if err := os.WriteFile(f, []byte("# 夹具\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return base
}

// ① 正控：未钉版 ⇒ 钉到 `v<当前发布版>`（读 version.Version，不是盘上最大的）；
//
//	③ 显式钉版：`pinVersion` 给了 ⇒ 就是它（兼容「钉死在某一版」的旧行为），空壳也认（人说了算）。
func TestDevDocsCurrentVersionDir_PinsToCurrentVersionAndHonoursPin(t *testing.T) {
	// 造一棵假的「项目文档」取源根：除当前版外再放两个「更高/更低」的版本目录 ——
	// 它们存在但**都不许**被默认档选中（旧实现会挑盘上最大的）。
	base := devDocsFixture(t, map[string]int{
		"v0.0.1":              3,
		"v9.9.9":              5, // 盘上版本号最大，但**不是**当前发布版 ⇒ 不许当选
		"v" + version.Version: 2, // 当前版：哪怕只有 2 篇（空壳）也认（人/真源说了算）
	})

	got, why := devDocsCurrentVersionDir("")
	if why != "" {
		t.Fatalf("正控：不该有 why，实得 %q", why)
	}
	if want := filepath.Join(base, "v"+version.Version); got != want {
		t.Fatalf("正控：未钉版应钉到当前发布版 %s，实得 %q（盘上最大的 v9.9.9 不许当选）", want, got)
	}

	// 钉版：人说了算，空壳也认。
	got, why = devDocsCurrentVersionDir(version.Version)
	if why != "" {
		t.Fatalf("钉版：不该有 why，实得 %q", why)
	}
	if want := filepath.Join(base, "v"+version.Version); got != want {
		t.Fatalf("钉版：`--docs-ver <当前版>` 应钉到 %s，实得 %q", want, got)
	}

	// 钉一个不存在的版本 ⇒ 不给结论（点名缺件路径）。
	if _, why = devDocsCurrentVersionDir("0.0.0"); why == "" {
		t.Fatal("钉一个不存在的版本竟然通过了（不给结论才对）")
	}
}

// ④ 缺件不静默：取源根不在盘上 ⇒ "" + 两个候选绝对路径都点名。
func TestDevDocsCurrentVersionDir_MissingBaseIsNotSilent(t *testing.T) {
	t.Setenv("ZERG_WORKSPACE", t.TempDir())
	t.Setenv("ZERG_DOCS_ALT", filepath.Join(t.TempDir(), "nope"))

	got, why := devDocsCurrentVersionDir("")
	if got != "" {
		t.Fatalf("取源根不在盘上时不该给出目录，实得 %q", got)
	}
	for _, want := range []string{"仓内根", "仓外根"} {
		if !strings.Contains(why, want) {
			t.Fatalf("缺件说明没点名 %s：%q", want, why)
		}
	}
}

// ⑤ 静态自证：**版本无关**（解析面零版本号字面量）+ **一处取源**（本包**零处** `"项目文档"` 拼装 —— 全走 `statepath.DocsBase()`）。
// 为什么只看「带引号的字面量」：注释里引旧代码的原样句是**记账**，不是硬编码；字符串字面量才是真源。
func TestDevDocsResolutionHasNoVersionLiteralAndSingleSource(t *testing.T) {
	reVer := regexp.MustCompile(`"v[0-9][0-9]*\.[0-9]`)
	files := []string{"devdocs.go", "export.go", "family_doc.go"}
	baseHits := map[string]int{}
	delegates := false
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读不到 %s：%v（go test 的 cwd 就是包目录）", f, err)
		}
		for i, ln := range strings.Split(string(b), "\n") {
			trimmed := strings.TrimSpace(ln)
			if strings.HasPrefix(trimmed, "//") {
				continue // 注释：记账允许
			}
			if m := reVer.FindString(ln); m != "" {
				t.Errorf("%s:%d 出现**版本号字面量** %s —— 解析面必须版本无关（树里 `v2.5.11`/`v2.6` 这类目录自动新增）", f, i+1, m)
			}
			baseHits[f] += strings.Count(ln, `"项目文档"`)
			if f == "devdocs.go" && strings.Contains(ln, "statepath.DocsBase()") {
				delegates = true
			}
		}
	}
	// 取源唯一：**本包一处都不许自己拼** `<…>/项目文档` —— 真源在 `statepath.DocsBase()`（门禁/工具同口径）。
	total := 0
	for _, n := range baseHits {
		total += n
	}
	if total != 0 {
		t.Fatalf("`\"项目文档\"` 字面量应**零处**（取源唯一 = 全走 statepath.DocsBase()）：共 %d 处，分布 %v", total, baseHits)
	}
	if !delegates {
		t.Fatal("devdocs.go 里没找到 `statepath.DocsBase()` 调用 —— 取源被换掉了？")
	}
}
