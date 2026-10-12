// devdocs.go —— 开发文档面「版本档案目录」的**唯一取源**（缺口 `G-17` + `G-19` 同根的那一处）。
//
// 病根（两条缺口同一个根 · 逐字现读）：
//
//	· `G-17` 导出的默认落点 = `filepath.Join(root, "..", "Zerg-内部文档", "项目文档", "v2.5.10")`
//	  （旧 `export.go:32`）—— **版本号写在命令里** ⇒ 落到**上一版**；
//	· `G-19` 被扫根 = 同一句硬编码（旧 `family_doc.go:233`）⇒ 本版新件**一件都不在扫描面**。
//
// 本件把「取源根 + 当前版目录」收成**一处**（两条命令都调它，不留第二处硬编码）：
//
//	① 取源根 = `statepath.DocsBase()`（**唯一真源**已在那儿：仓内 `<仓库根>/docs/项目文档` 优先、
//	   缺则 `<ZERG_DOCS_ALT|同级 Zerg-内部文档>/项目文档`）—— 本文件**不另立第二套**换根规则；
//	② 当前版目录 = `v<当前发布版>`（读 `version.Version`，只读不改）—— 默认档跟着收版走；
//	   钉版只认 `--docs-ver`（兼容旧「钉死某一版」行为）。版本真源唯一 = `version.Version`。
//
// 三条硬口径：
//
//	① **零版本字面量**：本文件里没有 `v2.5.x` 这类常量（版本目录自动新增也不漂 —— `AGENTS.md:52` 逐字「勿用示例版本号」）；
//	② **缺件不静默**：取源根不在盘上 ⇒ 返回 `""` + `statepath.DocsBaseMissingNote()`（点名两个候选绝对路径）；
//	   根在、但一个合格版本目录都没有 ⇒ 也返回 `""` + 逐条点名候选与篇数，**不猜**一个目录出来；
//	③ **显式旗标可钉版**：`--docs-ver <X.Y.Z>` 或 `--scope <目录>` ⇒ 走旧行为（钉某一版），默认档才是版本无关。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// ★ 2026-09-29（GAP-20260927-36 收口）：此处原有常量 `devDocsMinDocs`（值 `3` · 「不是空壳」下限）——
// 该常量**零消费点**（全仓零引用；`devDocsCurrentVersionDir` 只看版本目录在不在，不看篇数）⇒ 空壳常量、守卫无牙。
// 本件按缺口二选一里的**删除**一支收口。为什么不选「接消费点」：本包**已签**判据 `devdocs_test.go:58`
// 逐字钉住「当前版目录哪怕只有 2 篇（空壳）也认（人/真源说了算）」⇒ 在 `devDocsCurrentVersionDir()` 里
// 加「篇数 < 3 ⇒ 退非 0」会**推翻那条已签判据**（而 `devdocs_test.go` 不在本单可改之列）。
// 同批死件 `devDocsVersionKey` / `devDocsCompareVersion` / `devDocsCountMD` 于 2026-10-02 同批删除
//（全仓零引用 · 删后 `strconv` 导入一并摘除）。

// devDocsBase —— 版本档案（`<项目文档>`）取源根：**唯一真源 = statepath.DocsBase()**。
func devDocsBase() string { return statepath.DocsBase() }

// devDocsCurrentVersionDir —— 当前版目录（**版本无关**：现算，源码里 0 个版本号字面量）。
//
//	pinVersion == ""  ⇒ **与 `zerg_overview` 同源**：「现行」指针优先，缺则回退「含发布件的最高版」；两者皆无 ⇒ 退回 `v<当前发布版>`（版本真源 = `version.Version`）；
//	pinVersion != ""  ⇒ 钉到 `v<pinVersion>`（显式旗标，兼容旧行为）；不存在 ⇒ 报出来（不给结论）。
//
// ★ 版本真源 = `version.Version`：未钉版时**钉到 `v<当前发布版>`**（读 `core/internal/version/version.go`
// 的 Version 常量，**只读不改**）—— 默认档跟着收版走，不再是「盘上版本号最大的那个」（旧实现会挑到
// `v2.6` 这类空壳或已退版的目录）。钉版只认 `--docs-ver`（兼容钉死旧行为）。
//
// 返回 (绝对路径, ""); 判不了 ⇒ ("", 原因)。**原因里点名候选**（缺件不静默）。
func devDocsCurrentVersionDir(pinVersion string) (dir, why string) {
	base := devDocsBase()
	if base == "" {
		return "", statepath.DocsBaseMissingNote()
	}
	// 钉版（`--docs-ver`）⇒ 用它的值；未钉版 ⇒ 见下（默认档与 `zerg_overview` 同源）。
	pinned := strings.TrimPrefix(strings.TrimSpace(pinVersion), "v")
	if pinned == "" {
		// ★ 缺口 GAP-20260927-121：默认档与 `zerg_overview` **同源** —— 「现行」指针优先
		// （chat.currentVersionDir），缺则回退「含发布件的最高版」（chat.latestVersionDir）；
		// 两者皆无（无指针 ∧ 盘上无已发布版）⇒ 退回旧档 v<version.Version>
		// （兼容空壳/夹具环境 —— 已签判据 devdocs_test.go 逐字钉住这一档）。
		if d := chat.OverviewCurrentVersionDir(); d != "" {
			return d, ""
		}
		pinned = version.Version
	}
	p := filepath.Join(base, "v"+pinned)
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		// 缺件不静默：点名绝对路径（缺件/判不了 ⇒ 退 8 + 打印缺件路径）。
		return "", fmt.Sprintf("版本目录不在盘上（版本真源 v%s）：%s", pinned, p)
	}
	return p, ""
}

// devDocsHasReleaseArtifact —— 钉版目录里有没有「发布件」（使用指南）？
// 判据 glob 复用 chat 的**唯一真源**常量 `chat.ReleaseArtifactGlob`（✗ 不在本包另写字面量；与
// `zerg_overview` / `doc_search` / 判据 `check-doc-consumers` 同一口径）。
func devDocsHasReleaseArtifact(dir string) bool {
	if dir == "" {
		return false
	}
	m, _ := filepath.Glob(filepath.Join(dir, chat.ReleaseArtifactGlob))
	return len(m) > 0
}

// devDocsConsumable —— 消费者「可消费」判定（缺口 `GAP-20260927-121`）：钉版目录 **在盘 ∧ 含发布件** 才给结论。
//
//	existWhy != ""    ⇒ 目录本身判不了（不在盘等）⇒ **原样透传**那条原因（不覆盖既有档位理由）；
//	目录在盘但无发布件 ⇒ **不给结论** + 逐字明报「当前版目录 <d> 无发布件，故本消费者取不到」。
//
// 档位由**调用方按自己同族既有档位**给（本函数只决定「给不给结论 + 文案」，✗ 不新造档位）。
//
// ★ 为什么不并进 devDocsCurrentVersionDir：那条函数的契约被**已签判据** `devdocs_test.go` 逐字钉住
//
//	（合成夹具的当前版目录只有 2 篇 md、**无发布件**，仍须返回 `(dir, "")`）—— 故在此另立一格，不动那条。
func devDocsConsumable(dir, existWhy string) (string, string) {
	if existWhy != "" {
		return "", existWhy
	}
	if !devDocsHasReleaseArtifact(dir) {
		return "", fmt.Sprintf("当前版目录 %s 无发布件，故本消费者取不到（缺发布件 %s）", dir, chat.ReleaseArtifactGlob)
	}
	return dir, ""
}

// devDocsConsumablePinnedDir —— `devDocsCurrentVersionDir` 的**消费者版**：钉版后再过一遍可消费判定。
// `help export` 与 `doc meta fill` 两处取源都改调本函数（钉版语义一字不改，只多「无发布件 ⇒ 不给结论」这一格）。
func devDocsConsumablePinnedDir(pinVersion string) (dir, why string) {
	d, w := devDocsCurrentVersionDir(pinVersion)
	return devDocsConsumable(d, w)
}
