package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// cli_doctor_drift_test.go —— ⑦-b「源码 ↔ 制品两代对拍」（缺口 `GAP-20260927-410`）三态负/正控。
//
// 夹具 = **真文件**的临时 git 仓（`t.TempDir()` + `git init` · 不用 symlink 影子根、不用 rmtree）。
// 制品身份用 `version.Commit` 这个 **var** 直接换（`-ldflags -X` 的注入点就是它）⇒ 三态都能造。

// driftFixture —— 造真仓：c1(a.txt) → c2(b.txt)，返回仓根 + 两笔 sha。
func driftFixture(t *testing.T) (root, first, head string) {
	t.Helper()
	root = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		full := append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
		out, err := exec.Command("git", full...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v 失败：%v · %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-q", "-m", "c1")
	first = git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "b.txt")
	git("commit", "-q", "-m", "c2")
	head = git("rev-parse", "HEAD")
	return root, first, head
}

func TestDoctorDriftThreeStates(t *testing.T) {
	root, first, head := driftFixture(t)
	t.Setenv("ZERG_REPO", root)
	old := version.Commit
	t.Cleanup(func() { version.Commit = old })

	// ① 干净：制品 id == 源码 HEAD ⇒ PASS「未过期」
	version.Commit = head
	it := doctorDriftItem()
	if it["verdict"] != "PASS" || !strings.Contains(it["detail"], "未过期") {
		t.Errorf("① 干净态：verdict=%q detail=%q（要 PASS 且含「未过期」）", it["verdict"], it["detail"])
	}

	// ② 过期：制品 id = 第一笔 ⇒ REPORT「过期」且点名改过的 `b.txt`
	version.Commit = first
	it = doctorDriftItem()
	if it["verdict"] != "REPORT" || !strings.Contains(it["detail"], "过期") {
		t.Errorf("② 过期态：verdict=%q detail=%q（要 REPORT 且含「过期」）", it["verdict"], it["detail"])
	}
	if !strings.Contains(it["detail"], "b.txt") {
		t.Errorf("② 过期态没列出改动件（要含 b.txt）：%s", it["detail"])
	}

	// ②-b 未提交的工作树也算「改过」：HEAD 上未注入 id 前置，先造一个未提交件再判
	if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	it = doctorDriftItem()
	if !strings.Contains(it["detail"], "c.txt") {
		t.Errorf("②-b 未提交件（c.txt）没进对拍：%s", it["detail"])
	}

	// ②-c 截短同形（缺口 `GAP-20260928-60`）：真差异在第八位之后 / 制品身份带 `+dirty` 后缀 ⇒
	// 判等**仍旧按全串**（REPORT「过期」，退码不动），但出口**不许**印出两侧同值的 `X ≠ X`。
	version.Commit = head[:8] + "+dirty"
	it = doctorDriftItem()
	if it["verdict"] != "REPORT" || !strings.Contains(it["detail"], "过期") {
		t.Errorf("②-c 截短同形态：verdict=%q detail=%q（要 REPORT 且含「过期」，判等不许变）", it["verdict"], it["detail"])
	}
	if strings.Contains(it["detail"], driftShort(head)+" ≠ "+driftShort(head)) {
		t.Errorf("②-c 出口印出了两侧同值的「不等」行（判词与显示自相矛盾）：%s", it["detail"])
	}
	if !strings.Contains(it["detail"], "+dirty") {
		t.Errorf("②-c 制品身份的真差异（`+dirty` 后缀）在出口不可见：%s", it["detail"])
	}

	// ③ 拿不到结论：`version.Commit` 未注入（裸 go build / 进程内测试的形态）⇒ REPORT 明说拿不到
	version.Commit = "unknown"
	it = doctorDriftItem()
	if it["verdict"] != "REPORT" || !strings.Contains(it["detail"], "拿不到结论") {
		t.Errorf("③ 无 id 态：verdict=%q detail=%q（要 REPORT 且含「拿不到结论」）", it["verdict"], it["detail"])
	}

	// ③-b 仓根取不到 ⇒ 也是「拿不到结论」（不猜路径）
	version.Commit = head
	t.Setenv("ZERG_REPO", "")
	// 清掉环境后 repoRoot 会按可执行文件/当前目录上溯；测试进程所在目录注定不是仓根时才会空。
	// 这里只断言：无论空不空，都**不许**给出正文之外的结论（PASS/FAIL 不能凭猜）。
	it = doctorDriftItem()
	if it["verdict"] == "FAIL" {
		t.Errorf("③-b 真源取不到时不许判 FAIL（不许凭猜）：%+v", it)
	}
}

// TestDoctorDriftJSONFullFingerprints —— 缺口 `GAP-20260928-60` 的另一半：**机读面拿到两侧完整指纹原文**。
//
// 上一条用例（`TestDoctorDriftThreeStates`）治的是**显示层**（截短后同形时把窗口加长）。
// 本条治**机读面**：人面怎么掐位不变，但 `--json` 必须能拿到两侧**未截断**的原文 ——
// 消费方要能自己判「制品身份带 +dirty / 差异落在第九位之后」这类真差异。
//
// 三断言：① 同值态两侧都等于源码 HEAD 全串 · ② 真不同态两串必须不同且都不截断 · ③ 端到端点这两键
// **不许**落「未知字段」（`emitSelected` 逐行核键 ⇒ 别的行也要有键，否则整单退 2）。
func TestDoctorDriftJSONFullFingerprints(t *testing.T) {
	root, _, head := driftFixture(t)
	t.Setenv("ZERG_REPO", root)
	old := version.Commit
	t.Cleanup(func() { version.Commit = old })

	// ① 同值态：制品身份 == 源码 HEAD ⇒ 两侧原文都等于 HEAD 全串（40 位，未截断）
	version.Commit = head
	it := doctorDriftItem()
	if it["artifact_commit"] != head || it["source_head"] != head {
		t.Errorf("① 同值态机读面：artifact_commit=%q source_head=%q（要都等于源码 HEAD 全串 %q）",
			it["artifact_commit"], it["source_head"], head)
	}

	// ② 真不同态：制品身份带 `+dirty` ⇒ 机读面两串**必须不同**，且两侧都是原文（不截断）
	version.Commit = head[:8] + "+dirty"
	it = doctorDriftItem()
	if it["artifact_commit"] == it["source_head"] {
		t.Errorf("② 真不同态机读面两串同值（拿不到差异原文）：%q", it["artifact_commit"])
	}
	if it["source_head"] != head {
		t.Errorf("② source_head 不是源码 HEAD 全串（被截断或串错）：%q", it["source_head"])
	}
	if it["artifact_commit"] != head[:8]+"+dirty" {
		t.Errorf("② artifact_commit 不是制品身份原文（`+dirty` 后缀丢了或被截短）：%q", it["artifact_commit"])
	}

	// ③ 端到端：旧字段一个不删、新字段点得到 ⇒ rc 不许是 2（「未知字段」），包封里要真出现两键
	// （落点在本包：`runCapture` 在 main_test 包里够不着 ⇒ 这里直接走只读桥 `RunForTest`。）
	var out, errb bytes.Buffer
	rc := RunForTest([]string{"doctor", "--quick", "--json",
		"name,verdict,detail,advice,artifact_commit,source_head"}, &out, &errb)
	if rc == 2 {
		t.Fatalf("③ `--json` 点这两键被判未知字段（机读面够不着）：rc=%d stderr=%s", rc, errb.String())
	}
	for _, k := range []string{"artifact_commit", "source_head"} {
		if !strings.Contains(out.String(), k) {
			t.Errorf("③ `--json` 面里没有 %s 键：%s", k, out.String())
		}
	}
}
