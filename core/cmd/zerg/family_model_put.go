// family_model_put.go —— `zerg model put <模型@机> --to <路径>`（缺口 GAP-20260925-25 · D2 写面）。
//
// 口径（父代理自决「补哪一段：挂 model 族」· 本单只做 put）：
//
//	① 传输**复用既有 ssh 薄壳 `dplSSH`**（`deploy_launch.go`）＋ `rsync` —— ✗ 不新造第二条 ssh 通路：
//	   rsync 的 `-e` 就用 `dplSSHBin` 与同一组 `-o BatchMode=yes -o ConnectTimeout=10`（一份口径）；
//	② 落点**必须显式给**（`--to <路径>`）—— ✗ 不猜默认路径；
//	③ **sha256 双侧对拍**：发前本地算 ⟷ 对端（经 `dplSSH` 回读）—— 不一致 ⇒ **判红**（✗ 不许报成功）；
//	④ fail-closed：缺 `--yes` ⇒ 退 2 只出计划件；**目标机不在名册 ⇒ 判红并点名**；
//	   对端不可达 / 命令跑不起来 ⇒ **不给结论**（退 8 · ✗ 别当 0）。
//
// 退码（照本族既有档 · ✗ 自造码）：0 传输完且双侧 sha 一致 · 1 双侧 sha 不一致（判红）·
// 2 用法错（缺/坏对象 · 缺 --to · 目标机不在名册 · 缺 --yes）· 8 不给结论（读不到源件 /
// 对端不可达 / 回读跑不起来）。判面复用本族 `mvAgree` / `mvDisagree`（verdict 三档闭集）。
package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// mputRsyncBin —— 起 `rsync` 用的程序名（生产 = 系统 `rsync`；测试可注入替身，✗ 不是对外的机器面）。
var mputRsyncBin = "rsync"

// mputFields —— `--json` 字段面（序即登记序；命令树 `fields:` 逐字同此）。
var mputFields = []string{"egg_id", "model", "machine", "transport", "from", "to",
	"local_sha256", "remote_sha256", "verdict", "note"}

// mputDisplay —— 人面列（前 7 列）。
var mputDisplay = []string{"egg_id", "transport", "from", "to", "local_sha256", "remote_sha256", "verdict"}

// mputSSHTransport —— rsync 的 ssh 通道：**复用 dplSSH 那一组**（同一程序名 + 同一组 `-o`）。
func mputSSHTransport() string {
	return dplSSHBin + " -o BatchMode=yes -o ConnectTimeout=10"
}

// mputRemoteShaCmd —— 对端回读命令（经 `dplSSH` 跑）：先试 `sha256sum`、再退 `shasum -a 256`。
// 路径经 `dplBashQ` 引号化传入 `sh -c` 的 `$1` ⇒ 值**不作代码内插**（与 `dplSSH` 同一条引号纪律）。
func mputRemoteShaCmd(path string) string {
	return "sh -c 'command -v sha256sum >/dev/null 2>&1 && exec sha256sum \"$1\"; exec shasum -a 256 \"$1\"' _ " + dplBashQ(path)
}

// mputSha256File —— 本机件 sha256（十六进制小写）；读不到 ⇒ ok=false（调用方判「不给结论」）。
func mputSha256File(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false
	}
	return fmt.Sprintf("%x", h.Sum(nil)), true
}

// mputFirstToken —— 取回读输出的第一个空白分隔串（`sha256sum` / `shasum` 都印 `<hash>␠␠<path>`）。
func mputFirstToken(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

// mputRsync —— 跑一次传输：`rsync -a --partial -e <ssh 通道> <源件> <机>:<落点>`（stdout/stderr 逐字透传）。
// 返回 (rc, err)：err≠nil ⇒ rsync 程序起不动（缺 `rsync`）。
func mputRsync(host, src, to string, stdout, stderr io.Writer) (int, error) {
	cmd := armChildGroup(exec.Command(mputRsyncBin, "-a", "--partial", "-e", mputSSHTransport(), src, host+":"+to))
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

// cmdModelPut —— `zerg model put <模型@机> --to <路径> [--dry-run | --yes] [--json <字段>]`
// （缺口 GAP-20260925-25 的 D2 写面：把模型件从本机搬到目标机的**显式落点**）。
func cmdModelPut(inv *invocation, stdout, stderr io.Writer) int {
	// ── 用法面先判（零副作用 · stdout 0 字节）──
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺对象（模型@机）")
		fmt.Fprintf(stderr, "%s: `model put` 要一个对象 `<模型@机>`（例：`%s model put example-35b-v2@<机> --to <路径>`）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一个对象）")
		fmt.Fprintf(stderr, "%s: `model put` 只吃**一个**对象 `<模型@机>`（多给的是 %q）\n", progName, inv.args[1])
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
		fmt.Fprintf(stderr, "%s: 落点必须**显式给** `--to <路径>`（本命令 ✗ 不猜默认路径）\n", progName)
		return exitUsage
	}

	// ── `--json` 字段面前置判定（与 `cmdModelVerify` / `cmdModelShow` 同款）──
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		allowed := map[string]bool{}
		for _, f := range mputFields {
			allowed[f] = true
		}
		for _, f := range inv.fields {
			if !allowed[f] {
				inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
				return reportBadField(stderr, inv.path, f)
			}
		}
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
		// ★ 负控：**目标机不在名册 ⇒ 判红并点名**。
		inv.setErr("usage", "unknown_machine", "目标机不在名册")
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
		fmt.Fprintf(w, "⬆️  model put 计划件（%s）\n", eggID)
		fmt.Fprintf(w, "  对象     : %s\n", eggID)
		fmt.Fprintf(w, "  源件     : %s（本机）\n", src)
		fmt.Fprintf(w, "  落点     : %s:%s（**显式给** · ✗ 不猜默认）\n", machine, to)
		fmt.Fprintf(w, "  传输     : rsync -a --partial -e %q <源件> <%s:%s>（ssh 薄壳**复用 dplSSH**）\n",
			mputSSHTransport(), machine, to)
		fmt.Fprintf(w, "  对拍     : 发前本机 sha256 ⟷ 对端回读（经 dplSSH `sha256sum`）—— 不一致 ⇒ 判红（退 1）\n")
		fmt.Fprintf(w, "  授权判据 : 真跑要 `--yes`（D2 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不执行）\n")
		fmt.Fprintf(w, "  它会动   : 目标机落点 %s（本机源件与名册件**零改动**）\n", to)
		fmt.Fprintf(w, "  来源     : 缺口账 GAP-20260925-25 · §三 E 族 · 同族写面先例 model add（D2）\n")
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
		fmt.Fprintf(stderr, "%s: `model put` 是 D2 档（真传输）—— 缺 --yes ⇒ 不执行\n", progName)
		plan(stderr)
		fmt.Fprintf(stderr, "先看计划件：%s model put %s --to %s --dry-run\n", progName, eggID, to)
		fmt.Fprintf(stderr, "error.kind=usage · detail=yes_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}

	// ── 真跑：本机 sha256 ⇒ rsync 传输 ⇒ 对端回读 ⇒ 双侧对拍 ──
	localSha, ok := mputSha256File(src)
	if !ok {
		inv.setErr("blocked", "source_unreadable", "本机源件读不到")
		fmt.Fprintf(stderr, "%s: 本机源件读不到 %s ⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName, src)
		return exitBlocked
	}
	rcx, terr := mputRsync(machine, src, to, stdout, stderr)
	if terr != nil {
		inv.setErr("blocked", "rsync_absent", terr.Error())
		fmt.Fprintf(stderr, "%s: 起不动 `%s`：%v ⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName, mputRsyncBin, terr)
		return exitBlocked
	}
	if rcx != 0 {
		inv.setErr("blocked", "transfer_failed", fmt.Sprintf("rsync rc=%d", rcx))
		fmt.Fprintf(stderr, "%s: 传输没跑成（rsync rc=%d）⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName, rcx)
		return exitBlocked
	}
	var rb bytes.Buffer
	rcS, serr := dplSSH(machine, mputRemoteShaCmd(to), &rb, stderr)
	if serr != nil || rcS != 0 {
		inv.setErr("blocked", "readback_failed", fmt.Sprintf("ssh rc=%d", rcS))
		fmt.Fprintf(stderr, "%s: 对端回读跑不起来（ssh rc=%d）⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName, rcS)
		return exitBlocked
	}
	remoteSha := mputFirstToken(rb.String())
	if remoteSha == "" {
		inv.setErr("blocked", "readback_empty", "对端回读没有 sha256")
		fmt.Fprintf(stderr, "%s: 对端回读没给出 sha256 ⇒ **不给结论**（退码 8 · ✗ 别当 0）\n", progName)
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
		"from":          src,
		"to":            machine + ":" + to,
		"local_sha256":  localSha,
		"remote_sha256": remoteSha,
		"verdict":       verdict,
		"note":          "发前本机算 ⟷ 对端回读（dplSSH `sha256sum`）双侧对拍",
	}}
	if inv.jsonGiven {
		if e := emitSelected(stdout, stderr, inv, inv.path, inv.fields, rows); e != exitOK {
			return e
		}
		return rc
	}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		line := make([]string, 0, len(mputDisplay))
		for _, f := range mputDisplay {
			line = append(line, r[f])
		}
		table = append(table, line)
	}
	renderRows(stdout, stderr, inv.tty && !inv.plain, mputDisplay, table)
	if rc == exitFail {
		fmt.Fprintf(stderr, "%s: ✗ 双侧 sha256 不一致 ⇒ **判红**（✗ 不许报成功）—— local=%s · remote=%s\n",
			progName, localSha, remoteSha)
	} else {
		fmt.Fprintf(stderr, "%s: ✅ %s 传输完成 · 双侧 sha256 一致（%s）· 退码 0\n", progName, eggID, localSha)
	}
	return rc
}
