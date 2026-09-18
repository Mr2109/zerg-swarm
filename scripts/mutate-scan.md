# mutate-scan — 变异不自写（腿一：四维收窄 · 基线绿前置 · 清单归一 · 单点跑 · 六态）

> 规格：`docs/01-设计/设计-改码与变异安全门-v1.1-20260918.md`（§2.1 腿一 · §2.1.1 四维收窄 · §2.1.2 **基线绿是前置** ·
> §2.1.3 六态 · §2.1.4 归一后比较 · §2.1.5 cosmic-ray 五条硬限 · §2.1.6 命令样例 · §2.1.7 角色分工 ·
> §6.1/§6.3/§6.4/§6.5 工具与登记 · §7.1 J3/J5/J6 · §7.4 审计落点 · §8 B1–B8）+ 附录A（M1–M18 · C1–C14）。
> 实现：`scripts/mutate-scan`（python3 3.9 · **零三方依赖** · 门内不联网 · git 只读 · 墙钟符自带）。
> 与腿二 `scripts/edit-assert` 的分工：**腿一只回答「变异点由工具生成、跑出来什么状态」**，它写盘只发生在**副本**里；
> `edit-assert` 管「改动怎么落盘」。两腿解耦（§2.3），但**共用一个台账文件**（不同 `role`/`gate_id`）。

## 一条命令的五段（顺序写死，每段自带原因码）

```
①台账 → ②装前自证 → ③副本(cp -a) → ④基线绿前置 → ⑤清单(干跑) → ⑥归一+指纹 → ⑦四维收窄 → ⑧单点跑/六态 → ⑨begin/end 两行
 未收尾   登记目录+sha256   不许在真仓    不绿⇒rc=2     rc不采信     排序+白名单     收不到就明说    判据=产物/stdout JSON
 BLOCKED_*  BLOCKED_UNREG*  BLOCKED_SAND*  BLOCKED_BASELINE_RED  BLOCKED_MANIFEST_*  BLOCKED_POINT_*  KILLED/LIVED/…
```

★ **第 ④ 段是本门的骨头**：`gremlins` 在基线不绿时**清单根本不生成**（C3 实测）⇒ 必须报
`BLOCKED_BASELINE_RED`，**绝不能**报成「没有变异点」（`BLOCKED_MANIFEST_EMPTY` 只用于「清单可解析但 0 条」）。

## 用法

```bash
R="<repo>"

# ① 门自证（20 条子用例，真命令行 + 真退出码；不过 ⇒ 拒绝跑真目标）
python3 scripts/mutate-scan --self-test

# ② Go：一包干跑清单 + 归一（--j3 = 双跑，验「归一后一致」）
python3 scripts/mutate-scan --lang go --repo "$R" --module-dir core --pkg ./internal/sliceobs/ --j3

# ③ Go：四维收窄到**唯一变异点**再真跑（gremlins 没有单点选择器，只能收窄，§2.1.1）
python3 scripts/mutate-scan --lang go --repo "$R" --module-dir core --pkg ./internal/sliceobs/ \
    --only-types invert-negatives -E 'budget_layer.go' \
    --point 'cost_table.go:110:38:INVERT_NEGATIVES' --timeout-coeff 50 --manifest-out /tmp/m.json

# ④ Python：cosmic-ray 单点（--operator/--occurrence 就是单点；判据读 stdout JSON 的 test_outcome）
python3 scripts/mutate-scan --lang py --repo "$R" --target scripts/check_version.py --copy-paths scripts \
    --operator core/ReplaceComparisonOperator_NotEq_Eq --occurrence 0 --test-cmd 'bash judge.sh'
#    只拿 Python 清单不跑：加 --manifest-only

python3 scripts/mutate-scan --list-reasons | --help | --version
```

自检不过 ⇒ **拒绝跑真目标**（`precommit-gates.sh:36` 的项目口径）。`--no-self-test` 只给内部子进程用（防递归）。

## 四维收窄（§2.1.1；`gremlins` **无 id / 无 occurrence / 无位置选择器**）

| 维 | 手段 | 本机实测（真仓 `core/internal/sliceobs`） |
|---|---|---|
| 维1 路径/包 | `--pkg` + `-E <正则>`（可多次） | 全包 **146 点**；`-E budget_layer.go` 减一 |
| 维2 行 | `--diff <ref>` | ⚠ **包粒度不可用**：清单 `file_name` 是 basename，`git diff` 给的是仓库顶层相对路径 ⇒ 整包全 SKIPPED（`Runnable: 0`）⇒ 门报 `BLOCKED_DIFF_NO_MATCH`（详见探针稿「局限」） |
| 维3 类型 | `--only-types a,b`（11 个开关**全部显式写死**） | 只开 `invert-negatives` ⇒ **2 点** |
| 维4 逐点 | `--point <file>:<line>:<column>:<TYPE>` | 断言**可跑集合 == 1**，打印「**已收窄到唯一变异点**」；收不到 ⇒ `BLOCKED_POINT_AMBIGUOUS` / `BLOCKED_POINT_NOT_FOUND`（**明说收不到，不猜**） |

## 退出码三档 + 原因码（与 `edit-assert` **完全相同**）

| rc | 含义 | 原因码（节选；全集见 `--list-reasons`，37 条） |
|---|---|---|
| **0** | 通过 | `OK_SELFTEST` · `OK_MANIFEST` · `OK_POINT_KILLED` · `OK_MANIFEST_GREEN`（+ `WARN_*` 不阻断） |
| **1** | 失败（真红：机制没被覆盖） | `FAIL_MUTANT_LIVED` · `FAIL_MUTANT_NOT_COVERED` · `FAIL_MUTANTS_UNPROTECTED` |
| **2** | **不给结论**（不是红） | `BLOCKED_BASELINE_RED` · `BLOCKED_UNREGISTERED_TOOL` · `BLOCKED_MISSING_TOOL` · `BLOCKED_SANDBOX_UNSAFE` · `BLOCKED_SANDBOX_INCOMPLETE` · `BLOCKED_MANIFEST_MISSING/_UNPARSABLE/_EMPTY` · `BLOCKED_TIMEOUT` · `BLOCKED_TOOL_ERROR` · `BLOCKED_DIFF_NO_MATCH` · `BLOCKED_POINT_NOT_FOUND/AMBIGUOUS` · `BLOCKED_MUTANT_TIMED_OUT/_NOT_VIABLE/_SKIPPED` · `BLOCKED_CR_INIT` · `BLOCKED_OUTCOME_MISSING` · `BLOCKED_RESIDUAL` · `BLOCKED_UNFINISHED` · `BLOCKED_LEDGER_*` · `BLOCKED_USAGE` · `BLOCKED_SELFTEST` · `BLOCKED_ACCOUNT` |

**没有降级路径**（M5/C10）：缺件/未登记/基线红/清单缺件/超时**一律 rc=2**，不许换工具、不许改用文本匹配。

## 六态（§2.1.3；**不许把 SKIPPED/TIMED OUT/NOT VIABLE 当结论**）

| 状态 | 归因 | 门给 |
|---|---|---|
| `KILLED` | 判据有牙齿（**这一格才是「门在工作」的证据**） | rc=0 |
| `LIVED` | 覆盖缺口 / 判据太弱 | **rc=1** |
| `NOT COVERED` | 没有用例跑到（覆盖缺口，**不是**门没牙齿） | **rc=1** |
| `TIMED OUT` | **配置/规模问题**（系数/包大小/并发）——本机默认系数就撞上 | **rc=2**（调 `--timeout-coeff`） |
| `NOT VIABLE` | 变异使构建失败 ⇒ 不合法变异体，剔除不计分 | 单点 ⇒ rc=2；清单里只 `WARN_NOT_VIABLE_DROPPED` |
| `SKIPPED` | 被 `--diff`/过滤跳过 ⇒ **未验** | 单点 ⇒ rc=2；清单里 `WARN_SKIPPED_PRESENT` |
| *`RUNNABLE`* | **干跑态，不是六态之一**：只说明「覆盖率到得了且未被跳过」 | 不参与结论 |

## 清单归一（§2.1.4 / C4 / C6）

`解析 JSON → files 按名排序 → mutations 按 (line,column,type) 排序 → 剔白名单字段 → 比指纹`。
白名单**逐字段写理由**（只剔时间/派生计数）：`elapsed_time`（计时漂移）· `test_efficacy` ·
`mutations_coverage` · `mutator_statistics` · `mutants_total/killed/lived/not_viable/not_covered`（派生计数）。
★ 真仓实测：146 点双跑**未归一时 6 个字段不同、84–87 处顺序错位**（两次独立跑各测一次，差异本身即非确定性证据）；**归一后指纹一致**（J3 通过）。
★ **指纹 = 四元组多集摘要** `sha256(排序后的 (file,line,column,type) 多集)` ——
`gremlins` 无 `id`、`cosmic-ray` 的 `job_id` 是同输入两次 init 全不同的 UUID（C6/§2.1.5-⑤）⇒ **不许拿它们当指纹**；
Python 侧用 `(module_path, operator_name, occurrence, start_pos)` 同样派生。

## 工具（装前自证：登记目录 + binary sha256；`tools/cli-tools.json`）

| 工具 | 路径 · 版本 · binary sha256 |
|---|---|
| `gremlins` | `~/.zerg/tools/bin/gremlins` · 版本**不可考**（自报 `dev darwin/arm64`，模块 `v0.6.0`）· `9c1067d838650be0…37faf6` |
| `cosmic-ray` | `~/.zerg/tools/mutvenv/bin/cosmic-ray` · `8.7.0` · `07352e9cfb44b5b6…c1729a5` |
| `go-mutesting`（**只能 avito-tech 支**） | `~/.zerg/tools/bin/go-mutesting` · 模块 `v0.0.0-20251226130216-48d0401f00fb` · `57dddb8268abf606…651cfb8` |

安装（**锁版本**；`GOPROXY=off` 装不上 ⇒ 需联网，§6.5）：
```bash
GOBIN=~/.zerg/tools/bin GOPATH=~/.zerg/tools/gopath go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
GOBIN=~/.zerg/tools/bin GOPATH=~/.zerg/tools/gopath go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest
/usr/bin/python3 -m venv ~/.zerg/tools/mutvenv && ~/.zerg/tools/mutvenv/bin/pip install cosmic-ray==8.7.0
```
⚠ **不许** `github.com/zimmski/go-mutesting`（本机必崩（panic/SIGSEGV，C8））· **不许** `mutmut`（6/6 segfault 而 rc=0，C9）。
**`/tmp` 上的同名件一律不作数**（`statepath.go:1-3`：重启即清 + tmp_cleaner 3 天未访问即删）⇒ 工具必须落 `~/.zerg/tools/`。

## 台账（§7.4；与 `edit-assert` **同文件不同 role**，append-only JSONL，begin/end 两行）

```
${ZERG_STATE_DIR:-$HOME/.zerg/state}/edit-assert-ledger.jsonl     # 文件名不含日期（日期在行内 ts）
```
路径解析**逐字抄** `scripts/chat-harness.py:30`；**不许静默退化到 `/tmp`**（显式给才允许，且告警）。
字段：`ts · run_id · phase(begin|end) · gate_id=mutate-scan · actor · role(author|verifier) ·
spec_fingerprint(四元组多集摘要) · manifest_sha256 · tool+version+binary sha256 · pkg/target/point ·
diff/only_types/exclude_files · baseline(cmd,rc) · pre_sha/post_sha（前像/后像）· cmd/rc · status/reason_code · sandbox_root`。
**启动做未收尾检测（J6/C2）**：有 `begin` 无 `end` ⇒ `rc=2 BLOCKED_UNFINISHED` + 打印残留路径与**当前 sha256**
（与前像是否一致）。★ 门内正常失败也会补一行 `end`（`ABORTED` + 原因码），只有**进程被杀**才真留 `begin` 悬空。

## 边界（B1–B8 落到的位置）

- **只在副本里变异**：`cp -a` 到 `mktemp -d`（**必须带兄弟模块**：`core` + `shared`，`go.mod` 有 `replace … => ../shared`）；
  副本根 = 真仓根 / 在真仓内 / 已存在非空 ⇒ `BLOCKED_SANDBOX_UNSAFE`；跑前/跑后逐字节 sha256 比对（不等 ⇒ `BLOCKED_RESIDUAL`）。
- **只删自己建的**（B3）：自动 `mktemp` 的副本跑完即删；`--sandbox-root` 显式给的一律不删。
- **git 只读**（B4）· 不改用户工作树（B1）· 不碰 `~/.zerg/state`（除台账那一份）。
- **墙钟符自带**（C7）：本机**没有** `timeout`；`perl -e 'alarm shift; exec @ARGV' N -- cmd`（设计稿原样**带 `--`**）
  实测**什么都不跑却退 0** ⇒ 门内不用它，一律 python `subprocess` + 进程组 SIGKILL。
- **没挂进 `precommit-gates.sh`**：Q14（三档 rc 与门禁 `rc`/`empty` 两模式不相容）**未选型** ⇒ 本轮不改门禁，也不许挂成尾部软检查（B8）。
