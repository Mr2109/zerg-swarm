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
//	② 当前版目录 = 上面那根下**版本号最大、且非递归 `*.md` ≥ 3 篇**的 `v*` 目录 ——
//	   判据逐字取自仓内成文规矩（`AGENTS.md:52`：「**取版本号最大、且 ≥3 篇的目录**（…<3 篇 = 空壳…`v2.6` 就是这种）」）。
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
	"sort"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// devDocsMinDocs —— 「不是空壳」的下限：非递归 `*.md` ≥ 本数（`AGENTS.md:52` 逐字「且 ≥3 篇」）。
// 为什么要有它：`v2.6` 那类**只有 2 篇**的目录版本号最大，但它是历史壳（同 core 侧
// `zerg_overview` 的「<3 篇 = 空壳」守卫）—— 挑它当「当前版」会把落点与扫描面一起挑歪。
const devDocsMinDocs = 3

// devDocsBase —— 版本档案（`<项目文档>`）取源根：**唯一真源 = statepath.DocsBase()**。
func devDocsBase() string { return statepath.DocsBase() }

// devDocsCurrentVersionDir —— 当前版目录（**版本无关**：现算，源码里 0 个版本号字面量）。
//
//	pinVersion == ""  ⇒ 版本档案取源根下「版本号最大 + 非递归 md ≥3 篇」的那个 `v*` 目录；
//	pinVersion != ""  ⇒ 钉到 `v<pinVersion>`（显式旗标，兼容旧行为）；不存在 ⇒ 报出来（不给结论）。
//
// 返回 (绝对路径, ""); 判不了 ⇒ ("", 原因)。**原因里点名候选**（缺件不静默）。
func devDocsCurrentVersionDir(pinVersion string) (dir, why string) {
	base := devDocsBase()
	if base == "" {
		return "", statepath.DocsBaseMissingNote()
	}
	if pinVersion != "" {
		p := filepath.Join(base, "v"+strings.TrimPrefix(strings.TrimSpace(pinVersion), "v"))
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			return "", fmt.Sprintf("`--docs-ver %s` 指的版本目录不在盘上：%s", pinVersion, p)
		}
		return p, ""
	}
	dirs, err := filepath.Glob(filepath.Join(base, "v*"))
	if err != nil || len(dirs) == 0 {
		return "", fmt.Sprintf("版本档案取源根 %s 下没有 `v*` 版本目录（现状：%d 个候选）", base, len(dirs))
	}
	// 候选：只看**目录**且名字能解成 3 段/2 段版本号（解不动的记进说明，不当候选）。
	type cand struct {
		dir string
		key []int
		md  int
	}
	cands := make([]cand, 0, len(dirs))
	skipped := make([]string, 0, len(dirs))
	for _, d := range dirs {
		st, err := os.Stat(d)
		if err != nil || !st.IsDir() {
			continue
		}
		key := devDocsVersionKey(filepath.Base(d))
		if key == nil {
			skipped = append(skipped, filepath.Base(d)+"(版本号解不动)")
			continue
		}
		cands = append(cands, cand{dir: d, key: key, md: devDocsCountMD(d)})
	}
	// 版本号降序 —— 高版在前（`AGENTS.md:52`「取版本号最大」）。
	sort.Slice(cands, func(i, j int) bool { return devDocsCompareVersion(cands[i].key, cands[j].key) > 0 })
	for _, c := range cands {
		if c.md >= devDocsMinDocs {
			return c.dir, ""
		}
	}
	// 一个都不合格：逐条点名（篇数 + 空壳判语），**不猜**。
	detail := make([]string, 0, len(cands)+len(skipped))
	for _, c := range cands {
		note := "合格"
		if c.md < devDocsMinDocs {
			note = fmt.Sprintf("空壳（非递归 md %d 篇 < %d）", c.md, devDocsMinDocs)
		}
		detail = append(detail, filepath.Base(c.dir)+"="+note)
	}
	detail = append(detail, skipped...)
	return "", fmt.Sprintf("版本档案取源根 %s 下没有合格版本目录（%s）",
		base, strings.Join(detail, " · "))
}

// devDocsVersionKey —— `v2.5.11` → `[2,5,11]`；`v2.6` → `[2,6]`；解不动 ⇒ nil（不当候选、也不乱比）。
func devDocsVersionKey(name string) []int {
	s := strings.TrimPrefix(strings.TrimSpace(name), "v")
	if s == "" {
		return nil
	}
	s = strings.SplitN(s, "-", 2)[0] // 容忍 `v2.5.7-xxx`
	parts := strings.Split(s, ".")
	key := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		key = append(key, n)
	}
	return key
}

// devDocsCompareVersion —— 逐段比大小（缺段按 0 补：`2.6` 比 `2.5.11` 大，与 semver 口径同）。
func devDocsCompareVersion(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		ai, bi := 0, 0
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if ai != bi {
			return ai - bi
		}
	}
	return 0
}

// devDocsCountMD —— 目录下**非递归**的 `*.md` 篇数（隐藏件不计 · 与 `AGENTS.md:52` 的「篇」同口径）。
func devDocsCountMD(dir string) int {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			n++
		}
	}
	return n
}
