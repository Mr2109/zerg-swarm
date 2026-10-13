// Package zdb 是「虫族库」（zerg.db）的库层地基：任务面从「单行大 JSON 伪数据库」
// 迁到嵌入式 SQLite（modernc 纯 Go · WAL）的第一批真表。
//
// 设计真源：
//   - Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md §十.1（tasks / task_events / idempotency_keys 三表口径）
//   - Zerg-内部文档/01-设计/设计-虫族数据库-20261013.md §三.3（DSN/PRAGMA/写池）· §三.6（库位置与三条硬规矩）· §三.7（迁移）
//
// 三条硬规矩（§三.6 · 用户已拍 · 必须落进代码）：
//  1. 卷不在 ⇒ 拒启拒写（✗ 禁回退内盘新建空库）—— readVolumeFacts + checkVolumeFacts（volume*.go）。
//  2. 开库先体检 + 自管 checkpoint —— integrity_check 须回单行 ok → 确认 WAL 生效 → 自管 wal_checkpoint（open.go）。
//  3. 备份落另一卷（内盘 ~/.zerg/backup/）—— backup.go 的同卷守卫 + VACUUM INTO。
//
// 库位置（§三.6 已拍）：<仓>/.runtime/zerg.db（ZERG_DB_PATH 可覆盖，供测试/第二实例）。
// 命令面口径不变（§三.8）：库只换**后端**，✗ 不改任何既有命令的字段名与退码；本包只增不改。
package zdb
