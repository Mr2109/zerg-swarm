// ai_boundary_test.go —— AI 的边界与授权四条判据的机检（层①进程内 · 外部测试包 `package main_test` ·
// 开工单 T-60）。
//
// 判据（开工单逐字）：
//
//	① 「提 ≠ 批」落成**两个字段**（`subject` + `subject_kind` 四值闭集），**两者相同 ⇒ 红**；
//	② 模型侧进程 argv/环境里**零** `8580`/`8082`/`8100` 与 `ZERG_*` 令牌；
//	③ 「放文件即生效」的两处口子（技能目录 / MCP）按 `P-101` 收口（写权限只给人 + 清单默认不静默）；
//	④ `sudo` 的处置定案（`P-103`）。
//
// 每条都**带负控**（喂错的东西必须红）—— 「全过」之前先证明这台机器判得出红。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// proposalNew 跑一次 `dev proposal new`（提案目录指到临时目录 ⇒ 不碰本机状态）。
func proposalNewAuthz(t *testing.T, extra ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	argv := append([]string{"dev", "proposal", "new",
		"--title", "授权面", "--target", "待办:D1", "--goal", "g",
		"--evidence", "e", "--rollback", "r", "--criterion", "zerg gate run --fast"}, extra...)
	return runCapture(argv...)
}

// ---- 判据①：真源成形（四值闭集 / 两对字段 / 谁不许批）----

func TestAIBoundaryContract(t *testing.T) {
	spec, err := zerg.AIBoundarySpecOfForTest()
	if err != nil {
		t.Fatalf("AI 边界真源读不出来：%v", err)
	}
	want := []string{"human", "ai", "egg", "ci"}
	if strings.Join(spec.SubjectKinds, "|") != strings.Join(want, "|") {
		t.Errorf("subject_kind 闭集 = %v（要 %v —— §18.1 接缝第 18 行逐字）", spec.SubjectKinds, want)
	}
	if len(spec.FileDropDoors) != 2 {
		t.Errorf("「放文件即生效」的口子登记 = %v（要两处：技能目录 / MCP）", spec.FileDropDoors)
	}
	for _, d := range spec.FileDropDoors {
		if !strings.HasSuffix(d, "|human") {
			t.Errorf("口子 %q 的 writer 不是 human —— 写权限只给人（P-101 ①）", d)
		}
	}
	if spec.SudoDetail != "sudo_not_accepted" {
		t.Errorf("sudo 拒收的 detail = %q（要 sudo_not_accepted）", spec.SudoDetail)
	}
	// 负控：把闭集改一个字，上面的比对必须失败（证明这一格不是恒绿）。
	bad := append([]string{}, spec.SubjectKinds...)
	bad[0] = "Human"
	if strings.Join(bad, "|") == strings.Join(want, "|") {
		t.Error("负控失败：大小写不同的闭集被判成了「一样」")
	}
}

// ---- 判据①：四条规则逐条（含负控）----

func TestProposalAuthzRules(t *testing.T) {
	// ⓐ `--subject-kind ai` + 提出者 ⇒ 过得去（提者可以是 AI）。
	if rc, _, errb := proposalNewAuthz(t, "--subject", "某AI", "--subject-kind", "ai"); rc != 0 {
		t.Errorf("`--subject-kind ai` 退码 = %d（要 0）· stderr=%s", rc, errb)
	}
	// ⓑ 闭集外 ⇒ 2（精确相等，不认近义词）。
	rc, _, errb := proposalNewAuthz(t, "--subject", "某AI", "--subject-kind", "机器人")
	if rc != 2 || !strings.Contains(errb, "不在闭集里") {
		t.Errorf("闭集外的 kind：rc=%d（要 2）· stderr=%s", rc, errb)
	}
	// ⓑ′ 大小写折叠也不行（负控成对）。
	if rc, _, _ := proposalNewAuthz(t, "--subject", "某AI", "--subject-kind", "AI"); rc != 2 {
		t.Errorf("`AI` 大写形态：rc=%d（要 2 —— 不认大小写折叠）", rc)
	}
	// ⓒ kind=egg 缺 `--egg-id` ⇒ 2；给了 ⇒ 0。
	if rc, _, errb := proposalNewAuthz(t, "--subject", "egg-1", "--subject-kind", "egg"); rc != 2 ||
		!strings.Contains(errb, "--egg-id") {
		t.Errorf("kind=egg 缺卵 id：rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if rc, _, errb := proposalNewAuthz(t, "--subject", "egg-1", "--subject-kind", "egg", "--egg-id", "EGG-7"); rc != 0 {
		t.Errorf("kind=egg 带卵 id：rc=%d（要 0）· stderr=%s", rc, errb)
	}
	// ⓓ **提 ≠ 批**：提出者与批准者逐字相同 ⇒ 红。
	rc, _, errb = proposalNewAuthz(t, "--subject", "张三", "--approver", "张三")
	if rc != 2 || !strings.Contains(errb, "提 ≠ 批") {
		t.Errorf("自审自批：rc=%d（要 2）· stderr=%s", rc, errb)
	}
	// ⓔ 批只人给：approver_kind 不是 human ⇒ 红（含 ai / egg / ci）。
	for _, k := range []string{"ai", "egg", "ci"} {
		rc, _, errb := proposalNewAuthz(t, "--subject", "张三", "--approver", "李四", "--approver-kind", k)
		if rc != 2 || !strings.Contains(errb, "批只能人给") {
			t.Errorf("approver_kind=%s：rc=%d（要 2）· stderr=%s", k, rc, errb)
		}
	}
	// ⓕ 两人两字段分家 ⇒ 过得去，且件里落的就是这四个值。
	if rc, _, errb := proposalNewAuthz(t, "--subject", "张三", "--subject-kind", "human",
		"--approver", "李四"); rc != 0 {
		t.Fatalf("人提人批：rc=%d（要 0）· stderr=%s", rc, errb)
	}
	// 读回一件：subject/subject_kind 必须在；`by` 是兼容别名（两者同值）。
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	rc, out, errb := runCapture("dev", "proposal", "new", "--title", "读回", "--target", "待办:D1",
		"--goal", "g", "--evidence", "e", "--rollback", "r", "--by", "老王", "--subject-kind", "ai",
		"--criterion", "zerg gate run --fast")
	if rc != 0 {
		t.Fatalf("落件退码 = %d（要 0）· stderr=%s", rc, errb)
	}
	id := strings.TrimSpace(out)
	rc, out, errb = runCapture("dev", "proposal", "show", id, "--json", "subject,subject_kind,by")
	if rc != 0 {
		t.Fatalf("读回退码 = %d（要 0）· stderr=%s", rc, errb)
	}
	var doc struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil || len(doc.Items) != 1 {
		t.Fatalf("读回的 JSON 不成形：%v / %s", err, out)
	}
	it := doc.Items[0]
	if it["subject"] != "老王" || it["subject_kind"] != "ai" || it["by"] != "老王" {
		t.Errorf("件里的授权面字段不对：%v（`by` 要与 `subject` 同值 —— 兼容别名）", it)
	}
}

// ---- 判据④：`sudo` 一律不接受（调度前判 · 含负控）----

func TestSudoIsNeverAccepted(t *testing.T) {
	// ① 词形态的 sudo：裸词 / 路径形态 / 嵌在旗标值里 —— 三种都拒。
	for _, a := range []string{"sudo", "/usr/bin/sudo", "sudo rm -rf /", "--cmd=sudo ls"} {
		hit, ok := zerg.SudoRefusalForTest([]string{"script", "run", a})
		if !ok {
			t.Errorf("%q 没被拒收 —— 命令面一律不接受 sudo（P-103）", a)
		}
		if ok && hit != a {
			t.Errorf("命中的参数报成了 %q（要 %q）", hit, a)
		}
	}
	// ② 负控（成对）：「长得像但不是那个词」的不许被误伤 —— 否则判据会变成噪声。
	for _, a := range []string{"xsudo", "no-sudo", "sudon't", "sudoer", "sudos"} {
		if _, ok := zerg.SudoRefusalForTest([]string{"version", a}); ok {
			t.Errorf("%q 被误判成 sudo —— 词边界不对（假阳性会把判据变成噪声）", a)
		}
	}
	// ③ 端到端：真的走一遍调度（拒收发生在命令树解析**之前**）。
	rc, out, errb := runCapture("script", "run", "sudo", "--confirm=sudo", "--yes")
	if rc != 2 {
		t.Errorf("`script run sudo` 退码 = %d（要 2）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "命令面一律不接受") {
		t.Errorf("stderr 里没有逐字「命令面一律不接受」：%s", errb)
	}
	if out != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %q", out)
	}
	if !strings.Contains(errb, "要提权的事走人") {
		t.Errorf("stderr 里没有逐字「要提权的事走人」：%s", errb)
	}
	// 负控：不带 sudo 的同一条命令**不是**被 sudo 那一条拦下的（理由不同）。
	_, _, errb2 := runCapture("script", "run", "ls", "--confirm=ls", "--yes")
	if strings.Contains(errb2, "命令面一律不接受") {
		t.Errorf("不含 sudo 的命令被 sudo 那条拦了：%s", errb2)
	}
}

// ---- 判据②：模型侧 argv/环境零控制面端口与令牌（含词边界负控）----

func TestModelSideZeroControlPlaneTokens(t *testing.T) {
	root := zerg.RepoRootForTest()
	if root == "" {
		t.Fatal("解析不到仓根 ⇒ 不给结论（读不到不当没有）")
	}
	hits, scanned, err := zerg.ModelSideScanForTest(root)
	if err != nil {
		t.Fatalf("扫不动模型侧面：%v", err)
	}
	if scanned == 0 {
		t.Fatal("**空转**：一个文件都没扫到 —— 判据自己坏了，不许当绿")
	}
	if len(hits) != 0 {
		t.Errorf("模型侧面命中 %d 条禁令牌（要 0）：%v", len(hits), hits)
	}

	// 负控①：合成一棵树，往非测试 .go 里塞 `8580` ⇒ 必红。
	syn := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syn, "hatch"), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := "package hatch\n\nvar leak = \"http://127.0.0.1:8580\"\n"
	if err := os.WriteFile(filepath.Join(syn, "hatch", "leak.go"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, n2, err := zerg.ModelSideScanRawForTest(syn, []string{"hatch"}, []string{"8580", "8082", "8100"}, "ZERG_", []string{"_test.go"})
	if err != nil || n2 == 0 {
		t.Fatalf("合成夹具扫不动：%v / %d 件", err, n2)
	}
	if len(h2) != 1 {
		t.Errorf("负控①失败：塞了 8580 却判出 %d 条（要 1）", len(h2))
	}

	// 负控②：`ZERG_` 前缀命中（令牌泄漏）。
	if err := os.WriteFile(filepath.Join(syn, "hatch", "tok.go"),
		[]byte("package hatch\n\nconst k = \"ZERG_TOKEN\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h3, _, _ := zerg.ModelSideScanRawForTest(syn, []string{"hatch"}, []string{"8580"}, "ZERG_", []string{"_test.go"})
	if len(h3) == 0 {
		t.Error("负控②失败：`ZERG_TOKEN` 没被抓到")
	}

	// 负控③（词边界 · **成对**）：`58100` 里含 `8100` 的子串**不算命中** ——
	//   这条正是实测出来的假阳性来源（`agent/internal/hatch/hatch_test.go` 的 `--port 58100` 夹具）。
	syn2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syn2, "hatch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(syn2, "hatch", "ok.go"),
		[]byte("package hatch\n\nvar port = 58100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h4, _, _ := zerg.ModelSideScanRawForTest(syn2, []string{"hatch"}, []string{"8100"}, "ZERG_", []string{"_test.go"})
	if len(h4) != 0 {
		t.Errorf("负控③失败：`58100` 被子串匹配误伤成命中：%v", h4)
	}
	// 负控④：排除面有牙 —— 同一件塞进 `_test.go` ⇒ 不命中（扫描面宣言生效）。
	if err := os.WriteFile(filepath.Join(syn2, "hatch", "ok_test.go"),
		[]byte("package hatch\n\nvar p = 8100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h5, _, _ := zerg.ModelSideScanRawForTest(syn2, []string{"hatch"}, []string{"8100"}, "ZERG_", []string{"_test.go"})
	if len(h5) != 0 {
		t.Errorf("负控④失败：排除面没生效（_test.go 被扫了）：%v", h5)
	}
}

// ---- 判据③①：命令树里不许有「往两处口子写」的命令（含负控）----

func TestNoCommandWritesToDropDoors(t *testing.T) {
	hits, err := zerg.DoorWriteViolationsForTest()
	if err != nil {
		t.Fatalf("判定口读不出来：%v", err)
	}
	if len(hits) != 0 {
		t.Errorf("命令树里有 %d 条「往口子写」的命令（写权限只给人 · P-101 ①）：%v", len(hits), hits)
	}
	// 负控：喂一条写着「写技能目录」的命令 ⇒ 必红（证明这一格不是恒绿）。
	h2, err := zerg.DoorWriteViolationsRawForTest("skill put", "往技能目录写一件", "zerg skill put <件>", []string{"件"})
	if err != nil {
		t.Fatal(err)
	}
	if len(h2) != 1 {
		t.Errorf("负控失败：写着「写技能目录」的命令没被判红（判出 %d 条）：%v", len(h2), h2)
	}
	// 负控成对：只读命令（含关键词但无写动词）⇒ 不红。
	h3, _ := zerg.DoorWriteViolationsRawForTest("skill ls", "列出技能目录里的件", "zerg skill ls", nil)
	if len(h3) != 0 {
		t.Errorf("负控失败：只读命令被判红了：%v", h3)
	}
}

// ---- 判据③②：doctor 的两段清单（默认不静默）----

func TestDoctorListsDropDoors(t *testing.T) {
	names := zerg.DoctorDoorItemNamesForTest()
	if len(names) != 2 {
		t.Fatalf("doctor 的口子清单 = %v（要两段：技能目录 / MCP 工具清单）", names)
	}
	joined := strings.Join(names, "|")
	for _, want := range []string{"技能目录", "MCP"} {
		if !strings.Contains(joined, want) {
			t.Errorf("doctor 清单里没有 %q：%v", want, names)
		}
	}
	// 端到端：真跑 `zerg doctor --quick --json name,verdict`，「默认不静默」= 这两段真出现在输出里。
	// （`--quick` 跳过要打主控的贵项 ⇒ 测试不碰生产 8580。）
	rc, out, errb := runCapture("doctor", "--quick", "--json", "name,verdict")
	if rc != 0 && rc != 8 {
		t.Fatalf("doctor 退码 = %d（要 0 或 8）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "技能目录") || !strings.Contains(out, "MCP") {
		t.Errorf("doctor 的输出里没有那两段清单（默认不静默 · P-101 ②）：%s", out)
	}
}
