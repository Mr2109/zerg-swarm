// family_dev_reclaim_test.go —— `zerg dev reclaim`（**直写补正门** · 缺口 `GAP-20261010-30`）的判据。
//
// 四格（全部在合成仓根 + 合成状态目录里跑 · ✗ 不碰真仓的件）：
//
//	① 真跑（人签齐）：被非正门直写的件 ⇒ 补正后**盘上内容逐字节不变**、件级账**新增一行**且
//	   新行 `before` = 审计末行 after（真前态）· `after` = 补正前盘上 sha；
//	② 审计链已完整（末行 after 逐字＝现盘）⇒ **无需补正**（退 0）且**不新增审计行**（成对：证明 ① 不是「一律写」）；
//	③ 负控：真前态取不到（末行 after 与 git HEAD 不符且无 `--pre`）⇒ 拒并点名（退 2），盘上件一个字节不动；
//	④ 负控：`--pre` 交底件 sha 与末行 after 不符 ⇒ 拒（退 2）。
package main_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

func reclaimSHA(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读不了 %s：%v", p, err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func reclaimGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	full := append([]string{"-C", repo, "-c", "user.email=a@b", "-c", "user.name=t"}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v：%v · %s", args, err, out)
	}
}

// reclaimSynth 合成：git 仓 + 一件已被非正门直写的件 + 一条件级账（末行 after = 真前态）+ 提案 + 人签批准件。
// 返回 (仓根, 状态目录, 提案目录, 件相对路径, 真前态 sha, 补正前盘上 sha)。
func reclaimSynth(t *testing.T) (string, string, string, string, string, string) {
	t.Helper()
	repo, target := synthRepoForEdit(t)
	state, prop := t.TempDir(), t.TempDir()
	tp := filepath.Join(repo, target)
	mustWrite(t, tp, "package main\n\n// PRE\n")
	reclaimGit(t, repo, "init", "-q")
	reclaimGit(t, repo, "add", "-A")
	reclaimGit(t, repo, "commit", "-qm", "base")
	preSHA := reclaimSHA(t, tp)
	// 直写：盘上改成 AFTER（不在任何审计行里 ⇒ 审计链断在 PRE）
	mustWrite(t, tp, "package main\n\n// AFTER\n")
	diskSHA := reclaimSHA(t, tp)
	line := map[string]string{"event": "edit", "file": target,
		"after_sha256": preSHA, "before_sha256": strings.Repeat("0", 64)}
	b, _ := json.Marshal(line)
	mustWrite(t, filepath.Join(state, "edit_audit.jsonl"), string(b)+"\n")
	writeEditProposal(t, prop, "DEV-9001", target)
	sign := writeOperatorKey(t, state)
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "夹具", "2026-10-10T00:00:00+08:00", "补正链"))+"\n")
	return repo, state, prop, target, preSHA, diskSHA
}

func reclaimRun(t *testing.T, repo, prop, state string, argv ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", repo)
	t.Setenv("ZERG_PROPOSAL_DIR", prop)
	t.Setenv("ZERG_STATE_DIR", state)
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// reclaimLastLine 读件级账里该件的**最后一条** edit 行（取 after_sha256 / before_sha256 两格）。
func reclaimLastLine(t *testing.T, state, target string) (map[string]string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(state, "edit_audit.jsonl"))
	if err != nil {
		t.Fatalf("读不了件级账：%v", err)
	}
	out := map[string]string{}
	found := false
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(ln), &m) != nil {
			continue
		}
		if m["event"] != "edit" || m["file"] != target {
			continue
		}
		for _, k := range []string{"after_sha256", "before_sha256"} {
			if s, ok := m[k].(string); ok {
				out[k] = s
			} else {
				out[k] = ""
			}
		}
		found = true
	}
	return out, found
}

// ① 真跑：净零重落 —— 盘上内容逐字节不变 · 审计新增一行（before=真前态 · after=补正前盘上 sha）。
func TestDevReclaim_NetZeroReLandFixesAuditChain(t *testing.T) {
	repo, state, prop, target, preSHA, diskSHA := reclaimSynth(t)
	rc, out, errb := reclaimRun(t, repo, prop, state, "dev", "reclaim", target,
		"--proposal", "DEV-9001", "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("补正应退 0，得到 %d · out=%s · err=%s", rc, out, errb)
	}
	if got := reclaimSHA(t, filepath.Join(repo, target)); got != diskSHA {
		t.Fatalf("核 sha 逐字同失败：盘上 %s ≠ 补正前 %s（内容被动了）", got, diskSHA)
	}
	last, ok := reclaimLastLine(t, state, target)
	if !ok {
		t.Fatalf("件级账里没有 %s 的 edit 行", target)
	}
	if last["after_sha256"] != diskSHA {
		t.Errorf("新行 after 应 = 补正前盘上 sha %s，得 %s", diskSHA, last["after_sha256"])
	}
	if last["before_sha256"] != preSHA {
		t.Errorf("新行 before 应 = 真前态（审计末行 after）%s，得 %s", preSHA, last["before_sha256"])
	}
}

// ② 审计链已完整（末行 after 逐字＝现盘）⇒ 无需补正 · 退 0 · **不新增审计行**。
func TestDevReclaim_NoopWhenChainIntact(t *testing.T) {
	repo, state, prop, target, preSHA, _ := reclaimSynth(t)
	// 把盘上还原成与末行 after 一致（无直写）
	mustWrite(t, filepath.Join(repo, target), "package main\n\n// PRE\n")
	rc, out, errb := reclaimRun(t, repo, prop, state, "dev", "reclaim", target,
		"--proposal", "DEV-9001", "--confirm="+hostnameOf(t), "--yes")
	if rc != 0 {
		t.Fatalf("链已完整应退 0，得到 %d · err=%s", rc, errb)
	}
	last, _ := reclaimLastLine(t, state, target)
	if last["after_sha256"] != preSHA {
		t.Errorf("链已完整 ⇒ 不该新增行：末行 after 仍是 %s", last["after_sha256"])
	}
	if b, _ := os.ReadFile(filepath.Join(state, "edit_audit.jsonl")); strings.Count(string(b), "\n") != 1 {
		t.Errorf("链已完整 ⇒ 审计只该有原来那一行：%q · out=%s", string(b), out)
	}
}

// ③ 负控：真前态取不到（末行 after 与 git HEAD 不符且无 `--pre`）⇒ 拒并点名 · 件一个字节不动。
func TestDevReclaim_RefusesWhenPreStateUnresolvable(t *testing.T) {
	repo, state, prop, target, _, diskSHA := reclaimSynth(t)
	// 把末行 after 改成一个 git HEAD 里没有的 sha
	bad := strings.Repeat("de", 32)
	line := map[string]string{"event": "edit", "file": target, "after_sha256": bad, "before_sha256": strings.Repeat("0", 64)}
	b, _ := json.Marshal(line)
	mustWrite(t, filepath.Join(state, "edit_audit.jsonl"), string(b)+"\n")
	rc, _, errb := reclaimRun(t, repo, prop, state, "dev", "reclaim", target,
		"--proposal", "DEV-9001", "--confirm="+hostnameOf(t), "--yes")
	if rc != 2 {
		t.Fatalf("真前态取不到 ⇒ 应退 2，得到 %d · err=%s", rc, errb)
	}
	if !strings.Contains(errb, "审计末行 after 不符") {
		t.Errorf("要逐字点名「不符」：%q", errb)
	}
	if got := reclaimSHA(t, filepath.Join(repo, target)); got != diskSHA {
		t.Fatalf("被拒的那一格里件一个字节都不许动：%s ≠ %s", got, diskSHA)
	}
}

// ④ 负控：`--pre` 交底件 sha 与末行 after 不符 ⇒ 拒（退 2）。
func TestDevReclaim_RefusesWhenPreSHAOff(t *testing.T) {
	repo, state, prop, target, _, _ := reclaimSynth(t)
	wrong := filepath.Join(t.TempDir(), "wrong.pre")
	mustWrite(t, wrong, "package main\n\n// WRONG\n")
	rc, _, errb := reclaimRun(t, repo, prop, state, "dev", "reclaim", target,
		"--proposal", "DEV-9001", "--pre", target+"="+wrong,
		"--confirm="+hostnameOf(t), "--yes")
	if rc != 2 || !strings.Contains(errb, "`--pre` 件") {
		t.Fatalf("`--pre` sha 不符 ⇒ 应退 2 并点名，得到 %d · err=%s", rc, errb)
	}
}
