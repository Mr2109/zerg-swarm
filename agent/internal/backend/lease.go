// lease.go —— P3：借用租约的存档内核（存储 + 脱敏 + 到期判定）。
//
// 设计依据：docs/01-设计/设计-子端服务切换与基线服务声明-20260914.md §9.3（借还序列）、
// §10 R5（TTL）/R7（字段与回执）/R1（PID 复用守卫）、§11 M1（崩溃兜底）/M2（进程组）/M5（max_hold）。
//
// 三条铁律：
//  1. **停任何东西之前必先存档**（write-then-act）：没有租约就没有停止动作。
//  2. **凭证不落明文**：env 只收白名单键，值命中密钥形态一律写 [REDACTED]。
//  3. **租约是崩溃安全的**：到期用 wall-clock（存在文件里），任何进程读到过期租约都能执行归还——
//     而非"进程内计时"（子端崩溃后无人归还，正是 §11 M1 要堵的洞）。
package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 租约状态。
const (
	LeaseBorrowed      = "borrowed"       // 已借：服务已被停，等待归还
	LeaseRestored      = "restored"       // 已归还（正常终态，随后删除）
	LeaseRestoreFailed = "restore_failed" // 归还失败：必须保留并告警（绝不静默丢）
)

// LeaseDirName 租约目录（与 update_receipts 同级，便于统一审计）。
const LeaseDirName = "service_leases"

// LeaseDirEnv 可覆盖租约目录（测试与沙箱用）。
const LeaseDirEnv = "ZERG_LEASE_DIR"

// 默认时长（设计 §10 R2/R5/§11 M5）。
const (
	DefaultLeaseTTLS    = 300 // 空闲 TTL：对齐 Ollama 的 keep_alive（5m）
	DefaultGraceStopS   = 15  // TERM→KILL 宽限：systemd 默认 90s 太长，借用场景要短
	DefaultMaxHoldS     = 1800
	DefaultRestoreWaitS = 60
)

// ServiceLease 一次借用的完整存档（§10 R7）。
type ServiceLease struct {
	LeaseID    string `json:"lease_id"`
	Port       int    `json:"port"`
	Kind       string `json:"kind,omitempty"`  // llama | ds4
	Class      string `json:"class,omitempty"` // bare | screen | systemd
	ScreenName string `json:"screen_name,omitempty"`
	Unit       string `json:"unit,omitempty"`
	Identity   string `json:"identity,omitempty"` // /v1/models 报出的标识（归还后核身份用）
	PID        int    `json:"pid,omitempty"`
	// Pipeline 停止时涉及的 pid 集合（M2：screen 链是 3 个进程；只杀一个会留孤儿占显存）。
	Pipeline    []int             `json:"pipeline,omitempty"`
	Argv        []string          `json:"argv"`
	Cwd         string            `json:"cwd"`
	EnvFiltered map[string]string `json:"env_filtered,omitempty"`
	StartTime   uint64            `json:"start_time,omitempty"` // /proc/<pid>/stat 第 22 字段（PID 复用守卫）
	ApproxGB    float64           `json:"approx_gb,omitempty"`  // 借出时该服务实测占用（审计与归还评估用）
	State       string            `json:"state"`
	AcquiredAt  time.Time         `json:"acquired_at"`
	LastActive  time.Time         `json:"last_activity"`
	TTLS        int               `json:"ttl_s"`
	MaxHoldS    int               `json:"max_hold_s"`
	Note        string            `json:"note,omitempty"`
}

// LeaseDir 返回租约目录（可用 ZERG_LEASE_DIR 覆盖；测试/沙箱用）。
func LeaseDir() (string, error) {
	if d := strings.TrimSpace(os.Getenv(LeaseDirEnv)); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".zerg", LeaseDirName), nil
}

func leaseFile(port int) (string, error) {
	dir, err := LeaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%d.json", port)), nil
}

// SaveLease 原子写租约（临时文件 + rename；权限 0600——存档含命令行与 cwd，属敏感）。
func SaveLease(l ServiceLease) error {
	if l.Port <= 0 {
		return errors.New("lease: port 必须 > 0")
	}
	if l.State == "" {
		l.State = LeaseBorrowed
	}
	if l.AcquiredAt.IsZero() {
		l.AcquiredAt = time.Now()
	}
	if l.LastActive.IsZero() {
		l.LastActive = l.AcquiredAt
	}
	if l.TTLS <= 0 {
		l.TTLS = DefaultLeaseTTLS
	}
	if l.MaxHoldS <= 0 {
		l.MaxHoldS = DefaultMaxHoldS
	}
	if l.LeaseID == "" {
		l.LeaseID = fmt.Sprintf("%d-%d", l.Port, l.AcquiredAt.Unix())
	}
	dir, err := LeaseDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := leaseFile(l.Port)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadLease 读单个端口的租约；不存在返回 (nil,false)。
func LoadLease(port int) (*ServiceLease, bool) {
	path, err := leaseFile(port)
	if err != nil {
		return nil, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var l ServiceLease
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, false
	}
	return &l, true
}

// ListLeases 列出全部租约（按端口排序，可复现）。
func ListLeases() []ServiceLease {
	dir, err := LeaseDir()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []ServiceLease
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSuffix(name, ".json"))
		if err != nil {
			continue
		}
		if l, ok := LoadLease(port); ok {
			out = append(out, *l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

// DeleteLease 删除租约（归还成功后调用）。
func DeleteLease(port int) error {
	path, err := leaseFile(port)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LeaseActionable 判定一条租约此刻该不该动作（纯函数，便于测试）。
//
// 返回 (是否应归还, 原因)。**到期用 wall-clock**：max_hold 是绝对上限（先到先算），
// 空闲 TTL 从 last_activity 起算（有在飞请求时由调用方刷新 last_activity）。
func LeaseActionable(l ServiceLease, now time.Time) (bool, string) {
	if l.State == LeaseRestoreFailed {
		return true, "上次归还失败：立即重试归还"
	}
	if l.State != LeaseBorrowed {
		return false, "非借用中状态（" + l.State + "）"
	}
	maxHold := time.Duration(l.MaxHoldS) * time.Second
	if maxHold > 0 && !l.AcquiredAt.IsZero() && now.Sub(l.AcquiredAt) >= maxHold {
		return true, fmt.Sprintf("已达 max_hold(%ds)：无条件归还并告警", l.MaxHoldS)
	}
	ttl := time.Duration(l.TTLS) * time.Second
	base := l.LastActive
	if base.IsZero() {
		base = l.AcquiredAt
	}
	if ttl > 0 && !base.IsZero() && now.Sub(base) >= ttl {
		return true, fmt.Sprintf("空闲超过 ttl(%ds)：自动归还", l.TTLS)
	}
	return false, ""
}

// envWhitelist 允许落进租约的环境变量键（P3 归还时按此重放）。
// 只收"决定进程能否找到库/设备"的那些；其余一律不存——凭证绝不落明文。
var envWhitelist = map[string]bool{
	"PATH":            true,
	"LD_LIBRARY_PATH": true,
	"LD_PRELOAD":      true,
	"HOME":            true,
	"USER":            true,
}

// envWhitelistPrefix 前缀白名单（ROCm/HSA 等运行时变量，如 HSA_OVERRIDE_GFX_VERSION）。
var envWhitelistPrefix = []string{"HSA_", "ROCM_", "HIP_", "MIOPEN_", "GGML_", "LLAMA_"}

// leaseEnvAllowed 判定某个环境变量键是否允许存档。
func leaseEnvAllowed(key string) bool {
	if envWhitelist[key] {
		return true
	}
	for _, p := range envWhitelistPrefix {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// secretLooking 判定"值看起来像密钥"（脱敏用；宁可多脱也不漏）。
// 两条判据：① 键名含敏感词；② 值是长随机串（≥32 位且高熵形态）。
func secretLooking(key, value string) bool {
	k := strings.ToLower(key)
	for _, w := range []string{"token", "secret", "password", "passwd", "apikey", "api_key", "credential", "auth"} {
		if strings.Contains(k, w) {
			return true
		}
	}
	if len(value) < 32 {
		return false
	}
	// 高熵形态：只由 base64/hex 类字符组成且无空格
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '+' || r == '/' || r == '=' || r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// FilterEnvForLease 从 os.Environ 形态的 []string 里挑出白名单键并脱敏（纯函数，便于测试）。
func FilterEnvForLease(env []string) map[string]string {
	out := map[string]string{}
	for _, kv := range env {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		k, v := kv[:i], kv[i+1:]
		if !leaseEnvAllowed(k) {
			continue
		}
		if secretLooking(k, v) {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = v
	}
	return out
}
