# X3 Model Gateway — 设计文档（v2：多机模型群 + 三件套融合）

> 2026-08-07 · 需求方：Mr2109 · 设计：Hermes · 实现：Codex
> 版本历史：v1 单机网关（已废弃）→ v2 多机模型群

## 一、需求（用户定稿）

1. **一个 API 接口**，不同的 model 参数调不同的模型，不同请求按排队机制，自主加载/卸载
2. **多机模型群**：本机(Mac M4 Max 64G) + X3(Ubuntu 128G ROCm) + M1 Mini×2(16G，第3台待入) 协同工作
3. **本机=主控端**（agent 都在本机），其他机器=被控子端
4. **本机只跑 llama 系模型**（example-35b-v2 等），DS4/GLM 只能 X3 跑
5. **防爆机制**：内存/请求/磁盘/进程四重防护
6. **三件套融合**：模型管理器 App（管理）+ 群调度网关（路由/排队/防爆）+ X3Monitor（可视化）合一
7. **跨平台**：子端 Agent 必须同时支持 Ubuntu(X3) 和 macOS(Mini)

## 二、总体架构

```
┌─────────────────────────────────────────────────────────────┐
│  主控端 = 本机 Mac M4 Max 64G                                │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ 模型管理器 App（Swift 菜单栏，:8580 ControlAPI）        │  │
│  │  ├─ ServiceController（已有）本机 llama-server 启停     │  │
│  │  ├─ ProxyDaemon（已有）:8082 统一代理                  │  │
│  │  ├─ ProcessManager（已有）进程生命周期                  │  │
│  │  └─ ★ FleetController（新增）多机群调度                │  │
│  │       ├─ 路由表: model → (子端, 后端类型, 本地模型名)   │  │
│  │       ├─ 全局排队 FIFO + 切换锁                        │  │
│  │       ├─ 内存/磁盘/进程防爆                            │  │
│  │       └─ 子端 Agent 客户端（HTTP 下发指令）             │  │
│  └──────────────────────────────────────────────────────┘  │
│  统一入口 :8082（agent 只认这一个口）                       │
│  本机也参与推理（llama-server :8081，example-35b-v2 35B）           │
└─────────────────────────────────────────────────────────────┘
   │                          │
   │ HTTP :8100               │ HTTP :8100
   ▼                          ▼
┌─────────────────┐    ┌─────────────────┐
│ X3 (Ubuntu 子端) │    │ Mini×2 (macOS)  │
│ 128G + ROCm     │    │ 16G             │
│ ★ 子端Agent      │    │ ★ 子端Agent      │
│ (Python:8100)   │    │ (Python:8100)   │
│ ├─ ds4-server   │    │ ├─ llama-server │
│ │  deepseek-v4  │    │ │  3B 常驻      │
│ ├─ llama-server│    │ └─ 小模型池      │
│ │  GLM/其他     │    └─────────────────┘
│ └─ 本地排队+防爆 │
└─────────────────┘
```

## 三、组件设计

### 3.1 子端 Agent（新写，跨平台 Python）

**技术**：Python 3 标准库（http.server + subprocess + threading），无第三方依赖。
**X3 = Ubuntu 26.04 python3；Mini = macOS python3（自带，若版本旧用 brew python3）。**

**职责**：
1. 监听 :8100，提供 HTTP API：
   - `GET /status` → 上报：当前模型/内存/负载/可用模型清单/后端类型
   - `POST /load` `{"model": "GLM-5.2"}` → 本地加载（停旧→起新→健康检查）
   - `POST /infer` `{"model": "...", "messages": [...]}` → 转发推理（透传 OpenAI 格式）
   - `POST /unload` → 卸载当前
2. 本地排队：子端内请求按序（同主控全局排队的下层缓冲）
3. 本地防爆：加载前检查本机内存，不足返回 507
4. 崩溃自愈：后端进程死亡 → 下次 load 自动重启
5. 日志：/home/g01/agent/agent.log（X3）、~/agent/agent.log（Mini）

**后端管理**（子端内）：
- llama-server：`llama-server -m <file> --host 127.0.0.1 --port <动态> -ngl 999(Apple) / --rocm(Linux)`
- ds4-server（仅 X3）：`/home/g01/ds4-server --rocm --ctx 262144 --threads 16 --host 127.0.0.1 --port <动态>`
- 停止：SIGTERM → 30s → SIGKILL → 确认端口释放
- 端口：8001-8090 动态分配

### 3.2 主控 FleetController（模型管理器 App 内新增）

**路由表**（YAML 配置，三件套共用）：
```yaml
models:
  example-35b:      { host: local,      backend: llama-server, file: ~/models/example-35b-Q4_K_M.gguf, mem_gb: 22 }
  deepseek-v4-flash:   { host: x3,         backend: ds4-server,   file: /data/models/DeepSeek-V4-Flash-...gguf, mem_gb: 86 }
  GLM-5.2:             { host: x3,         backend: llama-server, file: /data/models/glm/GLM-...gguf, mem_gb: 100, ssd: true }
  qwen2.5-3b:          { host: mini1,      backend: llama-server, file: ..., mem_gb: 3 }
  # mini2 / mini3(待入) 同理
fleet:
  x3:    { host: <worker-host>, port: 8100, os: ubuntu }
  mini1: { host: <worker-host>,  port: 8100, os: macos }
  mini2: { host: <worker-host>,  port: 8100, os: macos }
  mini3: { host: <worker-host>,  port: 8100, os: macos }   # 几天后加入
```

**调度逻辑**：
1. 请求进 :8082 → 解析 model → 路由表查 (host, backend)
2. host=local → 本机 ServiceController 处理（复用已有）
3. host=x3/mini → 调子端 Agent API（HTTP）
4. 全局排队：同一时间只有一次"加载/切换"动作（跨机器也串行，防两台子端同时加载大模型抢电/带宽——X3 独享大模型，Mini 常驻小模型不冲突）
5. 状态聚合：/v1/models 汇总所有子端 + 本机

**驻留策略（用户定稿，2026-08-07）**：
- **X3 / Mini：常驻**——专用机，用户不直接使用，模型常驻内存不卸载
  - X3 常驻 DS4（deepseek-v4-flash），GLM 按需切换（201G 太大不能与 DS4 并存，切换时卸载 DS4）
  - Mini 常驻各自小模型（如 qwen2.5-3b），随时响应
- **本机：按需加载 + 空闲即卸载**——这是用户的工作机，需要内存给剪辑/其他工作
  - 本机 llama-server 有请求才加载（或已有则复用）
  - **空闲即卸载**：本机模型 idle 后立即/短时（如 60s）卸载释放内存
  - 本机默认不常驻任何模型（8081 由调度器接管，不再手动常驻）

**本机空闲卸载细节**（用户工作机铁律）：
- 请求完成后启动短计时器（60s），无新请求 → 卸载本机模型，内存归还
- 与 X3/Mini 的"空闲 30 分钟卸载"不同——本机要激进得多
- 用户明确："没有调用就要卸载模型"

**防爆（主控级，v2 升级：实时监控 + 动态拦截，用户定稿 2026-08-07）**：

核心思路：**防止爆，不是加载前算一次账——而是实时监控内存/显存，动态检测"是否两个模型同时被调用"**。

1. **实时内存监控（周期性采样，非一次性检查）**
   - 主控每 5s 采样所有机器（本机 + 各子端）的：
     - 系统内存：`MemAvailable`（/proc/meminfo，Linux）/ `vm_stat`（macOS）——不用 free（buff/cache 可回收）
     - 显存/GTT：X3 上报 ROCm 显存占用（`rocm-smi` 或 ds4-server 状态）；本机/Mini 上报 GPU 统一内存占用（`sysctl`/`powermetrics`）
     - 模型进程实际 RSS：子端 Agent 上报当前后端进程的 RSS（`ps -o rss`）
   - 采样数据进环形缓冲，供告警/决策

2. **双模型并发检测（核心防线）**
   - 规则：**同一台机器上，任何时刻只允许一个模型后端进程存活**（本机/Mini 各自同理）
   - 切换加载新模型前，必须确认：
     a. 旧后端进程已退出（SIGTERM→30s→SIGKILL→waitpid 确认非僵尸）
     b. 旧模型内存已回落（RSS 归零/接近基线，采样确认）
     c. 旧模型没有 in-flight 请求（请求计数归零，或等待其完成——见第 4 条）
   - 检测到双模型（异常情况：比如手动启动的残留进程）→ **立即告警 + 杀多余进程**，不静默

3. **内存预算硬门槛（静态兜底，与实时监控配合）**
   - 加载前仍查 `available >= mem_gb * 1.1`，不足 507 拒载
   - 但这只是第一道门——实时监控才是持续防线（运行时内存漂移/KV 膨胀也能发现）

4. **切换安全窗口（防"切换瞬间双模型"）**
   - 旧模型有 in-flight 请求时：**不硬切**——新模型请求排队等待，旧模型请求完成后再切换
   - 硬切换仅当：旧模型无活动请求 + 进程已死 + 内存已回落
   - 例外：旧模型请求挂死（超时）→ 强制终止切换，日志记录

5. **请求/磁盘/进程防爆（保留 v1）**
   - 请求：队列上限 20 → 429；等待超时 600s → 504
   - 磁盘：load 前检查 df
   - 进程：连续 3 次启动失败 → 熔断标记 error，人工干预
   - 空闲卸载策略差异化：本机 60s 激进卸载（工作机），X3/Mini 常驻（专用机，仅切换时卸载）

**实时监控数据流**：
```
子端 Agent（每 5s）→ 主控 FleetController 采样缓存
   上报: mem_available / gpu_used / backend_rss / active_requests
主控决策: 
   加载前: 静态预算 + 实时确认（无双模型/内存充足/无 in-flight）
   运行中: 周期检查 RSS 漂移 / 双后端并存 / 内存异常
   异常: 杀进程 + 告警日志 + 状态标记
```

### 3.3 X3Monitor 扩展（可视化层）

- 数据源：从「SSH 直连 X3」改为「调主控 :8580 API 拉全群状态」
- 显示：本机/X3/Mini1/Mini2/Mini3 每台的状态卡片（当前模型/内存/GPU/队列长度）
- 菜单栏简洁显示：X3 GPU + 活跃子端数（如 `GPU 100% · 3端在线`）

### 3.4 可靠性设计（P0 红线，用户审查补缺 2026-08-07）

1. **心跳与掉线处理**（无此=事故）
   - 子端 Agent 每 5s 主动上报心跳（含状态快照）到主控
   - 主控连续 3 次心跳超时（15s）→ 标记该子端 `offline`
   - offline 子端的请求：立即返回 503（不排队死等），附"子端离线"原因
   - 子端 Agent 由 systemd/launchd 托管，崩溃自动拉起；拉起后自动重新注册
   - 主控侧：路由表子端 IP 支持 DHCP 变化（雷电串联后建议固定 IP/主机名，待确认）

2. **访问控制（Token 认证）**
   - 主控 ↔ 子端之间共享 token（fleet.yaml 配置，硬编码即可，局域网自用）
   - 子端 Agent 所有端点校验 `X-Auth-Token`，不匹配返回 401
   - :8082 统一入口同样校验（本机 agent 调用自动带 token，通过环境变量/配置）
   - 防止局域网任意设备 load/unload 模型、白嫖 GPU

3. **多客户端优先级（防互相打架）**
   - 请求带来源标识：`X-Client: hermes|codex|yuanpai|...`
   - 优先级分级：interactive（用户交互，最高）> batch（批处理）> background（后台）
   - 同源亲和：同 client 连续请求优先聚到一起（减少切换）
   - 抢占策略：高优先级请求可插队到队列前部（不打断 in-flight，只影响排队顺序）
   - 队列满时：低优先级先丢（429），高优先级保留

4. **崩溃自愈矩阵**
   | 故障 | 检测 | 恢复 |
   |---|---|---|
   | 子端 Agent 崩溃 | systemd/launchd | 自动拉起 + 重新注册 |
   | 后端模型进程崩溃 | 健康检查 | 下次请求自动重载 |
   | 主控 FleetController 崩溃 | App 重启 | 路由表重载 + 子端状态恢复 |
   | 子端整机掉电 | 心跳超时 | 标记 offline，请求 503，恢复后自动回群 |

### 3.5 统一日志系统（用户强调：强大日志，查问题用）

**设计目标**：未来大量问题要靠日志排查——日志必须**跨机器聚合、结构化、可查询、带请求追踪**。

1. **日志分级（每台机器两层）**：
   - 子端 Agent 日志：加载/卸载/切换/健康检查/错误/心跳事件
   - 后端进程日志：llama-server/ds4-server 的 stdout/stderr（推理错误、OOM）
   - 主控 FleetController 日志：路由决策/排队/切换/防爆动作/子端状态变更

2. **聚合机制（子端 → 主控）**：
   - 子端 Agent 所有日志同时写本地文件 + **增量上报主控**（HTTP POST，批量，5s 一批）
   - 主控统一落库：**SQLite（~/.hermes/... 或 App 数据目录 fleet_logs.db）**
   - 表结构：`logs(id, ts, machine, level, component, model, request_id, message)`
   - 关键字段索引（ts/machine/component/request_id）

3. **请求追踪（request_id 贯穿全链路）**：
   - 每个请求主控生成 `request_id`（UUID），透传到子端、后端、日志
   - 一条请求的完整链路：`路由 → 排队 → 加载 → 推理 → 响应` 全部打上同一 request_id
   - 查问题：按 request_id 一条命令拉全链路日志

4. **查询工具**：
   - `fleet_logs.py`（主控侧小工具）：
     - `logs query --machine x3 --component agent --since 10m`
     - `logs trace <request_id>`（全链路）
     - `logs errors --since 1h`（只看错误）
     - `logs stats --model GLM-5.2`（某模型的加载/卸载/失败统计）
   - X3Monitor 菜单栏加"最近错误"角标（可选，二期）

5. **日志轮转**：
   - 本地文件：按大小轮转（100MB 保留 5 份）
   - 聚合库：按时间清理（保留 30 天，或用户配置）

6. **告警钩子（日志驱动）**：
   - 主控扫描日志：连续 3 次加载失败 / 双模型检测触发 / 心跳丢失 → 写 error 级日志
   - X3Monitor 显示 ⚠ 角标（红点），点开看详情

### 3.6 三件套融合点

| 组件 | 现状 | 融合后 |
|---|---|---|
| 模型管理器 App | 本机单机管理 | + FleetController 群调度 + 子端管理 |
| ProxyDaemon :8082 | 本机转发 | + 多机路由 + 排队 + 防爆 |
| X3Monitor | SSH 监控 X3 | 调 :8580 监控全群 |

## 四、API 兼容

- :8082 统一 OpenAI 兼容（/v1/chat/completions、/v1/models）
- 透传 /v1/responses（Codex 依赖，仅 X3 ds4-server 支持时透传）
- 子端 Agent :8100 用简化内部协议（非 OpenAI，主控翻译）

## 四·五、网络拓扑（重要约束）

- 当前：全部走 WiFi（约 24MB/s），模型传输慢但推理请求带宽足够
- 未来：所有机器有雷电口，购买雷电 4 线后改雷电串联（~5GB/s，快 200 倍）
- **架构零改动**：子端 Agent 走 HTTP/TCP/IP，雷电串联后 IP/协议不变，仅链路变快
- 雷电线到位后：模型传输直接走雷电，不再用 WiFi（GLM 201G 从 2 小时 → 40 秒）

## 四·六、多模态扩展（前瞻设计，用户 2026-08-07 提出）

**现状**：所有模型都是文字模型（chat/completions JSON 输入输出）。
**未来**：大量音频、视频本地模型会加入群（ASR/TTS、视频理解/生成等）。

**设计预留（现在不改代码，但架构必须兼容）**：

1. **模态字段进路由表**（fleet.yaml 每个模型加 `modality: text|audio|video|multimodal`）
   - 路由/排队/防爆逻辑不变（模态不影响内存预算和单后端约束）
   - 只有"推理协议"按模态区分

2. **推理协议分层**：
   - text：现有 OpenAI chat/completions JSON（透传）
   - audio/video：大文件输入输出，走**文件引用协议**——客户端上传/指定文件路径 → 主控转发 → 子端推理 → 返回结果文件引用（不是 JSON 里塞 base64 大文件）
   - 子端 Agent 预留 `/infer-file` 端点（现在不实现，接口形态预留）

3. **机器分工扩展**（模态亲和性）：
   - X3（128G ROCm）：可承载视频生成/大模型推理（显存大）
   - 本机 Mac（M4 Max 统一内存）：适合音频（Whisper 等，Metal 加速好）
   - Mini（16G）：仅轻量文字模型（3-8B），多模态不进 Mini（内存不够）
   - 路由表按 `modality + mem_gb` 决策机器

4. **音频/视频模型的驻留策略**：
   - 同样遵循"一台机器一个后端进程"（防爆规则不变）
   - 按需加载 + 空闲卸载（与文字模型同一套生命周期管理）
   - 常驻与否同样遵循用户定稿：X3/Mini 常驻、本机空闲即卸

5. **未来扩展步骤**（不在本期实现，仅在设计预留）：
   - 本期：路由表加 modality 字段（默认 text）
   - 未来：子端 Agent 加 /infer-file；主控加文件传输通道；X3Monitor 显示模态图标

## 五、部署步骤

1. **X3**：装 python3 子端 Agent（scp agent.py + systemd 常驻 :8100）
2. **Mini×2**：装 python3 子端 Agent（scp agent.py + launchd 常驻 :8100）
3. **本机**：模型管理器 App 升级（FleetController + 路由表 + X3Monitor 扩展）
4. **配置**：路由表 YAML 统一维护

## 六、验收标准

- [ ] 子端 Agent 在 X3(Ubuntu) 跑通：/status、/load、/infer、/unload
- [ ] 子端 Agent 在 Mini(macOS) 跑通：同上
- [ ] 主控 :8082 调 example-35b-v2（本地）成功
- [ ] 主控 :8082 调 deepseek-v4-flash（X3）成功——自动经子端
- [ ] 切模型：本地→X3 自动切换，全程无人工
- [ ] 排队：切换期间并发请求按序不丢
- [ ] 防爆：内存不足拒绝（507）、队列超限（429）、超时（504）
- [ ] X3Monitor 显示全群状态（从 :8580 拉）
- [ ] 崩溃自愈：kill 后端 → 下次请求自动重启
- [ ] 全流程日志

## 七、分工

- **Hermes**：设计、验收、部署协调、X3/Mini 端部署执行
- **Codex**：子端 Agent（agent.py 跨平台）、主控 FleetController（Swift，集成进模型管理器 App）、X3Monitor 扩展、路由表配置
