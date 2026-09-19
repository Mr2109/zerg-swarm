// hatch.go —— 孵化器（P1 施工）：把一枚卵孵成「封闭空间里的推理服务」。
//
// 设计真源：设计-子端沙箱化.md
//
//	§6.1 孵化形态（**已由 X3 真机实测修正**）：`bubblewrap` 管**封闭空间**（文件系统视图 +
//	     进程视图 + 只读），`systemd-run --user` 管**归属**（独立 slice/瞬态单元 ⇒ 不被
//	     `x3-agent.service` 的 KillMode 带走 = E1 的正面解法）。单用任一个都不完整。
//	§6.9 **静默失效不得当凭据**：隔离是否生效必须运行时核（读 `/proc/<pid>/mountinfo`），
//	     不得以单元状态为凭 —— 实测遇到过 `Result=success` 却零约束的挂载类选项。
//	§6.6 孵化边界（目录二分）：**一次性**（工作目录）随收卵销毁、**跨孵化保留**（KV 盘、
//	     留证日志）必须活下来 ⇒ 这两类落点都要用**可写绑定**（ExtraRWBinds → bwrap `--bind`）
//	     进空间，且 `Spec.WorkDir` 必须是**空间内**路径（宿主路径填它 ⇒ `--chdir` 必失败）。
//	§6.7 环境需求（EnvReq）：设备与卡号 / 库路径与版本 / 环境变量（含按卵覆盖
//	     `LD_LIBRARY_PATH`）/ 权重路径 / ulimit 与 mmap 限额 —— 孵化器**只照单执行**。
//	§4.3 第八项：`schema_version` 认不得就**明确报错**，不许静默按新格式跑错。
//	§8.4 标定铁律：**无有效实测档案不许孵**（闸门只读实测档案）。
//
// 配方三坑（X3 实测，务必照抄）：
//
//	① Ubuntu usrmerge ⇒ **必须自补 `/bin` `/lib` `/lib64` `/sbin` 符号链接**；
//	   `bwrap: execvp sh: No such file or directory` 是**缺链接，不是 userns 被拒**。
//	② 默认 `--dev /dev` **看不到 GPU** ⇒ 必须显式 `--dev-bind /dev/kfd` + `/dev/dri`。
//	③ `--unshare-user` 写不写行为一致（非 setuid ⇒ 总会自建 userns）。
//	④ **本机 `bwrap --setenv` 必失败**（bubblewrap 0.11.1，X3 真机 2026-09-15 实测：
//	   `bwrap … --setenv FOO=bar -- /bin/sh -c 'echo ok'` → `bwrap: setenv failed`，rc=1；
//	   照原样孵出来的单元 status=1 秒死）⇒ 凡声明了 env 的卵当场孵不出来 ⇒ 按卵环境变量
//	   改走**包装 exec**（`/bin/sh -c 'export …; exec "$@"'`，见 execWrapperScript）。
//	⑤ **缺 `/sys` ⇒ 引擎认不到 GPU 且静默降级 CPU**（真机 2026-09-15：`ggml_cuda_init:
//	   failed to initialize ROCm: no ROCm-capable device is detected`，即便 `/dev/kfd` 与
//	   `/dev/dri/renderD128` 已显式 `--dev-bind`；补 `--ro-bind /sys /sys` 后消失，空间内
//	   `/sys/class/kfd/kfd/topology/nodes/1/properties` 可读）⇒ 配方**恒**挂只读 `/sys`。
//	⑥ **systemd 的 `MainPID` 不是空间内进程**：它是 bwrap 的父进程，**留在原命名空间**
//	   （真机：MainPID=1379471 mnt-ns=mnt:[4026531832] 读到宿主视图 `/data` 可见 / `/models`
//	   不存在 / 550 pids；空间内进程 = 它的子进程 mnt-ns=mnt:[4026532767]）⇒ 封闭性核验
//	   必须选**空间内进程**，判据与解析见 hatch_pid.go / hatch_linux.go 的 SpacePID。
//	⑦ **权重逐文件挂**（2026-09-15 收口，缺陷 12）：`/models` 只由 `--dir` 造一个**空**目录，宿主权
//	   重文件由 `ExtraROBinds` **逐个只读**绑到 `/models/<基名>`。**不许**再把权重**所在目录**整挂
//	   到 `/models`：那会把同目录别的模型一起带进空间（真机 `ls /models` 列出好几个模型），而且
//	   `/models` 一旦成了只读挂载，逐文件绑定就落不进去（`bwrap: Can't create file …:
//	   Read-only file system`，与缺陷 5 同类）。判据同步见 hatch_check.go。
//
// 本文件是**平台无关**部分：声明校验 + 命令行构造（纯函数，可在 macOS 上单测）。
// 真正执行（起单元 / 停单元）在 hatch_linux.go，非 Linux 平台在 hatch_other.go 明确拒绝。
package hatch

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// SliceName 归属切片：孵化单元一律落这里（与 `x3-agent.service` 的 `/system.slice` 分离）。
const SliceName = "llm.slice"

// Spec 一次孵化的全部输入（**声明式**：孵化器不推断、不补默认值，缺就报错）。
type Spec struct {
	EggID         string // 卵名（注册表键）——单元名与日志的身份
	SchemaVersion int    // 卵声明格式版本（必须 = registry.EggSchemaVersionCurrent）

	EnginePathInSpace string   // 引擎在**空间内**的可执行路径（如 /engine/bin/llama-server）
	EngineArgs        []string // 引擎参数（由适配器产出，已展开占位符）
	EngineRoots       []string // 引擎自己的库/构建目录（主机侧）——只读挂进 /engine

	// WeightPath 该卵权重所在的**宿主侧**路径（原来语义「文件或目录」，现在只剩出证/日志用途）。
	//
	// **不再参与挂载**（2026-09-15 收口，缺陷 12）：它曾经被整目录 `--ro-bind` 到 `/models`，而那种
	// 挂法会让同目录的别的模型一起进空间（真机 `ls /models` 列出 Qwen2.5-VL / Flash-Next 等）。现在
	// 挂进空间的是下面 WeightFiles 逐个声明的**文件**；本字段保留：日志/出证仍要它（backend 的
	// 「孵化单元已建…权重=…」那一行就是它），且「权重一条声明都没有 ⇒ 拒孵」这条 fail-closed 也仍
	// 认它（见 Validate）。
	WeightPath string
	// WeightFiles 该卵点名的权重/投影/模板文件在**宿主侧**的绝对路径清单；每个文件在空间内的落点
	// 是 `/models/<基名>`（只读）。
	//
	// 口径（与 backend/hatch_spec.go 的 spaceWeightsDir 同源 —— 跨包契约，改一处必须改另一处）：
	//   - **逐文件、只读**：一个文件占一格，**不整目录挂** ⇒ 同目录的别的模型在空间内根本不存在；
	//   - 落点固定 `/models/<基名>`；**顺序即声明顺序**（argv 逐字可复现，见 BuildBwrapArgv ⑤）；
	//   - 空间内没有第二种权重落点（模板落在权重目录之外时，由映射侧的约定落 `/templates/<基名>`，
	//     那是「输入通道」而不是权重，不进本清单）。
	//
	// **绑定由谁产（单一来源，写死）**：由**映射侧**（backend `hatch_spec.go`）产 —— 它就是
	// `ExtraROBinds` 里的 `<宿主文件>:/models/<基名>`，与引擎参数改写**同一遍算出**（基名、落点、
	// 顺序天然同源）。hatch 这一侧**不再自己产一份**（两套都产 = 同一落点两条 `--ro-bind`，后挂的
	// 静默盖掉先挂的）；但 hatch 也不放手：`WeightFiles` 与 `ExtraROBinds` 的对应关系被当**契约**
	// 核（缺一、错挂、撞落点、整目录挂载一律拒孵，见 validateWeightFiles）。
	WeightFiles []string

	// Env 按卵环境变量（含按引擎覆盖 LD_LIBRARY_PATH / HIP_VISIBLE_DEVICES）。
	//
	// 下发方式**不是** bwrap `--setenv`（本机 bubblewrap 0.11.1 上必失败，见文件头坑④），
	// 而是**包装 exec**：`/bin/sh -c 'export K=V …; exec "$@"'`（见 execWrapperScript）。
	// 卵声明面不变 —— 声明里仍是这份键值表，换的只是把它送进空间的那条通路。
	Env map[string]string
	// Devices 要暴露的设备节点（缺省 /dev/kfd + /dev/dri/renderD128）。
	Devices []string

	ExtraROBinds []string // 额外只读绑定，形如 "<host>:<space>"（如输入通道）
	// ExtraRWBinds 额外**可写**绑定，形如 "<host>:<space>"，语义 = bwrap `--bind`（可写）。
	//
	// 为什么必须有它（2026-09-15 拍定）：封闭空间里凡要**写**的落点都得自己绑进来——
	// 一次性工作目录与 KV 盘（§6.6 的「跨孵化保留」侧 / §9.7④）都走这里。
	// 与 ExtraROBinds 并列且**同严格**：格式不对一律拒孵（认不得就报错，绝不猜落点）。
	ExtraRWBinds []string
	// WorkDir 工作目录——**空间内**路径（不是宿主路径！）。
	//
	// 契约（写死）：BuildBwrapArgv 在本空间里 `--chdir s.WorkDir`，故它必须是**空间内**的
	// 绝对路径（如 /work）；宿主侧目录要用 ExtraRWBinds 绑到那个落点上。
	// 宿主路径填这里 ⇒ 空间里根本不存在该目录 ⇒ 真孵化必以 chdir 失败告终（本字段的语义缺陷
	// 就在此，Validate 已把它变成明确的拒孵）。
	WorkDir string

	MemlockBytes int64 // >0 ⇒ 给 LimitMEMLOCK（大模型 mmap 需要；0 = 不设）

	Profile monitor.EggProfile // 实测档案（**必填**：无档案不许孵，§8.4）
}

// defaultDevices 缺省设备：KFD（计算接口）+ 渲染节点（缓冲分配/映射）。
// 注意：**默认 `--dev /dev` 看不到 GPU**（X3 实测），必须显式绑定这两个。
func defaultDevices() []string { return []string{"/dev/kfd", "/dev/dri/renderD128"} }

// Validate 声明校验（fail-closed：缺一律报错，绝不静默补默认值）。
func (s Spec) Validate() error {
	if s.EggID == "" {
		return fmt.Errorf("孵化声明缺 egg_id")
	}
	if s.SchemaVersion != registry.EggSchemaVersionCurrent {
		return fmt.Errorf("卵声明格式版本 %d 认不得（本端认得 %d）——拒孵，不按新格式猜着跑",
			s.SchemaVersion, registry.EggSchemaVersionCurrent)
	}
	if s.EnginePathInSpace == "" {
		return fmt.Errorf("孵化声明缺引擎在空间内的路径（engine_path_in_space）")
	}
	if !strings.HasPrefix(s.EnginePathInSpace, "/") {
		return fmt.Errorf("引擎路径必须是空间内的绝对路径（如 /engine/bin/llama-server），实得 %q", s.EnginePathInSpace)
	}
	// 权重：**两个字段都空**才算「没声明权重」⇒ 拒孵（fail-closed 不放宽：一条声明路都不给就走人，
	// 绝不孵一个空手进来的空间）。
	//
	// 为什么用「且」而不是「或」（2026-09-15 收口）：WeightPath 现在只用于出证/日志、**不参与挂载**，
	// 而真正决定「空间里挂了什么」的是 WeightFiles（+ 由它派生的 ExtraROBinds）⇒ 只认 WeightPath
	// 会放过「什么都没挂、权重在空间里根本不存在」的卵；只认 WeightFiles 又会把「还在只报出证信息」
	// 的旧形态一律拒掉。逐文件那半条 fail-closed 在 validateWeightFiles 里（两个字段都有时照样核）。
	if len(s.WeightFiles) == 0 && strings.TrimSpace(s.WeightPath) == "" {
		return fmt.Errorf("孵化声明缺权重（weight_files 与 weight_path 都是空）——拒孵：" +
			"孵化器不替卵挑权重，也不孵一枚空手进来的卵（空间里一份权重都没有 = 引擎找不到权重）")
	}
	// 工作目录的契约是**空间内**路径（BuildBwrapArgv 在空间里 --chdir 它）。
	// 诚实说明这一道能拦什么：宿主绝对路径与空间内落点**在语法上无法区分**（两者都是 / 开头）
	// ⇒ 本检查只能拦住非绝对的形态（相对路径、~ 前缀——`~` 在空间里不会被展开成家目录）；
	// 「宿主路径填进 WorkDir」这个原始缺陷由**映射侧**关掉（hatch_spec.go 恒填 /work + 可写绑定），
	// 并有用例钉住。
	if s.WorkDir != "" && !strings.HasPrefix(s.WorkDir, "/") {
		return fmt.Errorf("工作目录必须是**空间内**的绝对路径（如 /work），实得 %q（宿主侧形态）——"+
			"宿主目录要经 ExtraRWBinds 绑到该落点上，不能填进 WorkDir", s.WorkDir)
	}
	// 额外绑定：只读与可写**同严格**（格式不对即拒孵，绝不猜一个落点）
	if err := validateBinds("只读", s.ExtraROBinds); err != nil {
		return err
	}
	if err := validateBinds("可写", s.ExtraRWBinds); err != nil {
		return err
	}
	// 权重逐文件（声明 ↔ 绑定当契约核；硬边界「不许整目录挂 /models」恒核）
	if err := validateWeightFiles(s); err != nil {
		return err
	}
	// 环境变量：名字必须能被 shell 的 `export` 承接（名字非法 ⇒ 包装脚本语法错 = 整条卵静默走偏）
	if err := validateEnv(s.Env); err != nil {
		return err
	}
	if err := s.Profile.Validate(); err != nil {
		return fmt.Errorf("实测档案不可用，拒孵（§8.4 标定铁律）：%w", err)
	}
	return nil
}

// validateBinds 逐条校验额外绑定清单（形式 `<host>:<space>`）。
//
// 只读（--ro-bind）与可写（--bind）用**同一口径**：任一侧缺（或只有空白）就报错。
// 理由：绑定的目的就是「给空间里一个确切的落点」，形式认不得时任何「补一个默认值」的猜测
// 都会把卵挂到别的地方 —— 那正是 §6.9 要防的静默失效，故一律拒孵。
func validateBinds(kind string, binds []string) error {
	for _, b := range binds {
		if _, _, err := splitBind(kind, b); err != nil {
			return err
		}
	}
	return nil
}

// ── 权重逐文件（2026-09-15 收口，缺陷 12）─────────────────────────────────────

// spaceModelsDir 权重在空间内的落点根：每个声明的权重文件各占 `/models/<基名>` 一格（只读）。
//
// 与 backend/hatch_spec.go 的 `spaceWeightsDir` **同值**（跨包契约：两个包不互相依赖，故各留一份
// 常量；改这里必须同步改那边，反之亦然）。
const spaceModelsDir = "/models"

// spaceWeightLanding 声明文件在空间内的落点（`/models/<基名>`）。
//
// **落点规则只有这一份实现**（自检与封闭性判据都用它）：两侧各算一遍就会漂移出「核的不是挂的那个」
// —— 那类故障在核验里看不出来（被核的落点即使都在，也可能不是该卵点名的文件）。
func spaceWeightLanding(hostFile string) string {
	return spaceModelsDir + "/" + filepath.Base(filepath.Clean(hostFile))
}

// validateWeightFiles 逐文件权重声明的 fail-closed 自检：**声明（WeightFiles）↔ 绑定（ExtraROBinds）
// 不许对不上**，且**任何绑定都不许落在 `/models` 本身**。
//
// 为什么必须有它（2026-09-15 收口）：空间里到底挂了什么，由 `ExtraROBinds`（映射侧产）说了算；而
// 「这枚卵声明了哪些权重文件」在 `WeightFiles` 里。两者一旦对不上，真机上就是最贵的那类静默故障 ——
// 绑定少了 = 引擎在空间里找不到权重（「看着起来了，其实读不到模型」）；绑定多了/挂错 = 有一份静默
// 不见了或挂的是别的文件。所以这里当**契约**核，不满足即拒孵：
//
//	① 声明的宿主路径必须是绝对路径，且能推出唯一基名（空串 / `.` / `..` / `/` 一律拒）；
//	② 同一空间目录下**基名撞车**即拒（逐文件挂载时后挂的会盖住先挂的）；
//	③ 每条声明**必须恰好有一条** `<同一宿主文件>:/models/<基名>` 的只读绑定；
//	④ **任何**绑定（只读或可写）的落点都不许是 `/models` **本身**（见 rejectModelsDirMounts）；
//
// ①②③ 只在**给了声明**（WeightFiles 非空）时核（没给声明就没有可核的对象；此时 hatch 只保证 ④
// 这条硬边界）。路径比较用 `filepath.Clean` 而不是逐字相等：逐字比较会把「同一个文件的两种写法」
// 判成不符 ⇒ 误拒一枚好卵（比漏判更糟），而「clean 后不同」才是真的挂错了文件。
func validateWeightFiles(s Spec) error {
	if err := rejectModelsDirMounts(s); err != nil {
		return err
	}
	if len(s.WeightFiles) == 0 {
		return nil
	}
	// 空间内落点 → 已经出现过的绑定宿主（判「撞落点」与「挂错文件」）
	boundAt := make(map[string][]string, len(s.ExtraROBinds))
	for _, b := range s.ExtraROBinds {
		host, space, err := splitBind("只读", b)
		if err != nil {
			return err // 格式错由 validateBinds 报；这里不重复措辞（走到这里说明调用方跳过了那一关）
		}
		boundAt[space] = append(boundAt[space], host)
	}
	seen := make(map[string]string, len(s.WeightFiles)) // 空间内落点 → 声明来源
	for i, raw := range s.WeightFiles {
		decl := strings.TrimSpace(raw)
		if decl == "" {
			return fmt.Errorf("权重文件清单（weight_files）第 %d 项是空串——拒孵（文件路径拿不到，不猜）", i+1)
		}
		if !filepath.IsAbs(decl) {
			return fmt.Errorf("权重文件 %q 不是宿主侧绝对路径——拒孵（逐文件只读绑定与空间内落点 %s/<基名> 都要求宿主绝对路径）",
				decl, spaceModelsDir)
		}
		landing := spaceWeightLanding(decl)
		if base := filepath.Base(landing); base == "." || base == ".." || base == string(filepath.Separator) || base == "" {
			return fmt.Errorf("权重文件 %q 推不出基名——拒孵（落点 %s/<基名> 定不下来）", decl, spaceModelsDir)
		}
		if prev, dup := seen[landing]; dup {
			return fmt.Errorf("权重文件 %q 与 %q 的空间内落点都是 %q——拒孵：逐文件挂载时后挂的会盖住先挂的，"+
				"落点撞车等于有一份静默不见了", prev, decl, landing)
		}
		seen[landing] = decl
		hosts := boundAt[landing]
		switch {
		case len(hosts) == 0:
			return fmt.Errorf("权重文件 %q 没有对应的只读绑定（缺 %q）——拒孵：声明了却没挂进空间，"+
				"引擎在 %s 下根本找不到它", decl, decl+":"+landing, landing)
		case len(hosts) > 1:
			return fmt.Errorf("空间内落点 %q 上有多条只读绑定（%v）——拒孵：说不清哪一条生效（后挂的会静默盖掉先挂的）",
				landing, hosts)
		case filepath.Clean(hosts[0]) != filepath.Clean(decl):
			return fmt.Errorf("空间内落点 %q 上挂的是 %q，声明里写的是 %q——拒孵：挂的文件与声明报的不是同一个"+
				"（引擎会读到另一份权重，而核验看不见这个差异）", landing, hosts[0], decl)
		}
	}
	return nil
}

// rejectModelsDirMounts 硬边界：**任何**绑定（只读或可写）的落点都不许是 `/models` **本身**；
// **可写**绑定另外还不许落在 `/models/<…>` 之下（权重一律只读，要能写就绑 /work 那种可写落点）。
//
// 这一条是缺陷 12 的机器判据（旧实现正是 `--ro-bind <权重所在目录> /models`）：
//   - 整目录挂载会把**同目录的别的模型**一起带进空间（真机 `ls /models` 列出好几个模型），与
//     「只挂该卵自己的权重」直接矛盾 —— 而且**只读也不够**：只读说的是「不能改」，不是「只有这一份」；
//   - 它还会让逐文件绑定**落不进去**：`/models` 成了只读挂载之后，bwrap 建不出子挂点
//     （`bwrap: Can't create file …: Read-only file system`，与缺陷 5 同类）。
//
// 为什么连「可写绑到 /models/<…>」也拒：那种卵真机上会被封闭性核验判成不符（权重落点 rw ⇒
// 宿主权重可被引擎改写）⇒ 收卵 + 拒孵。**声明期就拒**比「孵起来再被核验杀掉」清楚得多（同一条口径
// 只写一遍，见 hatch_check.go 的 modelsCriterion）。
//
// 恒核（不受 WeightFiles 是否为空影响）：它是「空间里有什么」的边界，不是「声明齐不齐」的检查。
func rejectModelsDirMounts(s Spec) error {
	for _, group := range []struct {
		kind  string
		binds []string
	}{{"只读", s.ExtraROBinds}, {"可写", s.ExtraRWBinds}} {
		for _, b := range group.binds {
			_, space, err := splitBind(group.kind, b)
			if err != nil {
				return err
			}
			clean := filepath.Clean(space)
			switch {
			case clean == spaceModelsDir:
				return fmt.Errorf("额外%s绑定 %q 把落点定在 %s **本身**（整目录挂载）——拒孵：它会把同目录的别的模型一起带进空间"+
					"（缺陷 12），并且 %s 一旦成了只读挂载，逐文件绑定就落不进去"+
					"（`bwrap: Can't create file …: Read-only file system`）——权重一律逐文件绑到 %s/<基名>",
					group.kind, b, spaceModelsDir, spaceModelsDir, spaceModelsDir)
			case group.kind == "可写" && strings.HasPrefix(clean, spaceModelsDir+"/"):
				return fmt.Errorf("可写绑定 %q 把落点定在 %s 之下——拒孵：该落点上的权重会被引擎**改写**"+
					"（封闭性核验判「权重落点不是只读」⇒ 收卵拒孵）；要一个可写落点请用 /work 那种专用目录",
					b, spaceModelsDir)
			}
		}
	}
	return nil
}

// envVarName 合法环境变量名（POSIX：字母/下划线开头，其余字母数字下划线）。
//
// 为什么必须校（2026-09-15）：环境变量不再由 bwrap `--setenv` 下发，而是由包装 shell 里的
// `export` 承接（坑④）；名字不合法的变量在那条脚本里是**语法错误** ⇒ 整条包装脚本跑不起来
// 或跑偏，而这属于「看着起来了，其实变量没进去」—— 宁可拒孵，不许静默。
var envVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// validateEnv 校验按卵环境变量表（名字合法 + 值里没有 execve 不可能接受的 NUL）。
// 遍历顺序排序 ⇒ 同一份声明每次给出同一条错误（可复现）。
func validateEnv(env map[string]string) error {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !envVarName.MatchString(k) {
			return fmt.Errorf("环境变量名 %q 不是合法 POSIX 名（^[A-Za-z_][A-Za-z0-9_]*$）——"+
				"它要由包装 exec 的 `export` 承接，名字非法会让整条包装脚本走偏 ⇒ 拒孵", k)
		}
		if strings.ContainsRune(env[k], 0) {
			return fmt.Errorf("环境变量 %s 的值含 NUL —— execve 不可能接受，拒孵", k)
		}
	}
	return nil
}

// splitBind 把一条额外绑定拆成 (宿主路径, 空间内路径)。
// kind 只用于错误信息（「只读」/「可写」），两条路径的校验口径完全相同。
func splitBind(kind, b string) (string, string, error) {
	parts := strings.SplitN(b, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("额外%s绑定必须是 <host>:<space> 形式，实得 %q", kind, b)
	}
	return parts[0], parts[1], nil
}

// unitSafe 把卵名收敛成合法的 systemd 单元名片段（只留小写字母/数字/连字符）。
var unitSafe = regexp.MustCompile(`[^a-z0-9]+`)

// UnitName 由卵名派生**确定性**单元名：确定性 ⇒ 收卵可幂等（同名单元停两次不炸）。
func UnitName(eggID string) string {
	s := strings.ToLower(eggID)
	s = unitSafe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "egg"
	}
	if len(s) > 48 { // 单元名过长会让 systemd 报错，截断并靠前缀保持可读
		s = s[:48]
	}
	return "zerg-" + s
}

// BuildBwrapArgv 构造 bwrap 命令行（**纯函数**，可在任意平台单测）。
// 顺序遵循 X3 实测配方：先造根与链接，再挂权重/引擎/设备，最后降权进入。
func BuildBwrapArgv(s Spec) ([]string, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	argv := []string{
		// ① 全新根：只读挂系统目录（usrmerge 下 /bin /lib /lib64 /sbin 都是 /usr 的链接）
		"--ro-bind", "/usr", "/usr",
		"--symlink", "usr/bin", "/bin",
		"--symlink", "usr/sbin", "/sbin",
		"--symlink", "usr/lib", "/lib",
		"--symlink", "usr/lib64", "/lib64",
		// ② 基础伪文件系统
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		// ②b 宿主 `/sys` **只读**进空间 —— 通用需求，不是某引擎的特性。
		//
		// X3 真机 2026-09-15：缺它时引擎报 `ggml_cuda_init: failed to initialize ROCm:
		// no ROCm-capable device is detected` 并**静默降级 CPU**（即便 /dev/kfd 与
		// /dev/dri/renderD128 已显式 --dev-bind）；补上后该错误消失，空间内
		// `/sys/class/kfd/kfd/topology/nodes/1/properties` 可读（simd_count 80）。
		// 为什么算通用需求：设备**可不可用**这件事由 /sys 下的拓扑/属性描述（GPU 如此，
		// 别的加速器同理），任何要认设备的引擎都读它；只读 ⇒ 不放开写面。
		"--ro-bind", "/sys", "/sys",
		// ③ 权重的落点根：**只造一个空目录**。宿主权重文件由 `ExtraROBinds` **逐个只读**绑到
		//    `/models/<基名>`（在 ⑤ 里产出）；整目录挂载已退役（缺陷 12，见文件末「2026-09-15 收口」）。
		//
		// 为什么用 `--dir` 而不是 `--tmpfs`：`--dir` 只在新根的 tmpfs 上建一个**空**目录（不含任何
		// 宿主数据、随收卵销毁），mountinfo 里**不会**多出一条 `/models` 挂载 ⇒ 「`/models` 不是挂载点」
		// 这条判据才有判别力（换成 `--tmpfs`，「空目录打底」与「整目录挂载」在 mountinfo 里长得一样，
		// 判据就分不出谁是谁了）。
		//
		// 诚实说明 `--dir` 提供了什么、没提供什么：bwrap 给缺的父目录**自动补建**
		// （man bwrap：filesystem 类操作按给定顺序应用，`any missing parent directories … are
		// automatically created`）⇒ 它不是「否则建不出来」的必需品；真正需要它的是两件事：
		// ① 落点在配方里**显式**（读 argv 的人不必知道 bwrap 会自动补建）；② 顺序上明确它在那些
		// `/models/<基名>` 绑定**之前** —— 真机上的失败（`Can't mkdir parents for …: Read-only file
		// system`）来源正是「/models 先成了只读挂载」，而这一条保证 /models 是**空目录**、不是挂载。
		"--dir", spaceModelsDir,
	}
	// ④ 引擎自己的库/构建目录（主机侧）→ /engine
	for _, root := range s.EngineRoots {
		argv = append(argv, "--ro-bind", root, "/engine")
	}
	// ⑤ 额外只读绑定（host:space）
	for _, b := range s.ExtraROBinds {
		host, space, err := splitBind("只读", b)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "--ro-bind", host, space)
	}
	// ⑤b 额外**可写**绑定（host:space）——bwrap `--bind`（**可写**，不是 --ro-bind）。
	//     一次性工作目录与 KV 盘（§6.6「跨孵化保留」侧 / §9.7④）都靠它落进空间：
	//     空间里的落点是新根上的目录（bwrap 自己造），宿主侧那份才是数据真正住的地方。
	//     顺序 = 清单顺序（不改序）⇒ 同一 Spec 构造出的 argv 逐字可复现。
	for _, b := range s.ExtraRWBinds {
		host, space, err := splitBind("可写", b)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "--bind", host, space)
	}
	// ⑥ GPU：**必须显式**（默认 --dev 看不到）
	devices := s.Devices
	if len(devices) == 0 {
		devices = defaultDevices()
	}
	for _, d := range devices {
		argv = append(argv, "--dev-bind", d, d)
	}
	// ⑦ 隔离：进程视图（空间内 pid 1 就是引擎）
	argv = append(argv, "--unshare-pid", "--die-with-parent")
	// ⑧ 工作目录（**在包装脚本/引擎之前**：chdir 之后 exec，两边的 cwd 都是它）
	if s.WorkDir != "" {
		argv = append(argv, "--chdir", s.WorkDir)
	}
	// ⑨ 入口 —— 有按卵环境变量 ⇒ **包装 exec**（不是 `--setenv`，见坑④）；
	//     没有要下发的变量 ⇒ 直 exec，不加那层 sh（多一层会把 execve 失败的归因变含糊）。
	argv = append(argv, "--")
	if script, ok := execWrapperScript(s.Env); ok {
		argv = append(argv, shPath, "-c", script, wrapperArg0, s.EnginePathInSpace)
	} else {
		argv = append(argv, s.EnginePathInSpace)
	}
	argv = append(argv, s.EngineArgs...)
	return argv, nil
}

// shPath 空间内的 shell。`/bin` 是 usrmerge 符号链接（文件头坑①），真机实测可用。
const shPath = "/bin/sh"

// wrapperArg0 包装脚本的 `$0`（**不参与** `exec "$@"`）。
//
// 为什么不顺手写 `--` 当 $0（`sh -c <script> -- <引擎> <参数…>` 常见写法）：那个 `--` 到底是
// `$0` 还是**选项终止符**随 shell 实现而异 —— 若被当选项终止符，`$0` 会变成引擎路径、`$@`
// 少一个参数 ⇒ 引擎名丢了却照样能起（参数错位没人看得出来）。所以这里用一个固定普通字当
// `$0`，把「引擎 + 参数」完整留在 `$@` 里，`exec "$@"` 的语义就与实现无关（有用例真跑 shell 钉住）。
const wrapperArg0 = "zerg-egg"

// execWrapperScript 把按卵环境变量编成包装脚本：`export K=V …; exec "$@"`。
//
// **为什么不用 bwrap `--setenv`（X3 真机 2026-09-15）**：本机 bubblewrap 0.11.1 上
// `bwrap … --setenv K=V -- /bin/true` 直接失败（`bwrap: setenv failed`，rc=1）⇒ 凡声明了
// env 的卵当场秒死，`LD_LIBRARY_PATH` 这类**必需**通路全断。包装 exec 用同一份卵声明面
// （Spec.Env 不改）把变量送进空间，且不依赖 bwrap 的 setenv。
//
// **为什么不用 `systemd-run -p Environment=`**：那属于**归属层**（见文件头分工），变量会落在
// 单元上、绕过这个封闭空间的口径 —— 要的是「空间内进程的环境」，归属层给不了这个保证。
//
// 引号与语义：值一律用**单引号**包住，值里出现的单引号按 POSIX 惯例转义（闭引号 + 反斜杠+单引号
// + 重开引号，见 shellSingleQuote）⇒ 值里的空格/换行/`$`/反引号/双引号都不会被 shell 二次展开；
// 键名由 validateEnv 保证是合法 POSIX 名。键按名字排序 ⇒ 同一 Spec 构造出的 argv 逐字可复现。
// 第二个返回值 false = 没有要下发的变量（此时不加包装）。
func execWrapperScript(env map[string]string) (string, bool) {
	if len(env) == 0 {
		return "", false
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("export ")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(shellSingleQuote(env[k]))
	}
	b.WriteString(`; exec "$@"`)
	return b.String(), true
}

// shellSingleQuote 把任意字符串包成 shell 单引号字面量（值里的单引号按 POSIX 惯例转义）。
//
// 语义说明：单引号内除了单引号本身一切原样（`$`、反引号、反斜杠都不展开），所以这是把任意
// 字节安全送进 shell 的标准做法；遇到值里的单引号要先闭合、加一个转义的单引号、再重开。
func shellSingleQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

// BuildSystemdRunArgv 构造 `systemd-run --user` 命令行：**它只管归属**（slice/单元/限额），
// 挂载与设备全部交给 bwrap —— 因为挂载类选项在用户实例里**静默失效**（§6.1 ②③）。
func BuildSystemdRunArgv(s Spec, unit string) ([]string, error) {
	bwrapArgv, err := BuildBwrapArgv(s)
	if err != nil {
		return nil, err
	}
	if unit == "" {
		return nil, fmt.Errorf("缺单元名")
	}
	argv := []string{
		"systemd-run", "--user",
		"--unit=" + unit,
		"--slice=" + SliceName,
		// ── --collect（2026-09-19 ③，X3 真机实测）：单元退出/失败后**自动收走**，不留在 systemd
		// 里占名字。没有它：卵被 OOM 杀掉后单元以 failed 常驻 ⇒ 下一次同名 /load 直接
		// `Failed to start transient service unit: Unit zerg-<名>.service was already loaded
		// or has a fragment file.`（HTTP 500）⇒ **这枚卵再也孵不起来**（手工 reset-failed 后
		// 同一条 load 立刻 200，因果成立）。行内也保留孵化前的定点清理（见
		// PrepareHatchUnit / cleanupUnitBeforeHatch）：--collect 防新的，定点清理医已有的。
		"--collect",
		// Type=exec：execve 失败即算启动失败（否则 systemd 认为"起来了"）
		"--property=Type=exec",
		// 收卵语义：停单元时连它整棵子进程一起收（KillMode 默认 control-group 正是我们要的）
		"--property=KillMode=control-group",
	}
	if s.MemlockBytes > 0 {
		argv = append(argv, fmt.Sprintf("--property=LimitMEMLOCK=%d", s.MemlockBytes))
	}
	argv = append(argv, "--", "bwrap")
	argv = append(argv, bwrapArgv...)
	return argv, nil
}

// ═══ 2026-09-19 ③：孵化前的同名单元定点清理（OOM 后 unit 残留把孵化堵死）══════════
//
// 现象（X3 真机逐字）：OOM 之后 transient unit 以 failed 常驻 systemd（`Active: failed
// (Result: oom-kill)`）⇒ `POST /api/control/load` 直接
//
//	孵化单元创建失败（zerg-deepseek-v4-flash）：exit status 1：
//	Failed to start transient service unit: Unit zerg-deepseek-v4-flash.service was already
//	loaded or has a fragment file.
//
// 手工 `systemctl --user stop` + `reset-failed` 之后同一条 load 立刻 200（因果成立）。
// 处置两半（缺一不可）：
//
//	① `BuildSystemdRunArgv` 加 `--collect`（退出即收，正本清源——防**新**的残留）；
//	② 孵化前对**同名**单元做定点清理（stop + reset-failed，幂等）——医**已有**的残留。
//
// 为什么是「定点」而不是「再扫一遍 llm.slice」：孵化器只该为自己要占的那个名字负责。扫全量会把
// 别的卵卷进来（运行期收别人的卵是收卵/启动 GC 路径的事，见 backend/egg_gc.go），而孵化路径一旦
// 误停别的卵，就是在服务中把它们拆掉。backend 的启动 GC 已经在启动时清过一道；运行期（两次
// 重启之间）新留下的失败单元只有这里能治。

// UnitCmdRunner 跑一条 `systemctl --user …` 命令并回原文（错误也回）。
//
// 抽成参数而不是直接 exec 的理由与 backend.unitCmdRunner 同源：**开发机（macOS）上没有 systemd
// 用户实例**，真依赖只在 X3/生产上存在 ⇒ 清理逻辑必须能用假 runner 逐条钉住（判据：argv 逐个
// 比对 + 调用顺序）。生产实现见 hatch_linux.go 的 realUnitCmdRunner。
type UnitCmdRunner func(timeout time.Duration, name string, args ...string) (string, error)

// unitRunningStates `systemctl --user is-active` 输出里表示「单元此刻占着名字且在活动」的态。
var unitRunningStates = map[string]bool{
	"active":       true,
	"activating":   true,
	"reloading":    true,
	"deactivating": true,
}

// benignUnitErr 幂等口径：单元本来就不存在 / 没加载 ⇒ 清理成功（不是错误）。
// 与 hatch.collectOutcome / backend.benignStopErr 同一判据（各自一份是因为三处分属不同层，
// 反向依赖会把分层打乱）。
func benignUnitErr(msg string) bool {
	low := strings.ToLower(msg)
	for _, b := range []string{"not found", "not loaded", "no such unit", "could not be found"} {
		if strings.Contains(low, b) {
			return true
		}
	}
	return false
}

// cleanupUnitBeforeHatch 孵化前定点清理同名单元，返回**实际发出的动作序列**（供用例逐条钉住，
// 也供日志复盘）。幂等、可重入：单元不存在 ⇒ 只有 reset-failed 一条（它也幂等）。
//
// 三步（与 backend/egg_gc.go 的 gcLeftoverEggs 同序，只是范围收窄到一个名字）：
//
//	① 读 is-active（`is-active` 对 inactive/failed 也**非零退出** ⇒ 只看 stdout，不看退出码）；
//	② 活动或 failed ⇒ `stop`（幂等：not found / not loaded 算成功）；
//	③ `reset-failed`（失败态单元不 reset 会一直挂在 systemd 里占名字——这正是 500 的直接原因）。
//
// 错误一律按 benign 处理并**记日志**（清理是孵化前的尽力而为，不能因为清不掉就拒绝孵化：
// 真清不掉的话 systemd-run 自己会报错，那里的原文更准）。run == nil 或单元名为空 ⇒ 什么都不做
// （返回空序列，如实：没动过手）。
func cleanupUnitBeforeHatch(unit string, run UnitCmdRunner) []string {
	var actions []string
	unit = strings.TrimSpace(unit)
	if unit == "" || run == nil {
		return actions
	}
	isActiveOut, _ := run(10*time.Second, "systemctl", "--user", "is-active", unit)
	st := strings.ToLower(strings.TrimSpace(isActiveOut))
	if unitRunningStates[st] || st == "failed" {
		stopOut, stopErr := run(20*time.Second, "systemctl", "--user", "stop", unit)
		switch {
		case stopErr == nil || benignUnitErr(stopOut+" "+stopErr.Error()):
			actions = append(actions, "stop")
		default:
			// 停不掉照样往下走（reset-failed 仍可能把名字放出来）；留痕，不静默。
			log.Printf("[hatch] ⚠ 孵化前清理：stop %s 未成功（按 benign 继续）：%v：%s",
				unit, stopErr, strings.TrimSpace(stopOut))
			actions = append(actions, "stop-failed")
		}
	}
	rsOut, rsErr := run(10*time.Second, "systemctl", "--user", "reset-failed", unit)
	if rsErr != nil && !benignUnitErr(rsOut+" "+rsErr.Error()) {
		log.Printf("[hatch] ⚠ 孵化前清理：reset-failed %s 未成功（继续孵化；真清不掉 systemd-run 会自己报）：%v：%s",
			unit, rsErr, strings.TrimSpace(rsOut))
	} else {
		actions = append(actions, "reset-failed")
	}
	if len(actions) > 0 {
		log.Printf("[hatch] 孵化前清理 %s（is-active=%q）：%v —— ③ 治「OOM 后 transient unit 残留 ⇒ 500 already loaded」",
			unit, orDashStr(st), actions)
	}
	return actions
}

// orDashStr 空串显示成 "-"（日志里"空"与"没读到"要看得出来）。
func orDashStr(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// PrepareHatchUnit 孵化**前置**：构造 systemd-run 命令行 + 定点清理同名单元。
// Hatch（Linux 真路径）与用例都走它 ⇒ 「加 --collect」与「清理被调」两条判据钉在同一处。
// run == nil ⇒ 只构造命令行（dry-run：非 Linux 的用例、以及"只想看 argv"的调用方）。
func PrepareHatchUnit(unit string, spec Spec, run UnitCmdRunner) ([]string, error) {
	argv, err := BuildSystemdRunArgv(spec, unit)
	if err != nil {
		return nil, err
	}
	cleanupUnitBeforeHatch(unit, run)
	return argv, nil
}

// collectOutcome 把 `systemctl --user stop <unit>` 的结果翻译成结论（**纯函数**，便于单测）。
// X3 实测：二次 stop 返回 rc=5（单元已不存在）——这是**幂等成功**，不是错误。
func collectOutcome(rc int, stderr string) error {
	if rc == 0 {
		return nil
	}
	low := strings.ToLower(stderr)
	for _, benign := range []string{"not found", "not loaded", "no such unit", "could not be found"} {
		if strings.Contains(low, benign) {
			return nil // 幂等：本来就没有 ⇒ 收卵成功
		}
	}
	return fmt.Errorf("收卵失败（rc=%d）：%s", rc, strings.TrimSpace(stderr))
}

// ═══ 2026-09-15 收口：权重挂载改「逐文件」（缺陷 12 的最后一段）═══════════════
//
// 背景：`hatch_spec.go`（映射侧）已改成**逐文件**只读绑定（`entry.file` / mmproj / 同目录模板各落
// `/models/<基名>`），而孵化器这一侧还在挂 `--ro-bind <WeightPath> /models`（**整目录**）⇒ 真机上
// `/models` 先成了只读挂载，逐文件绑定落不进去（`bwrap: Can't create file …: Read-only file
// system`，与缺陷 5 同类：整卵 1–3ms 秒死）。本段是那一刀。
//
// 改了四件事（逐条都有用例钉住）：
//
//	① **去掉整目录挂载**：`BuildBwrapArgv` 不再产出 `--ro-bind <WeightPath> /models`；
//	② **显式造出空落点**：改产 `--dir /models`（新根 tmpfs 上的空目录），**位置在 ExtraROBinds 的
//	   逐文件绑定之前** —— 落点由配方显式给出、且它**不是**挂载点（判据据此；bwrap 对缺的父目录会
//	   自动补建，所以 `--dir` 的价值是「显式 + 在前」，不是「否则建不出来」）；
//	③ **新增 `Spec.WeightFiles`** + `validateWeightFiles`：逐文件声明的宿主清单，与 `ExtraROBinds`
//	   当**契约**核（缺一 / 挂错 / 撞落点 / 整目录挂载一律拒孵）；`WeightPath` **保留**但退出挂载面
//	   （只用于日志/出证）；
//	④ **封闭性判据同步**（`hatch_check.go`）：从「`/models` 存在且只读」改成「每个声明的权重文件都在
//	   （且只读）、且没有挂载点正好落在 `/models`」。
//
// **绑定由谁产（明确只选一边，写清理由）**：由**映射侧**（backend `hatch_spec.go`）产 —— 即
// `ExtraROBinds` 里的逐文件 `<宿主文件>:/models/<基名>`。孵化器**不自产**：
//
//	- 映射侧把「绑定」与「引擎参数改写」在**同一遍**里算出（`spaceInputMappings` 同时返回 rewrites
//	  与 binds）⇒ 基名、落点、顺序天然同源；孵化器若自产一份，就得把「模板在权重目录之外落
//	  /templates、基名撞车即拒、宿主路径含 `:` 即拒」这套规则复制一遍 —— 两套规则一漂移，轻则同一
//	  落点两条 `--ro-bind`（后挂的静默盖掉先挂的），重则**参数指向 A 而空间里挂的是 B**（引擎读不到
//	  权重，而核验看不见这个差异：两个落点都「在」，只是不是同一个文件）；
//	- `ExtraROBinds` 本就是通用输入通道（库目录 → `/libs/<名>`、模板 → `/templates/<基名>` 都走它）
//	  ⇒ 孵化器只做「把清单变成 bwrap 参数」，「清单是什么」留在映射层，职责不重叠。
//
// 孵化器这一侧并没有因此放手：`WeightFiles`（声明）与 `ExtraROBinds`（绑定）被当契约核（③），
// **两边对不上就拒孵** ⇒ 单一来源，但不留静默漂移的余地。
//
// ⚠ 未接线（如实登记，别当已做）：`VerifyEnclosure(pid int)` 只有 pid，拿不到 `Spec.WeightFiles`
// （后端的 `hatcher` 接口就是它，本批不许改 backend）⇒ 真机核验路径上「逐文件」那半条判据退化为
// **结构形态**（无整目录挂载 + 每个 `/models/<…>` 都只读 + 至少有一个），「正好是哪几个声明文件」
// 要等后端把 `spec.WeightFiles` 传进来（见 hatch_check.go 的 CheckEnclosure 注释）。
