# 架构（ARCHITECTURE）

[English](ARCHITECTURE.en.md) | **中文**

> *中文原版（唯一真相源）。英文版（[ARCHITECTURE.en.md](ARCHITECTURE.en.md)）为派生翻译；两版如有不一致，以**本文**为准。*

四个进程 + 一层"登记表"驱动的调度。所有组件只通过 HTTP 通信，令牌统一，无中心化数据库。

---

## 1. 组件与职责

### 主控 `zerg-core`（Go，`:8580`）

| 子系统 | 目录 | 职责 |
|---|---|---|
| 集群/调度 | `core/internal/api`、`core/internal/inference` | 模型候选打分（显存/负载/在线）、任务排队与派发、心跳汇总 |
| 网关 | `core/internal/gateway` | OpenAI 兼容入口（`:8082`），认证透传、路由到本机或子端、SSE 用量采样、前缀缓存命中率统计 |
| 对话 | `core/internal/chat` | 会话存储（SQLite + trigram FTS）、流式推理、工具循环、压缩与检索 |
| 工具 | `core/internal/agent` | 工具实现（文件、shell、搜索、截图、媒体、知识库…）+ 执行闸门 + 格式反馈协议 |
| 记忆 | `core/internal/memory` | 两级作用域条目、预算、威胁扫描、提示注入 |
| 任务 Agent | `core/internal/agent` + `cmd/zerg-agent` | CA（虫族 Agent）任务循环：子任务拆解、工具执行、反馈、熔断、报告 |

### 子端 `zerg-agent`（Go，`:8100`）

部署在每台工作机上，**唯一职责是把模型跑起来并保持健康**：

- HTTP 端点：`/status` `/load` `/infer` `/unload`
- 按需加载：根据登记表调用后端（`llama-server` / `ds4-server` / 自定义 `cmd`）
- 健康与自愈：就绪探测、崩溃重启、熔断；每 5 秒心跳上报主控
- **单槽约束**：一台机器同一时刻只跑一个被虫族托管的模型（冲突时整机清场，避免显存打架）

### 桌面 UI `zerg-ui`（Rust + egui）

纯客户端：对话 / 任务 / 集群 / 工具目录 / 文档 / 日志。所有请求走带令牌的 HTTP；
内部统一使用一个不经过系统代理的 HTTP client（回环请求必须绕过代理，否则会被本机代理吞掉）。

### 菜单栏 `app/`（Swift，可选）

macOS 只读状态面板，展示集群在线情况与模型占用。不参与核心逻辑，删掉不影响任何功能。

---

## 2. 数据流

### 推理请求（外部客户端）

```
客户端 → :8082 网关（校验令牌）
        → 路由决策：本机 llama-server ？ 还是某台子端？
        → 目标后端 /infer（SSE 流式）
        → 网关采样 usage/命中率，回传客户端
```

### 任务（CA）

```
POST /api/tasks
   → 主控写入任务队列
   → 调度：选模型 → 选机器（候选打分；必要时令子端加载模型）
   → spawn `zerg-agent -json -model <m> -workdir <任务目录>`
   → Agent 循环：模型调用 ↔ 工具执行（每轮带反馈）
   → 产物写入任务目录（报告/文件），状态与事件落盘
   → UI 轮询状态与日志
```

任务执行**不在主控进程内**：主控只负责调度与观测，执行在独立子进程，因此崩溃/超时不会拖垮主控。

---

## 3. 调度与"单槽"

- **登记表** `gateway/fleet.yaml` 声明：模型 → 候选列表（每台机器一条，含 `file` / `mem_gb` / `ctx_window`）
- **打分**：按可用显存、当前负载、是否已加载（热启动优先）、在线状态综合排序
- **单槽**：`localback` 与子端都对"整机只有一个托管模型"负责——这是显存现实的妥协，而不是设计缺陷
- **卸载**：空闲或需要腾地方时调子端 `/unload`

---

## 4. 工具系统与安全

工具分两层：

1. **常驻工具**（prompt 里直接注入定义，约 14 个）：读写/编辑文件、shell、搜索等高频能力
2. **延迟工具**（约 121 个，通过 `tool_search` 按需发现后调用）：媒体、知识库、文档、集群操作等

安全设计（`core/internal/agent`）：

- **删除范围闸门**：shell 的 `rm` 只允许作用于 { 任务工作区, 白名单目录 }；家目录、根目录、其祖先一律拒绝
- **文件工具路径域**：读写/edit 只允许 { 任务目录, `ZERG_EXTRA_ALLOW_DIR` }，越界即拒绝
- **拒绝必须"可执行"**：被拒绝时返回的是"合法可去处"的指引（哪个目录可以写、命令该怎么改），而不是干巴巴的 error——这是"没有弱模型，只有不完善的系统"在工具层的落地
- **格式反馈协议（FFP）**：工具参数格式错误（空参数、JSON 坏掉）时返回"系统断言 + 正确示例"，而不是让模型重发同样的坏请求；见 [design/格式反馈协议-FFP.md](design/格式反馈协议-FFP.md)

---

## 5. 记忆与对话

- **对话**：SQLite 单库（默认 `~/.zerg-chat/chat.db`），消息表 + trigram FTS（中文可检索）
- **压缩**：单一入口 `MaybeCompact`，按冷却（60s/300s/900s）+ full jitter 退避，连续失败三次熔断；压缩后保留**可回跳的原文指针**（`GET /api/chat/sessions/{id}/window`），UI 里可直接跳回被压缩的原文
- **记忆**：两级作用域（全局 + 每 Agent），条目化 + 预算上限 + 写入拒绝矩阵；细节见 [design/记忆体系.md](design/记忆体系.md)
- **会话检索**：`session_search` 支持 `include_archived=true` 搜索归档会话（默认排除归档）

---

## 6. 状态与落盘位置

| 内容 | 默认位置 | 覆盖变量 |
|---|---|---|
| 工具计数/事件/错误桶 | `~/.zerg/state/` | `ZERG_STATE_DIR` |
| 对话库 | `~/.zerg-chat/chat.db` | `ZERG_CHAT_DB_PATH` |
| 记忆 | `~/.zerg/memory/` | `ZERG_MEMORY_DIR` |
| 共享令牌 | `~/.zerg/token` | `ZERG_AUTH_TOKEN` |
| 任务目录 | `/tmp/zerg-tasks/<id>/` | `ZERG_TASK_ROOT` |
| CA 事件日志 | `/tmp/zerg-ca-logs/<ts>/events.jsonl` | `ZERG_CA_LOG_DIR` |
| 运行日志 | `/tmp/zerg-*.log` | `ZERG_LOG_DIR` |
| UI 偏好 | `~/.zerg-ui-prefs.json` | — |

> HTTP 服务的端口、路径、外部数据位置（知识库、searxng、媒体目录）**全部**可通过环境变量覆盖，见 [CONFIGURATION.zh-CN.md](CONFIGURATION.zh-CN.md)。

---

## 7. 扩展点

| 想做的事 | 改哪里 |
|---|---|
| 接入新模型家族（特殊启动参数/思考格式/工具风格） | `core/internal/chat/...` 适配器（按模型名前缀派发）+ `agent/internal/modeladapter` |
| 接入新推理引擎 | 子端登记表 `backend:` + 自定义 `cmd:`；或实现一个新的 adapter |
| 加一个工具 | `core/internal/agent`（实现 + 注册表 + 简介），并同步 `tools/<name>.md` 履历 |
| 加一个"虫茧"应用（UI 里的独立面板） | `ui/src/modules/`（可选 feature 方式编译进来，见 `zerg-roundtable` 的处理） |
| 换掉 UI | 只要遵守 HTTP API + 令牌，任意前端都可以 |

---

## 8. 设计取舍（写在明面上的）

- **单槽而非多槽**：显存是硬约束，宁可排队也不让两个模型互抢 OOM
- **HTTP + 令牌而非消息队列**：部署简单、可 curl 调试；代价是需要自己保证令牌安全
- **纯本地**：不向作者回传任何数据。代价是每个部署者要自己管好令牌与端口暴露（见 [SECURITY.md](../SECURITY.md)）
- **不做模型微调**：虫族是"**编排 + 系统改造**"路线——把现成模型用好，而不是自己训模型

---

## 9. 托管与装载

- **桌面 UI 由系统服务托管**：`zerg-ui` 跑在 launchd 常驻服务 `com.zerg.ui` 上（服务件 `~/Library/LaunchAgents/com.zerg.ui.plist`，`KeepAlive` + `RunAtLoad`）——掉线由系统拉起，不靠手工裸启；现状现读 `launchctl print gui/$(id -u)/com.zerg.ui`（`state = running`）
- **归档区是只读镜像挂载**：归档内容挂成一个**只读卷**（挂载点在仓外），仓边的**原路径是同名符号链接**指向它——所以读原路径 = 读只读镜像里的件，往它写必然被拒（这不是权限位没设对，`chmod` 也改不动只读卷）。装与卸只有一个入口：`bash scripts/svc/archive-mount.sh on|off|status`（**按需 · 不常驻 · 不装 launchd**）；未挂那一态原路径是**悬空符号链接**，读它会明确失败，`status` 会报"未挂"并退码 1
