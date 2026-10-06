package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// essT_run 跑入口并接住三面（rc / stdout / stderr）。stdin 走注入点 essStdin。
func essT_run(t *testing.T, argv []string, stdin string) (int, string, string) {
	t.Helper()
	out := &strings.Builder{}
	errb := &strings.Builder{}
	oldStdin := essStdin
	essStdin = strings.NewReader(stdin)
	defer func() { essStdin = oldStdin }()
	rc := evalStabilityStatsGateMain(out, errb, argv)
	return rc, out.String(), errb.String()
}

const essT_preflight = "预检自检：71 项全绿（本工具自己的正确性先过检）\n"

func essT_fixtures(t *testing.T) (string, string, string, string) {
	t.Helper()
	d := t.TempDir()
	obs1 := strings.Join(essT_rep(`{"group":"g","vector_hash":"aa"}`, 6), "\n") + "\n"
	obs2 := ""
	for i := 0; i < 20; i++ {
		if i > 0 {
			obs2 += "\n"
		}
		if i == 0 {
			obs2 += `{"group":"g","diverged":false}`
		} else {
			obs2 += `{"group":"g","diverged":true}`
		}
	}
	obs2 += "\n"
	obs3 := ""
	obs3 += strings.Join(essT_rep(`{"group":"a","vector_hash":"solo"}`, 5), "\n")
	for i := 0; i < 25; i++ {
		obs3 += "\n" + `{"group":"a","vector_hash":"h` + essT_itoa(i) + `"}`
	}
	obs3 += "\n" + `{"group":"b","vector_hash":"z","reference":null}`
	obs3 += "\n" + `{"group":"b","diverged":true}`
	obs3 += "\n"
	bad := "not-json\n"
	p1 := filepath.Join(d, "obs1.jsonl")
	p2 := filepath.Join(d, "obs2.jsonl")
	p3 := filepath.Join(d, "obs3.jsonl")
	pb := filepath.Join(d, "bad.jsonl")
	if err := os.WriteFile(p1, []byte(obs1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte(obs2), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p3, []byte(obs3), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pb, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	return p1, p2, p3, pb
}

func essT_rep(s string, n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, s)
	}
	return out
}

func essT_itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

func essT_assert(t *testing.T, name string, rc int, out, errb string, wantRC int, wantOut, wantErr string) {
	t.Helper()
	if rc != wantRC {
		t.Errorf("%s: rc 期望 %d 实得 %d", name, wantRC, rc)
	}
	if out != wantOut {
		t.Errorf("%s: stdout 不等\n got=%q\nwant=%q", name, out, wantOut)
	}
	if errb != wantErr {
		t.Errorf("%s: stderr 不等\n got=%q\nwant=%q", name, errb, wantErr)
	}
}

func TestEvalStabilityStatsGateMain_Grid(t *testing.T) {
	p1, p2, p3, pb := essT_fixtures(t)

	t.Run("自检", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{"--self-test"}, "")
		essT_assert(t, "自检", rc, out, errb, 0, essT_G_selftest, "")
	})
	t.Run("0分歧_文本", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{p1}, "")
		essT_assert(t, "0分歧_文本", rc, out, errb, 0, essT_G_obs1, essT_preflight)
	})
	t.Run("0分歧_json", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{p1, "--json"}, "")
		essT_assert(t, "0分歧_json", rc, out, errb, 0, essT_G_obs1_json, essT_preflight)
	})
	t.Run("分歧_文本", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{p2}, "")
		essT_assert(t, "分歧_文本", rc, out, errb, 0, essT_G_obs2, essT_preflight)
	})
	t.Run("多组_文本", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{p3}, "")
		essT_assert(t, "多组_文本", rc, out, errb, 0, essT_G_obs3, essT_preflight)
	})
	t.Run("显式参考_文本", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{p3, "--reference", "a=solo"}, "")
		essT_assert(t, "显式参考_文本", rc, out, errb, 0, essT_G_obs3, essT_preflight)
	})
	t.Run("无输入_帮助_rc2", func(t *testing.T) {
		rc, out, errb := essT_run(t, nil, "")
		essT_assert(t, "无输入_帮助_rc2", rc, out, errb, 2, essHelpText, "")
	})
	t.Run("短帮助", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{"-h"}, "")
		essT_assert(t, "短帮助", rc, out, errb, 0, essHelpText, "")
	})
	t.Run("坏JSON_rc2", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{pb}, "")
		want := essT_preflight + "输入错误 ⇒ 硬失败（rc=2）：" + pb +
			":1 不是合法 JSON：Expecting value: line 1 column 1 (char 0)\n"
		essT_assert(t, "坏JSON_rc2", rc, out, errb, 2, "", want)
	})
	t.Run("未知旗标_rc2", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{"--nope"}, "")
		want := essUsage + "stability-stats.py: error: unrecognized arguments: --nope\n"
		essT_assert(t, "未知旗标_rc2", rc, out, errb, 2, "", want)
	})
	t.Run("坏浮点_rc2", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{p1, "--alpha=abc"}, "")
		want := essUsage + "stability-stats.py: error: argument --alpha: invalid float value: 'abc'\n"
		essT_assert(t, "坏浮点_rc2", rc, out, errb, 2, "", want)
	})
	t.Run("缺件_rc2", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope.jsonl")
		rc, out, errb := essT_run(t, []string{missing}, "")
		want := essT_preflight + "输入错误 ⇒ 硬失败（rc=2）：[Errno 2] No such file or directory: '" + missing + "'\n"
		essT_assert(t, "缺件_rc2", rc, out, errb, 2, "", want)
	})
	t.Run("无对照表", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{p1, "--no-table"}, "")
		essT_assert(t, "无对照表", rc, out, errb, 0, essT_G_notable, essT_preflight)
	})
	t.Run("无预检", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{p1, "--no-preflight"}, "")
		essT_assert(t, "无预检", rc, out, errb, 0, essT_G_obs1, "")
	})
	t.Run("stdin_横线", func(t *testing.T) {
		si := `{"group":"g","vector_hash":"aa"}` + "\n" + `{"group":"g","vector_hash":"aa"}` + "\n" + `{"group":"g","vector_hash":"bb"}` + "\n"
		rc, out, errb := essT_run(t, []string{"-"}, si)
		essT_assert(t, "stdin_横线", rc, out, errb, 0, essT_G_stdin, essT_preflight)
	})
	t.Run("唯一前缀缩写", func(t *testing.T) {
		rc, out, errb := essT_run(t, []string{p1, "--sel"}, "")
		essT_assert(t, "唯一前缀缩写", rc, out, errb, 0, essT_G_selftest, "")
	})
}

// TestEvalStabilityStatsGateMain_JSONFloatFace 钉「JSON 里 k>0 的 CP 界浮点末位」这一已声明的运行时差：
// 结构与非浮点字段必须逐字同 py，浮点字段容差 1e-12（真因：Go math.Lgamma ≠ 系统 libm lgamma）。
func TestEvalStabilityStatsGateMain_JSONFloatFace(t *testing.T) {
	_, p2, p3, _ := essT_fixtures(t)
	for _, tc := range []struct {
		name string
		path string
		gold string
	}{{"obs2", p2, essT_G_obs2_json_py}, {"obs3", p3, essT_G_obs3_json_py}} {
		t.Run(tc.name, func(t *testing.T) {
			rc, out, _ := essT_run(t, []string{tc.path, "--json"}, "")
			if rc != 0 {
				t.Fatalf("rc 期望 0 实得 %d", rc)
			}
			var pg, gg map[string]interface{}
			if err := json.Unmarshal([]byte(tc.gold), &pg); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(out), &gg); err != nil {
				t.Fatalf("输出非法 JSON: %v", err)
			}
			essT_cmpJSON(t, "", pg, gg)
		})
	}
}

func essT_cmpJSON(t *testing.T, path string, a, b interface{}) {
	t.Helper()
	switch av := a.(type) {
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok {
			t.Errorf("%s: 类型不同", path)
			return
		}
		if len(av) != len(bv) {
			t.Errorf("%s: 键数 %d != %d", path, len(av), len(bv))
		}
		for k, v := range av {
			w, ok := bv[k]
			if !ok {
				t.Errorf("%s.%s: 缺键", path, k)
				continue
			}
			essT_cmpJSON(t, path+"."+k, v, w)
		}
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			t.Errorf("%s: 数组不同", path)
			return
		}
		for i := range av {
			essT_cmpJSON(t, path+"[]", av[i], bv[i])
		}
	case float64:
		bv, ok := b.(float64)
		if !ok || math.Abs(av-bv) > 1e-12 {
			t.Errorf("%s: 浮点 %v != %v", path, av, b)
		}
	case string:
		if bv, ok := b.(string); !ok || av != bv {
			t.Errorf("%s: 串 %q != %v", path, av, b)
		}
	default:
		if av != b {
			t.Errorf("%s: %v != %v", path, av, b)
		}
	}
}

// TestEvalStabilityStatsGateMain_负控 证明「预检门 / 自检门」两条硬失败路径真有牙（注入必红自检桩）。
func TestEvalStabilityStatsGateMain_负控(t *testing.T) {
	p1, _, _, _ := essT_fixtures(t)
	old := essRunSelfTest
	defer func() { essRunSelfTest = old }()
	essRunSelfTest = func() (int, []string) { return 0, []string{"注入的必红自检项"} }

	t.Run("负控_预检门有牙", func(t *testing.T) {
		rc, _, errb := essT_run(t, []string{p1}, "")
		if rc != 2 {
			t.Fatalf("注入必红自检后 rc 期望 2 实得 %d", rc)
		}
		if !strings.Contains(errb, "预检自测失败（1 项）⇒ 拒跑真目标（rc=2）") ||
			!strings.Contains(errb, "注入的必红自检项") {
			t.Fatalf("预检失败正文不符：%q", errb)
		}
	})
	t.Run("负控_自检门有牙", func(t *testing.T) {
		rc, out, _ := essT_run(t, []string{"--self-test"}, "")
		if rc != 2 {
			t.Fatalf("--self-test 注入必红后 rc 期望 2 实得 %d", rc)
		}
		if !strings.Contains(out, "自测红 ⇒ 拒跑真目标（rc=2）") {
			t.Fatalf("自检红正文不符：%q", out)
		}
	})
	t.Run("负控_摘掉桩恢复正常", func(t *testing.T) {
		essRunSelfTest = old
		rc, out, _ := essT_run(t, []string{"--self-test"}, "")
		if rc != 0 || !strings.Contains(out, "71 项通过，0 项失败") {
			t.Fatalf("恢复后自检应全绿：rc=%d out=%q", rc, out)
		}
	})
}

// TestEvalStabilityStatsGateMain_输入良构 钉「空/非法输入一律硬失败 rc=2，不静默降级」。
func TestEvalStabilityStatsGateMain_输入良构(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad2.jsonl")
	if err := os.WriteFile(p, []byte(`{"group":"g"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p0 := filepath.Join(d, "empty.jsonl")
	if err := os.WriteFile(p0, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := essT_run(t, []string{p}, "")
	if rc != 2 || out != "" || !strings.Contains(errb, "既没有可识别的向量指纹字段") {
		t.Fatalf("无字段行应 rc=2：rc=%d out=%q err=%q", rc, out, errb)
	}
	rc, _, errb = essT_run(t, []string{p0}, "")
	if rc != 2 || !strings.Contains(errb, "里没有可用观测") {
		t.Fatalf("空件应 rc=2：rc=%d err=%q", rc, errb)
	}
}

// 以下为逐字节金标（由对拍台从 py 现读嵌入；见回执）。
const essT_G_selftest = "自测：71 项通过，0 项失败\n自测全绿（已知答案：0/6=39.30%、0/20=13.91%、1/20=21.61%、5/59=17.00%、0/59=4.95%、0/300=0.99%、21/21 下界=86.71%、25/25 下界=88.71%）\n"
const essT_G_obs1 = "==============================================================================\n稳定性统计口径 · 分歧率 + Clopper-Pearson 单侧 95% 上界（alpha=0.05）\n==============================================================================\n\n组：g\n  N=6 · 分歧 k=0 · 点估计 0.00% · CP 上界 39.30% · CP 下界 0.00% · 唯一值 1 · 参考向量 aa · rule of three 50.00%\n  取值分布：aa×6\n  N=6、0/6 次分歧 ⇒ 分歧率 95% 上界 = 39.30%（精确 CP；rule of three 3/6 = 50.00%）\n  结论：只能声称「分歧率 < 50%」（由精确上界 39.30% 定档）；**不能**声称「< 15%」（需 N≥19，本组还差 13 个全一致样本）。**不能**声称「分歧率 = 0」。\n\n------------------------------------------------------------------------------\n样本量对照表（0 分歧时能声称什么 / 要多少样本才能捕到分歧）\n------------------------------------------------------------------------------\n  要声称「分歧率 <X%」(0 分歧)： <15% 需 N≥19 · <10% 需 N≥29 · <5% 需 N≥59 · <2% 需 N≥149 · <1% 需 N≥299 · <0.5% 需 N≥598\n  rule of three 3/n 近似：      <5% 需 N≥60 · <1% 需 N≥300\n  真率 p 要 ≥95% 概率捕到一次： p=10% 需 N≥29 · p=5% 需 N≥59 · p=1% 需 N≥299\n  口径铁律：N=2 的「两次一样」上界 ≈ 78%——**没有任何统计意义**。\n"
const essT_G_obs1_json = "{\n  \"alpha\": 0.05,\n  \"groups\": [\n    {\n      \"group\": \"g\",\n      \"n\": 6,\n      \"k\": 0,\n      \"unique\": 1,\n      \"reference\": \"aa\",\n      \"rate\": 0.0,\n      \"cp_upper\": 0.39303776899708276,\n      \"cp_lower\": 0.0,\n      \"rule_of_three\": 0.5,\n      \"alpha\": 0.05,\n      \"top_values\": [\n        {\n          \"hash\": \"aa\",\n          \"count\": 6\n        }\n      ],\n      \"mode_count\": 6,\n      \"verdict\": \"N=6、0/6 次分歧 ⇒ 分歧率 95% 上界 = 39.30%（精确 CP；rule of three 3/6 = 50.00%）\\n结论：只能声称「分歧率 < 50%」（由精确上界 39.30% 定档）；**不能**声称「< 15%」（需 N≥19，本组还差 13 个全一致样本）。**不能**声称「分歧率 = 0」。\"\n    }\n  ]\n}\n"
const essT_G_obs2 = "==============================================================================\n稳定性统计口径 · 分歧率 + Clopper-Pearson 单侧 95% 上界（alpha=0.05）\n==============================================================================\n\n组：g\n  N=20 · 分歧 k=19 · 点估计 95.00% · CP 上界 99.74% · CP 下界 78.39%\n  N=20、19 次分歧 ⇒ 点估计 95.00%；CP 单侧 95%（alpha=0.05） 上界 = 99.74%、下界 = 78.39%\n  结论：可声称「分歧率 < 99.74%」；点估计 95.00% **不是**可宣传的口径（N=20 太薄）\n\n------------------------------------------------------------------------------\n样本量对照表（0 分歧时能声称什么 / 要多少样本才能捕到分歧）\n------------------------------------------------------------------------------\n  要声称「分歧率 <X%」(0 分歧)： <15% 需 N≥19 · <10% 需 N≥29 · <5% 需 N≥59 · <2% 需 N≥149 · <1% 需 N≥299 · <0.5% 需 N≥598\n  rule of three 3/n 近似：      <5% 需 N≥60 · <1% 需 N≥300\n  真率 p 要 ≥95% 概率捕到一次： p=10% 需 N≥29 · p=5% 需 N≥59 · p=1% 需 N≥299\n  口径铁律：N=2 的「两次一样」上界 ≈ 78%——**没有任何统计意义**。\n"
const essT_G_obs3 = "==============================================================================\n稳定性统计口径 · 分歧率 + Clopper-Pearson 单侧 95% 上界（alpha=0.05）\n==============================================================================\n\n组：a\n  N=30 · 分歧 k=25 · 点估计 83.33% · CP 上界 93.19% · CP 下界 68.10% · 唯一值 26 · 参考向量 solo\n  取值分布：solo×5 | h0×1 | h1×1\n  N=30、25 次分歧 ⇒ 点估计 83.33%；CP 单侧 95%（alpha=0.05） 上界 = 93.19%、下界 = 68.10%\n  结论：可声称「分歧率 < 93.19%」；点估计 83.33% **不是**可宣传的口径（N=30 太薄）\n\n组：b\n  N=2 · 分歧 k=1 · 点估计 50.00% · CP 上界 97.47% · CP 下界 2.53% · 唯一值 1 · 参考向量 z\n  取值分布：z×1\n  N=2、1 次分歧 ⇒ 点估计 50.00%；CP 单侧 95%（alpha=0.05） 上界 = 97.47%、下界 = 2.53%\n  结论：可声称「分歧率 < 97.47%」；点估计 50.00% **不是**可宣传的口径（N=2 太薄）\n\n------------------------------------------------------------------------------\n样本量对照表（0 分歧时能声称什么 / 要多少样本才能捕到分歧）\n------------------------------------------------------------------------------\n  要声称「分歧率 <X%」(0 分歧)： <15% 需 N≥19 · <10% 需 N≥29 · <5% 需 N≥59 · <2% 需 N≥149 · <1% 需 N≥299 · <0.5% 需 N≥598\n  rule of three 3/n 近似：      <5% 需 N≥60 · <1% 需 N≥300\n  真率 p 要 ≥95% 概率捕到一次： p=10% 需 N≥29 · p=5% 需 N≥59 · p=1% 需 N≥299\n  口径铁律：N=2 的「两次一样」上界 ≈ 78%——**没有任何统计意义**。\n"
const essT_G_notable = "==============================================================================\n稳定性统计口径 · 分歧率 + Clopper-Pearson 单侧 95% 上界（alpha=0.05）\n==============================================================================\n\n组：g\n  N=6 · 分歧 k=0 · 点估计 0.00% · CP 上界 39.30% · CP 下界 0.00% · 唯一值 1 · 参考向量 aa · rule of three 50.00%\n  取值分布：aa×6\n  N=6、0/6 次分歧 ⇒ 分歧率 95% 上界 = 39.30%（精确 CP；rule of three 3/6 = 50.00%）\n  结论：只能声称「分歧率 < 50%」（由精确上界 39.30% 定档）；**不能**声称「< 15%」（需 N≥19，本组还差 13 个全一致样本）。**不能**声称「分歧率 = 0」。\n"
const essT_G_stdin = "==============================================================================\n稳定性统计口径 · 分歧率 + Clopper-Pearson 单侧 95% 上界（alpha=0.05）\n==============================================================================\n\n组：g\n  N=3 · 分歧 k=1 · 点估计 33.33% · CP 上界 86.46% · CP 下界 1.70% · 唯一值 2 · 参考向量 aa\n  取值分布：aa×2 | bb×1\n  N=3、1 次分歧 ⇒ 点估计 33.33%；CP 单侧 95%（alpha=0.05） 上界 = 86.46%、下界 = 1.70%\n  结论：可声称「分歧率 < 86.46%」；点估计 33.33% **不是**可宣传的口径（N=3 太薄）\n\n------------------------------------------------------------------------------\n样本量对照表（0 分歧时能声称什么 / 要多少样本才能捕到分歧）\n------------------------------------------------------------------------------\n  要声称「分歧率 <X%」(0 分歧)： <15% 需 N≥19 · <10% 需 N≥29 · <5% 需 N≥59 · <2% 需 N≥149 · <1% 需 N≥299 · <0.5% 需 N≥598\n  rule of three 3/n 近似：      <5% 需 N≥60 · <1% 需 N≥300\n  真率 p 要 ≥95% 概率捕到一次： p=10% 需 N≥29 · p=5% 需 N≥59 · p=1% 需 N≥299\n  口径铁律：N=2 的「两次一样」上界 ≈ 78%——**没有任何统计意义**。\n"
const essT_G_obs2_json_py = "{\n  \"alpha\": 0.05,\n  \"groups\": [\n    {\n      \"group\": \"g\",\n      \"n\": 20,\n      \"k\": 19,\n      \"unique\": null,\n      \"reference\": null,\n      \"rate\": 0.95,\n      \"cp_upper\": 0.9974386212234698,\n      \"cp_lower\": 0.7838938357931529,\n      \"rule_of_three\": null,\n      \"alpha\": 0.05,\n      \"verdict\": \"N=20、19 次分歧 ⇒ 点估计 95.00%；CP 单侧 95%（alpha=0.05） 上界 = 99.74%、下界 = 78.39%\\n结论：可声称「分歧率 < 99.74%」；点估计 95.00% **不是**可宣传的口径（N=20 太薄）\"\n    }\n  ]\n}\n"
const essT_G_obs3_json_py = "{\n  \"alpha\": 0.05,\n  \"groups\": [\n    {\n      \"group\": \"a\",\n      \"n\": 30,\n      \"k\": 25,\n      \"unique\": 26,\n      \"reference\": \"solo\",\n      \"rate\": 0.8333333333333334,\n      \"cp_upper\": 0.9319444264196155,\n      \"cp_lower\": 0.6810288180526811,\n      \"rule_of_three\": null,\n      \"alpha\": 0.05,\n      \"top_values\": [\n        {\n          \"hash\": \"solo\",\n          \"count\": 5\n        },\n        {\n          \"hash\": \"h0\",\n          \"count\": 1\n        },\n        {\n          \"hash\": \"h1\",\n          \"count\": 1\n        }\n      ],\n      \"mode_count\": 5,\n      \"verdict\": \"N=30、25 次分歧 ⇒ 点估计 83.33%；CP 单侧 95%（alpha=0.05） 上界 = 93.19%、下界 = 68.10%\\n结论：可声称「分歧率 < 93.19%」；点估计 83.33% **不是**可宣传的口径（N=30 太薄）\"\n    },\n    {\n      \"group\": \"b\",\n      \"n\": 2,\n      \"k\": 1,\n      \"unique\": 1,\n      \"reference\": \"z\",\n      \"rate\": 0.5,\n      \"cp_upper\": 0.9746794344808964,\n      \"cp_lower\": 0.025320565519103604,\n      \"rule_of_three\": null,\n      \"alpha\": 0.05,\n      \"top_values\": [\n        {\n          \"hash\": \"z\",\n          \"count\": 1\n        }\n      ],\n      \"mode_count\": 1,\n      \"verdict\": \"N=2、1 次分歧 ⇒ 点估计 50.00%；CP 单侧 95%（alpha=0.05） 上界 = 97.47%、下界 = 2.53%\\n结论：可声称「分歧率 < 97.47%」；点估计 50.00% **不是**可宣传的口径（N=2 太薄）\"\n    }\n  ]\n}\n"
