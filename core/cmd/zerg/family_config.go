// family_config.go —— `model add`（`Q-099` · P0）+ `config reload`（`Q-100`）—— 波① `T1a`。
//
// 为什么是这两条（`任务清单-缺口收口-20260923.md` §自排补遗 `S1` 逐字）：
//
//	「`Q-099` + `Q-100` 没进清单 —— 它们是**已提级的 P0/P1** 且调研结论明确
//	 「**纯 CLI 活 · 主控零改动**」⇒ 是**性价比最高**的两条（今天正是我手搓插坏名册两次的地方）」
//
// 两条的形状（原样）：
//
//	`zerg model add …`    含 `--dry-run` ✓ 先校验后写 ✓（不许再手改 YAML 当常态 ✗）
//	`zerg config reload`  照 `nginx -s reload`：**先校验、失败回滚**（校验不过 ⇒ 不发请求 ⇒ 旧配置继续跑）
//
// 主控侧**零改动**为什么成立：热加载路由**已经在跑的主控上**（`core/cmd/zerg-core/main.go`
// 逐字：`r.Post("/api/config/reload", handlers.ReloadConfigHandler)`），且处理器**自己先解析**
// （`handlers.go`：`config.LoadFleetConfig(h.ConfigPath)` 失败即回 `500 CONFIG_LOAD_FAILED`）。
// ⇒ 本件只补**命令面**：把「改哪一件、改成什么、改完长什么样」在校验里先走一遍。
//
// 三条纪律（本件自己的红线）：
//
//	① **先校验后写、失败回滚**：写 = 同目录临时件 + `rename`（原子）；写完**读回再校**；
//	   任何一步不过 ⇒ 把原文写回（回滚）⇒ 退码 1（**绝不留下半改的件**）。
//	② **不覆盖**：同一模型块下已有同名 / 同 file ⇒ 退码 2（fail-closed · 不替人改已有的条）。
//	③ **只碰名册件**：默认 `gateway/fleet.yaml`（可用 `--root` / `--path` 指到别处）；
//	   `--dry-run` 下**一个字节都不写**（判据件现场对拍 sha256）。
//
// 字段语义（`Q-099` 修面 · 2026-09-24 —— 块定位与字段面摆正）：
//
//	`--model` = **块名**：`models:` 段的一级键就是模型名（`config.FleetConfig.Models` =
//	           `map[模型名][]ModelCandidate`）⇒ 按它定位那一块；块不存在就**新建**（列表形）。
//	`--host`  = **字段值**：写进该条的 `host:`（这台模型跑在哪台机器）—— 不当块名用；
//	           值面照名册侧既有的规则 E 现核（`scripts/gates/check-nodes-roster.py`：
//	           `models:` 段里每个 `host:` 必须在名册里）。
//	两种正规形态都认（**不新造第三种**）：列表形 `  <模型名>:` + 条目行 · 裸映射形 `  <模型名>: { … }`。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// fleetYAMLPath 解析名册件路径（`--path` 优先 → `--root` → 仓根）。
// 与主控同一件：主控的 `ConfigPath` 指的就是它（`gateway/fleet.yaml`）。
func fleetYAMLPath(inv *invocation) (string, string) {
	if p := strings.TrimSpace(inv.flagVal("--path")); p != "" {
		return p, ""
	}
	root := strings.TrimSpace(inv.flagVal("--root"))
	if root == "" {
		root = repoRoot()
	}
	if root == "" {
		return "", "解析不到仓根（给 `--root <仓根>` 或设 `ZERG_REPO`）"
	}
	return filepath.Join(root, "gateway", "fleet.yaml"), ""
}

// loadFleet 读 + 解析名册件（唯一一处解析入口：`model add` 与 `config reload` 同用）。
//
// 乙案（2026-09-23 · 名册件口径 · Mr2109 拍板）：走**主控同一函数** `config.LoadFleetConfig`
// —— 主控启动（`core/cmd/zerg-core/main.go:99`）与热加载（`core/internal/api/handlers.go:1482`）
// 调的就是它，**同一件、同一函数**才有「同件同判」。
//
// 病根（本件改前）：这里原是裸 `yaml.Unmarshal(raw, &fc)`（**严格档**），而主控的解析器认得
// 两种正规形态（`core/internal/config/config.go:140` 逐字「兼容单 dict 和多候选数组两种格式」·
// 实现见 `:217 parseCandidates`；`gateway/design.md:80/81` 教的四条形就是**裸映射**形态，
// 真名册件第 64/67/68/70 行即此形）⇒ 同一件、两套解析器 ⇒ `model add` / `config reload`
// 对现名册件**恒退 1**（真跑记录：`yaml: unmarshal errors: line 64: cannot unmarshal !!map
// into []config.ModelCandidate`）——**不是名册坏，是解析器分叉**。
//
// 路径语义不变：`--path` 优先 → `--root` → 仓根（由 `fleetYAMLPath` 定，本入口**不绑固定路径**）。
// 一处主控侧既有行为如实记下：该入口会按 `ZERG_AUTH_TOKEN` / `~/.zerg/token` 覆写 `Auth.Token`
// （`config.go:90`）——对本包无影响（这里只读 `models` / `fleet` 两面）。
func loadFleet(path string) (*config.FleetConfig, error) {
	fc, err := config.LoadFleetConfig(path)
	if err != nil {
		// 文案分两路报，不混：读不到照主控原样（「读取配置文件失败: …」），
		// 解析不过沿用本包旧风格（调用方前面已各自 stat/ReadFile 挡过读失败面）。
		if _, serr := os.Stat(path); serr != nil {
			return nil, err
		}
		return nil, fmt.Errorf("YAML 解析不过：%v", err)
	}
	return fc, nil
}

// ---- `model add`（Q-099 · P0）---------------------------------------------------------------

// modelAddSpec —— 一枚待插的名册条（字段名与 `config.ModelCandidate` 的 yaml tag 逐字对齐）。
type modelAddSpec struct {
	Host, Backend, File, Arch, Desc, Mmproj, Added string
	Ctx                                            int
	MemGb                                          float64
	HasCtx, HasMem, HasVerified                    bool
	Verified                                       bool
}

// modelAddLine 出一行 flow-style 名册条（与现册同形：`    - { host: …, backend: …, file: … }`）。
func modelAddLine(indent int, s modelAddSpec) string {
	parts := []string{
		"host: " + s.Host,
		"backend: " + s.Backend,
		"file: " + yamlStr(s.File),
	}
	if s.HasMem {
		parts = append(parts, "mem_gb: "+strconv.FormatFloat(s.MemGb, 'f', -1, 64))
	}
	if s.HasCtx {
		parts = append(parts, "ctx_window: "+strconv.Itoa(s.Ctx))
	}
	if s.Arch != "" {
		parts = append(parts, "architecture: "+s.Arch)
	}
	if s.HasVerified {
		parts = append(parts, "verified: "+strconv.FormatBool(s.Verified))
	}
	parts = append(parts, "added: \""+s.Added+"\"")
	if s.Mmproj != "" {
		parts = append(parts, "mmproj: "+yamlStr(s.Mmproj))
	}
	if s.Desc != "" {
		parts = append(parts, "description: "+yamlStr(s.Desc))
	}
	return strings.Repeat(" ", indent) + "- { " + strings.Join(parts, ", ") + " }"
}

// yamlStr —— 一律**带引号**下发字符串（路径里的 `:` / `#` / 空格不会把 flow 行拆坏）。
func yamlStr(v string) string {
	return "\"" + strings.ReplaceAll(v, "\"", "\\\"") + "\""
}

// identish —— host / backend / architecture 这类标识符的取值域（挡住会把 YAML 拆坏的字符）。
func identish(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-' || r == '@':
		default:
			return false
		}
	}
	return true
}

// modelAddPlacement —— 块定位的结果（形态 + 落点 + 新条缩进）。
type modelAddPlacement struct {
	Form   string // "list" 列表形 · "bare" 裸映射形（单条）· "new" 新建块
	At     int    // 写回落点：list/new = 插入下标（0-based）；bare = 被替换的那一行下标
	Header int    // 块首行号（1-based；new = 拟新块首行号）
	Entry  int    // **写完之后**新条那一行的行号（1-based —— 计划件与行式面都报它）
	Indent int    // 新条缩进（跟随既有条目；新建 = 4，与现册同形）
	Bare   string // 裸映射形：原条正文（`{ … }` 逐字，不含键与缩进）
}

// snipHead —— 取前 n 个字符（报文本里点名「同行接的是什么」用）。
func snipHead(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// modelAddLocate —— 按**模型名**定位 `models:` 段里的那一块（`Q-099` 修面 · 2026-09-24）。
//
// 为什么按模型名（病根）：`models:` 段的一级键**就是模型名** —— 解析侧
// `config.FleetConfig.Models` 逐字是 `map[模型名][]ModelCandidate`
// （`core/internal/config/config.go:72`），真名册件的一级键（`Ternary-Bonsai-2-27B` /
// `GLM-4.7-Flash` / `deepseek-v4-flash` / `DeepSeek-V4-Flash-Vision-Exp` …）也全是模型名。
// 改前这里拿一级键当「在册主机」用（`fc.Models[s.Host]`）⇒ 传**真主机名**一律退 2、
// 报「在册主机」时列出来的其实是模型名（两条现场记录：`--host x3`〔真主机〕⇒ 2；
// `--host deepseek-v4-flash`〔在册模型〕⇒「没有 `deepseek-v4-flash:` 这一块」）。
//
// 本笔口径：**块按 `--model`（模型名）定位/新建**；`--host` 只作该条的 `host:` 字段值。
//
// 两种正规形态都认（**不新造第三种**）：
//
//	list `  <模型名>:` + 缩进更深的条目行（flow 形 `    - { … }` 与 block 形 `    - host: …` 都算）
//	bare `  <模型名>: { … }`（单条 · 裸映射形）
//
// 块不存在 ⇒ 新建（落在 `models:` 段末，写回时用**列表形** —— 两形态里的一种）。
// 其余形态（同行接的不是 `{ … }`：块形嵌套映射、标量等）⇒ 返报错文本，**不猜、不改、不新建**。
func modelAddLocate(lines []string, name string) (modelAddPlacement, string) {
	p := modelAddPlacement{Form: "new", Indent: 4}
	modelsAt := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "models:") {
			modelsAt = i
			break
		}
	}
	if modelsAt < 0 {
		return p, "名册件里没有 `models:` 段（新开一个段要人签 ⇒ 本命令不代劳）"
	}
	// `models:` 段的范围：段内行 = 空行 / 注释 / 有缩进的行（遇到顶级键 = 段到此）
	secEnd := len(lines)
	for i := modelsAt + 1; i < len(lines); i++ {
		l := lines[i]
		if l == "" || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "#") {
			continue
		}
		secEnd = i
		break
	}
	key := "  " + name + ":" // 键那一层（缩进 2）；key 自带冒号 ⇒ `  A:` 不会被 `  A1:` 误命中
	for i := modelsAt + 1; i < secEnd; i++ {
		l := lines[i]
		if !strings.HasPrefix(l, key) {
			continue
		}
		rest := strings.TrimSpace(l[len(key):])
		if rest == "" { // ── 键那一行：列表形块首（下面必须接条目行，不能是块形嵌套映射）──
			lastContent, indent, hasEntry, hasContent := i, 0, false, false
			for j := i + 1; j < secEnd; j++ {
				lj := lines[j]
				if strings.TrimSpace(lj) == "" {
					break // 空行 ⇒ 本块到此（后面的空行/注释不属于本块）
				}
				ind := len(lj) - len(strings.TrimLeft(lj, " "))
				if ind <= 2 {
					break // 回到键那一层（含 `  # 注释`）⇒ 本块到此
				}
				if strings.HasPrefix(strings.TrimLeft(lj, " 	"), "#") {
					continue // 块内注释：跳过（不作落点）
				}
				lastContent, hasContent = j, true
				if strings.HasPrefix(strings.TrimLeft(lj, " 	"), "-") {
					indent, hasEntry = ind, true // 跟随既有条目的缩进（flow 与 block 形条目都靠它）
				}
			}
			if hasContent && !hasEntry { // 块形嵌套映射（`  <名>:` + `    host: …`）—— 不是本命令认的两种形态
				return p, fmt.Sprintf("`%s:` 这一块是**块形嵌套映射**（键下面直接是字段行，没有 `- ` 条目）⇒ 本命令不猜（不改、不新建）", name)
			}
			if indent == 0 {
				indent = 4 // 空列表块（键在、条目 0 条）：照现册惯例用 4 空格
			}
			p.Form, p.At, p.Header, p.Indent = "list", lastContent+1, i+1, indent
			p.Entry = p.At + 1
			return p, ""
		}
		if !strings.HasPrefix(rest, "{") || !strings.HasSuffix(rest, "}") { // ── 认不出的形态 ──
			return p, fmt.Sprintf("`%s:` 这一块既不是列表形，也不是同行裸映射形（同行接的是 %q）⇒ 本命令不猜（不改、不新建）",
				name, snipHead(rest, 24))
		}
		p.Form, p.At, p.Header, p.Indent, p.Bare = "bare", i, i+1, 4, rest
		p.Entry = i + 3 // 裸映射形展开成 3 行：块首 + 原条 + 新条
		return p, ""
	}
	// ── 块不存在 ⇒ 新建：落在 `models:` 段内**最后一个内容行**之后 ──
	last := modelsAt
	for i := modelsAt + 1; i < secEnd; i++ {
		if t := strings.TrimSpace(lines[i]); t != "" && !strings.HasPrefix(t, "#") {
			last = i
		}
	}
	p.At, p.Header = last+1, last+2
	p.Entry = p.At + 2 // 新建块两行：块首 + 新条 ⇒ 新条落在插入点之后一行（1-based）
	return p, ""
}

// cmdModelAdd —— `zerg model add`：先校验后写（`--dry-run` 先行 · 真写要 `--yes` · 失败回滚）。
func cmdModelAdd(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && !requireFields(inv, stderr) {
		return exitFail
	}
	s := modelAddSpec{
		Host:    strings.TrimSpace(inv.flagVal("--host")),
		File:    strings.TrimSpace(inv.flagVal("--file")),
		Backend: strings.TrimSpace(inv.flagVal("--backend")),
		Arch:    strings.TrimSpace(inv.flagVal("--arch")),
		Desc:    strings.TrimSpace(inv.flagVal("--desc")),
		Mmproj:  strings.TrimSpace(inv.flagVal("--mmproj")),
		Added:   strings.TrimSpace(inv.flagVal("--added")),
	}
	name := strings.TrimSpace(inv.flagVal("--model"))
	if name == "" {
		// `--model` 有一处**专用收口**（`setValueFlag` 的 `--model` 分支进 `modelWant`，不进 kv）——
		// 两条都读，免得「命令行给了、命令面说没给」（今天这一格就踩过一次）。
		name = strings.TrimSpace(inv.modelWant)
	}
	if s.Backend == "" {
		s.Backend = "llama-server"
	}
	if s.Added == "" {
		s.Added = time.Now().Format("2006-01-02")
	}
	s.HasVerified, s.Verified = inv.verified, inv.verified
	// ── 用法面（执行前判 · 一件都没写）──
	bad := func(detail, msg string) int {
		inv.setErr("usage", detail, msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "  用法：zerg model add --model <模型名> --host <主机> --file <GGUF 路径> [--backend …] [--mem-gb …] [--ctx …] [--arch …] [--desc …] [--mmproj …] [--added 日期] [--verified] [--dry-run | --yes]\n")
		fmt.Fprintf(stderr, "        （`--model` = `models:` 段的**键/块名** ⇒ 按它定位或新建那一块；`--host` = 该条 `host:` 的**字段值**＝这台模型跑在哪台机器）\n")
		return exitUsage
	}
	if name == "" || s.Host == "" || s.File == "" {
		miss := []string{}
		if s.Host == "" {
			miss = append(miss, "--host")
		}
		if name == "" {
			miss = append(miss, "--model")
		}
		if s.File == "" {
			miss = append(miss, "--file")
		}
		return bad("missing_target", "缺 "+strings.Join(miss, " / ")+"（这三枚必给）")
	}
	if !identish(name) {
		return bad("bad_value", "`--model` 要当 `models:` 段的键（块名）⇒ 只收标识符（字母数字与 `._-@`）—— 其余字符会把名册拆坏（新开一块更要用它当键）")
	}
	if !identish(s.Host) || !identish(s.Backend) || (s.Arch != "" && !identish(s.Arch)) {
		return bad("bad_value", "`--host` / `--backend` / `--arch` 只收标识符（字母数字与 `._-@`）—— 其余字符会把名册行拆坏")
	}
	if ctx := strings.TrimSpace(inv.flagVal("--ctx")); ctx != "" {
		n, err := strconv.Atoi(ctx)
		if err != nil || n <= 0 {
			return bad("bad_value", fmt.Sprintf("`--ctx` 要正整数（给的是 %q）", ctx))
		}
		s.Ctx, s.HasCtx = n, true
	}
	if mg := strings.TrimSpace(inv.flagVal("--mem-gb")); mg != "" {
		f, err := strconv.ParseFloat(mg, 64)
		if err != nil || f <= 0 {
			return bad("bad_value", fmt.Sprintf("`--mem-gb` 要正数（给的是 %q）", mg))
		}
		s.MemGb, s.HasMem = f, true
	}
	// `name`（`--model`）从本笔起是**块名**：`models:` 段的键就是要写/新建的那一块（见 `modelAddLocate`）。
	// 它照旧不进条目的字段面（无 `name:` 键 —— 现册条靠 `host` + `file` 认），但**决定块定位与新增**。
	if inv.confirmGiven && inv.confirm != name {
		inv.setErr("usage", "confirm_mismatch", "`--confirm` 的值要与 `--model` 逐字相同（点名要加的是哪一枚）")
		fmt.Fprintf(stderr, "%s: `--confirm=%s` 与 `--model %s` 不一致\n", progName, inv.confirm, name)
		return exitUsage
	}
	// ── 名册件面 ──
	path, perr := fleetYAMLPath(inv)
	if perr != "" {
		inv.setErr("blocked", "fleet_path_absent", perr)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 8）\n", progName, perr)
		return exitBlocked
	}
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		inv.setErr("blocked", "fleet_unreadable", "名册件读不到")
		fmt.Fprintf(stderr, "%s: 名册件读不到（%s）：%v ⇒ 不给结论（退码 8）\n", progName, path, err)
		return exitBlocked
	}
	raw := string(rawBytes)
	fc, err := loadFleet(path)
	if err != nil {
		inv.setErr("failed", "fleet_invalid", "名册件现档就解析不过（先修它，别在坏档上加）")
		fmt.Fprintf(stderr, "%s: %s 现档就解析不过：%v ⇒ 判红（退码 1 · **不写**）\n", progName, path, err)
		return exitFail
	}
	// ── 字段面（`--host` 是**值**：该模型跑在哪台机器 ⇒ 写进该条的 `host:`）──
	//
	// 为什么当场核在册：名册侧本就有这道对账 —— `scripts/gates/check-nodes-roster.py` 规则 E
	// 「`models:` 段里每个 host 必须在名册里（模型驻留在哪台必须有条目）」。改前这里拿
	// `--host` 当**块名**用（`fc.Models[s.Host]`），于是「真主机名」一律退 2、「在册模型名」
	// 反倒过关 —— 两头的语义都错。本笔把它摆正：值面照规则 E 核，块面按 `--model` 定位。
	known := map[string]bool{}
	for h := range fc.Fleet {
		known[h] = true
	}
	for _, cands := range fc.Models {
		for _, c := range cands {
			if c.Host != "" {
				known[c.Host] = true
			}
		}
	}
	if !known[s.Host] {
		list := []string{}
		for h := range known {
			list = append(list, h)
		}
		sort.Strings(list)
		inv.setErr("usage", "host_not_declared", "该主机不在名册里（`models:` 条目的 `host:` 必须在册 —— 规则 E）")
		fmt.Fprintf(stderr, "%s: `--host %s` 不在名册（`fleet:` 节点 ∪ 现存条目的 `host:`）⇒ 判用法错（退码 2）· 在册主机：%s\n", progName, s.Host, strings.Join(list, " / "))
		return exitUsage
	}
	// ── 不覆盖：同一**模型名块**下已有同 `file` / 同 `name` 的条 ⇒ 拒（fail-closed）──
	for _, c := range fc.Models[name] {
		if c.File == s.File {
			inv.setErr("usage", "duplicate_entry", "该模型块下已有同一 `file` 的条（不覆盖）")
			fmt.Fprintf(stderr, "%s: `%s:` 块下已有 `file: %s` ⇒ 拒（fail-closed · **不覆盖**别人的条）\n", progName, name, s.File)
			return exitUsage
		}
		if c.Name != "" && c.Name == name {
			inv.setErr("usage", "duplicate_name", "该模型块下已有同名条（不覆盖）")
			fmt.Fprintf(stderr, "%s: `%s:` 块下已有 `name: %s` ⇒ 拒（不覆盖）\n", progName, name, name)
			return exitUsage
		}
	}
	// ── 构造新文本 + 双解析校验（原档 → 新档）──
	// 块按**模型名**定位/新建（两种形态都认：列表形与裸映射形 —— 见 `modelAddLocate`）。
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	pl, ierr := modelAddLocate(lines, name)
	if ierr != "" {
		inv.setErr("usage", "model_block_shape_unknown", ierr)
		fmt.Fprintf(stderr, "%s: %s\n", progName, ierr)
		return exitUsage
	}
	newLine := modelAddLine(pl.Indent, s)
	var newLines []string
	switch pl.Form {
	case "list": // 既有列表形块：块末直接续一条（形态不变）
		newLines = append([]string{}, lines[:pl.At]...)
		newLines = append(newLines, newLine)
		newLines = append(newLines, lines[pl.At:]...)
	case "bare": // 既有裸映射形块：只容一条 ⇒ 要加第二条只能改**列表形**（两形态里的另一种 · 不新造第三种）
		newLines = append([]string{}, lines[:pl.At]...)
		newLines = append(newLines, "  "+name+":", "    - "+pl.Bare, newLine)
		newLines = append(newLines, lines[pl.At+1:]...)
	default: // "new"：块不存在 ⇒ 在 `models:` 段末新建（列表形）
		newLines = append([]string{}, lines[:pl.At]...)
		newLines = append(newLines, "  "+name+":", newLine)
		newLines = append(newLines, lines[pl.At:]...)
	}
	// 末尾换行：**原档有没有就照原档**（真名册件 `gateway/fleet.yaml` 末行无换行 ⇒ 写回也不给它添一个 ——
	// 「其余逐字节不变」含末换行这一字节，否则一次真写会多出第二处改动）。
	nl := "\n"
	if !strings.HasSuffix(raw, "\n") {
		nl = ""
	}
	newText := strings.Join(newLines, "\n") + nl
	// 双解析校验的**改后**那一半：同样走**主控同一入口**（`config.ParseFleetConfig`
	// 就是 `LoadFleetConfig` 里面那一步 · 病根同类第三处：此前这里是裸 `yaml.Unmarshal`
	// ⇒ 现名册件的四条裸映射**改后文本**也会被自己的严格档误判（真跑：`line 64 … !!map into
	// []config.ModelCandidate`）⇒ `model add` 对现册恒退 1）。
	recheck, err := config.ParseFleetConfig([]byte(newText))
	if err != nil {
		inv.setErr("failed", "post_write_invalid", "改后文本解析不过（回滚档 · 不写）")
		fmt.Fprintf(stderr, "%s: 改后文本解析不过（%v）⇒ 判红（退码 1 · **不写** · 这就是先校验后写的用处）\n", progName, err)
		return exitFail
	}
	// 条数判据按**模型名**那一块算（改前按 `s.Host` 取键 ⇒ 比的是另一块，甚至取到空）。
	// 三形态都落在这条上：列表形 N → N+1 · 裸映射形 1 → 2（展开成列表）· 新建 0 → 1。
	before, after := len(fc.Models[name]), len(recheck.Models[name])
	if after != before+1 {
		inv.setErr("failed", "post_write_mismatch", "改后条数不是 +1（回滚档 · 不写）")
		fmt.Fprintf(stderr, "%s: `%s` 块改后条数 %d → %d（不是 +1）⇒ 判红（退码 1 · **不写**）\n", progName, name, before, after)
		return exitFail
	}
	// ── 干跑（零副作用：不写一个字节）──
	if inv.dryRun {
		formWord := map[string]string{
			"list": "列表形（`  " + name + ":` + 条目行）",
			"bare": "裸映射形（单条 · `  " + name + ": { … }`）",
			"new":  "新建块（`models:` 段末 · 用列表形）",
		}[pl.Form]
		where := map[string]string{
			"list": fmt.Sprintf("`models:` 的 `%s:` 块末（块首第 %d 行）", name, pl.Header),
			"bare": fmt.Sprintf("`models:` 的 `%s:` 那一行原位展开（原条 = 第 %d 行 ⇒ 改列表形，正文逐字保留）", name, pl.Header),
			"new":  fmt.Sprintf("`models:` 段末新建块（块首第 %d 行）", pl.Header),
		}[pl.Form]
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : model add（%s）\n", progName)
		fmt.Fprintf(stdout, "  名册件   : %s\n", path)
		fmt.Fprintf(stdout, "  块       : `models:` 的 `%s:`（**块按 `--model`（模型名）定位/新建** · `--host` 只作该条的 `host:` 字段值）· 形态：%s\n", name, formWord)
		fmt.Fprintf(stdout, "  落点     : 第 %d 行（%s）\n", pl.Entry, where)
		fmt.Fprintf(stdout, "  新增行   : %s\n", newLine)
		if pl.Form == "bare" {
			fmt.Fprintf(stdout, "  改写行   : %s\n", "    - "+pl.Bare)
			fmt.Fprintf(stdout, "             ↑ 裸映射形只容一条 ⇒ 原条同笔改成列表形（**正文逐字不变**，不新造第三种形态）\n")
		}
		fmt.Fprintf(stdout, "  校验     : ✔ 现档解析过（`%s` 下 %d 条）· ✔ 改后解析过（%d 条）· ✔ 无同名/同 file\n", name, before, after)
		fmt.Fprintf(stdout, "  它会动   : 只此一件（%s）—— 写 = 同目录临时件 + rename（原子）；写完读回再校，任一步不过 ⇒ 回滚\n", path)
		// `Q-021`（波① `T3`）**两档同一张授权表**：干跑要把**真跑要什么**写明（否则两档判据分叉）。
		fmt.Fprintf(stdout, "  授权判据 : 真写要 `--yes`（D2 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（**一个字节都不写**）\n")
		fmt.Fprintf(stdout, "  生效     : 要主控读它 ⇒ `%s config reload --yes`（本命令**不碰主控**）\n", progName)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: 写名册是 D2 档 —— 缺 --yes ⇒ 不执行（先看计划件：加 `--dry-run`）\n", progName)
		return exitUsage
	}
	// ── 真写：临时件 + rename（原子）；写完读回再校；失败 ⇒ 回滚 ──
	if rc := writeFleetAtomic(path, raw, newText, stderr); rc != exitOK {
		inv.setErr("failed", "write_failed", "写名册件失败（已回滚）")
		return rc
	}
	row := map[string]string{"model": name, "host": s.Host, "file": s.File,
		"fleet": path, "line": strconv.Itoa(pl.Entry), "added": s.Added}
	fmt.Fprintf(stderr, "%s: 名册已写（`%s:` 块 · 新条第 %d 行 · 只此一件）—— 要主控读它：`%s config reload --yes`\n", progName, name, pl.Entry, progName)
	return listCmd(inv, stdout, stderr, []string{"model", "host", "file", "fleet", "line"}, []map[string]string{row})
}

// writeFleetAtomic —— 原子写 + 读回校验 + 失败回滚。退码：0 成功 · 1 写不过（已回滚）· 8 读不回。
func writeFleetAtomic(path, oldText, newText string, stderr io.Writer) int {
	fi, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 写前 stat 不过：%v（**未写**）\n", progName, err)
		return exitBlocked
	}
	tmp := path + ".zerg-tmp"
	if err := os.WriteFile(tmp, []byte(newText), fi.Mode().Perm()); err != nil {
		fmt.Fprintf(stderr, "%s: 临时件写不过：%v（**未动原档**）\n", progName, err)
		os.Remove(tmp)
		return exitFail
	}
	if err := os.Rename(tmp, path); err != nil {
		fmt.Fprintf(stderr, "%s: rename 不过：%v ⇒ 回滚\n", progName, err)
		os.Remove(tmp)
		return exitFail
	}
	// 读回再校：解析 + 逐字节等于想写的那份
	back, err := os.ReadFile(path)
	if err != nil || string(back) != newText {
		fmt.Fprintf(stderr, "%s: 读回校验不过（err=%v）⇒ **回滚**（把原文写回）\n", progName, err)
		if werr := os.WriteFile(path, []byte(oldText), fi.Mode().Perm()); werr != nil {
			fmt.Fprintf(stderr, "%s: !! 回滚也失败：%v —— 请人处理（原档长度 %d 字节）\n", progName, werr, len(oldText))
			return exitFail
		}
		return exitFail
	}
	// 读回再校：**主控同一入口**（病根同类第四处 · 此前裸 `yaml.Unmarshal` ⇒ 现册裸映射形
	// 写完读回也会被自己的严格档误判 ⇒ 明明写成了却回滚）。
	if _, err := config.ParseFleetConfig(back); err != nil {
		fmt.Fprintf(stderr, "%s: 读回解析不过（%v）⇒ **回滚**\n", progName, err)
		_ = os.WriteFile(path, []byte(oldText), fi.Mode().Perm())
		return exitFail
	}
	return exitOK
}

// ---- `config reload`（Q-100）---------------------------------------------------------------

// cmdConfigReload —— `zerg config reload [--dry-run] --yes`：
// **先校验、失败回滚**（照 `nginx -s reload`）—— 名册件本地解析不过 ⇒ **不发请求**（旧配置继续跑）。
// 过了才打**已在路由上**的 `POST /api/config/reload`（主控零改动 ✓）。
func cmdConfigReload(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && !requireFields(inv, stderr) {
		return exitFail
	}
	path, perr := fleetYAMLPath(inv)
	if perr != "" {
		inv.setErr("blocked", "fleet_path_absent", perr)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 8）\n", progName, perr)
		return exitBlocked
	}
	if _, err := os.Stat(path); err != nil {
		inv.setErr("blocked", "fleet_unreadable", "名册件读不到")
		fmt.Fprintf(stderr, "%s: 名册件读不到（%s）：%v ⇒ 不给结论（退码 8）\n", progName, path, err)
		return exitBlocked
	}
	// ① 先校验（本地 · 与主控**同一函数** `config.LoadFleetConfig` · 同一件：`gateway/fleet.yaml`
	// —— 主控热加载侧逐字调的就是它（`core/internal/api/handlers.go:1482`））
	fc, err := loadFleet(path)
	if err != nil {
		inv.setErr("failed", "config_invalid", "名册件解析不过 ⇒ 不请求热加载")
		fmt.Fprintf(stderr, "%s: 名册件解析不过：%v\n", progName, err)
		fmt.Fprintf(stderr, "⇒ **不发请求**（旧配置继续跑 —— 这就是「先校验、失败回滚」）· 退码 1\n")
		return exitFail
	}
	models := []string{}
	for m := range fc.Models {
		models = append(models, m)
	}
	sort.Strings(models)
	// ② 干跑：只校验 + 出计划件（**一个请求都不发**）
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : config reload（%s）\n", progName)
		fmt.Fprintf(stdout, "  名册件   : %s（先校验：✔ 解析过）\n", path)
		fmt.Fprintf(stdout, "  在册模型 : %d 个（`models:` 段的一级键 = 模型名）：%s · %d 个 fleet 节点\n", len(fc.Models), strings.Join(models, " / "), len(fc.Fleet))
		fmt.Fprintf(stdout, "  将请求   : POST %s/api/config/reload（路由**已在跑的主控上** ⇒ 主控零改动）\n", newClient().base)
		// `Q-021`（波① `T3`）**两档同一张授权表**：干跑要把真跑要什么写明。
		fmt.Fprintf(stdout, "  授权判据 : 真跑要 `--yes`（D2 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（不请求）\n")
		fmt.Fprintf(stdout, "  它会动   : 主控重读该件、更新路由表；**已加载的模型不受影响**、不重启、不停任何进程\n")
		fmt.Fprintf(stdout, "  失败面   : 主控回 400 CONFIG_PATH_MISSING / 500 CONFIG_LOAD_FAILED ⇒ 旧配置继续跑（回滚档）\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: 热加载是 D2 档（生效面即时）—— 缺 --yes ⇒ 不执行（先看计划件：加 `--dry-run`）\n", progName)
		return exitUsage
	}
	// ③ 真跑：打既有路由
	status, respBody, err := sendJSON(newClient(), "POST", "/api/config/reload", "")
	if err != nil {
		inv.setErr("unreachable", "dial_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 打不到主控：%v ⇒ 旧配置继续跑（**没重启**）\n", progName, err)
		return exitUnreachable
	}
	if status >= 400 {
		code := errorCodeOf(respBody)
		inv.setErr("failed", code, "主控拒绝了这次热加载")
		fmt.Fprintf(stderr, "%s: 主控拒绝（http_%d）· code=%s ⇒ **旧配置继续跑**（回滚档 · 没重启）\n", progName, status, orDash(code))
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		return exitFail
	}
	var resp map[string]any
	row := map[string]string{"status": "ok", "fleet": path, "raw": strings.TrimSpace(respBody)}
	if json.Unmarshal([]byte(respBody), &resp) == nil {
		for _, k := range []string{"status", "models", "added", "fleet_nodes", "fingerprint"} {
			if v, ok := resp[k]; ok {
				row[k] = cell(v)
			}
		}
	}
	fmt.Fprintf(stderr, "%s: 主控已重读名册（路由表更新 · 已加载模型不受影响 · **没重启**）\n", progName)
	return listCmd(inv, stdout, stderr, []string{"status", "models", "added", "fleet_nodes", "fingerprint"}, []map[string]string{row})
}
