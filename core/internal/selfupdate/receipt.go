package selfupdate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LaunchReceipt —— `zerg update` 的**交接回执**：记录"它把活交给了哪个独立进程"。
// 权威回执（成功/失败/回滚）由内核 zerg-upgrade.sh 写同目录，两者 kind 不同、互不覆盖。
type LaunchReceipt struct {
	Schema     int           `json:"schema"`
	Kind       string        `json:"kind"`
	At         string        `json:"at"`
	Method     string        `json:"install_method"`
	Prefix     string        `json:"prefix"`
	Repo       string        `json:"repo"`
	From       string        `json:"from_sha"`
	Role       string        `json:"role,omitempty"`       // controller | node（B5）
	Components []string      `json:"components,omitempty"` // 本次构建/换装的组件集
	Toolchain  ToolchainInfo `json:"toolchain,omitempty"`  // G6：pin/actual/policy
	BuildTime  string        `json:"build_time,omitempty"` // 构建时间戳（机群共享同一戳 ⇒ 可逐字节比对）
	Target     TargetInfo    `json:"target"`
	Staging    string        `json:"staging"`
	Artifacts  []Artifact    `json:"artifacts"`
	Kernel     string        `json:"kernel"`
	KernelPID  int           `json:"kernel_pid"`
	KernelLog  string        `json:"kernel_log"`
	ReceiptDir string        `json:"receipt_dir"`
	Unmanaged  []string      `json:"unmanaged_components,omitempty"`
	Notes      []string      `json:"notes,omitempty"`
}

// TargetInfo —— 目标版本身份。
type TargetInfo struct {
	Tag    string `json:"tag"`
	Commit string `json:"commit"`
}

// WriteLaunchReceipt —— 写交接回执；失败只告警不阻断（回执不该成为升级的失败点）。
func WriteLaunchReceipt(receiptsDir string, r LaunchReceipt) (string, error) {
	if err := os.MkdirAll(receiptsDir, 0o755); err != nil {
		return "", err
	}
	r.Schema = 1
	r.Kind = "update-launch"
	if r.At == "" {
		r.At = time.Now().UTC().Format("20060102T150405Z")
	}
	name := fmt.Sprintf("update-launch-%s.json", r.At)
	path := filepath.Join(receiptsDir, name)
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	_ = os.WriteFile(filepath.Join(receiptsDir, "latest-update-launch.json"), append(b, '\n'), 0o644)
	return path, nil
}
