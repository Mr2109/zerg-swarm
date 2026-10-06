// check_gate_point_output_test.go —— `check_gate_point_output.go`（M2 搬运第 1 粒）的**成对负控**。
//
// 预期串 = 由「已落仓件重建的探针」**实测捕获**（非手抄），逐字节钉 rc/stdout/stderr 三面。
// ★ 含一条回归牙：`TestCgpoSelftestTmpCleanup` 钉 `defer os.RemoveAll(tmp)` 的回收——把它删掉该用例即红。
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func cgpoFixture(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "precommit-gates.sh"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root
}

func cgpoInvoke(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkGatePointOutputGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

const cgpoBodyA = "# c\nadd_step gates \"门1\" rc \"$R\" \"a.sh\"\nadd_step docs \"门2\" tri \"$R\" \"b.py\"\nadd_step gates \"门3\" rc \"$R\" \"c.sh\"\nadd_step doc2 \"门4\" weird \"$R\" \"d.py\"\n"

const cgpoWantGreen = "── 逐挂点输出表（挂点×档位×输出面）──\n  挂点               档位           输出面          门数\n  gates            rc           阻断面          2\n  docs             tri          阻断面          1\n  doc2             weird        （档位不在四档闭集内）  1\n  —— 共 4 步 · 3 个（挂点,档位）组\n"
const cgpoWantJSON = "{\"schema\": \"zerg/v1\", \"kind\": \"GatePointOutput\", \"steps\": 4, \"rows\": [{\"scope\": \"gates\", \"mode\": \"rc\", \"output_face\": \"阻断面\", \"count\": 2}, {\"scope\": \"docs\", \"mode\": \"tri\", \"output_face\": \"阻断面\", \"count\": 1}, {\"scope\": \"doc2\", \"mode\": \"weird\", \"output_face\": \"（档位不在四档闭集内）\", \"count\": 1}]}\n"
const cgpoWantHelp = "usage: check-gate-point-output.py [-h] [--root ROOT] [--json] [--self-test]\n\n逐挂点输出表元门（GAP-20260927-152）\n\noptional arguments:\n  -h, --help   show this help message and exit\n  --root ROOT\n  --json\n  --self-test\n"
const cgpoWantUnknown = "usage: check-gate-point-output.py [-h] [--root ROOT] [--json] [--self-test]\ncheck-gate-point-output.py: error: unrecognized arguments: --bogus\n"
const cgpoWantMissVal = "usage: check-gate-point-output.py [-h] [--root ROOT] [--json] [--self-test]\ncheck-gate-point-output.py: error: argument --root: expected one argument\n"

// 全绿档：逐字三面（含 rows 首遇序、counts、档位不在闭集内的回落文案）。
func TestCgpoGreenTableBytes(t *testing.T) {
	root := cgpoFixture(t, cgpoBodyA)
	rc, so, se := cgpoInvoke("--root", root)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, se)
	}
	if so != cgpoWantGreen {
		t.Fatalf("stdout 不符\n got=%q\nwant=%q", so, cgpoWantGreen)
	}
	if se != "" {
		t.Fatalf("stderr 应空, got=%q", se)
	}
}

func TestCgpoJSONBytesAndShape(t *testing.T) {
	root := cgpoFixture(t, cgpoBodyA)
	rc, so, se := cgpoInvoke("--root", root, "--json")
	if rc != 0 {
		t.Fatalf("rc=%d; stderr=%q", rc, se)
	}
	if so != cgpoWantJSON {
		t.Fatalf("json 不符\n got=%q\nwant=%q", so, cgpoWantJSON)
	}
	var m struct {
		Schema string `json:"schema"`
		Kind   string `json:"kind"`
		Steps  int    `json:"steps"`
		Rows   []struct {
			Scope, Mode, OutputFace string
			Count                   int
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(so), &m); err != nil {
		t.Fatalf("非法 JSON: %v", err)
	}
	if m.Schema != "zerg/v1" || m.Kind != "GatePointOutput" || m.Steps != 4 || len(m.Rows) != 3 {
		t.Fatalf("结构不符: %+v", m)
	}
	if m.Rows[0].Scope != "gates" || m.Rows[0].Count != 2 {
		t.Fatalf("首遇序/counts 不符: %+v", m.Rows)
	}
}

// 空转 ≠ 绿：0 步 ⇒ rc=2（不给结论），✗ 不许当绿。
func TestCgpoEmptyIsNotGreen(t *testing.T) {
	root := cgpoFixture(t, "# 无 add_step\n")
	rc, so, se := cgpoInvoke("--root", root)
	if rc != 2 {
		t.Fatalf("rc=%d want 2", rc)
	}
	if so != "" {
		t.Fatalf("stdout 应空, got=%q", so)
	}
	p := filepath.Join(root, "scripts", "gates", "precommit-gates.sh")
	if want := "⛔ 不给结论（rc=2）：现读 0 步（空转 = 假绿）—— " + p + "\n"; se != want {
		t.Fatalf("stderr 不符\n got=%q\nwant=%q", se, want)
	}
}

// 取不到输入 ⇒ rc=2。
func TestCgpoMissingInput(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nope")
	rc, so, se := cgpoInvoke("--root", root)
	if rc != 2 || so != "" {
		t.Fatalf("rc=%d stdout=%q", rc, so)
	}
	p := filepath.Join(root, "scripts", "gates", "precommit-gates.sh")
	if want := "⛔ 不给结论（rc=2）：门禁脚本不在盘：" + p + "\n"; se != want {
		t.Fatalf("stderr 不符\n got=%q\nwant=%q", se, want)
	}
}

// pyJoin 空段规则：--root "" ⇒ 路径回落相对串（✗ 不许变成 "/scripts/..."）。
func TestCgpoJoinEmptyRoot(t *testing.T) {
	rc, _, se := cgpoInvoke("--root", "")
	if rc != 2 {
		t.Fatalf("rc=%d want 2", rc)
	}
	if want := "⛔ 不给结论（rc=2）：门禁脚本不在盘：scripts/gates/precommit-gates.sh\n"; se != want {
		t.Fatalf("stderr 不符\n got=%q\nwant=%q", se, want)
	}
}

// 未知旗标 / --root 缺值：argparse 面逐字（rc=2 与 usage+error）。
func TestCgpoArgparseFaces(t *testing.T) {
	rc, _, se := cgpoInvoke("--bogus")
	if rc != 2 || se != cgpoWantUnknown {
		t.Fatalf("未知旗标 rc=%d se=%q", rc, se)
	}
	rc, _, se = cgpoInvoke("--root")
	if rc != 2 || se != cgpoWantMissVal {
		t.Fatalf("缺值 rc=%d se=%q", rc, se)
	}
}

// -h：rc=0 且正文逐字。
func TestCgpoHelpFace(t *testing.T) {
	rc, so, se := cgpoInvoke("-h")
	if rc != 0 || se != "" {
		t.Fatalf("rc=%d se=%q", rc, se)
	}
	if so != cgpoWantHelp {
		t.Fatalf("help 不符\n got=%q\nwant=%q", so, cgpoWantHelp)
	}
}

// argparse 唯一前缀缩写：--ro → --root。
func TestCgpoPrefixAbbrev(t *testing.T) {
	root := cgpoFixture(t, cgpoBodyA)
	rc, so, _ := cgpoInvoke("--ro", root)
	if rc != 0 || so != cgpoWantGreen {
		t.Fatalf("rc=%d", rc)
	}
}

// splitlines 多边界（\x0c 夹非换行 + CRLF + 裸 \r）：须切出 4 步。
func TestCgpoSplitlinesBoundaries(t *testing.T) {
	root := cgpoFixture(t, "add_step a \"n1\" rc \"$R\" \"x\"\x0cadd_step b \"n2\" tri \"$R\" \"y\"\r\nadd_step c \"n3\" tri-report \"$R\" \"z\"\radd_step d \"n4\" empty \"$R\" \"w\"\n")
	rc, so, _ := cgpoInvoke("--root", root, "--json")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	var m struct {
		Steps int `json:"steps"`
	}
	if err := json.Unmarshal([]byte(so), &m); err != nil || m.Steps != 4 {
		t.Fatalf("steps=%d err=%v", m.Steps, err)
	}
}

// 读不了的件（chmod 000）⇒ rc=2 ＋ py 形态的 EACCES 正文（非 root 才可复现）。
func TestCgpoUnreadableInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows 无 POSIX 权限档")
	}
	if os.Geteuid() == 0 {
		t.Skip("root 无视读权限位")
	}
	root := cgpoFixture(t, "add_step a \"n1\" rc \"$R\" \"x\"\n")
	p := filepath.Join(root, "scripts", "gates", "precommit-gates.sh")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(p, 0o644)
	rc, _, se := cgpoInvoke("--root", root)
	if rc != 2 {
		t.Fatalf("rc=%d want 2", rc)
	}
	if want := "⛔ 不给结论（rc=2）：门禁脚本读不到：[Errno 13] Permission denied: '" + p + "'\n"; se != want {
		t.Fatalf("stderr 不符\n got=%q\nwant=%q", se, want)
	}
}

// ★ 回归牙：--self-test 的临时目录必须自回收（defer os.RemoveAll）⇒ 私有 TMPDIR 跑前跑后计数 delta=0。
func TestCgpoSelftestTmpCleanup(t *testing.T) {
	priv := t.TempDir()
	t.Setenv("TMPDIR", priv) // os.TempDir() 每次现读 ⇒ 把自检产物收进私有域
	count := func() int {
		ents, err := os.ReadDir(priv)
		if err != nil {
			t.Fatalf("readdir: %v", err)
		}
		n := 0
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "tmp") {
				n++
			}
		}
		return n
	}
	before := count()
	rc, so, _ := cgpoInvoke("--self-test")
	after := count()
	if rc != 0 {
		t.Fatalf("self-test rc=%d", rc)
	}
	if !strings.Contains(so, "自检 PASS（3 格）") {
		t.Fatalf("self-test 未 PASS: %q", so)
	}
	if after != 0 || before != 0 {
		t.Fatalf("临时目录未回收：before=%d after=%d（defer os.RemoveAll 缺失？）", before, after)
	}
}
