// dev_edit_returnpoint_manifest_test.go —— 退点档读取面吃清单（缺口 `GAP-20261005-70` · 本单收尾半）。
//
// 病灶（缺口 remainder 逐字）：`dev edit` 的退点档 `devEditReturnPointOf` 只按**单值路径**读 `rollback_ref`
// ⇒ 多件提案的**提案级退点清单**（`{件: 前态件}`）被当「路径形」判死（`os.ReadFile` 读一枚 JSON 串）
// ⇒ 多件落件**每件退不了**。本单让退点档**清单在场时按件（`fileRel`）挑该件的退点**、再照单值路核
// `before_sha256`；清单不在场 ⇒ 逐字走原单值路（单值形态语义一字不改）。
//
// 判据（四条 · 各有相反的那一格）：
//
//	① 清单盖两件 ⇒ **每件各自可退**（tier=1 · 各自的 `cp -p <前态件> <本件>`）。
//	② 清单里**没有本件** ⇒ 不可退（tier=4 · ✗ 不拿别的件的退点冒充）。
//	③ 清单里本件的前态件 **sha 对不上** ⇒ 不可退（tier=2 · `before_sha256` 对拍照旧有牙）。
//	④ 负控（外部 · go -overlay）：哑化「按件挑项」⇒ ① 必红（两件都退不了）。
package main_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// gap70devItem —— 从 `dev edit --json` 的 stdout 取那唯一一条 items[0]。
func gap70devItem(t *testing.T, out string) map[string]string {
	t.Helper()
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "{") {
			continue
		}
		var env struct {
			Items []map[string]string `json:"items"`
		}
		if err := json.Unmarshal([]byte(ln), &env); err != nil {
			continue
		}
		if len(env.Items) >= 1 {
			out := env.Items[0]
			return out
		}
	}
	t.Fatalf("dev edit --json 没给出 items[0]：stdout=%q", out)
	return nil
}

// gap70writeProposal —— 直写一件提案 JSON（声明 files、退点 = rollback）——绕开 `new` 的入参校验面。
func gap70writeProposal(t *testing.T, dir, id, rollback string, files []string) {
	t.Helper()
	slash := make([]string, len(files))
	for i, f := range files {
		slash[i] = filepath.ToSlash(f)
	}
	rec := map[string]any{
		"id": id, "title": "清单件", "target": "D3b", "goal": "g",
		"evidence": []string{"GAP-20261005-70"}, "rollback_ref": rollback,
		"criterion": "zerg gate run --fast", "files": slash,
		"by": "夹具", "subject": "", "subject_kind": "human", "state": "未决",
		"created_at": "2026-10-10T00:00:00+08:00",
	}
	b, _ := json.MarshalIndent(rec, "", " ")
	mustWrite(t, filepath.Join(dir, id+".json"), string(b)+"\n")
}

func TestDevEditReturnPointPerFileManifest(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "core", "internal", "version", "version.go"), "package version\n")
	f1 := filepath.Join("core", "cmd", "zerg", "f1.go")
	f2 := filepath.Join("core", "cmd", "zerg", "f2.go")
	mustWrite(t, filepath.Join(repo, f1), "package main\n\n// f1 v1\n")
	mustWrite(t, filepath.Join(repo, f2), "package main\n\n// f2 v1\n")
	// 两件各自的前态件（内容 == 现盘 ⇒ sha 对得上）。
	pre1 := filepath.Join(t.TempDir(), "before1.go")
	pre2 := filepath.Join(t.TempDir(), "before2.go")
	mustWrite(t, pre1, "package main\n\n// f1 v1\n")
	mustWrite(t, pre2, "package main\n\n// f2 v1\n")
	from1 := filepath.Join(t.TempDir(), "n1.go")
	from2 := filepath.Join(t.TempDir(), "n2.go")
	mustWrite(t, from1, "package main\n\n// f1 v2\n")
	mustWrite(t, from2, "package main\n\n// f2 v2\n")

	prop, state := t.TempDir(), t.TempDir()

	// ① 清单盖两件 ⇒ **每件各自可退**（tier=1 · 各自的 `cp -p <本件前态件> <本件>`）。
	man, _ := json.Marshal(map[string]string{filepath.ToSlash(f1): pre1, filepath.ToSlash(f2): pre2})
	gap70writeProposal(t, prop, "DEV-7101", string(man), []string{f1, f2})
	cases := []struct{ file, from, wantPre string }{{f1, from1, pre1}, {f2, from2, pre2}}
	for _, c := range cases {
		rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-7101", "--file", filepath.ToSlash(c.file),
			"--from", c.from, "--dry-run", "--json", "rollback_verdict,rollback_tier,rollback_cmd")
		if rc != 0 {
			t.Fatalf("件 %s 干跑 ⇒ rc=0：rc=%d stderr=%s", c.file, rc, errb)
		}
		it := gap70devItem(t, out)
		if it["rollback_verdict"] != "可退" || it["rollback_tier"] != "1" {
			t.Errorf("清单在场 ⇒ 件 %s **每件可退**（要 tier=1「可退」）：verdict=%q tier=%q",
				c.file, it["rollback_verdict"], it["rollback_tier"])
		}
		if !strings.Contains(it["rollback_cmd"], c.wantPre) || !strings.Contains(it["rollback_cmd"], filepath.ToSlash(c.file)) {
			t.Errorf("件 %s 的退法要锚**本件自己的**前态件 %s 与本件：cmd=%q", c.file, c.wantPre, it["rollback_cmd"])
		}
	}

	// ② 清单里**没有本件** ⇒ 不可退（tier=4 · ✗ 不拿别的件的退点冒充）。
	man2, _ := json.Marshal(map[string]string{filepath.ToSlash(f1): pre1})
	gap70writeProposal(t, prop, "DEV-7102", string(man2), []string{f1, f2})
	rc, out, _ := runDevEdit(t, repo, prop, state, "--proposal", "DEV-7102", "--file", filepath.ToSlash(f2),
		"--from", from2, "--dry-run", "--json", "rollback_verdict,rollback_tier")
	if rc != 0 {
		t.Fatalf("清单缺本件 ⇒ 干跑仍 rc=0：rc=%d", rc)
	}
	if it := gap70devItem(t, out); it["rollback_verdict"] != "不可退" || it["rollback_tier"] != "4" {
		t.Errorf("清单里没有本件 ⇒ 不可退（tier=4）：verdict=%q tier=%q", it["rollback_verdict"], it["rollback_tier"])
	}

	// ③ 清单里本件的前态件 **sha 对不上** ⇒ 不可退（tier=2 · `before_sha256` 对拍照旧有牙）。
	other := filepath.Join(t.TempDir(), "other.go")
	mustWrite(t, other, "package main\n\n// f2 是另一版\n")
	man3, _ := json.Marshal(map[string]string{filepath.ToSlash(f1): pre1, filepath.ToSlash(f2): other})
	gap70writeProposal(t, prop, "DEV-7103", string(man3), []string{f1, f2})
	rc, out, _ = runDevEdit(t, repo, prop, state, "--proposal", "DEV-7103", "--file", filepath.ToSlash(f2),
		"--from", from2, "--dry-run", "--json", "rollback_verdict,rollback_tier")
	if rc != 0 {
		t.Fatalf("清单本件前态件 sha 不符 ⇒ 干跑仍 rc=0：rc=%d", rc)
	}
	if it := gap70devItem(t, out); it["rollback_verdict"] != "不可退" || it["rollback_tier"] != "2" {
		t.Errorf("清单里本件前态件 sha 对不上 ⇒ 不可退（tier=2）：verdict=%q tier=%q", it["rollback_verdict"], it["rollback_tier"])
	}
}
