package monitor

// cgroup_cpu.go —— 本单元 CPU 工时读数（活性看门狗的"有没有在花力气"证据）。
//
// 设计真源：docs/01-设计/设计-活性看门狗.md §2
//
// 口径：cgroup v2 `cpu.stat` 的 `usage_usec` = **该单元内所有进程**的 CPU 时间累计（微秒）。
//   · 为什么按单元取：整机 CPU% / GPU% 是全机共享的 —— 隔壁那枚卵、别的进程一忙，
//     会给"这枚卵还在干活"作伪证。按单元取才骗不了。
//   · 为什么是累计量：两次采样相减即可判"这段时间有没有花力气"，不受瞬时抖动影响。
//   · 读不到（非 Linux / 无 systemd / 权限不足 / 单元不在）⇒ 返回 error：
//     调用方据此**降级为固定超时**（fail-open），绝不因缺读数误杀在飞请求。

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// EggCPUUsecFromCgroup 读某 cgroup 目录的累计 CPU 工时（微秒）。
//
// dir 形如 /sys/fs/cgroup/user.slice/user-1000.slice/user@1000.service/llm.slice/<unit>.service
// （生产上由 systemctl show <unit> -p ControlGroup 解析出来，与封闭性核验同一口径）。
func EggCPUUsecFromCgroup(dir string) (uint64, error) {
	if strings.TrimSpace(dir) == "" {
		return 0, fmt.Errorf("cgroup 目录为空")
	}
	f, err := os.Open(filepath.Join(dir, "cpu.stat"))
	if err != nil {
		return 0, fmt.Errorf("读 cpu.stat 失败（%s）：%w", dir, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "usage_usec") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return 0, fmt.Errorf("usage_usec 行格式异常：%q", line)
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("usage_usec 解析失败（%q）：%w", fields[1], err)
		}
		return v, nil
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("读 cpu.stat 出错：%w", err)
	}
	return 0, fmt.Errorf("cpu.stat 里没有 usage_usec（%s）", dir)
}
