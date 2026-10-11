// cli_dev_edit_returnpoint_land_test.go —— ★ 退点前置（任务单 退-1·拍-6）回归牙。
//
// 判据（四态）：
//
//	① 正常档：真写 ⇒ 改前整件落 `<状态目录>/退点归档-<日期>/<件名>`（逐字节同改前）＋ 同目录落一枚
//	   可执行退点档 `<件名>.恢复.sh`（含 `cp -p <退点件> <原路径>` 与 sha256 断言）＋ 审计 append；
//	② fail-closed 档：退点落点被占住（建不出）⇒ **拒写** —— rc≠0 · 判词含「退点没落下 ⇒ 拒写」·
//	   目标件一字节不动 · 审计不落；
//	③ 可执行档：拿退点档里的恢复命令真跑 ⇒ 目标件 sha256 回到 `before_sha256`（空前态档 = `rm`）；
//	④ 不落下则不改：同 ②（sha256 前后比对）。
//
// 全部夹具在 t.TempDir() 的合成仓根 / 合成状态目录里跑（不碰真仓 · 不碰真 ~/.zerg/state）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// devEditArchiveFile —— 退点档落点（与 `family_dev_edit.go` 的 `devEditArchiveDir` / `devEditSafeName` 同口径）。
func devEditArchiveFile(state, target string) string {
	safe := strings.ReplaceAll(filepath.ToSlash(target), "/", "__")
	return filepath.Join(state, "退点归档-"+time.Now().Format("20060102"), safe)
}

// TestDevEdit_ReturnPointLandAndRestore —— ① 正常档 ＋ ③ 可执行档。
func TestDevEdit_ReturnPointLandAndRestore(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9401", target)
	sign := writeOperatorKey(t, state)
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-10-11T00:00:00+08:00", "放行本件"))+"\n")
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 改后\n")
	beforeBytes, err := os.ReadFile(filepath.Join(repo, target))
	if err != nil {
		t.Fatal(err)
	}
	beforeSHA := sha256hex(beforeBytes)

	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9401", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("正常档真写 ⇒ 退 0，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, "退点前置") {
		t.Errorf("真写档要点名退点前置已落：%q", errb)
	}

	// ①-a 退点件落盘、逐字节同改前。
	archive := devEditArchiveFile(state, target)
	got, err := os.ReadFile(archive)
	if err != nil {
		t.Fatalf("退点件没落下（%s）：%v", archive, err)
	}
	if sha256hex(got) != beforeSHA {
		t.Errorf("退点件要与改前整件逐字节相同：got=%s want=%s", sha256hex(got), beforeSHA)
	}
	// ①-b 可执行退点档：含恢复命令 + 断言。
	restore := archive + ".恢复.sh"
	rb, err := os.ReadFile(restore)
	if err != nil {
		t.Fatalf("可执行退点档没落下（%s）：%v", restore, err)
	}
	if !strings.Contains(string(rb), "cp -p") || !strings.Contains(string(rb), beforeSHA) {
		t.Errorf("可执行退点档要带 `cp -p` 与 sha256 断言：%q", rb)
	}
	// ①-c 审计 append。
	ab, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		t.Fatalf("真写要落审计：%v", err)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(ab))), &line); err != nil {
		t.Fatalf("审计不是一行 JSON：%v", err)
	}
	if line["before_sha256"] != beforeSHA {
		t.Errorf("审计 before_sha256 要 == 改前指纹：%v want %s", line["before_sha256"], beforeSHA)
	}

	// ③ 可执行档：真跑恢复命令（目标件在合成仓根里 = 仓外临时副本）⇒ sha256 回到 before。
	if o, err := exec.Command("sh", restore).CombinedOutput(); err != nil {
		t.Fatalf("退点档恢复命令跑不动：%v · %s", err, o)
	}
	back, _ := os.ReadFile(filepath.Join(repo, target))
	if sha256hex(back) != beforeSHA {
		t.Errorf("恢复后 sha256 要 == 改前指纹：got=%s want=%s", sha256hex(back), beforeSHA)
	}
}

// TestDevEdit_ReturnPointNewFileRestore —— 空前态：恢复档给 `rm` 那条，跑完该件不存在。
func TestDevEdit_ReturnPointNewFileRestore(t *testing.T) {
	repo, _ := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	target := filepath.Join("core", "cmd", "zerg", "brand_new.go")
	writeEditProposal(t, prop, "DEV-9402", target)
	sign := writeOperatorKey(t, state)
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-10-11T00:00:00+08:00", "放行本件"))+"\n")
	from := filepath.Join(t.TempDir(), "n.go")
	mustWrite(t, from, "package main\n\n// 新件\n")

	rc, _, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9402", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("新件真写 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	restore := devEditArchiveFile(state, target) + ".恢复.sh"
	rb, err := os.ReadFile(restore)
	if err != nil {
		t.Fatalf("新件的可执行退点档没落下：%v", err)
	}
	if !strings.Contains(string(rb), "rm -f") {
		t.Errorf("空前态恢复档要给 `rm` 那条：%q", rb)
	}
	if _, err := os.Stat(filepath.Join(repo, target)); err != nil {
		t.Fatalf("新件应已写上盘：%v", err)
	}
	if o, err := exec.Command("sh", restore).CombinedOutput(); err != nil {
		t.Fatalf("空前态恢复命令跑不动：%v · %s", err, o)
	}
	if _, err := os.Stat(filepath.Join(repo, target)); !os.IsNotExist(err) {
		t.Errorf("空前态恢复后该件应不存在：err=%v", err)
	}
}

// TestDevEdit_ReturnPointFailClosedRefusesWrite —— ② fail-closed ＋ ④ 不落下则不改。
func TestDevEdit_ReturnPointFailClosedRefusesWrite(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9403", target)
	sign := writeOperatorKey(t, state)
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-10-11T00:00:00+08:00", "放行本件"))+"\n")
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 改后\n")
	beforeBytes, _ := os.ReadFile(filepath.Join(repo, target))
	beforeSHA := sha256hex(beforeBytes)

	// 造「退点落不下」：把退点归档落点占成一个**普通件** ⇒ `MkdirAll` 建不出目录（状态目录本身仍可写，
	// 故审计这一道不受影响 —— 单独证明退点前置这颗牙）。
	blocker := filepath.Join(state, "退点归档-"+time.Now().Format("20060102"))
	mustWrite(t, blocker, "占位件\n")

	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9403", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	if rc == 0 {
		t.Fatalf("退点落不下 ⇒ 必须拒写（fail-closed），却退 0 · stdout=%s · stderr=%s", out, errb)
	}
	if !strings.Contains(errb, "退点没落下 ⇒ 拒写") {
		t.Errorf("判词要说清「退点没落下 ⇒ 拒写」：%q", errb)
	}
	// ④ 拒写时目标件一字未动。
	after, _ := os.ReadFile(filepath.Join(repo, target))
	if sha256hex(after) != beforeSHA {
		t.Errorf("拒写时目标件不许动：got=%s want=%s", sha256hex(after), beforeSHA)
	}
	if !strings.Contains(string(after), "// 原件") {
		t.Errorf("拒写时目标件仍是原件：%q", after)
	}
	// 拒写 ⇒ 审计也不落。
	if _, err := os.Stat(filepath.Join(state, "edit_audit.jsonl")); err == nil {
		t.Errorf("退点前置拒写 ⇒ 连审计都不该落（没写就不记账）")
	}
}
