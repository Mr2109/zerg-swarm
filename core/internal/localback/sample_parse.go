package localback

// sample_parse.go —— 本机采样的**纯解析**函数：无 IO、无平台依赖、可在任意平台表驱动测试。
//
// 为什么抽出来（待修补 #33）：Linux 分支要读 /proc/meminfo、/proc/loadavg，而开发机是 macOS
// （本机没有 Linux 环境，装不了跑不了那份代码）。把"文本 → 数值"这一步做成纯函数之后，
// 拿真实格式的样本字符串就能在 macOS 上验证（含缺字段 / 数值异常样本），
// 平台差异只留在薄薄一层 IO 里（sample_platform.go）。
//
// 【单位口径——与既有 macOS 分支完全一致】内存/显存一律 GiB（1024^3 字节），不是十进制 GB：
//
//	macOS : hw.memsize（字节）÷ GiB；vm_stat 页数 × 16384 ÷ GiB；负载取 vm.loadavg 第 2 字段。
//	Linux : /proc/meminfo（kB）÷ 1024 ÷ 1024；/proc/loadavg 第 1 字段。
//	GPU   : nvidia-smi（MiB）÷ 1024；rocm-smi（字节）÷ GiB。
//
// 【取值纪律——两平台同一套不变量】拿到 → 带值 + ok=true；拿不到 → ok=false（调用方留 0 = 缺席）。
// 绝不用 0 / 估算值冒充实测值：0 只在"已知缺席"时出现，且此时调用方必须同时给 known=false。

import (
	"math"
	"strconv"
	"strings"
)

// gib 是 1 GiB 的字节数——本包内存/显存换算的唯一基准。
const gib = 1024 * 1024 * 1024

// darwinPageSize 是 Apple Silicon 的页大小（既有 macOS 分支硬编码 16384，保持不变）。
const darwinPageSize = 16384.0

// kibToGiB / mibToGiB / bytesToGiB 单位换算（kB / MiB / B 是内核与工具的导出单位，GiB 是本包口径）。
func kibToGiB(kib float64) float64 { return kib * 1024 / float64(gib) }
func mibToGiB(mib float64) float64 { return mib * 1024 * 1024 / float64(gib) }
func bytesToGiB(b float64) float64 { return b / float64(gib) }

// saneNum 采样值是否可用于记账：非 NaN/Inf 且非负。
// 异常样本一律按"没采到"处理（不猜、不清洗、不当 0 用）。
func saneNum(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0
}

// parseMeminfo 解析 /proc/meminfo 文本，返回内存总量与可用量（GiB）及各自"采到了没有"。
//
// 规则：
//   - MemTotal 缺失 / 非数 / 单位不认识 / ≤0 → totalOK=false（缺席）；
//   - MemAvailable 存在且 >0 → 直接用它（内核自己的可用量估计）；
//   - MemAvailable 缺失（内核 < 3.14）→ 回退 MemFree+Buffers+Cached 求和
//     （proc(5) 记载的旧口径近似，这三项也都是内核导出的真实字段，不是我们估出来的）；
//   - MemFree 也缺 → availOK=false（缺席）。
func parseMeminfo(content string) (totalGB, availGB float64, totalOK, availOK bool) {
	var memTotalKB, memAvailKB, memFreeKB, buffersKB, cachedKB float64
	var hasTotal, hasAvail, hasFree, hasBuffers, hasCached bool
	for _, line := range strings.Split(content, "\n") {
		key, valKB, ok := parseMeminfoLine(line)
		if !ok {
			continue
		}
		switch key {
		case "MemTotal":
			memTotalKB, hasTotal = valKB, true
		case "MemAvailable":
			memAvailKB, hasAvail = valKB, true
		case "MemFree":
			memFreeKB, hasFree = valKB, true
		case "Buffers":
			buffersKB, hasBuffers = valKB, true
		case "Cached": // 精确匹配，别把 SwapCached 算进来
			cachedKB, hasCached = valKB, true
		}
	}
	if hasTotal && memTotalKB > 0 {
		totalGB, totalOK = kibToGiB(memTotalKB), true
	}
	switch {
	case hasAvail && memAvailKB > 0:
		availGB, availOK = kibToGiB(memAvailKB), true
	case hasFree:
		sum := memFreeKB
		if hasBuffers {
			sum += buffersKB
		}
		if hasCached {
			sum += cachedKB
		}
		if sum > 0 {
			availGB, availOK = kibToGiB(sum), true
		}
	}
	return totalGB, availGB, totalOK, availOK
}

// parseMeminfoLine 解析 /proc/meminfo 的一行（"MemTotal:       16267648 kB"），返回字段名与数值（kB）。
// 只接受"数值 + 可选的 kB 单位"：非数 / 负数 / 单位不是 kB / 垃圾行一律 ok=false，
// 让调用方按"该字段缺席"处理（不替内核猜它想说什么）。
func parseMeminfoLine(line string) (key string, valKB float64, ok bool) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", 0, false
	}
	key = strings.TrimSpace(line[:idx])
	rest := strings.TrimSpace(line[idx+1:])
	if rest == "" {
		return "", 0, false
	}
	fields := strings.Fields(rest)
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || !saneNum(v) {
		return "", 0, false
	}
	if len(fields) > 1 && fields[1] != "kB" {
		return "", 0, false // 单位不认识 → 不按 kB 硬套
	}
	return key, v, true
}

// parseProcLoadavg 解析 /proc/loadavg（"0.52 0.58 0.59 1/1234 5678"）的第 1 字段 = 1 分钟负载。
// 口径与 macOS 的 vm.loadavg 一致（同是 1 分钟平均运行队列长度）。
func parseProcLoadavg(content string) (float64, bool) {
	return loadField(strings.Fields(content), 0)
}

// parseDarwinLoadavg 解析 macOS "sysctl -n vm.loadavg" 的输出（"{ 1.79 2.03 2.14 }"）：
// 第 1 个字段是左花括号，故取第 2 个字段 = 1 分钟负载（与既有实现同一口径）。
func parseDarwinLoadavg(out string) (float64, bool) {
	return loadField(strings.Fields(out), 1)
}

// loadField 取 fields[idx] 作为负载值：越界 / 非数 / 负数 / NaN / Inf 一律 ok=false。
func loadField(fields []string, idx int) (float64, bool) {
	if idx < 0 || idx >= len(fields) {
		return 0, false
	}
	v, err := strconv.ParseFloat(fields[idx], 64)
	if err != nil || !saneNum(v) {
		return 0, false
	}
	return v, true
}

// parseVMPages 从 "Pages free: 12345." 提取页数（既有 macOS 实现原样保留：解析不出 → 0）。
func parseVMPages(line string) uint64 {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return 0
	}
	val := strings.Trim(strings.TrimSpace(parts[1]), ".")
	n, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parseVMStatAvailableGiB 从 vm_stat 输出求和 free + inactive + speculative（页 × 16384 → GiB）。
// 三项一个都没解析出来 → ok=false（不把"读不懂 vm_stat"当成"可用内存 0"）。
func parseVMStatAvailableGiB(vout string) (float64, bool) {
	free, inactive, speculative := uint64(0), uint64(0), uint64(0)
	for _, line := range strings.Split(vout, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Pages free:"):
			free = parseVMPages(line)
		case strings.HasPrefix(line, "Pages inactive:"):
			inactive = parseVMPages(line)
		case strings.HasPrefix(line, "Pages speculative:"):
			speculative = parseVMPages(line)
		}
	}
	sum := free + inactive + speculative
	if sum == 0 {
		return 0, false
	}
	return float64(sum) * darwinPageSize / float64(gib), true
}

// parseHwMemsizeGiB 解析 "sysctl -n hw.memsize" 的输出（字节数，如 "68719476736"）→ GiB。
func parseHwMemsizeGiB(out string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(out), 64)
	if err != nil || !saneNum(v) || v <= 0 {
		return 0, false
	}
	return bytesToGiB(v), true
}

// parseNvidiaSmiMem 解析 nvidia-smi 的查询输出：
//
//	nvidia-smi --query-gpu=memory.total,memory.used,memory.free --format=csv,noheader,nounits
//
// 单位 MiB，每块卡一行（"24564, 1234, 23330"）。多卡取**第一块**：本机账面要回答"这张卡装得下吗"，
// 跨卡求和会把两块不相邻的卡算成一块大显存 = 假能力。任一列读不出 / 总量 ≤0 → ok=false。
func parseNvidiaSmiMem(out string) (totalGB, usedGB, freeGB float64, ok bool) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return 0, 0, 0, false
		}
		vals := make([]float64, 3)
		for i, p := range parts {
			v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil || !saneNum(v) {
				return 0, 0, 0, false // "[N/A]" / 空列 → 采不到，不猜
			}
			vals[i] = v
		}
		if vals[0] <= 0 {
			return 0, 0, 0, false
		}
		return mibToGiB(vals[0]), mibToGiB(vals[1]), mibToGiB(vals[2]), true
	}
	return 0, 0, 0, false
}

// parseRocmSmiMem 解析 `rocm-smi --showmeminfo vram --csv` 的输出，单位字节。
// 支持两种真实形态（版本差异），表头按列名定位（列顺序/列数各版本不同，不硬编码列号）：
//
//	形态 A（CSV）：
//	  device,VRAM Total Memory (B),VRAM Total Used Memory (B)
//	  card0,17163091968,742391808
//	形态 B（人类可读）：
//	  GPU[0] : VRAM Total Memory (B): 17163091968
//	  GPU[0] : VRAM Total Used Memory (B): 742391808
//
// 认不出列名 / 数值非正 → ok=false（不猜列序）。rocm-smi 不给 free → free = total - used
// （拿真实值做减法，不是估算）。多卡同 parseNvidiaSmiMem：取第一块。
func parseRocmSmiMem(out string) (totalGB, usedGB, freeGB float64, ok bool) {
	if t, u, f, ok := parseRocmSmiCSV(out); ok {
		return t, u, f, true
	}
	return parseRocmSmiLabelled(out)
}

// parseRocmSmiCSV 解析 CSV 形态（表头定位列号，取第一条数据行）。
func parseRocmSmiCSV(out string) (totalGB, usedGB, freeGB float64, ok bool) {
	lines := strings.Split(out, "\n")
	totalIdx, usedIdx := -1, -1
	start := 0
	for i, line := range lines {
		cells := strings.Split(line, ",")
		if len(cells) < 2 {
			continue
		}
		ti, ui := -1, -1
		for j, c := range cells {
			cl := strings.ToLower(strings.TrimSpace(c))
			if strings.Contains(cl, "total memory") {
				ti = j
			}
			if strings.Contains(cl, "used memory") {
				ui = j
			}
		}
		if ti >= 0 && ui >= 0 {
			totalIdx, usedIdx, start = ti, ui, i+1
			break
		}
	}
	if totalIdx < 0 || usedIdx < 0 {
		return 0, 0, 0, false
	}
	for _, line := range lines[start:] {
		cells := strings.Split(line, ",")
		if len(cells) <= totalIdx || len(cells) <= usedIdx {
			continue
		}
		total, err1 := strconv.ParseFloat(strings.TrimSpace(cells[totalIdx]), 64)
		used, err2 := strconv.ParseFloat(strings.TrimSpace(cells[usedIdx]), 64)
		if err1 != nil || err2 != nil || !saneNum(total) || !saneNum(used) || total <= 0 {
			continue
		}
		return bytesToGiB(total), bytesToGiB(used), bytesToGiB(total - used), true
	}
	return 0, 0, 0, false
}

// parseRocmSmiLabelled 解析 "标签: 数值" 形态（无表头可定位列号）。
func parseRocmSmiLabelled(out string) (totalGB, usedGB, freeGB float64, ok bool) {
	total, used := -1.0, -1.0
	for _, line := range strings.Split(out, "\n") {
		cl := strings.ToLower(line)
		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(line[idx+1:]), 64)
		if err != nil || !saneNum(v) {
			continue
		}
		switch {
		case total < 0 && strings.Contains(cl, "total memory"):
			total = v
		case used < 0 && strings.Contains(cl, "used memory"):
			used = v
		}
	}
	if total <= 0 || used < 0 {
		return 0, 0, 0, false
	}
	return bytesToGiB(total), bytesToGiB(used), bytesToGiB(total - used), true
}
