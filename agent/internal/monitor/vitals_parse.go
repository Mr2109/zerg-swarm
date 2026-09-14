// vitals_parse.go —— 解析纯函式（**平台无关**，两个平台都编译、都能单测）。
//
// 与 baseline_mem.go 的教训一致：解析直接决定交叉校验口径的成立，
// 放 _linux.go 里会因隐式 GOOS 约束在 darwin 上消失、测试编译不过。
package monitor

import (
	"strconv"
	"strings"
)

// parseSysfsUintLine sysfs 单数字文件内容 → uint64（容忍首尾空白；拒绝空/非数字）。
func parseSysfsUintLine(content string) (uint64, bool) {
	v, err := strconv.ParseUint(strings.TrimSpace(content), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseDrmGttKibCommon drm-memory-gtt 的解析本体（KiB 合计）。
// Linux 版与桩版都转调这里 ⇒ X3 实测样例在 darwin 上即可验证真读层用的同一段逻辑。
//
// X3 实测格式（冒号后空格/制表符混排；vram/cpu 行不认）：
//
//	drm-memory-vram:	2208 KiB
//	drm-memory-gtt: 	34286572 KiB
//
// 单位支持 KiB/MiB/GiB；形状不符/数值不合法 ⇒ 该行不计（宁缺勿错）。
func parseDrmGttKibCommon(text string) float64 {
	var kib float64
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "drm-memory-gtt:") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "drm-memory-gtt:"))
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(fields[1]) {
		case "kib":
			kib += v
		case "mib":
			kib += v * 1024
		case "gib":
			kib += v * 1024 * 1024
		}
	}
	return kib
}
