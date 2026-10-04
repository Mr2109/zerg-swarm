// render.go —— 三态渲染（人面 / 行式面 / 机器面）。
//
// 照 §九 M14：**人面是行式面的渲染** —— 两者字段集 / 字段序 / 值文本**逐字节可对应**，
// 差别只在「对齐与表头装饰」；机器面（`--json`）不因 TTY 改形状。
//
//	· TTY（默认）      → 表格式：同名字段表头 + 空格对齐
//	· 非 TTY / --plain → 行式：同名字段表头 + 制表符分隔（不给 TTY 加装饰、不加颜色、不分页）
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

// ttyOf 判 stdout 是不是终端（不引第三方库：字符设备即 TTY）。
// 只判 stdout —— 进度/日志走 stderr，两者分开判（§九 M14：分别判 stdout/stderr）。
func ttyOf(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// renderRows 出表。tty=false 时走行式（制表符分隔），字段名与字段序与 TTY 一模一样。
//
// ★ TSV 安全（缺口 GAP-20260929-36）：单元格里的换行 / 制表符 / 回车会把行式（TSV）记录**撕裂**成
// 多条物理行 —— task ls 的 description 内嵌换行时，126 条任务输出成 662 物理行，凡按行解析的读面
// （含机器面管道）都把一条拆成多行。照 TSV 规矩把这三类控制符写成转义序列（`\n` / `	` / `\r`）⇒
// 「物理行数 == 条目数」恒成立。转义**只发生在展示层**（人面 / 行式面）；`--json` 面走 selectJSONList、
// JSON 自带转义 ⇒ 不受影响。发生转义时在 stderr 明示（机器面自校：行数 == 条目数）。
func renderRows(stdout, stderr io.Writer, tty bool, cols []string, rows [][]string) {
	escaped := 0
	escCols := make([]string, len(cols))
	for i, c := range cols {
		escCols[i], _ = tsvEscape(c)
	}
	escRows := make([][]string, len(rows))
	for ri, r := range rows {
		er := make([]string, len(r))
		for ci, v := range r {
			e, n := tsvEscape(v)
			escaped += n
			er[ci] = e
		}
		escRows[ri] = er
	}
	if escaped > 0 {
		fmt.Fprintf(stderr, "共 %d 条（%d 处单元格含换行/制表符 ⇒ 已按 TSV 规矩转义为转义序列，物理行数 == 条目数）\n",
			len(escRows), escaped)
	}
	if !tty {
		fmt.Fprintln(stdout, strings.Join(escCols, "	"))
		for _, r := range escRows {
			fmt.Fprintln(stdout, strings.Join(r, "	"))
		}
		if len(escRows) == 0 {
			fmt.Fprintln(stderr, "共 0 条")
		}
		return
	}
	widths := make([]int, len(escCols))
	for i, c := range escCols {
		widths[i] = displayWidth(c)
	}
	for _, r := range escRows {
		for i, v := range r {
			if i < len(widths) && displayWidth(v) > widths[i] {
				widths[i] = displayWidth(v)
			}
		}
	}
	line := func(vals []string) string {
		var parts []string
		for i, v := range vals {
			if i == len(vals)-1 {
				parts = append(parts, v)
				break
			}
			pad := widths[i] - displayWidth(v)
			if pad < 0 {
				pad = 0
			}
			parts = append(parts, v+strings.Repeat(" ", pad))
		}
		return strings.TrimRight(strings.Join(parts, "  "), " ")
	}
	fmt.Fprintln(stdout, line(cols))
	for _, r := range rows {
		fmt.Fprintln(stdout, line(r))
	}
	fmt.Fprintf(stderr, "共 %d 条\n", len(rows))
}

// tsvEscape 把一个单元格收敛成**单物理行**（TSV 安全）：换行 / 制表符 / 回车写成转义序列。
// 返回（转义后文本, 被替换的控制符个数）—— 计数供「行数 == 条目数」机器面自校。
// 无控制符 ⇒ 原样返回、计数 0（绝大多数单元格走这条 ⇒ 零额外分配、零行为差）。
func tsvEscape(s string) (string, int) {
	n := strings.Count(s, "\t") + strings.Count(s, "\n") + strings.Count(s, "\r")
	if n == 0 {
		return s, 0
	}
	return strings.NewReplacer("\t", `\t`, "\n", `\n`, "\r", `\r`).Replace(s), n
}

// displayWidth 按显示宽度算长度（CJK 与全角符号算 2，其余算 1）—— 只为对齐，不改值。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r > unicode.MaxASCII:
			w += 2
		default:
			w++
		}
	}
	return w
}
