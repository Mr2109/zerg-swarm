# 茧壁 `zerg-wall`（批 2' · 一档）

> 设计依据：`docs/01-设计/设计-茧壁-统一封闭契约与等级自证-20260916.md` §五之二（Mr2109 2026-09-16 拍板：
> 「这次 沙箱程序 必须用 Rust 写了」）。任务表：`docs/项目文档/v2.5.9/任务单-茧壁-20260916.md` §3。

## 它做什么

把「卵配方（绑定 / 环境 / 限额 / 设备）」翻译成**平台命令**，并**如实报出自己额外放行了什么**：

```text
zerg-wall plan --spec <spec.json>   # 纯函数：读配方 ⇒ stdout 一行 JSON；不 exec、不写盘
zerg-wall run  --spec <spec.json>   # 先出计划（写 stderr 留痕），再按 argv exec；子进程退出码原样透传
zerg-wall --version
```

**退出码**：`0` 成功 · `2` **硬失败**（配方被拒 / 用法错 / 平台没落地 / 可执行不在 PATH）。
硬失败与「跑起来了」必须一眼分得开：对消费侧而言两者都按红灯处理，绝不允许被读成「没封闭但照跑」。

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
cargo test                 # 单元 6 + 集成 10（含三条负例组与「不许放宽」不变式）+ CLI 4
bash scripts/build-all.sh  # 暂未接茧壁：是否进 5 件制品矩阵是发布契约（待拍板）
```

**对拍门禁**（在仓根跑；需要 Go 工具链）：`python3 scripts/compare-wall-argv.py --self-test`

**本批尚未做的**：macOS 一档（直调 Seatbelt / `sandbox_init`，任务 2'.4 —— 现在**明确拒绝**并说明原因）、
二档（直调原语）、Windows、制品矩阵接入（是否进 5 件矩阵是发布契约，待 Mr2109 决定）。
