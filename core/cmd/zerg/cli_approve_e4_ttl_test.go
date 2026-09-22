// cli_approve_e4_ttl_test.go —— `E4`（`approve` 族的 `--ttl` 面 + **同批** must-fail）的判据件。
//
// 判据映射（`Zerg-内部文档/项目文档/v2.5.11/任务单-人签实施-20260922.md` §三 `E4` 四条 · 逐条可单独失败）：
//
//	① **缺省不过期**：不给 `--ttl` ⇒ **件与今天逐字节同形态**（`approvalTicketFile` 的字段面仍是
//	   那八格、审计那一行仍写 `E1` 的字段面）· `show` 打「不过期（缺省）」· 判决仍「验过」·
//	   干跑输出里**没有**「有效期」那一行；
//	② **过期即不算批准**：超期件的 `show` 必须打出「**不算批准（已过期）**」（负控：抹掉记录 ⇒ 回
//	   「不过期（缺省）」）；同时 `state` 列那四档**一个不动**（`v1.3 §4.4`：不加「过期」到这一列）；
//	③ **must-fail 同批 + 执行前判**：`--ttl` 认不出 / `0` / 裸给 / 太短 —— 四种都是 **rc=2 ·
//	   stdout 0 字节** · **一个件都不落**（沙箱 approvals 里仍空）；
//	④ **今天那 19 条期望值一条不改**（`M-10`）：矩阵里那两条核心格逐字仍在，新增格只**追加**。
//
// 口径：全部在 `t.TempDir()` 的合成状态目录里跑（真状态目录一个字节不碰 · `ZERG_STATE_DIR` 落点解析
// 把这条钉住）；`--ttl` 的记录面 = **审计那一行**（`E1` 的事件 + 自己的字段名），件字段面**一字不加**。
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"crypto/ed25519"
	"crypto/rand"

	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

// e4SignedTicket —— 造一枚**真签名**批准件 + 在册公钥（都在沙箱状态目录里）。
func e4SignedTicket(t *testing.T, state, tool string) string {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	kid := control.KeyID(pubB64)
	eBatchWrite(t, operatorPubPath(), `{"alg": "ed25519", "pub": "`+pubB64+`", "key_id": "`+kid+`", "at": "2026-09-21T20:53:35+08:00"}`+"\n")
	at := "2026-09-22T10:00:00+08:00"
	payload := control.ApprovalPayload(tool, "*", "Mr2109", at, "E4 夹具")
	tk := approvalTicketFile{
		Tool: tool, Approver: "Mr2109", ApprovedAt: at, Scope: "*", Note: "E4 夹具",
		SigAlg: "ed25519", KeyID: kid,
		Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload)),
	}
	body, err := json.MarshalIndent(tk, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(state, "approvals", tool+".json")
	eBatchWrite(t, p, string(body)+"\n")
	return p
}

// e4LineOf —— 从人面输出里取某一标签那一行（`show` 的九行都是 `标签 : 值`）。
func e4LineOf(out, label string) string {
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, label) {
			return ln
		}
	}
	return ""
}

// e4LineValue —— 取标签那一行的**值**（去掉「标签 + 冒号 + 空白」）。
func e4LineValue(out, label string) string {
	ln := e4LineOf(out, label)
	i := strings.Index(ln, ":")
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(ln[i+1:])
}

// e4AuditLine —— 造一条**记录面**行（`E1` 的事件 + `E4` 两格 · 审计是追加只写的）。
func e4AuditLine(t *testing.T, ticket, expiresAt string, secs int64) string {
	t.Helper()
	line := `{"at":"2026-09-22T10:00:00+08:00","event":"` + approveAuditEvent + `","tool":"dev_edit",` +
		`"approver":"Mr2109","approval_path":"` + ticket + `","signed_payload_sha256":"` + strings.Repeat("a", 64) + `",` +
		`"ttl_seconds":` + strconv.FormatInt(secs, 10) + `,"expires_at":"` + expiresAt + `"}`
	eBatchWrite(t, editAuditPath(), line+"\n")
	return line
}

var (
	e4SummaryWhenRE = regexp.MustCompile(`签的时间 (\S+) ·`)
	e4ExpiryAtRE    = regexp.MustCompile(`^有效期   : 到 (\S+)（`)
)

// approvalTicketJSONTags —— 件结构的 JSON 字段面（**现取**：不写常量名单的副本）。
func approvalTicketJSONTags() []string {
	rt := reflect.TypeOf(approvalTicketFile{})
	out := []string{}
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		out = append(out, strings.Split(tag, ",")[0])
	}
	return out
}

// ── ① 缺省不过期（件与输出都与今天同形态）─────────────────────────────────────────────────

func TestApproveE4_TTLDefaultNoExpiry(t *testing.T) {
	state := eBatchState(t)
	ticket := e4SignedTicket(t, state, "dev_edit")

	// ①-a **件形态**：`approvalTicketFile` 的字段面 = 那八格（**多一格就是改了件字段面** —— 整批红线）
	want := []string{"tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "sig"}
	got := approvalTicketJSONTags()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("E4 ① 破：批准件字段面 = %v（要**逐字**那八格 %v —— 件字段面一字不加 / 不减）", got, want)
	}
	for _, banned := range []string{"ttl", "ttl_seconds", "expires_at"} {
		for _, f := range got {
			if f == banned {
				t.Errorf("E4 ① 破：件里出现了 %q 格（`--ttl` 的记录面**只在审计** ⇒ 件不许动）", banned)
			}
		}
	}

	// ①-b **记录面**：缺省（`ttl = 0`）那一行与 `E1` 的形态相同 —— 两格（`omitempty`）一个都不写
	tk := approvalTicketFile{
		Tool: "dev_edit", Approver: "Mr2109", ApprovedAt: "2026-09-22T10:00:00+08:00",
		Scope: "*", Note: "E4 夹具", SigAlg: "ed25519", KeyID: "0000000000000000",
	}
	payload := control.ApprovalPayload(tk.Tool, tk.Scope, tk.Approver, tk.ApprovedAt, tk.Note)
	p, err := recordApprovalAudit(tk, payload, ticket)
	if err != nil {
		t.Fatalf("E4 ① 破：缺省那一行记不下来：%v", err)
	}
	if p != filepath.Join(state, "edit_audit.jsonl") {
		t.Errorf("E4 ① 破：审计落点 = %q（要沙箱里的那一枚 —— 真状态目录一个字节不碰）", p)
	}
	line := strings.TrimRight(eBatchReadAll(t, p), "\n")
	for _, banned := range []string{"ttl_seconds", "expires_at"} {
		if strings.Contains(line, banned) {
			t.Errorf("E4 ① 破：缺省那一行写了 %q（两格是**可选** ⇒ 不给 `--ttl` 就不写）：\n%s", banned, line)
		}
	}

	// ①-c `show`：新增「有效期」那一行打**不过期（缺省）**，判决仍「验过」（在册钥没换 ⇒ M-3 口径）
	rc, out, errb := eBatchRun("approve", "show", "dev_edit")
	if rc != 0 {
		t.Fatalf("E4 ① 破：`show` 退码 %d\\nstdout=\\n%s\\nstderr=\\n%s", rc, out, errb)
	}
	if v := e4LineValue(out, "有效期"); !strings.Contains(v, "不过期（缺省") {
		t.Errorf("E4 ① 破：「有效期」那一行 = %q（要「不过期（缺省 …）」）", v)
	}
	if v := e4LineValue(out, "判决"); v != "验过" {
		t.Errorf("E4 ① 破：缺省时判决 = %q（要「验过」）", v)
	}
	// ①-d 干跑（不给 `--ttl`）⇒ 输出里**没有**「有效期」那一行 —— 「缺省没被动过」的最小可判面
	rc, out, errb = eBatchRun("approve", "new", "--dry-run", "--tool", "dev_edit", "--by", "Mr2109", "--note", "E4 夹具")
	if rc != 0 {
		t.Fatalf("E4 ① 破：干跑退码 %d\\nstderr=\\n%s", rc, errb)
	}
	if strings.Contains(out, "有效期") {
		t.Errorf("E4 ① 破：不给 `--ttl` 竟然打了「有效期」那一行（缺省该与今天逐字节同形态）：\\n%s", out)
	}
	// ①-e 干跑给 `--ttl 30m` ⇒ 多打那一行，且**到点可复算** = 摘要行里「签的时间」+ 30m
	rc, out, errb = eBatchRun("approve", "new", "--dry-run", "--tool", "dev_edit", "--by", "Mr2109", "--note", "E4 夹具", "--ttl", "30m")
	if rc != 0 {
		t.Fatalf("E4 ① 破：带 `--ttl` 的干跑退码 %d\\nstderr=\\n%s", rc, errb)
	}
	m := e4SummaryWhenRE.FindStringSubmatch(out)
	e := e4ExpiryAtRE.FindStringSubmatch(e4LineOf(out, "有效期"))
	if m == nil || e == nil {
		t.Fatalf("E4 ① 破：摘要 / 有效期两行没都打出来：\\n%s", out)
	}
	at, err := time.Parse(time.RFC3339, m[1])
	if err != nil {
		t.Fatalf("E4 ① 破：摘要里的「签的时间」不是 RFC3339：%q", m[1])
	}
	if want := at.Add(30 * time.Minute).Format(time.RFC3339); e[1] != want {
		t.Errorf("E4 ① 破：有效期到点 = %q（按摘要五格现算 = %q —— 第三方可复算那条口径）", e[1], want)
	}
}

// ── ② 过期即不算批准（且 `state` 列四档不动）──────────────────────────────────────────────

func TestApproveE4_TTLExpiredNotApproved(t *testing.T) {
	state := eBatchState(t)
	ticket := e4SignedTicket(t, state, "dev_edit")

	// 正控①：未过期 —— 打「到 …（剩 …）」·**不含**「不算批准」
	e4AuditLine(t, ticket, "2099-01-01T00:00:00+08:00", 3600)
	rc, out, errb := eBatchRun("approve", "show", "dev_edit")
	if rc != 0 {
		t.Fatalf("E4 ② 破：`show` 退码 %d\\nstderr=\\n%s", rc, errb)
	}
	if v := e4LineValue(out, "有效期"); !strings.Contains(v, "到 2099-01-01T00:00:00+08:00") || strings.Contains(v, "不算批准") {
		t.Errorf("E4 ② 破：未过期件的「有效期」行 = %q", v)
	}

	// 正控②：超期件（记录面那一行的人为夹具 · `expires_at` 在过去）⇒ **不算批准（已过期）**
	e4AuditLine(t, ticket, "2026-09-22T10:01:00+08:00", 60)
	rc, out, errb = eBatchRun("approve", "show", "dev_edit")
	if rc != 0 {
		t.Fatalf("E4 ② 破：`show` 退码 %d\\nstderr=\\n%s", rc, errb)
	}
	if v := e4LineValue(out, "有效期"); !strings.Contains(v, "不算批准（已过期）") {
		t.Errorf("E4 ② 破：超期件的「有效期」行 = %q（要「**不算批准（已过期）**」）", v)
	}
	// ★ `state` 列那四档**一个不动**：超期只进「有效期」这一行（`§4.4`：不加「过期」到这一列）
	if v := e4LineValue(out, "判决"); v != "验过" {
		t.Errorf("E4 ② 破：超期改了判决列（%q ⇒ 判决列四档不许扩、超期只走「有效期」那一行）", v)
	}
	// 机器面三格：`expires_at` 逐字 + `expired`=true（与那一行**同一枚**判定口 ⇒ 两处不漂）
	rc, out, errb = eBatchRun("approve", "show", "dev_edit", "--json", "expires_at,expired,ttl_seconds")
	if rc != 0 {
		t.Fatalf("E4 ② 破：`--json` 三格取不出来（退码 %d）\\nstderr=\\n%s", rc, errb)
	}
	if !strings.Contains(out, "2026-09-22T10:01:00+08:00") || !strings.Contains(out, `"expired":"true"`) || !strings.Contains(out, `"ttl_seconds":"60"`) {
		t.Errorf("E4 ② 破：`--json` 三格 = %s（要 `expires_at` 逐字 + `expired=true` + `ttl_seconds=60`）", out)
	}

	// 负控：**抹掉记录面**（审计清空）⇒ 回「不过期（缺省）」·**不再**判「不算批准」
	// （证明这一条判据真的在读记录面，而不是恒打一句话）
	eBatchWrite(t, editAuditPath(), "")
	rc, out, _ = eBatchRun("approve", "show", "dev_edit")
	if rc != 0 {
		t.Fatalf("E4 ② 破：负控那一态退码 %d", rc)
	}
	if v := e4LineValue(out, "有效期"); !strings.Contains(v, "不过期（缺省") || strings.Contains(v, "不算批准") {
		t.Errorf("E4 ② 破：抹掉记录后「有效期」行 = %q（要回「不过期（缺省 …）」）", v)
	}
	// 记录面口径只有一处：`approveTTLRecord` 与那一行 / 那三格同源
	if _, _, ok := approveTTLRecord(ticket); ok {
		t.Errorf("E4 ② 破：审计清空后 `approveTTLRecord` 仍说「有记录」")
	}
}

// ── ③ must-fail 同批 · 执行前判（rc=2 · stdout 0 字节 · 一个件都不落）─────────────────────────

func TestApproveE4_TTLBadValuesBeforeAnyWrite(t *testing.T) {
	state := eBatchState(t)
	approvals := filepath.Join(state, "approvals")
	bad := [][]string{
		{"approve", "new", "--tool", "dev_edit", "--by", "Mr2109", "--note", "E4 夹具", "--ttl", "等下"},
		{"approve", "new", "--tool", "dev_edit", "--by", "Mr2109", "--note", "E4 夹具", "--ttl", "0"},
		{"approve", "new", "--tool", "dev_edit", "--by", "Mr2109", "--note", "E4 夹具", "--ttl", "500ms"},
		{"approve", "new", "--tool", "dev_edit", "--by", "Mr2109", "--note", "E4 夹具", "--ttl=-30m"},
		{"approve", "new", "--tool", "dev_edit", "--by", "Mr2109", "--note", "E4 夹具", "--ttl"},
	}
	for _, argv := range bad {
		rc, out, errb := eBatchRun(argv...)
		if rc != 2 {
			t.Errorf("E4 ③ 破：%v 退码 %d（要 2 —— 用法错 · 执行前判）\\nstderr=\\n%s", argv, rc, errb)
		}
		if out != "" {
			t.Errorf("E4 ③ 破：%v 往 stdout 写了 %q（用法错那一档 stdout 必须 0 字节）", argv, out)
		}
		if !strings.Contains(errb, "--ttl") {
			t.Errorf("E4 ③ 破：%v 的拒因没点名 `--ttl`：\\n%s", argv, errb)
		}
		if ents, err := os.ReadDir(approvals); err == nil && len(ents) != 0 {
			t.Errorf("E4 ③ 破：%v 落了件（执行前判 ⇒ 一个件都不许落）：%v", argv, ents)
		}
	}
	// 正控：**合法**时长 ⇒ 不是用法错（干跑 ⇒ 0 · 零副作用）；非交互会话仍拒（但拒因是「人在终端上敲」）
	rc, _, errb := eBatchRun("approve", "new", "--dry-run", "--tool", "dev_edit", "--by", "Mr2109", "--note", "E4 夹具", "--ttl", "7d")
	if rc != 0 {
		t.Errorf("E4 ③ 破：`--ttl 7d` 的干跑退码 %d（要 0）\\nstderr=\\n%s", rc, errb)
	}
	rc, _, errb = eBatchRun("approve", "new", "--tool", "dev_edit", "--by", "Mr2109", "--note", "E4 夹具", "--ttl", "7d")
	if rc != 2 || !strings.Contains(errb, "终端") {
		t.Errorf("E4 ③ 破：非交互会话那一格 = (rc=%d, stderr=%q)（要 2 + 拒因是「人在终端上敲」）", rc, errb)
	}
	// 时长口径自证（纯函数面）：三档认得出，四种认不出
	for _, ok := range []string{"30m", "72h", "1h30m", "7d", "1s"} {
		if _, err := parseApproveTTL(ok); err != nil {
			t.Errorf("E4 ③ 破：`%s` 应该认得出，实际：%v", ok, err)
		}
	}
	for _, no := range []string{"", "等下", "0", "-30m", "500ms", "7", "d"} {
		if _, err := parseApproveTTL(no); err == nil {
			t.Errorf("E4 ③ 破：`%q` 应该认不出（**不许猜**），竟然过了", no)
		}
	}
}

// ── ④ must-fail 同批进矩阵 + 今天那 19 条期望值一条不改（M-10）─────────────────────────────

func TestApproveE4_TTLMatrixCasesSameBatch(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "cli-matrix.json"))
	if err != nil {
		t.Fatalf("E4 ④ 破：矩阵读不到：%v", err)
	}
	// 本文件在 `package main` 里（`matrixCase` 那枚在外部测试包）⇒ 这里自己取那一份要判的列。
	type e4Case struct {
		ID              string   `json:"id"`
		Command         string   `json:"command"`
		Argv            []string `json:"argv"`
		WantRC          int      `json:"want_rc"`
		WantStdoutBytes int      `json:"want_stdout_bytes"`
	}
	var m struct {
		Cases []e4Case `json:"cases"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("E4 ④ 破：矩阵解不动：%v", err)
	}
	byID := map[string]e4Case{}
	fam, famMustFail := 0, 0
	for _, c := range m.Cases {
		byID[c.ID] = c
		if strings.HasPrefix(c.Command, "approve") {
			fam++
			if c.WantRC != 0 {
				famMustFail++
			}
		}
	}
	// ④-a 今天那两条核心格**逐字仍在**（`M-10` · 期望值一条不改）
	if c, ok := byID["approve/new#非终端不给签（模型路径）"]; !ok {
		t.Errorf("E4 ④ 破：矩阵里少了两条核心格之一（`approve/new#非终端不给签（模型路径）`）—— 期望值一条不许改")
	} else if c.WantRC != 2 || c.WantStdoutBytes != 0 {
		t.Errorf("E4 ④ 破：非真终端那一格的期望值被改了（rc=%d · stdout=%d 字节 ⇒ 要 2 / 0）", c.WantRC, c.WantStdoutBytes)
	}
	if c, ok := byID["approve/new#--confirm 值不匹配"]; !ok || c.WantRC != 2 || c.WantStdoutBytes != 0 {
		t.Errorf("E4 ④ 破：确认值不匹配那一格的期望值被改了（%+v ⇒ 要 rc=2 · stdout 0 字节）", c)
	}
	// ④-b 新旗标的 must-fail **同批**进了矩阵（`P12` 铁律：不等实现完再补）
	for _, id := range []string{
		"approve/new#--ttl 认不出的时长（执行前判）",
		"approve/new#--ttl 0 不是不过期",
		"approve/new#--ttl 给了旗标没给时长",
	} {
		c, ok := byID[id]
		if !ok {
			t.Errorf("E4 ④ 破：新旗标那一格没同批进矩阵：%s", id)
			continue
		}
		if c.WantRC != 2 || c.WantStdoutBytes != 0 {
			t.Errorf("E4 ④ 破：%s 的期望值 = (rc=%d · stdout=%d)（要 2 / 0）", id, c.WantRC, c.WantStdoutBytes)
		}
		rc, out, _ := eBatchRun(c.Argv...)
		if rc != c.WantRC || len(out) != c.WantStdoutBytes {
			t.Errorf("E4 ④ 破：%s 现跑 = (rc=%d · stdout=%d 字节) ≠ 矩阵 (%d / %d)", id, rc, len(out), c.WantRC, c.WantStdoutBytes)
		}
	}
	// ④-c 全族仍**全是** must-fail（门⑮ `F3` 的口径：条数 == must-fail 条数）
	if fam != famMustFail {
		t.Errorf("E4 ④ 破：`approve` 族 %d 条里有 %d 条不是 must-fail（只许追加 must-fail 格）", fam, fam-famMustFail)
	}
	if fam < 19 {
		t.Errorf("E4 ④ 破：`approve` 族只剩 %d 条（今天那 19 条不许删）", fam)
	}
	// 基线只随动计数：矩阵的 must-fail 条数与基线记的数一致（现算 ⇒ 不写常量）
	braw, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "gates", "cli-contract-baseline.json"))
	if err != nil {
		t.Fatalf("E4 ④ 破：基线读不到：%v", err)
	}
	var b struct {
		MustfailCases int `json:"mustfail_cases"`
	}
	if err := json.Unmarshal(braw, &b); err != nil {
		t.Fatalf("E4 ④ 破：基线解不动：%v", err)
	}
	n := 0
	for _, c := range m.Cases {
		if c.WantRC != 0 {
			n++
		}
	}
	if b.MustfailCases != n {
		t.Errorf("E4 ④ 破：基线记 %d 格 must-fail，矩阵现算 %d 格（基线只许**随动计数**）", b.MustfailCases, n)
	}
}
