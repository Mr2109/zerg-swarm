package main

// 命令表 · publish 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_publish() []*command {
	return []*command{
		{
			// 组4 §二.4 `W-50`（`研-禁:129` `R-16`）· 任务单序132 · 与组3 `Q-026` 同条：
			// 凡「在 / 不在」的答案都要带「口径 + 树 `head_sha`」；两代产出树答案不同 ⇒
			// 判「口径不同」· **不许并成一个数**（一树一行 ⇒ 两行）。
			path: []string{"publish", "tree", "has"},
			kind: "PublishTreeHas",
			// ★ 字段表必须写成 `[]string{…}` **字面量** —— 与上面 `net probe` 那条**同一条纪律**
			//   （契约脚本的 `FIELDS_RE` 只认字面量；抽成变量 = 那一条被读成「没有字段表」）。
			summary:  "产出树「在 / 不在」**只读**读数（一树一行 · 每行带**口径 + 树 `head_sha`** · 树身份取不成 ⇒ 8 · **不给结论**）",
			usage:    "zerg publish tree has <件> --tree <树>… [--json <字段>]",
			arity:    "any",
			args:     []string{"件名（相对树根的相对路径 —— 形状不合口径 ⇒ 2）"},
			fields:   []string{"tree", "caliber", "head_sha", "present"},
			endpoint: "",
			run:      cmdPublishTreeHas,
		},
		{
			// 病 · 已入账 `GAP-20260927-407`（缺面）：「这件**会不会**发」在命令面上**没有免树正门**
			// （`publish tree has` 必须有现成成品树）⇒ 只能绕、只能代码坐实，拿不到一个可手敲的口。
			// 本条目 = 发布判定链的**只读**机器面（免树）：逐层现读真源件复算 ——
			//   ① publish/whitelist.txt ② EXCLUDES（scripts/build/publish-public.sh）
			//   ③④ publish/mirror-public-lib.py 的 DROP_EXACT/DROP_PREFIX ⑤ 同件 map_path 映射丢弃。
			// 链真源读不到 ⇒ 8 · **不给结论**（空表会读成比真值更宽的答案）。
			path: []string{"publish", "set"},
			kind: "PublishSet",
			// ★ 字段表必须写成 `[]string{…}` **字面量** —— 与上面 `net probe` / `publish tree has`
			//   两条**同一条纪律**（契约脚本的 `FIELDS_RE` 只认字面量；抽成变量 = 该条被读成「没有字段表」）。
			summary:  "「这件会不会发 + 哪一层拦的」**只读**读数（**免树** · 逐层现读判定链真源件复算 · 链真源读不到 ⇒ 8 · **不给结论**）· ★ **身份格 `caliber`（口径）不是可选投影格**：`--json` 里**必须点名**它，缺 ⇒ 2 `identity_cells_required`（缺口 `GAP-20260927-415`）",
			usage:    "zerg publish set <仓内路径>… [--json <字段>]",
			arity:    "any",
			args:     []string{"仓内路径（相对仓根 —— 形状不合口径 ⇒ 2）"},
			fields:   []string{"path", "caliber", "will_publish", "layer", "public_path", "private_face", "non_blob_basis"},
			endpoint: "",
			run:      cmdPublishSet,
		},
		{
			path:    []string{"publish", "preflight"},
			kind:    "PublishPreflight",
			summary: "公开面预检（**只读** · 跑 scripts/build/publish-preflight.sh <产物目录> · 不改任何状态）",
			usage:   "zerg publish preflight <产物目录>",
			arity:   "any",
			args:    []string{"产物目录（`publish run` 出的镜像树）"},
			run:     cmdPublishPreflight,
		},
		{
			path:    []string{"publish", "run"},
			machine: machineHostName,
			danger:  &dangerSpec{dangerD3, "主机名", "出公开镜像树（本地 dist/<版本>/release · 白名单+排除项+脱敏 · 不出网、不推送 · 底层 scripts/build/publish-public.sh）", "GAP-20261005-39 · family_publish_run.go:275 自印「危险档：D3」· §17.2", true, dangerScopeRepo},
			opened:  true,
			kind:    "PublishRun",
			summary: "出公开镜像树（**本地** dist/<版本>/release · 白名单+排除项+脱敏 · 不出网、不推送）",
			usage:   "zerg publish run [--node <机>] [--legacy] [--check-excludes-parity] [--dry-run | --confirm=<主机名> --yes]",
			// 本版已开放（Mr2109 2026-09-25「脱敏推」授权）：真跑只写**本地**产物目录。
			// 「推远端」不在命令面 —— 不可逆动作按规矩等 Mr2109 发话后单独执行（不另开执行路径）。
			run: cmdPublishRun,
		},
		{
			// ── 命令树登记批（2026-10-09）：`zerg publish verify` 入口 ──────────────────
			// 真身 = `family_publish_verify_native.go` 的 `cmdPublishVerify` → `publishVerifyRun`
			// （两器产出一致性验收：`zerg publish verify` ⟷ `bash scripts/build/test-publish-parity.sh`
			// · **本地** · 不出网、不推送 · DEV-2136）。
			// ★ run 指向**本族承载入口** `cmdPublishVerifyFamily`（宿主 `ui_start.go` · 门78
			// `check-newcmd-followup` 面④ 要求「新增 path 的实现件已在 `scripts/公开标记.tsv`」；
			// 真身件未跟踪、不在该表内 ⇒ 照 `mcp serve` 形状经**已登记宿主**转发）。入口只转发、
			// ✗ 不改退码口径（真跑退码由 `publishVerifyRun` 定）。
			path:    []string{"publish", "verify"},
			kind:    "PublishVerify",
			summary: "两器产出一致性验收（**本地** · 不出网、不推送 · 真身 `family_publish_verify_native.go:publishVerifyRun`；入口只转发）",
			usage:   "zerg publish verify",
			arity:   "none",
			opened:  true,
			run:     cmdPublishVerifyFamily,
		},
		{
			// ── 收口编排件（本单 · main.go 开窗）：`zerg publish accept` ───────────────────
			// 真身 = `family_publish_accept_native.go` 的 `publishAcceptRun`：把 8 条收口验收
			// **逐条 rc AND 成单一结论**（任一红 ⇒ 不收口）。★ 判据单一真源 = 任务单 ㈤v2；
			// 门已判的**只调不重判**（验-1 调 `publishVerifyRun` · 验-6/7/8 按名进程内调门 ·
			// 验-5 取值 `gate ls --json`）；仅验-2/3/4 无既有门 ⇒ 编排件内薄适配器。
			// ★ 同形先例 = `publish preflight`（Go 侧声明式步表 + 进程内按名调门 + 三档出口）。
			// ★ 出口三档（只读主档）：全 rc=0 ⇒ 0（收口）/ 任一 rc=1 ⇒ 1（不收口）/ 缺件跑不动 ⇒ 2
			//   （fail-closed · ✗ 不把 2 折成 0）。★ `--out <件>` = D2 报告件（照 `gate matrix --out`
			//   三态门 + 原子落件；落**调用方给的路径** · 命令面不预设入仓）。
			path:     []string{"publish", "accept"},
			kind:     "PublishAccept",
			summary:  "8 条收口验收逐条 rc AND（任一红 ⇒ 不收口 · 缺件/跑不动 ⇒ 2 不给结论）；真身 `family_publish_accept_native.go`（判据单一真源 = 任务单 ㈤v2）",
			usage:    "zerg publish accept [--out <件>] [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			fields:   []string{"id", "title", "rc", "criteria"},
			endpoint: "",
			run:      cmdPublishAccept,
		},
		{
			// ── 片3（本单 · C12 接线）：`zerg publish mirror` —— 逐提交镜像器入命令面 ───────────
			// 真身 = `family_mirror_public_native.go:mirrorPublicNative`（旧壳 `publish/mirror-public.sh`
			// 的 Go 真身 · 编排五段 · 引擎 `publish/mirror-public-lib.py` 逐字节内嵌）。原状
			// 「真身在盘、命令树**无**入口」⇒ `zerg publish mirror` 一律 rc=2（B2/R1 病根）。
			// ★ **B 式宿主桥**（C12 §模板B · **零新表行**）：真身件**不在** `scripts/公开标记.tsv`
			//   （门78 `check-newcmd-followup` 面④ 要求「新增 path 的实现件已在标记表」）⇒ `run:`
			//   指向**已登记宿主** `ui_start.go`（该表 `:158` `public=yes`）的桥 `cmdPublishMirror`
			//   —— 与同族 `publish verify`（`run: cmdPublishVerifyFamily`）**同一条形状**。
			// ★ **命令面只缴 `--dry-run`** ⇒ 桥映射到真身 `--list-only`（真·零写盘/零联网/零推送的
			//   只读预演）；真跑（出**本地**镜像树）按 **D3** 三态（照 `publish run`：缺
			//   `--confirm=<主机名>` 或 `--yes` ⇒ 2 fail-closed）。
			// ★ **`--push` 不上命令面**（族纪律「推远端不在命令面」· C12 §5.3②）—— 桥里**拒**该旗标。
			// ★ `passthrough: true`：除上面两枚外旗标逐字交真身自己的 argv 解析面（与 `doc kb-sync`
			//   同一种「透传型」形态：真身自带解析，命令面不复刻第二套旗标表）。
			path:        []string{"publish", "mirror"},
			kind:        "PublishMirror",
			summary:     "逐提交镜像（**本地** · 出镜像树 · 不出网、不推送 · 真身 `family_mirror_public_native.go:mirrorPublicNative`）；命令面只缴 `--dry-run` ⇒ 真身 `--list-only` 只读预演（零写盘）· `--push` 不上命令面",
			usage:       "zerg publish mirror [--dry-run | --confirm=<主机名> --yes]",
			arity:       "any",
			args:        []string{"（无位置参数：`--dry-run`（= 真身 `--list-only`）为只读预演；其余旗标逐字透传真身）"},
			endpoint:    "",
			danger:      &dangerSpec{dangerD3, "主机名", "出**本地**逐提交镜像树（真跑按 D3 三态：缺 `--confirm=<主机名>`/`--yes` ⇒ 2）—— **不出网、不推送**（`--push` 不上命令面）", "C12 §5.3② · 同族先例 publish run（D3）· 族纪律「推远端不在命令面」", true, dangerScopeRepo},
			opened:      true,
			passthrough: true,
			run:         cmdPublishMirror,
		},
		{
			// ── 命令树登记（本单）：`zerg publish snapshot-self-test` —— 发布链 Go 半余件接入命令面 ──
			// 真身 = `family_publish_snapshot_native.go:publishSnapshotSelfTestNative`（旧 py
			// `scripts/build/publish-snapshot-self-test.py` 205 行的 Go 真身）：把 `publish/ci/*.yml`
			// 里的 `run:` 步骤在**公开快照目录**上原样跑一遍（判据真源就是那两件 yml · 本件不另写判据）。
			// 原状「真身在盘、命令树无入口」⇒ 一律 rc=2 —— 缺口 `GAP-20261012-16`（门 `check-cli-ownership`
			// 类 B「未接 inproc 标准路由」余 1 件的正解）。
			// ★ run 指向**已登记宿主** `ui_start.go` 的桥 `cmdPublishSnapshotSelfTestFamily`（门78
			// `check-newcmd-followup` 面④：真身件不在 `scripts/公开标记.tsv` ⇒ 经已登记宿主转发）。
			// ★ `passthrough: true`（同 `publish mirror`）：`--job`/`--json` 逐字交真身自己的 argv 解析面。
			// ★ 字段表 = 真身 `--json` 的五格顶层键，必须写成 `[]string{…}` **字面量**（契约脚本 `FIELDS_RE`
			// 只认字面量 · 同 `net probe`/`publish tree has` 那条纪律）。
			path:        []string{"publish", "snapshot-self-test"},
			kind:        "PublishSnapshotSelfTest",
			summary:     "公开快照自测（**本地** · 把 `publish/ci/*.yml` 的 `run:` 步骤在公开快照目录上原样跑一遍 · 判据真源 = 那两件 yml · 真身 `family_publish_snapshot_native.go`；入口只转发）",
			usage:       "zerg publish snapshot-self-test <快照目录> [--job <作业>] [--json <字段>]",
			arity:       "any",
			args:        []string{"快照目录（公开快照的根 —— 不是目录/不存在 ⇒ 2）"},
			fields:      []string{"pass", "fail", "skip", "snapshot_has_git", "wall_secs"},
			endpoint:    "",
			passthrough: true,
			run:         cmdPublishSnapshotSelfTestFamily,
		},
	}
}
