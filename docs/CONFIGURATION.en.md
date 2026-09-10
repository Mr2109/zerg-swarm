# Configuration

> **English** | [中文](CONFIGURATION.zh-CN.md)
>
> *Source of truth: the Chinese original ([CONFIGURATION.zh-CN.md](CONFIGURATION.zh-CN.md)). If the two disagree, the Chinese text prevails.*

Zerg has no central configuration file: **token + registries + environment variables** are the whole configuration.
Everything hardcoded collapses into "sensible default + environment-variable override", so the same code also runs on your machine.

---

## 1. Token (required)

Resolution priority (identical across all components):

1. Environment variable `ZERG_AUTH_TOKEN` (legacy names `ZERG_API_TOKEN` / `ZERG_TOKEN` also work)
2. File `~/.zerg/token` (single line; `chmod 600` recommended)
3. Repository root `.env` (**not committed**, already covered by `.gitignore`) + the `.env.example` template

```bash
cp .env.example .env
printf 'ZERG_AUTH_TOKEN=%s\n' "$(openssl rand -hex 32)" >> .env
set -a; . ./.env; set +a          # make the current shell and its children see it
```

Rule: keep `auth.token` in `fleet.yaml` **empty**; agents receive the same value through `--token` or an environment variable.
Core and agents **refuse to start** when the token is missing (this avoids the invisible failure mode of "every API call returns 401 while the logs look fine").

---

## 2. Ports

| Purpose | Default | Environment variable (core side) | Agent argument |
|---|---|---|---|
| Core API | `8580` | `ZERG_PORT` | — |
| Gateway (OpenAI-compatible) | `8082` | `ZERG_GATEWAY_PORT` | — |
| Agent HTTP | `8100` | `ZERG_AGENT_PORT` | `--port` |
| Agent listen address | `127.0.0.1` | — | `--host` (use `0.0.0.0` for cross-machine deployment) |

The UI points at core / gateway:

| Variable | Default | Description |
|---|---|---|
| `ZERG_API_BASE` | `http://127.0.0.1:8580` | Core API |
| `ZERG_AI_BASE` | `http://127.0.0.1:8082` | Gateway |

> Ports are a "convention" rather than compile-time constants — to move a port, just change the environment variable; internal cross-calls follow automatically.

---

## 3. Paths and external data

| Variable | Default | Purpose |
|---|---|---|
| `ZERG_WORKSPACE` | Inferred automatically (searches upward for `.git`/`core/go.mod`, or the parent directory of `bin/`) | Workspace root |
| `ZERG_TMP_DIR` | `/tmp` | Temp root (parent of task directories / logs / caches) |
| `ZERG_TASK_ROOT` | `<tmp>/zerg-tasks` | CA task directory |
| `ZERG_CA_LOG_DIR` | `<tmp>/zerg-ca-logs` | CA event log (`events.jsonl`) |
| `ZERG_LOG_DIR` | `<tmp>` | Runtime log directory |
| `ZERG_STATE_DIR` | `~/.zerg/state` | Tool counters, events, error buckets |
| `ZERG_CHAT_DB_PATH` | `~/.zerg-chat/chat.db` | Conversation database |
| `ZERG_MEMORY_DIR` | `~/.zerg/memory` | Memory |
| `ZERG_KB_PATH` | `<工作区>/data/knowledge.db` | Knowledge-base SQLite (**related tools are skipped automatically when it does not exist**) |
| `ZERG_COMPRESS_MODELS` | `<工作区>/compress_models` | Optional compression models (LLMLingua-2 and the like) |
| `ZERG_EXTRA_ALLOW_DIR` | empty | **Extra allow-listed directories** for CA tasks (colon-separated) — both the file tools and the rm gate let these through |

> **`ZERG_EXTRA_ALLOW_DIR` is a security boundary**: only point it at directories you are willing to let the Agent read and write.

---

## 4. External integrations (optional)

| Variable | Description |
|---|---|
| `ZERG_SEARXNG_PY` / `ZERG_SEARXNG_SETTINGS` / `ZERG_SEARXNG_SRC` | Interpreter / settings file / source directory for a self-hosted searxng (by default it looks under `<工作区>/vendor/searxng/...`, then falls back to `python3` on `PATH`) |
| `ZERG_RTK` | `0` = disable the rtk command wrapper. **Degrades automatically when rtk is not installed**, so no setting is needed |
| `ZERG_ORNITH_TEMPLATE` | File path used when a model needs an external chat template to override a stale embedded one (also available as a registry field, see [MODELS.en.md](MODELS.en.md)) |
| `ZERG_AUTOCUT_DIR` / `ZERG_MUSIC_DIR` / `ZERG_DRP_FILE` | Optional directories / project files for the media toolchain |
| `ZERG_DF_PATHS` | Extra mount points counted by the disk-inspection tool (by default only `/` is inspected) |
| `ZERG_FONT_PATH` | CJK font file for the UI |

---

## 5. Runtime behaviour switches

| Variable | Default | Description |
|---|---|---|
| `ZERG_INTERNAL_TASKS` | **off** | Internal task engine (self-inspection / cleanup tasks). **Keep it off**: cleanup tasks delete files |
| `ZERG_LOG_LEVEL` / `ZERG_LOG_STDOUT` | `info` / — | Log level, and whether to also write to stdout |
| `ZERG_DEBUG` | — | Debug output |
| `ZERG_CHAT_CLEANUP_DRY_RUN` | — | Conversation cleanup preview, "report counts without touching data" |
| `ZERG_HERMES_TOOLS` | automatic | Use the XML tool-call protocol (more reliable for some models). Task scheduling sets this automatically |
| `ZERG_LEGACY_LOOP` | — | Fall back to the legacy tool loop (for troubleshooting) |

> The remaining variables starting with `ZERG_` are internal / development knobs (evals, replay, subtask experiments, and so on);
> do not change them in a production deployment. For the full list, just search the code: `rg -o 'ZERG_[A-Z_]+' core agent ui | sort -u`.

---

## 6. Model registry `gateway/fleet.yaml`

```yaml
auth:
  token: ""                 # keep empty: the token travels via environment variable / token file

models:
  <model-name>:             # the name is up to you
    - name: <model-name>
      host: local|worker1   # machine id declared in the fleet section
      backend: llama-server # or ds4-server, or a custom command via cmd:
      file: /abs/path/model.gguf
      mem_gb: 22            # estimated footprint while this model is loaded (used by scheduling)
      ctx_window: 262144    # context window (optional)
      modality: text        # or multimodal
      mmproj: /abs/path/mmproj.gguf   # multimodal projector (optional)
      cmd: "llama-server -m {file} --port {port}"  # fully custom launch command (optional)

aliases:                    # client alias → canonical model name (optional)
  my-alias: <model-name>

fleet:                      # machine list
  local:   { host: 127.0.0.1, port: 8100, os: macos }
  worker1: { host: 192.0.2.10, port: 8100, os: ubuntu }
```

No restart of core is needed after a change: `POST /api/config/reload` (token required).

---

## 7. Agent registry `agent_models.yaml`

An agent **itself** also needs a registry of "which models live on this machine" (it need not be identical to core's fleet — core cares about the "candidates", the agent cares about "how to bring them up"):

```yaml
<model-name>:
  backend: llama-server
  file: /data/models/x.gguf
  mem_gb: 22
  modality: text
  mmproj: /data/models/mmproj.gguf                    # optional
  chat_template: ~/.zerg/example-35b-v2_chat_template.jinja   # optional (~ is supported)
```

`chat_template` resolution priority: **registry field → `ZERG_ORNITH_TEMPLATE` → `~/.zerg/example-35b-v2_chat_template.jinja` → a relative path inside the repository**;
if any of them is configured explicitly but the file does not exist → log a WARN and keep falling back (a bad path is never handed to the backend).

---

## 8. UI preferences

`~/.zerg-ui-prefs.json` (generated automatically):

| Field | Description |
|---|---|
| `auth_token` | Token (if unset, the environment variable / `~/.zerg/token` is used) |
| `preview_renderer` | Markdown preview renderer choice (the startup default can be overridden with `ZERG_PREVIEW_RENDERER`) |

Runtime directory: `<tmp>/zerg-ui/` (module manifest and such); can be overridden with `ZERG_UI_DIR`.
