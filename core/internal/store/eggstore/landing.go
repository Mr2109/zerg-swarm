package eggstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var (
	// ErrCrossVolume 临时件与终名**不在同一卷**。
	// ★ 这条不是洁癖：`rename(2)` 的原子性**只在同一 mount 内**成立；临时件落了别的卷
	//（典型：落 `TMPDIR`，macOS 上常是另一个卷）⇒ `rename` 会**静默退化成 copy（非原子）**
	// ⇒ 「写临时名再换名」这套防线当场失效。⇒ 一律 fail-closed。
	ErrCrossVolume = errors.New("临时件与终名不在同一卷 ⇒ rename(2) 会静默退化成 copy（非原子）⇒ 拒")
	// ErrIncomplete 半成品（缺完工标记）。
	ErrIncomplete = errors.New("缺完工标记 ⇒ 半成品视为不可用")
)

// DoneMarkerName 完工标记件名（展开目录侧；★ 可用判据是**它的状态**，不是目录在不在）。
const DoneMarkerName = ".zerg-egg-done.json"

// LandOptions 落地参数。
type LandOptions struct {
	// ChunkSize 逐块校验的块大小（字节）。<=0 ⇒ 1 MiB。
	ChunkSize int
	// ExpectedBlocks 逐块期望 sha256（十六进制小写）。非空 ⇒ 逐块**必须**对上，
	// 对不上即 fail-closed（✗ 不「先落完再说」）。空 ⇒ 只做「写后回读」自校验。
	ExpectedBlocks []string
	// NoReadBack 关掉「写后回读」逐块自校验（默认开）。
	NoReadBack bool
	// TmpPath 显式指定临时件路径。缺省 ⇒ 与终名**同目录**的隐藏名。
	// ★ 给这个旋钮只为让「跨卷」这条负控能被**确定性**地造出来。
	TmpPath string
}

// LandResult 一次落地的读数。
type LandResult struct {
	Final       string `json:"final"`
	Tmp         string `json:"tmp"`
	Bytes       int64  `json:"bytes"`
	Blocks      int    `json:"blocks"`
	SHA256      string `json:"sha256"`
	Verified    int    `json:"verified"`
	DirSynced   bool   `json:"dir_synced"`
	ReadBack    bool   `json:"read_back"`
	CrossVolume bool   `json:"cross_volume"`
}

// TmpNameFor 终名的**同卷同目录**临时名（隐藏件）。
func TmpNameFor(dstPath string) string {
	return filepath.Join(filepath.Dir(dstPath), "."+filepath.Base(dstPath)+".part")
}

// LandFile 落地一枚卵：**写临时名（与终名同卷同目录）→ 逐块校验全过 → 原子 `rename`**，
// 最后 `fsync` 父目录抗掉电（macOS 上必需）。
//
// 半成品纪律（用户拍定）：任一步失败 ⇒ **临时件删掉、终名永不出现**；
// 于是「已落地的卵」与「半成品」在文件面上天然可分（✗ 不会出现半个 `<sha>.egg`）。
func LandFile(dstPath string, src io.Reader, opts LandOptions) (res LandResult, err error) {
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = 1 << 20
	}
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, err
	}
	tmpName := TmpNameFor(dstPath)
	if opts.TmpPath != "" {
		tmpName = opts.TmpPath
	}
	same, err := SameVolume(tmpName, dstPath)
	if err != nil {
		return res, fmt.Errorf("同卷判定失败（✗ 判不了就不落）：%w", err)
	}
	res.Tmp, res.Final, res.CrossVolume = tmpName, dstPath, !same
	if !same {
		return res, ErrCrossVolume
	}

	f, err := os.OpenFile(tmpName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return res, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(tmpName) // 半成品一律不留（终名下更不会有）
		}
	}()

	var ro *os.File
	if !opts.NoReadBack {
		ro, err = os.Open(tmpName)
		if err != nil {
			_ = f.Close()
			return res, err
		}
		defer func() { _ = ro.Close() }()
	}

	h := sha256.New()
	buf := make([]byte, opts.ChunkSize)
	back := make([]byte, opts.ChunkSize)
	var off int64
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			h.Write(chunk)
			if _, werr := f.Write(chunk); werr != nil {
				_ = f.Close()
				return res, fmt.Errorf("写临时件第 %d 块失败（%w）—— 半成品已弃，可重试", res.Blocks+1, werr)
			}
			got := sha256.Sum256(chunk)
			gotHex := hex.EncodeToString(got[:])
			if len(opts.ExpectedBlocks) > 0 {
				if res.Blocks >= len(opts.ExpectedBlocks) {
					_ = f.Close()
					return res, fmt.Errorf("源给的块数多于期望表（%d 块）", len(opts.ExpectedBlocks))
				}
				if want := opts.ExpectedBlocks[res.Blocks]; want != gotHex {
					_ = f.Close()
					return res, fmt.Errorf("第 %d 块 sha256 与期望不符（期望 %s · 实读 %s）⇒ 拒（✗ 不落成终名）",
						res.Blocks+1, want, gotHex)
				}
			}
			if ro != nil {
				if _, rerr := ro.ReadAt(back[:n], off); rerr != nil {
					_ = f.Close()
					return res, fmt.Errorf("回读第 %d 块失败（%w）—— 半成品已弃，可重试", res.Blocks+1, rerr)
				}
				gotBack := sha256.Sum256(back[:n])
				if hex.EncodeToString(gotBack[:]) != gotHex {
					_ = f.Close()
					return res, fmt.Errorf("第 %d 块回读不符（写盘后内容变了）⇒ 拒", res.Blocks+1)
				}
				res.Verified++
			}
			res.Blocks++
			res.Bytes += int64(n)
			off += int64(n)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = f.Close()
			return res, fmt.Errorf("读源第 %d 块失败（%w）—— 半成品已弃，可重试", res.Blocks+1, rerr)
		}
	}
	if len(opts.ExpectedBlocks) > 0 && res.Blocks != len(opts.ExpectedBlocks) {
		_ = f.Close()
		return res, fmt.Errorf("块数不符：期望 %d 块 · 实读 %d 块 ⇒ 拒", len(opts.ExpectedBlocks), res.Blocks)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return res, err
	}
	if err := f.Close(); err != nil {
		return res, err
	}
	if ro != nil {
		if err := ro.Close(); err != nil {
			return res, err
		}
		ro = nil
	}
	res.SHA256 = hex.EncodeToString(h.Sum(nil))
	res.ReadBack = !opts.NoReadBack

	// ★ 逐块校验全过 ⇒ 才换名（同卷同目录 ⇒ 原子）。
	if err := os.Rename(tmpName, dstPath); err != nil {
		return res, fmt.Errorf("原子 rename 失败（%w）—— 临时件已弃，可重试", err)
	}
	keep = true
	// 父目录 fsync：让 rename 本身落盘（抗掉电）。
	if d, derr := os.Open(dir); derr == nil {
		if serr := d.Sync(); serr == nil {
			res.DirSynced = true
		}
		_ = d.Close()
	}
	return res, nil
}

// DoneMarker 完工标记 —— 它是**状态**（含 sha / 块数 / 校验时刻），
// ✗ 不是「目录存在」这个事实（两者必须分开判，见 DirUsable）。
type DoneMarker struct {
	Version    int       `json:"version"`
	SHA256     string    `json:"sha256"`
	Blocks     int       `json:"blocks"`
	Bytes      int64     `json:"bytes"`
	VerifiedAt time.Time `json:"verified_at"`
}

// DoneMarkerPath 完工标记件路径。
func DoneMarkerPath(dir string) string { return filepath.Join(dir, DoneMarkerName) }

// WriteDoneMarker 写完工标记（原子 + fsync）。★ 空标记拒收（✗ 不许用空标记充「已完工」）。
func WriteDoneMarker(dir string, m DoneMarker) error {
	if m.Version == 0 {
		m.Version = 1
	}
	if m.SHA256 == "" || m.Blocks <= 0 {
		return fmt.Errorf("完工标记拒收：sha256 与块数必填（✗ 不许写空标记充数）：%s", DoneMarkerPath(dir))
	}
	if m.VerifiedAt.IsZero() {
		m.VerifiedAt = time.Now()
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(DoneMarkerPath(dir), append(b, '\n'))
}

// ReadDoneMarker 读完工标记。(marker, ok, err)：**ok=false 就是「不可用」**，
// 与「目录在不在」无关。
func ReadDoneMarker(dir string) (DoneMarker, bool, error) {
	b, err := os.ReadFile(DoneMarkerPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return DoneMarker{}, false, nil
		}
		return DoneMarker{}, false, err
	}
	var m DoneMarker
	if err := json.Unmarshal(b, &m); err != nil {
		return DoneMarker{}, false, fmt.Errorf("完工标记不可解析（%s）：%w", DoneMarkerPath(dir), err)
	}
	if m.SHA256 == "" || m.Blocks <= 0 {
		return m, false, fmt.Errorf("完工标记不完整（sha256/块数缺）⇒ ✗ 不当成可用：%s", DoneMarkerPath(dir))
	}
	return m, true, nil
}

// DirUsable ★ 可用判据是**完工标记这个状态**，✗ 不是「目录存在」。
//
// 反例（本判据存在的理由）：中断留下的半成品目录**照样在盘上**；拿 `os.Stat` 判
// 「在不在」会把半成品当可用件复用（设计 §16.3「完工标记是状态不是目录存在」）。
func DirUsable(dir string) (bool, string) {
	st, err := os.Stat(dir)
	if err != nil {
		return false, "目录不在盘"
	}
	if !st.IsDir() {
		return false, "路径不是目录"
	}
	_, ok, err := ReadDoneMarker(dir)
	if err != nil {
		return false, err.Error()
	}
	if !ok {
		return false, "缺完工标记（" + DoneMarkerName + "）—— ★ 目录存在 ≠ 可用：中断留下的半成品/半成品目录一律视为不可用"
	}
	return true, ""
}

// DirNote 人面一行（把两种判据的读数都摆出来，便于一眼看出我们没拿「存在」当可用）。
func DirNote(dir string) string {
	exists := "不在盘"
	if st, err := os.Stat(dir); err == nil {
		if st.IsDir() {
			exists = "在盘"
		} else {
			exists = "在盘（不是目录）"
		}
	}
	ok, why := DirUsable(dir)
	verdict := "可用"
	if !ok {
		verdict = "不可用（" + why + "）"
	}
	return fmt.Sprintf("%s：目录 %s · 判据(完工标记) %s", dir, exists, verdict)
}
