package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

// fakeCLIEngine 是一个最小的 OpenAI 兼容假端点（与 modelreg 包测试同款判据），
// 不碰外网、不碰真实引擎。
func fakeCLIEngine(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"object":"list","data":[{"id":"fake-llama","object":"model"}]}`)
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			bs := string(body)
			switch {
			case strings.Contains(bs, `"tools"`):
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_time","arguments":"{\"tz\":\"UTC\"}"}}]}}]}`)
			case strings.Contains(bs, "image_url"):
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":{"message":"image input is not supported by this model","type":"invalid_request_error"}}`)
			default:
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"你好"}}]}`)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"message":"no such route"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// 待修补 #24 的端到端验收（CLI 层）：同一模型连跑两次 probe --store，
// 两次都 exit 0；第二次 changed=false（记录不重写）；留痕兄弟文件存在。
func TestCmdProbeStoreTwiceExitZero(t *testing.T) {
	srv := fakeCLIEngine(t)
	root := t.TempDir()
	args := []string{srv.URL, "--store", "--store-root", root}

	if code := cmdProbe(args); code != 0 {
		t.Fatalf("第一次 probe --store 应 exit 0，实际 %d", code)
	}

	recPath := findOne(t, filepath.Join(root, "manifests"), "*.json", ".trace.json", ".capabilities.json")
	before, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatal(err)
	}
	fi1, err := os.Stat(recPath)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(1100 * time.Millisecond) // 跨秒边界：正文若含时间戳，第二次必 changed=true/冲突(exit 3)
	if code := cmdProbe(args); code != 0 {
		t.Fatalf("第二次 probe --store 应 exit 0（幂等 changed=false），实际 %d —— 若为 3 说明正文仍含易变信息", code)
	}

	after, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("第二次不该改写记录：\n  before=%s\n  after =%s", before, after)
	}
	fi2, err := os.Stat(recPath)
	if err != nil {
		t.Fatal(err)
	}
	if !fi1.ModTime().Equal(fi2.ModTime()) {
		t.Fatalf("记录文件被重写（mtime 变了）：%s → %s", fi1.ModTime(), fi2.ModTime())
	}

	// 留痕兄弟文件存在且可解析
	tracePath := modelreg.TraceSiblingPath(recPath)
	tb, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("留痕兄弟文件没写：%v", err)
	}
	var art modelreg.ProbeTraceArtifact
	if err := json.Unmarshal(tb, &art); err != nil {
		t.Fatalf("留痕兄弟文件不是合法 JSON：%v", err)
	}
	if art.Schema != modelreg.TraceSchemaV1 || len(art.Traces) == 0 {
		t.Fatalf("留痕兄弟文件内容不对：%+v", art)
	}

	// 能力快照兄弟文件同样落盘、可解析（能力断言 + 证据）
	snapPath := modelreg.CapabilitySnapshotPath(recPath)
	sb, err := os.ReadFile(snapPath)
	if err != nil {
		t.Fatalf("能力快照兄弟文件没写：%v", err)
	}
	var snap modelreg.CapabilitySnapshotArtifact
	if err := json.Unmarshal(sb, &snap); err != nil {
		t.Fatalf("能力快照不是合法 JSON：%v", err)
	}
	if snap.Schema != modelreg.CapabilitySnapshotSchemaV1 || len(snap.Capabilities) == 0 || snap.Endpoint != srv.URL {
		t.Fatalf("能力快照内容不对：%+v", snap)
	}

	// 目录健康：list 只看到 1 条记录、0 error（留痕兄弟被跳过）
	rows, err := modelreg.NewStore(root).List()
	if err != nil {
		t.Fatalf("list 报错：%v", err)
	}
	if len(rows) != 1 || rows[0].Errors != 0 || rows[0].Err != "" {
		t.Fatalf("目录应只有 1 条健康记录（留痕兄弟不进目录语义）：%+v", rows)
	}
}

// findOne 在 dir 下找第一个匹配 glob、且不以任何一个 excludeSuffix 结尾的文件。
func findOne(t *testing.T, dir, glob string, excludeSuffixes ...string) string {
	t.Helper()
	var hits []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ok, _ := filepath.Match(glob, info.Name())
		if !ok {
			return nil
		}
		for _, ex := range excludeSuffixes {
			if strings.HasSuffix(info.Name(), ex) {
				return nil
			}
		}
		hits = append(hits, p)
		return nil
	})
	if len(hits) == 0 {
		t.Fatalf("在 %s 下没找到匹配 %s 的文件", dir, glob)
	}
	return hits[0]
}

// 本批端到端（CLI 层）：先 probe <本地文件> --store（无端点，登记身份），
// 再 probe <同一文件> --endpoint <假端点> --store（补能力实测）→ **必须 exit 0**
// （不再 ConflictError/exit 3），记录正文逐字节不变（身份未被改写），
// 能力快照兄弟文件落盘且能看到带 evidence 的能力断言。
func TestCmdProbeRegisterThenAddCapabilities(t *testing.T) {
	srv := fakeCLIEngine(t)
	root := t.TempDir()
	brickwork := filepath.Join(t.TempDir(), "brickwork-Q4_K_M.gguf")
	if err := os.WriteFile(brickwork, []byte("ZERG-FAKE-BRICKWORK-BODY"), 0o600); err != nil {
		t.Fatal(err)
	}

	// 第一步：无端点 → 只登记身份
	if code := cmdProbe([]string{brickwork, "--store", "--store-root", root}); code != 0 {
		t.Fatalf("第一步（无端点登记）应 exit 0，实际 %d", code)
	}
	recPath := findOne(t, filepath.Join(root, "manifests"), "*.json", ".trace.json", ".capabilities.json")
	before, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(modelreg.CapabilitySnapshotPath(recPath)); err != nil {
		t.Fatalf("第一步就该写能力快照（哪怕还没有在线能力，也要如实记 online_probed=false）：%v", err)
	}
	// 无端点那次：快照如实写明"还没在线探过"（online_probed=false、能力为空）
	snap0, err := os.ReadFile(modelreg.CapabilitySnapshotPath(recPath))
	if err != nil {
		t.Fatalf("读第一步的能力快照失败：%v", err)
	}
	var art0 modelreg.CapabilitySnapshotArtifact
	if err := json.Unmarshal(snap0, &art0); err != nil {
		t.Fatalf("第一步的能力快照不是合法 JSON：%v", err)
	}
	if art0.OnlineProbed || len(art0.Capabilities) != 0 || art0.Endpoint != "" {
		t.Fatalf("第一步（无端点）不该有在线能力/端点：%+v", art0)
	}

	// 第二步：带端点 → 补能力实测；正文不变，故不得冲突
	if code := cmdProbe([]string{brickwork, "--endpoint", srv.URL, "--store", "--store-root", root}); code != 0 {
		t.Fatalf("第二步（补能力实测）必须 exit 0（不再是 ConflictError/3），实际 %d", code)
	}
	after, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("补能力实测不得改写记录正文（身份未被改写）：\n  before=%s\n  after =%s", before, after)
	}

	// 能力快照兄弟文件：落盘、可解析、带端点 + 证据
	snapPath := modelreg.CapabilitySnapshotPath(recPath)
	sb, err := os.ReadFile(snapPath)
	if err != nil {
		t.Fatalf("能力快照兄弟文件没写：%v", err)
	}
	var art modelreg.CapabilitySnapshotArtifact
	if err := json.Unmarshal(sb, &art); err != nil {
		t.Fatalf("能力快照不是合法 JSON：%v", err)
	}
	if art.Schema != modelreg.CapabilitySnapshotSchemaV1 || art.Endpoint != srv.URL || art.GeneratedAt == "" || !art.OnlineProbed {
		t.Fatalf("能力快照应记下 schema/端点/生成时间/online_probed：%+v", art)
	}
	var vision *modelreg.Capability
	for i := range art.Capabilities {
		if art.Capabilities[i].Name == "vision" {
			vision = &art.Capabilities[i]
		}
	}
	if vision == nil || vision.Source != "probed" || vision.Evidence == "" {
		t.Fatalf("能力快照里应看到带证据的 vision 断言：%+v", art.Capabilities)
	}
	t.Logf("capability snapshot: %s", snapPath)

	// 目录健康：list 只看到 1 条记录、0 error（留痕与能力快照两个兄弟都被跳过）
	rows, err := modelreg.NewStore(root).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Errors != 0 || rows[0].Err != "" {
		t.Fatalf("目录应只有 1 条健康记录（两个兄弟文件都不进目录语义）：%+v", rows)
	}
}
