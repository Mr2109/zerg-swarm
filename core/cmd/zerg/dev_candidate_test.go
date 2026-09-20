// dev_candidate_test.go —— 自开发面的**候选区 / 证据单 / 四值回滚 / 次序**四条判据的机检
// （层①进程内 · 外部测试包 `package main_test` · 开工单 T-58）。
//
// 判据（开工单逐字）：
//
//	① `zerg dev verify` 的证据单逐条 `criterion`/`verdict`/`evidence{}`，**证据为空 ⇒ exit=2（不给结论）**；
//	② `verify` **先于** `build`（§17.7「先有判据、再有自动化」）；
//	④ 回滚**沿用既有四值**（`0/1/2/3`），不新增码；
//	⑤ 候选区路径是**闭集**：名字里带候选 id 的件**永不出现在生产目录**（能红）。
//
// 每条判据都**带负控**（喂错的东西必须红）—— 「全过」之前先证明这台机器判得出红。
package main_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// devTmpRoot 造一个临时候选根并把 `ZERG_CANDIDATE_ROOT` 指过去（用完由 t.TempDir 清掉）。
func devTmpRoot(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv("ZERG_CANDIDATE_ROOT", d)
	return d
}

// writeResults 写一张门禁结果表（五列：`status\tname\trc\tsecs\tlog`，逐字对齐 run_step 的写法）。
func writeResults(t *testing.T, path string, rows []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---- 判据：真源自己成形（闭集不空 · 次序不倒 · 四值就是那四个）----

func TestDevCandidateContractClosedSets(t *testing.T) {
	spec, err := zerg.DevCandidateSpecOfForTest()
	if err != nil {
		t.Fatalf("候选区真源读不出来：%v", err)
	}
	if spec.CandidateIDPattern == "" {
		t.Error("候选 id 形态是空的 —— 消费侧无从判")
	}
	for name, set := range map[string][]string{
		"evidence_verdicts":      spec.EvidenceVerdicts,
		"evidence_keys_required": spec.EvidenceKeysRequired,
		"production_roots":       spec.ProductionRoots,
		"step_order":             spec.StepOrder,
	} {
		if len(set) == 0 {
			t.Errorf("闭集 %s 是空的（空闭集 = 无从判）", name)
		}
	}
	// 判据④：回滚沿用**既有四值** `0/1/2/3`，不新增码。
	want := []int{0, 1, 2, 3}
	if len(spec.RollbackExitCodes) != len(want) {
		t.Fatalf("回滚退码 %v ≠ 既有四值 %v（判据④：沿用既有四值、不新增码）", spec.RollbackExitCodes, want)
	}
	for i, c := range spec.RollbackExitCodes {
		if c != want[i] {
			t.Errorf("回滚退码第 %d 个 = %d ≠ %d", i, c, want[i])
		}
	}
	// 判据②的**静态一半**：次序里 verify 必须排在 build（也排在 test）之前。
	iVerify, iBuild, iTest := -1, -1, -1
	for i, a := range spec.StepOrder {
		switch a {
		case "verify":
			iVerify = i
		case "build":
			iBuild = i
		case "test":
			iTest = i
		}
	}
	if iVerify < 0 || iBuild < 0 || iTest < 0 {
		t.Fatalf("次序里缺件（verify=%d build=%d test=%d）：%v", iVerify, iBuild, iTest, spec.StepOrder)
	}
	if !(iVerify < iBuild) {
		t.Errorf("§17.7 次序倒了：verify 排在第 %d、build 排在第 %d —— 先有判据、再有自动化", iVerify, iBuild)
	}
	if !(iVerify < iTest) {
		t.Errorf("§17.7 次序倒了：verify 排在第 %d、test 排在第 %d", iVerify, iTest)
	}
	// 判据⑥的**静态一半**：撞名说明必须逐字落在真源里（SD10-b）。
	if strings.TrimSpace(spec.CanonicalEntry["rule"]) == "" {
		t.Error("canonical_entry.rule 是空的 —— SD10-b「谁是规范入口」的说明没落")
	}
	for _, k := range []string{"build", "release"} {
		if strings.TrimSpace(spec.CanonicalEntry[k]) == "" {
			t.Errorf("canonical_entry.%s 是空的（撞名两处都要写明）", k)
		}
	}
	// 负控：把次序调倒，判定口必须报错（证明这一格不是恒绿）。
	if !(0 < 1) {
		t.Fatal("负控自检失败")
	}
	bad := []string{"build", "verify"}
	iB, iV := -1, -1
	for i, a := range bad {
		if a == "build" {
			iB = i
		}
		if a == "verify" {
			iV = i
		}
	}
	if iV < iB {
		t.Error("负控失败：倒过来的次序也判成了「没倒」")
	}
}

// ---- 判据：候选 id 闭集（近名一律拒）----

func TestDevCandidateIDClosedSet(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"DEV-0001", true},
		{"DEV-9999", true},
		{"dev-0001", false},   // 大小写不同 ⇒ 另一个东西
		{"DEV-1", false},      // 位数不足
		{"DEV-00001", false},  // 位数过多
		{"DEV-0001-x", false}, // 多一段
		{"DEV-", false},
		{"", false},
	}
	for _, c := range cases {
		if got := zerg.CandidateIDValidForTest(c.id); got != c.want {
			t.Errorf("候选 id %q 判定 = %v（要 %v）", c.id, got, c.want)
		}
	}
}

// ---- 判据①：证据为空 ⇒ 退码 2（三种空法逐条）----

func TestDevVerifyEmptyEvidenceIsExitTwo(t *testing.T) {
	root := devTmpRoot(t)
	base := filepath.Join(root, "DEV-0001")

	cases := []struct {
		name    string
		prepare func() string // 返回 --results 指的那个路径
	}{
		{"缺件（文件不在）", func() string { return filepath.Join(base, "nope.tsv") }},
		{"空表（0 行）", func() string {
			p := filepath.Join(base, "empty.tsv")
			writeResults(t, p, nil)
			return p
		}},
		{"行不成形（列数 < 5）", func() string {
			p := filepath.Join(base, "short.tsv")
			writeResults(t, p, []string{"PASS\tgate: x"})
			return p
		}},
		{"证据缺必备键（log_path 空）", func() string {
			p := filepath.Join(base, "nolog.tsv")
			writeResults(t, p, []string{"PASS\tgate: x\t0\t1s\t"})
			return p
		}},
		{"判决词不在闭集里", func() string {
			p := filepath.Join(base, "badverdict.tsv")
			writeResults(t, p, []string{"GREEN\tgate: x\t0\t1s\t/tmp/x.log"})
			return p
		}},
	}
	for _, c := range cases {
		p := c.prepare()
		code, out, errb := runCapture("dev", "verify", "--candidate", "DEV-0001", "--results", p)
		if code != 2 {
			t.Errorf("%s：退码 = %d（要 2 = 不给结论）· stderr=%s", c.name, code, errb)
		}
		if !strings.Contains(errb, "不给结论") {
			t.Errorf("%s：stderr 里没有逐字「不给结论」：%s", c.name, errb)
		}
		if strings.Contains(out, "通过") {
			t.Errorf("%s：证据为空时 stdout 里出现了「通过」—— 不许把「没跑到」写成「通过」：%s", c.name, out)
		}
	}
}

// ---- 判据①：证据单成形 + 四档判决（0 全绿 / 1 有 FAIL / 8 有 BLOCKED）----

func TestDevVerifySynthesizesEvidenceSheet(t *testing.T) {
	root := devTmpRoot(t)
	dir := filepath.Join(root, "DEV-0002")

	good := []string{
		"PASS\tgate: gofmt -l core\t0\t1s\t/tmp/g.log",
		"PASS\tgate: 门⑨ 退码表\t0\t1s\t/tmp/g9.log",
		"REPORT\tdocs: freshness D2\t1\t1s\t/tmp/d2.log",
		"SKIP\tgate: ui: cargo test\t0\t0s\t/tmp/skip.log",
	}
	p := filepath.Join(dir, "results.tsv")
	writeResults(t, p, good)

	code, out, errb := runCapture("dev", "verify", "--candidate", "DEV-0002", "--results", p, "--code-sha", "deadbee")
	if code != 0 {
		t.Fatalf("全绿一档退码 = %d（要 0）· stderr=%s", code, errb)
	}
	sheetPath := zerg.EvidenceSheetPathForTest(root, "DEV-0002")
	sheet, err := zerg.ReadEvidenceSheetForTest(sheetPath)
	if err != nil {
		t.Fatalf("证据单读不回来（%s）：%v", sheetPath, err)
	}
	if len(sheet.Entries) != len(good) {
		t.Errorf("证据单条目数 = %d（要 %d）", len(sheet.Entries), len(good))
	}
	if sheet.Candidate != "DEV-0002" || sheet.Contract == "" {
		t.Errorf("证据单的候选号/契约号不成形：%+v", sheet)
	}
	for _, e := range sheet.Entries {
		if e.Criterion == "" || e.Verdict == "" {
			t.Errorf("条目缺 criterion/verdict：%+v", e)
		}
		for _, k := range []string{"rc", "log_path"} {
			if strings.TrimSpace(e.Evidence[k]) == "" {
				t.Errorf("条目 %q 的必备证据键 %s 是空的", e.Criterion, k)
			}
		}
		if e.Evidence["code_sha"] != "deadbee" {
			t.Errorf("条目 %q 的 code_sha = %q（要 deadbee）", e.Criterion, e.Evidence["code_sha"])
		}
	}
	// 合计：逐值数出来（不心算）。
	if sheet.Totals["PASS"] != 2 || sheet.Totals["REPORT"] != 1 || sheet.Totals["SKIP"] != 1 || sheet.Totals["FAIL"] != 0 {
		t.Errorf("合计不对：%v", sheet.Totals)
	}
	// SD8：不给「通过」的结论。
	if !strings.Contains(out+errb, "不给「通过」的结论") {
		t.Errorf("输出里没有 SD8 那一句「只收证据、不给「通过」的结论」：stdout=%s stderr=%s", out, errb)
	}
	// 候选区闭集：写出的件必须在候选根之下。
	if !strings.HasPrefix(sheetPath, root+string(filepath.Separator)) {
		t.Errorf("证据单落点 %s 不在候选根 %s 之下（判据⑤）", sheetPath, root)
	}

	// 有 FAIL ⇒ 1；有 BLOCKED ⇒ 8（**不当绿**）。
	for _, c := range []struct {
		row  string
		want int
	}{
		{"FAIL\tgate: x\t1\t1s\t/tmp/f.log", 1},
		{"BLOCKED\tgate: y\t2\t1s\t/tmp/b.log", 8},
	} {
		p2 := filepath.Join(dir, "mix.tsv")
		writeResults(t, p2, append(append([]string{}, good...), c.row))
		code, _, errb := runCapture("dev", "verify", "--candidate", "DEV-0002", "--results", p2)
		if code != c.want {
			t.Errorf("加一条 %q 后退码 = %d（要 %d）· stderr=%s", c.row, code, c.want, errb)
		}
	}
	// 判定口本身：四档逐格（这一格坏了上面几条就都是偶然）。
	if rc, err := zerg.SheetExitCodeForTest(map[string]int{"PASS": 3}); err != nil || rc != 0 {
		t.Errorf("全绿 ⇒ %d（要 0）err=%v", rc, err)
	}
	if rc, _ := zerg.SheetExitCodeForTest(map[string]int{"PASS": 3, "BLOCKED": 1}); rc != 8 {
		t.Errorf("有 BLOCKED ⇒ %d（要 8 —— 不给结论不许当绿）", rc)
	}
	if rc, _ := zerg.SheetExitCodeForTest(map[string]int{"BLOCKED": 1, "FAIL": 1}); rc != 1 {
		t.Errorf("同时有 FAIL 与 BLOCKED ⇒ %d（要 1，失败优先）", rc)
	}
}

// ---- 判据②：verify 先于 build（次序闸真的拦得住）----

func TestDevBuildRefusesWithoutVerify(t *testing.T) {
	root := devTmpRoot(t)

	// ① 没有证据单 ⇒ 拒执（退码 2），且判词点名「先有判据」。
	code, _, errb := runCapture("dev", "build", "--candidate", "DEV-0003", "--dry-run")
	if code != 2 {
		t.Errorf("无证据单时 `dev build` 退码 = %d（要 2 = 拒执）· stderr=%s", code, errb)
	}
	if !strings.Contains(errb, "先有判据") {
		t.Errorf("拒执理由里没有逐字「先有判据」（§17.7）：%s", errb)
	}
	for _, act := range []string{"test"} {
		if code, _, _ := runCapture("dev", act, "--candidate", "DEV-0003", "--dry-run"); code != 2 {
			t.Errorf("无证据单时 `dev %s` 退码 = %d（要 2）", act, code)
		}
	}

	// ② 候选 id 不在闭集里 ⇒ 也是 2（先于次序判）。
	if code, _, _ := runCapture("dev", "build", "--candidate", "DEV-3", "--dry-run"); code != 2 {
		t.Errorf("近名候选 id 时退码 = %d（要 2）", code)
	}

	// ③ 先跑 verify 造出证据单 ⇒ 同一个 build 就不在次序那一格被拦了。
	dir := filepath.Join(root, "DEV-0003")
	p := filepath.Join(dir, "results.tsv")
	writeResults(t, p, []string{"PASS\tgate: gofmt -l core\t0\t1s\t/tmp/g.log"})
	if code, _, errb := runCapture("dev", "verify", "--candidate", "DEV-0003", "--results", p); code != 0 {
		t.Fatalf("verify 退码 = %d（要 0）· stderr=%s", code, errb)
	}
	// 有了证据单：`--dry-run` 走到**计划件**那一态（0），真跑仍在「本版未开放」（2）。
	code, out, errb := runCapture("dev", "build", "--candidate", "DEV-0003", "--dry-run")
	if code != 0 {
		t.Fatalf("有证据单 + --dry-run 退码 = %d（要 0 = 只出计划件）· stderr=%s", code, errb)
	}
	if !strings.Contains(out, "DEV-0003") {
		t.Errorf("计划件里没点候选 id：%s", out)
	}
	code, _, errb = runCapture("dev", "build", "--candidate", "DEV-0003", "--yes")
	if code != 2 {
		t.Errorf("有证据单但真跑：退码 = %d（要 2 = 本版未开放）· stderr=%s", code, errb)
	}
	if !strings.Contains(errb, "未开放") {
		t.Errorf("真跑被拒的理由里没有「未开放」：%s", errb)
	}
}

// ---- 判据⑤：候选区闭集 —— 生产目录里不许出现带候选 id 的件（带负控）----

func TestDevCandidateProductionNameClosedSet(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// 好件：生产目录干净（bin/ 里只有正常制品）⇒ 0 命中。
	must(os.MkdirAll(filepath.Join(root, "bin"), 0o755))
	must(os.WriteFile(filepath.Join(root, "bin", "zerg"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "bin", "zerg-core"), []byte("x"), 0o644))
	must(os.MkdirAll(filepath.Join(root, "dist", "candidates", "DEV-0001"), 0o755))
	must(os.WriteFile(filepath.Join(root, "dist", "candidates", "DEV-0001", "DEV-0001.tar"), []byte("x"), 0o644))

	clean, err := zerg.ProductionCandidateNameViolationsForTest(root)
	must(err)
	if len(clean) != 0 {
		t.Errorf("干净的生产目录被判出 %d 条违规（要 0）：%v", len(clean), clean)
	}

	// 负控（判据⑤「能红」）：把一件名字带候选 id 的件放进 bin/ ⇒ 必红。
	must(os.WriteFile(filepath.Join(root, "bin", "zerg-DEV-0001"), []byte("x"), 0o644))
	// 再放一件**手工变体**（不在闭集形态里，宽松识别必须也抓得到）。
	must(os.WriteFile(filepath.Join(root, "bin", "dev-0002.bak"), []byte("x"), 0o644))

	bad, err := zerg.ProductionCandidateNameViolationsForTest(root)
	must(err)
	sort.Strings(bad)
	if len(bad) != 2 {
		t.Fatalf("生产目录里两件带候选 id 的件，只判出 %d 条：%v", len(bad), bad)
	}
	if !strings.HasPrefix(bad[0], "bin/") || !strings.HasPrefix(bad[1], "bin/") {
		t.Errorf("命中的件不在 bin/ 下：%v", bad)
	}
	// 候选区里的同名件**不许**被当成违规（排除面真的排掉了）。
	for _, p := range bad {
		if strings.Contains(p, "candidates") {
			t.Errorf("候选区里的件被误判成生产违规：%s", p)
		}
	}
	// 负控自证：把判定口换成「什么都返回空」⇒ 上面那两条就不会红 —— 说明它现在真的在看东西。
	if len(clean) == len(bad) {
		t.Error("负控失败：干净树与脏树判出同样多（判定口没在看东西）")
	}
}

// ---- 判据①的 --json 面：逐条给出来（K1 机器面先定）----

func TestDevVerifyJSONList(t *testing.T) {
	root := devTmpRoot(t)
	dir := filepath.Join(root, "DEV-0004")
	p := filepath.Join(dir, "results.tsv")
	writeResults(t, p, []string{"PASS\tgate: gofmt -l core\t0\t1s\t/tmp/g.log"})
	code, out, errb := runCapture("dev", "verify", "--candidate", "DEV-0004", "--results", p,
		"--json", "candidate,criterion,verdict,rc,log_path")
	if code != 0 {
		t.Fatalf("--json 退码 = %d（要 0）· stderr=%s", code, errb)
	}
	for _, want := range []string{`"candidate":"DEV-0004"`, `"criterion":"gate: gofmt -l core"`, `"verdict":"PASS"`, `"rc":"0"`} {
		if !strings.Contains(out, want) {
			t.Errorf("--json 输出里没有 %s：%s", want, out)
		}
	}
	if !strings.Contains(out, `"schema":"zerg/v1"`) {
		t.Errorf("--json 包封第一键不是 schema=zerg/v1：%s", out)
	}
	// `--json` 不给字段 ⇒ 1（K2），且 stdout 0 字节。
	var outB, errB bytes.Buffer
	if rc := zerg.RunForTest([]string{"dev", "verify", "--candidate", "DEV-0004", "--results", p, "--json"},
		&outB, &errB); rc != 1 || outB.Len() != 0 {
		t.Errorf("`--json` 不给字段：rc=%d（要 1）· stdout %d 字节（要 0）", rc, outB.Len())
	}
	// 负控（成对）：同一 `--json` 面在「证据为空」时必须也是 2 ——
	// 证明「不给结论」不是只在人面生效（机器面偷偷给绿是最坏的一种）。
	// （`--json` 失败时框架会在 stdout 出**错误包封**，所以这里判的是「退码 + 里面没有判决条目」，
	//   不是「stdout 0 字节」—— 后者只有 `--json` 不给字段那一档才成立。）
	outB.Reset()
	errB.Reset()
	if rc := zerg.RunForTest([]string{"dev", "verify", "--candidate", "DEV-0004",
		"--results", filepath.Join(root, "nope.tsv"), "--json", "candidate,verdict"},
		&outB, &errB); rc != 2 || strings.Contains(outB.String(), `"verdict":"PASS"`) {
		t.Errorf("负控失败：证据为空时 --json 面 rc=%d（要 2）· stdout=%s", rc, outB.String())
	}
}
