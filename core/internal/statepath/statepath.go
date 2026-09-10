// statepath.go — 甲批 T2（2026-09-10 Mr2109拍板）：持久状态目录
// 背景：工具计数/事件流水/错误桶原先落 /tmp —— macOS 重启即清 + tmp_cleaner 3 天未访问即删（实测本机两者都在）。
// 规则：默认 ~/.zerg/state/（ZERG_STATE_DIR 可覆盖）；首次访问自动从旧 /tmp 路径搬一次（目标已存在则不覆盖）。
package statepath

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// Dir — 状态目录（ZERG_STATE_DIR 覆盖 → 默认 ~/.zerg/state）
func Dir() string {
	if d := os.Getenv("ZERG_STATE_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/zerg-state"
	}
	return filepath.Join(home, ".zerg", "state")
}

// File — 状态目录下的文件绝对路径
func File(name string) string { return filepath.Join(Dir(), name) }

// MigrateIfNeeded — 旧 /tmp 文件 → 状态目录（一次性；目标已存在则跳过；失败仅告警不阻断）
func MigrateIfNeeded(legacyPath, name string) string {
	dst := File(name)
	if _, err := os.Stat(dst); err == nil {
		return dst // 已有新文件——幂等
	}
	src, err := os.Stat(legacyPath)
	if err != nil {
		return dst // 无旧文件——用新路径（首次创建）
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		log.Printf("⚠️ 状态目录创建失败（继续用旧路径 %s）：%v", legacyPath, err)
		return legacyPath
	}
	if src.IsDir() {
		if err := copyDir(legacyPath, dst); err != nil {
			log.Printf("⚠️ 状态目录迁移失败（继续用旧路径 %s）：%v", legacyPath, err)
			return legacyPath
		}
	} else if err := copyFile(legacyPath, dst); err != nil {
		log.Printf("⚠️ 状态文件迁移失败（继续用旧路径 %s）：%v", legacyPath, err)
		return legacyPath
	}
	log.Printf("✅ 状态已迁出 /tmp：%s → %s（旧文件保留未删）", legacyPath, dst)
	return dst
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := copyDir(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
