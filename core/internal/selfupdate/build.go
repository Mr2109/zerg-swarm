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

// Platform —— 本机平台标识（与 zerg-upgrade.sh 的 PLAT 同形：darwin-arm64 / linux-amd64）。
func Platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

// Artifact —— 一件构建产物（名/路径/sha256）。
type Artifact struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// BuildResult —— 一次本机构建的结果（staging 目录即"git 树构建产物"，交内核换装）。
type BuildResult struct {
	Staging   string     `json:"-"`
	Version   string     `json:"version"`
	Tag       string     `json:"tag"`
	Commit    string     `json:"commit"`
	BuildTime string     `json:"build_time"`
	Platform  string     `json:"platform"`
	Artifacts []Artifact `json:"artifacts"`
}

// BuildOptions —— 本机构建入参。
type BuildOptions struct {
	Source    string // 要构建的源码树（target commit 的临时检出）
	Staging   string // 产物落点（临时区）
	Commit    string // 目标 commit（注入身份）
	NoUI      bool   // 跳过 UI（--no-ui）
	LogWriter io.Writer
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

// Build —— 从 source 树本地构建主控/子端（Go），UI 仅在 darwin 构建（设计 §3：
// Rust UI 在 X3 上编不动，故 UI 仍只在 Mac 出）。产物 + manifest.json 落到 staging，
// 由既有六阶段内核校验后换装。
func Build(opts BuildOptions) (*BuildResult, error) {
	if opts.Source == "" || opts.Staging == "" {
		return nil, fmt.Errorf("Build 需要 source 与 staging")
	}
	logw := opts.LogWriter
	if logw == nil {
		logw = io.Discard
	}
	version, err := readVersionFromTree(opts.Source)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.Staging, 0o755); err != nil {
		return nil, err
	}
	buildTime := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	plat := Platform()
	ldflags := fmt.Sprintf("-s -w -X %s.Commit=%s -X %s.BuildTime=%s", ModulePath, opts.Commit, ModulePath, buildTime)

	res := &BuildResult{
		Staging: opts.Staging, Version: version, Tag: "v" + version,
		Commit: opts.Commit, BuildTime: buildTime, Platform: plat,
	}

	// 主控 / 子端（Go，同一 core 模块）。
	for _, c := range []struct{ name, pkg string }{
		{"zerg-core", "./cmd/zerg-core"},
		{"zerg-agent", "./cmd/zerg-agent"},
	} {
		out := filepath.Join(opts.Staging, fmt.Sprintf("%s-%s", c.name, plat))
		fmt.Fprintf(logw, "→ 构建 %s（%s）\n", c.name, c.pkg)
		if err := goBuild(filepath.Join(opts.Source, "core"), out, c.pkg, ldflags, logw); err != nil {
			return nil, fmt.Errorf("构建 %s 失败：%w", c.name, err)
		}
	}

	// UI：仅在 darwin 构建（设计 §3 / §4.2）。
	if !opts.NoUI && runtime.GOOS == "darwin" {
		out := filepath.Join(opts.Staging, "zerg-ui-"+plat)
		fmt.Fprintf(logw, "→ 构建 UI（cargo release）\n")
		if err := cargoBuildUI(filepath.Join(opts.Source, "ui"), out, logw); err != nil {
			return nil, fmt.Errorf("构建 UI 失败：%w", err)
		}
	}

	// 逐件算 sha256（**签名之前**的口径——内核会在换装时对暂存件先校 sha 再 ad-hoc 重签）。
	for _, name := range listArtifactNames(opts.Staging) {
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

func listArtifactNames(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, "zerg-") && !strings.HasSuffix(n, ".sha256") {
			out = append(out, filepath.Join(dir, n))
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

// writeManifest —— 写 staging 的 manifest.json（内核读它取目标版本/commit/逐件 sha256）。
func writeManifest(staging string, res *BuildResult) error {
	doc := map[string]interface{}{
		"schema": 1, "version": res.Version, "tag": res.Tag, "commit": res.Commit,
		"build_time": res.BuildTime, "platform": res.Platform,
		"generated_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"artifacts":    res.Artifacts,
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(staging, "manifest.json"), append(b, '\n'), 0o644)
}

func goBuild(dir, out, pkg, ldflags string, logw io.Writer) error {
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", out, pkg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOSUMDB=off")
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
