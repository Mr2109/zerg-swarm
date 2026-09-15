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
	// WorkReadWrite 工作目录 /work **存在且可写**（§6.6「一次性」侧：引擎要往那儿写缓存/日志/临时文件）。
	//
	// 与 /models 同严格（**必须见到、且不能是只读**）：孵化映射侧恒把 WorkDir 填成 /work、并把宿主
	// 每卵目录**可写**绑到那个落点上 ⇒ 见不到 /work（或它被挂成只读）就是那条绑定没生效，引擎的写入
	// 会落到只读的新根 / 上 —— 典型的「看着起来了，其实写不进去」。
	WorkReadWrite bool
	// KVDiskReadWrite KV 盘 /kvdisk 的可写性。缺省口径与 DataHidden 同风格：**没见到 /kvdisk 就是
	// 「该卵没有这个落点」**（不是每枚卵都有 KV 盘），见到却不可写 ⇒ **不符**（§9.7④ 的「跨孵化保留」
	// 侧写不进去 = 写盘静默失败，正是 §6.9 要防的形态）。
	KVDiskReadWrite bool
}

// Enclosed 全过才算「空间真的封闭」。
//
// 注意 /work 与 /kvdisk 两条的**不对称**（有意为之，不是漏写）：/work 必须出现且可写（每枚孵出来的卵
// 都有工作目录），/kvdisk 没出现即视为该卵无此落点（见字段注释）。
func (r EnclosureReport) Enclosed() bool {
	return r.ModelsReadOnly && r.DataHidden && r.HomeHidden && r.TmpIsTmpfs && r.NewPIDNamespace &&
		r.WorkReadWrite && r.KVDiskReadWrite
}

// String 人可读的一句结论（供日志与 /eggs 观测面）。
func (r EnclosureReport) String() string {
	verdict := "封闭性核验未通过"
	if r.Enclosed() {
		verdict = "封闭性核验通过"
	}
	return fmt.Sprintf("%s（models_ro=%v data_hidden=%v home_hidden=%v tmp_tmpfs=%v new_pidns=%v work_rw=%v kvdisk_rw=%v）",
		verdict, r.ModelsReadOnly, r.DataHidden, r.HomeHidden, r.TmpIsTmpfs, r.NewPIDNamespace,
		r.WorkReadWrite, r.KVDiskReadWrite)
}

// Failures 逐项列出**不符**的条目（供调用方写进拒孵理由：哪一项不符必须一眼看得见）。
//
// 顺序与字段声明同序、固定不变 ⇒ 同一份 report 每次给出同一段话（日志与用例都可复现）。
// 返回空切片 = 没有任何一项不符（即 Enclosed()，两者同源、不会各说各话）。
func (r EnclosureReport) Failures() []string {
	var out []string
	if !r.ModelsReadOnly {
		out = append(out, "/models 不是只读（宿主权重可被引擎改写）")
	}
	if !r.DataHidden {
		out = append(out, "宿主权重树 /data 在空间内可见")
	}
	if !r.HomeHidden {
		out = append(out, "宿主 /home 在空间内可见")
	}
	if !r.TmpIsTmpfs {
		out = append(out, "/tmp 不是空间内的 tmpfs")
	}
	if !r.NewPIDNamespace {
		out = append(out, "/proc 不是新的（进程视图未隔离）")
	}
	if !r.WorkReadWrite {
		out = append(out, "工作目录 /work 不是可写落点")
	}
	if !r.KVDiskReadWrite {
		out = append(out, "KV 盘 /kvdisk 不是可写落点")
	}
	return out
}

// optsHave 判定 mountinfo 的 options 字段（第 6 段，逗号分隔）里有没有某个**整词**选项。
// 不做子串匹配：「rw」不是「rwx」这样的前缀游戏，也不许把 "errors=remount-ro" 里的片段当选项。
func optsHave(opts, want string) bool {
	for _, o := range strings.Split(opts, ",") {
		if strings.TrimSpace(o) == want {
			return true
		}
	}
	return false
}

// mountWritable 该挂载点在 mountinfo 里是不是**可写**落点。
//
// 判据从严：**只要出现 ro 就按只读处理**（ro 与 rw 同时出现这种畸形不给好话）；没出现 ro 即视作可写
// —— 内核缺省就是可写，ro 才是要显式设上去的那个标记（bwrap 的 `--ro-bind` 才会 remount 成 ro）。
func mountWritable(opts string) bool { return !optsHave(opts, "ro") }

// mountinfoParseable 判定这段 mountinfo 文本「算不算读到了」：至少要解析得出一行。
//
// 为什么必须有它（§6.9 三态里的「读不到」，不是「不符」）：`/proc/<pid>/mountinfo` 读成功但内容
// 一行都认不得（空文件、被截断、格式全变）时，CheckEnclosure 会返回一份**零值报告**——那份报告
// 会被上层判成「实读且不符」。但真相是**没读到**，两件事的处置不同（不符 ⇒ 收卵拒孵；读不到 ⇒
// 只标未核验），所以「读到了但认不得」必须在读的那一层就按读不到报出来。
func mountinfoParseable(text string) error {
	if n := len(parseMountinfo(text)); n > 0 {
		return nil
	}
	return fmt.Errorf("mountinfo 一行都解析不出来（读到 %d 字节）——按「读不到」处理，不拿一份空报告当实读结论", len(text))
}

// CheckEnclosure 对给定 mountinfo 文本做核验（§6.9 的判据落地）。
// 缺省语义：**没在 mountinfo 里看到的东西就是"隐藏"**（空间内不可见）——所以 /data、/home
// 只有真出现才算漏；/models 必须出现且为 ro；/tmp 必须出现且是 tmpfs；
// /work 必须出现且可写；/kvdisk 没出现视为该卵无此落点（缺省可写），出现却只读才算漏。
// 进程视图那一项（NewPIDNamespace）由调用方按空间内实际进程数置位，本函数不猜。
func CheckEnclosure(mountinfo string) EnclosureReport {
	rep := EnclosureReport{
		DataHidden:      true, // 缺省隐藏；见到才算漏
		HomeHidden:      true,
		KVDiskReadWrite: true, // 缺省「没有这个落点」；见到且不可写才算漏
	}
	for _, e := range parseMountinfo(mountinfo) {
		switch {
		case e.MountPoint == "/models":
			rep.ModelsReadOnly = optsHave(e.Options, "ro")
		case e.MountPoint == "/data" || strings.HasPrefix(e.MountPoint, "/data/"):
			rep.DataHidden = false // 漏了：宿主权重树可见
		case e.MountPoint == "/home" || strings.HasPrefix(e.MountPoint, "/home/"):
			rep.HomeHidden = false
		case e.MountPoint == "/tmp":
			rep.TmpIsTmpfs = e.FSType == "tmpfs"
		case e.MountPoint == "/work":
			rep.WorkReadWrite = mountWritable(e.Options)
		case e.MountPoint == "/kvdisk":
			rep.KVDiskReadWrite = mountWritable(e.Options)
		}
	}
	return rep
}
