// cli_calib_eval_test.go —— `zerg calib` 与 `zerg eval` 两族的**成对负控判据**（T-56 余项 · DEV-0011 · 2026-09-21）。
//
// 八格（前七格全在**合成仓根**里跑，真仓一个字节不动；第八格读真仓盘上现况但只读）：
//
//	① 声明面：`calib ls` 3 件（全 ①）· `eval ls` 22 件，逐件带归属（①5 / ②4 / ④13 —— 现算不手写）；
//	② 台账不腐：两族点名的件在**真仓盘上**逐件存在（矩阵 `eval ls` ⟷ `scripts/evals/` 现读两面一致）；
//	③ 未知名 ⇒ 退 2 + 「最像的合法输入」+ 候选（§4.1 K14 第三件）；
//	④ 缺 `--yes` ⇒ 退 2 且**脚本一次都没跑**（fail-closed 那一格 —— 判据是「夹具的哨兵件没出现」）；
//	⑤ `--dry-run` ⇒ 退 0 且脚本一次都没跑（零副作用 · 门⑩ `dryrun.v1`）；
//	⑥ 真跑 ⇒ **退码原样转出**（夹具退 7 / 9，命令面就返 7 / 9 · 不翻译、不映射）；
//	⑦ 归属是闸：②/④ 的件 `run` ⇒ 退 2 并逐字给出归属与理由（且脚本没跑）；
//	⑧ 机器面：`--json` 不给字段 ⇒ 退 1 + stdout 0 字节；点错字段 ⇒ 退 2 + 列合法字段。
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
			": > \"$(dirname \"$0\")/ran.sentinel\"\necho \"夹具：x3 标定（合成）\"\nexit 7\n")
	mustWrite(t, filepath.Join(root, "scripts", "evals", "compare.py"),
		"#!/usr/bin/env python3\nimport os\n"+
			"open(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'ran.sentinel'), 'w').write('1')\n"+
			"print('夹具：compare（合成）')\nraise SystemExit(9)\n")
	return root, filepath.Join(root, "scripts", "calib", "ran.sentinel"), filepath.Join(root, "scripts", "evals", "ran.sentinel")
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
	// 上一枚的四类归属表：① 收编 5 · ② 保留内部 4 · ③ 退役 0 · ④ 维持待拍 13。
	want := map[string]int{"①": 5, "②": 4, "④": 13}
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

func TestCalibEval_RosterMatchesDisk(t *testing.T) {
	root := repoRootFromCLI(t)
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
		des, err := os.ReadDir(filepath.Join(root, c.dir))
		if err != nil {
			t.Fatalf("读不到 %s：%v", c.dir, err)
		}
		disk := map[string]bool{}
		for _, de := range des {
			if de.IsDir() || strings.HasPrefix(de.Name(), ".") {
				continue
			}
			disk[c.dir+"/"+de.Name()] = true
		}
		for rel := range named {
			if !disk[rel] {
				t.Errorf("幽灵条目：%s 在命令面的声明里，盘上却没有这一件", rel)
			}
		}
		for rel := range disk {
			if !named[rel] {
				t.Errorf("漏登记：盘上有 %s，命令面的声明里没有它（台账不腐这一格）", rel)
			}
		}
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
	if !strings.Contains(out, "夹具：x3 标定（合成）") {
		t.Errorf("真跑 ⇒ 脚本自己的输出原样进 stdout：%q", out)
	}

	// 同一套三态在评测族上再走一遍（夹具退 9 ⇒ 命令面返 9）
	rc, _, errb = runZergRepo(t, repo, "eval", "run", "compare", "--yes")
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
	for _, name := range []string{"make-manifest", "v25_real10"} {
		rc, out, errb := runZergRepo(t, repo, "eval", "run", name, "--yes")
		if errs := judgeRun(2, false, rc, exists(evalSentinel)); len(errs) > 0 {
			t.Errorf("②/④ 的件 `%s` 不许开 run 面：%v", name, errs)
		}
		if out != "" {
			t.Errorf("`%s` ⇒ stdout 0 字节，得到 %q", name, out)
		}
		if !strings.Contains(errb, "只对 **① 收编**的件开放") || !strings.Contains(errb, "理由：") {
			t.Errorf("`%s` ⇒ 要逐字给出归属与理由（不是「不支持」）：%q", name, errb)
		}
	}
	// 同一件 `show` 是**开着**的（只投影、不开 run —— 两件事不是一个开关）
	rc, out, errb := runZergRepo(t, repo, "eval", "show", "make-manifest", "--json", "name,class,state")
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
		// 给了 --json 不给字段 ⇒ 退 1 且 stdout 0 字节（§4.1 K2）
		rc, out, errb := runZergRepo(t, repo, fam, "ls", "--json")
		if rc != 1 || out != "" {
			t.Errorf("`%s ls --json` ⇒ 退 1 + 0 字节，得到 rc=%d out=%q", fam, rc, out)
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
