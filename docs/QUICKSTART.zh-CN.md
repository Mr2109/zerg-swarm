# 快速开始（QUICKSTART）

[English](QUICKSTART.en.md) | **中文**

> *中文原版（唯一真相源）。英文版为派生翻译；两版如有不一致，以**本文**为准。*

从零到一个能用的虫群，四步。全程只需要一条命令跑起来是**主控**；子端和 UI 可选但推荐。

> 约定：下文 `<repo>` 指你 clone 下来的目录。所有端口/路径都可用环境变量覆盖，见 [CONFIGURATION.zh-CN.md](CONFIGURATION.zh-CN.md)。

---

## 第 0 步：准备共享令牌

所有组件用同一个令牌（`X-Auth-Token` 头）。**绝不写进仓库**——放环境变量或 `~/.zerg/token`：

```bash
cd <repo>
cp .env.example .env
# 生成一个随机令牌（推荐 32 字节 hex）
TOKEN=$(openssl rand -hex 32)
sed -i '' "s|^ZERG_AUTH_TOKEN=.*|ZERG_AUTH_TOKEN=${TOKEN}|" .env   # macOS；Linux 用 sed -i
set -a; . ./.env; set +a
```

也可以完全不用 `.env`：把令牌写进 `~/.zerg/token`（单行、`chmod 600`）即可。

> ⚠️ **路径含空格或中文时必须在 `.env` 里加双引号**（例如 `ZERG_KB_PATH="/Volumes/My Disk/knowledge.db"`）——
> 否则 `set -a; . ./.env` 会在空格处截断变量，后面的部分还会被当成命令执行（报 `no such file or directory`）。
解析优先级：`ZERG_AUTH_TOKEN` → `~/.zerg/token`。

> 令牌缺失时主控与子端会**拒绝启动**并给出指引——这是故意的（空令牌会让所有 API 401，而日志看起来一切正常）。

## 第 1 步：写模型登记表

```bash
cp gateway/fleet.example.yaml gateway/fleet.yaml
$EDITOR gateway/fleet.yaml      # 填你自己的机器与权重路径
```

最小可用示例（单机）：

```yaml
auth:
  token: ""            # 留空：由环境变量/令牌文件注入
models:
  example-8b:
    - name: example-8b
      host: local
      backend: llama-server
      file: /path/to/models/your-model-Q4_K_M.gguf
      mem_gb: 6
      ctx_window: 32768
      modality: text
fleet:
  local: { host: 127.0.0.1, port: 8100, os: macos }
```

> **模型名可以随便起**——只要登记表里有、路径指对，虫族就能用。字段说明见 [MODELS.zh-CN.md](MODELS.zh-CN.md)。

## 第 2 步：启动主控

```bash
cd core
go build -o ../bin/zerg-core ./cmd/zerg-core
cd ..
set -a; . ./.env; set +a          # 让主控拿到令牌
./bin/zerg-core
```

主控监听 **`:8580`**（API）并同时起 **`:8082`**（OpenAI 兼容网关）。启动横幅会打印掩码后的令牌，形如 `🔐 认证令牌: xxxx******(len=64)`。

**验证**（这一条能同时证明"起来了"和"令牌对"）：

```bash
# 不带令牌 → 401（认证生效）
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8580/api/capabilities
# 带令牌 → 200
curl -s -H "X-Auth-Token: $ZERG_AUTH_TOKEN" http://127.0.0.1:8580/api/fleet/status | head -c 200
# OpenAI 兼容入口
curl -s -H "X-Auth-Token: $ZERG_AUTH_TOKEN" http://127.0.0.1:8082/v1/models | head -c 200
```

## 第 3 步（可选）：加一台子端 —— 集群就有了

子端负责在**工作机**上真正把模型拉起来。在那台机器上：

```bash
# 编译（可在别的机器交叉编译后拷过去）
cd <repo>/agent
go build -o zerg-agent ./cmd/zerg-agent

# 启动（把令牌给它）
ZERG_AUTH_TOKEN=<同一令牌> ./zerg-agent \
  --host 0.0.0.0 \
  --port 8100 \
  --machine worker1 \
  --controller http://<主控地址>:8580 \
  --registry agent_models.yaml
```

子端每 5 秒向主控心跳。在主控侧确认：

```bash
curl -s -H "X-Auth-Token: $ZERG_AUTH_TOKEN" http://127.0.0.1:8580/api/fleet/status
# 应该看到 worker1 的 online: true，以及它上面已加载的模型
```

> 然后在 `gateway/fleet.yaml` 的 `fleet:` 段里声明这台机器（`worker1: { host: <它的IP>, port: 8100, os: linux }`），
> 并在某个模型的候选列表里加上 `host: worker1` 的条目——调度器就会把该模型分配到它上面加载。

## 第 4 步（可选）：桌面 UI

```bash
cd ui
cargo build --release -p zerg-ui
cp target/release/zerg-ui ../bin/zerg-ui
../bin/zerg-ui
```

UI 从环境变量或 `~/.zerg/token` 取令牌；也可以写进 `~/.zerg-ui-prefs.json` 的 `auth_token` 字段。

- 端点可用 `ZERG_API_BASE` / `ZERG_AI_BASE` 覆盖（默认 `http://127.0.0.1:8580` / `:8082`）
- 中文字体自动探测（`ZERG_FONT_PATH` 可指定）

---

## 提交一个任务试试

```bash
curl -s -X POST -H "X-Auth-Token: $ZERG_AUTH_TOKEN" -H 'Content-Type: application/json' \
  -d '{"description":"列出当前目录的文件，写一份简短说明到 report.md"}' \
  http://127.0.0.1:8580/api/tasks
```

主控会：选模型 → 找一台有显存的机器（必要时加载模型）→ 派发子端执行 Agent 循环（工具调用 + 防呆反馈）→ 状态与日志在 UI 的"任务"面板可见。

## 改了配置 / 换了机器之后

`gateway/fleet.yaml`、端口或机器名改过之后，**别让旧进程带着旧配置继续跑**。两条都建议**先干跑**：

- **热加载配置**：建议先 `zerg config reload --dry-run` 看计划件（零副作用 · 一个字节不写），确认影响面无误再 `zerg config reload --yes`（需令牌 · 与 `POST /api/config/reload` 同效）。名册件本地解析不过 ⇒ **不发请求**，旧配置继续跑（照 `nginx -s reload`：先校验、失败回滚）。
- **换了机器之后重启主控**：机器名 / 端口 / 服务声明变了，热加载不够 —— `zerg core restart` 会**停 + 起主控**（**整个虫群的控制面会断一会儿**）。**建议**先 `zerg core restart --dry-run` 看计划件（将重启哪个 pid、影响哪些端口与服务），确认档齐了才真跑：建议**整串照抄** `zerg core restart --confirm=<主机名> --yes`。

两条都是**三态**：`--dry-run`（零副作用）· `--confirm=<目标>`（值必须与目标逐字相同）· `--yes`（确认档）；`core restart` 要 `--confirm` 与 `--yes` **同时到**，缺一个 ⇒ 只出计划件、**不执行**。

**就绪判据绑自己那份 pid**：重启前用 `zerg core ps` 记下 pid 与起时，重启后必须**换上新 pid**（监听端口的属主也跟着换）才算真起来了 —— pid 与起时都没变 ⇒ 还是旧进程，不许当绿。**失败回滚**：配置面不过 ⇒ 旧配置继续跑，改回文件再加载一次即可；进程面没起来 / 没就绪 ⇒ 进程面可再跑一次本命令，件的回滚归换件入口（`scripts/build/zerg-swap-core.sh`）。

## 常见坑

| 现象 | 原因 / 处理 |
|---|---|
| 所有 `/api/*` 返回 401 | 令牌没给到：`export ZERG_AUTH_TOKEN=...` 或写 `~/.zerg/token` |
| 主控启动即退出并提示"未配置共享令牌" | 同上（这是有意的防呆） |
| UI 显示"主控离线" | 端点不对（`ZERG_API_BASE`）或令牌不对 |
| 子端在线但模型加载失败 | 权重路径/显存不足；看子端日志与自己那台机器的 `llama-server` 输出 |
| `ls/git/go…` 命令报 `rtk: command not found` | 旧版遗留问题，现已自动降级；也可 `ZERG_RTK=0` 显式关闭 |
| `web_search` 不可用 | 需要自建 searxng，见 `scripts/install-searxng.sh`（AGPL 组件，不随仓库分发） |

更多见 [FAQ.zh-CN.md](FAQ.zh-CN.md)。
