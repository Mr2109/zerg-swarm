# Fleet 协议契约（子端 Agent ↔ 主控 FleetController ↔ X3Monitor）

> 本文件是子端 Agent、主控、控制 API、界面之间接口的**单一事实来源**。实现时必须严格遵循，避免并行开发漂移。
> **v2 状态（2026-08-09）**：主控已 Go 重写（zerg-core），子端已 Go 重写（zerg-agent），协议保持 v1 兼容（token/心跳/status 字段一致），新增 backend_state。详见文末 §12。

## 0. 共享 Token
- 值: `x3gw-shared-2026`（fleet.yaml 中 `auth.token`；agent.py 顶部默认常量；主控 FleetController 读取）
- 所有子端 :8100 端点、主控 :8580 的 fleet 端点、统一入口 :8082 均校验 HTTP 头 `X-Auth-Token`。
- 不匹配 → 401 `{"error":"unauthorized"}`。

## 1. 子端 Agent :8100（内部协议，非 OpenAI）
请求头: `X-Auth-Token: <token>`，`X-Request-ID: <uuid>`（可选，主控透传）。

### GET /status → 200
```json
{
  "machine": "x3",
  "model": "deepseek-v4-flash" | null,
  "backend": "llama-server" | "ds4-server" | null,
  "port": 8123 | null,
  "mem_available_gb": 12.3,
  "mem_total_gb": 128.0,
  "load": 0.42,
  "models": ["deepseek-v4-flash", "GLM-5.2"],
  "uptime": 3600.0,
  "gpu_used_gb": 11.2,
  "backend_rss_gb": 6.1,
  "active_requests": 0,
  "healthy": true,
  "error": null | "熔断标记文本"
}
```
- mem_available_gb/mem_total_gb: 系统可用/总内存（Linux /proc/meminfo MemAvailable；macOS vm_stat）
- gpu_used_gb: X3 用 rocm-smi 显存占用；Mac 用统一内存
- backend_rss_gb: 当前后端进程 RSS（ps -o rss），无进程为 0.0
- active_requests: 当前 in-flight 请求计数
- healthy: 后端进程存活（或无需后端时为 true）；熔断后 false + error 文本

### POST /load  body `{"model":"GLM-5.2","request_id":"..."}`
- 200 `{"ok":true,"model":"GLM-5.2","backend":"llama-server","port":8123,"message":"loaded"}`
- 已加载同模型 → 200 同构
- 内存不足 → 507 `{"error":"insufficient memory","available":12.3,"required":110.0}`
- 队列满（本地排队上限 20）→ 429 `{"error":"queue full"}`
- 熔断（连续 3 次启动失败）→ 503 `{"error":"circuit open"}`
- 无双后端检测失败 → 409 `{"error":"another backend running"}`
- 模型不存在 → 404 `{"error":"unknown model"}`

### POST /infer  body `{"model":"...","request_id":"...","messages":[...],"stream":bool, ...}`（OpenAI 透传）
- 透传到本地后端 `POST /v1/chat/completions`（后端端口动态分配）
- 非流式: 原样返回后端 JSON（含 status code）
- 流式: SSE 逐行透传（Content-Type: text/event-stream）
- 后端未加载该模型 → 自动 load（崩溃自愈）后转发

### POST /unload  body `{"request_id":"..."}` → 200 `{"ok":true}`
### POST /infer-file（预留，本期不实现）→ 404

## 2. 子端 → 主控上报（agent.py 主动，每 5s）
### 心跳 POST `http://<controller>:8580/api/fleet/heartbeat`
- 头 `X-Auth-Token`；body = 与 /status 相同形状的快照 JSON（含 machine）
- 主控更新该 machine 在线状态 + 快照；连续 3 次心跳超时（15s）标记 offline

### 日志批量 POST `http://<controller>:8580/api/fleet/logs`（每 5s 一批增量）
- 头 `X-Auth-Token`
- body `{"machine":"x3","logs":[{"ts":"2026-08-07T15:00:00Z","level":"INFO","component":"agent","model":"deepseek-v4-flash","request_id":"uuid","message":"..."}]}`
- level ∈ INFO/WARN/ERROR/DEBUG；component ∈ agent/backend/health/heartbeat/fleet

## 3. 主控控制 API :8580 fleet 端点（FleetAPI.swift 扩展 ControlAPI）
- `GET /api/fleet/status` → 全群状态聚合（本机 + 各子端实时采样快照）
- `GET /api/fleet/models` → 聚合模型清单（fleet.yaml models 合并本机扫描）
- `POST /api/fleet/load` body `{"model":"..."}` → 手动加载（调试）
- `POST /api/fleet/unload` body `{"machine":"x3"}` 或 `{"model":"..."}` → 手动卸载
- `POST /api/fleet/heartbeat` → 子端心跳接收（校验 token，更新在线/快照）
- `POST /api/fleet/logs` → 子端日志批量接收（校验 token，落 SQLite）
- 日志查询：
  - `GET /api/fleet/logs/query?machine=&component=&since=&level=`
  - `GET /api/fleet/logs/trace?request_id=`
  - `GET /api/fleet/logs/errors?since=`
  - `GET /api/fleet/logs/stats?model=`

## 4. SQLite 日志表（FleetLogStore.swift）
- 库: `~/Library/Logs/llama-control/fleet_logs.db`
- 表 `logs(id INTEGER PRIMARY KEY AUTOINCREMENT, ts TEXT, machine TEXT, level TEXT, component TEXT, model TEXT, request_id TEXT, message TEXT)`
- 索引: ts / machine / component / request_id
- 保留 30 天

## 5. 统一入口 :8082（ProxyDaemon 扩展）
- 解析 model 参数 → FleetController 路由（本机 ServiceController 或 HTTP 调子端）
- 生成 `request_id`(UUID) 透传全链路
- 校验 X-Auth-Token
- 支持 `X-Client` 优先级: interactive > batch > background；高优先级可插队，队列满低优先级先丢 429

## 6. 主控轮询子端
- FleetController 每 5s GET 每个子端 `/status`（带 token），缓存到环形缓冲
- 双模型并发检测：一台机器任何时刻只允许一个后端进程存活；切换前确认（旧进程死+内存回落+无 in-flight）；检测双后端 → 告警+杀多余
- 本机请求完成后 60s 空闲自动卸载；X3/Mini 常驻

## 7. X3Monitor 数据源
- 默认调 `http://127.0.0.1:8580/api/fleet/status` 拉全群
- :8580 不可用时回退 SSH 直连 X3（保留现有逻辑）

## 8. 模型注册表 agent_models.yaml（agent.py 同目录）
```yaml
# 每个条目: name → file/backend/mem_gb/modality/cmd
# cmd 可选：覆盖后端启动命令（绝对路径，{port} 占位符）；默认按 backend 类型生成
deepseek-v4-flash:
  file: /data/models/DeepSeek-V4-Flash-IQ2XXS-w2Q2K-AProjQ8-SExpQ8-OutQ8-chat-v2-imatrix-0731.gguf
  backend: ds4-server
  mem_gb: 86
  modality: text
  cmd: [/home/g01/ds4-server, --rocm, --ctx, "8192", --threads, "16", --host, 127.0.0.1, --port, "{port}", --model, /data/models/DeepSeek-V4-Flash-IQ2XXS-w2Q2K-AProjQ8-SExpQ8-OutQ8-chat-v2-imatrix-0731.gguf]
GLM-5.2:
  file: /data/models/glm/GLM-5.2-UD-IQ2_XXS_RoutedIQ2XXS_blk78Q2K.gguf
  backend: ds4-server
  mem_gb: 100
  modality: text
  cmd: [/home/g01/ds4-server, --rocm, --ctx, "8192", --threads, "16", --host, 127.0.0.1, --port, "{port}", --model, /data/models/glm/GLM-5.2-UD-IQ2_XXS_RoutedIQ2XXS_blk78Q2K.gguf, --ssd-streaming, --ssd-streaming-cache-experts, "100GB"]
qwen2.5-3b:
  file: /models/qwen2.5-3b.gguf
  backend: llama-server
  mem_gb: 3
  modality: text
```
- file 用绝对路径（X3 实际路径由部署时按机器覆盖）；backend ∈ llama-server/ds4-server；modality 默认 text
- **cmd 必须用绝对路径**（如 /home/g01/ds4-server）——agent 默认从 PATH 找二进制，systemd 环境 PATH 不含 /home/g01
- **健康检查交替探测 /health 与 /v1/models**——ds4-server 无 /health（404），只有 /v1/models 200；死等单个路径会卡死 300s（2026-08-08 实测修复）

## 8.1 子端模型生命周期（用户定稿：agent 是唯一模型管理器）
- **agent.py 是子端唯一常驻/自启程序**（systemd enabled，开机自启）
- **后端模型（ds4-server/llama-server）一律由 agent 按需加载**：收到模型请求 → spawn 后端 → 健康检查 → 复用；保证**同一时刻只有一个模型后端在跑**（双后端检测 409 拒绝）
- **禁止后端单独自启**（systemd 不要 enable ds4-server/llama-server）——否则与 agent 的双后端检测冲突（409）且违背"单模型"原则
- 内存预算硬门槛：available ≥ mem_gb × 1.1（不足返回 507 insufficient memory）
- 切换模型：agent 先停旧（SIGTERM→30s→SIGKILL）再起新，零重叠窗口

## 9. 路由表 fleet.yaml（主控 FleetController 读取，三件套共用）
```yaml
auth:
  token: x3gw-shared-2026
models:
  example-35b:  { host: local, backend: llama-server, file: ~/models/example-35b-Q4_K_M.gguf, mem_gb: 22 }
  deepseek-v4-flash: { host: x3, backend: ds4-server, file: /data/models/DeepSeek-V4-Flash.gguf, mem_gb: 86 }
  GLM-5.2: { host: x3, backend: llama-server, file: /data/models/glm/GLM-5.2.gguf, mem_gb: 100, ssd: true }
  qwen2.5-3b: { host: mini1, backend: llama-server, file: /models/qwen2.5-3b.gguf, mem_gb: 3 }
fleet:
  x3:    { host: <worker-host>, port: 8100, os: ubuntu }
  mini1: { host: <worker-host>,  port: 8100, os: macos }
  mini2: { host: <worker-host>,  port: 8100, os: macos }
  mini3: { host: <worker-host>,  port: 8100, os: macos }
```
- host=local → 本机 ServiceController；否则调子端 :8100
- FleetController 读取路径默认 `<volume-path>`（可配置）

## 10. 部署脚本
- deploy_x3.sh: scp agent/agent.py + agent/agent_models.yaml → X3，写 systemd 单元 x3-agent.service 常驻 :8100（**只写文件不执行 systemctl restart**）
- **systemd 单元必须加 `--host 0.0.0.0`**——agent 默认监听 127.0.0.1（仅本机可达），主控跨机器访问不到（2026-08-08 实测：8100 只监听到 127.0.0.1，主控拉状态失败；加 --host 0.0.0.0 后正常）
- deploy_mini.sh: scp → Mini，launchd plist 常驻 :8100
- install.sh: 本机模型管理器 App 重新构建
- 子端路径: X3=/home/g01/agent，Mini=~/agent；日志 agent.log 同目录

## 11. 实测联调记录（2026-08-08）
- **X3 子端部署成功**：agent.py + systemd enabled，8100 监听 0.0.0.0，心跳 5s/次上报主控 <worker-host>:8580
- **主控识别 x3 在线**：FleetController /api/fleet/status 显示 x3 online=True
- **全链路打通**：:8082 统一入口 → FleetController 路由 → X3 agent spawn ds4-server → 推理返回（deepseek-v4-flash 回复正常）
- **踩坑**：
  1. agent 默认监听 127.0.0.1 → 必须 --host 0.0.0.0
  2. cmd 路径必须绝对（PATH 不含 /home/g01）
  3. ds4-server 无 /health（404）→ _wait_healthy 死等 300s 卡死 → 已修复为交替探测
  4. DS4 由 agent 管理（禁止 systemd 自启）——systemd 常驻会与 agent 双后端检测冲突
  5. machine 名必须匹配路由表（--machine x3，不能用主机名 evo-x3）——否则心跳进 snapshots["evo-x3"]，主控遍历 machines["x3"] 永远找不到 → offline
  6. llama-server Linux 参数用 -ngl 999（--rocm 是无效参数）
  7. agent 内存检查必须"先停旧再查预算"（否则切换时旧模型占着内存误拒 507）
  8. systemd 单元需 Environment=PATH=/usr/bin:/bin:/home/g01/llama（llama-server 不在默认 PATH）

## 12. 调度能力压测结果（2026-08-08）
- **多 worker 并发路由**：FleetController 改为按机器分组 worker（fleet-dispatch 分发 → 每机器独立 worker + 独立锁）——跨机器并行已验证（DS4/X3 + example-35b-v2/本机同时服务）
- **切换链路**：空→qwen3-32b→DS4→glm-z1→example-35b-v2 连续轮换全部 OK（agent 停旧起新 + 端口确认）
- **并发排队**：3 并发同模型全 OK（串行处理，无丢失无 429）
- **内存保护**：DS4 占用时其他模型正确 507；切换场景先释放再检查
- **统一接口**：调用方只认 :8082/v1/chat/completions，model 字段路由——接口不动换模型名

## 12. v2 Go 重写说明（2026-08-09）

主控与子端已从 Swift/Python 重写为 Go，协议保持 v1 兼容：

### 组件
- **zerg-core**（Go 主控，headless）：8580 API + 8082 三标准网关 + localback 本机子端
- **zerg-agent**（Go 子端）：X3(Linux) / Mini(macOS) 的 :8100 agent
- **ZergApp**（Swift 界面壳）：菜单栏，读 8580 显示（可选，关闭不影响核心）

### 新增字段
- 心跳新增 `backend_state`：`idle / loading:<模型名> / ready / crashed`（模型加载状态可见）
- /api/fleet/status 的 machines 为 dict（机器名 → 快照），含 backend_state

### 关键部署坑（macOS）
- Mini 的 Go agent **必须 LaunchDaemon(root) 部署**（/Library/LaunchDaemons/）
- SSH/nohup/LaunchAgent 启动会被 macOS TCC 本地网络权限拦截 → no route to host（系统二进制 curl 豁免）
- 二进制放 /usr/local/bin + 重新签名（/tmp 下 ad-hoc 签名 launchd 不信任，OS_REASON_CODESIGNING）
- Go 的 :port 默认 IPv6-only → 必须 0.0.0.0:port（子端 IPv4 连不上）

### 兼容性
- token/心跳/status/load/infer/unload 协议与 v1 完全一致
- 子端 agent_models.yaml 注册表兼容（cmd 支持字符串/数组两种格式）
