# Quickstart (QUICKSTART)

> **English** | [中文](QUICKSTART.zh-CN.md)
>
> *Source of truth: the Chinese original ([QUICKSTART.zh-CN.md](QUICKSTART.zh-CN.md)). If the two disagree, the Chinese text prevails.*
> *Derived translation — 派生自 zh-CN（真源在 zh 侧）。*

From zero to a working swarm, in four steps. The only part you need running with a single command is the **controller**; agents and the UI are optional but recommended.

> Convention: below, `<repo>` is the directory you cloned. Every port/path can be overridden by an environment variable, see [CONFIGURATION.en.md](CONFIGURATION.en.md).

---

## Step 0: Prepare the shared token

All components use the same token (the `X-Auth-Token` header). **Never commit it to the repo** — put it in an environment variable or `~/.zerg/token`:

```bash
cd <repo>
cp .env.example .env
# generate a random token (32-byte hex recommended)
TOKEN=$(openssl rand -hex 32)
sed -i '' "s|^ZERG_AUTH_TOKEN=.*|ZERG_AUTH_TOKEN=${TOKEN}|" .env   # macOS; on Linux use sed -i
set -a; . ./.env; set +a
```

You can also skip `.env` entirely: just write the token into `~/.zerg/token` (single line, `chmod 600`).

> ⚠️ **Quote any `.env` value that contains spaces or non-ASCII text** (e.g. `ZERG_KB_PATH="/Volumes/My Disk/knowledge.db"`).
> Otherwise `set -a; . ./.env` truncates the variable at the first space and tries to run the rest as a command
> (`no such file or directory`).
Resolution priority: `ZERG_AUTH_TOKEN` → `~/.zerg/token`.

> When the token is missing, the controller and agents **refuse to start** and print guidance — this is deliberate (an empty token makes every API return 401 while the logs look perfectly normal).

## Step 1: Write the model registry

```bash
cp gateway/fleet.example.yaml gateway/fleet.yaml
$EDITOR gateway/fleet.yaml      # fill in your own machines and weight paths
```

Minimal working example (single machine):

```yaml
auth:
  token: ""            # leave empty: injected from the environment variable / token file
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

> **You can name a model whatever you like** — as long as it is in the registry and the path points to the right place, Zerg can use it. Field reference: [MODELS.en.md](MODELS.en.md).

## Step 2: Start the controller

```bash
cd core
go build -o ../bin/zerg-core ./cmd/zerg-core
cd ..
set -a; . ./.env; set +a          # let the controller pick up the token
./bin/zerg-core
```

The controller listens on **`:8580`** (API) and also brings up **`:8082`** (OpenAI-compatible gateway). The startup banner prints the masked token, in the form `🔐 认证令牌: xxxx******(len=64)`.

**Verify** (this single check proves both "it is up" and "the token is correct"):

```bash
# without the token → 401 (auth is in effect)
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8580/api/capabilities
# with the token → 200
curl -s -H "X-Auth-Token: $ZERG_AUTH_TOKEN" http://127.0.0.1:8580/api/fleet/status | head -c 200
# OpenAI-compatible entry point
curl -s -H "X-Auth-Token: $ZERG_AUTH_TOKEN" http://127.0.0.1:8082/v1/models | head -c 200
```

## Step 3 (optional): Add an agent — now you have a cluster

An agent is what actually brings models up on a **worker machine**. On that machine:

```bash
# build (you can cross-compile it elsewhere and copy the binary over)
cd <repo>/agent
go build -o zerg-agent ./cmd/zerg-agent

# start it (give it the token)
ZERG_AUTH_TOKEN=<same token> ./zerg-agent \
  --host 0.0.0.0 \
  --port 8100 \
  --machine worker1 \
  --controller http://<controller address>:8580 \
  --registry agent_models.yaml
```

The agent heartbeats to the controller every 5 seconds. Confirm on the controller side:

```bash
curl -s -H "X-Auth-Token: $ZERG_AUTH_TOKEN" http://127.0.0.1:8580/api/fleet/status
# you should see worker1 with online: true, plus the models loaded on it
```

> Then declare this machine in the `fleet:` section of `gateway/fleet.yaml` (`worker1: { host: <its IP>, port: 8100, os: linux }`),
> and add an entry with `host: worker1` to some model's candidate list — the scheduler will then assign that model to load on it.

## Step 4 (optional): Desktop UI

```bash
cd ui
cargo build --release -p zerg-ui
cp target/release/zerg-ui ../bin/zerg-ui
../bin/zerg-ui
```

The UI takes the token from the environment variable or `~/.zerg/token`; you can also put it in the `auth_token` field of `~/.zerg-ui-prefs.json`.

- Endpoints can be overridden with `ZERG_API_BASE` / `ZERG_AI_BASE` (defaults `http://127.0.0.1:8580` / `:8082`)
- Chinese fonts are detected automatically (`ZERG_FONT_PATH` to specify one)

---

## Submit a task and try it

```bash
curl -s -X POST -H "X-Auth-Token: $ZERG_AUTH_TOKEN" -H 'Content-Type: application/json' \
  -d '{"description":"列出当前目录的文件，写一份简短说明到 report.md"}' \
  http://127.0.0.1:8580/api/tasks
```

The controller will: pick a model → find a machine with free VRAM (loading the model if necessary) → dispatch the task to an agent, which runs the Agent loop (tool calls + guardrail feedback) → status and logs are visible in the UI's "Tasks" panel.

## After you change the config or move machines

Once you have changed `gateway/fleet.yaml`, a port or a machine name, **do not keep running the old process on the old config**. Both commands below are best **dry-run first**:

- **Hot-reload the config**: run `zerg config reload --dry-run` first to read the plan (zero side effects — not a single byte is written), and once the impact looks right, `zerg config reload --yes` (needs the token · same effect as `POST /api/config/reload`). If the registry file does not parse locally, **no request is sent** and the old config keeps running (like `nginx -s reload`: validate first, roll back on failure).
- **Restart the controller after moving machines**: machine name / ports / service declaration changed — hot reload is not enough — `zerg core restart` **stops and starts the controller** (**the whole swarm's control plane goes down for a moment**). **Recommended**: `zerg core restart --dry-run` first to read the plan (which pid will be restarted, which ports and services are affected), and only run for real once the confirmation flags are complete: recommended, copy the **whole string**: `zerg core restart --confirm=<hostname> --yes`.

Both are **three-state**: `--dry-run` (zero side effects) · `--confirm=<target>` (the value must match the target verbatim) · `--yes` (the confirmation flag); `core restart` needs `--confirm` and `--yes` **together** — with either one missing it prints the plan only and **does not execute**.

**The readiness check is bound to the pid it started itself**: before restarting, note the pid and start time from `zerg core ps`; after the restart the pid **must be a new one** (the listening ports must change owner too) to count as really up — same pid and same start time means the old process is still there; never call that green. **Rollback**: if the config side fails, the old config keeps running — fix the file and reload once more; if the process side does not come up / does not become ready, simply run this command again, and rolling back a *binary* is a different path (`scripts/build/zerg-swap-core.sh`).

## Common pitfalls

| Symptom | Cause / fix |
|---|---|
| All `/api/*` return 401 | The token was not supplied: `export ZERG_AUTH_TOKEN=...` or write `~/.zerg/token` |
| The controller exits immediately and reports "shared token not configured" | Same as above (this guardrail is intentional) |
| The UI shows "controller offline" | Wrong endpoint (`ZERG_API_BASE`) or wrong token |
| Agent is online but model loading fails | Wrong weight path / not enough VRAM; check the agent log and the `llama-server` output on that machine |
| `ls/git/go…` commands report `rtk: command not found` | Legacy issue from older versions, now degrades automatically; you can also disable it explicitly with `ZERG_RTK=0` |
| `web_search` is unavailable | Requires a self-hosted searxng, see `scripts/install-searxng.sh` (AGPL component, not distributed with the repo) |

See [FAQ.en.md](FAQ.en.md) for more.
