// Package selfupdate —— 源码式自更新（对齐 Hermes 的 `hermes update`；B3）。
//
// 为什么要"解析安装方式"：同一条 `zerg update` 命令在不同机器上语义不同——
//
//	· git    = 源码检出（本机可从公开仓 fetch 并自建）——可自更新
//	· fleet  = 机群下发（由主控编排，本机不该自己去 fetch）——拒绝
//	· unknown= 非 git 安装（发行版包/手工放置）——拒绝并给指引
//
// **印记放代码树旁而非状态目录**（照抄 Hermes hermes_cli/config.py:detect_install_method）：
// 状态目录（~/.zerg）是**共享数据**——同一台机器上的两个安装（如容器与宿主）可能绑同一份
// home；若印记按 home 存，一处写 `fleet` 会让另一处的 `zerg update` 误拒（Hermes #34397 的教训）。
// 代码树是"这份二进制从哪来"的天然坐标，故印记落 `<代码树>/.install_method`。
package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 安装方式取值（与 Hermes 的 _SUPPORTED_INSTALL_METHODS 同构，去掉容器/包管理器形态）。
const (
	MethodGit     = "git"     // 源码检出：可 fetch + 本机构建 + 自换装
	MethodFleet   = "fleet"   // 机群下发：由主控编排，本机自身不 fetch
	MethodUnknown = "unknown" // 未知/非 git 安装：拒绝（--force 可越）
)

var supportedMethods = map[string]bool{
	MethodGit:     true,
	MethodFleet:   true,
	MethodUnknown: true,
}

// readStamp 读印记文件；非受支持取值（或读不到）一律返回空串，不假装。
func readStamp(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	m := strings.ToLower(strings.TrimSpace(string(b)))
	if supportedMethods[m] {
		return m
	}
	return ""
}

// isGitCheckout 判定 root 是否为 git 检出（`.git` 目录，或 worktree 的 `gitdir:` 指针文件）。
func isGitCheckout(root string) bool {
	p := filepath.Join(root, ".git")
	st, err := os.Stat(p)
	if err != nil {
		return false
	}
	if st.IsDir() {
		return true
	}
	if b, err := os.ReadFile(p); err == nil {
		return strings.HasPrefix(strings.TrimSpace(string(b)), "gitdir:")
	}
	return false
}

// DetectInstallMethod 判定安装方式（权威顺序：代码树旁印记 → `.git` 检出 → unknown）。
func DetectInstallMethod(root string) string {
	if m := readStamp(filepath.Join(root, ".install_method")); m != "" {
		return m
	}
	if isGitCheckout(root) {
		return MethodGit
	}
	return MethodUnknown
}

// WriteStamp 写代码树旁的 `.install_method` 印记（安装器调用；`zerg update` 只读不写）。
func WriteStamp(root, method string) error {
	m := strings.ToLower(strings.TrimSpace(method))
	if !supportedMethods[m] {
		return fmt.Errorf("不支持的安装方式 %q（受支持：git/fleet/unknown）", method)
	}
	return os.WriteFile(filepath.Join(root, ".install_method"), []byte(m+"\n"), 0o644)
}
