// eggprofile.go — 读卵的**实测档案**里的"实际上下文"（事实），供动态输出预算取值。
//
// 原则（与"闸门只读实测档案"同一条理）：
//
//	配置里的 ctx_window 是**意图**（可能不准，实测 X3 的 Qwen 声明 1M 而实跑 -c 262144）；
//	档案里的 ctx_window 是**事实**（孵化/标定时记录的实际启动值）。
//	取值顺序：档案（事实）⇒ 配置声明（意图）⇒ 保守默认。三态必须在日志里可分。
//
// 档案路径：<state_dir>/egg-profiles/<模型名>.yaml（与 statepath 同根）。
// 解析不引第三方 yaml：只扫一行 `ctx_window:`（档案是受控格式，字段名固定）。
package gateway

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// EggProfileDir — 档案目录（可用 ZERG_EGG_PROFILE_DIR 覆盖，便于测试与部署）
func EggProfileDir() string {
	if d := os.Getenv("ZERG_EGG_PROFILE_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "egg-profiles")
}

// ProfileCtxWindow — 读档案里的实际上下文；返回 (值, 是否有档案)。
// 无档案 / 无该字段 / 解析失败 一律返回 (0,false)，由调用方退回"意图"或"默认"。
func ProfileCtxWindow(model string) (int, bool) {
	dir := EggProfileDir()
	if dir == "" || model == "" {
		return 0, false
	}
	// 档案名 = 模型名 + .yaml（允许大小写不敏感匹配，避免文件名大小写坑）
	path := filepath.Join(dir, model+".yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		// 大小写不敏感兜底：目录内找一个同名（忽略大小写）的文件
		ents, derr := os.ReadDir(dir)
		if derr != nil {
			return 0, false
		}
		for _, e := range ents {
			if strings.EqualFold(strings.TrimSuffix(e.Name(), ".yaml"), model) {
				b, err = os.ReadFile(filepath.Join(dir, e.Name()))
				break
			}
		}
		if err != nil {
			return 0, false
		}
	}
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			continue
		}
		if !strings.HasPrefix(t, "ctx_window:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(t, "ctx_window:"))
		if v == "" {
			continue
		}
		n, perr := strconv.Atoi(v)
		if perr != nil || n <= 0 {
			continue
		}
		return n, true
	}
	return 0, false
}
