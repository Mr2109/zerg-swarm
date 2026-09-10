// cmd/zerg-api/main.go — v2.5.2 外部接入 REST API
// 用法: zerg-api [-port 8083] [-workdir <有docs/的目录>]
// 端点:
//   POST /task         提交任务（JSON TaskSpec）→ 返回 issue 路径
//   GET  /task/{id}    查任务状态
//   GET  /capabilities 能力清单（机器可读——Agent 发现）
//   GET  /             使用说明（人类可读——curl 示例）
// 鉴权: 环境变量 ZERG_API_TOKEN——有则要求 Bearer——无则不鉴权

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

func main() {
	port := flag.Int("port", 8083, "监听端口")
	workDir := flag.String("workdir", ".", "工作目录（有 docs/issues/ 的目录）")
	flag.Parse()

	token := os.Getenv("ZERG_API_TOKEN")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /task", func(w http.ResponseWriter, r *http.Request) {
		if !authOK(w, r, token) {
			return
		}
		var spec agent.TaskSpec
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &spec); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, 400)
			return
		}
		// 临时切工作目录（SubmitTask 用相对 docs/issues/）
		orig, _ := os.Getwd()
		os.Chdir(*workDir)
		path, err := agent.SubmitTask(spec)
		os.Chdir(orig)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), 400)
			return
		}
		writeJSON(w, map[string]string{"issue_path": path, "status": "submitted"})
	})

	mux.HandleFunc("GET /task/", func(w http.ResponseWriter, r *http.Request) {
		if !authOK(w, r, token) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/task/")
		path := filepath.Join(*workDir, "docs", "issues", id+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, `{"error":"task not found"}`, 404)
			return
		}
		writeJSON(w, map[string]string{"id": id, "content": string(data)})
	})

	mux.HandleFunc("GET /capabilities", func(w http.ResponseWriter, r *http.Request) {
		if !authOK(w, r, token) {
			return
		}
		writeJSON(w, map[string]any{
			"name":        "zerg-api",
			"description": "虫族任务执行体外部接口——提交任务进 git 自循环",
			"endpoints": []map[string]any{
				{"method": "POST", "path": "/task", "desc": "提交任务", "body": `{"task":"...","type":"code|report|research|fix","priority":"high|normal|low","source":"git|cron|api|a2a"}`},
				{"method": "GET", "path": "/task/{id}", "desc": "查任务状态"},
				{"method": "GET", "path": "/status", "desc": "任务状态统计（监控面板）"},
				{"method": "GET", "path": "/capabilities", "desc": "本清单"},
			},
			"task_types": []string{"code", "report", "research", "fix"},
			"example": map[string]string{
				"curl": `curl -X POST http://localhost:8083/task -H "Content-Type: application/json" -d '{"task":"优化某函数","type":"code","priority":"normal","source":"api"}'`,
			},
		})
	})

	// P3 监控：任务状态统计（状态分布 + 失败类型 + 任务列表）
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		if !authOK(w, r, token) {
			return
		}
		issuesDir := filepath.Join(*workDir, "docs", "issues")
		entries, err := os.ReadDir(issuesDir)
		if err != nil {
			writeJSON(w, map[string]any{"tasks": 0, "status": map[string]int{}, "error": err.Error()})
			return
		}
		statusCount := map[string]int{}
		failTypeCount := map[string]int{}
		var tasks []map[string]string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(issuesDir, e.Name()))
			if err != nil {
				continue
			}
			content := string(data)
			status := extractField(content, "- **状态**:")
			ftype := extractField(content, "- **失败类型**:")
			statusCount[status]++
			if ftype != "" && status != "done" {
				failTypeCount[ftype]++
			}
			tasks = append(tasks, map[string]string{
				"id": strings.TrimSuffix(e.Name(), ".md"), "status": status, "type": ftype,
			})
		}
		writeJSON(w, map[string]any{
			"tasks":         len(tasks),
			"status":        statusCount,
			"failure_types": failTypeCount,
			"task_list":     tasks,
		})
	})

	// P3 临时 Web 面板（调 /status——浏览器看）
	mux.HandleFunc("GET /panel", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>虫族任务监控</title>
<style>body{font-family:monospace;background:#111;color:#0f0;padding:20px}
table{border-collapse:collapse}td,th{border:1px solid #333;padding:4px 10px}
.badge{display:inline-block;padding:2px 8px;border-radius:10px;margin:2px}
.open{background:#553} .queued{background:#353} .running{background:#335}
.fixing{background:#553} .verified{background:#353} .done{background:#353}
.escalated{background:#633} .dead{background:#633} .retry{background:#663}</style></head><body>
<h1>🐜 虫族任务执行体监控</h1>
<div id="d">加载中...</div>
<script>
async function load(){try{
  const r=await fetch('/status');const j=await r.json();
  let s='<h2>任务总数: '+j.tasks+'</h2><h3>状态分布</h3><p>';
  for(const[k,v]of Object.entries(j.status))s+='<span class="badge '+k+'">'+k+': '+v+'</span> ';
  s+='</p><h3>失败类型</h3><p>';
  for(const[k,v]of Object.entries(j.failure_types))s+='<span class="badge">'+k+': '+v+'</span> ';
  s+='</p><h3>任务列表</h3><table><tr><th>ID</th><th>状态</th><th>类型</th></tr>';
  for(const t of j.task_list)s+='<tr><td>'+t.id+'</td><td>'+t.status+'</td><td>'+t.type+'</td></tr>';
  s+='</table>';
  document.getElementById('d').innerHTML=s;
}catch(e){document.getElementById('d').textContent='错误: '+e;}}
load();setInterval(load,5000);
</script></body></html>`)
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, `虫族任务执行体 API
====================

提交任务:
  curl -X POST http://localhost:8083/task -H "Content-Type: application/json" \
    -d '{"task":"优化某函数","type":"code","priority":"normal","source":"api"}'

查状态:
  curl http://localhost:8083/task/20260814-123456-abcd

能力清单（机器可读）:
  curl http://localhost:8083/capabilities

鉴权（如设置 ZERG_API_TOKEN）:
  curl -H "Authorization: Bearer <token>" ...
`)
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("🚀 zerg-api 启动 %s（workdir=%s%s）", addr, *workDir, authNote(token))
	log.Fatal(http.ListenAndServe(addr, mux))
}

func authOK(w http.ResponseWriter, r *http.Request, token string) bool {
	if token == "" {
		return true // 未配置 token——不鉴权
	}
	if r.Header.Get("Authorization") != "Bearer "+token {
		http.Error(w, `{"error":"unauthorized"}`, 401)
		return false
	}
	return true
}

func authNote(token string) string {
	if token == "" {
		return "（无鉴权）"
	}
	return "（Bearer 鉴权）"
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// extractField — 提取 markdown 字段值（- **字段**: 值）
func extractField(content, prefix string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
