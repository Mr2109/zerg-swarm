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
{"schema_version":1,"platform":"linux","argv":["--ro-bind","/usr","/usr",…],"allowlist":["ro-mount:/sys（设备拓扑：任何要认设备的引擎都读它）","device:/dev/kfd",…],"note":"…"}
```

- `argv` —— **执行面**（逐字可复现；Linux 一档 = bwrap 参数序列，`argv[0]` 是 `bwrap`）；
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

## 门禁与跑法

```bash
cd wall
cargo fmt --check
cargo clippy --all-targets -- -D warnings
cargo test                 # 单元 6 + 集成 9（含三条负例组与「不许放宽」不变式）
python3 /tmp/mutate-wall.py   # 变异验证（脚本在交付回执里；每条变异必须有用例红）
```

**本批尚未做的**：macOS 一档（直调 Seatbelt / `sandbox_init`，任务 2'.4 —— 现在**明确拒绝**并说明原因）、
二档（直调原语）、Windows、制品矩阵接入（是否进 5 件矩阵是发布契约，待 Mr2109 决定）。
