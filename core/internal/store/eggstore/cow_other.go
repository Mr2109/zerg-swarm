//go:build !darwin && !linux

package eggstore

// 本平台（非 darwin、非 linux）**fail-closed**：clone 档、段档、copy_file_range 档
// 一律「不适用」⇒ 调用方退末档 `io.Copy`（真拷贝）；✗ 不假装判过。

// fallbackErrnos 本平台没有可判的 errno 白名单（无 unix 面）。
func fallbackErrnos() []error { return nil }

// platformCloneWhole 本平台无 clone ⇒ 不适用。
func platformCloneWhole(dst, src string) (CloneTier, error) { return "", ErrNoReflink }

// platformCloneRange 本平台无段级 reflink ⇒ 不适用。
func platformCloneRange(dst string, dstOff int64, src string, srcOff, srcLen int64) (CloneTier, error) {
	return "", ErrNoRangeClone
}

// platformCopyRange 本平台无 copy_file_range ⇒ 不适用。
func platformCopyRange(dst string, dstOff int64, src string, srcOff, srcLen int64) (int64, error) {
	return 0, ErrCopyRangeUnsupported
}

// hardlinkCount 本平台取不到硬链数（✗ 不假装）。
func hardlinkCount(path string) (uint64, error) { return 0, ErrNoVolumeFacts }
