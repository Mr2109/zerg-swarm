// family_approve.go —— **人批通道**（`zerg approve ls|show|new|keygen` · M18 `C4`② · D3b 第四步）。
//
// 病征（D3b 逐字）：`require_approval` 今天还是「**无审批即不执行**」✓，而**人签批准件的真通道没有命令**
// —— 批准件（`<状态目录>/approvals/<工具名>.json`）只能人手写；而**模型手里有写文件的工具** ⇒ 理论上
// 够得着那个目录 ⇒ 「模型自签自批」。这是整条线**安全上最要紧的一块**（提者 ≠ 批者那一层的落点）。
//
// 本件的落法（两道，缺一不可）：
//
//	① **人签**：`keygen` 由人在**终端**上敲口令（回显关掉）生成 Ed25519 密钥对 ——
//	   私钥**以口令派生的密钥加密**落盘（PBKDF2-HMAC-SHA256 + AES-256-GCM），公钥落 `operator.pub`。
//	   `new` 每次都要人**再敲一次口令**才解得开私钥（口令错 ⇒ 不给签）。
//	   ⇒ **模型写得出件的字节，但写不出那枚签名**：口令只在人脑子里；
//	② **消费者只认签名**（`core/internal/control/approval.go` 的 `VerifyApprovalSignature`）：
//	   无签名 / 签名坏 / 公钥读不到 ⇒ **不算批准**（fail-closed）。
//
// 还有一道**物理闸**（同批落在控制层 `rules.yaml`）：`write`/`edit`/`apply_patch`/`bash`/`terminal` 的
// `args_deny` 里加 `state/approvals` 一族的模式 ⇒ 模型手上那几件工具**够不着**这个目录。
//
// 本版照实说的残留（不许含糊 ✗）：**有 TTY + 知道口令**的一方仍然签得出件 —— 这条线靠「口令不在模型手里」
// 成立，不靠「模型不会敲命令」。它是**人不在场时默认保守档**（`SD7`）的前提，不是它的替代品。
//
// ★ E 批（`2.5.11` · 任务单-人签实施-20260922 §三 + §附十 两拍）：本件是 D / E 两批的**共同落点**，
// 本次落的是三件：
//
//	① `D1` **档列落地**（`P26` = 甲）：`ls` 加一列「档」· `show` 加一行「档」· `--json` 加**可选**字段
//	   `strength` / `key_id_in_use` —— 人面**显式打** `strength=passphrase`（不靠 `sig_alg` 反推）；
//	② `E1` **审计新事件**（`I-6`）：人签那一次落一行**自己的字段名**的审计（不蹭 `before_sha256` /
//	   `after_sha256` —— 那两格记的是**被改文件的前后 hash**，与「签名前内容 `sha256`」不是同一件事）；
//	③ `E2` 的**明标**（`L3-b` = **B 案**）与 `E3` 的**归档机制**（`I-8`）：明标落**审计侧**（件字段面
//	   一字不动 ✗）；归档 = 先复制 + `sha256` 逐字核对 + 才删原件。
//
// ★ `E4`（`--ttl`）/ `E5`（换钥确认对）**未开工**，两条都卡在「先停下问」的那一格（缘由见回执）：
// 前者要往批准件里加一格（撞 §4.2「八字段一字不加 / 不减」），后者要占 `--rekey` 这个名（`P27`
// 是**升级接口**的预留旗标名 · §五 ⑩ 不许占名）。**没开工的事不许说成做了** ✗。
package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

// approveFields —— `ls` / `show` 的机器面字段。
var approveFields = []string{"tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "state", "path"}

// approveNewFields —— `new` 的机器面字段（结果面）。
var approveNewFields = []string{"tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "result", "path"}

// ---- 档位面（`D1` 落地 · `P26` = **甲 = 显式打** `strength=passphrase`）----
//
// 口径（`v1.3 §4.1` / `§4.4` / `I-1`，逐条）：
//
//	· `ls` 人类面**加一列「档」**、`show` 人类面**加一行「档」** —— 两处**同一取值口**
//	  （下面那枚 `approveStrengthMark()`）⇒ 两处不漂；
//	· `--json` 加**可选**字段 `strength`（`ls` / `show`）+ `key_id_in_use`（`show`）；
//	· ★ **可选** = 它**不进** `approveFields` 那九键 —— 九键是**冻结面**（帮助导出面 ↔ 件 ↔ 矩阵
//	  三面**逐字**对拍，见门⑮），往里加一格 = 把冻结面改了；可选字段是**只增**的第二层
//	  ⇒ 旧消费者点九键照旧、新消费者多一个可点项（`D1` 红线：**不许**塞进必填集）；
//	· `strength` 取值**只增不改**（闭集：`passphrase`（今天这档）/ `token`（将来那档））——
//	  在册公钥件里**有**这一格（`I-1` 的预留位）就照它，**没有** ⇒ 按 `passphrase` 判；
//	· ★ 本版**不写**这一格（`I-1` = 只写接口、不落实现 · 在册公钥件**一个字节不动** ✗）。
var approveOptionalFields = []string{"strength", "key_id_in_use"}

// approveStrengthPassphrase —— 生效档的强度标记（本版唯一取值 · `v1.3 §9.1` 的生效档）。
const approveStrengthPassphrase = "passphrase"

// approveStrengthInUse —— 档位强度的**唯一取值口**（人面与机器面都从它取 · `D1`「两处不漂」）。
//
// 读法照 `I-1`：在册公钥件的**可选**字段 `strength`（读得动且非空 ⇒ 就是它）⇒ 否则 `passphrase`。
// ★ **只读**：这一枚函数**不写**任何件（`I-1` 是预留接口；在册三枚件的字节不许动 ✗）。
func approveStrengthInUse() string {
	if b, err := os.ReadFile(operatorPubPath()); err == nil {
		var opt struct {
			Strength string `json:"strength"`
		}
		if json.Unmarshal(b, &opt) == nil {
			if s := strings.TrimSpace(opt.Strength); s != "" {
				return s
			}
		}
	}
	return approveStrengthPassphrase
}

// approveStrengthMark —— 人面那一格：**显式打** `strength=<档>`（`P26` = 甲：不靠 `sig_alg` 反推）。
func approveStrengthMark() string { return "strength=" + approveStrengthInUse() }

// approveFieldOptional —— 这一格是不是「可选字段」（`--json` 的字段面判定用它）。
func approveFieldOptional(f string) bool {
	for _, o := range approveOptionalFields {
		if f == o {
			return true
		}
	}
	return false
}

// keyIterations —— PBKDF2 的轮数（显式常数 · `RC6`「阈值显式、不许魔数」）。
const keyIterations = 200000

// operatorKeyFile —— 私钥件（密文）与公钥件（明文）的落点。
type operatorKeyFile struct {
	Alg       string `json:"alg"`        // 固定 `ed25519+pbkdf2-sha256+aes-256-gcm`
	Salt      string `json:"salt"`       // base64
	Iters     int    `json:"iters"`      // PBKDF2 轮数（显式）
	Nonce     string `json:"nonce"`      // base64
	CT        string `json:"ct"`         // base64（解出来 = ed25519 私钥 64 字节）
	Pub       string `json:"pub"`        // base64（公钥 · 与 operator.pub 同值）
	CreatedAt string `json:"created_at"` // RFC3339
	By        string `json:"by"`         // 谁生成的（人名）
}

// operatorPubFile —— 公钥件（**消费者读它** ⇒ 明文、可回读）。
type operatorPubFile struct {
	Alg   string `json:"alg"`    // `ed25519`
	Pub   string `json:"pub"`    // base64
	KeyID string `json:"key_id"` // sha256(pub)[:16]
	At    string `json:"at"`
}

// approvalTicketFile —— 批准件（消费者读的那个面：字段名与 chat 侧的 `approvalTicket` **同一张表的超集**）。
type approvalTicketFile struct {
	Tool       string `json:"tool"`
	Approver   string `json:"approver"`
	ApprovedAt string `json:"approved_at"`
	Scope      string `json:"scope"`
	Note       string `json:"note"`
	SigAlg     string `json:"sig_alg"`
	KeyID      string `json:"key_id"`
	Sig        string `json:"sig"`
}

// approveDir —— 批准件目录（与消费者同一处：`<状态目录>/approvals`）。
func approveDir() string { return filepath.Join(stateDirOf(), "approvals") }

// stateDirOf —— 状态目录（`ZERG_STATE_DIR` > `~/.zerg/state`）——与 `core/internal/statepath.Dir()` 同口径。
func stateDirOf() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); d != "" {
		return d
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".zerg", "state")
	}
	return "/tmp/zerg-state"
}

func operatorKeyPath() string { return filepath.Join(approveDir(), "operator.key") }
func operatorPubPath() string { return filepath.Join(approveDir(), "operator.pub") }

// cmdApprove —— `zerg approve <动作>`（动作 = ls / show / new / keygen，各是命令树里一条）。
func cmdApprove(inv *invocation, stdout, stderr io.Writer) int {
	action := ""
	if len(inv.path) > 1 {
		action = inv.path[1]
	}
	switch action {
	case "ls":
		return cmdApproveLs(inv, stdout, stderr)
	case "show":
		return cmdApproveShow(inv, stdout, stderr)
	case "new":
		return cmdApproveNew(inv, stdout, stderr)
	case "keygen":
		return cmdApproveKeygen(inv, stdout, stderr)
	}
	inv.setErr("usage", "unknown_action", "未知动作")
	fmt.Fprintf(stderr, "%s: 未知 `approve` 动作 %q（可用：ls · show · new · keygen）\n", progName, action)
	return exitUsage
}

// ---- ls / show（只读面）----

func cmdApproveLs(inv *invocation, stdout, stderr io.Writer) int {
	// ★ 字段面**先判**（`--json <未知字段>` ⇒ 2）：不能等渲染到行才判 —— 件为 0 件时那条路
	// 会直接出「共 0 条」并退 0，于是「未知字段」在空结果集下**悄悄变成合法**（本批实测抓到：
	// 门⑪ 的矩阵 case `approve ls --json name` 在空状态目录下退 0、在有件的目录下退 2 —— 判据飘）。
	if inv.jsonGiven && len(inv.fields) > 0 {
		// ★ D1 落地：字段面判定抽成一枚共用口（九键 + **可选**字段）；九键那一份是冻结面，
		// **不许**并进 `approveFields`（见上方档位面那段口径）。
		if bad := approveBadField(inv.path, inv.fields); bad != "" {
			return reportBadField(stderr, inv.path, bad)
		}
	}
	dir := approveDir()
	ents, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		inv.setErr("blocked", "approvals_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了批准件目录 %s：%v ⇒ 不给结论（退码 8）\n", progName, dir, err)
		return exitBlocked
	}
	rows := []map[string]string{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), "operator.") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var tk approvalTicketFile
		if json.Unmarshal(b, &tk) != nil {
			continue
		}
		rows = append(rows, approveRow(tk, p, verifyTicketState(tk)))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["tool"] < rows[j]["tool"] })
	// ★ D1 落地（`P26` = 甲）：**只加列、不删列、不换序** —— 现跑那六列（`tool/approver/
	// approved_at/scope/state/path`）一个不动，新列「档」**追加在末尾**（`v1.3 §4.1` 的落法）。
	return listCmd(inv, stdout, stderr, []string{"tool", "approver", "approved_at", "scope", "state", "path", "档"}, rows)
}

func cmdApproveShow(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) < 1 {
		inv.setErr("usage", "missing_target", "缺工具名")
		fmt.Fprintf(stderr, "%s: `approve show` 要给工具名（先 `%s approve ls`）\n", progName, progName)
		return exitUsage
	}
	tool := inv.args[0]
	p := filepath.Join(approveDir(), tool+".json")
	b, err := os.ReadFile(p)
	if err != nil {
		inv.setErr("failed", "approval_not_found", "没有这枚批准件")
		fmt.Fprintf(stderr, "%s: 没有工具 %q 的批准件（%s）\n", progName, tool, p)
		fmt.Fprintf(stderr, "要放行它就由**人**签一枚：%s approve new --tool %s --by <人名> --note <理由>\n", progName, tool)
		return exitFail
	}
	var tk approvalTicketFile
	if err := json.Unmarshal(b, &tk); err != nil {
		inv.setErr("failed", "approval_unparsable", err.Error())
		fmt.Fprintf(stderr, "%s: 批准件解不动（%s）：%v\n", progName, p, err)
		return exitFail
	}
	state := verifyTicketState(tk)
	row := approveRow(tk, p, state)
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail
		}
		// ★ D1 落地：字段面**先判**（与 `ls` 同一枚判定口）——今天这一路**不看**字段表
		// （点 `--json name` 也 rc=0、还把九格全打出来）⇒ 可选字段（`strength` /
		// `key_id_in_use`）**根本取不出来**。落 D1 就得把这一格补上：**点哪几格给哪几格**（§4.1 K1）。
		// 九键**语义与顺序**一字不动（`approveFields` 那一份没改）；变的是「以前忽略你的点单」。
		if bad := approveBadField(inv.path, inv.fields); bad != "" {
			return reportBadField(stderr, inv.path, bad)
		}
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "工具     : %s\n", tk.Tool)
	fmt.Fprintf(stdout, "批准者   : %s\n", tk.Approver)
	fmt.Fprintf(stdout, "签的时间 : %s\n", tk.ApprovedAt)
	fmt.Fprintf(stdout, "范围     : %s\n", tk.Scope)
	fmt.Fprintf(stdout, "理由     : %s\n", tk.Note)
	fmt.Fprintf(stdout, "签名     : alg=%s key_id=%s\n", tk.SigAlg, tk.KeyID)
	// ★ D1 落地（`P26` = 甲）：**生效档显式打** —— 取值口与 `ls` 那一列是**同一枚**
	// `approveStrengthMark()`（两处不漂）；这一行是**新增行**，上面八行一字不动。
	fmt.Fprintf(stdout, "档       : %s\n", approveStrengthMark())
	fmt.Fprintf(stdout, "钥匙身份 : %s\n", keyIdentityLine(tk.KeyID))
	fmt.Fprintf(stdout, "判决     : %s\n", state)
	fmt.Fprintf(stdout, "落点     : %s\n", p)
	return exitOK
}

// keyIdentityLine —— `show` 的「钥匙身份」那一行（`D3` · `v1.3 §9.2 N2′`）。
//
// 口径（**只显示、不拦** ✗）：强度判定的真源是**在册公钥**，不是件里那一格 —— 所以这一行的取值口 =
// **在册** `operator.pub` 的 `key_id`（现成的 `control.KeyID()` 口径 · `approval.go:42`），**不新增判定口**，
// 也**不动**判决列那四档（`验过` / `无签名` / `签名坏` / `alg 不认` 的扩法见 `v1.3 §4.4`，
// 本任务**只加这一行**）。打「不在册」**不改**判决、**不改**放行 —— 「消费者是否**要求**一致」
// 是策略收紧（不可逆档 `I3`）⇒ 那一半**仍待亲选**（`P3`），本行照实只报告读数。
func keyIdentityLine(ticketKeyID string) string {
	inUse, ok := operatorKeyIDInUse()
	if !ok {
		return fmt.Sprintf("key_id=%s · 在册公钥件读不到（%s）—— **只是读数** · 不改判决",
			ticketKeyID, operatorPubPath())
	}
	if strings.TrimSpace(ticketKeyID) == inUse {
		return fmt.Sprintf("key_id=%s · **在册**（与在册公钥 %s 逐字一致）", ticketKeyID, inUse)
	}
	return fmt.Sprintf("key_id=%s · **不在册**（在册公钥是 %s）—— 只显示、不拦；判定口只在控制层",
		ticketKeyID, inUse)
}

// operatorKeyIDInUse —— **在册**公钥的 `key_id`（「件里那格是不是当前在册」的唯一取值口 · 只读）。
// 读不到 / 解不动 / 该格空 ⇒ `("", false)`：调用方照实打「读不到」，**不许**把它当「不在册」以外的结论用。
func operatorKeyIDInUse() (string, bool) {
	b, err := os.ReadFile(operatorPubPath())
	if err != nil {
		return "", false
	}
	var pf operatorPubFile
	if json.Unmarshal(b, &pf) != nil {
		return "", false
	}
	kid := strings.TrimSpace(pf.KeyID)
	if kid == "" {
		return "", false
	}
	return kid, true
}

// verifyTicketState —— 三值判决：`验过` / `无签名` / `签名坏`（消费者只认第一种）。
func verifyTicketState(tk approvalTicketFile) string {
	if strings.TrimSpace(tk.Sig) == "" {
		return "无签名（不算批准）"
	}
	pub, err := os.ReadFile(operatorPubPath())
	if err != nil {
		return "公钥读不到（不算批准 · fail-closed）"
	}
	var pf operatorPubFile
	if json.Unmarshal(pub, &pf) != nil {
		return "公钥件解不动（不算批准）"
	}
	payload := control.ApprovalPayload(tk.Tool, tk.Scope, tk.Approver, tk.ApprovedAt, tk.Note)
	if err := control.VerifyApprovalSignature(pf.Pub, tk.Sig, payload); err != nil {
		return "签名坏（不算批准）：" + err.Error()
	}
	return "验过"
}

func approveRow(tk approvalTicketFile, path, state string) map[string]string {
	// ★ `strength` / `key_id_in_use` 是**可选**字段（`D1` · `v1.3 §4.1`）——它们放在行里供
	// `--json` 点名取用，但**不进** `approveFields` 九键（九键是冻结面）；不点名就不出现在输出里。
	inUse, _ := operatorKeyIDInUse()
	return map[string]string{
		"tool": tk.Tool, "approver": tk.Approver, "approved_at": tk.ApprovedAt,
		"scope": tk.Scope, "note": tk.Note, "sig_alg": tk.SigAlg, "key_id": tk.KeyID,
		"state": state, "path": path,
		"strength": approveStrengthInUse(), "key_id_in_use": inUse,
		// 「档」那一列的人面取值（`ls` 的表头就是这四个字 · `D1`）——与 `show` 那行**同一枚**
		// `approveStrengthMark()`（两处不漂）。
		"档": approveStrengthMark(),
	}
}

// approveBadField —— 字段面**先判**（九键 + 可选字段）：返回点错的那一格（没点错 ⇒ 空串）。
//
// 两处（`ls` / `show`）共用这一枚 ⇒ 口径只有一份：**九键**是冻结面（点九键之外**非可选**的名
// ⇒ 报错并逐字列出九键）、**可选字段**（`strength` / `key_id_in_use`）点得出来但**不在**九键里。
func approveBadField(path []string, fields []string) string {
	known := map[string]bool{}
	for _, f := range fieldListOf(path) {
		known[f] = true
	}
	for _, f := range approveOptionalFields {
		known[f] = true
	}
	for _, f := range fields {
		if !known[f] {
			return f
		}
	}
	return ""
}

// ---- new（人签）----

func cmdApproveNew(inv *invocation, stdout, stderr io.Writer) int {
	if inv.selfTest {
		return approveSelfTest(stdout, stderr)
	}
	tool := strings.TrimSpace(inv.flagVal("--tool"))
	scope := strings.TrimSpace(inv.flagVal("--scope"))
	note := strings.TrimSpace(inv.flagVal("--note"))
	by := strings.TrimSpace(inv.flagVal("--by"))
	if tool == "" || by == "" || note == "" {
		missing := []string{}
		if tool == "" {
			missing = append(missing, "--tool <工具名>")
		}
		if by == "" {
			missing = append(missing, "--by <人名>")
		}
		if note == "" {
			missing = append(missing, "--note <理由>")
		}
		inv.setErr("usage", "missing_required_flag", "缺必需旗标")
		fmt.Fprintf(stderr, "%s: 缺 %s —— 批准件要**谁签的 + 为什么**（无签名的件不算批准）\n", progName, strings.Join(missing, " · "))
		return exitUsage
	}
	if scope == "" {
		scope = "*"
	}
	// ★ D2（`P16` / `M-12` 的落点）：**签前摘要两行** —— ① 人话摘要（五格都在行里 ⇒ 第三方拿同一组五格
	//   现算就能复算）② **待签字节**的 `sha256`。两行都在**真签之前**打出，`--dry-run` 也打。
	//   口径只有一处：下面这枚 `sha256` = `control.ApprovalPayload(tk 的五格)` 的 `sha256`
	//   （待签字节形态见 `core/internal/control/approval.go:25`：一句版本行 `zerg-approval/v1` + 五格）。
	//   ★ 它**不是**件文件的 `sha256`、也**不是**被改文件的前后 `sha256`（那两格记的是另一件事 · `[P13]`）。
	//   ★ 将来 `E1`（审计那一半）**必须复用这一口径**，不许各算各的（`v1.3 §8.4 P16`）。
	approvedAt := time.Now().Format(time.RFC3339)
	tk := approvalTicketFile{
		Tool: control.SanitizeApprovalField(tool), Approver: control.SanitizeApprovalField(by),
		ApprovedAt: approvedAt, Scope: control.SanitizeApprovalField(scope), Note: control.SanitizeApprovalField(note),
	}
	payload := control.ApprovalPayload(tk.Tool, tk.Scope, tk.Approver, tk.ApprovedAt, tk.Note)
	digest := sha256.Sum256(payload)
	printSummary := func() {
		if inv.jsonGiven {
			return
		}
		fmt.Fprintf(stdout, "摘要     : 工具 %s · 范围 %s · 批准者 %s · 签的时间 %s · 理由 %s\n",
			tk.Tool, tk.Scope, tk.Approver, tk.ApprovedAt, tk.Note)
		fmt.Fprintf(stdout, "待签 sha256: %s（%d 字节 · 待签字节 = `zerg-approval/v1` + 五格；**不是**件文件 / 被改文件的 sha256）\n",
			hex.EncodeToString(digest[:]), len(payload))
	}
	if inv.dryRun {
		// 干跑那一态：只出**计划面** —— 一个字节都不写（不落件 · 不落审计 · 不动在册件 · 不读口令）。
		// 这一态**不受终端判据约束**（它本来就不签）—— 两行照打，便于「AI 可提」那一半自证要签什么。
		printSummary()
		fmt.Fprintf(stderr, "（--dry-run：只出计划面 · **零副作用** —— 未签 · 未落件 · 未动在册件）\n")
		return exitOK
	}
	if !isTTYFile(os.Stdin) {
		// 这一态**一个字节都不往 stdout 写**（矩阵 `approve/new#非终端不给签（模型路径）` 判据 = stdout 0 字节 ·
		// 本仓铁律：拒绝那一态不产出任何「看起来像结果」的东西）。
		inv.setErr("usage", "not_a_human", "人签要人在终端上敲")
		fmt.Fprintf(stderr, "%s: **批准只能人在终端上敲** —— 非交互会话（管道 / 模型）一律不给签（退码 2）\n", progName)
		fmt.Fprintf(stderr, "口径（§17.3 铁律④③）：AI 可提、可建、可测、可验，**不可自批**；这一步的判据不是「谁敲的命令」而是「**谁的口令**」\n")
		return exitUsage
	}
	// 人在终端上：**先**把要签的东西打全（两行）—— 人看清了再敲口令，**然后**才签。
	printSummary()
	if _, err := os.Stat(operatorKeyPath()); err != nil {
		inv.setErr("usage", "operator_key_absent", "还没有操作员密钥")
		fmt.Fprintf(stderr, "%s: 还没有操作员密钥（%s）⇒ 不给签\n", progName, operatorKeyPath())
		fmt.Fprintf(stderr, "先由**人**在终端上生成：%s approve keygen --by <人名>\n", progName)
		return exitUsage
	}
	pass, err := readPassphrase("操作员口令（不会回显）：")
	if err != nil {
		inv.setErr("failed", "passphrase_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不到口令（%v）⇒ 不给签\n", progName, err)
		return exitFail
	}
	priv, err := unlockOperatorKey(pass)
	if err != nil {
		inv.setErr("usage", "passphrase_rejected", err.Error())
		fmt.Fprintf(stderr, "%s: %v ⇒ **不给签**（口令不对就是不能签，不重试、不猜）\n", progName, err)
		return exitUsage
	}
	// 签（在册私钥）—— 上面打出的两行讲的**就是**这串字节（同一枚 `payload`，口径不漂）。
	tk.SigAlg = "ed25519"
	tk.KeyID = control.KeyID(base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)))
	sig := ed25519.Sign(priv, payload)
	tk.Sig = base64.StdEncoding.EncodeToString(sig)

	// 写：**追加只写**那一族的口径（同名件不许悄悄覆盖 —— 要换先删，删是人的动作）
	if err := os.MkdirAll(approveDir(), 0o700); err != nil {
		inv.setErr("failed", "approvals_dir_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 建不了批准件目录 %s：%v（写失败即拒）\n", progName, approveDir(), err)
		return exitFail
	}
	p := filepath.Join(approveDir(), tk.Tool+".json")
	body, _ := json.MarshalIndent(tk, "", " ")
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		inv.setErr("failed", "approval_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不进批准件 %s：%v（同名件已存在就拒 —— 不吞不盖）\n", progName, p, err)
		fmt.Fprintf(stderr, "要换一枚：先由人把那枚件删掉（删除是人的动作，命令面不代劳）\n")
		return exitFail
	}
	if _, err := f.Write(append(body, '\n')); err != nil {
		f.Close()
		inv.setErr("failed", "approval_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写批准件失败：%v\n", progName, err)
		return exitFail
	}
	f.Close()
	// ★ E1（`I-6`）：把这一次签名落成审计**一行**（自己的字段名 · 只追加）。
	// **审计不是放行条件**：写不进审计 ⇒ 签名照旧有效，只在 stderr 如实报（把它接成放行条件
	// = 改真写前置 = 破坏性变更 ⇒ 越线 ✗）。
	if ap, aerr := recordApprovalAudit(tk, payload, p); aerr != nil {
		fmt.Fprintf(stderr, "%s: ⚠ 审计那一行没落成（%v）—— **本次签名照旧有效**（审计是记录面，不是放行条件；落点 %s）\n",
			progName, aerr, ap)
	} else {
		fmt.Fprintf(stderr, "  审计  ：%s（追加一行 `%s` · 只写不改）\n", ap, approveAuditEvent)
	}
	row := approveRow(tk, p, "验过")
	row["result"] = "signed"
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail
		}
		return selectJSON(stdout, stderr, inv, inv.path, approveNewFields, row)
	}
	fmt.Fprintf(stdout, "%s\t%s\t%s\n", tk.Tool, tk.Approver, approvedAt)
	fmt.Fprintf(stderr, "%s: 已签 %s（key_id=%s · 签了 %d 字节）\n", progName, p, tk.KeyID, len(sig))
	fmt.Fprintf(stderr, "它只作**逃生门**：控制层 action=require_approval 的工具凭它放行；签完的件能回读（`%s approve show %s`）\n", progName, tk.Tool)
	return exitOK
}

// ---- keygen（人签的密钥）----

func cmdApproveKeygen(inv *invocation, stdout, stderr io.Writer) int {
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		inv.setErr("usage", "missing_by", "缺 --by")
		fmt.Fprintf(stderr, "%s: `approve keygen` 要 `--by <人名>`（件里要留「谁生成的操作员密钥」）\n", progName)
		return exitUsage
	}
	if !isTTYFile(os.Stdin) {
		inv.setErr("usage", "not_a_human", "密钥要人在终端上生成")
		fmt.Fprintf(stderr, "%s: 生成操作员密钥要**人在终端上**敲（口令要人手输）—— 非交互会话一律拒（退码 2）\n", progName)
		return exitUsage
	}
	if _, err := os.Stat(operatorKeyPath()); err == nil {
		inv.setErr("usage", "operator_key_exists", "密钥已在")
		fmt.Fprintf(stderr, "%s: 操作员密钥已在（%s）⇒ 不覆盖（覆盖会让在册的旧件全部失效）\n", progName, operatorKeyPath())
		return exitUsage
	}
	pass, err := readPassphrase("设一个操作员口令（不会回显）：")
	if err != nil {
		inv.setErr("failed", "passphrase_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不到口令（%v）\n", progName, err)
		return exitFail
	}
	if len(pass) < 8 {
		inv.setErr("usage", "passphrase_too_short", "口令太短")
		fmt.Fprintf(stderr, "%s: 口令至少 8 个字符（这是**私钥的唯一保护**）⇒ 退码 2\n", progName)
		return exitUsage
	}
	again, err := readPassphrase("再敲一遍：")
	if err != nil || again != pass {
		inv.setErr("usage", "passphrase_mismatch", "两次口令不一致")
		fmt.Fprintf(stderr, "%s: 两次口令不一致 ⇒ 不生成（退码 2）\n", progName)
		return exitUsage
	}
	kf, pf, err := newOperatorKey(pass, by)
	if err != nil {
		inv.setErr("failed", "keygen_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 生成密钥失败：%v\n", progName, err)
		return exitFail
	}
	if err := os.MkdirAll(approveDir(), 0o700); err != nil {
		inv.setErr("failed", "approvals_dir_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 建不了目录 %s：%v\n", progName, approveDir(), err)
		return exitFail
	}
	kb, _ := json.MarshalIndent(kf, "", " ")
	pb, _ := json.MarshalIndent(pf, "", " ")
	if err := os.WriteFile(operatorKeyPath(), append(kb, '\n'), 0o600); err != nil {
		inv.setErr("failed", "key_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不了私钥件 %s：%v\n", progName, operatorKeyPath(), err)
		return exitFail
	}
	if err := os.WriteFile(operatorPubPath(), append(pb, '\n'), 0o644); err != nil {
		inv.setErr("failed", "pub_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不了公钥件 %s：%v\n", progName, operatorPubPath(), err)
		return exitFail
	}
	fmt.Fprintf(stdout, "%s\t%s\n", pf.KeyID, operatorPubPath())
	fmt.Fprintf(stderr, "%s: 密钥已生成（key_id=%s）\n", progName, pf.KeyID)
	fmt.Fprintf(stderr, "  私钥：%s（**口令加密** · 权限 600 —— 口令只在人脑子里）\n", operatorKeyPath())
	fmt.Fprintf(stderr, "  公钥：%s（消费者读它验签）\n", operatorPubPath())
	return exitOK
}

// ---- 审计面（`E1` · `I-6`：新增事件类型 + **自己的字段名**）----
//
// 为什么单列一节：现网 `edit_audit.jsonl` 那 179 行讲的是**改件**（`before_sha256` / `after_sha256`
// 记的是**被改文件的前后 hash**），本事件讲的是**签名前的内容**（待签字节 = `zerg-approval/v1` + 五格）。
// 两件事**不是同一件事**（`[P13]` 那条陷阱 = 事后把两格读成一格）⇒ 本事件的每一格都**用自己的名字**：
//
//	· 事件名      `event`                  = `approve_signed`
//	· 该 `sha256`  `signed_payload_sha256`（**不蹭** `before_sha256` / `after_sha256`）
//	· 字节数      `signed_payload_bytes` （**不蹭** `before_bytes` / `after_bytes`）
//	· 件名        `tool`
//	· 谁签的      `approver`
//	· 时刻        `at`
//	· 件落点      `approval_path`          （**不蹭** `approval` —— 那是别处的字段名）
//	· 明标        `signed_not_in_person`   （`E2` · `L3-b` = **B 案**：明标落**审计侧**，件字段面一字不动）
//	· 档位        `strength`               （`D1` / `P26` = 甲）
//	· 在册钥      `key_id`
//
// 三条纪律：① **追加只写**（`O_APPEND` · 历史行逐字节不变）；② **缺任一必需格 ⇒ 这一行不许写**（判据①）；
// ③ **不许接成放行条件**（写不进审计 ⇒ 照旧签，只报）。
type approvalAuditLine struct {
	At                  string `json:"at"`
	Event               string `json:"event"`
	Tool                string `json:"tool"`
	Approver            string `json:"approver"`
	ApprovalPath        string `json:"approval_path"`
	SignedPayloadSHA256 string `json:"signed_payload_sha256"`
	SignedPayloadBytes  int    `json:"signed_payload_bytes"`
	SignedNotInPerson   bool   `json:"signed_not_in_person"`
	Strength            string `json:"strength"`
	KeyID               string `json:"key_id"`
}

// approveAuditEvent —— 事件名（本事件唯一 · 与现网那个 `edit` 事件不同名）。
const approveAuditEvent = "approve_signed"

// approveAuditRequired —— **必需格**（缺任一 ⇒ 这一行不写 · 判据①）。
var approveAuditRequired = []string{"at", "event", "tool", "approver", "approval_path", "signed_payload_sha256"}

// approveAuditOwnFields —— 本事件**自己的**字段名（新概念一律不复用现网那六格的名字 · 判据②）。
var approveAuditOwnFields = []string{"signed_payload_sha256", "signed_payload_bytes", "approval_path", "signed_not_in_person"}

// approveAuditLegacyFields —— 现网 `edit_audit.jsonl` 尾条那六格：新字段名**不许**与它们撞（判据②）。
var approveAuditLegacyFields = []string{"before_sha256", "after_sha256", "before_bytes", "after_bytes", "approval", "approver"}

// auditCells —— 必需格那一族（判 `complete()` 用 · 只列必需的那几格）。
func (l approvalAuditLine) auditCells() map[string]string {
	return map[string]string{
		"at": l.At, "event": l.Event, "tool": l.Tool, "approver": l.Approver,
		"approval_path": l.ApprovalPath, "signed_payload_sha256": l.SignedPayloadSHA256,
	}
}

// complete —— 判「必需格齐不齐」：缺任一 ⇒ 报错（调用方**不许**写这一行 · 判据①）。
func (l approvalAuditLine) complete() error {
	cells := l.auditCells()
	missing := []string{}
	for _, f := range approveAuditRequired {
		if strings.TrimSpace(cells[f]) == "" {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("审计行缺必需格 %s ⇒ **这一行不写**", strings.Join(missing, " · "))
	}
	if len(l.SignedPayloadSHA256) != 64 {
		return fmt.Errorf("`signed_payload_sha256` 不是 64 位十六进制（%q）⇒ 这一行不写", l.SignedPayloadSHA256)
	}
	return nil
}

// appendApprovalAudit —— 追加一行（`O_APPEND` · 一行一事件）。**只写不改**：历史行逐字节不变（判据③）。
func appendApprovalAudit(path string, line approvalAuditLine) error {
	if path == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	if err := line.complete(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(line)
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

// recordApprovalAudit —— `new` 签完之后那一步（`E1`）：把这一次签名落成审计一行，返回落点。
//
// ★ `payload` **必须**是**待签字节**本身（与打出那两行、与签名用的是**同一枚** · `D2` 的口径）——
// `sha256` 口径**只有一处**，`E1` **不许**各算各的（`v1.3 §8.4 P16`）。
func recordApprovalAudit(tk approvalTicketFile, payload []byte, ticketPath string) (string, error) {
	line := approvalAuditLine{
		At:                  time.Now().Format(time.RFC3339),
		Event:               approveAuditEvent,
		Tool:                tk.Tool,
		Approver:            tk.Approver,
		ApprovalPath:        ticketPath,
		SignedPayloadSHA256: sha256Of(payload),
		SignedPayloadBytes:  len(payload),
		// `E2` · `L3-b` = **B 案**：这一格是**明标**（`signed_not_in_person`）。
		// 今天这一路 = **人在终端上当场签** ⇒ 恒 `false`；补签那一路（本版**不实现**）才写 `true`。
		SignedNotInPerson: false,
		Strength:          approveStrengthInUse(),
		KeyID:             tk.KeyID,
	}
	p := editAuditPath()
	if err := appendApprovalAudit(p, line); err != nil {
		return p, err
	}
	return p, nil
}

// approveAuditMarkOf —— `E2`（`L3-b` = B 案）的**判定口**：从审计一行的原始 JSON 里读**明标**。
//
//	("在场", nil) —— 有 `signed_not_in_person` 这一格，值为 false；
//	("补签", nil) —— 有这一格，值为 true；
//	("", err)     —— **这一格不在** ⇒ **不可分** ⇒ 判红（负控：抹掉明标不许蒙过去）。
//
// 口径与判据件（`scripts/gates/check-approve-backfill.py`）**同一套**：两处都只认**显式**的这一格，
// 「没这一格」**不是**「在场」（硬件稿 §2.3 逐字：否则两件在审计里长得一模一样）。
func approveAuditMarkOf(raw []byte) (string, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return "", fmt.Errorf("审计行解不动：%v", err)
	}
	v, ok := probe["signed_not_in_person"]
	if !ok {
		return "", errors.New("这一行**没有明标**（`signed_not_in_person` 不在）⇒ 不可分：当场判红")
	}
	var in bool
	if err := json.Unmarshal(v, &in); err != nil {
		return "", fmt.Errorf("明标解不动（%s）：%v", string(v), err)
	}
	if in {
		return "补签", nil
	}
	return "在场", nil
}

// ---- 换钥归档（`E3` · `§4.2` 的归档路径 + `I-8`）----
//
// 口径（逐字）：旧 `operator.key` / `operator.pub` **不删**，**移到** `<状态目录>/approvals/archive/
// <旧 key_id>.{key,pub}`（**状态目录内 · 不入仓 · 不进公开面 · 不进归档区** ✗）。
//
// 顺序照本仓两条既有纪律长（`v1.3 §⑥`）：
//
//	① **备份先行**（`scripts/build/zerg-swap-core.sh:324` 逐字「备份失败 ⇒ 不换件」）—— 先把两枚件
//	   复制进 archive 并**逐字核对 `sha256`**；核对不过 ⇒ **组件原样**（一步 `os.Remove` 都不走）；
//	② **先自检后不可逆**（`scripts/build/zerg-upgrade.sh:901` 逐字「sha 必须在签名前校验」）——
//	   确认值必须与**在册**那一枚 `key_id` **逐字相同**（差一字符 ⇒ 拒）：否则等于把 A 钥的件
//	   归档到 B 钥名下（那正是 `E5` 判据② 要挡的形态）。
//
// ★ 本版**入口未开**：换钥入口（`--rekey` + 确认对）属 `E5`，而 `--rekey` 是 `P27` 的**预留旗标名**
// ⇒ §五 ⑩「不许占用预留旗标名」⇒ 本函数今天**只被自检与测试调用**。**机制在、入口不在** ——
// 如实登记，**不许**把「机制已落」说成「换钥已可用」✗。
func archiveDirOf() string { return filepath.Join(approveDir(), "archive") }

// archiveOperatorKeys 把在册那一对旧钥归档（返回两枚归档件的路径）。任一步不过 ⇒ 原件一枚不删。
func archiveOperatorKeys(oldKeyID string) (string, string, error) {
	oldKeyID = strings.TrimSpace(oldKeyID)
	inUse, ok := operatorKeyIDInUse()
	if !ok {
		return "", "", fmt.Errorf("在册公钥件读不到 ⇒ 不归档（%s）", operatorPubPath())
	}
	if oldKeyID == "" || oldKeyID != inUse {
		return "", "", fmt.Errorf("确认值 %q 与在册 `key_id` %q **不逐字相同** ⇒ 不动作", oldKeyID, inUse)
	}
	type pair struct {
		src, dst string
		mode     os.FileMode
	}
	pairs := []pair{
		{operatorKeyPath(), filepath.Join(archiveDirOf(), oldKeyID+".key"), 0o600},
		{operatorPubPath(), filepath.Join(archiveDirOf(), oldKeyID+".pub"), 0o644},
	}
	for _, p := range pairs {
		if _, err := os.Stat(p.src); err != nil {
			return "", "", fmt.Errorf("旧钥件不在（%s）：%v ⇒ 不归档、不删", p.src, err)
		}
	}
	if err := os.MkdirAll(archiveDirOf(), 0o700); err != nil {
		return "", "", fmt.Errorf("建不了归档位 %s：%v ⇒ 不动作（备份先行）", archiveDirOf(), err)
	}
	done := []string{}
	for _, p := range pairs {
		b, err := os.ReadFile(p.src)
		if err != nil {
			return "", "", fmt.Errorf("读不了 %s：%v ⇒ 不动作（原件一枚没删）", p.src, err)
		}
		want := sha256Of(b)
		if err := os.WriteFile(p.dst, b, p.mode); err != nil {
			return "", "", fmt.Errorf("写不进归档件 %s：%v ⇒ 不动作（原件一枚没删）", p.dst, err)
		}
		got, err := os.ReadFile(p.dst)
		if err != nil || sha256Of(got) != want {
			return "", "", fmt.Errorf("归档件与原件 `sha256` 不一致（%s）⇒ 不动作、原件不删", p.dst)
		}
		done = append(done, p.dst)
	}
	// 两枚都进了归档（且逐字核对过）才删原件 —— 「先复制后删」。
	for _, p := range pairs {
		if err := os.Remove(p.src); err != nil {
			return done[0], done[1], fmt.Errorf("原件 %s 删不掉：%v（归档件已在 · 字节没丢）", p.src, err)
		}
	}
	return done[0], done[1], nil
}

// ---- 密钥与口令 ----

// newOperatorKey 生成密钥对：私钥用**口令派生的密钥**加密（PBKDF2-HMAC-SHA256 + AES-256-GCM）落盘。
func newOperatorKey(pass, by string) (operatorKeyFile, operatorPubFile, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return operatorKeyFile{}, operatorPubFile{}, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return operatorKeyFile{}, operatorPubFile{}, err
	}
	key := pbkdf2SHA256([]byte(pass), salt, keyIterations, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return operatorKeyFile{}, operatorPubFile{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return operatorKeyFile{}, operatorPubFile{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return operatorKeyFile{}, operatorPubFile{}, err
	}
	ct := gcm.Seal(nil, nonce, priv, nil)
	b64 := base64.StdEncoding.EncodeToString
	now := time.Now().Format(time.RFC3339)
	pubB64 := b64(pub)
	return operatorKeyFile{
			Alg: "ed25519+pbkdf2-sha256+aes-256-gcm", Salt: b64(salt), Iters: keyIterations,
			Nonce: b64(nonce), CT: b64(ct), Pub: pubB64, CreatedAt: now, By: by,
		}, operatorPubFile{
			Alg: "ed25519", Pub: pubB64, KeyID: control.KeyID(pubB64), At: now,
		}, nil
}

// unlockOperatorKey 用口令解开私钥（口令错 ⇒ 报错，**不重试不猜**）。
func unlockOperatorKey(pass string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(operatorKeyPath())
	if err != nil {
		return nil, fmt.Errorf("读不了私钥件：%v", err)
	}
	var kf operatorKeyFile
	if err := json.Unmarshal(b, &kf); err != nil {
		return nil, fmt.Errorf("私钥件解不动：%v", err)
	}
	salt, err1 := base64.StdEncoding.DecodeString(kf.Salt)
	nonce, err2 := base64.StdEncoding.DecodeString(kf.Nonce)
	ct, err3 := base64.StdEncoding.DecodeString(kf.CT)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, errors.New("私钥件字段不是合法 base64")
	}
	iters := kf.Iters
	if iters <= 0 {
		iters = keyIterations
	}
	key := pbkdf2SHA256([]byte(pass), salt, iters, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, errors.New("口令不对（解不开私钥 —— GCM 校验没过）")
	}
	if len(plain) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("解出来的私钥长度不对（%d 字节）", len(plain))
	}
	return ed25519.PrivateKey(plain), nil
}

// pbkdf2SHA256 —— PBKDF2-HMAC-SHA256（本仓 vendor 里没有 `x/crypto`，就这一处需要它 ⇒ 自带 10 行，
// 不引依赖：加依赖是比这个函数大得多的动作）。
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	h := sha256.New
	blocks := (keyLen + h().Size() - 1) / h().Size()
	out := make([]byte, 0, blocks*h().Size())
	for i := 1; i <= blocks; i++ {
		mac := hmac.New(h, password)
		mac.Write(salt)
		mac.Write([]byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)})
		u := mac.Sum(nil)
		t := append([]byte{}, u...)
		for j := 1; j < iter; j++ {
			mac = hmac.New(h, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for k := range t {
				t[k] ^= u[k]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

// isTTYFile —— 是不是**真终端**。
//
// ★ 为什么不能只看 `ModeCharDevice`（本件的第一个实现就踩了这坑，实测抓到的）：`/dev/null` 也是
// 字符设备 ⇒ 模型/脚本把 stdin 接到 `/dev/null` 时会被判成「人在终端上」——**这道闸当场形同虚设**。
// 真判据 = 拿这个 fd 跑一次 `stty -a`：它只在**有控制终端**时才成功（`/dev/null`、管道、重定向都不是）。
func isTTYFile(f *os.File) bool {
	if st, err := f.Stat(); err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	cmd := exec.Command("stty", "-a")
	cmd.Stdin = f
	return cmd.Run() == nil
}

// readPassphrase 在**终端**上读一行口令（回显关掉：`stty -echo`；读完立刻恢复 —— 不留副作用）。
func readPassphrase(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("开不了 /dev/tty：%v", err)
	}
	defer tty.Close()
	if err := sttyEcho(tty, false); err != nil {
		return "", err
	}
	fmt.Fprint(tty, prompt)
	r := bufio.NewReader(tty)
	s, err := r.ReadString('\n')
	_ = sttyEcho(tty, true)
	fmt.Fprintln(tty)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

// sttyEcho 开关回显（借系统的 `stty`：本仓不引 `x/term` 依赖；失败即拒 —— 宁可拒也不明文读口令）。
func sttyEcho(tty *os.File, on bool) error {
	arg := "-echo"
	if on {
		arg = "echo"
	}
	cmd := exec.Command("stty", arg)
	cmd.Stdin = tty
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`stty %s` 失败（%v）⇒ 不给签（不许明文读口令）", arg, err)
	}
	return nil
}

// ---- `--self-test`：人签链路的**成对负控**（合成夹具 · 不碰真目标）----
//
// 四格：① 密钥对生成 + 签名 + 验签 ⇒ 过；② 改一格 ⇒ 验不过；③ 换公钥 ⇒ 验不过；
// ④ 解错口令 ⇒ 解不开。全部在 `t.TempDir()` 等价的临时目录里（这里用 `os.MkdirTemp` + 显式清理）。
func approveSelfTest(stdout, stderr io.Writer) int {
	dir, err := os.MkdirTemp("", "zerg-approve-selftest-")
	if err != nil {
		fmt.Fprintf(stderr, "%s: 建不了夹具目录：%v\n", progName, err)
		return exitFail
	}
	defer os.RemoveAll(dir)
	old := os.Getenv("ZERG_STATE_DIR")
	defer os.Setenv("ZERG_STATE_DIR", old)
	os.Setenv("ZERG_STATE_DIR", dir)

	kf, pf, err := newOperatorKey("合成口令-passphrase", "自检")
	if err != nil {
		fmt.Fprintf(stderr, "%s: 自检① 生成密钥失败：%v\n", progName, err)
		return exitFail
	}
	key := pbkdf2SHA256([]byte("合成口令-passphrase"), mustB64(kf.Salt), kf.Iters, 32)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, mustB64(kf.Nonce), mustB64(kf.CT), nil)
	if err != nil || len(plain) != ed25519.PrivateKeySize {
		fmt.Fprintf(stderr, "%s: 自检① 口令解开私钥失败：%v\n", progName, err)
		return exitFail
	}
	priv := ed25519.PrivateKey(plain)
	payload := control.ApprovalPayload("terminal", "*", "Mr2109", "2026-09-21T00:00:00+08:00", "自检")
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))
	bad := 0
	// ① 正控：签了就能验过
	if err := control.VerifyApprovalSignature(pf.Pub, sig, payload); err != nil {
		fmt.Fprintf(stderr, "%s: 自检① 正控失败（签了却验不过）：%v\n", progName, err)
		bad++
	}
	// ② 负控：改一格（note）⇒ 验不过
	if err := control.VerifyApprovalSignature(pf.Pub, sig,
		control.ApprovalPayload("terminal", "*", "Mr2109", "2026-09-21T00:00:00+08:00", "改过的理由")); err == nil {
		fmt.Fprintf(stderr, "%s: 自检② 负控失败（改了格还验得过）\n", progName)
		bad++
	}
	// ③ 负控：拿**别的**公钥 ⇒ 验不过（手写件 / 自造密钥那条路）
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	if err := control.VerifyApprovalSignature(base64.StdEncoding.EncodeToString(other.Public().(ed25519.PublicKey)),
		sig, payload); err == nil {
		fmt.Fprintf(stderr, "%s: 自检③ 负控失败（别的公钥也验得过）\n", progName)
		bad++
	}
	// ④ 负控：错口令 ⇒ 解不开
	if _, err := gcm.Open(nil, mustB64(kf.Nonce), mustB64(kf.CT), nil); err == nil {
		_ = err
	}
	wrongKey := pbkdf2SHA256([]byte("错口令"), mustB64(kf.Salt), kf.Iters, 32)
	wb, _ := aes.NewCipher(wrongKey)
	wg, _ := cipher.NewGCM(wb)
	if _, err := wg.Open(nil, mustB64(kf.Nonce), mustB64(kf.CT), nil); err == nil {
		fmt.Fprintf(stderr, "%s: 自检④ 负控失败（错口令也解得开）\n", progName)
		bad++
	}
	// ⑤ 负控：空签名 ⇒ 不算批准
	if err := control.VerifyApprovalSignature(pf.Pub, "", payload); err == nil {
		fmt.Fprintf(stderr, "%s: 自检⑤ 负控失败（空签名也验得过）\n", progName)
		bad++
	}
	if bad > 0 {
		fmt.Fprintf(stderr, "%s: `approve --self-test` 判红（%d 格不过）\n", progName, bad)
		return exitFail
	}
	fmt.Fprintln(stdout, "approve --self-test：5 格全过（① 正控 + ②③④⑤ 负控 · 合成夹具 · 未碰真目标）")
	fmt.Fprintf(stderr, "（自检：合成夹具目录 %s 已清理 · 真状态目录一个字节未动）\n", dir)
	return exitOK
}

func mustB64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return []byte{}
	}
	return b
}
