// deploy_launch.go —— 发布「意图件 + 回读」的 **CLI 正门**（`zerg deploy launch`）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 4 · A13
//
//	（`deploy/scripts/deploy-launch.sh` · 125 行 · 现属面＝部署面/发布意图件+回读 · 归族 = 新族 `deploy`）。
//
// ★ 2026-10-08 **真搬**（本单）：此前本件是「壳转账」（Go 半只 `exec` 底层 .sh · 命令面不重写逻辑）。
// 现把该脚本的**实际行为**逐段搬进本件 —— 旗标解析 / 必填校验 / `date -u` 时间戳 / `mkdir -p` /
// 写意图件 / `printf '%q '` 引号拼远端命令 / 两处 `ssh` / 回读解析 / 全部文案与退码。
// **✗ 不再 `exec` 那支 .sh**（`deployLaunchScriptRel` 只剩「原件路径」的登记义 —— 对拍与退役面用；
// 本件正文 ✗ 不读它、✗ 不 stat 它、✗ 不 exec 它）。
//
// ★★ 物理不可化项（唯一保留的「薄壳」· 实现里逐处标 `[薄壳·系统工具]`）：**两处 `ssh` 调用**
//
//	（节点侧发布器 & 回读）。`ssh` 本身是 OS 工具 —— 节点侧 OpenSSH 客户端 · 认证 · 网络面，
//	Go 内不重实现 ⇒ 本件以 `os/exec` **直起 `ssh` 程序**（✗ 不经 .sh 中转；✗ 不是「留一支脚本当薄壳」）。
//	除此之外的一切 **全部 Go 化**：原脚本的 `date -u +%Y%m%dT%H%M%SZ` ⇒ `time`；`mkdir -p` ⇒
//	`os.MkdirAll`；`python3 - <<PY` 写意图件 ⇒ `encoding/json`；`printf '%q '` ⇒ 件内 `dplBashQ`
//	（逐字符实测对拍本机 GNU bash 3.2.57 的 `%q` 单参形态）；`python3 -c` 回读摘要 ⇒ 件内
//	`dplReadbackSummary`。⇒ 本件**自足**：底层 .sh 移出仓后，本件读数与在盘时**逐字同**。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `family_isolate_ro.go`，不自己创式样）：
//
//	① **退码逐档照底层脚本**：0 意图件与回读件齐 · 2 用法错 · 3 意图件写失败 · 4 节点发布器非 0 ·
//	   5 回读失败。这五档是**底层封装的自有码空间**（`exitcodes.go` 明写「组件的自有码空间与命令面
//	   这张表不是同一张表 —— 命令面只把它们原样转出」）⇒ 走**件内具名常量**（✗ 裸数字，见门⑪ T3）。
//	② **命令面自己不另取码**：Go 化后不再有「递不到脚本」那一档 ⇒ ✗ 不用 `exitBlocked`(8)。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封（契约在命令面只有一处）。
//
// ★ 声明项（**不可逐字复现**· 非「物理不可化」）：
//
//	(a) 原件里**某个旗标后面没有值**（如 `--host` 作 argv 末枚）时，`shift 2` 在 bash 里不移动位置参数
//	    ⇒ 原脚本**死循环**（实测 180s 不返回）。死循环不可对拍、也不该搬 ⇒ 本件按「值取空、只前进一格」
//	    收口（随后由必填校验判 `2`）。⇒ 该档是**行为差**，只此一处，如实登记。
//	(b) `%q` 面：≥0x80 的**多字节**入参在 bash 3.2 上随字节数不稳（`é` 走原样、`中` 走 `$'…'`）
//	    ⇒ 本件一律按「可打印·非须引号」放原样；**ASCII 入参逐字同**，多字节入参差异列入未覆盖面。
//	(c) `ssh` 程序**起不动**（✗ ExitError）时 bash 退 `127`；本件同取 `127`，但 bash 那句
//	    「command not found」带脚本路径与行号，本件不复刻其 stderr 正文。
//	(d) ★ **原件把实参原样内插进 python 源码**（heredoc `<<PY` 里 `"${FILE}"` 一类）⇒
//	    值含 `"` 或换行时该 python 源**语法坏掉**（原件退 `3` ＋ python SyntaxError，意图件不落）；
//	    值含 `\` 时被 python 解成转义序列（落盘内容与原值**不同**）。本件按「值原样」结构化写盘
//	    ⇒ **这几档是本件与原件的差**（原件那一侧是缺陷：把值当代码插进 python 源）。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// deployLaunchScriptRel —— 发布「意图件 + 回读」的**原件路径**（登记义：对拍 / 退役面用）。
// ★ 真搬后本件**不读它**（✗ stat · ✗ exec）；它只剩「本件是从这支脚本搬来的」这一条可核事实。
const deployLaunchScriptRel = "deploy/scripts/deploy-launch.sh"

// dplSSHBin —— 起 `ssh` 用的程序名（生产 = 系统 `ssh`；测试可注入替身，✗ 不是对外的机器面）。
var dplSSHBin = "ssh"

const (
	dplDefaultAgent = "zerg-deploy-agent.sh"
	dplReadbackLeaf = "latest-deploy.json.readback"
	dplParseFailMsg = "（解析失败：回读件非 JSON）"

	// 底层封装**自有码空间**（✗ 不在命令面真源退码表里 ⇒ ✗ 写裸数字，门⑪ T3）。
	dplRCIntentWrite = 3
	dplRCNodeAgent   = 4
	dplRCReadback    = 5
	dplRCSSHAbsent   = 127
)

// dplHelpText —— `-h|--help` 的正文（原件 `awk 'NR>=2 && NR<=30' "$0"` 的**逐字节**搬入）。
// ★ 原件那一条依赖「脚本自己在盘」；本件内化后，脚本移出仓也逐字同。
const dplHelpText = "# deploy-launch.sh —— 发布「**意图件 + 回读**」薄封装（主控侧发起 → 节点侧发布器）\n#\n# ★ 缺口 GAP-20260927-149（换件面 · P1）：\n#   「本机无留痕」≠「通道没跑过」——层级混同。deploy 通道的落点在 **节点**，不在本机\n#   （写点唯一 = deploy/scripts/zerg-deploy-agent.sh，Linux-only）。此前主控侧没有任何\n#   忠实见证件能一眼判「这次发布到底起没起」⇒ 本封装补上「**动手前写意图件 → ssh 跑节点发布器\n#   → 回读节点 latest-deploy.json**」这条链，落 **独立目录** ~/.zerg/deploy_launch/。\n#\n#   为什么独立目录（不是 ~/.zerg/update_receipts/）：升级器对 update_receipts/ 有 `*.json`\n#   裁剪（同族陷阱，见 GAP-20260927-149 正文 :1103 侧风）⇒ 把主控侧意图件/回读件放进那里会被裁掉。\n#   本封装一律写 ~/.zerg/deploy_launch/；意图件后缀 .json，回读件后缀 .json.readback（**永不被裁**）。\n#\n# 用法：\n#   deploy-launch.sh --host <节点> --file <发布器路径> --expect-sha256 <sha256> \\\n#                    [--agent <节点上发布器路径>] [--unit <单元>] [--target <安装路径>] \\\n#                    [--expect-version <串>] [--probe-url <URL>] [--dry-run]\n#\n# 选项：\n#   --host <ssh 目标>        必填（如 <主机名> 或 <用户>@<主机地址>）\n#   --agent <路径>           节点上发布器路径，默认 zerg-deploy-agent.sh（走节点 PATH）\n#   --launch-dir <目录>      主控侧落点，默认 ~/.zerg/deploy_launch\n#   --dry-run                只写意图件 + 打印将执行的 ssh 命令，**不 ssh、不回读**\n#\n# 结果码：0 意图件与回读件齐（节点发布器 rc 见回读件）· 2 用法错 · 3 意图件写失败\n#        · 4 节点发布器非 0（透传其 rc 见回读件）· 5 回读失败（节点 latest-deploy.json 读不到）\n#\n# ★ 边界（本封装**不是**安全边界）：真正的特权边界在 deploy-agent 的 P1a/P2 与 polkit 规则。\n#   本封装只做「主控侧留痕 + 回读」，不改任何判据。\nset -uo pipefail\n"

// dplIntentArtifact / … —— 意图件 body 的**逐字段定形**（字段序 = 原件 python dict 的序，
// ✗ 不许改序：`encoding/json` 按 struct 字段序输出，正好复刻 `json.dump` 的插入序）。
type dplIntentArtifact struct {
	File           string `json:"file"`
	Sha256Expected string `json:"sha256_expected"`
}

type dplIntentUnit struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

type dplIntentExpectation struct {
	Version  string `json:"version"`
	ProbeURL string `json:"probe_url"`
}

type dplIntentActor struct {
	User string `json:"user"`
	Self string `json:"self"`
}

type dplIntentReadback struct {
	NodeLatest string `json:"node_latest"`
}

type dplIntentDoc struct {
	Schema      int                  `json:"schema"`
	Kind        string               `json:"kind"`
	At          string               `json:"at"`
	Host        string               `json:"host"`
	Agent       string               `json:"agent"`
	Artifact    dplIntentArtifact    `json:"artifact"`
	Unit        dplIntentUnit        `json:"unit"`
	Expectation dplIntentExpectation `json:"expectation"`
	Actor       dplIntentActor       `json:"actor"`
	Readback    dplIntentReadback    `json:"readback"`
	Note        string               `json:"note"`
}

// cmdDeployLaunch —— `zerg deploy launch [--host … --file … --expect-sha256 … | --help] [--dry-run | --yes]`（A13 的 Go 正门）。
//
// 退码（**逐档照原件**）：0 意图件与回读件齐 · 2 用法错 · 3 意图件写失败 · 4 节点发布器非 0 ·
// 5 回读失败。★ 真搬后 ✗ 无「递不到脚本」档（本件自足）。
//
// ★ 写面（2026-10-08）：本件真落意图件 ⇒ 默认档 fail-closed —— 缺 `--yes` ⇒ 退 2 **零写**（见本函数里的 `writeGuardYes` 那一段）。
func cmdDeployLaunch(inv *invocation, stdout, stderr io.Writer) int {
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}

	host, agent, file, expectSHA := "", dplDefaultAgent, "", ""
	unit, target, expVer, probeURL := "", "", "", ""
	unitSet := false
	launchDir := os.Getenv("HOME") + "/.zerg/deploy_launch"
	dryRun := false

	for i := 0; i < len(tail); i++ {
		switch tail[i] {
		case "--host":
			host, i = dplTakeValue(tail, i)
		case "--agent":
			agent, i = dplTakeValue(tail, i)
		case "--file":
			file, i = dplTakeValue(tail, i)
		case "--expect-sha256":
			expectSHA, i = dplTakeValue(tail, i)
		case "--unit":
			unit, i = dplTakeValue(tail, i)
			unitSet = true
		case "--target":
			target, i = dplTakeValue(tail, i)
		case "--expect-version":
			expVer, i = dplTakeValue(tail, i)
		case "--probe-url":
			probeURL, i = dplTakeValue(tail, i)
		case "--launch-dir":
			launchDir, i = dplTakeValue(tail, i)
		case "--dry-run":
			dryRun = true
		case "--yes":
			// ★ 命令面的授权旗标 —— 已被上面的 `writeGuardYes` **消费**（本件原件没有它）。
			//   这里**只吞下、不另立语义**（`[--dry-run | --yes]` 两枚同给已由共享判定退 2）。
		case "-h", "--help":
			fmt.Fprint(stdout, dplHelpText)
			return exitOK
		default:
			fmt.Fprintf(stderr, "✗ 未知参数：%s（用 --help）\n", tail[i])
			return exitUsage
		}
	}

	if host == "" {
		fmt.Fprint(stderr, "✗ 缺 --host\n")
		return exitUsage
	}

	// ★ 默认单元名由现读的 `--host` 派生（`--host` 在解析循环里定值 ⇒ 本单取的最早点）：
	//   取 `@` 前第一段（无 `@` ⇒ 全串）＋ "-agent.service"。`--unit` 显式给出时不派生。
	if !unitSet {
		unit = dplUnitFromHost(host)
	}
	if file == "" {
		fmt.Fprint(stderr, "✗ 缺 --file（节点上发布器路径）\n")
		return exitUsage
	}
	if expectSHA == "" {
		fmt.Fprint(stderr, "✗ 缺 --expect-sha256\n")
		return exitUsage
	}

	// ★ 授权侧（`GAP-20260927-140` ① · 门100 写面登记 · 2026-10-08）：本件真落**意图件**
	//   （`os.MkdirAll` + `dplWriteIntent`）⇒ 是**写面** ⇒ 默认档 fail-closed —— 缺 `--yes` ⇒
	//   退 2、**零写**（✗ 不建目录、✗ 不落意图件、✗ 不起 `ssh`）；`--dry-run` 与 `--yes` 同给 ⇒
	//   共享判定 `dryRunYesConflict` 退 2（两道确认档自相矛盾）。判在**唯一写盘点之前**。
	//   ★ 本件**有自己的 `--dry-run` 支**（原件语义：只写意图件 + 打印将执行的 ssh、不 ssh 不回读）
	//   ⇒ 照 `dev proposal new` 同型只调 `writeGuardYes`：`guard.go` 的 `writeGuardDryRun` 是
	//   「**没有**自己的 `--dry-run` 支」那一族用的（见其函数注释）。✗ 不换 `cmdGuarded`。
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}

	at := time.Now().UTC().Format("20060102T150405Z")
	if err := os.MkdirAll(launchDir, 0o777); err != nil {
		fmt.Fprintf(stderr, "✗ 意图目录建不出来：%s\n", launchDir)
		return exitUsage
	}

	intent := launchDir + "/intent-" + at + "-" + dplHead8(expectSHA) + ".json"
	readback := launchDir + "/" + dplReadbackLeaf
	cwd, _ := os.Getwd()
	doc := dplIntentDoc{
		Schema: 1,
		Kind:   "deploy-launch-intent",
		At:     at,
		Host:   host,
		Agent:  agent,
		Artifact: dplIntentArtifact{
			File:           file,
			Sha256Expected: expectSHA,
		},
		Unit: dplIntentUnit{
			Name:   unit,
			Target: target,
		},
		Expectation: dplIntentExpectation{
			Version:  expVer,
			ProbeURL: probeURL,
		},
		Actor: dplIntentActor{
			User: os.Getenv("USER"),
			Self: filepath.Clean(cwd + "/-"),
		},
		Readback: dplIntentReadback{
			NodeLatest: readback,
		},
		Note: "意图件 = 主控侧留痕；真值以节点 latest-deploy.json 回读件为准（本件不证成功，只证「发起过」）",
	}
	if err := dplWriteIntent(intent, doc); err != nil {
		fmt.Fprintf(stdout, "✗ 意图件写失败：%s\n", intent)
		return dplRCIntentWrite
	}
	fmt.Fprintf(stdout, "🧾 意图件: %s\n", intent)

	remoteArgs := []string{"--file", file, "--expect-sha256", expectSHA, "--unit", unit}
	if target != "" {
		remoteArgs = append(remoteArgs, "--target", target)
	}
	if expVer != "" {
		remoteArgs = append(remoteArgs, "--expect-version", expVer)
	}
	if probeURL != "" {
		remoteArgs = append(remoteArgs, "--probe-url", probeURL)
	}
	remoteCmd := dplBashQJoin(append([]string{agent}, remoteArgs...))

	if dryRun {
		fmt.Fprintf(stdout, "🧪 DRY-RUN：将执行 ssh %s %s（不动手、不回读）\n", host, remoteCmd)
		return exitOK
	}

	// [薄壳·系统工具] 节点侧发布器（ssh · OS 工具 · 网络/认证面 ⇒ Go 内不重实现）。
	fmt.Fprintf(stdout, "▶️  ssh %s :: %s（节点发布器）\n", host, agent)
	rcAgent, _ := dplSSH(host, remoteCmd, stdout, stderr)
	fmt.Fprintf(stdout, "← 节点发布器 rc=%d\n", rcAgent)

	// [薄壳·系统工具] 回读（ssh 取节点 latest-deploy.json）。
	rf, ferr := os.Create(readback)
	if ferr == nil {
		rcRead, _ := dplSSH(host, dplNodeReceiptCmd, rf, stderr)
		rf.Close()
		if st, serr := os.Stat(readback); rcRead == 0 && serr == nil && st.Size() > 0 {
			fmt.Fprintf(stdout, "📥 回读件: %s\n", readback)
			fmt.Fprintf(stdout, "   回读摘要: %s\n", dplReadbackSummary(readback))
		} else {
			fmt.Fprint(stdout, "✗ 回读失败：节点 latest-deploy.json 读不到 ⇒ 通道可能没真跑（或节点未产出回执）\n")
			return dplRCReadback
		}
	} else {
		fmt.Fprint(stdout, "✗ 回读失败：节点 latest-deploy.json 读不到 ⇒ 通道可能没真跑（或节点未产出回执）\n")
		return dplRCReadback
	}

	if rcAgent != 0 {
		return dplRCNodeAgent
	}
	fmt.Fprint(stdout, "✅ 主控侧留痕齐（意图件 + 回读件）\n")
	return exitOK
}

// dplNodeReceiptCmd —— 回读那条远端命令（原件单引号里的**逐字**串：`${HOME}` 由**远端 shell** 展开）。
const dplNodeReceiptCmd = `cat "${HOME}/.zerg/update_receipts/latest-deploy.json" 2>/dev/null`

// dplAuthFlags —— 把「被授权门消费」的两枚旗标（`--dry-run` / `--yes`）落到解析字段上
// （与 `main.go` 的 `parseInvocation` 同形）。给**不经过解析面**的入口用：inproc 包装入口
// `deployLaunchGateMain` 与单测的进程内直调。★ 真实 CLI 路径由 `parseInvocation` 置位，
// 本函数在那边不参与（✗ 不改任何既有解析行为）。
func dplAuthFlags(inv *invocation, tail []string) {
	for _, a := range tail {
		switch a {
		case "--dry-run":
			inv.dryRun = true
		case "--yes":
			inv.yes = true
		}
	}
}

// dplTakeValue 取「<旗标> <值>」的值：有下一枚 ⇒ 收下并**把 i 停在值那一格**（循环的 i++ 再前进一格）；
// 尾上裸给 ⇒ 收空串、i 不动（下一步 i++ 前进一格）。
// ★ 原件此处 `shift 2` 越界时**不移动** ⇒ 死循环（见件头声明项 (a)）—— 本件按「值取空」收口。
func dplTakeValue(tail []string, i int) (string, int) {
	if i+1 < len(tail) {
		return tail[i+1], i + 1
	}
	return "", i
}

// dplUnitFromHost —— 默认单元名由 `--host` 派生：取 `@` 前第一段（无 `@` ⇒ 全串）＋ "-agent.service"。
// ★ `--host <名>` ⇒ `<名>-agent.service`；`--host <用户>@<主机地址>` ⇒ `<用户>-agent.service`（取 `@` 前那一段）。
func dplUnitFromHost(host string) string {
	name := host
	if at := strings.Index(host, "@"); at >= 0 {
		name = host[:at]
	}
	return name + "-agent.service"
}

// dplHead8 —— `${VAR:0:8}`（bash 子串按**字符**切 ⇒ 先 []rune）。
func dplHead8(s string) string {
	r := []rune(s)
	if len(r) > 8 {
		r = r[:8]
	}
	return string(r)
}

// dplWriteIntent 写意图件：字段序定形 + `indent=2` + `ensure_ascii=False` + 末尾一枚换行
// （= 原件 `python3 -c 'json.dump(body,f,ensure_ascii=False,indent=2); f.write("\n")'` 的逐字节等价）。
func dplWriteIntent(path string, doc dplIntentDoc) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// dplSSH 起 `ssh -o BatchMode=yes -o ConnectTimeout=10 <host> <remoteCmd>`（stdin 继承 · stdout/stderr 逐字透传）。
// 返回 ssh 退码（✗ 起不动 ⇒ `dplRCSSHAbsent`(127)，与 bash 的 command-not-found 同档）。
func dplSSH(host, remoteCmd string, outW io.Writer, stderr io.Writer) (int, error) {
	cmd := exec.Command(dplSSHBin, "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", host, remoteCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = outW
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), nil
		}
		return dplRCSSHAbsent, err
	}
	return 0, nil
}

// dplQBashEsc —— GNU bash 3.2 `printf '%q'` 会给它加反斜杠的那批「可打印·须引号」字符（逐字符实测）。
const dplQBashEsc = " !\"$&'()*,;<>?[\\]^`{|}"

// dplBashQ —— 复刻 GNU bash 3.2 `printf '%q'` 的**单参**引号形态（本机逐字符实测口径）：
//   - 空串 ⇒ `”`；- 只含安全字符 ⇒ 原样；- 含须引号的可打印字符 ⇒ 逐字符 `\` 前缀
//     （`#` **只在首字符位**才须引号：`#a`⇒`\#a` 而 `a#`⇒`a#`）；
//   - 含不可打印字节（<0x20 / 0x7f）⇒ 整串换 `$'…'`（`\'` · `\\` · 命名转义 · `\NNN` 八进制）。
//
// ★ 未覆盖面：≥0x80 的多字节入参（bash 3.2 的 isprint 判定随字节数不稳）⇒ 本件按可打印放原样。
func dplBashQ(s string) string {
	if s == "" {
		return "''"
	}
	raw := []byte(s)
	dplEsc := func(i int, b byte) bool {
		if b >= 0x80 {
			return false
		}
		if strings.IndexByte(dplQBashEsc, b) >= 0 {
			return true
		}
		return i == 0 && b == '#'
	}
	needEsc, dollar := false, false
	for i, b := range raw {
		if b < 0x20 || b == 0x7f {
			dollar = true
			continue
		}
		if dplEsc(i, b) {
			needEsc = true
		}
	}
	if dollar {
		var sb strings.Builder
		sb.WriteString("$'")
		for _, b := range raw {
			switch b {
			case '\'':
				sb.WriteString(`\'`)
			case '\\':
				sb.WriteString(`\\`)
			case 7:
				sb.WriteString(`\a`)
			case 8:
				sb.WriteString(`\b`)
			case 9:
				sb.WriteString("\\t")
			case 10:
				sb.WriteString(`\n`)
			case 11:
				sb.WriteString(`\v`)
			case 12:
				sb.WriteString(`\f`)
			case 13:
				sb.WriteString(`\r`)
			case 27:
				sb.WriteString(`\E`)
			default:
				if b < 0x20 || b == 0x7f {
					sb.WriteString(fmt.Sprintf(`\%03o`, b))
				} else {
					sb.WriteByte(b)
				}
			}
		}
		sb.WriteString("'")
		return sb.String()
	}
	if !needEsc {
		return s
	}
	var sb strings.Builder
	for i, b := range raw {
		if dplEsc(i, b) {
			sb.WriteByte('\\')
		}
		sb.WriteByte(b)
	}
	return sb.String()
}

// dplBashQJoin —— 复刻 `printf '%q ' <tok>…`：每枚 `%q` 后跟一个空格（**含末枚** ⇒ 尾带一空格）。
func dplBashQJoin(toks []string) string {
	var sb strings.Builder
	for _, t := range toks {
		sb.WriteString(dplBashQ(t))
		sb.WriteByte(' ')
	}
	return sb.String()
}

// dplReadbackSummary —— 复刻原件那段 `python3 -c` 的回读摘要：`result=%s code=%s unit=%s live_sha=%s`；
// 任何取不到 / 非 JSON / 取法在该件上会抛异常 ⇒ `dplParseFailMsg`（= 原件 `2>/dev/null || echo …`）。
func dplReadbackSummary(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return dplParseFailMsg
	}
	var b map[string]any
	if err := json.Unmarshal(raw, &b); err != nil || b == nil {
		return dplParseFailMsg
	}
	result, ok := dplMapGet(b, "result")
	if !ok {
		return dplParseFailMsg
	}
	code, ok := dplMapGet(b, "result_code")
	if !ok {
		return dplParseFailMsg
	}
	// (b.get("unit") or {}).get("name")：外值是假值 ⇒ 当空 dict（取不到 ⇒ None）；外值是真值但非 dict ⇒ 抛异常。
	unitName, ok := dplGetOrEmptyDict(b, "unit", "name")
	if !ok {
		return dplParseFailMsg
	}
	liveSHA, ok := dplGetOrEmptyDict(b, "observed", "live_exe_sha256")
	if !ok {
		return dplParseFailMsg
	}
	return fmt.Sprintf("result=%s code=%s unit=%s live_sha=%s",
		dplPyStr(result), dplPyStr(code), dplPyStr(unitName), dplPyStr(liveSHA))
}

// dplMapGet = py `m.get(k)`（缺键 ⇒ None）。
func dplMapGet(m map[string]any, k string) (any, bool) {
	return m[k], true
}

// dplGetOrEmptyDict 复刻 `(m.get(k) or {}).get(k2)`：外值缺/假值 ⇒ 当空 dict（⇒ None）；
// 外值真值但非 dict ⇒ 原件会抛 AttributeError ⇒ 本件判 ok=false（⇒ 走「解析失败」文案）。
func dplGetOrEmptyDict(m map[string]any, k, k2 string) (any, bool) {
	v := m[k]
	if !dplPyTruthy(v) {
		return nil, true
	}
	sub, isMap := v.(map[string]any)
	if !isMap {
		return nil, false
	}
	return sub[k2], true
}

// dplPyTruthy —— py 真值性（None/""/0/false/[]/{} ⇒ 假）。
func dplPyTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t != ""
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	}
	return true
}

// dplPyStr —— py `"%s" % v`（≈ `str(v)`）：None ⇒ "None"；bool ⇒ "True"/"False"；
// 数是整值 ⇒ 十进制整数形态（`json.load` 的 int 分支）· 否则最短往返小数；
// dict/list 的 py repr 不在逐字承诺内（回执件里不出现）。
func dplPyStr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return t
	case float64:
		if t == math.Trunc(t) && math.Abs(t) < 1e15 {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

// deployLaunchGateMain —— `cmdDeployLaunch` 的 **inproc 同签名包装入口**（与 `gateCheckGoEntry` 表值
// 同签名：`func(stdout, stderr io.Writer, argv []string) int`）。
//
// ★ **仅转发**：把 argv 原样当作 `zerg deploy launch` 之后的实参，转交 `cmdDeployLaunch`
// （唯一实现仍是 `cmdDeployLaunch` —— 本件 ✗ 不改退码口径、✗ 不新增机器面；两态三面逐字同）。
// ★ 授权输入随实参走：本入口**不经过** `parseInvocation` ⇒ 把被授权门消费的两枚旗标
// （`--dry-run` / `--yes`）落到解析字段上（否则 `writeGuardYes` 恒判「缺 `--yes`」⇒ 判据入口不可用）。
func deployLaunchGateMain(stdout, stderr io.Writer, argv []string) int {
	inv := &invocation{
		path: []string{"deploy", "launch"},
		orig: append([]string{"deploy", "launch"}, argv...),
	}
	dplAuthFlags(inv, argv)
	return cmdDeployLaunch(inv, stdout, stderr)
}
