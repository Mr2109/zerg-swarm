package monitor

// readSysfsUintContent 测试入口别名（与 linux 层 readSysfsUint 共用 parseSysfsUintLine）。
func readSysfsUintContent(content string) (uint64, bool) {
	return parseSysfsUintLine(content)
}
