package zdb

import (
	"fmt"
	"os"
)

// StampApplicationID —— **只打标不迁移**的身份标补写入口（命令面 `zerg core db stamp` 用它 · §三.4 附 G③ · §六 Q28）。
//
// ★★ 为什么必须有这条**独立**入口（✗ 不许拿 `Open()` 里的 `ensureApplicationID` 当补标动作）：
// `zdb.Open` 是**写路径** —— 同一函数体里 `migrate()` 排在 `ensureApplicationID()` **之前**
// ⇒ 对一枚旧库补标会**顺带把迁移跑一遍**（真库现读 `user_version=1`，而内嵌迁移现读三件 ⇒ 1→3，
// 其中 0002 是破坏性迁移）。「打标不得顺带迁移」是本入口存在的**唯一**理由。
//
// 开库句柄照**备份源库那条只读用途**路径（`openSnapshotSource` · DSN = `BuildSnapshotSourceDSN`，
// **只**带 `busy_timeout`）⇒ 本入口**一条写步骤都不走**：
//
//	✗ 不跑迁移（`migrate()` 一步都不调）· ✗ 不体检写面 · ✗ 不 checkpoint · ✗ 不动 `user_version`
//
// 只做一件事：`PRAGMA application_id` 的**三态** ＋ **回读自证**（与 `ensureApplicationID` /
// `applicationIDCheck` 逐条**同口径** · ✗ 不另立第二套）：
//
//	① `== ZergApplicationID` ⇒ **空动作**（一字节不写）—— 回读两值相等且都 = 本标；
//	② `== 0`（未打标：空库首建途中 / 本特性之前的旧库）⇒ 写**一次** `PRAGMA application_id = <本标>`
//	   　＋ 回读自证（回读 ≠ 本标 ⇒ fail-closed，✗ 不静默放过）；
//	③ **其它非 0 值** ⇒ **拒执** fail-closed（`ErrNotZergDB`）—— 该库身上挂着**别人**的身份标，
//	   　✗ 不覆盖（正是本标要防的「把 A5 库 / fleet_logs 库当本库开」）。
//
// 返回 (改前值, 改后值, err)：两值都来自**回读**（✗ 不是「我打算写什么」的复述）。
// ★ 库不在盘 ⇒ 当场拒：本入口是**补标**不是**建库**，放任 `sql.Open` 的惰性建库会在一条不存在的
// 路径上种出一枚空库（`PRAGMA` 一碰就落盘）。
func StampApplicationID(dbPath string) (before, after uint32, err error) {
	// 前置：库须**已在盘**（✗ 不建库 · ✗ 不认目录）。
	if st, serr := os.Stat(dbPath); serr != nil || st.IsDir() {
		return 0, 0, fmt.Errorf("zdb: 库不在盘上（%s）⇒ 拒执（本入口是补标，✗ 不建库、✗ 不认目录）", dbPath)
	}
	h, err := openSnapshotSource(dbPath)
	if err != nil {
		return 0, 0, err
	}
	defer h.Close()

	var b int64
	if err := h.QueryRow("PRAGMA application_id").Scan(&b); err != nil {
		return 0, 0, fmt.Errorf("zdb: 读 application_id 失败: %w", err)
	}
	switch {
	case b == int64(ZergApplicationID):
		// ① 已挂本标 ⇒ 空动作（一字节不写 · 与 ensureApplicationID 第一态同口径）。
		return ZergApplicationID, ZergApplicationID, nil
	case b != 0:
		// ③ 挂着**别人**的非 0 标 ⇒ 拒执（✗ 不覆盖 · 与 ensureApplicationID 第三态同口径）。
		return uint32(b), uint32(b), fmt.Errorf("%w：application_id=%d（0x%X）≠ 约定 %d（0x%X）⇒ 拒执（✗ 不覆盖别人的身份标）",
			ErrNotZergDB, b, b, int64(ZergApplicationID), int64(ZergApplicationID))
	}
	// ② b == 0：未打标 ⇒ 一次性补标 ＋ **回读自证**。
	if _, err := h.Exec(fmt.Sprintf("PRAGMA application_id = %d", int64(ZergApplicationID))); err != nil {
		return uint32(b), uint32(b), fmt.Errorf("zdb: 写 application_id 失败: %w", err)
	}
	var a int64
	if err := h.QueryRow("PRAGMA application_id").Scan(&a); err != nil {
		return uint32(b), uint32(b), fmt.Errorf("zdb: 回读 application_id 失败: %w", err)
	}
	if a != int64(ZergApplicationID) {
		return uint32(b), uint32(a), fmt.Errorf("zdb: application_id 写入未生效（回读 %d，期望 %d）", a, int64(ZergApplicationID))
	}
	return uint32(b), uint32(a), nil
}
