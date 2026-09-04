package chat

// chat_tool_overview.go — zerg_overview 虫族系统总览工具（v2.5.7——2026-09-02）
// 设计: docs/项目文档/v2.5.7/设计-zerg-overview-系统总览工具-20260902.md
// 核心: 指针不是快照——不存内容——执行时聚合活源（版本目录/代码目录/服务状态/git 历史）——永远最新——零维护
// 自进化: 了解系统→改系统→git commit→新版本目录→工具自动反映（Mr2109: 虫族自进化关键）

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ZergDocsBase — 版本档案根目录
const ZergDocsBase = "<repo>/docs/项目文档"
const ZergRepoRoot = "<repo>"

// zergOverview — zerg_overview 执行器（全景 / section 下钻）
func zergOverview(args map[string]any, workDir string) (string, error) {
	// section 下钻（模块/使用——深度信息）
	if sec, _ := args["section"].(string); sec != "" {
		return zergOverviewSection(strings.ToLower(strings.TrimSpace(sec)))
	}
	return zergOverviewFull()
}

// ============ ① 版本解析（多级 fallback——设计遗漏#5） ============

// latestVersionDir — 最新版本档案目录（glob docs/项目文档/v* → 版本号最大）
// fallback 级: ①版本号解析 ②修改时间 ③字典序
func latestVersionDir() string {
	dirs, _ := filepath.Glob(filepath.Join(ZergDocsBase, "v*"))
	if len(dirs) == 0 {
		return ""
	}
	// 候选排序（版本号降序）
	sort.Slice(dirs, func(i, j int) bool {
		vi, vj := parseVersion(filepath.Base(dirs[i])), parseVersion(filepath.Base(dirs[j]))
		return compareVersion(vi, vj) > 0
	})
	// 守卫: 版本目录文档数 <3 = 空壳（新版本刚建档未填充——2026-09-05 实测 v2.6 两文件导致
	// 使用指南/INDEX 断链）——回退到第一个文档齐全的版本
	for _, d := range dirs {
		if len(globMd(d)) >= 3 {
			return d
		}
	}
	return dirs[0]
}

// parseVersion — "v2.5.7" → [2,5,7]（容忍 v2.5.7-xxx / 2.5.7）
func parseVersion(s string) []int {
	s = strings.TrimPrefix(s, "v")
	s = strings.SplitN(s, "-", 2)[0]
	parts := strings.Split(s, ".")
	ver := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		ver = append(ver, n)
	}
	return ver
}

func compareVersion(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] - b[i]
		}
	}
	return len(a) - len(b)
}

// ============ ② 全景聚合 ============

// zergOverviewFull — 全景（精炼 ~800 字——架构 + 使用精简 + 变化 + 状态 + 文档 + 模块 + 工具）
func zergOverviewFull() (string, error) {
	verDir := latestVersionDir()
	verName := "未知"
	if verDir != "" {
		verName = filepath.Base(verDir)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# 虫族系统（%s）\n", verName))

	// 架构（组件 + 路径一句话——精简——防重复）
	b.WriteString("\n## 架构\n")
	b.WriteString("主控core(core/) 模型集群(X3 g01@<worker-ip>/local/mini) UI(ui/ egui) CA(任务执行) 工具库(136 tools/) 知识库(00 项目/知识库/knowledge.db)\n")

	// 使用精简（分流——完整见使用指南）
	b.WriteString("\n## 怎么使用\n")
	b.WriteString("请求→①即时问答:直答(带工具) ②系统查:task_list/fleet_status/service_status ③经验:kb_search ④长任务:派单→CA ⑤内部进化:内部任务16类 ⑥不确定:section下钻。中文回复。完整指南: section=使用\n")

	// 最近变化
	b.WriteString("\n## 最近变化\n")
	b.WriteString(recentChanges(3))

	// 运行状态（压缩）
	b.WriteString("\n## 运行状态\n")
	svc, serr := serviceStatus()
	if serr == nil && svc != "" {
		svcLines := strings.Split(strings.TrimSpace(svc), "\n")
		if len(svcLines) > 3 {
			svc = strings.Join(svcLines[:3], "\n")
		}
		b.WriteString(svc + "\n")
	} else {
		b.WriteString("（状态获取失败——服务可能未启动）\n")
	}
	// fleet 状态不内嵌（JSON 太长——模型需要时 section=集群 或直接调 fleet_status）

	// 文档入口（指针化——先读 INDEX 定向——doc_search 检索片段——不全文读）
	b.WriteString("\n## 文档（查文档先读目录 INDEX.md 定向——doc_search 检索关键词）\n")
	if verDir != "" {
		verBase := filepath.Base(verDir)
		b.WriteString(fmt.Sprintf("  当前版 %s: INDEX.md（%d 份文档——先读它定位）→ 或 doc_search query=\"主题\"\n", verBase, len(globMd(verDir))))
		b.WriteString("  总索引: docs/INDEX.md（版本史/常青/skills 导航）\n")
		b.WriteString("  历史版: docs/项目文档/vX.Y.Z/（INDEX 标历史档案——查旧版走 doc_search scope=vX.Y.Z）\n")
	} else {
		b.WriteString("  （未找到版本档案目录）\n")
	}

	// 模块代码地图（动态扫 internal/*——只列关键 10 个——省略省字）
	b.WriteString("\n## 模块地图（section= 名字 下钻——如 section=agent）\n")
	mods, _ := filepath.Glob(filepath.Join(ZergRepoRoot, "core/internal/*"))
	var names []string
	for _, m := range mods {
		names = append(names, filepath.Base(m))
	}
	if len(names) > 10 {
		b.WriteString("  core/internal/{" + strings.Join(names[:10], ",") + ",…}（共 " + strconv.Itoa(len(names)) + " 个）\n")
	} else {
		b.WriteString("  core/internal/{" + strings.Join(names, ",") + "}\n")
	}

	// 工具提示（精简）
	b.WriteString("\n## 工具\n")
	b.WriteString("task_list/fleet_status/service_status/web_search/doc_search(查文档)/tool_search——zerg_overview section=模块 下钻\n")

	out := b.String()
	if len([]rune(out)) > 950 {
		rs := []rune(out)
		out = string(rs[:950]) + "\n…（全景超长——深度用 section= 下钻）\n"
	}
	return out, nil
}

// globMd — 列版本目录 md（排序）
func globMd(dir string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	sort.Strings(files)
	return files
}

// recentChanges — git log 最近 N 条（缓存 30s——AHZ 卷 git 慢）
var overviewGitCache struct {
	ts   time.Time
	data string
}

func recentChanges(n int) string {
	if n <= 0 {
		n = 3
	}
	if time.Since(overviewGitCache.ts) < 30*time.Second && overviewGitCache.data != "" {
		return overviewGitCache.data
	}
	cmd := exec.Command("git", "-C", ZergRepoRoot, "log", "--oneline", "-5")
	out, err := cmd.Output()
	if err != nil {
		return "（git 历史获取失败）\n"
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sb strings.Builder
	for i, l := range lines {
		if i >= n || strings.TrimSpace(l) == "" {
			break
		}
		// 提交信息截断 40 字（注意力——全景紧凑）
		if len(l) > 40 {
			l = string([]rune(l)[:40]) + "…"
		}
		sb.WriteString("  " + l + "\n")
	}
	overviewGitCache.ts = time.Now()
	overviewGitCache.data = sb.String()
	return overviewGitCache.data
}

// ============ ③ section 下钻 ============

// zergOverviewSection — 模块/使用下钻（动态映射——模块名匹配文档 + 代码目录 + 相关工具）
func zergOverviewSection(sec string) (string, error) {
	verDir := latestVersionDir()
	// 特殊 section: 使用/指南 → 使用指南文档
	if sec == "使用" || sec == "指南" || sec == "usage" || sec == "使用虫族" {
		if verDir != "" {
			guide := filepath.Join(verDir, "使用-虫族指南-20260902.md")
			if _, err := os.Stat(guide); err == nil {
				b, _ := os.ReadFile(guide)
				return fmt.Sprintf("# 使用虫族指南（%s——全文——人模型双视角）\n%s", filepath.Base(verDir), string(b)), nil
			}
		}
		return "", fmt.Errorf("使用指南文档未找到")
	}
	// 架构 section
	if sec == "架构" || sec == "总览" || sec == "overview" || sec == "核心" {
		if verDir != "" {
			archFiles, _ := filepath.Glob(filepath.Join(verDir, "00-架构*.md"))
			if len(archFiles) > 0 {
				b, _ := os.ReadFile(archFiles[0])
				// 返回文档首 2500 字（深度——全文让模型 read）
				content := string(b)
				if len([]rune(content)) > 2500 {
					content = string([]rune(content)[:2500]) + "\n…（文档长——完整: read " + archFiles[0] + "）\n"
				}
				return fmt.Sprintf("# 虫族架构（%s）\n%s", filepath.Base(verDir), content), nil
			}
		}
	}
	// 模块 section: 匹配文档（01-模块-xxx）或代码目录（core/internal/xxx）
	if verDir != "" {
		modFiles, _ := filepath.Glob(filepath.Join(verDir, "*-模块-*.md"))
		codeDir := filepath.Join(ZergRepoRoot, "core/internal", sec)
		_, statErr := os.Stat(codeDir)
		codeExists := statErr == nil
		for _, mf := range modFiles {
			// 文档中文名（去掉数字前缀/模块-/日期后缀——如 "任务系统"）
			docName := moduleDocName(mf)
			if docName == "" {
				continue
			}
			// 匹配: section 含模块名 或 模块名含 section（宽松子串）
			if strings.Contains(strings.ToLower(sec), strings.ToLower(docName)) || strings.Contains(strings.ToLower(docName), strings.ToLower(sec)) {
				b, _ := os.ReadFile(mf)
				content := string(b)
				if len([]rune(content)) > 2000 {
					content = string([]rune(content)[:2000]) + "\n…（文档长——完整: read " + mf + "）\n"
				}
				return fmt.Sprintf("# 模块: %s\n%s", filepath.Base(mf), content), nil
			}
		}
		// 代码目录存在（无文档匹配）——列内容
		if codeExists {
			entries, _ := os.ReadDir(codeDir)
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# 模块代码: core/internal/%s\n", sec))
			for _, e := range entries {
				if e.IsDir() {
					sb.WriteString("  📁 " + e.Name() + "/\n")
				} else if strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
					sb.WriteString("  " + e.Name() + "\n")
				}
			}
			return sb.String(), nil
		}
	}
	// 未知 section——列可用模块（从文档列表推导）
	avail := "可用 section: 架构/使用"
	if verDir != "" {
		modFiles, _ := filepath.Glob(filepath.Join(verDir, "*-模块-*.md"))
		for _, mf := range modFiles {
			parts := strings.SplitN(filepath.Base(mf), "-", 3)
			if len(parts) == 3 {
				name := strings.TrimSuffix(parts[2], filepath.Ext(parts[2]))
				name = strings.TrimSuffix(name, "-20260829")
				avail += "/" + name
			}
		}
	}
	return "", fmt.Errorf("未知模块 %q——可用: %s（或 core/internal 目录名）", sec, avail)
}

// moduleDocName — 模块文档中文名（"01-模块-主控core-20260829.md" → "主控core"）
func moduleDocName(path string) string {
	base := filepath.Base(path)
	parts := strings.SplitN(base, "-", 3)
	if len(parts) != 3 {
		return ""
	}
	name := strings.TrimSuffix(parts[2], filepath.Ext(parts[2]))
	// 去日期后缀（-20260829 / -20260902 等）
	for {
		idx := strings.LastIndex(name, "-2")
		if idx <= 0 {
			break
		}
		suffix := name[idx+1:]
		if len(suffix) == 8 {
			if _, err := strconv.Atoi(suffix); err == nil {
				name = name[:idx]
				continue
			}
		}
		break
	}
	return name
}
