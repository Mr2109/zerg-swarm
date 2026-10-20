// family_api_serve.go —— `zerg api serve`（外部接入 REST API · **归入命令面** · 批 B4）。
//
// 源件 = 独立入口 `core/cmd/zerg-api/main.go`（227 行 · Go `flag` 三枚：`-port 8083` /
// `-workdir .` / `-insecure`）。本件是那一件的**逐面搬**：端口与工作目录 → 令牌闸 →
// 六个 HTTP 端点（`POST /task` · `GET /task/{id}` · `GET /capabilities` · `GET /status` ·
// `GET /panel` · `GET /`）→ 监听 `:<port>`。端点判词、JSON 键、面板 HTML、能力清单**逐字**照源件。
//
// 形态（旗标**逐字**保留源件 · 只把 Go `flag` 的单横线写成命令面的双横线）：
//
//	zerg api serve [--port <端口>] [--workdir <目录>] [--insecure]
//
//	--port <端口>     监听端口（源件缺省 8083；`0` = 由系统分配 —— 与源件 `:0` 同义）
//	--workdir <目录>  工作目录（含 `docs/issues/` 的那个目录 · 源件缺省 `.`）
//	--insecure        允许无令牌运行（**会打横幅** · 只在可信本机网络用）
//
// 鉴权：环境变量 `ZERG_API_TOKEN` —— 有则要求 `Authorization: Bearer <令牌>`；无则**拒启**
// （除非显式 `--insecure` 并打横幅）。口径逐字照源件（§十二 `P-007`：默认放行必须治）。
//
// 为什么并入 `api` 族、✗ 不新开族：`api` 族**已经在册**（`ls` / `openapi` / `help` 三条只读
// 投影 —— 打主控 `/api/*` 的**客户端**面）。本件搬的是**另一件事**：在本机**起**一个服务端
// （源件 `zerg-api` 的语义）。两者共用族名、动作名分家：`serve` = 起进程（本件）· 其余三条 =
// 读主控的投影。★ 与 `sched run` 同一条判法（族已在册 ⇒ 只增一格）。
//
// 退码（真源 = `exitcodes.go` 的表；本文件不写死数字）：
//
//	0  服务起 → 停干净（收到 SIGINT/SIGTERM 或 ctx 结束）
//	2  用法错（`--port` 裸给/非整数/超范围 · `--workdir` 裸给 · 多余位置参数 · 未知旗标）
//	1  缺令牌且未给 `--insecure`（源件 `log.Fatalf` ⇒ 同档）· 监听失败（源件 `log.Fatal` ⇒ 同档）
//
// 与源件的**照实差异**（五处 · 逐条写清，别处不猜）：
//
//	① 用法类退码 **1 → 2**：源件的旗标值不合法一律由 `log.Fatalf`/`flag` 走到 1 或 2 之间摇摆；
//	   命令面统一走 `exitcodes.go` 的 usage 档（同批 B1 `trace`、B2 `sched run`、B3 `review`
//	   的同一条归一）。
//	② `--port` 值域**前置判**：不给/裸给/非整数/不在 `0..65535` ⇒ 用法错 2（源件把它直接交给
//	   `ListenAndServe`，非法端口要到 `log.Fatal` 才炸、退 1）。`0` 保留（= `:0`，系统分配）。
//	③ 工作目录在**启动时**解析成绝对路径：源件逐请求拿 `*workDir` 拼相对路径 ⇒ 进程 cwd 一变
//	   路径就漂；常驻档不该有这种漂（`--workdir` 裸给/空值 ⇒ 用法错 2，与同族值旗标纪律
//	   `GAP-20260928-151` 一致；不给 = 当前目录，与源件缺省同义）。
//	④ **不切进程目录**：源件在 `POST /task` 里 `os.Chdir(workDir)` 再 `SubmitTask` —— 而
//	   `agent.SubmitTask` 的落点早已改成 `statepath.IssuesDir()`（**不看 cwd**，见
//	   `core/internal/agent/entry.go:118`）⇒ 那三行在源件里已是空转；常驻档里切进程 cwd 还是
//	   并发请求间的竞态 ⇒ 本件不切（行为零差，风险净减）。
//	⑤ **优雅停**：ctx 结束 ⇒ `http.Server.Shutdown` + `📡` / `🛑` 两行判词 + 退 0（源件被信号直接
//	   打断、无判词）。★ **实跑读数照实**：真二进制里 `main()` 的**全局信号处理器**（`SIGINT`/
//	   `SIGTERM`/`SIGHUP` ⇒ 判词 + `130`）与本节这一枚**都装了**，谁先赢不保证 —— 实测
//	   `kill -TERM` 打到常驻档：先出 `📡 stop signal received…`（本档收到 ctx），随后全局那枚赢、
//	   打「收到 terminated —— 只退订、不取消」并**退 130**（`🛑` 不再打）。ctx 由**内部**取消
//	   （判据件的 `apiServeRun` 直调档）才走完整的 `🛑` + 退 0 —— 两种都不留半开端口。
//	   启动行 `🚀 zerg-api started …` 逐字保留（含鉴权注记）。
//
// ★ 双轨期：`core/cmd/zerg-api` 二进制**一字不动**（本件只搬逻辑、不删入口）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// apiServeDefaultPort —— 源件 `-port` 的缺省值。
const apiServeDefaultPort = 8083

// apiServeDefaultWorkDir —— 源件 `-workdir` 的缺省值（`.` = 当前目录）。
const apiServeDefaultWorkDir = "."

// apiServeShutdownGrace —— 优雅停的宽限（`Shutdown` 等在用请求收尾；常驻档给 5s 够用）。
const apiServeShutdownGrace = 5 * time.Second

// apiServeUsageLine —— 用法串（与命令树 `usage` 字段**逐字同源**：一处写错两处就分家）。
const apiServeUsageLine = "zerg api serve [--port <端口>] [--workdir <目录>] [--insecure]"

// apiServeCfg —— 解析后的启动配置（`workDir` 已是绝对路径）。
type apiServeCfg struct {
	port     int
	workDir  string
	insecure bool
	token    string // 环境变量 `ZERG_API_TOKEN` 的现读值（空 = 没配）
}

// apiServeParse —— 旗标 → 配置（语义校验只在本处）。
//
// 退码：`0` 配置齐；`2` 用法错（`--port` 裸给/非整数/超范围 · `--workdir` 裸给/解析不出）。
func apiServeParse(inv *invocation, stderr io.Writer) (apiServeCfg, int) {
	cfg := apiServeCfg{port: apiServeDefaultPort, workDir: apiServeDefaultWorkDir}

	// `--port` 是**取值**旗标（源件 `-port`）：裸给/非整数/超范围 ⇒ 用法错 2（差异 ②）。
	if inv.hasFlag("--port") {
		txt := strings.TrimSpace(inv.flagVal("--port"))
		if txt == "" {
			inv.setErr("usage", "api_port_empty", "`--port` 裸给或空值")
			fmt.Fprintf(stderr, "%s: `--port` 要一个**端口号**（裸给/空值 ⇒ 不执行 —— 不给这一枚 = %d）\n",
				progName, apiServeDefaultPort)
			fmt.Fprintf(stderr, "用法: %s\n", apiServeUsageLine)
			return cfg, exitUsage
		}
		n, err := strconv.Atoi(txt)
		if err != nil || n < 0 || n > 65535 {
			inv.setErr("usage", "api_port_bad", "`--port` 不是 0..65535 的整数")
			fmt.Fprintf(stderr, "%s: `--port` 要 0..65535 的整数（给的是 %q）\n", progName, txt)
			fmt.Fprintf(stderr, "用法: %s\n", apiServeUsageLine)
			return cfg, exitUsage
		}
		cfg.port = n
	}

	// `--workdir` 是**取值**旗标（源件 `-workdir`）：裸给/空值 ⇒ 用法错 2（差异 ③）。
	wd := apiServeDefaultWorkDir
	if inv.hasFlag("--workdir") {
		wd = strings.TrimSpace(inv.flagVal("--workdir"))
		if wd == "" {
			inv.setErr("usage", "api_workdir_empty", "`--workdir` 裸给或空值")
			fmt.Fprintf(stderr, "%s: `--workdir` 要一个**目录**（裸给/空值 ⇒ 不执行 —— 不给这一枚 = 当前目录）\n", progName)
			fmt.Fprintf(stderr, "用法: %s\n", apiServeUsageLine)
			return cfg, exitUsage
		}
	}
	abs, err := schedResolveWorkDir(wd) // 同一条解析口径（`""`/`"."` ⇒ 当前目录，其余 ⇒ 绝对路径）
	if err != nil {
		inv.setErr("failed", "api_workdir_unresolvable", "工作目录解析不出来")
		fmt.Fprintf(stderr, "%s: ❌ failed to resolve workdir: %v\n", progName, err)
		return cfg, exitFail
	}
	cfg.workDir = abs

	cfg.insecure = inv.hasFlag("--insecure")
	cfg.token = os.Getenv("ZERG_API_TOKEN")
	return cfg, exitOK
}

// apiServeAuthOK —— 源件 `authOK` 的逐字搬（`Bearer <令牌>` 精确匹配；401 的正文逐字同）。
func apiServeAuthOK(w http.ResponseWriter, r *http.Request, token string) bool {
	if token == "" {
		return true // 未配置令牌 —— 不鉴权（此档只在 `--insecure` 下可能进得来）
	}
	if r.Header.Get("Authorization") != "Bearer "+token {
		http.Error(w, `{"error":"unauthorized"}`, 401)
		return false
	}
	return true
}

// apiServeAuthNote —— 源件 `authNote` 的逐字搬（启动行末尾那句注记）。
func apiServeAuthNote(token string) string {
	if token == "" {
		return "（无鉴权）"
	}
	return "（Bearer 鉴权）"
}

// apiServeWriteJSON —— 源件 `writeJSON` 的逐字搬（`application/json` + `Encode`）。
func apiServeWriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// apiServeField —— 源件 `extractField` 的逐字搬（提取 markdown 字段值 `- **字段**: 值`）。
func apiServeField(content, prefix string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// apiServeMux —— 源件的六条路由（方法 + 路径 + 判词逐字照搬）。
//
// ★ 分成「造 mux」与「起监听」两层：判据面用 `httptest` 直接打这一份 mux（不必真占端口），
// 实跑面走 `apiServeRun`。两层共用**同一条**实现 —— ✗ 不各写一份路由。
func apiServeMux(cfg apiServeCfg) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /task", func(w http.ResponseWriter, r *http.Request) {
		if !apiServeAuthOK(w, r, cfg.token) {
			return
		}
		var spec agent.TaskSpec
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &spec); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, 400)
			return
		}
		// 差异 ④：源件此处 `os.Chdir(cfg.workDir)`（在 `statepath.IssuesDir()` 改造后已是空转）。
		path, err := agent.SubmitTask(spec)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), 400)
			return
		}
		apiServeWriteJSON(w, map[string]string{"issue_path": path, "status": "submitted"})
	})

	mux.HandleFunc("GET /task/", func(w http.ResponseWriter, r *http.Request) {
		if !apiServeAuthOK(w, r, cfg.token) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/task/")
		path := filepath.Join(cfg.workDir, "docs", "issues", id+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, `{"error":"task not found"}`, 404)
			return
		}
		apiServeWriteJSON(w, map[string]string{"id": id, "content": string(data)})
	})

	mux.HandleFunc("GET /capabilities", func(w http.ResponseWriter, r *http.Request) {
		if !apiServeAuthOK(w, r, cfg.token) {
			return
		}
		apiServeWriteJSON(w, map[string]any{
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
		if !apiServeAuthOK(w, r, cfg.token) {
			return
		}
		issuesDir := filepath.Join(cfg.workDir, "docs", "issues")
		entries, err := os.ReadDir(issuesDir)
		if err != nil {
			apiServeWriteJSON(w, map[string]any{"tasks": 0, "status": map[string]int{}, "error": err.Error()})
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
			status := apiServeField(content, "- **状态**:")
			ftype := apiServeField(content, "- **失败类型**:")
			statusCount[status]++
			if ftype != "" && status != "done" {
				failTypeCount[ftype]++
			}
			tasks = append(tasks, map[string]string{
				"id": strings.TrimSuffix(e.Name(), ".md"), "status": status, "type": ftype,
			})
		}
		apiServeWriteJSON(w, map[string]any{
			"tasks":         len(tasks),
			"status":        statusCount,
			"failure_types": failTypeCount,
			"task_list":     tasks,
		})
	})

	// P3 临时 Web 面板（调 /status —— 浏览器看）
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
  curl -H "Authorization: Bearer ***" ...
`)
	})

	return mux
}

// apiServeRun —— 常驻档本体：令牌闸 → 起监听 → 等 ctx 结束 → 停干净（`0`）。
//
// ctx 的**唯一来源**在 `cmdAPIServe`（信号面）；本函数不认识信号 —— 判据面用已取消的 ctx
// 走完「起 → 停」一整趟（同一条实现，两个入口）。
//
// 退码：`0` 起→停干净；`1` 缺令牌且未 `--insecure`（源件 `log.Fatalf` 同档）/ 监听失败。
func apiServeRun(ctx context.Context, cfg apiServeCfg, stderr io.Writer) int {
	// 令牌闸（源件 §十二 `P-007` 那一段，判词逐字）：
	//   ① 缺令牌 ⇒ **拒启**（除非下面那枚显式旗标给了）；
	//   ② 显式 `--insecure` ⇒ 允许无令牌，但**必须**打横幅告警（不许静默裸奔）。
	if cfg.token == "" {
		if !cfg.insecure {
			fmt.Fprintf(stderr, "✗ ZERG_API_TOKEN 未设置 ⇒ 拒绝启动（§十二 P-007：无令牌即放行必须治）。\n"+
				"   要用无鉴权模式：显式给 --insecure（会打横幅告警）；要鉴权：export ZERG_API_TOKEN=<令牌>\n")
			return exitFail
		}
		fmt.Fprintf(stderr, "⚠️  横幅：zerg-api 以 --insecure 启动 —— **无鉴权**，任何能连到 %d 的进程都能提交任务\n", cfg.port)
	}

	addr := fmt.Sprintf(":%d", cfg.port)
	srv := &http.Server{Addr: addr, Handler: apiServeMux(cfg)}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	fmt.Fprintf(stderr, "🚀 zerg-api started %s (workdir=%s%s)\n", addr, cfg.workDir, apiServeAuthNote(cfg.token))

	select {
	case <-ctx.Done():
		fmt.Fprintln(stderr, "📡 stop signal received, shutting down gracefully...")
		shutCtx, cancel := context.WithTimeout(context.Background(), apiServeShutdownGrace)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			fmt.Fprintf(stderr, "%s: ⚠️ graceful shutdown failed: %v\n", progName, err)
			_ = srv.Close()
			return exitFail
		}
		fmt.Fprintln(stderr, "🛑 api server stopped")
		return exitOK
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return exitOK
		}
		fmt.Fprintf(stderr, "%s: ❌ listen on %s failed: %v\n", progName, addr, err)
		return exitFail
	}
}

// cmdAPIServe —— `zerg api serve`：在本机起一个外部接入 REST API（常驻到收到 SIGINT/SIGTERM）。
//
// ★ 信号面照实（与 `sched run` 同一条）：真二进制里 `main()` 的全局处理器
// （`SIGINT`/`SIGTERM`/`SIGHUP` ⇒ 判词 + `130`）先装；本处另装一枚（两枚都收得到）⇒ 实跑时
// Ctrl-C 可能退 130、也可能走本档的优雅停（0）—— 两者都不留半开端口。`SIGKILL` 一律到不了这里。
func cmdAPIServe(inv *invocation, stdout, stderr io.Writer) int {
	cfg, rc := apiServeParse(inv, stderr)
	if rc != exitOK {
		return rc
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return apiServeRun(ctx, cfg, stderr)
}
