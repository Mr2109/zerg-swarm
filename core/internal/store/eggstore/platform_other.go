//go:build !darwin && !linux

package eggstore

// rawVolume 平台原始读数。
type rawVolume struct {
	bsize, blocks, bfree, bavail, files, ffree uint64
	mount, fstype                              string
}

// statfsRaw 本平台没有 `syscall.Statfs` ⇒ **fail-closed**：
// 返回 `ErrNoVolumeFacts`，让上层「取不到就不放行」，✗ 不许拿零值当「空间无限」。
func statfsRaw(path string) (rawVolume, error) { return rawVolume{}, ErrNoVolumeFacts }

// deviceOf 本平台取不到设备号 ⇒ 同卷判定一律判不了（✗ 不假装同卷）。
func deviceOf(path string) (uint64, error) { return 0, ErrNoVolumeFacts }

// platformAPFSProbe 本平台不适用。
func platformAPFSProbe(mount string) (*uint64, string, string) {
	return nil, "", ErrNoVolumeFacts.Error()
}
