// determinism_audit_test.go —— T4.5（非确定性源清单化审计）的实证：
//
//	⑥ TestDeterminismAuditCatchesPlantedTimeNow  植入 time.Now ⇒ 红；真包自审 ⇒ 全绿；十类无空项
//	⑦ TestDeterminismAuditDetectionRules         各类禁模式/导入/map-range/漂移/过期豁免/未冻结 逐条钉死
//
// 纪律：每条都补**对侧**断言（"凡 range 都报红"与"什么都不报红"都是假绿，两个方向都要挡住）。
package replay

import (
	"errors"
	"strings"
	"testing"
)

// plantedTimeNow —— 人为植入的违规源（用例⑥的靶子）。
const plantedTimeNow = `package fake

import "time"

type clock struct{}

func (c *clock) now() time.Time {
	return time.Now()
}

func (c *clock) other() string {
	return "ok"
}
`

// ── ⑥ 审计能抓到人为植入的 time.Now（且真包自审全绿）───────────────────────

func TestDeterminismAuditCatchesPlantedTimeNow(t *testing.T) {
	aud, err := DefaultAuditor()
	if err != nil {
		t.Fatalf("DefaultAuditor（声明表里模式非法会在这里炸）：%v", err)
	}

	rep, err := aud.AuditSources(map[string]string{"planted.go": plantedTimeNow}, AuditOptions{})
	if err != nil {
		t.Fatalf("AuditSources：%v", err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("植入 1 处取时应恰好红 1 处，实得 %d：%+v", len(rep.Findings), rep.Findings)
	}
	f := rep.Findings[0]
	if f.Class != "time" || f.Line != 8 || f.File != "planted.go" {
		t.Errorf("红项应指到 planted.go:8 的 time 类，实得 %s:%d [%s]", f.File, f.Line, f.Class)
	}
	if !strings.Contains(f.Text, "time.Now()") {
		t.Errorf("红项应带命中行原文，实得 %q", f.Text)
	}
	if f.Allowed != nil {
		t.Errorf("植入点不应命中任何豁免：%+v", f.Allowed)
	}
	if err := rep.Verdict(); err == nil || !errors.Is(err, ErrDeterminismAudit) {
		t.Errorf("有红项时 Verdict 必须报 ErrDeterminismAudit，实得 %v", err)
	}
	// 注意：这里扫的是**单个植入文件**（不是整包），所以"整包才判得准"的那两类也会出现：
	//   · 过期豁免 —— 豁免表是按整包登记的，只扫一个文件时其它文件的豁免必然"没命中"；
	//   · 计数漂移 —— 声明的命中数是按整包数出来的。
	// 两者都是**如实**的红（不是噪声）：任何一次扫描都会把"声明与眼前的源码不符"摆出来。
	if rep.RedCount < 1 {
		t.Errorf("红项数至少应为 1（植入点），实得 %d", rep.RedCount)
	}

	// 对侧：同一台审计器扫**本包真源码** ⇒ 全绿（含 dispatch.go 的豁免、match.go 的豁免、8 处 map 豁免）
	self, err := aud.AuditDir(".", AuditOptions{})
	if err != nil {
		t.Fatalf("AuditDir：%v", err)
	}
	if self.RedCount != 0 {
		t.Fatalf("本包自审必须全绿，实得红项 %d：%+v\n%s", self.RedCount, self.Findings, self.String())
	}
	if err := self.Verdict(); err != nil {
		t.Fatalf("本包自审 Verdict 应为 nil：%v", err)
	}
	if len(self.Items) != 10 {
		t.Fatalf("声明表应恰好十类（E8 原文逐项），实得 %d", len(self.Items))
	}
	seen := map[string]bool{}
	for _, row := range self.Items {
		if row.Name == "" || row.Status == "" || row.Mechanism == "" || row.Reason == "" {
			t.Errorf("十类里每一项都必须给 状态/机制/理由（不许留空）：%+v", row)
		}
		switch SourceStatus(row.Status) {
		case StatusFrozen, StatusUnfrozen, StatusNA:
		default:
			t.Errorf("%s 的状态 %q 不是三态之一", row.ID, row.Status)
		}
		if row.ExpectHits < 0 {
			t.Errorf("%s 未声明命中数（ExpectHits 必须写死）", row.ID)
		}
		if row.Drift {
			t.Errorf("%s 计数漂移（声明 %d 实际 %d）：改了源码就要改声明", row.ID, row.ExpectHits, row.Hits)
		}
		seen[row.ID] = true
	}
	for _, want := range []string{"time", "random", "net", "dns", "fs", "env", "goroutine", "map", "cpu", "locale"} {
		if !seen[want] {
			t.Errorf("声明表缺类 %s（E8 的十类必须逐项在）", want)
		}
	}
	// 表要与结论并列（"把每项的结论写成表"）
	tbl := self.Table()
	if !strings.Contains(tbl, "| 状态 |") || strings.Count(tbl, "\n") < 12 {
		t.Errorf("结论表形态不对：\n%s", tbl)
	}
	// 报告必须**可复现**（同一份源码两次扫描逐字节相同 ⇒ 扫描器自己不引入迭代序抖动）
	self2, err := aud.AuditDir(".", AuditOptions{})
	if err != nil {
		t.Fatalf("AuditDir（第二次）：%v", err)
	}
	b1, err1 := jsonOf(self)
	b2, err2 := jsonOf(self2)
	if err1 != nil || err2 != nil {
		t.Fatalf("序列化审计报告：%v / %v", err1, err2)
	}
	if string(b1) != string(b2) {
		t.Errorf("两次自审的报告必须逐字节相同（扫描器不得依赖 map 迭代序）")
	}
	// 门禁夹具用的落盘件（脚本读它打印审计表）
	root := fixtureRoot(t)
	writeReport(t, root, "audit.json", self)
}

// ── ⑦ 各类检测规则逐条钉死（含反例）────────────────────────────────────────

func TestDeterminismAuditDetectionRules(t *testing.T) {
	aud, err := DefaultAuditor()
	if err != nil {
		t.Fatalf("DefaultAuditor：%v", err)
	}

	// 植入：map 字段 range（应红）· 同名切片 range（不应红）· 局部 map range（应红）
	// · 环境变量读取（应红，无豁免）· 禁 import（子树判定）
	const planted = `package fake

import (
	"net/http"

	"golang.org/x/text/language"
)

type box struct {
	seen map[string]int
}

func (b *box) list() {
	for k := range b.seen {
		_ = k
	}
}

func sliceRange() {
	keys := []string{"a"}
	for _, k := range keys {
		_ = k
	}
}

func localMap() {
	m := map[string]int{"a": 1}
	for k := range m {
		_ = k
	}
}

func envRead() string {
	return getenvish()
}

func getenvish() string {
	return "x"
}

func fetch() {
	_, _ = http.Get("http://example.invalid")
}

func langs() string {
	return language.English.String()
}
`
	// 环境变量读取：单独一份，因为要让它落到 env 类（os.Getenv）
	envSrc := "package fake\n\nimport \"os\"\n\nfunc readEnv() string {\n\treturn os.Getenv(\"HOME\")\n}\n"

	rep, err := aud.AuditSources(map[string]string{"planted.go": planted, "env.go": envSrc}, AuditOptions{})
	if err != nil {
		t.Fatalf("AuditSources：%v", err)
	}
	byClass := map[string][]Finding{}
	for _, f := range rep.Findings {
		byClass[f.Class] = append(byClass[f.Class], f)
	}
	// map 类：恰好 2 处（字段 range + 局部 map range）；同名切片那处**不得**被误报
	if got := len(byClass["map"]); got != 2 {
		t.Errorf("map 类应恰好 2 处（字段 + 局部量），实得 %d：%+v", got, byClass["map"])
	}
	for _, f := range byClass["map"] {
		if f.Line == 25 { // `for _, k := range keys`（keys 是切片）
			t.Errorf("切片 range 被误报成 map range（%s:%d）：%s", f.File, f.Line, f.Text)
		}
		if !strings.Contains(f.Text, "所属函数") {
			t.Errorf("AST 命中应带所属函数名（按函数豁免要靠它）：%s", f.Text)
		}
	}
	// 网络 / locale（禁 import 子树）/ 环境变量
	if len(byClass["net"]) == 0 {
		t.Errorf("http.Get 应报 net 类")
	}
	if len(byClass["locale"]) == 0 {
		t.Errorf("golang.org/x/text/language 应被 locale 类的 import 子树判定拦下")
	}
	if len(byClass["env"]) != 1 || !strings.Contains(byClass["env"][0].Text, "Getenv") {
		t.Errorf("os.Getenv 应报 env 类（无豁免），实得 %+v", byClass["env"])
	}
	// 每条被报的类都必须在报告里标红（不能只记不判）
	for _, row := range rep.Items {
		if len(byClass[row.ID]) > 0 && !row.Red {
			t.Errorf("%s 有红项但行内没标红", row.ID)
		}
	}

	// 逐条：植入 goroutine / CPU 探测 / 随机 / 时间也都要报
	for _, tc := range []struct {
		src   string
		class string
	}{
		{"package fake\n\nfunc spawn() {\n\tgo func() {}()\n}\n", "goroutine"},
		{"package fake\n\nimport \"runtime\"\n\nfunc n() int {\n\treturn runtime.NumCPU()\n}\n", "cpu"},
		{"package fake\n\nimport \"math/rand\"\n\nfunc r() int {\n\treturn rand.Intn(10)\n}\n", "random"},
		{"package fake\n\nimport \"crypto/rand\"\n\nvar _ = rand.Reader\n", "random"},
		{"package fake\n\nimport \"time\"\n\nfunc s() {\n\ttime.Sleep(1)\n}\n", "time"},
		{"package fake\n\nimport \"net\"\n\nfunc l() error {\n\t_, err := net.LookupHost(\"x\")\n\treturn err\n}\n", "dns"},
		{"package fake\n\nimport \"os\"\n\nfunc t() {\n\t_ = os.TempDir()\n}\n", "fs"},
	} {
		r1, err := aud.AuditSources(map[string]string{"x.go": tc.src}, AuditOptions{})
		if err != nil {
			t.Fatalf("AuditSources（%s）：%v", tc.class, err)
		}
		hit := false
		for _, f := range r1.Findings {
			if f.Class == tc.class {
				hit = true
			}
		}
		if !hit {
			t.Errorf("植入的 %s 违规未被抓到：%+v", tc.class, r1.Findings)
		}
	}

	// 豁免按**函数**生效：给 localMap 配一条豁免 ⇒ 该处转为记账、不再标红；另一处仍红
	custom := append([]AuditItem(nil), AuditTable...)
	for i := range custom {
		if custom[i].ID == "map" {
			custom[i].Allow = append(append([]AllowRule(nil), custom[i].Allow...),
				AllowRule{File: "planted.go", Func: "localMap", Reason: "用例：局部 map 只做查表用"})
			custom[i].ExpectHits = -1
		}
	}
	aud2, err := NewAuditor(custom)
	if err != nil {
		t.Fatalf("NewAuditor（自定义表）：%v", err)
	}
	r2, err := aud2.AuditSources(map[string]string{"planted.go": planted}, AuditOptions{})
	if err != nil {
		t.Fatalf("AuditSources（自定义表）：%v", err)
	}
	allowedFn := 0
	for _, f := range r2.Allowed {
		if f.Class == "map" && strings.Contains(f.Text, "localMap") {
			allowedFn++
		}
	}
	if allowedFn != 1 {
		t.Errorf("按函数豁免应把 localMap 那处转为记账，实得 %d：%+v", allowedFn, r2.Allowed)
	}
	redMap := 0
	for _, f := range r2.Findings {
		if f.Class == "map" {
			redMap++
		}
	}
	if redMap != 1 {
		t.Errorf("另一处（字段 range）仍须红，实得 %d", redMap)
	}

	// 计数漂移 ⇒ 红
	drift := append([]AuditItem(nil), AuditTable...)
	for i := range drift {
		if drift[i].ID == "fs" {
			drift[i].ExpectHits = 999
		}
	}
	aud3, err := NewAuditor(drift)
	if err != nil {
		t.Fatalf("NewAuditor（漂移表）：%v", err)
	}
	r3, err := aud3.AuditSources(map[string]string{"x.go": "package fake\n\nfunc t() string { return \"\" }\n"}, AuditOptions{})
	if err != nil {
		t.Fatalf("AuditSources（漂移表）：%v", err)
	}
	if err := r3.Verdict(); err == nil || !strings.Contains(err.Error(), "计数漂移") {
		t.Errorf("计数漂移必须红并说清是漂移：%v", err)
	}

	// 过期豁免 ⇒ 红（豁免表腐化要能被看见）
	stale := []AuditItem{{
		ID: "env", Name: "环境变量", Status: StatusFrozen, Mechanism: "x", Reason: "y",
		Forbidden: []string{`os\.Getenv`}, ExpectHits: 0,
		Allow: []AllowRule{{File: "nowhere.go", Pattern: `os\.Getenv`, Reason: "这条永远命中不了"}},
	}}
	aud4, err := NewAuditor(stale)
	if err != nil {
		t.Fatalf("NewAuditor（过期豁免表）：%v", err)
	}
	r4, err := aud4.AuditSources(map[string]string{"x.go": "package fake\n"}, AuditOptions{})
	if err != nil {
		t.Fatalf("AuditSources：%v", err)
	}
	if len(r4.Stale) != 1 {
		t.Fatalf("应有 1 条过期豁免，实得 %+v", r4.Stale)
	}
	if err := r4.Verdict(); err == nil || !strings.Contains(err.Error(), "过期豁免") {
		t.Errorf("过期豁免必须红：%v", err)
	}

	// 「未冻结」⇒ 恒定标红（声明了没处置就得红，而不是"没命中就算过"）
	unfrozen := []AuditItem{{
		ID: "time", Name: "时间", Status: StatusUnfrozen, Mechanism: "尚无注入点", Reason: "待处置",
		ExpectHits: 0,
	}}
	aud5, err := NewAuditor(unfrozen)
	if err != nil {
		t.Fatalf("NewAuditor（未冻结表）：%v", err)
	}
	r5, err := aud5.AuditSources(map[string]string{"x.go": "package fake\n"}, AuditOptions{})
	if err != nil {
		t.Fatalf("AuditSources：%v", err)
	}
	if err := r5.Verdict(); err == nil || !strings.Contains(err.Error(), "未冻结") {
		t.Errorf("未冻结项必须标红：%v", err)
	}
	if r5.RedCount == 0 {
		t.Errorf("未冻结项应计入红项数")
	}

	// 零输入 / 空表 / 非法模式 ⇒ 报错（绝不空跑出结论）
	if _, err := aud.AuditSources(nil, AuditOptions{}); err == nil || !errors.Is(err, ErrDeterminismAudit) {
		t.Errorf("零源文件必须报错，实得 %v", err)
	}
	empty, err := NewAuditor(nil)
	if err != nil {
		t.Fatalf("NewAuditor(nil)：%v", err)
	}
	if _, err := empty.AuditSources(map[string]string{"x.go": "package fake\n"}, AuditOptions{}); err == nil {
		t.Errorf("空声明表必须报错")
	}
	if _, err := NewAuditor([]AuditItem{{ID: "x", Forbidden: []string{"("}}}); err == nil {
		t.Errorf("非法模式必须报错（不静默跳过一条模式）")
	}
	// 源码语法错 ⇒ 红（AST 检查会静默漏检，必须可见）
	rbad, err := aud.AuditSources(map[string]string{"bad.go": "package fake\nfunc broken( {\n"}, AuditOptions{})
	if err != nil {
		t.Fatalf("AuditSources（语法错）：%v", err)
	}
	if len(rbad.ParseErrors) != 1 {
		t.Fatalf("语法错必须进 ParseErrors，实得 %+v", rbad.ParseErrors)
	}
	if err := rbad.Verdict(); err == nil {
		t.Errorf("解析失败必须红")
	}
	// 测试文件默认不扫（夹具允许用 time/环境变量）；显式打开才扫
	// 注意：全是 _test.go 的输入集会被当成"零输入"报错（要结论就显式 IncludeTests），
	// 所以这里带一个非测试文件，验的是"同一个源集里测试文件被跳过"
	testsrc := map[string]string{"x_test.go": plantedTimeNow, "x.go": "package fake\n"}
	skip, err := aud.AuditSources(testsrc, AuditOptions{})
	if err != nil {
		t.Fatalf("AuditSources（测试源）：%v", err)
	}
	if len(skip.Findings) != 0 {
		t.Errorf("默认不扫 _test.go（夹具例外），实得 %+v", skip.Findings)
	}
	if len(skip.Scanned) != 1 || skip.Scanned[0] != "x.go" {
		t.Errorf("报告里应只扫到 x.go，实得 %v", skip.Scanned)
	}
	inc, err := aud.AuditSources(testsrc, AuditOptions{IncludeTests: true})
	if err != nil {
		t.Fatalf("AuditSources（含测试）：%v", err)
	}
	if len(inc.Findings) == 0 {
		t.Errorf("IncludeTests=true 时应扫到植入的取时点")
	}
	// 全测试源集 ⇒ 报错（不静默空转）
	if _, err := aud.AuditSources(map[string]string{"x_test.go": plantedTimeNow}, AuditOptions{}); err == nil {
		t.Errorf("全是 _test.go 且未开 IncludeTests 必须报错（不空跑出结论）")
	}
}
