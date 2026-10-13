package zdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backup —— 硬规矩 ③：备份落**另一卷**（内盘 ~/.zerg/backup/），VACUUM INTO 落紧凑单文件快照。
// 同卷 ⇒ 拒（同卷掉盘 = 库与备份一起没）。
func (d *DB) Backup(dst string) error { return d.backupTo(dst, false) }

// backupTo —— 真备份动作。skipVolumeGuard=true 只供同包测试验「VACUUM INTO 真能产快照」。
func (d *DB) backupTo(dst string, skipVolumeGuard bool) error {
	if d == nil || d.db == nil {
		return errors.New("zdb: 库未打开")
	}
	if strings.TrimSpace(dst) == "" {
		return errors.New("zdb: 备份目标路径为空")
	}
	if !skipVolumeGuard {
		if err := ensureBackupTargetDifferentVolume(d.path, dst); err != nil {
			return err
		}
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("zdb: 备份目标已存在（%s）—— VACUUM INTO 要求目标不存在", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("zdb: 建备份目录失败: %w", err)
	}
	// 备份前先 checkpoint，把 -wal 里的已提交页并入主库（快照才完整且小）。
	if err := d.CheckpointTruncate(); err != nil {
		return err
	}
	q := "VACUUM INTO '" + strings.ReplaceAll(dst, "'", "''") + "'"
	if _, err := d.db.Exec(q); err != nil {
		return fmt.Errorf("zdb: VACUUM INTO 快照失败: %w", err)
	}
	return nil
}

// BackupDefault —— 落 <BackupDir()>/zerg-<时间戳>.db（默认 = 内盘 ~/.zerg/backup/，与库不同卷）。
func (d *DB) BackupDefault() (string, error) {
	dir := BackupDir()
	if dir == "" {
		return "", errors.New("zdb: 取不到家目录，无法定备份落点")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("zdb: 建备份目录失败: %w", err)
	}
	dst := filepath.Join(dir, fmt.Sprintf("zerg-%s.db", time.Now().Format("20060102-150405")))
	if err := d.Backup(dst); err != nil {
		return "", err
	}
	return dst, nil
}
