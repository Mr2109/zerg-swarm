// hatch_check.go —— 孵化后的**运行时封闭性核验**（设计 §6.9：静默失效不得当凭据）。
//
// 为什么必须有这一步：X3 实测里 systemd 的挂载类选项**报 `Result=success` 却零约束**——
// 账面"隔离好了"、实际什么都没有。所以凡声称"隔离生效"，一律**读 `/proc/<pid>/mountinfo`
// 实地核**，不得以单元状态为凭。
//
// 本文件是**纯函数**部分（可跨平台单测）；读 `/proc/<pid>/mountinfo` 在 hatch_linux.go。
package hatch

import (
	"fmt"
	"strings"
)

// mountEntry mountinfo 的一行（字段按 proc(5)：id parent major:minor root mountpoint opts ...）。
type mountEntry struct {
	MountPoint string
	FSType     string
	Options    string
}

// parseMountinfo 解析 /proc/<pid>/mountinfo（只取判断需要的三段：挂载点/文件系统类型/选项）。
func parseMountinfo(text string) []mountEntry {
	out := make([]mountEntry, 0, 32)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		// 字段 4 = mount point，5 = options，随后是可选 tags，再往后是 fstype
		mp, opts := f[4], f[5]
		fsType := ""
		for i := 6; i < len(f); i++ {
			if f[i] == "-" && i+1 < len(f) {
				fsType = f[i+1]
				break
			}
		}
		out = append(out, mountEntry{MountPoint: mp, FSType: fsType, Options: opts})
	}
	return out
}

// EnclosureReport 封闭性核验结果（每一项都要有据可查，便于写进日志/观测面）。
type EnclosureReport struct {
	ModelsReadOnly  bool // /models 存在且 **ro**
	DataHidden      bool // /data 不可见（宿主权重树没漏进来）
	HomeHidden      bool // /home 不可见
	TmpIsTmpfs      bool // /tmp 是空间内的 tmpfs（不是宿主 /tmp 同一份）
	NewPIDNamespace bool // /proc 是新的（进程视图隔离的间接证据）
}

// Enclosed 五项全过才算"空间真的封闭"。
func (r EnclosureReport) Enclosed() bool {
	return r.ModelsReadOnly && r.DataHidden && r.HomeHidden && r.TmpIsTmpfs && r.NewPIDNamespace
}

// String 人可读的一句结论（供日志与 /eggs 观测面）。
func (r EnclosureReport) String() string {
	verdict := "封闭性核验未通过"
	if r.Enclosed() {
		verdict = "封闭性核验通过"
	}
	return fmt.Sprintf("%s（models_ro=%v data_hidden=%v home_hidden=%v tmp_tmpfs=%v new_pidns=%v）",
		verdict, r.ModelsReadOnly, r.DataHidden, r.HomeHidden, r.TmpIsTmpfs, r.NewPIDNamespace)
}

// CheckEnclosure 对给定 mountinfo 文本做核验（§6.9 的判据落地）。
// 缺省语义：**没在 mountinfo 里看到的东西就是"隐藏"**（空间内不可见）——所以 /data、/home
// 只有真出现才算漏；/models 必须出现且为 ro；/tmp 必须出现且是 tmpfs。
// 进程视图那一项（NewPIDNamespace）由调用方按空间内实际进程数置位，本函数不猜。
func CheckEnclosure(mountinfo string) EnclosureReport {
	rep := EnclosureReport{
		DataHidden: true, // 缺省隐藏；见到才算漏
		HomeHidden: true,
	}
	for _, e := range parseMountinfo(mountinfo) {
		switch {
		case e.MountPoint == "/models":
			rep.ModelsReadOnly = strings.Contains(e.Options, "ro")
		case e.MountPoint == "/data" || strings.HasPrefix(e.MountPoint, "/data/"):
			rep.DataHidden = false // 漏了：宿主权重树可见
		case e.MountPoint == "/home" || strings.HasPrefix(e.MountPoint, "/home/"):
			rep.HomeHidden = false
		case e.MountPoint == "/tmp":
			rep.TmpIsTmpfs = e.FSType == "tmpfs"
		}
	}
	return rep
}
