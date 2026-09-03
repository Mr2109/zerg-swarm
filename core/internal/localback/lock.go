package localback

import (
	"fmt"
	"log"
	"os"
	"syscall"
)

// B12 治本层：flock 文件锁 + PID 文件。
// 原则（用户 2026-08-11 确认）：
//   1. 一台机器一套 llama 服务（单例）
//   2. 主控启动 llama 时加锁——其他程序无法再启动
//   3. PID 文件 + 存活探测（进程重复侦测）
// 治本到位后治标（孤儿清理）不再需要——锁机制从源头杜绝多实例。

// InstanceLock 单实例锁（flock）。
// 持锁 = 该模型服务已在运行；进程退出（含崩溃）时 flock 自动释放。
type InstanceLock struct {
	file *os.File
	path string
}

// AcquireLock 获取单实例锁（非阻塞）。
// 已持有（其他进程在跑同模型服务）→ 返回已存在标记。
// 未持有 → 拿锁成功，调用方负责启动服务。
func AcquireLock(modelKey string) (*InstanceLock, bool, error) {
	path := fmt.Sprintf("/tmp/zerg-llama-%s.lock", modelKey)

	// 打开/创建锁文件（不删除——flock 锁跟文件描述符走，删文件会造成竞态）
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, false, fmt.Errorf("打开锁文件失败 %s: %w", path, err)
	}

	// 非阻塞拿锁
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// 拿不到锁 = 已有实例在跑
		f.Close()
		log.Printf("[localback] B12: 模型 %s 已有实例持有锁 (%s)，复用", modelKey, path)
		return nil, true, nil
	}

	return &InstanceLock{file: f, path: path}, false, nil
}

// Release 释放锁（进程退出/优雅关闭时调用）。
func (l *InstanceLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	l.file.Close()
	l.file = nil
}

// WritePidFile 写 PID 文件（进程重复侦测辅助）。
func WritePidFile(modelKey string, pid int) error {
	path := fmt.Sprintf("/tmp/zerg-llama-%s.pid", modelKey)
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", pid)), 0644)
}

// RemovePidFile 删除 PID 文件（优雅退出时）。
func RemovePidFile(modelKey string) {
	path := fmt.Sprintf("/tmp/zerg-llama-%s.pid", modelKey)
	os.Remove(path)
}

// CheckPidAlive 探测 PID 存活（signal 0：ESRCH=死，EPERM=活）。
func CheckPidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
