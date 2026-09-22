package api

// ============ CA 日志目录落点（/tmp 硬编码治理批 D——2026-09-18）============
//
// 缺陷（改前）: master_scheduler.go 派 CA 任务时写死字面量
//
//	cmd.Env = append(baseEnv, "ZERG_LOG_DIR=/tmp/zerg-ca-logs")
//
// 而「读日志的一侧」走 statepath.CAEventLogRoot()（覆盖顺序 ZERG_CA_LOG_DIR → <ZERG_TMP_DIR|/tmp>/zerg-ca-logs）:
//   - core/internal/api/handlers.go findTaskLogDir（UI 轮次/耗时/取证关联）
//   - core/internal/api/handlers.go 任务日志尾部读取
//
// 两侧仅在没有覆盖变量时**巧合**一致 ⇒ 一旦设了官方开关 ZERG_CA_LOG_DIR
// （publish/docs/CONFIGURATION 记载），读在新根、写还在 /tmp/zerg-ca-logs——CA 日志读不到。
// 改后: 写侧 env 由 caLogEnv() 从 statepath.CAEventLogRoot() 派生。
// env **名**必须仍是 ZERG_LOG_DIR——CA 子进程（core/cmd/zerg-agent/main.go 建日志处）
// 只认这个字面量，不认 ZERG_CA_LOG_DIR；用例 3 用「只读 $ZERG_LOG_DIR」的假 CA 脚本把这条契约钉住。
//
// 隔离: 全部用例把日志根指到 t.TempDir()（ZERG_TMP_DIR / ZERG_CA_LOG_DIR），
// 绝不读写真机 /tmp/zerg-ca-logs（真机在跑的任务日志就在那），且从不删除任何目录/文件。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// caEnvValue 从 env 切片取 key 的值（取**最后一个**——与 os/exec 追加语义一致：后面的追加项才是产品意图）
func caEnvValue(env []string, key string) (string, bool) {
	prefix := key + "="
	val, found := "", false
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			val, found = strings.TrimPrefix(kv, prefix), true
		}
	}
	return val, found
}

// dirSnapshot 目录内容快照（相对路径 → 文件 sha256 / 目录标记）——用于证明「旧目录没被删、没被改」
func dirSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			if rel != "." {
				snap[rel] = "<dir>"
			}
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(b)
		snap[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("快照失败（%s）: %v", root, err)
	}
	return snap
}

// fakeCAScript 假 CA：落点解析逐字照抄 core/cmd/zerg-agent/main.go 建日志的语义
// （只读 ZERG_LOG_DIR；空则本脚本报错退出——生产链路必须一直给它）。
// 写出 <ZERG_LOG_DIR>/<ZERG_FAKE_TS>/{audit.jsonl,events.jsonl}，并把落点打到 stdout。
//
// 安全闸（2026-09-18 变异实验教训）: 必须显式许可根（ZERG_FAKE_ALLOW_ROOT，精确相等）才落盘。
// 变异实验把产品代码改回写死 /tmp/zerg-ca-logs 时，无此闸的假 CA 会真的往**真机**
// /tmp/zerg-ca-logs 里建目录（已实测发生一次，事后清除并逐字节复原）——
// 用例必须能「红」而不能有副作用。
const fakeCAScript = `#!/bin/sh
set -eu
root="${ZERG_LOG_DIR:-}"
if [ -z "$root" ]; then
  echo "❌ 日志创建失败: ZERG_LOG_DIR 未设置" >&2
  exit 3
fi
if [ "$root" != "${ZERG_FAKE_ALLOW_ROOT:-}" ]; then
  echo "❌ 拒绝写非许可根: $root（许可=${ZERG_FAKE_ALLOW_ROOT:-<未设>}；防污染真机 /tmp/zerg-ca-logs）" >&2
  exit 4
fi
d="$root/$ZERG_FAKE_TS"
mkdir -p "$d"
printf '{"task_id":"%s","started":"2026-09-18T00:00:00Z"}\n' "$ZERG_TASK_DIR" > "$d/audit.jsonl"
printf '{"Type":"callModel","Seq":1}\n' >> "$d/events.jsonl"
printf '%s\n' "$d"
`

// runFakeCA 用**产品代码产出的 env 项**（caLogEnv()）跑一次假 CA，返回它真实落盘的日志目录。
// 这条路径就是生产里 runTask 的 spawn 口径（cmd.Env 追加 caLogEnv()）——少了/改了这条，CA 日志就断。
// allowRoot 是唯一允许落盘的位置（t.TempDir() 下）——落点跑偏时脚本拒绝写盘并让用例变红。
func runFakeCA(t *testing.T, scriptDir, allowRoot, ts, taskDir string) (string, []string) {
	t.Helper()
	script := filepath.Join(scriptDir, "fake-ca.sh")
	if err := os.WriteFile(script, []byte(fakeCAScript), 0o755); err != nil {
		t.Fatalf("写假 CA 脚本失败: %v", err)
	}
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		caLogEnv(), // ← 产品代码产出的那一项（env 名 + 取值都来自它）
		"ZERG_TASK_DIR=" + taskDir,
		"ZERG_FAKE_TS=" + ts,
		"ZERG_FAKE_ALLOW_ROOT=" + allowRoot,
	}
	cmd := exec.Command("/bin/sh", script)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("假 CA 执行失败（日志根未落在许可根 %s 内）: %v\n输出: %s\nenv: %v", allowRoot, err, out, env)
	}
	got := strings.TrimSpace(string(out))
	if got == "" {
		t.Fatalf("假 CA 未打印落点\nenv: %v", env)
	}
	if got != filepath.Join(allowRoot, ts) {
		t.Fatalf("假 CA 落点异常: %q（许可根 %s）", got, allowRoot)
	}
	return got, env
}

// TestMasterScheduler_CALogDir_DefaultNotHardcodedTmp — 用例1: 默认落点不再写死 /tmp 字面量。
//
// 断言口径: ZERG_TMP_DIR 一改，CA 日志根跟着改且**不在 /tmp 下**——
// 写死字面量的旧实现无论 ZERG_TMP_DIR 怎么设都返回 /tmp/zerg-ca-logs（本用例必红）。
func TestMasterScheduler_CALogDir_DefaultNotHardcodedTmp(t *testing.T) {
	base := t.TempDir()
	t.Setenv("ZERG_CA_LOG_DIR", "") // 空=未设（statepath 按 TrimSpace 空判未设）
	t.Setenv("ZERG_TMP_DIR", base)

	want := filepath.Join(base, "zerg-ca-logs")
	got, ok := caEnvValue([]string{caLogEnv()}, "ZERG_LOG_DIR")
	if !ok {
		t.Fatal("caLogEnv() 未产出 ZERG_LOG_DIR 项——CA 子进程（只认该字面量）将无日志根")
	}
	if got != want {
		t.Fatalf("CA 日志根未随 statepath 派生: 期望 %q，实得 %q（写死 /tmp 即此断言红）", want, got)
	}
	// ★ 2026-09-23 波B（设计-CI适配-v1.1 §五 第二批 #3 · §八 待拍 2 裁定）：删掉旧断言
	//   「不以 /tmp/ 开头」—— 它在 TMPDIR=/tmp（Linux runner）下恒假，是自伤而非判据；
	//   真命题已由上一条 `got != want`（= ZERG_TMP_DIR 派生值）判过 ⇒ 不留第二条。
	if sp := statepath.CAEventLogRoot(); got != sp {
		t.Fatalf("写侧 env 与读侧 statepath.CAEventLogRoot() 不同源: 写 %q / 读 %q", got, sp)
	}
}

// TestMasterScheduler_CALogDir_DefaultEqualsLegacyLiteral — 用例2: 默认值与旧字面量逐字节等价（零回归）。
//
// 无任何覆盖时，派生结果必须仍是 /tmp/zerg-ca-logs——即改前写死的那条值，
// 生产既有部署（真机 /tmp/zerg-ca-logs 里的历史任务日志）照旧可读。
func TestMasterScheduler_CALogDir_DefaultEqualsLegacyLiteral(t *testing.T) {
	t.Setenv("ZERG_CA_LOG_DIR", "")
	t.Setenv("ZERG_TMP_DIR", "")

	const legacyLiteral = "/tmp/zerg-ca-logs" // 改前的写死值——仅作对照基准（治理目标就是不再依赖它）
	got, ok := caEnvValue([]string{caLogEnv()}, "ZERG_LOG_DIR")
	if !ok {
		t.Fatal("caLogEnv() 未产出 ZERG_LOG_DIR 项")
	}
	if got != legacyLiteral {
		t.Fatalf("默认落点与旧值不等价（既有部署的 CA 日志会读不到）: 期望 %q，实得 %q", legacyLiteral, got)
	}
	if sp := statepath.CAEventLogRoot(); sp != legacyLiteral {
		t.Fatalf("statepath 默认值漂移（约定：tmpBase 默认字面 /tmp）: %q", sp)
	}
}

// TestMasterScheduler_CALogDir_OverrideWriterReaderAgree — 用例3: ZERG_CA_LOG_DIR 覆盖对**写读两侧**同时生效。
//
// 改前: 读侧（findTaskLogDir）认 ZERG_CA_LOG_DIR，写侧写死 /tmp ⇒ 日志写进 /tmp、UI 去新根找 ⇒ 读不到（本用例红）。
// 改后: 假 CA（只认 $ZERG_LOG_DIR）落在覆盖根里，findTaskLogDir 正好找到它——写读同源。
func TestMasterScheduler_CALogDir_OverrideWriterReaderAgree(t *testing.T) {
	override := filepath.Join(t.TempDir(), "ca-logs-override")
	t.Setenv("ZERG_CA_LOG_DIR", override)
	t.Setenv("ZERG_TMP_DIR", "")

	if got, _ := caEnvValue([]string{caLogEnv()}, "ZERG_LOG_DIR"); got != override {
		t.Fatalf("ZERG_CA_LOG_DIR 覆盖未传到写侧 env: 期望 %q，实得 %q", override, got)
	}
	sp := statepath.CAEventLogRoot()
	if sp != override {
		t.Fatalf("statepath 覆盖未生效: %q", sp)
	}

	ts := "2026-09-18T13-00-00"
	taskDir := filepath.Join(t.TempDir(), "zerg-tasks", "task-cadir-override-1")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("建任务目录失败: %v", err)
	}
	wrote, _ := runFakeCA(t, t.TempDir(), override, ts, taskDir)

	wantDir := filepath.Join(override, ts)
	if wrote != wantDir {
		t.Fatalf("CA 日志落点错: 期望 %q，实得 %q", wantDir, wrote)
	}
	if strings.HasPrefix(wrote, "/tmp/zerg-ca-logs") {
		t.Fatalf("设了覆盖仍写死旧根: %q", wrote)
	}
	if _, err := os.Stat(filepath.Join(wrote, "events.jsonl")); err != nil {
		t.Fatalf("CA 未在派生根写出 events.jsonl: %v", err)
	}
	// 读侧（生产同一条路径）必须找到刚写的那份——「CA 日志仍可读」
	if got := findTaskLogDir("task-cadir-override-1"); got != wrote {
		t.Fatalf("读侧找不到写侧日志: findTaskLogDir=%q，实际落点=%q", got, wrote)
	}
}

// TestMasterScheduler_CALogDir_LegacyDirStillReadableNotDeleted — 用例4: 旧目录（默认根）里的历史日志仍可读、不被删改。
//
// 场景: 上报目录已存在（真机 /tmp/zerg-ca-logs 的形状——时间戳子目录 + audit.jsonl/events.jsonl），
// 新任务再落同一根: ① 历史日志仍被 findTaskLogDir 关联到（可读）；② 旧目录与旧文件字节不变（不删不改）。
func TestMasterScheduler_CALogDir_LegacyDirStillReadableNotDeleted(t *testing.T) {
	base := t.TempDir()
	t.Setenv("ZERG_CA_LOG_DIR", "")
	t.Setenv("ZERG_TMP_DIR", base) // 默认根 = <base>/zerg-ca-logs（与真机 /tmp/zerg-ca-logs 同形状）

	oldRoot := filepath.Join(base, "zerg-ca-logs")
	oldDir := filepath.Join(oldRoot, "2026-09-17T22-44-53")
	oldTaskDir := filepath.Join(base, "zerg-tasks", "task-cadir-legacy-1")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("建旧日志目录失败: %v", err)
	}
	if err := os.MkdirAll(oldTaskDir, 0o755); err != nil {
		t.Fatalf("建旧任务目录失败: %v", err)
	}
	audit := fmt.Sprintf("{\"task_id\":\"%s\",\"started\":\"2026-09-17T22:44:53+08:00\"}\n", oldTaskDir)
	if err := os.WriteFile(filepath.Join(oldDir, "audit.jsonl"), []byte(audit), 0o644); err != nil {
		t.Fatalf("写旧 audit.jsonl 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "events.jsonl"), []byte("{\"Type\":\"callModel\",\"Seq\":1}\n"), 0o644); err != nil {
		t.Fatalf("写旧 events.jsonl 失败: %v", err)
	}
	before := dirSnapshot(t, oldRoot)

	// ① 旧日志可读（默认口径——历史 CA 日志照旧被关联）
	if got := findTaskLogDir("task-cadir-legacy-1"); got != oldDir {
		t.Fatalf("旧目录历史日志读不到: findTaskLogDir=%q，期望 %q", got, oldDir)
	}
	// ② 新任务仍落默认根（与旧日志同根——写读口径未变）
	wrote, _ := runFakeCA(t, base, oldRoot, "2026-09-18T13-10-00", filepath.Join(base, "zerg-tasks", "task-cadir-legacy-2"))
	if filepath.Dir(wrote) != oldRoot {
		t.Fatalf("新任务未落默认根: %q（期望 %s 下）", wrote, oldRoot)
	}
	// ③ 旧目录/旧文件字节不变——不删不改
	after := dirSnapshot(t, oldRoot)
	for rel, sum := range before {
		gotSum, ok := after[rel]
		if !ok {
			t.Fatalf("旧条目被删: %s", rel)
		}
		if gotSum != sum {
			t.Fatalf("旧条目被改: %s（%s → %s）", rel, sum, gotSum)
		}
	}
	keys := make([]string, 0, len(after))
	for k := range after {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t.Logf("旧根快照（前后一致）: %v", keys)
}

// TestMasterScheduler_CALogDir_NoHardcodedLiteralInSource — 用例5: 源码级契约（钉住调用点，不只钉 caLogEnv 本体）。
//
// 为什么需要: 用例1-4 调的是 caLogEnv()，若有人把**调用点**改回
//
//	cmd.Env = append(baseEnv, "ZERG_LOG_DIR=/tmp/zerg-ca-logs")
//
// 单测仍会全绿而缺陷复活（变异自证里实测过这一支）。
// 断言: 非注释行里不得再出现写死的日志根字面量；且派发处确实调用了 caLogEnv()。
func TestMasterScheduler_CALogDir_NoHardcodedLiteralInSource(t *testing.T) {
	entries, err := os.ReadDir(".") // go test 的工作目录 = 包目录
	if err != nil {
		t.Fatalf("读包目录失败: %v", err)
	}
	callSiteFound := false
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", name, err)
		}
		scanned++
		for i, line := range strings.Split(string(b), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") { // 注释允许出现历史字面量（记录缺陷用）
				continue
			}
			if strings.Contains(line, "caLogEnv()") {
				callSiteFound = true
			}
			for _, bad := range []string{`ZERG_LOG_DIR=/`, `ZERG_LOG_DIR=` + "`" + `/`} {
				if strings.Contains(line, bad) {
					t.Fatalf("%s:%d 仍写死 CA 日志根: %s", name, i+1, trimmed)
				}
			}
		}
	}
	if scanned == 0 {
		t.Fatal("未扫到任何非测试源码——源契约校验形同虚设")
	}
	if !callSiteFound {
		t.Fatal("产品代码里没有 caLogEnv() 调用——派发 CA 的日志根不再由 statepath 派生（缺陷可能复活）")
	}
}
