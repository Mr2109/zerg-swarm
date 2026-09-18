# precommit-gates — 提交前门禁总控（三档 + `docs` scope）

> 脚本本体：`scripts/precommit-gates.sh`（本文件是它的说明书，改动必须与脚本同步）
> 本文件由 **2026-09-18 路 D（门禁挂接 · Q14）** 新增；只讲「三档退码」与「`docs` scope」两件事，
> 其余（一步一文件日志 · 失败数只从结果表读 · 尾部软门禁的来龙去脉）见脚本头部注释与 `docs/` 内相关稿。

## 一、步骤级三档：0 = PASS · 1 = FAIL · 2 = BLOCKED（不给结论）

| 模式 | 判据 | 落到结果表的状态 |
|---|---|---|
| `rc`（老） | `rc=0` ⇒ 过；**其余一律失败（含 2）** | `PASS` / `FAIL` |
| `empty`（老） | `rc=0` **且输出为空**才过 | `PASS` / `FAIL` |
| **`tri`（本路新增）** | `rc=0` ⇒ 过 · `rc=1` ⇒ 失败 · **`rc=2` ⇒ 不给结论** | `PASS` / `FAIL` / **`BLOCKED`** |
| **`tri` 遇异常码** | `rc=3` / `127` / 任何非 0/1/2 的码 ⇒ **按失败算** | `FAIL` |
| 模式名不在 `{rc,empty,tri}` | 拼错 / 漏写 ⇒ **硬红** | `FAIL` |

**四条拍板口径（写死在代码里）**

1. **BLOCKED 不计入失败项数**。「没结论」不是「错」：`count_fail` 只数 `^FAIL`，`count_blocked` 只数 `^BLOCKED`
   （111→114 行那两个纯读文件的函数，全脚本没有任何地方累加）。这直接来自 Q14 的 Ø14 加强版。
2. **BLOCKED 也不许当绿**。无失败项、但有 BLOCKED ⇒ 整脚本 `rc=2`（不是 `rc=0`），并且**不打印「门禁全绿」**。
3. **异常码不许冒充「没结论」**。进程根本跑不起来（127）、被信号打断、脚本自己崩（3）——都是**红**，
   不许借第三档把「跑不起来」洗成「不置可否」。这条在自检 ⑧b 里钉住。
4. **老模式一字未改**。`rc` 模式下 `rc=2` **仍旧算失败项**——现有 20 步（go 12 · rust 3 · pub 4 · tags 1）
   的名字、命令串、模式、目录全部原样；三档只对**自己申报 `tri` 的步骤**生效。自检 ⑧d 钉住这一条。

**实现位置（一次说清，避免再找）**

| 东西 | 行 | 说明 |
|---|---|---|
| `STEP_MODE` 三档说明 | `56–61` | 步骤表旁的口径注释 |
| **`_judge()`** | `94–124` | **唯一的判据出口**：`<模式> <rc> <日志>` ⇒ `PASS/FAIL/BLOCKED`。判定只看真退出码 |
| `run_step()` | `78–90` | 调 `_judge` 取状态，立刻追加一行到 `results.tsv` |
| `count_fail` / `count_blocked` / `count_pass` | `139 / 143 / 147` | 三个数**都只从结果表 grep 出来** |
| `report()` | `171–192` | `BLOCKED` 行照 FAIL 行一样打日志路径 + 尾部原文（前缀 `~`，措辞「不计入失败项数」） |
| 自检 ⑧/⑧b/⑧c/⑧d/⑧e | `274–326` | 三档断言（见下） |
| 出口判定 + `exit` | `576–592` / `611` | 步骤总数 / 通过 / 失败项数 / BLOCKED 数 → 再定 rc |

脚本退出码因此统一成一句话：**`0` 全绿 · `1` 有失败项 · `2` 不给结论**（用法错 / 前置缺件 / 自检不过 /
**有步骤报 BLOCKED**）。`2` 从来不是「错」，而是「这次没给出结论」。

## 二、`docs` scope（文档面只读门禁）

```bash
cd "<repo>"
bash scripts/precommit-gates.sh --scope docs --list        # 看这 5 步
ZERG_STATE_DIR=/tmp/zerg-gate-d bash scripts/precommit-gates.sh --scope docs --outdir /tmp/zerg-gate-d/docs
```

| # | 步骤名 | 模式 | 命令（工作目录 = 仓根） |
|---|---|---|---|
| 1 | `docs: meta --scope formal --missing=fail` | `tri` | `python3 scripts/check-doc-meta.py --scope formal --missing=fail` |
| 2 | `docs: name --scope repo` | `tri` | `python3 scripts/check-doc-name.py --scope repo` |
| 3 | `docs: freshness D1 引用路径存在` | `tri` | `python3 scripts/check-doc-freshness.py d1` |
| 4 | `docs: freshness D2 引用 文件:行 有效` | `tri` | `python3 scripts/check-doc-freshness.py d2` |
| 5 | `docs: freshness D3 断链断锚` | `tri` | `python3 scripts/check-doc-freshness.py d3` |

**三条硬规矩（挂接时特意守住的）**

- **ⓐ 不进尾部软检查位**：这 5 步是 `build_steps()` 里 `docs)` 分支（`441–461`）的**一等步骤**，rc 由 `_judge`
  取真退出码。脚本尾部 `598–605` 那种「`|| echo …` 报告不阻断」的位置是 2026-09-17「报红却退 0」事故的同款，
  新门禁一律不进。
- **ⓑ 判定不接管道**：命令串里没有 `| tee` / `| tail` / `| head`，所以 `$?` 就是门脚本自己的退出码。
- **ⓒ 现有 20 步不动**：加 scope 是**追加**，不是改写；默认 scope 集仍是 `go rust pub tags`。
- **每只门脚本自带 `--self-test`，本 scope 不传 `--no-self-test`**：先自证「会红」，再扫真目标。
- **D4 不挂**：`check-doc-freshness.py d4`（生成式参考 drift）要临时重建 + 逐字节比对，是**发布面**的事。

## 三、自检：三档断言（`--self-test`）

`bash scripts/precommit-gates.sh --self-test` 共 **28 条断言**（原 14 条 + 本路新增 14 条），全过 ⇒ 才允许跑真目标。

| 组 | 断言 | 期望 |
|---|---|---|
| ⑧ | 同一次 run_suite 里 `tri` 三步（rc=0/1/2）⇒ 状态依次 `PASS FAIL BLOCKED` | 逐格各就各位 |
| ⑧ | 该结果集：PASS 数=1 · **失败项数=1（只数 rc=1）** · BLOCKED 数=1 | 三个数分开 |
| ⑧ | BLOCKED 那行点名到那一步、rc 字段真是 `2`、它的原文仍在自己日志里 | 可取证 |
| ⑧b | `tri` 遇 `exit 3` / `exit 127` ⇒ 失败项数=2 · **BLOCKED 数=0** | 异常码按红 |
| ⑧c | 只有 BLOCKED、没有 FAIL ⇒ **失败项数=0** · BLOCKED 数=1 | 没结论 ≠ 错（也 ≠ 绿） |
| ⑧d | 同一串 `rc=2` 走**老 `rc` 模式** ⇒ 仍是失败项、BLOCKED 数=0 | 老语义一字未改 |
| ⑧e | 模式名打错（`tri2`）⇒ 硬红 | 拼错不许静默 |

## 四、真跑数字（2026-09-18 20:39–20:48 · 本机 · **docs 树正被其它路并行改动，数字带时点**）

### ① 单跑 `docs` scope

```bash
ZERG_STATE_DIR=/tmp/zerg-gate-d bash scripts/precommit-gates.sh --scope docs --outdir /tmp/zerg-gate-d/docs
```

| 步骤 | 状态 | rc | 在跑的那件事（原文摘要） |
|---|---|---|---|
| `docs: meta --scope formal --missing=fail` | **PASS** | 0 | 正式面 52 篇：合规 19 · 不合规 0 · 告警 2 · 剔除 1 |
| `docs: name --scope repo` | **FAIL** | 1 | 6 个不合规命中（可改面；N1/N3/N5 系） |
| `docs: freshness D1 引用路径存在` | **FAIL** | 1 | 断链 11 条（命中域分布：本地 20 · 外部 2）→ 如 `docs/index-nav.md:29 → 00-总览/` |
| `docs: freshness D2 引用 文件:行 有效` | **FAIL** | 1 | 红 235 条（文件不存在 162 · 无此文件 60 · 行号越界 13）+ 黄 53 条 |
| `docs: freshness D3 断链断锚` | **BLOCKED** | **2** | **空转**：main scope 语料 294 篇里 0 条带锚链接 ⇒ 不给结论 |

- **汇总行**：`步骤总数: 5 · 通过: 1 · 失败项数: 3 · 不给结论(BLOCKED): 1` ⇒ **脚本 rc=1**。
- **`--self-test`**：28 条断言全过 · `自检结论: 全过（断言失败 0 条）` · rc=0。
- **哪条现在红**：**name、D1、D2 红；D3 是「不给结论」不是红；meta 绿。**
  （D1 的“断链”计数在本路观测窗口内从 31 掉到 11 —— 别的路正在改 docs，不是本脚本的抖动；
  任务书里的「D2 红 69」也是另一次时点的口径，这里是本次实跑的逐条计数。）

### ② 全量两种跑法（同一台机、缓存已热）

| 跑法 | 步骤数 | 通过 | 失败项数 | BLOCKED | 脚本 rc | 挂钟 |
|---|---|---|---|---|---|---|
| 默认（`go rust pub tags`，**现状**） | 20 | 20 | 0 | 0 | **0 全绿** | **1:37.33**（97.3s） |
| 显式加 docs（`--scope go --scope rust --scope pub --scope tags --scope docs`） | **25** | 21 | **3** | **1** | **1** | **1:32.94**（92.9s） |

⇒ 两种跑法挂钟差 ≈4s（go/rust 那 20 步自身有 ±5s 抖动，比 docs 的净耗时还大），
**加 docs 的净成本以第 ③ 节的逐命令计时为准**：≈2.6s。全量重跑与单跑 `docs` 的 5 格状态、rc 完全一致。

### ③ 每提交多花多久

| 情形 | 实测 |
|---|---|
| `docs` scope 整体挂钟 | **≈3.5s**（两次 3.49s / 3.60s） |
| 其中 5 个门脚本净耗时 | **≈2.6s**（meta 0.67 · name 1.28 · D1 0.13 · D2 0.38 · D3 0.13） |
| 其中各门**自带** `--self-test` | ≈1.3s（不想要就加 `--no-self-test`，本路**故意保留**：先自证会红） |
| 总控自身 `--self-test` | ≈0.35s（任何跑法都要付；`--self-test` 整体 ≈1.1s 含尾部软门禁） |
| **`docs` 进默认 scope 集 ⇒ 每次提交多花** | **≈+2.6s**（≈+3.5s 含调用脚手架）· 相对 ≈95s 的全量基线 ≈ **+3%** |
| **`docs` 不进默认 ⇒ 每次提交多花（现状）** | **+0s** |

## 五、默认是否阻断 —— 一句待拍 + 建议

> **待拍（父代理/拍板人）**：`docs` 是否进默认 scope 集，以及这 5 步里谁阻断、谁只报告。
> **建议**：`meta`／`name`／**D1**／**D3** 进阻断；**D2 先只报告**（今天红 235 条，多是历史稿件里的
> `docs/INDEX.md:14`、`ui/src/app.rs:3136-3164` 这类**陈旧引用**，属存量债，直接阻断等于把提交闸锁死）；
> D3 目前是 `rc=2`「不给结论」（空转），维持「不计失败项、也不当绿」即可。
> 落地成本：进默认 = `main()` 里 `scopes=(go rust pub tags)` 改成 `… tags docs)`（一行）；「只报告」则需给
> `docs` 再拆一个不阻断的 scope（或给 `_judge` 加第四档 `report`）——**这两件都等拍板后再动，本路不擅自加。**

## 六、本路声明

- 只改 `scripts/precommit-gates.sh`（+ 新增本说明书）。**未碰** `docs/`、`core/`、`ui/`、`wall/` 任何文件。
- **未做任何 git 操作**；未删任何步骤；现有 20 步的名字/命令串/模式/目录逐行比对**未变**（`grep -n 'add_step \(go\|rust\|pub\|tags\) '` 计数 = 20）。
- 新增步骤数 5，落在 `docs` scope 内（`--list --scope docs` ⇒ 「共 5 步」）。
