// unresolved_entries_anchor_drift_test.go —— 判据① 的**漂移免疫**正控与成对负控（口径 v2 · 2026-09-22）。
//
// 本项要的那个性质只有一句话：**证据不许按字面行号钉**。
// 于是正控不能只是「再跑一遍现仓」——它必须直接打「别人在证据件前面插行」这一格（本轮元凶那类提交）：
//
//	正控 A（现仓现跑）    ：真证据件 + 真台账 ⇒ 判定口 0 err（每条锚命中唯一）
//	正控 B（漂移免疫）    ：把「证据面」（7 个入口目录 + main.go + 24 条证据件）原样拷进 /tmp 一份副本，
//	                       在副本的 `scripts/build/build-all.sh` **头部插一行注释**（= 复现本轮元凶那类提交），
//	                       **以新副本现跑** ⇒ 判定口**仍 0 err**（只在警告里记「行号提示漂了」）
//	对照（旧口径必红）    ：同一对副本（原样 / 头部插一行）喂**已废的 v1 行号口径** ⇒ 原样绿、插一行必红
//	                       —— 这就是「只挪数字 = 治标」的机检：不改口径，插一行就还会红
//	负控①（锚不存在）    ：副本里把锚串改掉（证据件那一句被改/被搬走）⇒ 必红，错因逐字含「锚不存在」
//	负控②（锚不唯一）    ：副本里把锚复制成第二份 ⇒ 必红，错因逐字含「锚不唯一（命中 2 处：行 …）」
//	                       · ②-a 单行锚（README 的 `zerg-scheduler` 登记行）复制成两行
//	                       · ②-b 上下文窗锚（compat 241+242 两行）整块复制 ⇒ 窗口锚照同一条规则判，不静默取第一处
//
// ★ 副本**只读真件、不写真仓**；跑完**不删**（中间产物留档 —— 红线「不删件」）：
//
//	用例把每份副本的路径打进日志，人要跟一遍就直接照路径看。
//
// ★ 副本有效性**逐件机检**：拷贝后逐件比对 sha256 与真件相同（否则「副本仍绿」这句话是空的）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// oldLinePinnedErrs —— **已废的 v1 行号口径**（`file` + `line`，那一行必须含入口名），只作对照用。
// 保留它只为一条断言：同一对副本（原样 / 头部插一行）在旧口径下 **绿→红** —— 那正是本项要治的病。
// 它不进任何判据：真实判据只有 judgeUnresolved（内容锚）。
func oldLinePinnedErrs(root string, led *contract.UnresolvedLedger) []error {
	var errs []error
	for _, e := range led.Entries {
		for _, c := range e.Callers {
			if c.LineHint <= 0 {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, c.File))
			if err != nil {
				errs = append(errs, fmt.Errorf("旧口径：证据件读不到 %s（%v）", c.File, err))
				continue
			}
			lines := strings.Split(string(b), "\n")
			if c.LineHint > len(lines) {
				errs = append(errs, fmt.Errorf("旧口径：行号越界 %s:%d（该件 %d 行）", c.File, c.LineHint, len(lines)))
				continue
			}
			if !strings.Contains(lines[c.LineHint-1], e.Bin) {
				errs = append(errs, fmt.Errorf("旧口径 破：%s:%d 里没有 %q（那一行现在是 %q）—— "+
					"在件头插过行的提交就会把它打红", c.File, c.LineHint, e.Bin, strings.TrimSpace(lines[c.LineHint-1])))
			}
		}
	}
	return errs
}

// sha256Of —— 件内容的 sha256（副本有效性靠它机检）。
func sha256Of(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读不到 %s：%v", path, err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// evidenceFaceFiles —— 「证据面」的件清单（7 个入口目录 + main.go + 每条证据件；去重后按序）。
func evidenceFaceFiles(led *contract.UnresolvedLedger) (files []string, dirs []string) {
	seenF, seenD := map[string]bool{}, map[string]bool{}
	for _, e := range led.Entries {
		if !seenD[e.Dir] {
			seenD[e.Dir] = true
			dirs = append(dirs, e.Dir)
		}
		for _, rel := range []string{e.Main} {
			if !seenF[rel] {
				seenF[rel] = true
				files = append(files, rel)
			}
		}
		for _, c := range e.Callers {
			if !seenF[c.File] {
				seenF[c.File] = true
				files = append(files, c.File)
			}
		}
	}
	return files, dirs
}

// newAnchorSandbox —— 把「证据面」原样拷进 /tmp 一份副本，返回副本根。
// **不删**（跑完留着当证据）：每次调用一个新目录，路径由调用方打进日志。
func newAnchorSandbox(t *testing.T, repo string, led *contract.UnresolvedLedger) string {
	t.Helper()
	sb, err := os.MkdirTemp("/tmp", "zerg-anchor-drift-")
	if err != nil {
		t.Fatalf("/tmp 副本建不出来：%v", err)
	}
	files, dirs := evidenceFaceFiles(led)
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(sb, d), 0o755); err != nil {
			t.Fatalf("副本目录建不出来 %s：%v", d, err)
		}
	}
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(repo, rel))
		if err != nil {
			t.Fatalf("真件读不到 %s：%v", rel, err)
		}
		dst := filepath.Join(sb, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("副本目录建不出来 %s：%v", filepath.Dir(rel), err)
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			t.Fatalf("副本写不进去 %s：%v", rel, err)
		}
	}
	// 副本有效性**逐件机检**：sha256 与真件逐条相同（否则「副本仍绿」是空话）
	for _, rel := range files {
		if a, b := sha256Of(t, filepath.Join(repo, rel)), sha256Of(t, filepath.Join(sb, rel)); a != b {
			t.Fatalf("副本与真件不一致 %s：%s ≠ %s", rel, a, b)
		}
	}
	t.Logf("副本：%s ⇒ %d 个入口目录 + %d 个证据件（逐件 sha256 与真件相同）", sb, len(dirs), len(files))
	return sb
}

// rewriteSandboxFile —— 在副本上改件（只动副本；真仓一个字节不碰）。返回改前/改后的 sha256。
func rewriteSandboxFile(t *testing.T, sb, rel, old, new string) (string, string) {
	t.Helper()
	p := filepath.Join(sb, rel)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("副本件读不到 %s：%v", rel, err)
	}
	before := sha256.Sum256(b)
	if !strings.Contains(string(b), old) {
		t.Fatalf("副本件 %s 里没有要改的目标串（改了就是改史）：%q", rel, old)
	}
	out := strings.Replace(string(b), old, new, 1)
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		t.Fatalf("副本件写不回去 %s：%v", rel, err)
	}
	after := sha256.Sum256([]byte(out))
	return hex.EncodeToString(before[:8]), hex.EncodeToString(after[:8])
}

// errsText / oneErr —— 负控断言的两把尺：错因原文、且错只该落在那一格（不许一片红）。
func errsText(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, " | ")
}

func hitFor(v unresolvedVerdict, entryID, file string) (anchorHit, bool) {
	for _, h := range v.Hits {
		if h.EntryID == entryID && h.File == file {
			return h, true
		}
	}
	return anchorHit{}, false
}

// TestUnresolvedEvidenceAnchorsAreDriftImmune —— 本项的正控（两条）+ 成对负控（两格）+ 旧口径对照。
func TestUnresolvedEvidenceAnchorsAreDriftImmune(t *testing.T) {
	repo := repoRootFromCLI(t)
	led, err := contract.Unresolved()
	if err != nil {
		t.Fatalf("取出台账读不出来：%v", err)
	}
	const bb = "scripts/build/build-all.sh"

	// ── 正控 A：现仓现跑（真件 + 真台账） ⇒ 0 err
	if v := judgeUnresolved(repo, *led, nil); !v.ok() {
		t.Fatalf("正控 A（现仓现跑）失败：%v", v.Errs)
	} else {
		t.Logf("正控 A：现仓现跑 ⇒ 0 err · %d 条证据（锚命中唯一）· %d 条提示警告", len(v.Hits), len(v.Warns))
	}

	// ── 副本一：原样（判据② 在副本里也照查：入口目录 + main.go 都在）
	sb0 := newAnchorSandbox(t, repo, led)
	if v := judgeUnresolved(sb0, *led, nil); !v.ok() {
		t.Fatalf("副本（原样）现跑本该绿，实测红：%v", v.Errs)
	}
	base := oldLinePinnedErrs(sb0, led)
	if len(base) != 0 {
		t.Fatalf("对照基线不正：**旧口径**在「原样副本」上本该绿（它现在只是提示），实测红：%v", base)
	}
	t.Logf("副本（原样）：新口径 0 err · 旧口径（v1 行号）0 err ⇒ 两者在「没插行」时看不出差别")

	// ── 正控 B：副本头部插一行注释 ⇒ 新口径**仍绿**，旧口径**必红**
	sb1 := newAnchorSandbox(t, repo, led)
	oldLines := strings.Count(mustRead(t, filepath.Join(sb1, bb)), "\n") + 1
	b0, b1 := rewriteSandboxFile(t, sb1, bb, "#!/usr/bin/env bash",
		"#!/usr/bin/env bash\n# 漂移正控：在件头插一行注释（本项要治的正是这种提交）")
	if strings.Count(mustRead(t, filepath.Join(sb1, bb)), "\n")+1 != oldLines+1 {
		t.Fatalf("正控 B 的前置动作没生效：件行数没 +1")
	}
	t.Logf("正控 B：副本 %s 的 %s 头部插了一行注释（sha256 前 8 位 %s→%s · 行数 %d→%d）",
		sb1, bb, b0, b1, oldLines, oldLines+1)

	v := judgeUnresolved(sb1, *led, nil)
	if !v.ok() {
		t.Fatalf("正控 B 失败：在证据件头部插一行之后判据**不该红**，实测红：%v", v.Errs)
	}
	hit, ok := hitFor(v, "compat", bb)
	if !ok {
		t.Fatalf("正控 B：副本上找不到 compat · %s 的锚命中记录", bb)
	}
	t.Logf("正控 B：判据仍绿 · compat 锚现起行 %d（台账提示 %d）· 跨 %d 行 · 警告 %d 条",
		hit.StartLine, hit.LineHint, hit.SpanLines, len(v.Warns))
	if len(v.Warns) == 0 {
		t.Error("正控 B：锚跟着文件走了，但「行号提示漂了」一条警告都没有（提示会悄悄失效）")
	}
	for _, w := range v.Warns {
		t.Logf("  正控 B 警告 · %v", w)
	}
	if old := oldLinePinnedErrs(sb1, led); len(old) == 0 {
		t.Error("对照失败：**旧口径**在同一份「头部插一行」的副本上本该红（这正是它被废的原因），实测绿")
	} else {
		for _, e := range old {
			t.Logf("  对照（旧口径）红 · %v", e)
		}
		if !strings.Contains(errsText(old), "build-all.sh:241") {
			t.Errorf("对照失败：旧口径该点名 compat 那条（build-all.sh:241），实测错因：%s", errsText(old))
		}
	}

	// ── 负控①：锚不存在（副本里把锚串改掉 = 证据件那一句被改/被搬走）⇒ 必红 + 错因逐字
	sb2 := newAnchorSandbox(t, repo, led)
	c0, c1 := rewriteSandboxFile(t, sb2, bb, "--only-compat）\"", "--only-compat-DISABLED）\"")
	t.Logf("负控①：副本 %s 的 %s 里把锚串 `--only-compat）\"` 改掉（sha256 前 8 位 %s→%s）", sb2, bb, c0, c1)
	v1 := judgeUnresolved(sb2, *led, nil)
	if v1.ok() {
		t.Fatal("负控① 失败：锚不存在没有被抓到（证据挂在空气上还判绿）")
	}
	if len(v1.Errs) != 1 || !strings.Contains(errsText(v1.Errs), "锚不存在") || !strings.Contains(errsText(v1.Errs), "compat") {
		t.Errorf("负控① 失败：要「恰好一条、点到 compat、错因写「锚不存在」」，实测 %d 条：%s",
			len(v1.Errs), errsText(v1.Errs))
	}
	t.Logf("负控① 红 · 错因 %v", v1.Errs)
	if len(oldLinePinnedErrs(sb2, led)) == 0 {
		t.Log("（附注：旧口径在这一格也可能不红 —— 它只看那一行在不在，不看那句话还在不在；这正是第二处病）")
	}

	// ── 负控②-a：单行锚被复制成两行（不唯一）⇒ 必红 + 错因逐字 + 点名命中行号
	sb3 := newAnchorSandbox(t, repo, led)
	const schedRow = "| zerg-scheduler | 调度器 |"
	rewriteSandboxFile(t, sb3, "README.md", schedRow, schedRow+"\n"+schedRow)
	t.Logf("负控②-a：副本 %s 的 README.md 里把 `%s` 复制成两行（单行锚 ⇒ 不唯一）", sb3, schedRow)
	v2 := judgeUnresolved(sb3, *led, nil)
	if v2.ok() {
		t.Fatal("负控②-a 失败：锚不唯一没有被抓到（静默取了第一处）")
	}
	if len(v2.Errs) != 1 || !strings.Contains(errsText(v2.Errs), "锚不唯一") ||
		!strings.Contains(errsText(v2.Errs), "scheduler") || !strings.Contains(errsText(v2.Errs), "命中 2 处") {
		t.Errorf("负控②-a 失败：要「恰好一条、点到 scheduler、错因写「锚不唯一（命中 2 处：行 …）」」，实测 %d 条：%s",
			len(v2.Errs), errsText(v2.Errs))
	}
	t.Logf("负控②-a 红 · 错因 %v", v2.Errs)

	// ── 负控②-b：**上下文窗锚**整块复制（不唯一）⇒ 同样必红（窗口不是特例）
	sb4 := newAnchorSandbox(t, repo, led)
	win, ok := anchorOf(led, "compat", bb)
	if !ok || !strings.Contains(win, "\n") {
		t.Fatalf("负控②-b 前置不成立：compat · %s 的台账锚不是上下文窗（%q）", bb, win)
	}
	rewriteSandboxFile(t, sb4, bb, win, win+"\n"+win)
	t.Logf("负控②-b：副本 %s 的 %s 里把**两行上下文窗**锚整块复制（跨 %d 行 ⇒ 不唯一）", sb4, bb,
		strings.Count(win, "\n")+1)
	v3 := judgeUnresolved(sb4, *led, nil)
	if v3.ok() {
		t.Fatal("负控②-b 失败：窗口锚不唯一没有被抓到")
	}
	if len(v3.Errs) != 1 || !strings.Contains(errsText(v3.Errs), "锚不唯一") || !strings.Contains(errsText(v3.Errs), "compat") {
		t.Errorf("负控②-b 失败：要「恰好一条、点到 compat、错因写「锚不唯一」」，实测 %d 条：%s", len(v3.Errs), errsText(v3.Errs))
	}
	t.Logf("负控②-b 红 · 错因 %v", v3.Errs)

	t.Logf("副本一览（**跑完不删**，可自行跟一遍）：原样 %s · 头部插一行 %s · 锚改掉 %s · 单行锚复制 %s · 窗口锚复制 %s",
		sb0, sb1, sb2, sb3, sb4)
	t.Logf("复现命令（契约件 verify_command_drift）：cd core && go test ./cmd/zerg/ -run TestUnresolvedEvidenceAnchorsAreDriftImmune -count=1 -v")
}

// anchorOf —— 取台账里某入口某件的那条锚（负控/正控要按台账现读，不另写第二份字面量）。
func anchorOf(led *contract.UnresolvedLedger, entryID, file string) (string, bool) {
	for _, e := range led.Entries {
		if e.ID != entryID {
			continue
		}
		for _, c := range e.Callers {
			if c.File == file {
				return c.Anchor, true
			}
		}
	}
	return "", false
}

// mustRead —— 读件（读不到直接 Fatal：副本前置动作没生效就别往下判）。
func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读不到 %s：%v", path, err)
	}
	return string(b)
}
