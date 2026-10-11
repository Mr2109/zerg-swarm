// family_publish_accept_native_test.go —— `zerg publish accept`（收口编排件）回归牙。
//
// 覆盖：① 三档聚合（0/1/2）② 负控 4 条（某门造红 ⇒ 1 · 缺件/跑不动 ⇒ 2 fail-closed ·
// `--out` 缺 `--yes` ⇒ 不落件 · `gates_total` 漂移 ⇒ 红）③ 验-3/验-4 适配器夹具。
// ★ 夹具只落仓外（`t.TempDir()`）· ✗ 不碰盘上件。
package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// gitCmd —— 仓外夹具用的 git（`-C <dir>` · 只读/建夹具仓）。
func gitCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Dir = dir
	return cmd
}

// paccStubStep —— 合成一格（取数口恒返回 rc · 用于负控注入）。
func paccStubStep(id string, rc int) paccStep {
	return paccStep{
		id: id, title: id, kind: paccKindAdapter, criteria: "test-stub",
		run: func(root string, out io.Writer) int { return rc },
	}
}

// ① 三档聚合（真跑 paccAggregate）。
func TestPublishAcceptAggregateThreeTiers(t *testing.T) {
	cases := []struct {
		name string
		rcs  []int
		want int
	}{
		{"全绿 ⇒ 收口 0", []int{0, 0, 0, 0, 0, 0, 0, 0}, 0},
		{"任一红 ⇒ 1", []int{0, 1, 0, 0, 0, 0, 0, 0}, 1},
		{"缺件 2 ⇒ 2（fail-closed）", []int{0, 0, 2, 0, 0, 0, 0, 0}, 2},
		{"红 + 缺件 ⇒ 2（fail-closed 优先 · 不折成 0/1）", []int{1, 2, 0, 0, 0, 0, 0, 0}, 2},
		{"表外退码 8 ⇒ 2（判不了）", []int{0, 8, 0, 0, 0, 0, 0, 0}, 2},
	}
	for _, c := range cases {
		if got := paccAggregate(c.rcs); got != c.want {
			t.Errorf("%s：paccAggregate(%v) = %d，要 %d", c.name, c.rcs, got, c.want)
		}
	}
}

// paccRunSynthetic —— 用合成步表真跑 publishAcceptRun（跑完还原全局表）。返回 rc、stdout、stderr。
func paccRunSynthetic(t *testing.T, steps []paccStep, extra *invocation) (int, string, string) {
	t.Helper()
	orig := publishAcceptSteps
	publishAcceptSteps = steps
	t.Cleanup(func() { publishAcceptSteps = orig })

	root := t.TempDir()
	t.Setenv("ZERG_REPO", root)

	inv := &invocation{path: []string{"publish", "accept"}, orig: []string{"publish", "accept"}}
	if extra != nil {
		inv.kv = extra.kv
		inv.yes = extra.yes
		inv.dryRun = extra.dryRun
		inv.jsonGiven = extra.jsonGiven
		inv.fields = extra.fields
	}
	var so, se bytes.Buffer
	rc := publishAcceptRun(inv, &so, &se)
	return rc, so.String(), se.String()
}

// ② 负控①：某门造 rc=1 ⇒ 聚合=1（证明 AND 有牙 · 不是空过）。
func TestPublishAcceptNegativeControlRed(t *testing.T) {
	rc, _, _ := paccRunSynthetic(t, []paccStep{
		paccStubStep("验-1", 0),
		paccStubStep("验-2", 1),
		paccStubStep("验-3", 0),
	}, nil)
	if rc != 1 {
		t.Fatalf("注一门 rc=1 ⇒ 聚合要 1，得 %d", rc)
	}
}

// ③ 负控②：缺件/跑不动 ⇒ 聚合=2（fail-closed · ✗ 不把「跑不动」当「没病」）。
func TestPublishAcceptNegativeControlBlocked(t *testing.T) {
	rc, _, _ := paccRunSynthetic(t, []paccStep{
		paccStubStep("验-1", 0),
		paccStubStep("验-2", 2),
		paccStubStep("验-3", 0),
	}, nil)
	if rc != 2 {
		t.Fatalf("注一门 rc=2 ⇒ 聚合要 2（fail-closed），得 %d", rc)
	}
}

// 全绿 ⇒ 聚合 0（收口）。
func TestPublishAcceptAllGreen(t *testing.T) {
	rc, _, _ := paccRunSynthetic(t, []paccStep{
		paccStubStep("验-1", 0), paccStubStep("验-2", 0),
	}, nil)
	if rc != 0 {
		t.Fatalf("全绿 ⇒ 要 0，得 %d", rc)
	}
}

// ④ 负控③：`--out` 缺 `--yes` ⇒ rc=2 且**不落件**（D2 档 · fail-closed）。
func TestPublishAcceptOutNeedsYes(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "report.txt")
	rc, _, _ := paccRunSynthetic(t, []paccStep{paccStubStep("验-1", 0)}, &invocation{
		kv: map[string][]string{"--out": {outPath}},
	})
	if rc != 2 {
		t.Fatalf("`--out` 缺 `--yes` ⇒ 要 2，得 %d", rc)
	}
	if _, err := os.Stat(outPath); err == nil {
		t.Fatalf("缺 `--yes` ⇒ ✗ 不许落件，但 %s 已落", outPath)
	}
}

// ④b `--out` + `--yes` ⇒ 落件 + 聚合退码。
func TestPublishAcceptOutWithYesLands(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "report.txt")
	rc, _, _ := paccRunSynthetic(t, []paccStep{paccStubStep("验-1", 1)}, &invocation{
		kv:  map[string][]string{"--out": {outPath}},
		yes: true,
	})
	if rc != 1 {
		t.Fatalf("有红 ⇒ 要 1，得 %d", rc)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("`--out --yes` ⇒ 应落件：%v", err)
	}
	if !bytes.Contains(b, []byte("验-1")) {
		t.Fatalf("报告件应含逐条读数，得：%q", string(b))
	}
}

// ④c `--out` 与 `--json` 同给 ⇒ 用法错 2。
func TestPublishAcceptOutJsonConflict(t *testing.T) {
	rc, _, _ := paccRunSynthetic(t, []paccStep{paccStubStep("验-1", 0)}, &invocation{
		kv:        map[string][]string{"--out": {"/tmp/never"}},
		jsonGiven: true, fields: []string{"id"},
	})
	if rc != 2 {
		t.Fatalf("`--out`+`--json` ⇒ 要 2，得 %d", rc)
	}
}

// paccWriteStubGate —— 写一枚仓外夹具 `precommit-gates.sh`（只认本单用到的三档）。
func paccWriteStubGate(t *testing.T, root string, jsonLine string) {
	t.Helper()
	dir := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/usr/bin/env bash\n" +
		"a=\"$1\"; b=\"$2\"; c=\"$3\"\n" +
		"if [ \"$a\" = \"--list\" ] && [ \"$b\" = \"--json\" ]; then printf '%s\\n' '" + jsonLine + "'; exit 0; fi\n" +
		"if [ \"$a\" = \"--list\" ]; then printf 'release\\ttri\\t两器产出一致性对拍（夹具）\\n'; exit 0; fi\n" +
		"if [ \"$a\" = \"--self-test\" ]; then exit 0; fi\n" +
		"exit 0\n"
	p := filepath.Join(dir, "precommit-gates.sh")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// ⑤ 负控④：`gates_total` 漂移（夹具造数）⇒ 验-5 红。
func TestPublishAcceptGatesTotalDrift(t *testing.T) {
	root := t.TempDir()
	// 内部自洽（count==steps==gates_total==2）但 ≠ 冻结值 211 ⇒ 漂移。
	paccWriteStubGate(t, root, `{"count":2,"gates_total":2,"steps":[{"name":"a"},{"name":"b"}]}`)

	gt, ok, note := paccGatesTotal(root)
	if !ok {
		t.Fatalf("夹具 gates_total 应读得到：%s", note)
	}
	if gt != 2 {
		t.Fatalf("夹具 gates_total 应 2，得 %d", gt)
	}
	if gt == paccGatesTotalFrozen {
		t.Fatalf("夹具值与冻结值不该相等")
	}
	var buf bytes.Buffer
	if rc := paccRunGateSelf(root, &buf); rc != 1 {
		t.Fatalf("gates_total 漂移 ⇒ 验-5 要红（1），得 %d（输出：%s）", rc, buf.String())
	}
}

// ⑤b `gates_total` == 冻结值（且自洽 count==steps）⇒ 验-5 绿。
func TestPublishAcceptGatesTotalPinned(t *testing.T) {
	root := t.TempDir()
	// 机读面自证要求 `gates_total==len(steps)` ⇒ 夹具须真给 N 步（N = 冻结值）。
	var sb strings.Builder
	sb.WriteString(`{"count":` + strconv.Itoa(paccGatesTotalFrozen) +
		`,"gates_total":` + strconv.Itoa(paccGatesTotalFrozen) + `,"steps":[`)
	for i := 0; i < paccGatesTotalFrozen; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"name":"s"}`)
	}
	sb.WriteString(`]}`)
	paccWriteStubGate(t, root, sb.String())
	var buf bytes.Buffer
	if rc := paccRunGateSelf(root, &buf); rc != 0 {
		t.Fatalf("gates_total==冻结值 ⇒ 验-5 要绿（0），得 %d（输出：%s）", rc, buf.String())
	}
}

// ⑥ 验-3 夹具：干净 git 仓 ⇒ 绿；造 1 个 dangling blob ⇒ 红（补抓 dangling）。
func TestPublishAcceptFsckFixture(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("无 git")
	}
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := gitCmd(repo, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	runGit("init", "-q")
	runGit("config", "user.email", "t@t.local")
	runGit("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "a.txt")
	runGit("commit", "-qm", "init")

	t.Setenv(paccEnvMirror, repo)
	var buf bytes.Buffer
	if rc := paccRunFsck("", &buf); rc != 0 {
		t.Fatalf("干净仓 ⇒ 验-3 要 0，得 %d（输出：%s）", rc, buf.String())
	}
	// 造 1 个 dangling blob（hash-object -w 不引用它）。
	cmd := gitCmd(repo, "hash-object", "-w", "--stdin")
	cmd.Stdin = bytes.NewBufferString("dangling\n")
	if _, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hash-object: %v", err)
	}
	buf.Reset()
	if rc := paccRunFsck("", &buf); rc != 1 {
		t.Fatalf("有 dangling ⇒ 验-3 要红（1），得 %d（输出：%s）", rc, buf.String())
	}
}

// ⑦ 验-4 夹具：记录值件在场且一致 ⇒ 绿；缺记录值 ⇒ 红。
func TestPublishAcceptRollbackFixture(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	file := filepath.Join(t.TempDir(), "artifact.bin")
	if err := os.WriteFile(file, []byte("payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(paccEnvRollback, file)

	// 缺记录值 ⇒ 红。
	var buf bytes.Buffer
	if rc := paccRunRollbackDigest("", &buf); rc != 1 {
		t.Fatalf("无记录值 ⇒ 验-4 要红（1），得 %d（输出：%s）", rc, buf.String())
	}

	// 落记录值（照退点归档体例：<状态目录>/退点归档-<日期>/<safeName>.sha256）。
	recDir := filepath.Join(stateDir, "退点归档-"+time.Now().Format("20060102"))
	if err := os.MkdirAll(recDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := paccSha256File(file)
	if now == "" {
		t.Skip("无 shasum")
	}
	rec := filepath.Join(recDir, devEditSafeName(file)+".sha256")
	if err := os.WriteFile(rec, []byte(now+"  "+file+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if rc := paccRunRollbackDigest("", &buf); rc != 0 {
		t.Fatalf("记录值一致 ⇒ 验-4 要 0，得 %d（输出：%s）", rc, buf.String())
	}
}

// ⑧ 门73（`check-flag-registry-parity`）六面读数 —— **只读**探针：进程内调门（`paccCallGate` ·
// 与 `publish accept` 同一条进程内入口）⇒ ✗ 不走 `zerg gate check`（那样会落 judge 台账）。
// 本格只**记读数**、不判绿（门73 的绿红归它自己；本单只借它核「新命令的旗标没造第二真源」）。
func TestPublishAcceptGate73FacesProbe(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Skip("无仓根（本探针只在真仓里读门73 读数）")
	}
	rc, log := paccCallGate(root, "check-flag-registry-parity", nil)
	t.Logf("门73 check-flag-registry-parity rc=%d\n%s", rc, log)
}

// ⑨ 三档出口**逐字**读数（真跑 publishAcceptRun · 合成步表 ⇒ 覆盖 0/1/2 三档）。
func TestPublishAcceptThreeTierExitLines(t *testing.T) {
	cases := []struct {
		name string
		rc   int
		want string
	}{
		{"全绿⇒收口", 0, "✅ 8 条收口验收全绿 ⇒ 收口（任一红即不收口）"},
		{"有红⇒不收口", 1, "❌ 有红 ⇒ 不收口"},
		{"判不了/缺件⇒不给结论", 2, "⚠ 有步「判不了／缺件」⇒ 不给结论（fail-closed · 不退 0 · 补齐后再跑）"},
	}
	for _, c := range cases {
		rc, _, se := paccRunSynthetic(t, []paccStep{paccStubStep("验-1", c.rc)}, nil)
		if rc != c.rc {
			t.Fatalf("%s：聚合要 %d，得 %d", c.name, c.rc, rc)
		}
		if !strings.Contains(se, c.want) {
			t.Fatalf("%s：出口行缺\nwant: %s\ngot:\n%s", c.name, c.want, se)
		}
		t.Logf("三档[%s] rc=%d · 出口行=%s", c.name, rc, c.want)
	}
}
