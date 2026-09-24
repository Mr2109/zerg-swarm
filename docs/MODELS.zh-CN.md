# 模型接入（MODELS）

[English](MODELS.en.md) | **中文**

> *中文原版（唯一真相源）。英文版（[MODELS.en.md](MODELS.en.md)）为派生翻译；两版如有不一致，以**本文**为准。*

虫族**不绑定任何推理引擎**，也不要求特定模型 —— 任何能通过 OpenAI 兼容接口或命令行拉起的模型都能接入。

---

## 1. 三层接入方式

| 方式 | 做法 | 适用 |
|---|---|---|
| **A. 登记表 + llama.cpp** | 在子端 `agent_models.yaml` 填 `file:`/`mem_gb:`，虫族自动拼 `llama-server` 参数 | 最常见：GGUF + llama.cpp |
| **B. 自定义启动命令** | 登记表 `cmd:` 写完整命令行（支持 `{file}`/`{port}`/`{dir}` 占位符） | 特殊参数、非标准二进制、包装脚本 |
| **C. 已在外运行的实例** | 登记表指向已在跑的端点（或用 `ZERG_GATEWAY_URL` 让外部引擎接进网关） | vLLM / ds4-server / Ollama 等 |

```yaml
# A
example-35b:
  backend: llama-server
  file: /data/models/x-Q4_K_M.gguf
  mem_gb: 22
  ctx_window: 262144
  modality: multimodal
  mmproj: /data/models/mmproj.gguf

# B（完全自己控制命令行）
example-custom:
  backend: llama-server
  file: /data/models/y.gguf
  mem_gb: 18
  cmd: "run-k2.sh -m {file} --port {port} -c 32768"
```

### 把模型登记进名册（命令行）

名册在命令行上也可读可写 —— 主控侧的那份名册（`gateway/fleet.yaml` 的 `models:` 段）就是模型面的真源：

```bash
zerg model ls        # 现有模型：id / host / backend / 模态 / 显存
zerg model show <模型名>   # 单枚模型的全貌
# 登记一条（先干跑：出计划件 · 零副作用）
zerg model add --host <机器名> --model <模型名> --file <GGUF 路径> --backend llama-server --mem-gb 18 --ctx 131072 --dry-run
```

`model add` 是写面：先校验后写、写完读回再校，任一步不过就回滚，且不覆盖已有的条；干跑换成真写要把 `--dry-run` 换成 `--yes`。

---

## 2. 适配器机制（模型"脾气"的处理层）

不同模型家族的启动参数、工具调用风格、思考格式都不一样。虫族用**适配器**（adapter）按**模型名前缀**自动派发：

| 适配内容 | 说明 |
|---|---|
| 启动参数 | 上下文、KV cache 量化、并行槽位、jinja 模板、reasoning 相关开关 |
| 工具调用风格 | 标准 `tool_calls`（JSON）还是 XML（`<tool_call>`） |
| 思考格式 | `thinking` 块如何解析/展示（deepseek 风格 / 纯文本 / 无） |
| 重提示策略 | 某些模型"该调工具却不调"，需在系统提示里加一句推动 |
| 上下文窗口 | 建议 `max_tokens` 与压缩阈值参考 |

**模型名匹配是前缀匹配、最长优先**：例如 `example-35b` 与 `example-35b-v2` 各自命中自己的适配器；没有匹配则落到通用（generic）适配器，仍然可用。

> 想接一个全新家族？在 `core/internal/chat`（客户端侧）与 `agent/internal/modeladapter`（子端侧）各加一个适配器即可，
> 不需要改动调度、UI 或网关。

---

## 3. 上下文与显存

- **上下文**：`ctx_window` 是"该模型能接受多少 token"，不是"一定分配那么多"；实际以启动参数为准
- **显存估算**：`mem_gb` 用于调度打分（能不能放下、放下后还剩多少）。填得准，调度才准
- **KV cache**：长上下文 + 大模型会吃额外显存；虫族默认对 KV cache 做量化（可关）
- **单槽**：一台机器同一时刻只托管一个虫族模型。若要同时用两个模型，请把它们放在不同机器（这正是集群的用途）

---

## 4. chat template（可选，但有些模型必需）

有些模型的 GGUF **内嵌模板过旧**（例如工具调用/思考开关解析不对），需要外部模板覆盖。

优先级（从高到低）：

1. 子端登记表字段 `chat_template: <path>`（支持 `~`）—— **推荐**
2. 环境变量 `ZERG_ORNITH_TEMPLATE`
3. `~/.zerg/example-35b-v2_chat_template.jinja`
4. 仓库内相对路径 `agent/example-35b-v2_chat_template.jinja`

任一处显式配置但文件不存在 → 记 WARN 并继续回退（不会把坏路径传给后端而导致启动失败）。

---

## 5. 模型选型建议（工程视角）

| 场景 | 建议 |
|---|---|
| 工具调用要稳 | 选**工具调用训练充分**的模型；不稳的模型靠 XML 协议 + 格式反馈兜底，而不是换更大模型 |
| 长文档/长代码 | 关注真实可用上下文（不是标称值）与 KV cache 显存开销 |
| 多模态（看图） | 需要 `mmproj` 投影文件；截图工具会走 OCR，必要时再走视觉模型 |
| 显存吃紧 | 量化版本（Q4_K_M 一类）优先；或用"按需加载"把不常用模型卸载 |

> 虫族的设计前提是：**模型的短板由系统补**。同样的模型，在提示设计、协议适配、防呆反馈到位的系统里，
> 表现会明显好于"裸调 API"。这也是本项目最核心的主张（见 [design/格式反馈协议-FFP.md](design/格式反馈协议-FFP.md)）。

---

## 引擎状态读哪些字段

`zerg agent show <子端> --json` 现在把**引擎侧**的状态也带出来（此前只有在线/心跳一类字段）：

| 字段 | 含义 |
|---|---|
| `model` | 当前装载的模型 id（未加载时为空） |
| `backend_state` | 引擎进程状态（如 `ready` / 未加载） |
| `active_requests` | 正在承接的请求数 |
| `gpu_used_gb` / `gpu_temp_c` | 显存占用与温度（统一内存机器上可读） |
| `backend_rss_gb` | 引擎进程实际驻留内存 |

**槽位与上下文**：两机实测均为**每机 4 槽 × 每槽 `n_ctx = 262144`**，各槽共享统一 KV（`kv_unified`）。引擎的启动参数真源在**卵清单的 `cmd:`**——写在那里的整段命令行会**整体覆盖**适配器默认参数（这一点常被误解，特此写明）。

**冷加载量级**：卸载后再加载一个 35B 级模型，工作机实测约 **2.3 秒**（页面缓存热态）；首次从磁盘读会明显更久。
