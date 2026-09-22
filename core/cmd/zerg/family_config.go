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
//	② **不覆盖**：同 host 下已有同名 / 同 file ⇒ 退码 2（fail-closed · 不替人改已有的条）。
//	③ **只碰名册件**：默认 `gateway/fleet.yaml`（可用 `--root` / `--path` 指到别处）；
//	   `--dry-run` 下**一个字节都不写**（判据件现场对拍 sha256）。
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
	"gopkg.in/yaml.v3"
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
func loadFleet(path string) (*config.FleetConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fc config.FleetConfig
	if err := yaml.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("YAML 解析不过：%v", err)
	}
	return &fc, nil
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

// modelAddInsertAt —— 找插入点：`models:` 下 `  <host>:` 那一块的**末行之后**。
// 返回 (行下标, 该 host 键的行号 1-based, 报错文本)。
func modelAddInsertAt(lines []string, host string) (int, int, string) {
	modelsAt := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "models:") {
			modelsAt = i
			break
		}
	}
	if modelsAt < 0 {
		return 0, 0, "名册件里没有 `models:` 段"
	}
	key := "  " + host + ":"
	hostAt := -1
	for i := modelsAt + 1; i < len(lines); i++ {
		l := lines[i]
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "#") {
			break // 出了 models 段
		}
		if l == key {
			hostAt = i
			break
		}
	}
	if hostAt < 0 {
		return 0, 0, fmt.Sprintf("`models:` 段里没有 `%s:` 这一块（先在册：新开一块要人签）", host)
	}
	end := hostAt + 1
	for end < len(lines) {
		l := lines[end]
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
			end++
			continue
		}
		if len(l)-len(strings.TrimLeft(l, " ")) <= 2 { // 回到 host 键那一层 ⇒ 本块结束
			break
		}
		end++
	}
	return end, hostAt + 1, ""
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
		fmt.Fprintf(stderr, "  用法：zerg model add --host <主机> --model <名> --file <GGUF 路径> [--backend …] [--mem-gb …] [--ctx …] [--arch …] [--desc …] [--mmproj …] [--added 日期] [--verified] [--dry-run | --yes]\n")
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
	if name != "" { // 名字进 description 之外不进 YAML（现册条没有 name 键时靠 host+file 认）
		_ = name
	}
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
	if _, ok := fc.Models[s.Host]; !ok {
		hosts := []string{}
		for h := range fc.Models {
			hosts = append(hosts, h)
		}
		sort.Strings(hosts)
		inv.setErr("usage", "host_not_declared", "该主机不在 `models:` 段里（新开一块要人签）")
		fmt.Fprintf(stderr, "%s: `models:` 段里没有 `%s` ⇒ 判用法错（退码 2）· 在册主机：%s\n", progName, s.Host, strings.Join(hosts, " / "))
		return exitUsage
	}
	for _, c := range fc.Models[s.Host] {
		if c.File == s.File {
			inv.setErr("usage", "duplicate_entry", "该 host 下已有同一 `file` 的条（不覆盖）")
			fmt.Fprintf(stderr, "%s: `%s` 下已有 `file: %s` ⇒ 拒（fail-closed · **不覆盖**别人的条）\n", progName, s.Host, s.File)
			return exitUsage
		}
		if c.Name != "" && c.Name == name {
			inv.setErr("usage", "duplicate_name", "该 host 下已有同名条（不覆盖）")
			fmt.Fprintf(stderr, "%s: `%s` 下已有 `name: %s` ⇒ 拒（不覆盖）\n", progName, s.Host, name)
			return exitUsage
		}
	}
	// ── 构造新文本 + 双解析校验（原档 → 新档）──
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	at, hostLine, ierr := modelAddInsertAt(lines, s.Host)
	if ierr != "" {
		inv.setErr("usage", "host_block_absent", ierr)
		fmt.Fprintf(stderr, "%s: %s\n", progName, ierr)
		return exitUsage
	}
	newLine := modelAddLine(4, s)
	newLines := append([]string{}, lines[:at]...)
	newLines = append(newLines, newLine)
	newLines = append(newLines, lines[at:]...)
	newText := strings.Join(newLines, "\n") + "\n"
	var recheck config.FleetConfig
	if err := yaml.Unmarshal([]byte(newText), &recheck); err != nil {
		inv.setErr("failed", "post_write_invalid", "改后文本解析不过（回滚档 · 不写）")
		fmt.Fprintf(stderr, "%s: 改后文本解析不过（%v）⇒ 判红（退码 1 · **不写** · 这就是先校验后写的用处）\n", progName, err)
		return exitFail
	}
	before, after := len(fc.Models[s.Host]), len(recheck.Models[s.Host])
	if after != before+1 {
		inv.setErr("failed", "post_write_mismatch", "改后条数不是 +1（回滚档 · 不写）")
		fmt.Fprintf(stderr, "%s: 改后条数 %d → %d（不是 +1）⇒ 判红（退码 1 · **不写**）\n", progName, before, after)
		return exitFail
	}
	// ── 干跑（零副作用：不写一个字节）──
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : model add（%s）\n", progName)
		fmt.Fprintf(stdout, "  名册件   : %s\n", path)
		fmt.Fprintf(stdout, "  落点     : 第 %d 行之后（`models:` 的 `%s:` 块末 · 块首第 %d 行）\n", at, s.Host, hostLine)
		fmt.Fprintf(stdout, "  新增行   : %s\n", newLine)
		fmt.Fprintf(stdout, "  校验     : ✔ 现档解析过（%s 下 %d 条）· ✔ 改后解析过（%d 条）· ✔ 无同名/同 file\n", s.Host, before, after)
		fmt.Fprintf(stdout, "  它会动   : 只此一件（%s）—— 写 = 同目录临时件 + rename（原子）；写完读回再校，任一步不过 ⇒ 回滚\n", path)
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
		"fleet": path, "line": strconv.Itoa(at + 1), "added": s.Added}
	fmt.Fprintf(stderr, "%s: 名册已写（第 %d 行 · 只此一件）—— 要主控读它：`%s config reload --yes`\n", progName, at+1, progName)
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
	var fc config.FleetConfig
	if err := yaml.Unmarshal(back, &fc); err != nil {
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
	// ① 先校验（本地 · 与主控同一份解析器 `config.LoadFleetConfig` 的同一件）
	fc, err := loadFleet(path)
	if err != nil {
		inv.setErr("failed", "config_invalid", "名册件解析不过 ⇒ 不请求热加载")
		fmt.Fprintf(stderr, "%s: 名册件解析不过：%v\n", progName, err)
		fmt.Fprintf(stderr, "⇒ **不发请求**（旧配置继续跑 —— 这就是「先校验、失败回滚」）· 退码 1\n")
		return exitFail
	}
	hosts := []string{}
	for h := range fc.Models {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	// ② 干跑：只校验 + 出计划件（**一个请求都不发**）
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : config reload（%s）\n", progName)
		fmt.Fprintf(stdout, "  名册件   : %s（先校验：✔ 解析过）\n", path)
		fmt.Fprintf(stdout, "  在册     : %d 个 host（%s）· %d 个 fleet 节点\n", len(fc.Models), strings.Join(hosts, " / "), len(fc.Fleet))
		fmt.Fprintf(stdout, "  将请求   : POST %s/api/config/reload（路由**已在跑的主控上** ⇒ 主控零改动）\n", newClient().base)
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
