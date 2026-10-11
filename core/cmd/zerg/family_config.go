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
	SizeBytes                                      int64
	HasCtx, HasMem, HasVerified, HasSize           bool
	Verified                                       bool
	Cmd                                            []string
	HasCmd                                         bool
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
	if s.HasSize {
		parts = append(parts, "size_bytes: "+strconv.FormatInt(s.SizeBytes, 10))
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
	if s.HasCmd {
		elems := make([]string, 0, len(s.Cmd))
		for _, e := range s.Cmd {
			elems = append(elems, yamlStr(e))
		}
		parts = append(parts, "cmd: ["+strings.Join(elems, ", ")+"]")
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
// 报「在册主机」时列出来的其实是模型名（两条现场记录：`--host <真主机名>`〔真主机〕⇒ 2；
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

// requireJSONFields —— 危险档 `--json` 用法面的**收口处**（本族唯一一份）。
//
// ★ 不是另开一份口径，而是把两个**既有**口子拼起来、顺序写死：
//   - 判退码：`requireFields`（`main.go:2702` · 退码表唯一出口 —— 本函数不自己定码）；
//   - 记错误面：`inv.setErr`（`errors.go:138` · 谁先报谁为准 —— 本函数不自己拼包封）。
//
// 补的是 `requireFields` 的已知缺口（`GAP-20260927-360`）：那个口子只写 stderr、**不调 `setErr`**
// ⇒ 危险档（`main.go:182-183` 的设计：危险动作没有结果面，但错误面必须机器可读 —— §九 M7）
// 走到这里时包封里没有 `error.detail`，退化成「（命令未报出 kind，按退码兜底）」，AI 自愈只剩退码。
// gap 族 6 处（`family_gap.go:558/726/990` · `family_gap_export.go:50` · `family_gap_state.go:223/356`）
// 是各写各的 `setErr("usage","json_fields_required","--json 不给字段")`；本族（`model add` /
// `config reload`）此前**一处都没写**。此处收成一份，与那 6 处**逐字同 kind 同 detail**。
func requireJSONFields(inv *invocation, stderr io.Writer) int {
	rc := requireFields(inv, stderr)
	if rc != exitOK {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
	}
	return rc
}

// cmdModelAdd —— `zerg model add`：先校验后写（`--dry-run` 先行 · 真写要 `--yes` · 失败回滚）。
func cmdModelAdd(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.jsonGiven {
		if rc := requireJSONFields(inv, stderr); rc != exitOK {
			return rc
		}
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
		fmt.Fprintf(stderr, "  用法：zerg model add --model <模型名> --host <主机> --file <GGUF 路径> --confirm <模型名> [--backend …] [--mem-gb …] [--size-bytes …] [--ctx …] [--arch …] [--desc …] [--mmproj …] [--added 日期] [--verified] [--cmd <JSON 数组>] [--dry-run | --yes]\n")
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
	if sb := strings.TrimSpace(inv.flagVal("--size-bytes")); sb != "" {
		n, err := strconv.ParseInt(sb, 10, 64)
		if err != nil || n <= 0 {
			return bad("bad_value", fmt.Sprintf("`--size-bytes` 要正整数（给的是 %q）", sb))
		}
		s.SizeBytes, s.HasSize = n, true
	}
	// `--cmd <JSON 数组>`（缺口 GAP-20260925-26）：子端自定义启动命令（专用 fork 二进制 + 包装脚本）。
	// 值是**一个 JSON 字符串数组**（如 `["/path/run.sh","-m","{file}"]`）—— 非数组 / 元素非串 / 空数组 ⇒ 退 2 点名。
	if cmdRaw := strings.TrimSpace(inv.flagVal("--cmd")); cmdRaw != "" {
		var arr []string
		if jerr := json.Unmarshal([]byte(cmdRaw), &arr); jerr != nil {
			return bad("bad_value", fmt.Sprintf("`--cmd` 要一个 JSON 字符串数组（如 `[\"/path/run.sh\",\"-m\",\"{file}\"]`）—— 解析不过：%v", jerr))
		}
		if len(arr) == 0 {
			return bad("bad_value", "`--cmd` 给的是空数组 —— 要么给一条真命令、要么整枚别给（✗ 不许静默当空）")
		}
		s.Cmd, s.HasCmd = arr, true
	}
	// ── V016 前置（缺口 GAP-20260925-26）：需要非主线引擎实现的卵**缺 cmd:** ⇒ 写入前就拒 ──
	// 口径同 config/validator.go 的 V016（`NeedsEngineImpl` ⇒ `len(Cmd)==0` 即 Fatal 拒孵）——
	// 把它从「孵卵时」前移到「写名册前」，免得写进一条 V016 必拒的条（会静默退回主线 llama-server）。
	if config.NeedsEngineImpl(config.ModelCandidate{Backend: s.Backend, Architecture: s.Arch}) && !s.HasCmd {
		return bad("engine_impl_required_missing_cmd",
			fmt.Sprintf("`--arch %s` 主线引擎不支持 ⇒ 必须同时给 `--cmd '<JSON 数组>'`（专用 fork 二进制 + 包装脚本）—— ✗ 不许静默退回主线 llama-server（V016）", s.Arch))
	}
	// `name`（`--model`）从本笔起是**块名**：`models:` 段的键就是要写/新建的那一块（见 `modelAddLocate`）。
	// 它照旧不进条目的字段面（无 `name:` 键 —— 现册条靠 `host` + `file` 认），但**决定块定位与新增**。
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
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
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
		fmt.Fprintf(stdout, "  生效     : 写后**即**打 `POST /api/config/reload` + **现读校验路由**（只读面 `GET /api/fleet/models` 认了才算数 · GAP-20260925-42）—— 仅当写的是主控现读件 `<仓根>/gateway/fleet.yaml`；`--path`/`--root` 指的别件不联动\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	//
	// ── 确认档（fail-closed · `GAP-20260927-87` 修面 · 2026-10-03）──
	// 位置口径（干跑豁免 · 照同族先例 `guard.go:59`→`:68` · `family_task.go:495`→`:513` ·
	// `family_publish_run.go:105`→`:131`）：`--dry-run` 已在上面短路返回 ⇒ 本块只对**真写**生效
	// （干跑一个字节都不写、也**不要求** `--confirm`）。
	// 改前：`if inv.confirmGiven && inv.confirm != name` —— **只在给了 `--confirm` 时才校验** ⇒
	// 「从不要求」= 危险档（写名册件）与确认档**不是一一映射**（§九 M3 C1 / §4.1 K7）。
	// 改后：缺 `--confirm` ⇒ 2（不执行）；值不逐字相同 ⇒ 2（给错值 = 拒绝，不是「当没给」）。
	if !inv.confirmGiven {
		inv.setErr("usage", "confirm_required", "写名册件要 `--confirm=<模型名>`（点名要加的是哪一枚）—— 缺确认 ⇒ 不执行（fail-closed）")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: `model add` 会写名册件 ⇒ 缺 `--confirm=%s` ⇒ 不执行（fail-closed）\n", progName, name)
		fmt.Fprintf(stderr, "        （`--confirm` 的值要与 `--model` 逐字相同 —— 危险档 ↔ 确认档一一映射 · §九 M3 C1）\n")
		return exitUsage
	}
	if inv.confirm != name {
		inv.setErr("usage", "confirm_mismatch", "`--confirm` 的值要与 `--model` 逐字相同（点名要加的是哪一枚）")
		guardMarkDecision(inv, guardDecisionBlock)
		fmt.Fprintf(stderr, "%s: `--confirm=%s` 与 `--model %s` 不一致 ⇒ 不执行（给错值 = 拒绝，不是「当没给」）\n", progName, inv.confirm, name)
		return exitUsage
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: 写名册是 D2 档 —— 缺 --yes ⇒ 不执行（先看计划件：加 `--dry-run`）\n", progName)
		return exitUsage
	}
	// ── 真写：临时件 + rename（原子）；写完读回再校；失败 ⇒ 回滚 ──
	if rc := writeFleetAtomic(path, raw, newText, stderr); rc != exitOK {
		inv.setErr("failed", "write_failed", "写名册件失败（已回滚）")
		return rc
	}
	// ── 缺口 GAP-20260925-42：写名册与路由生效**并成一步** ──────────────────────────────────
	// 改前：本命令只写名册，把「生效」推给下一条 `config reload --yes`，且**单凭它报的 ok**
	//   就当生效（`config reload` 侧同样只打请求、不看路由表）⇒「写进名册 + reload 报 ok，
	//   主控路由表仍不认该模型（孵卵接口回 404 未知模型）」正是本账症状。
	// 改后（Mr2109 拍）：写后**即**打 `POST /api/config/reload`，再用**只读面**
	//   `GET /api/fleet/models` **现读校验**路由表确已认这一条（模型名 × 主机）；不通过 ⇒
	//   **回滚名册** + rc≠0 并把「谁没认」点名（不再单一信任 reload 的 ok）。
	// ★ 只在写的是**主控现读的那份名册**（默认 `<仓根>/gateway/fleet.yaml`）时联动；`--path` /
	//   `--root` 指的别件 ⇒ 主控读的不是它，联动无意义（跳过并如实说明）。
	if rc := linkRosterToCore(inv, path, name, s.Host, s.File, raw, newText, stderr); rc != exitOK {
		return rc
	}
	row := map[string]string{"model": name, "host": s.Host, "file": s.File,
		"fleet": path, "line": strconv.Itoa(pl.Entry), "added": s.Added}
	fmt.Fprintf(stderr, "%s: 名册已写（`%s:` 块 · 新条第 %d 行 · 只此一件）—— 生效走写后即 reload + 现读校验（GAP-20260925-42 · 本命令已办）\n", progName, name, pl.Entry)
	return listCmd(inv, stdout, stderr, []string{"model", "host", "file", "fleet", "line"}, []map[string]string{row})
}

// coreRosterPath —— 主控**现读**的那份名册件的路径（`<仓根>/gateway/fleet.yaml` —— 主控的
// `ConfigPath` 指的就是它）。仓根解析不到 ⇒ 空串（调用方据此跳过联动 —— 不猜）。
func coreRosterPath() string {
	r := repoRoot()
	if r == "" {
		return ""
	}
	return filepath.Join(r, "gateway", "fleet.yaml")
}

// linkRosterToCore —— 缺口 GAP-20260925-42 的联动：写后即 reload + **现读校验路由**；不认即回滚 + rc≠0。
//
// 退码：0 —— 生效已证（路由表现读认了它）· 或 非主控现读件（不联动）；
//
//	2 —— 主控打不到 / 只读面 `/api/fleet/models` 读不到 ⇒ **路由未校验**（名册已写是事实、
//	    **不误回滚**，但结论面是「**不给结论**」—— 房规：取不到 ⇒ rc=2；✗ 不得 rc=0＋警告）；
//	1 —— 写后 reload 被主控拒 / reload 报 ok 而**现读路由表不认** ⇒ **已回滚**名册（回到写前那份）。
func linkRosterToCore(inv *invocation, path, name, host, file, oldText, newText string, stderr io.Writer) int {
	cp := coreRosterPath()
	if cp == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不联动主控 reload / 路由校验（名册已写 · GAP-20260925-42）\n", progName)
		return exitOK
	}
	if filepath.Clean(path) != filepath.Clean(cp) {
		fmt.Fprintf(stderr, "%s: 名册件 `%s` **不是主控现读件**（`%s`）⇒ 不联动 reload / 路由校验（名册已写 · GAP-20260925-42）\n",
			progName, path, cp)
		return exitOK
	}
	c := newClient()
	// ① 写后**即** reload（不再把「生效」推给下一条命令）。
	status, respBody, err := sendJSON(c, "POST", "/api/config/reload", "")
	if err != nil {
		// 主控打不到 ≠ 路由不认：名册已写且自洽 ⇒ 不误回滚（**名册已写是事实，照实写清**）；
		// 但**结论面必须是不给结论** —— 房规：取不到 ⇒ rc=2（✗ 不得 rc=0＋警告 ＝ 静默绿；
		// 缺口 `GAP-20260925-42` 口径裁定 · 2026-10-06）。
		inv.warnf("名册已写，但主控打不到（%v）⇒ **未**校验路由是否认它（不是「不认」，是「没核」）", err)
		inv.setErr("usage", "route_unverified",
			"名册已写，但主控打不到 ⇒ 路由未校验（没核 ≠ 不认）⇒ 不给结论")
		fmt.Fprintf(stderr, "%s: 名册已写（`%s:` 块）· 主控打不到（%v）⇒ **路由未校验**（不是不认，是没核）⇒ **不给结论（退码 2）**（复核：`%s gateway models`）\n",
			progName, name, err, progName)
		return exitUsage
	}
	if status >= 400 {
		code := errorCodeOf(respBody)
		rollbackRoster(path, newText, oldText, c, stderr)
		inv.setErr("failed", orDash(code), "主控拒绝了这次热加载（名册已回滚到写前那份）")
		fmt.Fprintf(stderr, "%s: 写后 reload 被主控拒（http_%d · code=%s）⇒ **回滚名册**（回到写前那份）· rc≠0\n",
			progName, status, orDash(code))
		fmt.Fprintf(stderr, "主控原样响应: %s\n", strings.TrimSpace(respBody))
		return exitFail
	}
	// ② **现读校验**（不再单一信任 reload 的 ok）：直接读只读面 `/api/fleet/models` 核准这一条。
	var fleet struct {
		Models []struct {
			ID   string `json:"id"`
			Host string `json:"host"`
			File string `json:"file"`
		} `json:"models"`
		Count int `json:"count"`
	}
	if err := c.getJSON("/api/fleet/models", &fleet); err != nil {
		// 同 ①：读不到只读面 ⇒ 没核 ⇒ **不给结论（退码 2）**（✗ 不得 rc=0＋警告）。
		inv.warnf("reload 已 ok，但只读面 `/api/fleet/models` 读不到（%v）⇒ **未**校验路由", err)
		inv.setErr("usage", "route_unverified",
			"reload 已 ok，但只读面读不到 ⇒ 路由未校验（没核 ≠ 不认）⇒ 不给结论")
		fmt.Fprintf(stderr, "%s: reload 已 ok，但只读面 `/api/fleet/models` 读不到（%v）⇒ **路由未校验**（不是不认，是没核）⇒ **不给结论（退码 2）**（复核：`%s gateway models`）\n",
			progName, err, progName)
		return exitUsage
	}
	for _, m := range fleet.Models {
		if m.ID == name && m.Host == host && (file == "" || m.File == file) {
			fmt.Fprintf(stderr, "%s: 名册已写 · reload 已发 · **现读路由表已认**该条（`%s` @ `%s` · 只读面 `/api/fleet/models` 共 %d 条 · GAP-20260925-42）\n",
				progName, name, host, len(fleet.Models))
			return exitOK
		}
	}
	// ③ reload 报 ok 而路由表**不认** ⇒ 回滚名册（并让主控也退回写前那份）+ rc≠0。
	rollbackRoster(path, newText, oldText, c, stderr)
	inv.setErr("failed", "route_not_recognized",
		"写册 + reload 均 ok，但现读路由表不认该模型（已回滚名册）")
	fmt.Fprintf(stderr, "%s: ✗ **reload 报 ok 而现读路由表不认** `%s`（host=%s）—— 路由表共 %d 条模型、无此配对 ⇒ **回滚名册** + rc≠0\n",
		progName, name, host, len(fleet.Models))
	fmt.Fprintf(stderr, "  （这就是「不再单一信任 reload 的 ok」：真查只读面 `/api/fleet/models` 才算数；复核：`%s gateway models`）\n", progName)
	return exitFail
}

// rollbackRoster —— 把名册从 newText 原子写回 oldText，并**再 reload 一次**让主控也退回写前那份。
func rollbackRoster(path, newText, oldText string, c *client, stderr io.Writer) {
	if rc := writeFleetAtomic(path, newText, oldText, stderr); rc != exitOK {
		fmt.Fprintf(stderr, "%s: !! 回滚名册失败 —— 请人处理（件 %s）\n", progName, path)
		return
	}
	if _, _, err := sendJSON(c, "POST", "/api/config/reload", ""); err != nil {
		fmt.Fprintf(stderr, "%s: 回滚后再 reload 打不到主控（%v）—— 主控路由表可能仍含刚回滚的那条（复核：`%s gateway models`）\n",
			progName, err, progName)
	}
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
	if inv.jsonGiven {
		if rc := requireJSONFields(inv, stderr); rc != exitOK {
			return rc
		}
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
		guardMarkDecision(inv, guardDecisionApproval)
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

// ---- `model rm` / `model set`（缺口账 `GAP-20260925-31` · 名册「改一条 / 删一条」两动作面）---------
//
// 病根（缺口 `GAP-20260925-31` 逐字）：名册件 `gateway/fleet.yaml` 此前只有 `model add`（纯追加）
// 与 `model ls`（只读）—— 条目的 `file` / `verified` / `desc` 一旦写错，或要退役一条，
// 只能手搓定点改文件（改完还得再跑一次 `config reload`），且「看不到自己改对没有」。
// 本笔补的就是那两个动作面：`rm`（删一条）与 `set`（改一条）。
//
// 三条纪律与 `model add` **同一张表**（不新造）：
//
//	① 先校验后写、失败回滚：写 = 同目录临时件 + `rename`；写完读回再校；任一步不过 ⇒ 回滚 ⇒ 退 1。
//	② 只碰名册件（`gateway/fleet.yaml`，可 `--path` / `--root` 指别处）；`--dry-run` 一个字节都不写。
//	③ 形态表与 `model add` 逐字同源（两种正规形态都认：列表形 + 裸映射形；其余形态 ⇒ 不猜、不改）。
//
// 定位口径：`--model` = `models:` 段的**键/块名**；`--host` = 该条的 `host:` 字段值；
// 同块同机多条时用 `--file` 点准那一条（不给而命中多条 ⇒ 退 2「请点名 --file」，**不猜**）。

// modelEntryHit —— 定位到的「那一条」。
type modelEntryHit struct {
	Form     string // "list"（列表形块的条目行）· "bare"（裸映射单条 · 与块首同行）
	KeyIdx   int    // 块首（`  <名>:`）那一行的下标
	EntryIdx int    // 条目行的下标（bare 形 = KeyIdx）
	Body     string // 条目正文（flow 形 `{ … }` 逐字）
	Remain   int    // 本条之外，本块还剩几条
}

// modelFlowSplitTop —— 顶层逗号切分（引号内与 `[]`/`{}` 内的逗号不算分隔符）。
// 名册条里 `file:` 路径可能含 `:`，`cmd:` 是列表（含逗号）—— 两者都要保住。
func modelFlowSplitTop(s string) []string {
	out := []string{}
	var cur strings.Builder
	depth, inq, esc := 0, false, false
	for _, r := range s {
		if inq {
			cur.WriteRune(r)
			if esc {
				esc = false
			} else if r == '\\' {
				esc = true
			} else if r == '"' {
				inq = false
			}
			continue
		}
		switch r {
		case '"':
			inq = true
			cur.WriteRune(r)
		case '[', '{':
			depth++
			cur.WriteRune(r)
		case ']', '}':
			depth--
			cur.WriteRune(r)
		case ',':
			if depth == 0 {
				out = append(out, cur.String())
				cur.Reset()
			} else {
				cur.WriteRune(r)
			}
		default:
			cur.WriteRune(r)
		}
	}
	out = append(out, cur.String())
	return out
}

// modelFlowPairs —— 把 flow 形正文 `{ k: v, … }` 拆成**保序**的 (键, 值原样) 对。
func modelFlowPairs(body string) ([][2]string, bool) {
	s := strings.TrimSpace(body)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return [][2]string{}, true
	}
	pairs := [][2]string{}
	for _, p := range modelFlowSplitTop(inner) {
		k, v, ok := strings.Cut(p, ":")
		if !ok {
			return nil, false
		}
		pairs = append(pairs, [2]string{strings.TrimSpace(k), strings.TrimSpace(v)})
	}
	return pairs, true
}

// modelFlowField —— 取一个标量字段的值（去掉包裹的引号）；取不到返 false。
func modelFlowField(body, key string) (string, bool) {
	pairs, ok := modelFlowPairs(body)
	if !ok {
		return "", false
	}
	for _, p := range pairs {
		if p[0] == key {
			return strings.Trim(p[1], `"`), true
		}
	}
	return "", false
}

// modelBlocksEnd —— `models:` 段的结束下标（下一个顶级键之前）。
func modelBlocksEnd(lines []string, modelsAt int) int {
	for i := modelsAt + 1; i < len(lines); i++ {
		l := lines[i]
		if l == "" || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "#") {
			continue
		}
		return i
	}
	return len(lines)
}

// modelEntryLocate —— 按块名（`--model`）+ `host:`（可选 `file:`）定位那一条。
// 命中 0 条 / 命中多条而没点名 `--file` / 认不出的形态 ⇒ 返错文本（调用方判用法错，**不猜**）。
func modelEntryLocate(lines []string, name, host, file string) (modelEntryHit, string) {
	h := modelEntryHit{KeyIdx: -1, EntryIdx: -1}
	modelsAt := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "models:") {
			modelsAt = i
			break
		}
	}
	if modelsAt < 0 {
		return h, "名册件里没有 `models:` 段"
	}
	secEnd := modelBlocksEnd(lines, modelsAt)
	key := "  " + name + ":"
	for i := modelsAt + 1; i < secEnd; i++ {
		l := lines[i]
		if !strings.HasPrefix(l, key) {
			continue
		}
		rest := strings.TrimSpace(l[len(key):])
		if rest != "" { // ── 裸映射形（单条 · 与块首同行）──
			if !strings.HasPrefix(rest, "{") || !strings.HasSuffix(rest, "}") {
				return h, fmt.Sprintf("`%s:` 这一块既不是列表形，也不是同行裸映射形（同行接的是 %q）⇒ 本命令不猜", name, snipHead(rest, 24))
			}
			eh, ok := modelFlowField(rest, "host")
			if !ok {
				return h, fmt.Sprintf("`%s:` 裸映射条里没有 `host:`（认不出属于哪台机）⇒ 不猜", name)
			}
			ef, _ := modelFlowField(rest, "file")
			if eh != host || (file != "" && ef != file) {
				return h, fmt.Sprintf("`%s:` 块（裸映射单条）里没有 `host: %s`%s 的条 ⇒ 无可办", name, host, fileClause(file))
			}
			h.Form, h.KeyIdx, h.EntryIdx, h.Body, h.Remain = "bare", i, i, rest, 0
			return h, ""
		}
		// ── 列表形块 ──
		entries := []int{}
		for j := i + 1; j < secEnd; j++ {
			lj := lines[j]
			if strings.TrimSpace(lj) == "" {
				break
			}
			if len(lj)-len(strings.TrimLeft(lj, " ")) <= 2 {
				break
			}
			if strings.HasPrefix(strings.TrimLeft(lj, " \t"), "#") {
				continue
			}
			if strings.HasPrefix(strings.TrimLeft(lj, " \t"), "-") {
				entries = append(entries, j)
			}
		}
		hits := []modelEntryHit{}
		for _, j := range entries {
			b := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[j]), "-"))
			if !strings.HasPrefix(b, "{") || !strings.HasSuffix(b, "}") {
				return h, fmt.Sprintf("`%s:` 块里第 %d 行是**多行条目**（`- ` 后不是同行 `{ … }`）⇒ 本命令不猜", name, j+1)
			}
			eh, ok := modelFlowField(b, "host")
			if !ok {
				return h, fmt.Sprintf("`%s:` 块里第 %d 行没有 `host:` ⇒ 认不出属于哪台机（不猜）", name, j+1)
			}
			ef, _ := modelFlowField(b, "file")
			if eh == host && (file == "" || ef == file) {
				hits = append(hits, modelEntryHit{Form: "list", KeyIdx: i, EntryIdx: j, Body: b, Remain: len(entries) - 1})
			}
		}
		switch {
		case len(hits) == 0:
			return h, fmt.Sprintf("`%s:` 块里没有 `host: %s`%s 的条 ⇒ 无可办（不猜 · 不误碰同块的别的条）", name, host, fileClause(file))
		case len(hits) > 1:
			return h, fmt.Sprintf("`%s:` 块里 `host: %s` 命中 %d 条 ⇒ 请用 `--file <GGUF>` 点准那一条（**不猜**）", name, host, len(hits))
		}
		return hits[0], ""
	}
	return h, fmt.Sprintf("名册件里没有 `%s:` 这一块", name)
}

// fileClause —— 报文本里点出 `--file` 那一格（给了才写）。
func fileClause(file string) string {
	if file == "" {
		return ""
	}
	return " / `file: " + file + "`"
}

// modelEntryRewriteLine —— 把条目行里的 flow 正文替换成 newBody（保留行首缩进 / `- ` / `键:`）。
func modelEntryRewriteLine(line, newBody string) string {
	if i := strings.Index(line, "{"); i >= 0 {
		return line[:i] + newBody
	}
	return line
}

// modelEntryRender —— 把保序的 (键, 值原样) 对重新拼成 flow 正文 `{ k: v, … }`。
func modelEntryRender(pairs [][2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, p[0]+": "+p[1])
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

// modelSetKind —— 字段的取值域（**闭集** · 逐字对齐 `config.ModelCandidate` 的 yaml tag）。
// 不在表里 ⇒ 退 2（不收无名键 —— 免得把名册写成主控解析器不认的样子）。
func modelSetKind(key string) string {
	switch key {
	case "verified", "ssd", "thinking", "tool_support", "engine_impl_required":
		return "bool"
	case "verified_state":
		// GAP-20261003-21（设计稿 §三乙案）：verified 三态闭集 true|false|reported ——
		// 旧 bool 键 `verified` 不动（零破坏）；第三态 `reported` 走这枚**加性**键。
		return "state"
	case "size_bytes":
		// 字节数锚（正整数）—— 名册自带、由 check-model-size-anchor 只 stat 对拍。
		return "int"
	case "mem_gb", "ctx_window", "max_tokens":
		return "num"
	case "name", "family", "host", "backend", "file", "arch", "architecture",
		"mmproj", "moe", "template", "modality", "description", "added", "verified_where":
		return "str"
	case "cmd", "env":
		return "list" // 列表值 ⇒ 本命令不收（要改它请整行手改 · 不猜）
	}
	return ""
}

// modelSetValueText —— 按字段取值域把 `--set` 的值渲染成 YAML 字面（返回渲染文本 + 是否合法）。
func modelSetValueText(key, v string) (string, bool) {
	switch modelSetKind(key) {
	case "bool":
		if v != "true" && v != "false" {
			return "", false
		}
		return v, true
	case "state":
		// GAP-20261003-21：三态闭集 —— true | false | reported（其它一律拒 · 值域不合即退 2）。
		if v != "true" && v != "false" && v != "reported" {
			return "", false
		}
		return v, true
	case "num":
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "", false
		}
		return v, true
	case "int":
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return "", false
		}
		return strconv.FormatInt(n, 10), true
	case "str":
		return yamlStr(v), true
	}
	return "", false
}

// cmdModelRm —— `zerg model rm`：从名册件删一条（先校验后写 · 失败回滚 · 删空连块首行一起删）。
func cmdModelRm(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.jsonGiven {
		if rc := requireJSONFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	name := strings.TrimSpace(inv.flagVal("--model"))
	if name == "" {
		name = strings.TrimSpace(inv.modelWant)
	}
	host := strings.TrimSpace(inv.flagVal("--host"))
	file := strings.TrimSpace(inv.flagVal("--file"))
	bad := func(detail, msg string) int {
		inv.setErr("usage", detail, msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "  用法：zerg model rm --model <模型名> --host <主机> [--file <GGUF 路径>] --confirm <模型名> [--dry-run | --yes]\n")
		fmt.Fprintf(stderr, "        （`--model` = `models:` 段的键/块名；`--host` = 该条 `host:`；同块同机多条时用 `--file` 点准再删）\n")
		return exitUsage
	}
	if name == "" || host == "" {
		miss := []string{}
		if name == "" {
			miss = append(miss, "--model")
		}
		if host == "" {
			miss = append(miss, "--host")
		}
		return bad("missing_target", "缺 "+strings.Join(miss, " / ")+"（这两枚必给）")
	}
	if !identish(name) {
		return bad("bad_value", "`--model` 要当 `models:` 段的键（块名）⇒ 只收标识符")
	}
	if !identish(host) {
		return bad("bad_value", "`--host` 只收标识符")
	}
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
		inv.setErr("failed", "fleet_invalid", "名册件现档就解析不过（先修它，别在坏档上删）")
		fmt.Fprintf(stderr, "%s: %s 现档就解析不过：%v ⇒ 判红（退码 1 · **不写**）\n", progName, path, err)
		return exitFail
	}
	before := len(fc.Models[name])
	if before == 0 {
		return bad("model_block_absent", fmt.Sprintf("名册件里 `models:` 段没有 `%s:` 这一块 ⇒ 无可删", name))
	}
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	hit, ierr := modelEntryLocate(lines, name, host, file)
	if ierr != "" {
		return bad("entry_not_found", ierr)
	}
	dropKey := hit.Form == "bare" || hit.Remain == 0
	newLines := make([]string, 0, len(lines))
	for i, l := range lines {
		if i == hit.EntryIdx {
			continue
		}
		if dropKey && i == hit.KeyIdx {
			continue
		}
		newLines = append(newLines, l)
	}
	nl := "\n"
	if !strings.HasSuffix(raw, "\n") {
		nl = ""
	}
	newText := strings.Join(newLines, "\n") + nl
	recheck, err := config.ParseFleetConfig([]byte(newText))
	if err != nil {
		inv.setErr("failed", "post_write_invalid", "改后文本解析不过（回滚档 · 不写）")
		fmt.Fprintf(stderr, "%s: 改后文本解析不过（%v）⇒ 判红（退码 1 · **不写**）\n", progName, err)
		return exitFail
	}
	after := len(recheck.Models[name])
	if after != before-1 {
		inv.setErr("failed", "post_write_mismatch", "改后条数不是 -1（回滚档 · 不写）")
		fmt.Fprintf(stderr, "%s: `%s` 块改后条数 %d → %d（不是 -1）⇒ 判红（退码 1 · **不写**）\n", progName, name, before, after)
		return exitFail
	}
	if inv.dryRun {
		keyLineNote := fmt.Sprintf("第 %d 行（保留 —— 本块还剩 %d 条）", hit.KeyIdx+1, hit.Remain)
		if dropKey {
			keyLineNote = fmt.Sprintf("第 %d 行（**本块删空 ⇒ 连块首行一起删**）", hit.KeyIdx+1)
		}
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : model rm（%s）\n", progName)
		fmt.Fprintf(stdout, "  名册件   : %s\n", path)
		fmt.Fprintf(stdout, "  块       : `models:` 的 `%s:` · 删这一条：host=%s%s\n", name, host, fileClause(file))
		fmt.Fprintf(stdout, "  删的行   : 第 %d 行 —— %s\n", hit.EntryIdx+1, hit.Body)
		fmt.Fprintf(stdout, "  块首行   : %s\n", keyLineNote)
		fmt.Fprintf(stdout, "  校验     : ✔ 现档解析过（`%s` 下 %d 条）· ✔ 改后解析过（%d 条）\n", name, before, after)
		fmt.Fprintf(stdout, "  授权判据 : 真写要 `--yes`（D2 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（**一个字节都不写**）\n")
		fmt.Fprintf(stdout, "  生效     : 要主控读它 ⇒ `%s config reload --yes`（本命令**不碰主控**）\n", progName)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !inv.confirmGiven {
		inv.setErr("usage", "confirm_required", "删名册条要 `--confirm=<模型名>` —— 缺确认 ⇒ 不执行（fail-closed）")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: `model rm` 会改名册件 ⇒ 缺 `--confirm=%s` ⇒ 不执行（fail-closed）\n", progName, name)
		return exitUsage
	}
	if inv.confirm != name {
		inv.setErr("usage", "confirm_mismatch", "`--confirm` 的值要与 `--model` 逐字相同")
		guardMarkDecision(inv, guardDecisionBlock)
		fmt.Fprintf(stderr, "%s: `--confirm=%s` 与 `--model %s` 不一致 ⇒ 不执行（给错值 = 拒绝，不是「当没给」）\n", progName, inv.confirm, name)
		return exitUsage
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: 改名册是 D2 档 —— 缺 --yes ⇒ 不执行（先看计划件：加 `--dry-run`）\n", progName)
		return exitUsage
	}
	if rc := writeFleetAtomic(path, raw, newText, stderr); rc != exitOK {
		inv.setErr("failed", "write_failed", "写名册件失败（已回滚）")
		return rc
	}
	row := map[string]string{"model": name, "host": host, "fleet": path,
		"line": strconv.Itoa(hit.EntryIdx + 1), "block_removed": strconv.FormatBool(dropKey),
		"remaining": strconv.Itoa(after)}
	fmt.Fprintf(stderr, "%s: 名册已删（`%s:` 块 · 删第 %d 行 · 块内剩 %d 条）—— 要主控读它：`%s config reload --yes`\n",
		progName, name, hit.EntryIdx+1, after, progName)
	return listCmd(inv, stdout, stderr, []string{"model", "host", "fleet", "line", "block_removed", "remaining"}, []map[string]string{row})
}

// cmdModelSet —— `zerg model set`：改名册里某一条的字段（`--set k=v` · 保序原位改 · 先校验后写）。
//
// 只改**点准的那一条**（`--model` + `--host` + 可选 `--file`）；同条里其余字段**逐字保留**（保序重拼）。
// 字段名走闭集（`modelSetKind`）；不在闭集 / 值域不合 / 是列表值 ⇒ 退 2（不猜、不写）。
func cmdModelSet(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.jsonGiven {
		if rc := requireJSONFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	name := strings.TrimSpace(inv.flagVal("--model"))
	if name == "" {
		name = strings.TrimSpace(inv.modelWant)
	}
	host := strings.TrimSpace(inv.flagVal("--host"))
	file := strings.TrimSpace(inv.flagVal("--file"))
	bad := func(detail, msg string) int {
		inv.setErr("usage", detail, msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "  用法：zerg model set --model <模型名> --host <主机> [--file <GGUF 路径>] --set k=v [--set k=v …] --confirm <模型名> [--dry-run | --yes]\n")
		fmt.Fprintf(stderr, "        （只改点准的那一条 · 其余字段逐字保留；可改字段：host backend file mem_gb size_bytes ctx_window max_tokens architecture arch template modality verified verified_state verified_where thinking tool_support ssd mmproj moe description added name family）\n")
		return exitUsage
	}
	if name == "" || host == "" {
		miss := []string{}
		if name == "" {
			miss = append(miss, "--model")
		}
		if host == "" {
			miss = append(miss, "--host")
		}
		return bad("missing_target", "缺 "+strings.Join(miss, " / ")+"（这两枚必给）")
	}
	if !identish(name) || !identish(host) {
		return bad("bad_value", "`--model` / `--host` 只收标识符")
	}
	if len(inv.setPairs) == 0 {
		return bad("no_params", "一个 `--set` 都没给（要改什么就点它出来）")
	}
	// 先把 `--set` 全部校验成 (键, 渲染文本) 的保序表（**一件都没写之前处处 fail-closed**）。
	sets := [][2]string{}
	for _, kv := range inv.setPairs {
		k, v, ok := strings.Cut(kv, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			return bad("bad_set_pair", fmt.Sprintf("`--set` 要 `键=值` 形态（给的是 %q）", kv))
		}
		txt, ok2 := modelSetValueText(k, v)
		if !ok2 {
			kind := modelSetKind(k)
			switch kind {
			case "":
				return bad("bad_field", fmt.Sprintf("`--set %s=` 不是可改字段（闭集见用法行）—— 不收无名键", k))
			case "list":
				return bad("bad_field", fmt.Sprintf("`--set %s=` 是**列表值**（`cmd` / `env`）—— 本命令不收，请整行手改（不猜）", k))
			case "bool":
				return bad("bad_value", fmt.Sprintf("`--set %s=` 要 true / false（给的是 %q）", k, v))
			case "state":
				return bad("bad_value", fmt.Sprintf("`--set %s=` 要 true / false / reported（给的是 %q）", k, v))
			case "int":
				return bad("bad_value", fmt.Sprintf("`--set %s=` 要正整数（字节数 · 给的是 %q）", k, v))
			default:
				return bad("bad_value", fmt.Sprintf("`--set %s=` 要数值（给的是 %q）", k, v))
			}
		}
		sets = append(sets, [2]string{k, txt})
	}
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
		inv.setErr("failed", "fleet_invalid", "名册件现档就解析不过（先修它，别在坏档上改）")
		fmt.Fprintf(stderr, "%s: %s 现档就解析不过：%v ⇒ 判红（退码 1 · **不写**）\n", progName, path, err)
		return exitFail
	}
	if len(fc.Models[name]) == 0 {
		return bad("model_block_absent", fmt.Sprintf("名册件里 `models:` 段没有 `%s:` 这一块 ⇒ 无可改", name))
	}
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	hit, ierr := modelEntryLocate(lines, name, host, file)
	if ierr != "" {
		return bad("entry_not_found", ierr)
	}
	pairs, ok := modelFlowPairs(hit.Body)
	if !ok {
		return bad("entry_shape_unknown", "点准的那一条不是同行 `{ … }` flow 形 ⇒ 本命令不猜（改它要动多行）")
	}
	// 保序原位改：命中的键改值；没命中的键**追加**在尾。
	changed := []string{}
	for _, s := range sets {
		found := false
		for i := range pairs {
			if pairs[i][0] == s[0] {
				pairs[i][1] = s[1]
				found = true
				break
			}
		}
		if !found {
			pairs = append(pairs, [2]string{s[0], s[1]})
		}
		changed = append(changed, s[0])
	}
	newBody := modelEntryRender(pairs)
	newLine := modelEntryRewriteLine(lines[hit.EntryIdx], newBody)
	newLines := append([]string{}, lines...)
	newLines[hit.EntryIdx] = newLine
	nl := "\n"
	if !strings.HasSuffix(raw, "\n") {
		nl = ""
	}
	newText := strings.Join(newLines, "\n") + nl
	recheck, err := config.ParseFleetConfig([]byte(newText))
	if err != nil {
		inv.setErr("failed", "post_write_invalid", "改后文本解析不过（回滚档 · 不写）")
		fmt.Fprintf(stderr, "%s: 改后文本解析不过（%v）⇒ 判红（退码 1 · **不写**）\n", progName, err)
		return exitFail
	}
	if len(recheck.Models[name]) != len(fc.Models[name]) {
		inv.setErr("failed", "post_write_mismatch", "改后条数变了（回滚档 · 不写）")
		fmt.Fprintf(stderr, "%s: `%s` 块条数 %d → %d（**改字段不该动条数**）⇒ 判红（退码 1 · **不写**）\n", progName, name, len(fc.Models[name]), len(recheck.Models[name]))
		return exitFail
	}
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : model set（%s）\n", progName)
		fmt.Fprintf(stdout, "  名册件   : %s\n", path)
		fmt.Fprintf(stdout, "  块       : `models:` 的 `%s:` · 改这一条：host=%s%s\n", name, host, fileClause(file))
		fmt.Fprintf(stdout, "  改的行   : 第 %d 行\n", hit.EntryIdx+1)
		fmt.Fprintf(stdout, "  改前     : %s\n", hit.Body)
		fmt.Fprintf(stdout, "  改后     : %s\n", newBody)
		fmt.Fprintf(stdout, "  改的字段 : %s\n", strings.Join(changed, " / "))
		fmt.Fprintf(stdout, "  校验     : ✔ 现档解析过（`%s` 下 %d 条）· ✔ 改后解析过（同条数）\n", name, len(fc.Models[name]))
		fmt.Fprintf(stdout, "  授权判据 : 真写要 `--yes`（D2 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（**一个字节都不写**）\n")
		fmt.Fprintf(stdout, "  生效     : 要主控读它 ⇒ `%s config reload --yes`（本命令**不碰主控**）\n", progName)
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !inv.confirmGiven {
		inv.setErr("usage", "confirm_required", "改名册条要 `--confirm=<模型名>` —— 缺确认 ⇒ 不执行（fail-closed）")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: `model set` 会改名册件 ⇒ 缺 `--confirm=%s` ⇒ 不执行（fail-closed）\n", progName, name)
		return exitUsage
	}
	if inv.confirm != name {
		inv.setErr("usage", "confirm_mismatch", "`--confirm` 的值要与 `--model` 逐字相同")
		guardMarkDecision(inv, guardDecisionBlock)
		fmt.Fprintf(stderr, "%s: `--confirm=%s` 与 `--model %s` 不一致 ⇒ 不执行（给错值 = 拒绝，不是「当没给」）\n", progName, inv.confirm, name)
		return exitUsage
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: 改名册是 D2 档 —— 缺 --yes ⇒ 不执行（先看计划件：加 `--dry-run`）\n", progName)
		return exitUsage
	}
	if rc := writeFleetAtomic(path, raw, newText, stderr); rc != exitOK {
		inv.setErr("failed", "write_failed", "写名册件失败（已回滚）")
		return rc
	}
	row := map[string]string{"model": name, "host": host, "fleet": path,
		"line": strconv.Itoa(hit.EntryIdx + 1), "changed": strings.Join(changed, " / "), "entry": newBody}
	fmt.Fprintf(stderr, "%s: 名册已改（`%s:` 块 · 第 %d 行 · 改了 %s）—— 要主控读它：`%s config reload --yes`\n",
		progName, name, hit.EntryIdx+1, strings.Join(changed, " / "), progName)
	return listCmd(inv, stdout, stderr, []string{"model", "host", "fleet", "line", "changed", "entry"}, []map[string]string{row})
}

// ---- `context set-port`（缺口账 `GAP-20260925-32` · 机器名册 `nodes:` 的第一条改面）------------------------
//
// 病根（缺口 `GAP-20260925-32` 逐字）：机器登记件 `~/.zerg/contexts/<档>.yaml` 的 `nodes:`（host/port/os/kind
// 的**唯一真源**）此前**只有读面**（`zerg context ls` 只出档位名册、连 nodes 都不列）⇒ 当某台机被迫换端口
// （实测 <某台机> 的 8100 被无关服务占着、子端挪到 8101）时只能手搓改 YAML，且「看不出主控到底认没认这次改动」。
//
// 本笔只补**第一条改面** `set-port`（一次改一台机的 `port:`），口径照同族写面 `model add` / `model rm`
// 的成熟做法：**先校验后写 · 同目录临时件 + rename（原子）· 写完读回逐字节再校 · 任一步不过 ⇒ 回滚**。
//
// 只改**现成的** `port:` 行（✗ 不新建字段、✗ 不带 `context add`/`rm` —— 那不属本笔，见账内 want_action）。

// contextYAMLPath —— 解析档位件路径（`~/.zerg/contexts/<档>.yaml`；`--context` 缺省 `default`）。
// 档名只收标识符 —— 挡住 `/` `..` 一类把路径带出 `contexts/` 的取值。
func contextYAMLPath(inv *invocation) (string, string) {
	name := strings.TrimSpace(inv.contextWant)
	if name == "" {
		name = "default"
	}
	if !identish(name) {
		return "", fmt.Sprintf("档名 %q 只收标识符（挡路径穿越）", name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "取不到家目录：" + err.Error()
	}
	return filepath.Join(home, ".zerg", "contexts", name+".yaml"), ""
}

// writeContextAtomic —— 原子写 + 读回逐字节校验 + 失败回滚（与 `writeFleetAtomic` 同形，
// 只是档位件不走 `config.LoadFleetConfig` —— 它是**另一份**格式，硬套会把好档判坏）。
func writeContextAtomic(path, oldText, newText string, stderr io.Writer) int {
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
	back, err := os.ReadFile(path)
	if err != nil || string(back) != newText {
		fmt.Fprintf(stderr, "%s: 读回校验不过（err=%v）⇒ **回滚**（把原文写回）\n", progName, err)
		if werr := os.WriteFile(path, []byte(oldText), fi.Mode().Perm()); werr != nil {
			fmt.Fprintf(stderr, "%s: !! 回滚也失败：%v —— 请人处理（原档长度 %d 字节）\n", progName, werr, len(oldText))
			return exitFail
		}
		return exitFail
	}
	return exitOK
}

// cmdContextSetPort —— `zerg context set-port`：改机器名册里某台机的 `port:`（先校验后写 · 失败回滚）。
func cmdContextSetPort(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.jsonGiven {
		if rc := requireJSONFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	bad := func(detail, msg string) int {
		inv.setErr("usage", detail, msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "  用法：zerg context set-port --node <机器名> --port <端口> [--context <档名>] --confirm=<机器名> [--dry-run | --yes]\n")
		fmt.Fprintf(stderr, "        （改的是机器名册真源 `~/.zerg/contexts/<档>.yaml` 里该节点**现成的** `port:` 行 —— host/port/os/kind 的唯一写入口；`--context` 缺省 default）\n")
		return exitUsage
	}
	if len(inv.nodes) == 0 {
		return bad("missing_target", "缺 `--node <机器名>`（改哪台机的端口）")
	}
	if len(inv.nodes) > 1 {
		return bad("bad_value", "`--node` 只收一个机器名（本动作一次改一台）")
	}
	node := strings.TrimSpace(inv.nodes[0])
	portTxt := strings.TrimSpace(inv.flagVal("--port"))
	if node == "" || portTxt == "" {
		miss := []string{}
		if node == "" {
			miss = append(miss, "--node")
		}
		if portTxt == "" {
			miss = append(miss, "--port")
		}
		return bad("missing_target", "缺 "+strings.Join(miss, " / ")+"（这两枚必给）")
	}
	if !identish(node) {
		return bad("bad_value", "`--node` 只收标识符（机器名是稳定 id，不许改）")
	}
	port, err := strconv.Atoi(portTxt)
	if err != nil || port < 1 || port > 65535 {
		return bad("bad_value", fmt.Sprintf("`--port` 要 1..65535 的整数（给的是 %q）", portTxt))
	}
	path, perr := contextYAMLPath(inv)
	if perr != "" {
		inv.setErr("blocked", "context_path_bad", perr)
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 8）\n", progName, perr)
		return exitBlocked
	}
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		inv.setErr("blocked", "context_unreadable", "档位件读不到")
		fmt.Fprintf(stderr, "%s: 档位件读不到（%s）：%v ⇒ 不给结论（退码 8）\n", progName, path, err)
		return exitBlocked
	}
	raw := string(rawBytes)
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	nodesAt := -1
	for i, l := range lines {
		if l == "nodes:" {
			nodesAt = i
			break
		}
	}
	if nodesAt < 0 {
		return bad("nodes_section_absent", "档位件里没有 `nodes:` 段")
	}
	secEnd := len(lines)
	for i := nodesAt + 1; i < len(lines); i++ {
		l := lines[i]
		if l == "" || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "#") {
			continue
		}
		secEnd = i
		break
	}
	key := "  " + node + ":"
	keyIdx := -1
	for i := nodesAt + 1; i < secEnd; i++ {
		if lines[i] == key {
			keyIdx = i
			break
		}
	}
	if keyIdx < 0 {
		return bad("node_absent", fmt.Sprintf("`nodes:` 段里没有 `%s:`（机器名是稳定 id ⇒ 只有已登记的名字可改；新登记一台另立项）", node))
	}
	portIdx := -1
	for j := keyIdx + 1; j < secEnd; j++ {
		l := lines[j]
		if strings.TrimSpace(l) == "" || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		if len(l)-len(strings.TrimLeft(l, " ")) <= 2 {
			break // 回到节点那一层 ⇒ 本节点到此
		}
		if strings.HasPrefix(strings.TrimSpace(l), "port:") {
			portIdx = j
			break
		}
	}
	if portIdx < 0 {
		return bad("port_line_absent", fmt.Sprintf("`%s:` 节点里没有 `port:` 行（本命令只改**现成的** port 行 —— 不给名册加它没有的键）", node))
	}
	oldLine := lines[portIdx]
	newLine := "    port: " + strconv.Itoa(port)
	if oldLine == newLine {
		row := map[string]string{"node": node, "port": strconv.Itoa(port), "context": path, "changed": "false"}
		fmt.Fprintf(stderr, "%s: `%s` 的 `port` 已经是 %d ⇒ **无变化**（未写）\n", progName, node, port)
		return listCmd(inv, stdout, stderr, []string{"node", "port", "context", "changed"}, []map[string]string{row})
	}
	newLines := append([]string{}, lines...)
	newLines[portIdx] = newLine
	nl := "\n"
	if !strings.HasSuffix(raw, "\n") {
		nl = ""
	}
	newText := strings.Join(newLines, "\n") + nl
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : context set-port（%s）\n", progName)
		fmt.Fprintf(stdout, "  档位件   : %s（机器名册真源 `nodes:`）\n", path)
		fmt.Fprintf(stdout, "  节点     : `%s:`（第 %d 行）\n", node, keyIdx+1)
		fmt.Fprintf(stdout, "  改的行   : 第 %d 行\n", portIdx+1)
		fmt.Fprintf(stdout, "  改前     : %s\n", oldLine)
		fmt.Fprintf(stdout, "  改后     : %s\n", newLine)
		fmt.Fprintf(stdout, "  它会动   : 只此一件（%s）—— 写 = 同目录临时件 + rename（原子）；写完读回逐字节再校，任一步不过 ⇒ 回滚\n", path)
		fmt.Fprintf(stdout, "  授权判据 : 真写要 `--confirm=%s` + `--yes`（D2 档）—— 干跑与真跑**同一张表**：缺它 ⇒ 退码 2（**一个字节都不写**）\n", node)
		fmt.Fprintf(stdout, "  生效     : 主控是否认这次改动要另证（本命令**不碰主控**）；名册是声明面，可行与否由逐机探测回答\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !inv.confirmGiven {
		inv.setErr("usage", "confirm_required", "改机器名册要 `--confirm=<机器名>` —— 缺确认 ⇒ 不执行（fail-closed）")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: `context set-port` 会改档位件 ⇒ 缺 `--confirm=%s` ⇒ 不执行（fail-closed）\n", progName, node)
		return exitUsage
	}
	if inv.confirm != node {
		inv.setErr("usage", "confirm_mismatch", "`--confirm` 的值要与 `--node` 逐字相同")
		guardMarkDecision(inv, guardDecisionBlock)
		fmt.Fprintf(stderr, "%s: `--confirm=%s` 与 `--node %s` 不一致 ⇒ 不执行（给错值 = 拒绝，不是「当没给」）\n", progName, inv.confirm, node)
		return exitUsage
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		guardMarkDecision(inv, guardDecisionApproval)
		fmt.Fprintf(stderr, "%s: 改机器名册是 D2 档 —— 缺 --yes ⇒ 不执行（先看计划件：加 `--dry-run`）\n", progName)
		return exitUsage
	}
	if rc := writeContextAtomic(path, raw, newText, stderr); rc != exitOK {
		inv.setErr("failed", "write_failed", "写档位件失败（已回滚）")
		return rc
	}
	row := map[string]string{"node": node, "port": strconv.Itoa(port), "context": path,
		"line": strconv.Itoa(portIdx + 1), "changed": "true"}
	fmt.Fprintf(stderr, "%s: 机器名册已改（`%s:` 的 `port` → %d · 第 %d 行 · 只此一件）\n", progName, node, port, portIdx+1)
	return listCmd(inv, stdout, stderr, []string{"node", "port", "context", "line", "changed"}, []map[string]string{row})
}
