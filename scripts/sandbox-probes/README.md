# sandbox-probes —— 茧壁设计稿的实测探针（2026-09-16）

这些是设计稿 `docs/01-设计/设计-茧壁-统一封闭契约与等级自证-20260916.md` §10.6~§10.10 用到的**无害靶子**，
从 `/tmp` 落进仓以免被系统清掉（判据要求"探针要落进仓"）。均为只读/无产出侧的探测，不碰生产服务 ✓。

| 文件 | 用途 | 对应节 |
|---|---|---|
| `miniprobe.py` | 空间内 unix socket / loopback TCP bind / 写空间内 / 出网 四态 | §10.6 |
| `pA.sb` `pD.sb` `pE.sb` `pF.sb` `pJ.sb` | macOS Seatbelt 各 profile 变体（pF = 黄金配方：bind 不过滤 + in/outbound 限 localhost） | §10.6 |
| `jitprobe.py` | RWX 单次映射 / W^X 路径（写码→mprotect RX→调用） | §10.8 |
| `conn_jit.py` | loopback **自己连自己** + W^X JIT（本机与 X3 四跑一致） | §10.10 |
| `lx2-linux.py` | Linux 侧四态探针（含**局域网**目标，因 X3 本身无出网） | §10.7 |
| `egg-prototype-pack.py` | 单文件封装**原型**（正式工具已是 `scripts/zerg-egg.py` ✓，此件仅存历史） | §10.8 |

跑法示例（本机）：
```bash
python3 scripts/sandbox-probes/conn_jit.py
sandbox-exec -f scripts/sandbox-probes/pF.sb python3 scripts/sandbox-probes/conn_jit.py
```
