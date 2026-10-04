package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// family_publish_run.go —— `zerg publish preflight|run`（缺口 `Q-223`：把发布流水线接进 CLI）。
//
// 判据（2026-09-25 现读 CLI 自证）：`zerg publish` 家族**只有** `publish tree has`（只读查树）✗；
// 与发布沾边者只有 `build release`（计划面开放 · 真跑拒执退 2）与 `build all --only cli`（自举档）
// ⇒ **出公开镜像 + 预检只能手跑脚本** ⇒ 本日因此手搓一路（猜路径/猜参数/编错件）✗，
// Mr2109 判定「频繁犯错 = 没以 CLI 为主要工作命令」。
//
// 口径逐条照 `family_build.go` 的 `cmdBuildPassthrough`（计划件 → 拒执/确认 → **原样转发脚本退码**）：
//   · `publish preflight <产物目录>` —— **只读**（不改任何状态）⇒ 不设 D3 确认；
//   · `publish run [--out <目录>]`  —— 真跑只写**本地**产物目录：**脚本自己的默认落点**是
//     `/tmp/zerg-public-<时间戳>`（底脚本第 63 行的 `OUT=\"${OUT:-/tmp/zerg-public-${TS}}\"` · 逐字现读）；
//     给了 `--out <目录>` 就用给的那个 —— `--out` **原样透传**给脚本（旧状「收下即丢」✗，已修）。
//     不出网、不推送 ⇒ 设 D3 三态确认。
// **「推远端」不在命令面**：不可逆动作按规矩等 Mr2109 发话后单独执行（不另开执行路径）。
//
// ★ 诚实标注（2026-09-26 现读对拍结论 · 逐字见 `publishRunPathNotice`）：**本入口走的是压平快照器
//   （已弃用 · 非公开面真源）；真源 = `publish/mirror-public.sh`（规则指纹/全树模式长在它身上）**。
//   ⇒ 真跑出的树只当**本地素材**看，不许当「公开面那一份」的证据；切换主路要人拍（本笔不切）。
//
// ★ 缺口 `GAP-20260926-21`（2026-09-26 落账 · 2026-10-04 拍板）—— 「`publish run --dry-run` 只打
//   静态文案、根本不落到发布器」：干跑过去**纯静态**（8 行写死的 `fmt.Fprintf` + `return exitOK`）⇒
//   **全树 / 差分 / 规则指纹在 CLI 面一样都看不到**。父代拍：**dry-run 语义照现有同族先例办**
//   —— 与真源的只读预演 `mirror-public-lib.py --list-only` **同形**。
//   ⇒ 本笔：`--dry-run` **先跑真源的只读档**（`publish/mirror-public-lib.py --list-only` · **零写盘 ·
//   零联网 · 零推送**）再把它的**真数字**搬进计划件 —— 逐字给：**会发 N 件 / 跳过 M 件（分类计数）/
//   空指 K 条 / 规则指纹**（`rules_fingerprint` **一字不改**地打）。
//   ★ **不另算一套** ✗：读数一律取真源自己那行机器面（`MIRROR_LIST_ONLY {json}`），本件**不**在 Go 侧
//   复算指纹/集合（复算 = 第二份口径 ⇒ 与真源漂移谁也看不见）。
//   ★ **出口三态**（与 `publish set` / `publish tree has` 同一条纪律「答案缺身份 ⇒ 宁可不答」）：
//     真源只读预演**拿不到结论**（仓根解析不到 / 脚本不在 / 无 `python3` / 退非 0 / 机器面缺 / 解析不了 /
//     自报非零副作用）⇒ **退码 8 · stdout 一行都不出**（计划件不许建在不洁读数上）。
//   ★ 本笔**只动干跑档**：真跑执行路径仍走压平快照器（切真源 = `GAP-20260926-22` · 另一笔）⇒
//     `走哪一路` 那条诚实标注**保留**（执行面事实未变）；切主路那一笔里它自会消失。

// publishRunPathNotice —— 「本入口走哪一路」的**逐字**标注（两个位置共用同一份文字 · 不许含糊）。
// 两个位置：① `--dry-run` 计划件里那一行；② 真跑执行前 stderr 那一行。
const publishRunPathNotice = "本入口走：压平快照器（已弃用 · **非公开面真源**）；真源 = `publish/mirror-public.sh`（规则指纹/全树模式长在它身上）"

// publishRunDefaultOut —— 底脚本（`scripts/build/publish-public.sh`）**不给 `--out` 时的默认落点**
// （逐字照它第 63 行；`<时间戳>` 是脚本自己的 `TS`，不由本命令编）。
const publishRunDefaultOut = "/tmp/zerg-public-<时间戳>"

// publishRunListOnlyPrefix —— 真源只读预演末尾那行机器面的**逐字前缀**（`mirror-public-lib.py`
// `list_only_report()` 打印；本件按它取数，✗ 不解析人面散文）。
const publishRunListOnlyPrefix = "MIRROR_LIST_ONLY "

// publishRunSkipLabels —— 跳过件的**分类计数**顺序（逐字对齐真源 `SKIP_LABELS`：`publish/mirror-public-lib.py`）。
// 顺序也照真源 —— 两处的分类口径必须同源，否则预演读数与真跑对不上。
var publishRunSkipLabels = []string{"EXCLUDES", "DROP_EXACT", "DROP_PREFIX", "映射丢弃", "非 blob"}

// publishListOnly —— 真源只读预演（`--list-only`）机器面里本入口要的那几格。
// 字段名逐字对齐 `mirror-public-lib.py:1177` 那行 `MIRROR_LIST_ONLY {json}`。
type publishListOnly struct {
	WillSend         int            `json:"will_send"`
	Skipped          int            `json:"skipped"`
	SkipByTable      map[string]int `json:"skip_by_table"`
	EmptyPointers    int            `json:"empty_pointers"`
	EmptyByTable     map[string]int `json:"empty_by_table"`
	Whitelisted      int            `json:"whitelisted"`
	Tracked          int            `json:"tracked"`
	PrivateFaceHits  int            `json:"private_face_hits"`
	OutTouched       bool           `json:"out_touched"`
	Network          bool           `json:"network"`
	RulesFingerprint string         `json:"rules_fingerprint"`
	ListOnly         bool           `json:"list_only"`
}

// publishRunSkipBreakdown —— 把分类计数按真源顺序拼成一行（`EXCLUDES a · DROP_EXACT b · …`）。
func publishRunSkipBreakdown(m map[string]int) string {
	parts := make([]string, 0, len(publishRunSkipLabels))
	for _, k := range publishRunSkipLabels {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(parts, " · ")
}

// publishRunListOnly —— 调**真源的只读预演**并取回机器面（`publish/mirror-public-lib.py --list-only`）。
//
// 那条路本身**零写盘 / 零联网**（它不调 `MirrorRepo.init()`、无 push/ssh/curl 子进程 —— 真源自证）。
// `why != ""` ⇒ **拿不到结论**（调用方按同族纪律退 8 · stdout 一行都不出）。
func publishRunListOnly(root string) (*publishListOnly, string) {
	libRel := "publish/mirror-public-lib.py"
	if _, err := os.Stat(filepath.Join(root, libRel)); err != nil {
		return nil, fmt.Sprintf("真源只读预演脚本不在（%s）", libRel)
	}
	py, err := exec.LookPath("python3")
	if err != nil {
		return nil, "找不到 python3（真源只读预演要它）"
	}
	cmd := armChildGroup(exec.Command(py, filepath.Join(root, libRel), "--list-only"))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Sprintf("真源只读预演跑不动：%v（%s）", err, strings.TrimSpace(errBuf.String()))
	}
	line := ""
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, publishRunListOnlyPrefix) {
			line = strings.TrimPrefix(l, publishRunListOnlyPrefix)
		}
	}
	if line == "" {
		return nil, "真源只读预演没给出机器面（缺 `MIRROR_LIST_ONLY` 一行）"
	}
	lo := &publishListOnly{}
	if err := json.Unmarshal([]byte(line), lo); err != nil {
		return nil, fmt.Sprintf("真源只读预演的机器面解析不了：%v", err)
	}
	// 自洁性防守：真源自报**非零副作用** ⇒ 拒（计划件不许建在不洁读数上）。
	if lo.OutTouched || lo.Network {
		return nil, fmt.Sprintf("真源只读预演自报**非零副作用**（out_touched=%t · network=%t）", lo.OutTouched, lo.Network)
	}
	return lo, ""
}

// cmdPublishPreflight —— `zerg publish preflight <产物目录>`：跑公开面预检（只读）。
func cmdPublishPreflight(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺产物目录")
		fmt.Fprintf(stderr, "%s: `publish preflight` 要一个产物目录（`publish run` 出镜像的那个目录）\n", progName)
		fmt.Fprintf(stderr, "用法：%s publish preflight <产物目录>\n", progName)
		return exitUsage
	}
	dir := inv.args[0]
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动预检（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := "scripts/build/publish-preflight.sh"
	if _, err := os.Stat(filepath.Join(root, script)); err != nil {
		inv.setErr("blocked", "script_absent", "预检脚本不在")
		fmt.Fprintf(stderr, "%s: 预检脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, script)
		return exitBlocked
	}
	fmt.Fprintf(stderr, "%s: 公开面预检（**只读** · 不改任何状态）—— bash %s %s\n", progName, script, dir)
	return runPublishChild(inv, root, script, []string{dir}, stdout, stderr)
}

// cmdPublishRun —— `zerg publish run [--out <目录>] [--dry-run | --confirm=<主机名> --yes]`：本地出公开镜像树。
func cmdPublishRun(inv *invocation, stdout, stderr io.Writer) int {
	const script = "scripts/build/publish-public.sh"
	// 落点口径 = **实落**（三修之二）：旧文案写 `dist/<版本>/release`，而脚本实落
	// `/tmp/zerg-public-<时间戳>`（给 `--out` 则用给的那个）⇒ 照错落点去找件 = 白跑一趟。
	out := strings.TrimSpace(inv.flagVal("--out"))
	landing := publishRunDefaultOut
	if out != "" {
		landing = out
	}
	effect := fmt.Sprintf("在**本地** `%s` 生成公开镜像树（白名单 + 排除项 + 脱敏替换 —— 底脚本不给 `--out` 时的默认落点就是 `%s`）—— **不出网、不推送**",
		landing, publishRunDefaultOut)
	scriptArgs := ""
	if out != "" {
		scriptArgs = " --out " + out
	}
	// ★ `--out` 旧状 = **收下即丢**（传 `args=nil` ⇒ 给 `--out <目录>` 仍落默认点，且 rc=0 不报错）。
	// 现在两态都**不静默**：① 给了旗标没给值 ⇒ 退 2；② 指的落点**已存在且不是目录** ⇒ 退 2
	// （底脚本会先 `rm -rf` 再 `mkdir` ⇒ 那个件会被**静默删掉** —— 不许吞）。
	if inv.hasFlag("--out") && out == "" {
		msg := "`--out` 给了旗标没给值 ⇒ **不许静默当「没给」**（给个目录：`--out <目录>`）"
		inv.setErr("usage", "out_value_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "用法：%s publish run --out <目录> [--dry-run | --confirm=%s --yes]\n", progName, planHost())
		return exitUsage
	}
	if out != "" {
		if fi, err := os.Stat(out); err == nil {
			if !fi.IsDir() {
				msg := fmt.Sprintf("`--out` 指的 `%s` **已经存在、而且不是目录** ⇒ 拒绝执行（底脚本会先 `rm -rf` 再 `mkdir` ⇒ 那个件会被静默删掉）", out)
				inv.setErr("usage", "out_not_a_dir", msg)
				fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
				return exitUsage
			}
			// 已存在的目录会被**清空重建**（底脚本 `rm -rf` + `mkdir -p`）—— 照实说一声，不当没看见。
			if ents, derr := os.ReadDir(out); derr == nil && len(ents) > 0 {
				fmt.Fprintf(stderr, "%s: 注意 —— `--out` 指的 `%s` 已存在且**非空**（%d 项）：底脚本会 `rm -rf` 后重建 ⇒ 里面原来的东西会被清掉\n",
					progName, out, len(ents))
			}
		}
	}
	if inv.dryRun {
		return publishRunDryRun(inv, stdout, stderr, script, scriptArgs, effect)
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动发布（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	if _, err := os.Stat(filepath.Join(root, script)); err != nil {
		inv.setErr("blocked", "script_absent", "发布脚本不在")
		fmt.Fprintf(stderr, "%s: 发布脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, script)
		return exitBlocked
	}
	// D3 三态：--confirm 的值必须与主机名**逐字相同** + --yes 同时到（§4.1 K7 / §6.3 S5）。
	host := planHost()
	if !inv.confirmGiven {
		msg := "`publish run` 会写产物目录 ⇒ 按 D3 档：**缺确认 ⇒ 不执行**"
		inv.setErr("usage", "confirm_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=%s --yes\n", host)
		fmt.Fprintf(stderr, "先看计划件：%s publish run --dry-run\n", progName)
		return exitUsage
	}
	if !devEditHostAccept(inv.confirm) {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，本机主机名是 %q）⇒ 不执行", inv.confirm, host)
		inv.setErr("usage", "confirm_mismatch", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与目标**逐字相同** —— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	if !inv.yes {
		msg := "`publish run` 是 D3 档 —— **缺 --yes ⇒ 不执行**"
		inv.setErr("usage", "yes_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitUsage
	}
	// ★ 诚实标注（同级第二处 · 与计划件那一行**逐字同一份文字**）：执行前先把「本入口走哪一路」说清。
	fmt.Fprintf(stderr, "%s: %s\n", progName, publishRunPathNotice)
	fmt.Fprintf(stderr, "%s: 发布档执行 —— bash %s%s\n", progName, script, scriptArgs)
	args := []string{}
	if out != "" {
		args = append(args, "--out", out)
	}
	return runPublishChild(inv, root, script, args, stdout, stderr)
}

// publishRunDryRun —— `zerg publish run --dry-run`：**真源只读预演 + 计划件**（零副作用）。
//
// 与旧状的区别（缺口 `GAP-20260926-21`）：旧状纯静态（8 行写死文案，集合并集/规则指纹一个字都没有）；
// 本状**先跑真源的只读档**（`publish/mirror-public-lib.py --list-only`，零写盘/零联网/零推送）再把
// 它的**真数字**逐字搬进计划件 —— 「会发 / 跳过（分类）/ 空指 / 规则指纹」全部与真源现读**同源**。
//
// 出口三态（同族纪律，`publish set` / `publish tree has`）：真源只读预演**拿不到结论** ⇒
// **退码 8 · stdout 一行都不出**（计划件不许建在不洁读数上 —— 静默给个「会发 0 件」比不答更坏）。
func publishRunDryRun(inv *invocation, stdout, stderr io.Writer, script, scriptArgs, effect string) int {
	// ① 真源只读预演（先取数，取不到就**不出**计划件）。
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 真源只读预演跑不动 ⇒ **不给结论**（退码 8 · stdout 一行都不出）\n", progName)
		return exitBlocked
	}
	lo, why := publishRunListOnly(root)
	if why != "" {
		inv.setErr("blocked", "list_only_unavailable", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ **不给结论**（退码 8 · stdout 一行都不出）\n", progName, why)
		fmt.Fprintf(stderr, "%s: 口径：计划件里的「会发/跳过/空指/规则指纹」必须与真源现读**同源**（`publish/mirror-public-lib.py --list-only` · 不另算一套）\n", progName)
		return exitBlocked
	}

	// ② 计划件（人面）—— 保留既有诸行，并把真源只读预演的真数字逐字搬进来。
	fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
	fmt.Fprintf(stdout, "  动作     : %s publish run\n", progName)
	fmt.Fprintf(stdout, "  危险档   : D3（发布档：会写产物目录；命中私有面门禁 ⇒ 整批自中止，一个字节都不推）\n")
	fmt.Fprintf(stdout, "  它会调   : bash %s%s\n", script, scriptArgs)
	fmt.Fprintf(stdout, "  它会动   : %s\n", effect)
	fmt.Fprintf(stdout, "  走哪一路 : %s\n", publishRunPathNotice)
	// ── 真源只读预演（`--list-only` 同形）：读数一律取真源自己那行机器面，本件不在 Go 侧复算 ──
	fmt.Fprintf(stdout, "  只读预演 : 真源 `publish/mirror-public-lib.py --list-only`（零写盘 · 零联网 · 零推送 —— 本入口的读数来源）\n")
	fmt.Fprintf(stdout, "  会发     : %d 件 / 跳过 %d 件（%s）/ 空指 %d 条\n",
		lo.WillSend, lo.Skipped, publishRunSkipBreakdown(lo.SkipByTable), lo.EmptyPointers)
	fmt.Fprintf(stdout, "  规则指纹 : %s（**一字不改** —— 与真源现读逐字相同）\n", lo.RulesFingerprint)
	fmt.Fprintf(stdout, "  对账     : 私有 HEAD 树 tracked %d 件 · 白名单命中 %d · 会发+跳过=%d · 私有面命中物化路径 %d\n",
		lo.Tracked, lo.Whitelisted, lo.WillSend+lo.Skipped, lo.PrivateFaceHits)
	fmt.Fprintf(stdout, "  只读证明 : out_touched=%t · network=%t（未建产出仓 · 未联网 · 未推送）\n", lo.OutTouched, lo.Network)
	fmt.Fprintf(stdout, "  执行要   : --confirm=<主机名> 与 --yes 同时到\n")
	fmt.Fprintf(stdout, "  本版状态 : **已开放**（Mr2109 2026-09-25「脱敏推」授权：出镜像属本地动作；**推远端另行发话**）\n")
	fmt.Fprintf(stdout, "  下一步   : %s publish preflight <产物目录>（只读复核 —— 目录用上面「它会动」那一行的落点）\n", progName)
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
	return exitOK
}

// runPublishChild 调脚本并把退码**原样转发**（与 gate 族/构建族同一条口径 · §4.3 U1）。
func runPublishChild(inv *invocation, root, script string, args []string, stdout, stderr io.Writer) int {
	cmd := armChildGroup(exec.Command("bash", append([]string{filepath.Join(root, script)}, args...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(stderr, "%s: %s 退 %d（原样转发，不翻译）\n", progName, filepath.Base(script), ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "publish_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动 %s：%v\n", progName, script, err)
		return exitFail
	}
	return exitOK
}
