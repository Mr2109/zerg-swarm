# zerg-agentd — 虫族子端 Agent（CA）

> 名字沿革：本件在**早期**叫 `zerg-agent`，与命令面 CLI（`core/cmd/zerg-agent`）**撞名**；
> 现名 **`zerg-agentd`**（制品名 `zerg-agentd-<os>-<arch>`）。★ 旧名 `zerg-agent-<os>-<arch>` 归属
> **命令面 CLI**（见 `publish/ci/release-agent.yml`），**不是本件** —— 别再按旧名找子端件。

集群的「另一半」：主控（`zerg-core`）负责调度，**子端 Agent 负责在每台工作机上真正把模型跑起来**。

## 职责

- HTTP 服务（默认 `:8100`）：`/status` `/load` `/infer` `/unload`
- 按需加载模型（后端 `llama-server` / `ds4-server`），支持单槽与多槽策略
- 健康检查 + 崩溃自愈 + 熔断
- 每 5 秒向主控心跳上报（在线状态、当前模型、显存占用）
- 认证：`X-Auth-Token`（与主控共享令牌）

## 构建

```bash
cd agent
go build -o zerg-agentd ./cmd/zerg-agentd
# 交叉编译示例（Linux 工作机）
GOOS=linux GOARCH=amd64 go build -o zerg-agentd-linux-amd64 ./cmd/zerg-agentd
```

## 运行

```bash
# 令牌**不走 argv**（会进本机 `ps` 与 shell 历史 ✗）：本机跑读 600 档 `~/.zerg/token`；
# 跨机由部署脚本入口写入 600 环境档（单元/plist 只引用它）——
#   gateway/deploy_x3.sh --token-file <600档> / printf '%s\n' <令牌> | $0 --token-stdin …
# 二进制名 zerg-agentd（本机件 bin/zerg-agentd；跨平台件 bin/zerg-agentd-<os>-<arch>）
./zerg-agentd \
  --host 0.0.0.0 \
  --port 8100 \
  --machine <本机标识> \
  --controller http://<主控地址>:8580 \
  --registry agent_models.yaml
```

| 参数 | 默认 | 说明 |
|---|---|---|
| `--host` | `127.0.0.1` | 监听地址；跨机部署用 `0.0.0.0` |
| `--port` | `8100` | 监听端口 |
| `--controller` | `http://127.0.0.1:8580` | 主控地址（心跳上报目标） |
| `--machine` | 主机名 | 机器标识（主控路由按此寻址） |
| `--registry` | — | 模型登记表（YAML：模型名 → 权重路径、上下文、显存需求等） |

> 令牌**不从命令行传**（`--token` 旗标已停用：一旦带上即硬失败）——只走环境变量或 `~/.zerg/token` 600 档。

## 模型登记表（agent_models.yaml）

最小条目：

```yaml
example-35b-v2:
  backend: llama-server
  file: /data/models/example-35b-v2/example-35b-v2-Q4_K_M.gguf
  mem_gb: 21
  modality: multimodal
  mmproj: /data/models/example-35b-v2/mmproj-example-35b-v2-BF16.gguf   # 可选：视觉投影
  chat_template: ~/.zerg/example-35b-v2_chat_template.jinja             # 可选：覆盖内嵌模板
```

**`chat_template`（可选）**：某些模型（如 example-35b-v2 系）的 GGUF 内嵌模板过旧，需外部模板覆盖。
路径支持 `~` 前缀；相对路径按 agent 工作目录解析。
解析优先级：**`chat_template` 字段 → 环境变量 `ZERG_EXAMPLE_TEMPLATE` → `~/.zerg/example-35b-v2_chat_template.jinja` → 仓库内相对路径**；
链路上显式配置但文件不存在时记 WARN 并继续回退（不会把坏路径传给后端）。

## 环境变量

| 变量 | 用途 |
|---|---|
| `ZERG_AUTH_TOKEN` | 共享令牌（推荐入口；兼容旧名 `ZERG_API_TOKEN` / `ZERG_TOKEN`） |
| `ZERG_EXAMPLE_TEMPLATE` | 可选：example-35b-v2 系模型的 chat template 路径（默认找 `~/.zerg/example-35b-v2_chat_template.jinja`） |

## 预编译二进制

每个版本提供：**子端** `zerg-agentd-<os>-<arch>`（darwin/linux × amd64/arm64）。

> ★ 同一批制品里还有**命令面 CLI** `zerg-agent-<os>-<arch>`（组件名 `agent`，真源 `core/cmd/zerg-agent`）——
> **两件同名族、不同物**：`agentd` 是常驻子端服务（本文件），`agent` 是任务执行的 CLI。

## 依赖

- 模型后端：`llama-server`（llama.cpp）或 `ds4-server`，需在 `PATH` 或 registry 中指明路径
- 无其它服务依赖；子端与主控之间只有 HTTP + 共享令牌

## 许可

Apache-2.0（见发布仓 `LICENSE`）
