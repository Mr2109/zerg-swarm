// hatch_spec.go —— 批 1：卵声明 → 孵化声明（hatch.Spec）的映射。
//
// 设计真源：《设计-子端隔离化-20260914》（仓库内该文档标题含旧词，按既有先例此处与
// 包内注释一律以「隔离化」指代同一份文件）
//
//	§4.3 卵声明字段表 · §6.6 目录二分（一次性 vs 跨孵化保留）· §6.7 环境供给五类 + 三个真坑
//	§8.4 标定铁律（无实测档案不许孵）· §6.9 静默失效不得当凭据 · §9.7④ KV 盘按卵分目录
//
// 本文件只做一件事：把「卵声明（registry.ModelEntry）+ 引擎实现名 + 端口 + 实测档案」翻译成
// hatch.Spec —— 孵化器的全部输入。四条口径：
//
//	① fail-closed：卵名 / 权重 / 引擎路径任一拿不到就报错拒孵，绝不编造默认值
//	   （孵化器只照单执行，不推断、不补默认，§6.7）；
//	② 只挂该卵在声明里点名的**文件**：entry.file → `/models/<基名>`、mmproj → `/models/<基名>`、
//	   chat_template → 与权重同目录落 `/models/<基名>`、否则落 `/templates/<基名>`（都是逐文件
//	   只读绑定），参数里的宿主路径改写成对应空间内路径 ⇒ **权重所在目录整体不再挂载** ——
//	   同目录的别的模型与整个 /data 在空间内根本不存在（§6.6 / §9.3）；
//	③ 空间内路径约定写死在本文件（/models、/engine、/libs、/templates、/work、/kvdisk），
//	   与 hatch 包的命令行配方一一对应；
//	④ 要**写**的落点一律走可写绑定（ExtraRWBinds → bwrap `--bind`）：WorkDir 给**空间内**
//	   /work（宿主每卵一次性目录绑过去）、KV 盘给空间内 /kvdisk（宿主 ~/.zerg/kvdisk/<卵名>/
//	   绑过去并把引擎参数里的宿主 KV 路径改写掉）——见文件末「2026-09-15 修正」一节；
//	⑤ 库目录与环境变量的关系（缺陷 6 修正）：env_req.lib_paths 里声明的库目录只读落到
//	   **独立的只读挂载点** `/libs/<目录名>`（不再嵌在 /engine 之下），并且**由孵化器据它生成**
//	   空间内的 `LD_LIBRARY_PATH`（引擎的 RUNPATH 写死宿主路径 ⇒ 空间内不设这个变量就找不到
//	   impl 库、单元 127）——见文件末「缺陷 5/6/11/12 修正」。
//
// 纯函数：不 exec、不写盘（只建一次性工作目录 / KV 盘目录 + os.Stat 判模板文件是否存在），
// 可在 macOS 上直接单测；真正的孵化在 hatch 包（非 Linux 明确拒绝）——本包只负责「把声明
// 翻译成孵化器的输入」。
//
// ⚠ 已知待拍板 / 未接线（如实标出，不擅自改 hatch 包的 X3 实测配方）：
//   - 卵声明里的 env_req.**weights** 与 entry.file 的一致性校验尚未接线：两边不一致时以谁为准
//     （还是干脆拒孵）要先拍板；本批不猜。
//   - env_req.**mmap_max_count**（vm.max_map_count）**暂不生效**（2026-09-15 二选一里取的这一支）：
//     它是**机器级 sysctl**、不是按进程的 rlimit，改它要 root 且影响整机，systemd 也没有对应属性
//     ⇒ 孵化层没有可下发的落点。字段注释已明说，绝不许让它看着像保障（详见文件末）。
//   - **逐文件挂载需要 hatch 侧同步**（本批只做到映射边界，见文件末「缺陷 12」）：hatch 侧必须去掉
//     `--ro-bind <WeightPath> /models` 那一条**整目录**挂载，否则本文件产出的逐文件绑定会落进
//     只读的 /models ⇒ bwrap 建不出文件（与缺陷 5 同一类失败）。
package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
	"github.com/Mr2109/zerg-swarm/agent/internal/modeladapter"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// ── 空间内路径约定（与 hatch 包 BuildBwrapArgv 的挂载点一一对应；改这里要同步改那边）──
const (
	// spaceWeightsDir 该卵点名的权重文件（entry.file / mmproj / 同目录模板）在空间内的**目录**：
	// 每个文件各占一格（`/models/<基名>`，只读）⇒ 同目录的别的模型在空间内不存在。
	spaceWeightsDir = "/models"
	// spaceEngineDir 引擎自己的可执行 + 库/构建目录（只读）。
	spaceEngineDir = "/engine"
	// spaceLibRoot 卵声明的库目录（env_req.lib_paths）在**空间内**的落点前缀：
	// 每个宿主库目录 → `/libs/<目录名>`（只读，见 engineLibMappings）。
	//
	// 为什么是**独立**挂载点 `/libs`（缺陷 5，2026-09-15 真机实测）：原来落在 `/engine/lib/<名>`，
	// 而 `/engine` 已经是引擎构建目录的**只读**挂载 ⇒ bwrap 要在只读挂载之下建中间目录，直接失败：
	// `bwrap: Can't mkdir parents for /engine/lib/bin: Read-only file system`（单元 status=1，
	// 1–3ms 秒死）⇒ **整卵孵不起来**。独立挂载点的父目录 /libs 是新根上的普通目录（可写），
	// bwrap 建得出，落点也就成了。`<目录名>` 那一层保留：多个库目录各占一格，谁都不遮挡谁。
	spaceLibRoot = "/libs"
	// spaceTemplatesDir 卵声明引用的模板类只读输入（ExtraROBinds 的「输入通道」：
	// 如 chat_template 落在权重目录之外时，只读挂到 /templates/<基名> 并改写参数）。
	spaceTemplatesDir = "/templates"
	// spaceWorkDir 每卵一次性工作目录在**空间内**的落点（§6.6「一次性」侧）。
	//
	// 宿主侧那份数据住在 ~/.zerg/work/<卵名>/，经 ExtraRWBinds 可写绑到这里。
	// `hatch.Spec.WorkDir` 的契约是**空间内**路径（BuildBwrapArgv 会 `--chdir` 它）——
	// 往它填宿主路径，空间里根本没有那个目录，真孵化必以 chdir 失败告终。
	spaceWorkDir = "/work"
	// spaceKVDiskDir KV 盘在空间内的落点（§6.6「跨孵化保留」侧 / §9.7④）。
	//
	// 宿主侧目录（缺省 ~/.zerg/kvdisk/<卵名>/）可写绑到这里；引擎参数里**精确等于**该宿主
	// 目录的那个值同时改写成它 ⇒ 引擎在空间内照常读写，数据落在跨孵化保留的宿主卷上。
	// 保留语义的由来：ds4 的 --kv-disk-dir 就是「跨服务器重启存活」（§9.4）⇒ 放进一次性
	// 目录等于每次收卵白扔一次复用机会。
	spaceKVDiskDir = "/kvdisk"
)

// kvDiskFlag KV 盘目录参数名（**ds4 归属**，§9.2：--kv-disk-* 是 ds4 的参数、不是 llama.cpp 的）。
// 本文件只**读**它（找宿主目录），不改变归属、不替任何引擎发这个参数。
const kvDiskFlag = "--kv-disk-dir"

// kfdNode AMD 计算接口节点（ROCm 必需）；renderNodeBase /dev/dri/renderD<N> 的基数
// （约定：第 0 张可见卡 = renderD128）。
const (
	kfdNode        = "/dev/kfd"
	renderNodeBase = 128
)

// EnvWorkDirRoot 每卵一次性工作目录的根（缺省 ~/.zerg/work；可覆盖，便于测试与运维改落点）。
const EnvWorkDirRoot = "ZERG_WORK_DIR"

// mainlineEngineProbe 主线引擎的宿主路径探测（缺省即既有的 detectLlamaServerPath——判据不许两处
// 漂移）。抽成变量只为让「主线卵」的映射在本机（未必装了 llama-server）也能被单测。
var mainlineEngineProbe = detectLlamaServerPath

// cardEnvKeys 卡号类环境变量（§6.7 C①：多卡必须声明用哪张卡）。口径与 registry 里那份一致，
// 这里再列一次是因为 registry 的那份未导出（本函数在 backend 包内）。
var cardEnvKeys = []string{"HIP_VISIBLE_DEVICES", "ROCR_VISIBLE_DEVICES", "CUDA_VISIBLE_DEVICES"}

// hatchSpecFor 把一枚卵的声明映射成孵化声明（**纯函数**，fail-closed）。
//
// 入参：
//
//	entry      —— 卵声明（注册表条目；卵名取注册表键，见 registry.ModelEntry.EggName）
//	engineImpl —— 实际会执行的引擎实现名（EngineImplOf 口径，观测面同源）
//	port       —— 本端为该卵分配的端口（引擎参数里的 --port）
//	profile    —— 该卵的实测档案（§8.4：无档案不许孵，本函数只把它搬进声明）
//
// 任一必需信息拿不到 ⇒ 返回错误（调用方据此拒孵）；绝不返回「半份能跑的声明」。
func hatchSpecFor(entry *registry.ModelEntry, engineImpl string, port int, profile monitor.EggProfile) (hatch.Spec, error) {
	var spec hatch.Spec
	if entry == nil {
		return spec, fmt.Errorf("卵声明为空（注册表里没有条目）——拒孵：孵化器不替不存在的卵编一份声明")
	}

	// ① 身份：卵名 = 注册表键（单元名 / 工作目录 / 实测档案一律以它定位）
	eggID := strings.TrimSpace(entry.EggName())
	if eggID == "" {
		return spec, fmt.Errorf("卵名拿不到（注册表键缺失）——拒孵：单元名、工作目录与实测档案都按卵名定位，不许猜")
	}
	spec.EggID = eggID
	spec.SchemaVersion = entry.SchemaVersion
	spec.Profile = profile

	// ② 权重：只挂该卵**点名的文件**（逐文件只读绑定；整目录不再挂）
	hostWeightFile := expandTilde(entry.File)
	if hostWeightFile == "" {
		return spec, fmt.Errorf("卵 %s：权重文件未声明（file 字段）——拒孵（孵化器不替卵挑权重）", eggID)
	}
	if !filepath.IsAbs(hostWeightFile) {
		return spec, fmt.Errorf("卵 %s：权重路径 %q 不是宿主侧绝对路径——拒孵（空间内只读挂载与 %s 内的相对路径都要求绝对路径）",
			eggID, entry.File, spaceWeightsDir)
	}
	// WeightPath 仍是权重**所在宿主目录**：缺陷 12 之后它只用于出证/日志（hatch 侧的整目录挂载
	// 必须同批去掉，见文件末）；真正决定「空间里挂了什么」的是下面逐文件算出的 ExtraROBinds。
	spec.WeightPath = filepath.Dir(hostWeightFile)

	// ③ 引擎：宿主路径 → 宿主目录（只读挂进 /engine）+ 空间内路径（/engine/<可执行名>）
	hostEngine, err := engineHostPath(entry, engineImpl)
	if err != nil {
		return spec, fmt.Errorf("卵 %s：%w", eggID, err)
	}
	spec.EngineRoots = []string{filepath.Dir(hostEngine)}
	spec.EnginePathInSpace = spaceEngineDir + "/" + filepath.Base(hostEngine)

	// ④ 参数：与裸 exec 路径**同一份构造**（buildEngineArgv），再把宿主侧输入改写成空间内路径
	rewrites, roBinds, err := spaceInputMappings(eggID, entry, hostWeightFile)
	if err != nil {
		return spec, err
	}
	_, engineArgs := buildEngineArgv(eggID, entry, port)

	// ④a 库目录（§6.7 第 3 类 / env_req.lib_paths）：每个宿主库目录**只读**落到
	//     /libs/<目录名>（独立只读挂载点，缺陷 5），并把 **env 值**里出现的库宿主路径改写成
	//     对应空间内路径。同名 / 不存在 / 非绝对一律拒孵（见 engineLibMappings）。
	libRewrites, libROBinds, libSpaces, err := engineLibMappings(eggID, entry)
	if err != nil {
		return spec, err
	}
	roBinds = append(roBinds, libROBinds...)

	// ④b KV 盘（§6.6「跨孵化保留」侧 / §9.7④）：宿主 KV 目录可写绑到 /kvdisk，并把引擎参数里
	//     精确等于它的那个值改写成 /kvdisk。判据用**引擎真正会收到的参数**，不再按卵名算一遍
	//     目录：路径规则只有一处真源（适配器的 kvDiskDir，落在 --kv-disk-dir 的值上），重算就是
	//     第二套规则 —— 两处一漂移，「绑的目录」与「引擎写的目录」就不是同一个了。
	kvRWBind := ""
	if kvHost, err := kvDiskHostDir(engineArgs); err != nil {
		return spec, fmt.Errorf("卵 %s：%w", eggID, err)
	} else if kvHost != "" {
		if err := os.MkdirAll(kvHost, 0o755); err != nil {
			return spec, fmt.Errorf("卵 %s：KV 盘目录 %s 建不出来：%w —— 拒孵（KV 盘属跨孵化保留侧，建不出就没有可写落点，§6.6）",
				eggID, kvHost, err)
		}
		rewrites = append(rewrites, pathRewrite{host: kvHost, space: spaceKVDiskDir})
		if kvRWBind, err = rwBind(kvHost, spaceKVDiskDir); err != nil {
			return spec, fmt.Errorf("卵 %s：%w", eggID, err)
		}
	}

	spec.EngineArgs = rewriteArgsToSpace(engineArgs, rewrites)
	spec.ExtraROBinds = roBinds

	// WeightFiles：落进 %s 的**声明文件**清单（宿主侧绝对路径），与上面逐文件绑定**同源**算出 ——
	// 核验层拿它把判据从「结构形态」收紧到「正好是这几个文件、各自一条只读挂载」
	// （2026-09-15 接线；此前核验只能验结构，挂了别的文件看不见）。基名撞车已在上游拒孵。
	for _, rw := range rewrites {
		if rw.space == spaceWeightsDir || strings.HasPrefix(rw.space, spaceWeightsDir+"/") {
			spec.WeightFiles = append(spec.WeightFiles, rw.host)
		}
	}
	if len(spec.WeightFiles) == 0 {
		return spec, fmt.Errorf("卵 %s：权重文件清单算不出来（%s 下没有任何声明文件）——拒孵（无清单 ⇒ 核验无从证「挂的是该挂的」）",
			eggID, spaceWeightsDir)
	}

	// ⑤ 环境与设备：卵声明照单执行（值里的库宿主路径改写成空间内落点 /libs/<名>），
	//     并据 lib_paths **生成**空间内 LD_LIBRARY_PATH（缺陷 6；卵自己显式写了就以它为准）；
	//     声明了卡号才决定可见的渲染节点
	cardIdx, cardKnown := declaredCardIndex(entry)
	spec.Env = hatchEnv(entry, cardIdx, cardKnown, libRewrites, libSpaces)
	spec.Devices = hatchDevices(entry, cardIdx, cardKnown)

	// ⑥ 一次性工作目录（每卵一份；不存在就建）——**空间内**路径 + 宿主目录可写绑过去。
	//     顺序与 §6.6 的表一致：先一次性（工作目录），再跨孵化保留（KV 盘）。
	hostWorkDir, err := ensureEggWorkDir(eggID)
	if err != nil {
		return spec, fmt.Errorf("卵 %s：%w", eggID, err)
	}
	workRWBind, err := rwBind(hostWorkDir, spaceWorkDir)
	if err != nil {
		return spec, fmt.Errorf("卵 %s：%w", eggID, err)
	}
	spec.WorkDir = spaceWorkDir
	spec.ExtraRWBinds = append(spec.ExtraRWBinds, workRWBind)
	if kvRWBind != "" {
		spec.ExtraRWBinds = append(spec.ExtraRWBinds, kvRWBind)
	}

	// ⑦ memlock 限额（§6.7 C②：RLIMIT_MEMLOCK 给不足直接崩，不是变慢）——声明了才下发，不猜
	if entry.EnvReq != nil && entry.EnvReq.MemlockKB > 0 {
		spec.MemlockBytes = int64(entry.EnvReq.MemlockKB) * 1024
	}
	// ⑦b env_req.**mmap_max_count**（vm.max_map_count）：**有意不映射**（缺陷 11，2026-09-15 二选一）。
	//     它是**机器级 sysctl**、不是按进程的 rlimit：没有「按卵下发」的落点（sysctl -w 要 root、
	//     一改全机生效，systemd 单元也没有对应属性），而 hatch.Spec 里同样没有可承载它的字段。
	//     ⇒ 明说「暂不生效」，绝不产出一个看着像保障、其实没人执行的东西（§6.9 同一精神）。
	//     要真生效只有两条路（都超本批范围，待拍板）：机器级前置校验（拿 /proc/sys/vm/max_map_count
	//     与声明值比，不够就拒孵）或给 hatch.Spec 加字段 + 由需要 root 的部署步骤统一改本机 sysctl。
	return spec, nil
}

// ── 引擎路径（④）────────────────────────────────────────────────────────────

// engineHostPath 引擎可执行的**宿主绝对路径**。
//
// 判据（按「实际会执行什么」给真值，与 EngineImplOf / serviceKind 同源）：
//   - cmd: 非空 ⇒ 取首词（非主线引擎必须走 cmd:，P1 守卫在 doStart 最前）；
//   - cmd: 空   ⇒ 主线引擎（llama 家族），走 detectLlamaServerPath（同一份探测，不许第二套）。
func engineHostPath(entry *registry.ModelEntry, engineImpl string) (string, error) {
	if cmd := strings.TrimSpace(string(entry.Cmd)); cmd != "" {
		fields := strings.Fields(cmd)
		if len(fields) == 0 {
			return "", fmt.Errorf("cmd: 声明为空——拒孵（引擎路径拿不到，不猜）")
		}
		p, err := resolveEngineExecutable(fields[0])
		if err != nil {
			return "", err
		}
		return checkEngineImplConsistency(p, engineImpl)
	}
	p, err := resolveEngineExecutable(mainlineEngineProbe())
	if err != nil {
		return "", err
	}
	return checkEngineImplConsistency(p, engineImpl)
}

// resolveEngineExecutable 把「引擎可执行」解成宿主绝对路径。
//
// 绝对路径直接用；裸名字按 PATH 找——**找不到就报错**：空间内要把引擎所在的宿主目录只读挂进
// /engine，找不到目录就没有可挂的东西（挂空路径等于孵出一个起不来的卵，比起不来更糟的是
// 「看起来起起来了」）。
func resolveEngineExecutable(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("引擎可执行为空——拒孵（不猜引擎路径）")
	}
	if filepath.IsAbs(p) {
		return p, nil
	}
	if found, err := exec.LookPath(p); err == nil {
		return found, nil
	}
	return "", fmt.Errorf("引擎可执行 %q 既不是绝对路径、在 PATH 里也找不到——拒孵（宿主目录拿不到，空间内挂不了 %s）",
		p, spaceEngineDir)
}

// checkEngineImplConsistency 引擎实现名（卵声明口径）与「实际会执行的可执行文件」必须相符：
// 声明与执行分叉时 fail-closed 拒孵，绝不静默按其中一个跑（§6.8.4 认不得就报错的精神）。
//
// 接受两种写法（都指向同一个可执行文件才算相符）：
//   - 可辨识短名：`build-hip-flash/llama-server`（推荐，能与主线 llama-server 区分开）
//   - 纯基名：`llama-server`
//
// 声明为空 ⇒ 不校验（调用方走自动推导，两侧同源必然一致）。
func checkEngineImplConsistency(hostEngine, engineImpl string) (string, error) {
	impl := strings.TrimSpace(engineImpl)
	if impl == "" {
		return hostEngine, nil
	}
	base := filepath.Base(hostEngine)
	short := shortEngineName(hostEngine)
	if impl == base || impl == short {
		return hostEngine, nil
	}
	return "", fmt.Errorf("引擎实现名 %q 与实际会执行的可执行文件 %q（可辨识短名 %q）不一致——拒孵（声明与执行分叉，不静默按其一跑）",
		impl, hostEngine, short)
}

// ── 库目录落点与库路径改写（④a，§6.7 第 3 类）────────────────────────────────

// engineLibMappings env_req.lib_paths → 「空间内落点（只读绑定）+ 值改写表 + 落点清单」。
//
// 口径（Mr2109 2026-09-15 拍，逐条落到本函数）：
//
//	① 每个宿主库目录 **只读** 绑到 `/libs/<目录名>`（**独立只读挂载点**，缺陷 5 修正：
//	   原来落 `/engine/lib/<名>` 而 `/engine` 本身是只读挂载 ⇒ bwrap 建不出中间目录、整卵孵不起来），
//	   <目录名> = filepath.Base(清理后的路径)；
//	② **两个库目录同名 ⇒ 拒孵**（落点撞车，说不清谁盖谁 —— 不许挑一个，也不许后来的赢）；
//	③ 宿主上**不存在** ⇒ 拒孵：不许静默留一个空间内也不存在的路径，那是「看着好了其实没生效」；
//	④ 值的改写只认**整段精确匹配**（`:` 切段；见 rewriteEnvValue），不做前缀/子串泛化；
//	⑤ **不在 lib_paths 里声明的宿主路径不管**（不猜、不自动挂）—— 本函数只看声明，不扫环境变量。
//
// 返回 (值改写表, 只读绑定清单, 空间内落点清单, error)：error ≠ nil ⇒ 调用方拒孵；
// 声明为空 ⇒ 四者全空（不编造）。**落点清单按声明顺序**，是「据 lib_paths 生成空间内
// LD_LIBRARY_PATH」的唯一依据（见 hatchEnv；顺序即搜索顺序，不是随手排的）。
func engineLibMappings(eggID string, entry *registry.ModelEntry) ([]pathRewrite, []string, []string, error) {
	if entry == nil || entry.EnvReq == nil || len(entry.EnvReq.LibPaths) == 0 {
		return nil, nil, nil, nil
	}
	var (
		rws    []pathRewrite
		binds  []string
		spaces []string
		seen   = make(map[string]string, len(entry.EnvReq.LibPaths)) // 目录名 → 已见声明（判同名）
	)
	for _, raw := range entry.EnvReq.LibPaths {
		decl := strings.TrimSpace(raw)
		if decl == "" {
			return nil, nil, nil, fmt.Errorf("卵 %s：env_req.lib_paths 里有一项是空串——拒孵（库目录拿不到，不猜）", eggID)
		}
		host := expandTilde(decl)
		if !filepath.IsAbs(host) {
			return nil, nil, nil, fmt.Errorf("卵 %s：库目录 %q 不是宿主侧绝对路径——拒孵（空间内只读落点与库路径改写都要求宿主绝对路径）",
				eggID, decl)
		}
		clean := filepath.Clean(host)
		name := filepath.Base(clean)
		if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
			return nil, nil, nil, fmt.Errorf("卵 %s：库目录 %q 推不出目录名——拒孵（落点 %s/<目录名> 定不下来）",
				eggID, decl, spaceLibRoot)
		}
		if prev, dup := seen[name]; dup {
			return nil, nil, nil, fmt.Errorf("卵 %s：库目录 %q 与 %q 的目录名都是 %q——拒孵：两个库目录同名会互相遮挡，不说谁盖谁",
				eggID, prev, decl, name)
		}
		seen[name] = decl
		st, err := os.Stat(clean)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("卵 %s：库目录 %q 在宿主上不存在（%v）——拒孵：声明了却不存在的路径，空间内同样不存在，"+
				"绑定与改写都会变成「看着好了其实没生效」", eggID, decl, err)
		}
		if !st.IsDir() {
			return nil, nil, nil, fmt.Errorf("卵 %s：库路径 %q 不是目录——拒孵（lib_paths 声明的是库**目录**，落点 %s/%s 是整目录挂载）",
				eggID, decl, spaceLibRoot, name)
		}
		space := spaceLibRoot + "/" + name
		bind, err := roBind(clean, space)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("卵 %s：%w", eggID, err)
		}
		binds = append(binds, bind)
		spaces = append(spaces, space)
		rws = append(rws, pathRewrite{host: clean, raw: decl, space: space})
	}
	return rws, binds, spaces, nil
}

// roBind 组装一条只读绑定（ExtraROBinds 的 `<host>:<space>` 形式）。
//
// 与 rwBind 同严格：宿主路径里带 `:` 一律拒孵（`:` 就是分隔符，带它的路径会被对侧拆错 ⇒ 静默挂到
// 别的落点，比挂不上更糟）。
func roBind(host, space string) (string, error) {
	if strings.Contains(host, ":") {
		return "", fmt.Errorf("只读绑定的宿主路径 %q 含 `:`——拒孵（`:` 是 <host>:<space> 的分隔符，带它的路径会被拆错、挂到别的落点上）", host)
	}
	return host + ":" + space, nil
}

// rewriteEnvValue 环境变量**值**里的库宿主路径改写：按 `:` 切段，**整段精确匹配**才改。
//
//	/old/bin:/old/other  → /libs/bin:/libs/other
//
// 三条边界（都是口径里写死的）：
//   - **不做前缀/子串替换**（见 pathRewrite.matches）：「/a/b」不等于「/a/bc」，也不许把
//     「/a/b/c」当「/a/b」——「乱改」比不改更糟，认不出就**原样传下去**；
//   - **空串保持清空语义**：`LD_LIBRARY_PATH=""` 是**故意**清空（§6.7 C③ 的 K2 那种 ABI 隔离）
//     ⇒ 原样执行、不改写、不报错；值里的空段（`::` 或结尾的 `:`）同样原样保留；
//   - 不在 lib_paths 里声明的路径**不管**（本函数只认改写表，表里没有的一律原样）。
func rewriteEnvValue(v string, rws []pathRewrite) string {
	if v == "" || len(rws) == 0 {
		return v
	}
	segs := strings.Split(v, ":")
	changed := false
	for i, s := range segs {
		if s == "" {
			continue // 空段不是路径：原样保留（清空语义的一部分）
		}
		for _, r := range rws {
			if r.matches(s) {
				segs[i] = r.space
				changed = true
				break
			}
		}
	}
	if !changed {
		return v
	}
	return strings.Join(segs, ":")
}

// ── 宿主侧输入 → 空间内路径（②）─────────────────────────────────────────────

// pathRewrite 一条「宿主侧输入 → 空间内路径」的改写（整 token / 整段精确匹配）。
type pathRewrite struct {
	host  string // 展开后的宿主路径
	raw   string // 卵声明里的原样字符串（可能带 ~；参数/环境变量里出现的可能正是它）
	space string // 空间内路径
}

// matches 待改写的**整段/token**是不是这一条声明的宿主路径。两种写法任一相符即算：
// 展开后的宿主路径（host）与声明里的原样字符串（raw，如带 `~` 的形态）。
//
// 刻意**只有相等**：没有前缀匹配、没有子串匹配、没有大小写宽容 —— 改错了比不改更难查。
func (r pathRewrite) matches(tok string) bool {
	return (r.host != "" && tok == r.host) || (r.raw != "" && tok == r.raw)
}

// spaceInputMappings 算出需要改写的宿主侧输入，以及需要额外只读绑定的清单（ExtraROBinds）。
//
// **逐文件**覆盖三类输入（都只读；缺陷 12 修正：整**目录**不再挂载 —— 旧实现把权重所在目录
// 整体挂成 /models，真机上空间内 `ls /models` 能列出同目录的别的模型，与「只挂该卵自己的权重」
// 的表述直接矛盾）：
//   - 权重文件（必给）：`<entry.file>:/models/<基名>`；
//   - 视觉投影 mmproj（可选）：`<mmproj>:/models/<基名>`（逐文件挂载后不再要求「与权重同目录」
//     —— 那条限制的由来就是整目录挂载，理由已随之消失；真危险的是**基名撞车**，见下）；
//   - 聊天模板 chat_template（可选）：文件存在才处理（适配器也只在存在时才发该参数）；
//     与权重同目录 ⇒ `/models/<基名>`，否则 ⇒ `/templates/<基名>`（沿用既有的输入通道落点）。
//
// 两条 fail-closed（都只针对「说不清谁赢」的形态）：
//   - **同一空间目录下基名撞车**（如权重与投影同名，或两个模板同名）⇒ 拒孵：逐文件挂载时后挂的
//     会盖住先挂的，落点相同就等于「有一份静默不见了」；
//   - 宿主路径里带 `:` ⇒ 拒孵（见 roBind：`:` 是 <host>:<space> 的分隔符，会被拆错落点）。
func spaceInputMappings(eggID string, entry *registry.ModelEntry, hostWeightFile string) ([]pathRewrite, []string, error) {
	weightDir := filepath.Dir(hostWeightFile)
	weightSpace := spaceWeightsDir + "/" + filepath.Base(hostWeightFile)

	rewrites := []pathRewrite{{
		host:  hostWeightFile,
		raw:   strings.TrimSpace(entry.File),
		space: weightSpace,
	}}
	// 空间内落点 → 声明来源（判「基名撞车」；同一个空间路径只能有一份来源）
	claimed := map[string]string{weightSpace: "权重 " + entry.File}
	weightBind, err := roBind(hostWeightFile, weightSpace)
	if err != nil {
		return nil, nil, fmt.Errorf("卵 %s：%w", eggID, err)
	}
	binds := []string{weightBind}

	if p := strings.TrimSpace(entry.MMProj); p != "" {
		host := expandTilde(p)
		space := spaceWeightsDir + "/" + filepath.Base(host)
		if prev, dup := claimed[space]; dup {
			return nil, nil, fmt.Errorf("卵 %s：视觉投影 %q 与 %s 的空间内落点都是 %q——拒孵：逐文件挂载时后挂的会盖住先挂的，"+
				"落点撞车等于有一份静默不见了", eggID, entry.MMProj, prev, space)
		}
		claimed[space] = "视觉投影 " + entry.MMProj
		b, err := roBind(host, space)
		if err != nil {
			return nil, nil, fmt.Errorf("卵 %s：%w", eggID, err)
		}
		binds = append(binds, b)
		rewrites = append(rewrites, pathRewrite{host: host, raw: p, space: space})
	}

	if p := strings.TrimSpace(entry.ChatTemplate); p != "" {
		host := expandTilde(p)
		if _, err := os.Stat(host); err == nil {
			space := spaceWeightsDir + "/" + filepath.Base(host)
			if filepath.Dir(host) != weightDir {
				space = spaceTemplatesDir + "/" + filepath.Base(host)
			}
			if prev, dup := claimed[space]; dup {
				return nil, nil, fmt.Errorf("卵 %s：聊天模板 %q 与 %s 的空间内落点都是 %q——拒孵：逐文件挂载时后挂的会盖住先挂的，"+
					"落点撞车等于有一份静默不见了", eggID, entry.ChatTemplate, prev, space)
			}
			claimed[space] = "聊天模板 " + entry.ChatTemplate
			b, err := roBind(host, space)
			if err != nil {
				return nil, nil, fmt.Errorf("卵 %s：%w", eggID, err)
			}
			binds = append(binds, b)
			rewrites = append(rewrites, pathRewrite{host: host, raw: p, space: space})
		}
	}
	return rewrites, binds, nil
}

// rewriteArgsToSpace 把引擎参数里的宿主侧输入改写成空间内路径。
//
// 只做**整 token 精确匹配**：不做子串泛化替换（那会误改 --lora / --cache-dir 之类别的路径），
// 认不出的路径原样传下去（不猜、不改——孵化器只照单执行）。
func rewriteArgsToSpace(args []string, rws []pathRewrite) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = arg
		for _, r := range rws {
			if r.matches(arg) {
				out[i] = r.space
				break
			}
		}
	}
	return out
}

// ── 设备与卡号（⑤，§6.7 第 2 类 / C①）─────────────────────────────────────

// declaredCardIndex 取卵声明的「用哪张卡」：
//   - env_req.devices 里的纯数字项（卡号）优先；
//   - 否则取 env_req.env 的卡号类变量（**单值**数字才认；"0,1" 这类多卡声明不认）。
//
// 返回 (卡号, 是否取到)；取不到 ⇒ 交缺省设备（绝不猜）。
func declaredCardIndex(entry *registry.ModelEntry) (int, bool) {
	if entry == nil || entry.EnvReq == nil {
		return 0, false
	}
	for _, d := range entry.EnvReq.Devices {
		if n, err := strconv.Atoi(strings.TrimSpace(d)); err == nil && n >= 0 && n < renderNodeBase {
			return n, true
		}
	}
	for _, k := range cardEnvKeys {
		v, ok := entry.EnvReq.Env[k]
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 && n < renderNodeBase {
			return n, true
		}
	}
	return 0, false
}

// hatchDevices 要暴露的设备节点（§6.7 第 2 类：精确 bind mount——挂多了是越界，挂少了跑不起来）：
//
//	声明了 /dev/... 显式节点 ⇒ 照单只挂这些（孵化器不推断、不加料）；
//	否则声明了卡号        ⇒ /dev/kfd + 该卡的渲染节点 /dev/dri/renderD{128+卡号}；
//	都没有               ⇒ nil（交孵化器缺省：/dev/kfd + /dev/dri/renderD128）。
func hatchDevices(entry *registry.ModelEntry, cardIdx int, cardKnown bool) []string {
	if nodes := declaredDeviceNodes(entry); len(nodes) > 0 {
		return nodes
	}
	if cardKnown {
		return []string{kfdNode, renderNode(cardIdx)}
	}
	return nil
}

// declaredDeviceNodes env_req.devices 里显式声明的设备节点（/dev/...；去重、保持声明顺序）。
func declaredDeviceNodes(entry *registry.ModelEntry) []string {
	if entry == nil || entry.EnvReq == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, d := range entry.EnvReq.Devices {
		d = strings.TrimSpace(d)
		if !strings.HasPrefix(d, "/dev/") || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

// renderNode 第 idx 张可见卡的渲染节点（约定：第 0 张 = renderD128）。
func renderNode(idx int) string { return fmt.Sprintf("/dev/dri/renderD%d", renderNodeBase+idx) }

// hatchEnv 按卵给环境变量（§6.7 第 4 类）：
//
//	env_req.env **照单执行**（含按卵覆盖 LD_LIBRARY_PATH；空串 = 显式清空，K2 的包装脚本就是
//	清它，§6.7 C③）；
//	**值**里出现的库宿主路径按 `:` 切段整段改写成空间内落点 /libs/<目录名>
//	（改写表来自 libRWs / engineLibMappings；空值原样保留 —— 清空是故意的）；
//	**据 lib_paths 生成空间内 LD_LIBRARY_PATH**（缺陷 6 修正，见下）；
//	声明了卡号再补一条 HIP_VISIBLE_DEVICES（卵已自己声明 HIP_VISIBLE_DEVICES 的以声明为准，不覆盖）。
//
// 未声明任何环境变量 ⇒ nil（不注入空壳）。
//
// 缺陷 6（2026-09-15 真机实测）：引擎的 RUNPATH 里写死的是**宿主**构建目录
// （`readelf -d` → `RUNPATH [/home/g01/llama.cpp-src/build-hip-flash/bin:]`）⇒ 空间内不设
// `LD_LIBRARY_PATH` 就是 `error while loading shared libraries: libllama-server-impl.so`
// + 单元 127（秒死）。所以孵化器**据 lib_paths 生成**它，不再指望每个引擎都手工声明。
//
// 生成口径（三条，都有意为之）：
//
//	① **只在卵声明了 lib_paths 时生成**（没声明就一个变量都不发明 —— 不猜）；
//	② **卵自己显式写了 LD_LIBRARY_PATH 就以它为准**（连空串「清空」语义也照旧），生成逻辑
//	   完全不插手 ⇒ 不会出现「声明与生成两头都写」的形态（谁赢说不清的那种）；
//	③ 生成值 = **声明的库落点在前、`/engine` 在最后**：
//	   - 声明的在前 ⇒ 卵点名的库版本优先命中（lib_paths 存在的理由就是「库版本按卵给，不许全机一套」）；
//	   - `/engine` 垫尾 ⇒ 引擎自己目录里的 impl/依赖库（宿主上正是 RUNPATH 指的那个目录）仍然找得到，
//	     新引擎不必再手工声明一次（这正是缺陷 6 的诉求：RUNPATH 写死宿主路径，空间内得有人补上）。
func hatchEnv(entry *registry.ModelEntry, cardIdx int, cardKnown bool, libRWs []pathRewrite, libSpaces []string) map[string]string {
	var env map[string]string
	if entry != nil && entry.EnvReq != nil && len(entry.EnvReq.Env) > 0 {
		env = make(map[string]string, len(entry.EnvReq.Env)+2)
		for k, v := range entry.EnvReq.Env {
			env[k] = rewriteEnvValue(v, libRWs)
		}
	}
	// ② 卵显式声明过 LD_LIBRARY_PATH（含空串 = 清空）⇒ 一句都不补
	declaredLibPath := false
	if entry != nil && entry.EnvReq != nil {
		_, declaredLibPath = entry.EnvReq.Env["LD_LIBRARY_PATH"]
	}
	if len(libSpaces) > 0 && !declaredLibPath {
		if env == nil {
			env = map[string]string{}
		}
		search := make([]string, 0, len(libSpaces)+1)
		search = append(search, libSpaces...)
		search = append(search, spaceEngineDir)
		env["LD_LIBRARY_PATH"] = strings.Join(search, ":")
	}
	if !cardKnown {
		return env
	}
	if env == nil {
		env = map[string]string{}
	}
	if _, ok := env["HIP_VISIBLE_DEVICES"]; !ok {
		env["HIP_VISIBLE_DEVICES"] = strconv.Itoa(cardIdx)
	}
	return env
}

// ── 一次性工作目录（⑥，§6.6）────────────────────────────────────────────────

// workDirRoot 每卵一次性工作目录的根（ZERG_WORK_DIR 可覆盖；缺省 ~/.zerg/work）。
func workDirRoot() string {
	if v := strings.TrimSpace(os.Getenv(EnvWorkDirRoot)); v != "" {
		return expandTilde(v)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".zerg", "work")
}

// ensureEggWorkDir 建（如缺）每卵一次性工作目录，返回其**宿主路径**。
//
// 返回值是**可写绑定的宿主侧来源**（ExtraRWBinds 的 host 半边），不是 `Spec.WorkDir` 的值
// ——后者是空间内路径（/work）。两者分工见 spaceWorkDir 的注释。
func ensureEggWorkDir(eggID string) (string, error) {
	root := workDirRoot()
	if root == "" {
		return "", fmt.Errorf("工作目录根算不出来（拿不到 HOME 且 %s 未配置）——拒孵", EnvWorkDirRoot)
	}
	dir := filepath.Join(root, safeEggDirName(eggID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("一次性工作目录 %s 建不出来：%w —— 拒孵（工作目录是卵的一次性空间，建不出就没有可写处）", dir, err)
	}
	return dir, nil
}

// ── KV 盘与可写绑定（④b / ⑥）─────────────────────────────────────────────────

// kvDiskHostDir 从引擎参数里取 KV 盘的**宿主目录**（`--kv-disk-dir` 紧随其后的那个值）。
//
// 返回 (目录, nil) = 参数里确实有 KV 盘；(“”, nil) = 没有 ⇒ 不加绑定、不做改写（**不编造**）。
//
// 为什么从参数里取、而不是照 §9.7④ 再按卵名算一遍：目录规则的真源在适配器的 kvDiskDir，
// 它的产物就是这个参数值。在这里重算是**第二套规则**，两处一漂移就会出现「绑的目录」与
// 「引擎写的目录」不是同一个 —— 那是最难查的一类静默故障。取执行面真值 ⇒ 绑定与改写必然一致。
//
// 目录不是宿主绝对路径 ⇒ 报错拒孵：相对路径在空间里会落到一次性工作目录上（本文件的
// `--chdir` 落点是 /work）⇒ KV 活不过收卵，与 §6.6 / §9.7④ 的「跨孵化保留」直接冲突。
// 这类形态要么是声明错、要么是 cmd: 写错，两种都不该静默降级跑起来（§6.9 同一精神）。
func kvDiskHostDir(args []string) (string, error) {
	for i, a := range args {
		if a != kvDiskFlag || i+1 >= len(args) {
			continue
		}
		dir := strings.TrimSpace(args[i+1])
		if dir == "" {
			return "", fmt.Errorf("%s 的值为空——拒孵（KV 盘目录拿不到，不许猜路径）", kvDiskFlag)
		}
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("KV 盘目录 %q 不是宿主绝对路径——拒孵：空间内只认绝对落点，相对路径会落进一次性工作目录（%s），"+
				"KV 就活不过收卵（§6.6 / §9.7④ 要求它落在跨孵化保留侧）", dir, spaceWorkDir)
		}
		return dir, nil
	}
	return "", nil
}

// rwBind 组装一条可写绑定（bwrap `--bind` 的 `<host>:<space>` 形式）。
//
// 宿主路径里带 `:` 一律拒孵：`:` 就是 <host>:<space> 的分隔符，带它的宿主路径会被对侧**拆错**
// （host 截在第一个冒号上）⇒ 静默挂到别的落点，比挂不上更糟。
func rwBind(host, space string) (string, error) {
	if strings.Contains(host, ":") {
		return "", fmt.Errorf("可写绑定的宿主路径 %q 含 `:`——拒孵（`:` 是 <host>:<space> 的分隔符，带它的路径会被拆错、挂到别的落点上）", host)
	}
	return host + ":" + space, nil
}

// eggDirUnsafe 卵名里不许进目录名的字符（防路径穿越：卵名可能带 / 或 ..，绝不许逃出工作根）。
var eggDirUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// safeEggDirName 把卵名收敛成**单层**目录名（其余字符换成 '-'，首尾的点划去）。
func safeEggDirName(eggID string) string {
	name := eggDirUnsafe.ReplaceAllString(eggID, "-")
	name = strings.Trim(name, "-.")
	if name == "" || name == "." || name == ".." {
		return "egg"
	}
	return name
}

// expandTilde 展开路径开头的 ~（只展开首个 ~；与仓内既有口径一致）。
func expandTilde(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == "~" {
		return home
	}
	return filepath.Join(home, p[2:])
}

// ── 引擎参数构造（裸 exec 路径与孵化路径共用同一份）──────────────────────────

// buildEngineArgv 构建引擎启动命令：适配器参数 → {file}/{port} 占位符替换 → cmd: 覆盖（兼容旧配置）
// → 空闲自退透传（P6/R6）。返回 (可执行路径, 参数)。
//
// 这段逻辑从 doStart 原样抽出，**只为「两处不漂移」**：孵化路径要把宿主权重路径改写成空间内路径，
// 必须拿到与裸 exec 路径逐字相同的参数序列（§6.7：参数由卵声明/适配器产出，孵化器只照单执行）。
func buildEngineArgv(modelName string, entry *registry.ModelEntry, port int) (string, []string) {
	// 构建启动命令（模型适配层：按模型名选适配器，管理启动参数/工具风格/重提示）
	// （P1 的「非主线引擎必须带 cmd:」守卫在 doStart 最前面，见 doStart 开头。）
	adp := modeladapter.Dispatch(modelName)
	cmdArgs := adp.BuildArgs(entry, port)
	cmdPath := ""
	if entry.Backend == "ds4-server" {
		cmdPath = "ds4-server"
	} else {
		cmdPath = detectLlamaServerPath()
	}
	// 替换 {file}/{port} 占位符（适配器可返回占位符）
	for i, arg := range cmdArgs {
		cur := strings.ReplaceAll(arg, "{port}", fmt.Sprintf("%d", port))
		cur = strings.ReplaceAll(cur, "{file}", entry.File)
		cmdArgs[i] = cur
	}

	// 如果有自定义 cmd（字符串，空格分隔），优先使用（兼容旧配置覆盖）
	if entry.Cmd != "" {
		cmdArgs = strings.Fields(string(entry.Cmd))
		for i, arg := range cmdArgs {
			cur := strings.ReplaceAll(arg, "{port}", fmt.Sprintf("%d", port))
			cur = strings.ReplaceAll(cur, "{file}", entry.File)
			cmdArgs[i] = cur
		}
		if len(cmdArgs) > 0 {
			cmdPath = cmdArgs[0]
		}
	}

	var execArgs []string
	if len(cmdArgs) > 0 && cmdArgs[0] == cmdPath {
		execArgs = cmdArgs[1:]
	} else {
		execArgs = cmdArgs
	}

	// P6（R6）：受管模型的空闲自退透传 —— 只对 llama 家族、且仅在配置了 ZERG_IDLE_SLEEP_S 时才加。
	// 位置刻意放在 cmd 覆盖之后：无论走适配器还是走 cmd:，最终参数都经过这一道。
	return cmdPath, applyIdleSelfSleep(execArgs, entry.Backend)
}

// ═══ 2026-09-15 修正：WorkDir 的语义缺陷与 KV 落点（Mr2109 拍板）═════════════
//
// 缺陷：`hatch.Spec.WorkDir` 的契约是**空间内**路径（孵化器会 `--chdir` 它），本文件原先把
// **宿主**每卵目录填了进去 ⇒ 那个目录在空间里根本不存在 ⇒ 真孵化必以 chdir 失败告终。
//
// 修法（三条，本文件与 hatch 包两侧都改了）：
//
//	① `Spec.ExtraRWBinds`（新）：`<host>:<space>` 的可写绑定（bwrap `--bind`），与只读的
//	   ExtraROBinds 并列、**同严格**（格式不对即拒孵）；
//	② `WorkDir` 填空间内 `/work`，宿主每卵目录 `~/.zerg/work/<卵名>/` 经 ExtraRWBinds 绑过去；
//	③ KV 盘（§6.6「跨孵化保留」侧 / §9.7④）：宿主目录可写绑到 `/kvdisk`，并把引擎参数里
//	   **精确等于**该宿主目录的那个值改写成 `/kvdisk`（只改整 token，不做前缀/子串替换）。
//
// 边界（**明确不改什么、为什么不改**，免得下一轮按「应该也能改吧」去猜）：
//   - 只认 `--kv-disk-dir <值>` 的**两段式**，且只认**第一个**：适配器（ds4.go）就是这个形状；
//     `--kv-disk-dir=<值>` 的等号形态不认（重写成 `--kv-disk-dir=/kvdisk` 属于对参数做加工，
//     本批不做）。
//   - 参数里没有该 flag ⇒ 不加绑定、不改写、**不报错**（没声明 KV 盘 / 适配器按归属铁律没发
//     该参数，两者都没有可绑的落点，不编造）。注意这是**执行面**判据：卵声明了 kv_disk 但
//     适配器没发（如 llama 系——§9.2 归属铁律）⇒ 同样不加绑定；反之 cmd: 里手写了该参数 ⇒
//     照样绑定与改写（引擎真会往那儿写，不绑就等于让它写一个空间内不存在的宿主路径）。
//   - 参数值不是宿主绝对路径 ⇒ **报错拒孵**（见 kvDiskHostDir）；「找不到就不改，不报错」
//     只适用于「压根没有该参数」，不适用于「有、但落点认不得」。
//   - 只改**引擎参数**，不改环境变量、不碰 `cmd:` 的其它部分、不动归属（谁发参数还是谁发）。
//
// ⚠ 未接（如实登记，别当已做）：`kv_disk.space_mb` 的**盘闸门**（到顶怎么办：清最旧还是拒孵）
// 与"N 天未用即清"的保留策略都还没有实现（§9.7 ②③ / §9.8），当前只保证「落点对 + 参数带上限」。

// ═══ 2026-09-15 修正：缺陷 5 / 6 / 11 / 12（第一枚卵真机实测）═══════════════════
//
// 报告：`docs/issues/报告-第一枚卵真机实测-20260915.md` §12。与本文件相关的四条逐条落到本文件
// ——**只到映射边界**（hatch 包的配方不归本文件，见 ⓵）。
//
//	5. **lib_paths 落点嵌在只读的 /engine 之下 ⇒ 整卵孵不起来**（原文
//	   `bwrap: Can't mkdir parents for /engine/lib/bin: Read-only file system`，单元 status=1，
//	   1–3ms 秒死）。改：落点 `/engine/lib/<名>` → **独立只读挂载点 `/libs/<名>`**
//	   （见 spaceLibRoot 注释：父目录 /libs 是新根上的普通目录，bwrap 建得出中间目录）。
//
//	6. **引擎 RUNPATH 写死宿主路径**（`readelf -d` → `RUNPATH [/home/g01/…/build-hip-flash/bin:]`）
//	   ⇒ 空间内不设 `LD_LIBRARY_PATH` 就是 `libllama-server-impl.so` 找不到 + 单元 127。
//	   改：**由孵化器据 `lib_paths` 生成空间内 `LD_LIBRARY_PATH`**（见 hatchEnv：声明的库落点
//	   在前、`/engine` 垫尾；**卵自己显式写了该变量就完全以卵的为准**，绝不两头都写）。
//
//	12. **「只挂该卵自己的权重」与实现不符**：旧实现挂的是权重**所在目录** ⇒ 同目录别的模型在
//	   空间内可见（真机 `ls /models` 列出 Qwen2.5-VL-32B / Qwen3.6-35B / Flash-Next 等）。
//	   改：**逐文件挂载**（entry.file + mmproj + chat_template 各有自己的只读绑定，落
//	   `/models/<基名>`；模板在权重目录之外时落 `/templates/<基名>`）⇒ 同目录别的模型既不在
//	   绑定里、也不在参数里（ship 了用例钉住）。一处副作用、**有意为之**：mmproj「必须与权重
//	   同目录」的限制**退役**（它的由来正是整目录挂载），改守真危险的那条 —— **同一空间目录下
//	   基名撞车即拒孵**（后挂的会盖住先挂的，等于有一份静默不见了）。
//
//	11. **env_req.mmap_max_count 在孵化器里无对应字段**：取「**暂不生效**」这一支（另一支是接进
//	   spec，但落地手段不存在，见 ⓶）。字段注释已明说（`registry/egg_decl.go`），本文件 ⑦b
//	   也写清为什么不映射 —— 宁可白纸黑字写「暂不生效」，也不留一个看着像保障、其实没人执行的东西。
//
// ⓵ **需要 hatch 侧同批同步（否则本次改动反而孵不起来）**：
//
//	hatch 包 `BuildBwrapArgv` 里那条 `--ro-bind s.WeightPath /models` 是**整目录**挂载，与逐文件
//	绑定**冲突**：/models 被它挂成只读之后，再往里绑 `/models/<基名>` 会以
//	`Can't create file …: Read-only file system` 失败（与缺陷 5 同一类）。hatch 侧要做的两件事：
//
//	  ① 去掉整目录挂载，换成「一个**空 /models** + 逐文件只读挂进来」（建议 `--tmpfs /models`
//	     打底，再由 ExtraROBinds 把每个文件落到 `/models/<基名>`）；
//	  ② **封闭性判据同步**：`hatch_check.go` 现在只认「/models 存在且 ro」这一条挂载，逐文件
//	     之后 /models 自己不再是那个只读挂载 ⇒ 判据须改成「/models 下每个声明文件都是 ro 挂载，
//	     且没有任何宿主目录被整挂进来」。报告 §12 缺陷 12 说的「判据侧同步」就是这条。
//
//	本文件这一侧已经就位：ExtraROBinds 里**只有**声明文件的逐文件绑定（基名与顺序可断言），
//	`WeightPath` 保留为「权重所在宿主目录」仅供出证/日志 —— hatch 侧去掉整挂后它自然退出挂载面。
//
// ⓶ **建议给 hatch.Spec 加的字段（本批用现有字段实现，请该包负责人/父代理拍）**：
//
//	- `WeightFiles []string`：要挂进 /models 的**逐个文件**（本文件已算好）。有了它，hatch 侧不必
//	  从 ExtraROBinds 里反推「哪些是权重文件」，`WeightPath` 也可以顺势退役（或改成单文件语义）；
//	- `MmapMaxCount int`：**不建议**照现在的形态加 —— `vm.max_map_count` 是**机器级 sysctl**、
//	  不是按进程的 rlimit，单元里下发不了；要真生效得配一条机器级前置校验（拿
//	  `/proc/sys/vm/max_map_count` 与声明值比，不够就拒孵），那是另一件事，得先拍板。
