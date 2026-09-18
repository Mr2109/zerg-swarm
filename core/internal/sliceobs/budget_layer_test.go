// budget_layer_test.go — B 项 **B10** 用例（其一）：**定层闭集 + 闸门只读档案**。
//
// 覆盖（正反两侧都要）：
//
//	A. 定层闭集**逐字三名**（`档案`/`声明`/`默认`）+ 闭集外一律**认不出**（不猜、不归一化）
//	B. **首次定义层**取值顺序：档案(事实) ⇒ 声明(意图) ⇒ 默认；三处都没有 ⇒ **未标定**
//	C. 真档案（只读）读数必须诚实：**键在 ⇔ 有值**（键在却报未标定 = 读漏；键缺却给数字 = 编造）
//	D. **非法值不猜**：负数 / 非数串 / NaN ⇒ 未标定（不截断、不取默认值）
//	E. **只读**：档案不存在时**不创建**；读前后**内容 / 大小 / mtime 一字不变**（写侧无路径）
package sliceobs

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── A. 定层闭集逐字三名 + 闭集外不认 ─────────────────────────────────────

func TestB10_DefinitionLayers_ClosedSetVerbatim(t *testing.T) {
	// 期望值**写成字面**（不引用常量）：常量的值本身就是要被钉住的东西 ——
	// 拿常量断言常量等于自证。设计稿 v2.1 §4.6-10 第 241 行 / §6.3 第 380 行逐字三名。
	want := []string{"档案", "声明", "默认"}
	got := DefinitionLayers()
	if len(got) != len(want) {
		t.Fatalf("定层闭集应 %d 个（§4.6-10 / §6.3 逐字「档案／声明／默认」），实际 %d 个：%v", len(want), len(got), got)
	}
	for i := range want {
		if string(got[i]) != want[i] {
			t.Fatalf("第 %d 个定层与稿面不符：期望 %q，实际 %q", i+1, want[i], got[i])
		}
		if !got[i].Valid() {
			t.Fatalf("闭集内的 %q 却被判非法", got[i])
		}
	}

	// 闭集外一律**认不出**（不得归一化、不得猜）
	junk := []string{"", "archive", "Archive", "ARCHIVE", " 档案", "档案 ", "声 明", "声明（意图）", "档案(事实)", "默认值", "profile"}
	for _, s := range junk {
		if l, ok := ParseDefinitionLayer(s); ok {
			t.Fatalf("闭集外的 %q 必须认不出（不猜），却解析成了 %q", s, l)
		}
		if DefinitionLayer(s).Valid() {
			t.Fatalf("闭集外的 %q 被判合法（Valid 必须与 Parse 同口径）", s)
		}
	}
	t.Logf("闭集 = %v；闭集外 %d 个样本全部认不出", want, len(junk))
}

// ── B. 首次定义层取值顺序 ────────────────────────────────────────────────

func TestB10_ResolveBudgetValue_FirstDefinitionLayer(t *testing.T) {
	arch := map[string]any{"wall_s_max": 43.6}
	decl := 7.0
	def := 9.0
	neg := -1.0
	nan := math.NaN()

	cases := []struct {
		name     string
		key      string
		declared *float64
		def      *float64
		wantText string
		wantLyr  string
	}{
		{"档案在 ⇒ 档案(事实)优先，声明与默认都不看", "wall_s_max", &decl, &def, "43.6", "档案"},
		{"档案缺 + 声明在 ⇒ 声明(意图)", "restate_every_n_steps", &decl, &def, "7", "声明"},
		{"档案缺 + 声明缺 + 默认在 ⇒ 默认", "steps_max", nil, &def, "9", "默认"},
		{"三处都没有 ⇒ 未标定", "depth", nil, nil, UncalibratedText, UncalibratedText},
		{"声明非法(负数) 且无默认 ⇒ 未标定（不猜）", "steps_max", &neg, nil, UncalibratedText, UncalibratedText},
		{"声明非法(NaN) 且无默认 ⇒ 未标定", "steps_max", &nan, nil, UncalibratedText, UncalibratedText},
		{"声明非法(负数) + 默认合法 ⇒ 退到默认（声明那档不算数）", "steps_max", &neg, &def, "9", "默认"},
	}
	for _, tc := range cases {
		got := ResolveBudgetValue(tc.key, arch, tc.declared, tc.def)
		if got.Text() != tc.wantText {
			t.Fatalf("%s：值应为 %q，实际 %q", tc.name, tc.wantText, got.Text())
		}
		if got.LayerText() != tc.wantLyr {
			t.Fatalf("%s：定层应为 %q，实际 %q", tc.name, tc.wantLyr, got.LayerText())
		}
		if got.Key != tc.key {
			t.Fatalf("%s：键名应原样带回 %q，实际 %q", tc.name, tc.key, got.Key)
		}
	}
	t.Logf("取值顺序 档案⇒声明⇒默认 逐条钉住；三处都没有/非法 ⇒ 「%s」", UncalibratedText)
}

// ── C. 真档案读数诚实（键在 ⇔ 有值）──────────────────────────────────────

func TestB10_BudgetCalib_RealArchiveReadingsAreHonest(t *testing.T) {
	realHome := os.Getenv("HOME")

	// ① 默认路径的形状（不猜路径；必须与 B 项⑥ 同一个解析点 ⇒ 指向同一个档案）
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	want := filepath.Join(fakeHome, ".zerg", "egg-profiles", "task-budget-calib.yaml")
	if got := DefaultSliceCalibPath(); got != want {
		t.Fatalf("默认档案路径应为 %q，实际 %q", want, got)
	}
	if got := LoadBudgetCalib().Path; got != want {
		t.Fatalf("LoadBudgetCalib 应读同一个档案 %q，实际 %q", want, got)
	}
	if c := LoadBudgetCalib(); c.ReadErr != calibReadErrNotExist {
		t.Fatalf("空 home ⇒ 应报「%s」，实际 %q", calibReadErrNotExist, c.ReadErr)
	}

	// ② 真档案（**只读**：不写、不迁移、不造目录）现读；本机没有 ⇒ 跳过（不是失败）
	t.Setenv("HOME", realHome)
	realPath := DefaultSliceCalibPath()
	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Skipf("本机没有标定档案（%s）⇒ 跳过真档案读数（不是失败：%v）", realPath, err)
	}
	c := LoadBudgetCalibFrom(realPath)
	if c.ReadErr != "" {
		t.Fatalf("真档案读失败了：%q", c.ReadErr)
	}
	keys := ArchiveBudgetKeys()
	if len(c.Items) != len(keys) {
		t.Fatalf("预算项应 %d 个（现档四个数值键），实际 %d 个", len(keys), len(c.Items))
	}
	for _, k := range keys {
		keyInFile := strings.Contains(string(data), k)
		it, ok := c.Item(k)
		if !ok {
			t.Fatalf("项集里少了 %q", k)
		}
		if keyInFile && !it.Calibrated() {
			t.Fatalf("档案里有键 %q 却报「%s」（读漏）", k, it.LayerText())
		}
		if !keyInFile && it.Calibrated() {
			t.Fatalf("档案里没有键 %q 却给出值 %q（编造）", k, it.Text())
		}
	}
	// 现档四个键都在 ⇒ 整档已标定；缺一个 ⇒ 恰好那一项「未标定」
	allIn := true
	for _, k := range keys {
		if !strings.Contains(string(data), k) {
			allIn = false
		}
	}
	if got := c.Calibrated(); got != allIn {
		t.Fatalf("整档已标定应为 %v（四个键是否都在：%v），实际 %v", allIn, allIn, got)
	}
	t.Logf("真档案 %s：%s=%s(%s) · %s=%s(%s) · %s=%s(%s) · %s=%s(%s)",
		realPath,
		keys[0], c.ItemText(keys[0]), c.ItemLayerText(keys[0]),
		keys[1], c.ItemText(keys[1]), c.ItemLayerText(keys[1]),
		keys[2], c.ItemText(keys[2]), c.ItemLayerText(keys[2]),
		keys[3], c.ItemText(keys[3]), c.ItemLayerText(keys[3]))
}

// ── D. 非法值不猜 ────────────────────────────────────────────────────────

func TestB10_BudgetCalib_JunkValuesAreUncalibrated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calib.yaml")
	// 现档同形的夹具：两个合法键 + 一串坏值（每种坏值走 floatKey 的不同分支）
	fixture := strings.Join([]string{
		"model: fixture-model",
		"steps_max: 3",                      // int 合法
		"tool_calls_max: \"2\"",             // 数字串合法（现格式允许）
		"wall_s_max: -1.5",                  // 负数（float64 分支）⇒ 不认
		"assistant_tokens_est_max: \"abc\"", // 非数串 ⇒ 不认
		"extra_unknown_key: 5",              // 认不出的键 ⇒ 根本不进项集（不发明项）
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatalf("造夹具失败：%v", err)
	}

	c := LoadBudgetCalibFrom(path)
	if c.ReadErr != "" {
		t.Fatalf("夹具应能读到，实际 %q", c.ReadErr)
	}
	if got := c.ItemText(BudgetKeyStepsMax); got != "3" {
		t.Fatalf("steps_max 应如实回显 3，实际 %q", got)
	}
	if got := c.ItemLayerText(BudgetKeyStepsMax); got != "档案" {
		t.Fatalf("steps_max 的定层应为「档案」，实际 %q", got)
	}
	if got := c.ItemText(BudgetKeyToolCallsMax); got != "2" {
		t.Fatalf("tool_calls_max 应如实回显 2（数字串按数字收），实际 %q", got)
	}
	for _, k := range []string{BudgetKeyWallSMax, BudgetKeyAssistantTokensMax} {
		it, ok := c.Item(k)
		if !ok {
			t.Fatalf("项集里少了 %q", k)
		}
		if it.Calibrated() || it.Text() != UncalibratedText || it.LayerText() != UncalibratedText {
			t.Fatalf("%q 的值不合法 ⇒ 必须「%s」（不截断/不四舍五入/不取默认值），实际值=%q 层=%q",
				k, UncalibratedText, it.Text(), it.LayerText())
		}
	}
	if got := c.ItemText("extra_unknown_key"); got != UncalibratedText {
		t.Fatalf("认不出的键必须「%s」（不发明项），实际 %q", UncalibratedText, got)
	}
	if c.Calibrated() {
		t.Fatalf("有一项未标定 ⇒ 整档不得报「已标定」")
	}
	t.Logf("坏值（-1.5 / abc）⇒ 全部「%s」；未知键不进项集", UncalibratedText)
}

// ── E. 只读：不创建、不改一字 ────────────────────────────────────────────

func TestB10_BudgetCalib_ReadOnly_NoWritePath(t *testing.T) {
	dir := t.TempDir()

	// ① 档案不存在 ⇒ 读不到就是读不到：**不创建文件**、不建目录
	missing := filepath.Join(dir, "nope", "task-budget-calib.yaml")
	c := LoadBudgetCalibFrom(missing)
	if c.ReadErr != calibReadErrNotExist {
		t.Fatalf("不存在的档案应报「%s」，实际 %q", calibReadErrNotExist, c.ReadErr)
	}
	if c.Calibrated() {
		t.Fatalf("读不到档案 ⇒ 整档不得报「已标定」")
	}
	if _, err := os.Stat(filepath.Join(dir, "nope")); !os.IsNotExist(err) {
		t.Fatalf("只读入口不得创建目录（写侧无路径），实际 %v", err)
	}

	// ② 档案在 ⇒ 读两次数值一致，且文件**内容 / 大小 / mtime 一字不变**
	path := filepath.Join(dir, "task-budget-calib.yaml")
	body := "model: fixture\nsteps_max: 3\ntool_calls_max: 2\nwall_s_max: 43.6\nassistant_tokens_est_max: 118\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("造夹具失败：%v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat 失败：%v", err)
	}
	sumBefore := sha256.Sum256([]byte(body))

	first := LoadBudgetCalibFrom(path)
	second := LoadBudgetCalibFrom(path)
	if first.ReadErr != "" || second.ReadErr != "" {
		t.Fatalf("夹具应能读到，实际 %q / %q", first.ReadErr, second.ReadErr)
	}
	if len(first.Items) != len(second.Items) {
		t.Fatalf("两次读数项数不同：%d vs %d", len(first.Items), len(second.Items))
	}
	for i := range first.Items {
		if first.Items[i].Text() != second.Items[i].Text() || first.Items[i].LayerText() != second.Items[i].LayerText() {
			t.Fatalf("同档案两次读数不一致：%+v vs %+v", first.Items[i], second.Items[i])
		}
	}
	if !first.Calibrated() {
		t.Fatalf("四个键全在 ⇒ 应报「已标定」，实际未标定")
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat 失败：%v", err)
	}
	now, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("回读失败：%v", err)
	}
	sumAfter := sha256.Sum256(now)
	if sumBefore != sumAfter {
		t.Fatalf("读档案把内容改了：前 %s / 后 %s", hex.EncodeToString(sumBefore[:]), hex.EncodeToString(sumAfter[:]))
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("读档案把文件元数据改了：size %d→%d，mtime %v→%v",
			before.Size(), after.Size(), before.ModTime(), after.ModTime())
	}
	t.Logf("只读自证：sha256 前 %s = 后 %s；size=%d；mtime=%v 未变",
		hex.EncodeToString(sumBefore[:8]), hex.EncodeToString(sumAfter[:8]), after.Size(), after.ModTime().Format("150405"))
}
