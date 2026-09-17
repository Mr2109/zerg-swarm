// effect_store.go —— 去重存储分层（E4 的"内存 LRU → 本地 KV → 服务端唯一约束"里**前两档**）。
//
// ── 两档各是什么 ────────────────────────────────────────────────────────────
//
//	① 内存档：*EffectLedger（进程内查表，纳秒级；进程一没，判断力就没了）
//	② 本地文件档：FileEffectLedger = 内存档 + **原子落盘的 JSONL 之外的整份快照**
//	   （落盘用"临时文件 + 改名"，半写的账本比没有账本更危险：它会被当成"确实没发生过"）
//
// ③ 服务端唯一约束那一档**不在本批**（见 trace.go 包尾"未接项"）——本包零网络依赖。
//
// ── 顺序口径：先落盘，后认领（跨崩溃面）───────────────────────────────────────
//
// ClaimPersisted 的顺序是"**先把这条认领写进文件**，再把 (EffectNew) 告诉调用方"。
// 落盘失败 ⇒ 撤销内存里的认领并报错 ⇒ 调用方**不得执行**。这条方向是刻意选的：
//
//	· 先认领后落盘（崩在中间）⇒ 文件里没有、内存里有 ⇒ 重启后重做一次 ⇒ 至少一次；
//	· 先落盘后认领（崩在中间）⇒ 文件里有、内存里没有 ⇒ 重启后判"已发生"，**效果不发生**。
//
// 两者都是"跨崩溃"的边界（进程内中断是 exactly-once，见 effects.go 文件头）。本档选后者，
// 因为"记不住就宁可当没发生过"比"没记住却当它发生了"更容易被人工发现与补偿（后者是静默少做）。
package replay

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SaveEffectLedger —— 原子落盘账本（临时文件 + 改名；半写的账本会被当成"没发生过"）。
//
// 逐条认领 = 每次全量重写，所以这一档适合"效果条数不大"的场景；批量场景请换成追加写的
// KV（属后续批，见包尾"未接项"）。序列化本身是稳定序（按 key 排序）⇒ 同样内容必然同样字节。
func SaveEffectLedger(path string, l *EffectLedger) error {
	if l == nil {
		// 「没有账本」与「空账本」是两件事：后者会把一切都判成"没发生过"。
		return fmt.Errorf("%w: 账本为 nil（拒绝把「没有账本」写成「空账本」）", ErrEffectLedger)
	}
	b, err := l.MarshalLedger()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%w: 建目录 %s：%v", ErrEffectLedger, filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("%w: 写账本临时件 %s：%v", ErrEffectLedger, tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("%w: 落账本 %s：%v", ErrEffectLedger, path, err)
	}
	return nil
}

// LoadEffectLedgerFrom —— 从文件读账本。第二个返回值 = "文件是否存在"（不存在 ≠ 错误：首次运行）。
//
// 严格：文件在但内容坏了 ⇒ 报错（账本坏掉 = 幂等判断全错，属"宁可起不来"的那一类），
// 绝不静默退化成空账本 —— 那会把已经做过的效果全部当成没做过。
func LoadEffectLedgerFrom(path string) (*EffectLedger, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewEffectLedger(), false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("%w: 读账本 %s：%v", ErrEffectLedger, path, err)
	}
	l, err := LoadEffectLedger(b)
	if err != nil {
		return nil, false, err
	}
	return l, true, nil
}

// FileEffectLedger —— **内存 + 本地文件**两档合成。
//
// 用法（恢复路径）：
//
//	f, err := OpenFileEffectLedger(path)          // 老账本在 ⇒ 读回来；不在 ⇒ 空账本
//	claim, entry, err := f.ClaimPersisted(e, sh)  // 落盘成功才返回 new ⇒ 这时才可以执行
//	... 执行 ...
//	err = f.CompletePersisted(e.Key, afterHash)   // 记执行后状态（E5）
type FileEffectLedger struct {
	*EffectLedger
	path string
	mu   sync.Mutex
}

// OpenFileEffectLedger —— 打开（或新建）文件档账本。
func OpenFileEffectLedger(path string) (*FileEffectLedger, error) {
	l, _, err := LoadEffectLedgerFrom(path)
	if err != nil {
		return nil, err
	}
	return &FileEffectLedger{EffectLedger: l, path: path}, nil
}

// Path —— 落盘路径。
func (f *FileEffectLedger) Path() string { return f.path }

// 编译期断言：两档都满足判重/补做需要的能力面（EffectStore，见 resume.go）。
// 任一档缺方法（比如以后有人改了签名）会在这里编译失败，而不是在补做路径上才发现。
var (
	_ EffectStore = (*EffectLedger)(nil)
	_ EffectStore = (*FileEffectLedger)(nil)
)

// Flush —— 把内存档整份原子落盘（重试安全：每次写的是全量快照）。
func (f *FileEffectLedger) Flush() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return SaveEffectLedger(f.path, f.EffectLedger)
}

// ClaimPersisted —— 认领 + 前置状态校验（E5）+ **先落盘**。
//
// 返回 (EffectNew, …) 之后才允许执行；落盘失败 ⇒ 撤销内存认领并返回错误（不得执行）。
// 已发生过时沿用 ClaimChecked 的语义：状态相符才回"已发生"，不符报 *StateMismatchError。
func (f *FileEffectLedger) ClaimPersisted(e EffectEntry, currentStateHash string) (EffectClaim, EffectEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	claim, entry, err := f.EffectLedger.ClaimChecked(e, currentStateHash)
	if err != nil || claim == EffectAlreadyApplied {
		return claim, entry, err
	}
	if err := SaveEffectLedger(f.path, f.EffectLedger); err != nil {
		if uerr := f.EffectLedger.Release(entry.Key); uerr != nil {
			return EffectClaim(""), entry, fmt.Errorf(
				"%w: 认领落盘失败（%v）且撤销内存认领也失败（%v）—— 账本已不可信，请人工介入，不要继续执行",
				ErrEffectLedger, err, uerr)
		}
		return EffectClaim(""), entry, err
	}
	return EffectNew, entry, nil
}

// CompletePersisted —— 记执行后状态（E5）+ 落盘。
//
// 落盘失败 ⇒ 返回错误，但**内存里已经记下了** observed_after_hash：重试请直接调 Flush()
// （写的是全量快照 ⇒ 重试安全）。这条边界如实写在这里：此刻"做过了但盘上没记"，
// 而 next start 会按"没记 = 没做过"处理 ⇒ 调用方必须重试 Flush，不能把错误吞掉。
func (f *FileEffectLedger) CompletePersisted(key, observedAfterHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.EffectLedger.Complete(key, observedAfterHash); err != nil {
		return err
	}
	if err := SaveEffectLedger(f.path, f.EffectLedger); err != nil {
		return fmt.Errorf("%w（内存已记、盘上未记：请重试 Flush —— 全量快照，重试安全）", err)
	}
	return nil
}

// Reload —— 从文件重新加载（外部进程可能写过同一份账本）。
//
// 如实说：这会**丢掉**内存里还没落盘的东西。本档的顺序口径保证"每条认领在返回前都已落盘"，
// 故只剩"CompletePersisted 落盘失败"那一种缺口 —— 那种情况下请先 Flush 再 Reload。
func (f *FileEffectLedger) Reload() error {
	l, _, err := LoadEffectLedgerFrom(f.path)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.EffectLedger = l
	return nil
}
