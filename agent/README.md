# zerg-agent — 虫族子端 Agent（CA）

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
go build -o zerg-agent ./cmd/zerg-agent
# 交叉编译示例（Linux 工作机）
GOOS=linux GOARCH=amd64 go build -o zerg-agent-linux-amd64 ./cmd/zerg-agent
```

## 运行

```bash
./zerg-agent \
  --host 0.0.0.0 \
  --port 8100 \
  --machine <本机标识> \
  --controller http://<主控地址>:8580 \
  --token <共享令牌> \
  --registry agent_models.yaml
```

| 参数 | 默认 | 说明 |
|---|---|---|
| `--host` | `127.0.0.1` | 监听地址；跨机部署用 `0.0.0.0` |
| `--port` | `8100` | 监听端口 |
| `--token` | 环境变量 `ZERG_TOKEN` | 共享认证令牌 |
| `--controller` | `http://127.0.0.1:8580` | 主控地址（心跳上报目标） |
| `--machine` | 主机名 | 机器标识（主控路由按此寻址） |
| `--registry` | — | 模型登记表（YAML：模型名 → 权重路径、上下文、显存需求等） |

## 环境变量

| 变量 | 用途 |
|---|---|
| `ZERG_TOKEN` | 共享令牌（等同 `--token`） |
| `ZERG_ORNITH_TEMPLATE` | 可选：ornith 系模型的 chat template 路径（默认找 `~/.zerg/ornith_chat_template.jinja`） |

## 预编译二进制

每个版本在 GitHub Releases 提供：`zerg-agent-<os>-<arch>`（darwin/linux × amd64/arm64）。

## 依赖

- 模型后端：`llama-server`（llama.cpp）或 `ds4-server`，需在 `PATH` 或 registry 中指明路径
- 无其它服务依赖；子端与主控之间只有 HTTP + 共享令牌

## 许可

Apache-2.0（见发布仓 `LICENSE`）
