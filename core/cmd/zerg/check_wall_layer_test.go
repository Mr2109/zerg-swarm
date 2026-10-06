// check_wall_layer_test.go —— `check_wall_layer.go` 的**定向单测 · 成对负控**（M2-0 路3）。
//
// 口径（与 py 原件两态对拍之外的「有牙」证据）：本用例**逐格判**rc 与**逐字判** stdout/stderr ——
// 任一条规则被哑化（R1 闭集不判 / R2 space 不判 / R3 argv 不判 / 空转当绿 / 未知旗标放行）⇒ **当场 FAIL**。
// 负控有牙的实证 = 把落在**仓外副本**上的同一件哑化一条判据后重跑本用例 ⇒ FAIL（见 SUMMARY 的逐字读数）。
//
// ✗ 本件不接线、不注册命令、不删 py 原件（M2-0 边界）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const wlTestGoodSrc = "const (\n\tlayerHost  = \"host\"\n\tlayerNode  = \"node\"\n" +
	"\tlayerSpace = \"space\"\n)\n" +
	"var c = command{layer: \"host\"}\n"

func wlTestTree(t *testing.T, src string, planRel, plan string, extra map[string]string) string {
	t.Helper()
	d := t.TempDir()
	cli := filepath.Join(d, "core", "cmd", "zerg")
	if err := os.MkdirAll(cli, 0o755); err != nil {
		t.Fatal(err)
	}
	if src != "" {
		if err := os.WriteFile(filepath.Join(cli, "wall.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if plan != "" {
		p := filepath.Join(d, planRel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(plan), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, body := range extra {
		p := filepath.Join(d, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func wlTestRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := walllayerGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckWallLayerGatePort(t *testing.T) {
	planOK := `{"argv": ["--ro-bind", "/usr", "/usr", "zerg-egg"]}`

	// ① 正控：全绿 ⇒ rc=0 且 stdout 逐字到「全绿」行。
	t.Run("正控-全绿", func(t *testing.T) {
		d := wlTestTree(t, wlTestGoodSrc, "wall/plan.json", planOK, nil)
		rc, out, errb := wlTestRun(d)
		if rc != 0 {
			t.Fatalf("正控-全绿 失败：得 rc=%d out=%q err=%q", rc, out, errb)
		}
		if !strings.Contains(out, "  层级闭集 = host/node/space（三档）\n") ||
			!strings.Contains(out, "⇒ 全绿（rc=0）\n") {
			t.Fatalf("正控-全绿 stdout 不符：%q", out)
		}
		if errb != "" {
			t.Fatalf("正控-全绿 stderr 应空：%q", errb)
		}
	})

	// ② 负控 R1：自造第四档 ⇒ rc=1 且逐字点名（哑化「闭集」判据必 FAIL）。
	t.Run("负控-R1-改档", func(t *testing.T) {
		src := strings.Replace(wlTestGoodSrc, `layerNode  = "node"`, `layerNode  = "target"`, 1)
		d := wlTestTree(t, src, "wall/plan.json", `{"argv": ["ok"]}`, nil)
		rc, out, _ := wlTestRun(d)
		if rc != 1 {
			t.Fatalf("负控-R1 失败：得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "✗ R1 层级闭集被改：layerNode = 'target'（闭集只有 host/node/space）\n") {
			t.Fatalf("负控-R1 未逐字点名：%q", out)
		}
		if !strings.Contains(out, "⇒ 门禁红（rc=1）：茧壁边界被放宽\n") {
			t.Fatalf("负控-R1 结论行不符：%q", out)
		}
	})

	// ③ 负控 R2：命令树出现 `layer: "space"` ⇒ rc=1（哑化 space 判据必 FAIL）。
	t.Run("负控-R2-space标记", func(t *testing.T) {
		src := strings.Replace(wlTestGoodSrc, `layer: "host"`, `layer: "space"`, 1)
		d := wlTestTree(t, src, "wall/plan.json", `{"argv": ["ok"]}`, nil)
		rc, out, _ := wlTestRun(d)
		if rc != 1 {
			t.Fatalf("负控-R2 失败：得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "✗ R2 命令树里出现 `layer: \"space\"`") {
			t.Fatalf("负控-R2 未点名：%q", out)
		}
	})
	// ③b 负控 R2（闭集外的命令层）：命令树出现 `layer: "target"`（非 space、亦非闭集）⇒ rc=1
	//    （本格 = 真仓那次「孤立字面量被 R2 咬住」所对应的判据分支；哑化「闭集外判」必 FAIL）。
	t.Run("负控-R2-闭集外命令层", func(t *testing.T) {
		src := strings.Replace(wlTestGoodSrc, `layer: "host"`, `layer: "target"`, 1)
		d := wlTestTree(t, src, "wall/plan.json", `{"argv": ["ok"]}`, nil)
		rc, out, _ := wlTestRun(d)
		if rc != 1 {
			t.Fatalf("负控-R2-闭集外 失败：得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "✗ R1/R2 命令树里出现闭集外的层级 'target'\n") {
			t.Fatalf("负控-R2-闭集外 未逐字点名：%q", out)
		}
	})

	// ④ 负控 R3：plan 的 argv 出现命令面件 ⇒ rc=1（哑化 argv 判据必 FAIL）。
	t.Run("负控-R3-argv命令面", func(t *testing.T) {
		d := wlTestTree(t, wlTestGoodSrc, "wall/plan.json",
			`{"argv": ["/bin/sh", "-c", "exec \"$@\"", "zerg-core"]}`, nil)
		rc, out, _ := wlTestRun(d)
		if rc != 1 {
			t.Fatalf("负控-R3 失败：得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "✗ R3 wall/plan.json 的 argv 里出现命令面件 'zerg-core'\n") {
			t.Fatalf("负控-R3 未逐字点名：%q", out)
		}
	})

	// ⑤ 负控 R3（agent 树 + .txt 行分隔 JSON 两路都要认）
	t.Run("负控-R3-agent与txt两路", func(t *testing.T) {
		d := wlTestTree(t, wlTestGoodSrc, "wall/plan.json", `{"argv": ["ok"]}`,
			map[string]string{
				"agent/run/plan.json":      `{"argv": ["zerg-wall", "-x"]}`,
				"wall/evidence-plan-1.txt": "{\"argv\": [\"precommit-gates.sh\"]}\n",
			})
		rc, out, _ := wlTestRun(d)
		if rc != 1 {
			t.Fatalf("负控-R3-两路 失败：得 rc=%d out=%q", rc, out)
		}
		for _, want := range []string{
			"✗ R3 agent/run/plan.json 的 argv 里出现命令面件 'zerg-wall'\n",
			"✗ R3 wall/evidence-plan-1.txt 的 argv 里出现命令面件 'precommit-gates.sh'\n",
			"  扫到 plan 产物 3 份（argv 均无命令面件）\n",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("负控-R3-两路 缺读数 %q：%q", want, out)
			}
		}
	})

	// ⑥ 空转 ≠ 绿：一份带 argv 的 plan 都没有 ⇒ rc=2（哑化「空转」判据必 FAIL）。
	t.Run("空转-不给结论", func(t *testing.T) {
		d := wlTestTree(t, wlTestGoodSrc, "", "", nil)
		rc, out, _ := wlTestRun(d)
		if rc != 2 {
			t.Fatalf("空转 失败：得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "⛔ 不给结论：一份带 `argv` 的 plan 产物都没找到（R3 空转 = 假覆盖）\n") {
			t.Fatalf("空转 未逐字点名：%q", out)
		}
	})

	// ⑦ 取不到输入：根不存在 ⇒ rc=2 且只印那一行。
	t.Run("取不到输入-根不存在", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope")
		rc, out, _ := wlTestRun(missing)
		if rc != 2 {
			t.Fatalf("取不到输入 失败：得 rc=%d out=%q", rc, out)
		}
		if out != "⛔ 不给结论：找不到 core/cmd/zerg/wall.go（本门只认命令面真源所在的那棵树）\n" {
			t.Fatalf("取不到输入 stdout 不符：%q", out)
		}
	})

	// ⑧ 未知旗标 ⇒ rc=2（stderr 逐字；哑化「未知旗标不放行」必 FAIL）。
	t.Run("未知旗标-拒", func(t *testing.T) {
		rc, out, errb := wlTestRun("--no-such-flag")
		if rc != 2 {
			t.Fatalf("未知旗标 失败：得 rc=%d", rc)
		}
		if out != "" {
			t.Fatalf("未知旗标 stdout 应空：%q", out)
		}
		want := "usage: check-wall-layer.py [-h] [--self-test] [root]\n" +
			"check-wall-layer.py: error: unrecognized arguments: --no-such-flag\n"
		if errb != want {
			t.Fatalf("未知旗标 stderr 不符：\n得 %q\n要 %q", errb, want)
		}
	})

	// ⑨ `--json` 无机器读面 ⇒ rc=2（拒收逐字）。
	t.Run("json无机器面-拒", func(t *testing.T) {
		rc, out, errb := wlTestRun("--json")
		if rc != 2 || out != "" {
			t.Fatalf("--json 失败：rc=%d out=%q", rc, out)
		}
		want := "⛔ 本门无机器读面：`--json` 在本门**未实现**（缺口 Q-176）\n" +
			"   字段表：无（本门只出人读面）\n" +
			"⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n"
		if errb != want {
			t.Fatalf("--json stderr 不符：\n得 %q\n要 %q", errb, want)
		}
	})

	// ⑩ `--self-test` 五格三档 ⇒ rc=0 且结论行逐字。
	t.Run("自检-五格三档", func(t *testing.T) {
		rc, out, _ := wlTestRun("--self-test")
		if rc != 0 {
			t.Fatalf("自检 失败：得 rc=%d out=%q", rc, out)
		}
		if !strings.Contains(out, "自检结论: 全过\n") {
			t.Fatalf("自检 结论行不符：%q", out)
		}
		if !strings.Contains(out, "自检⑤ 没有 plan 产物 ⇒ rc=2（要 2 —— 空转不给结论）\n") {
			t.Fatalf("自检⑤ 缺：%q", out)
		}
	})

	// ⑪ 真仓现读（cwd = 本包目录 ⇒ 入口按 `.` 上溯到真仓根）⇒ rc=0。
	t.Run("真仓现读-缺省根", func(t *testing.T) {
		rc, out, errb := wlTestRun(".")
		if rc != 0 {
			t.Fatalf("真仓现读 失败：rc=%d out=%q err=%q", rc, out, errb)
		}
		if !strings.Contains(out, "  层级闭集 = host/node/space（三档）\n") ||
			!strings.Contains(out, "⇒ 全绿（rc=0）\n") {
			t.Fatalf("真仓现读 stdout 不符：%q", out)
		}
	})

	// ⑫ 读失败不再静默（仓外牙 · 回归）：待扫 `.go` 件不可读 ⇒ 登记 `readFails` ⇒ rc=2，
	//    BLOCKED 头落**本件实际通道 stdout**（`wlCheck` 无 stderr 形参 ⇒ BLOCKED 随 lines 由
	//    入口 `walllayerGateMain` 打 stdout）+ 逐件点名那一件。
	t.Run("读失败-BLOCKED落stdout", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root 下 chmod 000 仍可读 ⇒ 该格无鉴别力")
		}
		blockedRel := "core/cmd/zerg/blocked.go"
		d := wlTestTree(t, wlTestGoodSrc, "wall/plan.json",
			`{"argv": ["--ro-bind", "/usr", "/usr", "zerg-egg"]}`,
			map[string]string{blockedRel: "package main\n"})
		blocked := filepath.Join(d, blockedRel)
		if err := os.Chmod(blocked, 0o000); err != nil {
			t.Fatalf("chmod 000 失败：%v", err)
		}
		defer os.Chmod(blocked, 0o644)
		rc, out, errb := wlTestRun(d)
		if rc != 2 {
			t.Fatalf("读失败应 rc=2（不给结论）：得 rc=%d out=%q err=%q", rc, out, errb)
		}
		if !strings.Contains(out, "⛔ BLOCKED：以下件读失败，覆盖面不完整 —— 不给结论（假覆盖）\n") {
			t.Fatalf("BLOCKED 头未落 stdout（本件实际通道）：%q", out)
		}
		if !strings.Contains(out, "   · "+blocked+"（") {
			t.Fatalf("未逐件点名读失败件：%q", out)
		}
		if errb != "" {
			t.Fatalf("本件 BLOCKED 不走 stderr ⇒ stderr 应空：%q", errb)
		}
	})
}
