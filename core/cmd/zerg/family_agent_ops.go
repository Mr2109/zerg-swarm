// family_agent_ops.go —— `agent` 族第二组：`reload` / `registry` / `bench`
// （缺口 `GAP-20260925-50` 的 ②③④；① `agent load|unload` 已由 `egg run|stop` 承接 ⇒ 本件**不重开**）。
//
// 三条命令各自**可复跑的通路**（回执里必须写清走的哪条 —— 不许含糊）：
//
//	`agent reload <机>`   读 `gateway/fleet.yaml` 的 `fleet:` 段定地址（`<host>:<port>` = 子端 HTTP 面）
//	                      → `POST http://<host>:<port>/infer/reload`，**必带 `X-Auth-Token`**
//	                      （不带就回 `{"error":"unauthorized"}` —— 令牌只从**凭据链**取
//	                      `env:ZERG_TOKEN > file:~/.zerg/token`，绝不进 argv / 绝不进日志 · §九 M2 `C2`/`C3`）
//	                      → 子端回 `{status, models, egg_profiles}`，**三格原样透传**。
//
//	`agent registry <机>` 子端**自带注册表**（`zerg-agentd --registry <件>`），注册表件在**那台机的盘上**：
//	                      本机子端的落点**现读** `deploy/com.zerg.agent-<机>.plist` 的 `--registry` 实参
//	                      （不写死路径 · 与启动声明同源）；远端机（host 不是回环）⇒ 本命令今天没有通路面
//	                      （子端没有注册表写端点，`ssh` 属 §十五.4 例外清单）⇒ `kind=blocked` + 退 8，**不给结论**。
//	                      形状：**顶层直接是模型名**（不许包 `models:` 层）、键名与名册模型 id **逐字相同**；
//	                      每条含 `file`/`backend`/`cmd`/`mem_gb`/`modality`/`note`（`cmd` 带 `{file}` 与 `{port}` 占位符）。
//
//	`agent bench <机>`    先 `GET /eggs`（退一步 `GET /status`）问出**在跑的引擎**（问不到 ⇒ `blocked` 退 8，
//	                      **不许编**），再**按名字/端口从子端回据里认准那一枚**（`--model` 点名 ⇒ 逐字认
//	                      那一条；多枚在跑而没点名 ⇒ 退 2 要你点名 —— **不许「抓碰巧第一个」**，参
//	                      `GAP-20260925-49`），对 `http://<host>:<engine_port>/v1/chat/completions` 发一次
//	                      固定形状的请求（`{"messages":[{"role":"user","content":"用一句话说明虫族是什么。"}],
//	                      "max_tokens":<N>,"temperature":0,"stream":false}`），输出 `timings.*` 那几格，
//	                      **并把 `content`（正文）与 `reasoning_content`（思考）分两格报**，落**三态判据**
//	                      （`verdict`）：**可读正文 / 只有思考（推理预算不够，未到正文）/ 真乱码** ——
//	                      三态各自成立、**不许塌成一格**（思考模型要烧 ~570 token 才吐正文 ⇒ 拿思维草稿
//	                      当「乱码」是误判 ✗ · `GAP-20260925-49`）。
//	                      退码纪律：**0**（测到了 —— 三态任一都是 0：**引擎输出差 ≠ 命令错**，写进 `warnings[]`）
//	                      / **2**（用法面）/ **8**（前置不在 ⇒ 不给结论）。
//
// 只读/写面纪律：`reload` 与 `bench` 只打子端的只读/热重载面；`registry` 的写面走 `--dry-run` 先行、
// 真写要 `--yes`、写前留 `.bak-<用途>-<日期>`、写完读回再校、不过即回滚（照 `model add` 的成熟做法）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ---- 地址：机器名 → host:port（`gateway/fleet.yaml` 的 `fleet:` 段）----

type agentAddr struct {
	Host string
	Port int
}

// agentFleetAddrFileRel —— 地址的现读落点（仓根相对 —— 不写死私有绝对路径）。
const agentFleetAddrFileRel = "gateway/fleet.yaml"

// agentFleetAddrs 现读 `fleet:` 段 → 机器名 → 地址 + 机器名清单（排序）。
//
// 为什么读这一段而不是 `~/.zerg/contexts/<档>.yaml` 的 `nodes:`：`Z2` 说真源是档，但档里
// 今天只有**键**（`loadRoster` 只取键，见 remote.go），host/port 取不出来；而 `fleet:` 段是
// 它的**投影源**，门④（`scripts/gates/check-nodes-roster.py`）逐字段核两者逐字一致 ⇒ 读它
// 与读真源同值。档里加了 host/port 之后本函数可整体切过去（换一处读，换一处评）。
func agentFleetAddrs(stderr io.Writer) (map[string]agentAddr, []string, int) {
	root := repoRoot()
	if root == "" {
		inv := &invocation{}
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 —— 读不到名册地址（%s；找 core/internal/version/version.go 失败）\n",
			progName, agentFleetAddrFileRel)
		return nil, nil, exitBlocked
	}
	b, err := os.ReadFile(filepath.Join(root, agentFleetAddrFileRel))
	if err != nil {
		inv := &invocation{}
		inv.setErr("blocked", "roster_file_absent", "名册件不在")
		fmt.Fprintf(stderr, "%s: 名册件读不到（%s：%v）⇒ 拿不到地址、不给结论（退码 8）\n",
			progName, agentFleetAddrFileRel, err)
		return nil, nil, exitBlocked
	}
	addrs := map[string]agentAddr{}
	inFleet := false
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inFleet = strings.TrimSpace(line) == "fleet:"
			continue
		}
		if !inFleet {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, rest, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		host := yamlInlineField(rest, "host")
		portStr := yamlInlineField(rest, "port")
		port, _ := strconv.Atoi(portStr)
		if name == "" || host == "" || port == 0 {
			continue
		}
		addrs[name] = agentAddr{Host: host, Port: port}
	}
	if len(addrs) == 0 {
		inv := &invocation{}
		inv.setErr("blocked", "roster_empty", "名册 `fleet:` 段一条地址都取不出来")
		fmt.Fprintf(stderr, "%s: 名册件读得到、但 `fleet:` 段一条地址都取不出来（%s）⇒ 不给结论（退码 8）\n",
			progName, agentFleetAddrFileRel)
		return nil, nil, exitBlocked
	}
	names := make([]string, 0, len(addrs))
	for n := range addrs {
		names = append(names, n)
	}
	sort.Strings(names)
	return addrs, names, exitOK
}

// yamlInlineField 从 `{ host: <worker-ip>, port: 8101, os: macos }` 这种**单行内联**里取一个字段值。
func yamlInlineField(rest, key string) string {
	seg := rest
	for {
		i := strings.Index(seg, key+":")
		if i < 0 {
			return ""
		}
		// 前一个字符必须是分隔/起始（防 `myport:` 命中 `port:`）
		if i > 0 && (seg[i-1] == '_' || seg[i-1] == '-' || seg[i-1] == '.' ||
			(seg[i-1] >= 'a' && seg[i-1] <= 'z') || (seg[i-1] >= 'A' && seg[i-1] <= 'Z')) {
			seg = seg[i+len(key)+1:]
			continue
		}
		v := strings.TrimSpace(seg[i+len(key)+1:])
		if j := strings.IndexAny(v, ",}"); j >= 0 {
			v = v[:j]
		}
		return strings.TrimSpace(v)
	}
}

// agentChildAddr 取一台机的子端地址；名册里没有 ⇒ **不是错、是没有**（退 2 + 列可选值 + 最像的名）。
func agentChildAddr(machine string, stderr io.Writer) (agentAddr, int) {
	addrs, names, rc := agentFleetAddrs(stderr)
	if rc != exitOK {
		return agentAddr{}, rc
	}
	a, ok := addrs[machine]
	if !ok {
		fmt.Fprintf(stderr, "%s: 名册（%s 的 `fleet:` 段）里没有这台机 %q ⇒ 退码 2（**不是错、是没有**）\n",
			progName, agentFleetAddrFileRel, machine)
		if s := nearestName(machine, names); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
		}
		fmt.Fprintf(stderr, "可选机器: %s\n", strings.Join(names, " · "))
		fmt.Fprintf(stderr, "error.kind=usage · detail=unknown_machine · retryable=false\n")
		return agentAddr{}, exitUsage
	}
	return a, exitOK
}

// ---- 打**子端**的两条最小通路（令牌只从凭据链取 —— 不进 argv、不进日志、不进错误串）----

func agentChildBase(a agentAddr) string {
	return "http://" + net.JoinHostPort(a.Host, strconv.Itoa(a.Port))
}

// agentChildDo 打子端的一条请求。**不许**在错误里回显令牌（`C3`：永不进日志）。
func agentChildDo(a agentAddr, method, path, body string, timeout time.Duration) (int, string, error) {
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, agentChildBase(a)+path, rdr)
	if err != nil {
		return 0, "", err
	}
	if tok, _ := resolveCredentials(); tok != "" {
		req.Header.Set("X-Auth-Token", tok) // 子端面唯一真源 header（§九 M2 C10）
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

// agentChildUnreachable 子端打不到 ⇒ `blocked`（前置不在）+ 退 8，**不换端点、不读缓存、不打第二枪**。
func agentChildUnreachable(inv *invocation, a agentAddr, path string, err error, stderr io.Writer) int {
	inv.setErr("blocked", "child_unreachable", fmt.Sprintf("子端 %s 打不到", agentChildBase(a)+path))
	fmt.Fprintf(stderr, "%s: 打不到子端 %s%s ⇒ **不给结论**（退码 8 · 前置不在）\n", progName, agentChildBase(a), path)
	fmt.Fprintf(stderr, "原因: %v\n", err)
	fmt.Fprintf(stderr, "error.kind=blocked · detail=child_unreachable · retryable=true · remedy=fix_precondition\n")
	fmt.Fprintf(stderr, "不做的：不换端点、不读缓存当结果、不打第二枪（§九 M20 O2/O6）\n")
	return exitBlocked
}

// agentChildAuthFail 子端回 401/403 ⇒ `unauthenticated`/`forbidden`（码 4 · kind 分家 · §十二 P-013 ④）。
func agentChildAuthFail(inv *invocation, a agentAddr, path string, status int, stderr io.Writer) int {
	kind := "unauthenticated"
	if status == http.StatusForbidden {
		kind = "forbidden"
	}
	_, src := resolveCredentials()
	inv.setErr(kind, fmt.Sprintf("http_%d", status), "子端拒了这次请求（令牌不对/权限不够）")
	fmt.Fprintf(stderr, "%s: 子端 %s%s 回 %d —— **令牌口径**：子端面要 `X-Auth-Token`（不带就回 `{\"error\":\"unauthorized\"}`）\n",
		progName, agentChildBase(a), path, status)
	fmt.Fprintf(stderr, "当前凭据来源: %s（令牌**绝不进 argv**；要非交互走 `--token-stdin`）\n", src)
	fmt.Fprintf(stderr, "error.kind=%s · detail=http_%d · retryable=false · remedy=%s\n", kind, status, remedyOf(kind))
	return codeOfKind(kind)
}

// agentCheckFields `--json` 字段闭集的判口（照 `eggAct` 的同一形状：闭集外 ⇒ 退 2 列闭集）。
func agentCheckFields(inv *invocation, stderr io.Writer, legal []string) int {
	ok := map[string]bool{}
	for _, f := range legal {
		ok[f] = true
	}
	for _, f := range inv.fields {
		if !ok[f] {
			return reportBadField(stderr, inv.path, f)
		}
	}
	return exitOK
}

// agentFillRow 把闭集里缺席的键补成空串（机器面字段投影要能对任一闭集键取值 · 不许「该键不存在」）。
func agentFillRow(row map[string]string, keys []string) map[string]string {
	for _, k := range keys {
		if _, exists := row[k]; !exists {
			row[k] = ""
		}
	}
	return row
}

// ---- ② `zerg agent reload <机>` ----

var agentReloadFields = []string{"machine", "host", "port", "via", "status", "models", "egg_profiles", "egg_profile_errors"}

// agentReloadDisplay —— 人面表格的列（子端那三格**原样**在最前）。
var agentReloadDisplay = []string{"machine", "status", "models", "egg_profiles"}

func cmdAgentReload(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面先判（零网络 · 零副作用）：`--json` 不给字段 / 闭集外字段。
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		if rc := agentCheckFields(inv, stderr, agentReloadFields); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent reload` 要一台机器名（`%s agent ls` 看名册）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数（本档只吃一台机）")
		fmt.Fprintf(stderr, "%s: `agent reload` 只吃**一台**机（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])
	a, rc := agentChildAddr(machine, stderr)
	if rc != exitOK {
		return rc
	}

	// ② 真打：子端热重载（**带令牌**）。
	//    无 `--json`：子端原文让人看见；带 `--json`：原文收进缓冲，机器面只出六键包封（人读不许混进来）。
	path := "/infer/reload"
	fmt.Fprintf(stderr, "%s: agent reload %s · via=%s%s（子端热重载 · 带 X-Auth-Token；令牌不入 argv/日志）\n",
		progName, machine, agentChildBase(a), path)
	status, body, err := agentChildDo(a, http.MethodPost, path, "", 120*time.Second)
	if err != nil {
		return agentChildUnreachable(inv, a, path, err, stderr)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return agentChildAuthFail(inv, a, path, status, stderr)
	}
	if status != http.StatusOK {
		inv.setErr("upstream_error", fmt.Sprintf("http_%d", status), "子端热重载没成功")
		fmt.Fprintf(stderr, "%s: POST %s%s ⇒ HTTP %d\n", progName, agentChildBase(a), path, status)
		fmt.Fprintf(stderr, "子端原样响应: %s\n", strings.TrimSpace(body))
		fmt.Fprintf(stderr, "error.kind=upstream_error · detail=http_%d · retryable=true · remedy=%s\n",
			status, remedyOf("upstream_error"))
		return codeOfKind("upstream_error")
	}

	var resp jsonObj
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		inv.setErr("upstream_error", "bad_json", "子端回了非 JSON")
		fmt.Fprintf(stderr, "%s: 子端 %s 的响应不是合法 JSON（%v）⇒ 三格无从透传，不给结论\n", progName, path, err)
		fmt.Fprintf(stderr, "子端原样响应: %s\n", strings.TrimSpace(body))
		return codeOfKind("upstream_error")
	}
	// 三格**原样透传**（子端自己的说法，命令面不翻译、不补 0、不把缺席写成空值）。
	row := map[string]string{
		"machine":            machine,
		"host":               a.Host,
		"port":               strconv.Itoa(a.Port),
		"via":                agentChildBase(a) + path,
		"status":             cell(resp["status"]),
		"models":             cell(resp["models"]),
		"egg_profiles":       cell(resp["egg_profiles"]),
		"egg_profile_errors": cell(resp["egg_profile_errors"]),
	}
	if errs := asList(resp["egg_profile_errors"]); len(errs) > 0 {
		inv.warnf("子端报 %d 枚卵档案有问题（%s）—— 热重载本身成功，但档案没全刷上", len(errs), cell(resp["egg_profile_errors"]))
	}
	if row["status"] == "" {
		inv.warnf("子端响应里没有 `status` 这一格（响应形状与预期不符）—— 照原样呈现，不补一个")
	}
	if !inv.jsonGiven {
		fmt.Fprintf(stdout, "子端原文: %s\n", strings.TrimSpace(body))
	}
	return listCmd(inv, stdout, stderr, agentReloadDisplay, []map[string]string{agentFillRow(row, agentReloadFields)})
}

// ---- ③ `zerg agent registry <机>` ----

var agentRegistryFields = []string{
	"machine", "registry_path", "model", "file", "backend", "cmd", "mem_gb", "modality", "note",
	"action", "dry_run", "changed", "backup",
}

// agentRegistryListDisplay —— 列表模式的人面列。
var agentRegistryListDisplay = []string{"model", "file", "backend", "modality", "mem_gb"}

// agentRegistryRow —— 注册表的一条（顶层键 = 模型名；下面那几格就是它缩进着的字段）。
type agentRegistryRow struct {
	Name     string
	File     string
	Backend  string
	Cmd      string
	MemGB    string
	Modality string
	Note     string
}

// agentRegistryParse —— 解析子端注册表（**顶层直接是模型名**的形状；`#` 注释与空行跳过）。
//
// 只认这一层：缩进键值里的 `键: 值`（值取到行尾，注释不剥离 —— 与子端 `registry` 包的读法同口径：
// 值原样，写面也不改写既有条目）。
func agentRegistryParse(text string) []agentRegistryRow {
	rows := []agentRegistryRow{}
	idx := map[string]int{}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			name := strings.TrimSpace(line)
			name = strings.TrimSuffix(name, ":")
			if name == "" || strings.Contains(name, ":") {
				continue // `{}` 这类空注册表形态
			}
			idx[name] = len(rows)
			rows = append(rows, agentRegistryRow{Name: name})
			continue
		}
		if len(rows) == 0 {
			continue
		}
		cur := &rows[len(rows)-1]
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "file":
			cur.File = v
		case "backend":
			cur.Backend = v
		case "cmd":
			cur.Cmd = v
		case "mem_gb":
			cur.MemGB = v
		case "modality":
			cur.Modality = v
		case "note":
			cur.Note = strings.Trim(v, "\"")
		}
	}
	return rows
}

// agentRegistryPath —— 子端注册表件的落点。
//
// 判据两条（**不写死私有绝对路径** —— 跑 `scripts/check-hardcoded-private-paths.py` 的面）：
//   - 子端**就跑在本机**（fleet 地址是回环）⇒ 落点现读 `deploy/com.zerg.agent-<机>.plist` 里
//     `--registry` 的实参（= 那台子端的**启动声明真源**，与跑着的进程同一份 args）；
//   - 远端机 ⇒ 注册表件在那台机的盘上，本命令**没有通路面**（子端今天没有注册表写端点，
//     `ssh` 属 §十五.4 例外清单 `F-1`–`F-3`）⇒ `kind=blocked` 退 8，**不给结论**（不假装改到了）。
func agentRegistryPath(machine string, a agentAddr, inv *invocation, stderr io.Writer) (string, int) {
	switch a.Host {
	case "127.0.0.1", "localhost", "::1":
	default:
		inv.setErr("blocked", "registry_remote_face_absent",
			fmt.Sprintf("子端 %s 在 %s（不是本机）⇒ 注册表件在它自己的盘上，本命令今天没有通路面", machine, a.Host))
		fmt.Fprintf(stderr, "%s: 子端 %s 的地址是 %s（不是本机）⇒ **改不到它的注册表**（退码 8 · 不给结论）\n",
			progName, machine, agentChildBase(a))
		fmt.Fprintf(stderr, "为什么：子端今天**没有注册表写端点**（只有 `POST /infer/reload` 重读盘上的件）；\n")
		fmt.Fprintf(stderr, "        跨机改件要走 `ssh`，而 `ssh` 属 §十五.4 例外清单（F-1/F-2/F-3）—— 本命令不偷偷开这条路\n")
		fmt.Fprintf(stderr, "现在能做的：在那台机上直接改注册表件，再 `%s agent reload %s`\n", progName, machine)
		fmt.Fprintf(stderr, "error.kind=blocked · detail=registry_remote_face_absent · retryable=false · remedy=fix_precondition\n")
		return "", exitBlocked
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不到子端启动声明件（退码 8）\n", progName)
		return "", exitBlocked
	}
	rel := filepath.Join("deploy", "com.zerg.agent-"+machine+".plist")
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		inv.setErr("blocked", "agent_decl_absent", fmt.Sprintf("子端启动声明件不在（%s）", rel))
		fmt.Fprintf(stderr, "%s: 读不到子端 %s 的启动声明件（%s：%v）⇒ 注册表落点取不出来，**不给结论**（退码 8）\n",
			progName, machine, rel, err)
		fmt.Fprintf(stderr, "不做的：不按「大概在 ~/agent/agent_models.yaml」猜一个路径（猜出来的路径会写错地方）\n")
		return "", exitBlocked
	}
	// `<string>--registry</string><string>/Users/…/agent_models.yaml</string>`
	const marker = "--registry"
	s := string(b)
	i := strings.Index(s, marker)
	if i < 0 {
		inv.setErr("blocked", "registry_arg_absent", fmt.Sprintf("%s 里没有 --registry 实参", rel))
		fmt.Fprintf(stderr, "%s: %s 里找不到 `--registry` 实参 ⇒ 注册表落点取不出来（退码 8）\n", progName, rel)
		return "", exitBlocked
	}
	tail := s[i+len(marker):]
	open := strings.Index(tail, "<string>")
	if open < 0 {
		inv.setErr("blocked", "registry_arg_unreadable", "plist 的 --registry 实参读不出来")
		fmt.Fprintf(stderr, "%s: %s 的 `--registry` 实参形状读不出来（退码 8 · 不给结论）\n", progName, rel)
		return "", exitBlocked
	}
	// ★ 收尾那个 `</string>` 必须从**值的开头之后**找：`--registry` 自己也是被 `</string>` 收尾的，
	//   从头找会一找一个准地抓到**旗标自己**的收尾（实测踩过：`--registry` 的实参一律读不出来）。
	after := tail[open+len("<string>"):]
	closeIdx := strings.Index(after, "</string>")
	if closeIdx < 0 {
		inv.setErr("blocked", "registry_arg_unreadable", "plist 的 --registry 实参读不出来")
		fmt.Fprintf(stderr, "%s: %s 的 `--registry` 实参形状读不出来（退码 8 · 不给结论）\n", progName, rel)
		return "", exitBlocked
	}
	p := strings.TrimSpace(after[:closeIdx])
	if p == "" {
		inv.setErr("blocked", "registry_arg_empty", "plist 的 --registry 实参是空的")
		fmt.Fprintf(stderr, "%s: %s 的 `--registry` 实参是空的（退码 8 · 不给结论）\n", progName, rel)
		return "", exitBlocked
	}
	return p, exitOK
}

// cmdAgentRegistry —— `zerg agent registry <机> [--list] [--add <模型名> --file <GGUF> --ctx <N> --mem-gb <N>] [--dry-run | --yes] [--json <字段>]`
func cmdAgentRegistry(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		if rc := agentCheckFields(inv, stderr, agentRegistryFields); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent registry` 要一台机器名（`%s agent ls` 看名册）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `agent registry` 只吃**一台**机（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	machine := strings.TrimSpace(inv.args[0])
	a, rc := agentChildAddr(machine, stderr)
	if rc != exitOK {
		return rc
	}
	regPath, rc := agentRegistryPath(machine, a, inv, stderr)
	if rc != exitOK {
		return rc
	}
	listing := inv.list
	model := strings.TrimSpace(inv.flagVal("--add"))
	switch {
	case listing && model != "":
		inv.setErr("usage", "conflicting_face", "`--list` 与 `--add` 是两面，不给同一轮")
		fmt.Fprintf(stderr, "%s: `--list`（只读面）与 `--add`（写面）**不许同给** ⇒ 退码 2\n", progName)
		return exitUsage
	case !listing && model == "":
		inv.setErr("usage", "face_required", "要么 `--list`，要么 `--add <模型名>`")
		fmt.Fprintf(stderr, "%s: `agent registry` 要挑一面：`--list`（只读）或 `--add <模型名> --file <GGUF> --ctx <N> --mem-gb <N>`（写面）\n", progName)
		fmt.Fprintf(stderr, "先看现状：%s agent registry %s --list\n", progName, machine)
		return exitUsage
	}

	raw, err := os.ReadFile(regPath)
	if err != nil {
		inv.setErr("blocked", "registry_unreadable", fmt.Sprintf("注册表件读不到（%s）", regPath))
		fmt.Fprintf(stderr, "%s: 注册表件读不到（%s：%v）⇒ **不给结论**（退码 8）\n", progName, regPath, err)
		return exitBlocked
	}
	existing := agentRegistryParse(string(raw))
	byName := map[string]bool{}
	for _, r := range existing {
		byName[r.Name] = true
	}

	// ---- 只读面：`--list` ----
	if listing {
		rows := []map[string]string{}
		for _, r := range existing {
			rows = append(rows, agentFillRow(map[string]string{
				"machine": machine, "registry_path": regPath,
				"model": r.Name, "file": r.File, "backend": r.Backend, "cmd": r.Cmd,
				"mem_gb": r.MemGB, "modality": r.Modality, "note": r.Note,
			}, agentRegistryFields))
		}
		fmt.Fprintf(stderr, "%s: agent registry %s --list · 注册表件 %s（顶层键 %d 条 · 只读）\n",
			progName, machine, regPath, len(rows))
		return listCmd(inv, stdout, stderr, agentRegistryListDisplay, rows)
	}

	// ---- 写面：`--add`（先校验 → 干跑先行 → `--yes` 才真写 → 写完读回再校）----
	file := strings.TrimSpace(inv.flagVal("--file"))
	ctxStr := strings.TrimSpace(inv.flagVal("--ctx"))
	memStr := strings.TrimSpace(inv.flagVal("--mem-gb"))
	for _, req := range []struct{ name, val string }{
		{"--file <GGUF>", file}, {"--ctx <N>", ctxStr}, {"--mem-gb <N>", memStr},
	} {
		if req.val == "" {
			inv.setErr("usage", "missing_required_flag", fmt.Sprintf("缺 %s", req.name))
			fmt.Fprintf(stderr, "%s: 写面要 `%s`（给的是空）⇒ 退码 2（**不许**拿缺省值替你编一个）\n", progName, req.name)
			return exitUsage
		}
	}
	ctx, err1 := strconv.Atoi(ctxStr)
	memGB, err2 := strconv.Atoi(memStr)
	if err1 != nil || ctx <= 0 {
		inv.setErr("usage", "bad_ctx", fmt.Sprintf("--ctx %q 不是正整数", ctxStr))
		fmt.Fprintf(stderr, "%s: `--ctx %q` 不是正整数 ⇒ 退码 2\n", progName, ctxStr)
		return exitUsage
	}
	if err2 != nil || memGB <= 0 {
		inv.setErr("usage", "bad_mem_gb", fmt.Sprintf("--mem-gb %q 不是正整数", memStr))
		fmt.Fprintf(stderr, "%s: `--mem-gb %q` 不是正整数 ⇒ 退码 2\n", progName, memStr)
		return exitUsage
	}
	if _, err := os.Stat(file); err != nil {
		inv.setErr("usage", "weights_absent", fmt.Sprintf("权重件不在盘上（%s）", file))
		fmt.Fprintf(stderr, "%s: `--file %s` 在本机盘上**不存在** ⇒ 退码 2（本命令不替你登记盘上没有的权重）\n", progName, file)
		return exitUsage
	}
	if byName[model] {
		inv.setErr("conflict", "already_registered", fmt.Sprintf("注册表里已经有 %q 这条", model))
		fmt.Fprintf(stderr, "%s: 子端注册表里**已经有** %q 这条（%s）⇒ 退码 14（冲突 · **不静默覆盖**同行）\n",
			progName, model, regPath)
		fmt.Fprintf(stderr, "error.kind=conflict · detail=already_registered · retryable=false · remedy=wait_or_reload\n")
		fmt.Fprintf(stderr, "要改它：先在那台机上改件，再 `%s agent reload %s`（覆盖式写面本版不开）\n", progName, machine)
		return codeOfKind("conflict")
	}

	// 键名必须与**名册模型 id 逐字相同** —— 问主控要名册（正门 `/api/fleet/models`，不自己解析 YAML）。
	c := newClient()
	var fleet jsonObj
	if rc := fetchInv(inv, c, "/api/fleet/models", &fleet, stderr); rc != exitOK {
		return rc
	}
	var entry jsonObj
	ids := []string{}
	for _, it := range asList(fleet["models"]) {
		o := asObj(it)
		if o == nil {
			continue
		}
		id := cell(o["id"])
		if id != "" {
			ids = append(ids, id)
		}
		if id == model && cell(o["host"]) == machine {
			entry = o
		}
	}
	sort.Strings(ids)
	if entry == nil {
		inv.setErr("usage", "no_such_model", fmt.Sprintf("名册里没有模型 %q（或它不在这台机上）", model))
		fmt.Fprintf(stderr, "%s: 名册（`/api/fleet/models`）里没有「%s 在这台机 %s 上」这一条 ⇒ 退码 2（**不是错、是没有**）\n",
			progName, model, machine)
		if s := nearestName(model, ids); s != "" {
			fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
		}
		fmt.Fprintf(stderr, "可选模型（名册）: %s\n", strings.Join(ids, " · "))
		fmt.Fprintf(stderr, "口径：注册表的键名必须与名册模型 id **逐字相同**（不同名 ⇒ 两条各自漂）\n")
		return exitUsage
	}
	if f := cell(entry["file"]); f != "" && f != file {
		inv.warnf("名册里 %s 的 file 是 %s，本次给的是 %s —— 注册表照**本次给的值**写", model, f, file)
	}

	// cmd 的引擎二进制：**现读同一份注册表里最近一条 `cmd:` 的第一个词**（不许编一个路径出来）。
	engineBin := ""
	for _, r := range existing {
		if r.Cmd == "" {
			continue
		}
		// 两种既有形状都认：YAML 列表式 `["/opt/homebrew/bin/llama-server", "-m", …]`
		// 与内联标量式 `/opt/homebrew/bin/llama-server -m {file} …` —— 取第一个词并**剥掉列表标点**
		// （不剥就会把 `["/opt/homebrew/bin/llama-server",` 原样写进 cmd ⇒ 子端起不来）。
		engineBin = agentEngineBinOf(r.Cmd)
		break
	}
	if engineBin == "" {
		inv.setErr("blocked", "engine_bin_unknown", "同一份注册表里没有一条现成的 `cmd:` ⇒ 引擎二进制取不出来")
		fmt.Fprintf(stderr, "%s: 注册表 %s 里没有任何一条现成的 `cmd:` ⇒ 引擎二进制路径**取不出来**（退码 8）\n",
			progName, regPath)
		fmt.Fprintf(stderr, "不做的：不按「大概是 /opt/homebrew/bin/llama-server」编一个路径（编出来的 cmd 起不来）\n")
		return exitBlocked
	}
	backend := cell(entry["backend"])
	if backend == "" {
		backend = "llama-server"
	}
	modality := cell(entry["modality"])
	if modality == "" {
		modality = "text"
	}
	mmproj := cell(entry["mmproj"])
	argv := []string{engineBin, "-m", "{file}"}
	if mmproj != "" {
		argv = append(argv, "--mmproj", mmproj)
	}
	argv = append(argv, "-c", strconv.Itoa(ctx), "-ngl", "999", "--host", "127.0.0.1", "--port", "{port}")
	block := agentRegistryBlock(model, file, backend, strings.Join(argv, " "), memGB, modality, agentStamp())

	plan := []string{
		fmt.Sprintf("子端     : %s（%s）", machine, agentChildBase(a)),
		fmt.Sprintf("注册表件 : %s（顶层键 %d 条 · 形状 = 顶层直接是模型名）", regPath, len(existing)),
		fmt.Sprintf("要写入的段（**顶层直接是模型名** · 缩进两格）:"),
	}
	for _, ln := range strings.Split(strings.TrimRight(block, "\n"), "\n") {
		plan = append(plan, "    "+ln)
	}
	plan = append(plan,
		fmt.Sprintf("它会动   : 给 %s 加一条注册表项（占它下一轮 `agent reload` 的名额 —— **本轮不改运行态**）", machine),
		"写前留档 : 同目录 `.bak-registry-add-<日期>`（本命令自己取，重名自动加序号）",
		"写完读回 : 重新解析该件，逐格核 `file`/`mem_gb`/`cmd` —— 不过即从备份回滚",
		"生效     : 写完要 `"+progName+" agent reload "+machine+"` 才进子端运行态",
	)
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.dryRun {
		w := stdout
		if inv.jsonGiven {
			w = stderr // 机器面 stdout 只许剩包封（`A3-b` 案的先例）
		}
		fmt.Fprintf(w, "计划件（--dry-run · 零副作用 —— 未写件、未留备份、未打子端）\n")
		fmt.Fprintf(w, "  动作     : %s agent registry %s --add %s\n", progName, machine, model)
		for _, ln := range plan {
			fmt.Fprintf(w, "  %s\n", ln)
		}
		fmt.Fprintf(w, "  授权判据 : 真写要 `--yes`（与干跑**同一张表**：缺它 ⇒ 退码 2）\n")
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）\n")
		changed := false
		inv.changed = &changed
		row := map[string]string{
			"machine": machine, "registry_path": regPath, "model": model, "file": file,
			"backend": backend, "cmd": strings.Join(argv, " "), "mem_gb": strconv.Itoa(memGB),
			"modality": modality, "action": "add", "dry_run": "true", "changed": "false", "backup": "",
		}
		if !inv.jsonGiven {
			// 人面：计划件本身就是结果面（不再多印一张单行表）。
			return exitOK
		}
		return listCmd(inv, stdout, stderr, []string{"machine", "model", "registry_path", "action", "dry_run"}, []map[string]string{agentFillRow(row, agentRegistryFields)})
	}
	if !inv.yes {
		inv.setErr("usage", "yes_required", "写面缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `agent registry --add` 会**改子端的注册表件** ⇒ **缺 --yes 不执行**（§九 M3 C2 fail-closed）\n", progName)
		fmt.Fprintf(stderr, "  动作     : %s agent registry %s --add %s\n", progName, machine, model)
		for _, ln := range plan {
			fmt.Fprintf(stderr, "  %s\n", ln)
		}
		fmt.Fprintf(stderr, "加 `--yes` 才写；先看计划件：%s agent registry %s --add %s --file %s --ctx %d --mem-gb %d --dry-run\n",
			progName, machine, model, file, ctx, memGB)
		return exitUsage
	}

	// 真写：留档 → 追加 → 读回再校 → 不过即回滚（照 `model add` 的成熟做法）。
	ash := agentSHA256(raw)
	backup := agentBackupPath(regPath, agentStamp())
	if err := agentCopyFile(regPath, backup); err != nil {
		inv.setErr("failed", "backup_failed", fmt.Sprintf("留档失败（%s）", backup))
		fmt.Fprintf(stderr, "%s: 写前留档失败（%s ← %s：%v）⇒ **一个字节都没写**（退码 1）\n", progName, backup, regPath, err)
		return exitFail
	}
	next := agentAppendBlock(string(raw), block)
	if err := agentWriteFile(regPath, []byte(next), string(raw)); err != nil {
		inv.setErr("failed", "write_failed", fmt.Sprintf("写件失败（%s）", regPath))
		fmt.Fprintf(stderr, "%s: 写注册表件失败（%s：%v）—— 备份在 %s，原档未变\n", progName, regPath, err, backup)
		return exitFail
	}
	// 读回再校：重新解析该件，逐格核。
	back, err := os.ReadFile(regPath)
	if err != nil {
		return agentRegistryRollback(inv, regPath, backup, ash, stderr, fmt.Sprintf("写完读回失败：%v", err))
	}
	rowsAfter := agentRegistryParse(string(back))
	var got *agentRegistryRow
	for i := range rowsAfter {
		if rowsAfter[i].Name == model {
			got = &rowsAfter[i]
		}
	}
	if got == nil {
		return agentRegistryRollback(inv, regPath, backup, ash, stderr, "读回后找不到刚写的那条键")
	}
	if got.File != file || got.MemGB != strconv.Itoa(memGB) {
		return agentRegistryRollback(inv, regPath, backup, ash, stderr,
			fmt.Sprintf("读回对不上：file=%q（期望 %q）· mem_gb=%q（期望 %d）", got.File, file, got.MemGB, memGB))
	}
	changed := true
	inv.changed = &changed
	fmt.Fprintf(stderr, "%s: 已写入 %s 的第 %d 条（%s）· 留档 %s · 读回逐格已校\n",
		progName, regPath, len(rowsAfter), model, backup)
	fmt.Fprintf(stderr, "生效：%s agent reload %s（现在改的还只是盘上的件）\n", progName, machine)
	row := map[string]string{
		"machine": machine, "registry_path": regPath, "model": model, "file": got.File,
		"backend": backend, "cmd": got.Cmd, "mem_gb": got.MemGB, "modality": got.Modality,
		"note": got.Note, "action": "add", "dry_run": "false", "changed": "true", "backup": backup,
	}
	return listCmd(inv, stdout, stderr, []string{"machine", "model", "registry_path", "action", "changed", "backup"},
		[]map[string]string{agentFillRow(row, agentRegistryFields)})
}

// agentRegistryBlock 生成要写入的**一整段**（顶层直接是模型名 —— 不许包 `models:` 层）。
func agentRegistryBlock(model, file, backend, cmd string, memGB int, modality, stamp string) string {
	return strings.Join([]string{
		model + ":",
		"  file: " + file,
		"  backend: " + backend,
		"  cmd: " + cmd,
		"  mem_gb: " + strconv.Itoa(memGB),
		"  modality: " + modality,
		"  note: \"" + "zerg agent registry " + stamp + " 写入（模型与名册 id 逐字同）" + "\"",
	}, "\n") + "\n"
}

// agentEngineBinOf 从一条现成的 `cmd:` 里取**引擎二进制路径**（列表式与内联式两种形状都认）。
func agentEngineBinOf(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], "[\"' ,")
}

// agentRegistryRollback —— 读回不过 ⇒ 用备份逐字复原，报 `rolled_back`（码 1 · §十二 `P-129`）。
func agentRegistryRollback(inv *invocation, regPath, backup, ash string, stderr io.Writer, why string) int {
	bak, err := os.ReadFile(backup)
	if err == nil {
		err = os.WriteFile(regPath, bak, 0o644)
	}
	if err != nil {
		inv.setErr("failed", "rollback_failed", fmt.Sprintf("回滚失败（%s）：%v", regPath, err))
		fmt.Fprintf(stderr, "%s: 读回不过（%s）**且回滚失败**（%v）—— 备份仍在 %s，请人工处置\n", progName, why, err, backup)
		return exitFail
	}
	now := ""
	if b, err := os.ReadFile(regPath); err == nil {
		now = agentSHA256(b)
	}
	inv.setErr("rolled_back", "readback_mismatch", why)
	fmt.Fprintf(stderr, "%s: 读回不过（%s）⇒ **已从备份回滚**（%s）\n", progName, why, regPath)
	fmt.Fprintf(stderr, "回滚后 sha256: %s（写前 %s）—— 逐字相等 = %t\n", now, ash, now == ash)
	fmt.Fprintf(stderr, "error.kind=rolled_back · detail=readback_mismatch · retryable=false · remedy=inspect_receipt\n")
	return codeOfKind("rolled_back")
}

// agentAppendBlock 把一段追加到件尾：保留原档的「末尾有没有换行」约定（照 `family_config.go` 的同一处教训）。
func agentAppendBlock(orig, block string) string {
	if orig == "" {
		return block + "\n"
	}
	out := orig
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if !strings.HasSuffix(out, "\n\n") {
		out += "\n"
	}
	return out + block + "\n"
}

// agentBackupPath 写前留档的落点：`<件>.bak-<用途>-<日期>`（重名自动加序号 —— 不覆盖任何现存备份）。
func agentBackupPath(path, stamp string) string {
	base := path + ".bak-registry-add-" + stamp
	if _, err := os.Stat(base); err != nil {
		return base
	}
	for i := 2; i < 100; i++ {
		cand := base + "-" + strconv.Itoa(i)
		if _, err := os.Stat(cand); err != nil {
			return cand
		}
	}
	return base
}

func agentCopyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(src); err == nil {
		mode = st.Mode().Perm()
	}
	return os.WriteFile(dst, b, mode)
}

// agentWriteFile 写件：保留原档的末尾换行约定（原档末行无换行 ⇒ 写回也不添）。
func agentWriteFile(path string, next []byte, orig string) error {
	if orig != "" && !strings.HasSuffix(orig, "\n") && strings.HasSuffix(string(next), "\n") {
		next = next[:len(next)-1]
	}
	return os.WriteFile(path, next, 0o644)
}

// ---- ④ `zerg agent bench <机>` ----

// agentBenchFields —— `--json` 闭集（**只增不改**：`path`/`text` 等旧名一个不删，新增的六格见下）。
//
//	engine_selfreport 那台**引擎自己**说它载的是哪份权重（`/props.model_path` 的件名）—— 子端回据
//	                  （名字+端口）的**旁证**；`verdict` 三态 / `content_chars` ⟷ `reasoning_chars`
//	                  把**正文与思考分开报**（`GAP-20260925-49` 的判据）。
var agentBenchFields = []string{
	"machine", "model", "engine_host", "engine_port", "engine_selfreport", "path", "n_predict",
	"prompt_n", "predicted_n", "prompt_per_second", "predicted_per_second", "finish_reason",
	"verdict", "readable", "content_chars", "reasoning_chars", "text", "reasoning",
}

var agentBenchDisplay = []string{"machine", "model", "engine_port", "verdict", "predicted_per_second", "prompt_per_second", "predicted_n"}

func cmdAgentBench(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		if rc := agentCheckFields(inv, stderr, agentBenchFields); rc != exitOK {
			return rc
		}
	}
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "缺机器名")
		fmt.Fprintf(stderr, "%s: `agent bench` 要一台机器名（`%s agent ls` 看名册）\n", progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `agent bench` 只吃**一台**机（多给的是 %q）\n", progName, inv.args[1])
		return exitUsage
	}
	nPredict := 96
	if v := strings.TrimSpace(inv.flagVal("--n-predict")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 4096 {
			inv.setErr("usage", "bad_n_predict", fmt.Sprintf("--n-predict %q 不是 1..4096 的整数", v))
			fmt.Fprintf(stderr, "%s: `--n-predict %q` 不是 1..4096 的整数 ⇒ 退码 2\n", progName, v)
			return exitUsage
		}
		nPredict = n
	}
	machine := strings.TrimSpace(inv.args[0])
	a, rc := agentChildAddr(machine, stderr)
	if rc != exitOK {
		return rc
	}

	// ① 从**子端回据**里认准要测的那一枚引擎（`/eggs` 优先、`/status` 兜底；两处都问不到 ⇒ blocked，
	//    **不许编**）。`--model` 点名 ⇒ 逐字认那一条；没点名而多枚在跑 ⇒ 退 2 要你点名（**不替你挑
	//    碰巧第一个** —— `GAP-20260925-49` 的教训）。
	want := strings.TrimSpace(inv.modelWant)
	engine, rc := agentEnginePick(inv, machine, a, want, stderr)
	if rc != exitOK {
		return rc
	}
	engineHost, enginePort, model := engine.Host, engine.Port, engine.Model
	if model == "" {
		model = machine + "（子端未自报模型名）"
	}
	// 认准的**旁证**：那台引擎自己说它载的是哪份权重（`/props.model_path`）。只按子端报的名字/端口
	// 就开测是「只看名字/端口」✗ ⇒ 再问一次引擎本人；对不上就在 warnings 里说清（**不许闷着**）。
	selfReport, selfErr := agentEngineSelfReport(engineHost, enginePort)
	if selfReport == "" {
		inv.warnf("认准的旁证这一格这次拿不到（%s）—— 本次只按子端回据的名字+端口认它", selfErr)
	} else if engine.Model != "" && !agentNameOverlaps(engine.Model, selfReport) {
		inv.warnf("子端回据报的模型是 %q，但那个端口上的引擎自报权重是 %q —— **两者对不上**，测的可能不是你以为的那一枚（结论按子端回据这一行认，本条照实报）",
			engine.Model, selfReport)
	}

	// ② 打引擎：`POST /v1/chat/completions`（固定形状 · temperature=0 可复跑）。
	//    为什么是 **chat 面**而不是 completion 面：思考模型的**正文只从 chat 面（带模板）出来**，
	//    completion 面拿到的是思维草稿 ⇒ 旧判据把草稿当「乱码」正是误判（`GAP-20260925-49`）。
	path := "/v1/chat/completions"
	eng := agentAddr{Host: engineHost, Port: enginePort}
	body := agentBenchBody(nPredict)
	fmt.Fprintf(stderr, "%s: agent bench %s · 认准的引擎 = 子端回据（%s）报的 %q · 端口 %d（state=%s）→ 直连 http://%s%s\n",
		progName, machine, engine.Via, model, enginePort, orDash(engine.State),
		net.JoinHostPort(engineHost, strconv.Itoa(enginePort)), path)
	status, respBody, err := agentChildDo(eng, http.MethodPost, path, body, 300*time.Second)
	if err != nil {
		inv.setErr("blocked", "engine_unreachable", fmt.Sprintf("引擎 %d 打不到（地址取自子端自报）", enginePort))
		fmt.Fprintf(stderr, "%s: 打不到引擎 http://%s%s ⇒ **不给结论**（退码 8）\n",
			progName, net.JoinHostPort(engineHost, strconv.Itoa(enginePort)), path)
		fmt.Fprintf(stderr, "原因: %v\n", err)
		fmt.Fprintf(stderr, "口径：引擎端口是**子端自报**的（子端引擎多为 127.0.0.1 监听 ⇒ 本命令今天只对**本机子端**跑得通；跨机要子端提供转发面）\n")
		fmt.Fprintf(stderr, "error.kind=blocked · detail=engine_unreachable · retryable=true · remedy=fix_precondition\n")
		return exitBlocked
	}
	if status != http.StatusOK {
		// 退码纪律 0/2/8：引擎没答应 ⇒ **测不到** = 前置不在（8），不给结论；引擎原样响应照贴。
		inv.setErr("blocked", fmt.Sprintf("engine_http_%d", status), "引擎没回 200（chat 面）")
		fmt.Fprintf(stderr, "%s: POST http://%s%s ⇒ HTTP %d\n",
			progName, net.JoinHostPort(engineHost, strconv.Itoa(enginePort)), path, status)
		fmt.Fprintf(stderr, "引擎原样响应: %s\n", strings.TrimSpace(respBody))
		fmt.Fprintf(stderr, "引擎没答应这一格 ⇒ **不给结论**（退码 8）；若该引擎不吃 chat 面，先 `%s engine argv` 那一类面（若能拿）核它的 --chat-template/--jinja\n", progName)
		fmt.Fprintf(stderr, "error.kind=blocked · detail=engine_http_%d · retryable=true · remedy=fix_precondition\n", status)
		return exitBlocked
	}
	var resp jsonObj
	if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
		inv.setErr("blocked", "bad_json", "引擎回了非 JSON")
		fmt.Fprintf(stderr, "%s: 引擎 %s 的响应不是合法 JSON（%v）⇒ 不给结论（退码 8）\n", progName, path, err)
		fmt.Fprintf(stderr, "引擎原样响应: %s\n", strings.TrimSpace(respBody))
		return exitBlocked
	}
	timings := asObj(resp["timings"])
	// 正文（`content`）与思考（`reasoning_content`）**分两格**取 —— 思考模型的两段不许混在一格判。
	content, reasoning, finish := "", "", ""
	if ch := asList(resp["choices"]); len(ch) > 0 {
		if c0 := asObj(ch[0]); c0 != nil {
			finish = cell(c0["finish_reason"])
			if msg := asObj(c0["message"]); msg != nil {
				content = cell(msg["content"])
				reasoning = cell(msg["reasoning_content"])
			}
			if content == "" && reasoning == "" {
				content = cell(c0["text"]) // 有的引擎 chat 面也回 `text` 那格（照原样认）
			}
		}
	}
	// `reasoning_content` 缺席、思考被内联进 `content`（`reasoning_format=none`/渠道标记）⇒ 现劈开：
	// 正文那一格只留正文，`<think>` 里的东西收进思考那一格。
	inlineBody, inlineThink := agentBenchSplitThink(content)
	content = inlineBody
	if inlineThink != "" {
		if reasoning != "" {
			reasoning += "\n"
		}
		reasoning += inlineThink
	}
	genTPS := cell(timings["predicted_per_second"])
	preTPS := cell(timings["prompt_per_second"])
	predN := cell(timings["predicted_n"])
	promptN := cell(timings["prompt_n"])
	if genTPS == "" || predN == "" {
		inv.setErr("blocked", "timings_absent", "引擎响应里没有 timings 几格")
		fmt.Fprintf(stderr, "%s: 引擎响应里**没有 `timings`**（或它缺席 predicted_per_second/predicted_n）⇒ 测速那几格给不出（退码 8）\n", progName)
		fmt.Fprintf(stderr, "不做的：不拿总耗时自己算一个假 tok/s（口径：只认引擎自报的 timings）\n")
		return exitBlocked
	}

	// ③ **三态判据**：可读正文 / 只有思考（推理预算不够）/ 真乱码 —— 三态各自成立、不许塌成一格。
	verdict, readable, reasons := agentBenchClassify(content, reasoning, finish)
	for _, r := range reasons {
		inv.warnf("%s：%s", agentBenchVerdictName(verdict), r)
	}
	row := map[string]string{
		"machine": machine, "model": model,
		"engine_host": engineHost, "engine_port": strconv.Itoa(enginePort),
		"engine_selfreport": selfReport, "path": path, "n_predict": strconv.Itoa(nPredict),
		"prompt_n": promptN, "predicted_n": predN,
		"prompt_per_second": preTPS, "predicted_per_second": genTPS,
		"finish_reason": finish, "verdict": verdict, "readable": strconv.FormatBool(readable),
		"content_chars":   strconv.Itoa(len([]rune(strings.TrimSpace(content)))),
		"reasoning_chars": strconv.Itoa(len([]rune(strings.TrimSpace(reasoning)))),
		"text":            agentSnippet(strings.TrimSpace(content), 120),
		"reasoning":       agentSnippet(strings.TrimSpace(reasoning), 120),
	}
	if !inv.jsonGiven {
		fmt.Fprintf(stderr, "%s: 生成 %s tok/s · 预填 %s tok/s · predicted_n %s · finish_reason=%s · 预算 n_predict=%d\n",
			progName, genTPS, preTPS, predN, orDash(finish), nPredict)
		fmt.Fprintf(stderr, "三态判据: %s（正文 %s 字 / 思考 %s 字）\n",
			agentBenchVerdictName(verdict), row["content_chars"], row["reasoning_chars"])
		fmt.Fprintf(stderr, "正文原文（截 120 字）: %s\n", row["text"])
		if row["reasoning_chars"] != "0" {
			fmt.Fprintf(stderr, "思考原文（截 120 字 · reasoning_content）: %s\n", row["reasoning"])
		}
		for _, r := range reasons {
			fmt.Fprintf(stderr, "⚠ %s: %s\n", agentBenchVerdictName(verdict), r)
		}
	}
	rcOut := listCmd(inv, stdout, stderr, agentBenchDisplay, []map[string]string{agentFillRow(row, agentBenchFields)})
	if rcOut != exitOK {
		return rcOut
	}
	// ④ 退码：**测到了就是 0** —— 三态（可读正文 / 只有思考 / 真乱码）都写进 `verdict` + `warnings[]`，
	//    **引擎输出差 ≠ 命令错**：命令面照实报，要不要拦是**消费方**按 `verdict` 判的事（`GAP-20260925-49`
	//    的「退 1」按本批退码纪律 0/2/8 改为 rc=0 + 警告；`garbled` 这一格是**结论**，不是命令失败）。
	return exitOK
}

// agentRunningEgg —— 子端**回据**里那一枚在跑的卵（名字 + 端口 + 状态 + 这条回据是哪个端点给的）。
//
// 为什么要把「名字」一起带着走：旧版只取**第一个** `port>0` 的卵 ⇒ 多枚在跑时会「抓碰巧第一个」
// （`GAP-20260925-49` 的教训）—— 认准的对象必须是**按名字/端口从回据里认出**的那一枚。
type agentRunningEgg struct {
	Host  string
	Model string
	Port  int
	State string
	Via   string
}

// agentRunningEggs 问子端要**在跑（managed 且 port>0）**的每一枚卵：
//
//	① `GET /eggs`（卵清单，带 `port` 与 `state`）—— 逐枚收，**不是**「取第一个就返回」；
//	② `GET /status` 的 `port`（子端自报的当前引擎端口，0 = 没在跑）—— 兜底合成一条。
//
// 两处都问不到 ⇒ `blocked` 退 8：**取不到就说取不到，不许编一个端口**。
func agentRunningEggs(inv *invocation, machine string, a agentAddr, stderr io.Writer) ([]agentRunningEgg, int) {
	var eggs jsonObj
	status, body, err := agentChildDo(a, http.MethodGet, "/eggs", "", 15*time.Second)
	if err != nil {
		return nil, agentChildUnreachable(inv, a, "/eggs", err, stderr)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, agentChildAuthFail(inv, a, "/eggs", status, stderr)
	}
	if status == http.StatusOK {
		if err := json.Unmarshal([]byte(body), &eggs); err != nil {
			fmt.Fprintf(stderr, "%s: 子端 /eggs 的响应不是合法 JSON（%v）—— 退一步问 /status\n", progName, err)
			eggs = nil
		}
	}
	running := []agentRunningEgg{}
	if eggs != nil {
		for _, it := range asList(eggs["eggs"]) {
			o := asObj(it)
			if o == nil {
				continue
			}
			port, _ := strconv.Atoi(cell(o["port"]))
			managed := o["managed"]
			isManaged := managed == nil || cell(managed) == "true"
			if port > 0 && isManaged {
				running = append(running, agentRunningEgg{
					Host: a.Host, Model: cell(o["model"]), Port: port,
					State: cell(o["state"]), Via: "/eggs",
				})
			}
		}
	}
	if len(running) > 0 {
		return running, exitOK
	}
	var st jsonObj
	status, body, err = agentChildDo(a, http.MethodGet, "/status", "", 15*time.Second)
	if err != nil {
		return nil, agentChildUnreachable(inv, a, "/status", err, stderr)
	}
	if status != http.StatusOK {
		inv.setErr("upstream_error", fmt.Sprintf("http_%d", status), "子端 /status 没回 200")
		fmt.Fprintf(stderr, "%s: 子端 /status ⇒ HTTP %d ⇒ 端口问不出来（退码 8）\n", progName, status)
		return nil, exitBlocked
	}
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		inv.setErr("blocked", "status_unparseable", "子端 /status 的响应不是合法 JSON")
		fmt.Fprintf(stderr, "%s: 子端 /status 的响应不是合法 JSON（%v）⇒ 端口问不出来（退码 8）\n", progName, err)
		return nil, exitBlocked
	}
	if port, _ := strconv.Atoi(cell(st["port"])); port > 0 {
		return []agentRunningEgg{{Host: a.Host, Model: cell(st["model"]), Port: port, State: cell(st["backend_state"]), Via: "/status"}}, exitOK
	}
	inv.setErr("blocked", "engine_port_unknown", "子端 /eggs 与 /status 都没报出在跑的引擎端口")
	fmt.Fprintf(stderr, "%s: 子端 %s 的 `/eggs`（managed 在跑的那一枚）与 `/status.port` **都没报出引擎端口** ⇒ 拿不到\n",
		progName, machine)
	fmt.Fprintf(stderr, "⇒ **取不到就说取不到**（退码 8）—— 本命令**不编**端口、也不自己起引擎（§九 M3 铁律：调 egg 一律经子端）\n")
	fmt.Fprintf(stderr, "现在能做的：先 `%s egg run <卵 id>` 把引擎装起来，再回来测速\n", progName)
	fmt.Fprintf(stderr, "error.kind=blocked · detail=engine_port_unknown · retryable=true · remedy=fix_precondition\n")
	return nil, exitBlocked
}

// agentEngineLine 一枚卵的人读一行（列清单与报错共用）。
func agentEngineLine(e agentRunningEgg) string {
	s := fmt.Sprintf("%s（端口 %d", e.Model, e.Port)
	if e.State != "" {
		s += " · state=" + e.State
	}
	return s + "）"
}

func agentEngineList(eggs []agentRunningEgg) string {
	ls := make([]string, 0, len(eggs))
	for _, e := range eggs {
		if e.Model == "" {
			e.Model = "<未自报模型名>"
		}
		ls = append(ls, agentEngineLine(e))
	}
	return strings.Join(ls, " · ")
}

// agentEnginePick 从**子端回据**里认准要测的那一枚引擎（**不许只看名字/端口、更不许抓碰巧第一个**）：
//
//	点名 `--model <名>`：逐字认那一条在跑的卵；它没在跑 ⇒ 再分清「注册表里没有」（2 · 不是错、是没有）
//	                    与「有但没装载」（8 · 前置不在），两条都**点名可选值**。
//	没点名：在跑的**只有一枚** ⇒ 认它；**多枚** ⇒ 退 2 要你点名（**不替你挑**碰巧第一个）。
func agentEnginePick(inv *invocation, machine string, a agentAddr, want string, stderr io.Writer) (agentRunningEgg, int) {
	eggs, rc := agentRunningEggs(inv, machine, a, stderr)
	if rc != exitOK {
		return agentRunningEgg{}, rc
	}
	if want != "" {
		for _, e := range eggs {
			if e.Model == want {
				return e, exitOK
			}
		}
		names, nrc := agentChildModelNames(inv, a, stderr)
		if nrc != exitOK {
			return agentRunningEgg{}, nrc
		}
		if !contains(names, want) {
			inv.setErr("usage", "no_such_model", fmt.Sprintf("子端注册表里没有模型 %q", want))
			fmt.Fprintf(stderr, "%s: 子端 %s 的注册表里没有模型 %q ⇒ 退码 2（**不是错、是没有**）\n", progName, machine, want)
			if s := nearestName(want, names); s != "" {
				fmt.Fprintf(stderr, "最像的合法输入: %s\n", s)
			}
			fmt.Fprintf(stderr, "可选模型（子端注册表）: %s\n", strings.Join(names, " · "))
			return agentRunningEgg{}, exitUsage
		}
		inv.setErr("blocked", "model_not_loaded", fmt.Sprintf("子端在跑的是 %s，不是 %q", agentEngineList(eggs), want))
		fmt.Fprintf(stderr, "%s: 子端**在跑**的引擎是 %s —— **不是** %q ⇒ 不给结论（退码 8）\n",
			progName, agentEngineList(eggs), want)
		fmt.Fprintf(stderr, "点名了就不再测别的：本命令**不回退去测在跑的那一枚**（测错对象比测不到更坏）\n")
		fmt.Fprintf(stderr, "本命令不替你换装载（换 = 动生产）：先 `%s egg run %s@%s` 装它，再回来测\n", progName, want, machine)
		fmt.Fprintf(stderr, "error.kind=blocked · detail=model_not_loaded · retryable=true · remedy=fix_precondition\n")
		return agentRunningEgg{}, exitBlocked
	}
	if len(eggs) == 1 {
		return eggs[0], exitOK
	}
	inv.setErr("usage", "engine_ambiguous", fmt.Sprintf("子端有 %d 枚引擎在跑、没点名测哪一枚", len(eggs)))
	fmt.Fprintf(stderr, "%s: 子端 %s 有 **%d 枚**引擎在跑：%s\n", progName, machine, len(eggs), agentEngineList(eggs))
	fmt.Fprintf(stderr, "⇒ 退码 2：**指名道姓**再测（本命令不替你挑「碰巧第一个」—— 测错对象比测不到更坏 · `GAP-20260925-49`）\n")
	fmt.Fprintf(stderr, "例: %s agent bench %s --model %s\n", progName, machine, eggs[0].Model)
	return agentRunningEgg{}, exitUsage
}

// agentEngineSelfReport 那台**引擎自己**说它载的是哪份权重：`GET /props` 的 `model_path` 优先，
// 退一步 `GET /v1/models` 的 `data[0].id`（都按件名报）。取不到 ⇒ 空串 + 一句为什么（**不编**）。
func agentEngineSelfReport(host string, port int) (string, string) {
	eng := agentAddr{Host: host, Port: port}
	if st, body, err := agentChildDo(eng, http.MethodGet, "/props", "", 10*time.Second); err == nil && st == http.StatusOK {
		var p jsonObj
		if json.Unmarshal([]byte(body), &p) == nil {
			if s := cell(p["model_path"]); s != "" {
				return filepath.Base(s), ""
			}
		}
	}
	if st, body, err := agentChildDo(eng, http.MethodGet, "/v1/models", "", 10*time.Second); err == nil && st == http.StatusOK {
		var m jsonObj
		if json.Unmarshal([]byte(body), &m) == nil {
			if list := asList(m["data"]); len(list) > 0 {
				if o := asObj(list[0]); o != nil {
					if s := cell(o["id"]); s != "" {
						return filepath.Base(s), ""
					}
				}
			}
		}
	}
	return "", "引擎没回 `/props.model_path`、也没回 `/v1/models[0].id`"
}

// agentNameOverlaps 名字 ↔ 件名是否**对得上**（任一方向包含即算 —— 件名常带量化/来源后缀，
// 例：`Qwen3.8-27B` ⟷ `Qwen3.8-27B-Q4_K_M-vcruz305.gguf`）。大小写不敏感。
func agentNameOverlaps(name, file string) bool {
	n, f := strings.ToLower(strings.TrimSpace(name)), strings.ToLower(strings.TrimSpace(file))
	if n == "" || f == "" {
		return true
	}
	return strings.Contains(f, n) || strings.Contains(n, f)
}

// agentChildModelNames 子端注册表的模型名清单（`GET /status` 的 `models` —— 子端自报，不猜）。
func agentChildModelNames(inv *invocation, a agentAddr, stderr io.Writer) ([]string, int) {
	status, body, err := agentChildDo(a, http.MethodGet, "/status", "", 15*time.Second)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 打不到子端 %s/status（%v）⇒ 拿不到模型清单（退码 8）\n", progName, agentChildBase(a), err)
		return nil, exitBlocked
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, agentChildAuthFail(inv, a, "/status", status, stderr)
	}
	if status != http.StatusOK {
		fmt.Fprintf(stderr, "%s: 子端 /status ⇒ HTTP %d ⇒ 拿不到模型清单（退码 8）\n", progName, status)
		return nil, exitBlocked
	}
	var st jsonObj
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		fmt.Fprintf(stderr, "%s: 子端 /status 不是合法 JSON（%v）⇒ 拿不到模型清单（退码 8）\n", progName, err)
		return nil, exitBlocked
	}
	names := []string{}
	for _, it := range asList(st["models"]) {
		if s := cell(it); s != "" {
			names = append(names, s)
		}
	}
	sort.Strings(names)
	return names, exitOK
}

// agentBenchBody 测速请求体（**固定形状** ⇒ 可复跑：temperature=0、不采样、不流式）。
//
// 打的是 **chat 面**（`POST /v1/chat/completions` + `max_tokens`）—— **思考模型的正文只从这条路
// 出来**：走 completion 面（`prompt`/`n_predict`）拿到的是**思维草稿**，旧判据把草稿当「乱码」
// 正是 `GAP-20260925-49` 的病根。
func agentBenchBody(nPredict int) string {
	return fmt.Sprintf("{\"messages\":[{\"role\":\"user\",\"content\":%s}],\"max_tokens\":%d,\"temperature\":0,\"stream\":false}",
		jstr("用一句话说明虫族是什么。"), nPredict)
}

// agentBenchSplitThink 把**内联在 `content` 里**的思考劈出来（`reasoning_format=none`/渠道标记的引擎
// 会把 ` thinking…` 原样塞进 content）⇒ 返回（正文, 思考）。没有内联思考 ⇒ （原样, 空串）。
//
// 为什么必须劈：三态判据要看的是「**正文**空不空」—— 不劈就会把「思考已出、正文未到」误判成
// 「正文里全是乱码」（`GAP-20260925-49` 的误判现场）。
func agentBenchSplitThink(content string) (string, string) {
	const open, closeTag = "<think", "</think>"
	i := strings.Index(content, open)
	if i < 0 {
		return content, ""
	}
	// `<think` 之后可能带 `>` 与渠道名 ⇒ 从它往后找到闭合标记。
	after := content[i+len(open):]
	gt := strings.Index(after, ">")
	if gt < 0 {
		return content, "" // 形状认不出来 ⇒ 原样，不劈（**不猜**）
	}
	rest := after[gt+1:]
	j := strings.Index(rest, closeTag)
	if j < 0 {
		// 只开了没闭（预算在思考中途烧完）⇒ 整段都是思考、正文只留 `<think` 之前那截。
		return strings.TrimSpace(content[:i]), strings.TrimSpace(rest)
	}
	think := rest[:j]
	body := strings.TrimSpace(content[:i] + rest[j+len(closeTag):])
	return body, strings.TrimSpace(think)
}

// agentBenchVerdictName 三态判据的**人读名**（机器面用 `verdict` 那格的原值；两处**同一处定义**）。
func agentBenchVerdictName(verdict string) string {
	switch verdict {
	case "readable":
		return "可读正文"
	case "reasoning_only":
		return "只有思考（推理预算不够，未到正文）"
	case "garbled":
		return "真乱码"
	case "empty":
		return "空输出（正文与思考都没有）"
	}
	return verdict
}

// agentBenchClassify —— **三态判据**（`GAP-20260925-49`），三条各自成立、**不许塌成一格**：
//
//	readable       有正文、且正文过乱码自检 ⇒ `readable=true`（**可读正文**）
//	reasoning_only 正文为空、思考非空     ⇒ **只有思考：推理预算不够，还没烧到正文**（**不是乱码** ✗）
//	garbled        正文非空但像乱码       ⇒ **真乱码**
//	empty          正文与思考都空         ⇒ 引擎什么也没吐（空 ≠ 乱码）
//
// 返回（verdict, readable, reasons）。reasons 逐条进 `warnings[]` —— **三态都退 0**（§硬性④⑤：
// 引擎输出差 ≠ 命令错，命令面照实报，要不要拦是消费方按 `verdict` 判的事）。
func agentBenchClassify(content, reasoning, finish string) (string, bool, []string) {
	body := strings.TrimSpace(content)
	think := strings.TrimSpace(reasoning)
	if body == "" {
		if think == "" {
			return "empty", false, []string{"引擎正文与思考**都是空的**（连思维草稿都没有）—— 空 ≠ 乱码，但也没有可读正文"}
		}
		why := fmt.Sprintf("正文 0 字 / 思考 %d 字 · finish_reason=%s —— **推理预算不够，还没烧到正文**（思考模型常要 ~570 token 才吐正文；这是判据的**第二态**，不是乱码）",
			len([]rune(think)), orDash(finish))
		if finish != "length" {
			why += "；且 finish_reason 不是 length（引擎自己停了）—— 同样**只有思考**"
		}
		why += "｜加大预算再测：`agent bench <机> --n-predict 1024`"
		return "reasoning_only", false, []string{why}
	}
	if rs := agentBenchGarble(body); len(rs) > 0 {
		return "garbled", false, rs
	}
	rs := []string{}
	if agentBenchHanCount(body) == 0 {
		rs = append(rs, "正文里一个汉字也没有（提示词是中文）—— **口径提示**：认不认得出是在回答由人看；**不改判据**，正文照样按可读过（旧口径把它与乱码混成一格，本批拆开）")
	}
	return "readable", true, rs
}

// agentBenchGarble 正文的乱码自检（**只判正文** —— 思维草稿不过这道闸 ✗）：
//
//	① 非中文/非英文的乱码字符占比 > 2%（泰卢固文、带变音符的拉丁乱拼之类 —— 旧的同一口径）；
//	② 通篇**没有一个汉字、也没有一个 ≥3 字母的拉丁词** —— 只剩标点/单字母的符号糊。
//
// 命中 ⇒ 真乱码（第三态）。**全英文的干净回答不算乱码**（那是判据①之外的另一回事，另起口径提示）。
func agentBenchGarble(body string) []string {
	rs := []rune(body)
	if len(rs) == 0 {
		return []string{"正文为空（连一个字都没有）"}
	}
	bad, han, latinWords := 0, 0, 0
	for _, r := range rs {
		switch {
		case r == '\n' || r == '\r' || r == '	':
		case r >= 0x20 && r <= 0x7E:
		case unicode.Is(unicode.Han, r):
			han++
		case r == 0x3000 || r == 0x3001 || r == 0x3002 ||
			(r >= 0x3008 && r <= 0x3011) || r == 0x3014 || r == 0x3015 ||
			r == 0xFF01 || r == 0xFF08 || r == 0xFF09 || r == 0xFF0C ||
			r == 0xFF1A || r == 0xFF1B || r == 0xFF1F ||
			r == 0x201C || r == 0x201D || r == 0x2018 || r == 0x2019 ||
			r == 0x2026 || r == 0x2014:
		default:
			bad++
		}
	}
	for _, w := range strings.FieldsFunc(body, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '\'')
	}) {
		if len(w) >= 3 {
			latinWords++
		}
	}
	reasons := []string{}
	if ratio := float64(bad) / float64(len(rs)); ratio > 0.02 {
		reasons = append(reasons, fmt.Sprintf("非中文/非英文乱码字符 %d/%d（%.1f%%）超过 2%% 阈值", bad, len(rs), ratio*100))
	}
	if han == 0 && latinWords == 0 {
		reasons = append(reasons, "正文里既没有一个汉字、也没有一个 ≥3 字母的拉丁词（只剩符号/单字母的糊）")
	}
	return reasons
}

// agentBenchHanCount 正文里的汉字数（口径提示用）。
func agentBenchHanCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			n++
		}
	}
	return n
}

// agentSnippet 截一段文本给人看（按**字符**截，不切坏 UTF-8）。
func agentSnippet(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}

// agentStamp 今天（本地时区 yyyymmdd）—— 备份件名与 note 的时间戳同源一处。
func agentStamp() string { return time.Now().Format("20060102") }

// agentSHA256 取件的 sha256（写前/回滚后的对拍用）。
func agentSHA256(b []byte) string {
	s := sha256.Sum256(b)
	return fmt.Sprintf("%x", s)
}

// ---- ④ 收尾：本族命令名（`N4` 硬占位表的落点面 · 与 eggCocoonNames 同形）----

func agentOpsNames() []string {
	names := []string{}
	for _, c := range catalog() {
		if len(c.path) == 2 && c.path[0] == "agent" {
			names = append(names, strings.Join(c.path, " "))
		}
	}
	return names
}
