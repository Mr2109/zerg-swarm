// cli_gate_find_test.go —— `zerg gate find <片段>` 的**真二进制**判据（缺口 `GAP-20260927-07`）。
//
// 为什么这条缺口值得一枚判据：本命令补的是**门件名 ⇒ 步名 的桥** —— 桥的两端各自早就有了
// （`gate ls` 出步名、`zerg find` 出件名），缺的恰好是**从手上的名字走到键**那一步。判据就钉这一步，
// 并把它与 `zerg code find` 的**零命中口径**钉在一起（同一个方言，别处再写一套必然漂）。
//
// 钉六件（全走**合成门禁脚本** · 不碰真门禁、不跑任何步骤 · 合成仓与夹具复用 `cli_gate_show_test.go`）：
//
//	① 正控①：片段 = **门件名**（`synthetic.py`，只出现在命令串里）⇒ rc=0 且命中步名 `合成步`；
//	② 正控②：片段 = **步名里的词**（`合成`）⇒ 同一格也命中（匹配面两段都在，缺一段就不算桥）；
//	③ 机器面：`--json` ⇒ stdout 是 JSON 且带 `name`（**零命中时不写 stdout** 是另一格，见 ④）；
//	④ 负控①：零命中 ⇒ rc=**1**（**不是 2** —— 与 `zerg code find` 同口径：「没有」不是「错」）；
//	⑤ 负控②：缺片段 ⇒ rc=2（用法错）· stdout **一个字节都不写**（K2 的 0 字节档）；
//	⑥ 只读：跑前 / 跑后整个合成仓逐件 sha256 清单**逐字相同**（本命令不写任何件）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestCLIGateFindBridgesFileToStep —— 桥本身 + 零命中口径 + 只读（缺口 `GAP-20260927-07`）。
func TestCLIGateFindBridgesFileToStep(t *testing.T) {
	bin := zergBinary(t)
	root := showRepo(t) // 合成仓：两行 add_step（`gofmt -l core` / `合成步`，命令串含 `synthetic.py`）

	// ① 片段 = 门件名 —— 只出现在**命令串**里（桥的存在理由）
	rc, out, errb := execCase(t, bin, root, "gate", "find", "synthetic.py")
	if rc != 0 {
		t.Fatalf("门件名应命中步名（rc=0），实得 rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, "合成步") {
		t.Fatalf("命中面必须给出步名「合成步」（这是桥的另一端），实得 stdout=%q", out)
	}

	// ② 片段 = 步名里的词 —— 匹配面两段（步名 + 命令串）缺一段就不算桥
	if rc2, out2, errb2 := execCase(t, bin, root, "gate", "find", "合成"); rc2 != 0 || !strings.Contains(out2, "合成步") {
		t.Fatalf("步名里的词应同样命中（rc=0 且含「合成步」），实得 rc=%d stdout=%q stderr=%q", rc2, out2, errb2)
	}

	// ③ 机器面：`--json` ⇒ stdout 是可解析的 JSON 且带 `name`
	rc3, out3, errb3 := execCase(t, bin, root, "gate", "find", "synthetic.py", "--json")
	if rc3 != 0 {
		t.Fatalf("--json 正控 rc=0，实得 rc=%d（stderr=%s）", rc3, errb3)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out3), &payload); err != nil {
		t.Fatalf("--json 的 stdout 必须是可解析 JSON（%v），实得 %q", err, out3)
	}
	if !strings.Contains(out3, `"name"`) {
		t.Fatalf("--json 的 items 必须带 name 格，实得 %q", out3)
	}

	// ④ 零命中 ⇒ rc=1（与 `zerg code find` 同口径）· stdout 空
	rc4, out4, errb4 := execCase(t, bin, root, "gate", "find", "zzz-没有这个片段-zzz")
	if rc4 != 1 {
		t.Fatalf("零命中必须 rc=1（不是 2 —— 与 `zerg code find` 同口径），实得 rc=%d\nstderr=%s", rc4, errb4)
	}
	if strings.TrimSpace(out4) != "" {
		t.Fatalf("零命中不许往 stdout 写东西（K2 的 0 字节档），实得 stdout=%q", out4)
	}

	// ⑤ 缺片段 ⇒ rc=2 · stdout 空
	rc5, out5, _ := execCase(t, bin, root, "gate", "find")
	if rc5 != 2 {
		t.Fatalf("缺片段必须 rc=2，实得 rc=%d", rc5)
	}
	if strings.TrimSpace(out5) != "" {
		t.Fatalf("用法错不许往 stdout 写东西，实得 stdout=%q", out5)
	}

	// ⑥ 只读：跑前 / 跑后整个合成仓逐件 sha256 逐字相同
	before := repoFingerprint(t, root)
	if rc6, _, _ := execCase(t, bin, root, "gate", "find", "synthetic.py", "--json"); rc6 != 0 {
		t.Fatalf("只读负控的前置跑失败：rc=%d", rc6)
	}
	after := repoFingerprint(t, root)
	if before != after {
		t.Fatalf("本命令必须**只读**：跑前/跑后合成仓指纹不同\n跑前：\n%s\n跑后：\n%s", before, after)
	}
}

// TestCLIGateFindWiringFaceForNonStepFile —— 第二半的判据：**门件不在步骤表里**时，
// 本命令必须给出「在哪跑得到、在哪跑不到」三处接线面的现读结论（缺口 `GAP-20260927-07` 的第二半）。
//
// 为什么这条判据必须存在：合成仓里的 `scripts/gates/precommit-gates.sh` 是**真件**、却**不在**
// `add_step` 命令串里（它是自举件本身）⇒ 旧行为只回「零命中」（= 把人引到「不存在」✗）。
func TestCLIGateFindWiringFaceForNonStepFile(t *testing.T) {
	bin := zergBinary(t)
	root := showRepo(t)

	rc, out, errb := execCase(t, bin, root, "gate", "find", "precommit-gates.sh", "--json", "name,wiring")
	if rc != 0 {
		t.Fatalf("命中真件名应 rc=0（「零命中」与「不存在」不许同形），实得 rc=%d\nstderr=%s", rc, errb)
	}
	if !strings.Contains(out, "precommit-gates.sh") {
		t.Fatalf("门件行必须点名该件，实得 stdout=%q", out)
	}
	if !strings.Contains(out, "wiring") {
		t.Fatalf("门件行必须带 wiring 格（三处接线面的现读结论），实得 stdout=%q", out)
	}
	if !strings.Contains(out, "all.sh") {
		t.Fatalf("wiring 必须含 all.sh 面（合成仓里该件是 .sh ⇒ 应记「不收」），实得 stdout=%q", out)
	}
}

// TestCLIGateFindEscapedQuoteDeclStillMatches —— **命令串内含层转义引号**的声明照样把件名认出来
// （本批量修的那一处解析：`addStepRE` 第五格旧形态 `[^"]*` 在第一个 `\"` 上就停）。
//
// 为什么必须常驻：真源 `scripts/gates/precommit-gates.sh:2866`（B11 那一步）的命令串里就有
// `\"${REL_D}\"` —— 旧解析把它截成 `[ -d \`，**件名整段落在捕获串之外**，于是
// `gate find check-placeholder-residue.py` 把「已挂在 B11 上的件」读成「不在步骤表里」（假阴性）。
// 本条合成夹具**逐字复刻那一形态**（夹具字符串里必须有 `\"`），把「件名 ⇒ 步名」这座桥钉在
// 「命令串含内层转义引号」这一格上：谁把第五格改回 `[^"]*`，本用例必红。
func TestCLIGateFindEscapedQuoteDeclStillMatches(t *testing.T) {
	root := escapedQuoteRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "find", "synthetic-esc.py")
	if rc != 0 {
		t.Fatalf("含内层转义引号的声明里点名的件必须命中（rc=0），实得 rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	// 桥的另一端：**步名**必须出现（只出现件名 = 仍是「未点名」那条假阴性）。
	if !strings.Contains(out, "合成转义引号步") {
		t.Fatalf("命令串含内层转义引号时，件名仍须命中到**步名**「合成转义引号步」（否则就是假阴性：把已接线的件读成未点名），实得 stdout=%q", out)
	}
	// 命令串那格必须是**完整**的（截断的 `[ -d \` 一眼可辨）：尾部那段 `--all` 得在。
	if !strings.Contains(out, "--all") {
		t.Fatalf("第五格必须是**完整命令串**（旧形态截到 `[ -d \\` 会丢掉尾部），实得 stdout=%q", out)
	}
}

// TestCLIGateFindUnwiredFileStillUnnamed —— **真未接线件仍报未点名**（防把假阴性治成假阳性）：
// 修截断后若把「命中」的标准放宽（例如只看行里有没有那个词），真未接线的件会被误报成「点名」。
//
// 夹具逐字形态：件名只出现在**heredoc 体**里（声明行的原文里没有）⇒ 解析面读不到它，
// 所以结论只能是「未点名」，且 `next` 仍是 `code find`（不是 `gate show`）。
func TestCLIGateFindUnwiredFileStillUnnamed(t *testing.T) {
	root := escapedQuoteRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "find", "unwired-only.py", "--json", "name,verdict,next,wiring")
	if rc != 0 {
		t.Fatalf("真件名命中应 rc=0，实得 rc=%d\nstderr=%s", rc, errb)
	}
	if !strings.Contains(out, "未点名") {
		t.Fatalf("真未接线件必须报「未点名」（不许多认出一步），实得 stdout=%q", out)
	}
	if strings.Contains(out, "gate show") {
		t.Fatalf("未接线件的 `next` 不许给 `gate show`（它没有步名可取），实得 stdout=%q", out)
	}
	if !strings.Contains(out, "code find") {
		t.Fatalf("未接线件的 `next` 应回落到 `code find`，实得 stdout=%q", out)
	}
}

// TestCLIGateFindUnparsableDeclSaysUncertain —— **解析不确定**必须显式点名（与「读不到不许当健康」同口径）：
// 一条 `add_step` 声明若命令串是多行/heredoc（四格引号串在**行内**不闭合），解析器读不出它 ——
// 而它的原文里点到过一个件名。此时那一件**不许**报「未点名」（点没点名判不了），必须报「解析不确定」+ 行号。
//
// 为什么这条是必需的另一半：只修「含转义引号的截断」仍会漏掉**根本读不出来的行**，
// 而那正是「假阴性」的第二条来路（`precommit-gates.sh:1794` 那条 heredoc 声明 · 实机复现见回执）。
func TestCLIGateFindUnparsableDeclSaysUncertain(t *testing.T) {
	root := escapedQuoteRepo(t)
	rc, out, errb := runZergRepo(t, root, "gate", "find", "uncertain-named.py", "--json", "name,verdict,wiring")
	if rc != 0 {
		t.Fatalf("真件名命中应 rc=0，实得 rc=%d\nstderr=%s", rc, errb)
	}
	if !strings.Contains(out, "解析不确定") {
		t.Fatalf("件名出现在读不出的声明行原文里 ⇒ 必须报「解析不确定」（不许报「未点名」），实得 stdout=%q", out)
	}
	if strings.Contains(out, "步骤表里未点名") {
		t.Fatalf("读不出的行**不许**被读成「未点名」（拿读不到当「没有」），实得 stdout=%q", out)
	}
	// 同一个结论也要在 stderr 的清单里点名行号（人核对面）。
	if !strings.Contains(errb, "解析不确定") || !strings.Contains(errb, "precommit-gates.sh:") {
		t.Fatalf("stderr 必须点名「解析不确定」与真源行号，实得 stderr=%q", errb)
	}
}

// escapedQuoteRepo —— 夹具合成仓（三件 · 一枚声明含**内层转义引号** + 一枚声明**读不出来**）：
//
//	`scripts/gates/synthetic-esc.py`    —— 由含 `\"` 的声明点名（桥的正控）；
//	`scripts/gates/unwired-only.py`     —— 只出现在 heredoc **体**里 ⇒ 真未接线（未点名的正控）；
//	`scripts/gates/uncertain-named.py`  —— 出现在**读不出**的声明行原文里 ⇒ 解析不确定（第三格）。
//
// 本仓真源里这三种形态**都有**（`precommit-gates.sh:2866` / `:1794`），故夹具逐字照造，不自创形态。
func escapedQuoteRepo(t *testing.T) string {
	t.Helper()
	root := syntheticRepo(t, "exit 0\n")
	for _, f := range []string{"synthetic-esc.py", "unwired-only.py", "uncertain-named.py"} {
		mustWrite(t, filepath.Join(root, "scripts", "gates", f), "print('合成分')\n")
	}
	appendToFile(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		// ① 第五格含 `\"`（与真源 :2866 逐字同形）。
		"\nadd_step release \"release: 合成转义引号步（只报告）\" tri-report \"${REPO_ROOT}\" \"[ -d \\\"${REL_D}\\\" ] || { echo \\\"缺件\\\" ; exit 2; }; python3 scripts/gates/synthetic-esc.py \\\"${REL_D}\\\" --all\"\n"+
			// ② 声明行原文里点到一件，但四格引号串**行内不闭合**（heredoc）⇒ 读不出来；件名只在 heredoc 体里出现的另一件 ⇒ 真未接线。
			"add_step pub \"合成 heredoc 步\" rc \"${REPO_ROOT}\" \"python3 - <<'PYEOF' scripts/gates/uncertain-named.py\n"+
			"print('scripts/gates/unwired-only.py')\n"+
			"PYEOF\"\n")
	return root
}

// repoFingerprint —— 合成仓的逐件 sha256 清单（按相对路径排序 · 只看**内容**，不看 mtime）。
func repoFingerprint(t *testing.T, root string) string {
	t.Helper()
	lines := []string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		lines = append(lines, rel+"  "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("读合成仓失败：%v", err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestCLIGateFindUnreadableGateDirNoVerdict —— `scripts/gates/` **读不到** ⇒ rc=8「不给结论」的
// **常驻负控**（缺口 `GAP-20260927-221` · 提案 `DEV-0387` 的「未做项①」）。
//
// 为什么这条必须是**测件里的常驻件**（不能只在仓外夹具里取证一次）：同族新出口 `gate_dir_unreadable`
// （退码 8）若只被仓外夹具证明过一次，就**没有棘轮** —— 谁把 `gateFilesMatching` 改回「吞错 `return nil`」，
// 旧形态会静默退化回「零命中退码 1」（= 拿半张表当全表），测件却全绿。本用例把**两态**钉成一对：
//
//	① 负控（权限 `0111` = 可进不可读）：`scripts/gates/` 读不到 ⇒ rc=**8**（不给结论）· stdout **0 字节** ·
//	   stderr 含「读不到」与「不给结论」；
//	② 阳控（同一合成仓 · 同一探针 · 权限复原 `0755`）：回到 rc=**1**「零命中」。
//
// 两态成对 ⇒ 探针**有鉴别力**：把①的期望值改错（8→1）本用例必 FAIL（见回执「期望值改错」反证）。
// 走**进程内** `runZergRepo`（读**当前源码** · 与 `cli_gate_results_test.go` 的 rc=8 同层）——
// `bin/zerg` 是 gitignore 的**按批重建制品**，本批不重建（同 `cli_gate_show_test.go` 头注那条口径）。
//
// 纪律：`0111` 是**危险态**（目录不可读），故 `defer` + `t.Cleanup` 双保险复原 —— 否则 `t.TempDir`
// 清不掉、同包后续用例受污。探针取一段**谁都不含**的串，保证阳控那态一定是「零命中」而非别的命中。
func TestCLIGateFindUnreadableGateDirNoVerdict(t *testing.T) {
	root := showRepo(t) // 合成仓：`scripts/gates/precommit-gates.sh` + 两行 add_step
	dir := filepath.Join(root, "scripts", "gates")

	// 复原权限的兜底：无论断言在哪一步退场（含 Fatalf），都把 0111 还原。
	restore := func() { _ = os.Chmod(dir, 0o755) }
	t.Cleanup(restore)
	defer restore()

	// 探针：不含任何步名/命令串/件名的串 ⇒ 权限正常时必然「零命中」（阳控的靶子）。
	const probe = "zzz-本片段谁都不含-zzz"

	// ① 负控：`scripts/gates/` 读不到（0111 = 可进不可读）⇒ rc=8「不给结论」。
	if err := os.Chmod(dir, 0o111); err != nil {
		t.Fatalf("造不出「目录读不到」态（chmod 0111 失败）：%v", err)
	}
	rc, out, errb := runZergRepo(t, root, "gate", "find", probe)
	if rc != 8 {
		t.Fatalf("`scripts/gates/` 读不到必须 rc=8「不给结论」（不是 1「零命中」—— 读不到≠没有），实得 rc=%d\nstdout=%q\nstderr=%q",
			rc, out, errb)
	}
	if len(out) != 0 {
		t.Fatalf("读不到 ⇒ stdout 必须 **0 字节**（不许拿半张表当全表出半份），实得 %d 字节：%q", len(out), out)
	}
	if !strings.Contains(errb, "读不到") || !strings.Contains(errb, "不给结论") {
		t.Fatalf("stderr 必须明说「读不到」与「不给结论」，实得 %q", errb)
	}

	// ② 阳控：权限复原 `0755` ⇒ 同仓同名探针回到 rc=1「零命中」——两态成对，证明探针真断到差异。
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("复原 0755 失败：%v", err)
	}
	rc2, out2, errb2 := runZergRepo(t, root, "gate", "find", probe)
	if rc2 != 1 {
		t.Fatalf("权限复原后同名探针必须 rc=1「零命中」（两态成对），实得 rc=%d\nstderr=%q", rc2, errb2)
	}
	if strings.TrimSpace(out2) != "" {
		t.Fatalf("零命中也不许往 stdout 写东西（K2 的 0 字节档），实得 stdout=%q", out2)
	}
}

// TestCLIGateFindPublishFaceForPublishGate —— GAP-20260928-29：发布面闸（按设计不在 add_step 里）的真家
// = scripts/build/publish-preflight.sh 的闸④ ⇒ `gate find` 的 wiring 格必须补印「发布面: 点名@N」
// （此前只印 precommit / all.sh / real-gates 三面、不指向这条真家 ⇒ 发布面闸「无正门读数口」）。
//
// 两态成对：① 负控（移走态）：publish-preflight.sh 不在盘 ⇒ wiring 不许出现「发布面」；
// ② 正控（在盘态）：闸④ 以真命令位调该件（第 1 行）⇒ wiring 补印「发布面: 点名@1」。
// 走进程内 `runZergRepo`（读当前源码 · 与同件 rc=8 那条同层 —— `bin/zerg` 是按批重建制品，本单不重建）。
func TestCLIGateFindPublishFaceForPublishGate(t *testing.T) {
	root := showRepo(t) // precommit-gates.sh 是自举件 ⇒ 不在 add_step ⇒ 走门件行
	pb := filepath.Join(root, "scripts", "build", "publish-preflight.sh")
	if err := os.MkdirAll(filepath.Dir(pb), 0o755); err != nil {
		t.Fatalf("建 scripts/build/ 失败：%v", err)
	}

	// ① 负控（移走态）：该件不在盘 ⇒ wiring 不许含「发布面」。
	rc0, out0, errb0 := runZergRepo(t, root, "gate", "find", "precommit-gates.sh", "--json", "name,wiring")
	if rc0 != 0 {
		t.Fatalf("门件行应 rc=0，实得 rc=%d（stderr=%s）", rc0, errb0)
	}
	if strings.Contains(out0, "发布面") {
		t.Fatalf("publish-preflight.sh 不在盘时 wiring 不许含「发布面」，实得 %q", out0)
	}

	// ② 正控（在盘态）：闸④ 以真命令位调它（第 1 行）⇒ wiring 补印「发布面: 点名@1」。
	if err := os.WriteFile(pb, []byte("python3 \"$Z/scripts/gates/precommit-gates.sh\" \"$D\" HEAD > /tmp/pf4.log 2>&1; RC4=$?\n"), 0o644); err != nil {
		t.Fatalf("写 publish-preflight.sh 失败：%v", err)
	}
	rc1, out1, errb1 := runZergRepo(t, root, "gate", "find", "precommit-gates.sh", "--json", "name,wiring")
	if rc1 != 0 {
		t.Fatalf("门件行应 rc=0，实得 rc=%d（stderr=%s）", rc1, errb1)
	}
	if !strings.Contains(out1, "发布面: 点名@1") {
		t.Fatalf("publish-preflight.sh 真命令位调用该件（第 1 行）⇒ wiring 必须补印「发布面: 点名@1」，实得 %q", out1)
	}
}

// TestCLIGateFindRelPathFormSameVisibilityAsBasename —— 治 `GAP-20260928-150` 的**第二半**：
// **入参形态**不许改变可见性（上一半修的是门件面 `gateFilesMatching`，本半同步步名面）。
//
// 病（父代理 2026-10-14 现读 · 两条逐字读数）：同一件只差入参形态就读出两种结论 ——
//
//	zerg gate find 合成步              ⇒ rc=0（片段命中**步名**）
//	zerg gate find scripts/gates/合成步 ⇒ rc=1「零命中」（整串在步名/命令串上命不中）
//
// 治法＝**步名面与门件面同尺**：整串与**基名**（`filepath.Base`）两档并试。基名 == 整串时逐字等价于
// 旧行为 ⇒ 缺省面零变化。三格成对（缺一格就证不了「同尺」且「不放宽」）：
//
//	① 基名形态（正控 · 旧行为）：rc=0 且命中该步；
//	② **仓内相对路径形态**（本单修的格）：rc=0 且**读数本体 `items` 与①逐字节同**（同尺就是同一读数）；
//	③ 反例：**不存在的相对路径** ⇒ 仍 rc=1「零命中」且 stdout 0 字节（补形态 ≠ 把「没有」读成「有」）。
//
// 走**进程内** `runZergRepo`（读**当前源码** —— `bin/zerg` 是按批重建制品，本单不重建）。
func TestCLIGateFindRelPathFormSameVisibilityAsBasename(t *testing.T) {
	root := showRepo(t) // 合成仓：步名「合成步」，命令串 `python3 scripts/gates/synthetic.py`

	// ① 基名形态（正控 · 旧行为一字不改）。
	rcA, outA, errA := runZergRepo(t, root, "gate", "find", "合成步", "--json", "name,cmd,wiring")
	if rcA != 0 || !strings.Contains(outA, "合成步") {
		t.Fatalf("基名形态（正控）应 rc=0 且命中「合成步」，实得 rc=%d stdout=%q stderr=%q", rcA, outA, errA)
	}

	// ② 仓内相对路径形态 —— 本单的修点：必须与①同尺（同一行集）。
	rcB, outB, errB := runZergRepo(t, root, "gate", "find", "scripts/gates/合成步", "--json", "name,cmd,wiring")
	if rcB != 0 {
		t.Fatalf("**仓内相对路径形态**必须与基名形态同尺（rc=0），实得 rc=%d\nstderr=%s", rcB, errB)
	}
	if !strings.Contains(outB, "合成步") {
		t.Fatalf("相对路径形态必须同样命中「合成步」，实得 stdout=%q", outB)
	}
	// 「同尺」＝**读数本体**逐字节同（包封里的 idempotency_key 是逐次派生量，不参与比较）。
	jA, _ := json.Marshal(envelopeItems(t, outA))
	jB, _ := json.Marshal(envelopeItems(t, outB))
	if string(jA) != string(jB) {
		t.Fatalf("两形态必须给出**同一读数本体**（同尺）\n基名形态：%s\n相对路径形态：%s", jA, jB)
	}

	// ③ 反例：不存在的相对路径仍须「零命中」（补形态后不许把「没有」读成「有」）。
	rcC, outC, errC := runZergRepo(t, root, "gate", "find", "scripts/gates/zzz-没有这一件-zzz")
	if rcC != 1 {
		t.Fatalf("不存在的相对路径必须仍 rc=1「零命中」，实得 rc=%d\nstderr=%s", rcC, errC)
	}
	if strings.TrimSpace(outC) != "" {
		t.Fatalf("零命中不许写 stdout（K2 的 0 字节档），实得 %q", outC)
	}
}

// TestCLIGateFindVariableIndirectCmdVisible —— ★ GAP-20260929-28：命令串先整串算进变量、`add_step` 第五格
// 只留变量名（`"${VAR}"`）时，门面**必须看得见**（三分类 + 坐实值集），✗ 不许再静默零命中。
//
// 病（现读 · 真仓逐字）：`add_step … "${B8_ROSTER_CMD}"` 的命令串里含 `unhealthy_count` ⇒
// `zerg gate find unhealthy_count` **零命中**（rc=1）且被读成「真没有」；`gate explain` 只回变量名不展开。
//
// 本用例钉四件（合成仓 · 进程内读**当前源码** —— `bin/zerg` 是按批重建制品，本单不重建）：
//
//	① 变量间接：片段 = 变量**值**里的串（只出现在赋值行）⇒ rc=0 且命中该步（命令内容 ⇒ 步名 的桥）；
//	② 出处：命中行的 `source` 必须点名**变量**与**它的赋值行**（`@L…`）—— 真身在哪要可读；
//	③ 反例（字面命令**不误报**）：字面命令串里的片段 ⇒ rc=0 命中该步（与改前逐字同行为）；
//	④ 零命中分家：变量间接在场时，零命中印「解析不到」而非「真没有」；退码仍 **1**、stdout 仍 **0 字节**。
func TestCLIGateFindVariableIndirectCmdVisible(t *testing.T) {
	root := syntheticRepo(t, syntheticGateList)
	appendToFile(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		// 变量间接（单行双引号赋值 · 命令串整串进变量）：值里含哨兵串 ZZZVARHIT。
		"\nVAR_CMD=\"python3 scripts/gates/var-hidden.py --marker=ZZZVARHIT\"\n"+
			"add_step gates \"变量间接步\" tri \"${REPO_ROOT}\" \"${VAR_CMD}\"\n"+
			// 字面命令（反例）：片段 ZZZLITHIT 逐字在命令串里。
			"add_step gates \"字面步\" tri \"${REPO_ROOT}\" \"python3 scripts/gates/lit.py --tok=ZZZLITHIT\"\n")
	mustWrite(t, filepath.Join(root, "scripts", "gates", "var-hidden.py"), "print(1)\n")
	mustWrite(t, filepath.Join(root, "scripts", "gates", "lit.py"), "print(1)\n")

	// ① 变量间接：片段只出现在**赋值行**（命令串可见面 = `${VAR_CMD}`）⇒ 必须命中该步。
	rc, out, errb := runZergRepo(t, root, "gate", "find", "ZZZVARHIT", "--json", "name,source")
	if rc != 0 {
		t.Fatalf("变量间接的命令内容必须看得见（rc=0），实得 rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, "变量间接步") {
		t.Fatalf("命中面必须给出步名「变量间接步」（命令内容 ⇒ 步名 的桥），实得 stdout=%q", out)
	}
	// ② 出处必须点名**变量**与**它的赋值行**（真身在哪）。
	if !strings.Contains(out, "VAR_CMD") || !strings.Contains(out, "@L") {
		t.Fatalf("出处必须点名变量与其赋值行（`${VAR_CMD}` + `@L…`），实得 stdout=%q", out)
	}

	// ③ 反例：字面命令的片段仍按原样判（✗ 不误报、✗ 不漏报）。
	rc2, out2, errb2 := runZergRepo(t, root, "gate", "find", "ZZZLITHIT", "--json", "name,source")
	if rc2 != 0 || !strings.Contains(out2, "字面步") {
		t.Fatalf("字面命令片段必须照旧命中「字面步」（rc=0），实得 rc=%d stdout=%q stderr=%q", rc2, out2, errb2)
	}

	// ④ 零命中分家：变量间接在场 ⇒ 印「解析不到」（不是「真没有」）· 退码仍 1 · stdout 空。
	rc3, out3, errb3 := runZergRepo(t, root, "gate", "find", "zzz-本片段谁都不含-zzz")
	if rc3 != 1 {
		t.Fatalf("零命中必须仍 rc=1（不是 2），实得 rc=%d", rc3)
	}
	if strings.TrimSpace(out3) != "" {
		t.Fatalf("零命中不许写 stdout（K2 的 0 字节档），实得 %q", out3)
	}
	if !strings.Contains(errb3, "解析不到") {
		t.Fatalf("变量间接在场时零命中必须印「解析不到」（✗ 不许拿「真没有」盖过去），实得 stderr=%q", errb3)
	}
	if strings.Contains(errb3, "是「没有」") {
		t.Fatalf("变量间接在场时 ✗ 不许再印「真没有」那句（拿读不到当「没有」＝本仓病根），实得 stderr=%q", errb3)
	}
}

// TestCLIGateFindUncertainFace —— ★ `GAP-20260929-28` 剩面（2.5.15-28c）：`gate find --uncertain` 把
// 「解析不确定」清单从 stderr 侧信道提到**正门** stdout / 机器面（`line`/`kind`/`why`/`raw` 表 + `--json`），
// 并钉住「用法串面已登记」与「字面命令不误报」。
//
// 钉五件（合成仓 · 进程内读**当前源码** —— `bin/zerg` 是按批重建制品，本单不重建）：
//
//	① 正控：`--uncertain` ⇒ rc=0 且 stdout 是 `line`/`kind`/`why`/`raw` 表（四格齐）；
//	② 机器面：`--uncertain --json line,kind,why,raw` ⇒ rc=0 且逐条带分类（变量间接 / 命令替换）；
//	③ 用法串面登记：`--json command_kind` 点得出来（rc=0，✗ 不再 `json_field_unknown`）；
//	④ 反例（字面命令**不误报**）：字面步**不进**不确定表，且它的片段照旧命中（rc=0）；
//	⑤ 零不确定 ⇒ 与 `gate find` 零命中同口径：rc=1 · stdout **0 字节** · stderr 有明报。
func TestCLIGateFindUncertainFace(t *testing.T) {
	root := syntheticRepo(t, syntheticGateList)
	appendToFile(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		// 变量间接：`VAR_CMD=$(date)` 是**裸值**（`=` 后不是引号串）⇒ 坐不实 ⇒ 进 unc · 分类=变量间接。
		"\nVAR_CMD=$(date)\n"+
			"add_step gates \"变量间接步\" tri \"${REPO_ROOT}\" \"${VAR_CMD}\"\n"+
			// 命令替换 ⇒ 进 unc · 分类=命令替换。
			"add_step gates \"命令替换步\" tri \"${REPO_ROOT}\" \"$(my_synth_fn arg)\"\n"+
			// 字面命令（反例）：片段 ZZZLITHIT 逐字在命令串里 ⇒ ✗ 不许进不确定表。
			"add_step gates \"字面步\" tri \"${REPO_ROOT}\" \"python3 scripts/gates/lit.py --tok=ZZZLITHIT\"\n")

	// ① 人面：`--uncertain` ⇒ rc=0 且表头四格齐（line/kind/why/raw）。
	rc, out, errb := runZergRepo(t, root, "gate", "find", "--uncertain")
	if rc != 0 {
		t.Fatalf("有不确定行 ⇒ rc=0，实得 rc=%d\nstdout=%s\nstderr=%s", rc, out, errb)
	}
	for _, col := range []string{"line", "kind", "why", "raw"} {
		if !strings.Contains(out, col) {
			t.Fatalf("正门表必须给 %s 格，实得 stdout=%q", col, out)
		}
	}

	// ② 机器面：逐条带分类（变量间接 / 命令替换）；字面步**不进**表。
	rc2, out2, errb2 := runZergRepo(t, root, "gate", "find", "--uncertain", "--json", "line,kind,why,raw")
	if rc2 != 0 {
		t.Fatalf("`--uncertain --json` 必须 rc=0，实得 rc=%d（stderr=%s）", rc2, errb2)
	}
	if !strings.Contains(out2, "变量间接") || !strings.Contains(out2, "命令替换") {
		t.Fatalf("不确定表必须逐条给分类（变量间接 / 命令替换），实得 %q", out2)
	}
	if strings.Contains(out2, "字面步") || strings.Contains(out2, "ZZZLITHIT") {
		t.Fatalf("字面命令**不许**进不确定表（✗ 误报），实得 %q", out2)
	}

	// ③ 用法串面登记：`--json command_kind` 点得出来（✗ 不再 json_field_unknown）。
	rc3, out3, errb3 := runZergRepo(t, root, "gate", "find", "ZZZLITHIT", "--json", "name,command_kind")
	if rc3 != 0 || !strings.Contains(out3, "command_kind") {
		t.Fatalf("`--json command_kind` 必须点得出来（已登 optFields），实得 rc=%d stdout=%q stderr=%q",
			rc3, out3, errb3)
	}

	// ④ 反例：字面步的片段照旧命中（✗ 不因新面而误报 / 漏报）。
	rc4, out4, errb4 := runZergRepo(t, root, "gate", "find", "ZZZLITHIT", "--json", "name")
	if rc4 != 0 || !strings.Contains(out4, "字面步") {
		t.Fatalf("字面步必须照旧命中（rc=0），实得 rc=%d stdout=%q stderr=%q", rc4, out4, errb4)
	}

	// ⑤ 零不确定 ⇒ 与 `gate find` 零命中同口径：rc=1 · stdout 0 字节 · stderr 明报。
	clean := syntheticRepo(t, syntheticGateList)
	rc5, out5, errb5 := runZergRepo(t, clean, "gate", "find", "--uncertain")
	if rc5 != 1 {
		t.Fatalf("零不确定必须 rc=1（与 `gate find` 零命中同口径），实得 rc=%d", rc5)
	}
	if strings.TrimSpace(out5) != "" {
		t.Fatalf("零不确定不许写 stdout（K2 的 0 字节档），实得 %q", out5)
	}
	if !strings.Contains(errb5, "解析不确定 0 条") {
		t.Fatalf("零不确定必须有 stderr 明报，实得 %q", errb5)
	}
}
