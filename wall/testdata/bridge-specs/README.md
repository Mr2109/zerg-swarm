# 迁移桥配方（任务 2'.3）

同一批 `*.json` **两侧都读**：Go 侧 `hatch.BuildBwrapArgv`（经 `agent/internal/hatch/wall_bridge_test.go`
装配）与 Rust 侧 `zerg-wall plan --platform linux`。判据 = **两侧 argv 逐条一致**（差一处即红）。

跑法（仓根）：

```bash
python3 scripts/evals/compare-wall-argv.py            # 门禁：0 绿 / 1 不一致 / 2 硬失败
python3 scripts/evals/compare-wall-argv.py --self-test # 先证「这面镜子能红」，再跑上面的门禁
```

**文件名就是期望值**（脚本据此判，不另开清单 —— 两份清单必然漂移）：

| 前缀 | 含义 | 两侧应表现 |
|---|---|---|
| `ok-*` | 好配方 | Go 出 argv、茧壁 `plan` rc=0 ⇒ 再比 argv |
| `reject-*` | 坏配方（**必须红**） | 两侧**都拒**才算一致；任一侧放行 ⇒ 红 |

**两侧口径必须写明的两处差异**（写实，不藏）：

1. **可执行名**：`hatch.BuildBwrapArgv` 只给**参数序列**（`bwrap` 由 `BuildSystemdRunArgv` 补），
   茧壁的 `argv` 是**完整命令行**（`argv[0] = "bwrap"`）⇒ 比的是 `wall.argv[1..]`；
   `wall.argv[0]` 另有一条断言（必须等于 `bwrap`，且不得以 `-` 开头）。
2. **决策面（实测档案 `profile`）**：茧壁**不消费**（那是每台机器的实测事实），Go 侧必须补一份合法档案
   （无档案拒孵）⇒ 桥的输入定义 = **策略面来自本目录的配方文件，决策面由 Go 侧合成**。
