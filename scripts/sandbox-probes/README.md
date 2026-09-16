# sandbox-probes —— 茧壁设计稿的实测探针（2026-09-16）

这些是设计稿 `docs/01-设计/设计-茧壁-统一封闭契约与等级自证-20260916.md` §10.6~§10.10 用到的**无害靶子**，
从 `/tmp` 落进仓以免被系统清掉（判据要求"探针要落进仓"）。均为只读/无产出侧的探测，不碰生产服务 ✓。

| 文件 | 用途 | 对应节 |
|---|---|---|
| `miniprobe.py` | 空间内 unix socket / loopback TCP bind / 写空间内 / 出网 四态 | §10.6 |
| `writeout-probe.py` | **写空间外**那一格（授权级封闭的本体：空间内可写、空间外写不进去）—— 批 2'.4 起给 macOS 一档当活体判据 | §10.6 / 批 2'.4 |
| `pA.sb` `pD.sb` `pE.sb` `pF.sb` `pJ.sb` | macOS Seatbelt 各 profile 变体（pF = 黄金配方：bind 不过滤 + in/outbound 限 localhost） | §10.6 |
| `jitprobe.py` | RWX 单次映射 / W^X 路径（写码→mprotect RX→调用） | §10.8 |
| `conn_jit.py` | loopback **自己连自己** + W^X JIT（本机与 X3 四跑一致） | §10.10 |
| `lx2-linux.py` | Linux 侧四态探针（含**局域网**目标，因 X3 本身无出网） | §10.7 |
| `egg-prototype-pack.py` | 单文件封装**原型**（正式工具已是 `scripts/zerg-egg.py` ✓，此件仅存历史） | §10.8 |
| `verify-two-states.py` | **判据 7 门禁**：两态可验（断言 A 空间内通信可用 / 断言 B 出网被拦），`--self-test` = 护栏自检 + 四态成对 | 判据 7 |
| `negctl-two-states.sh` | 判据 7 门禁的**负例活体控制**（探针缺失、基线无区分度 ⇒ 都必须硬失败 rc=2） | 判据 7 |
| `evidence-two-states-20260916.txt` | 上面三跑的**真实回执**（含 sha256、逐条输出、负例脚本原文） | 判据 7 |

跑法示例（本机）：
```bash
python3 scripts/sandbox-probes/conn_jit.py
sandbox-exec -f scripts/sandbox-probes/pF.sb python3 scripts/sandbox-probes/conn_jit.py

# 判据 7 门禁（退出码 0 绿 / 1 断言红 / 2 环境或探针问题**硬失败**，不许静默跳过）
python3 scripts/sandbox-probes/verify-two-states.py             # 真验
python3 scripts/sandbox-probes/verify-two-states.py --self-test # 变异验证（四态成对）
bash    scripts/sandbox-probes/negctl-two-states.sh             # 负例控制（两条都必须 rc=2）
```

**出网判别目标可用环境覆盖**（宿主本身没出网时必用 —— 实测 X3 基线对公网即 Timeout ⇒ 那一格没有区分度，
测出来是假的，§10.7 结论 2）：`miniprobe.py` 认 `PROBE_EGRESS_HOST/PROBE_EGRESS_PORT`（默认 1.1.1.1:443）、
`lx2-linux.py` 认 `PROBE_LAN_HOST/PROBE_LAN_PORT`（默认 <worker-host>:8580）与 `PROBE_WAN_HOST/PROBE_WAN_PORT`、
`verify-two-states.py` 认 `TWO_STATE_TARGETS="host:port[,host:port]"`（按序取第一个**基线可达**者）。
目标写进探针输出（`egress_target` / `lan_target`），证据自描述。
