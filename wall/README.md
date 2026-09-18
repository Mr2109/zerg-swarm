# 茧壁 `zerg-wall`（批 2' · 一档）

> 设计依据：`docs/01-设计/设计-茧壁-统一封闭契约与等级自证.md` §五之二（Mr2109 2026-09-16 拍板：
> 「这次 沙箱程序 必须用 Rust 写了」）。任务表：`docs/项目文档/v2.5.9/任务单-茧壁-20260916.md` §3。

## 它做什么

把「卵配方（绑定 / 环境 / 限额 / 设备）」翻译成**平台命令**，并**如实报出自己额外放行了什么**：

```text
zerg-wall plan --spec <spec.json>   # 纯函数：读配方 ⇒ stdout 一行 JSON；不 exec、不写盘
zerg-wall run  --spec <spec.json>   # 先出计划（写 stderr 留痕），再按 argv exec；子进程退出码原样透传
zerg-wall --version
```

**退出码**：`0` 成功 · `2` **硬失败**（配方被拒 / 用法错 / 认不得的平台 / 可执行不在 PATH）。
硬失败与「跑起来了」必须一眼分得开：对消费侧而言两者都按红灯处理，绝不允许被读成「没封闭但照跑」。

⚠ **一个如实记下的差别（批 2'.4 实测）**：配方里**声明了 env** 时，`argv[0]` 是 `/bin/sh`（包装 exec），
引擎自己起不来是**包装里**的 `exec "$@"` 失败 ⇒ 退出码是 shell 的 **127**；只有**不带 env**（直 exec 引擎）时
「可执行不在」才落在茧壁这一层、是 **2**。两者都是「起不来」，但消费侧读码时要知道这个分岔
（Linux 侧同形：bwrap 前面也有一层包装）。

## 计划的形状（一行 JSON）

```json
{"schema_version":1,"platform":"linux","argv":["bwrap","--ro-bind","/usr","/usr",…],"allowlist":["ro-mount:/sys（设备拓扑：任何要认设备的引擎都读它）","device:/dev/kfd",…],"note":"…"}
```

- `argv` —— **执行面**（**一条完整命令行**：`argv[0]` = 可执行名 `bwrap`，其后是它的参数序列；
  逐字可复现）。⚠ 与 Go 侧 `hatch.BuildBwrapArgv` 的**唯一差异**就在这个可执行名上：那个函数只给
  **参数**（`bwrap` 由 `BuildSystemdRunArgv` 补）⇒ 迁移桥比的是 `argv[1..]`，`argv[0]` 另有一条断言。
  为什么必须是完整命令行：`run` 就是 `Command::new(argv[0])`，退出码表里的「可执行不在 PATH ⇒ 2」
  也只有这时才说得通 —— 早先把 `argv[0]` 写成 `--ro-bind` 的版本，`plan` 载荷看起来完全正常，
  而 `run` 在**任何** Linux 机器上都只能以「起不来」收场（批 2'.3 修，见 `evidence-bridge-argv-20260916.txt`）；
- `allowlist` —— **自报面**：只列**超出基线**的授权（基线见 `note`；基线项不进清单）；
- `note` —— 如实说明（含「哪些不在本 argv 内」：归属层 `systemd-run` 的 slice/单元/限额、
  `memlock_bytes` 的落点；以及**等级由谁给**）。

**两个一档的形态差异（macOS 就是那个不同）**：Linux 把封闭**写进 argv**（`argv[0] = bwrap`）；
macOS 没有可写进去的外部沙箱程序（一档 = 直调 Seatbelt）⇒ 封闭由茧壁**本进程**施加，
`argv` 是**封闭之内**要跑的那条命令行。**载荷形状一字不加**：这件事由 `allowlist` + `note` 如实交代
（`note` 里明写「策略由茧壁本进程经 sandbox_init 施加（不经 argv）」；`run` 时策略原文另进 stderr 留痕）。

## 配方（`spec.json`）的键

`snake_case`，语义与 Go 侧 `hatch.Spec` 一一对应：

| 键 | 必需 | 说明 |
|---|---|---|
| `egg_id` | ✓ | 卵名（注册表键） |
| `schema_version` | ✓ | 必须 = 1（Go 侧 `registry.EggSchemaVersionCurrent`；陌生版本 ⇒ 拒） |
| `engine_path_in_space` | ✓ | 引擎在**空间内**的绝对路径（如 `/engine/bin/llama-server`） |
| `engine_args` | – | 引擎参数（占位符已展开） |
| `engine_roots` | – | 引擎自己的库/构建目录（宿主侧）→ 只读挂 `/engine` |
| `weight_path` | – | 权重所在宿主路径（**仅出证**，不参与挂载） |
| `weight_files` | – | 该卵点名的权重/投影/模板文件（宿主绝对路径），各落 `/models/<基名>` |
| `env` | – | 按卵环境变量（键名必须是合法 POSIX 名） |
| `devices` | – | 要暴露的设备节点；空 ⇒ 缺省 `/dev/kfd` + `/dev/dri/renderD128` |
| `extra_ro_binds` / `extra_rw_binds` | – | `<host>:<space>` 形式的额外只读 / 可写绑定 |
| `work_dir` | – | 工作目录（**空间内**路径） |
| `memlock_bytes` | – | `RLIMIT_MEMLOCK`；**不在本 argv 内**（归属层下发，只在 `note` 里说明） |

**多余键会被忽略**（配方里带着茧壁不消费的决策面字段，如实测档案）；**必需键缺 ⇒ 拒**。

## 口径（写死在代码里，别「顺手改进」）

1. **不判定等级** ✗：`observed` 一律由**外部实读**给出（设计稿 §3.1 引 GKE「沙箱内自报不可信」）
   ⇒ 茧壁唯一的自报面是 `allowlist`，**不新增等级字段**；
2. **不猜测、不补默认值** ✗：缺字段 / 陌生版本 / 认不得的平台 / 一档没落地一律**拒绝**，
   **绝不回落成裸 exec**（回落 = 一个没有封闭、却看起来正常的空间）；
3. **不放宽策略** ✗：平台适配模块执行决策，不得在宿主能力缺失时静默放宽（照抄 A3S-Lab/Sandbox 的自述）；
4. **argv 逐字对齐 Go 侧** `hatch.BuildBwrapArgv`（迁移桥 = 同一配方两侧 argv 逐条一致，任务单 2'.3）；
   要改配方先改设计稿与 Go 侧，两侧同批改。

## macOS 一档：直调 Seatbelt（批 2'.4）

`zerg-wall` 在 macOS 上**不经** `sandbox-exec` 命令行，而是 `extern "C"` 直调 libSystem 的
`sandbox_init`（`flags = 0` ⇒ 第一参数按**策略文本**解释），随后 exec `plan` 的 argv。
理由（设计稿 §五之二）：`sandbox-exec` 自 Sierra 起被 Apple 标 DEPRECATED、官方替代至今缺席
（`apple/containerization#737` 2026-05-12 起 Open）；`sandbox_init` 是同一条内核接口的库入口，
**本机实测（macOS 26）仍可用**（坏策略 `rc=-1` 且**不施加任何封闭** —— 这条正是「失败就是失败」判据的前提）。

**策略 = `scripts/sandbox-probes/pF.sb` 的黄金配方**（本机四态实测过，少放一条是一条）：
`(deny default)` + `process*` + `sysctl-read` + `mach-lookup`（基线）+ `file-read*` +
`file-write* (subpath 声明的可写区 = work_dir ∪ 可写绑定的**宿主侧**)` + **`(allow network-bind)`（不过滤）** +
`network-inbound/outbound` 限 localhost。

**如实标三条差距（别当 macOS 也有 Linux 那一级）** ✗：
1. **没有视图级封闭** ⇒ 拿不到 `enclosed.kernel`：声明里的路径**按宿主路径原样使用**，茧壁**不做**
   「空间↔宿主」映射（映射是视图层的事，凭空映射 = 造第二份真相）。⇒ 若一枚卵的声明是 Linux 形态
   （引擎 `/engine/bin/…`、权重 `/models/…`），macOS 上**跑不起来**（会以「起不来」收场，不会静默换个跑法）；
   要支持它就得有一层路径映射 —— **属设计缺口，已登记待 Mr2109 拍**（见自主作业日志的「等他拍板区」）；
2. **只读面收不窄** ⇒ 黄金配方给的是**全局 `file-read*`**（dyld/Framework/解释器的加载面无法按声明路径收口；
   收窄方案**未实测**）⇒ 它已**如实列进 `allowlist`**，且声明的只读面**不冒充额外授权**；
3. **设备/GPU（IOKit）通路未实测** ⇒ 不产生策略项、也不宣称（少放一条是一条；将来要 Metal 时先实测再放）。

**活体判据（脚本自己会红）**：

```bash
python3 scripts/wall-macos-evidence.py   # 四态 + 写空间外 + 变异验证 ⇒ 回执 wall/evidence-macos-seatbelt-20260916.txt
python3 scripts/wall-macos-mutate.py     # 四条变异：改坏实现 ⇒ 用例必须红 ⇒ 还原 sha 一致 ⇒ 复跑绿
```

回执里两种对照都在：**基线（无封闭）** 与 **封闭态** —— 出网基线不可达、或空间外本来就写不进去
⇒ 断言**没有区分度** ⇒ 脚本 rc=2 硬失败（不许把「测不出来」刷成绿）。

## 迁移桥：与 Go 侧 `hatch.BuildBwrapArgv` 逐条对拍（任务 2'.3）

茧壁是「同一个落点」的**第二份实现** ⇒ 两侧任一处「顺手改进」（少一个 `--unshare-*`、绑定顺序换一下、
包装 exec 少个 `$0`）在真机上都是「看着起来了、封闭面却不同」的静默故障。对拍是**不换件**前提下唯一
能把这种漂移钉死的手段：

```bash
python3 scripts/compare-wall-argv.py             # 门禁：0 全绿 / 1 有不一致 / 2 硬失败
python3 scripts/compare-wall-argv.py --self-test # 先证「这面镜子能红」，再跑门禁
```

- **配方**（两侧同读一批文件）：`wall/testdata/bridge-specs/*.json`，**文件名即期望值**
  （`ok-*` 两侧都应接受 ⇒ 再比 argv；`reject-*` 两侧都应拒绝）—— 成因见该目录 `README.md`；
- **两侧出口**：Go = `agent/internal/hatch/wall_bridge_test.go` 那条 `TestWallBridgeDumpBwrapArgv`
  （测试即出口：`BuildBwrapArgv` 是 Go 侧唯一真源，再包一个 cmd 就是多开一个会漂移的出口）；
  茧壁 = `zerg-wall plan --platform linux`；
- **判据两半**：① `argv[0] == "bwrap"`（可执行名，且不得以 `-` 开头）；② `argv[1..]` 与 Go 的 argv
  **逐条相同**（长度也在内）；不一致时打印**第几项**与两边原文；
- **fail-closed**：配方目录不在 / 只有 `ok-`（无区分度）/ 文件名前缀认不得 / Go 侧出口跑不起来 /
  茧壁二进制不在且 `--no-build` / 判据自检不过 —— 一律 **rc=2 硬失败**，**不许静默跳过**；
- **回执**：`wall/evidence-bridge-argv-20260916.txt`（含两侧 argv 逐条原文、自检输出、两侧各一次
  变异验证的真实红与还原后 sha 一致、制品与源码 sha256）。

## 门禁与跑法

```bash
cd wall
cargo fmt --check
cargo clippy --all-targets -- -D warnings
cargo test                 # 单元 8 + plan_linux 10 + plan_macos 10 + CLI 5 + 活体(macOS) 4
```

**macOS 侧的两条活体门禁**（在**仓根**跑）：

```bash
python3 scripts/wall-macos-evidence.py   # 0 全绿 / 1 断言红 / 2 硬失败（含基线无区分度）
python3 scripts/wall-macos-mutate.py     # 变异验证：改坏实现 ⇒ 用例必须红 ⇒ 还原 sha 一致 ⇒ 复跑绿
```

**对拍门禁**（在**仓根**跑；需要 Go 工具链）：

```bash
python3 scripts/compare-wall-argv.py --self-test   # 0 全绿 / 1 有不一致 / 2 硬失败
python3 scripts/wall-bridge-mutate.py              # 变异验证：两侧各改坏一次，门禁**必须红**
```

**判据 7 门禁 · 茧壁路线**（任务 2'.5；在**仓根**跑，先 `cd wall && cargo build`）：

```bash
python3 scripts/sandbox-probes/verify-two-states.py --wall wall/target/debug/zerg-wall
python3 scripts/sandbox-probes/verify-two-states.py --self-test --wall wall/target/debug/zerg-wall
python3 scripts/two-states-wall-evidence.py        # 生成回执 scripts/sandbox-probes/evidence-two-states-wall-20260916.txt
python3 scripts/two-states-gate-mutate.py          # 变异验证：改坏门禁 ⇒ 自检必须红（G1/G2/G3）
```

- **默认路线一字未改**（`sandbox-exec` + `pF.sb` / 自组 bwrap argv）；`--wall` 在时**多一条**：封闭态改由
  `zerg-wall run --spec <配方>` 施加（配方由门禁按探针生成），并与原生「黄金配方」路线**逐项对拍**（A/B/JIT）
  —— 茧壁是同一落点的第二份实现，**它的验收就是与基准一致**；
- **防假绿的判据落在「谁施加的封闭」上**：`--wall` 时茧壁**自己的留痕**（计划 JSON + 策略原文）必须在，
  缺一个即硬失败 —— 否则「路线被改坏成静默回落」会表现为**一路全绿**；
- **茧壁拒配方（`rc=2`）⇒ 门禁整体硬失败**，绝不把「没跑起来」读成「全被拦」（起不来的沙箱看起来最安全）；
- **已知缺口照实红**：Linux 侧茧壁沿用现行生产配方（只 `--unshare-pid`，缺 `--unshare-net`）⇒ 出网仍开，
  对拍必红 —— 那条红是**如实反映**（补它 = 改孵化行为，属待 Mr2109 拍板），不是门禁坏了；
- **回执里的制品 sha 只在可复现构建下才有意义**：dev profile 默认开增量编译，**同一份源码连编两次 sha 不同**
  （实测 `98bc845e…` / `c11b85ad…`），`CARGO_INCREMENTAL=0` 下逐字节相同 ⇒ 回执生成器第一步就做这条自检，
  通过后才记录制品 sha。

## 制品与构建（任务 2'.6）

```bash
bash scripts/build-wall.sh                   # release → bin/zerg-wall（本机平台）
bash scripts/build-wall.sh --debug           # debug   → wall/target/debug/zerg-wall（上面那些门禁读它）
bash scripts/negctl-build-wall.sh            # 负例活体控制：C0 真树必绿 · N1/N2/N3 各自真红在 rc=2
python3 scripts/build-wall-mutate.py         # 变异验证：改坏实现 ⇒ 负例控制**必须红在那一条**对照项上
bash scripts/probe-build-all-wall-wiring.sh  # 证 build-all.sh 的接线（桩树里跑原文，不碰生产 bin/）
```

- **构建入口只有一个**：`scripts/build-wall.sh`（`build-all.sh` 也调它，不复制第二份构造）。
  它**不交叉编译** —— 茧壁要在**目标机**上编（与 zerg-agent「该机自编」同一口径）；
- **只落 `bin/`、不进 dist 制品矩阵**：矩阵 5 件是**发布契约**，要不要加茧壁**待 Mr2109 拍板** ⚠
  （与 `cocoon-docs-service` 同一条边界）。接线已接上、也在桩树里照跑过 —— 但**跑真的 `build-all.sh`
  会覆盖 `bin/` 里正在被托管的生产制品**（= 换件，属自主作业禁区）⇒ 取证的等价做法是**桩树**：
  在 `/private/tmp` 搭一棵假 `go`、真 `cargo`、真 `wall/` 源码的树，把**未改动的 `build-all.sh` 原文**
  放进去跑（`--no-ui --no-sign`），产物落在桩树的 `bin/` ⇒ 接线验到、生产零接触。取证脚本
  `scripts/probe-build-all-wall-wiring.sh` 跑三次：R1 正例（rc=0 且桩树 `bin/zerg-wall` **在**）·
  R2 守卫另一支（无 cargo ⇒ rc=0、**没有**产物、留痕说明原因）· R3 区分度（把调用行换成 `:` ⇒
  产物**不在**，证明 R1 的产物断言真的会红）；
- **签名**：落 `bin/` 的件由 `build-all.sh` 的签名环节**统一**施加（稳定身份 + `com.zerg.wall`）——
  同一件事只有一处实现；`build-wall.sh` 单独跑时**只编不签**（cargo 产出的件自带 ad-hoc 签名，能跑）；
- **构建脚本永不「跳过」**：`build-wall.sh` 不成功一律 `rc=2`（构建脚本若以「跳过」成功退出，
  「没编出茧壁」就会看起来像绿）；要不要跳过由**调用方**决定 —— `build-all.sh` 只在**本机无 cargo**
  这一种情形跳过（环境缺工具），且**打印原因**；`wall/` 源码不在属**仓坏了**，硬失败（不许在这里被吞）。
- **「Linux 目标机能编」的现状（实测说清）**：`cargo check --target x86_64-unknown-linux-gnu` **rc=0**
  （Linux cfg 分支**类型检查级**通过；`rustup target add` 走代理装过 std）；`cargo build --target
  x86_64-unknown-linux-gnu` **rc=101**（本机无 Linux 链接器：`ld: unknown options: --as-needed …`）⇒
  **真编只能在目标机上做** —— 这正是不交叉编译的理由。
（不进矩阵也能随卵分发：先落 `bin/`）。

**本批尚未做的**：二档（直调原语）、Windows（本机无靶子 ⇒ 不许宣称）、制品矩阵接入（是否进 5 件矩阵是
发布契约，待 Mr2109 决定）、以及上面「如实标三条差距」里的路径映射（设计缺口）—— 任务 **2'.1 ~ 2'.5 已全部落地**。
