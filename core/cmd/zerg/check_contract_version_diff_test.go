package main

// check_contract_version_diff_test.go —— 上件（check_contract_version_diff.go）的成对负控单测件。
//
// 成对口径：正控 = 基准档（同件对同件 rc=0 / 仓外合成 git 仓真跑）；负控 = **必须报红**的改坏档
// （改字段 / 增条 / 删条 / 改 authority / 改 change_flow / 件读不到 / 基线取不到 / 根取不到）。
// 「永远报红」与「永远不报红」一样没用 —— 两侧都有牙，任一侧塌了本件必红。
//
// 夹具纪律：git 仓一律用 **t.TempDir() 里的仓外合成仓**（✗ 不指真仓）· 件读不到的负控只用临时目录 ·
// 测件**不依赖 python** ⇒ GAP-20261001-61 那一格**不适用**（理由：本件不调 python3，无外部解释器面）。
// GAP-20261001-64：原 py 件的自检只写 tempfile 目录、**不真改仓内件再还原** ⇒ 本件照搬「只写临时目录」，
// 不引入任何「改真件再还原」档。

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const cvdTRegistryV1 = `{
 "authority": {"truth": "代码内 schema", "rule": "改真源必须同批改导出物"},
 "entries": [
  {"id": "S-a", "name": "甲条", "truth": "core/a.json", "truth_shape": "x", "version_field": "v",
   "gate": "scripts/gates/a.py", "change_class": "B", "change_note": "n"},
  {"id": "S-b", "name": "乙条", "truth": "core/b.json", "truth_shape": "y", "version_field": "v",
   "gate": "scripts/gates/b.py", "change_class": "B", "change_note": "n"}
 ],
 "change_flow": {"author_not_approver": true, "steps": ["V0", "V1"]},
 "schema": "zerg-contract-registry/1"
}
`

const cvdTRegistryV2 = `{
 "authority": {"truth": "代码内 schema", "rule": "改真源必须同批改导出物（新版）"},
 "entries": [
  {"id": "S-a", "name": "甲条", "truth": "core/a.json", "truth_shape": "x", "version_field": "v",
   "gate": "scripts/gates/a.py", "change_class": "B", "change_note": "n"},
  {"id": "S-b", "name": "乙条", "truth": "core/b.json", "truth_shape": "y", "version_field": "v",
   "gate": "scripts/gates/b.py", "change_class": "B", "change_note": "n"},
  {"id": "S-c", "name": "丙条", "truth": "core/c.json", "truth_shape": "z", "version_field": "v",
   "gate": "scripts/gates/c.py", "change_class": "B", "change_note": "n"}
 ],
 "change_flow": {"author_not_approver": true, "steps": ["V0", "V1", "V2"]},
 "schema": "zerg-contract-registry/1"
}
`

// ── 测件自带小工具（全顶层私有符号一律 cvd 前缀，与主件同包不撞名） ──────────

func cvdTParse(t *testing.T, s string) map[string]any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("夹具 json 解析失败：%v", err)
	}
	return cvdAsMap(v)
}

func cvdTMustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败 %s：%v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写件失败 %s：%v", path, err)
	}
}

func cvdTEqual(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s：行数不符 got=%d want=%d\n got=%q\nwant=%q", name, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s：第 %d 行不符\n got=%q\nwant=%q", name, i, got[i], want[i])
		}
	}
}

func cvdTCapture(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe 建不了：%v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	rc := fn()
	_ = w.Close()
	os.Stdout = old
	return <-done, rc
}

func cvdTRequireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("本机没有 git ⇒ 仓外合成仓夹具建不了（跳过 · 不判绿）")
	}
}

// cvdTRepo 建一个**仓外合成 git 仓**（t.TempDir · ✗ 不指真仓）：
// 两个提交 · registry.json 在第二个提交里加了 S-c 且改了 authority.rule / change_flow.steps。
func cvdTRepo(t *testing.T) string {
	t.Helper()
	cvdTRequireGit(t)
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	rel := filepath.Join(dir, cvdRegistryRel)
	cvdTMustWrite(t, filepath.Join(dir, cvdRootAnchor), "package version\n")
	cvdTMustWrite(t, rel, cvdTRegistryV1)

	run := func(args ...string) {
		t.Helper()
		if rc, out := cvdGit(dir, args); rc != 0 {
			t.Fatalf("git %v rc=%d out=%s", args, rc, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	run("add", "-A")
	run("commit", "-qm", "v1")
	cvdTMustWrite(t, rel, cvdTRegistryV2)
	run("add", "-A")
	run("commit", "-qm", "v2")
	return dir
}

// ── 纯函数面 ────────────────────────────────────────────────────────────────

// GAP-20261001-64 牙①：空差异必须是非 nil 空片（nil 不得兼表「取不到」）。
func TestCVDCompareDocsIdenticalIsEmptyNotNil(t *testing.T) {
	a := cvdTParse(t, cvdTRegistryV1)
	b := cvdTParse(t, cvdTRegistryV1)
	got := cvdCompareDocs(a, b)
	if got == nil {
		t.Fatal("同件对同件必须给**非 nil**空片（nil 会与「取不到」混表 · GAP-64）")
	}
	if len(got) != 0 {
		t.Fatalf("同件对同件必须零差异行，得到 %q", got)
	}
}

// 负控：增/删两条分支都得有话，且**删除先于新增**（py 的遍历序）。
func TestCVDCompareDocsAddDeleteOrder(t *testing.T) {
	old := cvdTParse(t, `{"entries": [{"id": "S-a", "name": "甲"}, {"id": "S-b", "name": "乙"}]}`)
	new := cvdTParse(t, `{"entries": [{"id": "S-b", "name": "乙"}, {"id": "S-c", "name": "丙"}]}`)
	got := cvdCompareDocs(old, new)
	cvdTEqual(t, "增删序", got, []string{
		"S-a 整条 旧→新（**删除**条目：该 id 不在新版里）",
		"S-c 整条 旧→新（**新增**条目：该 id 不在旧版里）",
	})
}

// 负控：条目序按**新版**插入序 · 字段序按 cvdEntryFields 固定序（name < truth < gate ...）。
func TestCVDCompareDocsEntryAndFieldOrder(t *testing.T) {
	old := cvdTParse(t, `{"entries": [{"id": "S-a", "name": "n1", "gate": "g1"}, {"id": "S-b", "truth": "t1"}]}`)
	new := cvdTParse(t, `{"entries": [{"id": "S-a", "name": "n2", "gate": "g2"}, {"id": "S-b", "truth": "t2"}]}`)
	got := cvdCompareDocs(old, new)
	cvdTEqual(t, "条目/字段序", got, []string{
		"S-a name 旧→新 ｜ 旧=n1 新=n2",
		"S-a gate 旧→新 ｜ 旧=g1 新=g2",
		"S-b truth 旧→新 ｜ 旧=t1 新=t2",
	})
}

// 负控：顶层块按 authority → change_flow 固定序 · 块内路径按字典序 · 布尔按 py 的 true/false。
func TestCVDCompareDocsTopKeysAndSortedPaths(t *testing.T) {
	old := cvdTParse(t, `{"authority": {"b": "1", "a": "1"}, "change_flow": {"author_not_approver": true}}`)
	new := cvdTParse(t, `{"authority": {"b": "2", "a": "2"}, "change_flow": {"author_not_approver": false}}`)
	got := cvdCompareDocs(old, new)
	cvdTEqual(t, "顶层块", got, []string{
		"顶层 authority.a 旧→新 ｜ 旧=1 新=2",
		"顶层 authority.b 旧→新 ｜ 旧=1 新=2",
		"顶层 change_flow.author_not_approver 旧→新 ｜ 旧=true 新=false",
	})
}

func TestCVDFlattenPaths(t *testing.T) {
	out := map[string]any{}
	cvdFlatten(cvdTParse(t, `{"authority": {"a": {"b": 1}, "c": [7, 8]}}`), "", out)
	for k, want := range map[string]string{
		"authority.a.b": "1", "authority.c[0]": "7", "authority.c[1]": "8",
	} {
		v, ok := out[k]
		if !ok {
			t.Fatalf("路径 %s 缺（得到 %v）", k, out)
		}
		if got := cvdPyDump(v, "", 0); got != want {
			t.Fatalf("路径 %s 值 = %s，want %s", k, got, want)
		}
	}
}

func TestCVDShowTruncation(t *testing.T) {
	same := strings.Repeat("甲", 120)
	if got := cvdShow(same); got != same {
		t.Fatalf("120 字必须原样（得到 %d 字）", len([]rune(got)))
	}
	long := strings.Repeat("甲", 121)
	got := cvdShow(long)
	want := strings.Repeat("甲", 117) + "…"
	if got != want {
		t.Fatalf("121 字必须截成 117 + …，得到 %q", got)
	}
	if len([]rune(got)) != 118 {
		t.Fatalf("截断后应 118 字，得到 %d", len([]rune(got)))
	}
}

// GAP-20261001-64 牙②：缺 entries 必须给**非 nil**空片（不得用 nil 兼表「取不到」）。
func TestCVDEntriesMissingIsEmptyNotNil(t *testing.T) {
	got := cvdEntries(map[string]any{})
	if got == nil {
		t.Fatal("缺 entries 必须给非 nil 空片（GAP-64）")
	}
	if len(got) != 0 {
		t.Fatalf("缺 entries 应为空，得到 %d", len(got))
	}
	order, m := cvdIdMap(nil)
	if order == nil || m == nil {
		t.Fatal("空入参必须给非 nil 空片/空表（GAP-64）")
	}
}

// ── 机器面 / 人面的 python 形 ───────────────────────────────────────────────

func TestCVDPyJSONFlatSortedAndSpaced(t *testing.T) {
	got := cvdPyDump(map[string]any{"b": 2, "a": "x", "c": false}, "", 0)
	want := `{"a": "x", "b": 2, "c": false}`
	if got != want {
		t.Fatalf("--json 形不符（py sort_keys + 默认分隔符）\n got=%s\nwant=%s", got, want)
	}
}

func TestCVDPyDumpIndentShape(t *testing.T) {
	got := cvdPyDump(cvdObj{{K: "a", V: 1}, {K: "b", V: []string{"x", "y"}}}, " ", 0)
	want := "{\n \"a\": 1,\n \"b\": [\n  \"x\",\n  \"y\"\n ]\n}"
	if got != want {
		t.Fatalf("--out 形不符（py indent=1）\n got=%q\nwant=%q", got, want)
	}
}

// ── 夹具档 / 阻断档 ────────────────────────────────────────────────────────

func TestCVDCheckFixturePair(t *testing.T) {
	dir := t.TempDir()
	same := filepath.Join(dir, "same.json")
	cvdTMustWrite(t, same, cvdTRegistryV1)
	other := filepath.Join(dir, "other.json")
	cvdTMustWrite(t, other, cvdTRegistryV2)

	rc, lines, info := cvdCheck(cvdArgs{Registry: same, OldFile: same, NewFile: same})
	if rc != 0 || info.Verdict != "unchanged" || info.DiffLines != 0 {
		t.Fatalf("正控：同件对同件应 rc=0/unchanged/0 行，得到 rc=%d verdict=%s diff=%d", rc, info.Verdict, info.DiffLines)
	}
	if !cvdAnyContains(lines, "无变更") {
		t.Fatalf("正控必须逐字印「无变更」：%q", lines)
	}
	if info.OldEntries != 2 || info.NewEntries != 2 {
		t.Fatalf("条数读数：旧 %d / 新 %d（want 2/2）", info.OldEntries, info.NewEntries)
	}

	rc, lines, info = cvdCheck(cvdArgs{Registry: same, OldFile: same, NewFile: other})
	if rc != 1 || info.Verdict != "changed" || info.DiffLines != 3 {
		t.Fatalf("负控：改坏档应 rc=1/changed/3 行，得到 rc=%d verdict=%s diff=%d", rc, info.Verdict, info.DiffLines)
	}
	if info.NewEntries != 3 {
		t.Fatalf("新版条数应 3，得到 %d", info.NewEntries)
	}
	if !cvdAnyContains(lines, "**新增**条目") || !cvdAnyContains(lines, "顶层 change_flow.steps[2]") {
		t.Fatalf("负控判词缺分支：%q", lines)
	}
}

// 负控：夹具件读不到 ⇒ rc=2 不给结论（且 lines 非 nil · 错误文案按 py 形）。
func TestCVDCheckFixtureMissingBlocks(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.json")
	cvdTMustWrite(t, ok, cvdTRegistryV1)
	missing := filepath.Join(dir, "根本没这份旧件.json")

	rc, lines, info := cvdCheck(cvdArgs{Registry: ok, OldFile: missing, NewFile: ok})
	if rc != 2 || info.Verdict != "blocked" {
		t.Fatalf("件读不到必须 rc=2/blocked，得到 rc=%d verdict=%s", rc, info.Verdict)
	}
	if lines == nil || len(lines) == 0 {
		t.Fatal("阻断档必须给非 nil 且有话的 lines（GAP-64）")
	}
	if !cvdAnyContains(lines, "不给结论") {
		t.Fatalf("阻断档判词缺「不给结论」：%q", lines)
	}
	want := "No such file or directory: '" + missing + "'"
	if !cvdAnyContains(lines, want) {
		t.Fatalf("错误文案未按 py 形给（缺 %q）：%q", want, lines)
	}
}

// 负控：仓根解析不到 ⇒ rc=2 不给结论（禁当绿）。
func TestCVDCheckRootMissingBlocks(t *testing.T) {
	rc, lines, info := cvdCheck(cvdArgs{RootOK: false, Registry: cvdRegistryRel})
	if rc != 2 || info.Verdict != "blocked" {
		t.Fatalf("根取不到必须 rc=2/blocked，得到 rc=%d verdict=%s", rc, info.Verdict)
	}
	if !cvdAnyContains(lines, "解析不到仓根") {
		t.Fatalf("判词缺「解析不到仓根」：%q", lines)
	}
}

// ── 仓外合成 git 仓：真跑档 + 基线负控 + 成对自检 ───────────────────────────

// 负控牙（最硬的一条）：合成仓真跑必须 rc=1，且差异行**恰好**是那三行 ——
// 任一条分支塌了（漏「新增」/ 漏顶层块 / 路径序反了 / 缺条目计数）本件必红。
func TestCVDCheckRealRunOnSynthRepo(t *testing.T) {
	dir := cvdTRepo(t)
	rc, lines, info := cvdCheck(cvdArgs{Root: dir, RootOK: true, Registry: filepath.Join(dir, cvdRegistryRel)})
	if rc != 1 || info.Verdict != "changed" {
		t.Fatalf("合成仓真跑应 rc=1/changed，得到 rc=%d verdict=%s lines=%q", rc, info.Verdict, lines)
	}
	if info.HeadSHA == "" || info.BaseRev == "" {
		t.Fatalf("两枚指纹不能缺：head=%q base=%q", info.HeadSHA, info.BaseRev)
	}
	if info.WorktreeDirty {
		t.Fatal("合成仓工作树是干净的 ⇒ worktree_dirty 必须是 false")
	}
	if info.OldEntries != 2 || info.NewEntries != 3 || info.DiffLines != 3 {
		t.Fatalf("读数：旧 %d / 新 %d / 差异行 %d（want 2/3/3）", info.OldEntries, info.NewEntries, info.DiffLines)
	}
	cvdTEqual(t, "真跑差异行（尾三行）", lines[len(lines)-3:], []string{
		"S-c 整条 旧→新（**新增**条目：该 id 不在旧版里）",
		"顶层 authority.rule 旧→新 ｜ 旧=改真源必须同批改导出物 新=改真源必须同批改导出物（新版）",
		"顶层 change_flow.steps[2] 旧→新 ｜ 旧=<无> 新=V2",
	})
	if !cvdAnyContains(lines[:2], "旧版 = ") {
		t.Fatalf("首行必须是「旧版 = <父提交前 12 位>:<相对路径> ｜ 新版 = <件>」：%q", lines[:2])
	}
}

// 负控：基线 rev 取不到父提交 ⇒ rc=2 且逐字印「无上一版可比」（不许拿工作树糊过去）。
func TestCVDCheckBaseRevDeadBlocks(t *testing.T) {
	dir := cvdTRepo(t)
	rc, lines, info := cvdCheck(cvdArgs{Root: dir, RootOK: true, Registry: filepath.Join(dir, cvdRegistryRel),
		BaseRev: strings.Repeat("0", 40), HaveBaseRev: true})
	if rc != 2 || info.Verdict != "blocked" {
		t.Fatalf("基线取不到必须 rc=2/blocked，得到 rc=%d verdict=%s", rc, info.Verdict)
	}
	if !cvdAnyContains(lines, "无上一版可比") {
		t.Fatalf("判词缺「无上一版可比」：%q", lines)
	}
}

// 成对自检（正控 2 + 负控 11）在**仓外合成仓**上必须全过：13 组一组不许塌。
func TestCVDSelfTestOnSynthRepo(t *testing.T) {
	dir := cvdTRepo(t)
	out, rc := cvdTCapture(t, func() int { return cvdSelfTest(os.Stdout, dir, true) })
	if rc != 0 {
		t.Fatalf("自检应返回 0，得到 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "✓ 自检全过（13 组：正控 2 + 负控 11 · 成对）") {
		t.Fatalf("自检判词不符：%s", out)
	}
	if strings.Contains(out, "✗") {
		t.Fatalf("自检不该有失败行：%s", out)
	}
}
