# Zerg Swarm

**A self-hosted local LLM cluster** — turn a few ordinary machines (Apple Silicon Macs, NVIDIA boxes) into one "swarm": load models on demand, pick the machine by VRAM and load, expose a single OpenAI-compatible endpoint, and get autonomous task execution plus a chat workbench out of the box.

> **English** | [中文](README.zh-CN.md)
>
> *Source of truth: the Chinese original ([README.zh-CN.md](README.zh-CN.md)). If the two disagree, the Chinese text prevails.*

> Design principle: **there are no weak models, only incomplete systems.**
> When a model fails a task, drifts off format, or breaks a tool call, we blame the **system** first
> (prompt design, parsing layer, guardrails, feedback protocol) — never the model.
> This principle shapes much of the project (see the [Format Feedback Protocol](docs/design/格式反馈协议-FFP.md), in Chinese).

---

## What it solves

- **Many local models, not enough VRAM**: one machine cannot hold them all → Zerg **loads models on demand** onto whichever machine has room, and unloads them when idle.
- **Too many engines to unify**: llama.cpp / ds4-server / vLLM / any OpenAI-compatible engine → Zerg exposes them behind **one endpoint**.
- **Local models "not obeying"**: malformed tool calls, empty arguments, retry loops → handled at the system layer with format feedback and guardrails, instead of swapping in a bigger model.
- **You want your own AI workbench**: chat, tasks, tools, memory, logs, cluster status → one desktop UI plus an HTTP API.

## Architecture (four parts)

```
                    ┌───────────────────────────────┐
                    │  zerg-ui (Rust/egui desktop)  │
                    │  chat · tasks · cluster · tools · logs │
                    └───────────────┬───────────────┘
                                    │ HTTP + X-Auth-Token
                    ┌───────────────▼───────────────┐
                    │  controller zerg-core (Go, :8580) │
                    │  scheduling / chat / tools / memory / gateway │
                    └───────┬───────────────┬───────┘
      OpenAI-compatible     │               │  task dispatch + heartbeat
      entry (:8082 gateway) │               │
                    ┌───────▼──────┐  ┌─────▼─────────────────────┐
                    │ clients /    │  │ agent zerg-agent (Go, :8100) │
                    │ external tools │  │ load on demand · self-heal · circuit breaker │
                    └──────────────┘  └─────┬─────────────────────┘
                                            │ spawn / unload
                                  ┌─────────▼─────────┐
                                  │ llama-server /    │
                                  │ ds4-server / etc. │
                                  └───────────────────┘
```

| Component | Directory | Language | Role |
|---|---|---|---|
| Controller | `core/` | Go | Scheduling (pick machine by VRAM/load), chat and tool loop, memory, gateway (OpenAI-compatible `:8082`) |
| Agent | `agent/` | Go | Runs on every worker machine: load/unload models on demand, self-heal, 5-second heartbeat |
| Desktop UI | `ui/` | Rust + egui | Chat, tasks, cluster status, tool catalog, docs, logs |
| Menu bar app | `app/` | Swift | macOS read-only status panel (**optional**, not part of the core logic) |

## Features

- **Cluster scheduling**: the model registry declares the candidate machines for the same model; the runtime scores them by VRAM and load.
- **Load on demand**: models unload when unused; each machine is **single-slot** (one model at a time, with a machine-wide sweep on conflict).
- **Multiple engines**: any OpenAI-compatible backend can be plugged in; nothing is bound to a single engine.
- **Autonomous tasks**: hand it a natural-language task → agent loop (tool calls + feedback + retries + circuit breaker) → a report.
- **Tool system**: file read/write/edit, shell (with a deletion-scope gate), search, screenshot OCR, media processing, knowledge base, subtask decomposition … 135 registered tools.
- **Memory**: two scopes (global + per agent) with budgets, compression, and recall pointers (see [记忆体系](docs/design/记忆体系.md), in Chinese).
- **Desktop workbench**: streaming chat, Markdown rendering, task panel, cluster/model status, logs, document browsing.

## Quick start

```bash
git clone <this-repo> && cd zerg-swarm

# 1. Token (shared by all components; never commit it)
cp .env.example .env
printf '%s\n' "$(openssl rand -hex 32)" > .env.tmp && sed -i '' "s/^ZERG_AUTH_TOKEN=.*/ZERG_AUTH_TOKEN=$(cat .env.tmp)/" .env && rm .env.tmp
set -a; . ./.env; set +a

# 2. Model registry
cp gateway/fleet.example.yaml gateway/fleet.yaml   # edit for your machines/weights

# 3. Controller
cd core && go build -o ../bin/zerg-core ./cmd/zerg-core && cd ..
./bin/zerg-core
```

Details (adding an agent, wiring models, installing the desktop UI) are in [docs/QUICKSTART.en.md](docs/QUICKSTART.en.md).
The full configuration reference is [docs/CONFIGURATION.en.md](docs/CONFIGURATION.en.md).

## Requirements

| Component | Requirement |
|---|---|
| Controller / agent | Go 1.22+ (to build). No runtime dependencies (SQLite via a pure-Go driver) |
| Desktop UI | Rust stable (to build); runs on macOS 14+ / Linux |
| Model backend | [llama.cpp](https://github.com/ggml-org/llama.cpp) `llama-server` (recommended) or any OpenAI-compatible engine |
| Web search (optional) | Your own [searxng](https://github.com/searxng/searxng) (AGPL, **not distributed with this repo**) — see `scripts/install-searxng.sh` |
| Command-output compression (optional) | [rtk](https://github.com/rtk-ai/rtk) (Apache-2.0) — **degrades gracefully if missing**, or disable with `ZERG_RTK=0` |

## Documentation

| Document | Contents |
|---|---|
| [docs/QUICKSTART.en.md](docs/QUICKSTART.en.md) | From zero to running: single machine → add an agent → wire models → desktop UI |
| [docs/ARCHITECTURE.en.md](docs/ARCHITECTURE.en.md) | Component responsibilities, data flow, single-slot mechanism, task execution chain |
| [docs/CONFIGURATION.en.md](docs/CONFIGURATION.en.md) | Ports, environment variables, state directories, registry fields |
| [docs/MODELS.en.md](docs/MODELS.en.md) | Model onboarding, adapter mechanism, context, chat templates |
| [docs/TOOLS.en.md](docs/TOOLS.en.md) | Tool catalog (grouped by purpose) |
| [docs/FAQ.en.md](docs/FAQ.en.md) | Common questions and troubleshooting |
| [docs/design/](docs/design/) | Design notes: Format Feedback Protocol (FFP), memory system, tool-upgrade spec (Chinese) |

Every document also exists in Chinese (`docs/*.zh-CN.md`), and the Chinese version is the authoritative one.

## License and contributing

- **Apache-2.0** (see [LICENSE](LICENSE)); third-party components are listed in [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md).
- Contributions use **DCO**: `git commit -s`, see [CONTRIBUTING.md](CONTRIBUTING.md).
- Security policy and "notes for deployers" are in [SECURITY.md](SECURITY.md).

> Copyright line: `Copyright 2026 The Zerg Swarm Authors`. Apache-2.0 grants **no trademark rights** — the names "虫族 / Zerg Swarm" are not covered by the license.
