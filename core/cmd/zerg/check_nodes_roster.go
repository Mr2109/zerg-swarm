// check_nodes_roster.go —— 判据件 `check-nodes-roster.py`（819 行）的 **Go 真搬**
// （甲案 C 类 · M2 · 入口 `checkNodesRosterGateMain`）。
//
// 为什么有这件（M2 = 按价值真搬 Go · 落点 core/cmd/zerg/）：
//
//	验收判据 = 对同一 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码三面逐字相同
//	（除下列**显式声明**的口径差面与未覆盖面）。本件**只落码、不接线**（✗ 不注册命令 ·
//	✗ 不删 py 原件 · 入口暂无调用点）。
//
// 本门 = 「机器名册真源唯一」的核对门：读本机名册（~/.zerg 下，不入仓）+ 仓内 gateway 的
// fleet 件 + deploy 的 plist + 在跑的 agentd（`ps` 只读），判 B/C/D/E/F/G 六条。
//
// ═══════════════════ 出厂声明行 (a) 默认根定位差异 ═══════════════════
//
//	py 从 `__file__` 上溯两层得 REPO_ROOT（= 仓根）；Go 无 `__file__` 语境 ⇒ 默认根 = cwd
//	（`os.Getwd()`，同 check-curl-timeout / check-ai-direct-edits 的既有口径）。⇒ 「脚本位置 ≠ cwd」
//	时 `--fleet` / `--plist` 的默认推导面不同（✗ 不声称逐字对齐）。对拍甲档显式传三路径；
//	乙档把 py 副本放进 `<fx>/scripts/gates/` 且 Go 以 cwd=`<fx>` 跑 ⇒ 两面同根。
//
// ═══════════════════ 出厂声明行 (b) 自指检查 ═══════════════════
//
//	真判：本件是否落在它自己扫描的面内？本门扫描面 = **三件具体文件**（本机名册 · 仓内 fleet 件 ·
//	盘上 plist）＋ 一条 `ps` 读数；**不含** core/**、不含任何目录遍历、不含 `git ls-files` 面
//	⇒ 本件**不在自己的扫描面内**（依据：py 全件零 `walk`/`listdir`/`glob`/`scandir` 调用，
//	读面逐条点名为具体路径）。⇒ 本单**不做影子根**，理由与命中数见回执 selfref 栏。
//	第四形态（落在他门扫描面内）：本件正文以**描述式**复述被扫形态（如真源路径前缀、机器名
//	旗标名），✗ 不作独立可解析字面量；夹具串按 py 原样保留（它们是**夹具输入**、不是扫描面命中）。
//
// ═══════════════════ 输出行序的决定者 ═══════════════════
//
//	py 全件**零目录遍历**（无 walk/listdir/glob）⇒ 不存在「遍历序」面。输出行序由三处决定：
//	 ① 判据/检查项的**代码追加序**（结论层本机身份 → B fleet → B updates → C → D → E → G → 再 F plist / F ps）；
//	 ② py **明写 `sorted(...)`** 的序：名册行 `sorted(nodes)` · `sorted(set(nodes)-set(fleet))` ·
//	    `sorted(set(fleet)-set(nodes))` · `sorted(fleet.items())`（按**键**）· `sorted(hosts-set(nodes))`
//	    ⇒ Go 侧一律 `sort.Strings`（UTF-8 字节序 == py code point 序）；
//	 ③ 列表面 `updates.nodes[]` 按 **parse_seq 产出序**（= 行序）。
//	★ 零 Go map 迭代进输出：凡 py 迭代 dict 的点，Go 侧都走显式排序的键切片。
//
// ═══════════════════ 正则面（本件 2 条）═══════════════════
//
//	py E：`re.search(r"\bhost\s*:\s*([A-Za-z0-9_.\-]+)", l)`。语义差两条：
//	 ① `\s` = py Unicode 空白 ⇒ 本件 `cnrIsSpace` 按码点展开（含 \x1c-\x1f / \x85 / \xa0 …）；
//	 ② `\b` = py 的 **Unicode** 词边界（CJK 亦算词字符）⟷ Go RE2 的 `\b` 是 ASCII
//	    ⇒ 本件**手写**扫描器 `cnrModelsHost`：用 py 口径的词字符判定复刻 `\b`，再逐字匹配
//	    `host\s*:\s*([A-Za-z0-9_.\-]+)`（值类**原样 ASCII**，✗ 不换成 \p{Nd}）。
//	`re.escape` 只用于 `section_range` 的**常量键**（全 ASCII 字母）⇒ 无差。
//
//	py G：`ANCHOR_RE = re.compile(r"^[0-9a-fA-F]{64}$")` —— 全 ASCII 字符类 + 锚点。
//	本件 `cnrAnchorRe`（regexp）等价。★ 判据 G 的两处 `%r`（锚格式错）在 py 是 str repr，
//	本件走 `cnrRepr`（近似；非 ASCII 角落见未覆盖面）。
//
// ═══════════════════ py 现读硬伤（逐字沿用 · 交回入账 · ✗ 本单不修 py）═══════════════════
//
//	① **C 类**：py 无最外层兜底 —— `check_fleet` 里读 fleet 件**未捕异常**（件在盘但读不了 ⇒
//	   回溯 + rc=1）。本件 fail-closed **rc=2**（写进未覆盖面）。
//	② **B 类**：`load_roster` 的 `except Exception`（名册读不了 ⇒ 落 rc=2 文案）—— 逐字沿用。
//	③ **子进程面**：`running_agentd_machine()` 真跑 `ps -Ao command=`（只读 · 不碰进程）—— 逐字沿用。
//	④ **A 类**：`tempfile.TemporaryDirectory()` **有**回收（with 块）⇒ 本件 `defer os.RemoveAll` 对齐
//	   （自检前后同前缀目录计数 delta=0）。
//	⑤ **D 类**：`--list-rules` 真打印 `__doc__`（非死旗标）⇒ 本件逐字复刻其正文。
//
// ═══════════════════ 未覆盖面（✗ 不声称逐字对齐）═══════════════════
//
//	① **C 类兜底档**：见硬伤①（py 回溯 rc=1 ⟷ 本件 BLOCKED rc=2）。
//	② **argparse 极端内部形态**：`--` 分隔符 / 缩写歧义报话 / `--flag=v` 打在 flag 上 / 短旗标簇
//	   / 位置参数 —— 本件按「未知 ⇒ unrecognized / 缺值 ⇒ expected one argument」处理，深角正文不同。
//	③ **默认根定位**：见声明行 (a)。py 的 prog 名来自 argv[0] 基名；本件**定名**（同 py 常规调用）。
//	④ **非 UTF-8 角落**：py `open(encoding="utf-8")` 严格解码（非法序列抛异常）；本件 utf8.Valid 判否
//	   ⇒ 走「读不了」档。真仓三件现读全为合法 UTF-8 ⇒ 不触发。
//	⑤ **universal newlines**：py 文本模式把 `\r\n` / 裸 `\r` 折成 `\n`；本件显式同折。
//	⑥ **异常正文**：py `"名册读不了：%s" % e` / `"F plist 读不了（%s）：%s"` 里是 **py 异常 repr**；
//	   本件为 Go error 文本 ⇒ 极端读失败档的**正文字节**不同（rc 面一致）。
//	⑦ **`ps` 分列**：py `str.split()` 与 Go `strings.Fields` 对 `\x1c-\x1f` 一类判法略异 ⇒ 极端行不同。
//	⑧ **嵌套块取值**：py `str(dict)` 于「字段值竟是子块」时产出 `{...}`；本件走 `%v` ⇒ 该档不同
//	   （真仓三件的相关字段全是标量 ⇒ 不触发）。
//	⑨ **二进制 plist**：py `plistlib` 认 binary plist；本件只解 XML plist。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ─────────────────────────── py 出处的常量串（生成器注入 · 逐字节取自 py 原件）───────────────────────────

// cnrDoc —— py 模块 `__doc__`（`--list-rules` 的首段输出）。
const cnrDoc = "check-nodes-roster.py — 机器名册的**唯一真源核对**（契约 §九 M13 `Z2` / `P-069`）\n\n为什么存在\n----------\n「名册有三处并存、一个真源都没有」（`调研-M13` §4.1 实测）：`gateway/fleet.yaml` 的 `fleet:`\n（子端寻址）· 同文件的 `updates.nodes`（换件 ssh 目标）· 子端的机器名旗标（子端自报）。\n三处各自更新 ⇒ 任何「按名找到一台机器」的行为都建立在一张会漂的表上。\n\n定案（契约 §九 `Z2`）**名册真源唯一** = `~/.zerg/contexts/<档>.yaml` 的 `nodes:`；\n`fleet:` 与 `updates.nodes` 降为**投影源** —— 处置只给了两条路：① **由名册生成**，或\n② **被门禁逐项核对**。本门走 ②（①要动主控的配置加载面，属写面，不在本批）。\n\n⇒ **建而不收 = 又多一处**：只新建名册文件、不把三处旧来源绑到它，名册就只是第 4 份拷贝。\n本门就是那条「绑」——它同时判两件事：\n  ① 两段投影**有没有声明自己在投影**（段内必须写明真源路径）——没有声明 ⇒ rc=1（这才是「收」）；\n  ② 声明了之后**值面是否逐字相同**（host/port/os/ssh/root/prefix/api/components 逐字段）。\n\n判据（A–G；A–F 与 G 的「闭集 / reported」支逐条能独立变红，G 的「true 须带 64hex 锚」支**降级为只报告**）\n--------------------------\n  A. 名册可读且 `nodes:` 非空（否则 rc=2 不给结论 —— 真源不在，判不了）\n  B. `gateway/fleet.yaml` 的 `fleet:` 段与 `updates:` 段**各自**带投影源声明（段内出现\n     `~/.zerg/contexts/`）——「建而不收」的直接判据\n  C. `fleet:` 键集 == 名册键集（**两向**）；每台 host/port/os 与名册逐字相同\n  D. `updates.nodes[]`：name ∈ 名册；ssh/root/prefix/api/components 与名册逐字相同\n  E. `models:` 段里每个 host ∈ 名册键（模型驻留在哪台 —— 第 4 处，顺带核）\n  F. **本机**子端机器名旗标 ∈ 名册键：**本机盘上 plist**（`deploy/com.zerg.agent-Mr2109.plist`）\n     与**本机在跑的 `zerg-agentd`**（`ps` 读，只读）两处都核；没有在跑的进程 ⇒ 打印跳过（不是红）。\n     ★ 读面 = 本机（local）—— 远端子端本机读不到，见「结论范围」。\n  G. `models:` 段里每条的 `verified` 是**闭集值**，「实测级」带**可核的内容锚**为**报告项**：\n       · `verified: true`（实测级）—— **必须**带 `sha256:`/`digest:`/`sha:` 之一，且值 = 64 位\n         十六进制；缺锚 / 锚格式不对 ⇒ **降级为只报告**（逐条列名，**不进 rc**）——\n         GAP-20261003-21：`verified` 的消费者 schema 是 bool ⇒ 该锚恒缺、原「rc=1 点名」恒红且修不了。\n         给了 `--verify-sha` 且该条 host 是本机、`file:` 在盘上 ⇒ 现算 sha256 逐字比对\n         （内容寻址真核）；本机读不到（远端 host / 件不在 / 未开开关）⇒ 该锚记为**声明级**。\n       · `verified: reported`（声明级）—— 必须带 `verified_where:`（∈ 名册键），表示「由哪台自报」。\n       · `verified: false` ⇒ 放过（就是「没实测」）；闭集外的取值 ⇒ rc=1 点名。\n       · 没写 `verified` 字段的条目 ⇒ 本判据不管（E 已核它的 host 在册）。\n  H. **拨号地址（`--controller`）由名册可推导**（`GAP-20260927-03`）：仓内每个子端 plist 的\n     `--controller` 值（`${VAR:-默认}` 变量形态取**默认值**）必须**能由名册推出来** ——\n       host 必须 ∈ 可推导集 = { 回环 `127.0.0.1`/`::1`/`localhost` } ∪ { `nodes:` 每台的 host }；\n       端口必须 == 名册顶层 `core:` 的端口。\n     host 落在名册**没有的网段**（如 `192.168.110.x`）或端口不符 ⇒ **rc=1**，**逐字点名 plist 路径与该值**\n     （换网/改 IP 时 plist 与名册各说各话的那种漂移）。plist 不在 / 无 `--controller` ⇒ 打印跳过（不是红）。\n\n结论范围（read_layer × verdict_scope —— 本门**只用本机读数**，故判词必须带层限）\n------------------------------------------------------------------------------\n  本门的全部读取面都是**本机**：本机盘上的 `gateway/fleet.yaml`、本机 `~/.zerg/contexts/`\n  名册、本机 `deploy/…plist`、本机 `ps -Ao command=`（`read_layer=local`）。⇒ 判词一律\n  `verdict_scope=本机`，并**逐台列出本门读不到、因而不给结论的远端节点**；★ 严禁把本机\n  读数冒称「机群级/全对」。远端节点的一致性须由**各机自己**跑本门回答（或该机自报 +\n  `verified: reported` + `verified_where`）。\n\n退码（与仓内既有门一致的三档）\n------------------------------\n  0 = **本机层**全对（PASS · verdict_scope=本机）—— ★ 不表示远端节点也对（见「结论范围」）\n  1 = 有漂移（FAIL）—— 逐条给「哪一处 · 哪个字段 · 现值 vs 名册值」\n  2 = **不给结论**（缺件 / 解析不出 `nodes:` / `fleet.yaml` 不在 / 自检不过 / 空转假覆盖）\n\n★ 本机物件：名册住在 `~/.zerg/`（**不入任何仓**）⇒ 在没建名册的机器上本门**必然** rc=2\n  （「名册不在、判不了」是事实，不是「绿」）。所以它**不进提交闸的快速档**（`FAST_STEPS`）：\n  否则每台没建名册的机器**每次提交**都会被拦一格 —— 那不叫严格，是把闸废掉（同 `check-wired-scripts`\n  升档时的先例）。它挂在**默认（全量）档** 的 `gates` scope 里。\n\n用法\n----\n    python3 scripts/gates/check-nodes-roster.py                 # 核真目标（仓内 fleet.yaml + 本机名册）\n    python3 scripts/gates/check-nodes-roster.py --roster P      # 换名册路径（默认 ~/.zerg/contexts/default.yaml）\n    python3 scripts/gates/check-nodes-roster.py --list-rules    # 打判据 / 路径来源 / 退码\n    python3 scripts/gates/check-nodes-roster.py --self-test     # 只跑自检（合成夹具，不碰真目标）\n    python3 scripts/gates/check-nodes-roster.py --verify-sha    # 另对「本机可读的 verified: true」现算 sha256 真核\npython3 scripts/gates/check-nodes-roster.py --ctrl-plist P  # 换判据 H「拨号地址对账」要核的 plist（可重复）\n"

// cnrHelpText —— py argparse `-h/--help` 的 stdout **逐字**（Python 3.9 用 `optional arguments:`）。
const cnrHelpText = "usage: check-nodes-roster.py [-h] [--roster ROSTER] [--fleet FLEET]\n                             [--plist PLIST] [--self-test] [--list-rules]\n                             [--verify-sha] [--ctrl-plist P]\n\noptional arguments:\n  -h, --help       show this help message and exit\n  --roster ROSTER\n  --fleet FLEET\n  --plist PLIST\n  --self-test\n  --list-rules\n  --verify-sha     对本机可读的 verified: true 条目现算 sha256 真核（默认只查锚在不在/格式）\n  --ctrl-plist P   判据 H 要核的 plist（可重复；默认 仓内 deploy/ 与 gateway/deploy/ 两件）\n"

// cnrUsageLine —— argparse 出错时的 `usage:` 两行（与 help 首两行同）。
const cnrUsageLine = "usage: check-nodes-roster.py [-h] [--roster ROSTER] [--fleet FLEET]\n                             [--plist PLIST] [--self-test] [--list-rules]\n                             [--verify-sha] [--ctrl-plist P]\n"

// cnrRosterFix / cnrFleetFix / cnrPlistFix —— py 自检夹具（逐字节取自 py 原件）。
const cnrRosterFix = "# 夹具名册\ncontext: default\ncore: http://127.0.0.1:8580\nnodes:\n  edge-node:\n    host: <worker-ip>\n    port: 8100\n    os: ubuntu\n    ssh: g01@<worker-ip>\n    root: /home/<user>/zerg\n    prefix: /home/<user>/zerg/bin\n    api: \"http://127.0.0.1:8580\"\n    components: \"core,agentd\"\n  Mr2109:\n    host: 127.0.0.1\n    port: 8100\n    os: macos\n"

const cnrFleetFix = "# 夹具 fleet.yaml\nupdates:\n  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n  check: true\n  nodes:\n    - { name: edge-node, ssh: g01@<worker-ip>, root: /home/<user>/zerg, prefix: /home/<user>/zerg/bin, api: \"http://127.0.0.1:8580\", components: \"core,agentd\" }\nmodels:\n  m1: { host: edge-node, backend: llama-server }\nfleet:\n  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n  edge-node: { host: <worker-ip>, port: 8100, os: ubuntu }\n  Mr2109: { host: 127.0.0.1, port: 8100, os: macos }\n"

const cnrPlistFix = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict>\n  <key>Label</key><string>com.zerg.agent-Mr2109</string>\n  <key>ProgramArguments</key><array>\n    <string>/tmp/zerg-agentd</string>\n    <string>--machine</string><string>Mr2109</string>\n  </array>\n</dict></plist>\n"

// cnrPlistCtrlVarFix / cnrPlistCtrlLiteralFix —— py 判据 H 自检夹具（拨号地址 · `__CTRL__` 由 self_test 代入）；
//
//	变量形态（shell 包裹）与字面形态各一（逐字节取自 py 原件 PLIST_CTRL_VAR_FIX / PLIST_CTRL_LITERAL_FIX）。
const cnrPlistCtrlVarFix = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict>\n  <key>Label</key><string>com.x3gw.agent</string>\n  <key>ProgramArguments</key><array>\n    <string>/bin/sh</string>\n    <string>-c</string>\n    <string>exec /usr/bin/python3 ~/agent/agent.py --controller \"${ZERG_AGENT_CONTROLLER:-__CTRL__}\" --machine mini-node-1</string>\n  </array>\n</dict></plist>\n"

const cnrPlistCtrlLiteralFix = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict>\n  <key>Label</key><string>com.x3gw.agent</string>\n  <key>ProgramArguments</key><array>\n    <string>/usr/bin/python3</string><string>~/agent/agent.py</string>\n    <string>--controller</string><string>__CTRL__</string>\n    <string>--machine</string><string>mini-node-1</string>\n  </array>\n</dict></plist>\n"

// cnrVerifiedValues / cnrAnchorFields / cnrAnchorRe / cnrLocalHosts —— 判据 G 的闭集与内容锚
// （GAP-20260927-167/-171 · 逐字对齐 py 候件的 VERIFIED_VALUES / ANCHOR_FIELDS / ANCHOR_RE / LOCAL_HOSTS）。
var cnrVerifiedValues = []string{"true", "false", "reported"}
var cnrAnchorFields = []string{"sha256", "digest", "sha"}
var cnrAnchorRe = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// cnrLocalHosts —— 本机回环寻址（fleet: 段里 host 落这些值 = 本机 · read_layer=local 的身份源）。
var cnrLocalHosts = map[string]bool{"127.0.0.1": true, "::1": true, "localhost": true}

// ── 判据 H：仓内子端 plist 的拨号地址（`--controller`）**由名册可推导**（GAP-20260927-03）──
// cnrControllerPlists —— 判据 H 默认要核的仓内件（可被 --ctrl-plist 覆盖）：两枚子端 plist。
func cnrControllerPlists() []string {
	return []string{
		cnrPyJoin(cnrRepoRoot(), "deploy", "com.zerg.agent-Mr2109.plist"),
		cnrPyJoin(cnrRepoRoot(), "gateway", "deploy", "com.x3gw.agent.mini.plist"),
	}
}

// cnrCtrlVarRe —— py `CTRL_VAR_RE`（`${VAR:-默认}` 变量形态 · re.S ⇒ `(?s)`）。
var cnrCtrlVarRe = regexp.MustCompile(`(?s)^\$\{([A-Za-z_][A-Za-z0-9_]*):-(.*)\}$`)

// cnrCtrlURLRe —— py `CTRL_URL_RE`（`^(https?)://([^/:]+)(?::(\d+))?(/.*)?$`）。
var cnrCtrlURLRe = regexp.MustCompile(`^(https?)://([^/:]+)(?::(\d+))?(/.*)?$`)

// cnrFleetVerifiedFix —— py 自检夹具 `FLEET_VERIFIED_FIX`（逐字节取自 py 原件）。
const cnrFleetVerifiedFix = "# 夹具 fleet.yaml（判据 G）\nupdates:\n  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n  check: true\n  nodes:\n    - { name: edge-node, ssh: g01@<worker-ip>, root: /home/<user>/zerg, prefix: /home/<user>/zerg/bin, api: \"http://127.0.0.1:8580\", components: \"core,agentd\" }\nmodels:\n  mG:\n    - { host: __HOST__, backend: llama-server, file: __FILE__, __VERIFIED__ }\nfleet:\n  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n  edge-node: { host: <worker-ip>, port: 8100, os: ubuntu }\n  Mr2109: { host: 127.0.0.1, port: 8100, os: macos }\n"

// cnrValue —— py 无类型值的近似（string | map[string]cnrValue | []cnrValue）。
type cnrValue = interface{}

// ══════════════════════════ 路径 / 文件原语 ══════════════════════════

// cnrPyJoin —— 照 CPython posixpath.join 的逐段规则（✗ 不用 filepath.Join，见标准声明行）。
func cnrPyJoin(parts ...string) string {
	out := ""
	have := false
	for _, p := range parts {
		if !have {
			if p != "" {
				out = p
				have = true
			}
			continue
		}
		if len(p) > 0 && p[0] == '/' {
			out = p
			continue
		}
		if out == "" || out[len(out)-1] == '/' {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// cnrHome —— py `os.path.expanduser("~")`。
func cnrHome() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

// cnrIsFile —— py `os.path.isfile`（stat 跟随符号链接）。
func cnrIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// cnrDefaultRoster / Fleet / Plist —— py 三个缺省路径（REPO_ROOT 见声明行 (a)）。
func cnrRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func cnrDefaultRoster() string { return cnrPyJoin(cnrHome(), ".zerg", "contexts", "default.yaml") }
func cnrDefaultFleet() string  { return cnrPyJoin(cnrRepoRoot(), "gateway", "fleet.yaml") }
func cnrDefaultPlist() string {
	return cnrPyJoin(cnrRepoRoot(), "deploy", "com.zerg.agent-Mr2109.plist")
}

// cnrReadTextLines —— py `read_lines`：→ (raw, stripped)。文本模式折行同 py。
func cnrReadTextLines(path string) ([]string, []string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if !utf8.Valid(b) {
		return nil, nil, fmt.Errorf("invalid utf-8")
	}
	s := string(b)
	s = cnrReplaceAll(s, "\r\n", "\n")
	s = cnrReplaceAll(s, "\r", "\n")
	raw := cnrSplitStr(s, "\n")
	stripped := make([]string, len(raw))
	for i, l := range raw {
		stripped[i] = cnrStripComment(l)
	}
	return raw, stripped, nil
}

// ══════════════════════════ py 空白 / 字符串原语 ══════════════════════════

// cnrIsSpace —— py `\s`（Unicode · 含 \x1c-\x1f / \x85 / \xa0）。
func cnrIsSpace(r rune) bool {
	switch r {
	case 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// cnrPyStrip —— py `str.strip()`（无参 · Unicode 空白）。
func cnrPyStrip(s string) string {
	s = cnrTrimLeftFunc(s, cnrIsSpace)
	s = cnrTrimRightFunc(s, cnrIsSpace)
	return s
}

// cnrLstripSpaces —— py `s.lstrip(" ")`（**只剥 ASCII 空格**）。
func cnrLstripSpaces(s string) string {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return s[i:]
}

// cnrSkipSpace —— 跳过前导 py 空白（`\s*`）。
func cnrSkipSpace(s string) string {
	for len(s) > 0 {
		r, sz := utf8.DecodeRuneInString(s)
		if !cnrIsSpace(r) {
			break
		}
		s = s[sz:]
	}
	return s
}

// cnrStripComment —— py `strip_comment`。
func cnrStripComment(line string) string {
	var b []rune
	quote := rune(0)
	depth := 0
	prev := rune(0)
	first := true
	for _, ch := range line {
		if quote != 0 {
			b = append(b, ch)
			if ch == quote {
				quote = 0
			}
			prev = ch
			first = false
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			b = append(b, ch)
			prev = ch
			first = false
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			if depth > 0 {
				depth--
			}
		}
		if ch == '#' && depth == 0 && (first || prev == ' ' || prev == '\t') {
			break
		}
		b = append(b, ch)
		prev = ch
		first = false
	}
	return cnrPyRstrip(string(b))
}

// cnrPyRstrip —— py `str.rstrip()`（无参 · Unicode 空白）。
func cnrPyRstrip(s string) string { return cnrTrimRightFunc(s, cnrIsSpace) }

// cnrSplitTop —— py `split_top`（忽略引号与花括号内的 sep）。
func cnrSplitTop(text string, sep rune) []string {
	var parts []string
	var buf []rune
	quote := rune(0)
	depth := 0
	for _, ch := range text {
		if quote != 0 {
			buf = append(buf, ch)
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			buf = append(buf, ch)
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
		}
		if ch == sep && depth == 0 {
			parts = append(parts, string(buf))
			buf = nil
			continue
		}
		buf = append(buf, ch)
	}
	parts = append(parts, string(buf))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if cnrPyStrip(p) != "" {
			out = append(out, cnrPyStrip(p))
		}
	}
	return out
}

// cnrUnquote —— py `unquote`（剥外层成对引号）。
func cnrUnquote(v string) string {
	v = cnrPyStrip(v)
	r := []rune(v)
	if len(r) >= 2 && r[0] == r[len(r)-1] && (r[0] == '"' || r[0] == '\'') {
		return string(r[1 : len(r)-1])
	}
	return v
}

// cnrKVMatch —— py `_KV.match(s)`：`^([^:\s#][^:]*):\s*(.*)$` → (g1, g2, ok)。
func cnrKVMatch(s string) (string, string, bool) {
	idx := cnrIndexByte(s, ':')
	if idx < 0 {
		return "", "", false
	}
	key := s[:idx]
	if key == "" {
		return "", "", false
	}
	r, _ := utf8.DecodeRuneInString(key)
	if r == ':' || cnrIsSpace(r) || r == '#' {
		return "", "", false
	}
	return key, cnrPyStrip(s[idx+1:]), true
}

// cnrParseFlow —— py `parse_flow`：`{ k: v, ... }` → map（键值皆字符串）。
func cnrParseFlow(text string) map[string]cnrValue {
	inner := cnrPyStrip(text)
	if len(inner) > 0 && inner[0] == '{' {
		inner = inner[1:]
	}
	if len(inner) > 0 && inner[len(inner)-1] == '}' {
		inner = inner[:len(inner)-1]
	}
	out := map[string]cnrValue{}
	for _, part := range cnrSplitTop(inner, ',') {
		k, _, v := cnrPartition(part, ':')
		out[cnrUnquote(k)] = cnrUnquote(v)
	}
	return out
}

// cnrParseBlock —— py `parse_block`。
func cnrParseBlock(lines []string) map[string]cnrValue {
	out := map[string]cnrValue{}
	i := 0
	for i < len(lines) {
		raw := lines[i]
		if cnrPyStrip(raw) == "" {
			i++
			continue
		}
		ind := len(raw) - len(cnrLstripSpaces(raw))
		s := cnrPyStrip(raw)
		if len(s) > 0 && s[0] == '-' {
			i++
			continue
		}
		g1, val, ok := cnrKVMatch(s)
		if !ok {
			i++
			continue
		}
		key := cnrUnquote(g1)
		if val == "" {
			j := i + 1
			var sub []string
			for j < len(lines) {
				l2 := lines[j]
				if cnrPyStrip(l2) == "" {
					j++
					continue
				}
				if len(l2)-len(cnrLstripSpaces(l2)) <= ind {
					break
				}
				sub = append(sub, l2)
				j++
			}
			subStartDash := len(sub) > 0 && len(cnrPyStrip(sub[0])) > 0 && cnrPyStrip(sub[0])[0] == '-'
			if subStartDash {
				out[key] = cnrParseSeq(sub)
			} else {
				out[key] = cnrParseBlock(sub)
			}
			i = j
			continue
		}
		if len(val) > 0 && val[0] == '{' {
			out[key] = cnrParseFlow(val)
		} else {
			out[key] = cnrUnquote(val)
		}
		i++
	}
	return out
}

// cnrParseSeq —— py `parse_seq`。
func cnrParseSeq(lines []string) []cnrValue {
	var items []cnrValue
	var cur map[string]cnrValue
	hasCur := false
	i := 0
	for i < len(lines) {
		raw := lines[i]
		if cnrPyStrip(raw) == "" {
			i++
			continue
		}
		s := cnrPyStrip(raw)
		if len(s) > 0 && s[0] == '-' {
			ind := len(raw) - len(cnrLstripSpaces(raw))
			body := cnrPyStrip(s[1:])
			if hasCur {
				items = append(items, cur)
			}
			cur = map[string]cnrValue{}
			hasCur = true
			if len(body) > 0 && body[0] == '{' {
				for k, v := range cnrParseFlow(body) {
					cur[k] = v
				}
				i++
				continue
			}
			if g1, g2, ok := cnrKVMatch(body); ok {
				k := cnrUnquote(g1)
				if len(g2) > 0 && g2[0] == '{' {
					cur[k] = cnrParseFlow(g2)
				} else {
					cur[k] = cnrUnquote(g2)
				}
			}
			i++
			for i < len(lines) && cnrPyStrip(lines[i]) != "" &&
				(len(lines[i])-len(cnrLstripSpaces(lines[i]))) > ind {
				if g1, g2, ok := cnrKVMatch(cnrPyStrip(lines[i])); ok {
					k := cnrUnquote(g1)
					if len(g2) > 0 && g2[0] == '{' {
						cur[k] = cnrParseFlow(g2)
					} else {
						cur[k] = cnrUnquote(g2)
					}
				}
				i++
			}
			continue
		}
		i++
	}
	if hasCur {
		items = append(items, cur)
	}
	return items
}

// cnrSectionRange —— py `section_range`：顶层键 key 的半开区间。
func cnrSectionRange(lines []string, key string) (int, int, bool) {
	start := -1
	for i, l := range lines {
		if cnrHasPrefix(l, key) {
			rest := cnrSkipSpace(l[len(key):])
			if len(rest) > 0 && rest[0] == ':' {
				start = i + 1
				break
			}
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	end := len(lines)
	for j := start; j < len(lines); j++ {
		l := lines[j]
		if cnrPyStrip(l) != "" && !cnrHasPrefix(l, " ") && !cnrHasPrefix(l, "\t") && !cnrHasPrefix(l, "#") {
			end = j
			break
		}
	}
	return start, end, true
}

// cnrSectionLines —— py `section_lines`。
func cnrSectionLines(lines []string, key string) []string {
	s, e, ok := cnrSectionRange(lines, key)
	if !ok {
		return nil
	}
	return lines[s:e]
}

// ══════════════════════════ 取值 / 输出原语 ══════════════════════════

// cnrToString —— py `str(x)` 的近似（缺键 ⇒ 调用方已落 ""）。
func cnrToString(v cnrValue) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

// cnrStripQuotes —— py `str(x).strip('"')`。
func cnrStripQuotes(s string) string { return cnrTrimChars(s, '"') }

// cnrRepr —— py `repr(str)` 的近似（可读面 · 见未覆盖面）。
func cnrRepr(s string) string {
	var b []rune
	b = append(b, '\'')
	for _, r := range s {
		switch r {
		case '\\':
			b = append(b, '\\', '\\')
		case '\'':
			b = append(b, '\\', '\'')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			if r < 0x20 || r == 0x7f {
				b = append(b, []rune(fmt.Sprintf("\\x%02x", r))...)
			} else {
				b = append(b, r)
			}
		}
	}
	b = append(b, '\'')
	return string(b)
}

// cnrLjust —— py `"%-40s"`（**按码点**补齐，✗ 非字节）。
func cnrLjust(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return s
	}
	return s + cnrRepeat(" ", n-len(r))
}

// cnrSortedKeys —— 排好序的键切片（唯一进输出的 dict 序来源）。
func cnrSortedKeys(m map[string]cnrValue) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// ══════════════════════════ 判据 A–F ══════════════════════════

// cnrLoadRoster —— py `load_roster`：→ (nodes, 错误文案)。
func cnrLoadRoster(path string) (map[string]cnrValue, string) {
	if !cnrIsFile(path) {
		return nil, fmt.Sprintf("名册不在：%s（本机物件；建它 = 决议⑥ 的第 0 步）", path)
	}
	_, lines, err := cnrReadTextLines(path)
	if err != nil {
		return nil, fmt.Sprintf("名册读不了：%s", err)
	}
	sec := cnrSectionLines(lines, "nodes")
	if sec == nil {
		return nil, "名册里没有顶层键 nodes:"
	}
	all := cnrParseBlock(sec)
	out := map[string]cnrValue{}
	for k, v := range all {
		if m, ok := v.(map[string]cnrValue); ok && len(m) > 0 {
			out[k] = m
		}
	}
	if len(out) == 0 {
		return nil, "名册的 nodes: 解析不出任何机器（空转 = 假覆盖）"
	}
	return out, ""
}

// cnrModelsHost —— py E 的 `re.search` 手写复刻（见件头「正则面」）。
func cnrModelsHost(line string) (string, bool) {
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		if !cnrWordBoundary(rs, i) {
			continue
		}
		if i+4 > len(rs) || string(rs[i:i+4]) != "host" {
			continue
		}
		j := i + 4
		for j < len(rs) && cnrIsSpace(rs[j]) {
			j++
		}
		if j >= len(rs) || rs[j] != ':' {
			continue
		}
		j++
		for j < len(rs) && cnrIsSpace(rs[j]) {
			j++
		}
		k := j
		for k < len(rs) && cnrIsHostChar(rs[k]) {
			k++
		}
		if k > j {
			return string(rs[j:k]), true
		}
	}
	return "", false
}

func cnrIsWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func cnrWordBoundary(rs []rune, i int) bool {
	left := i > 0 && cnrIsWord(rs[i-1])
	right := i < len(rs) && cnrIsWord(rs[i])
	return left != right
}

func cnrIsHostChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
		r == '_' || r == '.' || r == '-'
}

// cnrSha256File —— py `sha256_file`（内容寻址真核用；1 MiB 分块 · 只读）。
func cnrSha256File(path string) string {
	h := sha256.New()
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 1<<20)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// cnrPlistMachine —— py `plist_machine`：→ (机器名, 错误文案)。只读。
func cnrPlistMachine(plistPath string) (string, string) {
	tokens, errMsg, _ := cnrPlistProgramArgv(plistPath)
	if errMsg != "" {
		return "", errMsg
	}
	if tokens == nil {
		return "", ""
	}
	if v, ok := cnrFindFlagValue(tokens, "--machine"); ok {
		return v, ""
	}
	for i, a := range tokens { // 兼容旧口径：任何以 machine 收尾的旗标
		if cnrHasSuffix(a, "machine") && i+1 < len(tokens) {
			return tokens[i+1], ""
		}
	}
	return "", ""
}

// cnrDeriveLocalNames —— py `derive_local_names`：→ (本机节点名集合, checks)。
func cnrDeriveLocalNames(fleetPath, plistPath string) (map[string]bool, []string) {
	names := map[string]bool{}
	checks := []string{}
	var sec []string
	if _, lines, err := cnrReadTextLines(fleetPath); err == nil {
		sec = cnrSectionLines(lines, "fleet")
	}
	if sec != nil {
		for k, v := range cnrParseBlock(sec) {
			if m, ok := v.(map[string]cnrValue); ok {
				if cnrLocalHosts[cnrStripQuotes(cnrToString(m["host"]))] {
					names[k] = true
				}
			}
		}
	}
	if pm, _ := cnrPlistMachine(plistPath); pm != "" {
		names[pm] = true
	}
	idStr := "**读不到**"
	if len(names) > 0 {
		ks := make([]string, 0, len(names))
		for k := range names {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		idStr = cnrJoinStr(ks, " · ")
	}
	checks = append(checks, fmt.Sprintf("结论层（read_layer=local）本机身份 = %s（fleet: 回环 host ∪ 本机 plist 机器名）", idStr))
	return names, checks
}

// cnrParseBlockOrdered —— 与 cnrParseBlock 同解析，但**另返回顶层键的出现序**（py dict 保序）。
func cnrParseBlockOrdered(lines []string) ([]string, map[string]cnrValue) {
	out := map[string]cnrValue{}
	var order []string
	seen := map[string]bool{}
	set := func(k string, v cnrValue) {
		if !seen[k] {
			seen[k] = true
			order = append(order, k)
		}
		out[k] = v
	}
	i := 0
	for i < len(lines) {
		raw := lines[i]
		if cnrPyStrip(raw) == "" {
			i++
			continue
		}
		ind := len(raw) - len(cnrLstripSpaces(raw))
		s := cnrPyStrip(raw)
		if len(s) > 0 && s[0] == '-' {
			i++
			continue
		}
		g1, val, ok := cnrKVMatch(s)
		if !ok {
			i++
			continue
		}
		key := cnrUnquote(g1)
		if val == "" {
			j := i + 1
			var sub []string
			for j < len(lines) {
				l2 := lines[j]
				if cnrPyStrip(l2) == "" {
					j++
					continue
				}
				if len(l2)-len(cnrLstripSpaces(l2)) <= ind {
					break
				}
				sub = append(sub, l2)
				j++
			}
			subStartDash := len(sub) > 0 && len(cnrPyStrip(sub[0])) > 0 && cnrPyStrip(sub[0])[0] == '-'
			if subStartDash {
				set(key, cnrParseSeq(sub))
			} else {
				set(key, cnrParseBlock(sub))
			}
			i = j
			continue
		}
		if len(val) > 0 && val[0] == '{' {
			set(key, cnrParseFlow(val))
		} else {
			set(key, cnrUnquote(val))
		}
		i++
	}
	return order, out
}

// cnrModelEntry —— py `models_entries` 的一条 (模型名, 条目dict)。
type cnrModelEntry struct {
	name string
	ent  map[string]cnrValue
}

// cnrModelsEntries —— py `models_entries`：把 models: 段展平成条目列表（flow / 块式都收 · 保序）。
func cnrModelsEntries(lines []string) []cnrModelEntry {
	sec := cnrSectionLines(lines, "models")
	if sec == nil {
		return nil
	}
	out := []cnrModelEntry{}
	order, m := cnrParseBlockOrdered(sec)
	for _, name := range order {
		v := m[name]
		if lst, ok := v.([]cnrValue); ok {
			for _, e := range lst {
				if d, ok := e.(map[string]cnrValue); ok {
					out = append(out, cnrModelEntry{name, d})
				}
			}
		} else if d, ok := v.(map[string]cnrValue); ok {
			out = append(out, cnrModelEntry{name, d})
		}
	}
	return out
}

// cnrCheckModelsVerified —— py `check_models_verified`（判据 G）。
func cnrCheckModelsVerified(lines []string, nodes map[string]cnrValue, localNames map[string]bool, verifySha bool) ([]string, []string) {
	fails := []string{}
	checks := []string{}
	ents := cnrModelsEntries(lines)
	if len(ents) == 0 {
		checks = append(checks, "G models: 段解析不出条目 ⇒ 跳过（E 仍核 host）")
		return fails, checks
	}
	nTrue, nRep, nOk := 0, 0, 0
	nNoAnchor := 0 // ★ 降级（GAP-20261003-21）：无有效锚的 true —— **只报告 · 不进 rc**
	for _, me := range ents {
		e := me.ent
		raw := cnrLower(cnrStripQuotes(cnrPyStrip(cnrToString(e["verified"]))))
		if raw == "" {
			continue
		}
		host := cnrStripQuotes(cnrToString(e["host"]))
		hostDisp := host
		if hostDisp == "" {
			hostDisp = "?"
		}
		tag := fmt.Sprintf("%s（host=%s）", me.name, hostDisp)
		if !cnrInStrings(raw, cnrVerifiedValues) {
			fails = append(fails, fmt.Sprintf("G `%s` 的 verified 取值 `%s` 不在闭集 %s",
				tag, raw, cnrJoinStr(cnrVerifiedValues, "/")))
			continue
		}
		if raw == "false" {
			continue
		}
		if raw == "true" {
			nTrue++
			anchor := ""
			for _, f := range cnrAnchorFields {
				if s := cnrStripQuotes(cnrPyStrip(cnrToString(e[f]))); s != "" {
					anchor = s
					break
				}
			}
			if anchor == "" {
				nNoAnchor++
				// ★ 降级（GAP-20261003-21）：消费者 schema 是 bool ⇒ 该锚恒缺、原 rc=1 恒红且修不了
				//   ⇒ 改判**只报告**（逐条列名，让「无锚的 true」可见，但不作门红）。
				checks = append(checks, fmt.Sprintf("G（报告·降级 GAP-20261003-21）`%s` 写 verified: true（实测级）却**没有内容锚**"+
					"（%s 之一，64 位十六进制）—— 只报告 · 不进 rc", tag, cnrJoinStr(cnrAnchorFields, "/")))
				continue
			}
			if !cnrAnchorRe.MatchString(anchor) {
				nNoAnchor++
				checks = append(checks, fmt.Sprintf("G（报告·降级 GAP-20261003-21）`%s` 的锚不是 64 位十六进制：%s —— 只报告 · 不进 rc", tag, cnrRepr(anchor)))
				continue
			}
			if localNames[host] {
				fpath := cnrStripQuotes(cnrPyStrip(cnrToString(e["file"])))
				if !verifySha {
					checks = append(checks, fmt.Sprintf("G `%s` 锚在且格式对；本机未复算（开 --verify-sha 即可真核）", tag))
				} else if fpath == "" {
					checks = append(checks, fmt.Sprintf("G `%s` 锚在且格式对；条目无 file: ⇒ 本机无法复算，记**声明级**", tag))
				} else if !cnrIsFile(fpath) {
					checks = append(checks, fmt.Sprintf("G `%s` 本机读不到件（%s）⇒ 锚记**声明级**，未复算", tag, fpath))
				} else {
					got := cnrSha256File(fpath)
					la := cnrLower(anchor)
					if got != la {
						fails = append(fails, fmt.Sprintf("G `%s` 的 sha256 与盘上件不符：条目=%s… 盘上=%s…",
							tag, cnrPrefix(la, 12), cnrPrefix(got, 12)))
					} else {
						nOk++
						checks = append(checks, fmt.Sprintf("G `%s` 锚**本机现算逐字相符**（%s…）", tag, cnrPrefix(anchor, 12)))
					}
				}
			} else {
				checks = append(checks, fmt.Sprintf("G `%s` 锚在且格式对；host 非本机 ⇒ 本机**无法复算**，记**声明级**", tag))
			}
		} else if raw == "reported" {
			nRep++
			where := cnrStripQuotes(cnrPyStrip(cnrToString(e["verified_where"])))
			if where == "" {
				fails = append(fails, fmt.Sprintf("G `%s` 写 verified: reported（声明级）却缺 verified_where（哪台自报）", tag))
			} else if _, ok := nodes[where]; !ok {
				fails = append(fails, fmt.Sprintf("G `%s` 的 verified_where `%s` 不在名册", tag, where))
			} else {
				checks = append(checks, fmt.Sprintf("G `%s` 声明级（自报机=%s）", tag, where))
			}
		}
	}
	checks = append([]string{fmt.Sprintf("G models: verified=true %d 条 · reported %d 条 · 本机现算相符 %d 条 · "+
		"**无有效锚的 true %d 处（降级只报告 · 不进 rc）**",
		nTrue, nRep, nOk, nNoAnchor)}, checks...)
	return fails, checks
}

// cnrCheckFleet —— py `check_fleet`：→ (fails, checks, hardErr)。
func cnrCheckFleet(fleetPath string, nodes map[string]cnrValue, localNames map[string]bool, verifySha bool) ([]string, []string, string) {
	var fails, checks []string
	if !cnrIsFile(fleetPath) {
		return []string{fmt.Sprintf("fleet.yaml 不在：%s", fleetPath)}, checks, ""
	}
	raw, lines, err := cnrReadTextLines(fleetPath)
	if err != nil {
		return nil, nil, fmt.Sprintf("fleet.yaml 读不了：%s", err)
	}

	// B. 两段各自声明「我是投影源」——看 **raw（含注释）**。
	for _, key := range []string{"fleet", "updates"} {
		s, e, ok := cnrSectionRange(lines, key)
		if !ok {
			fails = append(fails, fmt.Sprintf("fleet.yaml 里没有 %s: 段", key))
			continue
		}
		decl := false
		for _, l := range raw[s:e] {
			if cnrContains(l, "~/.zerg/contexts/") {
				decl = true
				break
			}
		}
		mark := "有"
		if !decl {
			mark = "**缺**"
		}
		checks = append(checks, fmt.Sprintf("B %s: 段投影源声明：%s", key, mark))
		if !decl {
			fails = append(fails, fmt.Sprintf(
				"%s: 段没有投影源声明（段内应出现真源路径 ~/.zerg/contexts/…）——「建而不收」就是不写这句", key))
		}
	}

	// C. fleet: 键集（两向）＋ 每台 host/port/os 逐字。
	if s, e, ok := cnrSectionRange(lines, "fleet"); ok {
		all := cnrParseBlock(lines[s:e])
		fleet := map[string]cnrValue{}
		for k, v := range all {
			if m, ok := v.(map[string]cnrValue); ok {
				fleet[k] = m
			}
		}
		onlyRoster := cnrSortedDiff(nodes, fleet)
		onlyFleet := cnrSortedDiff(fleet, nodes)
		checks = append(checks, fmt.Sprintf("C fleet: 键 %d 个 · 名册节点 %d 个", len(fleet), len(nodes)))
		for _, k := range onlyFleet {
			fails = append(fails, fmt.Sprintf("C fleet: 里的 `%s` 不在名册（子端名册没登记它）", k))
		}
		for _, k := range onlyRoster {
			fails = append(fails, fmt.Sprintf("C 名册里的 `%s` 不在 fleet: 段（名册有、投影没有）", k))
		}
		for _, name := range cnrSortedKeys(fleet) {
			if _, ok := nodes[name]; !ok {
				continue
			}
			ent := fleet[name].(map[string]cnrValue)
			nEnt := nodes[name].(map[string]cnrValue)
			for _, f := range []string{"host", "port", "os"} {
				want := cnrStripQuotes(cnrToString(nEnt[f]))
				got := cnrStripQuotes(cnrToString(ent[f]))
				if got != want {
					fails = append(fails, fmt.Sprintf("C `%s` 的 %s 漂了：fleet=%s 名册=%s",
						name, f, cnrRepr(got), cnrRepr(want)))
				}
			}
		}
	}

	// D. updates.nodes 逐字段。
	if s, e, ok := cnrSectionRange(lines, "updates"); ok {
		ups := cnrParseBlock(lines[s:e])
		var items []cnrValue
		if v, ok := ups["nodes"]; ok {
			if lst, ok := v.([]cnrValue); ok {
				items = lst
			}
		}
		checks = append(checks, fmt.Sprintf("D updates.nodes 条目 %d 个", len(items)))
		for _, itv := range items {
			it, ok := itv.(map[string]cnrValue)
			if !ok {
				continue
			}
			name := cnrStripQuotes(cnrToString(it["name"]))
			if name == "" {
				fails = append(fails, "D updates.nodes 有一项没有 name")
				continue
			}
			if _, ok := nodes[name]; !ok {
				fails = append(fails, fmt.Sprintf("D updates.nodes 里的 `%s` 不在名册", name))
				continue
			}
			nEnt := nodes[name].(map[string]cnrValue)
			for _, f := range []string{"ssh", "root", "prefix", "api", "components"} {
				want := cnrStripQuotes(cnrToString(nEnt[f]))
				got := cnrStripQuotes(cnrToString(it[f]))
				if got != want {
					fails = append(fails, fmt.Sprintf("D `%s` 的 %s 漂了：updates.nodes=%s 名册=%s",
						name, f, cnrRepr(got), cnrRepr(want)))
				}
			}
		}
	}

	// E. models: 里的 host 必须在名册。
	if s, e, ok := cnrSectionRange(lines, "models"); ok {
		hosts := map[string]bool{}
		for _, l := range lines[s:e] {
			if h, ok := cnrModelsHost(l); ok {
				hosts[h] = true
			}
		}
		checks = append(checks, fmt.Sprintf("E models: 段出现 %d 个 host", len(hosts)))
		var extra []string
		for h := range hosts {
			if _, ok := nodes[h]; !ok {
				extra = append(extra, h)
			}
		}
		sort.Strings(extra)
		for _, h := range extra {
			fails = append(fails, fmt.Sprintf("E models: 里的 host `%s` 不在名册（模型驻留在哪台必须有条目）", h))
		}
	}

	// G. models: 段的 `verified` 闭集 + 「实测级必须有可核内容锚」（GAP-20260927-167/-171）
	fg, cg := cnrCheckModelsVerified(lines, nodes, localNames, verifySha)
	fails = append(fails, fg...)
	checks = append(checks, cg...)
	return fails, checks, ""
}

// ══════════════════════════ 判据 I：host ↔ 路径用户名（GAP-20260927-06）═════════════════════════════

// cnrSshUser —— ssh 串 `user@host` 的用户段（无 `@` / 空 ⇒ ""）。**不猜**：只读声明值。
func cnrSshUser(ssh string) string {
	ssh = cnrPyStrip(cnrStripQuotes(ssh))
	at := cnrLastIndexByte(ssh, '@')
	if at <= 0 {
		return ""
	}
	return ssh[:at]
}

// cnrAccountSets —— 各机**声明**的账户名集合（源 = 名册 `nodes.<名>.ssh` ＋ `updates.nodes[].ssh` 用户段）。
func cnrAccountSets(nodes map[string]cnrValue, fleetLines []string) map[string]map[string]bool {
	acc := map[string]map[string]bool{}
	add := func(name, ssh string) {
		if name == "" {
			return
		}
		u := cnrSshUser(ssh)
		if u == "" {
			return
		}
		if acc[name] == nil {
			acc[name] = map[string]bool{}
		}
		acc[name][u] = true
	}
	for name, v := range nodes {
		if m, ok := v.(map[string]cnrValue); ok {
			add(name, cnrToString(m["ssh"]))
		}
	}
	if s, e, ok := cnrSectionRange(fleetLines, "updates"); ok {
		ups := cnrParseBlock(fleetLines[s:e])
		if v, ok := ups["nodes"]; ok {
			if lst, ok := v.([]cnrValue); ok {
				for _, itv := range lst {
					if it, ok := itv.(map[string]cnrValue); ok {
						add(cnrStripQuotes(cnrToString(it["name"])), cnrToString(it["ssh"]))
					}
				}
			}
		}
	}
	return acc
}

// cnrPathUsernames —— 从一段路径值里抽账户名（`/Users/<u>/…` 或 `/home/<u>/…`，可多处）。
func cnrPathUsernames(val string) []string {
	v := cnrPyStrip(cnrStripQuotes(val))
	var out []string
	for _, pre := range []string{"/Users/", "/home/"} {
		start := 0
		for {
			i := cnrIndexFrom(v, pre, start)
			if i < 0 {
				break
			}
			rest := v[i+len(pre):]
			j := cnrIndexByte(rest, '/')
			if j < 0 {
				j = len(rest)
			}
			if u := rest[:j]; u != "" {
				out = append(out, u)
			}
			start = i + len(pre)
		}
	}
	return out
}

// cnrIndexFrom —— `strings.Index(s[from:], sub)` 的偏移还原（找不到 ⇒ -1）。
func cnrIndexFrom(s, sub string, from int) int {
	if from > len(s) {
		return -1
	}
	i := cnrIndexStr(s[from:], sub)
	if i < 0 {
		return -1
	}
	return from + i
}

// cnrSortedSet —— map[string]bool → 排序后的键切片（唯一进输出的 dict 序来源）。
func cnrSortedSet(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// cnrCheckHostPathUsername —— 判据 I（GAP-20260927-06）：`models:` 每条里路径字段
// （`file` / `mmproj` / `cmd`）里的账户名必须 ∈ 该机**声明的**账户集
// （源 = 名册 `nodes.<名>.ssh` 与 `updates.nodes[].ssh` 的用户段）。
//
//	✦ ✗ 不按命名规律猜（`mini-node-1` 的账户是 `a1`，与机器名无关）；
//	✦ `host` 非名册键 ⇒ 交给 E 判；该机未声明 `ssh:`（账户集空）⇒ 跳过该条（报告跳过数）。
func cnrCheckHostPathUsername(fleetLines []string, nodes map[string]cnrValue) ([]string, []string) {
	fails := []string{}
	checks := []string{}
	acc := cnrAccountSets(nodes, fleetLines)
	ents := cnrModelsEntries(fleetLines)
	if len(ents) == 0 {
		checks = append(checks, "I host↔路径用户名：models: 段解析不出条目 ⇒ 跳过")
		return fails, checks
	}
	nChecked, nSkip := 0, 0
	for _, me := range ents {
		e := me.ent
		host := cnrStripQuotes(cnrPyStrip(cnrToString(e["host"])))
		if host == "" {
			continue
		}
		accs := acc[host]
		if len(accs) == 0 {
			nSkip++
			continue
		}
		for _, f := range []string{"file", "mmproj", "cmd"} {
			v, ok := e[f]
			if !ok {
				continue
			}
			var vals []string
			switch x := v.(type) {
			case []cnrValue:
				for _, it := range x {
					vals = append(vals, cnrToString(it))
				}
			default:
				vals = append(vals, cnrToString(v))
			}
			for _, val := range vals {
				for _, u := range cnrPathUsernames(val) {
					nChecked++
					if !accs[u] {
						fails = append(fails, fmt.Sprintf(
							"I `%s`（host=%s）路径里的账户名 `%s` 不在该机声明账户集 {%s}"+
								"（源：名册/updates 的 ssh 用户段）—— 路径与机器对不上",
							me.name, host, u, cnrJoinStr(cnrSortedSet(accs), "/")))
					}
				}
			}
		}
	}
	checks = append(checks, fmt.Sprintf("I host↔路径用户名：核对 %d 处账户名 · 跳过 %d 条（host 未声明 ssh: 或非名册键）", nChecked, nSkip))
	return fails, checks
}

// cnrRunningAgentdMachine —— py `running_agentd_machine`（只读 ps · 不碰进程）。
func cnrRunningAgentdMachine() (string, bool) {
	out, err := exec.Command("ps", "-Ao", "command=").Output()
	if err != nil {
		return "", false
	}
	for _, line := range cnrSplitStr(string(out), "\n") {
		if !cnrContains(line, "zerg-agentd") || cnrContains(line, "grep") {
			continue
		}
		parts := cnrFields(line)
		for i, p := range parts {
			if cnrHasSuffix(p, "machine") && i+1 < len(parts) {
				return parts[i+1], true
			}
		}
	}
	return "", false
}

// cnrPlistArgs —— py `plistlib.load(f).get("ProgramArguments") or []`（XML plist）。
func cnrPlistArgs(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	var args []string
	inPA := false
	pendingKey := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "key":
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return nil, err
				}
				pendingKey = s
			case "array":
				if pendingKey == "ProgramArguments" {
					inPA = true
				}
			case "string":
				if inPA {
					var s string
					if err := dec.DecodeElement(&s, &t); err != nil {
						return nil, err
					}
					args = append(args, s)
				}
			}
		case xml.EndElement:
			if t.Name.Local == "array" {
				inPA = false
			}
		}
	}
	return args, nil
}

// cnrCheckSubend —— py `check_subend`（F · **本机**）。
func cnrCheckSubend(plistPath string, nodes map[string]cnrValue) ([]string, []string) {
	var fails, checks []string
	val := ""
	hasVal := false
	if !cnrIsFile(plistPath) {
		checks = append(checks, fmt.Sprintf("F 本机盘上 plist 不在（%s）⇒ 跳过该处", plistPath))
	} else {
		v, perr := cnrPlistMachine(plistPath)
		if perr != "" {
			fails = append(fails, fmt.Sprintf("F %s", perr))
		} else {
			val, hasVal = v, true
		}
	}
	if hasVal {
		checks = append(checks, fmt.Sprintf("F 本机盘上 plist 机器名 = %s", val))
		if _, ok := nodes[val]; !ok {
			fails = append(fails, fmt.Sprintf("F plist 的机器名 `%s` 不在名册（改机器名要先改名册）", val))
		}
	}
	run, ok := cnrRunningAgentdMachine()
	if !ok {
		checks = append(checks, "F 本机在跑的 zerg-agentd：没找到（跳过该处）")
	} else {
		checks = append(checks, fmt.Sprintf("F 本机在跑的 zerg-agentd 机器名 = %s", run))
		if _, ok := nodes[run]; !ok {
			fails = append(fails, fmt.Sprintf("F 本机在跑的 zerg-agentd 机器名 `%s` 不在名册（子端自报和真源对不上）", run))
		}
	}
	return fails, checks
}

// ══════════════════════════ 判据 H：拨号地址（`--controller`）由名册可推导 ══════════════════════════

// cnrShlexSplit —— py `shlex.split(script)` 的 posix 子集（空白分隔 · 引号剥除 · 反斜杠转义）。
func cnrShlexSplit(s string) ([]string, error) {
	var out []string
	var buf []rune
	inWord := false
	quote := rune(0)
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		ch := rs[i]
		if quote == '\'' {
			if ch == '\'' {
				quote = 0
			} else {
				buf = append(buf, ch)
			}
			continue
		}
		if quote == '"' {
			if ch == '"' {
				quote = 0
				continue
			}
			if ch == '\\' && i+1 < len(rs) {
				nx := rs[i+1]
				if nx == '"' || nx == '\\' || nx == '$' || nx == '`' {
					buf = append(buf, nx)
					i++
					continue
				}
				if nx == '\n' {
					i++
					continue
				}
				buf = append(buf, ch)
				continue
			}
			buf = append(buf, ch)
			continue
		}
		switch {
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			if inWord {
				out = append(out, string(buf))
				buf = nil
				inWord = false
			}
		case ch == '\'' || ch == '"':
			quote = ch
			inWord = true
		case ch == '\\' && i+1 < len(rs):
			i++
			buf = append(buf, rs[i])
			inWord = true
		default:
			buf = append(buf, ch)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("No closing quotation")
	}
	if inWord {
		out = append(out, string(buf))
	}
	return out, nil
}

// cnrBaseName —— py `os.path.basename`（末段）。
func cnrBaseName(p string) string {
	if i := cnrLastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// cnrPlistProgramArgv —— py `plist_program_argv`：→ (tokens, 错误文案, 件在否)。
//
//	★ 两种形态都收：① 直给 argv 数组 —— 原样返回；② `/bin/sh -c "<脚本串>"` 壳 —— shlex 切脚本串。
func cnrPlistProgramArgv(plistPath string) ([]string, string, bool) {
	if !cnrIsFile(plistPath) {
		return nil, "", false
	}
	args, err := cnrPlistArgs(plistPath)
	if err != nil {
		return nil, fmt.Sprintf("plist 读不了（%s）：%s", plistPath, err), false
	}
	if len(args) >= 3 {
		base := cnrBaseName(args[0])
		if (base == "sh" || base == "bash" || base == "zsh") && args[1] == "-c" {
			toks, serr := cnrShlexSplit(args[2])
			if serr != nil {
				return nil, fmt.Sprintf("plist 脚本串切不开（%s）：%s", plistPath, serr), false
			}
			return toks, "", true
		}
	}
	return args, "", true
}

// cnrFindFlagValue —— py `find_flag_value`：命令行 `--flag <值>` / `--flag=<值>` 的取值。
func cnrFindFlagValue(tokens []string, flag string) (string, bool) {
	for i, a := range tokens {
		if a == flag && i+1 < len(tokens) {
			return tokens[i+1], true
		}
		if cnrHasPrefix(a, flag+"=") {
			return a[len(flag)+1:], true
		}
	}
	return "", false
}

// cnrPlistController —— py `plist_controller`：→ (有效值, 变量名, 错误文案, 有无值)。
//
//	`${VAR:-默认}` 变量形态 ⇒ 取**默认值**（= 未设该变量时真正拨出去的地址）；直给字面值 ⇒ 原样。
func cnrPlistController(plistPath string) (string, string, string, bool) {
	tokens, errMsg, _ := cnrPlistProgramArgv(plistPath)
	if errMsg != "" {
		return "", "", errMsg, false
	}
	if tokens == nil {
		return "", "", "", false
	}
	raw, ok := cnrFindFlagValue(tokens, "--controller")
	if !ok {
		return "", "", "", false
	}
	raw = cnrPyStrip(raw)
	if m := cnrCtrlVarRe.FindStringSubmatch(raw); m != nil {
		return cnrPyStrip(m[2]), m[1], "", true
	}
	return raw, "", "", true
}

// cnrTrailingPort —— py `re.search(r":(\d+)\s*$", u)` 的手写复刻。
func cnrTrailingPort(u string) (int, bool) {
	t := cnrPyRstrip(u)
	rs := []rune(t)
	i := len(rs)
	for i > 0 && rs[i-1] >= '0' && rs[i-1] <= '9' {
		i--
	}
	if i == len(rs) || i == 0 || rs[i-1] != ':' {
		return 0, false
	}
	n, err := strconv.Atoi(string(rs[i:]))
	if err != nil {
		return 0, false
	}
	return n, true
}

// cnrRosterCorePort —— py `roster_core_port`：名册顶层 `core:` 的端口（读不到 ⇒ false）。
func cnrRosterCorePort(lines []string) (int, bool) {
	for _, l := range lines {
		if !cnrHasPrefix(l, "core") {
			continue
		}
		rest := cnrSkipSpace(l[4:])
		if len(rest) == 0 || rest[0] != ':' {
			continue
		}
		rest = cnrSkipSpace(rest[1:])
		rs := []rune(rest)
		i := 0
		for i < len(rs) && !cnrIsSpace(rs[i]) {
			i++
		}
		if i == 0 {
			continue
		}
		if cnrPyStrip(string(rs[i:])) != "" {
			continue
		}
		return cnrTrailingPort(cnrUnquote(string(rs[:i])))
	}
	return 0, false
}

// cnrDerivableControllerHosts —— py `derivable_controller_hosts`：回环 ∪ 名册每台的 host。
func cnrDerivableControllerHosts(nodes map[string]cnrValue) map[string]bool {
	hosts := map[string]bool{}
	for h := range cnrLocalHosts {
		hosts[h] = true
	}
	for _, v := range nodes {
		if m, ok := v.(map[string]cnrValue); ok {
			h := cnrStripQuotes(cnrPyStrip(cnrToString(m["host"])))
			if h != "" {
				hosts[h] = true
			}
		}
	}
	return hosts
}

// cnrCheckController —— py `check_controller`（判据 H）。
func cnrCheckController(plistPaths []string, nodes map[string]cnrValue, rosterLines []string) ([]string, []string) {
	fails := []string{}
	checks := []string{}
	corePort, hasCore := cnrRosterCorePort(rosterLines)
	hosts := cnrDerivableControllerHosts(nodes)
	hs := make([]string, 0, len(hosts))
	for h := range hosts {
		hs = append(hs, h)
	}
	sort.Strings(hs)
	coreStr := "**读不到**"
	if hasCore {
		coreStr = strconv.Itoa(corePort)
	}
	checks = append(checks, fmt.Sprintf("H 拨号可推导集（回环 ∪ 名册 host）= %s · 名册 core 端口 = %s",
		cnrJoinStr(hs, " · "), coreStr))
	for _, p := range plistPaths {
		if !cnrIsFile(p) {
			checks = append(checks, fmt.Sprintf("H 拨号对账：plist 不在（%s）⇒ 跳过该件", p))
			continue
		}
		val, variable, errMsg, present := cnrPlistController(p)
		if errMsg != "" {
			fails = append(fails, "H "+errMsg)
			continue
		}
		if !present {
			checks = append(checks, fmt.Sprintf("H 拨号对账：%s 无 `--controller` ⇒ 跳过", p))
			continue
		}
		shown := p
		if variable != "" {
			shown = fmt.Sprintf("%s（变量 ${%s:-…} 的默认值）", p, variable)
		}
		m := cnrCtrlURLRe.FindStringSubmatch(val)
		if m == nil {
			fails = append(fails, fmt.Sprintf("H %s 的 `--controller` 不是合法 http(s) 地址：%s", shown, cnrRepr(val)))
			continue
		}
		host, port := m[2], m[3]
		var why []string
		if !hosts[host] {
			why = append(why, fmt.Sprintf("host=%s 不在名册可推导集", host))
		}
		if hasCore {
			if port == "" {
				why = append(why, fmt.Sprintf("缺端口（名册 core 端口=%d）", corePort))
			} else if pn, perr := strconv.Atoi(port); perr != nil || pn != corePort {
				why = append(why, fmt.Sprintf("port=%s ≠ 名册 core 端口=%d", port, corePort))
			}
		}
		if len(why) > 0 {
			fails = append(fails, fmt.Sprintf("H %s 的拨号地址推不出来：现值 %s —— %s（改 plist 或先把该地址登记进名册 nodes）",
				shown, cnrRepr(val), cnrJoinStr(why, "；")))
		} else {
			checks = append(checks, fmt.Sprintf("H 拨号对账 %s ⇒ 可推导（值 %s）", shown, cnrRepr(val)))
		}
	}
	return fails, checks
}

// cnrRunChecks —— py `run_checks`。
func cnrRunChecks(stdout io.Writer, roster, fleet, plist string, verbose bool, verifySha bool, ctrlPlists []string, ctrlCheck bool, pathUserCheck bool) int {
	nodes, errMsg := cnrLoadRoster(roster)
	if errMsg != "" {
		fmt.Fprintf(stdout, "⛔ 不给结论：%s\n", errMsg)
		return 2
	}
	fmt.Fprintf(stdout, "真源名册 = %s（节点 %d 个：%s）\n", roster, len(nodes),
		cnrJoinStr(cnrSortedKeys(nodes), " · "))
	localNames, lchecks := cnrDeriveLocalNames(fleet, plist)
	fails, checks, hard := cnrCheckFleet(fleet, nodes, localNames, verifySha)
	if hard != "" {
		fmt.Fprintf(stdout, "⛔ 不给结论：%s\n", hard)
		return 2
	}
	f2, c2 := cnrCheckSubend(plist, nodes)
	fails = append(fails, f2...)
	// H 拨号地址（`--controller`）由名册可推导（GAP-20260927-03）—— 负控可哑化（ctrlCheck=false）。
	var c3 []string
	if ctrlCheck {
		_, rosterLines, rerr := cnrReadTextLines(roster)
		if rerr != nil {
			rosterLines = nil
		}
		plistList := ctrlPlists
		if plistList == nil {
			plistList = cnrControllerPlists()
		}
		f3, c3x := cnrCheckController(plistList, nodes, rosterLines)
		fails = append(fails, f3...)
		c3 = c3x
	} else {
		c3 = []string{"H 拨号对账：断言哑化（ctrl_check=False · 负控用）⇒ 不报"}
	}
	// I host↔路径用户名（GAP-20260927-06）—— 负控可哑化（pathUserCheck=false）。
	var c4 []string
	if pathUserCheck {
		_, flines, ferr := cnrReadTextLines(fleet)
		if ferr != nil {
			flines = nil
		}
		f4, c4x := cnrCheckHostPathUsername(flines, nodes)
		fails = append(fails, f4...)
		c4 = c4x
	} else {
		c4 = []string{"I host↔路径用户名：断言哑化（path_user_check=False · 负控用）⇒ 不报"}
	}
	all := []string{}
	all = append(all, lchecks...)
	all = append(all, checks...)
	all = append(all, c2...)
	all = append(all, c3...)
	all = append(all, c4...)
	checks = all
	uncovered := []string{}
	for k := range nodes {
		if !localNames[k] {
			uncovered = append(uncovered, k)
		}
	}
	sort.Strings(uncovered)
	localStr := "**本机身份读不到**"
	if len(localNames) > 0 {
		ks := make([]string, 0, len(localNames))
		for k := range localNames {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		localStr = cnrJoinStr(ks, " · ")
	}
	uncovStr := "无"
	if len(uncovered) > 0 {
		uncovStr = cnrJoinStr(uncovered, " · ")
	}
	if verbose {
		for _, c := range checks {
			fmt.Fprintf(stdout, "   · %s\n", c)
		}
		fmt.Fprintf(stdout, "   ↳ read_layer=local · verdict_scope=**本机**（%s）—— 本门只读本机盘上的"+
			"fleet.yaml / 名册 / plist / ps，**不用任何远端读数**\n", localStr)
		fmt.Fprintf(stdout, "   ↳ 本门**不覆盖**远端节点（本机读不到 ⇒ 本门不给结论）：%s"+
			"（要它们的结论，须由各机自己跑本门回答）\n", uncovStr)
	}
	if len(fails) > 0 {
		fmt.Fprintf(stdout, "✗ **本机层**与名册有漂移：%d 处（verdict_scope=本机；远端不在结论内）\n", len(fails))
		for _, f := range fails {
			fmt.Fprintf(stdout, "   ✗ %s\n", f)
		}
		return 1
	}
	fmt.Fprint(stdout, "✓ **本机层**（read_layer=local）与名册逐字一致：fleet: · updates.nodes · "+
		"models: host 各在册 · G verified 闭集（无锚 true 仅报告·GAP-20261003-21） · "+
		"本机子端名册（plist/本机在跑 agentd） · H 拨号地址由名册可推导 · "+
		"I host↔路径用户名（路径账户名 ∈ 该机声明 ssh 用户段）\n")
	fmt.Fprintf(stdout, "   ↳ verdict_scope=**本机**（%s）—— 本门**不覆盖**远端：%s\n", localStr, uncovStr)
	return 0
}

// ══════════════════════════ 自检（合成夹具 · 成对负控 · 不碰真目标）══════════════════════════

// cnrSelfTest —— py `self_test()`。
func cnrSelfTest(stdout, stderr io.Writer) int {
	bad := 0
	d, err := os.MkdirTemp("", "tmp")
	if err != nil {
		fmt.Fprintf(stdout, "自检跑不了：造不出临时目录（%v）\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(d) }() // A 类对齐：py `with TemporaryDirectory()` 有回收
	w := func(name, text string) string {
		p := cnrPyJoin(d, name)
		_ = os.WriteFile(p, []byte(text), 0o644)
		return p
	}
	roster := w("roster.yaml", cnrRosterFix)
	fleet := w("fleet.yaml", cnrFleetFix)
	plist := w("a.plist", cnrPlistFix)

	type kase struct {
		name                 string
		roster, fleet, plist string
		want                 int
		vsha                 bool
	}
	cases := []kase{
		{"全对 ⇒ 0", roster, fleet, plist, 0, false},
		{"fleet: 少一台 ⇒ 1", roster, w("f2.yaml",
			cnrReplaceAll(cnrFleetFix, "  Mr2109: { host: 127.0.0.1, port: 8100, os: macos }\n", "")), plist, 1, false},
		{"fleet: host 改一位 ⇒ 1", roster, w("f3.yaml",
			cnrReplaceAll(cnrFleetFix, "edge-node: { host: <worker-ip>", "edge-node: { host: <worker-ip>")), plist, 1, false},
		{"fleet: 投影源声明删掉 ⇒ 1（建而不收）", roster, w("f4.yaml",
			cnrReplaceN(cnrFleetFix, "  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n", "", 1)), plist, 1, false},
		{"updates.nodes 的 ssh 改 ⇒ 1", roster, w("f5.yaml",
			cnrReplaceAll(cnrFleetFix, "ssh: g01@<worker-ip>", "ssh: root@<worker-ip>")), plist, 1, false},
		{"models: host 未登记 ⇒ 1", roster, w("f6.yaml",
			cnrReplaceAll(cnrFleetFix, "m1: { host: edge-node", "m1: { host: ghost")), plist, 1, false},
		{"updates 段声明删掉 ⇒ 1", roster, w("f7.yaml",
			cnrReplaceN(cnrFleetFix, "  # 投影源：真源 = ~/.zerg/contexts/default.yaml 的 nodes:\n", "", 2)), plist, 1, false},
		{"plist 机器名未登记 ⇒ 1", roster, fleet, w("b.plist",
			cnrReplaceAll(cnrPlistFix, "<string>Mr2109</string>", "<string>ghost</string>")), 1, false},
		{"名册不在 ⇒ 2（不给结论）", cnrPyJoin(d, "nope.yaml"), fleet, plist, 2, false},
		{"名册 nodes 空 ⇒ 2", w("r2.yaml", "nodes:\n"), fleet, plist, 2, false},
	}

	// ── 判据 H：拨号地址由名册可推导（GAP-20260927-03；成对负控）──
	cBad := w("ctrl_bad.plist", cnrReplaceAll(cnrPlistCtrlVarFix, "__CTRL__", "http://<worker-host>:8580"))
	cOK := w("ctrl_ok.plist", cnrReplaceAll(cnrPlistCtrlVarFix, "__CTRL__", "http://<worker-ip>:8580"))
	cPort := w("ctrl_port.plist", cnrReplaceAll(cnrPlistCtrlLiteralFix, "__CTRL__", "http://<worker-ip>:9"))
	cases = append(cases,
		kase{"H 拨号 host 不在名册（变量默认值）⇒ 1", roster, fleet, plist, 1, false},
		kase{"H 拨号可推导（变量默认值）⇒ 0", roster, fleet, plist, 0, false},
		kase{"H 拨号端口 ≠ core 端口 ⇒ 1", roster, fleet, plist, 1, false},
		kase{"H 负控：断言哑化（同一 bad）⇒ 0（回不报）", roster, fleet, plist, 0, false})

	// ── 判据 G：verified 闭集 + 「实测级必须有可核内容锚」（GAP-…-167/-171 成对负控）──
	wa := w("weights.bin", "zerg-verified-fixture\n")
	ahash := cnrSha256File(wa)  // 本机件真 sha256
	bogus := cnrRepeat("a", 64) // 格式对、内容不对的锚
	nonexistent := "/nonexistent/none.gguf"
	gfix := func(vline, host, fpath string) string {
		s := cnrReplaceAll(cnrFleetVerifiedFix, "__HOST__", host)
		s = cnrReplaceAll(s, "__FILE__", fpath)
		return cnrReplaceAll(s, "__VERIFIED__", vline)
	}
	cases = append(cases,
		kase{"G true+锚+本机件相符+--verify-sha ⇒ 0", roster,
			w("g1.yaml", gfix("verified: true, sha256: "+ahash, "Mr2109", wa)), plist, 0, true},
		kase{"G true+锚+本机件不符+--verify-sha ⇒ 1", roster,
			w("g2.yaml", gfix("verified: true, sha256: "+bogus, "Mr2109", wa)), plist, 1, true},
		kase{"G true 无内容锚 ⇒ 0（降级：只报告·点名）", roster,
			w("g3.yaml", gfix("verified: true", "Mr2109", nonexistent)), plist, 0, false},
		kase{"G 锚格式非 64 位 ⇒ 0（降级：只报告）", roster,
			w("g4.yaml", gfix("verified: true, sha256: deadbeef", "Mr2109", nonexistent)), plist, 0, false},
		kase{"G true+锚+远端 host ⇒ 0（记声明级）", roster,
			w("g5.yaml", gfix("verified: true, sha256: "+bogus, "edge-node", nonexistent)), plist, 0, false},
		kase{"G reported 缺 verified_where ⇒ 1", roster,
			w("g6.yaml", gfix("verified: reported", "Mr2109", nonexistent)), plist, 1, false},
		kase{"G reported+verified_where ∈ 名册 ⇒ 0", roster,
			w("g7.yaml", gfix("verified: reported, verified_where: edge-node", "Mr2109", nonexistent)), plist, 0, false},
		kase{"G 闭集外取值 ⇒ 1", roster,
			w("g8.yaml", gfix("verified: maybe", "Mr2109", nonexistent)), plist, 1, false})

	// ── 判据 I：host ↔ 路径用户名（GAP-20260927-06；正控 / 红 / 跳过 / 哑化成对）──
	// 夹具名册 edge-node 的 ssh = g01@<worker-ip> ⇒ 账户集 {g01}：路径 /home/<user>/… 应 ✓、/home/other/… 应 ✗。
	iOK := w("i_ok.yaml", cnrReplaceAll(cnrFleetFix,
		"m1: { host: edge-node, backend: llama-server }",
		"m1: { host: edge-node, backend: llama-server, file: /home/<user>/models/m.gguf }"))
	iBad := w("i_bad.yaml", cnrReplaceAll(cnrFleetFix,
		"m1: { host: edge-node, backend: llama-server }",
		"m1: { host: edge-node, backend: llama-server, file: /home/other/models/m.gguf }"))
	iSkip := w("i_skip.yaml", cnrReplaceAll(cnrFleetFix,
		"m1: { host: edge-node, backend: llama-server }",
		"m1: { host: Mr2109, backend: llama-server, file: /Users/nobody/models/m.gguf }"))
	cases = append(cases,
		kase{"I 路径账户名 ∈ 该机声明集 ⇒ 0", roster, iOK, plist, 0, false},
		kase{"I 路径账户名 ∉ 该机声明集 ⇒ 1", roster, iBad, plist, 1, false},
		kase{"I host 未声明 ssh: ⇒ 跳过 ⇒ 0", roster, iSkip, plist, 0, false},
		kase{"I 负控：断言哑化（同一 bad）⇒ 0（回不报）", roster, iBad, plist, 0, false})

	hCtrl := map[string]struct {
		ctrl []string
		cchk bool
	}{
		"H 拨号 host 不在名册（变量默认值）⇒ 1":  {ctrl: []string{cBad}, cchk: true},
		"H 拨号可推导（变量默认值）⇒ 0":         {ctrl: []string{cOK}, cchk: true},
		"H 拨号端口 ≠ core 端口 ⇒ 1":      {ctrl: []string{cPort}, cchk: true},
		"H 负控：断言哑化（同一 bad）⇒ 0（回不报）": {ctrl: []string{cBad}, cchk: false},
	}
	// 判据 I 哑化（负控）：同一 bad，关掉 I 断言 ⇒ 回不报（rc=0）
	iMute := map[string]bool{"I 负控：断言哑化（同一 bad）⇒ 0（回不报）": true}
	for _, c := range cases {
		ptrs, cchk, puc := []string{}, true, true
		if v, ok := hCtrl[c.name]; ok {
			ptrs, cchk = v.ctrl, v.cchk
		}
		if iMute[c.name] {
			puc = false
		}
		rc := cnrRunChecks(stdout, c.roster, c.fleet, c.plist, false, c.vsha, ptrs, cchk, puc)
		mark := "✓"
		if rc != c.want {
			mark = "✗"
			bad++
		}
		fmt.Fprintf(stdout, "   %s %s 期望 rc=%d 实得 rc=%d\n", mark, cnrLjust(c.name, 48), c.want, rc)
	}

	// ── 判词层限成对负控：判词必须带 read_layer / verdict_scope 与「不覆盖远端」声明 ──
	var vbuf strings.Builder
	_ = cnrRunChecks(&vbuf, roster, fleet, plist, true, false, []string{}, true, true)
	vout := vbuf.String()
	for _, kv := range []struct {
		name string
		ok   bool
	}{
		{"判词含 read_layer=local", strings.Contains(vout, "read_layer=local")},
		{"判词含 verdict_scope=**本机**", strings.Contains(vout, "verdict_scope=**本机**")},
		{"判词含「不覆盖」远端声明", strings.Contains(vout, "不覆盖")},
	} {
		mark := "✓"
		res := "在"
		if !kv.ok {
			mark = "✗"
			res = "**缺**"
			bad++
		}
		fmt.Fprintf(stdout, "   %s %s %s\n", mark, cnrLjust(kv.name, 48), res)
	}
	concl := "全过"
	if bad != 0 {
		concl = "**不过**"
	}
	fmt.Fprintf(stdout, "自检结论：%s（%d 条，失败 %d）\n", concl, len(cases)+3, bad)
	if bad == 0 {
		return 0
	}
	return 2
}

// ══════════════════════════ 缺口 Q-176：本门无机器读面 ⇒ 给了 `--json` 不许沉默 ══════════════════════════

// cnrJSONFields —— 本门机器读面字段表（**唯一真源**；空表 ⇒ 印「无」）。
var cnrJSONFields = []string{}

// cnrRefuseJSON —— py `refuse_json_without_machine_face`。
func cnrRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	var hit []string
	for _, x := range argv {
		if x == "--json" || cnrHasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	quoted := make([]string, len(hit))
	for i, h := range hit {
		quoted[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", cnrJoinStr(quoted, " · "))
	fs := "无（本门只出人读面）"
	if len(cnrJSONFields) > 0 {
		qs := make([]string, len(cnrJSONFields))
		for i, f := range cnrJSONFields {
			qs[i] = "`" + f + "`"
		}
		fs = cnrJoinStr(qs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fs)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2, true
}

// ══════════════════════════ argparse 复刻（子集）══════════════════════════

type cnrArgs struct {
	roster     string
	fleet      string
	plist      string
	selfTest   bool
	listRules  bool
	verifySha  bool
	ctrlPlists []string
}

var cnrOptSpecs = []struct {
	name  string
	takes bool
}{
	{"--roster", true},
	{"--fleet", true},
	{"--plist", true},
	{"--self-test", false},
	{"--list-rules", false},
	{"--verify-sha", false},
	{"--ctrl-plist", true},
	{"--help", false},
}

// cnrMatchOpt —— argparse 长旗标精确 / 唯一前缀匹配。
func cnrMatchOpt(name string) (string, bool, bool) {
	for _, s := range cnrOptSpecs {
		if s.name == name {
			return s.name, s.takes, true
		}
	}
	var cands []string
	for _, s := range cnrOptSpecs {
		if len(name) < len(s.name) && cnrHasPrefix(s.name, name) {
			cands = append(cands, s.name)
		}
	}
	if len(cands) == 1 {
		for _, s := range cnrOptSpecs {
			if s.name == cands[0] {
				return s.name, s.takes, true
			}
		}
	}
	return "", false, false
}

// cnrParseArgs —— 复刻六键 argparse 面。rc<0 ⇒ 成功；rc=0 ⇒ 打帮助；rc=2 ⇒ 用法错。
func cnrParseArgs(argv []string) (cnrArgs, int, string) {
	a := cnrArgs{roster: cnrDefaultRoster(), fleet: cnrDefaultFleet(), plist: cnrDefaultPlist()}
	var unknown []string
	i := 0
	for i < len(argv) {
		arg := argv[i]
		if arg == "--" {
			unknown = append(unknown, arg)
			unknown = append(unknown, argv[i+1:]...)
			break
		}
		if arg == "-h" || arg == "--help" {
			return cnrArgs{}, 0, ""
		}
		if cnrHasPrefix(arg, "--") {
			name, val, hasVal := arg, "", false
			if eq := cnrIndexByte(arg, '='); eq >= 0 {
				name, val, hasVal = arg[:eq], arg[eq+1:], true
			}
			canon, takes, ok := cnrMatchOpt(name)
			if !ok {
				unknown = append(unknown, arg)
				i++
				continue
			}
			if canon == "--help" {
				return cnrArgs{}, 0, ""
			}
			if takes {
				if !hasVal {
					if i+1 >= len(argv) {
						return cnrArgs{}, 2, "argument " + canon + ": expected one argument"
					}
					i++
					val = argv[i]
				}
				switch canon {
				case "--roster":
					a.roster = val
				case "--fleet":
					a.fleet = val
				case "--plist":
					a.plist = val
				case "--ctrl-plist":
					a.ctrlPlists = append(a.ctrlPlists, val)
				}
			} else {
				if hasVal {
					return cnrArgs{}, 2, "argument " + canon + ": ignored explicit argument '" + val + "'"
				}
				switch canon {
				case "--self-test":
					a.selfTest = true
				case "--list-rules":
					a.listRules = true
				case "--verify-sha":
					a.verifySha = true
				}
			}
			i++
			continue
		}
		unknown = append(unknown, arg)
		i++
	}
	if len(unknown) > 0 {
		return cnrArgs{}, 2, "unrecognized arguments: " + cnrJoinStr(unknown, " ")
	}
	return a, -1, ""
}

// ══════════════════════════ 入口 ══════════════════════════

// checkNodesRosterGateMain —— 入口（= py `main()`）。
func checkNodesRosterGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, hit := cnrRefuseJSON(stderr, argv); hit {
		return rc
	}
	a, rc, msg := cnrParseArgs(argv)
	if rc == 0 {
		fmt.Fprint(stdout, cnrHelpText)
		return 0
	}
	if rc > 0 {
		fmt.Fprint(stderr, cnrUsageLine)
		fmt.Fprintf(stderr, "check-nodes-roster.py: error: %s\n", msg)
		return 2
	}
	if a.listRules {
		fmt.Fprint(stdout, cnrDoc)
		fmt.Fprint(stdout, "\n")
		fmt.Fprintf(stdout, "路径来源：名册 %s · fleet %s · plist %s（都可覆盖；自检用夹具）\n",
			a.roster, a.fleet, a.plist)
		return 0
	}
	if a.selfTest {
		return cnrSelfTest(stdout, stderr)
	}
	rc2 := cnrSelfTest(stdout, stderr)
	if rc2 != 0 {
		fmt.Fprint(stderr, "⛔ 自检不过 ⇒ 拒绝核真目标\n")
		return 2
	}
	fmt.Fprint(stdout, "── 自检通过，开始核真目标 ──\n")
	return cnrRunChecks(stdout, a.roster, a.fleet, a.plist, true, a.verifySha, a.ctrlPlists, true, true)
}

// ══════════════════════════ 薄封装（件名前缀 · 便于并发路不撞名）══════════════════════════

func cnrReplaceAll(s, old, nw string) string { return strings.ReplaceAll(s, old, nw) }

func cnrReplaceN(s, old, nw string, n int) string { return strings.Replace(s, old, nw, n) }

func cnrSplitStr(s, sep string) []string { return strings.Split(s, sep) }

func cnrJoinStr(parts []string, sep string) string { return strings.Join(parts, sep) }

func cnrFields(s string) []string { return strings.Fields(s) }

func cnrContains(s, sub string) bool { return strings.Contains(s, sub) }

func cnrHasPrefix(s, p string) bool { return strings.HasPrefix(s, p) }

func cnrHasSuffix(s, suf string) bool { return strings.HasSuffix(s, suf) }

func cnrIndexByte(s string, b byte) int { return strings.IndexByte(s, b) }

func cnrIndexStr(s, sub string) int { return strings.Index(s, sub) }

func cnrLastIndexByte(s string, b byte) int { return strings.LastIndexByte(s, b) }

func cnrPartition(s string, sep byte) (string, string, string) {
	i := strings.IndexByte(s, sep)
	if i < 0 {
		return s, "", ""
	}
	return s[:i], string(sep), s[i+1:]
}

func cnrRepeat(s string, n int) string { return strings.Repeat(s, n) }

func cnrTrimChars(s string, cut rune) string { return strings.Trim(s, string(cut)) }

func cnrTrimLeftFunc(s string, f func(rune) bool) string { return strings.TrimLeftFunc(s, f) }

func cnrTrimRightFunc(s string, f func(rune) bool) string { return strings.TrimRightFunc(s, f) }

func cnrSortedDiff(a, b map[string]cnrValue) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// cnrLower / cnrInStrings / cnrPrefix —— py `str.lower()` / `in (…)` / 切片近似（判据 G 用）。
func cnrLower(s string) string { return strings.ToLower(s) }

func cnrInStrings(s string, xs []string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func cnrPrefix(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
