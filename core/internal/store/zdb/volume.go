package zdb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// volumesRoot —— 外置卷挂载根（macOS = /Volumes）。只用于判「声称在卷下的路径是否真挂上」。
const volumesRoot = "/Volumes"

// volumeFacts —— 判卷所需的事实（由各平台的 readVolumeFacts 采集；纯函数 checkVolumeFacts 只吃事实）。
type volumeFacts struct {
	Path     string // 库路径
	Mount    string // 最近存在祖先所在的挂载点
	MountDev uint64 // 该挂载点的设备号
	BootDev  uint64 // 根卷设备号
	FSType   string // 空串 = 该平台未提供文件系统类型（跳过盘型档）
}

// localFSTypes —— 允许的本地文件系统（设计稿 §三.6 盘型附加约束：须 APFS / HFS+）。
var localFSTypes = map[string]bool{
	"apfs": true,
	"hfs":  true,
	"hfsx": true,
}

// networkFSTypes —— 网络文件系统（损坏 SQLite 的禁区）。
var networkFSTypes = map[string]bool{
	"nfs":    true,
	"smbfs":  true,
	"afpfs":  true,
	"cifs":   true,
	"webdav": true,
}

// fatFSTypes —— exFAT / FAT（macOS 实测 integrity_check 当场损坏）。
var fatFSTypes = map[string]bool{
	"exfat": true,
	"msdos": true,
	"fat":   true,
}

// isUnder 判 p 是否等于 root 或落在 root 之下（都先 Clean）。
func isUnder(p, root string) bool {
	p = filepath.Clean(p)
	root = filepath.Clean(root)
	if p == root {
		return true
	}
	return strings.HasPrefix(p, root+string(os.PathSeparator))
}

// checkVolumeFacts —— 硬规矩 ① 的纯判据（吃事实、不碰盘 ⇒ 可定向单测）。
//
// 三条：
//   - 网络盘 ⇒ 拒（设计稿 §三.6：网络文件系统是「会损坏数据库」的禁区）。
//   - exFAT/FAT ⇒ 拒（须 APFS / HFS+）。
//   - 声称在 <volumesRoot>/<卷> 下的路径，其设备号 == 根卷设备号 ⇒ 该卷**未挂载**
//     （那只是个内盘上的同名空目录）⇒ 拒启拒写，✗ 禁回退内盘新建空库。
func checkVolumeFacts(f volumeFacts, root string) error {
	if f.FSType != "" {
		ft := strings.ToLower(strings.TrimSpace(f.FSType))
		switch {
		case networkFSTypes[ft]:
			return fmt.Errorf("%w：库所在卷是网络文件系统（%s）—— 网络盘会损坏 SQLite（设计稿 §三.6）", ErrVolume, ft)
		case fatFSTypes[ft]:
			return fmt.Errorf("%w：库所在卷格式为 %s（须 APFS / HFS+；exFAT/FAT 实测损坏）", ErrVolume, ft)
		case !localFSTypes[ft]:
			return fmt.Errorf("%w：库所在卷格式 %s 不在允许集（须 APFS / HFS+）", ErrVolume, ft)
		}
	}
	if isUnder(f.Path, root) {
		if f.MountDev == f.BootDev {
			return fmt.Errorf("%w：%s 声称在 %s 下，但该路径落在**根卷**（卷未挂载）⇒ 拒启拒写（✗ 禁回退内盘新建空库）", ErrVolume, f.Path, filepath.Clean(root))
		}
		if !isUnder(f.Path, f.Mount) {
			return fmt.Errorf("%w：%s 不在其挂载点 %s 之下 ⇒ 卷未就绪", ErrVolume, f.Path, f.Mount)
		}
	}
	return nil
}

// nearestExistingDir —— 从 p 上溯到第一个真实存在的目录（库文件可能还没建）。
func nearestExistingDir(p string) (string, error) {
	d := filepath.Clean(p)
	if st, err := os.Stat(d); err == nil && !st.IsDir() {
		d = filepath.Dir(d)
	}
	for i := 0; i < 64; i++ {
		if st, err := os.Stat(d); err == nil {
			if st.IsDir() {
				return d, nil
			}
			d = filepath.Dir(d)
			continue
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", fmt.Errorf("%w：从 %s 上溯找不到任何存在的目录", ErrVolume, p)
}

// ensureBackupTargetDifferentVolume —— 硬规矩 ③：备份目标必须落在**另一卷**（✗ 与库同卷）。
func ensureBackupTargetDifferentVolume(dbPath, dst string) error {
	dbDir, err := nearestExistingDir(dbPath)
	if err != nil {
		return err
	}
	dstParent, err := nearestExistingDir(dst)
	if err != nil {
		return err
	}
	dbDev, err := devOf(dbDir)
	if err != nil {
		return fmt.Errorf("%w：探测库所在卷设备号失败（%s）: %v", ErrVolume, dbDir, err)
	}
	dstDev, err := devOf(dstParent)
	if err != nil {
		return fmt.Errorf("%w：探测备份目标卷设备号失败（%s）: %v", ErrVolume, dstParent, err)
	}
	if dbDev == dstDev {
		return fmt.Errorf("%w：库 %s 与备份目标 %s 同卷 ⇒ 同卷掉盘 = 库与备份一起没（设计稿 §三.6 硬规矩③）", ErrSameVolumeBackup, dbDir, dstParent)
	}
	return nil
}
