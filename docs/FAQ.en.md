# Frequently Asked Questions (FAQ)

> **English** | [中文](FAQ.zh-CN.md)
>
> *Source of truth: the Chinese original ([FAQ.zh-CN.md](FAQ.zh-CN.md)). If the two disagree, the Chinese text prevails.*

## Deployment and startup

**Q: The controller exits right after startup, saying "shared token not configured"?**
This is deliberate fool-proofing. An empty token makes every `/api/*` return 401 while the process looks perfectly fine — the hardest class of failure to diagnose.
Set `ZERG_AUTH_TOKEN` as prompted, or write it to `~/.zerg/token` (a single line).

**Q: All APIs return 401?**
The token isn't getting through. Check: ① is `ZERG_AUTH_TOKEN` in that process's environment (`set -a; . ./.env; set +a` before starting);
② is the client sending the `X-Auth-Token` header; ③ the gateway also accepts `Authorization: Bearer` and `x-api-key` (they are converted to `X-Auth-Token` and passed through).

**Q: The UI shows "controller offline"?**
`curl` the controller port first: a 401 means the service is running and it's a token problem; if it can't connect, look at `ZERG_API_BASE` and the firewall.
(Inside the UI the system proxy is disabled for loopback requests — if you write your own client, note that some proxy settings will swallow `127.0.0.1` requests.)

**Q: An agent is online, but loading a model fails?**
Look at the agent log and the backend's real error: weight path, out of VRAM, backend binary not on `PATH`, port already taken.
Setting `mem_gb` lower than reality makes scheduling over-optimistic — err on the large side.

**Q: Can I skip installing `rtk`?**
Yes. When it isn't installed the system degrades automatically (commands are no longer prefixed); you can also turn it off explicitly with `ZERG_RTK=0`.

**Q: `web_search` reports an error?**
It depends on a self-hosted searxng (an AGPL component, **not distributed with the repo**). Install it with `scripts/install-searxng.sh`, then
point `ZERG_SEARXNG_PY`/`ZERG_SEARXNG_SETTINGS` at it; without it the web-search tool is unavailable and the rest of the functionality is unaffected.

---

## Usage

**Q: Can I name models whatever I like?**
Yes. The name is just a key — register a consistent key in `agent_models.yaml` (how the agent launches it) and `gateway/fleet.yaml` (which candidates the controller has).
Adapters match by **prefix**, so for example the `example-35b` family shares one set of launch parameters.

**Q: Can two models run at the same time?**
Not on a single machine (**single-slot** constraint: VRAM reality). Put the second model on another machine, as a candidate of the same model or as a different model, and they run in parallel.

**Q: Will task execution delete my files?**
Deletion-type shell commands are bound by the **scope gate**: they may only act on directories whitelisted in the task workspace `ZERG_EXTRA_ALLOW_DIR`;
the home directory, the root directory and their ancestors are always refused, with actionable alternative guidance. Do **not** point the whitelist at sensitive directories.

**Q: A task is stuck / keeps re-sending the same command?**
Start with the CA event log `events.jsonl` (default `/tmp/zerg-ca-logs/<时间戳>/`, or change it with `ZERG_CA_LOG_DIR`):
repeated tool calls plus a growing round count usually mean "the system isn't giving actionable feedback" (a path was refused, an argument was malformed).
Feed the refusal messages from the log to the maintainers, or check whether the paths written in the task description are inside the whitelist.

**Q: What is the internal task engine? Should I enable it?**
Don't. It contains self-inspection/cleanup tasks (which delete files) and is off by default (`ZERG_INTERNAL_TASKS` unset means off).
This project's position is: such automatic cleanup must be enabled by the user, after understanding the consequences.

---

## Data and privacy

**Q: Does Zerg send data back?**
No. Every component runs on your own machines; there is no telemetry and no account system. Model calls only go to backends you configured yourself.

**Q: Where are conversations/memory stored?**
Conversation DB `~/.zerg-chat/chat.db`; memory `~/.zerg/memory/`; tool state `~/.zerg/state/`.
All of them can be moved elsewhere with environment variables (see [CONFIGURATION.en.md](CONFIGURATION.en.md)).

**Q: How do I back up?**
Back up the three directories above (for SQLite, prefer `VACUUM INTO` over copying the .db file directly).

**Q: Are tokens/passwords in the repo?**
No. The repo has **zero plaintext credentials**: tokens go through environment variables or `~/.zerg/token`;
the release pipeline also scans for plaintext credentials, private paths and internal addresses before export, and aborts on a hit.

---

## Development

**Q: Where should I start reading the code?**
`core/internal/api` (scheduling and HTTP) → `core/internal/chat` (conversation and tool loop) → `core/internal/agent` (tools and gates)
→ `agent/` (the agent). The desktop UI is in `ui/src`.

**Q: How do I add a tool?**
Implement it + register it in the tool registry + write its `tools/<name>.md` record (and keep the tool summary in sync — it feeds both the UI and the model prompt).
Note that the directory holds two "manifests" (always-on L0 and lazily loaded); the registry is the single source of truth, and unit tests keep the two sides consistent.

**Q: How can I tell whether a mechanism is really running?**
Don't just read the code. This project leans heavily on "process + data, dual proof": for example, checking whether a counter actually lands on disk, whether an endpoint is really registered,
whether a flag is really read. This is also a hard requirement in the contribution guide.
