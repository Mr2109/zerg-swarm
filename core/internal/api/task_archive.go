package api

// task_archive.go — 任务目录归档器（2026-08-20 设计——Mr2109）
// 生命周期: 活跃（执行中）→ 30 天归档（tar.gz 压缩）→ 90 天删除
// 归档索引: /var/zerg/archive/index.jsonl（可检索——UI 显示已归档）

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	tasksDir     = "/tmp/zerg-tasks"     // 任务目录（活跃——Mac 可写路径）
	archiveDir   = "/tmp/zerg-archive"   // 归档目录（压缩包）
	archiveIndex = "/tmp/zerg-archive/index.jsonl" // 归档索引
	archiveAfter = 30 * 24 * time.Hour   // 30 天归档（Mr2109）
	deleteAfter  = 90 * 24 * time.Hour   // 归档保留 90 天删除（Mr2109——GitHub 默认标准）
)

// RunTaskArchive 归档器——扫描任务目录:
// 1. 任务目录最后修改 > 30 天 → 归档（tar.gz 压缩 + 索引 + 删活跃目录）
// 2. 归档文件 > 90 天 → 删除（+ 索引标记）
func RunTaskArchive() {
	os.MkdirAll(archiveDir, 0o755)
	now := time.Now()
	archived := 0
	deleted := 0

	// 1. 归档过期任务目录
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirPath := filepath.Join(tasksDir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) >= archiveAfter {
			if archiveTaskDir(dirPath, e.Name()) {
				archived++
			}
		}
	}

	// 2. 删除过期归档（> 90 天）
	archEntries, err := os.ReadDir(archiveDir)
	if err == nil {
		for _, e := range archEntries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".gz" {
				info, err := e.Info()
				if err != nil {
					continue
				}
				if now.Sub(info.ModTime()) >= deleteAfter {
					os.Remove(filepath.Join(archiveDir, e.Name()))
					markIndexDeleted(e.Name())
					deleted++
				}
			}
		}
	}

	if archived > 0 || deleted > 0 {
		log.Printf("🗄️ 任务归档: %d 归档 + %d 删除（30 天归档/90 天删除——Mr2109）\n", archived, deleted)
	}
}

// archiveTaskDir 归档任务目录（tar.gz 压缩 + 索引 + 删活跃目录）
func archiveTaskDir(dirPath, taskID string) bool {
	archivePath := filepath.Join(archiveDir, taskID+".tar.gz")
	cmd := exec.Command("tar", "-czf", archivePath, "-C", tasksDir, taskID)
	if err := cmd.Run(); err != nil {
		log.Printf("⚠️ 归档失败 %s: %v\n", taskID, err)
		return false
	}
	// 写索引（摘要——任务名/时间/大小——可检索）
	info, _ := os.Stat(archivePath)
	indexEntry := map[string]interface{}{
		"task_id":   taskID,
		"archived":  time.Now().Format(time.RFC3339),
		"size":      info.Size(),
		"expires":   time.Now().Add(deleteAfter).Format(time.RFC3339),
	}
	if b, err := json.Marshal(indexEntry); err == nil {
		f, err := os.OpenFile(archiveIndex, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			f.Write(append(b, '\n'))
			f.Close()
		}
	}
	// 删活跃目录（归档后——省空间）
	os.RemoveAll(dirPath)
	log.Printf("🗄️ 任务 %s 归档完成（%d 字节）\n", taskID, info.Size())
	return true
}

// markIndexDeleted 索引标记删除（90 天归档删除）
func markIndexDeleted(archiveName string) {
	// 简单: 索引追加删除标记（不重写大索引——防 IO）
	taskID := archiveName[:len(archiveName)-len(".tar.gz")]
	f, err := os.OpenFile(archiveIndex, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(map[string]interface{}{
		"task_id": taskID,
		"deleted": time.Now().Format(time.RFC3339),
	})
	f.Write(append(b, '\n'))
}
