# check-version-sources —— 门② · 版本源一致性（四处同版）+ 活文档旧版号只报告

> **状态（2026-09-18 收尾更新）：已挂闸 —— `gates` scope · 模式 `tri`（阻断）· 在默认集内**。
> 步骤名：`门② 版本源：四处同版（阻断）`。自检数字：**`--self-test` rc=0 · 16/16 断言通过**。
> 原文「挂接未做 —— 由父代理决定何时进 `scripts/precommit-gates.sh`」**已被事实取代**（不再适用）。
> 本文件 + `scripts/check-version-sources.py` **两件全是新增**；同批只另有**一处真斜账修复**：`wall/Cargo.toml` 的
> `version` 由 `2.5.9` → `2.5.10`（**只改版本值**，其余一字未动）。**未改任何其它文件 · 未做任何 git 操作**。
> 目标是**阻断档**：四处版本源当天不一致就红，而不是「先只报告」。

---

## 0. 一页速览

| 项 | 结果 |
|---|---|
| 交付物 | `scripts/check-version-sources.py`（36 KB · 单文件 · 只用标准库 · python3.9 可跑）· 本说明书 |
| 判据（阻断档） | **R1** 四处同版 · **R2** 仓根 `VERSION` 无数值 · **R3** `VERSION` 指针点名真源 · **R4** 缺件/不可解析 ⇒ BLOCKED · **R5** 空转 ⇒ BLOCKED |
| 只报告项 | **D1** 活文档里仍写着旧版本号字面量（逐条 `文件:行` + 形态分类；**不进退码**） |
| 三档退码 | `0` 全绿 · `1` 有失败项 · `2` **不给结论**（缺件/不可解析/空转/用法错）—— 与 `precommit-gates.sh` 的 `tri` 模式同口径 |
| 自证 | `--self-test`：**成对负控 16 格 · 16/16 通过 · rc=0**（好件必绿 ↔ 四处每处改一个数必红 ↔ 缺件必 rc=2 ↔ 只报告项不进 rc） |
| 修前首跑（真仓） | `rc=1` · 1 条失败项：`wall/Cargo.toml:3 version = 2.5.9 ≠ 基准 2.5.10`（**这正是今天那笔真斜账**） |
| 修后首跑（真仓） | `rc=0` · 四处同版 `2.5.10` · 只报告项 33 文件 / 117 处（裸数字 29 处） |
| 挂法（**已执行** · 2026-09-18 收尾更新） | 已进 `precommit-gates.sh` 步骤表：**`gates` scope · 模式 `tri`**（亚秒级只读，无外部依赖）；rc=2 记 BLOCKED、不计失败项。★ 原文「建议 · 未执行」已作废 |
| 取样时刻 | **2026-09-18T22:4x+08:00**（语料 602 篇；其它路同时在往 `docs/` 加件 ⇒ **谁跑谁记时刻**） |

---

## 1. 用法

```bash
cd "<repo>"

python3 scripts/check-version-sources.py                 # 扫真仓（阻断档断言 + 只报告清单）
python3 scripts/check-version-sources.py --list-rules    # 规则表（判据 / 排除域 / 旧版集合现算 / 退码映射）
python3 scripts/check-version-sources.py --self-test     # 成对负控自检（不碰真仓，夹具落 tempfile.mkdtemp）
python3 scripts/check-version-sources.py --json          # 机器可读（stdout 只有 JSON）
python3 scripts/check-version-sources.py --strict-docs   # 加严：活文档命中 ⇒ 计入 rc=1（默认只报告）
python3 scripts/check-version-sources.py --stale 2.5.9 --stale 2.5.8   # 手工指定旧版集合
python3 scripts/check-version-sources.py --stale-all     # 旧版集合 = 全部已知 3 段历史版本
python3 scripts/check-version-sources.py [仓库根]         # 指定根（自检夹具走的就是这条）
```

| 开关 | 语义 |
|---|---|
| （无参） | 四处版本源断言（阻断） + 活文档旧版号扫（只报告） |
| `--json` | 同一份判据，输出 JSON（`rc` / `sources` / `problems` / `blockers` / `docs.hits` 全在） |
| `--strict-docs` | 把 D1 的命中计进 rc=1 —— **改的是档位、不改判据**（与仓库既有口径一致） |
| `--stale X.Y.Z` / `--stale-all` | 覆盖旧版集合（默认 = 直接前驱版；见 §3） |
| `--self-test` | 好件/坏件/缺件成对负控 16 格；**不通过就别信它的红绿** |
| `--list-rules` | 规则表，含**本仓现算**的当前版 / 已知历史版本目录 / 旧版集合 |

**只读保证**：脚本不往仓内任何路径写（自检一律落 `tempfile.mkdtemp()` 并在收尾 `rmtree`）。
**不接管道取退码**：要拿退出码就别接 `| tail`（`$?` 会是管道后段的）。

---

## 2. 判据（阻断档 · 全部可判定，无词表）

| 判据 | 内容 | 判定 |
|---|---|---|
| **R1 四处同版** | **基准** = `core/internal/version/version.go` 的 `const Version = "X"`；`ui/Cargo.toml` 与 `wall/Cargo.toml` 的 **`[package]` 段** `version` 必须等于基准 | 不等 ⇒ **rc=1**，逐条打印 `文件:行 version = 值 ≠ 基准 值（基准真源 = version.go:行）` |
| **R2 VERSION 无数值** | 仓根 `VERSION` 里出现任何 `X.Y.Z` 字面量 | 出现 ⇒ **rc=1**（逐行点名 + 列出数值）。理由：`VERSION` 已**指针化**，写数字就是第二份真相 |
| **R3 VERSION 指真源** | 指针文本必须同时含两个**路径串**：`core/internal/version/version.go` 与 `ui/Cargo.toml` | 缺任一 ⇒ **rc=1**（判据是路径串，不是措辞） |
| **R4 缺件/不可解析** | 任一源缺失 · 不是 UTF-8 · `version.go` 无 `const Version` · Cargo.toml 的 `[package]` 段无 `version` · `VERSION` 是空文件 | ⇒ **rc=2（不给结论）**。★ **不许回落猜**：`[dependencies]` 里的 `version = "1"` 不算（自检 ⑪ 钉住） |
| **R5 空转防护** | 活文档语料 0 个文件被扫到（而旧版集合非空） | ⇒ **rc=2** —— 0 个可比对 ≠ 绿（与 `precommit-gates.sh` 的空转口径同族） |

**为什么基准取 `version.go`**：`VERSION` 的指针文本与 `version.go` 头注释都写明 Go 侧是「版本号单一来源」；
本门只是把这句话**机器化**（并顺带压住 `wall` 这条新出现的第四处）。**基准口径可拍板改**，改的是一处常量、判据不动。

| 只报告 | 内容 |
|---|---|
| **D1 活文档旧版本号** | 语料域 = `docs/**`（走 §3 排除域）+ 仓根 `*.md`；命中逐条列 `文件:行`，并按**形态**分两类：<br>· `REF` 版本名/路径形态（`v2.5.9`、`…/v2.5.9/…`、`变更-v2.5.9.md`）—— 指的是旧版的**名字**，多为合法引证<br>· `BARE` 裸数字形态（`2.5.9 → 2.5.10`、`2.5.9 增补`、`现 2.5.9`）—— **需人判读**的候选（可能是陈旧陈述，也可能是沿革陈述）<br>命中**只入清单**：不计失败项、不计 BLOCKED、不影响退出码 |

---

## 3. 活文档语料的排除域 与 旧版集合

| 排除域 | 理由 |
|---|---|
| `docs/虫族文档/`（1981 件） | 镜像副本（公开面镜像，冻结） |
| `docs/02-调研/raw/`（48 件） | 原始调研证据（一手日志，冻结） |
| `docs/issues/`（1086 件） | 历史报告 / 事故记录（记录型，冻结） |
| `docs/项目文档/v<X>/`（X ≠ 当前版；**293 件**） | 他版快照目录（17 套历史快照，冻结归档） |
| `docs/项目文档/v<当前版>/` | **不排除** —— 它是活文档，命中要进清单 |
| `docs/.*` · `*/.hidden` | 工具/隐藏目录（如 `docs/.obsidian/`） |

**排除域只影响 D1（只报告）那一段**；四处版本源的断言不受任何排除域影响。
**旧版集合**默认 = **直接前驱版** = `docs/项目文档/v*/` 里已知的 3 段版本号中 < 当前版的最大者
（今天 `{2.0.0, 2.5.0…2.5.10}` ⇒ 旧版 = **`2.5.9`**）；`--stale` / `--stale-all` 可覆盖。
**判读口径**（写清以免误读红）：旧版号出现在**沿革/变更账/引证**里是设计，不是债 —— 本项**只报告不判决**；
要不要转阻断档（比如「只剩沿革行才允许存在」）以及门槛多少，**待 Mr2109 拍板**。

---

## 4. 自检（成对负控 · 16 格）

夹具 = 临时目录里的**合成四源**（`9.9.9`）+ 活文档 + 冻结节；**同一份判据函数**跑每一格。

| 格 | 内容 | 期望 |
|---|---|---|
| 正控① | 四处同版 `9.9.9` + `VERSION` 纯指针 ⇒ 必绿（活文档命中只报告） | rc=0 |
| 负控② | `version.go` 改一个数 | rc=1 · 判词含基准路径 |
| 负控③ | `ui/Cargo.toml` 改一个数 | rc=1 · 判词含 `ui/Cargo.toml` |
| 负控④ | `wall/Cargo.toml` 改一个数 | rc=1 · 判词含 `wall/Cargo.toml` |
| 负控⑤ | `VERSION` 里又出现数值行 | rc=1 · 判词含 `VERSION` |
| 负控⑥ | `VERSION` 指针不再点名真源 | rc=1 · 判词含「没点名真源」 |
| 负控⑦–⑩ | 分别缺 `VERSION` / `version.go` / `ui/Cargo.toml` / `wall/Cargo.toml` | rc=2 · 判词含「缺件」 |
| 负控⑪ | Cargo.toml 的 `[package]` 段无 `version`（只有依赖行有） | rc=2 · 判词含「不可解析」 |
| 负控⑫ | `VERSION` 是**空文件** | rc=2 · 判词含「空文件」 |
| 负控⑬ | 活文档语料 0 个文件被扫到 | rc=2 · 判词含「空转」 |
| 成对⑭ | 活文档写着旧版号 ⇒ 默认**只报告**（rc 仍 0，但清单里有它） | rc=0 · 清单含活文档 |
| 成对⑮ | 同一夹具 + `--strict-docs` | rc=1（加严档有牙齿） |
| 成对⑯ | 冻结节/排除域里的旧版号**不入清单** | rc=0 · 排除域零命中 |

本机实跑：**16/16 通过 · rc=0**（`python3 scripts/check-version-sources.py --self-test`）。
落码口径：自检**跑不起来** ⇒ rc=2（不给结论）；有格失败 ⇒ rc=1。

---

## 5. 首跑真数字（真仓 · 现跑）

**修前**（`wall/Cargo.toml` 仍是 `2.5.9`）：

```
✗  wall/Cargo.toml:3   2.5.9   茧壁 crate（升级比较用）（取自 [package] 段）
问题（1 条）：
  FAIL wall/Cargo.toml:3 version = 2.5.9 ≠ 基准 2.5.10
       （基准真源 = core/internal/version/version.go:12 的 `const Version`）
       —— 四处版本源（version.go / ui / wall）必须同版
结果: FAIL（有失败项）  rc=1
```

**修后**（同批修了 `wall/Cargo.toml` 那一行）：

```
✓  core/internal/version/version.go:12   2.5.10   基准（真源）
✓  VERSION                               —        指针（非真源）（无数值行=True · 指向真源=True）
✓  ui/Cargo.toml:3                       2.5.10   UI 侧（egui 桌面端）
✓  wall/Cargo.toml:3                     2.5.10   茧壁 crate（升级比较用）
问题（0 条）：四处版本源同版 ✓     结果: OK（全绿）  rc=0
```

**只报告（同一跑 · 不进退码）**：语料 **602** 件（实扫 **601** · 跳过 1 = `docs/调研/multi-agent-源码/learn-claude-code/web/src/app/favicon.ico` 非 UTF-8）；
命中 **33 文件 / 117 处**（`REF` 88 · **`BARE` 29**）。**裸数字 29 处的落点（11 个文件）**：

| 文件 | 行 | 判读提示 |
|---|---|---|
| `docs/01-设计/设计-文档体系-v1.0.md` | 220(×2),221,226,559,1491,1492,1498,1504(×2),1543 | 多为**当日取证表/沿革**（写的是「当时的真源值」），须逐行判 |
| `docs/01-设计/设计-源码式发布与升级.md` | 352(×2) | tag↔版本盘点行（历史值） |
| `docs/01-设计/设计-自动升级模块.md` | 39, 258 | **L39「现 2.5.9」= 陈旧陈述**（今天已是 2.5.10）；L258 机群矩阵实测值 |
| `docs/01-设计/拍板单-文档体系-P1第三批.md` | 46, 51 | 标题即「两处仍写 `2.5.9`」的待办项 |
| `docs/01-设计/设计-UI-大调动-导航精简与分组.md` | 25, 193 | **「仍 2.5.9，由Mr2109决定何时开新版」= 当时的陈述**（今天已升） |
| `docs/01-设计/设计-升级器-TUF-最小子集.md` | 97 | 规格示例 `min_upgrader_version: "2.5.9"`（示例值） |
| `docs/01-设计/设计-虫茧独立仓.md` | 44 | 对比表（跟随主仓 `2.5.9`） |
| `docs/项目文档/v2.5.10/07-模块-UI界面-20260829.md` | 4 | **「当前 2.5.9」= 陈旧陈述**（活文档，指向真源的那句） |
| `docs/项目文档/v2.5.10/债务台账-20260918.md` / `.tsv` | 119(×2),182 / 46,47,48 | 台账**自述本条债**（含本门的建议行） |
| `docs/项目文档/v2.5.10/变更-v2.5.10.md` | 24 | 变更账「版本号两处真源 = 2.5.9（未动）」= 沿革，**应保留** |

⇒ 本门**不替人拍「哪条是债」**：上表里「陈旧陈述」与「沿革取证」必须分别处置（前者可改、后者须留并加注）。
`REF` 88 处（`v2.5.9` / 路径名 / 旧文件名）基本是**旧版的名字**，逐行清单用 `--json` 取。

---

## 6. `wall` 里除 `Cargo.toml` 之外还有没有写死版本？（只报告 · 不改）

| 落点 | 形态 | 判定 |
|---|---|---|
| `wall/src/main.rs:46` | `println!("zerg-wall {}", env!("CARGO_PKG_VERSION"))` | ✅ **没写死** —— 编译期取 `Cargo.toml`（且 `wall/tests/cli.rs:156` 有用例钉 `--version`） |
| `wall/Cargo.lock:7` | `version = "2.5.9"`（`zerg-wall` 包条目） | ❌ **是……但属机械同步物**：本批跑门禁（`cargo fmt/clippy/test`）时 `cargo` **当场自己把它改成了 `2.5.10`** ✓（实测：改动前是 `2.5.9`，跑完 `cargo test` 后 `git diff wall/Cargo.lock` 已在同一行追平）★ **本门不管锁文件**，见下 |
| `wall/README.md:4` | 指向 `docs/项目文档/v2.5.9/任务单-茧壁-20260916.md` | ⚠ **路径指路型**（指的是冻结快照目录的名字，不是「本版版本号」）—— **不建议改**（快照目录名是历史事实） |
| `wall/src/**` 其它 | 只有 `PLAN_SCHEMA_VERSION`（卵声明**格式版本**，=1，与虫族版本号是两个轴） | ✅ 与版本真源无关 |

★ **待拍（本路不动）**：`wall/Cargo.lock` 的 `version` 与 `Cargo.toml` 同源、**该不该入本门判据**？
现在的取舍 = **不判**（它是 `cargo` 的产物，判它会把「工具改锁」误报成红）；但它确实会滞后一格（今天仍是 `2.5.9`，
由下一次 `cargo` 调用自动追平）。若要与 `Cargo.toml` 一起断言，加一条 R 即可（判据是一行常量）。

---

## 7. 挂接（**已执行 · 2026-09-18 收尾更新**）与承接项

> ★ 原文写「建议 · 本路未执行」——**该状态已终结**：本门已挂进 `gates` scope，模式 `tri`（阻断），在默认集内。
> 真实落入步骤表的形态是：
> `add_step gates "门② 版本源：四处同版（阻断）" tri "${REPO_ROOT}" "python3 scripts/check-version-sources.py"`。
> 下面保留原「挂法」正文（scope 建议取 `pub` **未被采纳**，实挂 `gates`，理由 = 门② 的域是仓根四源 + `docs/` 语料，
> 与「门自己的门」同族；证据不变：本机单跑亚秒级、纯标准库）。
> - **挂法（原建议，未采纳）**：进 `scripts/precommit-gates.sh` 的**步骤表**（一等步骤，不是脚本尾部软检查位），
  模式取 **`tri`**：`add_step <scope> "版本源一致性（四处同版）" tri "${REPO_ROOT}" "python3 scripts/check-version-sources.py"`。
  建议 scope 取 `pub`（它只读仓根四文件 + `docs/` 扫描，公开面同样成立）；**证据**：本机单跑 **亚秒级**
  （602 件语料 + 四源解析），无外部依赖（纯标准库）。
- **承接项**：① ~~挂接（父代理）~~ **已办**（`gates` scope · `tri` · 默认集内）② D1 是否转阻断档 + 门槛（待 Mr2109 拍）③ `wall/Cargo.lock` 是否纳入判据（待拍）
  ④ 上表 3 条「陈旧陈述」型活文档的改正（属文档债，本路只报不改）⑤ 同族债：`core/internal/version/version.go`
  与 `agent/internal/version/version.go` 的**头注释**里仍写 `2.5.9`（代码注释，本次未动 —— 见父代理报告）
  ⑥ **本路新增的 `scripts/check-version-sources.{py,md}` 需在 `scripts/check-doc-name.py` 的 C 档豁免表登记**
  （与 C01–C04 同形：`{"id": "C05", "kind": "pair", "rules": ["N6"], "paths": [("scripts", "check-version-sources")], "reason": "门脚本 + 说明书（同 `scripts/edit-assert` 既有约定）"}`）
  —— **本路无权重写该文件**（父代理/文档面属主收口时一并办）。

---

## 8. 同批门禁实跑（2026-09-18 · `wall/Cargo.toml` 改版值之后）

命令：

```bash
cd "<repo>"
ZERG_STATE_DIR=/tmp/zerg-gate-state-vs bash scripts/precommit-gates.sh --outdir /tmp/gates-vs
```

**结果：步骤 25 · 通过 23 · 失败项 1 · 不给结论(BLOCKED) 1 ⇒ rc=1（红灯）**。逐条归属：

| 步骤 | 判定 | 是否本路（`wall/Cargo.toml` 改版值 / 新增两件）造成 |
|---|---|---|
| `gofmt`×2 · `build`×4 · `vet`×2 · `go test`×2 · `go test -race`×2 | PASS（0–25s） | 无关 |
| **`cargo fmt --check` / `cargo clippy -- -D warnings` / `cargo test`** | **PASS**（0s / 1s / 7s） | ★ **本路那行版本值就在这里被真跑过** ⇒ 改版值与 `rust` scope 相容 |
| `scripts/*.sh` · `*.py` · 无后缀 · `check-shell-unicode-vars` | PASS | ★ 新增 `.py` **确实被扫到**（`scripts/*.py` 的 `ast.parse` 步骤覆盖）——可见性已确认 |
| `双构建工程门禁`（tags） | PASS（9s） | ★ 与 `wall/Cargo.toml` 无关，但父代理要求跑，已跑 |
| `docs: meta --scope formal --missing=fail` | PASS | 新增 `.md` 通过 meta 门 |
| **`docs: name --scope repo`** | **FAIL（唯一失败项）** | ⚠ **`N6 同目录同基名、扩展名不同` 命中 3 个「门脚本 + 说明书」对**：`scripts/check-gate-coverage` · **`scripts/check-version-sources`（本路）** · `scripts/check-wired-scripts`。★ 决定性对照（同一条命令、只挪走本路 `.md`）：**不合规 3 → 2**、`N6 可改面 3 → 2` ⇒ **本路贡献恰好 1 条**，另 2 条属同批其它两路（三对都需按 §7⑥ 登记进 C 档） |
| `docs: freshness D1/D2` | PASS / 0 | 无关 |
| **`docs: freshness D3`** | **BLOCKED（rc=2）** | ⚠ **与本路无关且先于本批存在**：判词是「scope main 语料 303 篇里 **0 条带锚链接 ⇒ 空转**」——`scripts/check-doc-freshness.md` §0 早在今天 W3 交付时就记着 D3=rc=2 空转。⇒ **默认门禁集本来就是 rc=2（不绿）**，不是本批造成 |

**结论（如实）**：`wall/Cargo.toml` 那行版本值**没有**让任何一道门变红（`rust`/`tags`/`pub` 全绿，且门禁里没有任何一步读它的**值**）；
本批的 rc=1 来自 `docs: name` 的 N6 登记滞后（三对门脚本+说明书 × 1 条豁免表），rc=2 来自 D3 空转（先于本批）。
⇒ **未回退那行**：回退它不会让门禁变绿（红与它无关），只会把「四处版本源已同版」这个已修好的事实打回去。
`wall/Cargo.lock` 的那一行由 `cargo` 在本次跑门禁时自动追平（属机械同步，见 §6）。
