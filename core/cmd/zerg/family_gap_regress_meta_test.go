// family_gap_regress_meta_test.go —— `gap regress` 的 meta **外写自发现**（缺口 `GAP-20261005-44` **同形面**）的**机检**。
//
// 口径（与同族 `gap ls` 的 `family_gap_ls_meta_test.go` / `gap reread` 的 `family_gap_reread_test.go` **同一套** · 只读面）：
//
//	① 无外部写入：T0 快照 sha16 == 退出前重读 sha16 ⇒ `external_write_detected=false`（两 sha16 相等且都印出）
//	② 有外部写入（机检钩子 `gapRegressBetweenReads` 在两次读数之间改账）：两 sha16 不等 ⇒
//	   `external_write_detected=true`（机器面同印）+ 人面出告警行
//	③ 只读：本面一字节都不写真源（不带钩子的那一跑前后真源 sha256 逐字不变）
//
// 负控（哑化）：把实现里的 `gapRegressExtWrite := gapRegressReadbackSHA16 != gapRegressSnapSHA16` 强置 false
// ⇒ 态②不可见（靠 `go test -overlay` 的哑化两态验证：不带 overlay 应绿、带哑化 overlay 应红）。
//
// 落点：`package main` + 直调真实命令树（`RunForTest` ⇒ 与 `main()` 同一个 `run`）—— 需要够得着
// 包级钩子 `gapRegressBetweenReads`（外部测试包 `main_test` 看不见它 ⇒ 本件走包内测试）。
// `--dry-run`：本面只判**读数**，不真跑账里那些判据（干跑只列命令 ⇒ 零副作用）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gapRegressMetaUnit —— 夹具账里用的 `unit`：**必须是仓内真件**（`gap regress <件>` 先做件存在性检查 ⇒
// 不在仓里会被退 2 而跑不到 meta）。用本件所在的这一个真文件，与 `gap regress` 的参数同一枚。
const gapRegressMetaUnit = "core/cmd/zerg/family_gap.go"

// gapRegressMetaFixtureRec —— 造一条合法账行（闭集内的 state/impact/prio · 可跑的 verify_cmd）。
func gapRegressMetaFixtureRec(id string) map[string]any {
	return map[string]any{
		"id": id, "fp": "fp-" + id, "symptom": "夹具症状（只读对拍）", "handmade": "夹具",
		"impact": "命令面", "prio": "P1", "state": "仍缺",
		"want_family": "gap", "want_action": "ls", "repro_cmd": "zerg gap ls", "verify_cmd": "zerg gap ls",
		"found_at": "2026-10-01T00:00:00+08:00", "unit": gapRegressMetaUnit,
	}
}

func gapRegressMetaWriteLedger(t *testing.T, dir string, recs []map[string]any) string {
	t.Helper()
	p := filepath.Join(dir, "zerg-cli-gaps.jsonl")
	var b strings.Builder
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("夹具行序列化失败：%v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写夹具真源失败：%v", err)
	}
	return p
}

func gapRegressMetaFileSHA(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读夹具真源失败：%v", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// gapRegressMetaRun —— 走**真实命令树**（`RunForTest` ⇒ 与 `main()` 同一个 `run`）。
func gapRegressMetaRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// gapRegressMetaOf —— 从包封里取 `meta` 子对象（`--json` 那一格）。
func gapRegressMetaOf(t *testing.T, env string) map[string]any {
	t.Helper()
	var e struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(env), &e); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, env)
	}
	if e.Meta == nil {
		t.Fatalf("包封没有 meta：%q", env)
	}
	return e.Meta
}

func gapRegressMetaStr(t *testing.T, meta map[string]any, key string) string {
	t.Helper()
	v, ok := meta[key]
	if !ok {
		t.Fatalf("meta 缺格 %q（现得：%v）", key, meta)
	}
	s, _ := v.(string)
	return s
}

func TestGapRegressMetaExternalWriteSelfDiscovery(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	recs := []map[string]any{
		gapRegressMetaFixtureRec("GAP-20261005-44-A"),
		gapRegressMetaFixtureRec("GAP-20261005-44-B"),
	}
	ledger := gapRegressMetaWriteLedger(t, dir, recs)
	sha0 := gapRegressMetaFileSHA(t, ledger)
	jsonArgv := []string{"gap", "regress", gapRegressMetaUnit, "--dry-run", "--json", "file"}
	humanArgv := []string{"gap", "regress", gapRegressMetaUnit, "--dry-run"}

	// ① 无外部写入 ⇒ external_write_detected=false（两 sha16 相等且都印出）
	gapRegressBetweenReads = nil
	rc, out, errb := gapRegressMetaRun(t, jsonArgv...)
	if rc != 0 {
		t.Fatalf("无外写态要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	meta := gapRegressMetaOf(t, out)
	if v, ok := meta["external_write_detected"].(bool); !ok || v {
		t.Errorf("无外部写入要报 external_write_detected=false，得到 %v", meta["external_write_detected"])
	}
	snap, back := gapRegressMetaStr(t, meta, "snapshot_sha16"), gapRegressMetaStr(t, meta, "readback_sha16")
	if snap == "" || back == "" || snap != back {
		t.Errorf("无外部写入要两 sha16 相等且都印出：snapshot=%q readback=%q", snap, back)
	}
	if snap != sha0[:16] {
		t.Errorf("snapshot_sha16 要等于夹具真源 sha16：got=%q want=%q", snap, sha0[:16])
	}
	if n, ok := meta["snapshot_lines"].(float64); !ok || int(n) != len(recs) {
		t.Errorf("snapshot_lines 要 %d，得到 %v", len(recs), meta["snapshot_lines"])
	}
	// 既有格 `ledger_sha16` 语义一字未动（仍是现读那一枚 ⇒ 与 readback 同一份）
	if got := gapRegressMetaStr(t, meta, "ledger_sha16"); got != back {
		t.Errorf("既有格 ledger_sha16 要等于现读（readback）：got=%q readback=%q", got, back)
	}
	// 人面（异常行**不许**在无外写态出现 · 正常态逐字节不变）
	if rc, hout, _ := gapRegressMetaRun(t, humanArgv...); rc != 0 || strings.Contains(hout, "外写自发现") {
		t.Errorf("无外写态人面不许出告警行：rc=%d %q", rc, hout)
	}

	// ② 有外部写入 ⇒ external_write_detected=true（两 sha16 不等）
	shaPre := gapRegressMetaFileSHA(t, ledger)
	gapRegressBetweenReads = func() {
		extra := gapRegressMetaFixtureRec("GAP-20261005-44-EXT")
		line, merr := json.Marshal(extra)
		if merr != nil {
			t.Fatalf("夹具行序列化失败：%v", merr)
		}
		f, ferr := os.OpenFile(ledger, os.O_APPEND|os.O_WRONLY, 0o644)
		if ferr != nil {
			t.Fatalf("模拟外部写入失败：%v", ferr)
		}
		_, _ = f.Write(append(line, '\n'))
		_ = f.Close()
	}
	rc, out, errb = gapRegressMetaRun(t, jsonArgv...)
	if rc != 0 {
		gapRegressBetweenReads = nil
		t.Fatalf("有外写态也要 rc=0（**不进退出码**），得到 %d · stderr=%q", rc, errb)
	}
	meta = gapRegressMetaOf(t, out)
	if v, ok := meta["external_write_detected"].(bool); !ok || !v {
		gapRegressBetweenReads = nil
		t.Errorf("有外部写入要报 external_write_detected=true，得到 %v", meta["external_write_detected"])
	}
	snap, back = gapRegressMetaStr(t, meta, "snapshot_sha16"), gapRegressMetaStr(t, meta, "readback_sha16")
	if snap == "" || back == "" {
		gapRegressBetweenReads = nil
		t.Fatalf("机器面要同时印出两个 sha16：%q", out)
	}
	if snap == back {
		gapRegressBetweenReads = nil
		t.Errorf("有外部写入时两 sha16 **必须不等**：snapshot=%q readback=%q", snap, back)
	}
	if snap != shaPre[:16] {
		gapRegressBetweenReads = nil
		t.Errorf("snapshot_sha16 要仍是 T0（外写前）那一份：got=%q want=%q", snap, shaPre[:16])
	}
	// 人面同印告警行（另起一跑 · 钩子仍在 ⇒ 这一跑同样观测到外写）
	rc, hout, _ := gapRegressMetaRun(t, humanArgv...)
	gapRegressBetweenReads = nil
	if rc != 0 || !strings.Contains(hout, "外写自发现") {
		t.Errorf("有外写态人面要出告警行：rc=%d %q", rc, hout)
	}

	// ③ 只读：不带钩子的那一跑，本面一字节都不写真源
	s1 := gapRegressMetaFileSHA(t, ledger)
	if rc, _, _ := gapRegressMetaRun(t, jsonArgv...); rc != 0 {
		t.Errorf("只读那一跑要 rc=0，得到 %d", rc)
	}
	if got := gapRegressMetaFileSHA(t, ledger); got != s1 {
		t.Errorf("本面必须**只读**（真源 sha 变了）：%s → %s", s1, got)
	}
}
