// cli_approve_e5_rekey_test.go —— `E5`（**换钥确认对** · `--confirm=<旧 key_id>` + `--yes`）的判据件。
//
// 判据映射（`Zerg-内部文档/项目文档/v2.5.11/任务单-人签实施-20260922.md` §三 `E5` 五条 · 逐条可单独失败）：
//
//	① **确认对必填**（在册已有钥 ⇒ 两枚同时到；不逐字相同 ⇒ 拒）：纯函数真值表 + 命令行两态，
//	   且拒绝点必须在**读口令 / 终端判据之前**（执行前判：stderr 不提「终端」、不许落件）；
//	② **`--yes` 必填**：只给 `--confirm` ⇒ 拒（不能靠「值给对了」就少一枚）；
//	③ **备份先行**：归档那一步不过 ⇒ **不动作**（在册两枚件的字节一个不动）；
//	④ **先自检后不可逆**：自检失败 ⇒ **不写** `operator.pub`、**连归档都不做**（顺序铁律机器可判）；
//	⑤ 换钥正路：旧钥进归档（字节逐字与换前相同）· 新钥在册 · 旧钥签的件掉档、新钥签的件「验过」。
//
// 口径：全在 `t.TempDir()` 的沙箱状态目录里跑（真状态目录一个字节不碰）；私钥只在内存里过一手，
// 断言里不打印任何口令 / 私钥 / 密文（只说「字节相同 / 不同」）。
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/control"
)

// e5Keys —— 沙箱里落一对**真**钥（同一个生成口）⇒ 在册有钥那一态。
func e5Keys(t *testing.T, pass string) (ed25519.PrivateKey, string) {
	t.Helper()
	kf, pf, priv, err := newOperatorKeyFull(pass, "Mr2109")
	if err != nil {
		t.Fatalf("E5 夹具：生成口失败：%v", err)
	}
	if err := writeOperatorKeys(kf, pf); err != nil {
		t.Fatalf("E5 夹具：落件失败：%v", err)
	}
	return priv, pf.KeyID
}

// e5Piece —— 用给定私钥造一枚件（换钥前后各造一枚，看判决怎么掉/怎么立）。
func e5Piece(t *testing.T, state, tool string, priv ed25519.PrivateKey, keyID string) string {
	t.Helper()
	at := "2026-09-23T01:00:00+08:00"
	payload := control.ApprovalPayload(tool, "*", "Mr2109", at, "E5 夹具")
	tk := approvalTicketFile{
		Tool: tool, Approver: "Mr2109", ApprovedAt: at, Scope: "*", Note: "E5 夹具",
		SigAlg: "ed25519", KeyID: keyID,
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

// ── ①② 确认对真值表（口径只有一处：`operatorRekeyConfirmPair`）──────────────────────────────

func TestApproveE5_ConfirmPairTruthTable(t *testing.T) {
	inUse := "abcdef0123456789"
	cases := []struct {
		name                string
		hasKey              bool
		confirmGiven, yes   bool
		confirm             string
		wantErr, wantSubstr string
	}{
		{"第一次生成（在册没钥 · 两枚都不给）", false, false, false, "", "", ""},
		{"在册没钥却给了 --confirm", false, true, false, "abcdef0123456789", "没", "换钥"},
		{"在册没钥却给了 --yes", false, false, true, "", "没", "换钥"},
		{"在册有钥 · 两枚都不给", true, false, false, "", "有", "--confirm"},
		{"在册有钥 · 只给 --confirm", true, true, false, inUse, "有", "--yes"},
		{"在册有钥 · 只给 --yes", true, false, true, "", "有", "--confirm"},
		{"在册有钥 · 两枚都给但值不逐字相同", true, true, true, "不匹配-zz", "有", "不逐字相同"},
		{"在册有钥 · 两枚都给且值逐字相同", true, true, true, inUse, "", ""},
	}
	for _, c := range cases {
		inv := &invocation{confirmGiven: c.confirmGiven, confirm: c.confirm, yes: c.yes}
		err := operatorRekeyConfirmPair(inv, inUse, c.hasKey)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("E5 ①② 破：%s ⇒ 应该放行，实得 %v", c.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("E5 ①② 破：%s ⇒ 应该拒，实得放行", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSubstr) {
			t.Errorf("E5 ①② 破：%s ⇒ 拒因 %q 里没有 %q", c.name, err.Error(), c.wantSubstr)
		}
	}
}

// ── ①②（命令行面 · 执行前判：拒在终端判据 / 读口令**之前**）────────────────────────────────

func TestApproveE5_ConfirmPairBeforeAnyPrompt(t *testing.T) {
	state := eBatchState(t)
	e5Keys(t, "correct-horse-battery") // 在册有钥
	approvals := filepath.Join(state, "approvals")
	before, err := os.ReadDir(approvals)
	if err != nil {
		t.Fatal(err)
	}
	bad := [][]string{
		{"approve", "keygen", "--by", "张三"},                               // 缺确认对
		{"approve", "keygen", "--by", "张三", "--yes"},                      // 只给 --yes
		{"approve", "keygen", "--by", "张三", "--confirm=0000000000000000"}, // 只给 --confirm
		{"approve", "keygen", "--by", "张三", "--confirm=不匹配-zz", "--yes"},  // 值不逐字相同
		{"approve", "keygen", "--by", "张三", "--rekey", "--yes"},           // §五⑩ 预留名不许占
	}
	for _, argv := range bad {
		rc, out, errb := eBatchRun(argv...)
		if rc != 2 {
			t.Errorf("E5 ①② 破：%v 退码 %d（要 2）\\nstderr=\\n%s", argv, rc, errb)
		}
		if out != "" {
			t.Errorf("E5 ①② 破：%v 往 stdout 写了 %q（这一档必须 0 字节）", argv, out)
		}
		// **执行前判**：拒因不许是「要人在终端上敲」那一档（那就说明它跑到了终端/口令那一步）
		if strings.Contains(errb, "终端") {
			t.Errorf("E5 ①② 破：%v 拒在「读口令 / 终端判据」之后了（执行前判这一条破）：\\n%s", argv, errb)
		}
	}
	after, err := os.ReadDir(approvals)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("E5 ①② 破：被拒的那几格竟然动了 approvals（%d ⇒ %d 项）", len(before), len(after))
	}
	// 负控：把确认对给全 ⇒ 才允许走到**下一格**（终端判据 · 非交互会话 ⇒ 仍是 2，但拒因换成了「终端」）
	kid, ok := operatorKeyIDInUse()
	if !ok {
		t.Fatal("E5 夹具：在册公钥件没读到 key_id")
	}
	rc, out, errb := eBatchRun("approve", "keygen", "--by", "张三", "--confirm="+kid, "--yes")
	if rc != 2 || !strings.Contains(errb, "终端") || out != "" {
		t.Errorf("E5 ①② 破：确认对给全后应落到「非交互 ⇒ 要人在终端上敲」那一格（rc=%d · stdout=%q）\\nstderr=\\n%s", rc, out, errb)
	}
}

// ── ③ 备份先行（归档不过 ⇒ 不动作）────────────────────────────────────────────────────────

func TestApproveE5_BackupFirstBlocksOnArchive(t *testing.T) {
	state := eBatchState(t)
	pass := "correct-horse-battery"
	_, _ = e5Keys(t, pass)
	// 在册两枚件的**换前字节**（断言的锚）
	keyBefore := eBatchReadAll(t, operatorKeyPath())
	pubBefore := eBatchReadAll(t, operatorPubPath())
	if len(keyBefore) == 0 || len(pubBefore) == 0 {
		t.Fatal("E5 ③ 夹具：在册两枚件读不到")
	}
	// 让**归档那一步**必失败：`archive` 这个位置被一个**普通文件**占了（`MkdirAll` 必报 not a directory）
	if err := os.MkdirAll(filepath.Join(state, "approvals"), 0o700); err != nil {
		t.Fatal(err)
	}
	eBatchWrite(t, filepath.Join(state, "approvals", "archive"), "占位\n")

	inUse, _ := operatorKeyIDInUse()
	kf, pf, priv, err := newOperatorKeyFull(pass, "Mr2109")
	if err != nil {
		t.Fatal(err)
	}
	_, _, rerr := rekeyOperatorKey(kf, pf, priv, inUse, nil)
	if rerr == nil {
		t.Fatalf("E5 ③ 破：归档写不进去竟然还给换了钥")
	}
	if !strings.Contains(rerr.Error(), "备份先行") {
		t.Errorf("E5 ③ 破：失败不是因为「备份先行」那一格：%v", rerr)
	}
	// **不动作**：在册两枚件的字节**一个不动**
	if got := eBatchReadAll(t, operatorKeyPath()); got != keyBefore {
		t.Errorf("E5 ③ 破：备份先行没过却动了私钥件（字节变了）")
	}
	if got := eBatchReadAll(t, operatorPubPath()); got != pubBefore {
		t.Errorf("E5 ③ 破：备份先行没过却动了公钥件（字节变了）")
	}
}

// ── ④ 先自检后不可逆（自检失败 ⇒ 不写 `operator.pub`、连归档都不做）──────────────────────────

func TestApproveE5_SelfCheckBeforeIrreversible(t *testing.T) {
	_ = eBatchState(t)
	pass := "correct-horse-battery"
	_, _ = e5Keys(t, pass)
	pubBefore := eBatchReadAll(t, operatorPubPath())
	if pubBefore == "" {
		t.Fatal("E5 ④ 夹具：在册公钥件读不到")
	}
	inUse, _ := operatorKeyIDInUse()
	kf, pf, priv, err := newOperatorKeyFull(pass, "Mr2109")
	if err != nil {
		t.Fatal(err)
	}
	// 注入一个**必失败**的自检口（这一格可注入才有负控 ⇒ 才证得出「自检在不可逆之前」）
	boom := errors.New("注入的假自检失败")
	_, _, rerr := rekeyOperatorKey(kf, pf, priv, inUse, func(string, ed25519.PrivateKey) error { return boom })
	if rerr == nil || !strings.Contains(rerr.Error(), "自检没过") {
		t.Fatalf("E5 ④ 破：自检失败竟然没拦住（err=%v）", rerr)
	}
	if got := eBatchReadAll(t, operatorPubPath()); got != pubBefore {
		t.Errorf("E5 ④ 破：自检没过却改了 `operator.pub`（字节变了）")
	}
	if _, err := os.Stat(archiveDirOf()); err == nil {
		t.Errorf("E5 ④ 破：自检没过却已经建了归档位（%s）—— 顺序铁律「先自检 → 备份先行 → 才写」破", archiveDirOf())
	}

	// **顺序铁律机器可判**：`rekeyOperatorKey` 的源码里三步的出现次序不许换
	src, err := os.ReadFile("family_approve.go")
	if err != nil {
		t.Fatalf("E5 ④ 破：读不到源码：%v", err)
	}
	body := string(src)
	i := strings.Index(body, "func rekeyOperatorKey(")
	if i < 0 {
		t.Fatal("E5 ④ 破：找不到 `rekeyOperatorKey`")
	}
	body = body[i:]
	iSelf := strings.Index(body, "selfCheck(pf.Pub, priv)")
	iArch := strings.Index(body, "archiveOperatorKeys(oldKeyID)")
	iWrite := strings.Index(body, "writeOperatorKeys(kf, pf)")
	if iSelf < 0 || iArch < 0 || iWrite < 0 {
		t.Fatalf("E5 ④ 破：三步在源码里没找齐（self=%d arch=%d write=%d）", iSelf, iArch, iWrite)
	}
	if !(iSelf < iArch && iArch < iWrite) {
		t.Errorf("E5 ④ 破：三步次序 = 自检@%d / 归档@%d / 写@%d（要「先自检 → 备份先行 → 才写」）", iSelf, iArch, iWrite)
	}
	// 而默认真自检本身要能过（否则正路根本走不通）
	if err := operatorRekeySelfCheck(pf.Pub, priv); err != nil {
		t.Errorf("E5 ④ 破：真自检竟然不过：%v", err)
	}
}

// ── ⑤ 换钥正路：旧钥进归档 · 旧件掉档 · 新件「验过」─────────────────────────────────────────

func TestApproveE5_RekeyPathEndToEnd(t *testing.T) {
	state := eBatchState(t)
	pass := "correct-horse-battery"
	oldPriv, oldKid := e5Keys(t, pass)
	oldPiece := e5Piece(t, state, "dev_edit", oldPriv, oldKid)

	rc, out, errb := eBatchRun("approve", "show", "dev_edit")
	if rc != 0 || e4LineValue(out, "判决") != "验过" {
		t.Fatalf("E5 ⑤ 夹具：换钥前那枚件应是「验过」（rc=%d）\\nstdout=\\n%s\\nstderr=\\n%s", rc, out, errb)
	}

	keyBefore := eBatchReadAll(t, operatorKeyPath())
	pubBefore := eBatchReadAll(t, operatorPubPath())
	kf, pf, newPriv, err := newOperatorKeyFull(pass, "Mr2109")
	if err != nil {
		t.Fatal(err)
	}
	kArch, pArch, err := rekeyOperatorKey(kf, pf, newPriv, oldKid, nil)
	if err != nil {
		t.Fatalf("E5 ⑤ 破：正路换钥失败：%v", err)
	}
	// 归档两枚 = 换前字节（`E3` 的「先复制 + 逐字核对 `sha256` + 才移走」⇒ 字节必须逐字相同）
	if got := eBatchReadAll(t, kArch); got != keyBefore {
		t.Errorf("E5 ⑤ 破：归档的私钥件与换前字节不一致")
	}
	if got := eBatchReadAll(t, pArch); got != pubBefore {
		t.Errorf("E5 ⑤ 破：归档的公钥件与换前字节不一致")
	}
	if !strings.Contains(kArch, oldKid+".key") || !strings.Contains(pArch, oldKid+".pub") {
		t.Errorf("E5 ⑤ 破：归档落点没带旧 `key_id`：%s / %s", kArch, pArch)
	}
	// 在册公钥件已换（新 `key_id`）
	inUse, ok := operatorKeyIDInUse()
	if !ok || inUse != pf.KeyID || inUse == oldKid {
		t.Errorf("E5 ⑤ 破：在册 `key_id` = %q（要新钥 %q · 旧值 %q 不该还在）", inUse, pf.KeyID, oldKid)
	}
	// 旧件掉档（`E3` ①：签字钥不在册 ⇒ 判决掉出「验过」）
	rc, out, _ = eBatchRun("approve", "show", "dev_edit")
	if rc != 0 {
		t.Fatalf("E5 ⑤ 破：`show` 退码 %d", rc)
	}
	if v := e4LineValue(out, "判决"); v == "验过" {
		t.Errorf("E5 ⑤ 破：旧钥签的件换钥后仍判「验过」（旧件该全废）")
	}
	// 新钥签的件「验过」（新钥真能用）
	newPiece := e5Piece(t, state, "dev_test", newPriv, pf.KeyID)
	_ = newPiece
	rc, out, errb = eBatchRun("approve", "show", "dev_test")
	if rc != 0 || e4LineValue(out, "判决") != "验过" {
		t.Errorf("E5 ⑤ 破：新钥签的件应「验过」（rc=%d · 判决=%q）\\nstderr=\\n%s", rc, e4LineValue(out, "判决"), errb)
	}
	_ = oldPiece
}

// ── 矩阵棘轮：`E5` 那几格**同批**进矩阵（`P12`）────────────────────────────────────────────

func TestApproveE5_MatrixCasesSameBatch(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "cli-matrix.json"))
	if err != nil {
		t.Fatalf("E5 破：矩阵读不到：%v", err)
	}
	type e5Case struct {
		ID              string   `json:"id"`
		Argv            []string `json:"argv"`
		WantRC          int      `json:"want_rc"`
		WantStdoutBytes int      `json:"want_stdout_bytes"`
	}
	var m struct {
		Cases []e5Case `json:"cases"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("E5 破：矩阵解不动：%v", err)
	}
	byID := map[string]e5Case{}
	for _, c := range m.Cases {
		byID[c.ID] = c
	}
	for _, id := range []string{
		"approve/keygen#换钥缺确认对（--confirm 与 --yes 都要到）",
		"approve/keygen#换钥缺 --yes（只给 --confirm）",
		"approve/keygen#换钥 --confirm 值不逐字相同",
		"approve/keygen#--rekey 是升级接口的预留名（不许占）",
	} {
		c, ok := byID[id]
		if !ok {
			t.Errorf("E5 破：确认对那一格没同批进矩阵：%s", id)
			continue
		}
		if c.WantRC != 2 || c.WantStdoutBytes != 0 {
			t.Errorf("E5 破：%s 的期望值 = (rc=%d · stdout=%d)（要 2 / 0）", id, c.WantRC, c.WantStdoutBytes)
		}
		rc, out, _ := eBatchRun(c.Argv...)
		if rc != c.WantRC || len(out) != c.WantStdoutBytes {
			t.Errorf("E5 破：%s 现跑 = (rc=%d · stdout=%d 字节) ≠ 矩阵 (%d / %d)", id, rc, len(out), c.WantRC, c.WantStdoutBytes)
		}
	}
}
