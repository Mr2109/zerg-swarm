// archive_mount.go —— 归档区挂/卸口的 **CLI 正门**（`zerg archive mount`）·
// `scripts/svc/archive-mount.sh`（208 行）的 **Go 真搬**（2026-10-08 · M2 一枚 · 承接批 B5）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 3 · A11（原件当时按「落法 (b) 壳转账」
// 只转发、不翻译；本单把**底层实际行为**搬进 Go —— 脚本里可化的一切都不再经脚本本体）。
//
// ★★ 薄壳清单（系统工具 · **物理不可化** ⇒ 保留为子进程调用 · 逐条声明，✗ 不假装全搬）
//
//	① `hdiutil`（默认名 · env `ZERG_HDIUTIL` 可换）：`info` / `attach -readonly -mountpoint` /
//	   `detach`。macOS 原生磁盘镜像工具 —— 其用户态实现落在 DiskImages.framework 私有面里，
//	   Go 标准库与 `golang.org/x/sys` 都**没有**等价 API ⇒ 只能起子进程调（这一层搬不进 Go）。
//	② `mount`（默认名 · env `ZERG_MOUNT_LIST` 可换）：读挂载表。脚本口径就是「读 `mount` 输出的
//	   那几行**文本**再 grep」——判据面（`check-archive-mount` 门㉛）逐字对齐这层文本形态；
//	   Go 侧无稳定的 `getmntinfo` 绑定，且换成 `sysctl`/`getfsstat` 会改**可观察文本** ⇒ 保留子进程调。
//
// ★ 除这两样外**不再有** exec：脚本里 `pwd -P` / `grep -F` / `mkdir -p` / `rmdir` / `ls -1` /
//
//	`head -3` / `tr` / `sleep` / 参数分派 / 幂等判定 / 两处读数交叉核对 / 退码表 / 全部输出文案
//	**一律原生 Go 实现**（`filepath.EvalSymlinks` / `strings.Contains` / `os.MkdirAll` /
//	`os.Remove` / `os.ReadDir` / `time.Sleep`）。
//
// ★ 两条验收口径（本件成立的前提）：
//
//	· **脚本在盘** ⇒ 各档 rc/stdout/stderr 与 `bash scripts/svc/archive-mount.sh <档>` **逐字同**；
//	· **脚本移出仓** ⇒ 本入口读数与「在盘」**逐字同**（本件运行期**零引用**脚本 ⇒ ✗ 不再有
//	  「脚本不在 ⇒ 退 8」那一档 —— 那正是「壳转账」的残留，真搬后归零）。
//
// ★ 未覆盖边（照实声明 · ✗ 不冒充逐字）：
//
//	· `mkdir -p <挂载点>` 失败那一档：脚本在 `set -e` 下由 bash 打自带报错后整脚本退 1，本件
//	  改打一行自撰报错再退 1 —— **正文不同、退码同**（该档需不可写父目录，夹具不覆盖）。
//	· 两处读数（`mount` / `hdiutil info`）为**现读子进程**：两态跑在不同时刻读数可变；对拍一律
//	  用 env 旋钮 `ZERG_MOUNT_LIST` / `ZERG_HDIUTIL` 钉死夹具。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// archiveMountScriptRel —— 原件出处（**仅为两态对拍 / 退场留痕**；本件运行期**零引用**它）。
const archiveMountScriptRel = "scripts/svc/archive-mount.sh"

const (
	amDefaultMountpoint  = "/Volumes/ZERG-ARCHIVE"
	amDefaultArchiveName = "Zerg-归档"
	amDefaultDmgName     = "Zerg-归档-v2.5.10.dmg"

	amExitOK      = 0
	amExitFail    = 1  // 动作跑了但没成 / 回读不等
	amExitBlocked = 2  // 不给结论（前提不可判）
	amExitUsage   = 64 // 用法错误（脚本口径 · ✗ 不是命令面的 2）
)

// amUsage —— `-h`/`--help`/空参 时打印的用法正文。件头**取自**脚本第 2..50 行（真搬留痕），
// 但脚本已于 2026-10-08 退场（DEV-2031）⇒ **命令名与用法行按退场后正名改写为** `zerg archive mount`
// （✗ 不再指向已删除的 `scripts/svc/archive-mount.sh`）；其余照旧。
// amUsageSha256 —— `amUsage`（Go 真源文案）的 sha256 冻结回归值（2026-10-09 · 原件
// `scripts/svc/archive-mount.sh` 于 2026-10-08 退场 DEV-2031 后内化：金标真源由已删脚本
// 第 2..50 行**收回本件常量**本身 —— 退役态无缺口 · ✗ 运行时不读任何脚本）。
// 由 amUsageNativeSha256() 在 Go 内现算自证（见 archive_mount_test.go 的
// TestArchiveMountUsageGoldenInternalized · 抽掉内化即红）。
const amUsageSha256 = "1e72d3d5e50070596a6850a69e5d7ec2ba4e25c660d6fbe98d03443d65a7dca0"

// amUsageNativeSha256 —— 从 Go 真源 `amUsage` 现算 sha256（✗ 不读已退场脚本）：
// 原件退役后命令名/用法行已按正名改写（`zerg archive mount`）⇒ 金标只能内化进 Go 自身才可自证。
func amUsageNativeSha256() string {
	sum := sha256.Sum256([]byte(amUsage))
	return hex.EncodeToString(sum[:])
}

const amUsage = "# zerg archive mount — 归档区**按需挂载**（v2.5.10 归档 dmg · 2026-09-23 Mr2109 拍板）\n#\n# 为什么有它（病根 · 现读）\n#   归档区已被改成「**只读 dmg 镜像 + 原路径同名符号链接**」：\n#     · dmg    = <模型类>/Zerg-归档-v2.5.10.dmg（416,751 B · 只读）\n#     · 挂载点 = /Volumes/ZERG-ARCHIVE\n#     · 原路径 = <模型类>/Zerg-归档 → **符号链接** → 挂载点\n#   两条硬约束（现读实测，不是设计偏好）：\n#     ① 只读 dmg **不能挂在 `<volume-path>` 卷内的任何路径上**（现跑被拒）⇒ 挂载点必须留在 `/Volumes/` 下；\n#     ② Mr2109 口径（逐字）：「肯定是**按需加载** 而不是 一直加载」⇒ **不许**写 launchd 常驻 / 开机自动挂载。\n#   于是「挂」与「摘」只能是**人显式按需**的一次动作 —— 本命令就是那一次动作的命令面：\n#   它**不自启、不常驻、不装 launchd 件**；跑一次、成一次、退一次。\n#\n# 做的事（三个动作 · 各自幂等）\n#   on      `hdiutil attach -readonly -mountpoint <挂载点> <dmg>`；已在挂 ⇒ 如实说「已挂」，**不再挂一遍**\n#   off     `hdiutil detach <挂载点>`；未挂 ⇒ 如实说「未挂」，**不再摘一遍**\n#   status  报挂载态；★ **未挂时明确说明「原路径是悬空符号链接」**——那一刻读它**会失败**，\n#           所以这一档**不是「正常」**（退回码 1，见下）⇒ 这就是 fail-closed 的那一格。\n#\n# 用法：\n#   zerg archive mount on        # 挂上（已在挂则只报「已挂」）\n#   zerg archive mount off       # 摘下（未挂则只报「未挂」）\n#   zerg archive mount status    # 报挂载态（未挂 ⇒ 明说悬空符号链接 + 退 1）\n#\n# 环境（**只为夹具/自测能指到别的件**；默认即本机真物件 —— 照本目录 `fleet-relay.sh` 的\n# `ZERG_RELAY_SSH`「沙箱注入假命令、绝不连真机」同形）：\n#   ZERG_ARCHIVE_DMG      归档 dmg 路径（默认 <模型类>/Zerg-归档-v2.5.10.dmg）\n#   ZERG_ARCHIVE_MOUNT    挂载点（默认 /Volumes/ZERG-ARCHIVE）\n#   ZERG_ARCHIVE_LINK     原路径符号链接（默认 <模型类>/Zerg-归档）\n#   ZERG_HDIUTIL          hdiutil 命令（默认 hdiutil）\n#   ZERG_MOUNT_LIST       挂载表命令（默认 mount）\n#\n# 退出码（**照本目录现成口径** · 与 `fleet-relay.sh` 同一张表）：\n#   0  = 成功（`on`/`off` 动作到位 · `status` 报「已挂」）\n#   1  = 失败（动作跑了但没成 / 回读不等 · `status` 报「**未挂**」——它是按需态，但原路径此刻是\n#        悬空符号链接 ⇒ **不许当正常**）\n#   2  = 不给结论（前提不可判：dmg 不在 / 原路径不是符号链接 / 两处读数打架）\n#  64  = 用法错误\nset -euo pipefail\n\nREPO_ROOT=\"$(cd \"$(dirname \"${BASH_SOURCE[0]}\")/../..\" && pwd)\"\nKIN_ROOT=\"$(cd \"${REPO_ROOT}/..\" && pwd)\"\n\nDMG=\"${ZERG_ARCHIVE_DMG:-${KIN_ROOT}/Zerg-归档-v2.5.10.dmg}\"\nMOUNT=\"${ZERG_ARCHIVE_MOUNT:-/Volumes/ZERG-ARCHIVE}\"\nLINK=\"${ZERG_ARCHIVE_LINK:-${KIN_ROOT}/Zerg-归档}\"\nHDIUTIL=\"${ZERG_HDIUTIL:-hdiutil}\"\nMOUNTLIST=\"${ZERG_MOUNT_LIST:-mount}\"\n\n"

// amConfig —— 五处路径/命令（脚本 45–49 行 · `${VAR:-默认}` 语义：未设**或空**都取默认）。
type amConfig struct {
	dmg       string
	mount     string
	link      string
	hdiutil   string
	mountlist string
}

func amEnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// amLoadConfig —— 逐条照脚本：KIN_ROOT = 仓根的**父目录**（脚本 `cd REPO_ROOT/.. && pwd`）。
func amLoadConfig(root string) amConfig {
	kin := filepath.Dir(root)
	return amConfig{
		dmg:       amEnvOr("ZERG_ARCHIVE_DMG", filepath.Join(kin, amDefaultDmgName)),
		mount:     amEnvOr("ZERG_ARCHIVE_MOUNT", amDefaultMountpoint),
		link:      amEnvOr("ZERG_ARCHIVE_LINK", filepath.Join(kin, amDefaultArchiveName)),
		hdiutil:   amEnvOr("ZERG_HDIUTIL", "hdiutil"),
		mountlist: amEnvOr("ZERG_MOUNT_LIST", "mount"),
	}
}

// ---- 薄壳系统工具（见件头「薄壳清单」）------------------------------------------------------

// amToolOutput —— 跑一次薄壳工具、捕获 stdout（stderr 丢弃 —— 对应脚本里的 `2>/dev/null`）。
// 起不来 / 非零退码 ⇒ ok=false（调用方按脚本口径当「取不到读数」）。
func amToolOutput(name string, args ...string) (string, bool) {
	cmd := armChildGroup(exec.Command(name, args...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// amToolRuns —— 只取「跑不跑得起来」这一个读数（对应 `> /dev/null 2>&1`）。
func amToolRuns(name string, args ...string) bool {
	cmd := armChildGroup(exec.Command(name, args...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	return cmd.Run() == nil
}

// amToolInherit —— 跑薄壳工具并**原样转发**它的 stdout/stderr（`attach`/`detach` 档的脚本行为）。
func amToolInherit(stdout, stderr io.Writer, name string, args ...string) bool {
	cmd := armChildGroup(exec.Command(name, args...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run() == nil
}

// ---- 路径归一 / grep -F / 读数 --------------------------------------------------------------

// amMountReal —— 脚本 `mount_real()`：`( cd MOUNT && pwd -P ) || printf MOUNT`。
// 挂载点不在 ⇒ 原样返回（显式传的串）。
func amMountReal(mount string) string {
	if fi, err := os.Stat(mount); err == nil && fi.IsDir() {
		if rp, err := filepath.EvalSymlinks(mount); err == nil {
			return rp
		}
	}
	return mount
}

// amGrepF —— 复刻 `grep -F <needle>` 的行式面：逐行取含该子串的行，命令替换去尾换行后按原文序
// 用 "\n" 拼接（无命中 ⇒ 空串）。
func amGrepF(text, needle string) string {
	if needle == "" {
		return text // grep -F "" 匹配所有行（本件无此调用，留作同构）
	}
	var hits []string
	for _, ln := range strings.Split(text, "\n") {
		if strings.Contains(ln, needle) {
			hits = append(hits, ln)
		}
	}
	return strings.Join(hits, "\n")
}

// amMountRow —— 脚本 `mount_row()`：先按 MOUNT 原文取，再按 `mount_real` 取；都无 ⇒ 空。
func amMountRow(cfg amConfig) string {
	out, _ := amToolOutput(cfg.mountlist)
	if r := amGrepF(out, " on "+cfg.mount+" ("); r != "" {
		return r
	}
	return amGrepF(out, " on "+amMountReal(cfg.mount)+" (")
}

// amHdiutilRow —— 脚本 `hdiutil_row()`：`hdiutil info` 里含挂载点的那几行（无 ⇒ 空）。
func amHdiutilRow(cfg amConfig) string {
	out, _ := amToolOutput(cfg.hdiutil, "info")
	if r := amGrepF(out, cfg.mount); r != "" {
		return r
	}
	return amGrepF(out, amMountReal(cfg.mount))
}

func amIsMounted(cfg amConfig) bool { return amMountRow(cfg) != "" }

// amSettle —— 脚本 `settle()`：动作后**有界**等读数落地（≤10 次 · 每次 sleep 0.3s）。
func amSettle(cfg amConfig, wantMounted bool) bool {
	for i := 0; i < 10; i++ {
		if amIsMounted(cfg) == wantMounted {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

func amIsSymlink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// amReadable —— `[ -r <路径> ]` 的实用子集（打得开即视为可读）。
func amReadable(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// amSampleList —— 脚本 `ls -1 MOUNT | head -3 | tr '\n' ' '`：前 3 个非隐藏名以空格连接（末尾带空格）。
func amSampleList(mount string) string {
	ents, err := os.ReadDir(mount)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) > 3 {
		names = names[:3]
	}
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, " ") + " "
}

// ---- preflight / cross_check ---------------------------------------------------------------

// amPreflight —— 三件前提（缺一件 ⇒ 不给结论，✗ 不猜）。逐字照脚本 84–103 行。
func amPreflight(stderr io.Writer, cfg amConfig) int {
	if _, err := os.Stat(cfg.dmg); err != nil {
		fmt.Fprintf(stderr, "⛔ 不给结论：归档 dmg 不在盘上（%s）—— 没有可挂的件，本脚本不硬造\n", cfg.dmg)
		return amExitBlocked
	}
	if !amIsSymlink(cfg.link) {
		if _, err := os.Stat(cfg.link); err == nil {
			fmt.Fprintf(stderr, "⛔ 不给结论：原路径 %s **不是符号链接**（是实体）⇒ 本脚本的前提不成立（不许把实体目录当挂载点用）\n", cfg.link)
		} else {
			fmt.Fprintf(stderr, "⛔ 不给结论：原路径 %s 连符号链接都不在 ⇒ 挂上也没有那条路可走\n", cfg.link)
		}
		return amExitBlocked
	}
	if !amToolRuns(cfg.hdiutil, "info") {
		fmt.Fprintf(stderr, "⛔ 不给结论：%s 跑不起来（判据取不到读数 ⇒ 绿就是假绿）\n", cfg.hdiutil)
		return amExitBlocked
	}
	return amExitOK
}

// amCrossCheck —— 挂载表说挂了、`hdiutil info` 不认 ⇒ 两处读数打架 ⇒ 不给结论。
func amCrossCheck(stderr io.Writer, cfg amConfig) int {
	if amIsMounted(cfg) && amHdiutilRow(cfg) == "" {
		fmt.Fprintf(stderr, "⛔ 不给结论：挂载表说 %s 已挂，但 `%s info` 里没有这条挂载点 ⇒ 两处读数打架\n",
			cfg.mount, cfg.hdiutil)
		return amExitBlocked
	}
	return amExitOK
}

// ---- status_line / cmd_status --------------------------------------------------------------

func amStatusLine(stdout io.Writer, cfg amConfig) {
	if amIsMounted(cfg) {
		fmt.Fprint(stdout, "✅ 已挂（按需挂载态里「挂上」的那一态）\n")
		fmt.Fprintf(stdout, "  挂载行 : %s\n", amMountRow(cfg))
		fmt.Fprintf(stdout, "  hdiutil: %s\n", amHdiutilRow(cfg))
		fmt.Fprintf(stdout, "  原路径 : %s → %s（符号链接**解析得通** —— 读它 = 读只读镜像内的件）\n", cfg.link, cfg.mount)
		if amReadable(cfg.mount) {
			fmt.Fprintf(stdout, "  抽验读 : %s\n", amSampleList(cfg.mount))
		}
		return
	}
	fmt.Fprint(stdout, "⚠️ **未挂**（按需挂载态里「没挂」的那一态 —— 这是本命令的**正常业务态**，不是命令坏了）\n")
	fmt.Fprintf(stdout, "  原路径 : %s → %s\n", cfg.link, cfg.mount)
	if fi, err := os.Stat(cfg.mount); err == nil && fi.IsDir() {
		fmt.Fprintf(stdout, "  ★ 挂载点目录 %s 还在（空目录）⇒ 原路径**不报错、会静默返回空** ——\n", cfg.mount)
		fmt.Fprint(stdout, "     别把它当「归档是空的」；要真挂上：`zerg archive mount on`\n")
	} else {
		fmt.Fprintf(stdout, "  ★ 此刻 %s 不存在 ⇒ %s 是**真·悬空符号链接**：读 %s/… 会**明确失败**\n",
			cfg.mount, cfg.link, cfg.link)
		fmt.Fprint(stdout, "     （就是 `No such file or directory` —— 不是「空目录」、更不是「读到空内容」）\n")
	}
	fmt.Fprint(stdout, "  要读归档：先 `zerg archive mount on`；用完 `… off`（按需，不常驻）\n")
}

// amCmdStatus —— 未挂 ⇒ 退 1（**不许报成「正常」** —— 原路径此刻是悬空符号链接，读它会失败）。
func amCmdStatus(stdout, stderr io.Writer, cfg amConfig) int {
	if rc := amPreflight(stderr, cfg); rc != amExitOK {
		return rc
	}
	if rc := amCrossCheck(stderr, cfg); rc != amExitOK {
		return rc
	}
	amStatusLine(stdout, cfg)
	if amIsMounted(cfg) {
		return amExitOK
	}
	return amExitFail
}

// ---- cmd_on --------------------------------------------------------------------------------

func amCmdOn(stdout, stderr io.Writer, cfg amConfig) int {
	if rc := amPreflight(stderr, cfg); rc != amExitOK {
		return rc
	}
	if amIsMounted(cfg) {
		fmt.Fprintf(stdout, "✅ 已挂（幂等：%s 已在挂，本次**不重复挂**）\n", cfg.mount)
		fmt.Fprintf(stdout, "  挂载行 : %s\n", amMountRow(cfg))
		return amExitOK
	}
	if err := os.MkdirAll(cfg.mount, 0o755); err != nil {
		fmt.Fprintf(stderr, "❌ 挂不上：挂载点目录 %s 建不出来：%v\n", cfg.mount, err)
		return amExitFail
	}
	if !amToolInherit(stdout, stderr, cfg.hdiutil, "attach", "-readonly", "-mountpoint", cfg.mount, cfg.dmg) {
		fmt.Fprintf(stderr, "❌ 挂不上：`%s attach -readonly -mountpoint %s %s` 失败\n", cfg.hdiutil, cfg.mount, cfg.dmg)
		fmt.Fprint(stderr, "  常见原因：挂载点被别的卷占着 · 或挂载点落在**只读卷内的路径**上（现读实测：那会被拒）\n")
		return amExitFail
	}
	if !amSettle(cfg, true) {
		fmt.Fprintf(stderr, "❌ 挂载命令返回成功，但挂载表里 %s **不在**（回读不等）⇒ 判失败\n", cfg.mount)
		return amExitFail
	}
	fmt.Fprintf(stdout, "✅ 已挂：%s（只读）\n", cfg.mount)
	fmt.Fprintf(stdout, "  挂载行 : %s\n", amMountRow(cfg))
	fmt.Fprintf(stdout, "  原路径 : %s → %s（符号链接现在解析得通）\n", cfg.link, cfg.mount)
	return amExitOK
}

// ---- cmd_off -------------------------------------------------------------------------------

func amCmdOff(stdout, stderr io.Writer, cfg amConfig) int {
	if rc := amPreflight(stderr, cfg); rc != amExitOK {
		return rc
	}
	if !amIsMounted(cfg) {
		fmt.Fprintf(stdout, "✅ 未挂（幂等：%s 本就不在挂载表里，本次**不重复摘**）\n", cfg.mount)
		return amExitOK
	}
	if !amToolInherit(stdout, stderr, cfg.hdiutil, "detach", cfg.mount) {
		fmt.Fprintf(stderr, "❌ 摘不下：`%s detach %s` 失败（有进程正占着镜像里的件时会被拒）\n", cfg.hdiutil, cfg.mount)
		fmt.Fprintf(stderr, "  先看谁占着：`lsof +D %s`（**不要**换 `-force` 硬摘 —— 那是不可逆动作）\n", cfg.mount)
		return amExitFail
	}
	if !amSettle(cfg, false) {
		fmt.Fprintf(stderr, "❌ 摘除命令返回成功，但挂载表里 %s **还在**（回读不等）⇒ 判失败\n", cfg.mount)
		return amExitFail
	}
	mpNote := "挂载点目录还在（非空 ⇒ 留给下次 on 复用）"
	if fi, err := os.Lstat(cfg.mount); err == nil && fi.IsDir() {
		if os.Remove(cfg.mount) == nil {
			mpNote = "挂载点空目录已收起 ⇒ 原路径现在是**真·悬空符号链接**（读它 = `No such file or directory`）"
		}
	}
	fmt.Fprintf(stdout, "✅ 已摘：%s 不在挂载表里了\n", cfg.mount)
	fmt.Fprintf(stdout, "  %s\n", mpNote)
	fmt.Fprintf(stdout, "  原路径 : %s → %s（要读归档就再 `on` 一次 —— 按需，不常驻）\n", cfg.link, cfg.mount)
	return amExitOK
}

// ---- 入口 ----------------------------------------------------------------------------------

// cmdArchiveMount —— `zerg archive mount [on|off|status]`（Go 真搬 · 不再 exec 脚本本体）。
//
// 退码**逐档照原件**：`0` 成功 · `1` 失败 · `2` 不给结论 · `64` 用法错。
// 尾参**原样分派**（`on`/`off`/`status` 之后的一切忽略 —— 与脚本只读 `$1` 同）；`-h`/`--help`/空参 ⇒ usage。
func cmdArchiveMount(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		root = "." // 与脚本「KIN_ROOT = 仓根父目录」同形；env 旋钮齐全时此处不落进任何默认值。
	}
	cfg := amLoadConfig(root)
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	action := ""
	if len(tail) > 0 {
		action = tail[0]
	}
	switch action {
	case "on":
		return amCmdOn(stdout, stderr, cfg)
	case "off":
		return amCmdOff(stdout, stderr, cfg)
	case "status":
		return amCmdStatus(stdout, stderr, cfg)
	case "-h", "--help", "":
		fmt.Fprint(stdout, amUsage)
		return amExitOK
	default:
		fmt.Fprintf(stderr, "❌ 未知参数: %s（只认 on | off | status）\n", action)
		return amExitUsage
	}
}
