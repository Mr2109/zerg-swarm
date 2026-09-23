# 虫族 · Zerg Swarm

[English](README.md) | **中文**

> *中文原版（唯一真相源）。英文版为派生翻译；两版如有不一致，以**本文**为准。*

**自研本地 LLM 集群** —— 把几台普通机器（含 Apple Silicon Mac 与 NVIDIA 机器）组成一个"虫群"：
按需加载模型、按显存与负载选机、统一 OpenAI 兼容入口、自带任务自动执行与对话工作台。

> 设计理念：**没有弱模型，只有不完善的系统**。
> 模型任务失败、格式跑偏、工具调用异常——我们一律先归因**系统**（提示设计、解析层、防呆兜底、反馈协议），
> 而不是甩锅给模型。这条原则决定了本项目的很多设计（见 [docs/design/格式反馈协议-FFP.md](docs/design/格式反馈协议-FFP.md)）。

---

## 它解决什么问题

- **本地模型很多、显存不够**：同一台机器装不下所有模型 → 虫族按需求把模型**按需加载**到有显存的机器，用完可卸载
- **多机多引擎难统一**：llama.cpp / ds4-server / vLLM / 任何 OpenAI 兼容引擎 → 虫族统一成**一个入口**
- **本地模型"不听话"**：工具调用格式错、空参数、循环重发 → 系统层做格式反馈与防呆，而不是换更大的模型
- **想要自己的 AI 工作台**：对话、任务、工具、记忆、日志、集群状态 → 一套桌面 UI + HTTP API

## 架构（四件套）

```
                    ┌──────────────────────────────┐
                    │  zerg-ui（Rust/egui 桌面）    │
                    │  对话 · 任务 · 集群 · 工具 · 日志 │
                    └───────────────┬──────────────┘
                                    │ HTTP + X-Auth-Token
                    ┌───────────────▼──────────────┐
                    │  主控 zerg-core（Go，:8580）   │
                    │  调度 / 对话 / 工具 / 记忆 / 网关 │
                    └───────┬───────────────┬──────┘
        OpenAI 兼容入口      │               │  任务派发 + 心跳
        （:8082 网关）       │               │
                    ┌───────▼──────┐  ┌─────▼─────────────────────┐
                    │ 客户端/外部工具 │  │ 子端 zerg-agent（Go，:8100）│
                    └──────────────┘  │ 按需加载模型 · 健康自愈 · 熔断 │
                                      └─────┬─────────────────────┘
                                            │ 拉起/卸载
                                  ┌─────────▼─────────┐
                                  │ llama-server /    │
                                  │ ds4-server / 其他  │
                                  └───────────────────┘
```

| 组件 | 目录 | 语言 | 作用 |
|---|---|---|---|
| 主控 | `core/` | Go | 调度（按显存/负载选机）、对话与工具循环、记忆、网关（OpenAI 兼容 `:8082`） |
| 子端 | `agent/` | Go | 部署在每台工作机：按需加载/卸载模型、健康自愈、5 秒心跳上报 |
| 桌面 UI | `ui/` | Rust + egui | 对话、任务、集群状态、工具目录、文档、日志 |
| 菜单栏 | `app/` | Swift | macOS 只读状态面板（**可选**，不参与核心逻辑） |

## 特性

- **集群调度**：模型登记表声明"同一模型在多台机器上的候选"，运行时按显存/负载打分选机
- **按需加载**：不在用时自动卸载；同一机器**单槽**约束（一次只跑一个模型，冲突时整机清场）
- **多引擎**：任何 OpenAI 兼容后端皆可接入，不被单一引擎绑定
- **任务自动执行**：提交一个自然语言任务 → Agent 循环（工具调用 + 反馈 + 重试 + 熔断）→ 产出报告
- **工具系统**：文件读写/编辑、shell（带删除范围闸门）、搜索、截图 OCR、媒体处理、知识库、子任务拆解……共 135 个注册工具
- **记忆**：两级作用域（全局 + 每 Agent）+ 预算控制 + 压缩与召回回跳（见 [docs/design/记忆体系.md](docs/design/记忆体系.md)）
- **桌面工作台**：对话流式输出、Markdown 渲染、任务面板、集群/模型状态、日志、文档浏览

## 快速开始

**从源码装，一条命令。** `scripts/svc/setup-zerg.sh` 把一台新机器一次引导到位：
取公开镜像仓 → 按 `core/go.mod` 钉住的版本核验 Go 工具链 → 按角色构建 → 原子安装
（留 `bin/*.prev` 供回滚）→ 注册并重启服务 → 用**运行进程自报的提交号**与安装件对账。
它只会如实拒绝、绝不猜：工具链太旧、检出是脏树、在非 macOS 上要 UI —— 都会停下来说明原因。

```bash
git clone https://github.com/Mr2109/zerg-swarm.git && cd zerg-swarm

bash scripts/svc/setup-zerg.sh                 # 主控（macOS）：core + agent（+ ui）
bash scripts/svc/setup-zerg.sh --role node     # 机群节点（Linux）：core + agentd
bash scripts/svc/setup-zerg.sh --dry-run       # 只打印计划，零副作用
```

再配置共享令牌与模型登记表（macOS 上主控已由 launchd 托管；节点会得到一个管 `zerg-agentd`
的 systemd 单元）：

```bash
cp .env.example .env
printf '%s\n' "$(openssl rand -hex 32)" > .env.tmp && sed -i '' "s/^ZERG_AUTH_TOKEN=.*/ZERG_AUTH_TOKEN=$(cat .env.tmp)/" .env && rm .env.tmp
set -a; . ./.env; set +a
cp gateway/fleet.example.yaml gateway/fleet.yaml   # 按自己的机器/权重路径改
```

升级同样走源码 —— 客户端自己拉仓库、在本机构建，再把制品交给六阶段换装内核：

```bash
zerg-core update           # 已是最新则打印「无需更新」并以 2 退出
zerg-core update --check   # 先问一句（6 小时缓存，无副作用）
```

## 本仓库如何发布

本仓库是私有权威仓的**过滤镜像**。它是带真实历史的普通 git 仓，**逐提交一一对应**：
这里的每一笔提交与私有仓的一笔提交一一对应，并各自带 `GitOrigin-RevId:` trailer 记录原始
提交号。作者、日期与提交信息予以保留；私有面与私有称谓在镜像时已被替换（私有路径、内网
主机名、凭据 —— 某一笔的树若过不了泄露扫描，整批中止，一个字节都不推）。因此这里的
`git log`、`git blame`、`git diff`、`git bisect` 都能用；但某一笔的短 sha 与私有仓不同，
跨仓对照请用 trailer 回指。

细节（加子端、接模型、装桌面 UI）见 [docs/QUICKSTART.zh-CN.md](docs/QUICKSTART.zh-CN.md)。
配置项清单见 [docs/CONFIGURATION.zh-CN.md](docs/CONFIGURATION.zh-CN.md)。

## 环境要求

| 组件 | 要求 |
|---|---|
| 主控 / 子端 | Go，版本由 **`core/go.mod` 钉住**（构建）。运行时无额外依赖（SQLite 走纯 Go 驱动） |
| 桌面 UI | Rust stable（构建）；运行支持 macOS 14+ / Linux |
| 模型后端 | [llama.cpp](https://github.com/ggml-org/llama.cpp) 的 `llama-server`（推荐）或其他 OpenAI 兼容引擎 |
| 联网搜索（可选） | 自建 [searxng](https://github.com/searxng/searxng)（AGPL，**不随本仓库分发**）——见 `scripts/install-searxng.sh` |
| 命令输出压缩（可选） | [rtk](https://github.com/rtk-ai/rtk)（Apache-2.0）——**缺装自动降级**，也可 `ZERG_RTK=0` 关闭 |

## 文档

| 文档 | 内容 |
|---|---|
| [docs/QUICKSTART.zh-CN.md](docs/QUICKSTART.zh-CN.md) | 从零跑通：单机 → 加子端 → 接模型 → 桌面 UI |
| [docs/ARCHITECTURE.zh-CN.md](docs/ARCHITECTURE.zh-CN.md) | 组件职责、数据流、单槽机制、任务执行链路 |
| [docs/CONFIGURATION.zh-CN.md](docs/CONFIGURATION.zh-CN.md) | 端口、环境变量、状态目录、登记表字段 |
| [docs/MODELS.zh-CN.md](docs/MODELS.zh-CN.md) | 模型接入与适配器机制、上下文件、chat template |
| [docs/TOOLS.zh-CN.md](docs/TOOLS.zh-CN.md) | 工具目录（按用途分类） |
| [docs/FAQ.zh-CN.md](docs/FAQ.zh-CN.md) | 常见问题与排障 |
| [docs/design/](docs/design/) | 设计文档：格式反馈协议（FFP）、记忆体系、工具升级规范 |

每篇文档都有英文版（`docs/*.en.md`）。

## 许可与贡献

- **Apache-2.0**（见 [LICENSE](LICENSE)）；第三方组件见 [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)
- 贡献请用 **DCO**：`git commit -s`，详见 [CONTRIBUTING.md](CONTRIBUTING.md)
- 安全策略与"部署者须知"见 [SECURITY.md](SECURITY.md)

> 版权行 `Copyright 2026 The Zerg Swarm Authors`。Apache-2.0 **不授予商标许可**——"虫族 / Zerg Swarm"名称不在授权范围内。
