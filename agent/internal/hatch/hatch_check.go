// hatch_check.go —— 孵化后的**运行时封闭性核验**（设计 §6.9：静默失效不得当凭据）。
//
// 为什么必须有这一步：X3 实测里 systemd 的挂载类选项**报 `Result=success` 却零约束**——
// 账面"隔离好了"、实际什么都没有。所以凡声称"隔离生效"，一律**读 `/proc/<pid>/mountinfo`
// 实地核**，不得以单元状态为凭。
//
// 权重那一项的判据（2026-09-15 收口，缺陷 12）：从「`/models` 存在且只读」改成
// **「每个声明的权重文件都以只读挂载出现」+「没有任何挂载点正好落在 `/models`」**。
// 关键事实（下一个人别搞错）：逐文件挂载之后 **`/models` 本身不再是挂载点**（它只是 bwrap `--dir`
// 在新根 tmpfs 上造的**空目录**）⇒ **mountinfo 里本来就没有 `/models` 这一条**。「看不到 /models
// 挂载项」是**本配方的正常形态，不是故障**；反过来，**看见**了 `mountpoint=/models` 才是问题
// （有人把目录整挂进来了 = 同目录别的模型一起进空间）。
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
	// PID 本次核验**实际读的是哪个进程**的 mountinfo（0 = 未记录）。
	//
	// 为什么必须落在报告里（2026-09-15 缺陷 2）：选错进程（读 bwrap 父进程 = 宿主视图）时，
	// 结论会「看起来很像一次核验」，而判据字符串里看不到 pid 就无从分辨「谁被核了」。
	// 所有结论话术都要能一眼看见它是按哪个 pid 得出的（见 String）。
	PID int
	// PIDSource 核验对象的来由（如「给定 pid=1379471 在宿主挂载命名空间，已改验空间内进程
	// pid=1379473」）——写进留痕，事故复盘时不必再猜。
	PIDSource string

	// ModelsReadOnly 权重那一项：**每个声明的权重文件都以只读挂载出现**，且**没有任何挂载点正好落在
	// `/models`**（判据实现见 modelsCriterion；口径与演进见文件头与本字段下方的实证字段）。
	//
	// 判据演进（2026-09-15 收口，缺陷 12）：原来这一项是「/models 存在且 ro」——那时 /models 是权重
	// **所在目录**的整挂载，看一条挂载点就够。现在权重是**逐文件**绑进 /models/<基名> 的，/models
	// 自己只是新根 tmpfs 上的**空目录**（bwrap `--dir`）⇒ mountinfo 里根本没有 /models 这一条
	// （**看不到它是正常形态，不是故障**），要核的是「那几个文件在不在、是不是 ro」+「有没有人把目录
	// 整挂进来」。只读也不够：整目录挂载即便 ro 也不符（只读说的是「不能改」，不是「只有这一份」）。
	ModelsReadOnly bool
	// ModelsFiles 判据的**实证**：mountinfo 里见到的逐文件只读挂载点（`/models/<基名>`，按挂载顺序）。
	// 「核的是哪些文件」在日志/留痕里就靠它（结论话术与不符理由都点名它）。
	ModelsFiles []string
	// ModelsRWFiles 落在 `/models/<基名>` 却**不是只读**的挂载点（一条都不该有：宿主权重可被引擎改写）。
	ModelsRWFiles []string
	// ModelsDirMounted 出现挂载点**正好**是 `/models` 的挂载（= 整目录挂载：同目录别的模型与宿主权重
	// 树会一起进空间，缺陷 12 的形态）。它即便 ro 也算不符。
	ModelsDirMounted bool
	// WeightsChecked 本次判据**实际核过**的空间内落点（`/models/<基名>`，按声明顺序）。
	//
	// 空 = 调用方没给声明清单（此时只核结构那半条：无整目录挂载 + 每个 /models/<…> 都 ro + 至少有一个
	// ——见 CheckEnclosure）；非空 ⇒ 清单里每一个落点都必须见到（判据文案据此点名「核的是哪些文件」）。
	WeightsChecked  []string
	DataHidden      bool // /data 不可见（宿主权重树没漏进来）
	HomeHidden      bool // 空间内没有宿主家目录**内容**（判据 = mountinfo 里没有 /home* 挂载）
	TmpIsTmpfs      bool // /tmp 是空间内的 tmpfs（不是宿主 /tmp 同一份）
	NewPIDNamespace bool // /proc 是新的（进程视图隔离的间接证据）
	// HomeMounts 在 mountinfo 里**实地见到**的 /home* 挂载点（判据的实证；空 = 一个都没有）。
	//
	// 为什么留这一列：HomeHidden 只表示「mountinfo 里没有 /home* 挂载」⇒「无宿主家目录**内容**」，
	// 而**不**表示「空间内没有 /home」—— X3 真机 2026-09-15 实测：空间里仍会出现引擎**自建**的
	// `/home/g01/.cache/comgr`（空目录，宿主家目录内容未泄露）。判据文本必须按「内容」说，
	// 否则下一个读日志的人会把「/home/g01 存在」误当成泄露（或反过来，把这条判据当成没用）。
	HomeMounts []string
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
//
// 必须带上**核验对象的 pid**（缺陷 2 的教训）：不写 pid，读日志的人无法分辨这份结论是按
// 空间内进程读的、还是误按 bwrap 父进程（宿主视图）读的 —— 后者会把每枚好卵判成不符。
// 另外带上权重那一项的**实证**（点了哪些文件）：「权重逐文件只读=/models/a.gguf、/models/b.gguf」
// 与「判据核的落点=…」让读日志的人一眼看出这次核的是什么，而不是只看到一个 models_ro=true。
func (r EnclosureReport) String() string {
	verdict := "封闭性核验未通过"
	if r.Enclosed() {
		verdict = "封闭性核验通过"
	}
	pidNote := "pid=未知（本报告没记核验对象）"
	if r.PID > 0 {
		pidNote = fmt.Sprintf("pid=%d", r.PID)
		if strings.TrimSpace(r.PIDSource) != "" {
			pidNote = fmt.Sprintf("pid=%d %s", r.PID, r.PIDSource)
		}
	}
	items := []string{
		pidNote,
		// models_ro 这个键名保留（读日志的人与观测面已按它认这一项）；它的含义已改成「逐文件只读
		// 且无整目录挂载」，实证跟着列在后面。
		fmt.Sprintf("models_ro=%v", r.ModelsReadOnly),
		fmt.Sprintf("data_hidden=%v", r.DataHidden),
		fmt.Sprintf("home_hidden=%v", r.HomeHidden),
		fmt.Sprintf("tmp_tmpfs=%v", r.TmpIsTmpfs),
		fmt.Sprintf("new_pidns=%v", r.NewPIDNamespace),
		fmt.Sprintf("work_rw=%v", r.WorkReadWrite),
		fmt.Sprintf("kvdisk_rw=%v", r.KVDiskReadWrite),
	}
	if ev := r.weightsEvidence(); ev != "" {
		items = append(items, ev)
	}
	return fmt.Sprintf("%s（%s）", verdict, strings.Join(items, " "))
}

// weightsEvidence 权重那一项在结论话术里的**实证**（点了哪几个文件、有没有整目录挂载）。
//
// 为什么要它：判据从「一条挂载点」变成「一组逐文件挂载点」之后，只说 models_ro=true 已经看不出
// 到底挂了谁（真机上一次 `ls /models` 才发现列的是整个目录）。把落点列出来，日志自证。
func (r EnclosureReport) weightsEvidence() string {
	var parts []string
	if len(r.ModelsFiles) > 0 {
		parts = append(parts, "权重逐文件只读="+strings.Join(r.ModelsFiles, "、"))
	}
	if len(r.ModelsRWFiles) > 0 {
		parts = append(parts, "权重非只读="+strings.Join(r.ModelsRWFiles, "、"))
	}
	if r.ModelsDirMounted {
		parts = append(parts, "⚠整目录挂载="+spaceModelsDir)
	}
	if len(r.WeightsChecked) > 0 {
		parts = append(parts, "判据核的落点="+strings.Join(r.WeightsChecked, "、"))
	}
	return strings.Join(parts, " ")
}

// MissingWeightFiles 声明要核的落点里，**没有以只读挂载出现**的那些（判据文案与观测面用它点名）。
// 没有声明清单（WeightsChecked 空）⇒ 返回空（没有可核的对象，不是「都缺」）。
func (r EnclosureReport) MissingWeightFiles() []string {
	if len(r.WeightsChecked) == 0 {
		return nil
	}
	got := make(map[string]bool, len(r.ModelsFiles))
	for _, f := range r.ModelsFiles {
		got[f] = true
	}
	var missing []string
	for _, w := range r.WeightsChecked {
		if !got[w] {
			missing = append(missing, w)
		}
	}
	return missing
}

// modelsCriterion 权重那一项的判据（四条，全都要成立）：
//
//	① 没有任何挂载点正好落在 `/models`：整目录挂载 ⇒ 同目录别的模型一起进空间（缺陷 12 的形态），
//	   **只读也不放过**（要的是「只有该卵点名的文件」，不是「不能改」）；
//	② 凡落在 `/models/<…>` 的挂载**全部**是只读（有一条 rw 就不符：宿主权重可被引擎改写）；
//	③ 至少见到一个 `/models/<…>` 的只读挂载（一个都没有 = 权重压根没挂进来，
//	   「看着起来了，其实读不到模型」——正是这一类静默失效）；
//	④ 给了声明清单时，清单里**每一个**落点都见到了（声明了却没挂 ⇒ 不符）。
//
// 说明：mountinfo **认不出「绑的是文件还是目录」**（两者都只是挂载点）⇒ ①②只能保证「没有落点正好在
// /models 上」，至于「逐个落下去的那些是文件」由映射侧的逐文件绑定用例钉住（bind 的宿主侧是哪个文件
// 只有那一层知道）。这条限制写在这里，免得下一个人以为本判据能证明一切。
func modelsCriterion(rep EnclosureReport) bool {
	if rep.ModelsDirMounted || len(rep.ModelsRWFiles) > 0 || len(rep.ModelsFiles) == 0 {
		return false
	}
	return len(rep.MissingWeightFiles()) == 0
}

// weightsReason 写清权重那一项不符的**判据与实证**：点了哪些文件、错在哪一条。
//
// 措辞校准（2026-09-15 收口）：这一项从「/models 不是只读」变成一组条件之后，一句含糊的
// 「/models 不对」会让读日志的人无从下手（去空间里 `ls /models`？那里本来就该是空的？）。
// 所以文案把**判据本身**写出来（每个声明的权重文件都是只读挂载、且没有整目录的 /models 挂载），
// 再逐条点名不符的形态与实证（哪些文件没见到、哪些不是只读、有没有整目录挂载）。
func (r EnclosureReport) weightsReason() string {
	var why []string
	if r.ModelsDirMounted {
		why = append(why, fmt.Sprintf("出现了挂载点正好落在 %s 的**整目录**挂载（同目录的别的模型与宿主权重树会一起进空间；"+
			"只读也不够——要的是「只挂该卵点名的文件」，不是「不能改」）", spaceModelsDir))
	}
	if len(r.ModelsRWFiles) > 0 {
		why = append(why, fmt.Sprintf("%s 不是只读挂载（宿主权重可被引擎改写）", strings.Join(r.ModelsRWFiles, "、")))
	}
	if missing := r.MissingWeightFiles(); len(missing) > 0 {
		why = append(why, fmt.Sprintf("声明的权重文件 %s 没有以只读挂载出现（实测见到的逐文件落点：%s）",
			strings.Join(missing, "、"), orNone(r.ModelsFiles)))
	}
	if len(why) == 0 {
		why = append(why, fmt.Sprintf("空间内见不到任何逐文件权重挂载（%s 下应当每个声明的权重文件各占一格只读挂载；"+
			"注意 %s **本身不是挂载点**，看不到它的挂载项是正常形态，不是故障）", spaceModelsDir, spaceModelsDir))
	}
	return fmt.Sprintf("权重挂载不符（判据=每个声明的权重文件都是只读挂载、且没有整目录的 %s 挂载）：%s",
		spaceModelsDir, strings.Join(why, "；"))
}

// orNone 空清单的人可读写法（「一个都没有」），免得日志里出现「实测见到的逐文件落点：」这种半截话。
func orNone(items []string) string {
	if len(items) == 0 {
		return "一个都没有"
	}
	return strings.Join(items, "、")
}

// Failures 逐项列出**不符**的条目（供调用方写进拒孵理由：哪一项不符必须一眼看得见）。
//
// 顺序与字段声明同序、固定不变 ⇒ 同一份 report 每次给出同一段话（日志与用例都可复现）。
// 返回空切片 = 没有任何一项不符（即 Enclosed()，两者同源、不会各说各话）。
func (r EnclosureReport) Failures() []string {
	var out []string
	if !r.ModelsReadOnly {
		out = append(out, r.weightsReason())
	}
	if !r.DataHidden {
		out = append(out, "宿主权重树 /data 在空间内可见")
	}
	if !r.HomeHidden {
		out = append(out, homeLeakReason(r.HomeMounts))
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

// homeLeakReason 写清 /home 这一项不符的**判据与实证**。
//
// 措辞校准（2026-09-15，缺陷 13）：这一项只表示「mountinfo 里出现了 /home* 挂载」⇒ 宿主家目录
// **内容**进了空间；**不**表示「空间里没有 /home」——空间里仍会有引擎自建的 /home/<user>/.cache
// （空目录）。写「/home 可见」会被读成后者，于是要么误报、要么把这条判据当成噪声。
func homeLeakReason(mounts []string) string {
	if len(mounts) == 0 {
		return "宿主家目录内容在空间内可见（判据：mountinfo 里出现了 /home* 挂载）"
	}
	return fmt.Sprintf("宿主家目录内容在空间内可见（判据：%s）", strings.Join(mounts, "、"))
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
// 只有真出现才算漏；/tmp 必须出现且是 tmpfs；/work 必须出现且可写；/kvdisk 没出现视为该卵无此
// 落点（缺省可写），出现却只读才算漏。
//
// 权重（2026-09-15 收口，判据见 modelsCriterion）：**每个声明的权重文件都要以只读挂载出现，且没有
// 任何挂载点正好落在 `/models`**。两点务必分清：
//
//   - **`/models` 本身不是挂载点**（bwrap `--dir` 在新根 tmpfs 上造的**空目录**）⇒ mountinfo 里
//     本来就没有 `/models` 这一条。**看不到它是本配方的正常形态，不是故障**（旧判据「/models 存在
//     且 ro」在这里会把每一枚好卵都判成不符 —— 那正是缺陷 12 收起时的坑）；
//   - 反过来，**看见** `mountpoint=/models` 才是问题：有人把目录整挂进来了（同目录别的模型一起进
//     空间），它即便 ro 也不符（只读 ≠ 只有这一份）。
//
// declaredWeightFiles 是**声明清单**（`Spec.WeightFiles` 原样传进来，宿主侧绝对路径），本函数按
// 落点规则换成空间内落点 `/models/<基名>` 再核（落点规则只有 spaceWeightLanding 一份实现）。
// 传空（nil）= 拿不到声明，此时只核**结构**那半条：无整目录挂载 + 每个 `/models/<…>` 都只读 +
// 至少见到一个（见 modelsCriterion）。**这是有意的降级，不是「不用核」**：真机上后端调
// `VerifyEnclosure(pid int)`，那个接口只有 pid，拿不到 Spec（本批不许改 backend）——要核到「正好
// 是哪几个声明文件」，得后端把 `spec.WeightFiles` 传进来（未接线，见 hatch.go 文件末）。
//
// 进程视图那一项（NewPIDNamespace）由调用方按空间内实际进程数置位，本函数不猜。
//
// **前提（缺陷 2）**：传进来的必须是**空间内进程**的 mountinfo。传 bwrap 父进程的
// （systemd MainPID、留在原命名空间）得到的是宿主视图 ⇒ 必然"不符"。选进程的判据见 hatch_pid.go。
//
// /home 那一项的口径见 HomeHidden 字段注释：判的是「没有宿主家目录**内容**」，
// 不是「空间里没有 /home 这个目录」（引擎会自建空目录）。
func CheckEnclosure(mountinfo string, declaredWeightFiles []string) EnclosureReport {
	rep := EnclosureReport{
		DataHidden:      true, // 缺省隐藏；见到才算漏
		HomeHidden:      true,
		KVDiskReadWrite: true, // 缺省「没有这个落点」；见到且不可写才算漏
	}
	// 声明清单 → 空间内落点（判据「核的是哪些文件」的直接来源）
	for _, f := range declaredWeightFiles {
		rep.WeightsChecked = append(rep.WeightsChecked, spaceWeightLanding(f))
	}
	for _, e := range parseMountinfo(mountinfo) {
		switch {
		case e.MountPoint == spaceModelsDir:
			// **整目录**挂载：只读也不符（缺陷 12 的形态）
			rep.ModelsDirMounted = true
		case strings.HasPrefix(e.MountPoint, spaceModelsDir+"/"):
			// 逐文件落点：只读才算数；落在 /models 之下却不是只读的单独记下来（文案要能点名）
			if optsHave(e.Options, "ro") {
				rep.ModelsFiles = append(rep.ModelsFiles, e.MountPoint)
			} else {
				rep.ModelsRWFiles = append(rep.ModelsRWFiles, e.MountPoint)
			}
		case e.MountPoint == "/data" || strings.HasPrefix(e.MountPoint, "/data/"):
			rep.DataHidden = false // 漏了：宿主权重树可见
		case e.MountPoint == "/home" || strings.HasPrefix(e.MountPoint, "/home/"):
			rep.HomeHidden = false
			rep.HomeMounts = append(rep.HomeMounts, e.MountPoint) // 实证：是哪一条挂载让这一项不符
		case e.MountPoint == "/tmp":
			rep.TmpIsTmpfs = e.FSType == "tmpfs"
		case e.MountPoint == "/work":
			rep.WorkReadWrite = mountWritable(e.Options)
		case e.MountPoint == "/kvdisk":
			rep.KVDiskReadWrite = mountWritable(e.Options)
		}
	}
	rep.ModelsReadOnly = modelsCriterion(rep)
	return rep
}
