package credential

// 真源的判据（GAP-20261017-04）：优先级链 · 空值回退 · 不回显。
// 判据跟着实现走：实现搬到 shared/credential，判据就搬到这里（✗ 别只测影子）。

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// tctSampleToken —— 合成令牌（不是任何真令牌）：判据只关心「它有没有出现在输出里」。
const tctSampleToken = "tok-7f3a2b9c-synthetic-not-a-secret"

// tctIsolate 把三路都清干净，并把 HOME 指到临时目录（文件级可控）。
func tctIsolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, k := range []string{EnvPrimary, EnvLegacyAuth, EnvLegacyAPI} {
		t.Setenv(k, "")
	}
	return home
}

func tctWriteTokenFile(t *testing.T, home, v string) {
	t.Helper()
	dir := filepath.Join(home, ".zerg")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("建令牌目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte(v+"\n"), 0o600); err != nil {
		t.Fatalf("写令牌文件失败: %v", err)
	}
}

// TestPriorityChainTwoStates —— 四级的**顺序**（谁赢谁死）+ 空值回退，逐格两态。
// 只断言取值与来源标记，✗ 不打印令牌值。
func TestPriorityChainTwoStates(t *testing.T) {
	cases := []struct {
		name    string
		primary string
		legacy  map[string]string
		file    string
		want    string
		wantSrc Source
	}{
		{"① env 主路独占", tctSampleToken, nil, "", tctSampleToken, SourceEnv},
		{"② file（无 env）", "", nil, tctSampleToken, tctSampleToken, SourceFile},
		{"③ 旧名 AUTH 独占", "", map[string]string{EnvLegacyAuth: tctSampleToken}, "", tctSampleToken, SourceLegacyAuth},
		{"④ 旧名 API 独占", "", map[string]string{EnvLegacyAPI: tctSampleToken}, "", tctSampleToken, SourceLegacyAPI},
		{"①>②③④ 主路压过 file 与两旧名", tctSampleToken, map[string]string{EnvLegacyAuth: "old-a", EnvLegacyAPI: "old-b"}, "file-c", tctSampleToken, SourceEnv},
		{"②>③ 文件压过旧名（旧名已降级）", "", map[string]string{EnvLegacyAuth: tctSampleToken}, "file-c", "file-c", SourceFile},
		{"②>④ 文件压过 API 旧名", "", map[string]string{EnvLegacyAPI: tctSampleToken}, "file-c", "file-c", SourceFile},
		{"③>④ AUTH 压过 API", "", map[string]string{EnvLegacyAuth: "auth-w", EnvLegacyAPI: "api-l"}, "", "auth-w", SourceLegacyAuth},
		{"全空 ⇒ 缺", "", nil, "", "", SourceMissing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := tctIsolate(t)
			if c.primary != "" {
				t.Setenv(EnvPrimary, c.primary)
			}
			for k, v := range c.legacy {
				t.Setenv(k, v)
			}
			if c.file != "" {
				tctWriteTokenFile(t, home, c.file)
			}
			got, src := ResolveWithSource()
			if got != c.want {
				t.Fatalf("取值不符：want %q · got %q", c.want, got)
			}
			if src != c.wantSrc {
				t.Fatalf("来源标记不符：want %q · got %q", c.wantSrc, src)
			}
			if got != "" && strings.Contains(string(src), got) {
				t.Fatalf("来源标记里出现了令牌值（五不进之日志）：%q", src)
			}
		})
	}
}

// TestEmptyValuesFallThrough —— 空值/空白**回退**：设置了但为空不算命中，得继续往下找。
func TestEmptyValuesFallThrough(t *testing.T) {
	t.Run("空白 env 回退到 file", func(t *testing.T) {
		home := tctIsolate(t)
		t.Setenv(EnvPrimary, "   \n\t")
		tctWriteTokenFile(t, home, tctSampleToken)
		if got, src := ResolveWithSource(); got != tctSampleToken || src != SourceFile {
			t.Fatalf("空白 ZERG_TOKEN 未回退：got=%q src=%q", got, src)
		}
	})
	t.Run("空白令牌文件回退到旧名", func(t *testing.T) {
		home := tctIsolate(t)
		tctWriteTokenFile(t, home, "   ")
		t.Setenv(EnvLegacyAuth, tctSampleToken)
		if got, src := ResolveWithSource(); got != tctSampleToken || src != SourceLegacyAuth {
			t.Fatalf("空白令牌文件未回退：got=%q src=%q", got, src)
		}
	})
	t.Run("令牌文件不存在回退到旧名", func(t *testing.T) {
		tctIsolate(t)
		t.Setenv(EnvLegacyAPI, tctSampleToken)
		if got, src := ResolveWithSource(); got != tctSampleToken || src != SourceLegacyAPI {
			t.Fatalf("缺文件未回退：got=%q src=%q", got, src)
		}
	})
	t.Run("HOME 不可得 ⇒ 退到旧名", func(t *testing.T) {
		tctIsolate(t)
		t.Setenv("HOME", "")
		t.Setenv(EnvLegacyAuth, tctSampleToken)
		if got, src := ResolveWithSource(); got != tctSampleToken || src != SourceLegacyAuth {
			t.Fatalf("HOME 为空时未回退：got=%q src=%q", got, src)
		}
	})
	t.Run("令牌前后空白去掉", func(t *testing.T) {
		tctIsolate(t)
		t.Setenv(EnvPrimary, "  "+tctSampleToken+"\n")
		if got, _ := ResolveWithSource(); got != tctSampleToken {
			t.Fatalf("未去首尾空白：%q", got)
		}
	})
}

// TestFromEnvironSameOrder —— 快照解析与真源**同序**（主控 agentEnv 那条旁链不再只认旧名）。
func TestFromEnvironSameOrder(t *testing.T) {
	env := []string{
		EnvLegacyAPI + "=api-l",
		EnvLegacyAuth + "=auth-w",
		EnvPrimary + "=" + tctSampleToken,
	}
	if got, src := FromEnviron(env); got != tctSampleToken || src != SourceEnv {
		t.Fatalf("快照主路未优先：got=%q src=%q", got, src)
	}
	env = []string{EnvLegacyAPI + "=api-l", EnvLegacyAuth + "=auth-w"}
	if got, src := FromEnviron(env); got != "auth-w" || src != SourceLegacyAuth {
		t.Fatalf("快照旧名次序不符：got=%q src=%q", got, src)
	}
	if got, src := FromEnviron([]string{"ZERG_GATEWAY_URL=http://x", EnvPrimary + "= "}); got != "" || src != SourceMissing {
		t.Fatalf("快照空值应判缺：got=%q src=%q", got, src)
	}
}

// TestAnnounceOnceSourceOnly —— 旧名命中**出声一次**：只报来源、不回显令牌；主路不出声。
func TestAnnounceOnceSourceOnly(t *testing.T) {
	tctIsolate(t)
	t.Setenv(EnvLegacyAuth, tctSampleToken)

	var buf bytes.Buffer
	oldSink, oldOnce := legacySink, legacyOnce
	legacySink = &buf
	legacyOnce = &sync.Once{}
	t.Cleanup(func() { legacySink = oldSink; legacyOnce = oldOnce })

	v, src := ResolveWithSource()
	if v != tctSampleToken || src != SourceLegacyAuth {
		t.Fatalf("旧名取值不符：got=%q src=%q", v, src)
	}
	AnnounceLegacy(src)
	AnnounceLegacy(src) // 第二次：不许再出声
	AnnounceLegacy(SourceEnv)
	AnnounceLegacy(SourceFile)

	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("旧名命中却没出声（要求提示一次）")
	}
	if lines := strings.Split(out, "\n"); len(lines) != 1 {
		t.Fatalf("旧名提示应恰好一次，实得 %d 行：%q", len(lines), out)
	}
	if !strings.Contains(out, string(SourceLegacyAuth)) {
		t.Fatalf("提示里没点名来源标记：%q", out)
	}
	if err := tokenLeakGuard(out, tctSampleToken); err != nil {
		t.Fatalf("%v", err)
	}
}

// TestMissingHintMachineReadable —— 缺令牌报话：机读键齐、链可对拍、**无令牌值**。
func TestMissingHintMachineReadable(t *testing.T) {
	tctIsolate(t)
	t.Setenv(EnvLegacyAuth, "") // 全空
	v, src := ResolveWithSource()
	if v != "" || src != SourceMissing {
		t.Fatalf("全空应判缺：got=%q src=%q", v, src)
	}
	hint := MissingHint()
	for _, want := range []string{"token_source=" + string(SourceMissing), "TOKEN-CHAIN: " + Chain} {
		if !strings.Contains(hint, want) {
			t.Fatalf("报话缺 %q：%q", want, hint)
		}
	}
	// 链的次序机读：四个名字按升序出现。
	last := -1
	for _, name := range []string{EnvPrimary, "~/.zerg/token", EnvLegacyAuth, EnvLegacyAPI} {
		i := strings.Index(Chain, name)
		if i <= last {
			t.Fatalf("Chain 次序不符：%q 在 %d（前一级 %d）· %q", name, i, last, Chain)
		}
		last = i
	}
	if err := tokenLeakGuard(hint, tctSampleToken); err != nil {
		t.Fatalf("%v", err)
	}
}

// tokenLeakGuard —— 判据本体：输出里出现令牌值 ⇒ 非 nil（红）。
func tokenLeakGuard(out, token string) error {
	if token == "" {
		return nil
	}
	if strings.Contains(out, token) {
		return fmt.Errorf("令牌回显：输出里出现了令牌值（len=%d）—— 五不进之「不进日志」被破坏", len(token))
	}
	return nil
}

// TestTokenLeakGuardNegativeControl —— 负控：故意把令牌写进日志格式 ⇒ 判据必红；
// 正控：只报来源 ⇒ 不判红（判据不能过宽）。
func TestTokenLeakGuardNegativeControl(t *testing.T) {
	if err := tokenLeakGuard(fmt.Sprintf("令牌来源 %s · token=%s\n", SourceLegacyAuth, tctSampleToken), tctSampleToken); err == nil {
		t.Fatal("负控失败：故意把令牌放进日志格式，判据仍绿 ⇒ 判据没牙")
	} else {
		t.Logf("负控读数：红 · %v", err)
	}
	if err := tokenLeakGuard("令牌来源 "+string(SourceLegacyAuth)+"\n", tctSampleToken); err != nil {
		t.Fatalf("正控失败：只报来源的输出被判红 ⇒ 判据过宽：%v", err)
	}
}
