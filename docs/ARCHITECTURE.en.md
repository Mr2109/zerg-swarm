# Architecture (ARCHITECTURE)

> **English** | [中文](ARCHITECTURE.zh-CN.md)
>
> *Source of truth: the Chinese original ([ARCHITECTURE.zh-CN.md](ARCHITECTURE.zh-CN.md)). If the two disagree, the Chinese text prevails.*
> *Derived translation — 派生自 zh-CN（真源在 zh 侧）。*

Four processes plus a scheduling layer driven by a single "registry" file. Every component communicates over HTTP only; tokens are unified; there is no centralized database.

---

## 1. Components and Responsibilities

### Controller `zerg-core` (Go, `:8580`)

| Subsystem | Directory | Responsibility |
|---|---|---|
| Cluster/scheduling | `core/internal/api`, `core/internal/inference` | Model candidate scoring (VRAM/load/online), task queueing and dispatch, heartbeat aggregation |
| Gateway | `core/internal/gateway` | OpenAI-compatible entry point (`:8082`), auth passthrough, routing to the local machine or an agent, SSE usage sampling, prefix-cache hit-rate stats |
| Chat | `core/internal/chat` | Session storage (SQLite + trigram FTS), streaming inference, tool loop, compaction and retrieval |
| Tools | `core/internal/agent` | Tool implementations (files, shell, search, screenshot, media, knowledge base …) + execution gate + Format Feedback Protocol |
| Memory | `core/internal/memory` | Two-level scoped entries, budgets, threat scanning, prompt injection |
| Task Agent | `core/internal/agent` + `cmd/zerg-agent` | CA (Zerg Agent) task loop: subtask decomposition, tool execution, feedback, circuit breaker, report |

### Agent `zerg-agent` (Go, `:8100`)

Deployed on every worker machine; its **sole responsibility is to get models running and keep them healthy**:

- HTTP endpoints: `/status` `/load` `/infer` `/unload`
- Load on demand: call the backend per the registry (`llama-server` / `ds4-server` / custom `cmd`)
- Health and self-healing: readiness probing, restart on crash, circuit breaker; heartbeat to the controller every 5 seconds
- **Single-slot constraint**: a machine runs only one Zerg-managed model at a time (on conflict the whole machine is swept clean, so VRAM never fights itself)

### Desktop UI `zerg-ui` (Rust + egui)

Pure client: chat / tasks / cluster / tool catalog / docs / logs. All requests go over HTTP with a token;
internally it uses one HTTP client that does not go through the system proxy (loopback requests must bypass the proxy, otherwise the local proxy will swallow them).

### Menu bar `app/` (Swift, optional)

A macOS read-only status panel showing cluster online status and model occupancy. It takes no part in the core logic; removing it breaks nothing.

---

## 2. Data Flow

### Inference requests (external clients)

```
client → :8082 gateway (validate token)
       → routing decision: local llama-server? or one of the agents?
       → target backend /infer (SSE streaming)
       → gateway samples usage/hit rate, returns to client
```

### Tasks (CA)

```
POST /api/tasks
   → controller writes the task queue
   → scheduling: pick model → pick machine (candidate scoring; make an agent load the model if necessary)
   → spawn `zerg-agent -json -model <m> -workdir <task dir>`
   → agent loop: model call ↔ tool execution (with feedback every round)
   → artifacts written to the task directory (report/files), status and events persisted
   → UI polls status and logs
```

Task execution **does not happen inside the controller process**: the controller only schedules and observes; execution runs in a separate child process, so a crash or a timeout cannot take the controller down.

---

## 3. Scheduling and "single-slot"

- **Registry** `gateway/fleet.yaml` declares: model → candidate list (one entry per machine, with `file` / `mem_gb` / `ctx_window`)
- **Scoring**: ranked by available VRAM, current load, whether already loaded (warm start wins), and online status
- **Single slot**: both `localback` and the agents are responsible for "one managed model per machine" — a compromise forced by VRAM reality, not a design flaw
- **Unload**: call the agent's `/unload` when idle or when room must be made

---

## 4. Tool System and Safety

Tools come in two layers:

1. **Resident tools** (definitions injected straight into the prompt, about 14): high-frequency capabilities such as file read/write/edit, shell, search
2. **Deferred tools** (about 121, discovered on demand through `tool_search` and then called): media, knowledge base, documents, cluster operations, etc.

Safety design (`core/internal/agent`):

- **Deletion-scope gate**: shell `rm` may only act on { task workspace, whitelisted directories }; the home directory, the root directory, and any ancestor of them are always refused
- **File-tool path domain**: read/write/edit may only act on { task directory, `ZERG_EXTRA_ALLOW_DIR` }; anything outside is refused
- **A refusal must be "actionable"**: when something is refused, what comes back is guidance on the *legitimate place to go* (which directory you may write to, how to rewrite the command) instead of a bare error — this is "there are no weak models, only incomplete systems" landing at the tool layer
- **Format Feedback Protocol (FFP)**: when tool arguments are malformed (empty arguments, broken JSON) it returns a "system assertion + correct example" instead of making the model resend the same bad request; see [design/格式反馈协议-FFP.md](design/格式反馈协议-FFP.md) (in Chinese)

---

## 5. Memory and Chat

- **Chat**: a single SQLite database (default `~/.zerg-chat/chat.db`), message table + trigram FTS (searchable in Chinese)
- **Compaction**: a single entry point `MaybeCompact`, with cooldown (60s/300s/900s) + full jitter backoff, and a circuit breaker after three consecutive failures; after compaction it keeps **pointers back to the original text** (`GET /api/chat/sessions/{id}/window`), so the UI can jump straight back to the compacted original
- **Memory**: two scopes (global + per agent), entry-based with a budget cap and a write-rejection matrix; details in [design/记忆体系.md](design/记忆体系.md) (in Chinese)
- **Session search**: `session_search` supports `include_archived=true` to search archived sessions (archived sessions are excluded by default)

---

## 6. State and On-Disk Locations

| Content | Default location | Override variable |
|---|---|---|
| Tool counters/events/error buckets | `~/.zerg/state/` | `ZERG_STATE_DIR` |
| Chat database | `~/.zerg-chat/chat.db` | `ZERG_CHAT_DB_PATH` |
| Memory | `~/.zerg/memory/` | `ZERG_MEMORY_DIR` |
| Shared token | `~/.zerg/token` | `ZERG_AUTH_TOKEN` |
| Task directory | `/tmp/zerg-tasks/<id>/` | `ZERG_TASK_ROOT` |
| CA event log | `/tmp/zerg-ca-logs/<ts>/events.jsonl` | `ZERG_CA_LOG_DIR` |
| Runtime logs | `/tmp/zerg-*.log` | `ZERG_LOG_DIR` |
| UI preferences | `~/.zerg-ui-prefs.json` | — |

> The ports, paths, and external data locations (knowledge base, searxng, media directories) of the HTTP services **can all** be overridden through environment variables; see [CONFIGURATION.en.md](CONFIGURATION.en.md).

---

## 7. Extension Points

| What you want to do | Where to change it |
|---|---|
| Support a new model family (special launch arguments / thinking format / tool style) | the `core/internal/chat/...` adapter (dispatched by model-name prefix) + `agent/internal/modeladapter` |
| Support a new inference engine | the agent registry `backend:` + a custom `cmd:`; or implement a new adapter |
| Add a tool | `core/internal/agent` (implementation + registry + blurb), and keep the `tools/<name>.md` record in sync |
| Add a "container" app (a standalone panel in the UI) | `ui/src/modules/` (compiled in as an optional feature, as `zerg-roundtable` does) |
| Replace the UI | any frontend works, as long as it honors the HTTP API + token |

---

## 8. Design Trade-offs (out in the open)

- **Single-slot rather than multi-slot**: VRAM is a hard constraint; better to queue than to let two models OOM each other
- **HTTP + token rather than a message queue**: simple to deploy and debuggable with curl; the price is that you must secure the token yourself
- **Purely local**: no data is sent back to the author. The price is that every deployer must manage their own token and port exposure (see [SECURITY.md](../SECURITY.md))
- **No model fine-tuning**: Zerg takes the "**orchestration + system engineering**" route — get existing models to do the job well rather than train our own

---

## 9. Supervision and Mounting

- **The desktop UI is hosted as a system service**: `zerg-ui` runs under the launchd agent `com.zerg.ui` (service file `~/Library/LaunchAgents/com.zerg.ui.plist`, `KeepAlive` + `RunAtLoad`) — the system restarts it if it dies, so it is never launched bare by hand; read the current state with `launchctl print gui/$(id -u)/com.zerg.ui` (`state = running`)
- **The archive area is a read-only mounted image**: archive content is mounted as a **read-only volume** (the mount point is outside the repository), and the **original path beside the repository is a same-named symbolic link** pointing at it — so reading the original path reads files inside the read-only image, and writing there is always refused (this is not a permission bit set wrong; `chmod` cannot change a read-only volume either). There is a single entry point for attach/detach: `bash scripts/svc/archive-mount.sh on|off|status` (**on demand · never resident · never installed as a launchd job**); while unmounted the original path is a **dangling symbolic link**, reading it fails outright, and `status` reports "not mounted" and exits 1
