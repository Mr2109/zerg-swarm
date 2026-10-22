package main

// 命令表 · model 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_model() []*command {
	return []*command{
		{
			path:     []string{"model", "ls"},
			kind:     "Model",
			summary:  "可用模型（投影 /api/fleet/models）",
			usage:    "zerg model ls [--json <字段>]",
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "file"},
			endpoint: "GET /api/fleet/models",
			run:      cmdModelLs,
		},
		// ---- 批 D · T-45 `model` + `core` 两族（§三 E/C 族 · §7.1 `P11` · §十二 `P-029`）----
		{
			path:     []string{"model", "show"},
			kind:     "Model",
			summary:  "看一个模型（投影 /api/fleet/models 的单条；对象是**模型 id**，不是机器名）",
			usage:    "zerg model show <模型 id> [--json <字段>]",
			arity:    "any",
			args:     []string{"模型 id"},
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "peak_mem_gb", "file", "registry_id"},
			endpoint: "GET /api/fleet/models",
			run:      cmdModelShow,
		},
		{
			path:     []string{"model", "opts"},
			danger:   &dangerSpec{dangerD2, "模型 id", "改适配器参数（`set` 实时生效并**持久化**：`PUT /api/models/{name}/adapter-opts`；`get` 只读）", "GAP-20261005-40 · 同族先例 model add / model set（D2）· §三 E 族", false, dangerScopeRemote},
			opened:   true,
			kind:     "ModelOpts",
			summary:  "适配器参数（get 只读 / set 实时生效要 --yes）",
			usage:    "zerg model opts get <模型 id> [--json <字段>] | zerg model opts set <模型 id> --set k=v… [--dry-run | --yes]",
			arity:    "any",
			args:     []string{"动作（get|set）", "模型 id"},
			fields:   []string{"model", "schema", "note", "updated", "result"},
			endpoint: "GET|PUT /api/models/{name}/adapter-opts",
			run:      cmdModelOpts,
		},
		{
			path:     []string{"model", "verify"},
			kind:     "ModelVerify",
			summary:  "四方一次对账（只读）：(1)主控下发 (2)子端本地注册表 (3)模型档案 fleet 意图 + egg-profile 事实 (4)运行期实占（本版无通路）—— 人面 5 列 + 恒打 verdict（一致/不一致/缺席）",
			usage:    "zerg model verify 模型@机 [--json 字段]",
			arity:    "any",
			args:     []string{"对象（模型@机 · 例 example-35b-v2@Mr2109）"},
			fields:   []string{"egg_id", "side", "source", "value", "authority", "verdict", "key", "state", "note"},
			endpoint: "",
			run:      cmdModelVerify,
		},
		// ---- 批 B6（2026-10-20 · `zerg model record verify|probe|ls`）：独立入口
		// `core/cmd/zerg-model` 归入命令面 ----
		// 源件 668 行 · 三个子命令（`verify` / `probe` / `list`）· 十六枚旗标**逐字**保留。
		// ★ 并入**既有** `model` 族，✗ 不新开顶层族；但动作名**不能照抄** —— 源件的 `verify` 与 `list`
		//   在 `model` 族里都已被占了、且**语义不同**：
		//     `zerg model verify 模型@机` = 四方对账（`cmdModelVerify`，只读，对象是模型@机）
		//     `zerg model ls`            = 主控 `/api/fleet/models` 的机型投影（`cmdModelLs`）
		//   本件搬的是**另一件事**（标准 §三 的记录件面 · `core/internal/modelreg`）⇒ 挂一层 `record`
		//   中间段把这一面整块收纳（3 段路径，`resolve()` 已支持）；撞名的两条**一字不动**。
		// **双轨期**：`core/cmd/zerg-model` 二进制一字不动（它自己的 `0/1/2/3/4` 是**组件自有码空间**，
		//   `exitcode-namespaces.json` 里挂着号）。实现在 `family_model_record.go`，件头逐条写清与源件的
		//   六处差异（动作名 · 退码重取号 0/1/2/12/14 · 互斥旗标明拒 · `--json` 按 K2 口径 · `--strict`
		//   的精确点名 · 位置参数归用法错 2）。
		// 只读面（verify / ls）与写面（probe：`--out` 落件 / `--store` 进模型目录）混在同一条族段里：
		//   写面**不进危险表**，理由与 `model put/pull`（D2）不同 —— 这一面写的是**本机的记录件/模型目录**，
		//   真跑前有 `modelreg.Verify` 的门禁（不合标准即拒入目录、不写占位值凑绿），且 `--out` 的落点由
		//   调用者显式给；它与 `persona` 那类「改主控运行态」的写面不是一回事 ⇒ 无 `--yes`/`--dry-run` 档，
		//   与源件同形态（`--store` 幂等：同内容不重写、冲突拒覆盖）。
		{
			path:     []string{"model", "record", "verify"},
			kind:     "ModelRecordVerify",
			summary:  "校验一条模型登记记录（标准 §三）是否符合标准（只读）：`--integrity` 额外核对 files[] 与磁盘件（流式 sha256 + 大小）· `--in-place` 核「记录位置与身份是否自洽」（阶段 1 廉价篡改检出 · 不做签名 · 不代表权重可信）",
			usage:    modelRecordVerifyUsage,
			arity:    "any",
			args:     []string{"记录件路径（`--in-place` 档可给记录目录）"},
			fields:   []string{"path", "schema", "verdict", "level", "field", "detail", "id", "format_errors", "format_warns", "integrity_status", "integrity_name", "integrity_note"},
			endpoint: "",
			opened:   true,
			run:      cmdModelRecordVerify,
		},
		{
			path:     []string{"model", "record", "probe"},
			kind:     "ModelRecordProbe",
			summary:  "探测一个模型（本地路径或端点 URL）并生成登记记录：默认只打印不写盘 · `--out <件>` 原子写（兄弟文件 `<版>.trace.json` / `<版>.capabilities.json` 另写）· `--store` 进模型目录 manifests/（同内容不重写 · 不合标准**拒入**）· 血缘 `--parent`/`--base` 只照抄显式入参、**不推断**",
			usage:    modelRecordProbeUsage,
			arity:    "any",
			args:     []string{"目标（本地模型路径或端点 URL）"},
			fields:   []string{"id", "version", "digest", "files", "capabilities", "errors", "warnings", "target", "endpoint", "engine", "out", "changed", "verdict"},
			endpoint: "",
			opened:   true,
			run:      cmdModelRecordProbe,
		},
		{
			path:     []string{"model", "record", "ls"},
			kind:     "ModelRecordLs",
			summary:  "列出模型目录（manifests/）里的登记记录（只读）：逐条给 ID / 版本 / commercial / files / 能力 / error·warn / 落点，并标出「commercial != yes 不得作默认项」（标准 §五 红线）；目录里有不合标准的记录 ⇒ 退 1",
			usage:    modelRecordLsUsage,
			arity:    "none",
			args:     []string{"（不收位置参数：模型目录根走 `--root`、manifests 目录走 `--manifests`）"},
			fields:   []string{"id", "version", "commercial", "files", "capabilities", "errors", "warns", "path", "default_eligible", "detail"},
			endpoint: "",
			opened:   true,
			run:      cmdModelRecordLs,
		},
		{
			path:     []string{"model", "put"},
			kind:     "ModelPut",
			summary:  "把模型的本地件搬到目标机的**显式落点**（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2）：传输复用 `dplSSH` ＋ rsync · 落点必须显式给 `--to <路径>`（✗ 不猜默认）· 发前本机 sha256 ⟷ 对端回读**双侧对拍**，不一致 ⇒ 判红（退 1）",
			usage:    "zerg model put <模型@机> --to <路径> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"对象（模型@机 · 例 example-35b-v2@Mr2109）", "落点（--to：目标机上的**显式**路径）"},
			fields:   []string{"egg_id", "model", "machine", "transport", "from", "to", "local_sha256", "remote_sha256", "verdict", "note"},
			danger:   &dangerSpec{dangerD2, "对象（模型@机）", "把该模型的本地件经 ssh/rsync 搬到目标机的**显式落点** `--to`（✗ 不猜默认路径）；发前本机 sha256 ⟷ 对端回读双侧对拍，不一致 ⇒ 判红；不碰主控运行态、不改名册件；可逆：删掉对端落点件即原状", "缺口账 GAP-20260925-25 · 同族写面先例 model add（D2）· §九 M3 C1", false, dangerScopeRemote},
			opened:   true,
			endpoint: "",
			run:      cmdModelPut,
		},
		{
			path:     []string{"model", "pull"},
			kind:     "ModelPull",
			summary:  "把模型件从**源机**拉到**本机的显式落点**（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2）：`@机` = **对端机（= 源机）**（方向由动词给：put 本机→对端、pull 对端→本机）· 传输复用 `dplSSH` ＋ rsync · 落点必须显式给 `--to <本地路径>`（✗ 不猜默认）· 源机件回读 sha256 ⟷ 本机落点 sha256**双侧对拍**，不一致 ⇒ 判红（退 1）· `@本机` ⇒ 拒执（退 2）",
			usage:    "zerg model pull <模型@机> --to <本地路径> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"对象（模型@机 · @机 = **源机** · 例 example-35b-v2@ms02）", "落点（--to：**本机**上的**显式**路径）"},
			fields:   []string{"egg_id", "model", "machine", "transport", "from", "to", "local_sha256", "remote_sha256", "verdict", "note"},
			danger:   &dangerSpec{dangerD2, "对象（模型@机）", "把该模型件从**源机**（`@机`）经 ssh/rsync 拉到**本机的显式落点** `--to`（✗ 不猜默认路径）；源机件回读 sha256 ⟷ 本机落点 sha256 双侧对拍，不一致 ⇒ 判红；不碰主控运行态、不改名册件；可逆：删掉本机落点件即原状", "缺口账 GAP-20261010-14 · 同族写面先例 model put（D2）· 设计稿 §2.4 甲案", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdModelPull,
		},
		{
			path:     []string{"model", "stop"},
			kind:     "ModelStop",
			summary:  "停模型（D3 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<模型 id>`/`--yes` ⇒ 2 · 幂等优先：已停不报 500）",
			usage:    "zerg model stop <模型 id> [--dry-run | --confirm=<模型 id> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型 id"},
			fields:   []string{"name", "host", "status", "changed"},
			danger:   &dangerSpec{dangerD3, "模型 id", "停掉该模型的驻留（**在跑任务受影响**）· 幂等优先：已停不报 500", "§三 E 族 · §九 M4 · 开工单 T-45 · 同族写面先例 `agent unload`（D3）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/models/{name}/stop",
			run:      cmdModelStopGuarded,
		},
		{
			path:     []string{"model", "start"},
			kind:     "ModelStart",
			summary:  "起模型（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2）",
			usage:    "zerg model start <模型 id> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型 id"},
			fields:   []string{"name", "host", "status", "changed"},
			danger:   &dangerSpec{dangerD2, "模型 id", "把模型装载起来（占槽位 · 单槽机要排队）", "§三 E 族 · §九 M5 · 开工单 T-45 · 同族写面先例 `agent load`（D2）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/models/{name}/start",
			run:      cmdModelStartGuarded,
		},
		// ---- 波① · T1a `Q-099`/`Q-100`：加模型 + 热加载（**纯 CLI 活 · 主控零改动** · 2026-09-23）----
		// 为什么是它们（§自排补遗 `S1` 逐字）：`Q-099` 是 P0、`Q-100` 是 P1，调研结论「主控零改动」——
		// 热加载路由**已经在跑的主控上**（`core/cmd/zerg-core/main.go`：`r.Post("/api/config/reload", …)`），
		// 且处理器**自己先解析**（解析不过即回 `CONFIG_LOAD_FAILED`）⇒ 命令面这边只补「先校验后写 /
		// 先校验再请求」两道。今天这两件事只能手改 YAML（手搓插坏名册两次）或重启主控 ✗。
		// ★ `danger` 登记（**D2** · 2026-09-27）：本表项**真写盘**，修前无 `danger` ⇒ `zerg help` 把它当安全档 ✗。
		//   定档理由（逐条读 `family_config.go:cmdModelAdd` 得）：唯一的写盘点是 `writeFleetAtomic`
		//   （`family_config.go:513` 临时件 `os.WriteFile` + `:518` `os.Rename` 原子换入），只给
		//   `gateway/fleet.yaml` 这一件的 `models:` 段**加一条**；不删件、不改别人的条、不碰主控运行态
		//   ⇒ 可逆（删掉那一行即回原状）⇒ **D2**（`--yes` 即可），不是 D3。
		//   ★ 三态面的两枚真判**都已接**（本单只补登记，不动实现件）：`--dry-run`（`family_config.go:460`）
		//   ⇒ 计划件走 stdout + rc=0 且一个字节都不落；缺 `--yes`（`:489`）⇒ 计划件走 stderr + rc=2
		//   （fail-closed）—— 两条都判在 `writeFleetAtomic`（`:495`）**之前**。
		//   ★ 同族「`--dry-run` 与 `--yes` 同给 ⇒ 2」那条判本表项**未接**（全仓只有 `family_gate_matrix.go:99`
		//   一处）⇒ 本单**禁改既有函数行为** ⇒ 不擅自补（入缺口账，交父代理拍板）。
		//   ★ `opened: true` = 真跑已开放（`--yes` 就写；`--dry-run` 零副作用）—— 漏它会让三处显示面
		//   齐声报「本版未开放」（`cli_danger_opened_test.go` 的对账闸正是钉这个）。
		{
			path:     []string{"model", "add"},
			kind:     "ModelAdd",
			summary:  "往名册件（`gateway/fleet.yaml`）**先校验后写**加一条模型（`--dry-run` 先行 · 真写要 `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚 · 不覆盖别人的条）——**块按 `--model`（模型名）定位/新建**（列表形与裸映射形都认）· `--host` 只作该条的 `host:` 字段值",
			usage:    "zerg model add --model <模型名> --host <主机> --file <GGUF 路径> --confirm <模型名> [--backend …] [--mem-gb …] [--size-bytes …] [--ctx …] [--arch …] [--desc …] [--mmproj …] [--added 日期] [--verified] [--cmd <JSON 数组>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型名（--model：`models:` 段的键 / 块名 ⇒ 按它定位或新建那一块）", "主机（--host：该条 `host:` 的字段值 ＝ 这台模型跑在哪台机器）", "GGUF 路径（--file）"},
			fields:   []string{"model", "host", "file", "fleet", "line", "added"},
			danger:   &dangerSpec{dangerD2, "模型名（`--model`）", "往名册件 `gateway/fleet.yaml` 的 `models:` 段加一条（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；不删件、不改别人的条、不碰主控运行态）；可逆：删掉那一行即回原状", "缺口账 `GAP-20260927-342` · 同族写面先例 `zerg gap export`（真写盘）· `zerg gate matrix` · §九 M3 C1", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdModelAdd,
		},

		// ☆ 波3-A 命令面真修（缺口 `GAP-20260925-31`）：名册「删一条 / 改一条」两动作面。
		//   定档理由（逐条读 `family_config.go:cmdModelRm` 得）：唯一写盘点是 `writeFleetAtomic`
		//   （同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚），只动 `models:` 段里
		//   点准的那一条 ⇒ 可逆（把删掉那行加回即原状）⇒ **D2**（`--yes` 即可），不是 D3。
		//   三态面：`--dry-run` ⇒ 计划件走 stdout + rc=0 且一个字节都不落；缺 `--yes` ⇒ 计划件走 stderr
		//   + rc=2（fail-closed）；`--dry-run` 与 `--yes` 同给 ⇒ 2（`guard.go:dryRunYesConflict`）。
		{
			path:     []string{"model", "rm"},
			kind:     "ModelRm",
			summary:  "从名册件（`gateway/fleet.yaml`）**先校验后写**删一条模型（`--dry-run` 先行 · 真写要 `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚）——按 `--model`（块名）+ `--host`（该条 `host:`）点准那一条；删空则连块首行一起删",
			usage:    "zerg model rm --model <模型名> --host <主机> [--file <GGUF 路径>] --confirm <模型名> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型名（--model：`models:` 段的键/块名）", "主机（--host：要删那条的 `host:`）"},
			fields:   []string{"model", "host", "file", "fleet", "line", "block_removed", "remaining"},
			danger:   &dangerSpec{dangerD2, "模型名（`--model`）", "从名册件 `gateway/fleet.yaml` 的 `models:` 段删一条（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；不碰主控运行态）；可逆：把删掉那一行加回即原状", "缺口账 GAP-20260925-31 · 同族写面先例 zerg model add（D2）· §九 M3 C1", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdModelRm,
		},
		{
			path:     []string{"model", "set"},
			kind:     "ModelSet",
			summary:  "改名册件（`gateway/fleet.yaml`）里**点准的那一条**的字段（`--set k=v` · 保序原位改 · 其余字段逐字保留 · `--dry-run` 先行 · 真写要 `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚）——字段名闭集，无名键 / 列表值 / 值域不合 ⇒ 退 2（不猜）",
			usage:    "zerg model set --model <模型名> --host <主机> [--file <GGUF 路径>] --set k=v [--set k=v …] --confirm <模型名> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"模型名（--model：`models:` 段的键/块名）", "主机（--host：要改那条的 `host:`）"},
			fields:   []string{"model", "host", "fleet", "line", "changed", "entry"},
			danger:   &dangerSpec{dangerD2, "模型名（`--model`）", "改名册件 `gateway/fleet.yaml` 里点准的那一条的字段（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；不碰主控运行态）；可逆：把字段改回即原状", "缺口账 GAP-20260925-31 · 同族写面先例 zerg model add（D2）· §九 M3 C1", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdModelSet,
		},
	}
}
