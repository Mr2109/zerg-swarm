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
		known := map[string]bool{}
		for _, f := range fieldListOf(inv.path) {
			known[f] = true
		}
		for _, f := range inv.fields {
			if !known[f] {
				return reportBadField(stderr, inv.path, f)
			}
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
	return listCmd(inv, stdout, stderr, []string{"tool", "approver", "approved_at", "scope", "state", "path"}, rows)
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
		return selectJSON(stdout, stderr, inv, inv.path, approveFields, row)
	}
	fmt.Fprintf(stdout, "工具     : %s\n", tk.Tool)
	fmt.Fprintf(stdout, "批准者   : %s\n", tk.Approver)
	fmt.Fprintf(stdout, "签的时间 : %s\n", tk.ApprovedAt)
	fmt.Fprintf(stdout, "范围     : %s\n", tk.Scope)
	fmt.Fprintf(stdout, "理由     : %s\n", tk.Note)
	fmt.Fprintf(stdout, "签名     : alg=%s key_id=%s\n", tk.SigAlg, tk.KeyID)
	fmt.Fprintf(stdout, "判决     : %s\n", state)
	fmt.Fprintf(stdout, "落点     : %s\n", p)
	return exitOK
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
	return map[string]string{
		"tool": tk.Tool, "approver": tk.Approver, "approved_at": tk.ApprovedAt,
		"scope": tk.Scope, "note": tk.Note, "sig_alg": tk.SigAlg, "key_id": tk.KeyID,
		"state": state, "path": path,
	}
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
	if !isTTYFile(os.Stdin) {
		inv.setErr("usage", "not_a_human", "人签要人在终端上敲")
		fmt.Fprintf(stderr, "%s: **批准只能人在终端上敲** —— 非交互会话（管道 / 模型）一律不给签（退码 2）\n", progName)
		fmt.Fprintf(stderr, "口径（§17.3 铁律④③）：AI 可提、可建、可测、可验，**不可自批**；这一步的判据不是「谁敲的命令」而是「**谁的口令**」\n")
		return exitUsage
	}
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
	approvedAt := time.Now().Format(time.RFC3339)
	tk := approvalTicketFile{
		Tool: control.SanitizeApprovalField(tool), Approver: control.SanitizeApprovalField(by),
		ApprovedAt: approvedAt, Scope: control.SanitizeApprovalField(scope), Note: control.SanitizeApprovalField(note),
		SigAlg: "ed25519", KeyID: control.KeyID(base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))),
	}
	sig := ed25519.Sign(priv, control.ApprovalPayload(tk.Tool, tk.Scope, tk.Approver, tk.ApprovedAt, tk.Note))
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
