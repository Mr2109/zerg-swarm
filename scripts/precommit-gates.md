# precommit-gates — 提交前门禁总控（四档 · `docs`+`gates` 进默认集 · D2 只报告 · 门③ 已升阻断）

> 脚本本体：`scripts/precommit-gates.sh`（本文件是它的说明书，改动必须与脚本同步）
> 本文件由 **2026-09-18 路 D（门禁挂接 · Q14）** 新增；**2026-09-18 拍板①**（「按建议走」）后更新为
> 「**`docs` 进默认 scope 集** + **D2 改为只报告**（第四档 `tri-report`）」。
> **2026-09-18 第二波**（`scripts/precommit-gates.sh` 的 **第二波**）：**补真缺口**（`ui/` 三个 cargo 步 ·
> `shared/`+`scripts/exportnames/` 两个 Go module · 四棵外围目录+顶层入口的脚本语法）＋ **挂上「门自己的门」
> 三条**（门①②③ · 新 scope `gates` · 进默认集）⇒ 默认全量 **37 步**。逐条见 §二之二、§三、§四。
> **2026-09-18 ③批次（收尾二）**：门③ 报的 **8 只未挂门脚本逐只上岗**（docs +2 · pub +2 · 新 scope `tools` +1 ·
> 新 scope `slice` +1 · 发布闸 闸②b/闸⑥）⇒ 门③ 存量 **8→0** 并**升为阻断 `tri` + `--strict-report`**；
> 默认全量 39 → **45 步**（两个新 scope 都进默认集）。逐条见 **§七**（本批自查节）与 §二/§二之二/§五的 ★ 行。
> 其余（一步一文件日志 · 失败数只从结果表读 · 尾部软门禁的来龙去脉）见脚本头部注释与 `docs/` 内相关稿。

## 一、步骤级四档：PASS · FAIL（失败项）· BLOCKED（不给结论）· **REPORT（只报告）**

| 模式 | rc=0 | rc=1 | rc=2 | 异常码（3/127/…） | 计失败项数 | 计 BLOCKED | 影响退出码 |
|---|---|---|---|---|---|---|---|
| `rc`（老） | `PASS` | `FAIL` | **`FAIL`**（老语义：含 2 一律失败） | `FAIL` | ✓ | ✗ | ✓ |
| `empty`（老） | 输出空⇒`PASS`，非空⇒`FAIL` | `FAIL` | `FAIL` | `FAIL` | ✓ | ✗ | ✓ |
| `tri`（三档） | `PASS` | `FAIL` | `BLOCKED` | `FAIL` | 仅 rc=1 | ✓ | ✓（无 FAIL 而有 BLOCKED ⇒ rc=2） |
| **`tri-report`（本次新增·第四档「只报告」）** | `PASS` | **`REPORT`** | `BLOCKED` | `FAIL` | **✗（rc=1 不计）** | **✗（rc=1 不计）** | **✗（rc=1 不影响）** |
| 模式名不在 `{rc,empty,tri,tri-report}` | — | — | — | — | ✓ | ✗ | ✓（硬红） |

**`tri-report` 与 `tri` 只差 rc=1 那一格**：0/2/异常码三格的判定与 `tri` **一字不差** ——
「有红」才降为只报告；**「判据不可判 / 空转」仍是 BLOCKED，不许当绿**（rc=2 依旧把整脚本拉成 rc=2）。

**五条拍板口径（写死在代码里）**

1. **BLOCKED 不计入失败项数**。「没结论」不是「错」：`count_fail` 只数 `^FAIL`，`count_blocked` 只数 `^BLOCKED`
   （`count_*` 全是**纯读结果表**的函数，全脚本没有任何地方累加）。来自 Q14 的 Ø14 加强版。
2. **BLOCKED 也不许当绿**。无失败项、但有 BLOCKED ⇒ 整脚本 `rc=2`（不是 `rc=0`），并且**不打印「门禁全绿」**。
3. **REPORT 只入清单**（本批新增）：不计失败项数、不计 BLOCKED、**不进 0/1/2 这三档 ⇒ 不影响退出码**。
   它照 FAIL/BLOCKED 一样在报告里给出日志路径 + 尾部原文（前缀 `·`，措辞写明「只报告」），并进
   `状态计数`／`步骤总数` 两个汇总行（新增 `REPORT(只报告)` 一格）—— **可见，但不拦截**。
4. **异常码不许冒充「没结论」**。进程根本跑不起来（127）、被信号打断、脚本自己崩（3）——都是**红**；
   同理在 `tri-report` 下也**不许**借这一档洗成「只是报告了一下」。自检 ⑧b / ⑨c 钉住。
5. **老模式一字未改**。`rc` 模式下 `rc=2` **仍旧算失败项**——现有 20 步（go 12 · rust 3 · pub 4 · tags 1）的
   名字、命令串、模式、目录全部原样（`grep -c 'add_step \(go\|rust\|pub\|tags\) '` = **20**）；自检 ⑧d 钉住。

**可选型说明：为什么是「给 `_judge` 加第四档」，不是「另拆一个只报告子 scope」**

- 拆子 scope（如 `docs-report`）会让 D2 从 `--scope docs` 里消失 ⇒ 「docs = 5 步」这条既有契约被打散，
  默认集还要再加一个 scope 名；而 D2 的 `rc=2` 仍需按 BLOCKED 处理，子 scope 表达不了这件事。
- 第四档精确表达：**只动 rc=1 那一格的计账方式**，步骤本身（在 `docs` scope 里、照旧扫真目标、照旧打清单）一字不动。
- 命名选 `tri-report` 而非 `warn`：它明确是 `tri` 家族的一员（三格同 `tri`），`warn` 会掩盖它保留的 BLOCKED 语义。

**实现位置（一次说清，避免再找）**

| 东西 | 行 | 说明 |
|---|---|---|
| 文件头：退出码 / 四档 / 默认集 / docs 阻断面 / 第二波各 scope 段 | `19–82` | `-h\|--help` 打印的就是这一段（`sed -n '19,82p'`） |
| `DEFAULT_SCOPES` / `GATE_SCOPE`（默认集与「门自己的门」scope 名的**单来源**） | `85–91` | main() 与自检 ⑩ 共用这一串，写两份就会漂 |
| `STEP_MODE` 四档说明 | `97` 起 | 步骤表旁的口径注释 |
| **`_judge()`** | `140–181` | **唯一的判据出口**：`<模式> <rc> <日志>` ⇒ `PASS/FAIL/BLOCKED/REPORT`；`tri-report` 分支在其中 |
| `run_step()` | `123–135` | 调 `_judge` 取状态，立刻追加一行到 `results.tsv`（无子壳累加） |
| `count_fail` / `count_blocked` / **`count_report`** / `count_pass` | `198 / 202 / 206 / 210` | 四个数**都只从结果表 grep 出来** |
| **`_exit_rc()`** | `218–222` | **退出码的唯一出口**：FAIL⇒1 · 无 FAIL 而 BLOCKED⇒2 · 其余⇒0（REPORT 不参与；自检直接钉它） |
| `report()` | `244–267` | `REPORT` 行同 `FAIL/BLOCKED` 一样打日志路径 + 尾部原文（前缀 `·`）；末行状态计数含 `REPORT(只报告)` |
| 自检 ①–⑦ | `281–352` | 原有 14 条（empty 模式两侧 · 旧写法假绿对照 · 目录不存在硬红 · 无后缀脚本语法三步负控…） |
| 自检 **⑦′（第二波新增）** | `353–392` | 外围脚本语法的负控 5 条（坏件红 / 好件绿 / **排除面成对** / 空转红） |
| 自检 ⑧/⑧b/⑧c/⑧d/⑧e（三档） | `394–440` | 原有 14 条三档断言 |
| 自检 ⑨/⑨b/⑨c（第四档） | `441–481` | 只报告档 + **「D2 不影响退出码 / D1 影响」成对断言** |
| 自检 **⑩（第二波新增）** | `482–529` | **新挂三门的档位申报 + 0/1/2 三格各就各位**（22 条断言；见 §三） |
| `NOSUFFIX_CHECK_PY` / `nosuffix_syntax_cmd` | `545–600` | 无后缀脚本语法（单一来源，真步骤与自检 ⑦ 共用） |
| **`EXT_SYNTAX_PY` / `ext_syntax_cmd`（第二波新增）** | `602–668` | 四棵外围目录 + 顶层入口的脚本语法（单一来源，真步骤与自检 ⑦′ 共用） |
| 默认 scope 集 | `852` | `scopes=("${DEFAULT_SCOPES[@]}")`（常量在 `91` 行） |
| 出口判定 + `exit` | `927–950` / `969` | 步骤总数 / 通过 / 失败项数 / BLOCKED / **REPORT** → `_exit_rc` → rc |

脚本退出码因此仍是一句话：**`0` 全绿 · `1` 有失败项 · `2` 不给结论**（用法错 / 前置缺件 / 自检不过 / 有步骤报 BLOCKED）。
**第四档 REPORT 不进这三值** —— 它既不是「错」也不是「没结论」。

## 二、`docs` scope（文档面只读门禁 · **7 步**〔③批次 +2〕）+ 阻断面

> ★ **③批次（2026-09-18 收尾二）**：本 scope 追加 **2 步**（`glossary` · `i18n-drift`）⇒ **5 → 7 步**，
> 两条都用 **`tri-report`**（起步只报告）。下表第 6/7 行是新增；1–5 行**名字/命令串/模式一字未改**。

> **现状（2026-09-18 22:5x）**：这 5 步里 `meta`/`name`/`D1`/`D2` **已全绿**，只有 `D3` 仍是 **BLOCKED**（空转：main scope
> 语料里 0 条带锚链接）——`name`/`D1`/`D2` 是**并行的 docs 路**修的（本波没碰 docs 面任何文件）。
> 下面「为什么 D2 只报告」的理由段是**当时的存量债记录**（D2 曾红 69/235 条），档位是否收严仍属待拍。

```bash
cd "<repo>"
bash scripts/precommit-gates.sh --scope docs --list        # 看这 5 步
bash scripts/precommit-gates.sh --scope docs --outdir /tmp/zerg-gate-d/docs
```

| # | 步骤名 | 模式 | 命令（工作目录 = 仓根） | 阻断面 |
|---|---|---|---|---|
| 1 | `docs: meta --scope formal --missing=fail` | `tri` | `python3 scripts/check-doc-meta.py --scope formal --missing=fail` | **阻断** |
| 2 | `docs: name --scope repo` | `tri` | `python3 scripts/check-doc-name.py --scope repo` | **阻断** |
| 3 | `docs: freshness D1 引用路径存在` | `tri` | `python3 scripts/check-doc-freshness.py d1` | **阻断** |
| 4 | `docs: freshness D2 引用 文件:行 有效（只报告）` | **`tri-report`** | `python3 scripts/check-doc-freshness.py d2` | **只报告** |
| 5 | `docs: freshness D3 断链断锚` | `tri` | `python3 scripts/check-doc-freshness.py d3` | **阻断**（rc=2 维持 BLOCKED：不计失败项、也不当绿） |
| 6 | `docs: glossary 术语表 zh+en（只报告）` | **`tri-report`** | `python3 scripts/check-glossary.py` | **只报告**（③批次新增；现跑 rc=1 = T2 术语档 3 处 ⇒ 正好落 REPORT 那一格） |
| 7 | `docs: i18n-drift 双语漂移（只报告）` | **`tri-report`** | `bash scripts/check-i18n-drift.sh` | **只报告**（③批次新增；★ 现跑 **rc=2** ⇒ 落 **BLOCKED**：它把 rc=2 当「阻塞相失败」，与三档语义错位 ⇒ 见 §七 待拍 ⑧） |

**为什么单独把 D2 降成只报告（拍板①，落地 = 步骤模式）**：D2 今天**本地红 69 条**（门脚本自报计数跨域红 235：
文件不存在 162 · 无此文件 60 · 行号越界 13），几乎全是历史稿件里的陈旧 `文件:行` 引用（`docs/INDEX.md:14`、
`ui/src/app.rs:3136-3164` 这类）⇒ 属**存量债**，直接阻断等于把提交闸锁死。
★ **这是改判定档位，不是放宽 D2 的判据** —— `check-doc-freshness.py` 的判据与退码一字未改（本批只动
`precommit-gates.{sh,md}` 两个文件）；待存量债清到可接受，把该步模式改回 `tri` 即恢复阻断（一行）。

**三条硬规矩（挂接时特意守住的，本次未变）**

- **ⓐ 不进尾部软检查位**：这 5 步是 `build_steps()` 里 `docs)` 分支的一等步骤，rc 由 `_judge` 取真退出码。
  脚本尾部「`|| echo …` 报告不阻断」的位置是 2026-09-17「报红却退 0」事故的同款，新门禁一律不进。
- **ⓑ 判定不接管道**：命令串里没有 `| tee` / `| tail` / `| head`，所以 `$?` 就是门脚本自己的退出码。
  （第四档也没有绕过这条：`_judge` 拿到的仍是真退出码。）
- **ⓒ 现有 20 步不动**：加 scope、改 D2 档位都是**增量**改动；老 20 步在本轮真跑里 **20/20 PASS**。
- **每只门脚本自带 `--self-test`，本 scope 不传 `--no-self-test`**：先自证「会红」，再扫真目标。
- **D4 不挂**：`check-doc-freshness.py d4`（生成式参考 drift）要临时重建 + 逐字节比对，是**发布面**的事。

## 二之二、第二波补的三类步骤 + `gates` scope（「门自己的门」）

**增量不变量**：老 25 步（go 12 · rust 3 · pub 4 · tags 1 · docs 5）的名字 / 命令串 / 模式 / 工作目录**一字未改** +
**一条都没删**；本波只**追加 12 步**（go +5 · rust +3 · pub +1 · 新 scope `gates` +3）⇒ 默认全量 25 → **37 步**。
命令串一律**不接管道**（自检 ⑩ 有一条断言直接钉 `grep -c '|'` = 0）。逐条实测数字见下（时点 = 2026-09-18 22:4x–23:0x）。

### ⓐ `ui/` 三个 cargo 步（scope `rust` · 模式 `rc` · 工作目录 `ui/`，与 wall 三步逐行对齐）

| 步骤 | 命令（工作目录 = `ui/`） | 实测（2026-09-18 22:4x–22:5x · cargo/rustc 1.98.1） |
|---|---|---|
| `ui: cargo fmt --check` | `cargo fmt --check` | **rc=1 ⇒ FAIL**（全树未 rustfmt 过：483 行 diff——`build.rs` / `src/api.rs` / `src/modules/*`…） |
| `ui: cargo clippy -- -D warnings` | `cargo clippy -- -D warnings` | **rc=101 ⇒ FAIL**（43 条：`dead_code` / `unused import` / `deprecated` 等） |
| `ui: cargo test` | `cargo test` | **rc=0 ⇒ PASS**（97 passed / 0 failed） |

**为什么原来没有**：`--scope rust` 只 `add_step wall/`（债务台账 §0 第 1 行），而 `precheck` 只验 `ui/Cargo.toml`
**在不在**、不验它**过不过** ⇒ **ui 可以编译失败而闸全绿**（本波要治的形态本体）。
★ 三条**逐行对齐 wall 三步**（同命令串、同 `rc` 模式）⇒ 前两条的**真红照实进失败项**，**不放宽、不填白名单、不缝**；
要不要按 D2 先例降档，见 §待拍 ①（含「clippy 的 rc=101 属异常码 ⇒ `tri-report` 对它无效」这条机制事实）。
★ `ui/.cargo/config.toml` 的 `RUST_TEST_THREADS=1`（UI 有全局状态型用例）是仓内既有设置，本波没动。

### ⓑ 两个 Go module（scope `go` · 模式 `rc`）

| 步骤 | 命令（工作目录） | 实测 |
|---|---|---|
| `shared: build ./...` | `go build -buildvcs=false ./...`（`shared/`） | rc=0 · 0s |
| `shared: go vet ./...` | `go vet ./...`（`shared/`） | rc=0 · 0s |
| `shared: go test ./... -count=1` | `go test ./... -count=1`（`shared/`） | rc=0 · 1s（`ok …/shared/resources`） |
| `scripts/exportnames: build ./...` | `go build -buildvcs=false -o /dev/null ./...`（`scripts/exportnames/`） | rc=0 · 0s |
| `scripts/exportnames: go vet ./...` | `go vet ./...`（`scripts/exportnames/`） | rc=0 · 0s |

- **`exportnames` 不加 test 步**：实读 `find scripts/exportnames -name '*_test.go'` = **0 条**
  ⇒ 按任务书「有测试就加」**不加**（加了只是 `no test files` 的空转步，不缝）。
- **`exportnames` 那一步必须带 `-o`**：它是 **main 包** ⇒ `go build ./...` 会把二进制**写进当前目录**
  （实测当场冒出 7 MB `scripts/exportnames/zerg-exportnames`，未跟踪散件，已删）。`-o /dev/null` 实测 rc=0 且
  **仓内零新文件**；`-o` 与其值必须在包模式 `./...` **之前**（go 的旗标解析遇到第一个非旗标参数就停）。
- 两个 module 的 `gofmt -l .` 实测**零命中**（干净）—— 本波按任务书只加 build/vet(+test)，
  **没顺手加 gofmt 步**（那属扩面，见 §待拍 ③）。

### ⓒ 四棵外围目录 + 顶层入口的脚本语法（scope `pub` · 模式 `rc` · 一条步里遍历）

| 步骤 | 命令（工作目录 = 仓根） | 实测 |
|---|---|---|
| `mcp/gateway/publish/tools + 顶层入口 脚本语法（ast.parse / bash -n）` | `$(ext_syntax_cmd .)`（**单来源**：`EXT_SYNTAX_PY`） | rc=0 · 1s · **32 个被检查到** |

- 判法：遍历 `mcp/`(9) · `gateway/`(9) · `publish/`(8) · `tools/`(4) 的 `.py`（`ast.parse`）/`.sh`（`bash -n`）
  ＋ 顶层两个运行入口 `start-zerg-core.sh` / `start-zerg-ui.sh`（`bash -n`，**缺件即红**）。
- 排除 `vendor` / `venv` / `node_modules` / `target` / `dist` / `__pycache__` / `.venv` …（前缀 `tools/ocr/venv`）。
- **零命中必红**（空转 = 假覆盖）；自检 ⑦′ 用**同一串命令**做负控：坏件红 / 好件绿 / 空转红 ＋
  **排除面成对**（同一个坏件放在被排除的 `tools/ocr/venv/` 里 ⇒ 仍绿；挪到 `mcp/` ⇒ 立刻红）——
  「绿」不能靠「压根没扫到」蒙过去。
- 本波**只收任务书点名的四棵 + 两个顶层入口**；债务台账 §0 第 3 行还列了 `ui/`(2) · `zerg-evals/`(2) ·
  `deploy/`(1) · `core/`(1) ⇒ 见 §待拍 ②。

### `gates` scope —— 门自己的门（新 scope · **进默认集**）

| # | 步骤名 | 模式 | 命令（工作目录 = 仓根） | 档位 |
|---|---|---|---|---|
| 1 | `门① 覆盖：构建清单目录 + 脚本接线（阻断）` | `tri` | `python3 scripts/check-gate-coverage.py` | **阻断** |
| 2 | `门② 版本源：四处同版（阻断）` | `tri` | `python3 scripts/check-version-sources.py` | **阻断** |
| 3 | `门③ 接线：scripts 门脚本有没有被闸调用（阻断）` | **`tri`** | `python3 scripts/check-wired-scripts.py --strict-report` | **阻断**（③批次**升档**：升档前是 `tri-report` + 不带开关） |

- 三条都**进默认集**（`DEFAULT_SCOPES` 常量，`main()` 与自检 ⑩ 共用同一来源）；三条都**自带 `--self-test`**，
  本 scope **不传 `--no-self-test`**（先自证「会红」再扫真目标）。
- **不接管道**（自检 ⑩ 断言命令串里 0 个竖线）、**不进尾部软检查位**（是 `build_steps()` 里 `gates)` 分支的一等步骤，
  rc 由 `_judge` 取真退出码）。
- **门③ 的档位史（③批次升档）**：它起初按自己的建议**只报告**（首跑如实报「A 命中 8」= 存量债，阻断等于锁死提交闸）；
  ③批次把这 8 只**逐只上岗** ⇒ A 命中 0 · B 未登记 0 ⇒ 升成 **`tri` + `--strict-report`**（阻断）。
  ★ **两处必须一起改**：模式 `tri-report`→`tri` **且**命令串加 `--strict-report` ——
  脚本自身默认仍是「只报告 rc=0」，**漏掉那个开关就等于挂了个恒绿的步**（自检 ⑩ 组直接钉住这一点）。
  ★ 升档过程与「同一夹具同命中、只换开关 ⇒ rc 0/1」的成对证据见 `scripts/check-wired-scripts.md` §五之一。
- ★ 本波**没动那三只门脚本**（只往闸的步骤表里加调用）——它们不在本路允许改动面内；因此它们说明书里
  「本波不挂 / 第二波由父代理统一挂进」那句已过期，见 §待拍 ⑤。
- 挂上后**门① 的 A 段回绿**（`ui` / `shared` / `scripts/exportnames` 三个目录第一次被某步收进）：见 §四 ①。

## 三、自检：**79 条断言**（`--self-test`）

`bash scripts/precommit-gates.sh --self-test` ⇒ 全过才允许跑真目标。**79 条** = 路 D 的 41 条（①–⑨c）
＋ 第二波新增 **27 条**（**⑦′ 5 条** + **⑩ 22 条**）＋ ③批次新增 **11 条**（**⑩ +2**〔门③ 升阻断：档位改判 +
`--strict-report` 开关钉死〕+ **⑪ 9 条**），实测 `--self-test` rc=0、断言失败 0 条（`✓` 行计数 = 79）。

| 组 | 断言 | 期望 |
|---|---|---|
| ①–⑦ | 全过计 0 · 一条真红计 1 且点名 · 一步一日志 · empty 模式两侧 · 旧写法假绿对照 · 目录不存在硬红 · 无后缀脚本语法三步负控 | 原有 |
| **⑦′（第二波新增）** | 外围脚本语法（`ext_syntax_cmd` **同一串命令**）：坏 `.py`/`.sh` 件 ⇒ 必红 · 好件 ⇒ 必绿 · **排除面成对**（同一坏件在 `tools/ocr/venv/` 里 ⇒ 绿，挪到 `mcp/` ⇒ 红）· 0 个被检查到 ⇒ 必红 | 5 条 |
| ⑧/⑧b/⑧c/⑧d/⑧e | `tri` 三格各就各位 · 异常码按红 · 只有 BLOCKED ⇒ 失败项 0 · **rc 模式遇 rc=2 仍是失败项（老语义）** · 模式名打错硬红 | 原有 |
| **⑨** | `tri-report` 遇 rc=1 ⇒ 状态 `REPORT`（不是 FAIL）；两步 rc 都是 1（区分度来自模式）· 失败项数只数阻断那步 · REPORT 数=1 · **不计 BLOCKED** · REPORT 原文仍在自己日志里 | 第四档格位 |
| **⑨b** | **成对断言（本批硬要求）**：只有「只报告」红 ⇒ `_exit_rc` = **0（不影响退出码）**；只有「阻断」红 ⇒ `_exit_rc` = **1（影响退出码）**。同一组 rc=1、只换模式 ⇒ **有区分度** | D2 不影响 / D1 影响 |
| **⑨c** | 只报告档的另外三格与 `tri` 一致：rc=2 仍落 `BLOCKED`（不许当绿）· 异常码仍计失败项 · 该组退出码仍是 1（失败优先） | 第四档边界 |
| **⑩（第二波新增）** | **新挂三门**（镜子取步骤表里那三步**真申报的模式**）：① 结构性 6 条 —— 步数 3 · 档位申报逐字 `tri tri tri-report ` · 三条命令分别点名三只新门脚本 · 命令串 **0 个竖线**（不接管道）· 工作目录都 = 仓根 · `gates` **在 `DEFAULT_SCOPES` 里**；② 逐档 15 条 —— 门①/门②/门③ 各跑 rc=0/1/2 三格：`PASS` · rc=1 ⇒（阻断）失败项+1 且退出码 1 /（只报告）`REPORT`+1 且**不计失败项、退出码仍 0** · rc=2 ⇒ `BLOCKED`+1、**不计失败项**、退出码 2（不许当绿）；③ 成对 1 条 —— 同一串 rc=1：门① ⇒ 退出码 1 · 门③ ⇒ `REPORT` 1 且失败项 0 | **22 条** · 档位申报被 pinned（谁把门③改成阻断、或把门①改成只报告，立刻红） |
| **⑩ 的 ③批次修订（+2 条）** | ① 档位申报逐字改为 **`tri tri tri`**（门③ 升阻断：谁改回只报告立刻红）；② **门③ 命令串必须含 `--strict-report`**（漏了 = 挂了个恒绿步 ⇒ 红）。成对断言改为**跨档**：门③（阻断）⇒ 退出码 1 · ⑨b 那步（只报告）⇒ `REPORT` 1 且退出码 0（区分度仍由 ⑨/⑨b/⑨c 的合成成对扛） | **+2 条**（22 → 24） |
| **⑪（③批次新增）** | **本批新挂 6 步的挂接纪律**（镜子取 `docs/pub/tools/slice` 四个 scope 的**真步骤表**）：① 步数 16（docs 7 + pub 7 + tools 1 + slice 1）；② 四只门脚本按名出现在命令串里（glossary · i18n-drift · tool-version-sync · slice）；③ 两只自称门脚本的自检按名上榜（edit-assert · mutate-scan）；④ 本批 6 步被「点名命令」选中（不多不少）+ 它们的档位只在 `tri`/`tri-report` 两档；⑤ 命令串里 **0 个单竖线管道**（不接管道取 rc；`\|\|` 不算管道 —— 现存 pub 那条 `for … \|\| exit 1` 步就是靠这条才不会被误判）；⑥ 工作目录全部 = 仓根；⑦ 档位申报都在表内（`rc`/`empty`/`tri`/`tri-report`）；⑧ **四档计数之和 = 步数**（跑一遍四档合成步骤 ⇒ 「挂上了但没记账」不可能通过） | **9 条** |

**变异验证（证明这 13 条镜子有牙齿 —— 在 `/tmp` 副本上做，不碰仓内脚本）**

| 变异 | 注入（最小语义变异） | 结果 |
|---|---|---|
| ① | `tri-report` 里 `1) printf 'REPORT' ;;` → `'FAIL'`（只报告退化成阻断） | 自检 rc=2 · **断言失败 6 条**（含 ⑨b 退出码 `实际=1 期望=0`） |
| ② | 整段删掉 `_judge` 的 `tri-report)` 分支（第四档不存在 ⇒ 命中「未知模式名硬红」） | 自检 rc=2 · **断言失败 9 条**（⑨ 6 条 + ⑨c 3 条） |

两次变异都**先落 `/tmp` 副本 + `bash -n` 过了才跑**；恢复不需要动作（仓内文件从未被变异）。

## 四、真跑数字（本机 · **2026-09-18 22:58–23:01 那跑 = 本波** · 数字带时点）

### ① 默认全量（`go rust pub tags docs gates` ⇒ **37 步**）

命令：`bash scripts/precommit-gates.sh --outdir /tmp/zerg-gates-w2`（挂钟 = `date +%s` 前后差；**不接管道取 rc**）。

| 步骤总数 | 通过 | 失败项数 | BLOCKED | **REPORT（只报告）** | 脚本 rc | 挂钟 |
|---|---|---|---|---|---|---|
| **37** | **34** | **2** | **1** | **0** | **1** | **144s** |

汇总行（脚本原文）：`步骤总数: 37 · 通过: 34 · 失败项数: 2 · 不给结论(BLOCKED): 1 · 只报告(REPORT): 0` ⇒ **rc=1**。

- **非 PASS 的只有 3 步**（逐条原文摘要）：

| 步骤 | 状态 | rc | 原文摘要 |
|---|---|---|---|
| `ui: cargo fmt --check` | **FAIL** | 1 | 全树未 rustfmt 过（483 行 diff） |
| `ui: cargo clippy -- -D warnings` | **FAIL** | 101 | `error: could not compile `zerg-ui` (bin "zerg-ui") due to 43 previous errors` |
| `docs: freshness D3 断链断锚` | **BLOCKED** | **2** | 空转（不给结论 —— **不计失败项**，也不许当绿） |

- **老 25 步逐条**：go 12 全 PASS · wall 3 全 PASS · pub 4 全 PASS · tags 1 PASS ·
  docs 5 = `meta`/`name`/`D1`/`D2` **全 PASS** + **`D3` BLOCKED**。
  ★ 与路 D 那跑（25 步 · 通过 21 · 失败项 2 = `name`/`D1` · BLOCKED 1 · REPORT 1 = `D2` · rc 1 · 108.78s）相比：
  docs 面的 `name` / `D1` / `D2` 现在全绿 —— 那是**并行的 docs 路**修的（本波**没碰 docs 面任何文件**），
  所以「本波对既有门禁零回归」这句话在这里是**逐条可比**的，不是印象。
- **本波 12 步逐条**：`shared` 3 PASS · `exportnames` 2 PASS · `ui` 3 = **fmt FAIL / clippy FAIL / test PASS** ·
  pub 外围语法 1 PASS · **`gates` 3 全 PASS**：
  门①（rc=0 · `A 未收进：0` · `B 棘轮命中 69 / 基线 69`）· 门②（rc=0 · `结果: OK（全绿）`）·
  门③（rc=0 · 只报告档 · `【结论】A 命中 8 · B 未登记 0 · 配置错 0`）。
- **红灯的 2 个失败项全是 ui 的存量债**（本波按 wall 逐行对齐把它照实挂上，没为绿放宽任何判据）；
  BLOCKED 那 1 步是 docs 侧的空转（与路 D 时点一致，非本波引入）。

### ② 三门进默认集的挂钟增量（本波唯一被要求「先贴数字再定」的那一项）

| 口径 | 三门合计 | 门① | 门② | 门③ |
|---|---|---|---|---|
| 本次**全量跑内**（`results.tsv` 逐步骤耗时） | **+23s** | 2s | 2s | 19s |
| **单跑**（直接 `python3 scripts/…` 计时 · 本机） | **+8s** | 1.6s | 0.15s | 6.1s |

⇒ 两个口径**都 < 30s**（阈值内）。门③ 在全量跑里明显更慢（19s vs 6s：它的扫描面大、又与前面几步的 I/O 叠在一起）—— 两个数并列给出，不挑好看的报。
本波 **12 步合计 ≈ +31s**（其中 ui 三步 ≈ +7s **暖跑**；冷跑实测 `cargo test` **121s** · `cargo clippy` **57s**，
即「换机/清 target 后第一次跑全量」会明显更久，这也如实记在案）。

### ③ 逐 scope 步数自证（`--list`）

| scope | 步数 | 变化 |
|---|---|---|
| go | **17** | 12 + **5**（shared 3 · exportnames 2） |
| rust | **6** | 3 + **3**（ui） |
| pub | **5** | 4 + **1**（外围脚本语法） |
| tags | 1 | 不变 |
| docs | 5 | 不变 |
| **gates** | **3** | **新增**（门①②③） |
| **默认全量** | **37** | **25 + 12** |

### ④ 自检（`--self-test` ⇒ rc=0）

| 项 | 实测 |
|---|---|
| `bash scripts/precommit-gates.sh --self-test` | **rc=0** · 断言 **68 条全过**（`✓` 行计数 = 68）· 失败 0 条　★ ③批次后为 **79 条**（见 §七 ④） |
| **⑦′**（外围语法负控 · 新增 5 条） | 坏件 ⇒ 红 · 好件 ⇒ 绿 · **排除面成对**（同一坏件在 `tools/ocr/venv/` ⇒ 绿；挪到 `mcp/` ⇒ 红）· 0 个被检查到 ⇒ 红 |
| **⑩**（新门三门 · 第二波 22 条 + ③批次 +2 条 = **24 条**） | 档位申报逐字 **`tri tri tri`**（门③ 已升阻断）+ **门③ 命令串必带 `--strict-report`** + 三档三格各就各位 + 跨档成对断言「同一串 rc=1：阻断（门③）⇒ 退出码 1 · 只报告（⑨b 那步）⇒ REPORT 1 且退出码 0」 |
| **⑪**（③批次新增 · 9 条） | 本批 6 步的挂接纪律：步数 16 · 四只门脚本+两只自称门脚本按名上榜 · **0 个单竖线管道** · 工作目录全 = 仓根 · 档位都在表内且新步只用 `tri`/`tri-report` · **四档计数之和 = 步数** |

★ 两组的每条断言都走**真命令行 + 真退出码**（合成步骤），**不碰真目标**；闸本身的 `_judge`/`_exit_rc` 语义由路 D 那批 ⑧/⑨ 覆盖（§三 表）。

### 历史（路 D 那跑 · 25 步 · 2026-09-18 那两个时点）—— 原样保留，别当现状读

#### 史-① 三种跑法（路 D 时点 · 25 步）

| 跑法 | 步骤总数 | 通过 | 失败项数 | BLOCKED | **REPORT（只报告）** | 脚本 rc | 挂钟 |
|---|---|---|---|---|---|---|---|
| `--scope docs` | 5 | 1 | **2** | 1 | **1** | **1** | **3.70s** |
| 默认全量（`go rust pub tags docs`） | **25** | 21 | **2** | 1 | **1** | **1** | **108.78s**（25 步逐步耗时合计 107s） |
| 用法 `-h`（无 `--version` 这个参数） | 0（不跑步骤、无计数） | — | — | — | — | **0** | **0.63s** |

- **默认全量不是 0，如实报出**：因为 **D1 与 name 真红（rc=1）+ D3 报 BLOCKED（rc=2）** ——
  **没有为绿放宽任何判据**；D2（只报告）已从失败项里退出（详见下条）。
- 老 20 步在本轮真跑里 **20/20 PASS**（`go 12 · rust 3 · pub 4 · tags 1`）⇒ 本次改动对既有门禁**零回归**。
- 25 步里非 PASS 的只有 4 步：`name`(FAIL) · `D1`(FAIL) · `D2`(**REPORT**) · `D3`(BLOCKED)。
- 默认 `--list` ⇒ 「共 **25** 步」；`--scope docs --list` ⇒ 「共 **5** 步」（D2 行显示模式 `tri-report`）。

#### 史-② `--scope docs` 逐步状态（路 D 时点；`name`/`D1`/`D2` 现已由 docs 路修绿）

| 步骤 | 状态 | rc | 在跑的那件事（原文摘要） |
|---|---|---|---|
| `docs: meta --scope formal --missing=fail` | **PASS** | 0 | `OK：合规 19 篇（告警 2 条不阻断）`（M16 告警 2：type 与目录默认值不一致） |
| `docs: name --scope repo` | **FAIL** | 1 | FAIL：**1 个不合规命中**（另有 N5/N6/N7 告警与豁免计数） |
| `docs: freshness D1 引用路径存在` | **FAIL** | 1 | 计数：相对引用 399 · 过 381 · **断链=9**（命中域：外部 2 · 本地 18）⇒ **本地红 7 条** |
| `docs: freshness D2 引用 文件:行 有效（只报告）` | **REPORT** | 1 | 红 235 / 黄 53（跨域计数）；**本地红 69**（打印 40 + 「还有 29 条红」）· 本地黄 49；外部引用域命中 170 单列不进退码 |
| `docs: freshness D3 断链断锚` | **BLOCKED** | **2** | **空转**：main scope 语料 295 篇里 0 条带锚链接 ⇒ 不给结论 |

汇总行：`步骤总数: 5 · 通过: 1 · 失败项数: 2 · 不给结论(BLOCKED): 1 · 只报告(REPORT): 1` ⇒ rc=1。

#### 史-③ D2 只报告：前后对照 + 真目标成对探针

**前后对照（同一批 25 步、口径相同）**：路 D 那次（D2 走 `tri`）＝ 通过 21 · **失败项数 3** · BLOCKED 1 · rc=1；
本批 ＝ 通过 21 · **失败项数 2** · BLOCKED 1 · **REPORT 1** · rc=1 ⇒ **签名正是「D2 从失败项退出、改入只报告」**。

**真目标成对探针：同一串红、只换模式 ⇒ rc = 0 / 1**

默认 5 步/25 步跑法里 D1 与 name 同时红 ⇒ rc 无论如何都是 1，**D2 的影响在那两个跑法里观察不到**；
所以另做一对**真目标**探针（同一串真命令 `python3 scripts/check-doc-freshness.py d2` 扫**真 docs 树**，步骤表只留 D2，
两份副本**只差该步的判定模式**）：

| 探针 | D2 的模式 | D2 那步自报 | 步骤总数 / 通过 / 失败项 / BLOCKED / REPORT | 脚本 rc | 挂钟 |
|---|---|---|---|---|---|
| A | `tri-report` | **rc=1 真红**（打印 40 条 + 「还有 41 条红」） | 1 / 0 / **0** / 0 / **1** | **0**（门禁全绿） | 1.46s |
| B | `tri` | 同上（同命令、同树） | 1 / 0 / **1** / 0 / 0 | **1**（门禁红灯） | 1.45s |

⇒ **真目标上、同一串红、只换模式：只报告 ⇒ rc=0、阻断 ⇒ rc=1** —— 与自检 ⑨b 的合成成对断言互为印证。
探针实现：`/tmp/r1-gates/probe-D2-{A,B}.sh` = 仓内脚本的**副本**，只做两处改动 —— ① 钉住 `REPO_ROOT`
（副本在 `/tmp`，否则 `precheck` 必失败）；② 步骤表只留 D2（B 再把该步模式改成 `tri`）。
**仓内脚本与真步骤表未被改动**（两份副本只在 `/tmp`）。
（D2 本地红数在两个时点间由 69 → 81，正是别的路正在改 docs 的痕迹 —— 本表数字都带时点。）

#### 史-④ D1 当时红 7 条（路 D 时点 · 逐条清单；**现已全绿**）

| # | 位置 | 目标 | 类别 |
|---|---|---|---|
| 1 | `docs/index-nav.md:29` | `00-总览/` | 红·断链 |
| 2 | `docs/index-nav.md:32` | `03-计划/` | 红·断链 |
| 3 | `docs/index-nav.md:33` | `04-实施/` | 红·断链 |
| 4 | `docs/index-nav.md:34` | `05-问题/` | 红·断链 |
| 5 | `docs/index-nav.md:35` | `06-版本/` | 红·断链 |
| 6 | `docs/site/nav.md:178` | `变更-v2.5.9.md` | 红·断链 |
| 7 | `docs/site/nav.md:179` | `../../01-设计/设计-文件浏览器虫茧-20260913.md` | 红·断链 |

另有 **2 条外部引用域断链**（D1 计数里的「外部 2」）—— 非本地域默认不进退码（`--strict-all` 才判）。
这 7 条**一条都没被放宽**：D1 仍是 `tri`，仍把 rc 拉成 1。

#### 史-⑤ `docs` 进默认集时每次提交多花多久（路 D 的逐命令计时）

`docs` scope 整体 **≈3.5s**（本次实跑 3.70s，含各门 `--self-test` ≈1.3s + 总控自检 ≈0.35s + 5 个门脚本净耗时 ≈2.6s）
⇒ `docs` 进默认集后，每次提交多花 **≈+2.6s 净 / ≈+3.5s 含脚手架**，相对 ≈105s 的全量基线 ≈ **+3%**。

## 五、默认 scope 集

```bash
# 脚本常量（`DEFAULT_SCOPES` / `GATE_SCOPE`）· main() 只引用常量，不写第二份：
DEFAULT_SCOPES=(go rust pub tags docs gates tools slice)   # ③批次：+tools +slice（都进默认集）
GATE_SCOPE="gates"
```

⇒ `bash scripts/precommit-gates.sh`（不带 `--scope`）现在跑 **45 步**：
老 25 步（go 12 · rust 3 · pub 4 · tags 1 · docs 5）+ 第二波 12 步（go +5 · rust +3 · pub +1 · **gates 3**）
+ 收尾 2 步（go 的 shared/exportnames `gofmt -l`）+ **③批次 6 步**（docs +2 · pub +2 · **tools 1** · **slice 1**）。
其中**只报告档 4 步**：`docs` 的 D2 · `docs` 的 glossary · `docs` 的 i18n-drift · `slice` 的探针步
（rc=1 只入清单、不进退出码；**rc=2 仍是 BLOCKED**）。
- 若要把 D2 恢复阻断：把该步模式从 `tri-report` 改回 `tri`（一行，且要先把存量债清掉）。
- 门③ **已升阻断**（③批次）：要临时只报告就摘掉 `--strict-report`（**别忘改回来**，自检 ⑩ 会红）。
- `--scope gates --list` ⇒ 「共 3 步」（门③ 显示模式 `tri`）· `--scope docs --list` ⇒ 「共 7 步」·
  `--scope tools --list` ⇒ 「共 1 步」· `--scope slice --list` ⇒ 「共 1 步」。

## 六、本路（第二波）声明

- **只改两个文件**：`scripts/precommit-gates.sh`（+ 本说明书同步更新）。**未碰** `docs/`、`core/`、`agent/`、
  `ui/`、`wall/`、`shared/`、`publish/`、`mcp/`、`gateway/`、`tools/` 任何文件；**未碰**任何门脚本
  （`check-*.py` 的判据一字未改 —— 本波只是**调用**它们）。
- **未做任何 git 操作**（未 `add` / 未 `commit`；入库由父代理统一做）。
- **未删步骤**：老 25 步的名字 / 命令串 / 模式 / 工作目录**一字未改、一条未删**
  （逐条比对：本轮把老 25 步的 `(scope, 模式, 名)` 拿 `--scope <老 scope> --list` 的输出现读对账，全中；
  `git diff -U0 -- scripts/precommit-gates.sh` 里**没有被本波删掉的 `add_step` 行**）。
  现在的 scope 步数是**加过的**：go **17** · rust **6** · pub **5** · tags **1** · docs **5** · gates **3**（见 §四 ③）。
- **不接管道取 rc**：本波新增的 12 条命令串里 **0 个竖线**（自检 ⑩ 直接钉住），也没有任何一步落在脚本尾部软检查位。
- **没为凑绿填白名单 / 没放宽判据**：门① 的白名单一条没加（`whitelist.scripts` 仍空）；ui 的 fmt/clippy 真红照实进失败项。
- **只改了 `scripts/gate-coverage.config.json` 的 `baseline` 一个数字**（74 → 69，附自述更新）+ 该文件里原来的两个白名单条目**未动**。
  ★ 该文件不在「只许改两个文件」的字面清单里，但它就是门① 的**入参清单**、且任务书 §1 明写「把 baseline 改成新值」——
  除 baseline 与自述外**没有任何判据改动**（基线只许下调、永不上调）。
- **只在 `/tmp` 试跑**：日志与自检夹具在 `/tmp/zerg-gates-w2/` 与 `/tmp/zerg-gates-selftest-*`；
  未装任何全局包、未起后台常驻进程、**未删任何仓内文件**（唯一删掉的是自己跑出来的构建散件 `scripts/exportnames/zerg-exportnames`，
  未跟踪 · 非仓内既有件）。
- **未做（留给父代理/后继路）**：见 §待拍。

## 待拍（本波逐条列出的、不由本路决定的）

| # | 待拍项 | 现况（实测） | 本路已做的 | 需要拍的是什么 |
|---|---|---|---|---|
| ① | **`ui` 的 `fmt` / `clippy` 要不要降为只报告** | 两步今天**真红**：`cargo fmt --check` rc=1（483 行 diff）· `cargo clippy -- -D warnings` rc=101（43 条） | 按 wall 三步**逐行对齐**用 `rc`（阻断档）⇒ 默认全量**多两个失败项**，如实报 | 二选一：(a) 认下这两个红、排一批 UI 侧格式/告警清理；(b) 降档。★ **机制事实**：`tri-report` 只吃 rc=1 —— clippy 的 **rc=101 属「异常码 ⇒ FAIL」**（第四档不许把它洗成「只是报告」）⇒ 要降 clippy 的档必须另设形态（不是改一个词的事） |
| ② | **还有 4 棵目录的脚本没有语法门** | 债务台账 §0 第 3 行列全 9 处：`mcp(9) gateway(9) publish(8) tools(4)` ✓ 本波已收 + `ui(2) zerg-evals(2) deploy(1) core(1)` **仍未收** | 按任务书**只收点名的四棵 + 两个顶层入口**（不顺手扩面） | 要不要把剩下 4 棵（尤其 `core/*.sh`、`ui/scripts/*`）也并进同一步（改 `ROOTS` 一行） |
| ③ | **`shared/` / `scripts/exportnames/` 要不要加 `gofmt -l` 步** | 两处 `gofmt -l .` 实测**零命中**（现在加 = 立即绿、近乎零成本） | 任务书只点名 build + vet(+test) ⇒ **没加** | 要不要补齐（与 core/agent 的两条 `gofmt -l` 对齐） |
| ④ | **`precheck()` 要不要把两个新 module 的 `go.mod` 也列入前置** | precheck 现验 5 件（core/agent 的 go.mod · wall/ui 的 Cargo.toml · publish/whitelist.txt） | **没改** —— 改了会让 `scripts/check-gate-coverage.py` 里那句「precheck 要 …五件」的报错提示过期，而**那个文件不在本路允许改动面内** | 要么同时改两处（含门① 的提示文字，属另一路的文件面），要么维持现状（缺件时表现为 go 报错 ⇒ FAIL，而不是 BLOCKED） |
| ⑤ | **三只门的说明书里「本波不挂」已过期** | `check-gate-coverage.md` 等三份写「与既有门的接口（**本波不挂**；第二波由父代理统一挂进）」 | **没改**（不在本路允许改动面内） | 三份 .md 的属主路改一句「**已挂**：`gates` scope · `tri`/`tri-report`」 |
| ⑥ | **`ui/Cargo.lock` 被 cargo 顺带同步** | `zerg-ui` 版本 `2.5.9 → 2.5.10`；`ui/Cargo.lock` mtime = **22:48:53**，正落在本路第一次跑 `ui` 的 `cargo clippy`（22:4x）窗口内 ⇒ **本路跑 cargo 时 Cargo 自己写的**（`Cargo.toml` 早已是 2.5.10，锁文件落后一格）。与 `wall/Cargo.lock`（第一波那批 cargo 跑同步的，mtime 22:43:20）**同族 · 机械同步物** | 未回退、如实报出（`git status` 里它是 ` M`，不是本路手改） | 入库时把它算作机械同步（门② 明写不管锁文件）；若要回退，`git checkout -- ui/Cargo.lock` 会在下一次跑 ui 步时被再写一遍 |

## 七、③批次（2026-09-18 收尾二）：门③ 存量清偿 + 升阻断 + 两个新 scope

> 本节是 **③批次**的自查节。上面 §一–§六 是**路 D / 第二波 / 收尾一**的原文（一字未改，只有被 ★ 行点名的
> 计数/档位处同步）。本批**新挂 6 步**并**改 1 处既有步骤的档位**（门③）。

### ① 改了哪些步骤（逐条可验收）

| # | 步骤 | scope | 模式 | 命令（工作目录 = 仓根） | 首跑 rc（真目标 · 现跑） |
|---|---|---|---|---|---|
| 新 1 | `docs: glossary 术语表 zh+en（只报告）` | `docs` | `tri-report` | `python3 scripts/check-glossary.py` | **1** ⇒ `REPORT` |
| 新 2 | `docs: i18n-drift 双语漂移（只报告）` | `docs` | `tri-report` | `bash scripts/check-i18n-drift.sh` | **2** ⇒ **`BLOCKED`**（语义错位，见 ⑤） |
| 新 3 | `edit-assert --self-test（16 条成对负控）` | `pub` | `tri` | `python3 scripts/edit-assert --self-test` | **0** ⇒ `PASS` |
| 新 4 | `mutate-scan --self-test（26 条成对负控）` | `pub` | `tri` | `python3 scripts/mutate-scan --self-test` | **0** ⇒ `PASS` |
| 新 5 | `tools: 工具版本三处一致（台账/文档抬头/履历）` | **`tools`（新 scope）** | `tri` | `bash scripts/check-tool-version-sync.sh` | **0** ⇒ `PASS` |
| 新 6 | `slice: 切片合同探针集 + 混淆矩阵门槛（只报告）` | **`slice`（新 scope）** | `tri-report` | `python3 scripts/check-slice.py --probe` | **0** ⇒ `PASS` |
| 改 1 | `门③ 接线：scripts 门脚本有没有被闸调用（阻断）` | `gates` | **`tri`**（原 `tri-report`） | `python3 scripts/check-wired-scripts.py --strict-report`（**加开关**） | **0**（A 命中 **0**）⇒ `PASS`；命中即 `FAIL` |

- **两个新 scope 都进默认集**（`DEFAULT_SCOPES`，`main()` 与自检 ⑩/⑪ 共用同一来源）——
  新挂的步若不在默认集里跑，「上岗」就只是文件里的一行字。
- **硬要求逐条守住**：6 条命令串**无管道**（自检 ⑪ 直接钉 `grep -cE '(^|[^|])\|([^|]|$)'` = 0）·
  全部是 `build_steps()` 里的**一等步骤**（**不在**脚本尾部那个软检查位）· 工作目录一律仓根 ·
  6 步都进 `results.tsv` 计数（自检 ⑪ 的「四档计数之和 = 步数」那一条）。
- **发布闸侧另挂 2 步**（不进提交闸默认集，在 `scripts/publish-preflight.sh` 里）：
  **闸②b** `check-hardcoded-private-paths.py`（紧跟闸② 私有面）· **闸⑥** `check-manifest-freshness.py --strict`
  （发布制品清单新鲜度；缺件 ⇒ `BLOCKED`，见 ⑤）。两处的现跑与负控见 `scripts/check-wired-scripts.md` §五之一。

### ② 默认全量复跑（本机 · 2026-09-18 23:52–23:56）

命令：`bash scripts/precommit-gates.sh --outdir /tmp/zerg-gates-b3`（挂钟 = 前后 `date +%s` 差；**不接管道取 rc**）。

| 步骤总数 | 通过 | 失败项数 | BLOCKED | REPORT | 脚本 rc | 挂钟 |
|---|---|---|---|---|---|---|
| **45** | **42** | **0** | **2** | **1** | **2** | **233s** |

汇总行（脚本原文）：`步骤总数: 45 · 通过: 42 · 失败项数: 0 · 不给结论(BLOCKED): 2 · 只报告(REPORT): 1` ⇒ rc=2。

★ **末次复跑（同一批改动的**最终状态**）**：`--outdir /tmp/zerg-gates-final` ⇒
**45 · 通过 42 · 失败 0 · BLOCKED 2 · REPORT 1 · rc=2 · 挂钟 181s**（两组数字完全一致，只差挂钟 ——
第一次 233s 含冷一点的 cargo 目标树，第二次 181s 是暖跑）。逐 scope 从 `results.tsv` 分组数：
`go 19=PASS19 · rust 6=PASS6 · pub 7=PASS7 · tags 1=PASS1 · docs 7=PASS4/BLOCKED2/REPORT1 · gates 3=PASS3 ·
tools 1=PASS1 · slice 1=PASS1`。

**非 PASS 的 3 步（逐条原文摘要；**没有为绿放宽任何判据**）**

| 步骤 | 状态 | rc | 原文摘要 | 归因 |
|---|---|---|---|---|
| `docs: freshness D3 断链断锚` | **BLOCKED** | 2 | 空转（main scope 语料 0 条带锚链接） | **存量**（上一批就有；非本批引入） |
| `docs: glossary 术语表 zh+en（只报告）` | **REPORT** | 1 | T1 阻塞档 0 · **T2 术语档 3 处**（全在 `docs/zh/概念-虫卵与孵化.md`：正名「虫卵」）· T3 0 | 本批新挂；`rc=1` 正落「只报告」那一格 ⇒ **不进退出码** |
| `docs: i18n-drift 双语漂移（只报告）` | **BLOCKED** | 2 | 相① 缺 en 侧 **9 页** · 相② **1 篇**译页 `source_commit` 过期 · 相③ 缺注释 0 | 本批新挂；它的 rc=2 是「阻塞相失败」⇒ 与三档的「不给结论」语义错位（见 ⑤ ⑧），红照实进清单 |

**逐 scope 步数自证（`--list` 与单 scope 跑，现读）**

| scope | 步数 | 本批变化 | 单跑 PASS/FAIL/BLOCKED/REPORT | rc | 挂钟 |
|---|---|---|---|---|---|
| go | **19** | 不变（上一批 +2 gofmt） | — | — | — |
| rust | 6 | 不变 | — | — | — |
| pub | **7** | 4+1（外围语法）+ **2（本批：edit-assert · mutate-scan 各自检）** | — | — | — |
| tags | 1 | 不变 | — | — | — |
| docs | **7** | 5 + **2（本批：glossary · i18n-drift）** | **4/0/2/1** | **2** | 4s |
| gates | 3 | 不变（门③ 只改档位） | **3/0/0/0** | **0** | 10s |
| **tools** | **1** | **新增**（工具版本三处一致） | **1/0/0/0** | **0** | 2s |
| **slice** | **1** | **新增**（切片合同探针集） | **1/0/0/0** | **0** | 1s |
| **默认全量** | **45** | 39 + **6** | **42/0/2/1** | **2** | **233s** |

**成本（本批 6 步在默认全量里的逐步骤耗时，现读 `results.tsv`）**：
edit-assert **5s** · mutate-scan **24s** · glossary 0s · i18n-drift 0s · tools 0s · slice 0s ⇒ **合计 ≈29s**
（相对上一批 39 步 ≈190s 的基线，本批的真实增量主要在 mutate-scan 自检那 24s）。

### ③ 门③ 真目标复跑（升阻断后）

    python3 scripts/check-wired-scripts.py --strict-report   ⇒ rc=0
      【结论】A 命中 0 · B 未登记 0 · 配置错 0（已挂门脚本 25 只）
    python3 scripts/check-wired-scripts.py --self-test        ⇒ rc=0 · 16/16 通过

（升档前的首跑是「A 命中 8 · B 未登记 0」；前后对照与「同一夹具同命中、只换开关 ⇒ rc 0/1」的成对证据
见 `scripts/check-wired-scripts.md` §五之一。）

### ④ 自检（`--self-test` ⇒ rc=0）

| 项 | 实测 |
|---|---|
| `bash scripts/precommit-gates.sh --self-test` | **rc=0** · 断言 **79 条全过**（`✓` 行计数 = 79）· 失败 0 条 |
| **⑩ 的 ③批次 +2 条** | 档位申报逐字 **`tri tri tri`**（门③ 升阻断）+ **门③ 命令串含 `--strict-report`**（漏了 ⇒ 红）+ 跨档成对 |
| **⑪（新增 9 条）** | 本批 6 步的挂接纪律：步数 16 · 点名脚本上榜 · **0 个单竖线管道** · 工作目录 = 仓根 · 档位在表内且新步只用 `tri`/`tri-report` · **四档计数之和 = 步数** |

### ⑤ 本批待拍（不由本批决定 / 已查实但需一句话令）

| # | 待拍项 | 现况（实测） | 本批已做的 | 需要拍的是什么 |
|---|---|---|---|---|
| ⑦ | **发布闸 闸⑥ 的「缺件 ⇒ BLOCKED」要不要照此长期化** | 真镜像树 `$D` 没有同一版的 `manifest.json`（找过 `$D/manifest.json` · `dist/<ver>/release/manifest.json` · `dist/<ver>/manifest.json`，`<ver>` 取自被推的那棵树）⇒ 现在会 `BLOCKED`（rc=2 ⇒ **不许推**） | 按「缺件 ≠ 绿」实现（**不是**静默跳过、也不是 FAIL）；正/负控夹具已验证（缺件 ⇒ BLOCKED · 新鲜同源 ⇒ rc=0 · `dirty=true` ⇒ rc=1） | 二选一：**(a)** 推之前先 `scripts/pack-release.sh` 出同版清单（本闸按现形态长期化）；**(b)** 给 `publish-preflight.sh` 传第二个参数（制品目录）—— 后者改的是用法，不在本批允许改动面内 |
| ⑧ | **`check-i18n-drift.sh` 的 rc=2 语义错位** | 它的三档是 `0` 干净 / `1` 告警（相③）/ **`2` 阻塞相失败**；而仓内三档把 `2` 定义为**「不给结论」** ⇒ 它今天那 9 页缺 en 侧 + 1 篇过期**落成 `BLOCKED`（不计失败项）**，真红被记成「没结论」 | 按门③ 的现读建议挂上（`docs` scope · `tri-report`），**一个字没改它**；红照实进清单（报告里打 `~` 前缀与日志路径） | 二选一：**(a)** 给它加一档退码（如「阻塞失败」用 `3`，或 `1` 表示阻塞、告警另用别的码）——改的是门脚本本体；**(b)** 接受「缺 en 侧 ⇒ BLOCKED」这个现状并在 docs 面记账。**在拍下之前，默认全量的 rc=2 里会一直带着这一格** |
| ⑨ | **`slice` 步要不要升 `tri`** | `check-slice.py --probe` 现跑 rc=0 · 0.05s · 探针 36 条（假绿 0 / 假红 0.0%）；门③ 的现读建议是「成本未标定 ⇒ 起步只报告」 | 起步 `tri-report`（`PASS`） | 要不要升 `tri`（改一处模式；升了之后探针失守即红） |
| ⑩ | **门① 的 scripts 基线没同步下调** | 本批新挂 6 条脚本进「档① 真被某步跑」⇒ 门① 的「档③ 查无此名」命中数下降，而 `scripts/gate-coverage.config.json` 的 `baseline` 保持原值（现跑 门① rc=0 · 未超基线） | **没改**（那个文件不在本批允许改动面内） | 要不要随手把基线按新命中数下调（棘轮口径：只许下调、永不上调 ⇒ 现在只是留了余量，不影响判据） |
| ⑪ | **公开树（镜像）侧未复跑** | 公开树白名单里**没有 `docs/`**（只有 `publish/docs/*` 映射过去的几篇）⇒ 新挂的 `docs` 两步在公开树里会红：`check-glossary.py` 扫到 0 篇 ⇒ **rc=3（异常码 ⇒ `FAIL`）**；`check-i18n-drift.sh` 同族 | 本批**未做镜像**（发布暂缓；也不许 git 操作），所以这是**推断 + 现读白名单**得出的结论，未实跑 | 要不要在公开树侧给这两步做排除/换挂（属发布面），或把 `docs/zh|docs/en` 纳入公开面——**两条都超出本批允许改动面** |

### ⑥ 本批声明

- **只改 3 个脚本 + 各自的说明书**：`scripts/precommit-gates.sh`（+ 本 .md）· `scripts/check-wired-scripts.py`
  （+ 其 .md：**只改挂接说明**，判据/退码/白名单一字未改）· `scripts/publish-preflight.sh`（**只增两道闸步** +
  一个 `BLOCKED` 计数）。
- **未碰**：`ui/` · `docs/` 其它正文 · 任何门脚本的**判据** · `publish/` 其它件；**未做任何 git 操作**；**未删任何步骤**。
- **唯一被改动的既有步骤** = `gates` 的门③：模式 `tri-report` → `tri`、命令串**增加** `--strict-report`、
  名字里的档位标注「只报告」→「阻断」（步骤身份/scope/工作目录/门脚本判据都没动）。
- **没为凑绿填白名单 / 没加 allow / 没放宽判据**：门③ 的 `WHITELIST` 一条未加（仍是 4 条）；
  i18n-drift 的 9 页缺 en 侧与 1 篇过期、glossary 的 3 处术语档**照实进清单**。
- **只在 `/tmp` 落件**：日志 `/tmp/zerg-gates-b3*` · 夹具 `/tmp/wired-pair` · `/tmp/pf6-fx-*` · `/tmp/hcp-negctl`
  （未装任何全局包、未起常驻进程、未删仓内文件）。
