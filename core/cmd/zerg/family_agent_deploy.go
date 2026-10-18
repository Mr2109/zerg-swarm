// family_agent_deploy.go —— `zerg agent deploy <机> --file <本地件>`
// （缺口 `GAP-20261018-20`：子端换件**没有命令面正门** ＋ 换完 `code_sha` 不跟）。
//
// 病根（现读 · 逐字取自当日手工换件的三坑）：
//
//	① `scp` 直覆盖在跑的 `zerg-agentd` ⇒ `ETXTBSY`（dest open Failure）——
//	   正解是**先落 `.new`、再 `mv -f` 换名**（rename 只换 inode，不动在跑的那份映像）；
//	② `systemctl stop edge-node-agent.service` ⇒ `Access denied as the requested operation requires
//	   interactive authentication`（AI 不敲 sudo）—— 正解是**只 restart**（edge-node 的 polkit 免密格
//	   只授 `g01 × edge-node-agent.service × restart`），✗ 本命令一次都不 stop、✗ 一次都不 sudo；
//	③ 换完**身份不跟**：新件已在跑（restart rc=0、新进程在册），但 `zerg agent ls` 读到该机的
//	   `code_sha` 仍是旧值 ⇒ 运维面判不了「远端到底跑哪版」。
//
// 本命令把那条手工通路收成正门，四步一条不省（✗ 不许 scp 直覆盖、✗ 不许 systemctl stop、✗ 不许 sudo）：
//
//	A. 落地：经 `ssh` 把本地件**写进 `<prefix>/zerg-agentd.new`**（stdin 直喂，不经 scp）；
//	B. 对拍：远端 `sha256sum`（macOS 用 `shasum -a 256`）读回来，与本地件 sha256 **逐字相同**才继续；
//	C. 换名：`mv -f <件>.new <件>`（rename 换 inode ⇒ 不受 ETXTBSY 限制）；✗ 不删任何件；
//	D. 重启：复用同族 `agent restart` 的动作面（本机 `launchctl kickstart -k` · 远端经 `ssh` 落
//	   §十五.4 例外清单 `F-5`：edge-node 走 `systemctl restart edge-node-agent.service`）—— **只 restart**。
//
// 身份同步半（另一半在子端）：子端启动后从**自己这一件二进制**读身份（`agent/internal/version`：
// 嵌入 `-tags commit=` > `go version -m` 的 VCS 戳 > ldflags 回落），并在 `/status` 报四格：
// `code_sha` / `code_version` / `code_build_time` / `code_digest`。本命令换完就**现读 `/status` 对拍**：
//
//	· `code_sha`     必须 == 本地件内嵌的 commit（哪一笔提交）；
//	· `code_digest`  必须 == 本地件 sha256 前 16 位（**哪一件字节** —— 脏树两次构建同提交时，
//	  只有这一格能证明「跑的就是我刚换的那件」）。
//
// ⇒ 「身份跟没跟」从「靠人盯 `agent ls`」变成**本命令自己的退码**。核对不上 ⇒ 退 1（✗ 不报绿）。
//
// 退码：0 干跑 / 真换且身份核对通过 · 1 件不合格或身份没跟上 · 2 用法错（缺 `--file`/`--confirm`/`--yes`）·
//
//	8 拿不到结论（名册/落地路径取不到、ssh 打不到、子端身份面判不了）。
package main

import (
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// agentDeployDaemonName —— 落地件名（子端守护进程；`agent/cmd/zerg-agentd` 的产物名）。
	agentDeployDaemonName = "zerg-agentd"
	// agentDeployNewSuffix —— 落地用的**中转名**：先写它、再 `mv -f` 换名。
	// ★ 这是绕 `ETXTBSY` 的那一步（写在跑件的路径上必被内核拒），✗ 不许「scp 直覆盖」。
	agentDeployNewSuffix = ".new"
	// agentDeployAuditRel —— 审计件（<状态目录>/ 之下；语义是「换件」，与 restart 的审计分件）。
	agentDeployAuditRel = "agent_deploy_audit.jsonl"
	// agentDeployMinBytes —— 件的最小体量（1 MiB）。子端守护件现读 ~8 MB；低于此值的
	// 只可能是「给错了件 / 传了一半的件」⇒ 落地前就拒（✗ 拿远端试错）。
	agentDeployMinBytes = 1 << 20
	// agentDeployDigestLen —— 内容身份取 sha256 前多少位（与子端 `version.ReadSelf` 同口径 · 16）。
	agentDeployDigestLen = 16
	// agentDeployIdentityWait —— 换名重启后等子端自报新身份的窗口（子端 HTTP 面起得很快，
	// 但要等新进程落定；超过就如实报「没跟上」，✗ 不无限等）。
	agentDeployIdentityWait = 30 * time.Second
	// agentDeployFleetWait —— 主控面（`agent ls` 的同一份载荷）跟上新身份的最长等待。
	// 子端心跳间隔 5 s ⇒ 给 15 s 足够两跳；这只是**呈现**面，判据以直读子端为准。
	agentDeployFleetWait = 15 * time.Second
)

// agentDeployFields `--json` 闭集（人面列是它的子集 ⇒ 两个面同一份事实）。
var agentDeployFields = []string{
	"machine", "file", "file_sha256", "artifact_commit", "remote_path", "before_code_sha",
	"after_code_sha", "code_digest", "identity", "restarted", "changed", "audit",
}

// agentDeployDisplay 人面列（换件这件事要紧的四格：哪台机 · 换前/换后的代码身份 · 身份判词）。
var agentDeployDisplay = []string{"machine", "before_code_sha", "after_code_sha", "identity", "changed"}

// agentDeployShown —— 身份/形态格缺席时的**点名**。
// ★ ✗ 借用 `orDash`：那句「（未给 —— 位置参数里要写明 id/名）」是 **id 面**的词，
// 放在身份面会把人指到错的地方（本单实测：换前子端自报格打出这句，读起来像少给了参数）。
func agentDeployShown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（子端未报）"
	}
	return s
}

// agentDeployAuditPath 审计落点（测试用 `ZERG_AGENT_DEPLOY_AUDIT` 换源 ⇒ 不碰真状态目录）。
func agentDeployAuditPath() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_AGENT_DEPLOY_AUDIT")); d != "" {
		return d
	}
	return filepath.Join(stateDirOf(), agentDeployAuditRel)
}

// agentDeployArtifact —— 本地待换件（**先读、先验**，一个字节都不发出去之前就能拒）。
type agentDeployArtifact struct {
	Path      string
	Bytes     []byte
	SHA256    string // 整件 sha256（落地后远端/子端都拿它前 16 位对内容）
	Kind      string // elf / macho（与目标机 os 对拍）
	Commit    string // 内嵌 `-tags commit=`（或 VCS 戳）
	BuildTime string // 内嵌 build_time（取不到即空串 = 缺席）
}

// Digest 内容身份（sha256 前 16 位 · 与子端 `code_digest` 同口径）。
func (a agentDeployArtifact) Digest() string {
	if len(a.SHA256) < agentDeployDigestLen {
		return a.SHA256
	}
	return a.SHA256[:agentDeployDigestLen]
}

// agentDeploySelf —— 子端 `/status` 自报的身份四格（缺口 GAP-20261018-20 的子端半）。
type agentDeploySelf struct {
	Reachable bool
	SHA       string
	Digest    string
	BuildTime string
	Version   string
	Why       string // 打不到 / 回非 200 / 没有该键 —— 判不了时点名原因（✗ 不留空）
}

// ── 函数级接缝（测试桩在进程内替换 ⇒ 判据能在「不真 ssh、不真换件」的前提下端到端跑）──
var (
	agentDeployReadArtifactFn = agentDeployReadArtifact
	agentDeployTransferFn     = agentDeployTransferReal
	agentDeployRestartFn      = func(machine string, a agentAddr) error { return agentRestartAction(machine, a) }
	agentDeployProbeFn        = agentDeployProbeChild
	agentDeployFleetSHAFn     = agentDeployFleetCodeSHA
	agentDeploySleep          = time.Sleep
)

// agentDeployReadArtifact 读 + 验本地件（纯本地，零网络）：
//
//	① 存在 · 是普通件 · 体量 ≥ 1 MiB；
//	② 魔数与目标机 os 对拍（linux ⇒ ELF · macos ⇒ Mach-O）—— 拿错平台的件当场拒；
//	③ `debug/buildinfo` 读得出、且 `path` 是**子端守护件**（`…/cmd/zerg-agentd`）；
//	④ 件里带**代码身份**（嵌入 tags 的 `commit=` 优先，其次 VCS 戳）—— 没身份就无法在换完核对，
//	   fail-closed 拒（✗ 换了却判不了）。身份来源写进 `source` 由调用方打出。
func agentDeployReadArtifact(path, targetOS string) (agentDeployArtifact, string, error) {
	art := agentDeployArtifact{Path: path}
	st, err := os.Stat(path)
	if err != nil {
		return art, "", fmt.Errorf("件读不到（%v）", err)
	}
	if !st.Mode().IsRegular() {
		return art, "", fmt.Errorf("不是普通件（mode=%s）", st.Mode())
	}
	if st.Size() < agentDeployMinBytes {
		return art, "", fmt.Errorf("件只有 %d 字节（低于 %d 的下界 ⇒ 只可能是给错了件或传了一半）",
			st.Size(), agentDeployMinBytes)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return art, "", fmt.Errorf("件读不出来（%v）", err)
	}
	art.Bytes = b
	sum := sha256.Sum256(b)
	art.SHA256 = hex.EncodeToString(sum[:])
	art.Kind = agentDeployMagicKind(b)

	want := agentDeployWantKind(targetOS)
	if want != "" && art.Kind != want {
		return art, "", fmt.Errorf("件形态是 %q，而目标机 os=%s 要 %q（拿错平台的件 ⇒ 不落地）",
			art.Kind, targetOS, want)
	}

	bi, err := buildinfo.ReadFile(path)
	if err != nil {
		return art, "", fmt.Errorf("不是 Go 制品（`debug/buildinfo` 读不出：%v）", err)
	}
	if !strings.HasSuffix(bi.Path, "/cmd/"+agentDeployDaemonName) {
		return art, "", fmt.Errorf("不是子端守护件（件内 path=%q，要 `…/cmd/%s`）", bi.Path, agentDeployDaemonName)
	}
	source := ""
	for _, s := range bi.Settings {
		switch s.Key {
		case "-tags":
			kv := agentDeployParseTags(s.Value)
			art.Commit = strings.TrimSpace(kv["commit"])
			art.BuildTime = strings.TrimSpace(kv["build_time"])
			if art.Commit != "" {
				source = "embedded-tags"
			}
		case "vcs.revision":
			if art.Commit == "" {
				art.Commit, source = strings.TrimSpace(s.Value), "buildinfo-vcs"
			}
		case "vcs.modified":
			if source == "buildinfo-vcs" && strings.TrimSpace(s.Value) == "true" {
				art.Commit += "+dirty"
			}
		}
	}
	if strings.TrimSpace(art.Commit) == "" {
		return art, "", fmt.Errorf("件里没有代码身份（既无嵌入 `-tags commit=`、也无 VCS 戳）⇒ 换完判不了身份，拒换")
	}
	return art, source, nil
}

// agentDeployMagicKind 判件形态（只认 elf / macho；其余回空串 = 认不出）。
func agentDeployMagicKind(b []byte) string {
	if len(b) < 4 {
		return ""
	}
	if b[0] == 0x7f && b[1] == 'E' && b[2] == 'L' && b[3] == 'F' {
		return "elf"
	}
	be := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	switch be {
	case 0xfeedface, 0xfeedfacf, 0xcafebabe:
		return "macho"
	}
	// 小端序 Mach-O（0xcffaedfe / 0xcefaedfe 按字节写出来是 fe ed fa cf …）
	switch be {
	case 0xcefaedfe, 0xcffaedfe:
		return "macho"
	}
	return ""
}

// agentDeployWantKind 目标机 os ⇒ 期望件形态（空串 = 该 os 不认识，不判这一条）。
func agentDeployWantKind(targetOS string) string {
	switch strings.ToLower(strings.TrimSpace(targetOS)) {
	case "linux", "ubuntu", "debian", "centos", "fedora", "rhel", "alpine":
		return "elf"
	case "macos", "darwin", "osx":
		return "macho"
	}
	return ""
}

// agentDeployParseTags 拆 `commit=<sha>,build_time=<ts>`（与子端 `version.parseBuildTags` 同口径）。
func agentDeployParseTags(v string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(v, ",") {
		k, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(val)
	}
	return out
}

// agentDeployNodePrefix —— 目标机的**安装前缀**（落地目录的唯一真源）。
//
// 真源 = `gateway/fleet.yaml` 的 `updates.nodes`（换件面那一节；每台 `{ name, ssh, root, prefix, … }`，
// 与 `~/.zerg/contexts/<档>.yaml` 的 `nodes:` 逐字同源、由门 `check-nodes-roster` 对拍）。
// 取不到 ⇒ **不给结论**（退 8）：✗ 不猜一个 `/home/<user>/zerg/bin` 去写。
func agentDeployNodePrefix(machine string, stderr io.Writer) (string, int) {
	root := repoRoot()
	if root == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不到安装前缀（%s）⇒ 不给结论（退码 8）\n", progName, agentFleetAddrFileRel)
		return "", exitBlocked
	}
	b, err := os.ReadFile(filepath.Join(root, agentFleetAddrFileRel))
	if err != nil {
		fmt.Fprintf(stderr, "%s: 名册件读不到（%s：%v）⇒ 落地目录问不出来（退码 8）\n", progName, agentFleetAddrFileRel, err)
		return "", exitBlocked
	}
	inUpdates, inNodes := false, false
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inUpdates = strings.TrimSpace(line) == "updates:"
			inNodes = false
			continue
		}
		if !inUpdates {
			continue
		}
		t := strings.TrimSpace(line)
		if t == "nodes:" {
			inNodes = true
			continue
		}
		if !inNodes || !strings.HasPrefix(t, "-") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(t, "-"))
		if !strings.HasPrefix(body, "{") || yamlInlineField(body, "name") != machine {
			continue
		}
		if p := strings.TrimRight(yamlInlineField(body, "prefix"), "/"); p != "" {
			return p, exitOK
		}
		fmt.Fprintf(stderr, "%s: `updates.nodes` 里 %q 那条没有 `prefix` 字段 ⇒ 落地目录问不出来（退码 8）\n", progName, machine)
		return "", exitBlocked
	}
	fmt.Fprintf(stderr, "%s: `updates.nodes` 里没有 %q 这一条 ⇒ 落地目录问不出来（退码 8 · ✗ 不猜路径）\n", progName, machine)
	return "", exitBlocked
}

// agentDeploySSHRun —— 一次远端命令（唯一通路：`ssh <目标> <命令>`；✗ 不用 scp，✗ 不提权）。
// stdin 非 nil 时把那串字节喂给远端命令（「落 .new」那一步用它，等价于 scp 但不碰在跑件）。
func agentDeploySSHRun(machine string, a agentAddr, remoteCmd string, stdin []byte) (string, error) {
	target := agentRestartSSHTarget(machine, a.Host)
	cmd := armChildGroup(exec.Command("ssh", target, remoteCmd))
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	runningChild = cmd
	defer func() { runningChild = nil }()
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("%v（远端回话：%s）", err, agentSnippet(strings.TrimSpace(out.String()), 300))
	}
	return out.String(), nil
}

// agentDeployRemoteSumCmd 远端取 sha256 的命令（linux `sha256sum` · macOS `shasum -a 256`）。
func agentDeployRemoteSumCmd(targetOS, path string) string {
	if agentDeployWantKind(targetOS) == "macho" {
		return "shasum -a 256 " + agentRestartShQuote(path)
	}
	return "sha256sum " + agentRestartShQuote(path)
}

// agentDeployFirstField 取远端回话的第一栏（`<sha>  <件>` ⇒ `<sha>`）。
func agentDeployFirstField(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if f := strings.Fields(strings.TrimSpace(line)); len(f) > 0 {
			return f[0]
		}
	}
	return ""
}

// agentDeployTransferReal —— 落地 ＋ 对拍 ＋ 换名三步（✗ 不 scp 直覆盖、✗ 不删件、✗ 不提权）。
func agentDeployTransferReal(machine string, a agentAddr, remotePath string, art agentDeployArtifact, stderr io.Writer) error {
	newPath := remotePath + agentDeployNewSuffix
	if agentAddrIsLocal(a) {
		return agentDeployLocalSwap(remotePath, newPath, art, stderr)
	}
	// A. 落地 .new（stdin 直喂 —— 写在 .new 上，不碰在跑的那份映像）
	fmt.Fprintf(stderr, "%s: ① 落地 `%s`（ssh stdin 直喂；✗ scp 直覆盖 ⇒ 不会踩 ETXTBSY）\n", progName, newPath)
	writeCmd := "cat > " + agentRestartShQuote(newPath) + " && chmod 0755 " + agentRestartShQuote(newPath)
	if _, err := agentDeploySSHRun(machine, a, writeCmd, art.Bytes); err != nil {
		return fmt.Errorf("落 .new 失败（旧件一个字节未动）：%v", err)
	}
	// B. 对拍：远端字节必须与本地字节逐字同（不同 ⇒ 不换名）
	fmt.Fprintf(stderr, "%s: ② 远端 sha256 对拍（%s）\n", progName, agentDeployRemoteSumCmd(a.OS, newPath))
	out, err := agentDeploySSHRun(machine, a, agentDeployRemoteSumCmd(a.OS, newPath), nil)
	if err != nil {
		return fmt.Errorf("远端取 sha256 失败（未换名，旧件仍在跑）：%v", err)
	}
	if got := agentDeployFirstField(out); got != art.SHA256 {
		return fmt.Errorf("远端字节与本地件**不一致**（远端 %s · 本地 %s）⇒ 不换名，旧件仍在跑", got, art.SHA256)
	}
	// C. 换名：rename 换 inode ⇒ 在跑件不受影响（这是绕 ETXTBSY 的那一刀）
	fmt.Fprintf(stderr, "%s: ③ `mv -f %s %s`（rename 换 inode）\n", progName, newPath, remotePath)
	if _, err := agentDeploySSHRun(machine, a, "mv -f "+agentRestartShQuote(newPath)+" "+agentRestartShQuote(remotePath), nil); err != nil {
		return fmt.Errorf("换名失败（旧件仍在盘上；`.new` 保留原样，✗ 本命令不删它）：%v", err)
	}
	return nil
}

// agentDeployLocalSwap 本机回环那一支：写 `.new` → 读回对拍 → rename。
func agentDeployLocalSwap(remotePath, newPath string, art agentDeployArtifact, stderr io.Writer) error {
	fmt.Fprintf(stderr, "%s: ① 本机落地 `%s`（写中转名，不碰在跑件）\n", progName, newPath)
	if err := os.WriteFile(newPath, art.Bytes, 0o755); err != nil {
		return fmt.Errorf("写 .new 失败（旧件一个字节未动）：%v", err)
	}
	if err := os.Chmod(newPath, 0o755); err != nil {
		return fmt.Errorf("置可执行位失败（未换名）：%v", err)
	}
	b, err := os.ReadFile(newPath)
	if err != nil {
		return fmt.Errorf("读回 .new 失败（未换名）：%v", err)
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != art.SHA256 {
		return fmt.Errorf("`.new` 字节与本地件**不一致**（%s ≠ %s）⇒ 不换名", got, art.SHA256)
	}
	fmt.Fprintf(stderr, "%s: ② 本机 sha256 对拍通过 · ③ `mv -f %s %s`（rename 换 inode）\n", progName, newPath, remotePath)
	if err := os.Rename(newPath, remotePath); err != nil {
		return fmt.Errorf("换名失败（旧件仍在盘上；`.new` 保留原样，✗ 本命令不删它）：%v", err)
	}
	return nil
}

// agentDeployProbeChild 直读子端 `/status` 的身份四格（换完对拍「跑的是不是这一件」）。
// 打不到 / 回非 200 / 回据不是对象 ⇒ Reachable=false + Why（点名为什么判不了，✗ 不留空）。
func agentDeployProbeChild(a agentAddr) agentDeploySelf {
	status, body, err := agentChildDo(a, http.MethodGet, "/status", "", 5*time.Second)
	if err != nil {
		return agentDeploySelf{Why: fmt.Sprintf("子端 %s/status 打不到（%v）", agentChildBase(a), err)}
	}
	if status != http.StatusOK {
		return agentDeploySelf{Why: fmt.Sprintf("子端 %s/status 回 HTTP %d（不是 200）", agentChildBase(a), status)}
	}
	var st jsonObj
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		return agentDeploySelf{Why: fmt.Sprintf("子端 %s/status 回据不是 JSON 对象（%v）", agentChildBase(a), err)}
	}
	s := agentDeploySelf{
		Reachable: true,
		SHA:       strings.TrimSpace(cell(st["code_sha"])),
		Digest:    strings.TrimSpace(cell(st["code_digest"])),
		BuildTime: strings.TrimSpace(cell(st["code_build_time"])),
		Version:   strings.TrimSpace(cell(st["code_version"])),
	}
	if s.SHA == "" {
		s.Why = "子端 /status 没有 `code_sha` 格（该件不带身份自报面）⇒ 换完判不了「跑的是哪件」"
	}
	return s
}

// agentDeployFleetCodeSHA 主控面（`agent ls` / `agent show` 的同一份载荷）里该机的 `code_sha`。
// 只用于**呈现**换后状态；判据以直读子端 `/status` 为准（心跳最长 5 s 才到）。取不到 ⇒ 空串。
func agentDeployFleetCodeSHA(machine string) string {
	var snap jsonObj
	if err := newClient().getJSON("/api/fleet/status", &snap); err != nil {
		return ""
	}
	o := asObj(asObj(snap["machines"])[machine])
	if o == nil {
		return ""
	}
	return strings.TrimSpace(cell(o["code_sha"]))
}

// agentDeployVerdict 身份判词（三态，✗ 不塌成一格）：
//
//	verified   —— `code_sha` 逐字同 **且** `code_digest` == 本地件 sha256 前 16 位（跑的就是这一件）；
//	sha_only   —— `code_sha` 同、但子端没报 `code_digest`（该件没带内容身份）⇒ 只能证提交、证不了字节；
//	mismatch   —— 两格任一不同 ⇒ **换件没跟上**（退 1，✗ 不报绿）。
func agentDeployVerdict(self agentDeploySelf, art agentDeployArtifact) (string, string) {
	if !self.Reachable || self.SHA == "" {
		return "unjudgeable", self.Why
	}
	if self.SHA != art.Commit {
		return "mismatch", fmt.Sprintf("子端自报 code_sha=%q ≠ 本地件内嵌 commit=%q", self.SHA, art.Commit)
	}
	if self.Digest == "" {
		return "sha_only", "子端没报 code_digest（只能证提交、证不了字节）"
	}
	if !strings.HasPrefix(strings.ToLower(self.Digest), strings.ToLower(art.Digest())) {
		return "mismatch", fmt.Sprintf("子端自报 code_digest=%q ≠ 本地件 sha256 前 %d 位 %q",
			self.Digest, agentDeployDigestLen, art.Digest())
	}
	return "verified", "code_sha 与 code_digest 两格都对上"
}

// agentDeployWaitIdentity 换名重启后轮询子端身份，直到判词不再是 unjudgeable 或到窗口上限。
func agentDeployWaitIdentity(a agentAddr, art agentDeployArtifact, stderr io.Writer) (agentDeploySelf, string, string) {
	waited := time.Duration(0)
	for {
		self := agentDeployProbeFn(a)
		verdict, why := agentDeployVerdict(self, art)
		if verdict != "unjudgeable" {
			return self, verdict, why
		}
		if waited >= agentDeployIdentityWait {
			return self, verdict, why
		}
		agentDeploySleep(2 * time.Second)
		waited += 2 * time.Second
	}
}

// agentDeployWaitFleet 等主控面跟上新身份（呈现面 · 只报不改判）。
func agentDeployWaitFleet(machine, want string) string {
	waited := time.Duration(0)
	for {
		if got := agentDeployFleetSHAFn(machine); got == want {
			return got
		}
		if waited >= agentDeployFleetWait {
			return agentDeployFleetSHAFn(machine)
		}
		agentDeploySleep(2 * time.Second)
		waited += 2 * time.Second
	}
}

// agentDeployAuditLine 审计一行（换件专用；before/after 两态与身份判词逐字落盘）。
type agentDeployAuditLine struct {
	At             string `json:"at"`
	Event          string `json:"event"`
	By             string `json:"by"`
	Confirm        string `json:"confirm"`
	Machine        string `json:"machine"`
	File           string `json:"file"`
	FileSHA256     string `json:"file_sha256"`
	ArtifactCommit string `json:"artifact_commit"`
	RemotePath     string `json:"remote_path"`
	BeforeCodeSHA  string `json:"before_code_sha"`
	AfterCodeSHA   string `json:"after_code_sha"`
	Identity       string `json:"identity"`
	RC             int    `json:"rc"`
	AuditPath      string `json:"audit_path"`
	Note           string `json:"note,omitempty"`
}

// appendAgentDeployAudit 追加一行（写不进 ⇒ 调用方**不许**动远端）。
func appendAgentDeployAudit(path string, line agentDeployAuditLine) error {
	if path == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(line)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// agentDeployPlan 计划件（`--dry-run` · 零副作用：不发一个请求、不写一个字节、不写审计）。
func agentDeployPlan(w io.Writer, machine string, a agentAddr, remotePath string, art agentDeployArtifact, source string) {
	fmt.Fprintln(w, "计划件（--dry-run · 零副作用 —— 未 ssh、未写任何盘、未重启、未写审计）")
	fmt.Fprintf(w, "  动作     : %s agent deploy %s --file %s\n", progName, machine, art.Path)
	fmt.Fprintf(w, "  危险档   : %s（换件 ＋ 重启子端：在跑的任务会被打断）\n", dangerD3)
	fmt.Fprintf(w, "  目标     : machine=%s host=%s port=%d os=%s\n", machine, a.Host, a.Port, agentDeployShown(a.OS))
	fmt.Fprintf(w, "  落地件   : %s\n", remotePath)
	fmt.Fprintf(w, "  本地件   : %s\n", art.Path)
	fmt.Fprintf(w, "  整件 sha256 : %s\n", art.SHA256)
	fmt.Fprintf(w, "  件内身份 : commit=%s · build_time=%s · 来源=%s\n", art.Commit, agentDeployShown(art.BuildTime), source)
	fmt.Fprintf(w, "  件形态   : %s（与目标机 os 对拍已过）\n", art.Kind)
	fmt.Fprintln(w, "  将执行   : ① ssh stdin 落 `<件>.new`（✗ scp 直覆盖 ⇒ 不踩 ETXTBSY）")
	fmt.Fprintf(w, "             ② 远端 `%s` 对拍（不一致 ⇒ 不换名，旧件仍在跑）\n", agentDeployRemoteSumCmd(a.OS, remotePath+agentDeployNewSuffix))
	fmt.Fprintf(w, "             ③ `mv -f <件>.new <件>`（rename 换 inode）\n")
	if agentAddrIsLocal(a) {
		fmt.Fprintf(w, "             ④ 重启：`launchctl kickstart -k %s`（本机回环支）\n", agentRestartTarget(machine))
	} else {
		fmt.Fprintf(w, "             ④ 重启：`ssh %s '%s'`（§十五.4 例外清单 F-5 · **只 restart**）\n",
			agentRestartSSHTarget(machine, a.Host), agentRestartRemoteShell(machine, a))
	}
	fmt.Fprintln(w, "  不做的   : ✗ scp 直覆盖（ETXTBSY）· ✗ systemctl stop（要交互认证）· ✗ sudo · ✗ 删任何件")
	fmt.Fprintf(w, "  授权判据 : --confirm=<%s> 与 --yes 同时到（`--confirm` 值须与机名逐字相同）\n", machine)
	fmt.Fprintf(w, "  身份判据 : 换完直读子端 /status —— code_sha == %s 且 code_digest == %s（对不上 ⇒ 退 1）\n",
		art.Commit, art.Digest())
	fmt.Fprintln(w, "  可逆性   : 再跑一次本命令换回旧件即可；✗ 本命令不自动留回退件（旧件被 rename 覆盖）")
	fmt.Fprintln(w, "  来源     : 缺口 GAP-20261018-20 · 同族先例 agent restart（D3 · 远端 ssh F-5）")
}

// cmdAgentDeploy —— `zerg agent deploy <机> --file <本地件> [--dry-run | --confirm=<机> --yes]`（D3）。
func cmdAgentDeploy(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		if rc := agentCheckFields(inv, stderr, agentDeployFields); rc != exitOK {
			return rc
		}
	}
	// ── 位置参数（零副作用）──
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent deploy` 要一台机器名（`%s agent ls` 看名册）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一台机）")
		fmt.Fprintf(stderr, "%s: `agent deploy` 只吃**一台**机（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])

	// ── `--file` 必给（缺 ⇒ 退 2，✗ 不猜一个件）──
	filePath := strings.TrimSpace(inv.flagVal("--file"))
	if filePath == "" {
		inv.setErr("usage", "missing_file", "缺 --file <本地件>")
		fmt.Fprintf(stderr, "%s: `agent deploy` 要 `--file <本地件>`（要换上去的那件子端守护件）⇒ 退码 2\n", progName)
		fmt.Fprintf(stderr, "先看计划件：%s agent deploy %s --file <件> --dry-run\n", progName, machine)
		fmt.Fprintf(stderr, "error.kind=usage · detail=missing_file · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}

	a, rc := agentChildAddr(machine, stderr)
	if rc != exitOK {
		return rc
	}
	prefix, rc := agentDeployNodePrefix(machine, stderr)
	if rc != exitOK {
		return rc
	}
	remotePath := prefix + "/" + agentDeployDaemonName

	// ── 件先验（纯本地）：不合格的件在这里就拒，一个字节都不出本机 ──
	art, source, err := agentDeployReadArtifactFn(filePath, a.OS)
	if err != nil {
		inv.setErr("failed", "artifact_rejected", err.Error())
		fmt.Fprintf(stderr, "%s: **拒换** —— 本地件不合格：%v\n", progName, err)
		fmt.Fprintf(stderr, "（旧件一个字节未动、未 ssh、未重启 —— 这是**件**的问题，不是命中的问题）\n")
		fmt.Fprintf(stderr, "error.kind=failed · detail=artifact_rejected · retryable=false · remedy=fix_input\n")
		return exitFail
	}

	// ── 两道确认档自相矛盾 ⇒ 用法错（唯一共享判定 · 在任何盘面动作之前）──
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	// ① 干跑：计划件 · 零副作用。
	if inv.dryRun {
		agentDeployPlan(stdout, machine, a, remotePath, art, source)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未 ssh、未写盘、未重启、未写审计）")
		return exitOK
	}
	// ② D3 确认档（fail-closed：缺就拒 · 从不提问）。
	if !inv.confirmGiven {
		msg := "`agent deploy` 是 D3 档（换件 ＋ 重启子端）——**缺确认 ⇒ 不执行**"
		inv.setErr("usage", "confirm_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=%s --yes\n", machine)
		fmt.Fprintf(stderr, "先看计划件：%s agent deploy %s --file %s --dry-run\n", progName, machine, filePath)
		fmt.Fprintf(stderr, "error.kind=usage · detail=confirm_required · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	if inv.confirm != machine {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，目标是 %q）⇒ 不执行", inv.confirm, machine)
		inv.setErr("usage", "confirm_mismatch", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitUsage
	}
	if !inv.yes {
		msg := "`agent deploy` 是 D3 档 —— **缺 --yes ⇒ 不执行**"
		inv.setErr("usage", "yes_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "确认档齐了才真跑：%s agent deploy %s --file %s --confirm=%s --yes\n", progName, machine, filePath, machine)
		return exitUsage
	}

	// ── 换前两态（只读）：主控面 ＋ 直读子端 ──
	beforeFleet := agentDeployFleetSHAFn(machine)
	beforeSelf := agentDeployProbeFn(a)
	fmt.Fprintf(stderr, "%s: 换前 —— 主控面 code_sha=%s · 子端自报 code_sha=%s · code_digest=%s\n",
		progName, agentDeployShown(beforeFleet), agentDeployShown(beforeSelf.SHA), agentDeployShown(beforeSelf.Digest))

	// ── 审计先落盘（写不进 ⇒ 不许动远端）──
	auditPath := agentDeployAuditPath()
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = strings.TrimSpace(os.Getenv("ZERG_BY"))
	}
	if by == "" {
		by = "（未声明）"
	}
	base := agentDeployAuditLine{
		By: by, Confirm: inv.confirm, Machine: machine, File: filePath, FileSHA256: art.SHA256,
		ArtifactCommit: art.Commit, RemotePath: remotePath, BeforeCodeSHA: beforeSelf.SHA, AuditPath: auditPath,
	}
	pre := base
	pre.At, pre.Event, pre.RC, pre.Note = time.Now().Format(time.RFC3339), "executing", 0, "换件前一行（远端还没动）"
	if err := appendAgentDeployAudit(auditPath, pre); err != nil {
		msg := fmt.Sprintf("审计落不下（%s）：%v ⇒ **未动远端**（不给结论 · 退码 8）", auditPath, err)
		inv.setErr("blocked", "audit_unwritable", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitBlocked
	}

	// ── 换件三步（落地 .new → 对拍 → rename）──
	if err := agentDeployTransferFn(machine, a, remotePath, art, stderr); err != nil {
		line := base
		line.At, line.Event, line.RC, line.Note = time.Now().Format(time.RFC3339), "failed", exitFail, err.Error()
		if aerr := appendAgentDeployAudit(auditPath, line); aerr != nil {
			fmt.Fprintf(stderr, "%s: ⚠ 失败那一行审计落不下：%v（判词照实打，不改判决）\n", progName, aerr)
		}
		inv.setErr("failed", "swap_failed", err.Error())
		fmt.Fprintf(stderr, "%s: **换件失败 ⇒ 退码 1**：%v\n", progName, err)
		return exitFail
	}

	// ── 重启（只 restart · ✗ stop · ✗ sudo）──
	restarted := "false"
	if agentAddrIsLocal(a) {
		fmt.Fprintf(stderr, "%s: ④ 重启 `launchctl kickstart -k %s`（本机回环支）\n", progName, agentRestartTarget(machine))
	} else {
		fmt.Fprintf(stderr, "%s: ④ 重启 `ssh %s '%s'`（F-5 · 只 restart）\n",
			progName, agentRestartSSHTarget(machine, a.Host), agentRestartRemoteShell(machine, a))
	}
	restartErr := agentDeployRestartFn(machine, a)
	if restartErr == nil {
		restarted = "true"
	} else {
		fmt.Fprintf(stderr, "%s: ⚠ 重启动作报错：%v（身份面继续核 —— 没重启的话新件不会在跑）\n", progName, restartErr)
	}

	// ── 换后身份核对（判据以直读子端为准）──
	self, verdict, why := agentDeployWaitIdentity(a, art, stderr)
	afterFleet := ""
	if verdict == "verified" || verdict == "sha_only" {
		afterFleet = agentDeployWaitFleet(machine, art.Commit)
	}
	row := map[string]string{
		"machine": machine, "file": filePath, "file_sha256": art.SHA256,
		"artifact_commit": art.Commit, "remote_path": remotePath,
		"before_code_sha": beforeSelf.SHA, "after_code_sha": self.SHA,
		"code_digest": self.Digest, "identity": verdict, "restarted": restarted,
		"changed": map[bool]string{true: "true", false: "false"}[verdict == "verified" || verdict == "sha_only"],
		"audit":   auditPath,
	}
	finalRC := exitOK
	switch verdict {
	case "verified":
		fmt.Fprintf(stderr, "%s: ✓ 身份跟上 —— %s（code_sha=%s · code_digest=%s）\n", progName, why, self.SHA, self.Digest)
	case "sha_only":
		fmt.Fprintf(stderr, "%s: ✓ 提交对上、字节判不了 —— %s（code_sha=%s）\n", progName, why, self.SHA)
		fmt.Fprintf(stderr, "%s: 口径：该件不带 `code_digest` ⇒ 只能证「哪笔提交」，证不了「哪一件字节」\n", progName)
	case "mismatch":
		finalRC = exitFail
		inv.setErr("failed", "identity_not_following", why)
		fmt.Fprintf(stderr, "%s: **身份没跟上 ⇒ 退码 1**：%s\n", progName, why)
	default:
		finalRC = exitBlocked
		inv.setErr("blocked", "identity_unjudgeable", why)
		fmt.Fprintf(stderr, "%s: **身份判不了 ⇒ 退码 8**（拿不到就写拿不到 · ✗ 不假绿）：%s\n", progName, why)
	}
	if afterFleet != "" {
		fmt.Fprintf(stderr, "%s: 主控面（`agent ls` / `agent show` 同一份载荷）code_sha=%s\n", progName, afterFleet)
		if afterFleet != art.Commit {
			fmt.Fprintf(stderr, "%s: ⚠ 主控面还没跟上（心跳 5 s 一跳；判据以直读子端为准）\n", progName)
		}
	}
	if restartErr != nil && finalRC == exitOK {
		fmt.Fprintf(stderr, "%s: ⚠ 重启动作报过错（见上）—— 但身份面已核对通过\n", progName)
	}

	line := base
	line.At, line.Event, line.RC = time.Now().Format(time.RFC3339), verdict, finalRC
	line.AfterCodeSHA, line.Identity, line.Note = self.SHA, verdict, why
	if aerr := appendAgentDeployAudit(auditPath, line); aerr != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 收尾那一行审计落不下：%v（判词照实打，不改判决）\n", progName, aerr)
	}
	// ★ 判词已在上面定案（`finalRC`）；`listCmd` 的投射退码只覆盖「字段投影/写出」那一层 ——
	// 身份没跟上/判不了时**不许**被它冲成 0（那正是本缺口要防的假绿）。
	if rc := listCmd(inv, stdout, stderr, agentDeployDisplay, []map[string]string{agentFillRow(row, agentDeployFields)}); rc != exitOK {
		return rc
	}
	return finalRC
}
