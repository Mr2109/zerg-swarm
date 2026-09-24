# Frequently Asked Questions (FAQ)

> **English** | [中文](FAQ.zh-CN.md)
>
> *Source of truth: the Chinese original ([FAQ.zh-CN.md](FAQ.zh-CN.md)). If the two disagree, the Chinese text prevails.*
> *Derived translation — 派生自 zh-CN（真源在 zh 侧）。*

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

---

## Command line

**Q: The trust level in the resource API is a machine code now — what about my scripts?**
On the data plane it is an **ASCII machine code** from a closed set of three: `new` (newly added / before its 100th use), `official` (promoted), `unknown` (unregistered / missing).
Both the API responses (the face `zerg resource ls` projects) and the state file (`~/.zerg/state/zerg-resources.json`) carry machine codes;
Chinese lives only in the display layer (the keys in the UI locale files), so the interface still shows Chinese.
**Callers that hard-code the old Chinese literals must be updated**: the old values were `新` / `正式` / `未知`, and any script, client or adapter that compares against them has to compare against the machine codes instead.
The Chinese written by older versions is translated once on validation/read (`新` → `new` · `正式` → `official` · `未知` / empty → `unknown`); the write side only ever writes machine codes, and old files are neither deleted nor rewritten.
The values are part of a **breaking interface change** (server and UI artifact change in the same batch) — update one side only and the two stop agreeing.

**Q: Writing a file into the archive area is refused, saying read-only?**
The archive is not an ordinary directory: it is the **read-only image mounted on demand**. A refused write is its normal state — not a permission bit set wrong (`chmod` cannot change a read-only volume either).
There is a single mount surface: one script with three idempotent actions (nothing resident · nothing auto-mounted at boot):
`bash scripts/svc/archive-mount.sh status` (report the mount state) · `bash scripts/svc/archive-mount.sh off` (detach) · `bash scripts/svc/archive-mount.sh on` (attach).
To change archive content: run `off` first to detach the read-only image, make the change, then `on` to attach it back.
★ The "not mounted" state is **not** "normal": when `status` reports "not mounted" it exits **1**, and at that moment the original path is a **dangling symlink** — reading it **fails outright** (it does not "read empty"); don't treat the original path as an ordinary directory while it is unmounted.
Exit codes: `0` success · `1` failure (including `status` reporting "not mounted") · `2` no conclusion (a precondition cannot be judged: the image file is missing / the original path is not a symlink / the two readings disagree) · `64` usage error.
If it will not detach (some process is holding files inside the image), find out who holds it first; do **not** force it with `-force`.

**Q: I only want to commit the paths I name — and what if the gate is red but I need an audited exception?**
Name the path: `zerg repo commit --only <path> --message <subject>` — it does **not** `git add` (the named paths go straight to `git commit --only`), and the index **may be non-empty** (other people's staged work is not touched at all).
Several files take **one `--only` per file** (each takes a single value); `.` / `-A` / globs / `:` are all refused (they are the bulk-staging back door).
Three states: `--dry-run` prints the plan (zero side effects — nothing staged, nothing committed, no gate run) · without `--yes` it does **not execute** (exit 2, it never asks) · with both it really runs.
A real run goes **through the fast gate first** (`bash scripts/gates/precommit-gates.sh --fast --outdir <dir>`): if its exit code is not 0, **nothing is committed** and the index and work tree are byte-for-byte untouched.
When a human-signed exception is genuinely needed, use the audited flag approved for it: `--waive <step> --reason <one line>` — without `--reason` it is refused outright (exit 2: an exception with no reason is not granted);
the step name must **really appear in that run's report**, or the exception does not take effect; and every exception is written to the audit log (who · when · which step · why), so the red can be read back afterwards.
The command surface offers **no** `--no-verify` (bypassing is for a human to do explicitly).

---

## Multi-machine and publishing

**Q: Why must one session land on the same machine?**

A: Locally served models each hold their own KV cache and runtime state per box. Bouncing one session between two machines means every turn starts cold (the context must be prefilled again) — slow, and easy for the model to read as "a new conversation". So a session prefers to land back on the same box (see "Session affinity and kind-tier scheduling" in [ARCHITECTURE.en.md](ARCHITECTURE.en.md)).

**Q: Does `zerg publish run` push anything to a remote?**

A: No. `zerg publish run` only builds the public mirror tree in the local artifact dir (allowlist + excludes + redaction) and **never touches the network**. Pushing to a remote is a separate, explicit action with its own confirmation. To see what it would do first, use `zerg publish run --dry-run`.
