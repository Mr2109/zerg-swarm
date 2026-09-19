# check-wired-scripts — 门③：脚本接线自检（只报告档起步）

> 脚本本体：`scripts/check-wired-scripts.py`（本文件是它的说明书，改动必须与脚本同步）。
> 立项出处：2026-09-18 存量债务普查 `Zerg-内部文档/项目文档/v2.5.10/债务台账-20260918.md`（★ 2026-09-19 二次分家批5 改指：该件随整目录移出工作树）
> 的 §0 表 #11（「5 只门脚本既不在提交闸也不在发布闸」）与 §5（孤岛死重：`scripts/` 零引用 4 只），
> 以及该文末的三条建门建议之③（「`check-wired-scripts.py`（脚本接线自检 · 只报告档起步）」）。
> 本批**只新建**这两个文件：不挂闸、不改任何既有文件（挂接与升档见 §五）。
> ★ **状态更新（2026-09-18 收尾 · 第二波）：已挂闸** —— `gates` scope · 模式 **`tri-report`**（只报告）· 在默认集内。
> 步骤名：`门③ 接线：scripts 门脚本有没有被闸调用（只报告）`。自检数字：**`--self-test` rc=0 · 16/16 通过**。
> ★ **状态更新（2026-09-18 ③批次 · 升阻断已执行）**：§四 那张 A 命中表里的 **8 只已逐只上岗** ⇒
> 现跑 **A 命中 0 · B 未登记 0** ⇒ 门③ 已升成 **`tri` + `--strict-report`（阻断档）**，
> 步骤名同步改为 `门③ 接线：scripts 门脚本有没有被闸调用（阻断）`（判据与退码一字未改）。
> 升档全过程（含「同一夹具同命中、只换开关 ⇒ rc 0/1」的真命令行成对证据）见 §五。

## 一、它判什么（两条断言）

**断言 A —— 门脚本必须上岗**：凡**门脚本**必须出现在**至少一个闸**里。

    门脚本 = ① 命名以 `check-` 开头的脚本（`scripts/` 顶层：`*.py` / `*.sh` / 无后缀但首行 `#!`）
             ② **说明书自称门**的脚本（同干 `scripts/<名>.md` 的**头部 60 行**内出现「门」字）

    「出现在闸里」= **按名调用形态**：名字出现在闸文件的某一行上，且
      ① 该行**不是纯注释行**（去缩进后以 `#` 开头）——「注释里提过它」不算上岗；
      ② 该行**不是纯清单项**（整行就是 `"scripts/<名>"` / `'./scripts/<名>'` 这种数组/数据项）——
         「在 EXCLUDES 里被排除掉」不算上岗。

**断言 B —— 零引用件必须登记**：`scripts/` 顶层**零引用**（引用面里出现 0 次）的脚本，
必须登记进白名单，且每项带 **name + reason + date**（缺一 ⇒ rc=2）。

## 二、闸清单（判据只吃这份表；下表 = 2026-09-18 现读的真实文件名与调用方式）

| 类别 | 文件 | 标签 | 现读到的调用方式 |
|---|---|---|---|
| 提交闸 | `scripts/precommit-gates.sh` | 提交前门禁总控 | `bash scripts/precommit-gates.sh [--scope go\|rust\|pub\|tags\|docs]`（步骤表 `add_step` 的 `STEP_CMD`） |
| 发布闸 | `scripts/publish-preflight.sh` | 推送前置硬闸（闸⓪~⑥ · 含闸②b） | `bash scripts/publish-preflight.sh <产物目录>` —— 八道全过才允许推（rc=2 = 有闸缺件「不给结论」，也不许推） |
| 发布闸 | `publish/mirror-public.sh` | 逐提交镜像器 | `publish/mirror-public.sh --out DIR [--push]`（对物化后的产出树跑门禁） |
| 发布闸 | `publish/mirror-public-lib.py` | 镜像器库（跑 checker 的那半边） | 被 `publish/mirror-public.sh` / `publish/parity-compare.py` 调用；`check-history-secrets.py` / `check-public-tree-private.py` 的调用方 |
| 发布闸 | `publish/ci/ci.yml` | CI（公开仓推后闸） | GitHub Actions：`run: python3 scripts/…` |
| 发布闸 | `publish/ci/release-agent.yml` | CI（Release 重切） | GitHub Actions：`python3 scripts/check_version.py` · `scripts/make-manifest.py` |

**相关但不是闸**（打印进「排除域」，判据不吃 —— 别把验收套件/弃用器/测试当闸）：

- `publish/mirror-acceptance.sh` —— B1 发布器验收套件（逐条能失败，属测试面）
- `scripts/publish-public.sh` —— 压平快照发布器，**已弃用**（Q5 待废弃；现役是 `publish/mirror-public.sh`）
- `scripts/test-publish-parity.sh` —— 两器一致性测试（测试面）

现读这 6 个闸的调用点（复核用）：

    python3 scripts/check-build-tags.py                 # precommit-gates.sh:529（tags scope）
    python3 scripts/check-doc-meta.py …                 # precommit-gates.sh:546
    python3 scripts/check-doc-name.py --scope repo       # precommit-gates.sh:547
    python3 scripts/check-doc-freshness.py d1|d2|d3      # precommit-gates.sh:548-550
    python3 scripts/check-shell-unicode-vars.py --check   # precommit-gates.sh:522 · publish/ci/ci.yml:26
    python3 scripts/check-zh-en.py                       # precommit-gates.sh:704-706（尾部软检查位，不阻断）
    python3 "$Z/scripts/check-publish-face-sync.py"      # publish-preflight.sh 闸⓪（第 15 行）
    python3 "$Z/scripts/check-placeholder-residue.py"    # 闸①（第 20 行）
    python3 "$Z/scripts/check-public-tree-private.py"    # 闸②（第 25 行）· publish/mirror-public.sh:89
    python3 "$Z/scripts/check-public-tree-hazards.py"    # 闸④（第 47 行）
    python3 scripts/check_version.py · check-gotoolchain.py · check-compat-manifest.py · check_docs.py  # publish/ci/ci.yml:54/56/65/80

③批次（2026-09-18）新挂的 8 处调用点（门③ 断言 A 的存量全部清偿 · 逐只落点）：

    python3 scripts/check-glossary.py                      # precommit-gates.sh docs scope（tri-report）
    bash scripts/check-i18n-drift.sh                        # precommit-gates.sh docs scope（tri-report）
    bash scripts/check-tool-version-sync.sh                 # precommit-gates.sh tools scope（tri · 新 scope）
    python3 scripts/check-slice.py --probe                  # precommit-gates.sh slice scope（tri-report · 新 scope）
    python3 scripts/edit-assert --self-test                 # precommit-gates.sh pub scope（tri）
    python3 scripts/mutate-scan --self-test                 # precommit-gates.sh pub scope（tri）
    python3 "$Z/scripts/check-hardcoded-private-paths.py"   # publish-preflight.sh 闸②b（紧跟闸② 私有面 · 私有源树）
    python3 "$Z/scripts/check-manifest-freshness.py"        # publish-preflight.sh 闸⑥（发布制品清单新鲜度 · --strict）

## 三、口径（数字随口径写；三条排除必须说清）

**引用面** = `git ls-files` 的**已跟踪**文本文件（未跟踪件不在判据内；与债务台账的现读口径 `git grep` 一致）
减去：

1. **本门自指的两件**（`scripts/check-wired-scripts.py` / `.md`）——
   我自己的名单里写着这些脚本名，若不排除，我写下「它是零引用」这个动作本身就把零引用变成有引用了（自伤）。
2. **本门的结论落账面**：`docs/项目文档/*/债务台账-*`（glob）——
   **记录不能充作引用**。台账点名一个死件不会让它活过来；反过来，若把记录算作引用，
   那「写下这条债」这个动作就把债抹掉了（观测行为抹掉被观测事实）。这是本门判据里唯一需要解释的一条。
   ★ **2026-09-19「项目文档」二次分家批2（只加注、条目值一字不动）**：本条当时**仍在咬**（仓内现跑命中
   `docs/项目文档/v2.5.10/债务台账-20260918.md` / `.tsv` 两件）。
   ★ **2026-09-19 批5 现读（文案同步 · 条目值仍一字不动）**：批3 **已落盘**（整目录移出工作树 → 同级
   `Zerg-内部文档/项目文档/`）⇒ 本条今天**仓内零命中**（现跑记录面 0 件）；条目**本批不删**（删它属「零命中即删」
   的收尾 = 批6；且今天删/留行为等同 ⇒ 无为收红而动判据的理由）。**不改指**仓外
   （`../Zerg-内部文档/项目文档/*/债务台账-*`）的**改指要连同匹配口径一起改**：本门只列**仓内**相对路径
   （`git ls-files` / 文件系统遍历），`../` 形态的条目前置进不来，直接写仓外 glob = 空条目（另一种假覆盖）；
   要真覆盖仓外那棵须把**引用面**也加上仓外根 = 改判据 ⇒ 属批6 决策，本批不动。
3. 候选脚本**自身**那一个文件。

**口径对照（2026-09-18 现跑 · 候选 = `scripts/` 顶层 84 只脚本）**：

| 口径 | 零引用只数 | 说明 |
|---|---|---|
| V0 = 全仓 tracked（含记录面） | **0** | 台账已经把 4 只点名写进正文 ⇒ 数字被记录抹平（这就是不许把记录算引用的实证） |
| **V1 = 本门口径（V0 − 记录面）** | **4** | 与债务台账 §5 的「scripts/ 零引用 = 4」一致 |
| V2 = V1 再排除 `docs/` 整个文档面 | 29 | ✗ 不采：会把「只在文档里指路」的活门（如 `check-i18n-drift.sh`）误判成死件 |

复现命令：`python3 scripts/check-wired-scripts.py`（输出里打印口径行与「引用面 N 个文件（…排除 自指 2 · 记录面 2 …）」）。

**排除域（现读，写在输出里）**：`scripts/253/`(3) · `scripts/exportnames/`(2) · `scripts/sandbox-probes/`(18)
不在顶层 glob 口径里；未跟踪件（本批首跑时 6 件，含并行路上新出的 `check-gate-coverage.py` /
`check-version-sources.py` 两只门）不在判据内 —— **入库后它们会出现在断言 A 的命中里**（那是预期，不是缺陷）。

## 四、首跑真输出（2026-09-18 22:46:11 · HEAD `210534bd` · 工作树改动 43 件）

> 首跑原始输出（106 行）另存 `/tmp/wired-firstrun.txt`；`--strict-report` 的同跑输出见 `/tmp/wired-strict.txt`。
> 口径注：本门按 `git ls-files` 判「已跟踪」，所以**首跑时并行路上新出的两只门**
> （`check-gate-coverage.py` / `check-version-sources.py`，此刻未跟踪）不在命中里；
> 它们入库后**会**出现在下面的 A 命中表里（那是预期，不是缺陷 —— 见 §三 排除域）。

自检：`--self-test` 16/16 通过（成对负控，见 §六）。判据面：A 候选 22 只（命名 `check-` 19 · 说明书自称门 3）；
B 候选 84 只；引用面 2713 个已跟踪文件（排除 自指 2 · 记录面 2）；闸清单在位 6/6。

### 断言 A —— 命中 8 只（未挂）

> ★ **③批次（2026-09-18）状态**：下表 8 只**已逐只上岗** ⇒ 现跑 **A 命中 0 只**（`已挂门脚本 25 只`）、
> **B 未登记 0 只**、配置错 0 ⇒ 门③ 已升 `tri` + `--strict-report`（阻断档）。下表保留为**首跑记录**
> （当时的实话：8 只确实没被任何闸按名调用），**别当现状读**。逐只落点见 §二 末尾「③批次新挂的 8 处调用点」。

| # | 脚本 | 类型 | 闸里按名调用 | 建议挂到哪里 |
|---|---|---|---|---|
| 1 | `check-glossary.py` | 命名 check- | 0 处 | 提交闸 · docs scope（`tri-report` 起步） |
| 2 | `check-i18n-drift.sh` | 命名 check- | 0 处 | 提交闸 · docs scope（`tri-report` 起步） |
| 3 | `check-manifest-freshness.py` | 命名 check- | 0 处 | 发布闸 · `publish-preflight.sh` 新增一道（判产物清单新鲜度） |
| 4 | `check-slice.py` | 命名 check- | 0 处 | 提交闸 · 新 scope（例 `slice`，`tri-report` 起步） |
| 5 | `check-tool-version-sync.sh` | 命名 check- | 0 处 | 提交闸 · 新 scope（例 `tools`，`tri` 可直接阻断） |
| 6 | `check-hardcoded-private-paths.py` | 命名 check- | 0 处 | 发布闸 · `publish-preflight.sh` 新增一道（紧跟闸② 私有面） |
| 7 | `edit-assert` | 说明书自称门（`scripts/edit-assert.md:1`「凡写盘，必过一门」） | 0 处 | 提交闸 · pub scope 增一步 `scripts/edit-assert --self-test` |
| 8 | `mutate-scan` | 说明书自称门（`scripts/mutate-scan.md` 头部引《改码与变异安全门》） | 0 处 | 提交闸 · pub scope 增一步 `scripts/mutate-scan --self-test` |

**与子串口径的差异（#6，必须点名说清）**：#1–#5 与父任务给的「今日 5 只不在任何闸」逐条一致；
本门多报 **#6 `check-hardcoded-private-paths.py`** —— 它今天只被 `scripts/publish-public.sh:130`
（EXCLUDES **清单项**）与 `scripts/precommit-gates.sh:500`（**注释**）提到，**从没被任何闸按名执行**。
按「名字出现在闸文件里就算在闸里」的子串口径它会被算成「已挂」⇒ 这正是本门判据要排除的两类（见 §一）。
**#7/#8 是「说明书自称门」那一类**：它们只被 pub scope 的
「`scripts/*`（无后缀 + 首行 `#!`）按 shebang 语法」步（`precommit-gates.sh:521`）**按 glob** 覆盖语法，
**从没按名跑过自检** ⇒ 按本门判据算未挂（各自主张见 `scripts/edit-assert.md:34` · `scripts/mutate-scan.md:27,48`）。

### 断言 B —— 零引用 4 只（全部已登记 ✓ · 未登记 0）

    start-watchdog.sh      date=2026-09-18
      reason: 手工兜底启动器（非死件）：以「脱离会话」方式拉起主控保活看门狗；主控已由 launchd
              （com.zerg.core · KeepAlive）托管 ⇒ 运行时不再需要它，保留给「不想用 launchd」那种情况
              （脚本头注释 2-5 行自述）。
    v25_more_classes.py    date=2026-09-18
      reason: 一次性历史评测脚本：v2.5 补充任务类测试（example-35b-v2 · 2026-08-13），结果已并入当版文档；
              保留作评测口径的可复核原件。
    v25_more_tasks.py      date=2026-09-18
      reason: 一次性历史评测脚本：v2.5 更多实战任务（2026-08-13），同族口径。
    v25_real10_v2.py       date=2026-09-18
      reason: 一次性历史评测脚本：v2.5 真实任务集重测（responses 后 · 2026-08-13），同族口径。

**没有为了让首跑好看而填空**：白名单这 4 条就是 V1 口径下今日的**全部**零引用件，
逐条给的是「它为什么还该在仓里」，不是「先压下去」。另打印「⚠ 仅文档点名 25 只」一节
（`total>0` 但代码侧引用 0 —— 如 `run_all.sh` · `stability-stats.py` · `check-i18n-drift.sh`），
它们**不判红**，只供人工判断：这类「只在文档里被人指路」的件既不是死件、也不是活件。

### 结论行

    【结论】A 命中 8 · B 未登记 0 · 配置错 0
            只报告档 ⇒ rc=0（命中不阻断；挂闸时用 --strict-report 或按 .md §五 走棘轮）

**本门自身的接线状态（2026-09-18 收尾更新）**：`scripts/check-wired-scripts.py` 也已**挂闸**——
按名出现在 `gates` scope 的步骤串里（`python3 scripts/check-wired-scripts.py`，模式 `tri-report`），
所以它自己的断言 A 也把它算作「已上岗」。★ 原文写「此刻也还没挂闸（本批只准新建两个文件，不许改
`precommit-gates.sh`）」——那是挂闸前时点，**已被事实取代**；**升档（§五）已于 2026-09-18 ③批次执行**
（`tri` + `--strict-report`）。

## 五、怎么升到阻断（基线棘轮 · 同 D2 先例）—— ★ 2026-09-18 ③批次：**已执行**（前后对照见本节末）

**为什么起步只报告**：今天断言 A 的命中里最熟的那 5 只都是**存量债**——门写好了、跑起来还可能是红的
（例如 D2 那档 235 条陈旧引用）。直接阻断 = 把提交闸当场锁死 ⇒ 与 D2 同一处理：**先只报告，
判据与退码一字不改，改的只是「本档位如何计账」**（D2 的档位说明见 `precommit-gates.sh:51-56`，
它现在以 `tri-report` 挂在 docs scope，见 `precommit-gates.sh:549`）。

**升档动作（批准后一次做完，逐条可验收）**：

1. **清存量债**（或明确「哪几条认账」）：把 §四 的 A 命中逐条落成「接线」或「暂缓 + 理由」。
   未接线的项写明理由，**不许默默留白**。
2. **记基线快照**：新增 `scripts/check-wired-scripts.baseline.tsv`
   （列：类别 `A|B` · 脚本名 · 计数 · 理由/落点 `file:line` · 日期），内容 = 首次阻断时的逐条清单。
   基线的判据：**逐条点名**（不是「N 条」这种汇总数）。
3. **把本门挂进闸**：`precommit-gates.sh` 新增一步（建议与 docs scope 同族，或单列 `wired` scope）：
   `python3 scripts/check-wired-scripts.py --strict-report`，模式 `tri`
   （它的 0/1/2 与 `tri` 三档一一对应：0=无命中 · 1=有命中 · 2=缺件/配置错/空转/自检不过）。
   **挂闸那一刻起就是阻断档**：命中即红（`--strict-report` ⇒ rc=1）。
   ★ **实况（2026-09-18 收尾）**：第 1 步早已先做了半格 —— 本门已以 **`tri-report`（只报告）+ 不带
   `--strict-report`** 挂进 `gates` scope（在默认集内）。
   ★ **实况（2026-09-18 ③批次 · 已执行）**：第 1 步（清存量）与第 5 步（转完全阻断）**一并落地**——
   8 只逐只上岗 ⇒ A 命中 0 · B 未登记 0，于是把本步改成
   `python3 scripts/check-wired-scripts.py --strict-report` + 模式 **`tri`**（步骤名里的档位标注同步由
   「只报告」改为「阻断」）。**两处必须一起改**：只改模式不加开关 = 挂了个恒绿步（脚本默认 rc=0）。
   第 2 步的**基线快照表未建**：存量是当批清到 0、不是逐格下调 ⇒ 直接落在第 5 步那一格，不需要棘轮表。
4. **只许减不许增（棘轮）**：基线表里的每一条都对应「要么接线、要么保留有理由」；
   新出现的命中（**包括新写的零引用件**）一律红。每还掉 N 条就把基线表对应行删掉，
   基线表**只缩不长**；白名单同理（本门已有提示：已登记的件一旦有了引用就打印「可下架」）。
5. **转完全阻断**：A 命中 0 且 B 未登记 0 时，撤掉基线表与「可容忍」语义，
   从此任何新命中都是硬红（这一步之后 `--strict-report` 变成默认行为，脚本里改默认值即可）。
   ★ **实况（③批次 · 已执行）**：A=0 · B=0 已到，故**撤掉了「可容忍」语义**（不再有基线表）。
   与 .md 原文字的**一处偏差**：**没有**把 `--strict-report` 改成脚本默认值 ——
   那样会改这个脚本对**其它调用方**的行为（本批次只允许改它的「白名单/基线与说明」）。
   改法是「阻断面由闸的步骤给」：模式 `tri` + 命令串带 `--strict-report`（步骤名标注「阻断」）。
   效果等价（提交闸里命中即红），且**手工单独跑一下不会突然拦人**；代价是「谁摘掉那个开关就会退化成
   恒绿步」—— 这一格由 `precommit-gates.sh --self-test` 的 ⑩ 组**逐字钉住**（断言命令串里含
   `--strict-report`）。

**退码对照（本门三档，与仓内 0/1/2 惯例一致）**：

| 档 | 默认（只报告） | `--strict-report` |
|---|---|---|
| 无命中 | rc=0 | rc=0 |
| 有命中 | **rc=0**（只打印清单） | **rc=1** |
| 缺件/配置错/空转/自检不过 | **rc=2**（不给结论，两档都一样） | rc=2 |

`rc=2` 触发条件（都显式打印，不许静默）：仓根缺 `scripts/` · 闸清单**一个都不在位** ·
候选 0 只（空转） · 引用面 0 个文件 · 白名单缺 reason / 缺 date / date 形态不对 / 重名 /
**悬空**（点名的脚本已不存在） · `--self-test` 任一用例不过（自检不过 ⇒ 拒绝扫真目标）。

### 五之一、升阻断**前后对照**（③批次实测 · 每格都是真命令行 + 真退出码）

**① 本门在提交闸里的档位与请求串**

| 时点 | 步骤名 | 模式 | 命令串 | 真目标 rc（本机现跑） | 在提交闸里的落点 |
|---|---|---|---|---|---|
| 升档前（第二波） | `门③ 接线：scripts 门脚本有没有被闸调用（只报告）` | `tri-report` | `python3 scripts/check-wired-scripts.py` | **rc=0**（脚本默认档；命中 8 只只进清单） | `REPORT`（不影响退出码） |
| 升档后（③批次） | `门③ 接线：scripts 门脚本有没有被闸调用（阻断）` | **`tri`** | `python3 scripts/check-wired-scripts.py --strict-report` | **rc=0**（**A 命中 0** · B 未登记 0） | **`PASS`**；此后**命中即 `FAIL`** |

**② 成对证据：同一夹具、同一命中，只换「档位/开关」⇒ rc = 0 / 1**（升档的机制本体，不是印象）

    # 夹具 /tmp/wired-pair：scripts/{precommit-gates.sh, publish-preflight.sh} 在位，另有 1 只没被任何闸调用的
    # check-unwired.py；白名单传空表（[]），故 A 命中 1 · B 未登记 3 —— 两跑**同一夹具、同一命中数**
    python3 scripts/check-wired-scripts.py --root /tmp/wired-pair --whitelist /tmp/wired-wl.json --no-self-test
      ⇒ 【结论】A 命中 1 · B 未登记 3 · 配置错 0 ／ 只报告档 ⇒ **rc=0**
    python3 scripts/check-wired-scripts.py --root /tmp/wired-pair --whitelist /tmp/wired-wl.json --no-self-test --strict-report
      ⇒ 【结论】A 命中 1 · B 未登记 3 · 配置错 0 ／ --strict-report 且命中 4 ⇒ **rc=1**

⇒ 「从只报告变阻断」只需**同一个开关**：升档前挂着它也是阻断（本次就是**加开关 + 改模式标注**两处，
不涉及判据）。脚本自身的默认档**没动**（理由见 §五 第 5 条）。

**③ 门③ 报的 8 只 —— 逐只上岗的落点与首跑 rc（现跑）**

| # | 脚本 | 落点 | 模式 | 首跑 rc（现读） | 状态落点 |
|---|---|---|---|---|---|
| ① | `check-glossary.py` | 提交闸 `docs` scope | `tri-report` | **rc=1**（T2 术语档 3 处 · T1 阻塞档 0 ⇒ 「告警」不是「错」） | `REPORT`（不计失败项） |
| ② | `check-i18n-drift.sh` | 提交闸 `docs` scope | `tri-report` | **rc=2**（相① 缺 en 侧 9 页 · 相② 1 篇译页过期） | **`BLOCKED`**（它把 rc=2 当「阻塞相失败」，与三档的「不给结论」语义错位 ⇒ 已列待拍） |
| ③ | `check-tool-version-sync.sh` | 提交闸 **新 scope `tools`** | **`tri`** | **rc=0**（台账 141 工具 · 抬头一致 141 · 不一致 0） | `PASS`（可直接阻断） |
| ④ | `check-slice.py --probe` | 提交闸 **新 scope `slice`** | `tri-report` | **rc=0**（探针 36 条 · 假绿 0 · 假红 0.0% ⇒ 过 §3.5 门槛） | `PASS`（起步只报告；实测 0.05s ⇒ 可升 tri，留待拍） |
| ⑤ | `check-manifest-freshness.py` | 发布闸 **闸⑥**（`--strict`） | 阻断（0/1/64） | 真镜像树**缺同一版清单 ⇒ 缺件（BLOCKED）**；夹具正控 rc=0 / 负控（dirty=true）rc=1 | 缺件不许当绿、也不许推（rc=2） |
| ⑥ | `check-hardcoded-private-paths.py` | 发布闸 **闸②b**（紧跟闸② 私有面） | 阻断（0/1） | **rc=0**（私有源树 + 产物树两面都绿；负控夹具注入一处 `<volume-path>` ⇒ rc=1） | `PASS` |
| ⑦ | `edit-assert --self-test` | 提交闸 `pub` scope | **`tri`** | **rc=0**（自检 16/16 全过） | `PASS` |
| ⑧ | `mutate-scan --self-test` | 提交闸 `pub` scope | **`tri`** | **rc=0**（自检 26/26 全过） | `PASS` |

**④ 复跑（③批次收尾 · 本机）**：`python3 scripts/check-wired-scripts.py --strict-report` ⇒
**rc=0 · 【结论】A 命中 0 · B 未登记 0 · 配置错 0**（`已挂门脚本 25 只`）；`--self-test` 16/16 通过。
默认全量（45 步）与逐 scope 数字见 `scripts/precommit-gates.md` §七。

## 六、两条断言各自的自证方式

`--self-test` 走**真命令行 + 真退出码**（子进程重跑本脚本的 `--root <夹具>`），16 例全绿才允许扫真目标。
夹具在临时目录里现造，**不碰工作树**：

| # | 用例 | 打坏什么会红 |
|---|---|---|
| ① | 正控：全挂且白名单齐 ⇒ rc=0 且 0 命中 | 判据把该绿的判红（假红） |
| ② | 记录面点名**不算引用** ⇒ 零引用 1 只且已登记 | §三 排除② 失效（点一下台账就把死件「救活」） |
| ③ | 拿掉一条接线 ⇒ 报出它，只报告档仍 rc=0 | 断言 A 漏报 |
| ④ | 同一夹具 + `--strict-report` ⇒ rc=1 | 升档次序失效 |
| ⑤ | **只在注释行/纯清单项里出现 ⇒ 仍算未挂** | §一 两条排除被放宽（有牙齿的那一条） |
| ⑥ | 零引用且未登记 ⇒ 报出它（rc=0） | 断言 B 漏报 |
| ⑦ | 同夹具 + `--strict-report` ⇒ rc=1 | B 侧不参与升档 |
| ⑧a–d | 白名单缺 reason / 缺 date / date 形态不对 / 悬空 ⇒ 一律 rc=2 | 配置错被静默吞掉 |
| ⑨ | 候选 0 只 ⇒ rc=2（空转） | 空转报绿 |
| ⑩ | 闸清单一个都不在位 ⇒ rc=2（不给结论） | 判据无依据时报绿 |
| ⑪ | 同干 `.md` 头部含「门」⇒ 成为候选并报未挂 | 「说明书自称门」漏判 |
| ⑫ | 同干 `.md` 不提「门」⇒ 不成为候选 | 候选面过宽（把普通工具卷进来） |
| ⑬ | `--list-rules` 可打印判据/闸清单/升级路径 | 说明书与实现脱节 |

**手工复核（两条断言各一条命令，别只看本门的话）**：

    # 断言 A 的复核：某只门脚本到底有没有被任何闸按名调用（把注释/清单项排掉）
    grep -n "check-slice.py" scripts/precommit-gates.sh scripts/publish-preflight.sh \
        publish/mirror-public.sh publish/mirror-public-lib.py publish/ci/*.yml
    # 断言 B 的复核（本门口径：−记录面，逐只计数）
    for f in scripts/start-watchdog.sh scripts/v25_more_classes.py; do
      b=$(basename "$f"); n=$(git grep -l -F -e "$b" -- . \
        ':!docs/项目文档/*/债务台账-*' | grep -v "^scripts/$b$" | wc -l | tr -d ' '); echo "$b refs=$n"; done

## 七、用法

    python3 scripts/check-wired-scripts.py                  # 只报告（rc=0）
    python3 scripts/check-wired-scripts.py --strict-report  # 命中即 rc=1（临时/升档用）
    python3 scripts/check-wired-scripts.py --list-rules     # 判据 / 闸清单 / 口径 / 白名单 / 升级路径（rc=0）
    python3 scripts/check-wired-scripts.py --self-test      # 只跑自检（16 例；不过 ⇒ rc=2）
    python3 scripts/check-wired-scripts.py --root DIR --whitelist J   # 对夹具跑（自检内部用法）

## 八、维护规矩（改本门时必须同步的东西）

- **加闸 / 改名 / 搬家**：改 `.py` 顶部的 `GATES` 与 `NOT_GATES`，**同批**改本文件 §二。
  判据只吃 `GATES` —— 不认识的执行入口不进判据（否则任何脚本都能自称闸）。
- **白名单增删**：只在 `.py` 的 `WHITELIST` 里改，每项 `name/reason/date` 三件齐；
  删件后必须同批下架（否则 rc=2 悬空）。**增项要有理由**，不是「先压下去」。
- **口径（`SELF_FILES` / `RECORD_SURFACES`）**：改它就是改判据 ⇒ 必须同批改本文件 §三，
  并把新旧口径的零引用只数一起写进来说明为什么会变。
- **判据面不重开**：「引用」= 出现次数；「在闸里」= 按名调用形态（排除纯注释行 / 纯清单项）。
  想放宽这两条（例如「注释里提过也算」）等于把本门变成字面匹配 —— 那是另一件事，先谈设计。
- **升档后新增命中怎么处置**（2026-09-18 ③批次起本门是**阻断档**）：
  新写一只门脚本 / 新写一个零引用件 ⇒ 提交闸的 `门③` 步骤**立刻红**。处置只有两条路 ——
  **真接线**（在 `precommit-gates.sh` 或 `publish-preflight.sh` 里加一条按名调用，**不许**只写进注释或
  EXCLUDES 清单项）或**白名单登记**（`name`+`reason`+`date` 三件齐）。★ 不许把新命中压进白名单当消红手段。
- **阻断面在闸的步骤里，不在脚本默认值里**：本脚本默认仍是只报告（rc=0）——
  阻断由 `gates` scope 那一步的「模式 `tri` + `--strict-report`」给。要临时跑一次「只报告」，
  去掉开关即可；**要恢复阻断就别把开关删掉**（删了 = 恒绿假阻断，`precommit-gates.sh --self-test` ⑩ 组会红）。
- **与 `edit-assert` 的分工**：本门**只读**（不写任何文件、不跑别的门、不改仓），
  需要写盘的改动一律走 `scripts/edit-assert`。
- **已知的一笔「新增命名命中」（留给父代理收口，本批不许改既有文件）**：
  本门这一对 `scripts/check-wired-scripts.{py,md}` 会被 `scripts/check-doc-name.py` 的
  **N6（同目录同基名、扩展名不同）** 报到（现跑：该规则命中 15 · 可改面 3，其中一条是本门这一对）。
  这与 `scripts/edit-assert.{py,md}` 等既有「脚本 + 说明书」同族，仓内既有口径 = 在
  `check-doc-name.py` 的 `EXEMPT_C` 里逐对登记（C01/C02… 的理由原文：
  「本轮 W2 交付：门脚本 + 说明书（同 `scripts/edit-assert` 既有约定）· **待父代理批准**」）⇒
  收口动作 = 按其同口径为本门（以及并行路上新出的 `check-gate-coverage` / `check-version-sources`）补一条 C 档豁免。
  **只报告档的副作用仅此一处，判据与退码不受影响**（N6 属 check-doc-name.py 的既有规则，不是本门判据）。
