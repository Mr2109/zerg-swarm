# check-gate-coverage — 门覆盖自检（《债务台账-20260918》§7 建议门①）

> **规格（真源，只读）**
> `docs/项目文档/v2.5.10/债务台账-20260918.md` **§0**（「哪些域在所有门之外」11 行）与 **§7** 建议门①；
> 机读件 `docs/项目文档/v2.5.10/债务台账-20260918.tsv`。
> 被扫的「门」本身：`scripts/precommit-gates.sh`（步骤表的**唯一来源**，本门不复制第二份步骤表）。
> 入参清单：`scripts/gate-coverage.config.json`（扫描面 / 硬排除 / 白名单 / 基线棘轮）。
> 本门**只读**（除 `/tmp` 自检夹具外不写盘），**从不改任何已有文件**。

## 0. 一句话

**断言「该被门收进来的东西，真的被某一步收进来了」。**
其余所有门守的是仓里的代码；本门守的是**门自己**（有没有漏掉一整个目录 / 一整个 module / 一整类脚本）。

**为什么必须有这一层**（台账 §0 的共性根因，逐字）：

> 所有域的判定都靠「脚本里手写的 add_step 步骤表」；**没有一处能发现「一个新目录/新 module 没被任何 add_step 收进来」**。

⇒ 这就是「假绿」的形态本体：`ui/` 可以**编译失败**而闸全绿（`precheck` 只验 `ui/Cargo.toml` **在不在**、不验它**过不过**）。

## 1. 两条断言

### A 构建清单目录必须被某步收进 —— `[FAIL · 阻断]`

**候选集** = 全仓含 `go.mod` / `Cargo.toml` 的目录（`config.scan.build_manifests`），剪掉第一档硬排除。

**命中口径（三条，任一条成立即算「被收进」，报告逐条给「是哪一步」作证）**

| 口径 | 判据 | 例 |
|---|---|---|
| **A-a 工作目录命中** | 某步 `add_step` 的**工作目录** == 该构建目录 | `add_step go … rc "${REPO_ROOT}/core" "go build ./..."` |
| **A-b 命令串路径参数命中** | 命令串里**点名**该目录（按路径边界） | `shared` · `./shared` · `cd shared` · `shared/...` 都算；`ui` **不许**被 `build` 之类的子串蹭到 |
| **A-c 通配命中** | 命令串里的 shell 通配 token 经 `fnmatch` 后覆盖该目录 | `for d in core agent shared; do …` 里的 `shared` |

**★ 口径写死一条（被咬过的地方）：上级目录的 `./...` 不收下级嵌套 module。**
这是 Go 的多 module 语义（`./...` **不跨 `go.mod` 边界**）⇒「core 被收进」**不能**证明「shared 被收进」。
若哪天把这条放宽成「上级覆盖下级」，本门立刻失去意义（`core/…` 一挂就能把整个仓洗绿）。

### B scripts 脚本必须要么在步骤表、要么在白名单 —— `[FAIL · 阻断 · 基线棘轮]`

**候选集** = `scripts/*.sh` + `scripts/*.py`（`config.scan.script_globs`）+
`scripts/` 下**无后缀可执行件**（有执行位**或**首行 shebang 才算脚本）。

**三档（★ 计数自证：① + ② + ③ ≡ 候选总数，报告里直接打印恒等式）**

| 档 | 判据 | 意义 |
|---|---|---|
| **①** | 脚本名出现在某个 `add_step` 的**命令串**里 | **真被某步跑** —— 最强 |
| **②** | 出现在 `precommit-gates.sh` 里，但**不在任何 add_step 命令串**里 | **弱挂载**：注释里点名 / 尾部软门禁调用位 / 总控脚本自身 —— **单列出来让人看见**，「写在注释里的门」不算接了门 |
| **③** | 闸文件里**查无此名** | **棘轮命中数**的来源（扣掉②档白名单后与 `baseline` 比） |

**★ 第四档：通用语法步的 glob 兜底 `不计入` ①②③。**
`scripts/*.sh 语法（bash -n）` 与 `scripts/*.py 语法（ast.parse）` 两条步骤用**通配**覆盖了全部 `scripts/*.sh|*.py` ——
但**语法过 ≠ 接了门**：一只门脚本只要没人 `add_step` 它就永远不会有人跑它。
把 glob 当「在表里」正是本门要治的那个假绿，所以 glob 命中**单独打印**（「注：另 N 条被通用语法步的 glob 兜住」）。

## 2. 例外表（四档 · `--list-rules` 全列 · 逐档计数可见）

| 档 | 名 | 进不进候选 / 退码 | 口径 |
|---|---|---|---|
| 一 | **硬排除**（`config.exclude`） | **不进候选集** | 结构级：`.git` `vendor` `node_modules` `target` `__pycache__` `.history` `.obsidian` `dist` `bin` `data` `zerg-wt` `.zerg` `venv` `.cargo` `.venv` `htmlcov` + 前缀 `tools/ocr/venv`。★ 它们不是「放过」，是**压根不在候选里**（口径逐字对齐 check-doc-name/meta/freshness 的 EXCL/PRE 名单与台账 §0 第 9 行的 13 个硬排除名） |
| 二 | **白名单**（`config.whitelist`） | 不进退码，**计数可见** | 每条必须 `reason` + `date`；**缺 reason 或缺 date ⇒ rc=2**（不是红 —— 写不清理由的豁免就是洗白） |
| 三 | **基线棘轮**（`config.baseline`） | 不超基线不进退码 | 存量债的**已登记额度**；超一条即红；**只许下调、永不上调** |
| 四 | **通用语法 glob 兜底** | 不进 ①②③ | 只证明「语法过了」（见 §1 的 ★）；单列一条计数 |

**★ 悬空白名单**（写了 reason/date 但盘上已无此件）⇒ **照报，不改退码**。豁免面漂移必须看得见。

**★ 一条硬规矩（写进脚本的输出里）**：
**不许为了让本门变绿而给真漏项填白名单**。处置只有两条路 —— **真修**（补一步 `add_step`）或**白名单写清理由 + 日期**；
白名单条目在报告里进「② 白名单豁免（第二档）」**逐条可见**，不是隐藏。

## 3. 基线棘轮 · 怎么下调

**为什么有它**：B 档今天命中 **74** 条（见 §5 首跑）。**一次红 74 条 = 把提交闸锁死**，谁都会绕过它 ——
同 `docs: freshness D2` 的「只报告」先例（改**档位**，不动**判据**）：先落地、逐批接线、每批把基线往下拧一格。

**号令字句**（每批下调基线时，父代理/主代理**照抄**）：

> 「批 〈批次名〉：本批把下列 〈N〉 条从「闸文件查无此名」改成「步骤表里点名」：〈逐条脚本名〉。
> 　依据 = `python3 scripts/check-gate-coverage.py --baseline` 的 **③ 实测命中数**（改前/改后各跑一次）；
> 　动作 = 把 `scripts/gate-coverage.config.json` 的 `baseline` 由 〈旧〉 改成 〈新〉（**只改这一个数字**）；
> 　规矩 = **一次只许下调、永不上调**；若实测值比基线**高**（= 又冒出新脚本）⇒ 先接线或补白名单理由，
> 　**不许**把基线抬上去。」

`python3 scripts/check-gate-coverage.py --baseline` 会把这段 + 「③ 实测命中数 / 当前 baseline / 剩余 / 建议值」一起打印，
**照抄即可**，不需要人去数脚本。

## 4. 用法 · 退码

```bash
cd "<repo>"

python3 scripts/check-gate-coverage.py               # 自检 + 两条断言扫真目标
python3 scripts/check-gate-coverage.py --json        # 机读（A/B 分桶 + 三档清单 + 悬空白名单）
python3 scripts/check-gate-coverage.py --baseline    # 基线视图 + 下调号令字句（只读）
python3 scripts/check-gate-coverage.py --list-rules  # 两条断言 + 例外表四档 + 退码口径
python3 scripts/check-gate-coverage.py --self-test   # 只跑自检（合成夹具，不碰真目标）
python3 scripts/check-gate-coverage.py --no-self-test   # 内部子进程 / 迭代调试用（门禁别用）
python3 scripts/check-gate-coverage.py --max-examples 80  # 每桶最多列几条（默认 40）
```

**`--json` 约定**：**stdout 只有一份纯 JSON**（结论也机读得到：`verdict.code` ∈ `OK` / `RED` /
`EMPTY_SCAN` / `CERTIFY_FAILED`，另有 `rc` 字段）；人读的那一行结论走 **stderr**。
rc=2 且 **stdout 为空** = 缺件/不可判（理由在 stderr），此时**没有** JSON 可解 —— 这是有意的：
「判不了」不该伪装成一份完整报告。

**退码三档**（**优先级 2 > 1 > 0**，与 check-doc-meta / check-doc-name / check-doc-freshness 同口径）：

| rc | 含义 | 触发 |
|---|---|---|
| **0** | 全绿 | A 无未收进目录；B 命中数 ≤ 基线（**仍打印剩余数**，不许悄悄绿） |
| **1** | **有红** | A 有目录没被任何步收进、且不在白名单；**或** B 命中数 > 基线 |
| **2** | **不给结论** | 用法错 / 缺件（config 或闸文件不在）/ config 有本脚本未实现的键 / 白名单条目缺 `reason` 或 `date` / **空转**（候选集为 0 = 假覆盖）/ 计数自证不过 / 自检未过 |

★ **有「判不了」的项时不给结论，即便同时有红** —— 判不了的时候报绿报红都是猜（同 check-doc-meta 的「优先级 2 > 1 > 0」）。
★ **空转 = 假覆盖 = rc=2**：A 扫到 0 个构建清单目录、或 B 扫到 0 个脚本 ⇒ **绝不报绿**
（同 check-doc-name/meta 的「扫描域为空 ⇒ rc=2」口径；`precommit-gates.sh` 的无后缀语法步用的是 rc=1，
本门按仓里三档门的通行口径归 **2**，并在自检第 ⑧/⑨ 条钉死「空转绝不报绿」）。

## 5. 自检（成对负控 · `--self-test` · 28 条断言）

沿用仓内三只文档门「**先自证会红，再扫真目标**」的规矩；自检用**合成夹具 + 真命令行 + 真退出码**跑 15 组用例：

| # | 用例 | 断言 |
|---|---|---|
| ① | 好件（目录被收进 + 脚本在表里） | rc=0 |
| ② | **抽掉「收进 core」那一步** | rc=1 **且点名 `core`** 且打印「不许填白名单洗干净」 |
| ③ | **把这一步补回来** | rc=0 —— **与②成对**（漏掉必红 ↔ 补齐必绿） |
| ④ | B 命中 1 ≤ 基线 1 | rc=0 **且打印「剩余 1 条」**（悄悄绿 = 不许） |
| ⑤ | B 命中 1 > 基线 0（**同一夹具**） | rc=1 且点名「超过基线 1 条」—— **与④成对** |
| ⑥ | 白名单条目**缺 reason** | rc=**2**（不是 1） |
| ⑦ | 白名单条目**缺 date** | rc=2 |
| ⑧ | **A 空转**（0 个构建清单目录） | rc=2，**绝不报绿** |
| ⑨ | **B 空转**（0 个脚本） | rc=2，**绝不报绿** |
| ⑩ | config 缺件 | rc=2 且点名「config 不在」 |
| ⑪ | 闸文件缺件 | rc=2 且点名「闸文件不在」 |
| ⑫ | config 有**未实现的键** | rc=2 且点名那个键（schema 加了规则、门照绿 = 假覆盖） |
| ⑬ | **悬空白名单** | rc **不变**（0）但照报 |
| ⑭ | 只在闸文件注释里被「提到」的脚本 | 落 **② 档**、不并进 ① 档 |
| ⑮ | 报告含计数自证两条恒等式 | ①+②+③ ≡ 候选总数 |

### 5.1 真步骤表上的成对负控（建门当天实测 · 只读、只在 /tmp）

自检用的是合成夹具；为了证明**对着真 `precommit-gates.sh` 也有区分度**，另做了一次**只读**负控
（把仓的 `scripts/` + 六个构建清单目录拷进 `/tmp`，**一个字节都不动真仓**）：

| 步骤 | A 红数 | 结论 |
|---|---|---|
| ① 真步骤表（未改） | **3** | `ui` · `shared` · `scripts/exportnames` —— rc=1 |
| ② 抽掉 6 行 `add_step go … "${REPO_ROOT}/core" …` | **4**，且**点名 `core`** | rc=1 —— **漏掉必红**（真表上也红，不是夹具特有） |
| ③ 把那 6 行放回（= ①） | **3** | 回到起点 —— **补齐必绿**（与②成对） |

## 6. 首跑真数字（2026-09-18 · 建门当天）

```
python3 scripts/check-gate-coverage.py     # 自检 28 条全过 · 真目标 rc=1
```

**A（构建清单目录）** —— 候选 **8** = 收进 **3** + 白名单 **2** + **红 3**（`8 = 3+2+3` ✓）
（★ **2026-09-19 现跑：候选 7 = 收进 6 + 白名单 1 + 红 0** —— 镜像退役使候选少一条、白名单第二条已删，原红 3 条已接线。见本节末 ★）

| 分桶 | 条目 |
|---|---|
| ① 收进 3 | `agent`（工作目录命中）· `core`（工作目录命中）· `wall`（工作目录命中） |
| ② 白名单 2（**首跑时点**） | `docs/调研/子报告-内建调试版-20260917/redaction-probe`（调研复现夹具，非构建面）· ~~`docs/虫族文档/调研/…/redaction-probe`（主树镜像副本）~~—— 两条带 reason + date，逐条可见。★ **2026-09-19：镜像退役 ⇒ 第二条已从 `gate-coverage.config.json` 删除**（死条目 = 悬空白名单漂移），今天 **② 白名单恰剩 1 条** |
| **③ 红 3** | **`ui`** · **`shared`** · **`scripts/exportnames`** —— 与台账 §0 第 1/2 行点名的**完全同三条** |

**B（scripts 脚本）** —— 候选 **87**（sh 32 · py 53 · 无后缀 2）= ① **5** + ② **8** + ③ **74**（`87 = 5+8+74` ✓）

| 分桶 | 数 | 说明 |
|---|---|---|
| ① 命令串里点名 | 5 | `check-build-tags.py` · `check-doc-freshness.py` · `check-doc-meta.py` · `check-doc-name.py` · `check-shell-unicode-vars.py` |
| ② 弱挂载（只被「提到」） | 8 | `precommit-gates.sh`（总控自身）· `publish-preflight.sh` · `check-hardcoded-private-paths.py` · `check-public-tree-hazards.py` · `check-publish-face-sync.py` · `check-zh-en.py`（尾部软门禁调用位）· `edit-assert` · `mutate-scan` |
| ③ 查无此名 | 74 | 其中白名单豁免 **0** ⇒ **棘轮命中数 = 74** |
| 基线 | **74** | ⇒ **B 档自己不超额**（B 档判 rc=0）；整脚本仍是 **rc=1** —— 被 A 档那 3 条红拉着。报告仍打印「剩余 74 条」 |
| 第四档 glob 兜底 | 32 | `scripts/*.sh` 语法步——**语法过 ≠ 接了门** |

**★ 与台账口径的差：**台账 §0/§6 记的是 **71 / 82**（只数 `scripts/*.sh|*.py`）。
本门首跑是 **74 / 85**（`.sh/.py`）+ 2 条无后缀 = **87**。差的 **+3** 是**并发会话在同一个 22:4x 时点新建的脚本**：
`scripts/check-gate-coverage.py`（**本门自身**）、`scripts/check-version-sources.py`（门②）、`scripts/check-wired-scripts.py`（门③）。
⇒ 三个数字都是真的，只是**时点不同**（台账现跑时点 22:2x–22:4x vs 本门首跑 22:4x）。
**第二波**把三只新门挂进步骤表后，这 3 条从 ③ 落到 ①/②，命中数回落 **74 → 69**
（= 74 − 3 只新门 − 2 条只因新增的「外围脚本语法」步**点到了同名兄弟**而进 ② 档的件；
2026-09-18 收尾复测：**③ = 69 = baseline**，`baseline` 已按 §3 号令由 74 下调为 **69**，见 `scripts/gate-coverage.config.json` 的 `_baseline_note`）。

## 7. 与既有门的接口（**已挂 · 2026-09-18 收尾更新**）

> ★ 本节原文写于「本波不挂」时点，**已被事实取代**：本门**已挂进** `scripts/precommit-gates.sh`——
> **`gates` scope · 模式 `tri`（阻断）· 在默认集内**（`DEFAULT_SCOPES` 含 `gates`）。
> 步骤名：`门① 覆盖：构建清单目录 + 脚本接线（阻断）`。自检数字：**`--self-test` rc=0 · 28/28 断言全过**。
> 复测真目标（2026-09-18 收尾时点）：**rc=0** · A 收进 6 + 白名单 2 + 红 0 = 8 ✓ · B ① 8 + ② 10 + ③ 69 = 87 ✓
> · 棘轮命中 **69 = baseline 69**（baseline 已按 §3 号令由 74 下调为 **69**，理由见 `scripts/gate-coverage.config.json` 的 `_baseline_note`）。
> · **2026-09-19 复测（镜像退役后 · 现跑）**：**rc=0** · A 收进 **6** + 白名单 **1** + 红 **0** = **7**（候选 7 ✓；退役删掉的白名单条目是**盘上查无此件**的悬空条目，本就不进 ② 计数）·
> B ① 14 + ② 8 + ③ 65 = 87 ✓ · 棘轮 **65 ≤ 基线 69（剩余 65）** · **悬空白名单 = 0 条**（退役前是 1 条 = 镜像那条 ⇒ 删后归零，豁免面不再虚胖）。
> 下面保留原「挂法」正文（含真实落入步骤表的形态，供核对）：

```bash
add_step gates "门① 覆盖：构建清单目录 + 脚本接线（阻断）" tri "${REPO_ROOT}" \
         "python3 scripts/check-gate-coverage.py"
```

- **不接管道**（不写 `| tee` / `| tail`），rc 由 `_judge` 按**真退出码**判；本门 rc=2 落 **BLOCKED**（不计失败项数、但也不许当绿）。
- **别传 `--no-self-test`**：本门自带成对负控，先自证「会红」再扫真目标（同 docs 面三只门的纪律）。
- **别改判定档位来让它变绿**：白名单/基线都在 `scripts/gate-coverage.config.json` 里，改那里是**明账**；
  改 `_judge`/模式名是**暗账**。
- 与门③ `check-wired-scripts.py` 的分工：门③ 管「**门脚本**有没有被某个闸调用」（点），本门 B 管「**`scripts/` 下所有脚本**有没有在步骤表里」（面）；
  两者重叠的部分（`check-*.py|sh`）是有意的**双保险**。

## 8. 与 `check-doc-name.py` 的一条**交叉影响**（本波发现 · 留给父代理拍板）

本波按规格交了 **`scripts/check-gate-coverage.{py,md}` 一对**（脚本 + 说明书）—— 这会命中
`scripts/check-doc-name.py` 的 **N6 [FAIL]「同目录同基名、扩展名不同」**（`scripts/check-gate-coverage →
同基名不同扩展 .md/.py`）。实测（建门当天 · 只读跑一次）：

```
python3 scripts/check-doc-name.py --scope repo     # ⇒ rc=1 · FAIL：3 个不合规命中
  N6 [FAIL] … 命中 15（可改面 3 · 冻结区 0）
      · scripts/check-gate-coverage  → 同基名不同扩展 .md/.py   ← 本门
      · scripts/check-version-sources → 同基名不同扩展 .md/.py   ← 门②（并发会话）
      · scripts/check-wired-scripts   → 同基名不同扩展 .md/.py   ← 门③（并发会话）
```

**这 3 条正是本波三只新门**（`可改面 3 · 冻结区 0` ⇒ 之前可改面为 0）。处置**不在本波**：
`check-doc-name.py` 的例外表 **C 档**已有「脚本 + 说明书」四对先例（`C01–C03` = `check-doc-meta` /
`check-doc-name` / `check-doc-freshness`，`C04` = `precommit-gates`，均已拍），
把这三对登记成 **C05–C07** 即可（**同一先例、同一形态**，不是新豁免类别）。
★ 本波**不越界改** `check-doc-name.py`（父代理明令只新建三件）；此条**上报父代理**，
否则 docs 面的阻断步（默认集里的 `tri`）会因本波变成 rc=1。

## 9. 声明

- 本波**只新建三个文件**：`scripts/check-gate-coverage.py` · `scripts/check-gate-coverage.md` · `scripts/gate-coverage.config.json`；
  **未改任何已有文件**（含 `scripts/precommit-gates.sh`）；**未做任何 git 操作**；未安装任何全局包；只在 `/tmp` 试。
- 本门**只读**：除 `--self-test` 在 `$TMPDIR` 造的夹具（退出即清理）外，**不写盘**。
- 步骤表**只有一份真源**（`precommit-gates.sh` 自己）：本门用 `source + EXIT trap` 在闸脚本自己的 shell 里
  `main --list` 一遍把五个 indexed array 原样取出（命令串含 heredoc / 嵌套 `$()` / 中文，文本正则会漏会错）；
  闸脚本 `--list` 非零退出 ⇒ 本门 rc=2（**不给结论**），绝不猜。
- 本门**不能**证明「步骤表里的命令是对的」（那是各门自己的 `--self-test` 的活）；它只证明
  「**该被收进来的东西，确实有一步在管**」—— 上限写在这里，免得上限被当成全能。
