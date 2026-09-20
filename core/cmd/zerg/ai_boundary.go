// ai_boundary.go —— AI 的边界与授权里**不属于某一条命令**的那几件（§九 M18 `C2`/`C4`/`C6` ·
// §十二 `P-099`/`P-101`/`P-103` · §18.2 铁律 Ⅰ/Ⅲ · 开工单 T-60）。
//
// 形状真源 = `core/internal/contract/ai-boundary.json`（生产侧与消费侧读同一份）。
// 本件落四件的**可机检面**：
//
//	① `sudoRefusal` —— 命令面**一律不接受 `sudo`**（`P-103` 定案）。它是**调度前**判的：
//	   命令面是「人与 AI 唯一入口」，那就不能等进了某条命令再谈要不要提权。
//	② `modelSideViolations` —— 模型侧（茧内起引擎的那条 argv / 那份环境）里**零**控制面端口
//	   与 `ZERG_*` 令牌（§九 M18 `C2`）。判据的**扫描面**由真源给，不在代码里另写一份。
//	③ `doorWriteViolations` —— 「放文件即生效」两处口子的**收口①**：命令树里**不许**出现
//	   「往这两处写」的命令（写权限只给人）。
//	④ `doctorSkillItems` / `doctorMCPItems` —— 收口②：启动 / `doctor` 时**列出**全部已加载的
//	   技能与 MCP 工具清单（**默认不静默**）。
//
// ★ 一处**照实说**的边界（不粉饰）：运行期「已连接的 MCP 服务器清单」在**子端侧**，命令面
//
//	（`bin/zerg` 无常驻、不连子端）看不到 ⇒ `doctor` 那一项**明说不可见**，不假装扫过。
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ---- ① `sudo` 拒收（调度前判 · `P-103`）----

// sudoWordRE 匹配「一个**词**形态的 sudo」：
// 词首之前不许是字母/数字/下划线/点/减号（避免 `xsudo` / `no-sudo` 这类误伤），
// 但**允许** `/`（`/usr/bin/sudo` 是同一件事，不许靠写全路径绕过）；词尾同理。
var sudoWordRE = regexp.MustCompile(`(^|[^A-Za-z0-9_.\-])sudo([^A-Za-z0-9_\-]|$)`)

// sudoRefusal —— 命令面这一道：参数位里出现 `sudo` ⇒ 拒（退码 2）。命中返回 true 与命中的那个参数。
//
// 为什么**不区分**是不是子命令：`rules.yaml` 的 `terminal.args_deny` 是**子端侧**的第二道；
// 命令面这一道是「人与 AI 只敲命令」的入口 —— 入口都不该收，才谈得上「一律不接受」。
func sudoRefusal(args []string) (string, bool) {
	for _, a := range args {
		if sudoWordRE.MatchString(a) {
			return a, true
		}
	}
	return "", false
}

// ---- ② 模型侧零令牌（扫描面取自真源）----

// modelSideViolation —— 一条命中（相对仓根的路径 + 行号 + 原文）。
type modelSideViolation struct {
	File string
	Line int
	Text string
}

// tokenWordRE 把一枚令牌编成**词边界**匹配（数字串两侧不许是其它数字/字母/下划线）。
// ★ 为什么必须词边界：实测 `agent/internal/hatch/hatch_test.go` 里有 `--port 58100` 夹具 ——
// 裸子串匹配会把 `58100` 判成含 `8100`，那是**假阳性**，而假阳性会把判据变成噪声。
func tokenWordRE(tok string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[^0-9A-Za-z_])` + regexp.QuoteMeta(tok) + `([^0-9A-Za-z_]|$)`)
}

// modelSideViolations —— 扫真源声明的每一个面，逐行找禁令牌（端口 + `ZERG_` 前缀）。
//
// 返回 (命中, 扫描到的文件数, error)：文件数用来判**空转**（0 件被扫到 ⇒ 判据自己坏了，
// 由调用方给 2，**不许**当成「全绿」）。
func modelSideViolations(repoRoot string, spec *contract.ModelSideScan) ([]modelSideViolation, int, error) {
	res := []*regexp.Regexp{}
	for _, p := range spec.Ports {
		res = append(res, tokenWordRE(p))
	}
	if spec.EnvPrefix != "" {
		res = append(res, regexp.MustCompile(`(^|[^0-9A-Za-z_])`+regexp.QuoteMeta(spec.EnvPrefix)+`[A-Za-z0-9_]*`))
	}
	exclude := map[string]bool{}
	for _, s := range spec.ScanExcludeSuffixes {
		exclude[s] = true
	}
	out := []modelSideViolation{}
	scanned := 0
	for _, rel := range spec.ScanPaths {
		base := filepath.Join(repoRoot, filepath.FromSlash(rel))
		if _, err := os.Stat(base); err != nil {
			continue // 那个面不存在 = 没东西可扫（是不是错由「文件数」判）
		}
		err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			for suf := range exclude {
				if strings.HasSuffix(d.Name(), suf) {
					return nil
				}
			}
			if !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			body, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			scanned++
			rel2, _ := filepath.Rel(repoRoot, p)
			for i, line := range strings.Split(string(body), "\n") {
				for _, re := range res {
					if re.MatchString(line) {
						out = append(out, modelSideViolation{File: filepath.ToSlash(rel2), Line: i + 1, Text: strings.TrimSpace(line)})
						break
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, scanned, err
		}
	}
	return out, scanned, nil
}

// ---- ③ 「放文件即生效」两处口子的收口①：命令树里不许有「往这两处写」的命令 ----

// doorWriteRE 把真源里的口子关键词与写动词编成一条**同一行内**的命中式。
var doorWriteVerbs = []string{"写", "新增", "删除", "改", "覆盖"}

// doorCmd —— 判定口要吃的最小形状（**抽出来是为了让负控能喂合成的命令进来** ——
// 直接读全局 `commands` 的判定口没法做负控，那样它只能是「恒绿装置」）。
type doorCmd struct {
	Path    string
	Summary string
	Usage   string
	Args    []string
}

// doorWriteViolationsFor —— 对给定的一组命令判「有没有往两处口子写的命令」：
// 同一条的 用法串 / 概要 / 位置参数里**同时**出现口子关键词与写动词 ⇒ 红（**写权限只给人**）。
func doorWriteViolationsFor(spec *contract.AIBoundarySpec, cmds []doorCmd) []string {
	out := []string{}
	for _, c := range cmds {
		text := strings.Join([]string{c.Summary, c.Usage, strings.Join(c.Args, " ")}, " · ")
		hitDoor := ""
		for _, k := range spec.DoorWriteKeywords {
			if strings.Contains(text, k) {
				hitDoor = k
				break
			}
		}
		if hitDoor == "" {
			continue
		}
		for _, v := range doorWriteVerbs {
			if strings.Contains(text, v) {
				out = append(out, fmt.Sprintf("%s（命中口子 %q + 写动词 %q）", c.Path, hitDoor, v))
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// doorWriteViolations —— 扫**真命令树**（`commands` 是同一份真源）。
func doorWriteViolations(spec *contract.AIBoundarySpec) []string {
	cs := []doorCmd{}
	for _, c := range commands {
		cs = append(cs, doorCmd{Path: strings.Join(c.path, " "), Summary: c.summary, Usage: c.usage, Args: c.args})
	}
	return doorWriteViolationsFor(spec, cs)
}

// ---- ④ doctor 的两段清单（`P-101` ②：默认不静默）----

// doctorSkillItems —— 「技能目录」项：**逐条列出**已加载技能（读不到 ⇒ BLOCKED，不当「没有」）。
func doctorSkillItems() []map[string]string {
	dir := statepath.SkillsDir()
	if dir == "" {
		return []map[string]string{{
			"name": "技能目录（P-101 ②）", "verdict": "BLOCKED",
			"detail": "解析不到技能目录（`ZERG_SKILLS_DIR` 未设且仓内 core/internal/agent/skills 不在）",
			"advice": "设 ZERG_SKILLS_DIR 或在仓内跑 —— 「读不到」不当「没有」"}}
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return []map[string]string{{
			"name": "技能目录（P-101 ②）", "verdict": "BLOCKED",
			"detail": fmt.Sprintf("读不了技能目录 %s：%v", dir, err),
			"advice": "查权限 —— 同「读不到不当没有」"}}
	}
	names := []string{}
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return []map[string]string{{
		"name": "技能目录（P-101 ②）", "verdict": "PASS",
		"detail": fmt.Sprintf("%s · %d 件：%s", dir, len(names), strings.Join(names, ", ")),
		"advice": "写权限只给人（AI 不写这一处）—— 要加技能由人放件"}}
}

// doctorMCPItems —— 「MCP 工具清单」项：**明说不可见**（运行期清单在子端侧），不假装扫过。
func doctorMCPItems() []map[string]string {
	return []map[string]string{{
		"name": "MCP 工具清单（P-101 ②）", "verdict": "REPORT",
		"detail": "命令面侧**不可见**：MCP 服务器由子端启动时的 `--mcp` 声明（无持久登记表）⇒ 本项列出「可声明的来源 = 0 条」，**运行期已连接的清单在子端侧**",
		"advice": "查真清单看子端启动日志（`zerg agent logs <机>`）；写权限只给人（AI 不写这一处）"}}
}

// ---- 小工具（给两处调用共用：把一组命中打成一行判词）----

func joinViolations(vs []modelSideViolation, max int) string {
	parts := []string{}
	for i, v := range vs {
		if i >= max {
			parts = append(parts, fmt.Sprintf("…（共 %d 条）", len(vs)))
			break
		}
		parts = append(parts, fmt.Sprintf("%s:%d %s", v.File, v.Line, v.Text))
	}
	return strings.Join(parts, " | ")
}

// writeSudoRefusal —— 拒收的**唯一**输出口（命令面与测试共用，不两处各写一份文案）。
func writeSudoRefusal(inv *invocation, hit string, stdout, stderr io.Writer) int {
	spec, err := contract.AIBoundary()
	if err != nil {
		inv.setErr("blocked", "contract_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: AI 边界真源读不出来：%v ⇒ 不给结论\n", progName, err)
		return exitBlocked
	}
	inv.setErr("usage", spec.Sudo.Detail, fmt.Sprintf("参数里出现 `sudo`（%q）—— 命令面一律不接受", hit))
	fmt.Fprintf(stderr, "%s: 参数里出现 `sudo`（%q）—— **命令面一律不接受**（§十二 `P-103` 定案）\n", progName, hit)
	fmt.Fprintf(stderr, "口径：① 命令面不收；② AI 侧（含 bash 工具）不敲 sudo；③ **要提权的事走人**\n")
	fmt.Fprintf(stderr, "（子端侧第二道在 `rules.yaml` 的 `terminal.args_deny`；本道是「人与 AI 只敲命令」的入口）\n")
	return exitUsage
}
