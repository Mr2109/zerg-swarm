// family_model_pull.go —— `zerg model pull <模型@机> --to <本地路径>`（缺口 GAP-20261010-14 · D2 写面）。
//
// 口径（设计稿「设计-model拉取与两机拷贝-v0.1-20261010.md」§2.4 甲案 · §3.1 甲案 · 镜同族 put）：
//
//	① 传输**复用既有 ssh 薄壳 `dplSSH`**（`deploy_launch.go`）＋ `rsync` —— ✗ 不新造第二条 ssh 通路；
//	② ★ `@机` = **对端机（= 源机）**（方向由动词给：put 本机→对端、pull 对端→本机）—— 落点 `--to <本地路径>` **必须显式**；
//	③ **sha256 双侧对拍**：源机件（经 `dplSSH` 回读）⟷ 本机落点（本机算）—— 不一致 ⇒ **判红**（✗ 不许报成功）；
//	④ fail-closed：缺 `--yes` ⇒ 退 2 只出计划件；**源机不在名册 ⇒ 判红并点名**；`@本机` ⇒ 退 2（设计稿 Q8 甲）；
//	   对端不可达 / 回读跑不起来 ⇒ **不给结论**（退 8 · ✗ 别当 0）· 半途失败清残件（设计稿 Q5 甲）。
//
// 退码（照本族既有档 · ✗ 自造码）：0 传输完且双侧 sha 一致 · 1 双侧 sha 不一致（判红）·
// 2 用法错（缺/坏对象 · 缺 --to · 源机不在名册 · `@本机` · 缺 --yes）· 8 不给结论（源件回读不到 /
// 对端不可达 / 本机落点读不到）。判面复用本族 `mvAgree` / `mvDisagree`；sha 助手复用 `mput*`。
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// mpullRsyncBin —— 起 `rsync` 用的程序名（生产 = 系统 `rsync`；测试可注入替身，✗ 不是对外的机器面）。
var mpullRsyncBin = "rsync"

// mpullFields —— `--json` 字段面（序即登记序；命令树 `fields:` 逐字同此 · 与 put 同形、方向名随动词）。
var mpullFields = []string{"egg_id", "model", "machine", "transport", "from", "to",
	"local_sha256", "remote_sha256", "verdict", "note"}

// mpullDisplay —— 人面列（前 7 列）。
var mpullDisplay = []string{"egg_id", "transport", "from", "to", "local_sha256", "remote_sha256", "verdict"}

// mpullRsync —— 跑一次**拉取**传输：`rsync -a --partial -e <ssh 通道> <源机>:<源件> <本地落点>`（stdout/stderr 逐字透传）。
// 返回 (rc, err)：err≠nil ⇒ rsync 程序起不动（缺 `rsync`）。
func mpullRsync(host, remoteSrc, to string, stdout, stderr io.Writer) (int, error) {
	cmd := armChildGroup(exec.Command(mpullRsyncBin, "-a", "--partial", "-e", mputSSHTransport(), host+":"+remoteSrc, to))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
}

// mpullIsLocalMachine —— 判源机名是不是**本机**（逐字命中本机名或其短名 · 复用同族 `devEditHostAliases`）。
func mpullIsLocalMachine(machine string) bool {
	for _, a := range devEditHostAliases() {
		if machine == a {
			return true
		}
	}
	return false
}

// cmdModelPull —— `zerg model pull <模型@机> --to <本地路径> [--dry-run | --yes] [--json <字段>]`
// （缺口 GAP-20261010-14 的 D2 写面：把模型件从**源机**（`@机`）拉到**本机的显式落点**）。
func cmdModelPull(inv *invocation, stdout, stderr io.Writer) int {
	// ── 用法面先判（零副作用 · stdout 0 字节）──
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺对象（模型@机）")
		fmt.Fprintf(stderr, "%s: `model pull` 要一个对象 `<模型@机>`（例：`%s model pull example-35b-v2@<源机> --to <本地路径>`）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一个对象）")
		fmt.Fprintf(stderr, "%s: `model pull` 只吃**一个**对象 `<模型@机>`（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	target := strings.TrimSpace(inv.args[0])
	model, machine, hasAt := strings.Cut(target, "@")
	model, machine = strings.TrimSpace(model), strings.TrimSpace(machine)
	if !hasAt || model == "" || machine == "" {
		inv.setErr("usage", "bad_target", "对象要 `<模型>@<机>` 形态")
		fmt.Fprintf(stderr, "%s: 对象要 `<模型>@<机>` 形态（给的是 %q）—— 缺 `@` / 缺模型 / 缺机\n", progName, target)
		return exitUsage
	}
	to := strings.TrimSpace(inv.to)
	if to == "" {
		inv.setErr("usage", "missing_to", "缺 --to（落点必须显式给）")
		fmt.Fprintf(stderr, "%s: 落点必须**显式给** `--to <本地路径>`（本命令 ✗ 不猜默认路径）\n", progName)
		return exitUsage
	}

	// ── `--json` 字段面前置判定（与 `cmdModelPut` / `cmdModelVerify` 同款）──
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		allowed := map[string]bool{}
		for _, f := range mpullFields {
			allowed[f] = true
		}
		for _, f := range inv.fields {
			if !allowed[f] {
				inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
				return reportBadField(stderr, inv.path, f)
			}
		}
	}

	// ★ 设计稿 Q8 甲：源机 == 本机 ⇒ 拒执并点名（避免无意义往返）。
	if mpullIsLocalMachine(machine) {
		inv.setErr("usage", "pull_self", "源机 == 本机")
		fmt.Fprintf(stderr, "%s: `@机` 指的是**源机**，而 %q 就是**本机** —— pull 自己无意义（退码 2 拒执）\n", progName, machine)
		return exitUsage
	}

	// ── 读名册件（真源）：读不到 ⇒ 8（不给结论）──
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不到名册件（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	fleetRel := filepath.Join("gateway", "fleet.yaml")
	cfg, err := config.LoadFleetConfig(filepath.Join(root, fleetRel))
	if err != nil {
		inv.setErr("blocked", "fleet_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不到名册件 %s：%v（不给结论 · 退码 8）\n", progName, fleetRel, err)
		return exitBlocked
	}
	cands, ok := cfg.Models[model]
	if !ok {
		ids := make([]string, 0, len(cfg.Models))
		for k := range cfg.Models {
			ids = append(ids, k)
		}
		inv.setErr("usage", "unknown_model", "模型不在册")
		fmt.Fprintf(stderr, "%s: 名册里没有 %q（点名不在册）\n", progName, model)
		if s := nearestName(model, ids); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
		}
		return exitUsage
	}
	var cand *config.ModelCandidate
	for i := range cands {
		if cands[i].Host == machine {
			cand = &cands[i]
			break
		}
	}
	if cand == nil {
		// ★ 负控：**源机不在名册 ⇒ 判红并点名**。
		inv.setErr("usage", "unknown_machine", "源机不在名册")
		fmt.Fprintf(stderr, "%s: 名册里没有这台机 %q（**点名不在册**）—— %q 的名册候选里没有 host=%q\n",
			progName, machine, model, machine)
		return exitUsage
	}
	src := strings.TrimSpace(cand.File)
	if src == "" {
		inv.setErr("usage", "source_undeclared", "名册里该候选没给 file")
		fmt.Fprintf(stderr, "%s: 名册里 %s@%s 的候选没给 `file:` ⇒ 无从取源件（退码 2）\n", progName, model, machine)
		return exitUsage
	}
	eggID := model + "@" + machine

	// ── 写面 fail-closed：缺 `--yes` ⇒ 退 2 只出计划件（✗ 零传输）──
	plan := func(w io.Writer) {
		fmt.Fprintf(w, "⬇️  model pull 计划件（%s）\n", eggID)
		fmt.Fprintf(w, "  对象     : %s（`@机` = **源机**）\n", eggID)
		fmt.Fprintf(w, "  源件     : %s:%s（源机）\n", machine, src)
		fmt.Fprintf(w, "  落点     : %s（本机 · **显式给** · ✗ 不猜默认）\n", to)
		fmt.Fprintf(w, "  传输     : rsync -a --partial -e %q <%s:%s> <本机落点>（ssh 薄壳**复用 dplSSH**）\n",
			mputSSHTransport(), machine, src)
		fmt.Fprintf(w, "  对拍     : 源机件回读 sha256（经 dplSSH）⟷ 本机落点 sha256（本机算）—— 不一致 ⇒ 判红（退 1）\n")
		fmt.Fprintf(w, "  授权判据 : 真跑要 `--yes`（D2 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n")
		fmt.Fprintf(w, "  它会动   : 本机落点 %s（源机件与名册件**零改动**）\n", to)
		fmt.Fprintf(w, "  来源     : 缺口账 GAP-20261010-14 · 设计稿 §2.4 甲案 · 同族写面先例 model put（D2）\n")
	}
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun {
		plan(stdout)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未传输、未回读、未改任何状态）")
		return exitOK
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: `model pull` 是 D2 档（真拉取）—— 缺 --yes ⇒ 不执行\n", progName)
		plan(stderr)
		fmt.Fprintf(stderr, "先看计划件：%s model pull %s --to %s --dry-run\n", progName, eggID, to)
		fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}

	// ── 真跑：源机回读 sha256 ⇒ rsync 拉取 ⇒ 本机算 sha ⇒ 双侧对拍 ──
	var rb bytes.Buffer
	rcS, serr := dplSSH(machine, mputRemoteShaCmd(src), &rb, stderr)
	if serr != nil || rcS != 0 {
		inv.setErr("blocked", "readback_failed", fmt.Sprintf("ssh rc=%d", rcS))
		fmt.Fprintf(stderr, "%s: 源机回读跑不起来（ssh rc=%d）⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName, rcS)
		return exitBlocked
	}
	remoteSha := mputFirstToken(rb.String())
	if remoteSha == "" {
		inv.setErr("blocked", "readback_empty", "源机回读没有 sha256")
		fmt.Fprintf(stderr, "%s: 源机回读没给出 sha256 ⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName)
		return exitBlocked
	}
	rcx, terr := mpullRsync(machine, src, to, stdout, stderr)
	if terr != nil {
		inv.setErr("blocked", "rsync_absent", terr.Error())
		fmt.Fprintf(stderr, "%s: 起不动 `%s`：%v ⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName, mpullRsyncBin, terr)
		return exitBlocked
	}
	if rcx != 0 {
		// ★ 半途失败收口（设计稿 Q5 甲）：清 `rsync --partial` 可能留下的残件，再退 8。
		if rmerr := os.Remove(to); rmerr != nil {
			fmt.Fprintf(stderr, "%s: 传输没跑成（rsync rc=%d）—— 清残件 %s 也没成：%v\n", progName, rcx, to, rmerr)
		} else {
			fmt.Fprintf(stderr, "%s: 传输没跑成（rsync rc=%d）—— 已清本机落点残件 %s\n", progName, rcx, to)
		}
		inv.setErr("blocked", "transfer_failed", fmt.Sprintf("rsync rc=%d", rcx))
		fmt.Fprintf(stderr, "%s: 传输没跑成（rsync rc=%d）⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName, rcx)
		return exitBlocked
	}
	localSha, ok := mputSha256File(to)
	if !ok {
		inv.setErr("blocked", "local_unreadable", "本机落点读不到")
		fmt.Fprintf(stderr, "%s: 本机落点读不到 %s ⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName, to)
		return exitBlocked
	}
	verdict := mvAgree
	rc := exitOK
	if !strings.EqualFold(remoteSha, localSha) {
		verdict = mvDisagree
		rc = exitFail
	}
	rows := []map[string]string{{
		"egg_id":        eggID,
		"model":         model,
		"machine":       machine,
		"transport":     "rsync + ssh（复用 dplSSH）",
		"from":          machine + ":" + src,
		"to":            to,
		"local_sha256":  localSha,
		"remote_sha256": remoteSha,
		"verdict":       verdict,
		"note":          "源机件回读（dplSSH `sha256sum`）⟷ 本机落点（本机算）双侧对拍",
	}}
	if inv.jsonGiven {
		if e := emitSelected(stdout, stderr, inv, inv.path, inv.fields, rows); e != exitOK {
			return e
		}
		return rc
	}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		line := make([]string, 0, len(mpullDisplay))
		for _, f := range mpullDisplay {
			line = append(line, r[f])
		}
		table = append(table, line)
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, mpullDisplay, table)
	if rc == exitFail {
		fmt.Fprintf(stderr, "%s: ✗ 双侧 sha256 不一致 ⇒ **判红**（✗ 不许报成功）—— 源机=%s · 本机=%s\n",
			progName, remoteSha, localSha)
	} else {
		fmt.Fprintf(stderr, "%s: ✅ %s 拉取完成 · 双侧 sha256 一致（%s）· 退码 0\n", progName, eggID, localSha)
	}
	return rc
}
