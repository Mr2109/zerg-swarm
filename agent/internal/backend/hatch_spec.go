// hatch_spec.go —— 批 1：卵声明 → 孵化声明（hatch.Spec）的映射。
//
// 设计真源：《设计-子端隔离化-20260914》（仓库内该文档标题含旧词，按既有先例此处与
// 包内注释一律以「隔离化」指代同一份文件）
//
//	§4.3 卵声明字段表 · §6.6 目录二分（一次性 vs 跨孵化保留）· §6.7 环境供给五类 + 三个真坑
//	§8.4 标定铁律（无实测档案不许孵）· §6.9 静默失效不得当凭据
//
// 本文件只做一件事：把「卵声明（registry.ModelEntry）+ 引擎实现名 + 端口 + 实测档案」翻译成
// hatch.Spec —— 孵化器的全部输入。三条口径：
//
//	① fail-closed：卵名 / 权重 / 引擎路径任一拿不到就报错拒孵，绝不编造默认值
//	   （孵化器只照单执行，不推断、不补默认，§6.7）；
//	② 只挂该卵自己的东西：权重按「文件所在目录 → /models」挂，参数里的宿主权重路径改写为
//	   空间内路径 ⇒ 别的模型与整个 /data 在空间内根本不存在（§6.6 / §9.3）；
//	③ 空间内路径约定写死在本文件（/models、/engine、/templates），与 hatch 包的命令行配方一一对应。
//
// 纯函数：不 exec、不写盘（只建一次性工作目录 + os.Stat 判模板文件是否存在），可在 macOS 上
// 直接单测；真正的孵化在 hatch 包（非 Linux 明确拒绝）——本包只负责「把声明翻译成孵化器的输入」。
//
// ⚠ 已知待拍板（如实标出，不擅自改 hatch 包的 X3 实测配方）：
//   - hatch.Spec.WorkDir 的注释是「**空间内**工作目录」，本批按口径映射成**宿主**一次性目录
//     ~/.zerg/work/<卵名>；两者要同时成立，需要在封闭空间里给它一个**可写落点**（现 Spec 只有
//     只读 ExtraROBinds，没有可写绑定）⇒ 本批按宿主路径映射，并把这一点写进回报问 Mr2109。
//   - 卵声明里的 env_req.weights / lib_paths 尚未接线：weights 与 entry.file 的一致性校验、
//     lib_paths 的「空间内落点」（多个库目录都挂 /engine 会互相遮挡）都要先拍板；本批只原样
//     透传 env_req.env（含显式声明的 LD_LIBRARY_PATH，空串 = 显式清空）。
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
	// spaceWeightsDir 该卵自己的权重目录（只读；只挂它 ⇒ 别的模型在空间内不存在）。
	spaceWeightsDir = "/models"
	// spaceEngineDir 引擎自己的可执行 + 库/构建目录（只读）。
	spaceEngineDir = "/engine"
	// spaceTemplatesDir 卵声明引用的模板类只读输入（ExtraROBinds 的「输入通道」：
	// 如 chat_template 落在权重目录之外时，只读挂进来并改写参数）。
	spaceTemplatesDir = "/templates"
)

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

	// ② 权重：只挂该卵自己的权重目录（文件 → 所在目录；参数里的宿主路径 → 空间内路径）
	hostWeightFile := expandTilde(entry.File)
	if hostWeightFile == "" {
		return spec, fmt.Errorf("卵 %s：权重文件未声明（file 字段）——拒孵（孵化器不替卵挑权重）", eggID)
	}
	if !filepath.IsAbs(hostWeightFile) {
		return spec, fmt.Errorf("卵 %s：权重路径 %q 不是宿主侧绝对路径——拒孵（空间内只读挂载与 %s 内的相对路径都要求绝对路径）",
			eggID, entry.File, spaceWeightsDir)
	}
	weightDir := filepath.Dir(hostWeightFile)
	spec.WeightPath = weightDir

	// ③ 引擎：宿主路径 → 宿主目录（只读挂进 /engine）+ 空间内路径（/engine/<可执行名>）
	hostEngine, err := engineHostPath(entry, engineImpl)
	if err != nil {
		return spec, fmt.Errorf("卵 %s：%w", eggID, err)
	}
	spec.EngineRoots = []string{filepath.Dir(hostEngine)}
	spec.EnginePathInSpace = spaceEngineDir + "/" + filepath.Base(hostEngine)

	// ④ 参数：与裸 exec 路径**同一份构造**（buildEngineArgv），再把宿主侧输入改写成空间内路径
	rewrites, binds, err := spaceInputMappings(eggID, entry, weightDir, hostWeightFile)
	if err != nil {
		return spec, err
	}
	_, engineArgs := buildEngineArgv(eggID, entry, port)
	spec.EngineArgs = rewriteArgsToSpace(engineArgs, rewrites)
	spec.ExtraROBinds = binds

	// ⑤ 环境与设备：卵声明照单执行；声明了卡号才决定可见的渲染节点
	cardIdx, cardKnown := declaredCardIndex(entry)
	spec.Env = hatchEnv(entry, cardIdx, cardKnown)
	spec.Devices = hatchDevices(entry, cardIdx, cardKnown)

	// ⑥ 一次性工作目录（每卵一份；不存在就建）
	workDir, err := ensureEggWorkDir(eggID)
	if err != nil {
		return spec, fmt.Errorf("卵 %s：%w", eggID, err)
	}
	spec.WorkDir = workDir

	// ⑦ mmap 限额（§6.7 C②：RLIMIT_MEMLOCK 给不足直接崩，不是变慢）——声明了才下发，不猜
	if entry.EnvReq != nil && entry.EnvReq.MemlockKB > 0 {
		spec.MemlockBytes = int64(entry.EnvReq.MemlockKB) * 1024
	}
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

// checkEngineImplConsistency 引擎实现名（卵声明口径）与「实际会执行的可执行名」必须一致：
// 声明与执行分叉时 fail-closed 拒孵，绝不静默按其中一个跑（§6.8.4 认不得就报错的精神）。
func checkEngineImplConsistency(hostEngine, engineImpl string) (string, error) {
	base := filepath.Base(hostEngine)
	if want := filepath.Base(strings.TrimSpace(engineImpl)); engineImpl != "" && want != "" && want != base {
		return "", fmt.Errorf("引擎实现名 %q 与实际会执行的可执行名 %q 不一致——拒孵（声明与执行分叉，不静默按其一跑）",
			engineImpl, base)
	}
	return hostEngine, nil
}

// ── 宿主侧输入 → 空间内路径（②）─────────────────────────────────────────────

// pathRewrite 一条「宿主侧输入 → 空间内路径」的改写（整 token 精确匹配）。
type pathRewrite struct {
	host  string // 展开后的宿主路径
	raw   string // 卵声明里的原样字符串（可能带 ~；参数里出现的可能正是它）
	space string // 空间内路径
}

// spaceInputMappings 算出需要改写的宿主侧输入，以及需要额外只读绑定的清单（ExtraROBinds）。
//
// 覆盖三类输入（都只读）：
//   - 权重文件（必给）：空间内 = /models/<basename>，其目录就是 WeightPath 的挂载源；
//   - 视觉投影 mmproj（可选）：必须与权重同目录——不同目录一律拒孵（少挂了不是「少一点」，
//     而是「装好了但看不见图」的静默降级，正是 §6.9 要防的形态）；
//   - 聊天模板 chat_template（可选）：文件存在才处理（适配器也只在存在时才发该参数）；
//     与权重同目录 ⇒ 顺带可见；否则只读挂进 /templates/<basename>（ExtraROBinds 输入通道）。
func spaceInputMappings(eggID string, entry *registry.ModelEntry, weightDir, hostWeightFile string) ([]pathRewrite, []string, error) {
	rewrites := []pathRewrite{{
		host:  hostWeightFile,
		raw:   strings.TrimSpace(entry.File),
		space: spaceWeightsDir + "/" + filepath.Base(hostWeightFile),
	}}
	var binds []string

	if p := strings.TrimSpace(entry.MMProj); p != "" {
		host := expandTilde(p)
		if filepath.Dir(host) != weightDir {
			return nil, nil, fmt.Errorf("卵 %s：视觉投影 %q 与权重不在同一目录（权重目录 %s）——拒孵：空间内只挂该卵自己的权重目录，投影挂不进去会变成「装好了但看不见图」的静默降级",
				eggID, entry.MMProj, weightDir)
		}
		rewrites = append(rewrites, pathRewrite{host: host, raw: p, space: spaceWeightsDir + "/" + filepath.Base(host)})
	}

	if p := strings.TrimSpace(entry.ChatTemplate); p != "" {
		host := expandTilde(p)
		if _, err := os.Stat(host); err == nil {
			space := spaceWeightsDir + "/" + filepath.Base(host)
			if filepath.Dir(host) != weightDir {
				space = spaceTemplatesDir + "/" + filepath.Base(host)
				binds = append(binds, host+":"+space) // ExtraROBinds 形式：<host>:<space>
			}
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
			if (r.host != "" && arg == r.host) || (r.raw != "" && arg == r.raw) {
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
//	清它，§6.7 C③）；声明了卡号再补一条 HIP_VISIBLE_DEVICES（卵已自己声明 HIP_VISIBLE_DEVICES
//	的以声明为准，不覆盖）。未声明任何环境变量 ⇒ nil（不注入空壳）。
func hatchEnv(entry *registry.ModelEntry, cardIdx int, cardKnown bool) map[string]string {
	var env map[string]string
	if entry != nil && entry.EnvReq != nil && len(entry.EnvReq.Env) > 0 {
		env = make(map[string]string, len(entry.EnvReq.Env)+1)
		for k, v := range entry.EnvReq.Env {
			env[k] = v
		}
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

// ensureEggWorkDir 建（如缺）每卵一次性工作目录，返回其宿主路径。
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
