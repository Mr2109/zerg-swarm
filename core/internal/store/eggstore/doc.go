// Package eggstore —— 卵（*.egg）的**生命周期与体量**面（卵开工 · 卵2）。
//
// # 落点理由（现读选定 · 详见报告「S1」节）
//
//   - 与既有运行态体例同源：`core/internal/store/` 一族 —— `store.go` 是主控运行态
//     名册（内存快照 + `Forget`/`StaleMachines`），`store/zdb/` 是「子包 + 分平台件」
//     的现成形状（`volume_darwin.go` / `volume_other.go`）。
//   - 可用空间取证在仓内已有先例：`core/internal/store/zdb/volume_darwin.go` 与
//     `volume_other.go` 都用 `syscall.Statfs` 的 **`Bavail`**（非特权用户可用块）取
//     可用空间 —— 本包照同一口径，Linux 另**独立**查 `Ffree`（＝ `statvfs.f_favail`）。
//   - 落地写盘纪律先例：`core/internal/api/tasks_persist.go`（同目录临时件 → `fsync`
//     文件 → 原子 `rename` → `fsync` 父目录）；本包把「**同卷同目录**」提成一等判据。
//   - 锁文件体例先例：`core/internal/compat/compat.json` 的 `excluded` 里逐字写着
//     `llama-local.lock`「0 字节 —— 不是状态，不携带数据」⇒ 运行态互斥件与状态件在
//     本仓是**两回事**；本包的审计件是 append-only 流水，不是锁。
//   - 本包**纯库件、零 CLI 接线** ⇒ 不碰命令树（`core/cmd/zerg/main.go`）、不碰卵格式
//     面（`core/cmd/zerg/zerg_egg.go`）、不碰读端（`agent/internal/eggpkg/*`）。卵族
//     CLI 具名入口（`cocoon hatch` / `cocoon gc`）按任务单 卵7① 另批补。
//
// # 口径（用户已拍定 · 逐字照做）
//
//   - **八态闭集**（`landing/landed/verified/hatching/idle/reclaimable/deleted/broken`），
//     非法值即错；**只有 `verified` 允许进孵化入口**；状态变更留可审计记录。
//   - 卵 ⟷ **展开物 / 克隆副本同一份引用计数**；删卵＝**解引用**，真删判据＝**引用计数归零**。
//   - **只有 `df` 是地面真值**（`statfs.f_bavail`）；`du` 对克隆/共享块高估 10–100 倍。
//     ★✗ **严禁把已删卵文件的名义字节当「已释放 N GB」上报**。
//   - 水线 **高 85% / 低 80%** ＋ 硬线 `imagefs<15%` / `nodefs<10%` / `inodesFree<5%`；
//     Linux 走 `f_bavail` ＋ **独立**查 `f_favail`；macOS 另读 APFS 容器（`diskutil`），
//     ✗ 取不到就留空、不假装。
//   - 落地走**同卷同目录**临时名 → 逐块校验过 → 原子 `rename`；抗掉电 `fsync` 父目录；
//     **完工标记是状态**，✗ 不得用「目录存在」当可用判据。
//
// 出处：设计真源 §十六 16.2（生命周期与体量）· 16.3（分发 · 并发 · 观测 · 回滚）·
// §13.1（按卵 sha 建目录并复用）· §14.4 M1（引用计数）。
package eggstore

// SelfTestNote —— 自检口（本包无 CLI，自检走 `SelfTest()` ＋ 测试件面）。
const SelfTestNote = "自检口 = SelfTest()（测试件面跑：go test ./internal/store/eggstore -run TestEggstoreSelfTest -count=1）"
