# 配置（CONFIGURATION）

[English](CONFIGURATION.en.md) | **中文**

> *中文原版（唯一真相源）。英文版为派生翻译；两版如有不一致，以**本文**为准。*

虫族没有中心配置文件：**令牌 + 登记表 + 环境变量**就是全部配置。
所有硬编码都收敛到"合理默认值 + 环境变量覆盖"，因此同一份代码在你的机器上也能跑。

---

## 1. 令牌（必填）

解析优先级（所有组件一致）：

1. 环境变量 `ZERG_AUTH_TOKEN`（兼容旧名 `ZERG_API_TOKEN` / `ZERG_TOKEN`）
2. 文件 `~/.zerg/token`（单行；建议 `chmod 600`）
3. 仓库根 `.env`（**不入库**，`.gitignore` 已覆盖）+ `.env.example` 模板

```bash
cp .env.example .env
printf 'ZERG_AUTH_TOKEN=%s\n' "$(openssl rand -hex 32)" >> .env
set -a; . ./.env; set +a          # 让当前 shell 及子进程拿到
```

规则：`fleet.yaml` 里的 `auth.token` **保持空**；子端用 `--token` 或环境变量接收同一个值。
主控与子端在令牌缺失时**拒绝启动**（避免"所有 API 401 但日志正常"的隐形故障）。

---

## 2. 端口

| 用途 | 默认 | 环境变量（core 侧） | 子端参数 |
|---|---|---|---|
| 主控 API | `8580` | `ZERG_PORT` | — |
| 网关（OpenAI 兼容） | `8082` | `ZERG_GATEWAY_PORT` | — |
| 子端 HTTP | `8100` | `ZERG_AGENT_PORT` | `--port` |
| 子端监听地址 | `127.0.0.1` | — | `--host`（跨机部署用 `0.0.0.0`） |

UI 侧指向主控/网关：

| 变量 | 默认 | 说明 |
|---|---|---|
| `ZERG_API_BASE` | `http://127.0.0.1:8580` | 主控 API |
| `ZERG_AI_BASE` | `http://127.0.0.1:8082` | 网关 |

> 端口是"约定"而非编译期常量——想换端口只改环境变量，内部互调会自动跟随。

---

## 3. 路径与外部数据

| 变量 | 默认 | 用途 |
|---|---|---|
| `ZERG_WORKSPACE` | 自动推断（向上找 `.git`/`core/go.mod`，或 `bin/` 的父目录） | 工作区根 |
| `ZERG_TMP_DIR` | `/tmp` | 临时根（任务目录/日志/缓存的父目录） |
| `ZERG_TASK_ROOT` | `<tmp>/zerg-tasks` | CA 任务目录 |
| `ZERG_CA_LOG_DIR` | `<tmp>/zerg-ca-logs` | CA 事件日志（`events.jsonl`） |
| `ZERG_LOG_DIR` | `<tmp>` | 运行日志目录 |
| `ZERG_STATE_DIR` | `~/.zerg/state` | 工具计数、事件、错误桶 |
| `ZERG_CHAT_DB_PATH` | `~/.zerg-chat/chat.db` | 对话库 |
| `ZERG_MEMORY_DIR` | `~/.zerg/memory` | 记忆 |
| `ZERG_KB_PATH` | `<工作区>/data/knowledge.db` | 知识库 SQLite（**不存在则相关工具自动跳过**） |
| `ZERG_COMPRESS_MODELS` | `<工作区>/compress_models` | 可选压缩模型（LLMLingua-2 等） |
| `ZERG_EXTRA_ALLOW_DIR` | 空 | CA 任务的**额外白名单目录**（冒号分隔）——文件工具与 rm 闸门都会放行这些目录 |

> **`ZERG_EXTRA_ALLOW_DIR` 是安全边界**：只把它指到你愿意让 Agent 读写的目录。

---

## 4. 外部集成（可选）

| 变量 | 说明 |
|---|---|
| `ZERG_SEARXNG_PY` / `ZERG_SEARXNG_SETTINGS` / `ZERG_SEARXNG_SRC` | 自建 searxng 的解释器 / 设置文件 / 源码目录（默认找 `<工作区>/vendor/searxng/...`，再退到 `PATH` 里的 `python3`） |
| `ZERG_RTK` | `0` = 关闭 rtk 命令包装。**未安装 rtk 时自动降级**，无需设置 |
| `ZERG_ORNITH_TEMPLATE` | 某些模型需外部 chat template 覆盖内嵌旧模板时的文件路径（也支持登记表字段，见 [MODELS.md](MODELS.zh-CN.md)） |
| `ZERG_AUTOCUT_DIR` / `ZERG_MUSIC_DIR` / `ZERG_DRP_FILE` | 影音工具链的可选目录/工程文件 |
| `ZERG_DF_PATHS` | 磁盘巡检工具额外统计的挂载点（默认只看 `/`） |
| `ZERG_FONT_PATH` | UI 中文字体文件 |

---

## 5. 运行行为开关

| 变量 | 默认 | 说明 |
|---|---|---|
| `ZERG_INTERNAL_TASKS` | **关** | 内部任务引擎（自我巡检/清理类任务）。**保持关闭**：清理类任务会删文件 |
| `ZERG_LOG_LEVEL` / `ZERG_LOG_STDOUT` | `info` / — | 日志级别与是否输出到 stdout |
| `ZERG_DEBUG` | — | 调试输出 |
| `ZERG_CHAT_CLEANUP_DRY_RUN` | — | 对话清理"只报数不改数据"预览 |
| `ZERG_HERMES_TOOLS` | 自动 | 用 XML 工具调用协议（对某些模型更稳）。任务调度会自动设置 |
| `ZERG_LEGACY_LOOP` | — | 回退到旧版工具循环（排障用） |

> 其余以 `ZERG_` 开头的变量属内部/开发用途（评测、回放、子任务实验等），不建议在生产部署里改动；
> 完整清单可直接在代码里检索：`rg -o 'ZERG_[A-Z_]+' core agent ui | sort -u`。

---

## 6. 模型登记表 `gateway/fleet.yaml`

```yaml
auth:
  token: ""                 # 保持空：令牌走环境变量/令牌文件

models:
  <模型名>:                  # 名字随意起
    - name: <模型名>
      host: local|worker1   # fleet 段里声明的机器标识
      backend: llama-server # 或 ds4-server，或配合 cmd: 自定义
      file: /abs/path/model.gguf
      mem_gb: 22            # 加载该模型预计占用（调度用）
      ctx_window: 262144    # 上下文窗口（可选）
      modality: text        # 或 multimodal
      mmproj: /abs/path/mmproj.gguf   # 多模态投影（可选）
      cmd: "llama-server -m {file} --port {port}"  # 完全自定义启动命令（可选）

aliases:                    # 客户端别名 → 标准模型名（可选）
  my-alias: <模型名>

fleet:                      # 机器清单
  local:   { host: 127.0.0.1, port: 8100, os: macos }
  worker1: { host: 192.0.2.10, port: 8100, os: ubuntu }
```

改动后无需重启主控：`POST /api/config/reload`（需令牌）。

---

## 7. 子端登记表 `agent_models.yaml`

子端**自己**也需要一份"这台机器上有什么模型"的登记表（与主控的 fleet 不必完全相同——主控关心"候选"，子端关心"怎么拉起"）：

```yaml
<模型名>:
  backend: llama-server
  file: /data/models/x.gguf
  mem_gb: 22
  modality: text
  mmproj: /data/models/mmproj.gguf                    # 可选
  chat_template: ~/.zerg/example-35b-v2_chat_template.jinja   # 可选（支持 ~）
```

`chat_template` 的解析优先级：**登记表字段 → `ZERG_ORNITH_TEMPLATE` → `~/.zerg/example-35b-v2_chat_template.jinja` → 仓库内相对路径**；
任一处显式配置但文件不存在 → 记 WARN 并继续回退（不会把坏路径传给后端）。

---

## 8. UI 偏好

`~/.zerg-ui-prefs.json`（自动生成）：

| 字段 | 说明 |
|---|---|
| `auth_token` | 令牌（不设则走环境变量/`~/.zerg/token`） |
| `preview_renderer` | Markdown 预览渲染器选择（可用 `ZERG_PREVIEW_RENDERER` 覆盖启动默认） |

运行目录：`<tmp>/zerg-ui/`（模块清单等），可用 `ZERG_UI_DIR` 覆盖。

| `ZERG_AI_MODEL` | — | UI 文档 AI 动作（总结/续写/翻译/润色）使用的模型 id；也可写进 `~/.zerg-ui-prefs.json` 的 `ai_model` |
