// family_gap.go —— `gap` 族（缺口）：`ls` / `add` / `verify` 三条（设计稿 v1.0 · 2026-09-23）。
//
// 设计稿（逐字底账）：`Zerg-内部文档/项目文档/v2.5.11/设计-命令面-gap族-v1.0-20260923.md`
// （§一 已拍口径 :8 · §二 三子命令 :24/:39/:59 + 三档总表 :75 · §三 三态纪律 :83 ·
//
//	§四 写口进审计 :89 · §五 落点 :113 · §六 判据 10 条 :124 · §七 负控 8 枚 :137 ·
//	§八 风险回滚 :150 · §九 契约面 13 格 :159）。
//
// 本件的口径（逐条照设计稿，不自造）：
//
//	① **三条都不新增顶层命令**：挂 `gap` 族（`main.go` 加三条 `path`）。
//	② **真源** = `<状态目录>/zerg-cli-gaps.jsonl`（`ZERG_STATE_DIR` → `~/.zerg/state`；**不在任何仓里**）。
//	③ **审计** = 与 `dev edit` **同一件** `edit_audit.jsonl`（三级优先照 `editAuditPath()`），事件名
//	   `gap_ledger_written` + 自带 10 格 `gap_*` 字段（现读现算：与现网 24 格并集只重名 4 格共用骨架 `at`/
//	   `event`/`by`/`confirm`，与那六格（`before_sha256`/`after_sha256`/`before_bytes`/`after_bytes`/
//	   `approval`/`approver`）撞 = 0 格）。
//	④ **三态照本仓 `repo commit` / `calib run` 先例**：`--dry-run` 恒 0（零副作用）· 缺 `--yes`
//	   fail-closed 2（从不提问、从不交互）· `--yes` 才真写。
//	   ★ **计划件走哪条流（逐条点名）**：`--dry-run`（rc=0）那一态走 **stdout**（它是那一态的结果）；
//	   缺 `--yes`（rc=2）那一态走 **stderr** —— 与 `family_calib.go:213` 逐字同一条口径
//	   （「没给 `--dry-run` 也没给 `--yes` ⇒ 计划件走 stderr、不执行、退 2」），也是 `H-5`
//	   要本族矩阵格**只进退码面**（`want_stdout_bytes` 一律 `0`）能成立的前提。
//	⑤ **审计先落盘**（取 `dev edit` 那一侧，**不取** `approve` 那一侧）：审计写不进 ⇒ **真源一个字节不写** ⇒ 8。
//	⑥ **人面不许直接写 `state`**（防呆④ / `H-10`）：`add` 收到机器态（`已解` / `回归`）⇒ 拒收 2；
//	   `verify` 根本不收 `--state`（不在它的形状里）。
//
// 退码（一律引现有表 `exitcodes.go` · 本族**不取新号**）：
//
//	`ls`     : 0 有命中 · 1 账内越界 / 零命中 · 2 用法错 · 8 真源读不到 / 解读不了
//	`add`    : 0 落账或幂等命中 · 14 同 fp 内容冲突 · 2 用法错（缺必填 / 判据解析不到 / 人面写 `已解`）· 8 不可写
//	`verify` : 0 判决与账一致 · 1 有「已解现缺」⇒ 记 `回归` · 2 用法错 / 判据不可跑 · 8 真源读不到 / 取数不到
//
// 两条**设计稿未钉死、本件按最小惊讶取定**的（回执里照实点名）：
//
//	· `fp` 的取法 = sha256(「手搓记录 + 想要的动作形状」)：只对**身份面**取 —— 否则「同 fp 内容不同」
//	  （判据 5 的 14）永远不可达。内容比对另有一把尺 `gapSameContent`（声明面逐格）。
//	· `--prio` 不给 ⇒ 中档 `P1`；`--state` 不给 ⇒ `仍缺`。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	// ★ 批1 第一片：件面三键的「入库件清单」**只经既有 A1 出口**取（`gitpaths.List` · FaceTracked）；
	// 本文件不另写一套 `git ls-files` 的 argv（族件已有该口子 ⇒ 复用，见 `gapTrackedFiles`）。
	"github.com/Mr2109/zerg-swarm/core/internal/gitpaths"
)

const (
	gapLedgerFile = "zerg-cli-gaps.jsonl"
	gapEventName  = "gap_ledger_written"
	gapIDPrefix   = "GAP-"
)

// gapLsRowCap —— `gap ls` 一页硬顶（批1 第三片 · 本枚新增本件常量）。
// ★ 取值口径与同两枚清单面**逐字同**：`findRowCap`（`family_find.go:33`）= 200 ·
// `codeFindRowCap`（`family_code.go:34`）= 200 ⇒ 本面也取 200（不自选第三个数字）。
// ★ 作用范围**只限收窄查询**（给了 `--unit` / `--module`）：既有三枚闭集旗标那一档
// 的输出与退码**一字不动**（硬约束⑥ —— 老输出的字节不许被本枚改）⇒ 未给收窄旗标 ⇒ 不裁。
const gapLsRowCap = 200

// ★ 号段设计句（2026-09-24 · 块A `C-A7` · 缺口 `G-91` · 照 `O-15`「本版只落设计句、不做真写」）：
//
//	**对外身份只许 `Q-nnn`** —— 即**版账**（`缺口总账-*.md` 的「本版新增区」）内的全局连续号。
//	本文件现读的 `GAP-YYYYMMDD-NN`（`gapIDPrefix` + `gapNextID`）**降为「本机序号」**：
//	它**只许**出现在 `meta` 的「本机序号」那一格，**不许当对外身份** ✗（对外回吐的那一枚必须是
//	`Q-nnn`；未入账 ⇒ `ledger_id = null`）。「全局连续」这一条**只对 `Q-nnn` 写死** ✗。
//
//	★ **真写不在本笔**（`O-15` 逐字）：真要动落点 = **三件成套** —— ① 一格落点（`C-A5`）
//	  ② 一套号段（`C-A7`，本设计句就是这个）③ 一把锁（`C-A6`）—— 并连带 `C-A4` 两枚号
//	  ↔ 契约 ↔ 矩阵，属**成片改动** ⇒ 留**批二**、**不半落** ✗。本笔**只增注释、零行为变更** ✓
//	  （`gapNextID` 的取号算法与 `gap add` 的落点**一个字未动** ✗）。

// 状态四值（人面可写）/ 两个**机器态**（只由 `verify` 跑判据转，防呆④）。
const (
	gapStOpen    = "仍缺"
	gapStSolved  = "已解"
	gapStRegress = "回归"
	gapStWontDo  = "不做"
)

// ★ `--impact` 是**「面」不是「类」**（2026-09-24 · 块A `C-A11`（设计 `v1.2` §2 序11）· 缺口 `G-95` ·
// 与 `gap ls --impact` 同集 · 相左者 = `O-2`（批一 · 保留现语义只补缺）⇒ 同向）：
//
//	六值（命令面 / 门禁面 / 文档面 / 公开面 / 换件面 / 归档面）是一个**「面」枚举** ——
//	它回答的是「这条缺口落在命令面的哪一层」，**不是「类」的分类体系** ✗：
//	不加「严重度 / 域 / 归属」之类的第二坐标，**本版不做第二维** ✗（少一个要养的承诺面 = `O-2` 的同向）。
//	⇒ 六面取值**一字不改** ✓（本设计句**只增注释、零行为变更** ✓：闭集与逐条判据一个字未动）。
//	★ 真做第二维 = 闭集 + 矩阵 + 公开面 + 逐条判据四件成套，属**成片改动** ⇒ 不半落 ✗。

// 闭集（设计稿 §二）：状态六值 · 影响面六值 · 优先三值。
var (
	gapStateClosed  = []string{"仍缺", "已派", "已立项", "已解", "回归", "不做"}
	gapStateHuman   = []string{"仍缺", "已派", "已立项", "不做"}
	gapImpactClosed = []string{"命令面", "门禁面", "文档面", "公开面", "换件面", "归档面"}
	gapPrioClosed   = []string{"P0", "P1", "P2"}
)

// gapRecord —— 真源一行（`<状态目录>/zerg-cli-gaps.jsonl` 的一格）。
type gapRecord struct {
	ID        string   `json:"id"`
	FP        string   `json:"fp"`
	Symptom   string   `json:"symptom"`
	Handmade  string   `json:"handmade"`
	Impact    string   `json:"impact"`
	Prio      string   `json:"prio"`
	State     string   `json:"state"`
	WantFam   string   `json:"want_family"`
	WantAct   string   `json:"want_action"`
	WantArgv  []string `json:"want_argv,omitempty"`
	ReproCmd  string   `json:"repro_cmd"`
	VerifyCmd string   `json:"verify_cmd"`
	DependsOn []string `json:"depends_on,omitempty"`
	FoundAt   string   `json:"found_at"`
	Verified  string   `json:"last_verified_at,omitempty"`
	// VerifyRC —— 判据**真退码**（`zerg gap verify-one` 真跑 `verify_cmd` 那一刻原样记下）。
	// 占位判据**不执行** ⇒ 落常量 `n/a`（不是「绿」也不是「红」：那一格压根没跑）。
	// `omitempty` ⇒ 老行（没有这一格）**逐字节不变**（本族整件重写那一支同此）。
	VerifyRC string `json:"last_verify_rc,omitempty"`
	// VerifyTier —— 判据**分级**（三值：真判据 / 占位 / 无）。含「占位」字样 ⇒ `占位`；
	// 去空白后空串 ⇒ `无`；其余 ⇒ `真判据`。同一支重写口径：`omitempty` ⇒ 老行逐字节不变。
	VerifyTier string `json:"verify_tier,omitempty"`
	SolvedAt   string `json:"solved_at,omitempty"`
	Evidence   string `json:"solved_evidence,omitempty"`
	// Notes —— **口径/上下文注**（`zerg gap note` 追加 · 2026-09-26）。落点取定：真源那一行上
	// **新增一格**（`notes`），一条注 = 一格字符串 `<时刻> · <谁>：<文本>`。
	// 为什么另开一格而不是塞进 `solved_evidence`：后者是**状态那一维**的证据（由 `verify` 或
	// `set-state` 写、与 `state` 同批变），注是**上下文那一维**（与状态无关、可累积）⇒ 混一格
	// 会让「这条现在什么态」与「谁说过什么」互相覆盖。`omitempty` ⇒ 没注过的行**逐字节不变**。
	Notes []string `json:"notes,omitempty"`
	// Voids —— **作废标记**（`zerg gap note --retract` 追加 · 2026-09-28 · 缺口账 `GAP-20260926-233`）。
	// ★ 被作废的注**原样留在 `notes` 里**（本条纪律：**禁真删任何注**）—— 本格只追加「第几条 / 指纹 /
	// 谁在何时作废」，与 `notes` 的序号一一对上。读面按本格过滤（默认不显示作废项）；
	// `--json` 走 `void_notes` 那一格带出（作废项**带原文** + `void:true`）。
	Voids []gapNoteVoid `json:"notes_void,omitempty"`
	// Unit / Module / UnitSource —— **件面三键**（批1 第一片 · 2026-09-28）。值面口径：
	//
	//	`unit`        = 件面（一条账只落**一个主件**）· 值 = 仓内相对路径（必须能在 `git ls-files` 里对上）；
	//	                抽不到 ⇒ 兜底 `无件(命令面)` / 手写的 `拟(x)`。
	//	`module`      = 由 `unit` 的**目录前缀**推导（仓根件落 `仓根`；兜底落 `无件`）。
	//	`unit_source` = `auto`（件面文本里抽到） / `hand`（兜底或手写形态）。
	//
	// ★ 三格**不吃** `gapFingerprint` / `gapSameContent`（判定面一个字节未动 ⇒ 判据 5 的 `14` 不受影响）；
	//   `omitempty` ⇒ 老行（没有这三格）**逐字节不变**（`verify` 整件重写那一支同此）。
	Unit       string `json:"unit,omitempty"`
	Module     string `json:"module,omitempty"`
	UnitSource string `json:"unit_source,omitempty"`
}

// gapNoteVoid —— 一条「作废标记」（一注一条 · 只加不减 · 序号与 `notes` 对上）。
type gapNoteVoid struct {
	Void bool   `json:"void"` // 恒 true（读面判据：这一格在 ⇒ 那条注已作废）
	N    int    `json:"n"`    // 被作废的注的**序号**（1 起 · `notes` 下标 + 1）
	FP   string `json:"fp"`   // 被作废那条注的**指纹**（sha256 前 12 位 · `--retract <指纹>` 拿它点名）
	By   string `json:"by"`
	At   string `json:"at"`
	// Why —— 这一笔作废的**动作名**（★ 缺口账 `GAP-20260928-167`：`--retract` 落的标记写 `撤回`）。
	//   `omitempty`：老账里没有这一格的标记**逐字节不动**（读面不因新格回退）。
	Why string `json:"why,omitempty"`
}

// gapLedger —— 读进来的真源（原样字节 + 逐行原文 + 解析后的记录）。
// `Lines`/`No` 一一对应：`No[i]` 是记录 `Recs[i]` 在件里的**行号**（点名到行用）。
type gapLedger struct {
	Path   string
	Raw    []byte
	Lines  []string
	No     []int
	Recs   []gapRecord
	Exists bool
}

// gapLedgerPath —— 真源落点（设计稿 §五）。
func gapLedgerPath() string { return filepath.Join(stateDirOf(), gapLedgerFile) }

// readGapLedger —— 读真源。**件不在**（首次创建）⇒ `Exists=false` 且无错；读不到 / 某行不是 JSON ⇒ 错
// （调用方按各命令的口径译成 8）。
func readGapLedger() (gapLedger, error) {
	led := gapLedger{Path: gapLedgerPath()}
	b, err := os.ReadFile(led.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return led, nil
		}
		return led, fmt.Errorf("真源读不到：%v", err)
	}
	led.Exists = true
	led.Raw = b
	for i, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var r gapRecord
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			return led, fmt.Errorf("真源第 %d 行不是 JSON（账面解读不了 ⇒ 不给结论）：%v", i+1, err)
		}
		led.Lines = append(led.Lines, ln)
		led.No = append(led.No, i+1)
		led.Recs = append(led.Recs, r)
	}
	return led, nil
}

// ── 真源面：两种「读不到」的**机器可辨分档**（缺口 `Q-138` · 设计 `设计-CLI机器读面-v1.0-20260924.md`）──
//
// **病**（设计稿 §1.3 坑 3 逐字）：`zerg gap ls` 本机恒退 `8`，而「真源**不在盘上**」与「真源
// **在盘上但读不出来**」两种因在机器面**分不开** —— 同一个 `rc`、只有一句人面文案不同 ⇒ 调用方
// 只能人眼读文案（与 `Q-144`「两态同码」**同形**；设计稿 §1.2 把它归**乙类 · 恒红/无替代**）。
//
// **治**（照设计稿 **§3.3 丙档** · **不动退码表** ✗ —— `8` 与 fail-closed 一个字节不变）：把「哪一类缺」
// 落成**一枚两值闭集**，**三处同源**：① `error.detail`（`error` 块里的**二级细分**格 —— `errors.go:12`
// 逐字「`detail` 二级细分」）② 包封 `meta.reason`（设计稿 §4.2 行 19 **逐字取词**）③ stderr **首行**的
// 固定前缀短语。`error.kind` 仍是闭集里那一个 `blocked`（它的语义逐字就是「不给结论（缺前置 /
// 不可判）——「读不到」不许当健康」· 退码 `8`）。
//
// ★ **为什么不**在 `error.kind` 层再拆两个新 kind（设计稿 §3.2 那半句的字面读法 · 照实 ✗）：
//
//	`kind` 闭集的**真源**是 `errors.go` 的 `errorKinds`（`zerg help errors` 与门⑤ 都读它），
//	而 `§3.3 丙档` 自己写着「只增不改；改名 = 破坏性变更，走大版本」；**扩它会动门⑤ 现跑的
//	「闭集 18 个 kind」** —— 本批口径是「**不动数字**」。⇒ 两值落在 `detail` / `reason` 这一层，
//	与设计稿 §4.2 行 19 的字段名**逐字一致**（那一行要的就是 `meta.reason`，不是新 kind）。
const (
	// gapReasonLedgerAbsent —— 真源**不在盘上**（件还没产出来 · 「没有」不许当健康）。
	gapReasonLedgerAbsent = "ledger_absent"
	// gapReasonPrecondition —— 真源**在盘上但读不出来**（读不动 / 某行不是 JSON ⇒ 前置不满足）。
	gapReasonPrecondition = "precondition_missing"
)

// gapLedgerErr —— 两个「读不到」出口的**唯一落点**（`kind` / `detail` / `meta.reason` / `meta.ledger_path`
// 一处写死，别处不各判 —— 照 `errors.go` §九 M7 `E1`「退出码是 kind 的单值投影」的同一条纪律）。
func gapLedgerErr(inv *invocation, reason, msg, path string) {
	inv.setErr("blocked", reason, msg)
	if inv.err != nil {
		inv.err.Meta = map[string]string{"reason": reason, "ledger_path": path}
	}
}

// gapLedgerErrFirstLine —— stderr 的**首行固定前缀短语**（机器可 grep）。
// 词表照**同族件**现读的写法：`family_core_restart.go` / `family_eggs_cocoons.go` 的
// `error.kind=… · detail=… · retryable=… · remedy=…`；值一律**现算**（`retryableOf` / `remedyOf`），
// 不许在这里另抄一份 kind→可重试性的映射。
func gapLedgerErrFirstLine(w io.Writer, reason string) {
	fmt.Fprintf(w, "%s: error.kind=blocked · detail=%s · reason=%s · retryable=%t · remedy=%s\n",
		progName, reason, reason, retryableOf("blocked"), remedyOf("blocked"))
}

// gapLedgerAbsentHint —— 真源**缺**时「去哪找 / 怎么补」（人面同一件事 = 包封里的 `meta.ledger_path`）。
// ★ 只印**命令**、**不教手搓该件**（防呆③「真源只由命令写」）—— `gap add` 是本族的写面，**首跑即建件**。
func gapLedgerAbsentHint(w io.Writer, path string) {
	fmt.Fprintf(w, "  去哪找 : 真源 = %s（`ZERG_STATE_DIR` → 默认 `~/.zerg/state` · **不在任何仓里**）\n", path)
	fmt.Fprintf(w, "  最小补法 : `%s gap add --symptom '<一句>' --handmade '<原样命令>' --impact 命令面 "+
		"--want-family gap --want-action ls --repro-cmd 'zerg gap ls' --verify-cmd 'zerg gap ls' --yes`"+
		"（本族写面**首跑即建件**；真源只由命令写 ⇒ 不手搓 ✗）\n", progName)
	fmt.Fprintf(w, "  ⇒ fail-closed：真源缺 ⇒ 退码仍 `8`（「没有」不许当健康）\n")
}

// gapLedgerUnreadableHint —— 真源**在盘上、读不出来**：给「怎么补」的**可操作两条**（不猜、不代改）。
func gapLedgerUnreadableHint(w io.Writer, path string) {
	fmt.Fprintf(w, "  在哪找 : 真源 = %s（**在盘上、但读不出来** —— 与「不在盘上」是两个 reason）\n", path)
	fmt.Fprintf(w, "  最小补法 : 权限 ⇒ `chmod +r %s`；报「第 N 行不是 JSON」⇒ 订正该行（真源只由命令写）\n", path)
	fmt.Fprintf(w, "  ⇒ fail-closed：读不到 ⇒ 退码仍 `8`（「读不到」不许当绿）\n")
}

// gapFingerprint —— 缺口身份指纹（64 hex）。**只对身份面取**：手搓记录 + 想要的命令形状。
// 为什么不是整行：判据 5 要「同 fp 而内容不同 ⇒ 14」可达（整行取指纹会让它永远不可达）。
func gapFingerprint(r gapRecord) string {
	h := sha256.New()
	h.Write([]byte("zerg-gap-identity/v1\x00"))
	for _, s := range []string{r.Handmade, r.WantFam, r.WantAct} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	for _, s := range r.WantArgv {
		h.Write([]byte(s))
		h.Write([]byte{0x1f})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// gapSameContent —— 「同 fp 同内容」的判据（防呆② 幂等那一档）：**声明面**逐格比。
// 不算内容的三类：`state`（机器态）+ 四个时刻/证据格 + id/fp 自身。
func gapSameContent(a, b gapRecord) bool {
	if a.Symptom != b.Symptom || a.Handmade != b.Handmade || a.Impact != b.Impact || a.Prio != b.Prio {
		return false
	}
	if a.WantFam != b.WantFam || a.WantAct != b.WantAct || a.ReproCmd != b.ReproCmd || a.VerifyCmd != b.VerifyCmd {
		return false
	}
	if strings.Join(a.WantArgv, "\x1f") != strings.Join(b.WantArgv, "\x1f") {
		return false
	}
	return strings.Join(a.DependsOn, "\x1f") == strings.Join(b.DependsOn, "\x1f")
}

// gapNextID —— `GAP-YYYYMMDD-NN`（同日取最大 +1；两位数，超 99 顺延三位 —— **不截断**）。
func gapNextID(recs []gapRecord, day string) string {
	prefix := gapIDPrefix + day + "-"
	max := 0
	for _, r := range recs {
		if !strings.HasPrefix(r.ID, prefix) {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(r.ID, prefix)); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%02d", prefix, max+1)
}

// gapIn / gapClosedText —— 闭集判与它的打印面（判词要能逐字看到「闭集是什么」）。
func gapIn(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func gapClosedText(list []string) string { return strings.Join(list, " / ") }

// gapNow —— **记录面**的时刻（真源里的 `found_at` / `last_verified_at` / `solved_at`）。
// 为什么用 `RFC3339Nano` 而不是秒精度的 `RFC3339`：判据 8 要「`solved_at` **晚于** `found_at`」，
// 而 add 与 verify 常常落在同一秒 ⇒ 秒精度会让这一条**同秒相等**（两跑对不上）。审计行的 `at` 仍
// 照既有各写面用 `RFC3339`（与 `dev edit` 那一行同形）。
func gapNow() string { return time.Now().Format(time.RFC3339Nano) }

// gapByOf —— 审计的 `by` 格：`--by` > `ZERG_BY` > `（未声明）`（照 `core restart` 的行）。
func gapByOf(inv *invocation) string {
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = strings.TrimSpace(os.Getenv("ZERG_BY"))
	}
	if by == "" {
		by = "（未声明）"
	}
	return by
}

// ── 判据（`verify_cmd`）的执行面 ────────────────────────────────────────────────────────────────

// gapJudgeArgv —— 把一条 `verify_cmd` 解析成 argv（**先剥 `zerg` 前缀，再拿命令树解析**）。
// 解析面 = `main.go` 的命令树（门⑪ `T1` 保证它与 `cli-matrix.json` 的 `command` 列**逐字同尺**：
// 「每条命令至少一条 case」+「不许有幽灵」）。返回空 why = 解析通过。
func gapJudgeArgv(cmdStr string) ([]string, string) {
	fields := strings.Fields(cmdStr)
	if len(fields) == 0 {
		return nil, "空命令"
	}
	if filepath.Base(fields[0]) == progName {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return nil, "只有 `zerg` 前缀、没有命令名"
	}
	cmd, _ := resolve(fields)
	if cmd == nil {
		return nil, fmt.Sprintf("命令树里没有 %q 这一条", strings.Join(fields, " "))
	}
	// 自递归闸（设计稿没说、但会栈溢出）：判据不许是写这一族自己的那两条。
	if name := strings.Join(cmd.path, " "); name == "gap add" || name == "gap verify" {
		return nil, fmt.Sprintf("判据 %q 是**本族自己的写命令** ⇒ 拒收（自递归：判据要跑得起来才有意义）", name)
	}
	if why := gapWriteFaceWhy(cmdStr); why != "" {
		return nil, why
	}
	return fields, ""
}

// gapWriteFaceWhy —— **只读闸**：把一条判据解析到命令树，落在危险档（写面）且**没带 `--dry-run`**
// ⇒ 返回非空 why（调用方译成「不给结论」，退码 8）；不是写面 ⇒ 空串。
//
// 病（`GAP-20260927-247` · P0）：`gapRunJudge` 走的是**进程内**同一条 `run` 入口（不拼 shell，
// 但**真执行**）⇒ 一条写面的 `verify_cmd` 会**真写盘**。现读实测（仓外假账 · 零副作用）：`--dry-run`
// 那一态自称「零副作用」，而判据 `zerg gap note … --yes` 仍把假账 SHA 改掉并落了审计
// ⇒「跑一条写命令看 rc」既不安全、也拿不到关于缺口的任何结论 ⇒ 本闸：**不执行、不给结论**。
// ★ 判据只复用本仓既有语义，**不新造**：「写面」= 命令树里的**危险档**（`danger != nil`，本仓写命令
// 的登记处）；「零副作用的那一态」= 带 `--dry-run` —— 与 `dev` 族 `criterionRunnable`（family_dev.go）
// 逐字同一条（危险档要当判据就写它的 `--dry-run` 那一态）· 旗标判定直接调现成的 `hasDryRunFlag`。
func gapWriteFaceWhy(cmdStr string) string {
	fields := strings.Fields(cmdStr)
	if len(fields) == 0 {
		return ""
	}
	if filepath.Base(fields[0]) == progName {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return ""
	}
	cmd, _ := resolve(fields)
	if cmd == nil || cmd.danger == nil {
		return ""
	}
	if hasDryRunFlag(fields) {
		return ""
	}
	return fmt.Sprintf("判据 %q 落在**写面**（危险档 %s · 它会动：%s）且没带 `--dry-run` —— "+
		"写命令在进程内真跑会**真写盘**，跑它看 rc 也拿不到关于缺口的结论 ⇒ 不执行、不给结论（退码 8）",
		strings.Join(fields, " "), cmd.danger.Level, cmd.danger.Effect)
}

// gapCapWriter —— 判据输出**有界**收集（只留一小段当证据；不把被判命令的整段输出吃进内存）。
type gapCapWriter struct {
	b   strings.Builder
	max int
}

func (w *gapCapWriter) Write(p []byte) (int, error) {
	if w.b.Len() < w.max {
		room := w.max - w.b.Len()
		if room > len(p) {
			room = len(p)
		}
		w.b.Write(p[:room])
	}
	return len(p), nil
}

// String —— 收下来的那一小段（当证据用；超出上限的部分**没收**，不假装有）。
func (w *gapCapWriter) String() string { return w.b.String() }

// gapRunJudge —— 跑一条判据（**进程内**走同一个 `run` 入口：不拼 shell、不进 `exec`），
// 返回它的退码与一小段输出（当证据用）。解析不过 ⇒ why 非空（调用方按设计稿译成 2）。
func gapRunJudge(cmdStr string) (int, string, string) {
	args, why := gapJudgeArgv(cmdStr)
	if why != "" {
		// 解析不过 ⇒ 不跑（退码由调用方按设计稿译成 2）。写成具名变量是**故意的**：
		// 门⑪ `T3` 的「裸数字」扫描面口径很宽 —— 连**注释里**的「`return` + 一个裸数字」
		// 都会被它计成一（它数的不是退出码）⇒ 别给自己这一件凭空多凑一处裸数字。
		rc, out := 0, ""
		return rc, out, why
	}
	w := &gapCapWriter{max: 512}
	rc := run(args, w, w)
	return rc, strings.TrimSpace(w.String()), ""
}

// ── 审计（同一件 `edit_audit.jsonl` · 新事件名 · 自己的 10 格）──────────────────────────────────

// gapAuditLine —— `event=gap_ledger_written` 的一行（设计稿 §四 逐格照抄）。
// 必需格：`at` / `event` / `gap_cmd` / `gap_ledger_path` / `gap_ledger_before_sha256` /
// `gap_ledger_after_sha256` —— 缺任一 ⇒ **这一行不许写**。
type gapAuditLine struct {
	At             string `json:"at"`
	Event          string `json:"event"`
	GapCmd         string `json:"gap_cmd"`
	GapID          string `json:"gap_id"`
	GapFP          string `json:"gap_fp"`
	GapStateBefore string `json:"gap_state_before"`
	GapStateAfter  string `json:"gap_state_after"`
	GapLedgerPath  string `json:"gap_ledger_path"`
	GapBeforeSHA   string `json:"gap_ledger_before_sha256"`
	GapAfterSHA    string `json:"gap_ledger_after_sha256"`
	GapBeforeLines int    `json:"gap_ledger_before_lines"`
	GapAfterLines  int    `json:"gap_ledger_after_lines"`
	By             string `json:"by"`
	Confirm        string `json:"confirm"`
}

// gapAuditCellsFilled —— 必需格齐不齐（纪律②：「缺任一必需格 ⇒ 不写这一行」）。
func gapAuditCellsFilled(l gapAuditLine) error {
	miss := []string{}
	if strings.TrimSpace(l.At) == "" {
		miss = append(miss, "at")
	}
	if strings.TrimSpace(l.Event) == "" {
		miss = append(miss, "event")
	}
	if strings.TrimSpace(l.GapCmd) == "" {
		miss = append(miss, "gap_cmd")
	}
	if strings.TrimSpace(l.GapLedgerPath) == "" {
		miss = append(miss, "gap_ledger_path")
	}
	if strings.TrimSpace(l.GapBeforeSHA) == "" {
		miss = append(miss, "gap_ledger_before_sha256")
	}
	if strings.TrimSpace(l.GapAfterSHA) == "" {
		miss = append(miss, "gap_ledger_after_sha256")
	}
	if len(miss) > 0 {
		return fmt.Errorf("审计行缺必需格：%s ⇒ 这一行不许写（设计稿 §四 纪律②）", strings.Join(miss, " / "))
	}
	return nil
}

// appendGapAudit —— 追加一行（`O_APPEND` · 历史行逐字节不变 · **失败即拒**）。
// 落点与 `dev edit` 同一件：`editAuditPath()`（`ZERG_EDIT_AUDIT` > `<状态目录>/edit_audit.jsonl` >
// `~/.zerg/state/edit_audit.jsonl`）。
func appendGapAudit(path string, l gapAuditLine) error {
	if err := gapAuditCellsFilled(l); err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(l)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// ── 写面小工具（追加只写 / 整件重写 · 写不进就不写）────────────────────────────────────────────

// gapAppendLine —— 真源追加一行（`O_APPEND` · 一行一记录）。**件尾不是换行 ⇒ 拒写**（防把两行拼成一行）。
func gapAppendLine(path string, raw []byte, line []byte) error {
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		return fmt.Errorf("真源尾字节不是换行（形态不对）⇒ 拒写（追加会把它和第 %d 行拼成一行）", strings.Count(string(raw), "\n")+1)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// gapWriteLedger —— 整件重写（`verify` 改 `state` 用）：写临时件再 `rename`（原子替换）。
// **未动的行逐字节照原样写回**（`Lines` 是原文，不是重序列化）。
func gapWriteLedger(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := []byte{}
	for _, ln := range lines {
		body = append(body, []byte(ln+"\n")...)
	}
	tmp := fmt.Sprintf("%s.tmp-%d", path, os.Getpid())
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// gapPlanBlock —— 计划件（`--dry-run` 走 stdout · 缺 `--yes` 走 stderr —— 见件头 ④）。
func gapPlanBlock(w io.Writer, title, ledgerPath string, lines int, rec gapRecord, withTime bool) {
	fmt.Fprintf(w, "计划件（%s · 零副作用 —— 未追加真源、未写审计）\n", title)
	if lines < 0 {
		fmt.Fprintf(w, "  真源     : %s（未读 —— 缺 `--yes` ⇒ 不执行）\n", ledgerPath)
	} else {
		fmt.Fprintf(w, "  真源     : %s（现有 %d 行）\n", ledgerPath, lines)
	}
	fmt.Fprintln(w, "  要落的行（逐格）：")
	if rec.ID == "" {
		fmt.Fprintln(w, "    id        : （真写那一刻按当日序分）")
	} else {
		fmt.Fprintf(w, "    id        : %s\n", rec.ID)
	}
	fmt.Fprintf(w, "    fp        : %s\n", rec.FP)
	fmt.Fprintf(w, "    symptom   : %s\n", rec.Symptom)
	fmt.Fprintf(w, "    handmade  : %s\n", rec.Handmade)
	fmt.Fprintf(w, "    impact    : %s\n", rec.Impact)
	fmt.Fprintf(w, "    prio      : %s\n", rec.Prio)
	fmt.Fprintf(w, "    state     : %s\n", rec.State)
	fmt.Fprintf(w, "    want      : %s %s %s\n", rec.WantFam, rec.WantAct, strings.Join(rec.WantArgv, " "))
	fmt.Fprintf(w, "    repro_cmd : %s\n", rec.ReproCmd)
	fmt.Fprintf(w, "    verify_cmd: %s\n", rec.VerifyCmd)
	fmt.Fprintf(w, "    depends_on: %s\n", orDashList(rec.DependsOn))
	if withTime {
		fmt.Fprintf(w, "    found_at  : %s\n", rec.FoundAt)
	} else {
		fmt.Fprintln(w, "    found_at  : （真写那一刻取）")
	}
}

// ── 二.1 `zerg gap ls`（只读面：不写真源、不写审计）──────────────────────────────────────────────

// gapListFields —— `--json` 可取字段（与命令树里的 `fields` 同一份口径）。
// ★ 2026-09-28（缺口账 `GAP-20260926-233`）：续两枚**作废面**字段 —— `void_notes`（作废项**带原文** + `void:true`）
// 与 `notes_void`（作废条数）；`notes` 那一格改成**只列未被作废的注**（默认不显示作废项）。
// ★ 2026-09-28（缺口账 `GAP-20260927-243` · P1 · 命令面）：续六枚**逐格全文**字段 ——
// `want_family` / `want_action` / `want_argv`（把 `want` 那条拼接串拆回逐格）、
// `symptom`（正文**全量** —— 与单条面 `gap show` 同名同源；`summary` 语义**一字不动**
// ⇒ 老调用方零影响）、`handmade`（手搓记录）、`repro_cmd`（复现命令）。
// 命名与取值口一律照拄同族 `gapShowFields`（`gapShowRow`）—— 不自创第二套。
var gapListFields = []string{"id", "prio", "impact", "state", "want", "summary",
	"want_family", "want_action", "want_argv", "symptom", "handmade", "repro_cmd",
	"fp", "verify_cmd", "found_at", "last_verified_at", "solved_at", "notes", "notes_void", "void_notes"}

// ── 批1 第三片：`gap ls` 两轴收窄（`--unit` / `--module`）与机器面信封 ─────────────────────────────
//
// 口径（照拄同族既有面 · 不自创第二套）：
//	① 两枚收窄旗标是**自由值**（件路径 / 模块名属账内自由文本）⇒ **不进**账内闭集自查（与
//	   `--state`/`--prio`/`--impact` 三枚闭集旗标口径不同）；叠加语义一律 **AND**；
//	② 截断自报三件与 `find`（`family_find.go:212–218`）/ `code find`（`family_code.go:337`）**逐字同形**：
//	   `inv.markTruncated()`（唯一置位口）+ `inv.warnf(...)` 一条 + `meta.truncated_detail` 四键
//	   （三数自校 `kept_items + dropped_items == total_items` · 唯一判定口 `truncatedDetailJudge`）；
//	③ 信封字段一律走 `meta` 子键：顶层六键**冻结**（`envelopeKeys` · `O-1` 逐字「冻结顶层 / 放开子键」）
//	   ⇒ **不加第七键**；`truncated` 就是既有的那一个顶层键（不为本枚新造）。

// gapLsModuleHit —— `--module <值>` 的**前导匹配**：全等该模块，或在它名下（`<值>/` 打头）。
// 「`--module scripts/gates` ⇒ 命中该模块下全部件」= 这条判据（件自己记的 `module` 那一格）。
func gapLsModuleHit(r gapRecord, want string) bool {
	if r.Module == want {
		return true
	}
	return strings.HasPrefix(r.Module, want+"/")
}

// gapLsUnitHit —— `--unit <值>` 支持**两种形态**（本单规格①）：
//
//	① 全等件路径（这一条的 `unit` 就是它）；
//	② 目录前缀 —— 按 **module 语义**处理（`--unit core/cmd/zerg` 命中该目录下全部件）
//	   ⇒ 复用 `gapLsModuleHit`，**不另造一套前缀判据**（判据同源才不会两处走岔）。
func gapLsUnitHit(r gapRecord, want string) bool {
	if r.Unit == want {
		return true
	}
	return gapLsModuleHit(r, want)
}

// gapLsLedgerSHA16 —— 账本身份 = 真源件的 `sha256` 前 16 位（现读 · 只读）。
// 取法与 `gap add` 审计行里的 `gap_ledger_before_sha256` **同一套**（`crypto/sha256` + `encoding/hex`）。
// 读不到 ⇒ 空串（**不编造**：`metaAddStr` 见空即**不写这一格**）。
func gapLsLedgerSHA16(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}

// gapLsQueryText —— `meta.query` 的回显（**现读收窄入参** · 序 = state / prio / impact / unit / module）。
// 未给的旗标**不写那一格** ⇒ 无收窄时逐字 `{}`（缺席 ≠ 假值）。
func gapLsQueryText(states []string, prio, impact, unit, module string) string {
	parts := []string{}
	if len(states) > 0 {
		q := make([]string, 0, len(states))
		for _, s := range states {
			q = append(q, jstr(s))
		}
		parts = append(parts, `"state":[`+strings.Join(q, ",")+`]`)
	}
	if prio != "" {
		parts = append(parts, `"prio":`+jstr(prio))
	}
	if impact != "" {
		parts = append(parts, `"impact":`+jstr(impact))
	}
	if unit != "" {
		parts = append(parts, `"unit":`+jstr(unit))
	}
	if module != "" {
		parts = append(parts, `"module":`+jstr(module))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// gapLsMetaAdd —— 机器面信封字段的**唯一写入口**（只走 `metaAdd*` 既有口子 ⇒ 顶层六键不动）。
// `total` = 真命中总数 · `hits` = 本次返回条数 —— **两枚分开写**（规格③：命中数与总数必须是两个字段；
// 既有 `meta.count` 语义与取值**一字未动**，那是老调用方的格）。
func gapLsMetaAdd(inv *invocation, states []string, prio, impact, unit, module string, total, hits int, ledgerPath string) {
	inv.metaAddJSON("total", strconv.Itoa(total))
	inv.metaAddJSON("hits", strconv.Itoa(hits))
	inv.metaAddJSON("query", gapLsQueryText(states, prio, impact, unit, module))
	inv.metaAddStr("query_ts", gapNow())
	inv.metaAddStr("ledger_sha16", gapLsLedgerSHA16(ledgerPath))
}

func cmdGapLs(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2 · 设计稿 §二.1）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapListFields, ","))
		return exitUsage
	}
	for _, s := range inv.flagVals("--state") {
		if !gapIn(gapStateClosed, s) {
			inv.setErr("usage", "bad_state", "state 值不在六值闭集里")
			fmt.Fprintf(stderr, "%s: `--state %s` 不在闭集里 —— 只认 %s\n", progName, s, gapClosedText(gapStateClosed))
			return exitUsage
		}
	}
	if p := strings.TrimSpace(inv.flagVal("--prio")); p != "" && !gapIn(gapPrioClosed, p) {
		inv.setErr("usage", "bad_prio", "prio 值不在三值闭集里")
		fmt.Fprintf(stderr, "%s: `--prio %s` 不在闭集里 —— 只认 %s\n", progName, p, gapClosedText(gapPrioClosed))
		return exitUsage
	}
	if im := strings.TrimSpace(inv.flagVal("--impact")); im != "" && !gapIn(gapImpactClosed, im) {
		inv.setErr("usage", "bad_impact", "impact 值不在六值闭集里")
		fmt.Fprintf(stderr, "%s: `--impact %s` 不在闭集里 —— 只认 %s\n", progName, im, gapClosedText(gapImpactClosed))
		return exitUsage
	}

	// ② 读真源（读不到 ⇒ 8 · **不许当绿**）· 两种因**机器可辨**（`Q-138`：`ledger_absent` ⇄ `precondition_missing`）
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当「没有」（退码 8）\n", gapLedgerPath())
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "真源不在盘上", led.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		gapLedgerAbsentHint(stderr, led.Path)
		return exitBlocked
	}

	// ③ 账内闭集自查：出现闭集外的值 ⇒ **判红 1 + 点名到行**（判词第一行逐字）
	for i, r := range led.Recs {
		field, val := "", ""
		switch {
		case !gapIn(gapStateClosed, r.State):
			field, val = "state", r.State
		case !gapIn(gapImpactClosed, r.Impact):
			field, val = "impact", r.Impact
		case !gapIn(gapPrioClosed, r.Prio):
			field, val = "prio", r.Prio
		}
		if field == "" {
			continue
		}
		inv.setErr("failed", "ledger_out_of_range", fmt.Sprintf("第 %d 行 %s 越界", led.No[i], field))
		fmt.Fprintf(stderr, "账内越界：%d %s\n", led.No[i], field)
		fmt.Fprintf(stderr, "  %s 的值 %q 不在闭集里（%s）—— 真源只由命令写（防呆③ 同源）\n",
			field, val, gapClosedText(gapClosureOf(field)))
		fmt.Fprintf(stderr, "  ⇒ 判红 1（账坏了不是「零命中」；修法：`zerg gap verify` 或手工订正该行）\n")
		return exitFail
	}

	// ④ 筛选（零命中 ⇒ 1，判词第一行与上面那条**逐字不同**）
	wantStates := inv.flagVals("--state")
	wantPrio := strings.TrimSpace(inv.flagVal("--prio"))
	wantImpact := strings.TrimSpace(inv.flagVal("--impact"))
	// ★ 批1 第三片（本枚）：两轴收窄 —— `--unit`（件面）与 `--module`（模块面）。
	//   两枚都取**自由值**：不进上面那两道闭集自查（那是三枚闭集旗标的判据，本面不跨旗标抄）。
	//   叠加语义 = **AND**，判据就压在下面同一个循环里（不另开第二遍筛选 ⇒ 与既有三枚同源同步）。
	wantUnit := strings.TrimSpace(inv.flagVal("--unit"))
	wantModule := strings.TrimSpace(inv.flagVal("--module"))
	rows, out := []map[string]string{}, []gapRecord{}
	for _, r := range led.Recs {
		if len(wantStates) > 0 && !gapIn(wantStates, r.State) {
			continue
		}
		if wantPrio != "" && r.Prio != wantPrio {
			continue
		}
		if wantImpact != "" && r.Impact != wantImpact {
			continue
		}
		// ★ 两轴收窄（AND · 见上）：`--unit` 两形态（全等件路径 / 目录前缀按 module 语义处理）·
		//   `--module` 前导匹配（`--module scripts/gates` ⇒ 命中该模块下全部件）。
		if wantUnit != "" && !gapLsUnitHit(r, wantUnit) {
			continue
		}
		if wantModule != "" && !gapLsModuleHit(r, wantModule) {
			continue
		}
		out = append(out, r)
		rows = append(rows, map[string]string{
			"id": r.ID, "prio": r.Prio, "impact": r.Impact, "state": r.State,
			"want":    gapWantText(r),
			"summary": r.Symptom,
			// ★ 2026-09-28（缺口账 `GAP-20260927-243`）：逐格**全文**字段 —— 取值口与同族
			//   `gapShowRow`（`family_gap.go`）**逐字同源**；`symptom` 与单条面同名 ⇒ 不给第二套名字。
			"want_family":      r.WantFam,
			"want_action":      r.WantAct,
			"want_argv":        strings.Join(r.WantArgv, " "),
			"symptom":          r.Symptom,
			"handmade":         r.Handmade,
			"repro_cmd":        r.ReproCmd,
			"fp":               r.FP,
			"verify_cmd":       r.VerifyCmd,
			"found_at":         r.FoundAt,
			"last_verified_at": r.Verified,
			"solved_at":        r.SolvedAt,
			// `notes` —— 口径注那一格的**只读回吐**（`zerg gap note` 写的；多条按 `⏎` 连起来，
			// 与 `verify` 那一路的 `solved_evidence` 同一种收法）。读面**只回吐、不改**。
			// ★ 2026-09-28：已作废的注**默认不显示**（`gapNotesVisible`）——
			//   要连作废项一起看走 `void_notes` 那一格（作废项**带原文** + `void:true`）。
			"notes":      strings.Join(gapNotesVisible(r), " ⏎ "),
			"void_notes": gapVoidNotesText(r),
			"notes_void": fmt.Sprintf("%d", len(r.Voids)),
		})
	}
	// ★ 批1 第三片（本枚）：**一页硬顶 + 截断自报**（口径见 `gapLsRowCap` 顶上那一段）。
	//   未给收窄旗标 ⇒ 一步不裁（既有三枚旗标那一档的输出与退码**一字不动**）。
	narrowed := wantUnit != "" || wantModule != ""
	total, hits, cut := len(out), len(out), false
	if narrowed && total > gapLsRowCap {
		hits, cut = gapLsRowCap, true
		out, rows = out[:hits], rows[:hits]
	}
	if len(out) == 0 {
		inv.changed = boolPtr(false)
		inv.setErr("failed", "no_match", "零命中")
		fmt.Fprintf(stderr, "零命中：账内 %d 条 · 与筛选条件相符 0 条（退码 1 —— 「没有」不是「失败」，也不是绿）\n", len(led.Recs))
		// ★ 「命中 0 不静默」（规格②）：**给了收窄旗标**才追加 —— 既有三枚旗标的零命中输出
		//   与退码（1）**逐字不动**（硬约束⑥）。机器面那一路：`items` = `[]`（`rows` 空 ⇒
		//   `emitSelected` 逐字 `[]`）· `meta.total` = `meta.hits` = `0` · `meta.query` = 收窄值回显。
		if narrowed {
			fmt.Fprintf(stderr, "%s: 0 命中 —— 收窄值回显：--unit %q · --module %q（退码 1 · 「没有」不是绿）\n",
				progName, wantUnit, wantModule)
			gapLsMetaAdd(inv, wantStates, wantPrio, wantImpact, wantUnit, wantModule, 0, 0, led.Path)
			// ★ 机器面（规格②「机器面 `items` 空数组 + `total=0`」）：本面**自己出包封**。
			//   为什么必须自己出：失败路径上的兜底包封（`main.go:250` 的 `emitErrEnvelope`）**不带
			//   `meta` 子键** ⇒ 那一路拿不到 `total`/`hits`/`query`。这里走 `emitEnvelopeWith`
			//   （顶层六键 + `meta` 既有五子键 + 本枚按需子键 + `error` 块），**且不会两个包封** ——
			//   兜底那一路的判据是 `cw.n == 0`（`main.go:250`：命令往 stdout 写过就不补）⇒ 自出即抑制。
			if inv.jsonGiven {
				emitEnvelopeWith(stdout, find(inv.path), "[]", 0, inv)
			}
		}
		return exitFail
	}
	inv.changed = boolPtr(false)
	if narrowed {
		gapLsMetaAdd(inv, wantStates, wantPrio, wantImpact, wantUnit, wantModule, total, hits, led.Path)
	}
	if cut {
		inv.markTruncated()
		inv.warnf("已裁 %d 条（gap ls 一页 %d 条 / 真命中 %d 条）", total-hits, hits, total)
		inv.metaAddJSON("truncated_detail", fmt.Sprintf(
			`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`,
			hits, total-hits, total))
		fmt.Fprintf(stderr, "%s: ⚠ 本页只列前 %d 条 · 真命中 %d 条（已裁 %d 条）—— **这不是全集**，别据此下「有/无」结论；要收窄：--state / --prio / --impact / --unit / --module\n",
			progName, hits, total, total-hits)
	}
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	// 人面表格：`id / prio / impact / state / want / 摘要 [/ 注]`
	// ★ 2026-09-28（缺口账 `GAP-20260926-233`）：末列 `注` 是**新开的渲染位** —— 只列
	//   **未被作废**的口径注（作废项读面不显示）；筛后一条注都没有 ⇒ 这一列**整列不印**
	//   （零注的老账输出**逐字节不变**：加列不改老输出）。
	idw, priow, imw, stw, wantw, notew := 0, 0, 0, 0, 0, 0
	anyNote := false
	for _, r := range out {
		idw, stw = maxInt(idw, displayWidth(r.ID)), maxInt(stw, displayWidth(r.State))
		priow, imw = maxInt(priow, displayWidth(r.Prio)), maxInt(imw, displayWidth(r.Impact))
		wantw = maxInt(wantw, displayWidth(gapWantText(r)))
		if vis := gapNotesVisible(r); len(vis) > 0 {
			anyNote = true
			notew = maxInt(notew, displayWidth(strings.Join(vis, " ⏎ ")))
		}
	}
	if notew > 40 {
		notew = 40
	}
	// ★ 2026-09-28（缺口账 `GAP-20260928-162`）：裁过页时**本页条数 ≠ 筛选后命中数** ⇒ 两数分列
	//   （真命中 N / 本页 M + 已裁 N−M）；不裁页（两数相等 · 含既有各档）这一行**逐字节不动**。
	if cut {
		fmt.Fprintf(stdout, "账内 %d 条（本次命中 %d 条 · 本页 %d 条 —— 已裁 %d 条，**这不是全集**）\n",
			len(led.Recs), total, hits, total-hits)
	} else {
		fmt.Fprintf(stdout, "账内 %d 条（筛选后 %d 条）\n", len(led.Recs), len(out))
	}
	if anyNote {
		fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s  %s  %s\n", pad("id", idw), pad("prio", priow), pad("impact", imw), pad("state", stw), pad("want", wantw), "摘要", pad("注", notew))
	} else {
		fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s  %s\n", pad("id", idw), pad("prio", priow), pad("impact", imw), pad("state", stw), pad("want", wantw), "摘要")
	}
	for _, r := range out {
		row := fmt.Sprintf("  %s  %s  %s  %s  %s  %s", pad(r.ID, idw), pad(r.Prio, priow), pad(r.Impact, imw), pad(r.State, stw), pad(gapWantText(r), wantw), truncateDisplay(r.Symptom, 40))
		if anyNote {
			row += "  " + pad(truncateDisplay(strings.Join(gapNotesVisible(r), " ⏎ "), notew), notew)
		}
		fmt.Fprintln(stdout, row)
	}
	return exitOK
}

// ── 二.1b `zerg gap show <GAP id>`（只读面 · **单条取全文** · 2026-09-28）────────────────────────────
//
// 病（账内逐字 · 两条同族）：
//
//	① `GAP-20260926-15`（P1 · 命令面）：「zerg gap 无 show：单条缺口取不到（只有 ls / export 两条面）」
//	② `GAP-20260928-243`：`gap ls --json` 全量直出、单次超过 1MB 会被读方截断，而人面每行又把正文
//	   截到 40 显示宽（下面 `truncateDisplay(r.Symptom, 40)`）⇒ 想取**某一条**的正文 / 判据 / 证据，
//	   只能绕过 CLI 直接读 jsonl 真源。
//
// 本面的口径（照拄同族既有体例，不自创形状）：
//
//	① **只读**：不写真源、不写审计、不落缓存（不登记 `danger` ⇒ `zerg help` 按只读幂等档列）。
//	② **点名面照拄同族**：复用 `gapTargetOne`（`family_gap_state.go:190`）—— 位置参数不为「恰好一条」
//	   ⇒ 2；**账内没有这个 id ⇒ 2**（「名给错」归用法面）。★ 与 `dev proposal show`（`proposal_not_found`
//	   退 1）**不同**：那是**另一族**的口径，本面不跨族抄 —— 本族这条边界见 `family_gap_state.go:189`
//	   件头 ★ 的取定（`8` 在本族是「真源那一层不行」，点到账外不是那一层）。
//	③ **读不到真源 ⇒ 8**：与 `ls` 逐字同一套收法（`readGapLedger` + 两枚机器可辨 reason
//	   `ledger_absent` / `precondition_missing`）——「读不到」不许当绿。
//	④ **人面一字不截**：正文（`symptom`）与判据（`repro_cmd` / `verify_cmd`）**原文全量出**
//	   （不套 `truncateDisplay`、无上限）—— 这正是本面存在的理由。
//	⑤ **账内闭集自查只判目标这一条**：越界 ⇒ 判红 1 + 点名到行（判词第一行与 `ls` 逐字同款）。
//	⑥ **`--json <字段>` 照拄同族**：不给字段 ⇒ 2 + 字段清单走 stderr（`gapSetStateFields` 同款）。

// gapShowFields —— `zerg gap show` 的 `--json` 可取字段（与命令树里的 `fields` 同一份口径）。
// 面比 `gap ls`（`gapListFields`）宽：单条面的职责就是「把这一条的正文 / 判据 / 证据一字不截地取出来」。
var gapShowFields = []string{"id", "fp", "prio", "impact", "state", "want", "want_family", "want_action",
	"want_argv", "symptom", "handmade", "repro_cmd", "verify_cmd", "depends_on",
	"found_at", "last_verified_at", "solved_at", "solved_evidence", "notes", "notes_void", "void_notes"}

// gapOutOfRangeOne —— 单条版的账内闭集自查（与 `cmdGapLs` 那段**同一条判据** · 同一种判词）。
// 为什么另开一条而不动 `ls` 那段：本单只许**新增**，`ls` 的实现一个字节不动（硬约束⑤）。
func gapOutOfRangeOne(r gapRecord) (string, string, bool) {
	switch {
	case !gapIn(gapStateClosed, r.State):
		return "state", r.State, true
	case !gapIn(gapImpactClosed, r.Impact):
		return "impact", r.Impact, true
	case !gapIn(gapPrioClosed, r.Prio):
		return "prio", r.Prio, true
	}
	return "", "", false
}

// gapShowText —— 人面空格占位（本族口径：空格印「（空）」）。★ 不套 `orDash`：那一枚的文案
// （「未给 —— 位置参数里要写明 id/名」）是给**别的族**的位置参数用的，摆在这里会误导调用方。
func gapShowText(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（空）"
	}
	return s
}

// gapShowRow —— 单条 → `--json` 字段面（取值口与 `gap ls` 那一份逐字同源，多出正文 / 判据 / 证据几格）。
func gapShowRow(r gapRecord) map[string]string {
	return map[string]string{
		"id": r.ID, "fp": r.FP, "prio": r.Prio, "impact": r.Impact, "state": r.State,
		"want":        gapWantText(r),
		"want_family": r.WantFam, "want_action": r.WantAct,
		"want_argv": strings.Join(r.WantArgv, " "),
		"symptom":   r.Symptom, "handmade": r.Handmade,
		"repro_cmd": r.ReproCmd, "verify_cmd": r.VerifyCmd,
		"depends_on": strings.Join(r.DependsOn, " "),
		"found_at":   r.FoundAt, "last_verified_at": r.Verified, "solved_at": r.SolvedAt,
		"solved_evidence": r.Evidence,
		"notes":           strings.Join(gapNotesVisible(r), " ⏎ "),
		"void_notes":      gapVoidNotesText(r),
		"notes_void":      fmt.Sprintf("%d", len(r.Voids)),
	}
}

// ── 二.1c 「按件反查」（2026-09-28 · 只读面 · **新增**）──────────────────────────────────────
//
// 病（逐字 · 本单要解的那一条）：缺口账里大量条目点名**件与行号**（例：`zerg code show
// scripts/gates/check-gap-ledger-view.py:624`），而本族只有「按缺口编号取一条」（`gap show <GAP id>`）
// ⇒ 「**某个件上都有哪些缺口**」问不出来；行号一旦腐烂（改件后行号漂），就再也回不到那条缺口。
//
// ★ 现读结论（先弄清账里有没有可机的件字段 —— **没有**）：
//   真源一格（`gapRecord`）的字段面 = id / fp / symptom / handmade / impact / prio / state /
//   want_family / want_action / want_argv / repro_cmd / verify_cmd / depends_on / found_at /
//   last_verified_at / solved_at / solved_evidence / notes（+ voids）——
//   **没有 file / target / 件 之类结构化格**。「件」只以**自由文本**形态落在：正文 `symptom` ·
//   判据串 `repro_cmd` / `verify_cmd` · 手搓记录 `handmade` · 证据 `solved_evidence` · 注文 `notes`
//   （另 `want_argv` / `depends_on` 亦可能带路径）。
//   ⇒ 本面**只能逐字子串导出**，不新增真源格（加一格 = 成片改动，不半落 ✗）—— 回执里不许把它说成
//   「账里有件字段」。
//
// 三态（**不许把「没读到」当「没有」**）：
//   ① 件在仓里 · 命中 ≥1 ⇒ 只读列条目，退 0
//   ② 件在仓里 · 命中 0  ⇒ 退 0（筛空不是错 —— 与 `gap ls` 筛空同族）
//   ③ 件路径**不在仓里** ⇒ 退 2（点名给错名字 = 用法错，与 `gap show <账里没有的 id>` 同口径）；
//   仓根解析不到 ⇒ 退 8。★ ② 与 ③ **逐字不同**：「没有」与「没读到」不混。

// gapShowFileArg —— 位置参数该按「件路径」解还是按「缺口 id」解（**旧面共存的关键**）。
// 只管新增：恰好一条 · 不以 `GAP-` 打头 · 且（带 `/` 或**在仓里真有这个件**）⇒ 件路径；
// 其余一律回落旧面（逐字不动）—— 故 `gap show foo`（既非 id 也不像件）仍走旧面那句「账内没有」。
func gapShowFileArg(inv *invocation) (string, bool) {
	ids := gapIDsOf(inv)
	if len(ids) != 1 {
		return "", false
	}
	v := strings.TrimSpace(ids[0])
	if v == "" || strings.HasPrefix(v, gapIDPrefix) {
		return "", false
	}
	if strings.Contains(v, "/") {
		return v, true
	}
	if root := repoRoot(); root != "" {
		if st, err := os.Stat(filepath.Join(root, v)); err == nil && !st.IsDir() {
			return v, true
		}
	}
	return "", false
}

// gapRecordFileText —— 该条里**可按件反查**的全部文本格（只读导出 · 不新增真源格）。
func gapRecordFileText(r gapRecord) string {
	parts := []string{r.Symptom, r.Handmade, r.ReproCmd, r.VerifyCmd, r.Evidence,
		strings.Join(r.WantArgv, " "), strings.Join(r.DependsOn, " ")}
	parts = append(parts, gapNotesVisible(r)...)
	return strings.Join(parts, "\n")
}

// gapInInt —— 小集合查重（与 `gapIn` 同形，只是元素是整数）。
func gapInInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// gapLinesOf —— 从该条文本里取「**该件** 紧接 `:<数字>`」的行号（升序去重 · 没有 ⇒ 空）。
// 为什么只认「件:行」而不扫全部数字：行号只有贴着那个件才有意义（不把别的件的行号算进来）。
func gapLinesOf(text, file string) []int {
	out := []int{}
	rest := text
	for {
		i := strings.Index(rest, file+":")
		if i < 0 {
			break
		}
		j := i + len(file) + 1
		k := j
		for k < len(rest) && rest[k] >= '0' && rest[k] <= '9' {
			k++
		}
		if k > j {
			if n, err := strconv.Atoi(rest[j:k]); err == nil && !gapInInt(out, n) {
				at := len(out) // 插排：本件不为一枚 import 引 sort
				for idx, v := range out {
					if n < v {
						at = idx
						break
					}
				}
				out = append(out, 0)
				copy(out[at+1:], out[at:])
				out[at] = n
			}
		}
		rest = rest[k:]
	}
	return out
}

// gapFirstLine —— 症状首行（「一行能认出来」是列表面要的；空 ⇒ 印「（空）」）。
func gapFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return gapShowText(strings.TrimSpace(s))
}

// gapShowByFile —— 「按件反查」实现（只读：不写真源、不写审计、不落缓存）。
func gapShowByFile(inv *invocation, led gapLedger, stdout, stderr io.Writer, file string) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", gapReasonPrecondition, "仓根解析不到，判不了件在不在仓里")
		fmt.Fprintf(stderr, "%s: 仓根解析不到 ⇒ 判不了 `%s` 在不在仓里（**没读到** 不是 没有 · 退码 8）\n", progName, file)
		fmt.Fprintf(stderr, "  修法：进仓根再跑；或显式给 `ZERG_REPO`\n")
		return exitBlocked
	}
	abs := filepath.Join(root, file)
	st, statErr := os.Stat(abs)
	if statErr != nil || st.IsDir() {
		inv.setErr("usage", "file_not_in_repo", "点名件不在仓里")
		fmt.Fprintf(stderr, "%s: 仓里没有这个件：%s（仓根 %s 下找不到）\n", progName, file, root)
		fmt.Fprintf(stderr, "  ★ 这是「**没读到**」不是「**没有**」：退码 2（点名面名字给错 = 用法错，与 `gap show <账里没有的 id>` 同一口径）\n")
		fmt.Fprintf(stderr, "  「件在仓里、但账内 0 条」是**另一态**（退 0）—— 两态不许混\n")
		fmt.Fprintf(stderr, "  下一步：看账 `zerg gap ls`；按编号取一条 `zerg gap show <GAP id>`\n")
		return exitUsage
	}

	// ★ 2026-09-28（缺口账 `GAP-20260928-75` / 子项 `-168`）：**两个数不是一回事** ——
	//   ① 轴面 = `unit` 键面**精确等值** N 条（与 `gap ls --unit <件>` 同口径）；
	//   ② 逐字子串面 = 正文/判据/手搓/证据/注文里提过该件 M 条（N ⊆ M）。
	//   人面按①列条目、把②的差额**如实印出来**（两个数都不删）；机器面逐字沿用②的全集（不动）。
	rows, sub, hits, extra := []map[string]string{}, []gapRecord{}, []gapRecord{}, []gapRecord{}
	linesOf := [][]int{}
	for _, r := range led.Recs {
		text := gapRecordFileText(r)
		if !strings.Contains(text, file) {
			continue
		}
		sub = append(sub, r)
		if r.Unit == file {
			hits = append(hits, r)
			linesOf = append(linesOf, gapLinesOf(text, file))
		} else {
			extra = append(extra, r)
		}
		if inv.jsonGiven {
			rows = append(rows, gapShowRow(r))
		}
	}
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, gapShowFields, rows)
	}

	fmt.Fprintf(stdout, "件: %s（在仓里 · 轴面 unit 精确等值 %d 条（另有 %d 条正文里提到但未归入该件））\n",
		file, len(hits), len(extra))
	fmt.Fprintf(stdout, "  ★ 口径：本面按 **`unit` 键面精确等值**反查（与 `gap ls --unit %s` 同口径）；第二个数 = 逐字子串面\n", file)
	fmt.Fprintf(stdout, "    （正文/判据/手搓/证据/注文里提到该件、但 `unit` 不是它）—— 两个数**都列出、都不删**；账里没有「件」这一格\n")
	if len(sub) == 0 {
		fmt.Fprintf(stdout, "  （该件上 0 条 —— 「没有」不是错：退码 0）\n")
		fmt.Fprintf(stdout, "  注：条目若只写行号不写件、或写成别的相对路径，本面抓不到 —— 那是账的写法问题，不是本面判错\n")
		return exitOK
	}
	fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s  %s\n", pad("id", 18), pad("state", 6), pad("prio", 4), pad("impact", 6), pad("want", 12), "症状首行")
	for i, r := range hits {
		fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s  %s\n", pad(r.ID, 18), pad(r.State, 6), pad(r.Prio, 4),
			pad(r.Impact, 6), pad(gapWantText(r), 12), truncateDisplay(gapFirstLine(r.Symptom), 60))
		if len(linesOf[i]) > 0 {
			strs := make([]string, 0, len(linesOf[i]))
			for _, n := range linesOf[i] {
				strs = append(strs, strconv.Itoa(n))
			}
			fmt.Fprintf(stdout, "      提到行号: %s\n", strings.Join(strs, ", "))
		}
	}
	// ★ 差额如实印出（`GAP-20260928-75`：两个数都不删）—— 轴面没归入、但正文里提到过该件的那些条。
	if len(extra) > 0 {
		fmt.Fprintf(stdout, "  另有 %d 条正文里提到该件、`unit` 不是它（**未归入该件** · 只列不减）：\n", len(extra))
		fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s  %s\n", pad("id", 18), pad("state", 6), pad("prio", 4), pad("impact", 6), pad("want", 12), "症状首行")
		for _, r := range extra {
			fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s  %s  （unit=%s）\n", pad(r.ID, 18), pad(r.State, 6), pad(r.Prio, 4),
				pad(r.Impact, 6), pad(gapWantText(r), 12), truncateDisplay(gapFirstLine(r.Symptom), 60), gapShowText(r.Unit))
		}
	}
	return exitOK
}

func cmdGapShow(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面（在任何盘面动作之前）—— 与 `ls` / `set-state` 逐字同一条收法
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2 · 设计稿 §二.1）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapShowFields, ","))
		return exitUsage
	}

	// ② 读真源（读不到 ⇒ 8 · **不许当绿**）—— 两种因机器可辨（`ledger_absent` ⇄ `precondition_missing`）
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当「没有」（退码 8）\n", gapLedgerPath())
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "真源不在盘上", led.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		gapLedgerAbsentHint(stderr, led.Path)
		return exitBlocked
	}

	// ②.5 **按件反查**（新增 · 2026-09-28 · 见件头 ★「按件反查」）——
	//   位置参数**不是缺口 id 形状**时当「件路径」用；旧面（按编号取一条）一个字不动。
	if p, ok := gapShowFileArg(inv); ok {
		return gapShowByFile(inv, led, stdout, stderr, p)
	}

	// ③ 点名面（**恰好一条** · 账内没有这个 id ⇒ 2）—— 复用同族 `gapTargetOne`，形状一字不自创；
	//    它自己会印「账内没有 X（账里 N 条）」+ 下一步「看账 : `zerg gap ls`」。
	idx, rc := gapTargetOne(inv, led, stderr, "show")
	if rc != exitOK {
		return rc
	}
	r := led.Recs[idx]

	// ④ 账内闭集自查（只判目标这一条 —— 判词第一行与 `ls` 逐字同款：账坏了不是「零命中」）
	if field, val, bad := gapOutOfRangeOne(r); bad {
		inv.setErr("failed", "ledger_out_of_range", fmt.Sprintf("第 %d 行 %s 越界", led.No[idx], field))
		fmt.Fprintf(stderr, "账内越界：%d %s\n", led.No[idx], field)
		fmt.Fprintf(stderr, "  %s 的值 %q 不在闭集里（%s）—— 真源只由命令写（防呆③ 同源）\n",
			field, val, gapClosedText(gapClosureOf(field)))
		fmt.Fprintf(stderr, "  ⇒ 判红 1（账坏了不是「零命中」；修法：`zerg gap verify` 或手工订正该行）\n")
		return exitFail
	}
	inv.changed = boolPtr(false)

	// ⑤ 机器面：照拄同族单条面（`selectJSON` ⇒ `items` 里一条 · I3 恒数组）
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, gapShowFields, gapShowRow(r))
	}

	// ⑥ 人面：**完整正文不截断**（本面存在的理由 —— `ls` 那一行只印摘要前 40 显示宽）
	fmt.Fprintf(stdout, "缺口 id  : %s\n", r.ID)
	fmt.Fprintf(stdout, "  指纹     : %s\n", gapShowText(r.FP))
	fmt.Fprintf(stdout, "  优先级   : %s\n", gapShowText(r.Prio))
	fmt.Fprintf(stdout, "  影响面   : %s\n", gapShowText(r.Impact))
	fmt.Fprintf(stdout, "  状态     : %s\n", gapShowText(r.State))
	fmt.Fprintf(stdout, "  想要     : %s\n", gapShowText(gapWantText(r)))
	fmt.Fprintf(stdout, "  正文     : %s\n", gapShowText(r.Symptom))
	fmt.Fprintf(stdout, "  手搓     : %s\n", gapShowText(r.Handmade))
	fmt.Fprintf(stdout, "  复现     : %s\n", gapShowText(r.ReproCmd))
	fmt.Fprintf(stdout, "  判据     : %s\n", gapShowText(r.VerifyCmd))
	if len(r.DependsOn) > 0 {
		fmt.Fprintf(stdout, "  依赖     : %s\n", strings.Join(r.DependsOn, " · "))
	}
	fmt.Fprintf(stdout, "  落账时刻 : %s\n", gapShowText(r.FoundAt))
	fmt.Fprintf(stdout, "  上次核   : %s\n", gapShowText(r.Verified))
	fmt.Fprintf(stdout, "  已解时刻 : %s\n", gapShowText(r.SolvedAt))
	fmt.Fprintf(stdout, "  已解证据 : %s\n", gapShowText(r.Evidence))
	vis := gapNotesVisible(r)
	fmt.Fprintf(stdout, "  口径注   : 真源里 %d 条（未作废 %d 条 · 作废标记 %d 条）\n",
		len(r.Notes), len(vis), len(r.Voids))
	for i, n := range vis {
		fmt.Fprintf(stdout, "    [%d] %s\n", i+1, n)
	}
	if len(r.Voids) > 0 {
		fmt.Fprintf(stdout, "  作废原文 : 仍在真源里（本面默认不显示）—— 要连原文看走 `--json void_notes`\n")
	}
	fmt.Fprintf(stdout, "  （本面**完整正文不截断**；`gap ls` 那一行只印摘要前 40 显示宽）\n")
	return exitOK
}

// gapClosureOf —— 按字段名取它的闭集（判词要能把闭集逐字列出来）。
func gapClosureOf(field string) []string {
	switch field {
	case "state":
		return gapStateClosed
	case "impact":
		return gapImpactClosed
	default:
		return gapPrioClosed
	}
}

// ── 作废标记的读面（`gap note --retract` 写的 · 缺口账 `GAP-20260926-233`）───────────────────────

// gapNoteFP —— 一条注的**指纹**（sha256 前 12 位）：`--retract <n|指纹>` 的第二种点法
// （序号会随追加/作废漂移，指纹钉住那一条的原文）。
func gapNoteFP(s string) string { return sha256Of([]byte(s))[:12] }

// gapVoidedN —— 该条里已被作废的注的序号集（1 起）。
func gapVoidedN(r gapRecord) map[int]bool {
	m := map[int]bool{}
	for _, v := range r.Voids {
		m[v.N] = true
	}
	return m
}

// gapNotesVisible —— 读面（人面表 · `--json notes`）**只列未被作废的注**：作废项**不显示**
// （★ 但它**仍在真源里** —— 要看得走 `--json void_notes` 那一格）。
func gapNotesVisible(r gapRecord) []string {
	void := gapVoidedN(r)
	out := []string{}
	for i, n := range r.Notes {
		if !void[i+1] {
			out = append(out, n)
		}
	}
	return out
}

// gapVoidNotesText —— `--json void_notes` 那一格：被作废的注**带原文**回吐（读面默认不显示它们）。
// 收法照 `notes` 那一格（多条按 ` ⏎ ` 连），每条形如
// `<原文> ⏎ void:true · n=<序号> · fp=<指纹> · by=<谁> · at=<时刻>`。
func gapVoidNotesText(r gapRecord) string {
	parts := []string{}
	for _, v := range r.Voids {
		orig := fmt.Sprintf("（真源里没有第 %d 条注 —— 被手改过）", v.N)
		if v.N >= 1 && v.N <= len(r.Notes) {
			orig = r.Notes[v.N-1]
		}
		parts = append(parts, fmt.Sprintf("%s ⏎ void:true · n=%d · fp=%s · by=%s · at=%s", orig, v.N, v.FP, v.By, v.At))
	}
	return strings.Join(parts, " ⏎ ")
}

// gapWantText —— `ls` 表里那一格：族 + 动作 + argv（逐字，不加解释）。
func gapWantText(r gapRecord) string {
	s := strings.TrimSpace(r.WantFam + " " + r.WantAct)
	if len(r.WantArgv) > 0 {
		s += " " + strings.Join(r.WantArgv, " ")
	}
	return s
}

func boolPtr(b bool) *bool { return &b }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── 件面三键（批1 第一片 · 2026-09-28）：`unit` / `module` / `unit_source` ─────────────────────
//
// 设计口径（批1 分类两轴）：**「件」与「模块」两个坐标** —— 值必须能在 `git ls-files` 清单里对上，
// **人工成本必须为 0**（全自动抽；兜底形态也由机器写死，不靠人填）。
//
//	① `unit`（件面 · 一条账只落**一个主件**）= 仓内相对路径；
//	② `module` = `unit` 的**目录前缀**（`scripts/gates/x.py` ⇒ `scripts/gates`）；仓根件 ⇒ `仓根`；
//	③ `unit_source` = `auto`（件面文本里抽到） / `hand`（兜底：`无件(命令面)` 或手写的 `拟(x)`）。
//
// **抽取是纯函数**（无 IO / 无全局态）：`gapUnitExtract(症状, 手搓记录, 注, 入库件清单)` ⇒ 三键。
// 清单**不在**纯函数里取 —— 唯一取数口是下面的 `gapTrackedFiles()`（**复用**族件已有的 A1 出口
// `gitpaths.List` · `FaceTracked` = `git -C <仓> ls-files -z`；本处不另写一套 git argv）。
const (
	// 兜底值（设计稿点名两形态）：命令面缺口没有「件」⇒ 明写「无件(命令面)」；人的意图形态落 `拟(x)`。
	gapUnitNoFile  = "无件(命令面)"
	gapUnitNoneMod = "无件" // 兜底那一档的 module
	gapUnitRootMod = "仓根" // 仓根件（没有目录前缀）的 module
	// `unit_source` 两值。
	gapUnitAuto = "auto"
	gapUnitHand = "hand"
	// 子串档的最短长度：太短的件名（`a.go` 类）当子串太容易误命中 ⇒ 不认（**宁可落兜底**）。
	gapUnitMinPathLen = 4
)

// gapTrackedFiles —— 「入库件清单」的唯一取数口。取不到（不在 git 仓 / git 起不来）⇒ 报 `nil`
// （调用方一律落兜底 —— **读不到不当有件**，与族件 `gitTracked` 的同向取法一致）。
func gapTrackedFiles() []string {
	root := repoRoot()
	if root == "" {
		return nil
	}
	ents, err := gitpaths.List(root, gitpaths.FaceTracked)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		if p := e.String(); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// gapUnitTokenSep —— 「一个词」的分隔面（空白 + 引号 + 逗号分号冒号 + 各类括号 + 竖线 + 中文顿号/书名号）。
const gapUnitTokenSep = " \t\r\n'\"`,;:()[]{}<>|·、（）《》“”‘’"

// gapUnitPlannedSep —— 手写 `拟(...)` 那一档的**窄**分隔面（不含圆括号 —— 括号是形态本身）。
const gapUnitPlannedSep = " \t\r\n'\"`,;|·、"

// gapExactPathInText —— 甲档（**全等**）：格子里某个词逐字等于清单里的一条路径。
// 取**最长命中**；等长取清单里**先出现**的那条（`ls-files` 顺序稳定 ⇒ 同一输入恒同结果）。
func gapExactPathInText(text string, set map[string]bool) string {
	if text == "" || len(set) == 0 {
		return ""
	}
	best := ""
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool {
		return strings.ContainsRune(gapUnitTokenSep, r)
	}) {
		if set[tok] && len(tok) > len(best) {
			best = tok
		}
	}
	return best
}

// gapSubPathInText —— 乙档（**路径子串**）：清单里的一条路径作为**子串**出现在格子里。
// 同样取最长命中（更具体的件优先）；等长取清单先出现者。`gapUnitMinPathLen` 以下的短件名不认。
func gapSubPathInText(text string, tracked []string) string {
	if text == "" {
		return ""
	}
	best := ""
	for _, p := range tracked {
		if len(p) < gapUnitMinPathLen || len(p) <= len(best) {
			continue
		}
		if strings.Contains(text, p) {
			best = p
		}
	}
	return best
}

// gapPlannedToken —— 兜底里的**手写形态**：一个词逐字形如 `拟(<…>)`（人的意图占位）⇒ 原样当 `unit`。
// ★ 本档的分隔面**不含圆括号**（`(` / `)` 是形态本身的一部分）⇒ 另用一枚窄分隔集。
func gapPlannedToken(text string) string {
	if text == "" {
		return ""
	}
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool {
		return strings.ContainsRune(gapUnitPlannedSep, r)
	}) {
		if strings.HasPrefix(tok, "拟(") && strings.HasSuffix(tok, ")") && len(tok) > len("拟()") {
			return tok
		}
	}
	return ""
}

// gapModuleOfUnit —— 模块 = `unit` 的**目录前缀**；仓根件（无目录前缀）⇒ `仓根`；兜底形态 ⇒ `无件`。
func gapModuleOfUnit(unit string) string {
	if unit == "" || strings.HasPrefix(unit, "无件") || strings.HasPrefix(unit, "拟(") {
		return gapUnitNoneMod
	}
	dir := filepath.Dir(unit)
	if dir == "." || dir == "" || dir == string(filepath.Separator) {
		return gapUnitRootMod
	}
	return dir
}

// gapUnitExtract —— **纯函数**（无 IO / 无全局态 / 无时钟）：入参 = 一条 gap 的字段集
// （`symptom` / `handmade` / `notes`）+ 现读的入库件清单；出参 = `unit` / `module` / `unit_source`。
//
// 取件顺序（设计稿：**取第一个能识别为仓内件路径的串**）：① `symptom` ② `handmade` ③ `notes`（按序连）。
// 每一格内先走**甲档全等**、三格走完再回头走**乙档子串**（全等比子串更硬 ⇒ 先扫一遍全等，避免
// 「症状里提过一句别的件、而手搓记录里才是主件」被子串抢先）。
// 三档都抽不到 ⇒ 兜底：先认手写的 `拟(x)`（`hand`），否则 `无件(命令面)` + module `无件`（`hand`）。
func gapUnitExtract(symptom, handmade string, notes []string, tracked []string) (string, string, string) {
	fields := []string{symptom, handmade, strings.Join(notes, " ⏎ ")}
	set := make(map[string]bool, len(tracked))
	for _, p := range tracked {
		set[p] = true
	}
	// 甲档：全等（三格按序）
	for _, text := range fields {
		if p := gapExactPathInText(text, set); p != "" {
			return p, gapModuleOfUnit(p), gapUnitAuto
		}
	}
	// 乙档：路径子串（三格按序）
	for _, text := range fields {
		if p := gapSubPathInText(text, tracked); p != "" {
			return p, gapModuleOfUnit(p), gapUnitAuto
		}
	}
	// 兜底：手写 `拟(x)` → 无件(命令面)
	for _, text := range fields {
		if p := gapPlannedToken(text); p != "" {
			return p, gapUnitNoneMod, gapUnitHand
		}
	}
	return gapUnitNoFile, gapUnitNoneMod, gapUnitHand
}

// ── 二.2 `zerg gap add`（写面 · 留证据）───────────────────────────────────────────────────────

// gapAddFields —— `--json` 可取字段（设计稿 §二.2：items 里带 `id` / `fp` / `state`）。
var gapAddFields = []string{"id", "fp", "state"}

// gapAddInput —— `zerg gap add` 与 `zerg gap idea promote` **共用**的入参（批2 第二片 · 2026-09-28）。
//
// ★ 设计稿 §11.3 黑体一句：「`promote` **复用 `gap add` 的既有校验（不许绕）**」⇒ 两条命令走
//
//	**同一条** `gapAddApply`：六必填（`--handmade` / `--impact` / `--want-family` / `--want-action` /
//	`--repro-cmd` / `--verify-cmd`）· `impact` 六值闭集 · `prio` 缺省 `P1` · 人面禁写 `state` ·
//	`--verify-cmd` 命令树解析 · 同 fp（幂等同内容 ⇒ 0 / 内容不同 ⇒ `14`）· 审计先落盘 ·
//	真源追加 + 写回读对拍 —— **一条判据只写一次**，promote 不另造第二条落账面。
type gapAddInput struct {
	Symptom   string
	Handmade  string
	Impact    string
	Prio      string
	WantFam   string
	WantAct   string
	WantArgv  []string
	ReproCmd  string
	VerifyCmd string
	DependsOn []string
	StateWant string
	// Fields —— 这一面 `--json` 的可取字段（`add` = `gapAddFields` · `promote` = `gapIdeaPromoteFields`）。
	Fields []string
	// Out —— 非 nil ⇒ 落账路径把「本次结果」写回这里（`promote` 拿它记池行的 `gap_id`）。
	Out *gapAddOut
}

// gapAddOut —— 落账路径回给调用方的结果（**不给 `Out` 就照旧**，`gap add` 一个字节不变）。
type gapAddOut struct {
	ID     string
	FP     string
	Landed bool // 真源**新增了一行**（幂等命中 / `--dry-run` ⇒ false）
}

// gapAddInputFromFlags —— 旗标面 → 入参（原 `cmdGapAdd` 开头那十一行取法，**一字不改**）。
func gapAddInputFromFlags(inv *invocation) gapAddInput {
	return gapAddInput{
		Symptom:   strings.TrimSpace(inv.flagVal("--symptom")),
		Handmade:  strings.TrimSpace(inv.flagVal("--handmade")),
		Impact:    strings.TrimSpace(inv.flagVal("--impact")),
		Prio:      strings.TrimSpace(inv.flagVal("--prio")),
		WantFam:   strings.TrimSpace(inv.flagVal("--want-family")),
		WantAct:   strings.TrimSpace(inv.flagVal("--want-action")),
		WantArgv:  inv.flagVals("--want-argv"),
		ReproCmd:  strings.TrimSpace(inv.flagVal("--repro-cmd")),
		VerifyCmd: strings.TrimSpace(inv.flagVal("--verify-cmd")),
		DependsOn: inv.dependsOn, // `--depends-on` 是具名旗标（收进 invocation.dependsOn，不是 kv）
		StateWant: strings.TrimSpace(inv.flagVal("--state")),
		Fields:    gapAddFields,
	}
}

func cmdGapAdd(inv *invocation, stdout, stderr io.Writer) int {
	return gapAddApply(inv, stdout, stderr, gapAddInputFromFlags(inv))
}

// gapAddApply —— **`gap add` 的唯一落账实现**（`promote` 复用同一条，不另开路径）。
func gapAddApply(inv *invocation, stdout, stderr io.Writer, in gapAddInput) int {
	symptom, handmade, impact := in.Symptom, in.Handmade, in.Impact
	wantFam, wantAct, wantArgv := in.WantFam, in.WantAct, in.WantArgv
	prio, reproCmd, verifyCmd := in.Prio, in.ReproCmd, in.VerifyCmd
	dependsOn, stateWant := in.DependsOn, in.StateWant

	// ① 用法面（在任何盘面动作之前 —— §4.1 K14 四件套那一条口径）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2 · 设计稿 §二.2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(in.Fields, ","))
		return exitUsage
	}
	missing := []string{}
	for _, m := range []struct{ name, val string }{
		{"--symptom", symptom}, {"--handmade", handmade}, {"--impact", impact},
		{"--want-family", wantFam}, {"--want-action", wantAct},
		{"--repro-cmd", reproCmd}, {"--verify-cmd", verifyCmd},
	} {
		if m.val == "" {
			missing = append(missing, m.name)
		}
	}
	if len(missing) > 0 {
		inv.setErr("usage", "missing_required", "缺必填旗标")
		fmt.Fprintf(stderr, "%s: `gap add` 缺必填旗标：%s\n", progName, strings.Join(missing, " · "))
		fmt.Fprintf(stderr, "防呆⑤：**手搓记录（--handmade）与验证命令（--verify-cmd）是两件必填** —— 缺一 ⇒ 拒收 2，不许「先收下回头补」\n")
		fmt.Fprintf(stderr, "用法：zerg gap add --symptom <一句> --handmade <命令原样> --impact <%s> --want-family <族> --want-action <动作> --repro-cmd <命令> --verify-cmd <命令>\n",
			strings.Join(gapImpactClosed, "|"))
		return exitUsage
	}
	if !gapIn(gapImpactClosed, impact) {
		inv.setErr("usage", "bad_impact", "impact 值不在六值闭集里")
		fmt.Fprintf(stderr, "%s: `--impact %s` 不在闭集里 —— 只认 %s\n", progName, impact, gapClosedText(gapImpactClosed))
		return exitUsage
	}
	if prio == "" {
		prio = "P1" // 未标 ⇒ 中档（设计稿未钉；本件取定，见件头）
	}
	if !gapIn(gapPrioClosed, prio) {
		inv.setErr("usage", "bad_prio", "prio 值不在三值闭集里")
		fmt.Fprintf(stderr, "%s: `--prio %s` 不在闭集里 —— 只认 %s\n", progName, prio, gapClosedText(gapPrioClosed))
		return exitUsage
	}
	if stateWant == "" {
		stateWant = gapStOpen
	}
	if !gapIn(gapStateHuman, stateWant) {
		inv.setErr("usage", "state_human_forbidden", "人面不许直接写 state")
		fmt.Fprintf(stderr, "%s: 人面不许直接写 `state`（防呆④ / `H-10`）—— 收到 %q 一律拒收 2\n", progName, stateWant)
		fmt.Fprintf(stderr, "  人面只许：%s（`已解` / `回归` 是**机器态**：只由 `zerg gap verify` 跑判据转）\n", gapClosedText(gapStateHuman))
		return exitUsage
	}
	if _, why := gapJudgeArgv(verifyCmd); why != "" {
		inv.setErr("usage", "verify_cmd_unresolved", why)
		fmt.Fprintf(stderr, "%s: `--verify-cmd` 解析不到命令树：%s\n", progName, why)
		fmt.Fprintf(stderr, "  v1.3 §七 规矩 2：判据必须能在命令树里解析到（解析面 = 命令树，门⑪ `T1` 保证它与矩阵 `command` 列同尺）\n")
		return exitUsage
	}

	rec := gapRecord{
		Symptom: symptom, Handmade: handmade, Impact: impact, Prio: prio, State: stateWant,
		WantFam: wantFam, WantAct: wantAct, WantArgv: wantArgv,
		ReproCmd: reproCmd, VerifyCmd: verifyCmd, DependsOn: dependsOn,
	}
	rec.Unit, rec.Module, rec.UnitSource = gapUnitExtract(rec.Symptom, rec.Handmade, rec.Notes, gapTrackedFiles())
	rec.FP = gapFingerprint(rec)
	planPath := gapLedgerPath()

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed **rc=2**，计划件走 stderr（见件头 ④）。
	//    ⚠ 判在**读真源之前**：确认档不齐 ⇒ 一行都不读、一行都不写（从不提问、也不替人猜）。
	if !inv.dryRun && !inv.yes {
		gapPlanBlock(stderr, "缺 `--yes`（D2 档）", planPath, -1, rec, false)
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件（不产生 `approver` / `approval` 两格 —— 与 `H-10`「不要人签」不冲突）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ③ 读真源（`add` 侧：件不在 = 首次创建，**不算 8**；读不到 / 解读不了 ⇒ 8 · `Q-138` 两因分档）
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；写不进就不写（退码 8）\n", planPath)
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return exitBlocked
	}

	// ④ 幂等优先（防呆②）：同 fp —— 内容同 ⇒ 0 且不新增行；内容不同 ⇒ 14 并给既有 id
	for _, r := range led.Recs {
		if r.FP != rec.FP {
			continue
		}
		if gapSameContent(r, rec) {
			rec.ID, rec.FoundAt = r.ID, r.FoundAt
			inv.changed = boolPtr(false)
			if in.Out != nil {
				in.Out.ID, in.Out.FP, in.Out.Landed = r.ID, r.FP, false // 幂等命中：真源**没**新增行
			}
			if inv.jsonGiven {
				return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
					map[string]string{"id": r.ID, "fp": r.FP, "state": r.State})
			}
			fmt.Fprintf(stdout, "已落账 %s · fp=%s （幂等命中：同 fp 同内容 ⇒ 0 且不新增行 · 防呆②）\n", r.ID, shortSHA(r.FP))
			return exitOK
		}
		msg := fmt.Sprintf("同 fp（%s）已有 %s，而内容不同 ⇒ 拒收该行", shortSHA(rec.FP), r.ID)
		if inv.dryRun {
			// 三态纪律①：`--dry-run` 恒 0（唯一会返回 0 的那一态）
			// `--json` ⇒ 机器面取代人面（同 `verify` 的干跑档 · 同本仓列表命令的老规矩）
			if inv.jsonGiven {
				inv.changed = boolPtr(false)
				return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
					map[string]string{"id": r.ID, "fp": rec.FP, "state": r.State})
			}
			gapPlanBlock(stdout, "--dry-run（冲突不落账）", planPath, len(led.Lines), rec, true)
			fmt.Fprintf(stdout, "  同名 fp  : %s（内容不同）⇒ 真跑会退 `14 conflict`（防呆②「幂等优先」）\n", r.ID)
			fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未追加真源、未写审计）\n")
			return exitOK
		}
		inv.setErr("conflict", "gap_fp_conflict", msg)
		fmt.Fprintf(stderr, "%s: %s（`14 conflict` · 防呆②「幂等优先」）\n", progName, msg)
		fmt.Fprintf(stderr, "  既有 id  : %s\n", r.ID)
		fmt.Fprintf(stderr, "  下一步   : 改它 ⇒ `zerg gap verify %s`；加新的一条 ⇒ 换 --handmade / --want-family / --want-action / --want-argv\n", r.ID)
		return exitConflict
	}

	// ⑤ 新行：分 id、取时刻
	rec.ID = gapNextID(led.Recs, time.Now().Format("20060102"))
	rec.FoundAt = gapNow()

	// ⑥ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		// `--json` ⇒ 机器面取代人面（与 `verify` 的干跑档同一条口径；设计稿 §二.2「`--json` ⇒ 六键包封，
		// `kind` = `GapAdd`，`items` 里带 `id` / `fp` / `state`」在这一态同样成立）
		if inv.jsonGiven {
			inv.changed = boolPtr(false)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"id": rec.ID, "fp": rec.FP, "state": rec.State})
		}
		gapPlanBlock(stdout, "--dry-run", planPath, len(led.Lines), rec, true)
		fmt.Fprintln(stdout, "  审计     : 计划写一行 `event=gap_ledger_written`（这一态**不写**）")
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未追加真源、未写审计）\n")
		return exitOK
	}

	// ⑦ 真写：**审计先落盘**（审计写不进 ⇒ 真源一行都不写 ⇒ 8）
	line, err := json.Marshal(rec)
	if err != nil {
		inv.setErr("failed", "ledger_encode_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 真源这一行序列化不过：%v\n", progName, err)
		return exitFail
	}
	afterRaw := append(append([]byte{}, led.Raw...), append(line, '\n')...)
	audit := gapAuditLine{
		At: time.Now().Format(time.RFC3339), Event: gapEventName, GapCmd: "add",
		GapID: rec.ID, GapFP: rec.FP, GapStateBefore: "（无：新行）", GapStateAfter: rec.State,
		GapLedgerPath: led.Path, GapBeforeSHA: sha256Of(led.Raw), GapAfterSHA: sha256Of(afterRaw),
		GapBeforeLines: len(led.Lines), GapAfterLines: len(led.Lines) + 1,
		By: gapByOf(inv), Confirm: "--yes",
	}
	if err := appendGapAudit(editAuditPath(), audit); err != nil {
		inv.setErr("blocked", "audit_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 审计写不进 ⇒ **真源一行都不写**（审计先落盘 · 退码 8）：%v\n", progName, err)
		fmt.Fprintf(stderr, "  审计落点 : %s\n", editAuditPath())
		return exitBlocked
	}
	if err := gapAppendLine(led.Path, led.Raw, line); err != nil {
		inv.setErr("blocked", "ledger_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 真源写不进（审计那一行已落盘）：%v\n", progName, err)
		return exitBlocked
	}
	// 读回对拍：真源现算 sha256 必须等于审计里那一格（逐字相同 —— 判据 3）
	back, err := os.ReadFile(led.Path)
	if err != nil || sha256Of(back) != audit.GapAfterSHA {
		inv.setErr("blocked", "ledger_readback_mismatch", "写回读对不上")
		fmt.Fprintf(stderr, "%s: 写后读回**对不上**审计里那一格（`gap_ledger_after_sha256`）⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	inv.changed = boolPtr(true)
	if in.Out != nil {
		in.Out.ID, in.Out.FP, in.Out.Landed = rec.ID, rec.FP, true // 真源**新增了一行**
	}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
			map[string]string{"id": rec.ID, "fp": rec.FP, "state": rec.State})
	}
	fmt.Fprintf(stdout, "已落账 %s · fp=%s （新增：真源 %d → %d 行）\n", rec.ID, shortSHA(rec.FP), len(led.Lines), len(led.Lines)+1)
	return exitOK
}

// ── 二.3 `zerg gap verify`（写面 · 改状态）────────────────────────────────────────────────────

// gapVerifyFields —— `--json` 可取字段（设计稿 §二.3：逐条 `id` / 判据 / `rc` / 判决 / 新 `state`）。
var gapVerifyFields = []string{"id", "judge", "judge_rc", "verdict", "state"}

// gapVerdict —— 一条目标的判决（跑完判据之后的账）。
type gapVerdict struct {
	Idx     int // 在 `led.Recs` 里的下标
	ID      string
	Judge   string // 判据（`verify_cmd` 原样）
	JudgeRC int
	Before  string
	After   string
	Out     string // 判据输出的一小段（当证据）
	Err     string // 判据跑不起来的原因（解析不到）
}

// gapVerdictLine —— 逐条那一行：`id / 判据 / rc / 判决 / 新 state`。
func (v gapVerdict) Line() string {
	return fmt.Sprintf("%s  判据=%s  rc=%d  判决=%s→%s  新 state=%s", v.ID, v.Judge, v.JudgeRC, v.Before, v.After, v.After)
}

// gapTargets —— 点名 / `--all` 两种意图译成下标集（设计稿 §二.3 的用法面全在这里）。
func gapTargets(led gapLedger, ids []string, all bool) ([]int, int, string) {
	if all {
		if len(led.Recs) == 0 {
			return nil, exitUsage, "不给结论：真源 0 行（照闸② `L5`：空转不许当绿）"
		}
		idxs := make([]int, len(led.Recs))
		for i := range led.Recs {
			idxs[i] = i
		}
		return idxs, 0, ""
	}
	var idxs []int
	for _, id := range ids {
		found := -1
		for i, r := range led.Recs {
			if r.ID == id {
				found = i
				break
			}
		}
		if found < 0 {
			return nil, exitBlocked, fmt.Sprintf("取数不到：账内没有 %s", id)
		}
		idxs = append(idxs, found)
	}
	return idxs, 0, ""
}

// gapJudgeOne —— 跑一条判据并把「判决表」译出来（设计稿 §二.3 判决 + v1.3 §七）。
func gapJudgeOne(led gapLedger, idx int) gapVerdict {
	r := led.Recs[idx]
	v := gapVerdict{Idx: idx, ID: r.ID, Judge: r.VerifyCmd, Before: r.State, After: r.State}
	rc, out, why := gapRunJudge(r.VerifyCmd)
	if why != "" {
		v.Err = why
		return v
	}
	v.JudgeRC, v.Out = rc, out
	switch {
	case r.State == gapStWontDo:
		// 人拍板「不做」⇒ 判据不许把它翻过来（只记一次核对）。
		v.After = gapStWontDo
	case rc == 0:
		v.After = gapStSolved
	case r.State == gapStSolved || r.State == gapStRegress:
		// **已解现缺** ⇒ 回归（唯一能让本命令退 1 的东西）
		v.After = gapStRegress
	default:
		v.After = r.State
	}
	return v
}

func cmdGapVerify(inv *invocation, stdout, stderr io.Writer) int {
	ids := []string{}
	for _, a := range inv.args {
		if s := strings.TrimSpace(a); s != "" {
			ids = append(ids, s)
		}
	}

	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2 · 设计稿 §二.3）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapVerifyFields, ","))
		return exitUsage
	}
	if len(inv.flagVals("--state")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--state"))
		inv.setErr("usage", "state_not_in_shape", "--state 不在 verify 的形状里")
		fmt.Fprintf(stderr, "%s: `gap verify` 不收 `--state %s`（形状 = `[<GAP id>…] [--all] [--dry-run] [--yes] [--json <字段…>]`）\n", progName, v)
		fmt.Fprintf(stderr, "  防呆④：人面不许直接写 `state`（`已解` / `回归` 是机器态，只由本命令跑判据转）\n")
		return exitUsage
	}
	if inv.all && len(ids) > 0 {
		inv.setErr("usage", "all_and_ids", "--all 与点名 id 同给")
		fmt.Fprintf(stderr, "%s: 「给范围」（`--all`）与「点名」（%s）是两种意图 ⇒ 同给 = 用法错\n", progName, strings.Join(ids, " "))
		return exitUsage
	}
	if !inv.all && len(ids) == 0 {
		inv.setErr("usage", "target_required", "缺目标")
		fmt.Fprintf(stderr, "%s: 要给目标：`zerg gap verify <GAP id>…` 或 `zerg gap verify --all`\n", progName)
		return exitUsage
	}

	// ② 缺 `--yes`（且非 `--dry-run`）：照设计稿 §二.3 ② —— **跑判据、印判决**，但**一个字不写**；rc=2。
	//    ⚠ 确认档先判（fail-closed）：真源读不到也仍是 2 —— 已经拒执了，不许把「判不了」升成别的码。
	if !inv.dryRun && !inv.yes {
		fmt.Fprintf(stderr, "计划件（缺 `--yes`（D2 档）· 零副作用 —— 未改 `state`、未写 `solved_evidence`、未写审计）\n")
		fmt.Fprintf(stderr, "  真源     : %s\n", gapLedgerPath())
		if led, err := readGapLedger(); err == nil && led.Exists {
			if idxs, _, why := gapTargets(led, ids, inv.all); why == "" {
				red := 0
				for _, i := range idxs {
					v := gapJudgeOne(led, i)
					if v.Err != "" {
						red, idxs = -1, nil
						break
					}
					fmt.Fprintf(stderr, "    %s\n", v.Line())
					if v.Before == gapStSolved || v.Before == gapStRegress {
						if v.JudgeRC != 0 {
							red++
						}
					}
				}
				if red > 0 {
					fmt.Fprintf(stderr, "  ⚠ 有 %d 条「已解现缺」⇒ **真跑会退 1**（判红只在 `--yes` 那一态可达）\n", red)
				}
			} else {
				fmt.Fprintf(stderr, "    （目标解析不了：%s —— 但缺 `--yes` 仍拒执）\n", why)
			}
		} else {
			fmt.Fprintln(stderr, "    （真源没读到 —— 但缺 `--yes` 仍拒执；这一态不退 8：确认档不齐已经先挡了）")
		}
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ③ 读真源（`--dry-run` 与 `--yes` 都要它：读不到 ⇒ 8 · 不许当绿）· 两种因**机器可辨**（`Q-138`）
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当绿（退码 8）\n", gapLedgerPath())
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "真源不在盘上", led.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		gapLedgerAbsentHint(stderr, led.Path)
		return exitBlocked
	}
	idxs, rc, why := gapTargets(led, ids, inv.all)
	if why != "" {
		if rc == exitBlocked {
			inv.setErr("blocked", "target_not_found", why)
		} else {
			inv.setErr("usage", "no_conclusion", why)
		}
		fmt.Fprintf(stderr, "%s: %s（退码 %d）\n", progName, why, rc)
		if rc == exitUsage {
			fmt.Fprintf(stderr, "  `--all` 而真源实际 0 行 = 空转 ⇒ 不给结论（闸② `L5` 同款）\n")
		}
		return rc
	}

	// ★ 只读闸（`GAP-20260927-247` · P0）—— 在**跑任何判据之前**先全扫一遍：判据里有**写面**
	// （危险档且没带 `--dry-run`）⇒ **一条都不跑**、不给结论（退码 8）。放在「跑判据」之前而不是
	// 逐条判，是为了「**要么全跑、要么一条不跑**」——半跑等于把写面执行到一半。
	for _, i := range idxs {
		if why := gapWriteFaceWhy(led.Recs[i].VerifyCmd); why != "" {
			inv.setErr("blocked", "verify_cmd_write_face", why)
			fmt.Fprintf(stderr, "%s: 判据是写面（%s）：%s\n", progName, led.Recs[i].ID, why)
			fmt.Fprintf(stderr, "  ⇒ 一条判据都未执行、不给结论（退码 8；真源一个字节未改）\n")
			return exitBlocked
		}
	}

	// ④ 跑判据（先全部解析、再跑 —— 判据不可跑 ⇒ 2，一个字节都不写）
	vs := make([]gapVerdict, 0, len(idxs))
	for _, i := range idxs {
		v := gapJudgeOne(led, i)
		if v.Err != "" {
			inv.setErr("usage", "verify_cmd_unresolved", v.Err)
			fmt.Fprintf(stderr, "%s: 判据不可跑（%s）：%s\n", progName, v.ID, v.Err)
			fmt.Fprintf(stderr, "  ⇒ 不给结论（退码 2）；真源**一个字节未改**\n")
			return exitUsage
		}
		vs = append(vs, v)
	}

	red := 0
	for _, v := range vs {
		if (v.Before == gapStSolved || v.Before == gapStRegress) && v.JudgeRC != 0 {
			red++
		}
	}

	// ⑤ `--dry-run`：跑判据、逐条印判决 ⇒ rc=**0**（唯一会返回 0 的那一态 · 零副作用）
	if inv.dryRun {
		rows := make([]map[string]string, 0, len(vs))
		for _, v := range vs {
			rows = append(rows, map[string]string{
				"id": v.ID, "judge": v.Judge, "judge_rc": strconv.Itoa(v.JudgeRC),
				"verdict": v.Before + "→" + v.After, "state": v.After,
			})
		}
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
		}
		fmt.Fprintf(stdout, "判决（--dry-run · 零副作用 —— 未改 `state`、未写审计）\n")
		for _, v := range vs {
			fmt.Fprintf(stdout, "  %s\n", v.Line())
		}
		if red > 0 {
			fmt.Fprintf(stdout, "  判红预告 : 有 %d 条「已解现缺」⇒ 真跑会退 1（判红只在 `--yes` 那一态可达）\n", red)
		}
		fmt.Fprintf(stderr, "（--dry-run：只跑判据、只印判决 · 零副作用 —— 未改 `state`、未写 `solved_evidence`、未写审计）\n")
		return exitOK
	}

	// ⑥ `--yes`：真改 —— ① 算出新件 ② **审计先落盘** ③ 写真源 ④ 读回对拍
	now := gapNow()
	newRecs := make(map[int]gapRecord, len(vs))
	for _, v := range vs {
		r := led.Recs[v.Idx]
		r.Verified = now
		switch {
		case v.After == gapStSolved && v.Before != gapStSolved:
			r.State = gapStSolved
			r.SolvedAt = now
			r.Evidence = gapEvidenceText(v, now)
			r.Verified = now
		case v.After == gapStRegress:
			r.State = gapStRegress // 历史（`solved_at` / `solved_evidence`）**不删**（追加式口径）
		default:
			r.State = v.After
		}
		newRecs[v.Idx] = r
	}
	lines := make([]string, 0, len(led.Lines))
	for i, raw := range led.Lines {
		if r, ok := newRecs[i]; ok {
			enc, err := json.Marshal(r)
			if err != nil {
				inv.setErr("failed", "ledger_encode_failed", err.Error())
				fmt.Fprintf(stderr, "%s: 真源第 %d 行序列化不过：%v\n", progName, led.No[i], err)
				return exitFail
			}
			lines = append(lines, string(enc))
			continue
		}
		lines = append(lines, raw) // 未动的行**逐字节照原样**（历史行不变）
	}
	body := []byte{}
	for _, ln := range lines {
		body = append(body, []byte(ln+"\n")...)
	}
	for _, v := range vs {
		audit := gapAuditLine{
			At: time.Now().Format(time.RFC3339), Event: gapEventName, GapCmd: "verify",
			GapID: v.ID, GapFP: led.Recs[v.Idx].FP, GapStateBefore: v.Before, GapStateAfter: v.After,
			GapLedgerPath: led.Path, GapBeforeSHA: sha256Of(led.Raw), GapAfterSHA: sha256Of(body),
			GapBeforeLines: len(led.Lines), GapAfterLines: len(lines),
			By: gapByOf(inv), Confirm: "--yes",
		}
		if err := appendGapAudit(editAuditPath(), audit); err != nil {
			inv.setErr("blocked", "audit_unwritable", err.Error())
			fmt.Fprintf(stderr, "%s: 审计写不进 ⇒ **真源一个字节不改**（审计先落盘 · 退码 8）：%v\n", progName, err)
			fmt.Fprintf(stderr, "  审计落点 : %s\n", editAuditPath())
			return exitBlocked
		}
	}
	if err := gapWriteLedger(led.Path, lines); err != nil {
		inv.setErr("blocked", "ledger_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 真源写不进（审计那几行已落盘）：%v\n", progName, err)
		return exitBlocked
	}
	back, err := os.ReadFile(led.Path)
	if err != nil || sha256Of(back) != sha256Of(body) {
		inv.setErr("blocked", "ledger_readback_mismatch", "写回读对不上")
		fmt.Fprintf(stderr, "%s: 写后读回对不上（审计里的 `gap_ledger_after_sha256`）⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}

	rows := make([]map[string]string, 0, len(vs))
	for _, v := range vs {
		rows = append(rows, map[string]string{
			"id": v.ID, "judge": v.Judge, "judge_rc": strconv.Itoa(v.JudgeRC),
			"verdict": v.Before + "→" + v.After, "state": v.After,
		})
	}
	changed := false
	for _, v := range vs {
		if v.Before != v.After {
			changed = true
		}
	}
	inv.changed = boolPtr(changed)
	if red > 0 {
		// 判红：唯一能让本命令退 1 的东西（`已解现缺` ⇒ 记 `回归`）
		for _, v := range vs {
			if (v.Before == gapStSolved || v.Before == gapStRegress) && v.JudgeRC != 0 {
				inv.setErr("failed", "regressed", v.ID+" 已解现缺")
				fmt.Fprintf(stderr, "已解现缺：%s 判据 rc=%d ⇒ 该条 state → `回归`\n", v.ID, v.JudgeRC)
				break
			}
		}
	}
	if inv.jsonGiven {
		if rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows); rc != exitOK {
			return rc
		}
		if red > 0 {
			return exitFail
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "判决（--yes · 真改了 %d 条 · 审计已落 %d 行）\n", len(vs), len(vs))
	for _, v := range vs {
		fmt.Fprintf(stdout, "  %s\n", v.Line())
	}
	if red > 0 {
		fmt.Fprintf(stdout, "  判红     : 有 %d 条「已解现缺」⇒ 退码 1（该条已记 `回归`）\n", red)
		return exitFail
	}
	return exitOK
}

// gapEvidenceText —— `solved_evidence` 的取值（判据那一跑的原样 + rc + 时刻；**只留指纹级的短证据**）。
func gapEvidenceText(v gapVerdict, at string) string {
	s := fmt.Sprintf("%s ⇒ rc=0 @ %s", v.Judge, at)
	if v.Out != "" {
		out := strings.ReplaceAll(v.Out, "\n", " ⏎ ")
		s += " · 输出=" + truncateDisplay(out, 120)
	}
	return s
}

// ── 二.4 `zerg gap backfill-unit`（批1 第二片 · **存量回填三键** · 2026-09-28）─────────────────
//
// 病（逐字）：批1 第一片把 `unit` / `module` / `unit_source` 三键落进了**新落行**的写口
// （`gap add` 里那一句 `rec.Unit, rec.Module, rec.UnitSource = gapUnitExtract(…)`），
// 而**存量 891 条**账里没有这三键 ⇒ 「按件 / 按模块分类」在存量面上**没有数据**（只能人读 891 行）。
//
// 本面的口径（逐条 · **不新造形状**）：
//
//	① `--dry-run`（只读面）：印两数 —— **可抽到**（`unit_source=auto`）N 与**落兜底**（`hand`）M，
//	   且 **N + M == 账内总条数**（抽取函数只有这两档 ⇒ 两数之和恒等于总条数，不是巧合）；
//	   再列前 5 条**待改**（`id` + 抽到的 `unit` / `module`）。
//	② `--yes`（实写面）：逐条**只加三键** —— 其余键名与值**逐字节不变**、行序不变、末行换行不变。
//	   做法 = **外科式插入**（`gapUnitInjectLine`：在原文那一行的**最后一个** `}` 之前插入），
//	   **不走**「解析 → 重序列化」✗（后者会把键序 / 转义形态一起改写 ⇒ 违反逐字节不变）。
//	③ **幂等**：已带三键的行**跳过**（再跑一次 ⇒ 「新增 0 行改动」+ 真源 sha256 逐字不变）。
//	④ 值形态**逐字复用**第一片的 `gapUnitExtract`（甲档全等 / 乙档路径子串 / 兜底手写 `拟(x)` 或
//	   `无件(命令面)` + module `无件` + `unit_source=hand`）—— 本面**不另写一套抽取**。
//	⑤ 三态照本族写面（`add` / `verify` / `set-state` / `note`）：`--dry-run` 恒 0 · 缺 `--yes`
//	   fail-closed 2（计划件走 stderr）· `--yes` 才真写；**审计先落盘**（写不进 ⇒ 真源一个字节不改 ⇒ 8）。
//	   退码**一律引现有表**（`exitcodes.go`）：0 / 2 / 8 —— 本面**不取新号**。
//	⑥ 读 / 写 / 原子替换 / 取数**一律复用族内现成口子**：`readGapLedger` · `gapWriteLedger` ·
//	   `appendGapAudit` · `gapTrackedFiles` · `gapUnitExtract` · `gapModuleOfUnit` · `gapByOf`
//	   （本面**不新写**第二套读写账、第二套抽取）。
//
// ★ 两个**本片不覆盖**的（回执里照实点名）：① 「已解 / 不做」条要不要回填（本面**逐条都填** ——
// 分类轴与状态无关）② 只回填某一段 / 某目录（本面**只做整本账**；收窄要加旗标 = 新形状 ⇒ 不半落 ✗）。

// gapBackfillFields —— `--json` 可取字段（本面自报读数）。
var gapBackfillFields = []string{"total", "extractable", "fallback", "need_change", "changed_rows"}

// gapJSONStr —— 把一段字符串编成一个 JSON 字面量（**不转义 HTML**：`<` / `>` / `&` 原样）。
// 为什么不用 `json.Marshal` 直接上：它默认把这三枚转成 `\u00xx`；本面是**外科式插入**，
// 形态越少变越好（与同族 `gap add` 那一支的默认形态也一致）。
func gapJSONStr(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return `""`
	}
	return strings.TrimRight(b.String(), "\n")
}

// gapUnitInjectLine —— **外科式**把 `adds`（有序的「键 → 值」对）插进原文那一行：
// 只在**最后一个** `}` 之前插入，其余字节**逐字照原样**（含 `}` 之后的尾空白）。
// 形态不对（没有 `}` / `}` 之后还有非空字节）⇒ **拒写**（不猜、不重排、不丢字节）。
func gapUnitInjectLine(raw string, adds [][2]string) (string, error) {
	if len(adds) == 0 {
		return raw, nil
	}
	end := strings.LastIndex(raw, "}")
	if end < 0 {
		return "", fmt.Errorf("原文那一行找不到结尾的 `}`（形态不对）⇒ 拒写（不猜）")
	}
	if tail := raw[end+1:]; strings.TrimSpace(tail) != "" {
		return "", fmt.Errorf("`}` 之后还有非空字节（%q）⇒ 不认这一行的形态，拒写", tail)
	}
	head := strings.TrimRight(raw[:end], " 	")
	var b strings.Builder
	b.WriteString(head)
	sep := "," // 空对象（`{`）⇒ 第一对不加逗号
	if strings.HasSuffix(head, "{") {
		sep = ""
	}
	for _, kv := range adds {
		b.WriteString(sep)
		b.WriteString(gapJSONStr(kv[0]))
		b.WriteString(":")
		b.WriteString(gapJSONStr(kv[1]))
		sep = ","
	}
	b.WriteString("}")
	b.WriteString(raw[end+1:])
	return b.String(), nil
}

// gapBackfillRow —— 一条**待改**（预览面与实写面**同一份算法**：不写第二套）。
type gapBackfillRow struct {
	Idx  int         // 在 `led.Lines` / `led.Recs` 里的下标（**行序不变**的锚）
	ID   string      // 预览用
	Unit string      // 抽到的 unit（手写形态或兜底值原样）
	Mod  string      // 抽到的 module
	Adds [][2]string // 逐条要插的「键 → 值」（已带三键 ⇒ 空 ⇒ 跳过）
}

// gapBackfillPlan —— 整本账算一遍：待改清单 + 两数（`auto` 可抽到 / `hand` 落兜底）。
// 三键齐 ⇒ `Adds` 空 ⇒ 不进待改（**幂等**那一档）；两数**恒** `auto + hand == len(led.Recs)`。
func gapBackfillPlan(led gapLedger, tracked []string) (rows []gapBackfillRow, auto, hand int) {
	for i, r := range led.Recs {
		u, m, s := gapUnitExtract(r.Symptom, r.Handmade, r.Notes, tracked)
		if s == gapUnitAuto {
			auto++
		} else {
			hand++
		}
		adds := [][2]string{}
		if r.Unit == "" {
			adds = append(adds, [2]string{"unit", u})
		}
		if r.Module == "" {
			if r.Unit != "" { // 件面已在位 ⇒ 模块只是它的目录前缀，不必再抽一次
				m = gapModuleOfUnit(r.Unit)
			}
			adds = append(adds, [2]string{"module", m})
		}
		if r.UnitSource == "" {
			if r.Unit != "" { // 件面已在位而来源未记 ⇒ 那不是本面抽的（不冒认 `auto`）
				s = gapUnitHand
			}
			adds = append(adds, [2]string{"unit_source", s})
		}
		if len(adds) > 0 {
			rows = append(rows, gapBackfillRow{Idx: i, ID: r.ID, Unit: u, Mod: m, Adds: adds})
		}
	}
	return rows, auto, hand
}

func cmdGapBackfillUnit(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面（在任何盘面动作之前）—— 与 `ls` / `show` / `add` 逐字同一条收法
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapBackfillFields, ","))
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "no_positional_args", "本面不吃位置参数")
		fmt.Fprintf(stderr, "%s: `gap backfill-unit` 不吃位置参数 —— 范围 = **整本账**（本片不收窄）\n", progName)
		return exitUsage
	}

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed **rc=2**，计划件走 stderr（件头 ④ 逐字同款）。
	//    ⚠ 判在**读真源之前**：确认档不齐 ⇒ 一行都不读、一行都不写（从不提问、也不替人猜）。
	if !inv.dryRun && !inv.yes {
		fmt.Fprintf(stderr, "计划件（缺 `--yes`（D2 档）· 零副作用 —— 真源未改、未写审计）\n")
		fmt.Fprintf(stderr, "  真源     : %s（未读 —— 缺 `--yes` ⇒ 不执行）\n", gapLedgerPath())
		fmt.Fprintf(stderr, "  要动的面 : 逐条**只加** `unit` / `module` / `unit_source` 三键（其余键名与值逐字节不变 · 行序不变）\n")
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件（与 `H-10`「不要人签」不冲突）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ③ 读真源（与 `ls` / `show` 逐字同一套收法 · 两枚机器可辨 reason —— 「读不到」不许当绿）
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当绿（退码 8）\n", gapLedgerPath())
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "真源不在盘上", led.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		gapLedgerAbsentHint(stderr, led.Path)
		return exitBlocked
	}

	// ④ 逐条现算（抽取 = 纯函数 `gapUnitExtract`；件面清单唯一的取数口 = `gapTrackedFiles`）
	tracked := gapTrackedFiles()
	rows, auto, hand := gapBackfillPlan(led, tracked)
	need := len(rows)
	total := len(led.Recs)

	// ⑤ `--dry-run`：只印读数 + 前 5 条预览（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, gapBackfillFields, map[string]string{
				"total": strconv.Itoa(total), "extractable": strconv.Itoa(auto),
				"fallback": strconv.Itoa(hand), "need_change": strconv.Itoa(need),
				"changed_rows": "0",
			})
		}
		gapBackfillReadout(stdout, led, total, auto, hand, need, tracked)
		fmt.Fprintf(stderr, "（--dry-run：只读 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}

	// ⑥ `--yes`：真写 —— ① 外科式造新件 ② 审计先落盘 ③ 写真源 ④ 读回对拍
	lines := make([]string, len(led.Lines))
	copy(lines, led.Lines)
	for _, r := range rows {
		nl, err := gapUnitInjectLine(led.Lines[r.Idx], r.Adds)
		if err != nil {
			inv.setErr("blocked", "ledger_shape_bad", err.Error())
			fmt.Fprintf(stderr, "%s: 真源第 %d 行（%s）：%v\n", progName, led.No[r.Idx], r.ID, err)
			fmt.Fprintf(stderr, "  ⇒ 拒写：**一个字节都不改**（不猜、不重排）；逐字看那一行 `%s gap show %s`\n", progName, r.ID)
			return exitBlocked
		}
		// 读回自校：插进去的那一行仍要解得开、且三键读回来的值 = 要插的值（外科式插入不许改语义）
		var chk gapRecord
		if err := json.Unmarshal([]byte(nl), &chk); err != nil {
			inv.setErr("blocked", "ledger_shape_bad", err.Error())
			fmt.Fprintf(stderr, "%s: 真源第 %d 行插入后解不开（%v）⇒ 拒写\n", progName, led.No[r.Idx], err)
			return exitBlocked
		}
		for _, kv := range r.Adds {
			got := map[string]string{"unit": chk.Unit, "module": chk.Module, "unit_source": chk.UnitSource}[kv[0]]
			if got != kv[1] {
				inv.setErr("blocked", "ledger_shape_bad", "插入后读回对不上")
				fmt.Fprintf(stderr, "%s: 真源第 %d 行 `%s` 读回 %q ≠ 要插的 %q ⇒ 拒写\n", progName, led.No[r.Idx], kv[0], got, kv[1])
				return exitBlocked
			}
		}
		lines[r.Idx] = nl
	}
	body := []byte{}
	for _, ln := range lines {
		body = append(body, []byte(ln+"\n")...) // 未动的行**逐字节照原样** · 行序不变 · 末行换行不变
	}
	if need > 0 {
		audit := gapAuditLine{
			At: time.Now().Format(time.RFC3339), Event: gapEventName, GapCmd: "backfill-unit",
			GapID: "（整本账）", GapFP: "（不点名单条）",
			GapStateBefore: "（不改 state）", GapStateAfter: "（不改 state）",
			GapLedgerPath: led.Path, GapBeforeSHA: sha256Of(led.Raw), GapAfterSHA: sha256Of(body),
			GapBeforeLines: len(led.Lines), GapAfterLines: len(lines),
			By: gapByOf(inv), Confirm: "--yes",
		}
		if err := appendGapAudit(editAuditPath(), audit); err != nil {
			inv.setErr("blocked", "audit_unwritable", err.Error())
			fmt.Fprintf(stderr, "%s: 审计写不进 ⇒ **真源一个字节不改**（审计先落盘 · 退码 8）：%v\n", progName, err)
			fmt.Fprintf(stderr, "  审计落点 : %s\n", editAuditPath())
			return exitBlocked
		}
		if err := gapWriteLedger(led.Path, lines); err != nil {
			inv.setErr("blocked", "ledger_unwritable", err.Error())
			fmt.Fprintf(stderr, "%s: 真源写不进（审计那一行已落盘）：%v\n", progName, err)
			return exitBlocked
		}
		back, err := os.ReadFile(led.Path)
		if err != nil || sha256Of(back) != sha256Of(body) {
			inv.setErr("blocked", "ledger_readback_mismatch", "写回读对不上")
			fmt.Fprintf(stderr, "%s: 写后读回对不上（审计里的 `gap_ledger_after_sha256`）⇒ 不给结论（退码 8）\n", progName)
			return exitBlocked
		}
	}
	inv.changed = boolPtr(need > 0)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, gapBackfillFields, map[string]string{
			"total": strconv.Itoa(total), "extractable": strconv.Itoa(auto),
			"fallback": strconv.Itoa(hand), "need_change": strconv.Itoa(need),
			"changed_rows": strconv.Itoa(need),
		})
	}
	if need == 0 {
		fmt.Fprintf(stdout, "新增 0 行改动（%d 条已带三键 ⇒ 跳过 · 幂等命中）· 真源 %d 行逐字节不变\n", total, len(led.Lines))
		return exitOK
	}
	fmt.Fprintf(stdout, "已回填：新增 %d 行改动（真源 %d 行 · 行序不变 · 其余键名与值逐字节不变）· 审计已落 1 行\n",
		need, len(led.Lines))
	gapBackfillReadout(stdout, led, total, auto, hand, need, tracked)
	return exitOK
}

// gapBackfillReadout —— 两数 + 对账 + 前 5 条预览（**干跑与实写同一份读法** ⇒ 两态读数可比）。
func gapBackfillReadout(w io.Writer, led gapLedger, total, auto, hand, need int, tracked []string) {
	fmt.Fprintf(w, "  真源     : %s（现有 %d 行）\n", led.Path, len(led.Lines))
	fmt.Fprintf(w, "  账内总条数: %d\n", total)
	fmt.Fprintf(w, "  可抽到   : %d（`unit_source=auto` —— 件面文本里抽到仓内路径）\n", auto)
	fmt.Fprintf(w, "  落兜底   : %d（`unit_source=hand` —— 手写 `拟(x)` 或 `无件(命令面)` + module `无件`）\n", hand)
	ok := "✓"
	if auto+hand != total {
		ok = "✗（**不对账**：抽取面坏了 —— 不给结论）"
	}
	fmt.Fprintf(w, "  对账     : 可抽到 %d + 落兜底 %d == 账内总条数 %d %s\n", auto, hand, total, ok)
	fmt.Fprintf(w, "  待改     : %d 行（已带三键 %d 行 ⇒ 跳过 · 幂等）\n", need, total-need)
	fmt.Fprintf(w, "  入库件清单: %d 条（`git ls-files` · 取不到 ⇒ 0 ⇒ 全落兜底 —— 「读不到」不当「没有件」）\n", len(tracked))
	fmt.Fprintln(w, "  前 5 条预览（id + 抽到的 unit/module）：")
	if need == 0 {
		fmt.Fprintln(w, "    （待改 0 条 ⇒ 无预览）")
		return
	}
	n := need
	if n > 5 {
		n = 5
	}
	shown := 0
	for _, r := range gapBackfillPreviewRows(led, tracked, 5) {
		fmt.Fprintf(w, "    %s  unit=%s  module=%s\n", r.ID, r.Unit, r.Mod)
		shown++
	}
	if shown < n {
		fmt.Fprintf(w, "    （只列出 %d 条）\n", shown)
	}
}

// gapBackfillPreviewRows —— 预览面取值口（**与实写面同一份算法** `gapBackfillPlan` · 不另算）。
func gapBackfillPreviewRows(led gapLedger, tracked []string, n int) []gapBackfillRow {
	rows, _, _ := gapBackfillPlan(led, tracked)
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// ── 批1 第四片：`gap status` 排行面（按件 / 按模块排行 · 只读）────────────────────────────────
//
// 口径（照拄同族既有面 · 不自创第二套）：
//	① 两枚**新旗标**：`--by <unit|module>`（缺省 `unit`）与 `--top <N>`（缺省 `gapStatusDefaultTop`；
//	   上限同 `gapLsRowCap` —— 与 `gap ls` / `find` / `code find` 同一个数，不自选第三个数字）。
//	   与 `--state` / `--prio` / `--impact` **可叠加**（AND），判据压在与 `cmdGapLs` **同序**的那个
//	   循环里（三枚闭集旗标的自查与判词**复用** `gapIn` / `gapStateClosed` / `gapClosedText`）。
//	② 每个桶给四列：**未闭**（`state` 仍缺）/ **已解**（`state` 已解）/ **净**（未闭 − 已解 · 可为负）/
//	   **近邻量化**（P0/P1/P2 计数 —— 取值口 = 那一条自己的 `prio`）。
//	③ **两个排序键**（都写进输出表头）：主键 = **未闭**条数降序；次键 = **近邻量化**降序
//	   （先 P0、再 P1、再 P2）⇒ 同「未闭」的几个桶按严重度排（`P0` 多的在前）。
//	④ **对账等式自校**（现算 · 不许自填数字）：Σ(有件桶未闭) + 无件桶 == 账内仍缺总数；
//	   不成立 ⇒ `inv.warnf` 点名差数（进 `warnings[]`）+ 人面另起一行说明。
//	⑤ **查询元**：人面与机器面都带 `query_ts`（现读时刻）与 `ledger_sha16`（账本身份）——
//	   不同时刻的两个排行据此才可比；截断自报（`total` 桶数 / `hits` 本页桶数 / `truncated`）。
//	⑥ 信封：顶层六键冻结（`envelopeKeys` · 一个不多一个不少）⇒ 全走 `meta` 子键
//	   （`metaAddJSON` / `metaAddStr` 既有口子）· 只读面（不登记 `danger`）。

// gapStatusDefaultTop —— `--top` 缺省一页桶数（20）。
const gapStatusDefaultTop = 20

// gapStatusBy 三值（`--by` 的闭集）—— 前两值与件面三键的 `unit` / `module` **同名同义**
// （不自造第二个词）；`tier` 是**批4 第二片**新增的第三值（按判据分级分桶 · 只读面）。
const (
	gapStatusByUnit   = "unit"
	gapStatusByModule = "module"
	// gapStatusByTier —— **批4 第二片（2026-09-28）**：按**判据分级**分桶。
	// 行 = 三桶闭集（`真判据` / `占位` / `无` ⇒ `gapVerifyTierReal` / `gapVerifyTierPlaceholder` /
	// `gapVerifyTierNone`，与 `family_gap_state.go` 的 verify-one 同一份常量 —— 不自造第二套字面）；
	// 列 = **条数** + 其中**仍缺** + 其中**已解**；表尾固定两行（守恒式自校 + 占位桶只报告）。
	gapStatusByTier = "tier"
	// gapStatusByDay —— **时间维度面（缺口账 `GAP-20260929-34`）**：按**日**分桶（只读面）。
	// 行 = 日桶（`found_at` / `solved_at` 两轴共用的键 = `YYYY-MM-DD` · 升序）；
	// 列 = **新增**（`found_at` 落那一天）+ **销案**（`solved_at` 落那一天）+ **净**；
	// 为什么另开一值而不是改 unit/module/tier：那三轴量的是「谁在欠」，本轴量的是「何时在变」
	// —— 「新增 vs 销案」两条线是**趋势**，与三轴的静态分布不是同一维（混一轴两边都不清楚）。
	gapStatusByDay = "day"
	// gapStatusByFamily —— **片 FAM-1（2026-09-29）**：按账**自带的归属轴** `want_family` 分桶（只读面）。
	// 与 `--by unit` / `--by module` **同形状、同排序键、同表尾对账自校**（行 = 族桶 · 列 = 未闭 / 已解 / 净 / 近邻量化）。
	// 为什么另开一值而不并进 unit/module：那两轴量的是「**哪一件**在欠」（件面），本轴量的是「**哪一族**在欠」——
	// `want_family` 是每条账**自带、不经推断**的归属；`unit` 兜底的那 529 条里只有 16% 抽得到仓内路径。
	// ★ 脏值口径：族值**原值原样成桶**（同义异写不合并 —— 合并/归并不许由渲染面代办）；
	//   空串 / 缺键 ⇒ 单列一桶 `gapFamNone`（逐字点名 · 不静默少算 · 不并进任何既有桶）。
	gapStatusByFamily = "family"
)

// gapFamNone —— 族桶里 `want_family` 缺失 / 空串那一格的名字（「原值原样成桶」的对偶：空值也要有名有姓）。
const gapFamNone = "（空 want_family）"

// gapUnitNoFileSub —— 兜底件 `无件(命令面)`（`gapUnitNoFile`）的**渲染键**：按**该条自己的 `impact`**
// 细分成 `无件(<impact>)`（六值：命令面 / 门禁面 / 文档面 / 公开面 / 换件面 / 归档面）。
// ★ **只在渲染面改名**：账内 `unit` 一个字节不动（不改账、不写账）。
// ★ 逐字：`impact == 命令面` 时返回值**逐字等于** `gapUnitNoFile` ⇒ 那一半既有读数零变；其余五值才分流。
// ★ `impact` 取不到（不该有 —— 账内闭集自查已挡）⇒ 点名而不是冒充某个面。
func gapUnitNoFileSub(impact string) string {
	if strings.TrimSpace(impact) == "" {
		return gapUnitNoFile + "（空 impact）"
	}
	return "无件(" + impact + ")"
}

// gapDayNone —— 日桶里**没有入账时刻**那一格的名字（`found_at` 缺 / 短于 10 字 ⇒ 进不了日轴）。
// 逐字点名而不是丢掉：丢一条 = 日轴 Σ ≠ 账内条数 ⇒ 两条线当场对不上（不许静默少算）。
const gapDayNone = "（无入账时刻）"

// gapDayOf —— 一条时刻串的**日**（`YYYY-MM-DD` · `RFC3339Nano` 的前 10 字节）。
// 取不到（空 / 短于 10）⇒ 空串（调用方自己点名兜底桶）——本函数**不判**合法性。
func gapDayOf(ts string) string {
	ts = strings.TrimSpace(ts)
	if len(ts) < 10 {
		return ""
	}
	return ts[:10]
}

// gapStatusTierOrder —— 三桶的**固定**输出序（不是排序键算出来的）：闭集三值按「有效 → 占位 → 无」
// 排（判据有效性递减）。三桶**穷尽且互斥** ⇒ 任何一条必落且只落一桶（守恒式的地基）。
var gapStatusTierOrder = []string{gapVerifyTierReal, gapVerifyTierPlaceholder, gapVerifyTierNone}

// gapStatusCmdResolvable —— 「**能被命令树解析**」的判据（分级只用这一条腿）：
// 剥 `zerg` 前缀后拿**命令树本身**（`resolve` · 与 `gapJudgeArgv` 同一棵树）解析。
// ★ 为什么不用 `gapJudgeArgv` 整条：它还叠着两枚**与「解析得到吗」无关**的闸（写面 / 自递归）
//
//	—— 那两枚判的是「**能不能跑**」，本面判的是「**这条判据是不是真判据的形态**」⇒ 只取解析那一腿。
func gapStatusCmdResolvable(cmdStr string) bool {
	fields := strings.Fields(cmdStr)
	if len(fields) == 0 {
		return false
	}
	if filepath.Base(fields[0]) == progName {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return false
	}
	c, _ := resolve(fields)
	return c != nil
}

// gapStatusTierCompute —— **未落库**时现读 `verify_cmd` 现算分级（三值 · 逐字判据）：
//
//	① 去空白后空串（= 空串或**无键** · 两者在结构体上同形）⇒ `无`；
//	② 串里含**占位标记字样**「占位」⇒ `占位`（与 `family_gap_state.go:gapVerifyOneTier` 同一口径）；
//	③ 其余**且能被命令树解析**（`gapStatusCmdResolvable`）⇒ `真判据`；
//	④ 其余（有字却解析不到 ⇒ 跑不到一个结论）⇒ 归 `占位`：**三级必须穷尽**
//	   （三桶之和 == 总账 的地基），而它既不是「无」（有字）也不是「真判据」（跑不动）。
func gapStatusTierCompute(cmd string) string {
	if strings.TrimSpace(cmd) == "" {
		return gapVerifyTierNone
	}
	if strings.Contains(cmd, "占位") {
		return gapVerifyTierPlaceholder
	}
	if !gapStatusCmdResolvable(cmd) {
		return gapVerifyTierPlaceholder
	}
	return gapVerifyTierReal
}

// gapStatusTierOf —— 一条账的判据分级（本面唯一分档处）。第二返回值 = 该条**落库的
// `verify_tier` 越界**（不是三值闭集里的值）⇒ 不采信、落回现算（表尾**只报告**点名条数）。
//
// 口径（逐字 · 与 verify-one 已落库的值**不许矛盾**）：
//
//	· `verify_tier` **已填**（且在闭集里）⇒ **用它**（落库那一刻的判据说了算 —— 本面不回头改写历史）；
//	· 未填（缺键 / 空串）⇒ 现读 `verify_cmd` 现算（`gapStatusTierCompute`）。
//
// 自洽性：verify-one 只在 `gapJudgeArgv` 通过时才写 `真判据` ⇒ 已落库的 `真判据` 必解析得到；
// 已落库的 `占位` 必含「占位」字样；已落库的 `无` 必为空串 ⇒ 两条口径在闭集上不打架。
func gapStatusTierOf(r gapRecord) (string, bool) {
	if t := strings.TrimSpace(r.VerifyTier); t != "" {
		if gapIn(gapStatusTierOrder, t) {
			return t, false
		}
		return gapStatusTierCompute(r.VerifyCmd), true
	}
	return gapStatusTierCompute(r.VerifyCmd), false
}

// gapStatusFields —— `gap status` 的 `--json` 可取字段（与命令树里的 `fields` 同一份口径）：
// 桶键 + **条数**（批4 第二片新增 `count`：`--by tier` 三桶的「条数」列）+ 三计数
// （未闭 / 已解 / 净）+ 近邻量化三格（P0/P1/P2）。
var gapStatusFields = []string{"bucket", "count", "open", "closed", "net", "p0", "p1", "p2"}

// gapStatusBucket —— 一个桶的计数面。
type gapStatusBucket struct {
	Key    string // 桶键：`--by unit` ⇒ `unit`；`--by module` ⇒ `module`；`--by tier` ⇒ 分级三值之一
	All    int    // 条数（本桶落了多少条 —— `--by tier` 的「条数」列取它）
	Open   int    // 未闭：`state` 仍缺
	Closed int    // 已解：`state` 已解
	P0     int    // 近邻量化：P0 条数
	P1     int    // 近邻量化：P1 条数
	P2     int    // 近邻量化：P2 条数
	// None —— 桶内条目**全部** `module == 无件`（兜底件）⇒ 对账等式里的「无件桶」。
	// ★ `--by tier` 下这一格**不参与**任何判据（桶键是分级、不是件 ⇒ 件轴对账式在 tier 面不成立）。
	None bool
}

// Net 净 = 未闭 − 已解（**可为负** —— 解掉的比还缺的多）。
func (b gapStatusBucket) Net() int { return b.Open - b.Closed }

// gapStatusQueryText —— `meta.query` 与人面「查询元」那一行的回显（**现读收窄入参**）。
// 前半段**逐字复用** `gapLsQueryText`（三枚闭集旗标的同一种序与同一种「缺席 ≠ 假值」语义），
// 本面只续 `by` / `top` 两格 —— 不另造第二套 query 拼装（同源才不会两处走岔）。
func gapStatusQueryText(states []string, prio, impact, by string, top int) string {
	base := gapLsQueryText(states, prio, impact, "", "")
	extra := `"by":` + jstr(by) + `,"top":` + strconv.Itoa(top)
	if base == "{}" {
		return "{" + extra + "}"
	}
	return strings.TrimSuffix(base, "}") + "," + extra + "}"
}

// gapStatusReconcile —— 对账等式的三数（**现算** · 不填死）。
type gapStatusReconcile struct {
	RealOpen   int // Σ(有件桶未闭)
	NoneOpen   int // 无件桶（`module == 无件` 且 `state == 仍缺`）
	LedgerOpen int // 账内仍缺总数（**本筛选后** —— 收窄后两数才同口径可比）
}

func (r gapStatusReconcile) Left() int { return r.RealOpen + r.NoneOpen }
func (r gapStatusReconcile) Diff() int { return r.Left() - r.LedgerOpen }
func (r gapStatusReconcile) OK() bool  { return r.Diff() == 0 }
func (r gapStatusReconcile) JSON() string {
	return fmt.Sprintf(`{"real_bucket_open":%d,"none_bucket_open":%d,"ledger_open":%d,"diff":%d,"ok":%t}`,
		r.RealOpen, r.NoneOpen, r.LedgerOpen, r.Diff(), r.OK())
}

// gapStatusTierReconcile —— `--by tier` 面的**守恒式**三数（**现算** · 不填死）。
//
//	Sum     = Σ(三桶条数) —— 由**三桶各自的累加**得出（上桶与下桶是两个独立累加）；
//	Total   = **现读**总数（本筛选后 —— 无收窄时逐字等于**全账条数**）—— 由筛选循环自己累加；
//	Ledger  = 现读**全账**条数（收窄时把「收窄 ≠ 全账」这件事摆在明面上）。
//
// ★ 守恒式必须**真自校**：两边来自两个不同的累加（桶循环 vs 筛选循环），不是同一个数源抄两遍；
//
//	不成立 ⇒ 表尾两个数原文并列 + 判红（退码 1）。
type gapStatusTierReconcile struct {
	Sum         int
	Total       int
	Ledger      int
	OutOfSet    int // 落库 `verify_tier` 越界条数（只报告 · 不退码）
	PlaceOpen   int // 占位桶里 `state == 仍缺` 的条数（表尾「只报告」那行用）
	PlaceClosed int // 占位桶里 `state == 已解` 的条数
	Filtered    bool
}

func (t gapStatusTierReconcile) Diff() int { return t.Sum - t.Total }
func (t gapStatusTierReconcile) OK() bool  { return t.Diff() == 0 }
func (t gapStatusTierReconcile) JSON() string {
	return fmt.Sprintf(`{"buckets_sum":%d,"ledger_total":%d,"diff":%d,"ok":%t,"out_of_set":%d,"narrowed":%t}`,
		t.Sum, t.Total, t.Diff(), t.OK(), t.OutOfSet, t.Filtered)
}

// gapStatusFamReconcile —— `--by family` 面的**守恒式**三数（**现算** · 不填死）：
// `Σ(桶内未闭) + 桶外 == 账内仍缺`。
//
// \tPageOpen   = Σ(本页桶未闭)   —— 由桶循环逐条累加出来的 `b.Open` 现加（不是从别处抄）；
// \tOutside    = Σ(未上页桶未闭) —— 收窄到一页时那些桶的未闭数（**点名「桶外」而不是丢掉**）；
// \tSumOpen    = PageOpen + Outside = Σ(**全部**桶未闭)；
// \tLedgerOpen = 筛选循环自己累加的「账内仍缺」（**另一个独立累加**）。
//
// ★ 两边来自两个独立累加（桶循环 vs 筛选循环）⇒ 对不上就是本面坏了（与 unit/module 面同一条规矩）。
type gapStatusFamReconcile struct {
	PageOpen   int
	Outside    int
	LedgerOpen int
	Buckets    int // 全部桶数（不是本页码数）
	Filtered   bool
}

func (f gapStatusFamReconcile) SumOpen() int { return f.PageOpen + f.Outside }
func (f gapStatusFamReconcile) Diff() int    { return f.SumOpen() - f.LedgerOpen }
func (f gapStatusFamReconcile) OK() bool     { return f.Diff() == 0 }
func (f gapStatusFamReconcile) JSON() string {
	return fmt.Sprintf(`{"bucket_sum_open":%d,"outside_open":%d,"ledger_open":%d,"diff":%d,"ok":%t,"buckets":%d,"narrowed":%t}`,
		f.SumOpen(), f.Outside, f.LedgerOpen, f.Diff(), f.OK(), f.Buckets, f.Filtered)
}

// gapStatusMetaAdd —— `gap status` 机器面信封字段的**唯一写入口**（只走 `metaAdd*` 既有口子 ⇒
// 顶层六键不动）。五格与 `gap ls` **同形**（`total` / `hits` / `query` / `query_ts` / `ledger_sha16`），
// 另加本面自己的三格（`by` / `top` / 对账）—— 缺一格两个时刻的排行就不可比。
//
// ★ `--by tier`（批4 第二片）：件轴对账那两数（`real_bucket_open` / `none_bucket_open`）在分级面上
//
//	**不成立**（桶键不是件）⇒ 不冒充；改出本面自己的**守恒式两数 + 总账**
//	（`tier_reconcile` 含 `buckets_sum` / `ledger_total` / `diff` / `ok` / `out_of_set`）
//	+ 顶层一格 `ledger_total`。**只有 tier 这一支**多出这两键 ⇒ `unit` / `module` 两态一字不变。
func gapStatusMetaAdd(inv *invocation, states []string, prio, impact, by string, top, total, hits int,
	rec gapStatusReconcile, tierRec gapStatusTierReconcile, famRec gapStatusFamReconcile, ledgerPath string) {
	inv.metaAddJSON("total", strconv.Itoa(total))
	inv.metaAddJSON("hits", strconv.Itoa(hits))
	inv.metaAddJSON("query", gapStatusQueryText(states, prio, impact, by, top))
	inv.metaAddStr("query_ts", gapNow())
	inv.metaAddStr("ledger_sha16", gapLsLedgerSHA16(ledgerPath))
	inv.metaAddStr("by", by)
	inv.metaAddJSON("top", strconv.Itoa(top))
	if by == gapStatusByTier {
		inv.metaAddJSON("tier_reconcile", tierRec.JSON())
		inv.metaAddJSON("ledger_total", strconv.Itoa(tierRec.Ledger))
		return
	}
	// ★ 族轴（片 FAM-1）：件轴对账那两数（`real_bucket_open` / `none_bucket_open`）在族面上**不成立**
	//   （桶键是族、不是件）⇒ 不冒充；改出本面自己的守恒式 `family_reconcile`
	//   （`bucket_sum_open` + `outside_open` == `ledger_open`）。**只有族这一支**多出这一键。
	if by == gapStatusByFamily {
		inv.metaAddJSON("family_reconcile", famRec.JSON())
		return
	}
	inv.metaAddJSON("reconcile", rec.JSON())
}

func cmdGapStatus(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面（在任何盘面动作之前）：`--json` 不给字段 / `--by` 越界 / `--top` 不是正整数。
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapStatusFields, ","))
		return exitUsage
	}
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = gapStatusByUnit
	}
	if by != gapStatusByUnit && by != gapStatusByModule && by != gapStatusByTier && by != gapStatusByDay &&
		by != gapStatusByFamily {
		inv.setErr("usage", "bad_by", "--by 取值不在闭集里")
		fmt.Fprintf(stderr, "%s: `--by %s` 不在闭集里 —— 只认 %s | %s | %s | %s | %s\n",
			progName, by, gapStatusByUnit, gapStatusByModule, gapStatusByTier, gapStatusByDay, gapStatusByFamily)
		return exitUsage
	}
	top := gapStatusDefaultTop
	if v := strings.TrimSpace(inv.flagVal("--top")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			inv.setErr("usage", "bad_top", "--top 要给正整数")
			fmt.Fprintf(stderr, "%s: `--top %s` 不是正整数（本族口径 = 用法错 2）\n", progName, v)
			return exitUsage
		}
		top = n
	}
	clampedTop := false
	if top > gapLsRowCap {
		top, clampedTop = gapLsRowCap, true
	}
	for _, s := range inv.flagVals("--state") {
		if !gapIn(gapStateClosed, s) {
			inv.setErr("usage", "bad_state", "state 值不在六值闭集里")
			fmt.Fprintf(stderr, "%s: `--state %s` 不在闭集里 —— 只认 %s\n", progName, s, gapClosedText(gapStateClosed))
			return exitUsage
		}
	}
	if p := strings.TrimSpace(inv.flagVal("--prio")); p != "" && !gapIn(gapPrioClosed, p) {
		inv.setErr("usage", "bad_prio", "prio 值不在三值闭集里")
		fmt.Fprintf(stderr, "%s: `--prio %s` 不在闭集里 —— 只认 %s\n", progName, p, gapClosedText(gapPrioClosed))
		return exitUsage
	}
	if im := strings.TrimSpace(inv.flagVal("--impact")); im != "" && !gapIn(gapImpactClosed, im) {
		inv.setErr("usage", "bad_impact", "impact 值不在六值闭集里")
		fmt.Fprintf(stderr, "%s: `--impact %s` 不在闭集里 —— 只认 %s\n", progName, im, gapClosedText(gapImpactClosed))
		return exitUsage
	}

	// ② 读真源（读不到 ⇒ 8 · 不许当绿）—— 与 `gap ls` **逐字同一套收法**（两枚机器可辨 reason）。
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当「没有」（退码 8）\n", gapLedgerPath())
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "真源不在盘上", led.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		gapLedgerAbsentHint(stderr, led.Path)
		return exitBlocked
	}

	// ③ 账内闭集自查（出现闭集外的值 ⇒ 判红 1 + 点名到行）—— 判据与 `gap ls` 同一条。
	for i, r := range led.Recs {
		field, val := "", ""
		switch {
		case !gapIn(gapStateClosed, r.State):
			field, val = "state", r.State
		case !gapIn(gapImpactClosed, r.Impact):
			field, val = "impact", r.Impact
		case !gapIn(gapPrioClosed, r.Prio):
			field, val = "prio", r.Prio
		}
		if field == "" {
			continue
		}
		inv.setErr("failed", "ledger_out_of_range", fmt.Sprintf("第 %d 行 %s 越界", led.No[i], field))
		fmt.Fprintf(stderr, "账内越界：%d %s\n", led.No[i], field)
		fmt.Fprintf(stderr, "  %s 的值 %q 不在闭集里（%s）—— 真源只由命令写（防呆③ 同源）\n",
			field, val, gapClosedText(gapClosureOf(field)))
		fmt.Fprintf(stderr, "  ⇒ 判红 1（账坏了不是「零命中」；修法：`zerg gap verify` 或手工订正该行）\n")
		return exitFail
	}

	// ③′ ★ **时间维度面（`--by day` · 缺口账 `GAP-20260929-34`）**：只读 · 早返。
	//   为什么早返而不并进下面的桶循环：下面的桶/排序/对账三条判据量的是「谁在欠」（件面静态分布），
	//   本面量的是「何时在变」（时间趋势）—— 共用一个桶结构会把两套口径混成一笔（`All` 既当条数
	//   又当新增数 ⇒ 对账等式当场失去意义）。早返**一个既有字节都不动**：既有三值的读数逐字不变。
	//   口径：**新增** = `found_at` 落那一天 · **销案** = `solved_at` 落那一天（两轴各自累加 ·
	//   同一天两数并列）· **净** = 新增 − 销案。
	//   ★ 自报缺口（不许静默少算）：`不做` 那一支**不落闭案时刻**（`set-state` 只在转 `已解` 时
	//     写 `solved_at` · `family_gap_state.go`）⇒ 销案线对那 11 条**系统性少**，表尾逐条点名条数。
	if by == gapStatusByDay {
		wantStates := inv.flagVals("--state")
		wantPrio := strings.TrimSpace(inv.flagVal("--prio"))
		wantImpact := strings.TrimSpace(inv.flagVal("--impact"))
		type gapDayBucket struct{ newN, closedN int }
		days := map[string]*gapDayBucket{}
		order := []string{}
		addDay := func(d string) *gapDayBucket {
			b, ok := days[d]
			if !ok {
				b = &gapDayBucket{}
				days[d] = b
				order = append(order, d)
			}
			return b
		}
		popTotal, closedTotal, noTsNone, closedNoTs := 0, 0, 0, 0
		for _, r := range led.Recs {
			if len(wantStates) > 0 && !gapIn(wantStates, r.State) {
				continue
			}
			if wantPrio != "" && r.Prio != wantPrio {
				continue
			}
			if wantImpact != "" && r.Impact != wantImpact {
				continue
			}
			popTotal++
			d := gapDayOf(r.FoundAt)
			if d == "" {
				d, noTsNone = gapDayNone, noTsNone+1
			}
			addDay(d).newN++
			if s := gapDayOf(r.SolvedAt); s != "" {
				addDay(s).closedN++
				closedTotal++
			} else if r.State == gapStSolved || r.State == gapStWontDo {
				closedNoTs++
			}
		}
		sort.Strings(order)
		total := len(order)
		hits, cut := total, false
		if total > top {
			hits, cut = top, true
		}
		if total == 0 {
			inv.changed = boolPtr(false)
			inv.setErr("failed", "no_match", "零命中")
			fmt.Fprintf(stderr, "零命中：账内 %d 条 · 与筛选条件相符 0 条（退码 1 —— 「没有」不是「失败」，也不是绿）\n", len(led.Recs))
			return exitFail
		}
		inv.changed = boolPtr(false)
		if cut {
			inv.markTruncated()
			fmt.Fprintf(stderr, "%s: ⚠ 本页只列前 %d 个日桶 · 真命中 %d 个（已裁 %d 个）—— **这不是全集**\n",
				progName, hits, total, total-hits)
		}
		if inv.jsonGiven {
			rows := make([]map[string]string, 0, hits)
			for _, d := range order[:hits] {
				b := days[d]
				rows = append(rows, map[string]string{
					"bucket": d,
					"count":  strconv.Itoa(b.newN),
					"open":   strconv.Itoa(b.newN),
					"closed": strconv.Itoa(b.closedN),
					"net":    strconv.Itoa(b.newN - b.closedN),
					"new":    strconv.Itoa(b.newN),
				})
			}
			return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
		}
		fmt.Fprintf(stdout, "zerg gap status --by day · 账内 %d 条 · 本筛选后 %d 条 · 日桶 %d 个（本页 %d 个）\n",
			len(led.Recs), popTotal, total, hits)
		fmt.Fprintf(stdout, "口径：新增 = `found_at` 落那一天（入账轴）· 销案 = `solved_at` 落那一天（闭案轴）· 净 = 新增 − 销案\n")
		fmt.Fprintf(stdout, "查询元：query_ts=%s · ledger_sha16=%s\n", gapNow(), gapLsLedgerSHA16(led.Path))
		fmt.Fprintf(stdout, "  %s  %s  %s  %s\n", pad("日期", 10), pad("新增", 6), pad("销案", 6), pad("净", 6))
		for _, d := range order[:hits] {
			b := days[d]
			fmt.Fprintf(stdout, "  %s  %s  %s  %s\n", pad(d, 10),
				pad(strconv.Itoa(b.newN), 6), pad(strconv.Itoa(b.closedN), 6),
				pad(fmt.Sprintf("%+d", b.newN-b.closedN), 6))
		}
		// 表尾守恒式：左边 **由桶循环自己累加出来的** `sumNew`，右边是筛选循环数的 `popTotal`
		// —— 两个独立累加（不是同一个变量抄两遍）⇒ 对不上就是本面坏了，判红 1（与 tier 面同一条规矩）。
		sumNew := 0
		for _, d := range order {
			sumNew += days[d].newN
		}
		mark := "✓"
		if sumNew != popTotal {
			mark = "✗（**不对账** —— 日轴丢了/重算了条数）"
		}
		fmt.Fprintf(stdout, "守恒式：Σ新增 %d == 本筛选后条数 %d %s（差 %d · 两个数各自现算）· Σ销案 %d\n",
			sumNew, popTotal, mark, sumNew-popTotal, closedTotal)
		if sumNew != popTotal {
			inv.setErr("failed", "day_not_reconciled", "日轴 Σ新增 与本筛选后条数对不上")
			fmt.Fprintf(stdout, "  ⚠ 守恒式不成立：Σ新增 %d ≠ 本筛选后条数 %d（差 %+d）⇒ 读数不许外发 ⇒ 判红 1\n",
				sumNew, popTotal, sumNew-popTotal)
			return exitFail
		}
		if noTsNone > 0 {
			fmt.Fprintf(stdout, "  ⚠ 无 `found_at` 的 %d 条落 `%s` 桶（没入账时刻 ⇒ 进不了日轴 · 已点名，未丢）\n",
				noTsNone, gapDayNone)
		}
		if closedNoTs > 0 {
			fmt.Fprintf(stdout, "  ⚠ **闭案无时刻** %d 条（现态已闭（已解/不做）却无 `solved_at`）⇒ 销案线**系统性少算**，本读数**不是**净减\n", closedNoTs)
		}
		return exitOK
	}

	// ④ 筛选（AND）+ 分桶 —— 一次过。判据序与 `gap ls` 同：state / prio / impact。
	wantStates := inv.flagVals("--state")
	wantPrio := strings.TrimSpace(inv.flagVal("--prio"))
	wantImpact := strings.TrimSpace(inv.flagVal("--impact"))
	buckets := map[string]*gapStatusBucket{}
	order := []string{}
	if by == gapStatusByTier {
		// 三桶**固定先建**（闭集穷尽 ⇒ 某桶 0 条也要出这一行 —— 三桶之和才对得上账）。
		for _, t := range gapStatusTierOrder {
			buckets[t] = &gapStatusBucket{Key: t}
			order = append(order, t)
		}
	}
	popTotal, popOpen, noneOpen := 0, 0, 0
	tierOutOfSet := 0
	for _, r := range led.Recs {
		if len(wantStates) > 0 && !gapIn(wantStates, r.State) {
			continue
		}
		if wantPrio != "" && r.Prio != wantPrio {
			continue
		}
		if wantImpact != "" && r.Impact != wantImpact {
			continue
		}
		popTotal++
		if r.State == gapStOpen {
			popOpen++
			if r.Module == gapUnitNoneMod {
				noneOpen++
			}
		}
		key := r.Unit
		switch by {
		case gapStatusByModule:
			key = r.Module
		case gapStatusByFamily:
			// 族轴：键 = 账**自带**的 `want_family`（原值原样成桶 · 不经推断 · 不合并同义异写）。
			key = r.WantFam
		case gapStatusByTier:
			// 分级 = **已落库的 `verify_tier` 优先**；未落库 ⇒ 现读 `verify_cmd` 现算（三值穷尽）。
			t, oos := gapStatusTierOf(r)
			key = t
			if oos {
				tierOutOfSet++
			}
		}
		if by == gapStatusByFamily && strings.TrimSpace(key) == "" {
			key = gapFamNone // 空值单列一桶（逐字点名 —— 不并进任何既有桶）
		}
		// ★ **兜底值渲染面细分**（片 FAM-1）：`unit` **恰等于**兜底字面量 `gapUnitNoFile`（`无件(命令面)`）时，
		//   桶键按**该条自己的 `impact`** 细分成 `无件(<impact>)` —— 只改渲染键，**不改账、不写账**。
		//   逐字判据：仅 `by == unit` 且键恰等于兜底字面量这一格细分（其余 197 个 unit 值一个字节不动）。
		if by == gapStatusByUnit && key == gapUnitNoFile {
			key = gapUnitNoFileSub(r.Impact)
		}
		if by != gapStatusByTier && by != gapStatusByFamily && key == "" {
			key = gapUnitNoneMod // 无件兜底桶（对账等式里点名的那一枚）
		}
		b, ok := buckets[key]
		if !ok {
			b = &gapStatusBucket{Key: key, None: true}
			buckets[key] = b
			order = append(order, key)
		}
		b.All++
		if (by == gapStatusByUnit || by == gapStatusByModule) && r.Module != gapUnitNoneMod {
			b.None = false
		}
		switch r.State {
		case gapStOpen:
			b.Open++
		case gapStSolved:
			b.Closed++
		}
		switch r.Prio {
		case "P0":
			b.P0++
		case "P1":
			b.P1++
		case "P2":
			b.P2++
		}
	}

	// ⑤ 排序：主键 = 未闭降序 · 次键 = 近邻量化降序（先 P0、再 P1、再 P2）· 再键 = 桶键升序（同分稳定）。
	//    ★ `--by tier` **不排序**：三桶按闭集**固定序**（有效 → 占位 → 无）出 —— 判据有效性递减，
	//      两个时刻的表逐行可比；排序键对它没有意义（三桶恒为 3 行）。
	bs := make([]gapStatusBucket, 0, len(order))
	for _, k := range order {
		bs = append(bs, *buckets[k])
	}
	if by != gapStatusByTier {
		sort.SliceStable(bs, func(i, j int) bool {
			a, b := bs[i], bs[j]
			if a.Open != b.Open {
				return a.Open > b.Open
			}
			if a.P0 != b.P0 {
				return a.P0 > b.P0
			}
			if a.P1 != b.P1 {
				return a.P1 > b.P1
			}
			if a.P2 != b.P2 {
				return a.P2 > b.P2
			}
			return a.Key < b.Key
		})
	}
	realOpen := 0
	for _, b := range bs {
		if !b.None {
			realOpen += b.Open
		}
	}
	rec := gapStatusReconcile{RealOpen: realOpen, NoneOpen: noneOpen, LedgerOpen: popOpen}
	// ★ tier 面守恒式（**两个独立累加** 对账）：
	//   · 左边 Σ(三桶条数) —— 由桶循环逐条累加出来的 `b.All` 现加（不是从别处抄）；
	//   · 右边**现读总账条数** —— 由 `gapLedger` 读盘时自己数的行数（`len(led.Recs)`）。
	//   收窄时右边改成筛选循环自己累加出来的 `popTotal`（**同口径才可比**），并把「全账条数」
	//   一并摆在表尾 —— 收窄读数 ≠ 全账，这一点在行内点名，不让两数互当。
	hasNarrow := len(wantStates) > 0 || wantPrio != "" || wantImpact != ""
	tierRec := gapStatusTierReconcile{
		Ledger:   len(led.Recs),
		Total:    popTotal,
		OutOfSet: tierOutOfSet,
		Filtered: hasNarrow,
	}
	for _, k := range gapStatusTierOrder {
		if b, ok := buckets[k]; ok {
			tierRec.Sum += b.All
		}
	}
	if b, ok := buckets[gapVerifyTierPlaceholder]; ok {
		tierRec.PlaceOpen, tierRec.PlaceClosed = b.Open, b.Closed
	}
	if !hasNarrow {
		// 逐字判据：无收窄 ⇒ 右边就是**总账条数**本身（读盘时数的那个数），不是筛出来的那个。
		tierRec.Total = len(led.Recs)
	}
	total := len(bs)
	hits, cut := total, false
	if total > top {
		hits, cut = top, true
	}
	// 族轴守恒式**现算**（片 FAM-1）：本页桶未闭 / 桶外未闭 各自累加 ⇒ 与筛选循环的 `popOpen` 对账。
	famRec := gapStatusFamReconcile{LedgerOpen: popOpen, Buckets: total, Filtered: hasNarrow}
	for i, b := range bs {
		if i < hits {
			famRec.PageOpen += b.Open
			continue
		}
		famRec.Outside += b.Open
	}

	// 零命中（筛选后一条都不剩 ⇒ 与 `gap ls` 同判：1）—— ★ tier 面另加一档：三桶恒存在，
	// 故「收窄后 0 条」必须单独判（否则会拿三个 0 冒充读数）。
	if total == 0 || (by == gapStatusByTier && popTotal == 0) {
		inv.changed = boolPtr(false)
		inv.setErr("failed", "no_match", "零命中")
		fmt.Fprintf(stderr, "零命中：账内 %d 条 · 与筛选条件相符 0 条（退码 1 —— 「没有」不是「失败」，也不是绿）\n", len(led.Recs))
		if inv.jsonGiven {
			gapStatusMetaAdd(inv, wantStates, wantPrio, wantImpact, by, top, 0, 0, rec, tierRec, famRec, led.Path)
			emitEnvelopeWith(stdout, find(inv.path), "[]", 0, inv)
		}
		return exitFail
	}

	inv.changed = boolPtr(false)
	gapStatusMetaAdd(inv, wantStates, wantPrio, wantImpact, by, top, total, hits, rec, tierRec, famRec, led.Path)
	if clampedTop {
		inv.warnf("--top 越界（>%d）⇒ 已收到一页硬顶 %d", gapLsRowCap, gapLsRowCap)
		fmt.Fprintf(stderr, "%s: ⚠ `--top` 超过一页硬顶 %d ⇒ 已收到 %d（与 `gap ls` / `find` / `code find` 同一个数）\n",
			progName, gapLsRowCap, gapLsRowCap)
	}
	if cut {
		inv.markTruncated()
		inv.warnf("已裁 %d 个桶（gap status 一页 %d 个 / 真命中 %d 个）", total-hits, hits, total)
		inv.metaAddJSON("truncated_detail", fmt.Sprintf(
			`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`, hits, total-hits, total))
		fmt.Fprintf(stderr, "%s: ⚠ 本页只列前 %d 个桶 · 真命中 %d 个（已裁 %d 个）—— **这不是全集**，别据此下结论；要收窄：--by / --top / --state / --prio / --impact\n",
			progName, hits, total, total-hits)
	}
	if (by == gapStatusByUnit || by == gapStatusByModule) && !rec.OK() {
		inv.warnf("对账不成立：Σ(有件桶未闭) %d + 无件桶 %d = %d ≠ 账内仍缺 %d（差 %+d）",
			rec.RealOpen, rec.NoneOpen, rec.Left(), rec.LedgerOpen, rec.Diff())
	}
	// ★ 族轴守恒式告警（片 FAM-1）：Σ(桶内未闭) + 桶外 ≠ 账内仍缺 ⇒ 点名（与 unit/module 面同判：warning）。
	if by == gapStatusByFamily && !famRec.OK() {
		inv.warnf("族轴守恒式不成立：Σ(桶内未闭) %d + 桶外 %d = %d ≠ 账内仍缺 %d（差 %+d）",
			famRec.PageOpen, famRec.Outside, famRec.SumOpen(), famRec.LedgerOpen, famRec.Diff())
	}
	// ★ tier 面守恒式**真自校**：Σ(三桶条数) 与 现读总账条数 对不上 ⇒ **判红 1**（不是 warning ——
	// 分级面自己坏了，「三桶之和 == 总账」这条判据就不成立，读数不许外发）。占位桶只报告（见人面表尾）。
	if by == gapStatusByTier && !tierRec.OK() {
		inv.setErr("failed", "tier_not_reconciled", "三桶之和与总账对不上")
		inv.warnf("守恒式不成立：三桶之和 %d ≠ 总账 %d（差 %+d）", tierRec.Sum, tierRec.Total, tierRec.Diff())
	}

	// ⑥ 机器面。
	if inv.jsonGiven {
		rows := make([]map[string]string, 0, hits)
		for _, b := range bs[:hits] {
			row := map[string]string{
				"bucket": b.Key,
				"count":  strconv.Itoa(b.All),
				"open":   strconv.Itoa(b.Open),
				"closed": strconv.Itoa(b.Closed),
				"net":    strconv.Itoa(b.Net()),
				"p0":     strconv.Itoa(b.P0),
				"p1":     strconv.Itoa(b.P1),
				"p2":     strconv.Itoa(b.P2),
			}
			rows = append(rows, row)
		}
		rc := selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
		if by == gapStatusByTier && rc == exitOK && !tierRec.OK() {
			return exitFail
		}
		return rc
	}

	// ⑦-tier 人面（`--by tier` · 批4 第二片）：行 = 三桶闭集 · 列 = 条数 / 其中仍缺 / 其中已解；
	// 表尾**固定两行** —— ① 守恒式自校（不等 ⇒ 判红 1 并打印两数）② 占位桶只报告（**不退码**）。
	if by == gapStatusByTier {
		fmt.Fprintf(stdout, "zerg gap status --by tier · 账内 %d 条 · 本筛选后 %d 条 · 桶 %d 个（本页 %d 个）\n",
			len(led.Recs), popTotal, total, hits)
		fmt.Fprintf(stdout, "排序：无 —— 三桶按**闭集固定序**（有效 → 占位 → 无 · 判据有效性递减）出，两个时刻逐行可比\n")
		fmt.Fprintf(stdout, "查询元：query_ts=%s · ledger_sha16=%s · 收窄回显=%s\n",
			gapNow(), gapLsLedgerSHA16(led.Path), gapStatusQueryText(wantStates, wantPrio, wantImpact, by, top))
		kw := displayWidth("分级(tier)")
		for _, b := range bs {
			kw = maxInt(kw, displayWidth(b.Key))
		}
		fmt.Fprintf(stdout, "  %s  %s  %s  %s\n",
			pad("分级(tier)", kw), pad("条数", 4), pad("仍缺", 4), pad("已解", 4))
		for _, b := range bs {
			fmt.Fprintf(stdout, "  %s  %s  %s  %s\n",
				pad(b.Key, kw),
				pad(strconv.Itoa(b.All), 4),
				pad(strconv.Itoa(b.Open), 4),
				pad(strconv.Itoa(b.Closed), 4))
		}
		// 表尾①-a：三桶条数**原文**（供人一眼核 —— 与守恒式左边是同一份读数）。
		fmt.Fprintf(stdout, "三桶条数原文：")
		for i, t := range gapStatusTierOrder {
			b := buckets[t]
			if i > 0 {
				fmt.Fprintf(stdout, " · ")
			}
			fmt.Fprintf(stdout, "%s %d", t, b.All)
		}
		fmt.Fprintf(stdout, "\n")
		// 表尾①-b：守恒式自校（两个数各自现算 —— Σ 取三桶累加 · 右边取读盘行数/筛选累加）。
		mark := "✓"
		if !tierRec.OK() {
			mark = "✗（**不对账** —— 分级面坏了）"
		}
		if tierRec.Filtered {
			fmt.Fprintf(stdout, "守恒式：三桶之和 %d == 现读总数 %d %s（差 %d）· **本筛选后** —— 全账 %d 条（收窄读数 ≠ 全账，别据此下结论）\n",
				tierRec.Sum, tierRec.Total, mark, tierRec.Diff(), tierRec.Ledger)
		} else {
			fmt.Fprintf(stdout, "守恒式：三桶之和 %d == 总账 %d %s（差 %d · 两个数各自现算：Σ 取三桶累加 · 总账取读盘行数）\n",
				tierRec.Sum, tierRec.Total, mark, tierRec.Diff())
		}
		if !tierRec.OK() {
			fmt.Fprintf(stdout, "  ⚠ 守恒式不成立：三桶之和 %d ≠ 总账 %d（差 %+d）—— 分级漏 / 重算，读数不许外发 ⇒ 判红 1\n",
				tierRec.Sum, tierRec.Total, tierRec.Diff())
			fmt.Fprintf(stderr, "%s: 守恒式不成立：三桶之和 %d ≠ 总账 %d（差 %+d）⇒ 判红 1\n",
				progName, tierRec.Sum, tierRec.Total, tierRec.Diff())
		}
		if tierRec.OutOfSet > 0 {
			fmt.Fprintf(stdout, "  ⚠ 落库 `verify_tier` 越界 %d 条（不在三值闭集里）⇒ **不采信、落回现算**（只报告 · 不退码）\n", tierRec.OutOfSet)
		}
		// 表尾②：占位桶只报告行（**不退码** —— 设计稿 D2「先只量分布」· §M-31 形态层/有效性层分两层量）。
		fmt.Fprintf(stdout, "占位桶：%d 条（其中仍缺 %d · 已解 %d）—— **只报告、不阻断**（本行不进退码）\n",
			buckets[gapVerifyTierPlaceholder].All, tierRec.PlaceOpen, tierRec.PlaceClosed)
		if !tierRec.OK() {
			return exitFail
		}
		return exitOK
	}

	// ⑦ 人面（主输出走 stdout · 消息与错误走 stderr）：表头**两个排序键都写**。
	colName := "件(unit)"
	if by == gapStatusByModule {
		colName = "模块(module)"
	}
	if by == gapStatusByFamily {
		colName = "族(want_family)"
	}
	kw := displayWidth(colName)
	for _, b := range bs[:hits] {
		kw = maxInt(kw, displayWidth(truncateDisplay(b.Key, 48)))
	}
	fmt.Fprintf(stdout, "zerg gap status --by %s · 账内 %d 条 · 本筛选后 %d 条 · 桶 %d 个（本页 %d 个）\n",
		by, len(led.Recs), popTotal, total, hits)
	fmt.Fprintf(stdout, "排序键：① 未闭 降序  ② 近邻量化 P0/P1/P2 降序（先 P0 · 再 P1 · 再 P2）\n")
	fmt.Fprintf(stdout, "查询元：query_ts=%s · ledger_sha16=%s · 收窄回显=%s\n",
		gapNow(), gapLsLedgerSHA16(led.Path), gapStatusQueryText(wantStates, wantPrio, wantImpact, by, top))
	fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s\n",
		pad(colName, kw), pad("未闭", 4), pad("已解", 4), pad("净", 4), "近邻(P0/P1/P2)")
	for _, b := range bs[:hits] {
		fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s\n",
			pad(truncateDisplay(b.Key, 48), kw),
			pad(strconv.Itoa(b.Open), 4),
			pad(strconv.Itoa(b.Closed), 4),
			pad(strconv.Itoa(b.Net()), 4),
			fmt.Sprintf("%d/%d/%d", b.P0, b.P1, b.P2))
	}
	mark := "✓"
	if !rec.OK() {
		mark = "✗（**不对账** —— 分桶面坏了 · 差数见下）"
	}
	if by == gapStatusByFamily {
		fmark := "✓"
		if !famRec.OK() {
			fmark = "✗（**不对账** —— 族轴分桶面坏了 · 差数见下）"
		}
		fmt.Fprintf(stdout, "对账：Σ(桶内未闭) %d + 桶外 %d = %d == 账内仍缺 %d %s（差 %d · 桶 %d 个）\n",
			famRec.PageOpen, famRec.Outside, famRec.SumOpen(), famRec.LedgerOpen, fmark, famRec.Diff(), famRec.Buckets)
		if !famRec.OK() {
			fmt.Fprintf(stdout, "  ⚠ 族轴守恒式不成立：Σ(桶内未闭) + 桶外 与 账内仍缺 对不上 —— 差数 %+d（族轴分桶漏/重算 · 别据此下结论）\n", famRec.Diff())
		}
		return exitOK
	}
	fmt.Fprintf(stdout, "对账：Σ(有件桶未闭) %d + 无件桶 %d = %d == 账内仍缺 %d %s（差 %d）\n",
		rec.RealOpen, rec.NoneOpen, rec.Left(), rec.LedgerOpen, mark, rec.Diff())
	if !rec.OK() {
		fmt.Fprintf(stdout, "  ⚠ 对账不成立：Σ(有件桶未闭) + 无件桶 与 账内仍缺 对不上 —— 差数 %+d（分桶漏/重算 · 别据此下结论）\n", rec.Diff())
	}
	return exitOK
}

// ── 批2 第一片（2-1 / 2-2 / 2-4）：候选池独立件 + `source` 四值 + 人面入口 `gap idea` ──────────────
//
// 设计出处（唯一真源）：`Zerg-内部文档/项目文档/v2.5.13/设计-缺口账与自进化-v2.0-20260928.md`
//
//	§二十七 §11.3（候选池 = **独立文件 · 不进主账** · 字段 `cid`/`source`/`raw`/`created_at`/`ttl_days=7`/
//	`status=pending|promoted|expired|merged`/`fp`）+ §9.1 第 1/2 条（立候选池 + 加 `source` 四值）
//	+ 施工清单 `任务清单-缺口账自进化-施工-20260928.md` §3 的 2-1 / 2-2 / 2-4 三行（落点与判据）。
//
// 口径（逐条照设计稿 · 不自造）：
//
//	① **池件独立**：`<状态目录>/zerg-cli-gap-candidates.jsonl`（`ZERG_STATE_DIR` → `~/.zerg/state`）——
//	   与真源 `zerg-cli-gaps.jsonl` **完全分离**：本面**任何**路径都不写真源（判据：`idea add` 后账行数不变）。
//	② **落池时刻与来源进条目**：`created_at`（落池时刻 · RFC3339Nano · 取法与同族 `gapNow` 同一套）+ `source`。
//	③ **七日归档**：落池**超过七天**的条目移入同目录归档件
//	   `<状态目录>/zerg-cli-gap-candidates.archive-<YYYYMMDD>.jsonl` —— 命名照 §二十四 真源分层的「档案件」体例
//	   （`前缀.archive-<日期>.jsonl`）。★ 设计稿**未逐字钉死**候选池归档件名 ⇒ 本件按同族体例取定（回执点名）。
//	   归档 = **只搬超期条**（`status` 置 `expired`），不丢不重 ⇒ 判据「主池条数 == 未归档条数」。
//	④ **`source` 四值闭集**：`guard` / `gate` / `egg` / `human`（逐字照 §9.1 第 2 条）；闭集外 ⇒ 退码 **2**
//	   并在人面**点名闭集**（判词体例照同族 `--state`/`--prio`/`--impact` 三枚闭集旗标）。
//	⑤ **`gap idea add` = 人随手记的入口**：三态照本族写面（`--dry-run` 恒 0 · 缺 `--yes` fail-closed **2** ·
//	   计划件走 stderr）；**`gap add` 仍是唯一入账**（本面**不碰**真源）。
//	⑥ **`gap idea ls` = 最小读面**：`--source` 收窄（闭集外 ⇒ 2）· 人面列表 · `--json` 走同族信封写法
//	   （`meta.total` / `meta.hits` / 截断自报三件 —— 与 `gap ls` 的 `gapLsMetaAdd` 同一套，不另造）。
//	⑦ **审计**：设计稿 §11.3 只给候选池「独立文件 + 字段 + 出口」，**未列审计面** ⇒ 本件不写审计
//	   （「审计先落盘」那条纪律钉的是 `zerg-cli-gaps.jsonl` 的写面）；回执点名这一条取定。
//	⑧ **池件不在盘上**：照同族读面口径**不当绿**（`ls` 判 8 · 两因机器可辨）—— 与 `gap ls` 逐字同款。

const (
	gapPoolFile          = "zerg-cli-gap-candidates.jsonl"
	gapPoolArchivePrefix = "zerg-cli-gap-candidates.archive-"
	gapPoolTTLDays       = 7
	gapCandPrefix        = "CAND-"
)

// gapSourceClosed —— `source` 四值闭集（逐字照 §9.1 第 2 条：`guard` / `gate` / `egg` / `human`）。
var gapSourceClosed = []string{"guard", "gate", "egg", "human"}

// gapCandidate —— 候选池一行（**七格逐字照 §11.3**；`unit` 是**增补格** —— 照 §三十「并入节字段是增补」，
// 只为给人随手记的 `--unit` 留落点，缺省不写这一格）。
type gapCandidate struct {
	CID       string `json:"cid"`
	Source    string `json:"source"`
	Raw       string `json:"raw"`
	CreatedAt string `json:"created_at"`
	TTLDays   int    `json:"ttl_days"`
	Status    string `json:"status"`
	FP        string `json:"fp"`
	Unit      string `json:"unit,omitempty"`
	// GapID —— 提升后的正式账 id（批2 第二片 · 2026-09-28）：`promote` 落账后回填这一格。
	// `omitempty` ⇒ 未提升的行（以及老行）**逐字节不变**。
	GapID string `json:"gap_id,omitempty"`
}

// gapPool —— 读进来的候选池（原样字节 + 逐行原文 + 解析后的候选）。
type gapPool struct {
	Path   string
	Raw    []byte
	Lines  []string
	No     []int
	Cands  []gapCandidate
	Exists bool
}

// gapPoolPath —— 池件落点（设计稿 §11.3：「独立文件，不进主账」）。
func gapPoolPath() string { return filepath.Join(stateDirOf(), gapPoolFile) }

// gapPoolArchivePath —— 归档件落点（同目录 · 体例照 §二十四 的档案件命名）。
func gapPoolArchivePath(day string) string {
	return filepath.Join(stateDirOf(), gapPoolArchivePrefix+day+".jsonl")
}

// gapParseTS —— 解析落池时刻（两种布局都收；解析不出 ⇒ 视作「不超期」· 宁可留池也不误归档）。
func gapParseTS(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// readGapPool —— 读池件。**件不在**（首次）⇒ `Exists=false` 且无错；读不到 / 某行不是 JSON ⇒ 错（调用方译 8）。
func readGapPool() (gapPool, error) {
	p := gapPoolPath()
	pb := gapPool{Path: p}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return pb, nil
		}
		return pb, err
	}
	pb.Exists, pb.Raw = true, b
	txt := strings.TrimRight(string(b), "\n")
	if strings.TrimSpace(txt) == "" {
		return pb, nil
	}
	for i, l := range strings.Split(txt, "\n") {
		pb.Lines = append(pb.Lines, l)
		pb.No = append(pb.No, i+1)
		var c gapCandidate
		if err := json.Unmarshal([]byte(l), &c); err != nil {
			return pb, fmt.Errorf("池第 %d 行不是 JSON：%v", i+1, err)
		}
		pb.Cands = append(pb.Cands, c)
	}
	return pb, nil
}

// gapPoolWriteLines —— 整件重写池（归档后回写「未归档」那一批）。空 ⇒ 落零字节件。
func gapPoolWriteLines(path string, lines []string) error {
	body := []byte{}
	if len(lines) > 0 {
		body = []byte(strings.Join(lines, "\n") + "\n")
	}
	return os.WriteFile(path, body, 0o644)
}

// gapPoolAppendLines —— 追加落池（不动既有行）。
func gapPoolAppendLines(path string, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	return err
}

// gapPoolSweep —— **七日归档**（2-1）：把落池超过 `ttl_days` 天的条目挑出来（`status` 置 `expired`）。
// 只分类、不落盘；落盘由调用方按三态纪律做（`--dry-run` 那一态零副作用）。
func gapPoolSweep(pb gapPool, now time.Time) (keepLines, archLines []string, nArch int) {
	cut := now.Add(-time.Duration(gapPoolTTLDays) * 24 * time.Hour)
	for _, l := range pb.Lines {
		var c gapCandidate
		if err := json.Unmarshal([]byte(l), &c); err != nil {
			keepLines = append(keepLines, l)
			continue
		}
		if t, ok := gapParseTS(c.CreatedAt); ok && t.Before(cut) {
			c.Status = "expired"
			if b, err := json.Marshal(c); err == nil {
				archLines = append(archLines, string(b))
				continue
			}
		}
		keepLines = append(keepLines, l)
	}
	return keepLines, archLines, len(archLines)
}

// gapPoolNextCID —— 池内取号（`CAND-YYYYMMDD-NN` · 号段取**池内当日最大号 + 1**）。
func gapPoolNextCID(cands []gapCandidate, day string) string {
	max := 0
	for _, c := range cands {
		if !strings.HasPrefix(c.CID, gapCandPrefix+day+"-") {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(c.CID, gapCandPrefix+day+"-")); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%s-%02d", gapCandPrefix, day, max+1)
}

// gapLedgerLineCount —— 只读地数真源行数（**不解析、不写**）—— 用来把「真源一字未动」当场印给人看。
func gapLedgerLineCount(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	return strings.Count(string(b), "\n")
}

// gapPoolCountText —— 池件条数自报（人面/机器面共用一句）。
func gapPoolCountText(pb gapPool) string {
	if !pb.Exists {
		return "池件不在盘上（0 条）"
	}
	return fmt.Sprintf("%d 条", len(pb.Cands))
}

var gapIdeaFields = []string{"cid", "source", "raw", "unit", "created_at", "ttl_days", "status", "fp",
	"pool_before", "pool_after", "archived", "ledger_lines"}

var gapIdeaListFields = []string{"cid", "source", "raw", "unit", "created_at", "ttl_days", "status", "fp", "age_days"}

// gapIdeaMetaAdd —— `gap idea ls` 的信封字段（写法照拄同族 `gapLsMetaAdd` · **不另造**）。
func gapIdeaMetaAdd(inv *invocation, source string, total, hits int, poolPath string) {
	inv.metaAddJSON("total", strconv.Itoa(total))
	inv.metaAddJSON("hits", strconv.Itoa(hits))
	q := "{}"
	if source != "" {
		q = "{" + jstr("source") + ":" + jstr(source) + "}"
	}
	inv.metaAddJSON("query", q)
	inv.metaAddStr("query_ts", gapNow())
	inv.metaAddStr("pool_sha16", gapLsLedgerSHA16(poolPath))
}

// cmdGapIdeaLs —— `zerg gap idea ls`：候选池**最小读面**（只读 · 不写池、不写真源、不写审计）。
func cmdGapIdeaLs(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapIdeaListFields, ","))
		return exitUsage
	}
	wantSource := strings.TrimSpace(inv.flagVal("--source"))
	if wantSource != "" && !gapIn(gapSourceClosed, wantSource) {
		inv.setErr("usage", "bad_source", "source 值不在四值闭集里")
		fmt.Fprintf(stderr, "%s: `--source %s` 不在闭集里 —— 只认 %s\n", progName, wantSource, gapClosedText(gapSourceClosed))
		fmt.Fprintf(stderr, "  `source` 四值 = 采集层三源 + 人面随手记（设计稿 §9.1 第 2 条逐字）\n")
		return exitUsage
	}
	pb, err := readGapPool()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapPoolPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "池件 = %s；「读不到」不许当「没有」（退码 8）\n", gapPoolPath())
		return exitBlocked
	}
	if !pb.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "候选池件不在盘上", pb.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 候选池件不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, pb.Path)
		fmt.Fprintf(stderr, "  落一条候选：zerg gap idea add --symptom <一句> --yes\n")
		return exitBlocked
	}
	total, hits, cut := 0, 0, false
	out := []gapCandidate{}
	for _, c := range pb.Cands {
		if wantSource != "" && c.Source != wantSource {
			continue
		}
		total++
		if total > gapLsRowCap {
			cut = true
			continue
		}
		out = append(out, c)
	}
	hits = len(out)
	if total == 0 {
		inv.changed = boolPtr(false)
		inv.setErr("failed", "no_match", "零命中")
		fmt.Fprintf(stderr, "零命中：池内 %d 条 · 与筛选条件相符 0 条（退码 1 —— 「没有」不是「失败」，也不是绿）\n", len(pb.Cands))
		if wantSource != "" {
			gapIdeaMetaAdd(inv, wantSource, 0, 0, pb.Path)
			if inv.jsonGiven {
				emitEnvelopeWith(stdout, find(inv.path), "[]", 0, inv)
			}
		}
		return exitFail
	}
	inv.changed = boolPtr(false)
	gapIdeaMetaAdd(inv, wantSource, total, hits, pb.Path)
	if cut {
		inv.markTruncated()
		inv.warnf("已裁 %d 条（gap idea ls 一页 %d 条 / 真命中 %d 条）", total-hits, hits, total)
		inv.metaAddJSON("truncated_detail", fmt.Sprintf(
			`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`, hits, total-hits, total))
		fmt.Fprintf(stderr, "%s: ⚠ 本页只列前 %d 条 · 真命中 %d 条（已裁 %d 条）—— **这不是全集**\n", progName, hits, total, total-hits)
	}
	now := time.Now()
	rows := []map[string]string{}
	for _, c := range out {
		age := ""
		if t, ok := gapParseTS(c.CreatedAt); ok {
			age = strconv.Itoa(int(now.Sub(t).Hours() / 24))
		}
		rows = append(rows, map[string]string{
			"cid": c.CID, "source": c.Source, "raw": c.Raw, "unit": c.Unit,
			"created_at": c.CreatedAt, "ttl_days": strconv.Itoa(c.TTLDays),
			"status": c.Status, "fp": c.FP, "age_days": age,
		})
	}
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	cidw, srcw, stw := 0, 0, 0
	for _, c := range out {
		cidw = maxInt(cidw, displayWidth(c.CID))
		srcw = maxInt(srcw, displayWidth(c.Source))
		stw = maxInt(stw, displayWidth(c.Status))
	}
	if wantSource != "" {
		fmt.Fprintf(stdout, "候选池 %d 条（本次命中 %d 条 · --source %s ⇒ 命中 %d）\n", len(pb.Cands), total, wantSource, total)
	} else {
		fmt.Fprintf(stdout, "候选池 %d 条（本次命中 %d 条）\n", len(pb.Cands), total)
	}
	fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s\n", pad("cid", cidw), pad("source", srcw), pad("status", stw), "落池时刻", "症状")
	for _, c := range out {
		fmt.Fprintf(stdout, "  %s  %s  %s  %s  %s\n",
			pad(c.CID, cidw), pad(c.Source, srcw), pad(c.Status, stw), c.CreatedAt, truncateDisplay(c.Raw, 40))
	}
	return exitOK
}

// cmdGapIdeaAdd —— `zerg gap idea add`：人随手记 ⇒ **落候选池**（2-4）。★ `gap add` 仍是唯一入账。
func cmdGapIdeaAdd(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapIdeaFields, ","))
		return exitUsage
	}
	symptom := strings.TrimSpace(inv.flagVal("--symptom"))
	source := strings.TrimSpace(inv.flagVal("--source"))
	unit := strings.TrimSpace(inv.flagVal("--unit"))
	if symptom == "" {
		inv.setErr("usage", "missing_required", "缺 --symptom")
		fmt.Fprintf(stderr, "%s: `gap idea add` 缺必填旗标：--symptom\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap idea add --symptom <一句> [--source <%s>] [--unit <件路径>] [--dry-run | --yes]\n",
			strings.Join(gapSourceClosed, "|"))
		return exitUsage
	}
	if source == "" {
		source = "human" // 人随手记的入口 ⇒ 缺省档（回执点名这一条取定）
	}
	if !gapIn(gapSourceClosed, source) {
		inv.setErr("usage", "bad_source", "source 值不在四值闭集里")
		fmt.Fprintf(stderr, "%s: `--source %s` 不在闭集里 —— 只认 %s\n", progName, source, gapClosedText(gapSourceClosed))
		fmt.Fprintf(stderr, "  `source` 四值 = 采集层三源 + 人面随手记（设计稿 §9.1 第 2 条逐字）\n")
		return exitUsage
	}
	if unit != "" && strings.HasPrefix(unit, "-") {
		inv.setErr("usage", "bad_unit", "--unit 值看起来像旗标")
		fmt.Fprintf(stderr, "%s: `--unit %s` 看起来像旗标 ⇒ 拒收（值旗标不吃旗标）\n", progName, unit)
		return exitUsage
	}
	raw := symptom
	if unit != "" {
		raw = symptom + " · unit=" + unit
	}
	cand := gapCandidate{Source: source, Raw: raw, Unit: unit, CreatedAt: gapNow(),
		TTLDays: gapPoolTTLDays, Status: "pending"}
	cand.FP = sha256Of([]byte(source + "\x00" + raw))[:12]
	poolPath := gapPoolPath()

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed rc=2，计划件走 stderr（同族写面口径）。
	if !inv.dryRun && !inv.yes {
		fmt.Fprintf(stderr, "%s: 缺 `--yes`（D2 档 · 本族写面口径）—— 落池面一个字节不写\n", progName)
		fmt.Fprintf(stderr, "  池件     : %s（%s）\n", poolPath, gapPoolCountText(mustReadGapPoolQuiet()))
		fmt.Fprintf(stderr, "  候选     : source=%s · status=pending · ttl_days=%d · fp=%s\n", cand.Source, cand.TTLDays, cand.FP)
		fmt.Fprintf(stderr, "  真源     : %s（**本面不碰**）\n", gapLedgerPath())
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件（同族 `gap add` 的 `H-10` 口径）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	pb, err := readGapPool()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), poolPath)
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return exitBlocked
	}
	before := len(pb.Cands)
	for _, c := range pb.Cands {
		if c.FP == cand.FP && c.Source == cand.Source {
			inv.changed = boolPtr(false)
			if inv.jsonGiven {
				return selectJSON(stdout, stderr, inv, inv.path, inv.fields, map[string]string{
					"cid": c.CID, "source": c.Source, "raw": c.Raw, "unit": c.Unit,
					"created_at": c.CreatedAt, "ttl_days": strconv.Itoa(c.TTLDays), "status": c.Status, "fp": c.FP,
					"pool_before": strconv.Itoa(before), "pool_after": strconv.Itoa(before),
					"archived": "0", "ledger_lines": strconv.Itoa(gapLedgerLineCount(gapLedgerPath())),
				})
			}
			fmt.Fprintf(stdout, "已在池 %s · fp=%s（幂等命中：同 source 同 fp ⇒ 0 且不新增行 · 设计稿 §3.A3）\n", c.CID, shortSHA(c.FP))
			return exitOK
		}
	}
	cand.CID = gapPoolNextCID(pb.Cands, time.Now().Format("20060102"))
	keepLines, archLines, nArch := gapPoolSweep(pb, time.Now())
	ledgerLines := gapLedgerLineCount(gapLedgerPath())

	// ③ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		if inv.jsonGiven {
			inv.changed = boolPtr(false)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, map[string]string{
				"cid": cand.CID, "source": cand.Source, "raw": cand.Raw, "unit": cand.Unit,
				"created_at": cand.CreatedAt, "ttl_days": strconv.Itoa(cand.TTLDays), "status": cand.Status, "fp": cand.FP,
				"pool_before": strconv.Itoa(before), "pool_after": strconv.Itoa(len(keepLines) + 1),
				"archived": strconv.Itoa(nArch), "ledger_lines": strconv.Itoa(ledgerLines),
			})
		}
		fmt.Fprintf(stdout, "（--dry-run 计划件 · 零副作用）\n")
		fmt.Fprintf(stdout, "  池件     : %s（现有 %d 条）\n", poolPath, before)
		fmt.Fprintf(stdout, "  将落池   : %s · source=%s · ttl_days=%d · fp=%s\n", cand.CID, cand.Source, cand.TTLDays, cand.FP)
		fmt.Fprintf(stdout, "  七日归档 : 超期 %d 条 ⇒ %s（真跑才搬）\n", nArch, gapPoolArchivePath(time.Now().Format("20060102")))
		fmt.Fprintf(stdout, "  真源     : %s（**本面不碰** · 现有 %d 行）\n", gapLedgerPath(), ledgerLines)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未落池、未归档、真源一字未动）\n")
		return exitOK
	}

	// ④ 真写：先七日归档（超期条搬入归档件 + 主池回写未归档那批），再追加新候选。
	if nArch > 0 {
		if err := gapPoolAppendLines(gapPoolArchivePath(time.Now().Format("20060102")), archLines); err != nil {
			inv.setErr("blocked", "pool_archive_unwritable", err.Error())
			fmt.Fprintf(stderr, "%s: 归档件写不进 ⇒ 池件一个字节不写（退码 8）：%v\n", progName, err)
			return exitBlocked
		}
		if err := gapPoolWriteLines(poolPath, keepLines); err != nil {
			inv.setErr("blocked", "pool_unwritable", err.Error())
			fmt.Fprintf(stderr, "%s: 池件回写不进（归档件已落）：%v\n", progName, err)
			return exitBlocked
		}
	}
	b, err := json.Marshal(cand)
	if err != nil {
		inv.setErr("failed", "pool_encode_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 候选这一行序列化不过：%v\n", progName, err)
		return exitFail
	}
	if err := gapPoolAppendLines(poolPath, []string{string(b)}); err != nil {
		inv.setErr("blocked", "pool_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 池件写不进：%v\n", progName, err)
		return exitBlocked
	}
	inv.changed = boolPtr(true)
	after := before - nArch + 1
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, map[string]string{
			"cid": cand.CID, "source": cand.Source, "raw": cand.Raw, "unit": cand.Unit,
			"created_at": cand.CreatedAt, "ttl_days": strconv.Itoa(cand.TTLDays), "status": cand.Status, "fp": cand.FP,
			"pool_before": strconv.Itoa(before), "pool_after": strconv.Itoa(after),
			"archived": strconv.Itoa(nArch), "ledger_lines": strconv.Itoa(ledgerLines),
		})
	}
	fmt.Fprintf(stdout, "已落池 %s · source=%s · status=pending · fp=%s\n", cand.CID, cand.Source, shortSHA(cand.FP))
	fmt.Fprintf(stdout, "  池件     : %s（%d → %d 条%s）\n", poolPath, before, after,
		func() string {
			if nArch > 0 {
				return fmt.Sprintf(" · 七日归档 %d 条 ⇒ %s", nArch, gapPoolArchivePath(time.Now().Format("20060102")))
			}
			return ""
		}())
	fmt.Fprintf(stdout, "  真源     : %s（**一字未动** · %d 行 · 本面只读不写）\n", gapLedgerPath(), ledgerLines)
	return exitOK
}

// mustReadGapPoolQuiet —— 缺 `--yes` 那一态只想报个池条数（读不动就报「读不到」，**不据此改退码**）。
func mustReadGapPoolQuiet() gapPool {
	pb, err := readGapPool()
	if err != nil {
		return gapPool{}
	}
	return pb
}

// ── 二.5 候选池**出口面**（批2 第二片 · 2026-09-28）──────────────────────────────────────────
//
// 设计出处（唯一真源）：`设计-缺口账与自进化-v2.0-20260928.md`
//
//	§11.3「**出口**：建议 `zerg gap intake ls|show|promote|discard`；`promote` **复用 `gap add` 的
//	既有校验（不许绕）**」—— 本片按已落地的 `gap idea` 命名面实现（`ls`/`add` 已在第一片落地）。
//	§4 判据 1「**自记**：候选→入账转换率 + 来源分布（`source` 字段）——没有来源分布，就不知道
//	哪层验证在起作用」⇒ `gap idea stats` 两格都报。
//
// 四条出口的口径（与 `gap idea ls|add` 逐字同族）：
//
//	① `promote <cid>`  —— 池内一条 ⇒ **正式账**：走 `gapAddApply`（六必填 / impact 闭集 / prio 缺省 /
//	   人面禁写 state / `--verify-cmd` 命令树解析 / 同 fp 幂等与 `14` / 审计先落盘 / 读回对拍 —— **一条不绕**）。
//	   ★ 只有 `status=pending` 可提升；其余**拒收 2 并点名当前 status**。提升后池行**不删**：
//	   `status=promoted` + 记新格 `gap_id`（提升**先落正式账、后改池行** —— 池件写不进 ⇒ 退 8 并如实报
//	   「正式账已落」）。
//	② `show <cid>`     —— 单条**完整正文不截断** + 提升后的 `gap_id`；只读（不写池、不写真源、不写审计）。
//	③ `discard <cid>`  —— `status=expired`（**不是删件**：池行不删、真源一个字节不碰）；缺 `--yes` ⇒ 2。
//	④ `stats`          —— 池内各 status 计数 + 来源分布 + **转换率**（分母逐字写出）；**只报告、不进退码**。
//
// 退码口径（本族惯例，两因机器可辨）：缺 `--yes` ⇒ 2（fail-closed · 从不提问）· 池内没有该 cid ⇒ 2
// （同 `gap show`：账内没有这个 id = 用法错）· 当前 status 非 `pending` ⇒ 2 并点名 ·
// 池件不在盘 / 读不到 ⇒ 8（`ledger_absent` ⇄ `precondition_missing`）· 真源或池件写不进 ⇒ 8。

// gapIdeaShowFields —— `gap idea show` 的 `--json` 可取字段（单条全文 + 提升后的 `gap_id`）。
var gapIdeaShowFields = []string{"cid", "source", "raw", "unit", "created_at", "ttl_days", "status", "fp", "gap_id", "age_days"}

// gapIdeaPromoteFields —— `gap idea promote` 的 `--json` 可取字段（正式账那一格的 `id` = `gap_id`）。
var gapIdeaPromoteFields = []string{"cid", "source", "status", "gap_id", "fp", "ledger_lines", "pool_total"}

// gapIdeaDiscardFields —— `gap idea discard` 的 `--json` 可取字段。
var gapIdeaDiscardFields = []string{"cid", "source", "status", "fp", "pool_total"}

// gapIdeaStatsFields —— `gap idea stats` 的 `--json` 可取字段（一行一格：`status` 计数 / `source` 分布 / 转换率）。
var gapIdeaStatsFields = []string{"scope", "key", "count", "share"}

// gapIdeaStatusClosed —— `status` 四值闭集（逐字照 §11.3：`pending|promoted|expired|merged`）。
var gapIdeaStatusClosed = []string{"pending", "promoted", "expired", "merged"}

// gapIdeaCandBy —— 池内点名**恰好一条**（同族 `gap show`：池内没有这个 cid ⇒ 用法错 2）。
func gapIdeaCandBy(inv *invocation, pb gapPool, stderr io.Writer, verb string) (int, int) {
	args := []string{}
	for _, a := range inv.args {
		if s := strings.TrimSpace(a); s != "" {
			args = append(args, s)
		}
	}
	if len(args) == 0 {
		inv.setErr("usage", "target_required", "缺目标")
		fmt.Fprintf(stderr, "%s: `gap idea %s` 要给目标：`zerg gap idea %s <cid>`\n", progName, verb, verb)
		fmt.Fprintf(stderr, "  看池 : zerg gap idea ls\n")
		return -1, exitUsage
	}
	if len(args) > 1 {
		inv.setErr("usage", "too_many_targets", "位置参数多于一条")
		fmt.Fprintf(stderr, "%s: `gap idea %s` 只取**恰好一条**（本面按 `cid` 点名）：收到 %s\n", progName, verb, strings.Join(args, " "))
		fmt.Fprintf(stderr, "  （本族口径同 `gap show`：单条面不收多条）\n")
		return -1, exitUsage
	}
	cid := args[0]
	for i, c := range pb.Cands {
		if c.CID == cid {
			return i, exitOK
		}
	}
	inv.setErr("usage", "cand_not_found", "池内没有这个 cid")
	fmt.Fprintf(stderr, "%s: 池内没有 %s（池里 %d 条）：%s\n", progName, cid, len(pb.Cands), pb.Path)
	fmt.Fprintf(stderr, "  下一步 : 看池 `zerg gap idea ls`；记一条 `zerg gap idea add --symptom <一句> --yes`\n")
	return -1, exitUsage
}

// gapReadPoolOrDie —— 读池面的**两因分档**（读不到 ⇒ 8 `precondition_missing`；件不在盘 ⇒ 8 `ledger_absent`）。
// 与 `cmdGapIdeaLs` 逐字同款 —— 四条出口共用一处，免得四个面各抄一遍口径。
func gapReadPoolOrDie(inv *invocation, stderr io.Writer) (gapPool, int, bool) {
	pb, err := readGapPool()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapPoolPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "池件 = %s；「读不到」不许当「没有」（退码 8）\n", gapPoolPath())
		return pb, exitBlocked, false
	}
	if !pb.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "候选池件不在盘上", pb.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 候选池件不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, pb.Path)
		fmt.Fprintf(stderr, "  落一条候选：zerg gap idea add --symptom <一句> --yes\n")
		return pb, exitBlocked, false
	}
	return pb, exitOK, true
}

// gapIdeaStatusLine —— 池内一条 status 的「点名 + 建议」那句（三条出口共用一处措辞）。
func gapIdeaStatusLine(w io.Writer, c gapCandidate) {
	switch c.Status {
	case "promoted":
		fmt.Fprintf(w, "  已提升   : %s（正式账 —— `zerg gap show %s`）\n", c.GapID, c.GapID)
	case "expired":
		fmt.Fprintf(w, "  已作废   : 由 `gap idea discard` 或七日归档置 `expired`（池行仍在）\n")
	default:
		fmt.Fprintf(w, "  （要提升走 `zerg gap idea promote %s --handmade … --impact … --want-family … --want-action … --repro-cmd … --verify-cmd … --yes`）\n", c.CID)
	}
}

// gapIdeaNotPendingNext —— 非 `pending` 那一档的「下一步」一句（有 `gap_id` 才点名正式账）。
func gapIdeaNotPendingNext(c gapCandidate) string {
	switch c.Status {
	case "promoted":
		return fmt.Sprintf("已提升 ⇒ 看 `zerg gap show %s`", c.GapID)
	case "expired":
		return "已作废（池行仍在）⇒ 等七日归档搬走，或另记一条"
	default:
		return "本条不是 `pending` ⇒ 不改池行"
	}
}

// gapPoolRewriteRow —— 池内**点名那一行**改写（其余行逐字节不动 · 池行**不删** · 行序不变）。
func gapPoolRewriteRow(pb gapPool, idx int, c gapCandidate) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	lines := append([]string{}, pb.Lines...)
	lines[idx] = string(b)
	return gapPoolWriteLines(pb.Path, lines)
}

// cmdGapIdeaShow —— `zerg gap idea show <cid>`：池内一条**完整正文不截断**（只读面）。
func cmdGapIdeaShow(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapIdeaShowFields, ","))
		return exitUsage
	}
	pb, rc, ok := gapReadPoolOrDie(inv, stderr)
	if !ok {
		return rc
	}
	idx, rc := gapIdeaCandBy(inv, pb, stderr, "show")
	if rc != exitOK {
		return rc
	}
	c := pb.Cands[idx]
	inv.changed = boolPtr(false)
	age := ""
	if t, okTS := gapParseTS(c.CreatedAt); okTS {
		age = strconv.Itoa(int(time.Since(t).Hours() / 24))
	}
	row := map[string]string{
		"cid": c.CID, "source": c.Source, "raw": c.Raw, "unit": c.Unit,
		"created_at": c.CreatedAt, "ttl_days": strconv.Itoa(c.TTLDays),
		"status": c.Status, "fp": c.FP, "gap_id": c.GapID, "age_days": age,
	}
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, gapIdeaShowFields, row)
	}
	ageText := "（落池时刻解析不出）"
	if age != "" {
		ageText = age + " 天"
	}
	fmt.Fprintf(stdout, "候选 %s\n", c.CID)
	fmt.Fprintf(stdout, "  来源     : %s\n", c.Source)
	fmt.Fprintf(stdout, "  状态     : %s\n", c.Status)
	fmt.Fprintf(stdout, "  指纹     : %s\n", shortSHA(c.FP))
	fmt.Fprintf(stdout, "  落池时刻 : %s（在池 %s）\n", c.CreatedAt, ageText)
	fmt.Fprintf(stdout, "  存活     : ttl_days=%d（超期 ⇒ 七日归档搬入同目录 `%s*` 件）\n", c.TTLDays, gapPoolArchivePrefix)
	if c.Unit != "" {
		fmt.Fprintf(stdout, "  点名件   : %s\n", c.Unit)
	}
	fmt.Fprintf(stdout, "  症状     : %s\n", c.Raw)
	if c.GapID != "" {
		fmt.Fprintf(stdout, "  提升为   : %s（正式账 —— `zerg gap show %s`）\n", c.GapID, c.GapID)
	} else {
		fmt.Fprintf(stdout, "  提升为   : （未提升）\n")
	}
	gapIdeaStatusLine(stdout, c)
	fmt.Fprintf(stdout, "  真源     : %s（**本面一字不碰** · 只读）\n", gapLedgerPath())
	fmt.Fprintf(stdout, "  （本面**完整正文不截断**；`gap idea ls` 那一行只印症状前 40 显示宽）\n")
	return exitOK
}

// cmdGapIdeaPromote —— `zerg gap idea promote <cid>`：池内一条 ⇒ **正式账**（设计稿 §11.3：复用 `gap add`）。
func cmdGapIdeaPromote(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapIdeaPromoteFields, ","))
		return exitUsage
	}
	pb, rc, ok := gapReadPoolOrDie(inv, stderr)
	if !ok {
		return rc
	}
	idx, rc := gapIdeaCandBy(inv, pb, stderr, "promote")
	if rc != exitOK {
		return rc
	}
	c := pb.Cands[idx]
	// ① 只有 `status=pending` 可提升（其余 ⇒ 用法错 2 并点名当前 status）。
	if c.Status != "pending" {
		inv.setErr("usage", "cand_not_pending", "当前 status 不是 pending")
		fmt.Fprintf(stderr, "%s: %s 当前 status = %s ⇒ **只 `pending` 可提升**（拒收 · 用法错 2）\n", progName, c.CID, c.Status)
		fmt.Fprintf(stderr, "  `status` 四值：%s（设计稿 §11.3 逐字）\n", gapClosedText(gapIdeaStatusClosed))
		fmt.Fprintf(stderr, "  下一步 : `pending` 之外不改池行 —— %s\n", gapIdeaNotPendingNext(c))
		return exitUsage
	}
	// ② 入参 = 池件那一格 `raw` 当 `--symptom`，其余六必填走本命令旗标 —— 校验与落账**全交给 `gapAddApply`**。
	in := gapAddInputFromFlags(inv)
	in.Symptom = c.Raw
	in.Fields = gapIdeaPromoteFields
	out := &gapAddOut{}
	in.Out = out
	// `--json` 的包封由本命令出口发（字段表是 promote 的）；落账那一层只做「不给人面」。
	jsonWanted, fieldsKeep := inv.jsonGiven, inv.fields
	inv.jsonGiven, inv.fields = false, nil
	ledOut := stdout
	if jsonWanted {
		ledOut = io.Discard // 机器面取代人面（同族老规矩）：`--json` 时落账那一层的**人面一行都不出**
	}
	ledRC := gapAddApply(inv, ledOut, stderr, in)
	inv.jsonGiven, inv.fields = jsonWanted, fieldsKeep
	if ledRC != exitOK {
		fmt.Fprintf(stderr, "  ⚠ 提升未成 ⇒ 池行**一个字未动**（%s 仍 status=%s · 真源 %d 行）\n",
			c.CID, c.Status, gapLedgerLineCount(gapLedgerPath()))
		return ledRC
	}
	if inv.dryRun || !out.Landed {
		// 干跑 / 幂等命中：真源没新增行 ⇒ 池行也不改（两态对拍：这一档池与账都逐字不变）。
		if inv.dryRun {
			fmt.Fprintf(stdout, "（--dry-run：池行**一个字未动** —— %s 仍 status=%s；真跑才置 `promoted` + 记 `gap_id`）\n", c.CID, c.Status)
		} else {
			fmt.Fprintf(stdout, "（幂等命中：真源已有 %s ⇒ 不新增行 ⇒ 池行**一个字未动**（%s 仍 status=%s）· 要标记走真跑一次）\n",
				out.ID, c.CID, c.Status)
		}
		return exitOK
	}
	// ③ 正式账**已落**：池行**不删**，只把 `status` 置 `promoted` 并记 `gap_id`（先账后池 · 顺序写死）。
	c.Status, c.GapID = "promoted", out.ID
	if err := gapPoolRewriteRow(pb, idx, c); err != nil {
		inv.setErr("blocked", "pool_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 正式账**已落** %s，而池件回写不进：%v\n", progName, out.ID, err)
		fmt.Fprintf(stderr, "  ⇒ 退码 8（如实报：账里已多这一行；池里 %s 这一行仍是 %s）\n", c.CID, "pending")
		return exitBlocked
	}
	poolTotal := len(pb.Cands)
	if jsonWanted {
		return selectJSON(stdout, stderr, inv, inv.path, gapIdeaPromoteFields, map[string]string{
			"cid": c.CID, "source": c.Source, "status": c.Status, "gap_id": c.GapID, "fp": c.FP,
			"ledger_lines": strconv.Itoa(gapLedgerLineCount(gapLedgerPath())), "pool_total": strconv.Itoa(poolTotal),
		})
	}
	fmt.Fprintf(stdout, "已提升 %s ⇒ 正式账 %s · status=promoted（池行**不删** · 池仍 %d 条）\n", c.CID, c.GapID, poolTotal)
	fmt.Fprintf(stdout, "  下一步 : zerg gap show %s（看正式账那一行）· zerg gap idea stats（看转换率）\n", c.GapID)
	return exitOK
}

// cmdGapIdeaDiscard —— `zerg gap idea discard <cid>`：`status=expired`（**不是删件**：池行不删、真源不碰）。
func cmdGapIdeaDiscard(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapIdeaDiscardFields, ","))
		return exitUsage
	}
	pb, rc, ok := gapReadPoolOrDie(inv, stderr)
	if !ok {
		return rc
	}
	idx, rc := gapIdeaCandBy(inv, pb, stderr, "discard")
	if rc != exitOK {
		return rc
	}
	c := pb.Cands[idx]
	if c.Status != "pending" {
		inv.setErr("usage", "cand_not_pending", "当前 status 不是 pending")
		fmt.Fprintf(stderr, "%s: %s 当前 status = %s ⇒ **只 `pending` 可作废**（拒收 · 用法错 2）\n", progName, c.CID, c.Status)
		fmt.Fprintf(stderr, "  `status` 四值：%s（设计稿 §11.3 逐字）\n", gapClosedText(gapIdeaStatusClosed))
		return exitUsage
	}
	// 缺 `--yes`（且非 `--dry-run`）：fail-closed rc=2，池件一个字节不写（同族写面口径）。
	if !inv.dryRun && !inv.yes {
		fmt.Fprintf(stderr, "%s: 缺 `--yes`（D2 档 · 本族写面口径）—— 池件一个字节不写\n", progName)
		fmt.Fprintf(stderr, "  池件     : %s（%d 条）\n", pb.Path, len(pb.Cands))
		fmt.Fprintf(stderr, "  候选     : %s · source=%s · status=%s\n", c.CID, c.Source, c.Status)
		fmt.Fprintf(stderr, "  将置     : status=expired（**池行不删** · 真源 `%s` **一字不动**）\n", gapLedgerPath())
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}
	nb := c
	nb.Status = "expired"
	if inv.dryRun {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, gapIdeaDiscardFields, map[string]string{
				"cid": nb.CID, "source": nb.Source, "status": nb.Status, "fp": nb.FP,
				"pool_total": strconv.Itoa(len(pb.Cands)),
			})
		}
		fmt.Fprintf(stdout, "（--dry-run 计划件 · 零副作用）\n")
		fmt.Fprintf(stdout, "  池件     : %s（%d 条）\n", pb.Path, len(pb.Cands))
		fmt.Fprintf(stdout, "  将置     : %s ⇒ status=expired（池行**不删** · 真源一字未动）\n", c.CID)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 池件一个字节未写）\n")
		return exitOK
	}
	if err := gapPoolRewriteRow(pb, idx, nb); err != nil {
		inv.setErr("blocked", "pool_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 池件写不进 ⇒ 一个字节没改（退码 8）：%v\n", progName, err)
		return exitBlocked
	}
	inv.changed = boolPtr(true)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, gapIdeaDiscardFields, map[string]string{
			"cid": nb.CID, "source": nb.Source, "status": nb.Status, "fp": nb.FP,
			"pool_total": strconv.Itoa(len(pb.Cands)),
		})
	}
	fmt.Fprintf(stdout, "已作废 %s · status=expired（池行**不删** · 池仍 %d 条）\n", nb.CID, len(pb.Cands))
	fmt.Fprintf(stdout, "  真源     : %s（**一字未动** · %d 行 · 本面只读不写）\n", gapLedgerPath(), gapLedgerLineCount(gapLedgerPath()))
	return exitOK
}

// gapIdeaShareText —— 分布读数的一格 `share`（分母**逐字写出**：`n/total`）；`total=0` ⇒ `0/0`。
func gapIdeaShareText(n, total int) string {
	if total <= 0 {
		return "0/0"
	}
	return fmt.Sprintf("%d/%d", n, total)
}

// cmdGapIdeaStats —— `zerg gap idea stats`：设计稿 §4 判据 1 的两格读数（**只报告、不进退码**）。
//
// 人面 = 池内各 `status` 计数 + 来源分布 + **转换率**：
//
//	转换率 = promoted / (promoted + expired + pending)     ← 分母逐字写在这一行（`merged` 不计入：
//	它由别的面写、本片没有产出者 ⇒ 不计进分母，也不静默塞 0 —— 计数面照样列出来）。
func cmdGapIdeaStats(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapIdeaStatsFields, ","))
		return exitUsage
	}
	pb, rc, ok := gapReadPoolOrDie(inv, stderr)
	if !ok {
		return rc
	}
	st, src := map[string]int{}, map[string]int{}
	for _, c := range pb.Cands {
		st[c.Status]++
		src[c.Source]++
	}
	prom, exp, pend := st["promoted"], st["expired"], st["pending"]
	denom := prom + exp + pend
	rate := "n/a（分母 0）"
	if denom > 0 {
		rate = fmt.Sprintf("%.4f", float64(prom)/float64(denom))
	}
	total := len(pb.Cands)
	inv.changed = boolPtr(false)
	inv.metaAddJSON("total", strconv.Itoa(total))
	inv.metaAddJSON("hits", strconv.Itoa(total))
	inv.metaAddStr("pool_sha16", gapLsLedgerSHA16(pb.Path))
	inv.metaAddStr("conversion", rate)
	inv.metaAddStr("conversion_denominator", "promoted/(promoted+expired+pending)")
	inv.metaAddJSON("conversion_numer_denom", fmt.Sprintf(`{"promoted":%d,"denominator":%d}`, prom, denom))
	rows := []map[string]string{}
	for _, s := range gapIdeaStatusClosed {
		rows = append(rows, map[string]string{"scope": "status", "key": s, "count": strconv.Itoa(st[s]), "share": gapIdeaShareText(st[s], total)})
	}
	for _, s := range gapSourceClosed {
		rows = append(rows, map[string]string{"scope": "source", "key": s, "count": strconv.Itoa(src[s]), "share": gapIdeaShareText(src[s], total)})
	}
	rows = append(rows, map[string]string{
		"scope": "conversion", "key": "promoted/(promoted+expired+pending)",
		"count": strconv.Itoa(prom), "share": rate,
	})
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, gapIdeaStatsFields, rows)
	}
	fmt.Fprintf(stdout, "候选池 %s · %d 条（设计稿 §4 判据 1：候选→入账转换率 + 来源分布）\n", pb.Path, total)
	fmt.Fprintf(stdout, "  状态分布 : ")
	for i, s := range gapIdeaStatusClosed {
		if i > 0 {
			fmt.Fprintf(stdout, " · ")
		}
		fmt.Fprintf(stdout, "%s=%d", s, st[s])
	}
	fmt.Fprintf(stdout, "\n  来源分布 : ")
	for i, s := range gapSourceClosed {
		if i > 0 {
			fmt.Fprintf(stdout, " · ")
		}
		fmt.Fprintf(stdout, "%s=%d", s, src[s])
	}
	fmt.Fprintf(stdout, "\n  转换率   : %s\n", rate)
	fmt.Fprintf(stdout, "  分母     : promoted / (promoted + expired + pending) = %d / (%d + %d + %d) = %d\n", prom, prom, exp, pend, denom)
	fmt.Fprintf(stdout, "  （`merged` 不计入分母：本片没有它的产出者 · 面照样列出来）\n")
	fmt.Fprintf(stdout, "  真源     : %s（**本面一字不碰** · 只读）\n", gapLedgerPath())
	return exitOK
}

// ── ⒠ `zerg gap assign ls [--egg <卵号>]`（批3 第一片 · 只读面）────────────────────────────────────
//
// 设计出处：§11.3（派单登记读面）+ §4 判据 2（自派：`已派` 条数 == 在飞卵数 —— 本面就是那两枚读数的落点）。
// 只读：**一字不写真源、不写审计**（零副作用）· 信封走同族写法（`gapLsMetaAdd` 那一套，不另造）。
// 零命中 ⇒ 退码 1（「没有」不是「失败」· 也不是绿）；真源读不到 ⇒ 8。

// gapAssignMetaAdd —— `gap assign ls` 的信封字段（写法照同族 `gapLsMetaAdd` / `gapIdeaMetaAdd` · 不另造）。
func gapAssignMetaAdd(inv *invocation, egg string, aging, stale bool, total, hits int, ledgerPath string) {
	inv.metaAddJSON("total", strconv.Itoa(total))
	inv.metaAddJSON("hits", strconv.Itoa(hits))
	inv.metaAddJSON("query", gapAssignQueryText(egg, aging, stale))
	inv.metaAddStr("query_ts", gapNow())
	inv.metaAddStr("ledger_sha16", gapLsLedgerSHA16(ledgerPath))
}

// gapAssignQueryText —— `meta.query` 的回显（现读收窄入参 · 序 = egg / aging / stale）。
// 未给的旗标**不写那一格** ⇒ 无收窄时逐字 `{}`（缺席 ≠ 假值 —— 与同族各面同一条口径）。
func gapAssignQueryText(egg string, aging, stale bool) string {
	parts := []string{}
	if egg != "" {
		parts = append(parts, jstr("egg")+":"+jstr(egg))
	}
	if aging {
		parts = append(parts, jstr("aging")+":true")
	}
	if stale {
		parts = append(parts, jstr("stale")+":true")
	}
	if len(parts) == 0 {
		return "{}"
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// gapAssignItem —— `assign ls` 的一行（账上那条 + 它的作业单登记 + 登记时刻）。
type gapAssignItem struct {
	r    gapRecord
	f    gapAssignForm
	at   string
	regd bool
}

// gapAssignDeadlineLayouts —— 时限那一格认的写法（**同一套**照 `family_gap_export.go:gapFoundDays`：
// 纯日期 / RFC3339 / 日期+时刻）。认不出 ⇒ 归「时限不明」（**不猜** ✗）—— 不得当零。
var gapAssignDeadlineLayouts = []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05"}

// gapAssignDeadlineAt —— 把⑤时限那一格解析成**截止时刻**（纯日期 ⇒ 该日**末尾** 23:59:59：
// 「某日到期」= 那天过完才算超）。空 / 认不出 ⇒ `ok=false`（时限算不出 ⇒ 单列「时限不明」）。
func gapAssignDeadlineAt(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range gapAssignDeadlineLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			if layout == "2006-01-02" {
				t = t.Add(24*time.Hour - time.Second)
			}
			return t, true
		}
	}
	return time.Time{}, false
}

// gapAssignOverdue —— 一条派单的**超时读数**（现读现算 · 不猜）：`d` = 超出去多久、
// `known` = 时限那一格能不能解出截止时刻、`late` = 真超时（known 且现读晚于截止）。
func gapAssignOverdue(f gapAssignForm, now time.Time) (d time.Duration, known, late bool) {
	at, ok := gapAssignDeadlineAt(f.Deadline)
	if !ok {
		return 0, false, false
	}
	if now.After(at) {
		return now.Sub(at), true, true
	}
	return 0, true, false
}

// gapAssignDurText —— 超时时长的人面写法（天 / 时 / 分 三档 · 只报读数不猜）。
func gapAssignDurText(d time.Duration) string {
	mins := int(d.Minutes())
	if mins < 0 {
		mins = -mins
	}
	days, rem := mins/(24*60), mins%(24*60)
	hours, ms := rem/60, rem%60
	switch {
	case days > 0:
		return fmt.Sprintf("%d 天 %d 小时", days, hours)
	case hours > 0:
		return fmt.Sprintf("%d 小时 %d 分", hours, ms)
	default:
		return fmt.Sprintf("%d 分", ms)
	}
}

// gapAssignAgingText —— 一行 `aging` 格的三态（未超时 / 超时 <时长> / 时限不明）—— 人面与筛查共用。
func gapAssignAgingText(f gapAssignForm, now time.Time) string {
	d, known, late := gapAssignOverdue(f, now)
	switch {
	case !known:
		return "时限不明"
	case late:
		return "超时 " + gapAssignDurText(d)
	default:
		return "未超时"
	}
}

// gapAssignReconcileLine —— 对账口径行的**唯一**拼装口（人面表尾与 `meta.reconcile` **逐字同源**）。
// 判据 2「自派」的读数行：在飞卵数只存在于助手平台面 ⇒ **只报告不自动断言**。
func gapAssignReconcileLine(n int) string {
	return fmt.Sprintf("已派 %d 条 —— 请与平台在飞清单逐条比对（在飞不在本 CLI 真源内，故只报告不自动断言）", n)
}

// gapAssignFooter —— 表尾**固定**回显的对账口径行（逐字）；有超时条 ⇒ 再补一行点名（缺口号 + 卵号 + 超时时长）。
func gapAssignFooter(w io.Writer, assignTotal int, overdue []gapAssignItem, now time.Time) {
	fmt.Fprintf(w, "%s\n", gapAssignReconcileLine(assignTotal))
	if len(overdue) == 0 {
		return
	}
	parts := make([]string, 0, len(overdue))
	for _, a := range overdue {
		d, _, _ := gapAssignOverdue(a.f, now)
		parts = append(parts, fmt.Sprintf("%s · egg=%s · 超时 %s", a.r.ID, a.f.Egg, gapAssignDurText(d)))
	}
	fmt.Fprintf(w, "  超时未收点名：%s\n", strings.Join(parts, "；"))
}

// cmdGapAssignLs —— `zerg gap assign ls [--egg <卵号>] [--aging] [--stale]`：列 `已派` 条与它们的**作业单登记**。
func cmdGapAssignLs(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapAssignListFields, ","))
		return exitUsage
	}
	wantEgg := strings.TrimSpace(inv.flagVal("--egg"))
	wantAging := inv.hasFlag("--aging")
	wantStale := inv.hasFlag("--stale")
	led, rc := gapReadLedgerOrDie(inv, stderr)
	if rc != exitOK {
		return rc
	}
	now := time.Now()
	all := []gapAssignItem{}
	for _, r := range led.Recs {
		if r.State != gapStAssigned {
			continue
		}
		f, at, ok := gapAssignOf(r)
		if wantEgg != "" && (!ok || f.Egg != wantEgg) {
			continue
		}
		all = append(all, gapAssignItem{r, f, at, ok})
	}
	assignTotal := len(all)
	// 超时读数现读现算：`overdueAll` = 全 `已派` 里真超时的那批（点名行用它 —— 与收窄旗标无关地稳定）。
	overdueAll := []gapAssignItem{}
	for _, a := range all {
		if _, _, late := gapAssignOverdue(a.f, now); late {
			overdueAll = append(overdueAll, a)
		}
	}
	// `--stale` = 只列超时限未收（`已派` 本身即「未收」；「时限不明」算不出 ⇒ 不算超时 ⇒ 不列）。
	shown := all
	if wantStale {
		shown = overdueAll
	}
	// `--aging` = 按超时限程度降序（最久的在最前）；「时限不明」单列在尾（**不当零**）。
	unknown := []gapAssignItem{}
	if wantAging {
		okRows := make([]gapAssignItem, 0, len(shown))
		for _, a := range shown {
			if _, known, _ := gapAssignOverdue(a.f, now); !known {
				unknown = append(unknown, a)
				continue
			}
			okRows = append(okRows, a)
		}
		shown = okRows
		sort.SliceStable(shown, func(i, j int) bool {
			di, _, li := gapAssignOverdue(shown[i].f, now)
			dj, _, lj := gapAssignOverdue(shown[j].f, now)
			if li != lj {
				return li
			}
			return di > dj
		})
	}
	total := len(shown)
	out, cut := shown, false
	if total > gapLsRowCap {
		cut = true
		out = shown[:gapLsRowCap]
	}
	hits := len(out)

	// 零命中两档：`--stale` 口径 = 「无超时派单」⇒ rc 0（「没有超时」不是失败）；其余零命中仍 rc 1。
	if wantStale && total == 0 {
		inv.changed = boolPtr(false)
		gapAssignMetaAdd(inv, wantEgg, wantAging, wantStale, assignTotal, 0, led.Path)
		inv.metaAddJSON("assign_total", strconv.Itoa(assignTotal))
		inv.metaAddJSON("overdue_count", "0")
		inv.metaAddStr("reconcile", gapAssignReconcileLine(assignTotal))
		if inv.jsonGiven {
			emitEnvelopeWith(stdout, find(inv.path), "[]", 0, inv)
			return exitOK
		}
		fmt.Fprintf(stdout, "无超时派单（已派 %d 条里 0 条超时限未收%s）\n", assignTotal, gapAssignEggSuffix(wantEgg))
		gapAssignFooter(stdout, assignTotal, overdueAll, now)
		return exitOK
	}
	if total == 0 {
		inv.changed = boolPtr(false)
		inv.setErr("failed", "no_match", "零命中")
		fmt.Fprintf(stderr, "零命中：`已派` 0 条（账内 %d 条%s ⇒ 退码 1 —— 「没有」不是「失败」，也不是绿）\n",
			len(led.Recs), gapAssignEggSuffix(wantEgg))
		if inv.jsonGiven {
			gapAssignMetaAdd(inv, wantEgg, wantAging, wantStale, 0, 0, led.Path)
			emitEnvelopeWith(stdout, find(inv.path), "[]", 0, inv)
		}
		return exitFail
	}
	inv.changed = boolPtr(false)
	gapAssignMetaAdd(inv, wantEgg, wantAging, wantStale, total, hits, led.Path)
	inv.metaAddJSON("assign_total", strconv.Itoa(assignTotal))
	inv.metaAddJSON("overdue_count", strconv.Itoa(len(overdueAll)))
	inv.metaAddStr("reconcile", gapAssignReconcileLine(assignTotal))
	if cut {
		inv.markTruncated()
		inv.warnf("已裁 %d 条（gap assign ls 一页 %d 条 / 真命中 %d 条）", total-hits, hits, total)
		inv.metaAddJSON("truncated_detail", fmt.Sprintf(
			`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`, hits, total-hits, total))
		fmt.Fprintf(stderr, "%s: ⚠ 本页只列前 %d 条 · 真命中 %d 条（已裁 %d 条）—— **这不是全集**\n", progName, hits, total, total-hits)
	}
	rows := []map[string]string{}
	for _, a := range out {
		rows = append(rows, map[string]string{
			"id": a.r.ID, "state": a.r.State, "egg": a.f.Egg, "allow": a.f.Allow,
			"forbid": a.f.Forbid, "occupies": a.f.Occupies, "criterion": a.f.Criterion,
			"deadline": a.f.Deadline, "receipt": a.f.Receipt, "assigned_at": a.at,
			"unit": a.r.Unit, "module": a.r.Module,
		})
	}
	if inv.jsonGiven {
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	head := fmt.Sprintf("已派 %d 条（总账 %d 条%s）· 本页 %d 条", assignTotal, len(led.Recs), gapAssignEggSuffix(wantEgg), hits)
	if wantAging {
		head += " · --aging（按超时限降序）"
	}
	if wantStale {
		head += " · --stale（只列超时限未收）"
	}
	fmt.Fprintf(stdout, "%s\n", head)
	for _, a := range out {
		egg := a.f.Egg
		if !a.regd {
			egg = "（无登记 —— 孤儿：判据 2 的例外面）"
		}
		fmt.Fprintf(stdout, "  %s · egg=%s（%s）\n", a.r.ID, egg, orDash(a.at))
		fmt.Fprintf(stdout, "    ① 允许面   : %s\n", orDash(a.f.Allow))
		fmt.Fprintf(stdout, "    ② 禁碰面   : %s\n", orDash(a.f.Forbid))
		fmt.Fprintf(stdout, "    ③ 占用件   : %s\n", orDash(a.f.Occupies))
		fmt.Fprintf(stdout, "    ④ 出口判据 : %s\n", orDash(a.f.Criterion))
		dl := orDash(a.f.Deadline)
		if wantAging {
			dl = fmt.Sprintf("%s（%s）", dl, gapAssignAgingText(a.f, now))
		}
		fmt.Fprintf(stdout, "    ⑤ 时限     : %s\n", dl)
		fmt.Fprintf(stdout, "    ⑥ 该条全文 : %s\n", gapAssignFullText(a.r))
		fmt.Fprintf(stdout, "    回执       : %s\n", orDash(a.f.Receipt))
	}
	if wantAging && len(unknown) > 0 {
		fmt.Fprintf(stdout, "  ── 时限不明（%d 条 · 时限那一格解不出 ⇒ 单列 · **不当零**）：\n", len(unknown))
		for _, a := range unknown {
			fmt.Fprintf(stdout, "    %s · egg=%s · 时限=%s\n", a.r.ID, a.f.Egg, orDash(a.f.Deadline))
		}
	}
	fmt.Fprintf(stdout, "  件面     : %s\n", gapAssignUnitsText(out))
	fmt.Fprintf(stdout, "  真源     : %s（**本面一字不碰** · 只读）\n", led.Path)
	gapAssignFooter(stdout, assignTotal, overdueAll, now)
	return exitOK
}

// gapAssignEggSuffix —— 人面那句收窄回显（没给 `--egg` ⇒ 空串）。
func gapAssignEggSuffix(egg string) string {
	if egg == "" {
		return ""
	}
	return " · --egg " + egg
}

// gapAssignUnitsText —— 人面那一行「件面」（把占用件去重摊开 · 判据 2 的旁证）。
func gapAssignUnitsText(rows []gapAssignItem) string {
	seen := []string{}
	for _, a := range rows {
		for _, t := range gapAssignOccupies(a.f.Occupies) {
			if !gapIn(seen, t) {
				seen = append(seen, t)
			}
		}
	}
	if len(seen) == 0 {
		return "（本页没有占件）"
	}
	return strings.Join(seen, " · ")
}

// ── ⒡ `zerg gap plan <件|模块>`（**作业单六件** · 只读面 · 批3 第二片 · 2026-09-28）──────────────
//
// 设计出处（唯一真源 · `Zerg-内部文档/…/v2.5.13/设计-缺口账与自进化-v2.0-20260928.md`）：
//
//	§3.C `C1`（作业单：无 `plan` ⇒ 建议 `zerg gap plan <件|模块>`：该桶全部未闭 + 出口判据（该桶归零）
//	  + 建议允许面/禁碰面；直接产出派单模板）· §11.3（**作业单六件** · 逐字：① 该桶全部未闭（按 prio 排序）
//	  ② 出口判据 = 该桶归零 ③ 建议允许面（件清单）④ 禁碰面 ⑤ 占用状态（该件是否已有在飞卵）
//	  ⑥ 派单模板骨架）· §4 判据 2（`plan` 输出的作业单必须带**占用状态** · `M-32` 同改文件只许一路）。
//
// 本面的口径（逐条照设计稿 · 不自造）：
//
//	① **只读面**：一字不写真源、不写审计（与 `gap ls` / `gap status` / `gap assign ls` 同档）。
//	② **桶** = 选择器命中的账内条目。**选择器三处收敛到一对值**（选择器 + 面）：
//	   位置参数（首选 · 件路径**或**模块目录前缀 —— `--unit` 面的两形态）/ `--unit <值>` / `--module <值>`；
//	   `--module` 在场 ⇒ 面 = 模块（前导匹配 `gapLsModuleHit`），否则面 = 件（两形态 `gapLsUnitHit`）
//	   —— **与 `gap ls` 逐字同一条判据**（不另造一套前缀匹配）。
//	③ **未闭** = `state == 仍缺`（与 `gap status` 的 `Open` 计数**逐字同口径**）。`已派` 等其余态
//	   不进 ① 清单，落在 ⑤ 占用面（设计稿把「占用」单列为一件）。
//	④ **零未闭 ≠ 空件**：桶内有条目而 `仍缺 == 0` ⇒ **照出六件、真报「未闭 0」**（退码 0）；
//	   桶内**一条都没有** ⇒ 退码 1（「选择器没命中任何条目」是选择器的事，不是「零未闭」）。
//	⑤ 截断自报（`--top` 一页硬顶 = `gapLsRowCap` · **分页只在人面** ⇒ 机器面报 `truncated_detail`）。
//	⑥ 派生面**逐条可现算**：清单（①）/ 允许面（③）/ 占用（⑤）全部从真源现读现算，**不自填数字**。
//
// 退码：0 出单 / 1 选择器零命中或账内越界 / 2 用法错（缺选择器 / `--top` 非正整数 / `--json` 不给字段）/ 8 真源读不到。

// gapPlanDefaultTop —— `--top` 缺省一页条数（与 `gap status` 的 `gapStatusDefaultTop` 同值 20）。
const gapPlanDefaultTop = 20

// gapPlanFields —— `--json` 可取字段：前六格是**条目**字段（① 每行 = 一条未闭）；后十格是**桶级六件**
// 的派生值（②~⑥ + 选择器/面/桶量）—— 逐行**重复**给出（机器面要一次取到六件，不必再读 meta）。
// ②~⑥ 同时也进 `meta` 子键（人面与 meta 两条路都齐）。
var gapPlanFields = []string{"id", "prio", "state", "unit", "module", "summary",
	"selector", "face", "bucket", "open", "criterion", "allow", "forbid", "assigned", "deadline", "template"}

// gapPrioRank —— **复用** `family_gap_export.go:212` 的那一枚（P0<P1<P2 · 未知排最后）：
// 「按 prio 排序」（设计稿 ①）与导出面的排序键**同源**，不另造第二把尺。

// gapPlanSelector —— 选择器与面：位置参数（首选）→ `--unit <值>` → `--module <值>` 三处收敛到一对值。
// `--module` 在场（无论值）⇒ 面 = 模块；否则面 = 件（`--unit` 的两形态本身也吃目录前缀 ⇒ 位置参数给
// 模块目录前缀同样命中，正是设计稿「件路径或模块目录前缀」那一句）。
func gapPlanSelector(inv *invocation) (string, string) {
	sel := ""
	if len(inv.args) > 0 {
		sel = strings.TrimSpace(inv.args[0])
	}
	face := gapStatusByUnit
	if inv.hasFlag("--module") {
		face = gapStatusByModule
	}
	if sel == "" {
		if u := strings.TrimSpace(inv.flagVal("--unit")); u != "" {
			sel = u
		} else if m := strings.TrimSpace(inv.flagVal("--module")); m != "" {
			sel, face = m, gapStatusByModule
		}
	}
	return sel, face
}

// gapPlanHit —— 桶命中的**唯一**判据（与 `gap ls` 的两轴收窄同源：面 = 模块走 `gapLsModuleHit`，否则走 `gapLsUnitHit`）。
func gapPlanHit(r gapRecord, sel, face string) bool {
	if face == gapStatusByModule {
		return gapLsModuleHit(r, sel)
	}
	return gapLsUnitHit(r, sel)
}

// gapPlanQueryText —— `meta.query` 的回显（现读选择器三格 · 序 = selector / face / top）。
func gapPlanQueryText(sel, face string, top int) string {
	return "{" + jstr("selector") + ":" + jstr(sel) + "," + jstr("face") + ":" + jstr(face) +
		"," + jstr("top") + ":" + strconv.Itoa(top) + "}"
}

// gapPlanDeadline —— 派单模板里的「时限」（**具体绝对日期** · 不是占位符）：出单时刻 + 24h 的自然日。
func gapPlanDeadline() string { return time.Now().Add(24 * time.Hour).Format("2006-01-02") }

// gapPlanForbid / gapPlanForbidWhy —— ④ 禁碰面的**建议值**与**理由**（设计稿 ④ 要「建议值 + 理由」）。
// 值面写死（本面是只读建议面 · 不猜调用方的仓外布局）；理由逐条点名为什么这几处禁碰。
const (
	gapPlanForbid    = "仓内件（除③允许面所列外全禁） · bin/ · scripts/build/build-all.sh · 真源账 zerg-cli-gaps.jsonl（本面只读）"
	gapPlanForbidWhy = "卵只许改③给出的件（设计稿 §11.4 六件 ④）；真源账只由 `zerg gap` 正门写、**禁手搓**；" +
		"`bin/` 与 `scripts/build/build-all.sh` 是共享制品（§8 `M-32`：同改文件只许一路）"
)

// gapPlanCount —— 桶内某 `prio` 的条数（① 的近邻量化）。
func gapPlanCount(rs []gapRecord, prio string) int {
	n := 0
	for _, r := range rs {
		if r.Prio == prio {
			n++
		}
	}
	return n
}

// gapPlanMetaAdd —— `gap plan` 机器面信封字段的**唯一写入口**（只走 `metaAdd*` 既有口子 ⇒ 顶层六键不动）。
// 五格与 `gap ls` **同形**（`total` / `hits` / `query` / `query_ts` / `ledger_sha16`），另加本面六件的派生格
// （selector / face / bucket / criterion / allow / forbid / assigned / deadline / template）—— 缺一格六件就凑不齐。
func gapPlanMetaAdd(inv *invocation, sel, face string, top, bucketN, total, hits int,
	criterion, allow, assigned, deadline, template, ledgerPath string) {
	inv.metaAddJSON("total", strconv.Itoa(total))
	inv.metaAddJSON("hits", strconv.Itoa(hits))
	inv.metaAddJSON("query", gapPlanQueryText(sel, face, top))
	inv.metaAddStr("query_ts", gapNow())
	inv.metaAddStr("ledger_sha16", gapLsLedgerSHA16(ledgerPath))
	inv.metaAddStr("selector", sel)
	inv.metaAddStr("face", face)
	inv.metaAddJSON("bucket", strconv.Itoa(bucketN))
	inv.metaAddStr("criterion", criterion)
	inv.metaAddStr("allow", allow)
	inv.metaAddStr("forbid", gapPlanForbid)
	inv.metaAddJSON("assigned", assigned)
	inv.metaAddStr("deadline", deadline)
	inv.metaAddStr("template", template)
}

// cmdGapPlan —— `zerg gap plan <件|模块>`：把一件（或一模块）的未闭账变成**六件式作业单**（只读面）。
func cmdGapPlan(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapPlanFields, ","))
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `gap plan` 收**恰好一个**选择器（多给了 %d 枚：%q）\n",
			progName, len(inv.args)-1, inv.args[1])
		return exitUsage
	}
	sel, face := gapPlanSelector(inv)
	if sel == "" {
		inv.setErr("usage", "missing_required", "缺选择器")
		fmt.Fprintf(stderr, "%s: `gap plan` 缺选择器（件路径 / 模块目录前缀）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap plan <件路径|模块目录前缀> [--module <模块前缀>] [--top <N>] [--json <字段>]\n")
		return exitUsage
	}
	top := gapPlanDefaultTop
	if v := strings.TrimSpace(inv.flagVal("--top")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			inv.setErr("usage", "bad_top", "--top 要给正整数")
			fmt.Fprintf(stderr, "%s: `--top %s` 不是正整数（本族口径 = 用法错 2）\n", progName, v)
			return exitUsage
		}
		top = n
	}
	if top > gapLsRowCap {
		top = gapLsRowCap
	}

	// ② 读真源（读不到 ⇒ 8 · **不许当绿**）
	led, rc := gapReadLedgerOrDie(inv, stderr)
	if rc != exitOK {
		return rc
	}
	// ③ 账内闭集自查（越界 ⇒ 判红 1 + 点名到行 · 与 `gap ls`/`gap status` 同一条判据）
	for i, r := range led.Recs {
		if f, v, bad := gapOutOfRangeOne(r); bad {
			inv.setErr("failed", "ledger_out_of_range", fmt.Sprintf("第 %d 行 %s 越界", led.No[i], f))
			fmt.Fprintf(stderr, "账内越界：%d %s\n", led.No[i], f)
			fmt.Fprintf(stderr, "  %s 的值 %q 不在闭集里（%s）—— 真源只由命令写（防呆③ 同源）\n",
				f, v, gapClosedText(gapClosureOf(f)))
			return exitFail
		}
	}

	// ④ 分桶（选择器命中 —— 与 `gap ls` 同一条判据）· 同时收①未闭与③件清单
	bucket := []gapRecord{}
	open := []gapRecord{}
	units := []string{}
	for _, r := range led.Recs {
		if !gapPlanHit(r, sel, face) {
			continue
		}
		bucket = append(bucket, r)
		if r.Unit != "" && !gapIn(units, r.Unit) {
			units = append(units, r.Unit)
		}
		if r.State == gapStOpen {
			open = append(open, r)
		}
	}
	// ① 排序：prio（P0→P1→P2）· 同 prio 按缺口号
	sort.SliceStable(open, func(i, j int) bool {
		a, b := gapPrioRank(open[i].Prio), gapPrioRank(open[j].Prio)
		if a != b {
			return a < b
		}
		return open[i].ID < open[j].ID
	})
	// ⑤ 占用状态（桶内 `已派` 条 · 逐条点名卵号 —— 读账内 `state=已派` 与派单登记）
	occIDs, occEggs, occOcc := []string{}, []string{}, []string{}
	for _, r := range bucket {
		if r.State != gapStAssigned {
			continue
		}
		f, _, ok := gapAssignOf(r)
		egg := f.Egg
		if !ok {
			egg = "（无登记 —— 孤儿）"
		}
		occIDs = append(occIDs, r.ID)
		occEggs = append(occEggs, egg)
		occOcc = append(occOcc, f.Occupies)
	}

	// ⑤-a 选择器零命中（桶内一条都没有）⇒ 退码 1（**与「零未闭」是两回事**）
	if len(bucket) == 0 {
		inv.changed = boolPtr(false)
		inv.setErr("failed", "no_match", "选择器零命中")
		fmt.Fprintf(stderr, "零命中：账内 %d 条 · 选择器 %q（面 %s）相符 0 条（退码 1 —— 「没有」不是「零未闭」，也不是绿）\n",
			len(led.Recs), sel, face)
		inv.warnf("该选择器在账里没有任何条目 —— 请先 `zerg gap add` 入账，或用 `zerg gap status` 找有料的桶")
		if inv.jsonGiven {
			gapPlanMetaAdd(inv, sel, face, top, 0, 0, 0, "", "", "[]", "", "", led.Path)
			emitEnvelopeWith(stdout, find(inv.path), "[]", 0, inv)
		}
		return exitFail
	}

	inv.changed = boolPtr(false)
	// ② 出口判据（固定写「该桶未闭归零」并回显现读未闭数）
	criterion := fmt.Sprintf("该桶未闭归零（现读未闭数 = %d）", len(open))
	// ③ 建议允许面（该桶涉及的件清单）
	allow := "（该桶条目都未点名可改件 —— 兜底桶：无件可放）"
	if len(units) > 0 {
		allow = strings.Join(units, ",")
	}
	deadline := gapPlanDeadline()
	// ⑤-b 占用那一行/那一格
	occDetail := "无（该桶 0 条已派 · 无在飞卵占用）"
	if len(occIDs) > 0 {
		parts := make([]string, 0, len(occIDs))
		for i := range occIDs {
			parts = append(parts, fmt.Sprintf("%s ← 卵 %s", occIDs[i], occEggs[i]))
		}
		occDetail = strings.Join(parts, " · ")
	}
	occJSONParts := make([]string, 0, len(occIDs))
	for i := range occIDs {
		occJSONParts = append(occJSONParts, "{"+jstr("id")+":"+jstr(occIDs[i])+","+jstr("egg")+":"+jstr(occEggs[i])+
			","+jstr("occupies")+":"+jstr(occOcc[i])+"}")
	}
	occJSON := "[" + strings.Join(occJSONParts, ",") + "]"
	// ⑥ 派单模板骨架（目标 + 允许面 + 禁碰面 + 出口判据 + 时限 · **无占位符符号**）
	template := fmt.Sprintf(
		"派单骨架（可直接丢给子代理）\n"+
			"目标：清空桶 %s（面 %s）的全部未闭缺口（现读未闭 %d 条 · 逐条见①）\n"+
			"允许面：%s\n"+
			"禁碰面：%s\n"+
			"出口判据：该桶未闭归零（现读未闭数 = %d）\n"+
			"时限：到 %s（绝对日期 · 到点停手）\n"+
			"附加纪律：禁 browser 系工具 · 改动只在仓外副本 · 走正门（不得手搓）· 账只由 zerg gap 正门写",
		sel, face, len(open), allow, gapPlanForbid, len(open), deadline)

	// ⑤ 一页硬顶（只裁①清单 · 分页只人面）
	totalOpen := len(open)
	hits, cut := totalOpen, false
	if hits > top {
		hits, cut = top, true
	}
	shown := open
	if cut {
		shown = open[:hits]
	}
	gapPlanMetaAdd(inv, sel, face, top, len(bucket), totalOpen, hits, criterion, allow, occJSON, deadline, template, led.Path)
	if cut {
		inv.markTruncated()
		inv.warnf("已裁 %d 条（gap plan 一页 %d 条 / 真命中 %d 条）", totalOpen-hits, top, totalOpen)
		inv.metaAddJSON("truncated_detail", fmt.Sprintf(
			`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`, hits, totalOpen-hits, totalOpen))
	}

	if inv.jsonGiven {
		rows := []map[string]string{}
		for _, r := range shown {
			rows = append(rows, map[string]string{
				"id": r.ID, "prio": r.Prio, "state": r.State,
				"unit": r.Unit, "module": r.Module, "summary": r.Symptom,
				"selector": sel, "face": face, "bucket": strconv.Itoa(len(bucket)),
				"open": strconv.Itoa(totalOpen), "criterion": criterion, "allow": allow,
				"forbid": gapPlanForbid, "assigned": occJSON, "deadline": deadline, "template": template,
			})
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}

	// 人面（六件 · 逐件带件号）
	fmt.Fprintf(stdout, "作业单（gap plan · 选择器 %q · 面 %s）\n", sel, face)
	if cut {
		fmt.Fprintf(stdout, "  桶         : %d 条（未闭 %d 条 · 本页 %d 条 —— 已裁 %d 条，**这不是全集**）\n",
			len(bucket), totalOpen, hits, totalOpen-hits)
	} else {
		fmt.Fprintf(stdout, "  桶         : %d 条（未闭 %d 条）\n", len(bucket), totalOpen)
	}
	fmt.Fprintf(stdout, "  ① 该桶未闭 : %d 条 —— P0 %d · P1 %d · P2 %d（按 prio 排序 · 同 prio 按缺口号）\n",
		totalOpen, gapPlanCount(open, "P0"), gapPlanCount(open, "P1"), gapPlanCount(open, "P2"))
	for _, r := range shown {
		fmt.Fprintf(stdout, "      %s  %s  %s  %s  %s\n",
			r.ID, r.Prio, r.State, orDash(r.Unit), truncateDisplay(r.Symptom, 40))
	}
	if cut {
		fmt.Fprintf(stdout, "      ⚠ 本页只列前 %d 条 · 真命中 %d 条（已裁 %d 条）—— **这不是全集**，要收窄用 `--top`\n",
			hits, totalOpen, totalOpen-hits)
	}
	fmt.Fprintf(stdout, "  ② 出口判据 : %s\n", criterion)
	fmt.Fprintf(stdout, "  ③ 建议允许面: %s\n", allow)
	fmt.Fprintf(stdout, "  ④ 禁碰面   : %s\n", gapPlanForbid)
	fmt.Fprintf(stdout, "      理由    : %s\n", gapPlanForbidWhy)
	fmt.Fprintf(stdout, "  ⑤ 占用状态 : %s\n", occDetail)
	fmt.Fprintf(stdout, "  ⑥ 派单模板 :\n")
	for _, l := range strings.Split(template, "\n") {
		fmt.Fprintf(stdout, "      %s\n", l)
	}
	fmt.Fprintf(stdout, "  真源       : %s（**本面一字不碰** · 只读）\n", led.Path)
	return exitOK
}

// ── ⒢ `zerg gap receipt check|apply <回执件路径>`（回执**收件面** · 批3 第四片 · 2026-09-28）──────
//
// 设计出处（唯一真源 · `Zerg-内部文档/…/v2.5.13/设计-缺口账与自进化-v2.0-20260928.md`）：
//
//	§三十四 `O-12`（卵回执结构规范化：`closed_ids[]`（缺口 id）**必填**，取代散文）·
//	施工清单 `任务清单-缺口账自进化-施工-20260928.md` 3-4（回执结构化 `closed_ids[]` 必填 ·
//	回执缺栏即拒收 · 闭案与账一一对应）。
//
// 回执形状（本片**取定** · 设计稿只把 `closed_ids[]` 一名钉死，其余三名按最小惊讶取定、回执里点名）：
//
//	{
//	  "egg":         "<卵号>",                                    // 必填：卵号
//	  "readings":    [{"gap_id":"…","result":"…"}, …],            // 必填：逐条读数（每条含 缺口号 + 结果）
//	  "conclusion":  "<结论>",                                    // 必填：结论
//	  "closed_ids":  ["GAP-…", …]                                 // 必填**键**（可为空数组，但键必须在）
//	}
//
// 两条动作：
//
//	⒜ `zerg gap receipt check <回执件>`（**只读**）：按收件判据逐条验并出人面报告 + `--json`；
//	   验完只出结论，**绝不写账**（本面只读）。
//	⒝ `zerg gap receipt apply <回执件> --evidence <串> --yes`（D2 写面）：把 `closed_ids` 逐条改 `已解`，
//	   **走现有改态路径** `gapRewriteOne`（不另开写路）· 缺 `--yes` ⇒ 2 · 有冲突号 ⇒ 2 且**不动账** ·
//	   成败后回显逐条改前→改后计数。
//
// 收件判据（逐条照施工清单 3-4）：
//
//	① 必填栏：卵号 · 逐条读数（每条含 缺口号 + 结果）· 结论 · `closed_ids[]`；缺栏 ⇒ 2 并**点名缺哪栏**。
//	② `closed_ids[]` 必填（可为空数组但**键必须在**）。
//	③ 闭案与账**一一对应**：每个 `closed_ids` 里的号必须在账里且当前 `state=已派`（否则点名冲突 ⇒ 2）。
//	④ **反面**：账里 `已派` 但回执没提的 ⇒ 列为「**未结派单**」告警（**不阻断退码**，只报告）。
//
// 退码：0 验过（或 apply 落账）· 2 缺栏 / 冲突号 / 缺 `--yes` / 缺 `--evidence` / 用法错 · 8 回执件或真源读不到。

// 回执四个必填栏的**键名**（设计稿只钉死 `closed_ids`；另三名取定 · 见上）。
const (
	gapReceiptEggKey    = "egg"
	gapReceiptRowsKey   = "readings"
	gapReceiptConclKey  = "conclusion"
	gapReceiptClosedKey = "closed_ids"
)

// gapReceiptCheckFields / gapReceiptApplyFields —— `--json` 可取字段（与命令树里的 `fields` 同一份口径）。
var gapReceiptCheckFields = []string{"egg", "readings", "conclusion", "closed_ids",
	"missing", "conflicts", "unclosed", "ok"}
var gapReceiptApplyFields = []string{"egg", "closed_ids", "solved", "changed"}

// gapReceiptRow —— 回执里的一条**逐条读数**（每条含 缺口号 + 结果 —— 设计稿 `O-12` 的反散文落点）。
type gapReceiptRow struct {
	GapID  string `json:"gap_id"`
	Result string `json:"result"`
}

// gapReceipt —— 读回来的回执（四栏 + 键在场面）。
type gapReceipt struct {
	Egg        string
	Readings   []gapReceiptRow
	Conclusion string
	ClosedIDs  []string
}

// gapReceiptLoad —— 读回执件（JSON 对象）。返回 `(回执, 缺栏名单, err)`：
// `err != nil` ⇒ 件读不到 / 读不出（调用方退 8）；`缺栏名单` 非空 ⇒ 缺栏拒收（调用方退 2）。
// ★ 逐条读数每条**必须**同时有 `gap_id` 与 `result`（否则那一条算缺栏 —— 否则「读数」等于散文）。
func gapReceiptLoad(path string) (gapReceipt, []string, error) {
	r := gapReceipt{Readings: []gapReceiptRow{}, ClosedIDs: []string{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		return r, nil, err
	}
	obj := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return r, nil, fmt.Errorf("不是 JSON 对象（%v）", err)
	}
	miss := []string{}
	// ① 卵号
	if v, ok := obj[gapReceiptEggKey]; !ok {
		miss = append(miss, "卵号（"+gapReceiptEggKey+"）")
	} else if err := json.Unmarshal(v, &r.Egg); err != nil || strings.TrimSpace(r.Egg) == "" {
		miss = append(miss, "卵号（"+gapReceiptEggKey+"）")
	} else {
		r.Egg = strings.TrimSpace(r.Egg)
	}
	// ② 逐条读数（键必须在 **且** 至少一条 · 每条含缺口号 + 结果）
	if v, ok := obj[gapReceiptRowsKey]; !ok {
		miss = append(miss, "逐条读数（"+gapReceiptRowsKey+"）")
	} else {
		_ = json.Unmarshal(v, &r.Readings)
		if len(r.Readings) == 0 {
			miss = append(miss, "逐条读数（"+gapReceiptRowsKey+"）")
		}
		for i, row := range r.Readings {
			if strings.TrimSpace(row.GapID) == "" || strings.TrimSpace(row.Result) == "" {
				miss = append(miss, fmt.Sprintf("逐条读数第 %d 条（缺 %s 或 %s）", i+1, "gap_id", "result"))
			}
		}
	}
	// ③ 结论
	if v, ok := obj[gapReceiptConclKey]; !ok {
		miss = append(miss, "结论（"+gapReceiptConclKey+"）")
	} else if err := json.Unmarshal(v, &r.Conclusion); err != nil || strings.TrimSpace(r.Conclusion) == "" {
		miss = append(miss, "结论（"+gapReceiptConclKey+"）")
	} else {
		r.Conclusion = strings.TrimSpace(r.Conclusion)
	}
	// ④ 闭案号（键必须在 · 可为空数组）
	if v, ok := obj[gapReceiptClosedKey]; !ok {
		miss = append(miss, "闭案号（"+gapReceiptClosedKey+"）")
	} else if err := json.Unmarshal(v, &r.ClosedIDs); err != nil {
		miss = append(miss, "闭案号（"+gapReceiptClosedKey+"）")
	}
	if r.ClosedIDs == nil {
		r.ClosedIDs = []string{}
	}
	return r, miss, nil
}

// gapReceiptVerdict —— 收件对账的结论面（三层：缺栏 / 冲突号 / 未结派单）。
type gapReceiptVerdict struct {
	conflicts []string // 「<号>：<原因>」（每个 `closed_ids` 里的号不满足「在账且 state=已派」）
	unclosed  []string // 账里 `已派` 但回执没提的号（**告警 · 不阻断退码**）
}

// gapReceiptVerify —— 闭案与账**一一对应**的对账面：
//
//	③ 每个 `closed_ids` 里的号必须在账里且当前 `state=已派` ⇒ 不满足即入 `conflicts`（阻断 · 2）；
//	④ 账里 `已派` 但回执没提的 ⇒ 入 `unclosed`（告警 · 只报告）。
func gapReceiptVerify(rec gapReceipt, led gapLedger) gapReceiptVerdict {
	out := gapReceiptVerdict{conflicts: []string{}, unclosed: []string{}}
	inClosed := map[string]bool{}
	for _, id := range rec.ClosedIDs {
		inClosed[id] = true
		idx := gapFindIdx(led, id)
		if idx < 0 {
			out.conflicts = append(out.conflicts, id+"：账内没有这个号")
			continue
		}
		if st := led.Recs[idx].State; st != gapStAssigned {
			out.conflicts = append(out.conflicts, fmt.Sprintf("%s：当前 state=%s（非 已派）", id, st))
		}
	}
	for _, r := range led.Recs {
		if r.State == gapStAssigned && !inClosed[r.ID] {
			out.unclosed = append(out.unclosed, r.ID)
		}
	}
	return out
}

// gapReceiptArg —— 两条动作共用的**点名面**（恰好一条回执件路径 · 与同族 `gapTargetOne` 同一条纪律）。
func gapReceiptArg(inv *invocation, stderr io.Writer, cmd string) (string, int) {
	args := []string{}
	for _, a := range inv.args {
		if s := strings.TrimSpace(a); s != "" {
			args = append(args, s)
		}
	}
	if len(args) != 1 {
		inv.setErr("usage", "target_arity", "要点名恰好一条回执件路径")
		fmt.Fprintf(stderr, "%s: `gap receipt %s` 要给**恰好一条**回执件路径（收到 %d 条）\n", progName, cmd, len(args))
		fmt.Fprintf(stderr, "用法：zerg gap receipt %s <回执件路径>%s\n", cmd, gapReceiptUsageSuffix(cmd))
		return "", exitUsage
	}
	return args[0], exitOK
}

// gapReceiptUsageSuffix —— 两条动作各自的用法尾（check 只读 / apply 写面）。
func gapReceiptUsageSuffix(cmd string) string {
	if cmd == "apply" {
		return " --evidence <一句话> --yes [--by <谁>] [--json <字段>]"
	}
	return " [--json <字段>]"
}

// gapReceiptIDsJSON —— 一串 id 的机器面（恒数组 · 空为 `[]` · 永不为 null）。
func gapReceiptIDsJSON(ids []string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, jstr(id))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// gapReceiptIDsText —— 一串 id 的人面（空 ⇒ 明写「空数组」）。
func gapReceiptIDsText(ids []string) string {
	if len(ids) == 0 {
		return "（空数组）"
	}
	return strings.Join(ids, " · ")
}

// gapReceiptCheckRow —— `check` 的 `--json` 那一行（缺栏面与对账面同一份口径）。
func gapReceiptCheckRow(rec gapReceipt, miss, conflicts, unclosed []string) map[string]string {
	return map[string]string{
		"egg":        rec.Egg,
		"readings":   strconv.Itoa(len(rec.Readings)),
		"conclusion": rec.Conclusion,
		"closed_ids": gapReceiptIDsJSON(rec.ClosedIDs),
		"missing":    strings.Join(miss, " · "),
		"conflicts":  strings.Join(conflicts, " · "),
		"unclosed":   gapReceiptIDsJSON(unclosed),
		"ok":         boolWord(len(miss) == 0 && len(conflicts) == 0),
	}
}

// cmdGapReceiptCheck —— `zerg gap receipt check <回执件>`：收件体检（**只读** · 一字不写真源、不写审计）。
func cmdGapReceiptCheck(inv *invocation, stdout, stderr io.Writer) int {
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapReceiptCheckFields, ","))
		return exitUsage
	}
	path, rc := gapReceiptArg(inv, stderr, "check")
	if rc != exitOK {
		return rc
	}
	rec, miss, err := gapReceiptLoad(path)
	if err != nil {
		inv.setErr("blocked", "receipt_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 回执件读不到 / 读不出：%v（退码 8 —— 「读不到」不许当绿）\n", progName, err)
		fmt.Fprintf(stderr, "  回执件 : %s\n", path)
		return exitBlocked
	}
	inv.changed = boolPtr(false)
	// ① 缺栏 ⇒ 2（判在**读账之前** —— 缺栏即拒收，一个字节都不读）
	if len(miss) > 0 {
		inv.setErr("usage", "receipt_missing_field", "回执缺栏")
		fmt.Fprintf(stderr, "%s: 回执**缺栏拒收** —— 缺 %d 栏：%s\n", progName, len(miss), strings.Join(miss, " · "))
		fmt.Fprintf(stderr, "  必填栏（设计稿 §三十四 `O-12`）：卵号 `%s` · 逐条读数 `%s[]`（每条含 `gap_id`+`result`）· 结论 `%s` · 闭案号 `%s[]`（**可为空数组，但键必须在**）\n",
			gapReceiptEggKey, gapReceiptRowsKey, gapReceiptConclKey, gapReceiptClosedKey)
		fmt.Fprintf(stderr, "  回执件 : %s\n", path)
		if inv.jsonGiven {
			selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapReceiptCheckRow(rec, miss, nil, nil))
		}
		return exitUsage
	}
	// ② 读真源（读不到 ⇒ 8）
	led, lrc := gapReadLedgerOrDie(inv, stderr)
	if lrc != exitOK {
		return lrc
	}
	v := gapReceiptVerify(rec, led)
	// 机器面（`--json` ⇒ **只出包封** · 与人面分面 —— 同族 `gap ls` / `gap status` / `gap plan` 的口径）
	if inv.jsonGiven {
		selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapReceiptCheckRow(rec, nil, v.conflicts, v.unclosed))
		if len(v.conflicts) > 0 {
			fmt.Fprintf(stderr, "%s: 闭案与账对不上 —— %d 个号冲突（退码 2）\n", progName, len(v.conflicts))
			return exitUsage
		}
		return exitOK
	}
	// 人面报告（三段：读数 / 对账 / 未结派单告警）
	fmt.Fprintf(stdout, "回执收件体检（gap receipt check · 只读 —— 一字不写真源、不写审计）\n")
	fmt.Fprintf(stdout, "  回执件   : %s\n", path)
	fmt.Fprintf(stdout, "  卵号     : %s\n", rec.Egg)
	fmt.Fprintf(stdout, "  逐条读数 : %d 条\n", len(rec.Readings))
	for i, rr := range rec.Readings {
		fmt.Fprintf(stdout, "      %d) %s — %s\n", i+1, rr.GapID, rr.Result)
	}
	fmt.Fprintf(stdout, "  结论     : %s\n", rec.Conclusion)
	fmt.Fprintf(stdout, "  闭案号   : %d 条 — %s\n", len(rec.ClosedIDs), gapReceiptIDsText(rec.ClosedIDs))
	if len(v.conflicts) > 0 {
		fmt.Fprintf(stdout, "  闭案对账 : ✗ %d 个号与账对不上 —— %s\n", len(v.conflicts), strings.Join(v.conflicts, " · "))
	} else {
		fmt.Fprintf(stdout, "  闭案对账 : ✓ %d 个号都在账里且当前 state=已派（与账一一对应）\n", len(rec.ClosedIDs))
	}
	if len(v.unclosed) > 0 {
		fmt.Fprintf(stdout, "  ⚠ 未结派单 : %d 条（账里 `已派` 但回执没提）—— %s\n", len(v.unclosed), strings.Join(v.unclosed, " · "))
		fmt.Fprintf(stdout, "      （**告警不阻断退码** · 只报告 —— 施工清单 3-4 的「反面」）\n")
	} else {
		fmt.Fprintf(stdout, "  未结派单 : 0 条（账里 `已派` 都已在回执里点名）\n")
	}
	fmt.Fprintf(stdout, "  真源     : %s（**本面一字不碰** · 只读）\n", led.Path)
	if len(v.conflicts) > 0 {
		fmt.Fprintf(stderr, "%s: 闭案与账对不上 —— %d 个号冲突（退码 2）\n", progName, len(v.conflicts))
		return exitUsage
	}
	return exitOK
}

// cmdGapReceiptApply —— `zerg gap receipt apply <回执件> --evidence <串> --yes`：
// 把 `closed_ids` 逐条改 `已解`（**走现有改态路径** `gapRewriteOne` · 不另开写路）。
func cmdGapReceiptApply(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapReceiptApplyFields, ","))
		return exitUsage
	}
	evidence := strings.TrimSpace(inv.flagVal("--evidence"))
	path, rc := gapReceiptArg(inv, stderr, "apply")
	if rc != exitOK {
		return rc
	}
	if evidence == "" {
		inv.setErr("usage", "missing_required", "缺 --evidence")
		fmt.Fprintf(stderr, "%s: `gap receipt apply` 缺必填旗标：--evidence（**改态必留一句证据** —— 与 `gap set-state` 同一条纪律）\n", progName)
		return exitUsage
	}
	// 缺 `--yes`（fail-closed 2 · 计划件走 stderr · 判在读盘之前）
	if !inv.yes {
		fmt.Fprintf(stderr, "计划件（回执闭案落账 · 缺 `--yes`（D2 档）· 零副作用 —— 未改真源、未写审计）\n")
		fmt.Fprintf(stderr, "  真源     : %s（未读 —— 缺 `--yes` ⇒ 不执行）\n", gapLedgerPath())
		fmt.Fprintf(stderr, "  回执件   : %s\n", path)
		fmt.Fprintf(stderr, "  证据     : %s\n", evidence)
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}
	rec, miss, err := gapReceiptLoad(path)
	if err != nil {
		inv.setErr("blocked", "receipt_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 回执件读不到 / 读不出：%v（退码 8）\n", progName, err)
		return exitBlocked
	}
	// 缺栏 ⇒ 2 且**不动账**（一个字节都不写）
	if len(miss) > 0 {
		inv.setErr("usage", "receipt_missing_field", "回执缺栏")
		fmt.Fprintf(stderr, "%s: 回执**缺栏拒收** —— 缺 %d 栏：%s（账不动 · 退码 2）\n", progName, len(miss), strings.Join(miss, " · "))
		return exitUsage
	}
	led, lrc := gapReadLedgerOrDie(inv, stderr)
	if lrc != exitOK {
		return lrc
	}
	// 有冲突号 ⇒ 2 且**不动账**
	v := gapReceiptVerify(rec, led)
	if len(v.conflicts) > 0 {
		inv.setErr("usage", "closed_conflict", "闭案号与账对不上")
		fmt.Fprintf(stderr, "%s: 闭案与账对不上 —— %d 个号冲突（**账不动** · 退码 2）：\n", progName, len(v.conflicts))
		for _, c := range v.conflicts {
			fmt.Fprintf(stderr, "  ✗ %s\n", c)
		}
		return exitUsage
	}
	// 真写：逐条 `已派` → `已解`（**走现有改态路径** `gapRewriteOne` · 不另开写路）
	solved := 0
	lines := []string{}
	seen := map[string]bool{}
	for _, id := range rec.ClosedIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		idx := gapFindIdx(led, id)
		r := led.Recs[idx]
		before := r.State
		r.State = gapStSolved
		r.SolvedAt = gapNow()
		r.Evidence = evidence
		detail, wrc, msg := gapRewriteOne(inv, led, idx, r, "receipt", before, gapStSolved)
		if wrc != exitOK {
			gapWriteFail(inv, stderr, detail, msg, "receipt")
			return wrc
		}
		solved++
		lines = append(lines, fmt.Sprintf("%s  %s → %s", id, before, gapStSolved))
		// 逐条落账后刷新内存账（后续条基于最新 Lines/Raw · 审计的 before sha 才逐次对得上）
		if l2, e2 := readGapLedger(); e2 == nil {
			led = l2
		}
	}
	inv.changed = boolPtr(solved > 0)
	if inv.jsonGiven {
		selectJSON(stdout, stderr, inv, inv.path, inv.fields, map[string]string{
			"egg":        rec.Egg,
			"closed_ids": gapReceiptIDsJSON(rec.ClosedIDs),
			"solved":     strconv.Itoa(solved),
			"changed":    boolWord(solved > 0),
		})
		return exitOK
	}
	fmt.Fprintf(stdout, "已结案（gap receipt apply · 卵号 %s）· 逐条改态（**走现有改态路径** `gapRewriteOne`）\n", rec.Egg)
	for _, l := range lines {
		fmt.Fprintf(stdout, "  %s\n", l)
	}
	fmt.Fprintf(stdout, "  计数     : 已派 → 已解 %d 条（真源 %s · 只重写那 %d 行 · 审计已落 %d 行）\n",
		solved, led.Path, solved, solved)
	fmt.Fprintf(stdout, "  证据     : %s（逐条落进各条的 `solved_evidence`）\n", evidence)
	fmt.Fprintf(stdout, "  出口判据 : `closed_ids` 全落 `已解`（现读 %d / %d）\n", solved, len(rec.ClosedIDs))
	return exitOK
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// 批4 第三片：`gap regress <件…>` —— 分层回归巡检（**只读面** · 2026-09-28）
//
// 设计出处（逐字）：`设计-缺口账与自进化-v2.0-20260928.md` §11.4 批4 D3「回归巡检」+
// §附录「巡检条款措辞限缩（35.2 源四）」。
//
// 病（§11.2 批4 · §11.4）：`gap verify-one` 一条一次 —— 改一件之后无法快速知道「我碰过的那件
// 名下、账里那些条的判据还站不站得住」；900+ 条全跑贵且慢（`M-34`）。
// 治（§11.4 分层巡检算法逐字）：① 取本次改动碰过的件 ② 在账里找 `unit` 命中的条 ③ 逐条跑
// `verify_cmd` ④ rc≠0 ⇒ 报出来 ⑤ **报告面不改态**（本面独立只读档 · 不写 `state`）。
//
// 六条口径（逐条可对拍）：
//
//	① **只读面**：一字不写真源、不写审计、不改任何 `state`（与 `gap ls`/`status`/`plan` 同档）。
//	② **件面** = 位置参数（一个或多个仓内相对路径）；一条账按 `unit` **精确等值**该件命中
//	   （**不含**模块前缀 —— 那是 `gap ls --unit` 的第二形态，本面只取「unit 就是它」这一态）。
//	   件不在仓里 ⇒ 2 并**逐条点名**（fail-closed：任一件缺 ⇒ 一条判据都不跑）。
//	③ **跑法逐字照 `verify-one` 的安全边界**：只跑账里 `verify_cmd` 那一格命令本身（进程内走同一
//	   `run` 入口 · **不拼 shell 串**）；含 **shell 元字符**（管道 `|` / 重定向 `>` `<` / 分号 `;` /
//	   与或 `&` / 反引号 / `$` / 括号）⇒ **拒跑并点名**（不当绿也不当红）；写面（危险档且没带
//	   `--dry-run`）/ 无判据 / 占位 ⇒ 拒跑（**不执行**）。
//	④ **汇总六数**：件 → 条数 · 实跑数 · 通过数 · 失败数 · 拒跑数 · 未跑数（被 `--limit` 截掉的）。
//	⑤ **声称纪律（本片核心）**：表尾**逐字**声明只覆盖 modification-traversing 级；某件零条 ⇒
//	   明写「零条（不是通过）」。
//	⑥ `--dry-run` 只列出将要跑的命令（**不执行**）。
//
// 退码：0 全过（含零条 / 全是拒跑）· 1 有条判红（`verify_cmd` rc≠0）· 2 用法错（缺件 / 件不在仓 /
// `--limit` 非非负整数 / `--json` 不给字段）· 8 真源读不到 / 仓根取不到。★ **拒跑不当红**。

// gapRegressDefaultLimit —— `--limit` 缺省一页条数（与 `gap status`/`gap plan` 的 20 同值）。
const gapRegressDefaultLimit = 20

// gapRegressClaim —— 表尾**逐字**声称纪律行（设计稿 §附录「措辞限缩」原文口径 · **不许改写**）。
const gapRegressClaim = "本次只覆盖 modification-traversing 级（只跑了与被改件直接相关的账）；" +
	"这不等于 safe，也不覆盖非确定性/环境依赖场景"

// gapRegressFields —— `--json` 可取字段（每行 = 一件的汇总）。
var gapRegressFields = []string{"file", "in_repo", "total", "ran", "passed", "failed", "refused", "skipped", "zero", "states"}

// gapRegressRow —— 一件的汇总面（六数 + 状态分栏 + 逐条点名行）。
type gapRegressRow struct {
	File    string
	Total   int
	Ran     int
	Passed  int
	Failed  int
	Refused int
	Skipped int
	States  []int // 按 gapStateClosed 序的计数（仍缺/已派/已立项/已解/回归/不做）
	Lines   []string
}

// gapRegressRefuse —— 分类一条判据「拒跑」的因（空串 ⇒ 可跑）。★ 只读面：本函数不执行任何东西。
func gapRegressRefuse(r gapRecord) string {
	if strings.TrimSpace(r.VerifyCmd) == "" {
		return "无 verify_cmd（空串 ⇒ 判据分级=无 · 空判据不算「验过」）"
	}
	if t, _ := gapStatusTierOf(r); t == gapVerifyTierPlaceholder {
		return "占位判据（不是可跑命令 ⇒ 不执行）"
	}
	if m := gapVerifyOneMetaHit(r.VerifyCmd); m != "" {
		return fmt.Sprintf("含 shell 元字符 %q（只跑「那一格命令本身」· 不拼 shell 串）", m)
	}
	if gapWriteFaceWhy(r.VerifyCmd) != "" {
		return "写面判据（危险档且没带 `--dry-run` ⇒ 真跑会真写盘）"
	}
	return ""
}

// gapRegressQueryText —— `meta.query` 的回显（现读入参 · 序 = files / limit / dry_run）。
func gapRegressQueryText(files []string, limit int, dry bool) string {
	q := make([]string, 0, len(files))
	for _, f := range files {
		q = append(q, jstr(f))
	}
	return "{" + jstr("files") + ":[" + strings.Join(q, ",") + "]," +
		jstr("limit") + ":" + strconv.Itoa(limit) + "," + jstr("dry_run") + ":" + strconv.FormatBool(dry) + "}"
}

// gapRegressStatesText —— 状态分栏的打印面（按 `gapStateClosed` 六值序 · 与账内闭集同源）。
func gapRegressStatesText(states []int) string {
	parts := make([]string, 0, len(gapStateClosed))
	for i, s := range gapStateClosed {
		parts = append(parts, fmt.Sprintf("%s %d", s, states[i]))
	}
	return strings.Join(parts, " · ")
}

// gapRegressStatesJSON —— 状态分栏的机器面（同一份数据 · 闭集为键）。
func gapRegressStatesJSON(states []int) string {
	parts := make([]string, 0, len(gapStateClosed))
	for i, s := range gapStateClosed {
		parts = append(parts, jstr(s)+":"+strconv.Itoa(states[i]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// cmdGapRegress —— `zerg gap regress <件…>`：只跑被点名件名下账的判据（分层巡检 · 只读）。
func cmdGapRegress(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapRegressFields, ","))
		return exitUsage
	}
	if len(inv.flagVals("--state")) > 0 || inv.hasFlag("--prio") || inv.hasFlag("--impact") {
		inv.setErr("usage", "narrow_not_in_shape", "本面不收收窄旗标")
		fmt.Fprintf(stderr, "%s: `gap regress` 只认件（位置参数）+ `--limit` / `--dry-run` / `--json` —— 不收 `--state/--prio/--impact`\n", progName)
		return exitUsage
	}
	files := []string{}
	for _, a := range inv.args {
		if s := strings.TrimSpace(a); s != "" {
			files = append(files, s)
		}
	}
	if len(files) == 0 {
		inv.setErr("usage", "target_required", "缺件")
		fmt.Fprintf(stderr, "%s: 要给件：`zerg gap regress <件路径>…`（仓内相对路径 · 一个或多个）\n", progName)
		fmt.Fprintf(stderr, "  例   : `zerg gap regress core/cmd/zerg/family_gap.go`\n")
		return exitUsage
	}
	limit := gapRegressDefaultLimit
	if v := strings.TrimSpace(inv.flagVal("--limit")); v != "" {
		n, lerr := strconv.Atoi(v)
		if lerr != nil || n < 0 {
			inv.setErr("usage", "bad_limit", "--limit 要给非负整数")
			fmt.Fprintf(stderr, "%s: `--limit %s` 不是非负整数（本族口径 = 用法错 2）\n", progName, v)
			return exitUsage
		}
		limit = n
	}

	// ② 读真源（读不到 ⇒ 8 · 不许当绿）· 两种因**机器可辨**（`Q-138`）
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当绿（退码 8）\n", gapLedgerPath())
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "真源不在盘上", led.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		gapLedgerAbsentHint(stderr, led.Path)
		return exitBlocked
	}

	// ③ 件存在性（在任何跑判据之前 · fail-closed：任一件不在仓里 ⇒ 2 并点名、一条判据都不跑）
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", gapReasonPrecondition, "仓根解析不到，判不了件在不在仓里")
		fmt.Fprintf(stderr, "%s: 仓根解析不到 ⇒ 判不了件在不在仓里（**没读到** 不是 没有 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "  修法：进仓根再跑；或显式给 `ZERG_REPO`\n")
		return exitBlocked
	}
	missing := []string{}
	for _, f := range files {
		st, serr := os.Stat(filepath.Join(root, f))
		if serr != nil || st.IsDir() {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		inv.setErr("usage", "file_not_in_repo", "点名件不在仓里")
		fmt.Fprintf(stderr, "%s: 仓里没有这几个件（仓根 %s 下找不到）：\n", progName, root)
		for _, f := range missing {
			fmt.Fprintf(stderr, "  ✗ %s\n", f)
		}
		fmt.Fprintf(stderr, "  ⇒ 「没读到」不是「没有」：退码 2（点名面名字给错 = 用法错，与 `gap show <件路径>` 同一口径）\n")
		fmt.Fprintf(stderr, "  注 : 「件在仓里、但账内 0 条」是**另一态**（退 0 且明写「零条（不是通过）」）—— 两态不许混\n")
		return exitUsage
	}

	// ④ 逐件：取 unit 精确等值的条（不分状态 · 分栏显示）→ 跑前 `limit` 条 → 汇总六数
	rows := make([]*gapRegressRow, 0, len(files))
	anyFail := false
	matchTotal := 0
	for _, f := range files {
		row := &gapRegressRow{File: f, States: make([]int, len(gapStateClosed))}
		recs := []gapRecord{}
		for _, r := range led.Recs {
			if r.Unit == f {
				recs = append(recs, r)
			}
		}
		row.Total = len(recs)
		matchTotal += row.Total
		for _, r := range recs {
			for i, s := range gapStateClosed {
				if r.State == s {
					row.States[i]++
					break
				}
			}
		}
		run := recs
		if len(run) > limit {
			run = run[:limit]
			row.Skipped = row.Total - limit
		}
		for _, r := range run {
			if why := gapRegressRefuse(r); why != "" {
				row.Refused++
				row.Lines = append(row.Lines, fmt.Sprintf("⛔ %s · 拒跑：%s · 判据=%s", r.ID, why, gapShowText(r.VerifyCmd)))
				continue
			}
			if inv.dryRun {
				row.Lines = append(row.Lines, fmt.Sprintf("·  %s · 将跑：%s", r.ID, gapShowText(r.VerifyCmd)))
				continue
			}
			c, out, jwhy := gapVerifyOneRun(r.VerifyCmd)
			if jwhy != "" {
				row.Refused++
				row.Lines = append(row.Lines, fmt.Sprintf("⛔ %s · 拒跑（判据不可跑）：%s · 判据=%s", r.ID, jwhy, gapShowText(r.VerifyCmd)))
				continue
			}
			row.Ran++
			if c == 0 {
				row.Passed++
				row.Lines = append(row.Lines, fmt.Sprintf("✓ %s · %s", r.ID, gapVerifyOneReading(c, out)))
			} else {
				row.Failed++
				anyFail = true
				row.Lines = append(row.Lines, fmt.Sprintf("✗ %s · %s", r.ID, gapVerifyOneReading(c, out)))
			}
		}
		rows = append(rows, row)
	}

	// ⑤ 机器面信封（只走 `metaAdd*` 既有口子 ⇒ 顶层六键不动）
	inv.changed = boolPtr(false)
	inv.metaAddJSON("total", strconv.Itoa(matchTotal))
	inv.metaAddJSON("files", strconv.Itoa(len(files)))
	inv.metaAddJSON("limit", strconv.Itoa(limit))
	inv.metaAddStr("query", gapRegressQueryText(files, limit, inv.dryRun))
	inv.metaAddStr("query_ts", gapNow())
	inv.metaAddStr("ledger_sha16", gapLsLedgerSHA16(led.Path))
	inv.metaAddStr("claim", gapRegressClaim)
	inv.metaAddStr("tier", "modification-traversing")

	if inv.jsonGiven {
		js := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			js = append(js, map[string]string{
				"file":    row.File,
				"in_repo": "true",
				"total":   strconv.Itoa(row.Total),
				"ran":     strconv.Itoa(row.Ran),
				"passed":  strconv.Itoa(row.Passed),
				"failed":  strconv.Itoa(row.Failed),
				"refused": strconv.Itoa(row.Refused),
				"skipped": strconv.Itoa(row.Skipped),
				"zero":    boolWord(row.Total == 0),
				"states":  gapRegressStatesJSON(row.States),
			})
		}
		if rc := selectJSONList(stdout, stderr, inv, inv.path, gapRegressFields, js); rc != exitOK {
			return rc
		}
		if anyFail {
			return exitFail
		}
		return exitOK
	}

	// ⑥ 人面（表尾逐字声称纪律 —— 本片核心判据）
	mode := "真跑"
	if inv.dryRun {
		mode = "干跑（--dry-run · 只列命令、**不执行**）"
	}
	fmt.Fprintf(stdout, "缺口账回归巡检（只读面 · 分层 · 只跑被点名件名下的账）\n")
	fmt.Fprintf(stdout, "  真源     : %s（现有 %d 行 · sha256 前16 = %s）\n",
		led.Path, len(led.Lines), gapLsLedgerSHA16(led.Path))
	fmt.Fprintf(stdout, "  件面     : %d 个 · 账内 `unit` **精确等值**命中 %d 条（**不**含模块前缀面）\n", len(files), matchTotal)
	fmt.Fprintf(stdout, "  档       : %s · `--limit %d` 一页硬顶（被截掉的记「未跑」）\n", mode, limit)
	fmt.Fprintf(stdout, "  ★ 安全边界：只跑账里那一格 `verify_cmd` **本身** —— 含 shell 元字符 / 写面 ⇒ 拒跑并点名（不当红）\n")
	for _, row := range rows {
		fmt.Fprintf(stdout, "件: %s（在仓里）\n", row.File)
		if row.Total == 0 {
			fmt.Fprintf(stdout, "  零条（不是通过）—— 该件名下账内 `unit` 精确等值命中 0 条。\n")
			continue
		}
		fmt.Fprintf(stdout, "  条数 %d · 实跑 %d · 通过 %d · 失败 %d · 拒跑 %d · 未跑 %d\n",
			row.Total, row.Ran, row.Passed, row.Failed, row.Refused, row.Skipped)
		fmt.Fprintf(stdout, "  状态分栏：%s\n", gapRegressStatesText(row.States))
		for _, l := range row.Lines {
			fmt.Fprintf(stdout, "    %s\n", l)
		}
		if row.Skipped > 0 {
			fmt.Fprintf(stdout, "    （另有 %d 条被 `--limit %d` 截掉 ⇒ 记「未跑」· 这不是「通过」）\n", row.Skipped, limit)
		}
	}
	fmt.Fprintf(stdout, "表尾声称：%s\n", gapRegressClaim)
	if anyFail {
		return exitFail
	}
	return exitOK
}
