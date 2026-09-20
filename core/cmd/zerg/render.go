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
func renderRows(stdout, stderr io.Writer, tty bool, cols []string, rows [][]string) {
	if !tty {
		fmt.Fprintln(stdout, strings.Join(cols, "\t"))
		for _, r := range rows {
			fmt.Fprintln(stdout, strings.Join(r, "\t"))
		}
		if len(rows) == 0 {
			fmt.Fprintln(stderr, "共 0 条")
		}
		return
	}
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = displayWidth(c)
	}
	for _, r := range rows {
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
