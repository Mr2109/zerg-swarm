// doc_consumers.go —— 只读命令 `zerg doc consumers`（账 `GAP-20260927-129` ④
// 「把『定格』做成可现算读数」）。
//
// 命题：四个**消费判据**各自解析出的「版本目录」是一份**可现算读数**，逐消费者输出
// `consumer⇥dir⇥is_v*⇥has_release_artifact`，让人一眼看出**定格是否分叉**。
//
// 与各消费者的**真身同源**（✗ 不另立解析规则）：
//
//	· zerg_overview  ⇒ `chat.OverviewCurrentVersionDir()`（同源包装 currentVersionDir()：无 `现行` 指针 ⇒ 回退 latestVersionDir()）
//	· help export    ⇒ `devDocsCurrentVersionDir("")`（export.go 的同一处取源）
//	· doc meta fill  ⇒ `devDocsCurrentVersionDir("")`（family_doc.go 的 docFillRoot 同一处取源）
//	· doc_search     ⇒ `chat.OverviewCurrentVersionDir()`（chat_tool_docsearch.go 的 fail-closed 判据同源：
//	                     现行指针优先 → 缺则含发布件最高版；✗ 不再钉 `version.Tag` —— 旧钉法在收版中间态必判不了）
//
// 三条硬口径：
//
//	① **只读**：不写盘、不起子进程、不联网、不改任何消费者行为（本件只现读）；
//	② **缺件不静默**：版本档案取源根不在盘上 ⇒ 退 8 + 点名候选（✗ 不把「读不到」当「没有」）；
//	③ 判定口径与判据 `check-doc-consumers` **共用** `docConsumersRead()`（✗ 不两处各写一套）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// dcVStarRE —— 版本目录基名的形态判据（`^v[0-9.]+$`）。
var dcVStarRE = regexp.MustCompile(`^v[0-9.]+$`)

// dcStableLandingName —— **去版本号稳定落点**名（= `chat.StableLandingName`：`现行` · 一处真源，
// ✗ 不在本包另写字面量）。缺口 `GAP-20260927-121` 落点面。
var dcStableLandingName = chat.StableLandingName

// dcConsumersOrder —— 消费者现读次序（固定 · 人面可读）。
var dcConsumersOrder = []string{"zerg_overview", "help export", "doc meta fill", "doc_search"}

// dcReading —— 一个消费者现读的「定格」读数。
type dcReading struct {
	Consumer  string // 消费者名（人面）
	Dir       string // 现读解析出的版本目录（绝对路径；判不了 ⇒ 空）
	Why       string // 判不了的原因（空 = 有结论）
	OnDisk    bool   // 目录在盘上（且是目录）
	IsVStar   bool   // 基名匹配 ^v[0-9.]+$
	IsStable  bool   // 基名 = 稳定落点名（`现行` —— **去版本号** · 缺口 GAP-20260927-121 落点面）
	HasRelArt bool   // 含发布件（使用指南 glob）
}

// dcJudgeOne —— 对一个「解析出的目录」做三格判定（在盘 / is_v* / 含发布件）。
// baseDir 未用时保留形参位以便自检直接喂夹具；dir == "" ⇒ 三格皆不成立（并保留 why）。
func dcJudgeOne(name, dir, why string) dcReading {
	r := dcReading{Consumer: name, Dir: dir, Why: why}
	if dir == "" {
		return r
	}
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		r.OnDisk = true
	}
	r.IsVStar = dcVStarRE.MatchString(filepath.Base(dir))
	r.IsStable = filepath.Base(dir) == chat.StableLandingName
	if r.OnDisk {
		if m, _ := filepath.Glob(filepath.Join(dir, chat.ReleaseArtifactGlob)); len(m) > 0 {
			r.HasRelArt = true
		}
	}
	return r
}

// dcResolve —— 逐消费者现读「解析出的版本目录」：返回 consumer ⇒ (dir, why)。
// 取源根判不了 ⇒ (nil, 原因)。
func dcResolve() (map[string][2]string, string) {
	base := devDocsBase()
	if base == "" {
		return nil, statepath.DocsBaseMissingNote()
	}
	heDir, heWhy := devDocsConsumablePinnedDir("")
	dsDir, dsWhy := devDocsConsumable(chat.OverviewCurrentVersionDir(), "") // 同源 chat_tool_docsearch.go 的 fail-closed 判据
	return map[string][2]string{
		"zerg_overview": {chat.OverviewCurrentVersionDir(), ""},
		"help export":   {heDir, heWhy},
		"doc meta fill": {heDir, heWhy},
		"doc_search":    {dsDir, dsWhy},
	}, ""
}

// dcReadAll —— 现读全部消费者定格（含 per-格判定）。
// 返回 (读数, "") 或 (nil, 取源根判不了的原因)。
func dcReadAll() ([]dcReading, string) {
	res, why := dcResolve()
	if res == nil {
		return nil, why
	}
	out := make([]dcReading, 0, len(dcConsumersOrder))
	for _, c := range dcConsumersOrder {
		pair := res[c]
		out = append(out, dcJudgeOne(c, pair[0], pair[1]))
	}
	return out, ""
}

// dcBool —— 判定格的呈现（人面用）：是/否。
func dcBool(v bool) string {
	if v {
		return "是"
	}
	return "否"
}

// dcConforms —— 一个消费者「符公约」=（基名 `v*` ∨ **去版本号稳定落点**）∧ 在盘 ∧ 含发布件。
// ★ 缺口 `GAP-20260927-121` 落点面（2026-10-14）：稳定面「唯一份」的落点由带版本号的 `v*` 目录
// 去版本号为 `<base>/现行/`（版本只当元数据、落点路径不随版本变）⇒ 「符公约」多认这一格；
// 旧带版本号落点（`v*`）**照旧算符**（兼容面：已在盘的 `v*` 目录仍可直读 · ✗ 不静默丢）。
func dcConforms(r dcReading) bool { return (r.IsVStar || r.IsStable) && r.OnDisk && r.HasRelArt }

// docConsumersFields —— `--json` 的全部字段（K1：机器面先定）。
var docConsumersFields = []string{"consumer", "dir", "is_vstar", "on_disk", "has_release_artifact", "note"}

// cmdDocConsumers —— `zerg doc consumers`：只读列四消费者的定格读数。
//
// 形态 `zerg doc consumers [--json <字段>]`；退码 0 读到 · 8 取源根判不了（不给结论）。
func cmdDocConsumers(inv *invocation, stdout, stderr io.Writer) int {
	readings, why := dcReadAll()
	if readings == nil {
		inv.setErr("blocked", "docs_base_missing", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 退码 8（判不了 · 不许当绿）\n", progName, why)
		return exitBlocked
	}
	rows := make([]map[string]string, 0, len(readings))
	for _, r := range readings {
		rows = append(rows, map[string]string{
			"consumer":             r.Consumer,
			"dir":                  r.Dir,
			"is_vstar":             dcBool(r.IsVStar),
			"on_disk":              dcBool(r.OnDisk),
			"has_release_artifact": dcBool(r.HasRelArt),
			"note":                 r.Why,
		})
	}
	if !inv.jsonGiven {
		fmt.Fprintf(stderr, "%s: 版本档案取源根 = %s（消费者定格现读 · 逐消费者 dir 可比出分叉）\n", progName, devDocsBase())
	}
	return listCmd(inv, stdout, stderr, []string{"consumer", "dir", "is_vstar", "has_release_artifact"}, rows)
}
