// dualwrite.go —— M1 双写的**仲裁器**（文件腿 ＋ 库腿 · 文件腿是仲裁真源）。
//
// 真源：设计稿 §四.1 M1（双写：写侧同写文件 ＋ 库；读侧仍读文件 —— **文件是仲裁真源**）·
//
//	§二「不做无原子边界的『双写』」的**落地对策**（两次写之间崩 ⇒ 静默不一致 ⇒ 必须
//	把「谁先谁后 / 谁失败怎么办」写死成**唯一一条语义**）· §三.4 附 G①（三域库腿表）。
//
// ★ 两态语义（本件是**唯一**真源 · 别在调用点各写一套）：
//
//	① **先 fileLeg** —— 文件腿失败 ⇒ **立即**返文件错、`dbLeg` **一次都不调**
//	   （文件腿是仲裁真源：文件都没落成，库腿**不许**独自前进 —— 否则库面会出现
//	   文件面根本没有的行，对拍时反被判成「库里多了东西」）。
//	② **文件腿成功** ⇒ 才调 dbLeg —— 其错**只记「库落后量」**（`DualWriteLag()` 可读）、
//	   **不改**调用方看得见的结果（本函数仍返 nil）。库面落后由后续对拍 / 重放收敛。
//
// ★ 为什么库腿失败**不**升成调用方错误：调用方（`gap add` / `dev proposal new` 一类）的
//
//	结果面**由文件腿定**（M1 阶段读侧仍读文件）⇒ 把库腿的错抛上去会让「文件已落、库没落」
//	这**本可自愈**的状态变成本该成功的命令报错（把可收敛的落后升成硬失败 = 反效果）。
//	落后量必须**可读**（诊断 / 自检 / 对拍都靠它），但**不**参与结果判定。
package zdb

import (
	"sync"
	"time"
)

// DualWriteLeg —— 双写的一腿（一个写动作）：返回 nil = 该腿成功。
type DualWriteLeg func() error

// DualWriteLagReport —— 「库落后量」快照（文件面已前进、库面未跟上的累计读数）。
type DualWriteLagReport struct {
	Count   int       // 累计库腿失败次数（= 有**几笔**写没进库）
	LastErr string    // 最近一次库腿错误文本（空 = 从无落后）
	LastAt  time.Time // 最近一次库腿失败时刻（UTC）
}

var (
	dualWriteMu  sync.Mutex
	dualWriteLag DualWriteLagReport
)

// DualWrite —— 双写仲裁器（两态语义见件头注）。返回**只由文件腿决定**的错误。
func DualWrite(fileLeg, dbLeg DualWriteLeg) error {
	if err := fileLeg(); err != nil {
		// 态①：文件腿失败 ⇒ 库腿**一次都不调**，直接返文件错（仲裁真源在文件）。
		return err
	}
	if dbLeg == nil {
		// 无库腿（M0 单写文件的部署形态）⇒ 文件成即算成。
		return nil
	}
	if err := dbLeg(); err != nil {
		// 态②：文件成功 ＋ 库写失败 ⇒ **只记落后量**，不改调用方结果。
		dualWriteMu.Lock()
		dualWriteLag.Count++
		dualWriteLag.LastErr = err.Error()
		dualWriteLag.LastAt = time.Now().UTC()
		dualWriteMu.Unlock()
	}
	return nil
}

// DualWriteLag —— 读「库落后量」快照（零值 = 从无落后）。供自检 / 诊断 / 对拍读。
func DualWriteLag() DualWriteLagReport {
	dualWriteMu.Lock()
	defer dualWriteMu.Unlock()
	return dualWriteLag
}

// ResetDualWriteLag —— 清零落后量（**测试接缝**：进程内累计量跨用例共享 ⇒ 用例开头归零）。
// ✗ 生产路径不调它（清了就读不出「历史落后过多少笔」）。
func ResetDualWriteLag() {
	dualWriteMu.Lock()
	dualWriteLag = DualWriteLagReport{}
	dualWriteMu.Unlock()
}
