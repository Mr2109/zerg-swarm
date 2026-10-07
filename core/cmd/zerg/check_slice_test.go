// check_slice_test.go —— Go 真搬的**成对负控**定向单测（M2 · A 类最后一枚）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论(rc=2)」三态 + JSON null 面两态，
// 任一态被写死都必失败。用例名：TestCheckSliceGatePort（-run TestCheckSliceGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// csGoodSliceJSON —— 嵌入式合法片（取自探针集 template-正例 · 落点在测试里改写成夹具内文件）。
const csGoodSliceJSON = `{
  "defect_id": "DEF-20260918-S3-01",
  "landing": "../Zerg-内部文档/01-设计/切片合同-模板.md:1",
  "the_one_thing": "When 模板骨架已落盘（S3-C0 完成）时，../Zerg-内部文档/01-设计/切片合同-模板.md 的抬头必须写明 BCP 14 固定句。",
  "context_needed": [
    {
      "kind": "snippet",
      "value": "本片中的 MUST/SHOULD/MAY 按 BCP 14 解释（BCP 14 = RFC 2119 + RFC 8174；RFC 8174 逐字：only UPPERCASE usage of the key words have the defined special meanings）。"
    },
    {
      "kind": "file",
      "value": "../Zerg-内部文档/01-设计/切片合同-模板.md"
    },
    {
      "kind": "cmd",
      "value": "awk '/^## 0\\./,/^## 1\\./' ../Zerg-内部文档/01-设计/切片合同-模板.md | grep -c 'only UPPERCASE usage of the key words'"
    }
  ],
  "open_questions": "无",
  "criteria": [
    {
      "id": "C1",
      "cmd": "awk '/^## 0\\./,/^## 1\\./' ../Zerg-内部文档/01-设计/切片合同-模板.md | grep -c 'only UPPERCASE usage of the key words'",
      "expect": "1",
      "state": "改动前：红（骨架版 §0 无该句 ⇒ 输出 0，rc=1）｜落盘后：绿（实跑 = 1）"
    },
    {
      "id": "C2",
      "cmd": "awk '/^## 0\\./,/^## 1\\./' ../Zerg-内部文档/01-设计/切片合同-模板.md | grep -c 'MUST/SHOULD/MAY 按 BCP 14 解释'",
      "expect": "1",
      "state": "改动前：红（输出 0）｜落盘后：绿（实跑 = 1）"
    },
    {
      "id": "C3",
      "cmd": "awk '/^## 0\\./,/^## 1\\./' ../Zerg-内部文档/01-设计/切片合同-模板.md | grep -c 'BCP 14 subseries contains 2 RFCs'",
      "expect": "1",
      "state": "改动前：红（输出 0）｜落盘后：绿（实跑 = 1）"
    }
  ],
  "gate_cmds": [
    {
      "cmd": "awk '/^## 0\\./,/^## 1\\./' ../Zerg-内部文档/01-设计/切片合同-模板.md | grep -c 'only UPPERCASE usage of the key words'",
      "expect": {
        "exit": 0,
        "stdout": "1"
      }
    },
    {
      "cmd": "awk '/^## 0\\./,/^## 1\\./' ../Zerg-内部文档/01-设计/切片合同-模板.md | grep -c 'MUST/SHOULD/MAY 按 BCP 14 解释'",
      "expect": {
        "exit": 0,
        "stdout": "1"
      }
    },
    {
      "cmd": "awk '/^## 0\\./,/^## 1\\./' ../Zerg-内部文档/01-设计/切片合同-模板.md | grep -c 'BCP 14 subseries contains 2 RFCs'",
      "expect": {
        "exit": 0,
        "stdout": "1"
      }
    }
  ],
  "output_format": "① 正控（改动后）：C1/C2/C3 各跑一次，贴命令 + 原始输出 + 退出码（实跑：三条各 1）；② 负控（改动前必红）：把抬头那句改坏后重跑同三条命令，必须红（已实跑：三条的作用域计数由 1 变 0，rc=1）；③ 牙齿计数：C1/C2/C3 = 3/3 条都由绿变红；④ 还原：负控在副本上做，不动工作树原件，还原后原件 sha 逐字节一致。",
  "exit_cond": {
    "success": [
      "C1",
      "C2",
      "C3"
    ],
    "failure": "改动后任一命令输出与期望不符（C1 ≠ 1 / C2 无匹配行 / C3 ≠ 1），或负控三条中任一未由绿变红，且原地重试 ≤1 次仍不符 ⇒ 记失败，附命令、退出码、原始输出",
    "escalate": {
      "to": "Mr2109",
      "times": 2,
      "fields": [
        "片 id",
        "命令",
        "退出码",
        "原文",
        "时间"
      ]
    }
  },
  "hard_limits": [
    "① 只改 ../Zerg-内部文档/01-设计/切片合同-模板.md 一个文件（R7 默认单文件粒度）",
    "② 不改 v2.1 稿、不改 scripts/、不动 git、不删除任何文件",
    "③ 不新增 v2.1 之外的规则或数字"
  ],
  "side_effects": "只读",
  "escalate_if": "需要写工作树之外的文件 / 需要 git 写 / 需要删除任何文件 / 需要触网 ⇒ 升级 Mr2109",
  "budget": {
    "value": "steps_max 3 · tool_calls_max 2 · wall_s_max 43.6",
    "source": "~/.zerg/egg-profiles/task-budget-calib.yaml（常数只能当初始值）"
  },
  "atomicity": "落在执行者标定技能/门禁闭包内（该闭包字段待标定档案补齐），且 criteria 可在一次调用内判定（三条各是一条 grep）",
  "slice_id": "S3-C1",
  "depends_on": "无",
  "context_budget": {
    "files": 1,
    "tokens": 400,
    "source": "~/.zerg/egg-profiles/task-budget-calib.yaml"
  }
}`

func csRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var o, e bytes.Buffer
	rc := checkSliceGateMain(&o, &e, argv)
	return rc, o.String(), e.String()
}

func TestCheckSliceGatePort(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "gates", "check-slice.py"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := strings.Replace(csGoodSliceJSON, "../Zerg-内部文档/01-设计/切片合同-模板.md:1", "scripts/gates/check-slice.py", 1)

	// ── 正控（必然判绿）：合法片 ⇒ rc=0 ──
	fg := w("g.json", good)
	rcG, outG, _ := csRun(t, "--check", fg, "--repo-root", dir)
	if rcG != 0 || !strings.Contains(outG, "rc = 0（全部合法）") {
		t.Fatalf("正控失败：合法片应 rc=0，得 rc=%d out=%q", rcG, outG)
	}

	// ── 负控 R1（必然判红）：一片绑两个 defect_id ⇒ rc=1 ──
	fb := w("b.json", strings.Replace(good, "DEF-20260918-S3-01", "A、B", 1))
	rcR, outR, _ := csRun(t, "--check", fb, "--repo-root", dir)
	if rcR != 1 || !strings.Contains(outR, "R1-MULTI-DEFECT") {
		t.Fatalf("负控-R1 失败：得 rc=%d out=%q", rcR, outR)
	}
	if rcG == rcR {
		t.Fatalf("成对断言失败：绿 rc=%d 与红 rc=%d 同码 ⇒ 无鉴别力", rcG, rcR)
	}

	// ── 负控 strict（不给结论）：未知字段键 ⇒ rc=2 ──
	fs := w("s.json", strings.Replace(good, "{", "{\n  \"zz_unknown_field\": 1,", 1))
	rcS, _, _ := csRun(t, "--check", fs, "--repo-root", dir)
	if rcS != 2 {
		t.Fatalf("负控-strict 失败：未知字段键应 rc=2，得 rc=%d", rcS)
	}
	if rcS == rcR || rcR == rcG {
		t.Fatalf("三态不成对：绿=%d 红=%d 错=%d", rcG, rcR, rcS)
	}

	// ── JSON null 面：显式 null vs 缺键 —— 两态各成子用例 ──
	nullF := w("null.json", `{"hard_limits": null}`)
	missF := w("miss.json", `{}`)
	_, outN, _ := csRun(t, "--check", nullF, "--repo-root", dir)
	_, outM, _ := csRun(t, "--check", missF, "--repo-root", dir)
	if !strings.Contains(outN, "R5-EMPTY") || !strings.Contains(outN, "空字符串/None") {
		t.Fatalf("null 面失败：显式 null 应走 R5-EMPTY/空字符串/None，得 %q", outN)
	}
	if !strings.Contains(outM, "R5-MISSING") {
		t.Fatalf("缺键面失败：缺键应走 R5-MISSING，得 %q", outM)
	}
	if outN == outM {
		t.Fatalf("null/缺键 两态未分开 ⇒ 无法区分（Go struct 反序列化的通病）")
	}

	// ── 负控：--disable-rule 未登记 ⇒ rc=2 ──
	rcD, _, _ := csRun(t, "--check", fg, "--disable-rule", "R7", "--repo-root", dir)
	if rcD != 2 {
		t.Fatalf("负控-disable 失败：未登记禁用应 rc=2，得 rc=%d", rcD)
	}

	// ── R4 约束守卫：无改写建议的命中必须拒判（panic InternalError）──
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		(&result{name: "guard"}).hit("R4", "R4-VAGUE", "自造的无 fix 命中", "")
	}()
	if !panicked {
		t.Fatalf("R4 守卫失败：无改写建议的命中必须拒判（InternalError），实未 panic")
	}
}

// TestCheckSliceSelfTestAlign —— 战线① 对齐回归牙：自证面逐字对拍 py 现件。
// 牙在哪：① joinOrNo 空值恰「无」（回退成 joinOr ⇒ 变「(无)」→ 红）；② (a)/(c)/(d) 明细行捕获（不捕获 ⇒ 红）；
// ③ (f) 把探针件拷进仿真仓后走到缺件闸（不拷 ⇒ 先撞「探针集文件不存在」→ 缺件路径不出现 → 红）。
func TestCheckSliceSelfTestAlign(t *testing.T) {
	if joinOrNo(nil, "·") != "无" {
		t.Fatalf("joinOrNo 空值应「无」：得 %q", joinOrNo(nil, "·"))
	}
	if joinOr(nil, "·") != "(无)" {
		t.Fatalf("joinOr 空值应「(无)」：得 %q", joinOr(nil, "·"))
	}
	if joinOrNo([]string{"a", "b"}, "·") != "a·b" {
		t.Fatalf("joinOrNo 非空拼接错：得 %q", joinOrNo([]string{"a", "b"}, "·"))
	}
	// 自证依赖 cwd = 真仓根（入口 `csRepoRoot = csDefaultRepoRoot()` 按 cwd 推）⇒ 夹具先把 cwd 挪到真仓根，
	// 跑完还原（真仓根是真目录、不会被清 ⇒ 无「cwd 停在已删目录」之虞）。
	root := csFindRepoRootForTest(t)
	oldwd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir 真仓根失败：%v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()
	rc, out, errs := csRun(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自证应 rc=0，得 rc=%d err=%q out尾=%q", rc, errs, csTailStr(out, 400))
	}
	for _, want := range []string{
		"结论：共 1 片 · 合法 1",
		"缺：无",
		"多（未标注）：无",
		"缺件路径：../Zerg-内部文档",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("自证输出缺 %q（自证面未对齐 py 现件）", want)
		}
	}
}

func csFindRepoRootForTest(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for d := wd; ; {
		if _, err := os.Stat(filepath.Join(d, "scripts", "gates", "slice-probes.json")); err == nil {
			return d
		}
		nd := filepath.Dir(d)
		if nd == d {
			break
		}
		d = nd
	}
	t.Skip("找不到真仓根（无 scripts/gates/slice-probes.json）⇒ 跳过自证面牙")
	return ""
}

func csTailStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// ★ 退役容忍（缺口 `GAP-20261007-16` · 类①「硬要求在盘 ⇒ 退役态不过问」）：尺 `scripts/gates/check-slice.py`
// 退役（不在盘）而其 **Go 半** `check_slice.go` 在盘 ⇒ `impactCalibrationOf` **不落**那一格缺件「取不到」
// （否则退役态永久「取不到」）；改由 Go 半进程内现跑取数。两缺（真消失）⇒ 保留原「取不到」（fail-closed）。
// 哑化 `impactCalibScriptRetired` ⇒ 本用例 FAIL（带本行号）。
func TestImpactCalibScriptRetiredTolerant(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("core/cmd/zerg/check_slice.go", "package main\n")
	// ① py 缺 · Go 半在盘 ⇒ 退役；越过缺件格 ⇒ 作「未机检」（✗ 不是「取不到」）
	if !impactCalibScriptRetired(dir) {
		t.Fatal("退役判定错：py 缺 + Go 半在 ⇒ 应判退役")
	}
	if c := impactCalibrationOf(dir, false); c.Status != "未机检" {
		t.Fatalf("退役态应「未机检」（越过缺件格），实 Status=%q reason=%q", c.Status, c.Reason)
	}
	// ② py 在盘 ⇒ 非退役（在盘照走真子进程 · 单一真源不变）
	mk("scripts/gates/check-slice.py", "x\n")
	if impactCalibScriptRetired(dir) {
		t.Fatal("py 在盘 ⇒ 非退役")
	}
	// ③ 两缺（真消失）⇒ 非退役 ⇒ fail-closed 保留：「取不到」+ 点名「尺的件不在盘上」
	if err := os.Remove(filepath.Join(dir, "scripts", "gates", "check-slice.py")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "core", "cmd", "zerg", "check_slice.go")); err != nil {
		t.Fatal(err)
	}
	if impactCalibScriptRetired(dir) {
		t.Fatal("两缺 ⇒ 非退役（真消失，不许当退役）")
	}
	if c := impactCalibrationOf(dir, false); c.Status != "取不到" || !strings.Contains(c.Reason, "尺的件不在盘上") {
		t.Fatalf("两缺应「取不到」+ 点名缺件（fail-closed 保留），实 Status=%q reason=%q", c.Status, c.Reason)
	}
}
