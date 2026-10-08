// mcp_serve_test.go —— `mcp_serve.go`（8 工具真 Go 化）的单测面。
//
//	覆盖面（照设计稿步 1 验收）：
//	  ① `tools/list` 回 **8 个工具**，名字逐字同 Python（兼容面）；
//	  ② mock 主控（httptest）驱动每个 handler，断言 `{success,message,data}` 形状；
//	  ③ 负控：`machine` 非法 ⇒ `success:false`；主控打不到 ⇒ 该工具回 `success:false` 而**进程不退**；
//	  ④ 写面体裁：`load`/`switch` 体含 `{machine,model}`；`unload` 体**只** `{machine}`（真契约）。
//
//	★ 对拍说明：本件单测只做「本仓行为」断言；与 Python 的逐工具对拍见同批 `MCP_PARITY_URL`
//	面（`TestMCPParityDump`）+ 外部驱动脚本（设计稿步 4）。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// mcpTestFleetStatus —— 主控 `/api/fleet/status` 的固定夹具（值刻意含整数与小数）。
const mcpTestFleetStatus = `{"healthy_count": 2, "available_models": ["m1", "m2"], ` +
	`"machines": {` +
	`"node-a": {"healthy": true, "backend_state": "ok", "model": "m1", "mem_available_gb": 12.3, "active_requests": 1, "models": ["m1"]}, ` +
	`"Mr2109": {"healthy": false, "backend_state": "down", "model": null, "mem_available_gb": 0, "active_requests": 0, "models": ["m2"]}` +
	`}}`

func mcpTestRoster() *mcpRoster {
	return &mcpRoster{
		order: []string{"node-a", "node-b", "node-c", "Mr2109"},
		byName: map[string]mcpRosterNode{
			"node-a": {name: "node-a", host: "<worker-ip>"},
			"node-b": {name: "node-b", host: "<worker-ip>"},
			"node-c": {name: "node-c", host: "<worker-ip>"},
			"Mr2109":   {name: "Mr2109", host: "127.0.0.1"},
		},
	}
}

// mcpMockController —— 假主控：GET fleet/status 回夹具；POST load/unload 记录体并回固定串。
func mcpMockController(t *testing.T, got *[]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/fleet/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("fleet/status 期望 GET，实得 %s", r.Method)
		}
		_, _ = io.WriteString(w, mcpTestFleetStatus)
	})
	mux.HandleFunc("/api/control/load", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*got = append(*got, "load "+string(b))
		_, _ = io.WriteString(w, `{"accepted": true, "task_id": "t1"}`)
	})
	mux.HandleFunc("/api/control/unload", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*got = append(*got, "unload "+string(b))
		_, _ = io.WriteString(w, `{"ok": true}`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func mcpTestClient(base string) *client {
	return &client{base: base, token: "test-token", hc: &http.Client{Timeout: 2 * time.Second}}
}

func mcpDecodeMap(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("返回体不是合法 JSON：%v\n%s", err, s)
	}
	return m
}

// TestMCPToolsList —— `tools/list` 回 8 个工具，名字逐字同 Python。
func TestMCPToolsList(t *testing.T) {
	srv := newMCPZergServerWithClient(newClient(), mcpTestRoster())
	names, err := mcpStdioToolNames(srv)
	if err != nil {
		t.Fatalf("tools/list 失败：%v", err)
	}
	sort.Strings(names)
	want := []string{
		"fleet_status", "fleet_topology", "get_model_info", "list_models",
		"load_model", "switch_model", "unload_model", "zerg_help",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("工具集不符\n实得：%v\n期望：%v", names, want)
	}
}

// mcpTestListToolsRaw —— 管道跑 tools/list，回每个工具的原始对象（含 inputSchema）。
func mcpTestListToolsRaw(t *testing.T, srv *server.MCPServer) []map[string]any {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = mcpServeRun(srv, ctx, inR, outW) }()
	go func() {
		_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`+"\n")
		_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
		_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`+"\n")
	}()
	ch := make(chan []map[string]any, 1)
	go func() {
		dec := json.NewDecoder(outR)
		for {
			var msg struct {
				ID     any `json:"id"`
				Result struct {
					Tools []map[string]any `json:"tools"`
				} `json:"result"`
			}
			if err := dec.Decode(&msg); err != nil {
				ch <- nil
				return
			}
			if id, ok := msg.ID.(float64); ok && int(id) == 2 {
				ch <- msg.Result.Tools
				return
			}
		}
	}()
	select {
	case tools := <-ch:
		if tools == nil {
			t.Fatalf("tools/list 解码失败")
		}
		return tools
	case <-time.After(10 * time.Second):
		t.Fatalf("tools/list 超时")
		return nil
	}
}

// TestMCPToolSchemas —— 参数面与 Python 逐条同：无参工具零 properties；四个带参工具 properties/required。
func TestMCPToolSchemas(t *testing.T) {
	srv := newMCPZergServerWithClient(newClient(), mcpTestRoster())
	tools := mcpTestListToolsRaw(t, srv)

	propsOf := func(tool map[string]any) (map[string]any, []any) {
		is := tool["inputSchema"].(map[string]any)
		var props map[string]any
		if p, ok := is["properties"].(map[string]any); ok {
			props = p
		} else {
			props = map[string]any{}
		}
		var req []any
		if r, ok := is["required"].([]any); ok {
			req = r
		}
		return props, req
	}

	wantProps := map[string][]string{
		"zerg_help":      {},
		"fleet_status":   {},
		"list_models":    {},
		"fleet_topology": {},
		"get_model_info": {"model"},
		"load_model":     {"machine", "model"},
		"unload_model":   {"machine", "model"},
		"switch_model":   {"machine", "model"},
	}
	if len(tools) != 8 {
		t.Fatalf("应 8 工具：%d", len(tools))
	}
	for _, tool := range tools {
		name := tool["name"].(string)
		props, req := propsOf(tool)
		want, ok := wantProps[name]
		if !ok {
			t.Fatalf("多出的工具：%s", name)
		}
		if len(props) != len(want) {
			t.Fatalf("%s properties 应 %v，实得 %v", name, want, props)
		}
		for _, w := range want {
			if _, ok := props[w]; !ok {
				t.Fatalf("%s 缺参数 %s", name, w)
			}
		}
		if len(want) > 0 && len(req) != len(want) {
			t.Fatalf("%s required 应 %v，实得 %v", name, want, req)
		}
	}
}

// TestMCPReadToolPayloads —— 只读 4 工具（help/status/models/info）的返回形状。
func TestMCPReadToolPayloads(t *testing.T) {
	var got []string
	ts := mcpMockController(t, &got)
	zs := &mcpZergServer{c: mcpTestClient(ts.URL), roster: mcpTestRoster()}

	// fleet_status：两个计数 + machines 摘要。
	d := mcpDecodeMap(t, zs.fleetStatusJSON())
	if d["success"] != true || d["message"] != "ok" {
		t.Fatalf("fleet_status 顶层不符：%v", d)
	}
	data := d["data"].(map[string]any)
	if data["healthy_count"].(float64) != 2 {
		t.Fatalf("healthy_count 应为 2：%v", data["healthy_count"])
	}
	if data["available_models_count"].(float64) != 2 {
		t.Fatalf("available_models_count 应为 2：%v", data["available_models_count"])
	}
	machines := data["machines"].([]any)
	if len(machines) != 2 {
		t.Fatalf("machines 应 2 台：%v", machines)
	}
	first := machines[0].(map[string]any)
	if first["machine"] != "node-a" || first["mem_available_gb"].(float64) != 12.3 {
		t.Fatalf("首台摘要不符：%v", first)
	}

	// list_models：count + models + machine_models。
	d = mcpDecodeMap(t, zs.listModelsJSON())
	data = d["data"].(map[string]any)
	if data["count"].(float64) != 2 || len(data["models"].([]any)) != 2 {
		t.Fatalf("list_models 计数不符：%v", data)
	}
	mm := data["machine_models"].(map[string]any)
	if len(mm["node-a"].([]any)) != 1 {
		t.Fatalf("machine_models.edge-node 应 1 条：%v", mm)
	}

	// get_model_info：命中 ⇒ loaded_on/state；未命中 ⇒ success:false。
	d = mcpDecodeMap(t, zs.getModelInfoJSON("m1"))
	info := d["data"].(map[string]any)
	if d["success"] != true || info["loaded_on"] != "node-a" || info["state"] != "ok" {
		t.Fatalf("get_model_info(m1) 不符：%v", d)
	}
	d = mcpDecodeMap(t, zs.getModelInfoJSON("nope"))
	if d["success"] != false {
		t.Fatalf("get_model_info(未命中) 应 success:false：%v", d)
	}

	// zerg_help：本地拼串，data 有 7 键（zerg_help 自身不在其中）。
	d = mcpDecodeMap(t, zs.helpJSON())
	hd := d["data"].(map[string]any)
	if len(hd) != 7 {
		t.Fatalf("zerg_help data 应 7 键：%v", hd)
	}
}

// TestMCPFleetTopologyRoster —— host 从名册取（✗ 不硬编码）。
func TestMCPFleetTopologyRoster(t *testing.T) {
	var got []string
	ts := mcpMockController(t, &got)
	zs := &mcpZergServer{c: mcpTestClient(ts.URL), roster: mcpTestRoster()}
	d := mcpDecodeMap(t, zs.fleetTopologyJSON())
	conn := d["data"].(map[string]any)["connectivity"].(map[string]any)
	want := map[string]string{"node-a": "<worker-ip>", "node-b": "<worker-ip>", "node-c": "<worker-ip>", "Mr2109": "127.0.0.1"}
	for k, v := range want {
		if conn[k] != v {
			t.Fatalf("connectivity[%s] 应 %s：实得 %v", k, v, conn[k])
		}
	}
	if len(conn) != 4 {
		t.Fatalf("connectivity 应 4 机（与硬编码的 3 键分家）：%v", conn)
	}
}

// TestMCPWriteToolBodies —— 写面体裁：load/switch 含 {machine,model}；unload 只 {machine}。
func TestMCPWriteToolBodies(t *testing.T) {
	var got []string
	ts := mcpMockController(t, &got)
	zs := &mcpZergServer{c: mcpTestClient(ts.URL), roster: mcpTestRoster()}

	if d := mcpDecodeMap(t, zs.loadModelJSON("node-a", "m1")); d["success"] != true {
		t.Fatalf("load 应成功：%v", d)
	}
	d := mcpDecodeMap(t, zs.unloadModelJSON("node-a", "m1"))
	if d["success"] != true {
		t.Fatalf("unload 应成功：%v", d)
	}
	if d := mcpDecodeMap(t, zs.switchModelJSON("node-b", "m2")); d["success"] != true {
		t.Fatalf("switch 应成功：%v", d)
	}
	if len(got) != 3 {
		t.Fatalf("应记录 3 次写面：%v", got)
	}
	if got[0] != `load {"machine": "node-a", "model": "m1"}` {
		t.Fatalf("load 体裁不符（Python 空格分隔）：%q", got[0])
	}
	if got[1] != `unload {"machine": "node-a"}` {
		t.Fatalf("unload 体裁应为 {machine}（✗ 不带 model）：%q", got[1])
	}
}

// TestMCPNegativeControls —— 负控：非法 machine、主控打不到。
func TestMCPNegativeControls(t *testing.T) {
	var got []string
	ts := mcpMockController(t, &got)
	zs := &mcpZergServer{c: mcpTestClient(ts.URL), roster: mcpTestRoster()}

	// 非法 machine（local 已不在名册）⇒ success:false 且点名。
	d := mcpDecodeMap(t, zs.loadModelJSON("local", "m1"))
	if d["success"] != false {
		t.Fatalf("非法 machine 应 success:false：%v", d)
	}
	if !strings.Contains(d["message"].(string), "local") {
		t.Fatalf("非法 machine 应点名收到值：%v", d["message"])
	}

	// 主控打不到 ⇒ 工具回 success:false，进程不退（此处即函数正常返回）。
	dead := &mcpZergServer{c: mcpTestClient("http://127.0.0.1:1"), roster: mcpTestRoster()}
	d = mcpDecodeMap(t, dead.fleetStatusJSON())
	if d["success"] != false || !strings.HasPrefix(d["message"].(string), "连接主控失败") {
		t.Fatalf("打不到主控应回 连接主控失败：%v", d)
	}
	// 运行期错误不写进程退码 —— 同一 server 的只读工具仍可继续单独调用。
	if dd := mcpDecodeMap(t, dead.helpJSON()); dd["success"] != true {
		t.Fatalf("help 不依赖主控，应仍成功：%v", dd)
	}
}

// TestMCPCommandFace —— `--json` 拒收 rc=2、未知旗标 rc=2、`--help` rc=0。
func TestMCPCommandFace(t *testing.T) {
	var out, errBuf bytes.Buffer
	if rc := mcpServeMain(&out, &errBuf, strings.NewReader(""), []string{"--json"}); rc != exitUsage {
		t.Fatalf("--json 应 rc=2，实得 %d", rc)
	}
	out.Reset()
	errBuf.Reset()
	if rc := mcpServeMain(&out, &errBuf, strings.NewReader(""), []string{"--nosuchflag"}); rc != exitUsage {
		t.Fatalf("未知旗标应 rc=2，实得 %d", rc)
	}
	out.Reset()
	errBuf.Reset()
	if rc := mcpServeMain(&out, &errBuf, strings.NewReader(""), []string{"--help"}); rc != exitOK {
		t.Fatalf("--help 应 rc=0，实得 %d", rc)
	}
	if !strings.Contains(out.String(), "zerg mcp serve") {
		t.Fatalf("--help 正文不符：%q", out.String())
	}
}

// TestMCPSelfTest —— `--self-test` 面 rc=0。
func TestMCPSelfTest(t *testing.T) {
	var out, errBuf bytes.Buffer
	if rc := mcpSelfTest(&out, &errBuf); rc != exitOK {
		t.Fatalf("--self-test 应 rc=0，实得 %d\n%s\n%s", rc, out.String(), errBuf.String())
	}
}

// TestMCPParityDump —— 对拍用：给了 MCP_PARITY_URL 就把只读工具输出逐行打印（外部脚本与 Python 对拍）。
func TestMCPParityDump(t *testing.T) {
	url := os.Getenv("MCP_PARITY_URL")
	if url == "" {
		t.Skip("MCP_PARITY_URL 未设 ⇒ 跳过对拍转储")
	}
	zs := &mcpZergServer{c: mcpTestClient(url), roster: mcpTestRoster()}
	emit := func(name, s string) { t.Logf("TOOL %s %s", name, s) }
	emit(mcpToolHelp, zs.helpJSON())
	emit(mcpToolFleetStatus, zs.fleetStatusJSON())
	emit(mcpToolListModels, zs.listModelsJSON())
	emit(mcpToolGetModelInfo, zs.getModelInfoJSON("m1"))
	emit(mcpToolFleetTopo, zs.fleetTopologyJSON())
}
