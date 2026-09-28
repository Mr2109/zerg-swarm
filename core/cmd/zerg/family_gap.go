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
	SolvedAt  string   `json:"solved_at,omitempty"`
	Evidence  string   `json:"solved_evidence,omitempty"`
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

func cmdGapAdd(inv *invocation, stdout, stderr io.Writer) int {
	symptom := strings.TrimSpace(inv.flagVal("--symptom"))
	handmade := strings.TrimSpace(inv.flagVal("--handmade"))
	impact := strings.TrimSpace(inv.flagVal("--impact"))
	wantFam := strings.TrimSpace(inv.flagVal("--want-family"))
	wantAct := strings.TrimSpace(inv.flagVal("--want-action"))
	wantArgv := inv.flagVals("--want-argv")
	prio := strings.TrimSpace(inv.flagVal("--prio"))
	reproCmd := strings.TrimSpace(inv.flagVal("--repro-cmd"))
	verifyCmd := strings.TrimSpace(inv.flagVal("--verify-cmd"))
	dependsOn := inv.dependsOn // `--depends-on` 是具名旗标（收进 invocation.dependsOn，不是 kv）
	stateWant := strings.TrimSpace(inv.flagVal("--state"))

	// ① 用法面（在任何盘面动作之前 —— §4.1 K14 四件套那一条口径）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2 · 设计稿 §二.2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapAddFields, ","))
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

// gapStatusBy 两值（`--by` 的闭集）—— 与件面三键的 `unit` / `module` **同名同义**（不自造第二个词）。
const (
	gapStatusByUnit   = "unit"
	gapStatusByModule = "module"
)

// gapStatusFields —— `gap status` 的 `--json` 可取字段（与命令树里的 `fields` 同一份口径）：
// 桶键 + 三计数（未闭 / 已解 / 净）+ 近邻量化三格（P0/P1/P2）。
var gapStatusFields = []string{"bucket", "open", "closed", "net", "p0", "p1", "p2"}

// gapStatusBucket —— 一个桶的计数面。
type gapStatusBucket struct {
	Key    string // 桶键：`--by unit` ⇒ `unit`；`--by module` ⇒ `module`
	Open   int    // 未闭：`state` 仍缺
	Closed int    // 已解：`state` 已解
	P0     int    // 近邻量化：P0 条数
	P1     int    // 近邻量化：P1 条数
	P2     int    // 近邻量化：P2 条数
	// None —— 桶内条目**全部** `module == 无件`（兜底件）⇒ 对账等式里的「无件桶」。
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

// gapStatusMetaAdd —— `gap status` 机器面信封字段的**唯一写入口**（只走 `metaAdd*` 既有口子 ⇒
// 顶层六键不动）。五格与 `gap ls` **同形**（`total` / `hits` / `query` / `query_ts` / `ledger_sha16`），
// 另加本面自己的三格（`by` / `top` / `reconcile`）—— 缺一格两个时刻的排行就不可比。
func gapStatusMetaAdd(inv *invocation, states []string, prio, impact, by string, top, total, hits int, rec gapStatusReconcile, ledgerPath string) {
	inv.metaAddJSON("total", strconv.Itoa(total))
	inv.metaAddJSON("hits", strconv.Itoa(hits))
	inv.metaAddJSON("query", gapStatusQueryText(states, prio, impact, by, top))
	inv.metaAddStr("query_ts", gapNow())
	inv.metaAddStr("ledger_sha16", gapLsLedgerSHA16(ledgerPath))
	inv.metaAddStr("by", by)
	inv.metaAddJSON("top", strconv.Itoa(top))
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
	if by != gapStatusByUnit && by != gapStatusByModule {
		inv.setErr("usage", "bad_by", "--by 取值不在闭集里")
		fmt.Fprintf(stderr, "%s: `--by %s` 不在闭集里 —— 只认 %s | %s\n",
			progName, by, gapStatusByUnit, gapStatusByModule)
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

	// ④ 筛选（AND）+ 分桶 —— 一次过。判据序与 `gap ls` 同：state / prio / impact。
	wantStates := inv.flagVals("--state")
	wantPrio := strings.TrimSpace(inv.flagVal("--prio"))
	wantImpact := strings.TrimSpace(inv.flagVal("--impact"))
	buckets := map[string]*gapStatusBucket{}
	order := []string{}
	popTotal, popOpen, noneOpen := 0, 0, 0
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
		if by == gapStatusByModule {
			key = r.Module
		}
		if key == "" {
			key = gapUnitNoneMod // 无件兜底桶（对账等式里点名的那一枚）
		}
		b, ok := buckets[key]
		if !ok {
			b = &gapStatusBucket{Key: key, None: true}
			buckets[key] = b
			order = append(order, key)
		}
		if r.Module != gapUnitNoneMod {
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
	bs := make([]gapStatusBucket, 0, len(order))
	for _, k := range order {
		bs = append(bs, *buckets[k])
	}
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
	realOpen := 0
	for _, b := range bs {
		if !b.None {
			realOpen += b.Open
		}
	}
	rec := gapStatusReconcile{RealOpen: realOpen, NoneOpen: noneOpen, LedgerOpen: popOpen}
	total := len(bs)
	hits, cut := total, false
	if total > top {
		hits, cut = top, true
	}

	// 零命中（筛选后一个桶都没有）⇒ 1（判词与 `gap ls` 同款）。
	if total == 0 {
		inv.changed = boolPtr(false)
		inv.setErr("failed", "no_match", "零命中")
		fmt.Fprintf(stderr, "零命中：账内 %d 条 · 与筛选条件相符 0 条（退码 1 —— 「没有」不是「失败」，也不是绿）\n", len(led.Recs))
		if inv.jsonGiven {
			gapStatusMetaAdd(inv, wantStates, wantPrio, wantImpact, by, top, 0, 0, rec, led.Path)
			emitEnvelopeWith(stdout, find(inv.path), "[]", 0, inv)
		}
		return exitFail
	}

	inv.changed = boolPtr(false)
	gapStatusMetaAdd(inv, wantStates, wantPrio, wantImpact, by, top, total, hits, rec, led.Path)
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
	if !rec.OK() {
		inv.warnf("对账不成立：Σ(有件桶未闭) %d + 无件桶 %d = %d ≠ 账内仍缺 %d（差 %+d）",
			rec.RealOpen, rec.NoneOpen, rec.Left(), rec.LedgerOpen, rec.Diff())
	}

	// ⑥ 机器面。
	if inv.jsonGiven {
		rows := make([]map[string]string, 0, hits)
		for _, b := range bs[:hits] {
			rows = append(rows, map[string]string{
				"bucket": b.Key,
				"open":   strconv.Itoa(b.Open),
				"closed": strconv.Itoa(b.Closed),
				"net":    strconv.Itoa(b.Net()),
				"p0":     strconv.Itoa(b.P0),
				"p1":     strconv.Itoa(b.P1),
				"p2":     strconv.Itoa(b.P2),
			})
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}

	// ⑦ 人面（主输出走 stdout · 消息与错误走 stderr）：表头**两个排序键都写**。
	colName := "件(unit)"
	if by == gapStatusByModule {
		colName = "模块(module)"
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
	fmt.Fprintf(stdout, "对账：Σ(有件桶未闭) %d + 无件桶 %d = %d == 账内仍缺 %d %s（差 %d）\n",
		rec.RealOpen, rec.NoneOpen, rec.Left(), rec.LedgerOpen, mark, rec.Diff())
	if !rec.OK() {
		fmt.Fprintf(stdout, "  ⚠ 对账不成立：Σ(有件桶未闭) + 无件桶 与 账内仍缺 对不上 —— 差数 %+d（分桶漏/重算 · 别据此下结论）\n", rec.Diff())
	}
	return exitOK
}
