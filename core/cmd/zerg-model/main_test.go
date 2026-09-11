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

	recPath := findOne(t, filepath.Join(root, "manifests"), "*.json", ".trace.json")
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

	// 目录健康：list 只看到 1 条记录、0 error（留痕兄弟被跳过）
	rows, err := modelreg.NewStore(root).List()
	if err != nil {
		t.Fatalf("list 报错：%v", err)
	}
	if len(rows) != 1 || rows[0].Errors != 0 || rows[0].Err != "" {
		t.Fatalf("目录应只有 1 条健康记录（留痕兄弟不进目录语义）：%+v", rows)
	}
}

// findOne 在 dir 下找第一个匹配 glob、且不以 excludeSuffix 结尾的文件。
func findOne(t *testing.T, dir, glob, excludeSuffix string) string {
	t.Helper()
	var hits []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ok, _ := filepath.Match(glob, info.Name())
		if ok && !strings.HasSuffix(info.Name(), excludeSuffix) {
			hits = append(hits, p)
		}
		return nil
	})
	if len(hits) == 0 {
		t.Fatalf("在 %s 下没找到匹配 %s 的文件", dir, glob)
	}
	return hits[0]
}
