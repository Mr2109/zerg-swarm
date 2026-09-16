package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// ModulePath —— 版本包路径（-ldflags -X 注入代码身份用）。
const ModulePath = "github.com/Mr2109/zerg-swarm/core/internal/version"

// AgentModulePath —— 子端守护进程自己的版本包（agent 是独立模块，internal 规则禁止跨模块引用）。
const AgentModulePath = "github.com/Mr2109/zerg-swarm/agent/internal/version"

// ── 组件（B5：主控机与机群节点要的件不同）────────────────────────────────────
//
// core   = 主控（也是将来该机自更新的 CLI 入口）
// agent  = 子端一次性 CLI（核心模块里的 cmd/zerg-agent）
// agentd = 子端**常驻守护进程**（agent 模块里的 cmd/zerg-agentd）——X3 上真正在跑的就是它，
//
//	机群版本矩阵里 x3 那一行的 code_sha 来自它的心跳
//
// ui     = Rust 桌面 UI —— **仅 darwin 编译**（设计 §3：X3 上无 Rust 工具链）
const (
	CompCore   = "core"
	CompAgent  = "agent"
	CompAgentd = "agentd"
	CompUI     = "ui"
	// wall = 茧壁（Rust 自研的封闭外壳，§五之二）：**两平台都有**（macOS 一档直调 Seatbelt / Linux 调 bwrap），
	// 且机群节点也必须带（孵卵要用它）——与 ui 的"仅 darwin"不同。
	CompWall = "wall"
)

// DefaultComponents —— 主控机默认：core + agent（darwin 再加 ui）。
func DefaultComponents() []string {
	c := []string{CompCore, CompAgent}
	if runtime.GOOS == "darwin" {
		c = append(c, CompUI)
	}
	// 茧壁两平台都带（§五之三：机器上装的那一份，可升级可审计；卵里另带一份自包含兜底）
	c = append(c, CompWall)
	return c
}

// NodeComponents —— 机群节点（如 X3）：core（装上它，该机将来才能自己 `zerg-core update`）
// + agentd（该机真正在跑的守护进程）。**不含 ui**。
func NodeComponents() []string { return []string{CompCore, CompAgentd, CompWall} }

// NormalizeComponents —— 规范化组件列表（小写、去重、保序）+ 校验。
// 空列表 ⇒ DefaultComponents()；未知组件或 ui 出现在非 darwin ⇒ 报错（**不静默忽略**）。
func NormalizeComponents(list []string) ([]string, error) {
	raw := list
	flat := []string{}
	for _, s := range raw {
		for _, p := range strings.Split(s, ",") {
			if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
				flat = append(flat, p)
			}
		}
	}
	if len(flat) == 0 {
		flat = DefaultComponents()
	}
	seen := map[string]bool{}
	out := []string{}
	for _, c := range flat {
		switch c {
		case CompCore, CompAgent, CompAgentd, CompWall:
		case CompUI:
			if runtime.GOOS != "darwin" {
				return nil, fmt.Errorf("ui 组件仅在 darwin 编译（当前 %s）——设计 §3：UI 只出 Mac", runtime.GOOS)
			}
		default:
			return nil, fmt.Errorf("未知组件 %q（可选：core/agent/agentd/ui/wall）", c)
		}
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out, nil
}

// Platform —— 本机平台标识（与 zerg-upgrade.sh 的 PLAT 同形：darwin-arm64 / linux-amd64）。
func Platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

// ArtifactName —— 组件 → 制品名（内核按同形映射安装：zerg-agentd-linux-amd64 → zerg-agentd）。
func ArtifactName(component, plat string) string { return "zerg-" + component + "-" + plat }

// ToolchainInfo —— 一次构建用的 Go 工具链（G6：pin 与 actual 都要留痕，进 manifest/回执）。
type ToolchainInfo struct {
	Pin    string `json:"pin"`    // 钉住值（core/go.mod 的 toolchain 指令）
	Actual string `json:"actual"` // 本机实际使用
	Policy string `json:"policy"` // GOTOOLCHAIN（默认 local：不让 go 中途换工具链）
}

// Artifact —— 一件构建产物（名/路径/sha256）。
type Artifact struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// BuildResult —— 一次本机构建的结果（staging 目录即"git 树构建产物"，交内核换装）。
type BuildResult struct {
	Staging    string        `json:"-"`
	Version    string        `json:"version"`
	Tag        string        `json:"tag"`
	Commit     string        `json:"commit"`
	BuildTime  string        `json:"build_time"`
	Platform   string        `json:"platform"`
	Components []string      `json:"components"`
	Toolchain  ToolchainInfo `json:"toolchain"`
	Artifacts  []Artifact    `json:"artifacts"`
}

// BuildOptions —— 本机构建入参。
type BuildOptions struct {
	Source     string   // 要构建的源码树（target commit 的临时检出）
	Staging    string   // 产物落点（临时区）
	Commit     string   // 目标 commit（注入身份）
	Components []string // 要构建的组件（空 ⇒ 默认集）
	NoUI       bool     // 跳过 UI（--no-ui）
	BuildTime  string   // 构建时间戳（空 ⇒ 现在；机群用同一戳让各机可比）
	LogWriter  io.Writer
}

var versionRe = regexp.MustCompile(`(?m)^const Version = "([^"]+)"`)

// readVersionFromTree —— 从**目标树**读版本真源（core/internal/version/version.go）。
// 不从当前进程读：目标提交可能刚 bump 版本，用旧版本号会写错 manifest。
func readVersionFromTree(src string) (string, error) {
	p := filepath.Join(src, "core", "internal", "version", "version.go")
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("读目标树的版本真源失败：%w", err)
	}
	m := versionRe.FindSubmatch(b)
	if len(m) < 2 {
		return "", fmt.Errorf("从 %s 解析不到 `const Version`", p)
	}
	return string(m[1]), nil
}

// Build —— 从 source 树本地构建所请求的组件。
//
// 纪律：① UI 仅在 darwin 构建（设计 §3）；② sha256 于**签名之前**算（内核换装时会先校暂存件再 ad-hoc 重签）；
// ③ G6 工具链钉住：本机低于钉住值 ⇒ 直接报错不构建；构建一律 GOTOOLCHAIN=local 并把 pin/actual 写进 manifest；
// ④ 构建时间由调用方给定（机群同一戳 ⇒ 同平台可逐字节比对）。
func Build(opts BuildOptions) (*BuildResult, error) {
	if opts.Source == "" || opts.Staging == "" {
		return nil, fmt.Errorf("Build 需要 source 与 staging")
	}
	logw := opts.LogWriter
	if logw == nil {
		logw = io.Discard
	}
	components, err := NormalizeComponents(opts.Components)
	if err != nil {
		return nil, err
	}
	if opts.NoUI {
		components = dropComponent(components, CompUI)
	}
	pin, actual, err := VerifyToolchain(opts.Source)
	if err != nil {
		return nil, err
	}
	version, err := readVersionFromTree(opts.Source)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.Staging, 0o755); err != nil {
		return nil, err
	}
	buildTime := strings.TrimSpace(opts.BuildTime)
	if buildTime == "" {
		buildTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}
	plat := Platform()
	coreFlags := fmt.Sprintf("-s -w -X %s.Commit=%s -X %s.BuildTime=%s", ModulePath, opts.Commit, ModulePath, buildTime)
	agentdFlags := fmt.Sprintf("-s -w -X %s.Version=%s -X %s.Commit=%s -X %s.BuildTime=%s",
		AgentModulePath, version, AgentModulePath, opts.Commit, AgentModulePath, buildTime)

	res := &BuildResult{
		Staging: opts.Staging, Version: version, Tag: "v" + version,
		Commit: opts.Commit, BuildTime: buildTime, Platform: plat,
		Components: components,
		Toolchain:  ToolchainInfo{Pin: pin, Actual: actual, Policy: gotoolchainPolicy()},
	}
	fmt.Fprintf(logw, "🧰 工具链：钉住 %s · 本机 %s · GOTOOLCHAIN=%s\n", pin, actual, res.Toolchain.Policy)
	fmt.Fprintf(logw, "🧩 组件：%s（平台 %s）\n", strings.Join(components, ","), plat)

	for _, c := range components {
		switch c {
		case CompCore, CompAgent:
			dir := filepath.Join(opts.Source, "core")
			pkg := "./cmd/zerg-" + c
			out := filepath.Join(opts.Staging, ArtifactName(c, plat))
			fmt.Fprintf(logw, "→ 构建 zerg-%s（%s）\n", c, pkg)
			if err := goBuild(dir, out, pkg, coreFlags, logw); err != nil {
				return nil, fmt.Errorf("构建 zerg-%s 失败：%w", c, err)
			}
		case CompAgentd:
			dir := filepath.Join(opts.Source, "agent")
			// 目标树必须真有 agent 模块（独立模块）——缺了就明确报错，不静默跳过
			if st, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil || st.IsDir() {
				return nil, fmt.Errorf("目标树没有 agent 模块（%s/go.mod 缺失）——无法构建 zerg-agentd", dir)
			}
			out := filepath.Join(opts.Staging, ArtifactName(CompAgentd, plat))
			fmt.Fprintf(logw, "→ 构建 zerg-agentd（./cmd/zerg-agentd，agent 模块）\n")
			if err := goBuild(dir, out, "./cmd/zerg-agentd", agentdFlags, logw); err != nil {
				return nil, fmt.Errorf("构建 zerg-agentd 失败：%w", err)
			}
		case CompUI:
			if runtime.GOOS != "darwin" {
				return nil, fmt.Errorf("ui 组件仅在 darwin 编译（当前 %s）", runtime.GOOS)
			}
			out := filepath.Join(opts.Staging, ArtifactName(CompUI, plat))
			fmt.Fprintf(logw, "→ 构建 UI（cargo release）\n")
			if err := cargoBuildUI(filepath.Join(opts.Source, "ui"), out, logw); err != nil {
				return nil, fmt.Errorf("构建 UI 失败：%w", err)
			}
		}
	}

	// 逐件算 sha256（**签名之前**的口径——内核会在换装时对暂存件先校 sha 再 ad-hoc 重签）。
	// 只算本次请求的组件，避免把 staging 里的无关残留写进清单。
	for _, c := range components {
		name := filepath.Join(opts.Staging, ArtifactName(c, plat))
		if _, err := os.Stat(name); err != nil {
			return nil, fmt.Errorf("组件 %s 构建后无产物：%w", c, err)
		}
		a := Artifact{Name: filepath.Base(name)}
		if st, err := os.Stat(name); err == nil {
			a.Size = st.Size()
		}
		if a.SHA256, err = sha256File(name); err != nil {
			return nil, err
		}
		res.Artifacts = append(res.Artifacts, a)
	}
	if len(res.Artifacts) == 0 {
		return nil, fmt.Errorf("构建结束但没有产物——拒绝生成清单（空集不得冒充成功）")
	}

	if err := writeManifest(opts.Staging, res); err != nil {
		return nil, err
	}
	fmt.Fprintf(logw, "✅ 构建完成：%s（%d 件）→ %s\n", res.Tag, len(res.Artifacts), opts.Staging)
	return res, nil
}

func dropComponent(list []string, drop string) []string {
	out := []string{}
	for _, c := range list {
		if c != drop {
			out = append(out, c)
		}
	}
	return out
}

// sha256File —— 文件 sha256（十六进制小写）。
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// writeManifest —— 写 staging 的 manifest.json（内核读它取目标版本/commit/组件/逐件 sha256）。
func writeManifest(staging string, res *BuildResult) error {
	doc := map[string]interface{}{
		"schema": 1, "version": res.Version, "tag": res.Tag, "commit": res.Commit,
		"build_time": res.BuildTime, "platform": res.Platform,
		"components": res.Components, "toolchain": res.Toolchain,
		"generated_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"artifacts":    res.Artifacts,
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(staging, "manifest.json"), append(b, '\n'), 0o644)
}

// gotoolchainPolicy —— 本次构建的 GOTOOLCHAIN 取值（默认 local）。
func gotoolchainPolicy() string {
	if v := strings.TrimSpace(os.Getenv("ZERG_GOTOOLCHAIN")); v != "" {
		return v
	}
	return "local"
}

func goBuild(dir, out, pkg, ldflags string, logw io.Writer) error {
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", out, pkg)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "GOFLAGS=-mod=mod", "GOSUMDB=off"), goToolchainEnv()...)
	cmd.Stdout, cmd.Stderr = logw, logw
	if err := cmd.Run(); err != nil {
		return err
	}
	if _, err := os.Stat(out); err != nil {
		return fmt.Errorf("go build 未产出 %s：%w", out, err)
	}
	return nil
}

func cargoBuildUI(dir, out string, logw io.Writer) error {
	cmd := exec.Command("cargo", "build", "--release", "-p", "zerg-ui")
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = logw, logw
	if err := cmd.Run(); err != nil {
		return err
	}
	src := filepath.Join(dir, "target", "release", "zerg-ui")
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cargo 未产出 %s：%w", src, err)
	}
	defer in.Close()
	dst, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, in); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
