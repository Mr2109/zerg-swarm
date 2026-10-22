// origin.go —— 构建期「源提交」的唯一取法（缺口 `GAP-20261021-08`）。
//
// 为什么不能各写一份：`origin_sha` 是主控判「同源 / 混版 / 判不了」的**比对键**，比的是
// **逐字相同**。同一棵树若两处构建器取值形状不同（一处 40 位全 sha、一处 7 位短 sha），
// 三台机器必然被判成 `mixed`（假红）—— 那等于把「同源」这件事又变回判不了。
// 因此形状在这里**一处归一**，与 `scripts/build/cmd/zerg-build-all` 的 `gitOrigin` 同口径：
//
//	① 目标树的 HEAD 带 `GitOrigin-RevId` trailer（镜像/中继树）⇒ trailer 值 = 私有图源提交；
//	② 不带（私有权威树）⇒ HEAD 本身；
//	③ 读不出（不是 git 检出）⇒ 空串 = **判不了**（✗ 不回落成 Commit：镜像 tip ≠ 私有源提交）。
//
// ★ 一律截 `BuildOriginShortLen` 位；✗ 不带 `+dirty`（脏是**内容**不同，由件的内容身份答）。
package selfupdate

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// BuildOriginShortLen —— 源提交的规范长度（两侧逐字可比的唯一前提）。
// 12 位：trailer 里是 40 位全 sha、`rev-parse --short` 是 7~8 位，不归一化 ⇒ 必然假红。
const BuildOriginShortLen = 12

// buildOrigin 从**一棵源码树**读源提交（空树 / 非 git 检出 ⇒ 空串 = 判不了）。
//
// ★ 读的是目标树，不是本进程：`zerg agent deploy` 构建的那批件来自目标提交，
// 而本进程可能在跑另一笔 —— 拿本进程的身份冒充「这批件的来源」正是本缺口要挡的失真。
func buildOrigin(tree string) string {
	if strings.TrimSpace(tree) == "" {
		return ""
	}
	g := Git{Dir: tree}
	if v := strings.TrimSpace(g.OriginRevID("HEAD")); v != "" {
		return originPrefix(v)
	}
	cmd := exec.Command("git", "-C", tree, "rev-parse", "--short="+strconv.Itoa(BuildOriginShortLen), "HEAD")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return originPrefix(strings.TrimSpace(string(out)))
}

// originPrefix 截成 BuildOriginShortLen 位（短于该长度 ⇒ 原样；空 ⇒ 空）。
func originPrefix(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= BuildOriginShortLen {
		return v
	}
	return v[:BuildOriginShortLen]
}
