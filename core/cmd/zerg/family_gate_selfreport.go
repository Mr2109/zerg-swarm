// family_gate_selfreport.go —— `zerg gate selfreport diff --source <件> --against <件>`（T09）。
//
// 病灶（逐字）：自报件与**独立对拍方**之间没有正门对拍口 —— 今天判「自报面是不是照着真源报的」
// 只能手搓 `diff`／肉眼比对，且**没有**「判不了 ⇒ 不给结论」这一档。
//
// 六格（判据面逐字，缺一格即**判不了**）：
//
//	query_ts / computed_at / source / digest / verifier / verdict
//
// ★ 退码（**判不了 ≠ 绿** —— 本命令的退码判的是「对拍结论」，不是「自报面好看」）：
//
//	0  六格**逐字相同**（对拍方与自报方对上了）
//	1  六格有差异格（对拍不一致 —— 差异**逐格点名**，不合并成一句「不一致」）
//	2  用法错（缺 `--source` / `--against` / 同给 / 多余位置参数 / `--json` 字段表外）
//	8  **判不了**（任一枚件读不到 / JSON 不可解析 / 六格缺字段 / 格值不是字符串）⇒ 不给结论
//
// ★ 只读：`os.ReadFile`／`os.Stat` 两件，**不写不删不改**任何件（跑完仓内件 sha 不变）。
// ★ 独立面是**输入的一部分**：本命令不产自报件、也不改写它 —— 「禁自报面自打分」靠**两枚件分别给**
// 这条形状钉住（两枚同名 ⇒ 用法错 2：自己跟自己比必然恒同，那是**假绿**）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// gateSelfreportCells —— 六格闭集（逐字；顺序即人面顺序）。
var gateSelfreportCells = []string{"query_ts", "computed_at", "source", "digest", "verifier", "verdict"}

// gateSelfreportFields —— `--json` 的字段表（每格一行四列：格名 / 自报值 / 对拍值 / 是否相同）。
var gateSelfreportFields = []string{"field", "source", "against", "same"}

func gateSelfreportFieldLegal(f string) bool {
	for _, ok := range gateSelfreportFields {
		if f == ok {
			return true
		}
	}
	return false
}

// gateSelfreportRead —— 读一枚自报/对拍件：返回（六格值表 · 读或解析的错）。
// 错分成因逐字带上去（读不到 / 不可解析 / 不是对象 / 缺格 / 格值不是字符串）—— 不许吞成「不一致」。
func gateSelfreportRead(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("件读不到：%w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("JSON 不可解析：%w", err)
	}
	out := map[string]string{}
	for _, c := range gateSelfreportCells {
		v, ok := raw[c]
		if !ok {
			return nil, fmt.Errorf("六格缺字段 %q", c)
		}
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("格 %q 的值不是字符串（%T）—— 六格面是**逐字**字符串面", c, v)
		}
		out[c] = s
	}
	return out, nil
}

// cmdGateSelfreportDiff —— `gate selfreport diff`（只读 · 不走脚本 · 不改任何脚本退码）。
func cmdGateSelfreportDiff(inv *invocation, stdout, stderr io.Writer) int {
	src := strings.TrimSpace(inv.flagVal("--source"))
	against := strings.TrimSpace(inv.flagVal("--against"))
	if src == "" {
		inv.setErr("usage", "missing_source", "缺 --source（自报方）")
		fmt.Fprintf(stderr, "%s: `gate selfreport diff` 要给两枚件：`--source <自报件>` 与 `--against <对拍件>`\n", progName)
		return exitUsage
	}
	if against == "" {
		inv.setErr("usage", "missing_against", "缺 --against（对拍方）")
		fmt.Fprintf(stderr, "%s: 缺 `--against <对拍件>` —— 「对拍方必须独立、禁自报面自打分」是这条命令的形状前提\n", progName)
		return exitUsage
	}
	if src == against {
		inv.setErr("usage", "self_against_self", "两枚件同名")
		fmt.Fprintf(stderr, "%s: `--source` 与 `--against` **指同一枚件** %q ⇒ 自己跟自己比必然恒同（假绿）—— 用法错 2\n", progName, src)
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `gate selfreport diff` 不收位置参数（两枚件由 `--source` / `--against` 指）—— 多给了 %q\n", progName, inv.args[0])
		return exitUsage
	}
	fields := inv.fields
	if len(fields) == 0 {
		fields = gateSelfreportFields
	}
	for _, f := range fields {
		if !gateSelfreportFieldLegal(f) {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, inv.path, f)
		}
	}

	sCells, sErr := gateSelfreportRead(src)
	aCells, aErr := gateSelfreportRead(against)
	if sErr != nil || aErr != nil {
		// 判不了 ⇒ 不给结论（8）。两枚件的成因逐条出，**不合并**（哪一枚坏了要能一眼看出）。
		inv.setErr("blocked", "selfreport_unreadable", "一枚或两枚件读不到/不可解析")
		fmt.Fprintf(stderr, "%s: **判不了** ⇒ 不给结论（退码 8）\n", progName)
		if sErr != nil {
			fmt.Fprintf(stderr, "  --source  %s：%v\n", src, sErr)
		}
		if aErr != nil {
			fmt.Fprintf(stderr, "  --against %s：%v\n", against, aErr)
		}
		fmt.Fprintf(stderr, "口径：读不到/缺格既不是「相同」也不是「不同」—— 不许当绿\n")
		return exitBlocked
	}

	diff := []string{}
	for _, c := range gateSelfreportCells {
		if sCells[c] != aCells[c] {
			diff = append(diff, c)
		}
	}
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			inv.fields = fields
		}
		inv.metaAddStr("source_file", src)
		inv.metaAddStr("against_file", against)
		inv.metaAddJSON("cells", fmt.Sprintf("%d", len(gateSelfreportCells)))
		inv.metaAddJSON("differ", fmt.Sprintf("%d", len(diff)))
		items := make([]map[string]string, 0, len(gateSelfreportCells))
		for _, c := range gateSelfreportCells {
			same := "yes"
			if sCells[c] != aCells[c] {
				same = "no"
			}
			items = append(items, map[string]string{
				"field": c, "source": sCells[c], "against": aCells[c], "same": same,
			})
		}
		rc := selectJSONList(stdout, stderr, inv, inv.path, fields, items)
		if rc != exitOK {
			return rc
		}
		if len(diff) > 0 {
			return exitFail
		}
		return exitOK
	}

	fmt.Fprintf(stdout, "自报件：%s\n对拍件：%s\n", src, against)
	fmt.Fprintf(stdout, "── 六格逐格对拍 ──\n")
	for _, c := range gateSelfreportCells {
		mark := "同"
		if sCells[c] != aCells[c] {
			mark = "**异**"
		}
		fmt.Fprintf(stdout, "  %-12s %s\n    自报：%s\n    对拍：%s\n", c, mark, sCells[c], aCells[c])
	}
	if len(diff) > 0 {
		fmt.Fprintf(stdout, "结论：%d 格不一致（%s）⇒ 退 1\n", len(diff), strings.Join(diff, "、"))
		fmt.Fprintf(stdout, "（本命令只读 · 不改两枚件 · 也不产自报件）\n")
		return exitFail
	}
	fmt.Fprintf(stdout, "结论：六格逐字相同（退 0）—— 对拍方与自报方对上了\n")
	fmt.Fprintf(stdout, "（本命令只读 · 不改两枚件 · 也不产自报件）\n")
	return exitOK
}
