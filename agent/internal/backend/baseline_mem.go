// baseline_mem.go —— 推理进程占用的解析（**平台无关**，两个平台都编译、都能单测）。
//
// 为什么单独一个无平台标签的文件：这段解析直接决定准入扣减是否成立（M15），
// 必须能在开发机（darwin）上跑测试；放在 `_linux.go` 里会因隐式 GOOS 约束在 darwin 上消失。
package backend

import (
	"strconv"
	"strings"
)

// parseDrmMemoryGb 从一段 fdinfo 文本里解析 drm-memory-gtt/vram 合计（GB）。
//
// 形如（X3 实测原文，注意冒号后是 `空格+制表符` 混排）：
//
//	drm-memory-vram:	2208 KiB
//	drm-memory-gtt: 	34286572 KiB
//	drm-memory-cpu: 	0 KiB
//
// 只用 gtt + vram 两项：cpu 项是驱动页表/元数据，不是权重占用。
func parseDrmMemoryGb(text string) float64 {
	var kib float64
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "drm-memory-gtt:") && !strings.HasPrefix(line, "drm-memory-vram:") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(strings.TrimPrefix(line, "drm-memory-gtt:"), "drm-memory-vram:"))
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
	return kib / (1024 * 1024) // KiB → GB（GiB）
}
