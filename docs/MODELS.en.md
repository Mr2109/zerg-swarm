# Model Integration (MODELS)

> **English** | [中文](MODELS.zh-CN.md)
>
> *Source of truth: the Chinese original ([MODELS.zh-CN.md](MODELS.zh-CN.md)). If the two disagree, the Chinese text prevails.*

Zerg **does not bind to any inference engine**, nor does it require a specific model — any model that can be brought up through an OpenAI-compatible interface or a command line can be integrated.

---

## 1. Three ways to integrate

| Way | How | Applies to |
|---|---|---|
| **A. Registry + llama.cpp** | Fill in `file:`/`mem_gb:` in the agent's `agent_models.yaml`; Zerg assembles the `llama-server` arguments automatically | Most common: GGUF + llama.cpp |
| **B. Custom launch command** | Write the full command line in the registry's `cmd:` (supports `{file}`/`{port}`/`{dir}` placeholders) | Special arguments, non-standard binaries, wrapper scripts |
| **C. Instances already running elsewhere** | Point the registry at an endpoint that is already running (or use `ZERG_GATEWAY_URL` to let an external engine join the gateway) | vLLM / ds4-server / Ollama, etc. |

```yaml
# A
example-35b:
  backend: llama-server
  file: /data/models/x-Q4_K_M.gguf
  mem_gb: 22
  ctx_window: 262144
  modality: multimodal
  mmproj: /data/models/mmproj.gguf

# B (you control the command line entirely)
example-custom:
  backend: llama-server
  file: /data/models/y.gguf
  mem_gb: 18
  cmd: "run-k2.sh -m {file} --port {port} -c 32768"
```

---

## 2. Adapter mechanism (the layer that handles a model's "temperament")

Different model families differ in launch arguments, tool-calling style, and thinking format. Zerg uses **adapters** (adapter), dispatched automatically by **model-name prefix**:

| What is adapted | Description |
|---|---|
| Launch arguments | Context, KV cache quantization, parallel slots, jinja template, reasoning-related switches |
| Tool-calling style | Standard `tool_calls` (JSON) or XML (`<tool_call>`) |
| Thinking format | How `thinking` blocks are parsed/displayed (deepseek style / plain text / none) |
| Re-prompt strategy | Some models "should call a tool but don't"; a nudge sentence has to be added to the system prompt |
| Context window | Reference for the suggested `max_tokens` and the compression threshold |

**Model-name matching is prefix matching, longest first**: for example, `example-35b` and `example-35b-v2` each hit their own adapter; if nothing matches, it falls back to the generic adapter and still works.

> Want to add a whole new family? Just add one adapter in `core/internal/chat` (controller side) and one in `agent/internal/modeladapter` (agent side);
> no changes to scheduling, the UI, or the gateway are needed.

---

## 3. Context and VRAM

- **Context**: `ctx_window` is "how many tokens this model can accept", not "definitely allocate that many"; what actually counts are the launch arguments
- **VRAM estimate**: `mem_gb` is used for scheduling scores (whether it fits, and how much is left after it fits). Accurate values make scheduling accurate
- **KV cache**: long context + a large model consumes extra VRAM; Zerg quantizes the KV cache by default (can be turned off)
- **Single slot**: a machine hosts only one Zerg model at a time. To use two models at once, put them on different machines (which is exactly what the cluster is for)

---

## 4. chat template (optional, but required for some models)

Some models' GGUF files **embed an outdated template** (for example, tool-call/thinking switches are parsed incorrectly), so an external template must override it.

Priority (highest to lowest):

1. Agent registry field `chat_template: <path>` (supports `~`) — **recommended**
2. Environment variable `ZERG_ORNITH_TEMPLATE`
3. `~/.zerg/example-35b-v2_chat_template.jinja`
4. Repo-relative path `agent/example-35b-v2_chat_template.jinja`

If any of these is configured explicitly but the file does not exist → log a WARN and keep falling back (a bad path will never be passed to the backend and cause a startup failure).

---

## 5. Model selection advice (from an engineering perspective)

| Scenario | Advice |
|---|---|
| Tool calling must be reliable | Pick a model that is **well trained for tool calling**; for unstable models, fall back on the XML protocol plus format feedback rather than switching to a bigger model |
| Long documents / long code | Watch the real usable context (not the nominal value) and the KV cache VRAM cost |
| Multimodal (image understanding) | Requires an `mmproj` projection file; the screenshot tool goes through OCR first and only falls back to a vision model when necessary |
| Tight on VRAM | Prefer quantized versions (Q4_K_M and the like); or use "load on demand" to unload models that are rarely used |

> Zerg's design premise is: **a model's weak spots are made up for by the system**. The same model performs noticeably better in a system with solid prompt design, protocol adaptation, and guardrail feedback than it does when you "call the API raw".
> This is also the project's most central claim (see [design/格式反馈协议-FFP.md](design/格式反馈协议-FFP.md)).
