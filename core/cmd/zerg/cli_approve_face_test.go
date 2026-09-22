// cli_approve_face_test.go —— 人签**看得见**的两面（D 批）：`D2` 签前摘要两行 · `D3` 钥匙身份行。
//
// 本件判据（全部在 `t.TempDir()` 的合成状态目录里跑 · 真状态目录一个字节不碰）：
//
//	D2 ① 两行都在，且第二行的 `sha256` **逐字等于** `control.ApprovalPayload(五格)` 的 `sha256`
//	     （五格从**打出来的那一行摘要**里读回 ⇒ 第三方拿同一组五格现算即可复算）；
//	D2 ② **负控**：拿**件文件的** `sha256` / **被批准文件的** `sha256` 冒充 ⇒ 判据必须红；
//	D2 ③ 干跑零副作用：不落批准件 · 不落审计 · 被批准文件字节不变；
//	D2 ④ 打印**不许只打文件名**：第二行必须是 64 位十六进制的内容面摘要，第一行带全五格；
//	D3 ① 件里 `key_id` 与在册公钥一致 ⇒ 「钥匙身份」那行打「**在册**」；
//	D3 ② **负控**：件里 `key_id` 与在册**差一字符** ⇒ 打「**不在册**」，且**不许**因此改判决列；
//	D3 ③ 判决列仍是那四档（本件钉住「只显示、不拦」这一条：不在册也照退 0、判决照打）。
package main_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func sha256FileHex(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读夹具 %s 失败：%v", p, err)
	}
	return sha256Hex(b)
}

func fileExistsAt(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// parseApproveSummaryLine 从「摘要」那一行读回**待签五格**（标签顺序 = `ApprovalPayload` 的形参顺序）。
func parseApproveSummaryLine(line string) (map[string]string, bool) {
	parts := strings.Split(line, " · ")
	if len(parts) != 5 {
		return nil, false
	}
	labels := map[string]string{"工具": "tool", "范围": "scope", "批准者": "approver", "签的时间": "approved_at", "理由": "note"}
	out := map[string]string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if i := strings.Index(p, ":"); i >= 0 && strings.Contains(p[:i], "摘要") {
			p = strings.TrimSpace(p[i+1:])
		}
		sp := strings.Index(p, " ")
		if sp < 0 {
			return nil, false
		}
		key, ok := labels[p[:sp]]
		if !ok {
			return nil, false
		}
		out[key] = strings.TrimSpace(p[sp+1:])
	}
	return out, true
}

var digestLineRE = regexp.MustCompile(`(?m)^待签 sha256: ([0-9a-f]{64})（`)

// TestApproveNewDryRunPrintsPayloadDigest —— `D2`：签前摘要两行（干跑那一态，零副作用）。
func TestApproveNewDryRunPrintsPayloadDigest(t *testing.T) {
	// 夹具①（testdata）：五格 + 现算值 —— 先把**待签字节的形态**钉住（第三方复算口径）
	raw, err := os.ReadFile(filepath.Join("testdata", "approve-new-digest.json"))
	if err != nil {
		t.Fatalf("复算夹具读不到：%v", err)
	}
	var fx struct {
		Cells  map[string]string `json:"cells"`
		SHA256 string            `json:"sha256"`
		Bytes  int               `json:"payload_bytes"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("复算夹具解不动：%v", err)
	}
	fxPayload := control.ApprovalPayload(fx.Cells["tool"], fx.Cells["scope"], fx.Cells["approver"], fx.Cells["approved_at"], fx.Cells["note"])
	if got := sha256Hex(fxPayload); got != fx.SHA256 {
		t.Errorf("D2 判据① 破：夹具五格现算 = %s，夹具记的 = %s", got, fx.SHA256)
	}
	if len(fxPayload) != fx.Bytes {
		t.Errorf("D2 判据① 破：待签字节长度 %d ≠ 夹具记的 %d", len(fxPayload), fx.Bytes)
	}

	// 夹具②：干跑那一态
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	target := filepath.Join(state, "被批准文件.txt")
	mustWrite(t, target, "D2 负控夹具：一枚**被批准的文件**（它的 sha256 不许被当成待签字节的 sha256）\n")
	beforeTarget := sha256FileHex(t, target)

	rc, out, errb := runCapture("approve", "new", "--dry-run", "--tool", "dev_edit", "--by", "Mr2109", "--note", "夹具理由")
	if rc != 0 {
		t.Fatalf("D2 判据③ 破：干跑退码 %d（要 0）\nstdout=\n%s\nstderr=\n%s", rc, out, errb)
	}
	summary, digest := "", ""
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "摘要") {
			summary = ln
		}
		if m := digestLineRE.FindStringSubmatch(ln); m != nil {
			digest = m[1]
		}
	}
	if summary == "" || digest == "" {
		t.Fatalf("D2 判据① 破：两行没都打出来。stdout=\n%s\nstderr=\n%s", out, errb)
	}
	cells, ok := parseApproveSummaryLine(summary)
	if !ok {
		t.Fatalf("D2 判据④ 破：摘要行里读不回五格：%q", summary)
	}
	want := sha256Hex(control.ApprovalPayload(cells["tool"], cells["scope"], cells["approver"], cells["approved_at"], cells["note"]))
	if digest != want {
		t.Errorf("D2 判据① 破：打印的 sha256 = %s，同一组五格现算 = %s（摘要行 %q）", digest, want, summary)
	}
	// 判据②：负控 —— 拿别的对象的 sha256 冒充 ⇒ 判据必须红（这里钉的是「冒充值 ≠ 打印值」这条区分度）
	if digest == sha256FileHex(t, target) {
		t.Errorf("D2 判据② 破：打印的 sha256 与被批准文件的 sha256 相同（打错了对象）")
	}
	ticket := filepath.Join(state, "approvals", "dev_edit.json")
	if fileExistsAt(ticket) && digest == sha256FileHex(t, ticket) {
		t.Errorf("D2 判据② 破：打印的 sha256 与件文件的 sha256 相同（打错了对象）")
	}
	// 判据④：不许只打文件名 —— 第二行必须是 64 位十六进制的内容面摘要（判据用正则已钉住形态）
	if len(digest) != 64 {
		t.Errorf("D2 判据④ 破：摘要不是 64 位十六进制：%q", digest)
	}
	// 判据③：零副作用
	if fileExistsAt(ticket) {
		t.Errorf("D2 判据③ 破：干跑落了批准件 %s", ticket)
	}
	if fileExistsAt(filepath.Join(state, "edit_audit.jsonl")) {
		t.Errorf("D2 判据③ 破：干跑落了审计件")
	}
	if sha256FileHex(t, target) != beforeTarget {
		t.Errorf("D2 判据③ 破：被批准文件字节变了")
	}
}

// TestApproveShowKeyIdentityLine —— `D3`：「钥匙身份」行（只显示、不拦）。
func TestApproveShowKeyIdentityLine(t *testing.T) {
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	dir := filepath.Join(state, "approvals")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	kid := control.KeyID(pubB64)
	mustWrite(t, filepath.Join(dir, "operator.pub"),
		`{"alg": "ed25519", "pub": "`+pubB64+`", "key_id": "`+kid+`", "at": "2026-09-22T10:00:00+08:00"}`+"\n")

	writeTicket := func(name, keyID string) string {
		const tool, approver, at, scope, note = "合成件", "Mr2109", "2026-09-22T10:00:00+08:00", "*", "D3 夹具"
		sig := ed25519.Sign(priv, control.ApprovalPayload(tool, scope, approver, at, note))
		body, _ := json.MarshalIndent(map[string]string{
			"tool": tool, "approver": approver, "approved_at": at, "scope": scope, "note": note,
			"sig_alg": "ed25519", "key_id": keyID, "sig": base64.StdEncoding.EncodeToString(sig),
		}, "", " ")
		p := filepath.Join(dir, name+".json")
		mustWrite(t, p, string(body)+"\n")
		return p
	}
	writeTicket("合成件", kid)

	verdictOf := func(out string) (string, string) {
		key, jd := "", ""
		for _, ln := range strings.Split(out, "\n") {
			if strings.HasPrefix(ln, "钥匙身份") {
				key = ln
			}
			if strings.HasPrefix(ln, "判决") {
				jd = strings.TrimSpace(strings.TrimPrefix(ln, "判决"))
				jd = strings.TrimSpace(strings.TrimPrefix(jd, ":"))
			}
		}
		return key, jd
	}

	// 正控：与在册逐字一致 ⇒ 「在册」
	rc, out, errb := runCapture("approve", "show", "合成件")
	if rc != 0 {
		t.Fatalf("D3 判据① 破：退码 %d\nstdout=\n%s\nstderr=\n%s", rc, out, errb)
	}
	key, jd := verdictOf(out)
	if !strings.Contains(key, kid) || !strings.Contains(key, "在册") || strings.Contains(key, "不在册") {
		t.Errorf("D3 判据① 破：钥匙身份行 = %q（要含 %s + 「在册」）", key, kid)
	}
	if jd != "验过" {
		t.Errorf("D3 判据①/④ 破：判决 = %q（真签名件要「验过」）", jd)
	}

	// 负控：`key_id` 差一字符（签名本身仍然验得过 —— `key_id` 不在待签字节里）
	kid2 := kid[:len(kid)-1] + map[bool]string{true: "0", false: "1"}[kid[len(kid)-1] != '0']
	writeTicket("合成件2", kid2)
	rc2, out2, errb2 := runCapture("approve", "show", "合成件2")
	if rc2 != 0 {
		t.Fatalf("D3 判据③ 破：不在册就不给结论了（退码 %d）—— 这一行**只显示、不拦**\nstdout=\n%s\nstderr=\n%s", rc2, out2, errb2)
	}
	key2, jd2 := verdictOf(out2)
	if !strings.Contains(key2, "不在册") {
		t.Errorf("D3 判据② 破：钥匙身份行 = %q（要含「不在册」）", key2)
	}
	if jd2 != "验过" {
		t.Errorf("D3 判据② 破：判决列因 key_id 不一致变了（%q ⇒ 要仍是「验过」· 判决列四档不许扩、也不许因这一行收紧）", jd2)
	}
	// 判据④：M-3 的同类回归在真件上跑（这里钉住合成件那一路的判决取值仍在四档闭集里）
	allowed := map[string]bool{"验过": true, "无签名（不算批准）": true}
	if !strings.HasPrefix(jd2, "签名坏") && !allowed[jd2] {
		t.Errorf("D3 判据③ 破：判决取值 %q 不在既有的判决档里", jd2)
	}
	// 静态面：这一行只读在册件、不动件一个字节
	for _, p := range []string{filepath.Join(dir, "operator.pub"), filepath.Join(dir, "合成件.json"), filepath.Join(dir, "合成件2.json")} {
		if !fileExistsAt(p) {
			t.Errorf("D3 破：只读面动了件（%s 不见了）", p)
		}
	}
}
