# TASK — 实现 X3 Model Gateway（v2：多机模型群 + 三件套融合）

## 背景
阅读同目录 `DESIGN.md`（v2 设计文档，必须严格遵循）。实现多机模型群调度：
- 主控端（本机 Mac）= 模型管理器 App 升级（FleetController 群调度）
- 子端 Agent（X3 Ubuntu + Mini macOS）= 跨平台 Python 服务
- 统一入口 :8082（model 参数路由 + 排队 + 加载卸载 + 防爆）
- 三件套融合：模型管理器 + 群调度 + X3Monitor 扩展

## 实现要求（按优先级）

### P0：子端 Agent（跨平台 Python，独立可交付）
`agent.py` — 单文件，**只用 Python 3 标准库**（http.server/subprocess/threading/json），同时跑在 Ubuntu(X3) 和 macOS(Mini)。

功能：
1. 监听 :8100：
   - `GET /status` → `{model: 当前模型名|none, mem_available_gb, mem_total_gb, load, models: [可用模型清单], backend: llama-server|ds4-server, uptime, gpu_used_gb, backend_rss_gb, active_requests}`
   - `POST /load {"model": "名字"}` → 停旧→起新→健康检查→返回结果；若已加载同模型直接 ok
   - `POST /infer` → 透传 OpenAI 兼容请求到本地后端（chat/completions），返回后端响应（含流式 SSE 透传）
   - `POST /unload` → 卸载当前模型
2. 实时监控上报（防爆核心，用户定稿）：
   - **每 5s 主动采样**并缓存：系统内存可用（Linux: /proc/meminfo MemAvailable；macOS: vm_stat）、GPU/显存占用（X3: rocm-smi；Mini: sysctl 统一内存）、后端进程实际 RSS（ps -o rss）、当前 in-flight 请求计数
   - `GET /status` 返回最新采样（主控拉取即得实时数据）
   - 主控通过周期轮询 /status 实现"双模型并发检测"：任何时刻一台机器只允许一个后端进程存活
3. 模型注册表：`agent_models.yaml`（模型名 → 文件路径/后端类型/启动参数/mem_gb/**modality**），与 agent.py 同目录；modality 默认 text，本期全部 text，字段预留（未来 audio/video 用）
4. 后端管理：
   - llama-server：`llama-server -m <file> --host 127.0.0.1 --port <动态8001-8090> -c 65536` + 平台参数（macOS 加 `-ngl 999`；Linux 加 `--rocm` 若可用，否则 `--gpu-layers 0` 降级 CPU）
   - ds4-server（仅 X3 注册）：`/home/g01/ds4-server --rocm --ctx 262144 --threads 16 --host 127.0.0.1 --port <动态>`
   - 停止：SIGTERM → 30s → SIGKILL → 确认端口释放（lsof/ss 检查）
   - 健康检查：轮询 `GET /health` 或 `/v1/models`，最多 300s，失败杀进程重试（最多 3 次）
5. 防爆（子端级）：load 前检查 `available >= mem_gb * 1.1`，不足 507；**load 前确认本机无其他后端进程存活**（双模型检测本地版）；连续 3 次启动失败 → 熔断标记 error
6. 心跳（可靠性 P0）：每 5s 主动 POST 心跳到主控 `POST /api/fleet/heartbeat`（带状态快照）；主控连续 3 次超时标记 offline
7. Token 认证（可靠性 P0）：所有端点校验 `X-Auth-Token`（fleet.yaml 配置共享 token），不匹配 401
8. 日志（日志系统 P0）：所有日志写本地 agent.log + **增量批量上报主控**（POST /api/fleet/logs，5s 一批）；每条日志带 `request_id`（主控透传）

### P1：主控 FleetController（集成进模型管理器 App）
在 `<volume-path>` 的 Swift 工程中新增：
1. `Sources/LlamaControlCore/FleetController.swift`：
   - 路由表加载（fleet.yaml：model → host/backend/file/mem_gb）
   - 调度：请求 → 路由 → 本机 ServiceController 处理 或 HTTP 调子端 Agent
   - 全局排队：队列上限 20、等待超时 600s、单例切换锁
   - **实时监控采样（防爆核心）**：每 5s 轮询所有子端 /status + 本机状态，缓存内存/显存/RSS/请求数；环形缓冲
   - **双模型并发检测**：任何时刻一台机器只允许一个后端进程；切换前确认（旧进程死 + 内存回落 + 无 in-flight）；检测到双后端 → 告警 + 杀多余进程
   - **切换安全窗口**：旧模型有 in-flight 请求 → 新请求排队等旧完成；挂死超时 → 强制切换 + 日志
   - 静态预算兜底：加载前 available >= mem_gb*1.1，不足 507
   - 驻留策略：X3/Mini 常驻不动；本机请求完成后 60s 空闲自动卸载
2. `Sources/llama-control/Services/FleetAPI.swift`：ControlAPI(:8580) 扩展端点：
   - `GET /api/fleet/status` → 全群状态（各子端实时采样 + 本机）
   - `GET /api/fleet/models` → 聚合模型清单
   - `POST /api/fleet/load` / `unload` → 手动控制（调试用）
   - `POST /api/fleet/heartbeat` → 子端心跳接收（更新子端在线状态/快照）
   - `POST /api/fleet/logs` → 子端日志批量上报接收（落 SQLite）
3. `Sources/LlamaControlCore/FleetLogStore.swift`（日志系统 P0）：
   - SQLite 存储（fleet_logs.db），表 `logs(id, ts, machine, level, component, model, request_id, message)`
   - 索引：ts/machine/component/request_id
   - 聚合：子端上报日志 + 本机日志统一入库
   - 查询 API：按 machine/component/since/level 过滤、按 request_id 全链路 trace
4. `Sources/llama-control/Services/FleetLogQuery.swift`：日志查询端点（:8580）：
   - `GET /api/fleet/logs/query?machine=&component=&since=&level=`
   - `GET /api/fleet/logs/trace?request_id=`
   - `GET /api/fleet/logs/errors?since=`
   - `GET /api/fleet/logs/stats?model=`
5. `ProxyDaemon` 扩展：:8082 请求解析 model → 走 FleetController（本地/远端）；**生成 request_id 透传全链路**；校验 X-Auth-Token；支持 X-Client 优先级（interactive > batch > background，插队/丢弃策略）

### P2：X3Monitor 扩展（可视化）
`<volume-path>`：
1. 数据源从「SSH 直连 X3」改为「调主控 :8580/api/fleet/status」
2. 下拉菜单显示：每台机器（本机/X3/Mini1/Mini2/Mini3）当前模型/内存/GPU/状态
3. 菜单栏简洁显示：`GPU xx% · N端在线`
4. 保留 SSH 直连作为 X3 单机回退（:8580 不可用时）

### P3：部署脚本
- `deploy_x3.sh`：scp agent.py + agent_models.yaml → X3，systemd 单元（x3-agent.service）常驻 :8100
- `deploy_mini.sh`：scp → Mini，launchd plist 常驻 :8100
- `install.sh`：本机模型管理器 App 重新构建

## 环境
- X3: g01@<worker-host> —— **推荐 ssh key 免密**（已配好，`ssh -o BatchMode=yes` 可直接连）；如需密码，从环境变量 `X3_PASS` 提供，勿写进仓库
- Mini: Mr2109@<worker-host> / .137 —— 同上，走 ssh key；Mini SSH 认证当前被拒，部署前先解决登录（手动或配密钥）
- 本机: ~，模型管理器 App 在 <volume-path>
- 模型文件：本机 ~/models/*.gguf；X3 /data/models/**/*.gguf

## 验收
- [ ] agent.py 本机语法 + 单测通过（mock 后端：假 llama-server 进程模拟加载/排队/内存检查）
- [ ] agent.py 心跳上报（mock 主控接收）、token 校验（无 token 401）、日志批量上报（mock 接收）单测通过
- [ ] agent.py 在 X3 跑通（部署后 /status 返回真实数据，/load deepseek-v4-flash 成功）
- [ ] agent.py 在 Mini 跑通（或明确阻塞点）
- [ ] 主控 FleetController 编译通过 + 单测（mock 子端 HTTP）
- [ ] FleetLogStore SQLite 落库 + 查询单测通过（写入/按条件查/trace）
- [ ] ProxyDaemon 路由测试：model=example-35b-v2 → 本机；model=deepseek-v4-flash → X3（mock 或真实）；request_id 生成透传；token 校验
- [ ] 优先级测试：interactive 插队 / 队列满低优先级丢弃
- [ ] X3Monitor 改 :8580 数据源编译通过
- [ ] 全部 git commit

## 注意
- 本机是工作机：FleetController 本机模型请求完成后 60s 必须卸载（用户铁律）
- 子端 Agent 必须跨平台：不要在代码里写死 /home/g01（X3）或 /Users（Mini），用 os.path.expanduser 或相对路径 + 注册表配置
- llama-server 平台参数差异（-ngl 999 vs --rocm）必须在 agent.py 内按平台自动选择
- 密码/私钥硬编码仅限本地自用工具（用户明确无安全要求）
