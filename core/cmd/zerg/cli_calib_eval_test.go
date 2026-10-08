// cli_calib_eval_test.go —— `zerg calib` 与 `zerg eval` 两族的**成对负控判据**（T-56 余项 · DEV-0011 · 2026-09-21）。
//
// 八格（前七格全在**合成仓根**里跑，真仓一个字节不动；第八格读真仓盘上现况但只读）：
//
//	① 声明面：`calib ls` 3 件（全 ①）· `eval ls` 22 件，逐件带归属（①18 / ②4 / ④0 —— 现算不手写）；
//	② 台账不腐：两族点名的件在**真仓盘上**逐件存在 —— **已真搬 Go（`GoMain` 非空）的件例外**：旧脚本退役（移出仓）后 `File` 只是人面标名、允许缺席（矩阵 `eval ls` ⟷ `scripts/evals/` 现读两面一致）；
//	③ 未知名 ⇒ 退 2 + 「最像的合法输入」+ 候选（§4.1 K14 第三件）；
//	④ 缺 `--yes` ⇒ 退 2 且**脚本一次都没跑**（fail-closed 那一格 —— 判据是「夹具的哨兵件没出现」）；
//	⑤ `--dry-run` ⇒ 退 0 且脚本一次都没跑（零副作用 · 门⑩ `dryrun.v1`）；
//	⑥ 真跑 ⇒ **退码原样转出**（夹具退 7 / 9，命令面就返 7 / 9 · 不翻译、不映射）；
//	⑦ 归属是闸：② 的件 `run` ⇒ 退 2 并逐字给出归属与理由（且脚本没跑）；④ 本批已空（原 13 件已收编 ①）⇒ 以「④ 已空」断言守住这一档；
//	⑧ 机器面：`--json` 不给字段 ⇒ 退码取自退码表（`usage` · 归一后 = 2）+ stdout 0 字节；点错字段 ⇒ 退 2 + 列合法字段。
//
// ★ **成对负控**（最后一格）：判定口喂**改错的期望值**必须报错 —— 否则本件就是恒绿装置。
package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// synthCalibEvalRepo 建一棵合成仓根：标定/评测各一支夹具脚本（跑过就落哨兵件、退一个**非零**码）。
// 返回 (仓根, calib 哨兵件, eval 哨兵件)。
//
// 为什么哨兵件是「跑没跑」的判据：退码 2 与时序无关，只有盘上真出现的痕迹才证明**脚本被 exec 过**。
func synthCalibEvalRepo(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"), "package version\n")
	mustWrite(t, filepath.Join(root, "scripts", "calib", "calib-x3-model.sh"),
		"#!/bin/bash\n# 合成夹具（判据件用）：证明命令面真调了这支脚本、且退码原样转出。\n"+
			": > \"$(dirname \"$0\")/ran.sentinel\"\necho \"夹具：x"+"3 标定（合成）\"\nexit 7\n")
	// ★ 2026-10-09（family_eval 退役解耦随动 · chat-harness 真搬随动）：评测族的这一格改用**未真搬**的 ① 件
	// `v25_benchmark`（照旧 `python3 <件>` exec）。`compare` / `stability-stats` / `chat-harness` 已真搬 Go ⇒
	// `eval run` 的真跑走 **Go 真身**（✗ 不再 exec 旧脚本）⇒ 它们作「退码原样转出」靶件不再成立。
	// 本格语义（脚本被 exec 过 ＋ 退码原样转出）一字不动。
	mustWrite(t, filepath.Join(root, "scripts", "evals", "_archive", "v25_benchmark.py"),
		"#!/usr/bin/env python3\nimport os\n"+
			"open(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'ran.sentinel'), 'w').write('1')\n"+
			"print('夹具：v25_benchmark（合成）')\nraise SystemExit(9)\n")
	return root, filepath.Join(root, "scripts", "calib", "ran.sentinel"), filepath.Join(root, "scripts", "evals", "_archive", "ran.sentinel")
}

// runZergRepo 进程内跑一条命令，仓根指到**合成仓**（env 只走 `ZERG_REPO` 这一枚）。
func runZergRepo(t *testing.T, repo string, argv ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", repo)
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// runZergRealRepo 进程内跑一条命令，仓根走**现读推导**（不设 `ZERG_REPO` —— 与使用者敲命令时同一份推导）。
func runZergRealRepo(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// envelopeItems 解一份 `--json` 包封的 `items` 数组（键 → 值）。
func envelopeItems(t *testing.T, out string) []map[string]string {
	t.Helper()
	var doc struct {
		Schema string              `json:"schema"`
		Items  []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("`--json` 输出解不动：%v · %q", err, out)
	}
	if doc.Schema != "zerg/v1" {
		t.Errorf("包封第一键 schema = %q（要 `zerg/v1`）", doc.Schema)
	}
	return doc.Items
}

// judgeRun —— 本件「真跑那一格」的**唯一判定口**：退码 + 哨兵件（脚本跑没跑）。
// 参数一律显式传入 —— 为的是让负控能喂**改错的期望值**进来。
func judgeRun(wantRC int, wantRan bool, gotRC int, ran bool) []error {
	var errs []error
	if gotRC != wantRC {
		errs = append(errs, fmt.Errorf("退码 %d ≠ 期望 %d", gotRC, wantRC))
	}
	if ran != wantRan {
		errs = append(errs, fmt.Errorf("脚本「跑没跑」= %v ≠ 期望 %v（哨兵件这一格）", ran, wantRan))
	}
	return errs
}

// ── ① 声明面（两族的 `ls`）────────────────────────────────────────────────────────────

func TestCalibEval_LsRoster(t *testing.T) {
	repo, _, _ := synthCalibEvalRepo(t)

	rc, out, errb := runZergRepo(t, repo, "calib", "ls", "--json", "name,script,state")
	if rc != 0 {
		t.Fatalf("`calib ls --json` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	calib := envelopeItems(t, out)
	if len(calib) != 3 {
		t.Errorf("标定线应 3 件（① 收编 3），得到 %d 件：%v", len(calib), calib)
	}
	for _, it := range calib {
		if !strings.HasPrefix(it["state"], "①") {
			t.Errorf("标定线逐件都应归 ① 收编，%s 的归属是 %q", it["name"], it["state"])
		}
		if !strings.HasPrefix(it["script"], "scripts/calib/") {
			t.Errorf("%s 的件不在 `scripts/calib/` 下：%q", it["name"], it["script"])
		}
	}

	rc, out, errb = runZergRepo(t, repo, "eval", "ls", "--json", "name,script,class")
	if rc != 0 {
		t.Fatalf("`eval ls --json` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	ev := envelopeItems(t, out)
	if len(ev) != 22 {
		t.Errorf("评测线应 22 件，得到 %d 件", len(ev))
	}
	tally := map[string]int{}
	for _, it := range ev {
		tally[it["class"]]++
	}
	// 四类归属表现读：① 收编 18（原 5 + 2026-10-08 同批收编的原 ④「维持待拍」13）· ② 保留内部 4 · ③ 退役 0 · ④ 维持待拍 0。
	want := map[string]int{"①": 18, "②": 4, "④": 0}
	for k, v := range want {
		if tally[k] != v {
			t.Errorf("归属 %s 应是 %d 件，现算 %d 件（合计 %d/%d）", k, v, tally[k], len(ev), 22)
		}
	}
	if tally["③"] != 0 {
		t.Errorf("③ 退役应 0 件（**AI 不许判退役**），现算 %d 件", tally["③"])
	}
}

// ── ② 台账不腐（真仓现读 · 只读）────────────────────────────────────────────────────

// judgeRosterVsDisk —— 「台账不腐」这一格的**唯一判定口**（参数一律显式传入 ⇒ 负控能喂改错的期望值进来）。
//
// 双向：
//
//	① 点名而盘上没这件 ⇒ 幽灵条目 —— **仅当该件已真搬 Go（`goBacked[rel]`）时豁免**：真跑走进程内
//	   Go 真身，旧脚本退役（移出仓）后 `File` 只是人面标名（`family_eval.go:22-23`）；**未真搬的件
//	   缺席照样红** —— 这一支不放宽。
//	② 盘上有而命令面没点名 ⇒ 漏登记 ⇒ 一律红。
func judgeRosterVsDisk(family string, named, disk, goBacked map[string]bool) []error {
	var errs []error
	for rel := range named {
		if disk[rel] || goBacked[rel] {
			continue
		}
		errs = append(errs, fmt.Errorf("%s 族：幽灵条目：%s 在命令面的声明里，盘上却没有这一件（且它未真搬 Go ⇒ 不许缺席）", family, rel))
	}
	for rel := range disk {
		if !named[rel] {
			errs = append(errs, fmt.Errorf("%s 族：漏登记：盘上有 %s，命令面的声明里没有它", family, rel))
		}
	}
	return errs
}

func TestCalibEval_RosterMatchesDisk(t *testing.T) {
	root := repoRootFromCLI(t)
	// 已真搬 Go 的件（`GoMain` 非空）⇒ 旧脚本退役（移出仓）后 `File` 只是人面标名、允许缺席
	// （与 `family_eval.go:22-23` 的退役解耦口径同源 · 从名册真源取，别在测试里另抄一份）。
	goBacked := map[string]bool{}
	for _, rel := range zerg.EvalGoBackedFilesForTest() {
		goBacked[rel] = true
	}
	cases := []struct {
		family, dir string
		want        int
	}{
		{"calib", "scripts/calib", 3},
		{"eval", "scripts/evals", 22},
	}
	for _, c := range cases {
		rc, out, errb := runZergRealRepo(t, c.family, "ls", "--json", "script")
		if rc != 0 {
			t.Fatalf("`%s ls --json` ⇒ 退 0，得到 %d · stderr=%s", c.family, rc, errb)
		}
		named := map[string]bool{}
		for _, it := range envelopeItems(t, out) {
			named[it["script"]] = true
		}
		if len(named) != c.want {
			t.Errorf("%s 族应点名 %d 件，得到 %d 件", c.family, c.want, len(named))
		}
		// 现读盘面：**递归**收 c.dir 下全部件（含归档子目录 —— 2026-10-08 拍板：evals 历史评测载荷
		// 归档进 `scripts/evals/_archive/`，命令面仍点名它们 ⇒ 台账不腐这一格要跟着看进去）。
		disk := map[string]bool{}
		if werr := filepath.WalkDir(filepath.Join(root, c.dir), func(p string, de os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() {
				if strings.HasPrefix(de.Name(), ".") || de.Name() == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(de.Name(), ".") {
				return nil
			}
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				return rerr
			}
			disk[filepath.ToSlash(rel)] = true
			return nil
		}); werr != nil {
			t.Fatalf("读不到 %s：%v", c.dir, werr)
		}
		for _, e := range judgeRosterVsDisk(c.family, named, disk, goBacked) {
			t.Error(e)
		}
	}
}

// TestCalibEval_RosterJudgeHasTeeth —— 成对负控：上面的判定口必须**带牙**（✗ 不是恒绿装置）。
// 双向都要能红：未真搬的件缺席 ⇒ 红；漏登记 ⇒ 红；已真搬 Go 的件缺席 ⇒ 放行。
func TestCalibEval_RosterJudgeHasTeeth(t *testing.T) {
	// ① 未真搬 Go 的件缺席 ⇒ **必须红** —— 这是「不放宽存在性检查」的那颗牙
	if len(judgeRosterVsDisk("eval",
		map[string]bool{"scripts/evals/x.py": true}, nil, nil)) == 0 {
		t.Error("负控①：未真搬的件缺席却没报幽灵条目（存在性检查被放宽了）")
	}
	// ② 已真搬 Go 的件缺席 ⇒ **必须放行**（退役后 `File` 只是人面标名）
	if errs := judgeRosterVsDisk("eval",
		map[string]bool{"scripts/evals/x.py": true}, nil,
		map[string]bool{"scripts/evals/x.py": true}); len(errs) != 0 {
		t.Errorf("负控②：已真搬 Go 的件缺席被误判为幽灵条目：%v", errs)
	}
	// ③ 盘上有而命令面没点名 ⇒ **必须红**（另一方向同样不放宽）
	if len(judgeRosterVsDisk("eval",
		nil, map[string]bool{"scripts/evals/y.py": true}, nil)) == 0 {
		t.Error("负控③：漏登记没报红")
	}
	// ④ 正控：两面一致 ⇒ 不报
	same := map[string]bool{"scripts/evals/z.py": true}
	if errs := judgeRosterVsDisk("eval", same,
		map[string]bool{"scripts/evals/z.py": true}, nil); len(errs) != 0 {
		t.Errorf("正控：两面一致却报红：%v", errs)
	}
}

// ── ③ 未知名 ⇒ 2 + 最像的合法输入 ─────────────────────────────────────────────────────

func TestCalibEval_UnknownNameIsUsageError(t *testing.T) {
	repo, _, _ := synthCalibEvalRepo(t)
	cases := []struct {
		argv []string
		near string
	}{
		{[]string{"calib", "run", "calib-x3"}, "calib-x3-model"},
		{[]string{"calib", "show", "zzz-没有这件-zz"}, ""},
		{[]string{"eval", "run", "stability"}, "stability-stats"},
		{[]string{"eval", "show", "zzz-没有这件-zz"}, ""},
	}
	for _, c := range cases {
		rc, out, errb := runZergRepo(t, repo, c.argv...)
		if rc != 2 {
			t.Errorf("%v ⇒ 退 2（执行前判），得到 %d", c.argv, rc)
		}
		if out != "" {
			t.Errorf("%v ⇒ stdout 必须 0 字节（错误面走 stderr），得到 %q", c.argv, out)
		}
		if !strings.Contains(errb, "候选：") {
			t.Errorf("%v ⇒ 要列候选（K14 第三件），stderr=%q", c.argv, errb)
		}
		if c.near != "" && !strings.Contains(errb, "最像的合法输入: "+c.near) {
			t.Errorf("%v ⇒ 要给出「最像的合法输入: %s」，stderr=%q", c.argv, c.near, errb)
		}
	}
	// 一件都没给 ⇒ 也是 2（不是「跑了个空的」）
	for _, fam := range []string{"calib", "eval"} {
		rc, out, errb := runZergRepo(t, repo, fam, "run")
		if rc != 2 || out != "" || !strings.Contains(errb, "要给一枚件名") {
			t.Errorf("%s run（缺件名）⇒ 退 2 + 0 字节 + 点明缺什么：rc=%d out=%q err=%q", fam, rc, out, errb)
		}
	}
}

// ── ④⑤⑥ 三态：缺 --yes / --dry-run / 真跑（退码原样转出）────────────────────────────

func TestCalibEval_ThreeStatesAndExitForwarding(t *testing.T) {
	repo, calibSentinel, evalSentinel := synthCalibEvalRepo(t)

	// ④ 缺 --yes ⇒ 退 2 且**脚本一次都没跑**（fail-closed）
	rc, out, errb := runZergRepo(t, repo, "calib", "run", "calib-x3-model")
	if errs := judgeRun(2, false, rc, exists(calibSentinel)); len(errs) > 0 {
		t.Errorf("缺 --yes 那一格：%v", errs)
	}
	if out != "" {
		t.Errorf("缺 --yes ⇒ stdout 0 字节（没有结果面），得到 %q", out)
	}
	if !strings.Contains(errb, "缺 `--yes` ⇒ 不执行") {
		t.Errorf("缺 --yes ⇒ 要逐字说明 fail-closed：%q", errb)
	}

	// ⑤ --dry-run ⇒ 退 0 且脚本一次都没跑（零副作用）
	rc, out, errb = runZergRepo(t, repo, "calib", "run", "calib-x3-model", "--dry-run")
	if errs := judgeRun(0, false, rc, exists(calibSentinel)); len(errs) > 0 {
		t.Errorf("--dry-run 那一格：%v · stderr=%s", errs, errb)
	}
	if !strings.Contains(out, "计划件") || !strings.Contains(out, "scripts/calib/calib-x3-model.sh") {
		t.Errorf("--dry-run ⇒ 计划件要打在 stdout 上（含要跑的那一行）：%q", out)
	}

	// ⑥ 真跑 ⇒ 退码**原样转出**（夹具退 7 ⇒ 命令面返 7）且脚本真跑了
	rc, out, errb = runZergRepo(t, repo, "calib", "run", "calib-x3-model", "--yes")
	if errs := judgeRun(7, true, rc, exists(calibSentinel)); len(errs) > 0 {
		t.Errorf("真跑那一格（标定）：%v · stderr=%s", errs, errb)
	}
	if !strings.Contains(out, "夹具：x"+"3 标定（合成）") {
		t.Errorf("真跑 ⇒ 脚本自己的输出原样进 stdout：%q", out)
	}

	// 同一套三态在评测族上再走一遍（夹具退 9 ⇒ 命令面返 9）
	// ★ 靶件 = `v25_benchmark`（未真搬的 ① 件）—— `compare`/`stability-stats`/`chat-harness`
	//   已真搬 Go，其真跑走 Go 真身（✗ 不 exec 旧脚本）⇒ 拆到此格不再成立；本格判据一字不动。
	rc, _, errb = runZergRepo(t, repo, "eval", "run", "v25_benchmark", "--yes")
	if errs := judgeRun(9, true, rc, exists(evalSentinel)); len(errs) > 0 {
		t.Errorf("真跑那一格（评测）：%v · stderr=%s", errs, errb)
	}
}

// exists 哨兵件在不在（「脚本跑没跑」的唯一判据）。
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ── ⑦ 归属是闸：②/④ 的件不开 run 面 ──────────────────────────────────────────────────

func TestEvalRun_OnlyAdoptedIsOpen(t *testing.T) {
	repo, _, evalSentinel := synthCalibEvalRepo(t)
	for _, name := range []string{"make-manifest"} {
		rc, out, errb := runZergRepo(t, repo, "eval", "run", name, "--yes")
		if errs := judgeRun(2, false, rc, exists(evalSentinel)); len(errs) > 0 {
			t.Errorf("② 的件 `%s` 不许开 run 面：%v", name, errs)
		}
		if out != "" {
			t.Errorf("`%s` ⇒ stdout 0 字节，得到 %q", name, out)
		}
		if !strings.Contains(errb, "只对 **① 收编**的件开放") || !strings.Contains(errb, "理由：") {
			t.Errorf("`%s` ⇒ 要逐字给出归属与理由（不是「不支持」）：%q", name, errb)
		}
	}
	// ④ 档**本批已空**（原 13 件「维持待拍」2026-10-08 同批收编入 ①）⇒ 这一格改为断言「④ 已空」：
	//   非 ① 的件仍不许开 run 的语义**不删** —— 由上面 ② 那一格守着（✗ 不放宽、✗ 不删用例）。
	rc, out, errb := runZergRepo(t, repo, "eval", "ls", "--json", "name,class")
	if rc != 0 {
		t.Fatalf("`eval ls --json` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	pending := 0
	for _, it := range envelopeItems(t, out) {
		if strings.HasPrefix(it["class"], "④") {
			pending++
		}
	}
	if pending != 0 {
		t.Errorf("④ 维持待拍应 0 件（原 13 件已同批收编 ①），现算 %d 件", pending)
	}
	// 同一件 `show` 是**开着**的（只投影、不开 run —— 两件事不是一个开关）
	rc, out, errb = runZergRepo(t, repo, "eval", "show", "make-manifest", "--json", "name,class,state")
	if rc != 0 {
		t.Fatalf("`eval show make-manifest` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	items := envelopeItems(t, out)
	if len(items) != 1 || !strings.HasPrefix(items[0]["class"], "②") {
		t.Errorf("`show` 面要看得到 ② 的件与它的归属：%v", items)
	}
}

// ── ⑧ 机器面（K2 / 字段面）──────────────────────────────────────────────────────────

func TestCalibEval_JSONFieldFace(t *testing.T) {
	repo, _, _ := synthCalibEvalRepo(t)
	for _, fam := range []string{"calib", "eval"} {
		// 给了 --json 不给字段 ⇒ 退码取自退码表（`usage` · 归一后 = 2）；★ 本批起发射闸扩到
		//   `inv.err != nil` ⇒ **错误面**是把包封打成机器可读走 stdout（旧行为「stdout 0 字节」已过期，
		//   旧行为下下面这条会红 ⇒ 有牙）；但**结果面**仍必须空（`items` 一条都不出）—— 两个概念别混。
		wantK2 := usageCodeFromTable(t)
		rc, out, errb := runZergRepo(t, repo, fam, "ls", "--json")
		if rc != wantK2 {
			t.Errorf("`%s ls --json` ⇒ 退 %d，得到 rc=%d out=%q", fam, wantK2, rc, out)
		}
		if !strings.Contains(out, `"items":[]`) || !strings.Contains(out, `"error"`) {
			t.Errorf("`%s ls --json`（K2）⇒ stdout 必是 `error` 包封且 `items` 空，得到 %q", fam, out)
		}
		if !strings.Contains(errb, "可选字段") {
			t.Errorf("`%s ls --json` ⇒ 要列可选字段：%q", fam, errb)
		}
		// 点错字段 ⇒ 退 2 且列合法字段
		rc, out, errb = runZergRepo(t, repo, fam, "ls", "--json", "nosuchfield-zz")
		if rc != 2 {
			t.Errorf("`%s ls --json nosuchfield-zz` ⇒ 退 2，得到 rc=%d", fam, rc)
		}
		if !strings.Contains(errb, "合法字段") {
			t.Errorf("`%s ls --json nosuchfield-zz` ⇒ 要列合法字段：%q", fam, errb)
		}
		// ★ 这一格的 stdout **不是**空的：dispatch 给失败面补一份机器可读的 `error` 包封
		//   （§九 M7：AI 自愈只读 kind）—— 那是契约的一部分，不是「人面漏到 stdout」。
		if out != "" && (!strings.Contains(out, `"schema":"zerg/v1"`) || !strings.Contains(out, `"error"`)) {
			t.Errorf("`%s ls --json nosuchfield-zz` ⇒ stdout 要么空、要么是一份 `error` 包封，得到 %q", fam, out)
		}
		// 未知旗标 ⇒ 退 2（执行前判 · 不碰任何目标）
		rc, out, _ = runZergRepo(t, repo, fam, "ls", "--nosuchflag-zz")
		if rc != 2 || out != "" {
			t.Errorf("`%s ls --nosuchflag-zz` ⇒ 退 2 + 0 字节，得到 rc=%d out=%q", fam, rc, out)
		}
	}
}

// ── ★ 成对负控：判定口必须**带牙** ───────────────────────────────────────────────────

func TestCalibEval_JudgeHasTeeth(t *testing.T) {
	repo, calibSentinel, _ := synthCalibEvalRepo(t)

	rc, _, errb := runZergRepo(t, repo, "calib", "run", "calib-x3-model", "--yes")
	if errs := judgeRun(7, true, rc, exists(calibSentinel)); len(errs) > 0 {
		t.Fatalf("负控前的正控就不绿：%v · stderr=%s", errs, errb)
	}
	ran := exists(calibSentinel)
	// ① 把**期望退码**改错 ⇒ 必须报
	if len(judgeRun(rc+1, true, rc, ran)) == 0 {
		t.Error("负控① 失败：期望退码改错后没报错（恒绿装置）")
	}
	// ② 把**「跑没跑」**改错 ⇒ 必须报
	if len(judgeRun(rc, false, rc, ran)) == 0 {
		t.Error("负控② 失败：哨兵件一格改错后没报错")
	}
	// ③ 干跑那一格：把「跑没跑」的期望写成「跑了」而实际没跑 ⇒ 必须报（哨兵件这一格对干跑也带牙）
	rcDry, _, _ := runZergRepo(t, repo, "calib", "run", "calib-x3-model", "--dry-run")
	if len(judgeRun(0, true, rcDry, false)) == 0 {
		t.Error("负控③ 失败：干跑那一格把「跑没跑」写反，判定口没报红")
	}
	//    ★ 本件前面那一格已经用过 `--yes`（哨兵件已在盘上）⇒ ③ 这一格**显式**喂 false，
	//      判的是**判定口本身**带不带牙，不依赖盘上现状。
	// ④ 归属闸的负控：把 ② 的件当成「能跑」⇒ 判定口必须报（这一步证明「归属是闸」这一格**能红**）
	rcNo, _, errbNo := runZergRepo(t, repo, "eval", "run", "make-manifest", "--yes")
	if len(judgeRun(0, true, rcNo, false)) == 0 {
		t.Error("负控④ 失败：把未收编的件当成可跑，判定口没报红")
	}
	if rcNo != 2 || !strings.Contains(errbNo, "保留为内部实现") {
		t.Errorf("负控④：② 的件那一格要给「保留为内部实现」逐字：rc=%d stderr=%q", rcNo, errbNo)
	}
}

// ── ⑨ 干跑文案跟真身：GoMain 非空 ⇒ 打印现役真身（✗ 不再写 `bash|python3 <脚本>`）；为空 ⇒ 维持脚本形 ──

// TestCalibEval_PlanReflectsGoMain —— 计划件的「执行面」一行必须跟**真跑走哪条路**一致：
//
//	· GoMain 非空（真搬 Go）⇒ 真跑走进程内真身 ⇒ 计划件要打印真身形、**不许**再打印 `python3 <脚本>`；
//	· GoMain 为空（旧脚本仍是唯一真源）⇒ 维持 `bash|python3 <脚本>`（✗ 别把未搬的也写成 Go）。
//
// 成对负控：哪一支被哑化（一律写 Go / 一律写脚本）本件都会红。
func TestCalibEval_PlanReflectsGoMain(t *testing.T) {
	repo, _, _ := synthCalibEvalRepo(t)

	// GoMain 非空的件：真跑走 Go 真身 ⇒ 计划件要打印真身形（✗ 不再留 `python3 <脚本>`）
	rc, out, errb := runZergRepo(t, repo, "eval", "run", "compare", "--dry-run")
	if rc != 0 {
		t.Fatalf("`eval run compare --dry-run` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "Go 真身（进程内入口") {
		t.Errorf("GoMain 非空 ⇒ 计划件要打印现役真身形，得到：\n%s", out)
	}
	if !strings.Contains(out, "✗ 不再 exec 旧脚本 scripts/evals/compare.py") {
		t.Errorf("GoMain 非空 ⇒ 计划件要点明不走旧脚本，得到：\n%s", out)
	}
	if strings.Contains(out, "python3 ") {
		t.Errorf("GoMain 非空 ⇒ 计划件**不许**再打印 `python3 <脚本>`（干跑与实跑不一致）：\n%s", out)
	}
	if strings.Contains(out, "脚本本体是唯一真源") {
		t.Errorf("GoMain 非空 ⇒ 计划件**不许**再写「脚本本体是唯一真源」（那是未搬件的形）：\n%s", out)
	}

	// GoMain 为空的件：旧脚本仍是唯一真源 ⇒ 维持脚本形
	rc, out, errb = runZergRepo(t, repo, "eval", "run", "v25_benchmark", "--dry-run")
	if rc != 0 {
		t.Fatalf("`eval run v25_benchmark --dry-run` ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "脚本本体是唯一真源") {
		t.Errorf("GoMain 为空 ⇒ 计划件要维持脚本形（✗ 别写成 Go 真身）：\n%s", out)
	}
	if !strings.Contains(out, "/scripts/evals/_archive/v25_benchmark.py") {
		t.Errorf("GoMain 为空 ⇒ 执行面仍要打印 `python3 <脚本>`：\n%s", out)
	}
	if strings.Contains(out, "Go 真身（进程内入口") {
		t.Errorf("GoMain 为空 ⇒ 计划件**不许**写成 Go 真身（✗ 别把未搬的也写成 Go）：\n%s", out)
	}
}
