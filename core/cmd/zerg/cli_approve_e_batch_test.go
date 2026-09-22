// cli_approve_e_batch_test.go —— E 批落地（`E1` 审计新事件 / `E2` 明标 = `L3-b` **B 案** / `E3` 归档机制）
// ＋ `D1` 落地面（档列 / 档行 / `--json` 可选字段）的判据。
//
// 判据映射（任务单-人签实施-20260922 §二 `D1` + §三 `E1`/`E2`/`E3`；逐条可单独失败）：
//
//	D1 ① 人面**只加不换**：`ls` 六列 / `show` 八行按序仍在（新增列「档」/ 新增行「档」追加在末尾）；
//	D1 ② 字段面：`approveFields` **仍是九键**（`strength` / `key_id_in_use` **不在**必填集里）·
//	     `--json <未知字段>` ⇒ 2 · `--json strength` ⇒ 0（可选字段真的取得出来）；
//	D1 ③/④ 人面**显式**出现 `strength=passphrase`；输出里**不许**出现 `token` / `PIN`；
//	E1 ① 必需格缺任一 ⇒ **这一行不许写**（`complete()` 先判）；
//	E1 ② 新字段名与现网 `edit_audit.jsonl` 六格**不撞**（静态自检）；
//	E1 ③ 追加只写：连记两次 ⇒ `head -n <旧行数>` **逐字节不变**；
//	E1 ④ 零改件：全程 `t.TempDir()`（真状态目录一个字节不碰 —— 落点解析就把这条钉住）；
//	E2 ① 正控：明标 `false` ⇒ 「在场」· 明标 `true` ⇒ 「补签」；**负控：抹掉明标 ⇒ 判红**；
//	E3 ① 换钥后旧钥签的件**不算批准**（在册换掉 ⇒ 判决不是「验过」）；② 反面：在册没换 ⇒ 「验过」；
//	E3 ③ 归档位在**状态目录内**；④ 原件**字节不丢**（归档件与原件逐字一致 · 原件才被移走）。
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

// eBatchRun —— 跑一条口令，把两路输出带回来（本文件里判输出文本用）。
func eBatchRun(argv ...string) (int, string, string) {
	var out, errb strings.Builder
	rc := RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// eBatchState —— 造一个合成状态目录并把它设为 `ZERG_STATE_DIR`（真状态目录**一个字节不碰**）。
func eBatchState(t *testing.T) string {
	t.Helper()
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	if err := os.MkdirAll(filepath.Join(state, "approvals"), 0o700); err != nil {
		t.Fatal(err)
	}
	return state
}

// eBatchTicket —— 写一枚**无签名**批准件（只看渲染面与明标面时用它；判决那一路用真签名夹具）。
func eBatchTicket(t *testing.T, state, tool string) string {
	t.Helper()
	body := `{
 "tool": "` + tool + `",
 "approver": "Mr2109",
 "approved_at": "2026-09-22T10:00:00+08:00",
 "scope": "*",
 "note": "E 批夹具",
 "sig_alg": "ed25519",
 "key_id": "0000000000000000",
 "sig": ""
}
`
	p := filepath.Join(state, "approvals", tool+".json")
	eBatchWrite(t, p, body)
	return p
}

// eBatchLabels —— 从 `show` 的人面里读回行标签（`^标签\s*:`）。
func eBatchLabels(out string) []string {
	labels := []string{}
	for _, ln := range strings.Split(out, "\n") {
		i := strings.Index(ln, ":")
		if i <= 0 {
			continue
		}
		lab := strings.TrimSpace(ln[:i])
		if lab == "" || strings.ContainsAny(lab, " \t") {
			continue
		}
		labels = append(labels, lab)
	}
	return labels
}

// eBatchOrdered —— `expected` 必须按序出现在 `actual` 里（**新增项允许** ⇒ 用「有序子序列」判）。
func eBatchOrdered(actual, expected []string) bool {
	i := 0
	for _, e := range expected {
		found := false
		for ; i < len(actual); i++ {
			if actual[i] == e {
				found = true
				i++
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// TestApproveEBatch_D1TierFace —— `D1` 落地：`ls` 加一列「档」· `show` 加一行「档」· `--json` 可选字段。
func TestApproveEBatch_D1TierFace(t *testing.T) {
	state := eBatchState(t)
	eBatchTicket(t, state, "合成件")

	rc, out, errb := eBatchRun("approve", "ls")
	if rc != 0 {
		t.Fatalf("D1 ① 破：`approve ls` 退码 %d\nstderr=\n%s", rc, errb)
	}
	lines := []string{}
	for _, ln := range strings.Split(out, "\n") {
		if strings.TrimSpace(ln) != "" {
			lines = append(lines, ln)
		}
	}
	if len(lines) < 2 {
		t.Fatalf("D1 ① 破：`approve ls` 人面只有 %d 行（表头 + 至少一条数据行才有东西可判）\n%s", len(lines), out)
	}
	head := strings.Split(lines[0], "\t")
	six := []string{"tool", "approver", "approved_at", "scope", "state", "path"}
	if !eBatchOrdered(head, six) {
		t.Errorf("D1 ① 破：表头 = %v（那六列必须**按序**在 · 只许加列）", head)
	}
	if head[len(head)-1] != "档" {
		t.Errorf("D1 ① 破：新列没追加在末尾（表头末格 = %q · 要 `档`）", head[len(head)-1])
	}
	for i, ln := range lines {
		if n := len(strings.Split(ln, "\t")); n != len(head) {
			t.Errorf("D1 ① 破：第 %d 行格数 %d ≠ 表头 %d（少一格 = 某列没渲染出来）", i+1, n, len(head))
		}
	}
	if cells := strings.Split(lines[1], "\t"); cells[len(cells)-1] != "strength=passphrase" {
		t.Errorf("D1 ③ 破：档列取值 = %q（`P26` = 甲 ⇒ 人要**显式**看到 `strength=passphrase`）", cells[len(cells)-1])
	}
	if strings.Contains(out, "token") || strings.Contains(out, "PIN") {
		t.Errorf("D1 ③/④ 破：人面出现 `token` / `PIN`（`M-20` ② 不许）\n%s", out)
	}

	// `show`：八行按序仍在 + 新增「档」行
	rcS, outS, errS := eBatchRun("approve", "show", "合成件")
	if rcS != 0 {
		t.Fatalf("D1 ① 破：`approve show` 退码 %d\nstderr=\n%s", rcS, errS)
	}
	eight := []string{"工具", "批准者", "签的时间", "范围", "理由", "签名", "判决", "落点"}
	labels := eBatchLabels(outS)
	if !eBatchOrdered(labels, eight) {
		t.Errorf("D1 ① 破：`show` 行标签 = %v（那八行必须**按序**在 · 只许加行）", labels)
	}
	wantTier := ""
	for _, ln := range strings.Split(outS, "\n") {
		if strings.HasPrefix(ln, "档") {
			wantTier = strings.TrimSpace(strings.TrimSpace(strings.TrimPrefix(ln, "档"))[1:])
		}
	}
	if wantTier != "strength=passphrase" {
		t.Errorf("D1 ③ 破：`show` 的「档」行 = %q（要 `strength=passphrase`）", wantTier)
	}
	if strings.Contains(outS, "token") || strings.Contains(outS, "PIN") {
		t.Errorf("D1 ③/④ 破：`show` 人面出现 `token` / `PIN`\n%s", outS)
	}

	// 判据② 字段面：九键**一个不多一个不少**（`strength` / `key_id_in_use` 不在里面）
	if got := strings.Join(approveFields, ","); got != "tool,approver,approved_at,scope,note,sig_alg,key_id,state,path" {
		t.Errorf("D1 ② 破：`approveFields` = %s（必须仍是九键：可选字段**不许**塞进必填集）", got)
	}
	rcU, _, errU := eBatchRun("approve", "ls", "--json", "nosuchfield")
	if rcU != 2 {
		t.Errorf("D1 ② 破：`--json <未知字段>` 退码 %d（要 2）", rcU)
	}
	if !strings.Contains(errU, "未知字段") || !strings.Contains(errU, "tool,approver,approved_at,scope,note,sig_alg,key_id,state,path") {
		t.Errorf("D1 ② 破：未知字段那一态要逐字列**九键**；stderr=\n%s", errU)
	}
	// ★ 注：失败路径上 `--json` 会补一个**错误包封**（§九 M7）⇒ stdout 非 0 字节是**既有**口径
	// （矩阵里那条 `approve ls --json name` 的 `want_stdout_bytes=282` 就是它）——本判据只钉 rc=2。
	rcU2, _, errU2 := eBatchRun("approve", "show", "合成件", "--json", "nosuchfield")
	if rcU2 != 2 || !strings.Contains(errU2, "未知字段") {
		t.Errorf("D1 ② 破：`show --json <未知字段>` 退码 %d（要 2 · 字段面先判这一条两处同口径）\nstderr=\n%s", rcU2, errU2)
	}
	// `strength` / `key_id_in_use` 是**可选**字段：点得出来（判据②的正控那一半）
	rcOpt, outOpt, errOpt := eBatchRun("approve", "ls", "--json", "tool,strength")
	if rcOpt != 0 {
		t.Fatalf("D1 ② 破：`--json tool,strength` 退码 %d（可选字段点不出来）\nstderr=\n%s", rcOpt, errOpt)
	}
	if !strings.Contains(outOpt, `"strength":"passphrase"`) {
		t.Errorf("D1 ②/③ 破：`--json strength` 的值不是 `passphrase`\n%s", outOpt)
	}
	rcK, outK, errK := eBatchRun("approve", "show", "合成件", "--json", "key_id_in_use")
	if rcK != 0 {
		t.Fatalf("D1 ② 破：`show --json key_id_in_use` 退码 %d\nstderr=\n%s", rcK, errK)
	}
	if !strings.Contains(outK, `"key_id_in_use"`) {
		t.Errorf("D1 ② 破：`key_id_in_use` 没打出来\n%s", outK)
	}
}

// TestApproveEBatch_E1AuditLine —— `E1`：新事件 + 自己的字段名 + 追加只写 + 缺格不写。
func TestApproveEBatch_E1AuditLine(t *testing.T) {
	state := eBatchState(t)
	payload := control.ApprovalPayload("dev_edit", "*", "Mr2109", "2026-09-22T10:00:00+08:00", "E1 夹具理由")
	tk := approvalTicketFile{
		Tool: "dev_edit", Approver: "Mr2109", ApprovedAt: "2026-09-22T10:00:00+08:00",
		Scope: "*", Note: "E1 夹具理由", SigAlg: "ed25519", KeyID: "8b69ad95939df5cf",
	}
	ticket := filepath.Join(state, "approvals", "dev_edit.json")
	auditPath, err := recordApprovalAudit(tk, payload, ticket)
	if err != nil {
		t.Fatalf("E1 ① 破：记一行审计失败：%v", err)
	}
	// E1 ④ 零改件：落点必须在合成状态目录里（解析面把「会写到真目录」这件事挡住）
	if auditPath != filepath.Join(state, "edit_audit.jsonl") || editAuditPath() != auditPath {
		t.Errorf("E1 ④ 破：审计落点 = %q（要 %q —— 不许落到真状态目录）", auditPath, filepath.Join(state, "edit_audit.jsonl"))
	}
	first := eBatchReadAll(t, auditPath)
	lines := strings.Split(strings.TrimRight(first, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("E1 ① 破：追加一行之后行数 = %d（要 1）", len(lines))
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("E1 ① 破：这一行不是合法 JSON：%v\n%s", err, lines[0])
	}
	for _, f := range approveAuditRequired {
		if v, ok := got[f]; !ok || strings.TrimSpace(eBatchStr(v)) == "" {
			t.Errorf("E1 ① 破：必需格 %q 缺（或在但是空）—— 那一行本不该写出来", f)
		}
	}
	if got["event"] != approveAuditEvent {
		t.Errorf("E1 ① 破：事件名 = %v（要 %q）", got["event"], approveAuditEvent)
	}
	if got["signed_payload_sha256"] != sha256Of(payload) {
		t.Errorf("E1 ① 破：`signed_payload_sha256` = %v ≠ 待签字节现算 %s（`D2` 的口径只有一处）",
			got["signed_payload_sha256"], sha256Of(payload))
	}
	if got["approval_path"] != ticket {
		t.Errorf("E1 ① 破：`approval_path` = %v（要指向批准件 %s）", got["approval_path"], ticket)
	}
	if got["approver"] != "Mr2109" {
		t.Errorf("E1 ① 破：`approver` = %v", got["approver"])
	}
	// E2 ① 正控：当场签的 ⇒ 明标 false ⇒ 认成「在场」
	mark, err := approveAuditMarkOf([]byte(lines[0]))
	if err != nil || mark != "在场" {
		t.Errorf("E2 ① 破：明标判定 = (%q, %v)（要「在场」· 因为这一路是人在终端上当场签）", mark, err)
	}
	// E1 ② 静态自检：新字段名与现网六格**不撞**
	for _, own := range approveAuditOwnFields {
		for _, legacy := range approveAuditLegacyFields {
			if own == legacy {
				t.Errorf("E1 ② 破：新字段名 %q 与现网那一格同名（`I-6` 不许蹭 `before_sha256`/`after_sha256` 那一族）", own)
			}
		}
	}
	if !strings.Contains(lines[0], "signed_payload_sha256") || strings.Contains(lines[0], `"before_sha256"`) || strings.Contains(lines[0], `"after_sha256"`) {
		t.Errorf("E1 ② 破：这一行蹭了现网那两格的名字：\n%s", lines[0])
	}
	// E1 ③ 追加只写：再记一次 ⇒ 旧行**逐字节不变**
	if _, err := recordApprovalAudit(tk, payload, ticket); err != nil {
		t.Fatalf("E1 ③ 破：第二次记审计失败：%v", err)
	}
	second := eBatchReadAll(t, auditPath)
	if !strings.HasPrefix(second, first) {
		t.Errorf("E1 ③ 破：第二次之后旧行不再逐字节相同（追加只写破了）\n旧=\n%s\n新=\n%s", first, second)
	}
	if n := len(strings.Split(strings.TrimRight(second, "\n"), "\n")); n != 2 {
		t.Errorf("E1 ③ 破：两次之后行数 = %d（要 2）", n)
	}
	// E1 ① 负控：缺必需格 ⇒ 这一行**不许写**（行数不变）
	broken := approvalAuditLine{At: "2026-09-22T10:00:00+08:00", Event: approveAuditEvent, Tool: "dev_edit",
		Approver: "", ApprovalPath: ticket, SignedPayloadSHA256: sha256Of(payload)}
	if err := appendApprovalAudit(auditPath, broken); err == nil {
		t.Errorf("E1 ① 破：缺 `approver` 竟然写进去了")
	}
	if n := len(strings.Split(strings.TrimRight(eBatchReadAll(t, auditPath), "\n"), "\n")); n != 2 {
		t.Errorf("E1 ① 破：被拒写的那一行还是落了（行数 %d ≠ 2）", n)
	}
	// E2 ① 负控：抹掉明标 ⇒ **判红**（不可分不许蒙过去）
	noMark := []byte(`{"at":"2026-09-22T10:00:00+08:00","event":"approve_signed","tool":"dev_edit","approver":"Mr2109"}`)
	if m, err := approveAuditMarkOf(noMark); err == nil {
		t.Errorf("E2 ① 破：抹掉明标竟然判绿（%q）—— 补签件与在场件就不可分了", m)
	}
	// E2 ① 正控：明标 true ⇒ 「补签」
	if m, err := approveAuditMarkOf([]byte(`{"signed_not_in_person":true}`)); err != nil || m != "补签" {
		t.Errorf("E2 ① 破：明标 true ⇒ (%q, %v)（要「补签」）", m, err)
	}
}

// TestApproveEBatch_E3Archive —— `E3`：归档机制（先复制后删 + 逐字核对 + 确认值不逐字相同就不动作）。
func TestApproveEBatch_E3Archive(t *testing.T) {
	state := eBatchState(t)
	kid := "deadbeefdeadbeef"
	keyBody := `{"alg": "ed25519+pbkdf2-sha256+aes-256-gcm", "salt": "AA==", "iters": 200000, "nonce": "AA==", "ct": "AA==", "pub": "AA==", "created_at": "2026-09-21T20:53:35+08:00", "by": "Mr2109"}` + "\n"
	pubBody := `{"alg": "ed25519", "pub": "AA==", "key_id": "` + kid + `", "at": "2026-09-21T20:53:35+08:00"}` + "\n"
	eBatchWrite(t, operatorKeyPath(), keyBody)
	eBatchWrite(t, operatorPubPath(), pubBody)

	// E3 ③ 归档位必须在状态目录内
	if !strings.HasPrefix(filepath.Clean(archiveDirOf()), filepath.Clean(state)) {
		t.Fatalf("E3 ③ 破：归档位 %s 不在状态目录 %s 里", archiveDirOf(), state)
	}
	// E5 判据② 的机制那一半：确认值与在册 key_id 差一字符 ⇒ **不动作**（原件还在 · 归档位没建）
	bad := kid[:len(kid)-1] + "0"
	if _, _, err := archiveOperatorKeys(bad); err == nil {
		t.Fatalf("E3/E5 破：确认值 %q 与在册 %q 不逐字相同，竟然动作了", bad, kid)
	}
	if _, err := os.Stat(archiveDirOf()); err == nil {
		t.Errorf("E3 破：被拒的那一次还是建了归档位")
	}
	if eBatchReadAll(t, operatorPubPath()) != pubBody || eBatchReadAll(t, operatorKeyPath()) != keyBody {
		t.Fatalf("E3 破：被拒的那一次动了在册两枚件")
	}
	// 正控：确认值逐字相同 ⇒ 归档两枚 + 原件移走（**字节不丢**）
	kArch, pArch, err := archiveOperatorKeys(kid)
	if err != nil {
		t.Fatalf("E3 破：归档失败：%v", err)
	}
	if kArch != filepath.Join(archiveDirOf(), kid+".key") || pArch != filepath.Join(archiveDirOf(), kid+".pub") {
		t.Errorf("E3 破：归档落点 = (%q, %q)（要 `archive/<key_id>.{key,pub}`）", kArch, pArch)
	}
	if eBatchReadAll(t, kArch) != keyBody || eBatchReadAll(t, pArch) != pubBody {
		t.Errorf("E3 ④ 破：归档件与原件**不是逐字相同**（字节丢了）")
	}
	if sha256Of([]byte(eBatchReadAll(t, pArch))) != sha256Of([]byte(pubBody)) {
		t.Errorf("E3 ④ 破：归档件 `sha256` 与原件不一致")
	}
	// 原件被**移**走（不是复制一份在两地）：`§4.2` 逐字「不删，**移到**」
	for _, p := range []string{operatorKeyPath(), operatorPubPath()} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("E3 破：原件 %s 还在（该移走 —— 归档件已在，字节没丢）", p)
		}
	}
}

// TestApproveEBatch_E3OldTicketVoidAfterKeyChange —— `E3` 判据①/② 成对：换钥 ⇒ 旧件不算数；没换 ⇒ 验过。
func TestApproveEBatch_E3OldTicketVoidAfterKeyChange(t *testing.T) {
	state := eBatchState(t)
	pubA, privA, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	kidA := control.KeyID(base64.StdEncoding.EncodeToString(pubA))
	pubB, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	eBatchWrite(t, operatorPubPath(), `{"alg": "ed25519", "pub": "`+base64.StdEncoding.EncodeToString(pubA)+`", "key_id": "`+kidA+`", "at": "2026-09-21T20:53:35+08:00"}`+"\n")

	const tool, approver, at, scope, note = "dev_edit", "Mr2109", "2026-09-21T20:53:35+08:00", "*", "E3 夹具"
	sig := ed25519.Sign(privA, control.ApprovalPayload(tool, scope, approver, at, note))
	body, _ := json.MarshalIndent(map[string]string{
		"tool": tool, "approver": approver, "approved_at": at, "scope": scope, "note": note,
		"sig_alg": "ed25519", "key_id": kidA, "sig": base64.StdEncoding.EncodeToString(sig),
	}, "", " ")
	eBatchWrite(t, filepath.Join(state, "approvals", tool+".json"), string(body)+"\n")

	verdict := func() string {
		rc, out, errb := eBatchRun("approve", "show", tool)
		if rc != 0 {
			t.Fatalf("E3 破：`show` 退码 %d\nstderr=\n%s", rc, errb)
		}
		for _, ln := range strings.Split(out, "\n") {
			if strings.HasPrefix(ln, "判决") {
				return strings.TrimSpace(strings.TrimSpace(strings.TrimPrefix(ln, "判决"))[1:])
			}
		}
		t.Fatalf("E3 破：`show` 没有判决那一行\n%s", out)
		return ""
	}
	// ② 反面（`M-3` 的形态）：在册钥**没换** ⇒ 必须仍判「验过」
	if got := verdict(); got != "验过" {
		t.Errorf("E3 ② 破：在册没换时的判决 = %q（要「验过」）", got)
	}
	// ① 换钥：在册公钥换成 B ⇒ 旧钥签的件**不算批准**（`M-16` ① · `key_id` 改不了 ⇒ 自动不算数）
	eBatchWrite(t, operatorPubPath(), `{"alg": "ed25519", "pub": "`+base64.StdEncoding.EncodeToString(pubB)+`", "key_id": "`+control.KeyID(base64.StdEncoding.EncodeToString(pubB))+`", "at": "2026-09-22T10:00:00+08:00"}`+"\n")
	if got := verdict(); got == "验过" {
		t.Errorf("E3 ① 破：换了钥旧件还判「验过」（那等于换钥不生效）")
	}
}

// TestApproveEBatch_D1FieldFaceStillFrozen —— 冻结面自证：九键一字不动（门⑮ 那条对拍的对象是它）。
func TestApproveEBatch_D1FieldFaceStillFrozen(t *testing.T) {
	nine := []string{"tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "state", "path"}
	if strings.Join(approveFields, ",") != strings.Join(nine, ",") {
		t.Errorf("冻结面被改了：`approveFields` = %v", approveFields)
	}
	if len(approveOptionalFields) == 0 {
		t.Errorf("可选字段面是空的（`D1` 要 `--json` 加可选字段 `strength`）")
	}
	for _, o := range approveOptionalFields {
		for _, n := range nine {
			if o == n {
				t.Errorf("可选字段 %q 同时也在九键里（那会改冻结面）", o)
			}
		}
		if !approveFieldOptional(o) {
			t.Errorf("`approveFieldOptional(%q)` = false（可选字段判定坏了）", o)
		}
	}
	if approveFieldOptional("nosuchfield") {
		t.Errorf("`approveFieldOptional(\"nosuchfield\")` = true（未知字段被当可选字段）")
	}
}

// eBatchWrite —— 写一枚夹具件（本文件在 `package main` 里 ⇒ 自己带一枚，不用 `main_test` 那份）。
func eBatchWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// eBatchReadAll —— 读一个件的全部字节（判「逐字节」用；读不到 ⇒ Fatal）。
func eBatchReadAll(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", p, err)
	}
	return string(b)
}

// eBatchStr —— 把 JSON 里读回来的值转成字符串（判「非空」用）。
func eBatchStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}
