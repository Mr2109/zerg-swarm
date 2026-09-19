// statepath.go — 甲批 T2（2026-09-10 Mr2109拍板）：持久状态目录
// 背景：工具计数/事件流水/错误桶原先落 /tmp —— macOS 重启即清 + tmp_cleaner 3 天未访问即删（实测本机两者都在）。
// 规则：默认 ~/.zerg/state/（ZERG_STATE_DIR 可覆盖）；首次访问自动从旧 /tmp 路径搬一次（目标已存在则不覆盖）。
package statepath

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Dir — 状态目录（ZERG_STATE_DIR 覆盖 → 默认 ~/.zerg/state）
func Dir() string {
	if d := os.Getenv("ZERG_STATE_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/zerg-state"
	}
	return filepath.Join(home, ".zerg", "state")
}

// File — 状态目录下的文件绝对路径
func File(name string) string { return filepath.Join(Dir(), name) }

// MigrateIfNeeded — 旧 /tmp 文件 → 状态目录（一次性；目标已存在则跳过；失败仅告警不阻断）
func MigrateIfNeeded(legacyPath, name string) string {
	dst := File(name)
	if _, err := os.Stat(dst); err == nil {
		return dst // 已有新文件——幂等
	}
	src, err := os.Stat(legacyPath)
	if err != nil {
		return dst // 无旧文件——用新路径（首次创建）
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		log.Printf("⚠️ failed to create state directory (continuing with legacy path %s): %v", legacyPath, err)
		return legacyPath
	}
	if src.IsDir() {
		if err := copyDir(legacyPath, dst); err != nil {
			log.Printf("⚠️ state directory migration failed (continuing with legacy path %s): %v", legacyPath, err)
			return legacyPath
		}
	} else if err := copyFile(legacyPath, dst); err != nil {
		log.Printf("⚠️ state file migration failed (continuing with legacy path %s): %v", legacyPath, err)
		return legacyPath
	}
	log.Printf("✅ state migrated out of /tmp: %s → %s (legacy files kept, not deleted)", legacyPath, dst)
	return dst
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := copyDir(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// ─────────────── 2026-09-11 B 批（环境无关化）：路径与端口统一解析 ───────────────
// 目标：任何硬编码的机器路径/端口都集中在此，且都有合理默认值 + 环境变量覆盖，
// 使项目在别人的机器上也能跑（不再依赖 <volume-path>）。

// WorkspaceRoot — 工作区（仓库）根目录。
// 覆盖顺序：ZERG_WORKSPACE → 可执行文件所在目录的上一级（bin/ 的父目录）→ 当前工作目录。
func WorkspaceRoot() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_WORKSPACE")); d != "" {
		return d
	}
	// 从当前目录向上找仓库根（有 .git 或 core/go.mod 的那一级）——
	// 测试进程的 cwd 是包目录，靠这一条仍能定位仓库根（searxng vendor 路径依赖它）
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 8; i++ {
			if isRepoRoot(dir) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	// 可执行文件在 bin/ 下 → 其父目录即仓库根
	if exe, err := os.Executable(); err == nil {
		binDir := filepath.Dir(exe)
		if filepath.Base(binDir) == "bin" {
			return filepath.Dir(binDir)
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// isRepoRoot — 判定目录是否为仓库根（.git 或 core/go.mod 存在）。
func isRepoRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "core", "go.mod")); err == nil {
		return true
	}
	return false
}

// SkillsDir — CA（子端）技能目录：每个子目录一份 SKILL.md（v2.5.1 技能库）。
// 解析顺序：ZERG_SKILLS_DIR → <仓库根>/core/internal/agent/skills（存在才用）→ 空串（不猜、不假装有）。
// 为什么不写死绝对路径：写死只在作者那台机器上成立，换机器/换安装位置就瞎（B1 环境无关化定案）。
// 为什么不放进 ~/.zerg：技能是**随仓库发布**的文本资产，不是运行态状态。
func SkillsDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_SKILLS_DIR")); d != "" {
		return d
	}
	root := WorkspaceRoot()
	if root == "" {
		return ""
	}
	d := filepath.Join(root, "core", "internal", "agent", "skills")
	if st, err := os.Stat(d); err == nil && st.IsDir() {
		return d
	}
	return ""
}

// IssuesDir — 内部任务单（issue）目录：引擎写单与调度扫单的**唯一出口**。
// 解析顺序：ZERG_ISSUES_DIR → <ZERG_STATE_DIR>/issues（默认 ~/.zerg/state/issues/）。
// 显式覆盖 = 唯一来源，**不退旧**：即便该路径不存在也原样返回（不 stat、不回退仓内 docs/issues）——
// 口径照 idle_persist.go 的 idleReadPath：覆盖即「我已指定唯一来源」；不猜、不假装有。
// 为什么不默认放仓内：docs/issues/ 随 git 走、要评审要归档，而任务单是**运行态活数据**；
// 放统一状态目录才随 ZERG_STATE_DIR 一起可迁、第二实例天然隔离（B1 环境无关化定案）。
func IssuesDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_ISSUES_DIR")); d != "" {
		return d
	}
	return File("issues")
}

// IssuesDirExplicit — ZERG_ISSUES_DIR 是否被显式设置（唯一出口的覆盖开关）。
// 调用方（调度器扫单）靠它决定是否并入只读旧目录 <workDir>/docs/issues：
// 覆盖态 = 只扫 IssuesDir()，旧目录完全不看（行为变化，见设计稿 §4/§10.4）。
func IssuesDirExplicit() bool {
	return strings.TrimSpace(os.Getenv("ZERG_ISSUES_DIR")) != ""
}

// ─────────────── 2026-09-19「开发文档分家」：<项目文档> 取源根双认 ───────────────
// 背景：`docs/项目文档/` 整目录已移出工作树（真身 = 与仓库同级的 `Zerg-内部文档/项目文档/`）。
// 凡「读项目文档」的取源根一律走这里 ⇒ **仓内优先、缺则仓外**（与门禁/工具同口径，
// 见 `docs/site/export-and-build.sh:48` 的 `ZERG_DOCS_ALT` 与 `tools/kb_docs_sync.py:32-34`）。
// ★ 关键：**缺件不许静默**。原先各处把「仓内 docs/项目文档」写死成包级常量 ⇒ 分家后
// 目录不在盘上，`glob` 返回空、函数静默返回空串（= 假装「没有文档」而不是「没找到根」）。

// DocsAltRoot — 仓外「开发文档」根（Zerg-内部文档，与仓库同级、在工作树之外）。
// 覆盖顺序：ZERG_DOCS_ALT → <仓库根>/../Zerg-内部文档（与 export-and-build.sh 同根同默认）。
func DocsAltRoot() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_DOCS_ALT")); d != "" {
		return d
	}
	return filepath.Join(WorkspaceRoot(), "..", "Zerg-内部文档")
}

// DocsBaseCandidates — 「项目文档」取源根候选（**顺序即优先级**）：
//   - ① 仓内 <仓库根>/docs/项目文档（未分家的机器 / 公开树 / CI 快照）
//   - ② 仓外 <ZERG_DOCS_ALT|../Zerg-内部文档>/项目文档（2026-09-19 二次分家后的真身）
func DocsBaseCandidates() []string {
	return []string{
		filepath.Join(WorkspaceRoot(), "docs", "项目文档"),
		filepath.Join(DocsAltRoot(), "项目文档"),
	}
}

// DocsBase — 「项目文档」取源根：**仓内优先、缺则仓外**。
// 判定用 os.Stat（真在盘上才算），返回值已 Clean。
// **两处都缺 ⇒ 返回空串**——不猜、不假装有、不回退别的目录；调用方必须把这个空当「缺件」报出来
// （配 DocsBaseMissingNote），不许静默当成「目录里没有内容」。
func DocsBase() string {
	for _, p := range DocsBaseCandidates() {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return filepath.Clean(p)
		}
	}
	return ""
}

// DocsBaseMissingNote — 两处取源根都不在盘上时的**可读缺件说明**（逐个候选点名 + 绝对路径）。
// 「缺件不静默」的落地：任何因缺件而空的返回值/日志都要带上它 ⇒ 现场能一眼看出
// 「是仓内根没有、还是连仓外根也没找到」，而不是看到一片空白去猜。
func DocsBaseMissingNote() string {
	c := DocsBaseCandidates()
	return fmt.Sprintf("项目文档取源根未找到（仓内根 %s ✗ · 仓外根 %s ✗）", c[0], c[1])
}

// tmpBase — 临时根（覆盖顺序：ZERG_TMP_DIR → /tmp）。
// 注意：默认**必须**是字面 "/tmp"，不能换成 os.TempDir()——macOS 的 os.TempDir() 是
// /var/folders/…，而 CA 任务的执行闸门、文档与既有任务目录约定都按 /tmp 设计（换默认=回归）。
func tmpBase() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_TMP_DIR")); d != "" {
		return d
	}
	return "/tmp"
}

// TaskRoot — CA 任务根目录（每个任务一个子目录）。
// 覆盖顺序：ZERG_TASK_ROOT → <ZERG_TMP_DIR|/tmp>/zerg-tasks
func TaskRoot() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_TASK_ROOT")); d != "" {
		return d
	}
	return filepath.Join(tmpBase(), "zerg-tasks")
}

// CAEventLogRoot — CA 事件日志根目录（每次运行一个时间戳子目录）。
// 覆盖顺序：ZERG_CA_LOG_DIR → <ZERG_TMP_DIR|/tmp>/zerg-ca-logs
func CAEventLogRoot() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_CA_LOG_DIR")); d != "" {
		return d
	}
	return filepath.Join(tmpBase(), "zerg-ca-logs")
}

// RuntimeLogDir — 运行期日志目录（主控 stdout、心跳、localback 等）。
// 覆盖顺序：ZERG_LOG_DIR → <ZERG_TMP_DIR|/tmp>（默认与既有行为一致，便于 UI 日志面板读取）
func RuntimeLogDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_LOG_DIR")); d != "" {
		return d
	}
	return tmpBase()
}

// portFromEnv — 端口类环境变量解析（非法或越界则回落默认值）。
func portFromEnv(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 65536 {
			return n
		}
		log.Printf("⚠️ environment variable %s=%q invalid, using default port %d", key, v, def)
	}
	return def
}

// CorePort — 主控 API 端口（ZERG_PORT → 8580）
func CorePort() int { return portFromEnv("ZERG_PORT", 8580) }

// GatewayPort — 网关端口（ZERG_GATEWAY_PORT → 8082）
func GatewayPort() int { return portFromEnv("ZERG_GATEWAY_PORT", 8082) }

// AgentPort — 子端 HTTP 端口（ZERG_AGENT_PORT → 8100）
func AgentPort() int { return portFromEnv("ZERG_AGENT_PORT", 8100) }

// CoreBaseURL — 主控 API 基址（内部调用用——端口随 ZERG_PORT）
func CoreBaseURL() string { return "http://127.0.0.1:" + strconv.Itoa(CorePort()) }

// GatewayBaseURL — 网关基址（内部调用用——端口随 ZERG_GATEWAY_PORT）
func GatewayBaseURL() string { return "http://127.0.0.1:" + strconv.Itoa(GatewayPort()) }

// CompressModelsDir — 压缩模型目录（LLMLingua-2 等本地 ONNX 模型）。
// 覆盖顺序：ZERG_COMPRESS_MODELS → <工作区>/compress_models
func CompressModelsDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_COMPRESS_MODELS")); d != "" {
		return d
	}
	return filepath.Join(WorkspaceRoot(), "compress_models")
}
