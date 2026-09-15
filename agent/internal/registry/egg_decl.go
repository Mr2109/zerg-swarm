package registry

import (
	"fmt"
	"sort"
	"strings"
)

// ═══ 虫卵声明（P1）═══════════════════════════════════════════════════════════
//
// 设计真源：docs/01-设计/设计-子端沙箱化-20260914.md
//   §4.3 卵声明字段表（九项）· §6.7 环境供给五类需求与三个真坑
//   §6.8 子端升级与虫卵（含 schema_version）· §13 Q21 空窗收走阈值
//
// 三条口径（写死，不得简化）：
//  ① 虫卵不「理解」引擎——它只给一张能力菜单，引擎要什么**由卵声明**；
//     「加新引擎 = 加一份声明，不改孵化器」（§6.7 口径句）。
//  ② 认不得的声明格式版本 ⇒ **明确报错、拒孵**，不许静默按新格式跑错
//     （静默跑错是「服务起得来、行为不对」，比起不来糟得多，§6.8.4 / §6.9）。
//  ③ 「默认空」无例外——**没有「常驻卵」**，只有每枚卵各自的「空窗收走阈值」（§13 Q21）。
//
// 失败语义分两档（对齐本仓既有的 config.ValidationResult 体例）：
//   - Fatal    ⇒ **拒孵**，错误原样报出去；
//   - Warnings ⇒ 照孵，但**必须报出去**（日志），绝不许静默按缺省值跑（§6.9 第 3 条）。

// EggSchemaVersionCurrent 当前子端认得（并会孵）的卵声明格式版本号。
//
// 设计 §6.8.4：卵是**制品**、可跨子端版本长期躺在盘上 ⇒「新旧格式共存」是必然而非意外；
// 「认不认得」的判定发生在**孵化之前**（与准入闸门同一处：先判格式，再算账）。
const EggSchemaVersionCurrent = 1

// DefaultIdleUnloadSeconds 空窗收走阈值缺省值（秒）——设计 §4.3 第九项 / §13 Q21。
// 「默认空」无例外：一律收走，差别只在阈值长短。
const DefaultIdleUnloadSeconds = 600

// SmallModelIdleUnloadSeconds 小模型（嵌入 / 重排 / 分类类）的建议空窗收走阈值（秒）。
// 用户口径（Mr2109 2026-09-15）：「因为是小模型，需则装，用完就卸，所以，他们的卸时间可以短点 2 分钟」。
const SmallModelIdleUnloadSeconds = 120

// EnvReq 卵声明的「环境需求」（设计 §4.3 第七项 / §6.7）——孵化器**只照单执行**：
// 挂哪个库目录、给哪个 LD_LIBRARY_PATH、放哪张卡、开多大 RLIMIT_MEMLOCK / vm.max_map_count，
// 一律不许自己推断（不许按引擎名分支）。
//
// 三个真坑（§6.7 C，写进必填要求）：
//
//	① **多卡必须声明用哪张卡**（引擎会挑错卡）⇒ Devices；
//	② **大模型 mmap 限额给不足直接崩**（不是慢）⇒ MemlockKB / MmapMaxCount
//	   ⚠ 其中 **MmapMaxCount 暂不生效**（2026-09-15 第一枚卵真机实测缺陷 11 登记）：它是**机器级
//	   sysctl**（`vm.max_map_count`）、不是按进程的 rlimit，孵化单元里没有可下发的落点 ——
//	   声明它目前只是**声明完整性**（缺项仍拒孵），**不是保障**，别读成「已经管住了」；
//	③ **库版本冲突（K2 真实案例）**：同机两个引擎要不同版本的同一个库
//	   ⇒ LibPaths 给引擎自己的库目录（孵化器据它只读落到 `/libs/<目录名>` **并生成空间内
//	   `LD_LIBRARY_PATH`**）；**要清空**时用 Env 里的 `LD_LIBRARY_PATH: ""`
//	   显式清空（K2 的包装脚本正是 `exec env -u LD_LIBRARY_PATH`，附录 A.2 / 附录 C·C4）。
type EnvReq struct {
	// Devices 设备与卡号：要哪张卡（HIP_VISIBLE_DEVICES / ROCR_VISIBLE_DEVICES / CUDA_VISIBLE_DEVICES）。
	// 单卡机器可留空（无从挑错卡）。
	Devices []string `yaml:"devices,omitempty"`
	// LibPaths 库路径与版本：引擎自己的库目录（.so 所在目录）。
	// 孵化器据它给出 LD_LIBRARY_PATH：每个目录**只读**落到空间内 `/libs/<目录名>`（独立只读挂载点，
	// 不再嵌在只读的 /engine 之下 —— 2026-09-15 真机实测缺陷 5），并**按声明顺序生成**空间内
	// `LD_LIBRARY_PATH`（`/libs/<名>`… 在前、`/engine` 垫尾；卵在 Env 里显式写了该变量就以卵的为准）；
	// **库按卵给，不许「全机一套」**（§6.7 C③）。
	LibPaths []string `yaml:"lib_paths,omitempty"`
	// Env 环境变量：HSA_* / HIP_* / CUDA_* / OMP_NUM_THREADS，
	// 以及按引擎覆盖 LD_LIBRARY_PATH（值为空串 = 显式清空，合法且必须显式写出来）。
	Env map[string]string `yaml:"env,omitempty"`
	// Weights 权重路径：只读入卵的权重根（宿主一份、page cache 一份，§9.3）。
	Weights []string `yaml:"weights,omitempty"`
	// MemlockKB RLIMIT_MEMLOCK 限额（KB）——给不足直接崩，孵化器不许猜（§6.7 C②）。
	MemlockKB int `yaml:"memlock_kb,omitempty"`
	// MmapMaxCount vm.max_map_count 限额（映射区段数上限）——同上，给不足直接崩。
	//
	// ⚠ **暂不生效**（2026-09-15 第一枚卵真机实测缺陷 11，二选一里取「明说暂不生效」这一支）：
	// 它是**机器级 sysctl**，不是按进程的 rlimit —— 孵化单元里下发不了（`sysctl -w` 要 root、
	// 一改全机生效，systemd 也没有对应属性），孵化声明（hatch.Spec）里同样没有承载它的字段
	// ⇒ 当前**没有任何执行点**读它。**缺项仍然拒孵**（声明完整性），但那不等于保障：
	// 要真生效得另做机器级前置校验（与 `/proc/sys/vm/max_map_count` 比、不够就拒孵），先拍板。
	MmapMaxCount int `yaml:"mmap_max_count,omitempty"`
}

// 卡号类环境变量（§6.7 C①：多卡必须声明用哪张卡）。
// 判据只认这三个名字；声明了却给空值 ⇒ 等于没声明 ⇒ Fatal。
var cardEnvKeys = []string{"HIP_VISIBLE_DEVICES", "ROCR_VISIBLE_DEVICES", "CUDA_VISIBLE_DEVICES"}

// EggDeclarationResult 卵声明校验结果（Fatal ⇒ 拒孵；Warnings ⇒ 照孵但必须报出去）。
type EggDeclarationResult struct {
	// Fatal 拒孵理由（空 = 可以孵）。
	Fatal []string
	// Warnings 照孵但必须报出去的问题（绝不静默）。
	Warnings []string
}

// OK 是否可以孵（无 Fatal 即可）。
func (r EggDeclarationResult) OK() bool { return len(r.Fatal) == 0 }

// ErrorString 把 Fatal 拼成一行可读文案（报错用；无 Fatal 时返回空串）。
func (r EggDeclarationResult) ErrorString() string { return strings.Join(r.Fatal, "；") }

// WarningString 把 Warnings 拼成一行可读文案（日志用；无告警时返回空串）。
func (r EggDeclarationResult) WarningString() string { return strings.Join(r.Warnings, "；") }

// IdleUnloadSeconds 这枚卵的「空窗收走阈值」（秒；设计 §4.3 第九项 / §6.5 / §13 Q21）。
// 未声明（<=0）时取缺省 DefaultIdleUnloadSeconds（600s）；小模型建议 120s。
// ⚠ 缺省值只对**遗留条目**成立：卵清单（集群级真源）里必须**显式声明**（§4.3 ②-补 第 ① 条）。
func (e *ModelEntry) IdleUnloadSeconds() int {
	if e == nil || e.IdleUnloadS <= 0 {
		return DefaultIdleUnloadSeconds
	}
	return e.IdleUnloadS
}

// EggName 这枚卵的名字（注册表键；load/Reload 时由注册表盖进条目）。
// 手工构造的条目（单测/临时孵化）可能没有名字 ⇒ 返回空串，调用方自行决定回退口径。
// 用途：KV 盘「按卵分目录」的目录名（设计 §9.7④：~/.zerg/kvdisk/<卵名>/）。
func (e *ModelEntry) EggName() string {
	if e == nil {
		return ""
	}
	return e.name
}

// SetEggNameForTest 测试专用：给手工构造的条目盖上卵名（生产路径由注册表盖，见 load/Reload）。
func (e *ModelEntry) SetEggNameForTest(name string) {
	if e != nil {
		e.name = name
	}
}

// Declared 这枚卵是否声明了新格式的卵声明字段（三者任一）。
// 三者全空 = 遗留条目（P1 之前写的注册表，如 X3 本机的 agent_models.yaml）——
// 按告警处理而不是拒孵：本仓读不到那份文件，清单落地（P7）时才由清单侧强制补齐。
func (e *ModelEntry) Declared() bool {
	return e != nil && (e.SchemaVersion != 0 || e.EnvReq != nil || e.IdleUnloadS != 0)
}

// ValidateEggDeclaration 校验一枚卵的声明（**孵化前**调用；设计 §6.8.4「先判格式，再算账」）。
//
// name 只用于报错定位——设计要求「说清是哪一枚卵 / 期望的 schema_version / 实际的 schema_version」。
// 返回 Fatal ⇒ 调用方**必须拒孵**并把 Fatal 原文报出去；Warnings ⇒ 照孵但必须进日志。
func ValidateEggDeclaration(name string, entry *ModelEntry) EggDeclarationResult {
	var res EggDeclarationResult
	if entry == nil {
		res.Fatal = append(res.Fatal, fmt.Sprintf("卵 %s：声明为空（注册表里没有条目）", displayName(name)))
		return res
	}

	// ===== 1) schema_version：声明格式版本（§4.3 第八项 / §6.8.4）=====
	switch {
	case entry.SchemaVersion < 0:
		res.Fatal = append(res.Fatal, fmt.Sprintf(
			"卵 %s：schema_version=%d 不合法（声明格式版本号不能为负；期望 %d）",
			displayName(name), entry.SchemaVersion, EggSchemaVersionCurrent))
	case entry.SchemaVersion > 0 && entry.SchemaVersion != EggSchemaVersionCurrent:
		// 认不得的版本 ⇒ 明确报错、拒孵（不许静默按新格式跑错）。
		res.Fatal = append(res.Fatal, fmt.Sprintf(
			"卵 %s：认不得的声明格式版本 schema_version=%d（本子端期望 %d）——拒孵；"+
				"请按本子端认得的格式重写这枚卵的声明，不要用新格式的卵去跑",
			displayName(name), entry.SchemaVersion, EggSchemaVersionCurrent))
	case entry.SchemaVersion == 0:
		// 未声明（0 = 缺省零值）——遗留条目。**不是静默**：每次都报一条告警，且写明补齐期限。
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"卵 %s：未声明 schema_version（卵声明格式版本号，设计 §4.3 第八项）——"+
				"按遗留条目继续；清单落地（P7）时必须显式声明为 %d，届时未声明即拒孵",
			displayName(name), EggSchemaVersionCurrent))
	}

	// ===== 2) env_req：环境需求（§4.3 第七项 / §6.7）=====
	if entry.EnvReq == nil {
		if !entry.Declared() {
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"卵 %s：未声明任何卵声明字段（schema_version / env_req / idle_unload_s）——"+
					"按遗留条目继续：环境需求缺省为空（孵化器不注入任何引擎专属的库路径 / 卡号 / 限额），"+
					"空窗收走阈值取缺省 %ds；清单落地（P7）时必须补齐",
				displayName(name), DefaultIdleUnloadSeconds))
		}
	} else {
		res.Fatal = append(res.Fatal, envReqFatalReasons(name, entry.EnvReq)...)
	}

	// ===== 3) idle_unload_s：空窗收走阈值（§4.3 第九项 / §13 Q21）=====
	switch {
	case entry.IdleUnloadS < 0:
		res.Fatal = append(res.Fatal, fmt.Sprintf(
			"卵 %s：idle_unload_s=%d 不合法（空窗收走阈值不能为负；缺省 %d、小模型建议 %d）",
			displayName(name), entry.IdleUnloadS, DefaultIdleUnloadSeconds, SmallModelIdleUnloadSeconds))
	case entry.IdleUnloadS == 0 && entry.EnvReq != nil:
		// 已按新格式声明却又没给阈值 ⇒ 不允许隐式「自己决定」（§4.3 ②-补 第 ① 条）。
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"卵 %s：未显式声明 idle_unload_s（空窗收走阈值）——取缺省 %ds；"+
				"小模型（嵌入 / 重排 / 分类类）建议 %ds（设计 §13 Q21）",
			displayName(name), DefaultIdleUnloadSeconds, SmallModelIdleUnloadSeconds))
	}

	// ===== 4) kv_disk：KV 盘声明（P5，设计 §9.4 / §9.7）=====
	// 默认关闭（nil = 不发 --kv-disk-*）；一旦声明，写盘上限就是硬要求——
	// KV 落盘是持续写，上限不许留成「无限」（§9.7①），负数更是声明错误。
	switch {
	case entry.KVDisk != nil && entry.KVDisk.SpaceMB <= 0:
		res.Fatal = append(res.Fatal, fmt.Sprintf(
			"卵 %s：kv_disk.space_mb=%d 不合法（KV 盘写盘上限必须为正——声明了 KV 盘却不限大小，"+
				"等于把持续写敞成无限，设计 §9.7①）",
			displayName(name), entry.KVDisk.SpaceMB))
	case entry.KVDisk != nil && entry.KVDisk.SpaceMB > 0 && entry.KVDisk.Dir == "" && entry.EggName() == "":
		// 显式覆盖目录给得出就不需要卵名；两者都没有 ⇒ 按卵分目录无从落地 ⇒ 拒孵。
		res.Fatal = append(res.Fatal, fmt.Sprintf(
			"卵 %s：kv_disk 未声明目录且卵名缺失——按卵分目录（~/.zerg/kvdisk/<卵名>/，设计 §9.7④）无从落地；"+
				"请补 kv_disk.dir 或让注册表带上条目名",
			displayName(name)))
	case entry.SsdStreamingPreloadExperts < 0:
		res.Fatal = append(res.Fatal, fmt.Sprintf(
			"卵 %s：ssd_streaming_preload_experts=%d 不合法（预热专家数不能为负；"+
				"无实测档案就不声明，适配器自然不发该参数——无档案不预热，设计 §9.4 / §8.4 标定铁律）",
			displayName(name), entry.SsdStreamingPreloadExperts))
	}

	return res
}

// envReqFatalReasons 环境需求的必填项检查（缺必填项 ⇒ 报错，不许静默跑缺省值）。
func envReqFatalReasons(name string, req *EnvReq) []string {
	var missing []string
	if len(req.Weights) == 0 {
		missing = append(missing, "权重路径（weights）——卵只挂它自己声明的权重，孵化器不许猜路径")
	}
	if len(req.LibPaths) == 0 {
		missing = append(missing, "库路径与版本（lib_paths）——库按卵给，不许「全机一套」")
	}
	if req.MemlockKB <= 0 {
		missing = append(missing, "RLIMIT_MEMLOCK 限额（memlock_kb）——给不足直接崩，不是变慢")
	}
	if req.MmapMaxCount <= 0 {
		missing = append(missing, "vm.max_map_count 限额（mmap_max_count）——同上，给不足直接崩")
	}
	var reasons []string
	if len(missing) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"卵 %s：环境需求 env_req 缺必填项——%s（设计 §6.7 C；孵化器只照单执行，缺项即拒孵，不得按缺省值跑）",
			displayName(name), strings.Join(missing, "；")))
	}
	// 卡号类变量「声明了但为空」= 等于没声明（§6.7 C①：多卡必须声明用哪张卡）。
	var emptyCardVars []string
	for _, k := range cardEnvKeys {
		if v, ok := req.Env[k]; ok && strings.TrimSpace(v) == "" {
			emptyCardVars = append(emptyCardVars, k)
		}
	}
	if len(emptyCardVars) > 0 {
		sort.Strings(emptyCardVars)
		reasons = append(reasons, fmt.Sprintf(
			"卵 %s：环境需求 env_req.env 里 %s 为空值——声明了等于没声明（多卡必须声明用哪张卡，设计 §6.7 C①）；"+
				"要「清空」请只对 LD_LIBRARY_PATH 这类库搜索变量用空串",
			displayName(name), strings.Join(emptyCardVars, " / ")))
	}
	return reasons
}

// displayName 报错定位用（空名给一个可读占位，别让日志出现「卵 ：」这种半截话）。
func displayName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "(未命名)"
	}
	return name
}

// ValidateEgg 按模型名取条目并校验卵声明（孵化前校验的注册表入口）。
// 返回 Fatal ⇒ 拒孵；Warnings ⇒ 照孵但必须报出去。
func (r *Registry) ValidateEgg(name string) EggDeclarationResult {
	if r == nil {
		return EggDeclarationResult{Fatal: []string{fmt.Sprintf("卵 %s：注册表未初始化", displayName(name))}}
	}
	entry, ok := r.Get(name)
	if !ok {
		return EggDeclarationResult{Fatal: []string{fmt.Sprintf("卵 %s：不在注册表里（未登记的模型不能孵）", displayName(name))}}
	}
	return ValidateEggDeclaration(name, entry)
}
