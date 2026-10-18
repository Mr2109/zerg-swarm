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
//	   chat_template → 与权重同目录落 `/models/<基名>`、否则落 `/templates/<基名>`、ds4 的
//	   `vision` 视觉编码器 → `/models/<基名>`（都是逐文件只读绑定），参数里的宿主路径改写成
//	   对应空间内路径 ⇒ **权重所在目录整体不再挂载** —— 同目录的别的模型与整个 /data 在空间内
//	   根本不存在（§6.6 / §9.3）；
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
// ★ **唯一一处例外**（卵沙箱第 6 步）：声明里给了 `egg`（.egg 件的宿主路径）时，本文件会在
// 翻译之前**先把那枚蛋展开**到落点目录（`eggpkg.ExpandEgg` · 幂等、靠 `.complete` 跳过）。
// 为什么放在这里而不是另起一层：`egg_dir` 从前是**手写 yaml 键**（部署事实），现在要变成
// 「由蛋推出来」的事实，而 `egg_dir` 的唯一消费点就是本函数 —— 在这一处展开，就能保证
// 「挂的目录」与「展开的目录」永远是同一个，不会出现两套路径规则。
//
// ⚠ 已知待拍板 / 未接线（如实标出，不擅自改 hatch 包的 node-a 实测配方）：
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

	"github.com/Mr2109/zerg-swarm/agent/internal/eggpkg"
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

// weightShardRe 权重分片 gguf 的**尾式**：`-<五位序号>-of-<五位总片数>.gguf`
// （llama.cpp `gguf-split` 的产物命名；edge-node 实测样本 `Qwen3.8-Flash-Next-UD-Q4_K_XL-00001-of-00004.gguf`）。
//
// 只认尾式、不锚前缀：前缀（组键）在 spaceInputMappings 里按「截掉尾段」现算，口径与
// scripts/calib/calib-x3-model.sh 的「权重分片」段**同源**（组键 ＝ 前缀 ＋ 总片数）。
var weightShardRe = regexp.MustCompile(`-([0-9]{5})-of-([0-9]{5})\.gguf$`)

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
	// ③ 引擎：**从蛋孵化**时挂载源整体换成蛋展开目录（`<egg_dir>/engine` → `/engine`）；
	//    不是从蛋孵化时照旧取宿主目录（逐字不变 —— 蛋相关分支一行都不走）。
	egg, err := eggWiringFor(eggID, entry, engineImpl)
	if err != nil {
		return spec, err
	}
	if egg != nil {
		spec.EggRoot = egg.Root
		spec.EggLoaderDir = egg.LoaderDir
		spec.EggBinDir = egg.BinDir
		spec.EnginePathInSpace = egg.EnginePath
		// 蛋孵化时权重也来自蛋内 ⇒ 出证/日志里的「权重所在目录」跟着说实话
		// （否则日志会印出**宿主旧路径**，「原目录可删」的证据面就不干净了，GAP-20261018-16）。
		spec.WeightPath = egg.ModelsDir
	} else {
		hostEngine, herr := engineHostPath(entry, engineImpl)
		if herr != nil {
			return spec, fmt.Errorf("卵 %s：%w", eggID, herr)
		}
		spec.EngineRoots = []string{filepath.Dir(hostEngine)}
		spec.EnginePathInSpace = spaceEngineDir + "/" + filepath.Base(hostEngine)
	}

	// ④ 参数：与裸 exec 路径**同一份构造**（buildEngineArgv），再把宿主侧输入改写成空间内路径
	// 蛋孵化时挂载源整体换成蛋内副本（`<egg>/models/<基名>`）；宿主孵化时 eggModelsDir = "" ⇒ 逐字不变。
	eggModelsDir := ""
	if egg != nil {
		eggModelsDir = egg.ModelsDir
	}
	rewrites, roBinds, err := spaceInputMappings(eggID, entry, hostWeightFile, eggModelsDir)
	if err != nil {
		return spec, err
	}
	_, engineArgs := buildEngineArgv(eggID, entry, port)

	// ④a 库目录（§6.7 第 3 类 / env_req.lib_paths）：每个宿主库目录**只读**落到
	//     /libs/<目录名>（独立只读挂载点，缺陷 5），并把 **env 值**里出现的库宿主路径改写成
	//     对应空间内路径。同名 / 不存在 / 非绝对一律拒孵（见 engineLibMappings）。
	// ④a 库目录：两条路**互斥** ——
	//     蛋孵化：声明的每个库目录取**蛋内副本** `<egg_dir>/libs/<目录名>`，整棵 libs 一次落到
	//     `/libs`（不 stat 宿主路径：蛋孵化时宿主上有没有那些目录不再是条件，蛋里有才是条件）；
	//     宿主孵化：照旧（每目录一条 `/libs/<目录名>`，宿主上不存在即拒孵）。
	var libRewrites []pathRewrite
	var libSpaces []string
	if egg != nil {
		libRewrites, libSpaces = egg.LibRewrites, egg.LibSpaces
		roBinds = append(roBinds, egg.Binds...)
	} else {
		var libROBinds []string
		libRewrites, libROBinds, libSpaces, err = engineLibMappings(eggID, entry)
		if err != nil {
			return spec, err
		}
		roBinds = append(roBinds, libROBinds...)
	}

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
	//
	// ★ GAP-20261018-16：这一格必须是**绑定源**（蛋孵化时 = 蛋内副本），因为 hatch 侧的
	//   validateWeightFiles 当契约核「落点上绑的宿主 == 声明里写的」；写声明路径会让孵化器
	//   在 spec 验收期判红（而空间里其实挂对了）。蛋内副本同样是宿主上的绝对路径。
	for _, rw := range rewrites {
		if rw.space == spaceWeightsDir || strings.HasPrefix(rw.space, spaceWeightsDir+"/") {
			spec.WeightFiles = append(spec.WeightFiles, rw.bindSrc())
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
	var engineLibDirs []string
	if egg != nil {
		engineLibDirs = egg.EngineLibDirs
	}
	spec.Env = hatchEnv(entry, cardIdx, cardKnown, libRewrites, libSpaces, engineLibDirs)
	// ⑤b rocBLAS 的 dlopen 数据目录（DT_NEEDED 里看不见的那 556 MB）：
	//     蛋孵化且蛋里带了 rocblas/<版本>/library ⇒ 显式下发 ROCBLAS_TENSILE_LIBPATH
	//     （卵自己声明了就以它为准 —— 声明面永远优先，不两头都写）。
	if egg != nil && egg.RocblasLibPath != "" {
		if spec.Env == nil {
			spec.Env = map[string]string{}
		}
		if _, declared := spec.Env["ROCBLAS_TENSILE_LIBPATH"]; !declared {
			spec.Env["ROCBLAS_TENSILE_LIBPATH"] = egg.RocblasLibPath
		}
	}
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

// ── 蛋孵化（卵沙箱第 5 步）：挂载源换成「蛋展开后的目录」 ──────────────────────
//
// 声明面（卵声明的 inline 扩展键；读法与 `vision` / `ssd` 同源 = `entry.Custom[...]`。
// ★ 本单**不给 registry 加字段**：蛋的展开目录是**部署事实**、不是模型属性，放声明里即可）：
//
//	egg_dir          蛋**展开后**的宿主目录（空 = 不是从蛋孵化 ⇒ 本文件所有蛋分支都不走）
//	egg_loader_dir   蛋内 loader 目录（可选；缺省 `<egg_dir>/engine/lib`）
//	egg_rocblas_dir  蛋内 rocBLAS 数据目录的**父目录**（可选；缺省 `<egg_dir>/engine/rocblas`）
//
// 蛋内布局（**约定**，与 `cocoon pack --engine-dir` 的落点同源 —— 打包端把每个
// `--engine-dir <目录>` 整棵树收进 `engine/<目录基名>/…`）：
//
//	<egg_dir>/engine/<目录基名>/<可执行件>            ← 引擎（基名原样保留）
//	<egg_dir>/engine/lib/ld-linux-x86-64.so.2         ← loader（缺省 loader 目录）
//	<egg_dir>/libs/<目录名>/…                          ← 声明库目录的蛋内副本
//	<egg_dir>/engine/rocblas/<版本>/library/*.hsaco    ← rocBLAS 的 dlopen 数据（第三个 --engine-dir）
//
// 空间内落点（**与宿主上跑的时候逐字相同** —— 二进制里的绝对路径才继续成立）：
//
//	<egg_dir>/engine         → /engine        （hatch.BuildBwrapArgv ④）
//	<egg_dir>/engine/lib     → /lib64 与 /lib  （hatch.BuildBwrapArgv ①b，PT_INTERP 的落点）
//	<egg_dir>/libs           → /libs          （本文件产的 ExtraROBinds）
//	<egg_dir>/engine/rocblas → /opt/rocm/lib/rocblas
//	                           ＋ ROCBLAS_TENSILE_LIBPATH=/opt/rocm/lib/rocblas/<版本>/library
//
// ★ rocBLAS 那两条为什么都要（一手依据见 `~/.zerg/state/zerg-蛋内起沙箱接通-20261018.md`）：
//
//	① **固定落点**：目标机上 `librocblas.so.5.1` 的字节里就有默认绝对路径
//	   `/rocblas/5.1.0/library`（`strings` 现读命中），而实测机上**没有任何**全局
//	   `ROCBLAS_TENSILE_LIBPATH` 而引擎照常跑 ⇒ 它走的就是那条编译期绝对路径；
//	② **环境变量**：AMD 官方 rocBLAS 安装指南（Linux_Install_Guide · Static library 节）逐字
//	   「This runtime folder can be moved elsewhere, but the environment variable
//	   `ROCBLAS_TENSILE_LIBPATH` must be set to the new location.」 ⇒ 目录一搬就必须给变量。
//	两条同时给：变量优先（换版本/换落点都稳），绝对路径兜底（变量被别处抹掉也不炸）。
//	★ 版本号**不写死**：蛋内 `<rocblas>/` 下必须**恰好一个**版本层，否则拒孵（见 eggRocblas）。
type eggWiring struct {
	Root           string        // 蛋展开目录（宿主）
	ModelsDir      string        // 蛋内权重目录（宿主；`/models` 下的一切**只**从这里取，GAP-20261018-16）
	LoaderDir      string        // 蛋内 loader 目录（宿主；落到空间 /lib64 与 /lib）
	BinDir         string        // 蛋内 sysroot/bin（宿主；落到空间 /bin —— 里面那枚 sh 是包装 exec 的入口）
	EnginePath     string        // 引擎在空间内的路径（/engine/<目录基名>/<可执行名>）
	EngineLibDirs  []string      // 喂 LD_LIBRARY_PATH 的引擎侧目录（空间内；顺序即搜索顺序）
	LibRewrites    []pathRewrite // 声明库目录的宿主 → 空间改写表（供 env 值改写）
	LibSpaces      []string      // 声明库目录的空间落点（/libs/<目录名>）
	Binds          []string      // 本层新增的只读绑定（`<host>:<space>`，进 ExtraROBinds）
	RocblasLibPath string        // ROCBLAS_TENSILE_LIBPATH 的值（空间内 .hsaco 目录；无则空）
}

// 蛋孵化用到的声明键与约定落点。
const (
	// eggKeyEggFile —— 声明里的**蛋件路径**（宿主绝对路径）。给了它 ⇒ 本文件负责展开，
	// `egg_dir` 退化成「展开到哪」的**可选**覆盖（缺省 `~/.zerg/eggs/<卵名>`）—— 这就是
	// 「`egg_dir` 不再是手写 yaml」的落点：手写的是**蛋**，目录由它推。
	eggKeyEggFile    = "egg"
	eggKeyEggDir     = "egg_dir"
	eggKeyLoaderDir  = "egg_loader_dir"
	eggKeyRocblasDir = "egg_rocblas_dir"
	// eggEngineSubdir / eggLibsSubdir —— 蛋展开目录下的两个约定子目录。
	eggEngineSubdir = "engine"
	eggLibsSubdir   = "libs"
	// eggModelsSubdir —— 蛋内「权重（/models 下的一切）」的约定子目录（GAP-20261018-16）。
	//
	// 蛋孵化时**权重也走蛋内**：挂载源 = `<egg_dir>/models/<基名>`（不在蛋里就去 stat 宿主路径）。
	// 片组同形：打包端把权重树原样收进「源目录/models/」，四片按**原基名**平铺 ⇒ 展开后是**普通文件**
	// （不是蛋内偏移），与「挂宿主那件」同形 ⇒ mmap 语义不变、llama.cpp 不用改。
	// 这条是「原目录可删」成立的前提：否则宿主权重目录一改名，spec 期就 502（实测逐字见
	// `~/.zerg/state/zerg-蛋内权重-20261018.md`）。
	eggModelsSubdir = "models"
	// eggSysrootSubdir —— 蛋内「系统镜像」的约定子目录（`sysroot/lib` 与 `sysroot/bin` 同层）。
	//
	// `sysroot/bin` 提供空间里的 `/bin`：蛋模式**不挂宿主 `/usr`**（GAP-20261018-11）⇒ `/bin` 没有
	// 兜底来源，而**凡声明了按卵环境变量**的卵入口就是 `/bin/sh -c 'export …; exec "$@"'`
	// （bwrap `--setenv` 在 0.11.1 上必失败）⇒ 缺它逐字 `bwrap: execvp /bin/sh: No such file or
	// directory`（rc=1，edge-node 实测，GAP-20261018-12）。取现成段位（与 `sysroot/lib` 同层，打包端本
	// 就把它当普通源目录内容收进蛋），✗ 不另起一个蛋内路径。
	// 与 `hatch` 包的 `eggSysrootSubdir` **同值**（跨包契约：两个包不互相依赖）。
	eggSysrootSubdir = "sysroot"
	// eggBinSubdir —— `sysroot` 下放 `/bin` 那一层；蛋里那枚 sh 的路径 = `sysroot/bin/sh`。
	eggBinSubdir = "bin"
	// eggRocblasSpaceDir rocBLAS 数据目录在空间内的落点 = **宿主上的默认绝对路径**
	// （它的由来见上面文件头 ①；版本层再往下拼）。
	eggRocblasSpaceDir = "/opt/rocm/lib/rocblas"
)

// eggWiringFor 把「卵声明 ＋ 蛋展开目录」翻成孵化侧的蛋挂载事实（**fail-closed**）。
//
// 返回 (nil, nil) = 不是从蛋孵化（`egg_dir` 没声明）⇒ 调用方走原路，行为逐字不变。
// 其余任何「声明了却对不上」的形态都返回错误并**具名点名**（缺哪个目录 / 哪个可执行件 /
// 哪个版本层），绝不退回宿主挂载 —— 静默换源是这类改动最坏的失败形态。
func eggWiringFor(eggID string, entry *registry.ModelEntry, engineImpl string) (*eggWiring, error) {
	if entry == nil {
		return nil, nil
	}
	// ⓪ 先展开蛋（若声明了 `egg`）：`egg_dir` 从此是**推导结果**而不是手写事实。
	eggFile := strings.TrimSpace(eggCustomString(entry, eggKeyEggFile))
	root := strings.TrimSpace(eggCustomString(entry, eggKeyEggDir))
	if eggFile == "" && root == "" {
		return nil, nil
	}
	if eggFile != "" {
		res, err := ensureEggExpanded(eggID, entry)
		if err != nil {
			return nil, err
		}
		root = res.Dst // ★ 挂载源 = **这次展开的落点**（同一个值 ⇒ 两处不可能漂）
	}
	root = filepath.Clean(expandTilde(root))
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("卵 %s：%s=%q 不是宿主侧绝对路径——拒孵（蛋展开目录必须能直接 stat）",
			eggID, eggKeyEggDir, root)
	}
	if st, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("卵 %s：蛋展开目录 %s 读不到（%v）——拒孵（声明了蛋却拿不到，不退回宿主挂载）",
			eggID, root, err)
	} else if !st.IsDir() {
		return nil, fmt.Errorf("卵 %s：蛋展开目录 %s 不是目录——拒孵", eggID, root)
	}
	w := &eggWiring{Root: root}

	// ① loader 目录（默认 <root>/engine/lib）：必须是目录，且**真的含 loader**
	loader := strings.TrimSpace(eggCustomString(entry, eggKeyLoaderDir))
	if loader == "" {
		loader = filepath.Join(root, eggEngineSubdir, "lib")
	} else {
		loader = expandTilde(loader)
		if !filepath.IsAbs(loader) {
			loader = filepath.Join(root, loader)
		}
	}
	loader = filepath.Clean(loader)
	if st, err := os.Stat(loader); err != nil {
		return nil, fmt.Errorf("卵 %s：蛋内 loader 目录 %s 读不到（%v）——拒孵（PT_INTERP 写死 /lib64/ld-linux-*.so.2，"+
			"这个目录要落到 /lib64；没有它 exec 直接 ENOENT）", eggID, loader, err)
	} else if !st.IsDir() {
		return nil, fmt.Errorf("卵 %s：蛋内 loader 目录 %s 不是目录——拒孵", eggID, loader)
	}
	hasLoader, err := dirHasLoader(loader)
	if err != nil {
		return nil, fmt.Errorf("卵 %s：读不了蛋内 loader 目录 %s：%w", eggID, loader, err)
	}
	if !hasLoader {
		return nil, fmt.Errorf("卵 %s：蛋内 loader 目录 %s 里没有 ld-linux*/ld-musl*——拒孵"+
			"（PT_INTERP 的落点无处安放：这条绑定会把一个空目录挂到 /lib64）", eggID, loader)
	}
	w.LoaderDir = loader

	// ①b 蛋内 `/bin` 的来源（`<root>/sysroot/bin`，里面必须有一枚 `sh`）
	//
	// 为什么是硬要求（GAP-20261018-12）：蛋模式**不挂宿主 `/usr`**（GAP-20261018-11）⇒ usrmerge 那条
	// `--symlink usr/bin /bin` 退场，空间里没有任何 `/bin` 来源；而本函数产出的 spec **恒带**
	// `LD_LIBRARY_PATH`（⑤ 的 hatchEnv 会生成）⇒ 入口必然是包装 exec `/bin/sh -c 'export …; exec "$@"'`
	// ⇒ 空根档逐字 `bwrap: execvp /bin/sh: No such file or directory`（rc=1，edge-node 真跑）。
	// 取**现成段位** `sysroot/bin`（与 `sysroot/lib` 同层），不另起蛋内路径、不改蛋格式。
	binDir := filepath.Join(root, eggSysrootSubdir, eggBinSubdir)
	if st, serr := os.Stat(binDir); serr != nil || !st.IsDir() {
		return nil, fmt.Errorf("卵 %s：蛋里没有 %s/%s 这一层（%v）——拒孵：蛋模式不挂宿主 /usr，"+
			"空间里的 /bin/sh 只能由蛋提供（打蛋时把一枚 sh 收进「源目录/%s/%s/sh」，"+
			"它要在蛋内 sysroot 里跑得起来：静态或闭包齐）",
			eggID, eggSysrootSubdir, eggBinSubdir, serr, eggSysrootSubdir, eggBinSubdir)
	}
	const eggBinShName = "sh"
	shSt, sherr := os.Stat(filepath.Join(binDir, eggBinShName))
	if sherr != nil {
		return nil, fmt.Errorf("卵 %s：蛋内 %s/%s 里没有 %q（%v）——拒孵：包装 exec 的入口就是它"+
			"（缺的空根档逐字 `bwrap: execvp /bin/sh: No such file or directory`，rc=1）",
			eggID, eggSysrootSubdir, eggBinSubdir, eggBinShName, sherr)
	}
	if !shSt.Mode().IsRegular() || shSt.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("卵 %s：蛋内 %s/%s/%s 不是可执行的普通件（mode=%v）——拒孵："+
			"空间里 `/bin/sh` 起不来（包装 exec 会 execve 失败）",
			eggID, eggSysrootSubdir, eggBinSubdir, eggBinShName, shSt.Mode())
	}
	w.BinDir = binDir

	// ①c 蛋内 `/models` 的来源（`<root>/models`，GAP-20261018-16）
	//
	// 从蛋孵化时**权重也走蛋内**：空间里 `/models/<基名>` 的挂载源一律是 `<root>/models/<基名>`
	// （逐件的存在性由 spaceInputMappings 用 eggWeightSource 核，缺哪件点哪件）。
	// 这一层本身缺失 ⇒ 现在具名拒：否则「蛋声明了要权重却没有权重层」会被推到最后一件件的
	// 具名拒上（信息更散），而打包端最该看的是**这一层没收进蛋**。
	// ✗ 没有「蛋里没有就回落宿主」这一档：回落 = 原目录永远删不掉，而「原目录可删」正是本单目标。
	modelsDir := filepath.Join(root, eggModelsSubdir)
	if st, serr := os.Stat(modelsDir); serr != nil || !st.IsDir() {
		return nil, fmt.Errorf("卵 %s：蛋里没有 %s/ 这一层（%v）——拒孵：从蛋孵化时空间内 %s 下的一切只从蛋内取"+
			"（打包时把权重树收进「源目录/%s/」，片组按**原基名**平铺）；✗ 不退回宿主挂载",
			eggID, eggModelsSubdir, serr, spaceWeightsDir, eggModelsSubdir)
	}
	w.ModelsDir = modelsDir

	// ② 引擎可执行：在蛋里按**基名**找，必须唯一
	exeBase, err := eggExeBase(entry, engineImpl)
	if err != nil {
		return nil, fmt.Errorf("卵 %s：%w", eggID, err)
	}
	relExe, err := eggFindExecutable(root, exeBase)
	if err != nil {
		return nil, fmt.Errorf("卵 %s：%w", eggID, err)
	}
	w.EnginePath = "/" + filepath.ToSlash(relExe)
	w.EngineLibDirs = append(w.EngineLibDirs, "/"+filepath.ToSlash(filepath.Dir(relExe)))
	// loader 目录也是引擎侧的库目录（loader 之外还有 libc/libm/…）：空间路径由它在蛋内的相对位置算，
	// 不在蛋内（配置把 loader 指到蛋外）时给不出空间路径 ⇒ 只靠 ①b 的 /lib64 /lib 覆盖，这里不编。
	if rel, rerr := filepath.Rel(root, loader); rerr == nil && !strings.HasPrefix(rel, "..") {
		if sp := "/" + filepath.ToSlash(rel); sp != w.EngineLibDirs[0] {
			w.EngineLibDirs = append(w.EngineLibDirs, sp)
		}
	}

	// ③ 库目录：整棵 <root>/libs → /libs；声明的每个目录名在蛋里必须真有
	libsDir := filepath.Join(root, eggLibsSubdir)
	libsOK := false
	if st, serr := os.Stat(libsDir); serr == nil && st.IsDir() {
		bind, berr := roBind(libsDir, spaceLibRoot)
		if berr != nil {
			return nil, fmt.Errorf("卵 %s：%w", eggID, berr)
		}
		w.Binds = append(w.Binds, bind)
		libsOK = true
	}
	if entry.EnvReq != nil {
		seen := map[string]string{}
		for _, raw := range entry.EnvReq.LibPaths {
			decl := strings.TrimSpace(raw)
			if decl == "" {
				return nil, fmt.Errorf("卵 %s：env_req.lib_paths 里有一项是空串——拒孵（库目录拿不到，不猜）", eggID)
			}
			host := filepath.Clean(expandTilde(decl))
			name := filepath.Base(host)
			if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
				return nil, fmt.Errorf("卵 %s：库目录 %q 推不出目录名——拒孵（蛋内落点 %s/<目录名> 定不下来）",
					eggID, decl, eggLibsSubdir)
			}
			if prev, dup := seen[name]; dup {
				return nil, fmt.Errorf("卵 %s：库目录 %q 与 %q 的目录名都是 %q——拒孵：两个库目录同名会互相遮挡",
					eggID, prev, decl, name)
			}
			seen[name] = decl
			eggLib := filepath.Join(libsDir, name)
			if st, serr := os.Stat(eggLib); serr != nil || !st.IsDir() {
				return nil, fmt.Errorf("卵 %s：蛋里没有 %s/%s（声明的库目录 %q 的蛋内副本）——拒孵："+
					"打包时要把该目录收进蛋（`libs/` 这一层由构建脚本给），否则空间里 /libs/%s 是空的",
					eggID, eggLibsSubdir, name, decl, name)
			}
			space := spaceLibRoot + "/" + name
			w.LibSpaces = append(w.LibSpaces, space)
			w.LibRewrites = append(w.LibRewrites, pathRewrite{host: host, raw: decl, space: space})
		}
		if len(entry.EnvReq.LibPaths) > 0 && !libsOK {
			return nil, fmt.Errorf("卵 %s：声明了 %d 个库目录，但蛋里没有 %s/ 这一层——拒孵"+
				"（空间内 /libs 会是空的，引擎照旧 127）", eggID, len(entry.EnvReq.LibPaths), eggLibsSubdir)
		}
	}

	// ④ rocBLAS 数据目录（可选；显式声明了就必须在）
	rocRaw := strings.TrimSpace(eggCustomString(entry, eggKeyRocblasDir))
	rocDeclared := rocRaw != ""
	roc := rocRaw
	if roc == "" {
		roc = filepath.Join(root, eggEngineSubdir, "rocblas")
	} else {
		roc = expandTilde(roc)
		if !filepath.IsAbs(roc) {
			roc = filepath.Join(root, roc)
		}
	}
	roc = filepath.Clean(roc)
	if st, serr := os.Stat(roc); serr == nil && st.IsDir() {
		libPath, verr := eggRocblasLibPath(eggID, roc)
		if verr != nil {
			return nil, verr
		}
		bind, berr := roBind(roc, eggRocblasSpaceDir)
		if berr != nil {
			return nil, fmt.Errorf("卵 %s：%w", eggID, berr)
		}
		w.Binds = append(w.Binds, bind)
		w.RocblasLibPath = libPath
	} else if rocDeclared {
		return nil, fmt.Errorf("卵 %s：%s=%q 在蛋里读不到（%v）——拒孵（声明了 rocBLAS 数据目录就必须在蛋里）",
			eggID, eggKeyRocblasDir, roc, serr)
	}
	return w, nil
}

// eggCustomString 读卵声明的 inline 扩展键（非字符串 / 不存在都当空 —— 与 `vision` 同读法）。
func eggCustomString(entry *registry.ModelEntry, key string) string {
	if entry == nil || entry.Custom == nil {
		return ""
	}
	v, _ := entry.Custom[key].(string)
	return v
}

// eggExeBase 引擎可执行件的**基名**（蛋内按基名找；不按宿主路径找 —— 蛋孵化时宿主上可能根本没有它）。
//
// 序与 engineHostPath 同源：cmd 首词 → engine_impl 末段 → 主线探测结果的基名。
func eggExeBase(entry *registry.ModelEntry, engineImpl string) (string, error) {
	if cmd := strings.TrimSpace(string(entry.Cmd)); cmd != "" {
		fields := strings.Fields(cmd)
		if len(fields) == 0 {
			return "", fmt.Errorf("cmd: 声明为空——拒孵（引擎可执行件名拿不到，不猜）")
		}
		return filepath.Base(fields[0]), nil
	}
	if impl := strings.TrimSpace(engineImpl); impl != "" {
		return filepath.Base(impl), nil
	}
	p := strings.TrimSpace(mainlineEngineProbe())
	if p == "" {
		return "", fmt.Errorf("引擎可执行件名拿不到（cmd / engine_impl 都空，主线探测也没结果）——拒孵")
	}
	return filepath.Base(p), nil
}

// eggFindExecutable 在蛋内按基名找引擎，返回**相对蛋根的路径**（`engine/<基名>/<件>`）。
//
// 只认两处：`engine/<件>` 与 `engine/<基名>/<件>`（打包端 `--engine-dir` 的两种形态）。
// 命中 **必须恰好一处** —— 0 处（没打进蛋）与多处（两个 --engine-dir 都带同名件）都拒孵：
// 挑一个就是在猜「跑的是哪一份引擎」。
func eggFindExecutable(root, base string) (string, error) {
	if base == "" || base == "." || base == ".." || base == string(filepath.Separator) {
		return "", fmt.Errorf("引擎可执行件名 %q 不合法——拒孵", base)
	}
	engineRoot := filepath.Join(root, eggEngineSubdir)
	ents, err := os.ReadDir(engineRoot)
	if err != nil {
		return "", fmt.Errorf("蛋里没有 %s/ 这一层（%v）——拒孵：`cocoon pack --engine-dir` 的落点就是它",
			eggEngineSubdir, err)
	}
	var hits []string
	add := func(p string) {
		if st, serr := os.Stat(p); serr == nil && !st.IsDir() {
			hits = append(hits, p)
		}
	}
	add(filepath.Join(engineRoot, base))
	for _, e := range ents {
		if e.IsDir() {
			add(filepath.Join(engineRoot, e.Name(), base))
		}
	}
	switch len(hits) {
	case 0:
		return "", fmt.Errorf("蛋内找不到引擎可执行件 %q（找过 %s/ 与 %s/*/）——拒孵：打包时 `--engine-dir` 没把引擎目录收进来",
			base, eggEngineSubdir, eggEngineSubdir)
	case 1:
		rel, rerr := filepath.Rel(root, hits[0])
		if rerr != nil {
			return "", rerr
		}
		return rel, nil
	default:
		return "", fmt.Errorf("蛋内有 %d 份同名引擎可执行件（%s）——拒孵：说不清该跑哪一份（两个 --engine-dir 撞了基名/件名）",
			len(hits), strings.Join(hits, " · "))
	}
}

// eggRocblasLibPath 蛋内 rocBLAS 数据目录 → 空间内 .hsaco 目录（`ROCBLAS_TENSILE_LIBPATH` 的值）。
//
// 形如 `<roc>/<版本>/library`；版本层**必须恰好一个**（不写死 5.1.0：换 ROCm 版本不该改代码，
// 但两个版本并存时必须有人拍板用哪个 ⇒ 拒孵，不猜）。
func eggRocblasLibPath(eggID, roc string) (string, error) {
	ents, err := os.ReadDir(roc)
	if err != nil {
		return "", fmt.Errorf("卵 %s：读不了蛋内 rocBLAS 目录 %s：%w", eggID, roc, err)
	}
	var vers []string
	for _, e := range ents {
		if e.IsDir() {
			vers = append(vers, e.Name())
		}
	}
	if len(vers) != 1 {
		return "", fmt.Errorf("卵 %s：蛋内 rocBLAS 目录 %s 下有 %d 个版本层（%v）——拒孵：要恰好一个"+
			"（多个版本并存时说不清该用哪一份 kernel）", eggID, roc, len(vers), vers)
	}
	libDir := filepath.Join(roc, vers[0], "library")
	if st, serr := os.Stat(libDir); serr != nil || !st.IsDir() {
		return "", fmt.Errorf("卵 %s：蛋内 rocBLAS 版本层 %s 里没有 library/（%v）——拒孵："+
			"ROCBLAS_TENSILE_LIBPATH 要指的就是这一层（里面是 .hsaco/.dat）", eggID, vers[0], serr)
	}
	return eggRocblasSpaceDir + "/" + vers[0] + "/library", nil
}

// dirHasLoader 该目录里有没有 loader 本体（`ld-linux*` 或 musl 档 `ld-musl*`）。
func dirHasLoader(dir string) (bool, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, "ld-linux") || strings.HasPrefix(n, "ld-musl") {
			return true, nil
		}
	}
	return false, nil
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
	src   string // 挂载源；**只有从蛋孵化时才填**（= 蛋内副本 `<egg>/models/<基名>`），空 ⇒ 同 host
	raw   string // 卵声明里的原样字符串（可能带 ~；参数/环境变量里出现的可能正是它）
	space string // 空间内路径
}

// bindSrc 这条改写要**绑**的那件（宿主模式逐字 = host；蛋模式 = 蛋内副本）。
//
// 为什么把「改写的匹配键」与「绑定的源」分成两格：引擎参数里出现的永远是**声明里的宿主路径**
// （`-m {file}` 由 buildEngineArgv 展开），而要被 mmap 的字节在蛋模式下来自蛋内副本 ——
// 两者必须分开，否则要么改写匹配不到、要么绑的还是将被删掉的宿主件（GAP-20261018-16）。
func (r pathRewrite) bindSrc() string {
	if r.src != "" {
		return r.src
	}
	return r.host
}

// matches 待改写的**整段/token**是不是这一条声明的宿主路径。两种写法任一相符即算：
// 展开后的宿主路径（host）与声明里的原样字符串（raw，如带 `~` 的形态）。
//
// 刻意**只有相等**：没有前缀匹配、没有子串匹配、没有大小写宽容 —— 改错了比不改更难查。
func (r pathRewrite) matches(tok string) bool {
	return (r.host != "" && tok == r.host) || (r.raw != "" && tok == r.raw)
}

// visionPath 取卵声明里的视觉编码器路径（ds4 家族的口径：「有值才算声明」）。
//
// 键名与读法**与子端适配器同源**（`modeladapter/ds4.go`：`entry.Custom["vision"].(string)` 且非空
// 才发 `--vision`）——注册表里写哪个键、适配器读哪个键、本函数绑哪个键，三者必须是同一个；
// ✗ 不在这里另立键名（自创键名 = 绑了一份引擎根本不读的文件，看着像修好了）。
// 非字符串 / 空串一律当「未声明」（与适配器同口径，绝不猜）。
func visionPath(entry *registry.ModelEntry) string {
	if entry == nil {
		return ""
	}
	v, ok := entry.Custom["vision"].(string)
	if !ok {
		return ""
	}
	return v
}

// spaceInputMappings 算出需要改写的宿主侧输入，以及需要额外只读绑定的清单（ExtraROBinds）。
//
// **逐文件**覆盖三类输入（都只读；缺陷 12 修正：整**目录**不再挂载 —— 旧实现把权重所在目录
// 整体挂成 /models，真机上空间内 `ls /models` 能列出同目录的别的模型，与「只挂该卵自己的权重」
// 的表述直接矛盾）：
//   - 权重文件（必给）：`<entry.file>:/models/<基名>`；
//   - 视觉投影 mmproj（可选）：`<mmproj>:/models/<基名>`（逐文件挂载后不再要求「与权重同目录」
//     —— 那条限制的由来就是整目录挂载，理由已随之消失；真危险的是**基名撞车**，见下）；
//   - 视觉编码器 vision（可选，ds4 家族）：`<vision>:/models/<基名>`——它是注册表里的**另一枚
//     inline 键**（键名与读法跟子端适配器同源：`modeladapter/ds4.go` 的 `entry.Custom["vision"]`），
//     故不并进 mmproj，但处理完全同款（逐文件只读绑定 + 参数改写）；
//   - 聊天模板 chat_template（可选）：文件存在才处理（适配器也只在存在时才发该参数）；
//     与权重同目录 ⇒ `/models/<基名>`，否则 ⇒ `/templates/<基名>`（沿用既有的输入通道落点）。
//
// 两条 fail-closed（都只针对「说不清谁赢」的形态）：
//   - **同一空间目录下基名撞车**（如权重与投影同名，或两个模板同名）⇒ 拒孵：逐文件挂载时后挂的
//     会盖住先挂的，落点相同就等于「有一份静默不见了」；
//   - 宿主路径里带 `:` ⇒ 拒孵（见 roBind：`:` 是 <host>:<space> 的分隔符，会被拆错落点）。
func spaceInputMappings(eggID string, entry *registry.ModelEntry, hostWeightFile, eggModelsDir string) ([]pathRewrite, []string, error) {
	weightDir := filepath.Dir(hostWeightFile)
	weightSpace := spaceWeightsDir + "/" + filepath.Base(hostWeightFile)

	// mkRewrite 一条输入的「改写 ＋ 挂载源」：
	//
	//	eggModelsDir == ""（宿主孵化）⇒ 逐字沿用：源 = host（非蛋模式一行分支都不走 ⇒ 零回归）；
	//	eggModelsDir != ""（蛋孵化）  ⇒ 源 = `<egg>/models/<基名>`；没有就**具名拒孵**，
	//	✗ 不静默回落宿主 —— 回落等于「原目录可删」是假象，而且失败会推到引擎装载期（§6.9）。
	mkRewrite := func(host, raw, space, what string) (pathRewrite, error) {
		rw := pathRewrite{host: host, raw: raw, space: space}
		if eggModelsDir == "" {
			return rw, nil
		}
		src, err := eggWeightSource(eggID, eggModelsDir, host, what)
		if err != nil {
			return pathRewrite{}, err
		}
		rw.src = src
		return rw, nil
	}
	weightRW, err := mkRewrite(hostWeightFile, strings.TrimSpace(entry.File), weightSpace, "权重")
	if err != nil {
		return nil, nil, err
	}
	rewrites := []pathRewrite{weightRW}
	// 空间内落点 → 声明来源（判「基名撞车」；同一个空间路径只能有一份来源）
	claimed := map[string]string{weightSpace: "权重 " + entry.File}
	weightBind, err := roBind(weightRW.bindSrc(), weightSpace)
	if err != nil {
		return nil, nil, fmt.Errorf("卵 %s：%w", eggID, err)
	}
	binds := []string{weightBind}

	// 权重分片组展开（llama.cpp `gguf-split` 产物）——2026-10-16 edge-node 实测定论。
	//
	// 为什么必须**逐片**挂：引擎**自己算全部片**，不靠 argv 列出来 —— upstream
	// src/llama-model-loader.cpp:609-612 `splits = llama_get_list_splits(fname, idx, n_split)`，
	// 命名式 `SPLIT_PATH_FORMAT = "%s-%05d-of-%05d.gguf"`、`llama_split_prefix` 把尾部
	// `-<序号>-of-<总片数>.gguf` 当**纯字符串截掉**（含目录）⇒ 派生路径恒为
	// `<第 1 片目录>/<去序号段基名>-0000N-of-0000M.gguf`。只挂第 1 片时引擎装载期逐字退
	// `E gguf_init_from_file: failed to open GGUF file '/models/…-00002-of-00004.gguf' (No such file or directory)`。
	// ⇒ 每片只要以**原名**（含 `-NNNNN-of-MMMMM.gguf`）落到同一个 /models 下，引擎逐片都找得到；
	// `-m` 仍旧指第 1 片（entry.file）—— 缺片则拒孵并**点名缺哪片**：比「孵起来、引擎装载期秒退」
	// 更早更清楚（§6.9 静默失效不得当凭据）。文件名不带分片号（单片 gguf）⇒ 零动作，
	// rewrites/binds/WeightFiles 与改前**逐字相同**。
	if base := filepath.Base(hostWeightFile); weightShardRe.MatchString(base) {
		m := weightShardRe.FindStringSubmatch(base)
		total, cerr := strconv.Atoi(m[2])
		if cerr != nil || total <= 0 {
			return nil, nil, fmt.Errorf("卵 %s：权重文件 %q 的分片总片数读不出（%q）——拒孵（组键定不下来，不猜）",
				eggID, base, m[2])
		}
		prefix := base[:len(base)-len(m[0])]
		shardDir := filepath.Dir(hostWeightFile)
		shardHost := func(n int) string {
			return filepath.Join(shardDir, fmt.Sprintf("%s-%05d-of-%05d.gguf", prefix, n, total))
		}
		// 缺片判据按**挂载源那一侧**算：蛋模式看蛋内 `<egg>/models/`（宿主同目录这时可能已被改名，
		// 那正是本单要成立的形态），宿主模式看声明目录（逐字不变）。
		shardSrcPath := func(n int) string {
			if eggModelsDir == "" {
				return shardHost(n)
			}
			return filepath.Join(eggModelsDir, filepath.Base(shardHost(n)))
		}
		spaceOfShard := func(n int) string {
			return spaceWeightsDir + "/" + filepath.Base(shardHost(n))
		}
		var missing []string
		for n := 1; n <= total; n++ {
			if _, serr := os.Stat(shardSrcPath(n)); serr != nil {
				missing = append(missing, shardSrcPath(n))
			}
		}
		if len(missing) > 0 {
			if eggModelsDir != "" {
				return nil, nil, fmt.Errorf("卵 %s：权重文件 %q 是同组 %d 片（组键 %q ＋ `-of-%05d.gguf`）中的一片，但**蛋内** %s 下缺 %d 片：%s —— "+
					"拒孵：从蛋孵化时 %s 下的一切只从蛋内取（✗ 不回落宿主同目录）；引擎装载期会按命名式**自算全部片**"+
					"（llama.cpp SPLIT_PATH_FORMAT），缺一片就 `failed to open GGUF file` ⇒ 打包时要把整组按原基名收进「源目录/%s/」",
					eggID, base, total, prefix, total, eggModelsDir, len(missing), strings.Join(missing, "、"), spaceWeightsDir, eggModelsSubdir)
			}
			return nil, nil, fmt.Errorf("卵 %s：权重文件 %q 是同组 %d 片（组键 %q ＋ `-of-%05d.gguf`）中的一片，但同目录缺 %d 片：%s —— "+
				"拒孵：引擎装载期会按命名式**自算全部片**（llama.cpp SPLIT_PATH_FORMAT），缺一片就 `failed to open GGUF file`；"+
				"孵化器只照单挂文件，绝不替卵跳过或缺省缺失的片（§6.9）",
				eggID, base, total, prefix, total, len(missing), strings.Join(missing, "、"))
		}
		for n := 1; n <= total; n++ {
			host := shardHost(n)
			if filepath.Clean(host) == filepath.Clean(hostWeightFile) {
				continue // 第 1 片就是 entry.file 那一格（已绑、已声明）⇒ 不重复落绑定
			}
			space := spaceOfShard(n)
			if prev, dup := claimed[space]; dup {
				return nil, nil, fmt.Errorf("卵 %s：权重分片 %q 与 %s 的空间内落点都是 %q——拒孵：逐文件挂载时后挂的会盖住先挂的，"+
					"落点撞车等于有一份静默不见了", eggID, host, prev, space)
			}
			claimed[space] = "权重分片 " + host
			rw, rerr := mkRewrite(host, "", space, "权重分片")
			if rerr != nil {
				return nil, nil, rerr
			}
			b, berr := roBind(rw.bindSrc(), space)
			if berr != nil {
				return nil, nil, fmt.Errorf("卵 %s：%w", eggID, berr)
			}
			binds = append(binds, b)
			rewrites = append(rewrites, rw)
		}
	}

	if p := strings.TrimSpace(entry.MMProj); p != "" {
		host := expandTilde(p)
		space := spaceWeightsDir + "/" + filepath.Base(host)
		if prev, dup := claimed[space]; dup {
			return nil, nil, fmt.Errorf("卵 %s：视觉投影 %q 与 %s 的空间内落点都是 %q——拒孵：逐文件挂载时后挂的会盖住先挂的，"+
				"落点撞车等于有一份静默不见了", eggID, entry.MMProj, prev, space)
		}
		claimed[space] = "视觉投影 " + entry.MMProj
		rw, err := mkRewrite(host, p, space, "视觉投影")
		if err != nil {
			return nil, nil, err
		}
		b, err := roBind(rw.bindSrc(), space)
		if err != nil {
			return nil, nil, fmt.Errorf("卵 %s：%w", eggID, err)
		}
		binds = append(binds, b)
		rewrites = append(rewrites, rw)
	}

	if p := strings.TrimSpace(entry.ChatTemplate); p != "" {
		host := expandTilde(p)
		// 蛋模式：模板也是 `/models` 侧输入 ⇒ 同样只从蛋内取（缺则具名拒，见 mkRewrite）。
		// 宿主模式：沿用「宿主上不存在就静默跳过」（适配器也只在存在时才发该参数）—— 逐字不变。
		if eggModelsDir != "" {
			if _, err := os.Stat(filepath.Join(eggModelsDir, filepath.Base(host))); err != nil {
				return nil, nil, fmt.Errorf("卵 %s：聊天模板 %q 的蛋内副本 %s 读不到（%v）——拒孵：从蛋孵化时 %s 下的一切"+
					"只从蛋内取（✗ 不静默跳过、✗ 不回落宿主）", eggID, entry.ChatTemplate,
					filepath.Join(eggModelsDir, filepath.Base(host)), err, spaceWeightsDir)
			}
		}
		if _, err := os.Stat(host); err == nil || eggModelsDir != "" {
			space := spaceWeightsDir + "/" + filepath.Base(host)
			if filepath.Dir(host) != weightDir {
				space = spaceTemplatesDir + "/" + filepath.Base(host)
			}
			if prev, dup := claimed[space]; dup {
				return nil, nil, fmt.Errorf("卵 %s：聊天模板 %q 与 %s 的空间内落点都是 %q——拒孵：逐文件挂载时后挂的会盖住先挂的，"+
					"落点撞车等于有一份静默不见了", eggID, entry.ChatTemplate, prev, space)
			}
			claimed[space] = "聊天模板 " + entry.ChatTemplate
			rw, err := mkRewrite(host, p, space, "聊天模板")
			if err != nil {
				return nil, nil, err
			}
			b, err := roBind(rw.bindSrc(), space)
			if err != nil {
				return nil, nil, fmt.Errorf("卵 %s：%w", eggID, err)
			}
			binds = append(binds, b)
			rewrites = append(rewrites, rw)
		}
	}
	// 视觉编码器 vision（可选，ds4 家族）——与上面的 mmproj **同款**：逐文件只读绑进 /models/<基名>，
	// 并把引擎参数里那个宿主路径改写成空间内路径。
	// 不绑的后果（2026-10-13 edge-node 真机实测·逐字）：`--vision` 带着**宿主路径**原样进沙箱，而空间内
	// 既没有 /data 也没绑该文件 ⇒ 引擎退：
	//	ds4: cannot open model '/data/models/DeepSeek-V4-Flash-Vision-Encoder.gguf': No such file or directory
	if p := strings.TrimSpace(visionPath(entry)); p != "" {
		host := expandTilde(p)
		space := spaceWeightsDir + "/" + filepath.Base(host)
		if prev, dup := claimed[space]; dup {
			return nil, nil, fmt.Errorf("卵 %s：视觉编码器 %q 与 %s 的空间内落点都是 %q——拒孵：逐文件挂载时后挂的会盖住先挂的，"+
				"落点撞车等于有一份静默不见了", eggID, p, prev, space)
		}
		claimed[space] = "视觉编码器 " + p
		rw, err := mkRewrite(host, p, space, "视觉编码器")
		if err != nil {
			return nil, nil, err
		}
		b, err := roBind(rw.bindSrc(), space)
		if err != nil {
			return nil, nil, fmt.Errorf("卵 %s：%w", eggID, err)
		}
		binds = append(binds, b)
		rewrites = append(rewrites, rw)
	}
	return rewrites, binds, nil
}

// eggWeightSource 蛋孵化时某个「/models 输入」的**挂载源** = 它在蛋内的副本
// `<eggModelsDir>/<基名>`（GAP-20261018-16）。fail-closed：蛋里没有 ⇒ 具名拒孵。
//
// 判据顺序（写死，与单测两态一一对应）：
//
//	① 蛋内有这一件（普通文件）⇒ **用蛋内**（返回蛋内绝对路径）；
//	② 蛋内没有 ⇒ **具名拒孵**，✗ 不静默回落宿主路径。
//
// 为什么②不许回落（这是本单的核心取舍）：卵声明里的 `file` 是**宿主路径**，而「从蛋孵化」的
// 全部意义就是让那个宿主路径**可以被删/改名**。回落会让孵化**看起来成功**、实际挂的是即将消失的
// 宿主件 —— 于是「原目录可删」变成假象，且失败点被推到运行期（引擎装载期 `failed to open GGUF file`，
// 或更晚的 mmap 阶段），正是 §6.9 明令不许的形态。宁可 spec 期 0 秒拒、并点名缺的那一件。
func eggWeightSource(eggID, eggModelsDir, hostFile, what string) (string, error) {
	base := filepath.Base(filepath.Clean(hostFile))
	if base == "" || base == "." || base == ".." || base == string(filepath.Separator) {
		return "", fmt.Errorf("卵 %s：%s %q 推不出基名——拒孵（蛋内落点 %s/<基名> 定不下来）",
			eggID, what, hostFile, eggModelsSubdir)
	}
	src := filepath.Join(eggModelsDir, base)
	st, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("卵 %s：%s %q 的蛋内副本 %s 读不到（%v）——拒孵：从蛋孵化时 %s 下的一切只从蛋内取"+
			"（打包时把该件按原基名收进「源目录/%s/」）；✗ 不静默回落到宿主路径 —— 回落等于「原目录可删」不成立，"+
			"且失败会推到引擎装载期", eggID, what, hostFile, src, err, spaceWeightsDir, eggModelsSubdir)
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("卵 %s：蛋内副本 %s 不是普通文件（mode=%v）——拒孵：mmap 权重要的是普通文件"+
			"（蛋内按偏移直读会被 llama.cpp 的 GGUF 魔数检查否掉）", eggID, src, st.Mode())
	}
	return src, nil
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
// （`readelf -d` → `RUNPATH [/home/<user>/llama.cpp-src/build-hip-flash/bin:]`）⇒ 空间内不设
// `LD_LIBRARY_PATH` 就是 `error while loading shared libraries: libllama-server-impl.so`
// + 单元 127（秒死）。所以孵化器**据 lib_paths 生成**它，不再指望每个引擎都手工声明。
//
// 生成口径（三条，都有意为之）：
//
//	① **只在「有可垫的库落点」时生成**：宿主孵化 = 声明了 lib_paths 才有（没声明就一个变量都不发明 ——
//	   不猜）；蛋孵化 = 蛋里的引擎侧目录（`/engine/<目录基名>` 与 `/engine/lib`）本身就是必垫项
//	   ⇒ 即使没声明 lib_paths 也生成（否则蛋里的 RUNPATH 是死的，见 ②）；
//	② **卵自己显式写了 LD_LIBRARY_PATH 就以它为准**（连空串「清空」语义也照旧），生成逻辑
//	   完全不插手 ⇒ 不会出现「声明与生成两头都写」的形态（谁赢说不清的那种）；
//	③ 生成值 = **声明的库落点在前、`/engine` 在最后**：
//	   - 声明的在前 ⇒ 卵点名的库版本优先命中（lib_paths 存在的理由就是「库版本按卵给，不许全机一套」）；
//	   - `/engine` 垫尾 ⇒ 引擎自己目录里的 impl/依赖库（宿主上正是 RUNPATH 指的那个目录）仍然找得到，
//	     新引擎不必再手工声明一次（这正是缺陷 6 的诉求：RUNPATH 写死宿主路径，空间内得有人补上）。
func hatchEnv(entry *registry.ModelEntry, cardIdx int, cardKnown bool, libRWs []pathRewrite, libSpaces []string, engineTail []string) map[string]string {
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
	// ★ 蛋孵化时 `engineTail` 非空 ⇒ **即使一个库目录都没声明也要生成**：蛋里的引擎在
	//   `/engine/<目录基名>`（不在 `/engine` 根上），没有这条变量它的 RUNPATH（写死宿主构建目录）
	//   在空间里是死的 ⇒ 127。宿主孵化时 `engineTail` 恒空 ⇒ 本条件与旧行为逐字相同。
	if (len(libSpaces) > 0 || len(engineTail) > 0) && !declaredLibPath {
		if env == nil {
			env = map[string]string{}
		}
		// ③ 生成值 = 声明的库落点在前、引擎侧目录垫尾。缺省垫 `/engine`；蛋孵化时由映射侧给
		//    `/engine/<目录基名>`（与 `/engine/lib`）—— 蛋里的引擎不在 `/engine` 根上，垫错位置
		//    就是「RUNPATH 指向宿主、LD_LIBRARY_PATH 又指错」⇒ 127。
		tail := engineTail
		if len(tail) == 0 {
			tail = []string{spaceEngineDir}
		}
		search := make([]string, 0, len(libSpaces)+len(tail))
		search = append(search, libSpaces...)
		search = append(search, tail...)
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

// ── 蛋展开（卵沙箱第 6 步）────────────────────────────────────────────────────
//
// 声明面（`entry.Custom[...]`，读法与 `vision` / `ssd` 同源）：
//
//	egg      —— 蛋件（`*.egg`）的宿主绝对路径。★ 给了它 ⇒ **本文件负责展开**（幂等）；
//	egg_dir  —— **可选**的展开落点覆盖（宿主绝对路径）；缺省 `$ZERG_EGG_ROOT/<卵名>`，
//	            而 `$ZERG_EGG_ROOT` 缺省 `~/.zerg/eggs`。
//
// 为什么这样分工：`egg_dir` 从前是**手写 yaml 键**（部署事实，写错了只在孵化时才炸）；
// 现在「手写的是**蛋**」，目录由蛋推出 ⇒ 部署面少一个可漂的自由度，而「挂的目录」与
// 「展开的目录」在构造上就是同一个（同一次调用产出）。
//
// ★ 幂等：`eggpkg.ExpandEgg` 见到同一代的 `.complete` 就**一个字节都不写**并返回
// `Skipped=true` —— 每次装载都会走这条（`hatchSpecFor` 每次都调），所以它必须便宜：
// 实际只读 8 KiB 头 ＋ 段表 ＋ 一枚凭据，✗ 不重算整件 sha。

// EnvEggRoot —— 蛋展开根的环境变量名（缺省 `~/.zerg/eggs`）。
const EnvEggRoot = "ZERG_EGG_ROOT"

// eggRootDir 蛋展开根的**唯一取法**（取不到 ⇒ 报错，✗ 静默回落到某个相对目录）。
func eggRootDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv(EnvEggRoot)); v != "" {
		return expandTilde(v), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("蛋展开根算不出来（拿不到 HOME 且 %s 未配置）—— 拒孵", EnvEggRoot)
	}
	return filepath.Join(home, ".zerg", "eggs"), nil
}

// ensureEggExpanded 把声明里的 `egg` 展开到落点（幂等），返回这次展开的读数。
//
// fail-closed：蛋件读不到 / 落点算不出 / 落点已存在且有内容 / 落点属于**另一代** / 段表
// 任何一条穿越 —— 一律**具名**报错（✗ 不退回宿主挂载：静默换源是这类改动最坏的失败形态）。
func ensureEggExpanded(eggID string, entry *registry.ModelEntry) (*eggpkg.ExpandResult, error) {
	raw := strings.TrimSpace(eggCustomString(entry, eggKeyEggFile))
	if raw == "" {
		return nil, fmt.Errorf("卵 %s：蛋件路径为空 —— 拒孵（调到这里就说明上面判过它非空）", eggID)
	}
	eggFile := filepath.Clean(expandTilde(raw))
	if !filepath.IsAbs(eggFile) {
		return nil, fmt.Errorf("卵 %s：蛋件路径 %q 不是宿主绝对路径——拒孵（相对路径会随 cwd 漂，蛋会“时有时无”）", eggID, raw)
	}
	if st, err := os.Stat(eggFile); err != nil {
		return nil, fmt.Errorf("卵 %s：蛋件 %s 读不到（%v）——拒孵（声明了蛋却拿不到，不退回宿主挂载）", eggID, eggFile, err)
	} else if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("卵 %s：蛋件 %s 不是普通文件——拒孵", eggID, eggFile)
	}

	dst := strings.TrimSpace(eggCustomString(entry, eggKeyEggDir))
	if dst == "" {
		rootDir, err := eggRootDir()
		if err != nil {
			return nil, fmt.Errorf("卵 %s：%w", eggID, err)
		}
		dst = filepath.Join(rootDir, safeEggDirName(eggID))
	} else {
		dst = expandTilde(dst)
		if !filepath.IsAbs(dst) {
			return nil, fmt.Errorf("卵 %s：蛋展开落点 %q 不是宿主绝对路径——拒孵（蛋展开目录必须能直接 stat）", eggID, dst)
		}
	}
	res, err := eggpkg.ExpandEgg(eggFile, dst)
	if err != nil {
		return nil, fmt.Errorf("卵 %s：蛋展开失败（%s → %s）——拒孵：%w", eggID, eggFile, dst, err)
	}
	return res, nil
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

// buildEngineArgv 构建引擎启动命令：适配器参数 + 卵清单 `cmd:` **合并**（Q-218 · 2026-09-26 拍板：
// 合并而非覆盖 · 卵清单显式项优先 · 适配器补缺）→ {file}/{port} 占位符替换 → 空闲自退透传（P6/R6）。
// 返回 (可执行路径, 参数)。
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
	replaceArgvPlaceholders(cmdArgs, entry.File, port)

	// 有自定义 cmd（字符串，空格分隔）⇒ 与适配器参数**合并**（Q-218 · 2026-09-26 拍板「合并而非覆盖」）。
	// 改前这里是 `cmdArgs = strings.Fields(entry.Cmd)`（**整段覆盖**）⇒ 适配器整套族级参数对写了
	// `cmd:` 的卵从未生效 ✗，而读代码的人以为生效 ✗（同一模型的启动参数两个真源、谁说了算取决于
	// 「有没有写 cmd:」）。改后的键规则与优先级见 mergeEngineArgs。
	if entry.Cmd != "" {
		eggArgs := strings.Fields(string(entry.Cmd))
		if len(eggArgs) == 0 {
			// 空/空白 cmd: = 没声明（与 engineHostPath 的 TrimSpace 口径一致）⇒ 走适配器参数。
			// （旧行为：产出**空 argv**（起不来的卵）✗ —— 那不是任何一处声明的意图。）
		} else {
			replaceArgvPlaceholders(eggArgs, entry.File, port)
			// 首词 = 可执行文件/wrapper（非主线引擎的 run-k2.sh 就在这里，**只能由卵清单定** ✗）
			cmdPath = eggArgs[0]
			cmdArgs = append([]string{cmdPath}, mergeEngineArgs(cmdArgs, eggArgs[1:])...)
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

// engineArgvOf 拼出「可执行 + 参数」的完整 argv（观测面 /eggs 的 argv 真源 · GAP-20260926-01）。
// 逐字保留 buildEngineArgv 的产出：可执行在前、参数在后，一项不改（机读要与真命令行逐字相符）。
func engineArgvOf(cmdPath string, execArgs []string) []string {
	out := make([]string, 0, len(execArgs)+1)
	if cmdPath != "" {
		out = append(out, cmdPath)
	}
	return append(out, execArgs...)
}

// replaceArgvPlaceholders 就地替换参数里的 {file}/{port} 占位符（适配器与卵清单**两侧同一口径**）。
func replaceArgvPlaceholders(args []string, file string, port int) {
	for i, arg := range args {
		cur := strings.ReplaceAll(arg, "{port}", fmt.Sprintf("%d", port))
		args[i] = strings.ReplaceAll(cur, "{file}", file)
	}
}

// ═══ Q-218（2026-09-26 Mr2109 拍板）：启动参数「合并而非覆盖」═══════════════════
//
// **改前**：卵清单有 `cmd:` 就整段覆盖适配器参数 ⇒ 同一模型的启动参数有**两个真源**
// （适配器 `BuildArgs` vs 卵清单 `cmd:`），谁说了算只取决于「有没有写 cmd:」✗；实测两机在册的卵
// **全都**写了 `cmd:` ⇒ 适配器那一整套族级参数（`-ctk/-ctv/-fa/--cache-prompt/-cb/-np 1/…`）
// 对它们**从未生效**，而读代码的人以为生效 ✗（Q-218 的登记原文：读代码必误判）。
//
// **改后**：两侧**合并** —— 卵清单显式项优先、适配器补缺。键（旗标）规则如下：
//   - 键归一：`--foo=bar` 取 `--foo`；别名按 argKeyAlias 归一（只登记确知同义的）；
//   - 同名键 ⇒ **卵清单优先**，就地替换（保两侧各自的位次）⇒ 同名参数只留一个 ✓；
//   - 卵清单没发的键 ⇒ 适配器补缺，追加在末尾 ✓（两侧异名参数并存）；
//   - 可执行文件（`cmd:` 首词）**只能由卵清单定** ✗ —— 非主线引擎的 wrapper（`run-k2.sh` /
//     `run-bonsai.sh`）就在这里，适配器不许改写它（P1 守卫的判据是 `entry.Cmd != ""`，同源）。
//
// **为什么是「卵清单优先」而不是适配器优先**（三条，按分量排）：
//  ① 卵清单是「每机每模型」的**现场真源** —— 机器特有路径、内存上界（`-c 65536/131072`）、
//     非主线引擎 wrapper、实测处置（`--reasoning-budget 256`）都写在它里面；适配器是**族级默认**
//     （跨机同族共用一份），定位天然是补缺、不是覆盖；
//  ② 它**就是当前生效的那一侧**（现读两机运行态命令行与卵清单逐字一致 ✓）⇒ 只有让它优先，
//     拍板理由「运行参数一律不动（4 槽与 08-15 认知一致）」才成立：`-c`/`-ngl` 全不变；
//  ③ 适配器优先会把单机显式声明打掉（4 槽→1 槽那类），而那种「反向修法」已在 §五十.8 被判过 ✗。
//
// **同批**：其余适配器（`gemma/generic/qwen36/qwen38flash/k2horizon`）里「v2.5.5 单槽铁律」的
// `-np 1` 与本批**不动**（默认值不动 ✗）。但它原先带一条**后果**，现已由下面的例外解决（2026-09-26）：
//
//	合并会把这五处的 `-np 1` **首次**注入到「写了 `cmd:`、但没写 `-np`」的卵
//	（node-local 现读 7 枚全部属于这一类）⇒ 下次装载槽数从 `-np` 缺省（`-1 = auto`，实测 4 槽）变成 1 槽 ✗。
//	⇒ 解法 = `mergeEngineArgs` 的 **`-np` 例外**（见该函数注释）：卵清单**没显式写** `-np` 就摘掉
//	适配器那一份 ⇒ 槽数维持引擎缺省 ✓；而 `cmd:` 为空的卵（node-a 现读 4 枚：`GLM-4.7-Flash` /
//	`example-8b-quant` / `Qwen3.6-35B-A3B` / `qwen2.5-3b`）适配器仍是**唯一真源** ⇒ `-np 1` 照旧 ✓。
//	判据 = `hasExplicitNP(eggArgs)`，只作用于 `-np` 一个键 ✗ 其余参数的补缺规则不变。

// argKeyAlias 同一旗标的两种写法归一（免得合并后同义参数出现两次、负控「同名只留一个」失效）。
// 只登记**确知同义**的：`-mm` / `--mmproj` 是 llama-server 的同一个 flag（`-mm, --mmproj FILE`）。
// 不在这里「顺手」登记存疑的同义关系 —— 认错键会把两侧本来不同的参数误并成一个 ✗。
var argKeyAlias = map[string]string{"--mmproj": "-mm", "--parallel": "-np"} // --parallel：依据 llama-server --help 逐字「-np,   --parallel N」(env LLAMA_ARG_N_PARALLEL) ⇒ 确知同义（2026-09-29 现读）

// argKey 取参数的键：`--foo=bar` → `--foo`；别名归一；其余原样。
func argKey(tok string) string {
	key := tok
	if i := strings.Index(key, "="); i > 0 && strings.HasPrefix(key, "-") {
		key = key[:i]
	}
	if alias, ok := argKeyAlias[key]; ok {
		return alias
	}
	return key
}

// argvItem 参数序列里的一项：键 + 该项的原文 token（旗标，及其 0..n 个值）。
type argvItem struct {
	key    string
	tokens []string
}

// splitArgvItems 把参数切成「旗标 + 它的值」项序列。
// 值 = 紧跟旗标之后、且不以 `-` 开头的连续 token（`-c 0` / `-fa on` / `--chat-template-kwargs {…}`）。
// 首 token 不是旗标时（可执行文件）单列一项，键 = 它自己（只占位次、不参与同名判定）。
func splitArgvItems(args []string) []argvItem {
	items := make([]argvItem, 0, len(args))
	for i := 0; i < len(args); {
		tok := args[i]
		if !strings.HasPrefix(tok, "-") {
			items = append(items, argvItem{key: tok, tokens: []string{tok}})
			i++
			continue
		}
		item := argvItem{key: argKey(tok), tokens: []string{tok}}
		j := i + 1
		for j < len(args) && !strings.HasPrefix(args[j], "-") {
			item.tokens = append(item.tokens, args[j])
			j++
		}
		items = append(items, item)
		i = j
	}
	return items
}

// hasExplicitNP 卵清单是否**显式**声明了槽数（`-np 4` / `-np=4` / `--parallel 4` —— 长写法经 argKeyAlias 归一）。
func hasExplicitNP(args []string) bool {
	for _, a := range args {
		if argKey(a) == "-np" {
			return true
		}
	}
	return false
}

// dropFlagItems 摘掉某键及其值（`-np` 例外专用）。
func dropFlagItems(args []string, key string) []string {
	out := make([]string, 0, len(args))
	for _, it := range splitArgvItems(args) {
		if it.key == key {
			continue
		}
		out = append(out, it.tokens...)
	}
	return out
}

// mergeEngineArgs 合并两侧参数：**适配器为底、卵清单显式项优先**（同名就地替换、异名追加）。
// 入参都不含可执行文件（调用方单独拼首词）；任一侧为空 ⇒ 直接返回另一侧。
//
// **`-np` 例外**（Q-218 · 2026-09-26 父代理拍：`-np` 只认卵清单显式声明）：卵清单写了 `cmd:`
// 但**没写** `-np` ⇒ 先摘掉适配器的 `-np`，再照常合并。为什么 `-np` 不同于其他参数：其余参数是
// **能力/性能**声明（`-ctk/-fa/--cache-prompt/…` 是族级默认，适配器补缺正是补该补的），而槽数是
// **这台机这个模型**的并发容量 —— 卵清单不写 `-np` 的意思是「沿用引擎缺省（实测 4 槽）」，**不是**
// 「请适配器替我定成 1 槽」；让适配器补缺 = 静默把 4 槽压成 1 槽 ✗（= 改运行参数，与拍板理由相抵）。
// 卵清单 `cmd:` 为空时走上面的早退分支、压根不经过这里 ⇒ 适配器仍是**唯一真源**，`-np 1` 照旧生效 ✓。
func mergeEngineArgs(adapterArgs, eggArgs []string) []string {
	if len(eggArgs) == 0 {
		return adapterArgs
	}
	if len(adapterArgs) == 0 {
		return eggArgs
	}
	if !hasExplicitNP(eggArgs) {
		adapterArgs = dropFlagItems(adapterArgs, "-np")
	}
	out := splitArgvItems(adapterArgs)
	index := make(map[string]int, len(out))
	for i, it := range out {
		index[it.key] = i
	}
	for _, it := range splitArgvItems(eggArgs) {
		if at, ok := index[it.key]; ok {
			out[at] = it // 同名 ⇒ 卵清单优先（就地替换，位次留在适配器那一侧）
			continue
		}
		index[it.key] = len(out)
		out = append(out, it) // 异名 ⇒ 适配器补缺（追加在末尾，卵清单原有顺序在前）
	}
	flat := make([]string, 0, len(adapterArgs)+len(eggArgs))
	for _, it := range out {
		flat = append(flat, it.tokens...)
	}
	return flat
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
// 报告：`Zerg-内部文档/issues/报告-第一枚卵真机实测-20260915.md` §12（该报告已随开发文档分家迁出工作树，
// 原路径 `docs/issues/…`——2026-09-19）。与本文件相关的四条逐条落到本文件
// ——**只到映射边界**（hatch 包的配方不归本文件，见 ⓵）。
//
//	5. **lib_paths 落点嵌在只读的 /engine 之下 ⇒ 整卵孵不起来**（原文
//	   `bwrap: Can't mkdir parents for /engine/lib/bin: Read-only file system`，单元 status=1，
//	   1–3ms 秒死）。改：落点 `/engine/lib/<名>` → **独立只读挂载点 `/libs/<名>`**
//	   （见 spaceLibRoot 注释：父目录 /libs 是新根上的普通目录，bwrap 建得出中间目录）。
//
//	6. **引擎 RUNPATH 写死宿主路径**（`readelf -d` → `RUNPATH [/home/<user>/…/build-hip-flash/bin:]`）
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
