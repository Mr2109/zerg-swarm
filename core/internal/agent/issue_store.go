package agent

// issue_store.go —— 任务/issue 态的**唯一落盘点**（缺口 `GAP-20260926-18` 第二步）
//
// 病（现读取证 · 2026-10-23）：写 issue 态的地方有 **6 处**裸 `os.WriteFile`：
//
//	worker.go:102 · worker.go:116 · worker.go:195 · worker.go:218（工人侧）
//	scheduler.go:414（调度器侧）· issue_tracker.go:239（状态机的家）
//
// ⇒ 谁最后写谁赢；写半截（断电/并发）也没有任何检测；非法流转想绕过就绕过。
//
// 治：本文件收成两个函数 ——
//
//	WriteIssueNew   首次落件（没有 from，不谈流转）
//	WriteIssueState 改态（**先过 `TransitionIssue`**：非法 ⇒ 返回错、**一个字节都不写**）
//
// 两者都走同一条「同目录临时件 + rename + 读回比对」的路：
//   - 同目录 rename 才原子（跨目录不保证）；
//   - fsync 后才 rename（内容先落稳，再换名）；
//   - 读回逐字节比对，不一致就报错（✗ 不许「写了就当成功」）。
//
// ★ 边界：本文件只收「任务/issue 态」的写点。`memory.go` / `ctx_unload.go` / `exec.go` /
// `loop.go` / `entry.go` / `idle_persist.go` 与 scheduler 里那 5 处**不写 issue 态**，不在本单范围。

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// atomicWriteSameDir —— 临时件 + fsync + rename + chmod（都在同一目录）。
//
// 失败时**清掉临时件**（✗ 留一地 `.tmp-*` 让下一次跑批踩到）；成功时临时件已随 rename 消失。
func atomicWriteSameDir(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("建临时件失败（%s）：%w", dir, err)
	}
	tmp := f.Name()
	drop := func() {
		_ = f.Close()
		_ = os.Remove(tmp)
	}
	if _, err := f.Write(data); err != nil {
		drop()
		return fmt.Errorf("写临时件失败（%s）：%w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		drop()
		return fmt.Errorf("临时件 fsync 失败（%s）：%w", tmp, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("关临时件失败（%s）：%w", tmp, err)
	}
	if err := os.Chmod(tmp, perm); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("临时件改权限失败（%s）：%w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename 失败（%s → %s）：%w", tmp, path, err)
	}
	return nil
}

// writeIssueVerified —— 落盘 + 读回逐字节比对。
func writeIssueVerified(path, content string) error {
	if err := atomicWriteSameDir(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("issue 落盘失败（%s）：%w", path, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("issue 读回失败（%s）：%w", path, err)
	}
	if !bytes.Equal(got, []byte(content)) {
		return fmt.Errorf("issue 读回不一致（%s）：写 %d B · 读回 %d B", path, len(content), len(got))
	}
	return nil
}

// WriteIssueNew —— 首次落件（`open` 态那一刻；没有 from，因此不谈流转）。
func WriteIssueNew(path, content string) error {
	return writeIssueVerified(path, content)
}

// WriteIssueDoc —— 已经算好的正文，只做「原子写 + 读回」。
//
// 给**迁移期**的老路径用：那些位置自己已经过了 `TransitionIssue`（拿到 `newContent`），
// 这里只把它从「裸 os.WriteFile」换成同一条安全落盘路，✗ 不重复校验（避免语义漂移）。
func WriteIssueDoc(path, content string) error {
	return writeIssueVerified(path, content)
}

// WriteIssueState —— 改态：**先过 `TransitionIssue`**（同态 / 非法流转都在那里判）。
//
// ★ 非法流转 ⇒ 返回错且**盘上一个字节都不动**（负控用例断言文件 sha256 不变）。
func WriteIssueState(path, content, from, to string) error {
	next, err := TransitionIssue(content, from, to)
	if err != nil {
		return fmt.Errorf("issue 改态被拒（%s → %s）：%w", from, to, err)
	}
	return writeIssueVerified(path, next)
}
