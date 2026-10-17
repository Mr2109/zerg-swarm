// chat_gate_credential_test.go — GAP-20260927-67 的自检：凭据类路径/键名两侧口径。
//
// 判据（成对）：
//
//	· 正控：凭据类路径/键名（**合成串，非真凭据**）在命令面必须被 ChatGate 拦（与码面侧「点项默认不进」同向）。
//	· 负控①（伪装）：把凭据路径/变量**伪装成普通名形态**（`./.env` / `"/x/.env"` / `${VAR}` / `printenv VAR`）
//	  ⇒ **仍须被拦**（证明拦它的不是整串字面相等）。
//	· 负控②（口径哑化）：两侧 Stance 一旦分家，或码面侧（grep/glob）的点项缺省排除被拿掉 ⇒ 本判据必须**真红**。
package chat

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestChatGateCredentialClass — 命令面侧的凭据类拦截（正控 ＋ 负控① ＋ 防误杀）。
func TestChatGateCredentialClass(t *testing.T) {
	g := &ChatGate{}
	// ★ GAP-20261017-09（2026-10-17）：对话工具面收窄成「只能用虫族命令」⇒ 凭据类拦截的**判据面**
	// 从旧 `bash` 工具迁到 `zerg` 工具（ChatGate 先硬闸「首词逐字 zerg」，再拿命令体跑同一张规则表）。
	// 规则表本身一条未改：下面的正控/负控/防误杀清单逐字照旧，只是命令前加首词 zerg。
	mustBlock := []string{
		// 正控：凭据类路径/键名（合成串——✗ 不含任何真凭据值）
		`cat .env`, `cat .env.local`, `cp .env /tmp/x`, `head -n5 subdir/.env`,
		`cat id_rsa`, `cat ~/.ssh/id_ed25519`, `grep x auth.json`,
		`cat .zerg/token`, `cat ~/.aws/credentials`, `echo $ZERG_AUTH_TOKEN`,
		`printenv API_KEY`,
		// 负控①：伪装成普通路径/名形态 —— 仍须被拦
		`cat ./.env`, `cat "/tmp/x/.env"`, `cat sub/a/.env`, `cat .env.production`,
		`echo "${ZERG_AUTH_TOKEN}"`, `echo ${API_KEY}`, `printenv ZERG_TOKEN`,
	}
	for _, cmd := range mustBlock {
		d, err := g.Check("zerg", credCmdArgs(cmd), "chat")
		if err != nil {
			t.Fatalf("%s err=%v", cmd, err)
		}
		if d.Action != "block" {
			t.Errorf("凭据类命令 %q 必须被拦（安全面只许收紧），得到 %s", cmd, d.Action)
		}
	}
	// 防假阳性炸弹：正常开发命令不得误杀（收紧不许波及正常面）
	mustAllow := []string{
		`ls -la`, `go build -mod=mod ./...`, `docker run --env FOO=1 img`,
		`grep -rn environment .`, `cat x.env`, `printenv PATH`, `echo $HOME`,
		`zerg gate find check-password-argv`,
	}
	for _, cmd := range mustAllow {
		d, err := g.Check("zerg", credCmdArgs(cmd), "chat")
		if err != nil {
			t.Fatalf("%s err=%v", cmd, err)
		}
		if d.Action != "allow" {
			t.Errorf("正常命令 %q 不得误杀，得到 %s（msg=%s）", cmd, d.Action, d.Message)
		}
	}
}

// TestCredentialPoleConsistency — 负控②：两侧口径一致（机读 ＋ 跨侧真锚）。
func TestCredentialPoleConsistency(t *testing.T) {
	if CredentialGatePole == "" {
		t.Fatalf("CredentialGatePole 机读标记为空 ⇒ 口径不可机读（缺口 GAP-20260927-67）")
	}
	stances := map[string]int{}
	poles := 0
	for _, s := range CredentialPoleSides {
		stances[s.Stance]++
		if s.Pole {
			poles++
			if s.Side != "codepath" {
				t.Errorf("为准侧必须是 codepath（grep/glob 点项默认不进），得到 %q", s.Side)
			}
		}
	}
	if len(stances) != 1 {
		t.Errorf("凭据口径两侧 Stance 分家（%v）⇒ 口径哑化；两侧必须一致到严格侧（deny）", stances)
	}
	if poles != 1 {
		t.Errorf("CredentialPoleSides 必须恰有一格为准，得到 %d", poles)
	}
	// 跨侧真锚：码面侧（grep/glob）的点项缺省排除必须仍在源码里 —— 谁把它拿掉，本判据真红。
	root := credTestRepoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "core", "internal", "agent", "exec.go"))
	if err != nil {
		t.Fatalf("读码面侧真源 exec.go：%v", err)
	}
	src := string(body)
	if n := strings.Count(src, `strings.HasPrefix(fi.Name(), ".")`); n < 2 {
		t.Errorf("码面侧 grep 的点件缺省排除只剩 %d 处（应 ≥2）⇒ 码面侧被放宽，两侧口径分家（缺口 GAP-20260927-67）", n)
	}
	if n := strings.Count(src, `strings.HasPrefix(seg, ".")`); n < 1 {
		t.Errorf("码面侧 glob 的点项缺省排除只剩 %d 处（应 ≥1）⇒ 码面侧被放宽，两侧口径分家（缺口 GAP-20260927-67）", n)
	}
}

// credCmdArgs — 按真模型发来的形态造 zerg 工具 args（命令前补首词 zerg —— GAP-20261017-09 后的判据面）——
// ✗ 别用「裸拼」 `{"command":"<cmd>"}`：cmd 带 `"` 时会造出**非法 JSON**，extractCommand 会在
// 第一个裸 `"` 处截断 ⇒ 本判据测的就不是真链路了（实测踩过）。
func credCmdArgs(cmd string) string { return `{"command":` + strconv.Quote("zerg "+cmd) + `}` }

// credTestRepoRoot 从包目录上溯找仓根（认 core/internal/agent 这棵树）。
func credTestRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "core", "internal", "agent")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("找不到仓根（从 %s 上溯）", wd)
	return ""
}
