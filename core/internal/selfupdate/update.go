package selfupdate

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// 退出码 —— **唯一真源在 `core/internal/contract/selfupdate.json`**（§7.1 `P12` · §十七 `SD12` ·
// 开工单 T-52）。本包的常量是它的**投影**，由 `ExitCodesMatchTruthSource()` 逐格现跑对拍；
// 不相等 ⇒ 报错（**不许**两处各写一份然后漂 —— 漂过的那一格就是「同数字两义」）。
//
// ★ 本票修的一处真病（照实记）：收敛前这里把**用法错**写成 `4`，而内核脚本
//
//	`scripts/build/zerg-upgrade.sh` 的 `4` 是**校验不通过（未动文件）** —— 一个数字两个意思。
//	收敛后 `4` 只有一个意思（校验不通过），用法错照 sysexits 回到 `64`（脚本 :125 逐字 `exit 64`）。
const (
	ExitOK             = 0
	ExitFail           = 1
	ExitNoUpdate       = 2 // 已是最新 / 开发态领先——无需更新
	ExitRefused        = 3 // 安装方式拒绝（fleet/unknown；非 git 检出）· 有在途任务
	ExitVerifyFailed   = 4 // 校验不通过（**未动文件**）
	ExitRollbackFailed = 5 // **回滚也失败**（需人工介入 · error.kind=ROLLBACK_FAILED）
	ExitUsage          = 64
)

// ExitCodesMatchTruthSource —— 逐格对拍真源（数字 + 语义名），不一致 ⇒ 报错。
//
// 为什么要它而不是「顺手核一眼」：两处各写一份时，漂是**默认结果**、不漂是巧合。
// 判据落成函数 ⇒ `go test ./...` 每次都会问一遍。
func ExitCodesMatchTruthSource() error {
	return exitCodesMatchTruthSource(map[string]int{
		"ok": ExitOK, "fail": ExitFail, "no_update": ExitNoUpdate, "refused": ExitRefused,
		"verify_failed": ExitVerifyFailed, "rollback_failed": ExitRollbackFailed, "usage": ExitUsage,
	})
}

// exitCodesMatchTruthSource —— 判定口本体（**抽出来是为了让负控能喂错的表进来**；
// 直接读包常量的判定口没法做负控，那样它只能是「恒绿装置」）。
func exitCodesMatchTruthSource(mine map[string]int) error {
	spec, err := contract.SelfUpdate()
	if err != nil {
		return err
	}
	if len(spec.ExitCodes) != len(mine) {
		return fmt.Errorf("selfupdate: 真源 %d 格 ≠ 本包 %d 格（两处又开始各写一份了）",
			len(spec.ExitCodes), len(mine))
	}
	for _, e := range spec.ExitCodes {
		v, ok := mine[e.Name]
		if !ok {
			return fmt.Errorf("selfupdate: 真源里的 %q 在本包没有对应常量 —— 缺格也是漂", e.Name)
		}
		if v != e.Code {
			return fmt.Errorf("selfupdate: %s 真源 %d ≠ 本包 %d（同数字两义的病根）", e.Name, e.Code, v)
		}
	}
	return nil
}

// Options —— `zerg update` 的全部入参（含测试用的可注入接缝）。
type Options struct {
	CheckOnly  bool
	To         string
	Force      bool
	JSON       bool
	NoUI       bool
	UseCache   bool
	Role       string   // controller（默认）| node —— 决定要构建/换装哪些件与管哪些服务
	Components []string // 显式组件覆盖（空 ⇒ 由 role 推默认集）

	// ── 可注入接缝（默认从环境/工作区推导；沙箱测试用 ZERG_* 覆盖）──
	RepoDir   string // 本地检出
	Remote    string // fetch 的远端（名或 URL）
	Ref       string // fetch 的 ref（分支）
	Prefix    string // 安装前缀（bin/）
	Kernel    string // 六阶段内核脚本
	StateDir  string // ~/.zerg/state
	Receipts  string // ~/.zerg/update_receipts
	FleetYAML string // 配置（updates.check）

	// BuildTime —— 本次构建的时间戳（空 ⇒ 现在）。机群 `--fleet` 会给三台同一个戳，
	// 否则「同 commit + 同平台 ⇒ 同 sha256」在跨机比对时永远不成立（G6）。
	BuildTime string

	Stdout io.Writer
	Stderr io.Writer
}

// 角色（B5）：controller = 主控机（core + UI）；node = 机群节点（core + agentd，无 UI）。
const (
	RoleController = "controller"
	RoleNode       = "node"
)

// componentsForRole —— role → 组件集；显式 Components 优先。
func componentsForRole(role string, explicit []string) ([]string, error) {
	if len(explicit) > 0 {
		return NormalizeComponents(explicit)
	}
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", RoleController:
		return NormalizeComponents(DefaultComponents())
	case RoleNode:
		return NormalizeComponents(NodeComponents())
	default:
		return nil, fmt.Errorf("未知角色 %q（可选：controller/node）", role)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// DefaultOptions —— 从环境变量/工作区推导默认值。
// 环境覆盖（沙箱/异机）：ZERG_UPDATE_REPO · ZERG_UPDATE_REMOTE · ZERG_UPDATE_REF ·
// ZERG_PREFIX · ZERG_UPGRADE_SCRIPT · ZERG_STATE_DIR · ZERG_RECEIPTS_DIR。
func DefaultOptions() Options {
	root := statepath.WorkspaceRoot()
	if w := strings.TrimSpace(os.Getenv("ZERG_WORKSPACE")); w != "" {
		root = w
	}
	home, _ := os.UserHomeDir()
	o := Options{
		RepoDir:   firstNonEmpty(os.Getenv("ZERG_UPDATE_REPO"), root),
		Remote:    ResolveUpdateRemote(root), // 环境变量 > 已配置 origin > 内置公开仓 URL
		Ref:       firstNonEmpty(os.Getenv("ZERG_UPDATE_REF"), "main"),
		Prefix:    firstNonEmpty(os.Getenv("ZERG_PREFIX"), filepath.Join(root, "bin")),
		Kernel:    firstNonEmpty(os.Getenv("ZERG_UPGRADE_SCRIPT"), filepath.Join(root, "scripts", "build", "zerg-upgrade.sh")),
		StateDir:  firstNonEmpty(os.Getenv("ZERG_STATE_DIR"), statepath.Dir()),
		Receipts:  firstNonEmpty(os.Getenv("ZERG_RECEIPTS_DIR"), filepath.Join(home, ".zerg", "update_receipts")),
		FleetYAML: firstNonEmpty(os.Getenv("ZERG_FLEET_YAML"), filepath.Join(root, "gateway", "fleet.yaml")),
		Role:      firstNonEmpty(os.Getenv("ZERG_UPDATE_ROLE"), RoleController),
		BuildTime: strings.TrimSpace(os.Getenv("ZERG_BUILD_TIME")),
		UseCache:  true,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	}
	if v := strings.TrimSpace(os.Getenv("ZERG_UPDATE_COMPONENTS")); v != "" {
		o.Components = []string{v}
	}
	return o
}

func (o *Options) out() io.Writer {
	if o.Stdout == nil {
		return io.Discard
	}
	return o.Stdout
}

func (o *Options) errw() io.Writer {
	if o.Stderr == nil {
		return io.Discard
	}
	return o.Stderr
}

func (o *Options) say(format string, a ...interface{})  { fmt.Fprintf(o.out(), format+"\n", a...) }
func (o *Options) warn(format string, a ...interface{}) { fmt.Fprintf(o.errw(), format+"\n", a...) }

// Usage —— `zerg update` 帮助。
const Usage = `用法：
  zerg update [--check] [--to <ref>] [--force] [--json] [--no-ui] [--role controller|node] [--components <列表>]

源码式自更新（对齐 Hermes 的 hermes update）：
  解析安装方式 → scoped fetch 公开仓 → 比较目标 commit → 本机构建（按角色选组件）
  → 交既有六阶段内核换装 → verify（运行进程自报 sha）→ 回执

选项：
  --check      只 fetch + 比较 + 打印（不构建、不换装）；6 小时缓存
  --plan       --check 的别名（对齐 zerg-upgrade.sh --plan 的手感）
  --to <ref>   指定目标 ref/commit（显式降级需 --force，回执标注）
  --force      越过安装方式拒绝 / 开发态分叉
  --no-ui      跳过 UI 构建（快的自检）
  --role       构建/换装的角色（B5 机群）：
                 controller（默认）= core + agent（+ darwin 上的 ui）
                 node             = core + agentd（机群节点：X3 上真正在跑的是 zerg-agentd；无 ui）
  --components 显式组件覆盖（core,agent,agentd,ui；ui 仅 darwin）——优先于 --role
  --json       机器可读输出
  --no-cache   本次不走 6 小时缓存（强制 live 检查）

退出码（唯一真源 = core/internal/contract/selfupdate.json）：0 成功 / 1 失败（无回滚·已回滚成功两支） / 2 无需更新 / 3 拒绝（安装方式·有在途任务） / 4 校验不通过（未动文件） / 5 回滚也失败（需人工） / 64 用法错。
`

// CLIMain —— `zerg update` 的入口（core CLI 的子命令）。
func CLIMain(args []string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		check      = fs.Bool("check", false, "只检查")
		plan       = fs.Bool("plan", false, "只检查（别名）")
		to         = fs.String("to", "", "目标 ref/commit")
		force      = fs.Bool("force", false, "越过拒绝")
		jsonOut    = fs.Bool("json", false, "机器可读")
		noUI       = fs.Bool("no-ui", false, "跳过 UI 构建")
		noCache    = fs.Bool("no-cache", false, "不走缓存")
		role       = fs.String("role", "", "角色：controller|node")
		components = fs.String("components", "", "组件列表（core,agent,agentd,ui）")
		help       = fs.Bool("help", false, "帮助")
	)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "❌ 参数错误：", err)
		fmt.Fprint(os.Stderr, Usage)
		return ExitUsage
	}
	if *help {
		fmt.Print(Usage)
		return ExitOK
	}
	o := DefaultOptions()
	o.CheckOnly = *check || *plan
	o.To = *to
	o.Force = *force
	o.JSON = *jsonOut
	o.NoUI = *noUI
	if strings.TrimSpace(*role) != "" {
		o.Role = *role
	}
	if strings.TrimSpace(*components) != "" {
		o.Components = []string{*components}
	}
	if *noCache {
		o.UseCache = false
	}
	return Run(o)
}

// Run —— `zerg update` 主流程。
func Run(o Options) int {
	// ① 解析安装方式（代码树旁 .install_method 印记，权威）
	method := DetectInstallMethod(o.RepoDir)
	if method == MethodFleet && !o.Force {
		o.warn("⛔ 安装方式 = fleet（机群下发）：本机被子端/主控编排，自身不 fetch 源码。")
		o.warn("   要强制本机自更新请加 --force（回执会标注被强制）。")
		return ExitRefused
	}
	if method == MethodUnknown && !o.Force {
		o.warn("⛔ 安装方式 = unknown（不是 git 检出）：源码式自更新不适用。")
		o.warn("   请用发行渠道更新，或 --force 越过（回执会标注被强制）。")
		return ExitRefused
	}

	// 更新检查开关（updates.check:false 可关）
	if !updatesCheckEnabled(o.FleetYAML) {
		o.say("ℹ️  updates.check = false —— 更新检查已关闭（配置 %s）", o.FleetYAML)
		return ExitOK
	}

	g := Git{Dir: o.RepoDir}
	if !isGitCheckout(o.RepoDir) && !o.Force {
		o.warn("⛔ %s 不是 git 检出——无法 fetch/构建。", o.RepoDir)
		return ExitRefused
	}

	// ── --check：只 fetch + 比较 + 打印（不构建、不换装）─────────────────────
	if o.CheckOnly {
		res := Check(g, o.Remote, o.targetRef(o.Ref), o.StateDir, version.Version, o.UseCache)
		o.printCheck(res)
		if res.Status == StatusInconclusive {
			return ExitFail
		}
		return ExitOK
	}

	// ── 完整更新：fetch → 比较 → 构建 → 交独立进程 ─────────────────────────
	// 完整更新一律 live 比较（不吃缓存）：否则会拿 6 小时前的结论去做换装决策。
	ref := o.targetRef(o.Ref)
	local, err := g.HeadSHA()
	if err != nil {
		o.warn("❌ 读不到本地 HEAD：%v", err)
		return ExitFail
	}
	remoteSHA, err := g.FetchTarget(o.Remote, ref)
	if err != nil {
		o.warn("❌ fetch 失败（离线/非法 ref）：%v", err)
		o.warn("   未构建、未换装——本机现状毫发无损。")
		return ExitFail
	}
	cmp := Compare(local, remoteSHA, g.IsAncestor, 0)
	cmp.Dirty = g.IsDirty()
	if cmp.Status == StatusBehind {
		if n := g.CountAhead(local, remoteSHA); n >= 0 {
			cmp.Behind = n
		}
	} else if cmp.Status == StatusLocalAhead {
		if n := g.CountAhead(remoteSHA, local); n >= 0 {
			cmp.Ahead = n
		}
	}
	cmp.Message = messageFor(cmp, version.Version)

	switch cmp.Status {
	case StatusUpToDate:
		o.say("✅ 无需更新：%s", cmp.Message)
		return ExitNoUpdate
	case StatusLocalAhead:
		o.say("🧪 %s", cmp.Message)
		if !o.Force {
			return ExitNoUpdate
		}
		o.say("   （--force：仍按远端 %s 构建换装）", short(remoteSHA))
	case StatusInconclusive:
		o.warn("❌ %s", cmp.Message)
		return ExitFail
	case StatusDiverged:
		if !o.Force && strings.TrimSpace(o.To) == "" {
			o.warn("⛔ %s", cmp.Message)
			return ExitRefused
		}
	}

	o.say("🏷  目标：%s（%s）", ref, short(remoteSHA))

	// ② 本机构建（git 树构建产物 → 临时区）
	components, err := componentsForRole(o.Role, o.Components)
	if err != nil {
		o.warn("❌ %v", err)
		return ExitUsage
	}
	if o.NoUI {
		components = dropUI(components)
	}
	staging, err := os.MkdirTemp("", "zerg-update-staging-")
	if err != nil {
		o.warn("❌ 建临时区失败：%v", err)
		return ExitFail
	}
	src, cleanup, err := o.prepareSource(g, remoteSHA, staging)
	if err != nil {
		o.warn("❌ 准备构建源失败：%v", err)
		os.RemoveAll(staging)
		return ExitFail
	}
	defer cleanup()

	// G6：工具链低于钉住值 ⇒ 明确拒绝（在构建之前，什么也不动）
	if pin, actual, terr := VerifyToolchain(src); terr != nil {
		o.warn("❌ 工具链核对不过（钉住 %s / 本机 %s）：%v", pin, actual, terr)
		os.RemoveAll(staging)
		return ExitFail
	}

	o.say("🔨 本机构建（角色 %s · 组件 %s · 平台 %s）…", roleName(o.Role), strings.Join(components, ","), Platform())
	br, err := Build(BuildOptions{Source: src, Staging: staging, Commit: remoteSHA,
		Components: components, NoUI: o.NoUI, BuildTime: o.BuildTime, LogWriter: o.out()})
	if err != nil {
		o.warn("❌ 构建失败：%v", err)
		o.warn("   未换装——本机现状毫发无损。")
		os.RemoveAll(staging)
		return ExitFail
	}

	// ③ 交**独立进程**（G2：zerg update 自己要换的正是主控二进制 ⇒ 绝不进程内自换）
	pid, klog, err := o.spawnKernel(staging, components)
	if err != nil {
		o.warn("❌ 拉起独立升级进程失败：%v", err)
		os.RemoveAll(staging)
		return ExitFail
	}

	// ④ 交接回执（权威回执由内核写同目录）
	rc, _ := WriteLaunchReceipt(o.Receipts, LaunchReceipt{
		Method: method, Prefix: o.Prefix, Repo: o.RepoDir, From: local,
		Role: roleName(o.Role), Components: components, Toolchain: br.Toolchain,
		BuildTime: br.BuildTime,
		Target:    TargetInfo{Tag: br.Tag, Commit: br.Commit},
		Staging:   staging, Artifacts: br.Artifacts,
		Kernel: o.Kernel, KernelPID: pid, KernelLog: klog, ReceiptDir: o.Receipts,
	})

	if o.JSON {
		fmt.Fprintf(o.out(), "{\"status\":\"dispatched\",\"kernel_pid\":%d,\"target_commit\":%q,\"components\":%q,\"staging\":%q,\"launch_receipt\":%q}\n",
			pid, br.Commit, strings.Join(components, ","), staging, rc)
	} else {
		o.say("🚀 已交给独立升级进程（pid %d）；换装/重启/verify 由它完成（zerg update 自身不换自己）", pid)
		o.say("   内核日志：%s", klog)
		if rc != "" {
			o.say("   交接回执：%s", rc)
		}
		o.say("   ⏱  主控将重启（约 6 秒）+ 其管辖组件（UI 若不在管辖内会明确提示）")
	}
	return ExitOK
}

// targetRef —— --to 优先，其次配置的 ref。
func (o Options) targetRef(def string) string {
	if strings.TrimSpace(o.To) != "" {
		return strings.TrimSpace(o.To)
	}
	return def
}

func (o *Options) printCheck(res CheckResult) {
	if o.JSON {
		b, _ := json.MarshalIndent(struct {
			Status    string `json:"status"`
			LocalSHA  string `json:"local_sha"`
			RemoteSHA string `json:"remote_sha"`
			Behind    int    `json:"behind"`
			Ahead     int    `json:"ahead"`
			Dirty     bool   `json:"dirty"`
			Source    string `json:"source"`
			Message   string `json:"message"`
			Updatable bool   `json:"updatable"`
		}{res.Status, res.LocalSHA, res.RemoteSHA, res.Behind, res.Ahead, res.Dirty, res.Source, res.Message, res.Updatable()}, "", "  ")
		fmt.Fprintln(o.out(), string(b))
		return
	}
	icon := "•"
	switch {
	case res.Updatable():
		icon = "⬆️ "
	case res.Status == StatusUpToDate:
		icon = "✅"
	case res.Status == StatusLocalAhead:
		icon = "🧪"
	case res.Status == StatusInconclusive:
		icon = "⚠️ "
	}
	o.say("%s %s", icon, res.Message)
	if res.Source == "cache" {
		o.say("   （6 小时缓存；--no-cache 可强制实时检查）")
	}
}

// prepareSource —— 准备"要构建的那棵树"：在临时区落一个 target commit 的 detached 检出
// （**绝不碰主工作树**，对照 G4）。本地已在 target 且干净时直接就地构建（省一次检出）。
func (o *Options) prepareSource(g Git, target, staging string) (src string, cleanup func(), err error) {
	head, _ := g.HeadSHA()
	if head == target && !g.IsDirty() {
		return o.RepoDir, func() {}, nil
	}
	wt := filepath.Join(os.TempDir(), fmt.Sprintf("zerg-update-wt-%d", time.Now().UnixNano()))
	if err := g.WorktreeAddDetached(wt, target); err != nil {
		return "", nil, fmt.Errorf("临时检出目标 commit 失败（%s）：%w", short(target), err)
	}
	return wt, func() { g.WorktreeRemove(wt) }, nil
}

// spawnKernel —— 以**独立进程**拉起六阶段内核（setsid，不 Wait），返回 pid 与日志路径。
// 角色与组件随行下发：内核按同样的组件集换装（清单里缺哪件就在换装前拒绝）。
func (o *Options) spawnKernel(staging string, components []string) (int, string, error) {
	if _, err := os.Stat(o.Kernel); err != nil {
		return 0, "", fmt.Errorf("内核脚本不存在：%s", o.Kernel)
	}
	if err := os.MkdirAll(o.Receipts, 0o755); err != nil {
		return 0, "", err
	}
	klog := filepath.Join(o.Receipts, fmt.Sprintf("kernel-%s.log", time.Now().UTC().Format("20060102T150405Z")))
	lf, err := os.OpenFile(klog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, "", err
	}
	args := []string{o.Kernel, "--role", roleName(o.Role)}
	if len(components) > 0 {
		args = append(args, "--components", strings.Join(components, ","))
	}
	if o.NoUI {
		args = append(args, "--no-ui") // 与本次构建一致：跳过了 UI 构建，内核也别碰 UI
	}
	cmd := exec.Command("bash", args...)
	cmd.Env = append(os.Environ(),
		"ZERG_UPGRADE_SOURCE=file://"+staging, // git 树构建产物（B4 默认取件源）
		"ZERG_PREFIX="+o.Prefix,
		"ZERG_RECEIPTS_DIR="+o.Receipts,
		"ZERG_STATE_DIR="+o.StateDir,
	)
	if strings.TrimSpace(o.BuildTime) != "" {
		cmd.Env = append(cmd.Env, "ZERG_BUILD_TIME="+o.BuildTime)
	}
	cmd.Stdout, cmd.Stderr, cmd.Stdin = lf, lf, nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		lf.Close()
		return 0, klog, err
	}
	pid := cmd.Process.Pid
	// 不 Wait：内核是独立进程，zerg update 立即退出（G2）。
	go func() { _ = cmd.Wait(); lf.Close() }()
	return pid, klog, nil
}

// roleName —— 归一化的角色名（空 ⇒ controller）。
func roleName(role string) string {
	if strings.EqualFold(strings.TrimSpace(role), RoleNode) {
		return RoleNode
	}
	return RoleController
}

// dropUI —— 去掉 ui 组件（--no-ui 时构建与核保必须一致）。
func dropUI(components []string) []string {
	out := []string{}
	for _, c := range components {
		if c != CompUI {
			out = append(out, c)
		}
	}
	return out
}
