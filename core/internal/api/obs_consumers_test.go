// obs_consumers_test.go — §二十一 已红第 10 条（T-32「`internal/audit` 与 `obs/redact` 的 import 消费者 = 0」）
// 的**机检棘轮 + 成对负控**。
//
// 为什么要有它：这条已红的判据本身就是一个 grep（两条 import 计数非 0）——**grep 判据不会自己跑**，
// 谁把那一行 import 删掉，仓里不会有任何东西报红（这正是「规则写了、没人接电」的形态）。
// 这里把那条 grep **落成每次 `go test` 都跑的断言**，再加两格行为断言：脱敏真的脱、指纹真的稳。
package api

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRootFromAPI 从 core/internal/api 往上找仓根。
func repoRootFromAPI(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("找不到仓根（从 %s 往上）", wd)
	return ""
}

// scanImportConsumers 扫 `core/**` + `agent/**` 的**非测试 .go**，找 import 行的文件。
//   - 只看**真 import 行**（行首制表符 + 带引号的导入路径），注释里提到不算（这正是已红第 10 条
//     「`git grep -l internal/audit` 的 3 件全是注释行」那个坑）；
//   - 排除被导入的包自身（audit 包 / redact 包自己的文件当然会提到自己的名字）。
func scanImportConsumers(t *testing.T, root, importPath string, excludeDirs []string) []string {
	t.Helper()
	line := regexp.MustCompile(`^\s*"` + regexp.QuoteMeta(importPath) + `"\s*$`)
	var hits []string
	for _, base := range []string{"core", "agent"} {
		_ = filepath.Walk(filepath.Join(root, base), func(p string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			for _, ex := range excludeDirs {
				if strings.HasPrefix(rel, ex) {
					return nil
				}
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			for _, l := range strings.Split(string(b), "\n") {
				if line.MatchString(l) {
					hits = append(hits, rel)
					break
				}
			}
			return nil
		})
	}
	return hits
}

// TestAuditAndRedactHaveRealConsumers —— 判据本身（两条 import 计数非 0）改成每次测试都跑。
func TestAuditAndRedactHaveRealConsumers(t *testing.T) {
	root := repoRootFromAPI(t)
	const auditPath = "github.com/Mr2109/zerg-swarm/core/internal/audit"
	const redactPath = "github.com/Mr2109/zerg-swarm/core/internal/obs/redact"

	auditConsumers := scanImportConsumers(t, root, auditPath, []string{"core/internal/audit"})
	redactConsumers := scanImportConsumers(t, root, redactPath, []string{"core/internal/obs/redact"})
	if len(auditConsumers) == 0 {
		t.Errorf("`%s` 的 import 消费者 = 0（已红第 10 条的形态：规则写了、没人接电）", auditPath)
	}
	if len(redactConsumers) == 0 {
		t.Errorf("`%s` 的 import 消费者 = 0（同上）", redactPath)
	}
	t.Logf("audit 消费者 %d 件：%v", len(auditConsumers), auditConsumers)
	t.Logf("redact 消费者 %d 件：%v", len(redactConsumers), redactConsumers)

	// 成对负控：**注释里提到不算**（已红原话「3 件全是注释行」）——
	// 拿一个只出现在注释里的路径试一次，必须扫不到。
	if got := scanImportConsumers(t, root, "github.com/Mr2109/zerg-swarm/core/internal/zz_只有注释提到", nil); len(got) != 0 {
		t.Errorf("扫描器把不存在的东西也当消费者了：%v", got)
	}
}

// TestRedactionActuallyRedacts —— 行为格：真链路上用的那个函数必须真的遮住**带键的令牌形状**。
//
// 口径（照实测，不照想当然）：脱敏管线遮的是 **key/value 形状**（哨兵键 + 冒号或等号 + 值）。
// 本机现跑（2026-09-20 · 批 C · T-32）四种形状逐条:
//
//	token=<64hex>            → token=[REDACTED]
//	token: <64hex>           → token=[REDACTED]
//	X-Auth-Token: <64hex>    → X-Auth-Token=[REDACTED]
//	{"token":"<64hex>"}      → {"token=[REDACTED]"}
//
// ★ 同一跑里测出的一处**缺口**（登记在批 C 记录「所见非本批」，关联定稿 `U-011`「64 位 hex 令牌
// 能否被遮住——未实测」⇒ 现在实测了）：**裸 64-hex 串**（不带键、不带分隔）**不被遮**；
// `Authorization: Bearer <64hex>` 只遮到键、Bearer 后面那截原样留着。本批**只报不改**
// （改脱敏规则要连它的语料与基线一起动 = 另一个面上的活，且设计稿明写要先跑 `redact-probes.sh`
// + 自定义语料才定论）——所以这一格在这里**只打日志、不写成断言**（不把没遮住写成「预期如此」）。
func TestRedactionActuallyRedacts(t *testing.T) {
	const tok = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 64 hex（虫族令牌形状）
	shapes := []string{
		"token=" + tok,
		"token: " + tok,
		"X-Auth-Token: " + tok,
		`{"token":"` + tok + `"}`,
	}
	for _, in := range shapes {
		out := redactedMessage(in)
		if strings.Contains(out, tok) {
			t.Errorf("脱敏没生效（带键的令牌形状也原样吐出）：%q → %q", in, out)
		}
		if !strings.Contains(out, "REDACTED") {
			t.Errorf("脱敏应是替换成标记而不是删值：%q → %q", in, out)
		}
	}
	// 缺口照实打印（每次跑都看得见；不写成断言 ⇒ 不把它固化成「预期」）
	t.Logf("[已知缺口 · 只报不判] 裸 64-hex 不被遮：%q → %q", tok, redactedMessage(tok))
	t.Logf("[已知缺口 · 只报不判] Authorization: Bearer 只遮键：%q", redactedMessage("Authorization: Bearer "+tok))

	// 成对：普通文本不许被改坏（脱敏 ≠ 乱改）——口径是「内容还在」（管线会做键值分隔符归一化，
	// 所以不做逐字节相等断言：实测 `失败：exit=1` 会被归一成 `失败:exit=1`）。
	plain := "任务 wait-test-2 失败：exit=1"
	out := redactedMessage(plain)
	for _, must := range []string{"任务 wait-test-2", "exit=1"} {
		if !strings.Contains(out, must) {
			t.Errorf("普通文本被改坏（丢了 %q）：%q → %q", must, plain, out)
		}
	}
}

// TestControlFingerprintIsStableAndSensitive —— 行为格：指纹「同参同值 · 异参异值」（日志↔响应 对账的前提）。
func TestControlFingerprintIsStableAndSensitive(t *testing.T) {
	a := controlFingerprint("config_reload", map[string]any{"host": "Mr2109", "n": 1})
	b := controlFingerprint("config_reload", map[string]any{"host": "Mr2109", "n": 1})
	c := controlFingerprint("config_reload", map[string]any{"host": "x3", "n": 1})
	if a == "" {
		t.Fatalf("指纹为空（audit.ArgsFingerprint 没接上）")
	}
	if a != b {
		t.Errorf("同参必须同值：%q ≠ %q", a, b)
	}
	if a == c {
		t.Errorf("异参必须异值：%q == %q", a, c)
	}
	if !strings.HasPrefix(a, "config_reload#") {
		t.Errorf("指纹形态应为 <op>#<fp>：%q", a)
	}
}
