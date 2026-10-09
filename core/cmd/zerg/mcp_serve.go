// mcp_serve.go —— `mcp/mcp_zerg_server.py`（208 行 · FastMCP · 8 工具）的 **真 Go 化实现件**。
//
// ═══════════════════ 本件在 6 步里的位置（设计稿《mcp_zerg_server-真Go化-先期设计-20261008.md》）═══════════════════
//
//	★ **步 1（写实现）与步 3（命令树接线）均已落** —— 接线在 `main.go`（`mcp serve` → `cmdMCPServeFamily` → `cmdMCPServe` → `mcpServeMain`；收口检查点10 · `cae5fb90`）。本件**不碰** `main.go`、
//	  不碰 `gate_inproc_table.go` / `precommit-gates.sh` / `publish/**` / `adopt-retire.json`。
//	  退场（步 6）由他路做；本件仍只承载 8 工具的实现与自测。
//
// ═══════════════════ 协议面：复用仓内既有依赖（✗ 不引新第三方模块）═══════════════════
//
//	· 传输：**默认 stdio** —— 与 Python `mcp.run()` 默认逐字同（消费方零改动）。
//	· 协议：`github.com/mark3labs/mcp-go/server`（**已在 `core/go.mod` 直接依赖 v0.58.0**，
//	  客户端侧 `core/internal/agent/mcp_manager.go` 已在用）⇒ 服务端沿用**同一模块**，不手写 JSON-RPC、
//	  不引第二套 MCP 库（守「不另起第二真源」）。
//	· ★ **stdout 铁律**：stdio 档下 stdout 是 MCP 协议专用通道 —— 命令面**绝不**往 stdout 写 CLI 信封/横幅；
//	  所有启动信息/告警一律走 **stderr**（照仓内 `zerg help long-tasks` 同一条口径）。
//
// ═══════════════════ 8 工具 → Go（逐条映射；工具名/参数名/返回体逐字不变 —— 对外兼容面）═══════════════════
//
//	| 工具             | 参数            | 端点                    | 超时 | 备注                          |
//	|------------------|-----------------|-------------------------|------|-------------------------------|
//	| zerg_help        | —               | 本地拼串                | —    | 工具清单自述                   |
//	| fleet_status     | —               | GET /api/fleet/status   | 30s  | 重塑 machines 摘要 + 两个计数  |
//	| list_models      | —               | GET /api/fleet/status   | 30s  | available_models + machine_models |
//	| get_model_info   | model           | GET /api/fleet/status   | 30s  | 现读过滤；不在列表 ⇒ success:false |
//	| fleet_topology   | —               | GET /api/fleet/status   | 30s  | ★ host 从名册取（GAP-73 拍 B） |
//	| load_model       | machine, model  | POST /api/control/load  | 120s | 前置校验 machine ∈ 名册        |
//	| unload_model     | machine, model  | POST /api/control/unload| 60s  | ★ 体裁到 {machine}（真契约）   |
//	| switch_model     | machine, model  | POST /api/control/load  | 120s | 同 load_model（语义=换模型）   |
//
//	· 统一返回体 `{success, message, data}` —— 逐字保住消费方解析口径。
//	· **端点/令牌真源复用 `client.go`**：`newClient()` 给 base（`statepath.CoreBaseURL()`，ZERG_PORT→8580）
//	  与 token（`credentials.go` 唯一链 `ZERG_TOKEN` > `~/.zerg/token`）；本件**不新造第二套** HTTP 客户端真源。
//	· **错误文案逐字照 Python**（`_call` 的 `连接主控失败: …` / `主控返回 N: …`）—— 这是「与 py 对拍」的判据面。
//
// ═══════════════════ ★ 两处**有意**设计变更（照设计稿 §3.4/§3.7 · 与 Python 现读分家，已逐条记账）═══════════════════
//
//	① **host 真源**（设计稿 §3.7 · GAP-73 拍定 B）：`fleet_topology` 的机器地址 + `machine` 合法集
//	   **从名册取** —— 读 `gateway/fleet.yaml` 的 `fleet:` 段（现读 4 机：node-a/node-b/node-c/node-local）。
//	   `fleet.yaml` 自陈该段是**投影**（真源 = `~/.zerg/contexts/default.yaml` 的 `nodes:`，由门
//	   `check-nodes-roster.py` 逐项对账）。⇒ 本件照 B 读 `fleet.yaml`，✗ **不碰** `core/internal/store`。
//	   读不到 ⇒ 退 **8**（不给结论），✗ **不回退硬编码**（硬编码=假绿）。
//	② Python 硬编码 `{node-a,local,node-b}` **已与现名册漂**（无 `local`、缺 `node-c/node-local`）⇒ Go 版合法集 =
//	   名册机名；`local` 不再合法，`node-c`/`node-local` 转为合法。
//
// ═══════════════════ `--json` / 退码口径（设计稿 §3.6）═══════════════════
//
//	· `--json` 前置拒收 **rc=2**（理由：本命令的机器面 = MCP 协议本身，命令面**没有**第二张 `--json` 信封面）。
//	· 退码只表「服务器本身能不能起」：0 干净退出 · 2 用法错 · 8 名册读不到 · 1 收尾失败。
//	· ★ 运行期错误（主控打不到/未认证）一律走**工具结果** `{success:false,…}`，**进程不退**（照 Python）。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"
)

// ── 对外兼容面：8 工具名（逐字不变 —— 客户端暴露给模型的函数签名靠它）────────────────────────
const (
	mcpToolHelp         = "zerg_help"
	mcpToolFleetStatus  = "fleet_status"
	mcpToolListModels   = "list_models"
	mcpToolGetModelInfo = "get_model_info"
	mcpToolFleetTopo    = "fleet_topology"
	mcpToolLoadModel    = "load_model"
	mcpToolUnloadModel  = "unload_model"
	mcpToolSwitchModel  = "switch_model"
)

// mcpServerVersion —— MCP server 自报名版本（`NewMCPServer` 要求）。
const mcpServerVersion = "1.0.0"

// ── 文案常量（★ 中文一律只出现在 `const`/注释行 —— 自指前瞻消解：日志调用行零中文字面量）────────
const (
	mcpMsgHelpMessage = "虫族控制面工具：fleet_status / list_models / get_model_info / fleet_topology / load_model / unload_model / switch_model"

	mcpMsgNoJSON     = "⛔ 本命令的机器面 = MCP 协议本身（stdout 是协议通道），命令面没有第二张 `--json` 信封面（退码 2）"
	mcpMsgUnknownFlg = "zerg: 不认的旗标 %s（退码 2）"
	mcpMsgUnknownArg = "zerg: 不认的位置参数 %s（退码 2）"
	mcpMsgNoRoot     = "zerg: 解析不到仓根 ⇒ 读不到名册（退码 8 · 不给结论）"
	mcpMsgNoRoster   = "zerg: 名册读不到 gateway/fleet.yaml（%v）⇒ 不给结论（退码 8 · ✗ 不回退硬编码）"
	mcpMsgServeErr   = "zerg: MCP server 收尾失败（%v）"

	mcpMsgConnFail   = "连接主控失败: %v"
	mcpMsgHTTPFail   = "主控返回 %d: %s"
	mcpMsgModelGone  = "模型 %s 不在可用列表（共 %d 个）"
	mcpMsgModelInfo  = "模型 %s 详情"
	mcpMsgTopoOK     = "集群拓扑（直连为主，WiFi 已放弃）"
	mcpMsgMachineBad = "machine 必须是 %s，收到 %s"

	mcpHelpText = "用法：zerg mcp serve\n\n" +
		"虫族控制面 MCP server（默认 stdio）。\n" +
		"stdout 是 MCP 协议通道 —— 命令面没有 --json（退码 2）；进度/日志一律走 stderr。\n" +
		"8 个工具：zerg_help / fleet_status / list_models / get_model_info / fleet_topology / " +
		"load_model / unload_model / switch_model\n"
)

// ═══════════════════ Python 兼容 JSON 编码器 ═══════════════════
//
//	Python `json.dumps(..., ensure_ascii=False)` 的三条口径（对齐后才能「与 py 逐字对拍」）：
//	  ① 分隔符 = `", "` / `": "`（Go `encoding/json` 是 `","`/`":"` ✗）；
//	  ② 非 ASCII **原样输出**（不 `\uXXXX`）；只转义 `"`/`\`/控制字符（不转义 `<>&`）；
//	  ③ 键序 = 插入序 ⇒ 输出结构一律用**有序类型**（`mcpObjOut`/`mcpArrOut`），✗ 不用裸 map。

// mcpKVOut —— 有序对象的一个键值对。
type mcpKVOut struct {
	k string
	v any
}

// mcpObjOut —— 有序对象（对应 Python dict）。
type mcpObjOut []mcpKVOut

// mcpArrOut —— 数组（对应 Python list）。
type mcpArrOut []any

// mcpObjGet —— 从有序对象取字段（找不到 ⇒ nil，对应 Python `d.get(k)`）。
func mcpObjGet(o any, key string) any {
	if obj, ok := o.(mcpObjOut); ok {
		for _, kv := range obj {
			if kv.k == key {
				return kv.v
			}
		}
	}
	return nil
}

// mcpPyJSON —— 把有序结构编成 Python 口径的 JSON 串（无尾随换行）。
func mcpPyJSON(v any) string {
	var b strings.Builder
	mcpPyWrite(&b, v)
	return b.String()
}

func mcpPyWriteString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

func mcpPyWriteFloat(b *strings.Builder, f float64) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
		return
	}
	// Python repr：整数值的 float 也带 `.0`（`2.0` 而非 `2`）。
	if f == math.Trunc(f) && math.Abs(f) < 1e16 {
		b.WriteString(strconv.FormatFloat(f, 'f', 1, 64))
		return
	}
	b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
}

func mcpPyWriteNumber(b *strings.Builder, n json.Number) {
	s := n.String()
	if strings.ContainsAny(s, ".eE") {
		if f, err := n.Float64(); err == nil {
			mcpPyWriteFloat(b, f)
			return
		}
	}
	b.WriteString(s)
}

func mcpPyWrite(b *strings.Builder, v any) {
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		mcpPyWriteString(b, t)
	case int:
		b.WriteString(strconv.Itoa(t))
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
	case float64:
		mcpPyWriteFloat(b, t)
	case json.Number:
		mcpPyWriteNumber(b, t)
	case mcpObjOut:
		b.WriteByte('{')
		for i, kv := range t {
			if i > 0 {
				b.WriteString(", ")
			}
			mcpPyWriteString(b, kv.k)
			b.WriteString(": ")
			mcpPyWrite(b, kv.v)
		}
		b.WriteByte('}')
	case mcpArrOut:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteString(", ")
			}
			mcpPyWrite(b, e)
		}
		b.WriteByte(']')
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteString(", ")
			}
			mcpPyWrite(b, e)
		}
		b.WriteByte(']')
	case []string:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteString(", ")
			}
			mcpPyWrite(b, e)
		}
		b.WriteByte(']')
	case json.RawMessage:
		b.Write(t)
	default:
		raw, _ := json.Marshal(t)
		b.Write(raw)
	}
}

// mcpOrderedValue —— **保序**解析任意 JSON 值（对象→mcpObjOut · 数组→mcpArrOut · 数字→json.Number）。
func mcpOrderedValue(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	v, err := mcpOrderedToken(dec)
	if err != nil {
		return nil
	}
	return v
}

func mcpOrderedToken(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); ok {
		switch d {
		case '{':
			obj := mcpObjOut{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := kt.(string)
				val, err := mcpOrderedToken(dec)
				if err != nil {
					return nil, err
				}
				obj = append(obj, mcpKVOut{key, val})
			}
			if _, err := dec.Token(); err != nil { // 吃掉 '}'
				return nil, err
			}
			return obj, nil
		case '[':
			arr := mcpArrOut{}
			for dec.More() {
				val, err := mcpOrderedToken(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			if _, err := dec.Token(); err != nil { // 吃掉 ']'
				return nil, err
			}
			return arr, nil
		}
	}
	return tok, nil
}

// mcpArrayLen —— Python `len(list)`（非数组 ⇒ 0）。
func mcpArrayLen(v any) int {
	if arr, ok := v.(mcpArrOut); ok {
		return len(arr)
	}
	return 0
}

// mcpAsString —— 取字符串值（非字符串 ⇒ 空串）。
func mcpAsString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// mcpArrayHas —— Python `x in list`。
func mcpArrayHas(v any, want string) bool {
	arr, ok := v.(mcpArrOut)
	if !ok {
		return false
	}
	for _, e := range arr {
		if s, ok := e.(string); ok && s == want {
			return true
		}
	}
	return false
}

// mcpRound1 —— Python `round(x, 1)`；缺值（nil）⇒ 0（对应 `m.get(k, 0)`）。
//
//	★ 整数保持整数：Python `round(int, 1)` 仍是 `int`（JSON `0` 不变成 `0.0`）⇒ 无小数点/指数的
//	  `json.Number` 原样返回。
func mcpRound1(v any) any {
	switch t := v.(type) {
	case nil:
		return 0
	case json.Number:
		s := t.String()
		if !strings.ContainsAny(s, ".eE") {
			return t // 整数：Python round(int,1) 仍是 int
		}
		if f, err := t.Float64(); err == nil {
			return math.Round(f*10) / 10
		}
		return t
	case float64:
		return math.Round(t*10) / 10
	default:
		return v
	}
}

// ═══════════════════ 名册（GAP-73 拍 B：读 gateway/fleet.yaml 的 fleet: 段）═══════════════════

type mcpRosterNode struct {
	name string
	host string
	port int
	os   string
	kind string
}

type mcpRoster struct {
	order  []string
	byName map[string]mcpRosterNode
}

func (r *mcpRoster) has(name string) bool {
	_, ok := r.byName[name]
	return ok
}

func (r *mcpRoster) namesJoined(sep string) string {
	return strings.Join(r.order, sep)
}

// mcpYamlField —— 在 YAML 映射节点里按键取值（保序）。
func mcpYamlField(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// mcpLoadRoster —— 读 `<root>/gateway/fleet.yaml` 的 `fleet:` 段（★ 该段自陈是
// `~/.zerg/contexts/default.yaml nodes:` 的**投影**；本件照 B 读投影，✗ 不碰 core/internal/store）。
func mcpLoadRoster(root string) (*mcpRoster, error) {
	path := filepath.Join(root, "gateway", "fleet.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	rootNode := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		rootNode = doc.Content[0]
	}
	fleet := mcpYamlField(rootNode, "fleet")
	if fleet == nil || fleet.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("gateway/fleet.yaml 里没有 fleet: 段")
	}
	r := &mcpRoster{byName: map[string]mcpRosterNode{}}
	for i := 0; i+1 < len(fleet.Content); i += 2 {
		name := fleet.Content[i].Value
		val := fleet.Content[i+1]
		n := mcpRosterNode{name: name}
		if v := mcpYamlField(val, "host"); v != nil {
			n.host = v.Value
		}
		if v := mcpYamlField(val, "port"); v != nil {
			n.port, _ = strconv.Atoi(v.Value)
		}
		if v := mcpYamlField(val, "os"); v != nil {
			n.os = v.Value
		}
		if v := mcpYamlField(val, "kind"); v != nil {
			n.kind = v.Value
		}
		r.order = append(r.order, name)
		r.byName[name] = n
	}
	if len(r.order) == 0 {
		return nil, fmt.Errorf("gateway/fleet.yaml 的 fleet: 段为空")
	}
	return r, nil
}

// ═══════════════════ 主控调用（端点/令牌真源复用 client.go）═══════════════════

type mcpZergServer struct {
	c      *client
	roster *mcpRoster
}

// mcpCallResp —— 一次主控调用的结果（保序字段 + 原文）。
type mcpCallResp struct {
	ok     bool
	msg    string
	rawTop json.RawMessage
	fields []mcpKVOut
	body   string
}

func (r mcpCallResp) raw(key string) []byte {
	for _, kv := range r.fields {
		if kv.k == key {
			if rm, ok := kv.v.(json.RawMessage); ok {
				return rm
			}
		}
	}
	return nil
}

// call —— 照 Python `_call`：成功 `{ok, "ok", <resp>}`；HTTP>=400 回 `主控返回 N: <body[:300]>`；
// 其余（连接失败/超时）回 `连接主控失败: <err>`。base/token 取 `client` 的仓内真源；超时照 Python 逐档。
func (s *mcpZergServer) call(path, method, body string, timeout time.Duration) mcpCallResp {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, s.c.base+path, rdr)
	if err != nil {
		return mcpCallResp{msg: fmt.Sprintf(mcpMsgConnFail, err)}
	}
	req.Header.Set("X-Auth-Token", s.c.token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Do(req)
	if err != nil {
		return mcpCallResp{msg: fmt.Sprintf(mcpMsgConnFail, err)}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		msg := string(raw)
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return mcpCallResp{msg: fmt.Sprintf(mcpMsgHTTPFail, resp.StatusCode, msg)}
	}
	fields, _ := mcpOrderedFields(raw)
	return mcpCallResp{ok: true, msg: "ok", rawTop: json.RawMessage(raw), fields: fields}
}

// mcpOrderedFields —— 保序解析顶层对象的字段（值保留为 json.RawMessage）。
func mcpOrderedFields(raw []byte) ([]mcpKVOut, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, false
	}
	var out []mcpKVOut
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, _ := kt.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, false
		}
		out = append(out, mcpKVOut{key, v})
	}
	return out, true
}

// mcpResultObj —— 统一返回体 `{success, message, data}`（顺序逐字同 Python）。
func mcpResultObj(ok bool, msg string, data any) mcpObjOut {
	return mcpObjOut{{"success", ok}, {"message", msg}, {"data", data}}
}

// mcpFail —— 失败返回体（data 空对象，照 Python）。
func mcpFail(msg string) string {
	return mcpPyJSON(mcpResultObj(false, msg, mcpObjOut{}))
}

// ═══════════════════ 8 工具实现 ═══════════════════

// helpJSON —— `zerg_help`（本地拼串；machine 例子用**名册机名**，照 §3.4 与 Python 分家）。
func (s *mcpZergServer) helpJSON() string {
	machines := s.roster.namesJoined("|")
	data := mcpObjOut{
		{"fleet_status", "查看所有机器/模型/健康状态（只读）"},
		{"list_models", "列出可用模型（只读）"},
		{"get_model_info", mcpObjOut{{"model", "模型名"}, {"desc", "单模型详情（候选机器/文件/内存）"}}},
		{"fleet_topology", "集群拓扑（本机/X" + "3/mini 连接）"},
		{"load_model", mcpObjOut{{"machine", machines}, {"model", "模型名"}, {"desc", "指定机器加载模型"}}},
		{"unload_model", mcpObjOut{{"machine", machines}, {"model", "模型名"}, {"desc", "卸载模型"}}},
		{"switch_model", mcpObjOut{{"machine", machines}, {"model", "模型名"}, {"desc", "切换模型（加载新+卸载旧）"}}},
	}
	return mcpPyJSON(mcpResultObj(true, mcpMsgHelpMessage, data))
}

// fleetStatusJSON —— 重塑：`machines` 摘要 + `healthy_count`/`available_models_count`。
func (s *mcpZergServer) fleetStatusJSON() string {
	r := s.call("/api/fleet/status", http.MethodGet, "", 30*time.Second)
	if !r.ok {
		return mcpFail(r.msg)
	}
	var summary mcpArrOut
	if obj, ok := mcpOrderedValue(r.raw("machines")).(mcpObjOut); ok {
		for _, kv := range obj {
			m := kv.v
			summary = append(summary, mcpObjOut{
				{"machine", kv.k},
				{"healthy", mcpObjGet(m, "healthy")},
				{"state", mcpObjGet(m, "backend_state")},
				{"model", mcpObjGet(m, "model")},
				{"mem_available_gb", mcpRound1(mcpObjGet(m, "mem_available_gb"))},
				{"active_requests", mcpObjGet(m, "active_requests")},
			})
		}
	}
	data := mcpObjOut{
		{"healthy_count", mcpOrderedValue(r.raw("healthy_count"))},
		{"available_models_count", mcpArrayLen(mcpOrderedValue(r.raw("available_models")))},
		{"machines", summary},
	}
	return mcpPyJSON(mcpResultObj(true, "ok", data))
}

// listModelsJSON —— `available_models` + 各机 `models` 反推 `machine_models`。
func (s *mcpZergServer) listModelsJSON() string {
	r := s.call("/api/fleet/status", http.MethodGet, "", 30*time.Second)
	if !r.ok {
		return mcpFail(r.msg)
	}
	models := mcpOrderedValue(r.raw("available_models"))
	if models == nil {
		models = mcpArrOut{}
	}
	mm := mcpObjOut{}
	if obj, ok := mcpOrderedValue(r.raw("machines")).(mcpObjOut); ok {
		for _, kv := range obj {
			sub := mcpObjGet(kv.v, "models")
			if sub == nil {
				sub = mcpArrOut{}
			}
			mm = append(mm, mcpKVOut{kv.k, sub})
		}
	}
	data := mcpObjOut{
		{"count", mcpArrayLen(models)},
		{"models", models},
		{"machine_models", mm},
	}
	return mcpPyJSON(mcpResultObj(true, "ok", data))
}

// getModelInfoJSON —— 现读过滤；不在列表 ⇒ `success:false`（照 Python）。
func (s *mcpZergServer) getModelInfoJSON(model string) string {
	r := s.call("/api/fleet/status", http.MethodGet, "", 30*time.Second)
	if !r.ok {
		return mcpFail(r.msg)
	}
	available := mcpOrderedValue(r.raw("available_models"))
	if available == nil {
		available = mcpArrOut{}
	}
	if !mcpArrayHas(available, model) {
		data := mcpObjOut{{"available", available}}
		return mcpPyJSON(mcpResultObj(false, fmt.Sprintf(mcpMsgModelGone, model, mcpArrayLen(available)), data))
	}
	info := mcpObjOut{{"model", model}, {"available", true}}
	loadedOn := ""
	var state any
	found := false
	if obj, ok := mcpOrderedValue(r.raw("machines")).(mcpObjOut); ok {
		for _, kv := range obj {
			if mcpAsString(mcpObjGet(kv.v, "model")) == model {
				loadedOn, state, found = kv.k, mcpObjGet(kv.v, "backend_state"), true
			}
		}
	}
	if !found {
		info = append(info, mcpKVOut{"loaded_on", nil}, mcpKVOut{"state", "not_loaded"})
	} else {
		info = append(info, mcpKVOut{"loaded_on", loadedOn}, mcpKVOut{"state", state})
	}
	return mcpPyJSON(mcpResultObj(true, fmt.Sprintf(mcpMsgModelInfo, model), info))
}

// fleetTopologyJSON —— 拓扑；★ 机器地址从名册取（✗ 不硬编码）。
func (s *mcpZergServer) fleetTopologyJSON() string {
	r := s.call("/api/fleet/status", http.MethodGet, "", 30*time.Second)
	if !r.ok {
		return mcpFail(r.msg)
	}
	topo := mcpObjOut{}
	if obj, ok := mcpOrderedValue(r.raw("machines")).(mcpObjOut); ok {
		for _, kv := range obj {
			m := kv.v
			topo = append(topo, mcpKVOut{kv.k, mcpObjOut{
				{"healthy", mcpObjGet(m, "healthy")},
				{"model", mcpObjGet(m, "model")},
				{"mem_available_gb", mcpRound1(mcpObjGet(m, "mem_available_gb"))},
			}})
		}
	}
	conn := mcpObjOut{}
	for _, name := range s.roster.order {
		conn = append(conn, mcpKVOut{name, s.roster.byName[name].host})
	}
	data := mcpObjOut{{"machines", topo}, {"connectivity", conn}}
	return mcpPyJSON(mcpResultObj(true, mcpMsgTopoOK, data))
}

// mcpMachineInvalid —— 非法 machine 的返回体（`machine` 合法集 = 名册机名）。
func mcpMachineInvalid(r *mcpRoster, machine string) string {
	return mcpFail(fmt.Sprintf(mcpMsgMachineBad, r.namesJoined("/"), machine))
}

// loadModelJSON —— `POST /api/control/load` 体 `{machine,model}`（前置校验 ∈ 名册）。
func (s *mcpZergServer) loadModelJSON(machine, model string) string {
	if !s.roster.has(machine) {
		return mcpMachineInvalid(s.roster, machine)
	}
	body := mcpPyJSON(mcpObjOut{{"machine", machine}, {"model", model}})
	r := s.call("/api/control/load", http.MethodPost, body, 120*time.Second)
	return mcpPyJSON(mcpResultObj(r.ok, r.msg, mcpDecodedData(r)))
}

// unloadModelJSON —— ★ 体裁到 `{machine}`（真契约只收 machine；Python 多带的 `model` 主控不读）。
func (s *mcpZergServer) unloadModelJSON(machine, model string) string {
	_ = model
	if !s.roster.has(machine) {
		return mcpMachineInvalid(s.roster, machine)
	}
	body := mcpPyJSON(mcpObjOut{{"machine", machine}})
	r := s.call("/api/control/unload", http.MethodPost, body, 60*time.Second)
	return mcpPyJSON(mcpResultObj(r.ok, r.msg, mcpDecodedData(r)))
}

// switchModelJSON —— 同 `load_model`（语义=换模型）。
func (s *mcpZergServer) switchModelJSON(machine, model string) string {
	if !s.roster.has(machine) {
		return mcpMachineInvalid(s.roster, machine)
	}
	body := mcpPyJSON(mcpObjOut{{"machine", machine}, {"model", model}})
	r := s.call("/api/control/load", http.MethodPost, body, 120*time.Second)
	return mcpPyJSON(mcpResultObj(r.ok, r.msg, mcpDecodedData(r)))
}

// mcpDecodedData —— 写面成功时 `data` = 主控响应（保序解析）；失败时 `{}`。
func mcpDecodedData(r mcpCallResp) any {
	if !r.ok || len(r.rawTop) == 0 {
		return mcpObjOut{}
	}
	v := mcpOrderedValue(r.rawTop)
	if v == nil {
		return mcpObjOut{}
	}
	return v
}

// ═══════════════════ MCP server 组装（8 工具注册）═══════════════════

// newMCPZergServer —— 用仓内真源客户端造 server。
func newMCPZergServer(roster *mcpRoster) *server.MCPServer {
	return newMCPZergServerWithClient(newClient(), roster)
}

// newMCPZergServerWithClient —— 注入客户端（单测用 httptest 主控）。
func newMCPZergServerWithClient(c *client, roster *mcpRoster) *server.MCPServer {
	zs := &mcpZergServer{c: c, roster: roster}
	srv := server.NewMCPServer("zerg", mcpServerVersion)

	srv.AddTool(mcp.NewTool(mcpToolHelp,
		mcp.WithDescription("虫族 MCP 工具清单与用法。")),
		zs.handleHelp)

	srv.AddTool(mcp.NewTool(mcpToolFleetStatus,
		mcp.WithDescription("查看虫族集群所有机器的状态（健康/模型/负载/内存）。只读。")),
		zs.handleFleetStatus)

	srv.AddTool(mcp.NewTool(mcpToolListModels,
		mcp.WithDescription("列出虫族可用模型清单（含各模型候选机器）。只读。")),
		zs.handleListModels)

	srv.AddTool(mcp.NewTool(mcpToolGetModelInfo,
		mcp.WithDescription("查看单个模型的详情（候选机器/状态）。只读。参数 model: 模型名。"),
		mcp.WithString("model", mcp.Required(), mcp.Description("模型名"))),
		zs.handleGetModelInfo)

	srv.AddTool(mcp.NewTool(mcpToolFleetTopo,
		mcp.WithDescription("查看虫族集群拓扑（机器地址/直连方式）。只读。")),
		zs.handleFleetTopology)

	srv.AddTool(mcp.NewTool(mcpToolLoadModel,
		mcp.WithDescription("在指定机器加载模型。"),
		mcp.WithString("machine", mcp.Required(), mcp.Description("机器名")),
		mcp.WithString("model", mcp.Required(), mcp.Description("模型名"))),
		zs.handleLoadModel)

	srv.AddTool(mcp.NewTool(mcpToolUnloadModel,
		mcp.WithDescription("卸载指定机器上的模型。"),
		mcp.WithString("machine", mcp.Required(), mcp.Description("机器名")),
		mcp.WithString("model", mcp.Required(), mcp.Description("模型名"))),
		zs.handleUnloadModel)

	srv.AddTool(mcp.NewTool(mcpToolSwitchModel,
		mcp.WithDescription("切换指定机器上的模型（加载新模型）。"),
		mcp.WithString("machine", mcp.Required(), mcp.Description("机器名")),
		mcp.WithString("model", mcp.Required(), mcp.Description("模型名"))),
		zs.handleSwitchModel)

	return srv
}

func mcpText(s string) *mcp.CallToolResult { return mcp.NewToolResultText(s) }

func (s *mcpZergServer) handleHelp(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcpText(s.helpJSON()), nil
}

func (s *mcpZergServer) handleFleetStatus(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcpText(s.fleetStatusJSON()), nil
}

func (s *mcpZergServer) handleListModels(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcpText(s.listModelsJSON()), nil
}

func (s *mcpZergServer) handleGetModelInfo(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	model, err := req.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcpText(s.getModelInfoJSON(model)), nil
}

func (s *mcpZergServer) handleFleetTopology(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcpText(s.fleetTopologyJSON()), nil
}

func (s *mcpZergServer) mcpMachineModel(req mcp.CallToolRequest) (string, string, *mcp.CallToolResult) {
	machine, err := req.RequireString("machine")
	if err != nil {
		return "", "", mcp.NewToolResultError(err.Error())
	}
	model, err := req.RequireString("model")
	if err != nil {
		return "", "", mcp.NewToolResultError(err.Error())
	}
	return machine, model, nil
}

func (s *mcpZergServer) handleLoadModel(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	machine, model, bad := s.mcpMachineModel(req)
	if bad != nil {
		return bad, nil
	}
	return mcpText(s.loadModelJSON(machine, model)), nil
}

func (s *mcpZergServer) handleUnloadModel(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	machine, model, bad := s.mcpMachineModel(req)
	if bad != nil {
		return bad, nil
	}
	return mcpText(s.unloadModelJSON(machine, model)), nil
}

func (s *mcpZergServer) handleSwitchModel(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	machine, model, bad := s.mcpMachineModel(req)
	if bad != nil {
		return bad, nil
	}
	return mcpText(s.switchModelJSON(machine, model)), nil
}

// ═══════════════════ 传输 / 命令行入口 ═══════════════════

// mcpServeRun —— stdio 传输（默认档）：stdout 是协议通道。
func mcpServeRun(srv *server.MCPServer, ctx context.Context, in io.Reader, out io.Writer) error {
	return server.NewStdioServer(srv).Listen(ctx, in, out)
}

// mcpRefuseJSON —— `--json` / `--json=` 前置拒收 rc=2（照仓内 long-tasks 口径）。
func mcpRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	hit := false
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = true
			break
		}
	}
	if !hit {
		return 0, false
	}
	fmt.Fprintln(stderr, mcpMsgNoJSON)
	return exitUsage, true
}

// mcpServeMain —— 命令面入口（**已接线**：`main.go` 的 `mcp serve` 格 → `cmdMCPServeFamily` → `cmdMCPServe` → 本入口）。
func mcpServeMain(stdout, stderr io.Writer, in io.Reader, argv []string) int {
	if rc, refused := mcpRefuseJSON(stderr, argv); refused {
		return rc
	}
	if len(argv) == 1 && (argv[0] == "--self-test" || argv[0] == "--selftest") {
		return mcpSelfTest(stdout, stderr)
	}
	for _, a := range argv {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stdout, mcpHelpText)
			return exitOK
		}
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, mcpMsgUnknownFlg+"\n", a)
			return exitUsage
		}
		fmt.Fprintf(stderr, mcpMsgUnknownArg+"\n", a)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		fmt.Fprintln(stderr, mcpMsgNoRoot)
		return exitBlocked
	}
	roster, err := mcpLoadRoster(root)
	if err != nil {
		fmt.Fprintf(stderr, mcpMsgNoRoster+"\n", err)
		return exitBlocked
	}
	srv := newMCPZergServer(roster)
	if err := mcpServeRun(srv, context.Background(), in, stdout); err != nil {
		fmt.Fprintf(stderr, mcpMsgServeErr+"\n", err)
		return exitFail
	}
	return exitOK
}

// cmdMCPServe —— 族命令形入口（**已接线**：`main.go` 的 `mcp serve` 格经 `cmdMCPServeFamily` 调本入口）。
func cmdMCPServe(inv *invocation, stdout, stderr io.Writer) int {
	if inv != nil && inv.jsonGiven {
		fmt.Fprintln(stderr, mcpMsgNoJSON)
		return exitUsage
	}
	var argv []string
	if inv != nil {
		argv = inv.args
	}
	return mcpServeMain(stdout, stderr, os.Stdin, argv)
}

// ═══════════════════ `--self-test`（工具集 + 编码器 + 负控；不碰真主控）═══════════════════

func mcpSelfTest(stdout, stderr io.Writer) int {
	bad, total := 0, 0
	check := func(ok bool, desc, detail string) {
		total++
		mark := "✓"
		if !ok {
			bad++
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s %s\n", mark, desc, detail)
	}

	// ★ 名册（机群名→host）**从 fleet.yaml 真源派生**（✗ 不再件内写死机名/IP：那份字面量就是
	//   `gateway/fleet.yaml` 的 `fleet:` 段的**第二真源**）。派生源 = `mcpLoadRoster(repoRoot())`
	//   —— 本包读 fleet.yaml 的**唯一**入口（保 YAML 文档序 ⇒ `order` 与现读名册逐字同）。
	//   读不到 ⇒ 退 **8**（不给结论 · 照 `mcpServeMain` 同口径），✗ **不回退硬编码**（硬编码=假绿）。
	root := repoRoot()
	if root == "" {
		fmt.Fprintln(stderr, mcpMsgNoRoot)
		return exitBlocked
	}
	roster, err := mcpLoadRoster(root)
	if err != nil {
		fmt.Fprintf(stderr, mcpMsgNoRoster+"\n", err)
		return exitBlocked
	}
	srv := newMCPZergServerWithClient(newClient(), roster)
	names, err := mcpStdioToolNames(srv)
	if err != nil {
		fmt.Fprintf(stderr, "  ✗ tools/list 失败：%v\n", err)
		return exitFail
	}
	check(len(names) == 8, "tools/list 回 8 个工具", fmt.Sprintf("实得 %d：%v", len(names), names))

	got := mcpPyJSON(mcpObjOut{{"a", 1}, {"b", "中"}})
	check(got == `{"a": 1, "b": "中"}`, "Python 兼容 JSON（ensure_ascii=False + 空格分隔）", got)

	badMachine := mcpMachineInvalid(roster, "local")
	check(strings.Contains(badMachine, "local") && strings.Contains(badMachine, `"success": false`),
		"非法 machine（local）⇒ success:false 且点名", badMachine)

	fmt.Fprintf(stdout, "自检：%d/%d 通过\n", total-bad, total)
	if bad > 0 {
		return exitFail
	}
	return exitOK
}

// mcpStdioToolNames —— 管道起 stdio server，跑 initialize + tools/list，回工具名（自测/单测共用）。
func mcpStdioToolNames(srv *server.MCPServer) ([]string, error) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = mcpServeRun(srv, ctx, inR, outW) }()
	go func() {
		_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"selftest","version":"1"}}}`+"\n")
		_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
		_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`+"\n")
	}()

	type res struct {
		names []string
		err   error
	}
	ch := make(chan res, 1)
	go func() {
		dec := json.NewDecoder(outR)
		for {
			var msg struct {
				ID     any `json:"id"`
				Result struct {
					Tools []struct {
						Name string `json:"name"`
					} `json:"tools"`
				} `json:"result"`
			}
			if err := dec.Decode(&msg); err != nil {
				ch <- res{nil, err}
				return
			}
			if id, ok := msg.ID.(float64); ok && int(id) == 2 {
				var names []string
				for _, t := range msg.Result.Tools {
					names = append(names, t.Name)
				}
				ch <- res{names, nil}
				return
			}
		}
	}()
	select {
	case r := <-ch:
		return r.names, r.err
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("tools/list 超时")
	}
}
