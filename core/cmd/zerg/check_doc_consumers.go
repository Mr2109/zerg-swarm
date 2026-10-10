// check_doc_consumers.go —— 判据 `check-doc-consumers`（**Go 直落 · 不建 py** · 账
// `GAP-20260927-129` ④ · 承公约 `Zerg-内部文档/项目文档/v2.5.10/公约-稳定面与版本目录口径-20261008.md` §一）。
//
// 命题（公约 §一 逐字）：任一消费判据（zerg_overview / help export / doc meta fill / doc_search）
// 解析出的版本目录，必须是盘上**已存在**的 `v*` 目录、且含发布件（使用指南）。
//
// 断言：逐消费者 —— dir 基名匹配 `^v[0-9.]+$` ∧ 在盘 ∧ 含发布件；**跨消费者不一致 ⇒ 报**（✗ 不静默）。
//
// 退码：0 四消费者全符 · 1 有消费者不符（tri-report ⇒ 落 REPORT · 不锁提交链）· 2 取数面拿不到（不给结论）。
//
// 本件只读（不起子进程 · 不联网 · 不读仓外根除版本档案取源根外 · 默认不写盘）。判定口径与只读命令
// `zerg doc consumers` **共用** `dcReadAll()`（✗ 不两处各写一套）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	dcGateName = "check-doc-consumers"
	dcRCReport = 1 // 有消费者不符（tri-report ⇒ REPORT）
	dcRCBlock  = 2 // 取数面拿不到 ⇒ 不给结论
)

// dcDiverge —— 跨消费者是否分叉：返回（是否分叉, 逐目录计数 sorted 后的人面串）。
func dcDiverge(readings []dcReading) (bool, string) {
	cnt := map[string]int{}
	for _, r := range readings {
		key := r.Dir
		if key == "" {
			key = "（判不了）"
		}
		cnt[key]++
	}
	keys := make([]string, 0, len(cnt))
	for k := range cnt {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s×%d", filepath.Base(k), cnt[k]))
	}
	return len(cnt) > 1, strings.Join(parts, " ⟷ ")
}

// dcRender —— 人面渲染：`consumer⇥dir⇥is_v*⇥has_release_artifact` 逐行 + 分叉行 + 结论。
func dcRender(readings []dcReading, base string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "判据 %s（定格读数 · 只报告 · 账 GAP-20260927-129 ④）\n", dcGateName)
	fmt.Fprintf(&b, "版本档案取源根: %s\n", base)
	b.WriteString("consumer\tdir\tis_v*\thas_release_artifact\tverdict\n")
	nonConf := 0
	for _, r := range readings {
		dirTxt := r.Dir
		if dirTxt == "" {
			dirTxt = "（判不了：" + r.Why + "）"
		}
		verdict := "符"
		if !dcConforms(r) {
			verdict = "不符"
			nonConf++
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\n", r.Consumer, dirTxt, dcBool(r.IsVStar), dcBool(r.HasRelArt), verdict)
	}
	if div, seq := dcDiverge(readings); div {
		fmt.Fprintf(&b, "⚠ 跨消费者不一致（报 · 不静默）：%s\n", seq)
	}
	fmt.Fprintf(&b, "结论：不符 %d/%d（符 = 基名 v* ∧ 在盘 ∧ 含发布件）\n", nonConf, len(readings))
	return b.String()
}

// checkDocConsumersGateMain —— inproc 入口（argv 不含程序名 · 与 py `sys.argv[1:]` 同形）。
func checkDocConsumersGateMain(stdout, stderr io.Writer, argv []string) int {
	var (
		selfTest bool
		asJSON   bool
	)
	for _, a := range argv {
		switch a {
		case "--self-test":
			selfTest = true
		case "--json":
			asJSON = true
		case "-h", "--help":
			dcUsage(stdout)
			return exitOK
		case "":
		default:
			dcUsage(stderr)
			fmt.Fprintf(stderr, "%s: unknown arguments: %s\n", dcGateName, a)
			return dcRCBlock
		}
	}
	if selfTest {
		return dcSelfTest(stdout)
	}

	readings, why := dcReadAll()
	if readings == nil {
		fmt.Fprintf(stderr, "%s: %s\n", dcGateName, why)
		fmt.Fprintf(stderr, "档 : tri-report · ✗ 取数面拿不到 ⇒ 不给结论（rc=2）\n")
		return dcRCBlock
	}
	if asJSON {
		fmt.Fprintln(stdout, dcJSON(readings, devDocsBase()))
	} else {
		fmt.Fprintln(stdout, dcRender(readings, devDocsBase()))
	}
	for _, r := range readings {
		if !dcConforms(r) {
			return dcRCReport
		}
	}
	return exitOK
}

// dcUsage —— 用法（-h/--help 走它）。
func dcUsage(w io.Writer) {
	fmt.Fprintf(w, "usage: %s [-h] [--self-test] [--json]\n", dcGateName)
	fmt.Fprintf(w, "  逐消费者现读「定格」的版本目录：必须是盘上已存在的 v* 目录 ∧ 含发布件（使用指南）。\n")
	fmt.Fprintf(w, "  退码：0 全符 · 1 有消费者不符 · 2 取数面拿不到（不给结论）。\n")
}

// dcJSON —— `--json`（机器面：逐消费者一格）。
func dcJSON(readings []dcReading, base string) string {
	div, seq := dcDiverge(readings)
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(" \"schema\": \"zerg/doc-consumers/v1\",\n")
	fmt.Fprintf(&b, " \"base\": %s,\n", dcJSONStr(base))
	fmt.Fprintf(&b, " \"divergent\": %t,\n", div)
	fmt.Fprintf(&b, " \"divergence\": %s,\n", dcJSONStr(seq))
	b.WriteString(" \"consumers\": [\n")
	for i, r := range readings {
		comma := ","
		if i == len(readings)-1 {
			comma = ""
		}
		b.WriteString("  {\n")
		fmt.Fprintf(&b, "   \"consumer\": %s,\n", dcJSONStr(r.Consumer))
		fmt.Fprintf(&b, "   \"dir\": %s,\n", dcJSONStr(r.Dir))
		fmt.Fprintf(&b, "   \"is_vstar\": %t,\n", r.IsVStar)
		fmt.Fprintf(&b, "   \"on_disk\": %t,\n", r.OnDisk)
		fmt.Fprintf(&b, "   \"has_release_artifact\": %t,\n", r.HasRelArt)
		fmt.Fprintf(&b, "   \"conforms\": %t,\n", dcConforms(r))
		fmt.Fprintf(&b, "   \"note\": %s\n", dcJSONStr(r.Why))
		fmt.Fprintf(&b, "  }%s\n", comma)
	}
	b.WriteString(" ]\n")
	b.WriteString("}")
	return b.String()
}

// dcJSONStr —— JSON 字符串转义（取值含 ASCII/中文/路径 ⇒ 直引号转义足够）。
func dcJSONStr(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	return "\"" + r.Replace(s) + "\""
}

// ── 自检（`--self-test` · 仓外夹具 · 跑完自回收）────────────────────────────────

type dcCheck struct {
	name string
	ok   bool
	got  string
	want string
}

func dcChk(name string, got, want string) dcCheck {
	return dcCheck{name: name, ok: got == want, got: got, want: want}
}

func dcChkBool(name string, got, want bool) dcCheck {
	return dcChk(name, dcBool(got), dcBool(want))
}

// dcSelfTest —— 自检：造仓外夹具（v2.5.10 有发布件 · v2.5.13 无发布件 · v2.6 空壳 · 缺目录），
// 逐格对拍判定 + **成对负控**（抽掉发布件 ⇒ 该格由「符」翻「不符」）。
func dcSelfTest(stdout io.Writer) int {
	base, err := os.MkdirTemp("", "dc-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "✗ 自检夹具建不出：%v\n", err)
		return dcRCReport
	}
	defer os.RemoveAll(base) // 自回收（建点成功这一支）

	// 夹具：v2.5.10 含发布件；v2.5.13 无发布件；v2.6 空壳；v9.9.9 不在盘。
	dirOK := filepath.Join(base, "v2.5.10")
	dirNoArt := filepath.Join(base, "v2.5.13")
	dirEmpty := filepath.Join(base, "v2.6")
	for _, d := range []string{dirOK, dirNoArt, dirEmpty} {
		_ = os.MkdirAll(d, 0o755)
	}
	art := filepath.Join(dirOK, "使用-虫族指南-20260902.md")
	_ = os.WriteFile(art, []byte("guide\n"), 0o644)

	checks := []dcCheck{
		dcChkBool("v2.5.10（有发布件）· conforms", dcConforms(dcJudgeOne("a", dirOK, "")), true),
		dcChkBool("v2.5.10（有发布件）· is_v*", dcJudgeOne("a", dirOK, "").IsVStar, true),
		dcChkBool("v2.5.13（无发布件）· conforms", dcConforms(dcJudgeOne("a", dirNoArt, "")), false),
		dcChkBool("v2.5.13（无发布件）· has_release_artifact", dcJudgeOne("a", dirNoArt, "").HasRelArt, false),
		dcChkBool("v2.6（空壳无发布件）· conforms", dcConforms(dcJudgeOne("a", dirEmpty, "")), false),
		dcChkBool("v9.9.9（不在盘）· on_disk", dcJudgeOne("a", filepath.Join(base, "v9.9.9"), "").OnDisk, false),
		dcChkBool("NOTVER（基名非 v*）· is_v*", dcJudgeOne("a", filepath.Join(base, "NOTVER"), "").IsVStar, false),
		dcChkBool("dir 空（判不了）· conforms", dcConforms(dcJudgeOne("a", "", "取源根缺")), false),
	}
	// 分叉判定：两组同名目录 ⇒ 分叉；同目录 ⇒ 不分叉。
	divA, _ := dcDiverge([]dcReading{{Dir: dirOK}, {Dir: dirNoArt}})
	divB, _ := dcDiverge([]dcReading{{Dir: dirOK}, {Dir: dirOK}})
	checks = append(checks,
		dcChkBool("分叉判定：两不同目录 ⇒ 分叉", divA, true),
		dcChkBool("分叉判定：同目录 ⇒ 不分叉", divB, false),
	)

	// ★ GAP-20260927-121：消费者「钉版目录无发布件 ⇒ fail-closed 明报」—— 正控 + 成对负控
	//   （同一判定函数 devDocsConsumable：export / doc meta fill 两处消费者取源都走它；两态只差「目录里有没有发布件」）。
	okDir, okWhy := devDocsConsumable(dirOK, "")
	checks = append(checks,
		dcChkBool("正控·钉版目录含发布件 ⇒ 给结论（dir 原样 · why 空）", okDir == dirOK && okWhy == "", true),
	)
	_, noArtWhy := devDocsConsumable(dirNoArt, "")
	checks = append(checks,
		dcChkBool("负控·钉版目录无发布件 ⇒ 不给结论（why 非空）", noArtWhy != "", true),
		dcChkBool("负控·钉版目录无发布件 ⇒ 逐字明报「无发布件，故本消费者取不到」", strings.Contains(noArtWhy, "无发布件，故本消费者取不到"), true),
	)
	_, passWhy := devDocsConsumable(dirOK, "目录本身判不了（不在盘）")
	checks = append(checks,
		dcChkBool("判不了 ⇒ 原样透传既有 why（不覆盖）", passWhy == "目录本身判不了（不在盘）", true),
	)

	// 成对负控：抽掉 v2.5.10 的发布件 ⇒ 该格由「符」翻「不符」（同一判定函数 · 只差那一件）。
	before := dcConforms(dcJudgeOne("zerg_overview", dirOK, ""))
	_ = os.Remove(art)
	after := dcConforms(dcJudgeOne("zerg_overview", dirOK, ""))
	checks = append(checks,
		dcChkBool("负控·抽件前 zerg_overview 格 = 符", before, true),
		dcChkBool("负控·抽件后 zerg_overview 格 = 不符", after, false),
	)

	fails := 0
	for _, c := range checks {
		mark := "✓"
		if !c.ok {
			mark = "✗"
			fails++
		}
		fmt.Fprintf(stdout, "%s %s：got=%s want=%s\n", mark, c.name, c.got, c.want)
	}
	fmt.Fprintf(stdout, "self-test：%d 格 · %d 失败\n", len(checks), fails)
	if fails == 0 {
		return exitOK
	}
	return dcRCReport
}
